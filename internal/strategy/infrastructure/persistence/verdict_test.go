package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func illustrativeHurdle(t *testing.T) persistence.Hurdle {
	t.Helper()
	return persistence.Hurdle{
		// 4.6% as a ratio. summarise.py prints "4.6"; rate.go stores ratios.
		Annual: mustRate(t, "0.046"),
		Class:  "illustrative_constant",
		Source: "research/persistence/summarise.py HURDLE_PCT. NOT reference data: " +
			"configs/reference.*.json carry annual_rate 0.0369 at class hand_recorded.",
	}
}

func TestVerdict_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		cost                  string
		medianPct             string // the ratio's Decimal() form
		positive, beat, total int
	}{
		{"0.0021", "-0.001287547", 16, 4, 56},
		{"0.0029", "-0.002324779", 12, 3, 56},
		{"0.0090", "-0.014832616", 1, 0, 56},
	}
	for _, c := range cases {
		t.Run(c.cost, func(t *testing.T) {
			got, err := set.Verdict(sizeAt(t, "0.001", c.cost), illustrativeHurdle(t))
			if err != nil {
				t.Fatalf("Verdict: %v", err)
			}
			if got.PositiveSymbols != c.positive {
				t.Errorf("PositiveSymbols = %d, want %d", got.PositiveSymbols, c.positive)
			}
			if got.BeatHurdle != c.beat {
				t.Errorf("BeatHurdle = %d, want %d", got.BeatHurdle, c.beat)
			}
			if got.TotalSymbols != c.total {
				t.Errorf("TotalSymbols = %d, want %d", got.TotalSymbols, c.total)
			}
			// F60/persymbol_test.go's own denominator note: the level's
			// exclusions are a fact about the level, and Verdict carries them
			// rather than only their count. Measured, not guessed at.
			wantExcluded := []persistence.Exclusion{
				{Symbol: "CAPUSDT"}, {Symbol: "CFXUSDT"}, {Symbol: "WLDUSDT"},
				{Symbol: "XAUTUSDT"}, {Symbol: "XPLUSDT"},
			}
			if len(got.Excluded) != len(wantExcluded) {
				t.Fatalf("Excluded = %+v, want %+v", got.Excluded, wantExcluded)
			}
			for i, want := range wantExcluded {
				if got.Excluded[i] != want {
					t.Errorf("Excluded[%d] = %+v, want %+v", i, got.Excluded[i], want)
				}
			}
			if got.MedianAnnual.Decimal() != c.medianPct {
				t.Errorf("MedianAnnual = %s, want %s", got.MedianAnnual.Decimal(), c.medianPct)
			}
			// The hurdle rides on the verdict, with its class and its source.
			// F45: a stored comparison carries its comparand, and the count
			// beside a hurdle whose provenance is missing is what cost this
			// project a screen rendering 3.69 beside a count computed at 4.6.
			if got.Hurdle.Class != "illustrative_constant" {
				t.Errorf("Hurdle.Class = %q, want illustrative_constant", got.Hurdle.Class)
			}
			if got.Hurdle.Annual.Decimal() != "0.046" {
				t.Errorf("Hurdle.Annual = %s, want 0.046", got.Hurdle.Annual.Decimal())
			}
		})
	}
}

// An unset hurdle is refused. A Go constant here would re-commit F45 in a
// second language: summarise.py's HURDLE_PCT is illustrative, is not reference
// data, and is not a calibrated benchmark per build-spec §55.
func TestVerdict_RefusesAnUnsetHurdle(t *testing.T) {
	set := loadCommitted(t)
	_, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), persistence.Hurdle{})
	if err == nil {
		t.Fatal("Verdict with a zero Hurdle returned nil error")
	}
}

