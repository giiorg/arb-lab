package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func costAt(t *testing.T, entry, cost string) persistence.CostAt {
	t.Helper()
	return persistence.CostAt{
		EntryAt: persistence.EntryAt{Entry: mustRate(t, entry)},
		Cost:    mustRate(t, cost),
	}
}

// The committed file's run_vs_breakeven block is computed at entry 0.0005 and
// cost 0.0021, and records NEITHER — that pin is F55. Here it is a coordinate
// the caller names and the answer carries.
func TestBreakeven_AtTheArtifactsPin(t *testing.T) {
	set := loadCommitted(t)
	got, err := set.Breakeven(costAt(t, "0.0005", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	if len(got.Rows) == 0 {
		t.Fatal("Breakeven returned no rows")
	}
	want := []persistence.BreakevenRow{
		{Run: 1, Episodes: 1648, ClearedCost: 0},
		{Run: 2, Episodes: 746, ClearedCost: 19},
		{Run: 3, Episodes: 341, ClearedCost: 51},
	}
	for i, w := range want {
		if got.Rows[i] != w {
			t.Errorf("Rows[%d] = %+v, want %+v", i, got.Rows[i], w)
		}
	}
	last := got.Rows[len(got.Rows)-1]
	if !last.Plus || last.Run != 12 {
		t.Errorf("last row = %+v, want the 12+ bucket", last)
	}
	for _, r := range got.Rows[:len(got.Rows)-1] {
		if r.Plus {
			t.Errorf("row %+v is marked Plus but is not the last bucket", r)
		}
	}
	if got.At.Entry.Decimal() != "0.0005" || got.At.Cost.Decimal() != "0.0021" {
		t.Errorf("At = %s/%s, want 0.0005/0.0021", got.At.Entry.Decimal(), got.At.Cost.Decimal())
	}
}

// The table is a function of BOTH coordinates. A table asserted at one cost
// cannot tell whether the cost argument is used at all — the cleared_cost
// column is the one that moves.
func TestBreakeven_ClearedCountMovesWithTheCost(t *testing.T) {
	set := loadCommitted(t)
	cheap, err := set.Breakeven(costAt(t, "0.0005", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven cheap: %v", err)
	}
	dear, err := set.Breakeven(costAt(t, "0.0005", "0.0090"))
	if err != nil {
		t.Fatalf("Breakeven dear: %v", err)
	}
	cheapTotal, dearTotal := 0, 0
	for i := range cheap.Rows {
		cheapTotal += cheap.Rows[i].ClearedCost
		dearTotal += dear.Rows[i].ClearedCost
		if cheap.Rows[i].Episodes != dear.Rows[i].Episodes {
			t.Errorf("row %d: episode count moved with the cost (%d vs %d); it must not",
				i, cheap.Rows[i].Episodes, dear.Rows[i].Episodes)
		}
	}
	if cheapTotal != 586 || dearTotal != 125 {
		t.Fatalf("cleared totals = %d/%d, want 586/125", cheapTotal, dearTotal)
	}
}

// Bucketing caps at 12, so runs of 12, 20 and 36 all land in one row. The
// fixture must contain a run ABOVE 12 and a run AT 12, or "cap at 12" and "cap
// at 13" are the same table.
func TestBreakeven_BucketsCapAtTwelve(t *testing.T) {
	set := loadFixture(t, "testdata/buckets.json")
	got, err := set.Breakeven(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	last := got.Rows[len(got.Rows)-1]
	if last.Run != 12 || !last.Plus || last.Episodes != 3 {
		t.Fatalf("12+ bucket = %+v, want run 12, Plus, 3 episodes", last)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 (run 11 and the 12+ bucket)", len(got.Rows))
	}
}
