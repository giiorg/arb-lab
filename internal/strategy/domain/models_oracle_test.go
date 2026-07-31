package domain_test

// The slice B oracle, in the fixedpoint tradition: real defects in this repo
// have been found by two implementations disagreeing, not by review. The
// EWMA is recomputed in big.Float at 256 bits with NO scale-9 rounding
// anywhere, and the quantile selection is judged against exact big.Rat
// interpolation — a different representation and a different rounding regime
// from the production path, which is where the independence lives.
//
// Each check is a predicate plus an inline broken-implementation control,
// per the property_test.go discipline: a comparison proves nothing until the
// broken thing is shown to fail it.

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// ewmaTolerance is the stated error budget between the production EWMA and
// the ideal-arithmetic oracle, in absolute rate terms. Its components: the
// decay constant is within half an ulp of 2^(-1/4096), amplified by at most
// e^(m·5e-10) across m steps (≤ 1.4e-5 for any weight that still matters);
// the pow-by-squaring rounds half-even at scale 9 per multiplication
// (≤ ~2e-6 relative for significant weights); term products and the final
// division contribute single ulps. Weighted through rates bounded by the
// venue clamps (|r| ≤ 0.0075 in these generators), the composite lands
// comfortably under 2e-6.
const ewmaTolerance = "0.000002"

// bigRat parses a fixedpoint decimal into an exact rational.
func bigRat(t interface{ Fatalf(string, ...any) }, s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("SetString(%q)", s)
	}
	return r
}

// lambdaExact is 2^(-1/4096) at 256 bits: twelve square roots of 1/2.
func lambdaExact() *big.Float {
	x := new(big.Float).SetPrec(256).SetRat(big.NewRat(1, 2))
	for range 12 {
		x.Sqrt(x)
	}
	return x
}

// oracleEWMA recomputes the EWMA definition in big.Float: quantized
// exponents (the stated design), exact decay base, no scale-9 rounding.
func oracleEWMA(t *rapid.T, window []domain.RealizedSettlement, halfLife time.Duration) *big.Float {
	lambda := lambdaExact()
	newest := window[len(window)-1].At

	num := new(big.Float).SetPrec(256)
	den := new(big.Float).SetPrec(256)
	for _, s := range window {
		// steps = round(4096 · age / halfLife), half-up, in exact integers —
		// the same definition, independently written.
		age := big.NewInt(int64(newest.Sub(s.At)))
		n := new(big.Int).Mul(big.NewInt(4096), age)
		n.Lsh(n, 1)
		n.Add(n, big.NewInt(int64(halfLife)))
		n.Div(n, new(big.Int).Lsh(big.NewInt(int64(halfLife)), 1))

		// w = lambda^steps by squaring at full precision.
		w := new(big.Float).SetPrec(256).SetInt64(1)
		base := new(big.Float).SetPrec(256).Set(lambda)
		for steps := n.Int64(); steps > 0; steps >>= 1 {
			if steps&1 == 1 {
				w.Mul(w, base)
			}
			base.Mul(base, base)
		}

		r := new(big.Float).SetPrec(256).SetRat(bigRat(t, s.Rate.Decimal()))
		num.Add(num, new(big.Float).SetPrec(256).Mul(w, r))
		den.Add(den, w)
	}
	return new(big.Float).SetPrec(256).Quo(num, den)
}

// checkEWMA reports how the production answer differs from the oracle beyond
// the stated tolerance, or "" if it does not.
func checkEWMA(t *rapid.T, got string, oracle *big.Float, tolerance string) string {
	g := new(big.Float).SetPrec(256).SetRat(bigRat(t, got))
	diff := new(big.Float).SetPrec(256).Sub(g, oracle)
	diff.Abs(diff)
	tol := new(big.Float).SetPrec(256).SetRat(bigRat(t, tolerance))
	if diff.Cmp(tol) > 0 {
		return fmt.Sprintf("fp %s vs oracle %s: |diff| %s exceeds %s",
			got, oracle.Text('g', 20), diff.Text('g', 5), tolerance)
	}
	return ""
}

func TestProperty_EWMAMatchesOracle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		halfLife := time.Duration(rapid.Int64Range(1, 96).Draw(t, "halfLifeHours")) * time.Hour

		// Ascending distinct ages up to ~35 half-lives of 8h — past the
		// skip horizon, so the oracle's indifference to it is exercised too.
		n := rapid.IntRange(1, 12).Draw(t, "n")
		offsets := rapid.SliceOfNDistinct(rapid.Int64Range(0, 35*8*60), n, n, rapid.ID).Draw(t, "offsets")
		window := make([]domain.RealizedSettlement, n)
		for i, minutes := range offsets {
			window[i] = domain.RealizedSettlement{
				Instrument: perpInst,
				At:         baseAt.Add(-time.Duration(minutes) * time.Minute),
				Rate:       rapidRate(t, rapid.Int64Range(-7_500_000, 7_500_000).Draw(t, fmt.Sprintf("r%d", i))),
				Watermark:  watermark(baseAt, 5),
			}
		}
		// Sort ascending by construction: offsets descend in age order.
		sortWindow(window)

		got, err := domain.EWMA(window, halfLife)
		if err != nil {
			t.Fatalf("EWMA: %v", err)
		}
		oracle := oracleEWMA(t, window, halfLife)
		if diff := checkEWMA(t, got.Decimal(), oracle, ewmaTolerance); diff != "" {
			t.Fatalf("EWMA disagrees: %s", diff)
		}

		// The inline control: an answer shifted by three tolerances must be
		// caught, or the comparison above is vacuous.
		shifted := new(big.Float).SetPrec(256).SetRat(bigRat(t, got.Decimal()))
		bump := new(big.Float).SetPrec(256).SetRat(bigRat(t, "0.000006"))
		shifted.Add(shifted, bump)
		if diff := checkEWMA(t, shifted.Text('f', 12), oracle, ewmaTolerance); diff == "" {
			t.Fatal("the oracle accepted an answer three tolerances off — the check is vacuous")
		}
	})
}

