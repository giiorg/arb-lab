package domain

import "errors"

// The domain's refusals. Each is a sentinel so a caller can branch on the kind
// without matching text, and every construction site wraps it with the value
// that failed — §8f's lesson, paid for twice in platform/config: one sentinel
// cannot say WHICH check fired, so the message is the diagnosis somebody acts
// on and the sentinel only says where to look.
var (
	// ErrWatermarkUnset reports a watermark timestamp nobody populated. It is
	// separate from a stale one on purpose: an unset instant is the zero time,
	// whose age is about fifty-five years, so absorbing it into staleness would
	// report a healthy feed as permanently blind and send an operator to the
	// venue rather than to the field.
	ErrWatermarkUnset = errors.New("marketdata: watermark timestamp unset")

	// ErrClockInversion reports an instant preceding the one it is measured
	// from — §3.6 rule 5's two-disagreeing-clocks defect, arriving as a
	// negative duration. Named rather than clamped, because a negative age
	// compares as younger than every budget and would read as permanently
	// fresh.
	ErrClockInversion = errors.New("marketdata: clock inversion")

	// ErrFreshnessInvalid reports a Freshness outside the declared values.
	ErrFreshnessInvalid = errors.New("marketdata: freshness is not a declared value")

	// ErrBudgetInvalid reports a non-positive budget bound. Zero is refused
	// rather than read as "no budget": an unpopulated Budget would otherwise
	// mark every observation blind, or — with the comparison written the other
	// way — every observation fresh forever.
	ErrBudgetInvalid = errors.New("marketdata: freshness budget bound is not positive")

	// ErrBudgetUnreachable reports a budget whose stale rung cannot be reached
	// (MaxAge >= Blind). See Budget.Validate: nothing fails at runtime, the
	// ladder just silently has two rungs where it is configured to have three,
	// and the first symptom is the de-lever the warning existed to precede.
	ErrBudgetUnreachable = errors.New("marketdata: freshness budget leaves the stale rung unreachable")

	// ErrFundingOnSpot reports a funding observation naming a spot instrument.
	// Refused rather than answered with a zero rate: a spot leg reading
	// "funding 0" is indistinguishable from a perp in a flat regime, and the
	// cash-and-carry carry computation nets the two legs — so the wrong answer
	// would be silently absorbed into the number milestone 2's gate reads.
	ErrFundingOnSpot = errors.New("marketdata: spot instrument has no funding")

	// ErrClampInverted reports a cap below its floor. Not merely nonsense: the
	// forecast clamps later settlements to this band (§2.4), and clamping to an
	// inverted band either pins every estimate to one end or, depending on which
	// clamp is applied first, passes anything through.
	ErrClampInverted = errors.New("marketdata: funding cap below floor")

	// ErrIntervalInvalid reports a non-positive funding interval. Zero is the
	// dangerous one: Enumerate would not terminate, so the failure is a hung
	// process rather than a wrong number — and a test that hangs names no
	// defect.
	ErrIntervalInvalid = errors.New("marketdata: funding interval is not positive")

	// ErrNextSettleUnset reports an observation with no next settlement
	// instant. Its zero value is the zero time, which precedes every horizon,
	// so an unpopulated field would enumerate settlements back to year one.
	ErrNextSettleUnset = errors.New("marketdata: next settlement instant unset")

	// ErrHorizonUnset reports an enumeration asked for with no terminal
	// instant. Distinct from a horizon that is simply short: the zero time
	// yields an empty schedule, which reads as "this hold captures nothing"
	// — a real and very wrong answer to a question nobody meant to ask.
	ErrHorizonUnset = errors.New("marketdata: enumeration horizon unset")

	// ErrHorizonTooLong reports a horizon spanning more settlements than any
	// real hold plans. §2.4's holds are weeks; a request for thousands is a
	// unit error in the caller's horizon — nanoseconds handed to a method
	// expecting an instant — and refusing names it, where allocating the slice
	// would answer at length and let the mistake through.
	ErrHorizonTooLong = errors.New("marketdata: enumeration horizon spans too many settlements")

	// ErrSideUnset and ErrSideUnknown report a BookSide that is empty and one
	// that is neither declared value. Separate, on the same argument
	// events.StreamKind makes: an empty side is an unpopulated field and a
	// misspelt one is an adapter mapping a venue token we do not know, and the
	// two send a reader to different files.
	ErrSideUnset   = errors.New("marketdata: book side unset")
	ErrSideUnknown = errors.New("marketdata: book side is not a declared value")

	// ErrLevelQuantityNotPositive reports a level in a SNAPSHOT resting zero or
	// less. Zero is the one worth refusing: in delta language zero means REMOVE,
	// so a zero-quantity level in a reconstructed book is a removal that leaked
	// through as a resting order. A consumer testing len(Bids) > 0 sees a best
	// bid; anything routed to it fills nothing.
	ErrLevelQuantityNotPositive = errors.New("marketdata: book level quantity is not positive")

	// ErrChangeQuantityNegative reports a level CHANGE below zero. Zero is legal
	// here and means remove — that asymmetry with a snapshot level is the whole
	// encoding, and it is why the two refusals are separate sentinels rather
	// than one shared "bad quantity".
	ErrChangeQuantityNegative = errors.New("marketdata: book level change quantity is negative")

	// ErrBookUnordered reports a side whose prices do not descend (bids) or
	// ascend (asks) strictly. Strictly, because a repeated price is two resting
	// levels at one price — which is not a book state any venue publishes, it is
	// an apply that inserted where it should have replaced.
	ErrBookUnordered = errors.New("marketdata: book side is not strictly ordered")

	// ErrBookCrossed reports a best bid at or above the best ask.
	//
	// This is a desync detector, not a tidiness check. A correctly sequenced L2
	// book is never crossed, so a reconstruction that produces one has diverged
	// from the venue — and the direction of the refusal is what makes it matter:
	// a crossed book yields a NEGATIVE spread, which reads to a strategy hunting
	// price dislocation as unbounded edge. Refused rather than published with a
	// flag, because §3.4 makes what leaves reconstruction "the resulting
	// immutable snapshot" and a snapshot nobody may trade on is not one.
	ErrBookCrossed = errors.New("marketdata: book is crossed")

	// ErrBookAssetMismatch reports levels within one book whose price pairs or
	// quantity assets disagree. A book mixing two instruments' levels sorts
	// perfectly well and prices nothing — fixedpoint's runtime tag is what
	// notices, and this is where it is asked.
	ErrBookAssetMismatch = errors.New("marketdata: book levels disagree on asset")

	// ErrBookInstrumentMismatch reports a delta applied to a book for a
	// different instrument. §5.1 makes spot BTCUSDT and perp BTCUSDT distinct
	// instruments with independent everything; their books are also
	// price-plausible against each other, which is exactly why crossing them
	// produces a wrong answer rather than an obvious one.
	ErrBookInstrumentMismatch = errors.New("marketdata: delta names a different instrument than the book")

	// ErrDeltaSeqUnset reports a delta carrying no venue sequence.
	//
	// Watermark documents a zero VenueSeq as the ordinary case for a polled
	// read, and a delta is the one payload where it cannot be: gap detection is
	// the entire reason this type is separate from a snapshot, and it is
	// computed from nothing else. A stream that supplies no sequence must be
	// consumed as snapshots, not as deltas.
	ErrDeltaSeqUnset = errors.New("marketdata: book delta carries no venue sequence")

	// ErrDeltaSeqInverted reports LastSeq below FirstSeq — a range covering no
	// sequence at all, which every relation test would answer about vacuously.
	ErrDeltaSeqInverted = errors.New("marketdata: book delta last sequence precedes its first")

	// ErrDeltaSeqDisagrees reports a delta whose Watermark.VenueSeq is not its
	// LastSeq.
	//
	// The two fields state one fact, and a field that restates another is a
	// field that can disagree with it. Both exist because Watermark is the
	// provenance every stream carries — freshness code must not have to know a
	// book from a funding read — while the FirstSeq/LastSeq span is book-shaped.
	// Requiring agreement is what keeps a typo in one of them from making gap
	// detection and freshness disagree about where the source is.
	ErrDeltaSeqDisagrees = errors.New("marketdata: book delta sequence disagrees with its watermark")

	// ErrDeltaDuplicateLevel reports two changes to one (side, price) within a
	// single delta. Refused rather than resolved last-wins: applying them in
	// slice order makes the result depend on the order an adapter happened to
	// decode a JSON array in, and §3.5 makes determinism the parity mechanism
	// rather than a convenience.
	ErrDeltaDuplicateLevel = errors.New("marketdata: book delta changes one level twice")

	// ErrSeqRelationInvalid reports a SeqRelation outside the declared values —
	// the same refusal ErrFreshnessInvalid makes, for the same integer-enum
	// reason: an out-of-range value is not a state, it is a cast.
	ErrSeqRelationInvalid = errors.New("marketdata: sequence relation is not a declared value")
)
