package venue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's order READ surface for both product families
// (Profiles §8).
//
// # The two families disagree about more than the host, and the read half is
// # where that gets expensive
//
// C2a found newOrderRespType differing in enum AND default between spot and
// USDⓈ-M. The read surface has four more of the same kind, and each one is a
// value a reader would carry over from the other family without re-checking:
//
//	                         USDⓈ-M                       spot
//	order status weight      1                            4
//	executions weight        5                            20 (5 with orderId)
//	executions window cap    7 days                       24 HOURS
//	order retention          3 days unfilled / 90 days    not published
//
// The window cap is the one that fails silently. A sweeper written once against
// "the 7-day window" is wrong on spot by a factor of seven, on the same venue,
// with the parameter spelled identically — and a truncated sweep that reports
// success is a false watermark, which is what §6.7 step 3 collapses a
// possible-delta interval on.

// EncodeOrderStatus implements OrderQueryCodec.
//
// Binance has ONE order surface per family, so `surface` may only ever be
// SurfaceOrderStatus here. It is still a parameter and still checked, because a
// caller that reached this codec with SurfaceOrderHistory has a bug about which
// venue it is talking to, and silently answering from the wrong surface is how
// §52d's retention reasoning would be applied to the wrong retention.
func (c *BinanceOrderCodec) EncodeOrderStatus(ref OrderRef, surface QuerySurface) (transport.Call, error) {
	if err := c.checkClientOrderID(ref.ClientOrderID); err != nil {
		return transport.Call{}, err
	}
	if surface != SurfaceOrderStatus {
		return transport.Call{}, fmt.Errorf("binance has no %s surface: %w", surface, ErrQuerySurfaceInvalid)
	}

	path, cost, err := binanceOrderStatusRoute(ref.Instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}

	return transport.Call{
		Op:     string(c.venue) + ".OrderByClientID",
		Method: "GET",
		Path:   path,
		Params: transport.Params{
			{Key: "symbol", Value: ref.Instrument.Symbol},
			{Key: "origClientOrderId", Value: string(ref.ClientOrderID)},
		},
		Family: ref.Instrument.Product,
		Cost:   cost,
	}, nil
}

// binanceOrderStatusRoute is the path and the venue-documented cost per family.
//
// Exhaustive over the enum-like (§3.6 rule 10), and the weights DIFFER — "IP
// Weight 1" on USDⓈ-M against "Weight: 4" on spot. A shared constant would be
// wrong on one family whichever number it named, which is the same trap the
// create-order cost had one slice ago and the reason that one is not shared
// either.
func binanceOrderStatusRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		return "/api/v3/order", ratelimit.Cost{Weight: 4}, nil
	case ids.ProductLinearPerp:
		return "/fapi/v1/order", ratelimit.Cost{Weight: 1}, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceStatusWire is the query-order response, narrowed to what §6.7 step 2
// asks for: "terminal order status with the authoritative final cumulative
// executed quantity".
type binanceStatusWire struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
	ExecutedQty   string `json:"executedQty"`
}

// DecodeOrderStatus implements OrderQueryCodec.
//
// This decoder never produces OrderAbsent. On Binance an absence is a classified
// transport error the client resolves through AbsenceFromError before any body
// reaches here, so a response that got this far describes an order that exists.
func (c *BinanceOrderCodec) DecodeOrderStatus(ref OrderRef, assets InstrumentAssets, surface QuerySurface, resp transport.Response) (OrderStatus, error) {
	if surface != SurfaceOrderStatus {
		return OrderStatus{}, fmt.Errorf("binance has no %s surface: %w", surface, ErrQuerySurfaceInvalid)
	}

	var wire binanceStatusWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return OrderStatus{}, fmt.Errorf("binance order status: %w", err)
	}
	if wire.ClientOrderID != string(ref.ClientOrderID) {
		// The same check DecodePlaceAck makes and for the same reason: resolving
		// one order's state from another order's record is the failure the
		// recovery handle exists to prevent, and here it would write a foreign
		// order's executed quantity into our fill ledger.
		return OrderStatus{}, fmt.Errorf("binance order status echoes client id %q, asked for %q: %w",
			wire.ClientOrderID, ref.ClientOrderID, ErrAckUnreadable)
	}
	if wire.Symbol != ref.Instrument.Symbol {
		return OrderStatus{}, fmt.Errorf("binance order status is for symbol %q, asked for %q: %w",
			wire.Symbol, ref.Instrument.Symbol, ErrAckUnreadable)
	}

	state, err := binanceOrderState(wire.Status)
	if err != nil {
		return OrderStatus{}, err
	}
	executed, err := fixedpoint.ParseQuantity(assets.Base, wire.ExecutedQty)
	if err != nil {
		return OrderStatus{}, fmt.Errorf("binance order status executedQty %q: %w", wire.ExecutedQty, err)
	}

	return OrderStatus{
		Presence:         OrderPresent,
		State:            state,
		ExecutedQuantity: executed,
		VenueOrderID:     fmt.Sprintf("%d", wire.OrderID),
	}, nil
}

