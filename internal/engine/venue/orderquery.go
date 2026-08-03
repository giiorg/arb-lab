package venue

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is §4's OrderGateway read half, venue-side: OrderByClientID and the
// executions sweep, plus the two normalized results they answer with.
//
// # It is shaped by state-machines §2's finality contract, not by the endpoints
//
// The four steps are normative and they are what a caller in slice D will run:
// tear down the dispatch path, "read terminal order status with the
// authoritative final cumulative executed quantity", "sweep executions through a
// venue/server watermark strictly LATER than the order's cancellation-
// effectiveness / max landing horizon — never a synthetic stable-not-found
// alone", and reconcile position to that same watermark.
//
// Deriving the types from those four steps rather than from the venues' response
// shapes is the whole reason this slice could keep its own number: the query
// methods have no Go consumer until slice D declares one, so a shape answerable
// only to the endpoints would be frozen against nothing. Two of the three
// findings below were found by asking the contract, not the venues.
//
// # [SPEC EDIT OWED] §4's two read signatures are both WIDER here, and both
// # widenings are §6.7's requirements rather than conveniences
//
// §4 declares:
//
//	OrderByClientID(OrderRef) (OrderStatus, error)
//	Fills(VenueInstrument, since Cursor) ([]Fill, Cursor, error)
//
// Three departures, each stated so external review can tell change from drift:
//
//  1. The sweep takes a WINDOW (Since + Until), not a bare `since`. §6.7 step 3
//     sweeps "through a venue/server watermark strictly LATER than the order's
//     max landing horizon", and a signature with no horizon in it cannot say
//     which watermark it reached. All three venue surfaces return data when the
//     times are omitted and prove coverage through nothing, so a defaulted
//     window is not a substitute — see ExecutionWindow.
//  2. `Cursor` is a venue SERVER INSTANT, not a venue resume handle. §4 leaves
//     the type open and the obvious filling of it makes step 3 inexpressible —
//     see Cursor.
//  3. Both methods take InstrumentAssets and a ratelimit.Priority. The first is
//     the money kernel's requirement, the second is §7.6's shed class, which
//     C2a already added to Place as an argument for want of any carrier.
//
// The result types are wider too: OrderStatus carries a PRESENCE and a REACH
// (§52c, §52d), and the sweep returns a page carrying a watermark and its own
// continuation. Owed to core §4, and to §5.3 for the retention bound.

// OrderRef identifies one order to a venue.
//
// §4: "OrderRef{Instrument VenueInstrument, ClientOrderID} — spot/perp differ".
// The instrument is not decoration: spot BTCUSDT and perp BTCUSDT have
// independent order-ID namespaces, live behind different endpoints and — on
// Binance — different hosts, so the same ClientOrderID string can name two
// unrelated orders.
type OrderRef struct {
	// Instrument is which market the order was placed on.
	Instrument ids.VenueInstrument

	// ClientOrderID is the handle §5.3's recovery keys on.
	ClientOrderID ids.ClientOrderID
}

// Validate reports whether the reference names an order a venue could be asked
// about.
func (r OrderRef) Validate() error {
	if err := r.Instrument.Validate(); err != nil {
		return fmt.Errorf("order ref instrument: %w", err)
	}
	if r.ClientOrderID == "" {
		return ErrOrderClientIDUnset
	}
	return nil
}

// OrderPresence is whether the venue had a record of the order at all.
//
// # It is a VALUE and not an error, and that is this slice's second finding
//
// The two venues put "no such order" in opposite layers. Binance answers with an
// error envelope — code -2013, "Order does not exist." — which C1's classifier
// correctly turns into an *apperror.Error carrying EffectNotApplied. Bybit
// answers with retCode 0 and an empty list, which the envelope reader correctly
// calls a success. So the venue that states the semantic answer inside a
// TRANSPORT ERROR is Binance, exactly inverting C2a's finding, where Bybit stated
// a refusal inside a success status.
//
// Returning Binance's -2013 as an error would be a true statement about the
// QUERY — it was refused, nothing happened — and a category error about the
// ORDER. What the caller needs is "the venue answered, and its answer is: no such
// order", which is the INPUT to §5.3's stable-not-found rule. As an error it is
// indistinguishable at the call site from a query that was rate-limited or timed
// out, and §5.3 forbids precisely that conflation: "'not submitted' is never
// inferred from a transient not-found", and "if the resolving query is
// unavailable/rate-limited, new trading stays disabled".
//
// So an absence and a failure are different RETURNS, not one return with a
// sentinel inside it. The caller that cannot get this wrong is the one for which
// the wrong thing is not expressible.
type OrderPresence string

