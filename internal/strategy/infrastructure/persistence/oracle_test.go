package persistence_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

const committedSummary = "../../../../research/persistence/summary.json"

// summaryFile is summary.json's shape, decoded for use as an ORACLE only.
// Nothing in the package's production code decodes this file; record §103 §3.1
// records why, and the short form is that three of its four blocks are computed
// at a pinned coordinate they do not write down.
type summaryFile struct {
	Costs  []string `json:"costs"`
	Levels []string `json:"levels"`
	Pooled []struct {
		Entry     string            `json:"entry"`
		Episodes  int               `json:"episodes"`
		RunMean   string            `json:"run_mean"`
		RunP50    int               `json:"run_p50"`
		RunP75    int               `json:"run_p75"`
		RunP90    int               `json:"run_p90"`
		RunMax    int               `json:"run_max"`
		MeanGross string            `json:"mean_gross"`
		Wins      map[string]int    `json:"wins"`
		MeanNet   map[string]string `json:"mean_net"`
	} `json:"pooled"`
}

func loadSummary(t *testing.T) summaryFile {
	t.Helper()
	data, err := os.ReadFile(committedSummary)
	if err != nil {
		t.Fatalf("read %s: %v", committedSummary, err)
	}
	var s summaryFile
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("decode %s: %v", committedSummary, err)
	}
	return s
}

// roundedTo9 renders a Python Decimal string at the canonical scale, half-even.
//
// summary.json publishes values at up to 28 decimal places — mean_gross at
// level 0.0002 is 0.0006533427008602499594221717254, straight out of Python's
// Decimal. fixedpoint.Scale is 9. So this comparison is "agrees when the Python
// value is rounded to 9 places under the declared mode", and NOT byte equality.
// An assertion that passes must not imply one was available.
func roundedTo9(t *testing.T, s string) string {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("not a decimal: %q", s)
	}
	// big.Rat.FloatString rounds half away from zero, not half-even. The
	// artifact's values are irrational-looking quotients and none of them lands
	// on a tie at the ninth place — verified when this test was written — so the
	// two modes agree here. If a future artifact hits a tie this helper will
	// disagree with fixedpoint, and that disagreement is the signal, not noise.
	return trimZeros(r.FloatString(fixedpoint.Scale))
}

// roundedTo2 renders a decimal string at two places, half away from zero —
// the same rounding summarise.py:110's tot.quantize(D("0.01")) performs under
// Python's default decimal context (ROUND_HALF_EVEN). None of the 168
// per-symbol net figures lands on an exact half-cent tie — verified when this
// test was written, the same way roundedTo9 verifies its own ninth place — so
// the two rounding modes agree here and this is not a byte-equality claim.
func roundedTo2(t *testing.T, s string) string {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("not a decimal: %q", s)
	}
	return r.FloatString(2)
}

func trimZeros(s string) string {
	if !containsDot(s) {
		return s
	}
	i := len(s)
	for i > 0 && s[i-1] == '0' {
		i--
	}
	if i > 0 && s[i-1] == '.' {
		i--
	}
	return s[:i]
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

// The oracle. The Go fold and summarise.py are two independent implementations
// of the same arithmetic; this measures whether they agree on the committed
// data. Where they do not, the disagreement is NAMED and its magnitude is
// asserted — a tolerance would turn a finding into noise.
func TestOracle_PooledAgreesWithSummariseDotPy(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	if len(sum.Pooled) != 4 {
		t.Fatalf("summary.json has %d pooled rows, want 4", len(sum.Pooled))
	}
	// The other declared axis: 3 costs, so wins and mean_net are each checked
	// at 4 levels x 3 costs = 12 points. An empty Costs slice would make the
	// inner loop below run zero times per level without this test noticing.
	if len(sum.Costs) != 3 {
		t.Fatalf("summary.json has %d costs, want 3", len(sum.Costs))
	}

	for _, row := range sum.Pooled {
		t.Run(row.Entry, func(t *testing.T) {
			at := persistence.EntryAt{Entry: mustRate(t, row.Entry)}
			rl, err := set.RunLength(at)
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}

			// Percentiles and max agree EXACTLY at every level, censored
			// records included or excluded — five in 12 322 cannot move one.
			if rl.P50 != row.RunP50 || rl.P75 != row.RunP75 ||
				rl.P90 != row.RunP90 || rl.Max != row.RunMax {
				t.Errorf("percentiles %d/%d/%d max %d, artifact %d/%d/%d max %d",
					rl.P50, rl.P75, rl.P90, rl.Max,
					row.RunP50, row.RunP75, row.RunP90, row.RunMax)
			}

			// F103's first magnitude: the artifact's episode count includes the
			// censored records; this reader's does not, and the two reconcile.
			if rl.Episodes+rl.Censored != row.Episodes {
				t.Errorf("episodes %d + censored %d != artifact %d",
					rl.Episodes, rl.Censored, row.Episodes)
			}

			for _, cost := range sum.Costs {
				npe, err := set.NetPerEpisode(persistence.CostAt{EntryAt: at, Cost: mustRate(t, cost)})
				if err != nil {
					t.Fatalf("NetPerEpisode at %s: %v", cost, err)
				}
				// Wins agrees exactly at every level and every cost: a censored
				// record accrues "0" and clears nothing.
				if npe.Wins != row.Wins[cost] {
					t.Errorf("cost %s: wins %d, artifact %d", cost, npe.Wins, row.Wins[cost])
				}
				if rl.Censored == 0 && npe.MeanNet.Decimal() != roundedTo9(t, row.MeanNet[cost]) {
					t.Errorf("cost %s: mean_net %s, artifact (to 9) %s",
						cost, npe.MeanNet.Decimal(), roundedTo9(t, row.MeanNet[cost]))
				}
			}

			npe, err := set.NetPerEpisode(persistence.CostAt{EntryAt: at, Cost: mustRate(t, sum.Costs[0])})
			if err != nil {
				t.Fatalf("NetPerEpisode: %v", err)
			}
			if rl.Censored == 0 {
				if npe.MeanGross.Decimal() != roundedTo9(t, row.MeanGross) {
					t.Errorf("mean_gross %s, artifact (to 9) %s",
						npe.MeanGross.Decimal(), roundedTo9(t, row.MeanGross))
				}
			}
		})
	}
}

