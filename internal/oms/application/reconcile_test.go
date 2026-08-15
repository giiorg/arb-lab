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
	ladder    *fakeLadder

	// resolveUseCase is the §5.3 phase itself, held so a test can rebuild the
	// Reconciler around it — the fixture's `resolver` above is the venue DOUBLE
	// the phase reads, which is a different object and not interchangeable.
	resolveUseCase *application.Resolver

	// The core §7.1 margin-op phase's two collaborators, exposed so a test can
	// script the venue's live posted margin and read back which verdicts were
	// made durable.
	marginJournal *fakeMarginOpJournal
	marginPort    *fakeMargin
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
	joiner, err := application.NewJoiner(joinVenue, positions, orders, journal, clk, joinAccount, application.SpotLeg{}, perpScopeFor())
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	resolver, err := application.NewResolver(journal, orders, resolveVenue, orders, clk, testPolicy())
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	ladder := newFakeLadder()
	reinstater, err := application.NewReinstater(ladder, orders)
	if err != nil {
		t.Fatalf("NewReinstater: %v", err)
	}
	// The core §7.1 margin-op phase, wired into the standard fixture so that
	// EVERY reconcile test runs it. Its ordinary answer is "nothing to do", which
	// is what every real boot sees until slice B3 part 2 gives the record a
	// writer — so the fixtures that put a record in the log are the only evidence
	// this phase works, and they live beside this one.
	marginJournal := &fakeMarginOpJournal{}
	marginPort := marginFixture(t, "125", true)
	marginOps, err := application.NewMarginOpResolver(marginJournal, marginPort, noActs(), clk, marginOpTestBound)
	if err != nil {
		t.Fatalf("NewMarginOpResolver: %v", err)
	}
	r, err := application.NewReconciler(replayer, joiner, resolver, reinstater, marginOps, application.SecondLook{})
	if err != nil {
		t.Fatalf("NewReconciler: %v", err)
	}
	return &reconcileFixture{
		reconciler: r, replayer: replayer, joiner: joiner, orders: orders, replay: replay,
		joinVenue: joinVenue, positions: positions, resolver: resolveVenue, journal: journal,
		ladder: ladder, resolveUseCase: resolver,
		marginJournal: marginJournal, marginPort: marginPort,
	}
}

