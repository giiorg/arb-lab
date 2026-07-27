package venue_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// funding is the valid fixture every rejection case starts from and breaks one
// thing in, so a refusal is attributable to what was broken.
//
// The three rates are pairwise distinct and none is the negative of another.
// Cap and Floor are deliberately NOT symmetric — an 0.75%/-0.75% pair is the
// natural thing to write, and under it a comparison performed in the wrong
// direction and a comparison performed against the wrong operand give the same
// verdict for every input. Here the floor is shallower than the cap, which is
// also the real shape of Profiles §1's Binance overlay on a symbol whose clamp
// has moved.
func funding(t *testing.T) venue.Funding {
	t.Helper()
	return venue.Funding{
		Rate:       rate(t, "0.00013"),
		Interval:   8 * time.Hour,
		NextSettle: base.Add(4 * time.Hour),
		Cap:        rate(t, "0.0075"),
		Floor:      rate(t, "-0.0031"),
	}
}

func TestFundingFixtureIsValid(t *testing.T) {
	if err := funding(t).Validate(); err != nil {
		t.Fatalf("the valid fixture does not validate: %v — every rejection below would pass vacuously", err)
	}
}

// TestFundingRejects breaks one field at a time, and asserts WHICH field the
// error names as well as which sentinel it carries.
//
// Naming matters here more than usual because three of the five fields are the
// same type. Rate, Cap and Floor are all fixedpoint.Rate, so "refuses an unset
// rate" and "refuses an unset cap" carry the identical sentinel — check only
// that and the two cases are one case, and a Validate that checked Rate three
// times would pass both.
func TestFundingRejects(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*venue.Funding)
		want  error
		names string
	}{
		{
			name:  "rate unset",
			mut:   func(f *venue.Funding) { f.Rate = fixedpoint.Rate{} },
			want:  fixedpoint.ErrRateUnset,
			names: "rate",
		},
		{
			name:  "cap unset",
			mut:   func(f *venue.Funding) { f.Cap = fixedpoint.Rate{} },
			want:  fixedpoint.ErrRateUnset,
			names: "cap",
		},
		{
			name:  "floor unset",
			mut:   func(f *venue.Funding) { f.Floor = fixedpoint.Rate{} },
			want:  fixedpoint.ErrRateUnset,
			names: "floor",
		},
		{
			// The clamps arrive as two independent venue fields (Profiles §1),
			// so an inverted pair is a normalization mistake and not a venue
			// state. §2.4 clamps a forecast's later settlements to them, where
			// an inverted pair is an empty admissible range rather than a loud
			// failure.
			name:  "cap below floor",
			mut:   func(f *venue.Funding) { f.Cap, f.Floor = f.Floor, f.Cap },
			want:  venue.ErrClampInverted,
			names: "below floor",
		},
		{
			// The Bybit case, and the reason ErrIntervalInvalid exists: a
			// ticker DELTA omits unchanged fields, so a decode into a zeroed
			// struct carries interval zero. Profiles §1: "absent field =
			// unchanged, never null-coerce".
			name:  "interval zero",
			mut:   func(f *venue.Funding) { f.Interval = 0 },
			want:  venue.ErrIntervalInvalid,
			names: "interval",
		},
		{
			name:  "interval negative",
			mut:   func(f *venue.Funding) { f.Interval = -time.Hour },
			want:  venue.ErrIntervalInvalid,
			names: "interval",
		},
		{
			name:  "next settle unset",
			mut:   func(f *venue.Funding) { f.NextSettle = time.Time{} },
			want:  venue.ErrNextSettleUnset,
			names: "next-settlement",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := funding(t)
			c.mut(&f)
			err := f.Validate()
			if !errors.Is(err, c.want) {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
			if !strings.Contains(err.Error(), c.names) {
				t.Errorf("Validate() = %q, want it to name %q — three fields share one sentinel", err, c.names)
			}
		})
	}
}

