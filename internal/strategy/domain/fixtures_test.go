package domain_test

// Shared fixtures for the domain battery.
//
// Shapes are chosen against the fixture-collapse checklist, not for
// convenience:
//
//   - every window and schedule has MORE THAN ONE element, so "ascending",
//     "first" and "last" are distinct questions;
//   - the exit instant sits strictly BETWEEN two settlements, and the
//     boundary tests place it exactly ON one, from both sides;
//   - spot and perp fee rows carry DIFFERENT numbers, so a swapped-leg
//     mistake changes an assertion rather than nothing;
//   - one Money mantissa and one VenueSeq exceed 2^53, so any float sneaking
//     into the wire channel breaks byte equality instead of rounding
//     invisibly.

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

// fixtureCalibration is §40f's "explicit fixture-helper marker in tests": the
// only way this suite mints a calibration identity.
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

func revision(t *testing.T, num uint64, source string) config.Revision {
	t.Helper()
	// The leading hash bytes are chosen to hex-encode to LETTERS (ab, cd):
	// a hash of small integers encodes to digits only, which contains no
	// lowercase character to uppercase — collapsing the wire's
	// case-strictness rule into the length rule (found by a surviving
	// mutant on exactly that).
	return config.Revision{
		Num:      num,
		Hash:     [32]byte{0xab, 0xcd, 3, byte(num)},
		Source:   source,
		LoadedAt: baseAt.Add(-time.Hour),
	}
}

func provenance(t *testing.T) domain.Provenance {
	t.Helper()
	return domain.Provenance{
		Class:      domain.ProvenanceHandRecorded,
		ObservedAt: baseAt.Add(-2 * time.Hour),
		Rev:        revision(t, 1, "configs/reference.btcusdt.json"),
	}
}

// settlements: 14:00, 22:00 (day 0), 06:00 (day 1). Terminal 18:00 day 1.
func fixturePlanSchedule() domain.HoldPlan {
	return domain.HoldPlan{
		Settlements: []time.Time{
			baseAt.Add(2 * time.Hour),
			baseAt.Add(10 * time.Hour),
			baseAt.Add(18 * time.Hour),
		},
		Terminal: baseAt.Add(30 * time.Hour),
	}
}

func fixtureView(t *testing.T) domain.MarketView {
	t.Helper()
	return domain.MarketView{
		Spot: domain.PriceView{
			Instrument: spotInst,
			Bid:        mustPrice(t, "64998.10"),
			Ask:        mustPrice(t, "64998.20"),
			Watermark:  watermark(baseAt, 41),
		},
		Perp: domain.PriceView{
			Instrument: perpInst,
			Bid:        mustPrice(t, "65010.50"),
			Ask:        mustPrice(t, "65010.60"),
			// A venue sequence past 2^53: exact only off the float path.
			Watermark: watermark(baseAt.Add(-120*time.Millisecond), 9_007_199_254_740_995),
		},
		// Funding is left zero here; entryInput fills it. A fixtureView with
		// no funding block doubles as the refusal fixture's partial view.
		Realized: []domain.RealizedSettlement{
			{Instrument: perpInst, At: baseAt.Add(-14 * time.Hour), Rate: mustRate(t, "0.00009"), Watermark: watermark(baseAt.Add(-14*time.Hour), 11)},
			{Instrument: perpInst, At: baseAt.Add(-6 * time.Hour), Rate: mustRate(t, "0.00012"), Watermark: watermark(baseAt.Add(-6*time.Hour), 23)},
		},
	}
}

func fixtureFunding(t *testing.T) mdomain.FundingObservation {
	t.Helper()
	return mdomain.FundingObservation{
		Instrument: perpInst,
		Rate:       mustRate(t, "0.00011"),
		Interval:   8 * time.Hour,
		NextSettle: baseAt.Add(2 * time.Hour),
		Cap:        mustRate(t, "0.0075"),
		Floor:      mustRate(t, "-0.0075"),
		Watermark:  watermark(baseAt.Add(-500*time.Millisecond), 77),
	}
}

