package fixedpoint

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Property tests against the math/big oracle in oracle_test.go.
//
// Each property is written twice over: once as a *predicate* — a pure function
// from inputs to "here is how these two disagree, or empty if they do not" —
// and once as a rapid generator that feeds it. Splitting them that way is what
// makes the negative controls possible. A negative control runs the same
// predicate against a deliberately broken implementation over a fixed sweep of
// inputs, and fails if the predicate does not object.
//
// The discipline comes from docs/formal/: NC1-NC5 exist there because a passing
// liveness property and a vacuous one look identical from the outside. The same
// is true here. A property that compares two functions proves nothing until you
// have shown it can tell two functions apart.
//
// Reproducing a failure: rapid's seed defaults to 0, which means "choose a
// random one", so these run on fresh ground every time. On failure rapid prints
// both a fail file and the concrete -rapid.seed=N to replay. See the
// test-properties and test-properties-seeded make targets.

// ---------------------------------------------------------------------------
// Generators
// ---------------------------------------------------------------------------

// interestingMantissas are the values that have historically been where this
// package goes wrong: the range bounds, the scale factor, and the operands
// whose product lands exactly on a 64-bit boundary.
var interestingMantissas = []int64{
	0, 1, -1, 2, -2, 3, -3,
	scaleFactor, -scaleFactor,
	minInt64, math.MaxInt64,
	minInt64 + 1, math.MaxInt64 - 1,
	1 << 62, -(1 << 62),
	1 << 31, 1 << 32,
	4_294_967_295, 4_294_967_297, // (2^32-1) and (2^32+1)
	5_000_000_000, -5_000_000_000,
	1_000_000_000_000_000_000,
}

// genMantissa mixes the full int64 range with a band of small values and the
// interesting set. Uniform int64 alone would almost never produce a tie or a
// near-boundary result, which is where the bugs are.
func genMantissa() *rapid.Generator[int64] {
	return rapid.OneOf(
		rapid.Int64(),
		rapid.Int64Range(-1_000_000, 1_000_000),
		rapid.SampledFrom(interestingMantissas),
	)
}

func genMode() *rapid.Generator[RoundingMode] {
	return rapid.SampledFrom(declaredModes)
}

// ---------------------------------------------------------------------------
// Property 1: mulDivMantissa agrees with the oracle
// ---------------------------------------------------------------------------

type mulDivFunc func(a, b, den int64, mode RoundingMode) (int64, error)

// checkMulDiv reports how op differs from the oracle, or "" if it does not.
func checkMulDiv(op mulDivFunc, a, b, den int64, mode RoundingMode) string {
	want, wantErr := oracleMulDiv(a, b, den, mode)
	got, gotErr := op(a, b, den, mode)

	switch {
	case wantErr != nil && gotErr == nil:
		return "want " + wantErr.Error() + ", got " + strconv.FormatInt(got, 10)
	case wantErr == nil && gotErr != nil:
		return "want " + strconv.FormatInt(want, 10) + ", got error " + gotErr.Error()
	case wantErr != nil:
		if !errors.Is(gotErr, wantErr) {
			return "want error " + wantErr.Error() + ", got " + gotErr.Error()
		}
	case got != want:
		return "want " + strconv.FormatInt(want, 10) + ", got " + strconv.FormatInt(got, 10)
	}
	return ""
}

func TestProperty_MulDivMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genMantissa().Draw(t, "a")
		b := genMantissa().Draw(t, "b")
		den := genMantissa().Draw(t, "den")
		mode := genMode().Draw(t, "mode")

		if diff := checkMulDiv(mulDivMantissa, a, b, den, mode); diff != "" {
			t.Fatalf("mulDivMantissa(%d, %d, %d, %v): %s", a, b, den, mode, diff)
		}
	})
}

