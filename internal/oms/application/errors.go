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
	ErrJournalMissing    = errors.New("oms: dispatcher has no journal")
	ErrPlacerMissing     = errors.New("oms: dispatcher has no order placer")
	ErrReserverMissing   = errors.New("oms: executor has no capacity reserver")
	ErrHoldVectorInvalid = errors.New("oms: capacity reserver returned an invalid hold vector")

	// ErrFilterMaxAgeInvalid reports an executor whose filter map and row-age
	// bound disagree about whether filters are in play: a non-empty map with a
	// non-positive bound, or a bound declared over an empty map. The pair is
	// one policy (§69), and half of it is a wiring error whose message can
	// name the root — not a first order that refuses for a reason assembled
	// two fields apart.
	ErrFilterMaxAgeInvalid = errors.New("oms: executor filter max age and filter map disagree")

	// ErrFiltersStaleAtAdmission reports a package leg whose filters row is
	// older than the deployment's declared tolerance at the package's one
	// admission instant (§69). The boot-time check proves rows were fresh when
	// the process started; this is the same tolerance re-judged where the age
	// matters, because filters move server-side without notice and a
	// long-running process's map stays green while the venue judges the
	// current grid. The fix for tripping it is a fresh revision (arblab
	// reference fetch), never a wider bound.
	ErrFiltersStaleAtAdmission = errors.New("oms: trading filters older than the declared bound at package admission")

	// ErrFiltersFromTheFuture reports a filters row observed AFTER the
	// package's admission instant. A negative age would pass the staleness
	// comparison trivially, and it has a real producer: a backward wall-clock
	// step after boot, met by a map that was resolved before the step. Named
	// apart from staleness because the fix differs — a stale row wants a
	// fresh revision, a future row wants the clock investigated.
	ErrFiltersFromTheFuture = errors.New("oms: trading filters observed after the package admission instant")

	// ErrClockMissing reports a use case constructed with no clock.
	//
	// It is the dispatcher's because that is where §5.3's authorization anchor is
	// read, and the failure it prevents has no symptom at dispatch time: an order
	// sent without an anchor is one recovery can never resolve to
	// StateAbsentAfterFinality, which surfaces on the day an ack is lost and not
	// before. The resolver refuses for the same reason from the other end.
	ErrClockMissing = errors.New("oms: use case has no clock")

	// ErrJournalReplayMissing reports a Replayer constructed with no journal to
	// replay.
	//
	// Named separately from ErrJournalMissing, which is the WRITE half's, and the
	// separation is the ports' own: a use case holding both could append while
	// deciding what the log means. A caller branching on the two takes different
	// action — a missing write journal means no order may be planned or sent, a
	// missing read journal means this process cannot establish what it did last
	// time and must not leave reconcile-only.
	ErrJournalReplayMissing = errors.New("oms: replayer has no journal to replay")

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

	// ErrFiltersUnknown reports a plan for an instrument the planner holds no
	// trading filters for.
	//
	// This is the fail-closed reading of an ABSENT entry, and it is what makes
	// forgetting safe end to end: a root that wires no filters map at all
	// plans nothing, with an error naming the instrument, rather than planning
	// everything unchecked. The permissive reading — absent means
	// unconstrained — is exactly the defect this slice closes, reintroduced
	// one lookup higher.
	ErrFiltersUnknown = errors.New("oms: no trading filters for this leg's instrument")

	// ErrQuantityOffLotStep, ErrPriceOffTick, ErrBelowMinQty and
	// ErrBelowMinNotional are §2.4's verification half: the four ways a leg
	// can conform to this system's own rules and still be an order its venue
	// refuses.
	//
	// They are checked per leg BEFORE anything is journaled, and the harm they
	// prevent is not the rejection — it is the rejection's asymmetry. Each leg
	// of a package is judged by its own venue against its own grid (the two
	// spot steps differ from the two perp steps by two orders of magnitude,
	// profiles §5), so an off-grid package is one leg filled and one leg
	// refused: naked exposure, reached through two individually-plausible
	// numbers. A package refused HERE, whole, before the journal, is the §2.4
	// sizing failure surfacing where it is still free.
	ErrQuantityOffLotStep = errors.New("oms: leg quantity is not on the venue's lot grid")
	ErrPriceOffTick       = errors.New("oms: leg limit price is not on the venue's tick grid")
	ErrBelowMinQty        = errors.New("oms: leg quantity is below the venue's minimum")
	ErrBelowMinNotional   = errors.New("oms: leg notional at the limit price is below the venue's minimum")

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

	// ErrAuthorityMissing reports a dispatch path constructed with no
	// DispatchAuthority.
	//
	// Refused rather than defaulted to a fresh private set, because a private
	// set is the exact shape the authority replaced: a backstop scoped to one
	// path, blind to every other path's sends, whose narrowing nothing reports.
	// The caller who would discover the default is a second dispatch path
	// re-presenting a sent order over a broken projection — the one case the
	// backstop exists for (§59b).
	ErrAuthorityMissing = errors.New("oms: dispatch path has no dispatch authority")

	// ErrDispatcherMissing reports an Executor constructed with no dispatcher
	// to inherit its collaborators from. NewExecutor's doc records why the
	// dispatcher IS the parameter: the shared authority stops being a wire a
	// seam could cross.
	ErrDispatcherMissing = errors.New("oms: executor has no dispatcher to build from")

	// ErrPackageEmpty reports an Execute call with no legs.
	//
	// ErrPlanEmpty's argument one step later: a caller whose ref construction
	// silently produced nothing would get a successful execution of none of it,
	// and report a package that was sent.
	ErrPackageEmpty = errors.New("oms: package has no legs to execute")

	// ErrPackageMixedTrades and ErrPackageMixedAttempts report an Execute whose
	// legs do not belong to one planning of one trade.
	//
	// §6 step 5 advances the attempt for the WHOLE plan, so one package is one
	// (trade, attempt) pair, and the ids carry both — the check reads what the
	// caller cannot mis-state. Mixed trades would execute two packages' legs as
	// one, and mixed attempts would execute a replan beside the plan it
	// replaced, whose ids §5.3 says may still be live at the venue.
	ErrPackageMixedTrades   = errors.New("oms: package legs name more than one trade")
	ErrPackageMixedAttempts = errors.New("oms: package legs name more than one attempt")

	// ErrPackageDuplicateLeg reports the same ClientOrderID presented twice in
	// one Execute.
	//
	// With one trade and one attempt established, distinct ids imply distinct
	// leg indices — so this is also what guarantees each leg's slot dispatcher
	// is used by exactly one goroutine, which is the executor's whole
	// concurrency argument. The refusal is therefore load-bearing for memory
	// safety, not only for order semantics.
	ErrPackageDuplicateLeg = errors.New("oms: package presents the same leg twice")

	// ErrPackageQuantityMismatch reports legs whose base quantities differ —
	// §6 step 6's "identical base quantity", the delta-neutral construction.
	//
	// The comparison is Quantity.Cmp, which refuses mismatched assets, so
	// "identical" includes "in the same base asset": a package whose legs are
	// sized in different bases is refused as a mismatch, never compared
	// numerically.
	ErrPackageQuantityMismatch = errors.New("oms: package legs do not share one base quantity")

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

	// ErrResolverMissing reports a Resolver constructed with no venue read
	// surface. Named separately from ErrPlacerMissing for that sentinel's own
	// reason: it belongs to one use case's job, and the two ports are
	// deliberately not the same one — the resolver holds the read half and not
	// the write half so that §5.3's "never resubmitted" is unexpressible rather
	// than merely forbidden.
	ErrResolverMissing = errors.New("oms: resolver has no venue order resolver")

	// ErrVenueOrderIndexMissing reports a Resolver constructed without the
	// per-instrument venue-id index gate 3's rule-out reads. It is required
	// rather than defaulted to an always-empty lookup, because the two are
	// indistinguishable at every call site and only one of them is a
	// configuration: an empty index quietly reverts every Binance absence to
	// the pre-#174 permanent refusal, which is safe, silent, and exactly the
	// degraded mode a forgotten wire should not be able to select.
	ErrVenueOrderIndexMissing = errors.New("oms: resolver has no venue order index")

	// ErrVenueOrderIDMismatch reports a join that found the venue's live
	// answer and the store's persisted ack disagreeing about one order's
	// venue id. Two authorities over one identity, with no rule for which
	// wins — and the loser decides which sweep rows become this order's
	// fills — so the join refuses the order rather than picking.
	ErrVenueOrderIDMismatch = errors.New("oms: venue reports a different venue order id than the store persisted")

	// ErrResolutionPolicyUnset reports a resolution policy with a non-positive
	// duration in it.
	//
	// Zero is the reachable value and the unsafe one in both fields it can
	// appear in: a zero MaxLanding says a send stops being able to land the
	// instant it is authorized, which is the assumption SM §2 step 1 exists to
	// forbid, and a zero SweepMargin makes SM §2 step 3's "STRICTLY later than"
	// an equality. Both read as a working configuration and both remove a gate.
	ErrResolutionPolicyUnset = errors.New("oms: resolution policy duration is zero or negative")

	// ErrOrderNotResolvable reports a resolution attempted on an order the
	// domain would not accept proved absence for — StatePlanned, or any
	// terminal. It wraps the domain's own transition error rather than
	// restating the set, so the two cannot drift.
	ErrOrderNotResolvable = errors.New("oms: order is not in a state the stable-not-found rule resolves")

	// ErrResolutionOutcomeInvalid and ErrUnprovedReasonInvalid report a
	// Resolution whose enum fields were never assigned — the zero value of a
	// string type reaching a caller that branches on it.
	ErrResolutionOutcomeInvalid = errors.New("oms: resolution outcome is empty or undeclared")
	ErrUnprovedReasonInvalid    = errors.New("oms: unproved reason is empty or undeclared")

	// ErrAttributionUnhandled reports an attribution verdict the absent path has
	// no case for. It is this package's ErrEffectUnhandled: reachable only if a
	// verdict is added without a case, and present so that the switch's final
	// arm is distinguishable by a test from the arms above it rather than
	// returning what one of them already returns.
	ErrAttributionUnhandled = errors.New("oms: sweep attribution verdict has no case")

	// ErrHedgeGroupOrdersMissing reports a classifier constructed with no order
	// source.
	ErrHedgeGroupOrdersMissing = errors.New("oms: classifier has no hedge-group order source")

	// ErrHedgeGroupNotTracked and ErrHedgeGroupAlreadyTracked report a hedge
	// group the classifier does not hold and one it already holds.
	//
	// The first refuses rather than tracking on demand, and the direction is why:
	// a classifier that created a group when first asked would answer an unknown
	// group with "Determined, residual zero, no reservations" — the one verdict
	// that unblocks a compensating trade — for a group whose orders and
	// reservations it has simply never seen. A typo in a group key would then
	// read as a flat package.
	//
	// The second refuses rather than replacing, on domain.Ledger.Register's
	// argument: replacing discards the group's outstanding reservations while
	// their orders stay live at the venue, which is FM-5's harm arriving by a
	// different route — a subtrahend that stops being counted rather than one
	// counted too early.
	// ErrHedgeGroupAssetUnset reports a hedge group tracked with no base asset.
	ErrHedgeGroupAssetUnset = errors.New("oms: hedge group has no base asset")

	ErrHedgeGroupNotTracked     = errors.New("oms: hedge group is not tracked by this classifier")
	ErrHedgeGroupAlreadyTracked = errors.New("oms: hedge group is already tracked by this classifier")

	// ErrCompensationBeforeFinality reports a reduction sized from a verdict that
	// does not permit one — §6.7's central rule, "no path compensates the
	// uncertain part of an ambiguous residual before the finality barrier".
	ErrCompensationBeforeFinality = errors.New("oms: residual is not determined, so no compensating trade may be sized from it")

	// ErrFloorOutsideDeadlineBranch reports an interval floor asked for outside
	// the deadline-exceeded branch.
	//
	// §6.7 narrows the floor twice and this is the narrowing a reader loses
	// first: it is "the SOLE exception to 'no compensation before finality'",
	// AND it "applies only in the deadline-exceeded branch (normal operation
	// waits for finality and compensates the exact residual)". A merely-ambiguous
	// group has a guaranteed floor too, and it is deliberately not compensable —
	// which is also where formal model (a) is stricter than §6.7's general
	// sentence, on purpose.
	ErrFloorOutsideDeadlineBranch = errors.New("oms: the interval floor applies only in the deadline-exceeded branch")

	// ErrFloorAlreadyIssued reports a second interval floor in one
	// deadline-exceeded episode — §6.7's "never issued twice".
	//
	// It is defence in depth rather than the primary mechanism: reservation-aware
	// sizing already prevents a second floor reaching the opposite sign, checked
	// over model (a)'s complete state graph. This is the rule that survives a
	// defect in that mechanism, so its refusal is what a caller sees when the
	// other one has already failed silently.
	ErrFloorAlreadyIssued = errors.New("oms: the interval floor has already been issued for this episode")

	// ErrFloorMagnitudeZero reports an interval whose nearest-zero endpoint is
	// zero, so no nonzero compensation is invariant across resolutions.
	//
	// Distinct from a straddling interval, which domain refuses with its own
	// sentinel: there the range spans zero and nothing is safe, here the range
	// touches zero and doing nothing is exactly right. Both refuse, and merging
	// them would report "no safe action exists" for a group that is provably fine.
	ErrFloorMagnitudeZero = errors.New("oms: the interval's nearest-zero endpoint is zero, so no floor is issuable")

	// ErrVenuePositionsMissing reports a joiner constructed with no position
	// source. It is a separate sentinel from ErrResolverMissing because the two
	// ports answer different questions and only one of them can disagree with
	// us: without the enumeration the join still rebuilds every quantity and
	// asserts nothing, which is the failure that looks like a clean boot.
	ErrVenuePositionsMissing = errors.New("oms: joiner has no venue position source")

	// ErrJoinAccountUnset reports a joiner with no subaccount to attribute fills
	// to. §5.1 puts the account first in the fill dedupe key, and an empty one
	// merges two subaccounts that share a venue's per-instrument execution-ID
	// sequence — so the second subaccount's fills are dropped as duplicates.
	ErrJoinAccountUnset = errors.New("oms: joiner has no account to attribute fills to")

	// ErrUnassertedReasonInvalid reports an UnassertedReason that is the zero
	// value or undeclared.
	//
	// It exists because the reason decides whether an instrument outside the
	// net-delta invariant holds the boot posture, and an unclassifiable one has to
	// reach a caller as an error rather than as the bool alone — see
	// UnassertedReason.HoldsPosture, whose safe direction and whose reportable
	// direction are the same only if this sentinel travels with it.
	ErrUnassertedReasonInvalid = errors.New("oms: unasserted reason is empty or undeclared")

	// ErrOrderNotAtVenue reports a join asked about an order whose state means no
	// venue order can exist — StatePlanned, and only that.
	//
	// Refused rather than skipped: the caller derives its list from the same
	// domain predicate this checks, so an order that fails it means the two
	// readings have drifted, and the cost of guessing is a resolving query that
	// runs to its deadline over an order this system has merely not sent.
	ErrOrderNotAtVenue = errors.New("oms: order's state means no venue order can exist")

	// ErrReplayerMissing, ErrJoinerMissing and ErrResolveUseCaseMissing report a
	// reconciler constructed without one of §7.5's three phases.
	//
	// Three sentinels rather than one, because a root that supplied two of three
	// gets told which one it forgot — and because the phases are not
	// interchangeable: without the replay there is no dispatched set to join,
	// without the join no absence is ever found, and without the resolver the
	// absences that ARE found go to nobody, which is the one of the three that
	// still produces a plausible report.
	ErrReplayerMissing       = errors.New("oms: reconciler has no journal replayer")
	ErrJoinerMissing         = errors.New("oms: reconciler has no venue joiner")
	ErrResolveUseCaseMissing = errors.New("oms: reconciler has no stable-not-found resolver")

	// ErrChecklistMissing reports a use case called with no checklist to report
	// into — Reconcile's, and since §6 step 7 gained its live producer,
	// Execute's too.
	//
	// It is an error rather than a silent run because the checklist is the only
	// place the verdict goes: a reconcile that ran every phase and reported
	// nowhere leaves a composition root reading a posture that was never
	// constrained, which is the "nothing was checked and everything passed"
	// collapse §7.5 argues about the fold's seed. For Execute the stakes are the
	// same fact one moment later: a package sent with nowhere to file §6 step
	// 7's hold is a stale projection nothing will ever refuse an opening over.
	ErrChecklistMissing = errors.New("oms: reconcile has no boot checklist to report into")

	// ErrExecutionHeld reports a package refused because the checklist's
	// concluded checks do not permit its class — §6 step 7's "holds new
	// trading", read back at the write path.
	//
	// The read is boot.Checklist.PermitsNow over the urgency's act: an Opening
	// package needs ActOpen, a Safety package needs ActReduce, so a latched
	// OutcomeProjectionStale refuses the first and admits the second — the
	// safety verbs stay available while new trading is held, which is the
	// asymmetry §6 step 7 is FOR. It is not the composition root's boot verdict
	// re-checked: that gate runs once per run, and this one runs per package,
	// which is what makes a hold filed by the PREVIOUS package's fan-in bind
	// the next one in the same process.
	ErrExecutionHeld = errors.New("oms: the checklist's concluded checks do not permit this package's class of execution")

	// ErrHoldGateUndeclared reports an Execute over a checklist that never
	// declared GateNetDeltaAsserted — the gate §6 step 7's hold is filed
	// against.
	//
	// Refused in the preflight, with nothing sent, because the failure it
	// prevents is discovered at the fan-in: a stale leg whose report is refused
	// AFTER the sends is a hold with live orders behind it and nowhere to go.
	// Every root that declares §7.5's roster in full passes this for free; what
	// it refuses is a caller that built a bespoke short checklist, which is the
	// shape §7.5 argues against declaring at all.
	ErrHoldGateUndeclared = errors.New("oms: checklist does not declare the net-delta gate §6 step 7 files its hold against")

	// The spot-leg constructor refusals (task #179, record §70). Each names the
	// field whose zero value or contradiction it refuses; see
	// application.validateSpotLeg for the one cross-field rule.

	// ErrSpotRosterNotSpot reports a roster row naming a derivative instrument.
	// The roster is the spot assertion's coverage set — a perp in it would be
	// answered by the balance comparison, which reads a wallet its exposure
	// never touches (linear perps settle in the quote), so it would assert
	// "flat" about a position the position enumeration exists to see.
	ErrSpotRosterNotSpot = errors.New("oms: spot roster names a derivative instrument")

	// ErrSpotRosterBaseUnset reports a roster row with no base asset — the key
	// the whole comparison is grouped by, unusable as the zero value.
	ErrSpotRosterBaseUnset = errors.New("oms: spot roster row names no base asset")

	// ErrSpotRosterQuoteUnset reports a roster row with no quote asset — the
	// field the quote-overlap deny below reads, unusable as the zero value.
	ErrSpotRosterQuoteUnset = errors.New("oms: spot roster row names no quote asset")

	// ErrSpotBaselineIsQuote reports a baseline for an asset that is a
	// configured spot pair's QUOTE: that pair spends the asset as
	// consideration, which `ours` (base flows + same-asset fees) cannot see,
	// so every journaled trade on it would latch a false delta. Record §70c
	// rejects computing quote flows in favour of this deny.
	ErrSpotBaselineIsQuote = errors.New("oms: spot baseline names an asset a configured spot pair spends as its quote")

	// ErrSpotRosterDuplicate reports the same instrument twice in the roster:
	// both resolutions of a repeat are recoveries that look like they worked
	// (domain.Ledger.Register's argument, at the config seam).
	ErrSpotRosterDuplicate = errors.New("oms: spot roster names an instrument twice")

	// ErrSpotBaselineAssetUnset reports a baseline row with no asset.
	ErrSpotBaselineAssetUnset = errors.New("oms: spot baseline names no asset")

	// ErrSpotBaselineAssetMismatch reports a baseline whose Amount carries a
	// different asset tag than the row names — two copies of one fact
	// disagreeing, which is the state the tag exists to make loud.
	ErrSpotBaselineAssetMismatch = errors.New("oms: spot baseline amount is tagged with a different asset")

	// ErrSpotBaselineDuplicate reports two baselines for one (venue, asset).
	ErrSpotBaselineDuplicate = errors.New("oms: spot baseline recorded twice for one venue and asset")

	// ErrSpotBaselineUnconsumed reports a baseline for an asset that is not the
	// base of any configured spot instrument — a row nothing consumes is a
	// claim nobody checks, and the safe reading of one is that the config and
	// the document disagree about what this deployment trades.
	ErrSpotBaselineUnconsumed = errors.New("oms: spot baseline names an asset no configured spot instrument trades")

	// ErrSpotBalancesMissing reports a baseline with no balance surface to
	// compare it against — an assertion that could never run, refused where the
	// wiring is the subject.
	ErrSpotBalancesMissing = errors.New("oms: spot baselines are recorded but no balance surface is wired")

	// ErrBalanceRowDuplicate reports a balance enumeration stating one asset
	// twice — an adapter contract violation that would otherwise make the
	// asserted total a declaration-order coin flip.
	ErrBalanceRowDuplicate = errors.New("oms: balance report states an asset twice")

	// ErrSecondLookHalfWired reports a SecondLook with a Wait and no positive
	// Settle, or the reverse — either half alone reads as configured while the
	// bracket silently never runs.
	ErrSecondLookHalfWired = errors.New("oms: spot second look needs both a wait seam and a positive settle bound")

	// ErrSecondLookUnwired reports a spot delta meeting a Reconciler whose
	// second look was never configured. Failing by name is the point: one look
	// must not mint a permanent operator incident, and silently latching would
	// be exactly that (record §70c).
	ErrSecondLookUnwired = errors.New("oms: a spot delta needs the second look and none is wired")

	// ErrPremiseNettingAmbiguous reports a premise asked for on a group holding
	// more than one PackageDelever reservation. The netting identifies "this
	// generation's own entry" by its PRODUCER, so with two live generations it
	// would remove a sibling's entry as well — and the premise would then
	// contain another generation's subject, which is FM-9's own shape one layer
	// out. SM §7 forbids the situation; this refuses to compute through it.
	ErrPremiseNettingAmbiguous = errors.New("oms: hedge group holds more than one PackageDelever reservation")

	// ErrPremiseLegUnidentified reports an empty client order id in a
	// generation's own-leg set. An empty id matches no record, so the leg would
	// silently stay INSIDE the quantity that decides whether the generation may
	// continue.
	ErrPremiseLegUnidentified = errors.New("oms: generation own-leg set contains an empty client order id")

	// ErrClassifierMissing and ErrGenerationJournalMissing report a Deleverer
	// wired without §6.7's gate or without the durable generation log.
	//
	// Refused at construction on NewDispatcher's argument: the call that would
	// discover a nil generation journal is the one recording FLAG-3's
	// authorization, and finding the log absent THERE means the crash-boundary
	// record silently did not exist for a spot sale already being authorized. A
	// nil classifier is worse still — the ledger every other §6.7 client sizes
	// against would simply not be amended, which is FM-5 with the bookkeeping
	// done correctly.
	ErrClassifierMissing        = errors.New("oms: delever needs the §6.7 classifier")
	ErrGenerationJournalMissing = errors.New("oms: delever needs the generation journal")

	// ErrNoLiveGeneration reports a ladder step for a hedge group that has no
	// generation. The steps are not idempotent — each one journals — so this is
	// a refusal rather than a no-op.
	ErrNoLiveGeneration = errors.New("oms: hedge group has no live PackageDelever generation")

	// ErrGenerationAlreadyLive reports a second generation opened on a group
	// whose previous one has not ended — SM §7's "each generation is a bounded
	// attempt made only after the prior generation reaches §6.7 finality".
	//
	// It is the IN-PROCESS half of that gate. The other half is
	// ErrPremiseNettingAmbiguous, which catches a sibling this process never
	// created, and neither subsumes the other: this one names the generation and
	// its phase, which is what an operator needs, and that one survives a
	// restart, which is what correctness needs.
	ErrGenerationAlreadyLive = errors.New("oms: hedge group already has a live PackageDelever generation")

	// ErrGenerationAlreadyOpened reports a REPLAY finding two opening records
	// for one generation id.
	//
	// It is not ErrGenerationAlreadyLive arriving at boot, and collapsing the two
	// would lose the distinction an operator acts on. That one is SM §7's entry
	// gate refusing a second generation on a GROUP — a rule about the live set,
	// which is exactly what a bounded attempt means. This one is a claim about
	// the LOG: one identity, brought into existence twice, in an append-only
	// store where the second record cannot be a correction of the first. Nothing
	// this system can do produces it, so the realistic cause is two processes
	// having written to one journal — the condition the single connection and
	// BEGIN IMMEDIATE exist to prevent, evidenced after the fact.
	ErrGenerationAlreadyOpened = errors.New("oms: journal opens one generation twice")

	// ErrGenerationNotOpened reports a replayed generation record — a leg, an
	// authorization, a supersession, a phase — naming a generation no opening
	// record created.
	//
	// The generation-scope reading of "the log verifies and does not compose",
	// and the realistic cause is head truncation rather than corruption: §7.5
	// records that a chain "proves nothing about records that were never seen".
	// Refused rather than skipped, because the permissive reading is silently
	// wrong in one direction — a replay that carried on would report ZERO live
	// generations for a package that has one, and the reserved-reduction entry
	// the missing opening record names would be re-Reserved by nothing.
	ErrGenerationNotOpened = errors.New("oms: journal record names a generation no opening record created")

	// ErrGenerationLegWrongProduct reports a leg dispatched as the wrong half of
	// the cash-and-carry — a spot order presented as the perp reduction, or the
	// reverse.
	//
	// It is §7's perp-short-FIRST sequencing made checkable at the only place
	// both facts are in hand. A spot order submitted as the perp leg sells spot
	// before any perp reduction has confirmed, which drives the transient net
	// SHORT in the rally the generation was opened for — the one direction the
	// whole ordering exists to avoid — and nothing downstream objects, because
	// the quantity is right and the group is right.
	ErrGenerationLegWrongProduct = errors.New("oms: generation leg is on the wrong side of the package")

	// ErrGenerationLegMissized reports a leg whose order quantity is not the
	// size the generation requires — its target for the perp leg, its durable
	// authorization for the spot one.
	//
	// EXACTLY, not "no more than", and the direction that looks safe is the one
	// that is not: the ledger entry is amended from the generation's figure, so
	// an order SMALLER than it overstates outstanding_reserved_reductions. §6.7's
	// residual is `realized − reserved`, so an overstated subtrahend reads more
	// SHORT than the package is, and the determined branch BUYS — adding exposure
	// in the middle of reducing it.
	ErrGenerationLegMissized = errors.New("oms: generation leg is not the size the generation requires")

	// ErrPremiseLossNotExternal reports the premise-lost supersession offered to
	// Deleverer.Supersede, which serves SM §7's two EXTERNAL rows.
	//
	// The third row is this machine's own standing obligation, discharged by
	// Reassess from the netted quantity that decides it (FM-9). Admitting it as
	// an argument would let a caller assert a premise loss the ledger does not
	// show — a generation cancelled on nobody's evidence, at the moment its perp
	// short is nearing liquidation.
	ErrPremiseLossNotExternal = errors.New("oms: the premise-lost supersession is discharged, never asserted")

	// ErrGenerationMissing reports Deleverer.Restore handed a nil aggregate.
	//
	// A refusal rather than a no-op, because the caller is §7.5's reinstatement
	// and its whole contract is that a generation the log left live ends up in
	// the ladder WITH its ledger entry. A nil silently ignored is a boot that
	// reported the reinstatement done over a generation nobody reinstated — and
	// the ledger is then short by exactly the entry that keeps §6.7's determined
	// branch honest.
	ErrGenerationMissing = errors.New("oms: restore needs a generation")

	// ErrGenerationLadderBarred reports core §7.1's escalation ladder refused on
	// a hedge group whose generation ended on a path that does not resume it —
	// SM §7's "a supersession ends the GENERATION; whether it ends the LADDER is
	// a property of the PATH".
	//
	// The two paths that set it are `EmergencyClosePackage` and the
	// `FenceReadyLost` arbitration: in both, the foundation has taken the package
	// away from the leverage-reduction ladder, and a generation opening
	// afterwards is this machine re-authorising itself against a decision that
	// just fired. It carries the PATH in its message, because which arbitration
	// closed the ladder is the operator's first question and there is no other
	// record of it once the generation is gone.
	//
	// It is also what Deleverer.Restore reports for a log holding a live
	// generation on a barred group, which is a different claim in the same
	// sentinel: that is a history this build cannot produce, and the shared
	// sentinel is right because the operator's action is the same — the package
	// is closed to the ladder, and something believes otherwise.
	ErrGenerationLadderBarred = errors.New("oms: hedge group's PackageDelever ladder was closed by a supersession")

	// ErrGenerationLadderResumes reports Deleverer.BarLadder offered a path core
	// §7.1's ladder DOES resume from — the premise-lost row.
	//
	// A refusal rather than a silent no-op, and the direction is why: barring the
	// ladder on a premise loss would disable the one mechanism that reduces
	// leverage on a neutral package for the rest of the process's life, at exactly
	// the moment it is needed. SupersessionPath.ResumesLadder calls that the
	// unsafe direction outright, so a caller reaching here has confused the two
	// classes and must be told rather than obeyed.
	ErrGenerationLadderResumes = errors.New("oms: this supersession path resumes the ladder and cannot bar it")

	// ErrGenerationRestoreMissing reports a Reconciler wired with no
	// reinstatement over a journal that holds a live generation.
	//
	// It is ErrSecondLookUnwired's shape one gate over, and for the identical
	// reason: a deployment that never delevers legitimately has nothing to
	// reinstate, so the phase is optional — but an unwired phase meeting the one
	// condition it exists for must FAIL BY NAME rather than pass. Passing would
	// report §7.5's reinstatement gate satisfied on a boot that left a generation's
	// entry out of the shared ledger, which is the FM-5 hole this whole gate is.
	ErrGenerationRestoreMissing = errors.New("oms: journal holds a live generation and no reinstatement is wired")

	// ErrGenerationStepUnreachable reports a recorded generation phase the
	// reinstatement cannot replay.
	//
	// Two disjoint causes, deliberately one sentinel because both mean the same
	// thing about the log — a step sequence no live process could have written:
	// `Planned`, which is the seed rather than a step and can only appear as a
	// journal record if a phase record restated the opening; and the three ENDED
	// phases, which cannot appear in a LIVE generation's step list at all, since
	// the walk that produced it puts an ended generation in neither list.
	ErrGenerationStepUnreachable = errors.New("oms: generation step cannot be replayed at boot")

	// ErrGenerationLadderMissing reports a Reinstater constructed without the
	// ladder seam it exists to reach.
	//
	// Refused at construction rather than tolerated as a report-only mode, on
	// ErrClassifierMissing's argument: the call that would discover it is the one
	// re-entering a generation's reservation into the shared ledger, and a
	// reinstatement that reported success having reached no ladder is a boot that
	// cleared its gate over a ledger it never touched.
	ErrGenerationLadderMissing = errors.New("oms: reinstatement needs the PackageDelever ladder")
)
