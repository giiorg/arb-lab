package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is core §7.1's `MarginOp`: the durable record a margin top-up is
// journaled as before it is dispatched, and the rule by which a crashed or
// timed-out one is resolved afterwards.
//
// # Why a margin op needs a record at all, when an order does not
//
// An order carries its own identity to the venue — a client order id both sides
// agree on — so §5.3 can ask "what happened to THIS order" and get an answer
// keyed by the thing it asked about. A margin op has no such key on either
// venue: Bybit's add-margin answers with a position snapshot and no operation
// id, and Binance's `positionMargin/history` rows carry no id at all (Profiles
// §4). So the question "did my top-up apply" cannot be asked directly, and what
// replaces it is a BEFORE-STATE this side wrote down and a live reading taken
// afterwards. The record is the only half of that comparison the venue does not
// hold, which is why it is fsynced before the call rather than after it.
//
// # This slice is the RESOLUTION half
//
// Nothing here dispatches. The dispatcher — the read of the before-state, the
// journal write, the `AddIsolatedMargin` call and §7.1's one-op-in-flight-per-
// position serialization — is part 2 (record §81 §0), and it lands after this
// because it is a non-idempotent external write whose ambiguous outcome must
// have somewhere to be resolved before it may be issued at all.

// MarginOpID identifies one margin operation locally. Like GenerationID and
// ReservationID it is an opaque string minted by the caller: the venues supply
// no identity to borrow, which is the whole reason this record exists.
type MarginOpID string

// MarginOp is §7.1's journaled margin operation — "local op ID, intended delta,
// before-state snapshot + venue watermark".
//
// Every field is fixed at dispatch and none is ever amended. A margin op is not
// a state machine with phases: it is one external write that either landed or
// did not, and what evolves is the EVIDENCE about it, which lives at the venue.
type MarginOp struct {
	// ID is the local handle. It is this system's only name for the operation.
	ID MarginOpID

	// Instrument is the isolated perpetual position the margin was posted
	// against — what a resolution re-reads.
	Instrument ids.VenueInstrument

	// Delta is the intended change to the position's posted isolated margin,
	// SIGNED: positive adds, negative reduces.
	//
	// # One signed amount rather than a kind beside a magnitude
	//
	// Bybit's API is already this shape (`position/add-margin` takes a `margin`
	// that adds when positive and reduces when negative), and Binance's `type`
	// 1/2 is a rendering of the same sign. A `MarginOpKind` enum would therefore
	// be a second home for a fact the amount carries — and, with only the adding
	// direction having a caller, a one-member enum whose every fixture makes
	// "this op's direction" and "the only direction" the same answer.
	//
	// A ZERO delta is refused by Validate. It is the one amount under which
	// Applied and NotApplied are indistinguishable by any evidence either venue
	// publishes, so an op that moves nothing is not an op.
	Delta fixedpoint.Money

	// BeforePosted is the position's posted isolated margin as read immediately
	// before the call — Binance `isolatedWallet`, Bybit `positionIM`.
	//
	// It is the POSTED figure and not a margin balance, for the reason task
	// #208 established from the other side: both venues' posted figures exclude
	// unrealized PnL, so they do not move with the mark. That is what makes this
	// evidence at all — a figure that moved with price would be confounded by
	// exactly the rally that provoked the top-up.
	//
	// What still moves it is a FILL on the position, and that confound is not
	// designed away: see Resolve.
	BeforePosted fixedpoint.Money

	// Watermark is the VENUE's clock at the instant BeforePosted was read.
	//
	// Venue time, never local time, and the distinction is load-bearing rather
	// than pedantic. This instant anchors the evidence window, and the rows that
	// window will be matched against when Binance's `positionMargin/history`
	// half lands (record §81 §3b) are stamped by the venue. A window opened on
	// the local clock and closed against venue-stamped rows is wrong by the
	// calibration offset — a small, stable number that reads as correct.
	Watermark time.Time
}

