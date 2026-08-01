package crossvenue_test

// Episode-machine fixtures, checklist-first. The boundary fixtures sit ON
// their edges: a spacing exactly equal to maxGap must NOT close (the rule
// is strictly-wider), a signal exactly at the entry level must open, one
// exactly at the exit level must hold, and the exit-rounding fixture uses
// an odd mantissa so RoundTowardZero and RoundHalfEven give DIFFERENT exit
// levels — a fixture at an even mantissa could not tell them apart.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

const grid = 4 * time.Hour

// gridPoints builds a decision series at the 4h grid from signal decimals.
func gridPoints(t *testing.T, sigs ...string) []crossvenue.DecisionPoint {
	t.Helper()
	pts := make([]crossvenue.DecisionPoint, len(sigs))
	for i, s := range sigs {
		pts[i] = point(t, cvBase.Add(time.Duration(i)*grid), s)
	}
	return pts
}

func episodes(t *testing.T, pts []crossvenue.DecisionPoint, entry string) []crossvenue.Window {
	t.Helper()
	ws, err := crossvenue.Episodes(pts, rate(t, entry), 2, 2*grid)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	return ws
}

func TestEpisodes_TriggerObservedNotCounted(t *testing.T) {
	// Trigger at t0, hold t1, exit below E/2 at t2: one window open at t0,
	// closed at t2, with TWO points — the trigger is observed, never
	// counted; the exit print is.
	ws := episodes(t, gridPoints(t, "0.0005", "0.0004", "0.0001"), "0.0005")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1", len(ws))
	}
	w := ws[0]
	if !w.OpenAt.Equal(cvBase) || !w.CloseAt.Equal(cvBase.Add(2*grid)) {
		t.Fatalf("window %s..%s, want %s..%s", w.OpenAt, w.CloseAt, cvBase, cvBase.Add(2*grid))
	}
	if w.Sign != 1 {
		t.Fatalf("sign %d, want +1", w.Sign)
	}
	if w.Points != 2 {
		t.Fatalf("points %d, want 2 (hold + exit, trigger excluded)", w.Points)
	}
}

func TestEpisodes_EntryBoundaryOpens(t *testing.T) {
	// |signal| exactly AT the entry level opens; one tick below does not.
	if ws := episodes(t, gridPoints(t, "0.0005"), "0.0005"); len(ws) != 1 {
		t.Fatalf("at-entry: windows %d, want 1 (>= opens)", len(ws))
	}
	if ws := episodes(t, gridPoints(t, "0.000499999"), "0.0005"); len(ws) != 0 {
		t.Fatalf("below entry: windows %d, want 0", len(ws))
	}
}

func TestEpisodes_ExitBoundaryHolds(t *testing.T) {
	// Exit level is entry/2 = 0.00025: a print exactly AT it holds (close
	// is strictly-below), so the window runs to the end of data.
	ws := episodes(t, gridPoints(t, "0.0005", "0.00025", "0.00025"), "0.0005")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1", len(ws))
	}
	if !ws[0].CloseAt.Equal(cvBase.Add(2 * grid)) {
		t.Fatalf("close %s, want end of data %s", ws[0].CloseAt, cvBase.Add(2*grid))
	}
	if ws[0].Points != 2 {
		t.Fatalf("points %d, want 2", ws[0].Points)
	}
}

func TestEpisodes_ExitLevelRoundsTowardZero(t *testing.T) {
	// Entry mantissa 3 (0.000000003), divisor 2: toward-zero exit is
	// mantissa 1; half-even would round 1.5 up to 2. A signal at mantissa
	// 1 must therefore HOLD — under half-even it would close.
	pts := gridPoints(t, "0.000000003", "0.000000001", "0.000000001")
	ws := episodes(t, pts, "0.000000003")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1", len(ws))
	}
	if !ws[0].CloseAt.Equal(cvBase.Add(2*grid)) || ws[0].Points != 2 {
		t.Fatalf("window %+v: a signal at the toward-zero exit level must hold to end of data", ws[0])
	}
}

