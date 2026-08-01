package screening

// The §42b screening parameter roster — chosen and recorded AT the
// harness's own commit, the §41e pattern: the scoping record (§42)
// deliberately names no thresholds, so THIS file is where each value is
// chosen, and the rationale for each lives beside it. The screening is a
// ranking, not the gate — strategy #2's gate re-derives its own economics
// at its own procedure commit (§42e) — so each choice here errs toward
// conservatism in what it lets onto the shortlist, never toward rescuing
// a candidate.

import (
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// scrRate parses a roster rate constant; a malformed constant is a
// programmer error caught by the first test that touches the roster.
func scrRate(s string) fixedpoint.Rate {
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		panic(err)
	}
	return r
}

// scrUSDT parses a roster USDT amount, same contract as scrRate.
func scrUSDT(s string) fixedpoint.Money {
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		panic(err)
	}
	return m
}

// --- The data window ---

// CutoffUTC ends the screening window: only settlements STRICTLY BEFORE
// this instant are fetched or read. Pinned to the screening date's own
// midnight so the window is complete days, and so the forward capture
// this step triggers (§42c) starts after everything the ranking saw —
// the shortlist is selected on pre-cutoff data only, which is what lets
// the gate treat post-cutoff data as selection-clean. Pre-cutoff history
// is correspondingly selection-CONTAMINATED for gate purposes, and the
// screening record says so.
var CutoffUTC = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

// HistoryStart bounds the walk on the old side. 2019-01-01 predates both
// venues' USDT-perp funding history (Binance USDⓈ-M launched late 2019),
// so nothing real is excluded; the constant exists so a venue bug echoing
// epoch-zero rows cannot make a walk unbounded.
var HistoryStart = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

// ExcludedSymbols are intersection symbols the screening refuses to
// fetch, each with the reason the ledger records. BTCUSDT: §42e makes
// the H1 holdout (post-2026-02-01 BTC) permanent, and a screening read
// of BTC funding through the cutoff would cross it — v1's instrument
// stays sealed even though it would otherwise qualify for the universe.
var ExcludedSymbols = map[string]string{
	"BTCUSDT": "§42e H1 seal: post-2026-02-01 BTC stays untouched, permanently",
}

// --- Pairing ---

// PairingRound is the tolerance for matching the two venues' published
// settlement instants: each instant is rounded to the NEAREST multiple
// of this before comparison. Binance publishes with millisecond jitter
// (profiles §7: 8 ms observed past the grid instant); Bybit publishes
// exact. One minute absorbs any plausible jitter while staying far
// below the smallest funding interval (1 h), so it cannot merge two
// distinct settlements — and a venue that DID settle twice within one
// rounded minute is dirty data, which pairing drops and counts rather
// than silently keeping either row.
const PairingRound = time.Minute

// MaxGapFactor bounds what "consecutive" means in a matched series: two
// matched settlements are adjacent only if their spacing is at most
// this factor times the pair's modal interval. An episode may not
// accrue across a wider hole (the hole is unmatched settlements —
// evidence we do NOT have that the spread persisted), and the census
// splits events on the same rule.
const MaxGapFactor = 2

// --- The cost side (§42b step 3: four perp taker legs) ---

// Four-leg round trip: enter and exit a two-venue perp-perp position at
// taker on both venues. Base-tier published cells, venue-profiles §280
// block (observed 2026-07-31): Binance USDⓈ-M taker 0.0500%, Bybit
// linear-perp taker 0.0550%. Maker entry stays a tactic to price at the
// gate (§42e), never an assumption here — the screening charges the
// worst published path.
var (
	// BinanceTakerFee is one Binance USDⓈ-M perp taker leg.
	BinanceTakerFee = scrRate("0.0005")

	// BybitTakerFee is one Bybit linear-perp taker leg.
	BybitTakerFee = scrRate("0.00055")
)

