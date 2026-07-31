package driven_test

// The conservative-cost battery. Fixture geometry answers the checklist
// before the mutations do:
//
//   - the two BRANCHES (walk+adverse vs p90) each WIN somewhere — a fixture
//     set where one branch always dominates collapses "worse of" into "the
//     dominant branch";
//   - the two SIDES of each book differ — a symmetric book collapses
//     worse-side selection into "either side";
//   - the two LEGS' cells differ (perp p90 ≠ spot p90) — equal legs collapse
//     the per-instrument cell key into a pooled table (§41i);
//   - the classifier has TWO strata and one test's book lands in the second
//     — a single-stratum fixture collapses stratum selection into the
//     quantity-only table the strata exist to replace (§41i);
//   - the p90 bucket boundary is exercised exactly AT MaxQty, where `<=`
//     and `<` are different programs;
//   - every cell's Observations sits exactly AT MinEvidence, where `<` and
//     `<=` in the evidence check are different programs (§41i);
//   - the latency comparison is exercised exactly AT the calibrated value,
//     same reason.

import (
	"errors"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

const fixtureMethod = "walk_t_plus_latency_v1"

// fixtureClassifierSet: two strata split on relative spread at 0.0001. The
// flat fixture books all land in "normal"; the stratum test's widened book
// lands in "wide".
func fixtureClassifierSet(t *testing.T) domain.StratumClassifier {
	t.Helper()
	unboundedDepth := domain.QuantityInterval{
		Lo: domain.QuantityBound{Kind: domain.BoundExclusive, Value: mustQty(t, "0")},
		Hi: domain.QuantityBound{Kind: domain.BoundUnbounded},
	}
	return domain.StratumClassifier{Rules: []domain.StratumRule{
		{
			Name: "normal",
			Spread: domain.RateInterval{
				Lo: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "0")},
				Hi: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "0.0001")},
			},
			Depth: unboundedDepth,
		},
		{
			Name: "wide",
			Spread: domain.RateInterval{
				Lo: domain.RateBound{Kind: domain.BoundExclusive, Value: mustRate(t, "0.0001")},
				Hi: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "1")},
			},
			Depth: unboundedDepth,
		},
	}}
}

// cellsFor builds one (instrument, side) group's cells in the named stratum.
func cellsFor(t *testing.T, inst ids.VenueInstrument, side mdomain.BookSide, stratum string, rows [][2]string) []driven.CostCell {
	t.Helper()
	cells := make([]driven.CostCell, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, driven.CostCell{
			Instrument:   inst,
			Side:         side,
			Stratum:      stratum,
			MaxQty:       mustQty(t, r[0]),
			P90:          mustRate(t, r[1]),
			Observations: 30, // exactly MinEvidence — at-vs-above is observable
			Method:       fixtureMethod,
		})
	}
	return cells
}

// fixtureCalibrationSet: the "normal" table. Spot and perp deliberately
// carry DIFFERENT p90s, so a lookup that pools across legs moves the
// answer. Cell order interleaves the groups (all small buckets, then all
// large), which is legal — order is per-group — and a mutant comparing
// adjacent cells globally refuses this valid document.
func fixtureCalibrationSet(t *testing.T) driven.CostCalibration {
	t.Helper()
	small := append(
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "normal", [][2]string{{"0.05", "0.001"}}),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "normal", [][2]string{{"0.05", "0.001"}})...),
		append(cellsFor(t, mPerpInst, mdomain.SideBid, "normal", [][2]string{{"0.05", "0.0015"}}),
			cellsFor(t, mPerpInst, mdomain.SideAsk, "normal", [][2]string{{"0.05", "0.0015"}})...)...)
	large := append(
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "normal", [][2]string{{"0.5", "0.0002"}}),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "normal", [][2]string{{"0.5", "0.0002"}})...),
		append(cellsFor(t, mPerpInst, mdomain.SideBid, "normal", [][2]string{{"0.5", "0.0003"}}),
			cellsFor(t, mPerpInst, mdomain.SideAsk, "normal", [][2]string{{"0.5", "0.0003"}})...)...)
	return driven.CostCalibration{
		Artifact:    modelCalibration("slippage"),
		AdverseMove: mustRate(t, "0.0001"),
		Latency:     250 * time.Millisecond,
		Classifier:  fixtureClassifierSet(t),
		MinEvidence: 30,
		Cells:       append(small, large...),
	}
}

