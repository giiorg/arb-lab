package capture

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// validRestSource is the real overlay row — the fundingInfo facts Profiles
// verified on 2026-07-29.
func validRestSource() RestSourceConfig {
	return RestSourceConfig{
		Venue: "binance", Product: "linear_perp", Contract: "linear",
		Symbol: "BTCUSDT", Stream: "funding",
		Scheme: "https", Host: "fapi.binance.com", Path: "/fapi/v1/fundingInfo",
		Topic:          "fundingInfo",
		PollInterval:   config.NewDuration(time.Hour),
		RequestTimeout: config.NewDuration(10 * time.Second),
	}
}

// fakeVenue is an HTTP venue the test controls: what to answer, and a signal
// per ask. The signal fires BEFORE the response is written, and the tests use
// it as their ordering fence — everything a test did before an observed ask
// was fully processed by the pipeline's single goroutine before that ask's
// tick iteration began.
type fakeVenue struct {
	srv  *httptest.Server
	asks chan struct{}

	mu     sync.Mutex
	status int
	body   string
}

func newFakeVenue(t *testing.T) *fakeVenue {
	t.Helper()
	v := &fakeVenue{asks: make(chan struct{}, 16), status: http.StatusOK, body: `[]`}
	v.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v.asks <- struct{}{}
		v.mu.Lock()
		status, body := v.status, v.body
		v.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(v.srv.Close)
	return v
}

func (v *fakeVenue) answer(status int, body string) {
	v.mu.Lock()
	v.status, v.body = status, body
	v.mu.Unlock()
}

// waitAsk bounds the wait — a pipeline that never asks is a finding, not a
// hung suite.
func (v *fakeVenue) waitAsk(t *testing.T) {
	t.Helper()
	select {
	case <-v.asks:
	case <-time.After(10 * time.Second):
		t.Fatal("no ask within the bound")
	}
}

// restHarness runs one restPipeline against a fakeVenue under a fakeClock.
type restHarness struct {
	t      *testing.T
	cfg    Config
	clk    *fakeClock
	venue  *fakeVenue
	p      *restPipeline
	errs   chan error
	tick   chan struct{}
	done   chan error
	cancel context.CancelFunc
}

func newRestHarness(t *testing.T, root string, edit func(*Config)) *restHarness {
	t.Helper()
	venue := newFakeVenue(t)
	clk := newFakeClock()
	cfg := validConfig()
	cfg.Root = root
	src := validRestSource()
	src.Scheme = "http"
	src.Host = strings.TrimPrefix(venue.srv.URL, "http://")
	src.RequestTimeout = config.NewDuration(2 * time.Second)
	cfg.RestSources = []RestSourceConfig{src}
	if edit != nil {
		edit(&cfg)
	}
	reg := observability.NewRegistry()
	met, err := newMetrics(reg, cfg)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	errs := make(chan error, 1)
	p, err := newRestPipeline(cfg, cfg.RestSources[0], clk,
		Build{Tool: "arblab-test", GitSHA: "deadbeef"}, "cfg-hash-fixture",
		newSourceMetrics(met, clk, restLabelValues(cfg.RestSources[0])), errs)
	if err != nil {
		t.Fatalf("rest pipeline: %v", err)
	}
	return &restHarness{t: t, cfg: cfg, clk: clk, venue: venue, p: p, errs: errs,
		tick: make(chan struct{}, 1), done: make(chan error, 1)}
}

func (h *restHarness) start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- h.p.run(ctx, h.tick) }()
}

// settle returns once every iteration the pipeline began before the call has
// completed — including the in-flight ask waitAsk observed the START of. The
// first send buffers; the second can only be accepted after the first was
// received, and the single loop receives only between iterations. Without
// this fence, stop's cancellation races the response read and a 200 records
// as "context canceled".
func (h *restHarness) settle() {
	h.tick <- struct{}{}
	h.tick <- struct{}{}
}

