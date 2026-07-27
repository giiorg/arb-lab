// Package application is the OFFENDING fixture for the second policed layer.
//
// It exists separately from the domain fixtures because "domain and
// application are both inward" is a claim the code makes, and a suite that
// only ever offended from domain/ would pass identically against an analyzer
// that policed domain alone.
package application

import (
	"github.com/giiorg/arb-lab/internal/engine/state"                     // want `application must not import engine`
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven" // want `application must not import infrastructure`
)

type Service struct {
	MV   state.MarketView
	Feed driven.Client
}
