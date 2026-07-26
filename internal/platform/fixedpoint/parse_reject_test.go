package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// What the parser REFUSES is as much of the contract as what it accepts, and
// which error it returns is load-bearing: a caller reading a venue field wants
// to distinguish "this venue quotes finer than we store" (a finding worth
// alerting on) from "this is not a number" (a protocol bug).
func TestParseMoney_rejects(t *testing.T) {
	cases := []struct {
		in   string
		want error
	}{
		// Not a decimal number at all.
		{"", fixedpoint.ErrSyntax},
		{".", fixedpoint.ErrSyntax},
		{"+", fixedpoint.ErrSyntax},
		{"-", fixedpoint.ErrSyntax},
		{"abc", fixedpoint.ErrSyntax},
		{"1.2.3", fixedpoint.ErrSyntax},
		{"--1", fixedpoint.ErrSyntax},
		{"NaN", fixedpoint.ErrSyntax},
		{"Inf", fixedpoint.ErrSyntax},
		{"0x10", fixedpoint.ErrSyntax},
		{"1_000", fixedpoint.ErrSyntax},
		{"1e", fixedpoint.ErrSyntax},
		{"1e+", fixedpoint.ErrSyntax},
		{"1.2e3.4", fixedpoint.ErrSyntax},

		// Surrounding whitespace is rejected, not trimmed: a venue field with
		// stray padding is a parsing surprise we want to hear about.
		{" 1", fixedpoint.ErrSyntax},
		{"1 ", fixedpoint.ErrSyntax},
		{"1\n", fixedpoint.ErrSyntax},

		// Finer than the canonical scale, with a non-zero digit being dropped.
		{"0.0000000001", fixedpoint.ErrPrecisionLoss},
		{"1.0000000001", fixedpoint.ErrPrecisionLoss},
		{"1e-10", fixedpoint.ErrPrecisionLoss},
		{"1e-999999999999999999999", fixedpoint.ErrPrecisionLoss},

		// Outside the representable range. Note the asymmetry: MinInt64 has no
		// positive counterpart, so the positive boundary rejects one ulp
		// earlier than the negative one.
		{"9223372036.854775808", fixedpoint.ErrOverflow},
		{"-9223372036.854775809", fixedpoint.ErrOverflow},
		{"92233720368.5", fixedpoint.ErrOverflow},
		{"1e100", fixedpoint.ErrOverflow},
		{"1e999999999999999999999", fixedpoint.ErrOverflow},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := fixedpoint.ParseMoney(usdt, tc.in)
			if err == nil {
				t.Fatalf("ParseMoney(%q) = %s, nil; want error %v", tc.in, got.Decimal(), tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("ParseMoney(%q) error = %v; want it to wrap %v", tc.in, err, tc.want)
			}
		})
	}
}

// A zero exponent applied to zero digits is still zero, however it is spelled —
// this guards the early-return path that skips exponent range checking.
func TestParseMoney_zeroIsZeroWhateverTheExponent(t *testing.T) {
	for _, in := range []string{"0", "-0", "0.0", "0e100", "0e-100", "-0.000e5"} {
		m, err := fixedpoint.ParseMoney(usdt, in)
		if err != nil {
			t.Fatalf("ParseMoney(%q) returned %v; want zero", in, err)
		}
		if got := m.Decimal(); got != "0" {
			t.Errorf("ParseMoney(%q).Decimal() = %q; want %q", in, got, "0")
		}
	}
}

// The asset tag is checked before the text: an untagged amount is unusable no
// matter how well-formed the number is.
func TestParseMoney_requiresAsset(t *testing.T) {
	_, err := fixedpoint.ParseMoney("", "1")
	if !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Fatalf("ParseMoney with empty asset: err = %v; want ErrAssetUnset", err)
	}
}
