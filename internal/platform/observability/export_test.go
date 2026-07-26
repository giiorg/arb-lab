package observability_test

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// floatText is the only place in this suite that holds a float64.
//
// The table-driven alternative — a `want float64` column — needed a
// //nofloat:allow on every row, because rule 4 is type-based and an untyped
// constant in a float64 field IS a float64. Nine escapes across a table is how
// an annotated exception stops being read; one, in a named helper whose whole
// job is the wire seam, stays readable. Comparing the decimal rendering rather
// than the float is also the stricter assertion: 0.25 and 0.2500000001 differ
// as text and would need an epsilon as numbers.
func floatText(t *testing.T, s observability.Sample) (string, bool) {
	t.Helper()

	f, exact := observability.Float(s)                //nofloat:allow the metric wire seam under test
	return strconv.FormatFloat(f, 'g', -1, 64), exact //nofloat:allow rendering the seam's output
}

// TestFloatExactnessBoundary pins where a float64 stops being able to
// distinguish adjacent integers. Above 2^53, n and n+1 are the same double — so
// a conversion past it has not lost a little precision, it has merged a value
// with its neighbour.
func TestFloatExactnessBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        observability.Sample
		wantExact bool
	}{
		{"zero", observability.Count(0), true},
		{"one below the boundary", observability.Count(observability.MaxExactMantissa - 1), true},
		{"exactly at the boundary", observability.Count(observability.MaxExactMantissa), true},
		{"one above the boundary", observability.Count(observability.MaxExactMantissa + 1), false},
		{"negative, one below", observability.Count(-(observability.MaxExactMantissa - 1)), true},
		{"negative, at the boundary", observability.Count(-observability.MaxExactMantissa), true},
		{"negative, one above", observability.Count(-(observability.MaxExactMantissa + 1)), false},
		{"MaxInt64", observability.Count(math.MaxInt64), false},
		// MinInt64 has no int64 magnitude — negating it leaves it negative. A
		// magnitude check written as `mag <= limit` without also requiring
		// `mag >= 0` would report this one EXACT, which is the worst available
		// answer for the most extreme input.
		{"MinInt64", observability.Count(math.MinInt64), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, exact := floatText(t, tc.in); exact != tc.wantExact {
				t.Errorf("Float(%s) exact = %v; want %v", tc.in, exact, tc.wantExact)
			}
		})
	}
}

func TestFloatScalesDimensionalUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   observability.Sample
		want string
	}{
		// Counts and bytes are plain integers, deliberately unscaled: scaling a
		// tally would put its overflow ceiling at ~9.2e9 events.
		{"count is unscaled", observability.Count(3), "3"},
		{"bytes are unscaled", observability.Bytes(4096), "4096"},
		// Seconds, ratios and money divide by 10^Scale.
		{"one second", observability.Seconds(time.Second), "1"},
		{"250ms", observability.Seconds(250 * time.Millisecond), "0.25"},
		{"negative skew stays signed", observability.Seconds(-500 * time.Millisecond), "-0.5"},
		{"one nanosecond", observability.Seconds(time.Nanosecond), "1e-09"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, _ := floatText(t, tc.in); got != tc.want {
				t.Errorf("Float(%s) = %s; want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestFloatScalesRatioAndMoney(t *testing.T) {
	t.Parallel()

	r, err := fixedpoint.ParseRate("0.0001")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}
	ratio, err := observability.Ratio(r)
	if err != nil {
		t.Fatalf("Ratio: %v", err)
	}
	if got, _ := floatText(t, ratio); got != "0.0001" {
		t.Errorf("Float(0.0001 rate) = %s; want 0.0001 — rates are ratios, never percents", got)
	}

	money, err := observability.MoneyOf(mustMoney(t, "USD", "-38.42"))
	if err != nil {
		t.Fatalf("MoneyOf: %v", err)
	}
	if got, _ := floatText(t, money); got != "-38.42" {
		t.Errorf("Float(-38.42 USD) = %s; want -38.42", got)
	}
}

// TestFloatOfMoney checks the convenience path an exporter holding a Money
// would otherwise open-code as float64(m.Mantissa()) / 1e9 — where the digit
// count is the thing nobody double-checks and one digit is a factor of ten on a
// PnL panel.
func TestFloatOfMoney(t *testing.T) {
	t.Parallel()

	got, exact, err := floatOfText(t, mustMoney(t, "USD", "1234.56"))
	if err != nil {
		t.Fatalf("FloatOf: %v", err)
	}
	if !exact {
		t.Error("exact = false; 1234.56 at scale 9 is well inside the exact range")
	}
	if got != "1234.56" {
		t.Errorf("FloatOf = %s; want 1234.56", got)
	}

	// An unconstructed Money is refused rather than exported as zero.
	if _, _, err := floatOfText(t, fixedpoint.Money{}); err == nil {
		t.Error("FloatOf(zero Money) succeeded; an unset amount must not export as 0")
	}
}

func floatOfText(t *testing.T, m fixedpoint.Money) (string, bool, error) {
	t.Helper()

	f, exact, err := observability.FloatOf(m)              //nofloat:allow the same wire seam
	return strconv.FormatFloat(f, 'g', -1, 64), exact, err //nofloat:allow rendering its output
}
