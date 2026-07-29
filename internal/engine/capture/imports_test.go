package capture_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// §32b's fence, and it is a test rather than a seventh §3.6 analyzer.
//
// The requirement is §31e's: the raw TRANSPORT is shared with the future live
// path and the dispatcher never is. Getting that distinction precise matters,
// because the loose version — "keep capture separate" — would forbid the one
// sharing that makes capture worth doing: capturing through a different socket
// stack than the engine runs would make the archive a record of a pipeline we do
// not run. So the fence is not "capture imports little", it is "capture imports
// no ARBITRATION": no bounded context, and no dispatcher when one exists.
//
// # Why not a nocapturedrift analyzer
//
// It would be the house style, and §3.6's own argument — a linter you cannot
// unit-test cannot be SHOWN to bite — is real. Rejected on proportionality: a
// new gate is a spec edit to §3.6's ten rules, and the hazard here is a single
// import edge in two packages, which a list test in those packages' own suites
// covers completely. If capture ever spreads past two packages, revisit.
//
// # Production dependencies only, and that is the right scope
//
// `go list -deps` reports what the package itself imports, transitively, not
// what its tests do. The hazard §31e names is capture GROWING a runtime
// dependency on the dispatcher; a test-only import cannot ship in the capture
// binary, and scoping to the shipping graph is what keeps the fence about
// arbitration rather than about tidiness.
var forbiddenRoots = []string{
	"github.com/giiorg/arb-lab/internal/marketdata",
	"github.com/giiorg/arb-lab/internal/oms",
	"github.com/giiorg/arb-lab/internal/strategy",
	"github.com/giiorg/arb-lab/internal/risk",
	"github.com/giiorg/arb-lab/internal/treasury",
	"github.com/giiorg/arb-lab/internal/audit",

	// The dispatcher, named ahead of its existence. §12.1's parity dispatcher
	// is unbuilt; when it lands, this entry is already here rather than
	// remembered — which is the difference between a fence and an intention.
	"github.com/giiorg/arb-lab/internal/engine/dispatch",
}

// forbidden reports which of deps fall under a forbidden root.
//
// A package IS its root or lives under it — the `/` is required, so a future
// internal/omsutil is not matched by internal/oms. Same shape as the analyzers'
// within(): a bare prefix test is the defect hexboundary and kernelpurity both
// record.
func forbidden(deps []string) []string {
	var hits []string
	for _, dep := range deps {
		for _, root := range forbiddenRoots {
			if dep == root || strings.HasPrefix(dep, root+"/") {
				hits = append(hits, dep)
			}
		}
	}
	return hits
}

func TestCaptureDependsOnNoBoundedContext(t *testing.T) {
	const pkg = "github.com/giiorg/arb-lab/internal/engine/capture"

	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	deps := strings.Fields(string(out))

	// The positive control, and it is not decoration: a `go list` that silently
	// produced nothing would satisfy a forbid-everything check perfectly, which
	// is this repo's recurring failure shape — a clean run and a vacuous run
	// look identical. So the graph must be non-empty AND must contain a
	// dependency we know is really there.
	if len(deps) == 0 {
		t.Fatal("go list -deps returned an empty graph; the check below would " +
			"pass against any code at all")
	}
	if !slices.Contains(deps, "hash/crc32") {
		t.Errorf("the dependency graph does not contain hash/crc32, which codec.go "+
			"imports — go list is not reporting what this test assumes (%d entries)",
			len(deps))
	}

	if hits := forbidden(deps); len(hits) > 0 {
		t.Errorf("capture depends on %v — §32n: capture makes no arbitration "+
			"decision, so it may not depend on a bounded context or on the "+
			"dispatcher", hits)
	}
}

// TestForbiddenPredicateBites is the negative control for the predicate itself.
// Without it, a `forbidden` that returned nil unconditionally would make the
// test above pass forever.
func TestForbiddenPredicateBites(t *testing.T) {
	bad := []string{
		"github.com/giiorg/arb-lab/internal/oms",
		"github.com/giiorg/arb-lab/internal/oms/application/ports",
		"github.com/giiorg/arb-lab/internal/marketdata/domain",
		"github.com/giiorg/arb-lab/internal/engine/dispatch",
	}
	for _, dep := range bad {
		if hits := forbidden([]string{dep}); len(hits) == 0 {
			t.Errorf("forbidden(%q) found nothing", dep)
		}
	}

	// And the other direction, which is what stops the predicate from being
	// tightened into a general ban on internal/. Each of these is an import
	// capture legitimately has or will have — the shared transport in
	// engine/venue is the whole point of §31e's fence being about arbitration
	// rather than about separation.
	good := []string{
		"hash/crc32", "encoding/binary", "time",
		"github.com/giiorg/arb-lab/internal/platform/clock",
		"github.com/giiorg/arb-lab/internal/platform/config",
		"github.com/giiorg/arb-lab/internal/engine/venue",
		"github.com/giiorg/arb-lab/internal/engine/capture",

		// The prefix trap: a package whose path merely begins with a forbidden
		// root's name is not under it.
		"github.com/giiorg/arb-lab/internal/omsutil",
		"github.com/giiorg/arb-lab/internal/riskless",
	}
	for _, dep := range good {
		if hits := forbidden([]string{dep}); len(hits) > 0 {
			t.Errorf("forbidden(%q) = %v, want nothing", dep, hits)
		}
	}
}
