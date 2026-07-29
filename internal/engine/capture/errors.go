package capture

import "errors"

// The codec's refusals. Sentinels for branching, wrapped at the site with the
// values that failed — the same rule the contexts and the venue seam follow.
//
// # The split that carries the most weight is TRUNCATED versus everything else
//
// §32e gives the writer one recovery procedure: on opening an existing hour
// file, scan forward validating CRCs, truncate to the last good record boundary,
// and write gap{cause: torn_tail}. That is the right response to a process that
// died mid-write, and the WRONG response to a bad byte on the disk — the first
// is a recorded gap, the second is an archive that is lying. §32k gives verify
// two exit codes for exactly this distinction (1 = integrity failure, 2 =
// coverage below threshold), so the codec has to hand it the distinction rather
// than a single "malformed".
//
//	ErrTruncated  the bytes end inside a record. Nothing present is wrong.
//	everything else  the bytes are present and wrong.
//
// # The residual, stated because it cannot be tested away
//
// §32e puts frameLen BEFORE the checksum field, and the checksum covers "over
// everything after this field" — so the length field is the one header field
// outside its own integrity cover. A bad byte there has three outcomes, and the
// bound check is what keeps the window small:
//
//   - implausible length (below a header, or above MaxRecord) -> ErrFrameLen
//   - plausible but wrong, and the frame is not the last one -> the checksum is
//     computed over the wrong span and fails -> ErrChecksum
//   - plausible but wrong, and the frame IS the last one -> the buffer ends
//     early -> ErrTruncated, which mis-reports corruption as a torn tail
//
// Only the third case is a real mis-diagnosis, and §32e's layered integrity is
// what covers it from outside: a cleanly closed file carries a Close record and
// a SHA-256, so corruption is caught by the digest whatever the codec called it.
// A file with no Close ended in a crash, which is where a torn tail is the
// EXPECTED finding. The residual is a crashed file whose last record's length
// field is also corrupt, and it is recorded here rather than fixed because
// fixing it means moving the checksum ahead of the length in the frame — a
// format-major change, which is the design's call and not this slice's.
// TestDecodeRecordFrameLenIsOutsideTheChecksum asserts each of the three
// outcomes, so the residual is checked behaviour rather than prose.
var (
	// ErrMagic reports bytes that do not begin an arbcap file. Distinct from
	// ErrFormatVersion because they send a reader to different places: this one
	// says "this is not our file", which is a path or a config mistake, while a
	// version mismatch says "this is ours and this build cannot read it".
	ErrMagic = errors.New("capture: not an arbcap file")

	// ErrFormatVersion reports a file this build does not implement.
	//
	// A MINOR bump is refused as well as a major one, and that is deliberate.
	// A later minor may declare record kinds this build does not know, and
	// unknown kinds are refused (see ErrKindUnknown) — so accepting the header
	// and failing at the first new record would report a corrupt-looking file
	// at an arbitrary offset instead of an unreadable version at byte 6.
	ErrFormatVersion = errors.New("capture: unsupported archive format version")

	// ErrTruncated reports bytes that end inside a record. This is the torn-tail
	// signal and the only refusal §32e's recovery procedure may act on by
	// truncating; see the note above.
	ErrTruncated = errors.New("capture: record is truncated")

	// ErrFrameLen reports a frame length that cannot be right: shorter than the
	// header it must contain, or longer than MaxRecord.
	//
	// The lower bound is not pedantry, and a run of zero bytes is why. A length
	// below 4 puts the checksum field itself outside the frame it describes; a
	// length of exactly 4 makes the checksum span EMPTY, and CRC32C of nothing
	// is the constant 0 — so eight zero bytes would decode as a valid record and
	// the reader would advance eight bytes into the middle of a real frame,
	// resynchronizing on garbage. Anything below the full header leaves fields
	// that do not exist. The upper bound is what stops a corrupt length from
	// becoming an allocation.
	ErrFrameLen = errors.New("capture: frame length is out of range")

	// ErrChecksum reports a CRC32C mismatch: the bytes are present and at least
	// one of them is wrong. This is verify's exit-1 class — the archive is
	// lying, as opposed to thin.
	ErrChecksum = errors.New("capture: record checksum does not match")

	// ErrReservedNotZero reports a set bit in the flags or reserved fields.
	//
	// They exist to be spent later, and refusing non-zero now is what makes
	// spending them possible: a reader that ignored them would let a v1.1 writer
	// hand a v1.0 reader a record whose flags change what the payload means,
	// and the v1.0 reader would return a plausible record with the flag dropped.
	// Refusing costs nothing today and is unrecoverable to add later.
	ErrReservedNotZero = errors.New("capture: reserved header bits are not zero")

	// ErrKindUnspecified and ErrKindUnknown report a record kind that is not a
	// declared one.
	//
	// Two sentinels rather than one, on venue.ErrSideUnset's reasoning: they
	// come from different mistakes and send a reader to different places. The
	// zero kind is a header nobody populated — or, on a read, a zero-filled
	// region: a sparse-file hole, or a page the OS allocated and the process
	// died before writing. That is the case that decides RecordKind's zero
	// value, because if Data were 0 the most common corruption pattern on disk
	// would decode as a valid empty data record. An unknown non-zero kind is
	// either corruption or an archive from a build this one is older than.
	ErrKindUnspecified = errors.New("capture: record kind is unset")
	ErrKindUnknown     = errors.New("capture: record kind is not a declared kind")

	// ErrRecvTimeUnset reports the zero time.Time as a record's RecvTime.
	//
	// §32i makes RecvTime the timestamp §3.4 measures freshness and cross-leg
	// skew against, taken from the INJECTED clock. The zero time is a real,
	// distant instant (year 1), so an unpopulated field would not fail — it
	// would archive every record as maximally old, which reads at replay as a
	// feed that was stale for its entire life.
	ErrRecvTimeUnset = errors.New("capture: record RecvTime is the zero time")

	// ErrRecvTimeRange reports a RecvTime outside what Unix nanoseconds can
	// hold — before 1678 or after 2262. time.Time.UnixNano() does not report
	// this; it wraps, silently, and the archive gets a plausible instant in the
	// wrong century.
	ErrRecvTimeRange = errors.New("capture: record RecvTime is not representable in Unix nanoseconds")

	// ErrMonoNegative reports a negative monotonic reading.
	//
	// Mono is nanoseconds since process start, so negative is not a small value
	// — it is a subtraction written backwards, and it is invisible afterwards
	// because nothing downstream compares Mono against anything but itself. The
	// field's whole purpose is to survive a wall-clock step (§32i), which means
	// it is consulted exactly when RecvTime is already untrustworthy.
	ErrMonoNegative = errors.New("capture: record Mono is negative")
)

