// Package sub is a SUBpackage of a granted root, and it is reported. A grant
// covers the package it names and not its tree: an exact comparison and a
// prefix comparison agree on every path that is NOT a subpackage of a grant, so
// this is the only fixture that can separate them.
package sub

import "github.com/giiorg/arb-lab/internal/oms/application" // want `not granted OMS write authority.*the use cases themselves`

// Held is the authority a subpackage was never weighed for.
type Held struct{ E application.Executor }
