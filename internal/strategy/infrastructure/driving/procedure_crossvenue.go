package driving

// The §45 procedure parameters — strategy #2's cross-venue gate, chosen and
// recorded AT the procedure commit, which is this file's. procedure.go's
// rule applies here whole: the gate run embeds this commit's hash and the
// executable's digest in its artifact, so editing any value below after
// seeing a number is a visibly different artifact — a new version and a
// complete rerun.
//
// The ordering rule is stronger here than it was for the calibration
// artifact, and it is enforced rather than remembered: ruling §45f-2 seals
// post-cutoff evidence in code (crossvenue/seal.go), and the ONLY door
// through that seal is RunGateSymbols, which the gate runner calls. So every
// value below was necessarily pinned before any post-cutoff byte was read —
// not because the operator was disciplined, but because no other code path
// could have read one.
//
// # What informed these, and what did not
//
// Informed (ruling §45f-2's labeled-and-logged trail): the §43 screening's
// committed shapes, the one logged pre-cutoff exploration run
// (research/crossvenue/sims/s61-smoke-btc.json), the venues' own ledgered
// exchange_info/instruments and tickers files, and §41's calibration roster
// where the same question already had an answer.
//
// NOT informed by: any post-cutoff measurement, which is what the seal makes
// structural. Depth-weighted allocation was ruled OUT for exactly this
// reason (§45n-2) — measured alt depth is post-cutoff evidence, so a policy
// keyed on it could not have been declared here.
//
// # Two numbers are deliberately NOT in this file
//
// The per-venue taker fees and the hurdle are READ from the committed
// reference revision, not pinned here. Core §2.4 forbids hardcoded fee
// constants and §2.1 forbids a hardcoded hurdle in the same breath — both
// must arrive as dated, sourced, revision-controlled inputs read as-of the
// decision instant. The reference document already carries exactly the two
// cells this gate needs, and pinning copies of them here would create the
// second copy §2.4's "no second copy of the number in another module"
// names. ProcCrossVenueReference below names the revision instead.

import (
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/crossvenue"
)

// procCVQty parses a per-symbol candidate size; a malformed constant is a
// programmer error caught by the first test that touches the roster.
func procCVQty(asset, s string) fixedpoint.Quantity {
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), s)
	if err != nil {
		panic(err)
	}
	return q
}

// --- The shortlist, its entry levels and its sizes ---

