package fixedpoint

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"testing"
)

// The parser is this package's only untrusted-input boundary. Every other entry
// point takes an int64 or another category — already constrained by the type
// system — but parseMantissa takes bytes that came off a venue wire, and a table
// can only assert the cases somebody thought of.
//
// So this file adds a fuzz target. A fuzz target that only checks for panics is
// a crash test, and crashing is not this parser's likely failure mode: returning
// a plausible number, or the wrong error class, is. The body therefore asserts
// four invariants that must hold for EVERY input, well-formed or not:
//
//	I1  the input is rejected as ErrSyntax exactly when it is outside the
//	    documented grammar, judged by an independent description of that grammar
//	I2  every error is classified as exactly one of the three sentinels, and a
//	    rejected input yields no value
//	I3  an accepted value survives the canonical round trip, idempotently
//	I4  where the math/big oracle is also defined, the two agree
//
// The body is written as a predicate over a parseFunc rather than inline in the
// f.Fuzz closure, in the same shape and for the same reason as the properties in
// property_test.go: it lets the negative controls below run the identical
// invariants against deliberately broken parsers and show that each one bites.
// `go test -fuzz` on its own cannot demonstrate that — a target whose assertions
// are all vacuously true reports the same clean run as one that works.
//
// Note also that `go test` runs a fuzz target's seed corpus as an ordinary table
// even without -fuzz, so the seeds below are a committed regression suite that
// the CI gate executes. -fuzz is the explorer on top, the same division as
// test / test-properties in the Makefile. See `make fuzz`.

// decimalGrammar is a second, independent statement of the grammar parse.go's
// doc comment specifies:
//
//	[+-] digits [ "." digits ] [ ("e"|"E") [+-] digits ]
//
// with at least one digit in the mantissa. It exists so that I1 compares the
// hand-written walker against something that is not the walker. A regexp is the
// right second implementation here precisely because it shares nothing with an
// index-advancing loop: the defects a walker has — consuming a prefix and not
// noticing the tail, accepting an empty exponent, letting a second point
// through — are not defects a regexp can have.
//
// \A and \z rather than ^ and $ because Go's regexp anchors are the text
// anchors, but spelling them explicitly makes the "must consume everything"
// requirement visible rather than inferred. Capture group 1 is the exponent's
// digits, which I4's gate needs.
var decimalGrammar = regexp.MustCompile(
	`\A[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?([0-9]+))?\z`)

// errorClasses are the three ways text can fail to become a mantissa. They are
// mutually exclusive by construction, and a caller reading a venue field
// branches on which one it got — "this venue quotes finer than we store" is a
// finding worth alerting on, "this is not a number" is a protocol bug. An error
// that is none of them, or two of them, is unusable to that caller.
var errorClasses = []error{ErrSyntax, ErrPrecisionLoss, ErrOverflow}

// checkParseInvariants reports how parse violates I1-I4 on s, or "" if it does
// not. Every branch names the invariant, because a fuzz failure arrives with no
// context beyond the input that produced it.
func checkParseInvariants(parse parseFunc, s string) string {
	got, err := parse(s)

	// I1: syntax rejection agrees with the independent grammar, in both
	// directions. The reverse direction is the load-bearing one — a parser that
	// reports ErrSyntax for a well-formed number that merely carries too much
	// precision sends the caller looking for a protocol bug that isn't there.
	inGrammar := decimalGrammar.FindStringSubmatch(s)
	if errors.Is(err, ErrSyntax) != (inGrammar == nil) {
		if inGrammar == nil {
			return fmt.Sprintf("I1: %q is outside the grammar, but it was not "+
				"rejected as syntax: got %d, %v", s, got, err)
		}
		return fmt.Sprintf("I1: %q is within the grammar, but it was rejected "+
			"as syntax: %v", s, err)
	}

	// I2: the error is classified, and carries no value.
	if err != nil {
		var classes []string
		for _, class := range errorClasses {
			if errors.Is(err, class) {
				classes = append(classes, class.Error())
			}
		}
		if len(classes) != 1 {
			return fmt.Sprintf("I2: %q was rejected with %v, which matches %d "+
				"of the sentinel classes (%v); callers branch on exactly one",
				s, err, len(classes), classes)
		}
		if got != 0 {
			return fmt.Sprintf("I2: %q was rejected with %v but also returned "+
				"%d; a rejected input has no value", s, err, got)
		}
	}

	// I3: the canonical round trip. An accepted mantissa must survive being
	// written out and read back, and the text must be a fixed point of that
	// cycle — otherwise two equal values could persist as different strings,
	// which is what the SQL codec's comparability depends on.
	if err == nil {
		canon := formatMantissa(got)
		if !decimalGrammar.MatchString(canon) {
			return fmt.Sprintf("I3: %q parsed to %d, which formats as %q — "+
				"outside the grammar the parser accepts", s, got, canon)
		}
		back, backErr := parse(canon)
		switch {
		case backErr != nil:
			return fmt.Sprintf("I3: %q parsed to %d, whose canonical form %q "+
				"does not parse: %v", s, got, canon, backErr)
		case back != got:
			return fmt.Sprintf("I3: round trip %q -> %d -> %q -> %d",
				s, got, canon, back)
		}
		if again := formatMantissa(back); again != canon {
			return fmt.Sprintf("I3: canonical form is not a fixed point: "+
				"%q -> %q -> %q", s, canon, again)
		}
	}

	// I4: agreement with the math/big oracle, where the oracle is defined. This
	// is the only invariant that can catch a parser which is wrong about the
	// *value* while remaining perfectly self-consistent — one that ignores the
	// exponent, say, round-trips its own output happily.
	//
	// checkParse is reused verbatim rather than reimplemented so that "agrees
	// with the oracle" has one definition in this package. The cost is a second
	// big.Rat parse per call, which a test can afford.
	if oracleAgreementApplies(s, inGrammar) {
		if _, oracleErr := oracleParse(s); !errors.Is(oracleErr, ErrSyntax) {
			if diff := checkParse(parse, s); diff != "" {
				return "I4: disagrees with the math/big oracle: " + diff
			}
		}
	}

	return ""
}

