package ratelimit_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

var start = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

func newGovernor(t *testing.T) (*ratelimit.Governor, *clock.TestClock) {
	t.Helper()

	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, binanceShaped())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g, tc
}

func endpointShaped() ratelimit.Profile {
	return ratelimit.Profile{
		Venue:    ids.Venue("bybit"),
		Families: []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp},
		Limits: []ratelimit.Limit{
			{
				Name: "ip_5s", Scope: ratelimit.ScopeVenue,
				Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
				OpeningCeiling: 80, RoutineCeiling: 90, Capacity: 100,
			},
			{
				Name: "read_a", Scope: ratelimit.ScopeEndpoint,
				Dimension: ratelimit.DimensionRequests, Interval: time.Second,
				OpeningCeiling: 1, RoutineCeiling: 2, Capacity: 2,
			},
			{
				// A different capacity makes an accidental cross-charge visible as
				// arithmetic, rather than only as a refusal.
				Name: "read_b", Scope: ratelimit.ScopeEndpoint,
				Dimension: ratelimit.DimensionRequests, Interval: time.Second,
				OpeningCeiling: 2, RoutineCeiling: 3, Capacity: 3,
			},
			{
				Name: "order_create", Scope: ratelimit.ScopeEndpoint,
				Dimension: ratelimit.DimensionOrders, Interval: time.Second,
				OpeningCeiling: 7, RoutineCeiling: 9, Capacity: 10,
			},
		},
		Uncounted: []ratelimit.Uncounted{
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionWeight, Reason: "no weight budget exists"},
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered per endpoint"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionWeight, Reason: "no weight budget exists"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered per endpoint"},
		},
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	}
}

func newEndpointGovernor(t *testing.T) (*ratelimit.Governor, *clock.TestClock) {
	t.Helper()
	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, endpointShaped())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g, tc
}

func endpointRead(f ids.ProductCategory, name string, p ratelimit.Priority) ratelimit.Request {
	return ratelimit.Request{
		Family: f, Endpoint: name, Priority: p, Cost: ratelimit.RequestOnly(),
	}
}

func spotQuery(p ratelimit.Priority, weight uint64) ratelimit.Request {
	return ratelimit.Request{
		Family:   ids.ProductSpot,
		Priority: p,
		Cost:     ratelimit.Cost{Weight: weight},
	}
}

func perpOrder(p ratelimit.Priority) ratelimit.Request {
	return ratelimit.Request{
		Family:   ids.ProductLinearPerp,
		Priority: p,
		Cost:     ratelimit.Cost{Weight: 1, Orders: 1},
	}
}

// mustAcquire charges the request and fails the test if it was refused.
func mustAcquire(t *testing.T, g *ratelimit.Governor, r ratelimit.Request) {
	t.Helper()
	d, err := g.Acquire(r)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !d.Admitted {
		t.Fatalf("Acquire refused (%s, limit %q), want admitted", d.Reason, d.Limit)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}

	t.Run("a valid profile constructs", func(t *testing.T) {
		t.Parallel()
		if _, err := ratelimit.New(tc, binanceShaped()); err != nil {
			t.Fatalf("New: %v", err)
		}
	})
	t.Run("a nil clock is refused", func(t *testing.T) {
		t.Parallel()
		if _, err := ratelimit.New(nil, binanceShaped()); !errors.Is(err, ratelimit.ErrClockNil) {
			t.Fatalf("New(nil, …) = %v, want ErrClockNil", err)
		}
	})
	t.Run("an invalid profile is refused", func(t *testing.T) {
		t.Parallel()
		p := binanceShaped()
		p.Limits[0].Capacity = 0
		if _, err := ratelimit.New(tc, p); !errors.Is(err, ratelimit.ErrLimitCeilings) {
			t.Fatalf("New with a broken profile = %v, want ErrLimitCeilings", err)
		}
	})
}

func TestAcquireRefusesMalformedRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  ratelimit.Request
		want error
	}{
		{"an unset priority", ratelimit.Request{
			Family: ids.ProductSpot, Cost: ratelimit.Cost{Weight: 1},
		}, ratelimit.ErrPriorityInvalid},
		{"an unset cost", ratelimit.Request{
			Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine,
		}, ratelimit.ErrCostUnset},

		// A family the profile never declared would otherwise match no
		// family-scoped limit at all and sail past every product budget,
		// charging only the venue envelope — governed on paper, ungoverned in
		// fact.
		{"an undeclared family", ratelimit.Request{
			Family: ids.ProductInversePerp, Priority: ratelimit.PriorityRoutine,
			Cost: ratelimit.Cost{Weight: 1},
		}, ratelimit.ErrFamilyUnknown},
		{"an unset family", ratelimit.Request{
			Priority: ratelimit.PriorityRoutine, Cost: ratelimit.Cost{Weight: 1},
		}, ids.ErrProductUnset},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, _ := newGovernor(t)
			if _, err := g.Acquire(tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("Acquire = %v, want %v", err, tc.want)
			}
			if _, err := g.Check(tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("Check = %v, want %v", err, tc.want)
			}
		})
	}
}

