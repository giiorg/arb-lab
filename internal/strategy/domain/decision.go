package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Verdict is what a Decision concluded.
type Verdict string

const (
	// VerdictEnter and VerdictDecline are the entry direction's two scored
	// outcomes.
	VerdictEnter   Verdict = "enter"
	VerdictDecline Verdict = "decline"

	// VerdictContinue and VerdictClose are the exit direction's.
	VerdictContinue Verdict = "continue"
	VerdictClose    Verdict = "close"

	// VerdictRefuse is either direction declining to SCORE: stale view, skew
	// breach, unpopulated input, quantization failure. A refusal is a
	// journaled Decision naming its rule — a gate that only journals its
	// approvals cannot show it ever refused anything (§40f).
	VerdictRefuse Verdict = "refuse"
)

// Validate reports whether v is one of the declared verdicts.
func (v Verdict) Validate() error {
	switch v {
	case VerdictEnter, VerdictDecline, VerdictContinue, VerdictClose, VerdictRefuse:
		return nil
	}
	return fmt.Errorf("%q: %w", string(v), ErrKindInvalid)
}

func (v Verdict) String() string { return string(v) }

// allowedFor reports whether the verdict can arise from the kind. Refusal is
// reachable from both directions; the scored verdicts belong to exactly one
// each.
func (v Verdict) allowedFor(k DecisionKind) bool {
	switch v {
	case VerdictEnter, VerdictDecline:
		return k == KindEntry
	case VerdictContinue, VerdictClose:
		return k == KindExit
	case VerdictRefuse:
		return true
	}
	return false
}

// DecisionTerms is the money ledger of §40f: every term of the inequality
// that produced the verdict, each a dimensional line of its own. All amounts
// are Money over the HoldPlan in one unit — §2.4's "one unit, USD over the
// HoldPlan" rule; no rate is ever multiplied by hours.
type DecisionTerms struct {
	// FundingQuantile is Q_p[Σ funding] over the planned (entry) or
	// remaining (exit) settlements, under the joint aggregation rule.
	FundingQuantile fixedpoint.Money

	// Basis is the basis term's net contribution — conservative by default:
	// zero favorable convergence, the measured adverse move charged.
	Basis fixedpoint.Money

	// The four fee cells, per leg by side. Spot and perp are separate
	// because §2.4 says the numbers materially differ; entry and exit are
	// separate because the exit's Δ rule needs the closing half nameable.
	SpotEntryFee, SpotExitFee, PerpEntryFee, PerpExitFee fixedpoint.Money

	// Slippage is the ExecutionCostModel's expected charge (both legs).
	Slippage fixedpoint.Money

	// Opportunity is the reserve/opportunity term.
	Opportunity fixedpoint.Money

	// ClosingCostDelta and RiskDelta are the exit score's Δ lines — charged
	// only where the branches differ, never to continue alone (§2.4).
	// Declared zeros in the branch-symmetric common case.
	ClosingCostDelta, RiskDelta fixedpoint.Money

	// ScaledHurdle is hurdle · C_allocated · holdFraction — the right-hand
	// side of the entry inequality. The denominator inside it is C_allocated,
	// never C_legs (§40f rule 3). For an exit it is the same product over the
	// REMAINING hold — §2.4's exit "opportunity-cost" line, computed by the
	// core so the hurdle arithmetic has one home for both directions.
	ScaledHurdle fixedpoint.Money

	// HoldFraction is the hold as a fraction of a year — planned (entry) or
	// remaining (exit) — the scaling applied to the annualized hurdle.
	HoldFraction fixedpoint.Rate

	// EntryMargin is the C3 drift margin the entry score was judged against
	// (§2.2's "clears (round-trip cost + hurdle) WITH MARGIN"): the measured
	// EntryDrift rate, carried with its artifact exactly as Band carries the
	// exit band's. Entry only.
	EntryMargin MeasuredRate

	// Band is the exit band the score was judged against; exit only.
	Band MeasuredRate

	// Dwell is the UPDATED dwell state this evaluation produced — what the
	// next evaluation's DecisionInput feeds back; exit only. (The PRIOR
	// state it was fed is in the Input.)
	Dwell DwellState
}

