package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// DecodeDecision rebuilds a Decision from its wire form.
//
// Strict in the wire's stated sense: unknown fields, trailing data, an
// unsupported version or scale, and unparseable text are refusals here;
// which MEMBERS the decision must carry is the domain Validate's judgement,
// run on the rebuilt value before it is returned — a zero here is a
// flattering term, so nothing defaults.
func DecodeDecision(data []byte) (Decision, error) {
	var w decisionWire

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return Decision{}, fmt.Errorf("strategy: decode: %w: %w", ErrWireMalformed, err)
	}
	// One value per journal payload; a second one is a torn or doctored
	// frame, not an extra decision.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Decision{}, fmt.Errorf("strategy: decode: trailing data: %w", ErrWireMalformed)
	}

	if w.V == nil {
		return Decision{}, fmt.Errorf("strategy: decode: version: %w", ErrWireMalformed)
	}
	if *w.V != WireVersion {
		return Decision{}, fmt.Errorf("strategy: decode: v%d: %w", *w.V, ErrWireVersion)
	}

	d := Decision{}
	if w.Kind != nil {
		d.Kind = DecisionKind(*w.Kind)
	}
	if w.Verdict != nil {
		d.Verdict = Verdict(*w.Verdict)
	}
	at, err := parseTime("at", w.At)
	if err != nil {
		return Decision{}, err
	}
	d.At = at
	if w.Input != nil {
		in, err := parseInput(w.Input)
		if err != nil {
			return Decision{}, err
		}
		d.Input = in
	}
	if w.Terms != nil {
		t, err := parseTerms(w.Terms)
		if err != nil {
			return Decision{}, err
		}
		d.Terms = t
	}
	if w.RefusedBy != nil {
		d.RefusedBy = *w.RefusedBy
	}

	if err := d.Validate(); err != nil {
		return Decision{}, fmt.Errorf("strategy: decode: %w", err)
	}
	return d, nil
}

// --- composite decoders ----------------------------------------------------
//
// Each accepts nil and returns its domain zero value: absence on the wire IS
// the zero value, and whether the zero value is admissible is the Validate
// call above, not a per-field judgement here.

func parseMoney(field string, w *moneyWire) (fixedpoint.Money, error) {
	if w == nil {
		return fixedpoint.Money{}, nil
	}
	if err := checkScale(field, w.S); err != nil {
		return fixedpoint.Money{}, err
	}
	if w.Asset == nil {
		return fixedpoint.Money{}, fmt.Errorf("%s asset: %w", field, ErrWireMalformed)
	}
	m, err := parseI64(field, w.M)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	v, err := fixedpoint.MoneyFromMantissa(fixedpoint.Asset(*w.Asset), m)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s: %w: %w", field, ErrWireMalformed, err)
	}
	return v, nil
}

func parseQty(field string, w *qtyWire) (fixedpoint.Quantity, error) {
	if w == nil {
		return fixedpoint.Quantity{}, nil
	}
	if err := checkScale(field, w.S); err != nil {
		return fixedpoint.Quantity{}, err
	}
	if w.Asset == nil {
		return fixedpoint.Quantity{}, fmt.Errorf("%s asset: %w", field, ErrWireMalformed)
	}
	m, err := parseI64(field, w.M)
	if err != nil {
		return fixedpoint.Quantity{}, err
	}
	v, err := fixedpoint.QuantityFromMantissa(fixedpoint.Asset(*w.Asset), m)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("%s: %w: %w", field, ErrWireMalformed, err)
	}
	return v, nil
}

func parseRate(field string, w *rateWire) (fixedpoint.Rate, error) {
	if w == nil {
		return fixedpoint.Rate{}, nil
	}
	if err := checkScale(field, w.S); err != nil {
		return fixedpoint.Rate{}, err
	}
	m, err := parseI64(field, w.M)
	if err != nil {
		return fixedpoint.Rate{}, err
	}
	return fixedpoint.RateFromMantissa(m), nil
}

