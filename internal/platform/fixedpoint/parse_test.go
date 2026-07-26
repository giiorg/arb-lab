package fixedpoint_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Decimal text is the only representation venues speak: order prices, sizes and
// funding rates all arrive and depart as strings. Parsing must be exact and must
// never route through float64 — that is what §3.6 rule 4 is protecting.
//
// Every `want` below is a hand-written known-good literal, not a value
// recomputed the way the implementation computes it.
func TestParseMoney_roundTrip(t *testing.T) {
	const usdt = fixedpoint.Asset("USDT")

	cases := []struct {
		in   string
		want string // canonical Decimal() output
	}{
		// Plain integers.
		{"0", "0"},
		{"1", "1"},
		{"-1", "-1"},
		{"+1", "1"},
		{"-0", "0"},
		{"1000000", "1000000"},

		// Fractions, canonicalized: no trailing zeros.
		{"0.5", "0.5"},
		{"-1.25", "-1.25"},
		{"1.500", "1.5"},
		{"0.000000000", "0"},

		// Forms with an implicit zero on one side of the point.
		{"1.", "1"},
		{".5", "0.5"},

		// The scale boundary: exactly 9 fractional digits is exact, and the
		// smallest representable magnitude is one unit in the last place.
		{"123456789.123456789", "123456789.123456789"},
		{"0.000000001", "0.000000001"},
		{"-0.000000001", "-0.000000001"},

		// Trailing zeros past the scale are droppable because they are zero.
		{"1.0000000000000", "1"},

		// Scientific notation — venues and config files both emit it.
		{"1e-8", "0.00000001"},
		{"1E3", "1000"},
		{"1.5e2", "150"},
		{"-2.5E-1", "-0.25"},
		{"0e100", "0"},

		// The int64 mantissa boundary at scale 9. Beyond these two values the
		// representation overflows, which the rejection table covers.
		{"9223372036.854775807", "9223372036.854775807"},
		{"-9223372036.854775808", "-9223372036.854775808"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			m, err := fixedpoint.ParseMoney(usdt, tc.in)
			if err != nil {
				t.Fatalf("ParseMoney(%q) returned error %v; want it to parse", tc.in, err)
			}
			if err := m.Validate(); err != nil {
				t.Fatalf("parsed value fails Validate(): %v", err)
			}
			if got := m.Decimal(); got != tc.want {
				t.Errorf("ParseMoney(%q).Decimal() = %q; want %q", tc.in, got, tc.want)
			}
			// Re-parsing canonical output must be a fixed point, otherwise the
			// SQL and venue codecs cannot be trusted to survive a round trip.
			again, err := fixedpoint.ParseMoney(usdt, m.Decimal())
			if err != nil {
				t.Fatalf("re-parsing canonical %q failed: %v", m.Decimal(), err)
			}
			if again != m {
				t.Errorf("round trip changed the value: %q -> %q", m.Decimal(), again.Decimal())
			}
		})
	}
}

// The canonical scale is public because venue adapters and SQL codecs need it.
func TestScale(t *testing.T) {
	if fixedpoint.Scale != 9 {
		t.Fatalf("Scale = %d; want 9 (NautilusTrader standard-mode precision, §11)", fixedpoint.Scale)
	}
}
