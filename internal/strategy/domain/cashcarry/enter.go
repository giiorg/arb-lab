package cashcarry

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Enter scores §2.4's entry inequality over one validated entry input:
//
//	Q_p[ Σ funding + basis − round-trip − slippage ] − reserve
//	    ≥ hurdle · C_allocated · holdFraction  +  margin
//
// where the margin is §2.2's "clears (round-trip cost + hurdle) WITH MARGIN
// [→ C3]": the measured EntryDrift rate scaled exactly as the hurdle is —
// annualized rate · C_allocated · holdFraction. The quantile is taken under
// the stated joint rule (Aggregate); every term is Money over the HoldPlan in
// one unit, and no rate is ever multiplied by hours.
//
// The verdict order is refusals first (policy floors, the coherent-draw rule,
// quantization, capital), then the scored comparison. A refusal journals with
// its rule named and prices nothing.
func Enter(in domain.DecisionInput) (domain.Decision, error) {
	if err := in.Validate(); err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}
	if in.Kind != domain.KindEntry {
		return domain.Decision{}, fmt.Errorf("enter over %s input: %w", in.Kind, ErrKindWrong)
	}
	pol := in.Policy.Params

	// §5.2 min_funding_apr: the annualized live estimate must clear the
	// policy floor, or the hold is not evaluated further.
	apr, err := annualized(in.View.Funding.Rate, in.View.Funding.Interval)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}
	if c, err := apr.Cmp(pol.MinFundingAPR); err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	} else if c < 0 {
		return refusal(in, RefuseMinFundingAPR,
			fmt.Sprintf("live %s under floor %s", apr, pol.MinFundingAPR)), nil
	}

	// §5.2 min_hold_settlements: the proposed plan must commit to at least
	// the policy minimum.
	if uint32(len(in.Plan.Plan.Settlements)) < pol.MinHoldSettlements {
		return refusal(in, RefuseMinHold,
			fmt.Sprintf("%d planned, %d required", len(in.Plan.Plan.Settlements), pol.MinHoldSettlements)), nil
	}

	// The forecasts must be one coherent draw at the policy's quantile.
	if err := checkDraw(in.Forecasts, pol.DecisionQuantile); err != nil {
		return refusal(in, RefuseForecastDraw, err.Error()), nil
	}
	agg, err := domain.Aggregate(pol.Aggregation, in.Forecasts)
	if err != nil {
		return refusal(in, RefuseForecastDraw, err.Error()), nil
	}

	// §2.4 leg quantization, refusals by name.
	target, err := TargetQty(in)
	if err != nil {
		switch {
		case errors.Is(err, ErrLotIncompatible):
			return refusal(in, RefuseLots, err.Error()), nil
		case errors.Is(err, ErrResidualExceeded):
			return refusal(in, RefuseResidual, err.Error()), nil
		case errors.Is(err, ErrBelowMinQty):
			return refusal(in, RefuseMinQty, err.Error()), nil
		case errors.Is(err, ErrBelowMinNotional):
			return refusal(in, RefuseMinNotional, err.Error()), nil
		}
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}

	// The declared capital must fund the target: the spot leg inside the
	// spot principal, the perp leg inside initial margin at the pinned
	// leverage. Ask-priced, away-from-zero — the overstated requirement.
	if d, err := capitalShortfall(in, target); err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	} else if d != "" {
		return refusal(in, RefuseCapital, d), nil
	}

	// The ledger. Left-hand side first: funding at the quantile, basis,
	// minus the four fee cells, the round-trip slippage and the reserve.
	funding, err := fundingMoney(agg.Adverse, in.View.Perp.Bid, target)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}
	basis, err := basisNet(in.Basis)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}

	// The four fee cells. The entry spot leg follows the priced tactic —
	// §2.4's documented maker-entry opt-in routes exactly one leg maker, the
	// spot entry — and every other cell is taker: the perp hedge is
	// time-critical by §6.6's mandate, and the exit legs' tactic is not
	// entry's to promise.
	spotEntryRate := in.SpotFees.Taker
	if in.ExecCost.Tactic == domain.TacticMakerEntry {
		spotEntryRate = in.SpotFees.Maker
	}
	spotEntry, err := feeCell(spotEntryRate, in.View.Spot.Ask, target)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter spot entry fee: %w", err)
	}
	spotExit, err := feeCell(in.SpotFees.Taker, in.View.Spot.Ask, target)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter spot exit fee: %w", err)
	}
	perpEntry, err := feeCell(in.PerpFees.Taker, in.View.Perp.Ask, target)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter perp entry fee: %w", err)
	}
	perpExit, err := feeCell(in.PerpFees.Taker, in.View.Perp.Ask, target)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter perp exit fee: %w", err)
	}

	// Slippage covers the round trip: the estimate prices ONE execution of
	// the candidate, entry and exit are two, and the only book the entry
	// decision holds is today's — so the exit execution is charged at the
	// entry's estimate, doubled. Overstates nothing the model can know
	// better; understates only if exiting costs more than entering, which is
	// what the exit band's width exists to absorb.
	slippage, err := fixedpoint.Chain(in.ExecCost.SpotCost).
		Add(in.ExecCost.PerpCost).
		Add(in.ExecCost.SpotCost).
		Add(in.ExecCost.PerpCost).
		Result()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter slippage: %w", err)
	}

	lhs, err := fixedpoint.Chain(funding).
		Add(basis).
		Sub(spotEntry).Sub(spotExit).Sub(perpEntry).Sub(perpExit).
		Sub(slippage).
		Sub(in.Opportunity).
		Result()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter lhs: %w", err)
	}

	// Right-hand side: the scaled hurdle and the C3 margin, both
	// annualized-rate · C_allocated · holdFraction.
	capital, err := in.Capital.Total()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter capital: %w", err)
	}
	fraction, err := holdFraction(in.At, in.Plan.Plan.Terminal)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}
	hurdleRate, err := effectiveHurdle(in.Hurdle)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter: %w", err)
	}
	scaledHurdle, err := scaledThreshold(capital, hurdleRate, fraction)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter hurdle: %w", err)
	}
	margin, err := scaledThreshold(capital, pol.EntryDrift.Value, fraction)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter margin: %w", err)
	}

	// The verdict: lhs − scaledHurdle ≥ margin.
	score, err := lhs.Sub(scaledHurdle)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter score: %w", err)
	}
	c, err := score.Cmp(margin)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("enter verdict: %w", err)
	}
	verdict := domain.VerdictDecline
	if c >= 0 {
		verdict = domain.VerdictEnter
	}

	d := domain.Decision{
		Kind:    domain.KindEntry,
		Verdict: verdict,
		At:      in.At,
		Input:   in,
		Terms: domain.DecisionTerms{
			FundingQuantile:  funding,
			Basis:            basis,
			SpotEntryFee:     spotEntry,
			SpotExitFee:      spotExit,
			PerpEntryFee:     perpEntry,
			PerpExitFee:      perpExit,
			Slippage:         slippage,
			Opportunity:      in.Opportunity,
			ClosingCostDelta: in.ClosingCostDelta,
			RiskDelta:        in.RiskDelta,
			ScaledHurdle:     scaledHurdle,
			HoldFraction:     fraction,
			EntryMargin:      pol.EntryDrift,
		},
	}
	if err := d.Validate(); err != nil {
		return domain.Decision{}, fmt.Errorf("enter decision: %w", err)
	}
	return d, nil
}

