package application

import "errors"

// The use-case layer's refusals.
var (
	// ErrJournalMissing and ErrPlacerMissing report a Dispatcher constructed
	// without one of its ports.
	//
	// Refused at construction rather than at the first call, because the call
	// that would discover it is the one that submits an order. A nil journal
	// found there means the crash boundary silently did not exist for an order
	// already on its way to a venue.
	ErrJournalMissing = errors.New("oms: dispatcher has no journal")
	ErrPlacerMissing  = errors.New("oms: dispatcher has no order placer")

	// ErrOrderNotOMSMinted reports a dispatch of an order whose ClientOrderID
	// this system did not mint.
	//
	// domain.Order deliberately admits such ids — §6 step 10's foreign orders
	// carry venue-minted ones and the reconciler must be able to hold them — so
	// the check belongs here, at the one place where an id stops being something
	// we READ and becomes something we WRITE to a venue.
	//
	// What it costs when absent: §5.3 makes the ClientOrderID "a recovery
	// correlation handle", and ids.ParseClientOrderID is strict precisely so a
	// Binance "autoclose-…" cannot be read as a trade we placed. Submitting an
	// id that will not parse produces an order whose handle is unusable at the
	// exact moment it is needed — recovery reads it back, fails to parse it,
	// and routes a trade of ours to operator clearance as a foreign order.
	ErrOrderNotOMSMinted = errors.New("oms: client order id was not minted by this system")

	// ErrNotMarketableIOCLimit reports a dispatch of an order §6 step 6 does not
	// permit this system to send.
	//
	// §6 step 6 fixes the execution pattern: "parallel marketable-IOC limit
	// orders (never raw MARKET)". The domain can represent the others because
	// recovery holds orders we did not send; sending one is a different act.
	//
	// The check is a comparison against the two permitted values rather than a
	// switch over the enums, and the direction is the point: a member added to
	// OrderType or TimeInForce next year is refused by this check without anyone
	// editing it. An exhaustive switch would be the loud shape everywhere else in
	// this repo and the wrong one here, because it would have to be re-answered —
	// and answered wrongly once — for a value nobody intended to send.
	ErrNotMarketableIOCLimit = errors.New("oms: only marketable-IOC limit orders may be dispatched")

	// ErrOrderAlreadyDispatched reports a second Dispatch for one ClientOrderID
	// in this process.
	//
	// §5.3's single-Place rule is the invariant this whole path exists to keep,
	// and its durable enforcement is the journal: recovery finds an order at
	// DispatchAuthorized and resolves it by query, never by resubmitting. This
	// guard is the in-process backstop for the bug that never reaches recovery —
	// a caller that treats a failed dispatch as retryable when the failure
	// happened AFTER the send.
	//
	// It is a backstop rather than the mechanism, and the distinction matters:
	// a fresh process starts with an empty set, which is correct, because an
	// order it finds at DispatchAuthorized is one it must query about rather
	// than dispatch again.
	ErrOrderAlreadyDispatched = errors.New("oms: order has already been dispatched in this process")

	// ErrEffectUnhandled reports a Place failure whose apperror.Effect the
	// classifier has no case for.
	//
	// It is advisory: the order still gets a state, and that state is
	// SubmissionUnknown — the answer that sends the OMS to the venue to ask,
	// which is right for any effect whose meaning we do not know. The error
	// exists so the gap is reported rather than absorbed, and so the classifier's
	// fallthrough is distinguishable from its SubmissionUnknown cases by a test.
	//
	// Without it the fallthrough would return the same (state, nil) pair as the
	// EffectUnknown case, and dropping a member from that case would change
	// nothing any test could observe — the trailing-return trap this repo
	// diagnosed in domain.IsTerminal, one layer up.
	ErrEffectUnhandled = errors.New("oms: place failure carries an effect the classifier has no case for")
)
