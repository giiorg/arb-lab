package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func group() domain.HedgeGroup {
	return domain.HedgeGroup{Account: account, Venue: "binance", Symbol: "BTCUSDT"}
}

// Holds spans the PRODUCT CATEGORY and nothing else — §5.1's "deliberately
// grouping a trade's spot + perp legs … and distinct from per-instrument
// identity".
//
// spot() and perp() differ only in Product and Contract, which is the fixture
// the collapse checklist forces: a pair built from two different symbols could
// not tell "compares venue and symbol" from "compares the whole instrument",
// and this is the pair that actually occurs — one cash-and-carry trade holds an
// order on each at the same instant.
//
// The negatives vary ONE component each, for the same reason: a foreign
// instrument differing in both venue and symbol is refused by either check
// alone.
func TestHedgeGroupHoldsBothLegsAndNothingElse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		instrument ids.VenueInstrument
		want       bool
	}{
		{"the spot leg", spot(), true},
		{"the perp leg", perp(), true},
		{
			name: "another symbol on the same venue",
			instrument: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductSpot, Symbol: "ETHUSDT",
			},
			want: false,
		},
		{
			name: "the same symbol on another venue",
			instrument: ids.VenueInstrument{
				Venue: "bybit", Product: ids.ProductSpot, Symbol: "BTCUSDT",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := group().Holds(tc.instrument); got != tc.want {
				t.Fatalf("Holds(%s/%s/%s) = %t, want %t",
					tc.instrument.Venue, tc.instrument.Product, tc.instrument.Symbol, got, tc.want)
			}
		})
	}
}

// Every component is required, and each is varied alone.
//
// The account row is the one this type exists for: §5.1 puts it first in the
// key, §5.1's own Order field list has no account in it, and a group missing it
// merges every subaccount trading the symbol — which nets a naked long in one
// against a naked short in another to a residual of zero.
func TestHedgeGroupValidateRequiresAllThreeComponents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		group domain.HedgeGroup
		want  error
	}{
		{
			name:  "no account",
			group: domain.HedgeGroup{Venue: "binance", Symbol: "BTCUSDT"},
			want:  domain.ErrHedgeGroupAccountUnset,
		},
		{
			name:  "no symbol",
			group: domain.HedgeGroup{Account: account, Venue: "binance"},
			want:  domain.ErrHedgeGroupSymbolUnset,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.group.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("no venue", func(t *testing.T) {
		t.Parallel()
		g := domain.HedgeGroup{Account: account, Symbol: "BTCUSDT"}
		if err := g.Validate(); err == nil {
			t.Fatal("Validate = nil, want a refusal from the venue")
		}
	})

	t.Run("fully populated", func(t *testing.T) {
		t.Parallel()
		if err := group().Validate(); err != nil {
			t.Fatalf("Validate = %v, want nil — the positive control", err)
		}
	})
}