// capitalShortfall names the first leg the declared C_allocated decomposition
// cannot fund at the target, or "" when both fit. The requirement is
// overstated (ask-priced, away-from-zero) because an entry the capital only
// just affords at the mid is an entry a real fill can leave unfunded.
func capitalShortfall(in domain.DecisionInput, target fixedpoint.Quantity) (string, error) {
	spotNeed, err := fixedpoint.Notional(in.View.Spot.Ask, target, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return "", fmt.Errorf("capital spot: %w", err)
	}
	if c, err := spotNeed.Cmp(in.Capital.SpotPrincipal); err != nil {
		return "", fmt.Errorf("capital spot: %w", err)
	} else if c > 0 {
		return fmt.Sprintf("spot needs %s, principal %s", spotNeed, in.Capital.SpotPrincipal), nil
	}

	perpNotional, err := fixedpoint.Notional(in.View.Perp.Ask, target, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return "", fmt.Errorf("capital perp: %w", err)
	}
	supported, err := in.Capital.PerpInitialMargin.Mul(in.Policy.Params.PerpLeverage, fixedpoint.RoundTowardZero)
	if err != nil {
		return "", fmt.Errorf("capital perp: %w", err)
	}
	if c, err := perpNotional.Cmp(supported); err != nil {
		return "", fmt.Errorf("capital perp: %w", err)
	} else if c > 0 {
		return fmt.Sprintf("perp needs %s, margin supports %s", perpNotional, supported), nil
	}
	return "", nil
}
