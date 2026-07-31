package domain

import (
	"fmt"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// PriceView is one leg's top of book with its provenance.
//
// It carries bid AND ask rather than a collapsed mid, for the discipline
// venue-profiles records: the venue publishes the book; a mid is a locally
// derived judgement, and which side of the spread a term should be priced
// against is the CORE's judgement per §2.4's conservative direction — a
// pre-collapsed mid would make that judgement here, silently, for every
// consumer at once.
type PriceView struct {
	Instrument ids.VenueInstrument

	// Bid and Ask are the best resting levels' prices. Depth is deliberately
	// absent: the slippage charge is ExecutionCostModel's business and that
	// adapter sources its own book state (§40e); what a DECISION needs from
	// the view is where the market is, with provenance.
	Bid, Ask fixedpoint.Price

	// Watermark is the book observation's provenance (§3.4). Ages are derived
	// from it against an instant the caller supplies — the view stores no
	// freshness verdict, for engine/state's reason: a stored verdict ages
	// without anything arriving to re-judge it.
	Watermark mdomain.Watermark
}

// Validate reports whether the view could describe a real top of book.
func (p PriceView) Validate() error {
	if err := p.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if err := p.Bid.Validate(); err != nil {
		return fmt.Errorf("bid: %w", err)
	}
	if err := p.Ask.Validate(); err != nil {
		return fmt.Errorf("ask: %w", err)
	}
	c, err := p.Bid.Cmp(p.Ask)
	if err != nil {
		return fmt.Errorf("compare bid to ask: %w", err)
	}
	// A locked top (bid == ask) is admitted: some venues print it
	// legitimately during fee-tier promotions. A crossed one is not — see
	// ErrCrossedTop.
	if c > 0 {
		return fmt.Errorf("bid %s above ask %s: %w", p.Bid, p.Ask, ErrCrossedTop)
	}
	if err := p.Watermark.Validate(); err != nil {
		return fmt.Errorf("watermark: %w", err)
	}
	return nil
}

// RealizedSettlement is one funding settlement that actually happened: the
// venue's own settled rate at the venue's own settlement instant.
//
// Venue-published, never locally derived: the venue's computation of its own
// funding settlement IS the definition, and a rate this process derived from
// the estimate stream would drift from the ledger entry the venue actually
// pays. The EWMA (§2.4) and the exit dwell (§40f: "the dwell counts realized
// settlements observed, never entries in the mutable schedule") both consume
// these.
type RealizedSettlement struct {
	Instrument ids.VenueInstrument

	// At is the settlement instant, in the venue's own time base.
	At time.Time

	// Rate is the settled per-settlement rate.
	Rate fixedpoint.Rate

	// Watermark is the provenance of the observation that reported the
	// settlement.
	Watermark mdomain.Watermark
}

// Validate reports whether this could be a real settlement.
func (r RealizedSettlement) Validate() error {
	if err := r.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if !r.Instrument.Product.IsPerp() {
		return fmt.Errorf("instrument %s: %w", r.Instrument, ErrProductWrong)
	}
	if r.At.IsZero() {
		return fmt.Errorf("settled at: %w", ErrUnset)
	}
	if err := r.Rate.Validate(); err != nil {
		return fmt.Errorf("rate: %w", err)
	}
	if err := r.Watermark.Validate(); err != nil {
		return fmt.Errorf("watermark: %w", err)
	}
	return nil
}

// validateWindow checks a realized-settlement window: every entry valid, all
// on one instrument, strictly ascending. Shared by FundingMaterials and
// MarketView so the two cannot drift.
func validateWindow(w []RealizedSettlement, inst ids.VenueInstrument) error {
	for n, s := range w {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("realized %d: %w", n, err)
		}
		if s.Instrument != inst {
			return fmt.Errorf("realized %d is %s, window is %s: %w", n, s.Instrument, inst, ErrLegsMismatch)
		}
		if n > 0 && !w[n-1].At.Before(s.At) {
			return fmt.Errorf("realized %d at %s, %d at %s: %w", n-1, w[n-1].At, n, s.At, ErrNotAscending)
		}
	}
	return nil
}

