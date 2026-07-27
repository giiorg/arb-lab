package venue_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func linearPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

// TestFundingSourceNamesTheFundingStream asserts the STREAM, not merely that the
// result validates, and that distinction is the whole test.
//
// FundingSource's one real decision is which StreamKind constant it writes. A
// version writing StreamMark would return a source that validates identically —
// mark is perp-only too, so the product check passes, and the instrument is
// unchanged. Nothing about validity separates the two. What separates them is
// §3.4's mailbox: funding readings published to the mark mailbox leave the
// funding consumer hearing nothing, which presents as a dead feed rather than
// as a routing mistake.
func TestFundingSourceNamesTheFundingStream(t *testing.T) {
	src, err := venue.FundingSource(linearPerp())
	if err != nil {
		t.Fatalf("FundingSource(perp) = %v, want nil", err)
	}
	if src.Stream != events.StreamFunding {
		t.Errorf("Stream = %q, want %q — the mailbox key is the routing decision", src.Stream, events.StreamFunding)
	}
	if src.Instrument != linearPerp() {
		t.Errorf("Instrument = %v, want %v", src.Instrument, linearPerp())
	}
	if got := src.Lane(); got != events.LaneMarketData {
		t.Errorf("Lane() = %v, want LaneMarketData", got)
	}
}

// TestFundingSourceAcceptsAnInversePerp is the positive control for the product
// check, and it exists because every other perp fixture in this repo is linear.
//
// With only linear fixtures, "is a perp" and "is ProductLinearPerp" are the same
// predicate and the narrower one would pass everything. An inverse perp has
// funding — §5.1 declares the product precisely so that "this contract is
// linear" is a checked claim — so this is the case that would break under the
// narrower reading.
func TestFundingSourceAcceptsAnInversePerp(t *testing.T) {
	inverse := ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductInversePerp,
		Symbol: "BTCUSD", Contract: ids.ContractInverse,
	}
	src, err := venue.FundingSource(inverse)
	if err != nil {
		t.Fatalf("FundingSource(inverse perp) = %v, want nil — an inverse perp pays funding", err)
	}
	if src.Instrument.Product != ids.ProductInversePerp {
		t.Errorf("Product = %q, want %q", src.Instrument.Product, ids.ProductInversePerp)
	}
}

func TestFundingSourceRefuses(t *testing.T) {
	cases := []struct {
		name string
		inst ids.VenueInstrument
		want error
	}{
		{
			// §4 marks StreamFunding perp-only. Spot pays no funding at all,
			// and §2.2 holds spot against a perp — a spot leg reading zero is
			// indistinguishable from a perp in a flat regime, and the carry
			// computation nets the two.
			name: "spot instrument",
			inst: ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"},
			want: events.ErrStreamProductMismatch,
		},
		{
			// Delegated to events.SourceID, which delegates to ids: one
			// definition of a valid instrument, and this asserts the delegation
			// rather than restating the rule.
			name: "instrument unset",
			inst: ids.VenueInstrument{},
			want: ids.ErrVenueUnset,
		},
		{
			name: "perp with the wrong contract type",
			inst: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductLinearPerp,
				Symbol: "BTCUSDT", Contract: ids.ContractInverse,
			},
			want: ids.ErrContractMismatch,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := venue.FundingSource(c.inst)
			if !errors.Is(err, c.want) {
				t.Fatalf("FundingSource(%v) error = %v, want %v", c.inst, err, c.want)
			}
			// The zero source rather than a half-built one. A returned value
			// alongside an error is a value some caller will use, and a
			// SourceID carrying the refused instrument would be a mailbox key
			// built out of the mistake.
			if src != (events.SourceID{}) {
				t.Errorf("FundingSource returned %v alongside its error, want the zero SourceID", src)
			}
		})
	}
}
