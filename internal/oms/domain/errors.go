package domain

import "errors"

// The domain's refusals. Each is a sentinel so a caller can branch on the kind
// without matching text, and every construction site wraps it with the value
// that failed — one sentinel cannot say WHICH check fired, so the message is the
// diagnosis somebody acts on and the sentinel only says where to look.
var (
	// ErrOrderStateUnset and ErrOrderStateUnknown report an OrderState that is
	// empty and one that is neither declared value.
	//
	// Separate, on the argument events.StreamKind and marketdata's BookSide both
	// make: an empty state is a field nobody populated — a hydration that missed
	// a column, a zero-valued struct — while a misspelt one is a mapping from a
	// venue token or a stored row we do not know. The two send a reader to
	// different files, and here the first is the more dangerous: an order at the
	// zero state is one the recovery path cannot classify at all.
	ErrOrderStateUnset   = errors.New("oms: order state unset")
	ErrOrderStateUnknown = errors.New("oms: order state is not a declared value")

	// ErrOrderStateUnhandled reports a DECLARED order state that the transition
	// table has no case for.
	//
	// Distinct from ErrOrderStateUnknown, which is about a value that is not a
	// state at all. This one says the value is a perfectly good state and the
	// table forgot it — the §3.6 rule 10 failure, arriving at runtime because
	// something removed the gate that makes it a build failure. It denies rather
	// than permits: an unhandled state has no successors, so every transition out
	// of it is refused and an operator sees an order that will not move, rather
	// than an order that moves anywhere.
	ErrOrderStateUnhandled = errors.New("oms: order state has no transition-table entry")

	// ErrSideInvalid, ErrOrderTypeInvalid and ErrTimeInForceInvalid report an
	// order attribute that is neither declared value.
	//
	// One sentinel each rather than one shared "invalid order field", because the
	// three send a reader to different places: a bad side is a mapping bug in an
	// adapter's direction table, a bad type is usually a venue order kind nothing
	// has taught us to read, and a bad TIF is the field venues spell most
	// differently. Each is also, unlike OrderState, a single sentinel covering
	// both the empty and the misspelt value — see Side.Validate for why the
	// two-way split OrderState takes does not pay for itself on a two-member set.
	ErrSideInvalid        = errors.New("oms: order side is not a declared value")
	ErrOrderTypeInvalid   = errors.New("oms: order type is not a declared value")
	ErrTimeInForceInvalid = errors.New("oms: order time in force is not a declared value")

	// ErrOrderTypeUnhandled reports a DECLARED order type that a decision site
	// has no case for — ErrOrderStateUnhandled's counterpart one type over, and
	// it exists for the same reason: it makes the fallthrough of an exhaustive
	// switch observable, so the group that switch does NOT report on stays
	// testable.
	ErrOrderTypeUnhandled = errors.New("oms: order type has no case at this decision site")

	// ErrMarketOrderHasLimitPrice reports a market order carrying a price.
	//
	// The refusal runs in the direction nobody writes. "A limit order needs a
	// price" is the check that gets written and it fails loudly at the venue
	// anyway; this one fails silently everywhere. §6 step 4 sizes the naked-leg
	// price gap against the price the order is bounded by, so a market order with
	// a limit price presents a bound the venue ignores — the order fills through
	// it, and the slippage budget reconciles against a number that never bound
	// anything.
	ErrMarketOrderHasLimitPrice = errors.New("oms: market order carries a limit price")

	// ErrTransitionForbidden reports an order state change that is not an edge of
	// SM §1.
	//
	// The refusal is the point rather than the bookkeeping. Two of the forbidden
	// edges are things a plausible implementation does: returning a
	// DispatchAuthorized order to Planned ("the send never went out"), and
	// entering Rejected on a stable-not-found ("the venue does not have it").
	// §5.3 forbids the first outright — the single Place may have executed with
	// its response lost — and FLAG-2 forbids the second, because proved absence
	// and an explicit venue rejection are different evidence classes.
	ErrTransitionForbidden = errors.New("oms: order state transition is not an edge of the lifecycle")

	// ErrTransitionNoOp reports a transition from a state to itself.
	//
	// Refused rather than admitted as a harmless identity, because admitting it
	// would make the terminal claim untrue as stated: "Filled has no outgoing
	// edges" is what a reader relies on, and Filled -> Filled is an outgoing
	// edge. SM §1's one "stays" row (DispatchAuthorized on a crash) is the
	// ABSENCE of a transition, not a transition to the same place, and a caller
	// expressing it calls nothing.
	ErrTransitionNoOp = errors.New("oms: order state transition is to the same state")

	// ErrExecutionIDUnset reports a fill carrying no venue execution ID.
	//
	// It is the identity the whole dedupe rests on, and an empty one collapses
	// every unidentified fill for an instrument into a single key. The direction
	// of that failure is what makes it worth a named refusal rather than a
	// tolerated blank: the SECOND such fill is silently dropped as a duplicate,
	// so the position the ledger reports is short of venue reality by every fill
	// after the first, and nothing in the pipeline errors.
	ErrExecutionIDUnset = errors.New("oms: fill carries no execution ID")

	// ErrAccountUnset reports a fill naming no account.
	//
	// §5.1 makes Account the venue subaccount and puts it FIRST in the dedupe
	// key, because one deployment holds several. Blank, two subaccounts trading
	// the same symbol share an execution-ID namespace they do not actually
	// share — and venue execution IDs are per-instrument sequences, so the
	// collision is not hypothetical.
	ErrAccountUnset = errors.New("oms: fill names no account")

	// ErrFillQuantityNotPositive reports a fill of zero size or less.
	//
	// Zero is the one that gets written. A venue message with no execution
	// quantity is an order-lifecycle event that an adapter routed to the
	// execution stream — §4 notes fills arrive on both StreamOrders and
	// StreamExecutions — and admitting it puts a row in the ledger that changes
	// no quantity but consumes its dedupe key. A real fill arriving later with
	// that execution ID is then refused as a duplicate.
	ErrFillQuantityNotPositive = errors.New("oms: fill quantity is not positive")

	// ErrFillUnknownOrder reports a fill whose ClientOrderID names no order the
	// ledger holds.
	//
	// §5.3's never-closing ledger is scoped to a KNOWN order — "any deduped fill
	// for a *known* order regardless of its current state" — and that scope is
	// load-bearing. A fill for an unknown client ID is one of two things, and
	// neither is a ledger entry: a foreign order on the account (§6 step 10 sends
	// those to operator clearance) or a journal that lost the order it was
	// supposed to hold. Accepting it would let the second masquerade as the
	// first, since a ledger that invents orders on demand always reconciles.
	ErrFillUnknownOrder = errors.New("oms: fill names an order the ledger does not hold")

	// ErrFillInstrumentMismatch reports a fill whose instrument is not the one
	// its order was placed on.
	//
	// §5.1 makes spot BTCUSDT and perp BTCUSDT distinct instruments with
	// independent execution-ID namespaces, and a cash-and-carry trade holds one
	// order on EACH at the same moment. So the two legs' fills are mutually
	// plausible: swap them and both quantities are real, both prices are near
	// each other, and the net delta the §6.7 classifier computes reads as
	// perfectly hedged while each leg is individually wrong.
	ErrFillInstrumentMismatch = errors.New("oms: fill instrument is not the order's instrument")

	// ErrFillTimestampUnset reports a fill with no venue execution instant.
	//
	// The zero time is about fifty-five years ago, which is the reason it is
	// refused rather than tolerated as "unknown": every consumer that windows on
	// it — §5.1's funding-settlement keys, the §6 step 9 PnL decomposition,
	// §7.5's reconcile-only boot ordering fills against a recovery cursor —
	// sorts it before every real record and inside every lookback. It is
	// therefore not dropped from a window; it is included in all of them.
	//
	// (There is deliberately NO sentinel here for fills on one order whose
	// quantity assets disagree. fixedpoint.Quantity.Add already refuses that,
	// and a second check would be a second copy of one rule — the shape that
	// produced a surviving mutant in marketdata, where a helper validated what
	// its caller had already validated and no test could tell which check fired.
	// The Ledger wraps fixedpoint's refusal with the order and the execution
	// instead.)
	ErrFillTimestampUnset = errors.New("oms: fill carries no execution timestamp")

	// ErrDuplicateOrder reports registering a ClientOrderID the ledger already
	// holds.
	//
	// §5.3 makes a ClientOrderID "unique per submission" and "never reused within
	// the recoverable window". Silently replacing the existing entry would
	// discard its cumulative execution — the fills stay applied at the venue and
	// vanish from our count — and silently keeping it would attach the new
	// order's fills to the old order's instrument. Both are recoveries that look
	// like they worked.
	ErrDuplicateOrder = errors.New("oms: ledger already holds this client order ID")

	// ErrOrderUnset reports an order registered with no client order ID.
	ErrOrderUnset = errors.New("oms: order carries no client order ID")

	// ErrOrderedQuantityNotPositive reports an order registered for zero size or
	// less.
	//
	// Zero is the reading that would otherwise go through, and it inverts the
	// §6.7 classification rather than merely being odd: an order for nothing has
	// no remaining executable quantity from the instant it is registered, so the
	// classifier reads its residual as DETERMINED and its possible-delta
	// interval as the point zero — while the venue holds a live order whose size
	// nobody recorded. The compensating trade is then sized against a delta that
	// omits it.
	ErrOrderedQuantityNotPositive = errors.New("oms: ordered quantity is not positive")
)
