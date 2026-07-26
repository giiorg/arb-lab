// Package notimenow enforces build-spec §3.6 rule 5: no ambient wall-clock
// reads in domain/ or application/ code. Time is injected — §3.5's Clock models
// per-venue server-clock offset and uncertainty, and exchange server time is
// authoritative, so a domain decision made against this machine's clock is made
// against the wrong clock.
//
// Why this cannot be forbidigo configuration (§3): forbidigo would need the
// inverted form of this rule — a pattern denied everywhere, minus an exclusion
// list of the paths where reading a clock is legitimate. That list silently stops
// covering any bounded context added after it was written, which is the failure
// mode the whole gate exists to prevent. Asking "is this package a domain or
// application layer?" is structural, so a context added tomorrow is policed the
// day it appears with nothing to remember.
//
// # The whole ambient-time family, not just time.Now
//
// Rule 5 names time.Now(), and a gate that stopped there would be bypassed by
// the *idiomatic* alternative rather than a devious one:
//
//	time.Since(start)   // this IS time.Now().Sub(start)
//	time.After(d)       // schedules against the wall clock; §3.5 says SetTimer
//
// Nobody reaching for time.Since is evading anything — it is simply how you
// measure elapsed time in Go. So the rule covers every function in package time
// that reads or schedules against the clock. That is rule 5's intent rather than
// an extension of its scope: the scope is *which packages*, and that is exactly
// what build-spec says it is.
//
// # Resolved through go/types, not matched as text
//
// The check resolves each identifier to its types.Object, so
//
//	import t "time"
//	t.Now()
//
// is caught. A textual "time.Now" match would not be, and an import alias is not
// an exotic thing to find in code somebody wrote to get around a linter.
//
// It also reports a *reference*, not a call: `f := time.Now` followed by `f()`
// elsewhere is the same violation, and one that a call-shaped check would miss.
//
// # No escape hatch, unlike nofloat
//
// Deliberate, and the asymmetry has a reason. nofloat needs one because
// Prometheus takes float64 at the wire and something has to convert — there is a
// real seam with a real obligation. Here the seam is platform/clock itself, which
// sits outside domain/ and application/ by construction, so the exception is
// already expressed structurally. A domain layer that needs wall time is a
// design error rather than an exception, and the fix — inject the Clock — is
// always available. An annotation would only make it easy not to.
package notimenow

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that domain and application code never reads the wall clock

Time is injected (§3.5): exchange server time is authoritative and the Clock
models per-venue offset and uncertainty, so a domain decision made against this
machine's clock is made against the wrong one. Covers the whole ambient-time
family, not only time.Now, and resolves identifiers through go/types so an
import alias does not slip past.`

var Analyzer = &analysis.Analyzer{
	Name: "notimenow",
	Doc:  doc,
	Run:  run,
}

// policedLayers are the path segments that make a package subject to the rule.
// Segment-wise rather than substring, so a package named "domainless" is not
// caught by accident.
var policedLayers = []string{"domain", "application"}

// ambient maps each clock-reading function in package time to what to use
// instead. The value is not decoration: a gate that says only "no" gets worked
// around, and a gate that names the replacement gets followed.
var ambient = map[string]string{
	// Reads of the clock. Since and Until are Now with arithmetic attached.
	"Now":   "Clock.ServerNow(venue)",
	"Since": "Clock.ServerNow(venue) and subtract",
	"Until": "Clock.ServerNow(venue) and subtract",

	// Scheduling against the clock, which §3.5 assigns to the Clock port so
	// that the backtester can drive it.
	"Sleep":     "Clock.SetTimer",
	"After":     "Clock.SetTimer",
	"Tick":      "Clock.SetTimer",
	"NewTimer":  "Clock.SetTimer",
	"NewTicker": "Clock.SetTimer",
	"AfterFunc": "Clock.SetTimer",
}

func run(pass *analysis.Pass) (any, error) {
	layer, policed := policedLayer(pass.Pkg.Path())
	if !policed {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			// Uses rather than the syntax around it: this is what makes an
			// aliased or dot-imported time package resolve to the same object.
			fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
			if !ok {
				return true
			}
			if fn.Pkg() == nil || fn.Pkg().Path() != "time" {
				return true
			}
			// Only package-level functions are ambient. This is not a
			// formality: time.After schedules against the clock, while
			// time.Time.After is a pure comparison between two instants the
			// caller already had — and comparing two timestamps is the most
			// common legitimate thing domain code does with time. Without this
			// check the rule would report every t1.After(t2) and be turned off
			// within a week.
			if fn.Signature().Recv() != nil {
				return true
			}
			instead, banned := ambient[fn.Name()]
			if !banned {
				return true
			}
			pass.Reportf(ident.Pos(),
				"time.%s reads the wall clock, which %s code must not do: time is injected (§3.6 rule 5), so use %s",
				fn.Name(), layer, instead)
			return true
		})
	}
	return nil, nil
}

// policedLayer reports whether the package sits in a policed layer, and names it
// for the diagnostic. Nested packages count: internal/oms/domain/order is domain
// code, and a rule that only matched the leaf would be escapable by adding a
// subdirectory.
func policedLayer(pkgPath string) (string, bool) {
	for _, seg := range strings.Split(pkgPath, "/") {
		if slices.Contains(policedLayers, seg) {
			return seg, true
		}
	}
	return "", false
}
