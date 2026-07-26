package outbox

import (
	"fmt"
	"time"
)

// Topic names a consumer stream — D2's audit, observability and tax exports.
//
// It is an OPEN string key rather than §3.6 rule 8's closed enum, taking the
// same exception clock.TimerID takes and for the same reason: the set of names
// is open. A closed enum here would repeat §10a's mistake in a new place —
// adding a consumer would become a kernel edit, and a kernel that enumerates its
// consumers is one every context has to be listed in.
//
// What is enforced instead is a FORMAT, because a topic is written to an indexed
// store column, printed in log lines, and used as a §10 metric label. A newline
// in a metric label is not a formatting problem; it is a log line that parses as
// two.
type Topic string

// Key is the per-message ordering key: FIFO is preserved within one Key and
// deliberately not across two (see the package doc). Open and format-checked for
// the same reasons as Topic.
//
// In practice it is a TradeID for OMS events, but the type is not ids.TradeID:
// the outbox exports observability and tax events too, and narrowing the key to
// a trade would force those producers to invent a trade that does not exist.
type Key string

// MaxTopicLen and MaxKeyLen bound the two persisted identifiers.
//
// Bounded rather than free so that a malformed producer cannot write an
// unbounded string into what will be an indexed column in every store that
// implements the port. The specific numbers are not derived from anything — they
// are comfortably above every identifier this system mints (a Key is a 26-char
// Crockford TradeID plus a prefix) and comfortably below any index limit.
const (
	MaxTopicLen = 64
	MaxKeyLen   = 128
)

// Message is what a producer hands to the store, inside the business
// transaction. It carries no sequence number, because the sequence is what the
// store assigns when that transaction commits.
//
// # On retaining the payload
//
// A Message's Payload is serialized by the store during the append and is not
// retained by this package. A Record's Payload, by contrast, IS retained across
// the publish attempt — that contract is stated on Store.Claim, where it binds.
type Message struct {
	// Topic routes the message to a consumer.
	Topic Topic

	// Key is the ordering key. See the package doc on per-key FIFO.
	Key Key

	// Type names the payload's schema. The relay never reads it; consumers do.
	Type string

	// Payload is the event body as it was committed.
	//
	// Bytes rather than `any`, and the difference from events.Envelope.Payload
	// is deliberate. An Envelope is dispatched in-process and never persisted as
	// a value, so it can carry a typed Go object. A Record crossed a durability
	// boundary: these are the exact bytes that committed alongside the journal
	// write, and the audit projection they feed is hash-chained (§7.5, Z4).
	// Holding a Go object and re-marshalling it at delivery time would let the
	// exported bytes differ from the committed ones — a map iteration order or a
	// struct-tag change is enough — which breaks a hash chain in a way that
	// reads as tampering.
	Payload []byte
}

// Validate reports whether the message could be appended.
//
// Checked in the order a reader would debug: where it goes, how it is ordered,
// what it is, then what it says.
func (m Message) Validate() error {
	if err := m.Topic.Validate(); err != nil {
		return err
	}
	if err := m.Key.Validate(); err != nil {
		return err
	}
	if m.Type == "" {
		return ErrTypeUnset
	}
	if len(m.Payload) == 0 {
		return ErrPayloadEmpty
	}
	return nil
}

// Validate reports whether the topic is a usable name.
func (t Topic) Validate() error {
	if t == "" {
		return ErrTopicUnset
	}
	if len(t) > MaxTopicLen {
		return fmt.Errorf("topic %q is %d bytes, limit %d: %w",
			string(t), len(t), MaxTopicLen, ErrTopicInvalid)
	}
	if i := indexNotIdentifier(string(t)); i >= 0 {
		return fmt.Errorf("topic %q holds %q at byte %d: %w",
			string(t), string(t)[i], i, ErrTopicInvalid)
	}
	return nil
}

func (t Topic) String() string { return string(t) }

// Validate reports whether the key is a usable ordering key.
func (k Key) Validate() error {
	if k == "" {
		return ErrKeyUnset
	}
	if len(k) > MaxKeyLen {
		return fmt.Errorf("ordering key %q is %d bytes, limit %d: %w",
			string(k), len(k), MaxKeyLen, ErrKeyInvalid)
	}
	if i := indexNotIdentifier(string(k)); i >= 0 {
		return fmt.Errorf("ordering key %q holds %q at byte %d: %w",
			string(k), string(k)[i], i, ErrKeyInvalid)
	}
	return nil
}

func (k Key) String() string { return string(k) }

// indexNotIdentifier returns the byte index of the first character outside the
// permitted set, or -1.
//
// The set is an allow-list rather than a deny-list of "whitespace and control
// characters", and that direction is the point: a deny-list admits every
// character nobody thought to name, which over a persisted, logged, metric-
// labelled identifier is the wrong way for the omission to fail.
func indexNotIdentifier(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == ':':
			continue
		}
		return i
	}
	return -1
}

// Record is a Message that has been durably appended: the Message plus the
// facts the store owns.
type Record struct {
	// Seq is the durable, store-assigned sequence. 1-based; zero means
	// unassigned, matching events.Envelope's convention for its two stamps.
	//
	// It is the at-least-once DEDUP KEY (see the package doc). It is also
	// monotonic per store, which is what makes a claim cursor possible — but it
	// is NOT the per-key order by itself; the relay derives that from Seq within
	// a Key, and two keys' sequences interleave freely.
	Seq uint64

	Message

	// Attempts is how many delivery attempts this record has already survived.
	// Zero on a record that has never been claimed.
	//
	// Carried on the record rather than counted in the relay because the relay
	// is stateless across passes by construction — a restarted process must not
	// reset a poison record's backoff to zero and start hammering a consumer
	// that has been rejecting it for an hour.
	Attempts uint32

	// EnqueuedAt is when the appending transaction committed, from the injected
	// clock. It is what an export-lag metric is measured from; the relay itself
	// does not read it.
	EnqueuedAt time.Time
}

// Validate reports whether the record is one the store could have produced.
func (r Record) Validate() error {
	if r.Seq == 0 {
		return ErrSeqUnset
	}
	return r.Message.Validate()
}

// String renders the record for logs WITHOUT its payload.
//
// The omission is deliberate and matches §10f's ruling on config's
// ErrNotReloadable: an outbox payload is an OMS event, it may carry account
// identifiers and sizes, and a String that interpolated it would put those into
// every log line that formatted a record with %v.
func (r Record) String() string {
	return fmt.Sprintf("outbox record %d %s/%s type=%s attempts=%d",
		r.Seq, r.Topic, r.Key, r.Type, r.Attempts)
}
