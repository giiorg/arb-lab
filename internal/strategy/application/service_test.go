package application_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// The fakes are one small env: each port serves the env's data, the sink
// records every write in order, and each test tunes exactly the fields its
// claim needs. One fake per port on purpose — a fake implementing two ports
// would assert a relationship that does not exist (marketdata's ports doc).

var (
	baseAt   = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	spotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func mustPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func fixtureCal(name string) domain.CalibrationRef {
	return domain.CalibrationRef{ID: domain.FixturePrefix + name, Hash: "abcdef01"}
}

func watermark(at time.Time, seq uint64) mdomain.Watermark {
	return mdomain.Watermark{VenueSeq: seq, SourceTime: at.Add(-40 * time.Millisecond), RecvTime: at}
}

// env is the whole wired world for one test.
type env struct {
	now      time.Time
	funding  mdomain.FundingObservation
	haveFund bool
	spotTop  domain.PriceView
	perpTop  domain.PriceView
	haveTop  bool
	window   []domain.RealizedSettlement

	forecastRate string
	forecastErr  error

	sink *recordingSink
}

type envMarket struct{ e *env }

func (m envMarket) Funding(ids.VenueInstrument) (mdomain.FundingObservation, bool) {
	return m.e.funding, m.e.haveFund
}
func (m envMarket) TopOfBook(inst ids.VenueInstrument) (domain.PriceView, bool) {
	if !m.e.haveTop {
		return domain.PriceView{}, false
	}
	if inst == spotInst {
		return m.e.spotTop, true
	}
	return m.e.perpTop, true
}

type envHistory struct{ e *env }

func (h envHistory) Window(ids.VenueInstrument) ([]domain.RealizedSettlement, error) {
	return h.e.window, nil
}

type envForecast struct{ e *env }

func (f envForecast) Estimate(_ domain.FundingMaterials, horizon []time.Time) ([]domain.SettlementForecast, error) {
	if f.e.forecastErr != nil {
		return nil, f.e.forecastErr
	}
	out := make([]domain.SettlementForecast, len(horizon))
	for n, at := range horizon {
		out[n] = domain.SettlementForecast{
			At:       at,
			Expected: mustParsedRate(f.e.forecastRate),
			Adverse:  mustParsedRate(f.e.forecastRate),
			Quantile: mustParsedRate("0.25"),
			Dispersion: domain.CalibrationRef{
				ID: domain.FixturePrefix + "dispersion", Hash: "abcdef01",
			},
		}
	}
	return out, nil
}

func mustParsedRate(s string) fixedpoint.Rate {
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		panic(err)
	}
	return r
}

type envBasis struct{}

func (envBasis) EstimateBasis(view domain.MarketView, _ domain.HoldPlanRevision, _ fixedpoint.Quantity) (domain.BasisTerm, error) {
	return domain.BasisTerm{
		FavorableConvergence: fixedpoint.ZeroMoney("USDT"),
		AdverseMove:          domain.MeasuredMoney{Amount: fixedpoint.ZeroMoney("USDT"), Artifact: fixtureCal("basis")},
	}, nil
}

type envCost struct{}

func (envCost) EstimateCost(r domain.ExecutionCostRequest) (domain.ExecutionCostEstimate, error) {
	two, err := fixedpoint.ParseMoney("USDT", "2")
	if err != nil {
		return domain.ExecutionCostEstimate{}, err
	}
	return domain.ExecutionCostEstimate{
		SpotCost: two, PerpCost: two,
		Model: "conservative-bound.v1", Calibration: fixtureCal("slippage"),
		Tactic: r.Tactic, Qty: r.Qty,
	}, nil
}

type envReference struct{ e *env }

func (r envReference) prov() domain.Provenance {
	return domain.Provenance{
		Class:      domain.ProvenanceHandRecorded,
		ObservedAt: baseAt.Add(-2 * time.Hour),
		Rev: config.Revision{
			Num: 1, Hash: [32]byte{0xab, 0xcd, 9}, Source: "configs/reference.btcusdt.json",
			LoadedAt: baseAt.Add(-time.Hour),
		},
	}
}

