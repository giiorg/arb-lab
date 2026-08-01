package calibrate_test

// The slice-C oracles: the counterfactual slippage walk and the adverse
// fill move recomputed in exact big.Rat arithmetic — level fills, the
// reference notional and the ratio all as rationals — and the two-family
// merge recomputed as an exact discrete rule (no rounding exists in it, so
// the oracle demands equality, not tolerance). Each with the inline
// broken-implementation control the standing rule requires.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"pgregory.net/rapid"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

// slipTolerance bounds the production path's roundings: up to six per-level
// half-even Notionals (≤0.5 ulp each at scale 9) and the reference's one,
// scaled down by a reference notional the generator keeps at or above 1,
// plus the final division's ulp — comfortably under 1e-8.
const slipTolerance = "0.00000001"

// oracleBook builds a validated snapshot from mantissa ladders. Asks
// ascend from mid, bids descend below it — uncrossed by construction.
func oracleBook(t *rapid.T, at time.Time, seq uint64, mid int64, bidQ, askQ []int64) mdomain.BookSnapshot {
	mkLevel := func(price int64, q int64) mdomain.BookLevel {
		ps := new(big.Rat).SetFrac64(price, 1_000_000_000).FloatString(9)
		p, err := fixedpoint.ParsePrice("BTC", "USDT", ps)
		if err != nil {
			t.Fatalf("price %q: %v", ps, err)
		}
		qs := new(big.Rat).SetFrac64(q, 1_000_000_000).FloatString(9)
		qq, err := fixedpoint.ParseQuantity("BTC", qs)
		if err != nil {
			t.Fatalf("qty %q: %v", qs, err)
		}
		return mdomain.BookLevel{Price: p, Quantity: qq}
	}
	const tick = int64(100_000_000) // 0.1 quote per level step
	var bids, asks []mdomain.BookLevel
	for i, q := range bidQ {
		bids = append(bids, mkLevel(mid-int64(i+1)*tick, q))
	}
	for i, q := range askQ {
		asks = append(asks, mkLevel(mid+int64(i+1)*tick, q))
	}
	b := mdomain.BookSnapshot{
		Instrument: calSpot,
		Bids:       bids,
		Asks:       asks,
		Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at, VenueSeq: seq},
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("oracle book: %v", err)
	}
	return b
}

func TestProperty_CounterfactualSlippageMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		side := mdomain.SideAsk
		if rapid.Bool().Draw(t, "bid") {
			side = mdomain.SideBid
		}
		nLevels := rapid.IntRange(1, 6).Draw(t, "levels")
		qtys := func(label string) []int64 {
			out := make([]int64, nLevels)
			for i := range out {
				out[i] = rapid.Int64Range(100_000_000, 2_000_000_000).Draw(t, fmt.Sprintf("%s%d", label, i))
			}
			return out
		}
		midAt := rapid.Int64Range(50_000_000_000_000, 150_000_000_000_000).Draw(t, "midAt")
		midLater := midAt + rapid.Int64Range(-500_000_000, 500_000_000).Draw(t, "drift")
		atQ, laterQ := qtys("a"), qtys("l")

		at := oracleBook(t, calEra, 100, midAt, atQ, atQ)
		later := oracleBook(t, calEra.Add(250*time.Millisecond), 101, midLater, laterQ, laterQ)

		// Candidate between 0.1 and the later side's total, so the walk
		// both spans levels and can always fill.
		var total int64
		for _, q := range laterQ {
			total += q
		}
		cand := rapid.Int64Range(100_000_000, total).Draw(t, "cand")
		cs := new(big.Rat).SetFrac64(cand, 1_000_000_000).FloatString(9)
		candQty, err := fixedpoint.ParseQuantity("BTC", cs)
		if err != nil {
			t.Fatalf("candidate: %v", err)
		}

		got, err := calibrate.CounterfactualSlippage(at, later, side, candQty)
		if err != nil {
			t.Fatalf("CounterfactualSlippage: %v", err)
		}

		// Oracle: exact rational walk over the later book, reference at
		// the at book's best.
		levelsOf := func(b mdomain.BookSnapshot) []mdomain.BookLevel {
			if side == mdomain.SideBid {
				return b.Bids
			}
			return b.Asks
		}
		best := bigRatOf(t, levelsOf(at)[0].Price.Decimal())
		ref := new(big.Rat).Mul(best, bigRatOf(t, candQty.Decimal()))
		remaining := bigRatOf(t, candQty.Decimal())
		filled := new(big.Rat)
		for _, l := range levelsOf(later) {
			if remaining.Sign() == 0 {
				break
			}
			take := bigRatOf(t, l.Quantity.Decimal())
			if take.Cmp(remaining) > 0 {
				take = new(big.Rat).Set(remaining)
			}
			filled.Add(filled, new(big.Rat).Mul(bigRatOf(t, l.Price.Decimal()), take))
			remaining.Sub(remaining, take)
		}
		want := new(big.Rat)
		if side == mdomain.SideAsk {
			want.Sub(filled, ref)
		} else {
			want.Sub(ref, filled)
		}
		want.Quo(want, ref)

		if diff := ratWithin(t, got.Decimal(), want, slipTolerance); diff != "" {
			t.Fatalf("counterfactual slippage disagrees: %s", diff)
		}
		// Inline control: three tolerances off must be caught.
		off := new(big.Rat).Add(want, bigRatOf(t, "0.00000003"))
		if diff := ratWithin(t, off.FloatString(12), want, slipTolerance); diff == "" {
			t.Fatal("the oracle accepted a slippage three tolerances off — the check is vacuous")
		}
	})
}

