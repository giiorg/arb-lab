package clock_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// The conformance suite covers what both implementations must agree on. This file
// covers what only TestClock has: the driving side. Nothing here is shared
// behaviour, so putting it in the conformance suite would mean an invariant Wall
// could not satisfy.

func newDriven(t *testing.T) *clock.TestClock {
	t.Helper()
	c, err := clock.NewTestClock(base, calibrations)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

// Advancing by a negative duration is rejected rather than clamped. Unlike Wall's
// Now — which absorbs an NTP step because it has no error to return — Advance
// does have one, and a backtest that moved time backwards would replay a window
// twice with nothing failing.
func TestTestClock_Advance_rejectsNegative(t *testing.T) {
	c := newDriven(t)
	if err := c.Advance(-time.Nanosecond); !errors.Is(err, clock.ErrTimeReversed) {
		t.Errorf("Advance(-1ns) = %v; want ErrTimeReversed", err)
	}
	if got := c.Now(); !got.Equal(base) {
		t.Errorf("a rejected Advance moved the clock to %v; want it unchanged at %v", got, base)
	}
	// Zero is not negative, and it is what a replay of two envelopes sharing an
	// instant does. It must be allowed.
	if err := c.Advance(0); err != nil {
		t.Errorf("Advance(0) = %v; want nil — two envelopes may share an instant", err)
	}
}

// SetNow is what a replay uses: it places the clock at each persisted envelope's
// time rather than accumulating deltas, so a rounding error cannot drift.
func TestTestClock_SetNow(t *testing.T) {
	c := newDriven(t)

	target := base.Add(3 * time.Hour)
	if err := c.SetNow(target); err != nil {
		t.Fatalf("SetNow: %v", err)
	}
	if got := c.Now(); !got.Equal(target) {
		t.Errorf("Now() = %v; want %v", got, target)
	}

	// Backwards is a finding, not something to absorb: in a replay it means the
	// persisted dispatch log is out of order, which is exactly the thing §3.5's
	// replay key exists to guarantee.
	if err := c.SetNow(base); !errors.Is(err, clock.ErrTimeReversed) {
		t.Errorf("SetNow backwards = %v; want ErrTimeReversed", err)
	}
	if got := c.Now(); !got.Equal(target) {
		t.Errorf("a rejected SetNow moved the clock to %v; want it unchanged at %v", got, target)
	}

	// The same instant twice is not backwards.
	if err := c.SetNow(target); err != nil {
		t.Errorf("SetNow to the current instant = %v; want nil", err)
	}

	// The zero time is a real instant in year 1, so accepting it would move the
	// clock two millennia backwards and read as valid.
	if err := c.SetNow(time.Time{}); !errors.Is(err, clock.ErrTimeUnset) {
		t.Errorf("SetNow(zero time) = %v; want ErrTimeUnset", err)
	}
}

// AdvanceToNextTimer is the backtester's main-loop primitive: it jumps straight to
// the next scheduled instant, which is what makes replaying a month of 8-hour
// funding cycles finish in seconds rather than iterating a nominal tick.
func TestTestClock_AdvanceToNextTimer(t *testing.T) {
	c := newDriven(t)

	// With nothing armed there is nowhere to go, and the clock must not move.
	if at, ok := c.AdvanceToNextTimer(); ok {
		t.Errorf("AdvanceToNextTimer with no timers = (%v, true); want false", at)
	}
	if got := c.Now(); !got.Equal(base) {
		t.Errorf("AdvanceToNextTimer moved an empty clock to %v; want %v", got, base)
	}

	// It goes to the EARLIEST timer, not the first one armed.
	far := base.Add(8 * time.Hour)
	near := base.Add(30 * time.Minute)
	c.SetTimer("far", far, 0)
	c.SetTimer("near", near, 0)

	at, ok := c.AdvanceToNextTimer()
	if !ok {
		t.Fatal("AdvanceToNextTimer with two timers armed reported none")
	}
	if !at.Equal(near) {
		t.Errorf("advanced to %v; want the earlier %v", at, near)
	}
	if got := drain(c); !idsEqual(got, []clock.TimerID{"near"}) {
		t.Errorf("drained %v; want [near] — the far timer is not due yet", got)
	}

	// And then to the next one, which is now the only one left.
	at, ok = c.AdvanceToNextTimer()
	if !ok {
		t.Fatal("AdvanceToNextTimer with one timer left reported none")
	}
	if !at.Equal(far) {
		t.Errorf("advanced to %v; want %v", at, far)
	}
	if got := drain(c); !idsEqual(got, []clock.TimerID{"far"}) {
		t.Errorf("drained %v; want [far]", got)
	}
}

// An already-due timer must not move the clock backwards, and must still report
// true — otherwise a caller looping "advance, then drain" would either rewind or
// spin without making progress.
func TestTestClock_AdvanceToNextTimer_alreadyDue(t *testing.T) {
	c := newDriven(t)
	c.SetTimer("stale", base.Add(-time.Hour), 0)

	at, ok := c.AdvanceToNextTimer()
	if !ok {
		t.Fatal("AdvanceToNextTimer with an overdue timer reported none")
	}
	if !at.Equal(base) {
		t.Errorf("advanced to %v; want the current %v — an overdue timer must not rewind the clock", at, base)
	}
	if got := drain(c); !idsEqual(got, []clock.TimerID{"stale"}) {
		t.Errorf("drained %v; want [stale]", got)
	}
}

// The tie-break applies to AdvanceToNextTimer's choice as well, because the
// instant it picks determines which timers become due together. It reuses the
// same comparator as the queue, and this pins that rather than trusting it.
func TestTestClock_AdvanceToNextTimer_usesTheFullOrder(t *testing.T) {
	c := newDriven(t)
	at := base.Add(time.Hour)
	c.SetTimer("b", at, 7)
	c.SetTimer("a", at, 2)

	got, ok := c.AdvanceToNextTimer()
	if !ok {
		t.Fatal("reported no timers")
	}
	if !got.Equal(at) {
		t.Errorf("advanced to %v; want %v", got, at)
	}
	// Both share the instant, so both are now due, in (prio, id) order.
	if drained := drain(c); !idsEqual(drained, []clock.TimerID{"a", "b"}) {
		t.Errorf("drained %v; want [a b]", drained)
	}
}

// Construction rejects what cannot be recovered from later.
func TestNewTestClock_rejects(t *testing.T) {
	if _, err := clock.NewTestClock(time.Time{}, nil); !errors.Is(err, clock.ErrTimeUnset) {
		t.Errorf("NewTestClock(zero time) = %v; want ErrTimeUnset", err)
	}
}

// Calibration validation is shared by both constructors, so it is asserted
// against both — a check that lived in only one would let the other accept a
// wiring mistake.
func TestConstructors_rejectBadCalibrations(t *testing.T) {
	cases := []struct {
		name string
		cals []clock.Calibration
		want error
	}{
		{
			// An unnamed venue is what a zero-valued config entry produces, and
			// it would sit in the table under the key "" where no real lookup
			// can reach it.
			name: "venue unset",
			cals: []clock.Calibration{{Offset: time.Second, Uncertainty: time.Second}},
			want: clock.ErrVenueUnset,
		},
		{
			// A negative band would put Earliest() after Latest() and invert
			// every boundary comparison, so it is refused at the door rather
			// than at each use.
			name: "negative uncertainty",
			cals: []clock.Calibration{{Venue: binance, Uncertainty: -time.Nanosecond}},
			want: clock.ErrUncertaintyNegative,
		},
		{
			// Two entries for one venue has a silent winner otherwise, decided
			// by list order — a wiring mistake that would show up as a clock
			// that is subtly wrong on one exchange.
			name: "duplicate venue",
			cals: []clock.Calibration{
				{Venue: binance, Offset: time.Second},
				{Venue: binance, Offset: -time.Second},
			},
			want: clock.ErrVenueDuplicate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := clock.NewTestClock(base, tc.cals); !errors.Is(err, tc.want) {
				t.Errorf("NewTestClock = %v; want %v", err, tc.want)
			}
			if _, err := clock.NewWall(tc.cals); !errors.Is(err, tc.want) {
				t.Errorf("NewWall = %v; want %v", err, tc.want)
			}
		})
	}

	// The positive control: the calibrations the whole suite uses must be
	// accepted by both. Without it, a constructor that rejected everything would
	// pass every case above.
	if _, err := clock.NewTestClock(base, calibrations); err != nil {
		t.Errorf("NewTestClock rejected the suite's own calibrations: %v", err)
	}
	if _, err := clock.NewWall(calibrations); err != nil {
		t.Errorf("NewWall rejected the suite's own calibrations: %v", err)
	}
	// And no calibrations at all is legal — every venue is then uncalibrated,
	// which ServerNow reports honestly rather than treating as an error here.
	if _, err := clock.NewWall(nil); err != nil {
		t.Errorf("NewWall(nil) = %v; want nil", err)
	}
}

