package crossvenue

// The standing depth walk (§45c's "depth-cost bound", the walked half of
// WorseOf): fill the candidate quantity through ONE book instant's side,
// best-first, and measure the fill against that same instant's best price.
// It is calibrate.CounterfactualSlippage's arithmetic with both instants
// the same — deliberately NOT expressed by calling it, because the pair
// validation there demands a later instant strictly after the first, and
// satisfying a guard with a forged watermark is not this repo's move. The
// oracle beside this file is what keeps two walks honest instead of one
// guard dishonest.
//
// Measurement arithmetic rounds half-even (calibrate's stated convention);
// conservatism enters at the WorseOf/WorseSide selections, never here.

import (
	"fmt"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// WalkedFill is the standing depth-walked slippage, one sample:
// adverse-positive on both sides — walking the asks (a buy), slippage is
// (paid − best·qty)/(best·qty); walking the bids (a sell), it is
// (best·qty − received)/(best·qty). A book too thin to fill the quantity
// is ErrDepthShort: the sample measures nothing, and whether a thin
// instant is excluded-and-counted or refuses the episode is the walker's
// committed rule, not a silent zero here.
func WalkedFill(book mdomain.BookSnapshot, side mdomain.BookSide, qty fixedpoint.Quantity) (fixedpoint.Rate, error) {
	if err := book.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill book: %w", err)
	}
	if err := qty.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill candidate: %w", err)
	}
	if sign, err := qty.Sign(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill candidate: %w", err)
	} else if sign <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill candidate %s: %w", qty, ErrCandidateInvalid)
	}

	levels, err := book.Side(side)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
	}
	if len(levels) == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("%s side empty: %w", side, ErrDepthShort)
	}

	ref, err := fixedpoint.Notional(levels[0].Price, qty, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill reference: %w", err)
	}

	remaining := qty
	filled := fixedpoint.ZeroMoney(levels[0].Price.Quote())
	for _, l := range levels {
		sign, err := remaining.Sign()
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
		}
		if sign == 0 {
			break
		}
		take := l.Quantity
		if c, err := remaining.Cmp(l.Quantity); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
		} else if c < 0 {
			take = remaining
		}
		fill, err := fixedpoint.Notional(l.Price, take, fixedpoint.RoundHalfEven)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
		}
		if filled, err = filled.Add(fill); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
		}
		if remaining, err = remaining.Sub(take); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
		}
	}
	if sign, err := remaining.Sign(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill: %w", err)
	} else if sign != 0 {
		return fixedpoint.Rate{}, fmt.Errorf("short by %s on %s: %w", remaining, side, ErrDepthShort)
	}

	var adverse fixedpoint.Money
	switch side {
	case mdomain.SideAsk:
		adverse, err = filled.Sub(ref)
	case mdomain.SideBid:
		adverse, err = ref.Sub(filled)
	default:
		return fixedpoint.Rate{}, fmt.Errorf("walked fill side %q: %w", string(side), ErrSideInvalid)
	}
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill premium: %w", err)
	}
	slip, err := adverse.Div(ref, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("walked fill ratio: %w", err)
	}
	return slip, nil
}
