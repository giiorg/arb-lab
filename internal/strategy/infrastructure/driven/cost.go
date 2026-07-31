package driven

import (
	"fmt"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// costModelName names the milestone-2 conservative bound (§8's `$1K` model;
// the full adverse-selection model is the `$10K+` adapter, already scheduled
// by the spec — which is what makes the port's seam real, §40e).
const costModelName = "conservative_bound"

// CostBucket is one row of the artifact's p90-slippage table: for order
// quantities up to MaxQty, the measured p90 slippage as a fraction of
// notional. §8 conditions the measured slippage on order size; the buckets
// are that conditioning.
type CostBucket struct {
	// MaxQty is the bucket's inclusive upper bound.
	MaxQty fixedpoint.Quantity

	// P90 is the measured p90 slippage per unit of notional for orders in
	// this bucket.
	P90 fixedpoint.Rate
}

// CostCalibration is the artifact-owned number set behind the conservative
// bound — a context-owned schema beside its adapter, riding nothing but the
// artifact's identity (slice A's pattern for reference schemas: the kernel
// owns mechanisms, never a context's schema).
//
// Class 2 throughout (§40e): every number here is a measurement, so the set
// refuses to validate without the artifact identity, and the adapter refuses
// to exist without a valid set. There are no defaults — each number's zero
// flatters the gate (zero adverse move, zero slippage, zero latency).
type CostCalibration struct {
	// Artifact identifies the step-4 calibration artifact.
	Artifact domain.CalibrationRef

	// AdverseMove is the measured adverse price move over the calibrated
	// latency window, as a fraction of notional — the "+ adverse move" half
	// of §8's depth-walk branch. Non-negative: an adverse move that credits
	// is a sign error.
	AdverseMove fixedpoint.Rate

	// Latency is the decision-to-arrival latency the artifact measured at.
	Latency time.Duration

	// Buckets is the p90 table, strictly ascending in MaxQty.
	Buckets []CostBucket
}

// Validate reports whether the set is a complete calibration.
func (c CostCalibration) Validate() error {
	if err := c.Artifact.Validate(); err != nil {
		return fmt.Errorf("cost calibration: %w", err)
	}
	if err := c.AdverseMove.Validate(); err != nil {
		return fmt.Errorf("cost calibration adverse move: %w", err)
	}
	sign, err := c.AdverseMove.Sign()
	if err != nil {
		return fmt.Errorf("cost calibration adverse move: %w", err)
	}
	if sign < 0 {
		return fmt.Errorf("cost calibration adverse move %s: %w", c.AdverseMove, ErrChargeNegative)
	}
	if c.Latency <= 0 {
		return fmt.Errorf("cost calibration latency %v: %w", c.Latency, domain.ErrUnset)
	}
	if len(c.Buckets) == 0 {
		return fmt.Errorf("cost calibration buckets: %w", domain.ErrUnset)
	}
	for n, b := range c.Buckets {
		if err := b.MaxQty.Validate(); err != nil {
			return fmt.Errorf("cost calibration bucket %d max qty: %w", n, err)
		}
		qsign, err := b.MaxQty.Sign()
		if err != nil {
			return fmt.Errorf("cost calibration bucket %d max qty: %w", n, err)
		}
		if qsign <= 0 {
			return fmt.Errorf("cost calibration bucket %d max qty %s: %w", n, b.MaxQty, domain.ErrUnset)
		}
		if err := b.P90.Validate(); err != nil {
			return fmt.Errorf("cost calibration bucket %d p90: %w", n, err)
		}
		psign, err := b.P90.Sign()
		if err != nil {
			return fmt.Errorf("cost calibration bucket %d p90: %w", n, err)
		}
		if psign < 0 {
			return fmt.Errorf("cost calibration bucket %d p90 %s: %w", n, b.P90, ErrChargeNegative)
		}
		if n == 0 {
			continue
		}
		c2, err := c.Buckets[n-1].MaxQty.Cmp(b.MaxQty)
		if err != nil {
			return fmt.Errorf("cost calibration buckets %d,%d: %w", n-1, n, err)
		}
		if c2 >= 0 {
			return fmt.Errorf("cost calibration bucket %d max %s, %d max %s: %w",
				n-1, c.Buckets[n-1].MaxQty, n, b.MaxQty, ErrBucketOrder)
		}
	}
	return nil
}

// BookSource is the adapter's own book seam. §40e: the adapter sources its
// own book state — its wiring, not the strategy application's, so book
// plumbing stays out of the context. The false return is "no book held for
// this instrument", a state, not a fault (engine/state's cache rule).
//
// No freshness verdict crosses this seam either: the adapter prices the book
// the source holds, and whether that book is fresh enough to decide on is
// the assembly's §3.4 judgement, made where the ages are derived.
type BookSource interface {
	Snapshot(ids.VenueInstrument) (mdomain.BookSnapshot, bool)
}

// ConservativeCostModel is the milestone-2 ExecutionCostModel adapter: §8's
// conservative bound, structure fixed by the spec, numbers the artifact's.
//
// Per leg it takes the WORSE of §8's two branches:
//
//  1. the depth-walked fill premium for the candidate quantity, plus the
//     measured adverse move over the calibrated latency, and
//  2. the measured p90 slippage conditional on order size,
//
// and because the request names no direction (a bound priced for a decision
// must cover the entry and the exit it implies), each branch is evaluated on
// BOTH sides of the book and the worse side is charged. Every rounding in
// the chain goes away from zero: this is cost arithmetic, and an understated
// cost is the flattering direction.
//
// What it refuses, it refuses by name: a tactic the artifact never priced
// (ErrTacticUncalibrated — the maker-entry fork is a documented opt-in
// needing its own calibration), a latency assumption LONGER than the
// artifact measured (ErrLatencyOutsideCalibration — a shorter assumption is
// admissible because the calibrated charge then overstates), a quantity past
// the last bucket (ErrSizeOutsideCalibration), and a book too thin to fill
// the quantity (ErrDepthInsufficient — extrapolating beyond visible depth
// would be a number the model made up).
type ConservativeCostModel struct {
	cal   CostCalibration
	books BookSource
}

var _ ports.ExecutionCostModel = (*ConservativeCostModel)(nil)

// NewConservativeCostModel builds the adapter, refusing an invalid
// calibration or a missing book source.
func NewConservativeCostModel(cal CostCalibration, books BookSource) (*ConservativeCostModel, error) {
	if err := cal.Validate(); err != nil {
		return nil, err
	}
	if books == nil {
		return nil, fmt.Errorf("cost model book source: %w", domain.ErrUnset)
	}
	return &ConservativeCostModel{cal: cal, books: books}, nil
}

// EstimateCost implements the ExecutionCostModel port.
func (m *ConservativeCostModel) EstimateCost(req domain.ExecutionCostRequest) (domain.ExecutionCostEstimate, error) {
	if m == nil || m.books == nil {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("cost model unconstructed: %w", domain.ErrUnset)
	}
	if err := m.cal.Validate(); err != nil {
		return domain.ExecutionCostEstimate{}, err
	}
	if err := req.Validate(); err != nil {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost: %w", err)
	}
	if req.Tactic != domain.TacticTakerIOC {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost tactic %s: %w",
			req.Tactic, ErrTacticUncalibrated)
	}
	if req.Latency > m.cal.Latency {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost latency %v, calibrated at %v: %w",
			req.Latency, m.cal.Latency, ErrLatencyOutsideCalibration)
	}
	sign, err := req.Qty.Sign()
	if err != nil {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost qty: %w", err)
	}
	if sign <= 0 {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost qty %s: %w", req.Qty, ErrQtyNotPositive)
	}

	p90, err := m.p90For(req.Qty)
	if err != nil {
		return domain.ExecutionCostEstimate{}, err
	}

	spotCost, err := m.legCost("spot", req.Spot, req.Qty, p90)
	if err != nil {
		return domain.ExecutionCostEstimate{}, err
	}
	perpCost, err := m.legCost("perp", req.Perp, req.Qty, p90)
	if err != nil {
		return domain.ExecutionCostEstimate{}, err
	}

	est := domain.ExecutionCostEstimate{
		SpotCost:    spotCost,
		PerpCost:    perpCost,
		Model:       costModelName,
		Calibration: m.cal.Artifact,
		Tactic:      req.Tactic,
		Qty:         req.Qty,
	}
	if err := est.Validate(); err != nil {
		return domain.ExecutionCostEstimate{}, fmt.Errorf("estimate cost: %w", err)
	}
	return est, nil
}