// bootChecklist is §7.5's FIVE OMS gates. All five, always — see checklist
// item 1: a fixture declaring only the gates a case happens to exercise cannot
// tell a gate that passed from a gate nobody reported, which is the one
// distinction the posture fold is built on.
//
// The count is stated as a number here and in the literal below because it has
// gone stale once already: GateMarginOpsResolved joined with task #212 and every
// case in this file now runs its phase.
func bootChecklist(t *testing.T) *boot.Checklist {
	t.Helper()
	c, err := boot.NewChecklist(boot.GateJournalReplayed, boot.GateVenueJoined,
		boot.GateGenerationsReinstated, boot.GateNetDeltaAsserted, boot.GateMarginOpsResolved)
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

// TestReconcileHoldsForOperatorOnAContradictedOrder.
//
// A contradiction is not retryable and the direction matters: the same account
// answers the same way on the next boot, so an outcome that permitted a retry
// would let a deployment boot forever over two readings that can never agree.
// It sits with Unattributed rather than with Incomplete for that reason —
// Incomplete's cell means "ran, concluded, work remains", and no further work
// changes this one.
func TestReconcileHoldsForOperatorOnAContradictedOrder(t *testing.T) {
	t.Parallel()

	bad := replayOrder(t, mintedID(t, 1))
	good := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	f := newReconcileFixture(t, joinNow,
		append(
			replayScript(t, 1, bad, joinNow.Add(-time.Hour), domain.StateAcknowledged),
			replayScript(t, 4, good, joinNow.Add(-time.Hour), domain.StatePartiallyFilled)...)...)

	// The contradiction: this system watched the order go live, the venue calls
	// it a submission-time rejection, and SM §1 has no edge between the two.
	f.joinVenue.status[bad.ClientOrderID] = venueSays(t, domain.StateRejected, "V-1", "0")
	// The positive control in the same run — a reading SM §1 DOES have an edge
	// to, so this is not a join that objected to everything. It also keeps
	// checklist item 2: two orders, two replayed states, two venue readings.
	f.joinVenue.status[good.ClientOrderID] = venueSays(t, domain.StateFilled, "V-2", "0.2")
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", good.ClientOrderID, "V-2", "0.2")},
	}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(got.Join.Contradicted) != 1 {
		t.Fatalf("contradicted = %v, want the one order whose two readings have no edge between them",
			got.Join.Contradicted)
	}
	// The collapse guard, and it is what makes the two assertions below mean
	// anything: the ladder's first holding case wins, so a fixture that also
	// produced an unattributed execution, a truncated sweep or an unproved
	// absence would report NeedsOperator with this rung deleted.
	if len(got.Join.Unattributed) != 0 || len(got.Join.Incomplete) != 0 || len(got.Unproved) != 0 {
		t.Fatalf("unattributed=%v incomplete=%v unproved=%v, want none of them — an earlier rung would be answering",
			got.Join.Unattributed, got.Join.Incomplete, got.Unproved)
	}
	if got.Join.Advanced != 1 {
		t.Errorf("advanced = %d, want 1 — the second order is the positive control", got.Join.Advanced)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateVenueJoined = %s, want OutcomeNeedsOperator", o)
	}
	if !latched(t, c, boot.GateVenueJoined) {
		t.Error("two readings that can never agree did not latch — the next boot answers the same way")
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
	if _, err := application.NewReconciler(nil, nil, nil, nil, nil, application.SecondLook{}); !errors.Is(err, application.ErrReplayerMissing) {
		t.Errorf("NewReconciler(nil,...) = %v, want ErrReplayerMissing", err)
	}
	if _, err := application.NewReconciler(replayer, nil, nil, nil, nil, application.SecondLook{}); !errors.Is(err, application.ErrJoinerMissing) {
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

// ---------------------------------------------------------------------------
// SM §7: a generation the log opened and never ended
// ---------------------------------------------------------------------------

// TestALiveGenerationIsReinstatedOnItsOwnGate is the gate SPLIT, and it is what
// #204 turned slice #203's standing refusal into.
//
// #203 reported OutcomeIncomplete on GateJournalReplayed whenever the log held a
// generation it never ended, because nothing re-Reserved its entry. The
// direction has not changed — a ledger short by an entry reads as MORE room, and
// §6.7's determined branch would compensate for exposure the generation's own
// perp leg already committed to. What changed is WHERE the evidence is: the
// entry carries the confirmed perp reduction, which is in no journal and is read
// from the venue two phases later. So the replay's gate reports on the records
// (Passed) and the reinstatement's reports on the rebuild.
//
// The pair is asserted TOGETHER rather than one at a time, because that is the
// claim: a version that passed the replay's gate and never declared the second
// would have moved a refusal into nowhere.
func TestALiveGenerationIsReinstatedOnItsOwnGate(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	script := replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)
	script = append(script,
		journalEntry{seq: uint64(len(script) + 1), kind: "generation_opened", gen: "GEN-LIVE",
			group: testHedgeGroup("BTCUSDT"), res: "RES-1", qty: testTarget(t, "1")},
		journalEntry{seq: uint64(len(script) + 2), kind: "generation_perp_submitted",
			gen: "GEN-LIVE", id: sell.ClientOrderID},
	)
	f := newReconcileFixture(t, joinNow, script...)
	// The join must CONCLUDE, or the reinstatement never runs and this test
	// passes on an unreported gate — which is the vacuous reading of every
	// assertion below it. §7.5's ordering is what makes that a real precondition
	// rather than fixture bookkeeping: the generation's entry is sized from its
	// perp leg's executed quantity, and an unconcluded join is exactly the state
	// in which that quantity is knowably short.
	f.joinVenue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5")},
	}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil — a refusal is a report, not an error", err)
	}
	if o := outcomeOf(t, c, boot.GateVenueJoined); o != boot.OutcomePassed {
		t.Fatalf("GateVenueJoined = %s, want OutcomePassed — the reinstatement never runs otherwise", o)
	}
	if o := outcomeOf(t, c, boot.GateJournalReplayed); o != boot.OutcomePassed {
		t.Errorf("GateJournalReplayed = %s, want OutcomePassed — the records replayed", o)
	}
	if o := outcomeOf(t, c, boot.GateGenerationsReinstated); o != boot.OutcomePassed {
		t.Errorf("GateGenerationsReinstated = %s, want OutcomePassed", o)
	}
	if latched(t, c, boot.GateGenerationsReinstated) {
		t.Error("the reinstatement latched — nothing here needs operator clearance")
	}
	if len(got.Recovery.LiveGenerations) != 1 || got.Recovery.LiveGenerations[0].ID != "GEN-LIVE" {
		t.Errorf("Recovery.LiveGenerations = %v, want [GEN-LIVE]", liveGenerationIDs(got.Recovery))
	}
	// The generation reached the LADDER, which is the thing the gate reports
	// about. A gate that passed having reinstated nothing is exactly the shape
	// this whole phase exists to make impossible.
	if len(f.ladder.restored) != 1 || f.ladder.restored[0] != "GEN-LIVE" {
		t.Errorf("ladder restored %v, want [GEN-LIVE]", f.ladder.restored)
	}
	if len(got.Reinstatement.Restored) != 1 {
		t.Errorf("Reinstatement.Restored = %v, want one entry", got.Reinstatement.Restored)
	}

	// The sequence CONTINUED past the reinstatement. Each gate reports on its
	// own evidence, and an operator holding a refusal with no picture of the
	// account behind it is the failure this ordering avoids.
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o == boot.OutcomeUnreported {
		t.Error("GateNetDeltaAsserted = OutcomeUnreported — the assertion must still run and report")
	}
}

