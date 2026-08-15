package pair

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The fence for this composition root.
//
// It is reconcile's argument, transferred: a root that acquires a collaborator
// from another bounded context has acquired one that assumes a posture no phase
// here has decided. What is different is the SUBJECT. This root constructs two
// reconcile.Runners, each of which constructs an oms.Module, so both of those
// are in the graph on purpose and both carry fences of their own.
//
// # Two lists, because "in the binary" and "named by this package" are two claims
//
// A transitive dependency list cannot express "this package must not name
// ports": `internal/oms` imports it, so it is in every graph containing the OMS
// and always will be. The singlewriter analyzer polices DECLARED EDGES for
// exactly that reason, and the direct-import list below is the same claim
// written where a reader of this package will meet it.
//
// The other direction is equally real: `app/exercise` must not appear in the
// deployed graph AT ALL, not merely go unnamed here, because an edge to it
// through some third package would make one of the two roots a library just as
// surely.
//
// # internal/strategy is deliberately on NEITHER list
//
// This root resolves §2.4 filters from the strategy context's dated reference
// revision, exactly as app/exercise does, and filters.go carries the argument
// for why that adapter belongs at the root that PLANS rather than in §7.5's
// boot. Reconcile's own fence documents an exemption of the same shape
// (permittedByGate), and the general rule there is what this restates: a fence
// is keyed to its reason, not to its argument.
//
// internal/marketdata is likewise absent from the transitive list even though it
// is on reconcile's: `marketdata/domain` reaches this graph through
// `strategy/domain`, which is the edge above arriving one package further on. A
// transitive entry would forbid it and the direct entry is the claim that
// matters — this package names no marketdata package itself.
var forbiddenInTheBinary = []string{
	// The other roots. Two composition roots in one binary's dependency graph
	// means one of them is a library — and app/exercise is the nearest
	// neighbour, which is exactly why the edge must not exist: record §95 §3's
	// whole finding is that the single-venue root cannot express this package.
	// The shared parts are reconcile.Runner and oms.Module, which both roots
	// import directly.
	"github.com/giiorg/arb-lab/internal/app/capture",
	"github.com/giiorg/arb-lab/internal/app/sim",
	"github.com/giiorg/arb-lab/internal/app/exercise",

	// The dispatcher, named ahead of its existence (§12.1), as capture's copy
	// does — so the entry is already here rather than remembered.
	"github.com/giiorg/arb-lab/internal/engine/dispatch",
}

// forbiddenAsDirectImports is what THIS package may not name.
//
// The last two are §3.6 rule 6's, named here as well as policed by the
// singlewriter analyzer. The analyzer is the gate; this entry is the note
// saying the omission is deliberate, and it is why oms.LegState exists (task 1)
// — a cross-venue root cannot size a hedge against a fill it cannot see, so the
// read is a projection minted in the package the gate permits.
var forbiddenAsDirectImports = []string{
	"github.com/giiorg/arb-lab/internal/marketdata",
	"github.com/giiorg/arb-lab/internal/risk",
	"github.com/giiorg/arb-lab/internal/treasury",
	"github.com/giiorg/arb-lab/internal/audit",
	"github.com/giiorg/arb-lab/internal/oms/application/ports",
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven",
}

// under reports which of deps fall under one of the given roots. A package IS
// its root or lives under it — the `/` is required, so internal/riskutil would
// not be matched by internal/risk (the analyzers' within() defect, avoided the
// same way here).
func under(roots, deps []string) []string {
	var hits []string
	for _, dep := range deps {
		for _, root := range roots {
			if dep == root || strings.HasPrefix(dep, root+"/") {
				hits = append(hits, dep)
			}
		}
	}
	return hits
}

// productionDeps is the whole transitive graph that ships in the binary, rather
// than what the tests import — the distinction that makes this a fence and not a
// style rule.
func productionDeps(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	return strings.Fields(string(out))
}

// directImports is what this package NAMES, which is the granularity §3.6 rule
// 6 is enforced at.
func directImports(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, ".").Output()
	if err != nil {
		t.Fatalf("go list -f imports: %v", err)
	}
	return strings.Fields(string(out))
}

func TestRootImportsNoOtherRoot(t *testing.T) {
	t.Parallel()

	deps := productionDeps(t)
	if hits := under(forbiddenInTheBinary, deps); len(hits) > 0 {
		t.Errorf("this root's binary graph contains %v; one of the two roots would be a library", hits)
	}

	// The positive control, without which a `go list` that returned nothing
	// would read as a perfect score: the two things this root exists to compose
	// MUST be in the graph.
	for _, want := range []string{
		"github.com/giiorg/arb-lab/internal/oms",
		"github.com/giiorg/arb-lab/internal/app/reconcile",
	} {
		if !slices.Contains(deps, want) {
			t.Errorf("%s is not in the dependency graph — then this fence is measuring an empty list", want)
		}
	}
}

func TestRootNamesNoOtherContext(t *testing.T) {
	t.Parallel()

	imports := directImports(t)
	if hits := under(forbiddenAsDirectImports, imports); len(hits) > 0 {
		t.Errorf("this package names %v; a root that can name the write path can grow a second one", hits)
	}

	// The positive control for THIS enumeration, which is a different list from
	// the one above and can fail on its own: the package demonstrably names
	// app/reconcile and internal/oms.
	for _, want := range []string{
		"github.com/giiorg/arb-lab/internal/oms",
		"github.com/giiorg/arb-lab/internal/app/reconcile",
	} {
		if !slices.Contains(imports, want) {
			t.Errorf("%s is not among this package's direct imports — then this fence is measuring an empty list", want)
		}
	}
	// And the strategy edge is asserted PRESENT rather than left to be noticed,
	// because it is the one deliberate omission from the list above: if the
	// filter adapter ever leaves this root, the omission's reason has expired
	// and this line is what says so.
	if !slices.Contains(imports, "github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven") {
		t.Error("this root no longer names strategy's reference store; the deliberate absence of " +
			"internal/strategy from the fence has outlived its reason — see filters.go")
	}
}

// TestTheFenceStillBites is the negative control for both predicates. An empty
// hit list and a fence that stopped matching look identical from the two tests
// above, so this asks `under` directly about packages that are not in the graph.
func TestTheFenceStillBites(t *testing.T) {
	t.Parallel()

	for _, dep := range []string{
		"github.com/giiorg/arb-lab/internal/app/exercise",
		"github.com/giiorg/arb-lab/internal/app/exercise/internal/thing",
	} {
		if hits := under(forbiddenInTheBinary, []string{dep}); len(hits) == 0 {
			t.Errorf("%s must be refused by the binary-graph fence", dep)
		}
	}
	for _, dep := range []string{
		"github.com/giiorg/arb-lab/internal/oms/application/ports",
		"github.com/giiorg/arb-lab/internal/risk/domain",
	} {
		if hits := under(forbiddenAsDirectImports, []string{dep}); len(hits) == 0 {
			t.Errorf("%s must be refused by the direct-import fence", dep)
		}
	}
	// And the near-miss the analyzers' own within() defect was: a package whose
	// path merely STARTS with a forbidden root's name is not under it.
	if hits := under(forbiddenAsDirectImports, []string{"github.com/giiorg/arb-lab/internal/riskutil"}); len(hits) > 0 {
		t.Errorf("internal/riskutil matched %v; the fence is matching a prefix rather than a path", hits)
	}
}
