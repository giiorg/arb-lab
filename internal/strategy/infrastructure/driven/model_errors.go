package driven

import "fmt"

// Sentinels for the model adapters (the reference store's live in
// reference.go). Callers branch with errors.Is; each names the rule that
// refused, per §40f's journaled-refusal discipline.
var (
	// ErrQtyNotPositive reports a candidate quantity at or below zero. A
	// zero candidate prices as a free execution and a negative one as a
	// credit; neither is an execution anybody submits.
	ErrQtyNotPositive = fmt.Errorf("strategy/driven: candidate quantity not positive")

	// ErrChargeNegative reports a measured charge with a negative sign — an
	// adverse move or slippage figure that would CREDIT the position. That
	// is a sign error in the artifact, not a conservative number.
	ErrChargeNegative = fmt.Errorf("strategy/driven: measured charge is negative")

	// ErrTacticUncalibrated reports an execution tactic the calibration
	// never priced. The maker-entry fork is a documented opt-in (§2.4,
	// §12.2a) and needs its own artifact before it can be charged for.
	ErrTacticUncalibrated = fmt.Errorf("strategy/driven: tactic not covered by the calibration")

	// ErrLatencyOutsideCalibration reports a latency assumption LONGER than
	// the artifact measured. The adverse-move charge was measured over the
	// calibrated window; a longer window can move further, and the artifact
	// has no number for it. (A shorter assumption is admissible — the
	// calibrated charge then overstates, which is the conservative
	// direction.)
	ErrLatencyOutsideCalibration = fmt.Errorf("strategy/driven: latency assumption exceeds the calibrated window")

	// ErrSizeOutsideCalibration reports a candidate quantity past the last
	// p90 bucket. §8 conditions measured slippage on order size; beyond the
	// measured range there is no number, and extrapolating one would be the
	// model grading its own homework.
	ErrSizeOutsideCalibration = fmt.Errorf("strategy/driven: quantity outside the calibrated size range")

	// ErrBookUnavailable reports a leg whose book the source does not hold.
	// A cost priced against no book is a cost made up.
	ErrBookUnavailable = fmt.Errorf("strategy/driven: no book for this leg")

	// ErrDepthInsufficient reports visible depth too thin to fill the
	// candidate quantity — on either side, since the bound prices the worse
	// one. The conservative answer to "the book cannot absorb this order"
	// is a refusal, not a large number.
	ErrDepthInsufficient = fmt.Errorf("strategy/driven: book depth cannot fill the quantity")

	// ErrBucketOrder reports p90 cells whose MaxQty bounds are not strictly
	// ascending within one (instrument, side, stratum) group — two cells
	// claiming one size range.
	ErrBucketOrder = fmt.Errorf("strategy/driven: cost cells out of order")

	// ErrCellStratumUnknown reports a cell citing a stratum the
	// calibration's own classifier never declares. The classifier can never
	// produce the name, so the cell is unreachable — a number the artifact
	// froze that no book can ever select.
	ErrCellStratumUnknown = fmt.Errorf("strategy/driven: cell stratum not in the classifier")

	// ErrCellThin reports a cell whose observation count is below the
	// calibration's own declared minimum. §41b-ii-iii: a cell below the
	// evidence floor is ABSENT — absent is a refusal at lookup, and a
	// present-but-thin cell is a contradiction refused at construction,
	// never a number with an apology attached.
	ErrCellThin = fmt.Errorf("strategy/driven: cell below the minimum evidence count")

	// ErrStratumUncalibrated reports a lookup landing in a (instrument,
	// side, stratum) population the artifact holds no cells for. The
	// classified stratum is real; the artifact has no number for it on this
	// leg — and the answer is a refusal naming the population, never the
	// nearest cell (§41b-ii-b).
	ErrStratumUncalibrated = fmt.Errorf("strategy/driven: no cells for this leg, side and stratum")
)
