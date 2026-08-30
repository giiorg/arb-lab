// Package serve_test is the NEGATIVE mirror of the two exempt external test
// packages: the suffix is stripped here too, and what the strip reveals is a
// package that is not granted. Without this fixture, "strip then check the
// grant list" and "strip then exempt" are the same predicate on every fixture
// in this tree.
package serve_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/oms" // want `not granted OMS write authority.*OMS control surface`
)

func TestServer(t *testing.T) {
	var _ oms.Module
}