// ProcCrossVenueSymbols is the walked roster: the §43 shortlist, each symbol
// at its own committed entry level and its own lot-aligned candidate size.
//
// # Entry levels (ruling §45n-4)
//
// Per symbol, each the level at which the screening ranked that symbol —
// read from the ledgered research/screening/shortlist.json `best_entry`
// field, not transcribed from prose: KAITO at 5bp, LAB/MMT/M/BCH at 10bp,
// BEAT/SOON/ESPORTS at 20bp. A single modal level was rejected because it
// would measure a different strategy from the one the ranking evidenced,
// and the mismatch is not conservative in either direction — it churns
// round trips on the 5bp name and under-triggers the 20bp ones.
//
// # Sizes: one stated rule, applied identically to all eight
//
// The rule: 250 USDT of notional per LEG, at the §43-run price levels,
// rounded UP to the lot step common to BOTH venues. Every term of that has
// a reason.
//
//   - 250 per leg, eight symbols, two legs each is 4,006 USDT of gross
//     notional at these sizes; at ProcPerpLeverage (4x, procedure.go's
//     pinned figure) that is 1,001 USDT of margin — §8's $1K tier, which
//     is the tier this gate scores. The size is not a free parameter; it
//     is the tier divided by the roster.
//   - Rounded UP, never nearest: a larger order walks DEEPER into the
//     book, so the measured slippage it produces is higher. Rounding up
//     overstates the cost of the trade the strategy would actually place,
//     which is the adverse direction.
//   - Lot-aligned on BOTH venues because the position is one trade with two
//     legs — a size only one venue can fill is not a size. The steps were
//     read 2026-08-01 from the ledgered raw files (Binance
//     fapi/v1/exchangeInfo LOT_SIZE, Bybit v5 instruments-info qtyStep),
//     and they DIFFER on two names: KAITO is 0.1 on Binance and 1 on
//     Bybit, BCH is 0.001 on Binance and 0.01 on Bybit. On both the coarser
//     step is a multiple of the finer, so the common step is the coarser;
//     the alignment therefore binds on Bybit for KAITO and BCH and is
//     slack elsewhere. Binance's BCHUSDT row also carries a 20 USDT
//     minimum notional against every other row's 5 — the 251.57 USDT this
//     rule sizes clears it, checked rather than assumed.
//
// The prices are the ledgered screening tickers' last prices (LAB 0.1486,
// BEAT 4.638, KAITO 1.0966, MMT 0.1984, M 1.0934, SOON 0.2240, ESPORTS
// 0.01834, BCH 209.64). They are pre-cutoff by construction, so using them
// costs nothing against the seal; they enter the size and nowhere else.
var ProcCrossVenueSymbols = []crossvenue.SymbolSpec{
	{Symbol: "LABUSDT", Qty: procCVQty("LAB", "1683"), Entry: procRate("0.001")},
	{Symbol: "BEATUSDT", Qty: procCVQty("BEAT", "54"), Entry: procRate("0.002")},
	{Symbol: "KAITOUSDT", Qty: procCVQty("KAITO", "228"), Entry: procRate("0.0005")},
	{Symbol: "MMTUSDT", Qty: procCVQty("MMT", "1261"), Entry: procRate("0.001")},
	{Symbol: "MUSDT", Qty: procCVQty("M", "229"), Entry: procRate("0.001")},
	{Symbol: "SOONUSDT", Qty: procCVQty("SOON", "1117"), Entry: procRate("0.002")},
	{Symbol: "ESPORTSUSDT", Qty: procCVQty("ESPORTS", "13632"), Entry: procRate("0.002")},
	{Symbol: "BCHUSDT", Qty: procCVQty("BCH", "1.20"), Entry: procRate("0.001")},
}

// ProcCrossVenueNotionalTarget records the size rule's input in the form the
// rule states it, so the eight quantities above can be re-derived from this
// file rather than believed. Decimal USDT per leg.
const ProcCrossVenueNotionalTarget = "250"

// ProcCrossVenuePolicy is the pre-declared allocation policy (ruling
// §45f-3's pass shape): EQUAL weight over the shortlist, one eighth each
// (ruling §45n-2). Weights sum to exactly one at scale 9 — eight times
// 0.125 with no remainder, so the policy validator's exact-sum rule is met
// by construction rather than by a rounding convention.
//
// Rank-proportional weighting was rejected because it would import the
// pre-cutoff selection's magnitudes into the verdict statistic — LABUSDT's
// 71.6%/yr is the single most selection-contaminated number in the record,
// and a rank-proportional policy puts over half the portfolio behind it.
// Equal weight imports the SELECTION (which §43b states openly and §45f-4
// carries as a named holdout input) without also importing the ordering.
var ProcCrossVenuePolicy = func() []dcv.Allocation {
	out := make([]dcv.Allocation, 0, len(ProcCrossVenueSymbols))
	for _, s := range ProcCrossVenueSymbols {
		out = append(out, dcv.Allocation{Symbol: s.Symbol, Weight: procRate("0.125")})
	}
	return out
}()

// --- The evidence rules ---

// ProcCrossVenueMatchedFloor is ruling §45f-1's end rule: a shortlisted
// symbol is scored once it has 180 MATCHED post-cutoff settlements — about
// thirty days on the 4h grid seven of the eight settle at, and about sixty
// on BCH's 8h grid. It is the screening's own ≥180 evidence floor
// (research/screening/ledger.json min_matched_settlements), reused so the
// gate demands of each symbol exactly what the ranking demanded of it.
//
// Matched, never elapsed: a calendar trigger would let a capture outage
// leave a symbol thin while the date said go, and the gate's whole claim is
// that the trigger is computable from the evidence rather than declared by
// whoever is holding the numbers.
const ProcCrossVenueMatchedFloor = 180

