package capture

import (
	"fmt"
	"sync"
)

// Queue is §32f's bounded hand-off: reader → bounded channel → writer.
//
// # Never blocking, and the rejected alternative is the interesting one
//
// "Never lose data" sounds strictly better, so §32f states why it is not:
// blocking the socket reader applies TCP back-pressure, the venue eventually
// disconnects, and the outcome is a LARGER gap arrived at by a slower path.
// Dropping loses less and records what it lost.
//
// # Drop-newest, not drop-oldest
//
// Either policy loses a contiguous run. Drop-newest keeps the consumer's stream
// strictly increasing in CaptureSeq — the writer never has to reorder or rewrite
// — and it makes the hole self-describing: the lost sequences are exactly the
// ones between the last accepted and the next accepted.
//
// # One producer, one consumer
//
// §32d puts one reader goroutine on each (SourceID, Topic), so Push has a single
// caller and the drop bookkeeping needs no compare-and-swap. The mutex is there
// for Stats, which a metrics scrape reads from a third goroutine — and it is held
// across the send only because the send is non-blocking, so it can never park a
// goroutine holding a lock.
type Queue struct {
	ch chan queued

	mu        sync.Mutex
	pending   Gap
	drops     uint64
	accepted  uint64
	highWater int
}

// queued is one record plus the hole that closed immediately before it.
//
// Carrying the gap WITH the record rather than as a separate queue entry is what
// keeps the two in order under every interleaving: a gap pushed as its own entry
// could be dropped by the very overflow it describes, which is the one message
// that must never be lost. §32d makes the gap record corroboration rather than
// the only witness — the hole in CaptureSeq is the primary evidence — but losing
// the corroboration precisely when it matters is still avoidable, and here it
// costs a pointer.
type queued struct {
	Record Record
	Gap    *Gap
}

// NewQueue returns a queue of the given depth. §32j's default is 4096; there is
// no default here, for WriterConfig's reason — a depth of zero is an unbuffered
// channel, which turns "never blocks the reader" into "drops every frame the
// writer is not already waiting for".
func NewQueue(depth int) (*Queue, error) {
	if depth <= 0 {
		return nil, fmt.Errorf("queue depth %d: %w", depth, ErrQueueDepth)
	}
	return &Queue{ch: make(chan queued, depth)}, nil
}

// Push offers a record to the writer and reports whether it was accepted.
//
// Called by the reader goroutine the instant a frame comes off the socket, after
// it has stamped CaptureSeq — §32d's placement, and the reason a drop is visible
// as a hole in the file with no other evidence required.
func (q *Queue) Push(r Record) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := queued{Record: r}
	if q.pending.Count > 0 {
		hole := q.pending
		item.Gap = &hole
	}

	select {
	case q.ch <- item:
		q.pending = Gap{}
		q.accepted++
		if n := len(q.ch); n > q.highWater {
			q.highWater = n
		}
		return true
	default:
		q.drops++
		if q.pending.Count == 0 {
			q.pending = Gap{Cause: CauseOverflow, From: r.CaptureSeq, To: r.CaptureSeq, Count: 1}
		} else {
			q.pending.To = r.CaptureSeq
			q.pending.Count++
		}
		return false
	}
}

// Receive hands the consumer the next record, and the hole that preceded it if
// there was one. It reports false once the queue is closed and drained.
func (q *Queue) Receive() (Record, *Gap, bool) {
	item, ok := <-q.ch
	return item.Record, item.Gap, ok
}

// Close stops the queue. Only the producer may call it, and only once — the same
// contract as closing a channel, because that is what it is.
//
// A hole still open at close is REPORTED rather than dropped, via PendingGap:
// the last frames before a shutdown are exactly the ones a stop-under-load loses,
// and a queue that discarded that hole would make the tidy case — we stopped
// while overflowing — the one case with no record of the loss.
func (q *Queue) Close() { close(q.ch) }

// PendingGap reports a hole that has not yet been attached to a record, which
// happens when the queue overflowed and then stopped.
func (q *Queue) PendingGap() (Gap, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending, q.pending.Count > 0
}

// QueueStats is §32j's queue observability: drops and the high-water mark.
type QueueStats struct {
	Accepted  uint64
	Drops     uint64
	HighWater int
	Depth     int
}

// Stats is safe to call from a goroutine that is neither the producer nor the
// consumer — a metrics scrape, which is the realistic third reader.
func (q *Queue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return QueueStats{Accepted: q.accepted, Drops: q.drops, HighWater: q.highWater, Depth: cap(q.ch)}
}
