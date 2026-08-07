package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// markSilenceTimerPrio is the priority the mark's silence timer is scheduled
// at.
//
// It is DELIBERATELY the same rung as funding's, and not more urgent. The
// temptation is real — this timer is the one §7.1's liq-distance escalation
// hangs off, so it looks like the most important wakeup marketdata schedules —
// and taking a lower Prio would put a market-data timer ahead of the safety
// kernel's own, which is the comparison silenceTimerPrio already refuses. What
// this timer produces is a VERDICT that something else escalates on; the
// escalation's urgency belongs to the escalation.
const markSilenceTimerPrio uint8 = silenceTimerPrio

// MarkIngest is the mark path's use case: accept a reading, decide whether it
// advances the source, stamp freshness on it, publish it, and arm the timer
// that will notice if nothing follows.
//
// # It is FundingIngest's shape, and the timer is the reason it exists at all
//
// The mark is §3.4's third data class and gets the same treatment for the same
// reasons — no goroutine and no ticker, because every scheduling decision goes
// through Clock.SetTimer and delivery is pulled by the dispatcher (§3.5); and
// no mutex, because §3.4 makes the marketdata reducer single-writer and a lock
// here would make concurrent use compile while leaving the published order
// nondeterministic under replay.
//
// What is NOT shared with FundingIngest is the gap machinery: MarkSnapshot
// carries no gap verdict, because neither venue supplies a sequence at this
// class's granularity. See ports.MarkSnapshot.
//
// # The silence timer is the whole point, and §7.1 says why
//
// §7.1's trigger "MUST NOT be account-event-only", and the failure it names is
// the same shape one level down: a mark feed that simply stops emits no event,
// so an implementation that re-assesses freshness only on arrival marks the
// position fresh for the entire outage. The liq distance §7.1 escalates on is
// then computed from a mark nobody has questioned, and the first symptom is a
// liquidation. OnSilence is what makes the absence itself an event.
type MarkIngest struct {
	clock   clock.Clock
	publish ports.MarkPublisher
	budgets ports.MarkBudgets

	// held is the last accepted reading per instrument — what makes supersede
	// detection possible at all, and what OnSilence re-assesses.
	held map[ids.VenueInstrument]domain.MarkObservation

	// armed maps a live timer back to the instrument it watches, so a fired
	// timer can be routed without the caller parsing its ID. The map is also
	// the membership test ErrUnknownTimer needs.
	armed map[clock.TimerID]ids.VenueInstrument
}

// NewMarkIngest constructs the use case, refusing a missing dependency.
//
// Refused here rather than at first use, on this codebase's standing rule that
// the zero value must be unusable — and the direction is the one that matters:
// a MarkIngest{} would accept readings, publish them into a nil interface and
// panic on the first freshness computation, so the diagnosis would be a stack
// trace in the hot path instead of a named refusal at boot.
func NewMarkIngest(c clock.Clock, p ports.MarkPublisher, b ports.MarkBudgets) (*MarkIngest, error) {
	if c == nil {
		return nil, ErrClockNil
	}
	if p == nil {
		return nil, ErrPublisherNil
	}
	if b == nil {
		return nil, ErrBudgetsNil
	}
	return &MarkIngest{
		clock:   c,
		publish: p,
		budgets: b,
		held:    make(map[ids.VenueInstrument]domain.MarkObservation),
		armed:   make(map[clock.TimerID]ids.VenueInstrument),
	}, nil
}

// Ingest accepts one mark reading, together with the ports.AcceptOrdinal of the
// raw input it arrived on.
//
// FundingIngest.Ingest's step order, for its reason: validate what arrived,
// decide whether it is newer than what is held, only then let it replace the
// held reading, and only then publish. A reading that fails an earlier step
// never becomes the baseline the next one is compared against, so one corrupt
// message cannot poison supersede detection for the rest of the session.
func (m *MarkIngest) Ingest(obs domain.MarkObservation, trigger ports.AcceptOrdinal) (ports.MarkSnapshot, error) {
	// Checked before the observation, because the two failures have different
	// owners and this one is the caller's — §3.5's producer rule, which
	// Envelope.Validate deliberately cannot enforce (it is the absence-derived
	// reading, and Validate cannot see derivation).
	if trigger == 0 {
		return ports.MarkSnapshot{}, fmt.Errorf("instrument %s: %w", obs.Instrument, ErrNoTrigger)
	}
	if err := obs.Validate(); err != nil {
		return ports.MarkSnapshot{}, fmt.Errorf("mark observation: %w", err)
	}

	inst := obs.Instrument
	budget, ok := m.budgets.MarkBudget(inst.Venue)
	if !ok {
		return ports.MarkSnapshot{}, fmt.Errorf("venue %s: %w", inst.Venue, ErrNoBudget)
	}
	if err := budget.Validate(); err != nil {
		return ports.MarkSnapshot{}, fmt.Errorf("venue %s budget: %w", inst.Venue, err)
	}

	// advances is shared with the funding path. Its gap half is discarded here
	// rather than plumbed to a field: see ports.MarkSnapshot for why no gap
	// verdict is computable at this class's granularity.
	if prev, hadPrev := m.held[inst]; hadPrev {
		if advanced, _ := advances(prev.Watermark, obs.Watermark); !advanced {
			return ports.MarkSnapshot{}, fmt.Errorf(
				"instrument %s: venue seq %d, received %s: %w",
				inst, obs.Watermark.VenueSeq, obs.Watermark.RecvTime, ErrSuperseded)
		}
	}

	snap, err := m.assess(obs, budget, trigger)
	if err != nil {
		return ports.MarkSnapshot{}, err
	}

	// The held reading is replaced BEFORE publishing. If publish fails, the
	// reading still happened and the next one must be compared against it;
	// rolling back would make a publisher outage silently re-open the door to a
	// replayed delivery.
	m.held[inst] = obs
	m.arm(inst, obs.Watermark, budget)

	if err := m.publish.Publish(snap); err != nil {
		return snap, fmt.Errorf("publish %s: %w", inst, err)
	}
	return snap, nil
}

