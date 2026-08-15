package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apppair "github.com/giiorg/arb-lab/internal/app/pair"
	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

func TestPairRunExitCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		class apppair.ExitClass
		want  int
	}{
		{apppair.ExitClean, 0},
		{apppair.ExitOperator, 1},
		{apppair.ExitIncomplete, 2},
		{apppair.ExitExecution, 3},
		{apppair.ExitExposed, 4},
	}
	seen := map[int]apppair.ExitClass{}
	for _, tc := range cases {
		got := pairExitCode(tc.class)
		if got != tc.want {
			t.Errorf("pairExitCode(%s) = %d, want %d", tc.class, got, tc.want)
		}
		// Two classes sharing a code would make the exit status unable to tell
		// an exposed pair from a lost record, which is the whole reason 4 exists.
		if other, dup := seen[got]; dup {
			t.Errorf("%s and %s both exit %d", tc.class, other, got)
		}
		seen[got] = tc.class
	}
	// A class this switch has not been taught must NOT exit 0 — reconcileRun's
	// own rule, and the reason the switch names every member instead of
	// defaulting.
	if got := pairExitCode(apppair.ExitClass(99)); got == 0 {
		t.Error("an unrecognised exit class mapped to 0")
	}
}

func TestPairRunRefusesWithoutAConfig(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if got := pairRun(nil, &out, &errOut); got != 64 {
		t.Errorf("pairRun(no args) = %d, want 64", got)
	}
	if !strings.Contains(errOut.String(), "-config is required") {
		t.Errorf("stderr = %q, want the missing-flag diagnosis", errOut.String())
	}
}

func TestPairRunRefusesADocumentItCannotRead(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	missing := filepath.Join(t.TempDir(), "no-such-pair.json")
	if got := pairRun([]string{"-config", missing}, &out, &errOut); got != 70 {
		t.Errorf("pairRun(unreadable config) = %d, want 70", got)
	}
	if errOut.Len() == 0 {
		t.Error("stderr is empty; a refusal must name the file")
	}
}

// TestPairRunIsRoutedFromTheDispatcher: the arm exists and reaches pairRun
// rather than the usage block. Without it, `arblab pair run` would print usage
// and exit 64 while every unit test above passed.
func TestPairRunIsRoutedFromTheDispatcher(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	// No -config, so the routed command refuses with 64 AND its own message —
	// which is what tells this apart from the usage block's 64.
	if got := dispatch([]string{"pair", "run"}, &out, &errOut); got != 64 {
		t.Fatalf("dispatch(pair run) = %d, want 64", got)
	}
	if !strings.Contains(errOut.String(), "pair run: -config is required") {
		t.Errorf("stderr = %q; the dispatcher did not reach pairRun", errOut.String())
	}
}

// TestUsageDocumentsTheExposedExitCode. Exit 4 is the one an operator meets
// with a live unbalanced position and no automatic remedy, so the usage block
// has to say that nothing compensates it.
func TestUsageDocumentsTheExposedExitCode(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"arblab pair run", "EXPOSED", "compensates a 4"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage does not mention %q", want)
		}
	}
}

// exposedPairReport is a run that ended half-open with a ban earned on the way.
func exposedPairReport(banned ratelimit.Snapshot) apppair.Report {
	var r apppair.Report
	r.Drove = true
	r.Verdict = apppair.VerdictHalfOpen
	r.Sides[0] = apppair.SideReport{
		Venue: ids.Venue("binance"),
		Boot: appreconcile.Report{
			Posture: boot.PostureReconcileOnly, Blocking: []boot.Gate{boot.GateWriterAuthority},
			RateLimit: ratelimit.Snapshot{Venue: ids.Venue("binance")},
		},
	}
	r.Sides[1] = apppair.SideReport{
		Venue: ids.Venue("bybit"),
		Boot: appreconcile.Report{
			Posture: boot.PostureReconcileOnly, Blocking: []boot.Gate{boot.GateWriterAuthority},
			RateLimit: ratelimit.Snapshot{Venue: ids.Venue("bybit")},
		},
		RateLimit: banned,
	}
	return r
}

// TestPairReportPrintsADispatchTimeBanEvenWhenExposureOutranksIt is the
// precedence claim stated where it matters: ExitClass reduces a process EXIT
// CODE, not the operator-visible facts.
//
// The pair is exposed, so ExitClass answers ExitExposed and the ban loses the
// ranking — and the ban is the reason the next run will be refused by the
// venue, so it must still be on the page. The boot snapshots are deliberately
// CLEAN: a ban present at boot would print from the boot's own renderer and
// this test would pass without the post-drive snapshot being read at all.
func TestPairReportPrintsADispatchTimeBanEvenWhenExposureOutranksIt(t *testing.T) {
	t.Parallel()
	until := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	banned := ratelimit.Snapshot{
		Venue: ids.Venue("bybit"), Bans: 1,
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyBan,
	}
	rep := exposedPairReport(banned)
	if rep.ExitClass() != apppair.ExitExposed {
		t.Fatalf("ExitClass = %s, want ExitExposed; the ban must be the LOSING class here", rep.ExitClass())
	}

	var buf bytes.Buffer
	writePairReport(&buf, rep)
	got := buf.String()
	if !strings.Contains(got, "banned this IP 1 time(s)") {
		t.Errorf("report omitted the dispatch-time ban:\n%s", got)
	}
	if !strings.Contains(got, "EXPOSURE IS LIVE AND UNBALANCED") {
		t.Errorf("report omitted the exposure warning:\n%s", got)
	}
	if !strings.Contains(got, "reconcile run") {
		t.Errorf("report omitted the recovery path:\n%s", got)
	}
}

