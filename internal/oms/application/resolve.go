package application

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Resolver is §5.3's venue-specific stable-not-found rule, made executable.
//
// # It CANNOT resubmit, and that is structural rather than disciplined
//
// §5.3: an order at DispatchAuthorized or beyond "is resolved ONLY by
// OrderByClientID/Fills — never resubmitted, and 'not sent' is never inferred".
// This type holds ports.OrderResolver and does not hold ports.OrderPlacer, so
// there is no method here through which a resubmission could be written. The
// rule stops being something a reviewer has to check for and becomes something
// the type system does not offer.
//
// # The rule is FOUR GATES, and StateAbsentAfterFinality requires all of them
//
// §5.3 names the rule twice and never states its steps; SM §2 supplies the
// four-step finality contract and §5.3's retention paragraph supplies the bound.
// Composed:
//
//  1. LANDING HORIZON — no absence counts until a still-in-flight send can no
//     longer land. SM §2 step 1, and §6.7's own words: "a point-in-time
//     not-found cannot collapse an in-flight order — a DispatchAuthorized send
//     may land AFTER the query."
//  2. ABSENT FROM EVERY SURFACE — Bybit's realtime surface holds ~500 closed
//     orders and clears on a server restart, so §5.3 makes the history fallback
//     normative. The walk is engine/venue's; what arrives here is its answer.
//  3. A CONCLUSIVE SWEEP PAST THE WATERMARK — SM §2 step 3, "never a synthetic
//     stable-not-found alone". Coverage must be PROVEN through an instant
//     strictly later than gate 1's horizon, and must contain no execution this
//     system cannot rule out for this order.
//  4. WITHIN THE ANSWERING SURFACE'S REACH — §5.3 (v0.9.33): past the venue's
//     published cliff the query cannot tell "existed and expired unfilled" from
//     "never existed", so a not-found there is not proof of anything.
//
// Every gate that fails produces an UnprovedReason and no state change. That
// asymmetry is the design: this type can refuse to conclude, and it can never
// conclude wrongly, because each conclusion needs a positive answer from a
// venue rather than the absence of a negative one.
//
// # Not safe for concurrent use
//
// Dispatcher's argument, unchanged: §5.1 gives the aggregate an OCC Version, so
// serialisation is decided one layer out and the version is what makes a lost
// update visible rather than silent.
type Resolver struct {
	journal ports.Journal
	orders  ports.OrderWriteRepository
	venue   ports.OrderResolver
	clk     clock.Clock
	policy  ResolutionPolicy
}

// ResolutionPolicy is the local configuration the four gates need and no venue
// publishes.
//
// Every field here is an assumption with an owner, which is why they are
// configuration rather than constants: a constant in this file would be an
// assumption with no owner, indistinguishable at read time from a venue fact.
type ResolutionPolicy struct {
	// MaxLanding is gate 1's propagation bound: how long after we commit to a
	// Place the venue may still accept it.
	//
	// It exists because SM §2 step 1 requires a still-in-flight send to be
	// "waited past its max propagation/timeout so it cannot land after the
	// query". The value belongs with the transport that bounds it — the signed
	// client's timeout, and recvWindow beneath it, which is what bounds how long
	// a delayed submission may still execute at the venue after we stopped
	// waiting (Profiles §6: the acceptance window is asymmetric and only one
	// side is configurable). Set it ABOVE both, never below.
	MaxLanding time.Duration

	// SweepMargin is how far past the landing horizon the executions sweep must
	// prove coverage.
	//
	// It is a separate field from MaxLanding rather than being folded into it,
	// even though the code only ever adds them, because they answer different
	// normative sentences: MaxLanding is SM §2 step 1's propagation bound and
	// this is SM §2 step 3's "STRICTLY later than". Folded together, a later
	// reader shortening the sum could not tell which requirement they were
	// violating. Kept apart, "strictly later" is enforced by a validated
	// positive duration rather than by an inequality somebody has to notice.
	SweepMargin time.Duration

	// AssumedReach supplies gate 4's bound for a surface whose venue publishes
	// none — see the SurfaceKey doc for why this cannot be defaulted in either
	// direction.
	AssumedReach map[SurfaceKey]time.Duration
}

