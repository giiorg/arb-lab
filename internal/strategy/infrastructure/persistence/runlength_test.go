package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func TestRunLength_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		level              string
		p50, p75, p90, max int
		episodes, censored int
		mean               string // the Rate's Decimal() form, 9-place canonical
	}{
		// The brief's original constants for the 0.0002, 0.0005 and 0.002 rows
		// disagreed with the committed file: two independent folds (this
		// package, and a standalone Python fold over episodes.json using
		// decimal.Decimal for exact arithmetic) agree on the values below, and
		// all four agree with record §103's 3-decimal-place table (2.434,
		// 2.630, 2.572, 2.176) once rounded. 29975/12317, 9007/3425,
		// 3238/1259 and 1027/472 are the exact fractions; RoundHalfEven at
		// scale 9 gives the values here.
		{"0.0002", 1, 3, 5, 71, 12317, 5, "2.433628319"},
		{"0.0005", 2, 3, 6, 36, 3425, 0, "2.629781022"},
		{"0.001", 2, 3, 5, 20, 1259, 0, "2.571882446"},
		{"0.002", 1, 3, 4, 20, 472, 0, "2.175847458"},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, c.level)})
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}
			if got.P50 != c.p50 || got.P75 != c.p75 || got.P90 != c.p90 || got.Max != c.max {
				t.Errorf("percentiles = %d/%d/%d max %d, want %d/%d/%d max %d",
					got.P50, got.P75, got.P90, got.Max, c.p50, c.p75, c.p90, c.max)
			}
			if got.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", got.Episodes, c.episodes)
			}
			if got.Censored != c.censored {
				t.Errorf("Censored = %d, want %d", got.Censored, c.censored)
			}
			if got.Mean.Decimal() != c.mean {
				t.Errorf("Mean = %s, want %s", got.Mean.Decimal(), c.mean)
			}
			// The coordinate rides on the value. This is the whole point of the
			// package: a screen renders what it holds, and HANDOFF.md restating
			// "median episode is 2 settlements" with no level is what a correct
			// call looks like three hops downstream — at two of the four levels
			// the median is 1.
			if got.At.Entry.Decimal() != c.level {
				t.Errorf("At.Entry = %s, want %s", got.At.Entry.Decimal(), c.level)
			}
		})
	}
}

// The percentile is nearest-rank on the SORTED population, replicating
// summarise.py:38 exactly: v[min(len-1, (len*q)//100)]. Both off-by-one
// directions must be visible, so the fixture is a population whose q-th element
// differs from its neighbours at every quantile the reader reports.
func TestRunLength_PercentileIsNearestRank(t *testing.T) {
	// runs 1..10, one episode each. Nearest-rank indices:
	//   p50 -> min(9, 100//100=... ) for len=10: (10*50)//100 = 5 -> value 6
	//   p75 -> (10*75)//100 = 7 -> value 8
	//   p90 -> (10*90)//100 = 9 -> value 10
	// Every one of the three differs from its neighbour by exactly 1, so an
	// index shifted either way changes the answer.
	set := loadFixture(t, "testdata/percentile.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.P50 != 6 || got.P75 != 8 || got.P90 != 10 {
		t.Fatalf("percentiles = %d/%d/%d, want 6/8/10", got.P50, got.P75, got.P90)
	}
	if got.Max != 10 {
		t.Errorf("Max = %d, want 10", got.Max)
	}
}

// Nearest-rank is not a midpoint rule, and a two-element population is where
// the difference is loudest: for runs 3 and 9 the 50th percentile is 9, the
// SECOND element, because (2*50)//100 = 1. Any implementation that averaged the
// two middle values, or that used (len-1)*q/100, would answer 3 or 6 here.
//
// It also pins the top of the index range: (2*90)//100 = 1 = len-1, the last
// legal index. The min(len-1, ...) clamp itself fires only for q >= 100, which
// this package never asks — so this is the boundary the code must not cross,
// not a test of the clamp.
func TestRunLength_PercentileIsNotAMidpoint(t *testing.T) {
	set := loadFixture(t, "testdata/two-episodes.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.P50 != 9 {
		t.Errorf("P50 = %d, want 9 — nearest rank on [3 9] at q=50 is index 1", got.P50)
	}
	if got.P90 != 9 {
		t.Errorf("P90 = %d, want 9 (index len-1)", got.P90)
	}
	if got.Max != 9 {
		t.Errorf("Max = %d, want 9", got.Max)
	}
}

// A level with censored records but no runs at all still answers, and answers
// honestly: zero episodes, the censored count reported, and no percentile
// invented from an empty population.
func TestRunLength_AllCensoredIsNotAnEmptyLevel(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.Episodes != 0 || got.Censored != 3 {
		t.Fatalf("Episodes/Censored = %d/%d, want 0/3", got.Episodes, got.Censored)
	}
	if got.P50 != 0 || got.P75 != 0 || got.P90 != 0 || got.Max != 0 {
		t.Errorf("percentiles over an empty population = %d/%d/%d/%d, want all 0",
			got.P50, got.P75, got.P90, got.Max)
	}
	if got.Mean.Validate() != nil {
		t.Error("Mean over an empty population must be a constructed zero, not the zero value")
	}
}
