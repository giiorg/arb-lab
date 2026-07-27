package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// BookSource is the §3.5 source a book payload arrives on.
//
// # Written because FundingSource exists, which is this repo's mirror rule
//
// The argument for a constructor rather than a struct literal at each producer
// is FundingSource's and is not restated here: routing decided at each call site
// is routing that drifts. What IS worth stating is the difference, because it is
// the one a reader will assume away.
//
// # Book is the stream that is NOT perp-only, and that is the whole point of it
//
// §3.5's product table makes StreamFunding and StreamMark perpetual-only and
// StreamBook available to both families, and events.StreamKind.RequiresPerp is
// where that binds. So this constructor accepts a spot instrument where
// FundingSource refuses one — and it must, because the spot book is what §4.2
// says the spot leg is priced from. A copy of FundingSource with the stream
// token swapped would still be correct here, which is exactly why the asymmetry
// deserves a sentence: the next stream constructor written by analogy to this
// one inherits "both families" from a function that got it by accident.
//
// The refusal that remains is the ordinary one — an instrument that does not
// validate — and it is events.SourceID.Validate's, not renamed. A caller
// branching on events.ErrStreamProductMismatch keeps branching on the rule's
// owner.
func BookSource(inst ids.VenueInstrument) (events.SourceID, error) {
	src := events.SourceID{Instrument: inst, Stream: events.StreamBook}
	if err := src.Validate(); err != nil {
		return events.SourceID{}, fmt.Errorf("venue book source: %w", err)
	}
	return src, nil
}

// BookSide names which half of the book a change rests on, in the venue's own
// vocabulary.
//
// # It duplicates domain.BookSide, and the duplication is the boundary
//
// §3.6 rule 1 forbids marketdata/domain from importing the engine, so the domain
// cannot BE this type — the same argument driving/'s doc records for
// venue.Funding against domain.FundingObservation. What it buys here is more
// than a layering nicety: the translation between the two is a switch over a
// module-declared enum-like, so §3.6 rule 10 makes a side added on one side of
// the seam and not the other a BUILD failure. A shared type would make it a
// silent pass-through of a value the domain has no case for.
//
// `type X string` for domain.BookSide's stated reason, which applies here first:
// a side rides inside a delta and §3.5's capture lane persists the market-data
// stream, so under an integer encoding a renumbering re-reads every stored bid
// as an ask. Nothing fails; the book inverts.
type BookSide string