// oracleAgreementApplies gates I4. The oracle is a second implementation of the
// *arithmetic*, not of the grammar or of the limits, and the fuzzer will find
// every place those three diverge within seconds. Each exclusion below is a
// known, documented difference rather than a suppression:
//
//   - Grammar. big.Rat rejects "1." and ".5", which this parser accepts, and
//     accepts "1/2", hex floats and digit separators, which it does not. So I4
//     runs only on inputs inside *this* package's grammar (inGrammar != nil),
//     and the caller additionally skips any input the oracle called ErrSyntax.
//     A disagreement about which characters spell a number is not a
//     disagreement about what the number is.
//
//   - Limits. Rat.SetString refuses |exponent| > 1e6 outright (ratconv.go:181,
//     "avoid excessively large exponents") and reports it as a parse failure,
//     which is not what this package means by ErrSyntax: "1e2000000" is
//     well-formed text naming a value that is merely out of range. Capping the
//     exponent at four digits keeps every input well inside Rat's cutoff, so
//     the two are compared only where both have an opinion about the value —
//     and "1e9999" still exercises the real overflow path on both sides.
//
//   - Cost. Rat materializes 5^exponent to do this, and the length cap keeps a
//     fuzz iteration from spending milliseconds in math/big. The inputs this
//     excludes — thousand-digit mantissas — are exactly the ones the width
//     guard in parse.go rejects before doing any work, and I1-I3 still cover
//     them.
func oracleAgreementApplies(s string, inGrammar []string) bool {
	if inGrammar == nil || len(s) > 64 {
		return false
	}
	return len(strings.TrimLeft(inGrammar[1], "0")) <= 4
}

// fuzzSeeds are the shapes worth handing the fuzzer directly rather than hoping
// it stumbles onto them: the boundaries between the four outcomes, and the
// inputs where this parser and the oracle are known to part company.
//
// testdata/fuzz/ is deliberately not gitignored (see .gitignore): Go keeps its
// generated corpus in GOCACHE and writes into the repo only when it finds a
// FAILING input, so anything that appears there is a regression case to commit.
var fuzzSeeds = []string{
	// Not numbers at all, including the shapes a prefix-consuming walker would
	// accept.
	"", ".", "+", "-", "+.", "abc", "1.2.3", "--1", "1e", "1e+", "1.2e3.4",
	" 1", "1 ", "1\n", "1\x00", "0x10", "1_000", "1/2", "NaN", "Inf", "E5",

	// Accepted here, rejected by big.Rat — the grammar difference I4 excludes.
	"1.", ".5", "+.5", "-.0", "1.e5", "-0",

	// The zero paths, where the digits are all zero whatever the exponent says
	// and the exponent range check is skipped entirely.
	"0", "0.0", "0.000000000", "-0.000e5", "0e100", "0e-100", "0e2000000",
	"00000000000000000000.0",

	// The scale boundary: the last digit that fits, and the first that does not.
	"0.000000001", "0.0000000001", "1e-9", "1e-10", "1.0000000001",

	// The range boundary, and its asymmetry — MinInt64 has no positive
	// counterpart, so the positive side rejects one ulp earlier.
	"9223372036.854775807", "9223372036.854775808",
	"-9223372036.854775808", "-9223372036.854775809",

	// Exponents at and past this package's own cutoff, in both directions, and
	// past big.Rat's.
	"1e1000", "1e1001", "1e-1000", "1e9999", "1e2000000",
	"1e999999999999999999999", "1e-999999999999999999999",

	// Wide inputs, where the width guard in parse.go has to reject before
	// materializing anything.
	"1" + strings.Repeat("0", 30), "0." + strings.Repeat("0", 30) + "1",
	strings.Repeat("9", 40),
}