// mulDivSweep is the fixed input set the negative controls run over. It is
// deliberately not random: a negative control that only sometimes catches its
// defect is not a control.
var mulDivSweep = []struct{ a, b, den int64 }{
	{7, 1, 2}, {5, 1, 2}, {-7, 1, 2}, {-5, 1, 2}, // ties, both parities, both signs
	{1, 1, 3}, {2, 1, 3}, {10, 1, 3}, {-1, 1, 3}, // odd denominators, no exact tie
	{6, 1, 3},               // exact
	{7, 1, -2}, {-7, -1, 2}, // sign carried by the divisor and by both operands
	{5_000_000_000, 5_000_000_000, 3}, // 128-bit intermediate
	{5_000_000_000, 5_000_000_000, 1_000_000_000},
	{1 << 62, 2, 1}, {-(1 << 62), 2, 1}, // the asymmetric bounds
	{4_294_967_295, 4_294_967_297, 2},                         // truncates to exactly MaxInt64, with a tie
	{4_294_967_297, 4_294_967_297, 2},                         // out of range and inexact at once
	{1_000_000_000_000_000_000, 1_000_000_000_000_000_000, 1}, // quotient past 2^64
	{math.MaxInt64, scaleFactor, scaleFactor},
	{0, 12345, 7}, {12345, 0, 7}, // zero numerator either side
	{1, 1, 0}, // divide by zero
}

// runMulDivControl reports whether the predicate objected to op somewhere in
// the sweep, and where.
func runMulDivControl(op mulDivFunc) (caught bool, where string) {
	for _, in := range mulDivSweep {
		for _, mode := range append([]RoundingMode{RoundInvalid}, declaredModes...) {
			if diff := checkMulDiv(op, in.a, in.b, in.den, mode); diff != "" {
				return true, strconv.FormatInt(in.a, 10) + "*" + strconv.FormatInt(in.b, 10) +
					"/" + strconv.FormatInt(in.den, 10) + " " + mode.String() + ": " + diff
			}
		}
	}
	return false, ""
}

// TestProperty_MulDivNegativeControls is what makes the property above worth
// running. Each entry is a plausible way to get the kernel wrong, and the
// property must reject every one of them.
func TestProperty_MulDivNegativeControls(t *testing.T) {
	controls := map[string]mulDivFunc{
		// The single most likely defect: ignoring the mode and truncating,
		// which is what a first draft written around integer division does.
		"always truncates": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if err := mode.Validate(); err != nil {
				return 0, err
			}
			return oracleMulDiv(a, b, den, RoundTowardZero)
		},
		// Banker's rounding implemented as ties-away, which is what happens
		// when someone reads "half" and stops there.
		"half-even resolves ties away from zero": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if mode == RoundHalfEven {
				mode = RoundHalfUp
			}
			return oracleMulDiv(a, b, den, mode)
		},
		// Truncation mistaken for flooring — true only for positive values.
		"floor truncates": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if mode == RoundFloor {
				mode = RoundTowardZero
			}
			return oracleMulDiv(a, b, den, mode)
		},
		// An unset mode quietly given a default.
		"unset mode defaults": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if mode == RoundInvalid {
				mode = RoundHalfEven
			}
			return oracleMulDiv(a, b, den, mode)
		},
		// RoundExact absorbing the loss instead of reporting it.
		"exact silently truncates": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if mode == RoundExact {
				mode = RoundTowardZero
			}
			return oracleMulDiv(a, b, den, mode)
		},
		// A 64-bit intermediate: the defect that turns 5.0 * 5e9 into a
		// plausible number four times too small.
		"64-bit intermediate": func(a, b, den int64, mode RoundingMode) (int64, error) {
			if err := mode.Validate(); err != nil {
				return 0, err
			}
			if den == 0 {
				return 0, ErrDivideByZero
			}
			if a == minInt64 || b == minInt64 || den == -1 {
				return 0, ErrOverflow // sidestep the language's own panic
			}
			return a * b / den, nil
		},
		// No range check at all.
		"wraps instead of erroring": func(a, b, den int64, mode RoundingMode) (int64, error) {
			v, err := oracleMulDiv(a, b, den, mode)
			if errors.Is(err, ErrOverflow) {
				return 0, nil
			}
			return v, err
		},
	}

	for name, op := range controls {
		t.Run(name, func(t *testing.T) {
			caught, where := runMulDivControl(op)
			if !caught {
				t.Errorf("the property accepted a known-wrong implementation (%s); "+
					"it cannot be distinguishing anything", name)
				return
			}
			t.Logf("rejected at %s", where)
		})
	}

	// The positive control: the real implementation must survive the same
	// sweep. Without this, a predicate that always objects would look like a
	// perfect set of negative controls.
	if caught, where := runMulDivControl(mulDivMantissa); caught {
		t.Fatalf("the real mulDivMantissa was rejected by its own predicate at %s", where)
	}
}

