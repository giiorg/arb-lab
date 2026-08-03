package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// interval builds [lo, hi] the way a real hedge group produces one: a residual
// of lo, plus one pending BUY of the width.
//
// There is no exported constructor taking both endpoints, on purpose — an
// interval is DERIVED, from an order's side and remainder or from shifting a sum
// by a residual — so a fixture that wants [a, b] has to state which arrangement
// of orders and realized delta produces it. That constraint is worth keeping:
// it is what stops a test asserting over an interval no group could reach.
func interval(t *testing.T, lo, hi string) domain.PossibleDelta {
	t.Helper()
	width, err := qty(t, hi).Sub(qty(t, lo))
	if err != nil {
		t.Fatalf("width of [%s, %s]: %v", lo, hi, err)
	}
	pending, err := domain.PossibleDeltaFor(domain.SideBuy, width)
	if err != nil {
		t.Fatalf("PossibleDeltaFor(buy, %s): %v", width, err)
	}
	shifted, err := pending.Shift(qty(t, lo))
	if err != nil {
		t.Fatalf("Shift by %s: %v", lo, err)
	}
	return shifted
}

// The single-order intervals, in BOTH directions.
//
// The mirror rule, applied before the mutations rather than after: a buy and a
// sell are the same code path with one sign flipped, and this repo has twice
// paid for testing one direction of a symmetric pair (bybitExecutionSide's
// every-fixture-sells survivor, one slice ago). The spot leg of every
// cash-and-carry BUYS and the perp leg SELLS, so both directions are the
// ordinary case here rather than a corner.
func TestPossibleDeltaForSignsByOrderSide(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		side      domain.Side
		remaining string
		wantLo    string
		wantHi    string
	}{
		// The quantities differ from each other so that "returns lo" and
		// "returns hi" are distinguishable answers. A fixture using the same
		// number for both endpoints of the pair would pass against a
		// transposition.
		{"a buy can only raise the delta", domain.SideBuy, "1.5", "0", "1.5"},
		{"a sell can only lower it", domain.SideSell, "2.25", "-2.25", "0"},

		// Zero remaining is the boundary between "contributes an interval" and
		// "contributes the point zero", and it is reached by an ordinary fully
		// filled order rather than by a degenerate one.
		{"a fully executed buy contributes the point zero", domain.SideBuy, "0", "0", "0"},
		{"a fully executed sell contributes the point zero", domain.SideSell, "0", "0", "0"},

		// The over-execution clamp. A negative remainder is real — Ledger's
		// Remaining reports one when a venue over-fills — and it must NOT invert
		// the interval. Both directions, because the inversion is direction
		// dependent: unclamped, an over-filled buy would gain a short endpoint
		// and an over-filled sell a long one.
		{"an over-executed buy is clamped, not inverted", domain.SideBuy, "-0.4", "0", "0"},
		{"an over-executed sell is clamped, not inverted", domain.SideSell, "-0.4", "0", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.PossibleDeltaFor(tc.side, qty(t, tc.remaining))
			if err != nil {
				t.Fatalf("PossibleDeltaFor: %v", err)
			}
			assertEndpoints(t, got, tc.wantLo, tc.wantHi)
		})
	}
}

func TestPossibleDeltaForRefusesAnUndeclaredSide(t *testing.T) {
	t.Parallel()

	if _, err := domain.PossibleDeltaFor(domain.Side("short"), qty(t, "1")); !errors.Is(err, domain.ErrSideInvalid) {
		t.Fatalf("PossibleDeltaFor(short) = %v, want ErrSideInvalid", err)
	}
}

// Summation is endpoint-wise, and the fixture is a real hedge group.
//
// Three orders rather than two, and NOT all the same size. With two, "sums the
// endpoints" and "takes the wider of the two" agree whenever one interval
// contains the other; with equal sizes, a cash-and-carry's own legs cancel to
// [-1, 1] and every arithmetic that respects sign passes.
func TestPossibleDeltaAddSumsEndpointsAcrossLegs(t *testing.T) {
	t.Parallel()

	spotBuy, err := domain.PossibleDeltaFor(domain.SideBuy, qty(t, "1"))
	if err != nil {
		t.Fatalf("PossibleDeltaFor: %v", err)
	}
	perpSell, err := domain.PossibleDeltaFor(domain.SideSell, qty(t, "0.75"))
	if err != nil {
		t.Fatalf("PossibleDeltaFor: %v", err)
	}
	secondBuy, err := domain.PossibleDeltaFor(domain.SideBuy, qty(t, "0.25"))
	if err != nil {
		t.Fatalf("PossibleDeltaFor: %v", err)
	}

	sum, err := spotBuy.Add(perpSell)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	sum, err = sum.Add(secondBuy)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// 1 + 0.25 of possible buying, 0.75 of possible selling.
	assertEndpoints(t, sum, "-0.75", "1.25")
}

func TestPossibleDeltaShiftMovesBothEndpoints(t *testing.T) {
	t.Parallel()

	// A nonzero residual with an ASYMMETRIC interval: shifting [-1, 2] by 0.5
	// gives [-0.5, 2.5], where no pair of endpoints coincides and no endpoint
	// is zero. Shifted by a residual of zero — the tempting fixture — every
	// wrong implementation that returns the interval unchanged would pass.
	rng, err := interval(t, "-1", "2").Shift(qty(t, "0.5"))
	if err != nil {
		t.Fatalf("Shift: %v", err)
	}
	assertEndpoints(t, rng, "-0.5", "2.5")
}

