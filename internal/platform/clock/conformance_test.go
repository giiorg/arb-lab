package clock_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// The conformance suite. Every invariant below is written once, as a predicate
// over a Clock, and run against BOTH implementations — because §3.5's parity
// claim is that live and backtest run the same code over interchangeable
// adapters, and an adapter pair that was only ever tested separately is a pair
// that has never been shown to be interchangeable.
//
// The predicate/generator split is §5c's, reused as that section predicted it
// would be: a predicate returning "here is how this implementation is wrong, or
// empty if it is not" can be run against a deliberately broken Clock, and a
// negative control does exactly that over a fixed sweep. An invariant that has
// never rejected anything is an invariant that might be checking nothing — and
// for a conformance suite the risk is sharper than usual, because the easy
// invariants (does it compile, does Now return something) are all vacuous.
//
// The one genuine asymmetry between the two implementations is who moves time.
// TestClock is driven; Wall is not. So each implementation supplies an `advance`
// control and a `grain` — the smallest interval it can distinguish reliably. For
// TestClock the grain is zero and every assertion is exact. For Wall it is a real
// margin, because a sleeping goroutine under -race is not a precise instrument.

// ---------------------------------------------------------------------------
// The harness
// ---------------------------------------------------------------------------

// subject is an implementation plus the controls the predicates need.
type subject struct {
	name    string
	clock   clock.Clock
	timers  clock.TimerSource
	advance func(t *testing.T, d time.Duration)
	grain   time.Duration
}

const (
	// venues used across the suite. binance is calibrated ahead of us, bybit
	// behind, so a sign error in the offset cannot pass both.
	binance = clock.Venue("binance")
	bybit   = clock.Venue("bybit")
	// uncalibrated is deliberately never registered.
	uncalibrated = clock.Venue("kraken")
)

var calibrations = []clock.Calibration{
	{Venue: binance, Offset: 120 * time.Millisecond, Uncertainty: 15 * time.Millisecond},
	{Venue: bybit, Offset: -80 * time.Millisecond, Uncertainty: 40 * time.Millisecond},
}

// wallGrain is the margin Wall's assertions allow. Generous on purpose: a
// too-tight margin turns a correctness suite into a flake generator, and a flaky
// gate gets disabled, which costs more than the precision was worth.
const wallGrain = 150 * time.Millisecond

func newSubjects(t *testing.T) []subject {
	t.Helper()

	tc, err := clock.NewTestClock(base, calibrations)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	wall, err := clock.NewWall(calibrations)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}

	return []subject{
		{
			name: "TestClock", clock: tc, timers: tc, grain: 0,
			advance: func(t *testing.T, d time.Duration) {
				t.Helper()
				if err := tc.Advance(d); err != nil {
					t.Fatalf("Advance(%v): %v", d, err)
				}
			},
		},
		{
			name: "Wall", clock: wall, timers: wall, grain: wallGrain,
			advance: func(t *testing.T, d time.Duration) {
				t.Helper()
				time.Sleep(d)
			},
		},
	}
}

var base = time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

// drainLimit bounds the loop below, and the bound is not defensive padding — the
// negative controls found it. A correct TimerSource removes what it hands over,
// so an unbounded loop terminates; the "a consumed timer stays due" control does
// not, and the first run of that control hung the suite instead of failing it.
//
// A predicate that can hang on a broken implementation is a bad predicate. A
// failing test names the defect; a hanging one reports nothing, blocks CI, and
// looks like an infrastructure problem. The bound is generous — no fixture arms
// more than four timers.
const drainLimit = 64

// drainOverflowed is returned as a trailing id when the limit is hit, so the
// comparison fails with something legible rather than by length alone.
const drainOverflowed clock.TimerID = "<drain limit exceeded: a consumed timer stayed due>"

// drain consumes every due timer in order and returns their ids.
func drain(ts clock.TimerSource) []clock.TimerID {
	var got []clock.TimerID
	for len(got) < drainLimit {
		f, ok := ts.NextDue()
		if !ok {
			return got
		}
		got = append(got, f.ID)
	}
	return append(got, drainOverflowed)
}

