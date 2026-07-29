package capture

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"
)

// DecodeRecord is this package's only untrusted-input boundary. Everything else
// takes a Record the caller built; this takes bytes that came off a disk that
// may have been written by a process that crashed, copied over a network, or
// sat on a laptop SSD for three months — and §32k's whole point is that a
// capture archive is discovered to be wrong long after the window it covers is
// unrecoverable.
//
// A fuzz target that only checks for panics is a crash test, and crashing is not
// this decoder's likely failure mode. Returning a PLAUSIBLE record is. So the
// body asserts four invariants that must hold for every input, well-formed or
// not:
//
//	D1  every error is exactly one of the sentinel classes, and a rejected
//	    input yields no record and no length
//	D2  an accepted record consumed at least a header and at most the buffer
//	D3  CANONICITY — re-encoding an accepted record reproduces the accepted
//	    bytes exactly. §32l's "never reports a record the writer did not write",
//	    stated so that it is checkable
//	D4  the independently written oracle decoder agrees — on the value when the
//	    input is accepted, and on WHICH SENTINEL when it is not
//
// D4's failure half was added because a negative control walked through the
// first draft untouched. The invariants originally consulted the oracle only on
// the success path, so a decoder that answered ErrTruncated to every refusal
// satisfied all four: D1 asks that an error be exactly one sentinel, and
// ErrTruncated is one. The set was blind to precisely the misclassification that
// costs the most, since acting on ErrTruncated means TRUNCATING A FILE. Agreeing
// that an input is bad is worth little; agreeing why is the whole contract.
//
// D3 is the one worth dwelling on. It says every byte string the reader accepts
// is in the image of the writer, which is what makes "the archive is the fact"
// mean something: there is no frame the reader will interpret that this package
// could not have produced. It holds because Record.Validate is called on both
// sides — see its doc — and because the two fields with no wire-level
// constraint, RecvTime and CaptureSeq, are total: every int64 of Unix
// nanoseconds is a valid instant, so ErrRecvTimeUnset and ErrRecvTimeRange are
// reachable from the ENCODER only. That asymmetry is deliberate and is why they
// appear in the class list below without a decode fixture.
//
// The body is a predicate over a decodeFunc rather than inline in f.Fuzz, in the
// same shape and for the same reason as fixedpoint's parser fuzz: it lets the
// negative controls run the identical invariants against deliberately broken
// decoders and show that each one bites. `go test -fuzz` alone cannot
// demonstrate that — a target whose assertions are vacuous reports the same
// clean run as one that works.
//
// `go test` runs a fuzz target's seed corpus as an ordinary table even without
// -fuzz, so the seeds below are a committed regression suite the CI gate
// executes; -fuzz is the explorer on top.

type decodeFunc func([]byte) (Record, int, error)

// decodeErrorClasses are the ways bytes can fail to become a record. They are
// mutually exclusive by construction, and the split that carries weight is
// ErrTruncated against the rest: §32e's recovery may truncate a file on the
// first and must not on any other, and §32k gives verify two exit codes for the
// difference. An error that is none of them, or two of them, is unusable to that
// caller.
var decodeErrorClasses = []error{
	ErrTruncated, ErrFrameLen, ErrChecksum, ErrReservedNotZero,
	ErrKindUnspecified, ErrKindUnknown,
	ErrRecvTimeUnset, ErrRecvTimeRange, ErrMonoNegative,
}

// decodeErrorClass reports which sentinel err belongs to, and how many it
// matched — a count of anything but 1 is itself a D1 violation.
func decodeErrorClass(err error) (error, int) {
	var found error
	matched := 0
	for _, class := range decodeErrorClasses {
		if errors.Is(err, class) {
			found, matched = class, matched+1
		}
	}
	return found, matched
}

