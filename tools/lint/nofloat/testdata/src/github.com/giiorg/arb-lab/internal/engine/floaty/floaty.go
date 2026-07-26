// Package floaty is the offending fixture. Each case below is a different door
// a float comes in through, and the last three are the ones that make this a
// type-based rule rather than a grep: no float token appears on their lines.
package floaty

import (
	"strconv"
	"time"
)

// The obvious case, and the only one a regex would catch.
var Ratio float64 // want `float64 value under internal/`

// A named type does not launder it. The rule checks Underlying(), so Basis is
// policed at its declaration AND at every use — otherwise one type definition
// is all it takes to leave the gate behind.
type Basis float64 // want `Basis value under internal/`

func Widen(b Basis) Basis { return b * 2 } // want `Basis value under internal/`

// An untyped constant becoming a float by inference. There is no type
// expression anywhere on this line for a regex to match.
var inferred = 1.5 // want `float64 value under internal/`

// Both operands float-typed — the case §3 names as beyond a regex. The
// signature is reported once, and the division that has no float token in it is
// reported on its own line.
func Div(a, b float64) float64 { // want `float64 value under internal/`
	return a / b // want `float64 value under internal/`
}

// The standard library's door, and the one that happens in practice: a duration
// turned into seconds to compute a funding rate. Not one character of these two
// lines says "float".
func Seconds(d time.Duration) int64 {
	s := d.Seconds() // want `float64 value under internal/`
	return int64(s)  // want `float64 value under internal/`
}

// Converting the float away afterwards does not help, and the line above proves
// the rule says so: the float existed, and whatever it lost is already lost.

// A multi-value call's type is a tuple, not a basic type. sink's two arguments
// come from one call, so the ONLY float-typed expression on that line is the
// tuple itself — which is how this exercises the tuple branch rather than
// merely passing through it.
func sink(float64, error) {} // want `float64 value under internal/`

func Feed(s string) {
	sink(strconv.ParseFloat(s, 64)) // want `float64 value under internal/`
}

// Complex counts: a pair of floats carries the same representation error, and
// excluding it costs one term rather than a second analyzer later.
var spin = complex(1, 2) // want `complex128 value under internal/`
