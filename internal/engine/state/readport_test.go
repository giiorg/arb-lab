package state_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

// The StrategyReader battery: the translation is a projection, absence is
// false, and a topless book is absent rather than half-priced.

func newReader(t *testing.T) (*state.Cache, state.StrategyReader) {
	t.Helper()
	c := state.NewCache()
	return c, state.NewStrategyReader(c)
}

func TestStrategyReaderFundingIsTheAcceptedObservation(t *testing.T) {
	c, r := newReader(t)
	vi := perp()

	if _, ok := r.Funding(vi); ok {
		t.Fatal("funding answered before anything was put")
	}

	snap := fundingAt(t, vi, 0)
	if err := c.PutFunding(snap); err != nil {
		t.Fatal(err)
	}
	got, ok := r.Funding(vi)
	if !ok {
		t.Fatal("funding absent after put")
	}
	if got != snap.Observation {
		t.Fatalf("observation diverged:\n was %+v\n got %+v", snap.Observation, got)
	}
}

func TestStrategyReaderTopOfBookIsBestBidAndAsk(t *testing.T) {
	c, r := newReader(t)
	vi := spot()

	if _, ok := r.TopOfBook(vi); ok {
		t.Fatal("book answered before anything was put")
	}

	view := bookAt(t, vi, 0, 7)
	// Two levels a side, best NOT equal to the only level, so "best" and
	// "any" are different answers (the fixture-collapse rule).
	view.Book.Bids = append(view.Book.Bids, domain.BookLevel{
		Price: price(t, "99"), Quantity: qty(t, "5"),
	})
	view.Book.Asks = append(view.Book.Asks, domain.BookLevel{
		Price: price(t, "102"), Quantity: qty(t, "4"),
	})
	if err := c.PutBook(view); err != nil {
		t.Fatal(err)
	}

	pv, ok := r.TopOfBook(vi)
	if !ok {
		t.Fatal("top of book absent after put")
	}
	if pv.Instrument != vi {
		t.Fatalf("instrument %s", pv.Instrument)
	}
	if pv.Bid != price(t, "100") || pv.Ask != price(t, "101") {
		t.Fatalf("top %s / %s", pv.Bid, pv.Ask)
	}
	if pv.Watermark != view.Book.Watermark {
		t.Fatalf("watermark diverged: %+v", pv.Watermark)
	}
	if err := pv.Validate(); err != nil {
		t.Fatalf("translated view invalid: %v", err)
	}
}

func TestStrategyReaderRefusesHalfABook(t *testing.T) {
	c, r := newReader(t)
	vi := spot()

	// A book with bids and no asks validates as a snapshot (a one-sided
	// market is a real venue state) — but it has no top to price against.
	view := bookAt(t, vi, 0, 7)
	view.Book.Asks = nil
	if err := c.PutBook(view); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.TopOfBook(vi); ok {
		t.Fatal("half a book answered as a top")
	}
}
