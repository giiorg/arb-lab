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

	// ErrFillAssetMismatch reports fills on one order whose quantity assets
	// disagree.
	//
	// fixedpoint's asset tag is a runtime field rather than a compile-time type
	// (§3.6 rule 4 — dynamic venue assets cannot be phantom-typed), so this is
	// where the question gets asked. Summing a BTC quantity and an ETH quantity
	// into one cumulative execution is a number with no unit that still compares
	// against a target quantity.
	ErrFillAssetMismatch = errors.New("oms: fills on one order disagree on asset")

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
)