func TestEpisodes_SignFlipClosesAndRetriggers(t *testing.T) {
	// A flip that lands beyond the entry level closes the first window AT
	// the flip print and opens a second one AT the same instant, opposite
	// sign. The flip print is counted in the first window (it settled
	// against the open position) and observed for the second.
	ws := episodes(t, gridPoints(t, "0.0005", "-0.0006", "-0.0005", "0.0001"), "0.0005")
	if len(ws) != 2 {
		t.Fatalf("windows %d, want 2", len(ws))
	}
	flipAt := cvBase.Add(1 * grid)
	if !ws[0].CloseAt.Equal(flipAt) || ws[0].Points != 1 || ws[0].Sign != 1 {
		t.Fatalf("first window %+v, want close at flip with 1 point, sign +1", ws[0])
	}
	if !ws[1].OpenAt.Equal(flipAt) || ws[1].Sign != -1 {
		t.Fatalf("second window %+v, want open at flip, sign -1", ws[1])
	}
	if !ws[1].CloseAt.Equal(cvBase.Add(3*grid)) || ws[1].Points != 2 {
		t.Fatalf("second window %+v, want close at t3 with 2 points", ws[1])
	}
}

func TestEpisodes_FlipExactlyAtEntryRetriggers(t *testing.T) {
	// The flip lands EXACTLY at -entry: the retrigger comparison is >= —
	// a strictly-greater rule would drop this second window.
	ws := episodes(t, gridPoints(t, "0.0005", "-0.0005", "-0.0004"), "0.0005")
	if len(ws) != 2 {
		t.Fatalf("windows %d, want 2 — a flip landing at the entry level retriggers", len(ws))
	}
	if ws[1].Sign != -1 || !ws[1].OpenAt.Equal(cvBase.Add(1*grid)) {
		t.Fatalf("second window %+v, want sign -1 open at the flip", ws[1])
	}
}

func TestEpisodes_FlipBelowEntryClosesWithoutRetrigger(t *testing.T) {
	// A flip that lands below the entry level closes and stays flat.
	ws := episodes(t, gridPoints(t, "0.0005", "-0.0003", "-0.0004"), "0.0005")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1 — a flip below entry must not retrigger", len(ws))
	}
	if ws[0].Points != 1 {
		t.Fatalf("points %d, want 1", ws[0].Points)
	}
}

func TestEpisodes_GapBoundary(t *testing.T) {
	// maxGap is 2 grid steps. A spacing of EXACTLY 2 steps does not close
	// (strictly-wider closes); a spacing of 2 steps + 1ns does, at the
	// PREVIOUS instant, and the far print triggers a fresh window.
	entry := "0.0005"
	atGap := []crossvenue.DecisionPoint{
		point(t, cvBase, entry),
		point(t, cvBase.Add(2*grid), "0.0004"),
	}
	ws, err := crossvenue.Episodes(atGap, rate(t, entry), 2, 2*grid)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(ws) != 1 || ws[0].Points != 1 || !ws[0].CloseAt.Equal(cvBase.Add(2*grid)) {
		t.Fatalf("windows %+v: spacing exactly maxGap must NOT close early", ws)
	}

	pastGap := []crossvenue.DecisionPoint{
		point(t, cvBase, entry),
		point(t, cvBase.Add(2*grid+time.Nanosecond), entry),
		point(t, cvBase.Add(3*grid+time.Nanosecond), "0.0001"),
	}
	ws, err = crossvenue.Episodes(pastGap, rate(t, entry), 2, 2*grid)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(ws) != 2 {
		t.Fatalf("windows %d, want 2 — gap closes the first, far print retriggers", len(ws))
	}
	if !ws[0].CloseAt.Equal(cvBase) || ws[0].Points != 0 {
		t.Fatalf("first window %+v, want gap-close at the previous (trigger) instant with 0 points", ws[0])
	}
	if !ws[1].OpenAt.Equal(cvBase.Add(2*grid+time.Nanosecond)) || ws[1].Points != 1 {
		t.Fatalf("second window %+v, want open at the far print", ws[1])
	}
}

func TestEpisodes_EndOfDataCloses(t *testing.T) {
	ws := episodes(t, gridPoints(t, "0.0001", "0.0006", "0.0006"), "0.0005")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1", len(ws))
	}
	if !ws[0].OpenAt.Equal(cvBase.Add(1*grid)) || !ws[0].CloseAt.Equal(cvBase.Add(2*grid)) || ws[0].Points != 1 {
		t.Fatalf("window %+v, want open t1 close t2 with 1 point", ws[0])
	}
}

func TestEpisodes_ZeroSignalClosesOpenWindow(t *testing.T) {
	// A zero print while open is |0| < exit: close, no retrigger.
	ws := episodes(t, gridPoints(t, "0.0005", "0", "0"), "0.0005")
	if len(ws) != 1 {
		t.Fatalf("windows %d, want 1", len(ws))
	}
	if !ws[0].CloseAt.Equal(cvBase.Add(1 * grid)) {
		t.Fatalf("close %s, want the zero print's instant", ws[0].CloseAt)
	}
}

