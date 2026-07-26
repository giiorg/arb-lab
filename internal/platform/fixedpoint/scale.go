package fixedpoint

const (
	// Scale is the canonical number of decimal places every value is stored at.
	//
	// 9 is NautilusTrader's standard-mode precision (§11 names it as the
	// reference implementation), and it is chosen to land exactly on an int64
	// mantissa: the representable range is ±9,223,372,036.854775807, which is
	// ample for USDT notionals and BTC quantities. It also maps 1:1 onto
	// SQLite INTEGER and Postgres BIGINT, so the persistence codec is exact and
	// dialect-independent.
	//
	// Rate is the one category where 9 places could eventually pinch (a funding
	// rate compounded across ~1095 settlements). Because Rate is its own type
	// rather than an alias, raising *its* scale later is a change to one file
	// and its codec, not a repo-wide audit.
	Scale = 9

	// scaleFactor is 10^Scale.
	scaleFactor = 1_000_000_000

	// maxMantissaDigits is the decimal width of MaxInt64. It doubles as the
	// guard that stops a large exponent from making us materialize a huge digit
	// string before discovering it cannot fit.
	maxMantissaDigits = 19

	// maxExponent bounds |e| in scientific notation so that point-position
	// arithmetic cannot overflow an int. No venue emits anything near this.
	maxExponent = 1000
)