const (
	// SideBid is the buy half.
	SideBid BookSide = "bid"

	// SideAsk is the sell half.
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

// BookLevel is one resting price level as the venue published it.
//
// Both fields are dimensional, which is §4's units rule doing the work it exists
// for: a venue sends "43250.10" and "0.5" as strings, and the parse that turns
// them into a Price and a Quantity is the last point at which the two are
// distinguishable by anything but position.
type BookLevel struct {
	// Price is the level's price, carrying both assets.
	Price fixedpoint.Price

	// Quantity is the size resting at Price. Strictly positive in a SNAPSHOT —
	// a snapshot states what IS there, and zero size is not a resting order.
	// LevelChange is where zero is legal and means removal.
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

// BookSnapshot is one venue's full order book — §4's StreamBook snapshot
// payload.
//
// # It carries no instrument, no watermark and no sequence
//
// §4's rule, unchanged from Funding: the Envelope is authoritative for
// provenance. SourceID is the mailbox key, so a payload naming an instrument is
// a second copy of one fact with nothing forbidding the two to disagree; VenueSeq
// and both timestamps are envelope fields §3.5 defines for every stream. The
// position this book was taken at is env.VenueSeq, and that is the value the
// next delta's relation is computed against — carrying it twice is how the two
// come to disagree about where reconstruction actually is.
//
// # Both sides arrive best-first, and this type does not CHECK that
//
// The ordering is part of the contract — Bids descending, Asks ascending, which
// is what domain.BookSnapshot requires and what the translation copies
// unchanged. It is not re-asserted here, and the omission is deliberate rather
// than an oversight.
//
// Restating it would mean a second copy of strictlyOrders and of the cross
// check, in a package that cannot import the one that owns them, and two copies
// of a comparison rule are two things to keep in step. The domain's copy runs on
// every snapshot before any of it is used — BookReconstruct.OnSnapshot validates
// first — so nothing reaches a strategy unchecked either way. What differs is
// only which boundary the operator is sent to, and here that argument runs the
// OTHER way from Funding's: this type has no instrument, so a crossed-book
// refusal raised from it could not name the book it is about, while the domain's
// can.
//
// A venue that publishes its depth in some other order is handled where that is
// a known venue fact — in that venue's adapter, at decode (§20a), beside the
// Profiles entry recording it. Not by a sort here, which would silently absorb a
// venue changing its format.
type BookSnapshot struct {
	// Bids rest below the market, ordered by strictly DESCENDING price.
	Bids []BookLevel

	// Asks rest above the market, ordered by strictly ASCENDING price.
	Asks []BookLevel
}

// Validate reports whether every level is one that could rest in a real book.
//
// An EMPTY book passes, on domain.BookSnapshot's argument: a halted instrument
// really does publish nothing on a side, and refusing emptiness would make a
// delta that removes the last level unable to produce a valid result.
func (b BookSnapshot) Validate() error {
	for _, half := range []struct {
		side   BookSide
		levels []BookLevel
	}{{SideBid, b.Bids}, {SideAsk, b.Asks}} {
		for i, l := range half.levels {
			if err := l.Validate(); err != nil {
				return fmt.Errorf("%s level %d: %w", half.side, i, err)
			}
		}
	}
	return nil
}

// LevelChange is one instruction from a venue's incremental depth feed.
//
// Zero quantity means REMOVE — see domain.LevelChange for why that makes it a
// different type from a level rather than the same one with a looser check.
type LevelChange struct {
	// Side is which half to change.
	Side BookSide

	// Price identifies the level. An identity, not a measurement: the venue's
	// feed keys on it.
	Price fixedpoint.Price

	// Quantity is the new resting size, where ZERO means remove. Negative is
	// refused: no venue publishes it, and the reading that would admit it —
	// arithmetic against the existing size — is a different feed protocol.
	Quantity fixedpoint.Quantity
}

// Validate reports whether the change is one a venue could have sent.
func (c LevelChange) Validate() error {
	if err := c.Side.Validate(); err != nil {
		return fmt.Errorf("level change side: %w", err)
	}
	if err := c.Price.Validate(); err != nil {
		return fmt.Errorf("level change price: %w", err)
	}
	if err := c.Quantity.Validate(); err != nil {
		return fmt.Errorf("level change quantity: %w", err)
	}
	sign, err := c.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("level change quantity: %w", err)
	}
	if sign < 0 {
		return fmt.Errorf("%s at %s: %w", c.Quantity, c.Price, ErrChangeQuantityNegative)
	}
	return nil
}

// BookDelta is one incremental depth update — §4's StreamBook delta payload.
//
// # It carries FirstSeq and NOT LastSeq, which is the whole design of this type
//
// domain.BookDelta needs a sequence SPAN, because Binance's depth events cover
// several venue sequences in one message while Bybit's cover one. An envelope
// carries a single VenueSeq. So one end of the span has to come from the payload
// — and the question is which, not whether.
//
// LastSeq is the end already on the envelope: domain.BookDelta.Validate REQUIRES
// Watermark.VenueSeq == LastSeq, so a payload restating it is §4's forbidden
// second copy of one fact, in the one place the repo has already paid to learn
// what that costs. FirstSeq is genuinely absent from the envelope — it is not
// provenance, it is the message's own extent — so it travels here.
//
// The disagreement that remains is a FirstSeq past the envelope's VenueSeq, and
// it needs no sentinel of its own: it lands on domain's ErrDeltaSeqInverted,
// which is exactly what an inverted span IS. Had the payload carried both, that
// same defect would have needed a third check nobody would have thought to write
// — the payload's own span is consistent, and it is the envelope it contradicts.
//
// A single-sequence venue sets FirstSeq to the same value the envelope carries.
// That is not a special case: it is the span of length one, and it is why
// SeqRelationOverlap is unreachable for those venues (domain.BookDelta.Follows
// says so).
type BookDelta struct {
	// Changes are the level instructions, and MAY be empty. An empty delta is a
	// heartbeat, which real venues send and which carries real information: it
	// advances the sequence, so refusing it manufactures a gap out of a healthy
	// feed.
	Changes []LevelChange

	// FirstSeq is the first venue sequence this message covers, inclusive. The
	// last is the envelope's VenueSeq.
	FirstSeq uint64
}

// Validate reports whether the delta is one a venue could have sent.
//
// It cannot check the span — it holds one end of it — and that is the cost of
// the split above, paid knowingly. What it CAN check is the end it holds: a zero
// FirstSeq is what a decoder that dropped the field produces, and refused here
// it names the venue message rather than the reconstruction that could not place
// it.
func (d BookDelta) Validate() error {
	if d.FirstSeq == 0 {
		return ErrDeltaFirstSeqUnset
	}
	for i, c := range d.Changes {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("change %d: %w", i, err)
		}
	}
	return nil
}
