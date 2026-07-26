package events_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
)

// The control lane has exactly one guarantee — nothing is ever dropped — and
// three ways to break it that all look like working code:
//
//  1. Dropping on overflow. Caught by the back-pressure tests.
//  2. Reordering. §3.5 makes the consumed order the replay key, so a lane that
//     reordered would replay a sequence the state machine never saw. Caught by
//     TestControlLanePreservesOrder and, specifically, by the Prio test — a lane
//     that "helpfully" sorted by priority would break FIFO within a SourceID
//     where VenueSeq is authoritative.
//  3. Discarding the backlog at shutdown. §3.4 requires the lane to be DRAINED
//     before exit, so Close must mean "no more input", never "throw away what is
//     queued". Caught by the shutdown tests.
//
// Everything below is a variation on one of those three.

// seq builds n distinct envelopes, numbered so an order check can name the one
// that went wrong.
func seq(n int) []events.Envelope {
	out := make([]events.Envelope, n)
	for i := range out {
		e := anEnvelope()
		e.AcceptOrdinal = uint64(i + 1)
		e.IngressSeq = uint64(i + 1)
		e.Payload = i
		out[i] = e
	}
	return out
}

func TestNewControlLaneRejectsBadCapacity(t *testing.T) {
	// A zero-capacity lane is a rendezvous, not a queue: every send would wait
	// for a receiver to be already blocked, so a single-goroutine boot sequence
	// deadlocks on its first event.
	for _, capacity := range []int{0, -1} {
		if _, err := events.NewControlLane(capacity); !errors.Is(err, events.ErrLaneCapacity) {
			t.Errorf("NewControlLane(%d) error = %v, want ErrLaneCapacity", capacity, err)
		}
	}
	if _, err := events.NewControlLane(1); err != nil {
		t.Errorf("NewControlLane(1) = %v, want nil", err)
	}
}

// --- guarantee 1: nothing is dropped -----------------------------------------

