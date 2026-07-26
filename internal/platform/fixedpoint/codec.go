package fixedpoint

import "fmt"

// The persistence seam.
//
// This is a *value* codec, not a schema. It converts between a category and the
// columns a row holds — a mantissa plus its asset name (or names) — and says
// nothing about tables, which belong to infrastructure/driven/.
//
// The mantissa is an int64, which is exactly SQLite INTEGER and exactly
// Postgres BIGINT. So the stored form is lossless, identical across both
// dialects, and orderable and summable by the database itself. That is what
// makes the documented SQLite-to-Postgres path (D2) cheap, and it is the
// concrete reason the 128-bit representation was rejected: its codec would have
// diverged per dialect — TEXT or paired columns on SQLite against NUMERIC on
// Postgres — and the divergence would have leaked into every query that
// compares or aggregates one of these columns.
//
// The pair is deliberately two columns rather than one encoded string. A single
// column would force the sql.Scanner/driver.Valuer shape, which cannot carry an
// asset tag alongside a number, and the only way to fit both would be text —
// giving up ordering and SUM() in the database to save a column.
//
// Mantissa is the one place a raw scaled integer leaves this package. It exists
// for persistence and for migrations that need to reason about a column
// directly; it is not an arithmetic escape hatch. Doing sums on values obtained
// this way discards the asset check and the overflow rules that are the whole
// point of the categories.

// Mantissa returns the scaled integer stored for this amount.
func (m Money) Mantissa() int64 { return m.m }

// MoneyFromMantissa rebuilds an amount from its stored columns.
//
// An empty asset is rejected: that is how a NULL or blank asset column — a
// partially written row — reaches this function, and such a row must not be
// able to become a usable amount.
func MoneyFromMantissa(asset Asset, mantissa int64) (Money, error) {
	if asset == "" {
		return Money{}, fmt.Errorf("money from mantissa: %w", ErrAssetUnset)
	}
	return Money{m: mantissa, asset: asset}, nil
}

// Mantissa returns the scaled integer stored for this size.
func (q Quantity) Mantissa() int64 { return q.q }

// QuantityFromMantissa rebuilds a size from its stored columns.
func QuantityFromMantissa(asset Asset, mantissa int64) (Quantity, error) {
	if asset == "" {
		return Quantity{}, fmt.Errorf("quantity from mantissa: %w", ErrAssetUnset)
	}
	return Quantity{q: mantissa, asset: asset}, nil
}

// Mantissa returns the scaled integer stored for this price.
func (p Price) Mantissa() int64 { return p.p }

// PriceFromMantissa rebuilds a price from its stored columns. A price occupies
// three: the mantissa and both asset names.
func PriceFromMantissa(base, quote Asset, mantissa int64) (Price, error) {
	p := Price{base: base, quote: quote}
	if err := p.Validate(); err != nil {
		return Price{}, fmt.Errorf("price from mantissa: %w", err)
	}
	p.p = mantissa
	return p, nil
}

// Mantissa returns the scaled integer stored for this ratio.
func (r Rate) Mantissa() int64 { return r.r }

// RateFromMantissa rebuilds a ratio from its stored column.
//
// Unlike the other three this cannot fail, because a Rate has no asset tag to
// be missing — a single non-NULL integer column is the whole value. A NULL
// column has no Rate to build, and a repository must leave the field as the
// zero value so that Validate reports it.
func RateFromMantissa(mantissa int64) Rate {
	return Rate{r: mantissa, set: true}
}
