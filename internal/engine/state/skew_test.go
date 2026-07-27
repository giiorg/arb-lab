package state_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

// base is an arbitrary non-zero instant. Fixed rather than read from a clock:
// §3.6 rule 5 bans ambient time in engine/, and a skew is a pure function of two
// watermarks anyway — Skew takes no clock precisely so that a test needs none.
var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

// at builds a watermark delivered d after base.
//
// SourceTime is deliberately offset from RecvTime by a DIFFERENT amount than any
// test's expected skew. If the two timestamps moved together, a Skew that read
// SourceTime — the implementation this package spends a doc comment rejecting —
// would produce the same answer as the one that reads RecvTime, and every test
// below would pass against the forbidden version.
func at(d time.Duration) domain.Watermark {
	return domain.Watermark{
		VenueSeq:   1,
		SourceTime: base.Add(d).Add(-37 * time.Second),
		RecvTime:   base.Add(d),
	}
}

func TestSkewMeasuresReceiveTimeSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ws   []domain.Watermark
		want time.Duration
	}{
		{
			name: "two legs",
			ws:   []domain.Watermark{at(0), at(250 * time.Millisecond)},
			want: 250 * time.Millisecond,
		},
		{
			// The extremes are NOT the first and last elements. With only two
			// legs, "max minus min", "last minus first" and "absolute difference"
			// are the same computation, so a two-leg fixture cannot tell a
			// span-finder from a subtraction of the endpoints. Three legs with
			// the widest pair in the interior is what separates them.
			name: "three legs, widest pair in the interior",
			ws: []domain.Watermark{
				at(400 * time.Millisecond),
				at(900 * time.Millisecond),
				at(100 * time.Millisecond),
			},
			want: 800 * time.Millisecond,
		},
		{
			// Descending order: catches a Before/After swap that ascending input
			// would hide, since a swapped pair of reductions over sorted input
			// still brackets the same span.
			name: "descending arrival",
			ws:   []domain.Watermark{at(time.Second), at(500 * time.Millisecond), at(0)},
			want: time.Second,
		},
		{
			// Identical instants. This is the clock.TestClock case — a backtest
			// advances time only when the harness does, so legs dispatched
			// between advances share a RecvTime exactly. Zero is the right answer
			// HERE, where every leg was compared; it is the wrong answer when
			// nothing was, which is the case below.
			name: "simultaneous legs",
			ws:   []domain.Watermark{at(0), at(0)},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := state.Skew(tc.ws...)
			if err != nil {
				t.Fatalf("Skew: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Skew = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSkewIgnoresSourceTime is the positive control for the choice skew.go
// argues for: two legs delivered at the same instant by venues whose clocks
// disagree by half a second have zero skew, not half a second of it.
//
// Without this, every fixture above would also pass against a SourceTime
// implementation, since `at` moves both stamps together — the helper's offset
// makes the two readings differ, and this makes one of them wrong on purpose.
func TestSkewIgnoresSourceTime(t *testing.T) {
	t.Parallel()

	sameArrival := base.Add(time.Second)
	binance := domain.Watermark{VenueSeq: 1, SourceTime: sameArrival, RecvTime: sameArrival}
	bybit := domain.Watermark{
		// The venue's own clock runs 500ms behind. §3.5: the two venues stamp
		// with unsynchronized clocks, so this difference is a property of the
		// clocks and not of the market.
		VenueSeq:   1,
		SourceTime: sameArrival.Add(-500 * time.Millisecond),
		RecvTime:   sameArrival,
	}

	got, err := state.Skew(binance, bybit)
	if err != nil {
		t.Fatalf("Skew: unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("Skew = %v, want 0 — a venue clock offset is not cross-leg skew", got)
	}
}

// TestSkewEqualsDifferenceOfAges checks the identity skew.go claims: skew over
// RecvTime is |Age(a) - Age(b)| for any single instant, since the instant
// cancels.
//
// It is the oracle for this function — a second, independent computation of the
// same quantity, through domain.Watermark.Age rather than through Skew's own
// arithmetic. Several instants, because an identity that holds at one `now` and
// not another is not the identity claimed.
func TestSkewEqualsDifferenceOfAges(t *testing.T) {
	t.Parallel()

	a, b := at(0), at(750*time.Millisecond)
	skew, err := state.Skew(a, b)
	if err != nil {
		t.Fatalf("Skew: unexpected error: %v", err)
	}

	for _, offset := range []time.Duration{time.Second, time.Minute, time.Hour} {
		now := base.Add(offset)
		ageA, err := a.Age(now)
		if err != nil {
			t.Fatalf("Age(a) at +%v: %v", offset, err)
		}
		ageB, err := b.Age(now)
		if err != nil {
			t.Fatalf("Age(b) at +%v: %v", offset, err)
		}
		want := ageA - ageB
		if want < 0 {
			want = -want
		}
		if skew != want {
			t.Errorf("at now=+%v: skew %v != |ageA-ageB| %v", offset, skew, want)
		}
	}
}

// TestSkewRefusesFewerThanTwo is the fold-identity guard: zero is the most
// permissive answer this function has, so it must not be what a caller who
// compared nothing receives.
//
// Both boundary cases are present. One leg is the one that matters — it is the
// realistic caller bug, and a `len(ws) < 1` check would admit it while still
// refusing the empty case.
func TestSkewRefusesFewerThanTwo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ws   []domain.Watermark
	}{
		{name: "no legs", ws: nil},
		{name: "one leg", ws: []domain.Watermark{at(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := state.Skew(tc.ws...)
			if !errors.Is(err, state.ErrSkewUndefined) {
				t.Fatalf("Skew = (%v, %v), want ErrSkewUndefined", got, err)
			}
		})
	}
}

// TestSkewRefusesUnsetWatermark: an unset RecvTime is the zero instant, so a
// skew computed against it is decades. That reads as a wildly skewed package
// rather than as the unpopulated field it is, and no limit admits it — so the
// refusal has to be here rather than at the comparison.
func TestSkewRefusesUnsetWatermark(t *testing.T) {
	t.Parallel()

	// The unset leg is SECOND, so a check that only ever looked at the first
	// element would pass.
	_, err := state.Skew(at(0), domain.Watermark{VenueSeq: 1, SourceTime: base})
	if err == nil {
		t.Fatal("Skew accepted a watermark with no receive time")
	}
	if errors.Is(err, state.ErrSkewUndefined) {
		t.Fatalf("got ErrSkewUndefined, want the watermark's own refusal: %v", err)
	}
}

func TestSkewLimitValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit state.SkewLimit
		want  bool // want an error
	}{
		// Zero is the case with the argument behind it: it admits only legs
		// delivered in the very same instant, which halts a live engine while
		// passing a backtest.
		{name: "zero", limit: 0, want: true},
		{name: "negative", limit: state.SkewLimit(-time.Nanosecond), want: true},
		{name: "one nanosecond", limit: state.SkewLimit(time.Nanosecond), want: false},
		{name: "typical", limit: state.SkewLimit(200 * time.Millisecond), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.limit.Validate()
			if got := err != nil; got != tc.want {
				t.Fatalf("Validate() error = %v, want error: %v", err, tc.want)
			}
			if tc.want && !errors.Is(err, state.ErrSkewLimitInvalid) {
				t.Errorf("error = %v, want ErrSkewLimitInvalid", err)
			}
		})
	}
}

// TestSkewLimitAdmits pins the inclusive boundary.
//
// The three cases around the limit are the point: exactly-at, one tick below,
// one tick above. A fixture set that only tested "well inside" and "well
// outside" would pass against `<` and `<=` alike, which is the collapse this
// repo has now found nine times.
func TestSkewLimitAdmits(t *testing.T) {
	t.Parallel()

	const limit = state.SkewLimit(200 * time.Millisecond)

	tests := []struct {
		name string
		skew time.Duration
		want bool
	}{
		{name: "zero skew", skew: 0, want: true},
		{name: "one tick below the limit", skew: 200*time.Millisecond - time.Nanosecond, want: true},
		{name: "exactly at the limit", skew: 200 * time.Millisecond, want: true},
		{name: "one tick past the limit", skew: 200*time.Millisecond + time.Nanosecond, want: false},
		{name: "far past the limit", skew: time.Hour, want: false},
		// A negative cannot come from Skew, which returns a max minus a min. One
		// arriving here came from a caller's own arithmetic, and it compares as
		// smaller than every limit — i.e. it reads as perfectly aligned, which is
		// the direction Watermark.Age refuses for the same reason.
		{name: "negative skew is refused, not admitted", skew: -time.Nanosecond, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := limit.Admits(tc.skew); got != tc.want {
				t.Errorf("Admits(%v) = %v, want %v", tc.skew, got, tc.want)
			}
		})
	}
}

func TestSkewLimitString(t *testing.T) {
	t.Parallel()

	if got, want := state.SkewLimit(200*time.Millisecond).String(), "200ms"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
