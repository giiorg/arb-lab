package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// EWMAEstimator is §2.4's default funding estimator on §40e's
// explicit-materials signature: the imminent settlement takes the live
// per-settlement estimate; later settlements take the trailing
// realized-settlement EWMA; every forecast rate is clamped to the LIVE
// cap/floor from the materials, never a stored copy.
//
// It is pure domain math — the FundingForecast port wires it
// (infrastructure/driven holds the conformance assertion), and the seam it
// sits behind is earned by §2.4's own swappability requirement
// (forecast_model, hot-reloadable, out-of-sample validated), not by I/O.
//
// The dispersion draw applies to EVERY settlement, the imminent one
// included: the live estimate is §2.4's "evolving estimate", not a settled
// rate, and it keeps drifting until the settlement fires — exempting it
// would model the nearest lump, the one most of a short hold's carry sits
// in, as certain.
type EWMAEstimator struct {
	// halfLife is the EWMA's half-life (class 1 — §2.4's own "default ~3
	// days" rides config).
	halfLife time.Duration

	// quantile is the decision quantile every Adverse is drawn at (class 1 —
	// "default p25").
	quantile fixedpoint.Rate

	// dispersion is the measured adverse-drift distribution (class 2 —
	// refuses to exist without its artifact).
	dispersion Dispersion
}

// NewEWMAEstimator builds an estimator, refusing any member that is not a
// usable value — the zero value of this type is unusable, and Estimate
// re-checks, so a struct literal cannot smuggle an unvalidated one past the
// constructor.
func NewEWMAEstimator(halfLife time.Duration, quantile fixedpoint.Rate, dispersion Dispersion) (EWMAEstimator, error) {
	e := EWMAEstimator{halfLife: halfLife, quantile: quantile, dispersion: dispersion}
	if err := e.validate(); err != nil {
		return EWMAEstimator{}, err
	}
	return e, nil
}

func (e EWMAEstimator) validate() error {
	if e.halfLife <= 0 {
		return fmt.Errorf("estimator half-life %v: %w", e.halfLife, ErrUnset)
	}
	if err := validateQuantile(e.quantile); err != nil {
		return fmt.Errorf("estimator quantile: %w", err)
	}
	if err := e.dispersion.Validate(); err != nil {
		return fmt.Errorf("estimator: %w", err)
	}
	return nil
}

// Estimate implements the FundingForecast port contract: one forecast per
// horizon instant, in horizon order.
//
// The horizon is the plan's enumerated settlements — this function never
// invents instants. It refuses a horizon instant before the observation's
// NextSettle (ErrHorizonBehind: settled or unknown to the venue), requires
// strict ascent, and serves an empty horizon with an empty answer — a hold
// too short to reach a settlement captures nothing, Enumerate's own rule.
//
// A horizon reaching past the imminent settlement with an empty realized
// window is refused (ErrWindowEmpty): the imminent settlement needs only the
// live estimate, but anything later has no basis to average.
func (e EWMAEstimator) Estimate(m FundingMaterials, horizon []time.Time) ([]SettlementForecast, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("estimate: %w", err)
	}
	for n, t := range horizon {
		if t.IsZero() {
			return nil, fmt.Errorf("horizon %d: %w", n, ErrUnset)
		}
		if n > 0 && !horizon[n-1].Before(t) {
			return nil, fmt.Errorf("horizon %d at %s, %d at %s: %w",
				n-1, horizon[n-1], n, t, ErrNotAscending)
		}
		if t.Before(m.Observation.NextSettle) {
			return nil, fmt.Errorf("horizon %d at %s, next settlement %s: %w",
				n, t, m.Observation.NextSettle, ErrHorizonBehind)
		}
	}
	if len(horizon) == 0 {
		return nil, nil
	}

	shift, err := e.dispersion.ShiftAt(e.quantile)
	if err != nil {
		return nil, fmt.Errorf("estimate: %w", err)
	}

	// The EWMA is computed once, lazily: a horizon holding only the imminent
	// settlement must not fail on an empty window it never needed.
	var trailing fixedpoint.Rate
	haveTrailing := false

	out := make([]SettlementForecast, 0, len(horizon))
	for _, t := range horizon {
		var expected fixedpoint.Rate
		if t.Equal(m.Observation.NextSettle) {
			expected = m.Observation.Rate
		} else {
			if !haveTrailing {
				if len(m.Realized) == 0 {
					return nil, fmt.Errorf("estimate at %s: %w", t, ErrWindowEmpty)
				}
				if trailing, err = EWMA(m.Realized, e.halfLife); err != nil {
					return nil, fmt.Errorf("estimate: %w", err)
				}
				haveTrailing = true
			}
			expected = trailing
		}
		expected, err = clampRate(expected, m.Observation.Floor, m.Observation.Cap)
		if err != nil {
			return nil, fmt.Errorf("estimate at %s: %w", t, err)
		}
		adverse, err := expected.Add(shift)
		if err != nil {
			return nil, fmt.Errorf("estimate at %s: %w", t, err)
		}
		adverse, err = clampRate(adverse, m.Observation.Floor, m.Observation.Cap)
		if err != nil {
			return nil, fmt.Errorf("estimate at %s: %w", t, err)
		}
		out = append(out, SettlementForecast{
			At:         t,
			Expected:   expected,
			Adverse:    adverse,
			Quantile:   e.quantile,
			Dispersion: e.dispersion.Artifact,
		})
	}
	return out, nil
}

// clampRate bounds r to [floor, cap] — the venue's own clamp, applied to the
// forecast because the venue applies it to the settlement: no rate outside
// the clamps can ever settle, so a forecast outside them models an
// impossibility. Clamping an adverse draw UP to the floor is therefore
// accuracy, not flattery — the floor is where the venue itself stops the
// bleeding.
func clampRate(r, floor, cap fixedpoint.Rate) (fixedpoint.Rate, error) {
	if c, err := r.Cmp(cap); err != nil {
		return fixedpoint.Rate{}, err
	} else if c > 0 {
		return cap, nil
	}
	if c, err := r.Cmp(floor); err != nil {
		return fixedpoint.Rate{}, err
	} else if c < 0 {
		return floor, nil
	}
	return r, nil
}
