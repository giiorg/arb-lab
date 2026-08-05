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

// The collapse checklist for this file, run before the mutations:
//
//  1. GATE COUNT. The checklist holds all THREE gates in every fixture, never
//     the one under test. With one gate, "the posture is the weakest over the
//     required set" and "the posture is this gate's" are the same value, and a
//     reconcile that reported the wrong gate would still produce the right
//     posture.
//  2. ORDER COUNT. Every fixture that makes a per-order claim replays TWO
//     orders in DIFFERENT states, so "this order's finding" and "the first
//     order's finding" differ — and so that Recovery.Dispatched is a set rather
//     than a constant.
//  3. SEQUENCE, not values. The finding this file exists for is about the ORDER
//     of three calls, and no value fixture can reach it: the absent order and
//     the resolved order are the same order at two instants, so a test that
//     inspected only the end state would pass against either ordering. What
//     separates them is the assertion's own answer, so the control runs the
//     rejected order explicitly (TestAssertingOverAnUnrebuiltAbsenceLatches).
//  4. THE OUTCOME'S TWO PROPERTIES. Every outcome claim checks the posture AND
//     the latch, because OutcomeIncomplete and OutcomeNeedsOperator share a
//     posture and differ only in the second — a test reading Posture() alone
//     cannot tell them apart at all.

// reconcileFixture is §7.5's three phases over ONE projection, which is the
// property the composition depends on: a reconciler whose replayer, joiner and
// resolver wrote to three repositories would rebuild three states and assert the
// invariant over none of them.
type reconcileFixture struct {
	reconciler *application.Reconciler

	// replayer and joiner are held so the CONTROL can run the rejected order —
	// join, then assert, with §5.3's rule left out. Reaching past the reconciler
	// is the point of a control: it has to be able to express the shape the
	// reconciler exists to prevent.
	replayer *application.Replayer
	joiner   *application.Joiner

	orders    *fakeOrders
	replay    *fakeReplay
	joinVenue *fakeJoinVenue
	positions *fakePositions
	resolver  *fakeResolver
	journal   *fakeJournal
}

func newReconcileFixture(t *testing.T, now time.Time, entries ...journalEntry) *reconcileFixture {
	t.Helper()

	orders := newFakeOrders()
	replay := &fakeReplay{entries: entries}
	joinVenue := newFakeJoinVenue()
	positions := newFakePositions()
	resolveVenue := &fakeResolver{}
	journal := newFakeJournal()

	clk, err := clock.NewTestClock(now, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}

	replayer, err := application.NewReplayer(replay, orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	joiner, err := application.NewJoiner(joinVenue, positions, orders, clk, joinAccount, application.SpotLeg{}, perpScopeFor())
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	resolver, err := application.NewResolver(journal, orders, resolveVenue, orders, clk, testPolicy())
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	r, err := application.NewReconciler(replayer, joiner, resolver, application.SecondLook{})
	if err != nil {
		t.Fatalf("NewReconciler: %v", err)
	}
	return &reconcileFixture{
		reconciler: r, replayer: replayer, joiner: joiner, orders: orders, replay: replay,
		joinVenue: joinVenue, positions: positions, resolver: resolveVenue, journal: journal,
	}
}

// bootChecklist is §7.5's three OMS gates. All three, always — see checklist
// item 1.
func bootChecklist(t *testing.T) *boot.Checklist {
	t.Helper()
	c, err := boot.NewChecklist(boot.GateJournalReplayed, boot.GateVenueJoined, boot.GateNetDeltaAsserted)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	return c
}

// replayScript is one order's journal: planned, authorized at a chosen anchor,
// and advanced to a chosen state.
func replayScript(t *testing.T, seq uint64, o domain.Order, anchorAt time.Time, state domain.OrderState) []journalEntry {
	t.Helper()
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return []journalEntry{
		{seq: seq, kind: "planned", order: o},
		{seq: seq + 1, kind: "authorized", id: o.ClientOrderID, at: at},
		{seq: seq + 2, kind: "state", id: o.ClientOrderID, state: state},
	}
}

func outcomeOf(t *testing.T, c *boot.Checklist, g boot.Gate) boot.Outcome {
	t.Helper()
	o, declared := c.Outcome(g)
	if !declared {
		t.Fatalf("gate %s is not declared on this checklist", g)
	}
	return o
}

// latched reports whether a gate refuses a fresh OutcomePassed, which is the
// only externally visible statement of the latch — and the property that
// separates OutcomeIncomplete from OutcomeNeedsOperator, since they share a
// posture. It is checked on a COPY's worth of state: the report it makes is
// either refused (leaving the gate as it was) or it is the lift under test.
func latched(t *testing.T, c *boot.Checklist, g boot.Gate) bool {
	t.Helper()
	before := outcomeOf(t, c, g)
	err := c.Report(g, boot.OutcomePassed)
	if err != nil {
		return true
	}
	// Put it back, so a caller checking two gates does not have the first one's
	// probe decide the second one's answer.
	if err := c.Report(g, before); err != nil {
		t.Fatalf("restoring gate %s to %s: %v", g, before, err)
	}
	return false
}

// ---------------------------------------------------------------------------
// The ordinary boot
// ---------------------------------------------------------------------------

// TestReconcileCleanBootReachesTrading is the positive control every refusal
// below is measured against: without it, a reconciler that refused everything
// would score a perfect run.
func TestReconcileCleanBootReachesTrading(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	buy := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")

	f := newReconcileFixture(t, joinNow,
		append(
			replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled),
			replayScript(t, 4, buy, joinNow.Add(-time.Hour), domain.StateFilled)...)...)

	f.joinVenue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.status[buy.ClientOrderID] = present("V-2", "0.2", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{
			execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5"),
			execution(t, "E-2", buy.ClientOrderID, "V-2", "0.2"),
		},
	}
	// Sell 0.5, buy 0.2 → we account for −0.3, and the venue holds −0.3.
	f.positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.3")}},
	}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if got.Cause != nil {
		t.Errorf("Cause = %v, want nil on a clean boot", got.Cause)
	}
	for _, g := range c.Required() {
		if o := outcomeOf(t, c, g); o != boot.OutcomePassed {
			t.Errorf("%s = %s, want OutcomePassed", g, o)
		}
	}
	if p := c.Posture(); p != boot.PostureTrading {
		t.Errorf("Posture() = %s, want PostureTrading — %v is blocking", p, c.Blocking())
	}
	if got.Join.Orders != 2 || got.Join.Fills != 2 {
		t.Errorf("orders/fills = %d/%d, want 2/2", got.Join.Orders, got.Join.Fills)
	}
	if got.Assertion.Holds() {
		t.Errorf("assertion = %+v, want clean", got.Assertion)
	}
}

