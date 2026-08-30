// Package pair is a GRANTED root: §98's cross-venue pair driver.
package pair

import (
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
)

// Drive holds both halves of the surface, which is what the grant is for.
type Drive struct {
	M oms.Module
	E application.Executor
}
