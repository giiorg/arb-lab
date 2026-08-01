package driving_test

// The refusal-frame battery. §41i's named suspects, answered by
// construction: the H1 fixture set contains the boundary instant ITSELF
// (strictly-inside/outside fixtures make < and <= the same predicate); the
// completeness fixtures include a missing row AND an inserted row (a
// gap-only suite leaves the "or extra" half untested); and the
// execution-binding cases include the dirty and mismatch refusals BESIDE
// the clean positive control (a suite of failures alone cannot tell a
// working guard from one that refuses everything).

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

func TestGuardCalibrationInput(t *testing.T) {
	// Strictly before the boundary: calibration era.
	if err := driving.GuardCalibrationInput(driving.H1Boundary.Add(-time.Second)); err != nil {
		t.Errorf("one second before boundary refused: %v", err)
	}
	// EXACTLY the boundary instant: holdout — on-or-after, and this case
	// is what separates < from <=.
	if err := driving.GuardCalibrationInput(driving.H1Boundary); !errors.Is(err, driving.ErrHoldoutEra) {
		t.Errorf("boundary instant = %v; want ErrHoldoutEra", err)
	}
	if err := driving.GuardCalibrationInput(driving.H1Boundary.Add(time.Second)); !errors.Is(err, driving.ErrHoldoutEra) {
		t.Errorf("after boundary = %v; want ErrHoldoutEra", err)
	}
	// The zero instant would read as "before every boundary" and pass.
	if err := driving.GuardCalibrationInput(time.Time{}); !errors.Is(err, driving.ErrInstantUnset) {
		t.Errorf("zero instant = %v; want ErrInstantUnset", err)
	}
}

func TestGuardHoldoutFetch(t *testing.T) {
	after := driving.SealedAt.Add(24 * time.Hour)

	if err := driving.GuardHoldoutFetch(driving.SealVenue, after); err != nil {
		t.Errorf("sealed venue after seal refused: %v", err)
	}
	if err := driving.GuardHoldoutFetch(driving.SealVenue, driving.SealedAt.Add(-time.Second)); !errors.Is(err, driving.ErrFetchBeforeSeal) {
		t.Errorf("fetch before seal = %v; want ErrFetchBeforeSeal", err)
	}
	if err := driving.GuardHoldoutFetch("bybit", after); !errors.Is(err, driving.ErrIdentityUnsealed) {
		t.Errorf("unsealed venue = %v; want ErrIdentityUnsealed", err)
	}
	if err := driving.GuardHoldoutFetch(driving.SealVenue, time.Time{}); !errors.Is(err, driving.ErrInstantUnset) {
		t.Errorf("zero instant = %v; want ErrInstantUnset", err)
	}
}

func TestGuardArchiveDay(t *testing.T) {
	manifested := func(day string) bool { return day == "2026-07-30" }

	if err := driving.GuardArchiveDay("2026-07-30", manifested); err != nil {
		t.Errorf("manifested day refused: %v", err)
	}
	if err := driving.GuardArchiveDay("2026-07-31", manifested); !errors.Is(err, driving.ErrDayUnmanifested) {
		t.Errorf("unmanifested day = %v; want ErrDayUnmanifested", err)
	}
	if err := driving.GuardArchiveDay("2026-07-30", nil); !errors.Is(err, driving.ErrDayUnmanifested) {
		t.Errorf("nil predicate = %v; want ErrDayUnmanifested", err)
	}
	if err := driving.GuardArchiveDay("", manifested); !errors.Is(err, driving.ErrInstantUnset) {
		t.Errorf("empty day = %v; want ErrInstantUnset", err)
	}
}

