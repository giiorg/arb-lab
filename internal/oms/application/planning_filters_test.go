package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §2.4's verification half, at the planner. The broken leg in every fixture
// below is leg 1 (or later), so leg 0 has already minted, validated and
// conformed when the refusal lands — each test therefore also asserts the
// whole-plan property: NOTHING is journaled and nothing is applied.
//
// The fixture-collapse notes for the grid values live on testFilters; the
// off-grid values here are chosen to stay separated from digit counting
// (0.48 has TWO decimals against a THREE-decimal step of 0.05 — a
// decimal-places reading accepts it, the grid refuses it; 50120 is a round
// integer off the 25 grid).

func refusedPlanLeavesNothing(t *testing.T, j *fakeJournal, r *fakeOrders) {
	t.Helper()
	if len(j.planned) != 0 {
		t.Errorf("the journal recorded %d intents for a refused plan, want 0", len(j.planned))
	}
	if len(r.records) != 0 {
		t.Errorf("the repository holds %d records for a refused plan, want 0", len(r.records))
	}
}

func TestPlanRefusesAQuantityOffTheLotGrid(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	// Two decimal places against the perp's 0.05 step: 0.48/0.05 = 9.6. A
	// digit-counting check passes it; the venue's grid does not.
	plan.Legs[1].Quantity = qty(t, "0.48")

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Fatalf("Plan with an off-step leg = %v, want ErrQuantityOffLotStep", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

func TestPlanRefusesAQuantityBelowTheVenueMinimum(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	// 0.05 is ON the perp's 0.05 grid and below its 0.1 minimum, so this
	// isolates the floor check from the grid check.
	plan.Legs[1].Quantity = qty(t, "0.05")

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrBelowMinQty) {
		t.Fatalf("Plan with a sub-minimum leg = %v, want ErrBelowMinQty", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

func TestPlanRefusesALimitPriceOffTheTickGrid(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	// A round integer off the perp's 20 grid: 50110/20 = 2505.5. Quantity
	// stays conforming, so the refusal can only be the tick's.
	plan.Legs[1].LimitPrice = price(t, "50110")

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrPriceOffTick) {
		t.Fatalf("Plan with an off-tick leg = %v, want ErrPriceOffTick", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

func TestPlanRefusesANotionalBelowTheVenueFloor(t *testing.T) {
	t.Parallel()

	p, j, r := newPlanner(t)
	plan := cashAndCarryPlan(t)
	// 0.1 BTC at 50100 = 5010 USDT, under the perp's 10000 floor — while the
	// quantity is on-grid and at exactly the venue minimum, so every earlier
	// check passes and the refusal isolates the notional.
	plan.Legs[1].Quantity = qty(t, "0.1")

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrBelowMinNotional) {
		t.Fatalf("Plan with an under-notional leg = %v, want ErrBelowMinNotional", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

// TestNotionalRoundsTowardZeroAtTheFloor separates the rounding DIRECTION
// from the comparison — a dimension every other fixture collapses, because
// their products are exact at scale 9 and every rounding mode returns the
// same Money. Here the exact product needs scale 10: 0.5 × 0.000000003 =
// 0.0000000015, which TowardZero understates to 0.000000001 and HalfUp would
// lift to 0.000000002. Against a floor of exactly 0.000000002 the two modes
// give OPPOSITE verdicts, and the required one is the refusal: the window
// rounding opens must be refuse-what-the-venue-accepts, never the reverse.
func TestNotionalRoundsTowardZeroAtTheFloor(t *testing.T) {
	t.Parallel()

	filters := testFilters(t)
	edge := filters[spot()]
	edge.Step = qty(t, "0.5")
	edge.MinQty = qty(t, "0.5")
	edge.Tick = price(t, "0.000000001") // every scale-9 price conforms
	edge.MinNotional = usdt(t, "0.000000002")
	filters[spot()] = edge

	j := newFakeJournal()
	r := newFakeOrders()
	p, err := application.NewPlanner(j, r, filters)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	plan := cashAndCarryPlan(t)
	plan.Legs[0].LimitPrice = price(t, "0.000000003")

	if _, err := p.Plan(plan); !errors.Is(err, application.ErrBelowMinNotional) {
		t.Fatalf("Plan at the rounding edge = %v, want ErrBelowMinNotional — HalfUp would have passed it", err)
	}
	refusedPlanLeavesNothing(t, j, r)

	// The positive control, one ulp down: a floor of 0.000000001 is met by
	// the SAME truncated notional, so the refusal above is the direction
	// biting and not a fixture that refuses everything near zero.
	edge.MinNotional = usdt(t, "0.000000001")
	filters[spot()] = edge
	p2, err := application.NewPlanner(newFakeJournal(), newFakeOrders(), filters)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	if _, err := p2.Plan(plan); err != nil {
		t.Fatalf("Plan one ulp under the edge = %v, want nil", err)
	}
}

// TestPlanRefusesAnInstrumentWithoutFilters is the absent-entry reading:
// refuse and name the instrument, never treat absence as freedom.
func TestPlanRefusesAnInstrumentWithoutFilters(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	r := newFakeOrders()
	filters := testFilters(t)
	delete(filters, perp())
	p, err := application.NewPlanner(j, r, filters)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	if _, err := p.Plan(cashAndCarryPlan(t)); !errors.Is(err, application.ErrFiltersUnknown) {
		t.Fatalf("Plan with an unfiltered instrument = %v, want ErrFiltersUnknown", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

// TestPlanWithNoFiltersRefusesEveryLeg is the nil map's meaning, asserted: a
// root that wires nothing has built a deployment that plans nothing, with the
// refusal naming the first leg's instrument — not a deployment that plans
// everything unchecked, which is the defect this slice closes.
func TestPlanWithNoFiltersRefusesEveryLeg(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	r := newFakeOrders()
	p, err := application.NewPlanner(j, r, nil)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	if _, err := p.Plan(cashAndCarryPlan(t)); !errors.Is(err, application.ErrFiltersUnknown) {
		t.Fatalf("Plan over a nil filters map = %v, want ErrFiltersUnknown", err)
	}
	refusedPlanLeavesNothing(t, j, r)
}

// TestPlanReportsAFiltersLegDisagreementAsAnError covers the wiring defect
// the per-entry construction check cannot see: a filters entry that is
// internally coherent in the WRONG asset (an ETH row wired under the BTC
// perp's key). The mismatch must surface as fixedpoint's error, never fold
// into a false "off grid" — the two need different fixes and the message
// should point at the wiring, not the order.
func TestPlanReportsAFiltersLegDisagreementAsAnError(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	r := newFakeOrders()
	filters := testFilters(t)
	ethStep, err := fixedpoint.ParseQuantity("ETH", "0.05")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	ethMin, err := fixedpoint.ParseQuantity("ETH", "0.1")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	ethTick, err := fixedpoint.ParsePrice("ETH", "USDT", "20")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	wrong := filters[perp()]
	wrong.Step, wrong.MinQty, wrong.Tick = ethStep, ethMin, ethTick
	filters[perp()] = wrong

	p, err := application.NewPlanner(j, r, filters)
	if err != nil {
		// The entry is internally coherent, so construction must admit it;
		// only the leg can expose the disagreement.
		t.Fatalf("NewPlanner: %v", err)
	}

	_, err = p.Plan(cashAndCarryPlan(t))
	if !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Fatalf("Plan over wrong-asset filters = %v, want ErrAssetMismatch", err)
	}
	if errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Error("the wiring mismatch was reported as an off-grid quantity")
	}
	refusedPlanLeavesNothing(t, j, r)
}

// TestConstructionRefusesAnUnusableFiltersEntry: both constructors refuse a
// map holding an entry that could verify nothing, at construction — where the
// message names the wiring — rather than at the first real order it would
// misjudge. The bad entry is the zero step Binance actually publishes
// (MARKET_LOT_SIZE stepSize "0.00000000", observed live 2026-08-03), not an
// invented corruption.
func TestConstructionRefusesAnUnusableFiltersEntry(t *testing.T) {
	t.Parallel()

	broken := func() map[ids.VenueInstrument]domain.TradingFilters {
		filters := testFilters(t)
		bad := filters[perp()]
		bad.Step = fixedpoint.ZeroQuantity("BTC")
		filters[perp()] = bad
		return filters
	}

	if _, err := application.NewPlanner(newFakeJournal(), newFakeOrders(), broken()); !errors.Is(err, domain.ErrFilterStepNotPositive) {
		t.Errorf("NewPlanner over a zero-step entry = %v, want ErrFilterStepNotPositive", err)
	}

	j := newFakeJournal()
	if _, err := application.NewDispatcher(j, &fakePlacer{watch: j}, newFakeOrders(),
		testClock(t), application.NewDispatchAuthority(), broken(), testFilterMaxAge); !errors.Is(err, domain.ErrFilterStepNotPositive) {
		t.Errorf("NewDispatcher over a zero-step entry = %v, want ErrFilterStepNotPositive", err)
	}

	// The positive control for the line above, and NOT a duplicate of the
	// suite's other constructions: this one asserts the entry validation is
	// what refused, rather than the constructor objecting to something else
	// about a fixture assembled by `broken`.
	if _, err := application.NewDispatcher(j, &fakePlacer{watch: j}, newFakeOrders(),
		testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge); err != nil {
		t.Errorf("NewDispatcher over a sound map = %v, want nil", err)
	}
}

// TestExecuteRefusesAReplayedOffGridLegWhole is the second enforcement site's
// whole reason: records that never passed THIS process's Plan — §7.5's replay
// rebuilds exactly such records, and §5.3 says replan-or-discard, never
// dispatch. The legs are planted directly into the repository (the replayed
// shape), share one base quantity (so the package-shape checks all pass), and
// the quantity is on the SPOT grid while off the PERP grid — the asymmetric
// case. The preflight must refuse the package whole: no Place, no
// authorization journaled, both legs untouched.
func TestExecuteRefusesAReplayedOffGridLegWhole(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	// 0.475: on the spot's 0.025 grid (19 lots), off the perp's 0.05 grid
	// (9.5). Identical across legs, so only the filters check can object.
	legSpot := packageOrder(t, 0, "0.475")
	legPerp := packageOrder(t, 1, "0.475")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	_, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), application.UrgencyOpening)
	if !errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Fatalf("Execute of a replayed off-grid package = %v, want ErrQuantityOffLotStep", err)
	}
	if p.calls != 0 {
		t.Errorf("Place was called %d times for a refused package, want 0", p.calls)
	}
	if len(j.calls) != 0 {
		t.Errorf("the journal saw %d appends for a refused package, want 0", len(j.calls))
	}
}

// TestTheRealVenueAsymmetryIsRefusedWhole is the harm case at the observed
// values (profiles §5, 2026-08-03): the Binance spot grid admits 0.00023 BTC
// and the Binance perp grid does not. Sent, that package is one leg filled
// and one leg rejected — naked exposure; planned here, it is refused whole
// with nothing journaled. The conforming package beneath it is the positive
// control, at the same filters, so the refusal above is the check biting and
// not a fixture that refuses everything.
func TestTheRealVenueAsymmetryIsRefusedWhole(t *testing.T) {
	t.Parallel()

	filters := map[ids.VenueInstrument]domain.TradingFilters{
		spot(): {
			Step:        qty(t, "0.00001"),
			MinQty:      qty(t, "0.00001"),
			Tick:        price(t, "0.01"),
			MinNotional: usdt(t, "5"),
			ObservedAt:  testNow.Add(-1 * time.Hour),
		},
		perp(): {
			Step:        qty(t, "0.001"),
			MinQty:      qty(t, "0.001"),
			Tick:        price(t, "0.1"),
			MinNotional: usdt(t, "50"),
			ObservedAt:  testNow.Add(-2 * time.Hour),
		},
	}

	j := newFakeJournal()
	r := newFakeOrders()
	p, err := application.NewPlanner(j, r, filters)
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	plan := cashAndCarryPlan(t)
	for i := range plan.Legs {
		plan.Legs[i].Quantity = qty(t, "0.00023")
		plan.Legs[i].LimitPrice = price(t, "65000")
	}
	if _, err := p.Plan(plan); !errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Fatalf("Plan of the asymmetric package = %v, want ErrQuantityOffLotStep", err)
	}
	refusedPlanLeavesNothing(t, j, r)

	// The positive control: 0.001 BTC at 65000 is on both grids, above both
	// minima, and 65 USDT clears both notional floors.
	for i := range plan.Legs {
		plan.Legs[i].Quantity = qty(t, "0.001")
	}
	if _, err := p.Plan(plan); err != nil {
		t.Fatalf("Plan of the conforming package = %v, want nil", err)
	}
}
