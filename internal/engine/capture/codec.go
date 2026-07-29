package capture

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"time"
)

// §32e's frame, verbatim:
//
//	file  := magic ‖ record*
//	magic := "ARBCAP" ‖ u8 formatMajor(1) ‖ u8 formatMinor(0)
//
//	record := u32 frameLen        // bytes following this field; <= MaxRecord
//	        ‖ u32 crc32c          // over everything after this field
//	        ‖ u8  kind            // RecordKind
//	        ‖ u8  flags           // reserved, MUST be 0
//	        ‖ u16 reserved        // MUST be 0
//	        ‖ u64 captureSeq      // dense per (SourceID, Topic); stamped at arrival
//	        ‖ i64 recvTimeUnixNs  // from the INJECTED clock
//	        ‖ i64 monoNs          // monotonic ns since process start
//	        ‖ payload             // frameLen - 32 bytes, VERBATIM
//
// Big-endian, because the one time anybody reads this file as hex is when it is
// corrupt, and a big-endian captureSeq is legible in a hexdump.
//
// This file is written with encoding/binary and named offset constants — index
// arithmetic into a fixed header. codec_internal_test.go holds an independently
// written oracle that walks the bytes with a cursor and hand-rolled shifts, and
// computes CRC32C bitwise rather than through a table. The two share no code
// path, which is what makes their agreement evidence rather than a tautology
// (the discipline fixedpoint and ids established; ids records a golden value
// that was wrong until an oracle found it).
const (
	magic = "ARBCAP"

	// FormatMajor changes when the frame layout changes; FormatMinor when a
	// record kind or a flag is added. Both are refused when they do not match —
	// see ErrFormatVersion for why the minor is refused too.
	FormatMajor = 1
	FormatMinor = 0

	// FileHeaderLen is magic plus the two version bytes.
	FileHeaderLen = len(magic) + 2
)

// Header field offsets, measured from the first byte of the record.
const (
	offFrameLen   = 0
	offChecksum   = 4
	offKind       = 8
	offFlags      = 9
	offReserved   = 10
	offCaptureSeq = 12
	offRecvTime   = 20
	offMono       = 28

	// recordHeaderLen is the whole fixed header, frameLen included.
	recordHeaderLen = 36

	// framedHeaderLen is how much of the header frameLen counts: everything
	// after the frameLen field itself. §32e's "payload := frameLen - 32".
	framedHeaderLen = recordHeaderLen - 4
)

const (
	// MaxRecord bounds frameLen. A corrupt length must not become an
	// allocation, and 8 MiB is far above any venue frame while far below the
	// u32 the field can express.
	MaxRecord = 8 << 20

	// MaxPayload is what is left of MaxRecord after the header.
	MaxPayload = MaxRecord - framedHeaderLen
)

// crcTable is CRC32C (Castagnoli). Go's implementation uses the SSE4.2 / ARM
// CRC instructions when the CPU has them, which is a second reason the oracle
// computes it bitwise: agreement between a hardware path and a software one is
// worth having, and a wrong POLYNOMIAL is exactly the defect a round-trip test
// cannot see — a checksum that is self-consistently wrong verifies its own
// output perfectly.
var crcTable = crc32.MakeTable(crc32.Castagnoli)

// zeroHeader is appended and then written over, so the flags and reserved bytes
// are zero without a separate statement saying so.
var zeroHeader [recordHeaderLen]byte

// AppendFileHeader appends the magic and version bytes that begin every archive
// file.
func AppendFileHeader(dst []byte) []byte {
	dst = append(dst, magic...)
	return append(dst, FormatMajor, FormatMinor)
}

// ParseFileHeader validates the file header at the start of b and returns how
// many bytes it occupies.
//
// A buffer shorter than the header is ErrMagic rather than ErrTruncated, and the
// distinction is not cosmetic: ErrTruncated is the one refusal §32e's recovery
// procedure may act on by truncating the file, and a file whose first eight
// bytes are missing is not a torn tail — it is an empty or wrong file, and
// truncating it to zero would destroy nothing while reporting a gap that never
// happened.
func ParseFileHeader(b []byte) (int, error) {
	if len(b) < FileHeaderLen {
		return 0, fmt.Errorf("%d bytes, want at least %d: %w",
			len(b), FileHeaderLen, ErrMagic)
	}
	if string(b[:len(magic)]) != magic {
		return 0, fmt.Errorf("leading bytes %q: %w", b[:len(magic)], ErrMagic)
	}
	major, minor := b[len(magic)], b[len(magic)+1]
	if major != FormatMajor || minor != FormatMinor {
		return 0, fmt.Errorf("archive is v%d.%d, this build reads v%d.%d: %w",
			major, minor, FormatMajor, FormatMinor, ErrFormatVersion)
	}
	return FileHeaderLen, nil
}