func TestCheckComplete(t *testing.T) {
	t0 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	grid := driving.SettlementGrid{Start: t0, End: t0.Add(24 * time.Hour), Interval: 8 * time.Hour}
	full := []time.Time{t0, t0.Add(8 * time.Hour), t0.Add(16 * time.Hour), t0.Add(24 * time.Hour)}

	// The positive control: the exact grid, END INCLUSIVE.
	if err := driving.CheckComplete(grid, full); err != nil {
		t.Errorf("exact grid refused: %v", err)
	}

	t.Run("missing middle row", func(t *testing.T) {
		rows := []time.Time{full[0], full[2], full[3]}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetGap) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetGap", err)
		}
	})
	t.Run("missing trailing row", func(t *testing.T) {
		if err := driving.CheckComplete(grid, full[:3]); !errors.Is(err, driving.ErrDatasetGap) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetGap", err)
		}
	})
	t.Run("inserted off-grid row", func(t *testing.T) {
		rows := []time.Time{full[0], t0.Add(4 * time.Hour), full[1], full[2], full[3]}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetExtra) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetExtra", err)
		}
	})
	// A SUBSTITUTED row — one grid row replaced by an off-grid instant, the
	// COUNT unchanged. The count backstop cannot see this one: a mutant
	// that drops the positional check accepts the tampered dataset clean
	// (found by the mutation battery — the inserted-row fixture above also
	// changes the count, so the backstop masked the positional claim).
	t.Run("substituted off-grid row", func(t *testing.T) {
		rows := []time.Time{full[0], t0.Add(4 * time.Hour), full[2], full[3]}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetExtra) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetExtra", err)
		}
	})
	// The MIRROR of the substitution: rows shifted late, count unchanged —
	// t0+8h missing, t0+32h present. The gap must be named at its
	// position; a mutant that drops the After check consumes every row and
	// validates this tampered dataset clean.
	t.Run("late-shifted rows", func(t *testing.T) {
		rows := []time.Time{full[0], full[2], full[3], t0.Add(32 * time.Hour)}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetGap) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetGap", err)
		}
	})
	t.Run("trailing row past the range", func(t *testing.T) {
		rows := append(append([]time.Time{}, full...), t0.Add(32*time.Hour))
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetExtra) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetExtra", err)
		}
	})
	t.Run("rows out of order", func(t *testing.T) {
		rows := []time.Time{full[1], full[0], full[2], full[3]}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetOrder) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetOrder", err)
		}
	})
	t.Run("duplicate row", func(t *testing.T) {
		rows := []time.Time{full[0], full[0], full[1], full[2], full[3]}
		if err := driving.CheckComplete(grid, rows); !errors.Is(err, driving.ErrDatasetOrder) {
			t.Fatalf("CheckComplete = %v; want ErrDatasetOrder", err)
		}
	})
	t.Run("single-instant grid", func(t *testing.T) {
		g := driving.SettlementGrid{Start: t0, End: t0, Interval: 8 * time.Hour}
		if err := driving.CheckComplete(g, []time.Time{t0}); err != nil {
			t.Fatalf("single-instant grid refused: %v", err)
		}
		if err := driving.CheckComplete(g, nil); !errors.Is(err, driving.ErrDatasetGap) {
			t.Fatalf("empty rows = %v; want ErrDatasetGap", err)
		}
	})
	t.Run("malformed grid", func(t *testing.T) {
		g := driving.SettlementGrid{Start: t0, End: t0.Add(-time.Hour), Interval: 8 * time.Hour}
		if err := driving.CheckComplete(g, full); !errors.Is(err, driving.ErrDatasetOrder) {
			t.Fatalf("inverted grid = %v; want ErrDatasetOrder", err)
		}
		if err := driving.CheckComplete(driving.SettlementGrid{}, full); !errors.Is(err, driving.ErrInstantUnset) {
			t.Fatalf("zero grid = %v; want ErrInstantUnset", err)
		}
	})
}