// The ladder, end to end on one budget: openings stop at 70, routine reads keep
// going to 90, and the reserve above 90 is safety's alone.
func TestAcquireShedsTrafficUpTheLadder(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	// Fill the spot weight budget to its opening ceiling with seven charges of
	// ten, which leaves the venue envelope (42 requests) far from binding.
	for i := 0; i < 7; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PriorityOpening, 10))
	}

	d, err := g.Check(spotQuery(ratelimit.PriorityOpening, 1))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Admitted {
		t.Error("an opening was admitted at the opening ceiling, want refused")
	}
	if d.Limit != "weight_1m" {
		t.Errorf("refusal names limit %q, want weight_1m", d.Limit)
	}
	if d.Reason != ratelimit.ReasonCeiling {
		t.Errorf("refusal reason = %s, want ReasonCeiling", d.Reason)
	}

	// Routine still runs: monitoring outranks new exposure.
	for i := 0; i < 2; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PriorityRoutine, 10))
	}
	if d, _ := g.Check(spotQuery(ratelimit.PriorityRoutine, 1)); d.Admitted {
		t.Error("a routine read was admitted at the routine ceiling, want refused")
	}

	// The reserve — §7.1's reserved allocation — is safety's alone.
	if d, _ := g.Check(spotQuery(ratelimit.PrioritySafety, 10)); !d.Admitted {
		t.Error("safety was refused inside the reserve, want admitted")
	}
	for i := 0; i < 1; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PrioritySafety, 10))
	}

	// And capacity is a ceiling for safety too — the user's decision, and the
	// one line in the package that would be a bypass if it were wrong.
	d, _ = g.Check(spotQuery(ratelimit.PrioritySafety, 1))
	if d.Admitted {
		t.Error("safety was admitted past capacity, want refused: capacity is a ceiling, not a bypass")
	}
}

// Check answers the §6.3 risk-gate question without spending the budget it is
// reporting on.
func TestCheckDoesNotChargeAndAcquireDoes(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	for i := 0; i < 5; i++ {
		if _, err := g.Check(spotQuery(ratelimit.PriorityRoutine, 10)); err != nil {
			t.Fatalf("Check: %v", err)
		}
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 0 {
		t.Errorf("used after five Checks = %d, want 0", got)
	}

	mustAcquire(t, g, spotQuery(ratelimit.PriorityRoutine, 10))
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 10 {
		t.Errorf("used after one Acquire = %d, want 10", got)
	}
}

// A refused request charges NOTHING. Otherwise a request that the order budget
// turned away would still have spent the weight budget that admitted it, and a
// caller retrying in a loop would drain a budget it never got to use.
func TestAcquireChargesAllOrNothing(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	// Fill the perp order budget to capacity; each order also costs 1 weight.
	for i := 0; i < 10; i++ {
		mustAcquire(t, g, perpOrder(ratelimit.PrioritySafety))
	}
	weightBefore := usedOf(t, g, ids.ProductLinearPerp, "weight_1m")

	d, err := g.Acquire(perpOrder(ratelimit.PrioritySafety))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if d.Admitted {
		t.Fatal("an order was admitted past the order capacity, want refused")
	}
	if d.Limit != "orders_10s" {
		t.Errorf("refusal names limit %q, want orders_10s", d.Limit)
	}

	if got := usedOf(t, g, ids.ProductLinearPerp, "weight_1m"); got != weightBefore {
		t.Errorf("weight used = %d after a refused order, want %d unchanged", got, weightBefore)
	}
	if got := usedOf(t, g, ids.ProductLinearPerp, "orders_10s"); got != 10 {
		t.Errorf("orders used = %d after a refused order, want 10 unchanged", got)
	}
}

// A dimension a request does not touch cannot refuse it: an exhausted order
// budget must not stop the market-data reads that decide what to do next.
func TestAnExhaustedOrderBudgetDoesNotBlockQueries(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	for i := 0; i < 10; i++ {
		mustAcquire(t, g, perpOrder(ratelimit.PrioritySafety))
	}

	query := ratelimit.Request{
		Family: ids.ProductLinearPerp, Priority: ratelimit.PriorityRoutine,
		Cost: ratelimit.Cost{Weight: 1},
	}
	if d, _ := g.Check(query); !d.Admitted {
		t.Error("a weight-only query was refused by an exhausted order budget, want admitted")
	}
}

// The shared IP envelope is §7.6's hedge against a ban track the venues do not
// document. It binds across families, and the family budgets do not.
func TestTheVenueEnvelopeBindsAcrossFamilies(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	// 42 one-weight spot requests reach the envelope's opening ceiling while
	// the spot weight budget (70) is still well short.
	for i := 0; i < 42; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PriorityOpening, 1))
	}

	d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1))
	if d.Admitted || d.Limit != "ip_5s" {
		t.Errorf("spot opening: admitted=%v limit=%q, want refused by ip_5s", d.Admitted, d.Limit)
	}

	// The other family has spent none of its own weight and is refused anyway.
	perp := ratelimit.Request{
		Family: ids.ProductLinearPerp, Priority: ratelimit.PriorityOpening,
		Cost: ratelimit.Cost{Weight: 1},
	}
	d, _ = g.Check(perp)
	if d.Admitted || d.Limit != "ip_5s" {
		t.Errorf("perp opening: admitted=%v limit=%q, want refused by the shared ip_5s envelope",
			d.Admitted, d.Limit)
	}
	if got := usedOf(t, g, ids.ProductLinearPerp, "weight_1m"); got != 0 {
		t.Errorf("perp weight used = %d, want 0 — the family budget is untouched", got)
	}

	// Above the envelope's opening ceiling, safety still has the reserve.
	if d, _ := g.Check(perpOrder(ratelimit.PrioritySafety)); !d.Admitted {
		t.Error("safety was refused inside the envelope's reserve, want admitted")
	}
}