func (r envReference) Fees(inst ids.VenueInstrument, _ time.Time) (domain.LegFees, error) {
	if inst == spotInst {
		return domain.LegFees{
			Instrument: inst,
			Maker:      mustParsedRate("0.0008"), Taker: mustParsedRate("0.001"),
			Tier: "VIP0", Provenance: r.prov(),
		}, nil
	}
	return domain.LegFees{
		Instrument: inst,
		Maker:      mustParsedRate("0.0002"), Taker: mustParsedRate("0.0005"),
		Tier: "VIP0", Provenance: r.prov(),
	}, nil
}

func (r envReference) LotRules(inst ids.VenueInstrument, _ time.Time) (domain.LotRules, error) {
	step := "0.00001"
	if inst == perpInst {
		step = "0.001"
	}
	stepQ, err := fixedpoint.ParseQuantity("BTC", step)
	if err != nil {
		return domain.LotRules{}, err
	}
	minN, err := fixedpoint.ParseMoney("USDT", "5")
	if err != nil {
		return domain.LotRules{}, err
	}
	return domain.LotRules{
		Instrument: inst, Step: stepQ, MinQty: stepQ, MinNotional: minN,
		Provenance: r.prov(),
	}, nil
}

func (r envReference) Hurdle(time.Time) (domain.HurdleBenchmark, error) {
	zero := mustParsedRate("0")
	return domain.HurdleBenchmark{
		AnnualRate: mustParsedRate("0.046"),
		AccessCost: zero, LiquidityHaircut: zero, CounterpartyHaircut: zero,
		AsOf: baseAt.Add(-26 * time.Hour), Provenance: r.prov(),
	}, nil
}

// Each fake is pinned to the port it stands in for, so a port signature
// edit fails here — in the suite that wires them — rather than at the
// first real adapter.
var (
	_ ports.MarketStateReader  = envMarket{}
	_ ports.SettlementHistory  = envHistory{}
	_ ports.FundingForecast    = envForecast{}
	_ ports.BasisForecast      = envBasis{}
	_ ports.ExecutionCostModel = envCost{}
	_ ports.ReferenceData      = envReference{}
	_ ports.DecisionSink       = (*recordingSink)(nil)
)

// recordingSink records every write in order and can be staged to fail on
// the Nth write (1-based) — NOT hardwired to the last slot, which is
// §39q-2's own collapse.
type recordingSink struct {
	wrote    []domain.Decision
	attempts int
	failOn   int
}

func (s *recordingSink) Write(d domain.Decision) error {
	s.attempts++
	if s.failOn > 0 && s.attempts == s.failOn {
		return fmt.Errorf("staged sink failure")
	}
	s.wrote = append(s.wrote, d)
	return nil
}

// zeroEntropy is a deterministic entropy source for the ID generator.
type zeroEntropy struct{}

func (zeroEntropy) Read(p []byte) (int, error) {
	for n := range p {
		p[n] = 0x5a
	}
	return len(p), nil
}

func newEnv(t *testing.T, now time.Time) *env {
	t.Helper()
	flat := mustPrice(t, "100000")
	e := &env{
		now: now,
		funding: mdomain.FundingObservation{
			Instrument: perpInst,
			Rate:       mustRate(t, "0.00015"),
			Interval:   8 * time.Hour,
			NextSettle: baseAt.Add(8 * time.Hour),
			Cap:        mustRate(t, "0.0075"),
			Floor:      mustRate(t, "-0.0075"),
			Watermark:  watermark(now.Add(-500*time.Millisecond), 77),
		},
		haveFund: true,
		spotTop:  domain.PriceView{Instrument: spotInst, Bid: flat, Ask: flat, Watermark: watermark(now, 41)},
		perpTop:  domain.PriceView{Instrument: perpInst, Bid: flat, Ask: flat, Watermark: watermark(now, 43)},
		haveTop:  true,
		window: []domain.RealizedSettlement{
			{Instrument: perpInst, At: baseAt.Add(-8 * time.Hour), Rate: mustRate(t, "0.00014"), Watermark: watermark(baseAt.Add(-8*time.Hour), 11)},
			{Instrument: perpInst, At: baseAt, Rate: mustRate(t, "0.00016"), Watermark: watermark(baseAt, 23)},
		},
		forecastRate: "0.002",
		sink:         &recordingSink{},
	}
	return e
}

