package ids_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/ids"

	"pgregory.net/rapid"
)

// A ClientOrderID is a recovery correlation handle (§5.3): after a crash, the
// OMS re-derives it and asks the venue what happened to that order. Two
// properties carry that job, and they fail differently.
//
//   - Derivation is PURE. Same triple, same id, forever. If it ever consults a
//     clock or a counter, crash recovery derives an id nobody submitted and the
//     venue answers "no such order" — which §5.3 then forbids us from reading as
//     "not submitted", so the order is stuck unresolvable.
//   - Parsing is STRICT. An id that is not ours must not parse, because the ids
//     that are not ours include Binance's liquidation and ADL closes, and one of
//     those parsing as a trade attributes a forced liquidation to a trade we
//     placed.
//
// The tests below are organised around those two.

// aTradeID is a fixed, valid v7 TradeID for the cases that do not care which.
func aTradeID(t *testing.T) ids.TradeID {
	t.Helper()
	id, err := ids.ParseTradeID(rfcVector)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return id
}

func TestClientOrderIDFormat(t *testing.T) {
	got, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: aTradeID(t), Leg: 1, Attempt: 0})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}

	// Written out in full rather than assembled from the constants, so that a
	// change to the format is a visible edit here. This value is what goes on
	// the wire to a venue.
	//
	// The base32 field is NOT this implementation's output copied back in. It
	// was computed independently with math/big — see TestBase32AgreesWithBigInt,
	// which does the same cross-check over generated input. The first draft of
	// this line was a hand-encoded guess and it was wrong, which is the reason
	// the oracle exists rather than a story about it.
	want := ids.ClientOrderID("ab-01FWHE4YDGFK1SHH6W1G60EECF-01-00")
	if got != want {
		t.Errorf("NewClientOrderID() = %q, want %q", got, want)
	}
	if len(got) != 35 {
		t.Errorf("length %d, want the fixed 35", len(got))
	}
}

// TestDerivationIsPure is the property crash recovery rests on.
func TestDerivationIsPure(t *testing.T) {
	ref := ids.ClientOrderRef{Trade: aTradeID(t), Leg: 0, Attempt: 3}
	first, err := ids.NewClientOrderID(ref)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1000 {
		again, err := ids.NewClientOrderID(ref)
		if err != nil {
			t.Fatalf("derivation %d: %v", i, err)
		}
		if again != first {
			t.Fatalf("derivation %d produced %q, first produced %q — derivation is not pure", i, again, first)
		}
	}
}

func TestClientOrderIDRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// A TradeID drawn from arbitrary bytes rather than from the generator,
		// so the encoding is exercised across the whole 128-bit space instead
		// of only over timestamps near today. The version and variant nibbles
		// are forced, because those are what Validate requires.
		var raw [16]byte
		for i := range raw {
			raw[i] = byte(rapid.IntRange(0, 255).Draw(rt, "byte"))
		}
		raw[6] = raw[6]&0x0f | 0x70
		raw[8] = raw[8]&0x3f | 0x80
		if raw == [16]byte{} {
			rt.Skip("nil uuid")
		}

		text := ids.UUID(raw).String()
		trade, err := ids.ParseTradeID(text)
		if err != nil {
			rt.Fatalf("ParseTradeID(%q): %v", text, err)
		}

		ref := ids.ClientOrderRef{
			Trade:   trade,
			Leg:     rapid.IntRange(0, 99).Draw(rt, "leg"),
			Attempt: rapid.IntRange(0, 99).Draw(rt, "attempt"),
		}

		id, err := ids.NewClientOrderID(ref)
		if err != nil {
			rt.Fatalf("NewClientOrderID(%+v): %v", ref, err)
		}

		// The venue-side budget, asserted on every generated id rather than
		// argued once in a comment.
		if len(id) > 36 {
			rt.Fatalf("id %q is %d characters, past the 36-character budget", id, len(id))
		}
		for i := range len(id) {
			c := id[i]
			ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
			if !ok {
				rt.Fatalf("id %q contains %q, outside [A-Za-z0-9_-]", id, c)
			}
		}

		back, err := ids.ParseClientOrderID(string(id))
		if err != nil {
			rt.Fatalf("ParseClientOrderID(%q): %v", id, err)
		}
		if back != ref {
			rt.Fatalf("round trip: %+v -> %q -> %+v", ref, id, back)
		}
	})
}

// TestOrderingSortsByTradeAge pins the reason the base32 encoding is
// most-significant-first. A venue's order page sorted as text should read in
// trade order with nothing parsing it.
func TestOrderingSortsByTradeAge(t *testing.T) {
	c := newStubClock(1645557742000)
	g := mustGenerator(t, c, constEntropy{0x00})

	var prev ids.ClientOrderID
	for i := range 50 {
		c.set(c.at.UnixMilli() + 1)
		trade, err := g.NewTradeID()
		if err != nil {
			t.Fatal(err)
		}
		id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: 0, Attempt: 0})
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && id <= prev {
			t.Fatalf("id %d (%q) does not sort after the previous trade's (%q)", i, id, prev)
		}
		prev = id
	}
}

