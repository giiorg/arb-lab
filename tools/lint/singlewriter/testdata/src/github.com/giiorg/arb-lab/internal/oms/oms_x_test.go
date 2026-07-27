// Package oms_test is the context root's EXTERNAL test package, and it is the
// fixture for the one shape within() alone answers wrongly.
//
// Go suffixes the LAST SEGMENT of a package's path to name its external test
// package, so this loads as internal/oms_test — not as a package beneath
// internal/oms. within()'s equality arm matches internal/oms and its prefix arm
// matches internal/oms/, so both say NO here, and the owner's own test suite
// got reported as a second writer to order state.
//
// Every other external test package in the real tree hid the defect by
// accident: internal/oms/application_test and
// internal/oms/infrastructure/driven_test sit BELOW the exempt path, so the
// prefix arm already covers them. Only the OWNER package's external test
// suffixes the exempt path itself, and this testdata tree had no example of it.
//
// The import below is what a real module_test.go needs and cannot avoid: a test
// double for ports.OrderPlacer has to name the port's own types in its method
// signatures. Every import here is CLEAN.
package oms_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
)

// fakeRepository is the test double the import exists for.
type fakeRepository struct{}

func (fakeRepository) ApplyState(id string, expected int64, to string) error { return nil }

var _ ports.OrderWriteRepository = fakeRepository{}

func TestOwnerTestPackageMayNameItsOwnPorts(t *testing.T) {
	// The adapter too: an external test package of the root is also where a
	// composed-context test constructs the real thing.
	var _ ports.OrderWriteRepository = driven.Memory{}
}
