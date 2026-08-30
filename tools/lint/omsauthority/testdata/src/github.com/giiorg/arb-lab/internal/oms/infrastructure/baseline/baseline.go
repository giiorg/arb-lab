// Package baseline is §3.6 rule 3's documented exception, which exists so a
// composition root can construct §5.2's spot_baseline pin WITHOUT acquiring the
// write-adapter surface. Policing internal/oms as a tree would undo that.
//
// It is also within()'s PREFIX arm on the owner side.
package baseline

import "github.com/giiorg/arb-lab/internal/oms/application"

// Pin is §5.2's spot_baseline.
type Pin struct{ E application.Executor }