func TestClientOrderIDRangeChecks(t *testing.T) {
	trade := aTradeID(t)
	cases := []struct {
		name string
		ref  ids.ClientOrderRef
		want error
	}{
		{"leg 100", ids.ClientOrderRef{Trade: trade, Leg: 100}, ids.ErrLegRange},
		{"leg negative", ids.ClientOrderRef{Trade: trade, Leg: -1}, ids.ErrLegRange},
		{"attempt 100", ids.ClientOrderRef{Trade: trade, Attempt: 100}, ids.ErrAttemptRange},
		{"attempt negative", ids.ClientOrderRef{Trade: trade, Attempt: -1}, ids.ErrAttemptRange},
		{
			// The zero TradeID is the case that actually happens: a struct
			// field nobody populated. Without this check it would derive a
			// perfectly well-formed id for the Nil UUID, and every unpopulated
			// trade would submit orders under the same handle.
			name: "unset trade", ref: ids.ClientOrderRef{Leg: 0, Attempt: 0}, want: ids.ErrUUIDUnset,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ids.NewClientOrderID(tc.ref); !errors.Is(err, tc.want) {
				t.Errorf("NewClientOrderID(%+v) = %v, want %v", tc.ref, err, tc.want)
			}
		})
	}

	// Positive controls at both boundaries, or an off-by-one in either
	// direction would pass the table above.
	for _, ref := range []ids.ClientOrderRef{
		{Trade: trade, Leg: 0, Attempt: 0},
		{Trade: trade, Leg: 99, Attempt: 99},
	} {
		if _, err := ids.NewClientOrderID(ref); err != nil {
			t.Errorf("NewClientOrderID(%+v) = %v at a valid boundary", ref, err)
		}
	}
}

// TestParseRejectsVenueSystemIDs is the negative control that matters most.
//
// Binance stamps its own closes with clientOrderId prefixes — "autoclose-" for
// liquidation, "adl_autoclose" for ADL, "settlement_autoclose-" for delisting
// (venue-profiles §R1d-7). These arrive on the same field as ours. Each fixture
// below is padded to exactly 35 characters so it clears the length check and the
// prefix check has to do the work — a shorter fixture would pass this test with
// the prefix check deleted.
func TestParseRejectsVenueSystemIDs(t *testing.T) {
	pad := func(s string) string {
		if len(s) >= 35 {
			return s[:35]
		}
		return s + strings.Repeat("0", 35-len(s))
	}

	cases := []string{
		pad("autoclose-"),
		pad("adl_autoclose"),
		pad("settlement_autoclose-"),
		pad("x-"),
		pad("AB-"), // our prefix, wrong case
		pad("ab_"), // our prefix, wrong separator
		pad("ba-"),
	}

	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if len(raw) != 35 {
				t.Fatalf("fixture is %d characters; it would be rejected on length, not on prefix", len(raw))
			}
			if _, err := ids.ParseClientOrderID(raw); !errors.Is(err, ids.ErrClientOrderIDSyntax) {
				t.Errorf("ParseClientOrderID(%q) = %v, want ErrClientOrderIDSyntax", raw, err)
			}
		})
	}
}

// TestParseRequiresOurPrefixSpecifically isolates the prefix check.
//
// It exists because deleting that check SURVIVED the test above. Those fixtures
// are zero-padded, so with no prefix check they still failed — on the separator
// check, returning the same sentinel. The test proved the ids were rejected and
// proved nothing about which check rejected them.
//
// What isolates it is a structurally PERFECT id with only the prefix changed:
// right length, separators in the right places, canonical base32 over a real v7,
// two digits in each numeric field. Every check but one passes, so only the
// prefix check can reject it.
func TestParseRequiresOurPrefixSpecifically(t *testing.T) {
	valid, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: aTradeID(t), Leg: 1, Attempt: 2})
	if err != nil {
		t.Fatal(err)
	}
	s := string(valid)

	// Replace only the three prefix characters; everything after index 3 is
	// byte-identical to an id this package just produced.
	for _, prefix := range []string{"xy-", "AB-", "ab_", "ba-", "a--", "00-"} {
		t.Run(prefix, func(t *testing.T) {
			forged := prefix + s[3:]
			if len(forged) != len(s) {
				t.Fatalf("fixture length changed: %d vs %d", len(forged), len(s))
			}
			if _, err := ids.ParseClientOrderID(forged); !errors.Is(err, ids.ErrClientOrderIDSyntax) {
				t.Errorf("ParseClientOrderID(%q) = %v, want ErrClientOrderIDSyntax — only the prefix differs from a valid id",
					forged, err)
			}
		})
	}

	// Positive control: the same string with our prefix restored must parse, or
	// the cases above would pass with a parser that rejects everything.
	if _, err := ids.ParseClientOrderID("ab-" + s[3:]); err != nil {
		t.Errorf("ParseClientOrderID with the prefix restored = %v", err)
	}
}

