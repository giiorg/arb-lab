package crossvenue_test

// WalkedFill battery. Fixture discipline (§45g's suspects, applied before
// the battery): the walked quantity CROSSES levels — an order smaller than
// level 1 never walks, so a fixture that fits in the best level collapses
// the walk into a multiplication; one case drains the side EXACTLY, because
// exact-fill and one-unit-short sit on the two sides of the depth-short
// refusal and a wrong end-check conflates them; and the two sides' books
// are asymmetric, so the bid/ask orientation is pinned by different
// numbers, not by symmetry.

import (
	"errors"
	"testing"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

var depthInst = ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "ABCUSDT"}

func depthBook(t testing.TB, bids, asks [][2]string) mdomain.BookSnapshot {
	t.Helper()
	mk := func(pairs [][2]string) []mdomain.BookLevel {
		levels := make([]mdomain.BookLevel, 0, len(pairs))
		for _, p := range pairs {
			price, err := fixedpoint.ParsePrice("ABC", "USDT", p[0])
			if err != nil {
				t.Fatalf("price %q: %v", p[0], err)
			}
			q, err := fixedpoint.ParseQuantity("ABC", p[1])
			if err != nil {
				t.Fatalf("qty %q: %v", p[1], err)
			}
			levels = append(levels, mdomain.BookLevel{Price: price, Quantity: q})
		}
		return levels
	}
	b := mdomain.BookSnapshot{
		Instrument: depthInst,
		Bids:       mk(bids),
		Asks:       mk(asks),
		Watermark:  mdomain.Watermark{SourceTime: cvBase, RecvTime: cvBase, VenueSeq: 7},
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("fixture book: %v", err)
	}
	return b
}

func depthQty(t testing.TB, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("ABC", s)
	if err != nil {
		t.Fatalf("qty %q: %v", s, err)
	}
	return q
}

// The asymmetric two-level book every case below reads:
// asks 100.00×1 then 101.00×1 (a buy of 2 pays 201 against ref 200),
// bids 99.00×1 then 89.10×1 (a sell of 2 receives 188.10 against ref 198).
func depthFixture(t testing.TB) mdomain.BookSnapshot {
	return depthBook(t,
		[][2]string{{"99.00", "1"}, {"89.10", "1"}},
		[][2]string{{"100.00", "1"}, {"101.00", "1"}},
	)
}

// Walking the asks across levels: (201-200)/200, and the quantity drains
// the side EXACTLY — a fill, not a depth-short.
func TestWalkedFillAsksCrossesLevelsExactDrain(t *testing.T) {
	slip, err := crossvenue.WalkedFill(depthFixture(t), mdomain.SideAsk, depthQty(t, "2"))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	wantRate(t, "ask slip", slip, "0.005")
}

// Walking the bids is the OTHER orientation and the OTHER numbers:
// (198-188.10)/198 = 0.05.
func TestWalkedFillBidsCrossesLevels(t *testing.T) {
	slip, err := crossvenue.WalkedFill(depthFixture(t), mdomain.SideBid, depthQty(t, "2"))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	wantRate(t, "bid slip", slip, "0.05")
}

// An order inside the best level pays exactly the reference: zero slip,
// not an error and not a skipped walk.
func TestWalkedFillWithinBestLevelIsZero(t *testing.T) {
	slip, err := crossvenue.WalkedFill(depthFixture(t), mdomain.SideAsk, depthQty(t, "0.5"))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	wantRate(t, "best-level slip", slip, "0")
}

// One unit past the drained side is short — the refusal, not a zero.
func TestWalkedFillDepthShort(t *testing.T) {
	_, err := crossvenue.WalkedFill(depthFixture(t), mdomain.SideAsk, depthQty(t, "2.5"))
	if !errors.Is(err, crossvenue.ErrDepthShort) {
		t.Fatalf("err %v, want ErrDepthShort", err)
	}
}

// An empty side has no best and no walk.
func TestWalkedFillEmptySideRefuses(t *testing.T) {
	book := depthBook(t, [][2]string{{"100.00", "1"}}, nil)
	_, err := crossvenue.WalkedFill(book, mdomain.SideAsk, depthQty(t, "1"))
	if !errors.Is(err, crossvenue.ErrDepthShort) {
		t.Fatalf("err %v, want ErrDepthShort", err)
	}
}

// A non-positive candidate walks nothing and reads as free — refused.
func TestWalkedFillCandidateNotPositive(t *testing.T) {
	zero, err := fixedpoint.ParseQuantity("ABC", "0")
	if err != nil {
		t.Fatalf("zero qty: %v", err)
	}
	if _, err := crossvenue.WalkedFill(depthFixture(t), mdomain.SideAsk, zero); !errors.Is(err, crossvenue.ErrCandidateInvalid) {
		t.Fatalf("zero: err %v, want ErrCandidateInvalid", err)
	}
}

// An unvalidatable book refuses before any arithmetic.
func TestWalkedFillInvalidBookRefuses(t *testing.T) {
	book := depthFixture(t)
	book.Watermark = mdomain.Watermark{} // the unusable zero value
	if _, err := crossvenue.WalkedFill(book, mdomain.SideAsk, depthQty(t, "1")); err == nil {
		t.Fatal("invalid book walked")
	}
}
