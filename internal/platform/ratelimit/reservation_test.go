package ratelimit_test

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

func reservationProfile(capacity uint64) ratelimit.Profile {
	return ratelimit.Profile{
		Venue:    ids.Venue("binance"),
		Families: []ids.ProductCategory{ids.ProductSpot},
		Limits: []ratelimit.Limit{
			{Name: "weight_1m", Scope: ratelimit.ScopeFamily, Family: ids.ProductSpot,
				Dimension: ratelimit.DimensionWeight, Interval: time.Second,
				OpeningCeiling: capacity, RoutineCeiling: capacity, Capacity: capacity},
			{Name: "ip_5s", Scope: ratelimit.ScopeVenue,
				Dimension: ratelimit.DimensionRequests, Interval: time.Second,
				OpeningCeiling: capacity, RoutineCeiling: capacity, Capacity: capacity},
		},
		Uncounted: []ratelimit.Uncounted{{
			Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "orders are not used",
		}},
		ThrottlePause: 30 * time.Second,
		BanPause:      time.Minute,
	}
}

func newReservationGovernor(t *testing.T, capacity uint64) (*ratelimit.Governor, *clock.TestClock) {
	t.Helper()
	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, reservationProfile(capacity))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g, tc
}

func reservationRequest(weight uint64) ratelimit.Request {
	return ratelimit.Request{
		Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine,
		Cost: ratelimit.Cost{Weight: weight},
	}
}

func reservedOf(t *testing.T, g *ratelimit.Governor, family ids.ProductCategory, name string) uint64 {
	t.Helper()
	for _, l := range g.Snapshot().Limits {
		if l.Family == family && l.Name == name {
			return l.Reserved
		}
	}
	t.Fatalf("no limit %q under family %q", name, family)
	return 0
}

func TestReserveChecksTheWholeBatchAndLeavesRefusalUntouched(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, 5)
	before := g.Snapshot()

	h, d, err := g.Reserve([]ratelimit.Request{reservationRequest(3), reservationRequest(3)})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if h != nil || d.Admitted || d.Reason != ratelimit.ReasonCeiling {
		t.Fatalf("Reserve = (%v, %+v), want whole-batch ceiling refusal", h, d)
	}
	if got := g.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after refusal changed:\n got %+v\nwant %+v", got, before)
	}
}

func TestReserveIsAllOrNothingWhenALaterWindowRefuses(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, 5)
	// Only the venue window is occupied, and it is evaluated after the family
	// window. The first window therefore admits the batch before the second
	// refuses, separating atomic checking from charge-then-rollback.
	if err := g.Observe("", "ip_5s", 4); err != nil {
		t.Fatalf("Observe venue window: %v", err)
	}
	before := g.Snapshot()

	h, d, err := g.Reserve([]ratelimit.Request{reservationRequest(1), reservationRequest(1)})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if h != nil || d.Admitted {
		t.Fatalf("Reserve = (%v, %+v), want refusal", h, d)
	}
	if got := g.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("later-window refusal partially occupied: got %+v want %+v", got, before)
	}
}

func TestReserveRefusesEmptyMixedAndOverflowingBatches(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, math.MaxUint64)
	if _, _, err := g.Reserve(nil); !errors.Is(err, ratelimit.ErrReserveEmpty) {
		t.Fatalf("Reserve(nil) = %v, want ErrReserveEmpty", err)
	}
	mixed := []ratelimit.Request{reservationRequest(1), reservationRequest(1)}
	mixed[1].Priority = ratelimit.PrioritySafety
	if _, _, err := g.Reserve(mixed); !errors.Is(err, ratelimit.ErrReservePriorityMixed) {
		t.Fatalf("Reserve(mixed) = %v, want ErrReservePriorityMixed", err)
	}
	before := g.Snapshot()
	if _, _, err := g.Reserve([]ratelimit.Request{
		reservationRequest(math.MaxUint64), reservationRequest(1),
	}); !errors.Is(err, ratelimit.ErrReserveOverflow) {
		t.Fatalf("Reserve(overflow) = %v, want ErrReserveOverflow", err)
	}
	if got := g.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("overflow occupied capacity: got %+v want %+v", got, before)
	}
}

