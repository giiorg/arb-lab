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

	// ErrOrdersMissing reports a use case constructed without the write
	// repository — §6's APPLY half.
	//
	// It is one sentinel for both use cases rather than one each, unlike the two
	// above, and the asymmetry is worth a sentence. Those name a port that
	// belongs to ONE use case's job (the placer is the dispatch path's alone), so
	// the message can say what the missing wire was for. This port is the same
	// wire in both, and a caller branching on it takes the same action either
	// way: the composition root wired a use case with no current-state store.
	//
	// What it costs when absent: the planner would journal an intent and apply it
	// nowhere, so §6 step 5's record would exist and the order it describes would
	// be unknown to every subsequent read — including Dispatch's own load, which
	// is what makes an unplanned dispatch impossible to express.
	ErrOrdersMissing = errors.New("oms: use case has no order write repository")

	// ErrPlanEmpty reports a TradePlan with no legs.
	//
	// Refused rather than treated as a no-op returning zero records, because the
	// two readings differ in exactly the case that matters: a caller whose leg
	// construction silently produced nothing gets a successful plan, dispatches
	// none of it, and reports a trade that was planned. §6 step 5 exists to make
	// intent durable BEFORE submission; a plan with no intent has nothing to be
	// durable about.
	ErrPlanEmpty = errors.New("oms: trade plan has no legs")

	// ErrLegAlreadyPlanned reports a plan whose minted ClientOrderID the write
	// repository already holds.
	//
	// # It is checked BEFORE anything is journaled, and that ordering is the
	// # whole point of the check
	//
	// ApplyPlanned would refuse the duplicate on its own — domain.Ledger.Register
	// does, with the diagnosis of why neither replacing nor ignoring is safe — so
	// this looks like a second home for one rule. It is not, because the two
	// checks catch it at different moments and only the earlier one is in time.
	//
	// §5.3: a ClientOrderID is "unique per submission; never reused within the
	// recoverable window". The journal is append-only. So a duplicate detected at
	// APPLY time has already been journaled, and the journal now holds two
	// planned records under one ClientOrderID — with two different intents, if
	// anything about the order changed between the attempts. §7.5's replay then
	// has two answers to "what was this order" and no rule for which wins, which
	// is precisely the ambiguity ClientOrderID uniqueness exists to prevent.
	//
	// Detected first, nothing is written at all and the caller replans with a
	// fresh attempt — which is what ClientOrderRef.Attempt is for.
	ErrLegAlreadyPlanned = errors.New("oms: a leg's client order id is already known to the write repository")

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
