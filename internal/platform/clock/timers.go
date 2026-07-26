package clock

import "time"

// timerQueue is the ordering rule, shared by both implementations so that
// "earliest by (At, Prio, ID)" has one definition rather than two that drift.
// That sharing is deliberate: the conformance suite's whole claim is that Wall and
// TestClock are interchangeable, and a suite that passed against two independent
// comparators would only be proving they agree today.
//
// A map plus a linear scan, not a heap. A heap needs an index per entry to support
// cancel-and-replace by id, which is the operation §3.5 asks for most, and the
// queue holds a handful of funding-boundary and order-timeout timers rather than
// a million. The scan is O(n) on a small n; the bookkeeping would be a permanent
// source of index bugs for no measurable gain. Revisit if Pending() is ever
// observed in the thousands.
//
// Not safe for concurrent use on its own — each implementation owns the locking,
// because Wall needs it and TestClock does not want to pay for it.
type timerQueue struct {
	byID map[TimerID]Fired
}

func newTimerQueue() timerQueue {
	return timerQueue{byID: make(map[TimerID]Fired)}
}

// set arms or re-arms a timer. Re-arming replaces, which is what makes the id the
// timer's identity rather than just a label: the alternative — appending — fires
// a funding timer twice, once at the stale deadline, which reads downstream as
// two settlements.
func (q timerQueue) set(f Fired) {
	q.byID[f.ID] = f
}

func (q timerQueue) cancel(id TimerID) {
	delete(q.byID, id)
}

func (q timerQueue) pending() int {
	return len(q.byID)
}

// popDue removes and returns the earliest timer due at or before now.
//
// "Due at or before" rather than strictly before: a timer scheduled for exactly
// this instant has arrived. The boundary is the event, the same reading
// ServerTime.Passed takes.
func (q timerQueue) popDue(now time.Time) (Fired, bool) {
	var best Fired
	found := false
	for _, f := range q.byID {
		if f.At.After(now) {
			continue
		}
		if !found || earlier(f, best) {
			best, found = f, true
		}
	}
	if found {
		delete(q.byID, best.ID)
	}
	return best, found
}

// earliestScheduled returns the earliest timer regardless of whether it is due,
// which is what AdvanceToNextTimer needs to know where to jump to.
//
// It lives here rather than in testclock.go so that every use of the ordering
// goes through this file. AdvanceToNextTimer picking a different "earliest" than
// popDue would mean jumping to one instant and then firing a different timer, and
// the two loops would have to be kept in step by hand.
func (q timerQueue) earliestScheduled() (Fired, bool) {
	var best Fired
	found := false
	for _, f := range q.byID {
		if !found || earlier(f, best) {
			best, found = f, true
		}
	}
	return best, found
}

// clampMonotonic holds the no-going-backwards invariant, as a pure function so it
// can be tested. Wall reads time.Now() directly and cannot be given a fake, so
// wiring it up here is what makes the guarantee checkable rather than merely
// claimed — an untested guarantee is worse than none, because it gets relied on.
//
// The clamp is not paranoia about a hypothetical. NTP steps the system clock, and
// on a step backwards every deadline comparison in this package would see time
// reverse. Go's monotonic reading protects the difference between two time.Times,
// but Now hands out absolute instants that get compared against deadlines
// computed earlier, so the high-water mark is what actually holds the invariant.
func clampMonotonic(now, high time.Time) time.Time {
	if now.Before(high) {
		return high
	}
	return now
}

// earlier is the total order: instant, then priority, then id.
//
// Lower Prio is more urgent, which makes the zero value the MOST urgent. That
// direction is chosen rather than inherited: with the opposite convention an
// unset priority would silently send a safety timer to the back of the queue,
// and a timer that fires late is the failure this scheduler exists to prevent.
// Noisy-but-early beats quiet-but-late.
//
// The id tie-break is what makes the order total. Ids are unique keys, so no two
// timers can tie on all three — which is precisely the property a replay needs:
// draining the queue twice from the same state yields the same sequence.
func earlier(a, b Fired) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	if a.Prio != b.Prio {
		return a.Prio < b.Prio
	}
	return a.ID < b.ID
}
