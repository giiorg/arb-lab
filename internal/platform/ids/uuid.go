package ids

import (
	"fmt"
	"time"
)

// UUID is a 128-bit identifier laid out per RFC 9562. This package mints only
// version 7, so everything below assumes that layout:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          unix_ts_ms                           |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|          unix_ts_ms           |  ver  |        rand_a         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|var|                        rand_b                             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                            rand_b                             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// v7 rather than v4 because the timestamp prefix makes ids sort in mint order.
// That is not cosmetic here: it makes the SQLite primary-key index append rather
// than scatter, and it makes a base32-encoded ClientOrderID sort by trade age on
// a venue's order page with nothing parsing it.
//
// Written fresh rather than pulled from github.com/google/uuid, per §11's
// "reimplemented fresh; never a dependency" for platform/. That is a real cost —
// the layout below is the kind of thing a library gets right once — and it is
// paid back by the tests: this implementation is checked against RFC 9562's own
// worked example, which a dependency would not have been.
type UUID [16]byte

// nilUUID is RFC 9562's Nil UUID: sixteen zero bytes. It is a legitimate value
// in the standard and never one this package mints, which is what lets it serve
// as the unusable zero value every construction in this project has.
var nilUUID UUID

// Version reports the 4-bit version field. 7 for everything this package mints.
func (u UUID) Version() int { return int(u[6] >> 4) }

// Variant reports the 2-bit variant field, which RFC 9562 fixes at 0b10 for the
// versioned layouts. Anything else means the bytes came from somewhere with a
// different idea of the format.
func (u UUID) Variant() int { return int(u[8] >> 6) }

// Timestamp reports the embedded 48-bit millisecond instant.
//
// Only meaningful for a v7 UUID; the caller is expected to have validated. It is
// a UTC instant because the encoded value is a Unix millisecond count with no
// zone information, and returning it in the local zone would invite a
// comparison against something that is not.
func (u UUID) Timestamp() time.Time {
	ms := int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
		int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
	return time.UnixMilli(ms).UTC()
}

// Validate reports whether u is a well-formed UUIDv7.
//
// It checks three things and reports them separately, because they fail for
// different reasons: an unset field (the Nil UUID), bytes from a different
// generator (wrong version), and bytes that are not a UUID at all (wrong
// variant). A single "invalid uuid" would send all three to the same place.
func (u UUID) Validate() error {
	if u == nilUUID {
		return ErrUUIDUnset
	}
	if v := u.Version(); v != 7 {
		return fmt.Errorf("version %d: %w", v, ErrUUIDVersion)
	}
	if v := u.Variant(); v != 0b10 {
		return fmt.Errorf("variant %#b: %w", v, ErrUUIDVariant)
	}
	return nil
}

// String renders the canonical 8-4-4-4-12 lowercase hex form.
func (u UUID) String() string {
	const hexDigits = "0123456789abcdef"
	// 32 hex digits plus 4 hyphens.
	var out [36]byte
	j := 0
	for i, b := range u {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = hexDigits[b>>4]
		out[j+1] = hexDigits[b&0x0f]
		j += 2
	}
	return string(out[:])
}

// Base32 renders u as 26 Crockford base32 characters — the form a ClientOrderID
// embeds, where the canonical 36-character text would not fit.
func (u UUID) Base32() string { return encodeBase32(u) }

// ParseUUID reads the canonical 8-4-4-4-12 hex form.
//
// It accepts that form and only that form: no braces, no urn: prefix, no
// unhyphenated 32-character variant, and no uppercase. Every UUID this program
// parses is one it wrote, so leniency would buy nothing and cost the property
// that a parsed id renders back byte-identical.
func ParseUUID(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 {
		return u, fmt.Errorf("%d characters, want 36: %w", len(s), ErrUUIDSyntax)
	}
	for _, i := range [4]int{8, 13, 18, 23} {
		if s[i] != '-' {
			return u, fmt.Errorf("expected '-' at %d, got %q: %w", i, s[i], ErrUUIDSyntax)
		}
	}
	j := 0
	for i := 0; i < 36; i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		hi, ok := hexValue(s[i])
		if !ok {
			return UUID{}, fmt.Errorf("character %q at %d: %w", s[i], i, ErrUUIDSyntax)
		}
		lo, ok := hexValue(s[i+1])
		if !ok {
			return UUID{}, fmt.Errorf("character %q at %d: %w", s[i+1], i+1, ErrUUIDSyntax)
		}
		u[j] = hi<<4 | lo
		j++
		i++ // consumed two digits
	}
	return u, nil
}

// hexValue accepts lowercase hex only, matching what String emits. See ParseUUID
// on why the parser is not lenient.
func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}
