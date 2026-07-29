package transport

import (
	"fmt"
	"time"
)

// Class is what an inbound frame is, decided from its ENVELOPE alone.
//
// Three members and not more, because the set is not a taxonomy of venue
// messages — it is the answer to two operational questions: does this frame
// count as market data, and does it satisfy §32k's first-data assertion. Both
// answers are the same answer, and everything that is not positively recognised
// as protocol noise gets it.
type Class string

const (
	// ClassData is a venue data frame, and it is the DEFAULT — see the package
	// doc. An envelope nobody here has seen is Data, so it reaches the archive
	// verbatim rather than being discarded by the one component in the pipeline
	// whose entire purpose is not discarding things.
	ClassData Class = "data"

	// ClassAck is the venue's reply to our subscribe request. It is excluded
	// from Data because it already has a home — §32g puts the request and the
	// ack in the Open record's provenance — and because counting it as data
	// would let the first-data assertion be satisfied by the handshake, one
	// message before any market data exists.
	ClassAck Class = "ack"

	// ClassPong is the venue's reply to OUR keepalive ping. Excluded because a
	// liveness assertion answered by our own noise is not a liveness assertion:
	// a feed that has silently stopped still pongs, forever, and §31g rule 4 is
	// precisely the failure of not being able to tell that apart from a quiet
	// market.
	ClassPong Class = "pong"
)

// Validate reports whether c is a declared class.
func (c Class) Validate() error {
	switch c {
	case ClassData, ClassAck, ClassPong:
		return nil
	default:
		return fmt.Errorf("class %q: %w", c, ErrProtocolUnknown)
	}
}

func (c Class) String() string { return string(c) }

// ProtocolID names a venue's websocket dialect, and it is the config-facing
// spelling. It names a DIALECT rather than a venue, which the Binance member's
// name is deliberate about: Binance also accepts a SUBSCRIBE message on its
// /stream paths, and that would be a different member here rather than a flag on
// this one. A dialect a source does not use is a dialect nobody has to reason
// about.
type ProtocolID string

const (
	// ProtocolUnspecified is the zero value and names nothing — a config field
	// left blank refuses at startup rather than defaulting into somebody's
	// dialect.
	ProtocolUnspecified ProtocolID = ""

	// ProtocolBinancePath is the raw-stream path discipline:
	// wss://<host><prefix>/<topic>, no subscribe frame, no ack. Covers both the
	// futures host under its /market and /public prefixes and the spot host,
	// which takes neither (Profiles §5).
	ProtocolBinancePath ProtocolID = "binance-path"

	// ProtocolBybitV5 is the message discipline: connect to the complete path,
	// send {"op":"subscribe","args":[topic]}, wait for the ack, ping every 20 s.
	ProtocolBybitV5 ProtocolID = "bybit-v5"
)

// Validate reports whether p is a declared protocol, excluding the zero value.
func (p ProtocolID) Validate() error {
	switch p {
	case ProtocolUnspecified:
		return ErrProtocolUnknown
	case ProtocolBinancePath, ProtocolBybitV5:
		return nil
	default:
		return fmt.Errorf("protocol %q: %w", p, ErrProtocolUnknown)
	}
}

func (p ProtocolID) String() string { return string(p) }

// Protocol is one venue dialect. Implementations are stateless values: a Stream
// holds one and a reconnect reuses it, so a dialect cannot accumulate state that
// survives a socket it was supposed to be independent of.
type Protocol interface {
	// ID is the config-facing name.
	ID() ProtocolID

	// DialURL builds the URL for this endpoint and topic. Under the path
	// discipline the topic is a path segment; under the message discipline it
	// is absent and the URL is the endpoint alone.
	DialURL(ep Endpoint, topic string) (string, error)

	// Subscribe is the frame to send once the socket is up, or nil when the
	// discipline put the topic in the URL. A nil return also means there is no
	// ack to wait for.
	Subscribe(topic string) []byte

	// Classify reads the envelope, never the payload, and never re-encodes: the
	// frame the sink receives is the same slice that came off the socket
	// whatever this returns.
	//
	// It takes no topic, and that absence is a decision. Narrowing Data to
	// "frames whose envelope topic equals the one we subscribed to" would drop
	// venue data on a string mismatch — a venue that normalises the case of an
	// echoed topic would silently empty the archive — and dropping data is the
	// one thing this pipeline may not do. One socket carries one subscription
	// here, so the narrowing buys nothing against that risk.
	Classify(frame []byte) Class

	// AckResult interprets a frame Classify called ClassAck. detail is for the
	// operator and is never branched on.
	AckResult(frame []byte) (ok bool, detail string)

	// KeepAlive is the frame to send periodically and how often, or nil when
	// the websocket protocol's own ping/pong suffices. Binance sends protocol
	// pings and the library answers them; Bybit documents a JSON ping.
	KeepAlive() (frame []byte, every time.Duration)
}

// ProtocolFor resolves a config-facing name to a dialect.
//
// The switch names every member, which exhaustiveswitch requires and which
// matters more here than the rule's usual case: a member added without a case
// would fall to the default and report "unknown protocol" for a protocol this
// package declares — a refusal whose message is a lie about its own cause.
func ProtocolFor(id ProtocolID) (Protocol, error) {
	switch id {
	case ProtocolUnspecified:
		return nil, ErrProtocolUnknown
	case ProtocolBinancePath:
		return binancePath{}, nil
	case ProtocolBybitV5:
		return bybitV5{}, nil
	default:
		return nil, fmt.Errorf("protocol %q: %w", id, ErrProtocolUnknown)
	}
}
