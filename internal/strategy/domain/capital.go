package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// CapitalAllocation is §2.2's C_allocated decomposition: "spot principal +
// perp initial margin + rescue margin + fee/slippage reserve + committed
// venue float — ALL capital at risk". The five terms are carried separately
// because the denominator error §2.4's worked example pins (C_legs' flattering
// 7.12%) is exactly a term quietly dropped from this sum, and a decomposition
// in the journal is what makes the drop visible.
type CapitalAllocation struct {
	// SpotPrincipal funds the long leg.
	SpotPrincipal fixedpoint.Money

	// PerpInitialMargin is posted against the short.
	PerpInitialMargin fixedpoint.Money

	// RescueMargin is §7.1's pre-funded top-up collateral. Class 2: the
	// protocol that COMPUTES it live is milestone 4, so the gate consumes a
	// declared, artifact-identified figure (§40g) — its zero value reads as
	// free liquidation insurance.
	RescueMargin MeasuredMoney

	// FeeSlippageReserve is the committed fee/slippage buffer. May be a
	// declared zero.
	FeeSlippageReserve fixedpoint.Money

	// CommittedFloat is venue float committed to the strategy. May be a
	// declared zero.
	CommittedFloat fixedpoint.Money
}

// Validate reports whether the decomposition is complete and summable: every
// term populated, all in one asset.
func (c CapitalAllocation) Validate() error {
	if err := c.SpotPrincipal.Validate(); err != nil {
		return fmt.Errorf("spot principal: %w", err)
	}
	if err := c.PerpInitialMargin.Validate(); err != nil {
		return fmt.Errorf("perp initial margin: %w", err)
	}
	if err := c.RescueMargin.Validate(); err != nil {
		return fmt.Errorf("rescue margin: %w", err)
	}
	if err := c.FeeSlippageReserve.Validate(); err != nil {
		return fmt.Errorf("fee/slippage reserve: %w", err)
	}
	if err := c.CommittedFloat.Validate(); err != nil {
		return fmt.Errorf("committed float: %w", err)
	}
	want := c.SpotPrincipal.Asset()
	for _, t := range []struct {
		name  string
		asset fixedpoint.Asset
	}{
		{"perp initial margin", c.PerpInitialMargin.Asset()},
		{"rescue margin", c.RescueMargin.Amount.Asset()},
		{"fee/slippage reserve", c.FeeSlippageReserve.Asset()},
		{"committed float", c.CommittedFloat.Asset()},
	} {
		if t.asset != want {
			return fmt.Errorf("%s in %s, spot principal in %s: %w", t.name, t.asset, want, ErrAssetMixed)
		}
	}
	return nil
}

// Total is C_allocated itself: the hurdle's denominator, never C_legs
// (§40f rule 3). Validate first; summing an unvalidated decomposition is how
// an absent term becomes a free one.
func (c CapitalAllocation) Total() (fixedpoint.Money, error) {
	if err := c.Validate(); err != nil {
		return fixedpoint.Money{}, err
	}
	return fixedpoint.Chain(c.SpotPrincipal).
		Add(c.PerpInitialMargin).
		Add(c.RescueMargin.Amount).
		Add(c.FeeSlippageReserve).
		Add(c.CommittedFloat).
		Result()
}
