package fixedpoint

import (
	"fmt"
	"math/bits"
)

// This file is the integer kernel: operations on bare mantissas, with no asset
// tag and no category. Every one of the four categories (Money, Quantity,
// Price, Rate) funnels its arithmetic through here, so the overflow and
// rounding rules are written once and cannot drift apart between types.
//
// The rule these all share is §3.6's: a result outside the representable range
// is an error, never a wrap. Wrapping is the failure mode that does not
// announce itself — a balance that silently becomes its own negation is a
// plausible-looking number, and this package exists to stop those.

// addMantissa adds two mantissas, reporting overflow rather than wrapping.
//
// A sum of two int64 values can only leave the range when both operands share a
// sign, and when it does the result's sign is the opposite one. That is the
// whole check — no widening required.
func addMantissa(a, b int64) (int64, error) {
	s := a + b
	if (a > 0 && b > 0 && s < 0) || (a < 0 && b < 0 && s >= 0) {
		return 0, ErrOverflow
	}
	return s, nil
}

// subMantissa subtracts, reporting overflow rather than wrapping.
//
// It is deliberately not addMantissa(a, -b): negating minInt64 overflows, so
// that formulation would reject `x - minInt64` before discovering whether the
// difference actually fits. Subtraction can only leave the range when the
// operands' signs differ, and then the result takes the sign of b.
func subMantissa(a, b int64) (int64, error) {
	d := a - b
	if (a >= 0 && b < 0 && d < 0) || (a < 0 && b > 0 && d >= 0) {
		return 0, ErrOverflow
	}
	return d, nil
}

// negMantissa negates, reporting overflow rather than wrapping.
//
// This is the arithmetic face of the asymmetry the parser already had to
// handle: minInt64 has no positive counterpart, and in plain Go `-minInt64`
// evaluates back to minInt64. A sign flip that silently returns the same sign
// is exactly the kind of quiet wrong answer that reaches a report unnoticed.
func negMantissa(v int64) (int64, error) {
	if v == minInt64 {
		return 0, ErrOverflow
	}
	return -v, nil
}

