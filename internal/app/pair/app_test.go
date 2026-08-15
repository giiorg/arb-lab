package pair

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// stubSleep returns a runner that passes f as the settle loop's wait. It
// mutates no package state, so parallel tests cannot see each other's clock
// edges — oms.Deps.SettleWait's precedent.
func stubSleep(f func(time.Duration)) func(context.Context, config.Snapshot[Config], func() *observability.Registry, io.Writer) (Report, error) {
	return func(ctx context.Context, snap config.Snapshot[Config],
		reg func() *observability.Registry, stderr io.Writer) (Report, error) {
		return runWithWait(ctx, snap, reg, stderr, f)
	}
}

// TestSecondLegIsSizedToTheFirstLegsEstablishedFill is D5. The venue double
// fills side A PARTIALLY — 60 of the ordered 100 — and the assertion is on the
// quantity side B was ASKED for.
//
// 60, not 50: at one half the filled quantity equals its own remainder and a
// hedge sized to the unfilled part would produce the same number.
func TestSecondLegIsSizedToTheFirstLegsEstablishedFill(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60") // ordered 100, filled 60
	servers[1].fillIOC("60")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if got := servers[1].lastPlaceQuantity(); got != "60" {
		t.Fatalf("side B was asked for %q, want the established fill \"60\" and not the document's \"100\" (refusal %v)",
			got, rep.Refusal)
	}
	if rep.Verdict != VerdictIntact {
		t.Errorf("Verdict = %s, want VerdictIntact (refusal %v)", rep.Verdict, rep.Refusal)
	}
}

// TestSideBIsNotSentWhenSideAFilledNothing is D6. An IOC that did not fill
// opened nothing, so there is nothing to hedge — and sending B would open the
// naked leg this root exists to prevent.
func TestSideBIsNotSentWhenSideAFilledNothing(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("0")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if n := servers[1].placeCalls(); n != 0 {
		t.Fatalf("side B received %d Place calls after side A filled nothing, want 0", n)
	}
	if n := servers[0].placeCalls(); n != 1 {
		t.Fatalf("side A received %d Place calls, want 1 — the control for the assertion above", n)
	}
	if rep.Verdict != VerdictUnopened {
		t.Errorf("Verdict = %s, want VerdictUnopened (refusal %v)", rep.Verdict, rep.Refusal)
	}
	// D6 says this run is CLEAN, and the assertion is load-bearing rather than
	// decorative: with the zero-fill gate removed, leg B is planned at a
	// quantity of zero and the PLANNER refuses it — so side B still receives no
	// Place call and this test would pass for the wrong reason. The refusal is
	// where the two differ. (Confirmed: mutating the gate SURVIVED until this
	// line existed.)
	if rep.Refusal != nil {
		t.Errorf("Refusal = %v, want nil — an IOC that did not fill opened nothing, which is a real clean result", rep.Refusal)
	}
	if rep.Sides[1].Open != nil {
		t.Errorf("side B has an Open report (%+v); nothing may be ATTEMPTED on it either", rep.Sides[1].Open)
	}
}

// TestSideBIsNotSentWhileSideAIsIndeterminate is D4's whole point. The venue
// double leaves side A's order NON-TERMINAL for every settle attempt, so its
// cumulative reads zero — the same number TestSideBIsNotSentWhenSideAFilledNothing
// produces, from a completely different situation. The two tests together are
// what stop a confident zero standing in for an unfinished read.
func TestSideBIsNotSentWhileSideAIsIndeterminate(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].holdOrderNonTerminal()

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if n := servers[1].placeCalls(); n != 0 {
		t.Fatalf("side B received %d Place calls while side A was indeterminate, want 0", n)
	}
	if rep.Verdict != VerdictIndeterminate {
		t.Errorf("Verdict = %s, want VerdictIndeterminate (refusal %v)", rep.Verdict, rep.Refusal)
	}
	if rep.Sides[0].Exposure.Because == "" {
		t.Error("Exposure.Because is empty; an indeterminate leg must name why")
	}
	// The same masking as D6's, one branch over: with the Established guard
	// removed, leg B is planned against the projection's zero and refused by the
	// planner, so the Place count above stays 0 and only the REFUSAL says which
	// code ran. A run stopped because a reading was unfinished must not report a
	// hedge-sizing problem.
	if rep.Refusal != nil {
		t.Errorf("Refusal = %v, want nil — the run stops on the VERDICT, not on a refusal to size a hedge", rep.Refusal)
	}
	if rep.Sides[1].Open != nil {
		t.Errorf("side B has an Open report (%+v); an unestablished side A may not be hedged at all", rep.Sides[1].Open)
	}
}

