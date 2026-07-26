// Package main is outside internal/, so the rule does not apply here and this
// float must produce NO diagnostic. The scope is deliberate: §3's rejected
// alternative was zero-exception denial, and the reason it was rejected is that
// a conversion shim has to be able to live somewhere.
package main

var throughput float64

func main() { throughput = 1.5 }