// ProcCrossVenueVoidImbalance turns on ruling §45n-3's episode-integrity
// rule: an episode whose two legs settled unequal counts inside its window
// is refused by name and kept out of the return series. On same-grid pairs
// an imbalanced end usually marks missing settled rows or a venue interval
// change rather than economics, and scoring over suspect settlement data is
// the wrong default to hold.
const ProcCrossVenueVoidImbalance = true

// ProcCrossVenueRefusalCeiling is the other half of ruling §45n-3: a symbol
// more than a quarter of whose episodes were refused is refused BY NAME and
// leaves the portfolio before any statistic exists.
//
// A quarter, and why not looser: refusals are not a random sample of a
// symbol's episodes. An episode is refused when a boundary book is missing,
// thin or unmeasurable — and books go missing during stalls, reconnects and
// venue trouble, which are precisely the instants when crossing the spread
// costs most. So the surviving set is biased toward calm episodes, and the
// bias runs in the flattering direction. A quarter is where that bias stops
// being noise; a symbol scored on three-quarters of its episodes is still
// recognisably the symbol, and one scored on half is not.
//
// The void rule above feeds this same counter deliberately: an integrity
// void and a missing book are different faults with the same consequence —
// an episode that is not in the series — and a ceiling that counted only one
// of them would let the other thin the sample silently.
var ProcCrossVenueRefusalCeiling = procRate("0.25")

// --- The observation procedure ---

// ProcCrossVenueSample is the committed decision-point sampling procedure: a
// one-minute epoch grid with ten seconds of staleness slack.
//
// One minute because both venues' funding-estimate streams tick faster than
// that — Binance's markPrice at 1s, Bybit's tickers on change — so a minute
// grid DECIMATES rather than interpolates (ProcImminent's rule, same
// reason). Ten seconds of slack because an estimate older than that is not
// what the venue was showing at the grid instant.
//
// The slack's failure mode is visible rather than silent, which is what
// makes ten seconds safe to commit to: too tight, and grid instants are
// SKIPPED and counted per symbol in the artifact's evidence block, so a
// symbol whose stream was too sparse for this slack shows up as a coverage
// number rather than as a quiet market. That reporting is why the value
// did not need to be tuned against post-cutoff data to be defensible.
var ProcCrossVenueSample = crossvenue.SampleParams{
	Every: time.Minute,
	Slack: 10 * time.Second,
}

// ProcCrossVenueMaxGap closes an open episode when the decision series jumps
// more than ten minutes — ten sample instants at the cadence above. A hole
// that long is evidence we do not have, and holding a position across it in
// the model would be claiming a signal nobody observed. Ten rather than two
// or three because a brief reconnect is ordinary and should not manufacture
// a round trip's cost out of a data hiccup; ten minutes is past the point
// where a reconnect explains it.
const ProcCrossVenueMaxGap = 10 * time.Minute

// ProcCrossVenueExitDivisor is the hysteresis exit: the episode closes when
// the signal falls below entry/2. The screening's committed exit_divisor,
// reused unchanged, so the gate's episode boundaries are the same rule the
// ranking used.
const ProcCrossVenueExitDivisor = 2

// ProcCrossVenueRound is the settlement pairing bucket: one minute, the
// screening's committed pairing_round. It absorbs Binance's millisecond
// settlement jitter without merging two genuinely distinct settlements —
// the two venues' 4h grids are hours apart in phase, never minutes.
const ProcCrossVenueRound = time.Minute

// ProcCrossVenueP90 is the conditional quantile the measured cost
// distributions are read at: §8's own letter, p90 slippage — the same value
// ProcFillQuantile pins for strategy #1.
var ProcCrossVenueP90 = procRate("0.9")

