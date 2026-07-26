package ids_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The UUIDv7 layout here is hand-written rather than taken from
// github.com/google/uuid (§11: platform/ is reimplemented fresh, never a
// dependency). That is a genuine cost — bit layouts are exactly what a library
// gets right once — so the first test below is checked against RFC 9562's own
// worked example rather than against this implementation's output.
//
// That distinction is the whole point. A round-trip test proves the encoder and
// the decoder agree with each other, which they would even if both were wrong in
// the same direction. The RFC's vector is an input whose right answer was known
// before this code existed.

// rfcVector is the UUIDv7 example from RFC 9562 Appendix A.6, with the field
// decomposition the RFC prints beside it:
//
//	unix_ts_ms  48  0x17F22E279B0   (= 1645557742000, 2022-02-22T19:22:22Z)
//	ver          4  0x7
//	rand_a      12  0xCC3
//	var          2  0b10
//	rand_b      62  0b01, 0x8C4DC0C0C07398F
const rfcVector = "017f22e2-79b0-7cc3-98c4-dc0c0c07398f"

func TestParseUUID_RFC9562Vector(t *testing.T) {
	u, err := ids.ParseUUID(rfcVector)
	if err != nil {
		t.Fatalf("ParseUUID(%q) = %v", rfcVector, err)
	}

	if got := u.Version(); got != 7 {
		t.Errorf("Version() = %d, want 7", got)
	}
	if got := u.Variant(); got != 0b10 {
		t.Errorf("Variant() = %#b, want 0b10", got)
	}

	// The RFC prints the decimal instant beside the vector, so this asserts our
	// 48-bit extraction against a number we did not compute.
	wantMS := int64(1645557742000)
	if got := u.Timestamp().UnixMilli(); got != wantMS {
		t.Errorf("Timestamp().UnixMilli() = %d, want %d", got, wantMS)
	}
	// And in human terms, which is what catches an extraction that is right
	// modulo a factor of 1000.
	if got := u.Timestamp().UTC().Format(time.RFC3339); got != "2022-02-22T19:22:22Z" {
		t.Errorf("Timestamp() = %s, want 2022-02-22T19:22:22Z", got)
	}

	if err := u.Validate(); err != nil {
		t.Errorf("Validate() = %v on the RFC's own example", err)
	}
	if got := u.String(); got != rfcVector {
		t.Errorf("String() = %q, want %q", got, rfcVector)
	}
}

// TestUUIDTimestampIsUTC pins the zone. UnixMilli returns a local-zone Time, and
// a Timestamp() that carried the host's zone would compare correctly against
// another instant and print misleadingly in every log line and test failure.
func TestUUIDTimestampIsUTC(t *testing.T) {
	u, err := ids.ParseUUID(rfcVector)
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := u.Timestamp().Zone(); name != "UTC" {
		t.Errorf("Timestamp() is in zone %q, want UTC", name)
	}
}

