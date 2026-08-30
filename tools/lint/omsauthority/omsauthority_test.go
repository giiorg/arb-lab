package omsauthority_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/omsauthority"
)

// The gate under test: only granted composition roots may import internal/oms
// or internal/oms/application, per build-spec §3.6 rule 11.
//
// The `// want` comments in testdata are what make this a gate rather than a
// decoration: analysistest fails both when an expected diagnostic is missing
// AND when an unexpected one appears, so every clean package below is load
// bearing. This rule's own risk is over-reporting, because the packages it
// exempts outnumber the ones it reports and three different mechanisms do the
// exempting — the owner tree, the grant list and rule 6's exclusion.
func TestOMSAuthority(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), omsauthority.Analyzer,
		// Clean, and each is a DIFFERENT mechanism doing the exempting:
		//
		//   - internal/oms is the owner, matched by within()'s equality arm.
		//     Every other exempt package in the owner tree is a subpackage, so
		//     without this fixture the equality arm is untested — the shape
		//     that hid singlewriter's defect for a whole gate.
		//   - oms/infrastructure/baseline is the owner tree's PREFIX arm, and
		//     it is also §3.6 rule 3's documented exception: a root constructs
		//     §5.2's spot_baseline pin through it precisely so it can do that
		//     without acquiring the write surface.
		//   - internal/oms_test is the owner's EXTERNAL test package, which
		//     within() alone answers NO for.
		//   - internal/app/pair is a GRANT, and pair_test is the same
		//     normalisation reaching the OTHER branch. Both are exempt and
		//     neither is exempt for the same reason, which is why the mirror
		//     is a separate fixture rather than a duplicate.
		//   - cmd/arblab is a grant OUTSIDE internal/, which is what forces the
		//     module root to be resolved from the imported path.
		"github.com/giiorg/arb-lab/internal/oms",
		"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline",
		"github.com/giiorg/arb-lab/internal/app/pair",
		"github.com/giiorg/arb-lab/cmd/arblab",

		// Offending, and the motivating case: the web-facing tree record §100
		// designs. httpapi is §15.7's probe kept as a fixture — both of its
		// imports were measured silent against all six existing gates, and the
		// second is the route no facade can close. serve's external test
		// package is the negative mirror of the two exempt ones above: the
		// suffix is stripped there too, and what the strip reveals is a package
		// that is not on the list.
		"github.com/giiorg/arb-lab/internal/app/serve",
		"github.com/giiorg/arb-lab/internal/app/serve/httpapi",

		// Offending, and each isolates one predicate from the one it would
		// otherwise collapse into:
		//
		//   - internal/app/pair/sub separates the grant's EXACT comparison from
		//     a prefix comparison. The two agree on every path that is not a
		//     subpackage of a granted root, so this is the only fixture that
		//     can tell them apart.
		//   - internal/app/pairx and internal/omsx are the two prefix traps,
		//     one on the grant side and one on the owner side.
		"github.com/giiorg/arb-lab/internal/app/pair/sub",
		"github.com/giiorg/arb-lab/internal/app/pairx",
		"github.com/giiorg/arb-lab/internal/omsx",

		// Offending from OUTSIDE internal/, and it is the fixture that forces
		// the module root to be resolved from the IMPORTED path. cmd/arblab
		// above cannot do that job: it is granted, so it is silent either way,
		// and silent-for-the-right-reason and silent-for-the-wrong-one are the
		// same output. Only a cmd/ package that must be REPORTED separates them.
		"github.com/giiorg/arb-lab/cmd/rogue",

		// Offending, mixed: a bounded context whose file carries three reported
		// imports and six clean ones, so the diagnostics are shown to land on
		// the right LINES rather than merely to exist. Its clean imports are
		// the whole of the exempt surface in one place — rule 6's two trees,
		// oms/domain, the baseline exception, both prefix traps and a
		// third-party package laid out identically with no /internal/ segment.
		"github.com/giiorg/arb-lab/internal/risk/application",
	)
}

// TestSurfaceEntriesRenderDistinctly is the check testdata structurally cannot
// perform.
//
// The two policed entries are matched by opposite rules — internal/oms exactly,
// internal/oms/application as a tree — and every `want` regex in the tree keys
// on a phrase from one cost sentence. If entry 2's sentence grew to CONTAIN
// entry 1's, each `want` would still match its own line, both fixtures would
// pass, and the entries would have become indistinguishable to every assertion
// in the suite. An assertion cannot see a variation two branches render alike.
//
// It reads the analyzer's own strings. A pair of constants copied into this
// file would be a check fed only by the thing it is checking.
func TestSurfaceEntriesRenderDistinctly(t *testing.T) {
	entries := omsauthority.Surface()
	if len(entries) < 2 {
		t.Fatalf("the surface has %d entries; the comparison below is vacuous", len(entries))
	}
	for i, a := range entries {
		for j, b := range entries {
			if i == j {
				continue
			}
			if strings.Contains(a.Cost, b.Cost) {
				t.Errorf("the cost sentence for %q contains the one for %q; no `want` regex "+
					"can tell which entry produced a diagnostic", a.Dir, b.Dir)
			}
		}
	}

	// The shapes are opposite on purpose, and a suite that exercised only one
	// of them would leave the other's choice unmeasured. Stated here so the
	// two testdata fixtures that separate them — oms/domain clean under a
	// non-granted importer, oms/application/usecases reported — are known to be
	// a pair rather than two incidental cases.
	var exact, tree int
	for _, e := range entries {
		if e.Exact {
			exact++
		} else {
			tree++
		}
	}
	if exact == 0 || tree == 0 {
		t.Errorf("the surface is %d exact and %d tree entries; one of the two match "+
			"shapes has no fixture that can fail", exact, tree)
	}
}

