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
)

// There is deliberately no sentinel here for "funding asked of a spot
// instrument". §4 marks StreamFunding perp-only, and that rule now lives on
// events.SourceID.Validate, which is the type holding both the instrument and
// the stream — see StreamKind.RequiresPerp. FundingSource returns
// events.ErrStreamProductMismatch unchanged rather than translating it, because
// a second name for one refusal is a second thing to keep in step, and the
// value a caller branches on should name the rule's owner.
