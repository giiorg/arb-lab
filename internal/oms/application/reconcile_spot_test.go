package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// This file checks the spot delta's stability bracket — the Reconciler's
// second look (task #179, record §70c; §70h items 14 and 23).
//
// # The collapse the fixtures must not allow
//
// "Re-ran the join" and "re-read the balance" are different halves of one
// bracket, and a fixture whose world never changes between the looks cannot
// tell either from a bracket that re-ran nothing. So the moving fixtures
// mutate the venue INSIDE the injected Wait — the one instant the bracket
// promises to straddle — and the stable fixture asserts the wait happened at
// all (a recorded wait with the configured bound), because on a fake that
// never advances, "waited" and "did not wait" are otherwise the same run.

// spotReconcileFixture is reconcileFixture with a wired spot leg and second
// look. onWait runs inside the bracket's Wait — where a test stages what the
// second look will see.
type spotReconcileFixture struct {
	reconciler *application.Reconciler
	joinVenue  *fakeJoinVenue
	balances   *fakeSpotBalances
	resolver   *fakeResolver

	waits  []time.Duration
	onWait func()
}

func newSpotReconcileFixture(t *testing.T, leg application.SpotLeg, wired bool, entries ...journalEntry) *spotReconcileFixture {
	t.Helper()

	f := &spotReconcileFixture{joinVenue: newFakeJoinVenue(), resolver: &fakeResolver{}}
	orders := newFakeOrders()
	replay := &fakeReplay{entries: entries}
	journal := newFakeJournal()

	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	replayer, err := application.NewReplayer(replay, orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	joiner, err := application.NewJoiner(f.joinVenue, newFakePositions(), orders, clk, joinAccount, leg)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	resolver, err := application.NewResolver(journal, orders, f.resolver, orders, clk, testPolicy())
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	look := application.SecondLook{}
	if wired {
		look = application.SecondLook{
			Settle: 2 * time.Second,
			Wait: func(d time.Duration) {
				f.waits = append(f.waits, d)
				if f.onWait != nil {
					f.onWait()
				}
			},
		}
	}
	// No reinstatement: these fixtures journal no generation, which is the
	// deployment shape the phase is optional FOR. The gate still reports —
	// OutcomePassed over an empty log — so the spot bracket's assertions are made
	// against a checklist that is complete rather than one missing a required
	// gate.
	r, err := application.NewReconciler(replayer, joiner, resolver, nil, look)
	if err != nil {
		t.Fatalf("NewReconciler: %v", err)
	}
	f.reconciler = r
	return f
}

// spotDeltaBoot is the bracket's entry condition: one filled spot buy of 0.5,
// a wallet of 1.0 against a baseline of 0.3 — the first look computes ours
// 0.5, observed 0.7, a spot delta of 0.2 and nothing else holding.
func spotDeltaBoot(t *testing.T, wired bool) (*spotReconcileFixture, domain.Order) {
	t.Helper()
	buy := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t, balanceRow(t, "BTC", "1"))
	leg := application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	}
	f := newSpotReconcileFixture(t, leg, wired,
		replayScript(t, 1, buy, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.balances = balances
	f.joinVenue.status[buy.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[spot()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", buy.ClientOrderID, "V-1", "0.5")},
	}
	return f, buy
}

// TestReconcileASpotDeltaStableAcrossTheSecondLookLatches is §70h item 23's
// second half: the SAME delta a settle bound apart is a real finding —
// OutcomeNeedsOperator, latched — and the wait provably ran once with the
// configured bound.
func TestReconcileASpotDeltaStableAcrossTheSecondLookLatches(t *testing.T) {
	t.Parallel()

	f, _ := spotDeltaBoot(t, true)
	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeNeedsOperator", o)
	}
	if !latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("a stable spot delta must LATCH — a retry answers the same way")
	}
	if len(f.waits) != 1 || f.waits[0] != 2*time.Second {
		t.Errorf("waits = %v, want exactly [2s] — one bracket, the configured bound", f.waits)
	}
	if len(got.Assertion.SpotUnexplained) != 1 {
		t.Fatalf("SpotUnexplained = %+v, want the stable BTC delta", got.Assertion.SpotUnexplained)
	}
	spotDeltaEquals(t, got.Assertion.SpotUnexplained[0], "BTC", "0.5", "0.7", "0.2")
}

// TestReconcileAnOrderNewlyAbsentInTheSecondLookIsIncomplete is §70i: an order
// the venue held at the first look and does not hold inside the bracket
// changed evidence the ordering (join → resolve → assert) says only §5.3 may
// interpret — and the bracket does not run the Resolver, so the re-assert
// would read a projection that is not concluded. The outcome is Incomplete:
// the next boot re-derives everything, resolution included.
func TestReconcileAnOrderNewlyAbsentInTheSecondLookIsIncomplete(t *testing.T) {
	t.Parallel()

	f, buy := spotDeltaBoot(t, true)
	f.onWait = func() {
		f.joinVenue.status[buy.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	}
	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeIncomplete {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeIncomplete — the evidence moved mid-bracket", o)
	}
	if latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("a mid-bracket disappearance latched — that is a delta computed over an unresolved absence")
	}
	if len(f.waits) != 1 {
		t.Errorf("waits = %v, want exactly one bracket", f.waits)
	}
}

