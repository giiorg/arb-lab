package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// These tests speak the real websocket protocol to a real server rather than to
// a mock, and that is the point of them. A mocked Conn would let the handshake,
// the subscribe write and the ack read all be asserted against our own beliefs
// about them — which is the one thing a transport test must not do, because the
// defect this package exists to catch is a socket that connects and then does
// nothing.

// waitFor is the bound every channel read in this file goes through.
//
// A mutation here does not make a test fail, it makes a test BLOCK — the harness
// reports TIMEOUT!! and the claim was never checked while looking covered. That
// has now bitten clock and boot; this package has more channels than either, and
// nothing reads a channel without a deadline.
func waitFor[T any](t *testing.T, ch <-chan T, d time.Duration, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(d): //notimenow:allow the test harness's own bound on a channel read; it stamps nothing
		var zero T
		t.Fatalf("timed out after %v waiting for %s", d, what)
		return zero
	}
}

// wantNothing asserts a channel stays empty for d. Vacuous on its own — every
// such assertion in this file names the test that is its positive control.
func wantNothing[T any](t *testing.T, ch <-chan T, d time.Duration, what string) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("unexpected %s: %+v", what, v)
	case <-time.After(d): //notimenow:allow the test harness's own bound on a channel read; it stamps nothing
	}
}

type sinkEvent struct {
	Request []byte
	Ack     []byte
}

// testSink buffers generously so it never blocks the stream — Sink's contract is
// that no method blocks, and a test sink that violated it would be testing the
// wrong thing.
type testSink struct {
	data chan []byte
	subs chan sinkEvent
	dis  chan Disruption
}

func newSink() *testSink {
	return &testSink{
		data: make(chan []byte, 256),
		subs: make(chan sinkEvent, 16),
		dis:  make(chan Disruption, 16),
	}
}

func (s *testSink) Data(payload []byte)            { s.data <- payload }
func (s *testSink) Subscribed(request, ack []byte) { s.subs <- sinkEvent{request, ack} }
func (s *testSink) Disrupted(d Disruption)         { s.dis <- d }

// wsServer is an httptest server that speaks websocket. handle runs per
// connection, on the server's goroutine, and receives the connection count so a
// test can make the second connection behave differently from the first.
type wsServer struct {
	*httptest.Server
	host  string
	paths chan string
	conns atomic.Int64
}

func newWSServer(t *testing.T, handle func(c *websocket.Conn, n int64)) *wsServer {
	t.Helper()
	s := &wsServer{paths: make(chan string, 32)}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		select {
		case s.paths <- r.URL.RequestURI():
		default:
		}
		handle(c, s.conns.Add(1))
	}))
	t.Cleanup(s.Close)
	s.host = strings.TrimPrefix(s.Server.URL, "http://")
	return s
}

// send writes a text frame, ignoring the error: the stream closing the socket
// mid-test is normal and is not the server's business to report.
func send(c *websocket.Conn, msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.Write(ctx, websocket.MessageText, []byte(msg))
}

// sendOK is send with the error kept, for the server loops that must stop when
// the stream has gone away rather than spin for the rest of the test.
func sendOK(c *websocket.Conn, msg string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, []byte(msg)) == nil
}

// blockUntilClosed holds a connection open until the client goes away, which is
// how a server "sends nothing forever" without spinning.
func blockUntilClosed(c *websocket.Conn) {
	for {
		if _, _, err := c.Read(context.Background()); err != nil {
			return
		}
	}
}

// testConfig is a StreamConfig with short, explicit timings. Nothing is
// defaulted, mirroring StreamConfig's own rule.
func testConfig(host, prefix, topic string, proto ProtocolID, sink Sink) StreamConfig {
	return StreamConfig{
		Endpoint:         Endpoint{Scheme: "ws", Host: host, Prefix: prefix},
		Topic:            topic,
		Protocol:         proto,
		Sink:             sink,
		DialTimeout:      2 * time.Second,
		SubscribeTimeout: 2 * time.Second,
		FirstDataTimeout: 150 * time.Millisecond,
		ReadTimeout:      2 * time.Second,
		// Long enough that no test reconnects unless it means to. The reconnect
		// test overrides it.
		Backoff: []time.Duration{30 * time.Second},
	}
}

