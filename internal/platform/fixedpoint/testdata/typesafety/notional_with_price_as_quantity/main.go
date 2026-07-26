// Notional's argument order is enforced by the types: a Price cannot stand in
// for a Quantity, so transposing the two arguments does not build.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	px, _ := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	qty, _ := fixedpoint.ParseQuantity("BTC", "2")
	_, _ = fixedpoint.Notional(qty, px, fixedpoint.RoundExact)
}
