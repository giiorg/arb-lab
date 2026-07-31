package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// AggregateOutcome is the joint rule's answer over a hold's forecasts: the
// summed expected rate and the summed rate under the stated adverse
// scenario. Both are per-settlement-rate SUMS — dimensionless, converted to
// Money by the core against the position's notional, once, so the unit
// discipline stays in one place.
type AggregateOutcome struct {
	Expected fixedpoint.Rate
	Adverse  fixedpoint.Rate
}

// Aggregate applies §2.4's stated joint rule to a hold's per-settlement
// forecasts. The quantile of a sum is NOT the sum of marginal quantiles, so
// the rule is named, never implied — and the conservative default is ONE
// adverse scenario: every settlement at its per-settlement adverse quantile
// simultaneously, funding co-moving against the position, never independent
// draws.
//
// The forecasts must be a coherent draw: strictly ascending instants, one
// quantile (ErrQuantileMixed — a sum mixing p25 and p10 draws is a scenario
// nobody stated), one dispersion artifact (ErrArtifactMixed — §40e's
// one-artifact rule, checkable here because the identity rides every
// forecast). An empty slice is refused: the no-settlement case is the
// caller's to handle before asking for a scenario over nothing.
func Aggregate(rule Aggregation, fs []SettlementForecast) (AggregateOutcome, error) {
	if len(fs) == 0 {
		return AggregateOutcome{}, fmt.Errorf("aggregate forecasts: %w", ErrUnset)
	}
	for n, f := range fs {
		if err := f.Validate(); err != nil {
			return AggregateOutcome{}, fmt.Errorf("aggregate forecast %d: %w", n, err)
		}
		if n == 0 {
			continue
		}
		if !fs[n-1].At.Before(f.At) {
			return AggregateOutcome{}, fmt.Errorf("aggregate forecast %d at %s, %d at %s: %w",
				n-1, fs[n-1].At, n, f.At, ErrNotAscending)
		}
		if c, err := fs[n-1].Quantile.Cmp(f.Quantile); err != nil {
			return AggregateOutcome{}, err
		} else if c != 0 {
			return AggregateOutcome{}, fmt.Errorf("aggregate forecast %d at quantile %s, %d at %s: %w",
				n-1, fs[n-1].Quantile, n, f.Quantile, ErrQuantileMixed)
		}
		if fs[n-1].Dispersion != f.Dispersion {
			return AggregateOutcome{}, fmt.Errorf("aggregate forecast %d from %q, %d from %q: %w",
				n-1, fs[n-1].Dispersion.ID, n, f.Dispersion.ID, ErrArtifactMixed)
		}
	}

	switch rule {
	case AggregationConservative:
		expected := fixedpoint.ZeroRate()
		adverse := fixedpoint.ZeroRate()
		var err error
		for _, f := range fs {
			if expected, err = expected.Add(f.Expected); err != nil {
				return AggregateOutcome{}, fmt.Errorf("aggregate expected: %w", err)
			}
			if adverse, err = adverse.Add(f.Adverse); err != nil {
				return AggregateOutcome{}, fmt.Errorf("aggregate adverse: %w", err)
			}
		}
		return AggregateOutcome{Expected: expected, Adverse: adverse}, nil
	}
	// This return IS the refusal path for an undeclared rule, not a
	// default hiding a group: the switch names every member of Aggregation
	// (the exhaustiveswitch gate holds it to that), so only a value outside
	// the declared set lands here.
	return AggregateOutcome{}, fmt.Errorf("aggregation %q: %w", string(rule), ErrKindInvalid)
}
