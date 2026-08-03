package application_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # The fixture-collapse pass, run before a single assertion was written
//
// Five parameters of a trade plan sit at values that would make a distinction
// this use case draws unobservable, and each was moved off before anything was
// asserted. Listed because four of them are cheap to re-pin by accident.
//
//  1. THE ATTEMPT. ClientOrderID renders leg and attempt as two decimal digits
//     each, so a plan at attempt 0 mints "…-00-00" for leg 0 — where "leg,
//     attempt" and "attempt, leg" are the same string. Every plan below uses a
//     non-zero attempt, and planAttempt is deliberately not equal to any leg
//     index in the fixtures.
//  2. THE LEG COUNT. At one leg, "mints per leg" and "mints once" agree, and
//     "journals every leg before applying any" has no interleaving to be wrong
//     about. Two is the minimum that separates them and three is what the
//     ordering test uses, so that a failure on the LAST leg still leaves two
//     legs whose applies must not have happened.
//  3. THE SIDE. Both legs buying makes "carries this leg's side" and "carries
//     the first leg's side" the same answer. A cash-and-carry package buys spot
//     and sells the perp, so the realistic fixture is also the discriminating
//     one.
//  4. THE INSTRUMENT. Same for the venue instrument, and it carries a second
//     distinction for free: the spot leg has no ContractType and the perp leg
//     has one, so a Plan that copied one leg's instrument onto the other would
//     produce an order ids.VenueInstrument.Validate refuses.
//  5. THE QUANTITY AND PRICE. Distinct per leg, because a plan that carried one
//     leg's size onto both would otherwise be indistinguishable from a correct
//     one — and it is the field whose corruption is a naked leg rather than an
//     error.

// planAttempt is the fixtures' attempt number. Not 0, not 1, and not any leg
// index used below — see the header's first item.
const planAttempt = 7

// cashAndCarryPlan is §2.2's package: buy spot, sell the perp.
func cashAndCarryPlan(t *testing.T) application.TradePlan {
	t.Helper()
	return application.TradePlan{
		Trade:   testTrade(t),
		Attempt: planAttempt,
		Legs: []application.LegIntent{
			{
				Instrument: spot(),
				Side:       domain.SideBuy,
				Quantity:   qty(t, "0.5"),
				LimitPrice: price(t, "50000"),
			},
			{
				Instrument: perp(),
				Side:       domain.SideSell,
				Quantity:   qty(t, "0.25"),
				LimitPrice: price(t, "50100"),
			},
		},
	}
}

// threeLegPlan exists for the ordering claim only. Its legs differ in every
// field a leg has, for the reason the header gives.
func threeLegPlan(t *testing.T) application.TradePlan {
	t.Helper()
	plan := cashAndCarryPlan(t)
	plan.Legs = append(plan.Legs, application.LegIntent{
		Instrument: spot(),
		Side:       domain.SideSell,
		Quantity:   qty(t, "0.125"),
		LimitPrice: price(t, "50200"),
	})
	return plan
}

func newPlanner(t *testing.T) (*application.Planner, *fakeJournal, *fakeOrders) {
	t.Helper()
	j := newFakeJournal()
	r := newFakeOrders()
	p, err := application.NewPlanner(j, r)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	return p, j, r
}

// wantIDs derives what the plan's ids must be, through ids.NewClientOrderID
// rather than from string literals.
//
// This is the oracle, and its independence is the point: the derivation runs
// from (trade, leg index, attempt) with no reference to what Plan produced, so
// an agreement is evidence rather than a tautology. Literals would agree with
// any format change; a helper reading Plan's output would agree with anything.
func wantIDs(t *testing.T, plan application.TradePlan) []ids.ClientOrderID {
	t.Helper()
	out := make([]ids.ClientOrderID, len(plan.Legs))
	for i := range plan.Legs {
		id, err := ids.NewClientOrderID(ids.ClientOrderRef{
			Trade: plan.Trade, Leg: i, Attempt: plan.Attempt,
		})
		if err != nil {
			t.Fatalf("NewClientOrderID(leg %d): %v", i, err)
		}
		out[i] = id
	}
	return out
}

