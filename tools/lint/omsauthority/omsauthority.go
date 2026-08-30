// Package omsauthority enforces build-spec §3.6 rule 11: OMS write authority is
// granted to named composition roots, and default-denied to every other
// package.
//
// Rule 6 (singlewriter) forbids BYPASSING the write path — it polices
// oms/application/ports and oms/infrastructure/driven, the two ways to mutate
// order state without going through §6's journal-then-apply sequence. It says
// nothing about who may INVOKE that sequence, and it says so deliberately: its
// own text records that "a use case IS the journal-then-apply sequence, so
// calling one is what the rule wants."
//
// That was written when every caller was a composition root in the actuator's
// failure domain. Core §11:817 splits the deployment into separate failure
// domains and §11:826 gives the operator process its own binary described as
// "web-facing; NO actuator, NO fence credential" — so "who may call a use case"
// became a real question with a normative answer, and no gate was asking it.
//
// # The measurement this gate exists because of
//
// Record §100 §15.7 built a package at internal/app/serve/httpapi and ran
// `make lint` over it four times:
//
//	bare internal/oms imported, Module.Execute selected      all six gates silent
//	the same file plus oms/application/ports  (control)      singlewriter reports
//	internal/oms/application imported, *application.Executor  all six gates silent
//	oms.ReconcileMarginOp called with time.Now()             all six gates silent
//
// The control is the half that carries the information: had the ports import
// also been silent, the run would have shown that the invocation never selected
// the package and every row above would have been vacuous.
//
// The fourth row is the one that settled the design. oms.ReconcileMarginOp is a
// package-level exported function over ClearanceDeps{JournalPath, Account
// string} — two strings a server already reads from its own config — and it
// appends an operator act to §5.1's hash-chained journal. §100 §6a had specified
// a capability-limited facade as the guarantee and this gate as the backstop. A
// facade bounds a VALUE; there is no value here to wrap, so the ordering was
// backwards and this gate is the load-bearing remedy (F44).
//
// # Default-deny with a grant list, NOT deny-list-by-position — and the
// # difference is the whole design
//
// §100 §6a specified the construct as "declared import edges from the web-facing
// package tree (internal/app/serve/…, later internal/app/operator/…) into the
// OMS tree". Implemented literally that is an analyzer whose policed set is
// EMPTY in this repo — neither tree exists — which is the condition rules 1, 6
// and 10 were each deferred against: a lint over an empty tree passes trivially
// and that is indistinguishable from a broken lint.
//
// It also fails in the direction this repo has already rejected once. Rule 6
// refused an opt-in //arblab:writerepository marker because "the repository
// somebody adds next year without the marker is silently unpoliced". Naming the
// web trees is that same shape one layer out, with the importer rather than the
// import as its subject: internal/app/dashboard/, internal/app/ui/ and
// internal/app/admin/ are each unpoliced the day somebody creates them, and the
// gate reports nothing while the route it was built to close is open.
//
// So the surface below is the OMS use-case surface, which EXISTS and has three
// real importers today, and the exemption is a list of names. A web-facing root
// is denied because it is not on the list, not because somebody remembered to
// add its tree to a deny-list. That is the standing rule about defaults: when a
// default has two directions, pick the one that is right when somebody forgets
// it. Forgetting to grant a legitimate new root produces a refusal naming the
// package; forgetting to deny a new web root produces a running process whose
// HTTP handlers can append to the audit chain.
//
// Recorded as F47. The rejected alternative is the specification this replaces,
// and it is rejected on a measurement rather than on a preference.
//
// # What is policed, and why the two entries have different shapes
//
//   - internal/oms, matched EXACTLY. The package holds oms.Module — eleven
//     exported use cases, of which Dispatch and MarginTopUp are venue-facing
//     write paths — and oms.ReconcileMarginOp, oms.New and oms.RecordBaseline
//     as package-level functions. Its SUBTREE is deliberately not matched:
//     oms/domain is unpoliced for rule 6's own reason (a projection naming
//     SM §1's state set is §5.1's pattern working, and denying it would force a
//     read model to declare a second copy), and oms/infrastructure/baseline is
//     rule 3's documented exception, which exists precisely so a root can
//     construct §5.2's spot_baseline pin WITHOUT acquiring the write surface.
//     Policing the tree would undo both.
//
//   - internal/oms/application, matched as a TREE. Here the subtree is the
//     subject rather than an accident: a use case added in a subpackage next
//     year carries the same authority as one added beside it, and the exact-match
//     shape would leave it unpoliced with nothing failing. The one subtree that
//     is NOT this gate's is application/ports — see below.
//
// The two shapes are opposite for opposite reasons, so a fixture set that
// exercised only one of them would leave the other's choice unmeasured. There is
// a fixture for each direction: a non-granted package importing oms/domain must
// pass, and a non-granted package importing a subpackage of oms/application must
// fail.
//
// # The rule-6 exclusion, and why a second report would be a real cost
//
// oms/application/ports is inside this gate's application tree and is already
// singlewriter's by position. It is excluded here rather than reported twice,
// because §3.6's own bite-check note says a gate must be shown to bite where NO
// OTHER gate would: an import that two gates report cannot isolate either one,
// and the isolating probe for this gate is exactly the import that singlewriter
// is silent on. Keeping the surfaces disjoint is what keeps that probe
// meaningful. oms/infrastructure/driven is outside this gate's surface already —
// it is under infrastructure/, not under the two entries above — and is named in
// the exclusion anyway so the disjointness is stated in one place rather than
// inferred from two.
//
// # The grant list is the enumeration of who holds the authority
//
// §7.3 requires "least-privilege authorization per action (trip-kill vs
// edit-policy vs promote-tier vs fence are distinct grants)". This is that idea
// at build time and one layer down: holding the OMS use cases is a capability,
// the list below is who holds it, and adding a name is a visible act in a diff
// that a reviewer can weigh. A deny-list has no such moment — the grant arrives
// as the ABSENCE of an edit.
//
// Each entry carries its reason, for the same purpose §3.6 rule 3's documented
// exceptions carry theirs: an exemption whose argument is not written down gets
// re-judged from scratch by whoever meets it next, and generalised past the
// cause that justified it.
//
// # Grants are exact packages, never trees
//
// within() is used for the OWNER and for the application surface, and NOT for
// grants. internal/app/pair/sub is a package internal/app/pair's grant does not
// cover, and it is reported. Same default-direction argument: a granted tree
// hands the authority to a subpackage nobody weighed, and the failure is
// invisible. This is a distinction two predicates could collapse into one — an
// exact comparison and a prefix comparison agree on every path that is not a
// subpackage of a grant — so testdata carries a subpackage of a granted root
// specifically to separate them.
//
// # External test packages
//
// §3.6 states it once, ahead of the rules: a rule identified BY POSITION reads a
// Go external test package as the package it tests. This gate is by position, so
// internal/oms_test is read as internal/oms (owner, exempt) and
// internal/app/pair_test is read as internal/app/pair (granted, exempt).
//
// BOTH are normalised by the same call and both have a fixture, because the
// standing rule is that a fix applied in one place gets applied to its mirror:
// singlewriter shipped with this normalisation missing and reported oms's own
// module_test.go as a second writer, and a version of this gate that stripped
// the suffix only on the owner side would report internal/app/pair's external
// test package instead — the same defect, moved one branch over. The negative
// mirror is a fixture too: internal/app/serve_test is normalised to
// internal/app/serve, which is not granted, and is reported. Without it, "strip
// then check the grant list" and "strip then exempt" would be the same predicate
// on every fixture here.
//
// IN-package test files are not normalised and need no rule: they compile as the
// package itself, so a *_test.go file in package main holds exactly the
// authority package main holds. That is the honest reading rather than an
// oversight — and it is not a formality, because it is the whole of how
// cmd/arblab came to hold this authority at all. See the grant list: the
// binary's production files imported none of the surface, one fixture in one
// test file imported all of it, and the grant that bought covered every
// subcommand (F51).
//
// There is a THIRD path Go builds, and §3.6's sentence does not name it: the
// synthesized main, p.test, which imports p and p_test. This is the first gate
// that can meet it, because it is the first to police a package a generated
// main imports, and it met it on this suite's first run. See
// withoutTestSuffix, and F49.
//
// # The limit, stated rather than left to be discovered
//
// This gate reads DECLARED import edges, the same limit rule 6 states. A root
// that is granted may still pass an *oms.Module to a handler in a package that
// imports nothing — Go's interfaces are structural and a struct field needs no
// import to be assigned. What the gate makes impossible is acquiring the
// authority by ACCIDENT and INVISIBLY: an import is the short way and is what a
// hurried change reaches for, and §100 §6a's capability-limited facade is the
// backstop for the long way round. The facade is the second half; this is the
// first.
package omsauthority

