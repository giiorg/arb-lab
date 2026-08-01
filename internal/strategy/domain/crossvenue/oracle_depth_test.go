package crossvenue_test

// The WalkedFill oracle: an independent big.Rat walk. Real defects in this
// repo have been found by two implementations disagreeing, not by review —
// the oracle re-derives every generated case from the decimal strings.
//
// Rounding parity is handled by CONSTRUCTION, not by tolerance: prices and
// quantities are generated at ≤4 decimal places, so every per-level
// notional is exact at scale 9 and the ONLY rounding in the production
// walk is the final half-even division — which the oracle performs on
// exact rationals with an explicit half-even tie rule. The generator's
// stream is a pinned in-package SplitMix64: a rerun IS a replay.

import (
	"fmt"
	"math/big"
	"testing"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// The generator stream is oracle_score_test.go's independently transcribed
// SplitMix64, and the final rounding is that file's roundHalfEven9 — one
// test-side transcription of each, seeded per oracle.

// oracleWalk re-derives the walked slippage from the fixture's decimal
// strings: exact rational fill sum, exact reference, one half-even
// division. Returns ok=false where production must refuse depth.
func oracleWalk(t *testing.T, levels [][2]string, qtyStr string, buy bool) (string, bool) {
	t.Helper()
	q, _ := new(big.Rat).SetString(qtyStr)
	if len(levels) == 0 {
		return "", false
	}
	best, _ := new(big.Rat).SetString(levels[0][0])
	ref := new(big.Rat).Mul(best, q)

	remaining := new(big.Rat).Set(q)
	filled := new(big.Rat)
	for _, l := range levels {
		if remaining.Sign() == 0 {
			break
		}
		price, _ := new(big.Rat).SetString(l[0])
		size, _ := new(big.Rat).SetString(l[1])
		take := size
		if remaining.Cmp(size) < 0 {
			take = remaining
		}
		filled.Add(filled, new(big.Rat).Mul(price, take))
		remaining = new(big.Rat).Sub(remaining, take)
	}
	if remaining.Sign() != 0 {
		return "", false
	}
	adverse := new(big.Rat)
	if buy {
		adverse.Sub(filled, ref)
	} else {
		adverse.Sub(ref, filled)
	}
	slip := new(big.Rat).Quo(adverse, ref)
	return roundHalfEven9(slip), true
}

// TestWalkedFillOracle drives generated books through both walks. The
// generator varies level counts, price steps and the candidate's position
// relative to total depth, so exact drains, mid-level fills and shorts
// all occur (the stream was inspected once to confirm all three arise).
func TestWalkedFillOracle(t *testing.T) {
	g := &oracleSplitmix{state: 0x45C0DE5EED}
	shorts, fills := 0, 0
	for trial := 0; trial < 400; trial++ {
		nLevels := int(g.draw()%4) + 1
		side := make([][2]string, nLevels)
		// Prices ascend from a random base in 0.01 steps (an ask side);
		// the bid walk reuses the same numbers with the sell orientation.
		base := int64(g.draw()%9000) + 1000 // 10.00 .. 99.99
		for i := range side {
			price := fmt.Sprintf("%d.%02d", base/100, base%100)
			base += int64(g.draw()%50) + 1
			sizeCents := int64(g.draw()%400) + 1 // 0.01 .. 4.00
			size := fmt.Sprintf("%d.%02d", sizeCents/100, sizeCents%100)
			side[i] = [2]string{price, size}
		}
		qtyCents := int64(g.draw()%600) + 1
		qtyStr := fmt.Sprintf("%d.%02d", qtyCents/100, qtyCents%100)

		for _, buy := range []bool{true, false} {
			levels := side
			mside := mdomain.SideAsk
			var bids, asks [][2]string
			if buy {
				asks = levels
			} else {
				// A bid side must DESCEND: reverse the ascending fixture.
				bids = make([][2]string, nLevels)
				for i := range levels {
					bids[i] = levels[nLevels-1-i]
				}
				levels = bids
				mside = mdomain.SideBid
			}
			book := depthBook(t, bids, asks)

			want, ok := oracleWalk(t, levels, qtyStr, buy)
			got, err := crossvenue.WalkedFill(book, mside, depthQty(t, qtyStr))
			if !ok {
				shorts++
				if err == nil {
					t.Fatalf("trial %d buy=%v: oracle short, production filled %s", trial, buy, got.Decimal())
				}
				continue
			}
			fills++
			if err != nil {
				t.Fatalf("trial %d buy=%v: production refused a fillable walk: %v", trial, buy, err)
			}
			if got.Decimal() != trimRate(want) {
				t.Fatalf("trial %d buy=%v: production %s, oracle %s", trial, buy, got.Decimal(), want)
			}
		}
	}
	if shorts == 0 || fills == 0 {
		t.Fatalf("degenerate stream: %d shorts, %d fills — the generator no longer exercises both arms", shorts, fills)
	}
}

// trimRate normalizes the oracle's fixed 9-place rendering to fixedpoint's
// trimmed Decimal form.
func trimRate(s string) string {
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		return "unparseable:" + s
	}
	return r.Decimal()
}