func parsePrice(field string, w *priceWire) (fixedpoint.Price, error) {
	if w == nil {
		return fixedpoint.Price{}, nil
	}
	if err := checkScale(field, w.S); err != nil {
		return fixedpoint.Price{}, err
	}
	if w.Base == nil || w.Quote == nil {
		return fixedpoint.Price{}, fmt.Errorf("%s assets: %w", field, ErrWireMalformed)
	}
	m, err := parseI64(field, w.M)
	if err != nil {
		return fixedpoint.Price{}, err
	}
	v, err := fixedpoint.PriceFromMantissa(fixedpoint.Asset(*w.Base), fixedpoint.Asset(*w.Quote), m)
	if err != nil {
		return fixedpoint.Price{}, fmt.Errorf("%s: %w: %w", field, ErrWireMalformed, err)
	}
	return v, nil
}

func parseInstrument(w *instrumentWire) ids.VenueInstrument {
	if w == nil {
		return ids.VenueInstrument{}
	}
	var vi ids.VenueInstrument
	if w.Venue != nil {
		vi.Venue = ids.Venue(*w.Venue)
	}
	if w.Product != nil {
		vi.Product = ids.ProductCategory(*w.Product)
	}
	if w.Symbol != nil {
		vi.Symbol = *w.Symbol
	}
	if w.Contract != nil {
		vi.Contract = ids.ContractType(*w.Contract)
	}
	return vi
}

func parseWatermark(field string, w *watermarkWire) (mdomain.Watermark, error) {
	if w == nil {
		return mdomain.Watermark{}, nil
	}
	seq, err := parseU64(field+" seq", w.Seq)
	if err != nil {
		return mdomain.Watermark{}, err
	}
	src, err := parseTime(field+" source", w.Source)
	if err != nil {
		return mdomain.Watermark{}, err
	}
	recv, err := parseTime(field+" recv", w.Recv)
	if err != nil {
		return mdomain.Watermark{}, err
	}
	return mdomain.Watermark{VenueSeq: seq, SourceTime: src, RecvTime: recv}, nil
}

func parseRevision(field string, w *revisionWire) (config.Revision, error) {
	if w == nil {
		return config.Revision{}, nil
	}
	var r config.Revision
	num, err := parseU64(field+" num", w.Num)
	if err != nil {
		return config.Revision{}, err
	}
	r.Num = num
	if w.Hash != nil {
		var h [32]byte
		n, err := decodeHex(h[:], *w.Hash)
		if err != nil || n != len(h) {
			return config.Revision{}, fmt.Errorf("%s hash %q: %w", field, *w.Hash, ErrWireMalformed)
		}
		r.Hash = h
	}
	if w.Source != nil {
		r.Source = *w.Source
	}
	at, err := parseTime(field+" loaded at", w.LoadedAt)
	if err != nil {
		return config.Revision{}, err
	}
	r.LoadedAt = at
	return r, nil
}

func parseCalibration(w *calibrationWire) CalibrationRef {
	if w == nil {
		return CalibrationRef{}
	}
	var c CalibrationRef
	if w.ID != nil {
		c.ID = *w.ID
	}
	if w.Hash != nil {
		c.Hash = *w.Hash
	}
	return c
}

func parseMeasuredRate(field string, w *measuredRateWire) (MeasuredRate, error) {
	if w == nil {
		return MeasuredRate{}, nil
	}
	v, err := parseRate(field+" value", w.Value)
	if err != nil {
		return MeasuredRate{}, err
	}
	return MeasuredRate{Value: v, Artifact: parseCalibration(w.Artifact)}, nil
}

func parseMeasuredMoney(field string, w *measuredMoneyWire) (MeasuredMoney, error) {
	if w == nil {
		return MeasuredMoney{}, nil
	}
	v, err := parseMoney(field+" amount", w.Amount)
	if err != nil {
		return MeasuredMoney{}, err
	}
	return MeasuredMoney{Amount: v, Artifact: parseCalibration(w.Artifact)}, nil
}

