package ratelimit

import (
	"math"
	"time"
)

// charge is one consumption of a budget: how much, and when.
type charge struct {
	at    time.Time
	units uint64
}

// window is one limit's sliding-window counter.
//
// # Sliding rather than fixed, deliberately
//
// The venues meter over fixed intervals aligned to a server boundary we cannot
// see. Guessing that alignment wrong lets a burst land at the end of our window
// and again at the start of the next — twice the intended rate across the seam,
// with a limiter that looks correct. A sliding window returns budget only once a
// charge has been held for the whole interval, so it cannot produce that seam.
// It is stricter than the venue's own rule, and it errs by refusing.
//
// # used is recomputed, not decremented
//
// evict re-sums the surviving charges instead of subtracting the expired ones.
// Subtracting is O(1) and accumulates drift the moment any other path adjusts
// used — and there is such a path, because observe folds in a venue reading. A
// counter that drifts low is a governor that admits past capacity while
// reporting that it did not.
//
// The cost is bounded by the budget: a charge is only recorded when it was
// admitted, and admission caps the total at Capacity, so the log holds at most
// Capacity entries per window (plus one per observation).
type window struct {
	limit   Limit
	charges []charge
	used    uint64

	// reserved is provisional occupancy held beside charges. It is untimed and
	// invisible to observe because a venue report can never include calls that
	// have not been sent; folding the two categories together would let Cancel
	// erase consumption the venue actually reported.
	reserved uint64
}

func newWindow(l Limit) *window {
	return &window{limit: l}
}

// evict drops charges that have been held for the full interval and re-sums the
// rest.
//
// The comparison is `>= Interval`: a charge is held for the entire window and
// released at the instant it completes, which matches "a one-minute budget" as
// an operator reads it.
//
// Charges are appended in non-decreasing time order — Governor reads every
// instant from one clock.Clock, whose Wall implementation clamps against
// backwards steps — so the expired ones are a prefix and eviction is a slice.
// Re-slicing shrinks the capacity too, so the retained backing array is
// reallocated by a later append rather than growing forever.
func (w *window) evict(now time.Time) {
	cut := 0
	for cut < len(w.charges) && now.Sub(w.charges[cut].at) >= w.limit.Interval {
		cut++
	}
	if cut > 0 {
		w.charges = w.charges[cut:]
	}

	var total uint64
	for _, c := range w.charges {
		total = addSaturating(total, c.units)
	}
	w.used = total
}

// usedAt is the budget consumed as of now.
func (w *window) usedAt(now time.Time) uint64 {
	w.evict(now)
	return w.used
}

// admits reports whether units more may be charged without passing ceiling.
//
// A zero charge is always admitted: a request that costs this dimension nothing
// cannot push it over, and refusing one would mean an exhausted order budget
// blocked a plain market-data read that touches no order slot.
//
// The comparison is written as `units <= ceiling - used` rather than
// `used + units <= ceiling` so that a caller-supplied cost near the top of the
// uint64 range cannot wrap the sum into a small number and be admitted. The
// `used >= ceiling` guard above it is what keeps that subtraction from
// underflowing, and used CAN exceed a ceiling: a header reading ratchets in
// consumption we never spent ourselves, and the opening ceiling is well below
// capacity in any case.
//
// Expected surviving mutant: that guard narrowed from `>=` to `>` is equivalent.
// At exactly the ceiling the subtraction yields zero and a non-zero charge fails
// `units <= 0` anyway. Its real content is the strictly-above case, which is
// bite-checked by deleting the guard.
func (w *window) admits(now time.Time, units, ceiling uint64) bool {
	if units == 0 {
		return true
	}
	used := w.usedAt(now)
	if used >= ceiling {
		return false
	}
	avail := ceiling - used
	if w.reserved >= avail {
		return false
	}
	return units <= avail-w.reserved
}

