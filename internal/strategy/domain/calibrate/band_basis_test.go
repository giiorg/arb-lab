package calibrate_test

// The commit-2 battery: excursion band and adverse basis move.
//
// The band fixture is built so every rule has a row whose misreading
// MOVES the pinned answer — the §41i suspects plus the boundary cases:
//
//   - one settlement sits exactly AT the clearing level: counted as a dip
//     it would fuse two short recoveries into one dwell-length failure,
//     changing the depth set and the answer;
//   - one run is exactly persist long (NOT recovered — the dwell fires at
//     the persist-th settlement), one shorter, several shorter still;
//     both branches have instances, per the pre-named suspect;
//   - the final run is TRUNCATED by the era's end: including it changes
//     the answer;
//   - the quantile rank is inexact, so ceil and floor disagree.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

// bandSeries builds the pinned excursion fixture on an 8h grid; clearing
// is 0.0003, persist 3.
func bandSeries(t *testing.T) []domain.RealizedSettlement {
	t.Helper()
	rates := []string{
		"0.0005",  // 0 above
		"0.0002",  // 1 dip A
		"0.0003",  // 2 AT the level: ends A (len 1 → recovered, depth 0.0001)
		"0.00005", // 3 dip B
		"0.0004",  // 4 ends B (len 1 → recovered, depth 0.00025)
		"0.0001",  // 5 dip C
		"0.0002",  // 6 dip C
		"0.0001",  // 7 dip C
		"0.0005",  // 8 ends C (len 3 == persist → the dwell fired, NOT recovered)
		"0.00015", // 9 dip D
		"0.00025", // 10 dip D
		"0.0006",  // 11 ends D (len 2 → recovered, depth 0.00015)
		"0.0002",  // 12 dip E, truncated by the era's end → excluded
	}
	out := make([]domain.RealizedSettlement, len(rates))
	for i, r := range rates {
		out[i] = settled(t, calEra, time.Duration(i)*8*time.Hour, r)
	}
	return out
}

func TestExitBandWidthPinned(t *testing.T) {
	series := bandSeries(t)
	// Recovered depths sorted: [0.0001, 0.00015, 0.00025]. q=0.6 → rank
	// 1.2 → ceil → 0.00025; annualized at 8h (×1095) → 0.27375. A floor
	// selection lands 0.164250 instead; counting the at-level row as a
	// dip removes two depths; crediting the persist-length run adds one;
	// including the truncated run adds another — each moves this digit
	// string.
	got, err := calibrate.ExitBandWidth(series, mustRate(t, "0.0003"), 3, mustRate(t, "0.6"), 8*time.Hour)
	if err != nil {
		t.Fatalf("ExitBandWidth: %v", err)
	}
	if got.Decimal() != "0.27375" {
		t.Errorf("band width = %s; want 0.27375", got.Decimal())
	}

	// A tighter dwell turns run D (len 2) into a failure too: only the
	// two len-1 recoveries remain. q=0.6 → rank 0.6 → ceil → 0.00025.
	// The persist argument must reach the classification, not just
	// validation.
	got, err = calibrate.ExitBandWidth(series, mustRate(t, "0.0003"), 2, mustRate(t, "0.3"), 8*time.Hour)
	if err != nil {
		t.Fatalf("ExitBandWidth persist=2: %v", err)
	}
	// Depths [0.0001, 0.00025]; rank 0.3 → ceil → idx 1.
	if got.Decimal() != "0.27375" {
		t.Errorf("persist=2 width = %s; want 0.27375", got.Decimal())
	}
}

func TestExitBandWidthRefusals(t *testing.T) {
	series := bandSeries(t)
	clearing := mustRate(t, "0.0003")
	q := mustRate(t, "0.6")

	if _, err := calibrate.ExitBandWidth(nil, clearing, 3, q, 8*time.Hour); !errors.Is(err, domain.ErrWindowEmpty) {
		t.Errorf("empty history error = %v; want ErrWindowEmpty", err)
	}
	if _, err := calibrate.ExitBandWidth(series, fixedpoint.Rate{}, 3, q, 8*time.Hour); err == nil {
		t.Error("unset clearing accepted")
	}
	if _, err := calibrate.ExitBandWidth(series, clearing, 0, q, 8*time.Hour); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero persist error = %v; want ErrUnset", err)
	}
	if _, err := calibrate.ExitBandWidth(series, clearing, 3, mustRate(t, "1"), 8*time.Hour); !errors.Is(err, calibrate.ErrGridInvalid) {
		t.Errorf("quantile 1 error = %v; want ErrGridInvalid", err)
	}
	if _, err := calibrate.ExitBandWidth(series, clearing, 3, q, 0); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero interval error = %v; want ErrUnset", err)
	}

	// An era whose only dips ran the dwell out measures nothing: with
	// persist 1 no run is ever shorter than the dwell, and the refusal
	// names the state rather than inventing a width.
	if _, err := calibrate.ExitBandWidth(series, clearing, 1, q, 8*time.Hour); !errors.Is(err, calibrate.ErrNoExcursions) {
		t.Errorf("no-recovery error = %v; want ErrNoExcursions", err)
	}
}

// --- AdverseBasisMove ---

func kline(t *testing.T, base time.Time, offset time.Duration, price string) calibrate.KlinePoint {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", price)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", price, err)
	}
	return calibrate.KlinePoint{At: base.Add(offset), Price: p}
}

