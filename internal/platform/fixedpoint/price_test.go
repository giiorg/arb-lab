package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

const eth = fixedpoint.Asset("ETH")

func mustPrice(t *testing.T, base, quote fixedpoint.Asset, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice(base, quote, s)
	if err != nil {
		t.Fatalf("ParsePrice(%q, %q, %q) failed: %v", base, quote, s, err)
	}
	return p
}

// A Price carries both assets because that is what makes the runtime tag catch
// something. With only one tag, multiplying a BTC quantity by an ETH price
// produces a plausible number; with base and quote it is an error.
func TestPrice_Construction(t *testing.T) {
	p := mustPrice(t, btc, usdt, "43250.5")
	if p.Base() != btc || p.Quote() != usdt {
		t.Errorf("ParsePrice lost a tag: base %q quote %q", p.Base(), p.Quote())
	}
	if got, want := p.Decimal(), "43250.5"; got != want {
		t.Errorf("Decimal() = %q; want %q", got, want)
	}
	if got, want := p.String(), "43250.5 USDT/BTC"; got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}

	var unset fixedpoint.Price
	if err := unset.Validate(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("the zero value validates as %v; want ErrAssetUnset", err)
	}
	if got, want := unset.String(), "<invalid price>"; got != want {
		t.Errorf("the zero value prints as %q; want %q", got, want)
	}

	for _, tc := range []struct{ base, quote fixedpoint.Asset }{
		{"", usdt},
		{btc, ""},
		{"", ""},
	} {
		if _, err := fixedpoint.ParsePrice(tc.base, tc.quote, "1"); !errors.Is(err, fixedpoint.ErrAssetUnset) {
			t.Errorf("ParsePrice(%q, %q) = %v; want ErrAssetUnset", tc.base, tc.quote, err)
		}
	}

	// A price of an asset in itself is degenerate, and in practice means an
	// adapter populated both tags from the same field.
	if _, err := fixedpoint.ParsePrice(btc, btc, "1"); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("ParsePrice(BTC, BTC) = %v; want ErrAssetMismatch", err)
	}

	// Negative prices are permitted: spreads and some settled contracts trade
	// below zero, and it is not this package's business to forbid that.
	if _, err := fixedpoint.ParsePrice(btc, usdt, "-1.5"); err != nil {
		t.Errorf("ParsePrice with a negative price failed: %v", err)
	}
}

func TestPrice_Cmp(t *testing.T) {
	a := mustPrice(t, btc, usdt, "100")
	b := mustPrice(t, btc, usdt, "200")
	if got, err := a.Cmp(b); err != nil || got != -1 {
		t.Errorf("Cmp = %d, %v; want -1, nil", got, err)
	}
	if got, err := b.Cmp(a); err != nil || got != 1 {
		t.Errorf("Cmp = %d, %v; want 1, nil", got, err)
	}

	// Two numbers being comparable does not make two prices comparable.
	if _, err := a.Cmp(mustPrice(t, eth, usdt, "100")); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Error("comparing a BTC price with an ETH price must fail with ErrAssetMismatch")
	}
	if _, err := a.Cmp(mustPrice(t, btc, eth, "100")); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Error("comparing prices quoted in different assets must fail with ErrAssetMismatch")
	}
	var unset fixedpoint.Price
	if _, err := a.Cmp(unset); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Error("comparing against an unset price must fail with ErrAssetUnset")
	}
}