func FuzzParseMantissa(f *testing.F) {
	for _, s := range parseSweep {
		f.Add(s)
	}
	for _, s := range fuzzSeeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		if diff := checkParseInvariants(parseMantissa, s); diff != "" {
			t.Fatalf("%s", diff)
		}
	})
}

// invariantSweep is the fixed input set the negative controls run over: the fuzz
// seeds plus the property test's parse table. Deliberately not random, for the
// reason §5c gives — a control that only sometimes catches its defect is not a
// control.
var invariantSweep = append(append([]string{}, fuzzSeeds...), parseSweep...)

// TestParseInvariants_NegativeControls is what makes the fuzz target above worth
// running. Each control is a plausible way to get the parser wrong, and each is
// aimed at a specific invariant — so a control that stops being caught tells you
// which invariant went vacuous.
func TestParseInvariants_NegativeControls(t *testing.T) {
	controls := map[string]parseFunc{
		// I1. The defect a hand-written walker actually has: parsing the longest
		// valid prefix and never checking that the whole input was consumed.
		// This is the missing `i != len(s)` in parse.go, and it is why " 1" and
		// "1.2.3" have to fail rather than quietly become 1 and 1.2.
		"parses a prefix and ignores the rest": func(s string) (int64, error) {
			for n := len(s); n > 0; n-- {
				if decimalGrammar.MatchString(s[:n]) {
					return parseMantissa(s[:n])
				}
			}
			return parseMantissa(s)
		},

		// I1, the other direction. Excess precision reported as bad syntax:
		// well-formed text, misfiled as a protocol bug, so the venue change that
		// caused it never gets investigated.
		"reports excess precision as bad syntax": func(s string) (int64, error) {
			v, err := parseMantissa(s)
			if errors.Is(err, ErrPrecisionLoss) {
				return 0, fmt.Errorf("parse %q: %w", s, ErrSyntax)
			}
			return v, err
		},

		// I2. The classification collapsed into one opaque error — what happens
		// when someone consolidates the error returns and keeps only the
		// message. Syntax is left intact so that I1 does not catch this first.
		"loses the classification on numbers it cannot hold": func(s string) (int64, error) {
			v, err := parseMantissa(s)
			if err != nil && !errors.Is(err, ErrSyntax) {
				return 0, fmt.Errorf("fixedpoint: cannot parse %q", s)
			}
			return v, err
		},

		// I2's value clause. A tempting API: alongside ErrPrecisionLoss, hand
		// back what did fit. Tempting and wrong — a caller that checks the error
		// second, or logs it and carries on, silently gets the rounded value
		// this package exists to refuse.
		"returns the truncated value alongside ErrPrecisionLoss": func(s string) (int64, error) {
			v, err := parseMantissa(s)
			if errors.Is(err, ErrPrecisionLoss) {
				if r, ok := new(big.Rat).SetString(s); ok {
					scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(bigScaleFactor))
					if trunc := new(big.Int).Quo(scaled.Num(), scaled.Denom()); trunc.IsInt64() {
						return trunc.Int64(), err
					}
				}
			}
			return v, err
		},

		// I3. The scale applied once too few times. Syntactically flawless,
		// correctly classified, and off by a factor of ten — caught only because
		// the canonical form of the wrong answer no longer reads back as itself.
		"drops a digit of scale": func(s string) (int64, error) {
			v, err := parseMantissa(s)
			return v / 10, err
		},

		// I4. The exponent ignored. This is the one control that no amount of
		// self-consistency checking can catch: it round-trips its own output
		// perfectly, classifies every error correctly, and accepts exactly the
		// right grammar. Only a second opinion about the value finds it.
		"ignores the exponent": func(s string) (int64, error) {
			if i := strings.IndexAny(s, "eE"); i >= 0 {
				if v, err := parseMantissa(s); err != nil {
					return v, err // keep the real classification
				}
				return parseMantissa(s[:i])
			}
			return parseMantissa(s)
		},
	}

	for name, parse := range controls {
		t.Run(name, func(t *testing.T) {
			for _, s := range invariantSweep {
				if diff := checkParseInvariants(parse, s); diff != "" {
					t.Logf("rejected at %q: %s", s, diff)
					return
				}
			}
			t.Errorf("the invariants accepted a known-wrong parser (%s); "+
				"they cannot be distinguishing anything", name)
		})
	}

	// The positive control. Without it, a predicate that objects to everything
	// would read as a flawless set of negative controls.
	for _, s := range invariantSweep {
		if diff := checkParseInvariants(parseMantissa, s); diff != "" {
			t.Fatalf("the real parseMantissa was rejected by its own "+
				"invariants at %q: %s", s, diff)
		}
	}
}
