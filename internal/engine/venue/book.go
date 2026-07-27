package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// BookSource is the §3.5 source a book payload arrives on.
//
// # Written because FundingSource exists, which is this repo's mirror rule
//
// The argument for a constructor rather than a struct literal at each producer
// is FundingSource's and is not restated here: routing decided at each call site
// is routing that drifts. What IS worth stating is the difference, because it is
// the one a reader will assume away.
//
// # Book is the stream that is NOT perp-only, and that is the whole point of it
//
// §3.5's product table makes StreamFunding and StreamMark perpetual-only and
// StreamBook available to both families, and events.StreamKind.RequiresPerp is
// where that binds. So this constructor accepts a spot instrument where
// FundingSource refuses one — and it must, because the spot book is what §4.2
// says the spot leg is priced from. A copy of FundingSource with the stream
// token swapped would still be correct here, which is exactly why the asymmetry
// deserves a sentence: the next stream constructor written by analogy to this
// one inherits "both families" from a function that got it by accident.
//
// The refusal that remains is the ordinary one — an instrument that does not
// validate — and it is events.SourceID.Validate's, not renamed. A caller
// branching on events.ErrStreamProductMismatch keeps branching on the rule's
// owner.
func BookSource(inst ids.VenueInstrument) (events.SourceID, error) {
	src := events.SourceID{Instrument: inst, Stream: events.StreamBook}
	if err := src.Validate(); err != nil {
		return events.SourceID{}, fmt.Errorf("venue book source: %w", err)
	}
	return src, nil
}
