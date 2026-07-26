package fixedpoint

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
