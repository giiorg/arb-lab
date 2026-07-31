package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// QuantilePoint is one measured point of the adverse-drift distribution: at
// quantile Q, the drift relative to the expected rate is Shift.
type QuantilePoint struct {
	// Q is the quantile this point was measured at, in (0,1).
	Q fixedpoint.Rate

	// Shift is the drift at Q, relative to the expected rate — signed, so the
	// lower tail is typically negative for a funding receiver.
	Shift fixedpoint.Rate
}

// Dispersion is §2.4's per-symbol adverse-drift distribution (C3), as the
// decision path consumes it: measured quantile points plus the identity of
// the calibration artifact that measured them.
//
// It is a class-2 value (§40e): the points are measurements, so the type
// refuses to validate without the artifact identity — a dispersion with no
// artifact is a dispersion somebody made up, and its zero-width degenerate
// (every shift zero, adverse == expected) is the exact flattery the class-2
// rule exists to refuse. Zero-width is still representable WITH an artifact:
// a measured zero is a declaration, not a default.
type Dispersion struct {
	// Points are the measured quantile points, strictly ascending in Q, with
	// Shift non-decreasing — a quantile function is monotone.
	Points []QuantilePoint

	// Artifact identifies the calibration artifact the points came from.
	Artifact CalibrationRef
}

// Validate reports whether the points describe a measured distribution.
func (d Dispersion) Validate() error {
	if err := d.Artifact.Validate(); err != nil {
		return fmt.Errorf("dispersion: %w", err)
	}
	if len(d.Points) == 0 {
		return fmt.Errorf("dispersion points: %w", ErrUnset)
	}
	for n, p := range d.Points {
		if err := validateQuantile(p.Q); err != nil {
			return fmt.Errorf("dispersion point %d quantile: %w", n, err)
		}
		if err := p.Shift.Validate(); err != nil {
			return fmt.Errorf("dispersion point %d shift: %w", n, err)
		}
		if n == 0 {
			continue
		}
		qc, err := d.Points[n-1].Q.Cmp(p.Q)
		if err != nil {
			return fmt.Errorf("dispersion points %d,%d: %w", n-1, n, err)
		}
		if qc >= 0 {
			return fmt.Errorf("dispersion point %d quantile %s, %d quantile %s: %w",
				n-1, d.Points[n-1].Q, n, p.Q, ErrDispersionShape)
		}
		sc, err := d.Points[n-1].Shift.Cmp(p.Shift)
		if err != nil {
			return fmt.Errorf("dispersion points %d,%d: %w", n-1, n, err)
		}
		if sc > 0 {
			return fmt.Errorf("dispersion point %d shift %s above %d shift %s: %w",
				n-1, d.Points[n-1].Shift, n, p.Shift, ErrDispersionShape)
		}
	}
	return nil
}

// ShiftAt returns the drift at quantile q.
//
// # Selection rounds OUTWARD from the median, and that is the conservative
// direction for either tail
//
// The points are a discrete sample of a continuous quantile function, so a q
// between two points has no measured value. Interpolating would invent one;
// picking the nearest would flatter half the time. The rule instead keys on
// which tail the caller is drawing: q below the median takes the LOWER
// bracketing point (a more extreme lower-tail draw), q at or above the median
// takes the UPPER one (a more extreme upper-tail draw). Both directions
// overstate adversity, never understate it — and the asymmetry is what makes
// p25 and p75 draws distinguishable in a test, which is the checklist's
// symmetric-fixture suspect answered by construction.
//
// A q outside the measured span is refused (ErrQuantileUnsupported): the
// nearest measured point would UNDERSTATE a tail the artifact never reached,
// which is the flattering direction.
func (d Dispersion) ShiftAt(q fixedpoint.Rate) (fixedpoint.Rate, error) {
	// Re-validated per call: the zero value must be unusable, and a method on
	// a struct nobody constructed is exactly where it would otherwise leak in.
	if err := d.Validate(); err != nil {
		return fixedpoint.Rate{}, err
	}
	if err := validateQuantile(q); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("shift at: %w", err)
	}
	first, last := d.Points[0], d.Points[len(d.Points)-1]
	if c, err := q.Cmp(first.Q); err != nil {
		return fixedpoint.Rate{}, err
	} else if c < 0 {
		return fixedpoint.Rate{}, fmt.Errorf("quantile %s below measured span [%s, %s]: %w",
			q, first.Q, last.Q, ErrQuantileUnsupported)
	}
	if c, err := q.Cmp(last.Q); err != nil {
		return fixedpoint.Rate{}, err
	} else if c > 0 {
		return fixedpoint.Rate{}, fmt.Errorf("quantile %s above measured span [%s, %s]: %w",
			q, first.Q, last.Q, ErrQuantileUnsupported)
	}

	// Find the bracketing pair. The span check above guarantees one exists.
	for n, p := range d.Points {
		c, err := q.Cmp(p.Q)
		if err != nil {
			return fixedpoint.Rate{}, err
		}
		if c == 0 {
			return p.Shift, nil
		}
		if c < 0 {
			// Points[n-1].Q < q < Points[n].Q — n >= 1, per the span check.
			lower, upper := d.Points[n-1], p
			tail, err := q.Cmp(half())
			if err != nil {
				return fixedpoint.Rate{}, err
			}
			if tail < 0 {
				return lower.Shift, nil
			}
			return upper.Shift, nil
		}
	}
	// Unreachable: q <= last.Q, so the loop returns at or before the final
	// point. Kept as a refusal rather than a panic because "unreachable given
	// the checks above" is a property of the checks above, and they move.
	return fixedpoint.Rate{}, fmt.Errorf("quantile %s: %w", q, ErrQuantileUnsupported)
}

// half is Rate(0.5), minted per call for the same reason as one(): a
// programmer constant whose parse cannot fail.
func half() fixedpoint.Rate {
	r, err := fixedpoint.ParseRate("0.5")
	if err != nil {
		panic(err)
	}
	return r
}
