package ports

import "errors"

// The refusals a driven port makes, declared beside the interfaces rather than
// in the adapters that return them.
//
// # This placement is forced, and the force is §3.6 rule 1
//
// A sentinel is only useful if a caller can branch on it, and the callers here
// are use cases in application/. Declaring ErrVersionConflict in
// infrastructure/driven would mean every caller that wanted to retry an OCC
// conflict had to import an adapter package — which hexboundary refuses — or
// match on the message text, which is the same coupling with nothing checking
// it.
//
// So the error is part of the interface, in the literal sense: an
// implementation that reports a version conflict some other way does not
// satisfy this port, whatever its method set says. That is the same claim
// Journal's doc comment makes about durability, and it has the same consequence
// — the compile-time assertion §3.6 rule 2 requires checks the signature, and
// these two sentences are the part it cannot.
var (
	// ErrOrderUnknown reports a ClientOrderID the repository holds no record
	// for.
	//
	// Distinct from domain.ErrFillUnknownOrder, which the fill ledger raises for
	// the same underlying condition on the fill path. The two are not merged
	// because they answer different questions: this one says the WRITE
	// repository never created the order — a caller that skipped Create, or a
	// process that has not replayed the journal yet (§7.5) — while the domain
	// sentinel is about an execution naming an order the ledger does not hold.
	// A reader chasing the first goes to the write path; a reader chasing the
	// second goes to the venue's execution stream.
	ErrOrderUnknown = errors.New("oms: write repository holds no record for this client order id")

	// ErrVersionConflict reports a write whose expected version is not the
	// record's current one — §5.1's OCC Version, refusing a lost update.
	//
	// # Why the check exists at all when §3.6 rule 6 already makes oms the only
	// # writer
	//
	// Rule 6 is about which CONTEXT may write; it says nothing about how many
	// goroutines inside this one may. The concurrency §6 actually has is real:
	// the private order stream, the executions stream, the REST recovery sweep
	// and the §6.7 finality timers all produce writes for the same order, and
	// §5.3 puts fills and lifecycle events on deliberately independent paths.
	//
	// The failure this refuses is the classic read-modify-write loss, and its
	// shape here is what makes it worth a sentinel rather than a mutex alone: two
	// writers read an order at Acknowledged, one advances it to Canceled and the
	// other to PartiallyFilled, and both transitions are LEGAL under
	// domain.CanTransition. Nothing detects the collision from the state set;
	// only the version does. What is lost is not a field but an event — the
	// fill's, or the cancel's — and §6.7 then classifies a hedge group from a
	// record that never saw one of them.
	//
	// It is a caller-recoverable error, and the only correct recovery is to
	// re-read and re-decide: a retry with the same intent against the new
	// version is right for a fill, and wrong for a cancel that raced a fill,
	// which is why this port does not retry for anyone.
	ErrVersionConflict = errors.New("oms: write repository record has moved since it was read")

	// ErrAuthorizationNeedsAnchor reports an attempt to reach
	// StateDispatchAuthorized without the instant §5.3's stable-not-found rule
	// measures its two bounds from — either through ApplyState, which does not
	// carry one, or through ApplyAuthorized with an unset clock.ServerTime.
	//
	// It is one sentinel for both, and the joining is deliberate: a caller
	// branching on it takes the same action either way — produce a calibrated
	// venue instant and authorize again — and nothing was written in either case.
	// What it protects is stated on OrderRecord.AuthorizedAt: an order at the
	// crash boundary with no anchor cannot be resolved to StateAbsentAfterFinality
	// by any evidence, so it holds its hedge group ambiguous for ever while every
	// component reports healthy.
	ErrAuthorizationNeedsAnchor = errors.New("oms: an order may not reach the dispatch boundary without a venue-frame authorization instant")

	// ErrVenueOrderIDConflict reports an ApplyAck whose id contradicts the
	// store — this order already holds a different venue id, or another order
	// on the same instrument already holds this one.
	//
	// # Either way the venue and this store disagree about an identity, and
	// # that is held, not absorbed
	//
	// Under the banked venue fact (orderId self-increments per symbol, never
	// reused) neither contradiction has an innocent reading: one of the two
	// acks, or the store, is wrong, and every attribution premise the
	// resolver's rule-out rests on is wrong with it. So the refusal must
	// SURVIVE to wherever a conclusion could be drawn, and it does, on both
	// paths: live, Dispatch reports it as Outcome.AckConflict, which
	// HoldsNewTrading folds in and the executor's fan-in latches on; after a
	// crash, the journal holds both acked records and the replay's ApplyAck
	// refuses the second, which fails the boot — a log that verifies and does
	// not compose. A resolver therefore never reads an index this sentinel
	// fired about, which is the property the whole arm's safety rests on.
	ErrVenueOrderIDConflict = errors.New("oms: venue order id contradicts what the store holds for this instrument")

	// ErrJournalChainBroken reports a replay whose hash chain does not verify: a
	// record whose recomputed digest is not the one stored, a prev_hash that does
	// not name its predecessor, or a sequence with a hole in it.
	//
	// # It is HERE rather than in the adapter, and slice A predicted the move
	//
	// driven/journalerrors.go keeps the journal's own refusals beside the
	// adapter, on the stated test that no use case branches on them — and it
	// named the exception in advance: "if a later slice does need to branch —
	// §7.5's reconcile-only boot is the candidate, since a broken chain there is
	// an operator-clearance matter rather than a refusal — the sentinel moves to
	// ports/ WITH the port method that reports it." This is that slice, this is
	// that method, and the branch is real: §7.5's replayer reports a broken chain
	// as boot.OutcomeEvidenceUnusable and every other replay failure as a plain
	// refusal, which are different postures and different clearance rules.
	//
	// The prediction's REASONING needed re-deriving rather than inheriting,
	// though, and it came out one rung stricter than the sentence above: an
	// operator-clearance matter in §7.5's vocabulary is PostureReconcileOnly,
	// which permits the reconciliation writes this log is the input to. A chain
	// that does not verify halts as well as latching. Recorded because "already
	// decided, see the note" is exactly the premise nobody re-checks.
	ErrJournalChainBroken = errors.New("oms: journal hash chain does not verify")

	// ErrUrgencyInvalid reports a §7.6 shed class that is empty or outside the
	// two an order can carry. Empty is the reachable one and is the reason this
	// is checked at all: Urgency is a string, so a caller that never set it
	// passes the compiler, and the zero value must not be readable as either
	// class — one would shed a flatten and the other would spend the reserve.
	ErrUrgencyInvalid = errors.New("oms: order urgency is empty or not a declared shed class")

	// ErrVenuePresenceInvalid reports a resolution whose presence is empty or
	// undeclared. Empty is the reachable one: an adapter that returned a
	// zero-valued VenueOrder would otherwise present "we did not fill this in"
	// as a reading of the venue's answer, and the two readings it sits between
	// are "the order exists" and "prove it absent".
	ErrVenuePresenceInvalid = errors.New("oms: venue presence is empty or not a declared reading")

	// ErrSurfaceUnnamed reports a reach that does not say which surface produced
	// it. It is the one field of SurfaceReach with no safe default, because
	// every refusal the resolver writes about an absence names the surface that
	// answered, and an unnamed one sends an operator to look at both.
	ErrSurfaceUnnamed = errors.New("oms: surface reach does not name the surface that answered")

	// ErrSurfaceReachUnset reports a reach claiming a published retention while
	// carrying a zero duration for it — the zero-is-unusable rule at the one
	// place where a zero would read as "this surface remembers nothing", which
	// is the opposite of the unbounded promise it would actually license.
	ErrSurfaceReachUnset = errors.New("oms: surface reach claims a published retention but states none")

	// ErrSurfaceReachContradictory reports a reach that is unpublished and
	// carries durations anyway. It is refused rather than ignored because the
	// two fields would then disagree about whether the venue states a horizon,
	// and the resolver's gate 4 branches on exactly that.
	ErrSurfaceReachContradictory = errors.New("oms: surface reach states durations for an unpublished retention")

	// ErrPositionScopeSettleUnset reports a position scope with no settlement
	// asset.
	//
	// The zero-is-unusable rule at a seam where the zero value is ACCEPTED by one
	// of the two venues: Binance enumerates the whole family and would answer an
	// unset settlement asset perfectly well, so a caller that forgot it would see
	// the invariant assert cleanly on one venue and fail on the other, which is
	// the failure that gets diagnosed as a Bybit problem.
	ErrPositionScopeSettleUnset = errors.New("oms: position scope names no settlement asset")

	// ErrPositionScopeSurfaceAbsent reports a scope naming a product family no
	// venue publishes a position surface for — spot, and only spot.
	//
	// It is the mirror of engine/venue's own refusal, which is where the fact was
	// stated when only that layer could express it. What an instrument outside
	// every enumeration costs is a separate matter and has its own vocabulary
	// (application.UnassertedReason); this is about a scope that could never be
	// SERVED, which is a different mistake in a different field.
	ErrPositionScopeSurfaceAbsent = errors.New("oms: position scope names a family with no position surface")

	// ErrUnpinnedReasonInvalid reports an UnpinnedBalance whose reason is the
	// zero value or out of roster — an asset outside the assertion with no
	// account of why, which is the state the type exists to make unwritable
	// (UnassertedReason's argument, one port over).
	ErrUnpinnedReasonInvalid = errors.New("oms: unpinned balance names no valid reason")

	// ErrAbsenceSemanticsInvalid reports a BalanceReport that never stated
	// what its silences mean — the zero value or out of roster. Every
	// consumer refuses it: a report whose absences could be read either way
	// is a policy with no carrier, and inheriting a reading is the mistake
	// the field exists to make unwritable (§70i).
	ErrAbsenceSemanticsInvalid = errors.New("oms: balance report states no absence semantics")

	// ErrGenerationPhaseNeedsPayload reports a generation phase recorded through
	// GenerationJournal.AppendGenerationPhase when that phase carries a payload
	// of its own — `Planned` its target, `SpotReduceAuthorized` its size,
	// `Superseded` its path.
	//
	// It is one sentinel for all three, on ErrAuthorizationNeedsAnchor's
	// argument: the caller's action is identical in each case — call the method
	// that carries the field — and nothing was written in any of them. What it
	// protects differs per phase and is stated at each method; what they share is
	// that the payload is UNRECOVERABLE once the phase is recorded without it,
	// because none of the three is derivable from the order journal.
	ErrGenerationPhaseNeedsPayload = errors.New("oms: this generation phase may not be recorded without its payload")

	// ErrMarginReadingSurfaceAbsent reports a MarginReading on a spot
	// instrument. Neither venue publishes a position surface for spot, so no
	// adapter could have produced one — it is a fixture or a caller that built
	// the reading directly (task #208).
	ErrMarginReadingSurfaceAbsent = errors.New("oms: margin reading names an instrument with no position surface")

	// ErrMarginReadingAssetsDisagree reports a reading whose figures are not all
	// in one asset — the two money fields differing, or the liq price quoted in
	// something other than the margin asset. On a linear perpetual they are the
	// same asset, and that premise is what lets §7.1 compare them at all.
	ErrMarginReadingAssetsDisagree = errors.New("oms: margin reading figures are not all in one asset")

	// ErrMarginReadingLiqPriceNotPositive reports a stated liq price at or below
	// zero. §7.1 takes a distance between it and the mark, and a SHORT's
	// liquidation below its mark is a distance every formula reads as maximum
	// headroom.
	ErrMarginReadingLiqPriceNotPositive = errors.New("oms: margin reading liq price is not positive")

	// ErrMarginReadingLiqPriceContradicts reports a liq price beside a cleared
	// LiqPriceStated. The flag is the half consumers branch on, so the number
	// would sit unread until something copied the struct field by field.
	ErrMarginReadingLiqPriceContradicts = errors.New("oms: margin reading states no liq price but carries one")

	// ErrMarginReadingQuantityZero reports a reading that claims a held position
	// and carries no position quantity (task #219).
	//
	// The adapters cannot produce one — a zero quantity is what their presence
	// test reads as flat — so this exists for the DOUBLES. §7.1's two remedies
	// both multiply by it: a zero sizes a top-up of zero and a delever toward
	// nothing, so a use-case test built on a default-valued reading would show
	// the ladder running its remedies and moving the position not at all.
	ErrMarginReadingQuantityZero = errors.New("oms: margin reading carries no position quantity")

	// ErrMarginReadingEntryPriceNotPositive reports an entry price at or below
	// zero (task #219).
	//
	// Refused rather than flagged, unlike the liq price two entries up, because
	// neither venue documents an absence spelling for a live position's entry
	// and §85 §3's delever target has no branch that could take one: absorbed as
	// zero it enlarges the denominator `(1+MMR)·L* − e` and delevers harder than
	// asked.
	ErrMarginReadingEntryPriceNotPositive = errors.New("oms: margin reading entry price is not positive")

	// ErrMarginOpStatusNotConcluded reports an attempt to journal a `pending`
	// margin-op resolution.
	//
	// A pending is this system declining to answer because too little time has
	// passed for the venue's silence to be evidence. It is a momentary condition,
	// and an append-only log turns a momentary condition into a permanent record
	// every later reader must know to skip. Refused at the write end, which is
	// the last place a refusal costs nothing.
	ErrMarginOpStatusNotConcluded = errors.New("oms: margin op resolution is not a concluded verdict")
)
