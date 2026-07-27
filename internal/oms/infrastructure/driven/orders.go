package driven

import (
	"errors"
	"fmt"
	"sync"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §3.6 rule 2's compile-time assertion. It is the check that this adapter still
// satisfies the port after either side is edited, and it is worth having even
// though module.go will construct one: the wiring proves the interface is met
// only in the binary that wires it, while this fails in `go build ./...`.
var _ ports.OrderWriteRepository = (*MemoryOrderWriteRepository)(nil)

// firstVersion is the version an order's record is created at.
//
// One rather than zero, so the zero value of ports.OrderRecord is unusable in
// the sense the rest of this codebase means: an expected version of 0 comes
// from a caller that never loaded a record, and it must match nothing. See
// ports.OrderRecord.Version.
const firstVersion int64 = 1

// MemoryOrderWriteRepository is §3.6 rule 6's write repository for order and
// fill state, backed by process memory.
//
// # Safe for concurrent use, and it is the first type in oms that is
//
// domain.Ledger and application.Dispatcher are both deliberately
// unsynchronised, and the Ledger's comment says where the synchronisation went:
// "§5.1 gives the aggregate an OCC Version and §6 gives it a journal-then-apply
// write path, so serialisation is a property of how a write reaches this type,
// decided one layer out." This is that layer, and it takes BOTH halves of the
// problem, because they are two different failures:
//
//   - The mutex answers memory safety. Two goroutines writing one Go map is
//     undefined behaviour, and the race detector reports it as a crash — which
//     is the good case, since the alternative is a corrupted map read hours
//     later.
//   - The version answers lost updates. §6's writes come from genuinely
//     independent producers — the private order stream, the executions stream,
//     the REST recovery sweep, §6.7's finality timers — and a mutex makes those
//     two writers take turns while leaving the second one's read stale. The
//     collision that matters is not detectable from the state set: two writers
//     reading an order at Acknowledged and advancing it to Canceled and
//     PartiallyFilled are both making a LEGAL transition. Only the version
//     notices.
//
// Neither substitutes for the other, which is why the Ledger's refusal to hold
// a mutex is not contradicted here. Its point was that a mutex ALONE buys
// safety from the detector and not from the defect.
type MemoryOrderWriteRepository struct {
	mu sync.Mutex

	// records is the current-state store: what §6's apply step writes and what
	// the running system reads instead of replaying the journal.
	records map[ids.ClientOrderID]*record

	// ledger is the domain's cumulative-execution ledger, used rather than
	// reimplemented. Dedupe, the never-closing admission window and the
	// over-fill rule are all §5.3 rules that already have exactly one home, and
	// the adapter's job is to serialise access to that home and version its
	// effects — not to hold a second opinion about which fills count.
	ledger *domain.Ledger
}

// record is the mutable half of an order.
//
// It holds no cumulative quantity and no fill count: those live in the ledger,
// and a copy here would be a second statement of a fact that has one owner.
// snapshot reads them back out at the moment a record is handed to a caller,
// so the two cannot drift because there is nothing to keep in step.
type record struct {
	order   domain.Order
	state   domain.OrderState
	version int64
}

// NewMemoryOrderWriteRepository returns an empty repository.
//
// It refuses nothing and returns no error, unlike this repo's other
// constructors, because it has no dependency to refuse. The absence is worth a
// sentence only because the pattern elsewhere is the opposite: those
// constructors take ports and reject a nil one, and there is nothing to hand
// this constructor that could be wrong.
func NewMemoryOrderWriteRepository() *MemoryOrderWriteRepository {
	return &MemoryOrderWriteRepository{
		records: make(map[ids.ClientOrderID]*record),
		ledger:  domain.NewLedger(),
	}
}

// ApplyPlanned creates the record for a newly planned order.
//
// The duplicate check is domain.Ledger.Register's rather than this method's,
// and the delegation is deliberate: §5.3's "unique per submission, never reused
// within the recoverable window" is one rule, and the ledger already refuses a
// repeat with the diagnosis of why neither resolution is safe. Registering
// first also means a refused order leaves nothing behind here — the map write
// happens only once the ledger has accepted the id.
func (r *MemoryOrderWriteRepository) ApplyPlanned(o domain.Order) (ports.OrderRecord, error) {
	if err := o.Validate(); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("write repository: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ledger.Register(o.ClientOrderID, o.Instrument, o.Quantity); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("write repository: %w", err)
	}
	rec := &record{
		order:   o,
		state:   domain.StatePlanned,
		version: firstVersion,
	}
	r.records[o.ClientOrderID] = rec
	return r.snapshot(rec)
}

// Load returns the current record.
func (r *MemoryOrderWriteRepository) Load(id ids.ClientOrderID) (ports.OrderRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[id]
	if !ok {
		return ports.OrderRecord{}, fmt.Errorf("load order %q: %w", id, ports.ErrOrderUnknown)
	}
	return r.snapshot(rec)
}

// ApplyState moves the order along SM §1, under OCC.
//
// # The two checks are ordered, and the order is the diagnosis a reader gets
//
// Version first, legality second. A caller holding a stale version has a stale
// idea of the state as well, so the transition it computed may be illegal
// against what is actually stored — and reporting THAT would send somebody to
// SM §1's edge table to explain a race. The conflict is the cause; the
// forbidden edge is its symptom, and only the cause is actionable ("re-read and
// re-decide", ports.ErrVersionConflict).
//
// The legality check itself is domain.CanTransition, called rather than
// restated: SM §1's edge set has one home, and an adapter that inlined even a
// subset of it would be the second.
func (r *MemoryOrderWriteRepository) ApplyState(id ids.ClientOrderID, expected int64, to domain.OrderState) (ports.OrderRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[id]
	if !ok {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q to %s: %w", id, to, ports.ErrOrderUnknown)
	}
	if rec.version != expected {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q: record is at version %d, write expected %d: %w",
			id, rec.version, expected, ports.ErrVersionConflict)
	}
	if err := domain.CanTransition(rec.state, to); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q: %w", id, err)
	}

	// Mutation happens only once nothing further can fail — domain.Ledger.Apply's
	// discipline, for its reason: a refused write must leave no half-applied
	// state, and a caller's rollback cannot reach a map this method already
	// wrote.
	rec.state = to
	rec.version++
	return r.snapshot(rec)
}