// A family budget is not shared: exhausting spot leaves perp untouched.
func TestFamilyBudgetsAreIndependent(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	for i := 0; i < 10; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PrioritySafety, 10))
	}

	if d, _ := g.Check(spotQuery(ratelimit.PrioritySafety, 1)); d.Admitted {
		t.Fatal("spot admitted past its capacity, want refused")
	}
	perp := ratelimit.Request{
		Family: ids.ProductLinearPerp, Priority: ratelimit.PriorityOpening,
		Cost: ratelimit.Cost{Weight: 10},
	}
	if d, _ := g.Check(perp); !d.Admitted {
		t.Error("perp was refused because spot was exhausted, want admitted")
	}
}

// NotBefore is a promise, so it is tested as one: advancing the clock to exactly
// that instant makes the same request succeed, and stopping one nanosecond short
// does not.
func TestNotBeforeIsAnInstantTheRetrySucceedsAt(t *testing.T) {
	t.Parallel()

	g, tc := newGovernor(t)
	for i := 0; i < 10; i++ {
		mustAcquire(t, g, spotQuery(ratelimit.PrioritySafety, 10))
		if err := tc.Advance(time.Second); err != nil {
			t.Fatalf("Advance: %v", err)
		}
	}

	d, err := g.Acquire(spotQuery(ratelimit.PrioritySafety, 10))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if d.Admitted {
		t.Fatal("admitted past capacity, want refused")
	}
	if d.NotBefore.IsZero() {
		t.Fatal("NotBefore is the zero time, want an instant")
	}

	if err := tc.SetNow(d.NotBefore.Add(-1)); err != nil {
		t.Fatalf("SetNow: %v", err)
	}
	if retry, _ := g.Check(spotQuery(ratelimit.PrioritySafety, 10)); retry.Admitted {
		t.Error("admitted one nanosecond before NotBefore, want refused")
	}

	if err := tc.SetNow(d.NotBefore); err != nil {
		t.Fatalf("SetNow: %v", err)
	}
	retry, err := g.Acquire(spotQuery(ratelimit.PrioritySafety, 10))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !retry.Admitted {
		t.Errorf("refused at NotBefore (%s, limit %q) — NotBefore must be an instant a retry succeeds at",
			retry.Reason, retry.Limit)
	}
}

// ladderedProfile puts the SLOWER-freeing budget SECOND in profile order.
//
// That ordering is the whole point of the fixture. Limits are evaluated in
// profile order, so in a profile where the worst refusal happens to come first,
// "name the first refusal" and "name the worst refusal" are the same answer —
// and the two tests below would pass against either. This repo has now collapsed
// a fixture dimension five times; the tell is the same each time, which is that
// the fixture does not contain the thing under test.
func ladderedProfile() ratelimit.Profile {
	return ratelimit.Profile{
		Venue:    ids.Venue("laddered"),
		Families: []ids.ProductCategory{ids.ProductLinearPerp},
		Limits: []ratelimit.Limit{
			{
				Name: "orders_10s", Scope: ratelimit.ScopeFamily, Family: ids.ProductLinearPerp,
				Dimension: ratelimit.DimensionOrders, Interval: 10 * time.Second,
				OpeningCeiling: 7, RoutineCeiling: 9, Capacity: 10,
			},
			{
				Name: "weight_1m", Scope: ratelimit.ScopeFamily, Family: ids.ProductLinearPerp,
				Dimension: ratelimit.DimensionWeight, Interval: time.Minute,
				OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100,
			},
			{
				// The mandatory venue envelope, sized so it NEVER refuses in
				// this file: the fixtures below reason about which of the two
				// laddered budgets frees last, and a third refusing window
				// would rejoin exactly the collapse this fixture exists to
				// avoid. The suite's largest acquire count is well under 100.
				Name: "ip_5s", Scope: ratelimit.ScopeVenue,
				Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
				OpeningCeiling: 100000, RoutineCeiling: 100000, Capacity: 100000,
			},
		},
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	}
}

