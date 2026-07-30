package capture

import (
	"fmt"
	"time"
)

// FindingClass is §32k's two exit codes as a type:
//
//	1 = integrity failure — the archive is LYING
//	2 = coverage below threshold — the archive is HONEST AND THIN
//
// The distinction is the whole point of verify on a laptop, because on this host
// gaps are EXPECTED (§32a): an archive whose gaps are expected is worthless
// unless they are measured, and an archive that measures them wrongly is worse
// than one that does not measure at all. So the two are never summed into a
// single "problems" count. §32a's verdict line is the same sentence one level up:
// a recorded gap is not an error; an unexplained silence is.
//
// Nothing carries a FindingClass as a FIELD. It is derived by FindingKind.Class,
// so a finding cannot be constructed with a class that disagrees with its kind —
// and, more usefully, adding a kind is a compile-time failure at that switch
// until somebody decides which exit code it earns.
type FindingClass string

const (
	// ClassUnspecified is the zero value and names neither exit code. Refused,
	// for the reason the whole package refuses zero values: an unclassified
	// finding would be counted by neither exit path and so would be reported
	// and then ignored, which is indistinguishable from not finding it.
	ClassUnspecified FindingClass = ""

	// ClassIntegrity is exit 1: the bytes contradict themselves, or the report
	// cannot be trusted to describe them. A CRC failure, a digest mismatch, a
	// CaptureSeq that repeats, a file this build cannot read.
	//
	// Read it as "do not trust this run's numbers" rather than strictly as
	// "corrupt" — an unreadable format version is not corruption, and a coverage
	// figure computed while skipping a file nobody could read would be a lie of
	// the same size.
	ClassIntegrity FindingClass = "integrity"

	// ClassCoverage is exit 2: every byte present is exactly what the writer
	// wrote, and there are not enough of them. A recorded gap, a missing hour, a
	// short Alive count, an hour with no venue frame in it at all.
	ClassCoverage FindingClass = "coverage"
)

// Validate reports whether c is a declared class, excluding the zero value.
func (c FindingClass) Validate() error {
	switch c {
	case ClassUnspecified:
		return ErrFindingClassUnspecified
	case ClassIntegrity, ClassCoverage:
		return nil
	}
	return fmt.Errorf("class %q: %w", string(c), ErrFindingClassUnknown)
}

// String implements fmt.Stringer.
func (c FindingClass) String() string { return string(c) }

// FindingKind names exactly what verify found, and it is an enum rather than a
// message because §32k's positive control requires verify to "name each one
// specifically".
//
// A free-text detail alone would make that requirement untestable in the only way
// that matters: an acceptance test asserting on substrings passes when the checker
// objects for the WRONG reason, which is the same failure mode as a
// universally-objecting predicate scoring perfectly. With a kind, the corrupt-byte
// case asserts FindingChecksum and would fail if verify reported a torn tail
// instead — which it very nearly does, since errors.go's whole header note is
// about those two being confusable.
//
// Detail still exists on Finding, for the human reading the report six weeks
// later. Nothing branches on it.
type FindingKind string