// moveTolerance bounds two half-even Notionals and one division.
const moveTolerance = "0.000000002"

func TestProperty_AdverseFillMoveMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		side := mdomain.SideAsk
		if rapid.Bool().Draw(t, "bid") {
			side = mdomain.SideBid
		}
		midAt := rapid.Int64Range(50_000_000_000_000, 150_000_000_000_000).Draw(t, "midAt")
		midLater := midAt + rapid.Int64Range(-900_000_000, 900_000_000).Draw(t, "drift")
		at := oracleBook(t, calEra, 200, midAt, []int64{1_000_000_000}, []int64{1_000_000_000})
		later := oracleBook(t, calEra.Add(250*time.Millisecond), 201, midLater, []int64{1_000_000_000}, []int64{1_000_000_000})

		got, err := calibrate.AdverseFillMove(at, later, side)
		if err != nil {
			t.Fatalf("AdverseFillMove: %v", err)
		}

		pick := func(b mdomain.BookSnapshot) *big.Rat {
			if side == mdomain.SideBid {
				return bigRatOf(t, b.Bids[0].Price.Decimal())
			}
			return bigRatOf(t, b.Asks[0].Price.Decimal())
		}
		bAt, bLater := pick(at), pick(later)
		want := new(big.Rat)
		if side == mdomain.SideAsk {
			want.Sub(bLater, bAt)
		} else {
			want.Sub(bAt, bLater)
		}
		want.Quo(want, bAt)

		if diff := ratWithin(t, got.Decimal(), want, moveTolerance); diff != "" {
			t.Fatalf("adverse fill move disagrees: %s", diff)
		}
		off := new(big.Rat).Add(want, bigRatOf(t, "0.000000006"))
		if diff := ratWithin(t, off.FloatString(12), want, moveTolerance); diff == "" {
			t.Fatal("the oracle accepted a move three tolerances off — the check is vacuous")
		}
	})
}

