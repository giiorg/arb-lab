package domain

import (
	"encoding/json"
	"fmt"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// EncodeDecision renders a Decision to its versioned wire form.
//
// The Decision is validated first: the journal is the record §40f's
// completeness property is stated over, and an invalid Decision written
// durably is a hole that recovery would faithfully preserve.
//
// Encoding is deterministic — struct-driven field order, canonical mantissa
// text — which is what makes the byte-level round-trip a testable claim.
func EncodeDecision(d Decision) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("strategy: encode: %w", err)
	}
	w := decisionWire{
		V:       func() *int { v := WireVersion; return &v }(),
		Kind:    strPtr(string(d.Kind)),
		Verdict: strPtr(string(d.Verdict)),
		At:      wireTime(d.At),
		Input:   wireInput(d.Input),
	}
	if d.Terms != (DecisionTerms{}) {
		w.Terms = wireTerms(d.Terms)
	}
	if d.RefusedBy != "" {
		w.RefusedBy = strPtr(d.RefusedBy)
	}
	data, err := json.Marshal(w)
	if err != nil {
		// Unreachable for these shapes; surfaced rather than swallowed
		// because "unreachable" is a property of the shapes, and they move.
		return nil, fmt.Errorf("strategy: encode: %w", err)
	}
	return data, nil
}

// --- composite encoders ----------------------------------------------------
//
// Each returns nil for its domain zero value, so a partial (refusal) input
// encodes exactly what it held and nothing invents members. The zero checks
// are written against the SAME zero the decoder rebuilds, which the
// round-trip battery pins.

func wireMoney(m fixedpoint.Money) *moneyWire {
	if m == (fixedpoint.Money{}) {
		return nil
	}
	return &moneyWire{M: wireI64(m.Mantissa()), S: wireScale(), Asset: strPtr(string(m.Asset()))}
}

func wireQty(q fixedpoint.Quantity) *qtyWire {
	if q == (fixedpoint.Quantity{}) {
		return nil
	}
	return &qtyWire{M: wireI64(q.Mantissa()), S: wireScale(), Asset: strPtr(string(q.Asset()))}
}

func wireRate(r fixedpoint.Rate) *rateWire {
	if r == (fixedpoint.Rate{}) {
		return nil
	}
	return &rateWire{M: wireI64(r.Mantissa()), S: wireScale()}
}

func wirePrice(p fixedpoint.Price) *priceWire {
	if p == (fixedpoint.Price{}) {
		return nil
	}
	return &priceWire{
		M: wireI64(p.Mantissa()), S: wireScale(),
		Base: strPtr(string(p.Base())), Quote: strPtr(string(p.Quote())),
	}
}

func wireInstrument(vi ids.VenueInstrument) *instrumentWire {
	if vi == (ids.VenueInstrument{}) {
		return nil
	}
	return &instrumentWire{
		Venue:    strPtr(string(vi.Venue)),
		Product:  strPtr(string(vi.Product)),
		Symbol:   strPtr(vi.Symbol),
		Contract: strPtr(string(vi.Contract)),
	}
}

func wireWatermark(w mdomain.Watermark) *watermarkWire {
	if w == (mdomain.Watermark{}) {
		return nil
	}
	return &watermarkWire{Seq: wireU64(w.VenueSeq), Source: wireTime(w.SourceTime), Recv: wireTime(w.RecvTime)}
}

func wireRevision(r config.Revision) *revisionWire {
	if r == (config.Revision{}) {
		return nil
	}
	return &revisionWire{
		Num:      wireU64(r.Num),
		Hash:     strPtr(r.HashString()),
		Source:   strPtr(r.Source),
		LoadedAt: wireTime(r.LoadedAt),
	}
}

func wireCalibration(c CalibrationRef) *calibrationWire {
	if c == (CalibrationRef{}) {
		return nil
	}
	return &calibrationWire{ID: strPtr(c.ID), Hash: strPtr(c.Hash)}
}

func wireMeasuredRate(m MeasuredRate) *measuredRateWire {
	if m == (MeasuredRate{}) {
		return nil
	}
	return &measuredRateWire{Value: wireRate(m.Value), Artifact: wireCalibration(m.Artifact)}
}

func wireMeasuredMoney(m MeasuredMoney) *measuredMoneyWire {
	if m == (MeasuredMoney{}) {
		return nil
	}
	return &measuredMoneyWire{Amount: wireMoney(m.Amount), Artifact: wireCalibration(m.Artifact)}
}

func wirePriceView(p PriceView) *priceViewWire {
	if p == (PriceView{}) {
		return nil
	}
	return &priceViewWire{
		Instrument: wireInstrument(p.Instrument),
		Bid:        wirePrice(p.Bid),
		Ask:        wirePrice(p.Ask),
		Watermark:  wireWatermark(p.Watermark),
	}
}