// newExhaustedLaddered returns a governor with both budgets full, the
// second-listed one (weight_1m, a one-minute window) freeing a full 50 seconds
// after the first-listed one (orders_10s).
func newExhaustedLaddered(t *testing.T) (*ratelimit.Governor, *clock.TestClock) {
	t.Helper()

	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, ladderedProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for i := 0; i < 10; i++ {
		mustAcquire(t, g, perpOrder(ratelimit.PrioritySafety))
	}
	if err := tc.Advance(time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	for i := 0; i < 9; i++ {
		mustAcquire(t, g, ratelimit.Request{
			Family: ids.ProductLinearPerp, Priority: ratelimit.PrioritySafety,
			Cost: ratelimit.Cost{Weight: 10},
		})
	}
	return g, tc
}

// When two budgets refuse at once, NotBefore is the LATER of the two. Naming the
// earlier one sends the caller back to be refused again by the other, which
// turns a backoff into a poll against a venue already declining traffic.
func TestNotBeforeIsTheLatestOfEveryRefusingLimit(t *testing.T) {
	t.Parallel()

	g, tc := newExhaustedLaddered(t)

	d, err := g.Acquire(perpOrder(ratelimit.PrioritySafety))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if d.Admitted {
		t.Fatal("admitted with both budgets exhausted, want refused")
	}
	if d.Limit != "weight_1m" {
		t.Errorf("refusal names %q, want weight_1m — the later of the two to free, "+
			"and deliberately not the first one evaluated", d.Limit)
	}

	if err := tc.SetNow(d.NotBefore); err != nil {
		t.Fatalf("SetNow: %v", err)
	}
	if retry, _ := g.Check(perpOrder(ratelimit.PrioritySafety)); !retry.Admitted {
		t.Error("refused at NotBefore, want admitted — NotBefore must clear every refusing limit")
	}
}

func TestObserve(t *testing.T) {
	t.Parallel()

	t.Run("a reading raises the local count", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		mustAcquire(t, g, spotQuery(ratelimit.PriorityOpening, 10))

		if err := g.Observe(ids.ProductSpot, "weight_1m", 95); err != nil {
			t.Fatalf("Observe: %v", err)
		}
		if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 95 {
			t.Errorf("used = %d, want 95", got)
		}
		if d, _ := g.Check(spotQuery(ratelimit.PriorityRoutine, 1)); d.Admitted {
			t.Error("a routine read was admitted above the routine ceiling after a reading of 95")
		}
	})

	t.Run("an empty family addresses the venue-scoped limit", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		if err := g.Observe("", "ip_5s", 55); err != nil {
			t.Fatalf("Observe: %v", err)
		}
		if got := usedOf(t, g, "", "ip_5s"); got != 55 {
			t.Errorf("used = %d, want 55", got)
		}
	})

	t.Run("an unknown limit is refused", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		err := g.Observe(ids.ProductSpot, "weight_5m", 10)
		if !errors.Is(err, ratelimit.ErrLimitUnknown) {
			t.Fatalf("Observe = %v, want ErrLimitUnknown", err)
		}
	})

	t.Run("a limit from the wrong scope is refused", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		// ip_5s is venue-scoped, so it is not addressable under a family.
		if err := g.Observe(ids.ProductSpot, "ip_5s", 10); !errors.Is(err, ratelimit.ErrLimitUnknown) {
			t.Fatalf("Observe = %v, want ErrLimitUnknown", err)
		}
	})

	t.Run("an unknown family is refused", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		err := g.Observe(ids.ProductInversePerp, "weight_1m", 10)
		if !errors.Is(err, ratelimit.ErrFamilyUnknown) {
			t.Fatalf("Observe = %v, want ErrFamilyUnknown", err)
		}
	})
}

func TestPenalize(t *testing.T) {
	t.Parallel()

	t.Run("a pause refuses every priority, safety included", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}

		for _, p := range []ratelimit.Priority{
			ratelimit.PriorityOpening, ratelimit.PriorityRoutine, ratelimit.PrioritySafety,
		} {
			d, err := g.Check(spotQuery(p, 1))
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if d.Admitted {
				t.Errorf("%s was admitted during a pause, want refused", p)
			}
			if d.Reason != ratelimit.ReasonPaused {
				t.Errorf("%s refusal reason = %s, want ReasonPaused", p, d.Reason)
			}
			if !d.NotBefore.Equal(start.Add(30 * time.Second)) {
				t.Errorf("%s NotBefore = %v, want the pause end", p, d.NotBefore)
			}
		}
	})

	t.Run("the pause expires", func(t *testing.T) {
		t.Parallel()
		g, tc := newGovernor(t)
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}
		if err := tc.Advance(30 * time.Second); err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1)); !d.Admitted {
			t.Error("still refused at the pause end, want admitted")
		}
	})

	t.Run("a ban pauses the venue for its own, longer schedule", func(t *testing.T) {
		t.Parallel()
		g, tc := newGovernor(t)
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}
		if err := tc.Advance(30 * time.Second); err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1)); d.Admitted {
			t.Error("admitted 30s into a two-minute ban, want refused")
		}
	})

	t.Run("a ban holds every family off", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}
		if d, _ := g.Check(perpOrder(ratelimit.PrioritySafety)); d.Admitted {
			t.Error("the other family was admitted during a venue ban, want refused")
		}
	})

	t.Run("an unset kind is refused", func(t *testing.T) {
		t.Parallel()
		g, _ := newGovernor(t)
		err := g.Penalize(ratelimit.Penalty{})
		if !errors.Is(err, ratelimit.ErrPenaltyInvalid) {
			t.Fatalf("Penalize = %v, want ErrPenaltyInvalid", err)
		}
		if d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1)); !d.Admitted {
			t.Error("a refused Penalize paused the venue anyway")
		}
	})
}

