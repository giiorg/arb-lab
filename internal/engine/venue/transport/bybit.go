package transport

import (
	"encoding/json"
	"time"
)

// bybitPingInterval is Bybit's documented client heartbeat: "send the ping
// heartbeat packet every 20 seconds to maintain the WebSocket connection"
// (Profiles §5). A constant, not a clock read — notimenow polices reading the
// clock, and a duration is a number.
const bybitPingInterval = 20 * time.Second

// bybitV5 is the message discipline (Profiles §5).
//
//	wss://stream.bybit.com/v5/public/linear   perp funding + perp book
//	wss://stream.bybit.com/v5/public/spot     spot book
//
// The topic never appears in the URL — it goes in a subscribe frame after the
// handshake — which is why sources 5 and 6 of §32c's capture set have
// BYTE-IDENTICAL topic strings (orderbook.50.BTCUSDT on both) and are
// distinguished only by the path. A capture keyed on topic alone would merge
// them into one stream and nothing would report it.
//
// Unlike the path discipline this one CAN detect its own misconfiguration: a bad
// topic is refused with success:false and a message. That does not make §32k's
// first-data assertion redundant here — an accepted subscription to a live topic
// that then goes quiet is still indistinguishable from a quiet market — but it
// does mean a typo fails with a venue-supplied reason instead of a timeout.
type bybitV5 struct{}

// Compile-time proof, §3.6 rule 2's shape.
var _ Protocol = bybitV5{}

func (bybitV5) ID() ProtocolID { return ProtocolBybitV5 }

// DialURL is the endpoint alone. The topic is still validated, because it is
// about to become a JSON argument, an archive directory and an observability
// label value, and refusing it in one of those places and not the others is how
// a rule stops being statable from the config.
func (bybitV5) DialURL(ep Endpoint, topic string) (string, error) {
	if err := ep.Validate(); err != nil {
		return "", err
	}
	if err := validateTopic(topic); err != nil {
		return "", err
	}
	return ep.String(), nil
}

// bybitRequest is an outbound op frame. Field order is declaration order in
// encoding/json, so the bytes are deterministic — which matters because these
// exact bytes are archived in the Open record's provenance and a test asserts
// them.
type bybitRequest struct {
	Op    string   `json:"op"`
	Args  []string `json:"args,omitempty"`
	ReqID string   `json:"req_id,omitempty"`
}

// Subscribe builds {"op":"subscribe","args":[topic],"req_id":topic}.
//
// req_id is the topic rather than a counter or a UUID, for two reasons that
// point the same way. Bybit echoes req_id on the ack, so a human reading the
// archive six weeks later sees which subscription was acknowledged without
// cross-referencing anything. And it is deterministic, so the provenance bytes
// for a given source are identical across runs and a test can assert them
// without a clock or a random source in the picture.
func (bybitV5) Subscribe(topic string) []byte {
	// A struct with no channel, func or map field cannot fail to marshal, so
	// the error is structurally impossible rather than merely unlikely.
	// Returning nil on it would silently mean "path discipline, expect no ack",
	// which is the one wrong answer available here.
	frame, err := json.Marshal(bybitRequest{Op: "subscribe", Args: []string{topic}, ReqID: topic})
	if err != nil {
		panic("transport: bybit subscribe frame failed to marshal: " + err.Error())
	}
	return frame
}

// bybitEnvelope is the four fields Classify and AckResult need. Everything else
// in the frame — including the whole data payload — is decoded into nothing and
// then thrown away, which is what "reads the envelope, never the payload" means
// concretely.
type bybitEnvelope struct {
	Topic   string `json:"topic"`
	Op      string `json:"op"`
	Success *bool  `json:"success"`
	RetMsg  string `json:"ret_msg"`
}

// Classify reads the envelope.
//
// Every branch that is not a positive recognition falls to Data, INCLUDING a
// frame that is not JSON at all. That is the package default and it is the
// direction that is right when somebody forgets: an envelope Bybit introduces
// next year reaches the archive verbatim, where it is still evidence, instead of
// being dropped by the component whose job is to not drop things.
func (bybitV5) Classify(frame []byte) Class {
	var env bybitEnvelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return ClassData
	}
	// A topic present means market data. Checked FIRST because it is the only
	// branch on the hot path, and because a data frame never carries an op.
	if env.Topic != "" {
		return ClassData
	}
	switch env.Op {
	case "subscribe":
		return ClassAck
	case "ping", "pong":
		// Both spellings: the reply to our {"op":"ping"} echoes op:"ping" with
		// ret_msg:"pong" on the public streams, and some paths answer with
		// op:"pong" directly. Recognising one and not the other would let the
		// unrecognised one satisfy the first-data assertion.
		return ClassPong
	}
	return ClassData
}

// AckResult reports whether the venue accepted the subscription.
//
// An unparseable or success-less ack reports NOT ok. That is the opposite
// direction from Classify's default and deliberately so: the two defaults answer
// different questions. Classify asks "should this be kept?", where the safe
// answer is yes. This asks "may we now believe we are subscribed?", where the
// safe answer is no — believing a subscription that does not exist produces
// exactly the silent empty archive §32k exists to prevent.
func (bybitV5) AckResult(frame []byte) (bool, string) {
	var env bybitEnvelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return false, "unparseable subscribe ack: " + err.Error()
	}
	if env.Success == nil {
		return false, "subscribe ack carried no success field"
	}
	if !*env.Success {
		if env.RetMsg == "" {
			return false, "venue refused the subscription without a reason"
		}
		return false, env.RetMsg
	}
	return true, env.RetMsg
}

// KeepAlive is Bybit's documented JSON heartbeat.
func (bybitV5) KeepAlive() ([]byte, time.Duration) {
	frame, err := json.Marshal(bybitRequest{Op: "ping"})
	if err != nil {
		panic("transport: bybit ping frame failed to marshal: " + err.Error())
	}
	return frame, bybitPingInterval
}
