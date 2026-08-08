package application

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MarginOpDispatcher is core §7.1's margin top-up protocol, write half: read the
// before-state, fsync the `MarginOp`, call the venue exactly once, and record
// what is now known (task #215).
//
// # It is the crash boundary MarginOpResolver exists on the far side of
//
// Dispatcher's file comment for orders, one subject over and with the recovery
// story removed. §5.3 lets an order be resolved by asking the venue about its
// ClientOrderID; neither venue offers an identity for a margin operation
// (Profiles §4), so the ONLY thing that makes a crashed top-up resolvable is the
// before-state this type writes down before calling. Everything below follows
// from that: the read happens first, the fsync happens before the send, and
// nothing that can fail for a caller-controlled reason sits between them.
//
// # A returned call CONCLUDES, and consulting the evidence path instead would
// # make a successful remedy indistinguishable from a failed one
//
// This is the slice's central decision, and it looks backwards until the second
// half is worked through. The tempting shape is for every op to reach its verdict
// the same way — through domain.MarginOp.Resolve, so §7.1's evidence rule has one
// home. It is wrong twice:
//
//   - The evidence path is a RECONSTRUCTION of what the response would have said,
//     built for the case where no response was read. §4 scopes it that way —
//     `ResolveMarginOp(MarginOpRef)` takes "the journaled local op" — and
//     preferring it over the original observation is the same information at lower
//     fidelity, plus the confounds record §81 enumerated (a coincident fill moves
//     the posted figure too).
//   - Run immediately, it answers `Pending` on a venue that publishes an act
//     history. Task #214 established that the history lags INDEPENDENTLY of the
//     margin figure, so the bound guards its silence — and a top-up resolved
//     inside its own propagation bound therefore has no verdict, stays in flight,
//     and keeps its position latched by the rule below. The position's successful
//     remedy would look exactly like its failure, and §7.1's ladder would escalate
//     to a PackageDelever on a top-up that worked.
//
// So the venue's own acknowledgement is the verdict when there is one, and
// §7.1's evidence contract keeps the job it was written for: ops whose outcome
// nobody observed. What makes the acknowledgement strong enough to journal is
// that engine/venue CHECKS it — the direction and amount on Binance, the market
// and the one-way pin on Bybit — rather than reading a status code.
//
// # At most ONE op in flight per position, and the latch is released by
// # DURABILITY rather than by knowledge
//
// §7.1: "at most one margin op in flight per position (serialization removes the
// self-inflicted confound of two same-amount candidates in one evidence window)".
// The instrument is held from the fsync until the op's verdict is DURABLE, and
// the second half of that is the part that is easy to get wrong.
//
// A verdict this process knows but did not journal is one the evidence path will
// have to reconstruct later — and dispatching a second op against the same
// position is precisely what destroys the evidence, because the posted margin
// then moved by the sum of two deltas and matches neither. So a failed verdict
// write does not release. It is the same field as Outcome.Journaled on the order
// path and it is NOT the same kind of harmless: there, a lost state record is
// re-derived by a resolving query; here, the thing that would re-derive it is
// what the next act corrupts.
//
// # A failed call keeps the position latched, and that IS §7.1's escalation
//
// An op with no verdict — an unknown effect, a shed request, a lost response —
// stays in flight, so no further top-up may be dispatched against that position
// in this process. Nothing re-resolves it live (see Module and task #212's
// standing gap), so in practice the position is latched until the next boot,
// where §7.5's phase resolves it from evidence.
//
// That is not a gap dressed as a design. §7.1 escalates a failed top-up to a
// PackageDelever — "it re-queries margin + position first" — and the prohibition
// it is protecting is the blind delta retry. Under this latch the retry is not
// forbidden by a rule somebody must remember; it is unexpressible.
//
// # Not safe for concurrent use
//
// Dispatcher's contract, unchanged and for its reason. The check-then-hold window
// spans the authorization fsync, so a mutex here would satisfy the race detector
// while leaving two concurrent top-ups on one position exactly as able to
// interleave. Serialisation is the caller's; §3.4's single dispatcher provides it.
type MarginOpDispatcher struct {
	journal ports.MarginOpJournal
	margin  ports.VenueMarginStatus
	gateway ports.MarginGateway
	clk     clock.Clock

	// ids mints the local op identity. See dispatch for why this type mints it
	// rather than taking one.
	ids *ids.Generator

	// inflight is §7.1's serialization: the op currently unresolved on each
	// position.
	//
	// # It starts EMPTY, and unlike DispatchAuthority's empty start that rests on
	// # a premise outside this type
	//
	// DispatchAuthority may start empty because the journal enforces the durable
	// half — recovery finds an authorized order and resolves it by query, never by
	// resubmitting. There is no such enforcement here: an op left `Pending` by
	// §7.5's boot phase is genuinely still landing, and nothing in the log would
	// refuse a second top-up on its position.
	//
	// What covers it today is the POSTURE. MarginResolution.Outcome answers
	// OutcomeIncomplete for a pending op, which is PostureReconcileOnly, and a
	// process at that posture is not trading — so no top-up can be dispatched
	// while one is outstanding. The premise is therefore "a root never runs the
	// ladder below PostureStopOpening", and it is written down here because it
	// lives in a different package and could expire quietly: the day a root drives
	// safety actions through a reconcile-only boot, this map must be seeded from
	// MarginResolution.Resolved rather than started empty.
	//
	// A private field rather than DispatchAuthority's extracted type, on the
	// count that made that extraction necessary: there is exactly ONE producer of
	// margin ops (§7.1's ladder) and one dispatcher per process. A second
	// dispatch path is §59b's trigger to extract this, and it arrives with the
	// second caller rather than in anticipation of one.
	inflight map[ids.VenueInstrument]domain.MarginOpID
}