const (
	// OrderPresent means the venue returned a record for the order.
	OrderPresent OrderPresence = "present"

	// OrderAbsent means the venue answered, and answered that it has no such
	// order on the surfaces queried.
	//
	// It is NOT "the order was never placed". What it licenses depends on how
	// far back the answering surface can see — see QueryReach and §5.3's
	// venue-specific stable-not-found rule, which is slice D's.
	OrderAbsent OrderPresence = "absent"
)

// Validate reports whether p is one of the declared readings.
func (p OrderPresence) Validate() error {
	switch p {
	case OrderPresent, OrderAbsent:
		return nil
	}
	return fmt.Errorf("%q: %w", string(p), ErrOrderPresenceInvalid)
}

func (p OrderPresence) String() string { return string(p) }

// OrderState is the venue's account of one order's lifecycle, normalized.
//
// # Reading a status HERE does not contradict §4's Ack refusing to read one
//
// OrderAck deliberately carries no status, because both venues return one on the
// create response and the private order stream reports the same events — two
// authorities over one order with no rule for which wins. That argument is about
// the SUBMISSION path. This is the resolving query, and §6.7 step 2 requires it
// in terms: "read terminal order status with the authoritative final cumulative
// executed quantity". The stream is the authority while the stream is working;
// this surface exists for when it was not, which is the only time it is asked.
type OrderState string

const (
	// OrderStateOpen is accepted and still able to execute.
	OrderStateOpen OrderState = "open"

	// OrderStatePartiallyFilled has executed some quantity and can execute more.
	OrderStatePartiallyFilled OrderState = "partially_filled"

	// OrderStateFilled executed in full. Terminal.
	OrderStateFilled OrderState = "filled"

	// OrderStateTerminated is over, without an explicit venue rejection, and its
	// remainder will not execute. It may carry executions. Terminal.
	//
	// # Neither venue's own word is used, and there is a reason for that
	//
	// The obvious pair of members is Canceled ("ended by us") and Expired
	// ("ended by the order type's rules"), and the two venues make that split
	// UNNORMALIZABLE. This system sends nothing but marketable IOC limits (§6
	// step 6), so its ordinary terminal outcome is an IOC that did not fill in
	// full — and the three surfaces name that one event three ways:
	//
	//	Binance (both families)   EXPIRED
	//	Bybit linear             Cancelled  ("in derivatives, orders with this
	//	                                      status may have an executed qty")
	//	Bybit spot               PartiallyFilledCanceled  ("only spot has this")
	//
	// Bybit has no expired-by-rule status at all, so a Canceled/Expired split
	// would put the SAME real event in different normalized members depending on
	// which venue answered. That is worse than not making the distinction: a
	// member downstream code can branch on, filled differently per venue, is how
	// a per-venue accident becomes a rule nobody wrote.
	//
	// Nothing downstream needs who ended it. §6.7 needs terminal-or-not and the
	// cumulative executed quantity; §5.3 needs the one distinction below.
	OrderStateTerminated OrderState = "terminated"

	// OrderStateRejected is an EXPLICIT venue rejection. Terminal.
	//
	// This is the one distinction §5.3 does need, and it is kept because it is
	// an EVIDENCE class rather than a cause: a rejection is the venue saying it
	// refused, which no amount of not-finding-something can establish. That is
	// the whole of AbsentAfterFinality's separateness from Rejected, and both
	// venues state it in one word — Binance REJECTED, Bybit Rejected.
	OrderStateRejected OrderState = "rejected"
)

// Validate reports whether s is one of the declared states.
func (s OrderState) Validate() error {
	switch s {
	case OrderStateOpen, OrderStatePartiallyFilled, OrderStateFilled,
		OrderStateTerminated, OrderStateRejected:
		return nil
	}
	return fmt.Errorf("%q: %w", string(s), ErrOrderStateInvalid)
}

// Terminal reports whether the venue can still execute more of this order.
//
// Exhaustive over the declared members (§3.6 rule 10): a state added later that
// is silently non-terminal would let §6.7 collapse a possible-delta interval that
// has not collapsed.
func (s OrderState) Terminal() (bool, error) {
	switch s {
	case OrderStateFilled, OrderStateTerminated, OrderStateRejected:
		return true, nil
	case OrderStateOpen, OrderStatePartiallyFilled:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrOrderStateInvalid)
}

