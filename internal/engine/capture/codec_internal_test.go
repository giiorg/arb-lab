package capture

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"math"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// An oracle for the frame codec, in the discipline fixedpoint and ids
// established: the implementation is checked against an independently written
// second one that shares no code path with it, rather than against itself.
//
// codec.go indexes a fixed header through named offset constants, assembles
// integers with encoding/binary, and checksums through a table that Go compiles
// to the SSE4.2 / ARM CRC instruction where one exists. Everything below walks
// the bytes with a cursor, assembles integers with hand-rolled shifts, derives
// the frame length from len(body) rather than from a constant, and computes
// CRC32C one bit at a time. An off-by-one in the offset table does not exist in
// a cursor; a wrong framedHeaderLen does not exist in a length derived from what
// was actually appended.
//
// The one thing an oracle cannot check is whether BOTH are wrong in the same
// way, which for a checksum is the realistic case — a wrong polynomial verifies
// its own output perfectly. TestCRC32CMatchesTheStandardCheckValue pins both to
// a value published outside this repo.

// oracleCRC32C is CRC-32C (Castagnoli) computed bitwise: reflected input and
// output, polynomial 0x1EDC6F41 in its reflected form 0x82F63B78, initial and
// final value all-ones.
func oracleCRC32C(b []byte) uint32 {
	const reflectedPoly = 0x82F63B78
	crc := ^uint32(0)
	for _, c := range b {
		crc ^= uint32(c)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ reflectedPoly
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// oracleAppendBE appends v as width bytes, most significant first.
func oracleAppendBE(dst []byte, v uint64, width int) []byte {
	for i := width - 1; i >= 0; i-- {
		dst = append(dst, byte(v>>(8*i)))
	}
	return dst
}

// oracleEncode builds a frame sequentially. Note that the length it writes is
// derived from what it appended, not from a header-size constant.
func oracleEncode(r Record) []byte {
	body := []byte{byte(r.Kind), 0 /*flags*/, 0, 0 /*reserved*/}
	body = oracleAppendBE(body, r.CaptureSeq, 8)
	body = oracleAppendBE(body, uint64(r.RecvTime.UnixNano()), 8)
	body = oracleAppendBE(body, uint64(int64(r.Mono)), 8)
	body = append(body, r.Payload...)

	out := oracleAppendBE(nil, uint64(4+len(body)), 4) // frameLen counts the crc too
	out = oracleAppendBE(out, uint64(oracleCRC32C(body)), 4)
	return append(out, body...)
}

// oracleDecode walks a frame with a cursor. It reproduces DecodeRecord's ORDER
// of refusals, because the two have to agree on which sentinel an input earns —
// but it shares none of its arithmetic.
func oracleDecode(b []byte) (Record, int, error) {
	cur := 0
	take := func(n int) ([]byte, bool) {
		if cur+n > len(b) {
			return nil, false
		}
		out := b[cur : cur+n]
		cur += n
		return out, true
	}
	be := func(p []byte) uint64 {
		var v uint64
		for _, c := range p {
			v = v<<8 | uint64(c)
		}
		return v
	}

	lenBytes, ok := take(4)
	if !ok {
		return Record{}, 0, ErrTruncated
	}
	frameLen := be(lenBytes)
	if frameLen < 32 || frameLen > MaxRecord {
		return Record{}, 0, ErrFrameLen
	}
	rest, ok := take(int(frameLen))
	if !ok {
		return Record{}, 0, ErrTruncated
	}
	if be(rest[:4]) != uint64(oracleCRC32C(rest[4:])) {
		return Record{}, 0, ErrChecksum
	}

	body := rest[4:]
	if body[1] != 0 || body[2] != 0 || body[3] != 0 {
		return Record{}, 0, ErrReservedNotZero
	}
	r := Record{
		Kind:       RecordKind(body[0]),
		CaptureSeq: be(body[4:12]),
		RecvTime:   time.Unix(0, int64(be(body[12:20]))).UTC(),
		Mono:       time.Duration(int64(be(body[20:28]))),
	}
	if payload := body[28:]; len(payload) > 0 {
		r.Payload = append([]byte(nil), payload...)
	}
	if err := r.Kind.Validate(); err != nil {
		return Record{}, 0, err
	}
	if r.Mono < 0 {
		return Record{}, 0, ErrMonoNegative
	}
	return r, cur, nil
}

// TestCRC32CMatchesTheStandardCheckValue anchors both implementations to a
// number produced outside this repository.
//
// This is the check the oracle by itself cannot make. A polynomial typo shared
// by codec.go and oracleCRC32C would round-trip perfectly, agree with itself on
// every input, and produce an archive whose integrity field detects nothing —
// while every test in this file passed. 0xE3069283 is CRC-32C's published check
// value for the nine ASCII digits, the same constant the Castagnoli entry in
// every CRC catalogue carries.
func TestCRC32CMatchesTheStandardCheckValue(t *testing.T) {
	const check = "123456789"
	const want = uint32(0xE3069283)

	if got := crc32.Checksum([]byte(check), crcTable); got != want {
		t.Errorf("crcTable over %q = %08x, want the published CRC-32C check value %08x",
			check, got, want)
	}
	if got := oracleCRC32C([]byte(check)); got != want {
		t.Errorf("oracleCRC32C(%q) = %08x, want %08x", check, got, want)
	}
}

// drawRecord generates a record the encoder must accept. RecvTime is drawn as
// an arbitrary int64 of Unix nanoseconds so the whole representable span is
// reachable, including negative (pre-1970) instants — which are legal, wrong for
// a live capture, and exactly the kind of value a replayed archive from a
// mis-set host would carry.
func drawRecord(rt *rapid.T) Record {
	return Record{
		Kind:       RecordKind(rapid.IntRange(int(RecordKindOpen), int(maxRecordKind)).Draw(rt, "kind")),
		CaptureSeq: rapid.Uint64().Draw(rt, "captureSeq"),
		RecvTime:   time.Unix(0, rapid.Int64().Draw(rt, "recvNs")).UTC(),
		Mono:       time.Duration(rapid.Int64Range(0, math.MaxInt64).Draw(rt, "monoNs")),
		Payload:    rapid.SliceOfN(rapid.Uint8(), 0, 512).Draw(rt, "payload"),
	}
}

// recordsEqual compares by INSTANT rather than by ==. RecvTime is encoded as
// Unix nanoseconds, so the monotonic reading and the location do not survive the
// round trip; the instant does, exactly.
func recordsEqual(a, b Record) bool {
	return a.Kind == b.Kind &&
		a.CaptureSeq == b.CaptureSeq &&
		a.RecvTime.Equal(b.RecvTime) &&
		a.Mono == b.Mono &&
		bytes.Equal(a.Payload, b.Payload)
}

func TestProperty_RecordAgreesWithTheOracle(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		r := drawRecord(rt)

		got, err := AppendRecord(nil, r)
		if err != nil {
			rt.Fatalf("AppendRecord(%+v): %v", r, err)
		}
		if want := oracleEncode(r); !bytes.Equal(got, want) {
			rt.Fatalf("encode disagrees with the oracle:\n got %s\nwant %s",
				hex.EncodeToString(got), hex.EncodeToString(want))
		}

		back, n, err := DecodeRecord(got)
		if err != nil {
			rt.Fatalf("DecodeRecord(%s): %v", hex.EncodeToString(got), err)
		}
		if n != len(got) {
			rt.Fatalf("DecodeRecord consumed %d of %d bytes", n, len(got))
		}
		if !recordsEqual(back, r) {
			rt.Fatalf("round trip: %+v -> %+v", r, back)
		}

		oracleBack, oracleN, err := oracleDecode(got)
		if err != nil {
			rt.Fatalf("oracleDecode: %v", err)
		}
		if oracleN != n || !recordsEqual(oracleBack, r) {
			rt.Fatalf("oracle round trip: %+v -> %+v (%d bytes, real decoder said %d)",
				r, oracleBack, oracleN, n)
		}
	})
}