// The archive layout's refusals (§32e), slice B.
var (
	// ErrTopicUnset reports a StreamKey with no venue topic.
	//
	// The empty topic is not merely incomplete: it encodes to an empty path
	// segment, so two streams that differ only in topic would share a directory
	// and interleave their CaptureSeqs into what looks like one dense log —
	// §32f's two-writer hazard reached without a second writer.
	ErrTopicUnset = errors.New("capture: stream topic is unset")

	// ErrTopicChar reports a topic containing a separator or a control
	// character. Percent-encoding would round-trip them faithfully, which is
	// exactly the problem: a topic nobody could have subscribed to would be
	// stored as a legitimate stream rather than refused at the boundary where a
	// config file can still be fixed. Same reasoning as ids' refusal of
	// whitespace in a venue symbol.
	ErrTopicChar = errors.New("capture: stream topic contains a separator or control character")

	// ErrPathShape reports a path that is not one this package would have
	// written: the wrong depth, a file name that is not a rotation window, or a
	// window whose date directories disagree with its name.
	ErrPathShape = errors.New("capture: not an archive path")

	// ErrPathEncoding reports a path segment that is not this package's
	// percent-encoding — a lowercase hex digit, a truncated escape, or an escape
	// of a character that needed none.
	//
	// Refusing the redundant escape is the one that matters: accepting "%41" for
	// "A" would give two path spellings for one stream, and the whole reason the
	// encoder and its inverse exist as a tested pair is that there is exactly
	// one.
	ErrPathEncoding = errors.New("capture: path segment is not canonically encoded")
)