// TestReconcileAnAbsenceInBothLooksStillLatchesAStableDelta is the §70i
// control: an order absent at BOTH looks was resolved through §5.3 before the
// first assert ever ran, and it re-reports absent on every boot for as long
// as the venue has purged it — so it is NOT motion, and treating it as motion
// would leave any deployment holding one AbsentAfterFinality order unable to
// latch a real spot delta, for ever. The stable delta must still latch over
// it.
func TestReconcileAnAbsenceInBothLooksStillLatchesAStableDelta(t *testing.T) {
	t.Parallel()

	// The delta source: a filled spot buy of 0.5 against wallet 1.0 and
	// baseline 0.3 (spotDeltaBoot's arithmetic, restated because this fixture
	// needs a SECOND order in the journal).
	buy := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	// The stably-absent order: journaled to SubmissionUnknown an hour ago, so
	// its landing horizon has long passed and §5.3 can conclude the absence.
	gone := joinOrder(t, 2, spot(), domain.SideSell, "0.1")
	anchor := joinNow.Add(-time.Hour)
	at, err := clock.NewServerTime(anchor, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}

	balances := newFakeSpotBalances()
	balances.reports["binance"] = balanceReport(t, balanceRow(t, "BTC", "1"))
	leg := application.SpotLeg{
		Balances:  balances,
		Roster:    spotRoster(),
		Baselines: []application.SpotBaseline{btcBaseline(t, "0.3")},
	}
	entries := append(
		replayScript(t, 1, buy, anchor, domain.StateFilled),
		replayScript(t, 4, gone, anchor, domain.StateSubmissionUnknown)...)
	f := newSpotReconcileFixture(t, leg, true, entries...)
	f.balances = balances
	f.joinVenue.status[buy.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[spot()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", buy.ClientOrderID, "V-1", "0.5")},
	}
	// `gone` is in the status map of neither look: the venue reports it
	// absent both times. The Resolver concludes the absence at the first pass
	// (stable not-found: absent from the resolving surface, a proven sweep
	// past the horizon, inside the published reach).
	f.resolver.status = absentVenue(publishedReach())
	f.resolver.sweeps = []ports.ExecutionSweep{provenSweep(horizonOf(at, testPolicy()))}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeNeedsOperator — a both-looks absence is not motion", o)
	}
	if !latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("the stable delta did not latch — the resolved absence masked it")
	}
	if len(f.waits) != 1 {
		t.Errorf("waits = %v, want exactly one bracket", f.waits)
	}
	if len(got.Assertion.SpotUnexplained) != 1 {
		t.Fatalf("SpotUnexplained = %+v, want the stable BTC delta", got.Assertion.SpotUnexplained)
	}
	spotDeltaEquals(t, got.Assertion.SpotUnexplained[0], "BTC", "0.5", "0.7", "0.2")
}

// TestReconcileASpotDeltaThatMovesIsIncomplete is §70h item 23's first half:
// a late fill's trade-list row arrives inside the bracket, the re-join admits
// it, and the re-assert's delta set CHANGES — the world is still settling, so
// the outcome is Incomplete and nothing latches. The one-look reading of the
// same boot would have latched a 0.2 incident that was half a fill in flight.
func TestReconcileASpotDeltaThatMovesIsIncomplete(t *testing.T) {
	t.Parallel()

	f, buy := spotDeltaBoot(t, true)
	f.onWait = func() {
		// The late execution reports during the settle window: the order's
		// cumulative moves 0.5 → 0.6, and the surface agrees.
		f.joinVenue.status[buy.ClientOrderID] = present("V-1", "0.6", t)
		f.joinVenue.sweeps[spot()] = ports.ExecutionSweep{
			Executions: []ports.VenueExecution{
				execution(t, "E-1", buy.ClientOrderID, "V-1", "0.5"),
				execution(t, "E-2", buy.ClientOrderID, "V-1", "0.1"),
			},
		}
	}
	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeIncomplete {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeIncomplete — the delta moved, so it is not a finding yet", o)
	}
	if latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("a moving delta latched — the second look exists to prevent exactly this incident")
	}
	// The report carries the SECOND look's assertion: ours 0.6 against an
	// observed 0.7 — the residue still unexplained after the late row landed.
	if len(got.Assertion.SpotUnexplained) != 1 {
		t.Fatalf("SpotUnexplained = %+v, want the re-asserted delta", got.Assertion.SpotUnexplained)
	}
	spotDeltaEquals(t, got.Assertion.SpotUnexplained[0], "BTC", "0.6", "0.7", "0.1")
}

