package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// allSides, allOrderTypes and allTimesInForce are the declared members, written
// out rather than derived.
//
// Same argument as allStates in orderstate_test.go: a list built from the code
// under test cannot notice a member the code forgot, which is the defect
// platform/events carried for four slices when StreamOrders was missing.
var (
	allSides        = []domain.Side{domain.SideBuy, domain.SideSell}
	allOrderTypes   = []domain.OrderType{domain.OrderTypeLimit, domain.OrderTypeMarket}
	allTimesInForce = []domain.TimeInForce{
		domain.TimeInForceIOC,
		domain.TimeInForceGTC,
		domain.TimeInForceFOK,
		domain.TimeInForceGTD,
	}
)

// price builds a valid limit price.
func price(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

// validOrder is §6 step 6's own shape — a marketable-IOC limit — so every test
// below varies exactly the field it is about.
//
// The side is SELL because that is the perp leg of a cash-and-carry trade, and
// the tests that care about direction vary it explicitly rather than relying on
// this default: a suite whose every fixture is one side cannot tell "checks the
// side is declared" from "checks the side is sell".
func validOrder(t *testing.T) domain.Order {
	t.Helper()
	return domain.Order{
		ClientOrderID: order,
		Instrument:    perp(),
		Side:          domain.SideSell,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, "0.5"),
		LimitPrice:    price(t, "50000"),
	}
}

func TestOrderValidateAcceptsAValidOrder(t *testing.T) {
	t.Parallel()

	// The positive control. Without it every refusal below is satisfied by a
	// Validate that objects to everything.
	if err := validOrder(t).Validate(); err != nil {
		t.Fatalf("Validate() on a valid order = %v, want nil", err)
	}
}

// TestOrderValidateAcceptsEitherSide pins the claim that Validate checks the
// side is DECLARED rather than that it is one particular direction.
//
// A cash-and-carry package holds one order of each side at the same instant, so
// a Validate that admitted only one would refuse half of every trade — and the
// single-fixture suite that would miss it is the collapse this repo has now paid
// for nine times.
func TestOrderValidateAcceptsEitherSide(t *testing.T) {
	t.Parallel()

	for _, side := range allSides {
		o := validOrder(t)
		o.Side = side
		if err := o.Validate(); err != nil {
			t.Errorf("Validate() with side %s = %v, want nil", side, err)
		}
	}
}

// TestOrderValidateAcceptsEveryTimeInForce is the same claim for the four-member
// enum, and it matters more here: §6 step 6 lets this system SEND only IOC, so
// the tempting implementation refuses the other three at the type — which would
// make every recovered GTC order on the account unrepresentable.
func TestOrderValidateAcceptsEveryTimeInForce(t *testing.T) {
	t.Parallel()

	for _, tif := range allTimesInForce {
		o := validOrder(t)
		o.TimeInForce = tif
		if err := o.Validate(); err != nil {
			t.Errorf("Validate() with time in force %s = %v, want nil", tif, err)
		}
	}
}