func TestNewPlannerRefusesAMissingPort(t *testing.T) {
	t.Parallel()

	if _, err := application.NewPlanner(nil, newFakeOrders()); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewPlanner(nil, orders) = %v, want ErrJournalMissing", err)
	}
	if _, err := application.NewPlanner(newFakeJournal(), nil); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewPlanner(journal, nil) = %v, want ErrOrdersMissing", err)
	}
	// The positive control: without it a constructor refusing everything scores
	// two out of two above.
	if _, err := application.NewPlanner(newFakeJournal(), newFakeOrders()); err != nil {
		t.Errorf("NewPlanner with both ports = %v, want nil", err)
	}
}

// TestPlanMintsOneDeterministicIDPerLeg is §6 step 5's first clause.
func TestPlanMintsOneDeterministicIDPerLeg(t *testing.T) {
	t.Parallel()

	p, _, _ := newPlanner(t)
	plan := cashAndCarryPlan(t)

	records, err := p.Plan(plan)
	if err != nil {
		t.Fatalf("Plan = %v, want nil", err)
	}
	if len(records) != len(plan.Legs) {
		t.Fatalf("Plan returned %d records, want %d", len(records), len(plan.Legs))
	}

	want := wantIDs(t, plan)
	for i, rec := range records {
		if rec.Order.ClientOrderID != want[i] {
			t.Errorf("leg %d id = %q, want %q", i, rec.Order.ClientOrderID, want[i])
		}
	}

	// Distinct ids, asserted rather than implied by the derivation above: a
	// minting bug that ignored the leg index would still satisfy a per-leg
	// comparison if the oracle had the same bug, and this does not depend on the
	// oracle at all.
	seen := make(map[ids.ClientOrderID]int, len(records))
	for i, rec := range records {
		if first, dup := seen[rec.Order.ClientOrderID]; dup {
			t.Errorf("legs %d and %d share the id %q", first, i, rec.Order.ClientOrderID)
		}
		seen[rec.Order.ClientOrderID] = i
	}
}

// TestPlanIsAPureFunctionOfThePlan is §47f's "same plan -> same IDs, across
// processes".
//
// Two planners over completely independent fakes, which is as much of "another
// process" as one test binary can be — and it is the whole claim, because what
// makes the property hold is that nothing in the derivation consults a clock, a
// counter or a random source. A planner that had acquired any of those would
// disagree here on the second call.
func TestPlanIsAPureFunctionOfThePlan(t *testing.T) {
	t.Parallel()

	plan := cashAndCarryPlan(t)

	first, _, _ := newPlanner(t)
	second, _, _ := newPlanner(t)

	a, err := first.Plan(plan)
	if err != nil {
		t.Fatalf("first Plan = %v", err)
	}
	b, err := second.Plan(plan)
	if err != nil {
		t.Fatalf("second Plan = %v", err)
	}

	if len(a) != len(b) {
		t.Fatalf("the two planners produced %d and %d records", len(a), len(b))
	}
	for i := range a {
		if a[i].Order != b[i].Order {
			t.Errorf("leg %d differs between planners:\n  %+v\n  %+v", i, a[i].Order, b[i].Order)
		}
	}
}

// TestPlanCarriesEachLegsOwnIntent is the fixture-collapse pass cashed in: every
// field is compared against THAT leg's intent, over legs that differ in all of
// them.
func TestPlanCarriesEachLegsOwnIntent(t *testing.T) {
	t.Parallel()

	p, _, _ := newPlanner(t)
	plan := cashAndCarryPlan(t)

	records, err := p.Plan(plan)
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}

	for i, leg := range plan.Legs {
		got := records[i].Order
		if got.Instrument != leg.Instrument {
			t.Errorf("leg %d instrument = %+v, want %+v", i, got.Instrument, leg.Instrument)
		}
		if got.Side != leg.Side {
			t.Errorf("leg %d side = %s, want %s", i, got.Side, leg.Side)
		}
		if got.Quantity != leg.Quantity {
			t.Errorf("leg %d quantity = %s, want %s", i, got.Quantity, leg.Quantity)
		}
		if got.LimitPrice != leg.LimitPrice {
			t.Errorf("leg %d limit price = %s, want %s", i, got.LimitPrice, leg.LimitPrice)
		}
		if got.ReduceOnly != leg.ReduceOnly {
			t.Errorf("leg %d reduceOnly = %v, want %v", i, got.ReduceOnly, leg.ReduceOnly)
		}

		// §6 step 6's pattern, fixed by the use case rather than taken from the
		// caller — LegIntent has no field for either.
		if got.Type != domain.OrderTypeLimit || got.TimeInForce != domain.TimeInForceIOC {
			t.Errorf("leg %d is %s/%s, want %s/%s — §6 step 6 permits one pair",
				i, got.Type, got.TimeInForce, domain.OrderTypeLimit, domain.TimeInForceIOC)
		}

		// §5.3: an order enters the system at exactly one state, at the first
		// OCC version.
		if records[i].State != domain.StatePlanned {
			t.Errorf("leg %d state = %s, want %s", i, records[i].State, domain.StatePlanned)
		}
		if records[i].Version != 1 {
			t.Errorf("leg %d version = %d, want 1", i, records[i].Version)
		}
	}
}

