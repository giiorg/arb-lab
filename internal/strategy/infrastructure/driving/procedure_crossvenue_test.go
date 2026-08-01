package driving_test

// The §45 cross-venue roster, checked against its own sources rather than
// against itself. Three of these tests exist because the roster's numbers
// were derived by hand at the procedure commit, and a hand derivation that
// only ever gets read back is an assertion, not a measurement.
//
// What is checked where, and why the split:
//
//   - Entry levels are checked against research/screening/shortlist.json,
//     which is COMMITTED. That file's best_entry field is the ruling
//     §45n-4 cites, so the roster is compared to its source on every run.
//   - Lot steps and prices live in research/screening/raw/, which is
//     git-ignored (the ledger pins their digests instead). They are pinned
//     as declared constants below WITH their provenance, the size
//     arithmetic is checked against those pins on every run, and the pins
//     themselves are verified against the venues' own bytes only when the
//     raw tree is present. That last check announces itself when it does
//     not run — a skip that says nothing is how a suite comes to look
//     complete while checking less than it did.

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

// repoRoot is this package's distance to the tree root.
const repoRoot = "../../../.."

// cvVenueFilter is one symbol's lot filter on one venue, as read from the
// venue's own ledgered response on 2026-08-01.
type cvVenueFilter struct {
	binanceStep string
	bybitStep   string

	// binanceMinNotional is the Binance row's notional floor in USDT.
	// BCHUSDT's 20 against every other row's 5 is the reason this column
	// exists — a size rule that cleared one floor and not the other would
	// look identical in the roster.
	binanceMinNotional string

	// price is the ledgered screening ticker's last price, the level the
	// size rule sized at.
	price string
}

// cvPinned is the measured venue data the size rule consumed. Provenance:
// research/screening/raw/binance/exchange_info.json (fapi/v1/exchangeInfo,
// LOT_SIZE stepSize + NOTIONAL), research/screening/raw/bybit/
// instruments.json (v5 instruments-info, lotSizeFilter qtyStep), and both
// venues' tickers.json — all four fetched 2026-08-01 and digest-pinned in
// the committed research/screening/ledger.json.
var cvPinned = map[string]cvVenueFilter{
	"LABUSDT":     {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "0.1486"},
	"BEATUSDT":    {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "4.638"},
	"KAITOUSDT":   {binanceStep: "0.1", bybitStep: "1", binanceMinNotional: "5", price: "1.0966"},
	"MMTUSDT":     {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "0.1984"},
	"MUSDT":       {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "1.0934"},
	"SOONUSDT":    {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "0.2240"},
	"ESPORTSUSDT": {binanceStep: "1", bybitStep: "1", binanceMinNotional: "5", price: "0.01834"},
	"BCHUSDT":     {binanceStep: "0.001", bybitStep: "0.01", binanceMinNotional: "20", price: "209.64"},
}

func bigRat(t testing.TB, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("SetString(%q) failed", s)
	}
	return r
}

