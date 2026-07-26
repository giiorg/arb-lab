package fixedpoint_test

import (
	"errors"
	"math"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The persistence seam.
//
// This is a *value* codec, not a schema: it converts between a category and the
// pair of columns a row holds — a mantissa and an asset name — and says nothing
// about tables. The mantissa is an int64, which is exactly SQLite INTEGER and
// exactly Postgres BIGINT, so the encoding is lossless and identical on both.
// That is the property that makes the documented SQLite-to-Postgres path (D2)
// cheap, and it is why the 128-bit representation was rejected: its codec would
// have diverged per dialect (TEXT on one, NUMERIC on the other) and the
// divergence would have leaked into every query that compares or sums a column.
//
// Storing the decimal text instead would round-trip too, and would break
// ordering and SUM() in the database — the two things a mantissa column gets
// for free.
func TestCodec_MantissaRoundTrip(t *testing.T) {
	// Boundary mantissas, written as integers because that is what a row
	// holds. The int64 bounds are included because they are exactly what a
	// BIGINT column can hold, no more and no less.
	mantissas := []int64{
		0, 1, -1, 500_000_000, -500_000_000,
		1_000_000_000, 123_456_789_123_456_789,
		math.MaxInt64, math.MinInt64,
	}

	for _, m := range mantissas {
		money, err := fixedpoint.MoneyFromMantissa(usdt, m)
		if err != nil {
			t.Fatalf("MoneyFromMantissa(%d) failed: %v", m, err)
		}
		if got := money.Mantissa(); got != m {
			t.Errorf("Money mantissa round trip: %d -> %d", m, got)
		}
		if money.Asset() != usdt {
			t.Errorf("MoneyFromMantissa lost the asset tag")
		}

		qty, err := fixedpoint.QuantityFromMantissa(btc, m)
		if err != nil {
			t.Fatalf("QuantityFromMantissa(%d) failed: %v", m, err)
		}
		if got := qty.Mantissa(); got != m {
			t.Errorf("Quantity mantissa round trip: %d -> %d", m, got)
		}

		px, err := fixedpoint.PriceFromMantissa(btc, usdt, m)
		if err != nil {
			t.Fatalf("PriceFromMantissa(%d) failed: %v", m, err)
		}
		if got := px.Mantissa(); got != m {
			t.Errorf("Price mantissa round trip: %d -> %d", m, got)
		}

		rate := fixedpoint.RateFromMantissa(m)
		if got := rate.Mantissa(); got != m {
			t.Errorf("Rate mantissa round trip: %d -> %d", m, got)
		}
		if err := rate.Validate(); err != nil {
			t.Errorf("RateFromMantissa produced an unconstructed Rate: %v", err)
		}
	}
}

// The mantissa and the decimal text must describe the same number, so a column
// read back through either path agrees. This is what lets a migration or an
// ad-hoc query reason about the column without going through this package.
func TestCodec_MantissaAgreesWithDecimal(t *testing.T) {
	for _, tc := range []struct {
		decimal  string
		mantissa int64
	}{
		{"0", 0},
		{"1", 1_000_000_000},
		{"-1", -1_000_000_000},
		{"0.5", 500_000_000},
		{"0.000000001", 1},
		{"-0.000000001", -1},
		{"43250.5", 43_250_500_000_000},
		{maxDecimal, math.MaxInt64},
		{minDecimal, math.MinInt64},
	} {
		parsed := mustMoney(t, usdt, tc.decimal)
		if got := parsed.Mantissa(); got != tc.mantissa {
			t.Errorf("ParseMoney(%q).Mantissa() = %d; want %d", tc.decimal, got, tc.mantissa)
		}
		built, err := fixedpoint.MoneyFromMantissa(usdt, tc.mantissa)
		if err != nil {
			t.Fatalf("MoneyFromMantissa(%d) failed: %v", tc.mantissa, err)
		}
		if got := built.Decimal(); got != tc.decimal {
			t.Errorf("MoneyFromMantissa(%d).Decimal() = %q; want %q", tc.mantissa, got, tc.decimal)
		}
		if built != parsed {
			t.Errorf("the two construction paths disagree for %q", tc.decimal)
		}
	}
}

// Reading a row whose asset column is empty must fail. A NULL or blank asset is
// how a partially-written row reaches the codec, and the whole point of the
// invalid zero value is that such a row cannot become a usable amount.
func TestCodec_RejectsMissingTags(t *testing.T) {
	if _, err := fixedpoint.MoneyFromMantissa("", 1); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("MoneyFromMantissa with no asset = %v; want ErrAssetUnset", err)
	}
	if _, err := fixedpoint.QuantityFromMantissa("", 1); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("QuantityFromMantissa with no asset = %v; want ErrAssetUnset", err)
	}
	if _, err := fixedpoint.PriceFromMantissa("", usdt, 1); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("PriceFromMantissa with no base = %v; want ErrAssetUnset", err)
	}
	if _, err := fixedpoint.PriceFromMantissa(btc, "", 1); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("PriceFromMantissa with no quote = %v; want ErrAssetUnset", err)
	}
	if _, err := fixedpoint.PriceFromMantissa(btc, btc, 1); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("PriceFromMantissa with base == quote = %v; want ErrAssetMismatch", err)
	}
}