// TestPlanCarriesReduceOnlyThroughIsTheOtherHalfOfTheFlag exists because every
// leg above has ReduceOnly false, where "carries the caller's flag" and "always
// writes false" are the same predicate.
func TestPlanCarriesReduceOnlyThroughIsTheOtherHalfOfTheFlag(t *testing.T) {
	t.Parallel()

	p, _, _ := newPlanner(t)
	plan := cashAndCarryPlan(t)
	plan.Legs[1].ReduceOnly = true

	records, err := p.Plan(plan)
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}
	if records[0].Order.ReduceOnly {
		t.Error("leg 0 reduceOnly = true, want false")
	}
	if !records[1].Order.ReduceOnly {
		t.Error("leg 1 reduceOnly = false, want true")
	}
}

// TestPlanJournalsEveryLegBeforeApplyingAnyIsTheOrderingClaim is what makes the
// absent TradePlanned record safe, so it is asserted rather than argued.
//
// §6 step 5 requires intent to be durable "before any submission", and Plan's
// phase-wise shape makes the stronger statement: nothing enters the
// current-state store until every leg is on disk. The fixture breaks the LAST
// leg's journal write, so a per-leg interleaving would have applied the first
// two — and the assertion is that the repository is entirely empty.
func TestPlanJournalsEveryLegBeforeApplyingAny(t *testing.T) {
	t.Parallel()

	diskFull := errors.New("no space left on device")
	p, j, r := newPlanner(t)
	plan := threeLegPlan(t)
	j.failPlannedFrom = len(plan.Legs)
	j.failPlannedErr = diskFull

	records, err := p.Plan(plan)
	if !errors.Is(err, diskFull) {
		t.Fatalf("Plan = %v, want the journal's error", err)
	}
	if records != nil {
		t.Errorf("Plan returned %+v alongside its error, want nothing dispatchable", records)
	}
	if len(r.records) != 0 {
		t.Errorf("the repository holds %d records after a failed journal write, want 0", len(r.records))
	}

	// The premise: the first two legs DID reach the journal, so this test is
	// about ordering rather than about a journal that refused everything.
	if got := len(j.planned); got != len(plan.Legs) {
		t.Errorf("the journal saw %d intents, want %d — the fixture depends on the earlier legs being written",
			got, len(plan.Legs))
	}
}

