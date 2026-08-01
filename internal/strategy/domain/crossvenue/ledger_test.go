package crossvenue_test

// Phase-ledger fixtures, checklist-first (§45g): the phase term is pinned
// VISIBLE in both directions — a Binance-first print pushes PhaseMax up, a
// Bybit-first one pushes PhaseMin down — and the aligned-grid fixture
// proves the collapse to zero is exact, not approximate. The anchor
// fixture makes the last count-balanced value NON-zero, because against a
// zero anchor "deviation from the anchor" and "the running net itself"
// are the same number. The window-boundary fixtures sit exactly ON OpenAt
// and CloseAt.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func window(sign int, open, close time.Time) crossvenue.Window {
	return crossvenue.Window{OpenAt: open, CloseAt: close, Sign: sign, Points: 1}
}

func TestWindowAccrual_AlignedGridPhaseIsExactlyZero(t *testing.T) {
	// Both legs settle at the same instants: the phase term must be
	// exactly zero even though the running net moves — §45g's collapse
	// suspect, pinned from the collapse side.
	w := window(1, cvBase, cvBase.Add(12*time.Hour))
	bn := []crossvenue.Settlement{
		settle(t, cvBase.Add(4*time.Hour), "0.0005"),
		settle(t, cvBase.Add(8*time.Hour), "-0.0002"),
	}
	by := []crossvenue.Settlement{
		settle(t, cvBase.Add(4*time.Hour), "0.0001"),
		settle(t, cvBase.Add(8*time.Hour), "0.0003"),
	}
	led, err := crossvenue.WindowAccrual(w, bn, by)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led.Net, "-0.0001") // (5-1) + (-2-3) bp
	wantRate(t, "phase max", led.PhaseMax, "0")
	wantRate(t, "phase min", led.PhaseMin, "0")
	if led.EndImbalance != 0 {
		t.Fatalf("end imbalance %d, want 0", led.EndImbalance)
	}
	if led.BinancePrints != 2 || led.BybitPrints != 2 {
		t.Fatalf("prints %d/%d, want 2/2", led.BinancePrints, led.BybitPrints)
	}
	wantRate(t, "binance accrued", led.BinanceAccrued, "0.0003")
	wantRate(t, "bybit accrued", led.BybitAccrued, "0.0004")
}

func TestWindowAccrual_PhaseTermBothDirections(t *testing.T) {
	// Binance settles 30 minutes before Bybit at each grid instant. Under
	// sign +1 a positive Binance-first print is recognized-unhedged
	// upward; flip the print signs and the term must flip too.
	w := window(1, cvBase, cvBase.Add(12*time.Hour))
	bn := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour), "0.0005")}
	by := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour+30*time.Minute), "0.0001")}

	led, err := crossvenue.WindowAccrual(w, bn, by)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led.Net, "0.0004")
	wantRate(t, "phase max", led.PhaseMax, "0.0005") // the bn print alone, before by settles
	wantRate(t, "phase min", led.PhaseMin, "0")

	// Bybit first: the unhedged term is the SHORT bybit leg, downward.
	bn2 := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour+30*time.Minute), "0.0005")}
	by2 := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour), "0.0001")}
	led2, err := crossvenue.WindowAccrual(w, bn2, by2)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led2.Net, "0.0004")
	wantRate(t, "phase max", led2.PhaseMax, "0")
	wantRate(t, "phase min", led2.PhaseMin, "-0.0001")

	// Sign -1 mirrors the first case downward.
	wNeg := window(-1, cvBase, cvBase.Add(12*time.Hour))
	led3, err := crossvenue.WindowAccrual(wNeg, bn, by)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led3.Net, "-0.0004")
	wantRate(t, "phase max", led3.PhaseMax, "0")
	wantRate(t, "phase min", led3.PhaseMin, "-0.0005")
}

func TestWindowAccrual_PhaseAnchorIsLastBalancedNotZero(t *testing.T) {
	// A completed pair moves the running net to +0.0004 BEFORE the phase
	// window opens: the unhedged term of the following Binance-first print
	// is measured from THAT anchor, not from zero. An implementation
	// measuring from zero reports 0.0007 here.
	w := window(1, cvBase, cvBase.Add(12*time.Hour))
	bn := []crossvenue.Settlement{
		settle(t, cvBase.Add(4*time.Hour), "0.0005"),
		settle(t, cvBase.Add(8*time.Hour), "0.0003"),
	}
	by := []crossvenue.Settlement{
		settle(t, cvBase.Add(4*time.Hour), "0.0001"),
		settle(t, cvBase.Add(8*time.Hour+30*time.Minute), "0.0002"),
	}
	led, err := crossvenue.WindowAccrual(w, bn, by)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led.Net, "0.0005")
	wantRate(t, "phase max", led.PhaseMax, "0.0003")
	wantRate(t, "phase min", led.PhaseMin, "0")
}

