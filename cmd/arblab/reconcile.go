package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
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
// binary declares all eight of §7.5's gates and produces five of them, so
// reconcile-only is the honest answer on a completely clean account.
func writeReconcileReport(w io.Writer, r appreconcile.Report) {
	writeReconcileVerdict(w, r)
	writeRateLimitStatus(w, r.RateLimit)
	writeReconcileDiagnosis(w, r)
}

// writeReconcileVerdict is the report's judgement half: posture, exit class,
// and what latched. Split from the diagnosis so the exercise renderer can
// place the rate-limit section once, after its execute half.
func writeReconcileVerdict(w io.Writer, r appreconcile.Report) {
	fmt.Fprintf(w, "posture:  %s (this binary reports 5 of §7.5's 8 gates; the rest have no producer yet)\n", r.Posture)
	fmt.Fprintf(w, "verdict:  %s\n", r.ExitClass())
	if len(r.Latched) > 0 {
		fmt.Fprintf(w, "latched:  %v  <- operator clearance required; a retry will not lift these\n", r.Latched)
	} else {
		fmt.Fprintln(w, "latched:  none")
	}
}

func writeReconcileDiagnosis(w io.Writer, r appreconcile.Report) {
	fmt.Fprintf(w, "blocking: %v\n", r.Blocking)
	if r.Storage != nil {
		fmt.Fprintf(w, "storage:  %v\n", r.Storage)
	}
	// §4.1's verdict, and the two halves are not interchangeable: a cause means
	// the posture could not be ESTABLISHED (fix the wiring or retry), and a
	// failure list means it was read and the ACCOUNT is wrong. Every failed
	// check is printed rather than the first, so an operator acts once — a
	// checker that named one violation per boot costs a round trip per check.
	if r.PostureCause != nil {
		fmt.Fprintf(w, "posture!: could not be established: %v\n", r.PostureCause)
	}
	for _, f := range r.AccountPosture.Failures {
		fmt.Fprintf(w, "  posture %s\n", f)
	}

	rec := r.Reconciliation
	fmt.Fprintf(w, "replayed: %d records, %d dispatched orders\n",
		rec.Recovery.Records, len(rec.Recovery.Dispatched))
	fmt.Fprintf(w, "joined:   %d orders, %d fills admitted, %d duplicates\n",
		rec.Join.Orders, rec.Join.Fills, rec.Join.Duplicates)
	// The two join findings that LATCH, directly under the `joined:` line they
	// qualify and in joinOutcome's own ladder order — most restrictive first,
	// which is that ladder's rule and now this report's. The absence block
	// below is the same gate's retryable half (§5.3 concludes most of them), so
	// it reads after: an operator triaging a held boot wants the orders no
	// retry will clear before the ones the next boot may.
	//
	// Until task #242 the renderer printed NEITHER, while enumerating every
	// other finding the diagnosis carries. A boot could therefore halt with
	// `blocking: [GateVenueJoined]` and name no order and no reason — held, and
	// not told what for. The rule the omission broke is the one this whole
	// slice is about: a finding nobody reads is a finding nobody acts on.
	for _, id := range rec.Join.Unattributed {
		fmt.Fprintf(w, "  unattributed %s: the venue's cumulative disagrees with the executions this join could attribute\n", id)
	}
	for _, id := range rec.Join.Contradicted {
		fmt.Fprintf(w, "  contradicted %s: the venue and this system hold lifecycle states SM §1 has no edge between\n", id)
	}
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
	for _, d := range rec.Assertion.SpotUnexplained {
		// Beside the derivative deltas rather than after the sections below:
		// the two are the SAME finding — §7.5's unexplained delta — on the
		// leg with no position surface, and an operator triaging a latch
		// reads them together (task #179, record §70c).
		fmt.Fprintf(w, "  spot unexplained %s %s: balance-minus-baseline %s, ours %s, delta %s\n",
			d.Venue, d.Asset, d.Observed.Decimal(), d.Expected.Decimal(), d.Unexplained.Decimal())
	}
	for _, s := range rec.Assertion.Foreign {
		fmt.Fprintf(w, "  foreign symbol %s\n", s)
	}
	for _, u := range rec.Assertion.Unasserted {
		fmt.Fprintf(w, "  unasserted %s: %s\n", u.Instrument, u.Reason)
	}
	for _, u := range rec.Assertion.SpotUnconcluded {
		fmt.Fprintf(w, "  spot unconcluded %s %s: %s (re-reportable; a further attempt may pin it)\n",
			u.Venue, u.Asset, u.Reason)
	}
	if rec.Cause != nil {
		fmt.Fprintf(w, "cause:    %v\n", rec.Cause)
	}
}

// writeRateLimitStatus renders §7.6's rate-limit facts, and it draws that
// section's one line in the output: a ban is an OPERATOR ALERT, a throttle
// pause is information. A zero snapshot — no venue client was ever built, so no
// venue traffic happened — prints nothing at all, because a standing line that
// is almost always "nothing" is how an operator learns to skip the one that
// is not.
func writeRateLimitStatus(w io.Writer, s ratelimit.Snapshot) {
	for _, l := range s.Limits {
		if l.FoldMisses > 0 {
			fmt.Fprintf(w, "ratelimit: %s %s: %d response(s) carried no usable budget reading this run (§7.6 fold-health — venue-side corrections were missed)\n",
				s.Venue, l.Name, l.FoldMisses)
		}
	}
	for _, l := range s.Limits {
		if l.Reserved > 0 {
			fmt.Fprintf(w, "ratelimit: %s %s: %d unit(s) reserved for an in-flight package\n",
				s.Venue, l.Name, l.Reserved)
		}
	}
	if s.Bans > 0 {
		fmt.Fprintf(w, "ratelimit: %s banned this IP %d time(s) (418/IP ban)  <- §7.6 OPERATOR ALERT: do not retry — repeat violations escalate the ban",
			s.Venue, s.Bans)
		if s.Paused {
			fmt.Fprintf(w, "; paused until %s", s.PausedUntil.UTC().Format(time.RFC3339))
		}
		fmt.Fprintln(w)
		return
	}
	if s.Paused {
		fmt.Fprintf(w, "ratelimit: %s paused until %s (%s — a hard pause, not an operator matter)\n",
			s.Venue, s.PausedUntil.UTC().Format(time.RFC3339), s.PauseKind)
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
