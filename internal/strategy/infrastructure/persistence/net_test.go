package persistence_test

import (
	"testing"
)

func TestNetPerEpisode_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		entry, cost        string
		episodes, wins     int
		meanGross, meanNet string
	}{
		{"0.0002", "0.0021", 12317, 872, "0.000653608", "-0.001446392"},
		{"0.0005", "0.0021", 3425, 586, "0.001518849", "-0.000581151"},
		{"0.001", "0.0021", 1259, 343, "0.002613091", "0.000513091"},
		{"0.002", "0.0021", 472, 168, "0.00367787", "0.00157787"},
	}
	for _, c := range cases {
		t.Run(c.entry, func(t *testing.T) {
			got, err := set.NetPerEpisode(costAt(t, c.entry, c.cost))
			if err != nil {
				t.Fatalf("NetPerEpisode: %v", err)
			}
			if got.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", got.Episodes, c.episodes)
			}
			if got.Wins != c.wins {
				t.Errorf("Wins = %d, want %d", got.Wins, c.wins)
			}
			if got.MeanGross.Decimal() != c.meanGross {
				t.Errorf("MeanGross = %s, want %s", got.MeanGross.Decimal(), c.meanGross)
			}
			if got.MeanNet.Decimal() != c.meanNet {
				t.Errorf("MeanNet = %s, want %s", got.MeanNet.Decimal(), c.meanNet)
			}
		})
	}
}

// THE SIXTEENTH COLLAPSE, from record §103 §6.2.
//
// Wins counts accrued > cost. Measured over all 17 478 committed episodes,
// EXACTLY ZERO accrue any of the three costs, so `>` and `>=` return the same
// three counts (1 969 / 1 485 / 430 pooled across levels). The committed data
// cannot separate the two comparisons, and the oracle test in Task 8 passes
// against both. Only a hand-built episode accruing EXACTLY the cost can tell
// them apart, and this is it.
func TestNetPerEpisode_TheBoundaryIsStrict(t *testing.T) {
	// Three episodes: one just under the cost, one exactly at it, one just over.
	// Strictly-greater gives Wins = 1. Greater-or-equal would give 2.
	set := loadFixture(t, "testdata/cost-boundary.json")
	got, err := set.NetPerEpisode(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("NetPerEpisode: %v", err)
	}
	if got.Episodes != 3 {
		t.Fatalf("Episodes = %d, want 3", got.Episodes)
	}
	if got.Wins != 1 {
		t.Fatalf("Wins = %d, want 1 — an episode accruing exactly the cost does NOT clear it", got.Wins)
	}
}

// The same fixture separates the comparison in the break-even table, which uses
// its own copy of the rule. A fix applied in one place gets applied to its
// mirror, and a claim checked in one place gets checked in its mirror.
func TestBreakeven_TheBoundaryIsStrict(t *testing.T) {
	set := loadFixture(t, "testdata/cost-boundary.json")
	got, err := set.Breakeven(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	total := 0
	for _, r := range got.Rows {
		total += r.ClearedCost
	}
	if total != 1 {
		t.Fatalf("cleared total = %d, want 1", total)
	}
}

// The table is a function of BOTH coordinates. A test at one cost only cannot
// tell whether Wins and MeanNet use the Cost argument at all — Episodes and
// MeanGross must NOT move (the population and its gross accrual are entry-only),
// while Wins and MeanNet must. Mirrors breakeven_test.go's
// TestBreakeven_ClearedCountMovesWithTheCost.
func TestNetPerEpisode_WinsAndMeanNetMoveWithTheCost(t *testing.T) {
	set := loadCommitted(t)
	cheap, err := set.NetPerEpisode(costAt(t, "0.0005", "0.0021"))
	if err != nil {
		t.Fatalf("NetPerEpisode cheap: %v", err)
	}
	dear, err := set.NetPerEpisode(costAt(t, "0.0005", "0.0090"))
	if err != nil {
		t.Fatalf("NetPerEpisode dear: %v", err)
	}
	if cheap.Episodes != 3425 || dear.Episodes != 3425 {
		t.Errorf("Episodes = %d/%d, want 3425/3425 (population is entry-only)", cheap.Episodes, dear.Episodes)
	}
	if cheap.MeanGross.Decimal() != dear.MeanGross.Decimal() {
		t.Errorf("MeanGross = %s/%s, want equal (gross accrual is entry-only)",
			cheap.MeanGross.Decimal(), dear.MeanGross.Decimal())
	}
	if cheap.Wins != 586 || dear.Wins != 125 {
		t.Fatalf("Wins = %d/%d, want 586/125", cheap.Wins, dear.Wins)
	}
	if cheap.MeanNet.Decimal() != "-0.000581151" || dear.MeanNet.Decimal() != "-0.007481151" {
		t.Fatalf("MeanNet = %s/%s, want -0.000581151/-0.007481151",
			cheap.MeanNet.Decimal(), dear.MeanNet.Decimal())
	}
}

// Mirrors runlength_test.go's TestRunLength_AllCensoredIsNotAnEmptyLevel: a
// level whose only episodes are censored (run == 0) folds to an empty run
// population. MeanGross and MeanNet must be a CONSTRUCTED zero — Validate()
// passes — never the bare zero value, which Validate() would refuse. Without
// this, "no episodes" and "nobody computed this" render identically.
func TestNetPerEpisode_AllCensoredIsNotAnEmptyLevel(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.NetPerEpisode(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("NetPerEpisode: %v", err)
	}
	if got.Episodes != 0 || got.Wins != 0 {
		t.Fatalf("Episodes/Wins = %d/%d, want 0/0", got.Episodes, got.Wins)
	}
	if got.MeanGross.Validate() != nil {
		t.Error("MeanGross over an empty population must be a constructed zero, not the zero value")
	}
	if got.MeanNet.Validate() != nil {
		t.Error("MeanNet over an empty population must be a constructed zero, not the zero value")
	}
}