// binanceOrderState maps Binance's status onto the normalized lifecycle.
//
// Binance's set is "NEW, PARTIALLY_FILLED, FILLED, CANCELED, REJECTED, EXPIRED,
// EXPIRED_IN_MATCH", and all seven are named.
//
// # There is no default arm, and its absence is the design
//
// A default mapping the unrecognised onto a terminal state would be the
// switch-default-hides-its-own-group shape: every status Binance adds in future
// would silently join the terminal group, which is exactly the reading that lets
// §6.7 collapse a possible-delta interval that has NOT collapsed — an order the
// venue can still execute, read as one it cannot. Mapping the unrecognised onto
// OrderStateOpen instead would be safe in that direction and wrong in another:
// a genuinely finished order held open forever stalls the finality contract with
// every light green. So the unrecognised is an error, which is a query that could
// not be answered, which is the state §5.3 already has a rule for.
func binanceOrderState(status string) (OrderState, error) {
	switch status {
	case "NEW":
		return OrderStateOpen, nil
	case "PARTIALLY_FILLED":
		return OrderStatePartiallyFilled, nil
	case "FILLED":
		return OrderStateFilled, nil
	case "CANCELED", "EXPIRED", "EXPIRED_IN_MATCH":
		// All three are "over, and not an explicit refusal". EXPIRED_IN_MATCH is
		// self-trade prevention — "the order was expired by the exchange due to
		// STP" — and it is genuinely reachable here, because §6 step 6 sends two
		// legs and §7 can have a reduction generation live on the same
		// instrument at the same time.
		return OrderStateTerminated, nil
	case "REJECTED":
		return OrderStateRejected, nil
	}
	return "", fmt.Errorf("binance order status %q: %w", status, ErrOrderStateInvalid)
}

// AbsenceFromError implements OrderQueryCodec.
//
// Binance publishes exactly one code for it — "-2013 NO_SUCH_ORDER · Order does
// not exist." — and it is shared by spot and USDⓈ-M, which is the one thing the
// two families do agree on here.
//
// The effect is checked as well as the code, and that is not belt-and-braces.
// EffectNotApplied means the venue refused the request outright; a -2013 arriving
// with any other effect would be a response this project does not understand, and
// reading an absence out of it is precisely the inference §5.3 forbids. The
// conservative reading is "this is not an absence", which sends the caller down
// the error path with a query that failed.
func (c *BinanceOrderCodec) AbsenceFromError(err error) bool {
	var ae *apperror.Error
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Code() == binanceCodeNoSuchOrder && ae.Effect() == apperror.EffectNotApplied
}

// binanceCodeNoSuchOrder is "-2013 NO_SUCH_ORDER".
const binanceCodeNoSuchOrder = "-2013"

// FallbackSurface implements OrderQueryCodec.
//
// Binance has one order surface per family and therefore no fallback. This is
// where the two venues differ most: on Bybit the second surface is normative
// (§5.3), and a shared implementation would have to be one venue's answer.
func (c *BinanceOrderCodec) FallbackSurface(QuerySurface) (QuerySurface, bool) {
	return "", false
}

// StatusReach implements OrderQueryCodec — §52d's horizons, per family.
//
// # The USDⓈ-M cliff lands on THIS system's most common terminal state
//
// Verbatim, the orders USDⓈ-M "will not be found": "order status is CANCELED or
// EXPIRED AND order has NO filled trade AND created time + 3 days < current
// time", and unconditionally at "order create time + 90 days < current time".
//
// §6 step 6 sends "parallel marketable-IOC limit orders (never raw MARKET)" and
// nothing else, and a marketable IOC that does not fill is terminal at once with
// no filled trade — exactly the row that is dropped after THREE DAYS. So the
// single most common outcome of our own orders is the one that becomes
// unqueryable soonest.
//
// # Spot publishes nothing, and that is reported as unknown rather than as none
//
// The spot Query-order page states no retention at all. Encoding that as "no
// cliff" would be the same move as assuming spot's newClientOrderId rule from
// the futures one (Profiles §7), except that here the assumption runs in the
// UNSAFE direction: it would license treating a spot absence as proved absence
// forever. Unpublished says the unknown out loud, and slice D decides what an
// unpublished retention licenses.
func (c *BinanceOrderCodec) StatusReach(instrument ids.VenueInstrument, surface QuerySurface) (QueryReach, error) {
	if surface != SurfaceOrderStatus {
		return QueryReach{}, fmt.Errorf("binance has no %s surface: %w", surface, ErrQuerySurfaceInvalid)
	}
	switch instrument.Product {
	case ids.ProductSpot:
		return QueryReach{Surface: SurfaceOrderStatus, Unpublished: true}, nil
	case ids.ProductLinearPerp:
		return QueryReach{
			Surface:           SurfaceOrderStatus,
			UnfilledRetention: 3 * 24 * time.Hour,
			FilledRetention:   90 * 24 * time.Hour,
		}, nil
	case ids.ProductInversePerp:
		return QueryReach{}, fmt.Errorf("%s: %w", instrument.Product, ErrOrderFamilyUnrouted)
	}
	return QueryReach{}, fmt.Errorf("%s: %w", instrument.Product, ids.ErrProductUnknown)
}