// A venue hint and the configured pause are combined by taking the longer. A
// hint that undershoots puts us straight back into the ban we are serving; one
// that overshoots is the venue telling us something we did not know.
func TestPenaltyTakesTheLongerOfHintAndSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		retryAfter time.Duration
		want       time.Duration
	}{
		{"no hint uses the configured pause", 0, 30 * time.Second},
		{"a shorter hint does not shorten the pause", 5 * time.Second, 30 * time.Second},
		{"a longer hint wins", 90 * time.Second, 90 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, _ := newGovernor(t)
			err := g.Penalize(ratelimit.Penalty{
				Kind: ratelimit.PenaltyThrottle, RetryAfter: tc.retryAfter,
			})
			if err != nil {
				t.Fatalf("Penalize: %v", err)
			}
			d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1))
			if want := start.Add(tc.want); !d.NotBefore.Equal(want) {
				t.Errorf("NotBefore = %v, want %v", d.NotBefore, want)
			}
		})
	}
}

// The same ratchet the header reading gets, for the same reason: a later,
// milder penalty must not release a pause an earlier, harsher one imposed.
func TestPenaltyNeverShortensAnActivePause(t *testing.T) {
	t.Parallel()

	g, tc := newGovernor(t)
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if err := tc.Advance(time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}

	d, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 1))
	if want := start.Add(2 * time.Minute); !d.NotBefore.Equal(want) {
		t.Errorf("NotBefore = %v, want the ban's %v — a throttle must not shorten a ban", d.NotBefore, want)
	}

	// And the pause is still reported as a ban, because §7.6 alerts on that
	// and a throttle arriving behind it must not downgrade the alert.
	if kind := g.Snapshot().PauseKind; kind != ratelimit.PenaltyBan {
		t.Errorf("PauseKind = %s, want PenaltyBan", kind)
	}
}

// PauseKind describes the pause running NOW. A ban that expired last week must
// not keep outranking the throttle in force today — §7.6 alerts on a ban, and an
// alert raised for a pause that is not one is an alert nobody will trust twice.
func TestPauseKindDescribesTheCurrentPauseNotAPastOne(t *testing.T) {
	t.Parallel()

	g, tc := newGovernor(t)
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if err := tc.Advance(2 * time.Minute); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if s := g.Snapshot(); s.Paused {
		t.Fatal("still paused after the ban elapsed")
	}

	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if kind := g.Snapshot().PauseKind; kind != ratelimit.PenaltyThrottle {
		t.Errorf("PauseKind = %s, want PenaltyThrottle — the ban had already expired", kind)
	}
}

// The mirror of the ratchet: a ban arriving during a throttle upgrades the kind.
func TestABanDuringAThrottleUpgradesThePauseKind(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if kind := g.Snapshot().PauseKind; kind != ratelimit.PenaltyBan {
		t.Errorf("PauseKind = %s, want PenaltyBan", kind)
	}
}

