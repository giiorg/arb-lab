// Package arblab sits OUTSIDE internal/ entirely, and it is the fixture that
// forces the module root to be resolved from the imported path.
//
// hexboundary and kernelpurity both resolve the root by splitting the ANALYZED
// package's path on /internal/, which is sound for them: their rules only apply
// to packages that are under internal/ by construction. This rule applies to
// every importer there is, so resolving from the analyzed package would return
// "not ours" here and leave a binary free to import the write repository —
// a hole with no diagnostic anywhere.
package arblab

import "github.com/giiorg/arb-lab/internal/oms/infrastructure/driven" // want `only oms may import the OMS write path .*infrastructure/driven.*§3.6 rule 6`

func main() { _ = driven.Memory{} }
