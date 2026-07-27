// Package application stands in for oms's use cases, which import their own
// ports freely: rule 6 forbids bypassing the write path, and a use case IS the
// write path.
package application

import "github.com/giiorg/arb-lab/internal/oms/application/ports"

type Dispatcher struct{ Repo ports.OrderWriteRepository }
