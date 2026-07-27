// Package singlewriter enforces build-spec §3.6 rule 6: only oms mutates
// order/fill/trade/position state, through §6's journal-then-apply path, and no
// other context may import the OMS write repositories.
//
// [SPEC EDIT LANDED — 2026-07-27] §3.6's "Gate status" paragraph named rule 6
// as the last deferred gate, "until oms's write repository exists — a lint over
// an empty tree passes trivially, which is indistinguishable from a broken
// lint." That deferral expires when its subject appears, and its subject
// appeared this session: internal/oms/application/ports declares
// OrderWriteRepository and internal/oms/infrastructure/driven implements it.
// The gate lands in the same session rather than after it, as rules 1 and 10
// did.
//
// The trigger named the right construct, which is worth recording because rule
// 10's did not: it waited on a state machine while what it actually read was
// the kernel's enum-likes, and it found 13 silent fallthroughs that had been
// reachable for several slices. Rule 6 named "the write repository", and the
// write repository is exactly what this analyzer reads.
//
// # What is policed, and why the port is policed as well as the adapter
//
// Two package trees, both by POSITION rather than by naming a type:
//
//   - internal/oms/infrastructure/driven — the repositories themselves, rule
//     6's literal subject.
//   - internal/oms/application/ports — the interfaces they satisfy.
//
// Policing only the concrete adapter was considered and is the reading that
// looks stricter than it is. Depending on the abstraction rather than the
// implementation is the recommended way to depend on anything, so a gate that
// forbade only infrastructure/driven would forbid the shape nobody writes and
// permit the shape everybody does: a context that held ports.OrderWriteRepository
// would have full write authority over order state with a clean lint run.
//
// The position is the subject for the same reason the marker alternative was
// refused. A //arblab:writerepository comment on each interface is more precise
// and fails in the wrong direction: the repository somebody adds next year
// without the marker is silently unpoliced, which is a default whose failure
// mode is invisible. A TradeWriteRepository added to ports/ is covered the day
// it is declared, by nobody remembering anything.
//
// # What is NOT policed, and each absence is a decision
//
//   - internal/oms/domain. §5.1 makes audit and reconcile "projections over the
//     journal, never co-owners", and a projection naming domain.OrderState is
//     that pattern working. Denying it would push a read model into declaring
//     its own copy of the twelve-state set — a second home for SM §1's state
//     names, which is a worse outcome than the import.
//   - internal/oms/application. Rule 6 forbids BYPASSING the write path, not
//     using it: a use case IS the journal-then-apply sequence, so calling one
//     is what the rule wants. The ports and the repositories are the two ways
//     to write order state without going through it, and they are exactly what
//     is listed above.
//   - Other contexts' infrastructure/driven. Rule 6 is about the OMS's write
//     authority specifically. "No context may import another context's
//     internals" is a wider rule that might well be right, and it is not this
//     one; widening it is a spec edit rather than an implementation of it.
//
// # The composition root is policed too, and that is the deliberate reading
//
// Rule 6's wording is "forbids other contexts", and internal/app is not a
// context — it is §3.6 rule 7's hand-wiring seam. It is policed anyway, because
// the alternative direction fails silently: a root that may hold a repository
// may pass it to another context's constructor, and the receiving context needs
// no import at all to accept it (see the structural-typing limit below). Since
// each context wires its own adapters in module.go — the marketdata precedent —
// a root never needs to name one, and if one ever does, the answer is to move
// the type it needs rather than to widen this gate.
//
// # The limit this gate has, stated rather than left to be discovered
//
// It polices DECLARED import edges. Go's interfaces are structural, so a
// determined context can satisfy ports.OrderWriteRepository, or hand its own
// journal to application.NewDispatcher, without ever naming a policed package
// — and this analyzer will not see it. What the gate makes impossible is doing
// it by ACCIDENT and doing it INVISIBLY: the shapes above are conspicuous in
// review precisely because they are the long way round, whereas an import is
// the short way and is what a hurried change actually reaches for.
package singlewriter

