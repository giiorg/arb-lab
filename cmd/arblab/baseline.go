package main

import (
	"flag"
	"fmt"
	"io"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// baselineRecord is `arblab baseline record -config <file> -out <file>`: the
// operator half of task #179's two-act rule. It writes a NEW dated document
// under O_EXCL and prints the digest for the config pin; nothing governs until
// the operator edits `spot_baseline` in the deployment config — diagnosis and
// forgiveness must not share a keystroke, so there is no flag that does both.
func baselineRecord(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "reconcile configuration file (required)")
	outPath := fs.String("out", "", "path for the NEW baseline document (required; refused if it exists)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" || *outPath == "" {
		fmt.Fprintln(stderr, "baseline record: -config and -out are required")
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "baseline record:", err)
		return 70
	}
	snap, err := loadReconcileConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "baseline record:", err)
		return 70
	}
	// The venue clock frame, off the same calibration table the reconcile
	// boot uses — the document's ObservedAt is stated in it.
	calibrated, err := clock.NewWall(snap.Value.Calibrations())
	if err != nil {
		fmt.Fprintln(stderr, "baseline record:", err)
		return 70
	}

	recorded, err := appreconcile.RecordBaseline(snap.Value, calibrated, *outPath)
	if err != nil {
		// Every gauntlet refusal lands here by design: the protocol refuses
		// rather than waits in every arm, and the sentinel text says what to
		// do next (wait for terminal, resolve the absence, retry the sweep).
		fmt.Fprintln(stderr, "baseline record:", err)
		return 1
	}

	fmt.Fprintf(stdout, "recorded: %s\n", recorded.Path)
	fmt.Fprintf(stdout, "sha256:   %s\n", recorded.SHA256)
	fmt.Fprintf(stdout, "observed: %s (venue clock)\n", recorded.Document.ObservedAt)
	fmt.Fprintf(stdout, "chain:    seq %d\n", recorded.Document.ChainSeq)
	for _, b := range recorded.Baselines {
		fmt.Fprintf(stdout, "baseline: %s %s = %s (balance minus ours; recomputed by the verifying loader)\n",
			b.Venue, b.Asset, b.Amount.Decimal())
	}
	if recorded.PriorError != nil {
		fmt.Fprintf(stdout, "prior:    could not load the configured baseline for comparison: %v\n", recorded.PriorError)
	}
	for _, a := range recorded.Absorbed {
		fmt.Fprintf(stdout, "absorbs:  %s: %s -> %s (delta %s folded into \"not ours\")\n",
			a.Asset, a.Prior.Decimal(), a.New.Decimal(), a.Delta.Decimal())
	}
	fmt.Fprintln(stdout, "next:     pin it — set spot_baseline = {path, sha256} in the deployment config (the second act; nothing governs until then)")
	return 0
}
