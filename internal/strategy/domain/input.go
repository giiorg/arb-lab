package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// DecisionKind is which direction a decision evaluates: entry (no position)
// or exit (continue-vs-close on a held one). The two are different scores —
// §2.4 is explicit that exit is "an incremental continue-vs-close score, not
// a re-run of the entry test" — so the kind is a declared member, never
// inferred from which optional fields happen to be populated.
type DecisionKind string

const (
	KindEntry DecisionKind = "entry"
	KindExit  DecisionKind = "exit"
)

// Validate reports whether k is one of the declared kinds.
func (k DecisionKind) Validate() error {
	switch k {
	case KindEntry, KindExit:
		return nil
	}
	return fmt.Errorf("%q: %w", string(k), ErrKindInvalid)
}

func (k DecisionKind) String() string { return string(k) }

// DwellState is the exit dwell fed back through DecisionInput: how many
// realized settlements the forward score has stayed below the exit band, and
// the last settlement counted.
//
// §40b: exit_persist counts SETTLEMENTS, not calls, so the dwell is an input
// fed back from the prior journaled Decision, never a counter the core keeps
// — a core keeping state across calls is not §2.4's "two conforming
// implementations decide alike" shape.
//
// # Declared is a real bit, not decoration
//
// A count of zero with no last-counted instant is the legitimate state of
// every healthy position — and also exactly what a caller that forgot to
// feed the dwell back would pass. fixedpoint distinguishes absent from
// declared-zero with the asset tag; a counter has no tag, so the declaration
// is explicit. Forgetting it is a refusal naming the field, not a dwell that
// silently restarts (which would hold a loser one band-width longer per
// forgotten feedback).
type DwellState struct {
	// Declared records that somebody actually supplied this state.
	Declared bool

	// Count is how many consecutive realized settlements the score has been
	// below the band.
	Count uint32

	// LastRealized is the settlement instant last counted; zero exactly when
	// Count is zero.
	LastRealized time.Time
}

// Validate reports whether the state was fed back and is internally
// consistent.
func (d DwellState) Validate() error {
	if !d.Declared {
		return ErrDwellUndeclared
	}
	if (d.Count == 0) != d.LastRealized.IsZero() {
		return fmt.Errorf("count %d, last realized %v: %w", d.Count, d.LastRealized, ErrDwellInconsistent)
	}
	return nil
}

// DecisionInput is §40b's roster: the COMPLETE set of terms the verdict may
// depend on, assembled by the application layer from the ports and validated
// before the core sees it. Every member's ground is recorded in the design
// record; the property the roster serves is that the core is a pure function
// of this value — hidden state, ambient fetches and silently-zero costs all
// enter through terms that no declared input carries.
type DecisionInput struct {
	// Kind is the direction under evaluation.
	Kind DecisionKind

	// At is the evaluation instant, supplied by the caller from the injected
	// Clock. Every age the core judges is derived from a watermark against
	// this instant — §3.4's read-time re-judgement observed from the
	// consumer side.
	At time.Time

	// View is the assembled market view (§40c).
	View MarketView

	// Policy is the revision-identified params snapshot.
	Policy PolicyProjection

	// Forecasts are the FundingForecast port's per-settlement answers over
	// the plan's forecastable settlements — exactly those, §2.4's horizon
	// rule; see Validate.
	Forecasts []SettlementForecast

	// Basis is the BasisForecast port's answer.
	Basis BasisTerm

	// SpotFees and PerpFees are the two legs' fee rows, fetched separately
	// (§2.4's explicit trap: spot taker is materially higher).
	SpotFees, PerpFees LegFees

	// SpotLots and PerpLots are the two legs' quantity constraints; the
	// quantization (§40g) runs on these, never on ambient data.
	SpotLots, PerpLots LotRules

	// ResidualTolerance is §2.4's explicit residual-delta tolerance:
	// leftover cross-leg delta beyond it is a sizing FAILURE (a journaled
	// refusal), never silently accepted.
	ResidualTolerance fixedpoint.Quantity

	// CandidateQty is the quantity the sizing is scored at — explicit,
	// never inferred back out of C_allocated.
	CandidateQty fixedpoint.Quantity

	// ExecCost is the ExecutionCostModel port's expected charge for the
	// candidate.
	ExecCost ExecutionCostEstimate

	// Hurdle is the benchmark revision (§2.1).
	Hurdle HurdleBenchmark

	// Capital is the C_allocated decomposition (§2.2).
	Capital CapitalAllocation

	// Opportunity is the reserve/opportunity-cost term of §2.4's entry
	// inequality. May be a declared zero.
	Opportunity fixedpoint.Money

	// ClosingCostDelta and RiskDelta are §2.4's exit-score Δ terms, carried
	// as separate dimensional members — folding them into a generic cost
	// basket is how one gets silently charged to the wrong branch (§40b).
	// Each may legitimately BE zero (§2.4's branch-symmetric common case):
	// the refusal is of ABSENCE — a fixedpoint zero value with no asset —
	// never of a declared zero.
	ClosingCostDelta, RiskDelta fixedpoint.Money

	// Plan is the working schedule: the entry proposal at revision 1, or the
	// current journaled HoldPlanRevision for an exit (§40f's interval-flip
	// rule).
	Plan HoldPlanRevision

	// Position holds the exit direction's position facts. Exit-only; must be
	// the zero value for an entry evaluation (an entry scored "with a
	// position" is an exit wearing the wrong test).
	Position PositionView

	// Dwell is the prior exit-dwell state fed back from the last journaled
	// Decision. Exit-only.
	Dwell DwellState
}

