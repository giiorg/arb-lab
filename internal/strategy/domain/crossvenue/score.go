package crossvenue

// The gate's verdict statistic, shaped by ruling §45f-3: a PRE-DECLARED
// allocation policy over the shortlist, scored at the lower confidence
// bound of the portfolio's net return — per-symbol verdicts are evidence,
// never the pass condition. The bound is a stationary-bootstrap percentile
// LCB (§8's "the LCB, not the point estimate"), deterministic under its
// seed: the resampling stream is pinned to SplitMix64 exactly as published
// (Steele/Lea/Flood 2014), so a rerun IS a replay and the oracle can walk
// the identical index paths from an independent transcription.

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Allocation is one holding of the pre-declared policy.
type Allocation struct {
	Symbol string
	Weight fixedpoint.Rate
}

// PortfolioReturns combines per-symbol per-period return series under the
// declared policy: one shared period grid (the walker builds it; idle
// periods contribute zero — that construction is the utilization honesty),
// weighted sum per period at half-even. The policy must name unique
// symbols with positive weights summing EXACTLY to one, and the series set
// must match the policy's symbols exactly — a measured symbol the policy
// does not hold, or a held symbol nobody measured, is a refusal, not a
// skip.
func PortfolioReturns(policy []Allocation, series map[string][]fixedpoint.Rate) ([]fixedpoint.Rate, error) {
	if len(policy) == 0 {
		return nil, fmt.Errorf("empty policy: %w", ErrPolicyInvalid)
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err) // programmer constant
	}
	sum := fixedpoint.ZeroRate()
	seen := make(map[string]bool, len(policy))
	for _, a := range policy {
		if a.Symbol == "" {
			return nil, fmt.Errorf("unnamed holding: %w", ErrPolicyInvalid)
		}
		if seen[a.Symbol] {
			return nil, fmt.Errorf("holding %s repeated: %w", a.Symbol, ErrPolicyInvalid)
		}
		seen[a.Symbol] = true
		if err := a.Weight.Validate(); err != nil {
			return nil, fmt.Errorf("holding %s: %w", a.Symbol, err)
		}
		if s, err := a.Weight.Sign(); err != nil {
			return nil, fmt.Errorf("holding %s: %w", a.Symbol, err)
		} else if s <= 0 {
			return nil, fmt.Errorf("holding %s weight %s: %w", a.Symbol, a.Weight.Decimal(), ErrPolicyInvalid)
		}
		if sum, err = sum.Add(a.Weight); err != nil {
			return nil, fmt.Errorf("policy sum: %w", err)
		}
	}
	if c, err := sum.Cmp(one); err != nil {
		return nil, fmt.Errorf("policy sum: %w", err)
	} else if c != 0 {
		return nil, fmt.Errorf("policy weighs %s, not 1: %w", sum.Decimal(), ErrPolicyInvalid)
	}

	if len(series) != len(policy) {
		return nil, fmt.Errorf("%d series against %d holdings: %w", len(series), len(policy), ErrSeriesMisaligned)
	}
	periods := -1
	for _, a := range policy {
		s, ok := series[a.Symbol]
		if !ok {
			return nil, fmt.Errorf("holding %s has no series: %w", a.Symbol, ErrSeriesMisaligned)
		}
		if periods == -1 {
			periods = len(s)
		} else if len(s) != periods {
			return nil, fmt.Errorf("holding %s spans %d periods against %d: %w",
				a.Symbol, len(s), periods, ErrSeriesMisaligned)
		}
	}
	if periods == 0 {
		return nil, ErrNoReturns
	}

	out := make([]fixedpoint.Rate, periods)
	for t := range out {
		acc := fixedpoint.ZeroRate()
		for _, a := range policy {
			r := series[a.Symbol][t]
			if err := r.Validate(); err != nil {
				return nil, fmt.Errorf("holding %s period %d: %w", a.Symbol, t, err)
			}
			weighted, err := r.Mul(a.Weight, fixedpoint.RoundHalfEven)
			if err != nil {
				return nil, fmt.Errorf("holding %s period %d: %w", a.Symbol, t, err)
			}
			if acc, err = acc.Add(weighted); err != nil {
				return nil, fmt.Errorf("period %d: %w", t, err)
			}
		}
		out[t] = acc
	}
	return out, nil
}

