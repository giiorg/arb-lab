package capture

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// fakeClock is a TimeSource the test advances by hand. Mutex-guarded because
// the sink (test goroutine) and the writer (consumer goroutine) both read it.
type fakeClock struct {
	mu   sync.Mutex
	now  time.Time
	mono time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 7, 30, 12, 30, 0, 0, time.UTC), mono: time.Second}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Mono() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mono
}

func (c *fakeClock) Backsteps() (uint64, time.Duration) { return 0, 0 }

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.mono += d
}

// harness runs one pipeline with the test standing in for the stream
// goroutine: the test calls the sink's methods directly, which is exactly the
// transport's contract (one goroutine, in order).
type harness struct {
	t      *testing.T
	cfg    Config
	clk    *fakeClock
	reg    *observability.Registry
	p      *pipeline
	errs   chan error
	tick   chan struct{}
	done   chan error
	cancel context.CancelFunc
}

func newHarness(t *testing.T, root string, edit func(*Config)) *harness {
	t.Helper()
	clk := newFakeClock()
	cfg := validConfig()
	cfg.Root = root
	if edit != nil {
		edit(&cfg)
	}
	reg := observability.NewRegistry()
	met, err := newMetrics(reg, cfg)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	errs := make(chan error, len(cfg.Sources))
	p, err := newPipeline(cfg, cfg.Sources[0], clk,
		Build{Tool: "arblab-test", GitSHA: "deadbeef"}, "cfg-hash-fixture",
		newSourceMetrics(met, clk, cfg.Sources[0]), errs)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	return &harness{t: t, cfg: cfg, clk: clk, reg: reg, p: p, errs: errs,
		tick: make(chan struct{}, 1), done: make(chan error, 1)}
}

// start launches the pump and the consumer — everything except the transport.
func (h *harness) start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go h.p.pump()
	go func() { h.done <- h.p.consume(ctx, h.tick) }()
}

// stop is a clean shutdown: the producer (this test) closes the queue and the
// consumer drains, flushes and closes the writer. The wait is bounded — a
// consumer that hangs is a finding, not a hung test suite.
func (h *harness) stop() error {
	h.t.Helper()
	h.p.queue.Close()
	select {
	case err := <-h.done:
		if h.cancel != nil {
			h.cancel()
		}
		return err
	case <-time.After(10 * time.Second):
		h.t.Fatal("the consumer did not finish within its bound")
		return nil
	}
}

func (h *harness) sink() *sink { return (*sink)(h.p) }

// data pushes one venue frame through the sink.
func (h *harness) data(payload string) { h.sink().Data([]byte(payload)) }

// archivePath is where the harness's single stream writes the current window.
func (h *harness) archivePath() string {
	return h.cfg.Sources[0].Key().FilePath(h.cfg.Root,
		enginecapture.Window(h.clk.Now(), h.cfg.RotationPeriod.Duration()),
		h.cfg.RotationPeriod.Duration())
}

// readRecords walks the archive file with the exported codec — the same
// decoder verify uses, and not the writer's own bookkeeping.
func readRecords(t *testing.T, path string) []enginecapture.Record {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the archive: %v", err)
	}
	off, err := enginecapture.ParseFileHeader(b)
	if err != nil {
		t.Fatalf("file header: %v", err)
	}
	var recs []enginecapture.Record
	for off < len(b) {
		r, n, err := enginecapture.DecodeRecord(b[off:])
		if err != nil {
			t.Fatalf("record at offset %d: %v", off, err)
		}
		recs = append(recs, r)
		off += n
	}
	return recs
}

// kinds compresses a record list for order assertions.
func kinds(recs []enginecapture.Record) []enginecapture.RecordKind {
	out := make([]enginecapture.RecordKind, len(recs))
	for i, r := range recs {
		out[i] = r.Kind
	}
	return out
}

func decodeGap(t *testing.T, r enginecapture.Record) enginecapture.Gap {
	t.Helper()
	var g enginecapture.Gap
	if err := json.Unmarshal(r.Payload, &g); err != nil {
		t.Fatalf("gap payload: %v", err)
	}
	return g
}

