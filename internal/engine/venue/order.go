package venue

import (
	"context"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is §4's OrderGateway.Place, venue-side: the normalized request a
// context hands the seam, and the client that routes it to one of two venues
// and two product families.
//
// # It does NOT speak the OMS's types, and that is the layering rather than a
// # preference
//
// §4 gives engine/venue "raw Binance/Bybit clients and normalized DTOs" and has
// contexts consume "segregated capability interfaces declared in their own
// application/ports/ and satisfied by thin ACL adapters". So the translation
// from oms/domain.Order to OrderRequest happens in the OMS's own driven adapter,
// and this package never imports a bounded context.
//
// Nothing enforces that. hexboundary polices domain/ and application/ importing
// engine/ — this is the OTHER direction, engine/ importing a context's domain,
// and no gate has an opinion about it. The edge would compile, the tests would
// pass, and what would be lost is the property that makes a second venue cheap:
// an adapter written against oms/domain is an adapter that must be re-derived
// when the OMS's model moves for reasons that have nothing to do with venues.
// Recorded here because a rule with no gate behind it is a rule that has to be
// legible at the place it would be broken.

// OrderRequest is one order this system is placing.
//
// # There is no Type and no TimeInForce, and their absence is §6 step 6 enforced
// # by the type rather than by a check
//
// §6 step 6 fixes what this system may send: "parallel marketable-IOC limit
// orders (never raw MARKET)". oms/domain.Order is deliberately WIDER than that —
// it declares OrderTypeMarket and four TIFs because §4's OpenOrders returns
// orders other participants placed and §6 step 10 routes them to operator
// clearance, so the domain must be able to HOLD a foreign market order. This
// type is the mirror image: it is what leaves the process, so it is exactly as
// wide as the rule permits, and a market order is not expressible here at all.
//
// The argument for the narrowing has to be stated, because a narrowing carries
// one and the reason is what a later slice will re-open. It is NOT "we only need
// IOC limits today". It is that the alternative — an OrderType field whose only
// legal value is Limit, or a TimeInForce field whose only emitted value is IOC —
// declares enum members nothing emits, which is a shape this repo has already
// paid for: the compiler, exhaustiveswitch and switch coverage are all satisfied
// while the member is unreachable, and the first code to emit one finds no test
// anywhere that ever exercised it. The narrowed-away case is a RESTING order
// (GTC/GTX), and when a slice needs one it arrives with its consumer, its venue
// facts and its own tests — which is the repo's rule for a field, applied to a
// field that would otherwise have landed years early wearing a type annotation.
//
// dispatch.go already refuses a non-IOC-limit domain.Order. This is not that
// check repeated: that one can be edited, and this one cannot be expressed.
type OrderRequest struct {
	// Instrument is which market. Spot BTCUSDT and perp BTCUSDT are different
	// instruments with independent order-ID namespaces and — on Binance —
	// different HOSTS. See OrderClient.
	Instrument ids.VenueInstrument

	// ClientOrderID is §5.3's recovery correlation handle, and the venues
	// disagree about what one may contain. Each codec enforces its own venue's
	// rule (Profiles §7); nothing is checked here, because a shared check would
	// have to be the intersection and would then reject an id one venue accepts
	// for a reason belonging to the other.
	ClientOrderID ids.ClientOrderID

	// Side is the direction.
	Side OrderSide

	// Quantity is the ordered size in the instrument's base asset, strictly
	// positive. Direction is Side's job — the same single-statement-of-direction
	// rule domain.Fill follows.
	Quantity fixedpoint.Quantity

	// LimitPrice is §6 step 6's marketable limit. Required, because a request
	// with no price is a market order by another spelling.
	LimitPrice fixedpoint.Price

	// ReduceOnly is §5.3's flag: set "only when the order reduces an existing
	// position on that instrument — never merely because it reduces package
	// delta". A spot instrument has no position, so a reduce-only spot order is
	// REFUSED rather than silently sent without the flag — see Validate.
	ReduceOnly bool

	// Urgency is §7.6's shed class for this order.
	//
	// It is required and has no default, and the refusal is the point. §7.6 puts
	// exposure-creating orders under the ~70% back-off ceiling and safety
	// traffic — a flatten, a §6.7 finality query, §7.1's top-up — at capacity.
	// Both possible defaults are wrong and they are not equally wrong: defaulting
	// to Opening sheds a FLATTEN at 70% of budget, which fails during precisely
	// the incident the reserve exists for, while defaulting to Safety spends the
	// reserve on ordinary trading and needs a second coincidence to hurt. Neither
	// is acceptable silently, so this layer refuses to guess and the caller
	// states it.
	Urgency ratelimit.Priority
}

// OrderSide is the direction of an order, in this seam's own vocabulary.
//
// Separate from BookSide for the reason the ACL exists at all: a book's sides
// are bid/ask and an order's are buy/sell, and the venues spell them differently
// again (Binance "BUY", Bybit "Buy"). Collapsing them would give one type two
// meanings and the wire two spellings with nothing saying which is which.
type OrderSide string

const (
	// OrderSideBuy acquires the base asset — a cash-and-carry trade's spot leg.
	OrderSideBuy OrderSide = "buy"

	// OrderSideSell disposes of it, and on a perpetual opens the short this
	// strategy collects funding on.
	OrderSideSell OrderSide = "sell"
)

// Validate reports whether s is one of the declared directions.
func (s OrderSide) Validate() error {
	switch s {
	case OrderSideBuy, OrderSideSell:
		return nil
	}
	return fmt.Errorf("%q: %w", string(s), ErrOrderSideInvalid)
}

func (s OrderSide) String() string { return string(s) }

// Validate reports whether the request is one this system may send.
//
// It checks what is true of every venue. What each venue additionally requires —
// the ClientOrderID charset and length, the product families it serves — is the
// codec's, stated where the venue fact is recorded.
func (r OrderRequest) Validate() error {
	if err := r.Instrument.Validate(); err != nil {
		return fmt.Errorf("order %q instrument: %w", r.ClientOrderID, err)
	}
	if r.ClientOrderID == "" {
		return ErrOrderClientIDUnset
	}
	if err := r.Side.Validate(); err != nil {
		return fmt.Errorf("order %q side: %w", r.ClientOrderID, err)
	}
	if err := r.Quantity.Validate(); err != nil {
		return fmt.Errorf("order %q quantity: %w", r.ClientOrderID, err)
	}
	sign, err := r.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("order %q quantity: %w", r.ClientOrderID, err)
	}
	if sign <= 0 {
		return fmt.Errorf("order %q is for %s: %w", r.ClientOrderID, r.Quantity, ErrOrderQuantityNotPositive)
	}
	if err := r.LimitPrice.Validate(); err != nil {
		return fmt.Errorf("order %q limit price: %w", r.ClientOrderID, err)
	}
	if r.ReduceOnly && !r.Instrument.Product.IsPerp() {
		// Dropping the flag instead would send an order that can INCREASE a
		// holding when the caller asked for one that cannot. §5.3 ties
		// reduceOnly to "an existing position on that instrument", and a spot
		// instrument has none — so this is a category error at the call site,
		// not a venue capability to paper over.
		return fmt.Errorf("order %q on %s: %w", r.ClientOrderID, r.Instrument.Product, ErrReduceOnlyOnSpot)
	}
	if err := r.Urgency.Validate(); err != nil {
		return fmt.Errorf("order %q urgency: %w", r.ClientOrderID, err)
	}
	if r.Urgency == ratelimit.PriorityRoutine {
		// §7.6's Routine class is "the reads that keep the risk view live —
		// marks, books, account posture". Placing an order is not one of those
		// under any command in §5.3, so Routine here is a value that passed a
		// type check and states nothing true. It is refused rather than mapped
		// to a neighbour, because both neighbours are a real decision: Routine
		// sits between the two ceilings, so silently rounding it up spends the
		// safety reserve and rounding it down sheds the order at 70%.
		return fmt.Errorf("order %q: %w", r.ClientOrderID, ErrOrderUrgencyRoutine)
	}
	return nil
}

