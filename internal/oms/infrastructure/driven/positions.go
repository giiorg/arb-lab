package driven

import (
	"context"
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// VenuePositions is §7.5's position surface as an ACL adapter: engine/venue's
// enumeration translated into this context's port.
//
// A third adapter beside OrderGateway and OrderResolver, on the argument that
// split those two: the use case that asserts the net-delta invariant must not be
// able to place an order or to re-resolve one, so the type it holds has one
// method and that method reads an account.
type VenuePositions struct {
	client VenuePositionReader
	assets map[ids.VenueInstrument]venue.InstrumentAssets
}

// VenuePositionReader is the venue-side capability this adapter needs, declared
// where it is CONSUMED. A composition root satisfies it with *venue.OrderClient
// and never names this interface — Go's interfaces are structural, so no import
// edge is created.
type VenuePositionReader interface {
	Positions(ctx context.Context, scope venue.PositionScope, assets map[ids.VenueInstrument]venue.InstrumentAssets, urgency ratelimit.Priority) (venue.PositionReport, error)
}

// NewVenuePositions builds the adapter.
//
// The assets table is required for OrderResolver's reason and does one more job
// here: it is what decides whether a row the venue reported is one this
// deployment trades. A symbol missing from it becomes a PositionReport.
// Unrecognized entry — §7.5's foreign position — so an EMPTY table would report
// every position the account holds as foreign, which is why the constructor
// refuses one rather than treating it as "nothing declared yet".
//
// # It refuses a table spanning TWO venues, and that is what makes venueOf a fact
//
// This adapter speaks for one venue's client and derives which one from the
// table (see venueOf). Nothing else in the wiring forbids a table naming two:
// the key is an ids.VenueInstrument, so a root that configures a cross-venue
// deployment against one client produces one, and Go's map iteration order then
// decides which venue this adapter believes it is — per boot, and differently on
// two boots of the same binary. The scope check below would then admit a scope
// for venue A on one boot and refuse it on the next, which is the shape this repo
// names a coin flip in the code rather than a flaky test.
//
// So the ambiguity is removed rather than sampled, and the refusal names both
// venues, because a caller told only "mixed" has to go and diff the table.
func NewVenuePositions(client VenuePositionReader, assets map[ids.VenueInstrument]venue.InstrumentAssets) (*VenuePositions, error) {
	if client == nil {
		return nil, ErrVenuePositionReaderMissing
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
			// Sorted into a stable pair so the message does not depend on which one
			// the map happened to yield first — the very nondeterminism this refusal
			// exists to remove.
			a, b := first, instrument.Venue
			if b < a {
				a, b = b, a
			}
			return nil, fmt.Errorf("instrument table names %s and %s: %w", a, b, ErrInstrumentVenuesMixed)
		}
	}
	return &VenuePositions{client: client, assets: assets}, nil
}

// positionsUrgency is the §7.6 shed class this adapter's calls carry.
//
// Safety, on resolveUrgency's argument: §7.5's reconcile is named in §7.6's
// Safety row in as many words, and the enumeration is what the net-delta
// invariant is asserted against — an assertion shed under load is an assertion
// that does not happen, and this system's answer to "the invariant could not be
// checked" is to hold new trading, which frees the very band this call is
// reserved above.
const positionsUrgency = ratelimit.PrioritySafety

// Positions implements ports.VenuePositions.
func (p *VenuePositions) Positions(scope ports.PositionScope) (ports.PositionReport, error) {
	if err := scope.Validate(); err != nil {
		return ports.PositionReport{}, err
	}
	if scope.Venue != p.venueOf() {
		// One adapter speaks for one venue's client, and a scope naming another
		// would be answered by the wrong exchange entirely — the same accident
		// route() forecloses for orders, arriving here through configuration
		// rather than through a symbol.
		return ports.PositionReport{}, fmt.Errorf("scope %s: %w", scope, ErrPositionScopeVenueMismatch)
	}

	// Not cancellable, on Resolve's argument: cancelling this call converts "the
	// venue was about to tell us what it holds" into "the enumeration was not
	// answered", and the second is a refusal to resume trading.
	report, err := p.client.Positions(context.Background(), venue.PositionScope{
		Product: scope.Product,
		Settle:  scope.Settle,
	}, p.assets, positionsUrgency)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and an
		// enumeration that failed must not be re-derived into an empty one.
		return ports.PositionReport{}, err
	}

	out := ports.PositionReport{
		Positions:    make([]ports.VenuePosition, 0, len(report.Positions)),
		Unrecognized: report.Unrecognized,
	}
	for _, row := range report.Positions {
		if row.Instrument.Venue != scope.Venue || row.Instrument.Product != scope.Product {
			// The seam builds the instrument from the scope it was given, so a row
			// outside it means the two layers disagree about what was asked —
			// which is the one error here that produces a confident number for the
			// wrong market.
			return ports.PositionReport{}, fmt.Errorf("position on %s %s answered for scope %s: %w",
				row.Instrument.Venue, row.Instrument.Product, scope, ErrPositionOutsideScope)
		}
		out.Positions = append(out.Positions, ports.VenuePosition{
			Instrument: row.Instrument,
			Net:        row.Net,
		})
	}
	return out, nil
}

// venueOf is which exchange the configured instruments belong to.
//
// Derived from the assets table rather than stored, because the table is what
// the constructor already validated and a second field would be a copy of the
// same fact able to disagree with it.
//
// Returning the first key of a map is only a well-defined function because the
// constructor refuses a table whose instruments do not all share a venue. That
// used to be stated as a property of "every deployment this repo has", which was
// a claim about deployments rather than about the map — and the first deployment
// this repo configures from a FILE is the one that could have broken it, silently
// and differently on each boot. See NewVenuePositions.
func (p *VenuePositions) venueOf() ids.Venue {
	for instrument := range p.assets {
		return instrument.Venue
	}
	return ""
}

// The adapter's own refusals.
var (
	// ErrVenuePositionReaderMissing reports an adapter constructed with nothing
	// behind it.
	ErrVenuePositionReaderMissing = errors.New("oms: venue position reader missing")

	// ErrPositionScopeVenueMismatch reports a scope naming a venue this adapter
	// does not speak for.
	ErrPositionScopeVenueMismatch = errors.New("oms: position scope names a different venue than this adapter")

	// ErrInstrumentVenuesMixed reports an instrument table naming more than one
	// venue, which one venue client cannot answer for and which makes this
	// adapter's own venue a function of map iteration order.
	ErrInstrumentVenuesMixed = errors.New("oms: instrument table names more than one venue")

	// ErrPositionOutsideScope reports a row whose instrument is not in the scope
	// that was asked for.
	ErrPositionOutsideScope = errors.New("oms: venue answered with a position outside the scope requested")
)

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.VenuePositions = (*VenuePositions)(nil)
