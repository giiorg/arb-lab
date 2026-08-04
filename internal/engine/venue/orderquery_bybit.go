package venue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit V5's order READ surface (Profiles §8).
//
// # Two surfaces for one question, and §5.3 makes the second one normative
//
// GET /v5/order/realtime holds live orders plus, verbatim, "the most recent 500
// orders" of the closed ones — and "after a server release or restart, filled,
// cancelled, and rejected orders of Unified account should ONLY be queried
// through order history". So an absence from realtime is not an absence: it is
// also what a Bybit deploy looks like. §5.3: "resolution MUST fall back to
// GET /v5/order/history by orderLinkId … never infer 'not submitted' from
// realtime alone".
//
// # An absence arrives as a SUCCESS here, which is the mirror of C2a's finding
//
// Bybit answers a query for an unknown orderLinkId with retCode 0 and an empty
// result list, so ClassifyResponse sees a clean success and the emptiness is the
// answer. Binance puts the same answer in an error envelope. C2a found Bybit
// stating a REFUSAL inside a success status; this is the same seam with the
// venues swapped, and it is why OrderPresence is a value rather than an error.
//
// # ONE fact below is assumed rather than verified, and it is named as such
//
// That Bybit reports an unknown orderLinkId as retCode 0 with an empty list — as
// opposed to a non-zero retCode — could not be verified without a credential: the
// order surfaces are all signed, and the credential-free probes that settled §6
// and §7 reach only public endpoints. Task #171 owns the testnet credential.
//
// The code below is written so that the answer does not matter. An empty list
// yields OrderAbsent, and a non-zero retCode never reaches this decoder at all —
// ClassifyResponse turns it into a classified error, and AbsenceFromError
// declines to read an absence out of any of them. So the unverified branch fails
// CLOSED: if Bybit does signal absence by a code, this codec reports a query that
// could not be answered rather than an order that is not there, which is the
// conservative half of §5.3's rule and the half a wrong guess should land on.