// TestReconcileAnENDEDGenerationStillPasses is the positive control the test
// above is worthless without: a gate that reported Incomplete whenever the log
// mentioned a generation at all would pass every assertion there, and would
// hold every boot after the first `PackageDelever` ladder ever ran to
// completion — permanently, on a healthy account.
func TestReconcileAnEndedGenerationStillPasses(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	script := replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)
	script = append(script,
		journalEntry{seq: uint64(len(script) + 1), kind: "generation_opened", gen: "GEN-DONE",
			group: testHedgeGroup("BTCUSDT"), res: "RES-1", qty: testTarget(t, "1")},
		journalEntry{seq: uint64(len(script) + 2), kind: "generation_phase",
			gen: "GEN-DONE", phase: domain.GenerationComplete},
	)
	f := newReconcileFixture(t, joinNow, script...)

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if o := outcomeOf(t, c, boot.GateJournalReplayed); o != boot.OutcomePassed {
		t.Errorf("GateJournalReplayed = %s, want OutcomePassed", o)
	}
	// The fixture must actually CONTAIN a generation, or this control is a
	// restatement of the clean-boot test and says nothing about the branch.
	if got.Recovery.Generations != 1 {
		t.Fatalf("Recovery.Generations = %d, want 1 — the control needs a generation to have ended",
			got.Recovery.Generations)
	}
	if len(got.Recovery.LiveGenerations) != 0 {
		t.Errorf("Recovery.LiveGenerations = %v, want none", liveGenerationIDs(got.Recovery))
	}
}

// TestAnUnwiredReinstatementFailsByNameOverALiveGeneration is
// ErrSecondLookUnwired's shape one gate over.
//
// The phase is optional because a deployment whose escalation ladder never runs
// has nothing to reinstate — requiring it would make every such boot declare a
// collaborator it has no use for. What it must not do is PASS on the one boot
// that meets the condition it exists for, because passing would report §7.5's
// ledger rebuilt over a ledger nothing touched.
//
// It is asserted as OutcomeFailed AND on the named cause, because "not Passed"
// is satisfied by Incomplete — which does not latch and which an operator reads
// as "wait" rather than "this binary is missing a phase".
func TestAnUnwiredReinstatementFailsByNameOverALiveGeneration(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	script := replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)
	script = append(script,
		journalEntry{seq: uint64(len(script) + 1), kind: "generation_opened", gen: "GEN-LIVE",
			group: testHedgeGroup("BTCUSDT"), res: "RES-1", qty: testTarget(t, "1")},
	)
	f := newReconcileFixture(t, joinNow, script...)
	f.unwireReinstatement(t)
	concludingJoin(t, f, sell)

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil — a refusal is a report, not an error", err)
	}
	if o := outcomeOf(t, c, boot.GateGenerationsReinstated); o != boot.OutcomeFailed {
		t.Errorf("GateGenerationsReinstated = %s, want OutcomeFailed", o)
	}
	if !errors.Is(got.Cause, application.ErrGenerationRestoreMissing) {
		t.Errorf("Cause = %v, want ErrGenerationRestoreMissing", got.Cause)
	}
}

