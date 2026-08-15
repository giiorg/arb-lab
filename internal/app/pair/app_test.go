package pair

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// secretFile writes a credential to the test's own directory and returns the
// reference to it.
//
// A FILE reference rather than an environment one, and the reason is
// t.Parallel: t.Setenv refuses to run in a parallel test, so an env-backed
// credential would serialize every end-to-end test in this package. The words
// are "k" and "s" because pinnedPortfolioMarginAttestation pins the fingerprint
// of exactly that credential, and §4.1's attestation would otherwise be bound
// to a key this run does not use.
func secretFile(t *testing.T, name, value string) config.SecretRef {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write secret %s: %v", name, err)
	}
	return ref(t, "file:"+path)
}

// runnableConfig is validConfig pointed at two live venue doubles.
func runnableConfig(t *testing.T) (config.Snapshot[Config], [2]*venueDouble) {
	t.Helper()
	doubles := [2]*venueDouble{
		newVenueDouble(t, "binance", "LABUSDT", "LAB"),
		newVenueDouble(t, "bybit", "LABUSDT", "LAB"),
	}
	cfg := validConfig(t)
	for i := range cfg.Sides {
		v := &cfg.Sides[i].OMS.Venue
		v.APIKeyRef = secretFile(t, "api-key", "k")
		v.APISecretRef = secretFile(t, "api-secret", "s")
		for j := range v.Endpoints {
			v.Endpoints[j].Scheme = "http"
			v.Endpoints[j].Host = doubles[i].host()
		}
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the runnable fixture does not validate: %v", err)
	}
	return config.Snapshot[Config]{Value: cfg}, doubles
}

// runnableConfigWithPositionAppearingAfterBoot is runnableConfig where one
// side's venue reports `net` from the gate's read onward, having answered flat
// for the boot's own enumeration. See positionAppearingAfterBoot for why the
// position arrives when it does.
func runnableConfigWithPositionAppearingAfterBoot(t *testing.T, side int, net string) (config.Snapshot[Config], [2]*venueDouble) {
	t.Helper()
	snap, doubles := runnableConfig(t)
	leg := legOnSide(t, snap.Value, side)
	intent, err := snap.Value.legIntent(leg)
	if err != nil {
		t.Fatalf("legIntent(%d): %v", leg, err)
	}
	doubles[side].positionAppearingAfterBoot(intent.Instrument, net)
	return snap, doubles
}

// legOnSide is the index of the leg belonging to one side.
func legOnSide(t *testing.T, cfg Config, side int) int {
	t.Helper()
	for leg := range cfg.Pair.Legs {
		if i, ok := cfg.sideFor(cfg.Pair.Legs[leg].Venue); ok && i == side {
			return leg
		}
	}
	t.Fatalf("no leg names side %d's venue", side)
	return -1
}

// TestRunRefusesToDriveWhenEitherBootIsUnclean is the gate. It runs TWICE —
// once with side A unclean and once with side B — because a gate that only
// read side A would pass a one-sided test and then open a naked leg on the
// venue whose boot it never consulted. This is the mirror rule, on the side
// axis.
func TestRunRefusesToDriveWhenEitherBootIsUnclean(t *testing.T) {
	t.Parallel()
	for _, unclean := range []int{0, 1} {
		t.Run(fmt.Sprintf("side %d unclean", unclean), func(t *testing.T) {
			t.Parallel()
			snap, servers := runnableConfig(t)
			servers[unclean].refuseVenueJoin()

			rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
			if err != nil {
				t.Fatalf("Run() error = %v, want nil (a refusal belongs in the report)", err)
			}
			if rep.Drove {
				t.Fatal("Drove = true on an unclean boot; nothing may be sent")
			}
			if !errors.Is(rep.Refusal, ErrBootNotClean) {
				t.Errorf("Refusal = %v, want %v", rep.Refusal, ErrBootNotClean)
			}
			for i := range rep.Sides {
				if rep.Sides[i].Open != nil {
					t.Errorf("side %d attempted an open leg after an unclean boot on side %d", i, unclean)
				}
			}
			// Zero Place calls on BOTH sides is the claim, not just on the
			// unclean one: the clean side is the one that would have been
			// dispatched into before the other side's verdict was read.
			for i, s := range servers {
				if n := s.placeCalls(); n != 0 {
					t.Errorf("side %d received %d Place calls, want 0", i, n)
				}
			}
		})
	}
}

