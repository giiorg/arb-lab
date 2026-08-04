package ratelimit

import (
	"math"
	"testing"
	"time"
)

// base is a fixed instant. Nothing here reads an ambient clock, so every
// assertion below is about arithmetic rather than about scheduling.
var base = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

func testWindow() *window {
	return newWindow(Limit{
		Name:           "weight_1m",
		Scope:          ScopeVenue,
		Dimension:      DimensionWeight,
		Interval:       time.Minute,
		OpeningCeiling: 70,
		RoutineCeiling: 90,
		Capacity:       100,
	})
}

// The eviction boundary is asserted AT the interval, not near it. A charge and a
// probe at the same instant would make `>= interval` and `> interval` the same
// predicate, which is the fixture dimension this repo has now collapsed four
// times.
func TestWindowEvictsExactlyAtTheInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		age  time.Duration
		want uint64
	}{
		{"a charge just made is held", 0, 10},
		{"a charge one nanosecond short of the interval is held", time.Minute - 1, 10},
		{"a charge exactly at the interval is released", time.Minute, 0},
		{"a charge past the interval is released", time.Minute + 1, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := testWindow()
			w.charge(base, 10)

			if got := w.usedAt(base.Add(tc.age)); got != tc.want {
				t.Errorf("usedAt(base+%v) = %d, want %d", tc.age, got, tc.want)
			}
		})
	}
}

// Three charges of different sizes at different instants, so that "release the
// oldest" and "release everything" are distinguishable, and so that a partial
// release is visible as a number rather than as a boolean.
func TestWindowReleasesChargesIndividually(t *testing.T) {
	t.Parallel()

	w := testWindow()
	w.charge(base, 10)
	w.charge(base.Add(10*time.Second), 20)
	w.charge(base.Add(20*time.Second), 30)

	if got := w.usedAt(base.Add(20 * time.Second)); got != 60 {
		t.Fatalf("usedAt = %d, want 60", got)
	}
	if got := w.usedAt(base.Add(time.Minute)); got != 50 {
		t.Errorf("usedAt at base+1m = %d, want 50 — only the first charge has aged out", got)
	}
	if got := w.usedAt(base.Add(70 * time.Second)); got != 30 {
		t.Errorf("usedAt at base+70s = %d, want 30", got)
	}
	if got := w.usedAt(base.Add(80 * time.Second)); got != 0 {
		t.Errorf("usedAt at base+80s = %d, want 0", got)
	}
}

func TestWindowAdmits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		used    uint64
		units   uint64
		ceiling uint64
		want    bool
	}{
		{"room to spare", 10, 5, 100, true},
		{"exactly filling the ceiling is admitted", 90, 10, 100, true},
		{"one unit past the ceiling is refused", 90, 11, 100, false},
		{"an empty window admits a full-ceiling charge", 0, 100, 100, true},
		{"a charge larger than the ceiling is refused outright", 0, 101, 100, false},

		// A zero charge is admitted unconditionally: a request that costs this
		// dimension nothing cannot push it over. Without this, a plain query
		// would be refused by an exhausted ORDER budget it does not touch.
		{"a zero charge is admitted at the ceiling", 100, 0, 100, true},
		{"a zero charge is admitted past the ceiling", 150, 0, 100, true},

		// Past the ceiling is reachable: a header reading can ratchet used
		// above a ceiling we never spent up to ourselves.
		{"a charge is refused when already past the ceiling", 150, 1, 100, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := testWindow()
			if tc.used > 0 {
				w.charge(base, tc.used)
			}
			if got := w.admits(base, tc.units, tc.ceiling); got != tc.want {
				t.Errorf("admits(units=%d, ceiling=%d) with used=%d = %v, want %v",
					tc.units, tc.ceiling, tc.used, got, tc.want)
			}
		})
	}
}

