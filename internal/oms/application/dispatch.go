package application

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
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
func NewDispatcher(j ports.Journal, p ports.OrderPlacer) (*Dispatcher, error) {
	if j == nil {
		return nil, ErrJournalMissing
	}
	if p == nil {
		return nil, ErrPlacerMissing
	}
	return &Dispatcher{
		journal:    j,
		placer:     p,
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
}

// Dispatch submits one order to its venue, exactly once.
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
// # The double-dispatch guard is a backstop, not the mechanism
//
// Durable enforcement of the single-Place rule is the journal: recovery finds an
// order at StateDispatchAuthorized and resolves it by query, never by
// resubmitting. A fresh process therefore starts with an empty set and that is
// correct, not a gap. The guard earns its place because it defends against the
// misuse this method's own signature invites — a caller reading `err == nil,
// Cause != nil` as "that failed, try again".
func (d *Dispatcher) Dispatch(o domain.Order) (Outcome, error) {
	if err := d.admit(o); err != nil {
		return Outcome{}, err
	}

	// §6 step 6: "immediately before the single Place, fsync
	// DispatchAuthorized". ports.Journal returns only when the record is
	// durable, so there is no Sync to pair with this.
	if err := d.journal.AppendState(o.ClientOrderID, domain.StateDispatchAuthorized); err != nil {
		return Outcome{}, fmt.Errorf("authorize dispatch of order %q: %w", o.ClientOrderID, err)
	}

	// Recorded only now. Everything above sent nothing, so a caller that fixes
	// its order and calls again has not double-dispatched anything — and marking
	// it earlier would burn the id on a refusal.
	d.authorized[o.ClientOrderID] = struct{}{}

	ack, placeErr := d.placer.Place(o)

	state, unhandled := classifyPlace(placeErr)

	journalErr := d.journal.AppendState(o.ClientOrderID, state)

	return Outcome{
		State:     state,
		Ack:       ack,
		Cause:     errors.Join(placeErr, unhandled),
		Journaled: journalErr == nil,
	}, nil
}

// admit runs the three checks that must pass before anything is journaled or
// sent, in the order a wrong answer is cheapest to catch in.
func (d *Dispatcher) admit(o domain.Order) error {
	if err := o.Validate(); err != nil {
		return fmt.Errorf("dispatch: %w", err)
	}

	// domain.Order deliberately accepts ids this system did not mint, because
	// §6 step 10's foreign orders carry venue-minted ones and the reconciler
	// must hold them. The strict check belongs HERE — the one place an id stops
	// being something we read and becomes something we write to a venue.
	if _, err := ids.ParseClientOrderID(string(o.ClientOrderID)); err != nil {
		return fmt.Errorf("dispatch order %q: %w: %w", o.ClientOrderID, ErrOrderNotOMSMinted, err)
	}

	// §6 step 6: "parallel marketable-IOC limit orders (never raw MARKET)".
	//
	// A comparison against the two permitted values, NOT an exhaustive switch
	// over the two enums, and the direction is deliberate: a member added to
	// OrderType or TimeInForce next year is refused by this line without anyone
	// editing it. The gate-checked switch is the loud shape everywhere else in
	// this repo and the wrong one here, because it would have to be re-answered
	// — and answered wrongly once — for a value nobody intended to send.
	if o.Type != domain.OrderTypeLimit || o.TimeInForce != domain.TimeInForceIOC {
		return fmt.Errorf("dispatch order %q is %s/%s: %w",
			o.ClientOrderID, o.Type, o.TimeInForce, ErrNotMarketableIOCLimit)
	}

	if _, dispatched := d.authorized[o.ClientOrderID]; dispatched {
		return fmt.Errorf("dispatch order %q: %w", o.ClientOrderID, ErrOrderAlreadyDispatched)
	}

	return nil
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