// Validate reports whether the ledger is complete for its kind.
func (t DecisionTerms) Validate(kind DecisionKind) error {
	for _, m := range []struct {
		name string
		v    fixedpoint.Money
	}{
		{"funding quantile", t.FundingQuantile},
		{"basis", t.Basis},
		{"spot entry fee", t.SpotEntryFee},
		{"spot exit fee", t.SpotExitFee},
		{"perp entry fee", t.PerpEntryFee},
		{"perp exit fee", t.PerpExitFee},
		{"slippage", t.Slippage},
		{"opportunity", t.Opportunity},
		{"closing-cost delta", t.ClosingCostDelta},
		{"risk delta", t.RiskDelta},
		{"scaled hurdle", t.ScaledHurdle},
	} {
		if err := m.v.Validate(); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
	}
	if err := t.HoldFraction.Validate(); err != nil {
		return fmt.Errorf("hold fraction: %w", err)
	}
	switch kind {
	case KindEntry:
		if err := t.EntryMargin.Validate(); err != nil {
			return fmt.Errorf("entry margin: %w", err)
		}
		if t.Band != (MeasuredRate{}) {
			return fmt.Errorf("entry with exit band: %w", ErrVerdictKindMismatch)
		}
		if t.Dwell != (DwellState{}) {
			return fmt.Errorf("entry with dwell state: %w", ErrVerdictKindMismatch)
		}
		return nil
	case KindExit:
		if t.EntryMargin != (MeasuredRate{}) {
			return fmt.Errorf("exit with entry margin: %w", ErrVerdictKindMismatch)
		}
		if err := t.Band.Validate(); err != nil {
			return fmt.Errorf("band: %w", err)
		}
		if err := t.Dwell.Validate(); err != nil {
			return fmt.Errorf("dwell: %w", err)
		}
		return nil
	}
	return fmt.Errorf("kind %q: %w", kind, ErrKindInvalid)
}

// Decision is the core's output (§40f): the verdict, every term of the
// inequality that produced it, and the identity of every input it was
// produced from.
//
// # The input rides whole, and that is the completeness property
//
// §40f states completeness "as a property, not a roster": the journaled
// Decision must contain enough to RECOMPUTE the verdict and to answer, from
// the record alone, which revision of every input produced it. The input
// already carries each revision identity (fee/lot/hurdle provenance, the
// policy revision, the plan revision, the calibration identities, the
// watermarks); embedding it satisfies the property by construction, where a
// hand-picked identity roster satisfies it by curation — and a curated list
// is one added input away from an unanswerable "which revision" question.
type Decision struct {
	// Kind and Verdict; the pair must agree (see Verdict.allowedFor).
	Kind    DecisionKind
	Verdict Verdict

	// At is the evaluation instant — the same instant the input carries.
	At time.Time

	// Input is the complete, validated input the verdict was produced from.
	// For a REFUSAL it holds whatever had been assembled when the refusing
	// rule fired, and is deliberately NOT re-validated: an unpopulated input
	// is one of the things a refusal exists to record.
	Input DecisionInput

	// Terms is the money ledger. Scored verdicts only: a refusal prices
	// nothing, and partial arithmetic on a refused input would journal
	// numbers the verdict never depended on.
	Terms DecisionTerms

	// RefusedBy names the refusing rule, exactly when Verdict is
	// VerdictRefuse — "the refusing rule named" is §40f's requirement, and
	// an empty name would journal an approval-shaped hole.
	RefusedBy string
}

// Validate reports whether the decision is journalable.
func (d Decision) Validate() error {
	if err := d.Kind.Validate(); err != nil {
		return fmt.Errorf("kind: %w", err)
	}
	if err := d.Verdict.Validate(); err != nil {
		return fmt.Errorf("verdict: %w", err)
	}
	if !d.Verdict.allowedFor(d.Kind) {
		return fmt.Errorf("%s on %s: %w", d.Verdict, d.Kind, ErrVerdictKindMismatch)
	}
	if d.At.IsZero() {
		return fmt.Errorf("at: %w", ErrUnset)
	}

	if d.Verdict == VerdictRefuse {
		if d.RefusedBy == "" {
			return ErrRefusalUnnamed
		}
		// A refusal carries no ledger: the zero Terms is the only honest
		// value for arithmetic that never ran.
		if d.Terms != (DecisionTerms{}) {
			return fmt.Errorf("refusal with terms: %w", ErrRefusalMisplaced)
		}
		return nil
	}

	if d.RefusedBy != "" {
		return fmt.Errorf("%q on %s: %w", d.RefusedBy, d.Verdict, ErrRefusalMisplaced)
	}
	if err := d.Input.Validate(); err != nil {
		return fmt.Errorf("input: %w", err)
	}
	if d.Input.Kind != d.Kind {
		return fmt.Errorf("input kind %s, decision kind %s: %w", d.Input.Kind, d.Kind, ErrVerdictKindMismatch)
	}
	if !d.Input.At.Equal(d.At) {
		return fmt.Errorf("input at %s, decision at %s: %w", d.Input.At, d.At, ErrVerdictKindMismatch)
	}
	if err := d.Terms.Validate(d.Kind); err != nil {
		return fmt.Errorf("terms: %w", err)
	}
	return nil
}
