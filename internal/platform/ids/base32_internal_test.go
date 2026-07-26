package ids

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// An oracle for the base32 codec, in the discipline fixedpoint established: the
// implementation is checked against an independently written second one that
// shares no code path with it, rather than against itself.
//
// encodeBase32 walks bit positions. The oracle below builds the value with
// math/big and divides repeatedly by 32. Those are different enough that an
// off-by-one in one of them does not appear in the other — which is what makes
// agreement evidence.
//
// This is not hypothetical rigour. The golden value in clientorderid_test.go was
// first written as a hand-encoding, and it was wrong. The oracle is what found
// it, and if the fix had been "copy whatever the implementation prints" the
// golden test would have been an assertion that the code equals itself.

// oracleEncode renders 16 bytes as 26 Crockford characters by repeated division.
func oracleEncode(b [16]byte) string {
	n := new(big.Int).SetBytes(b[:])
	out := make([]byte, base32Len)
	q, r, m := new(big.Int).Set(n), new(big.Int), big.NewInt(32)
	for i := base32Len - 1; i >= 0; i-- {
		q.DivMod(q, m, r)
		out[i] = base32Alphabet[r.Int64()]
	}
	return string(out)
}

// oracleDecode is the inverse, by repeated multiplication.
func oracleDecode(s string) [16]byte {
	n := new(big.Int)
	m := big.NewInt(32)
	for i := range len(s) {
		v := strings.IndexByte(base32Alphabet, s[i])
		n.Mul(n, m)
		n.Add(n, big.NewInt(int64(v)))
	}
	var out [16]byte
	n.FillBytes(out[:])
	return out
}

func TestBase32AgreesWithBigInt(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		var b [16]byte
		for i := range b {
			b[i] = byte(rapid.IntRange(0, 255).Draw(rt, "byte"))
		}

		got, want := encodeBase32(b), oracleEncode(b)
		if got != want {
			rt.Fatalf("encodeBase32(%x) = %q, oracle says %q", b, got, want)
		}

		back, err := decodeBase32(got)
		if err != nil {
			rt.Fatalf("decodeBase32(%q): %v", got, err)
		}
		if back != b {
			rt.Fatalf("round trip: %x -> %q -> %x", b, got, back)
		}
		if oracleBack := oracleDecode(got); oracleBack != b {
			rt.Fatalf("oracle round trip: %x -> %q -> %x", b, got, oracleBack)
		}
	})
}

// TestBase32Boundaries covers the values a generated sweep is unlikely to reach
// and where an encoder traditionally goes wrong: all zeros, all ones, and the
// single-bit values at each end.
func TestBase32Boundaries(t *testing.T) {
	cases := []struct {
		name string
		in   [16]byte
		want string
	}{
		{"all zero", [16]byte{}, strings.Repeat("0", 26)},
		{
			// 2^128-1. The leading character encodes the top three bits, and
			// with the two spare bits necessarily zero the largest it can be is
			// 7 — which is the invariant decodeBase32's overflow check enforces
			// from the other side.
			name: "all ones",
			in:   [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			want: "7ZZZZZZZZZZZZZZZZZZZZZZZZZ",
		},
		{"least significant bit", [16]byte{15: 0x01}, strings.Repeat("0", 25) + "1"},
		{"most significant bit", [16]byte{0: 0x80}, "40000000000000000000000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeBase32(tc.in); got != tc.want {
				t.Errorf("encodeBase32(%x) = %q, want %q", tc.in, got, tc.want)
			}
			// Cross-checked against the oracle too, so a wrong expectation in
			// this table cannot agree with a wrong implementation.
			if got := oracleEncode(tc.in); got != tc.want {
				t.Errorf("oracle disagrees with the table: %q, want %q", got, tc.want)
			}
			back, err := decodeBase32(tc.want)
			if err != nil {
				t.Fatalf("decodeBase32(%q): %v", tc.want, err)
			}
			if back != tc.in {
				t.Errorf("decodeBase32(%q) = %x, want %x", tc.want, back, tc.in)
			}
		})
	}
}

// TestBase32PreservesOrder is what makes a ClientOrderID sort by trade age on a
// venue's order page. Most-significant-first encoding gives it; a
// least-significant-first one would round-trip identically and sort at random,
// which is exactly the kind of defect that survives a round-trip test.
func TestBase32PreservesOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		var a, b [16]byte
		for i := range a {
			a[i] = byte(rapid.IntRange(0, 255).Draw(rt, "a"))
			b[i] = byte(rapid.IntRange(0, 255).Draw(rt, "b"))
		}

		na := new(big.Int).SetBytes(a[:])
		nb := new(big.Int).SetBytes(b[:])
		ea, eb := encodeBase32(a), encodeBase32(b)

		switch na.Cmp(nb) {
		case -1:
			if !(ea < eb) {
				rt.Fatalf("%x < %x numerically but %q does not sort before %q", a, b, ea, eb)
			}
		case 0:
			if ea != eb {
				rt.Fatalf("%x == %x but encodings differ: %q vs %q", a, b, ea, eb)
			}
		case 1:
			if !(ea > eb) {
				rt.Fatalf("%x > %x numerically but %q does not sort after %q", a, b, ea, eb)
			}
		}
	})
}

func TestDecodeBase32Rejects(t *testing.T) {
	valid := strings.Repeat("0", 26)
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrBase32Syntax},
		{"too short", strings.Repeat("0", 25), ErrBase32Syntax},
		{"too long", strings.Repeat("0", 27), ErrBase32Syntax},
		{"lowercase", strings.Repeat("a", 26), ErrBase32Syntax},
		{"excluded I", "I" + valid[1:], ErrBase32Syntax},
		{"excluded L", "L" + valid[1:], ErrBase32Syntax},
		{"excluded O", "O" + valid[1:], ErrBase32Syntax},
		{"excluded U", "U" + valid[1:], ErrBase32Syntax},
		{"hyphen", "-" + valid[1:], ErrBase32Syntax},
		{"space", " " + valid[1:], ErrBase32Syntax},
		{
			// The two spare high bits are set. 26 characters carry 130 bits;
			// anything needing more than 128 was never produced here.
			name: "leading 8 overflows", in: "8" + valid[1:], want: ErrBase32Overflow,
		},
		{"leading Z overflows", "Z" + valid[1:], ErrBase32Overflow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeBase32(tc.in); err == nil {
				t.Fatalf("decodeBase32(%q) = nil error", tc.in)
			} else if !errors.Is(err, tc.want) {
				t.Errorf("decodeBase32(%q) = %v, want %v", tc.in, err, tc.want)
			}
		})
	}

	// Positive controls at the overflow boundary: 7 is the largest leading
	// character that fits, so it must be accepted while 8 is rejected. Without
	// this the check could reject everything above 0 and the table would still
	// pass.
	for _, lead := range []string{"0", "7"} {
		if _, err := decodeBase32(lead + valid[1:]); err != nil {
			t.Errorf("decodeBase32 with leading %q = %v; it fits in 128 bits", lead, err)
		}
	}
}
