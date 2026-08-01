package cashcarry

// Helper-level pins for every rounding site in the ledger. The battery's
// fixture products are mostly exact at scale 9 (real prices × round
// quantities — §40m's thirteenth checklist instance was exactly this), so
// each direction claim is pinned here on a manufactured sub-ulp product
// where the modes actually disagree.

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func money(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

var t0 = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

func TestHoldFractionPins(t *testing.T) {
	// §3.9's twenty days: 20/365 = 0.0547945205479… — the tenth digit
	// rounds the ninth up, so a floor mutant answers …520.
	f, err := holdFraction(t0, t0.Add(20*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Decimal(); got != "0.054794521" {
		t.Fatalf("20d/365d = %s", got)
	}
	// A span engineered to quotient exactly 2.5e-9 (78.84 ms against the
	// 365-day year): half-even keeps the even neighbour, a half-up mutant
	// answers 3e-9.
	f, err = holdFraction(t0, t0.Add(78840000*time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Decimal(); got != "0.000000002" {
		t.Fatalf("tie quotient = %s, want the even neighbour", got)
	}
	// Zero and negative spans are refusals: an entry against a terminal at
	// or before the evaluation instant prices a hold that has no future.
	if _, err := holdFraction(t0, t0); err == nil {
		t.Fatal("zero span accepted")
	}
	if _, err := holdFraction(t0, t0.Add(-time.Hour)); err == nil {
		t.Fatal("negative span accepted")
	}
}

func TestAnnualizedPins(t *testing.T) {
	// A 7-hour interval: 8760/7 is infinite in decimal, so both the
	// division and the multiply engage their modes.
	got, err := Annualized(rate(t, "0.0001"), 7*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.125142857" {
		t.Fatalf("annualized = %s", got.Decimal())
	}
	// The §3.9 interval is exact: 0.00015 × 1095.
	got, err = Annualized(rate(t, "0.00015"), 8*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.164250" && got.Decimal() != "0.16425" {
		t.Fatalf("annualized 8h = %s", got.Decimal())
	}
}

func TestEffectiveHurdleNetsAllThreeAdjustments(t *testing.T) {
	got, err := EffectiveHurdle(domain.HurdleBenchmark{
		AnnualRate:          rate(t, "0.046"),
		AccessCost:          rate(t, "0.001"),
		LiquidityHaircut:    rate(t, "0.002"),
		CounterpartyHaircut: rate(t, "0.003"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.04" {
		t.Fatalf("effective hurdle = %s", got.Decimal())
	}
}

func TestScaledThresholdRoundsAwayTwice(t *testing.T) {
	// 1.000000001 × 0.000000003 leaves a 3e-18 tail: away-from-zero lifts
	// the ninth digit, a half-even or floor mutant does not.
	got, err := scaledThreshold(money(t, "1.000000001"), rate(t, "0.000000003"), rate(t, "0.5"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000002" {
		t.Fatalf("threshold = %s", got.Decimal())
	}
	// The tail survives the first multiply only under away: floor there
	// yields 3e-9 → ×0.5 → 1.5e-9 → away 2e-9 as well, so pin the first
	// multiply alone through a fraction of 1.
	got, err = scaledThreshold(money(t, "1.000000001"), rate(t, "0.000000003"), rate(t, "1"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000004" {
		t.Fatalf("first multiply = %s, want the away-rounded tail", got.Decimal())
	}
	// And the second multiply alone: 4e-9 × 0.3 = 1.2e-9, away → 2e-9. A
	// floor mutant on the SECOND site answers 1e-9 while both pins above
	// stay green (their second products are exact).
	got, err = scaledThreshold(money(t, "1.000000001"), rate(t, "0.000000003"), rate(t, "0.3"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000002" {
		t.Fatalf("second multiply = %s, want the away-rounded tail", got.Decimal())
	}
}

func TestFundingMoneyUnderstatesIncomeAndOverstatesCharge(t *testing.T) {
	bid, err := fixedpoint.ParsePrice("BTC", "USDT", "0.999999999")
	if err != nil {
		t.Fatal(err)
	}
	qty, err := fixedpoint.ParseQuantity("BTC", "0.111111111")
	if err != nil {
		t.Fatal(err)
	}
	// Positive funding: notional floors to 0.111111110 (toward zero), the
	// product 1.666…e-9 floors to 1e-9 — a half-even mutant answers 2e-9.
	got, err := fundingMoney(rate(t, "0.000000015"), bid, qty)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000001" {
		t.Fatalf("income = %s", got.Decimal())
	}
	// Negative funding: RoundFloor makes the charge MORE negative — a
	// toward-zero mutant absorbs it to zero, flattering the carry.
	got, err = fundingMoney(rate(t, "-0.000000001"), bid, qty)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "-0.000000001" {
		t.Fatalf("charge = %s", got.Decimal())
	}

	// The NOTIONAL's own mode, isolated. The pins above cannot see it: the
	// two modes differ by one notional ulp, the product's floor absorbs
	// anything under rate × 1e-9, and the first battery run proved it — an
	// away-mutant SURVIVED. Here the product lands exactly astride a floor
	// boundary: 0.999999999 × 4e-9 = 3.999999996e-9 → toward zero 3e-9,
	// away 4e-9; × 0.5 → 1.5e-9 (floors to 1e-9) vs 2e-9 exactly.
	tiny, err := fixedpoint.ParseQuantity("BTC", "0.000000004")
	if err != nil {
		t.Fatal(err)
	}
	got, err = fundingMoney(rate(t, "0.5"), bid, tiny)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000001" {
		t.Fatalf("boundary income = %s, want the toward-zero notional's floor", got.Decimal())
	}
}

func TestFeeCellOverstates(t *testing.T) {
	ask, err := fixedpoint.ParsePrice("BTC", "USDT", "0.999999999")
	if err != nil {
		t.Fatal(err)
	}
	qty, err := fixedpoint.ParseQuantity("BTC", "0.111111111")
	if err != nil {
		t.Fatal(err)
	}
	// Notional rounds away to 0.111111111; the 1.444…e-9 product rounds
	// away to 2e-9. A floor mutant answers 1e-9.
	got, err := feeCell(rate(t, "0.000000013"), ask, qty)
	if err != nil {
		t.Fatal(err)
	}
	if got.Decimal() != "0.000000002" {
		t.Fatalf("fee = %s", got.Decimal())
	}
}

func TestAdvanceDwellRules(t *testing.T) {
	prior := domain.DwellState{Declared: true, Count: 2, LastRealized: t0}
	fresh := t0.Add(8 * time.Hour)

	// Above the band: reset, regardless of new settlements.
	got := advanceDwell(prior, false, fresh)
	if got != (domain.DwellState{Declared: true}) {
		t.Fatalf("reset = %+v", got)
	}
	// Below with an uncounted settlement: advance by one, to that instant.
	got = advanceDwell(prior, true, fresh)
	if got.Count != 3 || !got.LastRealized.Equal(fresh) {
		t.Fatalf("advance = %+v", got)
	}
	// Below with no new settlement: unchanged — settlements, never calls.
	if got := advanceDwell(prior, true, t0); got != prior {
		t.Fatalf("same-settlement call moved the dwell: %+v", got)
	}
	if got := advanceDwell(prior, true, time.Time{}); got != prior {
		t.Fatalf("empty window moved the dwell: %+v", got)
	}
	// First advance from a fresh reset records the instant.
	got = advanceDwell(domain.DwellState{Declared: true}, true, fresh)
	if got.Count != 1 || !got.LastRealized.Equal(fresh) {
		t.Fatalf("first advance = %+v", got)
	}
}

func TestNewestRealizedRespectsTheInstant(t *testing.T) {
	window := []domain.RealizedSettlement{
		{At: t0},
		{At: t0.Add(8 * time.Hour)},
		{At: t0.Add(16 * time.Hour)},
	}
	// Only entries at or before the evaluation instant count; the newest
	// eligible wins, and an entry exactly AT the instant is realized.
	if got := newestRealized(window, t0.Add(9*time.Hour)); !got.Equal(t0.Add(8 * time.Hour)) {
		t.Fatalf("newest = %v", got)
	}
	if got := newestRealized(window, t0.Add(16*time.Hour)); !got.Equal(t0.Add(16 * time.Hour)) {
		t.Fatalf("at-instant = %v", got)
	}
	if got := newestRealized(window, t0.Add(-time.Hour)); !got.IsZero() {
		t.Fatalf("pre-window = %v", got)
	}
	if got := newestRealized(nil, t0); !got.IsZero() {
		t.Fatalf("empty window = %v", got)
	}
}
