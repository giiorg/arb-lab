package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

// committedEpisodes is the artifact this package was built to read. The path is
// relative to this package directory, four levels below the repo root.
const committedEpisodes = "../../../../research/persistence/episodes.json"

func loadCommitted(t *testing.T) persistence.Set {
	t.Helper()
	set, err := persistence.Load(committedEpisodes)
	if err != nil {
		t.Fatalf("Load(%q): %v", committedEpisodes, err)
	}
	return set
}

func loadFixture(t *testing.T, path string) persistence.Set {
	t.Helper()
	set, err := persistence.Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	return set
}

func TestLoad_DecodesTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)

	if got, want := set.SymbolCount(), 61; got != want {
		t.Errorf("SymbolCount() = %d, want %d", got, want)
	}
	levels := set.Levels()
	want := []string{"0.0002", "0.0005", "0.001", "0.002"}
	if len(levels) != len(want) {
		t.Fatalf("Levels() has %d entries, want %d", len(levels), len(want))
	}
	for i, lv := range levels {
		if lv.Decimal() != want[i] {
			t.Errorf("Levels()[%d] = %s, want %s", i, lv.Decimal(), want[i])
		}
	}
}

// The four levels carry four DIFFERENT episode counts. This is the separating
// member record §103 §6.2 names: p75 is 3 at every level, so an assertion on
// p75 passes whatever the code does with the level argument.
func TestLoad_EpisodeCountsSeparateTheLevels(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		level    string
		episodes int
		censored int
	}{
		{"0.0002", 12317, 5},
		{"0.0005", 3425, 0},
		{"0.001", 1259, 0},
		{"0.002", 472, 0},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, c.level)})
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}
			if rl.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", rl.Episodes, c.episodes)
			}
			if rl.Censored != c.censored {
				t.Errorf("Censored = %d, want %d", rl.Censored, c.censored)
			}
		})
	}
}

// An undeclared level REFUSES. It does not return an empty answer, which is
// what a caller with a typo would read as "measured, and nothing happened".
func TestLoad_UndeclaredLevelRefuses(t *testing.T) {
	set := loadCommitted(t)
	_, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0003")})
	if !errors.Is(err, persistence.ErrLevelUndeclared) {
		t.Fatalf("RunLength(0.0003) = %v, want ErrLevelUndeclared", err)
	}
}

// F102: the emitter writes TWO timestamp layouts, 133 lines apart, and neither
// is RFC3339. Go's time.RFC3339 rejects the episode layout outright. This test
// is the record of that, and it is what dates the defect: summarise.py parses
// only the symbol-level pair, and Python's fromisoformat accepts both anyway,
// so 34 956 episode instants have never been parsed by any consumer.
func TestLoad_TwoTimestampLayouts(t *testing.T) {
	t.Run("the episode layout is not RFC3339", func(t *testing.T) {
		const sample = "2025-10-27T08:00Z"
		if _, err := time.Parse(time.RFC3339, sample); err == nil {
			t.Fatal("time.RFC3339 parsed the episode layout; F102 would be stale")
		}
		if _, err := time.Parse(persistence.EpisodeInstantLayout, sample); err != nil {
			t.Fatalf("EpisodeInstantLayout rejected %q: %v", sample, err)
		}
	})
	t.Run("the span layout is RFC3339-shaped", func(t *testing.T) {
		const sample = "2025-10-27T08:00:00Z"
		if _, err := time.Parse(persistence.SpanInstantLayout, sample); err != nil {
			t.Fatalf("SpanInstantLayout rejected %q: %v", sample, err)
		}
	})
	t.Run("both are decoded from the committed file", func(t *testing.T) {
		set := loadCommitted(t)
		rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0002")})
		if err != nil {
			t.Fatalf("RunLength: %v", err)
		}
		// The censored records are the ones whose instants we can name exactly:
		// all five sit at the tape's final decision point.
		if rl.Censored != 5 {
			t.Fatalf("Censored = %d, want 5", rl.Censored)
		}
	})
}

// A path that is not a file refuses, rather than yielding an empty Set whose
// every answer would be a confident zero.
func TestLoad_MissingFileRefuses(t *testing.T) {
	if _, err := persistence.Load("testdata/no-such-file.json"); err == nil {
		t.Fatal("Load of a missing path returned nil error")
	}
}
