package domain

import (
	"fmt"
	"time"
)

// HoldPlan is §2.4's durable planned hold: "explicit settlement timestamps +
// terminal time, recorded at entry, so exit evaluates its remaining
// settlements unambiguously".
//
// The schedule comes from FundingObservation.Enumerate and nowhere else
// (§40f rule 1) — this type carries a schedule, it never computes one.
type HoldPlan struct {
	// Settlements are the planned settlement instants, strictly ascending,
	// none after Terminal. Empty is representable — a hold too short to reach
	// a settlement captures nothing, which is a real answer (Enumerate's own
	// rule) — and it is policy (min_hold_settlements), not this type, that
	// refuses to ENTER such a hold.
	Settlements []time.Time

	// Terminal is the planned end of the hold. §40f's regeneration rule
	// regenerates "to the unchanged terminal time", so it is the one instant
	// an interval flip may never move.
	Terminal time.Time
}

// Validate reports whether the plan could have been enumerated from a real
// observation.
func (p HoldPlan) Validate() error {
	if p.Terminal.IsZero() {
		return fmt.Errorf("terminal: %w", ErrUnset)
	}
	for n, t := range p.Settlements {
		if t.IsZero() {
			return fmt.Errorf("settlement %d: %w", n, ErrUnset)
		}
		if n > 0 && !p.Settlements[n-1].Before(t) {
			return fmt.Errorf("settlement %d at %s, %d at %s: %w",
				n-1, p.Settlements[n-1], n, t, ErrNotAscending)
		}
		if t.After(p.Terminal) {
			return fmt.Errorf("settlement %d at %s, terminal %s: %w", n, t, p.Terminal, ErrScheduleOverrun)
		}
	}
	return nil
}

// PlanTrigger names what produced a HoldPlanRevision. Journaled with the
// revision (§40f: "the revision is journaled with its trigger"), because a
// schedule that changed with no recorded cause is indistinguishable from a
// schedule that was edited.
type PlanTrigger string

const (
	// TriggerEntry is revision 1: the plan recorded at entry, immutable
	// evidence thereafter.
	TriggerEntry PlanTrigger = "entry"

	// TriggerIntervalFlip is a regeneration caused by the venue changing the
	// funding interval mid-position — §2.4's own C8 case ("Bybit can flip an
	// alt's interval 8h↔1h without notice"). 8h→1h omits seven of every
	// eight real settlements if unregenerated (biasing exit premature);
	// 1h→8h invents settlements that will never happen (retaining a loser).
	TriggerIntervalFlip PlanTrigger = "interval_flip"

	// TriggerScheduleShift is a regeneration caused by the venue moving
	// NextSettle off the grid the previous revision assumed, with the
	// interval unchanged.
	TriggerScheduleShift PlanTrigger = "schedule_shift"
)

// Validate reports whether t is one of the declared triggers.
func (t PlanTrigger) Validate() error {
	switch t {
	case TriggerEntry, TriggerIntervalFlip, TriggerScheduleShift:
		return nil
	}
	return fmt.Errorf("%q: %w", string(t), ErrKindInvalid)
}

func (t PlanTrigger) String() string { return string(t) }

// HoldPlanRevision is the WORKING plan: §40f's rule that "the entry HoldPlan
// is immutable evidence, and the working plan is a journaled
// HoldPlanRevision". Continue consumes the current revision; the entry
// revision is Num 1 with TriggerEntry.
//
// The regeneration rule itself — retained completed settlements, future
// instants regenerated from the live Interval/NextSettle to the unchanged
// terminal — is slice C's function; this type is its journaled result.
type HoldPlanRevision struct {
	// Num counts from 1 (the entry plan). Zero is unset, refused — a
	// revision number is a journal coordinate, and §5.1's OCC argument
	// applies: the zero a caller never populated must match nothing.
	Num uint64

	// Trigger names what produced this revision.
	Trigger PlanTrigger

	// At is the instant the revision took effect, from the injected Clock.
	At time.Time

	// Plan is the schedule this revision holds.
	Plan HoldPlan
}

// Validate reports whether the revision is a journalable plan state.
func (r HoldPlanRevision) Validate() error {
	if r.Num == 0 {
		return fmt.Errorf("num: %w", ErrUnset)
	}
	if err := r.Trigger.Validate(); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	// Revision 1 is the entry plan and nothing else may claim to be: a later
	// "entry" is a regeneration wearing the entry's label, and the dwell rule
	// depends on the difference (see ErrRevisionSequence).
	if (r.Num == 1) != (r.Trigger == TriggerEntry) {
		return fmt.Errorf("num %d with trigger %s: %w", r.Num, r.Trigger, ErrRevisionSequence)
	}
	if r.At.IsZero() {
		return fmt.Errorf("at: %w", ErrUnset)
	}
	if err := r.Plan.Validate(); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	return nil
}
