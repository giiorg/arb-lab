package application

import (
	"errors"
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
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

	// Generations is how many distinct `PackageDelever` generations the journal
	// opened, and LiveGenerations carries the ones it never ended.
	//
	// # LiveGenerations is what a REINSTATEMENT is built from, and this is why
	//
	// A generation that the log opened and did not end held an entry in the
	// SHARED reserved-reduction ledger when the process died. That ledger is
	// §6.7's `outstanding_reserved_reductions` — the subtrahend every
	// compensating trade is sized from — and it lives in process memory, so a
	// fresh process starts with it empty.
	//
	// The failure direction is why the entry cannot simply be left out. A ledger
	// missing an entry reads as MORE room, not less: §2's determined branch sizes
	// a compensating trade from a residual that is too large by exactly the
	// generation's outstanding net intent, so it buys back the exposure that
	// generation's own perp leg already committed to buy. That is FM-5's shape
	// arriving through recovery rather than through a booking error, and it is
	// silent — every invariant that would object is computed FROM the ledger.
	//
	// # It carries the PAYLOADS now, and #203's reason for not carrying them has
	// # expired
	//
	// Slice #203 made this a list of bare IDs and said why: an unread field is a
	// worse defect than a named gap. Reinstater is the reader, so the gap closes
	// and the fields land. What this use case still does NOT do is re-`Reserve`
	// anything — that needs the Classifier the replayer deliberately does not
	// hold, and the decomposition that keeps it out is the whole of §75h: the
	// walk that decides what the records MEAN holds no mutator, and a separate
	// use case performs the reinstatement from the values it produced.
	//
	// Sorted by id, for Unresolved's reason.
	Generations     int
	LiveGenerations []GenerationRecovery

	// BarredLadders names the hedge groups whose escalation ladder a
	// supersession CLOSED — SM §7's "a supersession ends the GENERATION; whether
	// it ends the LADDER is a property of the PATH", read off the log.
	//
	// It is derived from the ENDED generations, which is the half of the record
	// stream LiveGenerations throws away, and it is the reason the two lists are
	// not one filtered list: an ended generation contributes nothing to the
	// ledger and exactly one thing to recovery, and that thing is this.
	//
	// The failure direction is the mirror of LiveGenerations'. A bar that is lost
	// across a restart lets core §7.1's ladder open a generation against an
	// `EmergencyClosePackage` or `FenceReadyLost` arbitration that has taken the
	// package away from it — this machine re-authorising itself against a
	// foundation decision, which is the one thing SupersessionPath.ResumesLadder
	// exists to answer. Sorted by group.
	BarredLadders []LadderBar

	// MarginOps is how many distinct core §7.1 margin operations the journal
	// dispatched, and the two lists below are the ones that still constrain this
	// process.
	//
	// # Why there are TWO lists and not one filtered one
	//
	// They carry different things because they need different work, and the split
	// is the stored-verdict ruling made visible (record §81 §2a). InFlight is an
	// op with no verdict at all: recovery must CONCLUDE it, which needs the whole
	// record — the delta, the before-state and the watermark are three parts of
	// one comparison. Ambiguous is an op whose verdict was already reached and
	// written down, so there is nothing left to compute and the id is the whole
	// of what a report needs.
	//
	// # The Ambiguous list is the reason the verdict is in the log at all
	//
	// §7.5 blocks new openings on an ambiguous margin op "until the staged-float
	// ledger is explicitly reconciled" — a fact about the OP, which outlives the
	// process that discovered it. Recomputed at each boot instead of read back,
	// it would be re-derived from a posted-margin delta that has drifted further
	// since, so a restart could conclude Applied and lift a §9 incident with
	// nobody having looked. Read back, a restart cannot launder it.
	//
	// # The four collections PARTITION the dispatched ops, and that is the
	// # invariant to test rather than each list's contents
	//
	// InFlight (no verdict), Ambiguous (an open §9 incident), Reconciled (one an
	// operator ended, task #213) and Settled (applied or not_applied — finished
	// business). Every dispatched op is in exactly one, which is what makes
	// "this id is in none of them" mean UNKNOWN TO THE LOG and nothing else. The
	// reconciler's diagnostic rests on that: an operator who typed the wrong id
	// and an operator naming an op the evidence settled are two different
	// conversations, and a partition is what keeps them from collapsing into
	// "not an open ambiguity".
	//
	// Settled exists ONLY for that distinction — no gate reads it, because a
	// settled op constrains nothing. It is the one collection here that is not
	// about what the boot must do.
	//
	// Sorted by id where ordered, for Unresolved's reason.
	MarginOps           int
	InFlightMarginOps   []domain.MarginOp
	AmbiguousMarginOps  []domain.MarginOpID
	ReconciledMarginOps []domain.MarginOpReconciliation
	SettledMarginOps    map[domain.MarginOpID]domain.MarginOpStatus
}

