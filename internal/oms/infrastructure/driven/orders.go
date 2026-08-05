package driven

import (
	"errors"
	"fmt"
	"sync"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §3.6 rule 2's compile-time assertion. It is the check that this adapter still
// satisfies the port after either side is edited, and it is worth having even
// though module.go will construct one: the wiring proves the interface is met
// only in the binary that wires it, while this fails in `go build ./...`.
var _ ports.OrderWriteRepository = (*MemoryOrderWriteRepository)(nil)

// The same assertion for the read port §6.7's classifier holds. It is a second
// line rather than a wider first one because the two ports are deliberately
// separate — see ports/hedgegroup.go — and an assertion naming only the write
// repository would go on compiling if OrdersIn were deleted.
var _ ports.HedgeGroupOrders = (*MemoryOrderWriteRepository)(nil)

// And for the read port §5.3's gate 3 rules sweep rows out against — a third
// line, on the same argument as the second.
var _ ports.VenueOrderIndex = (*MemoryOrderWriteRepository)(nil)

// This adapter's own refusals, both about §5.1's account component.
var (
	// ErrRepositoryAccountUnset reports a store constructed with no account.
	ErrRepositoryAccountUnset = errors.New("oms write repository: no account configured")

	// ErrRepositoryAccountMismatch reports a hedge group whose account is not
	// the one this store holds.
	//
	// It is refused rather than answered with an empty result, and the two are
	// not interchangeable: an empty result is a real state §6.7 classifies as
	// Determined with a zero residual, so the wrong account would return the one
	// answer that unblocks a compensating trade sized from nothing.
	ErrRepositoryAccountMismatch = errors.New("oms write repository: hedge group names a different account")

	// ErrRepositoryVenueOrderIDUnset reports an ApplyAck with no id to record.
	// The absent id's one spelling is an empty OrderRecord.VenueOrderID, which
	// is what a record has until this method runs — so an empty argument could
	// only ever be a caller that forgot to check its ack, recorded as a write
	// that changed nothing and bumped the version anyway.
	ErrRepositoryVenueOrderIDUnset = errors.New("oms write repository: ack carries no venue order id")

	// ErrRepositoryOrderNotAtVenue reports an ApplyAck for an order whose
	// state says no venue side exists to have acked it — StatePlanned, the
	// one state MayExistAtVenue answers false for. Application declares its
	// own sentinel for the same predicate on the join path
	// (ErrOrderNotAtVenue); they are not shared because no caller branches
	// across the two, and an adapter error a use case imported would be the
	// coupling ports/errors.go exists to prevent — in the other direction.
	ErrRepositoryOrderNotAtVenue = errors.New("oms write repository: order state admits no venue ack")
)

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

	// account is the venue subaccount whose orders this store holds — §5.1's
	// first hedge-group component, and the one an order record cannot carry.
	//
	// # It is here because §5.1's key has a component §5.1's Order type does not
	//
	// §5.1 makes the OMS lease exclusive "per hedge-group (account, venue,
	// symbol)" and puts Account first in the fill dedupe key. Its own Order field
	// list has no account in it, so an order record carries the venue and the
	// symbol and nothing that says whose they are. Before this field, the only
	// Account in the whole system was domain.Fill's — a field whose consumer
	// (the dedupe key) is real and whose producer does not exist yet.
	//
	// So one store holds one account's orders, and OrdersIn refuses any other.
	// That makes the single-account assumption a CHECKED premise instead of an
	// invisible one, and the direction it fails in is why it is worth a field: a
	// classifier that netted two subaccounts would sum a naked long in one
	// against a naked short in the other to a residual of ZERO — positions and
	// margin are per subaccount, so none of that netting is real. The group reads
	// flat, no compensating trade is sized, and both legs stay naked with every
	// invariant over the group agreeing. It is the rare failure here that
	// concludes rather than halts.
	//
	// If a deployment ever holds two subaccounts, the shape is two stores — or an
	// Account on the record, which is a §5.1 edit. Named so it is a decision
	// rather than a rediscovery.
	account string

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

	// authorizedAt is §5.3's anchor, unset until the order reaches
	// StateDispatchAuthorized. It is written once and never rewritten: SM §1
	// gives DispatchAuthorized no incoming edge from anywhere, so a second write
	// could only come from a second authorization — which is the thing the whole
	// dispatch path exists to prevent.
	authorizedAt clock.ServerTime

	// venueOrderID is the venue's identifier from the Place ack, empty until
	// ApplyAck records one. Write-once like the anchor, but the once is
	// ENFORCED here rather than inherited from SM §1's edge set: no state
	// machine forbids a second ack, so ApplyAck's own conflict check is the
	// only thing standing between this field and two authorities.
	venueOrderID string
}

