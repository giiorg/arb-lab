package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	apppair "github.com/giiorg/arb-lab/internal/app/pair"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// pairRun is `arblab pair run -config <file>`: ONE cross-venue package driven
// across TWO deployments — two §5.2 blocks, two journals, two reconcile-only
// boots, and on two clean verdicts the sequential drive.
//
// Thin on exerciseRun's contract: the exit precedence lives in
// pair.Report.ExitClass and this shell renders and maps it. The one code it adds
// to exercise's four is 4 — THE PAIR IS EXPOSED, which is deliberately distinct
// from 3's "a leg was dispatched and needs a human": after a 3 the record is in
// doubt, after a 4 a position is.
func pairRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pair run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "pair configuration file (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" {
		fmt.Fprintln(stderr, "pair run: -config is required")
		return 64
	}

	// The bootstrap wall reads the document only. The RUN builds its own from
	// BOTH sides' calibrations (D9), because a wall built here could only know
	// one side's venue frame and §5.3's bounds are measured in the venue's.
	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "pair run:", err)
		return 70
	}
	snap, err := loadPairConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "pair run:", err)
		return 70
	}

	report, err := apppair.Run(context.Background(), snap, observability.NewRegistry, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "pair run:", err)
		return 70
	}
	writePairReport(stdout, report)

	code := pairExitCode(report.ExitClass())
	if code == 70 {
		fmt.Fprintf(stderr, "pair run: unrecognised exit class %s\n", report.ExitClass())
	}
	return code
}

// pairExitCode maps the report's class to a process exit code.
//
// It is its own function rather than a switch inlined in pairRun, so a test can
// select the case it is about without standing up a run — exerciseRun inlines
// its switch and the corresponding test cannot reach it.
//
// Every member is named and none defaults, on reconcileRun's rule: a class added
// to the app package and not taught to this switch must not exit 0.
func pairExitCode(class apppair.ExitClass) int {
	switch class {
	case apppair.ExitClean:
		return 0
	case apppair.ExitOperator:
		return 1
	case apppair.ExitIncomplete:
		return 2
	case apppair.ExitExecution:
		return 3
	case apppair.ExitExposed:
		return 4
	}
	return 70
}

// writePairReport renders each side and then the pair. The pair line is last
// because it is the answer.
func writePairReport(w io.Writer, r apppair.Report) {
	for i := range r.Sides {
		fmt.Fprintf(w, "side %d — %s\n", i, r.Sides[i].Venue)
		writeReconcileVerdict(w, r.Sides[i].Boot)
		writeReconcileDiagnosis(w, r.Sides[i].Boot)
		writePairLeg(w, "open  ", r.Sides[i].Open)
		writePairLeg(w, "unwind", r.Sides[i].Unwind)
		if e := r.Sides[i].Exposure; e.Established && e.ID != "" {
			fmt.Fprintf(w, "  exposure: %s %s (state %s)\n", e.Side, e.Filled, e.State)
		} else if e.ID != "" {
			// An unestablished reading names WHY, because the number beside it
			// is not a measurement and must not be printed as one.
			fmt.Fprintf(w, "  exposure: NOT ESTABLISHED — %s\n", e.Because)
		}
		// Unconditional, and before any class-specific return below: precedence
		// reduces a process EXIT CODE, never the operator-visible facts. A ban
		// earned during dispatch must print even when exposure outranks it.
		writeRateLimitStatus(w, r.Sides[i].RateLimit)
	}
	if !r.Drove {
		fmt.Fprintf(w, "pair:     not driven: %v\n", r.Refusal)
		return
	}
	fmt.Fprintf(w, "pair:     %s\n", r.Verdict)
	if apppair.Exposed(r.Verdict) {
		fmt.Fprintln(w, "pair:     EXPOSURE IS LIVE AND UNBALANCED. §6.7's classifier cannot see a cross-venue")
		fmt.Fprintln(w, "pair:     pair (record §95 §4), so nothing will compensate this automatically.")
		fmt.Fprintln(w, "pair:     Recovery: `arblab reconcile run` on EACH side, then close by hand.")
	}
	if r.Refusal != nil {
		fmt.Fprintf(w, "pair:     %v\n", r.Refusal)
	}
}

// writePairLeg renders one leg, or nothing when it was never attempted.
//
// A nil report is silence rather than a line saying "none": the report already
// distinguishes attempted-and-refused from never-attempted, and printing a row
// for the second would invent an attempt.
func writePairLeg(w io.Writer, label string, leg *apppair.LegReport) {
	if leg == nil {
		return
	}
	if leg.Refusal != nil {
		fmt.Fprintf(w, "  %s %s: refused, nothing sent: %v\n", label, leg.ID, leg.Refusal)
		return
	}
	o := leg.Result.Outcome
	fmt.Fprintf(w, "  %s %s: %s venue_id=%q journaled=%t applied=%t",
		label, leg.ID, o.State, o.Ack.VenueOrderID, o.Journaled, o.Applied)
	if o.Ack.VenueOrderID != "" {
		// Printed only when there was an id to persist: on an id-less leg
		// "ack_journaled=false" would read as a failure that never had a write
		// to fail.
		fmt.Fprintf(w, " ack_journaled=%t ack_applied=%t", o.AckJournaled, o.AckApplied)
	}
	if o.AckConflict {
		fmt.Fprint(w, " ACK CONFLICT — venue id contradicts the store; new trading held")
	}
	if o.Cause != nil {
		fmt.Fprintf(w, " cause=%v", o.Cause)
	}
	fmt.Fprintln(w)
}

// loadPairConfig is loadReconcileConfig over the pair document.
func loadPairConfig(path string, wall *clock.Wall) (config.Snapshot[apppair.Config], error) {
	var zero config.Snapshot[apppair.Config]
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	store, err := config.New[apppair.Config](path, wall.Now(), data)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return store.Current(), nil
}
