package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// validFilters is the battery's base fixture. Every member carries a DIFFERENT
// number (step 0.001, min qty 0.002, tick 0.1, min notional 50) so a validator
// reading one member where it means another is distinguishable from a correct
// one — the fee-orientation lesson, applied before the refusals rather than
// after a survivor.
func validFilters(t *testing.T) domain.TradingFilters {
	t.Helper()
	return domain.TradingFilters{
		Step:        qty(t, "0.001"),
		MinQty:      qty(t, "0.002"),
		Tick:        price(t, "0.1"),
		MinNotional: filterUSDT(t, "50"),
	}
}

func filterUSDT(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

// TestTradingFiltersValidate: the positive control, then the declared-zero
// floors — the one distinction the two floor fields exist to carry (a venue
// MAY publish a zero floor; an UNSET floor must refuse), which makes zero the
// exact value where "floor" and "unpopulated" would collapse without the
// asset-tag split.
func TestTradingFiltersValidate(t *testing.T) {
	t.Parallel()

	if err := validFilters(t).Validate(); err != nil {
		t.Fatalf("valid filters refused: %v", err)
	}

	f := validFilters(t)
	f.MinQty = fixedpoint.ZeroQuantity("BTC")
	if err := f.Validate(); err != nil {
		t.Errorf("a declared-zero min qty refused: %v — venues publish these", err)
	}

	f = validFilters(t)
	f.MinNotional = fixedpoint.ZeroMoney("USDT")
	if err := f.Validate(); err != nil {
		t.Errorf("a declared-zero min notional refused: %v — Bybit's linear perp publishes one", err)
	}
}

// TestTradingFiltersZeroValueIsUnusable is §11's rule for this type: an
// unpopulated filters value verifies nothing, so it must refuse rather than
// read as "no constraint" — the flattering direction for every one of its
// four members.
func TestTradingFiltersZeroValueIsUnusable(t *testing.T) {
	t.Parallel()
	if err := (domain.TradingFilters{}).Validate(); err == nil {
		t.Fatal("zero TradingFilters validated")
	}
}

func TestTradingFiltersRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		edit  func(t *testing.T, f *domain.TradingFilters)
		want  error
		names string // the member the message must point at
	}{
		{
			// The zero that arrives from real data: Binance publishes
			// stepSize "0.00000000" on filters that do not apply.
			"a zero step reads as no constraint",
			func(t *testing.T, f *domain.TradingFilters) { f.Step = fixedpoint.ZeroQuantity("BTC") },
			domain.ErrFilterStepNotPositive, "step",
		},
		{
			"a negative step",
			func(t *testing.T, f *domain.TradingFilters) { f.Step = qty(t, "-0.001") },
			domain.ErrFilterStepNotPositive, "step",
		},
		{
			"an unset step",
			func(t *testing.T, f *domain.TradingFilters) { f.Step = fixedpoint.Quantity{} },
			nil, "step",
		},
		{
			"a negative min qty",
			func(t *testing.T, f *domain.TradingFilters) { f.MinQty = qty(t, "-0.002") },
			domain.ErrFilterFloorNegative, "min qty",
		},
		{
			"an unset min qty",
			func(t *testing.T, f *domain.TradingFilters) { f.MinQty = fixedpoint.Quantity{} },
			nil, "min qty",
		},
		{
			"a zero tick reads as every price conforming",
			func(t *testing.T, f *domain.TradingFilters) {
				p, err := fixedpoint.ParsePrice("BTC", "USDT", "0")
				if err != nil {
					t.Fatalf("ParsePrice: %v", err)
				}
				f.Tick = p
			},
			domain.ErrFilterTickNotPositive, "tick",
		},
		{
			"a negative tick",
			func(t *testing.T, f *domain.TradingFilters) { f.Tick = price(t, "-0.1") },
			domain.ErrFilterTickNotPositive, "tick",
		},
		{
			"an unset tick",
			func(t *testing.T, f *domain.TradingFilters) { f.Tick = fixedpoint.Price{} },
			nil, "tick",
		},
		{
			"a negative min notional",
			func(t *testing.T, f *domain.TradingFilters) { f.MinNotional = filterUSDT(t, "-50") },
			domain.ErrFilterFloorNegative, "min notional",
		},
		{
			"an unset min notional",
			func(t *testing.T, f *domain.TradingFilters) { f.MinNotional = fixedpoint.Money{} },
			nil, "min notional",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := validFilters(t)
			tc.edit(t, &f)
			err := f.Validate()
			if err == nil {
				t.Fatal("Validate() = nil")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("Validate() = %q, message does not name %q", err, tc.names)
			}
		})
	}
}

// TestTradingFiltersAssetCoherence: each of the three cross-member checks,
// isolated — the edited member is internally VALID in its wrong asset, so
// only the cross-member comparison can object, and each case trips a
// DIFFERENT comparison so a deleted check is named by its own case rather
// than shadowed by a sibling.
func TestTradingFiltersAssetCoherence(t *testing.T) {
	t.Parallel()

	ethQty := func(t *testing.T, s string) fixedpoint.Quantity {
		t.Helper()
		q, err := fixedpoint.ParseQuantity("ETH", s)
		if err != nil {
			t.Fatalf("ParseQuantity: %v", err)
		}
		return q
	}

	cases := []struct {
		name string
		edit func(t *testing.T, f *domain.TradingFilters)
	}{
		{
			"min qty counting a different base than step",
			func(t *testing.T, f *domain.TradingFilters) { f.MinQty = ethQty(t, "0.002") },
		},
		{
			"tick pricing a different base than step counts",
			func(t *testing.T, f *domain.TradingFilters) {
				p, err := fixedpoint.ParsePrice("ETH", "USDT", "0.1")
				if err != nil {
					t.Fatalf("ParsePrice: %v", err)
				}
				f.Tick = p
			},
		},
		{
			"min notional in a different quote than tick",
			func(t *testing.T, f *domain.TradingFilters) {
				m, err := fixedpoint.ParseMoney("USDC", "50")
				if err != nil {
					t.Fatalf("ParseMoney: %v", err)
				}
				f.MinNotional = m
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := validFilters(t)
			tc.edit(t, &f)
			if err := f.Validate(); !errors.Is(err, domain.ErrFilterAssetsDisagree) {
				t.Errorf("Validate() = %v, want ErrFilterAssetsDisagree", err)
			}
		})
	}
}
