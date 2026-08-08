package boot_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/boot"
)

// allGates is the full §7.5 roster, used where a test needs a checklist that
// can only reach PostureTrading by satisfying every one of them.
func allGates() []boot.Gate {
	return []boot.Gate{
		boot.GateJournalDurable,
		boot.GateJournalReplayed,
		boot.GateVenueJoined,
		boot.GateGenerationsReinstated,
		boot.GateNetDeltaAsserted,
		boot.GateAccountPosture,
		boot.GateDeactivationsResolved,
		boot.GateMarginOpsResolved,
		boot.GateWriterAuthority,
	}
}

func newChecklistT(t *testing.T, gates ...boot.Gate) *boot.Checklist {
	t.Helper()
	c, err := boot.NewChecklist(gates...)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	return c
}

func reportAllT(t *testing.T, c *boot.Checklist, o boot.Outcome) {
	t.Helper()
	for _, g := range c.Required() {
		if err := c.Report(g, o); err != nil {
			t.Fatalf("Report(%s, %s): %v", g, o, err)
		}
	}
}

func TestGateValidateAndString(t *testing.T) {
	t.Parallel()

	if err := boot.GateUnspecified.Validate(); !errors.Is(err, boot.ErrGateInvalid) {
		t.Errorf("GateUnspecified.Validate() = %v, want ErrGateInvalid", err)
	}
	// One past the last declared gate, written as a literal rather than derived
	// from allGates(): a bound computed from the same roster the loop below walks
	// would follow maxGate wherever it went, and the check that the sentinel is
	// refused would pass against any maxGate at all. The literal is what makes
	// adding a member without updating this file a failing test.
	if err := boot.Gate(10).Validate(); !errors.Is(err, boot.ErrGateInvalid) {
		t.Errorf("Gate(10).Validate() = %v, want ErrGateInvalid", err)
	}
	// And the roster is the same length, so a member added to the enum and not to
	// allGates() cannot hide behind a literal that happens to still be one past
	// the end.
	if len(allGates()) != 9 {
		t.Errorf("allGates() has %d gates, want 9 — the roster and Gate(10) above must move together", len(allGates()))
	}
	for _, g := range allGates() {
		if err := g.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", g, err)
		}
		if s := g.String(); strings.HasPrefix(s, "Gate(") {
			t.Errorf("Gate(%d).String() = %q, want a name", uint8(g), s)
		}
	}
	if got := boot.Gate(99).String(); got != "Gate(99)" {
		t.Errorf("Gate(99).String() = %q, want %q", got, "Gate(99)")
	}
}

func TestOutcomeValidateAndString(t *testing.T) {
	t.Parallel()

	// The zero value is legal: "unreported" is the state of a fresh checklist.
	for _, o := range []boot.Outcome{
		boot.OutcomeUnreported, boot.OutcomePending, boot.OutcomeFailed,
		boot.OutcomeEvidenceUnusable, boot.OutcomeIncomplete, boot.OutcomeNeedsOperator,
		boot.OutcomeBlocksOpening, boot.OutcomeProjectionStale, boot.OutcomePassed,
	} {
		if err := o.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", o, err)
		}
		if s := o.String(); strings.HasPrefix(s, "Outcome(") {
			t.Errorf("Outcome(%d).String() = %q, want a name", uint8(o), s)
		}
	}
	// One past the last declared member. It moves when a member is added, which
	// is the point: the check is that the LADDER ends where the enum does.
	if err := boot.Outcome(9).Validate(); !errors.Is(err, boot.ErrOutcomeInvalid) {
		t.Errorf("Outcome(9).Validate() = %v, want ErrOutcomeInvalid", err)
	}
	if got := boot.Outcome(9).String(); got != "Outcome(9)" {
		t.Errorf("Outcome(9).String() = %q, want %q", got, "Outcome(9)")
	}
}

func TestNewChecklistRefusals(t *testing.T) {
	t.Parallel()

	if _, err := boot.NewChecklist(); !errors.Is(err, boot.ErrChecklistEmpty) {
		t.Errorf("NewChecklist() = %v, want ErrChecklistEmpty", err)
	}
	if _, err := boot.NewChecklist(boot.GateUnspecified); !errors.Is(err, boot.ErrGateInvalid) {
		t.Errorf("NewChecklist(GateUnspecified) = %v, want ErrGateInvalid", err)
	}
	_, err := boot.NewChecklist(boot.GateVenueJoined, boot.GateAccountPosture, boot.GateVenueJoined)
	if !errors.Is(err, boot.ErrGateDuplicate) {
		t.Errorf("NewChecklist with a repeat = %v, want ErrGateDuplicate", err)
	}
	// The message must say WHICH gate repeated. A bare sentinel leaves an
	// operator diffing a list.
	if err != nil && !strings.Contains(err.Error(), "GateVenueJoined") {
		t.Errorf("duplicate error %q does not name the gate", err)
	}
}

