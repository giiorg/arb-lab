package clock_test

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// The negative controls for the conformance suite.
//
// A conformance suite is the easiest kind of test to write vacuously. Nine
// invariants that all pass against both implementations look like nine
// guarantees, and they look identical to nine assertions that could never fail —
// which for a clock is a live risk, because the cheap things to check (does Now
// return something, does SetTimer not panic) are exactly the vacuous ones.
//
// So each defect below is a plausible way to get a Clock wrong, and the suite
// must reject every one. §5c's discipline, applied to a two-implementation
// conformance suite rather than to a single function.

type defect string

const (
	noDefect defect = "faithful"

	// The queue defects.
	appendsInsteadOfReplacing defect = "SetTimer appends instead of replacing by id"
	cancelIsANoOp             defect = "CancelTimer does nothing"
	consumingDoesNotRemove    defect = "a consumed timer stays due"
	dropsPastTimers           defect = "a timer scheduled in the past is dropped"

	// The ordering defects. These are the ones a hand-written comparator gets
	// wrong, and each is a single clause out of place.
	idBeforePrio  defect = "ties broken by id before prio"
	prioReversed  defect = "higher prio treated as more urgent"
	ignoresPrio   defect = "prio ignored entirely"
	ignoresAtTime defect = "scheduled instant ignored, ordered by prio alone"

	// The clock defects. Two of them, because checkNowMonotonic has two
	// clauses and one defect only exercised one: a Now that retreats by the
	// same amount time advanced reads as *stalled*, not as reversed, so the
	// "went backwards" clause was never reached. Splitting the defect is what
	// makes both clauses non-vacuous — the same problem the controls exist to
	// find, found one level up.
	nowGoesBackwards defect = "Now goes backwards"
	nowStandsStill   defect = "Now never advances"

	// The calibration defect, and the most dangerous one: an unknown venue
	// answered with perfect confidence.
	confidentAboutUnknownVenue defect = "an uncalibrated venue reads as perfectly synchronised"
)

// broken is a Clock and TimerSource with exactly one thing wrong. Everything it
// does not deliberately break, it does correctly — a control that was wrong in
// several ways at once would not tell you which invariant caught it.
type broken struct {
	d     defect
	inner *clock.TestClock // owns time and the venue calibrations

	byID  map[clock.TimerID]clock.Fired
	list  []clock.Fired // only appendsInsteadOfReplacing uses this
	backs int           // only nowGoesBackwards uses this
}

func newBroken(t *testing.T, d defect) *broken {
	t.Helper()
	inner, err := clock.NewTestClock(base, calibrations)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return &broken{d: d, inner: inner, byID: make(map[clock.TimerID]clock.Fired)}
}

func (b *broken) Now() time.Time {
	switch b.d {
	case nowGoesBackwards:
		// Each call retreats faster than time advances, so successive readings
		// are strictly decreasing — an unclamped wall clock across an NTP step
		// backwards. The retreat has to outpace the advance, or this reads as a
		// stall instead and the wrong clause catches it.
		b.backs++
		return b.inner.Now().Add(-time.Duration(b.backs) * 2 * time.Second)
	case nowStandsStill:
		return base
	}
	return b.inner.Now()
}

func (b *broken) ServerNow(v clock.Venue) clock.ServerTime {
	if b.d == confidentAboutUnknownVenue {
		if st := b.inner.ServerNow(v); st.Validate() != nil {
			// Zero offset, zero band: "this venue's clock is exactly ours,
			// and I am certain" — asserted on the strength of no data.
			st, err := clock.NewServerTime(b.inner.Now(), 0)
			if err != nil {
				return clock.ServerTime{}
			}
			return st
		}
	}
	return b.inner.ServerNow(v)
}

func (b *broken) SetTimer(id clock.TimerID, at time.Time, prio uint8) {
	f := clock.Fired{ID: id, At: at, Prio: prio}
	if b.d == dropsPastTimers && at.Before(b.inner.Now()) {
		return
	}
	if b.d == appendsInsteadOfReplacing {
		b.list = append(b.list, f)
		return
	}
	b.byID[id] = f
}

func (b *broken) CancelTimer(id clock.TimerID) {
	if b.d == cancelIsANoOp {
		return
	}
	if b.d == appendsInsteadOfReplacing {
		kept := b.list[:0]
		for _, f := range b.list {
			if f.ID != id {
				kept = append(kept, f)
			}
		}
		b.list = kept
		return
	}
	delete(b.byID, id)
}