// checkDecodeInvariants reports how dec violates D1-D4 on b, or "" if it does
// not. Every branch names the invariant, because a fuzz failure arrives with no
// context beyond the input that produced it.
func checkDecodeInvariants(dec decodeFunc, b []byte) string {
	got, n, err := dec(b)
	oracleGot, oracleN, oracleErr := oracleDecode(b)

	// D1: classified, and carrying nothing.
	if err != nil {
		class, matched := decodeErrorClass(err)
		if matched != 1 {
			return fmt.Sprintf("D1: %s was rejected with %v, matching %d of the "+
				"sentinel classes; callers branch on exactly one",
				hex.EncodeToString(b), err, matched)
		}
		if n != 0 {
			return fmt.Sprintf("D1: %s was rejected with %v but reported %d bytes "+
				"consumed; a caller advancing by that resynchronizes on garbage",
				hex.EncodeToString(b), err, n)
		}
		if !recordsEqual(got, Record{}) {
			return fmt.Sprintf("D1: %s was rejected with %v but also returned %+v",
				hex.EncodeToString(b), err, got)
		}

		// D4 on the FAILURE path, and it is not an afterthought — see the
		// invariant's note above. Agreeing that an input is bad is worth little;
		// the value is in agreeing WHY, because ErrTruncated is the one verdict
		// §32e's recovery acts on by truncating a file.
		if oracleErr == nil {
			return fmt.Sprintf("D4: %s was rejected with %v, but the oracle "+
				"decoded it as %+v", hex.EncodeToString(b), err, oracleGot)
		}
		// Compared by CLASS rather than with errors.Is against the oracle's own
		// error, because both sides wrap their sentinels with the values that
		// failed and neither wrapper is in the other's chain.
		if oracleClass, _ := decodeErrorClass(oracleErr); oracleClass != class {
			return fmt.Sprintf("D4: %s was rejected as %v, oracle says %v",
				hex.EncodeToString(b), class, oracleClass)
		}
		return ""
	}

	// D2: the length is usable for walking forward.
	if n < recordHeaderLen || n > len(b) {
		return fmt.Sprintf("D2: %s decoded with %d bytes consumed, outside [%d, %d]",
			hex.EncodeToString(b), n, recordHeaderLen, len(b))
	}

	// D3: canonicity.
	reencoded, encErr := AppendRecord(nil, got)
	switch {
	case encErr != nil:
		return fmt.Sprintf("D3: %s decoded to %+v, which the encoder refuses: %v",
			hex.EncodeToString(b), got, encErr)
	case !bytes.Equal(reencoded, b[:n]):
		return fmt.Sprintf("D3: %s decoded to %+v, which re-encodes as %s — the "+
			"reader accepted a frame the writer would not have written",
			hex.EncodeToString(b[:n]), got, hex.EncodeToString(reencoded))
	}

	// D4 on the success path. The only invariant that can catch a decoder which
	// is wrong about a VALUE while remaining perfectly self-consistent — one
	// that reads captureSeq and recvTime from swapped offsets re-encodes its own
	// misreading back to the identical bytes.
	switch {
	case oracleErr != nil:
		return fmt.Sprintf("D4: %s decoded to %+v, but the oracle rejected it: %v",
			hex.EncodeToString(b), got, oracleErr)
	case oracleN != n:
		return fmt.Sprintf("D4: %s consumed %d bytes, oracle says %d",
			hex.EncodeToString(b), n, oracleN)
	case !recordsEqual(oracleGot, got):
		return fmt.Sprintf("D4: %s decoded to %+v, oracle says %+v",
			hex.EncodeToString(b), got, oracleGot)
	}
	return ""
}