// TestChecklistRequiredIsACopy pins the defensive copy. Without it a caller
// that sorted the returned slice would reorder the checklist's own record of
// what it requires, which is the slice Posture reduces over.
func TestChecklistRequiredIsACopy(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateVenueJoined, boot.GateAccountPosture)
	got := c.Required()
	got[0] = boot.GateWriterAuthority

	again := c.Required()
	if again[0] != boot.GateVenueJoined {
		t.Errorf("Required()[0] = %s after caller mutation, want GateVenueJoined", again[0])
	}
}

// TestFreshChecklistIsReconcileOnly is the zero-state claim: a boot that has
// started and reported nothing is exactly reconcile-only, with no caller having
// had to say so.
func TestFreshChecklistIsReconcileOnly(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("fresh checklist Posture() = %s, want PostureReconcileOnly", got)
	}
	if got := len(c.Blocking()); got != len(allGates()) {
		t.Errorf("fresh checklist Blocking() has %d gates, want %d", got, len(allGates()))
	}
}

// TestChecklistUnreportedGateHoldsPosture is the default-deny mechanism, and it
// is unreachable in any fixture that reports every declared gate. Six of the
// seven gates pass; the seventh is never mentioned, and that alone must hold
// the posture down.
func TestChecklistUnreportedGateHoldsPosture(t *testing.T) {
	t.Parallel()

	gates := allGates()
	c := newChecklistT(t, gates...)
	for _, g := range gates[:len(gates)-1] {
		if err := c.Report(g, boot.OutcomePassed); err != nil {
			t.Fatalf("Report(%s): %v", g, err)
		}
	}

	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("Posture() with one gate unreported = %s, want PostureReconcileOnly", got)
	}
	blocking := c.Blocking()
	if len(blocking) != 1 || blocking[0] != gates[len(gates)-1] {
		t.Errorf("Blocking() = %v, want exactly [%s]", blocking, gates[len(gates)-1])
	}

	// Positive control: reporting that last gate is all that stands between
	// this checklist and PostureTrading, so the refusal above is about the
	// unreported gate and not about something else in the fixture.
	if err := c.Report(gates[len(gates)-1], boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if got := c.Posture(); got != boot.PostureTrading {
		t.Errorf("Posture() with every gate passed = %s, want PostureTrading", got)
	}
	if got := c.Blocking(); len(got) != 0 {
		t.Errorf("Blocking() = %v, want empty at PostureTrading", got)
	}
}

// TestChecklistOutcomeToPosture pins each outcome's ceiling in isolation: every
// other gate passes, so the posture under test is the one gate's doing.
func TestChecklistOutcomeToPosture(t *testing.T) {
	t.Parallel()

	cases := []struct {
		outcome boot.Outcome
		want    boot.Posture
	}{
		{boot.OutcomePassed, boot.PostureTrading},
		{boot.OutcomeBlocksOpening, boot.PostureStopOpening},
		{boot.OutcomeUnreported, boot.PostureReconcileOnly},
		{boot.OutcomePending, boot.PostureReconcileOnly},
		{boot.OutcomeIncomplete, boot.PostureReconcileOnly},
		{boot.OutcomeNeedsOperator, boot.PostureReconcileOnly},
		{boot.OutcomeFailed, boot.PostureHalted},
		{boot.OutcomeEvidenceUnusable, boot.PostureHalted},
		{boot.OutcomeProjectionStale, boot.PostureStopOpening},
	}

	for _, tc := range cases {
		t.Run(tc.outcome.String(), func(t *testing.T) {
			t.Parallel()
			c := newChecklistT(t, allGates()...)
			reportAllT(t, c, boot.OutcomePassed)
			if err := c.Report(boot.GateMarginOpsResolved, tc.outcome); err != nil {
				t.Fatalf("Report: %v", err)
			}
			if got := c.Posture(); got != tc.want {
				t.Errorf("Posture() with %s = %s, want %s", tc.outcome, got, tc.want)
			}
		})
	}
}