// TestRestartIntoAnOpenHourContinuesTheSequence is §36m's fixture as a test
// against the COMPOSITION ROOT: a second run into an hour the first run
// already wrote must have its reader seeded from the archive, or every Data
// record of the second run is refused for the rest of that hour.
func TestRestartIntoAnOpenHourContinuesTheSequence(t *testing.T) {
	root := t.TempDir()

	h1 := newHarness(t, root, nil)
	h1.start()
	h1.sink().Subscribed(nil, nil)
	h1.data("f1")
	h1.data("f2")
	h1.data("f3")
	if err := h1.stop(); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// The restart, into the same hour. Without the LastSeq seed the first
	// Data below is refused with ErrSeqNotAscending and the run fails.
	h2 := newHarness(t, root, nil)
	h2.clk.advance(time.Minute)
	h2.start()
	h2.sink().Subscribed(nil, nil)
	h2.data("f4")
	h2.data("f5")
	if err := h2.stop(); err != nil {
		t.Fatalf("restarted run: %v", err)
	}

	var seqs []uint64
	for _, r := range readRecords(t, h2.archivePath()) {
		if r.Kind == enginecapture.RecordKindData {
			seqs = append(seqs, r.CaptureSeq)
		}
	}
	want := []uint64{1, 2, 3, 4, 5}
	if len(seqs) != len(want) {
		t.Fatalf("the hour holds %d data records %v, want %v", len(seqs), seqs, want)
	}
	for i := range want {
		if seqs[i] != want[i] {
			t.Fatalf("data sequences %v, want %v — the restarted reader did not "+
				"continue from the archive's last sequence", seqs, want)
		}
	}

	// The out-of-process check agrees: one continuous, integrity-clean hour.
	rep, err := enginecapture.Verify(h2.cfg.Verify(
		enginecapture.Window(h2.clk.Now(), time.Hour),
		enginecapture.Window(h2.clk.Now(), time.Hour).Add(time.Hour)))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := rep.ExitClass(); got == enginecapture.ClassIntegrity {
		t.Fatalf("verify reports integrity findings over the restarted hour: %v", rep.Findings)
	}
}

// TestDisruptionCausesReachTheArchive drives every declared transport cause
// through the sink and asserts the archived gap record carries the mapped
// cause — the switch the RESUME list requires exhaustiveswitch to bind on.
func TestDisruptionCausesReachTheArchive(t *testing.T) {
	cases := []struct {
		from transport.DisruptionCause
		want enginecapture.Cause
	}{
		{transport.DisruptionDial, enginecapture.CauseReconnect},
		{transport.DisruptionSubscribe, enginecapture.CauseReconnect},
		{transport.DisruptionRead, enginecapture.CauseReconnect},
		{transport.DisruptionNoFirstData, enginecapture.CauseNoFirstData},
	}
	for _, tc := range cases {
		t.Run(string(tc.from), func(t *testing.T) {
			h := newHarness(t, t.TempDir(), nil)
			h.start()
			h.sink().Subscribed(nil, nil)
			h.data("f1")
			h.sink().Disrupted(transport.Disruption{Cause: tc.from, Detail: "venue said so"})
			h.data("f2")
			if err := h.stop(); err != nil {
				t.Fatal(err)
			}

			var gaps []enginecapture.Gap
			for _, r := range readRecords(t, h.archivePath()) {
				if r.Kind == enginecapture.RecordKindGap {
					gaps = append(gaps, decodeGap(t, r))
				}
			}
			if len(gaps) != 1 {
				t.Fatalf("archive holds %d gap records, want exactly 1", len(gaps))
			}
			if gaps[0].Cause != tc.want {
				t.Errorf("gap cause %q, want %q", gaps[0].Cause, tc.want)
			}
			// The transport's own vocabulary survives in the detail, so an
			// operator can still tell a dial failure from a read timeout.
			if !strings.Contains(gaps[0].Detail, string(tc.from)) {
				t.Errorf("gap detail %q does not carry the transport cause %q",
					gaps[0].Detail, tc.from)
			}
		})
	}
}

