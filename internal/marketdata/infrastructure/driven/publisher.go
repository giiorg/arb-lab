package driven

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// fundingPrio is the priority every funding envelope is published at.
//
// # It is a constant, and the fact that it does not vary with the verdict is
// # the decision
//
// [SPEC EDIT LANDED — v0.9.8] §3.5 gave Prio a direction — lower is more
// urgent — and never said whether a source's priority may depend on the CONTENT
// of what it publishes. It may not, and the reason is a safety one that reads
// backwards; §3.5 now states the constancy as a separate requirement from the
// direction, because a spec that fixes only the direction leaves the reordering
// below available to anyone who reads it and complies fully:
// §3.4's mailboxes are per-SourceID and §3.5 makes VenueSeq authoritative
// within one, so two envelopes from one source published at different
// priorities are two envelopes that can be drained out of sequence. The
// tempting implementation escalates a FreshnessBlind snapshot because it fires
// §5.2's mark_blind_action — and the consequence of that reordering is that the
// blind reading arrives BEFORE the stale one it followed, so a consumer sees a
// degrading feed recover. A per-source constant is the requirement; which
// constant is a policy choice.
//
// The tempting alternative is to escalate a FreshnessBlind snapshot: it is the
// rung that fires §5.2's mark_blind_action, so it looks like safety traffic.
// Two reasons it is not done. §3.4 puts funding metadata explicitly below the
// execution path — "funding metadata tolerates longer" — and the mark, which is
// what liq-distance is actually computed from, is a different stream. And a
// priority derived from the verdict would REORDER two snapshots of one SourceID
// against each other, where §3.5 makes VenueSeq authoritative: a degrading
// feed's later, worse reading would overtake its earlier one inside the mailbox
// they share, and a consumer would see freshness recover.
//
// Not the zero value, and for events.Prio's own stated reason: PrioSafety is 0
// so that an envelope nobody classified is treated as urgent. Taking it here by
// default would put funding ahead of the safety kernel in a drain because
// nobody chose — the same argument application's silence timer records for the
// timer queue, and the two agreeing is deliberate.
const fundingPrio = events.PrioNormal

// LanePublisher is application.FundingPublisher over §3.4's market-data lane.
//
// # What it builds, and what it deliberately leaves unstamped
//
// §3.5 gives the two sequence numbers on an Envelope two different owners, and
// the difference between them decides what this adapter may write.
//
// IngressSeq is stamped by the DISPATCHER, on the envelope it consumes, and it
// is the replay key. This layer does not know the consumption order, so
// stamping one here would INVENT a value — application/ports.go records the
// argument, and it does not change by moving one package outward. It leaves.
//
// AcceptOrdinal is stamped at INGESTION, in raw arrival order across every raw
// input, one authority for the whole process. That authority is UPSTREAM of
// here, which is what makes it different in kind: the snapshot carries the
// ordinal of the input that produced it, so writing it is INHERITING a value,
// not minting a second one. §3.5 requires it — the dispatched envelope carries
// the ordinal of what triggered it, for causal traceability — and it is stamped
// here because this is the last place that knows both the snapshot and the
// envelope. Reading the no-sequences rule as covering both is what this file
// did until the reading pass, and the cost was a published snapshot no replay
// could tie to its cause.
//
// So an envelope leaving here carries one of the two. That is still a property
// of WHERE it is: everything on a market-data mailbox is pre-dispatch, because
// Consume is what dispatching starts with, and the missing IngressSeq is what
// says so. events.Envelope.Validate will therefore refuse an envelope from this
// mailbox until the dispatcher has stamped it — correct, and why Publish does
// not call it directly.
//
// [SPEC EDIT LANDED — v0.9.8] §3.5 said who stamps each sequence number and
// never said WHEN an envelope is required to carry them, which left Validate
// looking like an invariant of the type rather than of a position in the
// pipeline. The unsafe reading was available and it was the natural one: a
// producer that validates before publishing cannot satisfy the check without
// minting an IngressSeq, and a locally-minted replay key is a second authority
// over the order determinism keys on. §3.5's Envelope field comments now say it
// per field — AcceptOrdinal present from ingestion onward, IngressSeq assigned
// only at dispatch — and the paragraph states that Validate is the DISPATCHER's
// post-stamp check rather than a producer-side one.
//
// [SPEC EDIT LANDED — v0.9.8] §3.5's causal-traceability sentence stated the
// inheritance rule for an envelope derived from a raw input, and had no case for
// one derived from the ABSENCE of an input. §3.4's silence timer produces
// exactly that, so the rule needed SCOPING rather than extending. §3.5 now says
// the rule binds every producer and not only book reconstruction — the
// reconstructed snapshot was an example being read as the extent, which is how
// this file came to publish zero — that a timer-triggered envelope carries no
// ordinal and zero is how it says so, and that reusing the last input's ordinal
// is forbidden: it makes the capture log claim one raw input produced two
// dispatched envelopes.
//
// # Publishing is not fire-and-forget here either
//
// The lane never blocks, so Publish can only fail by refusing. It refuses two
// distinct things, and keeping them distinct is what makes the error name the
// layer that owns the mistake — see Publish.
type LanePublisher struct {
	lane *events.MarketDataLane
}

