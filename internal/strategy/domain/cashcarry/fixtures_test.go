package cashcarry_test

// Shared fixtures for the cashcarry core battery.
//
// Two families, each earning its shape against the fixture-collapse
// checklist:
//
//   - The FLAT family is tutorial §3.9's worked trade verbatim: BTC flat at
//     $100,000 (bid == ask on both legs, deliberately — the ledger's own
//     shape, so the pinned figures reproduce to the digit), zero-shift
//     dispersion (the §3.9 ledger has no dispersion line; the checklist
//     knows a zero-width dispersion collapses quantile into mean, so every
//     claim about the DRAW is made on the asymmetric family, never here),
//     and a zero entry-drift margin (declared, with artifact — §3.9 has no
//     margin line; margin claims live in the boundary tests that set it).
//   - The ASYMMETRIC family carries the domain battery's shapes: distinct
//     spot/perp fees, a real spread on both books, an odd candidate, so a
//     swapped leg, a wrong book side or a wrong fee column changes an
//     assertion rather than nothing.

import (
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

func mustQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func mustPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func fixtureCalibration(name string) domain.CalibrationRef {
	return domain.CalibrationRef{
		ID:   domain.FixturePrefix + name,
		Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

var (
	baseAt = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	spotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

func watermark(at time.Time, seq uint64) mdomain.Watermark {
	return mdomain.Watermark{VenueSeq: seq, SourceTime: at.Add(-40 * time.Millisecond), RecvTime: at}
}

func revision(num uint64, source string) config.Revision {
	return config.Revision{
		Num:      num,
		Hash:     [32]byte{0xab, 0xcd, 3, byte(num)},
		Source:   source,
		LoadedAt: baseAt.Add(-time.Hour),
	}
}

func provenance() domain.Provenance {
	return domain.Provenance{
		Class:      domain.ProvenanceHandRecorded,
		ObservedAt: baseAt.Add(-2 * time.Hour),
		Rev:        revision(1, "configs/reference.btcusdt.json"),
	}
}

// flatSchedule is §3.9's plan: 60 settlements, 8h apart, first at +8h,
// terminal at the 60th (+480h = twenty days).
func flatSchedule() domain.HoldPlan {
	s := make([]time.Time, 60)
	for n := range s {
		s[n] = baseAt.Add(time.Duration(n+1) * 8 * time.Hour)
	}
	return domain.HoldPlan{Settlements: s, Terminal: s[59]}
}

func flatPolicy(t *testing.T) domain.PolicyProjection {
	t.Helper()
	return domain.PolicyProjection{
		Params: domain.StrategyParams{
			MinFundingAPR:      mustRate(t, "0.05"),
			MinHoldSettlements: 3,
			ForecastModel:      "ewma.v1",
			EWMAHalfLife:       72 * time.Hour,
			DecisionQuantile:   mustRate(t, "0.25"),
			Aggregation:        domain.AggregationConservative,
			BasisModel:         "conservative.v1",
			// §3.9 has no margin line: a DECLARED zero drift, artifact
			// attached. Margin claims use withDrift below.
			EntryDrift:   domain.MeasuredRate{Value: mustRate(t, "0"), Artifact: fixtureCalibration("drift")},
			ExitBand:     domain.MeasuredRate{Value: mustRate(t, "0.01"), Artifact: fixtureCalibration("band")},
			ExitPersist:  2,
			PerpLeverage: mustRate(t, "3"),
		},
		Rev: revision(4, "configs/strategy.json"),
	}
}

func flatView(t *testing.T, realized []domain.RealizedSettlement) domain.MarketView {
	t.Helper()
	flat := mustPrice(t, "100000")
	return domain.MarketView{
		Spot: domain.PriceView{Instrument: spotInst, Bid: flat, Ask: flat, Watermark: watermark(baseAt, 41)},
		Perp: domain.PriceView{Instrument: perpInst, Bid: flat, Ask: flat, Watermark: watermark(baseAt, 43)},
		Funding: mdomain.FundingObservation{
			Instrument: perpInst,
			Rate:       mustRate(t, "0.00015"),
			Interval:   8 * time.Hour,
			NextSettle: baseAt.Add(8 * time.Hour),
			Cap:        mustRate(t, "0.0075"),
			Floor:      mustRate(t, "-0.0075"),
			Watermark:  watermark(baseAt.Add(-500*time.Millisecond), 77),
		},
		Realized: realized,
	}
}

func flatForecasts(t *testing.T, horizon []time.Time, rate string) []domain.SettlementForecast {
	t.Helper()
	out := make([]domain.SettlementForecast, len(horizon))
	for n, at := range horizon {
		out[n] = domain.SettlementForecast{
			At:         at,
			Expected:   mustRate(t, rate),
			Adverse:    mustRate(t, rate),
			Quantile:   mustRate(t, "0.25"),
			Dispersion: fixtureCalibration("dispersion"),
		}
	}
	return out
}

// flatEntry is tutorial §3.9's trade as a DecisionInput: $10,000 notional at
// 0.1 BTC, VIP0 taker (spot 0.10%, perp 0.05%), $2/leg slippage each way,
// no basis line, C_allocated = $16,706.66, 4.6% hurdle, 60 settlements at
// +0.015%/8h.
func flatEntry(t *testing.T) domain.DecisionInput {
	t.Helper()
	plan := flatSchedule()
	// Two realized settlements pre-entry so the window is never the
	// single-element shape the checklist warns about.
	realized := []domain.RealizedSettlement{
		{Instrument: perpInst, At: baseAt.Add(-16 * time.Hour), Rate: mustRate(t, "0.00014"), Watermark: watermark(baseAt.Add(-16*time.Hour), 11)},
		{Instrument: perpInst, At: baseAt.Add(-8 * time.Hour), Rate: mustRate(t, "0.00016"), Watermark: watermark(baseAt.Add(-8*time.Hour), 23)},
	}
	return domain.DecisionInput{
		Kind:      domain.KindEntry,
		At:        baseAt,
		View:      flatView(t, realized),
		Policy:    flatPolicy(t),
		Forecasts: flatForecasts(t, plan.Settlements, "0.00015"),
		Basis: domain.BasisTerm{
			FavorableConvergence: fixedpoint.ZeroMoney("USDT"),
			AdverseMove:          domain.MeasuredMoney{Amount: fixedpoint.ZeroMoney("USDT"), Artifact: fixtureCalibration("basis")},
		},
		SpotFees: domain.LegFees{Instrument: spotInst, Maker: mustRate(t, "0.0008"), Taker: mustRate(t, "0.001"), Tier: "VIP0", Provenance: provenance()},
		PerpFees: domain.LegFees{Instrument: perpInst, Maker: mustRate(t, "0.0002"), Taker: mustRate(t, "0.0005"), Tier: "VIP0", Provenance: provenance()},
		SpotLots: domain.LotRules{
			Instrument: spotInst, Step: mustQty(t, "0.00001"),
			MinQty: mustQty(t, "0.00001"), MinNotional: mustMoney(t, "5"),
			Provenance: provenance(),
		},
		PerpLots: domain.LotRules{
			Instrument: perpInst, Step: mustQty(t, "0.001"),
			MinQty: mustQty(t, "0.001"), MinNotional: mustMoney(t, "100"),
			Provenance: provenance(),
		},
		ResidualTolerance: mustQty(t, "0"),
		CandidateQty:      mustQty(t, "0.1"),
		ExecCost: domain.ExecutionCostEstimate{
			SpotCost:    mustMoney(t, "2"),
			PerpCost:    mustMoney(t, "2"),
			Model:       "conservative-bound.v1",
			Calibration: fixtureCalibration("slippage"),
			Tactic:      domain.TacticTakerIOC,
			Qty:         mustQty(t, "0.1"),
		},
		Hurdle: domain.HurdleBenchmark{
			AnnualRate:          mustRate(t, "0.046"),
			AccessCost:          mustRate(t, "0"),
			LiquidityHaircut:    mustRate(t, "0"),
			CounterpartyHaircut: mustRate(t, "0"),
			AsOf:                baseAt.Add(-26 * time.Hour),
			Provenance:          provenance(),
		},
		Capital: domain.CapitalAllocation{
			// The margin is one cent over N/3 on purpose: at exactly
			// 3333.33 the 3× leverage supports 9999.99 — one cent short of
			// the $10,000 perp leg — and the capital gate correctly refuses
			// §3.9's own trade. The worked example's "≈ $3,333" survives;
			// the boundary is the capital test's to pin.
			SpotPrincipal:      mustMoney(t, "10000"),
			PerpInitialMargin:  mustMoney(t, "3333.34"),
			RescueMargin:       domain.MeasuredMoney{Amount: mustMoney(t, "3333.33"), Artifact: fixtureCalibration("rescue")},
			FeeSlippageReserve: mustMoney(t, "40"),
			CommittedFloat:     fixedpoint.ZeroMoney("USDT"),
		},
		Opportunity:      fixedpoint.ZeroMoney("USDT"),
		ClosingCostDelta: fixedpoint.ZeroMoney("USDT"),
		RiskDelta:        fixedpoint.ZeroMoney("USDT"),
		Plan:             domain.HoldPlanRevision{Num: 1, Trigger: domain.TriggerEntry, At: baseAt, Plan: plan},
	}
}

// exitAt sits strictly between the 58th (+464h) and 59th (+472h)
// settlements: 15 hours of hold remain, two settlements forecastable.
var exitAt = baseAt.Add(465 * time.Hour)

// flatExit is the held §3.9 trade evaluated near its end: two settlements
// remain, the realized window's newest entry is the just-passed 58th
// settlement, and the fed-back dwell last counted the 57th.
func flatExit(t *testing.T) domain.DecisionInput {
	t.Helper()
	in := flatEntry(t)
	plan := flatSchedule()
	in.Kind = domain.KindExit
	in.At = exitAt

	realized := []domain.RealizedSettlement{
		{Instrument: perpInst, At: baseAt.Add(456 * time.Hour), Rate: mustRate(t, "0.00013"), Watermark: watermark(baseAt.Add(456*time.Hour), 210)},
		{Instrument: perpInst, At: baseAt.Add(464 * time.Hour), Rate: mustRate(t, "0.00011"), Watermark: watermark(baseAt.Add(464*time.Hour), 219)},
	}
	in.View = flatView(t, realized)
	in.View.Funding.NextSettle = baseAt.Add(472 * time.Hour)
	in.View.Funding.Watermark = watermark(exitAt.Add(-500*time.Millisecond), 230)
	in.View.Spot.Watermark = watermark(exitAt, 311)
	in.View.Perp.Watermark = watermark(exitAt, 313)

	in.Plan = domain.HoldPlanRevision{Num: 1, Trigger: domain.TriggerEntry, At: baseAt, Plan: plan}
	in.Forecasts = flatForecasts(t, plan.Settlements[58:], "0.0001")
	// The expected path diverges from the adverse draw (0.0002 vs 0.0001):
	// with the two equal — flatForecasts' default — an exit-side mutant
	// pricing the expected rate is invisible, and every $2.00 funding pin
	// would hold under it.
	for n := range in.Forecasts {
		in.Forecasts[n].Expected = mustRate(t, "0.0002")
	}
	in.Position = domain.PositionView{
		Spot: spotInst, Perp: perpInst,
		Qty:       mustQty(t, "0.1"),
		EnteredAt: baseAt,
		EntryPlan: plan,
	}
	in.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt.Add(456 * time.Hour)}
	return in
}

// withDrift returns the input with a nonzero measured entry-drift margin.
func withDrift(t *testing.T, in domain.DecisionInput, rate string) domain.DecisionInput {
	t.Helper()
	in.Policy.Params.EntryDrift = domain.MeasuredRate{Value: mustRate(t, rate), Artifact: fixtureCalibration("drift")}
	return in
}
