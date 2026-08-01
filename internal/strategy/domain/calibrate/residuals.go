package calibrate

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Residuals collects the EWMA part of the dispersion distribution (§41b
// family 1): the deployed trailing estimator run CAUSALLY over the
// calibration era's settlement history, each residual the realized rate
// minus the forecast the estimator would have held going into that
// settlement. The forecast at index i reads strictly history[:i], so every
// residual is out-of-sample by construction — no settlement ever grades a
// forecast it participated in.
//
// # The warmup is the EWMA's own saturation horizon, not a tunable
//
// A residual is collected only once the trailing window already spans
// domain.EWMAHorizon(halfLife) — the span past which older settlements
// contribute exactly nothing at the canonical scale. Every collected
// residual therefore sees the same effective window shape as the live
// steady state; residuals from a still-filling window would grade a
// different estimator than the one deployed. The rule is derived from the
// EWMA's own constants, so the governance freezes no extra number for it.
//
// # The forecast is the UNCLAMPED trailing EWMA, and that is a stated
// reading
//
// The deployed estimator clamps its forecast to the venue's LIVE cap and
// floor, but the refetchable settlement history carries no cap/floor
// series to replay — and the clamp only ever moves a forecast toward the
// realizable range the settlement itself was clamped into, so an
// unclamped residual is never smaller in magnitude than the deployed
// one. Measuring unclamped overstates dispersion, never understates it —
// the conservative direction, recorded for the slice C close's reading
// pass.
func Residuals(history []domain.RealizedSettlement, halfLife time.Duration) ([]fixedpoint.Rate, error) {
	if halfLife <= 0 {
		return nil, fmt.Errorf("residuals half-life %v: %w", halfLife, domain.ErrUnset)
	}
	// The whole history is judged once, up front. EWMA re-checks each
	// window it is handed, but the LAST row joins no window — it only
	// ever grades one — so without this pass a trailing out-of-order or
	// foreign-instrument row would slip through exactly where the era
	// ends.
	if err := validateHistory(history); err != nil {
		return nil, fmt.Errorf("residuals: %w", err)
	}
	horizon := domain.EWMAHorizon(halfLife)

	var out []fixedpoint.Rate
	for i := 1; i < len(history); i++ {
		// The window is history[:i]; its newest entry is history[i-1]. The
		// window spans the horizon exactly when that newest entry sits at
		// least one horizon after the era's first settlement.
		if history[i-1].At.Sub(history[0].At) < horizon {
			continue
		}
		// EWMA validates the window itself (ascending, one instrument,
		// every row real) — no second authority here.
		forecast, err := domain.EWMA(history[:i], halfLife)
		if err != nil {
			return nil, fmt.Errorf("residuals window ending %s: %w", history[i-1].At.Format(time.RFC3339), err)
		}
		r, err := history[i].Rate.Sub(forecast)
		if err != nil {
			return nil, fmt.Errorf("residual at %s: %w", history[i].At.Format(time.RFC3339), err)
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%d rows, horizon %v: %w", len(history), horizon, ErrHistoryShort)
	}
	return out, nil
}
