package calibrate

import (
	"fmt"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// Surrogate is §41b's pre-registered constant-cost frame: everything the
// Enter/Continue replay holds FIXED while the funding series varies. The
// calibration era has no books, and Enter demands an execution cost — so
// the replay runs on declared prices, the frozen surrogate cost, a
// declared candidate quantity, and the committed reference rows, with the
// conservative direction argued in the artifact's rationale (holding
// historical costs at the frozen bound overstates them, which suppresses
// entries and understates occupancy — the direction that hardens the
// gate).
//
// The SAME frame feeds ClearingLevel and Occupancy (§41b round 3's
// requirement: the band's entry-clearing level and the occupancy replay
// share one frozen definition — same candidate quantity, same surrogate
// cost), which is why it is one type rather than two argument lists.
//
// # Prices are a fixed frame, and that is a stated limitation
//
// The replay's tops are bid==ask at the declared frame prices, and they
// do not move with history: price level enters the verdicts only through
// notional-versus-capital scaling, and a frame that moved with klines
// would make the clearing level a per-instant curve no single artifact
// number could honestly summarize. The limitation is stated in the
// derivation's rationale rather than absorbed.
type Surrogate struct {
	// Spot and Perp are the pair's legs.
	Spot, Perp ids.VenueInstrument

	// SpotPrice and PerpPrice are the frame prices; each leg's top is
	// locked (bid == ask) at its price.
	SpotPrice, PerpPrice fixedpoint.Price

	// Interval is the funding interval the era ran on; Cap and Floor are
	// the venue's clamps.
	Interval   time.Duration
	Cap, Floor fixedpoint.Rate

	// Params is the frozen policy the replay decides under — including
	// the already-frozen EntryDrift and ExitBand (§41b's derivation
	// order: costs, dispersion and band freeze first; the occupancy
	// replay consumes them).
	Params domain.StrategyParams

	// PolicyRev identifies the params snapshot for the journaled inputs.
	PolicyRev config.Revision

	// Dispersion is the frozen adverse-drift distribution driving the
	// estimator.
	Dispersion domain.Dispersion

	// AdverseBasis is the frozen hold-scale basis rate; the frame charges
	// it exactly as the conservative runtime model does — rate × ask-
	// priced notional, zero favorable leg.
	AdverseBasis domain.MeasuredRate

	// SpotFees, PerpFees, SpotLots, PerpLots and Hurdle are the committed
	// reference rows.
	SpotFees, PerpFees domain.LegFees
	SpotLots, PerpLots domain.LotRules
	Hurdle             domain.HurdleBenchmark

	// Capital is the declared C_allocated decomposition.
	Capital domain.CapitalAllocation

	// CandidateQty is the declared candidate quantity, pre-registered
	// lot-aligned on both legs — the frame refuses a candidate the
	// quantization would move, because the exit direction scores the HELD
	// quantity and a drifted candidate would make the surrogate cost
	// estimate price a different trade than the position holds.
	CandidateQty      fixedpoint.Quantity
	ResidualTolerance fixedpoint.Quantity

	// ExecCost is the frozen surrogate execution cost for CandidateQty at
	// the declared stratum.
	ExecCost domain.ExecutionCostEstimate

	// Opportunity, ClosingCostDelta and RiskDelta are the declared Δ
	// terms — typically declared zeros, never absent.
	Opportunity                 fixedpoint.Money
	ClosingCostDelta, RiskDelta fixedpoint.Money
}

// Validate reports whether the frame could drive a replay. Every member
// is checked here once, so the per-instant assembly can be refusal-free.
func (s Surrogate) Validate() error {
	if err := s.SpotPrice.Validate(); err != nil {
		return fmt.Errorf("surrogate spot price: %w", err)
	}
	if err := s.PerpPrice.Validate(); err != nil {
		return fmt.Errorf("surrogate perp price: %w", err)
	}
	if s.Interval <= 0 {
		return fmt.Errorf("surrogate interval %v: %w", s.Interval, domain.ErrUnset)
	}
	if err := s.Params.Validate(); err != nil {
		return fmt.Errorf("surrogate params: %w", err)
	}
	if err := s.PolicyRev.Validate(); err != nil {
		return fmt.Errorf("surrogate policy rev: %w", err)
	}
	if err := s.Dispersion.Validate(); err != nil {
		return fmt.Errorf("surrogate dispersion: %w", err)
	}
	if err := s.AdverseBasis.Validate(); err != nil {
		return fmt.Errorf("surrogate adverse basis: %w", err)
	}
	if err := s.Hurdle.Validate(); err != nil {
		return fmt.Errorf("surrogate hurdle: %w", err)
	}
	if err := s.Capital.Validate(); err != nil {
		return fmt.Errorf("surrogate capital: %w", err)
	}
	if err := s.CandidateQty.Validate(); err != nil {
		return fmt.Errorf("surrogate candidate: %w", err)
	}
	if err := s.ResidualTolerance.Validate(); err != nil {
		return fmt.Errorf("surrogate residual tolerance: %w", err)
	}
	if err := s.ExecCost.Validate(); err != nil {
		return fmt.Errorf("surrogate exec cost: %w", err)
	}
	if s.ExecCost.Qty != s.CandidateQty {
		return fmt.Errorf("surrogate exec cost priced %s, candidate is %s: %w",
			s.ExecCost.Qty, s.CandidateQty, domain.ErrLegsMismatch)
	}
	if err := s.Opportunity.Validate(); err != nil {
		return fmt.Errorf("surrogate opportunity: %w", err)
	}
	if err := s.ClosingCostDelta.Validate(); err != nil {
		return fmt.Errorf("surrogate closing-cost delta: %w", err)
	}
	if err := s.RiskDelta.Validate(); err != nil {
		return fmt.Errorf("surrogate risk delta: %w", err)
	}
	// Fees, lots and the leg identities are validated by the assembled
	// input's own Validate — the one authority — on the first assembly;
	// duplicating their cross-checks here would be a second rule set.
	return nil
}

// basisTerm charges the frozen adverse basis exactly as the conservative
// runtime model does: zero favorable leg (declared), rate × ask-priced
// notional, both roundings away from zero — cost arithmetic. Restated
// here because the domain cannot import the driven adapter; the driven
// battery pins the same arithmetic on its side.
func (s Surrogate) basisTerm() (domain.BasisTerm, error) {
	notional, err := fixedpoint.Notional(s.SpotPrice, s.CandidateQty, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return domain.BasisTerm{}, fmt.Errorf("surrogate basis notional: %w", err)
	}
	charge, err := notional.Mul(s.AdverseBasis.Value, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return domain.BasisTerm{}, fmt.Errorf("surrogate basis charge: %w", err)
	}
	return domain.BasisTerm{
		FavorableConvergence: fixedpoint.ZeroMoney(s.SpotPrice.Quote()),
		AdverseMove:          domain.MeasuredMoney{Amount: charge, Artifact: s.AdverseBasis.Artifact},
	}, nil
}

// watermarkAt mints the replay's provenance: the surrogate observed the
// era's own instants — SourceTime and RecvTime are both the settlement
// instant, so every age the core derives is zero and freshness can never
// refuse a replayed input.
func watermarkAt(at time.Time) mdomain.Watermark {
	return mdomain.Watermark{SourceTime: at, RecvTime: at}
}

// entryInput assembles one entry evaluation: the minimum
// policy-admissible plan proposed from the observation, forecasts from
// the deployed estimator over exactly that plan.
func (s Surrogate) entryInput(at time.Time, estimate fixedpoint.Rate, realized []domain.RealizedSettlement) (domain.DecisionInput, error) {
	obs := s.observation(at, estimate)
	prop, err := cashcarry.ProposeHoldPlan(obs, s.Params.MinHoldSettlements)
	if err != nil {
		return domain.DecisionInput{}, fmt.Errorf("surrogate plan: %w", err)
	}
	forecasts, err := s.forecasts(obs, realized, prop.Settlements)
	if err != nil {
		return domain.DecisionInput{}, err
	}
	basis, err := s.basisTerm()
	if err != nil {
		return domain.DecisionInput{}, err
	}
	return domain.DecisionInput{
		Kind:              domain.KindEntry,
		At:                at,
		View:              s.view(at, obs, realized),
		Policy:            domain.PolicyProjection{Params: s.Params, Rev: s.PolicyRev},
		Forecasts:         forecasts,
		Basis:             basis,
		SpotFees:          s.SpotFees,
		PerpFees:          s.PerpFees,
		SpotLots:          s.SpotLots,
		PerpLots:          s.PerpLots,
		ResidualTolerance: s.ResidualTolerance,
		CandidateQty:      s.CandidateQty,
		ExecCost:          s.ExecCost,
		Hurdle:            s.Hurdle,
		Capital:           s.Capital,
		Opportunity:       s.Opportunity,
		ClosingCostDelta:  s.ClosingCostDelta,
		RiskDelta:         s.RiskDelta,
		Plan: domain.HoldPlanRevision{
			Num: 1, Trigger: domain.TriggerEntry, At: at, Plan: prop,
		},
	}, nil
}

// exitInput assembles one exit evaluation over the working plan's
// remaining settlements — strictly after the instant, DecisionInput's own
// boundary.
func (s Surrogate) exitInput(at time.Time, estimate fixedpoint.Rate, realized []domain.RealizedSettlement,
	plan domain.HoldPlanRevision, position domain.PositionView, dwell domain.DwellState) (domain.DecisionInput, error) {
	obs := s.observation(at, estimate)
	var remaining []time.Time
	for _, t := range plan.Plan.Settlements {
		if t.After(at) {
			remaining = append(remaining, t)
		}
	}
	forecasts, err := s.forecasts(obs, realized, remaining)
	if err != nil {
		return domain.DecisionInput{}, err
	}
	basis, err := s.basisTerm()
	if err != nil {
		return domain.DecisionInput{}, err
	}
	return domain.DecisionInput{
		Kind:              domain.KindExit,
		At:                at,
		View:              s.view(at, obs, realized),
		Policy:            domain.PolicyProjection{Params: s.Params, Rev: s.PolicyRev},
		Forecasts:         forecasts,
		Basis:             basis,
		SpotFees:          s.SpotFees,
		PerpFees:          s.PerpFees,
		SpotLots:          s.SpotLots,
		PerpLots:          s.PerpLots,
		ResidualTolerance: s.ResidualTolerance,
		CandidateQty:      position.Qty,
		ExecCost:          s.ExecCost,
		Hurdle:            s.Hurdle,
		Capital:           s.Capital,
		Opportunity:       s.Opportunity,
		ClosingCostDelta:  s.ClosingCostDelta,
		RiskDelta:         s.RiskDelta,
		Plan:              plan,
		Position:          position,
		Dwell:             dwell,
	}, nil
}

// forecasts runs the deployed estimator — the frame's dispersion at the
// policy's quantile and half-life — over a horizon.
func (s Surrogate) forecasts(obs mdomain.FundingObservation, realized []domain.RealizedSettlement, horizon []time.Time) ([]domain.SettlementForecast, error) {
	est, err := domain.NewEWMAEstimator(s.Params.EWMAHalfLife, s.Params.DecisionQuantile, s.Dispersion)
	if err != nil {
		return nil, fmt.Errorf("surrogate estimator: %w", err)
	}
	fs, err := est.Estimate(domain.FundingMaterials{Observation: obs, Realized: realized}, horizon)
	if err != nil {
		return nil, fmt.Errorf("surrogate forecasts: %w", err)
	}
	return fs, nil
}

// observation builds the funding block at an evaluation instant: the
// causal live-estimate surrogate (the trailing EWMA over the realized
// window — the refetchable history carries no venue estimate stream, and
// the trailing forecast is the deployed estimator's own later-settlement
// source), the frame clamps, and the next grid settlement.
func (s Surrogate) observation(at time.Time, estimate fixedpoint.Rate) mdomain.FundingObservation {
	return mdomain.FundingObservation{
		Instrument: s.Perp,
		Rate:       estimate,
		Interval:   s.Interval,
		NextSettle: at.Add(s.Interval),
		Cap:        s.Cap,
		Floor:      s.Floor,
		Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at},
	}
}

// view assembles the market view at an instant: locked tops at the frame
// prices, the observation, and the realized window.
func (s Surrogate) view(at time.Time, obs mdomain.FundingObservation, realized []domain.RealizedSettlement) domain.MarketView {
	wm := mdomain.Watermark{SourceTime: at, RecvTime: at}
	return domain.MarketView{
		Spot:     domain.PriceView{Instrument: s.Spot, Bid: s.SpotPrice, Ask: s.SpotPrice, Watermark: wm},
		Perp:     domain.PriceView{Instrument: s.Perp, Bid: s.PerpPrice, Ask: s.PerpPrice, Watermark: wm},
		Funding:  obs,
		Realized: realized,
	}
}
