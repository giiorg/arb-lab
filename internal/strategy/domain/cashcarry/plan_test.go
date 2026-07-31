package cashcarry_test

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

func TestProposeHoldPlanIsTheMinimumHold(t *testing.T) {
	obs := flatView(t, nil).Funding
	plan, err := cashcarry.ProposeHoldPlan(obs, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Settlements) != 3 {
		t.Fatalf("%d settlements, want 3", len(plan.Settlements))
	}
	for n, want := range []time.Time{
		obs.NextSettle,
		obs.NextSettle.Add(8 * time.Hour),
		obs.NextSettle.Add(16 * time.Hour),
	} {
		if !plan.Settlements[n].Equal(want) {
			t.Fatalf("settlement %d = %v, want %v", n, plan.Settlements[n], want)
		}
	}
	// The terminal IS the last settlement: the minimum commitment ends when
	// its last lump lands, and a longer terminal would scale the hurdle for
	// hold the plan never priced.
	if !plan.Terminal.Equal(plan.Settlements[2]) {
		t.Fatalf("terminal %v, want the last settlement", plan.Terminal)
	}
}

func TestProposeHoldPlanSingleSettlement(t *testing.T) {
	obs := flatView(t, nil).Funding
	plan, err := cashcarry.ProposeHoldPlan(obs, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Settlements) != 1 || !plan.Terminal.Equal(obs.NextSettle) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestProposeHoldPlanRefusesZeroMinHold(t *testing.T) {
	if _, err := cashcarry.ProposeHoldPlan(flatView(t, nil).Funding, 0); err == nil {
		t.Fatal("zero min hold accepted")
	}
}

// regenFixture is a working revision three settlements deep on an 8h grid:
// one completed (+2h, behind the evaluation instant at +3h), two future
// (+10h, +18h). The terminal sits at +19h30m — deliberately NOT the last
// settlement: every entry proposal ends exactly on its last settlement, so
// with terminal == last the §40f "regenerate to the UNCHANGED terminal"
// claim is indistinguishable from "regenerate to the last settlement" (the
// checklist's pinned-equal collapse, again).
func regenFixture(t *testing.T) (domain.HoldPlanRevision, time.Time) {
	t.Helper()
	entry := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	prior := domain.HoldPlanRevision{
		Num: 1, Trigger: domain.TriggerEntry, At: entry,
		Plan: domain.HoldPlan{
			Settlements: []time.Time{
				entry.Add(2 * time.Hour),
				entry.Add(10 * time.Hour),
				entry.Add(18 * time.Hour),
			},
			Terminal: entry.Add(19*time.Hour + 30*time.Minute),
		},
	}
	return prior, entry.Add(3 * time.Hour)
}

func TestRegenerateAgreementReturnsThePriorUntouched(t *testing.T) {
	prior, at := regenFixture(t)
	obs := flatView(t, nil).Funding
	obs.NextSettle = prior.Plan.Settlements[1]
	obs.Interval = 8 * time.Hour

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("agreement produced a new revision")
	}
	if got.Num != prior.Num || !got.At.Equal(prior.At) {
		t.Fatalf("prior revision altered: %+v", got)
	}
}

// TestRegenerate8hTo1hRestoresTheOmittedSettlements is §2.4's C8 direction:
// the venue flips to a 1h interval, and an unregenerated 8h plan would omit
// seven of every eight real settlements, biasing exit premature.
func TestRegenerate8hTo1hRestoresTheOmittedSettlements(t *testing.T) {
	prior, at := regenFixture(t)
	obs := flatView(t, nil).Funding
	obs.Interval = time.Hour
	obs.NextSettle = at.Add(time.Hour) // +4h, on the new 1h grid

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("interval flip produced no revision")
	}
	if got.Num != 2 || got.Trigger != domain.TriggerIntervalFlip {
		t.Fatalf("revision %d trigger %s", got.Num, got.Trigger)
	}
	// The completed +2h settlement is retained; the future is the 1h grid
	// from +4h up to the UNCHANGED +19h30m terminal: 16 fresh instants, the
	// last at +19h — inside the terminal, which no settlement sits on. A
	// mutant regenerating "to the last settlement" (+18h) loses one.
	if !got.Plan.Terminal.Equal(prior.Plan.Terminal) {
		t.Fatalf("terminal moved to %v", got.Plan.Terminal)
	}
	if len(got.Plan.Settlements) != 1+16 {
		t.Fatalf("%d settlements, want 17", len(got.Plan.Settlements))
	}
	if !got.Plan.Settlements[0].Equal(prior.Plan.Settlements[0]) {
		t.Fatalf("completed settlement not retained: %v", got.Plan.Settlements[0])
	}
	if !got.Plan.Settlements[1].Equal(at.Add(time.Hour)) {
		t.Fatalf("first regenerated = %v", got.Plan.Settlements[1])
	}
	if last := got.Plan.Settlements[16]; !last.Equal(prior.Plan.Settlements[0].Add(17 * time.Hour)) {
		t.Fatalf("last regenerated = %v, want +19h", last)
	}
}

