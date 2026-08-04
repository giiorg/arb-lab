package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	appexercise "github.com/giiorg/arb-lab/internal/app/exercise"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// exerciseRun is `arblab exercise run -config <file>`: §9's synthetic
// spot↔perp cycle driver — one reconcile-only boot, and on a clean verdict,
// one two-leg parallel marketable-IOC package.
//
// Thin on reconcileRun's contract: the exit precedence lives in
// exercise.Report.ExitClass, and this shell only renders and maps it. The one
// code it adds to reconcile's three is 3 — the package was DISPATCHED and a
// leg needs a human, which is deliberately distinct from 2's "nothing was
// sent, retry may lift": after a 3 the recovery path is `arblab reconcile
// run`, never a re-run of this command with the same trade id.
func exerciseRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("exercise run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "exercise configuration file (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" {
		fmt.Fprintln(stderr, "exercise run: -config is required")
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "exercise run:", err)
		return 70
	}
	snap, err := loadExerciseConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "exercise run:", err)
		return 70
	}
	calibrated, err := clock.NewWall(snap.Value.OMS.Calibrations())
	if err != nil {
		fmt.Fprintln(stderr, "exercise run:", err)
		return 70
	}

	report, err := appexercise.Run(context.Background(), snap, calibrated,
		observability.NewRegistry(), stderr)
	if err != nil {
		fmt.Fprintln(stderr, "exercise run:", err)
		return 70
	}
	writeExerciseReport(stdout, report)

	switch report.ExitClass() {
	case appexercise.ExitClean:
		return 0
	case appexercise.ExitOperator:
		return 1
	case appexercise.ExitIncomplete:
		return 2
	case appexercise.ExitExecution:
		return 3
	}
	// Named rather than defaulted, reconcileRun's own rule: a class added to
	// the app package and not to this switch must not exit 0.
	fmt.Fprintf(stderr, "exercise run: unrecognised exit class %s\n", report.ExitClass())
	return 70
}

// writeExerciseReport renders the boot exactly as reconcile does — one
// renderer, so the two commands cannot describe the same boot differently —
// then the execute step's own half.
func writeExerciseReport(w io.Writer, r appexercise.Report) {
	writeReconcileReport(w, r.Boot)
	if !r.Executed {
		if r.Refusal != nil {
			fmt.Fprintf(w, "execute:  refused, nothing sent: %v\n", r.Refusal)
		} else {
			fmt.Fprintln(w, "execute:  did not run")
		}
		return
	}
	fmt.Fprintf(w, "execute:  %d leg(s) dispatched\n", r.Execution.Dispatched())
	for _, leg := range r.Execution.Legs {
		if leg.Err != nil {
			fmt.Fprintf(w, "  leg %s: refused, nothing sent: %v\n", leg.ID, leg.Err)
			continue
		}
		fmt.Fprintf(w, "  leg %s: %s venue_id=%q journaled=%t applied=%t",
			leg.ID, leg.Outcome.State, leg.Outcome.Ack.VenueOrderID,
			leg.Outcome.Journaled, leg.Outcome.Applied)
		if leg.Outcome.Ack.VenueOrderID != "" {
			// Printed only when there was an id to persist: on an id-less leg
			// "ack_journaled=false" would read as a failure that never had a
			// write to fail.
			fmt.Fprintf(w, " ack_journaled=%t ack_applied=%t",
				leg.Outcome.AckJournaled, leg.Outcome.AckApplied)
		}
		if leg.Outcome.AckConflict {
			fmt.Fprint(w, " ACK CONFLICT — venue id contradicts the store; new trading held")
		}
		if leg.Outcome.Cause != nil {
			fmt.Fprintf(w, " cause=%v", leg.Outcome.Cause)
		}
		fmt.Fprintln(w)
	}
	if r.Execution.HoldsNewTrading() {
		fmt.Fprintln(w, "execute:  HOLDS NEW TRADING — a sent leg's projection write failed; §6 step 7 applies until reconcile re-derives it")
	}
	// The POST-execute rate-limit state, printed only on the executed path: the
	// boot's own line above rendered the pre-execute snapshot, and when nothing
	// was executed the two are the same facts — a second line would say it
	// twice. When the fan-out itself earned a ban, this line is the only one
	// that can say so.
	writeRateLimitStatus(w, r.RateLimit)
}

// loadExerciseConfig is loadReconcileConfig over the exercise document.
func loadExerciseConfig(path string, wall *clock.Wall) (config.Snapshot[appexercise.Config], error) {
	var zero config.Snapshot[appexercise.Config]
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	store, err := config.New[appexercise.Config](path, wall.Now(), data)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return store.Current(), nil
}