func startStream(t *testing.T, cfg StreamConfig) (*Stream, func()) {
	t.Helper()
	s, err := NewStream(cfg)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Run: %v", err)
				}
			case <-time.After(3 * time.Second): //notimenow:allow the test harness's bound on a goroutine shutdown; it stamps nothing
				t.Error("Run did not return within 3s of cancellation")
			}
		})
	}
	t.Cleanup(stop)
	return s, stop
}

// The path discipline end to end: the topic must reach the WIRE as a path
// segment, not merely be stored. Asserting the dialled path is what separates
// "the URL was built" from "the URL was built and used".
func TestStreamPathDisciplineDialsTheTopicPathAndDelivers(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, _ int64) {
		send(c, `{"e":"markPriceUpdate","s":"BTCUSDT","r":"0.00010000"}`)
		blockUntilClosed(c)
	})
	sink := newSink()
	_, stop := startStream(t, testConfig(srv.host, "/market/ws", "btcusdt@markPrice@1s",
		ProtocolBinancePath, sink))
	defer stop()

	if got := waitFor(t, srv.paths, 2*time.Second, "the dialled path"); got != "/market/ws/btcusdt@markPrice@1s" {
		t.Errorf("dialled %q; want /market/ws/btcusdt@markPrice@1s", got)
	}

	// Subscribed fires with two nils, and that nil is information: it is why
	// the first-data assertion is the only check this discipline has.
	sub := waitFor(t, sink.subs, 2*time.Second, "Subscribed")
	if sub.Request != nil || sub.Ack != nil {
		t.Errorf("Subscribed(%q, %q); want both nil under the path discipline", sub.Request, sub.Ack)
	}

	if got := string(waitFor(t, sink.data, 2*time.Second, "the data frame")); !strings.Contains(got, "markPriceUpdate") {
		t.Errorf("Data = %q", got)
	}
	wantNothing(t, sink.dis, 100*time.Millisecond, "disruption")
}

// The message discipline end to end: the subscribe frame must reach the wire and
// the venue's ack must reach the sink, because §32g archives both as provenance.
func TestStreamMessageDisciplineSubscribesAndCarriesTheAck(t *testing.T) {
	const ack = `{"success":true,"ret_msg":"","op":"subscribe","conn_id":"c-1","req_id":"tickers.BTCUSDT"}`
	got := make(chan string, 4)

	srv := newWSServer(t, func(c *websocket.Conn, _ int64) {
		_, req, err := c.Read(context.Background())
		if err != nil {
			return
		}
		got <- string(req)
		send(c, ack)
		send(c, `{"topic":"tickers.BTCUSDT","type":"snapshot","data":{"fundingRate":"0.0001"}}`)
		blockUntilClosed(c)
	})
	sink := newSink()
	_, stop := startStream(t, testConfig(srv.host, "/v5/public/linear", "tickers.BTCUSDT",
		ProtocolBybitV5, sink))
	defer stop()

	if req := waitFor(t, got, 2*time.Second, "the subscribe frame on the wire"); req !=
		`{"op":"subscribe","args":["tickers.BTCUSDT"],"req_id":"tickers.BTCUSDT"}` {
		t.Errorf("server received %s", req)
	}

	// The topic is NOT in the path here — that is the whole reason sources 5
	// and 6 need the URL to tell them apart.
	if p := waitFor(t, srv.paths, 2*time.Second, "the dialled path"); p != "/v5/public/linear" {
		t.Errorf("dialled %q; want /v5/public/linear with no topic in it", p)
	}

	sub := waitFor(t, sink.subs, 2*time.Second, "Subscribed")
	if string(sub.Ack) != ack {
		t.Errorf("Subscribed ack = %s; want the venue's bytes verbatim", sub.Ack)
	}
	if !strings.Contains(string(sub.Request), `"op":"subscribe"`) {
		t.Errorf("Subscribed request = %s", sub.Request)
	}

	if d := waitFor(t, sink.data, 2*time.Second, "the data frame"); !strings.Contains(string(d), "fundingRate") {
		t.Errorf("Data = %s", d)
	}
	// The ack did NOT arrive as data.
	wantNothing(t, sink.data, 100*time.Millisecond, "a second data frame (the ack must not be one)")
}

