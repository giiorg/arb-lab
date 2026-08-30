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

	// Verify the sum of episodes equals the population at this coordinate. The
	// population at entry 0.0005 is 3425 (independently verified by folding
	// research/persistence/episodes.json excluding run==0 censored records).
	totalEpisodes := 0
	for _, row := range got.Rows {
		totalEpisodes += row.Episodes
	}
	const populationAt0_0005 = 3425
	if totalEpisodes != populationAt0_0005 {
		t.Errorf("sum of Episodes = %d, want %d (population)", totalEpisodes, populationAt0_0005)
	}
	// The zero half of the censoring assertion. Level 0.0005 carries no
	// censored records, so the buckets ARE the whole population here and
	// summary.json's own 3 425 needs no reconciliation.
	// TestBreakeven_BucketsSumShortWhereRecordsAreCensored is the five half.
	if got.Censored != 0 {
		t.Errorf("Censored = %d, want 0 at level 0.0005", got.Censored)
	}
}

// F103, in the block that would show it if it could. The buckets start at run
// 1 and a censored record is not a run of length zero, so at level 0.0002 they
// sum FIVE short of the 12 322 summarise.py publishes — and without Censored
// on the table nothing on the value says why. Episodes summed plus Censored is
// the artifact's count.
//
// This is also why the artifact never showed it: summarise.py pins
// run_vs_breakeven to level 0.0005, which is the one level with censored
// records absent. The pinned coordinate F55 names is exactly the one that hides
// the zero bucket.
func TestBreakeven_BucketsSumShortWhereRecordsAreCensored(t *testing.T) {
	set := loadCommitted(t)
	got, err := set.Breakeven(costAt(t, "0.0002", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	if got.Censored != 5 {
		t.Errorf("Censored = %d, want 5", got.Censored)
	}
	total := 0
	for _, r := range got.Rows {
		total += r.Episodes
		if r.Run == 0 {
			t.Errorf("a run-0 bucket exists: %+v — a censored record is not a run of length zero", r)
		}
	}
	if total != 12317 {
		t.Errorf("sum of Episodes = %d, want 12317", total)
	}
	if total+got.Censored != 12322 {
		t.Errorf("episodes %d + censored %d != the artifact's 12322", total, got.Censored)
	}
}

// Mirrors the other three folds' all-censored tests. Breakeven was the one fold
// without one: a level whose only records are censored yields NO buckets, and
// the count of what was dropped is the only thing distinguishing that from a
// level that was never measured.
func TestBreakeven_AllCensoredIsNotAnEmptyLevel(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.Breakeven(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	if len(got.Rows) != 0 {
		t.Fatalf("Rows = %+v, want none", got.Rows)
	}
	if got.Censored != 3 {
		t.Errorf("Censored = %d, want 3 — an empty table with a reason", got.Censored)
	}
	if got.At.Entry.Decimal() != "0.001" || got.At.Cost.Decimal() != "0.0021" {
		t.Errorf("At = %s/%s, want 0.001/0.0021", got.At.Entry.Decimal(), got.At.Cost.Decimal())
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
	cheapEpisodes, dearEpisodes := 0, 0
	for i := range cheap.Rows {
		cheapTotal += cheap.Rows[i].ClearedCost
		dearTotal += dear.Rows[i].ClearedCost
		cheapEpisodes += cheap.Rows[i].Episodes
		dearEpisodes += dear.Rows[i].Episodes
		if cheap.Rows[i].Episodes != dear.Rows[i].Episodes {
			t.Errorf("row %d: episode count moved with the cost (%d vs %d); it must not",
				i, cheap.Rows[i].Episodes, dear.Rows[i].Episodes)
		}
	}
	if cheapTotal != 586 || dearTotal != 125 {
		t.Fatalf("cleared totals = %d/%d, want 586/125", cheapTotal, dearTotal)
	}

	// Verify episode counts sum to the population at this coordinate.
	const populationAt0_0005 = 3425
	if cheapEpisodes != populationAt0_0005 {
		t.Errorf("cheap: sum of Episodes = %d, want %d (population)", cheapEpisodes, populationAt0_0005)
	}
	if dearEpisodes != populationAt0_0005 {
		t.Errorf("dear: sum of Episodes = %d, want %d (population)", dearEpisodes, populationAt0_0005)
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
	if len(got.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 (run 11 and the 12+ bucket)", len(got.Rows))
	}

	// Assert the first bucket fully. The fixture has one run-11 episode with
	// accrued 0.001, which is NOT > 0.0021, so ClearedCost is 0.
	first := got.Rows[0]
	if first.Run != 11 || first.Plus || first.Episodes != 1 || first.ClearedCost != 0 {
		t.Errorf("Rows[0] = %+v, want {Run: 11, Plus: false, Episodes: 1, ClearedCost: 0}", first)
	}

	// Assert the last bucket (the 12+ bucket). Runs 12, 13, 30 collapse here: one
	// with accrued 0.001 (not cleared), two with 0.003 and 0.004 (both > 0.0021,
	// both cleared).
	last := got.Rows[1]
	if last.Run != 12 || !last.Plus || last.Episodes != 3 || last.ClearedCost != 2 {
		t.Errorf("Rows[1] = %+v, want {Run: 12, Plus: true, Episodes: 3, ClearedCost: 2}", last)
	}

	// Verify Plus is set only on the 12+ bucket.
	for i, r := range got.Rows {
		if (i == 1) != r.Plus {
			t.Errorf("Rows[%d]: Plus = %v, want %v", i, r.Plus, i == 1)
		}
	}

	// Verify episode counts sum to the fixture population (4 episodes total).
	totalEpisodes := 0
	for _, row := range got.Rows {
		totalEpisodes += row.Episodes
	}
	const fixturePopulation = 4
	if totalEpisodes != fixturePopulation {
		t.Errorf("sum of Episodes = %d, want %d (fixture population)", totalEpisodes, fixturePopulation)
	}
}