func parsePriceView(field string, w *priceViewWire) (PriceView, error) {
	if w == nil {
		return PriceView{}, nil
	}
	bid, err := parsePrice(field+" bid", w.Bid)
	if err != nil {
		return PriceView{}, err
	}
	ask, err := parsePrice(field+" ask", w.Ask)
	if err != nil {
		return PriceView{}, err
	}
	wm, err := parseWatermark(field+" watermark", w.Watermark)
	if err != nil {
		return PriceView{}, err
	}
	return PriceView{Instrument: parseInstrument(w.Instrument), Bid: bid, Ask: ask, Watermark: wm}, nil
}

func parseFunding(w *fundingWire) (mdomain.FundingObservation, error) {
	if w == nil {
		return mdomain.FundingObservation{}, nil
	}
	rate, err := parseRate("funding rate", w.Rate)
	if err != nil {
		return mdomain.FundingObservation{}, err
	}
	capr, err := parseRate("funding cap", w.Cap)
	if err != nil {
		return mdomain.FundingObservation{}, err
	}
	floor, err := parseRate("funding floor", w.Floor)
	if err != nil {
		return mdomain.FundingObservation{}, err
	}
	next, err := parseTime("funding next settle", w.NextSettle)
	if err != nil {
		return mdomain.FundingObservation{}, err
	}
	wm, err := parseWatermark("funding watermark", w.Watermark)
	if err != nil {
		return mdomain.FundingObservation{}, err
	}
	o := mdomain.FundingObservation{
		Instrument: parseInstrument(w.Instrument),
		Rate:       rate, Cap: capr, Floor: floor,
		NextSettle: next, Watermark: wm,
	}
	if w.IntervalNs != nil {
		ns, err := parseI64("funding interval", w.IntervalNs)
		if err != nil {
			return mdomain.FundingObservation{}, err
		}
		o.Interval = time.Duration(ns)
	}
	return o, nil
}

func parseRealized(ws []realizedWire) ([]RealizedSettlement, error) {
	if len(ws) == 0 {
		return nil, nil
	}
	out := make([]RealizedSettlement, len(ws))
	for n, w := range ws {
		at, err := parseTime(fmt.Sprintf("realized %d at", n), w.At)
		if err != nil {
			return nil, err
		}
		rate, err := parseRate(fmt.Sprintf("realized %d rate", n), w.Rate)
		if err != nil {
			return nil, err
		}
		wm, err := parseWatermark(fmt.Sprintf("realized %d watermark", n), w.Watermark)
		if err != nil {
			return nil, err
		}
		out[n] = RealizedSettlement{
			Instrument: parseInstrument(w.Instrument),
			At:         at, Rate: rate, Watermark: wm,
		}
	}
	return out, nil
}

func parseView(w *viewWire) (MarketView, error) {
	if w == nil {
		return MarketView{}, nil
	}
	spot, err := parsePriceView("view spot", w.Spot)
	if err != nil {
		return MarketView{}, err
	}
	perp, err := parsePriceView("view perp", w.Perp)
	if err != nil {
		return MarketView{}, err
	}
	fund, err := parseFunding(w.Funding)
	if err != nil {
		return MarketView{}, err
	}
	realized, err := parseRealized(w.Realized)
	if err != nil {
		return MarketView{}, err
	}
	return MarketView{Spot: spot, Perp: perp, Funding: fund, Realized: realized}, nil
}

