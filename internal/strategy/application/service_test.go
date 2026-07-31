package application_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Minimal fakes — one per port, each honest about being a stub. Narrow on
// purpose: a fake implementing two ports would assert a relationship that
// does not exist (marketdata's ports doc).

type fakeMarket struct{}

func (fakeMarket) Funding(ids.VenueInstrument) (mdomain.FundingObservation, bool) {
	return mdomain.FundingObservation{}, false
}
func (fakeMarket) TopOfBook(ids.VenueInstrument) (domain.PriceView, bool) {
	return domain.PriceView{}, false
}

type fakeHistory struct{}

func (fakeHistory) Window(ids.VenueInstrument) ([]domain.RealizedSettlement, error) {
	return nil, nil
}

type fakeForecast struct{}

func (fakeForecast) Estimate(domain.FundingMaterials, []time.Time) ([]domain.SettlementForecast, error) {
	return nil, nil
}

type fakeBasis struct{}

func (fakeBasis) EstimateBasis(domain.MarketView, domain.HoldPlanRevision, fixedpoint.Quantity) (domain.BasisTerm, error) {
	return domain.BasisTerm{}, nil
}

type fakeCost struct{}

func (fakeCost) EstimateCost(domain.ExecutionCostRequest) (domain.ExecutionCostEstimate, error) {
	return domain.ExecutionCostEstimate{}, nil
}

type fakeReference struct{}

func (fakeReference) Fees(ids.VenueInstrument, time.Time) (domain.LegFees, error) {
	return domain.LegFees{}, nil
}
func (fakeReference) LotRules(ids.VenueInstrument, time.Time) (domain.LotRules, error) {
	return domain.LotRules{}, nil
}
func (fakeReference) Hurdle(time.Time) (domain.HurdleBenchmark, error) {
	return domain.HurdleBenchmark{}, nil
}

type fakeSink struct{}

func (fakeSink) Write(domain.Decision) error { return nil }

// Each fake is pinned to the port it stands in for, so a port signature edit
// fails here — in the suite that wires them — rather than at the first real
// adapter.
var (
	_ ports.MarketStateReader  = fakeMarket{}
	_ ports.SettlementHistory  = fakeHistory{}
	_ ports.FundingForecast    = fakeForecast{}
	_ ports.BasisForecast      = fakeBasis{}
	_ ports.ExecutionCostModel = fakeCost{}
	_ ports.ReferenceData      = fakeReference{}
	_ ports.DecisionSink       = fakeSink{}
)

var (
	spotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

func testDeps(t *testing.T) application.Deps {
	t.Helper()
	c, err := clock.NewTestClock(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	return application.Deps{
		Clock:     c,
		Market:    fakeMarket{},
		History:   fakeHistory{},
		Forecast:  fakeForecast{},
		Basis:     fakeBasis{},
		Cost:      fakeCost{},
		Reference: fakeReference{},
		Sink:      fakeSink{},
	}
}

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func testParams(t *testing.T) domain.StrategyParams {
	t.Helper()
	cal := domain.CalibrationRef{ID: domain.FixturePrefix + "app", Hash: "ab"}
	return domain.StrategyParams{
		MinFundingAPR:      mustRate(t, "0.05"),
		MinHoldSettlements: 3,
		ForecastModel:      "ewma.v1",
		EWMAHalfLife:       72 * time.Hour,
		DecisionQuantile:   mustRate(t, "0.25"),
		Aggregation:        domain.AggregationConservative,
		BasisModel:         "conservative.v1",
		EntryDrift:         domain.MeasuredRate{Value: mustRate(t, "0.00004"), Artifact: cal},
		ExitBand:           domain.MeasuredRate{Value: mustRate(t, "0.00006"), Artifact: cal},
		ExitPersist:        3,
		PerpLeverage:       mustRate(t, "3"),
	}
}

func newService(t *testing.T) *application.Service {
	t.Helper()
	s, err := application.NewService(testDeps(t), spotInst, perpInst, testParams(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewServiceNamesTheMissingDependency(t *testing.T) {
	deps := testDeps(t)
	deps.Reference = nil
	_, err := application.NewService(deps, spotInst, perpInst, testParams(t))
	if !errors.Is(err, application.ErrDepNil) {
		t.Fatalf("want ErrDepNil, got %v", err)
	}
	if !strings.Contains(err.Error(), "reference data") {
		t.Fatalf("error does not name the port: %v", err)
	}
}

func TestNewServiceRefusesWrongUniverseShapes(t *testing.T) {
	// Legs swapped: perp where spot belongs.
	if _, err := application.NewService(testDeps(t), perpInst, spotInst, testParams(t)); err == nil {
		t.Fatal("swapped legs accepted")
	}
	// Cross-venue pair is strategy #2's shape, refused here.
	bybitPerp := perpInst
	bybitPerp.Venue = "bybit"
	if _, err := application.NewService(testDeps(t), spotInst, bybitPerp, testParams(t)); !errors.Is(err, application.ErrUniverseShape) {
		t.Fatalf("cross-venue pair: want ErrUniverseShape, got %v", err)
	}
	// Cross-symbol pair is a mispriced hedge, refused.
	ethPerp := perpInst
	ethPerp.Symbol = "ETHUSDT"
	if _, err := application.NewService(testDeps(t), spotInst, ethPerp, testParams(t)); !errors.Is(err, application.ErrUniverseShape) {
		t.Fatalf("cross-symbol pair: want ErrUniverseShape, got %v", err)
	}
}

func TestUniverseIsSpotThenPerpAndNotAliased(t *testing.T) {
	s := newService(t)
	u := s.Universe()
	if len(u) != 2 || u[0] != spotInst || u[1] != perpInst {
		t.Fatalf("universe %v", u)
	}
	// A caller editing its copy must not edit the service's.
	u[0] = perpInst
	if got := s.Universe(); got[0] != spotInst {
		t.Fatal("universe aliased to caller's slice")
	}
}

func TestDecideRefusesLoudlyUntilSliceC(t *testing.T) {
	s := newService(t)
	d, intents, err := s.Decide(nil)
	if !errors.Is(err, application.ErrCoreUnbuilt) {
		t.Fatalf("want ErrCoreUnbuilt, got %v", err)
	}
	if intents != nil {
		t.Fatal("intents alongside an error — the write-ahead contract forbids this")
	}
	if err := d.Validate(); err == nil {
		t.Fatal("a zero Decision validated")
	}
}

func TestDecideRefusesASecondPosition(t *testing.T) {
	s := newService(t)
	_, _, err := s.Decide(make([]domain.PositionView, 2))
	if !errors.Is(err, application.ErrTooManyPositions) {
		t.Fatalf("want ErrTooManyPositions, got %v", err)
	}
}

// TestAdapterDropsErrorsByReturningNoIntents pins the adapter's ONE behavior:
// on any Decide error the §3.3 face returns nil Intents — never a panic,
// never fabricated intents. (That drop is why it is not a production path.)
func TestAdapterDropsErrorsByReturningNoIntents(t *testing.T) {
	a := application.Adapter{S: newService(t)}
	if got := a.Evaluate(domain.MarketView{}); got != nil {
		t.Fatalf("Evaluate returned %v", got)
	}
	if got := a.OnPosition(domain.PositionView{}, domain.MarketView{}); got != nil {
		t.Fatalf("OnPosition returned %v", got)
	}
	if a.Name() != "cashcarry" {
		t.Fatalf("name %q", a.Name())
	}
}
