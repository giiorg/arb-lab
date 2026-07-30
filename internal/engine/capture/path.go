package capture

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §32e's directory layout:
//
//	<root>/v1/<venue>/<product>[/<contract>]/<symbol>/<stream>/<topic-slug>/
//	        2026/07/29/2026-07-29T14.arbcap
//	        2026/07/29/2026-07-29T14.arbcap.sha256      # written at close
//	<root>/v1/.../.writer.lock                          # per-stream, §32f
//
// One function derives the path and one parses it back, with a rapid round-trip
// property over both. That is §5.1's "exactly one spelling" rule made mechanical
// at the filesystem, the same way SourceID enforces it at the mailbox — and here
// it is load-bearing rather than tidy, because the path is the ONLY place the
// archive records which stream a file belongs to. A frame carries no SourceID.

const (
	// archiveExt is the file extension for an archive file, and digestExt is
	// appended to a whole archive file name for its sidecar — so the sidecar
	// sorts next to its file in any listing.
	archiveExt = ".arbcap"
	digestExt  = ".sha256"

	// lockName is the per-stream exclusive lock, §32f. A dotfile so an `ls` of a
	// stream directory shows archive files and nothing else.
	lockName = ".writer.lock"

	// dateLayout is the YYYY/MM/DD directory chain, in UTC.
	dateLayout = "2006/01/02"
)

// versionDir is the `v1` segment, and it is DERIVED from FormatMajor rather than
// written as a literal.
//
// That derivation is the point: a format-major bump moves the whole archive into
// a new tree, so a v1 reader can never be handed a v2 file by a path it walked.
// The alternative — one tree with mixed versions — makes every reader carry a
// version switch, and makes "which files can this build read" a question about
// file contents rather than about a directory name.
func versionDir() string { return "v" + strconv.Itoa(FormatMajor) }

// StreamKey identifies one capture stream. It is §32c's finding as a type:
//
//	The capture key is (SourceID, Topic), not SourceID.
//
// [SPEC EDIT LANDED — v0.9.13] Core §3.5 made the capture lane "append-only
// with per-`Source` sequence numbers"; it now says per-(Source, Topic) — design
// roster row 2, and this is where it first existed in code. The departure is FINER, so
// the drop detection §3.5 asks for still holds: every hole a per-Source counter
// would have shown is still a hole in one of these. What it adds is the ability
// to show a hole §3.5's spelling cannot, because a per-Source counter spanning
// two wire feeds has no hole when one of them stops.
//
// The archive stores wire bytes and one SourceID has more than one wire
// representation — §32c's sources 3 and 7 are the same SourceID under §3.5's
// (VenueInstrument, StreamKind) definition. The realistic accident is not that
// pair, though; it is somebody editing `@depth20@100ms` to `@depth20@250ms` in
// config six weeks in. Keyed on SourceID those two cadences land in one file with
// nothing recording the change, and the reader at step 5 finds a file whose
// message spacing doubles halfway through. Keyed on (SourceID, Topic) it is two
// directories and the change is visible from `ls`.
type StreamKey struct {
	Source events.SourceID

	// Topic is the venue's own subscription topic, verbatim — "btcusdt@markPrice@1s",
	// "tickers.BTCUSDT". Stored unmodified and encoded only on the way into a
	// path, so the value that round-trips is the one the venue was asked for.
	Topic string
}

// Validate reports whether the key could name a stream.
func (k StreamKey) Validate() error {
	if err := k.Source.Validate(); err != nil {
		return err
	}
	if k.Topic == "" {
		return ErrTopicUnset
	}
	if strings.ContainsFunc(k.Topic, func(r rune) bool { return r == '/' || r < ' ' }) {
		return fmt.Errorf("%q: %w", k.Topic, ErrTopicChar)
	}
	return nil
}

// String renders the key for logs and for lock-holder messages.
func (k StreamKey) String() string { return k.Source.String() + " " + k.Topic }

// Dir returns the directory holding every file for k.
//
// EVERY segment is percent-encoded, not just the topic — which §32e's wording
// ("topic-slug percent-encodes …") does not require and which is nonetheless the
// only version that round-trips. A venue symbol is validated against whitespace
// and nothing else (ids.VenueInstrument.Symbol says so explicitly, and gives the
// reason: folding or trimming would be a claim about venues), so a symbol
// containing a separator is a symbol this package must not turn into two
// directories. Encoding uniformly costs one function call per segment and makes
// the inverse total.
func (k StreamKey) Dir(root string) string {
	vi := k.Source.Instrument
	segs := []string{versionDir(), encodeSegment(string(vi.Venue)), encodeSegment(string(vi.Product))}
	if vi.Contract != "" {
		segs = append(segs, encodeSegment(string(vi.Contract)))
	}
	segs = append(segs,
		encodeSegment(vi.Symbol),
		encodeSegment(string(k.Source.Stream)),
		encodeSegment(k.Topic),
	)
	return filepath.Join(append([]string{root}, segs...)...)
}

