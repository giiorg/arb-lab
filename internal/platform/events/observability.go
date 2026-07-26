package events

import (
	"sync"
	"sync/atomic"
)

// ObservabilityLane is §3.4's isolated, lossy lane: metrics, logs and traces.
// "Back-pressure here never blocks trading."
//
// # Lossy is the guarantee, not the concession
//
// This is the mirror image of ControlLane, and the mirroring is deliberate. Both
// face the same question — what happens when the consumer falls behind — and the
// answers are opposite because the stakes are opposite. A lost execution event
// is lost execution truth. A lost trace is a lost trace.
//
// So Send never blocks and never grows the queue: it returns false and counts
// the drop. The specific failure this prevents is a Prometheus scrape or a log
// shipper stalling on a network hiccup and taking the trading loop down with it —
// observability that can halt the thing it observes is worse than none, because
// it adds a failure mode to the system it was added to protect.
//
// # It drops the NEWEST, which is the opposite of the market-data lane
//
// There, updates supersede one another, so the oldest is the one worth losing.
// Here they are independent log lines and spans — none supersedes another — so
// the lane refuses the newcomer, which is also the choice that touches no
// already-accepted entry. Stated because "drop" alone does not say which, and
// switching to drop-oldest would silently retract a metric that had already been
// accepted.
//
// # The drop count is exact, not approximate
//
// It is the only evidence that anything was lost. A racy counter turns "we
// dropped four thousand spans" into a number nobody can act on, so every send
// either lands or is counted, and the two add up.
//
// Safe for concurrent use.
type ObservabilityLane struct {
	q       chan Envelope
	dropped atomic.Uint64

	closeOnce sync.Once
	closed    chan struct{}
}

// NewObservabilityLane returns a lane holding up to capacity envelopes before it
// starts dropping.
//
// A non-positive capacity is refused rather than normalised: a zero-capacity
// lossy lane drops everything while reporting itself as working, which is the
// worst of both — no data and no signal that there is no data.
func NewObservabilityLane(capacity int) (*ObservabilityLane, error) {
	if capacity <= 0 {
		return nil, ErrLaneCapacity
	}
	return &ObservabilityLane{
		q:      make(chan Envelope, capacity),
		closed: make(chan struct{}),
	}, nil
}

// Send offers e to the lane and reports whether it was accepted. It never
// blocks.
//
// The boolean is returned rather than an error because a drop is this lane's
// normal, designed behaviour, not a fault — and an error would invite a caller
// to handle it, which on this lane means doing something expensive on the
// trading path in response to a metric not being recorded. Callers that want to
// know how much was lost read Dropped.
func (l *ObservabilityLane) Send(e Envelope) bool {
	// Checked first, and non-blockingly, so that a send after Close is
	// consistently refused rather than winning a random select against a free
	// slot.
	select {
	case <-l.closed:
		l.dropped.Add(1)
		return false
	default:
	}

	select {
	case l.q <- e:
		return true
	default:
		l.dropped.Add(1)
		return false
	}
}

// TryReceive takes the next envelope without blocking, reporting whether there
// was one.
//
// There is no blocking Receive, deliberately. An exporter that blocked waiting
// for telemetry would be a goroutine parked on the trading process's behalf for
// as long as the system is healthy and quiet — and the drain shape this lane
// wants is a periodic sweep, not a wait.
func (l *ObservabilityLane) TryReceive() (Envelope, bool) {
	select {
	case e := <-l.q:
		return e, true
	default:
		return Envelope{}, false
	}
}

// Close stops the lane accepting new envelopes. Idempotent, and a send
// afterwards drops and counts rather than panicking — shutdown ordering is not
// something a metrics emitter should have to know about.
//
// Queued envelopes stay readable via TryReceive, so a final flush is possible.
// Nothing requires one: §3.4 asks for the CONTROL lane to be drained before
// exit, and pointedly not this one.
func (l *ObservabilityLane) Close() {
	l.closeOnce.Do(func() { close(l.closed) })
}

// Dropped reports how many envelopes this lane has refused, cumulatively.
//
// The number a "we are flying blind" alarm watches. Never reset: a rate is the
// caller's to derive, and a counter that resets cannot answer "did we lose
// anything during that incident" after the fact.
func (l *ObservabilityLane) Dropped() uint64 { return l.dropped.Load() }

// Len reports the current backlog. Point-in-time, for metrics and tests.
func (l *ObservabilityLane) Len() int { return len(l.q) }