// Mirrors runlength_test.go's TestRunLength_AllCensoredIsNotAnEmptyLevel and
// net_test.go's TestNetPerEpisode_AllCensoredIsNotAnEmptyLevel: a level whose
// only episodes are censored (run == 0) folds to an empty per-symbol
// population, and Verdict's early-return branch (len(table.Rows) == 0) is the
// third fold with exactly this shape. MedianAnnual must be a CONSTRUCTED
// zero — Validate() passes — never the bare zero value, which Validate()
// would refuse. Asserting only that it equals zero would not separate the
// two: the bare zero value also equals zero.
func TestVerdict_AllCensoredIsNotAnEmptyLevel(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), illustrativeHurdle(t))
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if got.PositiveSymbols != 0 || got.BeatHurdle != 0 || got.TotalSymbols != 0 {
		t.Fatalf("PositiveSymbols/BeatHurdle/TotalSymbols = %d/%d/%d, want 0/0/0",
			got.PositiveSymbols, got.BeatHurdle, got.TotalSymbols)
	}
	// The exclusion carries WHY: three censored records, not "never entered".
	// Every committed exclusion carries Censored 0, so this is the only place
	// the non-zero reading is exercised.
	wantExcluded := persistence.Exclusion{Symbol: "FIXTURE", Censored: 3}
	if len(got.Excluded) != 1 || got.Excluded[0] != wantExcluded {
		t.Fatalf("Excluded = %+v, want [%+v]", got.Excluded, wantExcluded)
	}
	if got.MedianAnnual.Validate() != nil {
		t.Error("MedianAnnual over an empty population must be a constructed zero, not the zero value")
	}
}

// The hurdle is an ARGUMENT, not a constant Verdict could bake in. Every other
// test in this file passes 0.046, which would not catch Verdict ignoring h and
// comparing against a hidden literal. A lower hurdle must change BeatHurdle.
func TestVerdict_HurdleIsAnArgument(t *testing.T) {
	set := loadCommitted(t)
	low := illustrativeHurdle(t)
	low.Annual = mustRate(t, "0.02")

	got, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), low)
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if got.BeatHurdle != 5 {
		t.Errorf("BeatHurdle = %d, want 5 at hurdle 0.02 (4 at hurdle 0.046 — must differ)", got.BeatHurdle)
	}
	if got.Hurdle.Annual.Decimal() != "0.02" {
		t.Errorf("Hurdle.Annual = %s, want 0.02", got.Hurdle.Annual.Decimal())
	}
}

// THE SEVENTEENTH COLLAPSE. On the committed file, comparing the hurdle against
// the stored RATIO and against the DISPLAYED percentage give identical counts
// at every cost (4 / 3 / 0). So no assertion over that file can tell which rule
// the code implements — and this project has already shipped a display-rounding
// predicate once.
//
// The fixture carries two symbols:
//   - FIXTURE, whose annual return is 4.63% — strictly above the 4.6% hurdle
//     as a ratio, and ALSO 4.6% when rounded to one decimal, where a
//     strictly-greater comparison on the rounded value would exclude it.
//   - EXACT, whose annual return is 4.60% — exactly the hurdle. This is what
//     separates ">" from ">=": Task 5's committed-file collapse was this same
//     defect class one comparison over.
//
// Under the ratio rule (the one this package implements), BeatHurdle is 1:
// FIXTURE beats (0.0463 > 0.046), EXACT does not (0.046 is not > 0.046). Under
// the display rule, BeatHurdle is 0: both round to 4.6%, and 4.6 is not > 4.6.
func TestVerdict_ComparesTheRatioNotTheRoundedPercentage(t *testing.T) {
	set := loadFixture(t, "testdata/hurdle-boundary.json")
	got, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), illustrativeHurdle(t))
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if got.TotalSymbols != 2 {
		t.Fatalf("TotalSymbols = %d, want 2", got.TotalSymbols)
	}
	if got.BeatHurdle != 1 {
		t.Fatalf("BeatHurdle = %d, want 1 — FIXTURE (0.0463) beats strictly, "+
			"EXACT (0.046) does not, and neither reading collapses to 0 or 2", got.BeatHurdle)
	}
	// Both symbols here are positive; every committed-file median asserted
	// above is negative. Without this, an inverted sign somewhere in the
	// fold could pass unnoticed.
	if got.MedianAnnual.Decimal() != "0.0463" {
		t.Errorf("MedianAnnual = %s, want 0.0463", got.MedianAnnual.Decimal())
	}
}

// There is no test for Verdict.Excluded being a COPY of PerSymbol's slice, and
// that is a recorded result rather than an omission: Verdict calls PerSymbol
// itself, so the aliased table never leaves the function and no caller can hold
// both ends to observe the mutation. Deleting the copy was measured as an
// EQUIVALENT mutant (SURVIVED, diagnosed). verdict.go says the same beside the
// field, so the copy is not credited with a protection nothing checks.
