package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    ids.Venue("binance"),
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// obs is the fixture observation. The interval is 8h — the cash-and-carry
// baseline — and every horizon below is chosen so the discrete answer differs
// from the continuous one.
func obs(t *testing.T) domain.FundingObservation {
	t.Helper()
	return domain.FundingObservation{
		Instrument: perp(),
		Rate:       rate(t, "0.0001"),
		Interval:   8 * time.Hour,
		NextSettle: base,
		Cap:        rate(t, "0.0075"),
		Floor:      rate(t, "-0.0075"),
		Watermark:  wm(base),
	}
}

func TestFundingObservationValidate(t *testing.T) {
	t.Parallel()

	// Positive control, first. A Validate that refused everything would score a
	// perfect pass on the table below, and this project has been burned by a
	// universally-objecting predicate reading as a perfect score.
	if err := obs(t).Validate(); err != nil {
		t.Fatalf("the fixture observation must be valid: %v", err)
	}

	cases := []struct {
		name  string
		mut   func(*domain.FundingObservation)
		want  error
		names string // a substring the message must carry
	}{
		{
			name:  "spot has no funding",
			mut:   func(o *domain.FundingObservation) { o.Instrument.Product = ids.ProductSpot; o.Instrument.Contract = "" },
			want:  domain.ErrFundingOnSpot,
			names: "instrument",
		},
		{
			name:  "unconstructed rate",
			mut:   func(o *domain.FundingObservation) { o.Rate = fixedpoint.Rate{} },
			want:  fixedpoint.ErrUnset,
			names: "rate",
		},
		{
			name:  "unconstructed cap",
			mut:   func(o *domain.FundingObservation) { o.Cap = fixedpoint.Rate{} },
			want:  fixedpoint.ErrUnset,
			names: "cap",
		},
		{
			name:  "unconstructed floor",
			mut:   func(o *domain.FundingObservation) { o.Floor = fixedpoint.Rate{} },
			want:  fixedpoint.ErrUnset,
			names: "floor",
		},
		{
			name:  "cap below floor",
			mut:   func(o *domain.FundingObservation) { o.Cap, o.Floor = o.Floor, o.Cap },
			want:  domain.ErrClampInverted,
			names: "cap",
		},
		{
			// The dangerous one: a zero interval makes Enumerate's loop never
			// advance, so the failure would be a HANG rather than a wrong
			// number — and a hanging test names no defect.
			name:  "zero interval",
			mut:   func(o *domain.FundingObservation) { o.Interval = 0 },
			want:  domain.ErrIntervalInvalid,
			names: "interval",
		},
		{
			name:  "negative interval",
			mut:   func(o *domain.FundingObservation) { o.Interval = -time.Hour },
			want:  domain.ErrIntervalInvalid,
			names: "interval",
		},
		{
			name:  "unset next settle",
			mut:   func(o *domain.FundingObservation) { o.NextSettle = time.Time{} },
			want:  domain.ErrNextSettleUnset,
			names: "next settle",
		},
		{
			name:  "unset watermark",
			mut:   func(o *domain.FundingObservation) { o.Watermark.RecvTime = time.Time{} },
			want:  domain.ErrWatermarkUnset,
			names: "watermark",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			o := obs(t)
			c.mut(&o)
			err := o.Validate()
			if !errors.Is(err, c.want) {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
			// Assert the message names the field. Several cases share a
			// sentinel (three share fixedpoint.ErrUnset), so the sentinel alone
			// cannot say which check fired — §8f, found four times in one table
			// in platform/config.
			if !strings.Contains(err.Error(), c.names) {
				t.Errorf("message %q does not name %q", err, c.names)
			}
		})
	}
}

func TestEnumerate(t *testing.T) {
	t.Parallel()

	const h = time.Hour
	o := obs(t)

	cases := []struct {
		name  string
		until time.Time
		want  int
	}{
		{
			// THE case §2.4 exists for. A 20-hour hold on an 8-hour interval
			// captures settlements at +0h and +8h and +16h — three — while the
			// forbidden continuous calculation credits 20/8 = 2.5 intervals.
			// A horizon that is an exact multiple of the interval would make
			// the two agree and the rule untestable.
			name:  "20h hold on an 8h interval is not 20/8 intervals",
			until: base.Add(20 * h),
			want:  3,
		},
		{
			name:  "a horizon EXACTLY on a settlement includes it",
			until: base.Add(16 * h),
			want:  3,
		},
		{
			name:  "one ns before that settlement excludes it",
			until: base.Add(16*h - 1),
			want:  2,
		},
		{
			name:  "a horizon exactly at the next settlement captures one",
			until: base,
			want:  1,
		},
		{
			name:  "one ns before the next settlement captures none",
			until: base.Add(-1),
			want:  0,
		},
		{
			name:  "a horizon well before the next settlement captures none",
			until: base.Add(-30 * 24 * h),
			want:  0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := o.Enumerate(c.until)
			if err != nil {
				t.Fatalf("Enumerate: %v", err)
			}
			if len(got) != c.want {
				t.Fatalf("Enumerate(%s) returned %d settlements, want %d: %v", c.until, len(got), c.want, got)
			}
			// The instants themselves, not only the count: a count-correct
			// schedule at the wrong offsets prices a completely different hold.
			for i, ts := range got {
				want := base.Add(time.Duration(i) * o.Interval)
				if !ts.Equal(want) {
					t.Errorf("settlement %d = %s, want %s", i, ts, want)
				}
				if ts.After(c.until) {
					t.Errorf("settlement %d = %s is past the horizon %s", i, ts, c.until)
				}
			}
		})
	}
}

