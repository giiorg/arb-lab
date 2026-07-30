package capture_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// §32b's fence, in this package's own suite — the second of the two packages
// the design names. The requirement is §31e's: the raw transport is shared
// with the future live path, the dispatcher and the bounded contexts never
// are. A composition root is where such an import would arrive most naturally
// ("just read the funding rate off marketdata while we're here"), which is why
// the root carries its own copy rather than trusting the engine package's.
//
// Production dependencies only: `go list -deps` reports what ships in the
// binary, not what the tests import. See internal/engine/capture's copy for
// the full argument; the two tests deliberately share their shape so a change
// to one prompts the question about the other.
var forbiddenRoots = []string{
	"github.com/giiorg/arb-lab/internal/marketdata",
	"github.com/giiorg/arb-lab/internal/oms",
	"github.com/giiorg/arb-lab/internal/strategy",
	"github.com/giiorg/arb-lab/internal/risk",
	"github.com/giiorg/arb-lab/internal/treasury",
	"github.com/giiorg/arb-lab/internal/audit",

	// The dispatcher, named ahead of its existence (§12.1). When it lands,
	// this entry is already here rather than remembered.
	"github.com/giiorg/arb-lab/internal/engine/dispatch",
}

// forbidden reports which of deps fall under a forbidden root. A package IS
// its root or lives under it — the `/` is required, so internal/omsutil is not
// matched by internal/oms (the analyzers' within() defect, avoided the same
// way here).
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

func TestCaptureRootDependsOnNoBoundedContext(t *testing.T) {
	const pkg = "github.com/giiorg/arb-lab/internal/app/capture"

	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	deps := strings.Fields(string(out))

	// The positive control: a `go list` that silently produced nothing would
	// satisfy a forbid-everything check perfectly. The graph must be
	// non-empty AND contain platform/clock — the import this root
	// legitimately has (the Wall is its TimeSource), and the one §32b names
	// as the allowed case. A fence that cannot tell "nothing forbidden" from
	// "nothing at all" is not a fence.
	if len(deps) == 0 {
		t.Fatal("go list -deps returned an empty graph; the check below would " +
			"pass against any code at all")
	}
	if !slices.Contains(deps, "github.com/giiorg/arb-lab/internal/platform/clock") {
		t.Errorf("the dependency graph does not contain platform/clock, which "+
			"app.go imports — go list is not reporting what this test assumes "+
			"(%d entries)", len(deps))
	}

	if hits := forbidden(deps); len(hits) > 0 {
		t.Errorf("the capture composition root depends on %v — §32n: capture "+
			"makes no arbitration decision, so it may not depend on a bounded "+
			"context or on the dispatcher", hits)
	}
}

// TestForbiddenPredicateBites is the negative control for the predicate
// itself: a `forbidden` that returned nil unconditionally would make the test
// above pass forever.
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

	// The other direction: every import this root legitimately has, plus the
	// prefix trap — a package whose path merely begins with a forbidden
	// root's name is not under it.
	good := []string{
		"time", "context", "sync",
		"github.com/giiorg/arb-lab/internal/platform/clock",
		"github.com/giiorg/arb-lab/internal/platform/config",
		"github.com/giiorg/arb-lab/internal/platform/observability",
		"github.com/giiorg/arb-lab/internal/engine/capture",
		"github.com/giiorg/arb-lab/internal/engine/venue/transport",
		"github.com/giiorg/arb-lab/internal/omsutil",
		"github.com/giiorg/arb-lab/internal/riskless",
	}
	for _, dep := range good {
		if hits := forbidden([]string{dep}); len(hits) > 0 {
			t.Errorf("forbidden(%q) = %v, want nothing", dep, hits)
		}
	}
}