func wireFunding(o mdomain.FundingObservation) *fundingWire {
	if o == (mdomain.FundingObservation{}) {
		return nil
	}
	w := &fundingWire{
		Instrument: wireInstrument(o.Instrument),
		Rate:       wireRate(o.Rate),
		NextSettle: wireTime(o.NextSettle),
		Cap:        wireRate(o.Cap),
		Floor:      wireRate(o.Floor),
		Watermark:  wireWatermark(o.Watermark),
	}
	if o.Interval != 0 {
		w.IntervalNs = wireI64(int64(o.Interval))
	}
	return w
}

func wireRealized(rs []RealizedSettlement) []realizedWire {
	if len(rs) == 0 {
		return nil
	}
	out := make([]realizedWire, len(rs))
	for n, r := range rs {
		out[n] = realizedWire{
			Instrument: wireInstrument(r.Instrument),
			At:         wireTime(r.At),
			Rate:       wireRate(r.Rate),
			Watermark:  wireWatermark(r.Watermark),
		}
	}
	return out
}

func wireView(v MarketView) *viewWire {
	if v.Spot == (PriceView{}) && v.Perp == (PriceView{}) &&
		v.Funding == (mdomain.FundingObservation{}) && v.Realized == nil {
		return nil
	}
	return &viewWire{
		Spot:     wirePriceView(v.Spot),
		Perp:     wirePriceView(v.Perp),
		Funding:  wireFunding(v.Funding),
		Realized: wireRealized(v.Realized),
	}
}

func wireParams(p StrategyParams) *paramsWire {
	if p == (StrategyParams{}) {
		return nil
	}
	w := &paramsWire{
		MinFundingAPR:    wireRate(p.MinFundingAPR),
		DecisionQuantile: wireRate(p.DecisionQuantile),
		EntryDrift:       wireMeasuredRate(p.EntryDrift),
		ExitBand:         wireMeasuredRate(p.ExitBand),
		PerpLeverage:     wireRate(p.PerpLeverage),
	}
	if p.MinHoldSettlements != 0 {
		v := p.MinHoldSettlements
		w.MinHoldSettlements = &v
	}
	if p.ForecastModel != "" {
		w.ForecastModel = strPtr(p.ForecastModel)
	}
	if p.EWMAHalfLife != 0 {
		w.EWMAHalfLifeNs = wireI64(int64(p.EWMAHalfLife))
	}
	if p.Aggregation != "" {
		w.Aggregation = strPtr(string(p.Aggregation))
	}
	if p.BasisModel != "" {
		w.BasisModel = strPtr(p.BasisModel)
	}
	if p.ExitPersist != 0 {
		v := p.ExitPersist
		w.ExitPersist = &v
	}
	return w
}

func wirePolicy(p PolicyProjection) *policyWire {
	if p == (PolicyProjection{}) {
		return nil
	}
	return &policyWire{Params: wireParams(p.Params), Rev: wireRevision(p.Rev)}
}

func wireForecasts(fs []SettlementForecast) []forecastWire {
	if len(fs) == 0 {
		return nil
	}
	out := make([]forecastWire, len(fs))
	for n, f := range fs {
		out[n] = forecastWire{
			At:         wireTime(f.At),
			Expected:   wireRate(f.Expected),
			Adverse:    wireRate(f.Adverse),
			Quantile:   wireRate(f.Quantile),
			Dispersion: wireCalibration(f.Dispersion),
		}
	}
	return out
}

func wireBasis(b BasisTerm) *basisWire {
	if b == (BasisTerm{}) {
		return nil
	}
	return &basisWire{Favorable: wireMoney(b.FavorableConvergence), Adverse: wireMeasuredMoney(b.AdverseMove)}
}

func wireProvenance(p Provenance) *provenanceWire {
	if p == (Provenance{}) {
		return nil
	}
	w := &provenanceWire{ObservedAt: wireTime(p.ObservedAt), Rev: wireRevision(p.Rev)}
	if p.Class != "" {
		w.Class = strPtr(string(p.Class))
	}
	return w
}

func wireFees(f LegFees) *feesWire {
	if f == (LegFees{}) {
		return nil
	}
	w := &feesWire{
		Instrument: wireInstrument(f.Instrument),
		Maker:      wireRate(f.Maker),
		Taker:      wireRate(f.Taker),
		Provenance: wireProvenance(f.Provenance),
	}
	if f.Tier != "" {
		w.Tier = strPtr(f.Tier)
	}
	return w
}

func wireLots(l LotRules) *lotsWire {
	if l == (LotRules{}) {
		return nil
	}
	return &lotsWire{
		Instrument:  wireInstrument(l.Instrument),
		Step:        wireQty(l.Step),
		MinQty:      wireQty(l.MinQty),
		MinNotional: wireMoney(l.MinNotional),
		Provenance:  wireProvenance(l.Provenance),
	}
}

func wireHurdle(h HurdleBenchmark) *hurdleWire {
	if h == (HurdleBenchmark{}) {
		return nil
	}
	return &hurdleWire{
		AnnualRate:          wireRate(h.AnnualRate),
		AccessCost:          wireRate(h.AccessCost),
		LiquidityHaircut:    wireRate(h.LiquidityHaircut),
		CounterpartyHaircut: wireRate(h.CounterpartyHaircut),
		AsOf:                wireTime(h.AsOf),
		Provenance:          wireProvenance(h.Provenance),
	}
}