// NewMarginOpDispatcher constructs the use case, refusing a missing port.
//
// Refused at construction rather than at the first call, for NewDispatcher's
// reason with a shorter fuse: the call that would discover a nil journal is one
// posting margin to a venue during a liquidation-distance incident, and the
// missing crash boundary would be found not to exist at the only moment it
// matters.
//
// The id generator is refused on the same terms. Without it there is no op
// identity, and an identity is not decoration here — it is the journal's subject
// column, the key a resolution attaches its verdict to, and the thing an operator
// clearing a §9 incident names (task #213).
func NewMarginOpDispatcher(j ports.MarginOpJournal, m ports.VenueMarginStatus,
	g ports.MarginGateway, clk clock.Clock, gen *ids.Generator) (*MarginOpDispatcher, error) {

	if j == nil {
		return nil, ErrMarginOpJournalMissing
	}
	if m == nil {
		return nil, ErrVenueMarginStatusMissing
	}
	if g == nil {
		return nil, ErrMarginGatewayMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if gen == nil {
		return nil, ErrIDGeneratorMissing
	}
	return &MarginOpDispatcher{
		journal:  j,
		margin:   m,
		gateway:  g,
		clk:      clk,
		ids:      gen,
		inflight: make(map[ids.VenueInstrument]domain.MarginOpID),
	}, nil
}

// MarginOpDispatch is where one top-up ended up, for a dispatch that reached the
// venue.
//
// It is a separate type from Dispatch's error for Outcome's reason, and the
// safety property is the same one: err != nil means NOTHING WAS SENT.
type MarginOpDispatch struct {
	// Op is the record that was journaled, carried whole. An operator reading a
	// report can see the comparison a later resolution will make — the
	// before-state and the watermark — rather than only this call's answer.
	Op domain.MarginOp

	// Status is what is known now: Applied, NotApplied, or Pending.
	//
	// `Pending` here is the same refusal to conclude it is on the evidence path —
	// "too little has happened for silence to be evidence" — reached from a
	// second observation. `Ambiguous` is deliberately NOT reachable from this
	// path: it is a statement about EVIDENCE that could not be attributed, and a
	// call this system made and did not hear back from has produced no evidence
	// to be ambiguous about yet.
	Status domain.MarginOpStatus

	// Journaled reports whether a concluded Status reached the log. It is false
	// on a Pending, where there is deliberately nothing to write.
	//
	// A false beside a verdict is an alerting matter with a consequence: the
	// verdict must be re-derived from evidence at the next boot, and until it is,
	// the position stays latched — see InFlight.
	Journaled bool

	// Cause is why Status is not Applied — the gateway's failure, joined with
	// ErrEffectUnhandled if the classification came from the fallback. It is nil
	// on a clean call.
	Cause error
}

// InFlight reports whether this dispatch left the position latched under §7.1's
// one-op-per-position rule.
//
// Both disjuncts are the same claim from opposite sides: the op is still in
// flight unless its verdict is BOTH reached and durable. A caller does not need
// to re-derive that from the two fields, for HoldsNewTrading's reason — a
// predicate that lives beside the rule cannot drift from it.
func (d MarginOpDispatch) InFlight() bool {
	concluded, err := d.Status.Concluded()
	if err != nil {
		// Unreachable: dispatch returns one of three declared members. Reported
		// as still-in-flight rather than released, because an unreadable status
		// is not evidence that the position is free.
		return true
	}
	return !concluded || !d.Journaled
}

// InFlightOn reports the op currently unresolved on an instrument, if any.
//
// Exported for the caller §7.1's ladder will be: a Rebalance breach on a position
// that already has an op in flight is not an error to report but a rung to skip,
// and asking beforehand is cheaper than dispatching into a refusal.
func (d *MarginOpDispatcher) InFlightOn(instrument ids.VenueInstrument) (domain.MarginOpID, bool) {
	id, ok := d.inflight[instrument]
	return id, ok
}

// Dispatch posts additional isolated margin against one position, exactly once.
//
// # err != nil means NOTHING WAS SENT
//
// Dispatch's contract for orders, restated because the signature exists to make
// it the only readable one. When err is non-nil no venue call was made, no op is
// in flight, and the caller may act again; when err is nil the gateway ran
// exactly once and MarginOpDispatch says what is known — including when Cause is
// set, because a failed call whose effect is unknown is still a call.
//
// # The order of the steps is the safety argument
//
// Read the before-state, take the venue's clock, mint, journal, hold, send,
// classify, record. Everything above the journal write sends nothing, so a caller
// correcting a refusal and calling again is legitimate. Everything from the send
// onward is unrepeatable for this position until the op resolves.
//
// The read is FIRST and not merely early: the before-state is the whole recovery
// story, so a read that fails must refuse the top-up outright rather than let one
// be sent against a comparison nobody can make afterwards. §7.1's ladder has an
// answer for a refusal here — it escalates, having "re-queried margin + position
// first" — and no answer at all for an unresolvable op.
//
// # The amount is the caller's, and this method does not size it
//
// §6 step 4's `margin-for-target-liq-distance` is `risk`'s, and nothing in this
// system computes it yet. What this method enforces is what §4's port requires of
// any amount: strictly positive, in the position's own margin asset, refused
// rather than clamped.
func (d *MarginOpDispatcher) Dispatch(instrument ids.VenueInstrument, amount fixedpoint.Money) (MarginOpDispatch, error) {
	if err := instrument.Validate(); err != nil {
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up: %w", err)
	}
	if !instrument.Product.IsPerp() {
		// domain.MarginOp.Validate refuses this too, three steps later. Refused
		// here as well because this is the layer that would otherwise READ a spot
		// instrument's margin — a call ports.VenueMarginStatus answers with a
		// surface-absent error whose diagnosis names the venue rather than the
		// caller that asked for something no venue has.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: %w",
			instrument.Symbol, domain.ErrMarginOpNotPerp)
	}
	if held, ok := d.inflight[instrument]; ok {
		// §7.1's serialization. Refused before anything is read, because every
		// step below either spends budget or writes: the whole point of the rule
		// is that a second op must not exist, not that it must be tidied up.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: op %q is in flight: %w",
			instrument.Symbol, held, ErrMarginOpInFlight)
	}

	reading, positionHeld, err := d.margin.MarginStatus(instrument)
	if err != nil {
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: before-state: %w", instrument.Symbol, err)
	}
	if !positionHeld {
		// No position, so no isolated margin to post against and nothing a
		// before/after comparison could be about. This is the ladder acting on a
		// stale view — the position closed between the breach and the remedy —
		// and it is a refusal rather than a no-op, because a caller told "done"
		// would record a remedy that never happened.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: %w",
			instrument.Symbol, ErrMarginPositionFlat)
	}
	if err := reading.Validate(); err != nil {
		// The PORT's own checks, run here rather than assumed. This is an
		// interface, so what arrives is whatever satisfies it — and the figure
		// about to be written down as the before-state is the one half of §7.1's
		// comparison the venue does not keep a copy of.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: before-state: %w", instrument.Symbol, err)
	}
	if reading.Instrument != instrument {
		// Answered about another market. Validate cannot catch this — a reading
		// for a different perpetual is internally consistent — and the
		// before-state would be another position's posted margin, so every later
		// resolution would difference two unrelated figures. That resolves
		// Ambiguous, which is a §9 incident raised for a wiring defect.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: before-state describes %s: %w",
			instrument.Symbol, reading.Instrument.Symbol, ErrMarginBeforeStateMismatch)
	}

	// The VENUE's clock, because the watermark anchors an evidence window that
	// will be compared against venue-stamped history rows (domain.MarginOp.
	// Watermark). Read before the fsync, on Dispatch's rule: an uncalibrated
	// venue is deployment configuration, fixable, and a refusal after the fsync
	// would leave a durable record for an op nothing ever sent.
	watermark := d.clk.ServerNow(instrument.Venue)
	if err := watermark.Validate(); err != nil {
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s on %s: %w",
			instrument.Symbol, instrument.Venue, err)
	}

	id, err := d.mint()
	if err != nil {
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: %w", instrument.Symbol, err)
	}

	op := domain.MarginOp{
		ID:           id,
		Instrument:   instrument,
		Delta:        amount,
		BeforePosted: reading.PostedMargin,
		Watermark:    watermark.Instant(),
	}
	if err := op.Validate(); err != nil {
		// The record's own rules, applied before it is written rather than
		// discovered by the resolver that has to use it: a zero delta, an asset
		// disagreement between the amount and the before-state, a negative posted
		// figure. Every one of them is a record a resolution could not conclude
		// from, and the op it describes would already have been sent.
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: %w", instrument.Symbol, err)
	}

	// §7.1: the record is "fsynced BEFORE dispatch". ports.MarginOpJournal returns
	// only when it is durable, so there is no Sync to pair with this.
	if err := d.journal.AppendMarginOpDispatched(op); err != nil {
		return MarginOpDispatch{}, fmt.Errorf("dispatch margin top-up on %s: %w", instrument.Symbol, err)
	}

	// Held only now. Everything above sent nothing, so a caller that fixes its
	// argument and calls again has not consumed the position's one slot — and
	// holding earlier would latch a position on a refusal, which §7.1's ladder
	// reads as "a top-up is already landing" and skips.
	d.inflight[instrument] = op.ID

	callErr := d.gateway.AddIsolatedMargin(instrument, amount)
	status, unhandled := classifyMarginCall(callErr)

	journaled := false
	concluded, err := status.Concluded()
	if err != nil {
		// classifyMarginCall answers one of three declared members, so this is
		// §3.6 rule 10's failure arriving at runtime. The op stays in flight and
		// the error is reported — the call has ALREADY happened, so there is no
		// refusal available, only an accurate report of not knowing.
		return MarginOpDispatch{Op: op, Status: domain.MarginOpPending, Cause: err}, nil
	}
	if concluded {
		journaled = d.journal.AppendMarginOpResolved(op.ID, status) == nil
		if journaled {
			// Released only on a DURABLE verdict. See the type's doc: an
			// un-journaled verdict is one the evidence path must reconstruct, and
			// a second op on this position is what makes that impossible.
			delete(d.inflight, instrument)
		}
	}

	return MarginOpDispatch{
		Op:        op,
		Status:    status,
		Journaled: journaled,
		Cause:     errors.Join(callErr, unhandled),
	}, nil
}

