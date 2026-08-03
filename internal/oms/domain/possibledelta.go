package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// PossibleDelta is §6.7's "possible-delta INTERVAL, not a point": the closed
// range of base-asset exposure an order — or a whole hedge group — may still
// move to.
//
// # One order's interval always has zero as an endpoint, and that is a fact
// # about orders rather than a simplification
//
// A single order executes in ONE direction. A buy can only raise the delta, so
// its interval is [0, +remaining]; a sell can only lower it, so it is
// [−remaining, 0]. Neither straddles zero. **Straddling is a property of the
// SUM**, which is exactly where §6.7 forbids compensation — "if that interval
// does not straddle zero … the endpoint nearest zero is naked in every
// reachable state".
//
// The alternative encoding — a free (lo, hi) per order — was tried in formal
// model (a) and rejected as both unfaithful and unbounded (§46 §7a decision 1):
// it admits orders that can execute either way, which no venue offers, and its
// reachable state space has no ceiling.
//
// # The zero value is unusable
//
// Both endpoints are fixedpoint.Quantity, whose zero value carries no asset tag
// and fails Validate. An unpopulated interval therefore refuses rather than
// reading as the point zero, which is the reading that would let a live order
// contribute nothing.
type PossibleDelta struct {
	lo fixedpoint.Quantity
	hi fixedpoint.Quantity
}

// NoPossibleDelta is the point zero for an asset: what an order contributes when
// it cannot move the venue's delta any further.
//
// It is a real answer rather than a missing one. §6.7's ambiguity test is about
// orders with "remaining executable OR UNOBSERVED quantity", and the states that
// have neither — the four terminals, and Planned, which §5.3 says "was never
// dispatched (no venue order can exist)" — contribute exactly this.
func NoPossibleDelta(asset fixedpoint.Asset) PossibleDelta {
	zero := fixedpoint.ZeroQuantity(asset)
	return PossibleDelta{lo: zero, hi: zero}
}

// PossibleDeltaFor derives one order's interval from its side and its remaining
// executable quantity.
//
// # A NEGATIVE remainder is clamped to zero here, and NOT clamped in the residual
//
// Ledger.Remaining is signed and may be negative — a venue over-execution — and
// its own doc says clamping it at zero "would report a package as flat while it
// holds reverse exposure". That is true of the RESIDUAL and false of this
// interval, and the two consumers of the same number want opposite treatments:
//
//   - The residual must carry the overshoot. It is realized exposure, it is in
//     the fill ledger, and it moves the compensating trade's size.
//   - This interval must not. An execution cannot be un-executed, so an
//     over-filled order's remaining EXECUTABLE quantity is zero and its future
//     contribution is the point zero.
//
// Unclamped, a buy order over-filled by 0.1 would contribute [0, −0.1] — an
// inverted interval handing a buy a SHORT endpoint it can never reach. Summed,
// that widens the group's range toward straddling, and a straddling range is
// where §6.7 permits nothing but de-minimis. So the unclamped reading does not
// produce a wrong trade; it produces a group that cannot be compensated at all,
// for a reason no operator could find from the numbers.
//
// Both readings compile and both produce a plausible interval, which is why the
// distinction is written here rather than left to the call site.
func PossibleDeltaFor(side Side, remaining fixedpoint.Quantity) (PossibleDelta, error) {
	if err := side.Validate(); err != nil {
		return PossibleDelta{}, err
	}
	if err := remaining.Validate(); err != nil {
		return PossibleDelta{}, fmt.Errorf("remaining quantity: %w", err)
	}
	sign, err := remaining.Sign()
	if err != nil {
		return PossibleDelta{}, fmt.Errorf("remaining quantity: %w", err)
	}
	zero := fixedpoint.ZeroQuantity(remaining.Asset())
	if sign <= 0 {
		return PossibleDelta{lo: zero, hi: zero}, nil
	}

	switch side {
	case SideBuy:
		return PossibleDelta{lo: zero, hi: remaining}, nil
	case SideSell:
		negated, err := remaining.Neg()
		if err != nil {
			return PossibleDelta{}, fmt.Errorf("remaining quantity: %w", err)
		}
		return PossibleDelta{lo: negated, hi: zero}, nil
	}
	// Reachable only from a DECLARED side with no case above — §3.6 rule 10's
	// failure arriving at runtime. Unconditional rather than a trailing return,
	// on this package's established argument: a trailing return puts one group's
	// members beyond the reach of any test.
	return PossibleDelta{}, fmt.Errorf("%q: %w", string(side), ErrSideUnhandled)
}

// Lo and Hi are the endpoints.
func (p PossibleDelta) Lo() fixedpoint.Quantity { return p.lo }
func (p PossibleDelta) Hi() fixedpoint.Quantity { return p.hi }

// Validate reports whether the interval is well-formed: both endpoints usable,
// same asset, and lo not above hi.
func (p PossibleDelta) Validate() error {
	if err := p.lo.Validate(); err != nil {
		return fmt.Errorf("interval low endpoint: %w", err)
	}
	if err := p.hi.Validate(); err != nil {
		return fmt.Errorf("interval high endpoint: %w", err)
	}
	order, err := p.lo.Cmp(p.hi)
	if err != nil {
		return fmt.Errorf("interval endpoints: %w", err)
	}
	if order > 0 {
		return fmt.Errorf("interval [%s, %s]: %w", p.lo, p.hi, ErrIntervalInverted)
	}
	return nil
}

