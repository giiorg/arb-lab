package events

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// StreamKind names a kind of feed from a venue.
//
// §3.6 rule 8's `type X string` shape, and the deciding question is rule 8's
// own: does the value cross a persistence or wire boundary? It does — a SourceID
// is written to §3.5's capture lane, where the RawIngressSeq→DispatchSeq mapping
// is "explicit and persisted" and replay re-applies it. A renamed token orphans
// the log it has to be replayed against.
type StreamKind string

const (
	// StreamBook carries reconstructed order-book snapshots. Note
	// "reconstructed": §3.4 forbids coalescing or dropping RAW snapshot+delta
	// traffic, because dropping a delta desyncs the book. Deltas are applied
	// losslessly with gap detection and resnapshot inside marketdata, and what
	// reaches a lane under this StreamKind is the resulting immutable snapshot.
	StreamBook StreamKind = "book"

	// StreamMark carries the perp mark price. It is the one market-data stream
	// with a coalescing rule of its own — see AdversePreserving — because liq
	// distance is a function of mark, mark moves with no account event, and a
	// brief adverse excursion that breached a threshold must not be hidden by a
	// later recovery `[→ R2c-1]`.
	StreamMark StreamKind = "mark"

	// StreamFunding carries funding rate, interval and next-settlement
	// metadata. §3.4 says funding metadata tolerates a longer freshness budget
	// than a book does.
	StreamFunding StreamKind = "funding"

	// StreamExecutions carries private fills. Control lane: a lost fill is a
	// position the ledger does not know about.
	StreamExecutions StreamKind = "executions"

	// StreamAccount carries private account, position, margin, liquidation and
	// ADL updates. Control lane, same reason.
	StreamAccount StreamKind = "account"

	// StreamControl carries our own commands rather than anything a venue sent.
	// It is a StreamKind so that a command and a venue event can share one
	// ordered lane and one replay key, which is what §3.5 means by a documented
	// total order "over all inputs".
	StreamControl StreamKind = "control"
)

// Validate reports whether s is one of the declared values.
func (s StreamKind) Validate() error {
	switch s {
	case StreamBook, StreamMark, StreamFunding,
		StreamExecutions, StreamAccount, StreamControl:
		return nil
	case "":
		return ErrStreamUnset
	}
	return fmt.Errorf("%q: %w", string(s), ErrStreamUnknown)
}

func (s StreamKind) String() string { return string(s) }

// Lane reports which §3.4 lane this stream belongs on.
//
// The assignment is a property of the STREAM rather than of whoever publishes
// it, which is the point: routing decided at each call site is routing that
// drifts, and the drift is silent. A private execution event misrouted onto the
// market-data lane gets coalesced away, and a fill that vanishes leaves the
// net-delta invariant asserting against a number that is already wrong.
//
// An unrecognised stream answers LaneControl. That default is chosen rather than
// inherited: the control lane neither drops nor coalesces, so an unclassified
// stream is delivered intact. The opposite default would silently thin a feed
// nobody had gotten around to classifying — data loss as the consequence of an
// omission, which is the failure mode this package exists to prevent.
func (s StreamKind) Lane() Lane {
	switch s {
	case StreamBook, StreamMark, StreamFunding:
		return LaneMarketData
	}
	return LaneControl
}

// SourceID identifies one stream of one instrument — §3.5's "a VenueInstrument
// stream". It is NOT globally ordered `[→ Z1]`: VenueSeq is authoritative within
// a SourceID and means nothing across two.
//
// [SPEC EDIT LANDED — v0.9.6] §3.5 declared this as four flat fields —
// SourceID{Venue, Product ProductCategory, Symbol string, Stream StreamKind} —
// with the trailing comment "= a VenueInstrument stream". This composes the
// instrument instead of restating its three fields. Two reasons, and the second
// is the real one: the flat form has no way to express the ContractType that
// §5.1's VenueInstrument carries, so a source could not name an inverse contract
// at all; and duplicating the fields duplicates their validation, which is how
// two definitions of "a valid instrument" come to disagree. The spec's own
// comment said this IS a VenueInstrument stream; §3.5 now makes it structural.
type SourceID struct {
	Instrument ids.VenueInstrument
	Stream     StreamKind
}

// Validate reports whether the source could exist.
func (s SourceID) Validate() error {
	if err := s.Instrument.Validate(); err != nil {
		return err
	}
	return s.Stream.Validate()
}

// Lane reports the lane this source's traffic belongs on.
func (s SourceID) Lane() Lane { return s.Stream.Lane() }

// String renders the source for logs. Like VenueInstrument.String it is not
// claimed to be injective; the identity is the struct, which is comparable and
// is used directly as a mailbox key.
func (s SourceID) String() string {
	return fmt.Sprintf("%s/%s", s.Instrument, s.Stream)
}

// Prio orders envelopes within a lane.
//
// Lower is MORE urgent, so the zero value is the most urgent — the same
// direction and the same reasoning as clock's timer queue, and it is deliberate
// that the two agree. An envelope priority and a timer priority disagreeing
// about direction would put a safety timer and a safety event at opposite ends
// of the same drain, which is the sort of defect that only shows up in the
// incident it makes worse.
//
// Under the opposite convention an unset priority sends a safety event to the
// back of the queue. Noisy-but-early beats quiet-but-late.
//
// uint8 rather than rule 8's string shape, following §3.5's own signature: a
// priority is an ordering argument, not a stored token, so what matters is that
// the zero value be the safe one rather than that it be self-describing.
type Prio uint8

