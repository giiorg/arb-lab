package driving

// The §41e procedure parameters — chosen and recorded AT the procedure
// commit, which is this file's. Every constant here is a derivation
// parameter the artifact freezes: the derivation run embeds this commit's
// hash and the executable's digest in the artifact, so editing any value
// below after seeing the numbers is a visibly different artifact — a new
// version, a complete rerun, §8's own price. The design record (§41b,
// RESUME) deliberately names none of these values; THIS commit is where
// they are chosen, and the rationale for each lives beside it.
//
// Exploratory readings that informed (not tuned) the choices, all
// calibration-side (2026-08-01, archive days 07-30/07-31): spot top-20
// spread pinned at one tick ≈ 1.6e-7 with depth p50 ≈ 4.3 BTC/side; perp
// spread ≈ 1.6e-6 (tick 0.10) spiking to 1.5e-4, depth p50 ≈ 7–13 BTC;
// transport latency p50 ≈ 50–120 ms, healthy-hour p99 ≈ 100 ms, stall
// tails to minutes during reconnect churn; Binance fundingInfo (our own
// REST overlay, 2026-07-31): BTCUSDT cap ±0.003, interval 8 h.

import (
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// procRate parses a procedure rate constant; a malformed constant is a
// programmer error caught by the first test that touches the roster.
func procRate(s string) fixedpoint.Rate {
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		panic(err)
	}
	return r
}

func procQty(s string) fixedpoint.Quantity {
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		panic(err)
	}
	return q
}

// --- The dispersion / drift procedure (family 1 + the merge) ---

// ProcResidualGrid is the committed residual quantile grid: it spans the
// policy's decision quantile (0.25) and both tails, and every point is a
// grid MEMBER so ShiftAt never interpolates at the points the runtime
// draws. Seven points — enough shape for a per-settlement distribution,
// few enough that every point keeps real evidence behind it.
var ProcResidualGrid = []fixedpoint.Rate{
	procRate("0.05"), procRate("0.1"), procRate("0.25"), procRate("0.5"),
	procRate("0.75"), procRate("0.9"), procRate("0.95"),
}

// ProcEntryDriftTail is the adverse-tail quantile EntryDrift freezes from
// — the 5% lower tail, a grid member. The drift is derived from the
// FAMILY-1 (EWMA) residual quantiles, matching the §41b roster's family
// pinning; the merged distribution's tail coincides with it wherever the
// EWMA tail is the more adverse, which the short imminent window makes
// the expected case — checked, not assumed, by the emit-time invariant in
// run.go.
var ProcEntryDriftTail = procRate("0.05")

// ProcEWMAHalfLife is the trailing EWMA's half-life: §2.4's own stated
// default ("a strategy param, default ~3 days"), taken verbatim — chosen
// from the spec's words, not from the data.
const ProcEWMAHalfLife = 72 * time.Hour

// --- The frozen replay policy (the surrogate's StrategyParams) ---

// The occupancy replay decides under this policy; it is the §41b frame's
// "frozen policy", stated here because §41e wants every verdict-affecting
// choice in the procedure commit. Values follow the spec's defaults where
// it states one, and the step-3 tutorial ledger's shape where it does not.
var (
	// ProcMinFundingAPR: entry is not evaluated below 1% annualized —
	// §2.4's screening floor kept low enough that the hurdle, not the
	// floor, is the binding gate.
	ProcMinFundingAPR = procRate("0.01")

	// ProcMinHoldSettlements: three settlements (24 h) — the §3.9 ledger's
	// minimum hold, the shortest hold the exit machinery's dwell can
	// meaningfully act inside.
	ProcMinHoldSettlements uint32 = 3

	// ProcDecisionQuantile: §2.4's stated default, p25.
	ProcDecisionQuantile = procRate("0.25")

	// ProcExitPersist: two realized settlements below the band before
	// stand-down — the step-3 battery's dwell, one settlement of grace
	// against a single adverse print without letting a regime flip ride.
	ProcExitPersist uint32 = 2

	// ProcPerpLeverage: 4× isolated — the tutorial ledger's pinned
	// leverage; the capital decomposition below is sized to it.
	ProcPerpLeverage = procRate("4")
)

// --- The band / basis procedure (family 1) ---

// ProcBandQuantile: the exit band freezes the 90th percentile of
// recovered-dip depths — the upper statistic, deep enough to cover the
// regime wobble the dwell is meant to ride out, short of the 99th where
// two days' worth of extreme dips would set the band from a handful of
// observations.
var ProcBandQuantile = procRate("0.9")