// freeAt must accumulate across charges rather than answering with the first
// one, so the fixture uses three charges whose sizes make the two behaviours
// give different instants.
func TestWindowFreeAt(t *testing.T) {
	t.Parallel()

	newLoaded := func() *window {
		w := testWindow()
		w.charge(base, 10)
		w.charge(base.Add(10*time.Second), 20)
		w.charge(base.Add(20*time.Second), 30)
		return w
	}

	tests := []struct {
		name string
		need uint64
		want time.Time
	}{
		// used is 60 of a ceiling of 100 at base+20s.
		{"a charge that already fits is available now", 30, base.Add(20 * time.Second)},

		// The boundary: a need of exactly the remaining room. Every other case
		// here sits strictly inside or strictly outside it, where `<=` and `<`
		// are the same predicate — and the difference is not cosmetic, because
		// the strict form sends a caller away to wait for a budget it already
		// had.
		{"a charge that exactly fills the remaining room is available now",
			40, base.Add(20 * time.Second)},
		{"freeing the first charge is enough", 50, base.Add(time.Minute)},
		{"the first charge is not enough, so it waits for the second",
			60, base.Add(70 * time.Second)},
		{"a need that empties the window waits for the last charge",
			100, base.Add(80 * time.Second)},

		// Nothing frees a need larger than the ceiling, so there is no instant
		// to name. The zero time says "not by waiting" rather than lying with a
		// plausible one.
		{"a need above the ceiling never becomes available", 101, time.Time{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newLoaded()
			got := w.freeAt(base.Add(20*time.Second), tc.need, 100)
			if !got.Equal(tc.want) {
				t.Errorf("freeAt(need=%d) = %v, want %v", tc.need, got, tc.want)
			}
		})
	}
}

// An empty window has no charge to name an expiry from, so the already-fits
// branch is the only thing that can answer — and it has to answer `now` rather
// than the zero time, which a caller reads as "waiting will not help".
func TestWindowFreeAtOnAnEmptyWindowIsNow(t *testing.T) {
	t.Parallel()

	w := testWindow()
	if got := w.freeAt(base, 100, 100); !got.Equal(base) {
		t.Errorf("freeAt on an empty window = %v, want %v", got, base)
	}
}

func TestWindowFreeAtAfterObservationAboveCapacityIsFuture(t *testing.T) {
	t.Parallel()

	w := testWindow()
	w.observe(base, 150)
	want := base.Add(time.Minute)
	if got := w.freeAt(base, 1, 100); !got.Equal(want) {
		t.Errorf("freeAt after observation above capacity = %v, want %v", got, want)
	}
}

func TestWindowFreeAtAfterSafetySpendsPastRoutineCeilingIsFuture(t *testing.T) {
	t.Parallel()

	w := testWindow()
	// Safety may spend to Capacity while Routine stops lower. Asking when a
	// Routine unit fits therefore exercises used above the requested ceiling,
	// not merely above the limit's absolute capacity.
	w.charge(base, 90)
	want := base.Add(time.Minute)
	if got := w.freeAt(base, 1, 70); !got.Equal(want) {
		t.Errorf("freeAt above the routine ceiling = %v, want %v", got, want)
	}
}

func TestEndpointStyleWindowFreeAtNamesTheOldestRequiredExpiry(t *testing.T) {
	t.Parallel()

	w := newWindow(Limit{
		Name: "order_realtime", Scope: ScopeEndpoint,
		Dimension: DimensionRequests, Interval: time.Second,
		OpeningCeiling: 3, RoutineCeiling: 4, Capacity: 5,
	})
	w.charge(base, 2)
	w.charge(base.Add(100*time.Millisecond), 1)
	w.charge(base.Add(200*time.Millisecond), 2)

	// Three units must expire before a three-unit retry fits. The oldest charge
	// alone is insufficient, so the answer is the second charge's expiry rather
	// than now, the first expiry, or the newest charge's later expiry.
	want := base.Add(1100 * time.Millisecond)
	if got := w.freeAt(base.Add(200*time.Millisecond), 3, 5); !got.Equal(want) {
		t.Errorf("freeAt endpoint-style refusal = %v, want %v", got, want)
	}
}

