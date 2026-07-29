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

// monoPast waits for Mono to pass a reading, and it is BOUNDED because the
// failure it is watching for is a Mono that stands still — which, polled in an
// unbounded loop, is a test that hangs rather than a test that fails. `clock`
// found that on a non-blocking send and `boot` found it on a channel receive;
// this is the same discipline one package later.
//
// It polls rather than sleeping a fixed span because the claim is "advances",
// not "advances by at least X" — the monotonic clock's granularity is the host's
// business and pinning a number here would make the test a hardware assertion.
func monoPast(t *testing.T, w *Wall, from time.Duration) time.Duration {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := w.Mono(); got > from {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("Mono() never advanced past %v within 2s. A Mono derived from "+
				"the clamped Now stands still exactly here — see Mono's doc.", from)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestWall_MonoAdvancesAcrossABackstep is the assertion Mono's contract needs
// and its signature cannot carry: elapsed time keeps being measurable while the
// wall clock is going backwards.
//
// The fixture has to make the clamp ENGAGE, and that is the whole design of it.
// Against a forward-only clock, `time.Since(monoStart)` and the forbidden
// `w.Now().Sub(monoStart)` agree on every reading, so a test that never steps
// the clock back is a test both spellings pass.
//
// TWO clamped reads, not one, per the fixture-collapse checklist: with a single
// held read, "the clamp holds Now" and "Now happened to repeat once" are the
// same observation, and the run of identical instants that Mono exists to
// disambiguate is not present in the fixture at all.
func TestWall_MonoAdvancesAcrossABackstep(t *testing.T) {
	at := time.Date(2026, time.July, 29, 14, 0, 0, 0, time.UTC)

	// A plausible NTP correction: a reading, a 2.5 s step back, then time
	// resuming from the corrected value but still short of the high-water mark.
	w := newWallAt(t,
		at,
		at.Add(-2500*time.Millisecond), // the step
		at.Add(-2400*time.Millisecond), // corrected clock, still behind
	)

	first := w.Now()
	startMono := w.Mono()

	// The premise: for the next two reads the wall clock stands still, so
	// RecvTime cannot separate the records stamped in this window.
	for i, got := range []time.Time{w.Now(), w.Now()} {
		if !got.Equal(first) {
			t.Fatalf("clamped read %d: Now() = %v; want the held %v — the fixture is "+
				"not exercising the clamp, so it cannot discriminate Mono", i+1, got, first)
		}
	}

	// The clamp says a step happened...
	if count, total := w.Backsteps(); count != 1 || total != 2500*time.Millisecond {
		t.Errorf("Backsteps() = %d, %v; want 1, 2.5s", count, total)
	}

	// ...and Mono is what still measures duration while it is happening.
	if got := monoPast(t, w, startMono); got <= startMono {
		t.Errorf("Mono() = %v; want > %v", got, startMono)
	}
}

// TestWall_MonoMovesWhileTheWallClockIsFrozen is the limit case of the one
// above, and it is a separate test because it kills a different wrong
// implementation: a Mono that reads the system clock at all, whether or not it
// goes through the clamp. Pinned readings freeze Now, Backsteps AND any
// system-derived arithmetic; only a genuinely independent source still moves.
//
// This is also the realistic shape for capture's Alive ticks — a stalled host
// clock with the process still running is exactly the case where an archive must
// not silently claim a frozen timeline.
func TestWall_MonoMovesWhileTheWallClockIsFrozen(t *testing.T) {
	at := time.Date(2026, time.July, 29, 14, 0, 0, 0, time.UTC)
	w := newWallAt(t, at) // one reading, repeated forever

	before := w.Mono()
	if got := w.Now(); !got.Equal(at) {
		t.Fatalf("Now() = %v; want the pinned %v", got, at)
	}
	if count, _ := w.Backsteps(); count != 0 {
		t.Fatalf("Backsteps() count = %d; want 0 — a pinned clock never steps back", count)
	}

	after := monoPast(t, w, before)

	// And Now is still pinned afterwards, so the advance above cannot be
	// explained by anything the wall clock did.
	if got := w.Now(); !got.Equal(at) {
		t.Errorf("Now() = %v after Mono advanced to %v; want the pinned %v", got, after, at)
	}
}

// TestWall_MonoNeverRetreats is the ordinary property, asserted because Mono is
// stamped into every capture record and a reader at step 5 will sort on it.
func TestWall_MonoNeverRetreats(t *testing.T) {
	w, err := NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	prev := w.Mono()
	if prev < 0 {
		t.Fatalf("Mono() = %v immediately after NewWall; want >= 0", prev)
	}
	for i := range 200 {
		got := w.Mono()
		if got < prev {
			t.Fatalf("read %d: Mono() went backwards: %v then %v", i, prev, got)
		}
		prev = got
	}
}