// A funding boundary decided through the clock, end to end: the reason the band
// is carried at all. bybit's clock is 80ms behind ours with a 40ms band, so there
// is a window where the boundary is genuinely undecidable — and the point of the
// three-valued answer is that the window is visible instead of being resolved by
// whichever comparison the caller happened to write.
func TestServerNow_fundingBoundaryIsThreeValued(t *testing.T) {
	c := newDriven(t)
	// bybit's server time is Now-80ms ± 40ms, so it reads the boundary as passed
	// only once our own clock is 120ms past it, and as certainly-not until 40ms
	// past it.
	boundaryAt := base.Add(time.Hour)

	seen := map[clock.Certainty]bool{}
	for offset := 0 * time.Millisecond; offset <= 200*time.Millisecond; offset += 10 * time.Millisecond {
		if err := c.SetNow(boundaryAt.Add(offset - 50*time.Millisecond)); err != nil {
			t.Fatalf("SetNow: %v", err)
		}
		seen[c.ServerNow(bybit).Passed(boundaryAt)] = true
	}

	for _, want := range []clock.Certainty{clock.CertainlyNotPassed, clock.Uncertain, clock.CertainlyPassed} {
		if !seen[want] {
			t.Errorf("sweeping our clock across the boundary never produced %v; "+
				"the uncertainty band is not reaching the decision", want)
		}
	}
}
