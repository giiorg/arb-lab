package ratelimit_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// weightLimit is the shape every negative case below mutates one field of, so
// that each case fails the check it names rather than an earlier one.
func weightLimit() ratelimit.Limit {
	return ratelimit.Limit{
		Name:           "weight_1m",
		Scope:          ratelimit.ScopeFamily,
		Family:         ids.ProductSpot,
		Dimension:      ratelimit.DimensionWeight,
		Interval:       time.Minute,
		OpeningCeiling: 700,
		RoutineCeiling: 900,
		Capacity:       1000,
	}
}

func TestLimitValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*ratelimit.Limit)
		want    error
		wantMsg string
	}{
		// Positive controls across every scope.
		{"a family-scoped weight limit is usable", func(*ratelimit.Limit) {}, nil, ""},
		{"a venue-scoped envelope is usable", func(l *ratelimit.Limit) {
			l.Name = "ip_5s"
			l.Scope = ratelimit.ScopeVenue
			l.Family = ""
			l.Dimension = ratelimit.DimensionRequests
			l.Interval = 5 * time.Second
			l.OpeningCeiling, l.RoutineCeiling, l.Capacity = 420, 540, 600
		}, nil, ""},
		{"an endpoint-scoped request limit is usable", func(l *ratelimit.Limit) {
			l.Name = "order_realtime"
			l.Scope = ratelimit.ScopeEndpoint
			l.Family = ""
			l.Dimension = ratelimit.DimensionRequests
			l.Interval = time.Second
			l.OpeningCeiling, l.RoutineCeiling, l.Capacity = 35, 45, 50
		}, nil, ""},
		{"the three ceilings may coincide", func(l *ratelimit.Limit) {
			l.OpeningCeiling, l.RoutineCeiling, l.Capacity = 1000, 1000, 1000
		}, nil, ""},

		{"an unset name is refused", func(l *ratelimit.Limit) { l.Name = "" },
			ratelimit.ErrLimitNameUnset, ""},
		{"an uppercase name is refused", func(l *ratelimit.Limit) { l.Name = "Weight_1m" },
			ratelimit.ErrLimitNameInvalid, ""},
		{"a name with a hyphen is refused", func(l *ratelimit.Limit) { l.Name = "weight-1m" },
			ratelimit.ErrLimitNameInvalid, ""},
		{"an over-long name is refused", func(l *ratelimit.Limit) { l.Name = strings.Repeat("w", 65) },
			ratelimit.ErrLimitNameInvalid, ""},

		{"an unset scope is refused", func(l *ratelimit.Limit) { l.Scope = ratelimit.ScopeUnspecified },
			ratelimit.ErrScopeInvalid, ""},
		{"an unset dimension is refused", func(l *ratelimit.Limit) { l.Dimension = ratelimit.DimensionUnspecified },
			ratelimit.ErrDimensionInvalid, ""},
		{"an endpoint-scoped weight limit is refused", func(l *ratelimit.Limit) {
			l.Scope = ratelimit.ScopeEndpoint
			l.Family = ""
		}, ratelimit.ErrLimitEndpointDimension, "endpoint"},

		{"a zero interval is refused", func(l *ratelimit.Limit) { l.Interval = 0 },
			ratelimit.ErrLimitInterval, ""},
		{"a negative interval is refused", func(l *ratelimit.Limit) { l.Interval = -time.Second },
			ratelimit.ErrLimitInterval, ""},

		// Four ceiling failures share one sentinel, so each asserts the message
		// too: one sentinel cannot say which rung of the ladder is wrong, and
		// the message is what an operator acts on.
		{"a zero capacity is refused", func(l *ratelimit.Limit) {
			l.OpeningCeiling, l.RoutineCeiling, l.Capacity = 0, 0, 0
		}, ratelimit.ErrLimitCeilings, "capacity"},
		{"a zero opening ceiling is refused", func(l *ratelimit.Limit) { l.OpeningCeiling = 0 },
			ratelimit.ErrLimitCeilings, "opening"},
		{"an opening ceiling above the routine ceiling is refused", func(l *ratelimit.Limit) {
			l.OpeningCeiling = 950
		}, ratelimit.ErrLimitCeilings, "routine"},
		{"a routine ceiling above capacity is refused", func(l *ratelimit.Limit) {
			l.RoutineCeiling = 1100
		}, ratelimit.ErrLimitCeilings, "capacity"},

		{"a venue-scoped limit naming a family is refused", func(l *ratelimit.Limit) {
			l.Scope = ratelimit.ScopeVenue
		}, ratelimit.ErrLimitFamily, "venue"},
		{"a family-scoped limit with no family is refused", func(l *ratelimit.Limit) {
			l.Family = ""
		}, ratelimit.ErrLimitFamily, "family"},
		{"a family-scoped limit with an unknown family is refused", func(l *ratelimit.Limit) {
			l.Family = ids.ProductCategory("perp")
		}, ids.ErrProductUnknown, ""},
		{"an endpoint-scoped limit naming a family is refused", func(l *ratelimit.Limit) {
			// Family is held populated while only scope changes, so this checks
			// the inversion of the family-scope rule rather than an empty fixture.
			l.Scope = ratelimit.ScopeEndpoint
		}, ratelimit.ErrLimitFamily, "endpoint"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := weightLimit()
			tc.mutate(&l)

			err := l.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Limit.Validate() = %v, want %v", err, tc.want)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("Limit.Validate() = %q, want the message to contain %q", err, tc.wantMsg)
			}
		})
	}
}

