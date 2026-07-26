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
