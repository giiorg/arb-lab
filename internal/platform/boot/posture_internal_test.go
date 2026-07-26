package boot

import "testing"

// TestMinPostureForFallback pins a branch no exported call can reach, and it
// exists because the bite-check said so: mutating minPostureFor's final return
// from PostureHalted to PostureTrading SURVIVED the whole suite.
//
// The diagnosis is that Permits validates the act before consulting this
// function, so an unspecified or out-of-range Act is refused one line earlier
// and the fallback never runs. That does NOT make the fallback deletable —
// Go requires the function to return on every path — so the honest resolution
// is neither to remove it nor to leave it unchecked, but to test it where it
// can be reached. Same treatment fixedpoint gave its own unreachable-by-guard
// branches.
//
// The direction is the point. This is the value an act nothing recognises maps
// to, so it must be the one that permits nothing: a future caller that reaches
// minPostureFor without validating first should get a refusal, not the run of
// the venue.
func TestMinPostureForFallback(t *testing.T) {
	t.Parallel()

	for _, a := range []Act{ActUnspecified, Act(4), Act(255)} {
		if got := minPostureFor(a); got != PostureHalted {
			t.Errorf("minPostureFor(%s) = %s, want PostureHalted", a, got)
		}
	}

	// Positive control: the same function does return the real rungs for the
	// real acts, so the refusals above are about the fallback and not about a
	// function that has stopped mapping anything.
	for a, want := range map[Act]Posture{
		ActReconcile: PostureReconcileOnly,
		ActReduce:    PostureStopOpening,
		ActOpen:      PostureTrading,
	} {
		if got := minPostureFor(a); got != want {
			t.Errorf("minPostureFor(%s) = %s, want %s", a, got, want)
		}
	}
}
