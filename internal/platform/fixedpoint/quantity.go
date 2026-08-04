package fixedpoint

import "fmt"

// Quantity is a number of units of an instrument's base asset — an order size,
// a position, a fill amount.
//
// It is a distinct type from Money rather than an alias, which is the
// compile-time half of §3.6 rule 4: adding a size to a balance does not build.
// The runtime half is the asset tag, which catches mixing BTC with ETH — a
// distinction the compiler cannot make, because the set of venue assets is
// discovered at run time.
//
// The zero value is invalid; construct with ParseQuantity or ZeroQuantity.
type Quantity struct {
	q     int64 // mantissa at the canonical scale
	asset Asset
}

// Validate reports whether q is a usable value.
func (q Quantity) Validate() error {
	if q.asset == "" {
		return fmt.Errorf("quantity: %w", ErrAssetUnset)
	}
	return nil
}

// ParseQuantity converts exact decimal text — the form venues speak — into a
// Quantity of asset.
func ParseQuantity(asset Asset, s string) (Quantity, error) {
	if asset == "" {
		return Quantity{}, fmt.Errorf("quantity: %w", ErrAssetUnset)
	}
	m, err := parseMantissa(s)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity: %w", err)
	}
	return Quantity{q: m, asset: asset}, nil
}

// ZeroQuantity is the additive identity for an asset — a flat position.
func ZeroQuantity(asset Asset) Quantity { return Quantity{asset: asset} }

// Asset returns the base asset being counted.
func (q Quantity) Asset() Asset { return q.asset }

// Decimal renders the size as canonical decimal text, without the asset. This
// is the codec form: venue submissions and the SQL codec need the number alone.
func (q Quantity) Decimal() string { return formatMantissa(q.q) }

// String renders the size with its asset, for logs and wrapped errors.
func (q Quantity) String() string {
	if q.asset == "" {
		return "<invalid quantity>"
	}
	return formatMantissa(q.q) + " " + string(q.asset)
}

func (q Quantity) requireAsset(op string) error {
	if q.asset == "" {
		return fmt.Errorf("quantity %s: %w", op, ErrAssetUnset)
	}
	return nil
}

func binaryQuantityAsset(op string, a, b Quantity) (Asset, error) {
	if a.asset == "" || b.asset == "" {
		return "", fmt.Errorf("quantity %s: %w", op, ErrAssetUnset)
	}
	if a.asset != b.asset {
		return "", fmt.Errorf("quantity %s: %s vs %s: %w", op, a.asset, b.asset, ErrAssetMismatch)
	}
	return a.asset, nil
}

// Add returns q + o, exactly. Sizes at the same scale add without loss, so no
// RoundingMode is involved; overflow errors rather than wrapping.
func (q Quantity) Add(o Quantity) (Quantity, error) {
	asset, err := binaryQuantityAsset("add", q, o)
	if err != nil {
		return Quantity{}, err
	}
	s, err := addMantissa(q.q, o.q)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity add %s + %s: %w", q, o, err)
	}
	return Quantity{q: s, asset: asset}, nil
}

// Sub returns q - o, exactly.
func (q Quantity) Sub(o Quantity) (Quantity, error) {
	asset, err := binaryQuantityAsset("sub", q, o)
	if err != nil {
		return Quantity{}, err
	}
	d, err := subMantissa(q.q, o.q)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity sub %s - %s: %w", q, o, err)
	}
	return Quantity{q: d, asset: asset}, nil
}

// Neg returns -q. Flipping a long position to the equivalent short is the usual
// caller; it fails only at the negative bound, which has no positive twin.
func (q Quantity) Neg() (Quantity, error) {
	if err := q.requireAsset("neg"); err != nil {
		return Quantity{}, err
	}
	v, err := negMantissa(q.q)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity neg %s: %w", q, err)
	}
	return Quantity{q: v, asset: q.asset}, nil
}

// Abs returns |q| — a position's size irrespective of side.
func (q Quantity) Abs() (Quantity, error) {
	if err := q.requireAsset("abs"); err != nil {
		return Quantity{}, err
	}
	if q.q >= 0 {
		return q, nil
	}
	v, err := negMantissa(q.q)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity abs %s: %w", q, err)
	}
	return Quantity{q: v, asset: q.asset}, nil
}

// Cmp reports whether q sorts before (-1), with (0) or after (1) o.
func (q Quantity) Cmp(o Quantity) (int, error) {
	if _, err := binaryQuantityAsset("cmp", q, o); err != nil {
		return 0, err
	}
	return cmpMantissa(q.q, o.q), nil
}

// Sign reports -1 for a short, 0 for flat and 1 for a long. It is fallible for
// the reason Money.Sign is: a flat position and a position nobody populated
// would otherwise be indistinguishable.
func (q Quantity) Sign() (int, error) {
	if err := q.requireAsset("sign"); err != nil {
		return 0, err
	}
	return signMantissa(q.q), nil
}

// OnStep reports whether q sits on step's lot grid — is an exact whole
// multiple of it.
//
// It is a predicate rather than a rounding operation, and the split is §2.4's:
// DERIVING an on-grid size belongs to the sizing step, which rounds down and
// owns the residual tolerance, while the order path VERIFIES and refuses. A
// verification has no rounding decision to make, so there is no RoundingMode
// parameter to invite one.
//
// step must be strictly positive — see ErrStepNotPositive for why a zero is
// refused rather than read as "no constraint". A negative q is judged by the
// same arithmetic (|q| on the grid), which is what a lot filter means for a
// short.
func (q Quantity) OnStep(step Quantity) (bool, error) {
	if _, err := binaryQuantityAsset("on step", q, step); err != nil {
		return false, err
	}
	if step.q <= 0 {
		return false, fmt.Errorf("quantity on step %s: %w", step, ErrStepNotPositive)
	}
	return q.q%step.q == 0, nil
}

// Mul scales a size by a dimensionless ratio — taking a fraction of a position,
// or applying a contract multiplier. The rounding mode is the caller's, and for
// sizing it is normally RoundTowardZero: never size beyond what is backed.
func (q Quantity) Mul(r Rate, mode RoundingMode) (Quantity, error) {
	if err := q.requireAsset("mul"); err != nil {
		return Quantity{}, err
	}
	if err := r.requireSet("mul"); err != nil {
		return Quantity{}, err
	}
	v, err := mulDivMantissa(q.q, r.r, scaleFactor, mode)
	if err != nil {
		return Quantity{}, fmt.Errorf("quantity mul %s * %s: %w", q, r, err)
	}
	return Quantity{q: v, asset: q.asset}, nil
}
