package dispatch

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The dispatch log's sequence records — §3.5's persisted DispatchSeq log,
// §39e/§39n's shape: sequences, never payloads. A log entry freezes WHAT was
// dispatched WHEN in the consumed order; the dispatched payload itself is
// re-derived at replay from the raw capture archive, which stays the single
// authority for content (§31b — a second payload copy is a second authority).
//
// # The container is capture's frame, and nothing is stored twice
//
// Each entry is one §32e frame (capture.AppendRecord/WalkRecords): the frame's
// CaptureSeq IS the IngressSeq, its RecvTime is the input's own replay-stable
// instant (an envelope's RecvTime, a timer's scheduled At), and its Mono is the
// constant zero — a per-run process reading would make the parity probe's
// byte-identical-logs claim false on arrival while measuring nothing the log
// answers for (§39n-2). The payload holds only what the frame does not.
//
// # Payload schema: JSON, `k`-discriminated, no version integer
//
// gap.go's header/payload boundary, inherited: fixed-width lives in the frame
// header, self-describing tokens in the payload. The `k` token is the refusal
// surface and the evolution contract (§39n-3): an unknown token is refused, a
// shape change is a new token, an additive field is compatible because readers
// ignore unknown JSON fields, and a frame-layout change belongs to the
// container's version, not this schema.

// Payload discriminator tokens. A Close frame carries no token — its frame
// kind already discriminates it, and a second discriminator could disagree.
const (
	logKindEnvelope = "env"
	logKindTimer    = "timer"
)

// envPayload is the envelope entry's payload half. SourceTimeNs is omitted
// when the venue sent no timestamp — absence is the semantic (§3.5 carries
// SourceTime for freshness and skew only), so absence is the encoding.
type envPayload struct {
	K            string `json:"k"`
	Venue        string `json:"venue"`
	Product      string `json:"product"`
	Symbol       string `json:"symbol"`
	Contract     string `json:"contract"`
	Stream       string `json:"stream"`
	Accept       uint64 `json:"accept"`
	VenueSeq     uint64 `json:"venue_seq"`
	SourceTimeNs int64  `json:"source_time_ns,omitempty"`
	Prio         uint8  `json:"prio"`
}

// timerPayload is the timer entry's payload half; the scheduled At rides in
// the frame's RecvTime.
type timerPayload struct {
	K    string `json:"k"`
	ID   string `json:"id"`
	Prio uint8  `json:"prio"`
}

// closePayload seals the log: the entry count, which under density-from-1
// equals the last entry's seq. The frame's CaptureSeq repeats that seq — the
// one deliberate duplication in the format, because a seal that asserts
// nothing seals nothing; ReplayLog cross-checks both (§39n-4).
type closePayload struct {
	Entries uint64 `json:"entries"`
}

// EnvelopeRecord is a replayed envelope entry: everything the log froze about
// one dispatched envelope. The payload is deliberately absent — replay
// re-derives it from the raw archive (slice C's wiring).
type EnvelopeRecord struct {
	// IngressSeq is the position this dispatch occupies in the consumed order.
	IngressSeq uint64

	Source events.SourceID

	// AcceptOrdinal is the raw arrival ordinal of the input that triggered the
	// dispatched envelope — the pre-coalescing half of §3.5's persisted pair.
	// The reader does not judge it: zero is §3.5's absence-derived case, whose
	// entry door is deferred to its producer (§39m-5), and a reader that
	// refused it today would have to be un-taught in that slice.
	AcceptOrdinal uint64

	VenueSeq uint64

	// SourceTime is the venue's own stamp; the zero time means the venue sent
	// none, exactly as on the live envelope. Decoded in UTC (the container's
	// rule — a rendering that depends on $TZ turns a diff into a timezone
	// question); instants survive exactly, so compare with Equal.
	SourceTime time.Time

	// RecvTime is the instant the live pipeline received the input, from the
	// injected clock. Decoded in UTC.
	RecvTime time.Time

	Prio events.Prio
}

// TimerRecord is a replayed timer entry: the sequence position the firing
// occupied (§39m-6 — timers and envelopes share one dense counter) and the
// Fired exactly as the sink received it. Fired.At is the SCHEDULED instant,
// identical in a replay by construction; decoded in UTC.
type TimerRecord struct {
	IngressSeq uint64
	Fired      clock.Fired
}

// LogConsumer receives a replayed log in order. It mirrors Sink's two-method
// shape on purpose: replay's job is re-driving, and a consumer shaped like the
// write side composes with the wiring slice without an adapter. An error stops
// the replay and is returned to ReplayLog's caller, distinguishable from
// corruption by being the consumer's own error value.
type LogConsumer interface {
	OnEnvelopeRecord(r EnvelopeRecord) error
	OnTimerRecord(r TimerRecord) error
}

