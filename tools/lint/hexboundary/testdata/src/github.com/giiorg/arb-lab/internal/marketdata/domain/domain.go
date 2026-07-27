// Package domain is the CLEAN inward fixture. Every import here is one a
// domain package is entitled to make, and each separates the rule from a
// plausible over-broad reading of it.
//
// analysistest fails on an UNEXPECTED diagnostic as well as a missing one, so
// this file is the half of the gate that proves it does not over-report.
package domain

import (
	// The standard library. Always fine, and outside the module entirely.
	"time"

	// A third-party package whose path contains a segment named "engine". It
	// is not §3.5's parity seam, and a segment scan would report it.
	tp "example.com/thirdparty/engine"

	// A third-party package whose path contains a segment named
	// "infrastructure" — the one shape that tells classify's module-root guard
	// from its absence. See that package's own comment.
	tpinfra "example.com/thirdparty/infrastructure"

	// A context-local package named "engine", which is likewise not the parity
	// seam: rule 1's target is internal/engine/, identified by position.
	"github.com/giiorg/arb-lab/internal/marketdata/engine"

	// A package directly under internal/ whose name merely starts with
	// "engine". A prefix comparison that did not compare whole segments would
	// classify this as the parity seam.
	"github.com/giiorg/arb-lab/internal/engineering"

	// The shared kernel — the dependency spine running the right way.
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"

	// Another bounded context's domain. Rule 1 is about layers, not contexts,
	// and says nothing about this.
	riskdomain "github.com/giiorg/arb-lab/internal/risk/domain"
)

type Observation struct {
	At      time.Time
	Rate    fixedpoint.Rate
	Widget  tp.Widget
	Store   tpinfra.Store
	Reducer engine.Reducer
	Note    engineering.Note
	Limit   riskdomain.Limit
}
