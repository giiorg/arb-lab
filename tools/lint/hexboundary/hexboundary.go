// Package hexboundary enforces build-spec §3.6 rule 1: domain/ and
// application/ MUST NOT import infrastructure/ or engine/.
//
// This is the dependency-inversion half of ports-and-adapters, stated as a
// build failure. The inward layers define the ports; the outward layers
// implement them. An import running the other way does not break anything the
// day it lands — it compiles, the tests pass, and the cost arrives later, when
// the domain can no longer be tested without a socket and the backtest can no
// longer substitute an adapter because the domain named a concrete one.
//
// [SPEC EDIT OWED] §3.6's "Gate status" paragraph names rule 1 as deferred and
// lists three analyzers as enforced today; both are now false. §1's foundation
// roster says hexboundary "lands with the first context", which is what just
// happened, so that sentence is right and only the status is stale. Landed at
// slice close with the rest, not piecemeal.
//
// # Why this is the gate that had to exist BEFORE the first context's
// # application layer, not after it
//
// §1 makes the argument for the other three: a gate is built first "so the code
// they police is never written unpoliced". Rules 1, 6 and 10 were deferred
// against that preference for a reason that is not a weakening of it — a lint
// over an empty tree passes trivially, which is indistinguishable from a broken
// lint. That deferral expires the moment its subject exists. internal/marketdata
// has a domain/, so this gate can be shown to bite on real code, and the
// application/ layer that follows is the first hexagonal layering in the repo
// written with the check already live.
//
// # What "engine/" means here, and why the far end is resolved by module root
//
// Rule 1 names two forbidden targets and they are forbidden for different
// reasons:
//
//   - infrastructure/ is the adapter edge of the SAME architectural pattern.
//     A domain importing it inverts the port: the adapter is supposed to depend
//     on the interface, and now the interface's package depends on the adapter.
//   - engine/ is §3.5's parity seam — engine/state holds MarketView, engine/venue
//     holds the live-vs-replay adapters. Rule 1's own gloss says "strategy inputs
//     are domain-safe value types, not engine/state types (rule enforces §3.3)".
//     A domain type reaching into engine/state is what makes live and backtest
//     stop being substitutable, and parity is not a testing convenience here.
//
// Both classifications are scoped to THIS MODULE's internal/ tree rather than
// matched as a bare path segment. notimenow gets away with a segment scan
// because it only ever classifies the package under analysis, which is by
// construction ours. This gate classifies both ends of an import edge, and the
// far end can be anything on the network: a dependency published at
// example.com/thirdparty/engine is not §3.5's parity seam, and reporting it
// would be the gate inventing a violation. Scoping is the fix, and testdata
// carries that exact import so the distinction is measured rather than asserted.
//
// # Third-party imports in domain/ are NOT diagnosed, and that is a known limit
//
// The purest statement of the hexagonal rule denies the domain every dependency
// that is not the language and its own context — a domain package importing a
// database driver is the violation the pattern exists to prevent, and this gate
// would not see it. It is left out deliberately rather than overlooked. Rule 1
// as written names two path-shaped targets, widening it to "all third-party" is
// a spec change and not an implementation of the rule; and the widening has
// nothing to police today, because the engine module has zero production
// dependencies by recorded decision (§12a). Building it now would be a lint over
// an empty tree — the very thing the deferral of this gate was about. Revisit
// when the first production dependency lands, which is where the escape-hatch
// question (test-only? generated code?) can be answered against a real case
// instead of an imagined one.
//
// # Test files are policed, and the reason is a defect the fixtures found
//
// In-package test files are policed for free: they ARE the package, so a helper
// that starts life in domain/x_test.go and is later promoted to domain/x.go
// carries its imports with it, and the objection should arrive the first time
// rather than the second.
//
// External test packages took a decision, because the obvious implementation
// treats them INCONSISTENTLY and does so invisibly. Go names an external test
// package by appending _test to the last segment of the directory's import
// path. For internal/marketdata/domain that yields ".../domain_test", which
// contains no "domain" segment and escapes the rule — but for the nested
// .../domain/sub it yields ".../domain/sub_test", which still contains the
// "domain" segment and is policed. Same language construct, opposite treatment,
// and the thing deciding it is how deep the package happens to sit. That is not
// a rule anyone chose; a domain package acquires or loses lint coverage by
// being moved into a subdirectory.
//
// So classify normalises the suffix away and both are policed. The direction is
// the default-deny one this codebase takes everywhere a default has two: the
// cost of policing is that a genuine wiring test has to move, and there are two
// unpoliced homes for it (the adapter's own package under infrastructure/, or
// the context root) — whereas the cost of NOT policing is that the domain's
// entire test surface may name concrete adapters, which is how a domain becomes
// untestable without a socket one file at a time with nothing failing.
package hexboundary

