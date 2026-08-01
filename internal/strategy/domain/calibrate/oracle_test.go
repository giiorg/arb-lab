package calibrate_test

// The commit-1 oracles, in the standing tradition: each derivation is
// recomputed in a different representation with a different rounding
// regime — big.Float at 256 bits for the residual chain (no scale-9
// rounding anywhere), exact big.Rat rank arithmetic for the quantile
// selection — and every comparison carries an inline control showing a
// perturbed answer fails, so a vacuous predicate cannot read as a pass.
//
// The residual oracle DELIBERATELY re-implements the EWMA rather than
// importing the domain's own oracle helper: the independence is the point,
// and a shared helper would let one transcription error certify itself.

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

// residualTolerance matches the domain EWMA oracle's stated budget: the
// residual is one subtraction on top of the EWMA, adding a single ulp to
// the same composite bound.
const residualTolerance = "0.000002"

func bigRatOf(t interface{ Fatalf(string, ...any) }, s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("SetString(%q)", s)
	}
	return r
}

// lambda256 is 2^(-1/4096) at 256 bits: twelve square roots of 1/2.
func lambda256() *big.Float {
	x := new(big.Float).SetPrec(256).SetRat(big.NewRat(1, 2))
	for range 12 {
		x.Sqrt(x)
	}
	return x
}

// oracleTrailing recomputes the trailing EWMA definition in big.Float:
// quantized exponents (the stated design), exact decay base, the stated
// 31-half-life skip, no scale-9 rounding.
func oracleTrailing(t *rapid.T, window []domain.RealizedSettlement, halfLife time.Duration) *big.Float {
	lambda := lambda256()
	newest := window[len(window)-1].At

	num := new(big.Float).SetPrec(256)
	den := new(big.Float).SetPrec(256)
	for _, s := range window {
		age := newest.Sub(s.At)
		if age/halfLife >= 31 {
			continue
		}
		n := new(big.Int).Mul(big.NewInt(4096), big.NewInt(int64(age)))
		n.Lsh(n, 1)
		n.Add(n, big.NewInt(int64(halfLife)))
		n.Div(n, new(big.Int).Lsh(big.NewInt(int64(halfLife)), 1))

		w := new(big.Float).SetPrec(256).SetInt64(1)
		base := new(big.Float).SetPrec(256).Set(lambda)
		for steps := n.Int64(); steps > 0; steps >>= 1 {
			if steps&1 == 1 {
				w.Mul(w, base)
			}
			base.Mul(base, base)
		}
		r := new(big.Float).SetPrec(256).SetRat(bigRatOf(t, s.Rate.Decimal()))
		num.Add(num, new(big.Float).SetPrec(256).Mul(w, r))
		den.Add(den, w)
	}
	return new(big.Float).SetPrec(256).Quo(num, den)
}

// withinTol reports how got differs from the oracle beyond tolerance, or
// "" if it does not.
func withinTol(t *rapid.T, got string, oracle *big.Float, tolerance string) string {
	g := new(big.Float).SetPrec(256).SetRat(bigRatOf(t, got))
	diff := new(big.Float).SetPrec(256).Sub(g, oracle)
	diff.Abs(diff)
	tol := new(big.Float).SetPrec(256).SetRat(bigRatOf(t, tolerance))
	if diff.Cmp(tol) > 0 {
		return fmt.Sprintf("fp %s vs oracle %s: |diff| %s exceeds %s",
			got, oracle.Text('g', 20), diff.Text('g', 5), tolerance)
	}
	return ""
}