func (h *restHarness) stop() error {
	h.t.Helper()
	h.cancel()
	select {
	case err := <-h.done:
		return err
	case <-time.After(10 * time.Second):
		h.t.Fatal("the rest pipeline did not finish within its bound")
		return nil
	}
}

// restRecords reads every window file the run produced, in window order, and
// returns the decoded Rest payloads — through the exported codec, not the
// writer's bookkeeping.
func (h *restHarness) restRecords() []enginecapture.RestPayload {
	h.t.Helper()
	key := h.cfg.RestSources[0].Key()
	period := h.cfg.RotationPeriod.Duration()
	start := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	var out []enginecapture.RestPayload
	for w := start; !w.After(h.clk.Now()); w = w.Add(period) {
		path := key.FilePath(h.cfg.Root, w, period)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		for _, r := range readRecords(h.t, path) {
			if r.Kind != enginecapture.RecordKindRest {
				continue
			}
			p, err := enginecapture.DecodeRestPayload(r.Payload)
			if err != nil {
				h.t.Fatalf("decode rest payload: %v", err)
			}
			out = append(out, p)
		}
	}
	return out
}

func (h *restHarness) recordKinds() []enginecapture.RecordKind {
	h.t.Helper()
	key := h.cfg.RestSources[0].Key()
	period := h.cfg.RotationPeriod.Duration()
	start := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	var out []enginecapture.RecordKind
	for w := start; !w.After(h.clk.Now()); w = w.Add(period) {
		path := key.FilePath(h.cfg.Root, w, period)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, kinds(readRecords(h.t, path))...)
	}
	return out
}

func (h *restHarness) noPipelineError() {
	h.t.Helper()
	select {
	case err := <-h.errs:
		h.t.Fatalf("the pipeline reported an error: %v", err)
	default:
	}
}

func TestRestPipelineFirstPollIsImmediate(t *testing.T) {
	h := newRestHarness(t, t.TempDir(), nil)
	h.venue.answer(http.StatusOK, `[{"symbol":"BTCUSDT","fundingIntervalHours":8,"updateTime":null}]`)
	h.start()
	h.venue.waitAsk(t)
	h.settle()
	if err := h.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.noPipelineError()

	rests := h.restRecords()
	if len(rests) != 1 {
		t.Fatalf("rest records: %d, want 1 (the immediate first ask)", len(rests))
	}
	got := rests[0]
	if got.Meta.Method != http.MethodGet {
		t.Fatalf("method %q, want GET", got.Meta.Method)
	}
	if want := "http://" + h.cfg.RestSources[0].Host + "/fapi/v1/fundingInfo"; got.Meta.URL != want {
		t.Fatalf("url %q, want %q", got.Meta.URL, want)
	}
	if got.Meta.Status != http.StatusOK {
		t.Fatalf("status %d, want 200 (meta %+v)", got.Meta.Status, got.Meta)
	}
	// updateTime:null rides the archive verbatim — the trap Profiles fact 3
	// names lands intact for step 5 to face, not silently normalised here.
	if !strings.Contains(string(got.Body), `"updateTime":null`) {
		t.Fatalf("body %q lost the venue's null", got.Body)
	}
	if want := time.Date(2026, 7, 30, 12, 30, 0, 0, time.UTC); !got.Meta.AskedAt.Equal(want) {
		t.Fatalf("asked_at %v, want %v", got.Meta.AskedAt, want)
	}

	// The Open record's provenance names the request (§37f): Endpoint is the
	// URL, Subscribe is the request line, and there is no ack because none
	// exists to record.
	key := h.cfg.RestSources[0].Key()
	recs := readRecords(t, key.FilePath(h.cfg.Root,
		time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), h.cfg.RotationPeriod.Duration()))
	if len(recs) == 0 || recs[0].Kind != enginecapture.RecordKindOpen {
		t.Fatalf("the file does not begin with an Open record")
	}
	var open struct {
		Provenance struct {
			Endpoint     string `json:"endpoint"`
			Subscribe    []byte `json:"subscribe"`
			SubscribeAck []byte `json:"subscribe_ack"`
		} `json:"provenance"`
	}
	if err := json.Unmarshal(recs[0].Payload, &open); err != nil {
		t.Fatalf("decode open payload: %v", err)
	}
	if want := "http://" + h.cfg.RestSources[0].Host + "/fapi/v1/fundingInfo"; open.Provenance.Endpoint != want {
		t.Errorf("open endpoint %q, want %q", open.Provenance.Endpoint, want)
	}
	if want := "GET /fapi/v1/fundingInfo"; string(open.Provenance.Subscribe) != want {
		t.Errorf("open subscribe %q, want %q", open.Provenance.Subscribe, want)
	}
	if len(open.Provenance.SubscribeAck) != 0 {
		t.Errorf("open subscribe_ack %q: an overlay stream has no ack to record", open.Provenance.SubscribeAck)
	}
}

