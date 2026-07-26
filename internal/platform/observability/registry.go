package observability

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// Registry is an in-process Meter that retains what was recorded.
//
// It is both the implementation a process runs with before an exporter exists
// and the oracle every test in this package asserts against — the second use is
// why it retains values at all. A meter that only forwarded would leave every
// claim in this package checkable solely through whatever backend it forwarded
// to, which is how a metrics layer ends up with no tests of its own.
//
// # It is not a Prometheus exporter, deliberately
//
// build-spec §11 names prometheus/client_golang, and §3.6 rule 9 scopes
// kernelpurity to in-module imports specifically so this package may take it.
// The adapter is still deferred, on two grounds. Nothing under internal/ emits a
// metric yet, so a registered exporter would be wiring that no test can exercise
// end to end — the same reason hexboundary and the outbox store are deferred.
// And WHICH binary exposes /metrics on WHICH interface is a composition-root
// decision: §11 gives arblab-actuator no HTTP surface at all, so a kernel that
// mounted a handler would be wrong there and could not be talked out of it.
//
// What is NOT deferred is the conversion to the wire's float64 (export.go),
// because that belongs to metrics rather than to any one client library.
//
// Safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	metrics map[string]*metric

	// rejected counts every observation refused at record time. It is this
	// package's mirror of events.ObservabilityLane.Dropped: the only evidence
	// that anything was lost, exact rather than approximate, and never reset —
	// a rate is the caller's to derive, and a counter that resets cannot answer
	// "did we lose anything during that incident" afterwards.
	rejected atomic.Uint64
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{metrics: make(map[string]*metric)}
}

// Rejected reports how many observations this registry has refused,
// cumulatively — bad label values, unit or asset mismatches, arity mismatches
// and negative counter deltas.
//
// The number a "our instrumentation is lying to us" alarm watches. A refused
// observation is invisible in the series it was meant for: the panel shows a
// flat line, which reads as a quiet system.
func (r *Registry) Rejected() uint64 { return r.rejected.Load() }

// Counter returns a counter handle for s.
func (r *Registry) Counter(s Spec) (Counter, error) {
	m, err := r.register(s, KindCounter)
	if err != nil {
		return nil, err
	}
	return counterHandle{m}, nil
}

// Gauge returns a gauge handle for s.
func (r *Registry) Gauge(s Spec) (Gauge, error) {
	m, err := r.register(s, KindGauge)
	if err != nil {
		return nil, err
	}
	return gaugeHandle{m}, nil
}

// Histogram returns a histogram handle for s.
func (r *Registry) Histogram(s Spec) (Histogram, error) {
	m, err := r.register(s, KindHistogram)
	if err != nil {
		return nil, err
	}
	return histogramHandle{m}, nil
}

// register validates s, checks it against the requested kind, and returns the
// metric — creating it or returning the existing one.
func (r *Registry) register(s Spec, want Kind) (*metric, error) {
	if err := validateFor(s, want); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.metrics[s.Name]; ok {
		// Re-registering the identical descriptor is normal: two call sites
		// instrumenting one thing should share a series rather than race to
		// own it. Re-registering a DIFFERENT descriptor under one name is not,
		// and the winner would otherwise be whichever package initialised
		// first.
		if !existing.spec.equal(s) {
			return nil, fmt.Errorf("metric %q: %w", s.Name, ErrMetricDuplicate)
		}
		return existing, nil
	}

	m := &metric{spec: s, series: make(map[string]*seriesValue), registry: r}
	r.metrics[s.Name] = m
	return m, nil
}

// metric is one registered descriptor and every series recorded under it.
type metric struct {
	spec     Spec
	registry *Registry

	mu     sync.Mutex
	series map[string]*seriesValue
}

// seriesValue holds one label tuple's state. Which fields are meaningful
// depends on the metric's kind: value for a counter and a gauge, count/sum/
// buckets for a histogram.
type seriesValue struct {
	labels []string

	value int64

	count   uint64
	sum     int64
	buckets []uint64
}

// reject counts a refused observation and returns the error unchanged, so that
// every refusal path is one line and none can forget the counter.
func (m *metric) reject(err error) error {
	m.registry.rejected.Add(1)
	return err
}

// prepare validates a sample and label values against the descriptor, returning
// the series key.
func (m *metric) prepare(v Sample, labelValues []string) (string, error) {
	if err := v.matches(m.spec.Unit, m.spec.Asset); err != nil {
		return "", m.reject(newSampleError(m.spec, err))
	}
	key, err := checkLabels(m.spec, labelValues)
	if err != nil {
		return "", m.reject(err)
	}
	return key, nil
}

// at returns the series for key, creating it on first use.
// Caller holds m.mu.
func (m *metric) at(key string, labelValues []string) *seriesValue {
	if s, ok := m.series[key]; ok {
		return s
	}
	s := &seriesValue{labels: append([]string(nil), labelValues...)}
	if m.spec.Kind == KindHistogram {
		// One slot per declared bucket plus the implicit +Inf overflow bucket.
		// The overflow slot is not optional: without it an observation above
		// every bound is counted in `count` and in no bucket, so the cumulative
		// buckets stop summing to the count and every quantile derived from
		// them is silently biased low.
		s.buckets = make([]uint64, len(m.spec.Buckets)+1)
	}
	m.series[key] = s
	return s
}

type counterHandle struct{ m *metric }