// TestRegenerate1hTo8hDropsTheInventedSettlements is the mirror: a 1h plan
// on a venue now settling 8-hourly carries settlements that will never
// happen, retaining a loser on phantom carry. One fixture cannot witness
// both directions — the two failures bias opposite ways.
func TestRegenerate1hTo8hDropsTheInventedSettlements(t *testing.T) {
	entry := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	settlements := make([]time.Time, 16)
	for n := range settlements {
		settlements[n] = entry.Add(time.Duration(n+1) * time.Hour)
	}
	prior := domain.HoldPlanRevision{
		Num: 1, Trigger: domain.TriggerEntry, At: entry,
		Plan: domain.HoldPlan{Settlements: settlements, Terminal: settlements[15]},
	}
	at := entry.Add(90 * time.Minute) // one settlement completed

	obs := flatView(t, nil).Funding
	obs.Interval = 8 * time.Hour
	obs.NextSettle = entry.Add(10 * time.Hour)

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Trigger != domain.TriggerIntervalFlip {
		t.Fatalf("changed=%v trigger=%s", changed, got.Trigger)
	}
	// Retained: the one completed 1h settlement. Future: only +10h — the
	// +18h instant of the 8h grid is past the unchanged +16h terminal.
	if len(got.Plan.Settlements) != 2 {
		t.Fatalf("%d settlements, want 2 (one retained + one real)", len(got.Plan.Settlements))
	}
	if !got.Plan.Settlements[1].Equal(obs.NextSettle) {
		t.Fatalf("future = %v", got.Plan.Settlements[1])
	}
	if !got.Plan.Terminal.Equal(prior.Plan.Terminal) {
		t.Fatalf("terminal moved to %v", got.Plan.Terminal)
	}
}

func TestRegenerateScheduleShiftKeepsTheTrigger(t *testing.T) {
	prior, at := regenFixture(t)
	obs := flatView(t, nil).Funding
	obs.Interval = 8 * time.Hour
	// Same cadence, grid slid 30 minutes: a shift, not a flip.
	obs.NextSettle = prior.Plan.Settlements[1].Add(30 * time.Minute)

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Trigger != domain.TriggerScheduleShift {
		t.Fatalf("changed=%v trigger=%s", changed, got.Trigger)
	}
	if got.Num != 2 {
		t.Fatalf("num = %d", got.Num)
	}
}

// TestRegenerateSuccessiveRevisionsCountUp pins that regeneration composes:
// a second flip on top of a revision-2 shift lands at Num 3 with the flip's
// trigger, and revision 1 stays the only TriggerEntry.
func TestRegenerateSuccessiveRevisionsCountUp(t *testing.T) {
	prior, at := regenFixture(t)
	obs := flatView(t, nil).Funding
	obs.Interval = 8 * time.Hour
	obs.NextSettle = prior.Plan.Settlements[1].Add(30 * time.Minute)

	rev2, _, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	obs.Interval = time.Hour
	obs.NextSettle = at.Add(2 * time.Hour)
	rev3, changed, err := cashcarry.RegeneratePlan(rev2, obs, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || rev3.Num != 3 || rev3.Trigger != domain.TriggerIntervalFlip {
		t.Fatalf("rev3 = %+v changed=%v", rev3, changed)
	}
}

// TestRegenerateAtASettlementInstantRetainsIt evaluates exactly ON a prior
// settlement: an instant AT `at` is settling — realized, retained — and the
// regenerated future begins strictly later. No other fixture puts the
// evaluation instant on the grid, so "after" and "at or after" collapse
// everywhere else.
func TestRegenerateAtASettlementInstantRetainsIt(t *testing.T) {
	prior, _ := regenFixture(t)
	at := prior.Plan.Settlements[1] // exactly the +10h settlement
	obs := flatView(t, nil).Funding
	obs.Interval = time.Hour
	// The live grid's own next settlement is also AT the instant: those
	// enumerated entries belong to the realized half and must be dropped,
	// never double-retained beside the prior's copy of the same instant.
	obs.NextSettle = at

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("flip at a settlement instant produced no revision")
	}
	// Retained: +2h and the settling +10h. Future: the 1h grid +11h…+19h.
	if len(got.Plan.Settlements) != 2+9 {
		t.Fatalf("%d settlements", len(got.Plan.Settlements))
	}
	if !got.Plan.Settlements[1].Equal(at) {
		t.Fatalf("the settling instant was not retained: %v", got.Plan.Settlements[1])
	}
	if !got.Plan.Settlements[2].Equal(at.Add(time.Hour)) {
		t.Fatalf("future begins at %v", got.Plan.Settlements[2])
	}
}

func TestRegenerateTerminalBehindNextSettleEmptiesTheFuture(t *testing.T) {
	// The venue's next settlement is past the terminal: nothing left to
	// settle inside the hold. The revision's future is empty — a real
	// state (the plan runs out), not an error.
	prior, at := regenFixture(t)
	obs := flatView(t, nil).Funding
	obs.Interval = 8 * time.Hour
	obs.NextSettle = prior.Plan.Terminal.Add(time.Hour)

	got, changed, err := cashcarry.RegeneratePlan(prior, obs, at)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("want a revision: the prior still held two future settlements")
	}
	if len(got.Plan.Settlements) != 1 {
		t.Fatalf("%d settlements, want the one retained", len(got.Plan.Settlements))
	}
}