const (
	// FindingUnspecified is the zero value and names nothing.
	FindingUnspecified FindingKind = ""

	// FindingChecksum is a record whose CRC32C does not match its bytes: the
	// bytes are present and at least one of them is wrong (ErrChecksum). This is
	// the bad-byte-on-a-laptop-disk case §32e's per-record CRCs exist for, and
	// it is the one that must never be reported as a torn tail.
	FindingChecksum FindingKind = "checksum"

	// FindingMalformed is a record that is present and not a record this build
	// would have written — a set reserved bit, an undeclared kind, a frame
	// length outside its bounds, an unrepresentable RecvTime.
	//
	// One kind for all of them because the operator's response is identical: the
	// bytes are wrong, go and look at the offset. The specific refusal is in
	// Detail, wrapped from the codec's own sentinel.
	FindingMalformed FindingKind = "malformed"

	// FindingFileHeader is a file that does not begin with this build's magic
	// and version. Distinct from FindingMalformed because it sends a reader
	// somewhere else: not "byte 40 000 is wrong" but "this is not our file, or
	// it is ours and this build cannot read it".
	FindingFileHeader FindingKind = "file_header"

	// FindingDigestMismatch is a SHA-256 that disagrees with the bytes — either
	// the Close record's own field, which covers everything before it, or the
	// `.sha256` sidecar, which covers the complete file.
	//
	// Both are checked, and the two are not redundant: they differ by exactly
	// one record by design (writer.go's closePayload says so), so a file that
	// grew after it was closed fails the sidecar and passes the record, which is
	// a fact no single digest can express.
	FindingDigestMismatch FindingKind = "digest_mismatch"

	// FindingDigestMissing is a cleanly closed file with no `.sha256` sidecar.
	//
	// Integrity rather than coverage, and it is the classification most worth
	// defending. Nothing about the file is wrong; what is missing is the ability
	// to CHECK it, and §32a names the laptop holding the only copy of an
	// unrecoverable-by-construction archive as the single failure that cannot be
	// fixed afterwards. A tool that reported "clean" for a file whose off-host
	// copy is unverifiable would be reporting the absence of a check as the
	// absence of a problem.
	//
	// Scoped to files that HAVE a Close record, which is what keeps it honest: a
	// file with no Close ended in a crash, and its missing sidecar is that crash
	// rather than a second fault. That case is FindingNoClose, and coverage.
	FindingDigestMissing FindingKind = "digest_missing"

	// FindingRecordCount is a Close record whose record_count disagrees with a
	// walk of the file it closes.
	//
	// The count is redundant with the walk, and CHECKING the redundancy is what
	// turns it from an opportunity to disagree into evidence — the same argument
	// path.go makes for the date directories, and Gap makes for Count against
	// From/To. It is also the one check that can catch a writer that counted this
	// run's records instead of the file's, which is a real defect the writer
	// carries a dedicated field to avoid.
	FindingRecordCount FindingKind = "record_count"

	// FindingSeqNotAscending is a Data CaptureSeq that repeats or goes
	// backwards, which is §32k's "evidence of two writers".
	//
	// Two capture processes over one root interleave their sequences into
	// something that looks like a healthy dense log from every angle except this
	// one. The flock (§32f) is what prevents it; this is the out-of-process check
	// that it was prevented, and §31g rule 4's sentence applies — a monitor that
	// has never reported a problem has not been shown able to.
	FindingSeqNotAscending FindingKind = "seq_not_ascending"

	// FindingSeqHole is a hole in the CaptureSeq sequence that a recorded gap
	// accounts for. Coverage: frames are missing and the archive says so, which
	// is the case §32a says is not an error.
	FindingSeqHole FindingKind = "seq_hole"

	// FindingSeqHoleUnexplained is a hole in the CaptureSeq sequence with no gap
	// record covering it, and it is INTEGRITY.
	//
	// §32n's roster forbids dropping without a record, ever, and §3.5 requires
	// any drop to be a recorded capture gap. A hole with no gap is therefore not
	// "some data is missing" — it is the archive's own accounting disagreeing
	// with itself, which means the capture pipeline has a defect and the run's
	// other numbers describe a process that was not doing what it claimed.
	//
	// Rejected: classing it as coverage on the grounds that the outcome is the
	// same missing frames. It is not the same: an explained gap is a fact to
	// score around, and an unexplained one is a reason to stop trusting the
	// scoring.
	FindingSeqHoleUnexplained FindingKind = "seq_hole_unexplained"

	// FindingCrossFileBreak is an Open record whose prev_last_seq names a
	// sequence that is not where its predecessor ended.
	//
	// §32e put prev_last_seq in the Open record precisely so cross-file
	// continuity is checkable without reading either file in full, and this is
	// the check it was put there for. A value of ZERO is not this finding — zero
	// is the documented "no predecessor", and with a predecessor present it is
	// FindingRestartBoundary.
	FindingCrossFileBreak FindingKind = "cross_file_break"

	// FindingStrayFile is something under the archive root that this package
	// would not have written there: a file whose path is not an archive path, a
	// sidecar with no archive file beside it, an archive file whose date
	// directories disagree with its own name.
	//
	// Integrity, because the realistic cause is a sync script that moved files —
	// which path.go's date check exists to catch, and whose failure mode is a
	// coverage report that double-counts one day and finds another empty.
	FindingStrayFile FindingKind = "stray_file"

	// FindingWindowAbsent is an expected (stream, window) with no archive file
	// at all. The verdict for that window is VerdictAbsent.
	//
	// "Expected" comes from the REQUEST's [From, To], never from the files found,
	// and that direction is the only one that works: a set of expected windows
	// derived from the files present cannot notice a deleted hour, which is one
	// of the four faults §32k's positive control requires verify to name.
	FindingWindowAbsent FindingKind = "window_absent"

	// FindingTornTail is trailing bytes that end inside a record: ErrTruncated,
	// and nothing present is wrong.
	//
	// Coverage, straight off errors.go's split — that sentinel is the only
	// refusal §32e's recovery procedure may act on by truncating, and the reason
	// it is a separate sentinel at all is so this classification can exist.
	// Expected on a live stream's current file, which is why the request's range
	// is the operator's lever for excluding it.
	FindingTornTail FindingKind = "torn_tail"

	// FindingNoClose is a file with no Close record: the run that owned it died.
	// §32g makes the ABSENCE the signal, and this is where the absence is read.
	FindingNoClose FindingKind = "no_close"

	// FindingNoData is a window whose file holds not one venue frame.
	//
	// This is the Binance-trap signature at hour granularity (§32k): a process
	// that connected, waited politely and captured nothing. The Alive ticks are
	// all present, every byte is valid, the digests match — and the hour is
	// empty. Without this finding such a window differs from a healthy one only
	// in a count nobody compared against zero.
	FindingNoData FindingKind = "no_data"

	// FindingAliveShort is a window whose unconditional Alive ticks fall below
	// MinAlivePercent of the count the rotation period and interval imply.
	//
	// The expected count is a CONSTANT derived from the two intervals, never
	// from the file's own span (writer.go's nextTick comment reserves it), and
	// that is the anti-vacuity condition: expected ticks computed from the first
	// and last record in the file would make the check pass for every file that
	// exists, including one holding a single tick.
	FindingAliveShort FindingKind = "alive_short"

	// FindingGapRecord is a gap the writer recorded, reported by cause. Coverage
	// by construction — this is the archive doing its job.
	FindingGapRecord FindingKind = "gap_record"

	// FindingRestartBoundary is a file that declares no predecessor
	// (prev_last_seq == 0) while a predecessor exists.
	//
	// A new process legitimately restarts the reader's counter, and a writer
	// opening a FRESH file has no way to know where the previous hour ended
	// without reading it — so zero is honest rather than wrong, and the
	// discontinuity is real coverage loss: the process was down across the
	// boundary. Reported so the loss is counted, not so it is blamed.
	FindingRestartBoundary FindingKind = "restart_boundary"

	// FindingContinuityUnknown is a cross-file continuity claim that could not be
	// checked, because the walk of the predecessor file stopped at a corruption
	// and so never reached the sequence it ends on.
	//
	// It exists because the alternative is silence, and a check that quietly does
	// not run is the failure this repo has now found nine times under other
	// names: a clean run and a vacuous run look identical. Coverage rather than
	// integrity, because the thing that earns exit 1 here is the corruption in
	// the predecessor, which is already reported against that file — this row
	// only records that a second check was lost to it.
	FindingContinuityUnknown FindingKind = "continuity_unknown"

	// FindingManifestMissingFile is a manifest entry whose file is GONE, and it
	// is the finding the whole manifest layer exists to make possible (§38a): a
	// deleted archive file takes its sidecar with it, so without the entry that
	// outlives it, nothing on disk contradicts anything and the loss verifies
	// clean. Integrity, emphatically — a file the manifest vouches for is
	// missing, which on a copy means the copy is incomplete and on the source
	// means the unrecoverable thing happened.
	FindingManifestMissingFile FindingKind = "manifest_missing_file"

	// FindingManifestMismatch is a manifest entry that disagrees with the file
	// it describes — digest, record count, gap count or last sequence. Integrity:
	// two close-time claims about the same bytes cannot both be true, so either
	// the file changed after its entry was written or the entry never described
	// this file.
	FindingManifestMismatch FindingKind = "manifest_mismatch"

	// FindingManifestUnreadable is a day manifest that exists and cannot be
	// read: unparseable JSON, an unknown format token, a day disagreeing with
	// the file's own name. Integrity for DigestMissing's reason, one level up:
	// nothing proves the day's files wrong, but the completeness check for that
	// whole day silently did not run, and a report that read through it would
	// present the absence of a check as the absence of a problem.
	FindingManifestUnreadable FindingKind = "manifest_unreadable"

	// FindingUnmanifested is a cleanly closed file absent from a day manifest
	// that EXISTS. Coverage: this is §38a's one-sided crash-window artifact —
	// the process died between the sidecar write and the manifest write — and
	// the file itself is intact and fully checkable. The repair is the labeled
	// backfill (§38b), which is what keeps this from being a permanent finding
	// an operator learns to read past.
	FindingUnmanifested FindingKind = "unmanifested"

	// FindingManifestAbsent is a day in range with closed files and no manifest
	// file at all — one finding per day rather than one per file, because the
	// operator's action is per-day (run the backfill), and two hundred rows
	// naming the same absence teach exactly the read-past habit §37g warns
	// about. Coverage: the expected case is an archive written before the
	// manifest layer existed.
	FindingManifestAbsent FindingKind = "manifest_absent"
)

