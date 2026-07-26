package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q) failed: %v", s, err)
	}
	return r
}

// Rate is dimensionless — a funding rate, a fee rate, a ratio of two amounts —
// so it has no asset tag to hang validity on. It carries a construction flag
// instead, because the failure the invalid zero value prevents is worse here
// than anywhere else: a fee rate a venue adapter forgot to populate would read
// as *zero fees*, and milestone 2's verdict (§12.2) is a comparison of measured
// carry against a cost hurdle. Free trading would clear any hurdle.
func TestRate_ZeroValueIsInvalid(t *testing.T) {
	var r fixedpoint.Rate

	err := r.Validate()
	if !errors.Is(err, fixedpoint.ErrRateUnset) {
		t.Fatalf("Validate() on the zero value = %v; want it to wrap ErrRateUnset", err)
	}
	// The general sentinel must also match, so a caller that only cares that
	// *something* was left unpopulated has one check across all categories.
	if !errors.Is(err, fixedpoint.ErrUnset) {
		t.Errorf("Validate() = %v; want it to wrap ErrUnset as well", err)
	}
	if err := fixedpoint.ZeroRate().Validate(); err != nil {
		t.Errorf("ZeroRate() must be valid: %v", err)
	}
	// A constructed zero and an unconstructed value must not compare equal —
	// that difference is the whole point of the flag.
	if fixedpoint.ZeroRate() == r {
		t.Error("ZeroRate() == the zero value; the two must be distinguishable")
	}
	if got, want := r.String(), "<invalid rate>"; got != want {
		t.Errorf("the zero value prints as %q; want %q", got, want)
	}
}

// Money's ErrAssetUnset must chain to the same general sentinel, so the check
// above is genuinely uniform rather than true only for Rate.
func TestUnsetSentinelIsShared(t *testing.T) {
	var m fixedpoint.Money
	if err := m.Validate(); !errors.Is(err, fixedpoint.ErrUnset) {
		t.Errorf("Money.Validate() on the zero value = %v; want it to wrap ErrUnset", err)
	}
	var q fixedpoint.Quantity
	if err := q.Validate(); !errors.Is(err, fixedpoint.ErrUnset) {
		t.Errorf("Quantity.Validate() on the zero value = %v; want it to wrap ErrUnset", err)
	}
}

func TestRate_RoundTrip(t *testing.T) {
	// Rates are stored as plain ratios, not percentages: an 0.01% funding rate
	// is 0.0001. Nothing in this package multiplies by 100, deliberately —
	// a percent/ratio mix-up is a factor-of-100 error in the carry estimate.
	for _, s := range []string{"0", "0.0001", "-0.0001", "1", "0.000000001", "0.03"} {
		r := mustRate(t, s)
		if got := r.Decimal(); got != s {
			t.Errorf("ParseRate(%q).Decimal() = %q; want %q", s, got, s)
		}
		if got := r.String(); got != s {
			t.Errorf("ParseRate(%q).String() = %q; want %q", s, got, s)
		}
	}
}

func TestRate_OperationsAreGuarded(t *testing.T) {
	max := mustRate(t, maxDecimal)
	min := mustRate(t, minDecimal)
	one := mustRate(t, ulp)

	if _, err := max.Add(one); !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("max.Add(ulp) = %v; want ErrOverflow", err)
	}
	if _, err := min.Sub(one); !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Sub(ulp) = %v; want ErrOverflow", err)
	}
	if _, err := min.Neg(); !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Neg() = %v; want ErrOverflow", err)
	}
	if _, err := min.Abs(); !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Abs() = %v; want ErrOverflow", err)
	}
	if got, err := min.Cmp(max); err != nil || got != -1 {
		t.Errorf("min.Cmp(max) = %d, %v; want -1, nil", got, err)
	}

	sum, err := mustRate(t, "0.0001").Add(mustRate(t, "0.0002"))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if got, want := sum.Decimal(), "0.0003"; got != want {
		t.Errorf("0.0001 + 0.0002 = %s; want %s", got, want)
	}
}

