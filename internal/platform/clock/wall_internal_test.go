package clock

import (
	"sync"
	"testing"
	"time"
)

// These two tests exist because a mutation survived.
//
// clampMonotonic had its own tests and the conformance suite passed against Wall,
// yet removing the store to w.last — so the high-water mark was computed and
// thrown away — broke nothing. The clamp's logic was covered; its WIRING was not.
// A surviving mutant is either a test gap or a wrong mutant, and this one was a
// gap: without the store, w.last stays the zero time forever and the clamp never
// engages, which is indistinguishable from not having written it.
//
// Closing it needs a fake time source, which is why Wall has an unexported
// systemNow. Nothing exported can reach it, so the public surface is unchanged
// and production always reads the real clock.

func newWallAt(t *testing.T, readings ...time.Time) *Wall {
	t.Helper()
	w, err := NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	i := 0
	w.systemNow = func() time.Time {
		if i < len(readings) {
			r := readings[i]
			i++
			return r
		}
		return readings[len(readings)-1]
	}
	return w
}

// The wiring: an NTP step backwards must not make Now retreat.
func TestWall_NowNeverRetreats(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

	// A plausible NTP correction: forward, forward, then a step back, then
	// resuming from the corrected value.
	w := newWallAt(t,
		at,
		at.Add(500*time.Millisecond),
		at.Add(-2*time.Second), // the step
		at.Add(-1500*time.Millisecond),
		at.Add(2*time.Second), // past the old high-water mark at last
	)

	want := []time.Time{
		at,
		at.Add(500 * time.Millisecond),
		at.Add(500 * time.Millisecond), // held
		at.Add(500 * time.Millisecond), // still held
		at.Add(2 * time.Second),        // released
	}

	var prev time.Time
	for i, expect := range want {
		got := w.Now()
		if !got.Equal(expect) {
			t.Errorf("call %d: Now() = %v; want %v", i+1, got, expect)
		}
		if i > 0 && got.Before(prev) {
			t.Errorf("call %d: Now() went backwards: %v then %v", i+1, prev, got)
		}
		prev = got
	}
}

// TestWall_BackstepsCountsStepsNotClampedReads is the fixture-collapse case for
// the accessor, and the collapse it avoids is the one that decides the
// measurement's meaning.
//
// After a single correction the clamp keeps engaging on every read until real
// time catches up. A fixture with one step and one clamped read makes "counts
// backward steps" and "counts clamped reads" the same answer, so the recovery
// window below is not padding — it is the only part of the sequence where the
// two diverge. The two steps also have DIFFERENT magnitudes, so summing them and
// taking the largest are distinguishable, and neither equals the count.
func TestWall_BackstepsCountsStepsNotClampedReads(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

	w := newWallAt(t,
		at,                             // 1: first reading, nothing to compare against
		at.Add(500*time.Millisecond),   // 2: forward
		at.Add(-2*time.Second),         // 3: STEP BACK by 2.5s
		at.Add(-1500*time.Millisecond), // 4: recovering — clamped, but forward of 3
		at.Add(-time.Second),           // 5: recovering — clamped, but forward of 4
		at.Add(2*time.Second),          // 6: past the high-water mark; released
		at.Add(time.Second),            // 7: STEP BACK by 1s
		at.Add(3*time.Second),          // 8: forward again
	)
	for range 8 {
		w.Now()
	}

	count, total := w.Backsteps()
	if count != 2 {
		t.Errorf("Backsteps() count = %d, want 2 — reads 4, 5 and 7 are all "+
			"clamped, but only 3 and 7 are the clock moving backwards", count)
	}
	if want := 3500 * time.Millisecond; total != want {
		t.Errorf("Backsteps() total = %v, want %v (2.5s + 1s). Summing every "+
			"clamped read gives 7s; taking the largest gives 2.5s", total, want)
	}

	// Now's own contract is unchanged by the bookkeeping: the last reading is
	// past every high-water mark, so it must come through unclamped.
	if got := w.Now(); !got.Equal(at.Add(3 * time.Second)) {
		t.Errorf("Now() = %v, want %v", got, at.Add(3*time.Second))
	}
}