// F103's magnitude, asserted rather than tolerated. At level 0.0002 the two
// implementations DISAGREE, in three places and by exactly this much, because
// summarise.py counts five right-censored records as episodes of length zero.
// Recording the delta is what stops a later reader from either "fixing" the Go
// fold or widening the oracle until the finding disappears.
func TestOracle_CensoringIsTheOnlyDisagreement(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	var row = struct {
		episodes  int
		runMean   string
		meanGross string
	}{}
	for _, p := range sum.Pooled {
		if p.Entry == "0.0002" {
			row.episodes, row.runMean, row.meanGross = p.Episodes, p.RunMean, p.MeanGross
		}
	}
	if row.episodes != 12322 {
		t.Fatalf("artifact episodes at 0.0002 = %d, want 12322", row.episodes)
	}

	rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0002")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if rl.Episodes != 12317 || rl.Censored != 5 {
		t.Fatalf("episodes/censored = %d/%d, want 12317/5", rl.Episodes, rl.Censored)
	}
	// The artifact publishes run_mean 2.433 over 12 322; this reader reads
	// 2.433628319 over 12 317, which renders as 2.434 at three places. Both are
	// correct answers to different questions. (Verified independently: the
	// exact fraction is 29975/12317; a Fraction/Decimal fold over
	// research/persistence/episodes.json at 50-digit precision, rounded
	// half-even to 9 places, gives 2.433628319 — the task brief's own literal
	// here, 2.433628318, was off by one in the ninth place and is corrected.)
	if rl.Mean.Decimal() != "2.433628319" {
		t.Errorf("mean = %s, want 2.433628319 (which renders as 2.434 at three places)", rl.Mean.Decimal())
	}
	if row.runMean != "2.433" {
		t.Errorf("artifact run_mean = %s, want 2.433", row.runMean)
	}

	npe, err := set.NetPerEpisode(persistence.CostAt{
		EntryAt: persistence.EntryAt{Entry: mustRate(t, "0.0002")},
		Cost:    mustRate(t, "0.0021"),
	})
	if err != nil {
		t.Fatalf("NetPerEpisode: %v", err)
	}
	if npe.MeanGross.Decimal() != "0.000653608" {
		t.Errorf("mean_gross = %s, want 0.000653608", npe.MeanGross.Decimal())
	}
	if roundedTo9(t, row.meanGross) != "0.000653343" {
		t.Errorf("artifact mean_gross (to 9) = %s, want 0.000653343", roundedTo9(t, row.meanGross))
	}
}

