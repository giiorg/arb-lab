package transport

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The three real endpoint shapes from Profiles §5, which is the fixture that
// matters: a test using one host and one prefix cannot tell "the prefix is
// applied" from "the prefix is ignored and this one happened to be empty".
// §35b's whole finding is that the three differ, so the fixture has all three.
func TestDialURLCoversTheThreeBinanceHostShapes(t *testing.T) {
	cases := []struct {
		name  string
		ep    Endpoint
		topic string
		want  string
	}{{
		name:  "futures funding takes the /market prefix",
		ep:    Endpoint{Scheme: "wss", Host: "fstream.binance.com", Prefix: "/market/ws"},
		topic: "btcusdt@markPrice@1s",
		want:  "wss://fstream.binance.com/market/ws/btcusdt@markPrice@1s",
	}, {
		name:  "futures book takes the /public prefix on the SAME host",
		ep:    Endpoint{Scheme: "wss", Host: "fstream.binance.com", Prefix: "/public/ws"},
		topic: "btcusdt@depth20@100ms",
		want:  "wss://fstream.binance.com/public/ws/btcusdt@depth20@100ms",
	}, {
		name:  "spot is a different host, a port, and NEITHER prefix",
		ep:    Endpoint{Scheme: "wss", Host: "stream.binance.com:9443", Prefix: "/ws"},
		topic: "btcusdt@depth20@100ms",
		want:  "wss://stream.binance.com:9443/ws/btcusdt@depth20@100ms",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := binancePath{}.DialURL(c.ep, c.topic)
			if err != nil {
				t.Fatalf("DialURL: %v", err)
			}
			if got != c.want {
				t.Errorf("DialURL() = %q; want %q", got, c.want)
			}
		})
	}
}

// Sources 5 and 6 of §32c have byte-identical topics and are different streams.
// If this ever stops holding, a config keyed on topic has silently merged them.
func TestBybitDialURLSeparatesTwoSourcesWithIdenticalTopics(t *testing.T) {
	const topic = "orderbook.50.BTCUSDT"

	linear, err := bybitV5{}.DialURL(
		Endpoint{Scheme: "wss", Host: "stream.bybit.com", Prefix: "/v5/public/linear"}, topic)
	if err != nil {
		t.Fatalf("DialURL linear: %v", err)
	}
	spot, err := bybitV5{}.DialURL(
		Endpoint{Scheme: "wss", Host: "stream.bybit.com", Prefix: "/v5/public/spot"}, topic)
	if err != nil {
		t.Fatalf("DialURL spot: %v", err)
	}

	if linear == spot {
		t.Fatalf("both sources dialled %q from the same topic — the URL is the only "+
			"thing that separates §32c's sources 5 and 6", linear)
	}
	// And the topic is NOT in either URL, which is what makes that true.
	if strings.Contains(linear, topic) || strings.Contains(spot, topic) {
		t.Errorf("topic leaked into the URL under the message discipline: %q / %q", linear, spot)
	}
}

// TestTopicTraversalIsRefused is §34d's finding applied to its mirror. A topic
// of ".." climbed out of the ARCHIVE tree in slice B; a topic is a path segment
// in a URL too, and the same string does the same thing.
func TestTopicTraversalIsRefused(t *testing.T) {
	ep := Endpoint{Scheme: "wss", Host: "fstream.binance.com", Prefix: "/market/ws"}

	for _, topic := range []string{"..", ".", "../private/ws", "a/b", "/etc/passwd"} {
		t.Run(topic, func(t *testing.T) {
			got, err := binancePath{}.DialURL(ep, topic)
			if err == nil {
				t.Fatalf("DialURL(%q) = %q, no error; a topic that climbs the path "+
					"must be refused where the path is BUILT", topic, got)
			}
			if !errors.Is(err, ErrTopicTraversal) && !errors.Is(err, ErrTopicCharset) {
				t.Errorf("DialURL(%q) error = %v; want a traversal or charset refusal", topic, err)
			}
		})
	}

	// The positive control: a refusal that refuses everything is not a check.
	// Every topic in §32c's capture set must pass, including the ones carrying
	// @ ! . and : — which is why the charset is what it is.
	for _, topic := range []string{
		"btcusdt@markPrice@1s", "btcusdt@depth20@100ms", "btcusdt@depth@100ms",
		"tickers.BTCUSDT", "orderbook.50.BTCUSDT", "!markPrice@arr@1s",
	} {
		if _, err := (binancePath{}).DialURL(ep, topic); err != nil {
			t.Errorf("DialURL(%q) refused a real capture-set topic: %v", topic, err)
		}
	}
}

