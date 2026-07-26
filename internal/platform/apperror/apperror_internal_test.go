package apperror

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The tests in this file reach inside the package for two reasons that the
// external suite cannot cover.
//
// The first is resolveEffect's full input space. It is the rule that makes
// EffectOf's "outermost wins" safe, and the external tests exercise it only
// through the pairings that constructors can produce — which is most of them,
// but not the ones involving the unspecified sentinel on either side.
//
// The second is a class of bug no behavioural test can see: a constant declared
// and then left out of the list that Validate consults. Nothing fails, because
// the constant is never used in the test that would have caught it. So these
// tests parse this package's own source and compare what is DECLARED against
// what is REACHABLE, which is the only way to make "you added a constant and
// forgot the table" a test failure rather than a code-review hope.

func TestResolveEffect(t *testing.T) {
	all := []Effect{EffectUnspecified, EffectNotApplied, EffectUnknown, EffectApplied}

	// The rule in one sentence: agreement is preserved, disagreement collapses
	// to EffectUnknown, and an unspecified side defers to the other. The table
	// is written out in full rather than generated from that sentence, because
	// generating the expectation from the rule would test the rule against
	// itself.
	want := map[[2]Effect]Effect{
		// claimed unspecified: the wrapper said nothing, so it overrides nothing.
		{EffectUnspecified, EffectUnspecified}: EffectUnspecified,
		{EffectUnspecified, EffectNotApplied}:  EffectNotApplied,
		{EffectUnspecified, EffectUnknown}:     EffectUnknown,
		{EffectUnspecified, EffectApplied}:     EffectApplied,

		// recorded unspecified: nothing beneath, so the claim stands alone. It
		// is NOT promoted to Unknown here — New does that once, so a caller
		// reading a degraded error gets one report of the defect, not two.
		{EffectNotApplied, EffectUnspecified}: EffectNotApplied,
		{EffectUnknown, EffectUnspecified}:    EffectUnknown,
		{EffectApplied, EffectUnspecified}:    EffectApplied,

		// agreement.
		{EffectNotApplied, EffectNotApplied}: EffectNotApplied,
		{EffectUnknown, EffectUnknown}:       EffectUnknown,
		{EffectApplied, EffectApplied}:       EffectApplied,

		// disagreement, in every direction. The two that matter most are
		// {NotApplied, Unknown} — an outer layer trying to make an ambiguous
		// outcome certain — and {Applied, NotApplied}, where two layers flatly
		// contradict each other and doubt is the only honest answer.
		{EffectNotApplied, EffectUnknown}: EffectUnknown,
		{EffectNotApplied, EffectApplied}: EffectUnknown,
		{EffectUnknown, EffectNotApplied}: EffectUnknown,
		{EffectUnknown, EffectApplied}:    EffectUnknown,
		{EffectApplied, EffectNotApplied}: EffectUnknown,
		{EffectApplied, EffectUnknown}:    EffectUnknown,
	}

	if len(want) != len(all)*len(all) {
		t.Fatalf("the table covers %d of %d pairings", len(want), len(all)*len(all))
	}

	for _, claimed := range all {
		for _, recorded := range all {
			got := resolveEffect(claimed, recorded)
			if exp := want[[2]Effect{claimed, recorded}]; got != exp {
				t.Errorf("resolveEffect(claimed=%v, recorded=%v) = %v, want %v", claimed, recorded, got, exp)
			}
		}
	}
}

// TestResolveEffectNeverInventsCertainty restates the safety property directly
// rather than through the table, so that a wrong entry IN the table cannot make
// both tests agree on the same mistake.
func TestResolveEffectNeverInventsCertainty(t *testing.T) {
	for _, claimed := range []Effect{EffectUnspecified, EffectNotApplied, EffectUnknown, EffectApplied} {
		got := resolveEffect(claimed, EffectUnknown)
		if got != EffectUnknown {
			t.Errorf("resolveEffect(%v, EffectUnknown) = %v; an ambiguous cause must stay ambiguous", claimed, got)
		}
	}
}