func policy(t *testing.T) domain.PolicyProjection {
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
			EntryDrift:         domain.MeasuredRate{Value: mustRate(t, "0"), Artifact: fixtureCal("drift")},
			ExitBand:           domain.MeasuredRate{Value: mustRate(t, "0.01"), Artifact: fixtureCal("band")},
			ExitPersist:        2,
			PerpLeverage:       mustRate(t, "3"),
		},
		Rev: config.Revision{
			Num: 4, Hash: [32]byte{0xab, 0xcd, 4}, Source: "configs/strategy.json",
			LoadedAt: baseAt.Add(-time.Hour),
		},
	}
}

func settings(t *testing.T) application.Settings {
	t.Helper()
	return application.Settings{
		CandidateQty:      mustQty(t, "0.1"),
		ResidualTolerance: mustQty(t, "0"),
		Tactic:            domain.TacticTakerIOC,
		Latency:           100 * time.Millisecond,
		MaxBookAge:        5 * time.Second,
		MaxFundingAge:     10 * time.Second,
		SkewLimit:         2 * time.Second,
		IntentTTL:         30 * time.Second,
	}
}

func capital(t *testing.T) domain.CapitalAllocation {
	t.Helper()
	return domain.CapitalAllocation{
		SpotPrincipal:      mustMoney(t, "10000"),
		PerpInitialMargin:  mustMoney(t, "3333.34"),
		RescueMargin:       domain.MeasuredMoney{Amount: mustMoney(t, "3333.33"), Artifact: fixtureCal("rescue")},
		FeeSlippageReserve: mustMoney(t, "40"),
		CommittedFloat:     fixedpoint.ZeroMoney("USDT"),
	}
}

