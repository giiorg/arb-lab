package sub

// An IN-PACKAGE test file. Its imports are the domain package's imports, so
// the gate objects here too, and it does so for free rather than by a rule:
// this file IS package sub. A helper written in a _test.go file and later
// promoted to production carries its imports with it, and the objection should
// arrive the first time rather than the second.
import "github.com/giiorg/arb-lab/internal/engine/state" // want `domain must not import engine`

type fixture struct {
	MV state.MarketView
}
