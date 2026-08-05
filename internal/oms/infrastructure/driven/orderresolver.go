package driven

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// OrderResolver is §4's thin ACL adapter for the READ half — the translation
// between engine/venue's normalized order surface and this context's domain.
//
// It is a second adapter beside OrderGateway rather than one wider type, for the
// reason ports.OrderResolver is a second port: the resolution path must not be
// able to place an order, and a single adapter carrying both would put the
// method one line away from the code whose subject is not calling it.
type OrderResolver struct {
	client  VenueOrderReader
	assets  map[ids.VenueInstrument]venue.InstrumentAssets
	urgency ratelimit.Priority
}

// VenueOrderReader is the venue-side capability this adapter needs, declared
// where it is CONSUMED. A composition root satisfies it with *venue.OrderClient
// and never names this interface — Go's interfaces are structural, so no import
// edge is created. Same shape and same reason as VenueOrderPlacer.
type VenueOrderReader interface {
	OrderByClientID(ctx context.Context, ref venue.OrderRef, assets venue.InstrumentAssets, urgency ratelimit.Priority) (venue.OrderStatus, error)
	Executions(ctx context.Context, instrument ids.VenueInstrument, assets venue.InstrumentAssets, w venue.ExecutionWindow, urgency ratelimit.Priority) (venue.ExecutionPage, error)
}

// NewOrderResolver builds the adapter.
//
// # The assets table is required, and its absence would be a Money whose asset
// # tag is a lie
//
// The read path MINTS money from venue strings, and fixedpoint refuses to build
// a Quantity or a Price with no asset tag. ids.VenueInstrument does not carry
// the decomposition — Symbol is "the venue's own routing symbol", deliberately
// unparsed, because splitting it is a per-venue convention and a wrong split
// produces a Money whose tag the runtime check then enforces against everything
// else, failing far from here. So the pairs are stated by whoever configures the
// instruments this system trades.
//
// It is a map rather than a parser for that reason, and it refuses an instrument
// it has no entry for rather than guessing. §5.1's SymbolSpec is the construct
// that will own this — domain.Order.Validate already names it as the thing that
// would let quantity and price assets be checked against the instrument — and
// when it lands this table is what it replaces.
func NewOrderResolver(client VenueOrderReader, assets map[ids.VenueInstrument]venue.InstrumentAssets, urgency ratelimit.Priority) (*OrderResolver, error) {
	if client == nil {
		return nil, ErrVenueOrderReaderMissing
	}
	if len(assets) == 0 {
		return nil, ErrInstrumentAssetsMissing
	}
	for instrument, a := range assets {
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("instrument %s: %w", instrument.Symbol, err)
		}
	}
	if err := urgency.Validate(); err != nil {
		return nil, fmt.Errorf("order resolver urgency: %w", err)
	}
	return &OrderResolver{client: client, assets: assets, urgency: urgency}, nil
}

// The §7.6 shed class every call from this adapter carries is CONSTRUCTOR-
// pinned since task #179 (record §70f), where it was a const.
//
// The const's argument — "this adapter has exactly one caller with exactly one
// purpose" — expired when `arblab baseline record` became a second caller: an
// operator maintenance command whose join re-runs Resolve and whose sweep can
// issue up to 32 pages per order, which the Safety pin would spend out of the
// emergency reserve. §5.3's rule still holds for the boot path — the resolving
// queries are SAFETY traffic and boot wiring passes exactly that — while the
// record command's wiring passes Routine, so a maintenance sweep is shed
// before the reserve rather than out of it. The zero value is refused at
// construction, so a wiring that forgot to choose does not silently inherit
// either rung.

// maxSweepPages bounds the paging loop below.
//
// The bound exists because §5.3 requires rate-limit budget to be RESERVED for
// recovery: an unbounded loop is how a single resolution spends the reserve that
// every other resolution needs. engine/venue refuses to loop internally for the
// same reason and hands the decision here.
//
// It is also the hang bound. A venue that keeps answering with a full page and a
// continuation makes this loop run as long as the venue is willing to talk, and
// this repo has recorded what that looks like from a test: not a failure, a
// TIMEOUT, which names no defect. Exceeding it is not an error — the sweep
// returns what it PROVED, and the caller compares that watermark against the
// horizon it asked for. A truncated sweep is therefore reported as a sweep that
// proved less, which is exactly what it is.
const maxSweepPages = 32