// SurfaceKey identifies a venue's order surface for the purpose of gate 4.
//
// # Both defaults are unsafe, so the carrier is forced
//
// Binance SPOT's Query-order page states no retention at all, so its reach
// arrives Published: false. Neither reading of that is available:
//
//   - "no cliff" licenses treating a spot absence as PROVED absence for ever,
//     which writes an evidence class we do not have into a terminal state that
//     §7.5 then consumes as truth after arbitrary downtime.
//   - "never provable" means a lost-ack spot IOC that did not fill can never
//     reach StateAbsentAfterFinality. The spot leg of a cash-and-carry is a BUY,
//     so this is not a corner — it is half of every trade — and its hedge group
//     stays ambiguous for ever with new trading held.
//
// The second is the one that looks careful, which is why it is written down: it
// fails by halting the system quietly rather than by trading wrongly, and this
// repo has recorded that shape enough times to name it.
//
// So the assumption is REQUIRED to be stated by whoever deploys, and it is a
// local assumption rather than a venue fact — which is why it lives here and not
// in venue-profiles.md, whose §8 records the one assumed venue fact it does hold
// precisely so that it can be told apart from the verified ones.
//
// A surface that is unpublished AND unconfigured produces
// ReasonReachUnknown — a refusal to prove, never a proof.
type SurfaceKey struct {
	Venue   ids.Venue
	Product ids.ProductCategory
}

// Validate reports whether the policy is one the four gates can run on.
//
// # What it CANNOT check, stated because the gap is the interesting part
//
// It cannot check that AssumedReach COVERS the instruments this resolver will be
// asked about, because no instrument is known until a call arrives. The natural
// instinct — refuse at construction, above the irreversible commitment — does
// not transfer here: the irreversible commitment is the Place, and that happened
// in a different call path entirely. So a missing assumption is caught at gate 4
// as ReasonReachUnknown, which is safe (no proof, new trading held) and late.
//
// The check that WOULD be in time is a boot posture condition — refuse to trade
// an instrument whose absences could never be proved — and it belongs with
// §7.5's boot list rather than here. Named per §3.6's deferral rule: the
// construct is that list, not this type.
func (p ResolutionPolicy) Validate() error {
	if p.MaxLanding <= 0 {
		return fmt.Errorf("max landing %v: %w", p.MaxLanding, ErrResolutionPolicyUnset)
	}
	if p.SweepMargin <= 0 {
		return fmt.Errorf("sweep margin %v: %w", p.SweepMargin, ErrResolutionPolicyUnset)
	}
	for key, reach := range p.AssumedReach {
		if reach <= 0 {
			return fmt.Errorf("assumed reach for %s %s is %v: %w",
				key.Venue, key.Product, reach, ErrResolutionPolicyUnset)
		}
	}
	return nil
}

