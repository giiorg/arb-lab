package reconcile_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The fence for this composition root, and it is a DIFFERENT fence from
// app/capture's — which is the point of each root carrying its own.
//
// capture's forbids every bounded context, because capture is a data tool that
// must never reach into arbitration. This root's whole job is to construct one
// context, so `internal/oms` is not merely permitted here: it is the subject.
// What must stay out is every OTHER context, and the reason is §7.5's rather
// than §32b's — a reconcile-only boot exists to establish what a process may do
// BEFORE any strategy, risk gate or treasury path is live, and an import edge to
// one of them is how a boot phase acquires a collaborator that assumes the
// posture this phase has not yet decided.
//
// Stated as a list rather than left to review because a composition root is
// exactly where such an edge arrives most naturally — "while we are here, read
// the mark off marketdata so the report can price the delta".
var forbiddenRoots = []string{
	"github.com/giiorg/arb-lab/internal/marketdata",
	"github.com/giiorg/arb-lab/internal/strategy",
	"github.com/giiorg/arb-lab/internal/risk",
	"github.com/giiorg/arb-lab/internal/treasury",
	"github.com/giiorg/arb-lab/internal/audit",

	// The other roots. Two composition roots in one binary's dependency graph
	// means one of them is a library, and the one that would be imported is the
	// one holding a live capture pipeline.
	"github.com/giiorg/arb-lab/internal/app/capture",
	"github.com/giiorg/arb-lab/internal/app/sim",

	// The dispatcher, named ahead of its existence (§12.1), as capture's copy
	// does — so the entry is already here rather than remembered.
	"github.com/giiorg/arb-lab/internal/engine/dispatch",
}

// forbidden reports which of deps fall under a forbidden root. A package IS its
// root or lives under it — the `/` is required, so internal/riskutil would not be
// matched by internal/risk (the analyzers' within() defect, avoided the same way
// here).
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

// TestRootImportsNoOtherContext walks the PRODUCTION dependency graph.
//
// `go list -deps` reports what ships in the binary rather than what the tests
// import, which is the distinction that makes this a fence and not a style rule:
// a test may legitimately construct a strategy fixture, and no such edge reaches
// a deployed process.
func TestRootImportsNoOtherContext(t *testing.T) {
	t.Parallel()

	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	deps := strings.Fields(string(out))
	if hits := forbidden(deps); len(hits) > 0 {
		t.Errorf("this root imports %v, which §7.5's boot must not depend on", hits)
	}

	// The positive control, without which a `go list` that returned nothing would
	// read as a perfect score: the context this root exists to construct MUST be
	// in the graph.
	if !slices.Contains(deps, "github.com/giiorg/arb-lab/internal/oms") {
		t.Error("internal/oms is not in the dependency graph — then this fence is measuring an empty list")
	}
	if !slices.Contains(deps, "github.com/giiorg/arb-lab/internal/platform/boot") {
		t.Error("platform/boot is not in the dependency graph — this root is its first production importer")
	}
}