func TestEpisodes_NegativeEntryDirection(t *testing.T) {
	ws := episodes(t, gridPoints(t, "-0.0005", "-0.0004", "0"), "0.0005")
	if len(ws) != 1 || ws[0].Sign != -1 {
		t.Fatalf("windows %+v, want one with sign -1", ws)
	}
}

func TestEpisodes_ExitDivisorOneClosesBelowEntry(t *testing.T) {
	// Divisor 1: exit == entry, the no-hysteresis boundary. A hold print
	// below entry closes immediately.
	ws, err := crossvenue.Episodes(gridPoints(t, "0.0005", "0.0004", "0.0006"), rate(t, "0.0005"), 1, 2*grid)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(ws) != 2 {
		t.Fatalf("windows %d, want 2 — divisor 1 closes on the first sub-entry print, then t2 retriggers... (%+v)", len(ws), ws)
	}
	if !ws[0].CloseAt.Equal(cvBase.Add(1 * grid)) {
		t.Fatalf("first close %s, want t1", ws[0].CloseAt)
	}
}

func TestEpisodes_Refusals(t *testing.T) {
	good := gridPoints(t, "0.0005")

	if _, err := crossvenue.Episodes(nil, rate(t, "0.0005"), 2, grid); !errors.Is(err, crossvenue.ErrNoPoints) {
		t.Fatalf("empty points: %v, want ErrNoPoints", err)
	}
	if _, err := crossvenue.Episodes(good, rate(t, "0"), 2, grid); !errors.Is(err, crossvenue.ErrEntryInvalid) {
		t.Fatalf("zero entry: %v, want ErrEntryInvalid", err)
	}
	if _, err := crossvenue.Episodes(good, rate(t, "-0.0005"), 2, grid); !errors.Is(err, crossvenue.ErrEntryInvalid) {
		t.Fatalf("negative entry: %v, want ErrEntryInvalid", err)
	}
	if _, err := crossvenue.Episodes(good, rate(t, "0.0005"), 0, grid); !errors.Is(err, crossvenue.ErrExitDivisorInvalid) {
		t.Fatalf("divisor 0: %v, want ErrExitDivisorInvalid", err)
	}
	if _, err := crossvenue.Episodes(good, rate(t, "0.0005"), 2, 0); !errors.Is(err, crossvenue.ErrGapInvalid) {
		t.Fatalf("zero gap: %v, want ErrGapInvalid", err)
	}

	tied := []crossvenue.DecisionPoint{point(t, cvBase, "0.0001"), point(t, cvBase, "0.0002")}
	if _, err := crossvenue.Episodes(tied, rate(t, "0.0005"), 2, grid); !errors.Is(err, crossvenue.ErrSeriesDisordered) {
		t.Fatalf("tied instants: %v, want ErrSeriesDisordered", err)
	}
}

func TestWindowValidate(t *testing.T) {
	good := crossvenue.Window{OpenAt: cvBase, CloseAt: cvBase.Add(grid), Sign: 1}
	if err := good.Validate(); err != nil {
		t.Fatalf("good window refused: %v", err)
	}
	// Open == close is legal: a gap-killed trigger is an episode with no
	// prints, and it still gets charged.
	empty := crossvenue.Window{OpenAt: cvBase, CloseAt: cvBase, Sign: -1}
	if err := empty.Validate(); err != nil {
		t.Fatalf("empty window refused: %v", err)
	}
	for name, w := range map[string]crossvenue.Window{
		"zero value":   {},
		"sign 0":       {OpenAt: cvBase, CloseAt: cvBase.Add(grid)},
		"sign 2":       {OpenAt: cvBase, CloseAt: cvBase.Add(grid), Sign: 2},
		"close < open": {OpenAt: cvBase.Add(grid), CloseAt: cvBase, Sign: 1},
		"points -1":    {OpenAt: cvBase, CloseAt: cvBase.Add(grid), Sign: 1, Points: -1},
		"zero open":    {CloseAt: cvBase, Sign: 1},
	} {
		if err := w.Validate(); !errors.Is(err, crossvenue.ErrWindowInvalid) {
			t.Fatalf("%s: %v, want ErrWindowInvalid", name, err)
		}
	}
}