// AppendRecord appends r's frame to dst and returns the extended slice.
//
// Append rather than Write, because slice A carries no I/O (§32m) — and because
// the writer that arrives in slice B needs to know a record's exact bytes before
// committing to them, in order to decide whether it crosses a rotation boundary.
//
// On error dst is returned unchanged, so a refused record cannot leave a partial
// frame in the buffer. That matters more here than the usual "return the zero
// value" politeness: a partial frame is indistinguishable from a torn tail, and
// this one would be written into the MIDDLE of a file where a torn tail cannot
// occur.
func AppendRecord(dst []byte, r Record) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return dst, fmt.Errorf("capture: encode %s: %w", r.Kind, err)
	}

	start := len(dst)
	dst = append(dst, zeroHeader[:]...)
	dst = append(dst, r.Payload...)

	// Taken after both appends, because append may reallocate: a header slice
	// captured before the payload append would be written into the old array.
	frame := dst[start:]
	binary.BigEndian.PutUint32(frame[offFrameLen:], uint32(len(frame)-4))
	frame[offKind] = byte(r.Kind)
	binary.BigEndian.PutUint64(frame[offCaptureSeq:], r.CaptureSeq)
	binary.BigEndian.PutUint64(frame[offRecvTime:], uint64(r.RecvTime.UnixNano()))
	binary.BigEndian.PutUint64(frame[offMono:], uint64(int64(r.Mono)))

	// "over everything after this field" — from kind through the last payload
	// byte. Written last, so every field it covers is already in place.
	binary.BigEndian.PutUint32(frame[offChecksum:],
		crc32.Checksum(frame[offKind:], crcTable))

	return dst, nil
}

// DecodeRecord reads the record at the start of b and returns it with the number
// of bytes it consumed, so a caller can walk a buffer by re-slicing.
//
// The order of the checks is a decision rather than an accident, and it is the
// bound check ahead of the length check that earns its place: a wildly corrupt
// frameLen would otherwise run off the end of the buffer and be reported as
// ErrTruncated — corruption wearing a torn tail's clothes, which is the one
// mis-diagnosis errors.go's note exists about. Bounding first shrinks that
// window to lengths that are merely wrong rather than absurd.
func DecodeRecord(b []byte) (Record, int, error) {
	if len(b) < 4 {
		return Record{}, 0, fmt.Errorf("%d bytes, want at least 4 for the length: %w",
			len(b), ErrTruncated)
	}
	frameLen := binary.BigEndian.Uint32(b[offFrameLen:])
	if frameLen < framedHeaderLen || frameLen > MaxRecord {
		return Record{}, 0, fmt.Errorf("frame length %d outside [%d, %d]: %w",
			frameLen, framedHeaderLen, MaxRecord, ErrFrameLen)
	}
	total := int(frameLen) + 4 // safe: frameLen <= MaxRecord
	if len(b) < total {
		return Record{}, 0, fmt.Errorf("%d bytes, want %d: %w", len(b), total, ErrTruncated)
	}
	frame := b[:total]

	if want, got := binary.BigEndian.Uint32(frame[offChecksum:]),
		crc32.Checksum(frame[offKind:], crcTable); want != got {
		return Record{}, 0, fmt.Errorf("header says %08x, bytes give %08x: %w",
			want, got, ErrChecksum)
	}
	if frame[offFlags] != 0 || binary.BigEndian.Uint16(frame[offReserved:]) != 0 {
		return Record{}, 0, fmt.Errorf("flags %02x, reserved %04x: %w",
			frame[offFlags], binary.BigEndian.Uint16(frame[offReserved:]), ErrReservedNotZero)
	}

	r := Record{
		Kind:       RecordKind(frame[offKind]),
		CaptureSeq: binary.BigEndian.Uint64(frame[offCaptureSeq:]),
		// UTC rather than Local: the archive is read on hosts other than the
		// one that wrote it, and a decoded instant whose rendering depends on
		// $TZ turns a diff of two capture reports into a timezone question.
		RecvTime: time.Unix(0, int64(binary.BigEndian.Uint64(frame[offRecvTime:]))).UTC(),
		Mono:     time.Duration(int64(binary.BigEndian.Uint64(frame[offMono:]))),
	}
	if payload := frame[recordHeaderLen:]; len(payload) > 0 {
		r.Payload = append([]byte(nil), payload...)
	}

	// The same rules the encoder applied, which is what makes an accepted frame
	// exactly a frame this package would have written. See Record.Validate.
	if err := r.Validate(); err != nil {
		return Record{}, 0, fmt.Errorf("capture: decode: %w", err)
	}
	return r, total, nil
}