// GenerationRecovery is one `PackageDelever` generation exactly as the journal
// recorded it — the values SM §7's ladder fixed, plus the ordered phases it
// reached.
//
// # It is the RECORD and not a rebuilt aggregate, and the split is the seam
//
// Rebuilding needs two things this walk must not have: the shared ledger, and
// the venue's executed quantities (there is no perp-fill record — see
// Deleverer.RecordPerpFill, whose stated reason for not writing one was wrong in
// both halves and true in the conclusion). So the walk produces a VALUE and
// Reinstater consumes it, which is how the replayer stays without a Classifier
// while the reinstatement still happens (§75h).
//
// Every field is what one record kind carried, so a field's absence is a record
// the log does not hold rather than a payload this type dropped.
type GenerationRecovery struct {
	// ID, Group, Reservation and Target are the opening record's payload —
	// everything SM §7 fixes for the generation's lifetime.
	ID          domain.GenerationID
	Group       domain.HedgeGroup
	Reservation domain.ReservationID
	Target      fixedpoint.Quantity

	// PerpOrder and SpotOrder are the two submission records' ids, empty until
	// the log holds the submission. They are what the reinstated aggregate's
	// OwnLegs must contain — FM-9's netting set — and the perp one is also the
	// key the confirmed perp reduction is read back under.
	PerpOrder ids.ClientOrderID
	SpotOrder ids.ClientOrderID

	// Authorized is FLAG-3's fixed spot size. It is the zero VALUE of
	// fixedpoint.Quantity — untagged and failing its first operation — until an
	// authorization record is seen, which is deliberate and is not the same as a
	// zero quantity: an authorized size of zero is LEGAL (a perp reduction that
	// confirmed nothing), so a valid zero and no authorization at all must not be
	// the same value. Steps is what tells them apart.
	Authorized fixedpoint.Quantity

	// Steps is every phase the log recorded AFTER `Planned`, in record order.
	//
	// The whole sequence rather than the final phase, because replaying it is
	// what re-checks SM §7's edge set on every recorded transition — the
	// generation-scope reading of §7.5's "a log recording an impossible history
	// is refused rather than installed". A final phase alone would be reinstated
	// by whichever path the rebuilder chose, and the log's own claim about which
	// transitions happened would go unchecked.
	Steps []domain.GenerationPhase
}