// p90For selects the bucket for qty: the first whose MaxQty is at or above
// it (the bound is inclusive). A quantity past the last bucket is a size the
// artifact never measured — refused, never extrapolated.
func (m *ConservativeCostModel) p90For(qty fixedpoint.Quantity) (fixedpoint.Rate, error) {
	for _, b := range m.cal.Buckets {
		c, err := qty.Cmp(b.MaxQty)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("estimate cost bucket: %w", err)
		}
		if c <= 0 {
			return b.P90, nil
		}
	}
	last := m.cal.Buckets[len(m.cal.Buckets)-1].MaxQty
	return fixedpoint.Rate{}, fmt.Errorf("estimate cost qty %s past calibrated max %s: %w",
		qty, last, ErrSizeOutsideCalibration)
}

// legCost prices one leg: the worse side's worse branch.
func (m *ConservativeCostModel) legCost(leg string, inst ids.VenueInstrument, qty fixedpoint.Quantity, p90 fixedpoint.Rate) (fixedpoint.Money, error) {
	book, ok := m.books.Snapshot(inst)
	if !ok {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s: %w", leg, inst, ErrBookUnavailable)
	}
	if err := book.Validate(); err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg book: %w", leg, err)
	}
	if book.Instrument != inst {
		return fixedpoint.Money{}, fmt.Errorf("%s leg asked %s, book holds %s: %w",
			leg, inst, book.Instrument, domain.ErrLegsMismatch)
	}

	askCost, err := m.sideCost(leg, "ask", book.Asks, qty, p90)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	bidCost, err := m.sideCost(leg, "bid", book.Bids, qty, p90)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	c, err := askCost.Cmp(bidCost)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg: %w", leg, err)
	}
	if c >= 0 {
		return askCost, nil
	}
	return bidCost, nil
}