// EncodeExecutions implements OrderQueryCodec.
//
// # The window is sent explicitly and the id cursor is NOT used
//
// Both families offer `fromId`, and on both it is mutually exclusive with the
// time window — USDⓈ-M says "the parameter fromId cannot be sent with startTime
// or endTime"; spot publishes a whitelist of parameter combinations in which no
// fromId+time pair appears. §6.7 step 3 needs a sweep bounded by a WATERMARK, so
// the time window wins and the id is not the cursor. That is §52b reached from
// the venues rather than from the contract.
func (c *BinanceOrderCodec) EncodeExecutions(instrument ids.VenueInstrument, w ExecutionWindow) (transport.Call, error) {
	if err := w.Validate(); err != nil {
		return transport.Call{}, err
	}

	path, cost, cap, err := binanceExecutionsRoute(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	if span := w.Until.Sub(w.Since.At); span > cap {
		return transport.Call{}, fmt.Errorf("binance %s sweep of %s exceeds the published %s cap: %w",
			instrument.Product, span, cap, ErrExecutionWindowTooWide)
	}

	return transport.Call{
		Op:     string(c.venue) + ".Executions",
		Method: "GET",
		Path:   path,
		Params: transport.Params{
			{Key: "symbol", Value: instrument.Symbol},
			{Key: "startTime", Value: fmt.Sprintf("%d", w.Since.At.UnixMilli())},
			{Key: "endTime", Value: fmt.Sprintf("%d", w.Until.UnixMilli())},
			{Key: "limit", Value: fmt.Sprintf("%d", binanceExecutionsPageLimit)},
		},
		Family: instrument.Product,
		Cost:   cost,
	}, nil
}

// binanceExecutionsPageLimit is the published maximum on both families
// ("max: 1000" on USDⓈ-M, "Maximum: 1000" on spot), taken rather than defaulted
// because a page that ends early is a page whose last instant is not the
// watermark, and the sweep would then need more round trips to prove the same
// horizon.
const binanceExecutionsPageLimit = 1000

// binanceExecutionsRoute is the path, the cost and the WINDOW CAP per family.
//
// The cap is the finding: 7 days on USDⓈ-M — "the time between startTime and
// endTime cannot be longer than 7 days" — and TWENTY-FOUR HOURS on spot — "the
// time between startTime and endTime can't be longer than 24 hours". Same venue,
// same parameter names, a factor of seven apart.
//
// The spot weight is the conditional one: "Without orderId | 20 · With orderId |
// 5". This sweep is per-instrument and per-window rather than per-order (§4:
// "product+symbol-scoped … per-instrument cursor"), so it never carries an
// orderId and the honest charge is 20. Charging 5 would under-count the most
// expensive read this system makes by a factor of four against the budget that
// bans the IP.
func binanceExecutionsRoute(p ids.ProductCategory) (string, ratelimit.Cost, time.Duration, error) {
	switch p {
	case ids.ProductSpot:
		return "/api/v3/myTrades", ratelimit.Cost{Weight: 20}, 24 * time.Hour, nil
	case ids.ProductLinearPerp:
		return "/fapi/v1/userTrades", ratelimit.Cost{Weight: 5}, 7 * 24 * time.Hour, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, 0, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, 0, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceTradeWire is one row of either family's trade list.
//
// # The two families state the DIRECTION differently, and both spellings are here
//
// USDⓈ-M returns `"side":"SELL"` and also a `buyer` boolean; spot returns only
// `isBuyer`. A struct holding one of them decodes the other family's rows into a
// zero value that reads as a legitimate BUY, which is a fill entered into the
// ledger with the wrong sign — the one error in this file that changes the net
// delta rather than failing.
//
// # There is NO client order id on either family, and that is finding 4
//
// The USDⓈ-M response is {buyer, commission, commissionAsset, id, maker,
// orderId, price, qty, quoteQty, realizedPnl, side, positionSide, symbol, time}
// and spot's is {symbol, id, orderId, orderListId, price, qty, quoteQty,
// commission, commissionAsset, time, isBuyer, isMaker, isBestMatch}. Neither
// carries clientOrderId. So on Binance a fill can be attributed to one of OUR
// orders only through the venue's own orderId — see Execution.ClientOrderID.
type binanceTradeWire struct {
	ID              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	Symbol          string `json:"symbol"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`

	// Side is USDⓈ-M's spelling and is absent on spot.
	Side string `json:"side"`

	// IsBuyer is spot's spelling and is absent on USDⓈ-M. A pointer, so "the
	// venue said false" and "the venue did not say" are different states —
	// without it a missing field decodes to false and silently means SELL.
	IsBuyer *bool `json:"isBuyer"`
}

// DecodeExecutions implements OrderQueryCodec.
func (c *BinanceOrderCodec) DecodeExecutions(instrument ids.VenueInstrument, assets InstrumentAssets, w ExecutionWindow, resp transport.Response) (ExecutionPage, error) {
	var wire []binanceTradeWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return ExecutionPage{}, fmt.Errorf("binance executions: %w", err)
	}

	out := make([]Execution, 0, len(wire))
	for i, t := range wire {
		if t.Symbol != instrument.Symbol {
			return ExecutionPage{}, fmt.Errorf("binance execution %d is for symbol %q, swept %q: %w",
				i, t.Symbol, instrument.Symbol, ErrExecutionsUnreadable)
		}
		side, err := binanceTradeSide(t)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("binance execution %d: %w", i, err)
		}
		qty, err := fixedpoint.ParseQuantity(assets.Base, t.Qty)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("binance execution %d qty %q: %w", i, t.Qty, err)
		}
		price, err := fixedpoint.ParsePrice(assets.Base, assets.Quote, t.Price)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("binance execution %d price %q: %w", i, t.Price, err)
		}
		if t.CommissionAsset == "" {
			// The fee's asset is not cosmetic: §5.1 has no separate FeeAsset
			// column precisely because the Money carries its own tag, so an
			// empty one produces a fee that reconciles against nothing. A
			// Binance fill can legitimately be paid in BNB against a USDT
			// notional, which is the case that makes guessing wrong.
			return ExecutionPage{}, fmt.Errorf("binance execution %d carries no commissionAsset: %w",
				i, ErrExecutionsUnreadable)
		}
		fee, err := fixedpoint.ParseMoney(fixedpoint.Asset(t.CommissionAsset), t.Commission)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("binance execution %d commission %q: %w", i, t.Commission, err)
		}

		out = append(out, Execution{
			ExecutionID: ids.ExecutionID(fmt.Sprintf("%d", t.ID)),
			Instrument:  instrument,

			// Deliberately empty — see binanceTradeWire. Attribution on this
			// venue runs through VenueOrderID.
			ClientOrderID: "",
			VenueOrderID:  fmt.Sprintf("%d", t.OrderID),
			Side:          side,
			Quantity:      qty,
			Price:         price,
			Fee:           fee,
			At:            time.UnixMilli(t.Time).UTC(),
		})
	}

	// Binance hands back no page token — `fromId` is mutually exclusive with the
	// time window on both families — so the continuation is nil and the shared
	// builder resumes by time, which its ASCENDING order makes correct.
	return newExecutionPage(out, w, len(wire) >= binanceExecutionsPageLimit, nil)
}

