package screening

// Settlement pairing (§42b step 2): same interval is NOT same phase, so
// settlements pair by the venues' PUBLISHED instants — rounded to
// PairingRound to absorb Binance's millisecond jitter — never by an
// assumed shared grid origin (the recomputed-check-must-share-the-anchor
// lesson, applied up front).

import (
	"fmt"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Match is one paired settlement: the shared rounded instant and the
// spread, oriented Binance minus Bybit — positive means Binance's
// funding printed higher (the capturing position is short the Binance
// perp, long the Bybit perp).
type Match struct {
	At     time.Time
	Spread fixedpoint.Rate
}

// PairStats reports what pairing dropped, so coverage is a recorded
// fact rather than a silent property of the output length.
type PairStats struct {
	BinanceRows int `json:"binance_rows"`
	BybitRows   int `json:"bybit_rows"`
	Matched     int `json:"matched"`

	// Collisions counts ROWS dropped because a venue printed twice
	// within one rounded bucket — dirty data; every row in the bucket
	// is dropped, and each dropped row counts.
	BinanceCollisions int `json:"binance_collisions"`
	BybitCollisions   int `json:"bybit_collisions"`
}

// PairSettlements matches the two venues' series by rounded instant and
// returns the spread series ascending.
func PairSettlements(binance, bybit []Row) ([]Match, PairStats, error) {
	stats := PairStats{BinanceRows: len(binance), BybitRows: len(bybit)}

	bnByInstant, bnCollisions := bucket(binance)
	byByInstant, byCollisions := bucket(bybit)
	stats.BinanceCollisions = bnCollisions
	stats.BybitCollisions = byCollisions

	var out []Match
	for ms, bnRate := range bnByInstant {
		byRate, ok := byByInstant[ms]
		if !ok {
			continue
		}
		spread, err := bnRate.Sub(byRate)
		if err != nil {
			return nil, PairStats{}, fmt.Errorf("spread at %d: %w", ms, err)
		}
		out = append(out, Match{At: time.UnixMilli(ms).UTC(), Spread: spread})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	stats.Matched = len(out)
	return out, stats, nil
}

// bucket rounds each row's instant to the nearest PairingRound and
// indexes by it. A bucket hit twice is dropped entirely — for funding,
// two settlements inside one minute is venue dirt, and keeping either
// row would be inventing which one "really" settled.
func bucket(rows []Row) (map[int64]fixedpoint.Rate, int) {
	m := make(map[int64]fixedpoint.Rate, len(rows))
	dropped := make(map[int64]bool)
	collisions := 0
	for _, r := range rows {
		ms := r.At.Round(PairingRound).UnixMilli()
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
		m[ms] = r.Rate
	}
	return m, collisions
}