// Class maps a kind onto the exit code it earns.
//
// exhaustiveswitch (§3.6 rule 10) binds here, and this is the switch the whole
// enum exists to force: a finding kind added without a class decision would
// otherwise fall through to a default and be reported under an exit code nobody
// chose. There is no default — the zero value is named, so a new member is a
// compile-time failure rather than a silent coverage-class finding.
func (k FindingKind) Class() FindingClass {
	switch k {
	case FindingChecksum, FindingMalformed, FindingFileHeader,
		FindingDigestMismatch, FindingDigestMissing, FindingRecordCount,
		FindingSeqNotAscending, FindingSeqHoleUnexplained,
		FindingCrossFileBreak, FindingStrayFile,
		FindingManifestMissingFile, FindingManifestMismatch,
		FindingManifestUnreadable:
		return ClassIntegrity
	case FindingWindowAbsent, FindingTornTail, FindingNoClose, FindingNoData,
		FindingAliveShort, FindingGapRecord, FindingSeqHole,
		FindingRestartBoundary, FindingContinuityUnknown,
		FindingUnmanifested, FindingManifestAbsent:
		return ClassCoverage
	case FindingUnspecified:
		return ClassUnspecified
	}
	return ClassUnspecified
}

// Validate reports whether k is a declared kind, excluding the zero value.
func (k FindingKind) Validate() error {
	if k == FindingUnspecified {
		return ErrFindingKindUnspecified
	}
	if k.Class() == ClassUnspecified {
		return fmt.Errorf("finding %q: %w", string(k), ErrFindingKindUnknown)
	}
	return nil
}

