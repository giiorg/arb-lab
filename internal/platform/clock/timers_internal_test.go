package clock

import (
	"testing"
	"time"
)

// clampMonotonic is tested here rather than through Wall because Wall reads
// time.Now() directly and cannot be handed a fake. That is the whole reason the
// clamp was extracted: an NTP step backwards is not something a test can arrange,
// so the alternative was a guarantee in a doc comment with nothing checking it.
//
// Everything else in this package is covered through the exported API and the
// conformance suite. This is the one behaviour that genuinely cannot be, so it is
// the one internal test.
func TestClampMonotonic(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		now, high time.Time
		want      time.Time
	}{
		{
			// The normal case: time moved forward, so report it.
			name: "forward", now: at.Add(time.Second), high: at, want: at.Add(time.Second),
		},
		{
			// The NTP step. The high-water mark holds, so the clock stands still
			// rather than reversing — a timer fires late instead of a settlement
			// being evaluated twice.
			name: "backwards is clamped", now: at.Add(-time.Hour), high: at, want: at,
		},
		{
			// One nanosecond back is still back. The boundary matters: a
			// non-strict comparison here would let the smallest step through,
			// and the smallest step is the one NTP actually makes.
			name: "one nanosecond backwards", now: at.Add(-time.Nanosecond), high: at, want: at,
		},
		{
			name: "equal", now: at, high: at, want: at,
		},
		{
			// The first call, before any high-water mark exists. The zero time
			// must not win, or Wall would report year 1 forever.
			name: "no high-water mark yet", now: at, high: time.Time{}, want: at,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampMonotonic(tc.now, tc.high); !got.Equal(tc.want) {
				t.Errorf("clampMonotonic(%v, %v) = %v; want %v", tc.now, tc.high, got, tc.want)
			}
		})
	}
}

// Repeated clamping must be stable: feeding the result back in cannot move it.
// Wall relies on this, because it stores the clamped value as the next call's
// high-water mark.
func TestClampMonotonic_isIdempotent(t *testing.T) {
	at := time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)
	stepped := at.Add(-time.Hour)

	first := clampMonotonic(stepped, at)
	second := clampMonotonic(stepped, first)
	if !second.Equal(first) {
		t.Errorf("clamping twice moved the result: %v then %v", first, second)
	}
}