// Notional is where the two tags earn their place: it consumes a Quantity of
// the price's base asset and produces Money in the price's quote asset. Both
// halves of that sentence are checked at run time.
func TestNotional(t *testing.T) {
	cases := []struct {
		name      string
		base      fixedpoint.Asset
		quote     fixedpoint.Asset
		price     string
		qtyAsset  fixedpoint.Asset
		qty       string
		mode      fixedpoint.RoundingMode
		want      string // "" means the operation must fail with err
		wantAsset fixedpoint.Asset
		err       error
	}{
		{
			name: "whole units", base: btc, quote: usdt, price: "50000",
			qtyAsset: btc, qty: "2", mode: fixedpoint.RoundExact,
			want: "100000", wantAsset: usdt,
		},
		{
			name: "a realistic fill", base: btc, quote: usdt, price: "43250.5",
			qtyAsset: btc, qty: "0.001", mode: fixedpoint.RoundExact,
			want: "43.2505", wantAsset: usdt,
		},
		{
			name: "a short position", base: btc, quote: usdt, price: "100",
			qtyAsset: btc, qty: "-0.5", mode: fixedpoint.RoundExact,
			want: "-50", wantAsset: usdt,
		},
		{
			// The quantity is of the wrong asset. Without the base tag this
			// would silently produce a number.
			name: "quantity of the wrong asset", base: btc, quote: usdt, price: "50000",
			qtyAsset: eth, qty: "2", mode: fixedpoint.RoundExact,
			err: fixedpoint.ErrAssetMismatch,
		},
		{
			// Below the canonical scale: the caller must say what happens.
			name: "sub-scale product, exact refused", base: btc, quote: usdt, price: ulp,
			qtyAsset: btc, qty: ulp, mode: fixedpoint.RoundExact,
			err: fixedpoint.ErrPrecisionLoss,
		},
		{
			name: "sub-scale product, truncated", base: btc, quote: usdt, price: ulp,
			qtyAsset: btc, qty: ulp, mode: fixedpoint.RoundTowardZero,
			want: "0", wantAsset: usdt,
		},
		{
			// A cost rounded away from zero is never understated — the mode a
			// fee or a slippage allowance should use.
			name: "sub-scale product, away from zero", base: btc, quote: usdt, price: ulp,
			qtyAsset: btc, qty: ulp, mode: fixedpoint.RoundAwayFromZero,
			want: ulp, wantAsset: usdt,
		},
		{
			name: "overflow", base: btc, quote: usdt, price: maxDecimal,
			qtyAsset: btc, qty: "2", mode: fixedpoint.RoundExact,
			err: fixedpoint.ErrOverflow,
		},
		{
			name: "no rounding mode", base: btc, quote: usdt, price: "50000",
			qtyAsset: btc, qty: "2", mode: fixedpoint.RoundInvalid,
			err: fixedpoint.ErrRoundingMode,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			px := mustPrice(t, tc.base, tc.quote, tc.price)
			qty := mustQuantity(t, tc.qtyAsset, tc.qty)

			got, err := fixedpoint.Notional(px, qty, tc.mode)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("Notional = %s, nil; want %v", got, tc.err)
				}
				if !errors.Is(err, tc.err) {
					t.Fatalf("Notional error = %v; want it to wrap %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Notional failed: %v", err)
			}
			if got.Decimal() != tc.want {
				t.Errorf("Notional = %s; want %s", got.Decimal(), tc.want)
			}
			if got.Asset() != tc.wantAsset {
				t.Errorf("Notional is tagged %q; want the price's quote asset %q", got.Asset(), tc.wantAsset)
			}
		})
	}

	// Unconstructed operands, both sides.
	var unsetPx fixedpoint.Price
	var unsetQty fixedpoint.Quantity
	if _, err := fixedpoint.Notional(unsetPx, mustQuantity(t, btc, "1"), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Notional with an unset price = %v; want ErrAssetUnset", err)
	}
	if _, err := fixedpoint.Notional(mustPrice(t, btc, usdt, "1"), unsetQty, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Notional with an unset quantity = %v; want ErrAssetUnset", err)
	}
}

// Money.Mul(Rate) is how every fee and every funding payment is computed.
func TestMoney_MulRate(t *testing.T) {
	notional := mustMoney(t, usdt, "100000")

	cases := []struct {
		name string
		rate string
		mode fixedpoint.RoundingMode
		want string
		err  error
	}{
		{name: "funding at one basis point", rate: "0.0001", mode: fixedpoint.RoundExact, want: "10"},
		{name: "a taker fee", rate: "0.00045", mode: fixedpoint.RoundExact, want: "45"},
		{name: "a negative funding rate", rate: "-0.0001", mode: fixedpoint.RoundExact, want: "-10"},
		{name: "zero rate", rate: "0", mode: fixedpoint.RoundExact, want: "0"},
		{name: "no rounding mode", rate: "0.0001", mode: fixedpoint.RoundInvalid, err: fixedpoint.ErrRoundingMode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := notional.Mul(mustRate(t, tc.rate), tc.mode)
			if tc.want == "" {
				if !errors.Is(err, tc.err) {
					t.Fatalf("Mul error = %v; want it to wrap %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Mul failed: %v", err)
			}
			if got.Decimal() != tc.want {
				t.Errorf("Mul = %s; want %s", got.Decimal(), tc.want)
			}
			if got.Asset() != usdt {
				t.Errorf("Mul is tagged %q; want the money's asset unchanged", got.Asset())
			}
		})
	}

	// A rate nobody populated must not read as "no fee".
	var unset fixedpoint.Rate
	if _, err := notional.Mul(unset, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrRateUnset) {
		t.Errorf("Mul by an unconstructed rate = %v; want ErrRateUnset", err)
	}
	var unsetMoney fixedpoint.Money
	if _, err := unsetMoney.Mul(mustRate(t, "0.5"), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("Mul on an unset Money = %v; want ErrAssetUnset", err)
	}
}

