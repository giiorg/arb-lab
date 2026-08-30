// Package omsx is the owner-side prefix trap. internal/omsx is not
// internal/oms, and a bare HasPrefix exemption would hand it the owner's
// authority silently.
package omsx

import "github.com/giiorg/arb-lab/internal/oms" // want `not granted OMS write authority.*OMS control surface`

// Held is the authority it must not have.
type Held struct{ M oms.Module }
