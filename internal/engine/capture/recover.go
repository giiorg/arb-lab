package capture

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

// §32e's recovery procedure:
//
//	On opening an existing hour file: scan forward validating CRCs, truncate to
//	the last good record boundary, and write gap{cause: torn_tail, bytes: n}.
//	A torn tail is a RECORDED gap like any other.
//
// The whole procedure turns on the distinction errors.go spends its header note
// on, and it is worth restating as the reason this file refuses more than it
// repairs:
//
//	ErrTruncated     the bytes end inside a record. Nothing present is wrong.
//	everything else  the bytes are present and WRONG.
//
// Truncating on the first is recovery. Truncating on the second is destroying
// the evidence of a corruption — a bad byte on a laptop disk, which §32e names as
// the second thing per-record CRCs defend against — and it would leave an archive
// that verifies clean with a hole in it. So a checksum failure, a set reserved
// bit or an undeclared kind stops the open with ErrArchiveCorrupt, and §32k's
// verify (exit 1, "the archive is lying") is what reports it.

// maxFrame is the largest a single frame can be, so a buffer of this size can
// always hold one whole record and the scanner never has to give up on a record
// it could have read.
const maxFrame = MaxRecord + 4

// scanBufInit is the starting buffer. Records are small in practice — a venue
// book update is a few kilobytes — so the buffer only grows when a real record
// needs it, and the 8 MiB ceiling is reached by a file that contains an 8 MiB
// record rather than by every open.
const scanBufInit = 64 << 10

// WalkSummary is what a walk over an existing framed file turned out to be —
// the container-level half of scanResult, exported for callers that reuse the
// frame outside the archive (the dispatch log, §39n-7). The archive-specific
// aggregates (lastSeq, hadClose, the digest) stay in scanResult, built in its
// callback, because they are claims about the ARCHIVE's kinds and a caller with
// different kind semantics would inherit them as wrong answers.
type WalkSummary struct {
	// Fresh reports a file with no bytes at all — it needs a header, not a
	// recovery. Distinct from "a header and no records", which reports
	// GoodEnd == FileHeaderLen with zero calls to the callback.
	Fresh bool

	// GoodEnd is the offset one past the last record that decoded cleanly:
	// where an appender may resume, and where a torn tail would be truncated to.
	GoodEnd int64

	// TornBytes is how many trailing bytes end inside a record. Zero for a file
	// that ended on a record boundary — which is why a torn tail is not the only
	// crash signal a format can have; see §39n-4's seal argument.
	TornBytes int64
}

// WalkRecords walks an existing framed file from the start, calling fn for each
// record that decodes cleanly. raw is the record's exact frame bytes, valid only
// for the duration of the call — a caller that keeps them must copy.
//
// The refusal split is the package's standing one: a tail that ends INSIDE a
// record is recovery (reported via TornBytes, no error), while bytes that are
// present and wrong — a checksum, a reserved bit, an undeclared kind, a
// zero-filled region — are ErrArchiveCorrupt naming the offset. An error from fn
// stops the walk and is returned wrapped with the record's offset.
//
// This is the ONE public door over the sliding-window scanner below, and
// scanArchive is rebuilt on it deliberately (§39n-7): the recovery's tricky
// property — decode only on an exhausted buffer, or every record straddling a
// read boundary reports as a torn tail — stays proved by one battery for every
// caller.
func WalkRecords(f *os.File, fn func(rec Record, raw []byte) error) (WalkSummary, error) {
	var sum WalkSummary
	st, err := f.Stat()
	if err != nil {
		return sum, fmt.Errorf("stat: %w", err)
	}
	if st.Size() == 0 {
		sum.Fresh = true
		return sum, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return sum, fmt.Errorf("seek: %w", err)
	}
	s := &scanner{f: f, backing: make([]byte, scanBufInit)}

	if err := s.ensure(); err != nil {
		return sum, err
	}
	headerLen, err := ParseFileHeader(s.pending())
	if err != nil {
		// Not recoverable and deliberately not truncated. A file whose first
		// eight bytes are wrong is not a torn tail — ParseFileHeader says so in
		// its own comment — and truncating it to zero would report a gap that
		// never happened while destroying whatever is actually there.
		return sum, fmt.Errorf("%w: %w", ErrArchiveCorrupt, err)
	}
	sum.GoodEnd = int64(headerLen)
	s.lo += headerLen

	for {
		if err := s.ensure(); err != nil {
			return sum, err
		}
		rec, n, err := DecodeRecord(s.pending())
		if err != nil {
			if !errors.Is(err, ErrTruncated) {
				return sum, fmt.Errorf("at offset %d: %w: %w", sum.GoodEnd, ErrArchiveCorrupt, err)
			}
			// ensure() has already read everything the file will give, so a
			// truncation here is final: either the file ended exactly on a
			// record boundary (nothing pending) or it ended inside one.
			sum.TornBytes = int64(len(s.pending()))
			return sum, nil
		}
		if err := fn(rec, s.pending()[:n]); err != nil {
			return sum, fmt.Errorf("at offset %d: %w", sum.GoodEnd, err)
		}
		sum.GoodEnd += int64(n)
		s.lo += n
	}
}