// LadderBar is one hedge group whose ladder a supersession closed, with the row
// that closed it. The path is carried because it is what an operator is shown:
// "this package will not delever again" is a different conversation depending on
// whether the kill switch or the fence arbitration said so.
type LadderBar struct {
	Group domain.HedgeGroup
	Path  domain.SupersessionPath
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
	rb := &rebuild{
		orders: r.orders,
		known:  make(map[domain.GenerationID]*GenerationRecovery),
		ended:  make(map[domain.GenerationID]bool),
		barred: make(map[domain.HedgeGroup]domain.SupersessionPath),

		marginKnown:   make(map[domain.MarginOpID]domain.MarginOp),
		marginVerdict: make(map[domain.MarginOpID]domain.MarginOpStatus),
		marginCleared: make(map[domain.MarginOpID]domain.MarginOpReconciliation),
	}
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

	// opened is every generation the journal brought into existence, in record
	// order, and ended is the subset the log finished.
	//
	// TWO structures rather than one map to a liveness bool, because they answer
	// different questions and the walk can only answer one of them monotonically.
	// `opened` is an ordered set that never shrinks — it is what makes a record
	// about an unknown generation reportable — while `ended` only ever gains
	// members, so nothing can resurrect a finished generation by arriving late.
	// A single map would make both of those properties a discipline this method
	// set keeps rather than a shape it has.
	//
	// `known` maps each opened id to the record being accumulated for it. It
	// carries a POINTER because the later records amend the same value — a map of
	// values would silently accumulate into copies, which is the shape where the
	// authorization is recorded and the reinstatement is handed a generation that
	// never authorized anything.
	opened []domain.GenerationID
	known  map[domain.GenerationID]*GenerationRecovery
	ended  map[domain.GenerationID]bool

	// barred is the group→path fold behind Recovery.BarredLadders, written only
	// where SupersessionPath.ResumesLadder answers false.
	//
	// FIRST writer wins, which is domain.Generation.Supersede's own rule one
	// scope out: the path an operator is shown must be the arbitration that
	// closed the ladder, not whichever one arrived last against a package that
	// was already closed to it.
	barred map[domain.HedgeGroup]domain.SupersessionPath

	// marginOps is every margin operation the journal dispatched, in record
	// order, with `marginKnown` holding the record and `marginVerdict` the status
	// that ended it.
	//
	// The same two-structure shape as the generations above, and the ordered
	// slice earns its place for the same reason: it is what makes a resolution
	// record naming an op this walk has never seen reportable rather than
	// silently creating one. A resolution for an unknown op is not a stale
	// duplicate — it is a verdict about a write nothing recorded, so the record
	// that would say what was attempted is missing from an append-only log.
	//
	// marginVerdict is a map to a STATUS rather than to a bool, because the two
	// verdicts differ in what they oblige: an op resolved Ambiguous still
	// constrains this process, and one resolved Applied or NotApplied is finished
	// business. A liveness bool would collapse exactly that distinction.
	//
	// marginCleared holds the operator acts that ended an ambiguity's incident
	// (core §7.1's "explicit accounting / operator action", task #213). It is a
	// THIRD structure rather than a fourth MarginOpStatus for record §82 §1's
	// reason: the verdict answers what the evidence concluded and this answers
	// what a person did, and one map holding both would make the clearance
	// reachable by the fold that reads the verdict.
	//
	// It maps to the RECORD rather than to a bool, on marginVerdict's rule at a
	// weaker level: no consumer yet distinguishes two clearances, but the act's
	// actor and instant are the only things the log holds about who ended a §9
	// incident, and a bool would discard them at the one seam an operator report
	// reads from.
	marginOps     []domain.MarginOpID
	marginKnown   map[domain.MarginOpID]domain.MarginOp
	marginVerdict map[domain.MarginOpID]domain.MarginOpStatus
	marginCleared map[domain.MarginOpID]domain.MarginOpReconciliation

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

// GenerationOpened applies SM §7's `Planned` record.
//
// # The payload is KEPT now, and #203's reason for dropping it has expired
//
// The record carries a hedge group, a reservation and a per-leg target, and slice
// #203 dropped all three because nothing could use them: the aggregate needed a
// home and the reservation needed the Classifier this type must not hold. The
// second constraint has not moved and is not worked around — what changed is that
// Reinstater exists to consume these values one phase later, so the walk carries
// them as DATA and performs nothing.
//
// A SECOND opening of the same id is refused, on ApplyPlanned's rule one machine
// over: domain.Ledger.Register refuses a duplicate ClientOrderID at boot for the
// same reason it refuses one at planning, and a log claiming one generation was
// opened twice is a log that verifies and does not compose.
func (b *rebuild) GenerationOpened(
	seq uint64,
	id domain.GenerationID,
	group domain.HedgeGroup,
	reservation domain.ReservationID,
	target fixedpoint.Quantity,
) error {
	b.seen(seq)
	if _, ok := b.known[id]; ok {
		return fmt.Errorf("journal record %d: replay opening of generation %q: %w",
			seq, id, ErrGenerationAlreadyOpened)
	}
	b.known[id] = &GenerationRecovery{
		ID:          id,
		Group:       group,
		Reservation: reservation,
		Target:      target,
	}
	b.opened = append(b.opened, id)
	return nil
}

// GenerationPerpSubmitted applies the perp leg's submission record.
func (b *rebuild) GenerationPerpSubmitted(seq uint64, id domain.GenerationID, order ids.ClientOrderID) error {
	b.seen(seq)
	rec, err := b.knownGeneration(seq, id, fmt.Sprintf("perp leg %q", order))
	if err != nil {
		return err
	}
	rec.PerpOrder = order
	rec.Steps = append(rec.Steps, domain.GenerationPerpReduceInFlight)
	return nil
}

// GenerationAuthorized applies FLAG-3's record.
//
// The size is kept, and what one chain bought is still what makes it trustworthy:
// this record's sequence is lower than its spot leg's, in the same walk, verified
// by the same chain, so the reinstatement reads an authorization it knows
// preceded the order it authorized rather than assuming it.
func (b *rebuild) GenerationAuthorized(seq uint64, id domain.GenerationID, size fixedpoint.Quantity) error {
	b.seen(seq)
	rec, err := b.knownGeneration(seq, id, "authorization")
	if err != nil {
		return err
	}
	rec.Authorized = size
	rec.Steps = append(rec.Steps, domain.GenerationSpotReduceAuthorized)
	return nil
}

// GenerationSpotSubmitted applies the reserved spot leg's submission record.
func (b *rebuild) GenerationSpotSubmitted(seq uint64, id domain.GenerationID, order ids.ClientOrderID) error {
	b.seen(seq)
	rec, err := b.knownGeneration(seq, id, fmt.Sprintf("spot leg %q", order))
	if err != nil {
		return err
	}
	rec.SpotOrder = order
	rec.Steps = append(rec.Steps, domain.GenerationSpotReduceInFlight)
	return nil
}

// GenerationSuperseded ENDS a generation, and is where the LADDER's fate is
// decided as well as the generation's.
//
// The path is validated for the reason it always was — an undeclared path is a
// value §7.5 cannot replay — and it is now ACTED on, which is the half slice
// #203 named as a gap and left open. `SupersessionPath.ResumesLadder` is SM §7's
// own invariant ("a supersession ends the GENERATION; whether it ends the LADDER
// is a property of the PATH"), the two external rows answer false, and a bar
// lost across a restart is this machine re-authorising itself against an
// arbitration that has just taken the package away from it.
//
// The bar is recorded against the group from the generation's OWN opening
// record, which is why it can only be derived here and not from the supersession
// record alone: SM §7's gate is per hedge group, and the supersession names the
// generation.
func (b *rebuild) GenerationSuperseded(seq uint64, id domain.GenerationID, path domain.SupersessionPath) error {
	b.seen(seq)
	rec, err := b.knownGeneration(seq, id, fmt.Sprintf("supersession by %s", path))
	if err != nil {
		return err
	}
	if err := path.Validate(); err != nil {
		return fmt.Errorf("journal record %d: replay supersession of generation %q: %w", seq, id, err)
	}
	resumes, err := path.ResumesLadder()
	if err != nil {
		return fmt.Errorf("journal record %d: replay supersession of generation %q: %w", seq, id, err)
	}
	if !resumes {
		if _, already := b.barred[rec.Group]; !already {
			b.barred[rec.Group] = path
		}
	}
	b.ended[id] = true
	return nil
}

// Phase applies one of the five payload-free phase records, and is where a
// generation is found to have ENDED without being superseded.
//
// # Liveness is decided by the DOMAIN's predicate, and that is the whole design
//
// The alternative was a table in this file mapping each record kind to the phase
// it records — which the port states method by method, so a copy here would be a
// second home for it, drifting the moment a kind's phase changed. There is none.
// A generation is live unless the log records an ENDING, an ending is a phase
// where domain.GenerationPhase.IsActive is false, and the three mid-ladder kinds
// above do not participate in the question at all.
//
// That leaves exactly one claim to check rather than six: every non-active phase
// must be reachable through this method or through GenerationSuperseded. It is,
// by construction — AppendGenerationPhase owns `Complete` and `TerminalFlatten`
// and refuses the rest, and `Superseded` has its own kind — and
// TestEveryEndedGenerationPhaseIsReachableThroughAnEndingRecord is that claim as
// a test rather than as this paragraph.
func (b *rebuild) Phase(seq uint64, id domain.GenerationID, phase domain.GenerationPhase) error {
	b.seen(seq)
	rec, err := b.knownGeneration(seq, id, fmt.Sprintf("phase %s", phase))
	if err != nil {
		return err
	}
	active, err := phase.IsActive()
	if err != nil {
		return fmt.Errorf("journal record %d: replay phase of generation %q: %w", seq, id, err)
	}
	rec.Steps = append(rec.Steps, phase)
	if !active {
		b.ended[id] = true
	}
	return nil
}

// MarginOpDispatched applies core §7.1's `MarginOp` record: a margin write this
// system committed to before issuing it.
//
// A SECOND dispatch of the same id is refused, on GenerationOpened's rule. It is
// worth separating from §7.1's OTHER duplicate rule, which this is not: that one
// forbids two ops IN FLIGHT on one POSITION, and it is the dispatcher's to
// enforce (part 2) because it is a claim about the live world. This is a claim
// about the log — one id, one dispatch — and a log that says otherwise cannot be
// resolved at all, because the two records would name two different before-states
// for one comparison.
func (b *rebuild) MarginOpDispatched(seq uint64, op domain.MarginOp) error {
	b.seen(seq)
	if _, ok := b.marginKnown[op.ID]; ok {
		return fmt.Errorf("journal record %d: replay dispatch of margin op %q: %w",
			seq, op.ID, ErrMarginOpAlreadyDispatched)
	}
	b.marginKnown[op.ID] = op
	b.marginOps = append(b.marginOps, op.ID)
	return nil
}

// MarginOpResolved applies the verdict that ended a margin op.
//
// # A repeat of the SAME verdict is a duplicate; a DIFFERENT one is refused
//
// The asymmetry is deliberate, and it is where this record kind parts company
// with §5.3's duplicate-delivery tolerance. A restated verdict is harmless — it
// says what the log already says — and counting it keeps this walk's duplicate
// figure honest. Two DIFFERENT verdicts for one op is a log that verifies and
// does not compose, and it is the exact shape the stored verdict exists to make
// impossible: whichever one a replay picked would be a re-derivation choosing
// between answers, which is the decision §7.1 says must not be re-made.
//
// The direction it would fail in has no safe side, which is why it is refused
// rather than resolved by a rule. Taking the LAST would let a later `applied`
// lift an `ambiguous` §9 incident; taking the FIRST would let an `ambiguous`
// stand over an op that a later, better-evidenced pass concluded had applied.
func (b *rebuild) MarginOpResolved(seq uint64, id domain.MarginOpID, status domain.MarginOpStatus) error {
	b.seen(seq)
	if err := status.Validate(); err != nil {
		return fmt.Errorf("journal record %d: replay resolution of margin op %q: %w", seq, id, err)
	}
	if _, ok := b.marginKnown[id]; !ok {
		// knownGeneration's argument at margin-op scope, and the permissive
		// failure here is sharper: an unknown resolution silently dropped leaves
		// a verdict — possibly an `ambiguous` blocking openings — attached to
		// nothing, so the process would boot clean over a §9 incident the log
		// records.
		return fmt.Errorf("journal record %d: replay resolution of margin op %q: %w",
			seq, id, ErrMarginOpNotDispatched)
	}
	if prior, ok := b.marginVerdict[id]; ok {
		if prior != status {
			return fmt.Errorf("journal record %d: margin op %q resolved %s and then %s: %w",
				seq, id, prior, status, ErrMarginOpVerdictConflict)
		}
		b.duplicates++
		return nil
	}
	b.marginVerdict[id] = status
	return nil
}

// MarginOpReconciled applies the operator act that ended an Ambiguous op's
// incident — core §7.1's "the staged-float ledger is reconciled by explicit
// accounting / operator action" (task #213, record §82).
//
// # Three refusals, and each is a history that could not have happened
//
// The op must be known, for MarginOpResolved's reason. It must be RESOLVED and
// resolved AMBIGUOUS, because a clearance is defined by what it clears: an op
// still in flight has no incident yet, so a clearance ahead of it is a
// pre-authorisation of whatever the evidence turns out to say, and an op
// resolved Applied or NotApplied has no block for the act to lift.
//
// Both are checked HERE and not only at the write end, and the ordering claim is
// the single chain's: this record sits after the verdict in sequence order, so
// "is it ambiguous" is a question with an answer AT THIS POINT IN THE WALK
// rather than one about the log's final state. A clearance written before the
// resolution it names is refused by the same code path, which is what makes the
// ordering a fact the replay checks rather than a discipline the writer keeps.
//
// # A SECOND clearance is refused, and this is where it parts company with the
// # verdict's duplicate rule
//
// MarginOpResolved tolerates a restated verdict because it says what the log
// already says. Two reconciliations cannot be identical in any useful sense —
// they carry an actor, an instant and a note, so a repeat is a different act by
// construction — and neither reading of which one governs is defensible. Taking
// the first discards the later actor's attribution; taking the last lets a
// second person overwrite the record of who actually looked. §7.3 requires an
// immutable per-actor audit, and "the second one wins" is the opposite of that.
func (b *rebuild) MarginOpReconciled(seq uint64, rec domain.MarginOpReconciliation) error {
	b.seen(seq)
	if err := rec.Validate(); err != nil {
		return fmt.Errorf("journal record %d: replay reconciliation: %w", seq, err)
	}
	if _, ok := b.marginKnown[rec.Op]; !ok {
		return fmt.Errorf("journal record %d: replay reconciliation of margin op %q: %w",
			seq, rec.Op, ErrMarginOpNotDispatched)
	}
	status, resolved := b.marginVerdict[rec.Op]
	if !resolved {
		return fmt.Errorf("journal record %d: reconciliation of margin op %q, which no verdict has ended: %w",
			seq, rec.Op, ErrMarginOpNotAmbiguous)
	}
	if status != domain.MarginOpAmbiguous {
		return fmt.Errorf("journal record %d: reconciliation of margin op %q resolved %s: %w",
			seq, rec.Op, status, ErrMarginOpNotAmbiguous)
	}
	if prior, ok := b.marginCleared[rec.Op]; ok {
		return fmt.Errorf("journal record %d: margin op %q reconciled by %q and then by %q: %w",
			seq, rec.Op, prior.Actor, rec.Actor, ErrMarginOpAlreadyReconciled)
	}
	b.marginCleared[rec.Op] = rec
	return nil
}

// knownGeneration refuses a record about a generation no opening record created.
//
// It is the generation-scope reading of the failure reconcile.go names as "the
// log verifies and does not compose": a state record for an order that was never
// planned. The realistic cause is not corruption but TRUNCATION at the head —
// a journal whose earliest records were lost — and the direction matters. A
// replay that shrugged and carried on would report zero live generations for a
// package that has one, which is the permissive answer, and the entry the
// missing opening record names would never be re-Reserved by anything.
func (b *rebuild) knownGeneration(seq uint64, id domain.GenerationID, what string) (*GenerationRecovery, error) {
	rec, ok := b.known[id]
	if !ok {
		return nil, fmt.Errorf("journal record %d: replay %s of generation %q: %w",
			seq, what, id, ErrGenerationNotOpened)
	}
	return rec, nil
}

// seen records the walk's own arithmetic, in one place so the visitor methods
// cannot count differently.
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
		Generations:  len(b.opened),
		MarginOps:    len(b.marginOps),
	}

	// Both margin lists are derived from the two structures rather than
	// accumulated during the walk, on LiveGenerations' rule: an op that was
	// dispatched and then resolved must contribute to neither, and a walk that
	// appended as it saw records would have to remember to remove it.
	//
	// The Ambiguous arm is deliberately NOT written as "everything not in
	// InFlight that is not Applied": it names the one status that constrains, so
	// a status added later lands in neither list and shows up as a gap rather
	// than being swept into whichever arm a negation happened to put it in.
	//
	// The clearance (task #213) subtracts from the Ambiguous arm and from nothing
	// else, and the direction of its default is what matters: an empty
	// marginCleared leaves every ambiguity listed, so the failure of the whole
	// clearance mechanism — a dropped record, a fold that never runs — is a §9
	// incident that keeps blocking rather than one that quietly lifts. It is
	// asked of the AMBIGUOUS arm alone because that is the only arm a clearance
	// can be about: MarginOpReconciled refuses one for any other status, so a
	// cleared-and-in-flight op is not a state this walk can be handed.
	for _, id := range b.marginOps {
		status, resolved := b.marginVerdict[id]
		_, cleared := b.marginCleared[id]
		switch {
		case !resolved:
			rec.InFlightMarginOps = append(rec.InFlightMarginOps, b.marginKnown[id])
		case status != domain.MarginOpAmbiguous:
			if rec.SettledMarginOps == nil {
				rec.SettledMarginOps = make(map[domain.MarginOpID]domain.MarginOpStatus)
			}
			rec.SettledMarginOps[id] = status
		case cleared:
			rec.ReconciledMarginOps = append(rec.ReconciledMarginOps, b.marginCleared[id])
		default:
			rec.AmbiguousMarginOps = append(rec.AmbiguousMarginOps, id)
		}
	}

	// Derived from the two sets rather than accumulated during the walk, on the
	// unresolved set's rule and for the same reason: a generation that opened
	// and then completed is not live, and a walk that appended ids as it saw
	// them would have to remember to remove it. Expressing the property as the
	// way the value is computed is what makes the intermediate records unable to
	// contribute.
	for _, id := range b.opened {
		if !b.ended[id] {
			rec.LiveGenerations = append(rec.LiveGenerations, *b.known[id])
		}
	}
	// The bars are read off the map rather than accumulated in order, so they are
	// sorted below for Unresolved's reason: a boot report that names its causes in
	// map order is one an operator learns to distrust.
	for group, path := range b.barred {
		rec.BarredLadders = append(rec.BarredLadders, LadderBar{Group: group, Path: path})
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
	sort.Slice(rec.LiveGenerations, func(i, j int) bool { return rec.LiveGenerations[i].ID < rec.LiveGenerations[j].ID })
	sort.Slice(rec.BarredLadders, func(i, j int) bool {
		return rec.BarredLadders[i].Group.String() < rec.BarredLadders[j].Group.String()
	})
	sort.Slice(rec.InFlightMarginOps, func(i, j int) bool {
		return rec.InFlightMarginOps[i].ID < rec.InFlightMarginOps[j].ID
	})
	sort.Slice(rec.AmbiguousMarginOps, func(i, j int) bool {
		return rec.AmbiguousMarginOps[i] < rec.AmbiguousMarginOps[j]
	})
	// By op id and NOT by the reconciliation's instant, which is the field a
	// reader would reach for. The instant is the operator's own, so ordering by
	// it makes the report's row order depend on a clock this system does not
	// control — two acts an operator dated by hand can order either way, and a
	// list whose order a typo changes is one nobody can diff between boots.
	sort.Slice(rec.ReconciledMarginOps, func(i, j int) bool {
		return rec.ReconciledMarginOps[i].Op < rec.ReconciledMarginOps[j].Op
	})
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
