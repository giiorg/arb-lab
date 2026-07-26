package observability

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Unit names the dimension a metric measures.
//
// Integer enum rather than §3.6 rule 8's string shape, taking the same
// documented exception as fixedpoint.RoundingMode, apperror.Effect and
// events.Lane, and the deciding question is rule 8's own: does the value cross a
// persistence or wire boundary? It does not. What crosses to an exporter is the
// metric NAME, whose unit suffix this type is used to derive and check — the
// Unit itself is a construction-time argument that never leaves the process.
type Unit uint8

const (
	// UnitUnspecified is the zero value and names no dimension. Unusable, and
	// deliberately not aliased to UnitCount: "dimensionless" is a real
	// measurement claim, and defaulting to it would let a descriptor whose unit
	// nobody thought about pass every check this package makes.
	UnitUnspecified Unit = iota

	// UnitCount is a dimensionless tally: fills, reconnects, rejections,
	// dropped envelopes. Carries no name suffix of its own.
	UnitCount

	// UnitSeconds is elapsed time. §10's unhedged-exposure duration and every
	// §9 RTO budget are measured here.
	UnitSeconds

	// UnitBytes is a size in bytes.
	UnitBytes

	// UnitRatio is a dimensionless ratio in fixedpoint.Rate's convention —
	// plain ratios, never percentages, so an 0.01% funding rate is 0.0001.
	// §10's funding, fill rate and inventory drift live here.
	UnitRatio

	// UnitMoney is an amount denominated in a named asset. A descriptor using
	// it MUST name that asset, and a sample recorded against it MUST carry the
	// same one.
	UnitMoney
)

const maxUnit = UnitMoney

