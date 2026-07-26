package events

import "fmt"

// Lane names one of §3.4's three delivery lanes.
//
// It exists as a value, separate from the three concrete lane types, because
// routing has to be decidable BEFORE a lane is chosen — StreamKind.Lane answers
// "where does this belong" without holding a queue. Keeping the answer in one
// place is what stops per-call-site routing from drifting.
//
// Integer enum rather than §3.6 rule 8's string shape, taking the same
// documented exception as fixedpoint.RoundingMode and apperror.Effect, for the
// same reason: a Lane is a routing argument, never persisted and never
// transmitted — what reaches the capture lane is the StreamKind, which IS a
// string enum — so the property that matters is that the zero value be an
// unusable sentinel rather than that it be self-describing.
type Lane uint8

const (
	// LaneUnspecified is the zero value and names no lane. Unlike
	// apperror.EffectUnspecified it is not safe to act on, because there is no
	// conservative lane: routing an unclassified event to market data may drop
	// it, and routing everything to control may block trading on a metrics
	// backlog. So nothing defaults to this — StreamKind.Lane answers
	// LaneControl for an unrecognised stream rather than answering this.
	LaneUnspecified Lane = iota

	// LaneControl is lossless and ordered: private order, execution, account,
	// liquidation and ADL events, plus OMS commands. Never dropped; feeds the
	// single-writer OMS in a documented total order.
	LaneControl

	// LaneMarketData is bounded and latest-value: book, mark and funding
	// updates in per-VenueInstrument mailboxes. Superseded updates may be
	// coalesced — after reconstruction only, and adverse-preservingly for the
	// perp mark.
	LaneMarketData

	// LaneObservability is isolated and lossy: metrics, logs and traces.
	// Back-pressure here never blocks trading, which is the guarantee, not a
	// concession.
	LaneObservability
)

// maxLane is the last declared value. Validate compares against it, so adding a
// lane means extending this — which the declared-set test checks.
const maxLane = LaneObservability

// Validate reports whether l is one of the declared lanes, excluding the
// unspecified sentinel.
func (l Lane) Validate() error {
	if l == LaneUnspecified || l > maxLane {
		return fmt.Errorf("events: lane %d is not a declared lane", uint8(l))
	}
	return nil
}

// String implements fmt.Stringer.
//
// The shape the deferred exhaustiveswitch gate (§3.6 rule 10) exists to police:
// a lane added above falls through and prints as a number. Survivable in a
// Stringer; it would not be on the routing switch in StreamKind.Lane, which is
// why that one has no default branch reachable by a declared value.
func (l Lane) String() string {
	switch l {
	case LaneUnspecified:
		return "LaneUnspecified"
	case LaneControl:
		return "LaneControl"
	case LaneMarketData:
		return "LaneMarketData"
	case LaneObservability:
		return "LaneObservability"
	}
	return fmt.Sprintf("Lane(%d)", uint8(l))
}
