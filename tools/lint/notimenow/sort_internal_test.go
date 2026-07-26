package notimenow

import (
	"go/token"
	"testing"
)

// The same two comparators as nofloat's, tested the same way and for the same
// reason — see nofloat/sort_internal_test.go for the full argument. In short:
// diagnostics must come out in source order so an unchanged tree produces
// byte-identical output, and analysistest cannot see order, so reversing either
// comparator left the analyzer suite green.
//
// The duplication is deliberate and matches the standing decision about these
// two analyzers: the common parts get extracted at the THIRD gate, not the
// second, because until a third exists it is not yet clear which parts are
// common and which only look it.
func TestSortedLines_isAscendingByPosition(t *testing.T) {
	// Line number runs exactly backwards against position and file name is
	// scrambled against both, so a comparator reaching for the wrong field
	// fails here instead of passing on a coincidence.
	m := map[srcLine]finding{
		{file: "z.go", num: 10}: {pos: token.Pos(500), fn: "time.Now", verb: "clock"},
		{file: "a.go", num: 90}: {pos: token.Pos(100), fn: "time.Since", verb: "clock"},
		{file: "m.go", num: 50}: {pos: token.Pos(300), fn: "time.Now", verb: "clock"},
		{file: "b.go", num: 30}: {pos: token.Pos(400), fn: "time.Tick", verb: "clock"},
		{file: "y.go", num: 70}: {pos: token.Pos(200), fn: "time.After", verb: "clock"},
	}

	// Repeated: map iteration is randomized, so an absent sort would produce
	// ascending order by chance roughly once in 120 runs.
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