// NewResolver constructs the use case, refusing a missing port or an
// unusable policy.
func NewResolver(j ports.Journal, orders ports.OrderWriteRepository, v ports.OrderResolver, clk clock.Clock, policy ResolutionPolicy) (*Resolver, error) {
	if j == nil {
		return nil, ErrJournalMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if v == nil {
		return nil, ErrResolverMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("oms: resolution policy: %w", err)
	}
	return &Resolver{journal: j, orders: orders, venue: v, clk: clk, policy: policy}, nil
}

// ResolutionOutcome is whether the rule reached a conclusion.
type ResolutionOutcome string

const (
	// OutcomeResolved means the order's state was established and written.
	// Resolution.State says which, and StateAbsentAfterFinality is one of the
	// values it can take — that is the proved-absence case, and it is a
	// conclusion like any other rather than a separate outcome.
	OutcomeResolved ResolutionOutcome = "resolved"

	// OutcomeUnproved means no state could be established. Resolution.Reason
	// says which gate refused, nothing was journaled or applied, and the caller
	// holds new trading (§5.3: "if the resolving query is unavailable or
	// rate-limited, new trading stays disabled and rate-limit budget is reserved
	// for recovery").
	OutcomeUnproved ResolutionOutcome = "unproved"
)

// Validate reports whether o is one of the declared outcomes.
func (o ResolutionOutcome) Validate() error {
	switch o {
	case OutcomeResolved, OutcomeUnproved:
		return nil
	}
	return fmt.Errorf("%q: %w", string(o), ErrResolutionOutcomeInvalid)
}

func (o ResolutionOutcome) String() string { return string(o) }

// UnprovedReason names the gate that refused.
//
// It is an enum rather than a wrapped error because a caller BRANCHES on it:
// §6.7's deadline-exceeded branch cares whether the contract could not complete
// (a reason that will resolve itself given time) or whether it can never
// complete for this order (one that will not), and the two demand different
// operator action. A string in an error would make that a substring match.
type UnprovedReason string

const (
	// ReasonNoAnchor is gate 1 and 4 both: the record carries no authorization
	// instant, so neither time bound can be evaluated.
	//
	// Two different causes produce it and both want this refusal — an order not
	// yet at the crash boundary, and an order whose anchor died with the process
	// because the projection is in memory. The second is the one that matters:
	// it is the crash-recovered order §5.3 says is resolved by this very rule,
	// and until §7.5's replayer re-derives the instant from the journal it can
	// never reach StateAbsentAfterFinality. See ports.OrderRecord.AuthorizedAt.
	ReasonNoAnchor UnprovedReason = "no_anchor"

	// ReasonHorizonNotPassed is gate 1: a send authorized before the query may
	// still land after it. This one resolves itself — the horizon is a fixed
	// instant and the clock reaches it.
	ReasonHorizonNotPassed UnprovedReason = "horizon_not_passed"

	// ReasonReachExceeded is gate 4: the answering surface may already have
	// purged this order, so its absence is explicable by retention and proves
	// nothing. It never resolves itself, and it gets strictly worse with time.
	ReasonReachExceeded UnprovedReason = "reach_exceeded"

	// ReasonReachUnknown is gate 4 with no bound to apply: the venue publishes
	// no retention for this surface and no assumption was configured for it.
	// Also permanent, and fixed by configuration rather than by waiting.
	ReasonReachUnknown UnprovedReason = "reach_unknown"

	// ReasonSweepIncomplete is gate 3: the sweep did not prove coverage through
	// the horizon, so its silence is not evidence. Retryable.
	ReasonSweepIncomplete UnprovedReason = "sweep_incomplete"

	// ReasonSweepInconclusive is gate 3 with an execution the sweep can neither
	// attribute to this order nor rule out — Binance's trade lists carry no
	// client order id, so a row is attributable only by a venue order id we do
	// not have for an order the venue says it does not hold.
	ReasonSweepInconclusive UnprovedReason = "sweep_inconclusive"

	// ReasonExecutionsFound is the sweep REFUTING the absence: an execution
	// attributable to this order exists while the order surface reported none.
	//
	// It is unproved rather than resolved because the state this implies —
	// PartiallyFilled or Filled — is a function of cumulative executed quantity
	// against the ordered quantity, and admitting those executions is the fill
	// path's single transaction (§5.3), which no port here declares. The
	// executions are carried on the Resolution so the caller has them.
	ReasonExecutionsFound UnprovedReason = "executions_found"
)

// Validate reports whether r is one of the declared reasons.
func (r UnprovedReason) Validate() error {
	switch r {
	case ReasonNoAnchor, ReasonHorizonNotPassed, ReasonReachExceeded, ReasonReachUnknown,
		ReasonSweepIncomplete, ReasonSweepInconclusive, ReasonExecutionsFound:
		return nil
	}
	return fmt.Errorf("%q: %w", string(r), ErrUnprovedReasonInvalid)
}

func (r UnprovedReason) String() string { return string(r) }

// Resolution is what one run of the rule established.
//
// It is a value rather than an error for the reason Dispatch's Outcome is: the
// interesting answers are not failures. A venue that says "no such order" and a
// gate that refuses to conclude are both the rule WORKING, and returning either
// as an error would put them one `if err != nil { return }` away from being
// discarded.
type Resolution struct {
	// Outcome is whether a state was established.
	Outcome ResolutionOutcome

	// State is where the order is, meaningful only when Outcome is
	// OutcomeResolved.
	State domain.OrderState

	// Reason names the gate that refused, meaningful only when Outcome is
	// OutcomeUnproved.
	Reason UnprovedReason

	// Executions are the fills the sweep attributed to this order. Non-empty
	// only with ReasonExecutionsFound, and they are the reason that case is
	// reported rather than absorbed: they are exposure the ledger has not
	// admitted yet.
	Executions []ports.VenueExecution

	// Reach is the answering surface's horizon, carried whether or not it
	// decided anything, so a refusal names what it was measured against.
	Reach ports.SurfaceReach

	// Journaled and Applied report where a resolved state reached, on Dispatch's
	// Outcome semantics: a false Journaled loses a record and recovery is
	// unharmed, a false Applied loses a reading §6.7 gates a compensating trade
	// on. Both are false when Outcome is OutcomeUnproved, because nothing was
	// written.
	Journaled bool
	Applied   bool
}

// Resolve runs the rule for one order.
//
// # It takes no expected version, unlike Dispatch, and the asymmetry is real
//
// Dispatch takes one because its caller decided WHICH record to send and the
// version is that decision's assertion. Nothing analogous exists here: the
// caller is asking what the venue says about an order, and it has no claim to
// make about the record's state — the point of the call is that it does not
// know. So this method reads the record and writes against the version it read,
// and a concurrent write is reported as ports.ErrVersionConflict rather than
// silently overwritten.
//
// # err != nil means NOTHING WAS WRITTEN
//
// Dispatch's contract, for its reason. An error here is a query that was not
// answered, a record that could not be read, or a state the domain refuses —
// none of which changed anything. A Resolution is returned in exactly the cases
// where the rule ran.
func (r *Resolver) Resolve(id ids.ClientOrderID) (Resolution, error) {
	rec, err := r.orders.Load(id)
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: %w", id, err)
	}

	// The resolvable states are DERIVED from SM §1 rather than listed here: an
	// order is resolvable by this rule exactly when proved absence is one of the
	// answers the domain would accept for it. That is true of
	// DispatchAuthorized and SubmissionUnknown and of nothing else, which is
	// §5.3's set — and deriving it means the two cannot drift.
	//
	// StatePlanned failing this check is the case worth naming: §5.3 says such
	// an order "was never dispatched (no venue order can exist)", so querying a
	// venue about it would run the finality contract to its deadline and alert
	// an operator about our own unsent order.
	if err := domain.CanTransition(rec.State, domain.StateAbsentAfterFinality); err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q at %s: %w: %w",
			id, rec.State, ErrOrderNotResolvable, err)
	}

	// Gate 1, first half: without an anchor neither time bound exists, and no
	// venue call could supply one. Refused before spending a request.
	if err := rec.AuthorizedAt.Validate(); err != nil {
		return unproved(ReasonNoAnchor, ports.SurfaceReach{}), nil
	}

	venueNow := r.clk.ServerNow(rec.Order.Instrument.Venue)

	// Gate 1, second half. The boundary is built from the anchor's LATEST edge
	// because the order could have been created that late, and the sweep margin
	// is added here rather than at the sweep so that one comparison establishes
	// both "the send can no longer land" and "the horizon the sweep must reach
	// is in the past". A sweep cannot prove coverage through a future instant,
	// and asking it to would be a request that fails for a reason that reads
	// like a venue problem.
	horizon := rec.AuthorizedAt.Latest().Add(r.policy.MaxLanding + r.policy.SweepMargin)
	if venueNow.Passed(horizon) != clock.CertainlyPassed {
		return unproved(ReasonHorizonNotPassed, ports.SurfaceReach{}), nil
	}

	// Gate 2 is engine/venue's surface walk; what arrives is its answer. An
	// error is a query that was not answered, and §5.3 forbids reading one as an
	// absence — so it leaves by a different return, and this is the line that
	// makes that true.
	status, err := r.venue.Resolve(rec.Order)
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: %w", id, err)
	}
	if err := status.Presence.Validate(); err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: %w", id, err)
	}
	if err := status.Reach.Validate(); err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: %w", id, err)
	}

	if status.Presence == ports.VenueHasOrder {
		return r.present(rec, status, horizon)
	}
	return r.absent(rec, status, venueNow, horizon)
}

