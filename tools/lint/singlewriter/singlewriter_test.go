package singlewriter_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/giiorg/arb-lab/tools/lint/singlewriter"
)

// The gate under test: only oms may import the OMS write path —
// application/ports and infrastructure/driven — per build-spec §3.6 rule 6.
//
// The `// want` comments in testdata are what make this a gate rather than a
// decoration: analysistest fails both when an expected diagnostic is missing
// AND when an unexpected one appears, so the clean packages below are load
// bearing. Rule 6's own risk is over-reporting, since the ONE context that may
// import these packages is also the context that contains them.
func TestSingleWriter(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), singlewriter.Analyzer,
		// Clean, and each is a different way of being inside oms:
		//   - the context ROOT, whose path equals the exemption exactly
		//     (within()'s equality arm on the owner side, which every
		//     subpackage fixture leaves untested);
		//   - a use case importing its own ports, because rule 6 forbids
		//     bypassing the write path rather than using it;
		//   - the adapter importing the port it satisfies;
		//   - the root's EXTERNAL TEST package, which is a different shape from
		//     all of the above and is the one this gate got wrong. Go suffixes
		//     the last SEGMENT, so oms's own test suite loads as
		//     internal/oms_test and matches neither of within()'s arms. The
		//     other two external test packages in the real tree
		//     (oms/application_test, oms/infrastructure/driven_test) sit BELOW
		//     the exempt path and are covered by the prefix arm, which is why
		//     nothing here had this shape until oms/module_test.go was written
		//     and the gate reported it. Loaded from oms_x_test.go under the
		//     package below — analysistest loads with tests enabled, and the
		//     mutation harness confirms this fixture is analysed rather than
		//     merely present.
		"github.com/giiorg/arb-lab/internal/oms",
		"github.com/giiorg/arb-lab/internal/oms/application",
		"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven",

		// Offending, and the positive control: another context taking write
		// authority three ways — the port root (within()'s equality arm), a
		// package beneath it (the prefix arm) and the concrete repository. Its
		// clean imports matter as much: oms/domain is deliberately unpoliced,
		// and a third-party package laid out the same way without an
		// /internal/ segment is not ours to report.
		"github.com/giiorg/arb-lab/internal/risk/application",

		// Offending, sibling prefix: internal/omsx is not internal/oms, and a
		// bare HasPrefix exemption would silently hand it the one context's
		// authority.
		"github.com/giiorg/arb-lab/internal/omsx",

		// Offending, the composition root — the deliberate reading, since rule
		// 6 says "other contexts" and a root is not one.
		"github.com/giiorg/arb-lab/internal/app/arblab",

		// Offending from OUTSIDE internal/, which is the fixture that forces
		// the module root to be resolved from the imported path rather than
		// from the analyzed one.
		"github.com/giiorg/arb-lab/cmd/arblab",
	)
}
