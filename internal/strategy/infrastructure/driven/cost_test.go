package driven_test

// The conservative-cost battery. Fixture geometry answers the checklist
// before the mutations do:
//
//   - the two BRANCHES (walk+adverse vs p90) each WIN somewhere — a fixture
//     set where one branch always dominates collapses "worse of" into "the
//     dominant branch";
//   - the two SIDES of each book differ — a symmetric book collapses
//     worse-side selection into "either side";
//   - the p90 bucket boundary is exercised exactly AT MaxQty, where `<=`
//     and `<` are different programs;
//   - the latency comparison is exercised exactly AT the calibrated value,
//     same reason.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

func fixtureCalibrationSet(t *testing.T) driven.CostCalibration {
	t.Helper()
	return driven.CostCalibration{
		Artifact:    modelCalibration("slippage"),
		AdverseMove: mustRate(t, "0.0001"),
		Latency:     250 * time.Millisecond,
		Buckets: []driven.CostBucket{
			{MaxQty: mustQty(t, "0.05"), P90: mustRate(t, "0.001")},
			{MaxQty: mustQty(t, "0.5"), P90: mustRate(t, "0.0002")},
		},
	}
}

// flatBooks: gentle ladders where the p90 branch dominates on every side.
func flatBooks(t *testing.T) mapBooks {
	t.Helper()
	return mapBooks{
		mSpotInst: bookFor(t, mSpotInst,
			[][2]string{{"64000.00", "0.30"}, {"63999.00", "0.30"}},
			[][2]string{{"64000.50", "0.10"}, {"64001.00", "0.20"}},
		),
		mPerpInst: bookFor(t, mPerpInst,
			[][2]string{{"64012.00", "0.30"}, {"64010.00", "0.30"}},
			[][2]string{{"64012.50", "0.10"}, {"64013.00", "0.20"}},
		),
	}
}

func fixtureModel(t *testing.T, books mapBooks) *driven.ConservativeCostModel {
	t.Helper()
	m, err := driven.NewConservativeCostModel(fixtureCalibrationSet(t), books)
	if err != nil {
		t.Fatalf("NewConservativeCostModel: %v", err)
	}
	return m
}

func request(t *testing.T, qty string) domain.ExecutionCostRequest {
	t.Helper()
	return domain.ExecutionCostRequest{
		Spot:    mSpotInst,
		Perp:    mPerpInst,
		Qty:     mustQty(t, qty),
		Tactic:  domain.TacticTakerIOC,
		Latency: 200 * time.Millisecond,
	}
}

