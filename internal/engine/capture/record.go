package capture

import (
	"fmt"
	"time"
)

// RecordKind names what a record in the archive is. §32g's table gives each
// member by the failure it prevents, and the members below repeat that rather
// than describing the payload, because the payload is the boring half.
//
// # It is an integer enum, which is a SECOND documented exception to §3.6 rule 8
//
// [SPEC EDIT OWED] Rule 8 makes enum-likes `type X string` + constants +
// Validate(), with one documented exception (fixedpoint.RoundingMode), and it is
// explicit that the exception does not cover this case: "The `type X string`
// shape exists for enums crossing a persistence or wire boundary, where a
// self-describing stored token is worth the width. […] An enum-like that does
// cross either boundary follows the rule." A RecordKind crosses a persistence
// boundary — it is a byte in every frame of the archive — so by the letter of
// rule 8 it should be a string.
//
// It is not, on two grounds, and the first is mechanical: §32e's frame is a
// FIXED 32-byte header, and `payload := frameLen - 32` is only true if every
// header field has a fixed width. A variable-length token in the header means a
// second length to parse before the payload's length is known, in the one part
// of the format that must stay decodable from a corrupt file.
//
// The second is that rule 8's RATIONALE is already answered here by the
// mechanism §32e chose over NDJSON, one level down. Self-description was worth
// the width in a database column because the column is the only artefact; in
// this archive the storage format is deliberately not the readable one — "the
// archive is the fact; readability is a tool over it" — and `arblab capture cat`
// is where a kind becomes a name. Spending eight bytes per record to duplicate
// what a tool prints is width bought for nothing.
//
// What the integer shape buys instead is rule 8's OTHER half, and here it earns
// more than it does for events.Lane: RecordKindUnspecified is the zero value and
// is refused, so a zero-filled region of a file — a sparse hole, or a page
// allocated before a crash — cannot decode as a valid record. Had Data been
// zero, the most common corruption pattern on disk would parse cleanly as an
// empty data record.
type RecordKind uint8

const (
	// RecordKindUnspecified is the zero value and names no record. It is
	// refused on both encode and decode; see ErrKindUnspecified for why the
	// refusal on DECODE is the one that matters.
	RecordKindUnspecified RecordKind = iota

	// RecordKindOpen begins a file: tool version and git SHA, config revision
	// hash, host, endpoint URL, the subscription request bytes, the venue's
	// subscribe ack, and the previous file's last CaptureSeq.
	//
	// Provenance. Six weeks later "which build, which topic, which endpoint"
	// is unanswerable from the frames alone, and the previous file's last
	// sequence is what makes cross-file continuity checkable without reading
	// either file in full.
	RecordKindOpen

	// RecordKindData is a venue frame, verbatim. The only kind whose payload
	// this package did not construct, and the only one it must never touch.
	RecordKindData

	// RecordKindAlive is an empty tick written UNCONDITIONALLY every
	// alive_interval — not "when idle".
	//
	// A dead capture and a quiet market are identical from outside (§31g rule
	// 4). Writing the tick unconditionally makes coverage a COUNT (N ticks per
	// hour, compared against expected) rather than something inferred from the
	// absence of data, and inference from absence is precisely what fails
	// silently for three months.
	RecordKindAlive

	// RecordKindGap records a discontinuity: reconnect, overflow, torn tail,
	// process restart, no-first-data, or a clock step.
	//
	// §3.5 requires that any drop be a recorded capture gap and never a silent
	// reorder. It is corroboration rather than the only witness — CaptureSeq is
	// stamped by the READER, so a drop leaves a hole in the sequence whether or
	// not this record survived (§32d).
	RecordKindGap

	// RecordKindRest is a REST response: method, URL, params, status and
	// RecvTime as metadata, then the body verbatim.
	//
	// §32h: Binance's fundingInfo has the property that ABSENT means 8h, so
	// "we asked and it was empty" and "we never asked" are indistinguishable
	// unless the empty answer is recorded as a positive fact with a timestamp.
	RecordKindRest

	// RecordKindClose ends a file with its last CaptureSeq, record count and
	// SHA-256.
	//
	// Its ABSENCE is the signal: a file with no Close ended in a crash, which
	// is distinguishable from a clean stop only because the clean stop leaves
	// something behind.
	RecordKindClose

	// RecordKindClockStep records that platform/clock's Wall absorbed a
	// backwards wall-clock step: how many, how much in total, and the system
	// and clamped instants.
	//
	// §32i. The clamp is right for the engine — a timer fires late rather than
	// a settlement being evaluated twice — and wrong for an archive: on a
	// laptop an NTP step yields a run of IDENTICAL RecvTimes, §3.4's cache
	// accepts an equal RecvTime, and replay proceeds over a squashed time axis
	// with every freshness and skew number wrong and nothing failing.
	RecordKindClockStep
)

