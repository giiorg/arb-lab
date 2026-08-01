package calibrate

import "fmt"

// Sentinels. Every refusal names the rule that fired; the wrapping message
// carries the offending value.
var (
	// ErrHistoryShort reports a history too short to collect a single
	// residual under the saturated-window warmup rule — the trailing
	// window never came to span the EWMA's own horizon.
	ErrHistoryShort = fmt.Errorf("strategy/calibrate: history shorter than the EWMA saturation horizon")

	// ErrNoResiduals reports an empty residual set where a distribution
	// was to be measured. A quantile of nothing is not a conservative
	// zero — it is a refusal.
	ErrNoResiduals = fmt.Errorf("strategy/calibrate: no residuals to take quantiles over")

	// ErrGridInvalid reports a quantile grid that is empty, out of (0,1),
	// or not strictly ascending.
	ErrGridInvalid = fmt.Errorf("strategy/calibrate: quantile grid invalid")

	// ErrTailOffGrid reports an EntryDrift tail quantile that is not a
	// member of the measured grid. Selecting a nearby point would
	// understate a tail the grid never reached — the flattering
	// direction, refused for ShiftAt's own reason.
	ErrTailOffGrid = fmt.Errorf("strategy/calibrate: tail quantile not on the measured grid")
)