// TestCrossVenueEntryLevelsMatchTheScreening checks ruling §45n-4 against
// the committed shortlist: each symbol's roster entry level IS the level
// the screening ranked that symbol at. Transcribing eight levels from a
// prose summary is exactly where a digit goes missing, and a wrong entry
// level does not crash — it measures a different strategy.
func TestCrossVenueEntryLevelsMatchTheScreening(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "research/screening/shortlist.json"))
	if err != nil {
		t.Fatalf("committed shortlist: %v", err)
	}
	var file struct {
		Symbols []struct {
			Symbol    string `json:"symbol"`
			BestEntry string `json:"best_entry"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("shortlist decode: %v", err)
	}
	if len(file.Symbols) != len(driving.ProcCrossVenueSymbols) {
		t.Fatalf("shortlist holds %d symbols, roster holds %d",
			len(file.Symbols), len(driving.ProcCrossVenueSymbols))
	}

	best := map[string]string{}
	for _, s := range file.Symbols {
		best[s.Symbol] = s.BestEntry
	}
	// The order is checked too: the roster walks the shortlist's ranking
	// order, and a reordered roster would silently reweight nothing today
	// but would break every artifact-to-shortlist comparison by eye.
	for i, spec := range driving.ProcCrossVenueSymbols {
		if spec.Symbol != file.Symbols[i].Symbol {
			t.Fatalf("roster[%d] is %s, shortlist[%d] is %s", i, spec.Symbol, i, file.Symbols[i].Symbol)
		}
		want, ok := best[spec.Symbol]
		if !ok {
			t.Fatalf("%s is in the roster but not the shortlist", spec.Symbol)
		}
		if got := spec.Entry.Decimal(); got != want {
			t.Fatalf("%s: roster entry %s, screening best_entry %s", spec.Symbol, got, want)
		}
	}

	// The distinct-level count is asserted so a roster that collapsed to
	// one modal level — the alternative ruling §45n-4 REJECTED — cannot
	// pass this test by matching a shortlist somebody also flattened.
	levels := map[string]int{}
	for _, spec := range driving.ProcCrossVenueSymbols {
		levels[spec.Entry.Decimal()]++
	}
	if len(levels) != 3 {
		t.Fatalf("roster spans %d distinct entry levels, want 3 (5bp/10bp/20bp): %v", len(levels), levels)
	}
}

// TestCrossVenueSizesAreLotAlignedAndOnTarget checks the stated size rule
// end to end against the pinned venue data: 250 USDT per leg, rounded UP to
// the step common to both venues, clearing each venue's notional floor.
//
// The exactness matters in both directions. Alignment alone would admit any
// multiple of the step — including one ten times too large, which walks ten
// times deeper into the book and would read as a crushing cost rather than
// an arithmetic slip. So the test also pins the size as the SMALLEST
// admissible one: one common step smaller must fall below the target.
func TestCrossVenueSizesAreLotAlignedAndOnTarget(t *testing.T) {
	target := bigRat(t, driving.ProcCrossVenueNotionalTarget)

	for _, spec := range driving.ProcCrossVenueSymbols {
		pin, ok := cvPinned[spec.Symbol]
		if !ok {
			t.Fatalf("%s: no pinned venue data", spec.Symbol)
		}
		qty := bigRat(t, spec.Qty.Decimal())
		price := bigRat(t, pin.price)
		bnStep := bigRat(t, pin.binanceStep)
		byStep := bigRat(t, pin.bybitStep)

		// Aligned on BOTH venues: qty/step is a whole number on each.
		for name, step := range map[string]*big.Rat{"binance": bnStep, "bybit": byStep} {
			q := new(big.Rat).Quo(qty, step)
			if !q.IsInt() {
				t.Fatalf("%s: qty %s is not a multiple of the %s step %s",
					spec.Symbol, spec.Qty.Decimal(), name, step.RatString())
			}
		}

		// The common step is the coarser one — checked, not assumed: this
		// only holds because each pair's coarser step is a multiple of the
		// finer, which is true of these eight and is not a law.
		common := bnStep
		if bnStep.Cmp(byStep) < 0 {
			common = byStep
		}
		finer := byStep
		if common == byStep {
			finer = bnStep
		}
		if !new(big.Rat).Quo(common, finer).IsInt() {
			t.Fatalf("%s: steps %s and %s have no common multiple among themselves",
				spec.Symbol, bnStep.RatString(), byStep.RatString())
		}

		notional := new(big.Rat).Mul(qty, price)
		if notional.Cmp(target) < 0 {
			t.Fatalf("%s: notional %s is below the %s target",
				spec.Symbol, notional.FloatString(4), driving.ProcCrossVenueNotionalTarget)
		}

		// Smallest admissible: one common step down must undershoot.
		smaller := new(big.Rat).Sub(qty, common)
		if smaller.Sign() > 0 {
			if n := new(big.Rat).Mul(smaller, price); n.Cmp(target) >= 0 {
				t.Fatalf("%s: qty %s is not the smallest aligned size clearing %s — %s also clears it",
					spec.Symbol, spec.Qty.Decimal(), driving.ProcCrossVenueNotionalTarget, smaller.RatString())
			}
		}

		// Each venue's own notional floor. Bybit's rows are all 5.
		for name, floor := range map[string]string{"binance": pin.binanceMinNotional, "bybit": "5"} {
			if notional.Cmp(bigRat(t, floor)) < 0 {
				t.Fatalf("%s: notional %s is below the %s minimum %s",
					spec.Symbol, notional.FloatString(4), name, floor)
			}
		}

		// The quantity's asset tag is the symbol's base — a Quantity
		// tagged with the wrong asset is refused at runtime by the
		// dimensional check, but only once something multiplies it.
		base := spec.Symbol[:len(spec.Symbol)-len("USDT")]
		if got := spec.Qty.Asset(); got != fixedpoint.Asset(base) {
			t.Fatalf("%s: qty asset %s, want %s", spec.Symbol, got, base)
		}
	}
}

// TestCrossVenuePinnedFiltersMatchTheVenueBytes verifies the pins above
// against the venues' own ledgered responses. research/screening/raw/ is
// git-ignored, so this cannot run everywhere; when it cannot, it says so
// rather than passing quietly.
//
// One distinction this data cannot check: Binance serves both a LOT_SIZE
// and a MARKET_LOT_SIZE filter, each carrying a stepSize, and for all eight
// of these symbols the two are IDENTICAL. So reading the wrong one would
// agree here. LOT_SIZE is selected explicitly because it is the filter the
// committed reference revision's own lot rows cite, but that choice is
// argued rather than measured against these bytes.
func TestCrossVenuePinnedFiltersMatchTheVenueBytes(t *testing.T) {
	bnPath := filepath.Join(repoRoot, "research/screening/raw/binance/exchange_info.json")
	raw, err := os.ReadFile(bnPath)
	if err != nil {
		t.Skipf("the ledgered raw tree is absent (%s is git-ignored) — the pinned steps and prices in cvPinned were NOT verified against venue bytes on this run; the size arithmetic above was", bnPath)
	}
	var file struct {
		Pages []json.RawMessage `json:"pages"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("exchange_info decode: %v", err)
	}
	var page struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Filters []struct {
				Type     string `json:"filterType"`
				StepSize string `json:"stepSize"`
				Notional string `json:"notional"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(file.Pages[0], &page); err != nil {
		t.Fatalf("exchange_info page decode: %v", err)
	}

	seen := 0
	for _, s := range page.Symbols {
		pin, ok := cvPinned[s.Symbol]
		if !ok {
			continue
		}
		seen++
		var step, notional string
		for _, f := range s.Filters {
			switch f.Type {
			case "LOT_SIZE":
				step = f.StepSize
			case "MIN_NOTIONAL":
				notional = f.Notional
			}
		}
		if bigRat(t, step).Cmp(bigRat(t, pin.binanceStep)) != 0 {
			t.Fatalf("%s: venue binance step %s, pinned %s", s.Symbol, step, pin.binanceStep)
		}
		if bigRat(t, notional).Cmp(bigRat(t, pin.binanceMinNotional)) != 0 {
			t.Fatalf("%s: venue binance min notional %s, pinned %s", s.Symbol, notional, pin.binanceMinNotional)
		}
	}
	if seen != len(cvPinned) {
		t.Fatalf("exchange_info answered for %d of the %d pinned symbols", seen, len(cvPinned))
	}
}

// TestCrossVenuePolicyIsEqualWeightOverTheRoster pins ruling §45n-2: one
// holding per walked symbol, all weights identical, summing to exactly one.
func TestCrossVenuePolicyIsEqualWeightOverTheRoster(t *testing.T) {
	policy := driving.ProcCrossVenuePolicy
	if len(policy) != len(driving.ProcCrossVenueSymbols) {
		t.Fatalf("policy holds %d, roster walks %d", len(policy), len(driving.ProcCrossVenueSymbols))
	}
	held := map[string]bool{}
	sum := fixedpoint.ZeroRate()
	for i, a := range policy {
		if a.Symbol != driving.ProcCrossVenueSymbols[i].Symbol {
			t.Fatalf("policy[%d] holds %s, roster[%d] walks %s",
				i, a.Symbol, i, driving.ProcCrossVenueSymbols[i].Symbol)
		}
		if held[a.Symbol] {
			t.Fatalf("policy repeats %s", a.Symbol)
		}
		held[a.Symbol] = true
		if c, err := a.Weight.Cmp(policy[0].Weight); err != nil {
			t.Fatalf("policy[%d] weight: %v", i, err)
		} else if c != 0 {
			t.Fatalf("policy[%d] weighs %s, policy[0] weighs %s — ruling §45n-2 is EQUAL weight",
				i, a.Weight.Decimal(), policy[0].Weight.Decimal())
		}
		var err error
		if sum, err = sum.Add(a.Weight); err != nil {
			t.Fatalf("policy sum: %v", err)
		}
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}
	if c, err := sum.Cmp(one); err != nil {
		t.Fatalf("policy sum: %v", err)
	} else if c != 0 {
		t.Fatalf("policy weighs %s, not exactly 1", sum.Decimal())
	}
}

// TestCrossVenueSimParamsAssemble checks the assembly the runner consumes:
// it validates, and it refuses to be built with the fees left out — the
// zero-fee case is a free round trip, which is the flattering direction.
func TestCrossVenueSimParamsAssemble(t *testing.T) {
	bn, err := fixedpoint.ParseRate("0.0005")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}
	by, err := fixedpoint.ParseRate("0.00055")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}
	p := driving.ProcCrossVenueSimParams(bn, by)
	if err := p.Validate(); err != nil {
		t.Fatalf("assembled procedure does not validate: %v", err)
	}
	if p.Label != driving.ProcCrossVenueLabel {
		t.Fatalf("label %q, want %q", p.Label, driving.ProcCrossVenueLabel)
	}
	if !p.VoidImbalance {
		t.Fatal("VoidImbalance is off — ruling §45n-3 declares it on for the gate")
	}
	if c, err := p.BinanceFee.Cmp(bn); err != nil || c != 0 {
		t.Fatalf("binance fee %s did not reach the procedure (err %v)", p.BinanceFee.Decimal(), err)
	}
	if c, err := p.BybitFee.Cmp(by); err != nil || c != 0 {
		t.Fatalf("bybit fee %s did not reach the procedure (err %v)", p.BybitFee.Decimal(), err)
	}

	// The assembly copies: a caller mutating the returned slices must not
	// reach the package-level roster every later run reads.
	p.Symbols[0].Symbol = "MUTATED"
	if driving.ProcCrossVenueSymbols[0].Symbol == "MUTATED" {
		t.Fatal("the assembly aliases the roster — one run could rewrite the next run's procedure")
	}
}
