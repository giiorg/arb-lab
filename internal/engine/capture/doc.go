// Package capture is the archive format for §12.2's self-capture: the frame
// codec, and later the writer, the archive layout and verify.
//
// Design record: docs/superpowers/specs/2026-07-29-capture-tool-design.md §32,
// on the ordering argument in §31. This file states what the CODE has to keep
// true; the design states why those are the requirements.
//
// # What this package is not, and the fence is structural
//
// §32n's roster, because §31e's hazard is GROWTH and growth is easiest to refuse
// against a list: no parsing, no normalization, no cross-source ordering, no
// AcceptOrdinal, no IngressSeq, no coalescing, no dropping without a record, no
// credentials, and no imports of marketdata, oms, strategy or the dispatcher.
//
// The last one is checked by an import-graph test in this package rather than by
// a seventh §3.6 analyzer — see imports_test.go, which carries the reasoning and
// the positive control. The transport is SHARED with the future live path and
// the dispatcher never is: capturing through a different socket stack than the
// engine runs would make the archive a record of a pipeline we do not run, which
// is the one sharing that makes capture worth doing.
//
// # Bytes in, bytes out — and the boundary is exactly at "did we have to read
// # the message"
//
// Payloads are stored VERBATIM (§31g rule 1). Nothing is re-encoded on the way
// in, because a parser bug is redoable against the raw bytes forever and capture
// is not redoable at all. Transport-level discontinuities — reconnects, socket
// errors, queue overflow, a torn tail — are known WITHOUT reading a message and
// become Gap records. Venue-sequence gaps are not this package's job: reading
// u/U update IDs means parsing, which is how a capture tool grows a parser and
// then a normalizer. §3.4 already assigns sequence-gap detection and resnapshot
// to marketdata, at read time.
//
// # Absence is what has to be recorded, because absence is what cannot be
// # reconstructed later
//
// Gap, Alive and a MISSING Close are three instances of one idea, and it is the
// same idea as §3.4's silence timer: a feed that simply stops emits no event, so
// the stop has to be manufactured by something that was watching. A dead capture
// process and a quiet market are identical from outside (§31g rule 4), and the
// discovery lands three months later at scoring time. See RecordKind, where each
// member is named by the failure it prevents.
//
// # This slice
//
// Slice A of §32m: the frame codec, its oracle, a rapid round-trip property and
// a fuzz target. No I/O, no network, no venue facts — deliberately the one slice
// none of §13's verification debts block. The writer (rotation, the exclusive
// lock, torn-tail recovery, Alive ticks) is slice B and is not here; what is
// here is the format those will be written in, and the refusals that make a
// corrupt frame unreadable rather than plausible.
package capture
