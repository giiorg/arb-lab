// The same mistake in the other direction.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	balance, _ := fixedpoint.ParseMoney("USDT", "100")
	size, _ := fixedpoint.ParseQuantity("BTC", "2")
	_, _ = size.Add(balance)
}