func newService(t *testing.T, e *env) *application.Service {
	t.Helper()
	c, err := clock.NewTestClock(e.now, nil)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := ids.NewGenerator(c, zeroEntropy{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := application.NewService(application.Deps{
		Clock:     c,
		IDs:       gen,
		Market:    envMarket{e},
		History:   envHistory{e},
		Forecast:  envForecast{e},
		Basis:     envBasis{},
		Cost:      envCost{},
		Reference: envReference{e},
		Sink:      e.sink,
	}, spotInst, perpInst, policy(t), settings(t), capital(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// heldPosition is the entry the exit tests continue: 0.2 BTC entered at
// baseAt on the three-settlement minimum plan (+8h, +16h, +24h). The held
// quantity DIFFERS from the settings' 0.1 candidate deliberately — with the
// two equal, a mutant pricing the close at the entry candidate is invisible
// (the checklist's pinned-equal-values collapse).
func heldPosition(t *testing.T) (domain.PositionView, application.Continuation) {
	t.Helper()
	plan := domain.HoldPlan{
		Settlements: []time.Time{
			baseAt.Add(8 * time.Hour), baseAt.Add(16 * time.Hour), baseAt.Add(24 * time.Hour),
		},
		Terminal: baseAt.Add(24 * time.Hour),
	}
	p := domain.PositionView{
		Spot: spotInst, Perp: perpInst,
		Qty:       mustQty(t, "0.2"),
		EnteredAt: baseAt,
		EntryPlan: plan,
	}
	cont := application.Continuation{
		Plan:  domain.HoldPlanRevision{Num: 1, Trigger: domain.TriggerEntry, At: baseAt, Plan: plan},
		Dwell: domain.DwellState{Declared: true},
	}
	return p, cont
}

// exitEnv is the world nine hours in: one settlement realized, the next at
// +16h, watermarks fresh at the new instant.
func exitEnv(t *testing.T) *env {
	t.Helper()
	now := baseAt.Add(9 * time.Hour)
	e := newEnv(t, now)
	e.funding.NextSettle = baseAt.Add(16 * time.Hour)
	e.funding.Watermark = watermark(now.Add(-500*time.Millisecond), 177)
	e.spotTop.Watermark = watermark(now, 141)
	e.perpTop.Watermark = watermark(now, 143)
	e.window = append(e.window, domain.RealizedSettlement{
		Instrument: perpInst, At: baseAt.Add(8 * time.Hour),
		Rate: mustRate(t, "0.00015"), Watermark: watermark(baseAt.Add(8*time.Hour), 99),
	})
	return e
}

func TestNewServiceNamesTheMissingDependency(t *testing.T) {
	e := newEnv(t, baseAt)
	c, err := clock.NewTestClock(baseAt, nil)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := ids.NewGenerator(c, zeroEntropy{})
	if err != nil {
		t.Fatal(err)
	}
	deps := application.Deps{
		Clock: c, IDs: gen,
		Market: envMarket{e}, History: envHistory{e}, Forecast: envForecast{e},
		Basis: envBasis{}, Cost: envCost{}, Sink: e.sink,
		// Reference deliberately nil.
	}
	_, err = application.NewService(deps, spotInst, perpInst, policy(t), settings(t), capital(t))
	if !errors.Is(err, application.ErrDepNil) {
		t.Fatalf("want ErrDepNil, got %v", err)
	}
	if !strings.Contains(err.Error(), "reference data") {
		t.Fatalf("error does not name the port: %v", err)
	}
}

func TestNewServiceRefusesWrongUniverseShapes(t *testing.T) {
	e := newEnv(t, baseAt)
	c, _ := clock.NewTestClock(baseAt, nil)
	gen, _ := ids.NewGenerator(c, zeroEntropy{})
	deps := application.Deps{
		Clock: c, IDs: gen,
		Market: envMarket{e}, History: envHistory{e}, Forecast: envForecast{e},
		Basis: envBasis{}, Cost: envCost{}, Reference: envReference{e}, Sink: e.sink,
	}
	if _, err := application.NewService(deps, perpInst, spotInst, policy(t), settings(t), capital(t)); err == nil {
		t.Fatal("swapped legs accepted")
	}
	bybitPerp := perpInst
	bybitPerp.Venue = "bybit"
	if _, err := application.NewService(deps, spotInst, bybitPerp, policy(t), settings(t), capital(t)); !errors.Is(err, application.ErrUniverseShape) {
		t.Fatalf("cross-venue: %v", err)
	}
	bad := settings(t)
	bad.SkewLimit = 0
	if _, err := application.NewService(deps, spotInst, perpInst, policy(t), bad, capital(t)); err == nil {
		t.Fatal("zero skew limit accepted — §5.2's own named trap")
	}
}

func TestEntryDecidesSinksThenEmits(t *testing.T) {
	e := newEnv(t, baseAt)
	s := newService(t, e)

	d, intents, err := s.Decide(nil, application.Continuation{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictEnter {
		t.Fatalf("verdict %s (refused by %q)", d.Verdict, d.RefusedBy)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	// Write-ahead: the sink holds exactly this decision.
	if len(e.sink.wrote) != 1 {
		t.Fatalf("%d sink writes", len(e.sink.wrote))
	}
	if e.sink.wrote[0].Verdict != d.Verdict || !e.sink.wrote[0].At.Equal(d.At) {
		t.Fatal("sunk decision is not the returned decision")
	}
	// The proposed plan is the minimum hold from the live schedule.
	if d.Input.Plan.Num != 1 || d.Input.Plan.Trigger != domain.TriggerEntry {
		t.Fatalf("plan %+v", d.Input.Plan)
	}
	if n := len(d.Input.Plan.Plan.Settlements); n != 3 {
		t.Fatalf("%d planned settlements, want the minimum 3", n)
	}

	if len(intents) != 1 {
		t.Fatalf("%d intents", len(intents))
	}
	in := intents[0]
	if in.Kind != domain.IntentOpen || in.Revision != 1 {
		t.Fatalf("intent %+v", in)
	}
	if len(in.Legs) != 2 ||
		in.Legs[0] != (domain.LegSpec{Instrument: spotInst, Side: domain.SideBuy}) ||
		in.Legs[1] != (domain.LegSpec{Instrument: perpInst, Side: domain.SideSell}) {
		t.Fatalf("legs %+v", in.Legs)
	}
	if in.TargetQty != mustQty(t, "0.1") {
		t.Fatalf("target %s", in.TargetQty)
	}
	if !in.ValidUntil.Equal(baseAt.Add(30 * time.Second)) {
		t.Fatalf("valid until %v", in.ValidUntil)
	}
	if err := in.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestSinkFailureSuppressesIntentsMidSequence stages the failure on the
// SECOND of three writes: the §40f contract (zero Intents AND a non-nil
// error) must hold mid-run, and later writes must still succeed — the
// staged slot is deliberately not the last one (§39q-2).
func TestSinkFailureSuppressesIntentsMidSequence(t *testing.T) {
	e := newEnv(t, baseAt)
	s := newService(t, e)
	e.sink.failOn = 2

	if _, _, err := s.Decide(nil, application.Continuation{}); err != nil {
		t.Fatalf("first: %v", err)
	}
	d, intents, err := s.Decide(nil, application.Continuation{})
	if !errors.Is(err, application.ErrSinkFailed) {
		t.Fatalf("second: want ErrSinkFailed, got %v", err)
	}
	if intents != nil {
		t.Fatal("intents alongside a failed journal write")
	}
	if d.Verdict != domain.VerdictEnter {
		t.Fatalf("the decision itself should still be returned: %s", d.Verdict)
	}
	if _, _, err := s.Decide(nil, application.Continuation{}); err != nil {
		t.Fatalf("third: %v", err)
	}
	if len(e.sink.wrote) != 2 {
		t.Fatalf("%d journaled decisions, want writes 1 and 3", len(e.sink.wrote))
	}
}

func TestPreCoreRefusalsAreJournaled(t *testing.T) {
	cases := []struct {
		name string
		tune func(*env)
		rule string
	}{
		{"market absent", func(e *env) { e.haveFund = false }, application.RefuseMarketAbsent},
		{"stale book", func(e *env) {
			e.spotTop.Watermark = watermark(e.now.Add(-6*time.Second), 41)
		}, application.RefuseStale},
		{"skew breach", func(e *env) {
			e.perpTop.Watermark = watermark(e.now.Add(-3*time.Second), 43)
		}, application.RefuseSkew},
		// The mirror: the SPOT leg older. |a−b| has two signs, and a test
		// exercising only one lets a dropped absolute-value survive — the
		// standing mirror rule (Send/Receive's own lesson).
		{"skew breach mirrored", func(e *env) {
			e.spotTop.Watermark = watermark(e.now.Add(-3*time.Second), 41)
		}, application.RefuseSkew},
		{"port failure", func(e *env) { e.forecastErr = fmt.Errorf("boom") }, application.RefusePort},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, baseAt)
			c.tune(e)
			s := newService(t, e)
			d, intents, err := s.Decide(nil, application.Continuation{})
			if err != nil {
				t.Fatal(err)
			}
			if intents != nil {
				t.Fatal("intents from a refusal")
			}
			if d.Verdict != domain.VerdictRefuse || !strings.HasPrefix(d.RefusedBy, c.rule) {
				t.Fatalf("verdict %s refused by %q, want %s", d.Verdict, d.RefusedBy, c.rule)
			}
			if err := d.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(e.sink.wrote) != 1 {
				t.Fatalf("refusal not journaled: %d writes", len(e.sink.wrote))
			}
		})
	}
}

// TestStaleFundingHasItsOwnBudget separates the funding budget from the
// book budget: funding older than ITS budget refuses even with fresh books.
func TestStaleFundingHasItsOwnBudget(t *testing.T) {
	e := newEnv(t, baseAt)
	e.funding.Watermark = watermark(baseAt.Add(-11*time.Second), 77)
	s := newService(t, e)
	d, _, err := s.Decide(nil, application.Continuation{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictRefuse || !strings.Contains(d.RefusedBy, "funding") {
		t.Fatalf("refused by %q", d.RefusedBy)
	}
}

func TestEntryRefusesContinuationState(t *testing.T) {
	e := newEnv(t, baseAt)
	s := newService(t, e)
	_, cont := heldPosition(t)
	if _, _, err := s.Decide(nil, cont); !errors.Is(err, application.ErrEntryContinuation) {
		t.Fatalf("want ErrEntryContinuation, got %v", err)
	}
}

func TestDecideRefusesCallerBugs(t *testing.T) {
	e := newEnv(t, baseAt)
	s := newService(t, e)
	if _, _, err := s.Decide(make([]domain.PositionView, 2), application.Continuation{}); !errors.Is(err, application.ErrTooManyPositions) {
		t.Fatalf("two positions: %v", err)
	}
	p, cont := heldPosition(t)
	p.Perp.Venue = "bybit"
	if _, _, err := s.Decide([]domain.PositionView{p}, cont); !errors.Is(err, application.ErrForeignPosition) {
		t.Fatalf("foreign position: %v", err)
	}
}

func TestExitContinues(t *testing.T) {
	e := exitEnv(t)
	s := newService(t, e)
	p, cont := heldPosition(t)

	d, intents, err := s.Decide([]domain.PositionView{p}, cont)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("verdict %s (refused by %q)", d.Verdict, d.RefusedBy)
	}
	if intents != nil {
		t.Fatal("intents from a continue")
	}
	// The live schedule agrees with the working plan: no regeneration.
	if d.Input.Plan.Num != 1 {
		t.Fatalf("plan regenerated to %d with an unchanged schedule", d.Input.Plan.Num)
	}
	if len(e.sink.wrote) != 1 {
		t.Fatalf("%d writes", len(e.sink.wrote))
	}
}

// TestExitMissingContinuationIsARefusal is §40b's forgotten-feedback case:
// the dwell (and the working plan) must arrive from the prior journaled
// Decision, and forgetting them is a journaled refusal naming the state —
// never a dwell that silently restarts.
func TestExitMissingContinuationIsARefusal(t *testing.T) {
	e := exitEnv(t)
	s := newService(t, e)
	p, _ := heldPosition(t)

	d, intents, err := s.Decide([]domain.PositionView{p}, application.Continuation{})
	if err != nil {
		t.Fatal(err)
	}
	if intents != nil || d.Verdict != domain.VerdictRefuse ||
		!strings.HasPrefix(d.RefusedBy, application.RefuseContinuation) {
		t.Fatalf("verdict %s refused by %q", d.Verdict, d.RefusedBy)
	}
	if len(e.sink.wrote) != 1 {
		t.Fatal("refusal not journaled")
	}
}

// TestExitClosesAfterTheDwell drives the negative regime: the forecast
// clamps to the venue floor, the score dives far below the band, and the
// fed-back dwell (one settlement already counted) advances on the newly
// realized +8h settlement to reach exit_persist = 2 → Close, with the close
// intent flattening the held quantity.
func TestExitClosesAfterTheDwell(t *testing.T) {
	e := exitEnv(t)
	e.forecastRate = "-0.0075"
	s := newService(t, e)
	p, cont := heldPosition(t)
	cont.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt}

	d, intents, err := s.Decide([]domain.PositionView{p}, cont)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictClose {
		t.Fatalf("verdict %s", d.Verdict)
	}
	if d.Terms.Dwell.Count != 2 || !d.Terms.Dwell.LastRealized.Equal(baseAt.Add(8*time.Hour)) {
		t.Fatalf("dwell %+v", d.Terms.Dwell)
	}
	if len(intents) != 1 || intents[0].Kind != domain.IntentClose {
		t.Fatalf("intents %+v", intents)
	}
	if intents[0].TargetQty != p.Qty {
		t.Fatalf("close qty %s, held %s", intents[0].TargetQty, p.Qty)
	}
	if len(intents[0].Legs) != 2 ||
		intents[0].Legs[0] != (domain.LegSpec{Instrument: spotInst, Side: domain.SideSell}) ||
		intents[0].Legs[1] != (domain.LegSpec{Instrument: perpInst, Side: domain.SideBuy}) {
		t.Fatalf("close legs %+v", intents[0].Legs)
	}
}

// TestExitRegeneratesOnAnIntervalFlip flips the venue to a 1h interval
// mid-position: the working plan advances to revision 2 with the flip
// trigger, the journaled decision carries it, and the fed-back dwell rides
// through regeneration untouched — a schedule change must never reset or
// inflate the dwell (§40f).
func TestExitRegeneratesOnAnIntervalFlip(t *testing.T) {
	e := exitEnv(t)
	e.funding.Interval = time.Hour
	e.funding.NextSettle = baseAt.Add(10 * time.Hour)
	// Keep the regime healthy so the verdict is Continue and the reset
	// path (score above band) is what the dwell meets, proving the
	// regeneration itself did not touch the count on the way through.
	s := newService(t, e)
	p, cont := heldPosition(t)
	cont.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt}

	d, _, err := s.Decide([]domain.PositionView{p}, cont)
	if err != nil {
		t.Fatal(err)
	}
	if d.Input.Plan.Num != 2 || d.Input.Plan.Trigger != domain.TriggerIntervalFlip {
		t.Fatalf("plan %+v", d.Input.Plan)
	}
	if !d.Input.Plan.Plan.Terminal.Equal(p.EntryPlan.Terminal) {
		t.Fatalf("terminal moved: %v", d.Input.Plan.Plan.Terminal)
	}
	// The INPUT dwell is the fed-back state, untouched by regeneration.
	if d.Input.Dwell != cont.Dwell {
		t.Fatalf("regeneration moved the fed-back dwell: %+v", d.Input.Dwell)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("verdict %s", d.Verdict)
	}
}

// TestExitAtASettlementInstant evaluates exactly ON the +16h settlement:
// that settlement is settling — a realized fact, not a forecastable one —
// so the horizon must hold only +24h. A mutant widening "strictly after"
// to "at or after" forecasts the settling instant, and the input's own
// horizon rule turns that into an assembly refusal instead of a Continue.
func TestExitAtASettlementInstant(t *testing.T) {
	now := baseAt.Add(16 * time.Hour)
	e := newEnv(t, now)
	e.funding.NextSettle = baseAt.Add(24 * time.Hour)
	e.funding.Watermark = watermark(now.Add(-500*time.Millisecond), 188)
	e.spotTop.Watermark = watermark(now, 151)
	e.perpTop.Watermark = watermark(now, 153)
	e.window = append(e.window,
		domain.RealizedSettlement{
			Instrument: perpInst, At: baseAt.Add(8 * time.Hour),
			Rate: mustRate(t, "0.00015"), Watermark: watermark(baseAt.Add(8*time.Hour), 99),
		},
		domain.RealizedSettlement{
			Instrument: perpInst, At: now,
			Rate: mustRate(t, "0.00015"), Watermark: watermark(now, 101),
		},
	)
	s := newService(t, e)
	p, cont := heldPosition(t)

	d, _, err := s.Decide([]domain.PositionView{p}, cont)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("verdict %s (refused by %q)", d.Verdict, d.RefusedBy)
	}
	if len(d.Input.Forecasts) != 1 || !d.Input.Forecasts[0].At.Equal(baseAt.Add(24*time.Hour)) {
		t.Fatalf("forecasts %+v, want only the +24h settlement", d.Input.Forecasts)
	}
}

// TestFreshnessAndSkewAtTheBudgetPass pins the permissive side of both §3.4
// gates: an age exactly AT the budget and a skew exactly AT the limit both
// proceed — the rule is "over budget", and a ≥ mutant turns every
// budget-exact read into a refusal.
func TestFreshnessAndSkewAtTheBudgetPass(t *testing.T) {
	e := newEnv(t, baseAt)
	e.spotTop.Watermark = watermark(baseAt.Add(-5*time.Second), 41)  // == MaxBookAge
	e.perpTop.Watermark = watermark(baseAt.Add(-3*time.Second), 43)  // skew 2s == SkewLimit
	e.funding.Watermark = watermark(baseAt.Add(-10*time.Second), 77) // == MaxFundingAge
	s := newService(t, e)
	d, _, err := s.Decide(nil, application.Continuation{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict == domain.VerdictRefuse {
		t.Fatalf("budget-exact view refused: %q", d.RefusedBy)
	}
}

func TestAdapterDropsErrorsByReturningNoIntents(t *testing.T) {
	e := newEnv(t, baseAt)
	e.haveFund = false
	a := application.Adapter{S: newService(t, e)}
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
