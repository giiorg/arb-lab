package application

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Dispatcher is build-spec §6 step 6's crash boundary: the one place in this
// system where an order becomes a request at a venue.
//
// # The whole file exists for one sentence of §5.3
//
// "Place is NON-idempotent: submit once, NO transport retry; never infer 'not
// sent'." Every decision below follows from it. The journal is fsynced before
// the send so a process that dies mid-call leaves evidence; the send happens
// exactly once whatever it returns; and the failure is classified by the effect
// the adapter recorded rather than by reading the cause, because reading the
// cause IS the inference the spec forbids.
//
// # Not safe for concurrent use
//
// Deliberately unsynchronised, on Ledger's precedent and for its reason: §5.1
// gives the aggregate an OCC Version and §6 a journal-then-apply write path, so
// serialisation is a property of how a write reaches this type, decided one
// layer out. A mutex here would make two concurrent dispatchers safe from the
// race detector while leaving them exactly as wrong about that version — and
// worse here than in the ledger, since the thing racing is a venue submission.
type Dispatcher struct {
	journal ports.Journal
	placer  ports.OrderPlacer
	orders  ports.OrderWriteRepository

	// clk is read for exactly one thing: §5.3's authorization anchor, in the
	// venue's own frame. Nothing else in this use case is time-dependent, and
	// the narrowness is worth keeping — a second reader of this field would be
	// a decision about time taken on the dispatch path, which is the path with
	// the least room for one.
	clk clock.Clock

	// authorized is the in-process double-dispatch backstop. See Dispatch, where
	// the reason it is a backstop rather than the mechanism is stated.
	authorized map[ids.ClientOrderID]struct{}
}

