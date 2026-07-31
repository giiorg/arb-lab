package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// PositionView is §3.3's immutable view of a held package: the exit
// direction's position facts (§40b) — held quantity, entry evidence, and the
// recorded entry HoldPlan.
//
// The entry plan here is the IMMUTABLE record (§40f); the working schedule an
// exit evaluates is the current HoldPlanRevision carried separately in
// DecisionInput. Carrying both is not a second copy of one fact: they answer
// different questions ("what did we commit to" vs "what does the venue's live
// schedule make of it"), and the interval-flip rule is precisely that the two
// diverge.
type PositionView struct {
	// Spot and Perp are the package's legs, canonical per §5.1.
	Spot, Perp ids.VenueInstrument

	// Qty is the held base quantity, identical across legs for neutrality.
	Qty fixedpoint.Quantity

	// EnteredAt is the entry instant, engine clock.
	EnteredAt time.Time

	// EntryPlan is the plan recorded at entry — evidence, never the working
	// schedule.
	EntryPlan HoldPlan
}

// isZero reports whether nobody populated the view at all. Written out
// field-by-field because the struct holds a slice and cannot be compared;
// used by DecisionInput to refuse position facts on an ENTRY evaluation.
func (p PositionView) isZero() bool {
	return p.Spot == (ids.VenueInstrument{}) &&
		p.Perp == (ids.VenueInstrument{}) &&
		p.Qty == (fixedpoint.Quantity{}) &&
		p.EnteredAt.IsZero() &&
		p.EntryPlan.Settlements == nil &&
		p.EntryPlan.Terminal.IsZero()
}

// Validate reports whether the view describes a package that could be held.
func (p PositionView) Validate() error {
	if err := p.Spot.Validate(); err != nil {
		return fmt.Errorf("spot: %w", err)
	}
	if p.Spot.Product != ids.ProductSpot {
		return fmt.Errorf("spot leg is %s: %w", p.Spot, ErrProductWrong)
	}
	if err := p.Perp.Validate(); err != nil {
		return fmt.Errorf("perp: %w", err)
	}
	if !p.Perp.Product.IsPerp() {
		return fmt.Errorf("perp leg is %s: %w", p.Perp, ErrProductWrong)
	}
	if err := p.Qty.Validate(); err != nil {
		return fmt.Errorf("qty: %w", err)
	}
	if p.EnteredAt.IsZero() {
		return fmt.Errorf("entered at: %w", ErrUnset)
	}
	if err := p.EntryPlan.Validate(); err != nil {
		return fmt.Errorf("entry plan: %w", err)
	}
	return nil
}