// TestPlanJournalsBeforeApplyingAndInLegOrder pins the sequence of the happy
// path, which the failure test above cannot see.
func TestPlanJournalsBeforeApplyingAndInLegOrder(t *testing.T) {
	t.Parallel()

	p, j, _ := newPlanner(t)
	plan := threeLegPlan(t)

	if _, err := p.Plan(plan); err != nil {
		t.Fatalf("Plan = %v", err)
	}

	want := wantIDs(t, plan)
	got := j.plannedIDs()
	if len(got) != len(want) {
		t.Fatalf("journal recorded %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("journal intent %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestPlanRefusesAnEmptyPlan covers the reading that would otherwise be a
// silent success.
func TestPlanRefusesAnEmptyPlan(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	plan.Legs = nil

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrPlanEmpty) {
		t.Fatalf("Plan of a legless trade = %v, want ErrPlanEmpty", err)
	}
	if len(j.planned) != 0 || len(r.records) != 0 {
		t.Errorf("a refused plan wrote %d intents and %d records, want none", len(j.planned), len(r.records))
	}
}

// TestPlanRefusesAMalformedLegBeforeWritingAnything is the other half of "check
// everything first": leg 1 is the broken one, so leg 0 has already been minted
// and validated when the refusal happens.
func TestPlanRefusesAMalformedLegBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	plan.Legs[1].Quantity = qty(t, "0") // §5.3: an order for nothing is refused

	if _, err := p.Plan(plan); !errors.Is(err, domain.ErrOrderedQuantityNotPositive) {
		t.Fatalf("Plan with a zero-quantity leg = %v, want the domain's refusal", err)
	}
	if len(j.planned) != 0 {
		t.Errorf("the journal recorded %d intents for a plan that was refused, want 0", len(j.planned))
	}
	if len(r.records) != 0 {
		t.Errorf("the repository holds %d records, want 0", len(r.records))
	}
}

// TestPlanRefusesALegAlreadyKnownWithoutJournalingIt is the check whose ordering
// is its entire value.
//
// ApplyPlanned would refuse the duplicate anyway. What the pre-check buys is
// that the journal — which is append-only and therefore cannot be corrected —
// never comes to hold two intents under one ClientOrderID, which is what §7.5's
// replay would have no rule for.
func TestPlanRefusesALegAlreadyKnownWithoutJournalingIt(t *testing.T) {
	t.Parallel()

	p, j, _ := newPlanner(t)
	plan := cashAndCarryPlan(t)

	if _, err := p.Plan(plan); err != nil {
		t.Fatalf("first Plan = %v", err)
	}
	journaledAfterFirst := len(j.planned)

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrLegAlreadyPlanned) {
		t.Fatalf("replanning the same trade and attempt = %v, want ErrLegAlreadyPlanned", err)
	}
	if len(j.planned) != journaledAfterFirst {
		t.Errorf("the journal grew to %d intents on the refused replan, want it to stay at %d",
			len(j.planned), journaledAfterFirst)
	}
}

// TestPlanAcceptsAReplanAtAFreshAttempt is the refusal's positive control: what
// is refused is the reused id, not replanning.
func TestPlanAcceptsAReplanAtAFreshAttempt(t *testing.T) {
	t.Parallel()

	p, _, _ := newPlanner(t)
	plan := cashAndCarryPlan(t)

	first, err := p.Plan(plan)
	if err != nil {
		t.Fatalf("first Plan = %v", err)
	}

	plan.Attempt++
	second, err := p.Plan(plan)
	if err != nil {
		t.Fatalf("Plan at attempt %d = %v, want nil", plan.Attempt, err)
	}

	for i := range first {
		if first[i].Order.ClientOrderID == second[i].Order.ClientOrderID {
			t.Errorf("leg %d kept the id %q across attempts — §5.3 forbids reuse within the recoverable window",
				i, first[i].Order.ClientOrderID)
		}
	}
}

// TestPlanRefusesWhenTheRepositoryCannotAnswer is the distinction the loose
// reading of the pre-check would lose.
//
// "This id is free" and "the repository could not say" are different answers,
// and only one of them makes it safe to journal an intent. A Plan that treated
// any Load error as absence would write an intent for an id that may already
// have one — the exact outcome the pre-check exists to prevent, reached through
// the check itself.
func TestPlanRefusesWhenTheRepositoryCannotAnswer(t *testing.T) {
	t.Parallel()

	unavailable := errors.New("repository unavailable")
	j := newFakeJournal()
	r := newFakeOrders()
	r.failLoad = unavailable
	p, err := application.NewPlanner(j, r)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	_, err = p.Plan(cashAndCarryPlan(t))
	if !errors.Is(err, unavailable) {
		t.Fatalf("Plan against an unavailable repository = %v, want its error", err)
	}
	if errors.Is(err, application.ErrLegAlreadyPlanned) {
		t.Error("Plan reported ErrLegAlreadyPlanned for a repository that answered nothing")
	}
	if len(j.planned) != 0 {
		t.Errorf("the journal recorded %d intents, want 0", len(j.planned))
	}
}

