package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Aggregation is §2.4's stated joint rule for taking the quantile of a sum —
// "the quantile of a sum is NOT the sum of marginal quantiles", so the rule
// is named, never implied.
type Aggregation string

const (
	// AggregationConservative is §2.4's default and currently only member:
	// one adverse scenario, every settlement at its per-settlement adverse
	// quantile, funding and basis co-moving against us — never independent
	// draws. The set is one member on purpose: a second rule arrives only
	// when a calibration artifact justifies it in writing (§40e), and
	// declaring it early would give the config a value nothing implements.
	AggregationConservative Aggregation = "conservative"
)

// Validate reports whether a is one of the declared rules.
func (a Aggregation) Validate() error {
	switch a {
	case AggregationConservative:
		return nil
	}
	return fmt.Errorf("%q: %w", string(a), ErrKindInvalid)
}

func (a Aggregation) String() string { return string(a) }

// StrategyParams is §5.2's typed per-strategy roster for cash-and-carry, in
// domain vocabulary. The config DOCUMENT that populates it (with its
// config:"hot" judgements) is infrastructure's; this is the validated value
// the projection snapshots.
//
// Class-1 members (spec-defaulted: the half-life, the quantile, the
// aggregation rule) are plain fields — their defaults are the CORE's own
// statements and live at the config layer. Class-2 members (the entry-drift
// multiplier, the exit band) are Measured: their values are step-4
// measurements and refuse to exist without the artifact's identity.
type StrategyParams struct {
	// MinFundingAPR is §5.2's min_funding_apr: the annualized funding floor
	// below which entry is not evaluated further.
	MinFundingAPR fixedpoint.Rate

	// MinHoldSettlements is §5.2's min_hold_settlements: the fewest planned
	// settlements a hold may be entered for.
	MinHoldSettlements uint32

	// ForecastModel names the estimator behind the FundingForecast port
	// (§2.4: a strategy param, hot-reloadable, out-of-sample validated).
	ForecastModel string

	// EWMAHalfLife is the trailing realized-settlement EWMA's half-life
	// (§2.4: "a strategy param, default ~3 days" — the default is config's).
	EWMAHalfLife time.Duration

	// DecisionQuantile is §2.4's decision quantile (default p25), in (0,1).
	DecisionQuantile fixedpoint.Rate

	// Aggregation is the joint rule above.
	Aggregation Aggregation

	// BasisModel names the estimator behind the BasisForecast port; the
	// conservative default is an ADAPTER, so "absent a configured estimator"
	// is a wiring state, not a nil check (§40a).
	BasisModel string

	// EntryDrift is §5.2's entry_drift_pct: the C3 adverse-drift margin the
	// entry threshold is derived from. Measured — "not a flat 2×" is the
	// spec's own words for why it has no default.
	EntryDrift MeasuredRate

	// ExitBand is the hysteresis band's width, "materially more adverse than
	// entry", from measured regime-persistence (§2.4). Measured.
	ExitBand MeasuredRate

	// ExitPersist is the dwell: how many realized settlements the forward
	// score must stay below the band before standing down.
	ExitPersist uint32

	// PerpLeverage is the perp leg's pinned leverage (§5.2).
	PerpLeverage fixedpoint.Rate
}

// Validate reports whether the roster is complete and each member is in its
// stated range.
func (p StrategyParams) Validate() error {
	if err := p.MinFundingAPR.Validate(); err != nil {
		return fmt.Errorf("min funding apr: %w", err)
	}
	if p.MinHoldSettlements == 0 {
		return fmt.Errorf("min hold settlements: %w", ErrUnset)
	}
	if p.ForecastModel == "" {
		return fmt.Errorf("forecast model: %w", ErrUnset)
	}
	if p.EWMAHalfLife <= 0 {
		return fmt.Errorf("ewma half-life %v: %w", p.EWMAHalfLife, ErrUnset)
	}
	if err := validateQuantile(p.DecisionQuantile); err != nil {
		return fmt.Errorf("decision quantile: %w", err)
	}
	if err := p.Aggregation.Validate(); err != nil {
		return fmt.Errorf("aggregation: %w", err)
	}
	if p.BasisModel == "" {
		return fmt.Errorf("basis model: %w", ErrUnset)
	}
	if err := p.EntryDrift.Validate(); err != nil {
		return fmt.Errorf("entry drift: %w", err)
	}
	if err := p.ExitBand.Validate(); err != nil {
		return fmt.Errorf("exit band: %w", err)
	}
	if p.ExitPersist == 0 {
		return fmt.Errorf("exit persist: %w", ErrUnset)
	}
	if err := p.PerpLeverage.Validate(); err != nil {
		return fmt.Errorf("perp leverage: %w", err)
	}
	sign, err := p.PerpLeverage.Sign()
	if err != nil {
		return fmt.Errorf("perp leverage: %w", err)
	}
	if sign <= 0 {
		return fmt.Errorf("perp leverage %s: %w", p.PerpLeverage, ErrUnset)
	}
	return nil
}

// PolicyProjection is §40b's policy carrier: the validated,
// revision-identified snapshot of every StrategyParams value the core reads.
//
// It exists because the round-3 review caught the roster carrying the dwell
// STATE while the band and dwell POLICY had no declared carrier — a core
// reading ambient params is not a function of its inputs. The revision is
// what the Decision journals, so "which policy produced this verdict" is
// answerable from the record alone.
type PolicyProjection struct {
	Params StrategyParams

	// Rev identifies the params document in force when the snapshot was
	// taken.
	Rev config.Revision
}

// Validate reports whether the projection is complete.
func (p PolicyProjection) Validate() error {
	if err := p.Params.Validate(); err != nil {
		return fmt.Errorf("params: %w", err)
	}
	if err := p.Rev.Validate(); err != nil {
		return fmt.Errorf("rev: %w", err)
	}
	return nil
}