// Validate reports whether the record is one a resolution can be performed
// against.
//
// The asset agreement between the delta and the before-state is checked here
// rather than left to the first subtraction, because the two are read from
// different places at dispatch — the delta from a top-up decision, the
// before-state from a venue read — and a mismatch surfaces otherwise as a
// runtime tag failure inside Resolve, one layer away from the record that
// caused it.
func (o MarginOp) Validate() error {
	if o.ID == "" {
		return ErrMarginOpIDUnset
	}
	if err := o.Instrument.Validate(); err != nil {
		return fmt.Errorf("margin op %q instrument: %w", o.ID, err)
	}
	if !o.Instrument.Product.IsPerp() {
		// Isolated margin is a property of a derivatives position; neither venue
		// has a surface that would post margin against a spot holding. The same
		// refusal ports.MarginReading makes, for the same reason: a rule enforced
		// on one of two mirrors leaves the other holding the shape it exists to
		// eliminate.
		return fmt.Errorf("margin op %q on %s: %w", o.ID, o.Instrument.Symbol, ErrMarginOpNotPerp)
	}
	if err := o.Delta.Validate(); err != nil {
		return fmt.Errorf("margin op %q delta: %w", o.ID, err)
	}
	sign, err := o.Delta.Sign()
	if err != nil {
		return fmt.Errorf("margin op %q delta: %w", o.ID, err)
	}
	if sign == 0 {
		return fmt.Errorf("margin op %q: %w", o.ID, ErrMarginOpDeltaZero)
	}
	if err := o.BeforePosted.Validate(); err != nil {
		return fmt.Errorf("margin op %q before-state: %w", o.ID, err)
	}
	beforeSign, err := o.BeforePosted.Sign()
	if err != nil {
		return fmt.Errorf("margin op %q before-state: %w", o.ID, err)
	}
	if beforeSign < 0 {
		// Neither venue publishes a negative posted margin, so a negative one is
		// a parse that read a sign off the wrong field. It fails permissively
		// downstream: it makes every observed movement look larger than it was.
		return fmt.Errorf("margin op %q before-state is %s: %w", o.ID, o.BeforePosted, ErrMarginOpBeforeNegative)
	}
	if o.Delta.Asset() != o.BeforePosted.Asset() {
		return fmt.Errorf("margin op %q: delta in %s, before-state in %s: %w",
			o.ID, o.Delta.Asset(), o.BeforePosted.Asset(), ErrMarginOpAssetsDisagree)
	}
	if o.Watermark.IsZero() {
		// A record with no watermark cannot bound its own evidence window, so
		// every reading of it would look arbitrarily old — which is the reading
		// that concludes rather than waits.
		return fmt.Errorf("margin op %q: %w", o.ID, ErrMarginOpWatermarkUnset)
	}
	return nil
}

// MarginOpStatus is §7.1's four-member resolution outcome.
//
// A string rather than an ordinal because it is JOURNALED: the log is the thing
// a later process and a human both read, and a stored ordinal is a number whose
// meaning lives somewhere else and shifts when a member is inserted.
type MarginOpStatus string

const (
	// MarginOpUnresolved is the zero value and names no outcome. Validate
	// refuses it: the alternative is a resolution that claims a verdict by
	// omission.
	MarginOpUnresolved MarginOpStatus = ""

	// MarginOpApplied is evidence that the operation landed.
	MarginOpApplied MarginOpStatus = "applied"

	// MarginOpNotApplied is evidence that it did not, positively concluded
	// rather than assumed from silence.
	//
	// It is NOT an instruction to retry. §7.1 prohibits blind delta retries
	// outright, and the ladder above is robust to this outcome by re-reading
	// LIVE liq distance — which is the trigger's actual input and is indifferent
	// to whether a top-up was attempted.
	MarginOpNotApplied MarginOpStatus = "not_applied"

	// MarginOpPending is a refusal to conclude, because too little has happened
	// for the venue's silence to be evidence.
	//
	// It is the only member that is not a verdict, and it has TWO producers
	// which reach it from different observations (task #215):
	//
	//	Resolve            — the evidence is silent and the propagation bound
	//	                     has not elapsed
	//	the DISPATCH path  — the call itself returned an effect that establishes
	//	                     nothing, so there is no evidence yet to be silent
	//
	// The second was added rather than given its own member because the two say
	// the same thing to every consumer: no verdict, nothing to journal, and the
	// op is still in flight. A distinct "unheard" member would be a value whose
	// only difference from this one is which layer produced it, which is a fact
	// the journal already carries in the record's position.
	MarginOpPending MarginOpStatus = "pending"

	// MarginOpAmbiguous is evidence that something happened which this system
	// cannot attribute: §7.1's "coincident manual adjustment, position mutation,
	// or duplicate same-amount history row inside the window".
	//
	// It is a FIRST-CLASS outcome and not a failure — "evidence is not causal
	// identity" — and its consequence is stated by §7.1 and §7.5 together: new
	// openings are blocked, safety actions continue from live liq distance, and
	// the staged-float ledger is reconciled by explicit accounting or operator
	// action, which is a §9 case. Nothing a re-read can lift.
	MarginOpAmbiguous MarginOpStatus = "ambiguous"
)

