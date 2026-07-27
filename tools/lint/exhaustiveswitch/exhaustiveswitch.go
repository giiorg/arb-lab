// Package exhaustiveswitch enforces build-spec §3.6 rule 10: a switch over a
// rule-8 enum-like MUST name every declared member.
//
// The point is stated in the rule itself — "adding a state to a safety machine
// breaks compilation at every decision site instead of falling through to
// default". Go has no sum types, so a new enum member is not a compile error
// anywhere; it is a value that silently takes whichever branch the author of
// each switch happened to write last. The safety kernel is seven state machines
// whose formal model has already produced four counterexample-backed defects,
// and a fifth arriving as a silent fallthrough is not a class of bug worth
// leaving available.
//
// [SPEC EDIT LANDED — v0.9.8] Rule 10 carried "(planned, not yet enforced — no
// state machines exist to police)" and closed with "Evaluate golangci-lint's
// exhaustive first; build a custom analyzer only if it does not suffice". Both
// are discharged: the evaluation was run and is recorded below, and the rule is
// enforced. §3.6 now states rule 10 as enforced, that a `default:` does not
// excuse a missing member, and that there is no escape directive. It also keeps
// the part worth more than the status change — the deferral's stated trigger
// was WRONG. It waited on a STATE MACHINE, while what actually made the gate
// buildable was the shared kernel's eleven enum-likes, which had existed for
// several slices while the rule described itself as having nothing to police.
// A deferral should name the construct the analyzer reads, not the feature that
// motivated it, and rule 6's trigger was re-checked against that.
//
// # golangci-lint's `exhaustive` was evaluated first, as the rule instructs
//
// It was built against this module's x/tools and run over ./internal/..., and
// the numbers are why this package exists rather than a dependency:
//
//	defaults ........................ 18 reports
//	  of which reflect.Kind ..........  3   stdlib, never rule 10's subject
//	  of which test-declared enums ...  3   partial by design
//	  genuinely rule 10's ............ 12
//	-package-scope-only ............. 18   no effect: it means "members declared
//	                                       at package scope", not "enums from
//	                                       this package"
//	-default-signifies-exhaustive ... 15   inverts rule 10's stated purpose
//
// It finds the right things. What it cannot express is rule 10's SCOPE — "a
// rule-8 enum-like", meaning an enum this module declares. Its one scoping knob
// is -ignore-enum-types, a regex exclusion list that has to grow every time
// somebody switches on a new stdlib enum, and that is the precise mechanism
// §3.6 already rejects by name for forbidigo: an exclusion list silently stops
// covering whatever is added after it was written. Asking "was this enum
// declared in this module?" is structural, so an enum added tomorrow is policed
// the day it appears with nothing to remember.
//
// One measurement was worth more than the verdict: only 3 of the 18 sites use a
// `default:` clause at all. This codebase's idiom is a switch with no default
// followed by a trailing return, which is a fallthrough wearing different
// clothes — and `exhaustive` is right not to treat it as a default. Rule 10
// bites harder here than its own wording ("instead of falling through to
// default") anticipates.
//
// # A `default:` clause does not excuse a missing member
//
// That is the whole rule, so it is worth saying plainly. A default is where the
// new member lands silently; requiring it to be named instead is the entire
// mechanism. A switch may still HAVE a default — for a value outside the
// declared set, which Go's type system permits — it just may not use it as a
// substitute for deciding.
//
// # Scope: enums declared in non-test files, switches anywhere
//
// The two halves are deliberately different, because the two hazards are.
//
// An enum declared in a _test.go file is out of scope. platform/clock's test
// suite is the case that decided it: a `defect` enum of ten injectable faults,
// with a broken-clock double whose every method implements the two or three
// faults it can express and delegates the rest to the correct implementation.
// Exhaustiveness there is roughly forty no-op case labels across five methods,
// and a gate that produces that is a gate somebody turns off.
//
// A switch in a _test.go file over a PRODUCTION enum is in scope, and that is
// not a technicality. fixedpoint's math/big oracle dispatches on RoundingMode;
// a rounding mode added without a matching oracle branch is a mode whose
// independent implementation silently stops existing, while the suite goes on
// reporting green. That is this repo's recurring failure shape, and it is worth
// a lint precisely because it is invisible.
//
// # Members are grouped by VALUE, not by name
//
// Enums here carry bound sentinels — `maxUnit = UnitMoney`, `maxFreshness =
// FreshnessFresh` — which are the same member under a second name. Naming
// either one covers both. Without value-grouping the gate would demand a case
// for a constant that is already handled and, worse, would demand an
// UNEXPORTED one from a package that cannot name it.
//
// Cross-package, an unexported member that is not an alias cannot be required,
// because the switch has no way to write it. That is a real asymmetry and it is
// the correct one: the gate can only ask for what is nameable. An enum with
// unexported members that crosses a package boundary is under-policed, and the
// fix is to the enum rather than to this gate.
//
// # No escape hatch, and that is the deliberate half
//
// nofloat and notimenow both have one. This does not, because the argument that
// justified theirs runs the other way here. Their escape exists for a case with
// no in-language alternative — a live-only socket deadline has nowhere else to
// go. A switch that genuinely handles a subset of an enum has an alternative
// that is strictly better than an annotation: name the remaining members in one
// grouped case with a comment. That documents the decision at the decision
// site, in code the compiler checks, rather than in a comment it does not.
//
// The asymmetry that made notimenow's escape cheap to add early also makes this
// one cheap to add LATE: an escape is purely additive, so if a genuine case
// appears, nothing already written has to change. Shipping one now would
// instead have meant the thirteen existing sites became thirteen annotations
// rather than thirteen decisions.
package exhaustiveswitch

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that a switch over a module-declared enum names every member