// TestPlanReportsAFailedApplyAndLeavesThePartialPlanDiscardable pins the one
// state Plan can leave behind that is neither clean nor rolled back.
//
// There is no rollback available: the journal is append-only by construction, so
// undoing the applied legs would leave the two stores disagreeing about a trade
// the log says exists. What makes that acceptable is §5.3 — every applied record
// is at StatePlanned, and "a Planned-only order was never dispatched (no venue
// order can exist) -> safe to replan or discard". The caller's correct move is a
// fresh attempt, which the duplicate pre-check then forces rather than suggests.
func TestPlanReportsAFailedApplyAndLeavesThePartialPlanDiscardable(t *testing.T) {
	t.Parallel()

	unavailable := errors.New("repository unavailable")
	j := newFakeJournal()
	r := newFakeOrders()
	r.failApplyPlannedFrom = 2
	r.failApplyPlannedErr = unavailable
	p, err := application.NewPlanner(j, r)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	plan := cashAndCarryPlan(t)
	records, err := p.Plan(plan)
	if !errors.Is(err, unavailable) {
		t.Fatalf("Plan = %v, want the repository's error", err)
	}
	if records != nil {
		t.Errorf("Plan returned %+v alongside its error, want nothing dispatchable", records)
	}

	// Leg 0 stayed. That is the state being pinned, not an accident.
	if len(r.records) != 1 {
		t.Fatalf("the repository holds %d records, want the one leg that applied", len(r.records))
	}
	want := wantIDs(t, plan)
	rec, err := r.Load(want[0])
	if err != nil {
		t.Fatalf("Load leg 0: %v", err)
	}
	if rec.State != domain.StatePlanned {
		t.Errorf("the surviving record is at %s, want %s — §5.3's discardable state",
			rec.State, domain.StatePlanned)
	}

	// And a same-attempt replan is refused rather than silently colliding, so
	// the caller cannot recover by repeating itself.
	r.failApplyPlannedFrom = 0
	if _, err := p.Plan(plan); !errors.Is(err, application.ErrLegAlreadyPlanned) {
		t.Errorf("replan at the same attempt = %v, want ErrLegAlreadyPlanned", err)
	}
}

// TestPlanRefusesALegIndexTheIDFormatCannotHold is the boundary ids owns, seen
// from here: the format holds two decimal digits per field, so leg 100 is not
// expressible and the refusal must arrive before anything is written.
func TestPlanRefusesALegIndexTheIDFormatCannotHold(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	one := plan.Legs[0]
	for len(plan.Legs) < 101 {
		plan.Legs = append(plan.Legs, one)
	}

	if _, err := p.Plan(plan); !errors.Is(err, ids.ErrLegRange) {
		t.Fatalf("Plan with 101 legs = %v, want ErrLegRange", err)
	}
	if len(j.planned) != 0 || len(r.records) != 0 {
		t.Errorf("a refused plan wrote %d intents and %d records, want none", len(j.planned), len(r.records))
	}
}

// TestPlannedOrdersAreDispatchable closes the loop inside this package: the
// records Plan produces are exactly what Dispatch admits.
//
// Not a composition test — oms/module_test.go does that against the real
// adapters — but the claim that the two use cases agree on what a planned order
// looks like, which is the one thing a shared fake CAN establish. Without it,
// Plan could mint an order every admit check rejects and both suites would still
// be green.
func TestPlannedOrdersAreDispatchable(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	r := newFakeOrders()
	placer := &fakePlacer{}

	planner, err := application.NewPlanner(j, r)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	dispatcher, err := application.NewDispatcher(j, placer, r, testClock(t))
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	records, err := planner.Plan(cashAndCarryPlan(t))
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}

	for i, rec := range records {
		out, err := dispatcher.Dispatch(rec.Order.ClientOrderID, rec.Version, ports.UrgencyOpening)
		if err != nil {
			t.Fatalf("Dispatch of planned leg %d = %v, want nil", i, err)
		}
		if out.State != domain.StateSubmitted {
			t.Errorf("leg %d reached %s, want %s", i, out.State, domain.StateSubmitted)
		}
		if !out.Applied {
			t.Errorf("leg %d was not applied", i)
		}
	}
	if placer.calls != len(records) {
		t.Errorf("Place called %d times, want %d — once per leg", placer.calls, len(records))
	}
}