// TestProperty_AppendRecordExtendsWithoutDisturbing is the property a writer
// depends on and a round-trip test does not reach: appending record N+1 must not
// touch record N's bytes, and the buffer's existing prefix must survive.
//
// The fixture varies the number of records AND their payload lengths. Fixing
// either would collapse a distinction: with one record, "advance by the frame's
// length" and "advance by a constant" are the same; with equal-length records,
// they still are.
func TestProperty_AppendRecordExtendsWithoutDisturbing(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		prefix := rapid.SliceOfN(rapid.Uint8(), 0, 16).Draw(rt, "prefix")
		records := rapid.SliceOfN(rapid.Custom(drawRecord), 2, 6).Draw(rt, "records")

		buf := append([]byte(nil), prefix...)
		var offsets []int
		for _, r := range records {
			offsets = append(offsets, len(buf))
			var err error
			if buf, err = AppendRecord(buf, r); err != nil {
				rt.Fatalf("AppendRecord: %v", err)
			}
		}
		if !bytes.Equal(buf[:len(prefix)], prefix) {
			rt.Fatalf("the prefix was disturbed: %x -> %x", prefix, buf[:len(prefix)])
		}

		// Each record still decodes from its own offset, and walking forward by
		// the reported consumption lands on the next one.
		cur := len(prefix)
		for i, r := range records {
			if cur != offsets[i] {
				rt.Fatalf("record %d starts at %d, walking landed at %d", i, offsets[i], cur)
			}
			back, n, err := DecodeRecord(buf[cur:])
			if err != nil {
				rt.Fatalf("record %d at %d: %v", i, cur, err)
			}
			if !recordsEqual(back, r) {
				rt.Fatalf("record %d: %+v -> %+v", i, r, back)
			}
			cur += n
		}
		if cur != len(buf) {
			rt.Fatalf("walked to %d of %d bytes", cur, len(buf))
		}
	})
}