func fixtureParams(t *testing.T) domain.StrategyParams {
	t.Helper()
	return domain.StrategyParams{
		MinFundingAPR:      mustRate(t, "0.05"),
		MinHoldSettlements: 3,
		ForecastModel:      "ewma.v1",
		EWMAHalfLife:       72 * time.Hour,
		DecisionQuantile:   mustRate(t, "0.25"),
		Aggregation:        domain.AggregationConservative,
		BasisModel:         "conservative.v1",
		EntryDrift:         domain.MeasuredRate{Value: mustRate(t, "0.00004"), Artifact: fixtureCalibration("drift")},
		ExitBand:           domain.MeasuredRate{Value: mustRate(t, "0.00006"), Artifact: fixtureCalibration("band")},
		ExitPersist:        3,
		PerpLeverage:       mustRate(t, "3"),
	}
}

func fixtureForecast(t *testing.T, at time.Time) domain.SettlementForecast {
	t.Helper()
	return domain.SettlementForecast{
		At:         at,
		Expected:   mustRate(t, "0.00010"),
		Adverse:    mustRate(t, "0.00003"),
		Quantile:   mustRate(t, "0.25"),
		Dispersion: fixtureCalibration("dispersion"),
	}
}

// entryInput is a fully-populated, valid KindEntry DecisionInput.
func entryInput(t *testing.T) domain.DecisionInput {
	t.Helper()
	view := fixtureView(t)
	view.Funding = fixtureFunding(t)
	plan := fixturePlanSchedule()

	forecasts := make([]domain.SettlementForecast, len(plan.Settlements))
	for n, at := range plan.Settlements {
		forecasts[n] = fixtureForecast(t, at)
	}

	return domain.DecisionInput{
		Kind:      domain.KindEntry,
		At:        baseAt,
		View:      view,
		Policy:    domain.PolicyProjection{Params: fixtureParams(t), Rev: revision(t, 4, "configs/strategy.json")},
		Forecasts: forecasts,
		Basis: domain.BasisTerm{
			FavorableConvergence: fixedpoint.ZeroMoney("USDT"),
			AdverseMove:          domain.MeasuredMoney{Amount: mustMoney(t, "11.40"), Artifact: fixtureCalibration("basis")},
		},
		SpotFees: domain.LegFees{Instrument: spotInst, Maker: mustRate(t, "0.001"), Taker: mustRate(t, "0.001"), Tier: "VIP0", Provenance: provenance(t)},
		PerpFees: domain.LegFees{Instrument: perpInst, Maker: mustRate(t, "0.0002"), Taker: mustRate(t, "0.00055"), Tier: "VIP0", Provenance: provenance(t)},
		SpotLots: domain.LotRules{
			Instrument: spotInst, Step: mustQty(t, "0.00001"),
			MinQty: mustQty(t, "0.00001"), MinNotional: mustMoney(t, "5"),
			Provenance: provenance(t),
		},
		PerpLots: domain.LotRules{
			Instrument: perpInst, Step: mustQty(t, "0.001"),
			MinQty: mustQty(t, "0.001"), MinNotional: mustMoney(t, "100"),
			Provenance: provenance(t),
		},
		ResidualTolerance: mustQty(t, "0.0005"),
		CandidateQty:      mustQty(t, "0.153"),
		ExecCost: domain.ExecutionCostEstimate{
			SpotCost:    mustMoney(t, "3.30"),
			PerpCost:    mustMoney(t, "1.10"),
			Model:       "conservative-bound.v1",
			Calibration: fixtureCalibration("slippage"),
			Tactic:      domain.TacticTakerIOC,
			Qty:         mustQty(t, "0.153"),
		},
		Hurdle: domain.HurdleBenchmark{
			AnnualRate:          mustRate(t, "0.046"),
			AccessCost:          mustRate(t, "0.001"),
			LiquidityHaircut:    mustRate(t, "0.002"),
			CounterpartyHaircut: mustRate(t, "0.003"),
			AsOf:                baseAt.Add(-26 * time.Hour),
			Provenance:          provenance(t),
		},
		Capital: domain.CapitalAllocation{
			SpotPrincipal:     mustMoney(t, "10000"),
			PerpInitialMargin: mustMoney(t, "3334"),
			RescueMargin:      domain.MeasuredMoney{Amount: mustMoney(t, "1200"), Artifact: fixtureCalibration("rescue")},
			// Past 2^53 in mantissa units: exact only off the float path.
			FeeSlippageReserve: mustMoney(t, "9007199.254740993"),
			CommittedFloat:     fixedpoint.ZeroMoney("USDT"),
		},
		Opportunity:      mustMoney(t, "18.75"),
		ClosingCostDelta: fixedpoint.ZeroMoney("USDT"),
		RiskDelta:        fixedpoint.ZeroMoney("USDT"),
		Plan: domain.HoldPlanRevision{
			Num: 1, Trigger: domain.TriggerEntry, At: baseAt, Plan: plan,
		},
	}
}

