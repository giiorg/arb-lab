package reconcile

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// clearanceAct is a complete operator act — core §7.3's who / what / when plus
// the note. It names an op no journal here holds, which is deliberate: every
// claim below is about the config, and reaching the log at all would mean the
// config check did not fire.
func clearanceAct() domain.MarginOpReconciliation {
	return domain.MarginOpReconciliation{
		Op:    "MOP-1",
		Actor: "gb",
		At:    time.Date(2026, 8, 8, 11, 30, 0, 0, time.UTC),
		Note:  "counted the venue's isolated margin against the float ledger",
	}
}

// TestReconcileMarginOpValidatesTheWholeConfigNotOnlyWhatItReads is aimed at the
// boundary a mutant moved rather than at the case the guard is named for.
//
// The act uses two config fields, and both have downstream refusals: an empty
// journal path fails at Open and an empty account at the write repository. So a
// mutant deleting `cfg.Validate()` SURVIVED — the named case was still refused,
// by somebody else. What only this guard catches is a config that is complete in
// those two fields and inconsistent elsewhere, which is this fixture.
//
// It is kept rather than deleted as a duplicate because of the direction it
// fails in: the act binds to "the journal THIS deployment boots against", and
// reading two fields out of a config nobody has checked is how an operator ends
// up clearing an incident in a leftover journal path that no engine reads.
func TestReconcileMarginOpValidatesTheWholeConfigNotOnlyWhatItReads(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t, filepath.Join(t.TempDir(), "journal.db"))
	// Complete in both fields the act itself uses, and inconsistent in one it
	// does not: no endpoint serves the configured instruments' family.
	cfg.Venue.Endpoints = nil

	_, err := ReconcileMarginOp(cfg, clearanceAct())
	if err == nil {
		t.Fatal("ReconcileMarginOp accepted an inconsistent config")
	}
	if !errors.Is(err, ErrConfigFieldUnset) && !errors.Is(err, ErrFamilyUnrouted) {
		t.Fatalf("ReconcileMarginOp = %v, want the config's own refusal — "+
			"an error from the journal means the config was never checked", err)
	}
}

// TestReconcileMarginOpOpensTheCONFIGUREDJournal is the positive control, and it
// is built so that only the configured path can satisfy it.
//
// The obvious control — a valid config, expect a refusal that is not the
// config's — passed a mutant assigning `cfg.Account` to `JournalPath`: SQLite
// CREATES the file it is asked to open, so the act minted an empty journal named
// after the subaccount and refused with "never dispatched", which is exactly what
// the empty configured journal produces too. The two wirings gave the same answer.
//
// So the configured path is a DIRECTORY, which cannot be opened as a database,
// while the account is an ordinary name a stray file could be created under.
// Only a composition that opened the configured path fails this way.
func TestReconcileMarginOpOpensTheConfiguredJournal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := validConfig(t, dir) // the journal "path" is the directory itself

	_, err := ReconcileMarginOp(cfg, clearanceAct())
	if err == nil {
		t.Fatal("ReconcileMarginOp opened a directory as a journal")
	}
	if errors.Is(err, ErrConfigFieldUnset) || errors.Is(err, ErrFamilyUnrouted) {
		t.Fatalf("ReconcileMarginOp = %v, want a refusal from the JOURNAL — "+
			"a valid config was rejected as invalid", err)
	}
	if errors.Is(err, application.ErrMarginOpNotDispatched) {
		t.Fatalf("ReconcileMarginOp = %v — it replayed some OTHER journal; the "+
			"configured path is a directory and cannot be opened at all", err)
	}
}