// TestStreamFirstDataFiresOnASilentSocket is the Binance trap, reproduced.
//
// The legacy prefix-less fstream paths were decommissioned 2026-04-23 and accept
// the connection, then send nothing, forever. This server does exactly that.
// Without §32k's assertion the process sits here politely for three months and
// captures an empty file with every light green.
func TestStreamFirstDataFiresOnASilentSocket(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, _ int64) { blockUntilClosed(c) })
	sink := newSink()
	_, stop := startStream(t, testConfig(srv.host, "/ws", "btcusdt@markPrice@1s",
		ProtocolBinancePath, sink))
	defer stop()

	// It connects and subscribes happily — that is the trap.
	waitFor(t, sink.subs, 2*time.Second, "Subscribed")

	d := waitFor(t, sink.dis, 2*time.Second, "the first-data disruption")
	if d.Cause != DisruptionNoFirstData {
		t.Errorf("Disrupted cause = %s; want %s", d.Cause, DisruptionNoFirstData)
	}
	if d.Detail == "" {
		t.Error("the disruption carried no detail")
	}
	wantNothing(t, sink.data, 50*time.Millisecond, "data on a silent socket")
}

// TestStreamPongDoesNotExtendTheFirstDataWindow is the subtle half of the same
// assertion, and it is a SEPARATE test because it kills a different defect.
//
// Classify already refuses to count a pong as data. That is not enough: with a
// per-read deadline, every pong RESTARTS the window, so a venue heartbeat inside
// the first-data timeout renews it forever and the check never fires. The
// liveness monitor is then answered by our own keepalive after all — one
// indirection further out than the obvious version of the bug.
//
// The server here pongs faster than the window and sends no data ever.
func TestStreamPongDoesNotExtendTheFirstDataWindow(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, _ int64) {
		if _, _, err := c.Read(context.Background()); err != nil {
			return
		}
		send(c, `{"success":true,"ret_msg":"","op":"subscribe"}`)
		// A pong every 20 ms against a 150 ms first-data window: comfortably
		// often enough to renew a per-read deadline forever. The loop exits on
		// a write error, which is how it stops once the stream gives up.
		for range 500 {
			if !sendOK(c, `{"success":true,"ret_msg":"pong","op":"ping","conn_id":"c-1"}`) {
				return
			}
			time.Sleep(20 * time.Millisecond) //notimenow:allow the fake VENUE's send cadence, not the stream's; it stamps nothing
		}
	})
	sink := newSink()
	s, stop := startStream(t, testConfig(srv.host, "/v5/public/linear", "tickers.BTCUSDT",
		ProtocolBybitV5, sink))
	defer stop()

	waitFor(t, sink.subs, 2*time.Second, "Subscribed")

	d := waitFor(t, sink.dis, 2*time.Second,
		"the first-data disruption despite a steady pong")
	if d.Cause != DisruptionNoFirstData {
		t.Errorf("Disrupted cause = %s; want %s — pongs must not renew the window",
			d.Cause, DisruptionNoFirstData)
	}
	wantNothing(t, sink.data, 50*time.Millisecond, "a pong delivered as data")

	// The fixture-collapse guard, and this test earned it the hard way: the
	// first version of this server called Conn.Ping in its loop, which BLOCKS,
	// so exactly one pong was ever sent. The window then closed because nothing
	// was arriving — the ordinary silent-socket case — and the renewal defect
	// was never exercised. It passed against the broken implementation and
	// looked like coverage.
	//
	// Counting executions says nothing until you have counted how many reach
	// the interesting branch. Here that count is pongs actually delivered
	// DURING the window, and if it is not several, this test is the silent-
	// socket test wearing a different name.
	if got := s.Stats().PongFrames; got < 3 {
		t.Errorf("the fixture delivered %d pongs inside the first-data window; want >= 3. "+
			"With fewer, this test cannot distinguish 'a pong renews the window' from "+
			"'nothing arrived at all'", got)
	}
}

// A refused subscription must carry the VENUE's sentence out to the operator.
func TestStreamRefusedSubscriptionCarriesTheVenueReason(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, _ int64) {
		if _, _, err := c.Read(context.Background()); err != nil {
			return
		}
		send(c, `{"success":false,"ret_msg":"Invalid symbol","op":"subscribe","req_id":"x"}`)
		blockUntilClosed(c)
	})
	sink := newSink()
	_, stop := startStream(t, testConfig(srv.host, "/v5/public/spot", "orderbook.50.NOTACOIN",
		ProtocolBybitV5, sink))
	defer stop()

	d := waitFor(t, sink.dis, 2*time.Second, "the subscribe disruption")
	if d.Cause != DisruptionSubscribe {
		t.Errorf("cause = %s; want %s — the host was reachable, so this is not a dial failure",
			d.Cause, DisruptionSubscribe)
	}
	if d.Detail != "Invalid symbol" {
		t.Errorf("detail = %q; want the venue's own %q", d.Detail, "Invalid symbol")
	}
	// Subscribed must NOT have fired: we are not subscribed to anything.
	wantNothing(t, sink.subs, 100*time.Millisecond, "Subscribed after a refusal")
}