func (h counterHandle) Add(delta Sample, labelValues ...string) error {
	if delta.Sign() < 0 {
		// Checked before the unit match so that the message names the actual
		// problem. A negative money delta against a money counter is a counter
		// misuse, not a typing mistake, and reporting a unit error would send
		// the reader to the wrong question.
		if err := delta.Validate(); err == nil {
			return h.m.reject(newCounterDeltaError(h.m.spec, delta))
		}
	}
	key, err := h.m.prepare(delta, labelValues)
	if err != nil {
		return err
	}

	h.m.mu.Lock()
	defer h.m.mu.Unlock()

	s := h.m.at(key, labelValues)
	sum, err := addChecked(s.value, delta.Mantissa())
	if err != nil {
		return h.m.reject(fmt.Errorf("metric %q: %w", h.m.spec.Name, err))
	}
	s.value = sum
	return nil
}

func (h counterHandle) Inc(labelValues ...string) error {
	if h.m.spec.Unit != UnitCount {
		return h.m.reject(newCounterUnitError(h.m.spec))
	}
	return h.Add(Count(1), labelValues...)
}

type gaugeHandle struct{ m *metric }

func (h gaugeHandle) Set(v Sample, labelValues ...string) error {
	key, err := h.m.prepare(v, labelValues)
	if err != nil {
		return err
	}

	h.m.mu.Lock()
	defer h.m.mu.Unlock()

	h.m.at(key, labelValues).value = v.Mantissa()
	return nil
}

type histogramHandle struct{ m *metric }

func (h histogramHandle) Observe(v Sample, labelValues ...string) error {
	key, err := h.m.prepare(v, labelValues)
	if err != nil {
		return err
	}

	h.m.mu.Lock()
	defer h.m.mu.Unlock()

	s := h.m.at(key, labelValues)
	sum, err := addChecked(s.sum, v.Mantissa())
	if err != nil {
		return h.m.reject(fmt.Errorf("metric %q: %w", h.m.spec.Name, err))
	}

	// Cumulative, as the exposition format defines them: an observation counts
	// in its own bucket and in every wider one. Incrementing only the first
	// matching bucket is the classic mistake, and it produces a histogram whose
	// buckets look like a density rather than a distribution function — every
	// quantile then reads off the wrong curve without anything failing.
	i := sort.Search(len(h.m.spec.Buckets), func(i int) bool {
		return v.Mantissa() <= h.m.spec.Buckets[i].Mantissa()
	})
	for j := i; j < len(s.buckets); j++ {
		s.buckets[j]++
	}

	s.sum = sum
	s.count++
	return nil
}

// addChecked adds two mantissas, refusing an int64 overflow.
//
// Silent wrap is the failure this exists to stop, and it is the same failure the
// outbox backoff guards against by shifting on the right-hand side: a wrapped
// int64 does not become large, it becomes NEGATIVE. A counter that wraps
// therefore reports a decrease, every rate() over it re-bases as if the process
// restarted, and the series looks healthy while carrying no information.
func addChecked(a, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, fmt.Errorf("%d + %d exceeds int64: %w", a, b, ErrValueOverflow)
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, fmt.Errorf("%d + %d underflows int64: %w", a, b, ErrValueOverflow)
	}
	return a + b, nil
}

// Series is one recorded label tuple, as Snapshot reports it.
type Series struct {
	Name string
	Spec Spec

	// LabelValues is in the descriptor's declared label order.
	LabelValues []string

	// Value is the current value of a counter or a gauge, in the metric's unit.
	Value Sample

	// Count, Sum and Buckets describe a histogram. Buckets is cumulative and
	// one longer than the descriptor's, the last entry being the +Inf overflow.
	Count   uint64
	Sum     Sample
	Buckets []uint64
}

// Snapshot returns every recorded series, ordered by name then by label values,
// so that a test can compare against a literal without sorting.
//
// The determinism is the point: Go map iteration is randomized per range, so a
// snapshot in map order would make an assertion over two series pass and fail on
// alternate runs — a flake that reads as a concurrency bug in the code under
// test rather than in the assertion.
func (r *Registry) Snapshot() []Series {
	r.mu.Lock()
	metrics := make([]*metric, 0, len(r.metrics))
	for _, m := range r.metrics {
		metrics = append(metrics, m)
	}
	r.mu.Unlock()

	var out []Series
	for _, m := range metrics {
		m.mu.Lock()
		for _, s := range m.series {
			out = append(out, m.seriesOf(s))
		}
		m.mu.Unlock()
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return lessStrings(out[i].LabelValues, out[j].LabelValues)
	})
	return out
}

// seriesOf converts stored state into a reported Series.
// Caller holds m.mu.
func (m *metric) seriesOf(s *seriesValue) Series {
	out := Series{
		Name:        m.spec.Name,
		Spec:        m.spec,
		LabelValues: append([]string(nil), s.labels...),
	}
	sample := func(mantissa int64) Sample {
		return Sample{mantissa: mantissa, unit: m.spec.Unit, asset: m.spec.Asset}
	}
	if m.spec.Kind == KindHistogram {
		out.Count = s.count
		out.Sum = sample(s.sum)
		out.Buckets = append([]uint64(nil), s.buckets...)
		return out
	}
	out.Value = sample(s.value)
	return out
}

func lessStrings(a, b []string) bool {
	for i := range a {
		if i >= len(b) {
			return false
		}
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
