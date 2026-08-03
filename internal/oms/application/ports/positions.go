package ports

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// VenuePositions is §7.5's `Positions` surface: the venue's own account of what
// it holds, which is the second authority the net-delta invariant is asserted
// against.
//
// # It is a third read port rather than a method on OrderResolver, and the split
// # is the same safety property that split OrderResolver from OrderPlacer
//
// §7.5's join has two halves that fail differently. Resolving an ORDER is §5.3's
// stable-not-found rule, whose whole discipline is that an unanswered query is
// never read as an absence; enumerating POSITIONS is an assertion about the
// account, and its failure mode is the opposite one — an answer that arrives
// complete and describes a scope nobody meant to ask about. Keeping them apart
// means the use case that asserts the invariant cannot reach for a per-order
// query to explain a delta away, which is the move that turns an unexplained
// position into an explained one by looking harder at our own orders.
type VenuePositions interface {
	// Positions enumerates the account's open positions for one scope.
	//
	// An implementation MUST NOT narrow the enumeration to instruments the caller
	// named. §7.5 alerts on "foreign orders or unexplained position deltas", and a
	// symbol-scoped read can only ask about instruments this system already knows
	// it traded — the set that structurally cannot contain a foreign position.
	//
	// An error means the enumeration was not answered. It is never an empty
	// report: §5.3's rule about not inferring an absence from an unanswered query
	// is stated for orders and is the same rule here, one surface over, and the
	// consequence of breaking it is larger — an empty report is read as a flat
	// account, which is what the invariant passes on.
	Positions(PositionScope) (PositionReport, error)
}

// PositionScope is which slice of an account's positions to enumerate.
//
// Venue and Product are here rather than derived from an instrument because the
// enumeration is deliberately not per-instrument. Settle is here because one
// venue scopes on it and the other does not: Bybit's linear enumeration is
// illegal without a settleCoin when no symbol is sent, while Binance's covers
// the whole family — so the guarantee a caller gets is the narrower one, and an
// asset this scope did not name is not in the answer and is not evidence about
// anything.
type PositionScope struct {
	Venue   ids.Venue
	Product ids.ProductCategory
	Settle  fixedpoint.Asset
}

// Validate reports whether the scope names an enumeration a venue could serve.
func (s PositionScope) Validate() error {
	if err := s.Venue.Validate(); err != nil {
		return fmt.Errorf("position scope venue: %w", err)
	}
	if err := s.Product.Validate(); err != nil {
		return fmt.Errorf("position scope product: %w", err)
	}
	if !s.Product.IsPerp() {
		// The same normative fact engine/venue's own PositionScope refuses on, and
		// it was stated only there: no venue publishes a position surface for spot
		// — a spot balance is not a position. Left out here, a spot scope was
		// constructible, survived NewJoiner, and was refused two layers down by the
		// venue codec — which turns a configuration mistake into a FAILED
		// net-delta gate on every boot, halting a healthy process for a line in a
		// file. A rule enforced on one of two mirrors leaves the other holding
		// exactly the shape the rule exists to eliminate.
		return fmt.Errorf("position scope %s/%s: %w", s.Venue, s.Product, ErrPositionScopeSurfaceAbsent)
	}
	if s.Settle == "" {
		return fmt.Errorf("position scope %s/%s: %w", s.Venue, s.Product, ErrPositionScopeSettleUnset)
	}
	return nil
}

func (s PositionScope) String() string { return fmt.Sprintf("%s/%s/%s", s.Venue, s.Product, s.Settle) }

// VenuePosition is one instrument's net exposure at the venue.
type VenuePosition struct {
	// Instrument is which market.
	Instrument ids.VenueInstrument

	// Net is the SIGNED position size in the instrument's base asset: positive
	// long, negative short, zero for a row the venue returned while holding
	// nothing.
	//
	// Signed rather than a side beside an unsigned size, which inverts the rule
	// domain.Fill follows for the reason engine/venue.Position states: a fill is
	// an event whose direction belongs to its order, and this is a NET, which is
	// the result of arithmetic over both directions and has no side to carry.
	Net fixedpoint.Quantity
}

// PositionReport is one enumeration's whole answer.
type PositionReport struct {
	// Positions are the rows on instruments the adapter could type. A declared
	// instrument ABSENT from this slice is flat, and that reading is available
	// only because the enumeration is not scoped by symbol — see VenuePositions.
	Positions []VenuePosition

	// Unrecognized are the symbols the venue reported that this deployment does
	// not trade, in the venue's own spelling.
	//
	// They carry no quantity, and the absence is deliberate: the money kernel
	// cannot type a quantity for a symbol whose base asset nobody configured, and
	// §7.5 does not auto-correct a foreign position anyway — it alerts for
	// operator clearance, and what an alert needs is the name.
	Unrecognized []string
}
