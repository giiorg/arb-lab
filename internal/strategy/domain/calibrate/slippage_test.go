package calibrate

import (
	"errors"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Fixture discipline (§41i's named suspects, applied before the battery):
// the counterfactual fixtures' books CHANGE between t and t+latency — an
// unchanged pair collapses walk-at-t+λ into the plain depth walk; the
// merge fixtures make each input win on each side of the median AND
// exactly at it; the quantile fixtures hold both an exact rank and an
// inexact one, because ceil and identity agree at exact ranks.

var slipInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}

func slipBook(t *testing.T, at time.Time, seq uint64, bids, asks [][2]string) mdomain.BookSnapshot {
	t.Helper()
	mk := func(pairs [][2]string) []mdomain.BookLevel {
		levels := make([]mdomain.BookLevel, 0, len(pairs))
		for _, p := range pairs {
			price, err := fixedpoint.ParsePrice("BTC", "USDT", p[0])
			if err != nil {
				t.Fatalf("price %q: %v", p[0], err)
			}
			qty, err := fixedpoint.ParseQuantity("BTC", p[1])
			if err != nil {
				t.Fatalf("qty %q: %v", p[1], err)
			}
			levels = append(levels, mdomain.BookLevel{Price: price, Quantity: qty})
		}
		return levels
	}
	b := mdomain.BookSnapshot{
		Instrument: slipInst,
		Bids:       mk(bids),
		Asks:       mk(asks),
		Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at, VenueSeq: seq},
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("fixture book: %v", err)
	}
	return b
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("qty %q: %v", s, err)
	}
	return q
}

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("rate %q: %v", s, err)
	}
	return r
}

var slipT0 = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