// NewDispatcher constructs the use case, refusing a missing port.
//
// Refused at construction rather than at the first call, because the call that
// would discover a nil journal is the one that submits an order: the crash
// boundary would be found not to exist at the moment it was needed, for an order
// already on its way to a venue.
//
// The clock is refused on the same terms, and it earns them: without it the
// authorization has no anchor, so every order this dispatcher sends is one
// recovery could never resolve to StateAbsentAfterFinality. That is a defect
// with no symptom until the day an ack is lost, which is the profile this
// constructor's refusals exist for.
func NewDispatcher(j ports.Journal, p ports.OrderPlacer, orders ports.OrderWriteRepository, clk clock.Clock) (*Dispatcher, error) {
	if j == nil {
		return nil, ErrJournalMissing
	}
	if p == nil {
		return nil, ErrPlacerMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	return &Dispatcher{
		journal:    j,
		placer:     p,
		orders:     orders,
		clk:        clk,
		authorized: make(map[ids.ClientOrderID]struct{}),
	}, nil
}

// Outcome is where an order ended up, for a dispatch that reached the venue.
//
// It exists as a separate type from Dispatch's error for one reason, and the
// reason is the safety property of this whole package — see Dispatch.
type Outcome struct {
	// State is where the order actually is, and it is always safe to act on.
	// Every value it can take is a successor of StateDispatchAuthorized under
	// domain.CanTransition, which is asserted by test rather than at runtime.
	State domain.OrderState

	// Ack is what the venue returned, zero on any failed Place. The dispatcher
	// does not read it; see ports.Ack for why parsing it is downstream of the
	// transition rather than part of it.
	Ack ports.Ack

	// Cause is why State is not StateSubmitted — the Place failure, joined with
	// ErrEffectUnhandled if the classification came from the fallback. It is nil
	// on a clean place.
	//
	// The ErrEffectUnhandled arm of that join is UNOBSERVABLE from outside this
	// package, and the limit is worth stating rather than leaving for someone to
	// rediscover as a surviving mutant: reaching it needs an undeclared
	// apperror.Effect, and apperror.New degrades any invalid Spec to
	// EffectUnknown, so no error a ports.OrderPlacer can return will produce
	// one. A mutation deleting that arm therefore survives the suite. What
	// covers it is not a test but the exhaustiveswitch gate, which refuses to
	// build a classifyEffect that has stopped naming a member — the only way the
	// arm could come to matter. See §25.
	Cause error

	// Journaled reports whether State reached the journal.
	//
	// A field rather than a returned error, because a failure here is NOT a
	// safety failure: the journal still holds StateDispatchAuthorized, which
	// recovery resolves by query — exactly the right behaviour for an order
	// whose fate we could not record. It is an alerting matter, since a journal
	// that is failing threatens the NEXT order, whose authorization will not be
	// durable either.
	Journaled bool

	// Applied reports whether State reached the write repository — §6's apply
	// half, the current-state projection the running system reads.
	//
	// # It is NOT the same kind of "false" as Journaled, and the difference is a
	// # safety property rather than a severity
	//
	// A false Journaled loses a RECORD, and recovery is unharmed: the journal
	// still holds the authorization, which is the record §5.3 resolves an
	// in-flight order from. A false Applied loses a READING, and the reading is
	// one §6.7 gates a compensating trade on.
	//
	// The specific hazard, because it is not obvious from "the projection is
	// stale": a failed apply leaves this order at StatePlanned in the
	// repository, and StatePlanned is the ONE non-terminal state that
	// contributes no possible delta (domain.ContributesPossibleDelta says so
	// explicitly). So an order that was actually SENT reads to the classifier as
	// contributing nothing — which turns an ambiguous residual into a determined
	// one, and §6.7's determined branch compensates immediately rather than
	// quiescing to finality first. The projection does not merely lag; it lags
	// in the direction that unblocks the action the ambiguity existed to hold.
	//
	// [SPEC EDIT LANDED — v0.9.30] Nothing in this slice acts on that, and the
	// consequence belongs to §6.7's owner — so it is in §6 step 7 rather than
	// only here: a false Applied on a dispatch that reached the venue HOLDS NEW
	// TRADING until that order's state is re-derived from the journal. It is
	// recoverable rather than lost, because the journal is written first and is
	// what §7.5 replays.
	//
	// It is the FM-5 shape at the projection layer — a quantity read from a
	// store that a different store is the authority for — and it inherits that
	// family's consequence unchanged: no invariant computed FROM the projection
	// can detect it, so the report has to be this field, at the moment of the
	// write, from the one caller that knows the send happened.
	Applied bool
}

// Dispatch submits one planned order to its venue, exactly once.
//
// # It takes an id and a version, NOT an order, and that is the slice's central
// # decision
//
// The order this method sends is READ from the write repository, so the bytes
// that reach the venue are the bytes §6 step 5 journaled. A signature taking a
// domain.Order lets a caller submit an order that was never planned — no journal
// record, no ClientOrderID uniqueness, nothing for §7.5's recovery to correlate
// — and lets a caller that DID plan mutate the order between planning and
// dispatch, so that the journal describes one order and the venue receives
// another. Neither is expressible now: an id the repository does not hold is
// ports.ErrOrderUnknown, and there is no parameter through which a different
// order could arrive.
//
// expected is §5.1's OCC version, and it is the caller's assertion about WHICH
// record it decided to dispatch. See admit for why the version is read and
// compared there rather than relied on at the write.
//
// # err != nil means NOTHING WAS SENT
//
// That is the contract, and the signature exists to make it the only readable
// one. When err is non-nil the order is still StatePlanned and §5.3's "safe to
// replan or discard" applies. When err is nil, Place ran exactly once and
// Outcome.State is where the order is — including when Outcome.Cause is set,
// because a failed send whose effect is unknown is still a send.
//
// The rejected signature is (domain.OrderState, error), and rejecting it is the
// point rather than a style preference. Under it the returned state is
// authoritative EVEN WHEN the error is non-nil — a contract that reads fine in
// a doc comment and that the universal Go reflex `if err != nil { return }`
// silently breaks, dropping an order that may be live at a venue. Splitting the
// two makes that mistake impossible to express: when err != nil there is no
// state to ignore, because nothing was sent.
//
// # The order of the steps is the safety argument
//
// Admit, then journal StateDispatchAuthorized, then send, then classify, then
// journal the result. Steps 1 and 2 send nothing, so a caller correcting a
// refusal and retrying is legitimate; step 3 onward is unrepeatable for this
// ClientOrderID, ever, by anyone.
//
// A journal failure at step 2 refuses the send outright, and the asymmetry that
// justifies it is worth stating: the journal is LOCAL, so a failed fsync is a
// failed fsync — the one write in this sequence whose error means what it says.
// The network write that follows it is precisely the one whose error does not.
// That asymmetry is the entire reason the ordering is journal-then-network.
//
// # The double-dispatch guard is a backstop, and the repository did not make it
// # redundant
//
// Durable enforcement of the single-Place rule is the journal: recovery finds an
// order at StateDispatchAuthorized and resolves it by query, never by
// resubmitting. A fresh process therefore starts with an empty set and that is
// correct, not a gap.
//
// The repository now refuses a second dispatch too — the record sits at
// StateDispatchAuthorized, and domain.CanTransition answers ErrTransitionNoOp
// for that edge — which looks like grounds to delete this map. It is not, and
// the reason is the case where BOTH would otherwise be needed at once: if the
// apply failed (see Outcome.Applied) the record is still at StatePlanned, so the
// repository would happily admit the order a second time, for an order that has
// already been sent. The map is the mechanism that holds precisely when the
// projection is the thing that broke.
// # The urgency argument is the caller's, and it is checked BEFORE the fsync
//
// [SPEC EDIT LANDED — v0.9.32] §6 step 6 described this method as taking the
// order alone; it now states the argument and, more to the point, WHERE it is
// refused. ports.OrderPlacer.Place records why the §7.6 shed class has to travel
// with the call and why no default is available; what this method adds is the
// side of the fsync the refusal must land on.
// An invalid class must fail above the AppendState below, because that fsync is
// the commitment to call Place exactly once: refusing after it would leave a
// durable authorization for an order nothing ever sent, which §5.3 then resolves
// by querying a venue that never heard of it.
func (d *Dispatcher) Dispatch(id ids.ClientOrderID, expected int64, urgency ports.Urgency) (Outcome, error) {
	if err := urgency.Validate(); err != nil {
		return Outcome{}, fmt.Errorf("dispatch order %q urgency: %w", id, err)
	}

	o, err := d.admit(id, expected)
	if err != nil {
		return Outcome{}, err
	}

	// §5.3's anchor, read BEFORE the fsync and therefore before the send, which
	// is the direction that makes it safe to be wrong in. Our authorization
	// precedes the venue's creation of the order, so anchor ≤ venue_created and
	// every bound measured from it — the landing horizon, the retention cliff —
	// expires EARLIER than the venue's own. A resolver using it refuses slightly
	// early, never late. See ports.OrderRecord.AuthorizedAt.
	//
	// An uncalibrated venue yields the ZERO ServerTime rather than an error
	// (clock.serverNow: "the alternative — a zero offset and a zero band — would
	// be indistinguishable from a venue calibrated as perfectly synchronised"),
	// so the refusal has to be here, and it has to be ABOVE the fsync.
	//
	// That placement is §6 step 6's rule rather than a preference: "every check
	// on the dispatch path that can fail for a reason the CALLER controls
	// belongs on the same side of that fsync", because a refusal after it leaves
	// a durable authorization for an order nothing ever sent — which §5.3 then
	// resolves by querying a venue that never heard of the id, and which
	// recovery may never resubmit. A missing calibration is exactly such a
	// reason: it is deployment configuration, fixable, and costs nothing to
	// refuse here.
	//
	// ApplyAuthorized refuses the same instant again, and the duplication is the
	// ordinary one this file already carries for the marketable-IOC check: this
	// guard protects the dispatch path's fsync ordering, that one protects the
	// repository against every OTHER caller that will reach it.
	authorizedAt := d.clk.ServerNow(o.Instrument.Venue)
	if err := authorizedAt.Validate(); err != nil {
		return Outcome{}, fmt.Errorf("authorize dispatch of order %q on %s: %w: %w",
			id, o.Instrument.Venue, ports.ErrAuthorizationNeedsAnchor, err)
	}

	// §6 step 6: "immediately before the single Place, fsync
	// DispatchAuthorized". ports.Journal returns only when the record is
	// durable, so there is no Sync to pair with this.
	if err := d.journal.AppendState(id, domain.StateDispatchAuthorized); err != nil {
		return Outcome{}, fmt.Errorf("authorize dispatch of order %q: %w", id, err)
	}

	// Recorded only now. Everything above sent nothing, so a caller that fixes
	// its order and calls again has not double-dispatched anything — and marking
	// it earlier would burn the id on a refusal.
	d.authorized[id] = struct{}{}

	// The apply half of §6's journal-then-apply, in that order and on the far
	// side of the commitment. Its failure does NOT abort the send, and that is
	// forced rather than chosen: the fsync above is the commitment to call Place
	// exactly once, so refusing here would leave a durable authorization for an
	// order nothing ever sent — which §5.3 then resolves by querying a venue
	// that never heard of it. Reported through Outcome.Applied instead, where
	// the consequence is stated.
	authorized, applyErr := d.orders.ApplyAuthorized(id, expected, authorizedAt)

	ack, placeErr := d.placer.Place(o, urgency)

	state, unhandled := classifyPlace(placeErr)

	journalErr := d.journal.AppendState(id, state)

	// Skipped outright when the authorization never applied, rather than
	// attempted against a guessed version. The record is still at StatePlanned,
	// so every target this classification can produce is a forbidden edge from
	// it — the attempt could only fail, and it would fail with a transition
	// error that describes the projection's gap rather than its cause.
	applied := false
	if applyErr == nil {
		_, resultErr := d.orders.ApplyState(id, authorized.Version, state)
		applied = resultErr == nil
	}

	return Outcome{
		State:     state,
		Ack:       ack,
		Cause:     errors.Join(placeErr, unhandled),
		Journaled: journalErr == nil,
		Applied:   applied,
	}, nil
}

// admit establishes that this order may be sent, and returns the order to send.
//
// # Everything here is a READ, and that is what makes the version check useful
//
// The OCC comparison is done here — before the fsync — rather than being left to
// the ApplyState that follows the send. §6 step 6 wants nothing between the
// authorization fsync and the single Place, so a refusal discovered at the write
// arrives after the system has already committed to submitting; it could report
// the conflict but not prevent the order. Read first, and a caller holding a
// stale version is refused with nothing sent.
//
// What that costs, stated rather than left implicit: a write racing the window
// between this read and that write is still caught, but only by the ApplyState —
// which is to say caught after the send. Serialising Dispatch is the caller's,
// on this type's stated non-concurrency; the version is what makes the loss
// visible rather than silent.
//
// # Two checks that were here are gone, and one of them is the interesting one
//
// domain.Order.Validate is no longer called. It was the right check when a
// caller supplied the order; now the order comes from the repository, whose
// ApplyPlanned refuses a malformed one (the port says so), so a Validate here
// would be a branch no test could reach — an unreachable sentinel reading as a
// handled case, which this package has paid for before.
//
// The minted-id check STAYS, and moving the source did not weaken its reason —
// it strengthened it. §6 step 10 routes foreign orders to operator clearance and
// the reconciler holds them in this same repository, so "the order came from our
// store" is precisely NOT evidence that we minted its id.
func (d *Dispatcher) admit(id ids.ClientOrderID, expected int64) (domain.Order, error) {
	// First because it is the only check needing nothing loaded, and because a
	// second dispatch is the one refusal that must not depend on the projection
	// being intact.
	if _, dispatched := d.authorized[id]; dispatched {
		return domain.Order{}, fmt.Errorf("dispatch order %q: %w", id, ErrOrderAlreadyDispatched)
	}

	rec, err := d.orders.Load(id)
	if err != nil {
		// ports.ErrOrderUnknown here is §6 step 5 biting: an order that was never
		// planned has no journaled intent, so there is nothing to dispatch and
		// nothing recovery could correlate if we did.
		return domain.Order{}, fmt.Errorf("dispatch order %q: %w", id, err)
	}
	if rec.Version != expected {
		return domain.Order{}, fmt.Errorf("dispatch order %q: record is at version %d, dispatch expected %d: %w",
			id, rec.Version, expected, ports.ErrVersionConflict)
	}
	if err := domain.CanTransition(rec.State, domain.StateDispatchAuthorized); err != nil {
		return domain.Order{}, fmt.Errorf("dispatch order %q: %w", id, err)
	}

	o := rec.Order

	// domain.Order deliberately accepts ids this system did not mint, because
	// §6 step 10's foreign orders carry venue-minted ones and the reconciler
	// must hold them. The strict check belongs HERE — the one place an id stops
	// being something we read and becomes something we write to a venue.
	if _, err := ids.ParseClientOrderID(string(o.ClientOrderID)); err != nil {
		return domain.Order{}, fmt.Errorf("dispatch order %q: %w: %w", o.ClientOrderID, ErrOrderNotOMSMinted, err)
	}

	// §6 step 6: "parallel marketable-IOC limit orders (never raw MARKET)".
	//
	// A comparison against the two permitted values, NOT an exhaustive switch
	// over the two enums, and the direction is deliberate: a member added to
	// OrderType or TimeInForce next year is refused by this line without anyone
	// editing it. The gate-checked switch is the loud shape everywhere else in
	// this repo and the wrong one here, because it would have to be re-answered
	// — and answered wrongly once — for a value nobody intended to send.
	//
	// Planner mints only this pair, so the reachable case is a foreign order the
	// reconciler stored, not a planning bug.
	if o.Type != domain.OrderTypeLimit || o.TimeInForce != domain.TimeInForceIOC {
		return domain.Order{}, fmt.Errorf("dispatch order %q is %s/%s: %w",
			o.ClientOrderID, o.Type, o.TimeInForce, ErrNotMarketableIOCLimit)
	}

	return o, nil
}

// classifyPlace decides which lifecycle state a Place outcome puts the order in.
// It is the safety-critical decision of this use case.
//
// The returned state is ALWAYS safe to act on. The returned error means the
// classification came from the fallback rather than from a rule, and must be
// reported — it is advisory about this program, not about the order.
//
// # The tag is the effect, never the cause
//
// §5.3 forbids inferring "not sent" from what went wrong, so the adapter — the
// only layer that knows whether the request left the socket — records the answer
// and this function reads it. The switch is tagged on apperror.Effect so §3.6
// rule 10 forces every declared member to be named here: an effect added to that
// enum breaks this file at compile time instead of landing wherever the last
// case happens to be.
//
// # EffectNotApplied is not, by itself, a rejection
//
// The one row that took the argument. When the effect is NotApplied we know no
// order exists — the connection was refused before a byte was written, a local
// governor declined, our own Validate refused — so a terminal looks justified
// and the resolving query looks wasted. It is refused on evidence-class grounds:
// Effect answers "did the side effect happen", NOT "which evidence class proves
// it didn't", and SM §1's two absence terminals are distinguished by exactly
// that (FLAG-2: proved absence and explicit venue rejection are different
// evidence classes). EffectNotApplied explicitly covers both "never left the
// socket" and "the venue rejected it on its merits", so the value alone cannot
// choose between StateRejected and StateAbsentAfterFinality — and the latter is
// unavailable anyway, since it requires the stable-not-found rule to have
// COMPLETED. Inventing either would write an evidence class into the journal
// that no evidence supports. One resolving query converts an honest ambiguity
// into the right terminal, and it costs one REST call on a path that has already
// failed.
func classifyPlace(err error) (domain.OrderState, error) {
	if err == nil {
		// §5.3's own definition of StateSubmitted: "the single Place returned a
		// transport result". Tested before the switch because EffectOf(nil) is
		// EffectUnspecified, which classifyEffect sends to SubmissionUnknown.
		return domain.StateSubmitted, nil
	}
	return classifyEffect(apperror.EffectOf(err), apperror.ClassOf(err))
}

// classifyEffect is classifyPlace's decision, separated from the extraction of
// the two axes it decides on.
//
// # The split is not test convenience; it is what makes half the switch
// # observable at all
//
// Two of the four Effect members cannot reach this decision through apperror's
// PUBLIC API. apperror.New validates its Spec and degrades anything invalid to
// ClassInternal/EffectUnknown, so no constructed *apperror.Error can carry
// EffectUnspecified or an undeclared cast — and EffectOf answers
// EffectUnspecified only for a nil error, which classifyPlace has already
// handled. A test that could only inject ERRORS would therefore be unable to
// tell EffectUnspecified's membership in the group below from its absence, and
// unable to reach the fallthrough at all: exactly the
// trailing-return-hides-its-own-group trap this repo diagnosed in
// domain.IsTerminal, reappearing one layer up in the argument type rather than
// in the switch.
//
// Taking the two axes as values moves the seam to where every case is
// reachable. classifyPlace keeps the part that is genuinely about errors — nil,
// and §5.3's rule that the effect is read rather than the cause.
//
// Decision record §24e specified classifyPlace(error) alone; this seam is
// beneath it and changes no contract, so it is NOT a spec deviation and carries
// no marker. It is recorded in §25 because the design's own stated purpose for
// ErrEffectUnhandled — that the group be observable by a test — is
// unachievable without it.
func classifyEffect(effect apperror.Effect, class apperror.Class) (domain.OrderState, error) {
	switch effect {
	case apperror.EffectApplied:
		// The send definitely happened and the call failed anyway; what could
		// not be read is the ACK, which is exactly what StateSubmitted means.
		// Not SubmissionUnknown — that state is ambiguous about whether we
		// submitted, and here we are not.
		return domain.StateSubmitted, nil

	case apperror.EffectNotApplied:
		if class == apperror.ClassVenueReject {
			// FLAG-2 requires an explicit venue rejection for this terminal,
			// and this is precisely that evidence class.
			return domain.StateRejected, nil
		}
		return domain.StateSubmissionUnknown, nil

	case apperror.EffectUnknown, apperror.EffectUnspecified:
		// EffectUnspecified reaching here is apperror's deliberate
		// degrade-to-safe: a classification nobody filled in reads as "this may
		// have executed" rather than as a silent "proved not sent".
		return domain.StateSubmissionUnknown, nil
	}

	// Reachable only from an Effect with no case above, which is §3.6 rule 10's
	// failure arriving at runtime because something removed the gate.
	//
	// It returns a state AND an error, and needs both. State-only would make
	// this branch return the identical (SubmissionUnknown, nil) as the
	// EffectUnknown case, so deleting a member from that case would change
	// nothing any test could observe — the trailing-return-hides-its-own-group
	// trap this repo diagnosed in domain.IsTerminal, one layer up. Error-only
	// would leave a caller whose Place has ALREADY RUN with no state to record
	// for a possibly-live order.
	return domain.StateSubmissionUnknown, fmt.Errorf("%s: %w", effect, ErrEffectUnhandled)
}