// TestPairReportNamesAnUnestablishedExposureRatherThanPrintingItsNumber. A leg
// whose reading did not finish carries a quantity that is not a measurement,
// and printing it beside the established ones would let an operator read it as
// one.
func TestPairReportNamesAnUnestablishedExposureRatherThanPrintingItsNumber(t *testing.T) {
	t.Parallel()
	rep := exposedPairReport(ratelimit.Snapshot{Venue: ids.Venue("bybit")})
	rep.Verdict = apppair.VerdictIndeterminate
	rep.Sides[0].Exposure = apppair.LegExposure{
		Venue: "binance",
		ID:    ids.ClientOrderID("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-00"),
		// Not established, and the reason is the only thing worth printing.
		Because: `state "submitted" is not terminal`,
	}

	var buf bytes.Buffer
	writePairReport(&buf, rep)
	got := buf.String()
	if !strings.Contains(got, "NOT ESTABLISHED") {
		t.Errorf("report did not mark the unestablished leg:\n%s", got)
	}
	if !strings.Contains(got, "is not terminal") {
		t.Errorf("report did not carry the reason:\n%s", got)
	}
}

// TestPairReportSaysNothingAboutALegThatWasNeverAttempted. A nil leg report is
// silence, because the alternative — a row reading "none" — is an attempt this
// run did not make.
func TestPairReportSaysNothingAboutALegThatWasNeverAttempted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePairLeg(&buf, "unwind", nil)
	if buf.Len() != 0 {
		t.Errorf("a nil leg rendered %q, want nothing", buf.String())
	}
	// The control: an attempted-and-refused leg DOES print, and says nothing
	// was sent.
	writePairLeg(&buf, "unwind", &apppair.LegReport{
		ID: ids.ClientOrderID("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-01"),
		// A refusal is the one case where a leg exists and no order does.
		Refusal: errors.New("below the min quantity"),
	})
	if !strings.Contains(buf.String(), "refused, nothing sent") {
		t.Errorf("a refused leg rendered %q; it must say nothing was sent", buf.String())
	}
}

// TestPairReportOfANotDrivenRunNamesTheRefusal. The gate refused, so there is
// no verdict to print and the operator needs the reason instead.
func TestPairReportOfANotDrivenRunNamesTheRefusal(t *testing.T) {
	t.Parallel()
	// Side 1's snapshot carries a fold miss, so it has something to report:
	// writeRateLimitStatus is silent on an all-zero snapshot, and asserting a
	// line from one would be asserting the renderer's silence.
	rep := exposedPairReport(ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{{Name: "order_realtime", FoldMisses: 2}},
	})
	rep.Drove = false
	rep.Verdict = apppair.VerdictUnopened
	rep.Refusal = errors.New("side 1 (bybit) boot verdict is ExitIncomplete")

	var buf bytes.Buffer
	writePairReport(&buf, rep)
	got := buf.String()
	if !strings.Contains(got, "not driven") {
		t.Errorf("report does not say the run was not driven:\n%s", got)
	}
	if !strings.Contains(got, "ExitIncomplete") {
		t.Errorf("report does not carry the refusal:\n%s", got)
	}
	// The side's rate-limit line still prints: a refused drive is not a reason
	// to withhold what the boots observed.
	if n := strings.Count(got, "ratelimit:"); n != 1 {
		t.Errorf("report has %d rate-limit lines, want the one side that has something to report:\n%s", n, got)
	}
}

// TestExampleDocumentValidates loads the committed example and validates it.
//
// This is the check that stops the example rotting silently: a document nobody
// loads is prose, and this one is the shape an operator copies before a live
// demo run. It resolves NO secret — the document holds refs, and resolving them
// is a boot phase rather than validation.
//
// # The two journal paths are rewritten, and the reason is a real requirement
//
// sameJournal resolves each journal's PARENT directory, so a document whose
// `./run/pair/` does not exist is refused — by design, because a path that
// cannot be resolved cannot be told apart from the other side's. The example
// ships with the relative paths an operator will actually use, and this test
// runs from cmd/arblab where that directory does not exist, so it points both
// at a directory that does. Everything else in the document is exactly as
// committed. The runbook's first step is the operator's half of the same fact:
// `mkdir -p ./run/pair`.
func TestExampleDocumentValidates(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../configs/pair.demo.example.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	dir := t.TempDir()
	doc := string(data)
	for _, side := range []string{"binance", "bybit"} {
		want := "./run/pair/" + side + ".db"
		if !strings.Contains(doc, want) {
			t.Fatalf("the example no longer declares %s; this rewrite is silently doing nothing", want)
		}
		doc = strings.ReplaceAll(doc, want, filepath.Join(dir, side+".db"))
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	store, err := config.New[apppair.Config]("pair.demo.example.json", wall.Now(), []byte(doc))
	if err != nil {
		t.Fatalf("the committed example does not load: %v", err)
	}
	if err := store.Current().Value.Validate(); err != nil {
		t.Fatalf("the committed example does not validate: %v", err)
	}
}