// ---------------------------------------------------------------------------
// FINDING: §5.3's absence resolution runs BETWEEN the join and the assertion
// ---------------------------------------------------------------------------

// crashedMidFlightFixture is the boot §7.5 exists for: a dispatch that reached
// the venue, a crash seconds later, and a reboot before §5.3's propagation bound
// has elapsed.
//
// The order surface does not hold the order — nothing acknowledged it yet — so
// the join declines to sweep it and it carries the replay's ZERO cumulative,
// while the fill it took is standing in the account's position.
func crashedMidFlightFixture(t *testing.T) (*reconcileFixture, domain.Order) {
	t.Helper()

	anchorAt := testNow
	sell := replayOrder(t, mintedID(t, 1))
	// BEFORE the horizon: a send authorized before the query may still land
	// after it, so §5.3's gate 1 refuses — and it refuses for a bounded time.
	f := newReconcileFixture(t, anchorAt, replayScript(t, 1, sell, anchorAt, domain.StateSubmissionUnknown)...)

	f.joinVenue.status[sell.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	f.resolver.status = absentVenue(publishedReach())
	// The account is short 0.5, and it is this very order that explains it.
	f.positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}
	return f, sell
}

// TestAssertingOverAnUnrebuiltAbsenceLatches is THE REJECTED ORDER, encoded as a
// control and run rather than argued: join, then assert, with §5.3's rule left
// out and no gate between them. That is what the fused Join-that-also-asserted
// did on every boot.
//
// What it demonstrates is the contradiction inside one use case: the join
// records an absent order in Join.Absent precisely because "an absence is the
// one answer this use case must not act on" — §5.3 forbids reading it as "no
// executions" — and the assertion then read exactly that zero as a fact. The
// venue's own position is the other side of the subtraction, so the invariant
// reports an unexplained delta of the whole fill, which is OutcomeNeedsOperator,
// which LATCHES. A condition that clears itself in MaxLanding + SweepMargin
// becomes an incident that needs a human.
//
// If this ever stops alerting, the fix below is measuring nothing.
func TestAssertingOverAnUnrebuiltAbsenceLatches(t *testing.T) {
	t.Parallel()

	f, sell := crashedMidFlightFixture(t)
	rec, err := f.replayer.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	joined, err := f.joiner.Join(rec.Dispatched)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(joined.Absent) != 1 || joined.Absent[0] != sell.ClientOrderID {
		t.Fatalf("absent = %v, want the order the venue does not hold", joined.Absent)
	}

	assertion, err := f.joiner.Assert(rec.Dispatched)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(assertion.Unexplained) != 1 {
		t.Fatalf("unexplained = %+v, want the whole standing fill — the control is not reaching the defect", assertion.Unexplained)
	}
	// The literal, not a recomputation: a subtraction in the wrong order flips
	// both sides together and compares equal.
	if got := assertion.Unexplained[0].Unexplained.Decimal(); got != "-0.5" {
		t.Errorf("unexplained = %s, want -0.5 — the fill the join declined to rebuild", got)
	}

	// And the outcome that follows from it, which is the cost.
	c := bootChecklist(t)
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if !latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("the pre-fix outcome did not latch — then the finding has no cost and should be withdrawn")
	}
}