// Validate refuses the zero value and any undeclared status.
func (s MarginOpStatus) Validate() error {
	switch s {
	case MarginOpApplied, MarginOpNotApplied, MarginOpPending, MarginOpAmbiguous:
		return nil
	case MarginOpUnresolved:
		return ErrMarginOpStatusUnset
	}
	return fmt.Errorf("%q: %w", string(s), ErrMarginOpStatusInvalid)
}

// String implements fmt.Stringer.
func (s MarginOpStatus) String() string { return string(s) }

// Concluded reports whether this status is a VERDICT — an answer that gets
// written to the log and never re-derived.
//
// Three of the four are. MarginOpPending is not, and the asymmetry is the point:
// journaling a pending would put a value in the log that every later reader must
// know to ignore, which is the shape a stale record takes when somebody forgets.
//
// Why the verdicts are stored rather than recomputed at the next boot is the
// argument for this method existing at all, and it has two halves that fail in
// opposite directions. An AMBIGUOUS held only in a process's checklist dies with
// the process, so a restart would satisfy "until the staged-float ledger is
// explicitly reconciled" by rebooting. An APPLIED re-derived later inverts:
// Binance's `positionMargin/history` is a windowed query over rows the venue
// retains for a bounded time, so the same op resolves by a matching row today
// and by that row's absence tomorrow — NotApplied, the reading under which
// nothing happened.
func (s MarginOpStatus) Concluded() (bool, error) {
	switch s {
	case MarginOpApplied, MarginOpNotApplied, MarginOpAmbiguous:
		return true, nil
	case MarginOpPending:
		return false, nil
	case MarginOpUnresolved:
		return false, ErrMarginOpStatusUnset
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrMarginOpStatusInvalid)
}

// Resolve concludes what happened to this operation, from the position's LIVE
// posted margin.
//
// # The evidence is a delta against the journaled before-state
//
// live − BeforePosted is compared against Delta. On Bybit this is the entire
// contract §7.1 states — "the `positionIM` delta vs the journaled before-state",
// with the transaction log explicitly not to be relied on — and on Binance it is
// the half of the contract this slice builds, the `positionMargin/history` match
// being deferred with its cost named (record §81 §3b).
//
// # Pending is one branch's refusal, not a phase in front of the verdict
//
// The tempting shape is to ask "has the propagation bound elapsed?" first and
// answer Pending if not. That is wrong in both directions. A movement that
// already equals the delta is APPLIED — waiting adds nothing and delays a good
// answer — and a movement that equals neither is AMBIGUOUS, which more time
// cannot disambiguate either, so reporting it as "not yet" would dress a
// standing incident as a timer. Only "nothing has moved" is an observation that
// the passage of time can legitimately change, so it is the only one the bound
// is consulted for.
//
// The bound is SM §2 step 1's propagation bound, which this repo already holds
// as ResolutionPolicy.MaxLanding: how long after we stop waiting the venue may
// still accept the write. It is a property of the transport, and both surfaces
// go through the same signed client, so a second bound with its own name would
// be one transport with two assumptions about it, free to drift apart.
//
// The comparison is `<`, so an op exactly at the bound CONCLUDES. A bound is a
// statement about how long the venue may still act, and at its far edge it may
// no longer.
//
// # A flat position is Ambiguous, and it outranks every other branch
//
// held false means the venue reports no position on this instrument, so the
// subject the margin was posted against is gone and the before/after difference
// has no meaning: closing a position returns its isolated margin whether or not
// the top-up landed first. It outranks the bound because waiting cannot restore
// a closed position's evidence, and it outranks the arithmetic because there is
// no live posted margin to difference.
//
// The tempting answer — the position is gone, so nothing is owed — is a claim
// about the staged-float ledger wearing an evidence answer, and that ledger is
// precisely what §7.1 says must be reconciled explicitly.
//
// # now must be on the SAME clock as Watermark
//
// Venue time. Nothing here can check it, so it is stated: the caller reads
// clock.ServerNow(instrument.Venue), and passing a local instant makes the
// elapsed interval wrong by the calibration offset in whichever direction the
// host drifts.
//
// # Every error return is a WIRING defect
//
// A malformed record, a live reading in the wrong asset, a non-positive bound.
// Nothing a venue can cause reaches the error channel — the venue's answers are
// all statuses, LiqMonitor's rule one type over, and for the same reason: this
// section attaches a consequence to not knowing, and an error a caller logs and
// moves past has silently lifted it.
func (o MarginOp) Resolve(live fixedpoint.Money, held bool, hist MarginActHistory, now time.Time, bound time.Duration) (MarginOpStatus, error) {
	if err := o.Validate(); err != nil {
		return MarginOpUnresolved, err
	}
	if bound <= 0 {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: bound %v: %w", o.ID, bound, ErrMarginOpBoundNotPositive)
	}
	if now.IsZero() {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, ErrMarginOpResolutionInstantUnset)
	}
	if err := hist.Validate(); err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q act history: %w", o.ID, err)
	}
	if hist.Published() {
		return o.resolveWithActHistory(live, held, hist, now, bound)
	}
	return o.resolveFromDelta(live, held, now, bound)
}