func TestCheckExecutionBinding(t *testing.T) {
	const rev = "a5b5d33000000000000000000000000000000000"

	// The clean positive control, beside the refusals.
	if err := driving.CheckExecutionBinding(rev, false, rev); err != nil {
		t.Errorf("clean matching build refused: %v", err)
	}
	if err := driving.CheckExecutionBinding(rev, true, rev); !errors.Is(err, driving.ErrTreeDirty) {
		t.Errorf("dirty tree = %v; want ErrTreeDirty", err)
	}
	if err := driving.CheckExecutionBinding(rev, false, "b000000000000000000000000000000000000000"); !errors.Is(err, driving.ErrRevisionMismatch) {
		t.Errorf("mismatched revision = %v; want ErrRevisionMismatch", err)
	}
	if err := driving.CheckExecutionBinding("", false, rev); !errors.Is(err, driving.ErrRevisionUnavailable) {
		t.Errorf("no revision = %v; want ErrRevisionUnavailable", err)
	}
	if err := driving.CheckExecutionBinding(rev, false, ""); !errors.Is(err, driving.ErrRevisionUnavailable) {
		t.Errorf("no recording = %v; want ErrRevisionUnavailable", err)
	}
}

// ReadBuildRevision's answer depends on how the test binary was built; the
// claim testable everywhere is that it either refuses by name or returns a
// non-empty revision — never an empty success, which CheckExecutionBinding
// would then bind against nothing.
func TestReadBuildRevision(t *testing.T) {
	rev, _, err := driving.ReadBuildRevision()
	if err != nil {
		if !errors.Is(err, driving.ErrRevisionUnavailable) {
			t.Fatalf("ReadBuildRevision: %v", err)
		}
		return
	}
	if rev == "" {
		t.Fatal("nil error with empty revision")
	}
}

func TestGuardNewVersion(t *testing.T) {
	exists := func(p string) bool { return p == "configs/calibration.btcusdt.v1.json" }

	if err := driving.GuardNewVersion("configs/calibration.btcusdt.v2.json", exists); err != nil {
		t.Errorf("fresh version refused: %v", err)
	}
	if err := driving.GuardNewVersion("configs/calibration.btcusdt.v1.json", exists); !errors.Is(err, driving.ErrArtifactExists) {
		t.Errorf("existing version = %v; want ErrArtifactExists", err)
	}
	if err := driving.GuardNewVersion("", exists); !errors.Is(err, driving.ErrInstantUnset) {
		t.Errorf("empty path = %v; want ErrInstantUnset", err)
	}
}

func TestCheckDatasetDigest(t *testing.T) {
	data := []byte(`{"rows":[1,2,3]}`)
	// sha256 of data, lowercase — computed by the same rule the check
	// applies, so this is the positive control.
	recorded := "0a1952e4361b61e16dbdcac2e26925e489c8bc81fe637ea3f5af424f0fe882af"

	if err := driving.CheckDatasetDigest(recorded, data); err != nil {
		t.Errorf("matching digest refused: %v", err)
	}
	if err := driving.CheckDatasetDigest(recorded, append(data, '\n')); !errors.Is(err, driving.ErrDatasetDigestMismatch) {
		t.Errorf("changed bytes = %v; want ErrDatasetDigestMismatch", err)
	}
	// An UPPERCASE recorded digest is malformed, not case-normalized —
	// §41d's encoding, checked by driven's one implementation.
	if err := driving.CheckDatasetDigest("0A1952E4361B61E16DBDCAC2E26925E489C8BC81FE637EA3F5AF424F0FE882AF", data); !errors.Is(err, driven.ErrArtifactHashFormat) {
		t.Errorf("uppercase digest = %v; want ErrArtifactHashFormat", err)
	}
}

// The slice-B unbuilt door is retired: Run is real as of slice C, and its
// FIRST refusal is the world's completeness — an unspecified config
// refuses by name before the §41e binding, before one byte is derived
// (run_test.go holds the rest of Run's battery).
func TestCalibratorRunRefusesUnspecifiedConfig(t *testing.T) {
	_, err := (driving.Calibrator{}).Run(driving.RunConfig{})
	if !errors.Is(err, domain.ErrUnset) {
		t.Errorf("Run over an empty config = %v; want domain.ErrUnset (paths)", err)
	}
}