// reserve records untimed occupancy after Reserve has checked the batch sum.
// That admission bounds the addition below capacity, so saturation here would
// hide an invariant violation rather than make the governor safer.
func (w *window) reserve(units uint64) {
	w.reserved += units
}

// release removes occupancy for a still-reserved hold entry. Hold entries are
// the only callers, and their terminal state makes a second release
// inexpressible, so an underflow assertion here would duplicate that state
// machine without strengthening it.
func (w *window) release(units uint64) {
	w.reserved -= units
}

// consume converts occupancy into a charge under the governor's one lock. The
// conversion is one operation so no admission can observe a gap between its
// halves, and the charge starts at consume rather than at reservation time.
func (w *window) consume(now time.Time, units uint64) {
	w.reserved -= units
	w.charge(now, units)
}

// charge records units consumed at now.
func (w *window) charge(now time.Time, units uint64) {
	w.evict(now)
	if units == 0 {
		return
	}
	w.charges = append(w.charges, charge{at: now, units: units})
	w.used = addSaturating(w.used, units)
}

// freeAt is the earliest instant at which need units could be charged without
// passing ceiling.
//
// It walks the charges oldest-first and accumulates, because the answer is
// rarely the first expiry: freeing one charge may not free enough, and naming
// its expiry would send the caller back to be refused again. Accumulating is
// what makes the returned instant one a retry can actually succeed at.
//
// The zero time means "not by waiting" — the need exceeds the ceiling outright,
// so no amount of expiry helps. A caller must treat it as such rather than as
// an instant already past, which is why Decision documents it explicitly.
//
// Occupancy is deliberately ignored. It can disappear at any instant through
// Cancel, so the earliest instant a retry could succeed is the charges-only
// answer. Subtracting reserved from the ceiling would either overload zero as a
// false "never" or invent a release instant that occupancy does not have.
func (w *window) freeAt(now time.Time, need, ceiling uint64) time.Time {
	if need > ceiling {
		return time.Time{}
	}
	used := w.usedAt(now)
	// Write this comparison in the non-underflowing direction, mirroring
	// admits above. used may exceed ceiling after an observation or when a
	// lower-priority request reaches its own ceiling; subtracting used from the
	// ceiling would then wrap and falsely report that a retry succeeds now.
	// need <= ceiling was established above, so ceiling-need is safe.
	if used <= ceiling-need {
		return now
	}

	mustFree := used - (ceiling - need)
	var freed uint64
	for _, c := range w.charges {
		freed = addSaturating(freed, c.units)
		if freed >= mustFree {
			return c.at.Add(w.limit.Interval)
		}
	}

	// Unreachable through a validated profile: freeing every charge frees
	// `used`, and mustFree is at most used. Returning the zero time keeps the
	// function total rather than leaving a fall-through nobody can name.
	return time.Time{}
}

// observe folds a venue-reported consumption into the window, upward only.
//
// See the package doc for the argument. In short: a reading is always stale, so
// one BELOW the local count is what staleness looks like and adopting it would
// discard the requests sent after the one that carried the header; one ABOVE it
// is information we have no other way to get — an endpoint whose weight we
// under-estimated, or another consumer on the same IP.
//
// The correction is recorded as an ordinary charge at now rather than as a
// separate floor. That keeps one mechanism instead of two, and it ages the
// correction out conservatively: the usage it stands for really expired earlier
// than this models it expiring.
func (w *window) observe(now time.Time, reported uint64) {
	used := w.usedAt(now)
	if reported <= used {
		return
	}
	w.charge(now, reported-used)
}

// addSaturating clamps instead of wrapping.
//
// The inputs include a venue-reported number, so the range is not ours to
// bound. Wrapping would turn a nonsensical reading into a SMALL used value,
// which admits traffic; clamping turns it into a large one, which refuses. The
// direction is the whole reason the helper exists.
func addSaturating(a, b uint64) uint64 {
	if a > math.MaxUint64-b {
		return math.MaxUint64
	}
	return a + b
}