// OrderAck is what a venue returns from an accepted Place.
//
// It carries the venue's id and nothing else, for §4's stated reason: both
// venues return an order status on the create response, the private order stream
// reports the same events, and mapping the response's status into order state
// would make the two paths two authorities over one order with no rule for which
// wins. So the status is not carried, and it is not carried DELIBERATELY — a
// decoder that reads it and discards it is one edit away from a decoder that
// uses it.
type OrderAck struct {
	// VenueOrderID is the venue's own identifier. It may legitimately be empty
	// on a venue that returns none; recovery keys on ClientOrderID.
	VenueOrderID string
}

// OrderCodec is one venue's translation of the order surface.
//
// Encode and decode are separate and both are pure, which is what makes them
// checkable against the venues' own documented examples without a socket. The
// client below is the only part that needs a network.
type OrderCodec interface {
	// Venue is which exchange this codec speaks for.
	Venue() ids.Venue

	// EncodePlace renders the request as one signed call.
	EncodePlace(r OrderRequest) (transport.Call, error)

	// DecodePlaceAck reads the venue's acknowledgement.
	//
	// It takes the REQUEST as well as the response because the acknowledgement
	// has to be checked against what was asked: a venue echoing a different
	// ClientOrderID than the one submitted means the ack belongs to another
	// order, and the recovery handle §5.3 rests on would be recorded against the
	// wrong one.
	DecodePlaceAck(r OrderRequest, resp transport.Response) (OrderAck, error)

	// OrderQueryCodec is the read half (slice C2b).
	//
	// Embedded rather than segregated into its own codec, because §5.3 requires
	// a resolving query for every order this system places — "an order at
	// DispatchAuthorized or beyond is resolved ONLY by OrderByClientID/Fills".
	// A venue that could place but not resolve is not a venue this system may
	// trade on, so a separate interface would model something that cannot exist
	// and would give OrderClientConfig a nil case for it.
	OrderQueryCodec
}