func (s OrderState) String() string { return string(s) }

// QuerySurface names which of a venue's order surfaces answered.
//
// # Not observability — §5.3 needs it to know what an absence is worth
//
// Bybit resolves an order across TWO surfaces with different retention:
// GET /v5/order/realtime holds "the most recent 500 orders" and, after "a server
// release or restart", closed orders "should only be queried through order
// history"; GET /v5/order/history holds the last 7 days for closed statuses but
// only the last 24 HOURS for Cancelled, Rejected and Deactivated. So "absent from
// realtime" and "absent from history" are different claims, and only the composed
// one is admissible input to the stable-not-found rule.
type QuerySurface string

const (
	// SurfaceOrderStatus is a venue's point lookup on live+recent orders:
	// Binance GET /fapi/v1/order and GET /api/v3/order, Bybit
	// GET /v5/order/realtime.
	SurfaceOrderStatus QuerySurface = "order_status"

	// SurfaceOrderHistory is Bybit's GET /v5/order/history, reached only when
	// the status surface came back empty. §5.3 makes the fallback normative:
	// "resolution MUST fall back to GET /v5/order/history by orderLinkId …
	// never infer 'not submitted' from realtime alone".
	SurfaceOrderHistory QuerySurface = "order_history"
)

// Validate reports whether s names a declared surface.
func (s QuerySurface) Validate() error {
	switch s {
	case SurfaceOrderStatus, SurfaceOrderHistory:
		return nil
	}
	return fmt.Errorf("%q: %w", string(s), ErrQuerySurfaceInvalid)
}

func (s QuerySurface) String() string { return string(s) }

// QueryReach is how far back the answering surface can see, per the venue's own
// published retention.
//
// # This exists because the stable-not-found rule has an EXPIRY DATE
//
// Neither venue keeps orders forever, and each one's cliff lands on this system's
// most common outcome. Binance USDⓈ-M will not find an order when "order status
// is CANCELED or EXPIRED AND order has NO filled trade AND created time + 3 days
// < current time" — and an unfilled marketable IOC is exactly a CANCELED-or-
// EXPIRED order with no filled trade. Bybit's history drops Cancelled, Rejected
// and Deactivated orders after 24 HOURS. Past those horizons the query cannot
// distinguish "the order existed and expired unfilled" from "the order never
// existed", while §5.3 maps a stable not-found to AbsentAfterFinality — whose
// whole meaning is PROVED absence.
//
// The exposure reading is unchanged either way, which is what makes it easy to
// miss: both readings contribute zero delta, so nothing is mis-hedged and no
// invariant fires. What is wrong is the evidence class, which §5.3 spends a
// paragraph keeping distinct from Rejected, and which §7.5's reconcile-only boot
// consumes as truth after downtime that can trivially exceed three days.
//
// This layer REPORTS the horizon and does not apply it. Bounding
// AbsentAfterFinality by it is slice D's, recorded against the construct that
// triggers it per §3.6's deferral rule: the stable-not-found rule itself.
type QueryReach struct {
	// Surface is which surface produced the answer.
	Surface QuerySurface

	// UnfilledRetention is how long after creation this surface still returns an
	// order that reached a terminal state with NO execution.
	//
	// Zero is not "forever" — it is refused, per the zero-value rule. A venue
	// that publishes no retention gets ReachUnpublished, which says the
	// unknown out loud instead of encoding it as an unbounded promise.
	UnfilledRetention time.Duration

	// FilledRetention is the same for an order that has at least one execution.
	FilledRetention time.Duration

	// Unpublished reports that the venue states no retention for this surface,
	// so neither duration above is a venue fact.
	Unpublished bool
}

// Validate reports whether the reach is one a caller may reason from.
func (r QueryReach) Validate() error {
	if err := r.Surface.Validate(); err != nil {
		return fmt.Errorf("query reach: %w", err)
	}
	if r.Unpublished {
		if r.UnfilledRetention != 0 || r.FilledRetention != 0 {
			return fmt.Errorf("%s: %w", r.Surface, ErrQueryReachContradictory)
		}
		return nil
	}
	if r.UnfilledRetention <= 0 || r.FilledRetention <= 0 {
		return fmt.Errorf("%s: %w", r.Surface, ErrQueryReachUnset)
	}
	return nil
}

