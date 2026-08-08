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

	// ErrSideUnhandled reports a DECLARED side that a decision site has no case
	// for — ErrOrderStateUnhandled's counterpart on a two-member set, and it
	// exists for the same reason rather than because two members feel like
	// enough: without it, PossibleDeltaFor's switch would end in a trailing
	// return and one of the two directions would be untestable.
	ErrSideUnhandled = errors.New("oms: order side has no case at this decision site")

	// ErrHedgeGroupAccountUnset and ErrHedgeGroupSymbolUnset report an
	// incompletely populated §5.1 hedge-group key.
	//
	// The account one is the dangerous half and it is the half a struct literal
	// omits most easily. §5.1 makes the key (account, venue, symbol) and one
	// deployment holds several subaccounts; with the account blank, every
	// subaccount trading a symbol collapses into one group, and a naked long in
	// one nets against a naked short in another to a residual of ZERO. Positions
	// and margin are per subaccount, so nothing about that netting is real — the
	// group reads flat, no compensating trade is sized, and both legs stay naked
	// with every invariant over the group agreeing.
	ErrHedgeGroupAccountUnset = errors.New("oms: hedge group names no account")
	ErrHedgeGroupSymbolUnset  = errors.New("oms: hedge group names no symbol")

	// ErrOrderOutsideHedgeGroup reports an order handed to the classifier that
	// does not belong to the group being classified.
	//
	// It refuses rather than skipping the order, and the direction is the point:
	// silently skipping computes a residual over a SUBSET while reporting it as
	// the group's, which is the same wrong number as a mis-keyed group and is
	// harder to find because the caller's set looked right. §6.7's verdict is
	// only meaningful over the whole group.
	ErrOrderOutsideHedgeGroup = errors.New("oms: order does not belong to this hedge group")

	// ErrIntervalInverted reports a possible-delta interval whose low endpoint is
	// above its high one.
	//
	// Unreachable through PossibleDeltaFor, which clamps; reachable through
	// arithmetic on a hand-built interval, and worth refusing because an inverted
	// interval breaks the one property every consumer relies on — that Straddles
	// and FloorReduction partition the cases between them. Inverted, a range can
	// be both non-straddling and have no safe endpoint.
	ErrIntervalInverted = errors.New("oms: possible-delta interval is inverted")

	// ErrIntervalStraddlesZero reports a floor magnitude asked of an interval
	// that contains zero in its interior.
	//
	// §6.7: "if the interval straddles zero (a < 0 < b) no nonzero compensation
	// is safe in all states → de-minimis only". The refusal is an error rather
	// than a zero magnitude because "no safe action exists" and "the safe action
	// is to do nothing" are different answers, and only the second may be
	// reported to an operator as the rule complying.
	ErrIntervalStraddlesZero = errors.New("oms: possible-delta interval straddles zero")

	// ErrClassificationUnset and ErrClassificationUnknown report an empty §6.7
	// verdict and one that is not a declared value.
	//
	// Split for OrderState's reason: the empty one is a field nobody populated —
	// a zero-valued struct, a projection row that lost a column — and it is the
	// more dangerous, because a zero Classification is not "ambiguous by
	// default" but a value every switch refuses, so a group carrying one cannot
	// be classified at all rather than being classified safely.
	ErrClassificationUnset   = errors.New("oms: classification unset")
	ErrClassificationUnknown = errors.New("oms: classification is not a declared value")

	// ErrClassificationUnhandled reports a DECLARED verdict a decision site has
	// no case for.
	ErrClassificationUnhandled = errors.New("oms: classification has no case at this decision site")

	// ErrReductionProducerInvalid reports a reserved-reduction producer that is
	// neither declared value.
	ErrReductionProducerInvalid = errors.New("oms: reduction producer is not a declared value")

	// ErrReservationUnset reports a reservation created with no id.
	ErrReservationUnset = errors.New("oms: reservation carries no id")

	// ErrDuplicateReservation reports reserving an id the ledger already holds.
	//
	// Ledger.Register's argument, on the subtrahend rather than the minuend:
	// replacing the entry discards an outstanding commitment whose order is live
	// at the venue, and keeping it attaches a second generation to the first
	// one's amount. §6.7's "never issued twice" is a claim about entries, so an
	// id collision is exactly where it would be defeated.
	ErrDuplicateReservation = errors.New("oms: reserved-reduction ledger already holds this reservation id")

	// ErrUnknownReservation reports amending or releasing an id the ledger does
	// not hold.
	//
	// It refuses rather than creating on demand, on Ledger.Register's argument
	// one layer over: a ledger that invents entries on demand always reconciles,
	// and here the invented entry would be a SUBTRAHEND — so a typo in an id
	// silently reduces the residual and makes a naked group read as hedged.
	ErrUnknownReservation = errors.New("oms: reserved-reduction ledger does not hold this reservation id")

	// ErrReservationAssetMismatch reports amending an entry with a quantity in a
	// different asset from the ledger's.
	//
	// fixedpoint refuses a disagreeing tag on arithmetic, but Amend performs no
	// arithmetic — it stores — so without this check the disagreement would
	// surface later, inside Outstanding's fold, attributed to whichever entry the
	// map happened to visit first.
	ErrReservationAssetMismatch = errors.New("oms: reservation quantity is not the ledger's asset")

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

	// ErrFilterStepNotPositive and ErrFilterTickNotPositive report a
	// TradingFilters grid that is zero or negative. A zero grid admits every
	// value — the unpopulated constraint reading as "no constraint" — and the
	// zero arrives from real venue data, not only from a forgotten field:
	// Binance publishes stepSize "0.00000000" on filters that do not apply.
	ErrFilterStepNotPositive = errors.New("oms: trading filter lot step is not positive")
	ErrFilterTickNotPositive = errors.New("oms: trading filter price tick is not positive")

	// ErrFilterFloorNegative reports a negative MinQty or MinNotional. Zero is
	// a legal declared floor; below it there is no reading that is a venue
	// fact rather than a decode defect.
	ErrFilterFloorNegative = errors.New("oms: trading filter floor is negative")

	// ErrFilterAssetsDisagree reports TradingFilters members carrying
	// disagreeing asset tags — two instruments' rows assembled into one value,
	// or a wrong base/quote split. Caught at Validate so the disagreement is
	// named here rather than surfacing inside a planner check attributed to
	// whichever member the check happened to read.
	ErrFilterAssetsDisagree = errors.New("oms: trading filter members disagree about the instrument's assets")

	// ErrGenerationPhaseUnset and ErrGenerationPhaseUnknown report a
	// GenerationPhase that is empty and one that is neither declared value.
	//
	// Separate for OrderState's reason, and the first is the worse one here too:
	// SM §7's SpotReduceAuthorized is a DURABLE record, so a generation
	// recovered at the zero phase is one the ladder cannot place — it knows
	// neither whether the perp reduction confirmed nor whether a spot sale was
	// authorized, which are the two facts the whole never-net-short sequencing
	// rests on.
	ErrGenerationPhaseUnset   = errors.New("oms: generation phase unset")
	ErrGenerationPhaseUnknown = errors.New("oms: generation phase is not a declared value")

	// ErrGenerationPhaseUnhandled reports a DECLARED phase that a partition or
	// the transition table has no case for — the §3.6 rule 10 failure arriving
	// at runtime. It denies, so an unhandled phase has no successors and the
	// generation stops rather than moving anywhere.
	ErrGenerationPhaseUnhandled = errors.New("oms: generation phase has no case")

	// ErrGenerationNoOp and ErrGenerationTransitionForbidden report a
	// self-transition and an edge SM §7's ladder does not have.
	ErrGenerationNoOp                = errors.New("oms: generation phase transition to itself")
	ErrGenerationTransitionForbidden = errors.New("oms: generation phase transition is not an SM §7 edge")

	// ErrSupersessionPathUnset, ErrSupersessionPathUnknown and
	// ErrSupersessionPathUnhandled report the same three conditions for the
	// three rows that end a generation.
	ErrSupersessionPathUnset     = errors.New("oms: supersession path unset")
	ErrSupersessionPathUnknown   = errors.New("oms: supersession path is not a declared value")
	ErrSupersessionPathUnhandled = errors.New("oms: supersession path has no case")

	// ErrGenerationIDUnset reports a generation with no handle. The identity
	// outlives every order on both legs, so an empty one collapses distinct
	// bounded attempts into whichever the caller reached for last.
	ErrGenerationIDUnset = errors.New("oms: generation id unset")

	// ErrGenerationTargetNotPositive reports a generation opened for zero or
	// less. Zero is the reading that would otherwise go through, and it is not
	// merely useless: it runs the whole ladder, reduces leverage by nothing, and
	// holds §7's one-generation-at-a-time gate closed while it does.
	ErrGenerationTargetNotPositive = errors.New("oms: generation target is not positive")

	// ErrGenerationPremiseNotIntact reports a generation opened over a package
	// that is not still-neutral — SM §7's own entry precondition, which core
	// §5.3 states as "reduce leverage on a STILL-NEUTRAL package".
	ErrGenerationPremiseNotIntact = errors.New("oms: generation premise is not intact at entry")

	// ErrGenerationPremiseDissolved reports a generation asked to take a step
	// that commits new exposure while the premise it opened under has stopped
	// holding — FM-8's standing obligation refusing.
	//
	// Distinct from ErrGenerationPremiseNotIntact, which is the same condition
	// at entry. The two send a reader to different places: the first is a caller
	// that escalated over a package it should not have, and this one is a
	// package that moved UNDERNEATH a live generation, which is the situation
	// whose only correct answer is to supersede and quiesce.
	ErrGenerationPremiseDissolved = errors.New("oms: generation premise dissolved")

	// ErrGenerationLegNotInFlight reports a fill recorded against a generation
	// whose perp leg is not in flight.
	ErrGenerationLegNotInFlight = errors.New("oms: generation leg is not in flight")

	// ErrGenerationFillNotPositive reports a non-positive executed quantity. A
	// zero fill is an event that moves nothing while amending the ledger entry,
	// and a negative one is a sign convention error arriving as data.
	ErrGenerationFillNotPositive = errors.New("oms: generation fill quantity is not positive")

	// ErrGenerationPerpOvershootsTarget reports perp executions exceeding the
	// generation's own per-leg target — a fill attributed to the wrong
	// generation, or one counted twice. Admitting it would raise the reserved
	// entry above anything the generation can sell back, understating the
	// group's residual for as long as the entry lives.
	ErrGenerationPerpOvershootsTarget = errors.New("oms: generation perp executions exceed its target")

	// ErrGenerationSpotSizeNegative and ErrGenerationSpotExceedsConfirmedPerp
	// report the two ways an authorization breaks §7's sequencing. The second is
	// the one that matters: authorizing more spot than the CONFIRMED perp
	// reduction makes the transient net-SHORT in the rally that triggered the
	// generation, which is the direction the whole perp-first ordering exists to
	// avoid.
	ErrGenerationSpotSizeNegative         = errors.New("oms: generation spot authorization is negative")
	ErrGenerationSpotExceedsConfirmedPerp = errors.New("oms: generation spot authorization exceeds the confirmed perp reduction")

	// ErrGenerationSpotExceedsAuthorized reports the spot leg executing more
	// than the durable authorization fixed — exposure the ladder never
	// authorized, and the cap the never-net-short claim rests on.
	ErrGenerationSpotExceedsAuthorized = errors.New("oms: generation spot execution exceeds its authorization")

	// ErrGenerationNotActive reports the PREMISE-LOST row fired against a
	// generation that has already reached §6.7 finality. There is nothing left
	// to take further, so recording a premise loss there would attribute one to
	// an attempt that had already stopped. The two external rows are deliberately
	// NOT guarded on this — see Generation.Supersede.
	ErrGenerationNotActive = errors.New("oms: generation is no longer active")

	// ErrGenerationAlreadySuperseded reports a second supersession. The first
	// path is the one an operator is shown, and overwriting it would name
	// whichever arbitration arrived last.
	ErrGenerationAlreadySuperseded = errors.New("oms: generation has already been superseded")

	// ErrGenerationLegUnidentified reports a leg submitted without its
	// ClientOrderID. The id is what makes the leg netted out of the premise, so
	// an unnamed one leaves the generation's own order inside the quantity that
	// decides whether the generation may continue — FM-9, arriving through the
	// netting instead of through the trigger.
	ErrGenerationLegUnidentified = errors.New("oms: generation leg carries no client order id")

	// ErrGenerationLegNotSubmitted reports an order id supplied for a leg that
	// was never sent — a zero-size spot authorization has nothing on the wire,
	// and an id here would net an order the generation does not own out of its
	// own premise.
	ErrGenerationLegNotSubmitted = errors.New("oms: generation names a leg it did not submit")

	// ErrGenerationLegNotQuiesced reports a supersession attempted while the
	// reserved SECOND leg is still out at the venue. All three of SM §7's rows
	// quiesce it to §6.7 finality FIRST; abandoning a generation without doing
	// so strands the leg, which is the harm the rule exists for.
	ErrGenerationLegNotQuiesced = errors.New("oms: generation reserved leg is not quiesced")

	// ErrFilterObservedAtUnset reports a TradingFilters value with no
	// observation instant. The age question is asked at every package
	// admission (§69), and a zero instant would answer it in whichever
	// direction the comparison happened to take — so it is refused at
	// Validate, where the message names the field rather than the arithmetic.
	ErrFilterObservedAtUnset = errors.New("oms: trading filter observation instant unset")

	// The §7.1 MarginOp record's refusals. Every one of them describes a record
	// a resolution could not be performed against, which at boot is a process
	// unable to say whether a top-up moved real money.
	//
	// ErrMarginOpIDUnset reports a record with no local handle. The venues supply
	// no operation identity of their own (Profiles §4), so this is the only name
	// the operation has anywhere.
	ErrMarginOpIDUnset = errors.New("oms: margin op carries no id")

	// ErrMarginOpNotPerp reports a margin op against a spot instrument. Isolated
	// margin is a property of a derivatives position and neither venue has a
	// surface that would post it against a spot holding.
	ErrMarginOpNotPerp = errors.New("oms: margin op instrument has no isolated-margin surface")

	// ErrMarginOpDeltaZero reports an op that moves nothing — the one amount
	// under which Applied and NotApplied are indistinguishable by any evidence
	// either venue publishes.
	ErrMarginOpDeltaZero = errors.New("oms: margin op delta is zero")

	// ErrMarginOpBeforeNegative reports a negative posted-margin before-state.
	// Neither venue publishes one, so it is a parse that read a sign off the
	// wrong field — and it fails permissively, making every observed movement
	// look larger than it was.
	ErrMarginOpBeforeNegative = errors.New("oms: margin op before-state posted margin is negative")

	// ErrMarginOpAssetsDisagree reports a delta, a before-state and a live
	// reading that are not all in one asset. The three are read from three
	// places, so the mismatch is a wiring defect rather than a venue answer.
	ErrMarginOpAssetsDisagree = errors.New("oms: margin op amounts are in different assets")

	// ErrMarginOpWatermarkUnset reports a record with no venue instant. It is
	// what bounds the evidence window, and a zero one makes every reading look
	// arbitrarily old — the reading that concludes rather than waits.
	ErrMarginOpWatermarkUnset = errors.New("oms: margin op carries no venue watermark")

	// ErrMarginOpStatusUnset and ErrMarginOpStatusInvalid report a resolution
	// outcome that is the zero value or is not a declared member. The first is
	// the realistic accident — a record whose status column was never written —
	// and it must not read as any of the four.
	ErrMarginOpStatusUnset   = errors.New("oms: margin op status unset")
	ErrMarginOpStatusInvalid = errors.New("oms: margin op status is not a declared value")

	// ErrMarginOpBoundNotPositive reports a resolution attempted with no
	// propagation bound. A non-positive one would conclude NotApplied the instant
	// after dispatch, which is the answer that says a top-up did not happen while
	// it is still landing.
	ErrMarginOpBoundNotPositive = errors.New("oms: margin op propagation bound is not positive")

	// ErrMarginOpResolutionInstantUnset reports a resolution with no instant to
	// measure the watermark against — the zero time, which would make every op
	// look as though it were dispatched in the future.
	ErrMarginOpResolutionInstantUnset = errors.New("oms: margin op resolution instant unset")

	// The act-history refusals (task #214). Every one of them is a WIRING defect
	// on the same rule Resolve states: nothing a venue can legitimately answer
	// reaches this channel, because §7.1 attaches a consequence to not knowing and
	// an error a caller logs and moves past has silently lifted it.
	//
	// ErrMarginActCauseUnset and ErrMarginActCauseInvalid report a history row
	// whose cause never decoded. They are NOT the unrecognised-`deltaType` case,
	// which is a first-class value (MarginActUnattributable) and resolves
	// Ambiguous — these two mean no decoder ran at all, which would otherwise
	// count a row this system cannot classify as one it caused.
	ErrMarginActCauseUnset   = errors.New("oms: margin act carries no cause")
	ErrMarginActCauseInvalid = errors.New("oms: margin act cause is not a declared value")

	// ErrMarginActAmountNotPositive reports a row whose recorded amount is zero or
	// negative. The venue publishes an unsigned magnitude with the direction in
	// its own field, so a non-positive one is a decoder that read the sign off the
	// wrong place — and a zero would match the magnitude of an op whose own zero
	// delta is already refused as unresolvable.
	ErrMarginActAmountNotPositive = errors.New("oms: margin act amount is not positive")

	// ErrMarginActInstantUnset reports a row with no venue timestamp. It fails
	// permissively: an unplaceable row cannot be excluded by a window, so it would
	// be a candidate match for every op on the instrument.
	ErrMarginActInstantUnset = errors.New("oms: margin act carries no venue instant")

	// ErrMarginHistoryUnset reports the ZERO MarginActHistory reaching a
	// resolution.
	//
	// It is the refusal the whole three-state design exists for. The alternative
	// spelling — a nil slice standing for "the venue answered with nothing" —
	// makes the value a forgetful caller produces mean the one thing that inverts
	// a verdict: on a venue with no act history a matching delta is Applied, and
	// under a silent-history reading the identical arithmetic is Ambiguous, which
	// blocks openings until an operator clears it.
	ErrMarginHistoryUnset = errors.New("oms: margin act history reading unset")

	// ErrMarginHistoryAbsentWithRows reports rows attached to a venue declared to
	// publish no act history. Unreachable through the constructors and refused
	// anyway, because every matching rule below would silently ignore them.
	ErrMarginHistoryAbsentWithRows = errors.New("oms: margin act history is absent yet carries rows")

	// ErrMarginHistoryStateInvalid reports an undeclared history state.
	ErrMarginHistoryStateInvalid = errors.New("oms: margin act history state is not a declared value")

	// The reconciliation's three refusals, one per field core §7.3 names.
	//
	// They are separate sentinels rather than one malformed-record error because
	// each names a DIFFERENT thing the operator failed to supply, and this is the
	// one record in the log whose author is a human at a prompt: "who" is missing
	// and "why" is missing are different conversations, and a single error would
	// make the command's diagnostic say only that the act was rejected.
	//
	// An unattributed clearance is the exact artifact §7.3's audit requirement
	// exists to prevent, so it is refused rather than defaulted to the host user
	// — a default here would satisfy the audit's shape while recording nobody.
	ErrMarginOpReconcileActorUnset = errors.New("oms: margin op reconciliation names no actor")

	// ErrMarginOpReconcileNoteUnset reports a clearance with no accounting trail.
	//
	// The note is the only field carrying WHAT was done, because the finding is
	// deliberately not a field (record §82 §2a): core §7.1 calls this state
	// Ambiguous because the attribution may be impossible, so what the record can
	// honestly hold is the operator's own account of the reconciliation. Blank, a
	// clearance is a keystroke with no subject.
	ErrMarginOpReconcileNoteUnset = errors.New("oms: margin op reconciliation carries no note")

	// ErrMarginOpReconcileInstantUnset reports a clearance with no instant. It is
	// §7.3's "when", and the zero value is refused for the reason every other
	// unset instant in this package is: it reads as 1970 rather than as absent.
	ErrMarginOpReconcileInstantUnset = errors.New("oms: margin op reconciliation carries no instant")
)