// THE NEGATIVE CONTROL, required by record §103 §6.1 and by
// research/persistence/README.md's control C: "an agreement that was never
// shown capable of failing is the repo's standing trap."
//
// The same comparison at the WRONG level must fail. If it passes, the oracle is
// comparing something that does not depend on the level argument at all.
func TestOracle_NegativeControl_WrongLevelDisagrees(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	byEntry := map[string]int{}
	for _, p := range sum.Pooled {
		byEntry[p.Entry] = p.Episodes
	}
	// Every artifact row's episode count, against every OTHER level's fold.
	// Episodes is the separating member: 12322/3425/1259/472 are four distinct
	// values, where p75 is 3 at all four levels and separates nothing.
	for _, mine := range sum.Levels {
		rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, mine)})
		if err != nil {
			t.Fatalf("RunLength at %s: %v", mine, err)
		}
		got := rl.Episodes + rl.Censored
		for _, theirs := range sum.Levels {
			if theirs == mine {
				continue
			}
			if got == byEntry[theirs] {
				t.Errorf("fold at %s (%d episodes) matches the artifact row for %s; "+
					"the oracle cannot detect a level mix-up", mine, got, theirs)
			}
		}
		if got != byEntry[mine] {
			t.Errorf("fold at %s = %d, artifact = %d", mine, got, byEntry[mine])
		}
	}
}

// The per-symbol block, all 168 cells (56 symbols × 3 costs). The artifact
// publishes these at level 0.001 and records neither that level nor the leg
// notional that scales the numerator; this test names both.
func TestOracle_PerSymbolAgreesWithSummariseDotPy(t *testing.T) {
	data, err := os.ReadFile(committedSummary)
	if err != nil {
		t.Fatalf("read %s: %v", committedSummary, err)
	}
	var file struct {
		Costs     []string `json:"costs"`
		PerSymbol []struct {
			Symbol   string            `json:"symbol"`
			Episodes int               `json:"episodes"`
			Days     int               `json:"days"`
			Net      map[string]string `json:"net_usd_1000_per_leg"`
			Annual   map[string]string `json:"annual_pct_on_2000"`
		} `json:"per_symbol"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(file.PerSymbol) != 56 {
		t.Fatalf("artifact per_symbol has %d rows, want 56", len(file.PerSymbol))
	}

	set := loadCommitted(t)
	compared := 0
	for _, cost := range file.Costs {
		got, err := set.PerSymbol(sizeAt(t, "0.001", cost))
		if err != nil {
			t.Fatalf("PerSymbol at %s: %v", cost, err)
		}
		byName := map[string]persistence.SymbolReturn{}
		for _, r := range got.Rows {
			byName[r.Symbol] = r
		}
		for _, want := range file.PerSymbol {
			mine, ok := byName[want.Symbol]
			if !ok {
				t.Errorf("cost %s: %s missing from the fold", cost, want.Symbol)
				continue
			}
			compared++
			if mine.Episodes != want.Episodes || mine.Days != want.Days {
				t.Errorf("%s: episodes/days = %d/%d, artifact %d/%d",
					want.Symbol, mine.Episodes, mine.Days, want.Episodes, want.Days)
			}
			// Net: the artifact publishes summarise.py:110's tot.quantize(D("0.01"))
			// — the total ROUNDED to cents (persymbol_test.go:43-53 already
			// established this against LABUSDT: the artifact shows "543.88", the
			// unrounded fixedpoint total is "543.88166"). The brief's own literal
			// code here compared mine.Net directly against a raw parse of the
			// artifact's cents-rounded string, which cannot agree with a
			// scale-9 total on any cell with a fractional-cent remainder — every
			// one of the 168 cells failed that comparison on a first run. The fix
			// rounds this reader's total to two places, the same way the artifact
			// was rounded, and compares the RESULT.
			if got := roundedTo2(t, mine.Net.Decimal()); got != want.Net[cost] {
				t.Errorf("%s at %s: net (to 2dp) = %s, artifact = %s",
					want.Symbol, cost, got, want.Net[cost])
			}
			// Annual: the artifact publishes a PERCENTAGE at one decimal place.
			// The ×100 conversion lives here and nowhere else, because
			// rate.go:8 declines the unit.
			if pct := percentString(t, mine.Annual); pct != want.Annual[cost] {
				t.Errorf("%s at %s: annual = %s%%, artifact = %s%%",
					want.Symbol, cost, pct, want.Annual[cost])
			}
		}
	}
	// 56 symbols x 3 costs = 168 cells. A passing run over zero comparisons
	// would be vacuous; this makes the count visible rather than trusting the
	// loop shape.
	if compared != 168 {
		t.Fatalf("compared %d cells, want 168 (56 symbols x 3 costs)", compared)
	}
}

// percentString renders a ratio as the artifact's percentage: ×100, one decimal
// place, half-even. This is the only place in the package or its tests that
// multiplies by 100.
func percentString(t *testing.T, r fixedpoint.Rate) string {
	t.Helper()
	v, ok := new(big.Rat).SetString(r.Decimal())
	if !ok {
		t.Fatalf("not a decimal: %q", r.Decimal())
	}
	v.Mul(v, big.NewRat(100, 1))
	return v.FloatString(1)
}
