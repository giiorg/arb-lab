package calibrate_test

// The commit-2 oracles: the excursion band and the adverse basis move
// recomputed in exact big.Rat arithmetic — runs, depths, ranks and the
// annualization all as rationals, no scale-9 rounding anywhere — with the
// tolerance carrying only the production path's stated roundings.

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"pgregory.net/rapid"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

// bandTolerance bounds the production path's two half-even roundings in
// Annualized (the per-year ratio's ulp scaled by a depth bounded at 0.015,
// plus the final product's ulp): well under 2e-9.
const bandTolerance = "0.000000002"

// ratWithin reports how got differs from want beyond tolerance, or "".
func ratWithin(t interface{ Fatalf(string, ...any) }, got string, want *big.Rat, tolerance string) string {
	diff := new(big.Rat).Sub(bigRatOf(t, got), want)
	diff.Abs(diff)
	if diff.Cmp(bigRatOf(t, tolerance)) > 0 {
		return fmt.Sprintf("fp %s vs oracle %s: |diff| %s exceeds %s",
			got, want.FloatString(15), diff.FloatString(15), tolerance)
	}
	return ""
}

// ceilRank is the oracle's upper order statistic index: ceil(q·(n−1)) in
// exact integers.
func ceilRank(t interface{ Fatalf(string, ...any) }, q string, n int) int {
	rank := new(big.Rat).Mul(bigRatOf(t, q), big.NewRat(int64(n-1), 1))
	idx := new(big.Int).Quo(rank.Num(), rank.Denom()).Int64()
	if !rank.IsInt() {
		idx++
	}
	return int(idx)
}

func TestProperty_ExitBandWidthMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(4, 40).Draw(t, "n")
		intervalH := rapid.Int64Range(1, 24).Draw(t, "intervalHours")
		interval := time.Duration(intervalH) * time.Hour
		persist := uint32(rapid.IntRange(1, 5).Draw(t, "persist"))
		clearing := rapidRate(t, rapid.Int64Range(-2_000_000, 2_000_000).Draw(t, "clearing"))
		q := rapidRate(t, rapid.Int64Range(1, 999_999_999).Draw(t, "q"))

		history := make([]domain.RealizedSettlement, n)
		for i := range history {
			at := calEra.Add(time.Duration(i) * interval)
			history[i] = domain.RealizedSettlement{
				Instrument: calPerp,
				At:         at,
				Rate:       rapidRate(t, rapid.Int64Range(-7_500_000, 7_500_000).Draw(t, fmt.Sprintf("r%d", i))),
				Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at},
			}
		}

		got, err := calibrate.ExitBandWidth(history, clearing, persist, q, interval)
		if errors.Is(err, calibrate.ErrNoExcursions) {
			return // an era that measured nothing is honestly refused
		}
		if err != nil {
			t.Fatalf("ExitBandWidth: %v", err)
		}

		// Oracle: exact rational runs and depths.
		cl := bigRatOf(t, clearing.Decimal())
		var depths []*big.Rat
		runLen := uint32(0)
		var runMin *big.Rat
		for _, s := range history {
			r := bigRatOf(t, s.Rate.Decimal())
			if r.Cmp(cl) < 0 {
				if runLen == 0 || r.Cmp(runMin) < 0 {
					runMin = r
				}
				runLen++
				continue
			}
			if runLen > 0 && runLen < persist {
				depths = append(depths, new(big.Rat).Sub(cl, runMin))
			}
			runLen = 0
		}
		if len(depths) == 0 {
			t.Fatalf("production found a width %s where the oracle finds no recovered dip", got.Decimal())
		}
		sort.Slice(depths, func(i, j int) bool { return depths[i].Cmp(depths[j]) < 0 })
		depth := depths[ceilRank(t, q.Decimal(), len(depths))]
		want := new(big.Rat).Mul(depth, big.NewRat(365*24, intervalH))

		if diff := ratWithin(t, got.Decimal(), want, bandTolerance); diff != "" {
			t.Fatalf("band width disagrees: %s", diff)
		}
		// Inline control: three tolerances off must be caught.
		off := new(big.Rat).Add(want, bigRatOf(t, "0.000000006"))
		if diff := ratWithin(t, off.FloatString(12), want, bandTolerance); diff == "" {
			t.Fatal("the oracle accepted a width three tolerances off — the check is vacuous")
		}
	})
}

// basisTolerance bounds one half-even division per move (5e-10) plus a
// possible order swap between two moves closer than that (another ulp):
// 2e-9 with margin.
const basisTolerance = "0.000000002"

func TestProperty_AdverseBasisMoveMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(3, 30).Draw(t, "n")
		s := rapid.IntRange(1, 5).Draw(t, "steps")
		if s >= n {
			s = n - 1
		}
		q := rapidRate(t, rapid.Int64Range(1, 999_999_999).Draw(t, "q"))

		spot := make([]calibrate.KlinePoint, n)
		mark := make([]calibrate.KlinePoint, n)
		for i := range spot {
			at := calEra.Add(time.Duration(i) * time.Hour)
			sp := priceOf(t, rapid.Int64Range(50_000_000_000, 150_000_000_000).Draw(t, fmt.Sprintf("s%d", i)))
			mk := priceOf(t, rapid.Int64Range(50_000_000_000, 150_000_000_000).Draw(t, fmt.Sprintf("m%d", i)))
			spot[i] = calibrate.KlinePoint{At: at, Price: sp}
			mark[i] = calibrate.KlinePoint{At: at, Price: mk}
		}

		got, err := calibrate.AdverseBasisMove(spot, mark, time.Duration(s)*time.Hour, q)
		if err != nil {
			t.Fatalf("AdverseBasisMove: %v", err)
		}

		// Oracle: exact rational moves.
		moves := make([]*big.Rat, 0, n-s)
		for i := 0; i+s < n; i++ {
			b0 := new(big.Rat).Sub(bigRatOf(t, mark[i].Price.Decimal()), bigRatOf(t, spot[i].Price.Decimal()))
			b1 := new(big.Rat).Sub(bigRatOf(t, mark[i+s].Price.Decimal()), bigRatOf(t, spot[i+s].Price.Decimal()))
			move := new(big.Rat).Sub(b1, b0)
			move.Quo(move, bigRatOf(t, spot[i].Price.Decimal()))
			moves = append(moves, move)
		}
		sort.Slice(moves, func(i, j int) bool { return moves[i].Cmp(moves[j]) < 0 })
		want := moves[ceilRank(t, q.Decimal(), len(moves))]
		if want.Sign() < 0 {
			want = new(big.Rat)
		}

		if diff := ratWithin(t, got.Decimal(), want, basisTolerance); diff != "" {
			t.Fatalf("adverse basis move disagrees: %s", diff)
		}
		off := new(big.Rat).Add(want, bigRatOf(t, "0.000000006"))
		if diff := ratWithin(t, off.FloatString(12), want, basisTolerance); diff == "" {
			t.Fatal("the oracle accepted a move three tolerances off — the check is vacuous")
		}
	})
}

// priceOf renders a scale-9 mantissa as a BTC/USDT price via its exact
// decimal text.
func priceOf(t *rapid.T, mantissa int64) fixedpoint.Price {
	s := new(big.Rat).SetFrac64(mantissa, 1_000_000_000).FloatString(9)
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}