func TestReserveAndConsumeHonorPauses(t *testing.T) {
	t.Parallel()
	t.Run("pause at reserve", func(t *testing.T) {
		g, _ := newReservationGovernor(t, 10)
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}
		h, d, err := g.Reserve([]ratelimit.Request{reservationRequest(1)})
		if err != nil || h != nil || d.Reason != ratelimit.ReasonPaused {
			t.Fatalf("Reserve = (%v, %+v, %v), want paused", h, d, err)
		}
	})
	t.Run("pause at consume", func(t *testing.T) {
		g, _ := newReservationGovernor(t, 10)
		r := reservationRequest(2)
		h, d, err := g.Reserve([]ratelimit.Request{r})
		if err != nil || !d.Admitted {
			t.Fatalf("Reserve = (%v, %+v, %v)", h, d, err)
		}
		if err := g.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
			t.Fatalf("Penalize: %v", err)
		}
		d, err = h.Consume(r)
		if err != nil || d.Reason != ratelimit.ReasonPaused {
			t.Fatalf("Consume = (%+v, %v), want paused", d, err)
		}
		if got := reservedOf(t, g, ids.ProductSpot, "weight_1m"); got != 2 {
			t.Fatalf("reserved after pause = %d, want 2", got)
		}
	})
}

func TestHoldMatchesEntriesAndConsumesDuplicates(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, 20)
	a, b := reservationRequest(2), reservationRequest(3)
	h, _, err := g.Reserve([]ratelimit.Request{a, b})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if _, err := h.Consume(reservationRequest(4)); !errors.Is(err, ratelimit.ErrHoldEntryUnmatched) {
		t.Fatalf("Consume(mismatch) = %v, want ErrHoldEntryUnmatched", err)
	}
	if d, err := h.Consume(b); err != nil || !d.Admitted {
		t.Fatalf("Consume(second distinct entry) = (%+v, %v)", d, err)
	}
	if _, err := h.Consume(b); !errors.Is(err, ratelimit.ErrHoldEntryUnmatched) {
		t.Fatalf("double Consume = %v, want ErrHoldEntryUnmatched", err)
	}

	g2, _ := newReservationGovernor(t, 20)
	h2, _, err := g2.Reserve([]ratelimit.Request{a, a})
	if err != nil {
		t.Fatalf("Reserve duplicates: %v", err)
	}
	for i := 0; i < 2; i++ {
		if d, err := h2.Consume(a); err != nil || !d.Admitted {
			t.Fatalf("Consume duplicate %d = (%+v, %v)", i, d, err)
		}
	}
	if _, err := h2.Consume(a); !errors.Is(err, ratelimit.ErrHoldEntryUnmatched) {
		t.Fatalf("third duplicate Consume = %v, want ErrHoldEntryUnmatched", err)
	}
}

func TestCancelReleasesOnlyReservedEntriesAndIsIdempotent(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, 20)
	a, b := reservationRequest(2), reservationRequest(3)
	before := g.Snapshot()
	h, _, err := g.Reserve([]ratelimit.Request{a, b})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if _, err := h.Consume(a); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := h.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 2 {
		t.Fatalf("Used after partial consume/cancel = %d, want 2", got)
	}
	if got := reservedOf(t, g, ids.ProductSpot, "weight_1m"); got != 0 {
		t.Fatalf("Reserved after Cancel = %d, want 0", got)
	}
	after := g.Snapshot()
	if err := h.Cancel(); err != nil {
		t.Fatalf("second Cancel: %v", err)
	}
	if got := g.Snapshot(); !reflect.DeepEqual(got, after) {
		t.Fatalf("second Cancel changed snapshot: got %+v want %+v", got, after)
	}
	if reflect.DeepEqual(after, before) {
		t.Fatal("fixture collapsed: consumed charge is not visible")
	}

	g2, _ := newReservationGovernor(t, 20)
	h2, _, err := g2.Reserve([]ratelimit.Request{a})
	if err != nil {
		t.Fatalf("Reserve cancel-first: %v", err)
	}
	if err := h2.Cancel(); err != nil {
		t.Fatalf("Cancel first: %v", err)
	}
	if _, err := h2.Consume(a); !errors.Is(err, ratelimit.ErrHoldEntryUnmatched) {
		t.Fatalf("Consume after Cancel = %v, want ErrHoldEntryUnmatched", err)
	}
	for _, state := range g2.Snapshot().Limits {
		if state.Used != 0 || state.Reserved != 0 {
			t.Errorf("cancel-first limit %s = Used %d Reserved %d, want 0/0",
				state.Name, state.Used, state.Reserved)
		}
	}
}

