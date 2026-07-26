package clock

import (
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
