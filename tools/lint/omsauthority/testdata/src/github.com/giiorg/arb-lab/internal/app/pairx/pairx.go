// Package pairx is the GRANT-side prefix trap: internal/app/pairx is not
// internal/app/pair.
package pairx

import "github.com/giiorg/arb-lab/internal/oms" // want `not granted OMS write authority.*OMS control surface`

// Held is the authority a name-alike must not inherit.
type Held struct{ M oms.Module }