// A request costing more than a budget's whole capacity never becomes issuable,
// and "never" has to dominate every other refusal's release instant. The fixture
// therefore holds BOTH kinds of refusal at once: without the second limit, a
// comparison that mishandled the zero time would have nothing to lose to.
func TestANeverIssuableRequestDominatesEveryOtherRefusal(t *testing.T) {
	t.Parallel()

	g, _ := newExhaustedLaddered(t)

	// orders_10s is merely full and frees at a real instant; weight_1m — listed
	// SECOND, so it is compared against a real instant already in hand — cannot
	// admit 1000 at any consumption.
	d, err := g.Acquire(ratelimit.Request{
		Family: ids.ProductLinearPerp, Priority: ratelimit.PrioritySafety,
		Cost: ratelimit.Cost{Weight: 1000, Orders: 1},
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if d.Admitted {
		t.Fatal("admitted a request larger than the budget, want refused")
	}
	if !d.NotBefore.IsZero() {
		t.Errorf("NotBefore = %v, want the zero time: no amount of waiting frees 1000 of a 100 budget",
			d.NotBefore)
	}
	if d.Limit != "weight_1m" {
		t.Errorf("refusal names %q, want weight_1m", d.Limit)
	}
	if got := d.RetryAfter(); got != 0 {
		t.Errorf("RetryAfter = %v, want 0 — there is no wait that helps", got)
	}
	if msg := d.Err().Error(); !strings.Contains(msg, "any consumption") {
		t.Errorf("Err() = %q, want it to say waiting will not help", msg)
	}
}

// TestGovernorRefusesACostNoBudgetCounts is §7.6's headline sentence — "a
// governor MUST refuse a cost it cannot count" — driven through the only state
// Profile.Validate leaves it reachable from: a written Uncounted declaration.
// The declaration claims no spot cost charges an order slot; the request in
// hand falsifies the claim, and the refusal stands BECAUSE of the waiver, not
// despite it — a waiver is a claim about traffic, never a suppression.
func TestGovernorRefusesACostNoBudgetCounts(t *testing.T) {
	t.Parallel()

	p := binanceShaped()
	p.Limits = p.Limits[:4] // drop spot's orders budget
	p.Uncounted = []ratelimit.Uncounted{{
		Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders,
		Reason: "claim under test: no spot cost charges an order slot",
	}}
	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, p)
	if err != nil {
		t.Fatalf("New: a profile with a written waiver must construct: %v", err)
	}

	spotOrder := ratelimit.Request{
		Family: ids.ProductSpot, Priority: ratelimit.PrioritySafety,
		Cost: ratelimit.Cost{Weight: 1, Orders: 1},
	}
	// Check and Acquire agree: this is a profile-versus-adapter disagreement,
	// not a budget state, so it is an error rather than a refused Decision and
	// no amount of waiting changes it.
	if _, cerr := g.Check(spotOrder); !errors.Is(cerr, ratelimit.ErrCostUncounted) {
		t.Fatalf("Check = %v, want ErrCostUncounted", cerr)
	}
	_, aerr := g.Acquire(spotOrder)
	if !errors.Is(aerr, ratelimit.ErrCostUncounted) {
		t.Fatalf("Acquire = %v, want ErrCostUncounted", aerr)
	}
	// §7.6: the failure of forgetting is a refusal that NAMES the dimension.
	if msg := aerr.Error(); !strings.Contains(msg, "spot") || !strings.Contains(msg, "DimensionOrders") {
		t.Errorf("Acquire = %q, want the message to name the family and the dimension", msg)
	}

	// The refusal charged nothing, the waiver's scope is exactly one
	// (family, dimension), and traffic that does not charge the waived
	// dimension is untouched — three claims, one snapshot and two admissions.
	mustAcquire(t, g, perpOrder(ratelimit.PrioritySafety))
	mustAcquire(t, g, spotQuery(ratelimit.PrioritySafety, 10))
	for _, l := range g.Snapshot().Limits {
		if l.Family == ids.ProductSpot && l.Dimension == ratelimit.DimensionWeight && l.Used != 10 {
			t.Errorf("spot weight used = %d, want 10 — the refused order must have charged nothing", l.Used)
		}
	}
}

// TestRequestOnlyChargesTheEnvelopeAlone drives the carrier for "this call is
// metered per request alone" (Bybit's read surfaces) through the governor: the
// venue envelope is charged by construction, and no product budget moves.
func TestRequestOnlyChargesTheEnvelopeAlone(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	d, err := g.Acquire(ratelimit.Request{
		Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine,
		Cost: ratelimit.RequestOnly(),
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !d.Admitted {
		t.Fatalf("refused (%s, limit %q), want admitted", d.Reason, d.Limit)
	}
	for _, l := range g.Snapshot().Limits {
		switch {
		case l.Scope == ratelimit.ScopeVenue:
			if l.Used != 1 {
				t.Errorf("envelope used = %d, want 1", l.Used)
			}
		case l.Used != 0:
			t.Errorf("limit %q/%s used = %d, want 0 — a request-only cost charges no product budget",
				l.Name, l.Family, l.Used)
		}
	}
}

func TestGovernorRefusesEndpointProfileDisagreements(t *testing.T) {
	t.Parallel()

	g, _ := newEndpointGovernor(t)
	tests := []struct {
		name    string
		request ratelimit.Request
		want    error
		words   []string
	}{
		{
			name:    "a named endpoint with no budget",
			request: endpointRead(ids.ProductSpot, "unknown_read", ratelimit.PriorityRoutine),
			want:    ratelimit.ErrEndpointUnbudgeted, words: []string{"bybit", "unknown_read"},
		},
		{
			name:    "an unnamed request against an endpoint profile",
			request: ratelimit.Request{Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine, Cost: ratelimit.RequestOnly()},
			want:    ratelimit.ErrEndpointUnnamed, words: []string{"bybit", "spot"},
		},
		{
			name:    "a request-only cost under an orders endpoint",
			request: endpointRead(ids.ProductSpot, "order_create", ratelimit.PriorityRoutine),
			want:    ratelimit.ErrCostEndpointUncounted, words: []string{"order_create", "DimensionOrders"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := g.Acquire(tc.request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Acquire() = %v, want %v", err, tc.want)
			}
			for _, word := range tc.words {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("Acquire() = %q, want it to name %q", err, word)
				}
			}
		})
	}
}

func TestEndpointlessRequestRemainsLegalWithoutEndpointBudgets(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	mustAcquire(t, g, ratelimit.Request{
		Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine, Cost: ratelimit.RequestOnly(),
	})
}

func TestEndpointWindowsDoNotCrossCharge(t *testing.T) {
	t.Parallel()

	g, _ := newEndpointGovernor(t)
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))

	if got := usedOf(t, g, "", "read_b"); got != 0 {
		t.Fatalf("read_b used = %d, want 0 before its own call", got)
	}
	if d, err := g.Check(endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety)); err != nil || d.Admitted {
		t.Fatalf("read_a Check = (%+v, %v), want a ceiling refusal", d, err)
	}
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_b", ratelimit.PrioritySafety))
	if got := usedOf(t, g, "", "read_b"); got != 1 {
		t.Errorf("read_b used = %d, want exactly its own one request", got)
	}
	if got := usedOf(t, g, "", "ip_5s"); got != 3 {
		t.Errorf("envelope used = %d, want the three admitted requests", got)
	}
}

