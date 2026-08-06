package venue

import "errors"

// The venue seam's refusals. Sentinels for branching, wrapped at the site with
// the values that failed — the same rule the contexts follow.
var (
	// ErrClampInverted reports a cap below its floor. The pair comes from the
	// venue as two independent fields (Profiles §1: Binance overlays them via
	// fundingInfo, Bybit sends fundingCap on the ticker), so an inverted pair is
	// a normalization mistake rather than a venue state, and §2.4 clamps a
	// forecast's later settlements to them — an inverted clamp there produces an
	// empty admissible range, not a loud failure.
	ErrClampInverted = errors.New("venue: funding cap is below its floor")

	// ErrIntervalInvalid reports a non-positive funding interval.
	//
	// This is the refusal that makes Profiles §1's "absent field = unchanged,
	// never null-coerce" mechanical. Bybit's ticker sends fundingIntervalHour
	// only when it changes, so a delta decoded into a zero-valued struct field
	// carries an interval of zero — and a zero interval is not a slow feed, it
	// is a division in domain.Enumerate and a settlement count of infinity.
	ErrIntervalInvalid = errors.New("venue: funding interval is not positive")

	// ErrIntervalRange reports an interval-in-hours outside the band any venue
	// actually runs. See FundingIntervalFromHours: the band exists to catch a
	// UNIT error, not to police venue policy.
	ErrIntervalRange = errors.New("venue: funding interval hours out of range")

	// ErrNextSettleUnset reports a funding reading with no settlement instant.
	// §2.4's discrete-settlement rule enumerates from it, and an unset one makes
	// every settlement fall in year one — so a hold captures nothing and the
	// carry reads zero, which is indistinguishable from a flat regime.
	ErrNextSettleUnset = errors.New("venue: funding next-settlement instant unset")

	// ErrSideUnset and ErrSideUnknown report a book side that is empty or
	// outside the declared pair.
	//
	// Two sentinels rather than one, because they come from different mistakes
	// and only one of them is about the venue. An empty side is a decode that
	// never populated the field — a struct built from JSON that used a different
	// key — while an unknown one is a venue vocabulary this seam has not been
	// taught ("buy"/"sell", "b"/"a"). The first sends a reader to the decoder,
	// the second to the Profiles entry that should have recorded the spelling.
	ErrSideUnset   = errors.New("venue: book side unset")
	ErrSideUnknown = errors.New("venue: book side is not bid or ask")

	// ErrLevelQuantityNotPositive reports a snapshot level resting zero or
	// negative size.
	//
	// A snapshot states what IS on the book, so zero is not a small order — it
	// is the venue's removal encoding leaking into a payload that has no way to
	// express one. Admitted, it publishes a resting order of size zero, which
	// prices as available liquidity that no fill will ever come back from.
	ErrLevelQuantityNotPositive = errors.New("venue: book level quantity is not positive")

	// ErrChangeQuantityNegative reports a depth instruction with negative size.
	// Zero is legal here and means remove; negative means this seam is reading a
	// size-delta protocol as a size-set one, and the two diverge silently — the
	// book stays plausible and drifts.
	ErrChangeQuantityNegative = errors.New("venue: level change quantity is negative")

	// ErrWireUndecodable, ErrWireForeign and ErrWireIncomplete are the wire
	// decoders' three refusal families, split by where they send the reader.
	//
	// Undecodable is bytes that are not the format at all — the reader goes to
	// the file (corruption, wrong stream family). Foreign is well-formed bytes
	// from a stream this decoder does not own — a markPrice decoder handed a
	// kline, a ticker frame that is neither snapshot nor delta nor op — and the
	// reader goes to the wiring that routed it. Incomplete is the right stream
	// missing a field the venue always states — the reader goes to the
	// Profiles entry, because either the venue changed its wire or the entry
	// was wrong. Transport chatter (a Bybit op acknowledgement) is NONE of
	// these: it is consumed without news, because it genuinely rides the
	// stream.
	ErrWireUndecodable = errors.New("venue: wire bytes are not the expected format")
	ErrWireForeign     = errors.New("venue: wire frame from a stream this decoder does not own")
	ErrWireIncomplete  = errors.New("venue: wire frame missing a field the venue always states")

	// ErrDeltaFirstSeqUnset reports a depth message with no first sequence.
	//
	// It is the one half of the span this payload holds — BookDelta records why
	// the other half stays on the envelope — so a zero here is a dropped field,
	// not a venue starting from zero. Left through, it makes every delta claim a
	// span beginning at 1, which reads as an enormous overlap against any live
	// book and applies deltas the reconstruction has already applied.
	ErrDeltaFirstSeqUnset = errors.New("venue: book delta first sequence unset")

	// The order surface's refusals (§4's OrderGateway.Place, venue-side).
	//
	// Every one of these is caught BEFORE a byte leaves the process, which is
	// what makes them EffectNotApplied by evidence rather than by assumption —
	// the one Effect §5.3 forbids inferring from a cause.
	ErrOrderSideInvalid         = errors.New("venue: order side is empty or outside the declared pair")
	ErrOrderClientIDUnset       = errors.New("venue: order carries no client order id")
	ErrOrderQuantityNotPositive = errors.New("venue: order quantity is not positive")
	ErrOrderCodecUnset          = errors.New("venue: order codec unset")
	ErrOrderTransportsUnset     = errors.New("venue: order client has no signed transport for a product family")
	ErrOrderGovernorUnset       = errors.New("venue: order governor unset")
	ErrOrderGovernorMismatch    = errors.New("venue: order transport uses a different governor")
	ErrOrderReserveEmpty        = errors.New("venue: order reservation batch is empty")

	// ErrReduceOnlyOnSpot reports §5.3's flag on an instrument that has no
	// position to reduce. Refused rather than dropped: an order sent without a
	// flag the caller asked for is an order that can INCREASE a holding.
	ErrReduceOnlyOnSpot = errors.New("venue: reduce-only is set on a spot instrument")

	// ErrOrderUrgencyRoutine reports §7.6's read-traffic class on an order.
	// Refused rather than rounded, because Routine sits BETWEEN the two
	// ceilings and both roundings are a real decision — see OrderRequest.
	ErrOrderUrgencyRoutine = errors.New("venue: an order cannot carry the Routine shed class")

	// ErrOrderVenueMismatch reports an order routed to another venue's client.
	// It matters because the symbol usually exists on both: the request would
	// be signed with these credentials and accepted by this host.
	ErrOrderVenueMismatch = errors.New("venue: order is for a different venue than this client")

	// ErrOrderFamilyUnrouted reports a product family this client was not
	// configured for. Refused rather than defaulted for the reason
	// OrderClient's doc gives: on Binance the families are different HOSTS, and
	// "BTCUSDT" is a real, executable instrument on both.
	ErrOrderFamilyUnrouted = errors.New("venue: order client has no route for this product family")

	// ErrOrderIDUnacceptable reports a ClientOrderID the venue's own rule
	// rejects — too long, or carrying a character outside its published set
	// (Profiles §7). Caught here so the failure names the id rather than
	// arriving as a venue rejection that reads like a credential fault.
	ErrOrderIDUnacceptable = errors.New("venue: client order id is outside this venue's published rule")

	// ErrAckUnreadable reports an acknowledgement that decoded but does not
	// describe the order that was sent — a venue echoing a different client id,
	// or none at all. Distinct from a JSON failure: the bytes parsed, and what
	// they say is about another order.
	ErrAckUnreadable = errors.New("venue: order acknowledgement does not match the order sent")

	// The read surface's refusals (slice C2b, Profiles §8).
	//
	// # None of these means "the order is not there"
	//
	// That answer is a VALUE — OrderPresence — and keeping it out of this list is
	// the point of §52c: the two venues state an absence in opposite layers
	// (Binance an error envelope, -2013 "Order does not exist."; Bybit a success
	// with an empty list), and an absence arriving as an error is
	// indistinguishable at the call site from a query that was rate-limited,
	// which is the inference §5.3 forbids in terms.
	ErrOrderPresenceInvalid = errors.New("venue: order presence is empty or outside the declared pair")
	ErrOrderStateInvalid    = errors.New("venue: order state is empty or outside the declared set")
	ErrQuerySurfaceInvalid  = errors.New("venue: query surface is empty or outside the declared set")

	// ErrQueryReachUnset reports a surface whose retention nobody populated.
	// Zero is not "forever" — an unpopulated horizon reads as an unbounded
	// promise, and an unbounded promise is what makes a stale absence look like
	// proved absence (§52d).
	ErrQueryReachUnset = errors.New("venue: query surface reach is unset")

	// ErrQueryReachContradictory reports a reach claiming both that the venue
	// publishes no retention and a duration derived from one.
	ErrQueryReachContradictory = errors.New("venue: query surface reach is unpublished and yet states a retention")

	// ErrCursorUnset reports a resume point nobody populated. There is no zero
	// cursor meaning "from the beginning", because every surface here is
	// retention-bounded and the beginning does not exist.
	ErrCursorUnset = errors.New("venue: execution cursor names no resume instant")

	// ErrExecutionWindowUnset reports a sweep with no horizon. A defaulted
	// window returns executions on all three surfaces and proves coverage
	// through nothing, which is the only thing §6.7 step 3 wants.
	ErrExecutionWindowUnset = errors.New("venue: execution window names no horizon")

	// ErrExecutionWindowInverted reports a horizon at or before the resume point.
	ErrExecutionWindowInverted = errors.New("venue: execution window ends at or before it starts")

	// ErrExecutionWindowTooWide reports a window exceeding what the surface will
	// serve — 24 HOURS on Binance spot, 7 days on Binance USDⓈ-M and on Bybit.
	// Refused rather than truncated: a truncated sweep reporting success is a
	// false watermark, and a false watermark is what §6.7 step 3 collapses a
	// possible-delta interval on.
	ErrExecutionWindowTooWide = errors.New("venue: execution window is wider than this surface serves")

	// ErrExecutionsUnreadable reports a page that decoded but does not describe
	// the sweep that was asked for — an execution for another instrument, or one
	// outside the requested window.
	ErrExecutionsUnreadable = errors.New("venue: execution page does not match the sweep requested")

	// ErrExecutionsNoProgress reports a full page whose executions all share one
	// instant, so the inclusive cursor cannot advance past them.
	//
	// A refusal rather than a loop: the alternative is a sweep that issues venue
	// calls forever while every request succeeds and every page decodes. It is
	// not reachable for this system's own traffic — one IOC per leg produces a
	// bounded number of executions in one millisecond — and it is named because
	// the assumption is about our order flow rather than about the venue's.
	ErrExecutionsNoProgress = errors.New("venue: a full execution page shares one instant, so the cursor cannot advance")

	// ErrInstrumentAssetsUnset reports a base/quote decomposition nobody
	// populated. The read path MINTS money from venue strings and the kernel
	// refuses to build a Quantity without its tag, so an empty asset is not a
	// missing label — it is a Money whose runtime check will fail somewhere else.
	ErrInstrumentAssetsUnset = errors.New("venue: instrument base/quote assets unset")

	// ErrInstrumentAssetsIdentical reports base == quote, which no tradable
	// instrument has and which makes the kernel's cross-asset check vacuous:
	// a Price of BTC-per-BTC compares equal to everything it should refuse.
	ErrInstrumentAssetsIdentical = errors.New("venue: instrument base and quote assets are the same")

	// ErrPositionSettleUnset reports a position scope with no settlement asset.
	//
	// It is refused rather than defaulted because Bybit's linear enumeration is
	// ILLEGAL without one (either symbol or settleCoin is required, and this
	// surface deliberately sends no symbol), so the two candidate defaults are a
	// call the venue rejects and a call that silently enumerates the wrong
	// settlement asset — and the second answers "no positions" for an account
	// that holds them.
	ErrPositionSettleUnset = errors.New("venue: position scope names no settlement asset")

	// ErrPositionSurfaceAbsent reports a position read asked of a family that has
	// no position surface — spot, on both venues.
	//
	// A distinct sentinel from ErrOrderFamilyUnrouted, and the distinction is
	// what a reader does next: an unrouted family is a wiring gap someone can
	// close by configuring a transport, while this one cannot be closed at all. A
	// spot leg's exposure is a wallet balance, which is not a position and is not
	// comparable to one without a baseline this system does not record.
	ErrPositionSurfaceAbsent = errors.New("venue: this product family has no position surface")

	// ErrPositionsUnreadable reports a position answer that does not describe the
	// enumeration requested — a row with no symbol, a second row for a symbol
	// already answered, an envelope naming another category.
	ErrPositionsUnreadable = errors.New("venue: position answer does not match the enumeration requested")

	// ErrPositionModeNotOneWay reports a row that is not one-way — Binance
	// positionSide other than BOTH, Bybit positionIdx other than 0.
	//
	// §4.1 makes a hedge position mode refuse-to-trade, and this is where it is
	// first observable: a hedge-mode account answers with a LONG row and a SHORT
	// row for one symbol, whose net is a subtraction no field on either row
	// states. Refusing is what stops one leg being reported as the position.
	ErrPositionModeNotOneWay = errors.New("venue: position row is not one-way position mode")

	// ErrPositionSideUnreadable reports a row whose direction cannot be read: an
	// undeclared side, or Bybit's documented empty side beside a NONZERO size.
	//
	// The second is the one worth naming. An empty side with size 0 is the
	// documented flat row and is admitted; the same field with a live size is a
	// direction the venue did not state, and the two available defaults report a
	// real position as flat or as pointing the wrong way.
	ErrPositionSideUnreadable = errors.New("venue: position row states no readable direction")

	// ErrPositionInstrumentAmbiguous reports a row whose symbol matches more
	// than one declared instrument in the caller's table.
	//
	// Refused rather than resolved to either, because the resolution is a coin
	// flip: Go's map iteration is randomized, so "the first match" is a different
	// instrument on different runs, and the two carry different base assets in
	// exactly the case that produced the duplicate — an unvalidated ContractType
	// splitting one real instrument across two keys (domain.FillKey records the
	// same hazard on the fill dedupe identity).
	ErrPositionInstrumentAmbiguous = errors.New("venue: more than one declared instrument matches this position row")

	// ErrSpotUnpinnedReasonInvalid reports a SpotUnpinnedRow whose reason is
	// the zero value or out of roster — an asset outside the assertion with no
	// account of why (task #179).
	ErrSpotUnpinnedReasonInvalid = errors.New("venue: spot balance row names no valid unpinned reason")

	// ErrSpotAbsenceSemanticsInvalid reports a SpotBalanceReport that never
	// stated what its silences mean — the zero value or out of roster. The
	// codec stamps the venue's enumeration kind; a report without one would
	// leave every consumer to inherit a reading of absence, which is the
	// policy-with-no-carrier failure (§70i).
	ErrSpotAbsenceSemanticsInvalid = errors.New("venue: spot balance report states no absence semantics")

	// ErrSpotBalancesUnreadable reports a wallet enumeration whose answer
	// cannot be attributed row by row — a rowless Binance account body, a
	// missing or foreign accountType echo, a duplicate asset, a row with no
	// asset name. The positions surface's ErrPositionsUnreadable one surface
	// over, and separate from it because the two answers feed different
	// halves of §7.5's assertion (task #179, record §70d).
	ErrSpotBalancesUnreadable = errors.New("venue: spot balance answer is unreadable")

	// ErrSpotBalanceWordSet reports verbatim balance words that are not
	// exactly the venue's documented field set — one missing, one stated
	// twice, or one nothing consumes. The strictness is the baseline
	// document's: a stored word nothing reads is a claim nobody checks
	// (record §70b), and the live codec building its own words cannot miss,
	// so a miss is a codec defect rather than a venue surprise.
	ErrSpotBalanceWordSet = errors.New("venue: spot balance words do not match the venue's field set")

	// ErrPostureProbeInvalid reports a PostureStep or PostureObservation whose
	// probe is the zero value or out of range (task #198, record §73).
	ErrPostureProbeInvalid = errors.New("venue: posture step names no valid probe")

	// ErrPostureSettingInvalid reports a PostureObservation naming no setting.
	ErrPostureSettingInvalid = errors.New("venue: posture observation names no valid setting")

	// ErrPostureScopeInvalid reports a PostureObservation naming no scope.
	ErrPostureScopeInvalid = errors.New("venue: posture observation names no valid scope")

	// ErrPosturePresenceInvalid reports a PostureObservation that never stated
	// how much the venue said. The zero value is refused rather than read as
	// "stated", because a fixture built without thinking about presence would
	// otherwise assert the strongest reading of a field nobody supplied.
	ErrPosturePresenceInvalid = errors.New("venue: posture observation states no presence")

	// ErrPostureUnreadable reports a posture answer that cannot be attributed:
	// an observation whose scope and subject disagree, a value carried beside a
	// presence that says there was none, an observation tagged with a probe
	// that did not produce it, or a codec rendering no steps at all. The
	// balance surface's ErrSpotBalancesUnreadable one surface over.
	ErrPostureUnreadable = errors.New("venue: account posture answer is unreadable")

	// ErrPostureUnsupported reports a venue codec with no posture surface.
	//
	// Separate from ErrPostureUnreadable because the two mean opposite things
	// to an operator: unreadable is a venue that answered strangely, while
	// unsupported is this system never having taught itself to ask — and §4.1
	// makes the posture a precondition of trading at all, so a venue whose
	// posture cannot be read is not a venue this system may trade on.
	ErrPostureUnsupported = errors.New("venue: codec has no account posture surface")
)

// There is deliberately no sentinel here for "funding asked of a spot
// instrument". §4 marks StreamFunding perp-only, and that rule now lives on
// events.SourceID.Validate, which is the type holding both the instrument and
// the stream — see StreamKind.RequiresPerp. FundingSource returns
// events.ErrStreamProductMismatch unchanged rather than translating it, because
// a second name for one refusal is a second thing to keep in step, and the
// value a caller branches on should name the rule's owner.