// flatBooks: gentle ladders where the p90 branch dominates on every side
// and every relative spread is far inside the "normal" stratum.
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
// prices from ITS OWN cells (p90 0.0003): ask side 9601.875·0.0003 =
// 2.8805625 vs walk 0.9851875 → p90 wins; bid 9601.80·0.0003 = 2.88054 →
// ask worse. A lookup pooling the legs would print spot's 0.0002 here.
func TestEstimateCost_P90DominantSides(t *testing.T) {
	est, err := fixtureModel(t, flatBooks(t)).EstimateCost(request(t, "0.15"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "1.920015"; got != want {
		t.Errorf("SpotCost = %s; want %s", got, want)
	}
	if got, want := est.PerpCost.Decimal(), "2.8805625"; got != want {
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

// A widened spot book (best ask 64100 over best bid 64000: relative spread
// ≈ 0.00156) classifies into "wide", whose cells carry p90 0.002 — ten
// times "normal"'s large bucket. Spot ask: 641.00·0.002 = 1.282 (adverse
// 0.0641 keeps the walk branch behind); bid 640.00·0.002 = 1.28 → ask
// worse. A model that never classifies — or classifies and ignores the
// stratum in the key — prices this book from "normal" and prints 0.1282.
// The perp book stays normal and prices from perp's own small bucket:
// 640.125·0.0015 = 0.9601875.
func TestEstimateCost_StratumSelectsTheCell(t *testing.T) {
	cal := fixtureCalibrationSet(t)
	cal.Cells = append(cal.Cells,
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "wide", [][2]string{{"0.5", "0.002"}}),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "wide", [][2]string{{"0.5", "0.002"}})...)...)
	books := flatBooks(t)
	books[mSpotInst] = bookFor(t, mSpotInst,
		[][2]string{{"64000.00", "0.30"}, {"63999.00", "0.30"}},
		[][2]string{{"64100.00", "0.10"}, {"64101.00", "0.20"}},
	)
	m, err := driven.NewConservativeCostModel(cal, books)
	if err != nil {
		t.Fatalf("NewConservativeCostModel: %v", err)
	}
	est, err := m.EstimateCost(request(t, "0.01"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if got, want := est.SpotCost.Decimal(), "1.282"; got != want {
		t.Errorf("SpotCost = %s; want %s (the wide stratum's p90)", got, want)
	}
	if got, want := est.PerpCost.Decimal(), "0.9601875"; got != want {
		t.Errorf("PerpCost = %s; want %s (perp's own normal cell)", got, want)
	}
}

// A book state the artifact measured no cells for is a refusal naming the
// population — never the nearest cell. The base table has no "wide" cells,
// and the widened book classifies there.
func TestEstimateCost_UncalibratedStratumRefuses(t *testing.T) {
	books := flatBooks(t)
	books[mSpotInst] = bookFor(t, mSpotInst,
		[][2]string{{"64000.00", "0.30"}, {"63999.00", "0.30"}},
		[][2]string{{"64100.00", "0.10"}, {"64101.00", "0.20"}},
	)
	if _, err := fixtureModel(t, books).EstimateCost(request(t, "0.01")); !errors.Is(err, driven.ErrStratumUncalibrated) {
		t.Errorf("wide book on a normal-only table = %v; want ErrStratumUncalibrated", err)
	}
}

// The SIDE is part of the cell key: a table holding only ask-side spot
// cells refuses the bid-side lookup rather than answering from the ask's
// population.
func TestEstimateCost_SideIsPartOfTheKey(t *testing.T) {
	cal := fixtureCalibrationSet(t)
	kept := cal.Cells[:0]
	for _, cell := range cal.Cells {
		if cell.Instrument == mSpotInst && cell.Side == mdomain.SideBid {
			continue
		}
		kept = append(kept, cell)
	}
	cal.Cells = kept
	m, err := driven.NewConservativeCostModel(cal, flatBooks(t))
	if err != nil {
		t.Fatalf("NewConservativeCostModel: %v", err)
	}
	if _, err := m.EstimateCost(request(t, "0.15")); !errors.Is(err, driven.ErrStratumUncalibrated) {
		t.Errorf("bid lookup on an ask-only table = %v; want ErrStratumUncalibrated", err)
	}
}

// The spread ratio ROUNDS AWAY FROM ZERO, and that direction is what puts
// a boundary-straddling book in the wider-spread stratum. Bid 64000.00,
// ask 64000.50: spread/mid = 0.5/64000.25 = 0.00000781216…, which is
// 0.000007813 away-from-zero and 0.000007812 toward-zero at scale 9. A
// classifier cut INCLUSIVELY at 0.000007812 therefore separates the two
// directions: away lands "wide", toward would land "normal" — and the two
// strata carry different p90s, so the direction is a printed digit.
func TestEstimateCost_SpreadRoundsTowardTheWiderStratum(t *testing.T) {
	cal := fixtureCalibrationSet(t)
	unboundedDepth := domain.QuantityInterval{
		Lo: domain.QuantityBound{Kind: domain.BoundExclusive, Value: mustQty(t, "0")},
		Hi: domain.QuantityBound{Kind: domain.BoundUnbounded},
	}
	cal.Classifier = domain.StratumClassifier{Rules: []domain.StratumRule{
		{
			Name: "normal",
			Spread: domain.RateInterval{
				Lo: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "0")},
				Hi: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "0.000007812")},
			},
			Depth: unboundedDepth,
		},
		{
			Name: "wide",
			Spread: domain.RateInterval{
				Lo: domain.RateBound{Kind: domain.BoundExclusive, Value: mustRate(t, "0.000007812")},
				Hi: domain.RateBound{Kind: domain.BoundInclusive, Value: mustRate(t, "1")},
			},
			Depth: unboundedDepth,
		},
	}}
	normalRows := [][2]string{{"0.5", "0.0002"}}
	wideRows := [][2]string{{"0.5", "0.002"}}
	cal.Cells = append(
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "normal", normalRows),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "normal", normalRows)...),
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "wide", wideRows),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "wide", wideRows)...)...)
	cal.Cells = append(cal.Cells,
		append(cellsFor(t, mPerpInst, mdomain.SideBid, "normal", normalRows),
			cellsFor(t, mPerpInst, mdomain.SideAsk, "normal", normalRows)...)...)
	cal.AdverseMove = mustRate(t, "0")

	books := flatBooks(t)
	books[mSpotInst] = bookFor(t, mSpotInst,
		[][2]string{{"64000.00", "0.30"}},
		[][2]string{{"64000.50", "0.30"}},
	)
	m, err := driven.NewConservativeCostModel(cal, books)
	if err != nil {
		t.Fatalf("NewConservativeCostModel: %v", err)
	}
	est, err := m.EstimateCost(request(t, "0.01"))
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	// Wide's p90 on the ask side: 640.005 · 0.002 = 1.28001. The
	// toward-zero direction would classify "normal" and print 0.128001.
	if got, want := est.SpotCost.Decimal(), "1.28001"; got != want {
		t.Errorf("SpotCost = %s; want %s (the wide stratum via away-from-zero)", got, want)
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
	tiny := [][2]string{{"0.5", "0.000000123"}}
	cal.Cells = append(
		append(cellsFor(t, mSpotInst, mdomain.SideBid, "normal", tiny),
			cellsFor(t, mSpotInst, mdomain.SideAsk, "normal", tiny)...),
		append(cellsFor(t, mPerpInst, mdomain.SideBid, "normal", tiny),
			cellsFor(t, mPerpInst, mdomain.SideAsk, "normal", tiny)...)...)
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
		{"no classifier", func(c *driven.CostCalibration) { c.Classifier = domain.StratumClassifier{} }, domain.ErrUnset},
		{"zero min evidence", func(c *driven.CostCalibration) { c.MinEvidence = 0 }, domain.ErrUnset},
		{"no cells", func(c *driven.CostCalibration) { c.Cells = nil }, domain.ErrUnset},
		{"negative p90", func(c *driven.CostCalibration) { c.Cells[1].P90 = mustRate(t, "-0.0002") }, driven.ErrChargeNegative},
		{"zero cell qty", func(c *driven.CostCalibration) { c.Cells[0].MaxQty = mustQty(t, "0") }, domain.ErrUnset},
		{"cell stratum unknown", func(c *driven.CostCalibration) { c.Cells[0].Stratum = "vanished" }, driven.ErrCellStratumUnknown},
		{"cell method unset", func(c *driven.CostCalibration) { c.Cells[0].Method = "" }, domain.ErrUnset},
		// One observation BELOW the declared minimum: the cell should not
		// exist (§41b-ii-iii). Every other cell sits exactly AT the minimum,
		// so an at-vs-above mutant refuses the whole valid fixture.
		{"cell thin", func(c *driven.CostCalibration) { c.Cells[0].Observations = 29 }, driven.ErrCellThin},
		{"group out of order", func(c *driven.CostCalibration) {
			// Cells 1 (spot ask 0.05) and 5 (spot ask 0.5) share a group;
			// swapping their MaxQtys puts 0.5 before 0.05 within it.
			c.Cells[1].MaxQty, c.Cells[5].MaxQty = c.Cells[5].MaxQty, c.Cells[1].MaxQty
		}, driven.ErrBucketOrder},
		{"group duplicate qty", func(c *driven.CostCalibration) {
			c.Cells[5].MaxQty = c.Cells[1].MaxQty
		}, driven.ErrBucketOrder},
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

	// Positive control: the unbroken set constructs — including its
	// deliberately interleaved cross-group cell order, which a mutant
	// comparing ADJACENT cells globally refuses.
	if _, err := driven.NewConservativeCostModel(base, flatBooks(t)); err != nil {
		t.Fatalf("valid calibration refused: %v", err)
	}

	// A nil book source is a wiring hole, not an empty market.
	var nilBooks driven.BookSource
	if _, err := driven.NewConservativeCostModel(base, nilBooks); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("nil books = %v; want ErrUnset", err)
	}
}
