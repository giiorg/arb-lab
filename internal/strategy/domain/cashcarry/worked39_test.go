package cashcarry_test

// The tutorial §3.9 pinned-ledger test (§40f): the Go core must reproduce
// the worked trade's figures to the digit. This is the third independent
// implementation of the flagship's kill-math — tutorial prose, the guide's
// JS calculator, and this core — and per the guide's own governing rule, if
// any assertion here ever fails, the spec and the code disagree: THAT IS A
// FINDING, NEVER A TEST TO RELAX.
//
// The five figures, as §3.9 states them: $90 funding over 60 settlements,
// $38 round trip, ~8.4 days to break even, ~5.7% annualized in-position
// (the guide pins 5.68%), ~3.4% at 60% time-in-position (guide: 3.41%) —
// BELOW the 4.6% hurdle, while the entry verdict for the hold itself is
// Enter: utilization is §2.4's viability headline, not the per-hold gate,
// and the gap between those two numbers is the whole milestone-2 story.

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

func between(t *testing.T, name string, v fixedpoint.Rate, lo, hi string) {
	t.Helper()
	if c, err := v.Cmp(mustRate(t, lo)); err != nil || c < 0 {
		t.Fatalf("%s = %s, want ≥ %s (err %v)", name, v.Decimal(), lo, err)
	}
	if c, err := v.Cmp(mustRate(t, hi)); err != nil || c >= 0 {
		t.Fatalf("%s = %s, want < %s (err %v)", name, v.Decimal(), hi, err)
	}
}

func TestWorkedTradeLedgerIsPinned(t *testing.T) {
	d, err := cashcarry.Enter(flatEntry(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictEnter {
		t.Fatalf("the worked trade does not enter: %s", d.Verdict)
	}

	// The ledger's own rows.
	assertMoney(t, "60 settlements @ 0.015%", d.Terms.FundingQuantile, "90")
	assertMoney(t, "spot entry fee", d.Terms.SpotEntryFee, "10")
	assertMoney(t, "spot exit fee", d.Terms.SpotExitFee, "10")
	assertMoney(t, "perp entry fee", d.Terms.PerpEntryFee, "5")
	assertMoney(t, "perp exit fee", d.Terms.PerpExitFee, "5")
	assertMoney(t, "slippage both ways", d.Terms.Slippage, "8")
	assertMoney(t, "no basis line", d.Terms.Basis, "0")

	// Round trip ≈ $38 (0.38% of notional), §3.9's own subtotal.
	roundTrip, err := fixedpoint.Chain(d.Terms.SpotEntryFee).
		Add(d.Terms.SpotExitFee).
		Add(d.Terms.PerpEntryFee).
		Add(d.Terms.PerpExitFee).
		Add(d.Terms.Slippage).
		Result()
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "round trip", roundTrip, "38")

	// Net +$52 over the hold; the verdict's own inequality: 52 vs the
	// scaled hurdle 42.109963088 on C_allocated = $16,706.67.
	net, err := d.Terms.FundingQuantile.Sub(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "net carry", net, "52")
	assertMoney(t, "scaled hurdle", d.Terms.ScaledHurdle, "42.109963088")

	// Break-even: cost / funding × 20 days ≈ 8.44 — §3.9's "~8.4 days".
	breakEven, err := roundTrip.Div(d.Terms.FundingQuantile, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	twenty := mustRate(t, "20")
	days, err := breakEven.Mul(twenty, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	between(t, "days to break even", days, "8.35", "8.45")

	// Annualized in-position on C_allocated: net / capital / holdFraction —
	// the guide pins 5.68%, and the C_legs denominator would wrongly show
	// ~7.1% (the exact error §2.2 forbids).
	capital, err := d.Input.Capital.Total()
	if err != nil {
		t.Fatal(err)
	}
	inPos, err := net.Div(capital, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	annualized, err := inPos.Div(d.Terms.HoldFraction, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	between(t, "in-position annualized", annualized, "0.05675", "0.05685")

	// At 60% time-in-position the bankroll nets ~3.41% — BELOW the 4.6%
	// hurdle. The §12.2 forks' own predicted numbers.
	at60, err := annualized.Mul(mustRate(t, "0.6"), fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	between(t, "at 60% utilization", at60, "0.03405", "0.03415")
	if c, err := at60.Cmp(mustRate(t, "0.046")); err != nil || c >= 0 {
		t.Fatalf("utilization-scaled %s does not sit below the hurdle (err %v)", at60.Decimal(), err)
	}

	// And the C_legs error, pinned as a NEGATIVE fixture exactly as the
	// guide pins its 7.12%: the flattering denominator answers ~7.1%.
	cLegs, err := d.Input.Capital.SpotPrincipal.Add(d.Input.Capital.PerpInitialMargin)
	if err != nil {
		t.Fatal(err)
	}
	wrongInPos, err := net.Div(cLegs, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	wrongAnnualized, err := wrongInPos.Div(d.Terms.HoldFraction, fixedpoint.RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	between(t, "the forbidden C_legs figure", wrongAnnualized, "0.0711", "0.0713")
}
