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

// String renders the amount with its asset, for logs and wrapped errors.
//
// An unset value prints as a placeholder rather than as "0 " — a log line
// showing a plausible amount of no currency is how an unpopulated field gets
// read past. Use Decimal, not String, for anything a machine parses.
func (m Money) String() string {
	if m.asset == "" {
		return "<invalid money>"
	}
	return formatMantissa(m.m) + " " + string(m.asset)
}

// requireAsset is the unary half of the tag check.
func (m Money) requireAsset(op string) error {
	if m.asset == "" {
		return fmt.Errorf("money %s: %w", op, ErrAssetUnset)
	}
	return nil
}

// binaryAsset validates both operands of a two-value operation and returns the
// asset the result carries. Every binary operation goes through it, because a
// tag is only worth carrying if nothing can skip checking it.
func binaryAsset(op string, a, b Money) (Asset, error) {
	if a.asset == "" || b.asset == "" {
		return "", fmt.Errorf("money %s: %w", op, ErrAssetUnset)
	}
	if a.asset != b.asset {
		return "", fmt.Errorf("money %s: %s vs %s: %w", op, a.asset, b.asset, ErrAssetMismatch)
	}
	return a.asset, nil
}

// Add returns m + o. It takes no RoundingMode: two values at the same scale add
// exactly, so the only way out of the range is overflow, which errors.
func (m Money) Add(o Money) (Money, error) {
	asset, err := binaryAsset("add", m, o)
	if err != nil {
		return Money{}, err
	}
	s, err := addMantissa(m.m, o.m)
	if err != nil {
		return Money{}, fmt.Errorf("money add %s + %s: %w", m, o, err)
	}
	return Money{m: s, asset: asset}, nil
}

// Sub returns m - o, exactly, or an error if the difference leaves the range.
func (m Money) Sub(o Money) (Money, error) {
	asset, err := binaryAsset("sub", m, o)
	if err != nil {
		return Money{}, err
	}
	d, err := subMantissa(m.m, o.m)
	if err != nil {
		return Money{}, fmt.Errorf("money sub %s - %s: %w", m, o, err)
	}
	return Money{m: d, asset: asset}, nil
}

// Neg returns -m. It can fail: the most negative representable amount has no
// positive counterpart, so flipping the sign of it is an error rather than a
// silent no-op.
func (m Money) Neg() (Money, error) {
	if err := m.requireAsset("neg"); err != nil {
		return Money{}, err
	}
	v, err := negMantissa(m.m)
	if err != nil {
		return Money{}, fmt.Errorf("money neg %s: %w", m, err)
	}
	return Money{m: v, asset: m.asset}, nil
}

// Abs returns |m|, with the same caveat as Neg at the negative bound.
func (m Money) Abs() (Money, error) {
	if err := m.requireAsset("abs"); err != nil {
		return Money{}, err
	}
	if m.m >= 0 {
		return m, nil
	}
	v, err := negMantissa(m.m)
	if err != nil {
		return Money{}, fmt.Errorf("money abs %s: %w", m, err)
	}
	return Money{m: v, asset: m.asset}, nil
}

// Cmp reports whether m sorts before (-1), with (0) or after (1) o.
//
// Money is a comparable struct, so `a == b` also compiles — and it is *not* a
// substitute for this, because it silently reports false for two amounts of
// different assets instead of flagging the mismatch.
func (m Money) Cmp(o Money) (int, error) {
	if _, err := binaryAsset("cmp", m, o); err != nil {
		return 0, err
	}
	return cmpMantissa(m.m, o.m), nil
}

// Sign reports -1, 0 or 1.
//
// It is fallible for the same reason the zero value is invalid: 0 is a
// legitimate answer, so an infallible Sign could not distinguish "this amount
// is genuinely zero" from "nobody ever populated this field" — and a cost that
// reads as zero is a cost that reads as free.
func (m Money) Sign() (int, error) {
	if err := m.requireAsset("sign"); err != nil {
		return 0, err
	}
	return signMantissa(m.m), nil
}