// NewMemoryOrderWriteRepository returns an empty repository for one venue
// subaccount.
//
// The account is required and empty is refused, which is this constructor's only
// refusal. It is required rather than defaulted because the two candidate
// defaults are both wrong in the concluding direction: an empty account makes
// every subaccount's orders one hedge group (see the field's doc), and a
// hard-coded placeholder makes a real deployment's mismatch invisible until the
// day a second account exists — at which point every call site already compiles.
func NewMemoryOrderWriteRepository(account string) (*MemoryOrderWriteRepository, error) {
	if account == "" {
		return nil, ErrRepositoryAccountUnset
	}
	return &MemoryOrderWriteRepository{
		account: account,
		records: make(map[ids.ClientOrderID]*record),
		ledger:  domain.NewLedger(),
	}, nil
}

// OrdersIn is ports.HedgeGroupOrders: every order record in one §5.1 hedge
// group.
//
// # It matches on venue and symbol, and ASSERTS the account
//
// §5.1's key has three components and a record carries two of them, so the
// account cannot be matched — it can only be checked against the one this store
// was built for. A mismatch is refused rather than answered with an empty slice,
// because the two are indistinguishable to a caller and only one of them is a
// real answer: an empty group means "no orders", which §6.7 classifies as
// Determined with a zero residual and which unblocks a compensating trade sized
// from nothing.
//
// ProductCategory and ContractType are deliberately not compared. That is the
// whole of §5.1's "deliberately grouping a trade's spot + perp legs": the two
// legs of one cash-and-carry are different instruments, and their exposures
// netting IS the group.
func (r *MemoryOrderWriteRepository) OrdersIn(group domain.HedgeGroup) ([]ports.OrderRecord, error) {
	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("write repository: %w", err)
	}
	if group.Account != r.account {
		return nil, fmt.Errorf("write repository holds account %q, asked for group %s: %w",
			r.account, group, ErrRepositoryAccountMismatch)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var out []ports.OrderRecord
	for _, rec := range r.records {
		if !group.Holds(rec.order.Instrument) {
			continue
		}
		snapshot, err := r.snapshot(rec)
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", group, err)
		}
		out = append(out, snapshot)
	}
	return out, nil
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