// The gap taxonomy's refusals (§32g), slice B.
var (
	// ErrCauseUnspecified and ErrCauseUnknown report a gap cause that is not a
	// declared one, split for the same reason ErrKindUnspecified and
	// ErrKindUnknown are: they come from different mistakes. The zero cause is a
	// gap record nobody classified — which claims a discontinuity and refuses to
	// say what kind, and so stops anybody looking further while carrying no
	// information. An unknown non-zero cause is corruption or a newer build.
	ErrCauseUnspecified = errors.New("capture: gap cause is unset")
	ErrCauseUnknown     = errors.New("capture: gap cause is not a declared cause")

	// ErrGapRange reports a gap whose sequence range is not self-consistent, or
	// a cause that requires a range and carries none.
	//
	// §32f's drop-newest is what makes a lost run contiguous, so Count ==
	// To-From+1 is an invariant of the drop policy rather than bookkeeping. A
	// record that violates it is evidence the queue dropped in some other
	// pattern — which would mean the sequence holes elsewhere in the file no
	// longer mean what §32d says they mean.
	ErrGapRange = errors.New("capture: gap range is not self-consistent")
)

// The writer's refusals (§32e, §32f), slice B.
var (
	// ErrRootUnset, ErrPeriodRange and ErrIntervalRange report a WriterConfig
	// that was not populated. The zero value is unusable on purpose: a zero
	// rotation period would rotate on every record and a zero alive interval
	// would write a tick per pump, and both fail as an archive that is enormous
	// and healthy-looking rather than as a refusal naming the field.
	ErrRootUnset     = errors.New("capture: archive root is unset")
	ErrPeriodRange   = errors.New("capture: rotation period must be positive")
	ErrIntervalRange = errors.New("capture: interval must be positive")

	// ErrTimeSourceUnset reports a writer constructed without a clock. §3.6 rule
	// 5's whole point is that there is no ambient fallback to reach for.
	ErrTimeSourceUnset = errors.New("capture: time source is unset")

	// ErrLockHeld reports another writer already holding this stream's exclusive
	// lock, and the error text names the holder (§32f).
	//
	// Two capture processes over one root interleave CaptureSeq into something
	// that looks exactly like a healthy dense log. This is not hypothetical: the
	// per-stream layout makes "run a second process for the new source" an
	// attractive way to add a source without restarting the others.
	ErrLockHeld = errors.New("capture: another writer holds this stream's lock")

	// ErrLockUnsupported reports a platform with no advisory file locking, where
	// the single-writer invariant cannot be enforced.
	//
	// A refusal rather than a warning, because the failure it prevents is
	// silent: §32f's whole argument is that two writers produce an archive that
	// passes every check while being wrong. Running unprotected would be the
	// choice to accept that, and it is not a choice a default should make.
	ErrLockUnsupported = errors.New("capture: this platform has no advisory file locking")

	// ErrWriterClosed reports a call on a writer that has already been closed.
	ErrWriterClosed = errors.New("capture: writer is closed")

	// ErrKindNotData reports a caller handing Data() a record of another kind.
	// Every other kind is minted by the writer itself, from the injected clock,
	// and accepting one from outside would put a record in the archive whose
	// RecvTime the writer cannot vouch for.
	ErrKindNotData = errors.New("capture: only a Data record may be written from outside")

	// ErrWindowBackwards reports a record whose RecvTime falls in a rotation
	// window earlier than the open file's.
	//
	// Reachable only from a caller, not from the injected clock: platform/clock's
	// Wall clamps Now() so it never goes backwards (§32i), so a live reader's
	// stamps are non-decreasing within a run. It is refused rather than absorbed
	// because the alternative is a record sitting in a file whose NAME says a
	// different hour, and the file name is what every coverage report keys on.
	ErrWindowBackwards = errors.New("capture: record predates the open rotation window")

	// ErrQueueDepth reports a non-positive bounded-queue depth. Zero is an
	// unbuffered channel, which turns §32f's "never blocks the reader" into
	// "drops every frame the writer is not already parked waiting for" — a
	// capture that runs, reports drops honestly, and archives almost nothing.
	ErrQueueDepth = errors.New("capture: queue depth must be positive")

	// ErrSeqNotAscending reports a Data record whose CaptureSeq does not exceed
	// the last one written.
	//
	// §32d makes CaptureSeq the READER's counter, dense per stream, so a hole in
	// it means a dropped frame with no other evidence required. A repeat or a
	// reversal breaks that reading: after a reader silently restarts its
	// counter, later holes stop meaning "dropped" and start meaning nothing at
	// all — and every check downstream still passes. Refusing also forces the
	// counter to be 1-based, since zero is what an Open record's prev_last_seq
	// means by "no predecessor".
	ErrSeqNotAscending = errors.New("capture: record CaptureSeq does not ascend")

	// ErrArchiveCorrupt reports an existing hour file that this writer will not
	// append to, because recovering it would mean destroying evidence.
	//
	// This is the errors-above note made operational. A torn tail is truncated
	// and recorded as a gap; anything else — a bad checksum, a reserved bit, an
	// undeclared kind — means the bytes are present and wrong, and truncating to
	// the last good boundary would discard a corrupt region that somebody needs
	// to see. §32k's verify exists to report it, and exits 1.
	ErrArchiveCorrupt = errors.New("capture: existing archive file is corrupt, not merely truncated")
)