// A dropped socket is reported and then reconnected, and the second connection
// carries data again. This is also the positive control for the clean-shutdown
// test below: it shows the harness CAN observe a disruption.
func TestStreamReconnectsAndReportsTheDrop(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, n int64) {
		send(c, `{"e":"markPriceUpdate","n":`+string(rune('0'+byte(n)))+`}`)
		if n == 1 {
			_ = c.Close(websocket.StatusGoingAway, "restarting")
			return
		}
		blockUntilClosed(c)
	})
	cfg := testConfig(srv.host, "/market/ws", "btcusdt@markPrice@1s", ProtocolBinancePath, newSink())
	cfg.Backoff = []time.Duration{5 * time.Millisecond}
	sink := cfg.Sink.(*testSink)

	s, stop := startStream(t, cfg)
	defer stop()

	waitFor(t, sink.data, 2*time.Second, "data on the first connection")
	d := waitFor(t, sink.dis, 2*time.Second, "the drop")
	if d.Cause != DisruptionRead {
		t.Errorf("cause = %s; want %s", d.Cause, DisruptionRead)
	}
	waitFor(t, sink.data, 2*time.Second, "data on the reconnected socket")

	if got := s.Stats().Connects; got < 2 {
		t.Errorf("Stats().Connects = %d; want >= 2", got)
	}
}

// TestStreamCleanShutdownReportsNoDisruption asserts an absence, so it names its
// positive control: TestStreamReconnectsAndReportsTheDrop, which shows this same
// harness does observe a disruption when there is one.
//
// The absence matters because the socket DOES end mid-read on the way out. A
// stream that reported that would put a reconnect gap in the archive for every
// orderly stop, teaching whoever reads it later to discount the record that
// matters most.
func TestStreamCleanShutdownReportsNoDisruption(t *testing.T) {
	srv := newWSServer(t, func(c *websocket.Conn, _ int64) {
		send(c, `{"e":"markPriceUpdate"}`)
		blockUntilClosed(c)
	})
	sink := newSink()
	_, stop := startStream(t, testConfig(srv.host, "/market/ws", "btcusdt@markPrice@1s",
		ProtocolBinancePath, sink))

	waitFor(t, sink.data, 2*time.Second, "data before the shutdown")
	stop() // cancels and waits for Run to return

	wantNothing(t, sink.dis, 100*time.Millisecond, "a disruption from an orderly stop")
}

// A misconfiguration must be a startup refusal, not a reconnect loop that never
// carries data — errors.go's split, asserted.
func TestNewStreamRefusesConfigurationBeforeDialling(t *testing.T) {
	base := testConfig("a.com", "/market/ws", "btcusdt@markPrice@1s", ProtocolBinancePath, newSink())

	cases := map[string]func(*StreamConfig){
		"no protocol":          func(c *StreamConfig) { c.Protocol = ProtocolUnspecified },
		"no sink":              func(c *StreamConfig) { c.Sink = nil },
		"no first-data window": func(c *StreamConfig) { c.FirstDataTimeout = 0 },
		"no read timeout":      func(c *StreamConfig) { c.ReadTimeout = 0 },
		"no backoff":           func(c *StreamConfig) { c.Backoff = nil },
		"a negative backoff":   func(c *StreamConfig) { c.Backoff = []time.Duration{-time.Second} },
		"a traversal topic":    func(c *StreamConfig) { c.Topic = ".." },
		"a URL in the host":    func(c *StreamConfig) { c.Endpoint.Host = "wss://a.com/x" },
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := base
			break_(&cfg)
			if _, err := NewStream(cfg); err == nil {
				t.Fatal("NewStream accepted a config that can never carry data")
			}
		})
	}

	// The positive control: the unmodified config is accepted, or the check
	// above is a predicate that objects to everything.
	if _, err := NewStream(base); err != nil {
		t.Fatalf("NewStream refused a valid config: %v", err)
	}
}