// Hand-derived, exact, p90-dominant. Spot ask side: walk premium 0.025
// (0.10@64000.50 + 0.05@64001.00 against best·0.15 = 9600.075) + adverse
// 0.9600075 = 0.9850075, vs p90 9600.075·0.0002 = 1.920015 → p90 wins; bid
// side walks free (premium 0) and lands 1.92 → the ASK side is worse. Perp
// mirrors at its own prices → 1.920375 vs 1.92036.
func TestEstimateCost_P90DominantSides(t *testing.T) {
	est, err := fixtureModel(t, flatBooks(t)).EstimateCost(request(t, "0.15"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "1.920015"; got != want {
		t.Errorf("SpotCost = %s; want %s", got, want)
	}
	if got, want := est.PerpCost.Decimal(), "1.920375"; got != want {
		t.Errorf("PerpCost = %s; want %s", got, want)
	}

	// The estimate names everything it was produced from.
	if got, want := est.Model, "conservative_bound"; got != want {
		t.Errorf("Model = %q; want %q", got, want)
	}
	if got, want := est.Calibration, modelCalibration("slippage"); got != want {
		t.Errorf("Calibration = %+v; want %+v", got, want)
	}
	if got, want := est.Tactic, domain.TacticTakerIOC; got != want {
		t.Errorf("Tactic = %s; want %s", got, want)
	}
	if c, err := est.Qty.Cmp(mustQty(t, "0.15")); err != nil || c != 0 {
		t.Errorf("Qty = %s; want 0.15 (cmp %d, %v)", est.Qty, c, err)
	}
	if err := est.Validate(); err != nil {
		t.Errorf("estimate does not validate: %v", err)
	}
}

// A steep ask ladder makes the WALK branch win on the ask side while the
// bid side stays p90-bound: spot ask walk = premium 49.95 (the second level
// sits 499.50 away) + adverse 0.9600075 = 50.9100075, far above its p90
// 1.920015; the bid side answers 1.92 — so the ask side's WALK number is
// the leg cost. A min-over-sides mutant answers 1.92; a p90-only mutant
// answers 1.920015; both move this assertion.
func TestEstimateCost_WalkDominantAskSide(t *testing.T) {
	books := flatBooks(t)
	books[mSpotInst] = bookFor(t, mSpotInst,
		[][2]string{{"64000.00", "0.30"}, {"63999.00", "0.30"}},
		[][2]string{{"64000.50", "0.05"}, {"64500.00", "0.20"}},
	)
	est, err := fixtureModel(t, books).EstimateCost(request(t, "0.15"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "50.9100075"; got != want {
		t.Errorf("SpotCost = %s; want %s", got, want)
	}
}

// Exactly AT a bucket's MaxQty the bucket still answers — the bound is
// inclusive. 0.05 in the first bucket prices at p90 0.001: spot ask side
// 3200.025·0.001 = 3.200025; through the second bucket it would read
// 0.640005, so an exclusive-bound mutant is a visible digit change.
func TestEstimateCost_BucketBoundaryIsInclusive(t *testing.T) {
	est, err := fixtureModel(t, flatBooks(t)).EstimateCost(request(t, "0.05"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "3.200025"; got != want {
		t.Errorf("SpotCost = %s; want %s (the first bucket's p90)", got, want)
	}
}

// Every product in the fixtures above is EXACT at scale 9 (2-dp prices,
// round quantities), so no assertion there can see a rounding mode. This
// fixture manufactures a 12-decimal product on an otherwise flat, free
// market: qty 0.01 inside every best level (premiums zero), adverse move a
// measured zero, p90 0.000000123 → the spot ask charge is
// 640.005 · 0.000000123 = 0.00007872061 5, whose away-from-zero rounding is
// the visible last digit. Toward-zero or half-even would print ...720.
func TestEstimateCost_ChargesRoundAwayFromZero(t *testing.T) {
	cal := fixtureCalibrationSet(t)
	cal.AdverseMove = mustRate(t, "0")
	cal.Buckets = []driven.CostBucket{{MaxQty: mustQty(t, "0.5"), P90: mustRate(t, "0.000000123")}}
	m, err := driven.NewConservativeCostModel(cal, flatBooks(t))
	if err != nil {
		t.Fatalf("NewConservativeCostModel: %v", err)
	}
	est, err := m.EstimateCost(request(t, "0.01"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "0.000078721"; got != want {
		t.Errorf("SpotCost = %s; want %s (rounded away from zero)", got, want)
	}
}

func TestEstimateCost_LatencyBoundary(t *testing.T) {
	m := fixtureModel(t, flatBooks(t))

	// Exactly the calibrated window: admissible.
	req := request(t, "0.15")
	req.Latency = 250 * time.Millisecond
	if _, err := m.EstimateCost(req); err != nil {
		t.Errorf("latency == calibrated refused: %v", err)
	}

	// One tick past it: the artifact has no number for a longer window.
	req.Latency = 250*time.Millisecond + time.Nanosecond
	if _, err := m.EstimateCost(req); !errors.Is(err, driven.ErrLatencyOutsideCalibration) {
		t.Errorf("latency past calibrated = %v; want ErrLatencyOutsideCalibration", err)
	}
}

func TestEstimateCost_Refusals(t *testing.T) {
	m := fixtureModel(t, flatBooks(t))

	// The maker-entry fork is a documented opt-in with no calibration here.
	req := request(t, "0.15")
	req.Tactic = domain.TacticMakerEntry
	if _, err := m.EstimateCost(req); !errors.Is(err, driven.ErrTacticUncalibrated) {
		t.Errorf("maker tactic = %v; want ErrTacticUncalibrated", err)
	}

	// A size past the last bucket was never measured.
	if _, err := m.EstimateCost(request(t, "0.6")); !errors.Is(err, driven.ErrSizeOutsideCalibration) {
		t.Errorf("oversize = %v; want ErrSizeOutsideCalibration", err)
	}

	// Depth too thin to fill — the conservative answer is a refusal, not a
	// large number.
	if _, err := m.EstimateCost(request(t, "0.31")); !errors.Is(err, driven.ErrDepthInsufficient) {
		t.Errorf("thin depth = %v; want ErrDepthInsufficient", err)
	}

	// An empty side cannot be priced at all.
	oneSided := flatBooks(t)
	oneSided[mSpotInst] = bookFor(t, mSpotInst,
		nil,
		[][2]string{{"64000.50", "0.10"}, {"64001.00", "0.20"}},
	)
	if _, err := fixtureModel(t, oneSided).EstimateCost(request(t, "0.15")); !errors.Is(err, driven.ErrDepthInsufficient) {
		t.Errorf("empty bid side = %v; want ErrDepthInsufficient", err)
	}

	// A leg with no book at all.
	spotOnly := mapBooks{mSpotInst: flatBooks(t)[mSpotInst]}
	if _, err := fixtureModel(t, spotOnly).EstimateCost(request(t, "0.15")); !errors.Is(err, driven.ErrBookUnavailable) {
		t.Errorf("missing perp book = %v; want ErrBookUnavailable", err)
	}

	// A source answering with the WRONG instrument's book is an assembly
	// fault, not a price.
	crossed := flatBooks(t)
	wrong := crossed[mPerpInst]
	crossed[mSpotInst] = wrong
	if _, err := fixtureModel(t, crossed).EstimateCost(request(t, "0.15")); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Errorf("swapped book = %v; want ErrLegsMismatch", err)
	}

	// A malformed request refuses before any book is read.
	if _, err := m.EstimateCost(domain.ExecutionCostRequest{}); err == nil {
		t.Error("zero request priced")
	}

	// The zero adapter is unusable.
	var zero driven.ConservativeCostModel
	if _, err := zero.EstimateCost(request(t, "0.15")); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero adapter = %v; want ErrUnset", err)
	}
}

func TestCostCalibration_Refusals(t *testing.T) {
	base := fixtureCalibrationSet(t)

	cases := []struct {
		name    string
		corrupt func(*driven.CostCalibration)
		want    error
	}{
		{"no artifact", func(c *driven.CostCalibration) { c.Artifact = domain.CalibrationRef{} }, domain.ErrUnset},
		{"negative adverse", func(c *driven.CostCalibration) { c.AdverseMove = mustRate(t, "-0.0001") }, driven.ErrChargeNegative},
		{"zero latency", func(c *driven.CostCalibration) { c.Latency = 0 }, domain.ErrUnset},
		{"no buckets", func(c *driven.CostCalibration) { c.Buckets = nil }, domain.ErrUnset},
		{"negative p90", func(c *driven.CostCalibration) { c.Buckets[1].P90 = mustRate(t, "-0.0002") }, driven.ErrChargeNegative},
		{"zero bucket qty", func(c *driven.CostCalibration) { c.Buckets[0].MaxQty = mustQty(t, "0") }, domain.ErrUnset},
		{"buckets out of order", func(c *driven.CostCalibration) {
			c.Buckets[0].MaxQty, c.Buckets[1].MaxQty = c.Buckets[1].MaxQty, c.Buckets[0].MaxQty
		}, driven.ErrBucketOrder},
		{"buckets duplicated", func(c *driven.CostCalibration) { c.Buckets[1].MaxQty = c.Buckets[0].MaxQty }, driven.ErrBucketOrder},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cal := fixtureCalibrationSet(t)
			tc.corrupt(&cal)
			if _, err := driven.NewConservativeCostModel(cal, flatBooks(t)); !errors.Is(err, tc.want) {
				t.Fatalf("constructor = %v; want %v", err, tc.want)
			}
		})
	}

	// Positive control: the unbroken set constructs.
	if _, err := driven.NewConservativeCostModel(base, flatBooks(t)); err != nil {
		t.Fatalf("valid calibration refused: %v", err)
	}

	// A nil book source is a wiring hole, not an empty market.
	var nilBooks driven.BookSource
	if _, err := driven.NewConservativeCostModel(base, nilBooks); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("nil books = %v; want ErrUnset", err)
	}
}
