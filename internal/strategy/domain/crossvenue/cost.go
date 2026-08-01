package crossvenue

// The §8 conservative cost selections, named as rules (§45e: costs are
// measured, per symbol, from our books — no BTC figure transfers). The
// calibrate convention carries over: measurements round half-even, and the
// conservative direction is applied ONCE, visibly, at each selection —
// WorseOf between the two measurement methods, WorseSide between a venue's
// two books sides — never smeared through the arithmetic.

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// WorseOf is §8's selection between the two measurement methods: the worse
// (larger, adverse-positive) of the depth-walked fill cost for THIS order
// and the measured p90 cell conditional on size. A negative operand is a
// real measurement (the book improved) and survives selection — clamping
// belongs nowhere, and refusal of a negative BOUND is the round-trip
// assembly's rule, where charging is the act.
func WorseOf(walked, p90 fixedpoint.Rate) (fixedpoint.Rate, error) {
	if err := walked.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-of walked: %w", err)
	}
	if err := p90.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-of p90: %w", err)
	}
	c, err := walked.Cmp(p90)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-of: %w", err)
	}
	if c >= 0 {
		return walked, nil
	}
	return p90, nil
}

// WorseSide is the per-venue side selection: the worse of the two sides'
// cost cells, charged for both of that venue's transitions. Entering
// crosses one side and exiting crosses the other, but which side the entry
// crosses depends on the episode's direction — charging the worse side
// twice is the direction that is right when nobody chose. The two venues
// select INDEPENDENTLY: Binance's worse side may be the bids while
// Bybit's is the asks.
func WorseSide(bid, ask fixedpoint.Rate) (fixedpoint.Rate, error) {
	if err := bid.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-side bid: %w", err)
	}
	if err := ask.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-side ask: %w", err)
	}
	c, err := bid.Cmp(ask)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("worse-side: %w", err)
	}
	if c >= 0 {
		return bid, nil
	}
	return ask, nil
}

// RoundTrip assembles one episode's full charge as a fraction of one leg's
// notional: two taker legs plus two slippage-bound crossings on each
// venue — 2×(bnFee + bnSlip) + 2×(byFee + bySlip). Charging is the act
// here, so every cell must be non-negative: a negative fee or bound would
// CREDIT the trip, and an unpopulated cell reading as free is exactly the
// zero-value failure this repo refuses by construction.
func RoundTrip(bnFee, bnSlip, byFee, bySlip fixedpoint.Rate) (fixedpoint.Rate, error) {
	total := fixedpoint.ZeroRate()
	for _, cell := range []struct {
		name string
		r    fixedpoint.Rate
	}{
		{"binance fee", bnFee},
		{"binance slippage bound", bnSlip},
		{"bybit fee", byFee},
		{"bybit slippage bound", bySlip},
	} {
		if err := cell.r.Validate(); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("round trip %s: %w", cell.name, err)
		}
		s, err := cell.r.Sign()
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("round trip %s: %w", cell.name, err)
		}
		if s < 0 {
			return fixedpoint.Rate{}, fmt.Errorf("round trip %s %s: %w", cell.name, cell.r.Decimal(), ErrCostNegative)
		}
		once, err := total.Add(cell.r)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("round trip %s: %w", cell.name, err)
		}
		total, err = once.Add(cell.r) // each cell is crossed twice
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("round trip %s: %w", cell.name, err)
		}
	}
	return total, nil
}