// goldenRecord and goldenFrame pin the LAYOUT: which field sits at which offset,
// in which byte order.
//
// Every field value is chosen so that each of its bytes is distinct and
// non-zero, which is the fixture-collapse point here. With CaptureSeq = 1,
// RecvTime = 2 and Mono = 3 the high seven bytes of each field are zero, so
// reading a u32 where a u64 was written, or reading the wrong field's offset,
// produces the identical frame — and a golden test full of small numbers asserts
// almost nothing.
var (
	goldenRecord = Record{
		Kind:       RecordKindGap,
		CaptureSeq: 0x0102030405060708,
		RecvTime:   time.Unix(0, 0x1122334455667788).UTC(),
		Mono:       time.Duration(0x2a2b2c2d2e2f3031),
		Payload:    []byte("BTC"),
	}
	goldenFrame = []byte{
		0x00, 0x00, 0x00, 0x23, // frameLen: 32 header bytes + 3 of payload
		0x00, 0x00, 0x00, 0x00, // crc32c, filled in below
		0x04,       // kind: RecordKindGap
		0x00,       // flags
		0x00, 0x00, // reserved
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, // captureSeq
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, // recvTimeUnixNs
		0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, // monoNs
		'B', 'T', 'C',
	}
)

func TestAppendRecordGoldenLayout(t *testing.T) {
	// The checksum is the one field a table cannot carry by hand, so it comes
	// from the oracle — which the standard check value above has already pinned
	// to a constant published outside this repo. A wrong expectation here
	// therefore cannot agree with a wrong implementation, which is the failure
	// ids records: its first golden ClientOrderID was a hand-encoding, and it
	// was wrong.
	want := append([]byte(nil), goldenFrame...)
	binary.BigEndian.PutUint32(want[4:], oracleCRC32C(want[8:]))

	got, err := AppendRecord(nil, goldenRecord)
	if err != nil {
		t.Fatalf("AppendRecord: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("frame layout:\n got %s\nwant %s",
			hex.EncodeToString(got), hex.EncodeToString(want))
	}
	if got[4] == 0 && got[5] == 0 && got[6] == 0 && got[7] == 0 {
		t.Error("the checksum field is all zeros; this golden would pass against " +
			"an encoder that never writes one")
	}

	back, n, err := DecodeRecord(want)
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if n != len(want) || !recordsEqual(back, goldenRecord) {
		t.Errorf("decode: %+v (%d bytes), want %+v (%d)", back, n, goldenRecord, len(want))
	}
}

// goldenBytes is the golden frame with a valid checksum: the base every
// rejection fixture below mutates.
func goldenBytes() []byte {
	b := append([]byte(nil), goldenFrame...)
	binary.BigEndian.PutUint32(b[4:], oracleCRC32C(b[8:]))
	return b
}