// Every operation must reject an unconstructed operand, including the ones that
// look like pure inspection.
func TestRate_UnsetDiscipline(t *testing.T) {
	var unset fixedpoint.Rate
	valid := mustRate(t, "0.5")

	ops := map[string]func() error{
		"Add":         func() error { _, err := valid.Add(unset); return err },
		"Add reverse": func() error { _, err := unset.Add(valid); return err },
		"Sub":         func() error { _, err := valid.Sub(unset); return err },
		"Cmp":         func() error { _, err := valid.Cmp(unset); return err },
		"Neg":         func() error { _, err := unset.Neg(); return err },
		"Abs":         func() error { _, err := unset.Abs(); return err },
		"Sign":        func() error { _, err := unset.Sign(); return err },
		"Mul":         func() error { _, err := valid.Mul(unset, fixedpoint.RoundHalfEven); return err },
	}
	for name, op := range ops {
		if err := op(); !errors.Is(err, fixedpoint.ErrRateUnset) {
			t.Errorf("%s with an unconstructed Rate = %v; want ErrRateUnset", name, err)
		}
	}
}

// Multiplying two rates is the first operation that can leave the canonical
// scale, so it is the first that takes a RoundingMode.
func TestRate_Mul(t *testing.T) {
	cases := []struct {
		a, b string
		mode fixedpoint.RoundingMode
		want string // "" means the operation must fail with wantErr
		err  error
	}{
		{a: "0.5", b: "0.5", mode: fixedpoint.RoundExact, want: "0.25"},
		{a: "0.0001", b: "0.03", mode: fixedpoint.RoundExact, want: "0.000003"},
		{a: "-0.5", b: "0.5", mode: fixedpoint.RoundExact, want: "-0.25"},
		{a: "1", b: "0.0001", mode: fixedpoint.RoundExact, want: "0.0001"},

		// The product of two values at the smallest representable magnitude is
		// 1e-18, which the canonical scale cannot hold. What happens next is
		// the caller's decision, and every mode gives a different answer.
		{a: ulp, b: ulp, mode: fixedpoint.RoundExact, err: fixedpoint.ErrPrecisionLoss},
		{a: ulp, b: ulp, mode: fixedpoint.RoundTowardZero, want: "0"},
		{a: ulp, b: ulp, mode: fixedpoint.RoundAwayFromZero, want: ulp},
		{a: ulp, b: ulp, mode: fixedpoint.RoundCeil, want: ulp},
		{a: ulp, b: ulp, mode: fixedpoint.RoundFloor, want: "0"},
		{a: ulp, b: ulp, mode: fixedpoint.RoundHalfUp, want: "0"},
		{a: negUlp, b: ulp, mode: fixedpoint.RoundFloor, want: negUlp},
		{a: negUlp, b: ulp, mode: fixedpoint.RoundCeil, want: "0"},
		{a: negUlp, b: ulp, mode: fixedpoint.RoundAwayFromZero, want: negUlp},

		// A mode nobody set must fail even though 0.5 * 0.5 is exact.
		{a: "0.5", b: "0.5", mode: fixedpoint.RoundInvalid, err: fixedpoint.ErrRoundingMode},

		{a: maxDecimal, b: "2", mode: fixedpoint.RoundExact, err: fixedpoint.ErrOverflow},
	}

	for _, tc := range cases {
		name := tc.a + " * " + tc.b + " " + tc.mode.String()
		t.Run(name, func(t *testing.T) {
			got, err := mustRate(t, tc.a).Mul(mustRate(t, tc.b), tc.mode)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("Mul = %s, nil; want %v", got.Decimal(), tc.err)
				}
				if !errors.Is(err, tc.err) {
					t.Fatalf("Mul error = %v; want it to wrap %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Mul failed: %v", err)
			}
			if got.Decimal() != tc.want {
				t.Errorf("Mul = %s; want %s", got.Decimal(), tc.want)
			}
		})
	}
}