// present handles a venue that HAS the order.
//
// # A terminal state still needs the sweep, and the uniformity is deliberate
//
// SM §2 calls the four-step contract "the ONE canonical boundary … never a
// weaker cancel-ack-only variant", and SM §1 defines StateCanceled as reached by
// "an explicit venue-terminal cancel PLUS the final executions sweep". So a
// terminal reading from step 2 is not on its own enough to collapse the order's
// possible-delta interval; step 3 is what closes it.
//
// It applies to every terminal rather than to the ones where an execution is
// plausible. A per-state exemption — Rejected cannot have fills, so skip it —
// would be a second rule about evidence, held in one function, that a reader of
// SM §2 would have no reason to expect.
//
// A NON-terminal reading needs no sweep, and that is not an exemption: the
// order is live, its interval has not collapsed, and there is no finality to
// prove. The state is recorded so that the next classification sees the venue's
// answer rather than our stale one.
func (r *Resolver) present(rec ports.OrderRecord, status ports.VenueOrder, horizon time.Time) (Resolution, error) {
	terminal, err := status.State.IsTerminal()
	if err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: venue state: %w", rec.Order.ClientOrderID, err)
	}
	if !terminal {
		return r.record(rec, status.State, status.Reach, nil)
	}

	sweep, err := r.sweep(rec, horizon)
	if err != nil {
		return Resolution{}, err
	}
	if sweep.ProvenThrough.Before(horizon) {
		return unproved(ReasonSweepIncomplete, status.Reach), nil
	}
	return r.record(rec, status.State, status.Reach, nil)
}

