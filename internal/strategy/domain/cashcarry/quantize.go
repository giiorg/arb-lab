package cashcarry

import (
	"fmt"
	"math"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// TargetQty is §2.4's leg quantization: the candidate quantity rounded DOWN
// to the intersection of both legs' lot grids, with every venue floor
// checked. It is exported because two callers need the same answer — Enter
// uses it inside the verdict, and the application service uses it to size the
// open Intent a VerdictEnter produces — and two implementations of "the same"
// rounding is how a journaled decision and its Intent come to disagree by one
// lot step.
//
// The rounding direction is toward zero, always: sizing UP would order
// quantity the candidate never asked for, and §2.4 spells the leftover rule —
// residual delta beyond the declared tolerance is a sizing FAILURE, never
// silently accepted.
//
// The venue floors are judged conservatively: MinQty against the quantized
// target directly, MinNotional against the target priced at each leg's BID —
// the understated notional, so a pass here implies the venue's own check
// passes at any executable price.
func TargetQty(in domain.DecisionInput) (fixedpoint.Quantity, error) {
	step, err := intersectionStep(in.SpotLots.Step, in.PerpLots.Step)
	if err != nil {
		return fixedpoint.Quantity{}, err
	}

	// Floor the candidate onto the intersection grid. Mantissa arithmetic is
	// exact: both values are validated positive scale-9 quantities of one
	// asset, and n*step never exceeds the candidate it was floored from.
	lots := in.CandidateQty.Mantissa() / step.Mantissa()
	target, err := fixedpoint.QuantityFromMantissa(in.CandidateQty.Asset(), lots*step.Mantissa())
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("quantize target: %w", err)
	}

	residual, err := in.CandidateQty.Sub(target)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("quantize residual: %w", err)
	}
	if c, err := residual.Cmp(in.ResidualTolerance); err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("quantize residual: %w", err)
	} else if c > 0 {
		return fixedpoint.Quantity{}, fmt.Errorf("residual %s over tolerance %s: %w",
			residual, in.ResidualTolerance, ErrResidualExceeded)
	}

	for _, leg := range []struct {
		name string
		lots domain.LotRules
		bid  fixedpoint.Price
	}{
		{"spot", in.SpotLots, in.View.Spot.Bid},
		{"perp", in.PerpLots, in.View.Perp.Bid},
	} {
		if c, err := target.Cmp(leg.lots.MinQty); err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("quantize %s min qty: %w", leg.name, err)
		} else if c < 0 {
			return fixedpoint.Quantity{}, fmt.Errorf("%s target %s under min qty %s: %w",
				leg.name, target, leg.lots.MinQty, ErrBelowMinQty)
		}
		notional, err := fixedpoint.Notional(leg.bid, target, fixedpoint.RoundTowardZero)
		if err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("quantize %s notional: %w", leg.name, err)
		}
		if c, err := notional.Cmp(leg.lots.MinNotional); err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("quantize %s min notional: %w", leg.name, err)
		} else if c < 0 {
			return fixedpoint.Quantity{}, fmt.Errorf("%s notional %s under min %s: %w",
				leg.name, notional, leg.lots.MinNotional, ErrBelowMinNotional)
		}
	}
	return target, nil
}

// intersectionStep is the least common multiple of the two legs' lot steps —
// the coarsest grid every member of which sits on BOTH venues' grids. The
// intersection of two lot-size sets is exactly the multiples of the LCM,
// which is why this is the §2.4 rule and not an approximation of it.
func intersectionStep(a, b fixedpoint.Quantity) (fixedpoint.Quantity, error) {
	am, bm := a.Mantissa(), b.Mantissa()
	g := gcd(am, bm)
	// lcm = (a/g)*b, guarded: a step grid nobody can trade is a refusal, not
	// a wrap-around.
	if q := am / g; q > math.MaxInt64/bm {
		return fixedpoint.Quantity{}, fmt.Errorf("steps %s and %s: %w", a, b, ErrLotIncompatible)
	}
	return fixedpoint.QuantityFromMantissa(a.Asset(), (am/g)*bm)
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