// ---------------------------------------------------------------------------
// Property 2: parse and format round-trip, and agree with the oracle
// ---------------------------------------------------------------------------

type formatFunc func(int64) string

// checkFormat reports how format disagrees with the oracle formatter, or how it
// breaks the round trip through the real parser.
func checkFormat(format formatFunc, m int64) string {
	got := format(m)
	if want := oracleFormat(m); got != want {
		return "formatted " + strconv.FormatInt(m, 10) + " as " + got + "; oracle says " + want
	}
	back, err := parseMantissa(got)
	if err != nil {
		return "canonical output " + got + " does not parse: " + err.Error()
	}
	if back != m {
		return "round trip " + strconv.FormatInt(m, 10) + " -> " + got +
			" -> " + strconv.FormatInt(back, 10)
	}
	return ""
}

func TestProperty_FormatRoundTrips(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := genMantissa().Draw(t, "m")
		if diff := checkFormat(formatMantissa, m); diff != "" {
			t.Fatalf("%s", diff)
		}
	})
}

var formatSweep = append([]int64{
	7, -7, 500_000_000, -500_000_000, 1_500_000_000, 999_999_999, 1_000_000_001,
}, interestingMantissas...)

func TestProperty_FormatNegativeControls(t *testing.T) {
	controls := map[string]formatFunc{
		// Trailing zeros left in place: still parses back, but not canonical,
		// so two equal values would compare unequal as text in the SQL codec.
		"keeps trailing zeros": func(m int64) string {
			s := formatMantissa(m)
			if strings.Contains(s, ".") {
				return s + "0"
			}
			return s + ".0"
		},
		// The last digit of the fraction dropped — the shape of an off-by-one
		// in the scale padding.
		"drops the last fractional digit": func(m int64) string {
			s := formatMantissa(m)
			if i := strings.IndexByte(s, '.'); i >= 0 && len(s) > i+2 {
				return s[:len(s)-1]
			}
			return s
		},
		// The sign lost on negatives.
		"loses the sign": func(m int64) string {
			return strings.TrimPrefix(formatMantissa(m), "-")
		},
		// Rendered as if the mantissa were the value, ignoring the scale.
		"ignores the scale": func(m int64) string {
			return strconv.FormatInt(m, 10)
		},
	}

	for name, format := range controls {
		t.Run(name, func(t *testing.T) {
			for _, m := range formatSweep {
				if diff := checkFormat(format, m); diff != "" {
					t.Logf("rejected at %d: %s", m, diff)
					return
				}
			}
			t.Errorf("the property accepted a known-wrong formatter (%s)", name)
		})
	}

	for _, m := range formatSweep {
		if diff := checkFormat(formatMantissa, m); diff != "" {
			t.Fatalf("the real formatMantissa was rejected by its own predicate: %s", diff)
		}
	}
}

// ---------------------------------------------------------------------------
// Property 3: parseMantissa agrees with the oracle parser
// ---------------------------------------------------------------------------

type parseFunc func(string) (int64, error)