// binanceTradeSide reads whichever of the two direction fields this family sent,
// and refuses a row that sent neither.
//
// Refusing is the whole point. The obvious decoder reads one field, and on the
// other family that field is absent, decodes to its zero value and names a real
// direction — so every fill on one product family enters the ledger as a BUY.
// Nothing fails, the quantities are right, and the net delta is wrong by twice
// the swept volume.
func binanceTradeSide(t binanceTradeWire) (OrderSide, error) {
	switch {
	case t.Side != "" && t.IsBuyer != nil:
		// Both spellings present is USDⓈ-M's `side` beside its `buyer` boolean
		// under a different name, or a venue change. Either way this decoder
		// would be choosing which one to believe, so it does not.
		return "", fmt.Errorf("execution states both side %q and isBuyer: %w", t.Side, ErrExecutionsUnreadable)

	case t.Side != "":
		switch t.Side {
		case "BUY":
			return OrderSideBuy, nil
		case "SELL":
			return OrderSideSell, nil
		}
		return "", fmt.Errorf("execution side %q: %w", t.Side, ErrOrderSideInvalid)

	case t.IsBuyer != nil:
		if *t.IsBuyer {
			return OrderSideBuy, nil
		}
		return OrderSideSell, nil
	}
	return "", fmt.Errorf("execution states neither side nor isBuyer: %w", ErrExecutionsUnreadable)
}

// Compile-time proof that the codec satisfies the read half too (§3.6 rule 2).
var _ OrderQueryCodec = (*BinanceOrderCodec)(nil)
