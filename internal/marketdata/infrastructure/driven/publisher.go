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
// [SPEC EDIT OWED] §3.5 gives Prio a direction — lower is more urgent — and
// never says whether a source's priority may depend on the CONTENT of what it
// publishes. It may not, and the reason is a safety one that reads backwards:
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
// §3.5 gives the two sequence numbers on an Envelope two different owners.
// AcceptOrdinal is stamped at INGESTION, in raw arrival order across every raw
// input — one authority for the whole process, so an adapter minting its own
// would produce ordinals that collide with another source's and mean nothing
// when the capture lane is replayed. IngressSeq is stamped by the DISPATCHER,
// on the envelope it consumes, and it is the replay key; application/ports.go
// already records why this layer must not invent one, and the argument does not
// change by moving one package outward.
//
// So an envelope leaving here carries neither, and that is a property of WHERE
// it is rather than of what it holds: everything on a market-data mailbox is
// pre-dispatch by definition, because Consume is what dispatching starts with.
// The consequence worth stating plainly is that events.Envelope.Validate will
// refuse an envelope from this mailbox until the dispatcher has stamped it —
// which is correct, and is why Publish does not call it.
//
// [SPEC EDIT OWED] §3.5 says who stamps each sequence number and never says
// WHEN an envelope is required to carry them, which leaves Validate looking
// like an invariant of the type rather than of a position in the pipeline. The
// unsafe reading is available and it is the natural one: a producer that
// validates before publishing cannot satisfy the check without minting both
// values, and a locally-minted AcceptOrdinal is a second authority over raw
// arrival order — ordinals that collide across sources, silently, in the log
// §3.5 requires the RawIngressSeq→DispatchSeq mapping be replayed from. The
// requirement is that a lane holds pre-dispatch envelopes, that both sequence
// fields are unstamped there, and that Validate is the DISPATCHER's post-stamp
// check.
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
// sequence checks do not fire; the probe is discarded and the published
// envelope keeps them zero.
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