func (b *broken) Pending() int {
	if b.d == appendsInsteadOfReplacing {
		return len(b.list)
	}
	return len(b.byID)
}

func (b *broken) NextDue() (clock.Fired, bool) {
	now := b.inner.Now()
	pool := b.list
	if b.d != appendsInsteadOfReplacing {
		pool = pool[:0]
		for _, f := range b.byID {
			pool = append(pool, f)
		}
	}

	var best clock.Fired
	found := false
	for _, f := range pool {
		if f.At.After(now) {
			continue
		}
		if !found || b.earlier(f, best) {
			best, found = f, true
		}
	}
	if !found {
		return clock.Fired{}, false
	}
	if b.d != consumingDoesNotRemove {
		b.remove(best)
	}
	return best, true
}

func (b *broken) remove(f clock.Fired) {
	if b.d == appendsInsteadOfReplacing {
		for i, cand := range b.list {
			if cand == f {
				b.list = append(b.list[:i], b.list[i+1:]...)
				return
			}
		}
		return
	}
	delete(b.byID, f.ID)
}

// earlier is the comparator, wrong in one clause per ordering defect.
func (b *broken) earlier(x, y clock.Fired) bool {
	switch b.d {
	case idBeforePrio:
		if !x.At.Equal(y.At) {
			return x.At.Before(y.At)
		}
		if x.ID != y.ID {
			return x.ID < y.ID
		}
		return x.Prio < y.Prio
	case prioReversed:
		if !x.At.Equal(y.At) {
			return x.At.Before(y.At)
		}
		if x.Prio != y.Prio {
			return x.Prio > y.Prio
		}
		return x.ID < y.ID
	case ignoresPrio:
		if !x.At.Equal(y.At) {
			return x.At.Before(y.At)
		}
		return x.ID < y.ID
	case ignoresAtTime:
		if x.Prio != y.Prio {
			return x.Prio < y.Prio
		}
		return x.ID < y.ID
	default:
		if !x.At.Equal(y.At) {
			return x.At.Before(y.At)
		}
		if x.Prio != y.Prio {
			return x.Prio < y.Prio
		}
		return x.ID < y.ID
	}
}

// asSubject wraps a broken clock in the harness the predicates take. Its grain is
// zero and its advance drives the inner TestClock, so a control failure is never
// a timing artefact.
func (b *broken) asSubject() subject {
	return subject{
		name: string(b.d), clock: b, timers: b, grain: 0,
		advance: func(t *testing.T, d time.Duration) {
			t.Helper()
			if err := b.inner.Advance(d); err != nil {
				t.Fatalf("Advance(%v): %v", d, err)
			}
		},
	}
}

func TestConformance_NegativeControls(t *testing.T) {
	defects := []defect{
		appendsInsteadOfReplacing,
		cancelIsANoOp,
		consumingDoesNotRemove,
		dropsPastTimers,
		idBeforePrio,
		prioReversed,
		ignoresPrio,
		ignoresAtTime,
		nowGoesBackwards,
		nowStandsStill,
		confidentAboutUnknownVenue,
	}

	for _, d := range defects {
		t.Run(string(d), func(t *testing.T) {
			var caught []string
			for _, inv := range invariants {
				// A fresh broken clock per invariant, matching how
				// TestConformance runs the real ones.
				s := newBroken(t, d).asSubject()
				if diff := inv.check(t, s); diff != "" {
					caught = append(caught, inv.name+": "+diff)
				}
			}
			if len(caught) == 0 {
				t.Errorf("the conformance suite accepted a Clock with a known "+
					"defect (%s); it cannot be distinguishing anything", d)
				return
			}
			for _, c := range caught {
				t.Logf("rejected by %s", c)
			}
		})
	}

	// The positive control, and here it does double duty. It shows the
	// predicates do not object to a correct implementation, AND it shows the
	// broken-clock wrapper itself is not what causes the failures above — the
	// same wrapper, with no defect selected, must pass every invariant.
	t.Run("positive control: the faithful wrapper", func(t *testing.T) {
		for _, inv := range invariants {
			s := newBroken(t, noDefect).asSubject()
			if diff := inv.check(t, s); diff != "" {
				t.Errorf("a faithful implementation was rejected by %q: %s", inv.name, diff)
			}
		}
	})
}
