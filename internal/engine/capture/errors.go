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