// OnSilence re-assesses an instrument whose silence timer has fired.
//
// What it publishes is the SAME reading with a WORSE verdict, which is an
// honest statement rather than a fabricated observation: nothing new has been
// seen, and that is the news. For this class the news is what §5.2's
// mark_blind_action is armed from.
func (m *MarkIngest) OnSilence(id clock.TimerID) (ports.MarkSnapshot, error) {
	inst, ok := m.armed[id]
	if !ok {
		return ports.MarkSnapshot{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}
	// The timer has fired, so it is no longer pending. Dropped before anything
	// that can fail, or a re-assessment that errors would leave a dead ID
	// mapped and the next legitimate arm would look like a duplicate.
	delete(m.armed, id)

	obs, held := m.held[inst]
	if !held {
		// Unreachable while arm() is the only writer of both maps, and refused
		// rather than assumed: it is the shape a future caller introducing a
		// second eviction path would produce, and the failure would otherwise
		// be a zero-valued observation published as a real mark — which for
		// this class is a mark of zero entering a liq-distance denominator.
		return ports.MarkSnapshot{}, fmt.Errorf("timer %q: %w", id, ErrUnknownTimer)
	}

	budget, ok := m.budgets.MarkBudget(inst.Venue)
	if !ok {
		return ports.MarkSnapshot{}, fmt.Errorf("venue %s: %w", inst.Venue, ErrNoBudget)
	}

	// Trigger zero: this reading is derived from the ABSENCE of an input, so
	// §3.5's inheritance rule has nothing to inherit from. Reusing the held
	// reading's ordinal — the move an implementation with nothing to put here
	// reaches for — would make the capture log say one raw input produced two
	// dispatched envelopes, so a replay reconciling the two reports a phantom
	// duplicate and concludes the feed is fine. Which is the opposite of what
	// this timer fired to say.
	snap, err := m.assess(obs, budget, 0)
	if err != nil {
		return ports.MarkSnapshot{}, err
	}

	// Re-armed for the NEXT rung. NextTransition reports false once the reading
	// is blind, so a dead feed arms a bounded number of timers rather than one
	// per rung for ever.
	m.arm(inst, obs.Watermark, budget)

	if err := m.publish.Publish(snap); err != nil {
		return snap, fmt.Errorf("publish %s: %w", inst, err)
	}
	return snap, nil
}

// assess wraps the reading in the freshness verdicts computed for it. The
// two-clock rule lives in assessFreshness, shared with the funding and book
// paths — the half a reader gets wrong is that the DATA verdict is measured
// against Clock.ServerNow(venue) and never Clock.Now.
func (m *MarkIngest) assess(
	obs domain.MarkObservation, b domain.Budget, trigger ports.AcceptOrdinal,
) (ports.MarkSnapshot, error) {
	transport, data, err := assessFreshness(m.clock, b, obs.Watermark, obs.Instrument)
	if err != nil {
		return ports.MarkSnapshot{}, err
	}
	return ports.MarkSnapshot{
		Observation: obs,
		Transport:   transport,
		Data:        data,
		Trigger:     trigger,
	}, nil
}

// arm schedules the instant this instrument's freshness would next degrade.
//
// The arithmetic lives in domain.Budget.NextTransition so the boundary rule is
// not written twice — and that boundary is the one whose composition produced a
// timer SPIN on the funding path, which is why it is not restated here.
func (m *MarkIngest) arm(inst ids.VenueInstrument, w domain.Watermark, b domain.Budget) {
	id := markSilenceTimerID(inst)
	at, ok := b.NextTransition(w, m.clock.Now())
	if !ok {
		m.clock.CancelTimer(id)
		delete(m.armed, id)
		return
	}
	// SetTimer replaces by id (§3.5), so re-arming the same instrument does not
	// accumulate timers.
	m.clock.SetTimer(id, at, markSilenceTimerPrio)
	m.armed[id] = inst
}

// markSilenceTimerID names an instrument's mark silence timer.
//
// The "mark" segment is load-bearing, not decoration. SetTimer replaces by ID
// across ONE process-wide queue, so a mark timer sharing an ID with the funding
// timer for the same instrument would silently replace it — and the survivor
// would be whichever class armed last, leaving the other class's staleness
// undetectable while both call sites look correct.
func markSilenceTimerID(inst ids.VenueInstrument) clock.TimerID {
	return clock.TimerID("marketdata.mark.silence." + inst.String())
}
