package crossvenue_test

// The episode reference: a second implementation with a different shape —
// index-based interval scanning over plain int64 mantissas (no fixedpoint
// anywhere), integer division for the exit level (identical to
// RoundTowardZero on a positive dividend), explicit inner scan for the
// close index instead of a state machine. The generator draws signals
// from a lattice of quarter-entry steps so the entry and exit boundaries
// are hit constantly, and spacings that jump the gap bound occur.

import (
	"fmt"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

type refWindow struct {
	openAt, closeAt time.Time
	sign            int
	points          int
}

func refEpisodes(ats []time.Time, mants []int64, entry int64, divisor int, maxGap time.Duration) []refWindow {
	exit := entry / int64(divisor)
	abs := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	sgn := func(v int64) int {
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
		return 0
	}

	var out []refWindow
	n := len(ats)
	i := 0
	for i < n {
		if abs(mants[i]) < entry {
			i++
			continue
		}
		w := refWindow{openAt: ats[i], sign: sgn(mants[i])}
		j := i + 1
		reopenAt := -1 // index to resume scanning from
		for {
			if j >= n {
				w.closeAt = ats[n-1]
				reopenAt = n
				break
			}
			if ats[j].Sub(ats[j-1]) > maxGap {
				w.closeAt = ats[j-1]
				reopenAt = j // the far print may itself trigger
				break
			}
			w.points++
			if sgn(mants[j]) != w.sign || abs(mants[j]) < exit {
				w.closeAt = ats[j]
				reopenAt = j // a flip beyond entry retriggers on this print
				break
			}
			j++
		}
		out = append(out, w)
		if reopenAt == i {
			// Cannot happen: the close index is never the open index.
			panic("reference stuck")
		}
		i = reopenAt
		// A print that closed a window may only retrigger if it clears the
		// entry level — the top of the loop re-checks exactly that. A print
		// that closed on |s| < exit is below entry too, so the loop just
		// advances past it after the check fails.
	}
	return out
}

func TestProperty_EpisodesMatchReference(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		const entryMant = int64(400)
		divisor := rapid.IntRange(1, 3).Draw(t, "divisor")
		n := rapid.IntRange(1, 30).Draw(t, "n")

		// Spacings: mostly one grid step, sometimes three (past the two-step
		// gap bound).
		maxGap := 2 * grid
		ats := make([]time.Time, n)
		mants := make([]int64, n)
		at := cvBase
		for i := range n {
			if i > 0 {
				step := time.Duration(rapid.SampledFrom([]int{1, 1, 1, 3}).Draw(t, fmt.Sprintf("s%d", i)))
				at = at.Add(step * grid)
			}
			ats[i] = at
			// Quarter-entry lattice: 100·k for k in [-6,6] hits the entry
			// (±400) and every divisor's exit level exactly.
			mants[i] = 100 * int64(rapid.IntRange(-6, 6).Draw(t, fmt.Sprintf("m%d", i)))
		}

		pts := make([]crossvenue.DecisionPoint, n)
		for i := range pts {
			pts[i] = crossvenue.DecisionPoint{At: ats[i], Signal: fixedpoint.RateFromMantissa(mants[i])}
		}
		got, err := crossvenue.Episodes(pts, fixedpoint.RateFromMantissa(entryMant), divisor, maxGap)
		if err != nil {
			t.Fatalf("Episodes: %v", err)
		}
		want := refEpisodes(ats, mants, entryMant, divisor, maxGap)

		if len(got) != len(want) {
			t.Fatalf("windows %d, reference %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
		}
		for k := range got {
			g, r := got[k], want[k]
			if !g.OpenAt.Equal(r.openAt) || !g.CloseAt.Equal(r.closeAt) || g.Sign != r.sign || g.Points != r.points {
				t.Fatalf("window %d: production %+v, reference %+v", k, g, r)
			}
		}
	})
}
