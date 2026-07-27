package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func price(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func level(t *testing.T, p, q string) domain.BookLevel {
	t.Helper()
	return domain.BookLevel{Price: price(t, p), Quantity: qty(t, q)}
}

// bookWm is a watermark whose VenueSeq is stated, because a book's is never
// incidental: BookDelta.Validate requires it to equal the delta's LastSeq, and
// a fixture leaving it at the shared wm() helper's constant 1 would make every
// sequence assertion below compare 1 against 1.
func bookWm(seq uint64) domain.Watermark {
	return domain.Watermark{VenueSeq: seq, SourceTime: base, RecvTime: base}
}

// book is the fixture, and every dimension of it is chosen against a distinction
// the code makes:
//
//   - FIVE levels a side, not two. With two, "compare against the predecessor"
//     and "compare against the first level" are the same check, and a binary
//     search cannot be told from a linear scan. Five also gives locate a real
//     midpoint to land on.
//   - Both sides populated, because strictlyOrders is one function with a sign
//     flip and a book with only bids scores a pass on the ask branch.
//   - A one-unit spread (100 / 101), so a mutant that widens or narrows by a tick
//     changes the crossed verdict rather than staying comfortably inside it.
//   - Distinct quantities per level, so an apply that writes the right price with
//     the wrong size is visible.
func book(t *testing.T) domain.BookSnapshot {
	t.Helper()
	return domain.BookSnapshot{
		Instrument: perp(),
		Bids: []domain.BookLevel{
			level(t, "100", "1"), level(t, "99", "2"), level(t, "98", "3"),
			level(t, "97", "4"), level(t, "96", "5"),
		},
		Asks: []domain.BookLevel{
			level(t, "101", "6"), level(t, "102", "7"), level(t, "103", "8"),
			level(t, "104", "9"), level(t, "105", "10"),
		},
		Watermark: bookWm(1000),
	}
}

func TestBookSideValidate(t *testing.T) {
	t.Parallel()

	for _, s := range []domain.BookSide{domain.SideBid, domain.SideAsk} {
		if err := s.Validate(); err != nil {
			t.Errorf("%q.Validate() = %v, want nil", s, err)
		}
	}

	// Empty and misspelt are SEPARATE sentinels, and asserting both is what
	// keeps them separate: an implementation collapsing them into one error
	// passes a test that only checks "some error".
	if err := domain.BookSide("").Validate(); !errors.Is(err, domain.ErrSideUnset) {
		t.Errorf(`BookSide("").Validate() = %v, want ErrSideUnset`, err)
	}
	if err := domain.BookSide("buy").Validate(); !errors.Is(err, domain.ErrSideUnknown) {
		t.Errorf(`BookSide("buy").Validate() = %v, want ErrSideUnknown`, err)
	}
}

func TestBookLevelValidate(t *testing.T) {
	t.Parallel()

	if err := level(t, "100", "1").Validate(); err != nil {
		t.Fatalf("the fixture level must be valid: %v", err)
	}

	cases := []struct {
		name string
		lvl  domain.BookLevel
		want error
	}{
		{"zero quantity", domain.BookLevel{Price: price(t, "100"), Quantity: qty(t, "0")}, domain.ErrLevelQuantityNotPositive},
		{"negative quantity", domain.BookLevel{Price: price(t, "100"), Quantity: qty(t, "-1")}, domain.ErrLevelQuantityNotPositive},
		{"unset price", domain.BookLevel{Quantity: qty(t, "1")}, fixedpoint.ErrAssetUnset},
		{"unset quantity", domain.BookLevel{Price: price(t, "100")}, fixedpoint.ErrAssetUnset},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := c.lvl.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestBookSnapshotValidate(t *testing.T) {
	t.Parallel()

	// Positive control, first: a Validate that refused everything would score
	// perfectly on the table below.
	if err := book(t).Validate(); err != nil {
		t.Fatalf("the fixture book must be valid: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*testing.T, *domain.BookSnapshot)
		want error
	}{
		{
			"instrument unset",
			func(_ *testing.T, b *domain.BookSnapshot) { b.Instrument = ids.VenueInstrument{} },
			ids.ErrVenueUnset,
		},
		{
			"watermark unset",
			func(_ *testing.T, b *domain.BookSnapshot) { b.Watermark = domain.Watermark{} },
			domain.ErrWatermarkUnset,
		},
		{
			// The violation is at index 2, NOT index 1. At index 1 a check
			// comparing against levels[i-1] and one comparing against levels[0]
			// give the same answer, so the fixture would not distinguish them.
			"bids ascend at the third level",
			func(t *testing.T, b *domain.BookSnapshot) { b.Bids[2] = level(t, "99.5", "3") },
			domain.ErrBookUnordered,
		},
		{
			// Equal prices, again at index 2: this is what separates a strict
			// comparison from a non-strict one. Two resting levels at one price
			// is not a book state, it is an insert that should have replaced.
			"bids repeat a price",
			func(t *testing.T, b *domain.BookSnapshot) { b.Bids[2] = level(t, "99", "3") },
			domain.ErrBookUnordered,
		},
		{
			// Whole side reversed: this is the case that catches a sign flip in
			// strictlyOrders' BID arm specifically. The per-level cases above
			// would survive a flip that broke only the ask arm.
			"bids sorted the ask way",
			func(t *testing.T, b *domain.BookSnapshot) {
				b.Bids = []domain.BookLevel{level(t, "96", "1"), level(t, "97", "2"), level(t, "98", "3")}
			},
			domain.ErrBookUnordered,
		},
		{
			"asks descend at the third level",
			func(t *testing.T, b *domain.BookSnapshot) { b.Asks[2] = level(t, "101.5", "8") },
			domain.ErrBookUnordered,
		},
		{
			"asks repeat a price",
			func(t *testing.T, b *domain.BookSnapshot) { b.Asks[2] = level(t, "102", "8") },
			domain.ErrBookUnordered,
		},
		{
			// The mirror of "bids sorted the ask way", and it is here because
			// this repo's rule is that a fix applied in one place gets applied
			// to its mirror. One of the two would leave half of strictlyOrders
			// unchecked.
			"asks sorted the bid way",
			func(t *testing.T, b *domain.BookSnapshot) {
				b.Asks = []domain.BookLevel{level(t, "105", "6"), level(t, "104", "7"), level(t, "103", "8")}
			},
			domain.ErrBookUnordered,
		},
		{
			"a bid rests zero size",
			func(t *testing.T, b *domain.BookSnapshot) {
				b.Bids[1] = domain.BookLevel{Price: price(t, "99"), Quantity: qty(t, "0")}
			},
			domain.ErrLevelQuantityNotPositive,
		},
		{
			// Pins the precondition chain in Validate's doc comment. The level
			// sits at index 1, so the book's pair has already been established
			// from index 0 — an asset check running without per-level validation
			// would compare "" against BTC/USDT and report a MISMATCH, sending a
			// reader to the symbol table for a field nobody populated.
			"a bid has no price at all",
			func(t *testing.T, b *domain.BookSnapshot) {
				b.Bids[1] = domain.BookLevel{Quantity: qty(t, "2")}
			},
			fixedpoint.ErrAssetUnset,
		},
		{
			"a bid prices a different pair",
			func(t *testing.T, b *domain.BookSnapshot) {
				p, err := fixedpoint.ParsePrice("ETH", "USDT", "99")
				if err != nil {
					t.Fatalf("ParsePrice: %v", err)
				}
				b.Bids[1] = domain.BookLevel{Price: p, Quantity: qty(t, "2")}
			},
			domain.ErrBookAssetMismatch,
		},
		{
			// The case a per-side asset check would miss: each half is
			// internally consistent and the halves disagree with each other.
			// This is why validateAssets carries ONE `seen` flag across both.
			"the two sides price different pairs",
			func(t *testing.T, b *domain.BookSnapshot) {
				var asks []domain.BookLevel
				for _, s := range []string{"101", "102", "103"} {
					p, err := fixedpoint.ParsePrice("ETH", "USDT", s)
					if err != nil {
						t.Fatalf("ParsePrice: %v", err)
					}
					asks = append(asks, domain.BookLevel{Price: p, Quantity: qty(t, "1")})
				}
				b.Asks = asks
			},
			domain.ErrBookAssetMismatch,
		},
		{
			"an ask is sized in a different asset",
			func(t *testing.T, b *domain.BookSnapshot) {
				q, err := fixedpoint.ParseQuantity("ETH", "6")
				if err != nil {
					t.Fatalf("ParseQuantity: %v", err)
				}
				b.Asks[1] = domain.BookLevel{Price: price(t, "102"), Quantity: q}
			},
			domain.ErrBookAssetMismatch,
		},
		{
			"best bid above best ask",
			func(t *testing.T, b *domain.BookSnapshot) { b.Asks[0] = level(t, "99.5", "6") },
			domain.ErrBookCrossed,
		},
		{
			// The boundary: bid EQUAL to ask. This is what separates `cmp >= 0`
			// from `cmp > 0`, and a table containing only the strictly-crossed
			// case above scores the same on both.
			"best bid equal to best ask",
			func(t *testing.T, b *domain.BookSnapshot) {
				b.Asks = []domain.BookLevel{level(t, "100", "6"), level(t, "102", "7")}
			},
			domain.ErrBookCrossed,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := book(t)
			c.mut(t, &b)
			err := b.Validate()
			if !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestBookSnapshotEmptySidesAreValid pins the decision that an empty side is a
// book state rather than a refusal.
//
// It is not a completeness nicety. Apply must be able to remove the last level
// on a side and return a valid result; if emptiness were refused, a total
// function over legal inputs would start erroring, and the reconstructor would
// read an ordinary halted instrument as a desync and resnapshot forever.
//
// All three shapes are exercised deliberately. validateUncrossed short-circuits
// on `!hasBid || !hasAsk`, and a test with only the both-empty case leaves one
// of those two arms unexecuted — the classic pinned fixture, where the parameter
// that collapses the distinction is which side was emptied.
func TestBookSnapshotEmptySidesAreValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		mut              func(*domain.BookSnapshot)
		wantBid, wantAsk bool
	}{
		{"both sides empty", func(b *domain.BookSnapshot) { b.Bids, b.Asks = nil, nil }, false, false},
		{"bids only", func(b *domain.BookSnapshot) { b.Asks = nil }, true, false},
		{"asks only", func(b *domain.BookSnapshot) { b.Bids = nil }, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := book(t)
			c.mut(&b)
			if err := b.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if _, ok := b.BestBid(); ok != c.wantBid {
				t.Errorf("BestBid ok = %v, want %v", ok, c.wantBid)
			}
			if _, ok := b.BestAsk(); ok != c.wantAsk {
				t.Errorf("BestAsk ok = %v, want %v", ok, c.wantAsk)
			}
		})
	}
}

func TestBookSnapshotBest(t *testing.T) {
	t.Parallel()

	b := book(t)

	// Asserted against the price, not against the index. A mutant returning
	// Bids[len-1] would satisfy "returns a level" and "returns something from
	// the bid side"; only the value separates best from worst.
	bid, ok := b.BestBid()
	if !ok {
		t.Fatal("BestBid ok = false, want true")
	}
	if got := bid.Price.Decimal(); got != "100" {
		t.Errorf("BestBid price = %s, want 100 — the best bid is the HIGHEST", got)
	}
	ask, ok := b.BestAsk()
	if !ok {
		t.Fatal("BestAsk ok = false, want true")
	}
	if got := ask.Price.Decimal(); got != "101" {
		t.Errorf("BestAsk price = %s, want 101 — the best ask is the LOWEST", got)
	}
}

func TestBookSnapshotSide(t *testing.T) {
	t.Parallel()

	b := book(t)

	bids, err := b.Side(domain.SideBid)
	if err != nil {
		t.Fatalf("Side(bid) = %v, want nil", err)
	}
	// Compared by content rather than by length: two sides of five levels each
	// means a mutant returning Asks for SideBid passes any length check.
	if got := bids[0].Price.Decimal(); got != "100" {
		t.Errorf("Side(bid)[0] = %s, want 100 — that is the ASK side's best", got)
	}
	asks, err := b.Side(domain.SideAsk)
	if err != nil {
		t.Fatalf("Side(ask) = %v, want nil", err)
	}
	if got := asks[0].Price.Decimal(); got != "101" {
		t.Errorf("Side(ask)[0] = %s, want 101", got)
	}

	if _, err := b.Side(""); !errors.Is(err, domain.ErrSideUnset) {
		t.Errorf(`Side("") = %v, want ErrSideUnset`, err)
	}
	if _, err := b.Side("buy"); !errors.Is(err, domain.ErrSideUnknown) {
		t.Errorf(`Side("buy") = %v, want ErrSideUnknown`, err)
	}
}