// signMantissa is the three-way sign, shared so that every category's Sign and
// Cmp agree by construction.
func signMantissa(v int64) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// cmpMantissa is the three-way comparison. Subtracting and taking the sign
// would be shorter and would be wrong: the difference can overflow.
func cmpMantissa(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// magnitude returns |v| as a uint64.
//
// It is written the long way because -minInt64 is not representable: the two
// steps take v to -(v+1), which is in range, and then add the one back in the
// wider unsigned type where 2^63 exists.
func magnitude(v int64) uint64 {
	if v < 0 {
		return uint64(-(v + 1)) + 1
	}
	return uint64(v)
}

// mulDivMantissa computes a * b / den, rounded according to mode.
//
// This is the only operation in the package whose exact result can fail to land
// on the canonical scale, so it is the only place rounding policy exists. Every
// multiplication and division in every category routes through it — a scaled
// multiply is mulDivMantissa(a, b, scaleFactor) and a division is
// mulDivMantissa(a, scaleFactor, b) — so there is one implementation of "what
// happens to the part that does not fit", not one per operation.
//
// The intermediate is a full 128-bit product. Truncating it to 64 bits first is
// the bug this exists to avoid: at scale 9, two values as ordinary as 5.0 and
// 5.0 billion already produce a product past 2^64.
func mulDivMantissa(a, b, den int64, mode RoundingMode) (int64, error) {
	// The mode is validated before anything looks at the data. If it were
	// checked only on the path that actually rounds, an unset mode would
	// succeed whenever the result happened to be exact — passing every test
	// written with round numbers and failing in production on real prices.
	if err := mode.Validate(); err != nil {
		return 0, err
	}
	if den == 0 {
		return 0, ErrDivideByZero
	}

	// The sign is the product of three signs, and it is tracked separately
	// from the magnitudes so that the negative bound stays reachable: 2^63 is
	// a valid magnitude on the negative side and not on the positive one.
	neg := (a < 0) != (b < 0)
	if den < 0 {
		neg = !neg
	}

	hi, lo := bits.Mul64(magnitude(a), magnitude(b))
	uden := magnitude(den)

	// bits.Div64 panics when the quotient would not fit in 64 bits. Guarding
	// first turns that into an error, because a panic in here would land
	// exactly on the DispatchAuthorized crash boundary (§5.3).
	if hi >= uden {
		return 0, ErrOverflow
	}
	quo, rem := bits.Div64(hi, lo, uden)

	// The truncated magnitude must be representable before the mode is
	// consulted at all. When a result is both out of range and inexact, two
	// failures apply and one has to be reported: overflow wins, because a value
	// that cannot be held is a more fundamental problem than one that cannot be
	// held exactly, and telling a caller "precision loss" would send them to
	// their rounding policy when the number is orders of magnitude too big.
	//
	// Doing it here rather than after rounding also stops the classification
	// from depending on which side of an internal 64-bit boundary the magnitude
	// happened to fall — the same condition would otherwise report overflow
	// above 2^64 and precision loss between 2^63 and 2^64.
	if _, err := signedFromMagnitude(quo, neg); err != nil {
		return 0, err
	}

	inc, err := roundingIncrement(mode, neg, quo, rem, uden)
	if err != nil {
		return 0, err
	}
	if inc > 0 {
		if quo == ^uint64(0) {
			return 0, ErrOverflow
		}
		quo += inc
	}
	return signedFromMagnitude(quo, neg)
}

// roundingIncrement decides whether the truncated magnitude must be bumped by
// one, given the remainder rem out of den and the sign of the exact result.
//
// Working on magnitudes rather than signed values is what keeps the modes
// straight: "truncate" and "floor" are the same operation only for positive
// numbers, and stating both in terms of "does the magnitude grow" makes the
// difference explicit instead of implicit in the sign of a division.
func roundingIncrement(mode RoundingMode, neg bool, quo, rem, den uint64) (uint64, error) {
	if rem == 0 {
		return 0, nil
	}
	switch mode {
	case RoundExact:
		return 0, ErrPrecisionLoss
	case RoundTowardZero:
		return 0, nil
	case RoundAwayFromZero:
		return 1, nil
	case RoundFloor:
		if neg {
			return 1, nil
		}
		return 0, nil
	case RoundCeil:
		if neg {
			return 0, nil
		}
		return 1, nil
	case RoundHalfUp, RoundHalfEven:
		// Compare rem against half of den without forming 2*rem, which can
		// leave uint64. For an odd den there is no exact tie: den = 2k+1 puts
		// rem == k strictly below half, which is why the equal case has to
		// re-test the parity rather than assume a tie.
		half := den / 2
		switch {
		case rem > half:
			return 1, nil
		case rem < half:
			return 0, nil
		case den%2 != 0:
			return 0, nil
		}
		if mode == RoundHalfUp {
			return 1, nil
		}
		// Banker's rounding: the tie goes to whichever neighbour is even.
		if quo%2 != 0 {
			return 1, nil
		}
		return 0, nil
	}
	// Unreachable for a validated mode; the compiler cannot know that.
	return 0, fmt.Errorf("fixedpoint: rounding mode %d: %w", uint8(mode), ErrRoundingMode)
}

// signedFromMagnitude reassembles a signed mantissa, reporting overflow rather
// than wrapping. The two bounds are checked separately because they are not
// symmetric: 2^63 is representable as a negative value and not as a positive.
func signedFromMagnitude(mag uint64, neg bool) (int64, error) {
	if neg {
		if mag > 1<<63 {
			return 0, ErrOverflow
		}
		if mag == 1<<63 {
			return minInt64, nil
		}
		return -int64(mag), nil
	}
	if mag > 1<<63-1 {
		return 0, ErrOverflow
	}
	return int64(mag), nil
}
