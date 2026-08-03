package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// HedgeGroup is §5.1's hedge-group key: the unit the §6.7 classifier computes a
// residual over, and the unit the OMS holds an exclusive lease per.
//
// # It has THREE components, and the code used to carry two
//
// §5.1 defines it once and nowhere else: "an exclusive lease per hedge-group
// (account, venue, symbol) — deliberately grouping a trade's spot + perp legs
// (Account = the venue subaccount — the lease MUST span both legs, which on
// Binance are distinct spot/futures API domains under one subaccount and on
// Bybit are one UTA, §4.1), and distinct from per-instrument identity (§5.3)."
//
// Both exclusions in that sentence are load-bearing, and each rules out the
// implementation somebody reaches for first:
//
//   - NOT the trade. ClientOrderID embeds a TradeID, so grouping by trade is one
//     field away and it is wrong: §5.1's key has no trade in it, and the lease
//     exists precisely so a SUCCESSOR trade on the same key is refused. Netting
//     per trade classifies two trades on one symbol independently while they
//     share one venue position.
//   - NOT the instrument. Spot BTCUSDT and perp BTCUSDT are different
//     VenueInstruments with independent order and execution-ID namespaces, and
//     the entire point of the classifier is that their exposures net.
//
// # Account is here because leaving it out produces a WRONG NUMBER, not a halt
//
// This is the unusual direction for a defect in this kernel. Most of the
// classifier's failure modes end with the machine refusing to act; this one ends
// with it concluding. Keyed on (venue, symbol) alone, a naked long in subaccount
// A and a naked short in subaccount B net to a residual of ZERO — positions and
// margin are per subaccount, so nothing about that netting is real. The group
// reads flat, ExposureReduction is never sized, both subaccounts stay naked, and
// every invariant computed over the group agrees.
//
// domain.Order carries a VenueInstrument and no account; domain.Fill carries one
// because §5.1's dedupe key puts it first. So this type is assembled from an
// order record plus the account that holds it, and never inferred from the order
// alone.
//
// # The zero value is unusable
//
// Every component is required. An empty account is the field a hydration missed,
// and it is the one that silently merges every subaccount into one group — the
// failure above, arriving from a blank string rather than from a design choice.
type HedgeGroup struct {
	// Account is the venue subaccount, §5.1's first dedupe-key component. It is
	// a string for the same reason domain.Fill's is: the venues have no shared
	// notion of a subaccount identifier, and an opaque token compared for
	// equality is exactly what the key needs.
	Account string

	// Venue is the exchange. It is NOT derived from an instrument, because a
	// group is assembled before its orders are known to belong to it — that
	// check is the classifier's, and it needs something to check against.
	Venue ids.Venue

	// Symbol is the venue's own routing symbol, "BTCUSDT". It spans the product
	// categories deliberately: the spot leg and the perp leg of one
	// cash-and-carry trade share it, and that sharing IS the group.
	Symbol string
}

// Validate reports whether the key is fully populated.
func (g HedgeGroup) Validate() error {
	if g.Account == "" {
		return ErrHedgeGroupAccountUnset
	}
	if err := g.Venue.Validate(); err != nil {
		return fmt.Errorf("hedge group %q venue: %w", g.Account, err)
	}
	if g.Symbol == "" {
		return fmt.Errorf("hedge group %q on %s: %w", g.Account, g.Venue, ErrHedgeGroupSymbolUnset)
	}
	return nil
}

func (g HedgeGroup) String() string {
	return fmt.Sprintf("%s/%s/%s", g.Account, g.Venue, g.Symbol)
}

// Holds reports whether an instrument belongs to this group.
//
// It compares venue and symbol and deliberately ignores ProductCategory and
// ContractType — that is the whole of §5.1's "deliberately grouping a trade's
// spot + perp legs". The account is not an instrument's property and cannot be
// compared here; the classifier pairs this with the account it was given, which
// is why the two checks are separate rather than one Contains method.
func (g HedgeGroup) Holds(instrument ids.VenueInstrument) bool {
	return instrument.Venue == g.Venue && instrument.Symbol == g.Symbol
}
