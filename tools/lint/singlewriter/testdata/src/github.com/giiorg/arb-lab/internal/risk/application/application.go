// Package application is another context reaching into the OMS — rule 6's
// positive control, and one the type-checker will actually admit.
//
// That last part is not free. §3.6 records why rule 1's obvious probe scores a
// pass without executing: application/ importing infrastructure/ is an import
// cycle, so the compiler refuses before the analyzer runs. This gate's probe
// has no such problem — risk importing oms is a plain edge between two
// contexts, with no cycle — but the fixture is written out rather than assumed
// so the next gate's author sees the check being made.
package application

import (
	"github.com/giiorg/arb-lab/internal/oms/application/ports"     // want `only oms may import the OMS write path .*application/ports.*§3.6 rule 6`
	"github.com/giiorg/arb-lab/internal/oms/application/ports/sub" // want `only oms may import the OMS write path .*application/ports/sub.*§3.6 rule 6`
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven" // want `only oms may import the OMS write path .*infrastructure/driven.*§3.6 rule 6`

	// A third-party package laid out as oms/infrastructure/driven is not this
	// system's OMS. Reporting it would be the gate inventing a violation, so
	// the /internal/ scoping is measured here rather than asserted in a
	// comment.
	vendored "example.com/thirdparty/oms/infrastructure/driven"
)

// Gate stands in for a risk use case that has acquired write authority it must
// not have.
type Gate struct {
	Repo     ports.OrderWriteRepository
	Second   sub.Repo
	Concrete driven.Memory
	Vendored vendored.Client

	// The domain import above is deliberately unreported: a read model naming
	// an order state is §5.1's projection pattern, and denying it would force a
	// second copy of the state set.
	Observed domain.State
}
