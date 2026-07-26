package fixedpoint

// Calc is a chained money calculation that carries its first failure forward.
//
// It exists for one reason: Go has no `?`. A break-even or a settlement figure
// is a dozen consecutive fallible steps, and written longhand it becomes a
// dozen `if err != nil` blocks with the arithmetic hidden between them. The
// version that reviewers can actually check is
//
//	net, err := Chain(funding).Sub(entryFee).Sub(exitFee).Result()
//
// The safety property that makes a sticky error acceptable here — as against
// the usual objection to swallowing errors — is that it lives in a *local*
// calculation and can never end up in a stored value. Result is the only exit,
// it always returns the error, and when there is one it returns an invalid
// Money alongside it. A caller who ignores the error is therefore holding
// something that fails Validate and fails every subsequent operation, rather
// than a plausible amount.
//
// Calc is a value type, so a chain does not allocate. That is deliberate: the
// helper only prevents hand-rolled error checks if it is cheap enough that
// nobody is tempted to skip it.
//
// The zero value is not a usable accumulator — it holds an unconstructed Money
// — which Result reports. Start a chain with Chain.
type Calc struct {
	money Money
	err   error
}

// Chain starts a calculation from m.
//
// An unconstructed m fails here rather than silently at the end, so the error
// names the step that was actually wrong.
func Chain(m Money) Calc {
	if err := m.Validate(); err != nil {
		return Calc{err: err}
	}
	return Calc{money: m}
}

// failed reports whether the chain has already stopped. Every step consults it
// first, so the FIRST error is what survives: later failures are consequences
// of operating on a value that was never produced, and reporting one of those
// would name the symptom rather than the cause.
func (c Calc) failed() bool { return c.err != nil }

// Add adds o to the running value.
func (c Calc) Add(o Money) Calc {
	if c.failed() {
		return c
	}
	v, err := c.money.Add(o)
	return Calc{money: v, err: err}
}

// Sub subtracts o from the running value.
func (c Calc) Sub(o Money) Calc {
	if c.failed() {
		return c
	}
	v, err := c.money.Sub(o)
	return Calc{money: v, err: err}
}

// Mul scales the running value by a dimensionless ratio.
func (c Calc) Mul(r Rate, mode RoundingMode) Calc {
	if c.failed() {
		return c
	}
	v, err := c.money.Mul(r, mode)
	return Calc{money: v, err: err}
}

// Neg flips the sign of the running value.
func (c Calc) Neg() Calc {
	if c.failed() {
		return c
	}
	v, err := c.money.Neg()
	return Calc{money: v, err: err}
}

// Abs takes the magnitude of the running value.
func (c Calc) Abs() Calc {
	if c.failed() {
		return c
	}
	v, err := c.money.Abs()
	return Calc{money: v, err: err}
}

// Result ends the chain, returning the value and the first error along the way.
//
// The Validate call covers the zero value of Calc, which has no error recorded
// because no step ever ran, and would otherwise hand back a plausible zero of
// no currency.
func (c Calc) Result() (Money, error) {
	if c.err != nil {
		return Money{}, c.err
	}
	if err := c.money.Validate(); err != nil {
		return Money{}, err
	}
	return c.money, nil
}

// Ratio ends the chain by dividing the running value by o, giving a
// dimensionless Rate. It is terminal because the result leaves the Money
// category: a carry expressed against the capital that earned it is no longer
// an amount, and milestone 2's verdict is exactly that quotient.
func (c Calc) Ratio(o Money, mode RoundingMode) (Rate, error) {
	if c.err != nil {
		return Rate{}, c.err
	}
	if err := c.money.Validate(); err != nil {
		return Rate{}, err
	}
	return c.money.Div(o, mode)
}