// TestGapSitsBetweenTheFramesItInterrupted pins the afterSeq ordering: a
// disruption between frame 3 and frame 4 is archived between them, not
// wherever the consumer happened to be.
func TestGapSitsBetweenTheFramesItInterrupted(t *testing.T) {
	h := newHarness(t, t.TempDir(), nil)
	h.start()
	h.sink().Subscribed(nil, nil)
	h.data("f1")
	h.data("f2")
	h.data("f3")
	h.sink().Disrupted(transport.Disruption{Cause: transport.DisruptionRead, Detail: "socket died"})
	h.data("f4")
	h.data("f5")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	recs := readRecords(t, h.archivePath())
	got := kinds(recs)
	want := []enginecapture.RecordKind{
		enginecapture.RecordKindOpen,
		enginecapture.RecordKindData, enginecapture.RecordKindData, enginecapture.RecordKindData,
		enginecapture.RecordKindGap,
		enginecapture.RecordKindData, enginecapture.RecordKindData,
		enginecapture.RecordKindClose,
	}
	if len(got) != len(want) {
		t.Fatalf("record kinds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("record kinds %v, want %v — the gap is not between the "+
				"frames it interrupted", got, want)
		}
	}
	// The gap record echoes the last data sequence before it (§32d's "this
	// happened after frame N").
	if gapRec := recs[4]; gapRec.CaptureSeq != 3 {
		t.Errorf("the gap echoes sequence %d, want 3", gapRec.CaptureSeq)
	}
}

// TestPreOpenDisruptionIsTheRunsFirstFinding: failures before the first
// successful connection have no writer yet; they are parked and become the
// first records after the Open — what this run found on the way up.
func TestPreOpenDisruptionIsTheRunsFirstFinding(t *testing.T) {
	h := newHarness(t, t.TempDir(), nil)
	h.start()
	h.sink().Disrupted(transport.Disruption{Cause: transport.DisruptionDial, Detail: "refused"})
	h.sink().Disrupted(transport.Disruption{Cause: transport.DisruptionDial, Detail: "refused again"})
	h.sink().Subscribed(nil, nil)
	h.data("f1")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	got := kinds(readRecords(t, h.archivePath()))
	want := []enginecapture.RecordKind{
		enginecapture.RecordKindOpen,
		enginecapture.RecordKindGap, enginecapture.RecordKindGap,
		enginecapture.RecordKindData,
		enginecapture.RecordKindClose,
	}
	if len(got) != len(want) {
		t.Fatalf("record kinds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("record kinds %v, want %v", got, want)
		}
	}
}

// TestAckBecomesProvenance: the venue's subscribe ack lands in the Open
// record, which is the whole reason the writer opens on the first Subscribed
// rather than at boot (§32g, §36h).
func TestAckBecomesProvenance(t *testing.T) {
	h := newHarness(t, t.TempDir(), nil)
	h.start()
	request := []byte(`{"op":"subscribe","args":["test"]}`)
	ack := []byte(`{"success":true}`)
	h.sink().Subscribed(request, ack)
	h.data("f1")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	recs := readRecords(t, h.archivePath())
	if recs[0].Kind != enginecapture.RecordKindOpen {
		t.Fatalf("first record is %s, want Open", recs[0].Kind)
	}
	var open struct {
		Provenance enginecapture.Provenance `json:"provenance"`
	}
	if err := json.Unmarshal(recs[0].Payload, &open); err != nil {
		t.Fatalf("open payload: %v", err)
	}
	if string(open.Provenance.SubscribeAck) != string(ack) {
		t.Errorf("archived ack %q, want %q", open.Provenance.SubscribeAck, ack)
	}
	if string(open.Provenance.Subscribe) != string(request) {
		t.Errorf("archived request %q, want %q", open.Provenance.Subscribe, request)
	}
	if open.Provenance.Tool != "arblab-test" || open.Provenance.GitSHA != "deadbeef" {
		t.Errorf("build provenance %q/%q did not flow through",
			open.Provenance.Tool, open.Provenance.GitSHA)
	}
	if open.Provenance.ConfigHash != "cfg-hash-fixture" {
		t.Errorf("config hash %q did not flow through", open.Provenance.ConfigHash)
	}
	if open.Provenance.Endpoint == "" {
		t.Error("the Open record names no endpoint")
	}
}

