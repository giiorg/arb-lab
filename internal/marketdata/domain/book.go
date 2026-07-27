package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// BookSide names which half of the order book a level rests on.
//
// # It is `type X string`, and SeqRelation in the same context is not
//
// §3.6 rule 8's shape exists "for enums crossing a persistence or wire
// boundary, where a self-describing stored token is worth the width", and a
// side crosses one: it rides inside a BookDelta, and §3.5's capture lane
// persists the market-data stream so replay can re-apply it. Under an integer
// encoding, reordering the constants — or a second implementation numbering
// them the other way round — re-reads every stored bid as an ask. Nothing
// fails; the book inverts, the spread goes negative, and ErrBookCrossed fires
// on data that was captured correctly.
//
// SeqRelation, three files over, is the documented exception for the opposite
// reason: it is a verdict computed at a decision point and never stored, so what
// matters there is that the zero value be unusable.
type BookSide string

const (
	// SideBid is the buy half: prices descend from the best.
	SideBid BookSide = "bid"

	// SideAsk is the sell half: prices ascend from the best.
	SideAsk BookSide = "ask"
)

// Validate reports whether s is one of the declared values.
func (s BookSide) Validate() error {
	switch s {
	case SideBid, SideAsk:
		return nil
	case "":
		return ErrSideUnset
	}
	return fmt.Errorf("%q: %w", string(s), ErrSideUnknown)
}

func (s BookSide) String() string { return string(s) }

// BookLevel is one resting price level: a price and the quantity available at
// it.
//
// The zero value is unusable — both fields are fixedpoint types whose zero
// values carry no asset tag — which is the point. A level nobody populated
// would otherwise read as "free size at price zero", and on the bid side that
// is a resting order to buy at nothing, which sorts to the bottom and is
// invisible until something routes to it.
type BookLevel struct {
	// Price is the level's price, carrying both assets. It may be negative:
	// fixedpoint permits it for calendar spreads and settled contracts, and
	// screening that is a venue's business.
	Price fixedpoint.Price

	// Quantity is the size resting at Price, and in a SNAPSHOT it is strictly
	// positive. Zero is a removal — see LevelChange, where zero is legal and
	// means exactly that.
	Quantity fixedpoint.Quantity
}

// Validate reports whether the level could rest in a real book.
func (l BookLevel) Validate() error {
	if err := l.Price.Validate(); err != nil {
		return fmt.Errorf("book level price: %w", err)
	}
	if err := l.Quantity.Validate(); err != nil {
		return fmt.Errorf("book level quantity: %w", err)
	}
	sign, err := l.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("book level quantity: %w", err)
	}
	if sign <= 0 {
		return fmt.Errorf("book level %s at %s: %w", l.Quantity, l.Price, ErrLevelQuantityNotPositive)
	}
	return nil
}

// BookSnapshot is a reconstructed, immutable view of one instrument's order
// book.
//
// # This is what §3.4 permits on the market-data lane, and the raw feed is not
//
// §3.4: "A raw snapshot+delta order book is NEVER coalesced or drop-oldest'd on
// the wire — dropping a delta desyncs the book"; deltas get "lossless
// application with sequence-gap detection and automatic resnapshot in
// marketdata, and coalescing applies to the resulting reconstructed immutable
// snapshot". This type is that result. A BookDelta never reaches a lane.
//
// # Both sides are sorted best-first, and that is a representation choice
//
// Sorted slices rather than a price-keyed map. Two reasons, and the second is
// the one that would be expensive to discover later. A book is read by "the best
// N levels" far more often than by "the size at price X", so the ordering is the
// access pattern. And a map has no iteration order: publishing one would make
// the bytes on §3.5's capture lane differ run to run for an identical input
// sequence, which turns the parity check into a diff nobody can act on.
//
// # An empty book is VALID, and refusing one would break a pure function
//
// A halted or delisted instrument really does publish nothing on a side, but the
// decisive argument is Apply's: a delta removing the last level must produce a
// valid snapshot from valid inputs. If emptiness were a refusal, a total
// function over legal inputs would start returning errors, and the reconstructor
// would treat an ordinary book state as a desync and resnapshot forever.
// BestBid/BestAsk return an ok bool so a consumer cannot read a missing side as
// a price.
type BookSnapshot struct {
	// Instrument is the book's instrument, canonical per §5.1 — spot and perp
	// on one symbol are distinct books.
	Instrument ids.VenueInstrument

	// Bids rest below the market, ordered by STRICTLY DESCENDING price:
	// Bids[0] is the best bid.
	Bids []BookLevel

	// Asks rest above the market, ordered by STRICTLY ASCENDING price:
	// Asks[0] is the best ask.
	Asks []BookLevel

	// Watermark is the provenance of the observation this book was
	// reconstructed to. Its VenueSeq is the last sequence applied, which is what
	// the next delta's relation is computed against.
	Watermark Watermark
}