// TestRunRefusesToDriveWhenEitherLegStartsNonFlat is the second half of the
// gate, and it runs once per side. A gate that read only side 0 would pass one
// fixture and then trust a reduce-only inverse on the unread side's
// pre-existing position.
func TestRunRefusesToDriveWhenEitherLegStartsNonFlat(t *testing.T) {
	t.Parallel()
	for _, nonflat := range []int{0, 1} {
		t.Run(fmt.Sprintf("side %d non-flat", nonflat), func(t *testing.T) {
			t.Parallel()
			snap, servers := runnableConfigWithPositionAppearingAfterBoot(t, nonflat, "25")

			rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
			if err != nil {
				t.Fatalf("Run() = %v, want nil (a gate refusal belongs in the report)", err)
			}
			if rep.Drove {
				t.Fatal("Drove = true from a non-flat start; the inverse unwind may increase that position")
			}
			for i := range rep.Sides {
				if rep.Sides[i].Boot.ExitClass() != reconcile.ExitClean {
					t.Fatalf("side %d boot = %s, want ExitClean; the flat-start gate must be the refusal under test",
						i, rep.Sides[i].Boot.ExitClass())
				}
			}
			// The fixture's staging is asserted rather than assumed: the boot
			// made exactly the enumerations it answered flat for, so the gate's
			// read is the one that saw 25.
			if got := servers[nonflat].enumerationCount(); got < 2 {
				t.Fatalf("side %d answered %d enumerations; the gate's own read never happened",
					nonflat, got)
			}
			if !errors.Is(rep.Refusal, ErrPositionNotFlat) {
				t.Errorf("Refusal = %v, want %v", rep.Refusal, ErrPositionNotFlat)
			}
			for i, s := range servers {
				if n := s.placeCalls(); n != 0 {
					t.Errorf("side %d received %d Place calls from a non-flat start, want 0", i, n)
				}
			}
		})
	}
}

// TestRunBootsBothSidesAndReadsBothFlatBeforeDriving is the positive control:
// with two clean boots and explicit venue-flat answers the run proceeds.
// Reconciliation.Assertion's zero value means "did not run", not flat, so the
// positive control must come from VenuePositions too.
func TestRunBootsBothSidesAndReadsBothFlatBeforeDriving(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	for leg := range snap.Value.Pair.Legs {
		intent, err := snap.Value.legIntent(leg)
		if err != nil {
			t.Fatalf("legIntent(%d): %v", leg, err)
		}
		side, _ := snap.Value.sideFor(snap.Value.Pair.Legs[leg].Venue)
		servers[side].position(intent.Instrument, "0")
	}
	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	for i := range rep.Sides {
		if got := rep.Sides[i].Boot.ExitClass(); got != reconcile.ExitClean {
			t.Fatalf("side %d boot = %s, want ExitClean", i, got)
		}
		if rep.Sides[i].Venue != snap.Value.Sides[i].OMS.Venue.Venue {
			t.Errorf("side %d report names %s, want %s", i, rep.Sides[i].Venue, snap.Value.Sides[i].OMS.Venue.Venue)
		}
	}
	if !rep.Drove {
		t.Fatalf("Drove = false with two clean boots; Refusal = %v", rep.Refusal)
	}
}

// TestRunRefusesABadReferenceRevisionBeforeAnyBoot is the ordering claim the
// filters resolution rests on: a document naming a reference this run cannot
// read must fail with the FILE named, before a spend lock is taken, a journal
// is opened or a venue is called.
func TestRunRefusesABadReferenceRevisionBeforeAnyBoot(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	snap.Value.Reference.Path += ".missing"

	if _, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard); !errors.Is(err, ErrReferenceUnreadable) {
		t.Fatalf("Run() = %v, want %v", err, ErrReferenceUnreadable)
	}
	for i, s := range servers {
		if n := s.placeCalls(); n != 0 {
			t.Errorf("side %d received %d Place calls, want 0", i, n)
		}
		if n := s.enumerationCount(); n != 0 {
			t.Errorf("side %d answered %d enumerations; no venue may be reached before the reference resolves", i, n)
		}
	}
}

// TestPositionScopeForRefusesAnInstrumentNoScopeCovers keeps the wiring defect
// a refusal rather than a flat reading. Absence of an enumeration must never
// read as "we asked and it was flat" on the one read the unwind rests on.
func TestPositionScopeForRefusesAnInstrumentNoScopeCovers(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	side := c.Sides[0].OMS
	unknown := ids.VenueInstrument{
		Venue: side.Venue.Venue, Product: ids.ProductInversePerp,
		Contract: ids.ContractInverse, Symbol: "LABUSD",
	}
	if _, err := positionScopeFor(side, unknown); err == nil {
		t.Fatal("positionScopeFor(uncovered instrument) = nil; want an error")
	}
	// The positive control: the configured instrument DOES resolve, or a
	// function that objected to everything would score perfectly above.
	covered, err := c.legIntent(0)
	if err != nil {
		t.Fatalf("legIntent(0): %v", err)
	}
	scope, err := positionScopeFor(side, covered.Instrument)
	if err != nil {
		t.Fatalf("positionScopeFor(configured instrument) = %v, want nil", err)
	}
	if scope.Venue != covered.Instrument.Venue || scope.Product != covered.Instrument.Product {
		t.Errorf("scope = %s, want one covering %s", scope, covered.Instrument)
	}
}