// TestReconcileASecondLookThatExplainsTheDeltaPasses is the bracket's positive
// control: the late fill fully explains the first look's delta, and the boot
// passes rather than latching — the exact incident §70c's one-look reading
// would have minted.
func TestReconcileASecondLookThatExplainsTheDeltaPasses(t *testing.T) {
	t.Parallel()

	f, buy := spotDeltaBoot(t, true)
	f.onWait = func() {
		f.joinVenue.status[buy.ClientOrderID] = present("V-1", "0.7", t)
		f.joinVenue.sweeps[spot()] = ports.ExecutionSweep{
			Executions: []ports.VenueExecution{
				execution(t, "E-1", buy.ClientOrderID, "V-1", "0.5"),
				execution(t, "E-2", buy.ClientOrderID, "V-1", "0.2"),
			},
		}
	}
	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomePassed {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomePassed — the re-join explained the whole delta", o)
	}
	if p := c.Posture(); p != boot.PostureTrading {
		t.Errorf("Posture() = %s, want PostureTrading — %v is blocking", p, c.Blocking())
	}
}

// TestReconcileAnUnwiredBracketMeetingASpotDeltaFailsByName: a deployment that
// left the bracket unwired and then MET the one condition it exists for must
// not latch on one look and must not pass — it FAILS naming the wiring, which
// is an operator conversation about configuration, not about the wallet.
func TestReconcileAnUnwiredBracketMeetingASpotDeltaFailsByName(t *testing.T) {
	t.Parallel()

	f, _ := spotDeltaBoot(t, false)
	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeFailed {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeFailed", o)
	}
	if !errors.Is(got.Cause, application.ErrSecondLookUnwired) {
		t.Errorf("Cause = %v, want ErrSecondLookUnwired", got.Cause)
	}
}

// TestReconcileUnpinnedIsIncompleteAndNeverMasksALatch is §70h item 14: the
// precedence pair on one harness. An unpinned baselined asset ALONE is
// OutcomeIncomplete — re-reportable work, no latch, and the second look never
// runs because nothing holds. The same unpinned row BESIDE a stable spot
// delta changes nothing about the verdict the delta earns: NeedsOperator,
// latched — the incomplete finding must never mask the one that needs a
// human.
func TestReconcileUnpinnedIsIncompleteAndNeverMasksALatch(t *testing.T) {
	t.Parallel()

	twoAssetLeg := func(t *testing.T, report ports.BalanceReport) application.SpotLeg {
		t.Helper()
		balances := newFakeSpotBalances()
		balances.reports["binance"] = report
		return application.SpotLeg{
			Balances: balances,
			Roster: []application.SpotInstrument{
				{Instrument: spot(), Base: "BTC", Quote: "USDT"},
				{Instrument: ethSpot(), Base: "ETH", Quote: "USDT"},
			},
			Baselines: []application.SpotBaseline{
				btcBaseline(t, "0.3"),
				{Venue: "binance", Asset: "ETH", Amount: assetQty(t, "ETH", "1")},
			},
		}
	}

	t.Run("unpinned alone is incomplete, and the bracket never runs", func(t *testing.T) {
		t.Parallel()
		// BTC is clean (balance == baseline, no orders); ETH is unpinned.
		report := balanceReport(t, balanceRow(t, "BTC", "0.3"))
		report.Unpinned = []ports.UnpinnedBalance{{Asset: "ETH", Reason: ports.UnpinnedLockedSplitUnverified}}
		f := newSpotReconcileFixture(t, twoAssetLeg(t, report), true)

		c := bootChecklist(t)
		got, err := f.reconciler.Reconcile(c)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeIncomplete {
			t.Errorf("GateNetDeltaAsserted = %s, want OutcomeIncomplete", o)
		}
		if latched(t, c, boot.GateNetDeltaAsserted) {
			t.Error("an unpinned row latched — it is a timer, not an incident")
		}
		if len(f.waits) != 0 {
			t.Errorf("waits = %v, want none — nothing held, so there is nothing to bracket", f.waits)
		}
		want := application.UnconcludedBalance{Venue: "binance", Asset: "ETH", Reason: ports.UnpinnedLockedSplitUnverified}
		if len(got.Assertion.SpotUnconcluded) != 1 || got.Assertion.SpotUnconcluded[0] != want {
			t.Errorf("SpotUnconcluded = %+v, want [%+v]", got.Assertion.SpotUnconcluded, want)
		}
	})

	t.Run("a stable delta beside the unpinned row still latches", func(t *testing.T) {
		t.Parallel()
		// BTC carries a surplus of 0.2; ETH is unpinned on the same read.
		report := balanceReport(t, balanceRow(t, "BTC", "0.5"))
		report.Unpinned = []ports.UnpinnedBalance{{Asset: "ETH", Reason: ports.UnpinnedLockedSplitUnverified}}
		f := newSpotReconcileFixture(t, twoAssetLeg(t, report), true)

		c := bootChecklist(t)
		if _, err := f.reconciler.Reconcile(c); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeNeedsOperator {
			t.Errorf("GateNetDeltaAsserted = %s, want OutcomeNeedsOperator — the delta outranks the timer", o)
		}
		if !latched(t, c, boot.GateNetDeltaAsserted) {
			t.Error("the combined boot did not latch — the unpinned row masked the delta")
		}
	})
}