// OrderStatus is one venue's whole answer about one order.
type OrderStatus struct {
	// Presence is whether the venue had the order. Read this FIRST — every
	// field below is meaningful only when it is OrderPresent.
	Presence OrderPresence

	// Reach is how far back the answering surface can see, which is what says
	// whether an OrderAbsent is worth anything.
	Reach QueryReach

	// State is the venue's lifecycle reading.
	State OrderState

	// ExecutedQuantity is §6.7 step 2's "authoritative final cumulative executed
	// quantity", in the instrument's base asset.
	ExecutedQuantity fixedpoint.Quantity

	// VenueOrderID is the venue's own identifier, carried for the same reason
	// OrderAck carries it: an operator reconciling against a venue UI has only
	// this number to search by.
	VenueOrderID string
}

// Cursor is a resume point in a venue's execution history.
//
// # It is a venue SERVER TIME, and this slice's first finding is why
//
// §4 declares Fills(VenueInstrument, since Cursor) and says nothing about what a
// Cursor is. The three surfaces offer three different resume handles, and the
// idiomatic choice — "whatever the venue hands back" — implements §4's signature
// completely and correctly on all three while making §6.7 step 3 impossible:
//
//	Binance USDⓈ-M  fromId          ascending trade id      not comparable to a time
//	Binance spot    fromId          ascending trade id      not comparable to a time
//	Bybit           nextPageCursor  opaque, DESCENDING      not comparable to a time
//
// §6.7 step 3 sweeps "through a venue/server watermark strictly LATER than the
// order's cancellation-effectiveness / max landing horizon". That is a comparison
// against a time horizon, and none of the three handles above can be compared to
// one. So the persisted cursor is a server INSTANT and the venue's own handle is
// demoted to what it actually is: a paging detail inside one sweep.
//
// The venues corroborate this a second way. On both Binance families fromId and
// the time window are mutually exclusive — futures says "the parameter fromId
// cannot be sent with startTime or endTime", spot publishes a whitelist of
// parameter combinations containing no fromId+time pair — so a sweep bounded by a
// §6.7 watermark cannot ALSO resume by id. The watermark wins; the id is not the
// cursor.
//
// # There is no zero Cursor meaning "from the beginning"
//
// The beginning does not exist: every surface here is retention-bounded, so a
// sweep is always FROM an instant the caller states. A zero value that quietly
// meant "everything" would be the usual unpopulated-field failure wearing a
// legitimate meaning.
type Cursor struct {
	// At is the venue's own server time for the resume point. The sweep
	// re-reads this instant rather than starting after it — see the type doc.
	At time.Time
}

// Validate reports whether the cursor names a resume point.
func (c Cursor) Validate() error {
	if c.At.IsZero() {
		return ErrCursorUnset
	}
	return nil
}

// InstrumentAssets is the base/quote decomposition of an instrument.
//
// # The read path needs a fact the write path never did
//
// OrderRequest RECEIVES typed money — a Quantity and a Price the caller already
// built — so the write path never had to know what "BTCUSDT" decomposes into. The
// read path MINTS money from venue strings, and the money kernel refuses to build
// a Quantity or a Price without its asset tag, so the decomposition has to arrive
// from somewhere. ids.VenueInstrument does not carry it: Symbol is "the venue's
// own routing symbol", deliberately unparsed, because splitting it is a per-venue
// convention and a wrong split is a Money whose asset tag is a lie the runtime
// check then enforces against everything else.
//
// So it is a parameter, stated at each call site, rather than a guess made here.
type InstrumentAssets struct {
	// Base is what the quantity is denominated in.
	Base fixedpoint.Asset

	// Quote is what the price is denominated in.
	Quote fixedpoint.Asset
}

// Validate reports whether both assets were populated.
//
// fixedpoint.Asset is a bare string with no Validate of its own — deliberately,
// because the legal set is discovered from exchange metadata rather than known
// at compile time — so emptiness is the only check available and it is the one
// that matters: an unpopulated asset tag builds a Money the runtime check then
// enforces against every other Money, and the first failure is far from here.
func (a InstrumentAssets) Validate() error {
	if a.Base == "" || a.Quote == "" {
		return fmt.Errorf("base %q quote %q: %w", a.Base, a.Quote, ErrInstrumentAssetsUnset)
	}
	if a.Base == a.Quote {
		return fmt.Errorf("both %q: %w", a.Base, ErrInstrumentAssetsIdentical)
	}
	return nil
}