// TestFundingAcceptsACapEqualToItsFloor is the boundary the strictly-ordered
// fixture above cannot reach, and it is what separates `<` from `<=`.
//
// A venue whose cap equals its floor has pinned the rate. That is a degenerate
// clamp, not an inverted one, and the reading is real: it is the venue saying
// the next settlement can only pay one number. Refusing it would drop exactly
// the reading a rate pinned at its clamp produces.
func TestFundingAcceptsACapEqualToItsFloor(t *testing.T) {
	f := funding(t)
	f.Cap = rate(t, "0.0075")
	f.Floor = rate(t, "0.0075")
	if err := f.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil — an equal cap and floor is a pinned rate, not an inverted clamp", err)
	}
}

// TestFundingAcceptsARateOutsideItsClamps pins the deliberate non-check.
//
// The rate here is an evolving ESTIMATE for a settlement that has not happened,
// and a venue is free to publish one its own clamp will later bind. Refusing it
// would discard the single most informative funding reading there is — the one
// saying the rate is pinned at the cap — and discard it as a malformed message
// rather than as a clamped one. §2.4 clamps the FORECAST, which is the number
// entering a decision.
func TestFundingAcceptsARateOutsideItsClamps(t *testing.T) {
	f := funding(t)
	f.Rate = rate(t, "0.02") // above Cap = 0.0075
	if err := f.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil — the clamp bounds the forecast, not the wire reading", err)
	}

	f.Rate = rate(t, "-0.02") // below Floor = -0.0031
	if err := f.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a rate below the floor", err)
	}
}

// TestFundingIntervalFromHours checks both ends of the band, at the boundary
// rather than beside it.
//
// MaxFundingIntervalHours itself must be ACCEPTED and one hour past it refused,
// or `>` and `>=` are the same predicate and a fixture of 480 proves nothing
// about which one is written. Likewise zero must be refused specifically —
// testing only a negative leaves `<= 0` and `< 0` indistinguishable, and zero
// is the case that actually occurs: it is what an omitted Bybit delta field
// decodes to.
func TestFundingIntervalFromHours(t *testing.T) {
	for _, hours := range []int{1, 2, 4, 8, venue.MaxFundingIntervalHours} {
		got, err := venue.FundingIntervalFromHours(hours)
		if err != nil {
			t.Errorf("FundingIntervalFromHours(%d) = %v, want nil", hours, err)
			continue
		}
		if want := time.Duration(hours) * time.Hour; got != want {
			t.Errorf("FundingIntervalFromHours(%d) = %v, want %v", hours, got, want)
		}
	}

	refused := []struct {
		name  string
		hours int
		want  error
	}{
		{"zero — an omitted Bybit delta field", 0, venue.ErrIntervalInvalid},
		{"negative", -1, venue.ErrIntervalInvalid},
		{"one hour past the band", venue.MaxFundingIntervalHours + 1, venue.ErrIntervalRange},
		{"8h sent as 480 minutes", 480, venue.ErrIntervalRange},
		{"8h sent as 28800 seconds", 28800, venue.ErrIntervalRange},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			got, err := venue.FundingIntervalFromHours(c.hours)
			if !errors.Is(err, c.want) {
				t.Fatalf("FundingIntervalFromHours(%d) error = %v, want %v", c.hours, err, c.want)
			}
			if got != 0 {
				t.Errorf("FundingIntervalFromHours(%d) = %v alongside its error, want the zero duration", c.hours, got)
			}
		})
	}
}

// TestFundingIntervalHasNoEightHourFallback is the absence this design turns on,
// asserted rather than left to the doc comment.
//
// Profiles §1 gives Binance's rule as "absent ⇒ 8h default", and the tempting
// implementation of it is a fallback on the zero value. Zero reaches this
// function from three situations that a fallback would answer identically: an
// overlay genuinely reporting nothing (where 8h is right), an overlay never
// fetched, and a Bybit ticker delta whose unchanged field is simply absent. Two
// of the three are a wiring failure wearing a correct BTC reading's clothes,
// and the third venue can flip an alt to 1h with no notice — §2.4's C8 defect,
// where the hold's settlement count silently octuples.
func TestFundingIntervalHasNoEightHourFallback(t *testing.T) {
	got, err := venue.FundingIntervalFromHours(0)
	if err == nil {
		t.Fatalf("FundingIntervalFromHours(0) = %v with no error; a zero-value fallback makes an unfetched overlay indistinguishable from a real 8h symbol", got)
	}
	if got == 8*time.Hour {
		t.Error("FundingIntervalFromHours(0) returned 8h alongside its error")
	}
}
