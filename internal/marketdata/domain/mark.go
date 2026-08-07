package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MarkObservation is one reading of a venue's mark price — §3.4's third data
// class, beside funding and the book.
//
// # It is a class of its own even though it arrives on funding's frame
//
// On both MVP venues the mark rides the same message as the funding rate
// (Binance `<sym>@markPrice` states `p` beside `r`; Bybit `tickers.<sym>`
// states markPrice beside fundingRate). §5.2 nonetheless configures freshness
// budgets "one per (data class, venue) — book, mark and funding", and that is
// the whole reason to keep them apart: one struct is one Budget, and the mark
// would inherit funding's tolerance. ports.BookBudgets already names that
// failure from the book's side — a budget written for metadata that moves every
// few hours, applied to a number a decision is priced from.
//
// Here the decision is §7.1's: liq distance is a function of the mark, "which
// moves continuously with no account mutation", and a stale one is what §7.1's
// T_blind ladder escalates on. A mark measured against funding's budget would
// read fresh through the whole rally it exists to detect.
//
// # It is the VENUE's number
//
// The mark is never reconstructed here. §7.1 computes liq distance from the
// venue's mark against the venue's own published liquidation price, and there
// the venue's computation IS the definition: a locally-derived mark would be
// this system disagreeing with the engine that actually liquidates the
// position, and winning that disagreement is not a good outcome.
type MarkObservation struct {
	// Instrument is the perpetual this reads. See Validate for why spot is
	// refused rather than admitted with its last trade price.
	Instrument ids.VenueInstrument

	// Price is the venue's mark, in the instrument's quote asset.
	Price fixedpoint.Price

	// Watermark is the observation's provenance (§3.4).
	Watermark Watermark
}

// Validate reports whether the observation could have come from a real venue
// read.
//
// Checked in the order a reader would debug: what it describes, then what it
// says, then where it came from.
//
// # Spot is REFUSED, and the reason is not funding's reason reused
//
// ErrFundingOnSpot exists because a spot leg reading "funding 0" is
// indistinguishable from a perp in a flat regime. Nothing so subtle happens
// here: a spot instrument HAS a price, and a decoder pointed at a spot ticker
// would find a plausible number to put in this field — the last trade. What it
// would not have is a mark, which is a specific venue construction feeding a
// specific venue liquidation engine. §7.1's consumer would then compute a liq
// distance for a leg that cannot be liquidated, against a price that is not the
// one anything liquidates on. The refusal is what stops a wrong-but-plausible
// number entering the trigger.
//
// # A non-positive mark is REFUSED, and that is not fixedpoint's rule loosened
//
// fixedpoint.Price permits a negative value by design — "calendar spreads and
// some settled contracts trade below zero, and screening that is a venue's
// business". This is that business, and the consumer decides it: §7.1's liq
// distance is a ratio whose DENOMINATOR is this number. Zero divides by zero
// during the incident the trigger exists for; negative flips the sign of the
// headroom, reporting a breaching position as having room. venue.Mark refuses
// the same value at the wire, and this is not that check restated for tidiness
// — the two seams have different producers, and a future adapter that builds an
// observation without going through venue.Mark would otherwise carry a value
// nothing had screened.
func (o MarkObservation) Validate() error {
	if err := o.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if o.Instrument.Product == ids.ProductSpot {
		return fmt.Errorf("instrument %s: %w", o.Instrument, ErrMarkOnSpot)
	}
	if err := o.Price.Validate(); err != nil {
		return fmt.Errorf("mark price: %w", err)
	}
	sign, err := o.Price.Sign()
	if err != nil {
		return fmt.Errorf("mark price: %w", err)
	}
	if sign <= 0 {
		return fmt.Errorf("mark price %s: %w", o.Price, ErrMarkNotPositive)
	}
	if err := o.Watermark.Validate(); err != nil {
		return fmt.Errorf("watermark: %w", err)
	}
	return nil
}
