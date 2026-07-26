package observability_test

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/observability"
)

func TestBufferedHandlerRefusesNonPositiveCapacity(t *testing.T) {
	t.Parallel()

	for _, capacity := range []int{0, -1} {
		if _, err := observability.NewBufferedHandler(capacity, nil); !errors.Is(err, observability.ErrCapacityInvalid) {
			t.Errorf("NewBufferedHandler(%d) err = %v; want ErrCapacityInvalid", capacity, err)
		}
	}
	// A zero-capacity buffer would drop every record while reporting itself as
	// a working handler — no logs and no signal that there are none. Positive
	// control that a positive capacity is accepted.
	if _, err := observability.NewBufferedHandler(1, nil); err != nil {
		t.Errorf("NewBufferedHandler(1) = %v; want nil", err)
	}
}

// TestBufferedHandlerDropsTheNewest uses a capacity of 3, not 1.
//
// At capacity 1 "drop the newest" and "drop the oldest" are the same slot, so
// the distinction the code makes is unreachable and a suite pinned to that
// depth covers it while proving nothing — the identical fixture defect the
// events coalescing tests already paid for once.
func TestBufferedHandlerDropsTheNewest(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(3, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	for _, msg := range []string{"a", "b", "c", "d", "e"} {
		if err := h.Handle(context.Background(), newRecord(msg)); err != nil {
			t.Fatalf("Handle(%s) = %v; this handler never errors", msg, err)
		}
	}

	if got := h.Dropped(); got != 2 {
		t.Errorf("Dropped() = %d; want 2", got)
	}
	got := h.Drain()
	if len(got) != 3 {
		t.Fatalf("drained %d records; want 3", len(got))
	}
	// The first three survive. Had the handler dropped the oldest, this would
	// read c, d, e.
	for i, want := range []string{"a", "b", "c"} {
		if got[i].Message != want {
			t.Errorf("record %d = %q; want %q — the handler dropped the wrong end", i, got[i].Message, want)
		}
	}
}

func TestBufferedHandlerDrainEmptiesAndReopens(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(2, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	ctx := context.Background()

	mustNil(t, h.Handle(ctx, newRecord("a")))
	mustNil(t, h.Handle(ctx, newRecord("b")))
	mustNil(t, h.Handle(ctx, newRecord("c"))) // dropped

	if h.Len() != 2 {
		t.Errorf("Len() = %d; want 2", h.Len())
	}
	if len(h.Drain()) != 2 {
		t.Fatal("first Drain did not return the buffered records")
	}
	if h.Len() != 0 {
		t.Errorf("Len() after Drain = %d; want 0", h.Len())
	}
	if got := h.Drain(); len(got) != 0 {
		t.Errorf("second Drain returned %d records; want none", len(got))
	}

	// Capacity is restored, so a periodic sweep actually keeps the lane open.
	// Without this the handler would drop everything after its first fill and
	// still pass every assertion above.
	mustNil(t, h.Handle(ctx, newRecord("d")))
	if got := h.Drain(); len(got) != 1 || got[0].Message != "d" {
		t.Errorf("after draining, got %v; want the new record accepted", got)
	}

	// The drop counter survives the drain: a rate is the caller's to derive,
	// and a counter that resets cannot answer "did we lose anything during that
	// incident" afterwards.
	if got := h.Dropped(); got != 1 {
		t.Errorf("Dropped() = %d after drains; want the count retained at 1", got)
	}
}

// TestBufferedHandlerRetainsAttrsPastHandle checks that a buffered record keeps
// its own attrs when the caller goes on using the record it passed in.
//
// It deliberately does NOT claim to check the Clone. Dropping the Clone is an
// equivalent mutant for this package's callers — the only mutation available to
// a caller is AddAttrs, which appends, and an append never overwrites an element
// the retained copy can already see. The reasoning is recorded at the Clone
// itself; asserting it here would be a test that passes for a reason other than
// the one it names.
func TestBufferedHandlerRetainsAttrsPastHandle(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(4, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}

	r := newRecord("reused")
	// Past the fifth attr, slog spills into a heap-allocated array — the region
	// Clone exists for. Five or fewer live in the record's inline array, where
	// a struct copy is already a deep copy and the question cannot arise.
	for i := range 8 {
		r.AddAttrs(slog.Int("k", i))
	}
	mustNil(t, h.Handle(context.Background(), r))

	// The caller keeps using its record after Handle returned.
	r.AddAttrs(slog.String("late", "x"))

	got := drainOne(t, h)
	if n := got.NumAttrs(); n != 8 {
		t.Errorf("buffered record has %d attrs; want the 8 it was handed", n)
	}
	if strings.Contains(renderAttrs(got), "late") {
		t.Error("the buffered record picked up an attr added after Handle returned")
	}
}

// TestDrainReturnsRecordsTheHandlerNoLongerOwns is the aliasing failure that
// matters, and it is the one a same-instant assertion cannot see.
//
// Drain hands its slice to whoever owns the flush cadence — an encoder, a file
// writer — and the trading goroutines keep logging while that runs. If Drain
// reset the buffer by reslicing (buf = buf[:0]) instead of allocating, the next
// Handle would write straight into the record the encoder is still reading.
func TestDrainReturnsRecordsTheHandlerNoLongerOwns(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(4, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	ctx := context.Background()

	mustNil(t, h.Handle(ctx, newRecord("a")))
	mustNil(t, h.Handle(ctx, newRecord("b")))
	drained := h.Drain()

	// The sweep is still holding `drained` when the next records arrive.
	mustNil(t, h.Handle(ctx, newRecord("c")))
	mustNil(t, h.Handle(ctx, newRecord("d")))

	for i, want := range []string{"a", "b"} {
		if drained[i].Message != want {
			t.Errorf("drained[%d] = %q; want %q — later records overwrote a drained one",
				i, drained[i].Message, want)
		}
	}
}

// TestDerivedHandlersShareTheBuffer checks the property that makes the capacity
// bound process-wide. A per-derivation buffer would split the bound across every
// component that called With, and Dropped would report one derivation's losses
// while another's went unnoticed.
func TestDerivedHandlersShareTheBuffer(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(2, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	a := h.WithAttrs([]slog.Attr{slog.String("who", "a")})
	b := h.WithGroup("g").WithAttrs([]slog.Attr{slog.String("who", "b")})

	ctx := context.Background()
	mustNil(t, a.Handle(ctx, newRecord("1")))
	mustNil(t, b.Handle(ctx, newRecord("2")))
	// The third fills nothing: the shared buffer is already at capacity.
	mustNil(t, a.Handle(ctx, newRecord("3")))

	if got := h.Dropped(); got != 1 {
		t.Errorf("Dropped() = %d; want 1 counted on the shared sink", got)
	}
	got := h.Drain()
	if len(got) != 2 {
		t.Fatalf("drained %d; want 2 from the shared buffer", len(got))
	}
	if attrs := renderAttrs(got[0]); attrs != "who=a" {
		t.Errorf("first record attrs = %q; want who=a", attrs)
	}
	if attrs := renderAttrs(got[1]); attrs != "g.who=b" {
		t.Errorf("second record attrs = %q; want the group applied", attrs)
	}

	// The no-op derivations return the receiver rather than a new view.
	if h.WithAttrs(nil) != slog.Handler(h) {
		t.Error("WithAttrs(nil) returned a new handler")
	}
	if h.WithGroup("") != slog.Handler(h) {
		t.Error(`WithGroup("") returned a new handler`)
	}
}

// TestBufferedHandlerLosesNothingUnaccounted is the arithmetic that makes
// Dropped trustworthy: every record either landed or was counted, and the two
// add up. A racy counter turns "we dropped four thousand lines" into a number
// nobody can act on.
func TestBufferedHandlerLosesNothingUnaccounted(t *testing.T) {
	t.Parallel()

	const goroutines, each, capacity = 8, 100, 64

	h, err := observability.NewBufferedHandler(capacity, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted int
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range each {
				mustNilConcurrent(t, h.Handle(context.Background(), newRecord("x")))
				// Draining concurrently with sending is the interesting shape:
				// the periodic sweep this handler is designed around runs
				// while the trading goroutines are logging.
				n := len(h.Drain())
				mu.Lock()
				accepted += n
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	accepted += len(h.Drain())
	mu.Unlock()

	if total := accepted + int(h.Dropped()); total != goroutines*each {
		t.Errorf("accepted %d + dropped %d = %d; want %d — records went unaccounted",
			accepted, h.Dropped(), total, goroutines*each)
	}
}

func TestBufferedHandlerDefaultLevelIsInfo(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(4, nil)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelDebug) {
		t.Error("Enabled(Debug) = true under the nil-level default")
	}
	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(Info) = false under the nil-level default")
	}
}

func newRecord(msg string) slog.Record {
	// Time and PC are zero: this file tests the handler, and the clock is
	// Logger's concern.
	return slog.NewRecord(backtestEpoch, slog.LevelInfo, msg, 0)
}

// renderAttrs flattens a record's attrs into a stable, group-qualified string.
// Groups render dotted, so "the group was applied" and "the group was flattened
// away" are distinguishable — which a bare key list would not be.
func renderAttrs(r slog.Record) string {
	var parts []string
	var walk func(prefix string, a slog.Attr)
	walk = func(prefix string, a slog.Attr) {
		if a.Value.Kind() == slog.KindGroup {
			for _, g := range a.Value.Group() {
				walk(prefix+a.Key+".", g)
			}
			return
		}
		parts = append(parts, prefix+a.Key+"="+a.Value.String())
	}
	r.Attrs(func(a slog.Attr) bool {
		walk("", a)
		return true
	})
	return strings.Join(parts, " ")
}

func runtimeFrame(pc uintptr) (string, int) {
	frames := runtime.CallersFrames([]uintptr{pc})
	f, _ := frames.Next()
	return f.File, f.Line
}

func hasSuffix(s, suffix string) bool { return strings.HasSuffix(s, suffix) }

// mustNilConcurrent is mustNil without t.Fatalf, which may only be called from
// the goroutine running the test.
func mustNilConcurrent(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
