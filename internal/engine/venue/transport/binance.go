package transport

import "time"

// binancePath is the raw-stream path discipline (Profiles §5).
//
//	wss://fstream.binance.com/market/ws/btcusdt@markPrice@1s   funding
//	wss://fstream.binance.com/public/ws/btcusdt@depth20@100ms  futures book
//	wss://stream.binance.com:9443/ws/btcusdt@depth20@100ms     spot book
//
// Three hosts, three path disciplines, one dialect. The prefix differences live
// in the Endpoint (see its doc); what is common — and what makes this ONE
// protocol rather than three — is that the topic is a path segment, there is no
// subscribe frame, and there is therefore nothing to acknowledge.
//
// # The consequence that makes §32k non-negotiable
//
// With no subscribe and no ack, a successful handshake proves the HOST is alive
// and proves nothing whatever about the subscription. The legacy prefix-less
// paths decommissioned 2026-04-23 accept the connection and then send nothing,
// forever, which is why a mistyped prefix here is invisible to every check
// except the first-data timeout. This protocol cannot detect its own
// misconfiguration; §32k's assertion is the only thing that can.
type binancePath struct{}

// Compile-time proof, §3.6 rule 2's shape.
var _ Protocol = binancePath{}

func (binancePath) ID() ProtocolID { return ProtocolBinancePath }

// DialURL appends the topic as a path segment. Both parts are validated even
// though a caller reaching here has usually validated already — this is the
// function that turns a topic into a path, so it is where a traversal segment
// has to be refused, not one layer up where somebody can forget.
func (binancePath) DialURL(ep Endpoint, topic string) (string, error) {
	if err := ep.Validate(); err != nil {
		return "", err
	}
	if err := validateTopic(topic); err != nil {
		return "", err
	}
	return ep.String() + "/" + topic, nil
}

// Subscribe is nil: the topic was in the URL. A nil return is also how a Stream
// learns there is no ack to wait for, so the two facts cannot get out of step.
func (binancePath) Subscribe(string) []byte { return nil }

// Classify is unconditionally Data. On a raw stream path Binance sends stream
// payloads and nothing else — no envelope, no ack, no application-level
// keepalive. Returning Data for everything is therefore the accurate answer
// rather than a fallback, and it is the same answer the package default gives.
func (binancePath) Classify([]byte) Class { return ClassData }

// AckResult is unreachable through a Stream, because Subscribe returns nil and a
// Stream that was given no subscribe frame waits for no ack. It is a total
// function anyway and is tested directly, because "unreachable" is a claim about
// today's caller and a method that lies when someone reaches it is worse than
// one that refuses.
func (binancePath) AckResult([]byte) (bool, string) {
	return false, "binance path discipline sends no subscribe ack"
}

// KeepAlive is nil: Binance sends websocket protocol ping frames and the library
// answers them automatically. An application-level keepalive on top would add a
// second thing that can pong while the feed is dead.
func (binancePath) KeepAlive() ([]byte, time.Duration) { return nil, 0 }