// absent handles a venue that answered "no such order" on every surface it has.
//
// The gates run cheapest-first, and cheapest here means "spends no venue
// request": gate 4 is arithmetic over a duration the previous response already
// carried, so a reach that cannot license a proof refuses BEFORE the sweep is
// paid for. §5.3 requires rate-limit budget to be reserved for recovery, and
// spending it on a sweep whose answer could not be used is the way to fail that
// requirement while looking thorough.
func (r *Resolver) absent(rec ports.OrderRecord, status ports.VenueOrder, venueNow clock.ServerTime, horizon time.Time) (Resolution, error) {
	reach, ok := r.reachOf(rec.Order.Instrument, status.Reach)
	if !ok {
		return unproved(ReasonReachUnknown, status.Reach), nil
	}

	// The cliff is measured from the anchor's EARLIEST edge — the order could
	// have been created that early, so this is the soonest the row could have
	// been purged. The opposite edge is used for gate 1's horizon, and the two
	// are opposite for one reason: each gate takes the band edge that makes it
	// refuse sooner.
	//
	// CertainlyNotPassed is required rather than "not CertainlyPassed": an
	// Uncertain answer means the band straddles the cliff, so both readings are
	// consistent with what we know, and treating that as "still in reach" would
	// prove absence from a coin flip.
	cliff := rec.AuthorizedAt.Earliest().Add(reach)
	if venueNow.Passed(cliff) != clock.CertainlyNotPassed {
		return unproved(ReasonReachExceeded, status.Reach), nil
	}

	sweep, err := r.sweep(rec, horizon)
	if err != nil {
		return Resolution{}, err
	}
	if sweep.ProvenThrough.Before(horizon) {
		return unproved(ReasonSweepIncomplete, status.Reach), nil
	}

	ours, verdict := attribute(sweep.Executions, rec.Order.ClientOrderID)
	switch verdict {
	case attributionClean:
		// All four gates. This is the ONLY path in this file to
		// StateAbsentAfterFinality, and §5.3's separateness of that terminal
		// from StateRejected is what the four gates buy: a proved absence, not
		// an unanswered question and not a venue's refusal.
		return r.record(rec, domain.StateAbsentAfterFinality, status.Reach, nil)
	case attributionOurs:
		return unprovedWith(ReasonExecutionsFound, status.Reach, ours), nil
	case attributionUnknown:
		return unproved(ReasonSweepInconclusive, status.Reach), nil
	}
	return Resolution{}, fmt.Errorf("resolve order %q: %q: %w",
		rec.Order.ClientOrderID, string(verdict), ErrAttributionUnhandled)
}

