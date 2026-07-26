package outbox

import "errors"

var (
	// ErrTopicUnset reports a message with no destination. It is separate from
	// ErrTopicInvalid because the two have different causes — an unset topic is
	// a caller that never populated the struct, a malformed one is a caller
	// that built the name wrong — and one sentinel covering both cannot tell an
	// operator which.
	ErrTopicUnset = errors.New("outbox: topic unset")

	// ErrTopicInvalid reports a topic that is too long or holds a character
	// that would not survive the places a topic is read: an indexed store
	// column, a log line, and a §10 metric label.
	ErrTopicInvalid = errors.New("outbox: topic malformed")

	// ErrKeyUnset reports a message with no ordering key.
	//
	// Required rather than optional, and that is the default-deny direction
	// again. Treating an empty key as "no ordering constraint" would mean a
	// producer that simply forgot one gets its events interleaved — silently,
	// and visibly only in the projection, where a fill precedes its own order. A
	// producer that genuinely has no ordering constraint states a key anyway.
	ErrKeyUnset = errors.New("outbox: ordering key unset")

	// ErrKeyInvalid reports a key that is too long or malformed, on the same
	// grounds as ErrTopicInvalid.
	ErrKeyInvalid = errors.New("outbox: ordering key malformed")

	// ErrTypeUnset reports a payload with no schema name. The relay never reads
	// it; the consumer does, and without it the only way to learn a payload's
	// shape is to parse it — which makes routing depend on decoding, in a
	// pipeline whose whole point is to move opaque committed bytes.
	ErrTypeUnset = errors.New("outbox: payload type unset")

	// ErrPayloadEmpty reports a message with no body.
	//
	// Empty is rejected rather than allowed because of how it actually arises:
	// `payload, _ := marshal(x)` with the error dropped yields a nil slice, and
	// an event that reaches a tamper-evident projection carrying no facts is
	// worse than one that never committed.
	ErrPayloadEmpty = errors.New("outbox: payload empty")

	// ErrSeqUnset reports a record whose sequence was never assigned. Seq is
	// the at-least-once dedup key, so a zero one makes every unassigned record
	// look like the same record to a consumer.
	ErrSeqUnset = errors.New("outbox: sequence unassigned")

	// ErrStoreNil, ErrPublisherNil and ErrClockNil report a relay wired without
	// one of its three collaborators. Named separately so the failure says which
	// wire is missing rather than that the relay is unusable.
	ErrStoreNil     = errors.New("outbox: store is nil")
	ErrPublisherNil = errors.New("outbox: publisher is nil")
	ErrClockNil     = errors.New("outbox: clock is nil")

	// ErrBatchInvalid reports a non-positive claim batch. A batch of zero
	// claims nothing, so a relay configured with one runs forever, reports no
	// error, and exports nothing — a stall that looks exactly like an empty
	// outbox.
	ErrBatchInvalid = errors.New("outbox: claim batch must be positive")

	// ErrBackoffInvalid reports a backoff schedule that cannot produce a
	// retry delay: a non-positive base, or a ceiling below the base.
	ErrBackoffInvalid = errors.New("outbox: backoff schedule invalid")
)
