package fixedpoint

import "fmt"

// Price is the price of one unit of base, expressed in quote.
//
// It carries *both* assets, and that is what makes the runtime tag catch
// anything worth catching. With one tag — or none — multiplying a BTC quantity
// by an ETH price yields a plausible number that no later check can identify as
// wrong. With base and quote, Notional refuses it.
//
// A price may be negative: calendar spreads and some settled contracts trade
// below zero, and screening that is a venue's business, not this package's.
//
// The zero value is invalid; construct with ParsePrice.
type Price struct {
	p     int64 // mantissa at the canonical scale
	base  Asset // the asset being priced
	quote Asset // the asset it is priced in
}

// Validate reports whether p is a usable value.
func (p Price) Validate() error {
	if p.base == "" || p.quote == "" {
		return fmt.Errorf("price: %w", ErrAssetUnset)
	}
	if p.base == p.quote {
		return fmt.Errorf("price: base and quote are both %s: %w", p.base, ErrAssetMismatch)
	}
	return nil
}

// ParsePrice converts exact decimal text into a price of base in quote.
//
// base == quote is rejected rather than tolerated as a trivial 1:1: in practice
// it means an adapter filled both tags from the same field, and a degenerate
// price propagates into notionals as a silently wrong unit.
func ParsePrice(base, quote Asset, s string) (Price, error) {
	p := Price{base: base, quote: quote}
	if err := p.Validate(); err != nil {
		return Price{}, err
	}
	m, err := parseMantissa(s)
	if err != nil {
		return Price{}, fmt.Errorf("price: %w", err)
	}
	p.p = m
	return p, nil
}

// Base returns the asset being priced.
func (p Price) Base() Asset { return p.base }

// Quote returns the asset the price is expressed in.
func (p Price) Quote() Asset { return p.quote }

// Decimal renders the price as canonical decimal text, without the assets.
// This is the codec form: venue submissions need the number alone.
func (p Price) Decimal() string { return formatMantissa(p.p) }

// String renders the price with its pair, in the quote/base order a trader
// reads: "43250.5 USDT/BTC".
func (p Price) String() string {
	if p.base == "" || p.quote == "" {
		return "<invalid price>"
	}
	return formatMantissa(p.p) + " " + string(p.quote) + "/" + string(p.base)
}

// Cmp reports whether p sorts before (-1), with (0) or after (1) o. Both tags
// must agree: two prices being numbers does not make them comparable.
func (p Price) Cmp(o Price) (int, error) {
	if err := p.Validate(); err != nil {
		return 0, fmt.Errorf("price cmp: %w", err)
	}
	if err := o.Validate(); err != nil {
		return 0, fmt.Errorf("price cmp: %w", err)
	}
	if p.base != o.base || p.quote != o.quote {
		return 0, fmt.Errorf("price cmp: %s/%s vs %s/%s: %w",
			p.quote, p.base, o.quote, o.base, ErrAssetMismatch)
	}
	return cmpMantissa(p.p, o.p), nil
}

// binaryPair is Money's binaryAsset on the price axis: it refuses two prices
// that are not quoting the same base in the same quote.
//
// Both tags rather than one, for the reason the type carries both: a check on
// the quote alone admits BTC and ETH prices into one subtraction, and the
// difference between a BTC mark and an ETH liquidation price is a number with a
// plausible magnitude and no meaning.
func binaryPair(op string, a, b Price) (base, quote Asset, err error) {
	if err := a.Validate(); err != nil {
		return "", "", fmt.Errorf("price %s: %w", op, err)
	}
	if err := b.Validate(); err != nil {
		return "", "", fmt.Errorf("price %s: %w", op, err)
	}
	if a.base != b.base || a.quote != b.quote {
		return "", "", fmt.Errorf("price %s: %s/%s vs %s/%s: %w",
			op, a.quote, a.base, b.quote, b.base, ErrAssetMismatch)
	}
	return a.base, a.quote, nil
}

// Sub returns p - o, exactly, or an error if the difference leaves the range.
//
// Money.Sub's twin on the price axis, and it takes no RoundingMode for the same
// reason: two values at the canonical scale subtract exactly, so the only way
// out is overflow.
//
// The result is a Price and MAY be negative — the type documents that a price
// can be — so a caller reading a difference as a distance must ask Sign. §7.1's
// liq distance is this operation's first caller and does exactly that: for a
// short, `liqPrice - mark` is positive while the position has headroom and turns
// negative at the instant it does not, and collapsing the sign there would
// report the most urgent state on the ladder as the safest.
func (p Price) Sub(o Price) (Price, error) {
	base, quote, err := binaryPair("sub", p, o)
	if err != nil {
		return Price{}, err
	}
	d, err := subMantissa(p.p, o.p)
	if err != nil {
		return Price{}, fmt.Errorf("price sub %s - %s: %w", p, o, err)
	}
	return Price{p: d, base: base, quote: quote}, nil
}

