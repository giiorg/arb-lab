package nofloat_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/nofloat"
)

// The gate under test: no binary floating point anywhere under internal/, unless
// the line carries //nofloat:allow <reason>.
//
// The `// want` comments in testdata are what make this a gate rather than a
// decoration — analysistest fails both when an expected diagnostic is MISSING
// and when an unexpected one APPEARS. So the clean fixtures prove the rule does
// not over-report exactly as strongly as the offending fixture proves it bites,
// and a default-deny rule needs both: one that fires on everything would pass a
// test that only checked the offending cases.
func TestNoFloat(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nofloat.Analyzer,
		// Clean: integer money arithmetic, the shape the rule exists to force.
		"github.com/giiorg/arb-lab/internal/platform/fixedpoint",
		// Clean: the escape hatch used properly, on the seam §3 anticipated.
		"github.com/giiorg/arb-lab/internal/platform/observability",
		// Offending: every door a float arrives through, including the three
		// with no float token on the line.
		"github.com/giiorg/arb-lab/internal/engine/floaty",
		// Offending: the three ways the escape hatch can be misused.
		"github.com/giiorg/arb-lab/internal/engine/badescape",
	)
}

// The rule is scoped to internal/, and that scope is part of the contract rather
// than an implementation detail: cmd/ wiring and any future public package are
// outside it. Asserting it here means the scope cannot be widened by accident
// during a refactor of moduleRoot-style path handling.
func TestNoFloat_outsideInternalIsNotPoliced(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nofloat.Analyzer,
		"github.com/giiorg/arb-lab/cmd/floatingcmd",
	)
}