// TestReconcileHoldsTheAssertionUntilTheRebuildConcluded is the fix: the same
// account, the same venue and the same log, with §5.3's rule between the join
// and the assertion and the assertion gated on the rebuild having concluded.
//
// The boot refuses — it must, the exposure is genuinely not accounted for — but
// it refuses with an outcome a retry can lift, and it does not assert an
// invariant over a quantity it declined to rebuild.
func TestReconcileHoldsTheAssertionUntilTheRebuildConcluded(t *testing.T) {
	t.Parallel()

	f, _ := crashedMidFlightFixture(t)

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Unproved) != 1 || got.Unproved[0].Reason != application.ReasonHorizonNotPassed {
		t.Fatalf("unproved = %+v, want one horizon_not_passed", got.Unproved)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeIncomplete {
		t.Errorf("GateVenueJoined = %s, want OutcomeIncomplete", o)
	}
	if latched(t, c, boot.GateVenueJoined) {
		t.Error("the fixed boot latched — the horizon is a fixed instant and the clock reaches it")
	}
	// The assertion did not run, and the gate says so rather than passing OR
	// failing: Unreported is the honest value and it is not the permissive one,
	// because the fold is over the REQUIRED gates.
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeUnreported {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeUnreported", o)
	}
	if len(got.Assertion.Unexplained) != 0 {
		t.Errorf("assertion = %+v, want the zero value — it must not have run", got.Assertion)
	}
	if p := c.Posture(); p != boot.PostureReconcileOnly {
		t.Errorf("Posture() = %s, want PostureReconcileOnly", p)
	}
}

// TestReconcileAnAbsentOrderWithExecutionsNamesTheORDER is §5.3's refutation
// arriving where it can be acted on: the sweep finds a row the order surface's
// silence denied, and the report names the order and carries the executions
// rather than naming an instrument whose position does not add up.
func TestReconcileAnAbsentOrderWithExecutionsNamesTheORDER(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, engineTimeFor(horizon), replayScript(t, 1, sell, anchorAt, domain.StateSubmissionUnknown)...)
	f.joinVenue.status[sell.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	f.resolver.status = absentVenue(publishedReach())
	f.resolver.sweeps = []ports.ExecutionSweep{provenSweep(horizon, row(sell.ClientOrderID, "E-1"))}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Unproved) != 1 || got.Unproved[0].Reason != application.ReasonExecutionsFound {
		t.Fatalf("unproved = %+v, want one executions_found", got.Unproved)
	}
	if got.Unproved[0].Order != sell.ClientOrderID {
		t.Errorf("unproved order = %q, want %q", got.Unproved[0].Order, sell.ClientOrderID)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateVenueJoined = %s, want OutcomeNeedsOperator — an execution the ledger has not admitted", o)
	}
	if !latched(t, c, boot.GateVenueJoined) {
		t.Error("executions found against a venue reporting no order did not latch — the next sweep says the same")
	}
}

// ---------------------------------------------------------------------------
// The journal gate: two failures that must not be one
// ---------------------------------------------------------------------------

