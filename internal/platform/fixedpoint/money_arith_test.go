package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

const (
	usdt = fixedpoint.Asset("USDT")
	btc  = fixedpoint.Asset("BTC")

	// The int64 mantissa boundaries at scale 9 and their neighbours, written
	// out in full rather than computed, so a bug in the constants cannot hide
	// inside the expectation. Note the asymmetry: the negative bound has one
	// more unit of magnitude, because MinInt64 has no positive counterpart.
	maxDecimal  = "9223372036.854775807"
	maxMinusUlp = "9223372036.854775806"
	minDecimal  = "-9223372036.854775808"
	minPlusUlp  = "-9223372036.854775807"
	ulp         = "0.000000001"
	negUlp      = "-0.000000001"
)

func mustMoney(t *testing.T, asset fixedpoint.Asset, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(asset, s)
	if err != nil {
		t.Fatalf("ParseMoney(%q, %q) failed: %v", asset, s, err)
	}
	return m
}

// Addition and subtraction at a fixed scale are exact — that is the whole point
// of storing a mantissa rather than a float — so neither takes a RoundingMode.
// What they can do is leave the representable range, and the rule there is
// §3.6's: overflow errors, never wraps.
func TestMoney_AddSub(t *testing.T) {
	cases := []struct {
		name    string
		a, b    string
		wantAdd string // "" means the addition must fail with wantAddErr
		wantSub string // "" means the subtraction must fail with wantSubErr
	}{
		{name: "integers", a: "2", b: "3", wantAdd: "5", wantSub: "-1"},
		{name: "fractions", a: "1.5", b: "0.25", wantAdd: "1.75", wantSub: "1.25"},
		{name: "signs differ", a: "1.5", b: "-2.5", wantAdd: "-1", wantSub: "4"},
		{name: "both negative", a: "-1.5", b: "-2.5", wantAdd: "-4", wantSub: "1"},
		{name: "smallest units", a: ulp, b: "0.000000002", wantAdd: "0.000000003", wantSub: negUlp},
		{name: "identity", a: "123.456", b: "0", wantAdd: "123.456", wantSub: "123.456"},
		{name: "self cancels", a: "7.25", b: "7.25", wantAdd: "14.5", wantSub: "0"},

		// The extremes meet in the middle without leaving the range: the case a
		// naive "either operand is extreme, therefore overflow" guard would
		// reject wrongly. Their sum is one unit in the last place below zero,
		// not minus one, because the bounds differ by exactly one mantissa step.
		{name: "max plus min", a: maxDecimal, b: minDecimal, wantAdd: negUlp, wantSub: ""},
		{name: "max minus max", a: maxDecimal, b: maxDecimal, wantAdd: "", wantSub: "0"},

		// One unit in the last place past each bound.
		{name: "overflow at the top", a: maxDecimal, b: ulp, wantAdd: "", wantSub: maxMinusUlp},
		{name: "overflow at the bottom", a: minDecimal, b: negUlp, wantAdd: "", wantSub: minPlusUlp},
		{name: "subtract past the bottom", a: minDecimal, b: ulp, wantAdd: minPlusUlp, wantSub: ""},

		// Subtracting the negative bound from zero asks for its magnitude,
		// which is not representable. In plain Go `0 - x` here evaluates back
		// to the bound itself — a sign flip that silently does nothing.
		{name: "negate the bottom", a: "0", b: minDecimal, wantAdd: minDecimal, wantSub: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := mustMoney(t, usdt, tc.a)
			b := mustMoney(t, usdt, tc.b)

			got, err := a.Add(b)
			switch {
			case tc.wantAdd == "":
				if err == nil {
					t.Errorf("%s.Add(%s) = %s, nil; want ErrOverflow", tc.a, tc.b, got.Decimal())
				} else if !errors.Is(err, fixedpoint.ErrOverflow) {
					t.Errorf("%s.Add(%s) error = %v; want it to wrap ErrOverflow", tc.a, tc.b, err)
				}
			case err != nil:
				t.Errorf("%s.Add(%s) failed: %v", tc.a, tc.b, err)
			default:
				if got.Decimal() != tc.wantAdd {
					t.Errorf("%s.Add(%s) = %s; want %s", tc.a, tc.b, got.Decimal(), tc.wantAdd)
				}
				if got.Asset() != usdt {
					t.Errorf("Add dropped the asset tag: got %q", got.Asset())
				}
			}

			got, err = a.Sub(b)
			switch {
			case tc.wantSub == "":
				if err == nil {
					t.Errorf("%s.Sub(%s) = %s, nil; want ErrOverflow", tc.a, tc.b, got.Decimal())
				} else if !errors.Is(err, fixedpoint.ErrOverflow) {
					t.Errorf("%s.Sub(%s) error = %v; want it to wrap ErrOverflow", tc.a, tc.b, err)
				}
			case err != nil:
				t.Errorf("%s.Sub(%s) failed: %v", tc.a, tc.b, err)
			default:
				if got.Decimal() != tc.wantSub {
					t.Errorf("%s.Sub(%s) = %s; want %s", tc.a, tc.b, got.Decimal(), tc.wantSub)
				}
				if got.Asset() != usdt {
					t.Errorf("Sub dropped the asset tag: got %q", got.Asset())
				}
			}
		})
	}
}