func parseParams(w *paramsWire) (StrategyParams, error) {
	if w == nil {
		return StrategyParams{}, nil
	}
	var p StrategyParams
	var err error
	if p.MinFundingAPR, err = parseRate("params min funding apr", w.MinFundingAPR); err != nil {
		return StrategyParams{}, err
	}
	if w.MinHoldSettlements != nil {
		p.MinHoldSettlements = *w.MinHoldSettlements
	}
	if w.ForecastModel != nil {
		p.ForecastModel = *w.ForecastModel
	}
	if w.EWMAHalfLifeNs != nil {
		ns, err := parseI64("params ewma half-life", w.EWMAHalfLifeNs)
		if err != nil {
			return StrategyParams{}, err
		}
		p.EWMAHalfLife = time.Duration(ns)
	}
	if p.DecisionQuantile, err = parseRate("params decision quantile", w.DecisionQuantile); err != nil {
		return StrategyParams{}, err
	}
	if w.Aggregation != nil {
		p.Aggregation = Aggregation(*w.Aggregation)
	}
	if w.BasisModel != nil {
		p.BasisModel = *w.BasisModel
	}
	if p.EntryDrift, err = parseMeasuredRate("params entry drift", w.EntryDrift); err != nil {
		return StrategyParams{}, err
	}
	if p.ExitBand, err = parseMeasuredRate("params exit band", w.ExitBand); err != nil {
		return StrategyParams{}, err
	}
	if w.ExitPersist != nil {
		p.ExitPersist = *w.ExitPersist
	}
	if p.PerpLeverage, err = parseRate("params perp leverage", w.PerpLeverage); err != nil {
		return StrategyParams{}, err
	}
	return p, nil
}

func parsePolicy(w *policyWire) (PolicyProjection, error) {
	if w == nil {
		return PolicyProjection{}, nil
	}
	params, err := parseParams(w.Params)
	if err != nil {
		return PolicyProjection{}, err
	}
	rev, err := parseRevision("policy rev", w.Rev)
	if err != nil {
		return PolicyProjection{}, err
	}
	return PolicyProjection{Params: params, Rev: rev}, nil
}

func parseForecasts(ws []forecastWire) ([]SettlementForecast, error) {
	if len(ws) == 0 {
		return nil, nil
	}
	out := make([]SettlementForecast, len(ws))
	for n, w := range ws {
		at, err := parseTime(fmt.Sprintf("forecast %d at", n), w.At)
		if err != nil {
			return nil, err
		}
		exp, err := parseRate(fmt.Sprintf("forecast %d expected", n), w.Expected)
		if err != nil {
			return nil, err
		}
		adv, err := parseRate(fmt.Sprintf("forecast %d adverse", n), w.Adverse)
		if err != nil {
			return nil, err
		}
		q, err := parseRate(fmt.Sprintf("forecast %d quantile", n), w.Quantile)
		if err != nil {
			return nil, err
		}
		out[n] = SettlementForecast{
			At: at, Expected: exp, Adverse: adv, Quantile: q,
			Dispersion: parseCalibration(w.Dispersion),
		}
	}
	return out, nil
}

func parseBasis(w *basisWire) (BasisTerm, error) {
	if w == nil {
		return BasisTerm{}, nil
	}
	fav, err := parseMoney("basis favorable", w.Favorable)
	if err != nil {
		return BasisTerm{}, err
	}
	adv, err := parseMeasuredMoney("basis adverse", w.Adverse)
	if err != nil {
		return BasisTerm{}, err
	}
	return BasisTerm{FavorableConvergence: fav, AdverseMove: adv}, nil
}

func parseProvenance(field string, w *provenanceWire) (Provenance, error) {
	if w == nil {
		return Provenance{}, nil
	}
	var p Provenance
	if w.Class != nil {
		p.Class = ProvenanceClass(*w.Class)
	}
	at, err := parseTime(field+" observed at", w.ObservedAt)
	if err != nil {
		return Provenance{}, err
	}
	p.ObservedAt = at
	rev, err := parseRevision(field+" rev", w.Rev)
	if err != nil {
		return Provenance{}, err
	}
	p.Rev = rev
	return p, nil
}

func parseFees(field string, w *feesWire) (LegFees, error) {
	if w == nil {
		return LegFees{}, nil
	}
	maker, err := parseRate(field+" maker", w.Maker)
	if err != nil {
		return LegFees{}, err
	}
	taker, err := parseRate(field+" taker", w.Taker)
	if err != nil {
		return LegFees{}, err
	}
	prov, err := parseProvenance(field, w.Provenance)
	if err != nil {
		return LegFees{}, err
	}
	f := LegFees{Instrument: parseInstrument(w.Instrument), Maker: maker, Taker: taker, Provenance: prov}
	if w.Tier != nil {
		f.Tier = *w.Tier
	}
	return f, nil
}