// OrderClient is §4's OrderGateway.Place for one venue.
//
// # The route is per PRODUCT FAMILY, and on Binance that is a different HOST
//
// §4 requires every per-instrument method to key on VenueInstrument rather than
// on a symbol, and the usual reason given is that spot BTCUSDT and perp BTCUSDT
// have independent order-ID namespaces. On Binance the consequence is one step
// larger than that: the two families are on different hosts entirely —
// api.binance.com for spot and fapi.binance.com for USDⓈ-M (Profiles §3/§7) —
// so routing selects a base URL, a rate-limit budget and a signed client, not
// merely a path.
//
// Which is why an unconfigured family is REFUSED rather than falling back to any
// default. The accident this forecloses is specific and it does not look like an
// error: "BTCUSDT" exists on both hosts, so a spot order sent to the futures
// client is a valid, executable order on a real instrument that is not the one
// asked for — a leveraged short where a cash purchase was intended. There is no
// status code for that; the venue accepts it.
type OrderClient struct {
	codec      OrderCodec
	transports map[ids.ProductCategory]*transport.SignedClient
}

// OrderClientConfig is one venue's order surface.
type OrderClientConfig struct {
	// Codec is the venue's translation.
	Codec OrderCodec

	// Transports is the signed client per product family.
	//
	// Two entries on Binance and — legitimately — the SAME pointer twice on
	// Bybit, whose two families share api.bybit.com and differ by a `category`
	// parameter. The map is not a redundancy there: it is the statement that the
	// question "which transport serves this family" has an answer on every
	// venue, and that on one of them the answer happens to be the same twice.
	//
	// On Binance the two clients MUST share one *ratelimit.Governor, because
	// §7.6 keeps separate weight/order budgets per family under a shared IP-ban
	// envelope per venue. Two governors would be two IP envelopes for one IP,
	// each seeing half the traffic and neither seeing the ban coming.
	Transports map[ids.ProductCategory]*transport.SignedClient
}

// NewOrderClient validates the surface and builds the client.
func NewOrderClient(cfg OrderClientConfig) (*OrderClient, error) {
	if cfg.Codec == nil {
		return nil, ErrOrderCodecUnset
	}
	if err := cfg.Codec.Venue().Validate(); err != nil {
		return nil, err
	}
	if len(cfg.Transports) == 0 {
		return nil, ErrOrderTransportsUnset
	}
	routes := make(map[ids.ProductCategory]*transport.SignedClient, len(cfg.Transports))
	for family, client := range cfg.Transports {
		if err := family.Validate(); err != nil {
			return nil, fmt.Errorf("order transport: %w", err)
		}
		if client == nil {
			return nil, fmt.Errorf("order transport for %s: %w", family, ErrOrderTransportsUnset)
		}
		routes[family] = client
	}
	return &OrderClient{codec: cfg.Codec, transports: routes}, nil
}

// Venue is which exchange this client places orders on.
func (c *OrderClient) Venue() ids.Venue { return c.codec.Venue() }

// Place submits the order to the venue EXACTLY ONCE.
//
// §5.3: "Place is called once per order; transport retries forbidden". Nothing
// here retries and nothing below it does either — SignedClient is built without
// redirects or retries for this reason — so a second attempt is a decision made
// upstream with the journal in hand.
//
// # Every path out carries an apperror.Effect, including the ones before the send
//
// A refusal here is EffectNotApplied by evidence rather than by default: the
// request was never encoded, so no byte left the process. Once transport.Do is
// entered the answer is its to give, and a decode failure AFTER a success status
// is EffectApplied — the order is live and our record of it is what is broken.
func (c *OrderClient) Place(ctx context.Context, r OrderRequest) (OrderAck, error) {
	op := string(c.codec.Venue()) + ".Place"

	if err := r.Validate(); err != nil {
		return OrderAck{}, apperror.Precondition(op, "order request", err)
	}
	if r.Instrument.Venue != c.codec.Venue() {
		// A request for another venue reaching this client would otherwise be
		// signed with these credentials and sent to this host, where the symbol
		// may well exist.
		return OrderAck{}, apperror.Precondition(op, "venue",
			fmt.Errorf("order %q is for %s: %w", r.ClientOrderID, r.Instrument.Venue, ErrOrderVenueMismatch))
	}
	client, ok := c.transports[r.Instrument.Product]
	if !ok {
		return OrderAck{}, apperror.Precondition(op, "route",
			fmt.Errorf("order %q on %s: %w", r.ClientOrderID, r.Instrument.Product, ErrOrderFamilyUnrouted))
	}

	call, err := c.codec.EncodePlace(r)
	if err != nil {
		return OrderAck{}, apperror.Precondition(op, "encode", err)
	}

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Already classified: SignedClient.Do is where the effect is decided,
		// and §5.3 forbids re-deriving it from the cause here.
		return OrderAck{}, err
	}

	ack, err := c.codec.DecodePlaceAck(r, resp)
	if err != nil {
		// The venue accepted the call — a 2xx with no refusal in the body — and
		// we cannot read what it said. The order is LIVE; the local record is
		// what needs correcting, which is a different instruction from
		// EffectUnknown's "go and ask the venue".
		return OrderAck{}, transport.ClassifyAckDecodeFailure(call.Op, err)
	}
	return ack, nil
}
