package events_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
)

// The observability lane exists to be the one place back-pressure is not
// allowed to reach. §3.4: "Metrics/logs/traces; back-pressure here never blocks
// trading."
//
// That makes it the mirror image of the control lane, and the tests are
// deliberately written as the mirror image too — same situation, opposite
// required behaviour — because the failure being guarded against is somebody
// later "fixing" the drops by making Send wait. A metrics exporter that stalls
// must cost metrics, never fills.

// mustNotBlock runs fn and fails the test if it has not finished within d.
//
// Every test in this file fills a lane past its capacity, which is precisely
// what a broken (blocking) Send would park on — so without a bound the suite
// would HANG rather than fail. §5e is explicit about why that is unacceptable: a
// failing test names the defect, a hanging one reports nothing, blocks CI and
// reads as an infrastructure problem. Found the hard way, by a mutation that
// made Send block and wedged the whole mutation run.
//
// fn keeps running in its goroutine after a timeout — there is no way to
// interrupt a blocked channel send — so this fails fatally rather than letting
// a leaked goroutine touch t afterwards.
func mustNotBlock(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not finish within %v; the observability lane must never block", what, d)
	}
}

func TestNewObservabilityLaneRejectsBadCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		if _, err := events.NewObservabilityLane(capacity); !errors.Is(err, events.ErrLaneCapacity) {
			t.Errorf("NewObservabilityLane(%d) error = %v, want ErrLaneCapacity", capacity, err)
		}
	}
}

// TestObservabilityLaneDropsRatherThanBlocks is the guarantee, stated as the
// exact inverse of TestControlLaneBlocksWhenFull.
func TestObservabilityLaneDropsRatherThanBlocks(t *testing.T) {
	l, err := events.NewObservabilityLane(1)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(3)

	if ok := l.Send(envs[0]); !ok {
		t.Fatal("first send into an empty lane was dropped")
	}

	// The lane is now full. Both further sends must return immediately, and
	// must say they dropped rather than reporting a success that did not
	// happen — a metric silently going missing is how a dashboard lies.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, e := range envs[1:] {
			if ok := l.Send(e); ok {
				t.Error("Send into a full observability lane reported success; it must drop")
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Send blocked on a full observability lane; back-pressure here must never block trading")
	}

	if got := l.Dropped(); got != 2 {
		t.Errorf("Dropped() = %d, want 2", got)
	}
}

// TestObservabilityLaneDropsNewest is the direction, and it is the opposite of
// the market-data lane's.
//
// There, the newest update is closest to the truth, so the oldest is dropped
// here. On this lane the entries are independent log lines and traces — none
// supersedes another — so the cheap, allocation-free choice is to refuse the
// newcomer. Stated as a test because "drop" alone does not say which, and a
// later change to drop-oldest would silently make an already-emitted metric
// disappear after the fact.
func TestObservabilityLaneDropsNewest(t *testing.T) {
	l, err := events.NewObservabilityLane(2)
	if err != nil {
		t.Fatal(err)
	}
	envs := seq(4)
	mustNotBlock(t, 2*time.Second, "filling past capacity", func() {
		for _, e := range envs {
			l.Send(e)
		}
	})

	for _, want := range []int{0, 1} {
		got, ok := l.TryReceive()
		if !ok {
			t.Fatalf("lane emptied early, wanted payload %d", want)
		}
		if got.Payload != want {
			t.Errorf("got payload %v, want %d — a full lane refuses the newcomer", got.Payload, want)
		}
	}
	if _, ok := l.TryReceive(); ok {
		t.Error("lane held more than its capacity")
	}
}

// TestObservabilityLaneDeliversWhenItCan is the positive control.
//
// Without it, a Send hard-wired to `return false` would pass every test above:
// nothing blocks, everything drops, and the drop counter is perfectly accurate.
// A universally-objecting predicate reads as a perfect score.
func TestObservabilityLaneDeliversWhenItCan(t *testing.T) {
	l, err := events.NewObservabilityLane(8)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range seq(8) {
		if ok := l.Send(e); !ok {
			t.Fatalf("Send into a lane with room reported a drop")
		}
	}
	if got := l.Dropped(); got != 0 {
		t.Errorf("Dropped() = %d with room to spare, want 0", got)
	}
	for i := range 8 {
		got, ok := l.TryReceive()
		if !ok {
			t.Fatalf("nothing at position %d", i)
		}
		if got.Payload != i {
			t.Errorf("position %d = %v, want %d", i, got.Payload, i)
		}
	}
}

// TestObservabilityLaneCloseIsIdempotent, and a closed lane drops rather than
// panicking — shutdown ordering is not something a metrics emitter should have
// to know about.
func TestObservabilityLaneCloseIsIdempotent(t *testing.T) {
	l, err := events.NewObservabilityLane(4)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	l.Close()

	if ok := l.Send(seq(1)[0]); ok {
		t.Error("Send on a closed lane reported success")
	}
	if got := l.Dropped(); got != 1 {
		t.Errorf("Dropped() = %d after a send to a closed lane, want 1", got)
	}
}

// TestObservabilityLaneUnderContention runs with -race, and pins that the drop
// counter is exact rather than approximately right.
//
// Exact matters: the counter is the only evidence that anything was lost, so a
// racy increment turns "we dropped 4,000 traces" into a number nobody can act
// on. Every send either lands or is counted, and the two must add up.
func TestObservabilityLaneUnderContention(t *testing.T) {
	const (
		producers   = 8
		perProducer = 500
		capacity    = 16
	)
	l, err := events.NewObservabilityLane(capacity)
	if err != nil {
		t.Fatal(err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted int
	)
	for range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := 0
			for _, e := range seq(perProducer) {
				if l.Send(e) {
					local++
				}
			}
			mu.Lock()
			accepted += local
			mu.Unlock()
		}()
	}
	// Bounded for the same reason as everywhere else in this file: nothing here
	// drains the lane, so every producer runs straight into a full one, and a
	// blocking Send would deadlock all eight rather than fail.
	mustNotBlock(t, 10*time.Second, "concurrent producers", wg.Wait)

	total := producers * perProducer
	if got := accepted + int(l.Dropped()); got != total {
		t.Errorf("accepted %d + dropped %d = %d, want %d — every send must be either delivered or counted",
			accepted, l.Dropped(), got, total)
	}
	// The capacity bound is what makes this lane isolated at all; if it grew,
	// a stalled exporter would be an unbounded memory leak in the trading
	// process.
	if l.Len() > capacity {
		t.Errorf("Len() = %d, want at most the capacity %d", l.Len(), capacity)
	}
}