// Validate reports whether the book is one a venue could have published and a
// strategy may price against.
//
// # The check order is a PRECONDITION chain, not a readability preference
//
// Identity, provenance, assets, per-side ordering, then the invariant spanning
// both sides. The asset check sitting ahead of the ordering check is the part
// that is load-bearing rather than tidy: fixedpoint's Cmp REFUSES two prices
// whose tags disagree, so an ordering pass run first reports
// fixedpoint.ErrAssetMismatch from inside a sort check. One defect would then
// surface under two different sentinels depending on whether the mismatched
// levels happened to land on the same side of the book — and the one that
// reaches a caller says the prices could not be compared, which reads as a
// fixedpoint problem rather than as an adapter that filled a side from the wrong
// symbol's payload.
func (b BookSnapshot) Validate() error {
	if err := b.Instrument.Validate(); err != nil {
		return fmt.Errorf("book instrument: %w", err)
	}
	if err := b.Watermark.Validate(); err != nil {
		return fmt.Errorf("book watermark: %w", err)
	}
	// Checked HERE rather than in Watermark.Validate, which deliberately admits
	// a zero because a REST-polled funding read has no sequence. A book is the
	// case that exemption does not reach: the sequence is the position deltas are
	// placed against, so a zero one is not a stream without gap detection, it is
	// a book that can never advance. See ErrBookSeqUnset.
	if b.Watermark.VenueSeq == 0 {
		return fmt.Errorf("book %s: %w", b.Instrument, ErrBookSeqUnset)
	}
	if err := b.validateLevels(); err != nil {
		return err
	}
	if err := validateSide(SideBid, b.Bids); err != nil {
		return err
	}
	if err := validateSide(SideAsk, b.Asks); err != nil {
		return err
	}
	return b.validateUncrossed()
}

// BestBid is the highest resting bid, and whether there is one.
func (b BookSnapshot) BestBid() (BookLevel, bool) {
	if len(b.Bids) == 0 {
		return BookLevel{}, false
	}
	return b.Bids[0], true
}

// BestAsk is the lowest resting ask, and whether there is one.
func (b BookSnapshot) BestAsk() (BookLevel, bool) {
	if len(b.Asks) == 0 {
		return BookLevel{}, false
	}
	return b.Asks[0], true
}

// Side returns the levels on one half of the book.
//
// The switch names both members rather than defaulting, which is §3.6 rule 10
// working as intended: a third side is not a thing an order book has, but if one
// were ever declared, this returning nil for it would present as an empty book —
// a valid state — rather than as a build failure.
func (b BookSnapshot) Side(s BookSide) ([]BookLevel, error) {
	switch s {
	case SideBid:
		return b.Bids, nil
	case SideAsk:
		return b.Asks, nil
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("book side: %w", err)
	}
	// Unreachable while Validate covers exactly the two cases above, and
	// returned rather than panicked: a declared-but-unhandled side is a build
	// failure under exhaustiveswitch, so reaching here means the two lists have
	// been allowed to disagree, and that is a refusal rather than a crash in a
	// hot path.
	return nil, fmt.Errorf("book side %q: %w", string(s), ErrSideUnknown)
}

// validateSide checks that one half is strictly monotone in its direction.
//
// It assumes validateLevels has already run, which is why it may compare prices
// at all: Cmp refuses disagreeing tags, and a sort check is not the place a
// reader should learn that two levels priced different pairs.
func validateSide(side BookSide, levels []BookLevel) error {
	for i, l := range levels {
		if i == 0 {
			continue
		}
		cmp, err := levels[i-1].Price.Cmp(l.Price)
		if err != nil {
			return fmt.Errorf("%s level %d: %w", side, i, err)
		}
		if !strictlyOrders(side, cmp) {
			return fmt.Errorf(
				"%s level %d: %s does not follow %s: %w",
				side, i, l.Price, levels[i-1].Price, ErrBookUnordered)
		}
	}
	return nil
}

