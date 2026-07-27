package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// FundingSource is the §3.5 source a Funding payload arrives on.
//
// # One function rather than a struct literal at each producer
//
// The literal it replaces is events.SourceID{Instrument: inst, Stream:
// events.StreamFunding} — three tokens, one of which is the whole routing
// decision. §3.4 gives every SourceID its own mailbox and events.StreamKind.Lane
// makes exactly this argument one layer down: "routing decided at each call site
// is routing that drifts, and the drift is silent". A producer that wrote
// StreamMark here would publish funding readings onto the mark mailbox, where
// nothing is malformed — the envelope validates, the payload is a real venue
// reading, and the consumer of the mark stream type-asserts and rejects it while
// the funding consumer simply never hears from this instrument again. That
// presents as a dead feed, which §3.4's silence timer would report as a venue
// outage.
//
// # It validates, and the error is events' rather than this package's
//
// The refusal that matters here is §4's perp-only rule, and it is enforced by
// events.SourceID.Validate — see events.StreamKind.RequiresPerp for why it lives
// on the type that holds both fields. This function does not restate it and does
// not rename it: a caller branching on events.ErrStreamProductMismatch is
// branching on the rule's owner, which stays true if a second stream constructor
// lands beside this one.
//
// Validating here at all, rather than returning the value and leaving it to the
// dispatcher's Envelope.Validate, is the boundary argument: a source is
// constructed once per subscription and validated on every envelope. Refusing at
// construction names the subscription that is wrong; refusing at dispatch names
// the millionth message from it.
func FundingSource(inst ids.VenueInstrument) (events.SourceID, error) {
	src := events.SourceID{Instrument: inst, Stream: events.StreamFunding}
	if err := src.Validate(); err != nil {
		return events.SourceID{}, fmt.Errorf("venue funding source: %w", err)
	}
	return src, nil
}
