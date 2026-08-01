package calibrate_test

// The slice B battery for the residual, quantile and drift derivations.
//
// Fixture shapes are chosen against the fixture-collapse checklist, §41i's
// pre-named suspects first:
//
//   - every residual set is SKEWED about zero, so adverse-side selection
//     and sign conventions are distinct questions;
//   - quantile ranks land both exactly ON order statistics and strictly
//     between them, on BOTH sides of the median, so floor-vs-ceil and the
//     median boundary each have a fixture that moves if they flip;
//   - one warmup fixture spans EXACTLY the saturation horizon, so >= and >
//     are different answers;
//   - the drift fixture's interval is 7h, a value at which the annualized
//     ratio does not divide exactly — a hardcoded 8h constant or a wrong
//     rounding mode both move the pinned digits.

import (
	"errors"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

var calPerp = ids.VenueInstrument{
	Venue:    "binance",
	Product:  ids.ProductLinearPerp,
	Symbol:   "BTCUSDT",
	Contract: ids.ContractLinear,
}

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// settled builds one realized settlement at base+offset with the given
// rate.
func settled(t *testing.T, base time.Time, offset time.Duration, rate string) domain.RealizedSettlement {
	t.Helper()
	at := base.Add(offset)
	return domain.RealizedSettlement{
		Instrument: calPerp,
		At:         at,
		Rate:       mustRate(t, rate),
		Watermark:  mdomain.Watermark{SourceTime: at, RecvTime: at},
	}
}

var calEra = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

// --- Residuals ---

func TestResidualsCausalPinned(t *testing.T) {
	// halfLife 1h with rows ≥ 31h apart: every older row sits past the
	// saturation horizon, so each trailing EWMA is EXACTLY the newest
	// window rate and the residuals are hand-checkable differences. Rates
	// are skewed — no symmetric pair anywhere.
	history := []domain.RealizedSettlement{
		settled(t, calEra, 0, "0.0001"),
		settled(t, calEra, 40*time.Hour, "0.00035"),
		settled(t, calEra, 80*time.Hour, "-0.00021"),
		settled(t, calEra, 120*time.Hour, "0.00007"),
	}
	got, err := calibrate.Residuals(history, time.Hour)
	if err != nil {
		t.Fatalf("Residuals: %v", err)
	}
	// i=1 is pre-warmup (window spans 0h). i=2: forecast = 0.00035,
	// realized -0.00021. i=3: forecast = -0.00021, realized 0.00007.
	want := []string{"-0.00056", "0.00028"}
	if len(got) != len(want) {
		t.Fatalf("collected %d residuals; want %d", len(got), len(want))
	}
	for n, w := range want {
		if got[n].Decimal() != w {
			t.Errorf("residual %d = %s; want %s", n, got[n].Decimal(), w)
		}
	}
}

func TestResidualsWarmupBoundary(t *testing.T) {
	// The window spanning EXACTLY the horizon collects — "already spans"
	// is inclusive. halfLife 1h → horizon 31h; the second window's newest
	// row sits exactly 31h after the first.
	history := []domain.RealizedSettlement{
		settled(t, calEra, 0, "0.0001"),
		settled(t, calEra, 31*time.Hour, "0.0004"),
		settled(t, calEra, 32*time.Hour, "0.0001"),
	}
	got, err := calibrate.Residuals(history, time.Hour)
	if err != nil {
		t.Fatalf("Residuals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("collected %d residuals; want exactly the at-boundary one", len(got))
	}
	// Window rows: age 31h (skipped as saturated) and age 0 → forecast
	// 0.0004; realized 0.0001.
	if got[0].Decimal() != "-0.0003" {
		t.Errorf("boundary residual = %s; want -0.0003", got[0].Decimal())
	}

	// One hour short of the horizon: nothing collectible, and the refusal
	// names the state rather than returning an empty measured set.
	short := []domain.RealizedSettlement{
		settled(t, calEra, 0, "0.0001"),
		settled(t, calEra, 30*time.Hour, "0.0004"),
		settled(t, calEra, 30*time.Hour+30*time.Minute, "0.0001"),
	}
	if _, err := calibrate.Residuals(short, time.Hour); !errors.Is(err, calibrate.ErrHistoryShort) {
		t.Fatalf("short history error = %v; want ErrHistoryShort", err)
	}
}

func TestResidualsRefusals(t *testing.T) {
	valid := []domain.RealizedSettlement{
		settled(t, calEra, 0, "0.0001"),
		settled(t, calEra, 40*time.Hour, "0.0002"),
		settled(t, calEra, 80*time.Hour, "0.0003"),
	}

	if _, err := calibrate.Residuals(nil, time.Hour); !errors.Is(err, domain.ErrWindowEmpty) {
		t.Errorf("empty history error = %v; want ErrWindowEmpty", err)
	}
	if _, err := calibrate.Residuals(valid, 0); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero half-life error = %v; want ErrUnset", err)
	}

	// The LAST row is the one no window ever validates — the upfront pass
	// exists for exactly these two rows.
	unordered := append(append([]domain.RealizedSettlement(nil), valid...),
		settled(t, calEra, 79*time.Hour, "0.0004"))
	if _, err := calibrate.Residuals(unordered, time.Hour); !errors.Is(err, domain.ErrNotAscending) {
		t.Errorf("unordered last row error = %v; want ErrNotAscending", err)
	}

	foreign := settled(t, calEra, 120*time.Hour, "0.0004")
	foreign.Instrument = ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
	mixed := append(append([]domain.RealizedSettlement(nil), valid...), foreign)
	if _, err := calibrate.Residuals(mixed, time.Hour); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Errorf("foreign last row error = %v; want ErrLegsMismatch", err)
	}
}

