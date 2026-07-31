package domain

import (
	"fmt"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file is the Decision's versioned wire DTO (§40f).
//
// # Why it exists
//
// The domain value cannot serialize itself: fixedpoint's categories hold
// unexported fields, its codec is expressly a database-column codec, and
// nothing implements MarshalJSON — so encoding/json over a Decision writes
// empty objects for every economic term, and the recompute property dies
// silently at the first journal write. The round-2 review read the code and
// caught exactly this.
//
// # The wire rules
//
//   - Versioned: a "v" field, the dispatch log's token-is-the-version-surface
//     pattern. Decode refuses a version it does not hold.
//   - Every dimensional value travels as mantissa + scale + its tag(s),
//     explicitly. The mantissa is a STRING: an int64 mantissa can exceed
//     2^53, where a JSON number silently becomes the float this repo bans.
//     uint64 counters (revision numbers, venue sequences) ride as strings
//     for the same reason.
//   - Strict: unknown fields and trailing data are refusals, never zeros.
//   - Presence is a pointer: nil wires to the domain zero value and back.
//     What presence MEANS — which members a scored decision must carry — is
//     the domain Validate's judgement, not repeated here; decode finishes by
//     validating the rebuilt Decision, so a missing member is refused by the
//     ONE authority that knows which members its kind requires. A refusal
//     Decision's partial input is the case that forces this split.
//
// It is a representation, not a second authority: nothing here decides
// anything a validator elsewhere does not already decide.

// WireVersion is the version this package encodes and the only one it
// decodes.
const WireVersion = 1

// Wire-level sentinels.
var (
	// ErrWireVersion reports a version this build does not hold.
	ErrWireVersion = fmt.Errorf("strategy: unsupported wire version")

	// ErrWireScale reports a dimensional value at a scale this build does
	// not hold. Rate's scale may legitimately diverge from the shared one
	// someday (fixedpoint.Scale's own comment); when it does, the wire gains
	// the new scale by a version bump, never by silent acceptance.
	ErrWireScale = fmt.Errorf("strategy: unsupported wire scale")

	// ErrWireMalformed reports bytes that do not decode: not JSON, unknown
	// fields, trailing data, an unparseable mantissa or instant, or a
	// missing structural member (one the wire itself cannot do without,
	// like the version).
	ErrWireMalformed = fmt.Errorf("strategy: malformed decision wire")
)

// --- wire shapes -----------------------------------------------------------

type moneyWire struct {
	M     *string `json:"m"`
	S     *int    `json:"s"`
	Asset *string `json:"asset"`
}

type qtyWire struct {
	M     *string `json:"m"`
	S     *int    `json:"s"`
	Asset *string `json:"asset"`
}

type rateWire struct {
	M *string `json:"m"`
	S *int    `json:"s"`
}

type priceWire struct {
	M     *string `json:"m"`
	S     *int    `json:"s"`
	Base  *string `json:"base"`
	Quote *string `json:"quote"`
}

type instrumentWire struct {
	Venue    *string `json:"venue"`
	Product  *string `json:"product"`
	Symbol   *string `json:"symbol"`
	Contract *string `json:"contract"`
}

type watermarkWire struct {
	Seq    *string `json:"seq"`
	Source *string `json:"source"`
	Recv   *string `json:"recv"`
}

type revisionWire struct {
	Num      *string `json:"num"`
	Hash     *string `json:"hash"`
	Source   *string `json:"source"`
	LoadedAt *string `json:"loaded_at"`
}

type calibrationWire struct {
	ID   *string `json:"id"`
	Hash *string `json:"hash"`
}

type measuredRateWire struct {
	Value    *rateWire        `json:"value"`
	Artifact *calibrationWire `json:"artifact"`
}

type measuredMoneyWire struct {
	Amount   *moneyWire       `json:"amount"`
	Artifact *calibrationWire `json:"artifact"`
}

type priceViewWire struct {
	Instrument *instrumentWire `json:"instrument"`
	Bid        *priceWire      `json:"bid"`
	Ask        *priceWire      `json:"ask"`
	Watermark  *watermarkWire  `json:"watermark"`
}

type fundingWire struct {
	Instrument *instrumentWire `json:"instrument"`
	Rate       *rateWire       `json:"rate"`
	IntervalNs *string         `json:"interval_ns"`
	NextSettle *string         `json:"next_settle"`
	Cap        *rateWire       `json:"cap"`
	Floor      *rateWire       `json:"floor"`
	Watermark  *watermarkWire  `json:"watermark"`
}

type realizedWire struct {
	Instrument *instrumentWire `json:"instrument"`
	At         *string         `json:"at"`
	Rate       *rateWire       `json:"rate"`
	Watermark  *watermarkWire  `json:"watermark"`
}

type viewWire struct {
	Spot     *priceViewWire `json:"spot"`
	Perp     *priceViewWire `json:"perp"`
	Funding  *fundingWire   `json:"funding"`
	Realized []realizedWire `json:"realized"`
}

type paramsWire struct {
	MinFundingAPR      *rateWire         `json:"min_funding_apr"`
	MinHoldSettlements *uint32           `json:"min_hold_settlements"`
	ForecastModel      *string           `json:"forecast_model"`
	EWMAHalfLifeNs     *string           `json:"ewma_half_life_ns"`
	DecisionQuantile   *rateWire         `json:"decision_quantile"`
	Aggregation        *string           `json:"aggregation"`
	BasisModel         *string           `json:"basis_model"`
	EntryDrift         *measuredRateWire `json:"entry_drift"`
	ExitBand           *measuredRateWire `json:"exit_band"`
	ExitPersist        *uint32           `json:"exit_persist"`
	PerpLeverage       *rateWire         `json:"perp_leverage"`
}

type policyWire struct {
	Params *paramsWire   `json:"params"`
	Rev    *revisionWire `json:"rev"`
}

type forecastWire struct {
	At         *string          `json:"at"`
	Expected   *rateWire        `json:"expected"`
	Adverse    *rateWire        `json:"adverse"`
	Quantile   *rateWire        `json:"quantile"`
	Dispersion *calibrationWire `json:"dispersion"`
}

type basisWire struct {
	Favorable *moneyWire         `json:"favorable"`
	Adverse   *measuredMoneyWire `json:"adverse"`
}

type provenanceWire struct {
	Class      *string       `json:"class"`
	ObservedAt *string       `json:"observed_at"`
	Rev        *revisionWire `json:"rev"`
}

type feesWire struct {
	Instrument *instrumentWire `json:"instrument"`
	Maker      *rateWire       `json:"maker"`
	Taker      *rateWire       `json:"taker"`
	Tier       *string         `json:"tier"`
	Provenance *provenanceWire `json:"provenance"`
}

type lotsWire struct {
	Instrument  *instrumentWire `json:"instrument"`
	Step        *qtyWire        `json:"step"`
	MinQty      *qtyWire        `json:"min_qty"`
	MinNotional *moneyWire      `json:"min_notional"`
	Provenance  *provenanceWire `json:"provenance"`
}

type hurdleWire struct {
	AnnualRate          *rateWire       `json:"annual_rate"`
	AccessCost          *rateWire       `json:"access_cost"`
	LiquidityHaircut    *rateWire       `json:"liquidity_haircut"`
	CounterpartyHaircut *rateWire       `json:"counterparty_haircut"`
	AsOf                *string         `json:"as_of"`
	Provenance          *provenanceWire `json:"provenance"`
}

type capitalWire struct {
	SpotPrincipal      *moneyWire         `json:"spot_principal"`
	PerpInitialMargin  *moneyWire         `json:"perp_initial_margin"`
	RescueMargin       *measuredMoneyWire `json:"rescue_margin"`
	FeeSlippageReserve *moneyWire         `json:"fee_slippage_reserve"`
	CommittedFloat     *moneyWire         `json:"committed_float"`
}

type costWire struct {
	SpotCost    *moneyWire       `json:"spot_cost"`
	PerpCost    *moneyWire       `json:"perp_cost"`
	Model       *string          `json:"model"`
	Calibration *calibrationWire `json:"calibration"`
	Tactic      *string          `json:"tactic"`
	Qty         *qtyWire         `json:"qty"`
}

type planWire struct {
	Num         *string  `json:"num"`
	Trigger     *string  `json:"trigger"`
	At          *string  `json:"at"`
	Settlements []string `json:"settlements"`
	Terminal    *string  `json:"terminal"`
}

type positionWire struct {
	Spot      *instrumentWire   `json:"spot"`
	Perp      *instrumentWire   `json:"perp"`
	Qty       *qtyWire          `json:"qty"`
	EnteredAt *string           `json:"entered_at"`
	EntryPlan *planScheduleWire `json:"entry_plan"`
}

// planScheduleWire is a bare HoldPlan (no revision envelope) — the position's
// immutable entry evidence.
type planScheduleWire struct {
	Settlements []string `json:"settlements"`
	Terminal    *string  `json:"terminal"`
}

type dwellWire struct {
	Count        *uint32 `json:"count"`
	LastRealized *string `json:"last_realized"`
}

type inputWire struct {
	Kind              *string        `json:"kind"`
	At                *string        `json:"at"`
	View              *viewWire      `json:"view"`
	Policy            *policyWire    `json:"policy"`
	Forecasts         []forecastWire `json:"forecasts"`
	Basis             *basisWire     `json:"basis"`
	SpotFees          *feesWire      `json:"spot_fees"`
	PerpFees          *feesWire      `json:"perp_fees"`
	SpotLots          *lotsWire      `json:"spot_lots"`
	PerpLots          *lotsWire      `json:"perp_lots"`
	ResidualTolerance *qtyWire       `json:"residual_tolerance"`
	CandidateQty      *qtyWire       `json:"candidate_qty"`
	ExecCost          *costWire      `json:"exec_cost"`
	Hurdle            *hurdleWire    `json:"hurdle"`
	Capital           *capitalWire   `json:"capital"`
	Opportunity       *moneyWire     `json:"opportunity"`
	ClosingCostDelta  *moneyWire     `json:"closing_cost_delta"`
	RiskDelta         *moneyWire     `json:"risk_delta"`
	Plan              *planWire      `json:"plan"`
	Position          *positionWire  `json:"position"`
	Dwell             *dwellWire     `json:"dwell"`
}

type termsWire struct {
	FundingQuantile  *moneyWire        `json:"funding_quantile"`
	Basis            *moneyWire        `json:"basis"`
	SpotEntryFee     *moneyWire        `json:"spot_entry_fee"`
	SpotExitFee      *moneyWire        `json:"spot_exit_fee"`
	PerpEntryFee     *moneyWire        `json:"perp_entry_fee"`
	PerpExitFee      *moneyWire        `json:"perp_exit_fee"`
	Slippage         *moneyWire        `json:"slippage"`
	Opportunity      *moneyWire        `json:"opportunity"`
	ClosingCostDelta *moneyWire        `json:"closing_cost_delta"`
	RiskDelta        *moneyWire        `json:"risk_delta"`
	ScaledHurdle     *moneyWire        `json:"scaled_hurdle"`
	HoldFraction     *rateWire         `json:"hold_fraction"`
	EntryMargin      *measuredRateWire `json:"entry_margin"`
	Band             *measuredRateWire `json:"band"`
	Dwell            *dwellWire        `json:"dwell"`
}

type decisionWire struct {
	V         *int       `json:"v"`
	Kind      *string    `json:"kind"`
	Verdict   *string    `json:"verdict"`
	At        *string    `json:"at"`
	Input     *inputWire `json:"input"`
	Terms     *termsWire `json:"terms"`
	RefusedBy *string    `json:"refused_by"`
}

// --- scalar helpers --------------------------------------------------------

func wireScale() *int { s := fixedpoint.Scale; return &s }

func strPtr(s string) *string { return &s }

// wireTime renders an instant, empty for the zero instant. RFC3339Nano is
// exact for anything a Clock produces, and the zero instant must stay
// representable because partial (refusal) inputs legitimately carry it.
func wireTime(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	return strPtr(t.Format(time.RFC3339Nano))
}

func parseTime(field string, s *string) (time.Time, error) {
	if s == nil {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q: %w", field, *s, ErrWireMalformed)
	}
	return t, nil
}

func wireU64(v uint64) *string {
	if v == 0 {
		return nil
	}
	return strPtr(strconv.FormatUint(v, 10))
}

func parseU64(field string, s *string) (uint64, error) {
	if s == nil {
		return 0, nil
	}
	v, err := strconv.ParseUint(*s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s %q: %w", field, *s, ErrWireMalformed)
	}
	return v, nil
}

func wireI64(v int64) *string { return strPtr(strconv.FormatInt(v, 10)) }

func parseI64(field string, s *string) (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("%s: %w", field, ErrWireMalformed)
	}
	v, err := strconv.ParseInt(*s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s %q: %w", field, *s, ErrWireMalformed)
	}
	return v, nil
}

func checkScale(field string, s *int) error {
	if s == nil {
		return fmt.Errorf("%s scale: %w", field, ErrWireMalformed)
	}
	if *s != fixedpoint.Scale {
		return fmt.Errorf("%s scale %d: %w", field, *s, ErrWireScale)
	}
	return nil
}