const (
	// PrioSafety is the most urgent, and it is zero so that an envelope nobody
	// classified is treated as urgent rather than deprioritised.
	PrioSafety Prio = 0

	// PrioNormal is ordinary trading and market-data traffic.
	PrioNormal Prio = 128
)

// MoreUrgentThan is a strict order: a value is not more urgent than itself.
func (p Prio) MoreUrgentThan(other Prio) bool { return p < other }

// Envelope is §3.5's unit of dispatch and of replay.
//
// # The two sequence numbers are not redundant
//
// This is the subtlest thing in the package and the easiest to "simplify" into a
// defect, so it is stated here rather than left to the spec:
//
//   - AcceptOrdinal (a.k.a. RawIngressSeq) is stamped in raw ARRIVAL order at
//     ingestion, on every raw input. It is the causal and audit order — you
//     reacted to what you received, in the order you received it — and it is the
//     capture lane's authority.
//   - IngressSeq (a.k.a. DispatchSeq) is stamped on the DISPATCHED envelope in
//     the order the state machine actually consumes it. It is the REPLAY KEY.
//
// They genuinely differ, for two reasons §3.5 spells out. Reconstruction may
// HOLD a book delta behind a sequence gap while an independent source stays
// dispatchable — forcing strict global arrival order would head-of-line-block
// that independent source. And coalescing means some raw inputs produce no
// dispatched envelope at all — forcing it would require skipping ordinals.
// Replay re-applies IngressSeq, because determinism keys on what the state
// machine actually CONSUMED, not on what arrived.
//
// # SourceTime is not an ordering key, ever
//
// Two venues stamp with unsynchronized clocks, so sorting a merged stream by
// SourceTime can MANUFACTURE an incorrect order. It is carried for freshness,
// cross-leg skew and offline analysis only. Within a SourceID, VenueSeq is
// authoritative; across SourceIDs there is no temporal merge in the MVP.
//
// Fields are exported for capture-lane persistence and replay hydration.
type Envelope struct {
	// AcceptOrdinal is the raw arrival order. 1-based; zero means unstamped.
	AcceptOrdinal uint64

	// IngressSeq is the consumed order and the replay key. 1-based; zero means
	// unstamped.
	IngressSeq uint64

	Source SourceID

	// SourceTime is the venue's own timestamp, and MAY be absent — not every
	// message carries one. Requiring it would make adapters invent one, and an
	// invented venue timestamp is worse than an absent one because it would be
	// silently subtracted from a real one in a cross-leg skew metric. Use
	// HasSourceTime before reading it.
	SourceTime time.Time

	// RecvTime is when WE received it, from the injected clock. Unlike
	// SourceTime it is required: it is ours to stamp, so its absence is a wiring
	// bug, and it is what freshness is measured from.
	RecvTime time.Time

	// VenueSeq is the venue's sequence number. Authoritative WITHIN this
	// Source and meaningless across two — it can collide across symbols.
	//
	// Not validated: plenty of streams do not carry one, and a venue is free to
	// start at zero. Gap detection is marketdata's job, per SourceID, where the
	// expected next value is known.
	VenueSeq uint64

	Prio Prio

	// Payload is the typed event. `any` follows §3.5's own signature: the lanes
	// are generic over payload and the contexts type-assert what they
	// subscribed to.
	Payload any
}

// Validate reports whether the envelope is one that could have been dispatched.
//
// Checked in the order a reader would debug: the source, then the two stamps
// that make it replayable, then the timestamp freshness is measured from, then
// the payload.
func (e Envelope) Validate() error {
	if err := e.Source.Validate(); err != nil {
		return err
	}
	if e.AcceptOrdinal == 0 {
		return ErrAcceptOrdinalUnset
	}
	if e.IngressSeq == 0 {
		return ErrIngressSeqUnset
	}
	if e.RecvTime.IsZero() {
		return ErrRecvTimeUnset
	}
	if e.Payload == nil {
		return ErrPayloadNil
	}
	return nil
}

// HasSourceTime reports whether the venue supplied a timestamp.
func (e Envelope) HasSourceTime() bool { return !e.SourceTime.IsZero() }

// Age reports how stale the envelope is at now, measured from RecvTime.
//
// From RecvTime rather than SourceTime, and that is the whole point of the
// method existing rather than callers subtracting inline. §3.4 gates execution
// on freshness and drives liq-distance off a stale-mark budget; §3.6 rule 5
// spells out what goes wrong when two clocks end up either side of that
// subtraction. RecvTime comes from the injected clock, whose monotonic clamp is
// the guarantee being relied on. The venue's own offset is a separately modelled
// quantity — clock.ServerNow's uncertainty band — not something to fold into an
// age silently.
//
// A reading before RecvTime reports zero rather than a negative duration. A
// negative age reads as "fresher than now" and passes every max-age check ever
// written, which under a backwards step is exactly how a stale mark keeps gating
// execution. Zero is the conservative answer: it says "as fresh as possible",
// which is the claim a caller can act on safely, and any real staleness still
// shows up as soon as the clock recovers.
func (e Envelope) Age(now time.Time) time.Duration {
	if now.Before(e.RecvTime) {
		return 0
	}
	return now.Sub(e.RecvTime)
}