// ProcBasisHorizon is the hold-length horizon the adverse basis move is
// measured over: the minimum hold (3 settlements × 8 h = 24 h), the
// shortest horizon a position actually rides — and 24 exact kline steps.
const ProcBasisHorizon = 24 * time.Hour

// ProcBasisQuantile: the 99th percentile of horizon moves — the basis
// charge prices a hold-scale risk, and three years of hourly pairs give
// the 99th real evidence (≈25k observations).
var ProcBasisQuantile = procRate("0.99")

// --- The family-2 observation procedure ---

// ProcFamily2 is the committed book-observation procedure (§41b-ii-i):
// 10-second epoch-grid samples over the two archive days give ≈17k
// instants per leg; the 250 ms walk-ahead is the declared latency bound
// below; one second of slack bounds both legs' staleness (books arrive at
// ≈100 ms cadence, so slack only ever bites across a stall, which is
// exactly when a pair should not be measured).
//
// Candidates: 0.02 BTC is the $1K gate's candidate (the surrogate's, lot
// aligned on both legs); 0.10 and 0.50 give the ladder headroom so a
// larger reference revision prices without a v2. Strictly ascending —
// each is a cell's MaxQty.
var ProcFamily2 = Family2Params{
	SampleEvery: 10 * time.Second,
	Latency:     ProcLatencyAssumedBound,
	PairSlack:   time.Second,
	Candidates:  []fixedpoint.Quantity{procQty("0.02"), procQty("0.1"), procQty("0.5")},
	Classifier:  ProcClassifier(),
}

// ProcFillQuantile is §8's own letter: p90 slippage.
var ProcFillQuantile = procRate("0.9")

// ProcMoveQuantile: the adverse fill move freezes the 99th percentile —
// it is charged on EVERY decision as the walk branch's companion, so it
// prices the tail, not the median.
var ProcMoveQuantile = procRate("0.99")

// ProcMinEvidence is §41b-ii-iii's declared minimum observation count:
// 500 samples per cell. A stressed stratum that never accumulates 500
// instants in the window is ABSENT, and absent is a runtime refusal —
// the honest direction for a two-day archive.
const ProcMinEvidence uint64 = 500

// ProcLatencyAssumedBound is family 3's declared decision-to-arrival
// upper bound: 250 ms. No order path exists before milestone 3's OMS, so
// this is a DECLARATION (the §8 marker's subject), informed by family
// 2's transport statistics with a stated multiplier: healthy-hour
// transport p99 ≈ 100 ms (market-data hop, venue stamp to our clock),
// ×2.5 for the unmeasured order path — order entry traverses the same
// route with venue matching on top. The runtime adapter refuses any
// LONGER assumption at decision time, and a measured order-path latency
// is required before capital (the marker's owed §8 clause).
const ProcLatencyAssumedBound = 250 * time.Millisecond

// ProcImminent is the committed imminent-residual procedure: estimates
// sampled each minute (the stream ticks at 1 s, so the grid decimates
// rather than interpolates); one minute of slack bounds both the sampled
// estimate's staleness and the settling frame's distance from its
// instant.
var ProcImminent = ImminentParams{
	SampleEvery:   time.Minute,
	RealizedSlack: time.Minute,
}

// ProcClassifier is the frozen book-state classifier's parameter set
// (§41b-ii-b): three strata, ordered, first match wins, catch-all last so
// every book classifies at runtime (an unclassifiable book would be a
// refusal to price at all — reserve that for a classifier a v2 narrows).
//
// The cut points are LEG-AGNOSTIC because one classifier places books
// from both legs: spot's one-tick spread ≈1.6e-7 and perp's ≈1.6e-6 both
// sit well under the calm bound of 5e-6, while observed stress spikes
// (to 1.5e-4) fall through to normal or stressed. Depth cuts: 2 BTC on
// the walked side is ≈40× the gate candidate (calm); half a BTC still
// fills the 0.5 ladder top exactly once (normal); below that, stressed.
// Depth is measured in the base asset (BTC).
func ProcClassifier() domain.StratumClassifier {
	unboundedRate := domain.RateBound{Kind: domain.BoundUnbounded}
	unboundedQty := domain.QuantityBound{Kind: domain.BoundUnbounded}
	inclRate := func(s string) domain.RateBound {
		return domain.RateBound{Kind: domain.BoundInclusive, Value: procRate(s)}
	}
	inclQty := func(s string) domain.QuantityBound {
		return domain.QuantityBound{Kind: domain.BoundInclusive, Value: procQty(s)}
	}
	return domain.StratumClassifier{Rules: []domain.StratumRule{
		{
			Name:   "calm",
			Spread: domain.RateInterval{Lo: unboundedRate, Hi: inclRate("0.000005")},
			Depth:  domain.QuantityInterval{Lo: inclQty("2"), Hi: unboundedQty},
		},
		{
			Name:   "normal",
			Spread: domain.RateInterval{Lo: unboundedRate, Hi: inclRate("0.00002")},
			Depth:  domain.QuantityInterval{Lo: inclQty("0.5"), Hi: unboundedQty},
		},
		{
			Name:   "stressed",
			Spread: domain.RateInterval{Lo: unboundedRate, Hi: unboundedRate},
			Depth:  domain.QuantityInterval{Lo: unboundedQty, Hi: unboundedQty},
		},
	}}
}