// mint makes the local op identity.
//
// # This type mints it, where Deleverer.Open takes one
//
// A GenerationID is minted by the caller because the caller is the thing that
// decides a generation exists. A MarginOpID is different in the one way that
// matters: it never leaves this process — the venues supply no identity to borrow
// and none is sent — so a caller-supplied one is a parameter with no external
// meaning and one failure mode. A re-used id puts two dispatch records under one
// subject in an append-only log, which ports.JournalReplay refuses outright
// (ErrMarginOpAlreadyDispatched): the two records name two before-states for one
// comparison, nothing chooses between them, and the process cannot boot at all.
//
// Minting here makes that unexpressible from outside the package, which is the
// same trade domain.MarginActHistory took one slice earlier — close the wrong
// pairing in the signature rather than validate against it.
func (d *MarginOpDispatcher) mint() (domain.MarginOpID, error) {
	u, err := d.ids.NewUUIDv7()
	if err != nil {
		return "", fmt.Errorf("mint margin op id: %w", err)
	}
	return domain.MarginOpID(u.String()), nil
}

// classifyMarginCall decides what a gateway call's outcome establishes about the
// op. It is the safety-critical decision of this use case.
//
// The returned status is always safe to act on. The returned error means the
// classification came from the fallback rather than from a rule, and is advisory
// about this program rather than about the op.
//
// # The tag is the EFFECT, never the cause
//
// §5.3's rule, inherited whole: the adapter is the only layer that knows whether
// the request left the socket, so it records the answer and this function reads
// it. The switch is tagged on apperror.Effect so §3.6 rule 10 forces every
// declared member to be named.
//
// # EffectNotApplied concludes here, where on the ORDER path it does not
//
// This is the one row that differs from classifyEffect, and the difference is not
// a relaxation. There, EffectNotApplied cannot choose between StateRejected and
// StateAbsentAfterFinality, because SM §1 distinguishes those two absence
// terminals by EVIDENCE CLASS (FLAG-2) and the effect alone does not say which.
// A margin op has exactly one absence answer — the margin did not move — and both
// causes EffectNotApplied covers say it: a request that never left the socket
// moved nothing, and a venue that rejected it on its merits moved nothing either.
// So the value is a positive conclusion, which is what §7.1 requires of
// `NotApplied`: "positively concluded rather than assumed from silence".
//
// The failure direction is the mild one of the two §7.1 names. A wrong NotApplied
// ends in a second top-up — "capital-allocation drift, not naked exposure — still
// an incident", the section's own worst case for this direction — while a wrong
// Applied is the one it calls dangerous, because it stops the ladder topping up a
// position that has no margin.
//
// # EffectApplied does NOT conclude, and the asymmetry with the order path is
// # the same rule read the other way
//
// For an order it means StateSubmitted: the send happened, only the ack was
// unreadable. For a margin op "the request reached the venue" is not "the venue
// performed the act" — it may have rejected it after receipt — and there is no
// second authority to ask, so the op stays in flight and the evidence path
// decides. Concluding Applied here would be exactly the assumption §7.1 forbids.
func classifyMarginCall(err error) (domain.MarginOpStatus, error) {
	if err == nil {
		// The venue acknowledged the act, and engine/venue checked that the
		// acknowledgement describes THIS call rather than reading a status code.
		return domain.MarginOpApplied, nil
	}
	return classifyMarginEffect(apperror.EffectOf(err))
}