// rapidRate renders a scale-9 mantissa as a Rate via its exact decimal
// text.
func rapidRate(t *rapid.T, mantissa int64) fixedpoint.Rate {
	s := new(big.Rat).SetFrac64(mantissa, 1_000_000_000).FloatString(9)
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func TestProperty_ResidualsMatchOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		halfLife := time.Duration(rapid.Int64Range(1, 3).Draw(t, "halfLifeHours")) * time.Hour

		n := rapid.IntRange(5, 40).Draw(t, "n")
		offsets := rapid.SliceOfNDistinct(rapid.Int64Range(0, 12_000), n, n, rapid.ID).Draw(t, "offsets")
		sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
		history := make([]domain.RealizedSettlement, n)
		for i, minutes := range offsets {
			at := calEra.Add(time.Duration(minutes) * time.Minute)
			history[i] = domain.RealizedSettlement{
				Instrument: calPerp,
				At:         at,
				Rate:       rapidRate(t, rapid.Int64Range(-7_500_000, 7_500_000).Draw(t, fmt.Sprintf("r%d", i))),
				Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at},
			}
		}

		got, err := calibrate.Residuals(history, halfLife)
		if errors.Is(err, calibrate.ErrHistoryShort) {
			return // a span the warmup honestly refuses is not a case
		}
		if err != nil {
			t.Fatalf("Residuals: %v", err)
		}

		// The oracle recomputes the warmup rule and every forecast
		// independently, then walks the two residual lists in lockstep.
		horizon := 31 * halfLife
		k := 0
		for i := 1; i < n; i++ {
			if history[i-1].At.Sub(history[0].At) < horizon {
				continue
			}
			if k >= len(got) {
				t.Fatalf("oracle expects a residual at row %d; production collected only %d", i, len(got))
			}
			forecast := oracleTrailing(t, history[:i], halfLife)
			realized := new(big.Float).SetPrec(256).SetRat(bigRatOf(t, history[i].Rate.Decimal()))
			oracle := new(big.Float).SetPrec(256).Sub(realized, forecast)
			if diff := withinTol(t, got[k].Decimal(), oracle, residualTolerance); diff != "" {
				t.Fatalf("residual %d (row %d) disagrees: %s", k, i, diff)
			}
			k++
		}
		if k != len(got) {
			t.Fatalf("production collected %d residuals; oracle's warmup admits %d", len(got), k)
		}

		// Inline control: a residual three tolerances off must be caught.
		lastForecast := oracleTrailing(t, history[:n-1], halfLife)
		lastRealized := new(big.Float).SetPrec(256).SetRat(bigRatOf(t, history[n-1].Rate.Decimal()))
		lastOracle := new(big.Float).SetPrec(256).Sub(lastRealized, lastForecast)
		shifted := new(big.Float).SetPrec(256).Set(lastOracle)
		shifted.Add(shifted, new(big.Float).SetPrec(256).SetRat(bigRatOf(t, "0.000006")))
		if diff := withinTol(t, shifted.Text('f', 12), lastOracle, residualTolerance); diff == "" {
			t.Fatal("the oracle accepted a residual three tolerances off — the check is vacuous")
		}
	})
}

func TestProperty_PointsMatchOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 40).Draw(t, "n")
		residuals := make([]fixedpoint.Rate, n)
		for i := range residuals {
			residuals[i] = rapidRate(t, rapid.Int64Range(-7_500_000, 7_500_000).Draw(t, fmt.Sprintf("res%d", i)))
		}
		m := rapid.IntRange(1, 6).Draw(t, "m")
		qMantissas := rapid.SliceOfNDistinct(rapid.Int64Range(1, 999_999_999), m, m, rapid.ID).Draw(t, "qs")
		sort.Slice(qMantissas, func(i, j int) bool { return qMantissas[i] < qMantissas[j] })
		gridRates := make([]fixedpoint.Rate, m)
		for i, qm := range qMantissas {
			gridRates[i] = rapidRate(t, qm)
		}

		pts, err := calibrate.Points(residuals, gridRates)
		if err != nil {
			t.Fatalf("Points: %v", err)
		}

		// Oracle: exact big.Rat rank arithmetic over exactly-sorted
		// decimals — selection is an ORDER STATISTIC, so agreement is
		// byte equality, not a tolerance.
		sortedDec := make([]*big.Rat, n)
		for i, r := range residuals {
			sortedDec[i] = bigRatOf(t, r.Decimal())
		}
		sort.Slice(sortedDec, func(i, j int) bool { return sortedDec[i].Cmp(sortedDec[j]) < 0 })
		half := big.NewRat(1, 2)
		for i, p := range pts {
			q := bigRatOf(t, p.Q.Decimal())
			rank := new(big.Rat).Mul(q, big.NewRat(int64(n-1), 1))
			idx := new(big.Int).Quo(rank.Num(), rank.Denom()).Int64()
			if q.Cmp(half) >= 0 && !rank.IsInt() {
				idx++
			}
			want := sortedDec[idx].FloatString(9)
			gotDec := bigRatOf(t, p.Shift.Decimal()).FloatString(9)
			if gotDec != want {
				t.Fatalf("q=%s (point %d): production %s, oracle order statistic %d = %s",
					p.Q.Decimal(), i, gotDec, idx, want)
			}
		}
		// No tolerance, no perturbation control: the predicate is byte
		// equality against an independently computed order statistic, which
		// cannot be vacuous the way a tolerance can. The rule itself (floor
		// below the median, ceil at or above) is anchored by the
		// deterministic outward-selection fixture in calibrate_test.go,
		// whose 0.45/0.51 pair fails under nearest, inward, or a flipped
		// median comparison.
	})
}
