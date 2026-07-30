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
// # These slices
//
// Slice A of §32m is the frame codec, its oracle, a rapid round-trip property
// and a fuzz target — the format, and the refusals that make a corrupt frame
// unreadable rather than plausible.
//
// Slice B is the writer over it: the archive layout and its inverse (path.go),
// the gap taxonomy (gap.go), the exclusive per-stream lock (lock.go), torn-tail
// recovery (recover.go), the bounded drop-newest queue (queue.go) and the writer
// itself — files, hourly rotation on UTC RecvTime, unconditional Alive ticks,
// digests at close. Both are driven by an injected clock and a fake source; no
// network and no venue facts, which is why neither is blocked by any of §13's
// five verification debts.
//
// Slice C's share of this package is verify (verify.go, finding.go): §32k's
// out-of-process liveness check, and the only one of the three that is not inside
// the process whose death it detects. It re-derives every count, digest and
// continuity rule from the bytes rather than sharing the writer's bookkeeping,
// because a checker that agrees with the writer because it IS the writer confirms
// only that the code agrees with itself. Two axes, never summed: a coverage
// verdict per (stream, window) — replayable, gapped, absent — and a finding class
// that is §32k's two exit codes, 1 for an archive that is lying and 2 for one that
// is honest and thin.
//
// It reads no payload a venue wrote, which is the same fence one layer out: verify
// checks OUR CaptureSeq (§32d), stamped identically for all seven sources, so
// §32c's source 4 — Binance spot partial depth, the one stream with no wire-level
// gap check available at all — is verified exactly as strictly as the other six.
//
// Slices C–F have since landed: `capture run`'s config and composition root live
// in internal/app/capture and cmd/arblab, the REST overlay is rest.go (E), and
// slice F added the manifest (manifest.go) and `capture cat` (cat.go).
//
// The frame is also reused OUTSIDE the archive: engine/dispatch's sequence log
// (§39n) writes this container through AppendRecord and walks it through
// WalkRecords. That does not move the fence — it is unidirectional on purpose:
// dispatch may import capture, capture never imports the dispatcher, and this
// package learns nothing about what a CaptureSeq means to that caller.
package capture