func checkParse(parse parseFunc, s string) string {
	want, wantErr := oracleParse(s)
	got, gotErr := parse(s)

	switch {
	case wantErr != nil && gotErr == nil:
		return s + ": want " + wantErr.Error() + ", got " + strconv.FormatInt(got, 10)
	case wantErr == nil && gotErr != nil:
		return s + ": want " + strconv.FormatInt(want, 10) + ", got error " + gotErr.Error()
	case wantErr != nil:
		if !errors.Is(gotErr, wantErr) {
			return s + ": want error " + wantErr.Error() + ", got " + gotErr.Error()
		}
	case got != want:
		return s + ": want " + strconv.FormatInt(want, 10) + ", got " + strconv.FormatInt(got, 10)
	}
	return ""
}

// genDecimalText builds numbers from the grammar both parsers accept. The forms
// only this package's parser takes — a bare "1." or ".5" — are left to the
// hand-written table, because big.Rat rejects them and the disagreement would
// be about the grammar rather than about the arithmetic.
func genDecimalText(t *rapid.T) string {
	digits := rapid.RuneFrom([]rune("0123456789"))
	var b strings.Builder
	b.WriteString(rapid.SampledFrom([]string{"", "+", "-"}).Draw(t, "sign"))
	b.WriteString(rapid.StringOfN(digits, 1, 14, -1).Draw(t, "int"))
	if rapid.Bool().Draw(t, "hasFrac") {
		b.WriteByte('.')
		b.WriteString(rapid.StringOfN(digits, 1, 14, -1).Draw(t, "frac"))
	}
	if rapid.Bool().Draw(t, "hasExp") {
		b.WriteString(rapid.SampledFrom([]string{"e", "E"}).Draw(t, "e"))
		b.WriteString(rapid.SampledFrom([]string{"", "+", "-"}).Draw(t, "expSign"))
		b.WriteString(strconv.Itoa(rapid.IntRange(0, 40).Draw(t, "exp")))
	}
	return b.String()
}

func TestProperty_ParseMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := genDecimalText(t)
		if diff := checkParse(parseMantissa, s); diff != "" {
			t.Fatalf("parseMantissa: %s", diff)
		}
	})
}

var parseSweep = []string{
	"0", "1", "-1", "+1", "0.5", "-1.25", "1.500", "0.000000001", "-0.000000001",
	"123456789.123456789", "1e-8", "1E3", "1.5e2", "-2.5E-1", "0e100",
	"9223372036.854775807", "-9223372036.854775808", "9223372036.854775808",
	"1.0000000001", "0.0000000001", "1e-10", "1e30", "100000000000000000000",
}

func TestProperty_ParseNegativeControls(t *testing.T) {
	controls := map[string]parseFunc{
		// Excess precision absorbed rather than reported.
		"truncates excess precision": func(s string) (int64, error) {
			v, err := oracleParse(s)
			if errors.Is(err, ErrPrecisionLoss) {
				r, ok := new(big.Rat).SetString(s)
				if !ok {
					return 0, ErrSyntax
				}
				scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(bigScaleFactor))
				return new(big.Int).Quo(scaled.Num(), scaled.Denom()).Int64(), nil
			}
			return v, err
		},
		// The classic float detour, which is exactly what §3.6 rule 4 forbids.
		"routes through float64": func(s string) (int64, error) {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0, ErrSyntax
			}
			return int64(f * scaleFactor), nil
		},
		// The exponent ignored.
		"ignores the exponent": func(s string) (int64, error) {
			if i := strings.IndexAny(s, "eE"); i >= 0 {
				s = s[:i]
			}
			return oracleParse(s)
		},
		// Overflow wrapped rather than reported.
		"wraps on overflow": func(s string) (int64, error) {
			v, err := oracleParse(s)
			if errors.Is(err, ErrOverflow) {
				return 0, nil
			}
			return v, err
		},
	}

	for name, parse := range controls {
		t.Run(name, func(t *testing.T) {
			for _, s := range parseSweep {
				if diff := checkParse(parse, s); diff != "" {
					t.Logf("rejected at %q: %s", s, diff)
					return
				}
			}
			t.Errorf("the property accepted a known-wrong parser (%s)", name)
		})
	}

	for _, s := range parseSweep {
		if diff := checkParse(parseMantissa, s); diff != "" {
			t.Fatalf("the real parseMantissa was rejected by its own predicate: %s", diff)
		}
	}
}

