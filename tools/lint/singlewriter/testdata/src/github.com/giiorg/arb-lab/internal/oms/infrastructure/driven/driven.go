// Package driven stands in for oms's write repositories — rule 6's literal
// subject.
package driven

import "github.com/giiorg/arb-lab/internal/oms/application/ports"

// Memory is the adapter, and its import of the port above is CLEAN: oms
// importing its own write path is the pattern working, not a violation.
type Memory struct{}

func (Memory) ApplyState(id string, expected int64, to string) error { return nil }

var _ ports.OrderWriteRepository = Memory{}
