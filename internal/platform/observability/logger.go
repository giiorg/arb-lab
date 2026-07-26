package observability

import (
	"context"
	"log/slog"
	"runtime"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// Logger is §10's structured-log emitter: log/slog, with its timestamp taken
// from the injected clock.
//
// # The clock is the entire reason this type exists
//
// slog.Logger.Info and its siblings call time.Now() internally when they build
// the Record — it is not reachable through a Handler, an Attr, or any option.
// That is an ambient clock read, and it sits on the far side of build-spec
// §3.5's parity seam: a backtest replays a TestClock through 2024, and every log
// line it emits is stamped with today's wall clock. The log then cannot be
// joined to the replayed dispatch it describes, which is the one thing a replay
// log is for.
//
// Note what does NOT catch this. §3.6 rule 5's notimenow gate polices domain/,
// application/ and engine/ — platform/ is deliberately outside it, because
// platform/clock is where the ambient read has to live. So the kernel is exactly
// the region where an ambient clock read is invisible to the gate, and the only
// defence is that the code does not make one. This type is that defence: it
// builds slog.Record itself, from clock.Now.
//
// # Everything else is deliberately thin
//
// It wraps a slog.Handler and adds nothing to it. Formatting, filtering,
// destination and encoding are the handler's, so the standard library's
// JSONHandler and TextHandler are usable unchanged, and so is any third-party
// one — the seam this package owns is the clock, not the log format.
type Logger struct {
	clock   clock.Clock
	handler slog.Handler
}

// NewLogger returns a logger emitting through h and stamping through c.
func NewLogger(c clock.Clock, h slog.Handler) (*Logger, error) {
	if c == nil {
		return nil, ErrClockNil
	}
	if h == nil {
		return nil, ErrHandlerNil
	}
	return &Logger{clock: c, handler: h}, nil
}

// Handler returns the underlying handler.
func (l *Logger) Handler() slog.Handler { return l.handler }

// With returns a logger that adds attrs to every record.
func (l *Logger) With(attrs ...slog.Attr) *Logger {
	if len(attrs) == 0 {
		return l
	}
	return &Logger{clock: l.clock, handler: l.handler.WithAttrs(attrs)}
}

// WithGroup returns a logger nesting subsequent attrs under name.
func (l *Logger) WithGroup(name string) *Logger {
	if name == "" {
		return l
	}
	return &Logger{clock: l.clock, handler: l.handler.WithGroup(name)}
}

// Enabled reports whether a record at this level would be handled.
func (l *Logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.handler.Enabled(ctx, level)
}

// Debug, Info, Warn and Error emit at the four standard levels.
func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelDebug, msg, attrs)
}

func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelInfo, msg, attrs)
}

func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelWarn, msg, attrs)
}

func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelError, msg, attrs)
}

// Log emits at an arbitrary level, for a caller mapping some other severity
// scale onto slog's.
func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.log(ctx, level, msg, attrs)
}

// log builds and dispatches the record.
//
// The returned error from Handle is deliberately discarded. This is §3.4's
// lossy lane: a caller that had to check whether its log line landed would be
// doing error handling on the trading path in response to an observability
// failure, which is the coupling the lane exists to forbid. What was lost is
// counted by the handler — BufferedHandler.Dropped — rather than reported to the
// emitter.
func (l *Logger) log(ctx context.Context, level slog.Level, msg string, attrs []slog.Attr) {
	if !l.handler.Enabled(ctx, level) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Skip: runtime.Callers, this function, the exported wrapper. Captured so a
	// handler configured with AddSource names the emitting line rather than
	// naming this file for every record in the system.
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	// The line this whole type exists for: clock.Now, never time.Now.
	r := slog.NewRecord(l.clock.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.handler.Handle(ctx, r)
}