// The scheduling-rate claim (§37d), pinned the way §36e demands: MORE ticks
// than due polls, at varied below-boundary instants, with the boundary hit
// EXACTLY. The AskedAt values decoded from the archive say which instants
// polled — a pipeline that polls per tick, or one whose due check is > where
// it must be >=, produces a different set, not a different count alone.
func TestRestPipelinePollsOnTheGridNotOnTheTick(t *testing.T) {
	h := newRestHarness(t, t.TempDir(), nil)
	h.start()
	h.venue.waitAsk(t) // the immediate ask, at 12:30

	// Three ticks strictly inside the interval, the last one nanosecond
	// short of the boundary. None may poll.
	h.clk.advance(10 * time.Minute) // 12:40
	h.tick <- struct{}{}
	h.clk.advance(10 * time.Minute) // 12:50
	h.tick <- struct{}{}
	h.clk.advance(10*time.Minute - time.Nanosecond) // 13:00 - 1ns
	h.tick <- struct{}{}

	// Exactly ON the grid instant: this one must poll.
	h.clk.advance(time.Nanosecond) // 13:00 exactly
	h.tick <- struct{}{}
	h.venue.waitAsk(t)
	h.settle()

	if err := h.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.noPipelineError()

	rests := h.restRecords()
	if len(rests) != 2 {
		t.Fatalf("rest records: %d, want exactly 2 (start and the grid instant)", len(rests))
	}
	wantAsked := []time.Time{
		time.Date(2026, 7, 30, 12, 30, 0, 0, time.UTC),
		time.Date(2026, 7, 30, 13, 0, 0, 0, time.UTC),
	}
	for i, r := range rests {
		if !r.Meta.AskedAt.Equal(wantAsked[i]) {
			t.Fatalf("rest[%d] asked_at %v, want %v", i, r.Meta.AskedAt, wantAsked[i])
		}
	}
}

// The tick drives the WRITER's timekeeping too (§37f): the same iteration
// that polls has already run Tick, so an observed ask proves the Alive tick
// that was due got minted. Delete the w.Tick call and this fails.
func TestRestPipelineDrivesTheWritersTimekeeping(t *testing.T) {
	h := newRestHarness(t, t.TempDir(), nil)
	h.start()
	h.venue.waitAsk(t)

	h.clk.advance(31 * time.Minute) // 13:01 — past an Alive due AND the grid
	h.tick <- struct{}{}
	h.venue.waitAsk(t)

	if err := h.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	alive := 0
	for _, k := range h.recordKinds() {
		if k == enginecapture.RecordKindAlive {
			alive++
		}
	}
	if alive == 0 {
		t.Fatal("no Alive tick minted: the pipeline is not driving Writer.Tick")
	}
}

func TestRestPipelineRecordsTheVenueAnswerVerbatim(t *testing.T) {
	h := newRestHarness(t, t.TempDir(), nil)
	h.venue.answer(http.StatusTooManyRequests, `{"code":-1003,"msg":"Too many requests."}`)
	h.start()
	h.venue.waitAsk(t)
	h.settle()
	if err := h.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.noPipelineError()

	rests := h.restRecords()
	if len(rests) != 1 {
		t.Fatalf("rest records: %d, want 1", len(rests))
	}
	if rests[0].Meta.Status != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429 — a refusal is a result, not an error", rests[0].Meta.Status)
	}
	if !strings.Contains(string(rests[0].Body), "-1003") {
		t.Fatalf("body %q lost the venue's words", rests[0].Body)
	}
}

