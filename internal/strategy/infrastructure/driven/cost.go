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

// CostCell is one cell of the artifact's p90-slippage table. §8's letter is
// "p90 slippage conditional on order size AND book state", and the key
// names the population the number was measured from: the LEG it prices
// (instrument), the book side the counterfactual walk consumed, the frozen
// book-state stratum, and the quantity bucket. A pooled table is dominated
// by the liquid population exactly where the illiquid leg — or the stressed
// stratum — needs its own answer (§41b-ii).
type CostCell struct {
	// Instrument is the leg this cell prices.
	Instrument ids.VenueInstrument

	// Side is the book side the observation walked.
	Side mdomain.BookSide

	// Stratum names the frozen classifier stratum the observed books were
	// in. It must name a rule the calibration's own classifier declares.
	Stratum string

	// MaxQty is the quantity bucket's inclusive upper bound.
	MaxQty fixedpoint.Quantity

	// P90 is the measured p90 slippage per unit of notional for orders in
	// this cell.
	P90 fixedpoint.Rate

	// Observations counts the measured instants behind the number. A cell
	// below the calibration's MinEvidence is ABSENT by §41b-ii-iii — its
	// presence here is a contradiction, refused at Validate.
	Observations uint64

	// Method identifies the committed observation procedure that produced
	// this cell (§41b-ii-i: with no order path, "measured slippage" is
	// otherwise whatever the implementer meant).
	Method string
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

	// Latency is the artifact's ASSUMED decision-to-arrival upper bound —
	// declared, not measured: no order path exists before milestone 3's
	// OMS, so no decision-to-arrival latency was ever observable (§41b
	// family 3). The declaration is informed by the archive's transport
	// statistics with a stated multiplier, and the adapter's refusal of any
	// LONGER assumption at decision time is what keeps it honest: a live
	// system slower than the assumption refuses rather than flatters.
	//
	// [SPEC EDIT OWED] §8 demands MEASURED decision-to-arrival latency;
	// this field is a declared upper-bound assumption. The owed clause (the
	// §40d modeled-tier shape) authorizes the declared bound for milestone
	// 2's $1K gate and requires a measured order-path latency before
	// capital — lands at slice C's close (§41b family 3, §41j).
	Latency time.Duration

	// Classifier is the frozen book-state classifier (§41b-ii-b): its code
	// is domain code under the procedure commit, its parameters — these —
	// live in the hashed artifact. Calibration and runtime run the same
	// one.
	Classifier domain.StratumClassifier

	// MinEvidence is the declared minimum observation count below which a
	// cell may not exist. It must be positive: a zero minimum readmits
	// every thin-sample number the rule exists to refuse.
	MinEvidence uint64

	// Cells is the p90 table. Within one (instrument, side, stratum) group,
	// MaxQty is strictly ascending in document order.
	Cells []CostCell
}