// --- The surrogate frame (family 1's occupancy/clearing constants) ---

// The frame's clamps are the venue's own declared BTCUSDT funding bounds,
// measured from OUR archive's fundingInfo overlay (2026-07-31): ±0.003
// per 8 h. The 2023-era clamps are not refetchable; the current declared
// bound is the stated stand-in, and a tighter historical clamp would only
// have SHRUNK realizable rates — the direction that admits fewer entries,
// conservative for occupancy.
var (
	ProcFrameCap      = procRate("0.003")
	ProcFrameFloor    = procRate("-0.003")
	ProcFrameInterval = 8 * time.Hour
)

// ProcCandidateQty is the declared candidate: 0.02 BTC — lot-aligned on
// both legs (spot step 0.00001, perp step 0.001), the $1K gate's actual
// trade size at the era's price levels, and the smallest family-2 cell.
var ProcCandidateQty = procQty("0.02")

// ProcSurrogateStratum names the cell the frame's frozen execution cost
// is drawn from: calm — the frame holds costs at the frozen bound for the
// WHOLE era including calm hours, which §41b's rationale argues already
// overstates them; drawing the frame's constant from a stressed cell
// would double-charge stress into every historical instant.
const ProcSurrogateStratum = "calm"

// The declared C_allocated decomposition — the §3.9 ledger's shape:
// $1,100 spot principal, $300 perp initial margin (4× on the era's
// notionals with room), the interim $100 rescue margin (family 3,
// §40k-4's number, real derivation milestone 4), $50 fee/slippage
// reserve. Total $1,550.
var (
	ProcCapitalSpotPrincipal = "1100"
	ProcCapitalPerpMargin    = "300"
	ProcCapitalRescue        = "100"
	ProcCapitalReserve       = "50"
)

// --- The family-3 scoring protocol ---

var (
	// ProcRollingWindowDays: 90 — §8's "≥3-month rolling window", taken
	// at its floor; a longer window with a two-day-old capture would
	// idle the milestone for nothing.
	ProcRollingWindowDays uint32 = 90

	// ProcRegimeRule is the written negative-regime classifier, applied
	// mechanically by step 5.
	ProcRegimeRule = "negative-regime iff the sum of realized Binance BTCUSDT perp funding settlements inside the window is <= 0 (settlement instants inside, venue-published rates, exact fixedpoint sum)"

	// ProcStandbyBenchmark: §8's standby is what idle/deployable capital
	// earns — the committed reference's 13-week bill close (3.69%,
	// treasury.gov, as-of 2026-07-30), NOT the full hurdle (no venue
	// haircuts apply to capital that never goes on-venue).
	ProcStandbyBenchmark = procRate("0.0369")

	// ProcMaxDrawdownCeiling: 5% of allocated capital — the $1K gate's
	// tolerance for a strategy whose worst path is a stand-down plus two
	// leg crossings, sized ≈6× the measured round-trip cost share so an
	// ordinary exit cannot trip it while a genuinely wrong regime does.
	ProcMaxDrawdownCeiling = procRate("0.05")

	// ProcConfidenceLevel / Method: a 95% lower confidence bound via the
	// stationary block bootstrap's percentile interval — blocks because
	// funding is serially dependent; the exact block-length rule is
	// step 5's pre-registered implementation detail, named here so the
	// runner cannot choose it after seeing scores.
	ProcConfidenceLevel  = procRate("0.95")
	ProcConfidenceMethod = "stationary-block-bootstrap percentile LCB at 0.95; block length = one settlement interval per block minimum, chosen by the pre-registered Politis-White rule"

	// ProcMinUtilizationRelation: the eligibility cutoff is HALF the
	// pre-registered expectation, floored at scale 9 — a run idling at
	// less than half the expectation is not the strategy the artifact
	// priced, whatever its LCB says.
	ProcMinUtilizationRelation = "min_utilization = expected_utilization x 0.5, RoundFloor at scale 9"
)

// ProcMinUtilizationFactor is the relation's factor, applied at emit.
var ProcMinUtilizationFactor = procRate("0.5")