// classifyMarginEffect is classifyMarginCall's decision, separated from the
// extraction of the axis it decides on.
//
// The split is classifyEffect's and it exists for that function's reason rather
// than for test convenience: apperror.New degrades any invalid Spec to
// EffectUnknown and EffectOf answers EffectUnspecified only for a nil error, so
// two of the four members are unreachable through the public API. A test that
// could only inject ERRORS would be unable to tell EffectUnspecified's membership
// in the group below from its absence — the trailing-return-hides-its-own-group
// trap, arriving through the argument type.
func classifyMarginEffect(effect apperror.Effect) (domain.MarginOpStatus, error) {
	switch effect {
	case apperror.EffectNotApplied:
		return domain.MarginOpNotApplied, nil

	case apperror.EffectApplied, apperror.EffectUnknown, apperror.EffectUnspecified:
		// No conclusion is available. EffectUnspecified reaching here is
		// apperror's deliberate degrade-to-safe: a classification nobody filled
		// in reads as "this may have applied" rather than as a silent
		// "definitely not", which is the direction §7.1 calls dangerous.
		return domain.MarginOpPending, nil
	}

	// Reachable only from an Effect with no case above — §3.6 rule 10's failure
	// arriving at runtime because something removed the gate.
	//
	// It returns a status AND an error, and needs both. Status-only would make
	// this branch answer the identical Pending as the case above it, so deleting
	// a member from that group would change nothing any test could observe.
	// Error-only would leave a caller whose call has ALREADY run with no status
	// to record for an op that may have applied.
	return domain.MarginOpPending, fmt.Errorf("%s: %w", effect, ErrEffectUnhandled)
}