func wireCapital(c CapitalAllocation) *capitalWire {
	if c == (CapitalAllocation{}) {
		return nil
	}
	return &capitalWire{
		SpotPrincipal:      wireMoney(c.SpotPrincipal),
		PerpInitialMargin:  wireMoney(c.PerpInitialMargin),
		RescueMargin:       wireMeasuredMoney(c.RescueMargin),
		FeeSlippageReserve: wireMoney(c.FeeSlippageReserve),
		CommittedFloat:     wireMoney(c.CommittedFloat),
	}
}

func wireCost(e ExecutionCostEstimate) *costWire {
	if e == (ExecutionCostEstimate{}) {
		return nil
	}
	w := &costWire{
		SpotCost:    wireMoney(e.SpotCost),
		PerpCost:    wireMoney(e.PerpCost),
		Calibration: wireCalibration(e.Calibration),
		Qty:         wireQty(e.Qty),
	}
	if e.Model != "" {
		w.Model = strPtr(e.Model)
	}
	if e.Tactic != "" {
		w.Tactic = strPtr(string(e.Tactic))
	}
	return w
}

func wireSchedule(ts []time.Time) []string {
	if len(ts) == 0 {
		return nil
	}
	out := make([]string, len(ts))
	for n, t := range ts {
		out[n] = t.Format(time.RFC3339Nano)
	}
	return out
}

func wirePlan(p HoldPlanRevision) *planWire {
	if p.Num == 0 && p.Trigger == "" && p.At.IsZero() &&
		p.Plan.Settlements == nil && p.Plan.Terminal.IsZero() {
		return nil
	}
	w := &planWire{
		Num:         wireU64(p.Num),
		At:          wireTime(p.At),
		Settlements: wireSchedule(p.Plan.Settlements),
		Terminal:    wireTime(p.Plan.Terminal),
	}
	if p.Trigger != "" {
		w.Trigger = strPtr(string(p.Trigger))
	}
	return w
}

func wirePosition(p PositionView) *positionWire {
	if p.isZero() {
		return nil
	}
	w := &positionWire{
		Spot:      wireInstrument(p.Spot),
		Perp:      wireInstrument(p.Perp),
		Qty:       wireQty(p.Qty),
		EnteredAt: wireTime(p.EnteredAt),
	}
	if p.EntryPlan.Settlements != nil || !p.EntryPlan.Terminal.IsZero() {
		w.EntryPlan = &planScheduleWire{
			Settlements: wireSchedule(p.EntryPlan.Settlements),
			Terminal:    wireTime(p.EntryPlan.Terminal),
		}
	}
	return w
}

func wireDwell(d DwellState) *dwellWire {
	if !d.Declared {
		return nil
	}
	w := &dwellWire{LastRealized: wireTime(d.LastRealized)}
	c := d.Count
	w.Count = &c
	return w
}

func wireInput(in DecisionInput) *inputWire {
	w := &inputWire{
		At:                wireTime(in.At),
		View:              wireView(in.View),
		Policy:            wirePolicy(in.Policy),
		Forecasts:         wireForecasts(in.Forecasts),
		Basis:             wireBasis(in.Basis),
		SpotFees:          wireFees(in.SpotFees),
		PerpFees:          wireFees(in.PerpFees),
		SpotLots:          wireLots(in.SpotLots),
		PerpLots:          wireLots(in.PerpLots),
		ResidualTolerance: wireQty(in.ResidualTolerance),
		CandidateQty:      wireQty(in.CandidateQty),
		ExecCost:          wireCost(in.ExecCost),
		Hurdle:            wireHurdle(in.Hurdle),
		Capital:           wireCapital(in.Capital),
		Opportunity:       wireMoney(in.Opportunity),
		ClosingCostDelta:  wireMoney(in.ClosingCostDelta),
		RiskDelta:         wireMoney(in.RiskDelta),
		Plan:              wirePlan(in.Plan),
		Position:          wirePosition(in.Position),
		Dwell:             wireDwell(in.Dwell),
	}
	if in.Kind != "" {
		w.Kind = strPtr(string(in.Kind))
	}
	return w
}

func wireTerms(t DecisionTerms) *termsWire {
	return &termsWire{
		FundingQuantile:  wireMoney(t.FundingQuantile),
		Basis:            wireMoney(t.Basis),
		SpotEntryFee:     wireMoney(t.SpotEntryFee),
		SpotExitFee:      wireMoney(t.SpotExitFee),
		PerpEntryFee:     wireMoney(t.PerpEntryFee),
		PerpExitFee:      wireMoney(t.PerpExitFee),
		Slippage:         wireMoney(t.Slippage),
		Opportunity:      wireMoney(t.Opportunity),
		ClosingCostDelta: wireMoney(t.ClosingCostDelta),
		RiskDelta:        wireMoney(t.RiskDelta),
		ScaledHurdle:     wireMoney(t.ScaledHurdle),
		HoldFraction:     wireRate(t.HoldFraction),
		Band:             wireMeasuredRate(t.Band),
		Dwell:            wireDwell(t.Dwell),
	}
}