import (
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that only oms imports the OMS write path

build-spec §3.6 rule 6: only oms mutates order/fill/trade/position state,
through the journal-then-apply path of §6. A second writer does not corrupt
anything on the day it lands — it writes plausible rows — and the cost arrives
at §7.5's reconcile-only boot, which rebuilds state from the journal and finds
venue truth it has no record of authorising.`

var Analyzer = &analysis.Analyzer{
	Name: "singlewriter",
	Doc:  doc,
	Run:  run,
}

// internalSegment locates the module root without hardcoding it, as
// kernelpurity and hexboundary both do, so the rule travels with the module
// rather than naming it.
const internalSegment = "/internal/"

// owner is the one context permitted to import its own write path, relative to
// the module root.
const owner = "internal/oms"

// authority is one tree of the OMS write path and what importing it costs.
//
// The cost is carried because a gate that says only "no" gets worked around;
// naming the consequence and the alternative is what hexboundary and notimenow
// both do, and it is the difference between a rule and an obstacle.
type authority struct {
	// dir is the package tree, relative to the module root.
	dir string

	// cost names what the import buys the importer, and what to do instead.
	cost string
}

var writePath = []authority{
	{
		dir: "internal/oms/application/ports",
		cost: "these interfaces ARE the write authority — a value of one can journal a state change or place an order without the OMS knowing" +
			"; observe orders through a projection over the journal (§5.1), or have oms drive a port you declare in your own application/ports",
	},
	{
		dir: "internal/oms/infrastructure/driven",
		cost: "this is the write repository itself, and §6's journal-then-apply ordering is not something a second writer can honour on its own" +
			"; observe orders through a projection over the journal (§5.1), or have oms drive a port you declare in your own application/ports",
	},
}

func run(pass *analysis.Pass) (any, error) {
	self := pass.Pkg.Path()

	for _, f := range pass.Files {
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}

			// The module root is derived from the IMPORTED path, not from the
			// importing one, and that asymmetry is the point: an importer may
			// sit anywhere — cmd/, internal/app/, another module's tree — so
			// resolving the root from the analyzed package would leave every
			// importer outside internal/ unpoliced. hexboundary can resolve
			// from the importer because its rule only applies to packages
			// under internal/ by construction; this one does not have that.
			//
			// Deriving from the import is sound rather than merely convenient:
			// Go's own internal-visibility rule means only code under <root>/
			// can import <root>/internal/… at all, so a resolved root is
			// always the importer's own module tree. There is deliberately no
			// second check of that — an unreachable branch no fixture can
			// exercise is the shape this repo has already paid for twice.
			root, ok := moduleRoot(path)
			if !ok {
				continue
			}
			if within(self, root+"/"+owner) {
				// oms importing its own write path is the pattern working.
				continue
			}
			for _, a := range writePath {
				if !within(path, root+"/"+a.dir) {
					continue
				}
				pass.Reportf(spec.Path.Pos(),
					"only oms may import the OMS write path (%q): %s (§3.6 rule 6)",
					path, a.cost)
			}
		}
	}
	return nil, nil
}

// moduleRoot returns everything before the /internal/ segment of an import
// path.
//
// A path with no such segment is not a package this rule governs — a
// third-party dependency that happens to be laid out as oms/infrastructure/
// driven is not this system's OMS, and reporting it would be the gate
// inventing a violation. testdata carries that exact import so the scoping is
// measured rather than asserted.
func moduleRoot(pkgPath string) (string, bool) {
	root, _, ok := strings.Cut(pkgPath, internalSegment)
	return root, ok
}

// within reports whether pkgPath is dir itself or nested under it.
//
// Comparing full segments is what keeps internal/omsx from being read as
// internal/oms — a sibling whose name merely starts the same way would
// otherwise inherit the one context's exemption from this rule, which is the
// direction that fails silently. Both arms are exercised by testdata: the
// equality arm by an import of a policed package root, the prefix arm by an
// import of a package beneath it.
func within(pkgPath, dir string) bool {
	return pkgPath == dir || strings.HasPrefix(pkgPath, dir+"/")
}
