package clock

import (
	"sync"
	"time"
)

// Wall is the live Clock: real time, real venue calibrations.
//
// It is the only place in the engine that may call time.Now(), and it may because
// it is not domain or application code — the notimenow gate expresses that
// structurally rather than through an annotation, so this file needs no
// exemption of any kind.
//
// Concurrency: a mutex, because live callers are spread across goroutines — a WS
// reader arming an order timeout while the dispatcher drains due timers. TestClock
// locks too, for the reason given there.
type Wall struct {
	mu    sync.Mutex
	queue timerQueue
	cals  calibrationTable

	// last is Now's high-water mark. See Now.
	last time.Time

	// systemNow is the system clock, indirected for one specific reason: it is
	// the only way to test that the monotonic clamp is actually WIRED UP.
	//
	// The mutation that removed the store to w.last survived the whole suite
	// otherwise — the clamp's logic was covered by clampMonotonic's own tests,
	// but its wiring here was not, and without the store w.last stays the zero
	// time and the clamp silently never engages. That is a test gap, not a bad
	// mutation, so it is closed rather than argued with.
	//
	// Unexported and set only by an internal test. Nothing exported can change
	// it, and production always reads the real clock.
	systemNow func() time.Time
}

// Compile-time proof that Wall satisfies both seams, per §3.6 rule 2's shape.
var (
	_ Clock       = (*Wall)(nil)
	_ TimerSource = (*Wall)(nil)
)

// NewWall builds a live clock with the given per-venue calibrations. A venue
// absent from the list yields an unset ServerTime rather than a confident zero.
func NewWall(cals []Calibration) (*Wall, error) {
	table, err := newCalibrationTable(cals)
	if err != nil {
		return nil, err
	}
	return &Wall{queue: newTimerQueue(), cals: table, systemNow: time.Now}, nil
}

// Now returns wall time, clamped so it never goes backwards. The clamp itself is
// clampMonotonic in timers.go, which is where its reasoning and its tests are;
// that the clamp is wired up here — that the high-water mark is actually stored —
// is tested through systemNow.
//
// A backwards step is absorbed rather than reported, because Now has no error to
// return (§3.5). Absorbing it means a clock that briefly stands still, which is
// the conservative failure: a timer fires late rather than a settlement being
// evaluated twice.
func (w *Wall) Now() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nowLocked()
}

func (w *Wall) nowLocked() time.Time {
	w.last = clampMonotonic(w.systemNow(), w.last)
	return w.last
}

func (w *Wall) ServerNow(v Venue) ServerTime {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cals.serverNow(w.nowLocked(), v)
}

func (w *Wall) SetTimer(id TimerID, at time.Time, prio uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue.set(Fired{ID: id, At: at, Prio: prio})
}

func (w *Wall) CancelTimer(id TimerID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue.cancel(id)
}

func (w *Wall) NextDue() (Fired, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.queue.popDue(w.nowLocked())
}

func (w *Wall) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.queue.pending()
}
