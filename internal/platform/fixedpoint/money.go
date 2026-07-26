package fixedpoint

import "fmt"

// Money is an amount of a single asset — a balance, a fee, a notional, a PnL.
//
// The zero value is deliberately invalid: it carries no asset, so Validate
// rejects it and arithmetic against it fails. Construct with NewMoney or
// ZeroMoney.
type Money struct {
	m     int64 // mantissa at the canonical scale
	asset Asset
}

// Validate reports whether m is a usable value. It is the runtime half of
// §3.6 rule 4 — the check the compiler cannot perform for a dynamic asset tag.
func (m Money) Validate() error {
	if m.asset == "" {
		return fmt.Errorf("money: %w", ErrAssetUnset)
	}
	return nil
}

// ParseMoney converts exact decimal text — the only representation venues
// speak — into a Money tagged with asset.
func ParseMoney(asset Asset, s string) (Money, error) {
	if asset == "" {
		return Money{}, fmt.Errorf("money: %w", ErrAssetUnset)
	}
	m, err := parseMantissa(s)
	if err != nil {
		return Money{}, fmt.Errorf("money: %w", err)
	}
	return Money{m: m, asset: asset}, nil
}

// ZeroMoney is the additive identity for an asset. It exists because the zero
// value of Money is deliberately invalid, so `var total Money` is not a usable
// accumulator — say which asset you are accumulating.
func ZeroMoney(asset Asset) Money {
	return Money{asset: asset}
}

// Asset returns the unit tag.
func (m Money) Asset() Asset { return m.asset }

// Decimal renders the amount as canonical decimal text, without the asset.
// This is the codec/round-trip form: venue submissions and the SQL codec need
// the number alone.
func (m Money) Decimal() string { return formatMantissa(m.m) }