// costKey is the cell-group key the lookup joins on.
type costKey struct {
	inst    ids.VenueInstrument
	side    mdomain.BookSide
	stratum string
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
	if err := c.Classifier.Validate(); err != nil {
		return fmt.Errorf("cost calibration classifier: %w", err)
	}
	if c.MinEvidence == 0 {
		return fmt.Errorf("cost calibration min evidence: %w", domain.ErrUnset)
	}
	if len(c.Cells) == 0 {
		return fmt.Errorf("cost calibration cells: %w", domain.ErrUnset)
	}
	strata := map[string]bool{}
	for _, r := range c.Classifier.Rules {
		strata[r.Name] = true
	}
	lastInGroup := map[costKey]fixedpoint.Quantity{}
	for n, cell := range c.Cells {
		if err := cell.Instrument.Validate(); err != nil {
			return fmt.Errorf("cost calibration cell %d instrument: %w", n, err)
		}
		if err := cell.Side.Validate(); err != nil {
			return fmt.Errorf("cost calibration cell %d side: %w", n, err)
		}
		if cell.Stratum == "" {
			return fmt.Errorf("cost calibration cell %d stratum: %w", n, domain.ErrUnset)
		}
		if !strata[cell.Stratum] {
			return fmt.Errorf("cost calibration cell %d stratum %q: %w",
				n, cell.Stratum, ErrCellStratumUnknown)
		}
		if err := cell.MaxQty.Validate(); err != nil {
			return fmt.Errorf("cost calibration cell %d max qty: %w", n, err)
		}
		qsign, err := cell.MaxQty.Sign()
		if err != nil {
			return fmt.Errorf("cost calibration cell %d max qty: %w", n, err)
		}
		if qsign <= 0 {
			return fmt.Errorf("cost calibration cell %d max qty %s: %w", n, cell.MaxQty, domain.ErrUnset)
		}
		if err := cell.P90.Validate(); err != nil {
			return fmt.Errorf("cost calibration cell %d p90: %w", n, err)
		}
		psign, err := cell.P90.Sign()
		if err != nil {
			return fmt.Errorf("cost calibration cell %d p90: %w", n, err)
		}
		if psign < 0 {
			return fmt.Errorf("cost calibration cell %d p90 %s: %w", n, cell.P90, ErrChargeNegative)
		}
		if cell.Observations < c.MinEvidence {
			return fmt.Errorf("cost calibration cell %d observations %d below minimum %d: %w",
				n, cell.Observations, c.MinEvidence, ErrCellThin)
		}
		if cell.Method == "" {
			return fmt.Errorf("cost calibration cell %d method: %w", n, domain.ErrUnset)
		}
		key := costKey{inst: cell.Instrument, side: cell.Side, stratum: cell.Stratum}
		if prev, ok := lastInGroup[key]; ok {
			c2, err := prev.Cmp(cell.MaxQty)
			if err != nil {
				return fmt.Errorf("cost calibration cell %d: %w", n, err)
			}
			if c2 >= 0 {
				return fmt.Errorf("cost calibration cell %d max %s after %s in the same group: %w",
					n, cell.MaxQty, prev, ErrBucketOrder)
			}
		}
		lastInGroup[key] = cell.MaxQty
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

	spotCost, err := m.legCost("spot", req.Spot, req.Qty)
	if err != nil {
		return domain.ExecutionCostEstimate{}, err
	}
	perpCost, err := m.legCost("perp", req.Perp, req.Qty)
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

// BookFeaturesFor measures the classifier's features on one side of a book:
// the relative top-of-book spread (side-less) and the side's total displayed
// base quantity. It is EXPORTED because it is the one implementation of the
// observation both consumers must share (§41b-ii-b): the runtime pricing
// here and the calibration derivation over archived books — two feature
// extractors would let one book land in two strata.
//
// Roundings lean toward the wider-spread reading: the mid rounds toward
// zero and the ratio away from zero, so a boundary-straddling book falls in
// the more-stressed stratum of a monotone ladder — overstating stress
// overstates the charge, which is the conservative direction.
func BookFeaturesFor(book mdomain.BookSnapshot, side mdomain.BookSide) (domain.BookFeatures, error) {
	if len(book.Bids) == 0 {
		return domain.BookFeatures{}, fmt.Errorf("bid side empty: %w", ErrDepthInsufficient)
	}
	if len(book.Asks) == 0 {
		return domain.BookFeatures{}, fmt.Errorf("ask side empty: %w", ErrDepthInsufficient)
	}
	bestBid, bestAsk := book.Bids[0].Price, book.Asks[0].Price

	unit, err := fixedpoint.ParseQuantity(bestBid.Base(), "1")
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread unit: %w", err)
	}
	// Price × 1 is exact by construction; RoundExact states that instead of
	// silently absorbing a future scale change.
	bidN, err := fixedpoint.Notional(bestBid, unit, fixedpoint.RoundExact)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread bid: %w", err)
	}
	askN, err := fixedpoint.Notional(bestAsk, unit, fixedpoint.RoundExact)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread ask: %w", err)
	}
	spreadM, err := askN.Sub(bidN)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread width: %w", err)
	}
	sum, err := askN.Add(bidN)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread mid: %w", err)
	}
	half, err := fixedpoint.ParseRate("0.5")
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread mid: %w", err)
	}
	mid, err := sum.Mul(half, fixedpoint.RoundTowardZero)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread mid: %w", err)
	}
	spread, err := spreadM.Div(mid, fixedpoint.RoundAwayFromZero)
	if err != nil {
		return domain.BookFeatures{}, fmt.Errorf("spread ratio: %w", err)
	}

	var levels []mdomain.BookLevel
	switch side {
	case mdomain.SideBid:
		levels = book.Bids
	case mdomain.SideAsk:
		levels = book.Asks
	default:
		return domain.BookFeatures{}, fmt.Errorf("side %q: %w", string(side), domain.ErrSideInvalid)
	}
	depth := fixedpoint.Quantity{}
	for n, l := range levels {
		if n == 0 {
			depth = l.Quantity
			continue
		}
		if depth, err = depth.Add(l.Quantity); err != nil {
			return domain.BookFeatures{}, fmt.Errorf("depth: %w", err)
		}
	}

	f := domain.BookFeatures{Spread: spread, Depth: depth}
	if err := f.Validate(); err != nil {
		return domain.BookFeatures{}, err
	}
	return f, nil
}