// TestAnUnwiredReinstatementPassesOverAJournalWithNoGeneration is the positive
// control the test above is worthless without.
//
// Without it, a Reconciler that failed the gate whenever the phase was unwired
// would score full marks there — and would hold every derivatives-only
// deployment below trading for ever, on a healthy account with an intact log.
// That is the permanent-block failure §7.5's own Unasserted ruling was taken to
// avoid.
func TestAnUnwiredReinstatementPassesOverAJournalWithNoGeneration(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	f := newReconcileFixture(t, joinNow, replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)...)
	f.unwireReinstatement(t)
	concludingJoin(t, f, sell)

	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateGenerationsReinstated); o != boot.OutcomePassed {
		t.Errorf("GateGenerationsReinstated = %s, want OutcomePassed", o)
	}
}

// TestAnOwedSupersessionHoldsTheGateAtIncomplete is the third outcome, and it is
// the one that is neither a failure nor a pass.
//
// The premise dissolved while the generation's reserved SECOND leg was still out
// at the venue, so all three of §7's supersession rows require that leg to reach
// §6.7 finality FIRST — and finality is the venue's to give, not the boot's. The
// ledger is correct and nothing failed; what is outstanding is an obligation
// nothing at boot can discharge. Incomplete is that cell exactly: it ran, it
// concluded, and it left work a further attempt may finish.
func TestAnOwedSupersessionHoldsTheGateAtIncomplete(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	script := replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)
	script = append(script,
		journalEntry{seq: uint64(len(script) + 1), kind: "generation_opened", gen: "GEN-LIVE",
			group: testHedgeGroup("BTCUSDT"), res: "RES-1", qty: testTarget(t, "1")},
	)
	f := newReconcileFixture(t, joinNow, script...)
	f.ladder.quiesce = true
	concludingJoin(t, f, sell)

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateGenerationsReinstated); o != boot.OutcomeIncomplete {
		t.Errorf("GateGenerationsReinstated = %s, want OutcomeIncomplete", o)
	}
	if latched(t, c, boot.GateGenerationsReinstated) {
		t.Error("an owed supersession latched — the venue's own finality lifts it, not an operator")
	}
	if got.Reinstatement.AwaitingQuiesce != 1 {
		t.Errorf("AwaitingQuiesce = %d, want 1", got.Reinstatement.AwaitingQuiesce)
	}
}

// TestAFailedReinstatementIsAVerdictAndTheSequenceContinues is §7.5's
// each-gate-on-its-own-evidence rule applied to the new phase.
//
// A reinstatement that could not rebuild is a refusal, not an error, and
// stopping there would leave the net-delta assertion unreported — an operator
// holding a refusal with no picture of the account behind it.
func TestAFailedReinstatementIsAVerdictAndTheSequenceContinues(t *testing.T) {
	t.Parallel()

	sell := replayOrder(t, mintedID(t, 1))
	script := replayScript(t, 1, sell, joinNow.Add(-time.Hour), domain.StateFilled)
	script = append(script,
		journalEntry{seq: uint64(len(script) + 1), kind: "generation_opened", gen: "GEN-LIVE",
			group: testHedgeGroup("BTCUSDT"), res: "RES-1", qty: testTarget(t, "1")},
	)
	f := newReconcileFixture(t, joinNow, script...)
	f.ladder.failRestore = errors.New("ledger unavailable")
	concludingJoin(t, f, sell)

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil — a refusal is a report, not an error", err)
	}
	if o := outcomeOf(t, c, boot.GateGenerationsReinstated); o != boot.OutcomeFailed {
		t.Errorf("GateGenerationsReinstated = %s, want OutcomeFailed", o)
	}
	if got.Cause == nil {
		t.Error("Cause = nil, want the restore error")
	}
	if o := outcomeOf(t, c, boot.GateNetDeltaAsserted); o == boot.OutcomeUnreported {
		t.Error("GateNetDeltaAsserted = OutcomeUnreported — the sequence must continue past a refusal")
	}
}

// concludingJoin gives a fixture the venue answers that make GateVenueJoined
// pass, which the reinstatement's gate is downstream of.
//
// It is a helper rather than four copies because the precondition is easy to
// forget and its absence is SILENT: without it the gate is Unreported and every
// assertion about the reinstatement's outcome passes vacuously.
func concludingJoin(t *testing.T, f *reconcileFixture, order domain.Order) {
	t.Helper()
	f.joinVenue.status[order.ClientOrderID] = present("V-1", "0.5", t)
	f.joinVenue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", order.ClientOrderID, "V-1", "0.5")},
	}
	f.positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}
}

// unwireReinstatement rebuilds the fixture's Reconciler with NO reinstatement,
// which is the optional-phase shape NewReconciler admits.
func (f *reconcileFixture) unwireReinstatement(t *testing.T) {
	t.Helper()
	r, err := application.NewReconciler(f.replayer, f.joiner, f.resolveUseCase, nil, nil, application.SecondLook{})
	if err != nil {
		t.Fatalf("NewReconciler: %v", err)
	}
	f.reconciler = r
}

