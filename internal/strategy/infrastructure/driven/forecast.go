package driven

import (
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// The default funding estimator is pure domain math (§40e: the port earns its
// seam from §2.4's swappability requirement, not from I/O), so the "adapter"
// is the conformance assertion alone — the wiring constructs a
// domain.EWMAEstimator and hands it straight to the port. §3.6 rule 2 puts
// the assertion here rather than in domain, because domain naming the ports
// package would point the dependency outward.
var _ ports.FundingForecast = domain.EWMAEstimator{}