// Adds reports the direction of this op — true for a top-up, false for a
// reduction — from the sign of its delta.
//
// It exists so the one place that renders the sign as a venue's `type` field and
// the one place that matches a history row against it read the SAME derivation.
// A zero delta is refused by Validate, so there is no third answer.
func (o MarginOp) Adds() (bool, error) {
	sign, err := o.Delta.Sign()
	if err != nil {
		return false, fmt.Errorf("margin op %q delta: %w", o.ID, err)
	}
	if sign == 0 {
		return false, fmt.Errorf("margin op %q: %w", o.ID, ErrMarginOpDeltaZero)
	}
	return sign > 0, nil
}

// resolveFromDelta is the verdict on a venue that publishes no act history.
//
// Bybit, and there this is not a fallback but the ENTIRE contract §7.1 states —
// "the `positionIM` delta vs the journaled before-state", with the transaction
// log explicitly not to be relied on. Every rule in it is task #212's, unchanged,
// and the reason it is now a named arm rather than the whole function is that its
// twin below had to be allowed to differ: three of these rules turn out to rest
// on the delta being the ONLY source, which is a premise that stops holding the
// moment a second one exists.
func (o MarginOp) resolveFromDelta(live fixedpoint.Money, held bool, now time.Time, bound time.Duration) (MarginOpStatus, error) {
	if !held {
		return MarginOpAmbiguous, nil
	}

	if err := live.Validate(); err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q live posted margin: %w", o.ID, err)
	}
	if live.Asset() != o.BeforePosted.Asset() {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: live posted margin in %s, before-state in %s: %w",
			o.ID, live.Asset(), o.BeforePosted.Asset(), ErrMarginOpAssetsDisagree)
	}
	moved, err := live.Sub(o.BeforePosted)
	if err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, err)
	}

	// Exact equality, and inexactness is Ambiguous rather than Applied. Both
	// venues apply the amount that was asked for — a margin op is a transfer of
	// a stated quantity, not a computation the venue performs — so a near miss
	// is a second mover inside the window rather than rounding.
	cmp, err := moved.Cmp(o.Delta)
	if err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, err)
	}
	if cmp == 0 {
		return MarginOpApplied, nil
	}

	movedSign, err := moved.Sign()
	if err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, err)
	}
	if movedSign == 0 {
		if now.Sub(o.Watermark) < bound {
			return MarginOpPending, nil
		}
		return MarginOpNotApplied, nil
	}

	// The margin moved, and by something that is not what this op asked for.
	// §7.1's two causes reachable without an act history land here: a coincident
	// manual adjustment and a position mutation (a fill moves the posted figure
	// too, which is the confound this evidence cannot design away). The third —
	// a duplicate same-amount row — is a history observation and lives in the
	// arm below.
	return MarginOpAmbiguous, nil
}