// Validate reports whether u is a declared unit, excluding the sentinel.
func (u Unit) Validate() error {
	if u == UnitUnspecified || u > maxUnit {
		return fmt.Errorf("unit %d: %w", uint8(u), ErrUnitInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (u Unit) String() string {
	switch u {
	case UnitUnspecified:
		return "UnitUnspecified"
	case UnitCount:
		return "UnitCount"
	case UnitSeconds:
		return "UnitSeconds"
	case UnitBytes:
		return "UnitBytes"
	case UnitRatio:
		return "UnitRatio"
	case UnitMoney:
		return "UnitMoney"
	}
	return fmt.Sprintf("Unit(%d)", uint8(u))
}

// suffix is the name ending a metric of this unit must carry, empty for a unit
// that contributes none. Money's suffix depends on the asset and is handled by
// Spec, which holds it.
func (u Unit) suffix() string {
	switch u {
	case UnitSeconds:
		return "_seconds"
	case UnitBytes:
		return "_bytes"
	case UnitRatio:
		return "_ratio"
	}
	return ""
}

// Sample is a value that has been converted into a metric's unit, carrying the
// unit it was converted into.
//
// # Why this is not a float64
//
// Every wire metric format — Prometheus text and protobuf, OTLP — carries a
// double, so a float64 has to appear somewhere. The question is only where. If
// it appears in the port, then Counter.Add takes a float64, and every call site
// reporting a fill count, a realized PnL or a funding rate has to produce one:
// build-spec §3.6 rule 4's single sanctioned metric seam becomes a
// //nofloat:allow at every instrumented line, each written by somebody thinking
// about a dashboard rather than about rounding. So the port is typed in what the
// domain actually holds — an int64 mantissa at fixedpoint's canonical scale —
// and the conversion lives alone in export.go.
//
// # Why the unit is carried rather than checked at the call site
//
// The defect this exists to prevent is not exotic. It is observing
// d.Milliseconds() into a histogram whose buckets are in seconds: it compiles,
// it produces plausible numbers, and it is wrong by three orders of magnitude on
// every panel and every alert threshold derived from it. Carrying the unit turns
// that into ErrUnitMismatch naming both units, at the first observation.
//
// The zero value is unusable. Construct with Count, Seconds, Bytes, Ratio or
// MoneyOf.
type Sample struct {
	mantissa int64
	unit     Unit
	asset    fixedpoint.Asset
}

// Count returns a dimensionless tally.
func Count(n int64) Sample {
	return Sample{mantissa: n, unit: UnitCount}
}

// Seconds returns d as a UnitSeconds sample.
//
// # The conversion is exact, and that is not luck to rely on silently
//
// time.Duration is an int64 count of NANOSECONDS, and fixedpoint.Scale is 9, so
// a value in seconds at the canonical scale is that value multiplied by 10^9 —
// the same number. The mantissa is therefore d itself, with no multiply, no
// division and no rounding mode to get wrong.
//
// That identity is pinned by the compile-time assertion below rather than left
// as a comment, because it is the kind of coincidence a later scale change would
// break silently: the code would keep compiling and every duration would be
// wrong by a factor of ten, in the same direction, on every panel — which reads
// as a system that got slower, not as a bug.
func Seconds(d time.Duration) Sample {
	return Sample{mantissa: int64(d), unit: UnitSeconds}
}

// Seconds is the identity only while fixedpoint.Scale is exactly 9, since
// time.Duration counts nanoseconds. These two declarations pin it from both
// sides: a uint constant expression that would be negative does not compile, so
// Scale < 9 breaks the first and Scale > 9 breaks the second.
//
// Written as a pair because either alone is a one-sided bound, and a one-sided
// bound on an equality is the sort of check that passes while the thing it
// guards is broken.
const (
	_ = uint(fixedpoint.Scale - 9)
	_ = uint(9 - fixedpoint.Scale)
)

// Bytes returns n as a UnitBytes sample.
func Bytes(n int64) Sample {
	return Sample{mantissa: n, unit: UnitBytes}
}

// Ratio returns r as a UnitRatio sample. An unconstructed Rate is refused: the
// zero Rate is fixedpoint's "nobody populated this", and admitting it would put
// a fee rate of zero on a panel that is used to decide whether trading is
// profitable.
func Ratio(r fixedpoint.Rate) (Sample, error) {
	if err := r.Validate(); err != nil {
		return Sample{}, fmt.Errorf("observability: ratio sample: %w", err)
	}
	return Sample{mantissa: r.Mantissa(), unit: UnitRatio}, nil
}

// MoneyOf returns m as a UnitMoney sample, carrying m's asset tag.
func MoneyOf(m fixedpoint.Money) (Sample, error) {
	if err := m.Validate(); err != nil {
		return Sample{}, fmt.Errorf("observability: money sample: %w", err)
	}
	return Sample{mantissa: m.Mantissa(), unit: UnitMoney, asset: m.Asset()}, nil
}

// Validate reports whether s was constructed.
func (s Sample) Validate() error {
	if s.unit == UnitUnspecified {
		return ErrSampleUnset
	}
	return s.unit.Validate()
}

// Unit reports the dimension s was converted into.
func (s Sample) Unit() Unit { return s.unit }

// Asset reports the denomination of a money sample, and the empty asset
// otherwise.
func (s Sample) Asset() fixedpoint.Asset { return s.asset }

// Mantissa reports the underlying integer at fixedpoint's canonical scale for
// dimensional units, and the plain value for UnitCount and UnitBytes.
//
// The asymmetry is deliberate and is the one place this type is not uniform. A
// count of 3 fills is 3, not 3_000_000_000: scaling a tally would spend 9 of
// int64's 19 digits on a fractional part a count cannot have, and would put the
// overflow ceiling at ~9.2e9 events — reachable by a market-data counter inside
// a year.
func (s Sample) Mantissa() int64 { return s.mantissa }

// Sign reports -1, 0 or +1. Used by Counter to refuse a decrease; exported
// because the same question is worth asking before choosing a counter at all.
func (s Sample) Sign() int {
	switch {
	case s.mantissa < 0:
		return -1
	case s.mantissa > 0:
		return 1
	}
	return 0
}

// String renders the sample for errors and logs.
func (s Sample) String() string {
	if s.unit == UnitMoney {
		return fmt.Sprintf("%d %s(%s)", s.mantissa, s.unit, s.asset)
	}
	return fmt.Sprintf("%d %s", s.mantissa, s.unit)
}

// matches reports whether s may be recorded against a metric of the given unit
// and asset, and says which dimension disagreed if not.
func (s Sample) matches(unit Unit, asset fixedpoint.Asset) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.unit != unit {
		return fmt.Errorf("sample is %s, metric is %s: %w", s.unit, unit, ErrUnitMismatch)
	}
	if unit == UnitMoney && s.asset != asset {
		return fmt.Errorf("sample is %s, metric is %s: %w", s.asset, asset, ErrAssetMismatch)
	}
	return nil
}