func TestParseClientOrderIDRejects(t *testing.T) {
	valid, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: aTradeID(t), Leg: 1, Attempt: 2})
	if err != nil {
		t.Fatal(err)
	}
	s := string(valid)

	// Each mutation of a known-good id targets one check, so a deleted check
	// shows up as exactly one failing case rather than as a general collapse.
	cases := []struct {
		name string
		text string
		want error
	}{
		{"empty", "", ids.ErrClientOrderIDSyntax},
		{"truncated", s[:34], ids.ErrClientOrderIDSyntax},
		{"extended", s + "0", ids.ErrClientOrderIDSyntax},
		{"first separator moved", s[:29] + "0" + s[30:], ids.ErrClientOrderIDSyntax},
		{"second separator moved", s[:32] + "0" + s[33:], ids.ErrClientOrderIDSyntax},
		{"non-digit leg", s[:30] + "x" + s[31:], ids.ErrClientOrderIDSyntax},
		{"non-digit attempt", s[:33] + "x" + s[34:], ids.ErrClientOrderIDSyntax},
		{
			// strconv.Atoi would accept "+5" and " 5" here, and the id would
			// round-trip to different text than it arrived as.
			name: "signed leg", text: s[:30] + "+5" + s[32:], want: ids.ErrClientOrderIDSyntax,
		},
		{
			// Lowercase is outside the canonical alphabet. Crockford's own
			// specification says to accept it; this decoder does not, because a
			// correlation handle must have exactly one spelling.
			name: "lowercase base32", text: s[:3] + strings.ToLower(s[3:29]) + s[29:], want: ids.ErrBase32Syntax,
		},
		{
			// I, L, O and U are excluded from the alphabet precisely because
			// they are confusable, so accepting them would defeat the reason
			// for choosing this encoding.
			name: "excluded letter I", text: s[:3] + "I" + s[4:], want: ids.ErrBase32Syntax,
		},
		{"excluded letter L", s[:3] + "L" + s[4:], ids.ErrBase32Syntax},
		{"excluded letter O", s[:3] + "O" + s[4:], ids.ErrBase32Syntax},
		{"excluded letter U", s[:3] + "U" + s[4:], ids.ErrBase32Syntax},
		{
			// 26 base32 characters carry 130 bits and a UUID is 128, so the
			// leading character can encode at most 7. Anything above denotes a
			// number this package never produced.
			name: "leading character overflows 128 bits", text: s[:3] + "8" + s[4:], want: ids.ErrBase32Overflow,
		},
		{"leading character Z", s[:3] + "Z" + s[4:], ids.ErrBase32Overflow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ids.ParseClientOrderID(tc.text); !errors.Is(err, tc.want) {
				t.Errorf("ParseClientOrderID(%q) = %v, want %v", tc.text, err, tc.want)
			}
		})
	}

	// Positive control for the whole table: the unmutated id must parse.
	if _, err := ids.ParseClientOrderID(s); err != nil {
		t.Errorf("ParseClientOrderID(%q) = %v on a valid id", s, err)
	}
}

// TestParseRejectsANonV7Payload closes the gap between "these 26 characters
// decode" and "these 26 characters are one of our trade ids". Without the
// version check a base32 string encoding a v4 would parse into a TradeID that
// TradeID.Validate would then reject — a valid-looking value that fails later,
// somewhere else.
func TestParseRejectsANonV7Payload(t *testing.T) {
	v4, err := ids.ParseUUID("017f22e2-79b0-4cc3-98c4-dc0c0c07398f")
	if err != nil {
		t.Fatal(err)
	}
	forged := "ab-" + v4.Base32() + "-00-00"
	if len(forged) != 35 {
		t.Fatalf("fixture is %d characters", len(forged))
	}
	if _, err := ids.ParseClientOrderID(forged); !errors.Is(err, ids.ErrUUIDVersion) {
		t.Errorf("ParseClientOrderID on a v4 payload = %v, want ErrUUIDVersion", err)
	}

	// And the Nil UUID, which encodes to 26 zeros and is otherwise perfectly
	// well-formed text.
	nilForged := "ab-" + strings.Repeat("0", 26) + "-00-00"
	if _, err := ids.ParseClientOrderID(nilForged); !errors.Is(err, ids.ErrUUIDUnset) {
		t.Errorf("ParseClientOrderID on the nil uuid = %v, want ErrUUIDUnset", err)
	}
}
