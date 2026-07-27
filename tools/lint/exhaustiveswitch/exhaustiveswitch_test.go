package exhaustiveswitch_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/exhaustiveswitch"
)

// The gate under test: build-spec §3.6 rule 10 — a switch over a module-declared
// enum-like must name every member.
//
// analysistest fails on an unexpected diagnostic as well as a missing one, so
// the clean functions carry as much weight as the offending ones. Each is
// pinned to a distinct wrong reading:
//
//	enums.Exhaustive      value-grouping — keyed by name, maxKind is demanded
//	enums.WithDefault     a default: clause is NOT coverage (the whole rule)
//	enums.SingleMember    one constant is not a set
//	enums.BoolBacked      basic, two constants, neither integer nor string
//	enums.Grouped         the alternative offered instead of an escape hatch
//	enums.MissingLast     the reported name is the exported one, not maxKind
//	enums (test file)     defect is test-DECLARED and out of scope, while
//	                      oracle switches on a production enum FROM a test file
//	                      and is policed — opposite answers, one file
//	domain.CrossPackage   the decision site is usually not the enum's package
//	domain.StdlibEnum     reflect.Kind: the module-root scope, and three of
//	                      `exhaustive`'s eighteen real-tree reports
//	domain.NonConstant…   unjudgeable, so silent
//	domain.Tagless        no value to dispatch on
func TestExhaustiveSwitch(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), exhaustiveswitch.Analyzer,
		"github.com/giiorg/arb-lab/internal/platform/enums",
		"github.com/giiorg/arb-lab/internal/marketdata/domain",
	)
}