func parseLots(field string, w *lotsWire) (LotRules, error) {
	if w == nil {
		return LotRules{}, nil
	}
	step, err := parseQty(field+" step", w.Step)
	if err != nil {
		return LotRules{}, err
	}
	minQty, err := parseQty(field+" min qty", w.MinQty)
	if err != nil {
		return LotRules{}, err
	}
	minNotional, err := parseMoney(field+" min notional", w.MinNotional)
	if err != nil {
		return LotRules{}, err
	}
	tick, err := parsePrice(field+" tick", w.Tick)
	if err != nil {
		return LotRules{}, err
	}
	prov, err := parseProvenance(field, w.Provenance)
	if err != nil {
		return LotRules{}, err
	}
	return LotRules{
		Instrument: parseInstrument(w.Instrument),
		Step:       step, MinQty: minQty, MinNotional: minNotional,
		Tick:       tick,
		Provenance: prov,
	}, nil
}

func parseHurdle(w *hurdleWire) (HurdleBenchmark, error) {
	if w == nil {
		return HurdleBenchmark{}, nil
	}
	var h HurdleBenchmark
	var err error
	if h.AnnualRate, err = parseRate("hurdle annual rate", w.AnnualRate); err != nil {
		return HurdleBenchmark{}, err
	}
	if h.AccessCost, err = parseRate("hurdle access cost", w.AccessCost); err != nil {
		return HurdleBenchmark{}, err
	}
	if h.LiquidityHaircut, err = parseRate("hurdle liquidity haircut", w.LiquidityHaircut); err != nil {
		return HurdleBenchmark{}, err
	}
	if h.CounterpartyHaircut, err = parseRate("hurdle counterparty haircut", w.CounterpartyHaircut); err != nil {
		return HurdleBenchmark{}, err
	}
	if h.AsOf, err = parseTime("hurdle as of", w.AsOf); err != nil {
		return HurdleBenchmark{}, err
	}
	if h.Provenance, err = parseProvenance("hurdle", w.Provenance); err != nil {
		return HurdleBenchmark{}, err
	}
	return h, nil
}

func parseCapital(w *capitalWire) (CapitalAllocation, error) {
	if w == nil {
		return CapitalAllocation{}, nil
	}
	var c CapitalAllocation
	var err error
	if c.SpotPrincipal, err = parseMoney("capital spot principal", w.SpotPrincipal); err != nil {
		return CapitalAllocation{}, err
	}
	if c.PerpInitialMargin, err = parseMoney("capital perp initial margin", w.PerpInitialMargin); err != nil {
		return CapitalAllocation{}, err
	}
	if c.RescueMargin, err = parseMeasuredMoney("capital rescue margin", w.RescueMargin); err != nil {
		return CapitalAllocation{}, err
	}
	if c.FeeSlippageReserve, err = parseMoney("capital fee/slippage reserve", w.FeeSlippageReserve); err != nil {
		return CapitalAllocation{}, err
	}
	if c.CommittedFloat, err = parseMoney("capital committed float", w.CommittedFloat); err != nil {
		return CapitalAllocation{}, err
	}
	return c, nil
}

func parseCost(w *costWire) (ExecutionCostEstimate, error) {
	if w == nil {
		return ExecutionCostEstimate{}, nil
	}
	var e ExecutionCostEstimate
	var err error
	if e.SpotCost, err = parseMoney("exec cost spot", w.SpotCost); err != nil {
		return ExecutionCostEstimate{}, err
	}
	if e.PerpCost, err = parseMoney("exec cost perp", w.PerpCost); err != nil {
		return ExecutionCostEstimate{}, err
	}
	if w.Model != nil {
		e.Model = *w.Model
	}
	e.Calibration = parseCalibration(w.Calibration)
	if w.Tactic != nil {
		e.Tactic = ExecTactic(*w.Tactic)
	}
	if e.Qty, err = parseQty("exec cost qty", w.Qty); err != nil {
		return ExecutionCostEstimate{}, err
	}
	return e, nil
}

