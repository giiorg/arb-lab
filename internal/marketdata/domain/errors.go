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
)