import (
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that only granted composition roots hold OMS write authority

build-spec §3.6 rule 11: importing internal/oms or internal/oms/application is
holding the OMS use cases — Dispatch, MarginTopUp, Delever, ReconcileMarginOp —
and that authority belongs to named composition roots in the actuator's failure
domain. Core §11:826 puts the operator/dashboard process in a different one:
"web-facing; NO actuator, NO fence credential". Rule 6 polices the write path;
this rule polices who may invoke it.`

var Analyzer = &analysis.Analyzer{
	Name: "omsauthority",
	Doc:  doc,
	Run:  run,
}

// internalSegment locates the module root without hardcoding it, as
// kernelpurity, hexboundary and singlewriter all do, so the rule travels with
// the module rather than naming it.
const internalSegment = "/internal/"

// ownerTree is the context that contains the surface, relative to the module
// root. It and everything beneath it are exempt: oms importing its own use
// cases is the pattern working.
const ownerTree = "internal/oms"

// useCase is one package tree of the OMS use-case surface and what importing it
// costs.
//
// The cost is carried because a gate that says only "no" gets worked around;
// naming the consequence and the alternative is what hexboundary, notimenow and
// singlewriter all do, and it is the difference between a rule and an obstacle.
type useCase struct {
	// dir is the package or package tree, relative to the module root.
	dir string

	// exact makes dir a single package rather than a tree. See the package
	// comment: the two entries below take opposite values for opposite
	// reasons, and testdata separates them in both directions.
	exact bool

	// cost names what the import buys the importer, and what to do instead.
	//
	// The two costs are deliberately DIFFERENT sentences. An assertion cannot
	// see a variation two branches render identically, so a shared message
	// would leave every fixture unable to say which entry matched it.
	cost string
}

var surface = []useCase{
	{
		dir:   "internal/oms",
		exact: true,
		cost: "this package IS the OMS control surface — oms.Module carries eleven use cases (Dispatch and MarginTopUp reach a venue; Delever runs SM §7's ladder) and oms.ReconcileMarginOp appends to §5.1's hash chain from two strings, with no value a facade could wrap" +
			"; a read-only consumer takes a projection over the journal (§5.1) through a port it declares itself, and a root that must hold this authority is named in the grant list with its reason",
	},
	{
		dir:   "internal/oms/application",
		exact: false,
		cost: "these are the use cases themselves — holding one is holding §6's journal-then-apply path, which is authority over order state rather than a view of it" +
			"; a read-only consumer takes a projection over the journal (§5.1) through a port it declares itself, and a root that must hold this authority is named in the grant list with its reason",
	},
}

// ruleSix names what build-spec §3.6 rule 6 already polices AND that also falls
// inside this gate's surface, so exactly one gate reports it.
//
// There is one such tree. rule 6's other subject, oms/infrastructure/driven,
// sits under infrastructure/ and is outside both entries above, so listing it
// would add an element no input can make decide anything — the mutation would
// survive and the list would read as if it were doing twice the work it does.
// The disjointness of the two gates is therefore a property of the surface plus
// this one exclusion, and it is measured rather than asserted: §100 §15.7's
// probe, re-run against the live gate, put all three imports in one file and
// each was reported by exactly one analyzer.
var ruleSix = []string{
	"internal/oms/application/ports",
}

// grant is one package permitted to hold OMS write authority, and why.
type grant struct {
	// pkg is the package, relative to the module root, matched EXACTLY.
	pkg string

	// why records what this root does with the authority. An exemption with no
	// written argument is one that gets generalised past its cause.
	why string
}

// grants is the enumeration of who holds OMS write authority.
//
// Every entry was measured rather than assumed: `go list` over ./... reports
// exactly these three packages importing the surface above, and nothing outside
// internal/oms/ imports oms/application/ports, oms/infrastructure/driven or
// oms/infrastructure/markstate at all.
//
// All three are in the actuator's failure domain, which is the property that
// makes the list a principle rather than an inventory of what happened to
// exist. Core §11:817 draws that line and §11:826 puts the operator/dashboard
// process on the other side of it.
//
// # There WAS a fourth, and its deletion is the argument for the whole shape
//
// cmd/arblab held one until F51. Its reason read "the CLI binary — §11:826's
// actuator-domain process, as against arblab-operator, which is web-facing and
// has none of this" — and the other half of that same commit (v0.9.72, task
// #243) made `arblab serve` a cmd/arblab SUBCOMMAND and dropped
// cmd/arblab-operator/ entirely. The distinction the reason turned on stopped
// existing before the reason was a session old, and nothing failed: lint only
// ever reports a package MISSING from this list. Record §100 §4 then specified
// cmd/arblab/serve.go as the phase-1 HTTP entrypoint, so the gate built to keep
// a web-facing process out of the write path was green on the one file that
// most needed it — probed, with an ungranted control reporting in the same run.
//
// The grant existed for TEST FIXTURES: cmd/arblab/reconcile_test.go named
// application.Reconciliation to build a Report the renderer printed. The
// renderer named no such type — field access is structural in Go and needs no
// import — and no production file in the binary did either. F51's remedy moved
// the renderer and its fixtures to internal/app/reconcile, which holds the
// authority for a real reason, and deleted the grant (record §102).
//
// TestEveryGrantIsStillUsed is what turned the move into a refusal rather than
// a thing to remember: the fixtures moved, the `go list` graph lost the edge,
// and the test failed naming the grant and quoting its expired reason. It was
// written one session before it fired.
var grants = []grant{
	{
		pkg: "internal/app/pair",
		why: "§98's cross-venue pair root: it drives oms.Module through a package's worth of execution and holds oms.LegState",
	},
	{
		pkg: "internal/app/reconcile",
		why: "§7.5's reconcile-only boot, and the one caller of oms.ReconcileMarginOp — §9's Ambiguous margin-op clearance, issued offline at a terminal",
	},
	{
		pkg: "internal/app/exercise",
		why: "the exercise root drives the use cases against a demo venue; it is a bot-domain binary, not an operator one",
	},
}

// Surface exposes the policed entries so a test in this package can check the
// two cost sentences against each other.
//
// It has to read the REAL strings. The property is that no fixture's `want`
// regex can be satisfied by the wrong entry, and testdata alone cannot check it:
// if entry 2's sentence grew to CONTAIN entry 1's, every `want` in the tree
// would still match its own line and the collapse would be invisible.
func Surface() []struct {
	Dir   string
	Exact bool
	Cost  string
} {
	out := make([]struct {
		Dir   string
		Exact bool
		Cost  string
	}, 0, len(surface))
	for _, u := range surface {
		out = append(out, struct {
			Dir   string
			Exact bool
			Cost  string
		}{Dir: u.dir, Exact: u.exact, Cost: u.cost})
	}
	return out
}

// Grants exposes the list so a test in this package can check it against the
// real tree. The check is what keeps an entry from outliving its reason: lint
// itself only ever reports a package that is MISSING from the list, so a grant
// whose root stopped needing the authority stays there silently, and the next
// person to read it takes it as evidence that the authority is still held.
func Grants() []struct {
	Pkg string
	Why string
} {
	out := make([]struct {
		Pkg string
		Why string
	}, 0, len(grants))
	for _, g := range grants {
		out = append(out, struct {
			Pkg string
			Why string
		}{Pkg: g.pkg, Why: g.why})
	}
	return out
}

func run(pass *analysis.Pass) (any, error) {
	// The analyzed package is normalised once. §3.6: a rule identified by
	// position reads an external test package as the package it tests, and
	// that applies to the owner check and the grant check alike.
	self := withoutTestSuffix(pass.Pkg.Path())

	for _, f := range pass.Files {
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}

			// The module root is derived from the IMPORTED path rather than
			// from the importing one, for singlewriter's reason: an importer
			// may sit anywhere — cmd/, internal/app/, another module's tree —
			// so resolving from the analyzed package would leave every
			// importer outside internal/ unpoliced, and cmd/arblab is one.
			//
			// Sound rather than merely convenient: Go's own internal-visibility
			// rule means only code under <root>/ can import <root>/internal/…
			// at all, so a resolved root is always the importer's own module
			// tree.
			root := moduleRoot(path)
			if within(self, root+"/"+ownerTree) {
				// oms importing its own use cases is the pattern working.
				continue
			}
			if policedByRuleSix(path, root) {
				continue
			}
			for _, u := range surface {
				if !u.matches(path, root) {
					continue
				}
				if _, ok := grantedTo(self, root); ok {
					continue
				}
				pass.Reportf(spec.Path.Pos(),
					"%q is not granted OMS write authority, and %q is that authority: %s (§3.6 rule 11)",
					self, path, u.cost)
			}
		}
	}
	return nil, nil
}

// matches reports whether an imported path is this entry of the surface.
func (u useCase) matches(pkgPath, root string) bool {
	dir := root + "/" + u.dir
	if u.exact {
		return pkgPath == dir
	}
	return within(pkgPath, dir)
}

// policedByRuleSix reports whether singlewriter already covers this import.
func policedByRuleSix(pkgPath, root string) bool {
	for _, d := range ruleSix {
		if within(pkgPath, root+"/"+d) {
			return true
		}
	}
	return false
}

// grantedTo reports whether the analyzed package is a named holder of OMS write
// authority, and which entry said so.
//
// The comparison is EXACT rather than within(): a grant covers the package it
// names and not its subpackages. See the package comment — the two predicates
// agree on every path that is not a subpackage of a granted root, so testdata
// carries one that is.
func grantedTo(self, root string) (grant, bool) {
	for _, g := range grants {
		if self == root+"/"+g.pkg {
			return g, true
		}
	}
	return grant{}, false
}

// moduleRoot returns everything before the /internal/ segment of an import
// path, or the whole path when it has none.
//
// A path with no such segment is not a package this rule governs — a
// third-party dependency laid out as oms/application is not this system's OMS,
// and reporting it would be the gate inventing a violation. testdata carries
// that exact import so the scoping is measured rather than asserted.
//
// # There is no "not found" branch, and its ABSENCE is the measured part
//
// The obvious shape — `root, _, ok := Cut(...); if !ok { continue }` — is what
// singlewriter has and what this gate was written with. The mutation harness
// says it decides nothing: `if !ok` → `if !ok && false` SURVIVED, and the
// diagnosis is an equivalent mutant rather than a test gap, provable rather
// than sampled. Cut returns the WHOLE path when the separator is absent, so
// every policed dir built from that root is `root + "/internal/…"` — strictly
// longer than the import path that produced it — and cannot match it on the
// exact arm or the prefix arm. The one input that could reach the owner check,
// a package at <x>/internal/oms importing exactly <x>, takes the same `continue`
// one branch later.
//
// It is removed rather than kept-and-annotated because of the sharper cost: a
// guard that cannot fail reads in review as the thing protecting the
// third-party fixture, and the next person to simplify it has to re-derive that
// it never was. What actually protects that fixture is the arithmetic of the
// comparison, so that is where it is now stated. Same conclusion singlewriter
// reached about its own split inside withoutTestSuffix, one function over.
//
// singlewriter's twin was measured too — see F50 — and it is the same equivalent
// mutant there.
func moduleRoot(pkgPath string) string {
	root, _, _ := strings.Cut(pkgPath, internalSegment)
	return root
}

// Go builds a package's tests as up to THREE import paths, and a rule
// identified by position meets all three.
//
//   - p           the package itself, augmented with its in-package _test.go
//     files. Same path, no normalisation needed, and none wanted:
//     a *_test.go file in package main can call anything package
//     main can call, so it holds exactly the authority its package
//     holds.
//   - p_test      the external test package. §3.6 states this one.
//   - p.test      the SYNTHESIZED main that runs them, which imports p and
//     p_test and is written by the toolchain rather than by anyone.
//
// §3.6's normalisation sentence names only the second, and this gate is the
// first one able to meet the third: the generated main imports the package
// under test, and no previous gate policed a package that a test main imports —
// rule 6's subjects are oms/application/ports and oms/infrastructure/driven,
// which no generated main names. Policing internal/oms itself is what surfaced
// it, on this suite's first run, as a diagnostic against
// "github.com/giiorg/arb-lab/internal/oms.test" — the owner's own test binary
// reported for holding the owner's own authority.
//
// Both suffixes are stripped, and the reason is the same sentence §3.6 already
// carries: a test construct IS the package it tests, and the rule then applies
// to it exactly as it applies to that package. Recorded as F49, with the spec
// edit against §3.6's sentence.
const (
	externalTestSuffix  = "_test"
	generatedMainSuffix = ".test"
)

// withoutTestSuffix strips either suffix from the final segment, so a test
// construct is read as the package it tests.
//
// Copied deliberately rather than shared: tools/lint keeps each analyzer
// self-contained so a fixture failure names one gate, and hexboundary and
// singlewriter carry the same function for the same reason.
//
// A string-final _test is necessarily inside the final segment, so TrimSuffix
// already delivers the final-segment restriction that keeps a directory named
// application_test mid-path from being read as application/. singlewriter's
// mutation run settled that an explicit split adds nothing: replacing LastIndex
// with Index SURVIVED against a 21-path oracle with zero disagreements.
//
// The limit is the same one, and it is why this is not a TrimSuffix over the
// whole path: a real directory named internal/oms_test would inherit the owner
// exemption. That needs a sibling context literally named "oms_test" —
// conspicuous in review in a way an accidental import is not — while stripping
// mid-path would exempt internal/oms_test/anything.
func withoutTestSuffix(pkgPath string) string {
	// Order does not matter and cannot: Go never produces p_test.test or
	// p.test_test — the generated main is built FROM the two test packages, so
	// its path suffixes the package under test exactly once. Stated because a
	// chained TrimSuffix invites the question, and the answer is a property of
	// the toolchain rather than of this function.
	pkgPath = strings.TrimSuffix(pkgPath, generatedMainSuffix)
	return strings.TrimSuffix(pkgPath, externalTestSuffix)
}

// within reports whether pkgPath is dir itself or nested under it.
//
// Comparing full segments is what keeps internal/omsx from being read as
// internal/oms — a sibling whose name merely starts the same way would
// otherwise inherit the owner's exemption, which is the direction that fails
// silently. Both arms are exercised by testdata on both sides of the rule: the
// equality arm by internal/oms itself and by an import of internal/oms, the
// prefix arm by internal/oms/infrastructure/baseline and by an import of a
// subpackage of oms/application.
func within(pkgPath, dir string) bool {
	return pkgPath == dir || strings.HasPrefix(pkgPath, dir+"/")
}