func klineSeries(t *testing.T, step time.Duration, prices ...string) []calibrate.KlinePoint {
	t.Helper()
	out := make([]calibrate.KlinePoint, len(prices))
	for i, p := range prices {
		out[i] = kline(t, calEra, time.Duration(i)*step, p)
	}
	return out
}

func TestAdverseBasisMovePinned(t *testing.T) {
	// basis: 0.5, 0.2, 1.0, 1.5. Moves over 2h (s=2): (1.0−0.5)/100 =
	// 0.005 and (1.5−0.2)/101 = 0.012871287 (half-even at scale 9 — the
	// repeating 1287 tail pins the division's rounding). q=0.5 → rank
	// 0.5 → ceil → the larger move; a floor selection lands 0.005.
	spot := klineSeries(t, time.Hour, "100", "101", "103", "102")
	mark := klineSeries(t, time.Hour, "100.5", "101.2", "104", "103.5")

	got, err := calibrate.AdverseBasisMove(spot, mark, 2*time.Hour, mustRate(t, "0.5"))
	if err != nil {
		t.Fatalf("AdverseBasisMove: %v", err)
	}
	if got.Decimal() != "0.012871287" {
		t.Errorf("adverse basis move = %s; want 0.012871287", got.Decimal())
	}
}

func TestAdverseBasisMoveConvergingEraIsDeclaredZero(t *testing.T) {
	// The basis falls monotonically: every move is negative, so even the
	// upper tail converged — a DECLARED zero, mirroring the conservative
	// model's refusal to credit basis.
	spot := klineSeries(t, time.Hour, "100", "100", "100")
	mark := klineSeries(t, time.Hour, "103", "102", "101")

	got, err := calibrate.AdverseBasisMove(spot, mark, time.Hour, mustRate(t, "0.9"))
	if err != nil {
		t.Fatalf("AdverseBasisMove: %v", err)
	}
	if sign, err := got.Sign(); err != nil || sign != 0 {
		t.Errorf("converging-era move = %s (sign err %v); want declared zero", got.Decimal(), err)
	}
}

func TestAdverseBasisMoveRefusals(t *testing.T) {
	spot := klineSeries(t, time.Hour, "100", "101", "103", "102")
	mark := klineSeries(t, time.Hour, "100.5", "101.2", "104", "103.5")
	q := mustRate(t, "0.5")

	if _, err := calibrate.AdverseBasisMove(spot[:1], mark[:1], time.Hour, q); !errors.Is(err, domain.ErrWindowEmpty) {
		t.Errorf("one-row error = %v; want ErrWindowEmpty", err)
	}
	if _, err := calibrate.AdverseBasisMove(spot, mark[:3], time.Hour, q); !errors.Is(err, calibrate.ErrSeriesMismatch) {
		t.Errorf("length mismatch error = %v; want ErrSeriesMismatch", err)
	}
	if _, err := calibrate.AdverseBasisMove(spot, mark, 90*time.Minute, q); !errors.Is(err, calibrate.ErrHorizonOffGrid) {
		t.Errorf("off-grid horizon error = %v; want ErrHorizonOffGrid", err)
	}
	if _, err := calibrate.AdverseBasisMove(spot, mark, 0, q); !errors.Is(err, calibrate.ErrHorizonOffGrid) {
		t.Errorf("zero horizon error = %v; want ErrHorizonOffGrid", err)
	}
	if _, err := calibrate.AdverseBasisMove(spot, mark, 4*time.Hour, q); !errors.Is(err, calibrate.ErrHistoryShort) {
		t.Errorf("horizon-spans-series error = %v; want ErrHistoryShort", err)
	}
	if _, err := calibrate.AdverseBasisMove(spot, mark, time.Hour, mustRate(t, "1")); !errors.Is(err, calibrate.ErrGridInvalid) {
		t.Errorf("quantile 1 error = %v; want ErrGridInvalid", err)
	}

	// One misaligned mark instant.
	skewed := append([]calibrate.KlinePoint(nil), mark...)
	skewed[2].At = skewed[2].At.Add(time.Minute)
	if _, err := calibrate.AdverseBasisMove(spot, skewed, time.Hour, q); !errors.Is(err, calibrate.ErrSeriesMismatch) {
		t.Errorf("misaligned instant error = %v; want ErrSeriesMismatch", err)
	}

	// A hole in the spot grid: uniform-step enforcement.
	gappy := append([]calibrate.KlinePoint(nil), spot...)
	gappy[2].At = gappy[2].At.Add(30 * time.Minute)
	skewed2 := append([]calibrate.KlinePoint(nil), mark...)
	skewed2[2].At = gappy[2].At
	if _, err := calibrate.AdverseBasisMove(gappy, skewed2, time.Hour, q); !errors.Is(err, calibrate.ErrSeriesMismatch) {
		t.Errorf("non-uniform step error = %v; want ErrSeriesMismatch", err)
	}

	// A foreign asset pair mid-series.
	foreign := append([]calibrate.KlinePoint(nil), mark...)
	fp, err := fixedpoint.ParsePrice("ETH", "USDT", "104")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	foreign[2].Price = fp
	if _, err := calibrate.AdverseBasisMove(spot, foreign, time.Hour, q); !errors.Is(err, calibrate.ErrSeriesMismatch) {
		t.Errorf("foreign asset error = %v; want ErrSeriesMismatch", err)
	}
}