// StationaryBootstrapLCB bounds the mean per-period return from below at
// the given confidence, by the Politis–Romano stationary bootstrap:
// resampled index paths restart a block with probability 1/meanBlock and
// otherwise continue circularly; each resample's mean (half-even) enters
// the bootstrap distribution, and the bound is selected at the LOWER order
// statistic of the (1−confidence) rank — floor, never above the exact
// rank, so the frozen number can only understate the return it serves
// (UpperQuantile's rule, mirrored for a lower bound).
//
// The resampling stream, pinned: r0 = SplitMix64(seed); a draw d starts a
// new block iff d mod meanBlock == 0 (probability exactly 1/meanBlock); a
// new block's start is the next draw mod n; the first index of every path
// is a fresh block start. Modulo over a 64-bit draw carries a bias below
// one part in 2^54 for any plausible n — recorded, not corrected, because
// a rejection loop would make path length data-dependent and a replay
// fragile.
func StationaryBootstrapLCB(returns []fixedpoint.Rate, meanBlock, resamples int, confidence fixedpoint.Rate, seed uint64) (fixedpoint.Rate, error) {
	if len(returns) == 0 {
		return fixedpoint.Rate{}, ErrNoReturns
	}
	for i, r := range returns {
		if err := r.Validate(); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("return %d: %w", i, err)
		}
	}
	if meanBlock < 1 {
		return fixedpoint.Rate{}, fmt.Errorf("mean block %d: %w", meanBlock, ErrBootstrapInvalid)
	}
	if resamples < 1 {
		return fixedpoint.Rate{}, fmt.Errorf("resamples %d: %w", resamples, ErrBootstrapInvalid)
	}
	if err := confidence.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("confidence: %w", err)
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err) // programmer constant
	}
	if s, err := confidence.Sign(); err != nil {
		return fixedpoint.Rate{}, err
	} else if s <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("confidence %s: %w", confidence.Decimal(), ErrBootstrapInvalid)
	}
	if c, err := confidence.Cmp(one); err != nil {
		return fixedpoint.Rate{}, err
	} else if c >= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("confidence %s: %w", confidence.Decimal(), ErrBootstrapInvalid)
	}

	n := len(returns)
	nRate := fixedpoint.RateFromMantissa(int64(n) * 1_000_000_000)
	rng := splitmix64(seed)
	means := make([]fixedpoint.Rate, resamples)
	for b := range means {
		sum := fixedpoint.ZeroRate()
		idx := 0
		for t := 0; t < n; t++ {
			if t == 0 || rng.next()%uint64(meanBlock) == 0 {
				idx = int(rng.next() % uint64(n))
			} else {
				idx = (idx + 1) % n
			}
			if sum, err = sum.Add(returns[idx]); err != nil {
				return fixedpoint.Rate{}, fmt.Errorf("resample %d: %w", b, err)
			}
		}
		if means[b], err = sum.Div(nRate, fixedpoint.RoundHalfEven); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("resample %d mean: %w", b, err)
		}
	}

	sort.Slice(means, func(i, j int) bool {
		c, err := means[i].Cmp(means[j])
		if err != nil {
			panic(err) // both operands validated above
		}
		return c < 0
	})

	// rank = (1 − confidence) × (resamples − 1), floored: exact integer
	// arithmetic at the mantissa scale, no rounding mode to choose wrong.
	q := 1_000_000_000 - confidence.Mantissa()
	idx := q * int64(resamples-1) / 1_000_000_000
	return means[idx], nil
}

// splitmix64 is SplitMix64 exactly as published: state advances by the
// golden-gamma constant, output is the finalizing mix. Small enough to
// carry here, which is the point — the resampling stream is OUR committed
// definition, not a standard library's version-dependent one.
type splitmixState struct{ s uint64 }

func splitmix64(seed uint64) *splitmixState { return &splitmixState{s: seed} }

func (g *splitmixState) next() uint64 {
	g.s += 0x9E3779B97F4A7C15
	z := g.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
