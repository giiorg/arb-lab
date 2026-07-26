package observability_test

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// venueSpec carries TWO labels on purpose. A single-label fixture makes the
// arity check and the key-separator collision unreachable — both distinctions
// collapse when there is only one value to join.
func venueSpec() observability.Spec {
	return observability.Spec{
		Name: "arblab_fills_total",
		Help: "fills observed",
		Kind: observability.KindCounter,
		Unit: observability.UnitCount,
		Labels: []observability.Label{
			{Name: "venue", Values: []string{"binance", "bybit"}},
			{Name: "side", Values: []string{"buy", "sell"}},
		},
	}
}

func TestCounterAccumulatesPerLabelTuple(t *testing.T) {
	t.Parallel()

	r := observability.NewRegistry()
	c, err := r.Counter(venueSpec())
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}

	mustNil(t, c.Inc("binance", "buy"))
	mustNil(t, c.Add(observability.Count(4), "binance", "buy"))
	mustNil(t, c.Inc("bybit", "sell"))

	got := r.Snapshot()
	if len(got) != 2 {
		t.Fatalf("got %d series; want 2", len(got))
	}
	if v := got[0].Value.Mantissa(); v != 5 {
		t.Errorf("binance/buy = %d; want 5", v)
	}
	if v := got[1].Value.Mantissa(); v != 1 {
		t.Errorf("bybit/sell = %d; want 1", v)
	}
	if r.Rejected() != 0 {
		t.Errorf("Rejected() = %d; want 0 on the happy path", r.Rejected())
	}
}

// TestSeriesKeyDoesNotCollide is why the key separator is a character no label
// value holds.
//
// # The fixture has to contain the separator, or it proves nothing
//
// The obvious version of this test uses ("ab","c") and ("a","bc"). Those collide
// only under the empty separator, so it passes against every wrong choice a
// person would actually make. Label VALUES are not character-checked — only
// names are — so a value legitimately holds `_`, `.`, `-` and `:`, and each of
// those is a plausible separator. The table drives one colliding tuple pair per
// candidate: under separator c, ("a", "b"+c+"z") and ("a"+c+"b", "z") join to
// the same string.
//
// The honest limit: this cannot rule out an exotic separator nobody would pick.
// It rules out every ordinary one.
func TestSeriesKeyDoesNotCollide(t *testing.T) {
	t.Parallel()

	for _, sep := range []string{"_", ".", "-", ":", "/", "|", ",", " "} {
		t.Run("separator "+sep, func(t *testing.T) {
			t.Parallel()

			left, right := "a"+sep+"b", "b"+sep+"z"
			spec := observability.Spec{
				Name: "arblab_x_total", Help: "h",
				Kind: observability.KindCounter, Unit: observability.UnitCount,
				Labels: []observability.Label{
					{Name: "a", Values: []string{"a", left}},
					{Name: "b", Values: []string{right, "z"}},
				},
			}
			r := observability.NewRegistry()
			c, err := r.Counter(spec)
			if err != nil {
				t.Fatalf("Counter: %v", err)
			}

			mustNil(t, c.Add(observability.Count(1), "a", right))
			mustNil(t, c.Add(observability.Count(10), left, "z"))

			got := r.Snapshot()
			if len(got) != 2 {
				t.Fatalf("got %d series; want 2 — the tuples collided on one key under %q", len(got), sep)
			}
			for _, s := range got {
				if v := s.Value.Mantissa(); v != 1 && v != 10 {
					t.Errorf("series %v = %d; want the two counts kept apart", s.LabelValues, v)
				}
			}
		})
	}
}

