package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// reconcileRun is `arblab reconcile run -config <file>`: §11's reconcile
// subcommand, and §7.5's boot performed once against a real account.
//
// # Thin, on capture verify's contract
//
// The exit-code precedence lives in reconcile.Report.ExitClass and is not
// re-decided here. That matters more than usual for this command: the mapping
// from a boot's findings to "what does a human do now" is the one decision this
// slice found was NOT the posture, and a shell that re-derived it would be a
// second reading of §7.5's clearance rule able to disagree with the first.
func reconcileRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "reconcile configuration file (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" {
		fmt.Fprintln(stderr, "reconcile run: -config is required")
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "reconcile run:", err)
		return 70
	}
	snap, err := loadReconcileConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "reconcile run:", err)
		return 70
	}
	// The venue clock frame the document states, applied to THE process clock.
	// One Wall, because the anchor §5.3 measures both of its bounds from is
	// stamped in the venue's frame off this calibration table.
	calibrated, err := clock.NewWall(snap.Value.Calibrations())
	if err != nil {
		fmt.Fprintln(stderr, "reconcile run:", err)
		return 70
	}

	report, err := appreconcile.Run(context.Background(), snap, calibrated,
		observability.NewRegistry(), stderr)
	if err != nil {
		// A boot that could not be PERFORMED, which is a different thing from one
		// that refused. See reconcile.Run.
		fmt.Fprintln(stderr, "reconcile run:", err)
		return 70
	}
	writeReconcileReport(stdout, report)

	switch report.ExitClass() {
	case appreconcile.ExitClean:
		return 0
	case appreconcile.ExitOperator:
		return 1
	case appreconcile.ExitIncomplete:
		return 2
	}
	// An exit class this shell does not know is not a success. Named rather than
	// defaulted for §3.6 rule 10's reason, one layer out of the analyzer's reach:
	// a class added to the app package and not to this switch must not exit 0.
	fmt.Fprintf(stderr, "reconcile run: unrecognised exit class %s\n", report.ExitClass())
	return 70
}

// writeReconcileReport renders what the boot found.
//
// Posture is printed FIRST and labelled with why it is what it is, because it is
// the number a reader will reach for as the verdict and it is not one: this
// binary declares all eight of §7.5's gates and produces four of them, so
// reconcile-only is the honest answer on a completely clean account.
func writeReconcileReport(w io.Writer, r appreconcile.Report) {
	fmt.Fprintf(w, "posture:  %s (this binary reports 4 of §7.5's 8 gates; the rest have no producer yet)\n", r.Posture)
	fmt.Fprintf(w, "verdict:  %s\n", r.ExitClass())
	if len(r.Latched) > 0 {
		fmt.Fprintf(w, "latched:  %v  <- operator clearance required; a retry will not lift these\n", r.Latched)
	} else {
		fmt.Fprintln(w, "latched:  none")
	}
	fmt.Fprintf(w, "blocking: %v\n", r.Blocking)
	if r.Storage != nil {
		fmt.Fprintf(w, "storage:  %v\n", r.Storage)
	}

	rec := r.Reconciliation
	fmt.Fprintf(w, "replayed: %d records, %d dispatched orders\n",
		rec.Recovery.Records, len(rec.Recovery.Dispatched))
	fmt.Fprintf(w, "joined:   %d orders, %d fills admitted, %d duplicates\n",
		rec.Join.Orders, rec.Join.Fills, rec.Join.Duplicates)
	if n := len(rec.Join.Absent); n > 0 {
		fmt.Fprintf(w, "absent:   %d order(s) the venue does not hold; %d resolved, %d unproved\n",
			n, rec.Resolved, len(rec.Unproved))
	}
	for _, u := range rec.Unproved {
		fmt.Fprintf(w, "  unproved %s: %s\n", u.Order, u.Reason)
	}
	for _, id := range rec.Stale {
		fmt.Fprintf(w, "  stale projection for %s\n", id)
	}
	for _, d := range rec.Assertion.Unexplained {
		fmt.Fprintf(w, "  unexplained %s: venue %s, ours %s, delta %s\n",
			d.Instrument, d.Observed.Decimal(), d.Expected.Decimal(), d.Unexplained.Decimal())
	}
	for _, s := range rec.Assertion.Foreign {
		fmt.Fprintf(w, "  foreign symbol %s\n", s)
	}
	for _, u := range rec.Assertion.Unasserted {
		fmt.Fprintf(w, "  unasserted %s: %s\n", u.Instrument, u.Reason)
	}
	if rec.Cause != nil {
		fmt.Fprintf(w, "cause:    %v\n", rec.Cause)
	}
}

// loadReconcileConfig reads and strictly loads the reconcile configuration.
//
// The bootstrap read: it happens before the boot sequence exists, because the
// sequence's own teardown bound is validated against §7.4's watchdog threshold
// and both numbers live in this file. See reconcile.Run.
func loadReconcileConfig(path string, wall *clock.Wall) (config.Snapshot[appreconcile.Config], error) {
	var zero config.Snapshot[appreconcile.Config]
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	store, err := config.New[appreconcile.Config](path, wall.Now(), data)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return store.Current(), nil
}