// rapidRate renders a scale-9 mantissa as a Rate via its exact decimal text.
func rapidRate(t *rapid.T, mantissa int64) fixedpoint.Rate {
	s := new(big.Rat).SetFrac64(mantissa, 1_000_000_000).FloatString(9)
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// sortWindow orders settlements oldest-first without importing sort for one
// call site.
func sortWindow(w []domain.RealizedSettlement) {
	for i := 1; i < len(w); i++ {
		for j := i; j > 0 && w[j].At.Before(w[j-1].At); j-- {
			w[j], w[j-1] = w[j-1], w[j]
		}
	}
}

// checkShiftOutward judges a selection against exact big.Rat interpolation:
// inside the measured span, the selected shift must sit on or beyond the
// interpolated quantile function on the ADVERSE side of the requested tail —
// at or below it for a lower-tail draw, at or above for an upper-tail one.
// This is the semantic claim ("outward is the conservative direction") checked
// without reference to the selection rule's own mechanics.
func checkShiftOutward(t interface{ Fatalf(string, ...any) }, d domain.Dispersion, q string, got string) string {
	qr := bigRat(t, q)
	var lower, upper *domain.QuantilePoint
	for i := range d.Points {
		pq := bigRat(t, d.Points[i].Q.Decimal())
		if pq.Cmp(qr) <= 0 {
			lower = &d.Points[i]
		}
		if upper == nil && pq.Cmp(qr) >= 0 {
			upper = &d.Points[i]
		}
	}
	if lower == nil || upper == nil {
		return "q outside span reached the interpolation check"
	}
	lq, uq := bigRat(t, lower.Q.Decimal()), bigRat(t, upper.Q.Decimal())
	ls, us := bigRat(t, lower.Shift.Decimal()), bigRat(t, upper.Shift.Decimal())

	interp := new(big.Rat)
	if lq.Cmp(uq) == 0 {
		interp.Set(ls)
	} else {
		span := new(big.Rat).Sub(uq, lq)
		frac := new(big.Rat).Quo(new(big.Rat).Sub(qr, lq), span)
		interp.Add(ls, new(big.Rat).Mul(frac, new(big.Rat).Sub(us, ls)))
	}

	gr := bigRat(t, got)
	half := big.NewRat(1, 2)
	if qr.Cmp(half) < 0 {
		if gr.Cmp(interp) > 0 {
			return fmt.Sprintf("lower-tail q=%s selected %s ABOVE the interpolated %s", q, got, interp.FloatString(12))
		}
	} else if gr.Cmp(interp) < 0 {
		return fmt.Sprintf("upper-tail q=%s selected %s BELOW the interpolated %s", q, got, interp.FloatString(12))
	}
	return ""
}

func TestProperty_ShiftAtIsOutwardConservative(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 6).Draw(t, "n")
		qs := rapid.SliceOfNDistinct(rapid.Int64Range(1, 999_999_998), n, n, rapid.ID).Draw(t, "qs")
		shifts := rapid.SliceOfN(rapid.Int64Range(-500_000, 500_000), n, n).Draw(t, "shifts")
		sortInt64(qs)
		sortInt64(shifts)

		d := domain.Dispersion{Artifact: fixtureCalibration("dispersion")}
		for i := range n {
			d.Points = append(d.Points, domain.QuantilePoint{
				Q:     rapidRate(t, qs[i]),
				Shift: rapidRate(t, shifts[i]),
			})
		}
		if err := d.Validate(); err != nil {
			t.Fatalf("generated dispersion invalid: %v", err)
		}

		// A draw inside the measured span: selected shift must be one of the
		// points' shifts AND on the adverse side of the interpolation.
		qm := rapid.Int64Range(qs[0], qs[n-1]).Draw(t, "q")
		q := rapidRate(t, qm)
		got, err := d.ShiftAt(q)
		if err != nil {
			t.Fatalf("ShiftAt(%s): %v", q, err)
		}
		isPoint := false
		for _, p := range d.Points {
			if c, err := p.Shift.Cmp(got); err == nil && c == 0 {
				isPoint = true
				break
			}
		}
		if !isPoint {
			t.Fatalf("ShiftAt(%s) = %s, which is no measured point's shift — interpolation invented a value", q, got)
		}
		if diff := checkShiftOutward(t, d, q.Decimal(), got.Decimal()); diff != "" {
			t.Fatalf("%s", diff)
		}
	})
}

// The outward predicate's control: a selector that always takes the lower
// point violates the upper-tail half on the strictly-increasing fixture, so
// the predicate is shown to bite before the property above is believed.
func TestShiftOutwardPredicateControl(t *testing.T) {
	d := fixtureDispersion(t)
	// q=0.6 brackets (0.50 → 0.00002, 0.75 → 0.00009); always-lower answers
	// 0.00002, interpolation gives 0.0000448: an upper-tail draw below the
	// interpolated value must be objected to.
	if diff := checkShiftOutward(t, d, "0.6", "0.00002"); diff == "" {
		t.Fatal("the outward predicate accepted an always-lower selector — it is vacuous")
	}
}

func sortInt64(s []int64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