// Mantissa on an unconstructed value must not read as zero: a row written from
// a Money nobody populated would otherwise store a legitimate-looking 0.
func TestCodec_UnconstructedValuesCannotBeStored(t *testing.T) {
	var m fixedpoint.Money
	if err := m.Validate(); err == nil {
		t.Fatal("precondition: the zero Money must be invalid")
	}
	var r fixedpoint.Rate
	if err := r.Validate(); err == nil {
		t.Fatal("precondition: the zero Rate must be invalid")
	}
	// Validate is the gate a repository is expected to call before storing;
	// this pins that it is the gate that actually catches this.
	if _, err := fixedpoint.MoneyFromMantissa(m.Asset(), m.Mantissa()); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("round-tripping an unset Money through the codec = %v; want ErrAssetUnset", err)
	}
}

// The arithmetic must not allocate. Every one of these runs inside the
// per-settlement accounting loop, and a package that allocates on each fee
// calculation is one people route around.
func TestArithmeticDoesNotAllocate(t *testing.T) {
	a := mustMoney(t, usdt, "100000")
	b := mustMoney(t, usdt, "45")
	r := mustRate(t, "0.00045")
	px := mustPrice(t, btc, usdt, "43250.5")
	qty := mustQuantity(t, btc, "0.001")

	ops := map[string]func(){
		"Money.Add": func() { _, _ = a.Add(b) },
		"Money.Sub": func() { _, _ = a.Sub(b) },
		"Money.Mul": func() { _, _ = a.Mul(r, fixedpoint.RoundHalfEven) },
		"Money.Div": func() { _, _ = a.Div(b, fixedpoint.RoundHalfEven) },
		"Money.Cmp": func() { _, _ = a.Cmp(b) },
		"Money.Neg": func() { _, _ = a.Neg() },
		"Notional":  func() { _, _ = fixedpoint.Notional(px, qty, fixedpoint.RoundHalfEven) },
		"AveragePrice": func() {
			_, _ = fixedpoint.AveragePrice(a, qty, fixedpoint.RoundHalfEven)
		},
		"Quantity.Add": func() { _, _ = qty.Add(qty) },
		"Rate.Mul":     func() { _, _ = r.Mul(r, fixedpoint.RoundHalfEven) },
		"Mantissa":     func() { _ = a.Mantissa() },
	}
	for name, op := range ops {
		if got := testing.AllocsPerRun(100, op); got != 0 {
			t.Errorf("%s allocated %.0f times per run; want 0", name, got)
		}
	}
}