// TestChecklistTakesTheWeakest puts the constraining gate in a position where
// "first blocking gate wins" and "weakest wins" give different answers. §13g's
// lesson, applied before the mutation rather than after: a fixture whose worst
// outcome is also its first makes the two rules indistinguishable.
func TestChecklistTakesTheWeakest(t *testing.T) {
	t.Parallel()

	gates := allGates()
	c := newChecklistT(t, gates...)
	reportAllT(t, c, boot.OutcomePassed)

	// The MILDER refusal comes first in declaration order; the harsher one is
	// last. "Name the first" would answer PostureStopOpening.
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomeBlocksOpening); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := c.Report(boot.GateWriterAuthority, boot.OutcomeFailed); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if got := c.Posture(); got != boot.PostureHalted {
		t.Errorf("Posture() = %s, want PostureHalted (the weakest, not the first)", got)
	}

	// And the mirror: harsher first, milder last. Both orders must agree, which
	// is what makes the reduction order-independent rather than merely correct
	// on the fixture that happened to get written.
	d := newChecklistT(t, gates...)
	reportAllT(t, d, boot.OutcomePassed)
	if err := d.Report(boot.GateJournalReplayed, boot.OutcomeFailed); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := d.Report(boot.GateWriterAuthority, boot.OutcomeBlocksOpening); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if got := d.Posture(); got != boot.PostureHalted {
		t.Errorf("Posture() with the order reversed = %s, want PostureHalted", got)
	}
}

func TestChecklistReportRefusesUndeclaredGate(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateVenueJoined, boot.GateAccountPosture)

	err := c.Report(boot.GateWriterAuthority, boot.OutcomePassed)
	if !errors.Is(err, boot.ErrGateUndeclared) {
		t.Errorf("Report against an undeclared gate = %v, want ErrGateUndeclared", err)
	}
	if err != nil && !strings.Contains(err.Error(), "GateWriterAuthority") {
		t.Errorf("error %q does not name the gate", err)
	}
	if _, declared := c.Outcome(boot.GateWriterAuthority); declared {
		t.Error("Outcome() reports an undeclared gate as declared")
	}

	// Positive control: the same call against a declared gate is accepted, so
	// the refusal is about declaration and not about the outcome or the gate.
	if err := c.Report(boot.GateVenueJoined, boot.OutcomePassed); err != nil {
		t.Errorf("Report against a declared gate = %v, want nil", err)
	}
}

func TestChecklistReportRefusesInvalidOutcome(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateVenueJoined)
	if err := c.Report(boot.GateVenueJoined, boot.Outcome(9)); !errors.Is(err, boot.ErrOutcomeInvalid) {
		t.Errorf("Report with an invalid outcome = %v, want ErrOutcomeInvalid", err)
	}
	// The gate must be untouched by the rejected report.
	if o, _ := c.Outcome(boot.GateVenueJoined); o != boot.OutcomeUnreported {
		t.Errorf("outcome after a rejected report = %s, want OutcomeUnreported", o)
	}
}

// TestChecklistNeedsOperatorLatches is §7.4's "never health-flap re-entry",
// reached through §7.5's door. A gate that has alerted for operator clearance
// must not be liftable by the check that alerted simply running again and
// succeeding.
func TestChecklistNeedsOperatorLatches(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateVenueJoined, boot.OutcomeNeedsOperator); err != nil {
		t.Fatalf("Report: %v", err)
	}

	for _, o := range []boot.Outcome{
		boot.OutcomePassed, boot.OutcomeBlocksOpening, boot.OutcomePending, boot.OutcomeFailed,
	} {
		err := c.Report(boot.GateVenueJoined, o)
		if !errors.Is(err, boot.ErrGateLatched) {
			t.Errorf("Report(%s) over a latched gate = %v, want ErrGateLatched", o, err)
		}
	}
	// Re-reporting the same outcome is not a transition and is allowed: a
	// reconcile that runs again and finds the same foreign order should not
	// have to special-case its own report.
	if err := c.Report(boot.GateVenueJoined, boot.OutcomeNeedsOperator); err != nil {
		t.Errorf("re-reporting NeedsOperator = %v, want nil", err)
	}
	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("Posture() = %s, want PostureReconcileOnly", got)
	}
}