// The ladder is the user-decided semantics of the package: safety's ceiling IS
// capacity, so no class is ever admitted past the venue's own budget, and the
// band between the routine ceiling and capacity is §7.1's reserved allocation.
func TestLimitCeilingIsALadderToppingOutAtCapacity(t *testing.T) {
	t.Parallel()

	l := weightLimit()

	if got := l.Ceiling(ratelimit.PriorityOpening); got != l.OpeningCeiling {
		t.Errorf("Ceiling(PriorityOpening) = %d, want %d", got, l.OpeningCeiling)
	}
	if got := l.Ceiling(ratelimit.PriorityRoutine); got != l.RoutineCeiling {
		t.Errorf("Ceiling(PriorityRoutine) = %d, want %d", got, l.RoutineCeiling)
	}
	if got := l.Ceiling(ratelimit.PrioritySafety); got != l.Capacity {
		t.Errorf("Ceiling(PrioritySafety) = %d, want Capacity %d — safety gets the reserve, "+
			"not a bypass", got, l.Capacity)
	}
}

// An unusable priority admits nothing. The alternative — falling through to
// Capacity — would make a forgotten field the most privileged one in the system.
func TestLimitCeilingOfAnInvalidPriorityIsZero(t *testing.T) {
	t.Parallel()

	l := weightLimit()
	for _, p := range []ratelimit.Priority{ratelimit.PriorityUnspecified, ratelimit.Priority(200)} {
		if got := l.Ceiling(p); got != 0 {
			t.Errorf("Ceiling(%s) = %d, want 0", p, got)
		}
	}
}

// The reserve is the whole point of the ladder, so it is named rather than left
// as a subtraction a reader has to spot.
func TestLimitReserveIsTheBandAboveTheRoutineCeiling(t *testing.T) {
	t.Parallel()

	l := weightLimit()
	if got, want := l.Reserve(), uint64(100); got != want {
		t.Errorf("Reserve() = %d, want %d (capacity %d − routine ceiling %d)",
			got, want, l.Capacity, l.RoutineCeiling)
	}

	l.RoutineCeiling = l.Capacity
	if got := l.Reserve(); got != 0 {
		t.Errorf("Reserve() = %d, want 0 when the routine ceiling is capacity", got)
	}

	// Reserve is exported and callable on a Limit that never passed Validate,
	// so the guard has to hold above capacity too — and `>=` versus `>` are the
	// same predicate at exactly capacity, which is where the case above pins
	// the fixture. Vary it, or the subtraction underflows to 18 quintillion and
	// the reserve reads as unlimited.
	l.RoutineCeiling = l.Capacity + 1
	if got := l.Reserve(); got != 0 {
		t.Errorf("Reserve() = %d, want 0 when the routine ceiling is above capacity", got)
	}
}

func TestCostValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    ratelimit.Cost
		want error
	}{
		{"a query costs weight only", ratelimit.Cost{Weight: 2}, nil},
		{"an order costs weight and an order slot", ratelimit.Cost{Weight: 1, Orders: 1}, nil},

		// The row this check was rewritten for. Binance documents POST
		// /fapi/v1/order as "0 on IP rate limit(x-mbx-used-weight-1m)" and 1 on
		// each order-count limit (Profiles §7), so a weightless order is a
		// VENUE FACT and the earlier rule refused the truthful cost of the most
		// important call in the system.
		{"a futures order costs no weight and one order slot", ratelimit.Cost{Orders: 1}, nil},

		// What survives: a cost charging no dimension a caller fills in is the
		// field nobody populated, and a bucket that never fills looks exactly
		// like a system comfortably inside its budget right up to the ban.
		{"the zero value is refused", ratelimit.Cost{}, ratelimit.ErrCostUnset},

		// The one legitimate way to charge neither field (task #173). Bybit's
		// read surfaces are metered per request alone, and their truthful cost
		// is indistinguishable from the zero value without a carrier — so the
		// carrier is a constructor, and the zero value stays refused.
		{"a request-only cost is valid", ratelimit.RequestOnly(), nil},
		{"a request-only cost that still charges weight is refused",
			func() ratelimit.Cost { c := ratelimit.RequestOnly(); c.Weight = 5; return c }(),
			ratelimit.ErrCostContradicted},
		{"a request-only cost that still charges an order slot is refused",
			func() ratelimit.Cost { c := ratelimit.RequestOnly(); c.Orders = 1; return c }(),
			ratelimit.ErrCostContradicted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.c.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Cost%+v.Validate() = %v, want %v", tc.c, err, tc.want)
			}
		})
	}
}