// --- The verdict statistic ---

// ProcCrossVenuePeriod is the shared return grid's step: four hours.
//
// Four because seven of the eight symbols settle on the 4h grid, so a
// period is one settlement for most of the roster, and BCH's 8h grid is an
// exact multiple rather than a ragged fit. Over the ~30 days the end rule
// requires, that is ~180 periods — enough for the block bootstrap below to
// have blocks to resample, where a daily period would leave ~30.
//
// The grid is what makes utilization part of the measured number rather
// than an adjustment applied to it: every period a symbol was not in
// position contributes an exact zero, so the mean per-period return is
// already the utilization-adjusted figure §8 asks for. Nothing downstream
// multiplies by a utilization fraction, because doing that to this series
// would charge idleness twice.
const ProcCrossVenuePeriod = 4 * time.Hour

// ProcCrossVenueMeanBlock is the stationary bootstrap's mean block length in
// PERIODS: six, one day at the period above.
//
// Funding is serially dependent and the episode machine makes it more so —
// the hysteresis exit means an open position persists until the signal
// halves, so consecutive periods inside one episode are not independent
// draws. A block must therefore span more than one settlement; six periods
// spans six settlements on the 4h grid and three on BCH's, comfortably past
// §41's stated "one settlement interval per block minimum".
const ProcCrossVenueMeanBlock = 6

// ProcCrossVenueResamples is the bootstrap's resample count: 2000. At the
// confidence below, the bound is selected at the floored rank
// 0.05 x 1999 = 99 — the 100th smallest resample mean, which is a real
// order statistic with resamples either side of it. The pre-cutoff smoke ran
// 500, where the same rank is 24 and a single unlucky path moves the bound
// visibly.
const ProcCrossVenueResamples = 2000

// ProcCrossVenueConfidence is §8's letter again: the LCB at 95%, not the
// point estimate. ProcConfidenceLevel pins the same value for strategy #1's
// scoring protocol, and the two gates are held to one governance.
var ProcCrossVenueConfidence = procRate("0.95")

// ProcCrossVenueSeed pins the resampling stream. The VALUE carries no
// information — SplitMix64 is as good from one seed as another — so what
// matters is only that it was fixed before any post-cutoff read, and the
// number is the declaration's own date so that fact is checkable from the
// record rather than asserted. A seed chosen after seeing a bound is a
// bound chosen, which is the one thing the pre-registration exists to stop.
const ProcCrossVenueSeed uint64 = 20260801

// --- The read, not pinned, inputs ---

// ProcCrossVenueReference names the committed reference revision the gate
// reads its per-venue taker fees and its hurdle benchmark from, as-of the
// declared evidence bound (§2.4's as-of composition with §3.5).
//
// It is the BTCUSDT-named file because that is the one committed revision,
// and its rows answer this gate correctly: FeeRow is keyed on (venue,
// product) with no symbol column — deliberately, because venue base-tier
// fee schedules are schedule-wide — so the binance/linear_perp and
// bybit/linear_perp cells (5bp and 5.5bp taker, the published base-tier
// numbers behind the screening's 21bp two-venue round trip) are the fees
// for any linear perp on those venues, alt or not. The hurdle row is a
// capital benchmark and has no symbol dimension at all.
//
// What the file does NOT answer for is lot rules: its four LotRow entries
// are BTCUSDT's. The gate never asks — the shortlist's lot alignment is
// baked into ProcCrossVenueSymbols above, at sizes that were aligned against
// the venues' own ledgered filters when the roster was pinned.
const ProcCrossVenueReference = "configs/reference.btcusdt.json"

