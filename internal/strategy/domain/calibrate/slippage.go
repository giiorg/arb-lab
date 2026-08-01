package calibrate

import (
	"fmt"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// This file is family 2's pure measurement math (§41b-ii): the defined
// counterfactual slippage observation, the execution-scale adverse move,
// the upper-statistic selection the cells freeze, and the two-family
// dispersion merge. The archive reading, sampling cadence and pairing rule
// live in the driving adapter; what lives here is what a number MEANS once
// two book instants are in hand.
//
// # Measurements round half-even; conservatism enters at selection
//
// AdverseBasisMove's stated convention, kept: every observation here is a
// measurement, so its arithmetic uses the valuation rounding, and the
// conservative direction is applied ONCE, visibly, at the quantile
// selection (the upper order statistic). The runtime adapter's
// away-from-zero chain is CHARGE arithmetic — same numbers, different act.

// CounterfactualSlippage is §41b-ii-i's defined observation, one sample:
// walk the candidate quantity through the book at t+latency on the same
// side, and measure against the best price at t — the passive
// counterfactual for depth that vanished over the latency window.
//
// The sign convention is adverse-positive on BOTH sides: walking the asks
// (a buy), slippage is (paid − best-at-t·qty) / reference; walking the
// bids (a sell), it is (reference − received) / reference. A negative
// sample — the book improved over the window — is a real observation and
// stays in the distribution; the p90 selection is where the tail is read.
//
// A later book too thin to fill the quantity is ErrDepthShort: the sample
// measures nothing (the walk has no counterfactual fill), and whether a
// thin instant is excluded-and-counted or fails the cell is the committed
// procedure's call, made where the samples are collected — not silently
// zero here, which would put the thinnest books at the BOTTOM of the
// distribution the p90 reads.
func CounterfactualSlippage(at, later mdomain.BookSnapshot, side mdomain.BookSide, qty fixedpoint.Quantity) (fixedpoint.Rate, error) {
	if err := validateBookPair(at, later); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual slippage: %w", err)
	}
	if err := validateCandidate(qty); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual slippage: %w", err)
	}

	best, err := bestPrice(at, side)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual slippage at t: %w", err)
	}
	ref, err := fixedpoint.Notional(best, qty, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual reference: %w", err)
	}

	walked, err := walkSide(later, side, qty)
	if err != nil {
		return fixedpoint.Rate{}, err
	}

	// Adverse-positive on both sides: buys pay more, sells receive less.
	var adverse fixedpoint.Money
	switch side {
	case mdomain.SideAsk:
		adverse, err = walked.Sub(ref)
	case mdomain.SideBid:
		adverse, err = ref.Sub(walked)
	default:
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual side %q: %w", string(side), domain.ErrSideInvalid)
	}
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual premium: %w", err)
	}
	slip, err := adverse.Div(ref, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("counterfactual ratio: %w", err)
	}
	return slip, nil
}

// AdverseFillMove is one sample of family 2's execution-scale move
// (§41b-note: seconds, prices a fill — the hold-scale adverse_basis_move
// is another world): the relative move of the side's best price over the
// latency window, signed adverse-positive for an order consuming that side
// — asks rising and bids falling are adverse. Favorable moves stay
// negative in the distribution; the high-quantile selection reads the
// adverse tail.
func AdverseFillMove(at, later mdomain.BookSnapshot, side mdomain.BookSide) (fixedpoint.Rate, error) {
	if err := validateBookPair(at, later); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move: %w", err)
	}
	bestAt, err := bestPrice(at, side)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move at t: %w", err)
	}
	bestLater, err := bestPrice(later, side)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move at t+latency: %w", err)
	}

	unit, err := fixedpoint.ParseQuantity(bestAt.Base(), "1")
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move unit: %w", err)
	}
	atM, err := fixedpoint.Notional(bestAt, unit, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move: %w", err)
	}
	laterM, err := fixedpoint.Notional(bestLater, unit, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move: %w", err)
	}

	var diff fixedpoint.Money
	switch side {
	case mdomain.SideAsk:
		diff, err = laterM.Sub(atM)
	case mdomain.SideBid:
		diff, err = atM.Sub(laterM)
	default:
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move side %q: %w", string(side), domain.ErrSideInvalid)
	}
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move: %w", err)
	}
	move, err := diff.Div(atM, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse fill move ratio: %w", err)
	}
	return move, nil
}

// UpperQuantile selects the q-th quantile of a sample set at the UPPER
// order statistic — ceilIndex's rule, the same selection AdverseBasisMove
// applies and the one the p90 cells freeze: never below the exact rank, so
// the frozen number can only overstate the tail it serves.
func UpperQuantile(samples []fixedpoint.Rate, q fixedpoint.Rate) (fixedpoint.Rate, error) {
	if len(samples) == 0 {
		return fixedpoint.Rate{}, ErrNoResiduals
	}
	for i, s := range samples {
		if err := s.Validate(); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("sample %d: %w", i, err)
		}
	}
	if err := validateGrid([]fixedpoint.Rate{q}); err != nil {
		return fixedpoint.Rate{}, err
	}
	idx, err := ceilIndex(q, len(samples))
	if err != nil {
		return fixedpoint.Rate{}, err
	}
	return sortRates(samples)[idx], nil
}

