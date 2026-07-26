// Package main is the positive control for TestCategoriesDoNotMix: it uses the
// API the way it is meant to be used and MUST compile. Without it, a typo in a
// fixture path — or an API change that breaks every fixture — would look
// exactly like the type system doing its job.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	px, _ := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	qty, _ := fixedpoint.ParseQuantity("BTC", "2")
	fee, _ := fixedpoint.ParseRate("0.00045")

	notional, _ := fixedpoint.Notional(px, qty, fixedpoint.RoundHalfEven)
	cost, _ := notional.Mul(fee, fixedpoint.RoundAwayFromZero)
	net, _ := notional.Sub(cost)
	_ = net
}
