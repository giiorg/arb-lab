package venue_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func spot() ids.VenueInstrument {
	// No Contract: ids.ErrContractOnSpot refuses a settlement type on a
	// non-perp, and §5.1's canonical form leaves it empty.
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

// TestBookSourceNamesTheBookStream is FundingSource's assertion applied to its
// mirror: what is checked is the StreamKind constant, not that the result
// validates.
//
// A version writing StreamMark here returns a source that validates identically
// for a perp — same instrument, and mark is a market-data stream too. Nothing
// about validity separates them. What separates them is §3.4's mailbox, and the
// symptom of getting it wrong is a book consumer that never hears from the
// instrument, which presents as a venue outage.
func TestBookSourceNamesTheBookStream(t *testing.T) {
	src, err := venue.BookSource(linearPerp())
	if err != nil {
		t.Fatalf("BookSource(perp) = %v, want nil", err)
	}
	if src.Stream != events.StreamBook {
		t.Errorf("Stream = %q, want %q — the mailbox key is the routing decision", src.Stream, events.StreamBook)
	}
	if src.Instrument != linearPerp() {
		t.Errorf("Instrument = %v, want %v", src.Instrument, linearPerp())
	}
	if got := src.Lane(); got != events.LaneMarketData {
		t.Errorf("Lane() = %v, want LaneMarketData", got)
	}
}

// TestBookSourceAcceptsSpot is the case that makes BookSource a different
// function from FundingSource rather than a copy of it.
//
// §3.5's product table gives StreamBook to both families and StreamFunding to
// perps alone, and §4.2 makes the spot book how the spot leg is priced. So this
// asserting NIL is the whole claim: a constructor written by copying
// FundingSource and swapping the token would pass every other test in this file
// and would silently unsubscribe half of every cash-and-carry trade — the
// failure §3.5 calls out by name when it explains why the constraint lives on
// SourceID.
//
// The perp case above is not enough on its own. Both constructors accept a perp,
// so a fixture that only ever passes one tests nothing about the asymmetry.
func TestBookSourceAcceptsSpot(t *testing.T) {
	src, err := venue.BookSource(spot())
	if err != nil {
		t.Fatalf("BookSource(spot) = %v, want nil — a spot book is how §4.2 prices the spot leg", err)
	}
	if src.Instrument.Product != ids.ProductSpot {
		t.Errorf("Product = %q, want %q", src.Instrument.Product, ids.ProductSpot)
	}

	// The control, at the same call: funding on that same spot instrument MUST
	// be refused. Without it, a RequiresPerp that returned false for everything
	// would make the assertion above pass for the wrong reason.
	if _, err := venue.FundingSource(spot()); !errors.Is(err, events.ErrStreamProductMismatch) {
		t.Errorf("FundingSource(spot) = %v, want ErrStreamProductMismatch", err)
	}
}

// TestBookSourceRefusesAMalformedInstrument checks that the remaining refusal is
// still made, and that it is events' error rather than a renamed local one — a
// caller branching on the rule's owner keeps working when a third stream
// constructor lands beside these two.
func TestBookSourceRefusesAMalformedInstrument(t *testing.T) {
	if _, err := venue.BookSource(ids.VenueInstrument{}); !errors.Is(err, ids.ErrVenueUnset) {
		t.Errorf("BookSource(zero) = %v, want ErrVenueUnset", err)
	}
}
