package dispatch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// LogWriter is the dispatch log's writer, and it is a Sink on purpose: the log
// joins the dispatch path by wrapping the sink the state machine lives behind,
// so the dispatcher needs no second seam and the wiring slice composes it like
// any other consumer.
//
// # Write-ahead, and which failure each order leaves behind
//
// Every entry is frozen to the file BEFORE the wrapped sink sees the item
// (§39n-5). A log missing an entry the state applied un-freezes the
// arbitration §3.5 persists — replay would re-derive an order live already
// decided, which is the exact thing the log exists to prevent. The reverse
// residue — a logged entry the state never applied, because the process died
// between the write and the apply — replays harmlessly: replay re-applies the
// log into a fresh state, so the entry is simply applied then.
//
// # Fresh files only
//
// NewLogWriter refuses an existing path (O_EXCL). A dispatch log continues
// only if the dispatcher's counter continues, and no dispatcher state survives
// a crash in this milestone — resuming a dispatch sequence is the journal's
// business (milestone 3), and an append door built now would be walked only by
// tests. A log that starts mid-run is refused by the density assertion below
// for the same reason: it would be a lie about where the order began.
//
// One caller, the same contract as the Dispatcher it serves; no mutex, because
// the Dispatcher's own lock already serializes the sink path.
type LogWriter struct {
	f       *os.File
	bw      *bufio.Writer
	scratch []byte
	next    Sink

	// last is the last entry's seq, asserted — never trusted — against what
	// the dispatcher hands over: §39h's argument, a writer that verifies the
	// one guarantee it depends on with one comparison beats one that trusts
	// it. lastAt is that entry's instant, which is the only honest
	// replay-stable stamp a clockless writer has for the Close seal.
	last   uint64
	lastAt time.Time
	closed bool
}

// Compile-time: a LogWriter is a Sink (§3.6 rule 2 — breaking this is a build
// failure, which is the NOBUILD-is-the-claim case).
var _ Sink = (*LogWriter)(nil)

// NewLogWriter creates the log at path and wraps next. The path must not
// exist; next must.
func NewLogWriter(path string, next Sink) (*LogWriter, error) {
	if next == nil {
		return nil, ErrSinkNil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("dispatch: create log: %w", err)
	}
	bw := bufio.NewWriter(f)
	if _, err := bw.Write(capture.AppendFileHeader(nil)); err != nil {
		f.Close()
		return nil, fmt.Errorf("dispatch: write log header: %w", err)
	}
	return &LogWriter{f: f, bw: bw, next: next}, nil
}

// appendLogFrame wraps one entry in the container's frame. Mono is the
// constant zero, stated rather than defaulted: it is per-run process timing,
// the log's authority is order, and a real reading would make the parity
// probe's byte-identity false while measuring nothing the log answers for
// (§39n-2).
func appendLogFrame(dst []byte, seq uint64, at time.Time, body []byte, what string) ([]byte, error) {
	out, err := capture.AppendRecord(dst, capture.Record{
		Kind:       capture.RecordKindData,
		CaptureSeq: seq,
		RecvTime:   at,
		Payload:    body,
	})
	if err != nil {
		return dst, fmt.Errorf("dispatch: log frame for %s: %w", what, err)
	}
	return out, nil
}

// write asserts density, frames the entry and commits it to the buffer.
func (w *LogWriter) write(seq uint64, at time.Time,
	encode func([]byte) ([]byte, error)) error {
	if w.closed {
		return ErrLogClosed
	}
	if seq != w.last+1 {
		return fmt.Errorf("entry %d after %d: %w", seq, w.last, ErrLogSeqNotDense)
	}
	frame, err := encode(w.scratch[:0])
	if err != nil {
		return err
	}
	w.scratch = frame[:0]
	if _, err := w.bw.Write(frame); err != nil {
		return fmt.Errorf("dispatch: write log entry %d: %w", seq, err)
	}
	w.last, w.lastAt = seq, at
	return nil
}

// OnEnvelope freezes the envelope's sequence record, then hands the envelope
// to the wrapped sink. An error before the sink call means nothing was
// applied; the dispatcher still treats it as a spent stamp (§39m-4), and the
// density assertion makes any attempt to continue past it loud.
func (w *LogWriter) OnEnvelope(e events.Envelope) error {
	err := w.write(e.IngressSeq, e.RecvTime, func(dst []byte) ([]byte, error) {
		return encodeEnvelope(dst, e)
	})
	if err != nil {
		return err
	}
	return w.next.OnEnvelope(e)
}

// OnTimer freezes the firing's position in the same dense sequence (§39m-6),
// then forwards it.
func (w *LogWriter) OnTimer(seq uint64, f clock.Fired) error {
	err := w.write(seq, f.At, func(dst []byte) ([]byte, error) {
		return encodeTimer(dst, seq, f)
	})
	if err != nil {
		return err
	}
	return w.next.OnTimer(seq, f)
}

// Close seals the log and releases the file. The seal is what makes a clean
// end distinguishable from a crash that happened to tear at a record boundary
// (§39n-4) — its absence is the signal, exactly RecordKindClose's own
// argument. An empty log gets no seal: no entry means no instant to stamp one
// with, and a header-only file already reads as "empty" on replay.
func (w *LogWriter) Close() error {
	if w.closed {
		return ErrLogClosed
	}
	w.closed = true

	var firstErr error
	if w.last > 0 {
		if frame, err := capture.AppendRecord(w.scratch[:0], capture.Record{
			Kind:       capture.RecordKindClose,
			CaptureSeq: w.last,
			RecvTime:   w.lastAt,
			Payload:    mustJSON(closePayload{Entries: w.last}),
		}); err != nil {
			firstErr = fmt.Errorf("dispatch: encode log seal: %w", err)
		} else if _, err := w.bw.Write(frame); err != nil {
			firstErr = fmt.Errorf("dispatch: write log seal: %w", err)
		}
	}
	if err := w.bw.Flush(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("dispatch: flush log: %w", err)
	}
	if err := w.f.Sync(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("dispatch: sync log: %w", err)
	}
	if err := w.f.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("dispatch: close log: %w", err)
	}
	return firstErr
}

// mustJSON marshals a payload this package itself constructed. A marshal
// failure here is a programming error in this file, not a runtime condition —
// closePayload contains one integer.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("dispatch: marshal own payload: %v", err))
	}
	return b
}