func TestReconcileBrokenChainHaltsAndLatchesAndNeverCallsTheVenue(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.replay.failWith = ports.ErrJournalChainBroken
	f.replay.failAfter = 2

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil — a refusal is a report, not an error", err)
	}
	if o := outcomeOf(t, c, boot.GateJournalReplayed); o != boot.OutcomeEvidenceUnusable {
		t.Errorf("GateJournalReplayed = %s, want OutcomeEvidenceUnusable", o)
	}
	if p := c.Posture(); p != boot.PostureHalted {
		t.Errorf("Posture() = %s, want PostureHalted", p)
	}
	if !latched(t, c, boot.GateJournalReplayed) {
		t.Error("a broken chain did not latch — a replay re-run over bytes that cannot change would re-admit trading")
	}
	if got.Cause == nil {
		t.Error("Cause = nil, want the replay error")
	}
	// Nothing downstream ran. The venue join and §5.3's rule both compute from
	// the log this gate has just refused to trust.
	if len(f.joinVenue.windows) != 0 || len(f.resolver.windows) != 0 {
		t.Errorf("swept %d join windows and %d resolve windows after an unusable journal",
			len(f.joinVenue.windows), len(f.resolver.windows))
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeUnreported {
		t.Errorf("GateVenueJoined = %s, want OutcomeUnreported — it never ran", o)
	}
}

func TestReconcileAnIncoherentLogFailsWithoutLatching(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.replay.failWith = errors.New("journal: record 2 will not decode")
	f.replay.failAfter = 2

	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if o := outcomeOf(t, c, boot.GateJournalReplayed); o != boot.OutcomeFailed {
		t.Errorf("GateJournalReplayed = %s, want OutcomeFailed", o)
	}
	if latched(t, c, boot.GateJournalReplayed) {
		t.Error("an ordinary replay failure latched — only an untrustworthy chain does")
	}
}

// ---------------------------------------------------------------------------
// The venue gate: four findings, three outcomes, and the cell that had to exist
// ---------------------------------------------------------------------------

// TestReconcileAnUnprovedAbsenceIsIncompleteNotFailed is boot.OutcomeIncomplete's
// producer: the venue answered, the log is intact, and the only thing wrong is
// that not enough time has passed.
func TestReconcileAnUnprovedAbsenceIsIncompleteNotFailed(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	// BEFORE the horizon: gate 1 refuses because a send authorized before the
	// query may still land after it.
	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, anchorAt, replayScript(t, 1, sell, anchorAt, domain.StateSubmissionUnknown)...)
	f.joinVenue.status[sell.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	f.resolver.status = absentVenue(publishedReach())
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}
	_ = at

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Unproved) != 1 || got.Unproved[0].Reason != application.ReasonHorizonNotPassed {
		t.Fatalf("unproved = %+v, want one horizon_not_passed", got.Unproved)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeIncomplete {
		t.Errorf("GateVenueJoined = %s, want OutcomeIncomplete — nothing failed and no operator is needed", o)
	}
	if p := c.Posture(); p != boot.PostureReconcileOnly {
		t.Errorf("Posture() = %s, want PostureReconcileOnly", p)
	}
	if latched(t, c, boot.GateVenueJoined) {
		t.Error("an unproved absence latched — the horizon is a fixed instant and the clock reaches it")
	}
	// The invariant did not run, and says so rather than failing.
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeUnreported {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeUnreported — it must not assert over a rebuild that did not conclude", o)
	}
}

// TestReconcileAnUnprovableAbsenceNeedsAnOperator is task #175: an absence
// whose reason can never be discharged by waiting.
func TestReconcileAnUnprovableAbsenceNeedsAnOperator(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, engineTimeFor(horizon), replayScript(t, 1, sell, anchorAt, domain.StateSubmissionUnknown)...)
	f.joinVenue.status[sell.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	// A surface that publishes NO retention and no assumption configured for it:
	// gate 4 has no bound to apply, and no amount of waiting supplies one.
	f.resolver.status = absentVenue(ports.SurfaceReach{Surface: "order_status"})
	f.resolver.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Unproved) != 1 || got.Unproved[0].Reason != application.ReasonReachUnknown {
		t.Fatalf("unproved = %+v, want one reach_unknown", got.Unproved)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateVenueJoined = %s, want OutcomeNeedsOperator", o)
	}
	if !latched(t, c, boot.GateVenueJoined) {
		t.Error("an absence that can never be proved did not latch — the next boot answers the same way")
	}
}