// TestIncompleteRefusesWITHOUTLatching is the third empty cell's own claim, and
// it asserts BOTH halves for TestEvidenceUnusableHaltsANDLatches' reason one row
// over: the posture alone is already provided by OutcomePending and the
// non-latching alone by OutcomeFailed, so a fixture checking either half would
// pass against the wrong member.
//
// The producer is §7.5's boot finding work it could not finish — a sweep that
// did not prove coverage, an absence whose finality timer has not elapsed.
// Reconcile-only matters because halting a healthy process forbids §7.5's own
// DeactivationPending re-drive; not latching matters because the condition
// clears itself and demanding an operator for a timer is the
// halts-itself-with-every-light-green failure.
func TestIncompleteRefusesWITHOUTLatching(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateVenueJoined, boot.OutcomeIncomplete); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("Posture() = %s, want PostureReconcileOnly — halting forbids the reconcile writes that finish the work", got)
	}
	// The half no other reconcile-only member has: a later report lifts it, with
	// no operator in the loop.
	if err := c.Report(boot.GateVenueJoined, boot.OutcomePassed); err != nil {
		t.Fatalf("Report(Passed) over Incomplete = %v, want nil — the retry is what lifts it", err)
	}
	if got := c.Posture(); got != boot.PostureTrading {
		t.Errorf("Posture() = %s, want PostureTrading — %v is blocking", got, c.Blocking())
	}
	// And Clear must refuse it, which is the same claim from the other side: a
	// non-latching outcome is not an operator hold, so there is nothing to clear.
	if err := c.Clear(boot.GateVenueJoined); !errors.Is(err, boot.ErrGateNotLatched) {
		t.Errorf("Clear(Incomplete) = %v, want ErrGateNotLatched", err)
	}
}

// TestEvidenceUnusableHaltsANDLatches is the combination §7.5 needed and this
// enum did not have until slice F1, and the test asserts BOTH halves — because
// each half alone is already provided by an existing member, so a fixture
// checking one of them would pass against the wrong member.
//
// The producer is §7.5's journal replay finding a chain that does not verify.
// Halted matters because PostureReconcileOnly permits ActReconcile, and §7.5's
// reconcile writes — the DeactivationPending re-drive above all — are computed
// FROM the log that just failed to verify. Latching matters because the retry
// of a hash verification over unchanged bytes is deterministic, so a
// re-reportable refusal would be liftable by doing nothing.
func TestEvidenceUnusableHaltsAndLatches(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomeEvidenceUnusable); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if got := c.Posture(); got != boot.PostureHalted {
		t.Errorf("Posture() = %s, want PostureHalted — reconcile-only would permit "+
			"the re-drives this log is the input to", got)
	}
	if c.Posture().Permits(boot.ActReconcile) {
		t.Error("a process whose journal does not verify may reconcile — " +
			"ActReconcile is exposure-neutral given trustworthy inputs, " +
			"and this is the gate that says they are not")
	}

	for _, o := range []boot.Outcome{
		boot.OutcomePassed, boot.OutcomeBlocksOpening, boot.OutcomePending,
		boot.OutcomeFailed, boot.OutcomeNeedsOperator,
	} {
		if err := c.Report(boot.GateJournalReplayed, o); !errors.Is(err, boot.ErrGateLatched) {
			t.Errorf("Report(%s) over an EvidenceUnusable gate = %v, want ErrGateLatched", o, err)
		}
	}
	// The honest repeat: the same walk over the same bytes fails identically,
	// and a producer must not have to special-case its own second report.
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomeEvidenceUnusable); err != nil {
		t.Errorf("re-reporting EvidenceUnusable = %v, want nil", err)
	}
	// And it clears like any other latched gate — to Unreported, not to Passed.
	if err := c.Clear(boot.GateJournalReplayed); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if o, _ := c.Outcome(boot.GateJournalReplayed); o != boot.OutcomeUnreported {
		t.Errorf("outcome after Clear = %s, want OutcomeUnreported", o)
	}
}

// TestALatchedGateAdmitsAnEscalationAndRefusesEverythingElse pins the one
// transition between two latching outcomes, in both directions.
//
// The escalation is admitted because the alternative pins a gate at the weaker
// of two refusals by the accident of which was discovered first. The reverse is
// refused because it weakens a hold. And OutcomeFailed is refused despite being
// MORE restrictive than OutcomeNeedsOperator, which is the case a posture
// comparison alone would get wrong: Failed does not latch, so admitting it
// would launder the gate into one a later Passed could lift — clearance by two
// reports with no operator in either.
func TestALatchedGateAdmitsAnEscalationAndRefusesEverythingElse(t *testing.T) {
	t.Parallel()

	up := newChecklistT(t, allGates()...)
	if err := up.Report(boot.GateJournalReplayed, boot.OutcomeNeedsOperator); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := up.Report(boot.GateJournalReplayed, boot.OutcomeEvidenceUnusable); err != nil {
		t.Errorf("escalating a latched gate to a stricter latching outcome = %v, want nil", err)
	}
	if o, _ := up.Outcome(boot.GateJournalReplayed); o != boot.OutcomeEvidenceUnusable {
		t.Errorf("outcome after escalation = %s, want OutcomeEvidenceUnusable", o)
	}

	down := newChecklistT(t, allGates()...)
	if err := down.Report(boot.GateJournalReplayed, boot.OutcomeEvidenceUnusable); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := down.Report(boot.GateJournalReplayed, boot.OutcomeNeedsOperator); !errors.Is(err, boot.ErrGateLatched) {
		t.Errorf("weakening a latched gate to a more permissive latching outcome = %v, want ErrGateLatched", err)
	}
	if o, _ := down.Outcome(boot.GateJournalReplayed); o != boot.OutcomeEvidenceUnusable {
		t.Errorf("outcome after a refused weakening = %s, want OutcomeEvidenceUnusable", o)
	}
}

