package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// SettlementForecast is §2.4's per-settlement forecast contract:
// {ts, expRate, dispersion}, over the settlements enumerated in the planned
// hold.
//
// # How "dispersion" is represented — a stated reading
//
// §2.4's dispersion is "the per-symbol adverse-drift distribution (C3)". What
// a DECISION consumes from that distribution is one number: the rate at the
// stated adverse quantile, under the joint aggregation rule. So the value
// carries the expected rate, the adverse rate at the quantile it was drawn
// at, and the identity of the distribution both came from — rather than the
// distribution itself, whose machinery is the estimator's (slice B). The
// distribution's identity is a class-2 CalibrationRef: a dispersion with no
// artifact is a dispersion somebody made up, and its zero value (adverse ==
// expected) is the exact flattery §40e's rule exists to refuse.
type SettlementForecast struct {
	// At is the settlement instant this forecast is for — one of the plan's
	// enumerated settlements, never an interpolation.
	At time.Time

	// Expected is the expected per-settlement rate.
	Expected fixedpoint.Rate

	// Adverse is the rate at Quantile of the adverse-drift distribution. No
	// ordering against Expected is imposed here: which direction is adverse
	// belongs to the position's side, which this value does not know.
	Adverse fixedpoint.Rate

	// Quantile is the quantile Adverse was drawn at, in (0,1).
	Quantile fixedpoint.Rate

	// Dispersion identifies the measured distribution (C3) the draw came
	// from.
	Dispersion CalibrationRef
}

// Validate reports whether the forecast is complete.
func (f SettlementForecast) Validate() error {
	if f.At.IsZero() {
		return fmt.Errorf("at: %w", ErrUnset)
	}
	if err := f.Expected.Validate(); err != nil {
		return fmt.Errorf("expected: %w", err)
	}
	if err := f.Adverse.Validate(); err != nil {
		return fmt.Errorf("adverse: %w", err)
	}
	if err := validateQuantile(f.Quantile); err != nil {
		return fmt.Errorf("quantile: %w", err)
	}
	if err := f.Dispersion.Validate(); err != nil {
		return fmt.Errorf("dispersion: %w", err)
	}
	return nil
}

// validateQuantile checks a quantile lies strictly inside (0,1). Written once
// — SettlementForecast and StrategyParams both carry one, and two range
// checks are how one of them comes to admit 0, whose quantile draw is the
// distribution's infimum and reads as an infinitely conservative gate that
// never enters.
func validateQuantile(q fixedpoint.Rate) error {
	if err := q.Validate(); err != nil {
		return err
	}
	sign, err := q.Sign()
	if err != nil {
		return err
	}
	if sign <= 0 {
		return fmt.Errorf("%s: %w", q, ErrQuantileRange)
	}
	c, err := q.Cmp(one())
	if err != nil {
		return err
	}
	if c >= 0 {
		return fmt.Errorf("%s: %w", q, ErrQuantileRange)
	}
	return nil
}

// one is Rate(1), minted per call because Rate is immutable and cheap.
// Parse errors on a literal are impossible; the panic documents that this is
// a programmer constant, not data.
func one() fixedpoint.Rate {
	r, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err)
	}
	return r
}