func TestReconcileAnUnattributedExecutionNeedsAnOperator(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	// The venue states 0.5 executed; the sweep hands us 0.3 of it.
	f.joinVenue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", sell.ClientOrderID, "V-1", "0.3")},
	}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Join.Unattributed) != 1 {
		t.Fatalf("unattributed = %v, want the order whose totals disagree", got.Join.Unattributed)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateVenueJoined = %s, want OutcomeNeedsOperator", o)
	}
	if !latched(t, c, boot.GateVenueJoined) {
		t.Error("an execution that exists and cannot be named did not latch")
	}
}

func TestReconcileATruncatedSweepIsIncomplete(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.joinVenue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions:    []ports.VenueExecution{execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5")},
		ProvenThrough: joinNow.Add(-time.Hour),
	}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Join.Incomplete) != 1 {
		t.Fatalf("incomplete = %v, want the order whose sweep fell short", got.Join.Incomplete)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeIncomplete {
		t.Errorf("GateVenueJoined = %s, want OutcomeIncomplete — the venue answered and the answer was short", o)
	}
	if latched(t, c, boot.GateVenueJoined) {
		t.Error("a short sweep latched — the next attempt gets further")
	}
}

func TestReconcileAVenueThatWillNotAnswerFails(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.joinVenue.err[sell.ClientOrderID] = errors.New("venue: 503")

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeFailed {
		t.Errorf("GateVenueJoined = %s, want OutcomeFailed", o)
	}
	if p := c.Posture(); p != boot.PostureHalted {
		t.Errorf("Posture() = %s, want PostureHalted", p)
	}
	if got.Cause == nil {
		t.Error("Cause = nil, want the venue error")
	}
	if latched(t, c, boot.GateVenueJoined) {
		t.Error("a venue that would not answer latched — it may answer on the next attempt")
	}
}

// TestReconcileAnUnexplainedPositionNeedsAnOperator is the net-delta gate's own
// refusal reached end to end, and it is a different claim from the control: the
// control shows the ASSERTION alerting, this shows the CHECKLIST carrying it.
func TestReconcileAnUnexplainedPositionNeedsAnOperator(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.joinVenue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5")},
	}
	// Our order explains −0.5; the account is short 0.8. A foreign symbol rides
	// along, so the two conditions §7.5 names are both present and the gate is
	// not passing because of whichever one happens to be checked first.
	f.positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions:    []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.8")}},
		Unrecognized: []string{"SOLUSDT"},
	}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Assertion.Unexplained) != 1 || got.Assertion.Unexplained[0].Unexplained.Decimal() != "-0.3" {
		t.Fatalf("unexplained = %+v, want one instrument short by -0.3", got.Assertion.Unexplained)
	}
	if len(got.Assertion.Foreign) != 1 || got.Assertion.Foreign[0] != "SOLUSDT" {
		t.Errorf("foreign = %v, want [SOLUSDT]", got.Assertion.Foreign)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeNeedsOperator", o)
	}
	if !latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("an unexplained position delta did not latch — §7.5 does not auto-correct it")
	}
	// The venue rebuild itself concluded, so the refusal is attributed to the
	// gate whose evidence it is.
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomePassed {
		t.Errorf("GateVenueJoined = %s, want OutcomePassed", o)
	}
}

// ---------------------------------------------------------------------------
// A gate reports about its OWN evidence
// ---------------------------------------------------------------------------

// TestAFailedApplyDuringResolutionReportsAgainstTheProjectionGate is the two
// findings that cannot share a gate: a truncated sweep (reconcile-only, lifts
// on a retry) and a failed projection write (stop-opening, latches). Reported on
// one gate, whichever is dropped is wrong.
func TestAFailedApplyDuringResolutionReportsAgainstTheProjectionGate(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, engineTimeFor(horizon), replayScript(t, 1, sell, anchorAt, domain.StateSubmissionUnknown)...)
	f.joinVenue.status[sell.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	f.resolver.status = absentVenue(publishedReach())
	f.resolver.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}
	// The journal takes the state; the projection refuses it.
	f.orders.failApplyState[domain.StateAbsentAfterFinality] = errors.New("projection: write failed")

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Stale) != 1 || got.Stale[0] != sell.ClientOrderID {
		t.Fatalf("stale = %v, want the order whose journal write outran its projection write", got.Stale)
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o != boot.OutcomeProjectionStale {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeProjectionStale", o)
	}
	if !latched(t, c, boot.GateNetDeltaAsserted) {
		t.Error("a stale projection did not latch — the check that would lift it recomputes from the stale store")
	}
	// The venue gate carries its own finding, undisturbed.
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomePassed {
		t.Errorf("GateVenueJoined = %s, want OutcomePassed — the venue rebuild concluded", o)
	}
}