// FundingMaterials is the forecast's explicit input: §2.4's "MarketView
// carries the raw materials (live estimate, recent realized settlements, live
// cap/floor, interval)", carried as an argument rather than held by the
// estimator (§40e's explicit-materials signature — an estimator holding
// market state internally is untestable as a function).
type FundingMaterials struct {
	// Observation is the live funding block: estimate for the next
	// settlement, live interval, next settlement instant, live cap/floor.
	Observation mdomain.FundingObservation

	// Realized is the recent realized-settlement window, strictly ascending.
	// It may be empty: the imminent settlement needs only the live estimate,
	// and how much history LATER settlements need is the estimator's stated
	// business (§2.4's half-life param), not this type's.
	Realized []RealizedSettlement
}

// Validate reports whether the materials could feed a forecast.
func (m FundingMaterials) Validate() error {
	if err := m.Observation.Validate(); err != nil {
		return fmt.Errorf("observation: %w", err)
	}
	return validateWindow(m.Realized, m.Observation.Instrument)
}

// MarketView is §3.3's strategy-owned, immutable view of the market: exactly
// the raw materials §2.4 names, per instrument, plus the §3.4 watermarks —
// and nothing that exists to serve a consumer this milestone does not have
// (§40c).
//
// # Cross-leg skew is derived, not carried
//
// §3.4: skew = |Age(a) − Age(b)| at any single instant — the instant cancels,
// so the skew is a pure function of the two RecvTimes already carried here. A
// stored skew field would be a second authority over that subtraction, aging
// in a value type while the watermarks beside it stay authoritative. The core
// derives it at the read, which is §3.4's read-time re-judgement rule.
type MarketView struct {
	// Spot and Perp are the two legs' tops of book.
	Spot, Perp PriceView

	// Funding is the perp's live funding block — marketdata's own observation
	// type, imported rather than re-declared (see the package doc), so its
	// Enumerate stays the only settlement source (§40f).
	Funding mdomain.FundingObservation

	// Realized is the recent realized-settlement window for the perp,
	// strictly ascending. §2.4 names it a MarketView material explicitly.
	Realized []RealizedSettlement
}

// Validate reports whether the view is one a decision may be scored over.
func (v MarketView) Validate() error {
	if err := v.Spot.Validate(); err != nil {
		return fmt.Errorf("spot: %w", err)
	}
	if v.Spot.Instrument.Product != ids.ProductSpot {
		return fmt.Errorf("spot leg is %s: %w", v.Spot.Instrument, ErrProductWrong)
	}
	if err := v.Perp.Validate(); err != nil {
		return fmt.Errorf("perp: %w", err)
	}
	if !v.Perp.Instrument.Product.IsPerp() {
		return fmt.Errorf("perp leg is %s: %w", v.Perp.Instrument, ErrProductWrong)
	}
	if err := v.Funding.Validate(); err != nil {
		return fmt.Errorf("funding: %w", err)
	}
	// The funding block must describe the perp leg — a view assembled from
	// two ports (§40c) is exactly where one consumer's BTCUSDT and another's
	// ETHUSDT could meet without either port being wrong.
	if v.Funding.Instrument != v.Perp.Instrument {
		return fmt.Errorf("funding is %s, perp leg is %s: %w", v.Funding.Instrument, v.Perp.Instrument, ErrLegsMismatch)
	}
	return validateWindow(v.Realized, v.Perp.Instrument)
}

// Materials is the view's funding block in the form the FundingForecast port
// consumes. One derivation, here, so the assembly cannot pick a different
// window than the view holds.
func (v MarketView) Materials() FundingMaterials {
	return FundingMaterials{Observation: v.Funding, Realized: v.Realized}
}