// exitAt sits strictly between the first (+2h) and second (+10h) settlements.
var exitAt = baseAt.Add(3 * time.Hour)

// exitInput is a fully-populated, valid KindExit DecisionInput. The working
// plan is revision 2 (an interval flip), the dwell has counted the one
// realized settlement, and the forecasts cover exactly the settlements
// strictly after exitAt.
func exitInput(t *testing.T) domain.DecisionInput {
	t.Helper()
	in := entryInput(t)
	in.Kind = domain.KindExit
	in.At = exitAt

	in.Plan = domain.HoldPlanRevision{
		Num:     2,
		Trigger: domain.TriggerIntervalFlip,
		At:      baseAt.Add(2*time.Hour + 30*time.Minute),
		Plan:    fixturePlanSchedule(),
	}
	in.Forecasts = []domain.SettlementForecast{
		fixtureForecast(t, in.Plan.Plan.Settlements[1]),
		fixtureForecast(t, in.Plan.Plan.Settlements[2]),
	}
	in.Position = domain.PositionView{
		Spot: spotInst, Perp: perpInst,
		Qty:       mustQty(t, "0.153"),
		EnteredAt: baseAt,
		EntryPlan: fixturePlanSchedule(),
	}
	in.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt.Add(2 * time.Hour)}
	in.ClosingCostDelta = mustMoney(t, "0.85")
	in.RiskDelta = mustMoney(t, "2.10")
	return in
}

func entryTerms(t *testing.T) domain.DecisionTerms {
	t.Helper()
	return domain.DecisionTerms{
		FundingQuantile:  mustMoney(t, "62.10"),
		Basis:            mustMoney(t, "-11.40"),
		SpotEntryFee:     mustMoney(t, "9.95"),
		SpotExitFee:      mustMoney(t, "9.95"),
		PerpEntryFee:     mustMoney(t, "5.47"),
		PerpExitFee:      mustMoney(t, "5.47"),
		Slippage:         mustMoney(t, "4.40"),
		Opportunity:      mustMoney(t, "18.75"),
		ClosingCostDelta: fixedpoint.ZeroMoney("USDT"),
		RiskDelta:        fixedpoint.ZeroMoney("USDT"),
		ScaledHurdle:     mustMoney(t, "2.31"),
		HoldFraction:     mustRate(t, "0.003424657"),
		EntryMargin:      domain.MeasuredRate{Value: mustRate(t, "0.00004"), Artifact: fixtureCalibration("drift")},
	}
}

func exitTerms(t *testing.T) domain.DecisionTerms {
	t.Helper()
	terms := entryTerms(t)
	terms.EntryMargin = domain.MeasuredRate{}
	terms.ClosingCostDelta = mustMoney(t, "0.85")
	terms.RiskDelta = mustMoney(t, "2.10")
	terms.Band = domain.MeasuredRate{Value: mustRate(t, "0.00006"), Artifact: fixtureCalibration("band")}
	terms.Dwell = domain.DwellState{Declared: true, Count: 2, LastRealized: baseAt.Add(10 * time.Hour)}
	return terms
}

func entryDecision(t *testing.T) domain.Decision {
	t.Helper()
	return domain.Decision{
		Kind:    domain.KindEntry,
		Verdict: domain.VerdictEnter,
		At:      baseAt,
		Input:   entryInput(t),
		Terms:   entryTerms(t),
	}
}

func exitDecision(t *testing.T) domain.Decision {
	t.Helper()
	return domain.Decision{
		Kind:    domain.KindExit,
		Verdict: domain.VerdictContinue,
		At:      exitAt,
		Input:   exitInput(t),
		Terms:   exitTerms(t),
	}
}

// refusalDecision carries a PARTIAL input — the view arrived, nothing else
// had been assembled when the freshness rule refused — which is exactly the
// shape §40f requires the journal to hold.
func refusalDecision(t *testing.T) domain.Decision {
	t.Helper()
	view := fixtureView(t)
	view.Funding = fixtureFunding(t)
	return domain.Decision{
		Kind:    domain.KindEntry,
		Verdict: domain.VerdictRefuse,
		At:      baseAt,
		Input: domain.DecisionInput{
			Kind: domain.KindEntry,
			At:   baseAt,
			View: view,
		},
		RefusedBy: "§3.4 freshness: perp book age over budget",
	}
}
