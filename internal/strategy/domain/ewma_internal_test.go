package domain

// The decay constant's oracle: decayLambda is a digit string a human typed,
// and this test re-derives it from the definition with exact integer
// arithmetic — round-half-even of 10^9 · 2^(-1/4096) — so a mistyped digit
// fails here rather than skewing every weight by a whisker forever.

import (
	"math/big"
	"testing"
)

func TestDecayLambdaMatchesItsDefinition(t *testing.T) {
	const scale = 1_000_000_000

	sk := new(big.Int).Exp(big.NewInt(scale), big.NewInt(decaySteps), nil)

	// floor(scale · 2^(-1/K)) is the largest x with x^K · 2 <= scale^K.
	le := func(x *big.Int) bool {
		v := new(big.Int).Exp(x, big.NewInt(decaySteps), nil)
		v.Mul(v, big.NewInt(2))
		return v.Cmp(sk) <= 0
	}
	lo, hi := big.NewInt(0), big.NewInt(scale)
	for new(big.Int).Sub(hi, lo).Cmp(big.NewInt(1)) > 0 {
		mid := new(big.Int).Add(lo, hi)
		mid.Rsh(mid, 1)
		if le(mid) {
			lo.Set(mid)
		} else {
			hi.Set(mid)
		}
	}

	// Round: is the true value above the halfway point lo + 1/2? Compare
	// (2·lo+1)^K · 2 against scale^K · 2^K. (The true value is irrational,
	// so an exact tie — where half-even would differ from half-up — cannot
	// occur; the comparison decides every case.)
	half := new(big.Int).Add(new(big.Int).Lsh(lo, 1), big.NewInt(1))
	hv := new(big.Int).Exp(half, big.NewInt(decaySteps), nil)
	hv.Mul(hv, big.NewInt(2))
	rhs := new(big.Int).Lsh(sk, decaySteps)
	rounded := new(big.Int).Set(lo)
	if hv.Cmp(rhs) < 0 {
		rounded.Add(rounded, big.NewInt(1))
	}

	want := "0." + rounded.String()
	if decayLambda != want {
		t.Fatalf("decayLambda = %s; the definition derives %s", decayLambda, want)
	}
}