// TestChecklistClearReturnsToUnreported is the half of clearance that is easy
// to get wrong in the expensive direction. §7.4 requires clearance AND a fresh
// propagation test, so Clear must restore the gate's ABILITY to pass — not the
// pass. Clearing straight to Passed would turn an operator acknowledging an
// alert into an assertion about venue state they never made.
func TestChecklistClearReturnsToUnreported(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateWriterAuthority, boot.OutcomeNeedsOperator); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if err := c.Clear(boot.GateWriterAuthority); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if o, _ := c.Outcome(boot.GateWriterAuthority); o != boot.OutcomeUnreported {
		t.Errorf("outcome after Clear = %s, want OutcomeUnreported", o)
	}
	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("Posture() after Clear = %s, want PostureReconcileOnly — clearance is not a pass", got)
	}

	// Positive control: the gate is now reportable again, and re-verifying it
	// is what actually restores trading.
	if err := c.Report(boot.GateWriterAuthority, boot.OutcomePassed); err != nil {
		t.Fatalf("Report after Clear: %v", err)
	}
	if got := c.Posture(); got != boot.PostureTrading {
		t.Errorf("Posture() after a fresh pass = %s, want PostureTrading", got)
	}
}

func TestChecklistClearRefusals(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateVenueJoined, boot.GateAccountPosture)

	if err := c.Clear(boot.GateWriterAuthority); !errors.Is(err, boot.ErrGateUndeclared) {
		t.Errorf("Clear of an undeclared gate = %v, want ErrGateUndeclared", err)
	}

	// A Clear against a gate that is not latched is a control an operator
	// believes worked. Checked at each non-latching outcome, not just the zero
	// one, because "unreported" and "reported but not latched" are different
	// states and only one of them is the zero value.
	for _, o := range []boot.Outcome{
		boot.OutcomeUnreported, boot.OutcomePending, boot.OutcomeFailed,
		boot.OutcomeIncomplete, boot.OutcomePassed,
	} {
		if err := c.Report(boot.GateVenueJoined, o); err != nil {
			t.Fatalf("Report(%s): %v", o, err)
		}
		if err := c.Clear(boot.GateVenueJoined); !errors.Is(err, boot.ErrGateNotLatched) {
			t.Errorf("Clear of a gate at %s = %v, want ErrGateNotLatched", o, err)
		}
		if got, _ := c.Outcome(boot.GateVenueJoined); got != o {
			t.Errorf("outcome after a refused Clear = %s, want %s unchanged", got, o)
		}
	}
}

// TestChecklistBlockingIsSorted varies the fixture dimension that would
// otherwise make "sorted" and "declaration order" the same answer. The gates
// are declared in DESCENDING order, so a Blocking that simply walked the
// declaration would return them backwards.
func TestChecklistBlockingIsSorted(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t,
		boot.GateWriterAuthority,
		boot.GateMarginOpsResolved,
		boot.GateAccountPosture,
		boot.GateJournalReplayed,
	)
	if err := c.Report(boot.GateAccountPosture, boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}

	want := []boot.Gate{
		boot.GateJournalReplayed,
		boot.GateMarginOpsResolved,
		boot.GateWriterAuthority,
	}
	got := c.Blocking()
	if len(got) != len(want) {
		t.Fatalf("Blocking() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Blocking() = %v, want %v", got, want)
		}
	}
}