// p90For selects the cell for (instrument, side, stratum, qty): within the
// key's group, the first cell whose MaxQty is at or above the quantity (the
// bound is inclusive). A key with no cells at all is a population the
// artifact never measured (ErrStratumUncalibrated — never a nearest-cell
// guess); a quantity past the group's last bucket is a size it never
// measured (ErrSizeOutsideCalibration) — refused, never extrapolated.
func (m *ConservativeCostModel) p90For(inst ids.VenueInstrument, side mdomain.BookSide, stratum string, qty fixedpoint.Quantity) (fixedpoint.Rate, error) {
	key := costKey{inst: inst, side: side, stratum: stratum}
	found := false
	var last fixedpoint.Quantity
	for _, cell := range m.cal.Cells {
		if (costKey{inst: cell.Instrument, side: cell.Side, stratum: cell.Stratum}) != key {
			continue
		}
		found = true
		last = cell.MaxQty
		c, err := qty.Cmp(cell.MaxQty)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("estimate cost cell: %w", err)
		}
		if c <= 0 {
			return cell.P90, nil
		}
	}
	if !found {
		return fixedpoint.Rate{}, fmt.Errorf("estimate cost %s %s stratum %q: %w",
			inst, side, stratum, ErrStratumUncalibrated)
	}
	return fixedpoint.Rate{}, fmt.Errorf("estimate cost %s %s stratum %q qty %s past calibrated max %s: %w",
		inst, side, stratum, qty, last, ErrSizeOutsideCalibration)
}

// legCost prices one leg: the worse side's worse branch, each side priced
// against the cell its own classified book state selects.
func (m *ConservativeCostModel) legCost(leg string, inst ids.VenueInstrument, qty fixedpoint.Quantity) (fixedpoint.Money, error) {
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

	askCost, err := m.sideCost(leg, book, mdomain.SideAsk, qty)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	bidCost, err := m.sideCost(leg, book, mdomain.SideBid, qty)
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
// p90 charge), both against the side's best-level notional, with the p90
// drawn from the cell this side's CLASSIFIED book state selects — the same
// classifier the calibration ran, on the same features (§41b-ii-b).
//
// The walk premium is |Σ level fills − best·qty| with the roundings chosen
// so the premium can only be overstated: fills round away from zero on the
// way through the ladder, the best-price reference rounds toward zero for
// the aggressing comparison. levels[0] is the side's best by BookSnapshot's
// ordering invariant, whichever side this is.
func (m *ConservativeCostModel) sideCost(leg string, book mdomain.BookSnapshot, side mdomain.BookSide, qty fixedpoint.Quantity) (fixedpoint.Money, error) {
	feat, err := BookFeaturesFor(book, side)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s: %w", leg, side, err)
	}
	stratum, err := m.cal.Classifier.Classify(feat)
	if err != nil {
		return fixedpoint.Money{}, fmt.Errorf("%s leg %s: %w", leg, side, err)
	}
	p90, err := m.p90For(book.Instrument, side, stratum, qty)
	if err != nil {
		return fixedpoint.Money{}, err
	}

	var levels []mdomain.BookLevel
	switch side {
	case mdomain.SideBid:
		levels = book.Bids
	case mdomain.SideAsk:
		levels = book.Asks
	default:
		return fixedpoint.Money{}, fmt.Errorf("%s leg side %q: %w", leg, string(side), domain.ErrSideInvalid)
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
