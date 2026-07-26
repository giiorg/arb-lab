package kernelpurity_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/kernelpurity"
)

// The gate under test: a package under internal/platform/ may import only the
// standard library and other internal/platform/ packages.
//
// The `// want` comments in testdata are what make this a gate rather than a
// decoration — analysistest fails both when an expected diagnostic is missing
// AND when an unexpected one appears, so a clean fixture proves the analyzer
// does not over-report just as an offending fixture proves it bites.
func TestKernelPurity(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), kernelpurity.Analyzer,
		// Clean: stdlib only.
		"github.com/giiorg/arb-lab/internal/platform/clock",
		// Clean: one platform package importing another.
		"github.com/giiorg/arb-lab/internal/platform/fixedpoint",
		// Offending: the shared kernel reaching into a bounded context, and
		// the shared kernel inverting the dependency spine via engine/.
		"github.com/giiorg/arb-lab/internal/platform/leaky",
		// Not subject to the rule: a context may import whatever it likes.
		"github.com/giiorg/arb-lab/internal/oms/domain",
	)
}
