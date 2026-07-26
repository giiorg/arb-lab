package observability

// NopMeter is a Meter that discards observations but still validates them.
//
// # It validates, and that is the whole design decision
//
// The obvious no-op meter accepts everything and returns nil. It would be
// smaller, faster, and it would make every test that uses it vacuous: a
// descriptor production would refuse — a missing unit suffix, an undeclared
// label set, a bucket in the wrong unit — passes under test, and the failure
// arrives at boot in the process that matters. That is the same shape as a
// coverage number counted without checking what it reached, and this repo has
// paid for that reading three times.
//
// So NopMeter runs the identical validation path and differs only in retaining
// nothing. What it costs is a retained Spec per metric, which is bounded by the
// number of descriptors and not by the number of series — the growth term that
// matters is the second one.
//
// Its use is a component under test, or a binary that genuinely exports nothing
// — §11's arblab-actuator has no HTTP surface, so it has nowhere to expose a
// /metrics endpoint, and instrumenting it against a meter that rejected its
// descriptors would be worse than instrumenting it against one that drops them.
//
// Safe for concurrent use: it holds no mutable per-series state.
type NopMeter struct{}

// Counter returns a validating, discarding counter.
func (NopMeter) Counter(s Spec) (Counter, error) {
	if err := validateFor(s, KindCounter); err != nil {
		return nil, err
	}
	return nopCounter{s}, nil
}

// Gauge returns a validating, discarding gauge.
func (NopMeter) Gauge(s Spec) (Gauge, error) {
	if err := validateFor(s, KindGauge); err != nil {
		return nil, err
	}
	return nopRecorder{s}, nil
}

// Histogram returns a validating, discarding histogram.
func (NopMeter) Histogram(s Spec) (Histogram, error) {
	if err := validateFor(s, KindHistogram); err != nil {
		return nil, err
	}
	return nopRecorder{s}, nil
}

// validateFor is the construction-time check both meters share, so the two
// cannot drift into disagreeing about which descriptors are legal — which would
// be the vacuous-test failure arriving by a different route.
func validateFor(s Spec, want Kind) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Kind != want {
		return newKindMismatchError(s, want)
	}
	return nil
}

type nopCounter struct{ spec Spec }

func (c nopCounter) Add(delta Sample, labelValues ...string) error {
	if delta.Sign() < 0 {
		if err := delta.Validate(); err == nil {
			return newCounterDeltaError(c.spec, delta)
		}
	}
	return check(c.spec, delta, labelValues)
}

func (c nopCounter) Inc(labelValues ...string) error {
	if c.spec.Unit != UnitCount {
		return newCounterUnitError(c.spec)
	}
	return c.Add(Count(1), labelValues...)
}

// nopRecorder serves both the gauge and the histogram, whose discarding
// behaviour is identical — the difference between them is entirely in what a
// retaining meter stores.
type nopRecorder struct{ spec Spec }

func (r nopRecorder) Set(v Sample, labelValues ...string) error {
	return check(r.spec, v, labelValues)
}

func (r nopRecorder) Observe(v Sample, labelValues ...string) error {
	return check(r.spec, v, labelValues)
}

// check runs the record-time validation without recording.
//
// It does NOT count a rejection anywhere. A NopMeter has no Rejected() to read,
// and inventing a package-level counter for it would make a test's refusals
// accumulate into a global that another test then asserts on.
func check(s Spec, v Sample, labelValues []string) error {
	if err := v.matches(s.Unit, s.Asset); err != nil {
		return newSampleError(s, err)
	}
	_, err := checkLabels(s, labelValues)
	return err
}