// The positive control. Without it, an implementation that counted every read —
// or simply returned 2 — would satisfy the assertions above, and a monitor that
// has never reported a clean clock has not been shown able to.
func TestWall_BackstepsAreZeroOnAForwardClock(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

	// Fresh, then one reading, then a forward-only run: three states, because
	// "never stepped back" and "has not been called" are different situations
	// that a count starting at 1 would confuse.
	fresh, err := NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	if count, total := fresh.Backsteps(); count != 0 || total != 0 {
		t.Errorf("a Wall that has never been read reports %d, %v; want 0, 0", count, total)
	}

	one := newWallAt(t, at)
	one.Now()
	if count, total := one.Backsteps(); count != 0 || total != 0 {
		t.Errorf("after a single read: %d, %v; want 0, 0 — the first reading has "+
			"no predecessor to be behind", count, total)
	}

	forward := newWallAt(t, at, at.Add(time.Second), at.Add(2*time.Second), at.Add(time.Hour))
	for range 4 {
		forward.Now()
	}
	if count, total := forward.Backsteps(); count != 0 || total != 0 {
		t.Errorf("a forward-only clock reports %d, %v; want 0, 0", count, total)
	}
}

// ServerNow reaches the system clock through nowLocked, so it must feed the same
// counter. It reads the clock for the venue-offset decision — §2.4's funding
// boundary — which is exactly the call a squashed time axis would corrupt, so a
// backstep observed only through Now would leave the more consequential path
// unmeasured.
func TestWall_BackstepsSeeServerNow(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)
	venue := Venue("binance")

	w, err := NewWall([]Calibration{{Venue: venue, Offset: time.Second, Uncertainty: 0}})
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	readings := []time.Time{at, at.Add(-3 * time.Second)}
	i := 0
	w.systemNow = func() time.Time {
		r := readings[min(i, len(readings)-1)]
		i++
		return r
	}

	w.ServerNow(venue)
	w.ServerNow(venue)
	count, total := w.Backsteps()
	if count != 1 || total != 3*time.Second {
		t.Errorf("Backsteps() after two ServerNow calls = %d, %v; want 1, 3s", count, total)
	}
}

// The accessor takes the same mutex as everything else on Wall, and this is the
// test that says so — mutate.sh runs without -race and therefore cannot report a
// lock claim at all, so a lock claim needs its own race-enabled test.
//
// It waits on a WaitGroup and reads no channel, so there is nothing here a
// mutation can leave a test blocked on — the failure mode this package's own
// history warns about is a test that HANGS rather than fails, and the harness
// reports that as TIMEOUT!!, which names no defect.
func TestWall_BackstepsIsSafeUnderConcurrentReads(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)
	w := newWallAt(t, at, at.Add(-time.Second), at.Add(time.Second))

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() { defer wg.Done(); w.Now() }()
		go func() { defer wg.Done(); w.Backsteps() }()
	}
	wg.Wait()
}

// ServerNow reads the clock through the same path, so it must inherit the clamp
// rather than reaching around it — otherwise the funding-boundary decision, which
// is the one that matters, would see the reversed instant that Now hides.
func TestWall_ServerNowInheritsTheClamp(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)
	venue := Venue("binance")

	w, err := NewWall([]Calibration{{Venue: venue, Offset: time.Second, Uncertainty: 0}})
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	readings := []time.Time{at, at.Add(-time.Hour)}
	i := 0
	w.systemNow = func() time.Time {
		r := readings[min(i, len(readings)-1)]
		i++
		return r
	}

	first := w.ServerNow(venue)
	if err := first.Validate(); err != nil {
		t.Fatalf("ServerNow: %v", err)
	}
	second := w.ServerNow(venue)
	if second.Instant().Before(first.Instant()) {
		t.Errorf("ServerNow retreated across a backwards system step: %v then %v",
			first.Instant(), second.Instant())
	}
	if want := at.Add(time.Second); !second.Instant().Equal(want) {
		t.Errorf("ServerNow().Instant() = %v; want the clamped %v", second.Instant(), want)
	}
}
