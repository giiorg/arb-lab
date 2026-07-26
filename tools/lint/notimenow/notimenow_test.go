package notimenow_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/notimenow"
)

// The gate under test: domain/ and application/ code never reads the wall clock.
//
// analysistest fails both on a missing diagnostic and on an unexpected one, so
// the clean fixtures carry as much weight as the offending ones — and for this
// rule they carry more. The rule is about reading a clock, not about importing
// time: a gate that objected to time.Duration or time.Parse would be switched
// off within a week, and then rule 5 would not be enforced at all.
func TestNoTimeNow(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), notimenow.Analyzer,
		// Offending: every ambient-time function, plus a reference taken
		// without a call.
		"github.com/giiorg/arb-lab/internal/oms/domain",
		// Offending: nested under domain/, imported under an alias and again
		// dot-imported — neither of which a textual match would catch.
		"github.com/giiorg/arb-lab/internal/oms/domain/aliased",
		// Offending in one place; legitimate time use everywhere else in the
		// file, and a second layer name in the diagnostic.
		"github.com/giiorg/arb-lab/internal/oms/application",
		// Clean, and the fixture that matters most: the adapter whose job is to
		// read the clock. A rule that policed this one could not be satisfied.
		"github.com/giiorg/arb-lab/internal/platform/clock",
		// Offending, and the escape hatch in all four of its states: a stamped
		// timestamp caught, a transport deadline escaped, an escape without a
		// reason refused, an escape with nothing to suppress reported, and a
		// misspelled directive left inert.
		"github.com/giiorg/arb-lab/internal/engine/venue",
		// Clean, and the positive control for the widened scope: engine code
		// that takes the Clock. Without it the suite would only show engine
		// code being rejected, which a universally-objecting gate also does.
		"github.com/giiorg/arb-lab/internal/engine/state",
	)
}
