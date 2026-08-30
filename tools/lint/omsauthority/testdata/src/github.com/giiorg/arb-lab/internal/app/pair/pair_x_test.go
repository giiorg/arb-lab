// Package pair_test is a GRANTED root's external test package, and it is the
// MIRROR of the owner fixture rather than a duplicate of it. Both are exempt,
// and they are exempt through different branches: the owner through within(),
// this one through the grant list. A gate that stripped the _test suffix only
// on the owner side would report this file, which is singlewriter's own defect
// moved one branch over.
package pair_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/oms"
)

func TestDrive(t *testing.T) {
	var _ oms.Module
}
