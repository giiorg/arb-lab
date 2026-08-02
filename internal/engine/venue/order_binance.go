package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// BinanceOrderCodec is Binance's order surface for BOTH product families
// (Profiles §7).
//
// # One codec, two API families, and the difference is not a path
//
// USDⓈ-M futures is POST /fapi/v1/order on fapi.binance.com; spot is POST
// /api/v3/order on api.binance.com. They are separate hosts, separate
// rate-limit budgets and separate documents, and three of the differences below
// would each be invisible if the two were treated as one endpoint with a
// variable path:
//
//   - The ORDER COST differs. Futures new-order is verbatim "0 on IP rate
//     limit(x-mbx-used-weight-1m)" and 1 on each order-count limit; spot's is
//     "Weight: 1". A shared cost is wrong on one of them whichever it names.
//   - reduceOnly EXISTS ONLY ON FUTURES. Spot has no position to reduce.
//   - newOrderRespType has a DIFFERENT ENUM and a different default: futures is
//     {ACK, RESULT} defaulting to ACK, spot is {ACK, RESULT, FULL} where
//     "MARKET and LIMIT order types default to FULL". So the family that
//     defaults to the largest response is the one whose response we least want.
//
// See EncodePlace on why that last one is a safety property rather than a byte
// count.
type BinanceOrderCodec struct {
	venue ids.Venue
}

// NewBinanceOrderCodec builds the codec for one configured Binance venue.
func NewBinanceOrderCodec(venue ids.Venue) (*BinanceOrderCodec, error) {
	if err := venue.Validate(); err != nil {
		return nil, err
	}
	return &BinanceOrderCodec{venue: venue}, nil
}

// Venue implements OrderCodec.
func (c *BinanceOrderCodec) Venue() ids.Venue { return c.venue }

// EncodePlace implements OrderCodec.
//
// # newOrderRespType is stated EXPLICITLY, and the reason is §4's Ack contract
//
// §4 defines the acknowledgement as carrying "what reconciliation needs and
// nothing the state machine reads", and gives the reason: both venues return an
// order status on the create response, the private order stream reports the same
// events, and reading the response's status into order state would make them two
// authorities over one order with no rule for which wins when they disagree.
//
// On Binance SPOT, a LIMIT order's response DEFAULTS to FULL — which carries a
// `fills` array. So the default behaviour of the venue is to hand this system
// executions on the submission path, at the exact moment §5.3's fill ledger has
// no transaction open and §4 says the stream is the authority. They would be
// dropped, which is correct and is also the fragile kind of correct: the data
// would sit in the response, one edit from a reader who thinks applying it early
// is an optimization.
//
// Asking for ACK removes them from the wire. That converts a discipline into an
// absence, which is the same move the request type makes by having no OrderType
// field.
func (c *BinanceOrderCodec) EncodePlace(r OrderRequest) (transport.Call, error) {
	if err := c.checkClientOrderID(r.ClientOrderID); err != nil {
		return transport.Call{}, err
	}

	side, err := binanceSide(r.Side)
	if err != nil {
		return transport.Call{}, err
	}

	path, cost, err := binanceOrderRoute(r.Instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}

	// Ordered, not a map: both venues sign the query string as a STRING, and
	// Params exists because url.Values.Encode would sort it (see signing.go).
	// The order below is the one Binance's own published vectors use for the
	// fields they share.
	params := transport.Params{
		{Key: "symbol", Value: r.Instrument.Symbol},
		{Key: "side", Value: side},
		{Key: "type", Value: "LIMIT"},
		{Key: "timeInForce", Value: "IOC"},
		{Key: "quantity", Value: r.Quantity.Decimal()},
		{Key: "price", Value: r.LimitPrice.Decimal()},
		{Key: "newClientOrderId", Value: string(r.ClientOrderID)},
	}
	if r.ReduceOnly {
		// Sent only when true, and only reachable on a perp: OrderRequest
		// refuses reduce-only on spot, and false is the venue's documented
		// default, so the omission states the same thing the field would.
		//
		// The narrower reason for not sending "false" unconditionally: the
		// parameter "cannot be sent in Hedge Mode" (Profiles §1). §4.1 pins the
		// account to one-way and §7.5 asserts it at boot, so hedge mode is a
		// refuse-to-trade condition rather than a case to support — but if that
		// assertion ever fails open, an unconditional reduceOnly would make
		// EVERY order rejected rather than only the ones that meant it.
		params = append(params, transport.Param{Key: "reduceOnly", Value: "true"})
	}
	params = append(params, transport.Param{Key: "newOrderRespType", Value: "ACK"})

	return transport.Call{
		Op:       string(c.venue) + ".PlaceOrder",
		Method:   "POST",
		Path:     path,
		Params:   params,
		Family:   r.Instrument.Product,
		Priority: r.Urgency,
		Cost:     cost,
	}, nil
}