func TestEndpointWindowIsSharedAcrossFamilies(t *testing.T) {
	t.Parallel()

	g, _ := newEndpointGovernor(t)
	// A family-keyed endpoint window can make one package leg structurally
	// routable while its sibling is not. One venue-level window is conservative
	// under either venue counting model and makes both legs consult the same
	// capacity instead.
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
	mustAcquire(t, g, endpointRead(ids.ProductLinearPerp, "read_a", ratelimit.PrioritySafety))
	d, err := g.Check(endpointRead(ids.ProductLinearPerp, "read_a", ratelimit.PrioritySafety))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Admitted || d.Limit != "read_a" {
		t.Errorf("Check = admitted %v by %q, want shared read_a refusal", d.Admitted, d.Limit)
	}
}

func TestMatchedEndpointCountsItsOwnDimension(t *testing.T) {
	t.Parallel()

	g, _ := newEndpointGovernor(t)
	one := ratelimit.Request{
		Family: ids.ProductSpot, Endpoint: "order_create", Priority: ratelimit.PrioritySafety,
		Cost: ratelimit.Cost{Orders: 1},
	}
	mustAcquire(t, g, one)
	if got := usedOf(t, g, "", "order_create"); got != 1 {
		t.Errorf("order_create used = %d, want 1", got)
	}

	g, _ = newEndpointGovernor(t)
	batch := one
	batch.Cost = ratelimit.Cost{Orders: 5}
	mustAcquire(t, g, batch)
	if got := usedOf(t, g, "", "order_create"); got != 5 {
		t.Errorf("order_create used = %d, want the five orders carried by the batch", got)
	}

	g, _ = newEndpointGovernor(t)
	wrong := one
	wrong.Endpoint = "read_b"
	if _, err := g.Acquire(wrong); !errors.Is(err, ratelimit.ErrCostUncounted) {
		t.Fatalf("Acquire against requests-dimension endpoint = %v, want ErrCostUncounted", err)
	}
}

func TestEndpointObservationRatchetsTheMatchedWindow(t *testing.T) {
	t.Parallel()

	g, _ := newEndpointGovernor(t)
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_b", ratelimit.PrioritySafety))
	if err := g.Observe("", "read_b", 3); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if got := usedOf(t, g, "", "read_b"); got != 3 {
		t.Errorf("read_b used = %d, want observed ratchet to 3", got)
	}
	if err := g.Observe("", "read_b", 1); err != nil {
		t.Fatalf("Observe lower reading: %v", err)
	}
	if got := usedOf(t, g, "", "read_b"); got != 3 {
		t.Errorf("read_b used = %d after lower reading, want upward-only 3", got)
	}
}

