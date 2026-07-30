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

	// lastSys is the previous SYSTEM reading, and it is a separate field from
	// last on purpose — see Backsteps, where the difference between them is the
	// whole measurement.
	lastSys time.Time

	// backsteps and backstepTotal are what the clamp has absorbed. See
	// Backsteps.
	backsteps     uint64
	backstepTotal time.Duration

	// monoStart is Mono's baseline, and it is read from time.Now() DIRECTLY in
	// NewWall rather than through systemNow. That is not an oversight and it is
	// not laziness about the seam — it is the whole point, and see Mono.
	//
	// It is immutable after construction, which is why Mono takes no lock.
	monoStart time.Time

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
	return &Wall{
		queue:     newTimerQueue(),
		cals:      table,
		systemNow: time.Now,
		monoStart: time.Now(),
	}, nil
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
	sys := w.systemNow()
	// No first-call special case, for clampMonotonic's reason one function
	// along: the zero lastSys is year 1, so no reading a clock can produce is
	// before it. An explicit IsZero guard was written here first and a mutation
	// removing it survived — correctly, because nothing could reach it. A guard
	// no input can trigger reads as protection while protecting nothing.
	if sys.Before(w.lastSys) {
		w.backsteps++
		w.backstepTotal += w.lastSys.Sub(sys)
	}
	w.lastSys = sys
	w.last = clampMonotonic(sys, w.last)
	return w.last
}

// Backsteps reports what the clamp in Now has absorbed: how many times the
// system clock moved backwards, and by how much in total. It is a pure
// accessor; nothing here changes Now's contract.
//
// [SPEC EDIT LANDED — v0.9.13] §3.5's Clock block stated neither the clamp's
// existence nor its observability; it now states both, and that an archival
// consumer must record an engaged clamp as data. Capture's design record §32i
// is where this was argued.
//
// # Why a clock needs an observability seam at all
//
// Now's doc gives the clamp's reasoning honestly — "a backwards step is absorbed
// rather than reported, because Now has no error to return" — and that is right
// for the engine, where a timer firing late beats a settlement evaluated twice.
// It is wrong for anything writing an ARCHIVE, and the composition is what makes
// it quiet: NTP steps the host's clock backwards, this clamp makes a run of
// records carry IDENTICAL instants, §3.4's latest-value cache accepts an equal
// RecvTime by deliberate design, and the replay then proceeds over a squashed
// time axis with every freshness and cross-leg-skew number wrong and nothing
// failing. A capture archive is unrecoverable, so that cannot be repaired after
// the fact.
//
// The gap being closed is not capture's, it is this package's: until now NO
// caller could learn that the clamp engaged, and the engine has the same blind
// spot for the same reason.
//
// Two alternatives were rejected. A second monotonic reference held by the
// caller under //notimenow:allow works and leaves platform/ untouched, but it
// puts a second time authority in a tool whose job is to detect a defect in the
// first one. Making Now itself report the step changes every call site in the
// codebase to fix a problem one caller has.
//
// # What a "backstep" counts, and why it is not "how often the clamp engaged"
//
// Those are different numbers, and the difference is the reason lastSys exists
// as a field separate from last. After a single 2.5 s correction the clamp keeps
// engaging on every read until real time catches up, so counting engagements
// would report one NTP event as four or five backsteps and would sum several
// seconds of absorbed time that never elapsed.
//
// A backstep is therefore a reading BELOW THE PREVIOUS READING — the system
// clock actually moving backwards — and total is the sum of those jumps. Every
// backstep necessarily engages the clamp (last is the maximum of all readings,
// so a reading below the previous one is below the maximum too), which is what
// makes this the stricter and more informative of the two counts.
//
// Both are monotonic and never reset: a caller that wants a delta subtracts two
// observations. Resetting on read would make two readers silently steal each
// other's events, and the realistic second reader is a metrics scrape.
func (w *Wall) Backsteps() (count uint64, total time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.backsteps, w.backstepTotal
}

// Mono reports elapsed time since this Wall was constructed, measured on the
// host's MONOTONIC clock — a reading that keeps advancing across a wall-clock
// step, including the step Now absorbs and Backsteps counts.
//
// It is the third of the three readings capture stamps into every record
// (§32e), and the one whose requirement a signature cannot express. Backsteps
// says a step HAPPENED; Mono is what still measures duration WHILE it is
// happening. During an NTP correction Now stands still by design, so a run of
// records carries identical instants — and Mono is the only field in that run
// that still distinguishes them.
//
// # The implementation this must not have
//
// Not `w.Now().Sub(start)`. That satisfies the same signature, passes a test
// that only ever runs the clock forwards, and DEFEATS THE FIELD: Now is clamped,
// so a derived Mono stands still during precisely the window Mono exists to
// measure, and a record's Mono becomes redundant with its RecvTime exactly when
// RecvTime is untrustworthy. Capture's §34c states the contract for this reason.
//
// The trap is that the two spellings are one token apart and Go hides the
// difference: a time.Time from time.Now() carries a monotonic reading that Sub
// silently prefers, so `time.Since(monoStart)` is immune to wall-clock steps
// while `w.Now().Sub(monoStart)` — same package, same field, same arithmetic —
// is not. Nothing in the type system separates them. What separates them is that
// monoStart is filled from time.Now() rather than through systemNow, so no fake
// clock can reach it and the independence is structural rather than promised.
// TestWall_MonoAdvancesAcrossABackstep is the assertion, and it fails against
// the derived spelling.
//
// # Scope
//
// The baseline is per-Wall, so two Walls in one process yield two unrelated
// timelines. Records are only ever compared against records from the same
// process, and the composition root builds exactly one Wall — but the field is
// meaningless across a restart either way, which is what the Open record's
// provenance and CaptureSeq are for.
func (w *Wall) Mono() time.Duration { return time.Since(w.monoStart) }

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
