package oms_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// clearanceAct is the operator's act, complete (core §7.3's who / what / when
// plus the note).
func clearanceAct(t *testing.T, op domain.MarginOpID) domain.MarginOpReconciliation {
	t.Helper()
	return domain.MarginOpReconciliation{
		Op:    op,
		Actor: "gb",
		At:    time.Date(2026, 8, 8, 11, 30, 0, 0, time.UTC),
		Note:  "counted the venue's isolated margin against the float ledger",
	}
}

// journalAnAmbiguity writes a dispatch record for op and, through a SEPARATE
// journal handle on the same path, the ambiguous verdict that ends it.
//
// It is written directly rather than produced by a boot, and that is what makes
// the fixture reach the state at all: nothing writes a MarginOp until the
// dispatcher (#215), so the shape a real deployment reaches this state by is
// exactly this one — a predecessor process that crashed and a resolution that
// concluded Ambiguous.
func journalAnAmbiguity(t *testing.T, path string, ops ...domain.MarginOpID) {
	t.Helper()

	j, err := driven.OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("OpenSQLiteJournal: %v", err)
	}
	delta, err := fixedpoint.ParseMoney("USDT", "25")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	before, err := fixedpoint.ParseMoney("USDT", "100")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	for _, id := range ops {
		op := domain.MarginOp{
			ID:           id,
			Instrument:   perpInstrument(),
			Delta:        delta,
			BeforePosted: before,
			Watermark:    time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC),
		}
		if err := j.AppendMarginOpDispatched(op); err != nil {
			t.Fatalf("AppendMarginOpDispatched(%q): %v", id, err)
		}
		if err := j.AppendMarginOpResolved(id, domain.MarginOpAmbiguous); err != nil {
			t.Fatalf("AppendMarginOpResolved(%q): %v", id, err)
		}
	}
	if err := j.Close(); err != nil {
		t.Fatalf("closing the predecessor's journal: %v", err)
	}
}

// gateOutcome boots a module against deps and returns what
// GateMarginOpsResolved reported.
func gateOutcome(t *testing.T, deps oms.Deps) boot.Outcome {
	t.Helper()

	m := newModule(t, deps)
	c, err := boot.NewChecklist(boot.GateJournalReplayed, boot.GateVenueJoined,
		boot.GateGenerationsReinstated, boot.GateNetDeltaAsserted, boot.GateMarginOpsResolved)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	if _, err := m.Reconcile.Reconcile(c); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got, declared := c.Outcome(boot.GateMarginOpsResolved)
	if !declared {
		t.Fatal("GateMarginOpsResolved is not declared by this checklist")
	}
	return got
}

// TestTheOperatorActLiftsAJournaledAmbiguity is task #213's whole claim, end to
// end, and the BEFORE half is what makes it one.
//
// Task #212 made the ambiguity durable and latching, which is right, and left a
// §9 incident that nothing in the system could end: the next boot reads the
// verdict back and blocks again, for ever. So this test boots twice against the
// same journal — once before the act and once after — and the first boot is not
// a setup step but the assertion that there was something to lift.
//
// Without it the test would pass against a build where the gate never blocked at
// all, which is the state this whole slice exists inside.
func TestTheOperatorActLiftsAJournaledAmbiguity(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	// The resolver has to be WIRED for the gate to report an outcome at all: a
	// journal holding a margin op with no resolver behind it is OutcomeFailed
	// (ErrMarginOpResolveMissing), which would make both boots below agree for a
	// reason that has nothing to do with the clearance. It finds nothing to
	// resolve — every op here already carries its verdict — so what the venue
	// answers is irrelevant, and that is the point of asserting it this way.
	deps.VenueMargin = scriptedMargin{posted: "110"}
	journalAnAmbiguity(t, deps.JournalPath, "MOP-1")

	if got := gateOutcome(t, deps); got != boot.OutcomeBlocksOpening {
		t.Fatalf("before the act, GateMarginOpsResolved = %s, want OutcomeBlocksOpening — "+
			"there was no incident for the clearance to end", got)
	}

	cleared, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, clearanceAct(t, "MOP-1"))
	if err != nil {
		t.Fatalf("ReconcileMarginOp: %v", err)
	}
	if len(cleared.Remaining) != 0 {
		t.Errorf("Remaining = %v, want none", cleared.Remaining)
	}
	if cleared.ChainSeq == 0 {
		t.Error("ChainSeq = 0, want the journal's head after the append")
	}

	if got := gateOutcome(t, deps); got != boot.OutcomePassed {
		t.Fatalf("after the act, GateMarginOpsResolved = %s, want OutcomePassed — "+
			"the reconciliation did not lift the ambiguity", got)
	}
}