// TestDataBeforeAckOpensOnce: a venue that sends data before acking must not
// lose that data, and the later ack must not produce a second Open record.
// The Open's provenance honestly carries the request and no ack, because at
// open time no ack existed.
func TestDataBeforeAckOpensOnce(t *testing.T) {
	// A message-discipline source, so a subscribe frame exists to be recorded.
	h := newHarness(t, t.TempDir(), func(c *Config) {
		s := &c.Sources[0]
		s.Protocol, s.Prefix = "bybit-v5", "/v5/public/linear"
		s.Host = "stream.bybit.com"
		s.Topic = "orderbook.50.BTCUSDT"
	})
	h.start()
	h.data("early") // before any ack
	h.sink().Subscribed([]byte("req"), []byte("late-ack"))
	h.data("f2")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	recs := readRecords(t, h.archivePath())
	opens := 0
	for _, r := range recs {
		if r.Kind == enginecapture.RecordKindOpen {
			opens++
		}
	}
	if opens != 1 {
		t.Fatalf("archive holds %d Open records, want exactly 1", opens)
	}
	var open struct {
		Provenance enginecapture.Provenance `json:"provenance"`
	}
	if err := json.Unmarshal(recs[0].Payload, &open); err != nil {
		t.Fatal(err)
	}
	if len(open.Provenance.SubscribeAck) != 0 {
		t.Errorf("the Open record carries an ack %q that did not exist at open time",
			open.Provenance.SubscribeAck)
	}
	if len(open.Provenance.Subscribe) == 0 {
		t.Error("the Open record carries no subscribe request; the dialect has one")
	}
	// And the early frame is sequence 1 — captured, not sacrificed to the
	// handshake.
	for _, r := range recs {
		if r.Kind == enginecapture.RecordKindData {
			if r.CaptureSeq != 1 || string(r.Payload) != "early" {
				t.Errorf("first data record is seq %d payload %q, want 1/%q",
					r.CaptureSeq, r.Payload, "early")
			}
			break
		}
	}
}

// TestOverflowHoleIsRecordedAtShutdown: stopping while the queue is
// overflowing is exactly the case whose loss must not go unrecorded (§32f).
// Drop-newest keeps the two oldest frames; the hole is 3..5 and the gap
// record must cover it.
func TestOverflowHoleIsRecordedAtShutdown(t *testing.T) {
	h := newHarness(t, t.TempDir(), func(c *Config) { c.QueueDepth = 2 })
	// The consumer is deliberately NOT running: the queue must fill.
	h.sink().Subscribed(nil, nil)
	for _, f := range []string{"f1", "f2", "f3", "f4", "f5"} {
		h.data(f)
	}
	h.start()
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	recs := readRecords(t, h.archivePath())
	var gaps []enginecapture.Gap
	dataSeqs := []uint64{}
	for _, r := range recs {
		switch r.Kind {
		case enginecapture.RecordKindGap:
			gaps = append(gaps, decodeGap(t, r))
		case enginecapture.RecordKindData:
			dataSeqs = append(dataSeqs, r.CaptureSeq)
		case enginecapture.RecordKindUnspecified, enginecapture.RecordKindOpen,
			enginecapture.RecordKindAlive, enginecapture.RecordKindRest,
			enginecapture.RecordKindClose, enginecapture.RecordKindClockStep:
			// Not under test here; named so §3.6 rule 10 holds.
		}
	}
	if len(dataSeqs) != 2 || dataSeqs[0] != 1 || dataSeqs[1] != 2 {
		t.Fatalf("data sequences %v, want [1 2] (drop-newest keeps the oldest)", dataSeqs)
	}
	if len(gaps) != 1 {
		t.Fatalf("archive holds %d gap records, want 1", len(gaps))
	}
	g := gaps[0]
	if g.Cause != enginecapture.CauseOverflow || g.From != 3 || g.To != 5 || g.Count != 3 {
		t.Fatalf("overflow gap %+v, want cause=overflow [3,5] count 3", g)
	}
}