func TestUUIDValidate(t *testing.T) {
	// Each case is a real way sixteen bytes arrive wrong, and each reports its
	// own sentinel: an unset field, bytes from a different generator, and bytes
	// that are not a UUID at all are three different problems.
	cases := []struct {
		name string
		text string
		want error
	}{
		{
			// The Nil UUID is a legitimate RFC 9562 value and never one we
			// mint, which is what qualifies it as the unusable zero value.
			name: "nil uuid", text: "00000000-0000-0000-0000-000000000000", want: ids.ErrUUIDUnset,
		},
		{
			// A v4 is well-formed and useless here: every derivation in this
			// package reads the embedded timestamp, and a v4 has none.
			name: "version 4", text: "017f22e2-79b0-4cc3-98c4-dc0c0c07398f", want: ids.ErrUUIDVersion,
		},
		{
			name: "version 1", text: "017f22e2-79b0-1cc3-98c4-dc0c0c07398f", want: ids.ErrUUIDVersion,
		},
		{
			// Variant 0b110 — the Microsoft legacy layout. Byte 8 is 0xd8.
			name: "wrong variant", text: "017f22e2-79b0-7cc3-d8c4-dc0c0c07398f", want: ids.ErrUUIDVariant,
		},
		{
			// Variant 0b0 — the Apollo NCS legacy layout. Byte 8 is 0x18.
			name: "ncs variant", text: "017f22e2-79b0-7cc3-18c4-dc0c0c07398f", want: ids.ErrUUIDVariant,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := ids.ParseUUID(tc.text)
			if err != nil {
				t.Fatalf("ParseUUID(%q) = %v; the fixture itself is malformed", tc.text, err)
			}
			if err := u.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}

	// Positive control. Without it a Validate that rejected everything would
	// score a perfect result above.
	good, err := ids.ParseUUID(rfcVector)
	if err != nil {
		t.Fatal(err)
	}
	if err := good.Validate(); err != nil {
		t.Errorf("Validate() = %v on a well-formed v7", err)
	}
}

func TestParseUUIDRejects(t *testing.T) {
	// The parser accepts the canonical lowercase hyphenated form and nothing
	// else. Every UUID this program parses is one it wrote, so leniency buys
	// nothing and costs the property that a parsed id renders back identical.
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"too short", "017f22e2-79b0-7cc3-98c4-dc0c0c07398"},
		{"too long", "017f22e2-79b0-7cc3-98c4-dc0c0c07398ff"},
		{"unhyphenated", "017f22e279b07cc398c4dc0c0c07398f"},
		{"uppercase", "017F22E2-79B0-7CC3-98C4-DC0C0C07398F"},
		{"braced", "{017f22e2-79b0-7cc3-98c4-dc0c0c07398f}"},
		{"urn form", "urn:uuid:017f22e2-79b0-7cc3-98c4-dc0c0c0739"},
		{"hyphen in the wrong place", "017f22e27-9b0-7cc3-98c4-dc0c0c07398f"},
		{"underscore for hyphen", "017f22e2_79b0-7cc3-98c4-dc0c0c07398f"},
		{"non-hex digit", "017f22e2-79b0-7cc3-98c4-dc0c0c0739zz"},
		{"all spaces", strings.Repeat(" ", 36)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ids.ParseUUID(tc.text); !errors.Is(err, ids.ErrUUIDSyntax) {
				t.Errorf("ParseUUID(%q) = %v, want ErrUUIDSyntax", tc.text, err)
			}
		})
	}

	// Positive control for the same predicate.
	if _, err := ids.ParseUUID(rfcVector); err != nil {
		t.Errorf("ParseUUID(%q) = %v on the canonical form", rfcVector, err)
	}
}

func TestVenueValidate(t *testing.T) {
	if err := ids.Venue("binance").Validate(); err != nil {
		t.Errorf("Validate() = %v on a set venue", err)
	}
	if err := ids.Venue("").Validate(); !errors.Is(err, ids.ErrVenueUnset) {
		t.Errorf("Validate() = %v on an empty venue, want ErrVenueUnset", err)
	}
}

func TestExecutionIDValidate(t *testing.T) {
	// ExecutionID is received, never minted, so validation is emptiness and
	// nothing more. A shape check here would reject a legitimate fill the day a
	// venue changes its format — and dropping a fill is a position the ledger
	// does not know about.
	for _, raw := range []ids.ExecutionID{"12345", "abc-def", "0", ids.ExecutionID(strings.Repeat("x", 200))} {
		if err := raw.Validate(); err != nil {
			t.Errorf("Validate() = %v on %q; the venue's format is the venue's business", err, raw)
		}
	}
	if err := ids.ExecutionID("").Validate(); !errors.Is(err, ids.ErrExecutionIDUnset) {
		t.Errorf("Validate() = %v on an empty execution id", err)
	}
}
