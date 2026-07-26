// A balance plus a size is meaningless, and must not build.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	balance, _ := fixedpoint.ParseMoney("USDT", "100")
	size, _ := fixedpoint.ParseQuantity("BTC", "2")
	_, _ = balance.Add(size)
}