func TestOccupancySurvivesWindowAndChargeStartsAtConsume(t *testing.T) {
	t.Parallel()
	g, tc := newReservationGovernor(t, 5)
	r := reservationRequest(4)
	h, _, err := g.Reserve([]ratelimit.Request{r})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := tc.Advance(time.Second + 1); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if d, err := g.Acquire(reservationRequest(2)); err != nil || d.Admitted {
		t.Fatalf("interleaved Acquire = (%+v, %v), want occupancy refusal", d, err)
	}
	if d, err := h.Consume(r); err != nil || !d.Admitted {
		t.Fatalf("Consume = (%+v, %v)", d, err)
	}
	if err := tc.Advance(time.Second - 1); err != nil {
		t.Fatalf("Advance short: %v", err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 4 {
		t.Fatalf("Used just before consume+interval = %d, want 4", got)
	}
	if err := tc.Advance(time.Nanosecond); err != nil {
		t.Fatalf("Advance boundary: %v", err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 0 {
		t.Fatalf("Used at consume+interval = %d, want 0", got)
	}
}

func TestObserveDoesNotFoldOccupancyIntoUsed(t *testing.T) {
	t.Parallel()
	g, _ := newReservationGovernor(t, 20)
	h, _, err := g.Reserve([]ratelimit.Request{reservationRequest(6)})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := g.Observe(ids.ProductSpot, "weight_1m", 10); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := h.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 10 {
		t.Fatalf("Used after observe/cancel = %d, want 10", got)
	}

	g2, _ := newReservationGovernor(t, 20)
	mustAcquire(t, g2, reservationRequest(5))
	h2, _, err := g2.Reserve([]ratelimit.Request{reservationRequest(6)})
	if err != nil {
		t.Fatalf("Reserve below-report: %v", err)
	}
	if err := g2.Observe(ids.ProductSpot, "weight_1m", 4); err != nil {
		t.Fatalf("Observe below: %v", err)
	}
	if err := h2.Cancel(); err != nil {
		t.Fatalf("Cancel below: %v", err)
	}
	if got := usedOf(t, g2, ids.ProductSpot, "weight_1m"); got != 5 {
		t.Fatalf("Used after below-report/cancel = %d, want 5", got)
	}

	g3, _ := newReservationGovernor(t, 20)
	first, second := reservationRequest(2), reservationRequest(3)
	h3, _, err := g3.Reserve([]ratelimit.Request{first, second})
	if err != nil {
		t.Fatalf("Reserve partial-consume: %v", err)
	}
	if d, err := h3.Consume(first); err != nil || !d.Admitted {
		t.Fatalf("Consume partial: (%+v, %v)", d, err)
	}
	if err := g3.Observe(ids.ProductSpot, "weight_1m", 10); err != nil {
		t.Fatalf("Observe after consume: %v", err)
	}
	if err := h3.Cancel(); err != nil {
		t.Fatalf("Cancel remainder: %v", err)
	}
	if got := usedOf(t, g3, ids.ProductSpot, "weight_1m"); got != 10 {
		t.Fatalf("Used after partial consume/fold/cancel = %d, want 10", got)
	}
	if got := reservedOf(t, g3, ids.ProductSpot, "weight_1m"); got != 0 {
		t.Fatalf("Reserved after partial consume/fold/cancel = %d, want 0", got)
	}
}

// TestReserveSharesEveryAdmissionValidationGateWithCheck is the parity table
// that protects admitRequest's factorization. Each malformed request must leave
// through the same sentinel whether considered singly or as a one-leg batch;
// otherwise reservation can approve a request the send path refuses.
func TestReserveSharesEveryAdmissionValidationGateWithCheck(t *testing.T) {
	t.Parallel()

	uncountedGovernor := func(t *testing.T) *ratelimit.Governor {
		t.Helper()
		p := binanceShaped()
		p.Limits = p.Limits[:4]
		p.Uncounted = []ratelimit.Uncounted{{
			Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders,
			Reason: "fixture claim contradicted by the request",
		}}
		tc, err := clock.NewTestClock(start, nil)
		if err != nil {
			t.Fatalf("NewTestClock: %v", err)
		}
		g, err := ratelimit.New(tc, p)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return g
	}

	tests := []struct {
		name string
		gov  func(*testing.T) *ratelimit.Governor
		req  ratelimit.Request
		want error
	}{
		{"family", func(t *testing.T) *ratelimit.Governor { g, _ := newGovernor(t); return g }, ratelimit.Request{
			Family: ids.ProductInversePerp, Priority: ratelimit.PriorityRoutine, Cost: ratelimit.Cost{Weight: 1},
		}, ratelimit.ErrFamilyUnknown},
		{"R1 endpoint unbudgeted", func(t *testing.T) *ratelimit.Governor { g, _ := newEndpointGovernor(t); return g },
			endpointRead(ids.ProductSpot, "unknown_read", ratelimit.PriorityRoutine), ratelimit.ErrEndpointUnbudgeted},
		{"R2 endpoint unnamed", func(t *testing.T) *ratelimit.Governor { g, _ := newEndpointGovernor(t); return g }, ratelimit.Request{
			Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine, Cost: ratelimit.RequestOnly(),
		}, ratelimit.ErrEndpointUnnamed},
		{"R3 endpoint cannot count", func(t *testing.T) *ratelimit.Governor { g, _ := newEndpointGovernor(t); return g },
			endpointRead(ids.ProductSpot, "order_create", ratelimit.PriorityRoutine), ratelimit.ErrCostEndpointUncounted},
		{"cannot count", uncountedGovernor, ratelimit.Request{
			Family: ids.ProductSpot, Priority: ratelimit.PrioritySafety, Cost: ratelimit.Cost{Weight: 1, Orders: 1},
		}, ratelimit.ErrCostUncounted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.gov(t)
			if _, err := g.Check(tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("Check = %v, want %v", err, tc.want)
			}
			before := g.Snapshot()
			if _, _, err := g.Reserve([]ratelimit.Request{tc.req}); !errors.Is(err, tc.want) {
				t.Fatalf("Reserve = %v, want %v", err, tc.want)
			}
			if after := g.Snapshot(); !reflect.DeepEqual(after, before) {
				t.Errorf("Reserve validation changed state: got %+v want %+v", after, before)
			}
		})
	}
}

func TestConsumeAndCancelRaceHasOnlySpecifiedOutcomes(t *testing.T) {
	g, _ := newReservationGovernor(t, 10)
	r := reservationRequest(2)
	h, _, err := g.Reserve([]ratelimit.Request{r})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	startRace := make(chan struct{})
	var wg sync.WaitGroup
	var consumeErr error
	wg.Add(2)
	go func() { defer wg.Done(); <-startRace; _, consumeErr = h.Consume(r) }()
	go func() { defer wg.Done(); <-startRace; _ = h.Cancel() }()
	close(startRace)
	wg.Wait()
	used := usedOf(t, g, ids.ProductSpot, "weight_1m")
	reserved := reservedOf(t, g, ids.ProductSpot, "weight_1m")
	if reserved != 0 {
		t.Fatalf("Reserved = %d, want 0", reserved)
	}
	if consumeErr == nil && used != 2 {
		t.Fatalf("successful Consume left Used = %d, want 2", used)
	}
	if errors.Is(consumeErr, ratelimit.ErrHoldEntryUnmatched) && used != 0 {
		t.Fatalf("canceled-before-consume left Used = %d, want 0", used)
	}
	if consumeErr != nil && !errors.Is(consumeErr, ratelimit.ErrHoldEntryUnmatched) {
		t.Fatalf("Consume error = %v", consumeErr)
	}
}

func TestConsumeCapacityBoundaryAndWrap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		capacity uint64
		fold     uint64
		want     bool
	}{
		{"exactly capacity admits", 10, 8, true},
		{"one past capacity refuses", 10, 9, false},
		{"max uint64 does not wrap", math.MaxUint64, math.MaxUint64, false},
		{"an upward fold that still fits admits", 10, 7, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newReservationGovernor(t, tc.capacity)
			r := reservationRequest(2)
			h, _, err := g.Reserve([]ratelimit.Request{r})
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			if err := g.Observe(ids.ProductSpot, "weight_1m", tc.fold); err != nil {
				t.Fatalf("Observe: %v", err)
			}
			before := usedOf(t, g, ids.ProductSpot, "weight_1m")
			d, err := h.Consume(r)
			if err != nil {
				t.Fatalf("Consume: %v", err)
			}
			if d.Admitted != tc.want {
				t.Fatalf("Consume admitted = %v, want %v (%+v)", d.Admitted, tc.want, d)
			}
			if !tc.want {
				if d.Reason != ratelimit.ReasonCeiling {
					t.Fatalf("Reason = %s, want ReasonCeiling", d.Reason)
				}
				if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != before {
					t.Fatalf("refused Consume changed Used = %d, want %d", got, before)
				}
				_ = h.Cancel()
			}
		})
	}
}