// ApplyAuthorized moves the order to the crash boundary and records §5.3's
// anchor.
//
// The instant is refused before anything is looked up, and refused for being
// unset rather than for being implausible. There is no plausibility check
// available — any instant a venue clock could be at is a legitimate value — so
// the one thing this method can establish is that a caller supplied a
// calibrated reading at all. An unset ServerTime reaches every later comparison
// as Uncertain, which is safe but silent: the order would sit at the crash
// boundary with an anchor that refuses every question, and nothing would say
// when it stopped being answerable.
func (r *MemoryOrderWriteRepository) ApplyAuthorized(id ids.ClientOrderID, expected int64, at clock.ServerTime) (ports.OrderRecord, error) {
	if err := at.Validate(); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("authorize order %q: %w: %w", id, ports.ErrAuthorizationNeedsAnchor, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rec, err := r.advance(id, expected, domain.StateDispatchAuthorized)
	if err != nil {
		return ports.OrderRecord{}, err
	}
	rec.authorizedAt = at
	return r.snapshot(rec)
}

// ApplyAck records the venue's identifier for an order, refusing what the
// port requires refused.
//
// # The two contradiction checks are ONE rule with two witnesses, and the
// # error direction of each is the same
//
// The rule is the port's: one venue id, one owner, per instrument. A record
// already holding a different id and a different record already holding this
// id are the two ways a store can come to violate it, and both are refused
// with the same sentinel because a caller's recovery is identical — nothing,
// this is not recoverable by retry; it is the banked non-reuse premise
// failing, or an adapter mis-threading acks, and either way the index built
// over this store must never be consulted again until an operator has looked.
//
// The cross-order scan is O(records) under the lock, and that is accepted
// rather than indexed away: this method runs once per dispatched order, the
// map it would maintain is a second copy of a fact the records already state,
// and the copy is the one that drifts.
func (r *MemoryOrderWriteRepository) ApplyAck(id ids.ClientOrderID, expected int64, venueOrderID string) (ports.OrderRecord, error) {
	if venueOrderID == "" {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q: %w", id, ErrRepositoryVenueOrderIDUnset)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[id]
	if !ok {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q: %w", id, ports.ErrOrderUnknown)
	}
	if rec.version != expected {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q: record is at version %d, write expected %d: %w",
			id, rec.version, expected, ports.ErrVersionConflict)
	}

	// A Planned order has no venue side to have acked it, so the call
	// describes a history §5.3 says cannot have happened — the same predicate
	// the join and the replay report use, called rather than restated.
	mayExist, err := rec.state.MayExistAtVenue()
	if err != nil {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q at %s: %w", id, rec.state, err)
	}
	if !mayExist {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q at %s: %w", id, rec.state, ErrRepositoryOrderNotAtVenue)
	}

	if rec.venueOrderID == venueOrderID {
		// §7.5's replay may meet the same acked record twice; restating a fact
		// is not a change, so the version holds still.
		return r.snapshot(rec)
	}
	if rec.venueOrderID != "" {
		return ports.OrderRecord{}, fmt.Errorf("ack order %q: record holds venue id %q, ack names %q: %w",
			id, rec.venueOrderID, venueOrderID, ports.ErrVenueOrderIDConflict)
	}
	for otherID, other := range r.records {
		if otherID != id && other.venueOrderID == venueOrderID &&
			other.order.Instrument == rec.order.Instrument {
			return ports.OrderRecord{}, fmt.Errorf("ack order %q: venue id %q already names order %q on %s: %w",
				id, venueOrderID, otherID, rec.order.Instrument, ports.ErrVenueOrderIDConflict)
		}
	}

	rec.venueOrderID = venueOrderID
	rec.version++
	return r.snapshot(rec)
}

// VenueOrderIDsOn is ports.VenueOrderIndex: venue id -> owner for one
// instrument.
//
// The one-owner-per-key contract is inherited from ApplyAck rather than
// re-checked here — this loop cannot meet two records sharing an id on one
// instrument because the only writer of the field refused to create them, and
// a second detector would be a branch no test could reach through this type's
// public surface.
func (r *MemoryOrderWriteRepository) VenueOrderIDsOn(instrument ids.VenueInstrument) (map[string]ids.ClientOrderID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(map[string]ids.ClientOrderID)
	for id, rec := range r.records {
		if rec.venueOrderID == "" || rec.order.Instrument != instrument {
			continue
		}
		out[rec.venueOrderID] = id
	}
	return out, nil
}

// ApplyState moves the order along SM §1, under OCC.
//
// # DispatchAuthorized is refused HERE, on the target alone
//
// The port requires it, and the ordering is the part worth reading: the refusal
// is before the lookup, the version check and the legality check, so it fires
// for a caller that reached for the wrong method regardless of what is stored.
// Any later position would let a version conflict or a forbidden-edge error be
// the diagnosis instead — both of which describe a race, when what happened is
// that an authorization was attempted with no instant to resolve the order
// from.
//
// # The two remaining checks are ordered, and the order is the diagnosis
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
	if to == domain.StateDispatchAuthorized {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q to %s: %w",
			id, to, ports.ErrAuthorizationNeedsAnchor)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rec, err := r.advance(id, expected, to)
	if err != nil {
		return ports.OrderRecord{}, err
	}
	return r.snapshot(rec)
}

// advance is the load-check-mutate the two Apply*State methods share.
//
// Shared rather than duplicated because the checks are one rule — SM §1's edge
// set under OCC — and the two entry points differ only in what they do AFTER
// it. The caller must hold r.mu.
//
// Mutation happens only once nothing further can fail — domain.Ledger.Apply's
// discipline, for its reason: a refused write must leave no half-applied state,
// and a caller's rollback cannot reach a map this method already wrote.
func (r *MemoryOrderWriteRepository) advance(id ids.ClientOrderID, expected int64, to domain.OrderState) (*record, error) {
	rec, ok := r.records[id]
	if !ok {
		return nil, fmt.Errorf("advance order %q to %s: %w", id, to, ports.ErrOrderUnknown)
	}
	if rec.version != expected {
		return nil, fmt.Errorf("advance order %q: record is at version %d, write expected %d: %w",
			id, rec.version, expected, ports.ErrVersionConflict)
	}
	if err := domain.CanTransition(rec.state, to); err != nil {
		return nil, fmt.Errorf("advance order %q: %w", id, err)
	}

	rec.state = to
	rec.version++
	return rec, nil
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
	fees, feesErr := r.ledger.Fees(id)
	if err := errors.Join(cumulativeErr, fillsErr, feesErr); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("write repository: order %q: %w", id, err)
	}

	return ports.OrderRecord{
		Order:        rec.order,
		State:        rec.state,
		Cumulative:   cumulative,
		Fills:        fills,
		Fees:         fees,
		AuthorizedAt: rec.authorizedAt,
		VenueOrderID: rec.venueOrderID,
		Version:      rec.version,
	}, nil
}