// scanResult is what an existing archive file turned out to be.
type scanResult struct {
	// fresh reports a file with no bytes at all, which needs a header rather
	// than a recovery. Distinct from "a header and no records", which is a file
	// a previous run created and died before writing to.
	fresh bool

	// goodEnd is the offset one past the last record that decoded cleanly. It is
	// where this run appends, and where a torn tail is truncated to.
	goodEnd int64

	// tornBytes is how many trailing bytes end inside a record. Zero for a file
	// that ended on a boundary — including an empty one.
	tornBytes int64

	// hadClose reports that the last good record was a Close, so the previous
	// run ended cleanly and this open is a restart rather than a crash recovery.
	hadClose bool

	// lastSeq is the last Data record's CaptureSeq, and records is how many
	// records the file already holds — both needed so the Close record this run
	// eventually writes describes the whole file rather than this run's share.
	// gaps counts the Gap records among them, for the same reason one level up:
	// the manifest entry (§38a) describes the whole file, and a count of this
	// run's gaps would claim a continued file recorded fewer than it did.
	lastSeq uint64
	records uint64
	gaps    uint64

	// digest is SHA-256 over [0, goodEnd), so the rolling digest continues where
	// the previous run left off rather than starting over. A digest that covered
	// only this run's records would be a digest of nothing in particular.
	digest hash.Hash
}

// scanArchive walks an existing archive file and reports what may be appended to.
//
// Built on WalkRecords (§39n-7), with the archive's own aggregates in the
// callback. The digest is seeded with the file header BEFORE the walk, because
// the callback digests frames as they stream past and a hash is sequential —
// and the header bytes a validated file carries are exactly the constant
// AppendFileHeader produces, or WalkRecords would have refused it. A fresh file
// hands back an empty digest, because the caller writes (and digests) the
// header itself in that branch.
func scanArchive(f *os.File) (scanResult, error) {
	res := scanResult{digest: sha256.New()}
	res.digest.Write(AppendFileHeader(nil))

	sum, err := WalkRecords(f, func(rec Record, raw []byte) error {
		res.digest.Write(raw)
		res.records++
		res.hadClose = rec.Kind == RecordKindClose
		if rec.Kind == RecordKindData {
			res.lastSeq = rec.CaptureSeq
		}
		if rec.Kind == RecordKindGap {
			res.gaps++
		}
		return nil
	})
	res.fresh = sum.Fresh
	res.goodEnd = sum.GoodEnd
	res.tornBytes = sum.TornBytes
	if res.fresh {
		res.digest = sha256.New()
	}
	return res, err
}

// scanner is a sliding window over a file, big enough to hold one whole record.
type scanner struct {
	f       *os.File
	backing []byte
	lo, hi  int
	eof     bool
}

func (s *scanner) pending() []byte { return s.backing[s.lo:s.hi] }

// ensure reads until the file is exhausted or the buffer is full at its ceiling,
// so a decode failure afterwards is a fact about the file rather than about how
// much happened to be buffered.
//
// That is the property the caller depends on and it is easy to lose: a scanner
// that decoded on a partially filled buffer would report a torn tail for every
// record that straddled a read boundary, and the recovery would truncate a
// perfectly good file down to its first 64 KiB.
func (s *scanner) ensure() error {
	for !s.eof {
		switch {
		case s.hi < len(s.backing):
			n, err := s.f.Read(s.backing[s.hi:])
			s.hi += n
			if err != nil {
				if errors.Is(err, io.EOF) {
					s.eof = true
					return nil
				}
				return fmt.Errorf("read: %w", err)
			}
		case s.lo > 0:
			copy(s.backing, s.pending())
			s.hi -= s.lo
			s.lo = 0
		case len(s.backing) >= maxFrame:
			return nil
		default:
			grown := make([]byte, min(len(s.backing)*2, maxFrame))
			copy(grown, s.pending())
			s.backing = grown
		}
	}
	return nil
}
