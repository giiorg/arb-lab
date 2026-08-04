package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// BybitOrderCodec is Bybit V5's order surface for both product families
// (Profiles §7).
//
// # One host, one path, and the product family is a FIELD
//
// This is the shape Binance does not have: POST /v5/order/create on
// api.bybit.com serves spot and linear alike, and `category` selects between
// them. So the mis-routing hazard that makes Binance's per-family hosts
// load-bearing shows up here as a mis-set string inside a signed body, where
// nothing about the URL would look wrong.
//
// # The body is signed VERBATIM, which constrains how it may be built
//
// Bybit signs "timestamp + API key + recv_window + jsonBodyString" — the bytes
// actually transmitted (Profiles §6). transport.SignedRequest.Body is []byte for
// that reason, and this codec marshals ONCE: any layer that re-serialized
// between signing and sending would produce a body the signature does not cover,
// and the failure would arrive as an authentication error, which reads as a
// credential problem rather than an encoding one.
type BybitOrderCodec struct {
	venue ids.Venue
}

// NewBybitOrderCodec builds the codec for one configured Bybit venue.
func NewBybitOrderCodec(venue ids.Venue) (*BybitOrderCodec, error) {
	if err := venue.Validate(); err != nil {
		return nil, err
	}
	return &BybitOrderCodec{venue: venue}, nil
}

// Venue implements OrderCodec.
func (c *BybitOrderCodec) Venue() ids.Venue { return c.venue }

// bybitPlaceWire is the create-order body.
//
// # Neither optional field may carry omitempty, and isLeverage is why
//
// §4.1 and Profiles §2 make `isLeverage=0` normative on EVERY spot order — it is
// what keeps the spot leg an unborrowed cash purchase rather than a margin
// position, which is the premise of the whole "spot cannot be seized to cover
// the perp loss" property. The value that must be sent is ZERO, and `omitempty`
// drops exactly zero. So the field that must never be missing is the one the
// obvious struct tag would silently remove, and the resulting order is a
// perfectly valid margin-enabled spot buy.
//
// Pointers rather than bare values, so "not applicable to this product" and
// "applicable and false/zero" are different states with different wire forms.
type bybitPlaceWire struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	OrderType   string `json:"orderType"`
	Qty         string `json:"qty"`
	Price       string `json:"price"`
	TimeInForce string `json:"timeInForce"`
	OrderLinkID string `json:"orderLinkId"`
	ReduceOnly  *bool  `json:"reduceOnly,omitempty"`
	IsLeverage  *int   `json:"isLeverage,omitempty"`
}

// EncodePlace implements OrderCodec.
func (c *BybitOrderCodec) EncodePlace(r OrderRequest) (transport.Call, error) {
	if err := c.checkOrderLinkID(r.ClientOrderID); err != nil {
		return transport.Call{}, err
	}

	side, err := bybitSide(r.Side)
	if err != nil {
		return transport.Call{}, err
	}

	category, err := bybitCategory(r.Instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}

	body := bybitPlaceWire{
		Category:    category,
		Symbol:      r.Instrument.Symbol,
		Side:        side,
		OrderType:   "Limit",
		Qty:         r.Quantity.Decimal(),
		Price:       r.LimitPrice.Decimal(),
		TimeInForce: "IOC",
		OrderLinkID: string(r.ClientOrderID),
	}
	if r.Instrument.Product.IsPerp() {
		if r.ReduceOnly {
			reduce := true
			body.ReduceOnly = &reduce
		}
	} else {
		// Profiles §2, normative: "isLeverage=0 hard-coded on every spot
		// POST /v5/order/create". Not a default — the fact that it is a zero is
		// what makes it easy to lose, so it is set explicitly through a pointer
		// the encoder cannot elide.
		spotCash := 0
		body.IsLeverage = &spotCash
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return transport.Call{}, fmt.Errorf("bybit place body: %w", err)
	}

	return transport.Call{
		Op:     string(c.venue) + ".PlaceOrder",
		Method: "POST",
		Path:   "/v5/order/create",
		Body:   raw,
		Family: r.Instrument.Product,

		// Bybit's consumption formula counts the orders carried by an order
		// endpoint: one for this plain create, and N for a future batch. Carrying
		// that count in Cost lets the endpoint's orders-dimension budget use the
		// same units rule without inventing a weight the venue never published.
		Priority: r.Urgency,
		Cost:     ratelimit.Cost{Orders: 1},
	}, nil
}

