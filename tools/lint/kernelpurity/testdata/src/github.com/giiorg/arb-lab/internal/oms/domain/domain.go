// Package domain stands in for a bounded context's domain layer. It is NOT
// subject to the kernel-purity rule, so importing platform and engine from
// here must produce no diagnostic — that is what keeps the gate from
// over-reporting.
package domain

import (
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

const Name = "domain"

var (
	_ = clock.Now
	_ = venue.Name
)
