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
	if len(s.Limits) != 4 {
		t.Fatalf("len(Limits) = %d, want 4", len(s.Limits))
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
