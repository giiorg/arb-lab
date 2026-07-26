package nofloat

import (
	"go/token"
	"testing"
)

// sortedLines and sortedEscapes exist for one reason: diagnostics must come out
// in source order rather than in Go's randomized map order, so that two runs of
// the gate over an unchanged tree produce byte-identical output and a CI diff
// means something changed.
//
// Nothing tested that until the mutation audit, and the reason is worth keeping:
// analysistest matches diagnostics against `// want` comments attached to source
// lines, so it is order-insensitive by construction. Reversing either comparator
// left the whole analyzer suite green. That is the shape §5e already recorded —
// tested logic plus untested wiring is still an untested guarantee — except here
// the untested part is not wiring but the property the harness cannot see.
//
// So the comparators are tested directly, below the analyzer.
//
// Scope, stated rather than implied: this kills a REVERSED comparator. It does
// not kill `<` widened to `<=`, which is equivalent while every position is
// distinct — and positions are distinct, because a token.Pos identifies one
// place in one file. That survivor is accepted, not overlooked.
func TestSortedLines_isAscendingByPosition(t *testing.T) {
	// The keys are arranged so that position order disagrees with every other
	// order available: line number runs exactly BACKWARDS against it, and file
	// name is scrambled against both. A comparator reaching for the wrong field
	// therefore fails here rather than passing on a coincidence — which is not
	// a hypothetical, because the first version of this table had num and pos
	// agreeing and a comparator switched to keys[i].num sailed through it.
	m := map[srcLine]finding{
		{file: "z.go", num: 10}: {pos: token.Pos(500)},
		{file: "a.go", num: 90}: {pos: token.Pos(100)},
		{file: "m.go", num: 50}: {pos: token.Pos(300)},
		{file: "b.go", num: 30}: {pos: token.Pos(400)},
		{file: "y.go", num: 70}: {pos: token.Pos(200)},
	}

	// Repeated because one run proves little: map iteration is randomized, so a
	// no-op sort would land in ascending order by chance about once in 120
	// attempts. Fifty runs turn "it happened to be right" into a result.
	for range 50 {
		got := sortedLines(m)
		if len(got) != len(m) {
			t.Fatalf("sortedLines returned %d keys; want %d — a key was dropped", len(got), len(m))
		}
		for i := 1; i < len(got); i++ {
			if m[got[i-1]].pos >= m[got[i]].pos {
				t.Fatalf("sortedLines is not ascending by position: %v (pos %d) came before %v (pos %d)",
					got[i-1], m[got[i-1]].pos, got[i], m[got[i]].pos)
			}
		}
	}
}

func TestSortedEscapes_isAscendingByPosition(t *testing.T) {
	m := map[srcLine]escape{
		{file: "z.go", num: 10}: {pos: token.Pos(500), reason: "e"},
		{file: "a.go", num: 90}: {pos: token.Pos(100), reason: "a"},
		{file: "m.go", num: 50}: {pos: token.Pos(300), reason: "c"},
		{file: "b.go", num: 30}: {pos: token.Pos(400), reason: "d"},
		{file: "y.go", num: 70}: {pos: token.Pos(200), reason: "b"},
	}

	for range 50 {
		got := sortedEscapes(m)
		if len(got) != len(m) {
			t.Fatalf("sortedEscapes returned %d keys; want %d — a key was dropped", len(got), len(m))
		}
		for i := 1; i < len(got); i++ {
			if m[got[i-1]].pos >= m[got[i]].pos {
				t.Fatalf("sortedEscapes is not ascending by position: %v (pos %d) came before %v (pos %d)",
					got[i-1], m[got[i-1]].pos, got[i], m[got[i]].pos)
			}
		}
	}
}
