package observability_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

func TestSampleZeroValueIsUnusable(t *testing.T) {
	t.Parallel()

	var s observability.Sample
	if err := s.Validate(); !errors.Is(err, observability.ErrSampleUnset) {
		t.Fatalf("zero Sample Validate() = %v; want ErrSampleUnset", err)
	}
	if s.Unit() != observability.UnitUnspecified {
		t.Errorf("zero Sample unit = %v; want UnitUnspecified", s.Unit())
	}
}

// TestSecondsIsExactAtCanonicalScale pins the identity Seconds relies on:
// time.Duration counts nanoseconds and fixedpoint.Scale is 9, so a duration's
// nanosecond count IS its seconds mantissa.
//
// The compile-time assertion in sample.go pins Scale itself; this pins that the
// conversion actually uses it, in both signs and at both ends of the range.
func TestSecondsIsExactAtCanonicalScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want int64
	}{
		{"zero", 0, 0},
		{"one nanosecond", time.Nanosecond, 1},
		{"one second", time.Second, 1_000_000_000},
		{"one and a half seconds", 1500 * time.Millisecond, 1_500_000_000},
		{"negative, because skew is signed", -250 * time.Millisecond, -250_000_000},
		{"the RTO budget in §9", 5 * time.Second, 5_000_000_000},
		{"max duration", time.Duration(math.MaxInt64), math.MaxInt64},
		{"min duration", time.Duration(math.MinInt64), math.MinInt64},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := observability.Seconds(tc.in)
			if got := s.Mantissa(); got != tc.want {
				t.Errorf("Seconds(%v).Mantissa() = %d; want %d", tc.in, got, tc.want)
			}
			if s.Unit() != observability.UnitSeconds {
				t.Errorf("unit = %v; want UnitSeconds", s.Unit())
			}
		})
	}
}

// TestCountIsNotScaled is the other half of the asymmetry Mantissa documents. A
// tally of three is three, not three billion: scaling it would spend nine of
// int64's digits on a fractional part a count cannot have and would put the
// ceiling at ~9.2e9 events.
func TestCountIsNotScaled(t *testing.T) {
	t.Parallel()

	if got := observability.Count(3).Mantissa(); got != 3 {
		t.Errorf("Count(3).Mantissa() = %d; want 3", got)
	}
	if got := observability.Bytes(4096).Mantissa(); got != 4096 {
		t.Errorf("Bytes(4096).Mantissa() = %d; want 4096", got)
	}
}

func TestRatioAndMoneyRefuseUnconstructedValues(t *testing.T) {
	t.Parallel()

	// The zero Rate is fixedpoint's "nobody populated this". Admitting it would
	// put a fee rate of zero on the panel used to decide whether trading pays.
	if _, err := observability.Ratio(fixedpoint.Rate{}); err == nil {
		t.Error("Ratio(zero Rate) succeeded; an unset rate reads as zero fees")
	}
	if _, err := observability.MoneyOf(fixedpoint.Money{}); err == nil {
		t.Error("MoneyOf(zero Money) succeeded; an unset cost reads as free")
	}

	// Positive control: constructed values pass, or the two checks above are
	// satisfied by a function that refuses everything.
	r, err := fixedpoint.ParseRate("0.0001")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}
	if _, err := observability.Ratio(r); err != nil {
		t.Errorf("Ratio(0.0001) = %v; want nil", err)
	}
	m := mustMoney(t, "USD", "12.5")
	s, err := observability.MoneyOf(m)
	if err != nil {
		t.Fatalf("MoneyOf: %v", err)
	}
	if got, want := s.Mantissa(), int64(12_500_000_000); got != want {
		t.Errorf("mantissa = %d; want %d", got, want)
	}
	if s.Asset() != "USD" {
		t.Errorf("asset = %q; want USD", s.Asset())
	}
}

func TestSampleSign(t *testing.T) {
	t.Parallel()

	if got := observability.Count(-1).Sign(); got != -1 {
		t.Errorf("Sign(-1) = %d; want -1", got)
	}
	if got := observability.Count(0).Sign(); got != 0 {
		t.Errorf("Sign(0) = %d; want 0", got)
	}
	if got := observability.Count(1).Sign(); got != 1 {
		t.Errorf("Sign(1) = %d; want 1", got)
	}
}

// TestSampleStringNamesTheAssetForMoney matters because these strings appear in
// ErrAssetMismatch, whose whole job is to say which two denominations
// disagreed.
func TestSampleStringNamesTheAssetForMoney(t *testing.T) {
	t.Parallel()

	s, err := observability.MoneyOf(mustMoney(t, "BTC", "1"))
	if err != nil {
		t.Fatalf("MoneyOf: %v", err)
	}
	if got := s.String(); got != "1000000000 UnitMoney(BTC)" {
		t.Errorf("String() = %q; want the asset named", got)
	}
	if got := observability.Count(7).String(); got != "7 UnitCount" {
		t.Errorf("String() = %q; want no asset on a non-money sample", got)
	}
}

func mustMoney(t *testing.T, asset, dec string) fixedpoint.Money {
	t.Helper()

	m, err := fixedpoint.ParseMoney(fixedpoint.Asset(asset), dec)
	if err != nil {
		t.Fatalf("ParseMoney(%s, %s): %v", asset, dec, err)
	}
	return m
}