func TestWindowAccrual_SharedInstantIsAtomic(t *testing.T) {
	// Both legs at ONE instant with UNEQUAL rates: an implementation that
	// reads the phase term between the two legs' prints at the same
	// instant would report 0.0005 up; atomic processing reports zero.
	w := window(1, cvBase, cvBase.Add(8*time.Hour))
	bn := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour), "0.0005")}
	by := []crossvenue.Settlement{settle(t, cvBase.Add(4*time.Hour), "0.0003")}
	led, err := crossvenue.WindowAccrual(w, bn, by)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	wantRate(t, "net", led.Net, "0.0002")
	wantRate(t, "phase max", led.PhaseMax, "0")
	wantRate(t, "phase min", led.PhaseMin, "0")
}

func TestWindowAccrual_BoundariesExactlyOnTheEdge(t *testing.T) {
	// A print exactly AT OpenAt is the trigger settlement — observed,
	// never collected. A print exactly AT CloseAt is the exit print — the
	// position is open when it settles, so it accrues.
	w := window(1, cvBase, cvBase.Add(8*time.Hour))
	bn := []crossvenue.Settlement{
		settle(t, cvBase, "0.9"),                     // at OpenAt: excluded
		settle(t, cvBase.Add(8*time.Hour), "0.0005"), // at CloseAt: included
	}
	led, err := crossvenue.WindowAccrual(w, bn, nil)
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	if led.BinancePrints != 1 {
		t.Fatalf("prints %d, want 1 — OpenAt excluded, CloseAt included", led.BinancePrints)
	}
	wantRate(t, "net", led.Net, "0.0005")
	if led.EndImbalance != 1 {
		t.Fatalf("end imbalance %d, want 1", led.EndImbalance)
	}
}

func TestWindowAccrual_EmptyWindowIsZero(t *testing.T) {
	// A gap-killed trigger: open == close, nothing accrues, nothing
	// refuses — the episode still exists and will be charged its trip.
	w := crossvenue.Window{OpenAt: cvBase, CloseAt: cvBase, Sign: 1}
	led, err := crossvenue.WindowAccrual(w,
		[]crossvenue.Settlement{settle(t, cvBase, "0.0005")},
		[]crossvenue.Settlement{settle(t, cvBase.Add(time.Hour), "0.0001")})
	if err != nil {
		t.Fatalf("WindowAccrual: %v", err)
	}
	if led.BinancePrints != 0 || led.BybitPrints != 0 {
		t.Fatalf("prints %d/%d, want 0/0", led.BinancePrints, led.BybitPrints)
	}
	wantRate(t, "net", led.Net, "0")
}

func TestWindowAccrual_Refusals(t *testing.T) {
	w := window(1, cvBase, cvBase.Add(8*time.Hour))
	in := cvBase.Add(4 * time.Hour)

	if _, err := crossvenue.WindowAccrual(crossvenue.Window{}, nil, nil); !errors.Is(err, crossvenue.ErrWindowInvalid) {
		t.Fatalf("zero window: %v, want ErrWindowInvalid", err)
	}

	dup := []crossvenue.Settlement{settle(t, in, "0.0001"), settle(t, in, "0.0002")}
	if _, err := crossvenue.WindowAccrual(w, dup, nil); !errors.Is(err, crossvenue.ErrDuplicateSettlement) {
		t.Fatalf("binance duplicate: %v, want ErrDuplicateSettlement", err)
	}
	if _, err := crossvenue.WindowAccrual(w, nil, dup); !errors.Is(err, crossvenue.ErrDuplicateSettlement) {
		t.Fatalf("bybit duplicate: %v, want ErrDuplicateSettlement", err)
	}

	disordered := []crossvenue.Settlement{settle(t, in.Add(time.Hour), "0.0001"), settle(t, in, "0.0002")}
	if _, err := crossvenue.WindowAccrual(w, disordered, nil); !errors.Is(err, crossvenue.ErrSeriesDisordered) {
		t.Fatalf("disordered: %v, want ErrSeriesDisordered", err)
	}

	unset := []crossvenue.Settlement{{At: in}}
	if _, err := crossvenue.WindowAccrual(w, unset, nil); err == nil {
		t.Fatal("unset rate inside the window accepted")
	}
	// The same unset rate OUTSIDE the window is not the ledger's business.
	outside := []crossvenue.Settlement{{At: cvBase.Add(20 * time.Hour)}}
	if _, err := crossvenue.WindowAccrual(w, outside, nil); err != nil {
		t.Fatalf("unset rate outside the window refused: %v", err)
	}
}
