package calibrate

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Points measures the empirical quantile points of a residual set at a
// committed grid, under the same outward-from-the-median selection rule
// Dispersion.ShiftAt applies at read time: the two rules must agree on
// which way "conservative" points, or the artifact would be built under
// one convention and consumed under another.
//
// # Order statistics, never interpolation
//
// Each point is an OBSERVED residual: the rank q·(n−1) is computed as an
// exact rational, and a rank between two order statistics resolves
// outward — the lower statistic below the median, the upper one at or
// above it. Both directions overstate the tail they serve; interpolating
// would mint a residual nobody measured, and nearest-rank would flatter
// half the draws. The returned points are strictly ascending in Q (the
// grid's own discipline) with non-decreasing shifts (sorted order
// statistics under monotone ranks), so Dispersion.Validate accepts them
// by construction.
func Points(residuals []fixedpoint.Rate, grid []fixedpoint.Rate) ([]domain.QuantilePoint, error) {
	if len(residuals) == 0 {
		return nil, ErrNoResiduals
	}
	for i, r := range residuals {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("residual %d: %w", i, err)
		}
	}
	if err := validateGrid(grid); err != nil {
		return nil, err
	}

	sorted := sortRates(residuals)

	half, err := fixedpoint.ParseRate("0.5")
	if err != nil {
		panic(err) // programmer constant
	}

	out := make([]domain.QuantilePoint, 0, len(grid))
	for _, q := range grid {
		idx, err := outwardIndex(q, half, len(sorted))
		if err != nil {
			return nil, fmt.Errorf("quantile %s: %w", q, err)
		}
		out = append(out, domain.QuantilePoint{Q: q, Shift: sorted[idx]})
	}
	return out, nil
}

// sortRates returns an ascending copy. Sorting by mantissa is exactly
// Rate ordering: every Rate lives at the one canonical scale, so the
// integer order IS the value order — and it cannot error mid-sort the way
// Cmp-based comparison would have to ignore. Callers validate every
// element first.
func sortRates(rs []fixedpoint.Rate) []fixedpoint.Rate {
	sorted := append([]fixedpoint.Rate(nil), rs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Mantissa() < sorted[j].Mantissa() })
	return sorted
}

// outwardIndex resolves quantile q over n sorted observations to an index,
// rounding the exact rational rank q·(n−1) outward from the median: floor
// below it, ceil at or above it.
func outwardIndex(q, half fixedpoint.Rate, n int) (int, error) {
	// The rank's exact value is q.Mantissa()·(n−1) / 10^9. Both factors
	// are bounded (a validated quantile's mantissa is under 10^9, and n is
	// a residual count), so the product fits an int64 with room to spare.
	num := q.Mantissa() * int64(n-1)
	const den = int64(1_000_000_000)
	idx := num / den
	tail, err := q.Cmp(half)
	if err != nil {
		return 0, err
	}
	if tail >= 0 && num%den != 0 {
		idx++
	}
	return int(idx), nil
}

// validateGrid demands a non-empty, strictly ascending grid of quantiles
// strictly inside (0,1) — the same range rule the domain applies to every
// quantile, restated here because the grid exists before any domain value
// does.
func validateGrid(grid []fixedpoint.Rate) error {
	if len(grid) == 0 {
		return fmt.Errorf("empty grid: %w", ErrGridInvalid)
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err) // programmer constant
	}
	for i, q := range grid {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("grid %d: %w: %w", i, ErrGridInvalid, err)
		}
		sign, err := q.Sign()
		if err != nil {
			return fmt.Errorf("grid %d: %w", i, err)
		}
		c, err := q.Cmp(one)
		if err != nil {
			return fmt.Errorf("grid %d: %w", i, err)
		}
		if sign <= 0 || c >= 0 {
			return fmt.Errorf("grid %d quantile %s outside (0,1): %w", i, q, ErrGridInvalid)
		}
		if i > 0 {
			prev, err := grid[i-1].Cmp(q)
			if err != nil {
				return fmt.Errorf("grid %d: %w", i, err)
			}
			if prev >= 0 {
				return fmt.Errorf("grid %d %s not above %d %s: %w", i, q, i-1, grid[i-1], ErrGridInvalid)
			}
		}
	}
	return nil
}
