// Package domain here is nested UNDER infrastructure, and it must stay clean.
//
// This is the precedence fixture. classify takes the first layer segment in
// the path, because path order is containment order and the outer directory
// says what a package is. Under "any domain segment anywhere makes it domain"
// — the reading a person writes first — this package would be policed and this
// file would fail. The two rules are indistinguishable without a path
// containing both segments, which is why one exists here.
package domain

import "github.com/giiorg/arb-lab/internal/engine/state"

type Adapter struct {
	MV state.MarketView
}
