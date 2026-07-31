package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// ExecTactic names the execution pattern a cost estimate was priced for.
type ExecTactic string

const (
	// TacticTakerIOC is §2.4's default: parallel marketable-IOC on both legs.
	TacticTakerIOC ExecTactic = "taker_ioc"

	// TacticMakerEntry is the documented OPT-IN (§2.4/§12.2a): maker spot
	// entry + taker perp hedge + a fill-timeout fallback that reverts to
	// taker. Never the silent default — it trades against §6.6's leg-risk
	// mandate, and choosing it is a recorded decision.
	TacticMakerEntry ExecTactic = "maker_entry"
)

// Validate reports whether t is one of the declared tactics.
func (t ExecTactic) Validate() error {
	switch t {
	case TacticTakerIOC, TacticMakerEntry:
		return nil
	}
	return fmt.Errorf("%q: %w", string(t), ErrKindInvalid)
}

func (t ExecTactic) String() string { return string(t) }

// ExecutionCostRequest asks the ExecutionCostModel port (§40e) to price a
// candidate execution. The adapter sources its own book state — book plumbing
// stays out of this context — so the request names WHAT to price, never the
// book to price it against.
type ExecutionCostRequest struct {
	// Spot and Perp are the legs to price.
	Spot, Perp ids.VenueInstrument

	// Qty is the candidate quantity the sizing is being scored at —
	// explicit, never inferred back out of C_allocated (§40b).
	Qty fixedpoint.Quantity

	// Tactic is the execution pattern to price.
	Tactic ExecTactic

	// Latency is the model's assumed decision-to-venue latency.
	Latency time.Duration
}

// Validate reports whether the request prices a real execution.
func (r ExecutionCostRequest) Validate() error {
	if err := r.Spot.Validate(); err != nil {
		return fmt.Errorf("spot: %w", err)
	}
	if r.Spot.Product != ids.ProductSpot {
		return fmt.Errorf("spot leg is %s: %w", r.Spot, ErrProductWrong)
	}
	if err := r.Perp.Validate(); err != nil {
		return fmt.Errorf("perp: %w", err)
	}
	if !r.Perp.Product.IsPerp() {
		return fmt.Errorf("perp leg is %s: %w", r.Perp, ErrProductWrong)
	}
	if err := r.Qty.Validate(); err != nil {
		return fmt.Errorf("qty: %w", err)
	}
	if err := r.Tactic.Validate(); err != nil {
		return fmt.Errorf("tactic: %w", err)
	}
	if r.Latency <= 0 {
		return fmt.Errorf("latency %v: %w", r.Latency, ErrUnset)
	}
	return nil
}

// ExecutionCostEstimate is the port's answer: the EXPECTED per-leg execution
// cost for the candidate quantity, with the identity of the model and
// calibration that produced it.
//
// It is never the simulator's realized fill (§40e): one is the decision's
// input, the other is the run's output, and conflating them lets the sim
// grade its own homework.
type ExecutionCostEstimate struct {
	// SpotCost and PerpCost are the expected per-leg charges, in the quote
	// asset. A declared zero is admissible per leg (a maker leg modelled at
	// zero impact); absence is not.
	SpotCost, PerpCost fixedpoint.Money

	// Model names the cost model (§8's conservative bound at $1K; the full
	// adverse-selection model at $10K+).
	Model string

	// Calibration identifies the artifact the model's numbers came from.
	// Class 2 — the estimate refuses to exist without it.
	Calibration CalibrationRef

	// Tactic and Qty echo what was priced, so the estimate cannot be applied
	// to a different candidate than it was produced for.
	Tactic ExecTactic
	Qty    fixedpoint.Quantity
}

// Validate reports whether the estimate is complete.
func (e ExecutionCostEstimate) Validate() error {
	if err := e.SpotCost.Validate(); err != nil {
		return fmt.Errorf("spot cost: %w", err)
	}
	if err := e.PerpCost.Validate(); err != nil {
		return fmt.Errorf("perp cost: %w", err)
	}
	if e.Model == "" {
		return fmt.Errorf("model: %w", ErrUnset)
	}
	if err := e.Calibration.Validate(); err != nil {
		return fmt.Errorf("calibration: %w", err)
	}
	if err := e.Tactic.Validate(); err != nil {
		return fmt.Errorf("tactic: %w", err)
	}
	if err := e.Qty.Validate(); err != nil {
		return fmt.Errorf("qty: %w", err)
	}
	return nil
}

// BasisTerm is the BasisForecast port's answer (§2.4): what the basis
// contributes over the hold. The conservative default adapter answers a
// declared-zero favorable leg and the measured adverse move — "never an
// optimistic basis credit".
type BasisTerm struct {
	// FavorableConvergence is expected favorable basis movement. The default
	// is a DECLARED zero — fixedpoint's asset tag is the declaration.
	FavorableConvergence fixedpoint.Money

	// AdverseMove is the measured adverse basis move charged against the
	// hold. Class 2: its magnitude is a measurement (§40e).
	AdverseMove MeasuredMoney
}

// Validate reports whether the term is complete.
func (b BasisTerm) Validate() error {
	if err := b.FavorableConvergence.Validate(); err != nil {
		return fmt.Errorf("favorable convergence: %w", err)
	}
	if err := b.AdverseMove.Validate(); err != nil {
		return fmt.Errorf("adverse move: %w", err)
	}
	return nil
}
