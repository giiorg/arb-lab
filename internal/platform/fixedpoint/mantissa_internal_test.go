package fixedpoint

import (
	"errors"
	"testing"
)

// mulDivMantissa is the only place in the package where a result can fail to
// land on the canonical scale, so it is the only place rounding policy is
// implemented. It is tested directly rather than only through Money.Mul and
// friends: seven modes across both signs, ties, the 128-bit path and both range
// bounds is a table that would be unreadable tangled up with asset checks.
//
// declaredModes is written out in declaration order rather than derived by
// iterating until Validate fails, so that this table and the one in
// rounding_test.go have to be changed independently — a new mode cannot slip
// past both by satisfying a loop bound.
var declaredModes = []RoundingMode{
	RoundExact,
	RoundTowardZero,
	RoundAwayFromZero,
	RoundFloor,
	RoundCeil,
	RoundHalfUp,
	RoundHalfEven,
}

const (
	maxInt64 = 1<<63 - 1
	twoP62   = 1 << 62
)

func TestMulDivMantissa(t *testing.T) {
	// Every declared mode must appear in exactly one of want/wantErr, so a
	// mode cannot be left unexercised by being forgotten.
	cases := []struct {
		name      string
		a, b, den int64
		want      map[RoundingMode]int64
		wantErr   map[RoundingMode]error
	}{
		{
			name: "exact positive", a: 6, b: 1, den: 3,
			want: allModes(2),
		},
		{
			name: "exact negative", a: -6, b: 1, den: 3,
			want: allModes(-2),
		},
		{
			// A tie, and the truncated quotient is odd — the case where
			// HalfUp and HalfEven part company in the same direction.
			name: "positive tie, odd quotient (3.5)", a: 7, b: 1, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: 3, RoundAwayFromZero: 4,
				RoundFloor: 3, RoundCeil: 4,
				RoundHalfUp: 4, RoundHalfEven: 4,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// The same tie with an even quotient: HalfEven now goes the other
			// way from HalfUp. If these two ever agree on both, one of them is
			// not implemented.
			name: "positive tie, even quotient (2.5)", a: 5, b: 1, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: 2, RoundAwayFromZero: 3,
				RoundFloor: 2, RoundCeil: 3,
				RoundHalfUp: 3, RoundHalfEven: 2,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// Negative ties: Floor and Ceil swap roles relative to
			// TowardZero/AwayFromZero. Getting this wrong is the classic
			// "truncation is floor" bug, which is only true for positives.
			name: "negative tie, odd quotient (-3.5)", a: -7, b: 1, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: -3, RoundAwayFromZero: -4,
				RoundFloor: -4, RoundCeil: -3,
				RoundHalfUp: -4, RoundHalfEven: -4,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			name: "negative tie, even quotient (-2.5)", a: -5, b: 1, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: -2, RoundAwayFromZero: -3,
				RoundFloor: -3, RoundCeil: -2,
				RoundHalfUp: -3, RoundHalfEven: -2,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// Below half with an odd denominator, where no exact tie exists.
			// den/2 truncates to 1 and the remainder is 1, so a naive
			// "r >= den/2 rounds up" test would round this up wrongly.
			name: "below half (1/3)", a: 1, b: 1, den: 3,
			want: map[RoundingMode]int64{
				RoundTowardZero: 0, RoundAwayFromZero: 1,
				RoundFloor: 0, RoundCeil: 1,
				RoundHalfUp: 0, RoundHalfEven: 0,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// Below half with an odd denominator *and* an odd quotient. This
			// is the only shape that catches a HalfEven that mistakes
			// rem == den/2 for a tie: with an even quotient the parity rule
			// happens to give the right answer anyway.
			name: "below half, odd quotient (10/3)", a: 10, b: 1, den: 3,
			want: map[RoundingMode]int64{
				RoundTowardZero: 3, RoundAwayFromZero: 4,
				RoundFloor: 3, RoundCeil: 4,
				RoundHalfUp: 3, RoundHalfEven: 3,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			name: "above half (2/3)", a: 2, b: 1, den: 3,
			want: map[RoundingMode]int64{
				RoundTowardZero: 0, RoundAwayFromZero: 1,
				RoundFloor: 0, RoundCeil: 1,
				RoundHalfUp: 1, RoundHalfEven: 1,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// Rounding away from zero across zero: the magnitude leaves zero
			// even though the quotient is zero.
			name: "below half, negative (-1/3)", a: -1, b: 1, den: 3,
			want: map[RoundingMode]int64{
				RoundTowardZero: 0, RoundAwayFromZero: -1,
				RoundFloor: -1, RoundCeil: 0,
				RoundHalfUp: 0, RoundHalfEven: 0,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// The sign is the product of three signs, not two: a negative
			// divisor flips it as well. Same expectations as -3.5 above.
			name: "negative divisor", a: 7, b: 1, den: -2,
			want: map[RoundingMode]int64{
				RoundTowardZero: -3, RoundAwayFromZero: -4,
				RoundFloor: -4, RoundCeil: -3,
				RoundHalfUp: -4, RoundHalfEven: -4,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// Two negatives make a positive. Same expectations as 3.5.
			name: "two negatives", a: -7, b: -1, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: 3, RoundAwayFromZero: 4,
				RoundFloor: 3, RoundCeil: 4,
				RoundHalfUp: 4, RoundHalfEven: 4,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// 5e9 * 5e9 = 2.5e19, past 2^64: the intermediate genuinely needs
			// 128 bits. An int64 intermediate would wrap here.
			name: "128-bit intermediate, exact", a: 5_000_000_000, b: 5_000_000_000, den: 1_000_000_000,
			want: allModes(25_000_000_000),
		},
		{
			// Same 128-bit product, now with a remainder of 1 out of 3.
			name: "128-bit intermediate, inexact", a: 5_000_000_000, b: 5_000_000_000, den: 3,
			want: map[RoundingMode]int64{
				RoundTowardZero: 8_333_333_333_333_333_333, RoundAwayFromZero: 8_333_333_333_333_333_334,
				RoundFloor: 8_333_333_333_333_333_333, RoundCeil: 8_333_333_333_333_333_334,
				RoundHalfUp: 8_333_333_333_333_333_333, RoundHalfEven: 8_333_333_333_333_333_333,
			},
			wantErr: map[RoundingMode]error{RoundExact: ErrPrecisionLoss},
		},
		{
			// The asymmetry once more, now in the multiply path: 2^63 is
			// representable as a magnitude only on the negative side.
			name: "reaches the negative bound", a: -twoP62, b: 2, den: 1,
			want: allModes(minInt64),
		},
		{
			name: "one past the positive bound", a: twoP62, b: 2, den: 1,
			wantErr: allModeErrs(ErrOverflow),
		},
		{
			// The quotient needs more than 64 bits. This is the case that
			// makes math/bits.Div64 panic, so the guard has to come first —
			// a panic here would land on §5.3's crash boundary.
			name: "quotient exceeds 64 bits", a: 1_000_000_000_000_000_000, b: 1_000_000_000_000_000_000, den: 1,
			wantErr: allModeErrs(ErrOverflow),
		},
		{
			// (2^32-1) * (2^32+1) = 2^64-1, so over a denominator of 2 the
			// truncated quotient is exactly MaxInt64 with an exact-tie
			// remainder. Every mode that rounds the magnitude up therefore
			// leaves the range, while the modes that truncate do not.
			name: "rounding pushes past the bound", a: 4_294_967_295, b: 4_294_967_297, den: 2,
			want: map[RoundingMode]int64{
				RoundTowardZero: maxInt64,
				RoundFloor:      maxInt64,
			},
			wantErr: map[RoundingMode]error{
				RoundExact:        ErrPrecisionLoss,
				RoundAwayFromZero: ErrOverflow,
				RoundCeil:         ErrOverflow,
				RoundHalfUp:       ErrOverflow,
				RoundHalfEven:     ErrOverflow, // MaxInt64 is odd, so the tie rounds up
			},
		},
		{
			name: "zero numerator", a: 0, b: 12345, den: 7,
			want: allModes(0),
		},
		{
			// (2^32+1)^2 / 2 = 2^63 + 2^32 + 0.5. The truncated magnitude is
			// already past the positive bound *and* there is a remainder, so
			// two failures apply at once and the package has to pick one.
			//
			// Overflow wins, for every mode including RoundExact. A value that
			// cannot be held at all is a more fundamental failure than one
			// that cannot be held exactly, and reporting precision loss would
			// send a caller looking at their rounding policy when the actual
			// problem is that the number is 2^63 times too big. Reporting it
			// per-mode would be worse still: the classification would depend
			// on which side of an internal 64-bit boundary the magnitude fell.
			name: "out of range and inexact at once", a: 4_294_967_297, b: 4_294_967_297, den: 2,
			wantErr: allModeErrs(ErrOverflow),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range declaredModes {
				wantVal, hasVal := tc.want[mode]
				wantErr, hasErr := tc.wantErr[mode]
				if hasVal == hasErr {
					t.Fatalf("%v is listed %s in this case's expectations; "+
						"every declared mode must appear in exactly one of want/wantErr",
						mode, map[bool]string{true: "twice", false: "neither in want nor in wantErr"}[hasVal])
				}

				got, err := mulDivMantissa(tc.a, tc.b, tc.den, mode)
				if hasErr {
					if err == nil {
						t.Errorf("mulDivMantissa(%d, %d, %d, %v) = %d, nil; want %v",
							tc.a, tc.b, tc.den, mode, got, wantErr)
					} else if !errors.Is(err, wantErr) {
						t.Errorf("mulDivMantissa(%d, %d, %d, %v) error = %v; want it to wrap %v",
							tc.a, tc.b, tc.den, mode, err, wantErr)
					}
					continue
				}
				if err != nil {
					t.Errorf("mulDivMantissa(%d, %d, %d, %v) failed: %v", tc.a, tc.b, tc.den, mode, err)
					continue
				}
				if got != wantVal {
					t.Errorf("mulDivMantissa(%d, %d, %d, %v) = %d; want %d",
						tc.a, tc.b, tc.den, mode, got, wantVal)
				}
			}
		})
	}
}

// An unusable mode must be rejected even when the result happens to be exact.
// Otherwise `x.Mul(y, mode)` with a mode nobody set would succeed or fail
// depending on the data — the worst kind of intermittent, because it passes in
// every test that uses round numbers.
func TestMulDivMantissa_ModeIsCheckedBeforeTheResult(t *testing.T) {
	for _, mode := range []RoundingMode{RoundInvalid, RoundingMode(99)} {
		// 6/3 is exact, so nothing about the data needs a rounding decision.
		if _, err := mulDivMantissa(6, 1, 3, mode); !errors.Is(err, ErrRoundingMode) {
			t.Errorf("mulDivMantissa with %v on an exact result = %v; want ErrRoundingMode", mode, err)
		}
	}
}

func TestMulDivMantissa_DivideByZero(t *testing.T) {
	for _, mode := range declaredModes {
		if _, err := mulDivMantissa(1, 1, 0, mode); !errors.Is(err, ErrDivideByZero) {
			t.Errorf("mulDivMantissa(1, 1, 0, %v) = %v; want ErrDivideByZero", mode, err)
		}
	}
}

func allModes(v int64) map[RoundingMode]int64 {
	m := make(map[RoundingMode]int64, len(declaredModes))
	for _, mode := range declaredModes {
		m[mode] = v
	}
	return m
}

func allModeErrs(err error) map[RoundingMode]error {
	m := make(map[RoundingMode]error, len(declaredModes))
	for _, mode := range declaredModes {
		m[mode] = err
	}
	return m
}
