package main

import (
	"bytes"
	"strings"
	"testing"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/boot"
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

// TestReconcileReportNamesTheLatchAndDisclaimsThePosture is the operator-facing
// half of this slice's finding.
//
// The fixture's posture is PostureReconcileOnly — which is what a completely
// clean run of this binary also reports — so the rendered report has to make the
// difference legible, and the only thing that does is the latch. A report that
// printed the posture and stopped would be identical for both.
func TestReconcileReportNamesTheLatchAndDisclaimsThePosture(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateVenueJoined, boot.GateAccountPosture},
		Latched:  []boot.Gate{boot.GateVenueJoined},
	})
	got := buf.String()
	for _, want := range []string{
		"GateVenueJoined",    // the latch is named
		"operator clearance", // and what it means
		"no producer yet",    // the posture carries its own disclaimer
		appreconcile.ExitOperator.String(),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}

	// The clean case says "none" rather than printing an empty slice, because
	// "latched:  []" is the line an operator skims past.
	buf.Reset()
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateAccountPosture},
	})
	if !strings.Contains(buf.String(), "latched:  none") {
		t.Errorf("clean report does not say the latch set is empty:\n%s", buf.String())
	}
}
