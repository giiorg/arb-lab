package driven

import (
	"context"
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// VenueMarginGateway is core §7.1's margin TOP-UP as an ACL adapter: this
// context's one state-changing call that is not an order (task #215).
//
// A sixth adapter beside OrderGateway, OrderResolver, VenuePositions,
// VenueMarginStatus and VenueMarginActs, on the argument that split those — the
// use case that posts margin must not be able to place an order or enumerate the
// account.
//
// # It is the ONE adapter here that carries no translation
//
// Every other adapter in this package converts between two vocabularies: venue
// rows into MarginReading, an engine cause enum into a domain one, a triple into
// domain.MarginActHistory. This one passes an instrument and a Money through and
// returns an error. That is not an argument for deleting it — what it DOES carry
// is the two facts the use case must not hold, and each is a §7.6 or §4 decision
// rather than a §7.1 one: which shed class a top-up travels under, and which
// asset table types the amount. An application use case reaching engine/venue
// directly to supply either would break §3.6 rule 1 to move a policy into the
// layer that has no business stating it.
type VenueMarginGateway struct {
	client VenueMarginWriter
	assets map[ids.VenueInstrument]venue.InstrumentAssets
}

// VenueMarginWriter is the venue-side capability this adapter needs, declared
// where it is CONSUMED. A composition root satisfies it with *venue.OrderClient
// and never names this interface.
type VenueMarginWriter interface {
	AddIsolatedMargin(ctx context.Context, instrument ids.VenueInstrument, assets venue.InstrumentAssets,
		amount fixedpoint.Money, urgency ratelimit.Priority) error
}

// NewVenueMarginGateway builds the adapter.
//
// The assets table and its two refusals are NewVenueMarginStatus's, unchanged and
// for its reasons: an instrument missing from it cannot be acted on at all,
// because the venue's decimal rendering cannot be typed without its assets, and a
// mixed-venue table lets a caller name venue B's instrument against venue A's
// client and be refused two layers down by route().
//
// Here the second refusal is worth restating in this surface's own terms,
// because it is the only place in this package where getting it wrong SENDS
// something: a mixed table would let a top-up computed for one venue's position
// be signed with the other venue's credential, against a symbol that very
// probably exists there too.
func NewVenueMarginGateway(client VenueMarginWriter, assets map[ids.VenueInstrument]venue.InstrumentAssets) (*VenueMarginGateway, error) {
	if client == nil {
		return nil, ErrVenueMarginWriterMissing
	}
	if len(assets) == 0 {
		return nil, ErrInstrumentAssetsMissing
	}
	var first ids.Venue
	for instrument, a := range assets {
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("instrument %s: %w", instrument.Symbol, err)
		}
		if first == "" {
			first = instrument.Venue
			continue
		}
		if instrument.Venue != first {
			a, b := first, instrument.Venue
			if b < a {
				a, b = b, a
			}
			return nil, fmt.Errorf("instrument table names %s and %s: %w", a, b, ErrInstrumentVenuesMixed)
		}
	}
	return &VenueMarginGateway{client: client, assets: assets}, nil
}

// marginTopUpUrgency is the §7.6 shed class every top-up this adapter sends
// carries.
//
// It is a CONSTANT where marginStatusUrgency is a constructor argument, and the
// difference is not tidiness: §7.6 names this call in the Safety row in as many
// words — "§6.7 ambiguity resolution, §7.1's margin top-up, a flatten" — and,
// unlike the reads beside it, there is no second caller at a second rung. A
// margin read serves both a periodic risk sweep (Routine) and an incident
// (Safety), which is why its class is passed in. A top-up is issued from exactly
// one place, §7.1's Rebalance band, and every value other than Safety would shed
// the remedy at 70% of budget during the incident the reserve exists for.
const marginTopUpUrgency = ratelimit.PrioritySafety

// AddIsolatedMargin implements ports.MarginGateway.
func (g *VenueMarginGateway) AddIsolatedMargin(instrument ids.VenueInstrument, amount fixedpoint.Money) error {
	if err := instrument.Validate(); err != nil {
		return err
	}
	assets, ok := g.assets[instrument]
	if !ok {
		// An instrument this deployment did not declare. VenueMarginActs' refusal
		// with the sharpest consequence of the three: the two available
		// absorptions are guessing the margin asset and sending anyway, and
		// sending is a real transfer against a position on a live account.
		return fmt.Errorf("instrument %s: %w", instrument.Symbol, ErrMarginInstrumentUndeclared)
	}

	// Not cancellable, on VenueMarginStatus's argument with the write path's
	// consequence: cancelling a READ converts "the venue was about to state its
	// margin" into "the margin was not read", while cancelling THIS converts a
	// call that may already have applied into one the caller stops waiting for —
	// and §7.1 forbids the retry that would follow. A context whose cancellation
	// is visible here would be a way to produce an unresolvable op on purpose.
	err := g.client.AddIsolatedMargin(context.Background(), instrument, assets, amount, marginTopUpUrgency)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and §5.3 forbids
		// re-deriving it from the cause. The caller reads the effect and decides
		// what may be concluded — see application.classifyMarginCall.
		return err
	}
	return nil
}

// ErrVenueMarginWriterMissing reports a margin gateway wired with no venue
// client.
//
// Its own sentinel rather than ErrVenueMarginStatusReaderMissing reused, because
// the two say different things about a deployment: a missing READER is a
// deployment that cannot monitor liq distance, and a missing WRITER is one that
// can see the breach and do nothing about it. A root that conflated them would
// diagnose the second as the first and go looking at the wrong wiring.
var ErrVenueMarginWriterMissing = errors.New("oms: venue margin writer missing")

var _ ports.MarginGateway = (*VenueMarginGateway)(nil)
