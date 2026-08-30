// Package httpapi is record §100 §15.7's probe, kept as a fixture. Both imports
// were measured silent against all six gates, and the second is the route that
// no facade can close: oms.ReconcileMarginOp is a package-level function over
// ClearanceDeps{JournalPath, Account string}, so there is no value to wrap.
package httpapi

import (
	"github.com/giiorg/arb-lab/internal/oms"             // want `not granted OMS write authority.*OMS control surface`
	"github.com/giiorg/arb-lab/internal/oms/application" // want `not granted OMS write authority.*the use cases themselves`
)

// Handler is the realistic accident: a web handler that reached for the module
// its own composition root legitimately holds.
type Handler struct {
	M oms.Module
	E application.Executor
}