// The ask-side counterfactual: best at t is 100.00; over the window the
// best level VANISHES, so the walk at t+λ starts at 100.10 and spills to
// 100.20. Hand-derived: fill 0.5@100.10 + 0.5@100.20 = 100.15; ref =
// 100.00; slippage = 0.15/100 = 0.0015. Against the collapse: walking the
// book AT T instead gives 0.5@100.00+0.5@100.10 = 0.0005 — a third of the
// answer — so a mutant that walks the wrong instant cannot pass.
func TestCounterfactualSlippageAskVanishingDepth(t *testing.T) {
	at := slipBook(t, slipT0, 10,
		[][2]string{{"99.90", "1.0"}},
		[][2]string{{"100.00", "0.5"}, {"100.10", "0.5"}, {"100.20", "2.0"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 11,
		[][2]string{{"99.90", "1.0"}},
		[][2]string{{"100.10", "0.5"}, {"100.20", "2.0"}})

	got, err := CounterfactualSlippage(at, later, mdomain.SideAsk, qty(t, "1.0"))
	if err != nil {
		t.Fatalf("slippage: %v", err)
	}
	if want := rate(t, "0.0015"); got != want {
		t.Fatalf("slippage = %s, want %s", got, want)
	}
}

// The bid-side mirror, adverse-positive: selling 1.0 into a book whose
// best bid dropped. Ref at t = 100.00; fill at t+λ = 0.4@99.90 + 0.6@99.80
// = 99.84; slippage = (100.00 − 99.84)/100.00 = 0.0016. The two sides'
// answers differ (0.0015 vs 0.0016), so a side-swap mutant is visible.
func TestCounterfactualSlippageBidAdversePositive(t *testing.T) {
	at := slipBook(t, slipT0, 12,
		[][2]string{{"100.00", "1.0"}, {"99.90", "1.0"}},
		[][2]string{{"100.10", "1.0"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 13,
		[][2]string{{"99.90", "0.4"}, {"99.80", "2.0"}},
		[][2]string{{"100.10", "1.0"}})

	got, err := CounterfactualSlippage(at, later, mdomain.SideBid, qty(t, "1.0"))
	if err != nil {
		t.Fatalf("slippage: %v", err)
	}
	if want := rate(t, "0.0016"); got != want {
		t.Fatalf("slippage = %s, want %s", got, want)
	}
}

// A book that IMPROVED over the window yields a negative sample — kept in
// the distribution, never floored: flooring would thin the favorable mass
// and shift the p90 upward for the wrong reason (the honest reason to
// overstate is selection, not sample surgery).
func TestCounterfactualSlippageNegativeSampleKept(t *testing.T) {
	at := slipBook(t, slipT0, 14,
		[][2]string{{"99.90", "1.0"}},
		[][2]string{{"100.00", "1.0"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 15,
		[][2]string{{"99.70", "1.0"}},
		[][2]string{{"99.80", "2.0"}})

	got, err := CounterfactualSlippage(at, later, mdomain.SideAsk, qty(t, "1.0"))
	if err != nil {
		t.Fatalf("slippage: %v", err)
	}
	if want := rate(t, "-0.002"); got != want {
		t.Fatalf("slippage = %s, want %s (negative, kept)", got, want)
	}
}

func TestCounterfactualSlippageRefusals(t *testing.T) {
	at := slipBook(t, slipT0, 16, [][2]string{{"99.90", "1.0"}}, [][2]string{{"100.00", "0.5"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 17, [][2]string{{"99.90", "1.0"}}, [][2]string{{"100.10", "0.4"}})

	// Later book too thin for the candidate: ErrDepthShort, not a zero.
	if _, err := CounterfactualSlippage(at, later, mdomain.SideAsk, qty(t, "0.5")); !errors.Is(err, ErrDepthShort) {
		t.Fatalf("thin later book: %v, want ErrDepthShort", err)
	}
	// Pair order inverted.
	if _, err := CounterfactualSlippage(later, at, mdomain.SideAsk, qty(t, "0.1")); !errors.Is(err, ErrBookPairMismatch) {
		t.Fatalf("inverted pair: %v, want ErrBookPairMismatch", err)
	}
	// Two instruments are not one book.
	foreign := later
	foreign.Instrument = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
	if _, err := CounterfactualSlippage(at, foreign, mdomain.SideAsk, qty(t, "0.1")); !errors.Is(err, ErrBookPairMismatch) {
		t.Fatalf("cross-instrument pair: %v, want ErrBookPairMismatch", err)
	}
	// Zero candidate.
	zero := fixedpoint.Quantity{}
	if _, err := CounterfactualSlippage(at, later, mdomain.SideAsk, zero); err == nil {
		t.Fatal("zero candidate measured")
	}
}

// Adverse fill move, both sides against one fixture pair: asks moved UP
// 100.00→100.05 (adverse +0.0005 for a buyer) while bids moved UP
// 99.90→99.95 (favorable −0.0005... for a SELLER a rising bid is
// favorable, so the bid-side sample is negative). One pair, two signs — a
// direction-swap mutant fails both.
func TestAdverseFillMoveSigns(t *testing.T) {
	at := slipBook(t, slipT0, 18,
		[][2]string{{"99.90", "1.0"}},
		[][2]string{{"100.00", "1.0"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 19,
		[][2]string{{"99.95", "1.0"}},
		[][2]string{{"100.05", "1.0"}})

	up, err := AdverseFillMove(at, later, mdomain.SideAsk)
	if err != nil {
		t.Fatalf("ask move: %v", err)
	}
	if want := rate(t, "0.0005"); up != want {
		t.Fatalf("ask move = %s, want %s", up, want)
	}
	down, err := AdverseFillMove(at, later, mdomain.SideBid)
	if err != nil {
		t.Fatalf("bid move: %v", err)
	}
	if sign, err := down.Sign(); err != nil || sign >= 0 {
		t.Fatalf("bid move = %s, want negative (favorable for a seller)", down)
	}
}

// UpperQuantile: an inexact rank rounds UP (q=0.9 over 10 samples → rank
// 8.1 → index 9, the largest), while an exact rank takes ITS OWN statistic
// (q=0.9 over 11 samples → rank 9.0 → index 9, the second-largest). The
// two fixtures differ by one sample and give different answers, so the
// ceil-vs-identity boundary cannot collapse.
func TestUpperQuantileRankBoundary(t *testing.T) {
	mk := func(n int) []fixedpoint.Rate {
		out := make([]fixedpoint.Rate, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, rate(t, "0.00"+string(rune('0'+i%10)))) // 0.001..0.009,0.000 — need distinct; build below instead
		}
		return out
	}
	_ = mk
	ten := []fixedpoint.Rate{}
	for _, s := range []string{"0.001", "0.002", "0.003", "0.004", "0.005", "0.006", "0.007", "0.008", "0.009", "0.010"} {
		ten = append(ten, rate(t, s))
	}
	got, err := UpperQuantile(ten, rate(t, "0.9"))
	if err != nil {
		t.Fatalf("inexact rank: %v", err)
	}
	if want := rate(t, "0.010"); got != want {
		t.Fatalf("q90 of 10 = %s, want %s (ceil)", got, want)
	}
	eleven := append(append([]fixedpoint.Rate{}, ten...), rate(t, "0.011"))
	got, err = UpperQuantile(eleven, rate(t, "0.9"))
	if err != nil {
		t.Fatalf("exact rank: %v", err)
	}
	if want := rate(t, "0.010"); got != want {
		t.Fatalf("q90 of 11 = %s, want %s (exact rank, its own statistic)", got, want)
	}
	if _, err := UpperQuantile(nil, rate(t, "0.9")); !errors.Is(err, ErrNoResiduals) {
		t.Fatalf("empty samples: %v, want ErrNoResiduals", err)
	}
}

// The merge: four points, each input winning somewhere, INCLUDING exactly
// at the median — below the median the more NEGATIVE wins, at and above it
// the more POSITIVE. Every pair differs, so a merge that picks one input
// everywhere, or applies one direction everywhere, fails on some point.
func TestMergeAdversePointwise(t *testing.T) {
	qp := func(q, s string) domain.QuantilePoint {
		return domain.QuantilePoint{Q: rate(t, q), Shift: rate(t, s)}
	}
	a := []domain.QuantilePoint{
		qp("0.1", "-0.005"), // a more negative — a wins below median
		qp("0.3", "-0.001"), // b more negative — b wins below median
		qp("0.5", "0.0004"), // b larger — b wins AT the median (>= rule)
		qp("0.9", "0.006"),  // a larger — a wins above median
	}
	b := []domain.QuantilePoint{
		qp("0.1", "-0.004"),
		qp("0.3", "-0.002"),
		qp("0.5", "0.0007"),
		qp("0.9", "0.005"),
	}
	got, err := MergeAdverse(a, b)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	want := []string{"-0.005", "-0.002", "0.0007", "0.006"}
	for i, w := range want {
		if got[i].Shift != rate(t, w) {
			t.Fatalf("point %d shift = %s, want %s", i, got[i].Shift, w)
		}
		if got[i].Q != a[i].Q {
			t.Fatalf("point %d q moved", i)
		}
	}
	// The merged set must be consumable by the domain under the same
	// read-time rule — Dispersion.Validate is the authority.
	disp := domain.Dispersion{Points: got, Artifact: domain.CalibrationRef{ID: "fixture:merge", Hash: "fixture"}}
	if err := disp.Validate(); err != nil {
		t.Fatalf("merged points rejected by the consumer: %v", err)
	}
}

func TestMergeAdverseRefusals(t *testing.T) {
	qp := func(q, s string) domain.QuantilePoint {
		return domain.QuantilePoint{Q: rate(t, q), Shift: rate(t, s)}
	}
	a := []domain.QuantilePoint{qp("0.1", "-0.001"), qp("0.9", "0.002")}
	// Different grids: same length, different Q.
	shifted := []domain.QuantilePoint{qp("0.2", "-0.001"), qp("0.9", "0.002")}
	if _, err := MergeAdverse(a, shifted); !errors.Is(err, ErrMergeMisaligned) {
		t.Fatalf("misaligned grids: %v, want ErrMergeMisaligned", err)
	}
	// Different lengths.
	if _, err := MergeAdverse(a, a[:1]); !errors.Is(err, ErrMergeMisaligned) {
		t.Fatalf("length mismatch: %v, want ErrMergeMisaligned", err)
	}
	// Empty inputs.
	if _, err := MergeAdverse(nil, a); !errors.Is(err, ErrNoResiduals) {
		t.Fatalf("empty input: %v, want ErrNoResiduals", err)
	}
}

// The rounding-mode fixture: every other ratio in this file terminates
// before the ninth decimal, where half-even and truncate-toward-zero are
// the same function — the fixture-too-small collapse. This pair's exact
// ratio is 1.5e-9: half-even resolves the tie to the EVEN digit (2e-9);
// truncation answers 1e-9. One mantissa of daylight, which is exactly
// what a rounding-mode mutant needs to be visible.
func TestCounterfactualSlippageRoundingTie(t *testing.T) {
	at := slipBook(t, slipT0, 30,
		[][2]string{{"3.999999990", "2.0"}},
		[][2]string{{"4.000000000", "2.0"}})
	later := slipBook(t, slipT0.Add(250*time.Millisecond), 31,
		[][2]string{{"3.999999990", "2.0"}},
		[][2]string{{"4.000000006", "2.0"}})

	got, err := CounterfactualSlippage(at, later, mdomain.SideAsk, qty(t, "2.0"))
	if err != nil {
		t.Fatalf("slippage: %v", err)
	}
	// ref = 8.0; fill = 8.000000012; ratio = 1.5e-9 → half-even → 2e-9.
	if want := rate(t, "0.000000002"); got != want {
		t.Fatalf("slippage = %s, want %s (half-even resolves the tie upward to even)", got, want)
	}
}
