package oms_test

import (
	"context"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// eqQty fails the test unless got and want are the same quantity, INCLUDING the
// asset tag. An error from Cmp is a failure, not a mismatch — it means one of
// the two was never populated, which is the case this repo's fixedpoint refuses
// to let read as zero.
func eqQty(t *testing.T, got, want fixedpoint.Quantity) {
	t.Helper()
	cmp, err := got.Cmp(want)
	if err != nil {
		t.Fatalf("comparing %v and %v: %v", got, want, err)
	}
	if cmp != 0 {
		t.Errorf("quantity = %v, want %v", got, want)
	}
}

// plannedLeg is what moduleWithOnePlannedLeg planned, carried beside the module
// so that no assertion below re-derives a field the fixture already chose.
type plannedLeg struct {
	ID         ids.ClientOrderID
	Instrument ids.VenueInstrument
	Side       domain.Side
	Quantity   fixedpoint.Quantity
}

// moduleWithOnePlannedLeg builds the real context over a real journal and plans
// §2.2's package through it, returning the FIRST leg.
//
// It reuses module_test.go's fixtures rather than growing a second construction
// path: fullDeps is the populated wiring every construction case there breaks
// one field of, and cashAndCarry is the two-leg plan that keeps "minted per leg"
// and "minted once" distinguishable.
func moduleWithOnePlannedLeg(t *testing.T) (*oms.Module, plannedLeg) {
	t.Helper()
	m := newModule(t, fullDeps(t))
	records, err := m.Plan.Plan(cashAndCarry(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Plan returned no records")
	}
	rec := records[0]
	return m, plannedLeg{
		ID:         rec.Order.ClientOrderID,
		Instrument: rec.Order.Instrument,
		Side:       rec.Order.Side,
		Quantity:   rec.Order.Quantity,
	}
}

// scriptedPositions answers the venue's position enumeration with one row this
// test wrote, so that the venue's own reading and a journal-derived one cannot
// be the same fixture.
type scriptedPositions struct {
	report venue.PositionReport
}

func (s scriptedPositions) Positions(context.Context, venue.PositionScope,
	map[ids.VenueInstrument]venue.InstrumentAssets, ratelimit.Priority) (venue.PositionReport, error) {
	return s.report, nil
}

// moduleWithVenuePosition builds a module whose venue reports `net` on the perp
// while its journal is fresh and therefore flat.
//
// The non-zero default is the whole point: with the venue answering zero, a
// projection that read the JOURNAL instead of the venue would give the same
// answer and the test would pass against the wrong authority.
func moduleWithVenuePosition(t *testing.T, net string) (*oms.Module, application.PositionScope, ids.VenueInstrument, fixedpoint.Quantity) {
	t.Helper()
	instrument := perpInstrument()
	want := mustQty(t, net)
	deps := fullDeps(t)
	deps.VenuePositions = scriptedPositions{report: venue.PositionReport{
		Positions: []venue.Position{{Instrument: instrument, Net: want}},
	}}
	scope := application.PositionScope{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"}
	return newModule(t, deps), scope, instrument, want
}

// TestLegStateReportsAPlannedLeg is the positive control: a leg that exists
// reads back with its instrument, side, state and a ZERO cumulative of the
// ordered asset — which ports.OrderRecord documents as a real answer rather
// than a missing one.
func TestLegStateReportsAPlannedLeg(t *testing.T) {
	t.Parallel()
	m, planned := moduleWithOnePlannedLeg(t)

	got, err := m.LegState(planned.ID)
	if err != nil {
		t.Fatalf("LegState(%q) = %v, want nil", planned.ID, err)
	}
	if got.Instrument != planned.Instrument {
		t.Errorf("Instrument = %v, want %v", got.Instrument, planned.Instrument)
	}
	if got.Side != planned.Side {
		t.Errorf("Side = %v, want %v", got.Side, planned.Side)
	}
	if got.State != domain.StatePlanned {
		t.Errorf("State = %v, want %v", got.State, domain.StatePlanned)
	}
	eqQty(t, got.Ordered, planned.Quantity)
	// Flat is Sign()==0, and Sign's error is a FAILURE rather than "flat": a
	// cumulative nobody populated is exactly what fixedpoint refuses to let read
	// as zero, and a zero cumulative of the ordered asset is what
	// ports.OrderRecord promises here.
	if sign, err := got.Cumulative.Sign(); err != nil {
		t.Errorf("Cumulative.Sign() = %v; the record must carry a zero OF THE ORDERED ASSET, not an unpopulated value", err)
	} else if sign != 0 {
		t.Errorf("Cumulative = %v, want flat", got.Cumulative)
	}
	if got.Fills != 0 {
		t.Errorf("Fills = %d, want 0", got.Fills)
	}
}

// TestLegStateRefusesAnUnknownID is the negative control. A root asking about
// an id the store never held must get an error, not a zero LegState — a zero
// LegState reads as "planned, nothing filled", which is the flattering
// direction for a leg that may be live at a venue.
func TestLegStateRefusesAnUnknownID(t *testing.T) {
	t.Parallel()
	m, _ := moduleWithOnePlannedLeg(t)

	got, err := m.LegState(ids.ClientOrderID("ab-00000000000000000000000000-00-00"))
	if err == nil {
		t.Fatalf("LegState(unknown) = %+v, nil; want an error", got)
	}
	if got != (oms.LegState{}) {
		t.Errorf("LegState on error = %+v, want the zero value", got)
	}
}

// TestVenuePositionReportsTheVenuesOwnNet is the positive control. The fresh
// journal is flat while the venue double reports a pre-existing long, so a
// projection accidentally derived from the journal cannot pass.
func TestVenuePositionReportsTheVenuesOwnNet(t *testing.T) {
	t.Parallel()
	m, scope, instrument, want := moduleWithVenuePosition(t, "25")

	got, err := m.VenuePosition(scope, instrument)
	if err != nil {
		t.Fatalf("VenuePosition(%s) = %v, want nil", instrument, err)
	}
	if got.Instrument != instrument {
		t.Errorf("Instrument = %v, want %v", got.Instrument, instrument)
	}
	eqQty(t, got.Net, want)
}

// TestVenuePositionReportsADeclaredInstrumentTheVenueOmittedAsATypedZERO is the
// other half of the absence rule. A declared instrument absent from a COMPLETE
// enumeration is flat, and it must come back as a quantity in the instrument's
// own base asset — an untagged zero would be refused by every comparison the
// caller then makes, which is a flat account reported as an unreadable one.
func TestVenuePositionReportsADeclaredInstrumentTheVenueOmittedAsATypedZERO(t *testing.T) {
	t.Parallel()
	instrument := perpInstrument()
	deps := fullDeps(t)
	deps.VenuePositions = scriptedPositions{report: venue.PositionReport{}}
	m := newModule(t, deps)
	scope := application.PositionScope{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"}

	got, err := m.VenuePosition(scope, instrument)
	if err != nil {
		t.Fatalf("VenuePosition(%s) = %v, want nil", instrument, err)
	}
	eqQty(t, got.Net, mustQty(t, "0"))
}

// TestVenuePositionRefusesAnUnknownInstrument stops an absent row for an
// instrument this module never declared being promoted into evidence of
// flatness. Absence is flat only inside the configured enumeration.
func TestVenuePositionRefusesAnUnknownInstrument(t *testing.T) {
	t.Parallel()
	m, scope, _, _ := moduleWithVenuePosition(t, "0")
	unknown := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Contract: ids.ContractLinear, Symbol: "UNKNOWNUSDT",
	}

	if _, err := m.VenuePosition(scope, unknown); err == nil {
		t.Fatal("VenuePosition(unknown) = nil; want an error")
	}
}