// --- Points ---

func grid(t *testing.T, qs ...string) []fixedpoint.Rate {
	t.Helper()
	out := make([]fixedpoint.Rate, 0, len(qs))
	for _, q := range qs {
		out = append(out, mustRate(t, q))
	}
	return out
}

func TestPointsOutwardSelection(t *testing.T) {
	// Five skewed residuals, deliberately UNSORTED on input. Sorted:
	// [-0.00009, -0.00004, -0.00001, 0.00003, 0.0002].
	residuals := grid(t, "0.0002", "-0.00004", "-0.00009", "0.00003", "-0.00001")

	cases := []struct{ q, want string }{
		{"0.25", "-0.00004"}, // rank 1.0 exact — no rounding in sight
		{"0.3", "-0.00004"},  // rank 1.2, below median → floor → 1
		{"0.45", "-0.00004"}, // rank 1.8, below median → floor, NOT nearest
		{"0.5", "-0.00001"},  // rank 2.0 exact, the median itself
		{"0.51", "0.00003"},  // rank 2.04, at/above median → ceil, NOT nearest
		{"0.6", "0.00003"},   // rank 2.4, above median → ceil → 3
		{"0.75", "0.00003"},  // rank 3.0 exact
		{"0.9", "0.0002"},    // rank 3.6 → ceil → 4
	}
	qs := make([]string, len(cases))
	for n, c := range cases {
		qs[n] = c.q
	}
	pts, err := calibrate.Points(residuals, grid(t, qs...))
	if err != nil {
		t.Fatalf("Points: %v", err)
	}
	if len(pts) != len(cases) {
		t.Fatalf("%d points; want %d", len(pts), len(cases))
	}
	for n, c := range cases {
		if pts[n].Q.Decimal() != c.q {
			t.Errorf("point %d Q = %s; want %s", n, pts[n].Q.Decimal(), c.q)
		}
		if pts[n].Shift.Decimal() != c.want {
			t.Errorf("q=%s shift = %s; want %s", c.q, pts[n].Shift.Decimal(), c.want)
		}
	}

	// The median itself with an INEXACT rank: four residuals put q=0.5 at
	// rank 1.5, where "at the median takes the upper" and "above the
	// median takes the upper" are different rules — every five-element
	// case above pins the median at an exact rank, where they collapse.
	four := grid(t, "-0.00009", "-0.00004", "0.00003", "0.0002")
	atMedian, err := calibrate.Points(four, grid(t, "0.5"))
	if err != nil {
		t.Fatalf("Points at median: %v", err)
	}
	if atMedian[0].Shift.Decimal() != "0.00003" {
		t.Errorf("q=0.5 rank 1.5 shift = %s; want the UPPER statistic 0.00003", atMedian[0].Shift.Decimal())
	}

	// The emitted points must satisfy the domain's own dispersion shape —
	// strictly ascending Q, non-decreasing Shift — under a real artifact
	// identity, because that is exactly how the tool will freeze them.
	d := domain.Dispersion{Points: pts, Artifact: domain.CalibrationRef{
		ID: domain.FixturePrefix + "points", Hash: "points",
	}}
	if err := d.Validate(); err != nil {
		t.Errorf("emitted points fail Dispersion.Validate: %v", err)
	}
}