// LockPath is the per-stream exclusive lock file, §32f.
//
// Per STREAM rather than per source, and the difference is §32c's again: the
// thing a lock protects is a file tree, and a file tree is keyed by (SourceID,
// Topic). A lock one level up would refuse a second writer for a DIFFERENT topic
// of the same source, which is a legitimate configuration.
func (k StreamKey) LockPath(root string) string {
	return filepath.Join(k.Dir(root), lockName)
}

// FilePath is the archive file covering the rotation window starting at `window`.
func (k StreamKey) FilePath(root string, window time.Time, period time.Duration) string {
	w := window.UTC()
	return filepath.Join(k.Dir(root), w.Format(dateLayout), fileName(w, period)+archiveExt)
}

// Window returns the start of the rotation window containing t.
//
// UTC, per §32e's "rotation is hourly on UTC RecvTime" — a local-time boundary
// would make an archive's file boundaries depend on $TZ of the host that wrote
// it, and a DST transition would produce either a 25th file or a collision.
func Window(t time.Time, period time.Duration) time.Time {
	return t.UTC().Truncate(period)
}

// fileName renders a window start at exactly the precision the period needs.
//
// §32e's example is hourly and names the file `2026-07-29T14`, which cannot
// distinguish two windows of a sub-hour period. Rather than constrain the period
// to whole hours, the name grows: a period that is a whole number of hours keeps
// §32e's spelling exactly, and shorter periods add the fields that make the name
// unique. The three layouts have three distinct lengths, so the inverse needs no
// separator counting.
//
// `.` rather than `:` between the fields, because `:` is not a legal file-name
// character everywhere the archive may be copied, and it is outside the
// unencoded set this package allows anywhere else.
func fileName(window time.Time, period time.Duration) string {
	switch {
	case period%time.Hour == 0:
		return window.Format("2006-01-02T15")
	case period%time.Minute == 0:
		return window.Format("2006-01-02T15.04")
	default:
		return window.Format("2006-01-02T15.04.05")
	}
}

// windowLayouts is fileName's inverse, keyed by rendered length.
var windowLayouts = map[int]string{
	13: "2006-01-02T15",
	16: "2006-01-02T15.04",
	19: "2006-01-02T15.04.05",
}

// ParseFilePath is Dir+FilePath's inverse: it recovers the stream and the
// rotation window from an archive path.
//
// It is what makes the layout a bijection rather than a convention, and the
// round-trip property test is the check. A one-way naming scheme would be
// perfectly serviceable until the first time a verify run has to answer "which
// stream is this file" from a directory walk — at which point the answer would
// be re-derived by a second piece of code, which is how two spellings start.
func ParseFilePath(root, p string) (StreamKey, time.Time, error) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return StreamKey{}, time.Time{}, fmt.Errorf("%q is not under %q: %w", p, root, ErrPathShape)
	}
	segs := strings.Split(filepath.ToSlash(rel), "/")

	// version, venue, product, [contract], symbol, stream, topic, Y, M, D, file
	const minSegs = 10
	if len(segs) < minSegs || len(segs) > minSegs+1 {
		return StreamKey{}, time.Time{}, fmt.Errorf("%d path segments, want %d or %d: %w",
			len(segs), minSegs, minSegs+1, ErrPathShape)
	}
	if segs[0] != versionDir() {
		return StreamKey{}, time.Time{}, fmt.Errorf("archive tree is %q, this build writes %q: %w",
			segs[0], versionDir(), ErrFormatVersion)
	}

	decoded := make([]string, len(segs))
	for i, s := range segs {
		d, err := decodeSegment(s)
		if err != nil {
			return StreamKey{}, time.Time{}, fmt.Errorf("segment %d: %w", i, err)
		}
		decoded[i] = d
	}

	// The contract segment is present exactly when the product is a derivative
	// (ids.VenueInstrument: "Contract is set exactly when Product is a
	// derivative"), so the shape is decidable from the product rather than from
	// the segment count — which is what keeps a symbol that happens to look like
	// a contract type from shifting the whole parse by one.
	vi := ids.VenueInstrument{
		Venue:   ids.Venue(decoded[1]),
		Product: ids.ProductCategory(decoded[2]),
	}
	rest := decoded[3:]
	if vi.Product != ids.ProductSpot {
		vi.Contract = ids.ContractType(rest[0])
		rest = rest[1:]
	}
	// symbol, stream, topic, YYYY, MM, DD, file
	if len(rest) != 7 {
		return StreamKey{}, time.Time{}, fmt.Errorf(
			"%d segments after the product for a %s instrument: %w", len(rest), vi.Product, ErrPathShape)
	}
	vi.Symbol = rest[0]

	key := StreamKey{
		Source: events.SourceID{Instrument: vi, Stream: events.StreamKind(rest[1])},
		Topic:  rest[2],
	}
	if err := key.Validate(); err != nil {
		return StreamKey{}, time.Time{}, err
	}

	base := rest[6]
	name, ok := strings.CutSuffix(base, archiveExt)
	if !ok {
		return StreamKey{}, time.Time{}, fmt.Errorf("%q does not end in %s: %w",
			base, archiveExt, ErrPathShape)
	}
	layout, ok := windowLayouts[len(name)]
	if !ok {
		return StreamKey{}, time.Time{}, fmt.Errorf("%q is not a window name: %w", name, ErrPathShape)
	}
	window, err := time.ParseInLocation(layout, name, time.UTC)
	if err != nil {
		return StreamKey{}, time.Time{}, fmt.Errorf("%q: %w: %w", name, ErrPathShape, err)
	}

	// The date directories are redundant with the file name, and CHECKING that
	// redundancy is what turns it from an opportunity to disagree into evidence.
	// A file moved into the wrong day's directory by a sync script is otherwise
	// reported under the day its name claims, silently, and a coverage report
	// that reads the tree by directory would then double-count one day and find
	// another empty.
	if got, want := path.Join(rest[3], rest[4], rest[5]), window.Format(dateLayout); got != want {
		return StreamKey{}, time.Time{}, fmt.Errorf(
			"file %q sits under %q, which is not its window's date %q: %w",
			base, got, want, ErrPathShape)
	}
	return key, window, nil
}

