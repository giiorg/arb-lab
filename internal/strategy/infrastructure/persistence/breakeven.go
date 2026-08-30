package persistence

import (
	"fmt"
	"sort"
)

// breakevenCap is the top bucket. Runs at or above it collapse into one row,
// replicating summarise.py:82's min(e["run"], 12).
const breakevenCap = 12

// BreakevenRow is one run-length bucket: how many episodes ran that long, and
// how many of them cleared the round-trip cost.
type BreakevenRow struct {
	// Run is the bucket's run length; Plus marks the open-ended top bucket,
	// which the artifact renders as "12+".
	Run  int
	Plus bool

	// Episodes is how many episodes fell in the bucket, and ClearedCost how
	// many of those accrued STRICTLY more than the cost.
	Episodes    int
	ClearedCost int
}

// BreakevenTable answers record §99's "run length is not the break-even
// question": a long run is not the same thing as a profitable one.
//
// summary.json publishes exactly one of these, at entry level 0.0005 and cost
// 0.0021, and records neither — F55. Here both are named by the caller and
// carried on the answer.
type BreakevenTable struct {
	Rows []BreakevenRow
	At   CostAt
}

// Breakeven buckets the episodes at one entry level by run length and counts,
// per bucket, how many cleared the named cost.
func (s Set) Breakeven(at CostAt) (BreakevenTable, error) {
	if err := at.Validate(); err != nil {
		return BreakevenTable{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return BreakevenTable{}, err
	}

	episodes := map[int]int{}
	cleared := map[int]int{}
	for _, r := range rows {
		for _, e := range r.episodes {
			b := e.Run
			if b > breakevenCap {
				b = breakevenCap
			}
			episodes[b]++
			cmp, err := e.Accrued.Cmp(at.Cost)
			if err != nil {
				return BreakevenTable{}, fmt.Errorf("persistence: compare accrued to cost: %w", err)
			}
			// Strictly greater. summarise.py:86 writes `>`, and the committed
			// data cannot separate `>` from `>=` — measured: exactly zero of
			// 17 478 episodes accrue any of the three costs. The hand-built
			// fixture in net_test.go is what makes the choice checkable.
			if cmp > 0 {
				cleared[b]++
			}
		}
	}

	buckets := make([]int, 0, len(episodes))
	for b := range episodes {
		buckets = append(buckets, b)
	}
	sort.Ints(buckets)

	table := BreakevenTable{At: at, Rows: make([]BreakevenRow, 0, len(buckets))}
	for _, b := range buckets {
		table.Rows = append(table.Rows, BreakevenRow{
			Run: b, Plus: b == breakevenCap,
			Episodes: episodes[b], ClearedCost: cleared[b],
		})
	}
	return table, nil
}
