package fixedpoint

// Asset is the runtime-checked unit tag carried by every dimensional value:
// "USDT", "BTC", "USDC".
//
// It is deliberately NOT a compile-time (phantom) type parameter. Venue assets
// are discovered dynamically from exchange metadata, so the set is not known
// when this package is compiled — build-spec §3.6 rule 4 makes exactly this
// split: the four *categories* (Money/Quantity/Price/Rate) are distinct
// compile-time types so cross-category arithmetic will not build, while the
// asset tag within a category is checked at runtime.
type Asset string
