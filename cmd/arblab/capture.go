package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	appcapture "github.com/giiorg/arb-lab/internal/app/capture"
	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// loadConfig reads and strictly loads the capture configuration. The wall
// instant is the load's audit timestamp; the returned snapshot's revision hash
// is what every Open record carries as config provenance.
func loadConfig(path string, wall *clock.Wall) (config.Snapshot[appcapture.Config], error) {
	var zero config.Snapshot[appcapture.Config]
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	store, err := config.New[appcapture.Config](path, wall.Now(), data)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return store.Current(), nil
}

// captureRun is `arblab capture run -config <file>`: capture every configured
// source until interrupted.
func captureRun(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "capture configuration file (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" {
		fmt.Fprintln(stderr, "capture run: -config is required")
		return 64
	}

	// One Wall for the process: Mono's baseline is per-Wall, and every record
	// this run stamps must sit on one timeline.
	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "capture run:", err)
		return 1
	}
	snap, err := loadConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "capture run:", err)
		return 1
	}

	tool, gitSHA := buildInfo()
	fmt.Fprintf(stderr, "capture run: %d sources under %s (config %s, build %s); interrupt to stop\n",
		len(snap.Value.Sources)+len(snap.Value.RestSources), snap.Value.Root, snap.Rev.HashString()[:12], gitSHA)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := appcapture.Run(ctx, snap, wall, observability.NewRegistry(),
		appcapture.Build{Tool: tool, GitSHA: gitSHA}); err != nil {
		fmt.Fprintln(stderr, "capture run:", err)
		return 1
	}
	fmt.Fprintln(stderr, "capture run: stopped cleanly")
	return 0
}

// captureVerify is `arblab capture verify`: a thin shell over capture.Verify.
//
// Thin is the contract (§36): the exit-code precedence lives in
// Report.ExitClass and is not re-decided here — this command's own work is
// flag parsing and the report's formatting, nothing else.
func captureVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "capture configuration file (required)")
	fromArg := fs.String("from", "", "start of the claimed range, RFC3339 (required)")
	toArg := fs.String("to", "", "end of the claimed range, RFC3339 (required)")
	rootArg := fs.String("root", "", "archive root (default: the config's root)")
	minAlive := fs.Uint("min-alive", 0, "coverage threshold in whole percent (default: the config's)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *cfgPath == "" || *fromArg == "" || *toArg == "" {
		fmt.Fprintln(stderr, "capture verify: -config, -from and -to are required")
		return 64
	}
	from, err := time.Parse(time.RFC3339, *fromArg)
	if err != nil {
		fmt.Fprintln(stderr, "capture verify: -from:", err)
		return 64
	}
	to, err := time.Parse(time.RFC3339, *toArg)
	if err != nil {
		fmt.Fprintln(stderr, "capture verify: -to:", err)
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "capture verify:", err)
		return 1
	}
	snap, err := loadConfig(*cfgPath, wall)
	if err != nil {
		fmt.Fprintln(stderr, "capture verify:", err)
		return 1
	}

	// The rotation period and Alive interval always come from the config —
	// supplying them is what makes the check a comparison of the archive
	// against the config rather than against itself (§36o). Root and the
	// threshold are overridable: verifying a COPY of the archive, or asking a
	// stricter question of the same one, changes neither of those two.
	vcfg := snap.Value.Verify(from, to)
	if *rootArg != "" {
		vcfg.Root = *rootArg
	}
	if *minAlive != 0 {
		vcfg.MinAlivePercent = *minAlive
	}

	rep, err := enginecapture.Verify(vcfg)
	if err != nil {
		fmt.Fprintln(stderr, "capture verify:", err)
		return 1
	}
	formatReport(stdout, rep, vcfg)
	return exitCode(rep.ExitClass())
}

// exitCode spells a FindingClass as a process exit code. A renaming, not a
// decision: the integrity-over-coverage precedence is Report.ExitClass's and
// must not be re-decided here. The switch is exhaustive (§3.6 rule 10), so a
// class added later refuses to compile until somebody assigns its number.
func exitCode(c enginecapture.FindingClass) int {
	switch c {
	case enginecapture.ClassIntegrity:
		return 1
	case enginecapture.ClassCoverage:
		return 2
	case enginecapture.ClassUnspecified:
		return 0
	}
	return 0
}

// formatReport renders a verify report for an operator.
//
// The two axes are printed SEPARATELY and never summed into a "problems"
// count (§36i): the verdict column says what a replay of each window would
// get, and the findings section says whether to trust the archive at all. One
// number covering both would send an operator to widen a capture window when
// what they need to do is stop trusting the file.
func formatReport(w io.Writer, rep *enginecapture.Report, cfg enginecapture.VerifyConfig) {
	fmt.Fprintf(w, "capture verify %s\nrange %s .. %s, alive threshold %d%%\n\n",
		cfg.Root,
		rep.From.UTC().Format(time.RFC3339), rep.To.UTC().Format(time.RFC3339),
		cfg.MinAlivePercent)

	if len(rep.Windows) == 0 {
		fmt.Fprintln(w, "no streams found in the archive for this range")
	}
	for _, wr := range rep.Windows {
		fmt.Fprintf(w, "%-10s %s %s  records %d, data %d, alive %d/%d",
			wr.Verdict,
			wr.Window.UTC().Format(time.RFC3339),
			wr.Stream,
			wr.Records, wr.DataRecords, wr.AliveTicks, wr.AliveExpected)
		if wr.RestRecords > 0 {
			fmt.Fprintf(w, ", rest %d", wr.RestRecords)
		}
		if wr.DataRecords > 0 {
			fmt.Fprintf(w, ", seq %d..%d", wr.FirstSeq, wr.LastSeq)
		}
		if len(wr.Gaps) > 0 {
			fmt.Fprintf(w, ", gaps %s", formatGaps(wr.Gaps))
		}
		fmt.Fprintln(w)
	}

	integrity, coverage := rep.Counts()
	fmt.Fprintf(w, "\nfindings: %d integrity, %d coverage\n", integrity, coverage)
	for _, f := range rep.Findings {
		fmt.Fprintf(w, "  %s\n", f)
	}

	switch rep.ExitClass() {
	case enginecapture.ClassIntegrity:
		fmt.Fprintln(w, "\nverdict: INTEGRITY — the archive contradicts itself; do not trust this range")
	case enginecapture.ClassCoverage:
		fmt.Fprintln(w, "\nverdict: COVERAGE — the archive is honest and thin over this range")
	case enginecapture.ClassUnspecified:
		fmt.Fprintln(w, "\nverdict: clean")
	}
}

// formatGaps renders a window's gap tally deterministically, sorted by cause.
func formatGaps(gaps map[enginecapture.Cause]uint64) string {
	causes := make([]string, 0, len(gaps))
	for c := range gaps {
		causes = append(causes, string(c))
	}
	sort.Strings(causes)
	out := ""
	for i, c := range causes {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s×%d", c, gaps[enginecapture.Cause(c)])
	}
	return out
}
