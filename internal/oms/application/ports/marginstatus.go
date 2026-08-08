package ports

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// VenueMarginStatus is §4's `AccountSource.MarginStatus` as this context reads
// it: the ACCOUNT-side half of §7.1's liq-distance trigger, beside
// MarkRiskReader's market-side half.
//
// # Two ports for one trigger, and the split is not a layering preference
//
// §7.1's distance is taken between the venue's mark and the venue's published
// liquidation price. The two numbers arrive by completely different routes — the
// mark on a continuous public stream through marketdata and engine/state, the
// liq price on a signed per-instrument REST read — and they fail differently:
// the mark goes STALE while still arriving, and this one is either answered or
// not answered at all. MarkRiskReader consequently returns ages and no verdict,
// and this one returns no age at all, because a REST answer is as fresh as the
// call that fetched it and a caller holding one for later is holding a fact it
// has no way to re-date.
//
// # A third read port beside VenuePositions, over the same venue rows
//
// On Binance both read `/fapi/v3/positionRisk`; on Bybit both read
// `/v5/position/list`. They stay apart because the questions do (record §78 §5):
// §7.5 enumerates with no symbol to find exposure nothing here can account for,
// and this one asks about one instrument this system knows it holds. Folding them
// would give §7.5's net-delta invariant a second source for the position, which
// is the thing engine/venue's position codec is narrowed to prevent.
type VenueMarginStatus interface {
	// MarginStatus reads one perpetual position's margin state at the venue.
	//
	// The bool reports whether the venue holds a position on this instrument.
	// It is not an error: a flat instrument is the ordinary answer for anything
	// this system has closed, and §7.1 has nothing to compute for one. It is
	// also not the same answer as an error, and a caller must not collapse them
	// — flat means "this position needs no monitoring", while an unanswered read
	// means "this position's liq distance is unknown", which is the stale-input
	// rung of §7.1's ladder and blocks openings.
	//
	// An error therefore MUST NOT be re-derived into a flat instrument. That is
	// §5.3's rule about not inferring an absence from an unanswered query, and
	// the consequence of breaking it here is that the trigger silently stops
	// watching a live short.
	MarginStatus(ids.VenueInstrument) (MarginReading, bool, error)
}

// MarginReading is one position's margin state as this context reads it.
//
// It mirrors engine/venue.MarginStatus field for field, and unlike MarkReading
// that is not a narrowing choice — §3.6 rule 1 forbids this package from
// importing engine/, so an ACL type is the only shape available and the adapter
// translates. Every field here has a §7.1 consumer, so there is nothing to drop.
type MarginReading struct {
	// Instrument is the perpetual this describes.
	Instrument ids.VenueInstrument

	// LiqPrice is the venue's own published liquidation price, in the
	// instrument's quote asset. Meaningful only when LiqPriceStated.
	LiqPrice fixedpoint.Price

	// LiqPriceStated is whether the venue published one at all.
	//
	// Both venues have a state in which they do not: Bybit documents an empty
	// string "when liqPrice <= minPrice or liqPrice >= maxPrice", and neither
	// venue's degenerate spellings are safe to read as a number. A false here is
	// NEVER "far from liquidation" — no distance is computable, which is the
	// same rung of §7.1's ladder as a stale mark.
	LiqPriceStated bool

	// Maintenance is the maintenance margin the venue requires to keep the
	// position open.
	Maintenance fixedpoint.Money

	// PostedMargin is the isolated margin posted against the position.
	//
	// It EXCLUDES unrealized PnL on both venues, so it is not a margin balance
	// and `PostedMargin - Maintenance` is not headroom: the difference is
	// exactly the adverse PnL, which is largest when this trigger fires. The
	// headroom statement is LiqPrice against the mark. See
	// engine/venue.MarginStatus for why §4's "available margin" is a quantity
	// neither venue publishes per position.
	PostedMargin fixedpoint.Money
}

// Validate reports whether the reading is one §7.1 may compute from.
//
// The same checks engine/venue.MarginStatus makes, restated rather than trusted,
// because this is a PORT: a test double satisfies it directly, and a fixture
// built without thinking about the asset agreement or the stated flag would let
// a use-case test pass against a reading no real venue could produce.
func (m MarginReading) Validate() error {
	if err := m.Instrument.Validate(); err != nil {
		return fmt.Errorf("margin reading instrument: %w", err)
	}
	if !m.Instrument.Product.IsPerp() {
		// Neither venue publishes a position surface for spot, so a spot reading
		// is one no adapter could have produced. VenuePositions' PositionScope
		// makes the same refusal for the same reason: a rule enforced on one of
		// two mirrors leaves the other holding the shape it exists to eliminate.
		return fmt.Errorf("margin reading %s: %w", m.Instrument.Symbol, ErrMarginReadingSurfaceAbsent)
	}
	if err := m.Maintenance.Validate(); err != nil {
		return fmt.Errorf("margin reading %s maintenance: %w", m.Instrument.Symbol, err)
	}
	if err := m.PostedMargin.Validate(); err != nil {
		return fmt.Errorf("margin reading %s posted margin: %w", m.Instrument.Symbol, err)
	}
	if m.Maintenance.Asset() != m.PostedMargin.Asset() {
		return fmt.Errorf("margin reading %s: maintenance in %s, posted margin in %s: %w",
			m.Instrument.Symbol, m.Maintenance.Asset(), m.PostedMargin.Asset(), ErrMarginReadingAssetsDisagree)
	}

	if !m.LiqPriceStated {
		if m.LiqPrice != (fixedpoint.Price{}) {
			return fmt.Errorf("margin reading %s states no liq price but carries %s: %w",
				m.Instrument.Symbol, m.LiqPrice, ErrMarginReadingLiqPriceContradicts)
		}
		return nil
	}
	if err := m.LiqPrice.Validate(); err != nil {
		return fmt.Errorf("margin reading %s liq price: %w", m.Instrument.Symbol, err)
	}
	sign, err := m.LiqPrice.Sign()
	if err != nil {
		return fmt.Errorf("margin reading %s liq price: %w", m.Instrument.Symbol, err)
	}
	if sign <= 0 {
		return fmt.Errorf("margin reading %s liq price %s: %w",
			m.Instrument.Symbol, m.LiqPrice, ErrMarginReadingLiqPriceNotPositive)
	}
	if m.LiqPrice.Quote() != m.Maintenance.Asset() {
		return fmt.Errorf("margin reading %s: liq price quoted in %s, margin in %s: %w",
			m.Instrument.Symbol, m.LiqPrice.Quote(), m.Maintenance.Asset(), ErrMarginReadingAssetsDisagree)
	}
	return nil
}
