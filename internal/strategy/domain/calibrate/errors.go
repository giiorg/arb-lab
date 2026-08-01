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

	// ErrNoExcursions reports an era in which no dip below the clearing
	// level ever recovered within the dwell — a band width would have to
	// be invented, and what an evidence-free era means is the
	// governance's question, not this package's.
	ErrNoExcursions = fmt.Errorf("strategy/calibrate: no recovered excursion to measure a band from")

	// ErrSeriesMismatch reports two kline series that do not describe the
	// same instants, assets, or step — a basis computed across misaligned
	// grids is a subtraction between two different moments.
	ErrSeriesMismatch = fmt.Errorf("strategy/calibrate: kline series misaligned")

	// ErrHorizonOffGrid reports a hold horizon that is not a positive
	// whole number of kline steps — the move would need instants the
	// series does not carry.
	ErrHorizonOffGrid = fmt.Errorf("strategy/calibrate: horizon not a whole number of kline steps")
)
