package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The zero value must be INVALID.
//
// A Money field nobody populated has to fail loudly rather than read as "zero
// of some currency". The concrete failure this prevents: a cost field a venue
// adapter forgot to fill would otherwise become a *free* cost, and milestone
// 2's economic gate (≈3.4% measured against a ~4.6% hurdle, §12.2) would be
// computed in our favour with nothing failing.
func TestMoney_ZeroValueIsInvalid(t *testing.T) {
	var m fixedpoint.Money

	err := m.Validate()
	if err == nil {
		t.Fatal("Validate() on the zero value returned nil; want an error (asset unset)")
	}
	if !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Fatalf("Validate() = %v; want it to wrap ErrAssetUnset", err)
	}
}