// The straddle predicate, at and around both boundaries.
//
// This is the `<` versus `<=` collapse, which this repo has recorded twice
// (ratelimit's freeAt, and the ceiling before it). Every case here sits exactly
// ON a boundary or exactly one step off it, because a fixture strictly inside
// or strictly outside makes the two predicates the same predicate.
func TestPossibleDeltaStraddlesRequiresZeroInTheInterior(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lo   string
		hi   string
		want bool
	}{
		{"zero strictly inside", "-1", "2", true},
		{"the low endpoint IS zero", "0", "2", false},
		{"the high endpoint IS zero", "-1", "0", false},
		{"fully collapsed at zero", "0", "0", false},
		{"guaranteed long, not touching zero", "0.5", "2", false},
		{"guaranteed short, not touching zero", "-2", "-0.5", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := straddles(t, tc.lo, tc.hi)
			if err != nil {
				t.Fatalf("Straddles: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Straddles([%s, %s]) = %t, want %t", tc.lo, tc.hi, got, tc.want)
			}
		})
	}
}

// The floor magnitude is the endpoint NEAREST zero, and the case that says so is
// model (a)'s NC1.
//
// Every row states the expected value POSITIVELY rather than asserting that it
// differs from the worst endpoint: "not the worst one" is satisfied by zero, by
// the midpoint, and by every wrong answer below it — the assertion-shaped gap
// slice D found twice.
func TestPossibleDeltaFloorReductionTakesTheNearestEndpoint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		lo     string
		hi     string
		want   string
		reason string
	}{
		{
			name: "guaranteed net long reduces by the near endpoint",
			lo:   "0.4", hi: "1", want: "-0.4",
			reason: "0.4 is naked under every resolution; 1 is not",
		},
		{
			name: "guaranteed net short buys back the near endpoint",
			lo:   "-1", hi: "-0.4", want: "0.4",
			reason: "the sign mirror of the row above — |a| = 1 is the WORST endpoint here, and R1d-2 is the correction that says so",
		},
		{
			name: "a near endpoint of zero yields no change",
			lo:   "0", hi: "1", want: "0",
			reason: "NC1: a residual of +1 with a pending sell of 1. The pending hedge may already cover the whole position, so nothing is naked under every resolution — worst-endpoint sizing would sell 1 and reach -1",
		},
		{
			name: "a fully collapsed flat group yields no change",
			lo:   "0", hi: "0", want: "0",
			reason: "the degenerate case, which must not be an error: nothing to do is a legal answer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := interval(t, tc.lo, tc.hi).FloorReduction()
			if err != nil {
				t.Fatalf("FloorReduction: %v (%s)", err, tc.reason)
			}
			if got.Decimal() != qty(t, tc.want).Decimal() {
				t.Fatalf("FloorReduction([%s, %s]) = %s, want %s (%s)",
					tc.lo, tc.hi, got, tc.want, tc.reason)
			}
		})
	}
}

func TestPossibleDeltaFloorReductionRefusesAStraddlingInterval(t *testing.T) {
	t.Parallel()

	_, err := interval(t, "-1", "2").FloorReduction()
	if !errors.Is(err, domain.ErrIntervalStraddlesZero) {
		t.Fatalf("FloorReduction = %v, want ErrIntervalStraddlesZero", err)
	}
}

// IsPoint is not "the residual is zero", and this is the fixture that says so.
//
// A group certainly short by exactly 1 is a POINT — determined — and it is
// precisely the case the compensating branch exists for. A test whose only
// collapsed fixture were [0, 0] could not tell IsPoint from "both endpoints are
// zero".
func TestPossibleDeltaIsPointIsNotIsZero(t *testing.T) {
	t.Parallel()

	collapsedAtMinusOne, err := domain.NoPossibleDelta("BTC").Shift(qty(t, "-1"))
	if err != nil {
		t.Fatalf("Shift: %v", err)
	}
	point, err := collapsedAtMinusOne.IsPoint()
	if err != nil {
		t.Fatalf("IsPoint: %v", err)
	}
	if !point {
		t.Fatalf("IsPoint(%s) = false, want true", collapsedAtMinusOne)
	}

	open, err := interval(t, "-1", "2").IsPoint()
	if err != nil {
		t.Fatalf("IsPoint: %v", err)
	}
	if open {
		t.Fatal("IsPoint([-1, 2]) = true, want false")
	}
}

func TestPossibleDeltaValidateRefusesTheZeroValue(t *testing.T) {
	t.Parallel()

	var unpopulated domain.PossibleDelta
	if err := unpopulated.Validate(); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Fatalf("Validate = %v, want ErrAssetUnset", err)
	}
}

func assertEndpoints(t *testing.T, p domain.PossibleDelta, wantLo, wantHi string) {
	t.Helper()
	if got := p.Lo().Decimal(); got != qty(t, wantLo).Decimal() {
		t.Fatalf("low endpoint = %s, want %s (interval %s)", got, wantLo, p)
	}
	if got := p.Hi().Decimal(); got != qty(t, wantHi).Decimal() {
		t.Fatalf("high endpoint = %s, want %s (interval %s)", got, wantHi, p)
	}
}

func straddles(t *testing.T, lo, hi string) (bool, error) {
	t.Helper()
	return interval(t, lo, hi).Straddles()
}