// binanceOrderRoute is the path and the venue-documented cost per family.
//
// The switch is over a module-declared enum-like, so §3.6 rule 10 requires every
// member to be named: a product family added to ids without a route here is a
// compile error rather than an order sent to whichever host the last case
// happened to be.
func binanceOrderRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		// "Weight: 1" (spot rest-api.md). Spot also meters order counts, so the
		// order dimension is charged as well.
		return "/api/v3/order", ratelimit.Cost{Weight: 1, Orders: 1}, nil

	case ids.ProductLinearPerp:
		// Verbatim: "1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S); 1 on
		// 1min order rate limit(X-MBX-ORDER-COUNT-1M); 0 on IP rate
		// limit(x-mbx-used-weight-1m)". The zero is a venue fact, and
		// ratelimit.Cost.Validate had to be corrected to accept it.
		return "/fapi/v1/order", ratelimit.Cost{Weight: 0, Orders: 1}, nil

	case ids.ProductInversePerp:
		// COIN-M is a third host and a third document (dapi.binance.com), and
		// no venue fact for it is recorded. Named rather than routed to the
		// USDⓈ-M path, which would place a coin-margined order — quantity in
		// USD, margin in the base coin — against arithmetic written for linear.
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceSide renders the direction in Binance's spelling.
func binanceSide(s OrderSide) (string, error) {
	switch s {
	case OrderSideBuy:
		return "BUY", nil
	case OrderSideSell:
		return "SELL", nil
	}
	return "", fmt.Errorf("%q: %w", string(s), ErrOrderSideInvalid)
}

// checkClientOrderID holds Binance's published rule for newClientOrderId.
//
// Verbatim from the USDⓈ-M New Order docs (Profiles §7): "Can only be string
// following the rule: ^[\.A-Z\:/a-z0-9_-]{1,36}$".
//
// # It is applied to SPOT too, and that is a stated assumption rather than a
// # verified fact
//
// The spot document states neither a charset nor a length for the field. So the
// rule enforced on spot is the futures rule, chosen because it is the only
// published one — not because spot was checked. If spot turns out to permit
// LESS, this check passes an id spot rejects, and the failure arrives as a venue
// rejection at submission time. It is written down here rather than assumed
// because ids.ClientOrderID's own doc had to carry exactly this kind of
// unverified budget for several slices, and naming it is what let this slice
// close it.
func (c *BinanceOrderCodec) checkClientOrderID(id ids.ClientOrderID) error {
	if len(id) == 0 || len(id) > binanceClientOrderIDMax {
		return fmt.Errorf("binance client order id %q is %d characters, the rule allows 1..%d: %w",
			id, len(id), binanceClientOrderIDMax, ErrOrderIDUnacceptable)
	}
	for i := 0; i < len(id); i++ {
		if !binanceClientOrderIDChar(id[i]) {
			return fmt.Errorf("binance client order id %q at byte %d (%q): %w",
				id, i, string(id[i]), ErrOrderIDUnacceptable)
		}
	}
	return nil
}

// binanceClientOrderIDMax is the 36 of ^…{1,36}$.
const binanceClientOrderIDMax = 36

// binanceClientOrderIDChar is the published character class, transcribed one
// character at a time from ^[\.A-Z\:/a-z0-9_-]{1,36}$.
func binanceClientOrderIDChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '.', ':', '/', '_', '-':
		return true
	}
	return false
}

// binanceAckWire is the ACK response, which is identical in the fields we read
// across both families: {"orderId":…,"clientOrderId":"…", …}.
//
// orderId is a json.Number rather than an int64 or a float64. float64 is banned
// under internal/ and would also be WRONG — Binance order ids are int64 and pass
// 2^53 — while json.Number keeps the venue's own digits, which is what an
// operator searching a venue UI will type.
type binanceAckWire struct {
	OrderID       json.Number `json:"orderId"`
	ClientOrderID string      `json:"clientOrderId"`
}

// DecodePlaceAck implements OrderCodec.
//
// The echo check is the substance. A venue answering with a DIFFERENT
// clientOrderId than the one submitted is describing another order, and §5.3
// makes that id the only handle recovery has — so recording this ack against
// this order would attach the venue's id for someone else's order to ours, and
// every later resolving query would answer about the wrong one.
//
// Both failures below are EffectApplied at the call site (OrderClient.Place
// classifies them), which is the correct and uncomfortable reading: the venue
// returned 2xx, so an order exists, and what is broken is our record of it.
func (c *BinanceOrderCodec) DecodePlaceAck(r OrderRequest, resp transport.Response) (OrderAck, error) {
	var wire binanceAckWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return OrderAck{}, fmt.Errorf("binance place ack: %w", err)
	}
	if wire.ClientOrderID != string(r.ClientOrderID) {
		return OrderAck{}, fmt.Errorf("binance place ack echoes client order id %q, sent %q: %w",
			wire.ClientOrderID, r.ClientOrderID, ErrAckUnreadable)
	}
	if wire.OrderID.String() == "" {
		return OrderAck{}, fmt.Errorf("binance place ack for %q carries no orderId: %w",
			r.ClientOrderID, ErrAckUnreadable)
	}
	return OrderAck{VenueOrderID: wire.OrderID.String()}, nil
}

// Compile-time proof that the codec satisfies the port (§3.6 rule 2).
var _ OrderCodec = (*BinanceOrderCodec)(nil)