func TestRecordTimeRefusals(t *testing.T) {
	t.Parallel()

	usdGauge := observability.Spec{
		Name: "arblab_pnl_usd", Help: "h",
		Kind: observability.KindGauge, Unit: observability.UnitMoney,
		Asset: fixedpoint.Asset("USD"),
	}

	tests := []struct {
		name string
		run  func(t *testing.T, r *observability.Registry) error
		want error
	}{
		{
			name: "label arity short",
			run: func(t *testing.T, r *observability.Registry) error {
				return counterOf(t, r, venueSpec()).Inc("binance")
			},
			want: observability.ErrLabelArity,
		},
		{
			name: "label arity long",
			run: func(t *testing.T, r *observability.Registry) error {
				return counterOf(t, r, venueSpec()).Inc("binance", "buy", "extra")
			},
			want: observability.ErrLabelArity,
		},
		{
			name: "undeclared label value in the first position",
			run: func(t *testing.T, r *observability.Registry) error {
				return counterOf(t, r, venueSpec()).Inc("kraken", "buy")
			},
			want: observability.ErrLabelValueUndeclared,
		},
		{
			// The second position too. A checker that only validated values[0]
			// would pass the case above and admit unbounded cardinality on
			// every other label.
			name: "undeclared label value in a later position",
			run: func(t *testing.T, r *observability.Registry) error {
				return counterOf(t, r, venueSpec()).Inc("binance", "hold")
			},
			want: observability.ErrLabelValueUndeclared,
		},
		{
			name: "negative counter delta",
			run: func(t *testing.T, r *observability.Registry) error {
				return counterOf(t, r, venueSpec()).Add(observability.Count(-1), "binance", "buy")
			},
			want: observability.ErrCounterDelta,
		},
		{
			name: "Inc on a non-count unit",
			run: func(t *testing.T, r *observability.Registry) error {
				spec := observability.Spec{
					Name: "arblab_funding_usd_total", Help: "h",
					Kind: observability.KindCounter, Unit: observability.UnitMoney,
					Asset: fixedpoint.Asset("USD"),
				}
				return counterOf(t, r, spec).Inc()
			},
			want: observability.ErrCounterUnit,
		},
		{
			name: "unit mismatch",
			run: func(t *testing.T, r *observability.Registry) error {
				return gaugeOf(t, r, usdGauge).Set(observability.Count(1))
			},
			want: observability.ErrUnitMismatch,
		},
		{
			// Neither the compiler nor the unit check catches this: BTC-Money
			// and USD-Money are the same Go type at the same unit. It is wrong
			// by a factor of the BTC price with nothing failing.
			name: "asset mismatch",
			run: func(t *testing.T, r *observability.Registry) error {
				s, err := observability.MoneyOf(mustMoney(t, "BTC", "1"))
				if err != nil {
					t.Fatalf("MoneyOf: %v", err)
				}
				return gaugeOf(t, r, usdGauge).Set(s)
			},
			want: observability.ErrAssetMismatch,
		},
		{
			name: "unconstructed sample",
			run: func(t *testing.T, r *observability.Registry) error {
				return gaugeOf(t, r, usdGauge).Set(observability.Sample{})
			},
			want: observability.ErrSampleUnset,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := observability.NewRegistry()
			err := tc.run(t, r)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v; want %v", err, tc.want)
			}
			// Every refusal is counted. This is the package's only evidence
			// that anything was lost: a refused observation is invisible in the
			// series it was meant for, and a flat line reads as a quiet system.
			if got := r.Rejected(); got != 1 {
				t.Errorf("Rejected() = %d; want 1", got)
			}
			// And nothing was partially applied.
			if got := r.Snapshot(); len(got) != 0 {
				t.Errorf("Snapshot() has %d series; a refused observation must record nothing", len(got))
			}
		})
	}
}

func TestCounterRefusesOverflow(t *testing.T) {
	t.Parallel()

	spec := observability.Spec{
		Name: "arblab_big_total", Help: "h",
		Kind: observability.KindCounter, Unit: observability.UnitCount,
	}
	r := observability.NewRegistry()
	c := counterOf(t, r, spec)

	mustNil(t, c.Add(observability.Count(math.MaxInt64)))
	if err := c.Add(observability.Count(1)); !errors.Is(err, observability.ErrValueOverflow) {
		t.Fatalf("err = %v; want ErrValueOverflow", err)
	}
	// The value is unchanged, not wrapped. A wrapped int64 goes NEGATIVE, which
	// every rate() reads as a process restart.
	if got := r.Snapshot()[0].Value.Mantissa(); got != math.MaxInt64 {
		t.Errorf("value = %d; want it left at MaxInt64", got)
	}
}

func TestHistogramBucketsAreCumulative(t *testing.T) {
	t.Parallel()

	sec := func(n int64) observability.Sample { return observability.Seconds(time.Duration(n) * time.Second) }
	spec := observability.Spec{
		Name: "arblab_rtt_seconds", Help: "h",
		Kind: observability.KindHistogram, Unit: observability.UnitSeconds,
		Buckets: []observability.Sample{sec(1), sec(2), sec(5)},
	}
	r := observability.NewRegistry()
	h, err := r.Histogram(spec)
	if err != nil {
		t.Fatalf("Histogram: %v", err)
	}

	// One observation in each region, INCLUDING one above every bound. Without
	// the last, the +Inf overflow slot is never exercised and a histogram whose
	// buckets stop summing to its count looks correct.
	for _, d := range []int64{1, 2, 3, 9} {
		mustNil(t, h.Observe(sec(d)))
	}

	s := r.Snapshot()[0]
	if s.Count != 4 {
		t.Fatalf("Count = %d; want 4", s.Count)
	}
	// Cumulative: le=1 → {1}; le=2 → {1,2}; le=5 → {1,2,3}; +Inf → all four.
	want := []uint64{1, 2, 3, 4}
	if len(s.Buckets) != len(want) {
		t.Fatalf("got %d buckets; want %d (declared + the +Inf overflow)", len(s.Buckets), len(want))
	}
	for i := range want {
		if s.Buckets[i] != want[i] {
			t.Errorf("bucket %d = %d; want %d (buckets are cumulative)", i, s.Buckets[i], want[i])
		}
	}
	if got, want := s.Sum.Mantissa(), int64(15_000_000_000); got != want {
		t.Errorf("Sum = %d; want %d", got, want)
	}
	// The last cumulative bucket must equal the count, or quantiles read off a
	// curve that does not reach 1.
	if s.Buckets[len(s.Buckets)-1] != s.Count {
		t.Errorf("the +Inf bucket (%d) does not equal Count (%d)", s.Buckets[len(s.Buckets)-1], s.Count)
	}
}

