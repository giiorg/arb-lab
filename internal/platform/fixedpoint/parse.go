package fixedpoint

import (
	"fmt"
	"strconv"
	"strings"
)

// parseMantissa converts decimal text to a mantissa at the canonical scale,
// exactly. It never routes through float64 — the whole point of §3.6 rule 4 is
// that a value which merely *looks* right after a float detour is the bug this
// package exists to prevent.
//
// The grammar accepted is strconv's, minus Inf/NaN/hex/underscores:
//
//	[+-] digits [ "." digits ] [ ("e"|"E") [+-] digits ]
//
// with at least one digit somewhere in the mantissa. Whitespace, trailing junk
// and doubled points are rejected rather than tolerated.
func parseMantissa(s string) (int64, error) {
	i := 0
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}

	start := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	intDigits := s[start:i]

	var fracDigits string
	if i < len(s) && s[i] == '.' {
		i++
		start = i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		fracDigits = s[start:i]
	}

	if intDigits == "" && fracDigits == "" {
		return 0, fmt.Errorf("parse %q: %w", s, ErrSyntax)
	}

	// Exponent, validated syntactically here and interpreted below — a
	// syntactically broken exponent is a syntax error even when the mantissa
	// digits are all zero.
	expNeg := false
	var expDigits string
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			expNeg = s[i] == '-'
			i++
		}
		start = i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		expDigits = s[start:i]
		if expDigits == "" {
			return 0, fmt.Errorf("parse %q: %w", s, ErrSyntax)
		}
	}

	// Anything left over — whitespace, a second point, stray letters — is a
	// syntax error. Consuming the whole input is what makes "1.2.3" and " 1"
	// fail instead of silently parsing a prefix.
	if i != len(s) {
		return 0, fmt.Errorf("parse %q: %w", s, ErrSyntax)
	}

	// Leading zeros do not affect the integer value of the digit string, and
	// pointPos below is derived from the original fraction length, so stripping
	// them here is safe and keeps the width checks tight.
	digits := strings.TrimLeft(intDigits+fracDigits, "0")
	if digits == "" {
		// Zero, whatever the sign and exponent say.
		return 0, nil
	}

	exp := 0
	if expDigits != "" {
		v, err := strconv.Atoi(expDigits)
		if err != nil || v > maxExponent {
			// Too large to reason about; classify by direction. Digits are
			// known non-zero here, so neither branch can be a legitimate zero.
			if expNeg {
				return 0, fmt.Errorf("parse %q: %w", s, ErrPrecisionLoss)
			}
			return 0, fmt.Errorf("parse %q: %w", s, ErrOverflow)
		}
		exp = v
		if expNeg {
			exp = -exp
		}
	}

	// The value is `digits` scaled by 10^-pointPos; we want it at 10^-Scale.
	pointPos := len(fracDigits) - exp
	shift := Scale - pointPos

	switch {
	case shift < 0:
		// More precision than we store: the digits being dropped must all be
		// zero, or the value genuinely does not fit.
		drop := -shift
		if drop >= len(digits) {
			return 0, fmt.Errorf("parse %q: %w", s, ErrPrecisionLoss)
		}
		if strings.Trim(digits[len(digits)-drop:], "0") != "" {
			return 0, fmt.Errorf("parse %q: %w", s, ErrPrecisionLoss)
		}
		digits = digits[:len(digits)-drop]
	case shift > 0:
		// Check the width BEFORE materializing the zeros, so a hostile exponent
		// cannot make us allocate to then reject.
		//
		// How much this is worth today, stated honestly because the mutation
		// audit measured it: maxExponent already caps shift at Scale+1000, so
		// the allocation this skips is about a kilobyte, not the gigabyte an
		// unbounded exponent would buy. Mutating the `+` to `-` is therefore
		// observably equivalent — identical values and identical errors across
		// every exponent-heavy input, differing only in bytes allocated on a
		// path that errors either way — which is why no test pins it and why it
		// survives the audit as a known, accepted survivor rather than a gap.
		// It stays because it is the check that keeps that claim true if
		// maxExponent is ever raised, and raising it is a one-line change.
		if len(digits)+shift > maxMantissaDigits {
			return 0, fmt.Errorf("parse %q: %w", s, ErrOverflow)
		}
		digits += strings.Repeat("0", shift)
	}

	// Parse unsigned, then apply the sign, because MinInt64 has no positive
	// counterpart: "-9223372036.854775808" is representable while
	// "9223372036.854775808" is not.
	u, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", s, ErrOverflow)
	}
	if neg {
		if u > 1<<63 {
			return 0, fmt.Errorf("parse %q: %w", s, ErrOverflow)
		}
		if u == 1<<63 {
			return minInt64, nil
		}
		return -int64(u), nil
	}
	if u > 1<<63-1 {
		return 0, fmt.Errorf("parse %q: %w", s, ErrOverflow)
	}
	return int64(u), nil
}

// formatMantissa renders a mantissa as canonical decimal text: no exponent, no
// trailing zeros, and "0" for zero. Shared by every category so that the round
// trip is identical whichever type persisted the value.
func formatMantissa(v int64) string {
	if v == 0 {
		return "0"
	}

	// Take the magnitude via uint64: negating MinInt64 would overflow.
	neg := v < 0
	var u uint64
	if neg {
		u = uint64(-(v + 1)) + 1
	} else {
		u = uint64(v)
	}

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	b.WriteString(strconv.FormatUint(u/scaleFactor, 10))
	if frac := u % scaleFactor; frac != 0 {
		fs := strconv.FormatUint(frac, 10)
		fs = strings.Repeat("0", Scale-len(fs)) + fs // left-pad to the scale
		b.WriteByte('.')
		b.WriteString(strings.TrimRight(fs, "0"))
	}
	return b.String()
}

const minInt64 = -1 << 63

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
