package reconcile

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/boot"
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

// permittedByGate is the fence's one exemption, and it exists because the
// fence's own argument turned out not to cover a case §7.5 had already declared
// (task #198, record §73k F12).
//
// # Why `internal/risk` was the wrong shape of entry
//
// The paragraph above forbids the other contexts because importing one is how a
// boot phase acquires a collaborator that ASSUMES the posture this phase has not
// yet decided. That is true of marketdata, strategy, treasury and audit. It is
// false of §4.1's account-posture guard, which does not assume the boot posture:
// it is one of the eight gates that DECIDES it, declared in declaredGates()
// since slice G2 and named in §7.5's roster before any of this code existed. The
// fence and the roster disagreed, and the fence was the one stating a general
// rule from a single family of cases ([[an-exemption-argued-from-one-cause]]).
//
// # The exemption is keyed to its REASON, not to its argument
//
// A blanket "risk/application is fine" would let the NEXT use case added to that
// package ride in free — a live position-limit gate, say, which genuinely does
// assume the posture. So two things bound it. Exact package paths, never
// prefixes: a new package under risk/ is refused until somebody adds it here and
// argues. And TestRiskExemptionIsKeyedToTheGateItProduces below, which fails if
// this root imports risk while NOT declaring the gate risk produces — so the
// permission lapses automatically the day its reason does.
//
// What it still does not catch, stated rather than left implied: a second use
// case added INSIDE one of these five packages is admitted silently. The
// remaining defence there is that a boot gate has to be declared to be reported,
// which is a weaker check than this one and is why the list is exact-path.
var permittedByGate = []string{
	"github.com/giiorg/arb-lab/internal/risk/application",
	"github.com/giiorg/arb-lab/internal/risk/application/ports",
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/driven",
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation",
}

// permittedByPolicyOwnership is the fence's SECOND exemption, and it exists
// because the first one silently covered an edge it was never argued for (task
// #210).
//
// # The edge, and why it is not the gate's
//
// `risk/domain` was on the list above from the day the posture guard landed, and
// it arrived there through `risk/application` — this root constructs the guard,
// the guard uses the domain's rule tables. Task #210 gave the same package a
// completely different route in: §7.1's liq-distance policy is single-sourced to
// §5.2 with consumers in two contexts (this monitor and §6 step 4's sizing), so
// it lives in `risk/domain`, and `oms/application` now imports it. `internal/oms`
// is this root's SUBJECT, so the edge is unavoidable and permanent.
//
// Nothing failed when that landed, and that is the finding: the entry was
// already present, so the new edge was admitted by a permission granted for the
// account-posture gate — the exact shape this file's own comment warns about
// ("a blanket risk/application is fine would let the NEXT use case ride in
// free"), reached through a package that was already listed rather than through
// one that was added. An exemption generalizes over its PREDICATE, not over the
// argument that motivated it ([[an-exemption-argued-from-one-cause]]).
//
// # Why the edge is nonetheless correct
//
// The fence's stated reason is that importing another context is how a boot
// phase acquires a collaborator that ASSUMES the posture this phase has not yet
// decided. `risk/domain`'s liq-distance policy assumes nothing: it is a
// threshold table and two pure comparisons, with no port, no clock and no venue.
// So it is exempted on its own ground, and the ground is written down here
// rather than inferred from the fact that nothing went red.
//
// The two lists are separate so that TestRiskExemptionIsKeyedToTheGateItProduces
// keys only on the packages whose reason really is the gate. Were `risk/domain`
// left on that list alone, dropping GateAccountPosture from this root's roster
// would report a permission outliving its argument for a package whose argument
// had not expired.
var permittedByPolicyOwnership = []string{
	"github.com/giiorg/arb-lab/internal/risk/domain",
}

// forbidden reports which of deps fall under a forbidden root. A package IS its
// root or lives under it — the `/` is required, so internal/riskutil would not be
// matched by internal/risk (the analyzers' within() defect, avoided the same way
// here).
func forbidden(deps []string) []string {
	var hits []string
	for _, dep := range deps {
		if slices.Contains(permittedByGate, dep) || slices.Contains(permittedByPolicyOwnership, dep) {
			continue
		}
		for _, root := range forbiddenRoots {
			if dep == root || strings.HasPrefix(dep, root+"/") {
				hits = append(hits, dep)
			}
		}
	}
	return hits
}

// productionDeps is what ships in the binary rather than what the tests import,
// which is the distinction that makes this a fence and not a style rule: a test
// may legitimately construct a strategy fixture, and no such edge reaches a
// deployed process.
func productionDeps(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	return strings.Fields(string(out))
}

func TestRootImportsNoOtherContext(t *testing.T) {
	t.Parallel()

	deps := productionDeps(t)
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

// TestForbiddenStillBitesOnAnUnexemptedRiskPackage is the negative control for
// the exemption itself. Five permitted paths and a fence that stopped biting
// look identical from TestRootImportsNoOtherContext, because both report zero
// hits — so this asks the predicate directly about a risk package NOT on the
// list, and about a subpackage of one that is.
func TestForbiddenStillBitesOnAnUnexemptedRiskPackage(t *testing.T) {
	t.Parallel()

	for _, dep := range []string{
		"github.com/giiorg/arb-lab/internal/risk",
		"github.com/giiorg/arb-lab/internal/risk/application/limits",
		"github.com/giiorg/arb-lab/internal/risk/domain/sizing",
	} {
		if hits := forbidden([]string{dep}); len(hits) == 0 {
			t.Errorf("%s is not on the exemption list and must still be refused", dep)
		}
	}
	// And neither exemption is vacuous: every path on either list passes.
	if hits := forbidden(permittedByGate); len(hits) > 0 {
		t.Errorf("the gate exemption list does not exempt %v", hits)
	}
	if hits := forbidden(permittedByPolicyOwnership); len(hits) > 0 {
		t.Errorf("the policy-ownership exemption list does not exempt %v", hits)
	}
}

// TestRiskExemptionIsKeyedToTheGateItProduces ties the permission to the reason
// it was granted.
//
// The exemption exists so this root can produce GateAccountPosture. If that gate
// ever leaves declaredGates(), the reason is gone and the import edge must go
// with it — otherwise the permission outlives its argument, which is how a
// narrow exemption becomes a general one nobody re-derives
// ([[a-rejections-reason-can-expire]]).
//
// It keys on permittedByGate ALONE. `risk/domain` moved to its own list in task
// #210 precisely so that this assertion keeps naming only the packages whose
// reason is this gate — the policy package now reaches this graph through
// `internal/oms` as well, and its presence says nothing about whether the gate
// is still declared.
func TestRiskExemptionIsKeyedToTheGateItProduces(t *testing.T) {
	t.Parallel()

	importsRisk := false
	for _, dep := range productionDeps(t) {
		if slices.Contains(permittedByGate, dep) {
			importsRisk = true
			break
		}
	}
	declaresGate := slices.Contains(declaredGates(), boot.GateAccountPosture)

	if importsRisk && !declaresGate {
		t.Error("this root imports internal/risk under an exemption granted for GateAccountPosture, which it no longer declares")
	}
	// The other direction is deliberately NOT asserted as a failure of this
	// fence: declaring a gate this root does not produce is §7.5's own recorded
	// shape, sorted by Report.ExitClass, and four of the eight are exactly that.
	if !declaresGate {
		t.Error("GateAccountPosture is not in this root's roster — then this test is measuring nothing")
	}
}
