// Package oms is the context ROOT — §3.6 rule 7's module.go, the one place
// that knows which adapter satisfies which port.
//
// It is the fixture for within()'s equality arm on the OWNER side: this
// package's path is exactly internal/oms, so a prefix-only exemption test would
// leave the composition seam unable to compose anything. Every import below is
// clean.
package oms

import (
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
)

func New() ports.OrderWriteRepository { return driven.Memory{} }
