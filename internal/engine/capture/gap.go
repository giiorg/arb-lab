package capture

import (
	"encoding/json"
	"fmt"
)

// Cause names why a discontinuity happened — §32g's gap payload, `cause` field.
//
// # It IS §3.6 rule 8's string shape, and RecordKind's exception does not extend
// # to it
//
// RecordKind takes a documented exception to rule 8 because it is a fixed-width
// byte in the frame HEADER, where a variable-length token would mean a second
// length to parse before the payload's length is known. A Cause lives in the
// gap record's JSON PAYLOAD, where nothing is fixed-width and the self-describing
// token costs a handful of bytes on a record that is rare by construction. So the
// rule applies unmodified: `type X string` + constants + Validate.
//
// The header/payload boundary is the whole distinction, and it is worth stating
// because the two decisions look contradictory from a distance — one enum-like in
// this package is an integer and the next is a string, in the same format, for
// reasons that are not about taste.
//
// exhaustiveswitch (§3.6 rule 10) binds on this type, so a cause added below and
// not named in Validate is a compile-time failure rather than a discontinuity
// that validates as legal with no range attached.
type Cause string

const (
	// CauseUnspecified is the zero value and names nothing. A gap record whose
	// cause nobody set is the worst possible record: it says a discontinuity
	// happened and refuses to say what kind, which is strictly less useful than
	// no record at all because it also stops anyone looking further.
	CauseUnspecified Cause = ""

	// CauseReconnect is a socket that dropped and came back. Known without
	// reading a message, which is what keeps it on capture's side of §32n's
	// no-parsing fence.
	CauseReconnect Cause = "reconnect"

	// CauseOverflow is §32f's bounded queue refusing a frame under load. The
	// range is the hole it left: drop-newest means the lost frames are
	// contiguous and the sequence resumes above them.
	CauseOverflow Cause = "overflow"

	// CauseTornTail is a file whose last record was cut off by a process that
	// died mid-write, found and truncated at the next open (§32e).
	//
	// It carries Bytes rather than a sequence range, and that asymmetry is the
	// reason Gap has both: the lost bytes were never a whole record, so no
	// CaptureSeq is known to be missing — the previous run's counter died with
	// the process.
	CauseTornTail Cause = "torn_tail"

	// CauseProcessRestart is an hour file reopened by a new run. Distinct from a
	// torn tail because the previous run ended CLEANLY — the Mono readings on
	// either side of it belong to different processes and cannot be subtracted.
	CauseProcessRestart Cause = "process_restart"

	// CauseNoFirstData is §32k's first-data assertion failing: subscribed,
	// acked, and nothing arrived inside first_data_timeout.
	//
	// This is the one that kills the Binance trap — the legacy /ws/ path
	// decommissioned 2026-04-23 returns NOTHING rather than an error, so an old
	// code sample yields a process that connects, waits politely, and captures
	// an empty file for three months.
	CauseNoFirstData Cause = "no_first_data"

	// CauseClockStep is a wall-clock discontinuity large enough to be a coverage
	// discontinuity rather than only a stamping one. The stamping side is the
	// ClockStep RECORD (§32i); this cause is for a step that moved the rotation
	// window, where whole windows have no file because they never elapsed.
	CauseClockStep Cause = "clock_step"
)

// Validate reports whether c is a declared cause, excluding the zero value.
func (c Cause) Validate() error {
	switch c {
	case CauseUnspecified:
		return ErrCauseUnspecified
	case CauseReconnect, CauseOverflow, CauseTornTail,
		CauseProcessRestart, CauseNoFirstData, CauseClockStep:
		return nil
	}
	return fmt.Errorf("cause %q: %w", string(c), ErrCauseUnknown)
}

// String implements fmt.Stringer.
func (c Cause) String() string { return string(c) }

// hasRange reports whether this cause describes a hole in the CaptureSeq
// sequence, as opposed to a discontinuity with no sequence range to name.
//
// A method rather than a set literal at the call site, so exhaustiveswitch binds
// here too: a cause added later is a compile-time failure at this switch rather
// than a silent "no, it has no range", which would make Validate stop asking for
// the one field that makes the record useful.
func (c Cause) hasRange() bool {
	switch c {
	case CauseOverflow:
		return true
	case CauseUnspecified, CauseReconnect, CauseTornTail,
		CauseProcessRestart, CauseNoFirstData, CauseClockStep:
		return false
	}
	return false
}

// Gap is the payload of a RecordKindGap record — §32g's `{cause, from, to,
// count, detail}`, plus §32e's `bytes`.
//
// Both spellings appear in the design (§32e writes `gap{cause: torn_tail, bytes:
// n}`; §32g's table names the five-field form), and this is their union rather
// than a choice between them, because the two describe different causes: a
// sequence hole has a range and no byte count, a torn tail has a byte count and
// no range. Reconciled in the slice's record rather than absorbed.
type Gap struct {
	Cause Cause `json:"cause"`

	// From and To bound the lost CaptureSeq range, inclusive on both ends, and
	// Count is how many were lost. Count is redundant with the range and is
	// stored anyway: a reader that trusts one and computes the other cannot
	// notice a writer that got them out of step, and Validate checks them
	// against each other on the way in.
	From  uint64 `json:"from,omitempty"`
	To    uint64 `json:"to,omitempty"`
	Count uint64 `json:"count,omitempty"`

	// Bytes is how many trailing bytes were discarded, for a torn tail.
	Bytes int64 `json:"bytes,omitempty"`

	// Detail is free text for a human reading the archive six weeks later. It is
	// never parsed, and nothing branches on it.
	Detail string `json:"detail,omitempty"`
}

// Validate reports whether the gap describes a discontinuity completely enough
// to be worth recording.
//
// The range check is the load-bearing one. §32f's drop-newest is what makes the
// lost run contiguous, so `Count == To-From+1` is a real invariant rather than
// bookkeeping — and a gap record that violates it is evidence the queue dropped
// in some other pattern, which would mean the sequence holes in the file no
// longer mean what §32d says they mean.
func (g Gap) Validate() error {
	if err := g.Cause.Validate(); err != nil {
		return err
	}
	if g.Bytes < 0 {
		return fmt.Errorf("%d bytes: %w", g.Bytes, ErrGapRange)
	}
	if !g.hasRangeFields() {
		if g.Cause.hasRange() {
			return fmt.Errorf("%s gap names no sequence range: %w", g.Cause, ErrGapRange)
		}
		return nil
	}
	if g.To < g.From {
		return fmt.Errorf("%s gap runs [%d, %d]: %w", g.Cause, g.From, g.To, ErrGapRange)
	}
	if want := g.To - g.From + 1; g.Count != want {
		return fmt.Errorf("%s gap [%d, %d] is %d sequences, count says %d: %w",
			g.Cause, g.From, g.To, want, g.Count, ErrGapRange)
	}
	return nil
}

// hasRangeFields reports whether any of the three range fields was populated.
//
// Asking "did the caller fill any of them" rather than "is Count > 0" is what
// makes a half-populated range an error instead of a silently ignored one: a
// caller that sets From and To and forgets Count is describing a real hole with
// a wrong size, and the zero Count would otherwise read as "no range given".
func (g Gap) hasRangeFields() bool { return g.From != 0 || g.To != 0 || g.Count != 0 }

// MarshalPayload renders the gap as the bytes of a RecordKindGap payload.
func (g Gap) MarshalPayload() ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	// Struct field order, so the encoding is deterministic — two runs that
	// record the same gap produce byte-identical payloads, which is what lets a
	// golden test and a content digest mean anything.
	return json.Marshal(g)
}
