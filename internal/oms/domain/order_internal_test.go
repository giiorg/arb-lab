package domain

import (
	"errors"
	"testing"
)

// TestValidatePriceFallthroughIsReachable checks the one branch of this package
// that no exported call can reach.
//
// Order.Validate runs Type.Validate first, so an undeclared type is refused
// before validatePrice sees it — which makes the unconditional refusal at the
// bottom of validatePrice dead code from outside, and dead code is exactly what
// a reader deletes. It is not dead: it is what makes the switch's two groups
// observable. With a trailing `return nil` there instead, deleting
// OrderTypeMarket from its case would move market orders into the "no price
// rule applies" answer that the fallthrough already gives, and no test in the
// exported suite could tell.
//
// So the branch is asserted from inside the package, and the test doubles as
// the record of why it exists. It becomes reachable from outside the day a third
// order type is declared without a case here — which §3.6 rule 10 turns into a
// build failure first.
func TestValidatePriceFallthroughIsReachable(t *testing.T) {
	t.Parallel()

	o := Order{ClientOrderID: "ab-x", Type: OrderType("stop_limit")}

	err := o.validatePrice()
	if !errors.Is(err, ErrOrderTypeUnhandled) {
		t.Fatalf("validatePrice() with an unhandled type = %v, want %v", err, ErrOrderTypeUnhandled)
	}
}

// TestValidatePriceGroupsAreDistinguishable is the positive half of the claim
// above: each declared type reaches its OWN case rather than the fallthrough.
//
// Without it, an implementation whose switch had no cases at all would pass the
// test above and fail nothing else — the universally-objecting predicate this
// repo requires a positive control against.
func TestValidatePriceGroupsAreDistinguishable(t *testing.T) {
	t.Parallel()

	for _, typ := range []OrderType{OrderTypeLimit, OrderTypeMarket} {
		o := Order{ClientOrderID: "ab-x", Type: typ}

		// A market order with no price is legal; a limit order with no price is
		// refused by fixedpoint rather than by the fallthrough. Neither answer is
		// ErrOrderTypeUnhandled, which is the claim.
		if err := o.validatePrice(); errors.Is(err, ErrOrderTypeUnhandled) {
			t.Errorf("validatePrice() with declared type %s reached the fallthrough", typ)
		}
	}
}
