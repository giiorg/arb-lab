package fixedpoint

import "errors"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
var (
	// ErrAssetUnset reports a value whose asset tag was never set — in
	// practice, a zero value that reached arithmetic or persistence.
	ErrAssetUnset = errors.New("fixedpoint: asset unset")

	// ErrSyntax reports text that is not a decimal number at all.
	ErrSyntax = errors.New("fixedpoint: invalid decimal syntax")

	// ErrPrecisionLoss reports a value that carries more precision than the
	// canonical scale can hold. It is deliberately an error rather than a
	// silent round: a venue that starts quoting finer than we store is a
	// finding, not something to paper over. Callers that genuinely want to
	// round say so explicitly with a RoundingMode.
	ErrPrecisionLoss = errors.New("fixedpoint: value exceeds canonical scale")

	// ErrOverflow reports a magnitude outside the representable range. It is
	// returned instead of wrapping, always.
	ErrOverflow = errors.New("fixedpoint: value out of range")
)
