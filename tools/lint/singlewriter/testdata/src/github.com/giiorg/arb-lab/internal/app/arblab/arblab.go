// Package arblab stands in for §11's composition root, and it is policed —
// the deliberate reading recorded in the analyzer's doc.
//
// Rule 6 says "other contexts", and a root is not a context. It is policed
// anyway because the other direction fails silently: a root that may hold a
// repository may pass it to another context's constructor, and Go's structural
// typing means the receiving context needs no import to accept it. Each context
// wires its own adapters in module.go, so a root never needs to name one.
package arblab

import "github.com/giiorg/arb-lab/internal/oms/infrastructure/driven" // want `only oms may import the OMS write path .*infrastructure/driven.*§3.6 rule 6`

func Wire() driven.Memory { return driven.Memory{} }