// resolveWithActHistory is the verdict on a venue that records margin ACTS.
//
// Binance, via `positionMargin/history` (task #214). §7.1 words its evidence as
// the history match "plus the position-risk margin delta", and the "plus" is not
// a conjunction — marginevidence.go's file comment is the argument, and the
// division of labour it lands on is:
//
//	a matching row is the ONLY thing here that concludes Applied
//	the delta is the ONLY thing here that concludes NotApplied
//
// # THREE of the delta arm's rules rest on it being the only source
//
// Each was correct when task #212 wrote it and each is re-derived here rather
// than inherited, because a rule's REASON is what carries across a change and
// two of these reasons do not.
//
// **(i) A flat position no longer outranks everything.** #212's argument was
// that "closing a position returns its isolated margin whether or not the top-up
// landed first", so the before/after difference has no meaning. That is a claim
// about the DELTA's evidence, not about the act's: a venue that recorded the
// adjustment recorded it whether or not the position was closed afterwards. So
// flatness blinds this arm only once the history has said nothing, and there it
// still blinds it completely — there is no live posted margin to difference.
//
// **(ii) The bound guards the history's SILENCE, not the margin's stillness.**
// #212 consulted the bound on exactly one observation, on the ground that "only
// 'nothing has moved' is an observation that the passage of time can legitimately
// change". With a second source that premise is simply false: "the act log has no
// row yet" is also an observation time can change, and it can lag independently
// of the margin figure. Concluding Ambiguous on a still-propagating op would make
// the half built to REDUCE over-ambiguation introduce a new source of it, so the
// bound is consulted whenever the history is silent — whatever the margin did.
//
// A row that HAS arrived is never held for the bound. An act the venue has
// recorded is not made more or less true by waiting, and the reason #212 gave for
// not hoisting the bound applies here with full force: it would delay a good
// answer and dress a standing incident as a timer.
//
// **(iii) A movement equal to the delta is NOT Applied here.** This is the rule
// that looks like a regression and is the point of the slice. On this venue the
// act log has been asked whether the adjustment happened and has said no; the
// margin moving by exactly that amount anyway is a second mover whose amount
// coincides, which is more suspicious than a mismatch, not less. §7.1 says it
// directly — "no row and the margin moved ⇒ Ambiguous, not NotApplied" — and the
// verdict this arm gives up in exchange is bought back many times over by the
// case it wins: a partial fill inside the window, which moves the delta to a
// number matching nothing and leaves a clean row in the act log.
func (o MarginOp) resolveWithActHistory(live fixedpoint.Money, held bool, hist MarginActHistory, now time.Time, bound time.Duration) (MarginOpStatus, error) {
	rows, err := hist.matches(o, bound)
	if err != nil {
		return MarginOpUnresolved, err
	}

	if len(rows) > 1 {
		// §7.1's named duplicate case, verbatim: "duplicate same-amount history
		// row inside the window". The rows are indistinguishable — these records
		// carry no id (Profiles §4) — so this system cannot tell its own act from
		// a coincident one, which is what Ambiguous means.
		return MarginOpAmbiguous, nil
	}
	if len(rows) == 1 {
		switch rows[0].Cause {
		case MarginActUserAdjust:
			return MarginOpApplied, nil
		case MarginActUnattributable:
			// The venue recorded an act of this amount, in this direction, inside
			// this op's window, for a reason that is not a manual adjustment or is
			// one this system does not recognise. Reported rather than dropped —
			// see MarginActUnattributable for why the silent reading is the unsafe
			// one here.
			return MarginOpAmbiguous, nil
		case MarginActCauseUnset:
			return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, ErrMarginActCauseUnset)
		}
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %q: %w", o.ID, string(rows[0].Cause), ErrMarginActCauseInvalid)
	}

	// The history is SILENT: the venue answered and recorded no act this op could
	// be. From here the delta speaks, and only to separate NotApplied from
	// Ambiguous.
	if now.Sub(o.Watermark) < bound {
		return MarginOpPending, nil
	}
	if !held {
		return MarginOpAmbiguous, nil
	}
	if err := live.Validate(); err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q live posted margin: %w", o.ID, err)
	}
	if live.Asset() != o.BeforePosted.Asset() {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: live posted margin in %s, before-state in %s: %w",
			o.ID, live.Asset(), o.BeforePosted.Asset(), ErrMarginOpAssetsDisagree)
	}
	moved, err := live.Sub(o.BeforePosted)
	if err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, err)
	}
	movedSign, err := moved.Sign()
	if err != nil {
		return MarginOpUnresolved, fmt.Errorf("margin op %q: %w", o.ID, err)
	}
	if movedSign == 0 {
		return MarginOpNotApplied, nil
	}
	return MarginOpAmbiguous, nil
}