// Resolve implements ports.OrderResolver.
func (r *OrderResolver) Resolve(o domain.Order) (ports.VenueOrder, error) {
	assets, err := r.assetsFor(o.Instrument)
	if err != nil {
		return ports.VenueOrder{}, err
	}

	// The context is deliberately not cancellable, on OrderGateway.Place's
	// argument narrowed to this path: cancelling a resolving query converts "the
	// venue was about to tell us" into "the query was not answered", which §5.3
	// turns into a hold on new trading. An operator's Ctrl-C would manufacture
	// the ambiguity this call exists to remove.
	status, err := r.client.OrderByClientID(context.Background(), venue.OrderRef{
		Instrument:    o.Instrument,
		ClientOrderID: o.ClientOrderID,
	}, assets, r.urgency)
	if err != nil {
		// Passed through unwrapped: engine/venue already classified whether this
		// was a refusal, a timeout or a rate limit, and §5.3 forbids re-deriving
		// it here.
		return ports.VenueOrder{}, err
	}

	presence, err := toPresence(status.Presence)
	if err != nil {
		return ports.VenueOrder{}, fmt.Errorf("order %q: %w", o.ClientOrderID, err)
	}
	reach, err := toReach(status.Reach)
	if err != nil {
		return ports.VenueOrder{}, fmt.Errorf("order %q: %w", o.ClientOrderID, err)
	}

	resolved := ports.VenueOrder{
		Presence:     presence,
		Reach:        reach,
		VenueOrderID: status.VenueOrderID,
	}
	if presence == ports.VenueHasNoOrder {
		// Every remaining field is meaningless for an absence, and leaving them
		// at their zero values is what the port's "read Presence FIRST" contract
		// describes. Translating the venue's zero state would produce a
		// domain.OrderState the caller could branch on for an order that does
		// not exist.
		return resolved, nil
	}

	resolved.State, err = toDomainState(status.State)
	if err != nil {
		return ports.VenueOrder{}, fmt.Errorf("order %q: %w", o.ClientOrderID, err)
	}
	resolved.Executed = status.ExecutedQuantity
	return resolved, nil
}

// SweepExecutions implements ports.OrderResolver.
//
// # The loop advances on the venue's own continuation, never on a rebuilt one
//
// engine/venue returns the next window as a whole ExecutionWindow rather than a
// bare token, precisely so that a caller cannot reconstruct one wrongly: Bybit's
// page cursor walks BACKWARD into older data while a resume instant moves
// forward, and the two are both "a string you hand to the next call". Handing
// back exactly what was returned is the only correct loop, and it is why this
// one carries no cursor arithmetic of its own.
func (r *OrderResolver) SweepExecutions(o domain.Order, since, until time.Time) (ports.ExecutionSweep, error) {
	assets, err := r.assetsFor(o.Instrument)
	if err != nil {
		return ports.ExecutionSweep{}, err
	}

	window := venue.ExecutionWindow{Since: venue.Cursor{At: since}, Until: until}
	sweep := ports.ExecutionSweep{}

	for pages := 0; pages < maxSweepPages; pages++ {
		page, err := r.client.Executions(context.Background(), o.Instrument, assets, window, r.urgency)
		if err != nil {
			return ports.ExecutionSweep{}, err
		}
		for _, e := range page.Executions {
			sweep.Executions = append(sweep.Executions, ports.VenueExecution{
				ExecutionID:   e.ExecutionID,
				ClientOrderID: e.ClientOrderID,
				VenueOrderID:  e.VenueOrderID,
				Quantity:      e.Quantity,
				At:            e.At,

				// Carried since slice F2, when the port stopped being wide enough
				// only to ANSWER a question about a row and became wide enough to
				// admit one as a fill. The venue's Side is deliberately still not
				// carried: direction lives on the order (domain.Fill's
				// single-statement-of-direction rule), and a side beside it would
				// be a second place for a sign error to live.
				Price: e.Price,
				Fee:   e.Fee,
			})
		}

		// Taken from every page rather than only the last, because the loop can
		// exit three ways — exhausted, page budget spent, venue done — and only
		// the watermark of the page we actually read is a claim about coverage.
		sweep.ProvenThrough = page.Watermark.At
		if page.More == nil {
			return sweep, nil
		}
		window = *page.More
	}

	// The budget ran out. This is a successful call reporting less coverage, not
	// a failure: ProvenThrough says how far we got, the caller compares it
	// against its horizon, and SM §2 step 3 refuses to collapse an interval on a
	// watermark that fell short. Returning an error here would lose the
	// executions already read.
	return sweep, nil
}