// bybitCategory maps the canonical product family onto Bybit's `category`.
//
// Exhaustive over the declared enum-like (§3.6 rule 10). The consequence of
// getting this wrong is not an error: `category` selects which market the order
// lands on, and BTCUSDT exists in both spot and linear, so a mislabelled request
// is an accepted order on the wrong instrument.
func bybitCategory(p ids.ProductCategory) (string, error) {
	switch p {
	case ids.ProductSpot:
		return "spot", nil
	case ids.ProductLinearPerp:
		return "linear", nil
	case ids.ProductInversePerp:
		// Bybit does serve `inverse`, and no venue fact for trading it is
		// recorded and no sizing arithmetic in this repo is written for
		// coin-margined quantity. Refused for the same reason as Binance's
		// COIN-M rather than passed through because the venue would accept it.
		return "", fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// bybitSide renders the direction in Bybit's spelling — title case, where
// Binance uses upper. The difference is the reason OrderSide exists.
func bybitSide(s OrderSide) (string, error) {
	switch s {
	case OrderSideBuy:
		return "Buy", nil
	case OrderSideSell:
		return "Sell", nil
	}
	return "", fmt.Errorf("%q: %w", string(s), ErrOrderSideInvalid)
}

// checkOrderLinkID holds Bybit's published rule (Profiles §7): "A max of 36
// characters. Combinations of numbers, letters (upper and lower cases), dashes,
// and underscores are supported."
//
// It is NARROWER than Binance's, which permits "." ":" and "/" as well, and the
// asymmetry is the point: a shared check would have to be one venue's rule, and
// whichever it was would be wrong about the other in a direction that only shows
// up as a venue rejection at submission time.
func (c *BybitOrderCodec) checkOrderLinkID(id ids.ClientOrderID) error {
	if len(id) == 0 || len(id) > bybitOrderLinkIDMax {
		return fmt.Errorf("bybit order link id %q is %d characters, the rule allows 1..%d: %w",
			id, len(id), bybitOrderLinkIDMax, ErrOrderIDUnacceptable)
	}
	for i := 0; i < len(id); i++ {
		if !bybitOrderLinkIDChar(id[i]) {
			return fmt.Errorf("bybit order link id %q at byte %d (%q): %w",
				id, i, string(id[i]), ErrOrderIDUnacceptable)
		}
	}
	return nil
}

// bybitOrderLinkIDMax is the published "max of 36 characters".
const bybitOrderLinkIDMax = 36

// bybitOrderLinkIDChar is "numbers, letters (upper and lower cases), dashes,
// and underscores" and nothing else.
func bybitOrderLinkIDChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '_', '-':
		return true
	}
	return false
}

// bybitAckWire is the create-order response envelope.
//
// retCode and retMsg are not read here: transport.ClassifyResponse has already
// refused a non-zero retCode, which on this venue arrives with HTTP 200. This
// decoder therefore runs only on an accepted order, and duplicating the check
// would put a second, weaker copy of the rule where the first one is not visible.
type bybitAckWire struct {
	Result struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	} `json:"result"`
}

// DecodePlaceAck implements OrderCodec.
//
// # The ack is an ASYNC ACCEPT, and that is why nothing but the ids is read
//
// Profiles §1: Bybit's create response is "an async accept (returns
// orderId/orderLinkId); status/fills must be confirmed via the private WS". So
// there is no status here to be tempted by — the venue's own contract says the
// stream is the authority, which is the conclusion §4's Ack reaches for both
// venues on design grounds.
func (c *BybitOrderCodec) DecodePlaceAck(r OrderRequest, resp transport.Response) (OrderAck, error) {
	var wire bybitAckWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return OrderAck{}, fmt.Errorf("bybit place ack: %w", err)
	}
	if wire.Result.OrderLinkID != string(r.ClientOrderID) {
		return OrderAck{}, fmt.Errorf("bybit place ack echoes order link id %q, sent %q: %w",
			wire.Result.OrderLinkID, r.ClientOrderID, ErrAckUnreadable)
	}
	if wire.Result.OrderID == "" {
		return OrderAck{}, fmt.Errorf("bybit place ack for %q carries no orderId: %w",
			r.ClientOrderID, ErrAckUnreadable)
	}
	return OrderAck{VenueOrderID: wire.Result.OrderID}, nil
}

// Compile-time proof that the codec satisfies the port (§3.6 rule 2).
var _ OrderCodec = (*BybitOrderCodec)(nil)
