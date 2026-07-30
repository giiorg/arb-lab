package application

import (
	"fmt"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// silenceTimerPrio is the priority the silence timer is scheduled at.
//
// §3.5 makes the direction normative — LOWER Prio is MORE urgent, so the zero
// value is the most urgent — and this deliberately does not take the zero. A
// stale-mark detection is not more urgent than the safety kernel's own timers,
// and taking 0 by default would put marketdata ahead of them in a drain purely
// because nobody chose. One rung down is a choice; 0 would have been an
// omission wearing the same clothes.
const silenceTimerPrio uint8 = 1

// FundingIngest is the funding path's use case: accept an observation, decide
// whether it advances the source, stamp freshness on it, publish it, and arm
// the timer that will notice if nothing follows.
//
// # It has no goroutine and no ticker, and that is the same call clock made
//
// Every scheduling decision goes through Clock.SetTimer, and delivery is pulled
// by the dispatcher through TimerSource (§3.5). A goroutine here would put the
// arbitration that decides the replay key in two places, and would make the
// backtest's TestClock unable to drive this layer at all — which is not a
// testing inconvenience, it is §3.5's parity mechanism.
//
// # It is not safe for concurrent use, deliberately
//
// §3.4 makes the marketdata reducer single-writer. A mutex here would make
// concurrent use COMPILE and leave the interleaving — and therefore the
// published order — nondeterministic under replay. Not locking is the honest
// encoding of the constraint: it is the dispatcher's job to call this from one
// goroutine, and the race detector will say so if it does not.
type FundingIngest struct {
	clock   clock.Clock
	publish ports.FundingPublisher
	budgets ports.FundingBudgets

	// held is the last accepted observation per instrument. It is what makes
	// supersede and gap detection possible at all — both are questions about a
	// predecessor, which is precisely what the domain cannot answer, being a
	// set of pure functions over a single reading.
	held map[ids.VenueInstrument]domain.FundingObservation

	// armed maps a live timer back to the instrument it watches, so a fired
	// timer can be routed without the caller having to parse its ID. The map
	// is also the membership test ErrUnknownTimer needs.
	armed map[clock.TimerID]ids.VenueInstrument
}

// NewFundingIngest constructs the use case, refusing a missing dependency.
//
// Refused here rather than at first use, on this codebase's standing rule that
// the zero value must be unusable: a FundingIngest{} would otherwise accept
// observations, publish them into a nil interface, and panic on the first
// freshness computation — at which point the diagnosis is a stack trace in the
// hot path rather than a named refusal at boot.
func NewFundingIngest(c clock.Clock, p ports.FundingPublisher, b ports.FundingBudgets) (*FundingIngest, error) {
	if c == nil {
		return nil, ErrClockNil
	}
	if p == nil {
		return nil, ErrPublisherNil
	}
	if b == nil {
		return nil, ErrBudgetsNil
	}
	return &FundingIngest{
		clock:   c,
		publish: p,
		budgets: b,
		held:    make(map[ids.VenueInstrument]domain.FundingObservation),
		armed:   make(map[clock.TimerID]ids.VenueInstrument),
	}, nil
}

// Ingest accepts one funding observation, together with the ports.AcceptOrdinal of
// the raw input it arrived on.
//
// The order of the steps is the point, and it is the order a wrong answer would
// be cheapest to catch in: validate what arrived, decide whether it is newer
// than what is held, only then let it replace the held reading, and only then
// publish. An observation that fails any earlier step never becomes the
// baseline the next one is compared against — which is what stops one corrupt
// reading from poisoning supersede detection for the rest of the session.
//
// # The trigger is a parameter, not a field on the observation
//
// An ports.AcceptOrdinal is a fact about the DELIVERY, not about the reading — the
// same funding numbers re-delivered are a second raw input with a second
// ordinal, and domain.FundingObservation is a value that two deliveries can
// legitimately share. Putting it on the observation would also put a transport
// concern inside the domain, which §3.6 rule 1 exists to prevent.
func (f *FundingIngest) Ingest(obs domain.FundingObservation, trigger ports.AcceptOrdinal) (ports.FundingSnapshot, error) {
	// Checked before the observation, because the two failures have different
	// owners and this one is the caller's. This refusal is the load-bearing
	// half of §3.5's producer rule — "a producer MUST refuse a zero ordinal on
	// an input-derived envelope" — since Envelope.Validate deliberately passes
	// zero (it is the absence-derived reading, and Validate cannot see
	// derivation; §39p-5). Every observation entering here is input-derived by
	// definition of Ingest, so zero means the trace was left out, and
	// reporting a malformed reading first would send the reader to the feed.
	if trigger == 0 {
		return ports.FundingSnapshot{}, fmt.Errorf("instrument %s: %w", obs.Instrument, ErrNoTrigger)
	}
	if err := obs.Validate(); err != nil {
		return ports.FundingSnapshot{}, fmt.Errorf("funding observation: %w", err)
	}

	inst := obs.Instrument
	budget, ok := f.budgets.FundingBudget(inst.Venue)
	if !ok {
		return ports.FundingSnapshot{}, fmt.Errorf("venue %s: %w", inst.Venue, ErrNoBudget)
	}
	if err := budget.Validate(); err != nil {
		return ports.FundingSnapshot{}, fmt.Errorf("venue %s budget: %w", inst.Venue, err)
	}

	prev, hadPrev := f.held[inst]
	gap := false
	if hadPrev {
		advanced, hadGap := advances(prev.Watermark, obs.Watermark)
		if !advanced {
			return ports.FundingSnapshot{}, fmt.Errorf(
				"instrument %s: venue seq %d, received %s: %w",
				inst, obs.Watermark.VenueSeq, obs.Watermark.RecvTime, ErrSuperseded)
		}
		gap = hadGap
	}

	snap, err := f.assess(obs, budget, gap, trigger)
	if err != nil {
		return ports.FundingSnapshot{}, err
	}

	// The held reading is replaced BEFORE publishing. If publish fails, the
	// observation still happened and the next one must be compared against it;
	// rolling back would make a publisher outage silently re-open the door to
	// a replayed delivery.
	f.held[inst] = obs
	f.arm(inst, obs.Watermark, budget)

	if err := f.publish.Publish(snap); err != nil {
		return snap, fmt.Errorf("publish %s: %w", inst, err)
	}
	return snap, nil
}

// OnSilence re-assesses an instrument whose silence timer has fired.
//
// # This is the whole reason the timer exists
//
// §3.4: "a feed that simply stops emits no event, so staleness MUST be detected
// by a timer, not only by the next update". The natural implementation checks
// freshness when an update arrives, which is exactly the check a dead feed never
// triggers — the position stays marked fresh for the entire outage, and the
// liq-distance that §7.1 escalates on is computed from a mark nobody has
// questioned.
//
// What it publishes is the SAME observation with a WORSE verdict, which is an
// honest statement and not a fabricated reading: nothing new has been observed,
// and that is the news.
func (f *FundingIngest) OnSilence(id clock.TimerID) (ports.FundingSnapshot, error) {
	inst, ok := f.armed[id]
	if !ok {
		return ports.FundingSnapshot{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}
	// The timer has fired, so it is no longer pending. Dropped before anything
	// that can fail, or a re-assessment that errors would leave a dead ID
	// mapped and the next legitimate arm would look like a duplicate.
	delete(f.armed, id)

	obs, held := f.held[inst]
	if !held {
		// Unreachable while arm() is the only writer of both maps, and refused
		// rather than assumed: it is the shape a future caller introducing a
		// second eviction path would produce, and the failure would otherwise
		// be a zero-valued observation published as real data.
		return ports.FundingSnapshot{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}

	budget, ok := f.budgets.FundingBudget(inst.Venue)
	if !ok {
		return ports.FundingSnapshot{}, fmt.Errorf("venue %s: %w", inst.Venue, ErrNoBudget)
	}

	// Trigger zero, and this is the ONLY place that value is produced. §3.5's
	// inheritance rule covers an envelope derived from a raw input; this one is
	// derived from the absence of one, and there is nothing to inherit.
	//
	// Rejected, and it is what an implementation with nothing to put here would
	// reach for: reusing the held observation's ordinal. That makes the capture
	// log say one raw input produced two dispatched envelopes, so a replay
	// reconciling the two reports a phantom duplicate — and concludes the feed
	// is fine, which is the exact opposite of what this timer fired to say.
	snap, err := f.assess(obs, budget, false, 0)
	if err != nil {
		return ports.FundingSnapshot{}, err
	}

	// Re-armed for the NEXT rung. NextTransition reports false once the
	// observation is blind, so a dead feed arms a bounded number of timers
	// rather than one per rung forever.
	f.arm(inst, obs.Watermark, budget)

	if err := f.publish.Publish(snap); err != nil {
		return snap, fmt.Errorf("publish %s: %w", inst, err)
	}
	return snap, nil
}

// assess wraps the observation in the freshness verdicts computed for it.
//
// The two-clock rule itself lives in assessFreshness, shared with the book path.
// It was inline here until the book path needed the same pair, and the reason it
// moved rather than being copied is the half a reader gets wrong: the DATA
// verdict is measured against Clock.ServerNow(venue), never Clock.Now, and
// Clock.Now is the one already in hand. Two copies of that is two chances to
// write the comparison §3.5 forbids.
func (f *FundingIngest) assess(obs domain.FundingObservation, b domain.Budget, gap bool, trigger ports.AcceptOrdinal) (ports.FundingSnapshot, error) {
	w := obs.Watermark

	transport, data, err := assessFreshness(f.clock, b, w, obs.Instrument)
	if err != nil {
		return ports.FundingSnapshot{}, err
	}

	return ports.FundingSnapshot{
		Observation:  obs,
		Transport:    transport,
		Data:         data,
		GapDetected:  gap,
		SeqAvailable: w.VenueSeq != 0,
		Trigger:      trigger,
	}, nil
}

// arm schedules the instant this instrument's freshness would next degrade.
//
// The arithmetic lives in domain.Budget.NextTransition; what happens here is
// only the scheduling, so the boundary rule is not written down twice. When
// there is no later transition — the observation is already blind, or its
// provenance is broken — any live timer is cancelled rather than left pending:
// a timer that can only re-derive a verdict already reached is a wakeup that
// changes nothing, and §3.4's lane is bounded for a reason.
func (f *FundingIngest) arm(inst ids.VenueInstrument, w domain.Watermark, b domain.Budget) {
	id := silenceTimerID(inst)
	at, ok := b.NextTransition(w, f.clock.Now())
	if !ok {
		f.clock.CancelTimer(id)
		delete(f.armed, id)
		return
	}
	// SetTimer replaces by id (§3.5), so re-arming the same instrument does not
	// accumulate timers. That is why the ID is derived from the instrument and
	// not made unique per call.
	f.clock.SetTimer(id, at, silenceTimerPrio)
	f.armed[id] = inst
}

// silenceTimerID names an instrument's silence timer.
//
// Derived from the instrument so that SetTimer's replace-by-id semantics do the
// deduplication, and prefixed so that one process-wide timer queue shared with
// the safety kernel cannot collide on a bare symbol.
func silenceTimerID(inst ids.VenueInstrument) clock.TimerID {
	return clock.TimerID("marketdata.funding.silence." + inst.String())
}

// advances reports whether next is a later reading of the source than prev,
// and whether the venue's own sequence skipped.
//
// # Two orderings, because only one of them is always available
//
// §3.5 makes VenueSeq authoritative WITHIN a SourceID, so where both readings
// carry one it is the answer, and a jump of more than one is §3.4's sequence
// gap. Where either lacks one — the ordinary case for a REST-polled funding
// read, per domain.Watermark's field comment — the fallback is arrival order,
// which is causally honest: you reacted to what you received, in the order you
// received it.
//
// SourceTime is deliberately NOT the fallback, though it is the tempting one.
// §3.5 rejects it as a reorder key because venue clocks are unsynchronized, and
// a venue that restamps an unchanged funding rate with a fresher SourceTime
// would make a duplicate look like an advance.
//
// Equal arrival instants advance. Two polls landing in the same clock tick are
// ordinary, and the alternative — refusing the second — would silently drop a
// real update whenever the clock's resolution is coarser than the poll
// interval, which is a defect that only appears on someone else's machine.
func advances(prev, next domain.Watermark) (advanced, gap bool) {
	if prev.VenueSeq != 0 && next.VenueSeq != 0 {
		if next.VenueSeq <= prev.VenueSeq {
			return false, false
		}
		return true, next.VenueSeq > prev.VenueSeq+1
	}
	// No sequence on at least one side, so no gap can be claimed either — and
	// reporting false for "gap" here is exactly why ports.FundingSnapshot carries
	// SeqAvailable beside it.
	return !next.RecvTime.Before(prev.RecvTime), false
}