// MergeAdverse is §41b's committed two-family merge rule: the frozen
// dispersion points are the POINTWISE MORE-ADVERSE of the EWMA residual
// quantiles (family 1) and the imminent-estimate residual quantiles
// (family 2), on one shared grid. "More adverse" follows ShiftAt's own
// outward convention — below the median the more NEGATIVE shift wins, at
// or above it the more POSITIVE — so the merged distribution is
// conservative at every quantile under the exact read-time rule that will
// consume it.
//
// Both sets must sit on the same grid: a merge across two different grids
// would interpolate, and interpolating mints a residual nobody measured
// (Points' own refusal, one level up).
func MergeAdverse(a, b []domain.QuantilePoint) ([]domain.QuantilePoint, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, ErrNoResiduals
	}
	if len(a) != len(b) {
		return nil, fmt.Errorf("%d points against %d: %w", len(a), len(b), ErrMergeMisaligned)
	}
	half, err := fixedpoint.ParseRate("0.5")
	if err != nil {
		panic(err) // programmer constant
	}
	out := make([]domain.QuantilePoint, 0, len(a))
	for i := range a {
		if c, err := a[i].Q.Cmp(b[i].Q); err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		} else if c != 0 {
			return nil, fmt.Errorf("point %d q %s against %s: %w", i, a[i].Q, b[i].Q, ErrMergeMisaligned)
		}
		tail, err := a[i].Q.Cmp(half)
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}
		c, err := a[i].Shift.Cmp(b[i].Shift)
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}
		pick := a[i]
		// Below the median the smaller shift is the more adverse; at or
		// above it, the larger — outward, ShiftAt's own direction.
		if (tail < 0 && c > 0) || (tail >= 0 && c < 0) {
			pick = b[i]
		}
		out = append(out, domain.QuantilePoint{Q: a[i].Q, Shift: pick.Shift})
	}
	return out, nil
}

// bestPrice is the side's best level's price; an empty side has no best
// and the observation cannot exist at this instant.
func bestPrice(book mdomain.BookSnapshot, side mdomain.BookSide) (fixedpoint.Price, error) {
	levels, err := book.Side(side)
	if err != nil {
		return fixedpoint.Price{}, err
	}
	if len(levels) == 0 {
		return fixedpoint.Price{}, fmt.Errorf("%s side empty: %w", side, ErrDepthShort)
	}
	return levels[0].Price, nil
}

// walkSide fills qty through the side's levels best-first at half-even —
// the measured counterfactual fill. Depth running out is ErrDepthShort.
func walkSide(book mdomain.BookSnapshot, side mdomain.BookSide, qty fixedpoint.Quantity) (fixedpoint.Money, error) {
	levels, err := book.Side(side)
	if err != nil {
		return fixedpoint.Money{}, err
	}
	if len(levels) == 0 {
		return fixedpoint.Money{}, fmt.Errorf("%s side empty: %w", side, ErrDepthShort)
	}
	remaining := qty
	filled := fixedpoint.ZeroMoney(levels[0].Price.Quote())
	for _, l := range levels {
		sign, err := remaining.Sign()
		if err != nil {
			return fixedpoint.Money{}, fmt.Errorf("counterfactual walk: %w", err)
		}
		if sign == 0 {
			break
		}
		take := l.Quantity
		if c, err := remaining.Cmp(l.Quantity); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("counterfactual walk: %w", err)
		} else if c < 0 {
			take = remaining
		}
		fill, err := fixedpoint.Notional(l.Price, take, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Money{}, fmt.Errorf("counterfactual fill: %w", err)
		}
		if filled, err = filled.Add(fill); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("counterfactual fill: %w", err)
		}
		if remaining, err = remaining.Sub(take); err != nil {
			return fixedpoint.Money{}, fmt.Errorf("counterfactual walk: %w", err)
		}
	}
	if sign, err := remaining.Sign(); err != nil {
		return fixedpoint.Money{}, fmt.Errorf("counterfactual walk: %w", err)
	} else if sign != 0 {
		return fixedpoint.Money{}, fmt.Errorf("short by %s on %s: %w", remaining, side, ErrDepthShort)
	}
	return filled, nil
}

// validateBookPair demands two validated instants of ONE book: the same
// instrument, the later strictly after — a pair from two instruments, or
// in the wrong order, measures a difference between two different
// questions.
func validateBookPair(at, later mdomain.BookSnapshot) error {
	if err := at.Validate(); err != nil {
		return fmt.Errorf("book at t: %w", err)
	}
	if err := later.Validate(); err != nil {
		return fmt.Errorf("book at t+latency: %w", err)
	}
	if at.Instrument != later.Instrument {
		return fmt.Errorf("books %s and %s: %w", at.Instrument, later.Instrument, ErrBookPairMismatch)
	}
	if !later.Watermark.RecvTime.After(at.Watermark.RecvTime) {
		return fmt.Errorf("later %s not after %s: %w",
			later.Watermark.RecvTime.Format("15:04:05.000"), at.Watermark.RecvTime.Format("15:04:05.000"),
			ErrBookPairMismatch)
	}
	return nil
}

// validateCandidate demands a positive, validated quantity.
func validateCandidate(qty fixedpoint.Quantity) error {
	if err := qty.Validate(); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	sign, err := qty.Sign()
	if err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	if sign <= 0 {
		return fmt.Errorf("candidate %s: %w", qty, domain.ErrUnset)
	}
	return nil
}