// reframe applies mutate to a copy of the golden frame and then RESTORES the
// checksum.
//
// Restoring it is the whole point, and it is this package's version of the
// isolating-probe rule §3.6's singlewriter note states: a fixture that flips the
// flags byte and leaves the checksum stale is refused by the checksum, so the
// test passes while showing that SOMETHING objected rather than that the rule
// under test did. Every semantic rejection below goes through here; the
// checksum's own coverage is TestChecksumCoversEveryHeaderField, which
// deliberately does not.
func reframe(mutate func([]byte)) []byte {
	b := goldenBytes()
	mutate(b)
	if len(b) >= 8 {
		binary.BigEndian.PutUint32(b[4:], oracleCRC32C(b[8:]))
	}
	return b
}

func TestDecodeRecordRejects(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want error
	}{
		// Truncation. Three positions, because "ends inside the header" and
		// "ends inside the payload" are different code paths, and the boundary
		// case is where truncation and cleanliness collapse into one another —
		// see the accepted-boundary control below.
		{"empty", nil, ErrTruncated},
		{"inside the length field", goldenBytes()[:3], ErrTruncated},
		{"exactly the length field", goldenBytes()[:4], ErrTruncated},
		{"inside the header", goldenBytes()[:20], ErrTruncated},
		{"inside the payload", goldenBytes()[:len(goldenFrame)-1], ErrTruncated},

		// The frame-length bound, at both edges and one step either side, so
		// `<` and `<=` are distinguishable.
		{"length one below the header", reframe(func(b []byte) {
			binary.BigEndian.PutUint32(b, framedHeaderLen-1)
		}), ErrFrameLen},
		{"length zero", reframe(func(b []byte) {
			binary.BigEndian.PutUint32(b, 0)
		}), ErrFrameLen},
		{"length one above MaxRecord", reframe(func(b []byte) {
			binary.BigEndian.PutUint32(b, MaxRecord+1)
		}), ErrFrameLen},
		{"length at the u32 ceiling", reframe(func(b []byte) {
			binary.BigEndian.PutUint32(b, math.MaxUint32)
		}), ErrFrameLen},

		// Reserved bits, one field at a time. Setting flags and reserved
		// together would make "checks flags" and "checks reserved" the same
		// assertion, and the reserved field is two bytes, so each is set alone.
		{"flags set", reframe(func(b []byte) { b[offFlags] = 0x01 }), ErrReservedNotZero},
		{"flags all set", reframe(func(b []byte) { b[offFlags] = 0xff }), ErrReservedNotZero},
		{"reserved high byte set", reframe(func(b []byte) { b[offReserved] = 0x01 }), ErrReservedNotZero},
		{"reserved low byte set", reframe(func(b []byte) { b[offReserved+1] = 0x01 }), ErrReservedNotZero},

		// Record kind. The zero case is the one that matters on a read: a
		// zero-filled region of a file must not decode as a valid record.
		{"kind unspecified", reframe(func(b []byte) { b[offKind] = 0 }), ErrKindUnspecified},
		{"kind one past the last declared", reframe(func(b []byte) {
			b[offKind] = byte(maxRecordKind) + 1
		}), ErrKindUnknown},
		{"kind at the byte ceiling", reframe(func(b []byte) { b[offKind] = 0xff }), ErrKindUnknown},

		// Mono. Negative is a subtraction written backwards; -1 rather than a
		// large negative, because that is the value a `>` written as `>=`
		// produces.
		{"mono negative by one", reframe(func(b []byte) {
			binary.BigEndian.PutUint64(b[offMono:], math.MaxUint64)
		}), ErrMonoNegative},
		{"mono at the int64 floor", reframe(func(b []byte) {
			binary.BigEndian.PutUint64(b[offMono:], 1<<63)
		}), ErrMonoNegative},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, n, err := DecodeRecord(tc.in)
			if err == nil {
				t.Fatalf("DecodeRecord = nil error, want %v", tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("DecodeRecord = %v, want %v", err, tc.want)
			}
			if n != 0 {
				t.Errorf("DecodeRecord consumed %d bytes on a refusal; a rejected "+
					"frame has no length, and a caller that advanced by it would "+
					"resynchronize on garbage", n)
			}

			// The oracle earns its place on the refusals too: agreeing about
			// which inputs are valid is most of a codec's contract.
			if _, _, oracleErr := oracleDecode(tc.in); !errors.Is(oracleErr, tc.want) {
				t.Errorf("oracleDecode = %v, want %v", oracleErr, tc.want)
			}
		})
	}

	// The positive controls, without which a decoder that refused everything
	// would score perfectly on the table above.
	t.Run("accepts the boundary cases the table brackets", func(t *testing.T) {
		accepted := []struct {
			name string
			in   []byte
		}{
			// Exactly one whole frame, no trailing byte: the case that
			// distinguishes truncation from a clean end.
			{"exactly one frame", goldenBytes()},
			{"mono zero", reframe(func(b []byte) {
				binary.BigEndian.PutUint64(b[offMono:], 0)
			})},
			{"mono at the int64 ceiling", reframe(func(b []byte) {
				binary.BigEndian.PutUint64(b[offMono:], math.MaxInt64)
			})},
			{"kind at the last declared value", reframe(func(b []byte) {
				b[offKind] = byte(maxRecordKind)
			})},
			{"kind at the first declared value", reframe(func(b []byte) {
				b[offKind] = byte(RecordKindOpen)
			})},
		}
		for _, tc := range accepted {
			if _, _, err := DecodeRecord(tc.in); err != nil {
				t.Errorf("%s: DecodeRecord = %v, want it accepted", tc.name, err)
			}
		}
	})
}

