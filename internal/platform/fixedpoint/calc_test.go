package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Calc exists because Go has no `?`. A break-even computation is a dozen
// consecutive fallible steps, and written out longhand it is a dozen
// `if err != nil` blocks in which the actual arithmetic is hard to see.
//
// The safety property is that the sticky error lives in a *local* calculation
// and never in a stored value: Result is the only way out, and it hands back
// the error alongside a Money that is itself invalid — so a caller who ignores
// the error still cannot store or spend the result.
func TestCalc_HappyPath(t *testing.T) {
	notional := mustMoney(t, usdt, "100000")
	funding := mustMoney(t, usdt, "30")
	entryFee := mustMoney(t, usdt, "45")
	exitFee := mustMoney(t, usdt, "45")

	net, err := fixedpoint.Chain(funding).Sub(entryFee).Sub(exitFee).Result()
	if err != nil {
		t.Fatalf("the chain failed: %v", err)
	}
	if got, want := net.Decimal(), "-60"; got != want {
		t.Errorf("30 - 45 - 45 = %s; want %s", got, want)
	}
	if net.Asset() != usdt {
		t.Errorf("the chain lost the asset tag: %q", net.Asset())
	}

	// The same numbers as a carry expressed against the capital that earned
	// it — the shape milestone 2's verdict takes.
	ratio, err := fixedpoint.Chain(funding).Sub(entryFee).Sub(exitFee).
		Ratio(notional, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatalf("Ratio failed: %v", err)
	}
	if got, want := ratio.Decimal(), "-0.0006"; got != want {
		t.Errorf("-60 / 100000 = %s; want %s", got, want)
	}

	scaled, err := fixedpoint.Chain(notional).
		Mul(mustRate(t, "0.0001"), fixedpoint.RoundExact).
		Add(mustMoney(t, usdt, "1")).
		Neg().
		Abs().
		Result()
	if err != nil {
		t.Fatalf("the chain failed: %v", err)
	}
	if got, want := scaled.Decimal(), "11"; got != want {
		t.Errorf("|-(100000 * 0.0001 + 1)| = %s; want %s", got, want)
	}
}

// Once a step fails, every later step is a no-op and the FIRST error is what
// comes out. Later errors are consequences of operating on a failed value, so
// reporting the last one would name the symptom instead of the cause.
func TestCalc_FirstErrorSticks(t *testing.T) {
	good := mustMoney(t, usdt, "100")
	wrongAsset := mustMoney(t, btc, "1")

	_, err := fixedpoint.Chain(good).
		Sub(wrongAsset).                                // fails here
		Add(fixedpoint.ZeroMoney(usdt)).                // no-op
		Mul(mustRate(t, "2"), fixedpoint.RoundInvalid). // would fail too
		Result()
	if !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Fatalf("the chain reported %v; want the first failure, ErrAssetMismatch", err)
	}
	if errors.Is(err, fixedpoint.ErrRoundingMode) {
		t.Error("a later failure overwrote the first one")
	}
}

// A caller who ignores the error must not end up holding a usable value.
func TestCalc_FailedResultIsUnusable(t *testing.T) {
	got, err := fixedpoint.Chain(mustMoney(t, usdt, "100")).
		Sub(mustMoney(t, btc, "1")).
		Result()
	if err == nil {
		t.Fatal("the chain must fail")
	}
	if verr := got.Validate(); verr == nil {
		t.Error("Result returned a valid Money alongside an error; " +
			"ignoring the error must not yield a spendable amount")
	}
	// And it must stay unusable downstream.
	if _, aerr := got.Add(mustMoney(t, usdt, "1")); aerr == nil {
		t.Error("the failed result can still be added to")
	}
}

// The zero value of Calc is not a usable accumulator: it holds an
// unconstructed Money, so Result must say so rather than hand back a plausible
// zero of no currency.
func TestCalc_ZeroValueIsInvalid(t *testing.T) {
	var c fixedpoint.Calc

	if _, err := c.Result(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("the zero Calc's Result = %v; want ErrAssetUnset", err)
	}
	if _, err := c.Add(mustMoney(t, usdt, "1")).Result(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("adding to the zero Calc = %v; want ErrAssetUnset", err)
	}
	// Starting a chain from an unconstructed value fails at the first step,
	// not silently at the end.
	var unset fixedpoint.Money
	if _, err := fixedpoint.Chain(unset).Result(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Chain on an unset Money = %v; want ErrAssetUnset", err)
	}
}

// Each operation must propagate its own failure into the chain, not just the
// ones that happen to be exercised above.
func TestCalc_EveryStepCanFail(t *testing.T) {
	start := mustMoney(t, usdt, "100")
	other := mustMoney(t, btc, "1")
	min := mustMoney(t, usdt, minDecimal)

	steps := map[string]struct {
		run  func() (fixedpoint.Money, error)
		want error
	}{
		"Add mismatch": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(start).Add(other).Result()
		}, fixedpoint.ErrAssetMismatch},
		"Sub mismatch": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(start).Sub(other).Result()
		}, fixedpoint.ErrAssetMismatch},
		"Mul unset rate": {func() (fixedpoint.Money, error) {
			var r fixedpoint.Rate
			return fixedpoint.Chain(start).Mul(r, fixedpoint.RoundExact).Result()
		}, fixedpoint.ErrRateUnset},
		"Mul no mode": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(start).Mul(mustRate(t, "2"), fixedpoint.RoundInvalid).Result()
		}, fixedpoint.ErrRoundingMode},
		"Neg at the bound": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(min).Neg().Result()
		}, fixedpoint.ErrOverflow},
		"Abs at the bound": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(min).Abs().Result()
		}, fixedpoint.ErrOverflow},
		"Add overflow": {func() (fixedpoint.Money, error) {
			return fixedpoint.Chain(min).Add(mustMoney(t, usdt, negUlp)).Result()
		}, fixedpoint.ErrOverflow},
	}
	for name, tc := range steps {
		if _, err := tc.run(); !errors.Is(err, tc.want) {
			t.Errorf("%s: chain reported %v; want %v", name, err, tc.want)
		}
	}

	// Ratio's own failures, which Result cannot report because Ratio is
	// terminal.
	if _, err := fixedpoint.Chain(start).Ratio(fixedpoint.ZeroMoney(usdt), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrDivideByZero) {
		t.Errorf("Ratio by zero = %v; want ErrDivideByZero", err)
	}
	if _, err := fixedpoint.Chain(start).Sub(other).Ratio(start, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("Ratio after a failed step = %v; want the earlier ErrAssetMismatch", err)
	}
}

// Calc is a value type so that a chain does not allocate. This matters less for
// correctness than for keeping it cheap enough that nobody is tempted to skip
// it and hand-roll the error checks.
func TestCalc_DoesNotAllocate(t *testing.T) {
	a := mustMoney(t, usdt, "100000")
	b := mustMoney(t, usdt, "45")
	r := mustRate(t, "0.0001")

	mustNotAllocate(t, "a successful chain", func() {
		_, _ = fixedpoint.Chain(a).Sub(b).Mul(r, fixedpoint.RoundHalfEven).Result()
	})
}