// The subscribe frame's bytes are archived in the Open record's provenance, so
// they are asserted exactly rather than round-tripped through a decoder — a
// decoder would agree with any field order, and the archive keeps the bytes.
func TestBybitSubscribeFrameBytes(t *testing.T) {
	got := string(bybitV5{}.Subscribe("tickers.BTCUSDT"))
	want := `{"op":"subscribe","args":["tickers.BTCUSDT"],"req_id":"tickers.BTCUSDT"}`
	if got != want {
		t.Errorf("Subscribe() = %s; want %s", got, want)
	}
	if (binancePath{}).Subscribe("btcusdt@markPrice@1s") != nil {
		t.Error("path discipline returned a subscribe frame; nil is how a Stream " +
			"learns there is no ack to wait for")
	}
}

// Classify's table, and the default direction is the point of it. Every row that
// is not a positive recognition must land on Data, INCLUDING bytes that are not
// JSON — because the alternative is this package deciding what history contains.
func TestBybitClassifyDefaultsToData(t *testing.T) {
	cases := []struct {
		name  string
		frame string
		want  Class
	}{
		{"a data frame carries a topic", `{"topic":"tickers.BTCUSDT","type":"snapshot","data":{}}`, ClassData},
		{"the subscribe ack", `{"success":true,"ret_msg":"","op":"subscribe","conn_id":"x"}`, ClassAck},
		{"a refused subscribe is still an ack", `{"success":false,"ret_msg":"Invalid symbol","op":"subscribe"}`, ClassAck},
		{"the ping reply echoes op:ping", `{"success":true,"ret_msg":"pong","op":"ping","conn_id":"x"}`, ClassPong},
		{"some paths answer op:pong", `{"op":"pong","args":["1700000000000"]}`, ClassPong},
		{"an envelope nobody here has seen", `{"op":"something_new","detail":"hello"}`, ClassData},
		{"not JSON at all", `<html>502 Bad Gateway</html>`, ClassData},
		{"empty", ``, ClassData},
		{"a topic AND an op — topic wins", `{"topic":"tickers.BTCUSDT","op":"ping"}`, ClassData},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (bybitV5{}).Classify([]byte(c.frame)); got != c.want {
				t.Errorf("Classify(%s) = %s; want %s", c.frame, got, c.want)
			}
		})
	}
}

// AckResult's default runs the OPPOSITE way from Classify's, and the two
// directions are asserted together so nobody "fixes" the asymmetry later.
func TestBybitAckResultDefaultsToNotSubscribed(t *testing.T) {
	cases := []struct {
		name   string
		frame  string
		wantOK bool
	}{
		{"accepted", `{"success":true,"ret_msg":"","op":"subscribe"}`, true},
		{"refused with a reason", `{"success":false,"ret_msg":"Invalid symbol","op":"subscribe"}`, false},
		{"refused with no reason", `{"success":false,"op":"subscribe"}`, false},
		{"no success field", `{"op":"subscribe","conn_id":"x"}`, false},
		{"unparseable", `{`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, detail := bybitV5{}.AckResult([]byte(c.frame))
			if ok != c.wantOK {
				t.Errorf("AckResult(%s) ok = %v; want %v (detail %q)", c.frame, ok, c.wantOK, detail)
			}
			if !ok && detail == "" {
				t.Error("a refusal with no detail: the venue's own words are the " +
					"most useful thing in the archive six weeks later")
			}
		})
	}

	// The refusal carries the VENUE's sentence, not ours.
	if _, detail := (bybitV5{}).AckResult([]byte(`{"success":false,"ret_msg":"Invalid symbol"}`)); detail != "Invalid symbol" {
		t.Errorf("detail = %q; want the venue's own %q", detail, "Invalid symbol")
	}
}

// binancePath.AckResult is unreachable through a Stream. It is asserted anyway,
// because "unreachable" is a claim about today's caller.
func TestBinancePathAckResultIsTotal(t *testing.T) {
	ok, detail := binancePath{}.AckResult([]byte(`{"success":true}`))
	if ok {
		t.Error("AckResult reported a subscription the path discipline never made")
	}
	if detail == "" {
		t.Error("AckResult gave no reason")
	}
}