// TestDecodeRecordEmptyPayload covers frameLen at its minimum — the Alive tick,
// which is the record kind with no payload at all and therefore the one where
// `frameLen - 32` is zero.
//
// It is here rather than folded into the table because a nil and an empty
// payload must produce IDENTICAL bytes: the format offers one spelling for "no
// payload", the same "exactly one spelling" rule ids enforces for identifiers
// and §32e's path round-trip enforces at the filesystem.
func TestDecodeRecordEmptyPayload(t *testing.T) {
	base := Record{
		Kind:       RecordKindAlive,
		CaptureSeq: 0x0807060504030201,
		RecvTime:   time.Unix(0, 0x0102030405060708).UTC(),
		Mono:       time.Duration(0x1112131415161718),
	}
	nilPayload, err := AppendRecord(nil, base)
	if err != nil {
		t.Fatalf("AppendRecord (nil payload): %v", err)
	}
	withEmpty := base
	withEmpty.Payload = []byte{}
	emptyPayload, err := AppendRecord(nil, withEmpty)
	if err != nil {
		t.Fatalf("AppendRecord (empty payload): %v", err)
	}
	if !bytes.Equal(nilPayload, emptyPayload) {
		t.Errorf("a nil and an empty payload encode differently:\n%s\n%s",
			hex.EncodeToString(nilPayload), hex.EncodeToString(emptyPayload))
	}
	if got := len(nilPayload); got != recordHeaderLen {
		t.Errorf("an empty record is %d bytes, want %d", got, recordHeaderLen)
	}
	if got := binary.BigEndian.Uint32(nilPayload); got != framedHeaderLen {
		t.Errorf("frameLen = %d, want the header minimum %d", got, framedHeaderLen)
	}

	back, n, err := DecodeRecord(nilPayload)
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if n != recordHeaderLen {
		t.Errorf("consumed %d bytes, want %d", n, recordHeaderLen)
	}
	if back.Payload != nil {
		t.Errorf("Payload = %v, want nil for a record that carries none", back.Payload)
	}
}

// TestChecksumCoversEveryHeaderField is the fixture-collapse case that matters
// most in this file.
//
// A test that corrupts only a payload byte passes against a checksum computed
// over the payload alone — and such a checksum would leave every header field
// unprotected while reporting a healthy archive. So a byte is flipped in each
// field region in turn, WITHOUT restoring the checksum, and each must be caught.
//
// frameLen is excluded and that exclusion is the format's known residual, so it
// gets its own test below rather than a silent absence here.
func TestChecksumCoversEveryHeaderField(t *testing.T) {
	regions := map[string]int{
		"kind":       offKind,
		"flags":      offFlags,
		"reserved":   offReserved,
		"captureSeq": offCaptureSeq,
		"recvTime":   offRecvTime,
		"mono":       offMono,
		"payload":    recordHeaderLen,
	}
	for name, at := range regions {
		t.Run(name, func(t *testing.T) {
			b := goldenBytes()
			b[at] ^= 0x01 // one bit, the smallest corruption a disk produces
			if _, _, err := DecodeRecord(b); !errors.Is(err, ErrChecksum) {
				t.Errorf("flipping a bit at offset %d (%s) gave %v, want %v",
					at, name, err, ErrChecksum)
			}
		})
	}

	// The positive control. Without it, a decoder that reported ErrChecksum
	// unconditionally would pass every case above.
	if _, _, err := DecodeRecord(goldenBytes()); err != nil {
		t.Errorf("the unmodified golden frame was refused: %v", err)
	}
}