// §37b: a failed ask is a Rest record with the transport's words, and the
// pipeline stays healthy — the next poll is the retry.
func TestRestPipelineRecordsAFailedAsk(t *testing.T) {
	h := newRestHarness(t, t.TempDir(), nil)
	h.venue.srv.Close() // the venue is gone before the first ask
	h.start()
	if err := h.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.noPipelineError()

	rests := h.restRecords()
	if len(rests) != 1 {
		t.Fatalf("rest records: %d, want 1 — the failed ask IS the record", len(rests))
	}
	if rests[0].Meta.Status != 0 || rests[0].Meta.Error == "" {
		t.Fatalf("meta %+v: a transport failure records status 0 and the transport's words",
			rests[0].Meta)
	}
	if len(rests[0].Body) != 0 {
		t.Fatalf("body %q: a failed ask has no body", rests[0].Body)
	}

	// The file closed cleanly behind it: alive, asks failing, shut down.
	ks := h.recordKinds()
	if len(ks) == 0 || ks[len(ks)-1] != enginecapture.RecordKindClose {
		t.Fatalf("kinds %v: the run must still close its file", ks)
	}
}

func TestConfigValidateRestSources(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Config)
		want error
	}{
		{"the real overlay row validates", func(c *Config) {}, nil},
		{"poll_interval zero", func(c *Config) {
			c.RestSources[0].PollInterval = config.Duration{}
		}, ErrDurationRequired},
		{"request_timeout zero", func(c *Config) {
			c.RestSources[0].RequestTimeout = config.Duration{}
		}, ErrDurationRequired},
		{"request_timeout equal to alive_interval is legal", func(c *Config) {
			c.RestSources[0].RequestTimeout = c.AliveInterval
		}, nil},
		{"request_timeout past alive_interval", func(c *Config) {
			c.RestSources[0].RequestTimeout =
				config.NewDuration(c.AliveInterval.Duration() + time.Second)
		}, ErrRestTimeoutRange},
		{"socket scheme on an overlay row", func(c *Config) {
			c.RestSources[0].Scheme = "wss"
		}, transport.ErrRestScheme},
		{"topic unset", func(c *Config) {
			c.RestSources[0].Topic = ""
		}, enginecapture.ErrTopicUnset},
		{"duplicate key across the two lists", func(c *Config) {
			// Same SourceID as the WS row, same topic: one tree, two writers.
			s := c.Sources[0]
			c.RestSources[0].Venue = s.Venue
			c.RestSources[0].Product = s.Product
			c.RestSources[0].Contract = s.Contract
			c.RestSources[0].Symbol = s.Symbol
			c.RestSources[0].Stream = s.Stream
			c.RestSources[0].Topic = s.Topic
		}, ErrSourceDuplicate},
		{"duplicate key inside the overlay list", func(c *Config) {
			c.RestSources = append(c.RestSources, c.RestSources[0])
		}, ErrSourceDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.RestSources = []RestSourceConfig{validRestSource()}
			tc.edit(&cfg)
			err := cfg.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// An overlay row left out of closedLabels is a stream whose every metric
// write is silently refused — §36t's one-source collapse, on the second list.
func TestClosedLabelsIncludeRestSources(t *testing.T) {
	cfg := validConfig()
	cfg.RestSources = []RestSourceConfig{validRestSource()}

	reg := observability.NewRegistry()
	m, err := newMetrics(reg, cfg)
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	sm := newSourceMetrics(m, newFakeClock(), restLabelValues(cfg.RestSources[0]))
	sm.exportWriter(enginecapture.WriterStats{Records: 3, Bytes: 128})

	if rej := reg.Rejected(); rej != 0 {
		t.Fatalf("the registry rejected %d observations for the overlay row; "+
			"its label values were not declared", rej)
	}
}