// The ratchet: a reading above the local count is adopted, one below it is not.
func TestWindowObserveRatchetsUpwardOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reported uint64
		want     uint64
	}{
		{"a higher reading is adopted", 80, 80},
		{"an equal reading changes nothing", 60, 60},

		// A reading below the local count is what a stale header looks like:
		// it describes the server before the requests still in flight behind it
		// were counted. Adopting it would discard them.
		{"a lower reading is ignored", 10, 60},
		{"a zero reading is ignored", 0, 60},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := testWindow()
			w.charge(base, 60)

			w.observe(base.Add(time.Second), tc.reported)
			if got := w.usedAt(base.Add(time.Second)); got != tc.want {
				t.Errorf("after observe(%d), usedAt = %d, want %d", tc.reported, got, tc.want)
			}
		})
	}
}

// The correction is applied as a charge at the observation instant, so it ages
// out on the window's own terms — and later than the usage it corrects for,
// which is the conservative direction.
func TestWindowObserveCorrectionAgesOutFromTheObservationInstant(t *testing.T) {
	t.Parallel()

	w := testWindow()
	w.charge(base, 60)
	w.observe(base.Add(30*time.Second), 80)

	// The original charge has aged out; the 20-unit correction has not.
	if got := w.usedAt(base.Add(time.Minute)); got != 20 {
		t.Errorf("usedAt at base+1m = %d, want 20 — the correction is younger than the charge", got)
	}
	if got := w.usedAt(base.Add(90 * time.Second)); got != 0 {
		t.Errorf("usedAt at base+90s = %d, want 0 — the correction has aged out too", got)
	}
}

// An observation is evaluated against the window as it stands at that instant,
// not against a stale total. Without eviction first, a reading of 30 arriving
// after the local charges had expired would be discarded as "below 60".
func TestWindowObserveEvictsBeforeComparing(t *testing.T) {
	t.Parallel()

	w := testWindow()
	w.charge(base, 60)

	at := base.Add(2 * time.Minute)
	w.observe(at, 30)
	if got := w.usedAt(at); got != 30 {
		t.Errorf("usedAt = %d, want 30 — the old charge had expired, so 30 is an increase", got)
	}
}

// The saturating add exists for one input class — a venue reading this package
// does not get to bound — so the clamp is asserted directly. Reached only
// through observe in practice, which is exactly why it needs its own test:
// nothing in the behavioural suite drives a uint64 anywhere near the top.
func TestAddSaturatingClampsRatherThanWrapping(t *testing.T) {
	t.Parallel()

	const max = uint64(math.MaxUint64)
	tests := []struct {
		name string
		a, b uint64
		want uint64
	}{
		{"an ordinary sum", 10, 20, 30},
		{"a sum that exactly reaches the top", max - 5, 5, max},
		{"a sum that would wrap clamps", max - 5, 6, max},
		{"a sum that would wrap far past clamps", max, max, max},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := addSaturating(tc.a, tc.b); got != tc.want {
				t.Errorf("addSaturating(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// Wrapping would turn an absurd reading into a SMALL used value, which admits
// traffic. The direction is the whole reason the helper exists, so it is stated
// as a behaviour of the window and not only of the arithmetic.
func TestWindowObserveOfAnAbsurdReadingRefusesRatherThanAdmits(t *testing.T) {
	t.Parallel()

	w := testWindow()
	w.charge(base, 10)
	w.observe(base, math.MaxUint64)

	if w.admits(base, 1, 100) {
		t.Error("admitted after a reading of MaxUint64; a wrapped total would look like an empty budget")
	}
}

// The charge log is compacted rather than appended to forever. It is bounded by
// the budget itself, and the assertion is on the retained length because a leak
// here is invisible in every behavioural test.
func TestWindowCompactsItsChargeLog(t *testing.T) {
	t.Parallel()

	w := testWindow()
	at := base
	for i := 0; i < 500; i++ {
		at = at.Add(time.Second)
		w.charge(at, 1)
	}

	// A one-minute window at one charge per second retains 60.
	if got := len(w.charges); got != 60 {
		t.Errorf("len(charges) = %d after 500 charges, want 60", got)
	}
	if got := w.usedAt(at); got != 60 {
		t.Errorf("usedAt = %d, want 60", got)
	}
}
