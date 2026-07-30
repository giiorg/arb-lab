package capture

import (
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// sourceLabels is the label dimension set §32j names, in declaration order.
// Every one is a closed set declared from the config at registration, so §10's
// default-deny label rule is satisfied by declaration and no Unbounded reason
// is needed anywhere in this file.
var sourceLabelNames = [...]string{"venue", "product", "symbol", "stream", "topic"}

// rootCauses is the closed set of gap causes this composition root records:
// the transport's two mapped outcomes plus the queue's overflow. The writer
// also mints torn_tail and process_restart gaps during open-time recovery;
// those are archive facts for verify, not live counters — they happen at most
// once per open, before this process was even capturing, and a counter here
// would double-report what the file already states.
var rootCauses = []string{
	string(enginecapture.CauseReconnect),
	string(enginecapture.CauseNoFirstData),
	string(enginecapture.CauseOverflow),
}

// metrics is the §32j capture metric set, registered once per run.
type metrics struct {
	records      observability.Counter
	dataRecords  observability.Counter
	writtenBytes observability.Counter
	gaps         observability.Counter
	drops        observability.Counter
	highWater    observability.Gauge
	dataAge      observability.Gauge
	consecFails  observability.Gauge
}

// newMetrics registers the capture metric set with label value sets drawn from
// the configuration: the only venues, symbols and topics these series can ever
// carry are the ones the operator wrote down.
func newMetrics(reg *observability.Registry, cfg Config) (*metrics, error) {
	labels := closedLabels(cfg)

	m := &metrics{}
	var err error
	if m.records, err = reg.Counter(observability.Spec{
		Name: "capture_records_total", Help: "Archive records written, all kinds.",
		Kind: observability.KindCounter, Unit: observability.UnitCount, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.dataRecords, err = reg.Counter(observability.Spec{
		Name: "capture_data_records_total", Help: "Venue frames archived. Zero while records_total grows is the dead-feed signature.",
		Kind: observability.KindCounter, Unit: observability.UnitCount, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.writtenBytes, err = reg.Counter(observability.Spec{
		Name: "capture_written_bytes_total", Help: "Bytes appended to the archive.",
		Kind: observability.KindCounter, Unit: observability.UnitBytes, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.gaps, err = reg.Counter(observability.Spec{
		Name: "capture_gaps_total", Help: "Gap records written by this process, by cause.",
		Kind: observability.KindCounter, Unit: observability.UnitCount,
		Labels: append(labels, observability.Label{Name: "cause", Values: rootCauses}),
	}); err != nil {
		return nil, err
	}
	if m.drops, err = reg.Counter(observability.Spec{
		Name: "capture_queue_drops_total", Help: "Frames dropped at the bounded queue (each is inside a recorded overflow gap).",
		Kind: observability.KindCounter, Unit: observability.UnitCount, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.highWater, err = reg.Gauge(observability.Spec{
		Name: "capture_queue_high_water", Help: "Deepest the bounded queue has been.",
		Kind: observability.KindGauge, Unit: observability.UnitCount, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.dataAge, err = reg.Gauge(observability.Spec{
		Name: "capture_last_data_age_seconds", Help: "Time since the last venue frame was archived.",
		Kind: observability.KindGauge, Unit: observability.UnitSeconds, Labels: labels,
	}); err != nil {
		return nil, err
	}
	if m.consecFails, err = reg.Gauge(observability.Spec{
		Name: "capture_stream_consecutive_failures", Help: "Sessions failed in a row without carrying data. Rising with zero data frames is a permanent misconfiguration.",
		Kind: observability.KindGauge, Unit: observability.UnitCount, Labels: labels,
	}); err != nil {
		return nil, err
	}
	return m, nil
}

// closedLabels builds the five declared label dimensions from BOTH source
// lists — an overlay row left out here is a stream whose every metric write
// is silently refused, with Registry.Rejected as the only witness (§37e).
func closedLabels(cfg Config) []observability.Label {
	sets := make([]map[string]struct{}, len(sourceLabelNames))
	for i := range sets {
		sets[i] = make(map[string]struct{})
	}
	for _, s := range cfg.Sources {
		for i, v := range labelValues(s) {
			sets[i][v] = struct{}{}
		}
	}
	for _, r := range cfg.RestSources {
		for i, v := range restLabelValues(r) {
			sets[i][v] = struct{}{}
		}
	}
	labels := make([]observability.Label, len(sourceLabelNames))
	for i, name := range sourceLabelNames {
		values := make([]string, 0, len(sets[i]))
		for v := range sets[i] {
			values = append(values, v)
		}
		labels[i] = observability.Label{Name: name, Values: values}
	}
	return labels
}

// labelValues is one source's label tuple, in sourceLabelNames order.
func labelValues(s SourceConfig) []string {
	return []string{
		string(s.Venue), string(s.Product), s.Symbol, string(s.Stream), s.Topic,
	}
}

// restLabelValues is one overlay row's tuple, same order, same dimensions —
// the overlay is a stream like any other to the metrics.
func restLabelValues(r RestSourceConfig) []string {
	return []string{
		string(r.Venue), string(r.Product), r.Symbol, string(r.Stream), r.Topic,
	}
}

// sourceMetrics is one pipeline's view: the shared handles plus this source's
// label tuple and the last exported snapshots, for counter deltas.
//
// Touched only by the pipeline's consumer goroutine, which is what makes the
// unguarded delta fields safe.
type sourceMetrics struct {
	m      *metrics
	clock  enginecapture.TimeSource
	labels []string

	lastWriter enginecapture.WriterStats
	lastQueue  enginecapture.QueueStats
	lastData   time.Time
}

func newSourceMetrics(m *metrics, clock enginecapture.TimeSource, labels []string) *sourceMetrics {
	return &sourceMetrics{m: m, clock: clock, labels: labels}
}

// Record-time errors below are deliberately discarded: every refusal is
// counted by Registry.Rejected, which is the "instrumentation is lying" alarm,
// and a metrics refusal must not stop capture — the same isolation §3.4 states
// for the observability lane, one binary earlier.

// noteData marks a venue frame's arrival instant, for the age gauge.
func (s *sourceMetrics) noteData(recv time.Time) { s.lastData = recv }

// countGap counts one gap record this process wrote.
func (s *sourceMetrics) countGap(c enginecapture.Cause) {
	_ = s.m.gaps.Add(observability.Count(1), append(s.labels, string(c))...)
}

// exportWriter folds a writer snapshot into the counters and gauges.
func (s *sourceMetrics) exportWriter(st enginecapture.WriterStats) {
	_ = s.m.records.Add(observability.Count(int64(st.Records-s.lastWriter.Records)), s.labels...)
	_ = s.m.dataRecords.Add(observability.Count(int64(st.DataRecords-s.lastWriter.DataRecords)), s.labels...)
	_ = s.m.writtenBytes.Add(observability.Bytes(st.Bytes-s.lastWriter.Bytes), s.labels...)
	s.lastWriter = st
	if !s.lastData.IsZero() {
		_ = s.m.dataAge.Set(observability.Seconds(s.clock.Now().Sub(s.lastData)), s.labels...)
	}
}

// exportQueue folds a queue snapshot in.
func (s *sourceMetrics) exportQueue(st enginecapture.QueueStats) {
	_ = s.m.drops.Add(observability.Count(int64(st.Drops-s.lastQueue.Drops)), s.labels...)
	_ = s.m.highWater.Set(observability.Count(int64(st.HighWater)), s.labels...)
	s.lastQueue = st
}

// exportStream folds a transport snapshot in.
func (s *sourceMetrics) exportStream(consecutiveFailures uint64) {
	_ = s.m.consecFails.Set(observability.Count(int64(consecutiveFailures)), s.labels...)
}