func TestHistogramAcceptsNegativeObservations(t *testing.T) {
	t.Parallel()

	sec := func(n int64) observability.Sample { return observability.Seconds(time.Duration(n) * time.Second) }
	spec := observability.Spec{
		Name: "arblab_skew_seconds", Help: "h",
		Kind: observability.KindHistogram, Unit: observability.UnitSeconds,
		Buckets: []observability.Sample{sec(-1), sec(0), sec(1)},
	}
	r := observability.NewRegistry()
	h, err := r.Histogram(spec)
	if err != nil {
		t.Fatalf("Histogram: %v", err)
	}

	// Cross-leg skew is signed, and forcing an absolute value would discard
	// which leg is ahead.
	mustNil(t, h.Observe(sec(-2)))
	s := r.Snapshot()[0]
	if s.Buckets[0] != 1 || s.Sum.Mantissa() != -2_000_000_000 {
		t.Errorf("buckets = %v, sum = %d; want the negative observation in the first bucket",
			s.Buckets, s.Sum.Mantissa())
	}
}

// TestHistogramRefusesSumUnderflow reaches the branch a counter cannot.
//
// The overflow guard has two sides, and the counter only ever exercises one:
// Counter.Add refuses a negative delta, so a counter's accumulator never moves
// down. A histogram's sum does — negative observations are permitted, because
// cross-leg skew is signed — which makes the underflow side reachable and makes
// testing only the overflow side a half-checked guard on a symmetric predicate.
func TestHistogramRefusesSumUnderflow(t *testing.T) {
	t.Parallel()

	sec := func(d time.Duration) observability.Sample { return observability.Seconds(d) }
	spec := observability.Spec{
		Name: "arblab_skew_seconds", Help: "h",
		Kind: observability.KindHistogram, Unit: observability.UnitSeconds,
		Buckets: []observability.Sample{sec(-1), sec(0), sec(1)},
	}
	r := observability.NewRegistry()
	h, err := r.Histogram(spec)
	if err != nil {
		t.Fatalf("Histogram: %v", err)
	}

	mustNil(t, h.Observe(sec(time.Duration(math.MinInt64))))
	if err := h.Observe(sec(-1)); !errors.Is(err, observability.ErrValueOverflow) {
		t.Fatalf("err = %v; want ErrValueOverflow on the underflow side", err)
	}

	// Refused cleanly: the sum is unchanged and neither the count nor a bucket
	// moved, so a rejected observation cannot leave the histogram's cumulative
	// buckets disagreeing with its count.
	s := r.Snapshot()[0]
	if s.Sum.Mantissa() != math.MinInt64 {
		t.Errorf("Sum = %d; want it left at MinInt64", s.Sum.Mantissa())
	}
	if s.Count != 1 {
		t.Errorf("Count = %d; want 1", s.Count)
	}
	if s.Buckets[len(s.Buckets)-1] != s.Count {
		t.Errorf("the +Inf bucket (%d) does not equal Count (%d)", s.Buckets[len(s.Buckets)-1], s.Count)
	}
	if r.Rejected() != 1 {
		t.Errorf("Rejected() = %d; want 1", r.Rejected())
	}
}

func TestRegisteringTheSameNameTwice(t *testing.T) {
	t.Parallel()

	r := observability.NewRegistry()

	// The identical descriptor is fine and shares the series: two call sites
	// instrumenting one thing should not race to own it.
	a := counterOf(t, r, venueSpec())
	b := counterOf(t, r, venueSpec())
	mustNil(t, a.Inc("binance", "buy"))
	mustNil(t, b.Inc("binance", "buy"))
	if got := r.Snapshot(); len(got) != 1 || got[0].Value.Mantissa() != 2 {
		t.Errorf("snapshot = %v; want one shared series at 2", got)
	}

	// A different descriptor under the same name is not, and the winner would
	// otherwise be whichever package initialised first.
	diff := venueSpec()
	diff.Help = "something else"
	if _, err := r.Counter(diff); !errors.Is(err, observability.ErrMetricDuplicate) {
		t.Fatalf("err = %v; want ErrMetricDuplicate", err)
	}

	// Including a difference only in the label declarations, which is where a
	// shallow equality check would let two disagreeing cardinality bounds
	// through.
	relabelled := venueSpec()
	relabelled.Labels[0].Values = []string{"binance", "bybit", "okx"}
	if _, err := r.Counter(relabelled); !errors.Is(err, observability.ErrMetricDuplicate) {
		t.Fatalf("err = %v; want ErrMetricDuplicate on a changed value set", err)
	}
}

