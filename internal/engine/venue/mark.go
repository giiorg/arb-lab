package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Mark is one venue's mark price — §3.4's third data class, and the input
// §7.1's liq-distance trigger is a function of.
//
// # It is a data class of its own, though it arrives on the funding frame
//
// On both venues the mark and the funding rate ride the SAME message: Binance's
// `<sym>@markPrice` carries `p` beside `r`, Bybit's `tickers.<sym>` carries
// markPrice beside fundingRate. That makes one struct tempting and it is wrong,
// for two independent reasons — either alone would settle it.
//
// FIRST, §5.2 configures freshness budgets "one per (data class, venue) — book,
// mark and funding", so one struct is one budget, and the mark would inherit
// funding's. That is the failure ports.BookBudgets already names from the other
// side: a tolerance written for metadata that moves every few hours, applied to
// the number a liq distance is computed from. §7.1's whole point is that the
// mark "moves continuously with no account mutation".
//
// SECOND, and specific to Bybit: BybitTickerState emits "exactly when the
// message is a snapshot or a delta carrying at least one funding field",
// because most deltas are churn and "emitting ~10/s identical readings would
// round-trip the full two-hop dispatch for nothing". That rule is right for
// funding and inverted for the mark — those ~10/s deltas ARE the mark's news.
// A mark riding the funding emission gate would refresh roughly once a minute,
// which is §7.1's own defect one class over: a trigger frozen behind a slower
// event's cadence, and "a pure rally would otherwise go undetected until
// liquidation".
//
// # It carries no provenance and no instrument
//
// Funding's rule, for Funding's reason: "the Envelope is authoritative for
// provenance; a normalized payload carries none", and SourceID is the mailbox
// key so a payload naming an instrument is a second copy of one fact.
//
// # It carries the mark and nothing beside it
//
// Binance's frame also states an index price (`i`) and an estimated settlement
// price (`P`); Bybit's ticker states indexPrice. None is decoded, on this
// codebase's standing rule that a field nobody reads is an unchecked claim
// wearing a type annotation. §7.1 names one input — the mark — and the venue's
// own mark is the definition of the number its liquidation engine uses. When a
// consumer for the index arrives it comes with the rule that reads it.
type Mark struct {
	// Price is the venue's mark price for the instrument, in its quote asset.
	//
	// It is the VENUE's number and never one this system reconstructs. §7.1
	// computes liq distance against the venue's mark and its published
	// liquidation price, and there the venue's computation IS the definition —
	// a locally-derived mark would be this system disagreeing with the engine
	// that actually liquidates the position.
	Price fixedpoint.Price
}

// Validate reports whether the reading could have come from a real venue.
//
// # A non-positive mark is REFUSED, and that is not fixedpoint's rule loosened
//
// fixedpoint.Price permits a negative value on purpose — "calendar spreads and
// some settled contracts trade below zero, and screening that is a venue's
// business, not this package's". This is that venue business. A perpetual's
// mark price is strictly positive on both MVP venues, and the consumer decides
// it: §7.1's liq distance is a ratio whose DENOMINATOR is this number, so a
// zero mark is a division by zero reached during the incident the trigger
// exists for, and a negative one silently flips the sign of the distance —
// reporting a breaching position as having ample headroom.
//
// The zero value fails here for the same reason it fails everywhere in
// platform/: an unpopulated mark reads as a legitimate number, and every
// direction it computes the trigger in is the permissive one.
func (m Mark) Validate() error {
	if err := m.Price.Validate(); err != nil {
		return fmt.Errorf("mark price: %w", err)
	}
	sign, err := m.Price.Sign()
	if err != nil {
		return fmt.Errorf("mark price: %w", err)
	}
	if sign <= 0 {
		return fmt.Errorf("mark price %s: %w", m.Price, ErrMarkNotPositive)
	}
	return nil
}