// encodeEnvelope builds the frame for a dispatched envelope. The caller has
// already asserted density, so e.IngressSeq is trusted here.
func encodeEnvelope(dst []byte, e events.Envelope) ([]byte, error) {
	if !e.SourceTime.IsZero() {
		// UnixNano wraps silently outside ~1678..2262 (the codec's own
		// RecvTime reasoning, applied to the one time field the payload
		// carries): round-tripping is the exact test.
		if ns := e.SourceTime.UnixNano(); !time.Unix(0, ns).Equal(e.SourceTime) {
			return dst, fmt.Errorf("dispatch: source %s SourceTime %s: %w",
				e.Source, e.SourceTime.Format(time.RFC3339), ErrLogTimeRange)
		}
	}
	p := envPayload{
		K:        logKindEnvelope,
		Venue:    string(e.Source.Instrument.Venue),
		Product:  string(e.Source.Instrument.Product),
		Symbol:   e.Source.Instrument.Symbol,
		Contract: string(e.Source.Instrument.Contract),
		Stream:   string(e.Source.Stream),
		Accept:   e.AcceptOrdinal,
		VenueSeq: e.VenueSeq,
		Prio:     uint8(e.Prio),
	}
	if !e.SourceTime.IsZero() {
		p.SourceTimeNs = e.SourceTime.UnixNano()
	}
	body, err := json.Marshal(p)
	if err != nil {
		return dst, fmt.Errorf("dispatch: encode envelope entry: %w", err)
	}
	return appendLogFrame(dst, e.IngressSeq, e.RecvTime, body,
		fmt.Sprintf("envelope from %s", e.Source))
}

// encodeTimer builds the frame for a timer firing. The scheduled At becomes
// the frame's RecvTime; a zero At is refused by the container's own check,
// which is honest — such a timer cannot exist under the zero-value rule.
func encodeTimer(dst []byte, seq uint64, f clock.Fired) ([]byte, error) {
	body, err := json.Marshal(timerPayload{K: logKindTimer, ID: string(f.ID), Prio: f.Prio})
	if err != nil {
		return dst, fmt.Errorf("dispatch: encode timer entry: %w", err)
	}
	return appendLogFrame(dst, seq, f.At, body, fmt.Sprintf("timer %s", f.ID))
}

// decodeEntry turns a Data frame back into the consumer call it froze.
func decodeEntry(rec logFrame, c LogConsumer) error {
	var probe struct {
		K string `json:"k"`
	}
	if err := json.Unmarshal(rec.Payload, &probe); err != nil {
		return fmt.Errorf("%w: %w", ErrLogEntryInvalid, err)
	}
	switch probe.K {
	case logKindEnvelope:
		var p envPayload
		if err := json.Unmarshal(rec.Payload, &p); err != nil {
			return fmt.Errorf("%w: %w", ErrLogEntryInvalid, err)
		}
		r := EnvelopeRecord{
			IngressSeq: rec.Seq,
			Source: events.SourceID{
				Instrument: ids.VenueInstrument{
					Venue:    ids.Venue(p.Venue),
					Product:  ids.ProductCategory(p.Product),
					Symbol:   p.Symbol,
					Contract: ids.ContractType(p.Contract),
				},
				Stream: events.StreamKind(p.Stream),
			},
			AcceptOrdinal: p.Accept,
			VenueSeq:      p.VenueSeq,
			RecvTime:      rec.RecvTime,
			Prio:          events.Prio(p.Prio),
		}
		if p.SourceTimeNs != 0 {
			r.SourceTime = time.Unix(0, p.SourceTimeNs).UTC()
		}
		// The writer trusted the dispatcher's post-stamp Validate; the reader
		// re-derives the source's validity independently — the two-sides
		// discipline, and it catches a log written by a build whose producer
		// checks were broken (§39n-6).
		if err := r.Source.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrLogEntryInvalid, err)
		}
		return c.OnEnvelopeRecord(r)

	case logKindTimer:
		var p timerPayload
		if err := json.Unmarshal(rec.Payload, &p); err != nil {
			return fmt.Errorf("%w: %w", ErrLogEntryInvalid, err)
		}
		if p.ID == "" {
			return fmt.Errorf("timer entry with no ID: %w", ErrLogEntryInvalid)
		}
		return c.OnTimerRecord(TimerRecord{
			IngressSeq: rec.Seq,
			Fired:      clock.Fired{ID: clock.TimerID(p.ID), At: rec.RecvTime, Prio: p.Prio},
		})
	}
	return fmt.Errorf("token %q: %w", probe.K, ErrLogEntryUnknown)
}

// logFrame is the slice of a container record the payload codec needs —
// keeping capture types out of this file's signatures so the container stays
// swappable in exactly one place (logwriter.go / logreader.go).
type logFrame struct {
	Seq      uint64
	RecvTime time.Time
	Payload  []byte
}
