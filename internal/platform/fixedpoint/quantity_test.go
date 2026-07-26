package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

func mustQuantity(t *testing.T, asset fixedpoint.Asset, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(asset, s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q, %q) failed: %v", asset, s, err)
	}
	return q
}

// Quantity counts units of an instrument's base asset. It is a separate type
// from Money rather than an alias so that adding a size to a balance does not
// compile — see TestCategoriesDoNotMix, which proves that claim against the
// compiler rather than asserting it in prose.
func TestQuantity_ZeroValueIsInvalid(t *testing.T) {
	var q fixedpoint.Quantity

	if err := q.Validate(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Fatalf("Validate() on the zero value = %v; want it to wrap ErrAssetUnset", err)
	}
	if got, want := q.String(), "<invalid quantity>"; got != want {
		t.Errorf("the zero value prints as %q; want %q", got, want)
	}
	if err := fixedpoint.ZeroQuantity(btc).Validate(); err != nil {
		t.Errorf("ZeroQuantity(BTC) must be valid: %v", err)
	}
}

func TestQuantity_RoundTrip(t *testing.T) {
	for _, s := range []string{"0", "1", "-1", "0.5", "0.000000001", maxDecimal, minDecimal} {
		q := mustQuantity(t, btc, s)
		if got := q.Decimal(); got != s {
			t.Errorf("ParseQuantity(%q).Decimal() = %q; want %q", s, got, s)
		}
		if q.Asset() != btc {
			t.Errorf("ParseQuantity(%q) lost the asset tag: %q", s, q.Asset())
		}
	}
	if got, want := mustQuantity(t, btc, "1.5").String(), "1.5 BTC"; got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

// Quantity shares the integer kernel with Money, so this does not re-derive the
// whole overflow table. What it checks is that each of Quantity's operations
// actually routes through the guarded helper rather than using a bare operator
// — which is what a copy-paste of the type without its checks would look like.
func TestQuantity_OperationsAreGuarded(t *testing.T) {
	max := mustQuantity(t, btc, maxDecimal)
	min := mustQuantity(t, btc, minDecimal)
	one := mustQuantity(t, btc, ulp)

	if got, err := max.Add(one); err == nil {
		t.Errorf("max.Add(ulp) = %s, nil; want ErrOverflow", got.Decimal())
	} else if !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("max.Add(ulp) = %v; want ErrOverflow", err)
	}
	if got, err := min.Sub(one); err == nil {
		t.Errorf("min.Sub(ulp) = %s, nil; want ErrOverflow", got.Decimal())
	} else if !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Sub(ulp) = %v; want ErrOverflow", err)
	}
	if got, err := min.Neg(); err == nil {
		t.Errorf("min.Neg() = %s, nil; want ErrOverflow", got.Decimal())
	} else if !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Neg() = %v; want ErrOverflow", err)
	}
	if got, err := min.Abs(); err == nil {
		t.Errorf("min.Abs() = %s, nil; want ErrOverflow", got.Decimal())
	} else if !errors.Is(err, fixedpoint.ErrOverflow) {
		t.Errorf("min.Abs() = %v; want ErrOverflow", err)
	}

	// And that the ordinary path still computes the right answer.
	sum, err := mustQuantity(t, btc, "1.5").Add(mustQuantity(t, btc, "0.25"))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if got, want := sum.Decimal(), "1.75"; got != want {
		t.Errorf("1.5 + 0.25 = %s; want %s", got, want)
	}
	diff, err := mustQuantity(t, btc, "1.5").Sub(mustQuantity(t, btc, "2"))
	if err != nil {
		t.Fatalf("Sub failed: %v", err)
	}
	if got, want := diff.Decimal(), "-0.5"; got != want {
		t.Errorf("1.5 - 2 = %s; want %s", got, want)
	}

	// Cmp must compare, not subtract-and-take-the-sign.
	if got, err := min.Cmp(max); err != nil || got != -1 {
		t.Errorf("min.Cmp(max) = %d, %v; want -1, nil", got, err)
	}
	if got, err := min.Sign(); err != nil || got != -1 {
		t.Errorf("min.Sign() = %d, %v; want -1, nil", got, err)
	}
}

func TestQuantity_AssetDiscipline(t *testing.T) {
	var unset fixedpoint.Quantity
	valid := mustQuantity(t, btc, "1")
	other := mustQuantity(t, fixedpoint.Asset("ETH"), "1")

	binary := map[string]func(a, b fixedpoint.Quantity) error{
		"Add": func(a, b fixedpoint.Quantity) error { _, err := a.Add(b); return err },
		"Sub": func(a, b fixedpoint.Quantity) error { _, err := a.Sub(b); return err },
		"Cmp": func(a, b fixedpoint.Quantity) error { _, err := a.Cmp(b); return err },
	}
	for name, op := range binary {
		if err := op(unset, valid); !errors.Is(err, fixedpoint.ErrAssetUnset) {
			t.Errorf("%s with an unset receiver = %v; want ErrAssetUnset", name, err)
		}
		if err := op(valid, unset); !errors.Is(err, fixedpoint.ErrAssetUnset) {
			t.Errorf("%s with an unset argument = %v; want ErrAssetUnset", name, err)
		}
		if err := op(valid, other); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
			t.Errorf("%s across BTC and ETH = %v; want ErrAssetMismatch", name, err)
		}
	}

	unary := map[string]func(q fixedpoint.Quantity) error{
		"Neg":  func(q fixedpoint.Quantity) error { _, err := q.Neg(); return err },
		"Abs":  func(q fixedpoint.Quantity) error { _, err := q.Abs(); return err },
		"Sign": func(q fixedpoint.Quantity) error { _, err := q.Sign(); return err },
	}
	for name, op := range unary {
		if err := op(unset); !errors.Is(err, fixedpoint.ErrAssetUnset) {
			t.Errorf("%s on the zero value = %v; want ErrAssetUnset", name, err)
		}
	}
}

