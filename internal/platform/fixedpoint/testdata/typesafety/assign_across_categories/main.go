// The categories are distinct types, not aliases of one another, so a value of
// one cannot be assigned into a variable of another.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	balance, _ := fixedpoint.ParseMoney("USDT", "100")
	var size fixedpoint.Quantity = balance
	_ = size
}
