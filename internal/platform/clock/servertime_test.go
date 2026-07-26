package clock_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// ServerTime exists because a funding boundary is decided against an exchange's
// clock, not this machine's, and the two differ by an amount we only know to
// within a band. So the question "has the settlement at T happened?" has THREE
// answers, and the type's job is to make the third one impossible to skip.
//
// The alternative — handing back an instant and a duration and letting each call
// site compare them — is what this file is written to rule out. Every call site
// doing that arithmetic gets it slightly differently, and the one that forgets
// the band answers "unknown" as either yes or no. That is a decision made on a
// coin flip, in the code path that decides whether we are holding a position
// through a funding settlement.

var boundary = time.Date(2026, time.July, 26, 8, 0, 0, 0, time.UTC)

func TestServerTime_Passed(t *testing.T) {
	// Read each case as: the true server time is somewhere in
	// [instant-band, instant+band], and the boundary is at 08:00:00.
	cases := []struct {
		name   string
		offset time.Duration // instant, relative to the boundary
		band   time.Duration
		want   clock.Certainty
	}{
		{
			// The whole band is past the boundary, so no reading inside it is
			// before — the settlement has happened however the band falls.
			name: "band entirely after", offset: time.Second, band: 100 * time.Millisecond,
			want: clock.CertainlyPassed,
		},
		{
			name: "band entirely before", offset: -time.Second, band: 100 * time.Millisecond,
			want: clock.CertainlyNotPassed,
		},
		{
			// The band straddles the boundary. This is the case the type exists
			// for: both answers are consistent with what we know, so neither is
			// a legitimate answer.
			name: "band straddles the boundary", offset: 0, band: time.Second,
			want: clock.Uncertain,
		},
		{
			// Asymmetric straddles, both directions — the estimate is on one
			// side while the band still reaches the other.
			name: "estimate past, band reaches back", offset: 500 * time.Millisecond, band: time.Second,
			want: clock.Uncertain,
		},
		{
			name: "estimate before, band reaches forward", offset: -500 * time.Millisecond, band: time.Second,
			want: clock.Uncertain,
		},
		{
			// A zero band is a claim of perfect knowledge, and it is the one
			// case where the boundary's own inclusivity shows. The boundary is
			// the settlement instant, so being exactly on it counts as passed.
			name: "exactly on the boundary, no band", offset: 0, band: 0,
			want: clock.CertainlyPassed,
		},
		{
			name: "one nanosecond before, no band", offset: -time.Nanosecond, band: 0,
			want: clock.CertainlyNotPassed,
		},
		{
			// The band's edge exactly on the boundary. Earliest == boundary is
			// "passed" for the same reason as above: the interval's worst case
			// is still on the settlement instant.
			name: "band's earliest edge exactly on the boundary", offset: time.Second, band: time.Second,
			want: clock.CertainlyPassed,
		},
		{
			// And the far edge. Latest == boundary means the best case is
			// exactly on it, which counts as passed — so this is uncertain, not
			// certainly-not.
			name: "band's latest edge exactly on the boundary", offset: -time.Second, band: time.Second,
			want: clock.Uncertain,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := clock.NewServerTime(boundary.Add(tc.offset), tc.band)
			if err != nil {
				t.Fatalf("NewServerTime: %v", err)
			}
			if got := st.Passed(boundary); got != tc.want {
				t.Errorf("Passed() = %v; want %v (instant %v, band %v)",
					got, tc.want, tc.offset, tc.band)
			}
		})
	}
}

// The zero value must be unusable, for the reason fixedpoint's categories are:
// an unpopulated field that reads as a usable value computes the economic gate in
// our favour with nothing failing. Here the specific hazard is a venue whose
// clock was never calibrated — ServerNow has no error to return, so this is where
// that shows up.
func TestServerTime_zeroValueIsUnusable(t *testing.T) {
	var unset clock.ServerTime

	if err := unset.Validate(); !errors.Is(err, clock.ErrServerTimeUnset) {
		t.Errorf("zero ServerTime.Validate() = %v; want ErrServerTimeUnset", err)
	}

	// Safe by default AND loud on request. Passed() cannot answer, so it says
	// so — a caller who skips Validate stalls rather than acting on a clock it
	// does not have. Answering either way here would be the worst outcome: it
	// would make an uncalibrated venue look like a working one.
	for _, at := range []time.Time{boundary, boundary.Add(-time.Hour), boundary.Add(time.Hour)} {
		if got := unset.Passed(at); got != clock.Uncertain {
			t.Errorf("zero ServerTime.Passed(%v) = %v; want Uncertain", at, got)
		}
	}
}

// A negative band is not a wide band, it is a caller who computed it wrong —
// most likely by subtracting two instants in the wrong order. Accepting it would
// make Earliest() later than Latest() and quietly invert every comparison above.
func TestNewServerTime_rejects(t *testing.T) {
	if _, err := clock.NewServerTime(boundary, -time.Nanosecond); !errors.Is(err, clock.ErrUncertaintyNegative) {
		t.Errorf("NewServerTime with a negative band = %v; want ErrUncertaintyNegative", err)
	}
	if _, err := clock.NewServerTime(time.Time{}, 0); !errors.Is(err, clock.ErrServerTimeUnset) {
		t.Errorf("NewServerTime with a zero instant = %v; want ErrServerTimeUnset", err)
	}
}

// The band's edges are what every comparison is actually about, so they are part
// of the contract rather than an implementation detail. A caller reporting "we
// are inside the band" wants to say how wide it is.
func TestServerTime_band(t *testing.T) {
	st, err := clock.NewServerTime(boundary, 250*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	if got, want := st.Instant(), boundary; !got.Equal(want) {
		t.Errorf("Instant() = %v; want %v", got, want)
	}
	if got, want := st.Uncertainty(), 250*time.Millisecond; got != want {
		t.Errorf("Uncertainty() = %v; want %v", got, want)
	}
	if got, want := st.Earliest(), boundary.Add(-250*time.Millisecond); !got.Equal(want) {
		t.Errorf("Earliest() = %v; want %v", got, want)
	}
	if got, want := st.Latest(), boundary.Add(250*time.Millisecond); !got.Equal(want) {
		t.Errorf("Latest() = %v; want %v", got, want)
	}
}

// Certainty is a §3.6 rule 8 enum-like: a string type with declared constants and
// a Validate. The zero value is not one of them, which is what makes a Certainty
// that was never assigned fail rather than pass as a decision.
func TestCertainty_Validate(t *testing.T) {
	for _, c := range []clock.Certainty{clock.CertainlyPassed, clock.CertainlyNotPassed, clock.Uncertain} {
		if err := c.Validate(); err != nil {
			t.Errorf("%q.Validate() = %v; want nil", c, err)
		}
	}
	for _, c := range []clock.Certainty{"", "maybe", "PASSED"} {
		if err := c.Validate(); !errors.Is(err, clock.ErrCertaintyInvalid) {
			t.Errorf("%q.Validate() = %v; want ErrCertaintyInvalid", c, err)
		}
	}
}