// TestControlLaneBlocksWhenFull is the delivery contract stated directly.
//
// It is deliberately the opposite of the observability lane's test: there, a
// send into a full lane must return immediately having dropped. Here it must not
// return at all. Back-pressure reaching the socket reader is the DESIGNED
// behaviour — a visible, measurable stall rather than a silent gap in execution
// truth.
func TestControlLaneBlocksWhenFull(t *testing.T) {
	l, err := events.NewControlLane(1)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(2)
	if err := l.Send(t.Context(), envs[0]); err != nil {
		t.Fatalf("first send: %v", err)
	}

	// The bound is what makes this a test rather than a hang. A predicate that
	// can hang on a broken implementation reports nothing and reads as an
	// infrastructure problem — the §5e lesson.
	sent := make(chan error, 1)
	go func() { sent <- l.Send(t.Context(), envs[1]) }()

	select {
	case err := <-sent:
		t.Fatalf("Send into a full control lane returned %v; it must block, never drop", err)
	case <-time.After(50 * time.Millisecond):
	}

	// Draining one slot must release the blocked sender, or "blocks" would mean
	// "deadlocks".
	if _, err := l.Receive(t.Context()); err != nil {
		t.Fatalf("receive: %v", err)
	}
	select {
	case err := <-sent:
		if err != nil {
			t.Errorf("released send returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send stayed blocked after a slot was freed")
	}
}

// TestControlLaneSendRespectsContext is the escape hatch that keeps "blocks
// forever" from being literally true.
//
// Without it a wedged consumer wedges shutdown too: every producer is parked in
// Send, holding whatever it was reading from, and the process cannot exit
// without being killed — which loses the very backlog the lane exists to
// preserve.
func TestControlLaneSendRespectsContext(t *testing.T) {
	l, err := events.NewControlLane(1)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(2)
	if err := l.Send(t.Context(), envs[0]); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	sent := make(chan error, 1)
	go func() { sent <- l.Send(ctx, envs[1]) }()
	time.Sleep(20 * time.Millisecond) // let it park
	cancel()

	select {
	case err := <-sent:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled Send = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the context did not release the blocked sender")
	}
}

// --- guarantee 2: order is preserved -----------------------------------------

func TestControlLanePreservesOrder(t *testing.T) {
	const n = 64
	l, err := events.NewControlLane(n)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(n)
	for _, e := range envs {
		if err := l.Send(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	for i := range n {
		got, err := l.Receive(t.Context())
		if err != nil {
			t.Fatalf("receive %d: %v", i, err)
		}
		if got.Payload != i {
			t.Fatalf("position %d holds payload %v, want %d", i, got.Payload, i)
		}
	}
}

// TestControlLaneDoesNotSortByPrio is the reordering trap named explicitly.
//
// Envelope carries a Prio, and a lane feeding "the single-writer OMS in a
// documented total order" looks like it ought to use it. It must not. §3.5's
// total order is the DISPATCH order (IngressSeq) — Prio is an input to the
// dispatcher's arbitration and to the timer queue, applied before an envelope
// reaches this lane. Sorting here would reorder envelopes WITHIN a SourceID,
// where VenueSeq is authoritative, and a replay would then apply a book delta
// before the snapshot it depends on.
func TestControlLaneDoesNotSortByPrio(t *testing.T) {
	l, err := events.NewControlLane(4)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(2)
	envs[0].Prio = events.PrioNormal // sent first, less urgent
	envs[1].Prio = events.PrioSafety // sent second, most urgent

	for _, e := range envs {
		if err := l.Send(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}

	first, err := l.Receive(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first.Payload != 0 {
		t.Errorf("the urgent envelope overtook the one sent before it; the control lane is FIFO, and Prio is the dispatcher's business")
	}
}

// --- guarantee 3: shutdown drains, never discards ----------------------------

// TestControlLaneCloseStopsSendsButNotReceives is §3.4's "shutdown drains the
// control lane before exit", as a test.
func TestControlLaneCloseStopsSendsButNotReceives(t *testing.T) {
	l, err := events.NewControlLane(4)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(3)
	for _, e := range envs {
		if err := l.Send(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()

	if err := l.Send(t.Context(), envs[0]); !errors.Is(err, events.ErrLaneClosed) {
		t.Errorf("Send after Close = %v, want ErrLaneClosed", err)
	}

	// The three already accepted must still come out, in order. A Close that
	// discarded them would lose execution truth at exactly the moment the
	// journal is being closed out.
	for i := range 3 {
		got, err := l.Receive(t.Context())
		if err != nil {
			t.Fatalf("receive %d after Close: %v, want the queued envelope", i, err)
		}
		if got.Payload != i {
			t.Fatalf("position %d holds %v, want %d", i, got.Payload, i)
		}
	}
	if _, err := l.Receive(t.Context()); !errors.Is(err, events.ErrLaneClosed) {
		t.Errorf("Receive on a drained closed lane = %v, want ErrLaneClosed", err)
	}
}

// TestControlLaneReceivePrefersQueuedOverClosed pins the determinism the drain
// depends on.
//
// Both the queue and the closed signal are ready at once here, and Go's select
// picks a ready case at RANDOM. Left to the select, a shutdown would drain a
// random prefix of the backlog and report the lane empty — reproducible in
// neither direction and green most of the time. The implementation must check
// the queue first, non-blockingly, before it considers the closed signal.
//
// Repeated because a one-shot check of a random choice passes half the time.
func TestControlLaneReceivePrefersQueuedOverClosed(t *testing.T) {
	for trial := range 200 {
		l, err := events.NewControlLane(1)
		if err != nil {
			t.Fatal(err)
		}
		e := seq(1)[0]
		if err := l.Send(t.Context(), e); err != nil {
			t.Fatal(err)
		}
		l.Close()

		got, err := l.Receive(t.Context())
		if err != nil {
			t.Fatalf("trial %d: Receive = %v, want the queued envelope; a closed lane must still drain", trial, err)
		}
		if got.Payload != 0 {
			t.Fatalf("trial %d: got payload %v", trial, got.Payload)
		}
	}
}

// TestControlLaneSendAfterCloseIsConsistentlyRefused is the mirror of
// TestControlLaneReceivePrefersQueuedOverClosed, and it exists because the
// single-shot version of this check above passes half the time.
//
// The situation is the same one: with room in the queue AND the lane closed,
// both cases of a select are ready and Go picks at random. A Send that only
// consulted the closed signal inside that select would accept post-close
// envelopes roughly every other call — the kind of defect that passes CI, passes
// review, and shows up as an event journaled after the shutdown record.
//
// Found by a surviving mutant: replacing Send's non-blocking pre-check with a
// channel that never closes left the single-shot test green.
func TestControlLaneSendAfterCloseIsConsistentlyRefused(t *testing.T) {
	for trial := range 200 {
		// Capacity deliberately larger than the backlog, so the send WOULD
		// succeed if the closed signal were merely one racing option.
		l, err := events.NewControlLane(4)
		if err != nil {
			t.Fatal(err)
		}
		l.Close()

		if err := l.Send(t.Context(), seq(1)[0]); !errors.Is(err, events.ErrLaneClosed) {
			t.Fatalf("trial %d: Send after Close = %v, want ErrLaneClosed every time, not most of the time", trial, err)
		}
		if got := l.Len(); got != 0 {
			t.Fatalf("trial %d: a refused Send enqueued anyway (Len = %d)", trial, got)
		}
	}
}

// TestControlLaneCloseReleasesBlockedSender covers the wedge Close exists to
// break: a producer parked on a full lane while shutdown begins.
func TestControlLaneCloseReleasesBlockedSender(t *testing.T) {
	l, err := events.NewControlLane(1)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(2)
	if err := l.Send(t.Context(), envs[0]); err != nil {
		t.Fatal(err)
	}

	sent := make(chan error, 1)
	go func() { sent <- l.Send(t.Context(), envs[1]) }()
	time.Sleep(20 * time.Millisecond)
	l.Close()

	select {
	case err := <-sent:
		if !errors.Is(err, events.ErrLaneClosed) {
			t.Errorf("blocked Send released with %v, want ErrLaneClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not release the blocked sender")
	}
}

// TestControlLaneCloseIsIdempotent — shutdown paths run twice more often than
// anyone plans for, and closing a Go channel twice panics.
func TestControlLaneCloseIsIdempotent(t *testing.T) {
	l, err := events.NewControlLane(1)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	l.Close()
	l.Close()
}

// --- the property that ties them together ------------------------------------

// TestControlLaneLosesNothingUnderContention is the guarantee end to end, with
// the race detector on (make test runs -race).
//
// Many producers, one consumer, a lane far too small to hold the traffic: every
// envelope sent must be received exactly once. A drop, a duplicate or a data
// race all fail it.
func TestControlLaneLosesNothingUnderContention(t *testing.T) {
	const (
		producers   = 8
		perProducer = 250
		capacity    = 4 // deliberately tiny: force the blocking path constantly
	)
	l, err := events.NewControlLane(capacity)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perProducer {
				e := anEnvelope()
				e.Payload = [2]int{p, i}
				if err := l.Send(t.Context(), e); err != nil {
					t.Errorf("producer %d: %v", p, err)
					return
				}
			}
		}()
	}

	got := make(map[[2]int]int)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range producers * perProducer {
			e, err := l.Receive(t.Context())
			if err != nil {
				t.Errorf("receive: %v", err)
				return
			}
			got[e.Payload.([2]int)]++
		}
	}()

	wg.Wait()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not receive every envelope")
	}

	if len(got) != producers*perProducer {
		t.Errorf("received %d distinct envelopes, want %d", len(got), producers*perProducer)
	}
	for k, n := range got {
		if n != 1 {
			t.Errorf("envelope %v delivered %d times, want exactly 1", k, n)
		}
	}
}

// TestControlLaneLen reports the backlog, which is what a slow-consumer alarm
// watches. It is a point-in-time reading and says so.
func TestControlLaneLen(t *testing.T) {
	l, err := events.NewControlLane(4)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Len(); got != 0 {
		t.Errorf("empty lane Len() = %d, want 0", got)
	}
	for _, e := range seq(3) {
		if err := l.Send(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	if got := l.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if _, err := l.Receive(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := l.Len(); got != 2 {
		t.Errorf("Len() after one receive = %d, want 2", got)
	}
}