// reachOf answers gate 4's duration, and answers false when there is none.
//
// # It takes the MINIMUM of the two retentions, and the obvious field is wrong
//
// The tempting read is UnfilledRetention, since §6 step 6 sends marketable IOCs
// and their commonest outcome is an unfilled one. It is backwards. Gate 4 asks
// whether an ABSENCE is explicable by purging, and at that moment we do not know
// which kind of order it was — that is what the absence means. So the absence
// must be inexplicable under EVERY hypothesis, which is the retention that
// expires FIRST.
//
// min() rather than reading UnfilledRetention even though both venues publish
// the shorter one for unfilled orders today: that ordering is a venue fact, not
// a type invariant, and taking a field by its name rather than by its role is
// how a venue that publishes them the other way round silently widens the bound.
func (r *Resolver) reachOf(instrument ids.VenueInstrument, reach ports.SurfaceReach) (time.Duration, bool) {
	if reach.Published {
		return min(reach.UnfilledRetention, reach.FilledRetention), true
	}
	assumed, ok := r.policy.AssumedReach[SurfaceKey{Venue: instrument.Venue, Product: instrument.Product}]
	return assumed, ok
}

// sweep runs SM §2 step 3 for one order.
//
// The window starts at the anchor's EARLIEST edge, which cannot be after the
// venue created the order, so no execution of ours can precede it. It ends at
// the same horizon gate 1 established, which is strictly later than the max
// landing instant by SweepMargin's construction.
func (r *Resolver) sweep(rec ports.OrderRecord, horizon time.Time) (ports.ExecutionSweep, error) {
	sweep, err := r.venue.SweepExecutions(rec.Order, rec.AuthorizedAt.Earliest(), horizon)
	if err != nil {
		return ports.ExecutionSweep{}, fmt.Errorf("sweep executions for order %q: %w", rec.Order.ClientOrderID, err)
	}
	return sweep, nil
}

// attributionVerdict is what a sweep's rows say about one order.
type attributionVerdict string

const (
	// attributionClean means every row was positively ruled out.
	attributionClean attributionVerdict = "clean"

	// attributionOurs means at least one row belongs to this order.
	attributionOurs attributionVerdict = "ours"

	// attributionUnknown means at least one row could be neither attributed nor
	// ruled out, and no row was ours.
	attributionUnknown attributionVerdict = "unknown"
)