// assetsFor is the configured decomposition, refused rather than guessed.
func (r *OrderResolver) assetsFor(instrument ids.VenueInstrument) (venue.InstrumentAssets, error) {
	assets, ok := r.assets[instrument]
	if !ok {
		return venue.InstrumentAssets{}, fmt.Errorf("%s %s %s: %w",
			instrument.Venue, instrument.Product, instrument.Symbol, ErrInstrumentAssetsUnconfigured)
	}
	return assets, nil
}

// toPresence maps the seam's reading onto the port's.
//
// A switch and not a conversion, on toVenueSide's argument: the two enums spell
// the same distinction differently today and would keep compiling if either side
// respelled a member. §3.6 rule 10 forces both to be named.
func toPresence(p venue.OrderPresence) (ports.VenuePresence, error) {
	switch p {
	case venue.OrderPresent:
		return ports.VenueHasOrder, nil
	case venue.OrderAbsent:
		return ports.VenueHasNoOrder, nil
	}
	return "", fmt.Errorf("%q: %w", string(p), ports.ErrVenuePresenceInvalid)
}

// toReach carries both horizons across and drops nothing.
//
// It does NOT combine them, and that is the boundary between an adapter and a
// rule: which retention bounds an absence is a policy — the resolver takes the
// minimum, and says why — while this layer's job is to state what the venue
// published. A pre-combining adapter would hide a safety rule in a translation,
// where no test about the rule would look for it.
func toReach(r venue.QueryReach) (ports.SurfaceReach, error) {
	if err := r.Validate(); err != nil {
		return ports.SurfaceReach{}, err
	}
	if err := r.Surface.Validate(); err != nil {
		return ports.SurfaceReach{}, err
	}
	return ports.SurfaceReach{
		Surface:           r.Surface.String(),
		UnfilledRetention: r.UnfilledRetention,
		FilledRetention:   r.FilledRetention,
		Published:         !r.Unpublished,
	}, nil
}

// toDomainState maps engine/venue's normalized lifecycle onto SM §1's.
//
// # The interesting row is Terminated → StateCanceled
//
// engine/venue refuses a Canceled/Expired split because the two venues make it
// unnormalizable — the same real event (an IOC that did not fill in full) is
// Binance EXPIRED, Bybit linear Cancelled and Bybit spot PartiallyFilledCanceled
// — so its terminal-without-rejection member is one state. SM §1 has one member
// for it too, and its evidence rule is satisfied by the caller rather than here:
// StateCanceled needs "an explicit venue-terminal cancel PLUS the final
// executions sweep", and Resolver.present runs the sweep before recording any
// terminal.
//
// # Open → StateAcknowledged, and it is a widening this layer is entitled to
//
// The venue says "accepted and still able to execute", which is exactly what SM
// §1 means by Acknowledged. The narrower reading — that Acknowledged requires
// having SEEN the ack — is not available on this path: the resolving query runs
// precisely because the ack was not read, and the venue holding a live order is
// better evidence than the message we lost.
func toDomainState(s venue.OrderState) (domain.OrderState, error) {
	switch s {
	case venue.OrderStateOpen:
		return domain.StateAcknowledged, nil
	case venue.OrderStatePartiallyFilled:
		return domain.StatePartiallyFilled, nil
	case venue.OrderStateFilled:
		return domain.StateFilled, nil
	case venue.OrderStateTerminated:
		return domain.StateCanceled, nil
	case venue.OrderStateRejected:
		return domain.StateRejected, nil
	}
	return "", fmt.Errorf("%q: %w", string(s), venue.ErrOrderStateInvalid)
}

// The adapter's own refusals.
var (
	// ErrVenueOrderReaderMissing reports an adapter constructed with nothing
	// behind it, on ErrVenueOrderPlacerMissing's terms.
	ErrVenueOrderReaderMissing = errors.New("oms: venue order reader missing")

	// ErrInstrumentAssetsMissing reports an adapter constructed with no
	// base/quote decompositions at all — a resolver that could not mint money
	// for any instrument, which is a wiring mistake rather than a per-call one.
	ErrInstrumentAssetsMissing = errors.New("oms: order resolver has no instrument asset decompositions")

	// ErrInstrumentAssetsUnconfigured reports a call about an instrument the
	// table has no entry for. Distinct from the constructor's sentinel because
	// the two send a reader to different places: that one to the composition
	// root, this one to the instrument list it was given.
	ErrInstrumentAssetsUnconfigured = errors.New("oms: no base/quote decomposition is configured for this instrument")
)

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.OrderResolver = (*OrderResolver)(nil)
