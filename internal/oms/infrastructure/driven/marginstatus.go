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

// VenueMarginStatus is §7.1's account-side input as an ACL adapter:
// engine/venue's per-position margin read translated into this context's port.
//
// A fourth adapter beside OrderGateway, OrderResolver and VenuePositions, on the
// argument that split those three: the use case that monitors liq distance must
// not be able to place an order, re-resolve one, or enumerate the account, so the
// type it holds has one method and that method reads one position's margin.
type VenueMarginStatus struct {
	client VenueMarginStatusReader
	assets map[ids.VenueInstrument]venue.InstrumentAssets
}

// VenueMarginStatusReader is the venue-side capability this adapter needs,
// declared where it is CONSUMED. A composition root satisfies it with
// *venue.OrderClient and never names this interface — Go's interfaces are
// structural, so no import edge is created.
type VenueMarginStatusReader interface {
	MarginStatus(ctx context.Context, instrument ids.VenueInstrument, assets venue.InstrumentAssets, urgency ratelimit.Priority) (venue.MarginStatus, bool, error)
}

// NewVenueMarginStatus builds the adapter.
//
// # The assets table is per-instrument here, and that changes what an empty one
// # MEANS
//
// VenuePositions refuses an empty table because an empty one reports every
// position the account holds as FOREIGN — a wrong answer that arrives looking
// complete. Here the table is a lookup: an instrument missing from it cannot be
// read at all, because the venue's decimal strings cannot be typed without its
// assets. So an empty table is refused for a different reason — it makes the
// adapter answer nothing, which §7.1 would meet as "this position's liq distance
// is unknown" on every instrument, forever, with every call succeeding at the
// transport.
//
// The mixed-venue refusal is VenuePositions' unchanged, and here it is load
// bearing in a second way: the instrument this method is CALLED with carries its
// own venue, so a mixed table would let a caller name venue B's instrument
// against venue A's client and be refused two layers down by route() — a runtime
// diagnosis for a configuration mistake this constructor can name.
func NewVenueMarginStatus(client VenueMarginStatusReader, assets map[ids.VenueInstrument]venue.InstrumentAssets) (*VenueMarginStatus, error) {
	if client == nil {
		return nil, ErrVenueMarginStatusReaderMissing
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
	return &VenueMarginStatus{client: client, assets: assets}, nil
}

// marginStatusUrgency is the §7.6 shed class this adapter's calls carry.
//
// Safety, and §7.1 says so in as many words twice: the stale-mark failover polls
// this surface "from the reserved rate-limit allocation", and the top-up
// escalation "re-queries margin + position first" before issuing a
// PackageDelever. Both are the incident, not the steady state. Shedding this read
// under load leaves the escalation deciding from a stale before-state during
// exactly the traffic burst that produced the stress.
//
// Pinned here rather than passed through, unlike the engine client's argument:
// the client keeps it an argument because a periodic risk sweep is a legitimate
// Routine caller, and this adapter is the SAFETY one. A second caller wanting
// Routine gets a second adapter with its own constant, which is cheaper than a
// parameter every call site would copy from the one above it.
const marginStatusUrgency = ratelimit.PrioritySafety

// MarginStatus implements ports.VenueMarginStatus.
func (m *VenueMarginStatus) MarginStatus(instrument ids.VenueInstrument) (ports.MarginReading, bool, error) {
	if err := instrument.Validate(); err != nil {
		return ports.MarginReading{}, false, err
	}
	assets, ok := m.assets[instrument]
	if !ok {
		// An instrument this deployment did not declare. Refused rather than
		// reported flat, and the distinction is the whole contract: flat means
		// "the venue holds nothing here", and this means "nobody can say" — the
		// answer §7.1 must escalate on. Reading it as flat would take the trigger
		// off a live short because of a line missing from a config file.
		return ports.MarginReading{}, false, fmt.Errorf("instrument %s: %w",
			instrument.Symbol, ErrMarginInstrumentUndeclared)
	}

	// Not cancellable, on VenuePositions' argument: cancelling converts "the
	// venue was about to state this position's liquidation price" into "the
	// margin read was not answered", and the second is a rung of §7.1's ladder.
	status, held, err := m.client.MarginStatus(context.Background(), instrument, assets, marginStatusUrgency)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and a read that
		// failed must not be re-derived into a flat instrument.
		return ports.MarginReading{}, false, err
	}
	if !held {
		return ports.MarginReading{}, false, nil
	}
	if status.Instrument != instrument {
		// The engine builds the instrument from the one it was given, so a
		// mismatch means the two layers disagree about which market answered —
		// the one error here that yields a confident liquidation price for the
		// wrong position.
		return ports.MarginReading{}, false, fmt.Errorf("margin status for %s answered a read of %s: %w",
			status.Instrument.Symbol, instrument.Symbol, ErrMarginStatusOutsideScope)
	}

	reading := ports.MarginReading{
		Instrument:     status.Instrument,
		LiqPrice:       status.LiqPrice,
		LiqPriceStated: status.LiqPriceStated,
		Maintenance:    status.Maintenance,
		PostedMargin:   status.PostedMargin,
	}
	if err := reading.Validate(); err != nil {
		// The PORT's checks, run here rather than inherited from the engine's.
		// This adapter's reader is an interface, so what reaches this line is
		// whatever satisfies it — the real client validates first, and nothing
		// about the seam requires that.
		//
		// What this catches and what it does not is worth being exact about,
		// because a mutation found the difference. It catches a reading whose
		// LiqPriceStated was lost while its LiqPrice survived — the copy above
		// is field-for-field, and that is the one field whose loss leaves a
		// reading the ENGINE's own status would still validate. It does NOT
		// catch a transposition of the two money fields: both are non-negative
		// and both in the margin asset, so each is legal in the other's slot,
		// and the only thing that can see it is a test asserting the values.
		return ports.MarginReading{}, false, err
	}
	return reading, true, nil
}

// The adapter's own refusals.
var (
	// ErrVenueMarginStatusReaderMissing reports an adapter constructed with
	// nothing behind it.
	ErrVenueMarginStatusReaderMissing = errors.New("oms: venue margin status reader missing")

	// ErrVenueMarginActReaderMissing reports an act-history adapter constructed
	// with no venue reader (task #214). Its own sentinel rather than a shared
	// "reader missing", because the two adapters fail differently: a missing
	// margin-status reader makes every op unresolvable, and a missing act reader
	// would — if it were ever absorbed rather than refused — resolve Binance ops
	// by Bybit's single-source contract while every call succeeded.
	ErrVenueMarginActReaderMissing = errors.New("oms: venue margin act reader missing")

	// ErrMarginInstrumentUndeclared reports a read of an instrument this
	// deployment's assets table does not name — an answer nobody can type, and
	// specifically not a flat position.
	ErrMarginInstrumentUndeclared = errors.New("oms: margin status asked of an instrument this deployment does not declare")

	// ErrMarginStatusOutsideScope reports a status whose instrument is not the
	// one that was asked about.
	ErrMarginStatusOutsideScope = errors.New("oms: venue answered a margin read for a different instrument")
)

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.VenueMarginStatus = (*VenueMarginStatus)(nil)