// TestTheTwoZeroCumulativeReadingsAreDIFFERENT is the venue double's own
// self-test, and it is what keeps the two tests above from being one test
// written twice.
//
// Both produce a cumulative of zero. If the double could not answer the ORDER
// QUERY differently in the two modes, one of them would be passing for the
// other's reason and neither assertion would be about what it says.
func TestTheTwoZeroCumulativeReadingsAreDifferent(t *testing.T) {
	t.Parallel()
	held := newVenueDouble(t, "binance", "LABUSDT", "LAB")
	held.holdOrderNonTerminal()
	settled := newVenueDouble(t, "binance", "LABUSDT", "LAB")
	settled.fillIOC("0")

	heldStatus, heldQty := held.outcome("ord-1")
	settledStatus, settledQty := settled.outcome("ord-1")
	if heldQty != settledQty {
		t.Fatalf("the two modes report %q and %q; this self-test is about a SHARED zero", heldQty, settledQty)
	}
	if heldStatus == settledStatus {
		t.Fatalf("both modes answer %q; the double cannot tell 'not terminal yet' from 'terminal, filled nothing'",
			heldStatus)
	}
}

// TestTheSettleLoopIsBounded. A mutation that stopped the venue answering
// would otherwise make this test HANG rather than fail, and a hanging test
// names no defect. The injected sleep counts calls and the assertion is on the
// count, so the bound is checked rather than waited out.
func TestTheSettleLoopIsBounded(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].holdOrderNonTerminal()
	var sleeps int
	run := stubSleep(func(time.Duration) { sleeps++ })

	if _, err := run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	// settle_attempts is 3 in the fixture, so at most 2 waits BETWEEN 3 reads.
	if sleeps > 2 {
		t.Fatalf("settle loop slept %d times, want at most 2 for settle_attempts=3", sleeps)
	}
	// And it must have slept at ALL, or "bounded" and "never retried" are the
	// same observation and a loop that read once would pass.
	if sleeps == 0 {
		t.Fatal("settle loop never waited; a single read is not a settle loop")
	}
}

// TestLegOrderDoesNotSelectTheSideModule reverses the leg list while leaving
// Sides in binance/bybit order. Leg order decides A then B; each leg's Venue
// decides which runner, filter map and report slot receive it.
func TestLegOrderDoesNotSelectTheSideModule(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	snap.Value.Pair.Legs[0], snap.Value.Pair.Legs[1] = snap.Value.Pair.Legs[1], snap.Value.Pair.Legs[0]
	servers[1].fillIOC("100") // leg A is now bybit
	servers[0].fillIOC("100") // leg B is now binance

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if servers[0].placeCalls() != 1 || servers[1].placeCalls() != 1 {
		t.Fatalf("Place calls binance/bybit = %d/%d, want 1/1 on each leg's own module (refusal %v)",
			servers[0].placeCalls(), servers[1].placeCalls(), rep.Refusal)
	}
	bybitPlace, ok := servers[1].firstPlace()
	if !ok {
		t.Fatal("bybit received no order")
	}
	if !strings.EqualFold(bybitPlace.Side, "buy") {
		t.Errorf("bybit module received %s, want reversed leg A's buy", bybitPlace.Side)
	}
	binancePlace, ok := servers[0].firstPlace()
	if !ok {
		t.Fatal("binance received no order")
	}
	if !strings.EqualFold(binancePlace.Side, "sell") {
		t.Errorf("binance module received %s, want reversed leg B's sell", binancePlace.Side)
	}
	// And the ORDER of the two sends is the sequencing claim: leg A's venue was
	// asked first, which is the property a side-indexed drive would invert.
	if !servers[1].sawPlaceBefore(binancePlace.At) {
		t.Error("binance was sent before bybit; the reversed document's leg A is bybit's")
	}
}

// TestSideBIsNotSentWhileSideAHoldsUNMEASUREDExposure is the isolating fixture
// for D4's Established guard, and it exists because the mutation proving that
// guard SURVIVED without it.
//
// Side A's order is live at the venue with 60 already executed against it. Its
// exposure is therefore NON-ZERO and NOT ESTABLISHED — the state
// TestSideBIsNotSentWhileSideAIsIndeterminate cannot reach, because there the
// leg is unestablished AND flat, so the two guards are one predicate and either
// alone stops the run.
//
// This is the most dangerous reading in this root: real exposure that has not
// been measured. Hedging it sizes side B from a number that is still moving.
func TestSideBIsNotSentWhileSideAHoldsUnmeasuredExposure(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].holdOrderNonTerminalWithFills("60")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	// The fixture's own precondition, asserted rather than assumed: without a
	// non-zero unestablished quantity this test is the previous one again.
	a := rep.Sides[0].Exposure
	if a.Established {
		t.Fatalf("side A reads established (%+v); the fixture is not holding the order", a)
	}
	if sign, err := a.Filled.Sign(); err != nil || sign == 0 {
		t.Fatalf("side A's unestablished quantity is %v (err %v); this test needs a NON-ZERO one, "+
			"or the flat guard alone would stop the run", a.Filled, err)
	}
	if n := servers[1].placeCalls(); n != 0 {
		t.Fatalf("side B received %d Place calls against side A's unmeasured exposure, want 0", n)
	}
	if rep.Verdict != VerdictIndeterminate {
		t.Errorf("Verdict = %s, want VerdictIndeterminate (refusal %v)", rep.Verdict, rep.Refusal)
	}
}