// Validate reports whether the input is complete for its Kind. Every member
// above is checked — absence anywhere is a refusal, because every one of
// these terms reads as zero when forgotten, and every zero flatters the gate.
func (in DecisionInput) Validate() error {
	if err := in.Kind.Validate(); err != nil {
		return fmt.Errorf("kind: %w", err)
	}
	if in.At.IsZero() {
		return fmt.Errorf("at: %w", ErrUnset)
	}
	if err := in.View.Validate(); err != nil {
		return fmt.Errorf("view: %w", err)
	}
	if err := in.Policy.Validate(); err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	if err := in.validateForecasts(); err != nil {
		return err
	}
	if err := in.Basis.Validate(); err != nil {
		return fmt.Errorf("basis: %w", err)
	}
	if err := in.SpotFees.Validate(); err != nil {
		return fmt.Errorf("spot fees: %w", err)
	}
	if in.SpotFees.Instrument != in.View.Spot.Instrument {
		return fmt.Errorf("spot fees for %s, view holds %s: %w",
			in.SpotFees.Instrument, in.View.Spot.Instrument, ErrLegsMismatch)
	}
	if err := in.PerpFees.Validate(); err != nil {
		return fmt.Errorf("perp fees: %w", err)
	}
	if in.PerpFees.Instrument != in.View.Perp.Instrument {
		return fmt.Errorf("perp fees for %s, view holds %s: %w",
			in.PerpFees.Instrument, in.View.Perp.Instrument, ErrLegsMismatch)
	}
	if err := in.SpotLots.Validate(); err != nil {
		return fmt.Errorf("spot lots: %w", err)
	}
	if in.SpotLots.Instrument != in.View.Spot.Instrument {
		return fmt.Errorf("spot lots for %s, view holds %s: %w",
			in.SpotLots.Instrument, in.View.Spot.Instrument, ErrLegsMismatch)
	}
	if err := in.PerpLots.Validate(); err != nil {
		return fmt.Errorf("perp lots: %w", err)
	}
	if in.PerpLots.Instrument != in.View.Perp.Instrument {
		return fmt.Errorf("perp lots for %s, view holds %s: %w",
			in.PerpLots.Instrument, in.View.Perp.Instrument, ErrLegsMismatch)
	}
	if err := in.ResidualTolerance.Validate(); err != nil {
		return fmt.Errorf("residual tolerance: %w", err)
	}
	if err := in.CandidateQty.Validate(); err != nil {
		return fmt.Errorf("candidate qty: %w", err)
	}
	if err := in.ExecCost.Validate(); err != nil {
		return fmt.Errorf("exec cost: %w", err)
	}
	// The estimate must have priced THIS candidate — an estimate for a
	// different quantity is a cost model for a different trade.
	if in.ExecCost.Qty != in.CandidateQty {
		return fmt.Errorf("exec cost priced %s, candidate is %s: %w",
			in.ExecCost.Qty, in.CandidateQty, ErrLegsMismatch)
	}
	if err := in.Hurdle.Validate(); err != nil {
		return fmt.Errorf("hurdle: %w", err)
	}
	if err := in.Capital.Validate(); err != nil {
		return fmt.Errorf("capital: %w", err)
	}
	if err := in.Opportunity.Validate(); err != nil {
		return fmt.Errorf("opportunity: %w", err)
	}
	if err := in.ClosingCostDelta.Validate(); err != nil {
		return fmt.Errorf("closing-cost delta: %w", err)
	}
	if err := in.RiskDelta.Validate(); err != nil {
		return fmt.Errorf("risk delta: %w", err)
	}
	if err := in.Plan.Validate(); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	return in.validateKindMembers()
}