// ExecutionWindow is one bounded sweep of a venue's execution history.
//
// # Until is REQUIRED, and a defaulted window cannot replace it
//
// All three surfaces return data when the time parameters are omitted — Binance
// USDⓈ-M "the last 7 days' data", Binance spot "the most recent trades", Bybit
// "return 7 days by default". Every one of those gets you executions and none of
// them proves coverage THROUGH a particular instant, which is the only thing
// §6.7 step 3 wants. The proof is that the venue was asked through Until and
// answered; that is why the horizon is sent rather than defaulted, and why a
// sweep that returns nothing still advances the watermark to Until.
type ExecutionWindow struct {
	// Since is where the sweep resumes, inclusive.
	Since Cursor

	// Until is the horizon the sweep must cover, inclusive.
	Until time.Time

	// Continuation is the venue's own handle for the next page OF THIS WINDOW,
	// empty on the first call.
	//
	// # It is a separate field from Since because the two point OPPOSITE WAYS
	//
	// Since is between-sweep and moves forward in time. A venue's paging handle
	// is within-sweep and moves in whatever direction that venue lists results,
	// which on Bybit is BACKWARD: the execution list is "sorted by execTime in
	// descending order", so nextPageCursor advances into older data. The two are
	// both "a string you hand back to the next call", and storing a
	// nextPageCursor where a Since belongs resumes at the OLDEST record of the
	// previous sweep and re-reads the same window forever — while every request
	// succeeds, every page decodes, and the fill ledger's dedup silently absorbs
	// the duplicates. Nothing fails.
	//
	// Binance leaves this empty on both families: `fromId` cannot be combined
	// with a time window there, so its within-window paging is expressed by
	// advancing Since instead, which its ascending order makes correct.
	Continuation string
}

// Validate reports whether the window is one a venue could be asked for.
func (w ExecutionWindow) Validate() error {
	if err := w.Since.Validate(); err != nil {
		return fmt.Errorf("execution window: %w", err)
	}
	if w.Until.IsZero() {
		return ErrExecutionWindowUnset
	}
	if !w.Until.After(w.Since.At) {
		return fmt.Errorf("execution window %s..%s: %w",
			w.Since.At.UTC().Format(time.RFC3339Nano), w.Until.UTC().Format(time.RFC3339Nano),
			ErrExecutionWindowInverted)
	}
	return nil
}

// Execution is one fill, normalized.
//
// It is minted here rather than reaching for oms/domain.Fill, for §4's layering
// reason (engine/venue never imports a bounded context) and for a practical one:
// domain.Fill carries an Account, which is an OMS concept this layer has no way
// to populate.
type Execution struct {
	// ExecutionID is the venue's immutable fill id — Binance tradeId, Bybit
	// execId — unique only WITHIN an instrument (§5.1).
	ExecutionID ids.ExecutionID

	// Instrument is which market, and it is what makes ExecutionID unique.
	Instrument ids.VenueInstrument

	// ClientOrderID is the order this fill belongs to, where the venue reports
	// it. It may be empty on a surface that reports only the venue's own id.
	ClientOrderID ids.ClientOrderID

	// VenueOrderID is the venue's identifier for the order.
	VenueOrderID string

	// Side is the direction. Single statement of direction: Quantity is
	// unsigned, as on OrderRequest.
	Side OrderSide

	// Quantity is the executed size in the instrument's base asset.
	Quantity fixedpoint.Quantity

	// Price is the execution price.
	Price fixedpoint.Price

	// Fee is what the venue charged, carrying its own asset tag — which is why
	// §5.1 has no separate FeeAsset column.
	Fee fixedpoint.Money

	// At is the venue's server time for the execution. This is the quantity the
	// cursor is built from.
	At time.Time
}

// ExecutionPage is one sweep's answer.
type ExecutionPage struct {
	// Executions are the fills observed, OLDEST FIRST.
	//
	// Normalized to ascending regardless of the venue's own sort, because a
	// caller folding these into a ledger and taking the last one's instant as a
	// watermark is right on Binance and silently wrong on Bybit.
	Executions []Execution

	// Watermark is how far this sweep has PROVEN coverage — the field §6.7
	// step 3 actually needs, and it is always meaningful.
	//
	// A page holding fewer rows than the venue's page limit exhausted the
	// window, so coverage is proven through the window's Until — including the
	// EMPTY page, because "nothing happened through T" is a positive answer and
	// the commonest one. A full page proves coverage through nothing beyond its
	// own last row, so the watermark stops there.
	Watermark Cursor

	// More is the rest of this window, or nil when the window was exhausted.
	//
	// A whole window rather than a bare token, because the two venues need
	// different things to continue and a bare string would be Bybit's
	// backward-walking page cursor sitting in the same shaped slot as Binance's
	// forward-moving instant. Nil-or-not is the only state a caller has to
	// branch on, and a caller that ignores it gets a truncated sweep it can
	// still not mistake for a complete one, because Watermark did not reach
	// Until.
	More *ExecutionWindow
}

