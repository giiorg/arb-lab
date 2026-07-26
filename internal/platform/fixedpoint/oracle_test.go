package fixedpoint

import (
	"math/big"
	"strings"
)

// This file is the oracle: a second, independent implementation of what this
// package computes, written to be obviously correct rather than fast.
//
// The tdd skill's warning about tautological tests is the whole design
// constraint here. An oracle that paraphrases the implementation — deriving an
// increment from a remainder and a sign, the way mulDivMantissa does — would
// agree with it for the same reasons it is wrong, and a property test against
// it would prove nothing. So the algorithm below is deliberately a different
// one:
//
//   - It computes the exact result as a big.Rat, at unbounded precision, with
//     no 128-bit intermediates and no overflow reasoning at all.
//   - It rounds by naming the two integer neighbours of the exact value and
//     *comparing the distances to each*, rather than by inspecting a remainder.
//     "Which of these two is closer, and what happens on a tie" is the
//     definition of rounding; the remainder formulation is an optimisation of it.
//   - It checks representability only at the end, by asking big.Int whether the
//     answer fits.
//
// It shares no helper with the production path — not magnitude, not
// signedFromMagnitude, not roundingIncrement.

var (
	bigOne         = big.NewInt(1)
	bigScaleFactor = big.NewInt(scaleFactor)
)

// oracleMulDiv computes a*b/den rounded to a mantissa, exactly.
//
// The error precedence it implements is the package's stated one: an invalid
// mode first, then division by zero, then a value that cannot be represented at
// all, then — only for values that were in range to begin with — inexactness
// under RoundExact.
func oracleMulDiv(a, b, den int64, mode RoundingMode) (int64, error) {
	if mode.Validate() != nil {
		return 0, ErrRoundingMode
	}
	if den == 0 {
		return 0, ErrDivideByZero
	}

	// The exact value, with no rounding and no range anywhere in sight.
	// big.Rat normalises the sign onto the numerator, so Denom is positive.
	exact := new(big.Rat).SetFrac(
		new(big.Int).Mul(big.NewInt(a), big.NewInt(b)),
		big.NewInt(den),
	)

	// Truncation toward zero is what the value would be with no rounding at
	// all. If even that cannot be held, no choice of mode can rescue it.
	trunc := new(big.Int).Quo(exact.Num(), exact.Denom())
	if !trunc.IsInt64() {
		return 0, ErrOverflow
	}

	// The two integer neighbours. big.Int.Div floors for a positive divisor,
	// and Rat guarantees a positive denominator, so lo really is the floor.
	lo := new(big.Int).Div(exact.Num(), exact.Denom())
	loRat := new(big.Rat).SetInt(lo)
	if loRat.Cmp(exact) == 0 {
		return fitsInt64(lo)
	}
	if mode == RoundExact {
		return 0, ErrPrecisionLoss
	}
	hi := new(big.Int).Add(lo, bigOne)
	hiRat := new(big.Rat).SetInt(hi)

	// Distances to each neighbour, compared directly. This is the definition
	// of rounding rather than a restatement of the fast path's arithmetic.
	distDown := new(big.Rat).Sub(exact, loRat)
	distUp := new(big.Rat).Sub(hiRat, exact)
	positive := exact.Sign() > 0

	var chosen *big.Int
	switch mode {
	case RoundFloor:
		chosen = lo
	case RoundCeil:
		chosen = hi
	case RoundTowardZero:
		chosen = pick(positive, lo, hi)
	case RoundAwayFromZero:
		chosen = pick(positive, hi, lo)
	case RoundHalfUp:
		switch distDown.Cmp(distUp) {
		case -1:
			chosen = lo
		case 1:
			chosen = hi
		default:
			chosen = pick(positive, hi, lo) // a tie goes away from zero
		}
	case RoundHalfEven:
		switch distDown.Cmp(distUp) {
		case -1:
			chosen = lo
		case 1:
			chosen = hi
		default:
			// Bit(0) is the parity of the value in two's complement, which is
			// the parity of the integer for negatives as well.
			chosen = pick(lo.Bit(0) == 0, lo, hi)
		}
	default:
		panic("oracle: unhandled rounding mode " + mode.String())
	}
	return fitsInt64(chosen)
}

func pick(cond bool, ifTrue, ifFalse *big.Int) *big.Int {
	if cond {
		return ifTrue
	}
	return ifFalse
}

func fitsInt64(v *big.Int) (int64, error) {
	if !v.IsInt64() {
		return 0, ErrOverflow
	}
	return v.Int64(), nil
}

// oracleParse converts decimal text to a mantissa using big.Rat's own parser,
// which accepts the same grammar this package does for the inputs the property
// generates. Where parseMantissa walks the string and tracks a point position,
// this defers the whole numeric question to math/big and only decides what to
// do about excess precision.
func oracleParse(s string) (int64, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, ErrSyntax
	}
	// Move the value onto the canonical scale, then require it to be integral:
	// anything left over is precision this representation cannot hold.
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(bigScaleFactor))
	if !scaled.IsInt() {
		return 0, ErrPrecisionLoss
	}
	return fitsInt64(scaled.Num())
}

// oracleFormat renders a mantissa as canonical decimal text by long division in
// big.Int, rather than by the scaleFactor quotient/remainder pair the
// production formatter uses.
func oracleFormat(m int64) string {
	v := big.NewInt(m)
	neg := v.Sign() < 0
	v.Abs(v)

	whole, frac := new(big.Int), new(big.Int)
	whole.QuoRem(v, bigScaleFactor, frac)

	out := whole.String()
	if frac.Sign() != 0 {
		digits := frac.String()
		digits = strings.Repeat("0", Scale-len(digits)) + digits
		out += "." + strings.TrimRight(digits, "0")
	}
	if neg && (whole.Sign() != 0 || frac.Sign() != 0) {
		out = "-" + out
	}
	return out
}