// fuzzSeeds are the shapes worth handing the fuzzer directly rather than hoping
// it stumbles onto them: the boundaries between the outcomes, and the byte
// patterns a failing disk or a dying process actually produces.
//
// testdata/fuzz/ is deliberately not gitignored: Go keeps its generated corpus
// in GOCACHE and writes into the repo only when it finds a FAILING input, so
// anything that appears there is a regression case to commit.
func fuzzSeeds() [][]byte {
	valid := goldenBytes()
	empty, _ := AppendRecord(nil, Record{
		Kind:     RecordKindAlive,
		RecvTime: time.Unix(0, 1).UTC(),
	})

	withLen := func(v uint32) []byte {
		b := goldenBytes()
		binary.BigEndian.PutUint32(b, v)
		return b
	}
	patch := func(at int, to byte) []byte {
		b := goldenBytes()
		b[at] = to
		return b
	}

	return [][]byte{
		// Not records at all, including the pattern that matters most: a run of
		// zeros, which is what a sparse hole and a pre-allocated page look like.
		nil, {}, {0}, {0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0},
		bytes.Repeat([]byte{0}, recordHeaderLen),
		bytes.Repeat([]byte{0xff}, recordHeaderLen),
		[]byte(magic), AppendFileHeader(nil),

		// Well-formed, at both payload extremes, and two in a row.
		valid, empty, append(append([]byte(nil), valid...), empty...),

		// One byte of trailing garbage after a whole frame: the case where a
		// reader that trusts len(b) instead of frameLen diverges.
		append(append([]byte(nil), valid...), 0x00),

		// Truncations at each structural boundary.
		valid[:3], valid[:4], valid[:8], valid[:recordHeaderLen-1],
		valid[:recordHeaderLen], valid[:len(valid)-1],

		// The frame-length bound, on both sides of each edge.
		withLen(0), withLen(framedHeaderLen - 1), withLen(framedHeaderLen),
		withLen(MaxRecord), withLen(MaxRecord + 1), withLen(math.MaxUint32),

		// Single-bit corruption in each header field, checksum left stale —
		// what a bad byte on a disk actually is.
		patch(offKind, goldenFrame[offKind]^1),
		patch(offFlags, 1), patch(offReserved, 1), patch(offReserved+1, 1),
		patch(offCaptureSeq, 0xff), patch(offRecvTime, 0xff), patch(offMono, 0xff),
		patch(offChecksum, 0xff), patch(recordHeaderLen, 0xff),

		// Semantic refusals with the checksum RESTORED, so the fuzzer starts
		// from inputs that reach past the integrity check.
		reframe(func(b []byte) { b[offKind] = 0 }),
		reframe(func(b []byte) { b[offKind] = 0xff }),
		reframe(func(b []byte) { b[offFlags] = 1 }),
		reframe(func(b []byte) { binary.BigEndian.PutUint64(b[offMono:], math.MaxUint64) }),
		reframe(func(b []byte) { binary.BigEndian.PutUint64(b[offRecvTime:], 0) }),
	}
}

func FuzzDecodeRecord(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if diff := checkDecodeInvariants(DecodeRecord, b); diff != "" {
			t.Fatalf("%s", diff)
		}
	})
}

