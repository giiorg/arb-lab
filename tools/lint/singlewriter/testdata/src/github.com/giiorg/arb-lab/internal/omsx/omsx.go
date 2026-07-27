// Package omsx is the sibling-prefix trap, and it is the fixture whose
// diagnostic is easiest to lose.
//
// internal/omsx starts with internal/oms. An exemption written as a bare
// HasPrefix would read this package as the OMS itself and hand it the one
// context's write authority — silently, since the failure mode of a wrong
// exemption is a clean run. Comparing whole segments is what makes the import
// below reportable, and this fixture is how that is measured rather than
// asserted.
package omsx

import "github.com/giiorg/arb-lab/internal/oms/infrastructure/driven" // want `only oms may import the OMS write path .*infrastructure/driven.*§3.6 rule 6`

type Store struct{ Repo driven.Memory }