// TestDecodeRecordFrameLenIsOutsideTheChecksum turns errors.go's residual note
// into checked behaviour.
//
// §32e puts frameLen before the checksum field and the checksum covers only what
// follows itself, so the length is the one header field with no integrity cover.
// The three outcomes are asserted here so that a later change to the frame — or
// to the bound — cannot quietly widen the window in which corruption is reported
// as a torn tail.
func TestDecodeRecordFrameLenIsOutsideTheChecksum(t *testing.T) {
	// Implausible: caught by the bound, before anything reads past the buffer.
	implausible := goldenBytes()
	binary.BigEndian.PutUint32(implausible, math.MaxUint32)
	if _, _, err := DecodeRecord(implausible); !errors.Is(err, ErrFrameLen) {
		t.Errorf("an absurd length gave %v, want %v", err, ErrFrameLen)
	}

	// Plausible but wrong, with more bytes available: the checksum is computed
	// over the wrong span and fails, so this reports as corruption.
	twoFrames := append(goldenBytes(), goldenBytes()...)
	binary.BigEndian.PutUint32(twoFrames, framedHeaderLen+2)
	if _, _, err := DecodeRecord(twoFrames); !errors.Is(err, ErrChecksum) {
		t.Errorf("a short length inside a longer buffer gave %v, want %v", err, ErrChecksum)
	}

	// Plausible but wrong on the LAST frame: the buffer ends early and the
	// refusal is ErrTruncated. This is the documented mis-diagnosis — corruption
	// wearing a torn tail's clothes — and §32e's per-file digest is what covers
	// it from outside. Asserted so that it is a known property rather than a
	// surprise found at recovery time.
	lastFrame := goldenBytes()
	binary.BigEndian.PutUint32(lastFrame, uint32(len(lastFrame)))
	if _, _, err := DecodeRecord(lastFrame); !errors.Is(err, ErrTruncated) {
		t.Errorf("a long length on the last frame gave %v, want %v — "+
			"if this changed, errors.go's residual note is stale", err, ErrTruncated)
	}
}

func TestFileHeaderRoundTrip(t *testing.T) {
	header := AppendFileHeader(nil)
	if len(header) != FileHeaderLen {
		t.Fatalf("AppendFileHeader wrote %d bytes, want %d", len(header), FileHeaderLen)
	}
	if got := string(header[:len(magic)]); got != magic {
		t.Errorf("magic = %q, want %q", got, magic)
	}
	n, err := ParseFileHeader(header)
	if err != nil {
		t.Fatalf("ParseFileHeader: %v", err)
	}
	if n != FileHeaderLen {
		t.Errorf("ParseFileHeader consumed %d bytes, want %d", n, FileHeaderLen)
	}

	// A header followed by a record: ParseFileHeader must report only its own
	// length, or a caller walking the file starts the first record at the wrong
	// offset.
	file := AppendFileHeader(nil)
	file = append(file, goldenBytes()...)
	if n, err := ParseFileHeader(file); err != nil || n != FileHeaderLen {
		t.Errorf("ParseFileHeader over a whole file = %d, %v; want %d, nil",
			n, err, FileHeaderLen)
	}
}