// EncodeOrderStatus implements OrderQueryCodec.
func (c *BybitOrderCodec) EncodeOrderStatus(ref OrderRef, surface QuerySurface) (transport.Call, error) {
	if err := c.checkOrderLinkID(ref.ClientOrderID); err != nil {
		return transport.Call{}, err
	}
	category, err := bybitCategory(ref.Instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	path, limit, err := bybitOrderStatusRoute(surface)
	if err != nil {
		return transport.Call{}, err
	}

	return transport.Call{
		Op:     string(c.venue) + ".OrderByClientID",
		Method: "GET",
		Path:   path,
		Params: transport.Params{
			{Key: "category", Value: category},
			{Key: "symbol", Value: ref.Instrument.Symbol},
			{Key: "orderLinkId", Value: string(ref.ClientOrderID)},
		},
		Family: ref.Instrument.Product,

		// Bybit publishes no per-endpoint weight; its trade limits are requests
		// per second per endpoint, and both read surfaces are 50/s. Every
		// Acquire charges one request by construction, so there is nothing to
		// add — and nothing to invent, which is the point: a fabricated weight
		// would be charged against a budget the venue does not meter.
		// RequestOnly is that claim with a carrier: the bare zero Cost reads as
		// the field nobody filled in and Cost.Validate refuses it before
		// Acquire, which made this call path always-refused until task #173.
		Cost:  ratelimit.RequestOnly(),
		Limit: limit,
	}, nil
}

// bybitOrderStatusRoute is the path and §7.6 profile limit name per surface.
//
// Exhaustive over the enum-like (§3.6 rule 10). The two surfaces are separate
// limit names because Bybit meters per endpoint — folding them into one budget
// would let the fallback spend the primary's allowance during exactly the
// incident (a Bybit restart) that makes every open order need the fallback at
// once.
func bybitOrderStatusRoute(s QuerySurface) (string, string, error) {
	switch s {
	case SurfaceOrderStatus:
		return "/v5/order/realtime", BybitOrderRealtimeLimit, nil
	case SurfaceOrderHistory:
		return "/v5/order/history", BybitOrderHistoryLimit, nil
	}
	return "", "", fmt.Errorf("%q: %w", string(s), ErrQuerySurfaceInvalid)
}

// The §7.6 profile limit names Bybit's read surfaces report against. Exported
// for the reason BybitOrderCreateLimit is: the profile that DECLARES the limit
// and the call that charges it must agree on the string, and a second spelling
// means the header fold attributes to a limit nobody declared.
const (
	BybitOrderRealtimeLimit = "order_realtime"
	BybitOrderHistoryLimit  = "order_history"
	BybitExecutionListLimit = "execution_list"
)

// bybitOrderWire is one row of either order surface. The two return the same
// order object, which is why one struct serves both.
type bybitOrderWire struct {
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
	Symbol      string `json:"symbol"`
	OrderStatus string `json:"orderStatus"`
	CumExecQty  string `json:"cumExecQty"`
}

// bybitListWire is the envelope both surfaces wrap their rows in.
type bybitListWire struct {
	Result struct {
		Category       string           `json:"category"`
		List           []bybitOrderWire `json:"list"`
		NextPageCursor string           `json:"nextPageCursor"`
	} `json:"result"`
}

// DecodeOrderStatus implements OrderQueryCodec.
//
// # The empty list is the absence, and a second row is a refusal
//
// A query by orderLinkId names one order, so a list of two is a venue answering
// a question that was not asked — a dropped filter, a symbol collision across
// categories. Taking the first row would resolve one order's state from a set
// that might not contain it.
func (c *BybitOrderCodec) DecodeOrderStatus(ref OrderRef, assets InstrumentAssets, surface QuerySurface, resp transport.Response) (OrderStatus, error) {
	if err := surface.Validate(); err != nil {
		return OrderStatus{}, err
	}

	var wire bybitListWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return OrderStatus{}, fmt.Errorf("bybit order status: %w", err)
	}

	switch len(wire.Result.List) {
	case 0:
		// retCode 0 with nothing in it: the venue answered, and its answer is
		// that this surface has no such order. Not an error — see the file
		// header and OrderPresence.
		return OrderStatus{Presence: OrderAbsent}, nil
	case 1:
	default:
		return OrderStatus{}, fmt.Errorf("bybit order status for %q returned %d orders: %w",
			ref.ClientOrderID, len(wire.Result.List), ErrAckUnreadable)
	}

	row := wire.Result.List[0]
	if row.OrderLinkID != string(ref.ClientOrderID) {
		return OrderStatus{}, fmt.Errorf("bybit order status echoes order link id %q, asked for %q: %w",
			row.OrderLinkID, ref.ClientOrderID, ErrAckUnreadable)
	}
	if row.Symbol != ref.Instrument.Symbol {
		return OrderStatus{}, fmt.Errorf("bybit order status is for symbol %q, asked for %q: %w",
			row.Symbol, ref.Instrument.Symbol, ErrAckUnreadable)
	}

	state, err := bybitOrderState(row.OrderStatus)
	if err != nil {
		return OrderStatus{}, err
	}
	executed, err := fixedpoint.ParseQuantity(assets.Base, row.CumExecQty)
	if err != nil {
		return OrderStatus{}, fmt.Errorf("bybit order status cumExecQty %q: %w", row.CumExecQty, err)
	}

	return OrderStatus{
		Presence:         OrderPresent,
		State:            state,
		ExecutedQuantity: executed,
		VenueOrderID:     row.OrderID,
	}, nil
}