func TestEffect_mayHaveApplied(t *testing.T) {
	// Only NotApplied — the single value that is a claim about evidence —
	// answers false. Stated here as an exhaustive sweep over the whole uint8
	// range rather than over the declared values, because the predicate's
	// safety rests on it being written as a negation.
	for i := 0; i <= 255; i++ {
		e := Effect(i)
		want := e != EffectNotApplied
		if got := e.mayHaveApplied(); got != want {
			t.Fatalf("Effect(%d).mayHaveApplied() = %v, want %v", i, got, want)
		}
	}
}

// TestClassDeclaredSetIsComplete parses this package's own source and asserts
// that every declared Class constant appears in the `classes` slice Validate
// consults.
//
// The failure it exists for is silent: declare ClassThrottled, forget the slice,
// and Validate rejects the constant the package itself declares — so every error
// built with it degrades to ClassInternal and the new class simply never appears
// on any dashboard. No test fails, because the tests were written against the
// classes that were already there.
func TestClassDeclaredSetIsComplete(t *testing.T) {
	declared := declaredConstants(t, "class.go", "Class")
	if len(declared) == 0 {
		t.Fatal("found no Class constants in class.go; the parser is looking in the wrong place")
	}

	inSlice := make(map[string]bool, len(classes))
	for _, c := range classes {
		inSlice[string(c)] = true
	}

	for name, value := range declared {
		if !inSlice[value] {
			t.Errorf("constant %s = %q is declared but missing from the classes slice, so Validate rejects it", name, value)
		}
		// And the round trip: a declared constant must validate and print bare.
		c := Class(value)
		if err := c.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v", name, err)
		}
	}

	if len(declared) != len(classes) {
		t.Errorf("%d constants declared, %d in the classes slice — the slice has an entry no constant declares", len(declared), len(classes))
	}
}

// TestEffectDeclaredSetIsComplete does the same for Effect, where the mechanism
// is maxEffect rather than a slice: Validate accepts everything from 1 to
// maxEffect, so a value declared past it is rejected and a maxEffect raised past
// the last constant silently admits a gap.
func TestEffectDeclaredSetIsComplete(t *testing.T) {
	names := declaredEffectNames(t)
	if len(names) == 0 {
		t.Fatal("found no Effect constants in effect.go; the parser is looking in the wrong place")
	}

	// The constants are an iota block, so the count IS the highest value: n
	// constants starting at EffectUnspecified=0 means the last is n-1.
	wantMax := Effect(len(names) - 1)
	if maxEffect != wantMax {
		t.Errorf("maxEffect = %d but effect.go declares %d constants (last value %d) — Validate %s",
			maxEffect, len(names), wantMax,
			map[bool]string{true: "rejects a declared value", false: "admits an undeclared one"}[maxEffect < wantMax])
	}

	// Every declared value must also print as itself rather than falling
	// through String's default, which is how a forgotten switch arm shows up.
	for i, name := range names {
		if got := Effect(i).String(); got != name {
			t.Errorf("Effect(%d).String() = %q, want %q — the String switch is missing an arm", i, got, name)
		}
	}
}

// declaredConstants returns name→value for every constant of the named string
// type declared in the given file of this package.
func declaredConstants(t *testing.T, filename, typeName string) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", filename, err)
	}

	found := make(map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != typeName {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				found[name.Name] = strings.Trim(lit.Value, `"`)
			}
		}
	}
	return found
}

// declaredEffectNames returns the Effect constants in declaration order. The
// iota block gives them no literal values to read, so order is the value.
func declaredEffectNames(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "effect.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing effect.go: %v", err)
	}

	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		// Only the iota block declares Effect values; maxEffect is a separate
		// const with no type ident on its ValueSpec, and skipping it is what
		// keeps the count equal to the highest value.
		typed := false
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if ident, ok := vs.Type.(*ast.Ident); ok && ident.Name == "Effect" {
				typed = true
			}
			if !typed {
				continue
			}
			for _, name := range vs.Names {
				names = append(names, name.Name)
			}
		}
	}
	return names
}