func TestConsumeAdmitsZeroChargePastSiblingCapacity(t *testing.T) {
	t.Parallel()
	p := reservationProfile(5)
	p.Limits = append(p.Limits, ratelimit.Limit{
		Name: "orders_1m", Scope: ratelimit.ScopeFamily, Family: ids.ProductSpot,
		Dimension: ratelimit.DimensionOrders, Interval: time.Second,
		OpeningCeiling: 5, RoutineCeiling: 5, Capacity: 5,
	})
	p.Uncounted = nil
	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := ratelimit.Request{
		Family: ids.ProductSpot, Priority: ratelimit.PriorityRoutine,
		Cost: ratelimit.Cost{Orders: 1},
	}
	h, _, err := g.Reserve([]ratelimit.Request{r})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := g.Observe(ids.ProductSpot, "weight_1m", 6); err != nil {
		t.Fatalf("Observe weight window: %v", err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 6 {
		t.Fatalf("Used after fold = %d, want 6 past capacity 5", got)
	}

	d, err := h.Consume(r)
	if err != nil || !d.Admitted {
		t.Fatalf("Consume = (%+v, %v), want admission", d, err)
	}
	if got := usedOf(t, g, ids.ProductSpot, "orders_1m"); got != 1 {
		t.Fatalf("Used on charged sibling = %d, want 1", got)
	}
}

func TestFreeAtIgnoresOccupancyWhileAdmissionCountsIt(t *testing.T) {
	t.Parallel()
	p := reservationProfile(10)
	p.Limits[0].OpeningCeiling = 8
	p.Limits[0].RoutineCeiling = 9
	p.Limits[1].OpeningCeiling = 8
	p.Limits[1].RoutineCeiling = 9
	tc, err := clock.NewTestClock(start, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(tc, p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	charge := reservationRequest(8)
	charge.Priority = ratelimit.PrioritySafety
	mustAcquire(t, g, charge)
	occupied := reservationRequest(1)
	occupied.Priority = ratelimit.PrioritySafety
	h, _, err := g.Reserve([]ratelimit.Request{occupied})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	probe := reservationRequest(1)
	probe.Priority = ratelimit.PriorityOpening
	d, err := g.Check(probe)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Admitted || !d.NotBefore.Equal(start.Add(time.Second)) {
		t.Fatalf("Check = %+v, want refusal with charges-only expiry", d)
	}
	_ = h.Cancel()
}

func TestAdmissionCountsReservedAtCeilingBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		reserved uint64
		want     bool
	}{
		{"one unit fits at ceiling minus one", 5, true},
		{"one unit is refused at the ceiling", 6, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newReservationGovernor(t, 10)
			mustAcquire(t, g, reservationRequest(4))
			h, _, err := g.Reserve([]ratelimit.Request{reservationRequest(tc.reserved)})
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			t.Cleanup(func() { _ = h.Cancel() })
			if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 4 {
				t.Fatalf("Used = %d, want 4", got)
			}
			if got := reservedOf(t, g, ids.ProductSpot, "weight_1m"); got != tc.reserved {
				t.Fatalf("Reserved = %d, want %d", got, tc.reserved)
			}

			d, err := g.Check(reservationRequest(1))
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if d.Admitted != tc.want {
				t.Fatalf("Check admitted = %v, want %v (%+v)", d.Admitted, tc.want, d)
			}
		})
	}

	t.Run("reservation above a lower-priority availability does not underflow", func(t *testing.T) {
		p := reservationProfile(10)
		p.Limits[0].OpeningCeiling = 8
		tc, err := clock.NewTestClock(start, nil)
		if err != nil {
			t.Fatalf("NewTestClock: %v", err)
		}
		g, err := ratelimit.New(tc, p)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		charge := reservationRequest(4)
		charge.Priority = ratelimit.PrioritySafety
		mustAcquire(t, g, charge)
		occupied := reservationRequest(6)
		occupied.Priority = ratelimit.PrioritySafety
		h, _, err := g.Reserve([]ratelimit.Request{occupied})
		if err != nil {
			t.Fatalf("Reserve: %v", err)
		}
		t.Cleanup(func() { _ = h.Cancel() })
		if got := usedOf(t, g, ids.ProductSpot, "weight_1m"); got != 4 {
			t.Fatalf("Used = %d, want 4 below opening ceiling 8", got)
		}
		if got := reservedOf(t, g, ids.ProductSpot, "weight_1m"); got != 6 {
			t.Fatalf("Reserved = %d, want 6 above availability 4", got)
		}

		probe := reservationRequest(1)
		probe.Priority = ratelimit.PriorityOpening
		d, err := g.Check(probe)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if d.Admitted || d.Reason != ratelimit.ReasonCeiling {
			t.Fatalf("Check = %+v, want ceiling refusal", d)
		}
	})
}