// OrderQueryCodec is one venue's translation of the order READ surface.
//
// Kept on the same OrderCodec as the write half rather than segregated, because
// §5.3 requires a resolving query for every order this system places: a venue
// that could place but not resolve is not a venue this system may trade on, so
// the segregation would model something that cannot exist.
type OrderQueryCodec interface {
	// EncodeOrderStatus renders the point lookup for one order on the given
	// surface.
	EncodeOrderStatus(ref OrderRef, surface QuerySurface) (transport.Call, error)

	// DecodeOrderStatus reads the venue's answer, including its absence.
	//
	// It takes the REF for the same reason DecodePlaceAck takes the request: a
	// venue echoing a different client order id means the answer belongs to
	// another order, and resolving one order's state from another's is the
	// failure the recovery handle exists to prevent.
	DecodeOrderStatus(ref OrderRef, assets InstrumentAssets, surface QuerySurface, resp transport.Response) (OrderStatus, error)

	// AbsenceFromError reports whether a FAILED call is this venue stating that
	// it has no such order, rather than a failure to answer at all.
	//
	// # This method exists because Binance puts a semantic answer in an error
	//
	// The transport classifies before any codec sees the body: SignedClient.Do
	// turns Binance's {"code":-2013,"msg":"Order does not exist."} into an
	// *apperror.Error and returns it, so DecodeOrderStatus is never reached. The
	// classification is right — a 4xx with a documented refusal code IS
	// EffectNotApplied for the request — and it is an answer about the QUERY.
	// Reading the ORDER's absence out of it is a venue fact, so it lives on the
	// venue's codec; Bybit's implementation returns false for everything,
	// because on Bybit an absence never arrives as an error at all.
	AbsenceFromError(err error) bool

	// FallbackSurface reports the next surface to try when one answered
	// OrderAbsent, and false when this venue has none left.
	FallbackSurface(tried QuerySurface) (QuerySurface, bool)

	// EncodeExecutions renders one bounded sweep.
	//
	// It returns ErrExecutionWindowTooWide when the window exceeds what the
	// surface will serve, rather than truncating it: a truncated sweep reporting
	// success is a false watermark, and a false watermark is what §6.7 step 3
	// collapses a possible-delta interval on. Chunking is the caller's, because
	// the cap is a venue fact and the loop is a policy.
	EncodeExecutions(instrument ids.VenueInstrument, w ExecutionWindow) (transport.Call, error)

	// DecodeExecutions reads one page and states the watermark it reached.
	DecodeExecutions(instrument ids.VenueInstrument, assets InstrumentAssets, w ExecutionWindow, resp transport.Response) (ExecutionPage, error)

	// StatusReach reports how far back a surface can see, per the venue's own
	// published retention (§52d). It is on the codec because it is a venue fact,
	// and it is a method rather than a table so that a venue publishing nothing
	// says so — see QueryReach.Unpublished.
	StatusReach(instrument ids.VenueInstrument, surface QuerySurface) (QueryReach, error)
}