// ProcCrossVenueDenominator states the capital base the gate's return is
// reported on, and why that choice cannot flatter.
//
// The per-episode returns the walker produces are rates on the per-leg
// NOTIONAL: funding accrues on notional and the round trip is charged on it
// across both legs. The gate annualizes their mean per-period value and
// compares that against the hurdle directly — it applies NO leverage
// uplift, so no leverage figure appears anywhere in the verdict arithmetic.
//
// §2.1 requires the report to be on C_allocated, and permits another
// denominator on one condition: that it is DECLARED and demonstrably at
// least C_allocated (the clause landed v0.9.23 with this gate — the
// prohibition it qualifies names only smaller denominators, because the
// risk it bounds is a flattered return). This constant is that
// declaration, and it is carried into every artifact this gate writes.
//
// The demonstration: at ProcPerpLeverage two legs post
// notional/2 in margin, and C_allocated adds only the rescue margin and the
// fee/slippage reserve on top, so C_allocated at these sizes sits BELOW the
// per-leg notional. Dividing by the larger number understates the return,
// which makes the hurdle comparison harder than §2.1 demands rather than
// easier — the adverse direction, stated here so a later reader does not
// "correct" it into a 2x uplift.
const ProcCrossVenueDenominator = "annualized mean per-period return on the per-leg notional; no leverage uplift applied, so the figure understates return on C_allocated (which is strictly smaller at ProcPerpLeverage) — the adverse direction, §2.1 satisfied a fortiori"

// ProcCrossVenueHoldout is ruling §45f-4's holdout declaration, recorded in
// the artifact: the clean window IS the promotion sample, and the holdout
// discipline is the end rule plus §43b's selection statement carried as a
// named input rather than a second split of an already-scarce sample.
const ProcCrossVenueHoldout = "ruling §45f-4: the post-cutoff clean window IS the promotion sample. Selection contamination is DECLARED, not eliminated — the eight symbols were chosen by the §43 screening over pre-cutoff settled funding (§43b), so this gate measures a pre-selected roster and says so. The holdout protections are structural: ruling §45f-2 seals post-cutoff evidence in code so every parameter was pinned before any of it could be read, and ruling §45f-1's end rule fixes the scoring trigger to a computable evidence count rather than an operator-chosen date."

// ProcCrossVenueSimParams assembles the roster into the walker's argument.
// One assembly, so the gate runner cannot compose a procedure that differs
// from the one this file declares — the label names the commit's own
// procedure rather than an operator's run name.
//
// The two fees are PARAMETERS of this function rather than fields of the
// roster, and that is the §2.4 rule made structural: they are the values
// the runner read from the committed reference revision as-of the evidence
// bound, so the assembly cannot be completed without having done that read.
// A roster that carried its own fee fields would assemble fine with them
// left at zero — and a zero taker fee is a free round trip, the flattering
// direction, computing the gate in our favour with nothing failing.
func ProcCrossVenueSimParams(binanceTaker, bybitTaker fixedpoint.Rate) crossvenue.SimParams {
	return crossvenue.SimParams{
		BinanceFee:    binanceTaker,
		BybitFee:      bybitTaker,
		Symbols:       append([]crossvenue.SymbolSpec(nil), ProcCrossVenueSymbols...),
		Sample:        ProcCrossVenueSample,
		ExitDivisor:   ProcCrossVenueExitDivisor,
		MaxGap:        ProcCrossVenueMaxGap,
		VoidImbalance: ProcCrossVenueVoidImbalance,
		Round:         ProcCrossVenueRound,
		P90:           ProcCrossVenueP90,
		Period:        ProcCrossVenuePeriod,
		Policy:        append([]dcv.Allocation(nil), ProcCrossVenuePolicy...),
		MeanBlock:     ProcCrossVenueMeanBlock,
		Resamples:     ProcCrossVenueResamples,
		Confidence:    ProcCrossVenueConfidence,
		Seed:          ProcCrossVenueSeed,
		Label:         ProcCrossVenueLabel,
	}
}

// ProcCrossVenueLabel names the committed procedure in every artifact it
// produces.
const ProcCrossVenueLabel = "crossvenue-gate-v1"
