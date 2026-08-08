package application_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// The clearance's fixture (task #213, record §82).
//
// # Every Recovery below holds FOUR margin ops, one per collection
//
// The partition is what the use case's diagnostics rest on, and a fixture with
// one op in it would make "this op's collection" and "the only collection" the
// same answer. So each case names an op that is in the collection under test
// while the other three stay populated — the collapse checklist's cardinality
// dimension, at the level the claim is actually about.
const (
	clearanceOpen      = domain.MarginOpID("MOP-open")     // ambiguous, unreconciled
	clearanceCleared   = domain.MarginOpID("MOP-cleared")  // ambiguous, reconciled
	clearanceInFlight  = domain.MarginOpID("MOP-inflight") // dispatched, no verdict
	clearanceSettled   = domain.MarginOpID("MOP-settled")  // resolved applied
	clearanceUnknownID = domain.MarginOpID("MOP-not-a-real-op")
)

func clearanceAct(op domain.MarginOpID) domain.MarginOpReconciliation {
	return domain.MarginOpReconciliation{
		Op:    op,
		Actor: "gb",
		At:    time.Date(2026, 8, 8, 11, 30, 0, 0, time.UTC),
		Note:  "counted the venue's isolated margin against the float ledger",
	}
}

// clearancePriorActor and clearancePriorAt are the FIRST act's fields, and they
// differ from clearanceAct's in both — so the already-reconciled refusal's
// message cannot be satisfied by echoing the act it just refused.
const clearancePriorActor = "an-earlier-operator"

func clearanceRecovery(t *testing.T) application.Recovery {
	t.Helper()
	return application.Recovery{
		MarginOps: 4,
		InFlightMarginOps: []domain.MarginOp{{
			ID:           clearanceInFlight,
			Instrument:   perp(),
			Delta:        marginTestMoney(t, "25"),
			BeforePosted: marginTestMoney(t, "100"),
			Watermark:    time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC),
		}},
		AmbiguousMarginOps: []domain.MarginOpID{clearanceOpen},
		ReconciledMarginOps: []domain.MarginOpReconciliation{{
			Op:    clearanceCleared,
			Actor: clearancePriorActor,
			At:    time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC),
			Note:  "reconciled against the July statement",
		}},
		SettledMarginOps: map[domain.MarginOpID]domain.MarginOpStatus{
			clearanceSettled: domain.MarginOpApplied,
		},
	}
}

func newClearance(t *testing.T) (*application.MarginOpReconciler, *fakeMarginOpJournal) {
	t.Helper()
	j := &fakeMarginOpJournal{}
	r, err := application.NewMarginOpReconciler(j)
	if err != nil {
		t.Fatalf("NewMarginOpReconciler: %v", err)
	}
	return r, j
}

// TestReconcileRecordsTheWholeActOnAnOpenAmbiguity is the positive control, and
// it asserts the RECORD rather than the call.
//
// Core §7.3 requires who / what / when, so a use case that appended a record
// carrying only the op id would satisfy "an append happened" and record nobody.
// Each field is compared to the value that went in.
func TestReconcileRecordsTheWholeActOnAnOpenAmbiguity(t *testing.T) {
	t.Parallel()

	r, j := newClearance(t)
	act := clearanceAct(clearanceOpen)
	if err := r.Reconcile(clearanceRecovery(t), act); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(j.reconciled) != 1 {
		t.Fatalf("journal holds %d reconciliations, want 1", len(j.reconciled))
	}
	got := j.reconciled[0]
	if got.Op != act.Op {
		t.Errorf("recorded op %q, want %q", got.Op, act.Op)
	}
	if got.Actor != act.Actor {
		t.Errorf("recorded actor %q, want %q", got.Actor, act.Actor)
	}
	if !got.At.Equal(act.At) {
		t.Errorf("recorded instant %s, want %s", got.At, act.At)
	}
	if got.Note != act.Note {
		t.Errorf("recorded note %q, want %q", got.Note, act.Note)
	}
}

