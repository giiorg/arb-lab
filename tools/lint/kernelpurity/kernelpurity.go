// Package kernelpurity enforces the shared-kernel bright line.
//
// A package under internal/platform/ may import only the standard library and
// other internal/platform/ packages. It catches two distinct failure modes
// that happen to have the same fix:
//
//   - Domain leak. The kernel learning a business concept (importing a bounded
//     context) turns a technical shared library into a shared *domain* kernel —
//     the most tightly coupled integration pattern DDD describes, and the one
//     it warns against. An 11-package kernel makes this a live risk.
//
//   - Spine inversion. engine/ already sits on top of platform/ (engine/venue
//     consumes fixedpoint, clock and ids), so a platform -> engine import
//     inverts the dependency spine and invites an import cycle.
//
// Scope, deliberately: only imports *inside this module* are diagnosed.
// Third-party imports are out of scope because platform/observability
// legitimately needs prometheus/client_golang (§11). Tightening that would
// need an annotation escape like nofloat's, and is not part of this rule.
//
// This rule is NOT one of the eight in build-spec §3.6 — it was added during
// implementation of milestone 1 and is recorded as such in adr-history §3.
package kernelpurity

import (
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that the shared kernel stays technical

A package under internal/platform/ may import only the standard library and
other internal/platform/ packages. Reaching into a bounded context makes the
kernel a shared domain kernel; reaching into engine/ inverts the dependency
spine.`

var Analyzer = &analysis.Analyzer{
	Name: "kernelpurity",
	Doc:  doc,
	Run:  run,
}

// internalSegment is how we locate the module root without hardcoding it: the
// analyzed package's own path is split at /internal/, so the rule travels with
// the module rather than naming it.
const internalSegment = "/internal/"

// kernelDir is the shared kernel, relative to the module root.
const kernelDir = "internal/platform"

func run(pass *analysis.Pass) (any, error) {
	self := pass.Pkg.Path()

	// A package outside internal/ cannot be within the kernel and returns
	// below; see moduleRoot for why that needs no branch of its own.
	root := moduleRoot(self)
	kernel := root + "/" + kernelDir
	if !within(self, kernel) {
		// Not a kernel package. Contexts and engine/ may import freely; saying
		// nothing here is what keeps the gate from over-reporting.
		return nil, nil
	}

	for _, f := range pass.Files {
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			// Outside this module (stdlib or third-party): out of scope.
			if !strings.HasPrefix(path, root+"/") {
				continue
			}
			if within(path, kernel) {
				continue
			}
			pass.Reportf(spec.Path.Pos(),
				"platform must not import %q (the shared kernel may import only the standard library and other %s packages)",
				path, kernelDir)
		}
	}
	return nil, nil
}

// moduleRoot returns everything before the /internal/ segment of an import
// path, which is the module root for any package the internal rule lets us
// import in the first place.
//
// # There is no "not found" branch, and its ABSENCE is the measured part
//
// The obvious shape — `root, ok := moduleRoot(self); if !ok { return }` — was
// here and decided nothing: `if !ok` → `if !ok && false` SURVIVED, and the
// diagnosis is an equivalent mutant rather than a test gap, provable rather
// than sampled. Cut returns the WHOLE path when the separator is absent, so
// every dir built from that root is `root + "/internal/…"` — strictly longer
// than the path that produced it — and the classification below answers the
// same way with the branch as without it.
//
// Removed rather than kept-and-annotated, for the reason this file already
// gives about its own withoutTestSuffix one function down: a guard that cannot
// fail reads in review as the thing providing a protection the arithmetic was
// providing all along, and the next person to simplify it has to re-derive that
// it never was. Found in omsauthority (§3.6 rule 11) and carried to all four
// gates the same session, under this repo's rule that a fix applied in one
// place is applied to its mirror immediately. Recorded as F50.
func moduleRoot(pkgPath string) string {
	root, _, _ := strings.Cut(pkgPath, internalSegment)
	return root
}

// within reports whether pkgPath is dir itself or nested under it. Comparing
// full segments avoids matching a sibling whose name merely starts the same
// way, e.g. internal/platformx.
func within(pkgPath, dir string) bool {
	return pkgPath == dir || strings.HasPrefix(pkgPath, dir+"/")
}