func parseSchedule(field string, ws []string) ([]time.Time, error) {
	if len(ws) == 0 {
		return nil, nil
	}
	out := make([]time.Time, len(ws))
	for n, s := range ws {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, fmt.Errorf("%s %d %q: %w", field, n, s, ErrWireMalformed)
		}
		out[n] = t
	}
	return out, nil
}

func parsePlan(w *planWire) (HoldPlanRevision, error) {
	if w == nil {
		return HoldPlanRevision{}, nil
	}
	var p HoldPlanRevision
	num, err := parseU64("plan num", w.Num)
	if err != nil {
		return HoldPlanRevision{}, err
	}
	p.Num = num
	if w.Trigger != nil {
		p.Trigger = PlanTrigger(*w.Trigger)
	}
	if p.At, err = parseTime("plan at", w.At); err != nil {
		return HoldPlanRevision{}, err
	}
	if p.Plan.Settlements, err = parseSchedule("plan settlement", w.Settlements); err != nil {
		return HoldPlanRevision{}, err
	}
	if p.Plan.Terminal, err = parseTime("plan terminal", w.Terminal); err != nil {
		return HoldPlanRevision{}, err
	}
	return p, nil
}

func parsePosition(w *positionWire) (PositionView, error) {
	if w == nil {
		return PositionView{}, nil
	}
	var p PositionView
	var err error
	p.Spot = parseInstrument(w.Spot)
	p.Perp = parseInstrument(w.Perp)
	if p.Qty, err = parseQty("position qty", w.Qty); err != nil {
		return PositionView{}, err
	}
	if p.EnteredAt, err = parseTime("position entered at", w.EnteredAt); err != nil {
		return PositionView{}, err
	}
	if w.EntryPlan != nil {
		if p.EntryPlan.Settlements, err = parseSchedule("entry plan settlement", w.EntryPlan.Settlements); err != nil {
			return PositionView{}, err
		}
		if p.EntryPlan.Terminal, err = parseTime("entry plan terminal", w.EntryPlan.Terminal); err != nil {
			return PositionView{}, err
		}
	}
	return p, nil
}

func parseDwell(w *dwellWire) (DwellState, error) {
	if w == nil {
		return DwellState{}, nil
	}
	d := DwellState{Declared: true}
	if w.Count != nil {
		d.Count = *w.Count
	}
	var err error
	if d.LastRealized, err = parseTime("dwell last realized", w.LastRealized); err != nil {
		return DwellState{}, err
	}
	return d, nil
}

func parseInput(w *inputWire) (DecisionInput, error) {
	var in DecisionInput
	var err error
	if w.Kind != nil {
		in.Kind = DecisionKind(*w.Kind)
	}
	if in.At, err = parseTime("input at", w.At); err != nil {
		return DecisionInput{}, err
	}
	if in.View, err = parseView(w.View); err != nil {
		return DecisionInput{}, err
	}
	if in.Policy, err = parsePolicy(w.Policy); err != nil {
		return DecisionInput{}, err
	}
	if in.Forecasts, err = parseForecasts(w.Forecasts); err != nil {
		return DecisionInput{}, err
	}
	if in.Basis, err = parseBasis(w.Basis); err != nil {
		return DecisionInput{}, err
	}
	if in.SpotFees, err = parseFees("spot fees", w.SpotFees); err != nil {
		return DecisionInput{}, err
	}
	if in.PerpFees, err = parseFees("perp fees", w.PerpFees); err != nil {
		return DecisionInput{}, err
	}
	if in.SpotLots, err = parseLots("spot lots", w.SpotLots); err != nil {
		return DecisionInput{}, err
	}
	if in.PerpLots, err = parseLots("perp lots", w.PerpLots); err != nil {
		return DecisionInput{}, err
	}
	if in.ResidualTolerance, err = parseQty("residual tolerance", w.ResidualTolerance); err != nil {
		return DecisionInput{}, err
	}
	if in.CandidateQty, err = parseQty("candidate qty", w.CandidateQty); err != nil {
		return DecisionInput{}, err
	}
	if in.ExecCost, err = parseCost(w.ExecCost); err != nil {
		return DecisionInput{}, err
	}
	if in.Hurdle, err = parseHurdle(w.Hurdle); err != nil {
		return DecisionInput{}, err
	}
	if in.Capital, err = parseCapital(w.Capital); err != nil {
		return DecisionInput{}, err
	}
	if in.Opportunity, err = parseMoney("opportunity", w.Opportunity); err != nil {
		return DecisionInput{}, err
	}
	if in.ClosingCostDelta, err = parseMoney("closing-cost delta", w.ClosingCostDelta); err != nil {
		return DecisionInput{}, err
	}
	if in.RiskDelta, err = parseMoney("risk delta", w.RiskDelta); err != nil {
		return DecisionInput{}, err
	}
	if in.Plan, err = parsePlan(w.Plan); err != nil {
		return DecisionInput{}, err
	}
	if in.Position, err = parsePosition(w.Position); err != nil {
		return DecisionInput{}, err
	}
	if in.Dwell, err = parseDwell(w.Dwell); err != nil {
		return DecisionInput{}, err
	}
	return in, nil
}

