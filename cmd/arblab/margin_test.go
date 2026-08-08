package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestMarginDispatchRoutesAndNamesEveryRequiredFlag: the subcommand is reachable,
// and its usage refusal names all five rather than only the first.
//
// The five are named together deliberately. Core §7.3's audit is who / what /
// when, and an operator who is told about `-config` alone will supply it, be told
// about `-op`, supply that, and discover `-actor` on the third attempt — during
// a §9 incident. There is no default for any of them (record §82 §2), so the
// refusal has to be the whole list.
func TestMarginDispatchRoutesAndNamesEveryRequiredFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := dispatch([]string{"margin", "reconcile"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(margin reconcile) = %d, want 64 (EX_USAGE)", got)
	}
	// The assertion is the COMMAND's own message, not the flag names. A mutant
	// that broke the routing survived a test asserting on "-config": the
	// fall-through prints the usage block, which advertises every flag this
	// command takes — so the two outputs were indistinguishable to the check, and
	// both return 64. Only text nothing else emits separates routed from
	// unrouted.
	if !strings.Contains(stderr.String(), "-config, -op, -actor, -at and -note are all required") {
		t.Errorf("stderr = %q — margin reconcile was not routed; this is the usage fall-through",
			stderr.String())
	}

	// An unrouted subcommand still falls through to usage, so the new arm did not
	// swallow the fall-through beneath it.
	stdout.Reset()
	stderr.Reset()
	if got := dispatch([]string{"margin", "nonesuch"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(margin nonesuch) = %d, want 64", got)
	}
	if !strings.Contains(stderr.String(), "arblab margin reconcile") {
		t.Errorf("usage does not advertise the subcommand: %q", stderr.String())
	}
}

// TestMarginReconcileRequiresEachFlagSEPARATELY is the per-flag claim, and it
// exists because the all-empty invocation above cannot make it.
//
// With every flag missing, the FIRST check trips and the other four are dead
// code — a mutant deleting the `-note` requirement survived on exactly that. So
// each case supplies a complete invocation and removes ONE flag, which is the
// only shape under which each check is the one doing the work.
//
// Each of the five is required for its own reason (record §82 §2): core §7.3's
// who / what / when have no honest default, the note is the only field saying
// what was reconciled, and the config is what binds the act to the journal this
// deployment actually boots against.
func TestMarginReconcileRequiresEachFlagSeparately(t *testing.T) {
	complete := map[string]string{
		"-config": "/nonexistent/reconcile.json",
		"-op":     "MOP-1",
		"-actor":  "gb",
		"-at":     "2026-08-08T11:30:00Z",
		"-note":   "counted the float",
	}
	for missing := range complete {
		t.Run("without "+missing, func(t *testing.T) {
			args := []string{"margin", "reconcile"}
			for flag, value := range complete {
				if flag == missing {
					continue
				}
				args = append(args, flag, value)
			}
			var stdout, stderr bytes.Buffer
			if got := dispatch(args, &stdout, &stderr); got != 64 {
				t.Fatalf("dispatch without %s = %d, want 64 (EX_USAGE) — the flag is not required", missing, got)
			}
			if !strings.Contains(stderr.String(), "are all required") {
				t.Fatalf("stderr = %q, want the required-flag refusal", stderr.String())
			}
		})
	}
}

// TestMarginReconcileRefusesAMalformedInstantAsUsage is the one parse this
// command does, and the exit code is the claim.
//
// A malformed `-at` is the operator's typing, so it is EX_USAGE (64) rather than
// the refusal code (1) that means "the log says this act has no subject". Left
// to travel deeper it would arrive as the zero instant and be refused by the
// record's own validation — a correct refusal, reported as though the journal had
// something to say about it.
//
// Every other flag is present and well-formed, so the only thing this can fail
// on is the instant. The config path does not exist, which is what makes the
// assertion sharp: reaching the config at all would produce a different code.
func TestMarginReconcileRefusesAMalformedInstantAsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{
		"margin", "reconcile",
		"-config", "/nonexistent/reconcile.json",
		"-op", "MOP-1",
		"-actor", "gb",
		"-at", "yesterday",
		"-note", "counted the float",
	}, &stdout, &stderr)
	if code != 64 {
		t.Fatalf("dispatch with a malformed -at = %d, want 64 (EX_USAGE)", code)
	}
	if !strings.Contains(stderr.String(), "RFC3339") {
		t.Errorf("stderr = %q, want the instant's format named", stderr.String())
	}
}

// TestMarginReconcileUsageStatesTheEngineMustBeStopped is a documentation claim
// asserted as a test, because it is the one operational fact a user cannot infer
// from the flags.
//
// The journal is single-writer, so the command refuses at open against a live
// engine — and the act takes effect at the next boot, because nothing lifts a
// latch inside a running process (record §82 §4). Both follow from the design and
// neither is visible in a flag list.
func TestMarginReconcileUsageStatesTheEngineMustBeStopped(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dispatch([]string{"margin"}, &stdout, &stderr)
	got := stderr.String()
	for _, want := range []string{"ENGINE MUST BE STOPPED", "next boot"} {
		if !strings.Contains(got, want) {
			t.Errorf("usage does not state %q", want)
		}
	}
}