func TestEndpointValidate(t *testing.T) {
	cases := []struct {
		name string
		ep   Endpoint
		want error
	}{
		{"ok", Endpoint{Scheme: "wss", Host: "a.com", Prefix: "/x"}, nil},
		{"ok, empty prefix means root", Endpoint{Scheme: "wss", Host: "a.com"}, nil},
		{"ok, host with a port", Endpoint{Scheme: "wss", Host: "a.com:9443"}, nil},
		{"no scheme", Endpoint{Host: "a.com"}, ErrSchemeUnset},
		{"http is not a websocket scheme", Endpoint{Scheme: "https", Host: "a.com"}, ErrSchemeUnknown},
		{"no host", Endpoint{Scheme: "wss"}, ErrHostUnset},
		{"the paste error: a whole URL in the host field",
			Endpoint{Scheme: "wss", Host: "wss://a.com/market/ws"}, ErrHostMalformed},
		{"a path in the host field", Endpoint{Scheme: "wss", Host: "a.com/x"}, ErrHostMalformed},
		{"prefix with no leading slash", Endpoint{Scheme: "wss", Host: "a.com", Prefix: "x"}, ErrPrefixMalformed},
		{"prefix with a trailing slash", Endpoint{Scheme: "wss", Host: "a.com", Prefix: "/x/"}, ErrPrefixMalformed},
		{"prefix carrying a query", Endpoint{Scheme: "wss", Host: "a.com", Prefix: "/x?y=1"}, ErrPrefixMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.ep.Validate()
			if c.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v; want nil", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v; want %v", err, c.want)
			}
		})
	}
}

func TestProtocolForNamesEveryDialect(t *testing.T) {
	for _, id := range []ProtocolID{ProtocolBinancePath, ProtocolBybitV5} {
		p, err := ProtocolFor(id)
		if err != nil {
			t.Fatalf("ProtocolFor(%s): %v", id, err)
		}
		if p.ID() != id {
			t.Errorf("ProtocolFor(%s).ID() = %s", id, p.ID())
		}
	}
	if _, err := ProtocolFor(ProtocolUnspecified); !errors.Is(err, ErrProtocolUnknown) {
		t.Errorf("ProtocolFor(zero) = %v; want ErrProtocolUnknown — a blank config "+
			"field must refuse rather than default into somebody's dialect", err)
	}
	if _, err := ProtocolFor("kraken-v3"); !errors.Is(err, ErrProtocolUnknown) {
		t.Errorf("ProtocolFor(unknown) = %v; want ErrProtocolUnknown", err)
	}
}

// The keepalive interval is a venue fact (Profiles §5: "every 20 seconds"), so
// it is asserted rather than left to drift.
func TestKeepAliveMatchesTheDocumentedHeartbeat(t *testing.T) {
	frame, every := bybitV5{}.KeepAlive()
	if string(frame) != `{"op":"ping"}` {
		t.Errorf("KeepAlive frame = %s; want {\"op\":\"ping\"}", frame)
	}
	if every != 20*time.Second {
		t.Errorf("KeepAlive interval = %v; want 20s per Profiles §5", every)
	}
	if f, _ := (binancePath{}).KeepAlive(); f != nil {
		t.Error("binance declared an application keepalive; the library answers " +
			"its protocol pings, and a second thing that can pong while the feed " +
			"is dead is exactly what §31g rule 4 warns about")
	}
}

// backoff walks the schedule and HOLDS at the last entry rather than running off
// the end or wrapping to the start. Wrapping would silently reset a stuck stream
// to a 100 ms retry loop against a venue that is refusing it.
func TestBackoffWalksThenHolds(t *testing.T) {
	s := &Stream{cfg: StreamConfig{Backoff: []time.Duration{
		10 * time.Millisecond, 50 * time.Millisecond, 200 * time.Millisecond,
	}}}
	want := []time.Duration{
		10 * time.Millisecond, 50 * time.Millisecond, 200 * time.Millisecond,
		200 * time.Millisecond, 200 * time.Millisecond,
	}
	for i, w := range want {
		if got := s.backoff(i); got != w {
			t.Errorf("backoff(%d) = %v; want %v", i, got, w)
		}
	}
}
