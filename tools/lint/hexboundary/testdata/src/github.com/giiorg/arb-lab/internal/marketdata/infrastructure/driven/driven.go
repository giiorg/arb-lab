// Package driven is the CLEAN outward fixture: an adapter importing both the
// parity seam and the domain it adapts.
//
// This is the direction the pattern is made of, and it is also the fixture
// that separates the rule from "nothing may import engine/". Without it, an
// analyzer that policed every package would score identically.
package driven

import (
	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

type Client struct {
	MV  state.MarketView
	Obs domain.Observation
}