// Quantity.Mul had no test at all until the mutation audit reported it at 0.0%
// coverage, and the way it was missed is worth keeping in view: the two tables
// above enumerate operations by SIGNATURE SHAPE — binary ones taking a Quantity,
// unary ones taking nothing. Mul is the package's only Quantity method with a
// third shape, (Rate, RoundingMode), so it fell between two tables that each
// looked exhaustive from the inside. A table driven by shape is a coverage claim
// about the shapes it names, not about the method set.
//
// Scaling a size by a dimensionless ratio is how a fraction of a position is
// taken and how a contract multiplier is applied, so every guard the other
// operations get is checked here too — including the one the other tables cannot
// have, an unset Rate.
func TestQuantity_Mul(t *testing.T) {
	cases := []struct {
		q, r string
		mode fixedpoint.RoundingMode
		want string // "" means the operation must fail with err
		err  error
	}{
		// The ordinary path: taking half a position, and applying a multiplier
		// bigger than one.
		{q: "2", r: "0.5", mode: fixedpoint.RoundExact, want: "1"},
		{q: "1.5", r: "0.25", mode: fixedpoint.RoundExact, want: "0.375"},
		{q: "3", r: "1.5", mode: fixedpoint.RoundExact, want: "4.5"},
		{q: "-2", r: "0.5", mode: fixedpoint.RoundExact, want: "-1"},
		{q: "2", r: "0", mode: fixedpoint.RoundExact, want: "0"},

		// A third of a position is not representable, so the mode decides — and
		// RoundExact refuses to decide. RoundTowardZero is the sizing default
		// for the reason the doc comment gives: never size beyond what is
		// backed, on either side of zero.
		{q: "1", r: "0.333333333", mode: fixedpoint.RoundExact, want: "0.333333333"},
		{q: ulp, r: ulp, mode: fixedpoint.RoundExact, err: fixedpoint.ErrPrecisionLoss},
		{q: ulp, r: ulp, mode: fixedpoint.RoundTowardZero, want: "0"},
		{q: ulp, r: ulp, mode: fixedpoint.RoundAwayFromZero, want: ulp},
		{q: negUlp, r: ulp, mode: fixedpoint.RoundTowardZero, want: "0"},
		{q: negUlp, r: ulp, mode: fixedpoint.RoundAwayFromZero, want: negUlp},
		{q: negUlp, r: ulp, mode: fixedpoint.RoundFloor, want: negUlp},
		{q: negUlp, r: ulp, mode: fixedpoint.RoundCeil, want: "0"},

		// An unset mode fails even when the arithmetic is exact, so that a
		// suite written with round numbers cannot pass one through.
		{q: "2", r: "0.5", mode: fixedpoint.RoundInvalid, err: fixedpoint.ErrRoundingMode},

		{q: maxDecimal, r: "2", mode: fixedpoint.RoundExact, err: fixedpoint.ErrOverflow},
	}

	for _, tc := range cases {
		name := tc.q + " * " + tc.r + " " + tc.mode.String()
		t.Run(name, func(t *testing.T) {
			got, err := mustQuantity(t, btc, tc.q).Mul(mustRate(t, tc.r), tc.mode)
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
			// The asset rides through unchanged: a Rate is dimensionless, so
			// scaling a BTC size by one yields a BTC size and never an untagged
			// number that the next operation would have to guess about.
			if got.Asset() != btc {
				t.Errorf("Mul dropped the asset: got %q; want %q", got.Asset(), btc)
			}
		})
	}
}

// The unset guards, separated out because each has a different sentinel and the
// Rate side has no asset tag to be missing — it carries a construction flag, and
// this is the only Quantity operation that can report on it.
func TestQuantity_MulUnsetOperands(t *testing.T) {
	var unsetQ fixedpoint.Quantity
	var unsetR fixedpoint.Rate
	validQ := mustQuantity(t, btc, "1")
	validR := mustRate(t, "0.5")

	if _, err := unsetQ.Mul(validR, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Mul on the zero Quantity = %v; want ErrAssetUnset", err)
	}
	if _, err := validQ.Mul(unsetR, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrRateUnset) {
		t.Errorf("Mul by the zero Rate = %v; want ErrRateUnset", err)
	}
	// Both unset: the receiver is checked first, and which sentinel comes back
	// is part of the contract rather than an accident of statement order.
	if _, err := unsetQ.Mul(unsetR, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Mul with both operands unset = %v; want ErrAssetUnset", err)
	}
}
