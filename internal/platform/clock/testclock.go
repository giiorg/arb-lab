package clock

import (
	"fmt"
	"sync"
	"time"
)

// TestClock is the deterministic Clock the backtester drives (§3.5). It ships in
// production code rather than in a _test.go file because the backtester is
// production: `cmd/arblab-backtest` wires it, and the parity claim is that the
// same strategy, risk, sizing, OMS and monitor code runs over it.
//
// Nothing here reads the wall clock. Time moves only when a caller says so, which
// is what makes a replay reproduce a run rather than approximate it.
type TestClock struct {
	mu    sync.Mutex
	now   time.Time
	queue timerQueue
	cals  calibrationTable
}

var (
	_ Clock       = (*TestClock)(nil)
	_ TimerSource = (*TestClock)(nil)
)

// NewTestClock starts a driven clock at `start`.
//
// The zero time is rejected. It is a real instant (year 1), so accepting it would
// make every funding boundary read as long past — a default that answers the
// question this package exists to ask carefully.
func NewTestClock(start time.Time, cals []Calibration) (*TestClock, error) {
	if start.IsZero() {
		return nil, fmt.Errorf("start: %w", ErrTimeUnset)
	}
	table, err := newCalibrationTable(cals)
	if err != nil {
		return nil, err
	}
	return &TestClock{now: start, queue: newTimerQueue(), cals: table}, nil
}

// The Clock port. Identical in shape to Wall's, and that is the point.

func (c *TestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *TestClock) ServerNow(v Venue) ServerTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cals.serverNow(c.now, v)
}

func (c *TestClock) SetTimer(id TimerID, at time.Time, prio uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue.set(Fired{ID: id, At: at, Prio: prio})
}

func (c *TestClock) CancelTimer(id TimerID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue.cancel(id)
}

func (c *TestClock) NextDue() (Fired, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queue.popDue(c.now)
}

func (c *TestClock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queue.pending()
}

// The driving side. Only the backtester's loop holds a *TestClock; everything
// else holds a Clock and cannot move time at all.
//
// It is mutex-guarded even though a backtest is single-threaded, for one reason:
// the race detector runs over the same tests, and an implementation that is only
// safe when nobody looks is one that will eventually be used from a goroutine by
// someone who read the interface and not the comment.

// Advance moves time forward by d. Timers that become due are NOT delivered here —
// the caller drains them with NextDue, because §3.5 gives the dispatcher the total
// order over events and timers together, and a clock that delivered on its own
// would be arbitrating an order that is not its to arbitrate.
func (c *TestClock) Advance(d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d < 0 {
		return fmt.Errorf("advance by %v: %w", d, ErrTimeReversed)
	}
	c.now = c.now.Add(d)
	return nil
}

// SetNow moves time to a specific instant, which is what a replay does: it places
// the clock at each persisted envelope's time rather than accumulating deltas.
// Moving backwards is an error rather than a clamp — in a replay it means the
// dispatch log is out of order, and that is a finding, not something to absorb.
func (c *TestClock) SetNow(at time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if at.IsZero() {
		return fmt.Errorf("target: %w", ErrTimeUnset)
	}
	if at.Before(c.now) {
		return fmt.Errorf("from %v to %v: %w", c.now, at, ErrTimeReversed)
	}
	c.now = at
	return nil
}

// AdvanceToNextTimer moves time to the earliest scheduled timer and reports the
// instant it moved to. It reports false when no timers are armed.
//
// This is the backtester's main-loop primitive, and the reason it belongs here
// rather than in the caller: jumping straight to the next scheduled instant is
// what makes a replay of a month of 8-hour funding cycles finish in seconds
// instead of iterating a nominal tick. A caller that guessed a step size would
// either skip past timers or waste most of its iterations.
//
// If the earliest timer is already due, time does not move and the instant
// returned is the current one — so a caller looping "advance, then drain" makes
// progress rather than spinning.
func (c *TestClock) AdvanceToNextTimer() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next, found := c.queue.earliestScheduled()
	if !found {
		return c.now, false
	}
	if next.At.After(c.now) {
		c.now = next.At
	}
	return c.now, true
}