// ---------------------------------------------------------------------------
// Property 4: addition and subtraction agree with unbounded integers
// ---------------------------------------------------------------------------

type binOpFunc func(a, b int64) (int64, error)

// checkBinOp compares op against the same operation performed in big.Int, where
// no range exists: the exact answer is right when it fits and must be reported
// as overflow when it does not.
func checkBinOp(op binOpFunc, exact func(a, b *big.Int) *big.Int, a, b int64) string {
	want := exact(big.NewInt(a), big.NewInt(b))
	got, gotErr := op(a, b)

	if !want.IsInt64() {
		if gotErr == nil {
			return strconv.FormatInt(a, 10) + ", " + strconv.FormatInt(b, 10) +
				": want ErrOverflow (exact result " + want.String() + "), got " +
				strconv.FormatInt(got, 10)
		}
		if !errors.Is(gotErr, ErrOverflow) {
			return "want ErrOverflow, got " + gotErr.Error()
		}
		return ""
	}
	if gotErr != nil {
		return strconv.FormatInt(a, 10) + ", " + strconv.FormatInt(b, 10) +
			": want " + want.String() + ", got error " + gotErr.Error()
	}
	if got != want.Int64() {
		return "want " + want.String() + ", got " + strconv.FormatInt(got, 10)
	}
	return ""
}

func bigAdd(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) }
func bigSub(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) }

func TestProperty_AddSubMatchUnboundedIntegers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genMantissa().Draw(t, "a")
		b := genMantissa().Draw(t, "b")

		if diff := checkBinOp(addMantissa, bigAdd, a, b); diff != "" {
			t.Fatalf("addMantissa: %s", diff)
		}
		if diff := checkBinOp(subMantissa, bigSub, a, b); diff != "" {
			t.Fatalf("subMantissa: %s", diff)
		}
	})
}

func TestProperty_AddSubNegativeControls(t *testing.T) {
	type control struct {
		op    binOpFunc
		exact func(a, b *big.Int) *big.Int
	}
	controls := map[string]control{
		"add wraps": {func(a, b int64) (int64, error) { return a + b, nil }, bigAdd},
		"sub wraps": {func(a, b int64) (int64, error) { return a - b, nil }, bigSub},
		// The refactor that looks obviously equivalent and is not: negating b
		// wraps at the negative bound before anything checks it.
		"sub as add of the negation": {
			func(a, b int64) (int64, error) { return addMantissa(a, -b) }, bigSub,
		},
		// Overflow detected only on the positive side.
		"add checks only the top": {
			func(a, b int64) (int64, error) {
				s := a + b
				if a > 0 && b > 0 && s < 0 {
					return 0, ErrOverflow
				}
				return s, nil
			}, bigAdd,
		},
	}

	sweep := append([]int64{4, -4, 100, -100}, interestingMantissas...)
	for name, c := range controls {
		t.Run(name, func(t *testing.T) {
			for _, a := range sweep {
				for _, b := range sweep {
					if diff := checkBinOp(c.op, c.exact, a, b); diff != "" {
						t.Logf("rejected at (%d, %d): %s", a, b, diff)
						return
					}
				}
			}
			t.Errorf("the property accepted a known-wrong implementation (%s)", name)
		})
	}

	for _, a := range sweep {
		for _, b := range sweep {
			if diff := checkBinOp(addMantissa, bigAdd, a, b); diff != "" {
				t.Fatalf("the real addMantissa was rejected by its own predicate: %s", diff)
			}
			if diff := checkBinOp(subMantissa, bigSub, a, b); diff != "" {
				t.Fatalf("the real subMantissa was rejected by its own predicate: %s", diff)
			}
		}
	}
}
