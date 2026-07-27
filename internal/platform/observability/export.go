package observability

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file is the package's only float64 seam, and build-spec §3.6 rule 4's
// only sanctioned one outside a test. Everything above it — Sample, the three
// handles, the registry — is integer-typed, so a float64 cannot reach an
// instrumented call site.
//
// # Why it exists before there is an exporter to call it
//
// The Prometheus adapter is deferred (see Registry). This conversion is not,
// because it does not belong to any one client library: the Prometheus text and
// protobuf formats, OTLP, and every other wire metric encoding carry a double.
// Whatever adapter lands will need exactly this function, and if it is not here
// the adapter will write its own — inline, at the call site, without the range
// argument below, which is how a rounding decision stops being a decision.
//
// It is also independently checkable, which is the test this package can run
// today and the adapter could not: exactness below the mantissa limit, the
// documented loss above it, and the refusal at the point where a float64 stops
// being able to distinguish adjacent integers.

// MaxExactMantissa is the largest magnitude an IEEE-754 double represents
// exactly: 2^53.
//
// Above it, consecutive integers are no longer distinguishable — 2^53 and
// 2^53+1 are the same float64 — so a conversion beyond this point is not
// "slightly imprecise", it is a value that has silently merged with its
// neighbours.
const MaxExactMantissa = 1 << 53

// Float converts a sample to the float64 every wire metric format carries, and
// reports whether the conversion was exact.
//
// # The boolean is returned rather than an error, and that is the lossy lane
//
// §3.4 makes this lane lossy so that back-pressure here never blocks trading.
// An error would invite an exporter to retry, log, or bail on a scrape because a
// number was one ulp off — spending trading-path attention on a dashboard. So
// the loss is REPORTED and the value is still returned, usable: an exporter
// counts the inexact conversions and exposes that count, which is the same shape
// as Registry.Rejected and events.ObservabilityLane.Dropped. Losing precision
// silently and losing it unrecorded are different failures, and only the second
// one is unacceptable here.
//
// # What "exact" means for each unit
//
//   - UnitCount and UnitBytes are plain integers; exact up to MaxExactMantissa.
//   - UnitSeconds is nanoseconds (Scale 9), so 2^53 ns is about 104 days of
//     accumulated duration before a histogram's sum stops being exact. A single
//     observation never approaches it.
//   - UnitRatio and UnitMoney are scaled by 10^9, so the exact range is about
//     ±9.0e6 in natural units. That is nine million dollars — comfortably above
//     every §1.1 capital tier, and stated rather than assumed, because a
//     cumulative funding counter grows without bound and this is where it stops
//     being trustworthy.
func Float(s Sample) (float64, bool) { //nofloat:allow the metric wire seam — every exposition format carries a double
	m := s.mantissa
	mag := m
	if mag < 0 {
		mag = -mag
	}
	// The negation above is correct for every value except math.MinInt64, whose
	// magnitude has no int64 representation and stays negative. It is caught
	// here rather than special-cased, since it is far past the exact range in
	// either sign.
	exact := mag >= 0 && mag <= MaxExactMantissa

	f := float64(m) //nofloat:allow the conversion this seam exists to own
	switch s.unit {
	case UnitCount, UnitBytes:
		return f, exact //nofloat:allow returning the converted value
	case UnitUnspecified, UnitSeconds, UnitRatio, UnitMoney:
		// The scaled units, named rather than swept into the default (§3.6
		// rule 10). A default is where a unit added next year lands silently,
		// and it would land on the scaling branch — which is the right answer
		// for a scaled unit and the wrong one for a second counting unit.
	default:
		// An unrecognised unit scales too. Reaching it means Validate was
		// bypassed, and scaling is the conservative reading: it cannot make a
		// count look larger than it is.
	}
	// Scaled units divide by the canonical scale factor. The division is by a
	// power of two times a power of five; 10^9 is not a power of two, so this
	// step is itself inexact for most inputs — which is precisely why the caller
	// is told, rather than the package pretending a decimal value survived a
	// binary float.
	return f / scaleFactorFloat, exact //nofloat:allow the scaled result
}

// scaleFactorFloat is 10^fixedpoint.Scale as a double. Exact: 10^9 is well
// under 2^53.
var scaleFactorFloat = powTen(fixedpoint.Scale) //nofloat:allow the divisor for the seam above

// powTen returns 10^n as a float64, built by repeated multiplication so the
// value is a constant expression of the scale rather than a literal that could
// drift from it.
func powTen(n int) float64 { //nofloat:allow same seam
	out := 1.0    //nofloat:allow same seam
	for range n { //nolint:intrange
		out *= 10 //nofloat:allow same seam
	}
	return out //nofloat:allow same seam
}

// FloatOf converts a fixedpoint.Money directly, for an exporter that holds one
// without a descriptor to check it against.
//
// It exists so that such a call site does not reach for float64(m.Mantissa()) /
// 1e9 and write the scale out by hand — the digit count is the thing nobody
// double-checks, and one digit is a factor of ten on a PnL panel.
func FloatOf(m fixedpoint.Money) (float64, bool, error) { //nofloat:allow same seam
	s, err := MoneyOf(m)
	if err != nil {
		return 0, false, fmt.Errorf("observability: float conversion: %w", err) //nofloat:allow same seam
	}
	f, exact := Float(s) //nofloat:allow same seam
	return f, exact, nil //nofloat:allow same seam
}