func TestPointsDuplicatesAndSingle(t *testing.T) {
	// Duplicate residual values: shifts stay non-decreasing, never refuse.
	dup := grid(t, "-0.00002", "-0.00002", "0.00011")
	pts, err := calibrate.Points(dup, grid(t, "0.25", "0.75"))
	if err != nil {
		t.Fatalf("Points with duplicates: %v", err)
	}
	if pts[0].Shift.Decimal() != "-0.00002" || pts[1].Shift.Decimal() != "0.00011" {
		t.Errorf("duplicate shifts = %s, %s", pts[0].Shift.Decimal(), pts[1].Shift.Decimal())
	}

	// A single residual answers every quantile with itself — validity
	// only; selection claims live in the five-element fixture above.
	one, err := calibrate.Points(grid(t, "0.00042"), grid(t, "0.1", "0.9"))
	if err != nil {
		t.Fatalf("Points single: %v", err)
	}
	for _, p := range one {
		if p.Shift.Decimal() != "0.00042" {
			t.Errorf("single-residual shift at %s = %s", p.Q.Decimal(), p.Shift.Decimal())
		}
	}
}

func TestPointsRefusals(t *testing.T) {
	residuals := grid(t, "-0.00009", "0.0002")

	if _, err := calibrate.Points(nil, grid(t, "0.5")); !errors.Is(err, calibrate.ErrNoResiduals) {
		t.Errorf("empty residuals error = %v; want ErrNoResiduals", err)
	}
	for name, bad := range map[string][]fixedpoint.Rate{
		"empty":      nil,
		"zero":       {fixedpoint.ZeroRate()},
		"one":        grid(t, "1"),
		"descending": grid(t, "0.75", "0.25"),
		"duplicate":  grid(t, "0.5", "0.5"),
	} {
		if _, err := calibrate.Points(residuals, bad); !errors.Is(err, calibrate.ErrGridInvalid) {
			t.Errorf("%s grid error = %v; want ErrGridInvalid", name, err)
		}
	}
}

// --- EntryDriftFrom ---

func TestEntryDriftFromTail(t *testing.T) {
	pts := []domain.QuantilePoint{
		{Q: mustRate(t, "0.05"), Shift: mustRate(t, "-0.0002")},
		{Q: mustRate(t, "0.5"), Shift: mustRate(t, "0.00001")},
	}

	// 7h interval: year/interval = 8760/7 = 1251.428571428…, which rounds
	// half-even to 1251.428571429 at scale 9; ×0.0002 → 0.250285714. A
	// hardcoded 8h (1095) or a truncating division both move these digits.
	got, err := calibrate.EntryDriftFrom(pts, mustRate(t, "0.05"), 7*time.Hour)
	if err != nil {
		t.Fatalf("EntryDriftFrom: %v", err)
	}
	if got.Decimal() != "0.250285714" {
		t.Errorf("drift = %s; want 0.250285714", got.Decimal())
	}

	// A non-negative tail shift measures no drift for a receiver to fear:
	// a DECLARED zero, not a refusal and not an invented floor.
	up := []domain.QuantilePoint{
		{Q: mustRate(t, "0.05"), Shift: mustRate(t, "0.00003")},
	}
	got, err = calibrate.EntryDriftFrom(up, mustRate(t, "0.05"), 8*time.Hour)
	if err != nil {
		t.Fatalf("EntryDriftFrom positive tail: %v", err)
	}
	if sign, err := got.Sign(); err != nil || sign != 0 {
		t.Errorf("positive-tail drift = %s (sign err %v); want declared zero", got.Decimal(), err)
	}
}

func TestEntryDriftFromRefusals(t *testing.T) {
	pts := []domain.QuantilePoint{
		{Q: mustRate(t, "0.05"), Shift: mustRate(t, "-0.0002")},
	}
	if _, err := calibrate.EntryDriftFrom(nil, mustRate(t, "0.05"), 8*time.Hour); !errors.Is(err, calibrate.ErrNoResiduals) {
		t.Errorf("empty points error = %v; want ErrNoResiduals", err)
	}
	if _, err := calibrate.EntryDriftFrom(pts, mustRate(t, "0.1"), 8*time.Hour); !errors.Is(err, calibrate.ErrTailOffGrid) {
		t.Errorf("off-grid tail error = %v; want ErrTailOffGrid", err)
	}
	if _, err := calibrate.EntryDriftFrom(pts, mustRate(t, "0.05"), 0); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero interval error = %v; want ErrUnset", err)
	}
}