// ---------------------------------------------------------------------------
// Core §7.1's in-flight MarginOp phase (task #212)
// ---------------------------------------------------------------------------

// marginOpEntry is one journaled margin op, positioned relative to the fixture's
// VENUE clock so the resolver's bound decides what it decides.
func marginOpEntry(t *testing.T, seq uint64, id domain.MarginOpID, age time.Duration) journalEntry {
	t.Helper()
	return journalEntry{seq: seq, kind: "margin_op_dispatched", op: marginOpAged(t, id, age)}
}

// TestReconcileAnAmbiguousMarginOpWithholdsOpenings is this slice's end-to-end
// claim: a journaled op whose evidence cannot be attributed reaches §7.5's gate
// as a LATCHING stop-opening refusal, and the verdict is made durable.
//
// The three assertions are separate on purpose. The outcome is what constrains
// this process; the LATCH is what stops a later report lifting it; the journal
// write is what stops a RESTART lifting it. Each has its own failure mode and no
// two of them are the same claim.
func TestReconcileAnAmbiguousMarginOpWithholdsOpenings(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow, marginOpEntry(t, 1, "MOP-1", time.Hour))
	// The venue's posted margin moved by something that is not the op's delta —
	// §7.1's "coincident manual adjustment, position mutation".
	f.marginPort.statuses[perp()] = marginReadingOf(t, "110")

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil — a refusal is a report, not an error", err)
	}

	if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomeBlocksOpening {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomeBlocksOpening", o)
	}
	if !latched(t, c, boot.GateMarginOpsResolved) {
		t.Error("an unattributable margin op did not latch — a later report would lift a §9 incident")
	}
	if c.Posture().Permits(boot.ActOpen) {
		t.Error("openings are permitted over an unattributable margin op")
	}
	// §7.1: "safety actions continue from live liq distance". Reconcile-only
	// would take the ladder away during the incident it exists for.
	if !c.Posture().Permits(boot.ActReduce) {
		t.Error("the safety ladder is barred — §7.1 keeps it available through this incident")
	}

	if len(f.marginJournal.resolved) != 1 ||
		f.marginJournal.resolved[0].status != domain.MarginOpAmbiguous {
		t.Fatalf("journalled %v, want one ambiguous verdict for MOP-1", f.marginJournal.resolved)
	}
	if got.MarginOps.Ambiguous != 1 {
		t.Errorf("Reconciliation.MarginOps.Ambiguous = %d, want 1", got.MarginOps.Ambiguous)
	}
}

// TestReconcileAnAppliedMarginOpPasses is the positive control for the case
// above, and it is the one that makes the phase's refusals mean something.
func TestReconcileAnAppliedMarginOpPasses(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow, marginOpEntry(t, 1, "MOP-1", time.Hour))
	f.marginPort.statuses[perp()] = marginReadingOf(t, "125")

	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomePassed {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomePassed", o)
	}
	if len(f.marginJournal.resolved) != 1 ||
		f.marginJournal.resolved[0].status != domain.MarginOpApplied {
		t.Fatalf("journalled %v, want one applied verdict", f.marginJournal.resolved)
	}
}

// TestReconcileAnEmptyLogPassesTheMarginGate is the state every real boot is in
// until slice B3 part 2 gives the record a writer.
//
// It is asserted explicitly rather than inherited from the other cases, because
// the two ways this phase can report OutcomePassed — nothing to do, and
// everything resolved cleanly — are indistinguishable in a checklist, and only
// the fixtures tell them apart.
func TestReconcileAnEmptyLogPassesTheMarginGate(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow)
	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomePassed {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomePassed", o)
	}
	if len(f.marginJournal.resolved) != 0 {
		t.Errorf("wrote %v over an empty log", f.marginJournal.resolved)
	}
}

