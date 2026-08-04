package driven

import (
	"context"
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// OrderGateway is §4's "thin ACL adapter": the translation between this
// context's domain and engine/venue's normalized order surface.
//
// # It exists so that engine/venue does not import oms/domain
//
// §4 puts "raw Binance/Bybit clients and normalized DTOs" in engine/venue and
// has each context consume a segregated capability interface "declared in their
// own application/ports/ and satisfied by thin ACL adapters". The direction
// matters: a venue client written against oms/domain.Order would have to be
// re-derived every time the OMS's model moved for reasons that have nothing to
// do with venues, and no §3.6 gate polices that edge — hexboundary watches
// domain/ and application/ importing engine/, not engine/ importing a context.
//
// So this file is where the two vocabularies meet, and it is the only place they
// do.
//
// # The translation REFUSES more than it converts, and that is the shape
//
// domain.Order is deliberately wider than what this system may send: it holds
// OrderTypeMarket and four TIFs because §4's OpenOrders returns other
// participants' orders and §6 step 10 routes them to operator clearance.
// venue.OrderRequest is deliberately narrower: it has no OrderType field at all,
// so a market order is not expressible past this point. The conversion between a
// wide type and a narrow one is therefore mostly a set of refusals, and each is
// a rule that would otherwise be a comment.
type OrderGateway struct {
	client VenueOrderPlacer
}

// VenueOrderPlacer is the venue-side capability this adapter needs, declared
// where it is CONSUMED rather than imported from engine/venue as a concrete
// type.
//
// A composition root satisfies it by passing *venue.OrderClient and never names
// this interface — Go's interfaces are structural, so no import edge is created.
// module.go warns about that same property doing harm in the other direction (a
// write path escaping the context without an edge singlewriter could see); here
// it is what lets the policed package hold the interface while an unpoliced root
// still supplies the value.
type VenueOrderPlacer interface {
	Place(ctx context.Context, r venue.OrderRequest) (venue.OrderAck, error)

	// ReservePlace stays on the same venue capability as Place because an
	// independent carrier could reserve on one governor and send on another.
	ReservePlace([]venue.OrderRequest) (*ratelimit.Hold, ratelimit.Decision, error)
}

// NewOrderGateway builds the adapter, refusing one with nothing behind it.
func NewOrderGateway(client VenueOrderPlacer) (*OrderGateway, error) {
	if client == nil {
		return nil, ErrVenueOrderPlacerMissing
	}
	return &OrderGateway{client: client}, nil
}

// Place implements ports.OrderPlacer.
//
// # The context is deliberately NOT cancellable, and that is a safety decision
//
// §4's signature carries no context, so this adapter supplies one, and the
// tempting choice — a context wired to process shutdown — is the wrong one.
// Cancelling a Place in flight does not stop the order: it stops us WAITING for
// the answer, converting an outcome the venue was about to tell us into
// EffectUnknown, which §5.3 then resolves with a query and a hold on new
// trading. An operator's Ctrl-C would manufacture exactly the ambiguity the
// whole crash boundary exists to minimise.
//
// What bounds this call instead is SignedClientConfig.Timeout, whose own doc
// already says it is "a safety parameter, not a responsiveness one" — and
// recvWindow beneath it, which bounds how long a delayed submission may still
// execute at the venue after we stopped waiting.
func (g *OrderGateway) Place(o domain.Order, u ports.Urgency, hold *ratelimit.Hold) (ports.Ack, error) {
	const op = "oms.Place"

	req, err := toVenueOrder(o, u)
	if err != nil {
		// Nothing was encoded, so nothing left the process. EffectNotApplied by
		// evidence rather than by default — the one Effect §5.3 forbids
		// inferring from a cause.
		return ports.Ack{}, apperror.Precondition(op, "translate order", err)
	}
	req.Hold = hold

	ack, err := g.client.Place(context.Background(), req)
	if err != nil {
		// Passed through UNWRAPPED-in-effect: engine/venue is the only layer
		// that knows whether the request left the socket, and §5.3 forbids
		// re-deriving that here. apperror.Precondition would overwrite it.
		return ports.Ack{}, err
	}
	return ports.Ack{VenueOrderID: ack.VenueOrderID}, nil
}

// ReservePlacements translates the entire package before asking the one venue
// client to reserve it once. A refusal is returned as an error because nothing
// was sent, while admission expands the one venue hold into the port's per-leg,
// index-aligned shape, including the last leg.
func (g *OrderGateway) ReservePlacements(orders []domain.Order, u ports.Urgency) ([]*ratelimit.Hold, error) {
	reqs := make([]venue.OrderRequest, len(orders))
	for i, order := range orders {
		req, err := toVenueOrder(order, u)
		if err != nil {
			return nil, err
		}
		reqs[i] = req
	}
	hold, decision, err := g.client.ReservePlace(reqs)
	if err != nil {
		return nil, err
	}
	if !decision.Admitted {
		return nil, fmt.Errorf("venue %s limit %q not before %s: %w",
			decision.Venue, decision.Limit, decision.NotBefore, ErrPackageCapacityRefused)
	}
	holds := make([]*ratelimit.Hold, len(orders))
	for i := range holds {
		holds[i] = hold
	}
	return holds, nil
}

// toVenueOrder is the translation, and every refusal in it names its rule.
func toVenueOrder(o domain.Order, u ports.Urgency) (venue.OrderRequest, error) {
	if err := o.Validate(); err != nil {
		return venue.OrderRequest{}, err
	}
	if err := u.Validate(); err != nil {
		return venue.OrderRequest{}, err
	}

	// §6 step 6: "parallel marketable-IOC limit orders (never raw MARKET)".
	// Dispatcher.admit refuses this too, and the duplication is deliberate:
	// that check guards the OMS's own path, and this one guards the boundary
	// where an order becomes bytes — which is also the boundary a future second
	// caller of this adapter would arrive at without passing through admit.
	if o.Type != domain.OrderTypeLimit || o.TimeInForce != domain.TimeInForceIOC {
		return venue.OrderRequest{}, fmt.Errorf("order %q is %s/%s: %w",
			o.ClientOrderID, o.Type, o.TimeInForce, ErrNotSendableAtVenue)
	}

	side, err := toVenueSide(o.Side)
	if err != nil {
		return venue.OrderRequest{}, fmt.Errorf("order %q: %w", o.ClientOrderID, err)
	}

	return venue.OrderRequest{
		Instrument:    o.Instrument,
		ClientOrderID: o.ClientOrderID,
		Side:          side,
		Quantity:      o.Quantity,
		LimitPrice:    o.LimitPrice,
		ReduceOnly:    o.ReduceOnly,
		Urgency:       toRateLimitPriority(u),
	}, nil
}

// toVenueSide maps the domain's direction onto the seam's.
//
// The two enums are byte-identical today, which is exactly why this is a switch
// and not a conversion: `venue.OrderSide(o.Side)` would compile, keep compiling,
// and start lying the moment either side adds or respells a member. §3.6 rule 10
// forces both to be named here, so a new domain direction is a compile error
// rather than a value that crosses the seam meaning nothing.
func toVenueSide(s domain.Side) (venue.OrderSide, error) {
	switch s {
	case domain.SideBuy:
		return venue.OrderSideBuy, nil
	case domain.SideSell:
		return venue.OrderSideSell, nil
	}
	return "", fmt.Errorf("%q: %w", string(s), domain.ErrSideInvalid)
}

// toRateLimitPriority widens the port's two-member class into §7.6's three.
//
// It is total and cannot fail, which is the property the narrowing bought:
// ports.Urgency has no member that maps to PriorityRoutine, so there is no
// rounding decision to be taken here and no third case to get wrong. The
// exhaustive switch is what keeps that true — a member added to Urgency stops
// this file compiling rather than falling through to whichever class is last.
func toRateLimitPriority(u ports.Urgency) ratelimit.Priority {
	switch u {
	case ports.UrgencyOpening:
		return ratelimit.PriorityOpening
	case ports.UrgencySafety:
		return ratelimit.PrioritySafety
	}
	// Unreachable through Place, which validates first. Answering with the
	// UNSPECIFIED value rather than a class is what makes that true rather than
	// merely likely: ratelimit.Request.Validate refuses it, so a path that ever
	// reached here would be refused by the governor before a byte went out —
	// where answering Safety would silently spend the reserve instead.
	return ratelimit.PriorityUnspecified
}

// The adapter's own refusals.
var (
	// ErrVenueOrderPlacerMissing reports an adapter constructed with nothing
	// behind it. It is the constructor's sentinel so that oms.New can report a
	// half-configured root by the name of the wire that is missing.
	ErrVenueOrderPlacerMissing = errors.New("oms: venue order placer missing")

	// ErrNotSendableAtVenue reports an order §6 step 6 does not permit this
	// system to send. Distinct from application.ErrNotMarketableIOCLimit on
	// purpose: that one names a dispatch this context refused, this one names a
	// translation the boundary refused, and an operator seeing the second has
	// learned that something reached the adapter without passing through
	// Dispatch.
	ErrNotSendableAtVenue = errors.New("oms: order is not a marketable-IOC limit and cannot be sent")

	ErrPackageCapacityRefused = errors.New("oms: venue refused package capacity")
)

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.OrderPlacer = (*OrderGateway)(nil)
var _ ports.CapacityReserver = (*OrderGateway)(nil)
