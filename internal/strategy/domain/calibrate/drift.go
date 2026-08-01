package calibrate

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// EntryDriftFrom freezes §5.2's entry_drift_pct from the dispersion
// distribution's adverse tail (§41b family 1): the magnitude of the shift
// at the committed tail quantile, annualized to the unit scaledThreshold
// consumes — so the margin charged against an entry is exactly the funding
// shortfall of a hold whose drift sits at that tail for its whole span.
//
// # The rule, stated
//
// The tail must be a MEMBER of the measured grid — selecting between
// points would re-answer a question ShiftAt already answers at read time,
// and answering it differently here would freeze a number the runtime
// never draws. A tail shift at or above zero annualizes to a DECLARED
// zero margin: an era whose adverse tail never went negative measured no
// drift for a receiver to fear, and inventing a floor here would be a
// threshold chosen outside the governance. Negative shifts (the receiver's
// adverse direction) freeze as their positive magnitude.
func EntryDriftFrom(points []domain.QuantilePoint, tail fixedpoint.Rate, interval time.Duration) (fixedpoint.Rate, error) {
	if len(points) == 0 {
		return fixedpoint.Rate{}, ErrNoResiduals
	}
	if interval <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("entry drift interval %v: %w", interval, domain.ErrUnset)
	}
	if err := tail.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("entry drift tail: %w", err)
	}
	for _, p := range points {
		c, err := tail.Cmp(p.Q)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("entry drift tail: %w", err)
		}
		if c != 0 {
			continue
		}
		sign, err := p.Shift.Sign()
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("entry drift shift: %w", err)
		}
		if sign >= 0 {
			return fixedpoint.ZeroRate(), nil
		}
		magnitude, err := p.Shift.Neg()
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("entry drift shift: %w", err)
		}
		return cashcarry.Annualized(magnitude, interval)
	}
	return fixedpoint.Rate{}, fmt.Errorf("tail %s: %w", tail, ErrTailOffGrid)
}