// OrderByClientID resolves one order's state at the venue.
//
// §4: "product-routed; resolve SubmissionUnknown — NEVER blind-resubmit". This
// method never places anything and has no path that could.
//
// # The fallback loop is the venue's rule, executed here
//
// A venue whose first surface answers OrderAbsent may have another surface that
// knows better — on Bybit that is normative, because realtime holds only "the
// most recent 500 orders" and is cleared on a server restart. The codec decides
// which surface is next; this loop only runs it, so Binance (which has one
// surface) reaches the same code with nothing venue-specific in it.
//
// The returned OrderStatus always names the surface that ANSWERED, which for an
// absence is the last one tried.
//
// # Urgency here admits all THREE of §7.6's classes, where an order admits two
//
// OrderRequest refuses ratelimit.PriorityRoutine, because no §5.3 command places
// an order under §7.6's read-traffic class. This surface is the other half of
// that sentence: Routine IS "the reads that keep the risk view live", and a
// §6.7 finality query is Safety, and a scheduled reconciliation sweep is neither
// urgent nor safety-critical. All three are correct answers here for different
// callers, so the parameter is validated and not narrowed — the narrowing on the
// write path was the venue fact, not a house style to mirror.
func (c *OrderClient) OrderByClientID(ctx context.Context, ref OrderRef, assets InstrumentAssets, urgency ratelimit.Priority) (OrderStatus, error) {
	op := string(c.codec.Venue()) + ".OrderByClientID"

	if err := assets.Validate(); err != nil {
		return OrderStatus{}, apperror.Precondition(op, "assets", err)
	}
	client, err := c.route(op, ref, urgency)
	if err != nil {
		return OrderStatus{}, err
	}

	surface := SurfaceOrderStatus
	for {
		call, err := c.codec.EncodeOrderStatus(ref, surface)
		if err != nil {
			return OrderStatus{}, apperror.Precondition(op, "encode", err)
		}
		call.Priority = urgency

		var status OrderStatus
		resp, err := client.Do(ctx, call)
		switch {
		case err != nil && c.codec.AbsenceFromError(err):
			// The venue answered, and its answer was "no such order" — which on
			// Binance arrives as a classified transport error (code -2013). The
			// error is discarded HERE and nowhere else: it has served its whole
			// purpose, which was to carry a semantic answer through a layer that
			// only speaks in effects.
			status = OrderStatus{Presence: OrderAbsent}

		case err != nil:
			// SignedClient.Do already classified the effect, and §5.3 forbids
			// re-deriving it here. A query that could not be answered and an
			// order that is not there must not leave by the same path — that is
			// the whole of §52c, and it is why the case above is a value.
			return OrderStatus{}, err

		default:
			status, err = c.codec.DecodeOrderStatus(ref, assets, surface, resp)
			if err != nil {
				return OrderStatus{}, apperror.Precondition(op, "decode", err)
			}
		}

		// The reach is attached HERE rather than inside the decoder, because it
		// is a property of the surface that answered and the decoder is handed
		// one response at a time. Attaching it in the loop is what makes an
		// absence from Bybit's realtime surface and an absence from its history
		// surface different values rather than the same empty list twice.
		reach, err := c.codec.StatusReach(ref.Instrument, surface)
		if err != nil {
			return OrderStatus{}, apperror.Precondition(op, "reach", err)
		}
		if err := reach.Validate(); err != nil {
			return OrderStatus{}, apperror.Precondition(op, "reach", err)
		}
		status.Reach = reach

		if status.Presence == OrderPresent {
			return status, nil
		}

		next, ok := c.codec.FallbackSurface(surface)
		if !ok {
			return status, nil
		}
		surface = next
	}
}

// Executions sweeps one instrument's fills across a bounded window.
//
// §4: "product+symbol-scoped (venues require it); per-instrument cursor". Both
// scopings are the venues': every surface here requires a symbol, and every
// execution id is unique only within an instrument (§5.1).
//
// One call is one PAGE. Walking to the horizon is the caller's loop, for the
// reason the window cap is refused rather than truncated — how far to sweep and
// how much budget to spend on it is §6.7's policy, and a client that looped
// internally would spend an unbounded number of §7.6 acquisitions inside one call.
func (c *OrderClient) Executions(ctx context.Context, instrument ids.VenueInstrument, assets InstrumentAssets, w ExecutionWindow, urgency ratelimit.Priority) (ExecutionPage, error) {
	op := string(c.codec.Venue()) + ".Executions"

	if err := w.Validate(); err != nil {
		return ExecutionPage{}, apperror.Precondition(op, "window", err)
	}
	if err := assets.Validate(); err != nil {
		return ExecutionPage{}, apperror.Precondition(op, "assets", err)
	}
	client, err := c.route(op, OrderRef{Instrument: instrument, ClientOrderID: executionsSweepRef}, urgency)
	if err != nil {
		return ExecutionPage{}, err
	}

	call, err := c.codec.EncodeExecutions(instrument, w)
	if err != nil {
		return ExecutionPage{}, apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		return ExecutionPage{}, err
	}

	page, err := c.codec.DecodeExecutions(instrument, assets, w, resp)
	if err != nil {
		return ExecutionPage{}, apperror.Precondition(op, "decode", err)
	}
	return page, nil
}

