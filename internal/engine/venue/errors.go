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

	// ErrDeltaFirstSeqUnset reports a depth message with no first sequence.
	//
	// It is the one half of the span this payload holds — BookDelta records why
	// the other half stays on the envelope — so a zero here is a dropped field,
	// not a venue starting from zero. Left through, it makes every delta claim a
	// span beginning at 1, which reads as an enormous overlap against any live
	// book and applies deltas the reconstruction has already applied.
	ErrDeltaFirstSeqUnset = errors.New("venue: book delta first sequence unset")
)

// There is deliberately no sentinel here for "funding asked of a spot
// instrument". §4 marks StreamFunding perp-only, and that rule now lives on
// events.SourceID.Validate, which is the type holding both the instrument and
// the stream — see StreamKind.RequiresPerp. FundingSource returns
// events.ErrStreamProductMismatch unchanged rather than translating it, because
// a second name for one refusal is a second thing to keep in step, and the
// value a caller branches on should name the rule's owner.