func TestEndpointWindowEnforcesPriorityAndNamesRetry(t *testing.T) {
	t.Parallel()

	g, tc := newEndpointGovernor(t)
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PriorityOpening))
	if d, err := g.Check(endpointRead(ids.ProductSpot, "read_a", ratelimit.PriorityOpening)); err != nil || d.Admitted {
		t.Fatalf("opening Check = (%+v, %v), want refusal at the opening ceiling", d, err)
	}
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PriorityRoutine))
	d, err := g.Check(endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
	if err != nil {
		t.Fatalf("safety Check: %v", err)
	}
	if d.Admitted || d.Limit != "read_a" || !d.NotBefore.Equal(start.Add(time.Second)) {
		t.Fatalf("safety Check = %+v, want read_a refused until %v", d, start.Add(time.Second))
	}
	if err := tc.Advance(time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
}

func TestGovernorNamesTheLatestOfEndpointAndEnvelopeRefusals(t *testing.T) {
	t.Parallel()

	// With only the envelope-later ordering, worst-selection and first-selection
	// are indistinguishable: the envelope is both first-evaluated and latest.
	t.Run("envelope releases later", func(t *testing.T) {
		p := endpointShaped()
		p.Limits[0].Interval = 2 * time.Second
		p.Limits[0].OpeningCeiling, p.Limits[0].RoutineCeiling, p.Limits[0].Capacity = 1, 1, 1
		p.Limits[1].OpeningCeiling, p.Limits[1].RoutineCeiling, p.Limits[1].Capacity = 1, 1, 1
		tc, err := clock.NewTestClock(start, nil)
		if err != nil {
			t.Fatalf("NewTestClock: %v", err)
		}
		g, err := ratelimit.New(tc, p)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
		d, err := g.Check(endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if d.Limit != "ip_5s" || !d.NotBefore.Equal(start.Add(2*time.Second)) {
			t.Errorf("Check = limit %q until %v, want latest release ip_5s at %v",
				d.Limit, d.NotBefore, start.Add(2*time.Second))
		}
	})

	t.Run("endpoint releases later", func(t *testing.T) {
		p := endpointShaped()
		p.Limits[0].Interval = time.Second
		p.Limits[0].OpeningCeiling, p.Limits[0].RoutineCeiling, p.Limits[0].Capacity = 1, 1, 1
		p.Limits[1].Interval = 2 * time.Second
		p.Limits[1].OpeningCeiling, p.Limits[1].RoutineCeiling, p.Limits[1].Capacity = 1, 1, 1
		tc, err := clock.NewTestClock(start, nil)
		if err != nil {
			t.Fatalf("NewTestClock: %v", err)
		}
		g, err := ratelimit.New(tc, p)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		mustAcquire(t, g, endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
		d, err := g.Check(endpointRead(ids.ProductSpot, "read_a", ratelimit.PrioritySafety))
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if d.Limit != "read_a" || !d.NotBefore.Equal(start.Add(2*time.Second)) {
			t.Errorf("Check = limit %q until %v, want latest release read_a at %v",
				d.Limit, d.NotBefore, start.Add(2*time.Second))
		}
	})
}

func TestSnapshot(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)
	mustAcquire(t, g, spotQuery(ratelimit.PriorityOpening, 10))
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}

	s := g.Snapshot()
	if s.Venue != ids.Venue("binance") {
		t.Errorf("Venue = %q, want binance", s.Venue)
	}
	if !s.Paused {
		t.Error("Paused = false, want true")
	}
	// §7.6 requires an IP ban to raise an immediate operator alert. This
	// package does not deliver it; this counter is the fact it is raised on.
	if s.Bans != 1 || s.Throttles != 1 {
		t.Errorf("Bans=%d Throttles=%d, want 1 and 1", s.Bans, s.Throttles)
	}
	if len(s.Limits) != 5 {
		t.Fatalf("len(Limits) = %d, want 5", len(s.Limits))
	}

	found := false
	for _, l := range s.Limits {
		if l.Name == "weight_1m" && l.Family == ids.ProductSpot {
			found = true
			if l.Used != 10 || l.Capacity != 100 || l.Reserve != 10 {
				t.Errorf("spot weight_1m: used=%d capacity=%d reserve=%d, want 10, 100, 10",
					l.Used, l.Capacity, l.Reserve)
			}
		}
	}
	if !found {
		t.Error("the snapshot does not report the spot weight limit")
	}
}

// A refusal converts to the taxonomy §5.1 routes on, and it asserts the one
// thing this package can honestly assert about the side effect: a request the
// governor refused was never sent.
func TestDecisionErr(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	admitted, err := g.Acquire(spotQuery(ratelimit.PriorityOpening, 10))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if got := admitted.Err(); got != nil {
		t.Errorf("Err() on an admitted decision = %v, want nil", got)
	}

	if err := g.Penalize(ratelimit.Penalty{
		Kind: ratelimit.PenaltyBan, RetryAfter: 3 * time.Minute,
	}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}
	refused, _ := g.Check(spotQuery(ratelimit.PriorityOpening, 10))

	rerr := refused.Err()
	if rerr == nil {
		t.Fatal("Err() on a refused decision = nil, want an error")
	}
	if got := apperror.ClassOf(rerr); got != apperror.ClassRateLimit {
		t.Errorf("ClassOf = %s, want ClassRateLimit", got)
	}
	if apperror.MayHaveApplied(rerr) {
		t.Error("MayHaveApplied = true, want false: a refused request was never sent")
	}
	var ae *apperror.Error
	if !errors.As(rerr, &ae) {
		t.Fatalf("Err() = %T, want *apperror.Error", rerr)
	}
	if got := ae.RetryAfter(); got != 3*time.Minute {
		t.Errorf("RetryAfter = %v, want 3m — the wait the decision already computed", got)
	}
	if !strings.Contains(rerr.Error(), "binance") {
		t.Errorf("Err() = %q, want the message to name the venue", rerr)
	}
}

// Execution runs per-venue worker goroutines (§3.2), so the governor is shared.
// The claim under -race is not just "no data race" but the one that matters: no
// two concurrent callers are admitted past capacity between them.
func TestConcurrentAcquireNeverOverspends(t *testing.T) {
	t.Parallel()

	g, _ := newGovernor(t)

	const workers = 8
	const each = 40
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				d, err := g.Acquire(spotQuery(ratelimit.PrioritySafety, 10))
				if err != nil {
					t.Errorf("Acquire: %v", err)
					return
				}
				if d.Admitted {
					mu.Lock()
					admitted++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// Capacity 100 at 10 weight a call, and the clock never advanced.
	if admitted != 10 {
		t.Errorf("%d of %d requests admitted, want exactly 10", admitted, workers*each)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 100 {
		t.Errorf("used = %d, want 100", got)
	}
}

// usedOf reads one limit's consumption out of a snapshot.
func usedOf(t *testing.T, g *ratelimit.Governor, family ids.ProductCategory, name string) uint64 {
	t.Helper()
	for _, l := range g.Snapshot().Limits {
		if l.Name == name && l.Family == family {
			return l.Used
		}
	}
	t.Fatalf("no limit %q under family %q in the snapshot", name, family)
	return 0
}
