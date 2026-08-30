// Package oms is the owner. within()'s EQUALITY arm on the owner side: every
// other exempt fixture here is a SUBpackage, which the prefix arm covers for
// free, so without this one the equality arm is untested.
package oms

import (
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
)

// Module is the eleven-use-case control surface.
type Module struct {
	Execute application.Executor
	Writer  ports.OrderWriteRepository
}