// TestEveryGrantIsStillUsed is the check `make lint` cannot perform.
//
// The analyzer only ever reports a package MISSING from the grant list. A grant
// whose root stopped importing the surface stays there silently, and the next
// person to read the list takes it as evidence that the authority is still
// held — a rejection's reason expiring with nothing failing. §3.6 rule 4 has
// the same shape stated as a rule: "an escape with nothing to suppress is
// itself reported."
//
// It cannot be an analyzer diagnostic, and the reason is worth recording rather
// than rediscovering: an analyzer runs per package VARIANT, and cmd/arblab's
// production variant imports none of the surface — only its in-package test
// file does. A per-variant "this grant permits nothing" report would fire on a
// real, correct package.
//
// So it is a `go list` over the engine module, which is the shape
// internal/app/capture/imports_test.go already uses for §32b's fence.
func TestEveryGrantIsStillUsed(t *testing.T) {
	const module = "github.com/giiorg/arb-lab"
	surface := []string{
		module + "/internal/oms",
		module + "/internal/oms/application",
	}

	// Every import class, because a grant held for a TEST file is still held:
	// a *_test.go file in package main can call anything package main can.
	cmd := exec.Command("go", "list", "-f",
		`{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`,
		"./...")
	cmd.Dir = "../../.." // the engine module root; tools/lint is nested on purpose
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list over the engine module: %v", err)
	}

	// The positive control. A `go list` that silently produced nothing would
	// satisfy the loop below perfectly, and the whole check would read as a
	// pass. The graph must be non-empty AND contain the owner itself — the one
	// package that must appear whatever else changed.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("go list returned nothing; the check below would pass against any grant list at all")
	}
	var sawOwner bool
	users := map[string]bool{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		importer, imports := fields[0], fields[1:]
		if importer == module+"/internal/oms" {
			sawOwner = true
		}
		if strings.HasPrefix(importer, module+"/internal/oms") {
			continue // the owner tree is exempt and is not a grant
		}
		for _, imp := range imports {
			if slices.Contains(surface, imp) {
				users[importer] = true
			}
		}
	}
	if !sawOwner {
		t.Fatalf("go list did not report %s/internal/oms; it is not enumerating the packages this test assumes (%d lines)",
			module, len(lines))
	}

	for _, g := range omsauthority.Grants() {
		full := module + "/" + g.Pkg
		if !users[full] {
			t.Errorf("the grant for %s is unused — nothing in it imports the OMS use-case surface. "+
				"It was justified as: %s. That reason has expired; remove the grant rather than "+
				"leaving it as evidence of an authority nobody holds", g.Pkg, g.Why)
		}
	}

	// The other direction is redundant with `make lint` and is cheap, and it
	// catches the one case lint cannot: a grant list edited without re-running
	// the gate over the tree.
	for importer := range users {
		granted := slices.ContainsFunc(omsauthority.Grants(), func(g struct{ Pkg, Why string }) bool {
			return module+"/"+g.Pkg == importer
		})
		if !granted {
			t.Errorf("%s imports the OMS use-case surface and is not granted", importer)
		}
	}
}

// TestGrantsAreExactPackagePaths guards the one property the grant list's shape
// depends on: the comparison in grantedTo is exact, so a trailing slash or a
// leading module prefix in an entry would make it match nothing and the grant
// would silently stop working while the list still read as if it did.
func TestGrantsAreExactPackagePaths(t *testing.T) {
	if len(omsauthority.Grants()) == 0 {
		t.Fatal("the grant list is empty; every assertion below is vacuous")
	}
	for _, g := range omsauthority.Grants() {
		if strings.HasPrefix(g.Pkg, "/") || strings.HasSuffix(g.Pkg, "/") {
			t.Errorf("grant %q is not a bare package path", g.Pkg)
		}
		if strings.Contains(g.Pkg, "github.com/") {
			t.Errorf("grant %q carries the module prefix; grants are relative to the module root", g.Pkg)
		}
		if strings.TrimSpace(g.Why) == "" {
			t.Errorf("grant %q carries no reason", g.Pkg)
		}
	}
}
