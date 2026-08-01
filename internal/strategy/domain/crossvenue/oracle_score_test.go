package crossvenue_test

// The scoring oracles. The portfolio oracle re-rounds every weighted term
// half-even at scale 9 in big.Int and demands byte equality. The LCB
// oracle replays the pinned resampling stream from an INDEPENDENT
// SplitMix64 transcription (the constants re-typed from the published
// algorithm, not imported), keeps every resample mean as an exact big.Rat,
// and selects at exact-rational rank arithmetic — floored — with an inline
// control that the CEIL statistic differs whenever the two straddle
// distinct values, so "floor, never above the exact rank" is pinned and
// not vacuous.

import (
	"fmt"
	"math/big"
	"sort"
	"testing"

	"pgregory.net/rapid"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// oracleSplitmix is the independent transcription: SplitMix64 per
// Steele/Lea/Flood, golden-gamma increment and the murmur-style finalizer.
type oracleSplitmix struct{ state uint64 }

func (o *oracleSplitmix) draw() uint64 {
	o.state += 0x9E3779B97F4A7C15
	x := o.state
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// roundHalfEven9 rounds an exact big.Rat to scale 9, half to even, and
// renders it at 9 places — the valuation rounding, recomputed in integer
// arithmetic.
func roundHalfEven9(r *big.Rat) string {
	scaled := new(big.Rat).Mul(r, big.NewRat(1_000_000_000, 1))
	num, den := new(big.Int).Set(scaled.Num()), new(big.Int).Set(scaled.Denom()) // den > 0
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if rem.Sign() != 0 {
		twice := new(big.Int).Abs(rem)
		twice.Lsh(twice, 1)
		c := twice.Cmp(den)
		roundAway := c > 0 || (c == 0 && q.Bit(0) == 1)
		if roundAway {
			if num.Sign() < 0 {
				q.Sub(q, big.NewInt(1))
			} else {
				q.Add(q, big.NewInt(1))
			}
		}
	}
	out := new(big.Rat).SetFrac(q, big.NewInt(1_000_000_000))
	return out.FloatString(9)
}

func TestProperty_PortfolioReturnsMatchOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		k := rapid.IntRange(2, 4).Draw(t, "symbols")
		// Weights: k-1 distinct cuts of the unit mantissa give positive
		// weights summing to exactly 1.
		cuts := rapid.SliceOfNDistinct(rapid.Int64Range(1, 999_999_999), k-1, k-1, rapid.ID).Draw(t, "cuts")
		sort.Slice(cuts, func(i, j int) bool { return cuts[i] < cuts[j] })
		bounds := append(append([]int64{0}, cuts...), 1_000_000_000)
		policy := make([]crossvenue.Allocation, k)
		for i := range policy {
			policy[i] = crossvenue.Allocation{
				Symbol: fmt.Sprintf("SYM%dUSDT", i),
				Weight: fixedpoint.RateFromMantissa(bounds[i+1] - bounds[i]),
			}
		}

		periods := rapid.IntRange(1, 8).Draw(t, "periods")
		series := map[string][]fixedpoint.Rate{}
		for i := range policy {
			s := make([]fixedpoint.Rate, periods)
			for p := range s {
				s[p] = fixedpoint.RateFromMantissa(
					rapid.Int64Range(-5_000_000, 5_000_000).Draw(t, fmt.Sprintf("r%d_%d", i, p)))
			}
			series[policy[i].Symbol] = s
		}

		got, err := crossvenue.PortfolioReturns(policy, series)
		if err != nil {
			t.Fatalf("PortfolioReturns: %v", err)
		}
		// Exact rational equality against the independently re-rounded
		// terms — an order-of-operations or rounding-mode drift shows as a
		// one-ulp disagreement, which exact comparison cannot miss (the
		// Points-oracle precedent: byte equality needs no perturbation
		// control the way a tolerance does).
		for p := range periods {
			acc := new(big.Rat)
			for _, a := range policy {
				term := new(big.Rat).Mul(
					bigRatDec(t, series[a.Symbol][p].Decimal()),
					bigRatDec(t, a.Weight.Decimal()))
				acc.Add(acc, bigRatDec(t, roundHalfEven9(term)))
			}
			if bigRatDec(t, got[p].Decimal()).Cmp(acc) != 0 {
				t.Fatalf("period %d: production %s, oracle %s", p, got[p].Decimal(), acc.FloatString(9))
			}
		}
	})
}

func TestProperty_LCBMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 12).Draw(t, "n")
		returns := make([]fixedpoint.Rate, n)
		for i := range returns {
			returns[i] = fixedpoint.RateFromMantissa(
				rapid.Int64Range(-5_000_000, 5_000_000).Draw(t, fmt.Sprintf("r%d", i)))
		}
		meanBlock := rapid.IntRange(1, 5).Draw(t, "meanBlock")
		resamples := rapid.IntRange(1, 60).Draw(t, "resamples")
		confMant := rapid.SampledFrom([]int64{750_000_000, 900_000_000, 950_000_000, 990_000_000}).
			Draw(t, "confidence")
		confidence := fixedpoint.RateFromMantissa(confMant)
		seed := rapid.Uint64().Draw(t, "seed")

		got, err := crossvenue.StationaryBootstrapLCB(returns, meanBlock, resamples, confidence, seed)
		if err != nil {
			t.Fatalf("LCB: %v", err)
		}

		// Replay the pinned stream independently and keep exact means.
		rng := &oracleSplitmix{state: seed}
		means := make([]*big.Rat, resamples)
		nRat := big.NewRat(int64(n), 1)
		for b := range means {
			sum := new(big.Rat)
			idx := 0
			for step := 0; step < n; step++ {
				if step == 0 || rng.draw()%uint64(meanBlock) == 0 {
					idx = int(rng.draw() % uint64(n))
				} else {
					idx = (idx + 1) % n
				}
				sum.Add(sum, bigRatDec(t, returns[idx].Decimal()))
			}
			means[b] = new(big.Rat).Quo(sum, nRat)
		}
		sort.Slice(means, func(i, j int) bool { return means[i].Cmp(means[j]) < 0 })

		// rank = (1 − confidence) × (resamples − 1), floored — exact
		// integer arithmetic, independently restated.
		rankNum := new(big.Int).Mul(
			big.NewInt(1_000_000_000-confMant),
			big.NewInt(int64(resamples-1)))
		idx := new(big.Int).Quo(rankNum, big.NewInt(1_000_000_000))
		want := roundHalfEven9(means[idx.Int64()])
		if bigRatDec(t, got.Decimal()).Cmp(bigRatDec(t, want)) != 0 {
			t.Fatalf("LCB %s, oracle order statistic %d = %s", got.Decimal(), idx.Int64(), want)
		}

		// Inline control: whenever the exact rank is non-integer and the
		// ceil statistic rounds to a DIFFERENT value, the floor rule must
		// be distinguishable — production agreeing with ceil here would
		// mean the conservative direction flipped.
		exactRankIsInt := new(big.Int).Mod(rankNum, big.NewInt(1_000_000_000)).Sign() == 0
		if !exactRankIsInt && idx.Int64()+1 < int64(resamples) {
			ceilStat := roundHalfEven9(means[idx.Int64()+1])
			if ceilStat != want && bigRatDec(t, got.Decimal()).Cmp(bigRatDec(t, ceilStat)) == 0 {
				t.Fatalf("LCB %s equals the CEIL statistic %s, not the floor %s", got.Decimal(), ceilStat, want)
			}
		}
	})
}