func TestKindMismatch(t *testing.T) {
	t.Parallel()

	r := observability.NewRegistry()
	if _, err := r.Gauge(venueSpec()); !errors.Is(err, observability.ErrKindMismatch) {
		t.Fatalf("err = %v; want ErrKindMismatch", err)
	}
	if _, err := r.Histogram(venueSpec()); !errors.Is(err, observability.ErrKindMismatch) {
		t.Fatalf("err = %v; want ErrKindMismatch", err)
	}
}

// TestSnapshotIsDeterministic guards against an assertion that passes on
// alternate runs. Go randomizes map iteration per range, so a snapshot in map
// order would flake in a way that reads as a bug in the code under test.
func TestSnapshotIsDeterministic(t *testing.T) {
	t.Parallel()

	r := observability.NewRegistry()
	c := counterOf(t, r, venueSpec())
	for _, v := range []string{"bybit", "binance"} {
		for _, s := range []string{"sell", "buy"} {
			mustNil(t, c.Inc(v, s))
		}
	}

	first := r.Snapshot()
	for range 20 {
		got := r.Snapshot()
		for i := range got {
			if len(got) != len(first) || !equalSlices(got[i].LabelValues, first[i].LabelValues) {
				t.Fatalf("snapshot order changed: %v then %v", first, got)
			}
		}
	}
	want := [][]string{
		{"binance", "buy"}, {"binance", "sell"}, {"bybit", "buy"}, {"bybit", "sell"},
	}
	for i := range want {
		if !equalSlices(first[i].LabelValues, want[i]) {
			t.Errorf("series %d = %v; want %v", i, first[i].LabelValues, want[i])
		}
	}
}

func TestRegistryIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	r := observability.NewRegistry()
	const goroutines, each = 8, 200

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Every goroutine registers as well as records, so the
			// register-vs-record race is exercised and not just the record one.
			c, err := r.Counter(venueSpec())
			if err != nil {
				t.Errorf("Counter: %v", err)
				return
			}
			side := []string{"buy", "sell"}[g%2]
			for range each {
				_ = c.Inc("binance", side)
			}
		}()
	}
	wg.Wait()

	var total int64
	for _, s := range r.Snapshot() {
		total += s.Value.Mantissa()
	}
	if want := int64(goroutines * each); total != want {
		t.Errorf("total = %d; want %d — increments were lost", total, want)
	}
}

// TestUnboundedLabelAcceptsAnyNonEmptyValue is the escape working as declared.
// Without it, the refusal tests above would be satisfied by a package that
// refused every label value.
func TestUnboundedLabelAcceptsAnyNonEmptyValue(t *testing.T) {
	t.Parallel()

	spec := observability.Spec{
		Name: "arblab_venue_errors_total", Help: "h",
		Kind: observability.KindCounter, Unit: observability.UnitCount,
		Labels: []observability.Label{
			{Name: "code", Unbounded: "venue error codes are an open set"},
		},
	}
	r := observability.NewRegistry()
	c := counterOf(t, r, spec)

	mustNil(t, c.Inc("-1021"))
	mustNil(t, c.Inc("10001"))
	if got := len(r.Snapshot()); got != 2 {
		t.Errorf("got %d series; want 2", got)
	}
	// The empty string is still refused: it is what a caller passes when the
	// value was never populated, and it would silently merge every unlabelled
	// observation into one series.
	if err := c.Inc(""); !errors.Is(err, observability.ErrLabelValueUndeclared) {
		t.Errorf("err = %v; want the empty value refused even on an unbounded label", err)
	}
}

func counterOf(t *testing.T, r *observability.Registry, s observability.Spec) observability.Counter {
	t.Helper()

	c, err := r.Counter(s)
	if err != nil {
		t.Fatalf("Counter(%s): %v", s.Name, err)
	}
	return c
}

func gaugeOf(t *testing.T, r *observability.Registry, s observability.Spec) observability.Gauge {
	t.Helper()

	g, err := r.Gauge(s)
	if err != nil {
		t.Fatalf("Gauge(%s): %v", s.Name, err)
	}
	return g
}

func mustNil(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
