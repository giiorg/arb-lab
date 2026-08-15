package oms

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// LegState is what a composition root may read about one leg it dispatched.
//
// # Why it exists here and not at the root
//
// §3.6 rule 6 bars every app/ root from importing oms/application/ports, by
// position, and ports.OrderRecord is where the state and the cumulative live.
// The rule is right — a root that can name the write port can grow a second
// write path — and the read it forbids is one a cross-venue root genuinely
// needs: it cannot size a hedge against a fill it cannot see. So the read is a
// PROJECTION, minted in the one package the gate permits, into the vocabulary
// oms/domain already speaks. That is §5.1's own pattern and it is the reason
// oms/domain is deliberately outside the gate's scope.
//
// # It is read-only and it is not a verdict
//
// It reports what the current-state projection holds. It does NOT say the
// reading is trustworthy: a Cumulative of zero on a non-terminal order means
// "nothing has been attributed yet", which is not the same claim as "nothing
// filled". The caller composes that judgement from this plus its own join
// report — see internal/app/pair's Compose, which refuses to conclude on any
// leg that is not terminal for state.
type LegState struct {
	// Instrument and Side are the plan's own, fixed when the order was planned.
	Instrument ids.VenueInstrument
	Side       domain.Side

	// State is where the order is in SM §1's lifecycle.
	State domain.OrderState

	// Ordered is the planned size; Cumulative is what the fill ledger has
	// admitted, zero OF THE ORDERED ASSET on a record with no fills.
	Ordered    fixedpoint.Quantity
	Cumulative fixedpoint.Quantity

	// Fills is how many distinct executions were admitted. It is here for the
	// same reason ports.OrderRecord carries it: a cumulative that matches while
	// the count does not is what a double-counted fill plus a missed one looks
	// like from the quantity alone.
	Fills int
}

// LegState reports the projection's current answer for one leg.
//
// An unknown id is an ERROR rather than a zero value, and the direction is the
// point: a zero LegState reads as "planned, nothing filled", which is the
// flattering answer for a leg that may be live at a venue.
func (m *Module) LegState(id ids.ClientOrderID) (LegState, error) {
	rec, err := m.orders.Load(id)
	if err != nil {
		return LegState{}, fmt.Errorf("leg state %q: %w", id, err)
	}
	return LegState{
		Instrument: rec.Order.Instrument,
		Side:       rec.Order.Side,
		State:      rec.State,
		Ordered:    rec.Order.Quantity,
		Cumulative: rec.Cumulative,
		Fills:      rec.Fills,
	}, nil
}

// VenuePosition is the venue-owned position projection for one instrument.
type VenuePosition struct {
	Instrument ids.VenueInstrument
	Net        fixedpoint.Quantity
}

// VenuePosition reports the VENUE'S OWN net position for one declared
// instrument. It does not derive from the journal: a fresh journal is flat no
// matter what the account already holds, which is precisely the non-flat start
// the pair root must refuse before a reduce-only inverse can be trusted.
//
// A declared instrument absent from the venue's complete scope is returned as
// a typed zero. An instrument the module never declared is refused; treating
// those two absences alike turns "we enumerated it and it was absent" into
// "we never asked about it" while reporting both as flat.
func (m *Module) VenuePosition(scope application.PositionScope, instrument ids.VenueInstrument) (VenuePosition, error) {
	assets, ok := m.instrumentAssets[instrument]
	if !ok {
		return VenuePosition{}, fmt.Errorf("venue position %s: instrument is not declared", instrument)
	}
	report, err := m.venuePositions.Positions(scope)
	if err != nil {
		return VenuePosition{}, fmt.Errorf("venue position %s: %w", instrument, err)
	}
	for _, row := range report.Positions {
		if row.Instrument == instrument {
			return VenuePosition{Instrument: instrument, Net: row.Net}, nil
		}
	}
	zero, err := fixedpoint.ParseQuantity(assets.Base, "0")
	if err != nil {
		return VenuePosition{}, fmt.Errorf("venue position %s typed zero: %w", instrument, err)
	}
	return VenuePosition{Instrument: instrument, Net: zero}, nil
}