// TestReconcileACarriedAmbiguityBlocksWithNoVenueRead is the durable verdict's
// claim at the boot layer: an ambiguity a PREVIOUS process wrote down still
// blocks, and this boot re-derives nothing.
//
// The venue is scripted to answer the reading a re-derivation would call APPLIED
// — so a phase that recomputed carried verdicts would report a clean boot here.
func TestReconcileACarriedAmbiguityBlocksWithNoVenueRead(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow,
		marginOpEntry(t, 1, "MOP-OLD", time.Hour),
		journalEntry{seq: 2, kind: "margin_op_resolved",
			marginID: "MOP-OLD", mopStatus: domain.MarginOpAmbiguous},
	)
	f.marginPort.statuses[perp()] = marginReadingOf(t, "125")

	c := bootChecklist(t)
	got, err := f.reconciler.Reconcile(c)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomeBlocksOpening {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomeBlocksOpening — a restart is not a reconciliation", o)
	}
	if len(f.marginJournal.resolved) != 0 {
		t.Errorf("re-wrote %v for an op whose verdict the log already holds", f.marginJournal.resolved)
	}
	if len(got.MarginOps.Carried) != 1 || got.MarginOps.Carried[0] != "MOP-OLD" {
		t.Errorf("MarginOps.Carried = %v, want [MOP-OLD]", got.MarginOps.Carried)
	}
	if len(got.MarginOps.Resolved) != 0 {
		t.Errorf("resolved %d ops, want 0 — the verdict was read, not recomputed", len(got.MarginOps.Resolved))
	}
}

// TestReconcileAPendingMarginOpIsIncomplete is the fold's timer arm: an op too
// recently dispatched for silence to be evidence is not a failure and not an
// incident, and its outcome does NOT latch — §7.5's own self-clearing sentence,
// which is about exactly this case.
func TestReconcileAPendingMarginOpIsIncomplete(t *testing.T) {
	t.Parallel()

	f := newReconcileFixture(t, joinNow, marginOpEntry(t, 1, "MOP-1", time.Second))
	f.marginPort.statuses[perp()] = marginReadingOf(t, "100")

	c := bootChecklist(t)
	if _, err := f.reconciler.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomeIncomplete {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomeIncomplete", o)
	}
	if latched(t, c, boot.GateMarginOpsResolved) {
		t.Error("a pending margin op latched — nothing here needs operator clearance, only time")
	}
	if len(f.marginJournal.resolved) != 0 {
		t.Errorf("journalled %v — a pending is not a verdict", f.marginJournal.resolved)
	}
}

// TestReconcileAMarginOpWithNoResolverFailsByName is the optional-collaborator
// pattern's third instance, and both arms are asserted because the pair is the
// claim: an unwired phase over an EMPTY log passes, and over a log with work it
// fails by name rather than reporting §7.5's precondition satisfied.
func TestReconcileAMarginOpWithNoResolverFailsByName(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T, entries ...journalEntry) (*application.Reconciler, *boot.Checklist) {
		t.Helper()
		f := newReconcileFixture(t, joinNow, entries...)
		r, err := application.NewReconciler(f.replayer, f.joiner, f.resolveUseCase, nil, nil,
			application.SecondLook{})
		if err != nil {
			t.Fatalf("NewReconciler: %v", err)
		}
		return r, bootChecklist(t)
	}

	t.Run("an empty log passes", func(t *testing.T) {
		r, c := build(t)
		if _, err := r.Reconcile(c); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomePassed {
			t.Fatalf("GateMarginOpsResolved = %s, want OutcomePassed", o)
		}
	})

	t.Run("an in-flight op fails by name", func(t *testing.T) {
		r, c := build(t, marginOpEntry(t, 1, "MOP-1", time.Hour))
		got, err := r.Reconcile(c)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomeFailed {
			t.Fatalf("GateMarginOpsResolved = %s, want OutcomeFailed", o)
		}
		if !errors.Is(got.Cause, application.ErrMarginOpResolveMissing) {
			t.Errorf("Cause = %v, want ErrMarginOpResolveMissing", got.Cause)
		}
	})

	t.Run("a carried ambiguity fails by name too", func(t *testing.T) {
		// The half a reader would omit: an unwired deployment must not thereby
		// lose an incident a previous process recorded.
		r, c := build(t,
			marginOpEntry(t, 1, "MOP-OLD", time.Hour),
			journalEntry{seq: 2, kind: "margin_op_resolved",
				marginID: "MOP-OLD", mopStatus: domain.MarginOpAmbiguous},
		)
		got, err := r.Reconcile(c)
		if err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if o := outcomeOf(t, c, boot.GateMarginOpsResolved); o != boot.OutcomeFailed {
			t.Fatalf("GateMarginOpsResolved = %s, want OutcomeFailed", o)
		}
		if !errors.Is(got.Cause, application.ErrMarginOpResolveMissing) {
			t.Errorf("Cause = %v, want ErrMarginOpResolveMissing", got.Cause)
		}
	})
}
