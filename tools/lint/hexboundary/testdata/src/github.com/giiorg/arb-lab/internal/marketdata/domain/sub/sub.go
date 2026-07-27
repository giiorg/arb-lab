// Package sub is a NESTED domain package, and it is offending on both targets.
//
// Nesting is the point: a rule matching only the leaf directory would be
// evaded by adding a subdirectory, which is an escape hatch nobody voted for.
package sub

import (
	"github.com/giiorg/arb-lab/internal/engine"                           // want `domain must not import engine`
	"github.com/giiorg/arb-lab/internal/engine/state"                     // want `domain must not import engine`
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven" // want `domain must not import infrastructure`
)

type View struct {
	Spine engine.Spine // the parity seam's own package, matched by within()'s equality arm
	MV    state.MarketView
	Feed  driven.Client
}