// TestLatchedIsASubsetOfBlockingAndNamesOnlyTheOperatorHolds is the read-only
// answer to "what is waiting for a human".
//
// The fixture varies the parameter that makes the two methods different answers.
// Four gates: one Passed (in neither list), one Unreported (Blocking only — the
// structural case, and on a real deployment the commonest), one OutcomeFailed
// (Blocking only, and the one a reader would most expect to be latched — it is
// the MOST restrictive outcome and does not latch), and one OutcomeNeedsOperator
// (both). Without the OutcomeFailed row, "blocking" and "latched" would agree on
// every gate in the fixture and a Latched that simply returned Blocking would
// pass.
func TestLatchedIsASubsetOfBlockingAndNamesOnlyTheOperatorHolds(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t,
		boot.GateWriterAuthority,
		boot.GateMarginOpsResolved,
		boot.GateAccountPosture,
		boot.GateJournalDurable,
		boot.GateJournalReplayed,
	)
	if err := c.Report(boot.GateAccountPosture, boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomeFailed); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := c.Report(boot.GateJournalDurable, boot.OutcomeEvidenceUnusable); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := c.Report(boot.GateWriterAuthority, boot.OutcomeNeedsOperator); err != nil {
		t.Fatalf("Report: %v", err)
	}

	// GateMarginOpsResolved is unreported and GateJournalReplayed merely failed:
	// both hold the posture and neither waits for a person.
	//
	// Asserted 50 times over the SAME checklist, because the answer is assembled
	// by walking a Go map and the two latched gates are declared in the reverse of
	// their Gate order. One call is a coin flip about whether an unsorted
	// implementation happens to yield them the right way round — a mutation
	// deleting the sort SURVIVED a single-call version of this test, which is this
	// repo's recorded lesson about Send and Receive arriving on a map instead of a
	// select.
	wantLatched := []boot.Gate{boot.GateJournalDurable, boot.GateWriterAuthority}
	for i := 0; i < 50; i++ {
		if got := c.Latched(); !reflect.DeepEqual(got, wantLatched) {
			t.Fatalf("trial %d: Latched() = %v, want %v — an alert that names a different "+
				"first cause on two identical boots is one an operator learns to distrust",
				i, got, wantLatched)
		}
	}
	wantBlocking := []boot.Gate{
		boot.GateJournalDurable,
		boot.GateJournalReplayed,
		boot.GateMarginOpsResolved,
		boot.GateWriterAuthority,
	}
	if got := c.Blocking(); !reflect.DeepEqual(got, wantBlocking) {
		t.Errorf("Blocking() = %v, want %v", got, wantBlocking)
	}
}

// TestLatchedIsEmptyOnAFreshChecklist is the positive control the method needs:
// a checklist nobody has reported on blocks everything and latches nothing, so a
// Latched that answered "every required gate" would not read as correct.
func TestLatchedIsEmptyOnAFreshChecklist(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	if got := c.Latched(); len(got) != 0 {
		t.Errorf("Latched() on a fresh checklist = %v, want none", got)
	}
	if got := c.Blocking(); len(got) != len(allGates()) {
		t.Errorf("Blocking() on a fresh checklist = %v, want every gate", got)
	}
}

// TestProjectionStaleWithholdsOpeningsAndLatches is the second empty cell of the
// outcome product, and the two halves fail in opposite directions.
//
// Not halted: §6 step 7's condition is a send that REACHED the venue, so the
// order may be executing while the projection says it was never dispatched, and
// a halted process cannot flatten it. Latching: the check that would lift it —
// the net-delta assertion — recomputes from the projection that is known stale,
// and the stale reading is the PERMISSIVE one (StatePlanned contributes no
// possible delta, so the group reads determined and flat). A freely
// re-reportable outcome would therefore be lifted by an invariant agreeing with
// its own corrupted input.
func TestProjectionStaleWithholdsOpeningsAndLatches(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeProjectionStale); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if got := c.Posture(); got != boot.PostureStopOpening {
		t.Errorf("Posture() = %s, want PostureStopOpening", got)
	}
	if !c.Posture().Permits(boot.ActReduce) {
		t.Error("a process holding a stale projection may not reduce exposure — " +
			"the order this outcome is about may be filling right now")
	}
	if c.Posture().Permits(boot.ActOpen) {
		t.Error("openings are permitted while the projection is known stale")
	}

	for _, o := range []boot.Outcome{
		boot.OutcomePassed, boot.OutcomeBlocksOpening, boot.OutcomePending, boot.OutcomeFailed,
	} {
		if err := c.Report(boot.GateNetDeltaAsserted, o); !errors.Is(err, boot.ErrGateLatched) {
			t.Errorf("Report(%s) over a ProjectionStale gate = %v, want ErrGateLatched", o, err)
		}
	}
	// OutcomeFailed is in that list deliberately: it is MORE restrictive and does
	// not latch, so admitting it would launder the hold into one a later Passed
	// could lift.

	// The two strictly-more-restrictive latching outcomes are admitted, because
	// a gate pinned at the weaker of two refusals by the accident of discovery
	// order is the failure the escalation rule exists to prevent.
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator); err != nil {
		t.Errorf("escalating ProjectionStale to NeedsOperator = %v, want nil", err)
	}
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeEvidenceUnusable); err != nil {
		t.Errorf("escalating NeedsOperator to EvidenceUnusable = %v, want nil", err)
	}
}

