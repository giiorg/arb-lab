package observability_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// TestNopMeterValidatesIdenticallyToRegistry is the claim NopMeter exists for.
//
// The obvious no-op meter accepts everything and returns nil, which makes every
// test written against it vacuous: a descriptor production would refuse passes
// under test, and the failure arrives at boot in the process that matters. So
// the two meters are driven through the same refusals and must agree on all of
// them.
func TestNopMeterValidatesIdenticallyToRegistry(t *testing.T) {
	t.Parallel()

	badSpecs := []struct {
		name string
		spec observability.Spec
	}{
		{"zero value", observability.Spec{}},
		{"no help", observability.Spec{
			Name: "arblab_x_total", Kind: observability.KindCounter, Unit: observability.UnitCount,
		}},
		{"missing counter suffix", withName(countSpec(), "arblab_fills")},
		{"undeclared label set", withLabels(countSpec(), observability.Label{Name: "venue"})},
		{"cardinality bound exceeded", withLabels(countSpec(), observability.Label{
			Name: "order", Values: manyValues(observability.MaxLabelValues + 1),
		})},
	}

	for _, tc := range badSpecs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, nopErr := observability.NopMeter{}.Counter(tc.spec)
			_, regErr := observability.NewRegistry().Counter(tc.spec)

			if nopErr == nil {
				t.Fatalf("NopMeter accepted %+v; a no-op meter that accepts everything makes every test vacuous", tc.spec)
			}
			if regErr == nil {
				t.Fatalf("Registry accepted %+v", tc.spec)
			}
			// Identical messages, not merely both non-nil: a test asserting on
			// an error string must not pass against one meter and fail against
			// the other.
			if nopErr.Error() != regErr.Error() {
				t.Errorf("NopMeter said %q; Registry said %q — the two meters have drifted", nopErr, regErr)
			}
		})
	}
}

func TestNopMeterEnforcesRecordTimeRules(t *testing.T) {
	t.Parallel()

	c, err := observability.NopMeter{}.Counter(venueSpec())
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}

	if err := c.Inc("kraken", "buy"); !errors.Is(err, observability.ErrLabelValueUndeclared) {
		t.Errorf("err = %v; want the undeclared value refused by the no-op meter too", err)
	}
	if err := c.Inc("binance"); !errors.Is(err, observability.ErrLabelArity) {
		t.Errorf("err = %v; want ErrLabelArity", err)
	}
	if err := c.Add(observability.Count(-1), "binance", "buy"); !errors.Is(err, observability.ErrCounterDelta) {
		t.Errorf("err = %v; want ErrCounterDelta", err)
	}

	// Positive control: a valid observation is accepted and simply discarded.
	// Without it, the three refusals above would be satisfied by a meter that
	// objects to everything.
	if err := c.Inc("binance", "buy"); err != nil {
		t.Errorf("a valid observation was refused: %v", err)
	}
}

func TestNopMeterServesAllThreeKinds(t *testing.T) {
	t.Parallel()

	m := observability.NopMeter{}

	gaugeSpec := observability.Spec{
		Name: "arblab_open_positions", Help: "h",
		Kind: observability.KindGauge, Unit: observability.UnitCount,
	}
	g, err := m.Gauge(gaugeSpec)
	if err != nil {
		t.Fatalf("Gauge: %v", err)
	}
	if err := g.Set(observability.Count(2)); err != nil {
		t.Errorf("Set: %v", err)
	}
	if err := g.Set(observability.Bytes(2)); !errors.Is(err, observability.ErrUnitMismatch) {
		t.Errorf("err = %v; want ErrUnitMismatch", err)
	}

	histSpec := observability.Spec{
		Name: "arblab_rtt_seconds", Help: "h",
		Kind: observability.KindHistogram, Unit: observability.UnitSeconds,
		Buckets: []observability.Sample{observability.Seconds(1), observability.Seconds(2)},
	}
	h, err := m.Histogram(histSpec)
	if err != nil {
		t.Fatalf("Histogram: %v", err)
	}
	if err := h.Observe(observability.Seconds(1)); err != nil {
		t.Errorf("Observe: %v", err)
	}

	// And the kind check binds on the no-op meter as well.
	if _, err := m.Gauge(histSpec); !errors.Is(err, observability.ErrKindMismatch) {
		t.Errorf("err = %v; want ErrKindMismatch", err)
	}
}

// TestMetersSatisfyTheMeterPort is a compile-time claim: both implementations
// are substitutable, which is the whole reason the port exists.
func TestMetersSatisfyTheMeterPort(t *testing.T) {
	t.Parallel()

	var meters = []observability.Meter{
		observability.NopMeter{},
		observability.NewRegistry(),
	}
	for _, m := range meters {
		if _, err := m.Counter(countSpec()); err != nil {
			t.Errorf("%T rejected a valid descriptor: %v", m, err)
		}
	}
}