// Add sums two intervals endpoint-wise — §6.7's "the group's net exposure lies
// in a summed interval".
func (p PossibleDelta) Add(o PossibleDelta) (PossibleDelta, error) {
	lo, err := p.lo.Add(o.lo)
	if err != nil {
		return PossibleDelta{}, fmt.Errorf("summing interval low endpoints: %w", err)
	}
	hi, err := p.hi.Add(o.hi)
	if err != nil {
		return PossibleDelta{}, fmt.Errorf("summing interval high endpoints: %w", err)
	}
	return PossibleDelta{lo: lo, hi: hi}, nil
}

// Shift translates the interval by a fixed quantity — how the group's realized
// residual and its summed possible movement become one range.
func (p PossibleDelta) Shift(by fixedpoint.Quantity) (PossibleDelta, error) {
	lo, err := p.lo.Add(by)
	if err != nil {
		return PossibleDelta{}, fmt.Errorf("shifting interval low endpoint: %w", err)
	}
	hi, err := p.hi.Add(by)
	if err != nil {
		return PossibleDelta{}, fmt.Errorf("shifting interval high endpoint: %w", err)
	}
	return PossibleDelta{lo: lo, hi: hi}, nil
}

// IsPoint reports whether the interval has collapsed — both endpoints equal.
//
// A collapsed interval is what §6.7 means by "every possible-delta interval has
// collapsed to a FIXED residual". It is not the same question as "is the
// residual zero": a group can be certainly short by exactly 1, which is a point
// and is precisely the case the determined branch exists to compensate.
func (p PossibleDelta) IsPoint() (bool, error) {
	order, err := p.lo.Cmp(p.hi)
	if err != nil {
		return false, fmt.Errorf("interval endpoints: %w", err)
	}
	return order == 0, nil
}

// Straddles reports whether the interval contains zero in its INTERIOR —
// §6.7's "a < 0 < b", where "no nonzero compensation is safe in all states →
// de-minimis only".
//
// The interior is the right test and the boundary cases are why. An interval of
// [0, 1] does not straddle: its nearest-zero endpoint is zero, so the floor rule
// issues nothing, and treating it as straddling would reach the same action by a
// route that also mislabels [0, 0] — a fully collapsed, flat group — as
// uncertain. An interval of [−1, 0] likewise: guaranteed net-short or flat, and
// the nearest-zero endpoint is again zero.
func (p PossibleDelta) Straddles() (bool, error) {
	loSign, err := p.lo.Sign()
	if err != nil {
		return false, fmt.Errorf("interval low endpoint: %w", err)
	}
	hiSign, err := p.hi.Sign()
	if err != nil {
		return false, fmt.Errorf("interval high endpoint: %w", err)
	}
	return loSign < 0 && hiSign > 0, nil
}

// FloorReduction is the signed exposure change §6.7's interval-floor rule
// permits: "the endpoint NEAREST ZERO is naked in every reachable state, so
// reducing exposure by exactly that magnitude can never overshoot into the
// opposite sign under any resolution".
//
// It returns the change to APPLY, so its sign is the opposite of the endpoint
// it is derived from: a guaranteed-net-long group ([a, b] with a ≥ 0) yields
// −a, a sale; a guaranteed-net-short one (b ≤ 0) yields −b, a buy. A straddling
// interval yields an error rather than zero, because "no safe action" and "the
// safe action happens to be nothing" are different answers and only one of them
// may be reported to an operator as compliance.
//
// # Why the NEAREST endpoint and not the worst one
//
// Model (a)'s NC1 is the control that flips exactly this, and it reproduces
// R1d-2's asymmetry: a residual of +1 with a pending sell of 1 gives a summed
// interval of [0, 1] — the near endpoint is ZERO, because the pending hedge may
// already cover the whole position. Nearest-zero issues nothing; worst-endpoint
// sells 1 and lands the group at −1, a naked SHORT created by the rule that
// exists to prevent one.
//
// The un-compensated remainder is bounded by max(|a|, |b|) — the interval's
// worst-magnitude endpoint, NOT |b| — and covering it is §6 step 4's gap budget,
// which is money and is not this type's. Stated because the v0.7.1 phrasing had
// |b| and was correct only for the net-long case.
func (p PossibleDelta) FloorReduction() (fixedpoint.Quantity, error) {
	straddles, err := p.Straddles()
	if err != nil {
		return fixedpoint.Quantity{}, err
	}
	if straddles {
		return fixedpoint.Quantity{}, fmt.Errorf("interval [%s, %s]: %w", p.lo, p.hi, ErrIntervalStraddlesZero)
	}
	loSign, err := p.lo.Sign()
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("interval low endpoint: %w", err)
	}
	if loSign >= 0 {
		// Guaranteed net-long or flat: the nearest-zero endpoint is lo.
		return p.lo.Neg()
	}
	// Not straddling and lo < 0, so hi ≤ 0: guaranteed net-short or flat, and
	// the nearest-zero endpoint is hi.
	return p.hi.Neg()
}

func (p PossibleDelta) String() string {
	return fmt.Sprintf("[%s, %s]", p.lo, p.hi)
}
