package events

import "sync"

// MarketDataLane is §3.4's bounded, latest-value lane: book, mark and funding
// updates in per-VenueInstrument mailboxes.
//
// # It never blocks, so it has to lose something
//
// The opposite resolution from ControlLane, and deliberately so. A market-data
// feed must never apply back-pressure to a reader that also carries execution
// traffic, so Publish always returns immediately — which means a slow consumer
// costs data rather than latency. The design is entirely about losing the right
// data:
//
//   - A superseded book snapshot is worth nothing once a newer one exists, so
//     it coalesces away at no cost (LatestWins).
//   - A superseded perp MARK is not, because the worst excursion is exactly the
//     value the safety path needs (AdversePreserving) `[→ R2c-1]`.
//   - Anything that does not coalesce sits in a bounded ring that drops its
//     OLDEST entry and counts the drop — §3.4's "drop-oldest + freshness gap
//     counter", never unbounded growth.
//
// Drop-oldest rather than drop-newest: the newest update is the one closest to
// the truth, so refusing it to preserve history would leave the consumer reading
// the past.
//
// # Per-source mailboxes, and why that is not just tidiness
//
// One shared queue would let a busy source starve a quiet one and would apply
// one source's coalescing rule to another's traffic — a spot book update
// coalescing over a perp mark excursion, which is R2c-1's failure with extra
// steps. Spot and perp on one symbol are distinct sources `[→ Z1]`, so they get
// distinct mailboxes.
//
// # Pull-based
//
// Consume is called by the dispatcher; the lane never pushes. Same shape as
// clock.TimerSource and for the same reason recorded there — the arbitration
// that decides the consumed order belongs in one place, and a lane that pushed
// would force the dispatcher to re-queue what it had been handed.
//
// Safe for concurrent use.
type MarketDataLane struct {
	depth  int
	policy CoalescePolicy

	mu    sync.Mutex
	boxes map[SourceID]*mailbox
}

// mailbox is one source's bounded ring.
//
// A slice rather than a container/ring: depth is small (a handful of snapshots),
// the operations are append and take-front, and the merge needs to address the
// NEWEST entry — which is a bare index here and an index-arithmetic bug waiting
// to happen in a ring buffer. The same trade clock's timerQueue records.
type mailbox struct {
	q       []Envelope
	dropped uint64
	merge   MergeFunc
}

// NewMarketDataLane returns a lane whose mailboxes hold up to depth envelopes,
// coalescing according to policy.
//
// policy is required rather than defaulted, and that is the "zero value is
// unusable" rule (§11) applied to a behaviour instead of a field. Either default
// would be silently wrong somewhere: defaulting to coalescing erases mark
// excursions, and defaulting to none turns the lane into a drop-oldest ring that
// looks like it is coalescing. NoCoalescing exists so that choosing it is a
// visible act.
func NewMarketDataLane(depth int, policy CoalescePolicy) (*MarketDataLane, error) {
	if depth <= 0 {
		return nil, ErrLaneCapacity
	}
	if policy == nil {
		return nil, ErrCoalescePolicyNil
	}
	return &MarketDataLane{
		depth:  depth,
		policy: policy,
		boxes:  make(map[SourceID]*mailbox),
	}, nil
}

// Publish delivers e to its source's mailbox. It never blocks.
//
// It refuses traffic that does not belong on this lane. That guard is the one
// place this package can catch the misrouting its whole design is aimed at: an
// execution event delivered here would be coalesced away, and a fill that never
// arrives is a position the ledger does not know about — no error, no log, and a
// net-delta invariant asserting against a number that is already wrong. Routing
// is StreamKind.Lane's answer, and this checks it rather than trusting the
// caller to have asked.
func (l *MarketDataLane) Publish(e Envelope) error {
	if e.Source.Lane() != LaneMarketData {
		return ErrWrongLane
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	box, ok := l.boxes[e.Source]
	if !ok {
		box = &mailbox{merge: l.policy(e.Source)}
		l.boxes[e.Source] = box
	}
	box.publish(e, l.depth)
	return nil
}

// publish applies the merge rule and then the bound, in that order.
//
// The order matters: merging first is what lets a lane with depth 1 hold a
// coalescing source at all. Bounding first would drop the incumbent an
// adverse-preserving rule was about to keep.
func (m *mailbox) publish(e Envelope, depth int) {
	if m.merge != nil && len(m.q) > 0 {
		newest := len(m.q) - 1
		if merged, ok := m.merge(m.q[newest], e); ok {
			m.q[newest] = merged
			return
		}
	}

	m.q = append(m.q, e)
	for len(m.q) > depth {
		// Drop-oldest. The count is the "freshness gap counter" §3.4 asks for:
		// it is what makes the loss measured rather than silent, and a consumer
		// reading a non-zero value knows its view has a hole in it.
		m.q = m.q[1:]
		m.dropped++
	}
}

// Consume takes the oldest envelope from a source's mailbox, reporting whether
// there was one.
//
// Oldest-first even though the mailbox is latest-value: coalescing decides WHAT
// is in the queue, and once something is in it, order is order. Taking newest
// first would reorder within a SourceID, where VenueSeq is authoritative.
//
// This is also what resets an adverse-preserving window — R2c-1's rule is "the
// worst mark SINCE THE OMS LAST CONSUMED IT", so consumption is the boundary,
// and without it the rule would be a permanent high-water mark that never came
// back down.
func (l *MarketDataLane) Consume(s SourceID) (Envelope, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	box, ok := l.boxes[s]
	if !ok || len(box.q) == 0 {
		return Envelope{}, false
	}
	e := box.q[0]

	// Clear the slot before reslicing so the envelope's payload is not kept
	// alive by the backing array. The queue is small but a book snapshot is
	// not, and this is a per-instrument mailbox that lives for the process.
	box.q[0] = Envelope{}
	box.q = box.q[1:]
	return e, true
}

// Dropped reports how many envelopes this source's mailbox has discarded on
// overflow — §3.4's freshness gap counter.
//
// Cumulative and never reset: it answers "has this source ever had a gap",
// which is the question a replay's completeness depends on. A rate is the
// caller's to derive.
func (l *MarketDataLane) Dropped(s SourceID) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if box, ok := l.boxes[s]; ok {
		return box.dropped
	}
	return 0
}

// Len reports a source's current backlog. Point-in-time, for metrics and tests.
func (l *MarketDataLane) Len(s SourceID) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if box, ok := l.boxes[s]; ok {
		return len(box.q)
	}
	return 0
}

// Sources lists the sources that have had traffic.
//
// Unordered — it is built from a map, and Go randomises that iteration
// deliberately. Callers that need a stable order must sort, and none should be
// deriving a dispatch order from this: the consumed order is the dispatcher's to
// decide and to persist, not something to read off a map.
func (l *MarketDataLane) Sources() []SourceID {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]SourceID, 0, len(l.boxes))
	for s := range l.boxes {
		out = append(out, s)
	}
	return out
}