// Div returns p / o as a dimensionless Rate: dividing two prices of the same
// pair cancels both units, exactly as Money.Div cancels one.
//
// Expressing a price difference against the price it is measured from is §7.1's
// liq distance — "how far the mark must move, as a fraction of where it is" —
// and it is this operation.
//
// A zero divisor is an error rather than an infinity, Money.Div's rule. It is
// worth more here than there: the divisor §7.1 supplies is the venue's mark, and
// every layer between the wire and this call already refuses a non-positive one
// (venue.Mark.Validate, MarkReading's screening) — so reaching this branch means
// one of those refusals was bypassed, and returning any number at all would give
// the ladder a distance computed from a mark of nothing.
func (p Price) Div(o Price, mode RoundingMode) (Rate, error) {
	if _, _, err := binaryPair("div", p, o); err != nil {
		return Rate{}, err
	}
	if o.p == 0 {
		return Rate{}, fmt.Errorf("price div %s by zero: %w", p, ErrDivideByZero)
	}
	// Scale the numerator up before dividing so the quotient lands on the
	// canonical scale rather than being truncated to whole units — Money.Div's
	// comment, and the same helper.
	v, err := mulDivMantissa(p.p, scaleFactor, o.p, mode)
	if err != nil {
		return Rate{}, fmt.Errorf("price div %s / %s: %w", p, o, err)
	}
	return Rate{r: v, set: true}, nil
}

// Sign reports -1, 0 or 1.
//
// Money.Sign's twin on the price axis, and fallible for Money.Sign's reason: 0
// is a legitimate answer, so an infallible Sign could not distinguish "this
// price is genuinely zero" from "nobody ever populated this field".
//
// It exists because the positivity checks this package makes internally — see
// OnTick's tick.p <= 0 — are unreachable from outside it, and a caller holding a
// price that must be strictly positive has no other way to say so. The first
// such caller is venue.Mark, whose value §7.1's liq distance DIVIDES BY.
func (p Price) Sign() (int, error) {
	if err := p.Validate(); err != nil {
		return 0, fmt.Errorf("price sign: %w", err)
	}
	return signMantissa(p.p), nil
}

// OnTick reports whether p sits on tick's price grid — is an exact whole
// multiple of it. Quantity.OnStep's twin for the price axis: the same
// predicate-not-rounding contract, stated there.
//
// Both tags must agree, Cmp's rule: a tick for one instrument says nothing
// about a price on another pair. tick must be strictly positive — see
// ErrStepNotPositive.
func (p Price) OnTick(tick Price) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, fmt.Errorf("price on tick: %w", err)
	}
	if err := tick.Validate(); err != nil {
		return false, fmt.Errorf("price on tick: %w", err)
	}
	if p.base != tick.base || p.quote != tick.quote {
		return false, fmt.Errorf("price on tick: %s/%s vs %s/%s: %w",
			p.quote, p.base, tick.quote, tick.base, ErrAssetMismatch)
	}
	if tick.p <= 0 {
		return false, fmt.Errorf("price on tick %s: %w", tick, ErrStepNotPositive)
	}
	return p.p%tick.p == 0, nil
}

// Notional returns the value of qty at px: an amount of the price's quote
// asset. It is the operation that turns a size into money.
//
// The quantity must be denominated in the price's base asset. That check is the
// entire reason Price carries two tags — it is what turns "multiplied a BTC
// size by an ETH price" from a plausible number into an error.
func Notional(px Price, qty Quantity, mode RoundingMode) (Money, error) {
	if err := px.Validate(); err != nil {
		return Money{}, fmt.Errorf("notional: %w", err)
	}
	if err := qty.Validate(); err != nil {
		return Money{}, fmt.Errorf("notional: %w", err)
	}
	if qty.asset != px.base {
		return Money{}, fmt.Errorf("notional: %s quantity at a %s/%s price: %w",
			qty.asset, px.quote, px.base, ErrAssetMismatch)
	}
	v, err := mulDivMantissa(px.p, qty.q, scaleFactor, mode)
	if err != nil {
		return Money{}, fmt.Errorf("notional %s at %s: %w", qty, px, err)
	}
	return Money{m: v, asset: px.quote}, nil
}

// AveragePrice is Notional's inverse: the price implied by paying total for
// qty. An OMS reports it as the average fill across partial executions.
//
// The result is tagged as qty's asset priced in total's asset, so the two must
// differ — a total and a size in the same asset describe a ratio, not a price,
// and Rate is the type for that.
func AveragePrice(total Money, qty Quantity, mode RoundingMode) (Price, error) {
	if err := total.Validate(); err != nil {
		return Price{}, fmt.Errorf("average price: %w", err)
	}
	if err := qty.Validate(); err != nil {
		return Price{}, fmt.Errorf("average price: %w", err)
	}
	if qty.q == 0 {
		return Price{}, fmt.Errorf("average price over a flat position: %w", ErrDivideByZero)
	}
	px := Price{base: qty.asset, quote: total.asset}
	if err := px.Validate(); err != nil {
		return Price{}, fmt.Errorf("average price: %w", err)
	}
	v, err := mulDivMantissa(total.m, scaleFactor, qty.q, mode)
	if err != nil {
		return Price{}, fmt.Errorf("average price %s over %s: %w", total, qty, err)
	}
	px.p = v
	return px, nil
}
