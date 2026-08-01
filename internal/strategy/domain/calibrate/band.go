package calibrate

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// ExitBandWidth freezes the hysteresis band's width from measured regime
// persistence (§2.4's own words, §41b family 1): the committed rule reads
// calibration-era settlement excursions — dips below the entry-clearing
// level and whether they recovered within the dwell — and emits the width
// in the annualized unit scaledThreshold consumes.
//
// # The rule, stated
//
// A DIP is a maximal run of consecutive settlements whose settled rate is
// strictly below the clearing level; a settlement exactly AT the level
// clears entry (the inequality is ≥), so it ends a dip rather than
// deepening one. A dip RECOVERED WITHIN THE DWELL iff the rate returned to
// or above the level before the dwell would have fired — run length
// strictly under persist, because the deployed Continue stands down at the
// persist-th consecutive settlement observed below. A run still open when
// the era ends is not evidence of recovery and is excluded. The width is
// the q-th quantile of recovered-dip depths (depth = level minus the run's
// minimum), selected at the UPPER order statistic: a wider band is §2.4's
// own direction ("materially more adverse than entry"), and the band
// exists so that dips history shows to be recoverable do not trigger the
// exit the dwell would otherwise arm. Depths annualize exactly as
// EntryDrift does — a sustained per-settlement shortfall of d over the
// remaining hold scores −Annualized(d)·C·fraction, which is precisely the
// floor Continue compares against.
//
// An era with no recovered dip refuses (ErrNoExcursions): the number would
// otherwise be invented, and what to do about an era that measured
// nothing is the governance's decision, not this function's.
func ExitBandWidth(history []domain.RealizedSettlement, clearing fixedpoint.Rate, persist uint32, q fixedpoint.Rate, interval time.Duration) (fixedpoint.Rate, error) {
	if err := validateHistory(history); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("exit band: %w", err)
	}
	if err := clearing.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("exit band clearing: %w", err)
	}
	if persist == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("exit band persist: %w", domain.ErrUnset)
	}
	if err := validateGrid([]fixedpoint.Rate{q}); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("exit band quantile: %w", err)
	}
	if interval <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("exit band interval %v: %w", interval, domain.ErrUnset)
	}

	var depths []fixedpoint.Rate
	var runLen uint32
	var runMin fixedpoint.Rate
	for _, s := range history {
		c, err := s.Rate.Cmp(clearing)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("exit band at %s: %w", s.At.Format(time.RFC3339), err)
		}
		if c < 0 {
			if runLen == 0 {
				runMin = s.Rate
			} else if mc, err := s.Rate.Cmp(runMin); err != nil {
				return fixedpoint.Rate{}, fmt.Errorf("exit band at %s: %w", s.At.Format(time.RFC3339), err)
			} else if mc < 0 {
				runMin = s.Rate
			}
			runLen++
			continue
		}
		// The run just ended at or above the level: recovered iff it was
		// shorter than the dwell that would have closed it.
		if runLen > 0 && runLen < persist {
			depth, err := clearing.Sub(runMin)
			if err != nil {
				return fixedpoint.Rate{}, fmt.Errorf("exit band depth: %w", err)
			}
			depths = append(depths, depth)
		}
		runLen = 0
	}
	// A run still open at the era's end recovered from nothing — excluded,
	// whatever its length.

	if len(depths) == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("clearing %s over %d rows: %w", clearing, len(history), ErrNoExcursions)
	}

	idx, err := ceilIndex(q, len(depths))
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("exit band quantile: %w", err)
	}
	sorted := sortRates(depths)
	return cashcarry.Annualized(sorted[idx], interval)
}

// ceilIndex resolves quantile q over n sorted observations to the UPPER
// bracketing order statistic — ceil of the exact rational rank q·(n−1),
// whichever tail q sits in. Selection-only conservatism for quantities
// where larger is always the harder number (band widths, adverse moves).
func ceilIndex(q fixedpoint.Rate, n int) (int, error) {
	if err := q.Validate(); err != nil {
		return 0, err
	}
	num := q.Mantissa() * int64(n-1)
	const den = int64(1_000_000_000)
	idx := num / den
	if num%den != 0 {
		idx++
	}
	return int(idx), nil
}
