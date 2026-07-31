package cashcarry

import (
	"fmt"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// ProposeHoldPlan builds the entry proposal: the MINIMUM policy-admissible
// hold — exactly minHold settlements from the live observation, terminal at
// the last of them.
//
// # Why the minimum, and not a length param
//
// §5.2's cash-and-carry roster declares no hold-length param, and inventing
// one would be a config knob the spec never asked for. What the roster does
// declare is min_hold_settlements — the fewest settlements a hold may be
// entered for — and the exit direction re-scores continuously: a hold whose
// carry stays good outlives its plan through repeated Continue verdicts and a
// successor entry, while a hold that can't justify even the minimum
// commitment should not open. So the entry gate scores the minimum
// commitment, which is also the CONSERVATIVE horizon: the near settlements
// are the ones the live estimate and the EWMA know most about. Recorded for
// the slice C close's reading-pass docket.
//
// FundingObservation.Enumerate is the only settlement source (§40f rule 1);
// this function derives the `until` bound and carries the result.
func ProposeHoldPlan(obs mdomain.FundingObservation, minHold uint32) (domain.HoldPlan, error) {
	if err := obs.Validate(); err != nil {
		return domain.HoldPlan{}, fmt.Errorf("propose plan: %w", err)
	}
	if minHold == 0 {
		return domain.HoldPlan{}, fmt.Errorf("propose plan: %w", domain.ErrUnset)
	}
	until := obs.NextSettle.Add(time.Duration(minHold-1) * obs.Interval)
	settlements, err := obs.Enumerate(until)
	if err != nil {
		return domain.HoldPlan{}, fmt.Errorf("propose plan: %w", err)
	}
	plan := domain.HoldPlan{Settlements: settlements, Terminal: until}
	if err := plan.Validate(); err != nil {
		return domain.HoldPlan{}, fmt.Errorf("propose plan: %w", err)
	}
	return plan, nil
}

// RegeneratePlan is §40f's interval-flip rule: if the venue's live schedule
// disagrees with the working revision's remaining settlements, produce the
// next revision — completed settlements retained, future instants regenerated
// from the live Interval/NextSettle, terminal unchanged. The bool reports
// whether a new revision was produced; false returns the prior revision
// untouched.
//
// "Completed" is at-or-before `at`, matching DecisionInput's forecast
// boundary exactly (a settlement AT the evaluation instant is settling — a
// realized fact, not a forecastable one), so the retained half here and the
// forecastable half there partition the schedule with no instant in both.
//
// The trigger classification is best-effort labeling over the schedules
// themselves: a prior cadence (any two consecutive retained-or-future
// instants) that disagrees with the live interval is an interval flip;
// anything else that moved the grid is a schedule shift. The schedule is the
// normative content; the trigger is the journal's explanation of it.
func RegeneratePlan(prior domain.HoldPlanRevision, obs mdomain.FundingObservation, at time.Time) (domain.HoldPlanRevision, bool, error) {
	if err := prior.Validate(); err != nil {
		return domain.HoldPlanRevision{}, false, fmt.Errorf("regenerate: prior: %w", err)
	}
	if err := obs.Validate(); err != nil {
		return domain.HoldPlanRevision{}, false, fmt.Errorf("regenerate: observation: %w", err)
	}
	if at.IsZero() {
		return domain.HoldPlanRevision{}, false, fmt.Errorf("regenerate at: %w", domain.ErrUnset)
	}

	terminal := prior.Plan.Terminal
	var retained, priorFuture []time.Time
	for _, t := range prior.Plan.Settlements {
		if t.After(at) {
			priorFuture = append(priorFuture, t)
		} else {
			retained = append(retained, t)
		}
	}

	// The live future: what the venue's own schedule puts strictly after
	// `at`, up to the unchanged terminal. Enumerate may legitimately begin at
	// or before `at` (NextSettle can be the settling instant); those entries
	// belong to the realized half and are dropped, not retained — retention
	// is the PRIOR revision's evidence, never the live grid's back-fill.
	var liveFuture []time.Time
	if !terminal.Before(obs.NextSettle) {
		enumerated, err := obs.Enumerate(terminal)
		if err != nil {
			return domain.HoldPlanRevision{}, false, fmt.Errorf("regenerate: %w", err)
		}
		for _, t := range enumerated {
			if t.After(at) {
				liveFuture = append(liveFuture, t)
			}
		}
	}

	if equalInstants(priorFuture, liveFuture) {
		return prior, false, nil
	}

	trigger := domain.TriggerScheduleShift
	if cadence, ok := priorCadence(prior.Plan.Settlements); ok && cadence != obs.Interval {
		trigger = domain.TriggerIntervalFlip
	}

	next := domain.HoldPlanRevision{
		Num:     prior.Num + 1,
		Trigger: trigger,
		At:      at,
		Plan: domain.HoldPlan{
			Settlements: append(append([]time.Time{}, retained...), liveFuture...),
			Terminal:    terminal,
		},
	}
	if err := next.Validate(); err != nil {
		return domain.HoldPlanRevision{}, false, fmt.Errorf("regenerate: %w", err)
	}
	return next, true, nil
}

func equalInstants(a, b []time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for n := range a {
		if !a[n].Equal(b[n]) {
			return false
		}
	}
	return true
}

// priorCadence is the prior schedule's own spacing: the gap between its last
// two instants, the pair most likely to reflect the revision's current
// interval (an earlier flip already regenerated everything after it).
func priorCadence(settlements []time.Time) (time.Duration, bool) {
	if len(settlements) < 2 {
		return 0, false
	}
	last := len(settlements) - 1
	return settlements[last].Sub(settlements[last-1]), true
}
