package observability_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// backtestEpoch is deliberately far from any wall clock, so a record stamped by
// time.Now() cannot accidentally match one stamped by the injected clock.
var backtestEpoch = time.Date(2024, 3, 14, 9, 30, 0, 0, time.UTC)

// TestLoggerStampsFromTheInjectedClock is the finding this type exists for.
//
// slog.Logger.Info calls time.Now() internally when it builds the Record — not
// reachable through a Handler, an Attr or any option. §3.6 rule 5's notimenow
// gate polices domain/, application/ and engine/, NOT platform/, so the kernel
// is exactly where such a read is invisible to the gate. A backtest replaying
// 2024 would emit log lines stamped today, and the log could not be joined to
// the replayed dispatch it describes.
func TestLoggerStampsFromTheInjectedClock(t *testing.T) {
	t.Parallel()

	c, h, log := newTestLogger(t)
	log.Info(context.Background(), "entered")

	got := drainOne(t, h)
	if !got.Time.Equal(backtestEpoch) {
		t.Fatalf("record time = %v; want the injected clock's %v", got.Time, backtestEpoch)
	}

	// Advance the clock and confirm the stamp FOLLOWS it. A logger that read
	// the clock once at construction would pass the assertion above and still
	// stamp every subsequent line identically.
	if err := c.Advance(90 * time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	log.Info(context.Background(), "settled")

	got = drainOne(t, h)
	if want := backtestEpoch.Add(90 * time.Second); !got.Time.Equal(want) {
		t.Fatalf("record time = %v; want %v — the clock is read per record", got.Time, want)
	}

	// And the stamp is nowhere near the wall clock, which is the failure in its
	// observable form.
	if time.Since(got.Time) < 24*time.Hour {
		t.Errorf("record time %v is close to now; a wall-clock read would look like this", got.Time)
	}
}

func TestLoggerRefusesMissingCollaborators(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(4, nil)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	c := newTestClock(t)

	if _, err := observability.NewLogger(nil, h); !errors.Is(err, observability.ErrClockNil) {
		t.Errorf("err = %v; want ErrClockNil", err)
	}
	if _, err := observability.NewLogger(c, nil); !errors.Is(err, observability.ErrHandlerNil) {
		t.Errorf("err = %v; want ErrHandlerNil", err)
	}
	// Named separately so the failure says which wire is missing. Positive
	// control that both together succeed.
	if _, err := observability.NewLogger(c, h); err != nil {
		t.Errorf("NewLogger with both collaborators: %v", err)
	}
}

func TestLoggerLevels(t *testing.T) {
	t.Parallel()

	h, err := observability.NewBufferedHandler(16, slog.LevelWarn)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	log, err := observability.NewLogger(newTestClock(t), h)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	ctx := context.Background()
	log.Debug(ctx, "debug")
	log.Info(ctx, "info")
	log.Warn(ctx, "warn")
	log.Error(ctx, "error")
	log.Log(ctx, slog.LevelError+4, "fatal-ish")

	got := h.Drain()
	if len(got) != 3 {
		t.Fatalf("got %d records; want 3 (warn and above)", len(got))
	}
	for i, want := range []string{"warn", "error", "fatal-ish"} {
		if got[i].Message != want {
			t.Errorf("record %d = %q; want %q", i, got[i].Message, want)
		}
	}

	if log.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(Info) = true at a Warn threshold")
	}
	if !log.Enabled(ctx, slog.LevelWarn) {
		t.Error("Enabled(Warn) = false at a Warn threshold")
	}
}

func TestLoggerCapturesTheCallSite(t *testing.T) {
	t.Parallel()

	_, h, log := newTestLogger(t)
	log.Info(context.Background(), "here")

	got := drainOne(t, h)
	if got.PC == 0 {
		t.Fatal("PC = 0; a handler with AddSource would name nothing")
	}
	// It must name THIS file, not logger.go — the whole point of the frame skip
	// is that every record would otherwise be attributed to the wrapper.
	frame, _ := runtimeFrame(got.PC)
	if frame == "" {
		t.Fatal("could not resolve the captured PC")
	}
	if !hasSuffix(frame, "logger_test.go") {
		t.Errorf("PC resolved to %q; want this test file, not the logger wrapper", frame)
	}
}

func TestLoggerWithAndWithGroup(t *testing.T) {
	t.Parallel()

	_, h, log := newTestLogger(t)

	// Two levels of nesting, because a single level cannot distinguish "groups
	// nest" from "the last group wins".
	log.With(slog.String("component", "oms")).
		WithGroup("order").
		With(slog.String("id", "abc")).
		WithGroup("venue").
		Info(context.Background(), "submitted", slog.String("name", "binance"))

	got := drainOne(t, h)
	if want := "component=oms order.id=abc order.venue.name=binance"; renderAttrs(got) != want {
		t.Errorf("attrs = %q; want %q", renderAttrs(got), want)
	}

	// The identity cases: neither allocates a derivation.
	if log.With() != log {
		t.Error("With() with no attrs returned a new logger")
	}
	if log.WithGroup("") != log {
		t.Error(`WithGroup("") returned a new logger`)
	}
}

// TestLoggerDerivationsDoNotAliasEachOther is the classic aliased-append defect,
// whose symptom is one component's log lines carrying another's fields.
func TestLoggerDerivationsDoNotAliasEachOther(t *testing.T) {
	t.Parallel()

	_, h, log := newTestLogger(t)

	base := log.With(slog.String("shared", "yes"))
	a := base.With(slog.String("who", "a"))
	b := base.With(slog.String("who", "b"))

	ctx := context.Background()
	a.Info(ctx, "a")
	b.Info(ctx, "b")
	base.Info(ctx, "base")

	got := h.Drain()
	if len(got) != 3 {
		t.Fatalf("got %d records; want 3", len(got))
	}
	want := []string{"shared=yes who=a", "shared=yes who=b", "shared=yes"}
	for i := range want {
		if renderAttrs(got[i]) != want[i] {
			t.Errorf("record %d attrs = %q; want %q", i, renderAttrs(got[i]), want[i])
		}
	}
}

func newTestClock(t *testing.T) *clock.TestClock {
	t.Helper()

	c, err := clock.NewTestClock(backtestEpoch, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

func newTestLogger(t *testing.T) (*clock.TestClock, *observability.BufferedHandler, *observability.Logger) {
	t.Helper()

	c := newTestClock(t)
	h, err := observability.NewBufferedHandler(16, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewBufferedHandler: %v", err)
	}
	log, err := observability.NewLogger(c, h)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	return c, h, log
}

func drainOne(t *testing.T, h *observability.BufferedHandler) slog.Record {
	t.Helper()

	got := h.Drain()
	if len(got) != 1 {
		t.Fatalf("got %d records; want exactly 1", len(got))
	}
	return got[0]
}