func TestParseFileHeaderRejects(t *testing.T) {
	good := AppendFileHeader(nil)

	// The magic's wrong byte is varied across FIRST, MIDDLE and LAST position.
	// A fixture that only corrupts the middle passes against a check that
	// compares one end — outbox recorded exactly that: a malformed-identifier
	// fixture with the bad character mid-string made `i >= 0` and `i > 0` the
	// same predicate, so " audit" validated.
	corrupt := func(at int) []byte {
		b := append([]byte(nil), good...)
		b[at] = 'X'
		return b
	}

	cases := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, ErrMagic},
		{"one byte short", good[:FileHeaderLen-1], ErrMagic},
		{"magic wrong at the first byte", corrupt(0), ErrMagic},
		{"magic wrong in the middle", corrupt(len(magic) / 2), ErrMagic},
		{"magic wrong at the last byte", corrupt(len(magic) - 1), ErrMagic},

		// Major and minor are wrong one at a time, so "checks the major" and
		// "checks the minor" cannot be the same assertion.
		{"major ahead", corruptVersion(good, FormatMajor+1, FormatMinor), ErrFormatVersion},
		{"major behind", corruptVersion(good, FormatMajor-1, FormatMinor), ErrFormatVersion},
		{"minor ahead", corruptVersion(good, FormatMajor, FormatMinor+1), ErrFormatVersion},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParseFileHeader(tc.in)
			if !errors.Is(err, tc.want) {
				t.Errorf("ParseFileHeader = %v, want %v", err, tc.want)
			}
			if n != 0 {
				t.Errorf("ParseFileHeader consumed %d bytes on a refusal", n)
			}
		})
	}

	// The positive control: the real header is accepted. Without it every case
	// above is satisfied by a function that returns ErrMagic unconditionally.
	if _, err := ParseFileHeader(good); err != nil {
		t.Errorf("the header this package writes was refused by its own parser: %v", err)
	}
}

func corruptVersion(good []byte, major, minor byte) []byte {
	b := append([]byte(nil), good...)
	b[len(magic)], b[len(magic)+1] = major, minor
	return b
}

// TestAppendRecordRefusesWithoutDisturbingTheBuffer covers the encoder's own
// refusals and, in the same breath, the claim that a refused record leaves no
// partial frame behind — which a writer relies on and which nothing else here
// would notice, because a partial frame in the middle of a file is
// indistinguishable from a torn tail at the end of one.
func TestAppendRecordRefusesWithoutDisturbingTheBuffer(t *testing.T) {
	valid := Record{
		Kind:     RecordKindData,
		RecvTime: time.Unix(0, 0x0102030405060708).UTC(),
	}
	cases := []struct {
		name string
		in   Record
		want error
	}{
		{"zero value", Record{}, ErrKindUnspecified},
		{"undeclared kind", withKind(valid, maxRecordKind+1), ErrKindUnknown},
		{"unset RecvTime", withRecvTime(valid, time.Time{}), ErrRecvTimeUnset},
		{"RecvTime before the Unix nanosecond floor",
			withRecvTime(valid, time.Unix(0, math.MinInt64).Add(-time.Nanosecond)), ErrRecvTimeRange},
		{"RecvTime after the Unix nanosecond ceiling",
			withRecvTime(valid, time.Unix(0, math.MaxInt64).Add(time.Nanosecond)), ErrRecvTimeRange},
		{"negative Mono", withMono(valid, -time.Nanosecond), ErrMonoNegative},
		{"payload one byte over the limit",
			withPayload(valid, make([]byte, MaxPayload+1)), ErrFrameLen},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := []byte("existing")
			got, err := AppendRecord(buf, tc.in)
			if !errors.Is(err, tc.want) {
				t.Errorf("AppendRecord = %v, want %v", err, tc.want)
			}
			if !bytes.Equal(got, buf) {
				t.Errorf("a refused record extended the buffer to %q", got)
			}
		})
	}

	// Positive controls at each boundary the table brackets, so `<` and `<=`
	// stay distinguishable and a refuse-everything encoder cannot pass.
	accepted := []struct {
		name string
		in   Record
	}{
		{"the first declared kind", withKind(valid, RecordKindOpen)},
		{"the last declared kind", withKind(valid, maxRecordKind)},
		{"RecvTime at the Unix nanosecond floor", withRecvTime(valid, time.Unix(0, math.MinInt64))},
		{"RecvTime at the Unix nanosecond ceiling", withRecvTime(valid, time.Unix(0, math.MaxInt64))},
		{"Mono of exactly zero", withMono(valid, 0)},
		{"a payload of exactly the limit", withPayload(valid, make([]byte, MaxPayload))},
	}
	for _, tc := range accepted {
		if _, err := AppendRecord(nil, tc.in); err != nil {
			t.Errorf("%s: AppendRecord = %v, want it accepted", tc.name, err)
		}
	}
}

func withKind(r Record, k RecordKind) Record    { r.Kind = k; return r }
func withRecvTime(r Record, t time.Time) Record { r.RecvTime = t; return r }
func withMono(r Record, d time.Duration) Record { r.Mono = d; return r }
func withPayload(r Record, p []byte) Record     { r.Payload = p; return r }
