package observability

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// BufferedHandler is a slog.Handler that never blocks the emitting goroutine on
// I/O, dropping records instead once its buffer is full.
//
// # It is the deliberate mirror of events.ObservabilityLane
//
// Same lane, same guarantee, same three answers, and the symmetry is the
// argument rather than a coincidence: Handle never blocks, it drops the NEWEST
// record, and the drop count is exact and never reset. A log line is not
// superseded by a later one — unlike a market-data update, where the newest is
// the one worth keeping — so refusing the newcomer is also the choice that
// touches nothing already accepted.
//
// The failure it prevents is specific and has taken systems down: a JSON handler
// writing to a pipe whose reader has stalled blocks in write(2), and the
// goroutine holding that block is whichever one logged — which, in this process,
// is a market-data reducer or the OMS single writer. Observability that can halt
// the thing it observes is worse than none, because it adds a failure mode to
// the system it was added to protect.
//
// # There is no internal goroutine, deliberately
//
// The obvious alternative — a channel and a drain goroutine — would give the
// kernel a lifecycle to own: started by whom, stopped in what order relative to
// the control lane's drain, leaked if a caller forgets. Instead Drain is a
// periodic sweep the caller schedules, exactly as ObservabilityLane offers
// TryReceive and no blocking Receive. The buffer is guarded by a mutex held only
// across a slice append, never across a syscall, so the emitting goroutine's
// worst case is contention with another emitter rather than with a write.
//
// Safe for concurrent use.
type BufferedHandler struct {
	level slog.Leveler

	mu  sync.Mutex
	buf []slog.Record
	cap int

	dropped atomic.Uint64
}

// NewBufferedHandler returns a handler buffering up to capacity records and
// emitting at or above level.
//
// A non-positive capacity is refused rather than normalised, the same refusal
// events.NewObservabilityLane makes: a zero-capacity buffer drops every record
// while reporting itself as a working handler, which is the worst combination —
// no logs and no signal that there are no logs.
//
// A nil level means slog.LevelInfo. That default is safe in the direction that
// matters here: the zero slog.Level IS LevelInfo, so a caller who passes nothing
// gets the level they would have got from the zero value anyway, and the choice
// cannot silently enable debug logging on the trading path.
func NewBufferedHandler(capacity int, level slog.Leveler) (*BufferedHandler, error) {
	if capacity <= 0 {
		return nil, ErrCapacityInvalid
	}
	if level == nil {
		level = slog.LevelInfo
	}
	return &BufferedHandler{
		level: level,
		buf:   make([]slog.Record, 0, capacity),
		cap:   capacity,
	}, nil
}

// Enabled implements slog.Handler.
func (h *BufferedHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

// Handle implements slog.Handler. It never blocks and never returns an error.
//
// The nil return is not laziness. A handler on this lane has no error a caller
// could act on: the only failure available to it is "the buffer was full", which
// is designed behaviour, and reporting it would push the emitter into handling
// it. Callers that want to know how much was lost read Dropped.
func (h *BufferedHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.buf) >= h.cap {
		h.dropped.Add(1)
		return nil
	}
	// Cloned because slog.Record's attrs spill into a shared backing array
	// past the fifth, and this handler retains the record long after Handle
	// returns — which is precisely the case slog.Record.Clone documents.
	//
	// An expected surviving mutant lives here, and it is worth naming rather
	// than leaving for the next person to re-derive. Dropping the Clone is not
	// observable through THIS package's callers: the only mutation available to
	// a caller is AddAttrs, which appends, and an append never overwrites an
	// element the retained copy can already see. It becomes observable the
	// moment two records are derived from one base — both then write the same
	// shared slot — which is a shape no current caller produces and any future
	// one might. So the Clone stays on the contract rather than on a
	// reachability argument that a later edit would silently invalidate.
	h.buf = append(h.buf, r.Clone())
	return nil
}