// Negation is where the int64 asymmetry stops being a parser curiosity and
// becomes an arithmetic trap: -MinInt64 is not representable, and in Go it
// silently evaluates back to MinInt64. A PnL flip on the most extreme
// representable loss would otherwise return a *loss of the same size*.
func TestMoney_NegAbs(t *testing.T) {
	cases := []struct {
		in               string
		wantNeg, wantAbs string // "" means the operation must fail
	}{
		{in: "1.5", wantNeg: "-1.5", wantAbs: "1.5"},
		{in: "-1.5", wantNeg: "1.5", wantAbs: "1.5"},
		{in: "0", wantNeg: "0", wantAbs: "0"},
		{in: maxDecimal, wantNeg: minPlusUlp, wantAbs: maxDecimal},
		{in: minDecimal, wantNeg: "", wantAbs: ""},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			m := mustMoney(t, usdt, tc.in)

			got, err := m.Neg()
			if tc.wantNeg == "" {
				if err == nil {
					t.Errorf("(%s).Neg() = %s, nil; want ErrOverflow", tc.in, got.Decimal())
				} else if !errors.Is(err, fixedpoint.ErrOverflow) {
					t.Errorf("(%s).Neg() error = %v; want it to wrap ErrOverflow", tc.in, err)
				}
			} else if err != nil {
				t.Errorf("(%s).Neg() failed: %v", tc.in, err)
			} else if got.Decimal() != tc.wantNeg {
				t.Errorf("(%s).Neg() = %s; want %s", tc.in, got.Decimal(), tc.wantNeg)
			}

			got, err = m.Abs()
			if tc.wantAbs == "" {
				if err == nil {
					t.Errorf("(%s).Abs() = %s, nil; want ErrOverflow", tc.in, got.Decimal())
				} else if !errors.Is(err, fixedpoint.ErrOverflow) {
					t.Errorf("(%s).Abs() error = %v; want it to wrap ErrOverflow", tc.in, err)
				}
			} else if err != nil {
				t.Errorf("(%s).Abs() failed: %v", tc.in, err)
			} else if got.Decimal() != tc.wantAbs {
				t.Errorf("(%s).Abs() = %s; want %s", tc.in, got.Decimal(), tc.wantAbs)
			}
		})
	}
}

func TestMoney_CmpAndSign(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"1.5", "1.5", 0},
		{"0", "0", 0},
		{"-1", "1", -1},
		{"-2", "-1", -1},
		{ulp, "0", 1},
		{minDecimal, maxDecimal, -1},
		// Differing by a single unit in the last place must not compare equal;
		// this is what a lossy comparison would get wrong.
		{"1.000000001", "1", 1},
	}

	for _, tc := range cases {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			a := mustMoney(t, usdt, tc.a)
			b := mustMoney(t, usdt, tc.b)

			got, err := a.Cmp(b)
			if err != nil {
				t.Fatalf("Cmp failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("(%s).Cmp(%s) = %d; want %d", tc.a, tc.b, got, tc.want)
			}
			// Antisymmetry, checked rather than assumed.
			rev, err := b.Cmp(a)
			if err != nil {
				t.Fatalf("reversed Cmp failed: %v", err)
			}
			if rev != -tc.want {
				t.Errorf("(%s).Cmp(%s) = %d; want %d (antisymmetry)", tc.b, tc.a, rev, -tc.want)
			}
		})
	}

	// Sign must agree with a comparison against the additive identity. Stated
	// as a relation between two public operations, so neither can drift alone.
	for _, s := range []string{"0", "1", "-1", ulp, negUlp, maxDecimal, minDecimal} {
		m := mustMoney(t, usdt, s)
		sign, err := m.Sign()
		if err != nil {
			t.Fatalf("(%s).Sign() failed: %v", s, err)
		}
		want, err := m.Cmp(fixedpoint.ZeroMoney(usdt))
		if err != nil {
			t.Fatalf("(%s).Cmp(zero) failed: %v", s, err)
		}
		if sign != want {
			t.Errorf("(%s).Sign() = %d; want %d, to agree with Cmp against zero", s, sign, want)
		}
	}
}