// RoundTripCost is the four-leg total: 2× each venue's taker fee,
// as a fraction of one leg's notional (the two legs' notionals are
// equal by construction in a perp-perp spread).
func RoundTripCost() fixedpoint.Rate {
	twoLegsBinance, err := BinanceTakerFee.Add(BinanceTakerFee)
	if err != nil {
		panic(err)
	}
	twoLegsBybit, err := BybitTakerFee.Add(BybitTakerFee)
	if err != nil {
		panic(err)
	}
	rt, err := twoLegsBinance.Add(twoLegsBybit)
	if err != nil {
		panic(err)
	}
	return rt
}

// --- The episode model (M1) ---

// EntryLevels is the entry-trigger grid: an episode opens at a matched
// settlement whose |spread| is at or above a level. Four points from 2bp
// to 20bp — below 2bp a spread cannot plausibly amortize the 21bp round
// trip inside any realistic run, and above 20bp a single print nearly
// pays the trip. The ranking takes each pair's best level and REPORTS
// which it was; picking the best of four is mild selection and the
// screening record carries that caveat rather than hiding the grid.
var EntryLevels = []fixedpoint.Rate{
	scrRate("0.0002"),
	scrRate("0.0005"),
	scrRate("0.001"),
	scrRate("0.002"),
}

// ExitDivisor sets the hysteresis exit level at entry/ExitDivisor: an
// open episode holds while the spread keeps its sign and |spread| stays
// at or above E/2, and closes on the first print below that (or on a
// sign flip, a gap, or the end of data). Half the entry level is the
// classic hysteresis midpoint: exit-at-entry would churn episodes on
// noise around E and re-charge the round trip each time; exit-at-zero
// would hold through prints that no longer pay either venue's fee.
// Integer divisor so the halving is exact at every grid point.
const ExitDivisor = 2

// --- Eligibility for the ranking ---

// MinMatchedSettlements is the ranking floor on evidence: a pair with
// fewer matched settlements (180 ≈ 30 days at the 4h majority grid) can
// produce a large annualized number from a handful of prints, and §42a's
// listing-date honesty says a thin history supports only a weak
// persistence claim. Thin pairs stay in the census; they just cannot
// rank.
const MinMatchedSettlements = 180

// FloorTurnover24h is the tradability floor (§42b step 4), applied to
// BOTH venues' published 24h quote turnover at fetch time: a pair we
// could never deploy real size on is screening noise, and the floor is
// what makes the later §42c depth measurement meaningful. 5M USDT/day
// is deliberately permissive — the floor exists to drop dead markets,
// not to pre-empt the depth measurement's job. Volume is the only floor
// input: open interest would cost one authenticated-shape call per
// symbol on Binance, and the shortlist's real deployability evidence is
// the §42c book capture itself. Bybit's openInterestValue arrives free
// in the same tickers call and is recorded as evidence, not gated on.
var FloorTurnover24h = scrUSDT("5000000")

// ShortlistSize is ruling §42f-1: ~8 symbols get the four-stream
// capture tier.
const ShortlistSize = 8

// --- The dislocation census (§42b-1) ---

// CensusLevel defines a dislocation print: |spread| at or above 25bp at
// one matched settlement — larger than the whole 21bp round trip, so a
// single such print nearly pays the trip on its own, and 2.8× the
// BTC era's all-time |spread| maximum (§41p: 8.8bp), so nothing
// steady-state qualifies. The census records every event over the WHOLE
// intersection (mismatched-grid pairs included, matched at coincident
// instants), because §42f-2 prices the broad tier from this table.
var CensusLevel = scrRate("0.0025")

// --- Fetch mechanics ---

// Page limits are each venue's documented maximum for the funding
// endpoint (profiles §7); MaxFetchPages is the runaway backstop, far
// above any real walk (three years of 1h settlements at 200/page is
// ~132 pages).
const (
	BinancePageLimit = 1000
	BybitPageLimit   = 200
	MaxFetchPages    = 10_000
)
