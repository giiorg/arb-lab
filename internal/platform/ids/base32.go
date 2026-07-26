package ids

import "fmt"

// Crockford base32, used to pack a 128-bit UUID into the 26 characters a
// ClientOrderID's length budget can afford. Standard hex would need 32.
//
// The alphabet excludes I, L, O and U: the first three because they are
// confusable with 1, 1 and 0 in a log line or on a venue's order page, and U
// because Crockford excludes it to avoid accidental obscenity. That confusability
// argument is the whole reason for choosing this encoding over base64 — these
// ids are read aloud and re-typed during incidents, which is exactly when the
// journal that could have answered the question is unavailable (§11: the journal
// is host-local and lost on host death).
const base32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// base32Len is the number of characters 128 bits occupies at 5 bits each,
// rounded up: 26 characters carry 130 bits, so the top two are always zero.
const base32Len = 26

// encodeBase32 renders 16 bytes as 26 Crockford base32 characters,
// most-significant first.
//
// Most-significant-first matters: the leading 48 bits of a UUIDv7 are a
// millisecond timestamp, so the encoded text sorts lexicographically in mint
// order. A ClientOrderID therefore sorts by trade age with no parsing, which is
// what makes a venue's order list readable.
//
// The bit extraction is written as an explicit loop over bit positions rather
// than as the usual hand-unrolled shift table. It is O(130) on a path that runs
// a handful of times per trade, and the unrolled form is where this kind of
// encoder traditionally goes wrong — an off-by-one in one of 26 hand-written
// expressions is invisible to inspection and produces ids that round-trip
// perfectly while sorting subtly wrong.
func encodeBase32(b [16]byte) string {
	var out [base32Len]byte
	for i := range base32Len {
		// Character i covers bits [lo, lo+5) of a 130-bit space whose low 128
		// bits are the value. i == 0 is the most significant.
		lo := 5 * (base32Len - 1 - i)
		out[i] = base32Alphabet[bitsAt(b, lo)]
	}
	return string(out[:])
}

// bitsAt returns the 5 bits at [lo, lo+5) of b read as a 128-bit big-endian
// integer, where bit 0 is the least significant bit of b[15]. Positions at or
// above 128 read as zero, which is what makes the two spare bits of the top
// character always zero rather than a special case.
func bitsAt(b [16]byte, lo int) byte {
	var v byte
	for k := range 5 {
		idx := lo + k
		if idx >= 128 {
			continue
		}
		bit := (b[15-idx/8] >> (idx % 8)) & 1
		v |= bit << k
	}
	return v
}

// decodeBase32 is encodeBase32's inverse.
//
// It is STRICT where Crockford's specification is lenient, and the difference is
// deliberate. Crockford tells decoders to accept lowercase and to fold the
// excluded letters onto their lookalikes — I and L to 1, O to 0. A ClientOrderID
// is a correlation handle we send to a venue and match against what the venue
// echoes back (§5.3), so the property that matters is that exactly one string
// denotes each id. Folding aliases would make ParseClientOrderID(s).String() != s
// for a family of inputs, and "the id we sent" and "the id we parsed" would stop
// being the same object while every round-trip test still passed on canonical
// input.
func decodeBase32(s string) ([16]byte, error) {
	var b [16]byte
	if len(s) != base32Len {
		return b, fmt.Errorf("%d characters, want %d: %w", len(s), base32Len, ErrBase32Syntax)
	}
	for i := range base32Len {
		v, ok := base32Value(s[i])
		if !ok {
			return [16]byte{}, fmt.Errorf("character %q at %d: %w", s[i], i, ErrBase32Syntax)
		}
		lo := 5 * (base32Len - 1 - i)
		for k := range 5 {
			if v>>k&1 == 0 {
				continue
			}
			idx := lo + k
			if idx >= 128 {
				// A set bit in the two spare positions of the leading
				// character. The text is syntactically fine and denotes a
				// number this package never encoded, so it is not ours.
				return [16]byte{}, fmt.Errorf("leading character %q: %w", s[0], ErrBase32Overflow)
			}
			b[15-idx/8] |= 1 << (idx % 8)
		}
	}
	return b, nil
}

// base32Value maps a character to its 5-bit value, rejecting everything outside
// the canonical uppercase alphabet — including lowercase and the four excluded
// letters. A linear scan over 32 entries; a table would be faster and this is
// not a hot path.
func base32Value(c byte) (byte, bool) {
	for i := range len(base32Alphabet) {
		if base32Alphabet[i] == c {
			return byte(i), true
		}
	}
	return 0, false
}