// newExecutionPage normalizes one venue's rows into a page and decides the
// watermark.
//
// # The three decisions here are shared because they are §6.7's, not a venue's
//
// Sorting ascending, stopping the watermark at the last row of a full page, and
// re-reading the tied instant are all consequences of "sweep executions through a
// venue/server watermark", so they belong in one place rather than once per
// codec — which is also what stops the two codecs drifting into two different
// readings of the same clause.
//
// `full` is the codec's answer to "did this page hit the venue's limit", because
// the limit is a venue fact. `more` is the codec's continuation, non-nil only
// when it has one.
func newExecutionPage(rows []Execution, w ExecutionWindow, full bool, more *ExecutionWindow) (ExecutionPage, error) {
	for i, e := range rows {
		if e.At.Before(w.Since.At) || e.At.After(w.Until) {
			// A row outside the window means the request was not the one this
			// page answers — a defaulted window, a dropped parameter, a venue
			// ignoring a bound. Admitting it would fold executions into the
			// ledger that the watermark does not account for, which is the
			// silent half of a false watermark.
			return ExecutionPage{}, fmt.Errorf("execution %d at %s is outside the swept window %s..%s: %w",
				i, e.At.UTC().Format(time.RFC3339Nano),
				w.Since.At.UTC().Format(time.RFC3339Nano), w.Until.UTC().Format(time.RFC3339Nano),
				ErrExecutionsUnreadable)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].At.Equal(rows[j].At) {
			return rows[i].At.Before(rows[j].At)
		}
		// Bybit states execTime is not unique — "when execTime matches, it is
		// recommended to sort according to execId+OrderId+leavesQty" — so a
		// time-only comparison leaves tied rows in whatever order the venue sent
		// them, and two sweeps of the same window would produce two orderings of
		// the same data. The id is the tiebreak that makes this function a
		// function.
		return rows[i].ExecutionID < rows[j].ExecutionID
	})

	if !full || len(rows) == 0 {
		// The window is exhausted, so the venue has answered for every instant
		// up to the horizon we asked about. This is the arm the empty page takes
		// as well, and it is why a sweep that finds nothing still advances the
		// watermark.
		return ExecutionPage{Executions: rows, Watermark: Cursor{At: w.Until}}, nil
	}

	last := rows[len(rows)-1].At
	if more == nil {
		// No continuation, so the only way onward is to resume by time — which
		// works on an ASCENDING venue and is why Binance leaves Continuation
		// empty. The next window starts AT the last instant rather than after
		// it: fills dedupe on (account, venue, product, symbol, execution_id),
		// so a re-read costs a row check and a skip costs an execution the net
		// delta is computed without. Duplicates are free here; gaps are not.
		if !last.After(w.Since.At) {
			// Every row in a full page shares the resume instant, so an
			// inclusive cursor cannot move. A refusal rather than a loop that
			// issues venue calls forever while every request succeeds.
			return ExecutionPage{}, fmt.Errorf("a full page of %d executions is all at %s: %w",
				len(rows), last.UTC().Format(time.RFC3339Nano), ErrExecutionsNoProgress)
		}
		more = &ExecutionWindow{Since: Cursor{At: last}, Until: w.Until}
	}
	return ExecutionPage{Executions: rows, Watermark: Cursor{At: last}, More: more}, nil
}

// executionsSweepRef is the placeholder client order id the sweep's routing
// check carries. A sweep is not about one order, and route's validation is
// per-order; giving it a named constant keeps the reason legible where an empty
// string would read as a bug.
const executionsSweepRef ids.ClientOrderID = "executions-sweep"

// route is the shared precondition for every read call: the reference is one
// this client may act on, and the product family has a transport.
//
// Split out of Place's body rather than duplicated, and the shared part is
// exactly the part §51e argued about — an unconfigured family is REFUSED, never
// defaulted, because "BTCUSDT" is a real instrument on both Binance hosts.
func (c *OrderClient) route(op string, ref OrderRef, urgency ratelimit.Priority) (*transport.SignedClient, error) {
	if err := ref.Validate(); err != nil {
		return nil, apperror.Precondition(op, "order ref", err)
	}
	if err := urgency.Validate(); err != nil {
		return nil, apperror.Precondition(op, "urgency", err)
	}
	if ref.Instrument.Venue != c.codec.Venue() {
		return nil, apperror.Precondition(op, "venue",
			fmt.Errorf("order %q is for %s: %w", ref.ClientOrderID, ref.Instrument.Venue, ErrOrderVenueMismatch))
	}
	client, ok := c.transports[ref.Instrument.Product]
	if !ok {
		return nil, apperror.Precondition(op, "route",
			fmt.Errorf("order %q on %s: %w", ref.ClientOrderID, ref.Instrument.Product, ErrOrderFamilyUnrouted))
	}
	return client, nil
}
