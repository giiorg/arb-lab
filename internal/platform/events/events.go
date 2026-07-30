// Package events is build-spec §3.4's typed delivery lanes, and the §3.5
// Envelope they carry.
//
// # Why lanes are typed rather than one bus
//
// §3.4 opens with the reason: "a single trading process needs explicit delivery
// semantics or a slow consumer silently corrupts either latency or the audit
// log." The three lanes here have three incompatible answers to one question —
// what happens when the consumer falls behind — and a single generic channel
// would satisfy all three signatures while satisfying none of the guarantees.
//
//	control        never drops. A lost execution event is lost execution
//	               truth, so the producer blocks instead.
//	market data    never blocks and never grows. A superseded book is worth
//	               less than the newest one, so it coalesces; the perp mark
//	               coalesces adverse-preservingly, which is a different rule.
//	observability  never blocks and never grows either, and resolves the
//	               conflict the opposite way from market data: it drops, and
//	               counts what it dropped.
//
// Choosing wrongly between them is quiet in exactly the way this project cares
// about. An execution event routed onto the market-data lane would be coalesced
// away, and a fill that never arrives is a position the ledger does not know
// about — nothing errors, nothing logs, and the net-delta invariant is asserted
// against a number that is already wrong.
//
// # What is deliberately NOT here
//
// The DISPATCHER is not. §3.5 gives it the job of assigning IngressSeq, of
// arbitrating between sources that become ready at once, and of waking a live
// consumer — and that is engine/dispatch, which composes these lanes from the
// outside. The lanes are its primitives: they carry envelopes and enforce their
// own delivery contract, and none of them assigns a sequence number or decides
// an order between two different sources. (The persisted DispatchSeq log the
// dispatcher replays from is its own slice, in the capture lane — never here.)
//
// The COMMAND SEQUENCER of §3.4's fourth bullet is not here either. It orders
// Risk→OMS commands, and it is NOT built with the dispatcher, because neither a
// Risk context nor an OMS exists to order anything between — a sequencer over
// an empty tree is indistinguishable from a broken one. Its trigger is the
// construct it reads: the first Risk→OMS command path, milestone 4.
//
// # This package holds no clock
//
// Freshness, max-age and the stale-mark timer are §3.4 concerns, and they live
// with the dispatcher and marketdata rather than here, because enforcing an age
// needs a Clock and a timer and the lanes need neither. What the lanes do is
// carry the RecvTime the age will be computed from, and Envelope.Age is a pure
// function of an instant the caller already holds — which is why importing
// platform/clock is unnecessary rather than merely avoided.
package events

import "errors"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
var (
	// ErrStreamUnset reports an empty StreamKind.
	ErrStreamUnset = errors.New("events: stream kind unset")

	// ErrStreamUnknown reports a StreamKind outside the declared set.
	ErrStreamUnknown = errors.New("events: unknown stream kind")

	// ErrStreamProductMismatch reports a SourceID naming a stream the
	// instrument's product family does not have — funding or mark on a spot
	// instrument. See StreamKind.RequiresPerp for why the rule is here rather
	// than left to each consuming context.
	ErrStreamProductMismatch = errors.New("events: stream does not exist for this product")

	// ErrIngressSeqUnset reports an unstamped replay key. 1-based so that the
	// zero value is unusable — §11's rule applied to a number, and it matters
	// more here than usual: an unstamped envelope reading as "the first one"
	// would sort ahead of everything in a replay. There is deliberately no
	// sibling for AcceptOrdinal: zero is that field's absence-derived reading
	// (§3.5), so Validate cannot judge it and a sentinel nothing returns would
	// be a false branch. The producer-side refusals own the input-derived case
	// (application's ErrNoTrigger).
	ErrIngressSeqUnset = errors.New("events: ingress seq unstamped")

	// ErrRecvTimeUnset reports an envelope with no local receive timestamp.
	// This one is ours to stamp, so its absence is a wiring bug rather than a
	// venue that did not send one — see Envelope.SourceTime for the asymmetry.
	ErrRecvTimeUnset = errors.New("events: recv time unset")

	// ErrPayloadNil reports an envelope carrying nothing.
	ErrPayloadNil = errors.New("events: payload is nil")

	// ErrLaneClosed reports a send to, or a receive from, a closed lane. It is
	// what a blocked control-lane producer gets when shutdown overtakes it.
	ErrLaneClosed = errors.New("events: lane closed")

	// ErrWrongLane reports traffic offered to a lane it does not belong on —
	// the misrouting guard. It is a programming error rather than a runtime
	// condition, and it is checked at runtime anyway because the consequence is
	// silent: an execution event accepted onto the market-data lane would be
	// coalesced away, and a fill that never arrives is a position the ledger
	// does not know about.
	ErrWrongLane = errors.New("events: envelope offered to the wrong lane")

	// ErrCoalescePolicyNil reports a market-data lane constructed without a
	// coalescing policy. Refused rather than defaulted — see NewMarketDataLane.
	ErrCoalescePolicyNil = errors.New("events: coalesce policy is nil")

	// ErrLaneCapacity reports a lane constructed with a non-positive capacity.
	// A zero-capacity control lane would make every send wait for a receiver to
	// be already blocked — a rendezvous, not a queue — and a zero-capacity
	// lossy lane would drop everything while reporting success.
	ErrLaneCapacity = errors.New("events: lane capacity must be positive")
)