// unreserved is the set left alone by encodeSegment: RFC 3986's unreserved
// characters minus `~`, which is the set §32e names.
func unreserved(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	case b == '.' || b == '_' || b == '-':
		return true
	}
	return false
}

// encodeSegment percent-encodes every byte outside the unreserved set, plus a
// LEADING dot.
//
// Byte-wise rather than rune-wise, so a topic carrying non-UTF-8 bytes encodes
// and decodes exactly rather than through a replacement character. `%` is itself
// outside the set and therefore escapes to `%25`, which is what makes the
// mapping injective — an encoder that left `%` alone would map the two distinct
// topics "a%2Fb" and "a/b" onto one directory.
//
// # The leading dot is not tidiness, and the round-trip property is what found it
//
// `.` is in §32e's unencoded set, so a topic of "." or ".." encoded to itself —
// and filepath.Join CLEANS those away. A topic of "." silently produced the path
// of a different stream, and a topic of ".." climbed a level out of the archive
// tree, with the encoder, the decoder and every hand-written fixture agreeing
// that nothing had happened. Escaping only the leading dot fixes both while
// keeping the readable form §32c's "visible from `ls`" argument depends on:
// "tickers.BTCUSDT" is unchanged, and only "." ".." and ".hidden" move.
//
// It buys one more thing for free: no stream directory can collide with a
// dotfile, so `.writer.lock` and the archive tree can never be confused for one
// another however a topic is spelled.
//
// Note what is deliberately NOT done: case is preserved. On a case-insensitive
// filesystem two symbols differing only in case share a directory, and their
// CaptureSeqs interleave into something that looks like a healthy dense log —
// which is exactly §32f's two-writer hazard arriving through a different door,
// and is covered by the same mechanism: the second writer is refused by the
// exclusive lock, by name, rather than silently joining the first.
func encodeSegment(s string) string {
	need := strings.HasPrefix(s, ".")
	for i := range len(s) {
		if !unreserved(s[i]) {
			need = true
			break
		}
	}
	if !need {
		return s
	}
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := range len(s) {
		if c := s[i]; unreserved(c) && !(i == 0 && c == '.') {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

// decodeSegment is encodeSegment's inverse.
//
// It refuses what the encoder would never emit: a lowercase hex digit, a stray
// `%`, an escape of a character that did not need escaping, and a LITERAL
// leading dot. That last refusal is what makes the traversal fix hold in both
// directions — a path segment of ".." reaching this function is a path this
// package did not write, and decoding it would hand a caller a stream key
// derived from a directory outside the archive.
//
// The redundant-escape refusal has the matching carve-out: `%2E` is legal at
// position 0 and nowhere else, exactly mirroring where the encoder emits it.
// Without the carve-out the encoder's own output for a leading dot would be
// rejected by its inverse; without the position limit there would again be two
// spellings for one stream, which is the whole reason the pair exists.
func decodeSegment(s string) (string, error) {
	if strings.HasPrefix(s, ".") {
		return "", fmt.Errorf("%q: a leading dot is escaped by the encoder: %w", s, ErrPathEncoding)
	}
	i := strings.IndexByte(s, '%')
	if i < 0 {
		return s, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		c := s[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		if i+2 >= len(s) {
			return "", fmt.Errorf("%q: truncated escape: %w", s, ErrPathEncoding)
		}
		hi, lo := unhex(s[i+1]), unhex(s[i+2])
		if hi < 0 || lo < 0 {
			return "", fmt.Errorf("%q: %q is not an uppercase hex escape: %w",
				s, s[i:i+3], ErrPathEncoding)
		}
		v := byte(hi<<4 | lo)
		if unreserved(v) && !(i == 0 && v == '.') {
			return "", fmt.Errorf("%q: %q escapes a character that needs no escape: %w",
				s, s[i:i+3], ErrPathEncoding)
		}
		b.WriteByte(v)
		i += 2
	}
	return b.String(), nil
}

// unhex accepts only the digits and UPPERCASE letters encodeSegment emits, so
// "%2f" and "%2F" cannot both decode. Two spellings for one byte is two path
// spellings for one stream.
func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