// MarginOpReconciliation is the operator act that ends an Ambiguous op's
// incident — core §7.1's "the staged-float ledger is reconciled by explicit
// accounting / operator action, a §9 case" (task #213, record §82).
//
// # It is the only record in this log whose author is a person
//
// Everything else here is something this system did and wrote down. This is
// something a HUMAN did away from the system and is reporting, which is why its
// fields are §7.3's three — "an immutable per-actor audit of every control
// action (who / what / when, appended to the tamper-evident log, §5.1)" — and
// why §7.3 is also what decides the record belongs in this chain rather than in
// a document beside it.
//
// # There is no FINDING field, and the omission is a ruling rather than a gap
//
// The tempting fifth field is what the accounting concluded: did the top-up
// land? §4.1's Portfolio-Margin attestation is the precedent that argues for one
// — a record states what was OBSERVED, never the verdict — and it is refused
// here on §7.1's own terms.
//
// §7.1's word for this state is Ambiguous because "evidence is not causal
// identity": a coincident manual adjustment, a position mutation, or a duplicate
// same-amount row. A required finding demands from the operator exactly the
// determination the section says the evidence cannot support. And no reachable
// value of one would keep the block on — §7.1 addresses the worst case by name
// ("a doubled top-up is capital-allocation drift, not naked exposure — still an
// incident (alert + reconcile the float), never silently absorbed"), which
// reconciles and lifts like every other. A field whose every value produces one
// effect is a name, not a decision.
//
// What §7.1 asks to be reconciled is the FLOAT rather than the op, and the
// ledger that will hold it is `treasury`'s, which is unbuilt. When it lands, the
// reconciliation gains a DERIVED half — re-anchoring the belief to the venue's
// posted figure, which is machine work — and that half arrives with its
// consumer. Record §82 §2a is where to start.
//
// So this record is, plainly, a checkbox, and that is the correct shape: §7.1
// makes the operator's act the precondition, and a system demanding evidence for
// it would be inventing a rule the section does not state. The design work is in
// making the act attributable, durable, scoped to ONE incident, and impossible
// to take by accident — see ports.MarginOpJournal and the use case.
type MarginOpReconciliation struct {
	// Op is the single incident this act ends — §7.3's "what".
	//
	// One id, and there is deliberately no bulk form anywhere above this type. A
	// clearance covering every open ambiguity would let one keystroke end an
	// incident the operator has not looked at, which satisfies the audit
	// requirement's shape while making it worthless.
	Op MarginOpID

	// Actor is who took the act — §7.3's "who".
	//
	// Free text and self-asserted, because §7.4's approval kernel is
	// "solo-honest by default" and there is no identity service to check it
	// against. Self-asserted is not the same as absent: the value is inside the
	// hash chain, so it is a claim that cannot be edited afterwards without
	// breaking every record that follows it.
	Actor string

	// At is when the reconciliation was performed — §7.3's "when".
	//
	// The operator's own instant rather than a venue clock, and unlike
	// MarginOp.Watermark that is right rather than a shortcut: this instant
	// anchors no evidence window and is matched against no venue-stamped row. It
	// is the audit's timestamp, and the frame an audit is read in is the one the
	// human acted in.
	At time.Time

	// Note is the accounting trail — what was reconciled and against what.
	//
	// Required, because with the finding deliberately absent it is the only field
	// carrying what was actually done. See ErrMarginOpReconcileNoteUnset.
	Note string
}

// Validate refuses a reconciliation missing any of §7.3's three, and the note.
//
// Every field is required and none has a usable zero — `platform/`'s
// zero-value-is-unusable rule reaching a record outside `platform/`, for exactly
// the reason it exists there: an unpopulated actor reads as an act nobody took,
// and this is the record whose whole purpose is saying who took it.
func (r MarginOpReconciliation) Validate() error {
	if r.Op == "" {
		return ErrMarginOpIDUnset
	}
	if r.Actor == "" {
		return fmt.Errorf("margin op %q reconciliation: %w", r.Op, ErrMarginOpReconcileActorUnset)
	}
	if r.At.IsZero() {
		return fmt.Errorf("margin op %q reconciliation: %w", r.Op, ErrMarginOpReconcileInstantUnset)
	}
	if r.Note == "" {
		return fmt.Errorf("margin op %q reconciliation: %w", r.Op, ErrMarginOpReconcileNoteUnset)
	}
	return nil
}