// maxRecordKind is the last declared value. Validate compares against it, so
// adding a kind means extending this — which the declared-set test checks.
const maxRecordKind = RecordKindClockStep

// Validate reports whether k is a declared kind, excluding the zero sentinel.
func (k RecordKind) Validate() error {
	switch {
	case k == RecordKindUnspecified:
		return ErrKindUnspecified
	case k > maxRecordKind:
		return fmt.Errorf("kind %d: %w", uint8(k), ErrKindUnknown)
	}
	return nil
}

// String implements fmt.Stringer.
//
// exhaustiveswitch (§3.6 rule 10) binds on this switch: a kind added above and
// not named here is a compile-time failure rather than a value that prints as a
// number. The trailing return is still reachable for an UNDECLARED value, which
// Go's type system permits and the gate cannot police — so
// TestRecordKindStringCoversEveryDeclaredValue walks the declared range as well.
func (k RecordKind) String() string {
	switch k {
	case RecordKindUnspecified:
		return "RecordKindUnspecified"
	case RecordKindOpen:
		return "RecordKindOpen"
	case RecordKindData:
		return "RecordKindData"
	case RecordKindAlive:
		return "RecordKindAlive"
	case RecordKindGap:
		return "RecordKindGap"
	case RecordKindRest:
		return "RecordKindRest"
	case RecordKindClose:
		return "RecordKindClose"
	case RecordKindClockStep:
		return "RecordKindClockStep"
	}
	return fmt.Sprintf("RecordKind(%d)", uint8(k))
}

// Record is one entry in the archive: §32e's frame with the wire encoding taken
// off. The flags and reserved header fields have no counterpart here on purpose
// — they are required to be zero, so representing them would offer a caller a
// value whose only legal setting is the one it already has.
type Record struct {
	Kind RecordKind

	// CaptureSeq is dense per (SourceID, Topic) and is stamped by the READER at
	// arrival, never by the writer (§32d). That placement is what makes
	// losslessness checkable rather than asserted: a frame dropped at the queue
	// leaves a hole here, visible in the file with no other evidence required.
	// Assigned at the writer it would be dense by construction, and a drop would
	// be invisible unless the gap record itself survived.
	//
	// The key is (SourceID, Topic) and not SourceID alone, because one SourceID
	// has more than one wire representation (§32c) — that is a per-file property
	// and so lives in the archive path, not in the frame.
	CaptureSeq uint64

	// RecvTime is when the frame arrived, from the INJECTED clock (§32i). Never
	// time.Now(): §3.6 rule 5's transport escape covers socket DEADLINES, not
	// stamps, and this is the timestamp §3.4 measures freshness and cross-leg
	// skew against.
	//
	// Encoded as Unix nanoseconds, so the monotonic reading and the location do
	// not survive a round trip and equality must be tested with Equal rather
	// than ==. The instant does survive exactly.
	RecvTime time.Time

	// Mono is nanoseconds since process start, and it is here because RecvTime
	// alone cannot be trusted across a clock step (§32i). Two records' Mono
	// difference is meaningful within one process run and meaningless across
	// runs — which is why an Open record exists to mark where a run began.
	Mono time.Duration

	// Payload is stored verbatim. Decode returns a COPY rather than a slice of
	// the caller's buffer: capture is optimized for losslessness and provenance,
	// never latency (§31c), and a reader that reuses its buffer would otherwise
	// mutate records it has already handed out. A nil and an empty payload
	// encode identically, so Decode reports the empty one as nil.
	Payload []byte
}

// Validate reports whether r can be encoded, and — this is the load-bearing
// part — it is called on DECODE as well.
//
// One statement of the rules in both directions is what makes the format
// canonical: every byte string the reader accepts is exactly the one the writer
// would have produced for the record it reports. That is §32l's "never reports a
// record the writer did not write" as a property rather than an intention, and
// FuzzDecodeRecord asserts it directly.
func (r Record) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if r.RecvTime.IsZero() {
		return ErrRecvTimeUnset
	}
	// UnixNano is only defined for roughly 1678..2262 and wraps outside it
	// rather than reporting. Round-tripping the value is the exact test, and it
	// costs one allocation-free conversion.
	if ns := r.RecvTime.UnixNano(); !time.Unix(0, ns).Equal(r.RecvTime) {
		return fmt.Errorf("%s: %w", r.RecvTime.Format(time.RFC3339), ErrRecvTimeRange)
	}
	if r.Mono < 0 {
		return fmt.Errorf("%s: %w", r.Mono, ErrMonoNegative)
	}
	if len(r.Payload) > MaxPayload {
		return fmt.Errorf("payload is %d bytes, limit %d: %w",
			len(r.Payload), MaxPayload, ErrFrameLen)
	}
	return nil
}