// strictlyOrders reports whether cmp — the comparison of a level against its
// PREDECESSOR on this side — is the strict direction that side requires.
//
// One function for both sides rather than two loops, because the bid and ask
// checks are the same check with one sign flipped, and a fix applied to a
// duplicated loop is a fix applied to one of them.
func strictlyOrders(side BookSide, cmp int) bool {
	switch side {
	case SideBid:
		return cmp > 0 // predecessor strictly higher: descending
	case SideAsk:
		return cmp < 0 // predecessor strictly lower: ascending
	}
	return false
}

// validateLevels walks every level in the book once, checking that each is
// individually usable and that they all price the same pair and size the same
// asset.
//
// # Both jobs in one pass, because the second depends on the first
//
// The asset comparison reads Price.Base()/Quote() directly, and on an
// unpopulated level those are the empty string — so an asset check running
// alone would report the FIRST populated level as disagreeing with a book whose
// pair it had established from a level nobody filled in. Validating each level
// as it is reached puts the unset field's own error first, which is the
// diagnosis that sends a reader to the adapter rather than to the symbol table.
//
// fixedpoint tags assets at runtime because venue assets are dynamic, so this is
// where the tag is actually interrogated for a book. The failure it catches is
// an adapter that filled one side's levels from a second symbol's payload: the
// prices are plausible, the sort succeeds, and the cross check compares a BTC
// bid against an ETH ask and reports a spread.
func (b BookSnapshot) validateLevels() error {
	var (
		base, quote fixedpoint.Asset
		size        fixedpoint.Asset
		seen        bool
	)
	// The two sides are walked with ONE `seen` flag between them, which is the
	// case worth stating: the book's assets are established by whichever level
	// comes first anywhere, so a book whose bids are all in one pair and whose
	// asks are all in another is caught. Resetting per side — the shape a
	// per-side helper would naturally take — would validate each half against
	// itself and find both internally consistent.
	for _, half := range []struct {
		side   BookSide
		levels []BookLevel
	}{{SideBid, b.Bids}, {SideAsk, b.Asks}} {
		for i, l := range half.levels {
			if err := l.Validate(); err != nil {
				return fmt.Errorf("%s level %d: %w", half.side, i, err)
			}
			if !seen {
				base, quote, size = l.Price.Base(), l.Price.Quote(), l.Quantity.Asset()
				seen = true
				continue
			}
			if l.Price.Base() != base || l.Price.Quote() != quote {
				return fmt.Errorf(
					"%s level %d prices %s/%s, book prices %s/%s: %w",
					half.side, i, l.Price.Quote(), l.Price.Base(), quote, base, ErrBookAssetMismatch)
			}
			if l.Quantity.Asset() != size {
				return fmt.Errorf(
					"%s level %d is sized in %s, book is sized in %s: %w",
					half.side, i, l.Quantity.Asset(), size, ErrBookAssetMismatch)
			}
		}
	}
	return nil
}

// validateUncrossed refuses a best bid at or above the best ask.
//
// AT is included, and it is not pedantry: a bid equal to the ask is a match the
// venue's own engine would have executed, so observing one means the two halves
// were reconstructed to different points in the sequence. It also yields a zero
// spread, which is the one number a cost-hurdle comparison treats as free
// execution.
//
// With either side empty there is nothing to cross, and that is not a special
// case bolted on — it is why the ok bools exist.
func (b BookSnapshot) validateUncrossed() error {
	bid, hasBid := b.BestBid()
	ask, hasAsk := b.BestAsk()
	if !hasBid || !hasAsk {
		return nil
	}
	cmp, err := bid.Price.Cmp(ask.Price)
	if err != nil {
		return fmt.Errorf("book cross check: %w", err)
	}
	if cmp >= 0 {
		return fmt.Errorf("best bid %s, best ask %s: %w", bid.Price, ask.Price, ErrBookCrossed)
	}
	return nil
}