// TestEnumerateIsNotContinuous states the forbidden calculation directly and
// asserts the implementation disagrees with it. Without this, every case above
// could pass against an implementation that happened to round the continuous
// answer the same way on those particular fixtures.
func TestEnumerateIsNotContinuous(t *testing.T) {
	t.Parallel()

	o := obs(t)

	for _, hold := range []time.Duration{
		20 * time.Hour, // 2.5 intervals
		9 * time.Hour,  // 1.125
		25 * time.Hour, // 3.125
		7 * time.Hour,  // 0.875
	} {
		got, err := o.Enumerate(base.Add(hold))
		if err != nil {
			t.Fatalf("Enumerate: %v", err)
		}
		// The continuous approximation, written out so the difference is
		// visible rather than asserted about.
		continuous := int(hold / o.Interval)
		if len(got) == continuous {
			t.Errorf("hold %v: discrete count %d equals the continuous count %d — this fixture cannot tell them apart", hold, len(got), continuous)
		}
	}
}

// TestEnumerateCountAgreesWithIteration checks the division against the loop.
// Enumerate computes the settlement count twice — once by division, to refuse
// an absurd horizon before allocating, and once by iterating — and the refusal
// is only sound while the two agree. This is the repo's "write the oracle
// before trusting the implementation" rule at the smallest scale that still
// has a real defect available: the two are independent computations of one
// number, so an off-by-one in either is visible only against the other.
func TestEnumerateCountAgreesWithIteration(t *testing.T) {
	t.Parallel()

	for _, interval := range []time.Duration{time.Hour, 4 * time.Hour, 8 * time.Hour, 90 * time.Minute} {
		for _, hold := range []time.Duration{
			0, 1, interval - 1, interval, interval + 1,
			3*interval - 1, 3 * interval, 3*interval + 1,
			100 * interval,
		} {
			o := obs(t)
			o.Interval = interval

			got, err := o.Enumerate(base.Add(hold))
			if err != nil {
				t.Fatalf("interval %v hold %v: %v", interval, hold, err)
			}

			// The independent count: iterate without any division.
			want := 0
			for ts := base; !ts.After(base.Add(hold)); ts = ts.Add(interval) {
				want++
			}
			if len(got) != want {
				t.Errorf("interval %v hold %v: Enumerate returned %d, iteration counts %d", interval, hold, len(got), want)
			}
		}
	}
}

func TestEnumerateRefusals(t *testing.T) {
	t.Parallel()

	o := obs(t)

	if _, err := o.Enumerate(time.Time{}); !errors.Is(err, domain.ErrHorizonUnset) {
		t.Errorf("Enumerate(zero time) = %v, want ErrHorizonUnset", err)
	}

	// An invalid observation is refused before the horizon is looked at, so a
	// zero interval cannot reach the loop it would hang.
	bad := obs(t)
	bad.Interval = 0
	if _, err := bad.Enumerate(base.Add(time.Hour)); !errors.Is(err, domain.ErrIntervalInvalid) {
		t.Errorf("Enumerate on a zero-interval observation = %v, want ErrIntervalInvalid", err)
	}

	// The horizon cap. Tested at the boundary rather than with an absurd value:
	// with only a wildly-too-long fixture, the exact threshold is unchecked and
	// an off-by-one in the comparison survives.
	hourly := obs(t)
	hourly.Interval = time.Hour

	atLimit := base.Add(time.Duration(10_000-1) * time.Hour)
	got, err := hourly.Enumerate(atLimit)
	if err != nil {
		t.Fatalf("Enumerate at exactly the limit = %v, want nil", err)
	}
	if len(got) != 10_000 {
		t.Errorf("Enumerate at exactly the limit returned %d, want 10000", len(got))
	}

	overLimit := base.Add(time.Duration(10_000) * time.Hour)
	if _, err := hourly.Enumerate(overLimit); !errors.Is(err, domain.ErrHorizonTooLong) {
		t.Errorf("Enumerate one settlement past the limit = %v, want ErrHorizonTooLong", err)
	}
}