// Every operation must reject an unset asset and a mismatched pair. The tag is
// only worth carrying if it is checked at every seam — one operation that skips
// the check is enough to let a BTC amount into a USDT total.
func TestMoney_AssetDiscipline(t *testing.T) {
	var unset fixedpoint.Money
	valid := mustMoney(t, usdt, "1")
	other := mustMoney(t, btc, "1")

	binary := map[string]func(a, b fixedpoint.Money) error{
		"Add": func(a, b fixedpoint.Money) error { _, err := a.Add(b); return err },
		"Sub": func(a, b fixedpoint.Money) error { _, err := a.Sub(b); return err },
		"Cmp": func(a, b fixedpoint.Money) error { _, err := a.Cmp(b); return err },
	}
	for name, op := range binary {
		t.Run(name, func(t *testing.T) {
			for _, pair := range []struct {
				label string
				a, b  fixedpoint.Money
				want  error
			}{
				{"unset receiver", unset, valid, fixedpoint.ErrAssetUnset},
				{"unset argument", valid, unset, fixedpoint.ErrAssetUnset},
				{"both unset", unset, unset, fixedpoint.ErrAssetUnset},
				{"mismatch", valid, other, fixedpoint.ErrAssetMismatch},
				{"mismatch reversed", other, valid, fixedpoint.ErrAssetMismatch},
			} {
				err := op(pair.a, pair.b)
				if err == nil {
					t.Errorf("%s with %s returned nil; want %v", name, pair.label, pair.want)
					continue
				}
				if !errors.Is(err, pair.want) {
					t.Errorf("%s with %s = %v; want it to wrap %v", name, pair.label, err, pair.want)
				}
			}
		})
	}

	unary := map[string]func(m fixedpoint.Money) error{
		"Neg":  func(m fixedpoint.Money) error { _, err := m.Neg(); return err },
		"Abs":  func(m fixedpoint.Money) error { _, err := m.Abs(); return err },
		"Sign": func(m fixedpoint.Money) error { _, err := m.Sign(); return err },
	}
	for name, op := range unary {
		t.Run(name, func(t *testing.T) {
			err := op(unset)
			if err == nil {
				t.Fatalf("%s on the zero value returned nil; want ErrAssetUnset", name)
			}
			if !errors.Is(err, fixedpoint.ErrAssetUnset) {
				t.Errorf("%s on the zero value = %v; want it to wrap ErrAssetUnset", name, err)
			}
		})
	}
}

// TestMoneyDivByANegativeAmount closes a gap that Price.Div's mutation campaign
// found here (task #210).
//
// The zero-divisor guard in Div is redundant with mulDivMantissa's — both return
// ErrDivideByZero — so `o.m == 0` → `o.m < 0` SURVIVES every existing test: the
// zero case is unchanged and the only behaviour that moves is the negative
// divisor, which nothing in this package ever exercised on either type. That
// mutant was aimed at Price.Div, and it was carrying it to its mirror that
// showed the older operation had the same hole.
//
// A negative divisor is legal here: Money may be negative, and Div's own subject
// — milestone 2's carry-over-capital number — is a ratio whose sign a caller is
// entitled to read.
func TestMoneyDivByANegativeAmount(t *testing.T) {
	got, err := mustMoney(t, usdt, "10").Div(mustMoney(t, usdt, "-100"), fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div by a negative amount: %v", err)
	}
	if got.Decimal() != "-0.1" {
		t.Errorf("10 / -100 = %s, want -0.1", got.Decimal())
	}
	// The positive control beside it, so the assertion above is measuring the
	// sign rather than an operation that returns the same thing either way.
	pos, err := mustMoney(t, usdt, "10").Div(mustMoney(t, usdt, "100"), fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div: %v", err)
	}
	if pos.Decimal() != "0.1" {
		t.Errorf("10 / 100 = %s, want 0.1", pos.Decimal())
	}
}

// String is what a log line or a wrapped error shows. An unset value must be
// unmistakable there rather than printing as a plausible amount of nothing.
func TestMoney_String(t *testing.T) {
	if got, want := mustMoney(t, usdt, "1.5").String(), "1.5 USDT"; got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
	if got, want := fixedpoint.ZeroMoney(btc).String(), "0 BTC"; got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
	var unset fixedpoint.Money
	if got, want := unset.String(), "<invalid money>"; got != want {
		t.Errorf("the zero value prints as %q; want %q", got, want)
	}
}
