package crossvenue_test

// The ledger oracle: brute-force big.Rat recomputation with a different
// loop structure — for every event instant it re-sums BOTH legs' prefixes
// from scratch (no running state, no merge), derives the unhedged term
// from the definition (net minus net-at-last-balanced-instant), and
// compares every ledger field byte-for-byte. All ledger arithmetic is
// exact at scale 9 (additions only), so agreement is equality, not a
// tolerance.

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

func genSettlements(t *rapid.T, label string) []crossvenue.Settlement {
	n := rapid.IntRange(0, 10).Draw(t, label+"N")
	offsets := rapid.SliceOfNDistinct(rapid.Int64Range(0, 96), n, n, rapid.ID).Draw(t, label+"Off")
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
	rows := make([]crossvenue.Settlement, n)
	for i, off := range offsets {
		// Half-hour steps put prints on both sides of every window edge
		// and interleave the legs' instants (shared instants included,
		// since both legs draw from the same offset lattice).
		rows[i] = crossvenue.Settlement{
			At:   cvBase.Add(time.Duration(off) * 30 * time.Minute),
			Rate: fixedpoint.RateFromMantissa(rapid.Int64Range(-5_000_000, 5_000_000).Draw(t, fmt.Sprintf("%sR%d", label, i))),
		}
	}
	return rows
}

func TestProperty_WindowAccrualMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		openOff := rapid.Int64Range(0, 40).Draw(t, "open")
		closeOff := rapid.Int64Range(openOff, 96).Draw(t, "close")
		sign := rapid.SampledFrom([]int{1, -1}).Draw(t, "sign")
		w := crossvenue.Window{
			OpenAt:  cvBase.Add(time.Duration(openOff) * 30 * time.Minute),
			CloseAt: cvBase.Add(time.Duration(closeOff) * 30 * time.Minute),
			Sign:    sign,
		}
		bn := genSettlements(t, "bn")
		by := genSettlements(t, "by")

		led, err := crossvenue.WindowAccrual(w, bn, by)
		if err != nil {
			t.Fatalf("WindowAccrual: %v", err)
		}

		// Oracle: filter (open, close] from the definition.
		inWindow := func(rows []crossvenue.Settlement) []crossvenue.Settlement {
			var out []crossvenue.Settlement
			for _, r := range rows {
				if r.At.After(w.OpenAt) && !r.At.After(w.CloseAt) {
					out = append(out, r)
				}
			}
			return out
		}
		bnIn, byIn := inWindow(bn), inWindow(by)
		if led.BinancePrints != len(bnIn) || led.BybitPrints != len(byIn) {
			t.Fatalf("prints %d/%d, oracle %d/%d", led.BinancePrints, led.BybitPrints, len(bnIn), len(byIn))
		}
		if led.EndImbalance != len(bnIn)-len(byIn) {
			t.Fatalf("end imbalance %d, oracle %d", led.EndImbalance, len(bnIn)-len(byIn))
		}

		sumRats := func(rows []crossvenue.Settlement) *big.Rat {
			s := new(big.Rat)
			for _, r := range rows {
				s.Add(s, bigRatDec(t, r.Rate.Decimal()))
			}
			return s
		}
		bnSum, bySum := sumRats(bnIn), sumRats(byIn)
		if got := bigRatDec(t, led.BinanceAccrued.Decimal()); got.Cmp(bnSum) != 0 {
			t.Fatalf("binance accrued %s, oracle %s", led.BinanceAccrued.Decimal(), bnSum.FloatString(9))
		}
		if got := bigRatDec(t, led.BybitAccrued.Decimal()); got.Cmp(bySum) != 0 {
			t.Fatalf("bybit accrued %s, oracle %s", led.BybitAccrued.Decimal(), bySum.FloatString(9))
		}

		// netAt re-derives the signed net at instant e from scratch.
		signRat := big.NewRat(int64(sign), 1)
		netAt := func(e time.Time) *big.Rat {
			n := new(big.Rat)
			for _, r := range bnIn {
				if !r.At.After(e) {
					n.Add(n, bigRatDec(t, r.Rate.Decimal()))
				}
			}
			for _, r := range byIn {
				if !r.At.After(e) {
					n.Sub(n, bigRatDec(t, r.Rate.Decimal()))
				}
			}
			return n.Mul(n, signRat)
		}
		countsAt := func(e time.Time) (int, int) {
			cb, cy := 0, 0
			for _, r := range bnIn {
				if !r.At.After(e) {
					cb++
				}
			}
			for _, r := range byIn {
				if !r.At.After(e) {
					cy++
				}
			}
			return cb, cy
		}

		// Event instants: the union of both legs' in-window instants.
		eventSet := map[int64]time.Time{}
		for _, r := range append(append([]crossvenue.Settlement{}, bnIn...), byIn...) {
			eventSet[r.At.UnixNano()] = r.At
		}
		var events []time.Time
		for _, e := range eventSet {
			events = append(events, e)
		}
		sort.Slice(events, func(i, j int) bool { return events[i].Before(events[j]) })

		anchor := new(big.Rat)
		phaseMax, phaseMin := new(big.Rat), new(big.Rat)
		var finalNet *big.Rat = new(big.Rat)
		for _, e := range events {
			net := netAt(e)
			finalNet = net
			cb, cy := countsAt(e)
			if cb == cy {
				anchor = net
				continue
			}
			u := new(big.Rat).Sub(net, anchor)
			if u.Cmp(phaseMax) > 0 {
				phaseMax = u
			}
			if u.Cmp(phaseMin) < 0 {
				phaseMin = u
			}
		}

		if got := bigRatDec(t, led.Net.Decimal()); got.Cmp(finalNet) != 0 {
			t.Fatalf("net %s, oracle %s", led.Net.Decimal(), finalNet.FloatString(9))
		}
		if got := bigRatDec(t, led.PhaseMax.Decimal()); got.Cmp(phaseMax) != 0 {
			t.Fatalf("phase max %s, oracle %s", led.PhaseMax.Decimal(), phaseMax.FloatString(9))
		}
		if got := bigRatDec(t, led.PhaseMin.Decimal()); got.Cmp(phaseMin) != 0 {
			t.Fatalf("phase min %s, oracle %s", led.PhaseMin.Decimal(), phaseMin.FloatString(9))
		}
	})
}
