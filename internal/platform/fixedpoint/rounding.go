package fixedpoint

import "fmt"

// RoundingMode names how an operation whose exact result does not land on the
// canonical scale must resolve.
//
// Build-spec §3.6 rule 4 requires it to be passed explicitly at every such call
// site rather than defaulted. Rounding is a *policy* decision about money — a
// fee understated by one unit in the last place is a different kind of wrong
// from an order size overstated by one — and a default would hand that decision
// to whoever wrote the helper instead of whoever owns the number.
//
// It is an integer enum rather than the `type X string` shape §3.6 rule 8
// prescribes, deliberately. Rule 8's shape exists for enums that cross a
// persistence or wire boundary, where a self-describing stored token is worth
// the width. A RoundingMode is an argument: never persisted, never transmitted.
// The property that matters here is the opposite one — that the zero value be
// an unusable sentinel — and only an integer enum gives that for free.
type RoundingMode uint8

const (
	// RoundInvalid is the zero value and is never a valid argument. A field or
	// variable nobody set therefore errors instead of silently meaning
	// whichever mode happened to be declared first. This is the same move as
	// Money's invalid zero value, applied to the other half of the signature.
	RoundInvalid RoundingMode = iota

	// RoundExact fails with ErrPrecisionLoss unless the result is exact. It is
	// how an invariant test asserts "this computation must not lose anything",
	// which is how a scale bug gets caught rather than absorbed.
	RoundExact

	// RoundTowardZero truncates the magnitude. Use it for lot quantization:
	// never size an order beyond the balance backing it.
	RoundTowardZero

	// RoundAwayFromZero increases the magnitude. Use it for costs and fees: a
	// cost must never be understated, because milestone 2's verdict (§12.2) is
	// a comparison of measured carry against a cost hurdle.
	RoundAwayFromZero

	// RoundFloor rounds toward negative infinity.
	RoundFloor

	// RoundCeil rounds toward positive infinity. With RoundFloor this is tick
	// quantization, where the side of the book dictates the direction.
	RoundCeil

	// RoundHalfUp resolves ties away from zero, and is what an exchange's
	// published fee schedule normally means by "rounded".
	RoundHalfUp

	// RoundHalfEven resolves ties toward the even neighbour. Use it for
	// valuation and PnL, where a biased tie rule accumulates across thousands
	// of settlements rather than cancelling out.
	RoundHalfEven
)

// maxRoundingMode is the last declared mode. Validate compares against it, so
// adding a mode means extending this — which the declared-set test checks.
const maxRoundingMode = RoundHalfEven

// Validate reports whether r is one of the declared modes.
//
// RoundingMode is a defined uint8, so RoundingMode(n) compiles for every n.
// Validate is the only thing between a stray cast and an operation quietly
// taking a branch nobody wrote.
func (r RoundingMode) Validate() error {
	if r == RoundInvalid || r > maxRoundingMode {
		return fmt.Errorf("fixedpoint: rounding mode %d: %w", uint8(r), ErrRoundingMode)
	}
	return nil
}

// String implements fmt.Stringer.
//
// This switch is the shape the deferred exhaustiveswitch gate exists to police:
// today a new constant added above falls through to the default and prints as a
// number, which is survivable here and would not be in a safety state machine.
func (r RoundingMode) String() string {
	switch r {
	case RoundInvalid:
		return "RoundInvalid"
	case RoundExact:
		return "RoundExact"
	case RoundTowardZero:
		return "RoundTowardZero"
	case RoundAwayFromZero:
		return "RoundAwayFromZero"
	case RoundFloor:
		return "RoundFloor"
	case RoundCeil:
		return "RoundCeil"
	case RoundHalfUp:
		return "RoundHalfUp"
	case RoundHalfEven:
		return "RoundHalfEven"
	}
	return fmt.Sprintf("RoundingMode(%d)", uint8(r))
}
