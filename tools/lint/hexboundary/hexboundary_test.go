package hexboundary_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/hexboundary"
)

// The gate under test: build-spec §3.6 rule 1 — domain/ and application/ must
// not import infrastructure/ or engine/.
//
// The `// want` comments in testdata are what make this a gate rather than a
// decoration: analysistest fails both when an expected diagnostic is missing
// AND when an unexpected one appears. So the clean fixtures are not padding —
// they are the half of the suite that proves the analyzer does not over-report,
// and every one of them is pinned to a specific wrong reading of the rule.
func TestHexBoundary(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), hexboundary.Analyzer,
		// Offending, from BOTH policed layers. Testing only one would score
		// identically against an analyzer that policed only that one.
		"github.com/giiorg/arb-lab/internal/marketdata/domain/sub", // nested domain: a subdirectory is not an escape hatch
		"github.com/giiorg/arb-lab/internal/marketdata/application",

		// Clean, and each pinned to a distinct wrong reading:
		//
		//   domain          — stdlib, kernel, another context's domain, a
		//                     third-party path containing "engine", a
		//                     context-local package named "engine", and
		//                     internal/engineering. Five ways to over-report.
		//   infrastructure/driven — the adapter importing the parity seam and
		//                     the domain it adapts, which is the pattern
		//                     working; separates the rule from "nobody may
		//                     import engine/".
		//   infrastructure/domain — a domain segment nested under
		//                     infrastructure. Separates "first layer segment
		//                     wins" from "any domain segment anywhere".
		//   oms/domainservice     — the importer-side substring trap.
		"github.com/giiorg/arb-lab/internal/marketdata/domain",
		"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven",
		"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/domain",
		"github.com/giiorg/arb-lab/internal/oms/domainservice",
	)
}
