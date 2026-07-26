package events

import (
	"context"
	"sync"
)

// ControlLane is §3.4's lossless, ordered lane: private order, execution,
// account, liquidation and ADL events, plus OMS commands. "Never dropped; feeds
// the single-writer OMS in a documented total order."
//
// # Bounded, and a full lane BLOCKS
//
// §3.4 says two things that pull against each other — never drop, and never grow
// without bound — and this is where they meet. The resolution is back-pressure:
// the queue is bounded, and a producer that fills it waits.
//
// That means back-pressure reaches the socket reader, and that is the designed
// behaviour rather than a side effect. A stalled reader is visible in a metric
// and in a latency graph; a dropped execution event is visible nowhere, and the
// net-delta invariant then asserts against a number that is already wrong. The
// two rejected alternatives are recorded because both look reasonable: an
// UNBOUNDED queue never drops and never blocks, but its failure mode is an OOM
// kill mid-journal-write; and FAILING FAST on a full lane turns a transient
// consumer stall into a trading halt, which is a large penalty for a condition
// that resolves itself.
//
// Every blocking call takes a context, so "blocks" never means "cannot be shut
// down" — see Close.
//
// # It does not reorder, and it does not look at Prio
//
// FIFO, strictly. §3.5's documented total order is the DISPATCH order
// (IngressSeq), decided before an envelope reaches a lane; Prio is an input to
// the dispatcher's arbitration and to clock's timer queue, not to this queue.
// A lane that sorted by Prio would reorder envelopes within a SourceID — where
// VenueSeq is authoritative — and a replay would then apply a book delta ahead
// of the snapshot it depends on.
//
// # It does not validate envelopes, on purpose
//
// A lane is transport. §3.5 splits the stamping: a producer stamps
// AcceptOrdinal at ingestion, and the dispatcher stamps IngressSeq when it
// releases the envelope for consumption. An envelope in flight toward the
// dispatcher therefore has no IngressSeq yet and fails Envelope.Validate —
// so a lane that validated on Send would reject exactly the traffic it exists to
// carry. Validation belongs at the dispatch boundary, where the envelope is
// complete.
//
// Safe for concurrent use by multiple producers and consumers.
type ControlLane struct {
	q chan Envelope

	// closeOnce and closed implement shutdown without ever closing q.
	//
	// Closing a channel that senders are still writing to panics, and this lane
	// is specifically designed to have senders parked on it — so q is never
	// closed. The separate closed channel is what a parked sender selects on,
	// and closeOnce is what makes a shutdown path that runs twice survivable.
	closeOnce sync.Once
	closed    chan struct{}
}

// NewControlLane returns a lane holding up to capacity envelopes before senders
// block.
//
// A non-positive capacity is refused rather than normalised. Zero would make the
// channel a rendezvous — every send waiting for a receiver to be already
// blocked — so a single-goroutine boot sequence would deadlock on its first
// event, and "the lane is full" would be indistinguishable from "the consumer
// has not started yet".
func NewControlLane(capacity int) (*ControlLane, error) {
	if capacity <= 0 {
		return nil, ErrLaneCapacity
	}
	return &ControlLane{
		q:      make(chan Envelope, capacity),
		closed: make(chan struct{}),
	}, nil
}

// Send enqueues e, blocking while the lane is full.
//
// Returns ErrLaneClosed if the lane is closed, or the context's error if the
// caller gives up first. It never returns nil without having enqueued.
func (l *ControlLane) Send(ctx context.Context, e Envelope) error {
	// Checked before the blocking select rather than only inside it. With both
	// a free slot and a closed lane ready, a select picks at random — so
	// without this a send after Close would succeed roughly half the time,
	// which is worse than either behaviour consistently.
	select {
	case <-l.closed:
		return ErrLaneClosed
	default:
	}

	select {
	case l.q <- e:
		return nil
	case <-l.closed:
		return ErrLaneClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive returns the next envelope in submission order, blocking until one
// arrives.
//
// Returns ErrLaneClosed only once the lane is closed AND drained — that
// ordering is §3.4's "shutdown drains the control lane before exit", and it is
// why Close means "no more input" rather than "discard the backlog".
func (l *ControlLane) Receive(ctx context.Context) (Envelope, error) {
	// The non-blocking check is what makes the drain deterministic. On a closed
	// lane with a backlog, both cases below are ready and select would take
	// them in random order — draining a random prefix and then reporting the
	// lane empty, green most of the time and reproducible in neither direction.
	select {
	case e := <-l.q:
		return e, nil
	default:
	}

	select {
	case e := <-l.q:
		return e, nil
	case <-l.closed:
		return Envelope{}, ErrLaneClosed
	case <-ctx.Done():
		return Envelope{}, ctx.Err()
	}
}

// TryReceive returns a queued envelope without blocking, reporting whether there
// was one.
//
// This is the shutdown drain's primitive, and it is also how a dispatcher pumps
// the lane from a select loop without committing to a blocking read — the same
// pull-based shape clock.TimerSource takes, and for the same reason: the
// arbitration that decides the consumed order belongs in one place, and a lane
// that pushed would force the dispatcher to re-queue what it was handed.
func (l *ControlLane) TryReceive() (Envelope, bool) {
	select {
	case e := <-l.q:
		return e, true
	default:
		return Envelope{}, false
	}
}

// Close stops the lane accepting new envelopes and releases every parked sender.
//
// It does NOT discard the backlog: Receive keeps returning queued envelopes
// until there are none left. Idempotent, because shutdown paths run twice more
// often than anyone plans for.
func (l *ControlLane) Close() {
	l.closeOnce.Do(func() { close(l.closed) })
}

// Len reports the current backlog — what a slow-consumer alarm watches.
//
// A point-in-time reading, and inherently stale the instant it returns. It is
// for metrics and for tests, never for a control decision: "is there room" is
// answered by Send blocking or not, not by comparing this against a capacity.
func (l *ControlLane) Len() int { return len(l.q) }