// TestOrderValidatePriceTypePairing walks all FOUR cells of the type × price
// matrix, not the two on the diagonal.
//
// The collapse this avoids is specific. Testing only "limit with a price is
// fine" and "limit without one is refused" leaves the market row entirely, so an
// implementation that checks nothing about market orders passes — and that is
// the direction that fails silently, since §6 step 4 sizes the naked-leg gap
// against a bound the venue will ignore.
func TestOrderValidatePriceTypePairing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		typ      domain.OrderType
		hasPrice bool
		want     error // nil means the pairing is legal
	}{
		{"limit with a price", domain.OrderTypeLimit, true, nil},
		{"limit without a price", domain.OrderTypeLimit, false, fixedpoint.ErrAssetUnset},
		{"market without a price", domain.OrderTypeMarket, false, nil},
		{"market with a price", domain.OrderTypeMarket, true, domain.ErrMarketOrderHasLimitPrice},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := validOrder(t)
			o.Type = tc.typ
			if !tc.hasPrice {
				o.LimitPrice = fixedpoint.Price{}
			}

			err := o.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestOrderValidateRefusesBadFields varies one field at a time from a valid
// order, so each case names the check that fired rather than the first of
// several.
func TestOrderValidateRefusesBadFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*testing.T, *domain.Order)
		want   error
	}{
		{
			name:   "no client order id",
			mutate: func(_ *testing.T, o *domain.Order) { o.ClientOrderID = "" },
			want:   domain.ErrOrderUnset,
		},
		{
			name:   "instrument contradicts itself",
			mutate: func(_ *testing.T, o *domain.Order) { o.Instrument.Contract = "inverse" },
			want:   nil, // any error; ids owns the sentinel
		},
		{
			// Empty and misspelt are one sentinel here by decision, and both are
			// exercised: a suite testing only "" passes against a Validate that
			// checks for emptiness alone, and the mapping bug that actually
			// happens is a venue token reaching the field unnormalised.
			name:   "side is empty",
			mutate: func(_ *testing.T, o *domain.Order) { o.Side = "" },
			want:   domain.ErrSideInvalid,
		},
		{
			name:   "side is a venue spelling",
			mutate: func(_ *testing.T, o *domain.Order) { o.Side = "SELL" },
			want:   domain.ErrSideInvalid,
		},
		{
			name:   "type is empty",
			mutate: func(_ *testing.T, o *domain.Order) { o.Type = "" },
			want:   domain.ErrOrderTypeInvalid,
		},
		{
			name:   "type is a venue spelling",
			mutate: func(_ *testing.T, o *domain.Order) { o.Type = "LIMIT" },
			want:   domain.ErrOrderTypeInvalid,
		},
		{
			name:   "time in force is empty",
			mutate: func(_ *testing.T, o *domain.Order) { o.TimeInForce = "" },
			want:   domain.ErrTimeInForceInvalid,
		},
		{
			name:   "time in force is a venue spelling",
			mutate: func(_ *testing.T, o *domain.Order) { o.TimeInForce = "IOC" },
			want:   domain.ErrTimeInForceInvalid,
		},
		{
			name:   "quantity carries no asset",
			mutate: func(_ *testing.T, o *domain.Order) { o.Quantity = fixedpoint.Quantity{} },
			want:   fixedpoint.ErrAssetUnset,
		},
		{
			// Zero and negative are separate cases because `sign <= 0` and
			// `sign < 0` give the same answer on a negative quantity. Zero is the
			// one that inverts §6.7's classification while looking like a
			// harmless empty order.
			name:   "quantity is zero",
			mutate: func(t *testing.T, o *domain.Order) { o.Quantity = qty(t, "0") },
			want:   domain.ErrOrderedQuantityNotPositive,
		},
		{
			name:   "quantity is negative",
			mutate: func(t *testing.T, o *domain.Order) { o.Quantity = qty(t, "-0.5") },
			want:   domain.ErrOrderedQuantityNotPositive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := validOrder(t)
			tc.mutate(t, &o)

			err := o.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestOrderValidateNamesTheOrder checks that a refusal carries the client order
// id.
//
// One sentinel cannot say which order failed, and a dispatch path refusing an
// order without naming it leaves an operator with a package of two legs and no
// way to tell which one was rejected.
func TestOrderValidateNamesTheOrder(t *testing.T) {
	t.Parallel()

	o := validOrder(t)
	o.Side = "SELL"

	err := o.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	if !strings.Contains(err.Error(), string(order)) {
		t.Fatalf("Validate() = %q, want it to name order %q", err, order)
	}
}

// TestOrderValidateUnsetIDIsNotWrapped pins the one deliberate asymmetry: the
// missing-id refusal is the bare sentinel, because there is no id to name and a
// wrap would print an empty pair of quotes as if it were the answer.
func TestOrderValidateUnsetIDIsNotWrapped(t *testing.T) {
	t.Parallel()

	o := validOrder(t)
	o.ClientOrderID = ""

	if err := o.Validate(); !errors.Is(err, domain.ErrOrderUnset) {
		t.Fatalf("Validate() = %v, want %v", err, domain.ErrOrderUnset)
	}
}

func TestSideValidate(t *testing.T) {
	t.Parallel()

	for _, s := range allSides {
		if err := s.Validate(); err != nil {
			t.Errorf("Side(%q).Validate() = %v, want nil", s, err)
		}
		if s.String() != string(s) {
			t.Errorf("Side(%q).String() = %q", s, s.String())
		}
	}
	for _, s := range []domain.Side{"", "BUY", "long", "b"} {
		if err := s.Validate(); !errors.Is(err, domain.ErrSideInvalid) {
			t.Errorf("Side(%q).Validate() = %v, want %v", s, err, domain.ErrSideInvalid)
		}
	}
}

func TestOrderTypeValidate(t *testing.T) {
	t.Parallel()

	for _, ot := range allOrderTypes {
		if err := ot.Validate(); err != nil {
			t.Errorf("OrderType(%q).Validate() = %v, want nil", ot, err)
		}
		if ot.String() != string(ot) {
			t.Errorf("OrderType(%q).String() = %q", ot, ot.String())
		}
	}
	for _, ot := range []domain.OrderType{"", "LIMIT", "stop", "limit_maker"} {
		if err := ot.Validate(); !errors.Is(err, domain.ErrOrderTypeInvalid) {
			t.Errorf("OrderType(%q).Validate() = %v, want %v", ot, err, domain.ErrOrderTypeInvalid)
		}
	}
}

func TestTimeInForceValidate(t *testing.T) {
	t.Parallel()

	for _, tif := range allTimesInForce {
		if err := tif.Validate(); err != nil {
			t.Errorf("TimeInForce(%q).Validate() = %v, want nil", tif, err)
		}
		if tif.String() != string(tif) {
			t.Errorf("TimeInForce(%q).String() = %q", tif, tif.String())
		}
	}
	for _, tif := range []domain.TimeInForce{"", "IOC", "day", "post_only"} {
		if err := tif.Validate(); !errors.Is(err, domain.ErrTimeInForceInvalid) {
			t.Errorf("TimeInForce(%q).Validate() = %v, want %v", tif, err, domain.ErrTimeInForceInvalid)
		}
	}
}

// TestDeclaredSetsAreDistinct catches a copy-paste that gives two constants the
// same string, which would make the enum silently smaller than it reads.
func TestDeclaredSetsAreDistinct(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, s := range allSides {
		if seen[string(s)] {
			t.Errorf("side %q is declared twice", s)
		}
		seen[string(s)] = true
	}
	seen = map[string]bool{}
	for _, ot := range allOrderTypes {
		if seen[string(ot)] {
			t.Errorf("order type %q is declared twice", ot)
		}
		seen[string(ot)] = true
	}
	seen = map[string]bool{}
	for _, tif := range allTimesInForce {
		if seen[string(tif)] {
			t.Errorf("time in force %q is declared twice", tif)
		}
		seen[string(tif)] = true
	}
}
