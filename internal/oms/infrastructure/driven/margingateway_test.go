package driven_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks the margin TOP-UP ACL (task #215) — the one adapter in this
// package whose calls change something at a venue.
//
// # What the checklist demanded here
//
//   - THE WRITER RECORDS THE URGENCY IT WAS PASSED. §7.6's shed class is the
//     whole reason this adapter carries no translation and still exists, and a
//     wrong one is invisible in the returned value: at Opening the top-up is shed
//     at ~70% of budget, during exactly the incident the reserve exists for.
//   - THE WRITER RECORDS THE ASSETS. The table's job is to type the amount, and
//     an adapter that passed the zero InstrumentAssets would be refused three
//     layers down with a message about the venue rather than about the wiring.
//   - THE UNDECLARED-INSTRUMENT FIXTURE ASSERTS THE WRITER WAS NEVER CALLED. On
//     the act-history adapter that claim is about budget; here it is about
//     whether real margin moved.

// recordingMarginWriter is driven.VenueMarginWriter.
type recordingMarginWriter struct {
	err   error
	calls []marginWriteCall
}

type marginWriteCall struct {
	instrument ids.VenueInstrument
	assets     venue.InstrumentAssets
	amount     fixedpoint.Money
	urgency    ratelimit.Priority
}

func (w *recordingMarginWriter) AddIsolatedMargin(_ context.Context, instrument ids.VenueInstrument,
	assets venue.InstrumentAssets, amount fixedpoint.Money, urgency ratelimit.Priority) error {

	w.calls = append(w.calls, marginWriteCall{
		instrument: instrument, assets: assets, amount: amount, urgency: urgency,
	})
	return w.err
}

func newMarginGateway(t *testing.T, w *recordingMarginWriter) *driven.VenueMarginGateway {
	t.Helper()
	g, err := driven.NewVenueMarginGateway(w, positionAssets())
	if err != nil {
		t.Fatalf("NewVenueMarginGateway: %v", err)
	}
	return g
}

func topUpMoney(t *testing.T, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", v)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", v, err)
	}
	return m
}

func TestTheTopUpTravelsAtSAFETYUrgencyWithTheInstrumentsOwnAssets(t *testing.T) {
	w := &recordingMarginWriter{}

	if err := newMarginGateway(t, w).AddIsolatedMargin(perpInstrument(), topUpMoney(t, "137.45")); err != nil {
		t.Fatalf("AddIsolatedMargin: %v", err)
	}
	if len(w.calls) != 1 {
		t.Fatalf("the writer saw %d calls, want 1", len(w.calls))
	}
	call := w.calls[0]
	// §7.6 names "§7.1's margin top-up" in the Safety row in as many words, and
	// this adapter pins it rather than taking it — there is no second caller at
	// a second rung, and every other class sheds the remedy under load.
	if call.urgency != ratelimit.PrioritySafety {
		t.Errorf("urgency = %v, want Safety", call.urgency)
	}
	if call.assets != positionAssets()[perpInstrument()] {
		t.Errorf("assets = %+v, want the instrument's declared pair", call.assets)
	}
	if call.amount.Decimal() != "137.45" {
		t.Errorf("amount = %s, want 137.45", call.amount.Decimal())
	}
	if call.instrument != perpInstrument() {
		t.Errorf("instrument = %+v", call.instrument)
	}
}

func TestAnUndeclaredInstrumentIsRefusedBEFOREAnythingIsSent(t *testing.T) {
	other := ids.VenueInstrument{
		Venue: perpInstrument().Venue, Product: ids.ProductLinearPerp,
		Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
	w := &recordingMarginWriter{}

	err := newMarginGateway(t, w).AddIsolatedMargin(other, topUpMoney(t, "137.45"))
	if !errors.Is(err, driven.ErrMarginInstrumentUndeclared) {
		t.Fatalf("got %v, want ErrMarginInstrumentUndeclared", err)
	}
	// The two absorptions available here are guessing the margin asset and
	// sending anyway. Sending is a real transfer against a live position.
	if len(w.calls) != 0 {
		t.Errorf("the writer saw %d calls for an instrument this deployment never declared", len(w.calls))
	}
}

func TestAFailedTopUpIsReturnedUNWRAPPED(t *testing.T) {
	// engine/venue already classified the effect, and §5.3 forbids re-deriving
	// it from the cause. An adapter that wrapped this would still satisfy
	// errors.Is — what it would break is apperror.EffectOf, which reads the
	// error's own type.
	sentinel := errors.New("venue refused")
	w := &recordingMarginWriter{err: sentinel}

	err := newMarginGateway(t, w).AddIsolatedMargin(perpInstrument(), topUpMoney(t, "137.45"))
	if err != sentinel { //nolint:errorlint // the identity IS the claim
		t.Fatalf("got %v, want the writer's own error value", err)
	}
}

func TestVenueMarginGatewayRefusals(t *testing.T) {
	t.Run("no writer", func(t *testing.T) {
		if _, err := driven.NewVenueMarginGateway(nil, positionAssets()); !errors.Is(err, driven.ErrVenueMarginWriterMissing) {
			t.Fatalf("got %v, want ErrVenueMarginWriterMissing", err)
		}
	})
	t.Run("no assets", func(t *testing.T) {
		if _, err := driven.NewVenueMarginGateway(&recordingMarginWriter{}, nil); !errors.Is(err, driven.ErrInstrumentAssetsMissing) {
			t.Fatalf("got %v, want ErrInstrumentAssetsMissing", err)
		}
	})
	t.Run("a mixed-venue table would sign one venue's top-up with the other's key", func(t *testing.T) {
		mixed := map[ids.VenueInstrument]venue.InstrumentAssets{}
		for k, v := range positionAssets() {
			mixed[k] = v
		}
		mixed[ids.VenueInstrument{
			Venue: "bybit", Product: ids.ProductLinearPerp,
			Symbol: "BTCUSDT", Contract: ids.ContractLinear,
		}] = venue.InstrumentAssets{Base: "BTC", Quote: "USDT"}

		if _, err := driven.NewVenueMarginGateway(&recordingMarginWriter{}, mixed); !errors.Is(err, driven.ErrInstrumentVenuesMixed) {
			t.Fatalf("got %v, want ErrInstrumentVenuesMixed", err)
		}
	})

	// The positive control.
	if _, err := driven.NewVenueMarginGateway(&recordingMarginWriter{}, positionAssets()); err != nil {
		t.Fatalf("NewVenueMarginGateway with everything wired: %v", err)
	}
}