build-spec §3.6 rule 10. Go has no sum types, so a new enum member is not a
compile error anywhere — it silently takes whichever branch each switch happens
to end with. A default: clause does not excuse a missing member; it is where the
new member lands silently, which is the thing being prevented. Scoped to enums
this module declares outside test files, so stdlib enums and test-fixture enums
are not its business. No escape annotation: a subset switch should name the
remaining members in one grouped case, which the compiler checks.`

var Analyzer = &analysis.Analyzer{
	Name: "exhaustiveswitch",
	Doc:  doc,
	Run:  run,
}

const (
	internalSegment = "/internal/"
	testFileSuffix  = "_test.go"
)

// minMembers is how many same-typed constants make a named type an enum-like.
// One constant of a named type is a named constant, not a set to be exhausted
// over, and requiring a lone member would report every `switch someID` against
// a package that happens to declare one sentinel.
const minMembers = 2

func run(pass *analysis.Pass) (any, error) {
	root, ok := moduleRoot(pass.Pkg.Path())
	if !ok {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok || sw.Tag == nil {
				// A tagless switch is a chain of booleans, not a dispatch over
				// a value, so there is no member set to exhaust.
				return true
			}
			check(pass, root, sw)
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, root string, sw *ast.SwitchStmt) {
	named, ok := enumType(pass, root, sw.Tag)
	if !ok {
		return
	}

	members := membersOf(named)
	if len(members) < minMembers {
		return
	}

	covered, ok := coveredValues(pass, sw)
	if !ok {
		// At least one case expression is not a constant, so what the switch
		// covers cannot be decided statically. Saying nothing is the right
		// failure: this gate's diagnostics are build failures, and a gate that
		// guesses gets configured away.
		return
	}

	var missing []string
	for _, v := range sortedKeys(members) {
		if !covered[v] {
			missing = append(missing, members[v])
		}
	}
	if len(missing) == 0 {
		return
	}

	pass.Reportf(sw.Pos(),
		"switch over %s does not name %s: every declared member must be named so that adding one breaks the build here rather than falling through (§3.6 rule 10)",
		named.Obj().Name(), strings.Join(missing, ", "))
}

// enumType resolves a switch tag to the enum-like it dispatches on, if it is
// one this rule governs.
func enumType(pass *analysis.Pass, root string, tag ast.Expr) (*types.Named, bool) {
	tv, ok := pass.TypesInfo.Types[tag]
	if !ok {
		return nil, false
	}
	named, ok := types.Unalias(tv.Type).(*types.Named)
	if !ok {
		return nil, false
	}

	// Rule 8's enum-likes are integer enums. Strings are admitted too: a
	// venue or symbol enum written as a string type is the same construct with
	// the same hazard, and excluding it would make the gate's coverage depend
	// on a representation choice rather than on what the type is for.
	basic, ok := named.Underlying().(*types.Basic)
	if !ok {
		return nil, false
	}
	if basic.Info()&(types.IsInteger|types.IsString) == 0 {
		return nil, false
	}

	obj := named.Obj()
	if obj.Pkg() == nil {
		return nil, false
	}
	// Declared by this module, under internal/. This is what excludes
	// reflect.Kind and every other enum the rule was never about.
	if !strings.HasPrefix(obj.Pkg().Path(), root+internalSegment) {
		return nil, false
	}
	// Declared outside a test file. See the package comment: a test-fixture
	// enum's partial switches are the design, not a defect.
	if strings.HasSuffix(pass.Fset.Position(obj.Pos()).Filename, testFileSuffix) {
		return nil, false
	}
	return named, true
}

// membersOf returns the enum's declared members, keyed by constant value so
// that a bound sentinel aliasing a real member does not become a second thing
// to name. The retained name is the one a switch should write: exported wins,
// and among equals the shortest, so `maxUnit = UnitMoney` reports as UnitMoney.
func membersOf(named *types.Named) map[string]string {
	scope := named.Obj().Pkg().Scope()
	members := make(map[string]string)
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok || !types.Identical(c.Type(), named) {
			continue
		}
		key := c.Val().ExactString()
		if prev, seen := members[key]; !seen || betterName(name, prev) {
			members[key] = name
		}
	}
	return members
}

// betterName reports whether a should be reported in preference to b.
func betterName(a, b string) bool {
	if ax, bx := ast.IsExported(a), ast.IsExported(b); ax != bx {
		return ax
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// coveredValues collects the constant values the switch names. The bool reports
// whether every case expression was resolvable; a false means the switch cannot
// be judged and must be left alone.
func coveredValues(pass *analysis.Pass, sw *ast.SwitchStmt) (map[string]bool, bool) {
	covered := make(map[string]bool)
	for _, stmt := range sw.Body.List {
		// Semantically equivalent under mutation, and kept anyway: Go's grammar
		// admits nothing but case clauses in a switch body, so forcing ok to
		// true changes no behaviour and the suite correctly does not fail. It
		// stays because the alternative spelling is a bare assertion that
		// panics, and this gate's other nil-guard mutation already demonstrated
		// what a panicking analyzer costs — a PANIC!! verdict where the claim
		// under test goes unchecked.
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			return nil, false
		}
		// A nil List is the default clause. It is skipped rather than treated
		// as coverage — that is the rule.
		for _, expr := range clause.List {
			// A nil Value is how go/types reports "not a constant", which is
			// the only unjudgeable case reachable here — analysistest will not
			// load a package that does not typecheck, so a constant of unknown
			// kind cannot be constructed as a fixture. The check against it was
			// dropped rather than kept unmeasured: an untestable branch in a
			// build-failing gate is indistinguishable from a broken one.
			tv, ok := pass.TypesInfo.Types[expr]
			if !ok || tv.Value == nil {
				return nil, false
			}
			covered[tv.Value.ExactString()] = true
		}
	}
	return covered, true
}

// sortedKeys orders the member keys so a diagnostic listing several missing
// members reads the same on every run. Sorted by the member NAME rather than by
// the raw constant value, because the value's exact string sorts numerically
// only by accident and the name is what the reader has to go and write.
func sortedKeys(members map[string]string) []string {
	keys := make([]string, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return members[keys[i]] < members[keys[j]] })
	return keys
}

// moduleRoot returns everything before the /internal/ segment of an import
// path, which is the module root for any package the internal rule lets us
// import in the first place.
func moduleRoot(pkgPath string) (string, bool) {
	root, _, ok := strings.Cut(pkgPath, internalSegment)
	return root, ok
}
