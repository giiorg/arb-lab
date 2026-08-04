package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// # The fixture-collapse pass for the two grid predicates
//
// The step below is 0.005 — deliberately NOT a power of ten. At a
// power-of-ten step, "is a whole multiple of step" and "has no more than k
// decimal places" are the same predicate, so an implementation that merely
// counted digits would pass every fixture. 0.013 is the value that separates
// them: three decimal places, off the 0.005 grid.
//
// The two predicates are mirrors and the suites below are kept symmetric on
// purpose (the Send/Receive lesson: a distinction earned by one twin is
// checked on the other immediately).

func TestQuantity_OnStep(t *testing.T) {
	step := mustQuantity(t, btc, "0.005")

	for _, tc := range []struct {
		qty  string
		want bool
	}{
		{"0.015", true},
		{"0.013", false}, // 3 decimals, off-grid: the digit-counting collapse
		{"0.005", true},  // exactly one lot
		{"0.003", false}, // below one lot, nonzero
		{"0", true},      // zero is a whole multiple; the planner refuses it earlier on its own ground
		{"-0.015", true}, // a short on the grid
		{"-0.013", false},
	} {
		got, err := mustQuantity(t, btc, tc.qty).OnStep(step)
		if err != nil {
			t.Errorf("OnStep(%s) error: %v", tc.qty, err)
			continue
		}
		if got != tc.want {
			t.Errorf("OnStep(%s, step 0.005) = %v; want %v", tc.qty, got, tc.want)
		}
	}

	// The harm case this predicate exists for, at the real venue values: the
	// Binance spot step admits 0.00023 BTC and the perp step refuses it.
	spotStep := mustQuantity(t, btc, "0.00001")
	perpStep := mustQuantity(t, btc, "0.001")
	qty := mustQuantity(t, btc, "0.00023")
	if on, err := qty.OnStep(spotStep); err != nil || !on {
		t.Errorf("0.00023 on the spot grid = %v, %v; want true", on, err)
	}
	if on, err := qty.OnStep(perpStep); err != nil || on {
		t.Errorf("0.00023 on the perp grid = %v, %v; want false — this asymmetry is the naked-leg case", on, err)
	}
}

func TestQuantity_OnStepRefusals(t *testing.T) {
	q := mustQuantity(t, btc, "0.015")

	if _, err := q.OnStep(fixedpoint.ZeroQuantity(btc)); !errors.Is(err, fixedpoint.ErrStepNotPositive) {
		t.Errorf("OnStep(zero step) = %v; want ErrStepNotPositive — a zero step reads as no constraint", err)
	}
	if _, err := q.OnStep(mustQuantity(t, btc, "-0.005")); !errors.Is(err, fixedpoint.ErrStepNotPositive) {
		t.Errorf("OnStep(negative step) = %v; want ErrStepNotPositive", err)
	}
	if _, err := q.OnStep(mustQuantity(t, eth, "0.005")); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("OnStep across assets = %v; want ErrAssetMismatch", err)
	}
	if _, err := q.OnStep(fixedpoint.Quantity{}); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("OnStep(zero value) = %v; want ErrAssetUnset", err)
	}
	if _, err := (fixedpoint.Quantity{}).OnStep(mustQuantity(t, btc, "0.005")); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("zero value OnStep = %v; want ErrAssetUnset", err)
	}
}

func TestPrice_OnTick(t *testing.T) {
	tick := mustPrice(t, btc, usdt, "0.25") // not a power of ten, OnStep's reason

	for _, tc := range []struct {
		px   string
		want bool
	}{
		{"50000.25", true},
		{"50000.13", false}, // two decimals, off the 0.25 grid: the digit-counting collapse
		{"0.25", true},
		{"0.13", false},
		{"0", true},
		{"-0.25", true}, // negative prices trade (calendar spreads); the grid judges them alike
		{"-0.13", false},
	} {
		got, err := mustPrice(t, btc, usdt, tc.px).OnTick(tick)
		if err != nil {
			t.Errorf("OnTick(%s) error: %v", tc.px, err)
			continue
		}
		if got != tc.want {
			t.Errorf("OnTick(%s, tick 0.25) = %v; want %v", tc.px, got, tc.want)
		}
	}

	// The real venue asymmetry, observed 2026-08-03: Binance spot ticks at
	// 0.01 and its perp at 0.10, so 50000.05 is a valid spot price and an
	// off-grid perp price — the price axis of the naked-leg case.
	spotTick := mustPrice(t, btc, usdt, "0.01")
	perpTick := mustPrice(t, btc, usdt, "0.1")
	px := mustPrice(t, btc, usdt, "50000.05")
	if on, err := px.OnTick(spotTick); err != nil || !on {
		t.Errorf("50000.05 on the spot tick = %v, %v; want true", on, err)
	}
	if on, err := px.OnTick(perpTick); err != nil || on {
		t.Errorf("50000.05 on the perp tick = %v, %v; want false", on, err)
	}
}

func TestPrice_OnTickRefusals(t *testing.T) {
	px := mustPrice(t, btc, usdt, "50000.25")

	if _, err := px.OnTick(mustPrice(t, btc, usdt, "0")); !errors.Is(err, fixedpoint.ErrStepNotPositive) {
		t.Errorf("OnTick(zero tick) = %v; want ErrStepNotPositive", err)
	}
	if _, err := px.OnTick(mustPrice(t, btc, usdt, "-0.25")); !errors.Is(err, fixedpoint.ErrStepNotPositive) {
		t.Errorf("OnTick(negative tick) = %v; want ErrStepNotPositive", err)
	}
	if _, err := px.OnTick(mustPrice(t, eth, usdt, "0.25")); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("OnTick across bases = %v; want ErrAssetMismatch", err)
	}
	if _, err := px.OnTick(mustPrice(t, btc, eth, "0.25")); !errors.Is(err, fixedpoint.ErrAssetMismatch) {
		t.Errorf("OnTick across quotes = %v; want ErrAssetMismatch", err)
	}
	if _, err := px.OnTick(fixedpoint.Price{}); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("OnTick(zero value) = %v; want ErrAssetUnset", err)
	}
	if _, err := (fixedpoint.Price{}).OnTick(mustPrice(t, btc, usdt, "0.25")); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Errorf("zero value OnTick = %v; want ErrAssetUnset", err)
	}
}