// sideCost prices one side of one leg: max(walk premium + adverse charge,
// p90 charge), both against the side's best-level notional.
//
// The walk premium is |Σ level fills − best·qty| with the roundings chosen
// so the premium can only be overstated: fills round away from zero on the
// way through the ladder, the best-price reference rounds toward zero for
// the aggressing comparison. levels[0] is the side's best by BookSnapshot's
// ordering invariant, whichever side this is.
func (m *ConservativeCostModel) sideCost(leg, side string, levels []mdomain.BookLevel, qty fixedpoint.Quantity, p90 fixedpoint.Rate) (fixedpoint.Money, error) {
	if len(levels) == 0 {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s side empty: %w", leg, side, ErrDepthInsufficient)
	}
	best := levels[0].Price

	remaining := qty
	filled := fixedpoint.ZeroMoney(best.Quote())
	for _, l := range levels {
		sign, err := remaining.Sign()
		if err != nil {
			return fixedpoint.Money{}, fmt.Errorf("%s leg %s walk: %w", leg, side, err)
		}
		if sign == 0 {
			break
		}
		take := l.Quantity
		if c, err := remaining.Cmp(l.Quantity); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("%s leg %s walk: %w", leg, side, err)
		} else if c < 0 {
			take = remaining
		}
		fill, err := fixedpoint.Notional(l.Price, take, fixedpoint.RoundAwayFromZero)
		if err != nil {
			return fixedpoint.Money{}, fmt.Errorf("%s leg %s fill: %w", leg, side, err)
		}
		if filled, err = filled.Add(fill); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("%s leg %s fill: %w", leg, side, err)
		}
		if remaining, err = remaining.Sub(take); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("%s leg %s walk: %w", leg, side, err)
		}
	}
	if sign, err := remaining.Sign(); err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s walk: %w", leg, side, err)
	} else if sign != 0 {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s depth short by %s: %w",
			leg, side, remaining, ErrDepthInsufficient)
	}

	ref, err := fixedpoint.Notional(best, qty, fixedpoint.RoundTowardZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s reference: %w", leg, side, err)
	}
	diff, err := filled.Sub(ref)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s premium: %w", leg, side, err)
	}
	premium, err := diff.Abs()
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s premium: %w", leg, side, err)
	}

	refUp, err := fixedpoint.Notional(best, qty, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s notional: %w", leg, side, err)
	}
	adverse, err := refUp.Mul(m.cal.AdverseMove, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s adverse: %w", leg, side, err)
	}
	walk, err := premium.Add(adverse)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s walk branch: %w", leg, side, err)
	}

	p90Charge, err := refUp.Mul(p90, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s p90 branch: %w", leg, side, err)
	}

	c, err := walk.Cmp(p90Charge)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s: %w", leg, side, err)
	}
	if c >= 0 {
		return walk, nil
	}
	return p90Charge, nil
}
