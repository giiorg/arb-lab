// Adding a rate to an amount is the units error behind every "we added the fee
// rate instead of the fee" bug. It must not build.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	balance, _ := fixedpoint.ParseMoney("USDT", "100")
	rate, _ := fixedpoint.ParseRate("0.0001")
	_, _ = balance.Add(rate)
}