// String implements fmt.Stringer.
func (k FindingKind) String() string { return string(k) }

// Finding is one thing verify found, located precisely enough to act on.
//
// Every field that locates it is filled where it is known and left zero where it
// is not — a stray file has no window, a missing hour has no path — rather than
// split across several finding types. The consumer that matters is a person
// reading a list, and a list whose rows have the same shape is the one that can
// be sorted and diffed between runs.
type Finding struct {
	Kind FindingKind

	// Stream and Window locate the finding in the archive. Window is the
	// rotation window, not the instant of whatever went wrong.
	Stream StreamKey
	Window time.Time

	// Path is the archive file, and Offset is the byte offset within it for the
	// findings that have one (a checksum failure, a malformed record). Offset is
	// -1 when the finding is about the file as a whole.
	Path   string
	Offset int64

	// Cause is set only for FindingGapRecord: which discontinuity was recorded.
	Cause Cause

	// Detail is free text for a human. Nothing branches on it.
	Detail string
}

// Class is the exit code this finding earns, derived from its kind.
func (f Finding) Class() FindingClass { return f.Kind.Class() }

// String renders a finding as one line: class, kind, where, and why.
func (f Finding) String() string {
	where := f.Path
	if where == "" {
		where = f.Stream.String()
	}
	if f.Offset >= 0 {
		where = fmt.Sprintf("%s+%d", where, f.Offset)
	}
	return fmt.Sprintf("%s %s %s: %s", f.Class(), f.Kind, where, f.Detail)
}

// Verdict is §32k's per-(source, hour) judgement.
//
// It is about REPLAY, and it is deliberately not a third exit code. An integrity
// failure is a separate axis, reported alongside — because §32k's own structure
// is two exit codes over three verdicts, which only works if the two are not
// derived from each other. A corrupt window is VerdictGapped and carries an
// integrity finding: the first says what replay would get, the second says
// whether to believe the report at all.
type Verdict string

const (
	// VerdictUnspecified is the zero value and names no judgement.
	VerdictUnspecified Verdict = ""

	// VerdictReplayable is a window with a file, no findings, and Alive
	// coverage at or above the threshold. Step 5 can replay it as recorded.
	VerdictReplayable Verdict = "replayable"

	// VerdictGapped is a window with a file and something missing from it.
	VerdictGapped Verdict = "gapped"

	// VerdictAbsent is an expected window with no file at all — which is worse
	// than gapped and is a different thing, because a recorded gap is a fact and
	// an absent hour is the absence of any fact whatsoever.
	VerdictAbsent Verdict = "absent"
)

// Validate reports whether v is a declared verdict, excluding the zero value.
func (v Verdict) Validate() error {
	switch v {
	case VerdictUnspecified:
		return ErrVerdictUnspecified
	case VerdictReplayable, VerdictGapped, VerdictAbsent:
		return nil
	}
	return fmt.Errorf("verdict %q: %w", string(v), ErrVerdictUnknown)
}

// String implements fmt.Stringer.
func (v Verdict) String() string { return string(v) }