// WithAttrs implements slog.Handler.
//
// The returned handler shares this one's buffer and drop counter. Sharing is
// required rather than convenient: a per-derivation buffer would split the
// capacity bound across every component that called With, so the process-wide
// bound this type exists to provide would not exist, and Dropped would report
// one derivation's losses while another's went unnoticed.
func (h *BufferedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &handlerView{sink: h, attrs: cloneAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h *BufferedHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &handlerView{sink: h, group: name}
}

// Drain removes and returns every buffered record, oldest first.
//
// Returning them rather than writing them keeps this type independent of a
// destination: the caller owning the flush cadence also owns the encoder and the
// file, which is where §11's per-binary differences live.
func (h *BufferedHandler) Drain() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.buf) == 0 {
		// An expected surviving mutant lives here: returning an empty non-nil
		// slice instead of nil is indistinguishable to every caller, since both
		// range zero times and both report len 0. It is a fast path, not a
		// claim.
		return nil
	}
	out := h.buf
	h.buf = make([]slog.Record, 0, h.cap)
	return out
}

// Len reports the current backlog. Point-in-time, for metrics and tests.
func (h *BufferedHandler) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.buf)
}

// Dropped reports how many records this handler has refused, cumulatively.
//
// Never reset, for the reason ObservabilityLane.Dropped is not: a rate is the
// caller's to derive, and a counter that resets cannot answer "did we lose
// anything during that incident" once the incident is over.
func (h *BufferedHandler) Dropped() uint64 { return h.dropped.Load() }

// handlerView is one derivation of a BufferedHandler: a set of attrs, possibly
// nested inside a group, over the shared sink.
//
// slog's contract is that WithGroup opens a group which every later attr —
// including the record's own — falls inside, and that groups nest. That is a
// tree, so a view holds a parent pointer rather than a flat list, and qualify
// folds from the innermost level outward.
type handlerView struct {
	sink   *BufferedHandler
	parent *handlerView

	// group, when non-empty, nests this level's attrs and everything below it.
	group string
	attrs []slog.Attr
}

func (v *handlerView) Enabled(ctx context.Context, l slog.Level) bool {
	return v.sink.Enabled(ctx, l)
}

func (v *handlerView) Handle(ctx context.Context, r slog.Record) error {
	// The record's own attrs are collected and re-added qualified, rather than
	// left in place, because slog nests them inside any open group. Leaving
	// them at the top level would silently flatten a grouped logger's output,
	// so two components using the same key would collide in the destination.
	own := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		own = append(own, a)
		return true
	})

	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	out.AddAttrs(v.qualify(own)...)
	return v.sink.Handle(ctx, out)
}

// qualify wraps attrs in this view's accumulated groups and prepends its
// accumulated attrs, innermost level first.
func (v *handlerView) qualify(attrs []slog.Attr) []slog.Attr {
	for n := v; n != nil; n = n.parent {
		combined := make([]slog.Attr, 0, len(n.attrs)+len(attrs))
		combined = append(combined, n.attrs...)
		combined = append(combined, attrs...)
		if n.group != "" {
			attrs = []slog.Attr{{Key: n.group, Value: slog.GroupValue(combined...)}}
			continue
		}
		attrs = combined
	}
	return attrs
}

func (v *handlerView) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return v
	}
	// Appended to a COPY rather than to v.attrs. Two derivations of one logger
	// share a parent, and appending in place would let the second one's attrs
	// land in the first's backing array — the classic aliased-append defect,
	// whose symptom is one component's log lines carrying another's fields.
	next := &handlerView{sink: v.sink, parent: v.parent, group: v.group}
	next.attrs = append(cloneAttrs(v.attrs), cloneAttrs(attrs)...)
	return next
}

func (v *handlerView) WithGroup(name string) slog.Handler {
	if name == "" {
		return v
	}
	return &handlerView{sink: v.sink, parent: v, group: name}
}

func cloneAttrs(a []slog.Attr) []slog.Attr {
	if len(a) == 0 {
		return nil
	}
	out := make([]slog.Attr, len(a))
	copy(out, a)
	return out
}
