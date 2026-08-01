package crossvenue_test

// The pairing oracle: an independent O(buckets) re-derivation — integer
// millisecond arithmetic for the rounding (no time.Round), occupancy
// counting for the collision rule, big.Rat for the spreads — compared
// byte-for-byte against production. Bucket identity is by construction
// (jitter stays under half the round), so what the property exercises is
// the matching, collision and spread logic; the rounding BOUNDARY itself
// is pinned by the deterministic straddle fixture in pairing_test.go.

import (
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func bigRatDec(t interface{ Fatalf(string, ...any) }, s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("big.Rat SetString(%q)", s)
	}
	return r
}

// oracleRow is one generated settlement with its bucket index kept.
type oracleRow struct {
	bucket int
	at     time.Time
	rate   fixedpoint.Rate
}

func genLeg(t *rapid.T, label string, round time.Duration) []oracleRow {
	n := rapid.IntRange(0, 12).Draw(t, label+"N")
	rows := make([]oracleRow, n)
	for i := range rows {
		bucket := rapid.IntRange(0, 9).Draw(t, fmt.Sprintf("%sB%d", label, i))
		// Jitter strictly under round/2 keeps the bucket identity the
		// generator chose.
		jitterMs := rapid.Int64Range(-(round.Milliseconds()/2-1), round.Milliseconds()/2-1).
			Draw(t, fmt.Sprintf("%sJ%d", label, i))
		mant := rapid.Int64Range(-5_000_000, 5_000_000).Draw(t, fmt.Sprintf("%sR%d", label, i))
		rows[i] = oracleRow{
			bucket: bucket,
			at:     cvBase.Add(time.Duration(bucket)*4*time.Hour + time.Duration(jitterMs)*time.Millisecond),
			rate:   fixedpoint.RateFromMantissa(mant),
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].at.Before(rows[j].at) })
	return rows
}

func toSettlements(rows []oracleRow) []crossvenue.Settlement {
	out := make([]crossvenue.Settlement, len(rows))
	for i, r := range rows {
		out[i] = crossvenue.Settlement{At: r.at, Rate: r.rate}
	}
	return out
}

func TestProperty_PairMatchesOracle(t *testing.T) {
	const round = time.Minute
	rapid.Check(t, func(t *rapid.T) {
		bnRows := genLeg(t, "bn", round)
		byRows := genLeg(t, "by", round)

		matches, stats, err := crossvenue.Pair(toSettlements(bnRows), toSettlements(byRows), round)
		if err != nil {
			t.Fatalf("Pair: %v", err)
		}

		// Oracle occupancy per bucket, independent of production's map.
		bnByBucket := map[int][]oracleRow{}
		for _, r := range bnRows {
			bnByBucket[r.bucket] = append(bnByBucket[r.bucket], r)
		}
		byByBucket := map[int][]oracleRow{}
		for _, r := range byRows {
			byByBucket[r.bucket] = append(byByBucket[r.bucket], r)
		}
		bnCollisions, byCollisions := 0, 0
		for _, rs := range bnByBucket {
			if len(rs) >= 2 {
				bnCollisions += len(rs)
			}
		}
		for _, rs := range byByBucket {
			if len(rs) >= 2 {
				byCollisions += len(rs)
			}
		}
		if stats.BinanceCollisions != bnCollisions || stats.BybitCollisions != byCollisions {
			t.Fatalf("collisions %d/%d, oracle %d/%d",
				stats.BinanceCollisions, stats.BybitCollisions, bnCollisions, byCollisions)
		}
		if stats.BinanceRows != len(bnRows) || stats.BybitRows != len(byRows) {
			t.Fatalf("rows %d/%d, oracle %d/%d", stats.BinanceRows, stats.BybitRows, len(bnRows), len(byRows))
		}

		// Expected matches: buckets where BOTH legs have exactly one row.
		type expMatch struct{ bn, by oracleRow }
		expected := map[int]expMatch{}
		for b, bnRs := range bnByBucket {
			byRs, ok := byByBucket[b]
			if ok && len(bnRs) == 1 && len(byRs) == 1 {
				expected[b] = expMatch{bn: bnRs[0], by: byRs[0]}
			}
		}
		if len(matches) != len(expected) || stats.Matched != len(expected) {
			t.Fatalf("matched %d (stats %d), oracle %d", len(matches), stats.Matched, len(expected))
		}

		// The independent bucket instant: integer ms arithmetic, round half
		// away from zero — no time.Round anywhere in the check.
		roundMs := round.Milliseconds()
		for _, m := range matches {
			// Which oracle bucket does this match claim? Recompute from the
			// exact Binance instant the match carries.
			ms := m.BinanceAt.UnixMilli()
			bucketMs := (ms + roundMs/2) / roundMs * roundMs
			if got := m.At.UnixMilli(); got != bucketMs {
				t.Fatalf("bucket instant %d, oracle %d", got, bucketMs)
			}
			bucketIdx := int(time.UnixMilli(bucketMs).Sub(cvBase) / (4 * time.Hour))
			exp, ok := expected[bucketIdx]
			if !ok {
				t.Fatalf("production matched bucket %d the oracle did not", bucketIdx)
			}
			if !m.BinanceAt.Equal(exp.bn.at) || !m.BybitAt.Equal(exp.by.at) {
				t.Fatalf("bucket %d instants %s/%s, oracle %s/%s",
					bucketIdx, m.BinanceAt, m.BybitAt, exp.bn.at, exp.by.at)
			}
			spread := new(big.Rat).Sub(bigRatDec(t, exp.bn.rate.Decimal()), bigRatDec(t, exp.by.rate.Decimal()))
			if got := bigRatDec(t, m.Spread.Decimal()); got.Cmp(spread) != 0 {
				t.Fatalf("bucket %d spread %s, oracle %s", bucketIdx, m.Spread.Decimal(), spread.FloatString(9))
			}
			delete(expected, bucketIdx)
		}
		if len(expected) != 0 {
			t.Fatalf("oracle expected %d more matches", len(expected))
		}
	})
}