// attribute decides whether a sweep contains an execution belonging to one
// order.
//
// # The ONLY key it can use is the client order id, and that is a venue fact
// # with a sharper edge than it first appears
//
// The natural second key is the venue's own order id, and it is not available
// here. Binance's trade lists carry no client order id at all — neither
// /fapi/v1/userTrades nor /api/v3/myTrades returns one — so on Binance the venue
// id is the only join key; and this function is reached from ONE place, the
// absent path, where a venue that reports no such order has supplied no id for
// it either. A parameter for it would be an argument every caller passes empty,
// which is this repo's unreachable-sentinel shape: two arms nothing can reach,
// reading as handled cases.
//
// So the consequence is stated plainly rather than papered over with a
// parameter: ON BINANCE, ANY execution row inside the window makes the sweep
// inconclusive. Not a rare corner — another of this system's own orders filling
// on the same instrument within the window does it, and the window is tens of
// seconds. What would close it is knowing the venue ids of our OTHER orders, so
// that their rows could be ruled out; that means persisting Ack.VenueOrderID and
// reconciling against the set, which is §7.5's reconciler and not this rule.
// C2b recorded that Fills and OrderByClientID are a COMPOSITION rather than
// independent; this is the same fact with its operational cost priced.
//
// # Ours WINS over unknown, and the ordering is the safety property
//
// An execution we can attribute is exposure, and it must not be masked by an
// unrelated row we merely could not classify. So every row is examined before
// the verdict is taken, rather than returning on the first ambiguous one.
func attribute(rows []ports.VenueExecution, id ids.ClientOrderID) ([]ports.VenueExecution, attributionVerdict) {
	var ours []ports.VenueExecution
	unknown := false

	for _, e := range rows {
		switch {
		case e.ClientOrderID == id:
			// Reached only when the venue named an order AND it is this one:
			// an empty id cannot match, because domain.Order.Validate refuses an
			// order with no ClientOrderID and Resolve loads its order from the
			// repository that enforced it.
			ours = append(ours, e)
		case e.ClientOrderID != "":
			// The venue named a DIFFERENT order. Ruled out — and this is the arm
			// that makes Bybit's sweep conclusive where Binance's is not.
		default:
			unknown = true
		}
	}

	switch {
	case len(ours) > 0:
		return ours, attributionOurs
	case unknown:
		return nil, attributionUnknown
	}
	return nil, attributionClean
}

// record is journal-then-apply for a resolved state.
//
// # Legality is established BEFORE the journal, and that ordering is forced
//
// ports.Journal is append-only and is "a recorder [that] refuses to hold a
// second copy of the edge set", so it accepts whatever it is given; the write
// repository checks domain.CanTransition and would refuse. Journaling first and
// discovering the refusal second would leave the durable log holding a
// transition the projection rejected — and §7.5 replays the log. So the caller
// establishes legality, which is exactly what Journal's doc requires of it.
func (r *Resolver) record(rec ports.OrderRecord, to domain.OrderState, reach ports.SurfaceReach, executions []ports.VenueExecution) (Resolution, error) {
	id := rec.Order.ClientOrderID
	if err := domain.CanTransition(rec.State, to); err != nil {
		return Resolution{}, fmt.Errorf("resolve order %q: venue reports %s: %w", id, to, err)
	}

	journalErr := r.journal.AppendState(id, to)

	// Attempted even when the journal failed, unlike Dispatch's second apply,
	// and the asymmetry follows from what each failure means. There, a failed
	// authorization apply left the record at a state from which every target was
	// a forbidden edge, so the attempt could only fail. Here the record is
	// unchanged and the transition is still legal, so the apply can succeed —
	// and it is the projection §6.7 reads, so declining to try would hold the
	// classifier on a stale state to punish a journal we already reported.
	_, applyErr := r.orders.ApplyState(id, rec.Version, to)

	return Resolution{
		Outcome:    OutcomeResolved,
		State:      to,
		Executions: executions,
		Reach:      reach,
		Journaled:  journalErr == nil,
		Applied:    applyErr == nil,
	}, nil
}

// unproved and unprovedWith build the refusal, which is a value and not an
// error. Split so that the common case cannot accidentally carry executions and
// the one case that must carry them cannot forget.
func unproved(reason UnprovedReason, reach ports.SurfaceReach) Resolution {
	return Resolution{Outcome: OutcomeUnproved, Reason: reason, Reach: reach}
}

func unprovedWith(reason UnprovedReason, reach ports.SurfaceReach, executions []ports.VenueExecution) Resolution {
	res := unproved(reason, reach)
	res.Executions = executions
	return res
}
