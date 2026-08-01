package crossvenue_test

// Cost-selection fixtures, checklist-first (§45g): the assembly fixture
// makes the two venues DISAGREE about which side is worse — Binance's
// bids, Bybit's asks — so a "pick one side globally" implementation
// cannot reproduce the total. Zero cells sit exactly ON the negativity
// boundary and must pass; negatives must refuse.

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func TestWorseOf_PicksTheLarger(t *testing.T) {
	for _, tc := range []struct{ walked, p90, want string }{
		{"0.0003", "0.0001", "0.0003"},
		{"0.0001", "0.0003", "0.0003"},
		{"0.0002", "0.0002", "0.0002"},
		// A negative sample survives selection — the book improved, and
		// clamping is nobody's job here.
		{"-0.0002", "-0.0004", "-0.0002"},
	} {
		got, err := crossvenue.WorseOf(rate(t, tc.walked), rate(t, tc.p90))
		if err != nil {
			t.Fatalf("WorseOf(%s,%s): %v", tc.walked, tc.p90, err)
		}
		wantRate(t, "worse-of", got, tc.want)
	}
	if _, err := crossvenue.WorseOf(rate(t, "0.0001"), crossvenue.Settlement{}.Rate); err == nil {
		t.Fatal("unset p90 accepted")
	}
}

func TestWorseSide_PicksThePerVenueWorse(t *testing.T) {
	got, err := crossvenue.WorseSide(rate(t, "0.0005"), rate(t, "0.0003"))
	if err != nil {
		t.Fatalf("WorseSide: %v", err)
	}
	wantRate(t, "bid worse", got, "0.0005")
	got, err = crossvenue.WorseSide(rate(t, "0.0002"), rate(t, "0.0007"))
	if err != nil {
		t.Fatalf("WorseSide: %v", err)
	}
	wantRate(t, "ask worse", got, "0.0007")
}

func TestRoundTrip_VenuesDisagreeAboutTheWorseSide(t *testing.T) {
	// Binance: bids 5bp, asks 3bp → worse side 5bp. Bybit: bids 2bp, asks
	// 7bp → worse side 7bp. Fees 5bp / 5.5bp. Total = 2×(5+5) + 2×(5.5+7)
	// = 45bp. A global-bid implementation says 2×(5+5)+2×(5.5+2)=35bp; a
	// global-ask one says 2×(5+3)+2×(5.5+7)=41bp.
	bnSlip, err := crossvenue.WorseSide(rate(t, "0.0005"), rate(t, "0.0003"))
	if err != nil {
		t.Fatalf("binance side: %v", err)
	}
	bySlip, err := crossvenue.WorseSide(rate(t, "0.0002"), rate(t, "0.0007"))
	if err != nil {
		t.Fatalf("bybit side: %v", err)
	}
	total, err := crossvenue.RoundTrip(rate(t, "0.0005"), bnSlip, rate(t, "0.00055"), bySlip)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	wantRate(t, "round trip", total, "0.0045")
}

func TestRoundTrip_ZeroCellsPass(t *testing.T) {
	// Zero sits ON the boundary: a zero-fee tier or a zero slippage bound
	// is a legal measured cell, and only NEGATIVE refuses.
	total, err := crossvenue.RoundTrip(rate(t, "0"), rate(t, "0"), rate(t, "0"), rate(t, "0"))
	if err != nil {
		t.Fatalf("all-zero round trip refused: %v", err)
	}
	wantRate(t, "round trip", total, "0")
}

func TestRoundTrip_NegativeCellRefused(t *testing.T) {
	// One mantissa unit below zero, rotated through every cell position.
	for i := range 4 {
		args := [4]string{"0.0005", "0.0005", "0.0005", "0.0005"}
		args[i] = "-0.000000001"
		_, err := crossvenue.RoundTrip(rate(t, args[0]), rate(t, args[1]), rate(t, args[2]), rate(t, args[3]))
		if !errors.Is(err, crossvenue.ErrCostNegative) {
			t.Fatalf("negative cell %d: %v, want ErrCostNegative", i, err)
		}
	}
}