// bybitOrderState maps Bybit's orderStatus onto the normalized lifecycle.
//
// # A terminal Bybit order may carry executions, and the docs say so twice
//
// "Cancelled — in derivatives, orders with this status may have an executed qty",
// and spot has its own PartiallyFilledCanceled. So cumExecQty is read on EVERY
// state rather than only the filled ones: an implementation that shortcuts a
// terminal-but-not-Filled order to zero executed quantity loses real fills, and
// loses them precisely on this system's ordinary outcome — a marketable IOC that
// took part of the book.
//
// Untriggered and Triggered are conditional-order states this system cannot
// create (§6 step 6 sends IOC limits and nothing else) and they are still mapped
// rather than refused, in the direction that is safe if the impossible happens:
// NOT terminal, so §6.7 keeps the interval open instead of collapsing one that
// has not.
func bybitOrderState(status string) (OrderState, error) {
	switch status {
	case "New":
		return OrderStateOpen, nil
	case "PartiallyFilled":
		return OrderStatePartiallyFilled, nil
	case "Untriggered", "Triggered":
		return OrderStateOpen, nil
	case "Filled":
		return OrderStateFilled, nil
	case "Cancelled", "PartiallyFilledCanceled", "Deactivated":
		return OrderStateTerminated, nil
	case "Rejected":
		return OrderStateRejected, nil
	}
	return "", fmt.Errorf("bybit order status %q: %w", status, ErrOrderStateInvalid)
}

// AbsenceFromError implements OrderQueryCodec.
//
// Always false, and that is a venue fact rather than a stub. On Bybit an absence
// is a SUCCESS carrying an empty list, so nothing that arrives as an error is
// one — every classified error here is a query that could not be answered.
//
// It is also where the file header's one unverified assumption is made safe: if
// Bybit turns out to signal an unknown orderLinkId with a retCode after all,
// this returning false means the caller sees a failed query rather than a proved
// absence. Wrong in the direction §5.3 tolerates.
func (c *BybitOrderCodec) AbsenceFromError(error) bool { return false }

// FallbackSurface implements OrderQueryCodec — §5.3's normative realtime→history
// fallback.
//
// Exhaustive over the enum-like: history has no successor, and a member added
// later must state its own place in the chain rather than inherit "no fallback"
// from a default arm, which is how a surface would silently stop being consulted.
func (c *BybitOrderCodec) FallbackSurface(tried QuerySurface) (QuerySurface, bool) {
	switch tried {
	case SurfaceOrderStatus:
		return SurfaceOrderHistory, true
	case SurfaceOrderHistory:
		return "", false
	}
	return "", false
}

// StatusReach implements OrderQueryCodec — §52d's horizons, per surface.
//
// # History's cliff for a REFUSED order is twenty-four hours
//
// Verbatim: "the orders in the last 7 days: support querying all closed status
// except 'Cancelled', 'Rejected', 'Deactivated' status. The orders in the last 24
// hours: orders with 'Cancelled', 'Rejected', 'Deactivated' can be queried. The
// orders beyond 7 days: supports querying orders which have fills only."
//
// Read against §6 step 6, that is the same collision Binance's 3-day rule is: an
// unfilled marketable IOC ends Cancelled on Bybit, so THIS system's commonest
// terminal order is queryable for one day. §5.3's own parenthetical — "a filled
// order is always recoverable there" — survives, and it is the ONLY one of the
// three rows that does.
//
// Realtime's reach is not a duration at all. It is "the most recent 500 orders",
// cleared on a server restart — a COUNT and an event, neither of which is a time.
// Reported Unpublished rather than approximated, because any duration written
// here would be this project's guess wearing a venue fact's clothes.
func (c *BybitOrderCodec) StatusReach(_ ids.VenueInstrument, surface QuerySurface) (QueryReach, error) {
	switch surface {
	case SurfaceOrderStatus:
		return QueryReach{Surface: SurfaceOrderStatus, Unpublished: true}, nil
	case SurfaceOrderHistory:
		return QueryReach{
			Surface:           SurfaceOrderHistory,
			UnfilledRetention: 24 * time.Hour,
			FilledRetention:   7 * 24 * time.Hour,
		}, nil
	}
	return QueryReach{}, fmt.Errorf("%q: %w", string(surface), ErrQuerySurfaceInvalid)
}

