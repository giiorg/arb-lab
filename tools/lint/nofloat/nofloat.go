// Package nofloat enforces the float half of build-spec §3.6 rule 4: no
// binary floating point anywhere under internal/.
//
// Why this cannot be golangci-lint configuration. The rule is not "the token
// float64 does not appear" — it is "no value in this code is a float", which
// needs type information. A regex over source text misses all three of the ways
// a float actually arrives:
//
//	x := 1.5                  // an untyped constant becoming float64 by inference
//	return a / b              // both operands float-typed; no float token in sight
//	elapsed := d.Seconds()    // a float64 handed over by the standard library
//
// The third is how it happens in practice. time.Duration.Seconds() and
// strconv.ParseFloat are the doors, and neither spells "float" at the call site.
// So this analyzer asks go/types what each expression *is*, which makes the rule
// sound rather than sound-for-the-cases-someone-listed.
//
// Default-deny with an annotated escape. A float is a diagnostic anywhere under
// internal/ unless its line carries
//
//	//nofloat:allow <reason>
//
// Rejected alternatives (§3): a configured list of money-path packages — a
// package nobody remembers to add is unpoliced, which is the same failure mode
// that disqualified depguard; and zero-exception absolute denial, which would
// force a conversion shim outside internal/ just for the Prometheus seam.
// Anticipated legitimate escapes: platform/observability metric emission, and
// later engine/backtest statistics.
//
// Three things about the escape are deliberate:
//
//   - The reason is mandatory. An escape without one is not an exception, it is
//     an unexplained hole, so a bare //nofloat:allow does not exempt anything.
//   - An escape on a line with no float is reported as unused. Otherwise it
//     survives the deletion of the float it was written for and silently
//     pre-authorises the next one to land on that line.
//   - A misspelled directive (//nofloat:allowance) is simply not a directive.
//     That needs no diagnostic of its own: the float diagnostic it failed to
//     suppress is still there, which is how the author finds the typo.
//
// Both /*nofloat:allow ...*/ and //nofloat:allow ... are recognised, and the
// block form is not decoration. Go allows one // comment per line, so
//
//	var x float64 //nofloat:allow r // want "..."
//
// is a *single* comment whose reason reads "r // want ...", and this analyzer's
// own analysistest fixtures could not express a malformed escape at all. A rule
// whose test cannot be written in the rule's own syntax is a rule that goes
// untested. Greppability is unaffected: `grep -rn nofloat:allow` finds both.
package nofloat

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = `check that no binary floating point is used under internal/

Money is fixedpoint (§3.6 rule 4). A float64 in a money path is a silently wrong
number, not a crash. The check is type-based, so it also catches an untyped
constant that becomes a float on assignment and a float handed over by the
standard library. Annotate a genuine exception with //nofloat:allow <reason>.`

var Analyzer = &analysis.Analyzer{
	Name: "nofloat",
	Doc:  doc,
	Run:  run,
}

const (
	// directiveBody is matched against a comment's text with its opener
	// stripped, so that the // and /* */ forms share one definition.
	directiveBody = "nofloat:allow"

	// directive is the spelling used in diagnostics. Messages name the line
	// form because that is the one to reach for; the block form exists for the
	// fixture reason given in the package comment.
	directive = "//" + directiveBody
)

// internalSegment scopes the rule without hardcoding the module path, the same
// way kernelpurity locates the module root. analysistest fixtures live in a
// GOPATH-style testdata/src/ tree, so a hardcoded path would pass in tests and
// mis-scope in production.
const internalSegment = "/internal/"

// srcLine identifies a source line across the package's files. It is the unit
// the whole analyzer works in, because it is the unit the escape hatch works in:
// diagnosing per-expression while suppressing per-line would mean one annotation
// silently answering several findings.
type srcLine struct {
	file string
	num  int
}

type finding struct {
	pos token.Pos
	typ types.Type
}

type escape struct {
	pos    token.Pos
	reason string
}

func run(pass *analysis.Pass) (any, error) {
	if !underInternal(pass.Pkg.Path()) {
		return nil, nil
	}

	escapes := collectEscapes(pass)
	found := collectFloats(pass)

	for _, k := range sortedLines(found) {
		if e, ok := escapes[k]; ok && e.reason != "" {
			continue
		}
		f := found[k]
		// RelativeTo keeps a local named type reported as "Basis" rather than
		// as its full import path, which is most of the line in this module.
		pass.Reportf(f.pos,
			"%s value under internal/: money is fixedpoint (§3.6 rule 4); annotate a genuine exception with %s <reason>",
			types.TypeString(f.typ, types.RelativeTo(pass.Pkg)), directive)
	}

	for _, k := range sortedEscapes(escapes) {
		e := escapes[k]
		if _, used := found[k]; !used {
			pass.Reportf(e.pos,
				"unused %s: no float on this line, so this annotation would silently authorise the next one to land here",
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

// collectEscapes indexes the annotations by line before anything is diagnosed,
// so that matching a float to its annotation does not depend on walk order.
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

// collectFloats reduces every float-typed expression to at most one finding per
// line, kept at the earliest position on that line so the diagnostic lands on
// the start of the offending construct rather than somewhere inside it.
func collectFloats(pass *analysis.Pass) map[srcLine]finding {
	found := make(map[srcLine]finding)
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			expr, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			typ, ok := floatType(pass.TypesInfo.TypeOf(expr))
			if !ok {
				return true
			}
			at := pass.Fset.Position(expr.Pos())
			k := srcLine{at.Filename, at.Line}
			if prev, seen := found[k]; !seen || expr.Pos() < prev.pos {
				found[k] = finding{expr.Pos(), typ}
			}
			return true
		})
	}
	return found
}

// floatType reports whether t is or contains a floating-point type, returning
// the offending type so the diagnostic can name it.
//
// Underlying() rather than the type itself, so that `type Ratio float64` is
// policed at every use of Ratio and not only at its declaration — otherwise one
// named type is all it takes to leave the rule behind.
//
// Tuples are unwrapped because a multi-value call's type is a tuple, not a
// basic: `strconv.ParseFloat` returns (float64, error), and without this the
// most likely single door into the money path would be the one case the rule
// missed.
//
// complex64/complex128 count. They are pairs of floats, they carry exactly the
// representation error the rule exists to exclude, and excluding them costs one
// term here rather than a second analyzer later.
func floatType(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	if tuple, ok := t.(*types.Tuple); ok {
		for i := range tuple.Len() {
			if inner, ok := floatType(tuple.At(i).Type()); ok {
				return inner, true
			}
		}
		return nil, false
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return nil, false
	}
	if basic.Info()&(types.IsFloat|types.IsComplex) == 0 {
		return nil, false
	}
	return t, true
}

// escapeReason reports whether the comment is the escape directive and, if so,
// the reason following it. The separator check is what stops //nofloat:allowance
// from reading as an escape with the reason "ance".
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

func underInternal(pkgPath string) bool {
	return strings.Contains(pkgPath, internalSegment) ||
		strings.HasSuffix(pkgPath, strings.TrimSuffix(internalSegment, "/"))
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