// Dividing two amounts of the same asset cancels the units, so the result is a
// Rate rather than Money. This is how a carry is expressed against the capital
// that earned it — milestone 2's headline number.
func TestMoney_Div(t *testing.T) {
	capital := mustMoney(t, usdt, "100000")

	got, err := mustMoney(t, usdt, "10").Div(capital, fixedpoint.RoundExact)
	if err != nil {
		t.Fatalf("Div failed: %v", err)
	}
	if want := "0.0001"; got.Decimal() != want {
		t.Errorf("10 / 100000 = %s; want %s", got.Decimal(), want)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("the resulting Rate is not constructed: %v", err)
	}

	// A third does not land on the canonical scale, so the mode decides.
	third := mustMoney(t, usdt, "1")
	three := mustMoney(t, usdt, "3")
	for _, tc := range []struct {
		mode fixedpoint.RoundingMode
		want string
		err  error
	}{
		{mode: fixedpoint.RoundExact, err: fixedpoint.ErrPrecisionLoss},
		{mode: fixedpoint.RoundTowardZero, want: "0.333333333"},
		{mode: fixedpoint.RoundAwayFromZero, want: "0.333333334"},
		{mode: fixedpoint.RoundHalfEven, want: "0.333333333"},
	} {
		got, err := third.Div(three, tc.mode)
		if tc.want == "" {
			if !errors.Is(err, tc.err) {
				t.Errorf("Div with %v = %v; want %v", tc.mode, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Div with %v failed: %v", tc.mode, err)
			continue
		}
		if got.Decimal() != tc.want {
			t.Errorf("1/3 with %v = %s; want %s", tc.mode, got.Decimal(), tc.want)
		}
	}

	if _, err := capital.Div(fixedpoint.ZeroMoney(usdt), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrDivideByZero) {
		t.Errorf("dividing by zero = %v; want ErrDivideByZero", err)
	}
	if _, err := capital.Div(mustMoney(t, btc, "1"), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("dividing USDT by BTC = %v; want ErrAssetMismatch", err)
	}
	var unset fixedpoint.Money
	if _, err := capital.Div(unset, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("dividing by an unset Money = %v; want ErrAssetUnset", err)
	}
}

// AveragePrice is Notional's inverse: it recovers the price implied by a total
// and a size, which is how an OMS reports an average fill across partials.
func TestAveragePrice(t *testing.T) {
	total := mustMoney(t, usdt, "100000")
	qty := mustQuantity(t, btc, "2")

	px, err := fixedpoint.AveragePrice(total, qty, fixedpoint.RoundExact)
	if err != nil {
		t.Fatalf("AveragePrice failed: %v", err)
	}
	if got, want := px.Decimal(), "50000"; got != want {
		t.Errorf("AveragePrice = %s; want %s", got, want)
	}
	if px.Base() != btc || px.Quote() != usdt {
		t.Errorf("AveragePrice is tagged %s/%s; want the size's asset priced in the total's asset",
			px.Quote(), px.Base())
	}

	if _, err := fixedpoint.AveragePrice(total, fixedpoint.ZeroQuantity(btc), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrDivideByZero) {
		t.Errorf("AveragePrice over a flat position = %v; want ErrDivideByZero", err)
	}
	// A total and a size in the same asset would produce a degenerate price.
	if _, err := fixedpoint.AveragePrice(total, mustQuantity(t, usdt, "2"), fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("AveragePrice with matching assets = %v; want ErrAssetMismatch", err)
	}
}

// Notional and AveragePrice must be inverses on values where no rounding
// intervenes. Stating it as a relation between the two operations means neither
// can drift without the other, which a pair of independent tables would allow.
func TestNotionalAndAveragePriceAreInverse(t *testing.T) {
	for _, tc := range []struct{ price, qty string }{
		{"50000", "2"},
		{"43250.5", "0.001"},
		{"0.25", "4"},
		{"100", "-0.5"},
		{"1", "1"},
	} {
		px := mustPrice(t, btc, usdt, tc.price)
		qty := mustQuantity(t, btc, tc.qty)

		total, err := fixedpoint.Notional(px, qty, fixedpoint.RoundExact)
		if err != nil {
			t.Fatalf("Notional(%s, %s) failed: %v", tc.price, tc.qty, err)
		}
		back, err := fixedpoint.AveragePrice(total, qty, fixedpoint.RoundExact)
		if err != nil {
			t.Fatalf("AveragePrice(%s, %s) failed: %v", total, tc.qty, err)
		}
		if back != px {
			t.Errorf("round trip changed the price: %s -> %s -> %s", px, total, back)
		}
	}
}
