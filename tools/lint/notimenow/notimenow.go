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
// # engine/ is policed too, because a clamp is only a guarantee if everyone
// # reads the same clock
//
// Rule 5's original scope was domain/ and application/. It reaches engine/ as
// well, and the argument is not parity purity — it is platform/clock's monotonic
// clamp. Wall absorbs backwards time rather than erroring (a stalled clock beats
// a re-evaluated settlement); time.Now does not clamp at all. So an engine
// adapter that stamps RecvTime from the raw clock, feeding a freshness
// computation that reads the injected one, produces a subtraction between two
// clocks that disagree exactly across an NTP step backwards — yielding a
// negative or inflated age in code where freshness gates execution (§3.4) and
// stale marks drive liq-distance. The defect is two clocks meeting at a
// subtraction; the ambient call is only where it starts.
//
// The scope stays structural rather than becoming a list of the engine packages
// that run under both live and replay wiring. That list would be sharper against
// today's hazard and would silently stop covering the next such package, which
// is the failure mode this whole gate exists to prevent — the argument does not
// get an exemption when it is inconvenient to itself.
//
// # An escape, but only in engine/
//
// The asymmetry is the point. In domain/ and application/ a wall-clock read is a
// design error, injecting the Clock is always available as the fix, and an
// annotation would only make it easy not to. That reasoning does not transfer to
// transport: a socket read deadline has no venue-server-time semantics, never
// runs under replay (§3.5 substitutes the adapter wholesale), and must fire on
// real elapsed time even when the dispatcher is wedged. Routing it through
// Clock.SetTimer would put transport liveness on the *dispatch* timer queue —
// worse architecture, not stricter discipline.
//
//	//notimenow:allow <reason>
//
// suppresses the line, in engine/ only. Written in a policed layer that has no
// escape it suppresses nothing and says so, because a directive that silently
// did nothing would read as a working suppression to whoever wrote it. The
// grammar matches nofloat's deliberately — mandatory reason, unused escape
// reported, misspelled directive inert — and is duplicated rather than shared
// because only the comment parsing is common: the policy differs (nofloat allows
// an escape anywhere under internal/, this one only in engine/), and a shared
// helper wrapping thirty lines of parsing under two different policies would be
// an abstraction over the wrong seam. Worth extracting at a third gate.
//
// This is a prediction about code not yet written: that engine/ transport will
// need wall-clock scheduling. If engine/venue ships and no escape is ever used,
// the prediction was wrong and the mechanism should be deleted — cheap, whereas
// retrofitting a missing escape after engine/state ships is not. That asymmetry
// is the whole reason the escape exists rather than absolute denial.
package notimenow

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that domain, application and engine code never reads the wall clock

Time is injected (§3.5): exchange server time is authoritative and the Clock
models per-venue offset and uncertainty, so a domain decision made against this
machine's clock is made against the wrong one. In engine/ the hazard is the
monotonic clamp — an age computed between the raw clock and the injected one is
wrong in exactly the case the clamp exists for. Covers the whole ambient-time
family, not only time.Now, and resolves identifiers through go/types so an
import alias does not slip past. Live-only transport in engine/ may annotate a
genuine exception with //notimenow:allow <reason>.`

var Analyzer = &analysis.Analyzer{
	Name: "notimenow",
	Doc:  doc,
	Run:  run,
}

// policedLayers are the path segments that make a package subject to the rule.
// Segment-wise rather than substring, so a package named "domainless" is not
// caught by accident.
var policedLayers = []string{"domain", "application", "engine"}

// escapableLayer is the one policed layer where an annotated exception is
// available. A single constant rather than a set: the moment this becomes a list
// it is the exclusion list the package comment argues against.
const escapableLayer = "engine"

const (
	// directiveBody is matched against a comment's text with its opener
	// stripped, so the // and /* */ forms share one definition.
	directiveBody = "notimenow:allow"

	// directive is the spelling used in diagnostics. Messages name the line
	// form because that is the one to reach for; the block form exists so a
	// fixture can carry a directive and an assertion on one line.
	directive = "//" + directiveBody
)

// srcLine identifies a source line across the package's files. It is the unit
// the escape works in, and therefore the unit findings are reduced to: diagnosing
// per-reference while suppressing per-line would let one annotation silently
// answer several findings.
type srcLine struct {
	file string
	num  int
}