// TestOpenFailureFailsTheRun: a stream that cannot open its archive (here:
// the flock is already held) reports the error and captures nothing, rather
// than reading a venue feed into nowhere.
func TestOpenFailureFailsTheRun(t *testing.T) {
	root := t.TempDir()

	h1 := newHarness(t, root, nil)
	h1.start()
	h1.sink().Subscribed(nil, nil)
	h1.data("f1")

	// Same key, same root, while the first run holds the lock.
	h2 := newHarness(t, root, nil)
	h2.start()
	h2.sink().Subscribed(nil, nil)

	select {
	case err := <-h2.errs:
		if err == nil {
			t.Fatal("the second writer's open reported a nil error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second run never reported its open failure")
	}
	// Frames offered after the failure are dropped, not stamped into a
	// nonexistent archive; the sink is dead but must not panic or block.
	h2.data("into the void")

	if err := h1.stop(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	h2.p.queue.Close()
	select {
	case <-h2.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the failed run's consumer did not finish")
	}
}

// TestMetricsFlowThroughTheClosedLabelSets: the §32j metric set registers with
// label sets declared from config, and a full pipeline pass records without a
// single rejection — the "no Unbounded reason needed" claim, measured.
func TestMetricsFlowThroughTheClosedLabelSets(t *testing.T) {
	h := newHarness(t, t.TempDir(), nil)
	h.start()
	h.sink().Subscribed(nil, nil)
	h.data("f1")
	h.data("f2")
	h.sink().Disrupted(transport.Disruption{Cause: transport.DisruptionRead, Detail: "x"})
	h.data("f3")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	if rej := h.reg.Rejected(); rej != 0 {
		t.Fatalf("the registry rejected %d observations; the label sets do not "+
			"cover the values the pipeline records", rej)
	}
	series := h.reg.Snapshot()
	find := func(name string) (observability.Series, bool) {
		for _, s := range series {
			if s.Name == name {
				return s, true
			}
		}
		return observability.Series{}, false
	}
	if s, ok := find("capture_data_records_total"); !ok || s.Value.Mantissa() != 3 {
		t.Errorf("capture_data_records_total = %+v, want 3", s.Value)
	}
	if s, ok := find("capture_gaps_total"); !ok || s.Value.Mantissa() != 1 {
		t.Errorf("capture_gaps_total = %+v, want 1", s.Value)
	}
	if s, ok := find("capture_records_total"); !ok || s.Value.Mantissa() < 5 {
		// Open + 3 data + gap + Close at minimum.
		t.Errorf("capture_records_total = %+v, want at least 5", s.Value)
	}
}

// TestConsumerErrorIsReported: a writer refusal inside the steady loop (here
// forced by a sequence regression, which the writer refuses by design) fails
// the pipeline loudly instead of dropping records in silence.
func TestConsumerErrorIsReported(t *testing.T) {
	h := newHarness(t, t.TempDir(), nil)
	h.start()
	h.sink().Subscribed(nil, nil)
	h.data("f1")
	// Reach around the sink and push a stamped record with a regressed
	// sequence — the shape a defective reader would produce.
	h.p.queue.Push(enginecapture.Record{
		Kind:       enginecapture.RecordKindData,
		CaptureSeq: 1,
		RecvTime:   h.clk.Now(),
		Mono:       h.clk.Mono(),
		Payload:    []byte("regressed"),
	})
	select {
	case err := <-h.errs:
		if !errors.Is(err, enginecapture.ErrSeqNotAscending) {
			t.Fatalf("reported %v, want ErrSeqNotAscending", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the consumer never reported the writer's refusal")
	}
	h.p.queue.Close()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the consumer did not finish after its error")
	}
}
