package driven

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// ConservativeBasis is §2.4's default basis model: "assume zero favorable
// convergence and charge the measured adverse basis move — never an
// optimistic basis credit."
//
// The favorable leg is a DECLARED zero (the asset tag is the declaration —
// distinguishable from a zero nobody set), and the adverse leg is a class-2
// measurement: the adverse move rate is the step-4 artifact's number,
// carried as a MeasuredRate that refuses to exist without the artifact
// identity (§40e).
//
// # The charge does not scale with the plan's length, and that is the
// conservative reading
//
// The adverse basis move is a level-shift risk, not an accruing flow: the
// measured magnitude is what the basis can move against the position, and it
// can move that far in the first hour of a twenty-day hold as easily as in
// the last. Charging the full measured move regardless of remaining horizon
// overstates the risk of short holds and never understates a long one. The
// plan still participates in validation — a malformed plan refuses here, not
// downstream.
type ConservativeBasis struct {
	adverse domain.MeasuredRate
}

var _ ports.BasisForecast = ConservativeBasis{}

// NewConservativeBasis builds the model around the artifact's measured
// adverse move. A negative rate is refused: an "adverse move" that credits
// the position is a sign error in the artifact, not a conservative charge.
func NewConservativeBasis(adverse domain.MeasuredRate) (ConservativeBasis, error) {
	if err := adverse.Validate(); err != nil {
		return ConservativeBasis{}, fmt.Errorf("conservative basis: %w", err)
	}
	sign, err := adverse.Value.Sign()
	if err != nil {
		return ConservativeBasis{}, fmt.Errorf("conservative basis: %w", err)
	}
	if sign < 0 {
		return ConservativeBasis{}, fmt.Errorf("conservative basis adverse move %s: %w",
			adverse.Value, ErrChargeNegative)
	}
	return ConservativeBasis{adverse: adverse}, nil
}

// EstimateBasis implements the BasisForecast port: zero favorable
// convergence, and the measured adverse move charged against the candidate
// notional.
//
// The notional is priced at the spot ASK and rounded away from zero, and the
// charge rounds away from zero again — both are cost arithmetic, where the
// conservative direction is up (fixedpoint's own rule: a cost must never be
// understated, because the verdict is a comparison against a hurdle).
func (b ConservativeBasis) EstimateBasis(view domain.MarketView, plan domain.HoldPlanRevision, qty fixedpoint.Quantity) (domain.BasisTerm, error) {
	if b.adverse.Validate() != nil {
		// The zero value took a method call — reconstruct the refusal with
		// its name rather than pass judgement on a struct nobody built.
		return domain.BasisTerm{}, fmt.Errorf("conservative basis unconstructed: %w", domain.ErrUnset)
	}
	if err := view.Validate(); err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis view: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis plan: %w", err)
	}
	if err := qty.Validate(); err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis qty: %w", err)
	}
	sign, err := qty.Sign()
	if err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis qty: %w", err)
	}
	if sign <= 0 {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis qty %s: %w", qty, ErrQtyNotPositive)
	}

	notional, err := fixedpoint.Notional(view.Spot.Ask, qty, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis notional: %w", err)
	}
	charge, err := notional.Mul(b.adverse.Value, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis charge: %w", err)
	}

	term := domain.BasisTerm{
		FavorableConvergence: fixedpoint.ZeroMoney(view.Spot.Ask.Quote()),
		AdverseMove:          domain.MeasuredMoney{Amount: charge, Artifact: b.adverse.Artifact},
	}
	if err := term.Validate(); err != nil {
		return domain.BasisTerm{}, fmt.Errorf("estimate basis: %w", err)
	}
	return term, nil
}