// TestReconcileRefusesEveryOpWithNoOpenIncident is the four refusals, each
// asserted on its own sentinel.
//
// The refusing path is the untested one in every end-to-end fixture, because
// fixtures serve the healthy world — so these are the cases, and the sentinel is
// the assertion rather than "an error", because the four are four different
// operator conversations and a single "not an open ambiguity" would collapse
// them.
//
// Every case also asserts that NOTHING was written. A refusal that appended
// first and reported afterwards would leave an audit entry for an act the system
// declined to take.
func TestReconcileRefusesEveryOpWithNoOpenIncident(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		op   domain.MarginOpID
		want error
	}{
		{"an id the log has no dispatch record for", clearanceUnknownID, application.ErrMarginOpNotDispatched},
		{"an op still in flight", clearanceInFlight, application.ErrMarginOpNotAmbiguous},
		{"an op the evidence settled", clearanceSettled, application.ErrMarginOpNotAmbiguous},
		{"an op already reconciled", clearanceCleared, application.ErrMarginOpAlreadyReconciled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, j := newClearance(t)
			err := r.Reconcile(clearanceRecovery(t), clearanceAct(c.op))
			if !errors.Is(err, c.want) {
				t.Fatalf("Reconcile(%q) = %v, want %v", c.op, err, c.want)
			}
			if len(j.reconciled) != 0 {
				t.Fatalf("a refused act wrote %d records, want 0", len(j.reconciled))
			}
		})
	}
}

// TestReconcileNamesThePriorActorOnASecondAct is the already-reconciled
// message's own claim, and it is separate from the sentinel above because they
// fail independently.
//
// An operator told only "already reconciled" has to go and read the journal to
// find out by whom — during a §9 incident, about a record the command is holding
// in its hand. The prior actor's name is in the fixture and nowhere in the act
// being refused, so a message built from the wrong one cannot pass.
func TestReconcileNamesThePriorActorOnASecondAct(t *testing.T) {
	t.Parallel()

	r, _ := newClearance(t)
	err := r.Reconcile(clearanceRecovery(t), clearanceAct(clearanceCleared))
	if err == nil {
		t.Fatal("Reconcile on an already-reconciled op returned nil")
	}
	if !strings.Contains(err.Error(), clearancePriorActor) {
		t.Fatalf("Reconcile error %q does not name the prior actor %q", err, clearancePriorActor)
	}
}

// TestReconcileRefusesAMalformedActBeforeConsultingTheLog is the ordering
// claim, and it is observable because the fixture's Recovery would ADMIT the op.
//
// The act names clearanceOpen — an open ambiguity — and is missing its actor. A
// use case that consulted the log first and validated afterwards would still
// refuse, so the sentinel is what distinguishes them: this must be §7.3's
// missing "who", not anything about the op.
func TestReconcileRefusesAMalformedActBeforeConsultingTheLog(t *testing.T) {
	t.Parallel()

	r, j := newClearance(t)
	act := clearanceAct(clearanceOpen)
	act.Actor = ""
	err := r.Reconcile(clearanceRecovery(t), act)
	if !errors.Is(err, domain.ErrMarginOpReconcileActorUnset) {
		t.Fatalf("Reconcile = %v, want ErrMarginOpReconcileActorUnset", err)
	}
	if len(j.reconciled) != 0 {
		t.Fatalf("a malformed act wrote %d records, want 0", len(j.reconciled))
	}
}

// TestNewMarginOpReconcilerRefusesNoJournal is the constructor's one claim. A
// clearance with nowhere to write it is a command that reports success and lifts
// nothing.
func TestNewMarginOpReconcilerRefusesNoJournal(t *testing.T) {
	t.Parallel()

	if _, err := application.NewMarginOpReconciler(nil); !errors.Is(err, application.ErrMarginOpJournalMissing) {
		t.Fatalf("NewMarginOpReconciler(nil) = %v, want ErrMarginOpJournalMissing", err)
	}
}
