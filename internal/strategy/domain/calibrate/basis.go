package calibrate

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// KlinePoint is one point of a venue-published price series at kline
// granularity: the instant is the kline's open time, the price its open —
// the price observable AT that instant, which keeps the series causal in
// the same sense the residual collection is.
type KlinePoint struct {
	At    time.Time
	Price fixedpoint.Price
}

// AdverseBasisMove freezes the hold-scale basis quantity (§41b family 1,
// the §41b-note's adverse_basis_move — seconds-scale adverse_fill_move is
// family 2's, one letter apart and a different world): the high quantile
// of the RELATIVE basis rise over hold-length horizons, measured across
// every horizon-separated pair in the calibration era.
//
// # The rule, stated
//
// The basis at instant t is mark minus spot, priced per unit of the base
// asset; the move over horizon h is (basis(t+h) − basis(t)) / spot(t) — a
// dimensionless rate, because only a relative move transfers across an
// era in which the underlying's price tripled, and because the runtime
// consumer (NewConservativeBasis) charges exactly rate × notional. A RISE
// is the adverse direction for the carry (entry sells the basis, exit
// buys it back), so the frozen number is the q-th quantile of the move
// distribution at the UPPER order statistic. A negative quantile — an era
// in which even the adverse tail converged — freezes as a DECLARED zero:
// the conservative model never credits basis, and NewConservativeBasis
// refuses a negative charge outright.
//
// Both series must describe the same instants on a uniform step, and the
// horizon must be a whole number of steps — the move would otherwise need
// instants nobody measured.
func AdverseBasisMove(spot, mark []KlinePoint, horizon time.Duration, q fixedpoint.Rate) (fixedpoint.Rate, error) {
	if err := validateSeries(spot, mark); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse basis: %w", err)
	}
	if err := validateGrid([]fixedpoint.Rate{q}); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse basis quantile: %w", err)
	}
	step := spot[1].At.Sub(spot[0].At)
	if horizon <= 0 || horizon%step != 0 {
		return fixedpoint.Rate{}, fmt.Errorf("horizon %v over step %v: %w", horizon, step, ErrHorizonOffGrid)
	}
	s := int(horizon / step)
	if s >= len(spot) {
		return fixedpoint.Rate{}, fmt.Errorf("horizon %v spans %d steps, series has %d rows: %w",
			horizon, s, len(spot), ErrHistoryShort)
	}

	unit, err := fixedpoint.ParseQuantity(spot[0].Price.Base(), "1")
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse basis unit: %w", err)
	}
	// Per-unit legs: Notional(price, 1) is exact, so basis differences
	// carry no rounding until the one division below.
	basisAt := func(i int) (fixedpoint.Money, fixedpoint.Money, error) {
		markM, err := fixedpoint.Notional(mark[i].Price, unit, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Money{}, fixedpoint.Money{}, err
		}
		spotM, err := fixedpoint.Notional(spot[i].Price, unit, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Money{}, fixedpoint.Money{}, err
		}
		basis, err := markM.Sub(spotM)
		return basis, spotM, err
	}

	moves := make([]fixedpoint.Rate, 0, len(spot)-s)
	for i := 0; i+s < len(spot); i++ {
		b0, spot0, err := basisAt(i)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("adverse basis at %s: %w", spot[i].At.Format(time.RFC3339), err)
		}
		b1, _, err := basisAt(i + s)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("adverse basis at %s: %w", spot[i+s].At.Format(time.RFC3339), err)
		}
		diff, err := b1.Sub(b0)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("adverse basis move at %s: %w", spot[i].At.Format(time.RFC3339), err)
		}
		// A measurement, not a charge: half-even, the valuation rounding.
		// Conservatism enters at the selection below, once, visibly.
		move, err := diff.Div(spot0, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("adverse basis move at %s: %w", spot[i].At.Format(time.RFC3339), err)
		}
		moves = append(moves, move)
	}

	idx, err := ceilIndex(q, len(moves))
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse basis quantile: %w", err)
	}
	picked := sortRates(moves)[idx]
	sign, err := picked.Sign()
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("adverse basis: %w", err)
	}
	if sign < 0 {
		return fixedpoint.ZeroRate(), nil
	}
	return picked, nil
}

// validateSeries demands two aligned kline series: same length ≥ 2, same
// instants pairwise, valid prices on one asset pair, strictly ascending
// on a uniform step.
func validateSeries(spot, mark []KlinePoint) error {
	if len(spot) < 2 {
		return fmt.Errorf("%d spot rows: %w", len(spot), domain.ErrWindowEmpty)
	}
	if len(mark) != len(spot) {
		return fmt.Errorf("%d spot rows, %d mark rows: %w", len(spot), len(mark), ErrSeriesMismatch)
	}
	step := spot[1].At.Sub(spot[0].At)
	if step <= 0 {
		return fmt.Errorf("rows 0,1 %v apart: %w", step, domain.ErrNotAscending)
	}
	for i := range spot {
		if spot[i].At.IsZero() {
			return fmt.Errorf("spot row %d instant: %w", i, domain.ErrUnset)
		}
		if !mark[i].At.Equal(spot[i].At) {
			return fmt.Errorf("row %d spot %s, mark %s: %w",
				i, spot[i].At.Format(time.RFC3339), mark[i].At.Format(time.RFC3339), ErrSeriesMismatch)
		}
		if err := spot[i].Price.Validate(); err != nil {
			return fmt.Errorf("spot row %d price: %w", i, err)
		}
		if err := mark[i].Price.Validate(); err != nil {
			return fmt.Errorf("mark row %d price: %w", i, err)
		}
		if spot[i].Price.Base() != spot[0].Price.Base() || spot[i].Price.Quote() != spot[0].Price.Quote() ||
			mark[i].Price.Base() != spot[0].Price.Base() || mark[i].Price.Quote() != spot[0].Price.Quote() {
			return fmt.Errorf("row %d assets: %w", i, ErrSeriesMismatch)
		}
		if i > 0 && spot[i].At.Sub(spot[i-1].At) != step {
			return fmt.Errorf("row %d %v after row %d, step is %v: %w",
				i, spot[i].At.Sub(spot[i-1].At), i-1, step, ErrSeriesMismatch)
		}
	}
	return nil
}