func parseTerms(w *termsWire) (DecisionTerms, error) {
	var t DecisionTerms
	var err error
	if t.FundingQuantile, err = parseMoney("terms funding quantile", w.FundingQuantile); err != nil {
		return DecisionTerms{}, err
	}
	if t.Basis, err = parseMoney("terms basis", w.Basis); err != nil {
		return DecisionTerms{}, err
	}
	if t.SpotEntryFee, err = parseMoney("terms spot entry fee", w.SpotEntryFee); err != nil {
		return DecisionTerms{}, err
	}
	if t.SpotExitFee, err = parseMoney("terms spot exit fee", w.SpotExitFee); err != nil {
		return DecisionTerms{}, err
	}
	if t.PerpEntryFee, err = parseMoney("terms perp entry fee", w.PerpEntryFee); err != nil {
		return DecisionTerms{}, err
	}
	if t.PerpExitFee, err = parseMoney("terms perp exit fee", w.PerpExitFee); err != nil {
		return DecisionTerms{}, err
	}
	if t.Slippage, err = parseMoney("terms slippage", w.Slippage); err != nil {
		return DecisionTerms{}, err
	}
	if t.Opportunity, err = parseMoney("terms opportunity", w.Opportunity); err != nil {
		return DecisionTerms{}, err
	}
	if t.ClosingCostDelta, err = parseMoney("terms closing-cost delta", w.ClosingCostDelta); err != nil {
		return DecisionTerms{}, err
	}
	if t.RiskDelta, err = parseMoney("terms risk delta", w.RiskDelta); err != nil {
		return DecisionTerms{}, err
	}
	if t.ScaledHurdle, err = parseMoney("terms scaled hurdle", w.ScaledHurdle); err != nil {
		return DecisionTerms{}, err
	}
	if t.EntryMargin, err = parseMeasuredRate("terms entry margin", w.EntryMargin); err != nil {
		return DecisionTerms{}, err
	}
	if t.HoldFraction, err = parseRate("terms hold fraction", w.HoldFraction); err != nil {
		return DecisionTerms{}, err
	}
	if t.Band, err = parseMeasuredRate("terms band", w.Band); err != nil {
		return DecisionTerms{}, err
	}
	if t.Dwell, err = parseDwell(w.Dwell); err != nil {
		return DecisionTerms{}, err
	}
	return t, nil
}

// decodeHex is encoding/hex.Decode with this package's strictness: it
// refuses uppercase digits, because the encoder emits lowercase and a
// case-folded hash has two spellings — the one-spelling rule applies to a
// journal token exactly as it applies to a symbol.
func decodeHex(dst []byte, s string) (int, error) {
	if len(s) != 2*len(dst) {
		return 0, ErrWireMalformed
	}
	for i := 0; i < len(dst); i++ {
		hi, ok := hexNibble(s[2*i])
		lo, ok2 := hexNibble(s[2*i+1])
		if !ok || !ok2 {
			return 0, ErrWireMalformed
		}
		dst[i] = hi<<4 | lo
	}
	return len(dst), nil
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}