// The merge is a discrete rule with no arithmetic to round, so its oracle
// is EXACT: an independent big.Rat implementation of "outward from the
// median" must agree on every point, mantissa for mantissa.
func TestProperty_MergeAdverseMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 9).Draw(t, "n")
		// A strictly ascending grid inside (0,1).
		grid := make([]fixedpoint.Rate, n)
		step := int64(999_999_999) / int64(n+1)
		for i := range grid {
			jitter := rapid.Int64Range(1, step/2).Draw(t, fmt.Sprintf("g%d", i))
			grid[i] = rapidRate(t, int64(i+1)*step-step/2+jitter)
		}
		mkShifts := func(label string) []fixedpoint.Rate {
			out := make([]fixedpoint.Rate, n)
			var last int64 = -3_000_000
			for i := range out {
				last += rapid.Int64Range(0, 1_000_000).Draw(t, fmt.Sprintf("%s%d", label, i))
				out[i] = rapidRate(t, last)
			}
			return out
		}
		aS, bS := mkShifts("a"), mkShifts("b")
		a := make([]domain.QuantilePoint, n)
		b := make([]domain.QuantilePoint, n)
		for i := range a {
			a[i] = domain.QuantilePoint{Q: grid[i], Shift: aS[i]}
			b[i] = domain.QuantilePoint{Q: grid[i], Shift: bS[i]}
		}

		got, err := calibrate.MergeAdverse(a, b)
		if err != nil {
			t.Fatalf("MergeAdverse: %v", err)
		}

		half := bigRatOf(t, "0.5")
		for i := range got {
			q := bigRatOf(t, grid[i].Decimal())
			sa, sb := bigRatOf(t, aS[i].Decimal()), bigRatOf(t, bS[i].Decimal())
			want := sa
			if q.Cmp(half) < 0 {
				if sb.Cmp(sa) < 0 {
					want = sb
				}
			} else if sb.Cmp(sa) > 0 {
				want = sb
			}
			if bigRatOf(t, got[i].Shift.Decimal()).Cmp(want) != 0 {
				t.Fatalf("point %d: merged %s, oracle %s", i, got[i].Shift, want.FloatString(9))
			}
		}
		// Inline control: the anti-rule (inward pick) must disagree with
		// the oracle somewhere whenever the two inputs differ at a point.
		for i := range got {
			q := bigRatOf(t, grid[i].Decimal())
			sa, sb := bigRatOf(t, aS[i].Decimal()), bigRatOf(t, bS[i].Decimal())
			if sa.Cmp(sb) == 0 {
				continue
			}
			inward := sa
			if q.Cmp(half) < 0 {
				if sb.Cmp(sa) > 0 {
					inward = sb
				}
			} else if sb.Cmp(sa) < 0 {
				inward = sb
			}
			if bigRatOf(t, got[i].Shift.Decimal()).Cmp(inward) == 0 {
				t.Fatalf("point %d: the outward pick equals the inward pick on differing inputs — the control is vacuous", i)
			}
			break
		}
	})
}

// The depth-short direction agrees with the oracle too: when the later
// side's total is below the candidate, production must refuse — never
// return a number the walk could not have measured.
func TestCounterfactualSlippageDepthShortAgreesWithOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		total := rapid.Int64Range(200_000_000, 1_000_000_000).Draw(t, "total")
		at := oracleBookT(t, calEra, 300, 100_000_000_000_000, []int64{total + 500_000_000})
		later := oracleBookT(t, calEra.Add(250*time.Millisecond), 301, 100_000_000_000_000, []int64{total})
		cs := new(big.Rat).SetFrac64(total+100_000_000, 1_000_000_000).FloatString(9)
		cand, err := fixedpoint.ParseQuantity("BTC", cs)
		if err != nil {
			t.Fatalf("candidate: %v", err)
		}
		if _, err := calibrate.CounterfactualSlippage(at, later, mdomain.SideAsk, cand); !errors.Is(err, calibrate.ErrDepthShort) {
			t.Fatalf("candidate past the later side's depth: %v, want ErrDepthShort", err)
		}
	})
}

// oracleBookT builds a symmetric one-arg book (same ladder both sides).
func oracleBookT(t *rapid.T, at time.Time, seq uint64, mid int64, q []int64) mdomain.BookSnapshot {
	return oracleBook(t, at, seq, mid, q, q)
}
