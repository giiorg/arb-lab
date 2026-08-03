package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
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

// TestWriteRateLimitStatusDrawsSection76sLine: a ban is an OPERATOR ALERT, a
// throttle pause is information, and a run with no venue traffic prints
// nothing. The three cases are asserted against each other's markers — a
// renderer that alerted on every pause would pass the first and fail the
// second, and one that printed a standing "nothing" line would fail the third.
func TestWriteRateLimitStatusDrawsSection76sLine(t *testing.T) {
	until := time.Date(2026, 8, 3, 14, 7, 9, 0, time.UTC)

	var buf bytes.Buffer
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue: ids.Venue("binance"), Bans: 1,
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyBan,
	})
	got := buf.String()
	for _, want := range []string{
		"OPERATOR ALERT",
		"binance",
		"banned this IP 1 time(s)",
		"do not retry",
		"paused until 2026-08-03T14:07:09Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ban line does not contain %q:\n%s", want, got)
		}
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyThrottle,
	})
	got = buf.String()
	if !strings.Contains(got, "paused until 2026-08-03T14:07:09Z") ||
		!strings.Contains(got, "PenaltyThrottle") {
		t.Errorf("throttle line does not name the pause and its kind:\n%s", got)
	}
	if strings.Contains(got, "OPERATOR ALERT") {
		t.Errorf("a throttle pause is §7.6's hard pause, NOT its alert:\n%s", got)
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{})
	if buf.Len() != 0 {
		t.Errorf("zero snapshot printed %q — no venue traffic means no line", buf.String())
	}
}

// TestReconcileReportCarriesTheBanAlert: the alert through the full report
// renderer, once — the count matters, because the exercise renderer composes
// this one and adds its own call, and "printed" alone cannot see a double.
// Bans is 2 here and 1 in the unit test above, so a renderer gating on an
// exact count fails one of the pair.
func TestReconcileReportCarriesTheBanAlert(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateAccountPosture},
		RateLimit: ratelimit.Snapshot{
			Venue: ids.Venue("binance"), Bans: 2,
			Paused: true, PausedUntil: time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC),
			PauseKind: ratelimit.PenaltyBan,
		},
	})
	got := buf.String()
	if n := strings.Count(got, "OPERATOR ALERT"); n != 1 {
		t.Errorf("report contains %d alert lines, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "banned this IP 2 time(s)") {
		t.Errorf("report does not carry the count:\n%s", got)
	}
	if !strings.Contains(got, appreconcile.ExitOperator.String()) {
		t.Errorf("a banned report's verdict is not ExitOperator:\n%s", got)
	}
}
