// Command arblab-lint runs arb-lab's custom structural gates over the engine.
//
// These are hand-written analyzers rather than golangci-lint configuration for
// one reason: a gate that cannot be unit-tested cannot be shown to bite, and
// build-spec §3.6 says each rule is "checked by a boundary/lint test in CI".
// Every analyzer here has testdata fixtures that MUST produce diagnostics.
//
// Built and run via `make lint`.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/giiorg/arb-lab/tools/lint/kernelpurity"
	"github.com/giiorg/arb-lab/tools/lint/nofloat"
)

func main() {
	multichecker.Main(
		kernelpurity.Analyzer,
		nofloat.Analyzer,
	)
}