// TestAmbiguousMarginOpWithholdsOpeningsAndLatches is task #212's claim: §7.5's
// Ambiguous MarginOp blocks new openings and only an explicit reconciliation
// lifts it.
//
// Stop-opening, because §7.1 says the safety actions continue from LIVE liq
// distance — the ladder must stay available during the incident that produced
// the ambiguity. Latching, because §7.5 blocks openings "until the staged-float
// ledger is explicitly reconciled" and the evidence a re-run would read is a
// delta against a journaled before-state, which every later position mutation
// moves further away.
//
// It is the mirror of TestProjectionStaleWithholdsOpeningsAndLatches, and the
// pair is the point: two members now share the stop-opening latching cell, so
// each one's claims have to be asserted from its own side.
func TestAmbiguousMarginOpWithholdsOpeningsAndLatches(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, allGates()...)
	reportAllT(t, c, boot.OutcomePassed)
	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomeBlocksOpening); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if got := c.Posture(); got != boot.PostureStopOpening {
		t.Errorf("Posture() = %s, want PostureStopOpening", got)
	}
	if !c.Posture().Permits(boot.ActReduce) {
		t.Error("a process holding an ambiguous margin op may not reduce exposure — " +
			"§7.1's safety actions read live liq distance and are explicitly not blocked")
	}
	if c.Posture().Permits(boot.ActOpen) {
		t.Error("openings are permitted while a margin op is unattributable")
	}

	// Every weakening report, including the one that made this finding: a fresh
	// OutcomePassed is exactly what a later op's successful resolution would
	// report, and under the old classification it lifted the incident.
	for _, o := range []boot.Outcome{
		boot.OutcomePassed, boot.OutcomePending, boot.OutcomeIncomplete, boot.OutcomeFailed,
	} {
		if err := c.Report(boot.GateMarginOpsResolved, o); !errors.Is(err, boot.ErrGateLatched) {
			t.Errorf("Report(%s) over an ambiguous margin op = %v, want ErrGateLatched", o, err)
		}
	}

	// The SAME-CELL member is refused too, and that is the half a `<=` posture
	// comparison admits. It weakens nothing — the hold is identical — but it
	// replaces the cause an operator reads with whichever refusal was discovered
	// last, and the cause is the only thing the two members differ in.
	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomeProjectionStale); !errors.Is(err, boot.ErrGateLatched) {
		t.Errorf("Report(ProjectionStale) over an ambiguous margin op = %v, want ErrGateLatched", err)
	}

	// The repeat is honest and admitted: a boot that found an ambiguity finds it
	// again on every attempt.
	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomeBlocksOpening); err != nil {
		t.Errorf("restating the same ambiguity = %v, want nil", err)
	}
	// And the escalation is admitted, because it is strictly more restrictive.
	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomeNeedsOperator); err != nil {
		t.Errorf("escalating an ambiguous margin op to NeedsOperator = %v, want nil", err)
	}
}

// TestAmbiguousMarginOpClears pins the one thing that DOES lift it, so the latch
// above is not mistaken for a permanent halt.
func TestAmbiguousMarginOpClears(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateMarginOpsResolved)
	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomeBlocksOpening); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := c.Clear(boot.GateMarginOpsResolved); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	// §7.5's rule that clearance returns a gate to UNREPORTED rather than to
	// passed: the check still has to run and succeed afterwards.
	if got, _ := c.Outcome(boot.GateMarginOpsResolved); got != boot.OutcomeUnreported {
		t.Errorf("outcome after Clear = %s, want OutcomeUnreported", got)
	}
}

// # PermitsNow: the running-process read — verdicts constrain, silence does not,
// # and NO verdicts refuses everything