import (
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that the inward layers never import the outward ones

build-spec §3.6 rule 1: domain/ and application/ must not import
infrastructure/ or engine/. The inward layers declare the ports; the outward
layers implement them. An import running the other way inverts the port and
costs nothing on the day it lands — it compiles and the tests pass — while
making the domain untestable without its adapters and breaking §3.5's
live-vs-backtest substitutability.`

var Analyzer = &analysis.Analyzer{
	Name: "hexboundary",
	Doc:  doc,
	Run:  run,
}

// internalSegment locates the module root without hardcoding it, exactly as
// kernelpurity does: the analyzed package's own path is split here, so the rule
// travels with the module rather than naming it.
const internalSegment = "/internal/"

// layer is a package's architectural role. It is deliberately a total
// classification — every in-module package gets exactly one — so that the
// importer side and the imported side are answered by ONE function. Two
// predicates, one asking "is this inward?" and another asking "is this
// outward?", is how a package comes to be neither and slip through, or both and
// be reported against itself.
type layer uint8

const (
	// layerOther is anything the rule has nothing to say about: platform/,
	// app/, a context's root package, an external _test package.
	layerOther layer = iota

	// The inward layers, subject to the rule.
	layerDomain
	layerApplication

	// The outward layers, forbidden as targets.
	layerInfrastructure
	layerEngine
)

// segments names the path segment that identifies each layer. engine is absent
// on purpose: it is identified by position (internal/engine/...), not by a
// segment anywhere in the path, because a context is free to contain a package
// called engine and that would not be §3.5's parity seam.
var segments = map[string]layer{
	"domain":         layerDomain,
	"application":    layerApplication,
	"infrastructure": layerInfrastructure,
}

// engineDir is the parity seam, relative to the module root.
const engineDir = "internal/engine"

// inward reports whether the rule polices packages in this layer.
func (l layer) inward() bool { return l == layerDomain || l == layerApplication }

// outward reports whether the rule forbids importing this layer.
func (l layer) outward() bool { return l == layerInfrastructure || l == layerEngine }

func (l layer) String() string {
	switch l {
	case layerDomain:
		return "domain"
	case layerApplication:
		return "application"
	case layerInfrastructure:
		return "infrastructure"
	case layerEngine:
		return "engine"
	case layerOther:
		return "other"
	}
	return "other"
}

// why names what the import costs, per forbidden target. A gate that says only
// "no" gets worked around; naming the consequence is the same choice notimenow
// makes in reporting the replacement rather than only the ban.
func (l layer) why() string {
	switch l {
	case layerInfrastructure:
		return "the adapter is meant to depend on the port, not the port on the adapter — declare an interface here and implement it there"
	case layerEngine:
		return "engine/ is §3.5's live-vs-replay parity seam, and a domain type naming engine/state is what makes the two stop being substitutable (§3.3) — take a domain-safe value type instead"
	case layerOther, layerDomain, layerApplication:
		return ""
	}
	return ""
}

func run(pass *analysis.Pass) (any, error) {
	self := pass.Pkg.Path()

	root, ok := moduleRoot(self)
	if !ok {
		// Not under internal/ at all — the rule cannot apply.
		return nil, nil
	}

	from := classify(self, root)
	if !from.inward() {
		// infrastructure/, engine/, platform/ and app/ may import freely.
		// Saying nothing here is what keeps the gate from over-reporting, and
		// an adapter importing the domain it adapts is the pattern working.
		return nil, nil
	}

	for _, f := range pass.Files {
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			to := classify(path, root)
			if !to.outward() {
				continue
			}
			pass.Reportf(spec.Path.Pos(),
				"%s must not import %s (%q): %s (§3.6 rule 1)",
				from, to, path, to.why())
		}
	}
	return nil, nil
}

// classify names a package's architectural layer. Anything outside this
// module's own internal/ tree — the standard library, a third-party dependency,
// this module's non-internal packages — is layerOther, so the gate never
// reports an import it does not govern.
//
// Within internal/, the FIRST layer segment wins. Path order is containment
// order, so the outermost layer directory is the one that says what a package
// is: internal/x/infrastructure/domain is an adapter that models something, not
// a domain that happens to be nested. Taking the first match rather than the
// last makes that answer independent of how deeply a package is nested, which
// is also what stops a subdirectory from being an escape hatch — internal/oms/
// domain/order is domain code, and a rule matching only the leaf would be
// evaded by adding a directory.
//
// The external-test suffix is normalised away first. See the package comment:
// without it, whether an external test package is policed depends on how deeply
// its subject sits, which is nobody's rule.
func classify(pkgPath, root string) layer {
	if !within(pkgPath, root+"/internal") {
		return layerOther
	}
	if within(pkgPath, root+"/"+engineDir) {
		return layerEngine
	}
	for _, seg := range strings.Split(withoutTestSuffix(pkgPath), "/") {
		if l, ok := segments[seg]; ok {
			return l
		}
	}
	return layerOther
}

// externalTestSuffix is what Go appends to a directory's import path to name the
// external test package compiled from its _test.go files.
const externalTestSuffix = "_test"

// withoutTestSuffix strips that suffix from the final segment, so an external
// test package classifies as the layer it tests.
//
// It is applied only to the final segment, and only there because that is the
// only place Go puts it. Stripping it anywhere in the path would be a wider
// claim than the one being made, and a real directory named application_test
// somewhere mid-path would silently start being read as application/.
func withoutTestSuffix(pkgPath string) string {
	i := strings.LastIndex(pkgPath, "/")
	last, ok := strings.CutSuffix(pkgPath[i+1:], externalTestSuffix)
	if !ok {
		return pkgPath
	}
	return pkgPath[:i+1] + last
}

// moduleRoot returns everything before the /internal/ segment of an import
// path, which is the module root for any package the internal rule lets us
// import in the first place.
func moduleRoot(pkgPath string) (string, bool) {
	root, _, ok := strings.Cut(pkgPath, internalSegment)
	return root, ok
}

// within reports whether pkgPath is dir itself or nested under it. Comparing
// full segments avoids matching a sibling whose name merely starts the same
// way, e.g. internal/engineering.
func within(pkgPath, dir string) bool {
	return pkgPath == dir || strings.HasPrefix(pkgPath, dir+"/")
}
