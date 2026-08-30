package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestReconcileDispatchRoutesAndRefusesAMissingConfig: the subcommand is reachable
// and its usage refusal is EX_USAGE rather than a run that failed.
//
// The two are worth separating because they are what an operator's shell script
// branches on: 64 means the invocation was wrong, and 0/1/2 are verdicts about an
// account.
func TestReconcileDispatchRoutesAndRefusesAMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := dispatch([]string{"reconcile", "run"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(reconcile run) = %d, want 64 (EX_USAGE)", got)
	}
	if !strings.Contains(stderr.String(), "-config is required") {
		t.Errorf("stderr = %q, want the missing flag named", stderr.String())
	}

	// And an unrouted subcommand still falls through to usage, so the new `if`
	// above did not swallow the arm below it.
	stdout.Reset()
	stderr.Reset()
	if got := dispatch([]string{"reconcile", "nonesuch"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(reconcile nonesuch) = %d, want 64", got)
	}
	if !strings.Contains(stderr.String(), "arblab reconcile run") {
		t.Errorf("usage does not advertise the subcommand: %q", stderr.String())
	}
}
