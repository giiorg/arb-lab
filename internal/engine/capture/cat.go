package capture

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// §32e's promise, kept in §38d: the archive stores verbatim bytes and
// readability is a TOOL over it — "arblab capture cat renders NDJSON on
// demand", which is also the sentence core §3.6 rule 8 spends when it argues a
// RecordKind byte needs no self-describing token ("capture cat is where a kind
// becomes a name").
//
// One line per record. A payload that is valid JSON is embedded VERBATIM — it
// left the venue as JSON, and escaping it into a string would be §31g's
// re-encoding hazard arriving at read time. Anything else rides base64 under a
// name that says so. A Rest record's §37a framing is decoded rather than
// dumped, because rendering opaquely what this package itself framed would be
// a tool refusing to read its own format.
//
// # cat stops at the first invalid frame
//
// Reported with its byte offset, and the refusal is the §38d decision:
// resynchronizing past a bad frame is recovery's job, under recovery's rules
// (truncate, record the gap) — a render tool that silently skipped corrupt
// bytes is how a corrupt archive gets quoted as clean fact. The operator's
// tool for a file cat refuses is verify, which names every fault precisely.

// catLine is one rendered record. Exactly one of Payload/PayloadB64 is set for
// a kind with a body, and Rest records carry Meta+Body instead.
type catLine struct {
	Seq      uint64 `json:"seq"`
	Kind     string `json:"kind"`
	RecvTime string `json:"recv_time"`
	MonoNs   int64  `json:"mono_ns"`

	Payload    json.RawMessage `json:"payload,omitempty"`
	PayloadB64 string          `json:"payload_b64,omitempty"`

	Meta    *RestMeta       `json:"meta,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	BodyB64 string          `json:"body_b64,omitempty"`
}

// kindToken is a RecordKind as cat prints it: the archive's own vocabulary in
// lowercase, stable because six-week-old shell pipelines will grep for these.
func kindToken(k RecordKind) string {
	switch k {
	case RecordKindOpen:
		return "open"
	case RecordKindData:
		return "data"
	case RecordKindAlive:
		return "alive"
	case RecordKindGap:
		return "gap"
	case RecordKindRest:
		return "rest"
	case RecordKindClose:
		return "close"
	case RecordKindClockStep:
		return "clock_step"
	case RecordKindUnspecified:
		// DecodeRecord refuses the zero kind (ErrKindUnspecified), so cat can
		// only meet it if the codec's contract broke — named here because the
		// switch must be exhaustive, and named honestly.
		return "unspecified"
	}
	return fmt.Sprintf("kind_%d", uint8(k))
}

// Cat renders one archive file as NDJSON onto dst.
func Cat(dst io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("capture: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	out := bufio.NewWriter(dst)
	enc := json.NewEncoder(out)
	// Venue payloads carry URLs and comparison operators; & for every
	// ampersand would defeat the one purpose this tool has.
	enc.SetEscapeHTML(false)

	s := &scanner{f: f, backing: make([]byte, scanBufInit)}
	if err := s.ensure(); err != nil {
		return fmt.Errorf("capture: read %s: %w", path, err)
	}
	headerLen, err := ParseFileHeader(s.pending())
	if err != nil {
		return fmt.Errorf("capture: %s: %w", path, err)
	}
	offset := int64(headerLen)
	s.lo += headerLen

	for {
		if err := s.ensure(); err != nil {
			return fmt.Errorf("capture: read %s: %w", path, err)
		}
		if len(s.pending()) == 0 {
			break
		}
		rec, used, err := DecodeRecord(s.pending())
		if err != nil {
			// Flushed before refusing, so everything up to the bad frame is on
			// stdout, complete lines only — the operator sees where the archive
			// stopped being readable, and no torn line poses as a record.
			_ = out.Flush()
			return fmt.Errorf("capture: %s+%d: %w", path, offset, err)
		}
		line, err := renderLine(rec)
		if err != nil {
			_ = out.Flush()
			return fmt.Errorf("capture: %s+%d: %w", path, offset, err)
		}
		if err := enc.Encode(line); err != nil {
			return fmt.Errorf("capture: render %s+%d: %w", path, offset, err)
		}
		offset += int64(used)
		s.lo += used
	}
	if err := out.Flush(); err != nil {
		return fmt.Errorf("capture: write output: %w", err)
	}
	return nil
}

// renderLine shapes one record.
func renderLine(rec Record) (catLine, error) {
	line := catLine{
		Seq:      rec.CaptureSeq,
		Kind:     kindToken(rec.Kind),
		RecvTime: rec.RecvTime.UTC().Format(time.RFC3339Nano),
		MonoNs:   int64(rec.Mono),
	}
	if rec.Kind == RecordKindRest {
		rp, err := DecodeRestPayload(rec.Payload)
		if err != nil {
			return catLine{}, err
		}
		line.Meta = &rp.Meta
		line.Body, line.BodyB64 = renderBytes(rp.Body)
		return line, nil
	}
	line.Payload, line.PayloadB64 = renderBytes(rec.Payload)
	return line, nil
}

// renderBytes picks the honest rendering: verbatim for valid JSON, base64
// under a name that admits it for anything else, neither for nothing.
func renderBytes(b []byte) (json.RawMessage, string) {
	switch {
	case len(b) == 0:
		return nil, ""
	case json.Valid(b):
		return json.RawMessage(b), ""
	default:
		return nil, base64.StdEncoding.EncodeToString(b)
	}
}