// validateForecasts is §2.4's horizon rule as a structural check: the
// forecasts must cover exactly the plan's forecastable settlements.
//
// # The boundary is strict-after for exits, and that is a stated reading
//
// For an entry every planned settlement is forecastable. For an exit the
// remaining settlements are those strictly after At: a settlement exactly AT
// the evaluation instant is settling now — it is about to be a realized fact,
// and forecasting a fact invites the forecast and the realization to be
// summed once each. Recorded as a reading for the slice C close's reading
// pass, beside §40e's and §40f's.
func (in DecisionInput) validateForecasts() error {
	want := in.Plan.Plan.Settlements
	if in.Kind == KindExit {
		n := 0
		for _, t := range want {
			if t.After(in.At) {
				break
			}
			n++
		}
		want = want[n:]
	}
	if len(in.Forecasts) != len(want) {
		return fmt.Errorf("%d forecasts over %d forecastable settlements: %w",
			len(in.Forecasts), len(want), ErrForecastHorizon)
	}
	for n, f := range in.Forecasts {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("forecast %d: %w", n, err)
		}
		if !f.At.Equal(want[n]) {
			return fmt.Errorf("forecast %d at %s, plan settles %s: %w", n, f.At, want[n], ErrForecastHorizon)
		}
	}
	return nil
}

// validateKindMembers holds the kind-dependent half: the exit-only members
// must be present for an exit and absent for an entry — in both directions,
// because a populated position on an entry evaluation is not extra data, it
// is evidence the caller ran the wrong test.
func (in DecisionInput) validateKindMembers() error {
	switch in.Kind {
	case KindEntry:
		if !in.Position.isZero() {
			return fmt.Errorf("entry with position facts: %w", ErrVerdictKindMismatch)
		}
		if in.Dwell != (DwellState{}) {
			return fmt.Errorf("entry with dwell state: %w", ErrVerdictKindMismatch)
		}
		if in.Plan.Num != 1 {
			return fmt.Errorf("entry against plan revision %d: %w", in.Plan.Num, ErrRevisionSequence)
		}
		return nil
	case KindExit:
		if err := in.Position.Validate(); err != nil {
			return fmt.Errorf("position: %w", err)
		}
		if in.Position.Spot != in.View.Spot.Instrument || in.Position.Perp != in.View.Perp.Instrument {
			return fmt.Errorf("position %s/%s, view %s/%s: %w",
				in.Position.Spot, in.Position.Perp,
				in.View.Spot.Instrument, in.View.Perp.Instrument, ErrLegsMismatch)
		}
		if err := in.Dwell.Validate(); err != nil {
			return fmt.Errorf("dwell: %w", err)
		}
		return nil
	}
	// Unreachable past Validate's kind check; named so a new kind fails here
	// rather than validating vacuously.
	return fmt.Errorf("kind %q: %w", in.Kind, ErrKindInvalid)
}