// Verify's refusals (§32k), slice C.
//
// Note what is NOT here: no sentinel reports a finding. A corrupt archive is a
// successful Verify run with an integrity finding in its report, because verify's
// job is to describe an archive rather than to refuse one — an error return would
// stop at the first fault and the operator needs the whole list. These sentinels
// are only for a request verify cannot answer at all.
var (
	// ErrVerifyRangeEmpty reports a [From, To] containing no COMPLETE rotation
	// window.
	//
	// A refusal rather than an empty clean report, and this is the one refusal in
	// the file that exists purely because of this repo's own history: a verify
	// run over a range with nothing in it would report zero findings, exit zero,
	// and be indistinguishable from an archive that verified. That is the vacuous
	// run this codebase has now met nine times, and here it is reachable by a
	// plausible typo in a `--from`/`--to` pair.
	//
	// Whole windows only, and the partial ones are excluded rather than
	// pro-rated: the expected Alive count for a window is a constant derived from
	// the intervals (§32k), so holding a half-elapsed hour to it would report the
	// live hour as thin on every run. Excluding it is the operator's lever, and
	// pro-rating it would be a coverage number computed from the data it is
	// supposed to judge.
	ErrVerifyRangeEmpty = errors.New("capture: verify range contains no complete rotation window")

	// ErrAlivePercentRange reports a coverage threshold outside 1..100.
	//
	// Zero is refused for the reason every zero value in this package is: it
	// would pass every window, including one holding a single Alive tick, and it
	// would do so while the report looked identical to a real check.
	ErrAlivePercentRange = errors.New("capture: alive coverage threshold must be in 1..100")

	// ErrFindingClassUnspecified and ErrFindingClassUnknown report a finding
	// class that is not a declared one. The zero class is a finding counted by
	// neither exit path — reported and then ignored, which is the same outcome as
	// not finding it.
	ErrFindingClassUnspecified = errors.New("capture: finding class is unset")
	ErrFindingClassUnknown     = errors.New("capture: finding class is not a declared class")

	// ErrFindingKindUnspecified and ErrFindingKindUnknown report a finding kind
	// that is not a declared one.
	ErrFindingKindUnspecified = errors.New("capture: finding kind is unset")
	ErrFindingKindUnknown     = errors.New("capture: finding kind is not a declared kind")

	// ErrVerdictUnspecified and ErrVerdictUnknown report a verdict that is not a
	// declared one.
	ErrVerdictUnspecified = errors.New("capture: verdict is unset")
	ErrVerdictUnknown     = errors.New("capture: verdict is not a declared verdict")
)