// TestASecondOpenAmbiguityStillBlocksAfterOneIsCleared is the per-op claim, and
// it is the case the CHECKLIST cannot express.
//
// §7.5's checklist holds one outcome per gate rather than one per op, so a boot
// with a second open incident reports exactly what a boot with the first one
// did. Clearing one and expecting the gate to pass is therefore the mistake an
// operator will actually make — which is why the command prints what is left,
// asserted here.
func TestASecondOpenAmbiguityStillBlocksAfterOneIsCleared(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	deps.VenueMargin = scriptedMargin{posted: "110"}
	journalAnAmbiguity(t, deps.JournalPath, "MOP-1", "MOP-2")

	cleared, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, clearanceAct(t, "MOP-1"))
	if err != nil {
		t.Fatalf("ReconcileMarginOp: %v", err)
	}
	if len(cleared.Remaining) != 1 || cleared.Remaining[0] != "MOP-2" {
		t.Fatalf("Remaining = %v, want exactly MOP-2", cleared.Remaining)
	}

	if got := gateOutcome(t, deps); got != boot.OutcomeBlocksOpening {
		t.Fatalf("GateMarginOpsResolved = %s, want OutcomeBlocksOpening — "+
			"one clearance must not lift a second open incident", got)
	}
}

// TestReconcileMarginOpRefusesASecondActOnOneIncident is the durable half of the
// use case's already-reconciled refusal.
//
// The in-memory version is checked one package down; what this adds is that the
// refusal survives the log — the first act was written by a process that has
// exited, so the second command learns of it only by replaying.
func TestReconcileMarginOpRefusesASecondActOnOneIncident(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	journalAnAmbiguity(t, deps.JournalPath, "MOP-1")

	if _, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, clearanceAct(t, "MOP-1")); err != nil {
		t.Fatalf("first ReconcileMarginOp: %v", err)
	}
	second := clearanceAct(t, "MOP-1")
	second.Actor = "somebody-else"
	_, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, second)
	if !errors.Is(err, application.ErrMarginOpAlreadyReconciled) {
		t.Fatalf("second ReconcileMarginOp = %v, want ErrMarginOpAlreadyReconciled", err)
	}
}

// TestReconcileMarginOpRefusesAnIdTheLogDoesNotHold is the mistyped-id case,
// which is the realistic one: the operator is copying an id out of a boot report
// during an incident.
//
// The journal holds a DIFFERENT ambiguity, so this cannot pass by the log being
// empty — the refusal has to be about the id.
func TestReconcileMarginOpRefusesAnIdTheLogDoesNotHold(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	journalAnAmbiguity(t, deps.JournalPath, "MOP-1")

	_, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, clearanceAct(t, "MOP-2"))
	if !errors.Is(err, application.ErrMarginOpNotDispatched) {
		t.Fatalf("ReconcileMarginOp on an unknown id = %v, want ErrMarginOpNotDispatched", err)
	}
}

// TestReconcileMarginOpNeedsNoVenueSurface is the property that makes the act
// available when it is needed.
//
// A §9 accounting incident is exactly when a venue may be unreachable or a key
// rotated. fullDeps supplies no VenueMargin, so the module's whole §7.1 account
// branch is unwired — and this must still work, because it reads the log and
// nothing else. A refactor that reached for a venue read here would fail on this
// fixture rather than in production during an incident.
func TestReconcileMarginOpNeedsNoVenueSurface(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	if deps.VenueMargin != nil {
		t.Fatal("this fixture assumes no VenueMargin — the claim is that the act does not need one")
	}
	journalAnAmbiguity(t, deps.JournalPath, "MOP-1")

	if _, err := oms.ReconcileMarginOp(oms.ClearanceDeps{JournalPath: deps.JournalPath, Account: deps.Account}, clearanceAct(t, "MOP-1")); err != nil {
		t.Fatalf("ReconcileMarginOp with no venue surface wired: %v", err)
	}
}
