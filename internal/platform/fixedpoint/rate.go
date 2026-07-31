package fixedpoint

import "fmt"

// Rate is a dimensionless ratio: a funding rate, a fee rate, a leverage factor,
// the quotient of two amounts of the same asset.
//
// Rates are stored as plain ratios, never as percentages — an 0.01% funding
// rate is 0.0001. Nothing in this package multiplies or divides by 100, so a
// percent/ratio mix-up cannot be introduced here; it has to be introduced at
// the venue adapter, where it is one conversion in one place and reviewable.
//
// The zero value is invalid. Rate has no asset tag to hang that on, so it
// carries an explicit construction flag: an unpopulated fee rate would
// otherwise read as *zero fees*, and milestone 2's verdict (§12.2) is a
// comparison of measured carry against a cost hurdle that free trading clears.
// Construct with ParseRate or ZeroRate.
type Rate struct {
	r   int64 // mantissa at the canonical scale
	set bool
}

// Validate reports whether r is a usable value.
func (r Rate) Validate() error {
	if !r.set {
		return fmt.Errorf("rate: %w", ErrRateUnset)
	}
	return nil
}

// ParseRate converts exact decimal text into a Rate.
func ParseRate(s string) (Rate, error) {
	m, err := parseMantissa(s)
	if err != nil {
		return Rate{}, fmt.Errorf("rate: %w", err)
	}
	return Rate{r: m, set: true}, nil
}

// ZeroRate is a constructed zero — genuinely no funding, genuinely no fee — as
// distinct from the zero value, which is a rate nobody ever set.
func ZeroRate() Rate { return Rate{set: true} }

// Decimal renders the ratio as canonical decimal text. This is the codec form.
func (r Rate) Decimal() string { return formatMantissa(r.r) }

// String renders the ratio for logs and wrapped errors, with an unconstructed
// value printing as a placeholder rather than as a plausible zero.
func (r Rate) String() string {
	if !r.set {
		return "<invalid rate>"
	}
	return formatMantissa(r.r)
}

func (r Rate) requireSet(op string) error {
	if !r.set {
		return fmt.Errorf("rate %s: %w", op, ErrRateUnset)
	}
	return nil
}

func binaryRate(op string, a, b Rate) error {
	if !a.set || !b.set {
		return fmt.Errorf("rate %s: %w", op, ErrRateUnset)
	}
	return nil
}

// Add returns r + o, exactly — accumulating funding across settlements, or
// summing a maker fee and a slippage allowance into one cost rate.
func (r Rate) Add(o Rate) (Rate, error) {
	if err := binaryRate("add", r, o); err != nil {
		return Rate{}, err
	}
	s, err := addMantissa(r.r, o.r)
	if err != nil {
		return Rate{}, fmt.Errorf("rate add %s + %s: %w", r, o, err)
	}
	return Rate{r: s, set: true}, nil
}

// Sub returns r - o, exactly — the usual caller being carry minus hurdle.
func (r Rate) Sub(o Rate) (Rate, error) {
	if err := binaryRate("sub", r, o); err != nil {
		return Rate{}, err
	}
	d, err := subMantissa(r.r, o.r)
	if err != nil {
		return Rate{}, fmt.Errorf("rate sub %s - %s: %w", r, o, err)
	}
	return Rate{r: d, set: true}, nil
}

// Neg returns -r: the same funding rate seen from the other side of the trade.
func (r Rate) Neg() (Rate, error) {
	if err := r.requireSet("neg"); err != nil {
		return Rate{}, err
	}
	v, err := negMantissa(r.r)
	if err != nil {
		return Rate{}, fmt.Errorf("rate neg %s: %w", r, err)
	}
	return Rate{r: v, set: true}, nil
}

// Abs returns |r|.
func (r Rate) Abs() (Rate, error) {
	if err := r.requireSet("abs"); err != nil {
		return Rate{}, err
	}
	if r.r >= 0 {
		return r, nil
	}
	v, err := negMantissa(r.r)
	if err != nil {
		return Rate{}, fmt.Errorf("rate abs %s: %w", r, err)
	}
	return Rate{r: v, set: true}, nil
}

// Cmp reports whether r sorts before (-1), with (0) or after (1) o. Comparing
// measured carry against a cost hurdle is milestone 2's entire verdict.
func (r Rate) Cmp(o Rate) (int, error) {
	if err := binaryRate("cmp", r, o); err != nil {
		return 0, err
	}
	return cmpMantissa(r.r, o.r), nil
}

// Sign reports -1, 0 or 1 — which side pays the funding.
func (r Rate) Sign() (int, error) {
	if err := r.requireSet("sign"); err != nil {
		return 0, err
	}
	return signMantissa(r.r), nil
}

// Mul composes two ratios: a fee rate applied per leg across two legs, or a
// funding rate scaled by a holding fraction.
//
// This is the first operation in the package whose result can miss the
// canonical scale — the product of two small ratios has up to eighteen decimal
// places — so it is the first that takes a RoundingMode.
func (r Rate) Mul(o Rate, mode RoundingMode) (Rate, error) {
	if err := binaryRate("mul", r, o); err != nil {
		return Rate{}, err
	}
	v, err := mulDivMantissa(r.r, o.r, scaleFactor, mode)
	if err != nil {
		return Rate{}, fmt.Errorf("rate mul %s * %s: %w", r, o, err)
	}
	return Rate{r: v, set: true}, nil
}

// Div returns r / o: normalizing a weighted sum by its weights, or expressing
// one ratio as a fraction of another. Money.Div's mechanics with both operands
// already dimensionless — the quotient of two ratios is a ratio.
func (r Rate) Div(o Rate, mode RoundingMode) (Rate, error) {
	if err := binaryRate("div", r, o); err != nil {
		return Rate{}, err
	}
	if o.r == 0 {
		return Rate{}, fmt.Errorf("rate div %s by zero: %w", r, ErrDivideByZero)
	}
	// Scale the numerator up before dividing so the quotient lands on the
	// canonical scale rather than being truncated to whole units.
	v, err := mulDivMantissa(r.r, scaleFactor, o.r, mode)
	if err != nil {
		return Rate{}, fmt.Errorf("rate div %s / %s: %w", r, o, err)
	}
	return Rate{r: v, set: true}, nil
}
