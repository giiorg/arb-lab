package persistence

import (
	"fmt"
	"strconv"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// NetPerEpisode is the pooled per-episode economics at one (level, cost).
//
// Both figures are RATIOS, not percentages and not currency: this is what one
// episode accrues per unit of leg notional. Turning that into money needs a
// notional, which is why PerSymbol takes a SizeAt and this does not.
type NetPerEpisode struct {
	// Episodes is the population size — censored records excluded.
	Episodes int

	// Censored is how many entries at this level triggered with no settlement
	// after them, and so are in no mean and in no win count here.
	//
	// It rides on the value for the same reason the coordinate does. F103's
	// five records are counted in summarise.py's published `episodes` (12 322
	// against this reader's 12 317) and averaged into its published means, so a
	// figure from this fold and a figure from the artifact are answers over
	// DIFFERENT populations. Without this field the difference is a silent
	// shrinking denominator, which is F60's defect — and RunLength reporting it
	// does not help a caller holding a NetPerEpisode. Episodes + Censored is
	// the artifact's count.
	Censored int

	// Wins counts episodes whose accrual was STRICTLY greater than the cost.
	Wins int

	// MeanGross is the mean accrual before cost; MeanNet subtracts one
	// round-trip cost per episode.
	MeanGross fixedpoint.Rate
	MeanNet   fixedpoint.Rate

	// At is the coordinate this answer was computed at.
	At CostAt
}

// NetPerEpisode folds the accruals at one entry level against one cost.
func (s Set) NetPerEpisode(at CostAt) (NetPerEpisode, error) {
	if err := at.Validate(); err != nil {
		return NetPerEpisode{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return NetPerEpisode{}, err
	}

	sum := fixedpoint.ZeroRate()
	count, wins, censored := 0, 0, 0
	for _, r := range rows {
		censored += len(r.censored)
		for _, e := range r.episodes {
			count++
			sum, err = sum.Add(e.Accrued)
			if err != nil {
				return NetPerEpisode{}, fmt.Errorf("persistence: accumulate accrual: %w", err)
			}
			cmp, err := e.Accrued.Cmp(at.Cost)
			if err != nil {
				return NetPerEpisode{}, fmt.Errorf("persistence: compare accrued to cost: %w", err)
			}
			if cmp > 0 {
				wins++
			}
		}
	}

	out := NetPerEpisode{
		Episodes: count, Wins: wins, Censored: censored, At: at,
		MeanGross: fixedpoint.ZeroRate(), MeanNet: fixedpoint.ZeroRate(),
	}
	if count == 0 {
		return out, nil
	}

	n, err := fixedpoint.ParseRate(strconv.Itoa(count))
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: episode count %d: %w", count, err)
	}
	gross, err := sum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: mean gross: %w", err)
	}
	// Total cost is one round trip per episode, subtracted before the mean —
	// the same order summarise.py:53 uses, so the two implementations round at
	// the same point rather than agreeing by luck.
	totalCost, err := at.Cost.Mul(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: total cost: %w", err)
	}
	netSum, err := sum.Sub(totalCost)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: net total: %w", err)
	}
	net, err := netSum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: mean net: %w", err)
	}
	out.MeanGross, out.MeanNet = gross, net
	return out, nil
}