type finding struct {
	pos  token.Pos
	fn   string
	verb string
}

type escape struct {
	pos    token.Pos
	reason string
}

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
	// An escape written in an unpoliced package is left alone rather than
	// reported as unused: nothing there is policed, so the annotation is inert
	// text and a diagnostic would be the gate commenting on code it does not
	// govern.
	escapable := layer == escapableLayer

	escapes := collectEscapes(pass)
	found := collectAmbient(pass)

	for _, k := range sortedLines(found) {
		if escapable {
			if e, ok := escapes[k]; ok && e.reason != "" {
				continue
			}
		}
		f := found[k]
		if escapable {
			pass.Reportf(f.pos,
				"time.%s reads the wall clock, which %s code must not do: time is injected (§3.6 rule 5), so use %s; if this is live-only transport, annotate it with %s <reason>",
				f.fn, layer, f.verb, directive)
			continue
		}
		pass.Reportf(f.pos,
			"time.%s reads the wall clock, which %s code must not do: time is injected (§3.6 rule 5), so use %s",
			f.fn, layer, f.verb)
	}

	for _, k := range sortedEscapes(escapes) {
		e := escapes[k]
		if !escapable {
			pass.Reportf(e.pos,
				"%s is not available in %s code: rule 5 admits no exception there, so this suppresses nothing — inject the Clock instead",
				directive, layer)
			continue
		}
		if _, used := found[k]; !used {
			pass.Reportf(e.pos,
				"unused %s: no ambient clock read on this line, so this annotation would silently authorise the next one to land here",
				directive)
			continue
		}
		if e.reason == "" {
			pass.Reportf(e.pos,
				"%s needs a reason: an exception without one is an unexplained hole, so this does not exempt anything",
				directive)
		}
	}

	return nil, nil
}

// collectAmbient reduces every ambient-time reference to at most one finding per
// line, kept at the earliest position on that line.
func collectAmbient(pass *analysis.Pass) map[srcLine]finding {
	found := make(map[srcLine]finding)
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
			at := pass.Fset.Position(ident.Pos())
			k := srcLine{at.Filename, at.Line}
			if prev, seen := found[k]; !seen || ident.Pos() < prev.pos {
				found[k] = finding{ident.Pos(), fn.Name(), instead}
			}
			return true
		})
	}
	return found
}

// collectEscapes indexes the annotations by line before anything is diagnosed,
// so that matching a read to its annotation does not depend on walk order.
func collectEscapes(pass *analysis.Pass) map[srcLine]escape {
	escapes := make(map[srcLine]escape)
	for _, f := range pass.Files {
		for _, group := range f.Comments {
			for _, c := range group.List {
				reason, ok := escapeReason(c.Text)
				if !ok {
					continue
				}
				at := pass.Fset.Position(c.Slash)
				escapes[srcLine{at.Filename, at.Line}] = escape{c.Slash, reason}
			}
		}
	}
	return escapes
}

// escapeReason reports whether the comment is the escape directive and, if so,
// the reason following it. The separator check is what stops
// //notimenow:allowance from reading as an escape with the reason "ance".
func escapeReason(text string) (string, bool) {
	body, ok := commentBody(text)
	if !ok {
		return "", false
	}
	rest, ok := strings.CutPrefix(body, directiveBody)
	if !ok {
		return "", false
	}
	if rest != "" && !isSpace(rest[0]) {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// commentBody strips a comment's opener and closer. ast.Comment.Text keeps them,
// and both forms have to reach the same directive check.
func commentBody(text string) (string, bool) {
	if body, ok := strings.CutPrefix(text, "//"); ok {
		return body, true
	}
	if body, ok := strings.CutPrefix(text, "/*"); ok {
		return strings.TrimSuffix(body, "*/"), true
	}
	return "", false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t'
}

// Both sorts exist so the driver's output is deterministic; analysistest matches
// by line and would not notice, but a gate whose output reorders between runs is
// a gate whose diff is unreadable.

func sortedLines(m map[srcLine]finding) []srcLine {
	keys := make([]srcLine, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]].pos < m[keys[j]].pos })
	return keys
}

func sortedEscapes(m map[srcLine]escape) []srcLine {
	keys := make([]srcLine, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]].pos < m[keys[j]].pos })
	return keys
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
