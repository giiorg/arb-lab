package cashcarry

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Continue scores §2.4's incremental continue-vs-close rule over one
// validated exit input:
//
//	score = Q_p[ remaining funding + basis-from-here ]
//	        − opportunity − Δ(closing cost) − Δrisk
//	        − hurdle · C_allocated · remainingFraction
//
// Entry costs are SUNK: no fee cell and no slippage is re-subtracted here —
// the two wrong shapes §40f rule 2 names (re-subtracting the round trip;
// charging the close to continue alone) both bias toward premature exit, and
// both are aimed mutations. Cost terms enter only where the branches differ:
// the Δ members, declared zeros in the branch-symmetric common case.
//
// Stand-down needs both halves of the hysteresis: the score below
// −(exit band · C_allocated · remainingFraction), and the dwell — realized
// settlements observed with the score below, never calls — at or past
// exit_persist. A hold whose remaining schedule is empty closes as complete:
// there is nothing left to earn and the dwell exists to prevent whipsaw
// exits mid-plan, not to hold a finished plan open.
func Continue(in domain.DecisionInput) (domain.Decision, error) {
	if err := in.Validate(); err != nil {
		return domain.Decision{}, fmt.Errorf("continue: %w", err)
	}
	if in.Kind != domain.KindExit {
		return domain.Decision{}, fmt.Errorf("continue over %s input: %w", in.Kind, ErrKindWrong)
	}
	pol := in.Policy.Params

	capital, err := in.Capital.Total()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue capital: %w", err)
	}
	quote := capital.Asset()

	// Plan complete: no forecastable settlement remains. Close, with a
	// zero ledger — the honest value for arithmetic that has nothing left to
	// price — and the dwell passed through unchanged.
	if len(in.Forecasts) == 0 {
		return closeDecision(in, zeroExitTerms(in, quote, pol.ExitBand, in.Dwell))
	}

	if err := checkDraw(in.Forecasts, pol.DecisionQuantile); err != nil {
		return refusal(in, RefuseForecastDraw, err.Error()), nil
	}
	agg, err := domain.Aggregate(pol.Aggregation, in.Forecasts)
	if err != nil {
		return refusal(in, RefuseForecastDraw, err.Error()), nil
	}

	funding, err := fundingMoney(agg.Adverse, in.View.Perp.Bid, in.CandidateQty)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue: %w", err)
	}
	basis, err := basisNet(in.Basis)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue: %w", err)
	}
	fraction, err := holdFraction(in.At, in.Plan.Plan.Terminal)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue: %w", err)
	}
	hurdleRate, err := EffectiveHurdle(in.Hurdle)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue: %w", err)
	}
	scaledHurdle, err := scaledThreshold(capital, hurdleRate, fraction)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue hurdle: %w", err)
	}

	score, err := fixedpoint.Chain(funding).
		Add(basis).
		Sub(in.Opportunity).
		Sub(in.ClosingCostDelta).
		Sub(in.RiskDelta).
		Sub(scaledHurdle).
		Result()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue score: %w", err)
	}

	band, err := scaledThreshold(capital, pol.ExitBand.Value, fraction)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue band: %w", err)
	}
	floor, err := band.Neg()
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue band: %w", err)
	}
	c, err := score.Cmp(floor)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("continue verdict: %w", err)
	}
	below := c < 0

	dwell := advanceDwell(in.Dwell, below, newestRealized(in.View.Realized, in.At))

	terms := domain.DecisionTerms{
		FundingQuantile:  funding,
		Basis:            basis,
		SpotEntryFee:     fixedpoint.ZeroMoney(quote),
		SpotExitFee:      fixedpoint.ZeroMoney(quote),
		PerpEntryFee:     fixedpoint.ZeroMoney(quote),
		PerpExitFee:      fixedpoint.ZeroMoney(quote),
		Slippage:         fixedpoint.ZeroMoney(quote),
		Opportunity:      in.Opportunity,
		ClosingCostDelta: in.ClosingCostDelta,
		RiskDelta:        in.RiskDelta,
		ScaledHurdle:     scaledHurdle,
		HoldFraction:     fraction,
		Band:             pol.ExitBand,
		Dwell:            dwell,
	}

	if below && dwell.Count >= pol.ExitPersist {
		return closeDecision(in, terms)
	}
	d := domain.Decision{
		Kind:    domain.KindExit,
		Verdict: domain.VerdictContinue,
		At:      in.At,
		Input:   in,
		Terms:   terms,
	}
	if err := d.Validate(); err != nil {
		return domain.Decision{}, fmt.Errorf("continue decision: %w", err)
	}
	return d, nil
}

// advanceDwell is the §40f dwell rule. Above the band the dwell RESETS — the
// regime recovered, and "stays below" means consecutively. Below the band it
// advances by ONE exactly when a realized settlement not yet counted has been
// observed: settlements, never calls, and never settlements the engine did
// not see the score below at — an unobserved settlement carries no evidence,
// and crediting it would flatter the exit.
func advanceDwell(prior domain.DwellState, below bool, newest time.Time) domain.DwellState {
	if !below {
		return domain.DwellState{Declared: true}
	}
	if !newest.IsZero() && newest.After(prior.LastRealized) {
		return domain.DwellState{Declared: true, Count: prior.Count + 1, LastRealized: newest}
	}
	return prior
}

// newestRealized is the most recent realized settlement at or before the
// evaluation instant, zero when the window holds none. The window is
// validated ascending, so the newest eligible entry is the last one that is
// not after `at`.
func newestRealized(window []domain.RealizedSettlement, at time.Time) time.Time {
	var newest time.Time
	for _, r := range window {
		if r.At.After(at) {
			break
		}
		newest = r.At
	}
	return newest
}

// zeroExitTerms is the plan-complete ledger: every economic line a declared
// zero in the capital's asset, the band and the passed-through dwell carried
// so the record still shows what the close was judged under.
func zeroExitTerms(in domain.DecisionInput, quote fixedpoint.Asset, band domain.MeasuredRate, dwell domain.DwellState) domain.DecisionTerms {
	return domain.DecisionTerms{
		FundingQuantile:  fixedpoint.ZeroMoney(quote),
		Basis:            fixedpoint.ZeroMoney(quote),
		SpotEntryFee:     fixedpoint.ZeroMoney(quote),
		SpotExitFee:      fixedpoint.ZeroMoney(quote),
		PerpEntryFee:     fixedpoint.ZeroMoney(quote),
		PerpExitFee:      fixedpoint.ZeroMoney(quote),
		Slippage:         fixedpoint.ZeroMoney(quote),
		Opportunity:      in.Opportunity,
		ClosingCostDelta: in.ClosingCostDelta,
		RiskDelta:        in.RiskDelta,
		ScaledHurdle:     fixedpoint.ZeroMoney(quote),
		HoldFraction:     zeroFraction(),
		Band:             band,
		Dwell:            dwell,
	}
}

// zeroFraction is Rate(0) as a set value — the plan-complete hold has no
// remaining span, and an unset Rate would fail the terms validation that
// exists to catch members nobody populated.
func zeroFraction() fixedpoint.Rate {
	return fixedpoint.ZeroRate()
}

func closeDecision(in domain.DecisionInput, terms domain.DecisionTerms) (domain.Decision, error) {
	d := domain.Decision{
		Kind:    domain.KindExit,
		Verdict: domain.VerdictClose,
		At:      in.At,
		Input:   in,
		Terms:   terms,
	}
	if err := d.Validate(); err != nil {
		return domain.Decision{}, fmt.Errorf("close decision: %w", err)
	}
	return d, nil
}
