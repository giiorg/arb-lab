package crossvenue

// Settlement pairing — the screening's committed rules (§43b) restated as
// domain code: same interval is NOT same phase, so settlements pair by the
// venues' PUBLISHED instants, rounded to a caller-chosen bucket to absorb
// Binance's millisecond jitter, never by an assumed shared grid origin.
// What the restatement ADDS over the screening is what the gate needs and
// the screening threw away: each leg's exact instant survives into the
// Match, because the phase ledger accrues at venue instants, not buckets.

import (
	"fmt"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Settlement is one venue-published settled funding print: the venue's own
// instant, exact as published, and the settled rate.
type Settlement struct {
	At   time.Time
	Rate fixedpoint.Rate
}

// Match is one paired settlement. At is the shared rounded bucket the pair
// matched on; BinanceAt and BybitAt keep each venue's exact instant (their
// difference is the pair's phase gap). Spread is oriented Binance minus
// Bybit — positive means Binance's funding printed higher, so the
// capturing position is short the Binance perp, long the Bybit perp.
type Match struct {
	At        time.Time
	BinanceAt time.Time
	BybitAt   time.Time
	Binance   fixedpoint.Rate
	Bybit     fixedpoint.Rate
	Spread    fixedpoint.Rate
}

// PairStats reports what pairing dropped, so coverage is a recorded fact
// rather than a silent property of the output length.
type PairStats struct {
	BinanceRows int
	BybitRows   int
	Matched     int

	// Collisions counts ROWS dropped because a venue printed twice within
	// one rounded bucket — venue dirt; every row in the bucket is dropped,
	// and each dropped row counts.
	BinanceCollisions int
	BybitCollisions   int
}

// Pair matches the two venues' series by rounded instant and returns the
// matched series ascending by bucket. Each input series must be ascending
// (equal instants are allowed through to the collision rule — same-bucket
// dirt is dropped and counted, not refused); a decreasing series is a
// walker bug and is refused.
func Pair(binance, bybit []Settlement, round time.Duration) ([]Match, PairStats, error) {
	if round <= 0 {
		return nil, PairStats{}, fmt.Errorf("round %s: %w", round, ErrRoundInvalid)
	}
	stats := PairStats{BinanceRows: len(binance), BybitRows: len(bybit)}

	bn, bnCollisions, err := bucket("binance", binance, round)
	if err != nil {
		return nil, PairStats{}, err
	}
	by, byCollisions, err := bucket("bybit", bybit, round)
	if err != nil {
		return nil, PairStats{}, err
	}
	stats.BinanceCollisions = bnCollisions
	stats.BybitCollisions = byCollisions

	var out []Match
	for ms, b := range bn {
		y, ok := by[ms]
		if !ok {
			continue
		}
		spread, err := b.Rate.Sub(y.Rate)
		if err != nil {
			return nil, PairStats{}, fmt.Errorf("spread at %d: %w", ms, err)
		}
		out = append(out, Match{
			At:        time.UnixMilli(ms).UTC(),
			BinanceAt: b.At,
			BybitAt:   y.At,
			Binance:   b.Rate,
			Bybit:     y.Rate,
			Spread:    spread,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	stats.Matched = len(out)
	return out, stats, nil
}

// bucket rounds each row's instant to the nearest multiple of round and
// indexes by it. A bucket hit twice is dropped entirely — for funding, two
// settlements inside one bucket is venue dirt, and keeping either row
// would be inventing which one "really" settled.
func bucket(leg string, rows []Settlement, round time.Duration) (map[int64]Settlement, int, error) {
	m := make(map[int64]Settlement, len(rows))
	dropped := make(map[int64]bool)
	collisions := 0
	for i, r := range rows {
		if err := r.Rate.Validate(); err != nil {
			return nil, 0, fmt.Errorf("%s row %d at %s: %w", leg, i, r.At.Format(time.RFC3339Nano), err)
		}
		if i > 0 && r.At.Before(rows[i-1].At) {
			return nil, 0, fmt.Errorf("%s row %d at %s after %s: %w",
				leg, i, r.At.Format(time.RFC3339Nano), rows[i-1].At.Format(time.RFC3339Nano), ErrSeriesDisordered)
		}
		ms := r.At.Round(round).UnixMilli()
		if dropped[ms] {
			collisions++
			continue
		}
		if _, dup := m[ms]; dup {
			delete(m, ms)
			dropped[ms] = true
			collisions += 2 // the row already kept and this one
			continue
		}
		m[ms] = r
	}
	return m, collisions, nil
}
