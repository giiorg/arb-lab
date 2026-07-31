package domain

import (
	"fmt"
	"math/big"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The EWMA's decay is TIME-based: a settlement's weight is 2^(-Δt/halfLife),
// with Δt measured back from the newest settlement in the window. §2.4 states
// the half-life in days, not in settlements, and time-based decay is the
// reading that stays honest across an interval flip (§40f): a window
// spanning 8h and 1h regimes weights each settlement by how old it IS, not by
// how many rows it sits from the end.
//
// The exponent is quantized to decaySteps per half-life and the per-step
// factor is a scale-9 constant, so the whole computation is fixedpoint Rate
// arithmetic — no floats, no transcendental calls, one rounding mode
// throughout (half-even: valuation, where a biased tie accumulates).
const (
	// decaySteps quantizes the exponent: Δt/halfLife is resolved to 1/4096ths
	// of a half-life. The worst-case weight error from quantization is a
	// factor of 2^(1/8192) ≈ 1.00008 — under a hundredth of a percent, well
	// below funding-rate noise.
	decaySteps = 4096

	// decayLambda is round(10^9 · 2^(-1/4096)) — the per-step decay factor at
	// the canonical scale, derived with exact integer arithmetic (the oracle
	// test re-derives it from scratch and fails if this digit string is
	// wrong).
	decayLambda = "0.999830789"

	// maxDecayHalfLives is where a weight underflows the canonical scale:
	// 2^-31 ≈ 4.7e-10 rounds to zero at scale 9, so a settlement older than
	// 31 half-lives contributes nothing representable and is skipped rather
	// than dragged through the arithmetic as a zero. This is not dropping
	// data — it is the scale-9 truth about its weight.
	maxDecayHalfLives = 31
)

// EWMA is the trailing realized-settlement average §2.4's default estimator
// uses for settlements beyond the imminent one: exponentially weighted by
// age, half-life a strategy param.
//
// The window must be non-empty (an average of nothing is ErrWindowEmpty, and
// the caller decides what that refusal means), valid, and strictly ascending;
// the newest settlement anchors Δt = 0 and therefore always carries weight 1,
// which is what makes the weight sum provably non-zero.
func EWMA(window []RealizedSettlement, halfLife time.Duration) (fixedpoint.Rate, error) {
	if halfLife <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("ewma half-life %v: %w", halfLife, ErrUnset)
	}
	if len(window) == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("ewma: %w", ErrWindowEmpty)
	}
	if err := validateWindow(window, window[0].Instrument); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("ewma: %w", err)
	}

	lambda, err := fixedpoint.ParseRate(decayLambda)
	if err != nil {
		panic(err) // a programmer constant, not data
	}

	newest := window[len(window)-1].At
	num := fixedpoint.ZeroRate()
	den := fixedpoint.ZeroRate()
	for _, s := range window {
		age := newest.Sub(s.At)
		if age/halfLife >= maxDecayHalfLives {
			continue // the scale-9 weight is exactly zero — see the constant
		}
		w, err := decayWeight(lambda, age, halfLife)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("ewma weight at %s: %w", s.At, err)
		}
		wr, err := w.Mul(s.Rate, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("ewma term at %s: %w", s.At, err)
		}
		if num, err = num.Add(wr); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("ewma sum: %w", err)
		}
		if den, err = den.Add(w); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("ewma weights: %w", err)
		}
	}
	// The newest settlement has age zero, is never skipped, and weighs 1, so
	// a zero denominator is unreachable — checked anyway, because that chain
	// of reasons lives in the lines above and they move.
	if sign, err := den.Sign(); err != nil || sign == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("ewma weights sum to zero: %w", ErrWindowEmpty)
	}
	return num.Div(den, fixedpoint.RoundHalfEven)
}

// decayWeight is lambda^steps for steps = round(decaySteps · age/halfLife),
// by squaring, half-even per multiplication.
//
// The exponent is computed in big.Int because decaySteps·age overflows int64
// for a window spanning weeks — the one spot in this file where int64 cannot
// carry the intermediate, kept to exactly that spot.
func decayWeight(lambda fixedpoint.Rate, age, halfLife time.Duration) (fixedpoint.Rate, error) {
	// steps = floor((2·K·age + halfLife) / (2·halfLife)) — round-half-up on
	// K·age/halfLife, in exact integer arithmetic.
	n := new(big.Int).Mul(big.NewInt(decaySteps), big.NewInt(int64(age)))
	n.Lsh(n, 1)
	n.Add(n, big.NewInt(int64(halfLife)))
	n.Div(n, new(big.Int).Lsh(big.NewInt(int64(halfLife)), 1))
	// The caller has already excluded ages at or past maxDecayHalfLives, so
	// steps fits comfortably in an int64.
	steps := n.Int64()

	result, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err) // programmer constant
	}
	base := lambda
	for steps > 0 {
		if steps&1 == 1 {
			if result, err = result.Mul(base, fixedpoint.RoundHalfEven); err != nil {
				return fixedpoint.Rate{}, err
			}
		}
		steps >>= 1
		if steps == 0 {
			break
		}
		if base, err = base.Mul(base, fixedpoint.RoundHalfEven); err != nil {
			return fixedpoint.Rate{}, err
		}
	}
	return result, nil
}