// TestPermitsNowRefusesEverythingWithNoVerdict is the fail-closed half: a fold
// over only-the-answers has PostureTrading for its identity, and a fresh
// checklist must not read as "everything passed". Pending is included because
// it is the OTHER pre-verdict member — a checklist whose every gate is running
// has still concluded nothing.
func TestPermitsNowRefusesEverythingWithNoVerdict(t *testing.T) {
	t.Parallel()

	fresh := newChecklistT(t, boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
	pending := newChecklistT(t, boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
	if err := pending.Report(boot.GateNetDeltaAsserted, boot.OutcomePending); err != nil {
		t.Fatalf("Report: %v", err)
	}

	for name, c := range map[string]*boot.Checklist{"fresh": fresh, "all pending": pending} {
		for _, a := range []boot.Act{boot.ActReconcile, boot.ActReduce, boot.ActOpen, boot.ActUnspecified} {
			if c.PermitsNow(a) {
				t.Errorf("%s checklist PermitsNow(%s) = true, want false — no check has concluded", name, a)
			}
		}
	}
}

// TestPermitsNowIgnoresSilenceWherePostureCannot is the distinction this method
// exists for, asserted on ONE fixture from both sides: a Passed verdict beside
// an Unreported gate. Posture() must stay reconcile-only (default-deny over the
// roster) while PermitsNow answers from the verdict alone — this is the
// produced-versus-structural split the reconcile root's ExitClass draws,
// available per act.
func TestPermitsNowIgnoresSilenceWherePostureCannot(t *testing.T) {
	t.Parallel()

	c := newChecklistT(t, boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if got := c.Posture(); got != boot.PostureReconcileOnly {
		t.Errorf("Posture() = %s, want PostureReconcileOnly — the unreported gate must hold the boot fold down", got)
	}
	if !c.PermitsNow(boot.ActOpen) {
		t.Error("PermitsNow(ActOpen) = false with the only verdict Passed — silence constrained the verdict fold")
	}
	if c.PermitsNow(boot.ActUnspecified) {
		t.Error("PermitsNow(ActUnspecified) = true — a caller that has not said what it wants must be refused")
	}
}

// TestPermitsNowConstrainsByTheVerdictsCeiling walks every verdict-bearing
// member through the three acts on a fixture whose OTHER gate holds a Passed
// verdict — so the member under test is never the only verdict, and a fold
// that read the wrong gate would answer from Passed and fail the case.
func TestPermitsNowConstrainsByTheVerdictsCeiling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		outcome                 boot.Outcome
		reconcile, reduce, open bool
	}{
		{boot.OutcomeFailed, false, false, false},
		{boot.OutcomeEvidenceUnusable, false, false, false},
		{boot.OutcomeIncomplete, true, false, false},
		{boot.OutcomeNeedsOperator, true, false, false},
		{boot.OutcomeBlocksOpening, true, true, false},
		{boot.OutcomeProjectionStale, true, true, false},
		{boot.OutcomePassed, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.outcome.String(), func(t *testing.T) {
			t.Parallel()

			c := newChecklistT(t, boot.GateWriterAuthority, boot.GateNetDeltaAsserted)
			if err := c.Report(boot.GateWriterAuthority, boot.OutcomePassed); err != nil {
				t.Fatalf("Report(WriterAuthority, Passed): %v", err)
			}
			if err := c.Report(boot.GateNetDeltaAsserted, tc.outcome); err != nil {
				t.Fatalf("Report(NetDeltaAsserted, %s): %v", tc.outcome, err)
			}

			got := [3]bool{
				c.PermitsNow(boot.ActReconcile),
				c.PermitsNow(boot.ActReduce),
				c.PermitsNow(boot.ActOpen),
			}
			want := [3]bool{tc.reconcile, tc.reduce, tc.open}
			if got != want {
				t.Errorf("PermitsNow(reconcile/reduce/open) = %v, want %v", got, want)
			}
		})
	}
}

// TestPermitsNowTakesTheWeakestVerdictEitherWayRound: with two verdicts the
// answer is their minimum, and it must not depend on declaration order — the
// mutant this kills is a fold that keeps the FIRST (or last) verdict it meets.
func TestPermitsNowTakesTheWeakestVerdictEitherWayRound(t *testing.T) {
	t.Parallel()

	orders := map[string][2]boot.Gate{
		"constraining gate declared first": {boot.GateNetDeltaAsserted, boot.GateWriterAuthority},
		"constraining gate declared last":  {boot.GateWriterAuthority, boot.GateNetDeltaAsserted},
	}
	for name, gates := range orders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := newChecklistT(t, gates[0], gates[1])
			if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeProjectionStale); err != nil {
				t.Fatalf("Report: %v", err)
			}
			if err := c.Report(boot.GateWriterAuthority, boot.OutcomePassed); err != nil {
				t.Fatalf("Report: %v", err)
			}

			if !c.PermitsNow(boot.ActReduce) {
				t.Error("PermitsNow(ActReduce) = false — stop-opening is the stale verdict's whole point")
			}
			if c.PermitsNow(boot.ActOpen) {
				t.Error("PermitsNow(ActOpen) = true over a ProjectionStale verdict — the weaker verdict won the fold")
			}
		})
	}
}
