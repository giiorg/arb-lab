package application

import (
	"errors"
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Replayer is the first half of build-spec §7.5's reconcile-only boot: "state is
// rebuilt from the write-ahead journal", before any venue call.
//
// # What it is for, stated against the store it writes into
//
// §6's write path is journal-then-apply. The journal is durable and survives the
// process; the current-state projection is process memory and does not. So a
// fresh process holds a repository that knows nothing and a log that knows
// everything, and this use case is the one that turns the second into the first.
// Every other use case in this package reads the projection as though it were
// true, which it is exactly as far as this one has run.
//
// # It writes through the SAME port every other writer uses, deliberately
//
// The alternative — a bulk load that installs records into the repository
// directly — is faster and gives up the property that makes the rebuilt state
// worth having: replaying through ApplyPlanned/ApplyAuthorized/ApplyState means
// the projection can only reach states the LIVE path could have produced. SM
// §1's edge set is checked on every replayed transition, the OCC version
// advances exactly as it did the first time, and domain.Ledger.Register refuses
// a duplicate ClientOrderID at boot for the same reason it refuses one at
// planning. A journal recording an impossible history is therefore refused
// rather than installed, which is the difference between a projection that is a
// replay of the log and one that merely came from it.
//
// # What this slice does NOT rebuild, and why the absence is not a gap
//
// Cumulative executed quantity. ports.Journal has no AppendFill — it is
// "narrowed to what the dispatch path writes" — so no fill has ever been
// journaled and no replay can produce one. Every record this use case rebuilds
// therefore has a zero Cumulative and zero Fills, and §7.5 says where the
// number comes from instead: the journal is JOINED TO VENUE TRUTH, and fills
// are the venue's (Fills, and the executions sweep §5.3 already uses). That
// join is slice F2, and its inputs are the venue clients this milestone builds.
//
// The consequence must be read the right way round, because the wrong reading
// is the dangerous one: after this use case runs, an order's STATE is
// authoritative and its executed QUANTITY is not yet a claim at all. Nothing
// may size a compensating trade from a residual computed over these records
// until the venue join has run — which is precisely what boot.Checklist is for,
// since GateNetDeltaAsserted is a separate gate from GateJournalReplayed and
// the posture is the weakest of the two.
type Replayer struct {
	journal ports.JournalReplay
	orders  ports.OrderWriteRepository
}

// NewReplayer constructs the use case, refusing a missing port.
//
// It holds the journal's READ half and no ports.Journal, so a replay cannot
// append — see ports.JournalReplay for why that composition is the one recovery
// must not have.
func NewReplayer(j ports.JournalReplay, orders ports.OrderWriteRepository) (*Replayer, error) {
	if j == nil {
		return nil, ErrJournalReplayMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	return &Replayer{journal: j, orders: orders}, nil
}

// Recovery is what one replay rebuilt.
//
// The counts are for the operator report §7.5 implies and for the tests, and
// Unresolved is the field with consequences: it is this slice's whole handoff to
// the venue join.
type Recovery struct {
	// Records is how many journal records were replayed, and LastSequence is the
	// sequence of the last one. Both are zero for an empty journal — a first
	// boot, which is a successful replay of nothing.
	//
	// They are two numbers rather than one because they answer different
	// questions and disagree exactly when something is wrong: the sequence is
	// dense from 1, so LastSequence != Records means the walk skipped a record,
	// which the chain verification is supposed to have refused. Carrying both
	// makes that a checkable claim rather than an assumption.
	Records      int
	LastSequence uint64

	// Orders is how many distinct orders the journal created.
	Orders int

	// Duplicates is how many records restated a fact the projection already
	// held — a state record naming the state the order was in, or an acked
	// record naming the venue id it already carried.
	//
	// §5.3's duplicate deliveries make those legitimate — journalcodec.go says
	// so in as many words, which is why a replay must not treat one as a corrupt
	// log — and they are counted rather than ignored because a journal that is
	// mostly duplicates means something upstream is re-delivering, and nothing
	// else in the system would report it.
	Duplicates int

	// Unresolved names the orders whose replayed state means the VENUE may hold
	// something we cannot account for: everything §6.7 counts as contributing a
	// possible-delta interval — DispatchAuthorized through CancelUnknown.
	//
	// It is the set §5.3 says recovery must resolve by query, and it is exactly
	// the set the ambiguity classifier will refuse to conclude around. Sorted, so
	// two identical journals produce two identical reports; a boot alert that
	// names its causes in map order is one an operator learns to distrust.
	//
	// StatePlanned is deliberately NOT in it, and that is domain's distinction
	// rather than this type's: "a Planned-only order was never dispatched (no
	// venue order can exist)", so there is nothing at the venue to ask about and
	// a query would run to its deadline over an order this system merely has not
	// sent. Using ContributesPossibleDelta rather than negating IsTerminal is
	// what keeps that one difference in one place.
	Unresolved []ids.ClientOrderID

	// Dispatched names every order the venue may hold a record of, sorted, and
	// it is a SUPERSET of Unresolved.
	//
	// # The difference between the two lists is where this system's fills are
	//
	// Unresolved is §6.7's question — which orders' exposure is still uncertain —
	// so it holds no terminal. Dispatched is §7.5's — which orders can the venue
	// be ASKED about — so it holds every terminal except the one that was never
	// sent. That gap is not an edge case: §6 step 6 sends marketable IOC limits,
	// whose ordinary outcome is terminal within milliseconds, so the orders that
	// actually moved the position are exactly the ones the uncertain-set excludes.
	//
	// The venue join takes THIS list, and a join that took Unresolved instead
	// would rebuild executed quantity for the orders that have none and leave a
	// zero cumulative on the ones that filled — which the §6.7 classifier reads
	// as a flat, DETERMINED group, and a determined group is the one where
	// compensating trades are permitted.
	Dispatched []ids.ClientOrderID
}

// Rebuild replays the journal into the write repository.
//
// # err != nil means the projection is NOT usable, and the caller must not
// # trade on it
//
// It reports two failures a caller has to tell apart, and telling them apart is
// the whole reason ports.ErrJournalChainBroken exists as a sentinel:
//
//   - The chain does not verify. The log itself cannot be trusted, no retry can
//     change that, and reconciliation writes computed from it are exactly the
//     ones §7.5 would otherwise re-drive. The caller reports
//     boot.OutcomeEvidenceUnusable — halted AND latched — and that combination
//     exists because of this call site.
//   - The log verifies and does not compose: a state record for an order that
//     was never planned, a transition SM §1 has no edge for, a planned record
//     that will not rebuild into a valid order. The log is intact and this build
//     cannot make a state out of it, which is a different report and a different
//     conversation with the operator.
//
// A partial rebuild is left in the repository on failure rather than rolled
// back, and that is deliberate: the records applied before the failure are the
// ones the verified prefix of the log states, and discarding them would leave an
// operator inspecting an empty store while the log says otherwise. Nothing may
// trade on it — the posture is what enforces that, not the emptiness of a map.
func (r *Replayer) Rebuild() (Recovery, error) {
	rb := &rebuild{orders: r.orders}
	if err := r.journal.Replay(rb); err != nil {
		return Recovery{}, fmt.Errorf("rebuild from journal: %w", err)
	}
	return rb.report()
}

// rebuild is the ports.JournalVisitor half, and it is UNEXPORTED for a reason
// worth stating: it is the only thing in this package holding a write
// repository it did not get from a use case's constructor, and every record it
// applies has been verified by the walk that handed it over. Exported, it would
// be a way to write arbitrary orders into the projection with no journal record
// behind them — the shape ports.HedgeGroupOrders exists to deny the classifier,
// arriving through recovery instead.
type rebuild struct {
	orders ports.OrderWriteRepository

	// created is every order the journal brought into existence, in the order
	// the records created them.
	//
	// It holds IDS and no states, and the emptiness of it is the decision: the
	// repository is where an order's state lives, and a map from id to state
	// kept alongside would be a second copy of a fact with one owner — able to
	// disagree with it in exactly the case that matters, a record the repository
	// refused. The report reads the state back out per order, which costs one
	// Load each on a path that already does one per state record.
	created []ids.ClientOrderID

	records    int
	lastSeq    uint64
	duplicates int
}

// Planned applies §6 step 5's record: an order this system decided to place.
func (b *rebuild) Planned(seq uint64, o domain.Order) error {
	b.seen(seq)
	if _, err := b.orders.ApplyPlanned(o); err != nil {
		return fmt.Errorf("journal record %d: replay planned order %q: %w", seq, o.ClientOrderID, err)
	}
	b.created = append(b.created, o.ClientOrderID)
	return nil
}

// Authorized applies §6 step 6's record, WITH the anchor — which is the whole
// reason this slice added the record kind.
//
// The instant reaches ApplyAuthorized unchanged, so a recovered order's anchor
// is the one the live process stamped rather than a boot-time approximation of
// it. Approximating would be worse than leaving it unset: §5.3's two bounds are
// both measured FORWARD from this instant, so a later value makes both expire
// later, and a resolver that refuses late is the one failure direction the
// stable-not-found rule is designed to avoid.
func (b *rebuild) Authorized(seq uint64, id ids.ClientOrderID, at clock.ServerTime) error {
	b.seen(seq)

	rec, err := b.orders.Load(id)
	if err != nil {
		return fmt.Errorf("journal record %d: replay authorization of order %q: %w", seq, id, err)
	}
	if _, err := b.orders.ApplyAuthorized(id, rec.Version, at); err != nil {
		return fmt.Errorf("journal record %d: replay authorization of order %q: %w", seq, id, err)
	}
	return nil
}

// Acked applies one venue-identifier record.
//
// The repository's own semantics do the two jobs this method would otherwise
// hold rules for: re-recording the id the order already holds is ApplyAck's
// no-op (counted below as a duplicate, on State's rationale — a journal that
// is mostly restatements means something upstream re-delivered), and an id
// that CONTRADICTS the store — this order under a different id, or the id
// naming another order on the instrument — is ApplyAck's refusal, which fails
// the replay. That failure is the crash-path half of the conflict design
// (ports.ErrVenueOrderIDConflict): a journal holding both sides of a
// contradiction verifies as a log and does not compose as a history, and a
// boot that cannot compose its history does not trade.
func (b *rebuild) Acked(seq uint64, id ids.ClientOrderID, venueOrderID string) error {
	b.seen(seq)

	rec, err := b.orders.Load(id)
	if err != nil {
		return fmt.Errorf("journal record %d: replay ack of order %q: %w", seq, id, err)
	}
	if rec.VenueOrderID == venueOrderID {
		b.duplicates++
		return nil
	}
	if _, err := b.orders.ApplyAck(id, rec.Version, venueOrderID); err != nil {
		return fmt.Errorf("journal record %d: replay ack of order %q: %w", seq, id, err)
	}
	return nil
}

// State applies one lifecycle record.
//
// # A record restating the state the order is already in is a NO-OP, not a
// # failure, and the journal's own format comment is the authority
//
// journalcodec.go puts the sequence inside the hashed preimage precisely
// because "§5.3's duplicate deliveries mean AppendState may legitimately record
// the same order reaching the same state twice" — two records that are
// byte-identical in every other field. domain.CanTransition answers
// ErrTransitionNoOp for that edge, and a replay that treated it as a broken log
// would refuse to boot over a journal the spec says is well-formed.
//
// It is checked BEFORE the write rather than by inspecting the error the write
// returns, because the two are not the same claim: the repository refuses a
// self-edge on the CURRENT state of its record, and what a replay needs to know
// is whether THIS record advances the history it is rebuilding. They agree
// today; they would part company the moment anything else could write to the
// repository during a replay, and then the version conflict would be reported
// as a duplicate delivery.
func (b *rebuild) State(seq uint64, id ids.ClientOrderID, s domain.OrderState) error {
	b.seen(seq)

	rec, err := b.orders.Load(id)
	if err != nil {
		return fmt.Errorf("journal record %d: replay state %s of order %q: %w", seq, s, id, err)
	}
	if rec.State == s {
		b.duplicates++
		return nil
	}
	if _, err := b.orders.ApplyState(id, rec.Version, s); err != nil {
		return fmt.Errorf("journal record %d: replay state %s of order %q: %w", seq, s, id, err)
	}
	return nil
}

// seen records the walk's own arithmetic, in one place so the three visitor
// methods cannot count differently.
func (b *rebuild) seen(seq uint64) {
	b.records++
	b.lastSeq = seq
}

// report derives the Recovery by reading each order's FINAL state back out of
// the projection.
//
// The unresolved set is computed here rather than accumulated during the walk,
// and the two readings disagree on a real case: an order that reached
// DispatchAuthorized and then Filled is resolved, while a walk that added ids as
// it saw them would have to remember to remove it. Deriving from the final state
// makes the intermediate ones unable to contribute — the property expressed as
// the way the value is computed rather than as a rule the accumulation has to
// follow.
//
// A Load failure here is fatal to the report rather than skipped. A missing
// order is not a resolved order: the id is in this list because a journal record
// created it, so a repository that will not produce it means the projection and
// the log disagree about what exists, and a report that quietly omitted it would
// under-count exactly the orders §7.5 must resolve.
//
// The ContributesPossibleDelta error is the one arm no test can reach, and it is
// named rather than dropped: every state read here came back from the
// repository, which refuses an undeclared one, so a mutation deleting this
// branch survives the suite. What covers it is the domain's own switch, which
// exhaustiveswitch refuses to let go stale — the same standing this package's
// ErrEffectUnhandled arm has, recorded for the same reason.
func (b *rebuild) report() (Recovery, error) {
	rec := Recovery{
		Records:      b.records,
		LastSequence: b.lastSeq,
		Orders:       len(b.created),
		Duplicates:   b.duplicates,
	}
	for _, id := range b.created {
		current, err := b.orders.Load(id)
		if err != nil {
			return Recovery{}, fmt.Errorf("report on replayed order %q: %w", id, err)
		}
		contributes, err := current.State.ContributesPossibleDelta()
		if err != nil {
			return Recovery{}, fmt.Errorf("report on replayed order %q at %s: %w", id, current.State, err)
		}
		if contributes {
			rec.Unresolved = append(rec.Unresolved, id)
		}

		// Derived from its own predicate rather than from `contributes ||
		// IsTerminal`, which is the same set today and is the composition that
		// silently stops holding: the two lists answer different questions, and
		// expressing one as arithmetic over the other would make a change to
		// §6.7's uncertain-set quietly change which orders §7.5 asks the venue
		// about.
		atVenue, err := current.State.MayExistAtVenue()
		if err != nil {
			return Recovery{}, fmt.Errorf("report on replayed order %q at %s: %w", id, current.State, err)
		}
		if atVenue {
			rec.Dispatched = append(rec.Dispatched, id)
		}
	}
	sort.Slice(rec.Unresolved, func(i, j int) bool { return rec.Unresolved[i] < rec.Unresolved[j] })
	sort.Slice(rec.Dispatched, func(i, j int) bool { return rec.Dispatched[i] < rec.Dispatched[j] })
	return rec, nil
}

// ChainBroken reports whether a Rebuild failure was the hash chain rather than
// the log's contents.
//
// A named predicate rather than leaving every caller to write
// errors.Is(err, ports.ErrJournalChainBroken): the two failures map to two
// different boot outcomes with two different clearance rules, and the branch is
// the one place a reader will look for the distinction. It also keeps the
// sentinel's package out of the boot sequence's imports, which is the same
// reason ports/errors.go gives for where sentinels live.
func ChainBroken(err error) bool { return errors.Is(err, ports.ErrJournalChainBroken) }
