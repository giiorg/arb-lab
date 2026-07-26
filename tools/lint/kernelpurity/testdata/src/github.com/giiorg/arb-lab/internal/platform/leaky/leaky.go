// Package leaky is the offending fixture: a shared-kernel package that reaches
// out of the kernel in both of the ways the gate exists to catch.
package leaky

import (
	"fmt"

	// Spine inversion: engine/ already sits on top of platform/, so importing
	// it from the kernel makes the dependency spine cyclic in principle.
	"github.com/giiorg/arb-lab/internal/engine/venue" // want `platform must not import "github\.com/giiorg/arb-lab/internal/engine/venue"`

	// Domain leak: the kernel learning a business concept is what turns a
	// technical shared library into a shared DOMAIN kernel.
	"github.com/giiorg/arb-lab/internal/oms/domain" // want `platform must not import "github\.com/giiorg/arb-lab/internal/oms/domain"`
)

func Describe() string {
	return fmt.Sprint(venue.Name, domain.Name)
}
