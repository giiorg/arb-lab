// The mantissa is unexported, so no caller outside the package can read or
// write it directly. Every value has to go through a checked constructor.
package main

import "github.com/giiorg/arb-lab/internal/platform/fixedpoint"

func main() {
	balance, _ := fixedpoint.ParseMoney("USDT", "100")
	_ = balance.m
}
