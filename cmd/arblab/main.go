// Command arblab is the ops CLI §11 names: capture, replay, reconcile, key
// checks. What exists today is capture — `arblab capture run` and `arblab
// capture verify` — because milestone 2 is calendar-bound on funding history
// and capture is the only work whose cost grows while everything else happens.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// dispatch routes to a subcommand and returns the process exit code. It is a
// function over (args, writers) rather than main's body so tests can call it.
func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 2 && args[0] == "capture" {
		switch args[1] {
		case "run":
			return captureRun(args[2:], stderr)
		case "verify":
			return captureVerify(args[2:], stdout, stderr)
		case "cat":
			return captureCat(args[2:], stdout, stderr)
		case "manifest":
			return captureManifest(args[2:], stdout, stderr)
		}
	}
	if len(args) >= 2 && args[0] == "calibrate" {
		switch args[1] {
		case "fetch":
			return calibrateFetch(args[2:], stdout, stderr)
		case "run":
			return calibrateRun(args[2:], stderr)
		}
	}
	fmt.Fprint(stderr, usage)
	return 64 // EX_USAGE: the caller's request was malformed, not a capture verdict
}

const usage = `arblab — funding-rate arbitrage ops CLI

usage:
  arblab capture run      -config <file>
  arblab capture verify   -config <file> -from <RFC3339> -to <RFC3339> [-root <dir>] [-min-alive <pct>]
  arblab capture cat      <file.arbcap> [...]
  arblab capture manifest -config <file> -day <YYYY-MM-DD> [-root <dir>]
  arblab calibrate fetch  -venue <v> -series <s> -symbol <sym> -start <RFC3339> -end <RFC3339> [-out <dir>]
  arblab calibrate run

calibrate fetch commits one closed family-1 dataset identity (venue REST
history, verbatim, digested) under datasets/; it refuses holdout-era
ranges, incomplete grids, and existing files. calibrate run is the
governed derivation door — it refuses until step 4 slice C lands the
freeze.

capture verify exit codes: 0 clean, 1 integrity (the archive is lying),
2 coverage (the archive is honest and thin). Integrity decides when both
are present.

capture cat renders archive records as NDJSON; it stops at the first
invalid frame — diagnosing a file cat refuses is verify's job.

capture manifest backfills the day's manifest from sidecars, labeled
"backfill"; it never overwrites a close-time entry.
`

// buildInfo names this binary for the archive's provenance records.
//
// The git SHA comes from the toolchain's own VCS stamp rather than an ldflags
// variable: an ldflags value is whatever the build script said, and a script
// that goes stale writes a wrong SHA into every Open record with nothing
// failing. "unknown" is the honest reading for a binary built outside a
// checkout (go test, go run).
func buildInfo() (tool, gitSHA string) {
	tool, gitSHA = "arblab", "unknown"
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return tool, gitSHA
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			gitSHA = s.Value
		}
	}
	return tool, gitSHA
}