// EncodeExecutions implements OrderQueryCodec.
func (c *BybitOrderCodec) EncodeExecutions(instrument ids.VenueInstrument, w ExecutionWindow) (transport.Call, error) {
	if err := w.Validate(); err != nil {
		return transport.Call{}, err
	}
	category, err := bybitCategory(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	if span := w.Until.Sub(w.Since.At); span > bybitExecutionsWindowCap {
		// "endTime - startTime <= 7 days", published as a hard constraint.
		return transport.Call{}, fmt.Errorf("bybit sweep of %s exceeds the published %s cap: %w",
			span, bybitExecutionsWindowCap, ErrExecutionWindowTooWide)
	}

	params := transport.Params{
		{Key: "category", Value: category},
		{Key: "symbol", Value: instrument.Symbol},
		{Key: "startTime", Value: fmt.Sprintf("%d", w.Since.At.UnixMilli())},
		{Key: "endTime", Value: fmt.Sprintf("%d", w.Until.UnixMilli())},
		{Key: "limit", Value: fmt.Sprintf("%d", bybitExecutionsPageLimit)},
	}
	if w.Continuation != "" {
		// The venue's own page handle, and it walks BACKWARD — see
		// ExecutionWindow.Continuation. It is appended last and never merged
		// into Since, which is the whole reason the two are separate fields.
		params = append(params, transport.Param{Key: "cursor", Value: w.Continuation})
	}

	return transport.Call{
		Op:     string(c.venue) + ".Executions",
		Method: "GET",
		Path:   "/v5/execution/list",
		Params: params,
		Family: instrument.Product,
		// Request-only for OrderByClientID's reason, stated there.
		Cost:  ratelimit.RequestOnly(),
		Limit: BybitExecutionListLimit,
	}, nil
}

const (
	// bybitExecutionsPageLimit is the published maximum, "[1, 100]".
	//
	// It is a QUARTER of Binance's 1000, so the same swept span costs ten times
	// the round trips here. Taken rather than defaulted (the default is 50) for
	// the reason the Binance one is: a page that ends early ends the watermark
	// early.
	bybitExecutionsPageLimit = 100

	// bybitExecutionsWindowCap is "endTime - startTime <= 7 days".
	bybitExecutionsWindowCap = 7 * 24 * time.Hour
)

// bybitExecutionWire is one row of GET /v5/execution/list.
//
// Unlike Binance's trade rows, this one CARRIES orderLinkId — so on Bybit a fill
// can be attributed to one of our orders directly, and on Binance it cannot.
// That asymmetry is finding 4; see Execution.ClientOrderID.
type bybitExecutionWire struct {
	ExecID      string `json:"execId"`
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	ExecQty     string `json:"execQty"`
	ExecPrice   string `json:"execPrice"`
	ExecFee     string `json:"execFee"`
	FeeCurrency string `json:"feeCurrency"`
	ExecTime    string `json:"execTime"`
}

type bybitExecutionListWire struct {
	Result struct {
		List           []bybitExecutionWire `json:"list"`
		NextPageCursor string               `json:"nextPageCursor"`
	} `json:"result"`
}

// DecodeExecutions implements OrderQueryCodec.
//
// # The rows arrive NEWEST FIRST and leave oldest first
//
// Bybit lists executions "sorted by execTime in descending order". The shared
// page builder sorts ascending, so this decoder does not reverse anything itself
// — but the DIRECTION still shows up here, in what a full page means. On an
// ascending venue a full page means more data AFTER the last row, reachable by
// advancing the time cursor. Here it means more data BEFORE the first row, which
// advancing the time cursor would skip entirely. That is why a full page hands
// back nextPageCursor and does not touch Since.
func (c *BybitOrderCodec) DecodeExecutions(instrument ids.VenueInstrument, assets InstrumentAssets, w ExecutionWindow, resp transport.Response) (ExecutionPage, error) {
	var wire bybitExecutionListWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return ExecutionPage{}, fmt.Errorf("bybit executions: %w", err)
	}

	out := make([]Execution, 0, len(wire.Result.List))
	for i, e := range wire.Result.List {
		if e.Symbol != instrument.Symbol {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d is for symbol %q, swept %q: %w",
				i, e.Symbol, instrument.Symbol, ErrExecutionsUnreadable)
		}
		side, err := bybitExecutionSide(e.Side)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d: %w", i, err)
		}
		qty, err := fixedpoint.ParseQuantity(assets.Base, e.ExecQty)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d execQty %q: %w", i, e.ExecQty, err)
		}
		price, err := fixedpoint.ParsePrice(assets.Base, assets.Quote, e.ExecPrice)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d execPrice %q: %w", i, e.ExecPrice, err)
		}
		if e.FeeCurrency == "" {
			// Bybit documents feeCurrency without stating that it is always
			// present, and the fee's asset tag is what §5.1 relies on instead of
			// a separate FeeAsset column. Refused rather than defaulted to the
			// quote asset: Bybit spot can charge in the BASE coin on a buy, so
			// the plausible default is wrong exactly on the cash-and-carry entry
			// leg.
			return ExecutionPage{}, fmt.Errorf("bybit execution %d carries no feeCurrency: %w",
				i, ErrExecutionsUnreadable)
		}
		fee, err := fixedpoint.ParseMoney(fixedpoint.Asset(e.FeeCurrency), e.ExecFee)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d execFee %q: %w", i, e.ExecFee, err)
		}
		at, err := bybitExecTime(e.ExecTime)
		if err != nil {
			return ExecutionPage{}, fmt.Errorf("bybit execution %d: %w", i, err)
		}

		out = append(out, Execution{
			ExecutionID:   ids.ExecutionID(e.ExecID),
			Instrument:    instrument,
			ClientOrderID: ids.ClientOrderID(e.OrderLinkID),
			VenueOrderID:  e.OrderID,
			Side:          side,
			Quantity:      qty,
			Price:         price,
			Fee:           fee,
			At:            at,
		})
	}

	full := len(wire.Result.List) >= bybitExecutionsPageLimit
	var more *ExecutionWindow
	if full && wire.Result.NextPageCursor != "" {
		more = &ExecutionWindow{Since: w.Since, Until: w.Until, Continuation: wire.Result.NextPageCursor}
	}
	if full && more == nil {
		// A full page with no cursor leaves no way to reach the rows behind it,
		// and they are OLDER than everything returned — so a time-based resume
		// would step over them silently. Refused, because the alternative is a
		// watermark that claims a window it did not cover.
		return ExecutionPage{}, fmt.Errorf("bybit returned a full page of %d executions with no nextPageCursor: %w",
			len(out), ErrExecutionsUnreadable)
	}
	return newExecutionPage(out, w, full, more)
}

// bybitExecTime parses the millisecond timestamp Bybit sends as a STRING.
//
// It is a string here and an integer on the order surfaces' createdTime/
// updatedTime — same venue, same unit, two JSON types — so a shared numeric
// decoder would fail on this one field with a type error that names JSON rather
// than the venue.
func bybitExecTime(s string) (time.Time, error) {
	var ms int64
	if _, err := fmt.Sscanf(s, "%d", &ms); err != nil || ms <= 0 {
		return time.Time{}, fmt.Errorf("execTime %q: %w", s, ErrExecutionsUnreadable)
	}
	return time.UnixMilli(ms).UTC(), nil
}

// bybitExecutionSide reads Bybit's title-case direction — the same spelling
// difference from Binance that OrderSide exists for, arriving on the read path.
func bybitExecutionSide(s string) (OrderSide, error) {
	switch s {
	case "Buy":
		return OrderSideBuy, nil
	case "Sell":
		return OrderSideSell, nil
	}
	return "", fmt.Errorf("execution side %q: %w", s, ErrOrderSideInvalid)
}

// Compile-time proof that the codec satisfies the read half too (§3.6 rule 2).
var _ OrderQueryCodec = (*BybitOrderCodec)(nil)