// NewLanePublisher binds the adapter to a lane, refusing a nil one.
//
// The lane's coalescing policy is NOT a parameter here. It is chosen where the
// lane is constructed, which is app/, because it is a claim about how every
// source on that lane behaves — the perp mark coalesces adverse-preservingly
// and a book snapshot does not, and one publisher cannot answer for both.
func NewLanePublisher(lane *events.MarketDataLane) (*LanePublisher, error) {
	if lane == nil {
		return nil, ErrLaneNil
	}
	return &LanePublisher{lane: lane}, nil
}

// Publish puts a snapshot on its instrument's funding mailbox.
//
// # Two checks, two owners, and the order is what makes both reachable
//
// The first asks whether the snapshot can be ADDRESSED on the lane — a source
// that identifies an instrument, a receive time, a payload. That is this
// adapter's own concern, and it is asked by validating a probe rather than by
// restating events.Envelope's field list, so a required field added to the
// envelope tomorrow is caught here instead of silently skipped. The probe
// carries fabricated sequence numbers ONLY so that Validate's two unstamped-
// sequence checks do not fire, and it is discarded: the published envelope
// keeps its IngressSeq zero and its inherited AcceptOrdinal.
//
// BOTH are forced on the probe, including the one the envelope really carries.
// A snapshot from the silence timer legitimately has a zero ordinal, so probing
// with the real value would make Publish refuse precisely the envelope §3.4's
// timer exists to emit — a feed dying is the one case that would go unreported.
//
// The second asks whether the snapshot is a real READING — which the domain
// owns, and which covers the one thing the envelope cannot: SourceTime is
// optional on a lane, because not every venue message carries one, while
// domain.Watermark requires it, because an observation with no venue stamp
// cannot answer DataAge and the venue-side stall is precisely what that
// measurement exists to catch. An unvalidated snapshot would therefore reach
// the lane looking perfectly well-formed and be undiagnosable downstream.
//
// A refused snapshot does not reach the mailbox. Both checks run before
// anything is delivered, so an error here means nothing was published — a
// publisher that reported failure after delivering would leave a consumer
// reading a record its producer believes it rejected.
func (p *LanePublisher) Publish(snap application.FundingSnapshot) error {
	w := snap.Observation.Watermark

	env := events.Envelope{
		// Inherited, not minted. §3.5 requires the dispatched envelope to carry
		// the ordinal of the input that triggered it, and the use case carried
		// it here for exactly that. Zero is a legitimate value and means the
		// snapshot came from a silence timer — from no input at all — which is
		// why this is copied unconditionally rather than guarded.
		AcceptOrdinal: uint64(snap.Trigger),

		Source:     events.SourceID{Instrument: snap.Observation.Instrument, Stream: events.StreamFunding},
		SourceTime: w.SourceTime,
		RecvTime:   w.RecvTime,
		VenueSeq:   w.VenueSeq,
		Prio:       fundingPrio,
		Payload:    snap,
	}

	probe := env
	probe.AcceptOrdinal, probe.IngressSeq = 1, 1
	if err := probe.Validate(); err != nil {
		return fmt.Errorf("marketdata funding publish: %w", err)
	}
	if err := snap.Observation.Validate(); err != nil {
		return fmt.Errorf("marketdata funding publish %s: %w", snap.Observation.Instrument, err)
	}

	return p.lane.Publish(env)
}
