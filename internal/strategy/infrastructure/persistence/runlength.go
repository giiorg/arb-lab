package persistence

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// RunLength is how long a persistence episode lasts, at one entry level.
//
// The coordinate rides on the value rather than being a required argument, and
// the reason is F55/F60/F83/F84/F100: the family is not "the function forgot an
// argument", it is "the value travelled without its base". A required argument
// cannot reach three hops downstream, where HANDOFF.md restates "median episode
// is 2 settlements" with no level attached — and at two of the four declared
// levels the median is 1. A coordinate on the struct can, because a screen
// renders what it holds.
type RunLength struct {
	// P50, P75, P90 are nearest-rank percentiles over the run lengths, and Max
	// is the longest. Over an empty population all four are 0.
	P50, P75, P90, Max int

	// Mean is the arithmetic mean run length, as a ratio at the canonical
	// scale. Over an empty population it is a CONSTRUCTED zero, never the zero
	// value: "no episodes" and "nobody computed this" are different facts.
	Mean fixedpoint.Rate

	// Episodes is the size of the run-length population — censored records
	// excluded.
	Episodes int

	// Censored is how many entries triggered with no settlement after them.
	//
	// F103: five such records exist in the committed file, all at level 0.0002,
	// all with start == end, all accruing "0", all at the tape's final decision
	// point. summarise.py counts them in `episodes` and averages them in, which
	// moves the published run_mean from 2.434 to 2.433. They are right-censored
	// observations, not runs of length zero, so they are reported here rather
	// than admitted to the population — and reported rather than dropped
	// silently, because a shrinking denominator nobody records is F60's defect.
	Censored int

	// At is the coordinate this answer was computed at.
	At EntryAt
}

// RunLength folds the file's episodes at one entry level.
func (s Set) RunLength(at EntryAt) (RunLength, error) {
	rows, err := s.at(at)
	if err != nil {
		return RunLength{}, err
	}

	var runs []int
	censored := 0
	total := 0
	for _, r := range rows {
		censored += len(r.censored)
		for _, e := range r.episodes {
			runs = append(runs, e.Run)
			total += e.Run
		}
	}
	sort.Ints(runs)

	out := RunLength{Episodes: len(runs), Censored: censored, At: at, Mean: fixedpoint.ZeroRate()}
	if len(runs) == 0 {
		return out, nil
	}
	out.P50 = percentile(runs, 50)
	out.P75 = percentile(runs, 75)
	out.P90 = percentile(runs, 90)
	out.Max = runs[len(runs)-1]

	sum, err := fixedpoint.ParseRate(strconv.Itoa(total))
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: run total %d: %w", total, err)
	}
	n, err := fixedpoint.ParseRate(strconv.Itoa(len(runs)))
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: run count %d: %w", len(runs), err)
	}
	mean, err := sum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: mean run length: %w", err)
	}
	out.Mean = mean
	return out, nil
}

// percentile is summarise.py:38's nearest-rank rule, replicated exactly:
// sorted[min(len-1, (len*q)//100)]. It is NOT a midpoint or interpolated
// percentile, and the difference is visible at small populations — for two
// elements the 50th percentile is the SECOND one.
//
// sorted must already be ascending and non-empty.
func percentile(sorted []int, q int) int {
	i := (len(sorted) * q) / 100
	if i > len(sorted)-1 {
		i = len(sorted) - 1
	}
	return sorted[i]
}