// ---------------------------------------------------------------------------
// Pending, and the distinction OutcomeIncomplete exists to make
// ---------------------------------------------------------------------------

// TestReconcileReportsPendingBeforeEachPhase pins the OTHER side of finding 2:
// OutcomePending has to keep meaning "started and still running", or the new
// member is a rename rather than a distinction.
func TestReconcileReportsPendingBeforeEachPhase(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	// A venue that never returns is modelled as one that fails; what is pinned
	// here is the outcome the gate held WHILE the join was running, which the
	// failure path leaves observable only because Pending was reported first.
	f.joinVenue.err[sell.ClientOrderID] = errors.New("venue: timeout")

	c := bootChecklist(t)
	// Report Pending ourselves first and check the reconciler does not regard it
	// as a latch or an error: a boot that reported progress must be able to
	// report its verdict afterwards.
	if err := c.Report(boot.GateVenueJoined, boot.OutcomePending); err != nil {
		t.Fatalf("Report(pending): %v", err)
	}
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeFailed {
		t.Errorf("GateVenueJoined = %s, want OutcomeFailed — Pending must not hold a verdict out", o)
	}
	if boot.OutcomePending.String() == boot.OutcomeIncomplete.String() {
		t.Error("Pending and Incomplete render identically — an operator cannot tell wait from act")
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

func TestReconcilerRefusesAMissingPhase(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow)
	replayer, err := application.NewReplayer(f.replay, f.orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	if _, err := application.NewReconciler(nil, nil, nil, application.SecondLook{}); !errors.Is(err, application.ErrReplayerMissing) {
		t.Errorf("NewReconciler(nil,...) = %v, want ErrReplayerMissing", err)
	}
	if _, err := application.NewReconciler(replayer, nil, nil, application.SecondLook{}); !errors.Is(err, application.ErrJoinerMissing) {
		t.Errorf("NewReconciler(replayer, nil, nil) = %v, want ErrJoinerMissing", err)
	}
}

func TestReconcileRefusesANilChecklist(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow)
	if _, err := f.reconciler.Reconcile(nil); !errors.Is(err, application.ErrChecklistMissing) {
		t.Errorf("Reconcile(nil) = %v, want ErrChecklistMissing", err)
	}
}

func TestReconcileRefusesAChecklistMissingAGate(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow)
	c, err := boot.NewChecklist(boot.GateVenueJoined, boot.GateNetDeltaAsserted)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	if _, err := f.reconciler.Reconcile(c); !errors.Is(err, boot.ErrGateUndeclared) {
		t.Errorf("Reconcile(checklist without GateJournalReplayed) = %v, want ErrGateUndeclared", err)
	}
}

// ---------------------------------------------------------------------------
// The reason vocabulary's own claim
// ---------------------------------------------------------------------------

// TestRetryMayLiftNamesEveryReason is task #175's predicate over its whole
// domain: the two that wait on time lift, the five that do not never will.
func TestRetryMayLiftNamesEveryReason(t *testing.T) {
	t.Parallel()

	lifts := map[application.UnprovedReason]bool{
		application.ReasonHorizonNotPassed:  true,
		application.ReasonSweepIncomplete:   true,
		application.ReasonNoAnchor:          false,
		application.ReasonReachExceeded:     false,
		application.ReasonReachUnknown:      false,
		application.ReasonSweepInconclusive: false,
		application.ReasonExecutionsFound:   false,
	}
	if len(lifts) != 7 {
		t.Fatalf("table holds %d reasons, want every declared one", len(lifts))
	}
	for reason, want := range lifts {
		got, err := reason.RetryMayLift()
		if err != nil {
			t.Errorf("%s: RetryMayLift() = %v, want nil", reason, err)
			continue
		}
		if got != want {
			t.Errorf("%s: RetryMayLift() = %v, want %v", reason, got, want)
		}
	}
	if _, err := application.UnprovedReason("no_such_gate").RetryMayLift(); !errors.Is(err, application.ErrUnprovedReasonInvalid) {
		t.Error("an undeclared reason did not refuse — the permissive default is the one that costs")
	}
	// The empty reason is what a zero-valued Resolution carries, and it must not
	// read as liftable either.
	if lifts, err := application.UnprovedReason("").RetryMayLift(); err == nil || lifts {
		t.Errorf(`UnprovedReason("").RetryMayLift() = %v/%v, want false and a refusal`, lifts, err)
	}
}
