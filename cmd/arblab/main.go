// Command arblab is the ops CLI §11 names: capture, replay, reconcile, key
// checks. Capture came first because milestone 2 is calendar-bound on funding
// history and capture is the only work whose cost grows while everything else
// happens; `arblab reconcile run` landed with milestone 3 slice G2, and it is
// this repo's first binary that constructs a boot.Checklist at all.
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
			return calibrateRun(args[2:], stdout, stderr)
		}
	}
	if len(args) >= 2 && args[0] == "screen" && args[1] == "run" {
		return screenRun(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "crossvenue" && args[1] == "check" {
		return crossvenueCheck(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "crossvenue" && args[1] == "fetch" {
		return crossvenueFetch(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "crossvenue" && args[1] == "sim" {
		return crossvenueSim(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "crossvenue" && args[1] == "gate" {
		return crossvenueGate(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "reconcile" && args[1] == "run" {
		return reconcileRun(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "reference" && args[1] == "fetch" {
		return referenceFetch(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "baseline" && args[1] == "record" {
		return baselineRecord(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "exercise" && args[1] == "run" {
		return exerciseRun(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "pair" && args[1] == "run" {
		return pairRun(args[2:], stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "margin" && args[1] == "reconcile" {
		return marginReconcile(args[2:], stdout, stderr)
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
  arblab screen run       [-root <dir>] [-out <dir>] [-stage fetch|compute|all]
  arblab crossvenue check -day <YYYY-MM-DD> [-root <dir>]
  arblab crossvenue fetch -symbols <a,b,...> -cutoff <RFC3339> [-rest <dir>]
  arblab crossvenue sim   -params <file> -out <file> [-root <dir>] [-rest <dir>]
  arblab crossvenue gate  -before <RFC3339> -out <file> [-root <dir>] [-rest <dir>] [-reference <file>]
  arblab reconcile run    -config <file>
  arblab reference fetch  -reference <in.json> -out <out.json>
  arblab baseline record  -config <file> -out <file>
  arblab exercise run     -config <file>
  arblab pair run         -config <file>
  arblab margin reconcile -config <file> -op <id> -actor <name> -at <RFC3339> -note <text>

calibrate fetch commits one closed family-1 dataset identity (venue REST
history, verbatim, digested) under datasets/; it refuses holdout-era
ranges, incomplete grids, and existing files. calibrate run is the
governed derivation door — it refuses until step 4 slice C lands the
freeze.

reconcile run performs §7.5's reconcile-only boot once: journal replayed,
venue joined, §4.1's account and margin-mode posture asserted, every
absence put to §5.3's stable-not-found rule, net-delta invariant asserted.
Exit codes: 0 nothing needs a human, 1 an operator alert — a LATCHED gate
(§7.5; a retry will not lift it, and a posture read and found wrong is one:
this engine never repairs a posture) or a venue IP ban (§7.6's 418/10018; a
retry ESCALATES it), 2 the reconciliation could not finish. The reported
POSTURE is not the verdict — this binary declares all eight of §7.5's gates
and produces five, so reconcile-only is the honest answer on a clean
account.

exercise run is §9's synthetic spot↔perp cycle driver: one reconcile-only
boot to conclusion, and on a clean verdict one parallel marketable-IOC
package from the config's trade block. Exit codes: reconcile's three, plus
3 — the package was DISPATCHED and a leg needs a human (refused, rejected,
unjournaled, or a stale projection holding new trading). After a 3 the
recovery path is "reconcile run"; re-running exercise with the same trade
id is refused by §5.3's id-reuse rule.

pair run drives ONE cross-venue package across TWO deployments: two
§5.2 blocks, two journals, two reconcile-only boots, and — only if both
verdicts are clean and both legs' venues are FLAT — leg A, then leg B
sized to leg A's ESTABLISHED fill. A leg that filled nothing opens no
hedge; a leg whose fill could not be read stops the run. On a half-open
pair it issues one reduce-only close of leg A. Exit codes: reconcile's
three, plus 3 (a leg was dispatched and needs a human) and 4 — THE PAIR
IS EXPOSED: the two venues hold unbalanced positions. §6.7's classifier
cannot see a cross-venue pair, so nothing compensates a 4
automatically; the recovery path is "reconcile run" on each side and a
manual close.

margin reconcile records core §7.1's "explicit accounting / operator
action" against ONE Ambiguous margin op — the §9 incident that blocks new
openings until it is reconciled, and the only thing that lifts one. It
reads no venue and needs no credential; it appends who / what / when plus
the note to §5.1's chain. The ENGINE MUST BE STOPPED: the journal is a
single-writer store and the act takes effect at the next boot, because
nothing lifts a latch inside a running process. Exit codes: 0 recorded,
1 refused (the id is not in the log, the op has no verdict yet, it
resolved applied/not_applied and blocks nothing, or it is already
reconciled). A 0 still prints any incidents left open.

capture verify exit codes: 0 clean, 1 integrity (the archive is lying),
2 coverage (the archive is honest and thin). Integrity decides when both
are present.

capture cat renders archive records as NDJSON; it stops at the first
invalid frame — diagnosing a file cat refuses is verify's job.

capture manifest backfills the day's manifest from sidecars, labeled
"backfill"; it never overwrites a close-time entry.

screen run is the §42b screening harness: both venues' settled-funding
REST history for the intersection universe (raw pages verbatim under
-root, git-ignored), then the committed ledger, ranking, shortlist and
dislocation census under -out. Parameters are pinned in the screening
package; the fetch stage resumes past files already on disk.
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
