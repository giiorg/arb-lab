package screening

// Pairing tests. The rounding fixture uses a 45-second offset — far
// beyond real jitter — because at 8 ms every plausible rounding rule
// agrees; 45 s is where round-to-NEAREST and truncate-to-minute give
// different buckets, so the pinned semantic is the one under test.

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

func row(tb testing.TB, at time.Time, rate string) Row {
	tb.Helper()
	r, err := fixedpoint.ParseRate(rate)
	if err != nil {
		tb.Fatalf("parse %q: %v", rate, err)
	}
	return Row{At: at, Rate: r}
}

// Millisecond jitter (the observed Binance shape, profiles §7) pairs
// with the other venue's exact grid instant, and the spread is oriented
// Binance minus Bybit.
func TestPairSettlementsAbsorbsJitterAndOrientsSpread(t *testing.T) {
	grid := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
	matches, stats, err := PairSettlements(
		[]Row{row(t, grid.Add(8*time.Millisecond), "0.0003")},
		[]Row{row(t, grid, "0.0001")},
	)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if len(matches) != 1 || stats.Matched != 1 {
		t.Fatalf("matched %d, want 1", len(matches))
	}
	if matches[0].Spread.Decimal() != "0.0002" {
		t.Fatalf("spread %s, want 0.0002 (binance minus bybit)", matches[0].Spread.Decimal())
	}
	if !matches[0].At.Equal(grid) {
		t.Fatalf("instant %s, want %s", matches[0].At, grid)
	}
}

// Rounding is to the NEAREST minute: an instant 45 s past the minute
// buckets forward, so it pairs with a row on the next minute and not
// with one on its own truncation.
func TestPairSettlementsRoundsToNearest(t *testing.T) {
	minute := time.Date(2025, 3, 1, 8, 1, 0, 0, time.UTC)
	matches, _, err := PairSettlements(
		[]Row{row(t, minute.Add(-15*time.Second), "0.0003")}, // 08:00:45
		[]Row{row(t, minute, "0.0001")},
	)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matched %d, want 1 (nearest-minute rounding)", len(matches))
	}
}

// Same interval is not same phase: two 4h grids offset by 2h never
// pair.
func TestPairSettlementsPhaseMismatchYieldsNothing(t *testing.T) {
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	var bn, by []Row
	for i := range 6 {
		bn = append(bn, row(t, base.Add(time.Duration(i)*4*time.Hour), "0.0001"))
		by = append(by, row(t, base.Add(2*time.Hour).Add(time.Duration(i)*4*time.Hour), "0.0001"))
	}
	matches, stats, err := PairSettlements(bn, by)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matched %d, want 0 (phase mismatch)", len(matches))
	}
	if stats.BinanceRows != 6 || stats.BybitRows != 6 {
		t.Fatalf("stats rows %d/%d, want 6/6", stats.BinanceRows, stats.BybitRows)
	}
}

// Two prints inside one rounded bucket are venue dirt: both rows drop,
// both count, and the instant does not match even though the other
// venue printed there.
func TestPairSettlementsDropsCollidingBucket(t *testing.T) {
	grid := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
	matches, stats, err := PairSettlements(
		[]Row{
			row(t, grid, "0.0003"),
			row(t, grid.Add(10*time.Second), "0.0009"),
			row(t, grid.Add(4*time.Hour), "0.0002"),
		},
		[]Row{
			row(t, grid, "0.0001"),
			row(t, grid.Add(4*time.Hour), "0.0001"),
		},
	)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if stats.BinanceCollisions != 2 {
		t.Fatalf("binance collisions %d, want 2 (both colliding rows)", stats.BinanceCollisions)
	}
	if len(matches) != 1 || !matches[0].At.Equal(grid.Add(4*time.Hour)) {
		t.Fatalf("matches %v, want only the clean instant", matches)
	}
}

// Output is ascending by instant regardless of input order.
func TestPairSettlementsSortsAscending(t *testing.T) {
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	bn := []Row{
		row(t, base.Add(8*time.Hour), "0.0001"),
		row(t, base, "0.0001"),
		row(t, base.Add(4*time.Hour), "0.0001"),
	}
	by := []Row{
		row(t, base.Add(4*time.Hour), "0.0001"),
		row(t, base.Add(8*time.Hour), "0.0001"),
		row(t, base, "0.0001"),
	}
	matches, _, err := PairSettlements(bn, by)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("matched %d, want 3", len(matches))
	}
	for i := 1; i < len(matches); i++ {
		if !matches[i-1].At.Before(matches[i].At) {
			t.Fatalf("not ascending at %d: %s then %s", i, matches[i-1].At, matches[i].At)
		}
	}
}