// TestDecodeInvariants_NegativeControls is what makes the fuzz target above
// worth running. Each control is a plausible way to get this decoder wrong, and
// each is aimed at a specific invariant — so a control that stops being caught
// tells you which invariant went vacuous.
func TestDecodeInvariants_NegativeControls(t *testing.T) {
	controls := map[string]decodeFunc{
		// D3. The checksum verified and then ignored — what happens when
		// somebody "fixes" a file full of CRC failures by trusting the bytes.
		// The archive still parses; every integrity claim in §32e evaporates.
		"repairs the checksum instead of refusing": func(b []byte) (Record, int, error) {
			if len(b) >= recordHeaderLen {
				fixed := append([]byte(nil), b...)
				frameLen := binary.BigEndian.Uint32(fixed)
				if frameLen >= framedHeaderLen && frameLen <= MaxRecord &&
					int(frameLen)+4 <= len(fixed) {
					binary.BigEndian.PutUint32(fixed[offChecksum:],
						oracleCRC32C(fixed[offKind:int(frameLen)+4]))
					return DecodeRecord(fixed)
				}
			}
			return DecodeRecord(b)
		},

		// D3. Reserved bits read and discarded, which is the natural shape of
		// "we do not use those yet". A v1.1 writer's flag would arrive here as a
		// silently dropped modifier on a record that otherwise looks perfect.
		"ignores the reserved bits": func(b []byte) (Record, int, error) {
			if len(b) >= recordHeaderLen {
				cleared := append([]byte(nil), b...)
				cleared[offFlags], cleared[offReserved], cleared[offReserved+1] = 0, 0, 0
				frameLen := binary.BigEndian.Uint32(cleared)
				if frameLen >= framedHeaderLen && frameLen <= MaxRecord &&
					int(frameLen)+4 <= len(cleared) {
					binary.BigEndian.PutUint32(cleared[offChecksum:],
						oracleCRC32C(cleared[offKind:int(frameLen)+4]))
					return DecodeRecord(cleared)
				}
			}
			return DecodeRecord(b)
		},

		// D3. An undeclared kind admitted rather than refused — the shape a
		// reader takes when somebody decides an unknown record should be
		// "passed through". It parses; nothing downstream knows what it holds.
		"admits an undeclared record kind": func(b []byte) (Record, int, error) {
			r, n, err := DecodeRecord(b)
			if errors.Is(err, ErrKindUnknown) || errors.Is(err, ErrKindUnspecified) {
				patched := append([]byte(nil), b...)
				original := patched[offKind]
				patched[offKind] = byte(RecordKindData)
				frameLen := binary.BigEndian.Uint32(patched)
				binary.BigEndian.PutUint32(patched[offChecksum:],
					oracleCRC32C(patched[offKind:int(frameLen)+4]))
				if r2, n2, err2 := DecodeRecord(patched); err2 == nil {
					r2.Kind = RecordKind(original)
					return r2, n2, nil
				}
			}
			return r, n, err
		},

		// D2/D3. The length reported as a constant rather than as the frame's.
		// A writer would not notice; a reader walking a file would land four
		// bytes into the next record's payload and resynchronize on garbage.
		"reports a fixed advance": func(b []byte) (Record, int, error) {
			r, n, err := DecodeRecord(b)
			if err == nil {
				return r, recordHeaderLen, nil
			}
			return r, n, err
		},

		// D4. The torn-tail split collapsed: corruption reported as truncation.
		// This is the control that guards the distinction errors.go is built
		// around, because acting on it means TRUNCATING A FILE — the one
		// recovery step that destroys data on a false positive.
		"reports corruption as a torn tail": func(b []byte) (Record, int, error) {
			r, n, err := DecodeRecord(b)
			if err != nil && !errors.Is(err, ErrTruncated) {
				return Record{}, 0, fmt.Errorf("decode: %w", ErrTruncated)
			}
			return r, n, err
		},

		// D4. Two fields read from swapped offsets. It re-encodes its own
		// misreading back to the identical bytes, classifies every error
		// correctly, and consumes the right number of bytes — so only a second
		// opinion about the VALUES finds it. Note what this would do in
		// production: §32i's clock-step detection compares Mono against
		// RecvTime, so a swap makes the one cross-check that would catch a
		// squashed time axis compare a field with itself.
		"reads captureSeq and recvTime from swapped offsets": func(b []byte) (Record, int, error) {
			r, n, err := DecodeRecord(b)
			if err == nil {
				r.CaptureSeq, r.RecvTime =
					uint64(r.RecvTime.UnixNano()), time.Unix(0, int64(r.CaptureSeq)).UTC()
			}
			return r, n, err
		},
	}

	sweep := fuzzSeeds()
	for name, dec := range controls {
		t.Run(name, func(t *testing.T) {
			for _, b := range sweep {
				if diff := checkDecodeInvariants(dec, b); diff != "" {
					t.Logf("rejected at %s: %s", hex.EncodeToString(b), diff)
					return
				}
			}
			t.Errorf("the invariants accepted a known-wrong decoder (%s); they "+
				"cannot be distinguishing anything", name)
		})
	}

	// The positive control. Without it, a predicate that objects to everything
	// would read as a flawless set of negative controls.
	for _, b := range sweep {
		if diff := checkDecodeInvariants(DecodeRecord, b); diff != "" {
			t.Fatalf("the real DecodeRecord was rejected by its own invariants "+
				"at %s: %s", hex.EncodeToString(b), diff)
		}
	}
}

// TestDecodeRecordCopiesThePayload covers the one claim in Record's doc that
// none of D1-D4 can reach: the payload is a copy, not a window onto the caller's
// buffer.
//
// Aliasing would pass every invariant in this file and every test in the one
// beside it, and would fail in slice B, where a writer reads into a reused
// buffer — records already handed out would change underneath their holder, and
// the symptom is an archive report whose numbers move between runs over
// identical bytes.
func TestDecodeRecordCopiesThePayload(t *testing.T) {
	buf := goldenBytes()
	got, _, err := DecodeRecord(buf)
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	before := append([]byte(nil), got.Payload...)

	for i := range buf {
		buf[i] = 0xAA
	}
	if !bytes.Equal(got.Payload, before) {
		t.Errorf("Payload changed when the source buffer was overwritten: %q -> %q; "+
			"it aliases the caller's bytes", before, got.Payload)
	}
}