// ApplyFill admits an execution, reporting whether it changed the record.
//
// # The version is checked before the dedupe, and that is not the free choice
// # it looks like
//
// A duplicate cannot lose an update, so refusing one on a stale version buys
// nothing arithmetically — the tempting order is dedupe first, answer
// (record, false, nil), and never bother the caller. It is refused on
// domain.Ledger.Apply's own argument, one layer up: that method puts its dedupe
// check AFTER the order and instrument checks because "the second copy is
// exactly where a mapping error becomes visible". A stale writer is the same
// kind of evidence. Its duplicate fill is harmless; the fact that something in
// this process is holding a version nobody else has is not, and silence there
// hides precisely the second-writer condition §3.6 rule 6 exists to prevent.
func (r *MemoryOrderWriteRepository) ApplyFill(f domain.Fill, expected int64) (ports.OrderRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[f.Order]
	if !ok {
		return ports.OrderRecord{}, false, fmt.Errorf("apply execution %q naming order %q: %w",
			f.ExecutionID, f.Order, ports.ErrOrderUnknown)
	}
	if rec.version != expected {
		return ports.OrderRecord{}, false, fmt.Errorf("apply execution %q to order %q: record is at version %d, write expected %d: %w",
			f.ExecutionID, f.Order, rec.version, expected, ports.ErrVersionConflict)
	}

	// Validation, dedupe, the instrument check and the over-fill rule are all
	// the ledger's. Nothing is re-checked here; see the field comment.
	applied, err := r.ledger.Apply(f)
	if err != nil {
		return ports.OrderRecord{}, false, fmt.Errorf("write repository: %w", err)
	}
	if !applied {
		// A no-op leaves the version alone. Bumping it would invalidate every
		// other reader's expectation to record that nothing happened.
		current, err := r.snapshot(rec)
		return current, false, err
	}

	rec.version++
	current, err := r.snapshot(rec)
	return current, true, err
}

// snapshot assembles the value a caller sees, reading the executed quantity
// back out of the ledger rather than from a field this type maintains.
//
// The caller must hold r.mu: it reads both stores, and a snapshot assembled
// across a concurrent write would pair one order's state with another write's
// cumulative quantity — the one interleaving that produces a record which is
// individually plausible and jointly wrong.
func (r *MemoryOrderWriteRepository) snapshot(rec *record) (ports.OrderRecord, error) {
	id := rec.order.ClientOrderID

	// Both reads are joined into ONE refusal rather than checked separately,
	// and the mutation harness is what settled that. Each accessor fails for
	// exactly one reason — the ledger does not hold the order — so two
	// sequential checks are two guards on one fact, and neither can be deleted
	// without the other still catching it. Both mutations therefore SURVIVED
	// while the claim itself was checked, which is the "second copy of one
	// rule" shape this repo has paid for before. Joined, there is one branch,
	// it is necessary, and orders_internal_test.go reaches it.
	cumulative, cumulativeErr := r.ledger.Cumulative(id)
	fills, fillsErr := r.ledger.FillCount(id)
	if err := errors.Join(cumulativeErr, fillsErr); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("write repository: order %q: %w", id, err)
	}

	return ports.OrderRecord{
		Order:      rec.order,
		State:      rec.state,
		Cumulative: cumulative,
		Fills:      fills,
		Version:    rec.version,
	}, nil
}
