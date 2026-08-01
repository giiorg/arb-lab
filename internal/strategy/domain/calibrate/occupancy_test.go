package calibrate_test

// The commit-3 battery: the surrogate frame, the clearing level, and the
// Enter/Continue occupancy replay.
//
// Checklist shapes, §41i's pre-named suspects first:
//
//   - the DWELL scenario's utilization is 16h/56h = 0.285714285714…, a
//     ratio whose tenth digit is ≥ 5 — RoundFloor answers …285 where
//     half-even answers …286, so "rounded down" is a pinned claim, not a
//     coincidence (the exact-division suspect, answered by construction);
//   - the ELIGIBILITY scenario has exactly ONE clearing instant whose
//     hold spans three settlements — occupancy 0.5 against a clearing
//     fraction of 1/7, the replay-vs-proxy distinction §41b's round-2
//     catch demands;
//   - the clearing boundary is exercised from BOTH sides at one mantissa
//     apart, so a bisection off-by-one moves an assertion.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

var calSpot = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}

func fixtureRef(name string) domain.CalibrationRef {
	return domain.CalibrationRef{
		ID:   domain.FixturePrefix + name,
		Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

func mustQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

func mustPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func frameProvenance(t *testing.T) domain.Provenance {
	t.Helper()
	return domain.Provenance{
		Class:      domain.ProvenanceHandRecorded,
		ObservedAt: calEra.Add(-24 * time.Hour),
		Rev: config.Revision{
			Num: 1, Hash: [32]byte{0xab, 1}, Source: "configs/reference.btcusdt.json",
			LoadedAt: calEra.Add(-24 * time.Hour),
		},
	}
}

// testFrame is the replay battery's surrogate: locked $50,000 tops on
// both legs, 0.02 BTC candidate (lot-aligned), taker fees 0.10%/0.05%,
// $0.50/leg surrogate cost, C_allocated $1,550, a 3.65% hurdle, a
// 15-minute half-life (saturation 7.75h < the 8h grid, so the replay's
// estimate is exactly the just-settled rate — every verdict below is
// hand-derivable), min hold 3 settlements, dwell 2, band 5%.
func testFrame(t *testing.T) calibrate.Surrogate {
	t.Helper()
	prov := frameProvenance(t)
	return calibrate.Surrogate{
		Spot:      calSpot,
		Perp:      calPerp,
		SpotPrice: mustPrice(t, "50000"),
		PerpPrice: mustPrice(t, "50000"),
		Interval:  8 * time.Hour,
		Cap:       mustRate(t, "0.0075"),
		Floor:     mustRate(t, "-0.0075"),
		Params: domain.StrategyParams{
			MinFundingAPR:      mustRate(t, "0.01"),
			MinHoldSettlements: 3,
			ForecastModel:      "ewma.v1",
			EWMAHalfLife:       15 * time.Minute,
			DecisionQuantile:   mustRate(t, "0.25"),
			Aggregation:        domain.AggregationConservative,
			BasisModel:         "conservative.v1",
			EntryDrift:         domain.MeasuredRate{Value: mustRate(t, "0.01"), Artifact: fixtureRef("drift")},
			ExitBand:           domain.MeasuredRate{Value: mustRate(t, "0.05"), Artifact: fixtureRef("band")},
			ExitPersist:        2,
			PerpLeverage:       mustRate(t, "4"),
		},
		PolicyRev: config.Revision{
			Num: 2, Hash: [32]byte{0xcd, 2}, Source: "configs/strategy.json",
			LoadedAt: calEra.Add(-24 * time.Hour),
		},
		Dispersion: domain.Dispersion{
			Points: []domain.QuantilePoint{
				{Q: mustRate(t, "0.25"), Shift: mustRate(t, "-0.00001")},
				{Q: mustRate(t, "0.5"), Shift: mustRate(t, "0")},
			},
			Artifact: fixtureRef("dispersion"),
		},
		AdverseBasis: domain.MeasuredRate{Value: mustRate(t, "0.0001"), Artifact: fixtureRef("basis")},
		SpotFees: domain.LegFees{
			Instrument: calSpot, Maker: mustRate(t, "0.0008"), Taker: mustRate(t, "0.001"),
			Tier: "VIP0", Provenance: prov,
		},
		PerpFees: domain.LegFees{
			Instrument: calPerp, Maker: mustRate(t, "0.0002"), Taker: mustRate(t, "0.0005"),
			Tier: "VIP0", Provenance: prov,
		},
		SpotLots: domain.LotRules{
			Instrument: calSpot, Step: mustQty(t, "0.001"),
			MinQty: mustQty(t, "0.001"), MinNotional: mustMoney(t, "50"), Provenance: prov,
		},
		PerpLots: domain.LotRules{
			Instrument: calPerp, Step: mustQty(t, "0.001"),
			MinQty: mustQty(t, "0.001"), MinNotional: mustMoney(t, "50"), Provenance: prov,
		},
		Hurdle: domain.HurdleBenchmark{
			AnnualRate: mustRate(t, "0.0365"),
			AccessCost: mustRate(t, "0"), LiquidityHaircut: mustRate(t, "0"), CounterpartyHaircut: mustRate(t, "0"),
			AsOf: calEra.Add(-48 * time.Hour), Provenance: prov,
		},
		Capital: domain.CapitalAllocation{
			SpotPrincipal:      mustMoney(t, "1100"),
			PerpInitialMargin:  mustMoney(t, "300"),
			RescueMargin:       domain.MeasuredMoney{Amount: mustMoney(t, "100"), Artifact: fixtureRef("rescue")},
			FeeSlippageReserve: mustMoney(t, "50"),
			CommittedFloat:     mustMoney(t, "0"),
		},
		CandidateQty:      mustQty(t, "0.02"),
		ResidualTolerance: mustQty(t, "0"),
		ExecCost: domain.ExecutionCostEstimate{
			SpotCost: mustMoney(t, "0.5"), PerpCost: mustMoney(t, "0.5"),
			Model: "surrogate.v1", Calibration: fixtureRef("cost"),
			Tactic: domain.TacticTakerIOC, Qty: mustQty(t, "0.02"),
		},
		Opportunity:      mustMoney(t, "0"),
		ClosingCostDelta: mustMoney(t, "0"),
		RiskDelta:        mustMoney(t, "0"),
	}
}

// series builds a settlement history on the frame's 8h grid.
func series(t *testing.T, rates ...string) []domain.RealizedSettlement {
	t.Helper()
	out := make([]domain.RealizedSettlement, len(rates))
	for i, r := range rates {
		out[i] = settled(t, calEra, time.Duration(i)*8*time.Hour, r)
	}
	return out
}

func TestOccupancyChainedHoldsAndTruncation(t *testing.T) {
	sur := testFrame(t)
	// Every settlement clears: enter at the first evaluable instant (8h),
	// close plan-complete at 32h, re-enter the SAME instant, era ends at
	// 48h with the successor open — truncation counts its span.
	res, err := calibrate.Occupancy(series(t,
		"0.003", "0.003", "0.003", "0.003", "0.003", "0.003", "0.003"), sur)
	if err != nil {
		t.Fatalf("Occupancy: %v", err)
	}
	if res.Entries != 2 || res.Closes != 1 {
		t.Errorf("entries %d, closes %d; want 2, 1 (plan-complete close, seamless re-entry)", res.Entries, res.Closes)
	}
	if res.Occupied != 40*time.Hour || res.Deployable != 40*time.Hour {
		t.Errorf("occupied %v over %v; want 40h over 40h", res.Occupied, res.Deployable)
	}
	if res.Utilization.Decimal() != "1" {
		t.Errorf("utilization = %s; want 1", res.Utilization.Decimal())
	}
	if res.Evaluated != 6 {
		t.Errorf("evaluated %d; want 6", res.Evaluated)
	}
}

func TestOccupancyDwellCloseRoundsDown(t *testing.T) {
	sur := testFrame(t)
	// Enter at 8h; funding collapses to zero: the score sits below the
	// band at 16h (dwell 1) and 24h (dwell 2 == persist → close). The
	// fall-through entry at 24h refuses on the funding floor — idle, not
	// an error. Occupied 16h over deployable 56h = 0.285714285714…, whose
	// tenth digit forces RoundFloor and half-even apart.
	res, err := calibrate.Occupancy(series(t,
		"0.003", "0.003", "0", "0", "0", "0", "0", "0", "0"), sur)
	if err != nil {
		t.Fatalf("Occupancy: %v", err)
	}
	if res.Entries != 1 || res.Closes != 1 {
		t.Errorf("entries %d, closes %d; want 1, 1", res.Entries, res.Closes)
	}
	if res.Occupied != 16*time.Hour || res.Deployable != 56*time.Hour {
		t.Errorf("occupied %v over %v; want 16h over 56h", res.Occupied, res.Deployable)
	}
	if res.Utilization.Decimal() != "0.285714285" {
		t.Errorf("utilization = %s; want 0.285714285 (rounded DOWN)", res.Utilization.Decimal())
	}
}

func TestOccupancyExceedsEligibilityProxy(t *testing.T) {
	sur := testFrame(t)
	// Exactly ONE clearing instant (16h) out of seven evaluated; the hold
	// it starts spans three settlements and closes plan-complete at 40h.
	// Occupancy 24h/48h = 0.5 against a clearing fraction of 1/7 — the
	// §41b round-2 catch that a clearing-window proxy is not even
	// conservative, embodied.
	res, err := calibrate.Occupancy(series(t,
		"0.0005", "0.0005", "0.003", "0.0005", "0.0005", "0.0005", "0.0005", "0.0005"), sur)
	if err != nil {
		t.Fatalf("Occupancy: %v", err)
	}
	if res.Entries != 1 || res.Closes != 1 {
		t.Errorf("entries %d, closes %d; want 1, 1", res.Entries, res.Closes)
	}
	if res.Utilization.Decimal() != "0.5" {
		t.Errorf("utilization = %s; want 0.5 — three-plus times the 1/7 clearing fraction", res.Utilization.Decimal())
	}
}

func TestClearingLevelBoundary(t *testing.T) {
	sur := testFrame(t)
	level, err := calibrate.ClearingLevel(sur)
	if err != nil {
		t.Fatalf("ClearingLevel: %v", err)
	}

	// The level itself enters; one mantissa below does not — the replay
	// is the judge, so a bisection off-by-one moves these counts.
	flat := level.Decimal()
	atLevel, err := calibrate.Occupancy(series(t, flat, flat, flat, flat, flat), sur)
	if err != nil {
		t.Fatalf("Occupancy at level: %v", err)
	}
	if atLevel.Entries == 0 {
		t.Errorf("no entry at the clearing level %s", flat)
	}
	below := fixedpoint.RateFromMantissa(level.Mantissa() - 1).Decimal()
	belowLevel, err := calibrate.Occupancy(series(t, below, below, below, below, below), sur)
	if err != nil {
		t.Fatalf("Occupancy below level: %v", err)
	}
	if belowLevel.Entries != 0 {
		t.Errorf("entered %d times one mantissa below the clearing level", belowLevel.Entries)
	}
	if belowLevel.Utilization.Decimal() != "0" {
		t.Errorf("idle utilization = %s; want 0", belowLevel.Utilization.Decimal())
	}
}

func TestClearingLevelEndpoints(t *testing.T) {
	// A hurdle no realizable rate can clear refuses by name.
	sur := testFrame(t)
	sur.Hurdle.AnnualRate = mustRate(t, "5")
	if _, err := calibrate.ClearingLevel(sur); !errors.Is(err, calibrate.ErrNeverClears) {
		t.Errorf("500%% hurdle error = %v; want ErrNeverClears", err)
	}

	// A frame whose floor already clears answers the floor: the funding
	// floor and the hurdle are both driven far negative, so even the
	// venue's worst settlement rate enters.
	sur = testFrame(t)
	sur.Params.MinFundingAPR = mustRate(t, "-9")
	sur.Hurdle.AnnualRate = mustRate(t, "-8")
	level, err := calibrate.ClearingLevel(sur)
	if err != nil {
		t.Fatalf("ClearingLevel: %v", err)
	}
	if level != sur.Floor {
		t.Errorf("level = %s; want the floor %s", level.Decimal(), sur.Floor.Decimal())
	}
}

func TestOccupancyRefusalFrames(t *testing.T) {
	// A capital decomposition that cannot fund the candidate is a frame
	// defect, not an idle era.
	sur := testFrame(t)
	sur.Capital.SpotPrincipal = mustMoney(t, "900")
	if _, err := calibrate.Occupancy(series(t, "0.003", "0.003", "0.003"), sur); !errors.Is(err, calibrate.ErrReplayRefused) {
		t.Errorf("underfunded frame error = %v; want ErrReplayRefused", err)
	}

	// A candidate the quantization moves prices a different trade than
	// the position would hold.
	sur = testFrame(t)
	sur.CandidateQty = mustQty(t, "0.0205")
	sur.ExecCost.Qty = sur.CandidateQty
	sur.ResidualTolerance = mustQty(t, "0.001")
	if _, err := calibrate.Occupancy(series(t, "0.003", "0.003", "0.003"), sur); !errors.Is(err, calibrate.ErrCandidateMisaligned) {
		t.Errorf("misaligned candidate error = %v; want ErrCandidateMisaligned", err)
	}

	// A hole in the settlement grid breaks the replay's world.
	sur = testFrame(t)
	gappy := series(t, "0.003", "0.003", "0.003")
	gappy[2].At = gappy[2].At.Add(8 * time.Hour)
	if _, err := calibrate.Occupancy(gappy, sur); !errors.Is(err, calibrate.ErrGridBroken) {
		t.Errorf("gappy grid error = %v; want ErrGridBroken", err)
	}

	// Too short for the warmup, or nothing after the first evaluable
	// instant: refused, never a zero-over-zero utilization.
	if _, err := calibrate.Occupancy(series(t, "0.003"), sur); !errors.Is(err, calibrate.ErrHistoryShort) {
		t.Errorf("one-row error = %v; want ErrHistoryShort", err)
	}
	if _, err := calibrate.Occupancy(series(t, "0.003", "0.003"), sur); !errors.Is(err, calibrate.ErrHistoryShort) {
		t.Errorf("two-row error = %v; want ErrHistoryShort (first evaluable instant is the last row)", err)
	}
}