func idsEqual(got, want []clock.TimerID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// The invariants, each as a predicate
// ---------------------------------------------------------------------------

// checkNowMonotonic: Now never goes backwards. Every timer comparison in the
// package assumes it, and in a backtest a clock that stepped back would replay a
// window twice with no error anywhere.
func checkNowMonotonic(t *testing.T, s subject) string {
	first := s.clock.Now()
	s.advance(t, s.step())
	second := s.clock.Now()
	if second.Before(first) {
		return fmt.Sprintf("Now went backwards: %v then %v", first, second)
	}
	if !second.After(first) {
		return fmt.Sprintf("Now did not advance across an advance of %v: %v twice", s.step(), first)
	}
	return ""
}

// checkTimerFiresOnceWhenDue: a timer is due once it is reached, and consuming it
// removes it. A timer that stayed due would be re-consumed forever.
func checkTimerFiresOnceWhenDue(t *testing.T, s subject) string {
	at := s.clock.Now().Add(s.step())
	s.clock.SetTimer("once", at, 0)

	if f, ok := s.timers.NextDue(); ok {
		return fmt.Sprintf("a timer scheduled %v ahead was already due: %+v", s.step(), f)
	}

	s.advance(t, s.step()+s.grain)
	f, ok := s.timers.NextDue()
	if !ok {
		return "a timer whose time has passed was not due"
	}
	if f.ID != "once" {
		return fmt.Sprintf("NextDue returned id %q; want %q", f.ID, "once")
	}
	// At is the SCHEDULED instant, not the delivery instant — this is the
	// property replay depends on, so it is asserted rather than assumed.
	if !f.At.Equal(at) {
		return fmt.Sprintf("Fired.At = %v; want the scheduled %v", f.At, at)
	}
	if again, ok := s.timers.NextDue(); ok {
		return fmt.Sprintf("the same timer was due twice: %+v", again)
	}
	if n := s.timers.Pending(); n != 0 {
		return fmt.Sprintf("Pending() = %d after consuming the only timer; want 0", n)
	}
	return ""
}

// checkPastTimerIsImmediatelyDue: scheduling in the past is not an error and not
// a silent drop. A funding boundary computed from a stale snapshot must fire at
// once, which is the useful reading and the safe one.
func checkPastTimerIsImmediatelyDue(t *testing.T, s subject) string {
	s.clock.SetTimer("stale", s.clock.Now().Add(-time.Hour), 0)
	f, ok := s.timers.NextDue()
	if !ok {
		return "a timer scheduled an hour in the past was not due"
	}
	if f.ID != "stale" {
		return fmt.Sprintf("NextDue returned %q; want %q", f.ID, "stale")
	}
	return ""
}

// checkSetReplacesByID: re-arming an id replaces it. §3.5 says "cancel/replace by
// id", and the failure mode if it appends instead is a funding timer that fires
// twice — once at the stale time — which reads as two settlements.
func checkSetReplacesByID(t *testing.T, s subject) string {
	now := s.clock.Now()
	s.clock.SetTimer("reused", now.Add(s.step()), 0)
	s.clock.SetTimer("reused", now.Add(3*s.step()), 0)

	if n := s.timers.Pending(); n != 1 {
		return fmt.Sprintf("Pending() = %d after re-arming one id; want 1", n)
	}

	// The first, replaced deadline must not fire at all.
	s.advance(t, s.step()+s.grain)
	if f, ok := s.timers.NextDue(); ok {
		return fmt.Sprintf("the replaced deadline still fired: %+v", f)
	}

	s.advance(t, 2*s.step()+s.grain)
	if got := drain(s.timers); !idsEqual(got, []clock.TimerID{"reused"}) {
		return fmt.Sprintf("after the replacement deadline, drained %v; want [reused] exactly once", got)
	}
	return ""
}

// checkCancelRemoves: a cancelled timer never fires.
func checkCancelRemoves(t *testing.T, s subject) string {
	s.clock.SetTimer("doomed", s.clock.Now().Add(s.step()), 0)
	s.clock.CancelTimer("doomed")
	if n := s.timers.Pending(); n != 0 {
		return fmt.Sprintf("Pending() = %d after cancelling the only timer; want 0", n)
	}
	s.advance(t, s.step()+s.grain)
	if f, ok := s.timers.NextDue(); ok {
		return fmt.Sprintf("a cancelled timer fired: %+v", f)
	}
	return ""
}

// checkCancelUnknownIsNoOp: cancelling an id that was never armed changes
// nothing. A caller that cancels on every exit path should not have to track
// whether it ever armed the timer — and a cancel that removed something else
// would be far worse than one that errored.
func checkCancelUnknownIsNoOp(t *testing.T, s subject) string {
	s.clock.SetTimer("keeper", s.clock.Now().Add(s.step()), 0)
	s.clock.CancelTimer("never-armed")
	s.clock.CancelTimer("")
	if n := s.timers.Pending(); n != 1 {
		return fmt.Sprintf("Pending() = %d after cancelling unknown ids; want the 1 real timer intact", n)
	}
	s.advance(t, s.step()+s.grain)
	if got := drain(s.timers); !idsEqual(got, []clock.TimerID{"keeper"}) {
		return fmt.Sprintf("drained %v; want [keeper] — an unknown cancel disturbed a real timer", got)
	}
	return ""
}

// checkFiringOrder: the total order is (At, Prio, ID).
//
// This is the invariant with the most ways to be subtly wrong, so the fixture
// separates the three keys. Two timers share an instant and differ only in prio;
// two more share instant AND prio and differ only in id; and a later instant with
// the most urgent prio must still come last, so a comparator that sorted on prio
// first would be caught rather than merely reordered.
//
// Lower prio is more urgent, which makes the zero value the MOST urgent. That
// direction is deliberate: an unset priority on a safety timer must not silently
// send it to the back of the queue.
func checkFiringOrder(t *testing.T, s subject) string {
	now := s.clock.Now()
	early, late := now.Add(s.step()), now.Add(2*s.step())

	// Armed in an order that is wrong by every key, so nothing can pass by
	// accident of insertion order.
	s.clock.SetTimer("d-late-urgent", late, 0)
	s.clock.SetTimer("c-early-tie-z", early, 5)
	s.clock.SetTimer("a-early-tie-a", early, 5)
	s.clock.SetTimer("b-early-urgent", early, 1)

	s.advance(t, 2*s.step()+s.grain)

	want := []clock.TimerID{
		"b-early-urgent", // same instant as the ties, lower prio wins
		"a-early-tie-a",  // instant and prio tie, id breaks it
		"c-early-tie-z",
		"d-late-urgent", // later instant loses despite prio 0
	}
	if got := drain(s.timers); !idsEqual(got, want) {
		return fmt.Sprintf("firing order = %v; want %v", got, want)
	}
	return ""
}

// checkServerNowAppliesCalibration: ServerNow is Now plus the venue's offset,
// with the venue's band. Both signs are checked, because a sign error in the
// offset passes any test that only calibrates a venue ahead of us.
func checkServerNowAppliesCalibration(t *testing.T, s subject) string {
	for _, cal := range calibrations {
		before := s.clock.Now()
		st := s.clock.ServerNow(cal.Venue)
		after := s.clock.Now()

		if err := st.Validate(); err != nil {
			return fmt.Sprintf("ServerNow(%q).Validate() = %v; want a usable ServerTime", cal.Venue, err)
		}
		if got := st.Uncertainty(); got != cal.Uncertainty {
			return fmt.Sprintf("ServerNow(%q).Uncertainty() = %v; want %v", cal.Venue, got, cal.Uncertainty)
		}
		// The instant must sit between the two readings of Now, shifted by the
		// offset. Bracketing rather than equality is what lets one predicate
		// serve a clock that moves on its own.
		lo, hi := before.Add(cal.Offset), after.Add(cal.Offset)
		if st.Instant().Before(lo) || st.Instant().After(hi) {
			return fmt.Sprintf("ServerNow(%q).Instant() = %v; want it in [%v, %v] (Now + offset %v)",
				cal.Venue, st.Instant(), lo, hi, cal.Offset)
		}
	}
	return ""
}

// checkUncalibratedVenueIsUnusable: a venue with no calibration yields an unset
// ServerTime.
//
// This is the invariant that matters most, and it is about what NOT to return. A
// zero offset with a zero band is the most confident answer this package can
// give — "this venue's clock is exactly ours" — and returning it for an unknown
// venue would give that answer on the strength of no data at all. Every funding
// boundary would then read as decided.
func checkUncalibratedVenueIsUnusable(t *testing.T, s subject) string {
	st := s.clock.ServerNow(uncalibrated)
	if err := st.Validate(); !errors.Is(err, clock.ErrServerTimeUnset) {
		return fmt.Sprintf("ServerNow(%q).Validate() = %v; want ErrServerTimeUnset", uncalibrated, err)
	}
	if got := st.Passed(base); got != clock.Uncertain {
		return fmt.Sprintf("an uncalibrated venue answered Passed() = %v; want Uncertain", got)
	}
	// The empty venue is the same case, and it is the one a zero-valued config
	// field produces.
	if err := s.clock.ServerNow("").Validate(); !errors.Is(err, clock.ErrServerTimeUnset) {
		return fmt.Sprintf("ServerNow(\"\") validated; want ErrServerTimeUnset")
	}
	return ""
}

// step is the interval a predicate advances by. Real for Wall, nominal for
// TestClock — where it could be a nanosecond, but keeping it the same shape in
// both makes a predicate that accidentally depends on the magnitude visible.
func (s subject) step() time.Duration {
	if s.grain == 0 {
		return time.Second
	}
	return s.grain
}

// invariants is the suite. Named, so a failure says which contract broke.
var invariants = []struct {
	name  string
	check func(*testing.T, subject) string
}{
	{"Now is monotonic", checkNowMonotonic},
	{"a timer fires once, when due", checkTimerFiresOnceWhenDue},
	{"a past timer is immediately due", checkPastTimerIsImmediatelyDue},
	{"SetTimer replaces by id", checkSetReplacesByID},
	{"CancelTimer removes", checkCancelRemoves},
	{"cancelling an unknown id is a no-op", checkCancelUnknownIsNoOp},
	{"firing order is (At, Prio, ID)", checkFiringOrder},
	{"ServerNow applies the venue calibration", checkServerNowAppliesCalibration},
	{"an uncalibrated venue is unusable", checkUncalibratedVenueIsUnusable},
}

// ---------------------------------------------------------------------------
// Both implementations, same invariants
// ---------------------------------------------------------------------------

func TestConformance(t *testing.T) {
	for _, inv := range invariants {
		t.Run(inv.name, func(t *testing.T) {
			// A fresh pair per invariant: a predicate that left a timer armed
			// would otherwise fail the next one, and debugging that is
			// debugging the harness rather than the code.
			for _, s := range newSubjects(t) {
				t.Run(s.name, func(t *testing.T) {
					if diff := inv.check(t, s); diff != "" {
						t.Errorf("%s: %s", s.name, diff)
					}
				})
			}
		})
	}
}
