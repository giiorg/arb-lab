package cashcarry_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

func assertMoney(t *testing.T, name string, got fixedpoint.Money, want string) {
	t.Helper()
	c, err := got.Cmp(mustMoney(t, want))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if c != 0 {
		t.Fatalf("%s = %s, want %s", name, got.Decimal(), want)
	}
}

func assertRefusal(t *testing.T, d domain.Decision, rule string) {
	t.Helper()
	if d.Verdict != domain.VerdictRefuse {
		t.Fatalf("verdict %s, want a refusal (%s)", d.Verdict, rule)
	}
	if !strings.HasPrefix(d.RefusedBy, rule) {
		t.Fatalf("refused by %q, want rule %q", d.RefusedBy, rule)
	}
	if d.Terms != (domain.DecisionTerms{}) {
		t.Fatal("refusal carries a priced ledger")
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("refusal does not validate: %v", err)
	}
}

// TestEnterMarginBoundary pins §2.2's "clears (cost + hurdle) WITH MARGIN"
// to the ulp, from both sides, with a nonzero measured drift: at an
// opportunity charge that puts the score exactly ON the margin the verdict
// is Enter (≥, not >), and one ulp more declines. A mutant that drops the
// margin term entirely still enters on the second input — score 0.915… — so
// the pair kills both the comparison direction and the term.
func TestEnterMarginBoundary(t *testing.T) {
	in := withDrift(t, flatEntry(t), "0.001")
	in.Opportunity = mustMoney(t, "8.974602931")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictEnter {
		t.Fatalf("at the margin: %s", d.Verdict)
	}
	if d.Terms.EntryMargin != in.Policy.Params.EntryDrift {
		t.Fatalf("terms margin %+v", d.Terms.EntryMargin)
	}

	in.Opportunity = mustMoney(t, "8.974602932")
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictDecline {
		t.Fatalf("one ulp under the margin: %s", d.Verdict)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("decline does not validate: %v", err)
	}
}

// TestEnterZeroMarginBoundary is the flat fixture's own edge: with a
// declared-zero drift the threshold is the scaled hurdle alone, and the
// score sits at zero exactly when opportunity eats the whole excess.
func TestEnterZeroMarginBoundary(t *testing.T) {
	in := flatEntry(t)
	in.Opportunity = mustMoney(t, "9.890036912")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictEnter {
		t.Fatalf("score zero: %s", d.Verdict)
	}
	in.Opportunity = mustMoney(t, "9.890036913")
	if d, err = cashcarry.Enter(in); err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictDecline {
		t.Fatalf("score −1 ulp: %s", d.Verdict)
	}
}

// TestEnterFundingIsTheAdverseDrawOnTheBid separates the three wrong reads
// at once: expected-instead-of-adverse (60 ≠ 90), ask-instead-of-bid
// (59.994 ≠ 60.006 — hmm, both pinned), and the sum being over the plan's
// settlements.
func TestEnterFundingIsTheAdverseDrawOnTheBid(t *testing.T) {
	in := flatEntry(t)
	// Adverse 0.0001 while expected stays 0.00015: the draw must price the
	// adverse scenario.
	for n := range in.Forecasts {
		in.Forecasts[n].Adverse = mustRate(t, "0.0001")
	}
	// A real spread: bid 99990, ask 100010. Income prices on the bid. The
	// higher ask raises the perp leg's margin requirement past the flat
	// fixture's one-cent headroom, so the margin grows with it — this test
	// is about the funding side, not the capital gate.
	in.View.Perp.Bid = mustPrice(t, "99990")
	in.View.Perp.Ask = mustPrice(t, "100010")
	in.Capital.PerpInitialMargin = mustMoney(t, "3333.67")

	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	// 0.0001 × 60 × (0.1 × 99990) = 59.994. The expected-rate mutant
	// answers 89.991; the ask mutant answers 60.006.
	assertMoney(t, "funding quantile", d.Terms.FundingQuantile, "59.994")
}

// TestEnterFeeCells pins all four cells on the flat ledger and the two
// tactic columns: taker entry spot 10, maker entry spot 8, perp cells 5 —
// the spot/perp asymmetry makes any swapped-leg or swapped-column mutant
// change a pinned number.
func TestEnterFeeCells(t *testing.T) {
	d, err := cashcarry.Enter(flatEntry(t))
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "spot entry", d.Terms.SpotEntryFee, "10")
	assertMoney(t, "spot exit", d.Terms.SpotExitFee, "10")
	assertMoney(t, "perp entry", d.Terms.PerpEntryFee, "5")
	assertMoney(t, "perp exit", d.Terms.PerpExitFee, "5")
	assertMoney(t, "slippage", d.Terms.Slippage, "8")

	// §2.4's documented opt-in: the maker tactic reroutes exactly ONE cell
	// — the spot entry — to the maker column.
	in := flatEntry(t)
	in.ExecCost.Tactic = domain.TacticMakerEntry
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "maker spot entry", d.Terms.SpotEntryFee, "8")
	assertMoney(t, "maker spot exit", d.Terms.SpotExitFee, "10")
	assertMoney(t, "maker perp entry", d.Terms.PerpEntryFee, "5")

	// Asymmetric execution costs: the flat fixture's 2/2 legs make a
	// spot/perp swap in the round-trip chain invisible (the checklist's
	// pinned-equal collapse), so the doubling claim is pinned on 3.30/1.10
	// — 2 × (3.30 + 1.10) = 8.80, where any dropped or doubled single leg
	// lands elsewhere.
	in = flatEntry(t)
	in.ExecCost.SpotCost = mustMoney(t, "3.30")
	in.ExecCost.PerpCost = mustMoney(t, "1.10")
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "asymmetric slippage", d.Terms.Slippage, "8.80")
}

// TestEnterHurdleUsesAllFiveCapitalTerms pins the C_allocated discipline:
// doubling the rescue margin — a term C_legs ignores — must move the scaled
// hurdle, because the denominator error §2.4's worked example forbids is
// exactly a term quietly dropped from this sum.
func TestEnterHurdleUsesAllFiveCapitalTerms(t *testing.T) {
	d, err := cashcarry.Enter(flatEntry(t))
	if err != nil {
		t.Fatal(err)
	}
	assertMoney(t, "scaled hurdle", d.Terms.ScaledHurdle, "42.109963088")
	if d.Terms.HoldFraction.Decimal() != "0.054794521" {
		t.Fatalf("hold fraction %s", d.Terms.HoldFraction.Decimal())
	}

	in := flatEntry(t)
	in.Capital.RescueMargin.Amount = mustMoney(t, "6666.66")
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	// 20040.00 × 0.046 × 20/365 → the C_legs mutant leaves 42.109963088.
	assertMoney(t, "hurdle with doubled rescue", d.Terms.ScaledHurdle, "50.511781239")
}

// TestEnterEffectiveHurdleNetsTheHaircuts gives the benchmark nonzero
// recorded adjustments: 4.6% − 0.1% − 0.2% − 0.3% = 4.0%. A mutant using
// the raw annual rate answers the flat fixture's 42.10…; the netted rate
// answers 36.61….
func TestEnterEffectiveHurdleNetsTheHaircuts(t *testing.T) {
	in := flatEntry(t)
	in.Hurdle.AccessCost = mustRate(t, "0.001")
	in.Hurdle.LiquidityHaircut = mustRate(t, "0.002")
	in.Hurdle.CounterpartyHaircut = mustRate(t, "0.003")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	// 16706.67 × 0.040 × 0.054794521, both multiplies away from zero.
	assertMoney(t, "netted hurdle", d.Terms.ScaledHurdle, "36.617359207")
}

func TestEnterMinFundingAPRBoundary(t *testing.T) {
	// The live estimate annualizes to exactly 0.16425 (0.00015 × 1095). A
	// floor AT that value proceeds; one ulp above refuses, naming the rule.
	in := flatEntry(t)
	in.Policy.Params.MinFundingAPR = mustRate(t, "0.16425")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict == domain.VerdictRefuse {
		t.Fatalf("floor at the estimate refused: %s", d.RefusedBy)
	}

	in.Policy.Params.MinFundingAPR = mustRate(t, "0.164250001")
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseMinFundingAPR)
}

// TestEnterMinFundingAPRReadsTheLiveInterval varies the interval away from
// the 8h every other fixture pins — the checklist's constant-collapse: with
// interval == 8h everywhere, "annualize at the live interval" and
// "annualize at 8h" agree on every input. At 7h the estimate annualizes to
// ~0.1877, clearing a 0.18 floor that the 8h reading's 0.164 would refuse.
func TestEnterMinFundingAPRReadsTheLiveInterval(t *testing.T) {
	in := flatEntry(t)
	in.View.Funding.Interval = 7 * time.Hour
	in.Policy.Params.MinFundingAPR = mustRate(t, "0.18")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict == domain.VerdictRefuse {
		t.Fatalf("7h interval refused under a floor only the 8h reading misses: %s", d.RefusedBy)
	}
}

func TestEnterMinHoldBoundary(t *testing.T) {
	in := flatEntry(t)
	in.Policy.Params.MinHoldSettlements = 60
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict == domain.VerdictRefuse {
		t.Fatalf("plan at the minimum refused: %s", d.RefusedBy)
	}

	in.Policy.Params.MinHoldSettlements = 61
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseMinHold)
}

func TestEnterRefusesAForeignQuantileDraw(t *testing.T) {
	in := flatEntry(t)
	in.Forecasts[7].Quantile = mustRate(t, "0.1")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseForecastDraw)
}

func TestEnterQuantizationRefusalsAreNamed(t *testing.T) {
	in := flatEntry(t)
	in.CandidateQty = mustQty(t, "0.0004")
	in.ExecCost.Qty = in.CandidateQty
	in.ResidualTolerance = mustQty(t, "0.0004")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseMinQty)

	in = flatEntry(t)
	in.ResidualTolerance = mustQty(t, "0")
	in.CandidateQty = mustQty(t, "0.1005")
	in.ExecCost.Qty = in.CandidateQty
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseResidual)
}

func TestEnterCapitalGateRefusesEitherLeg(t *testing.T) {
	// The flat fixture sits ON the spot boundary: notional 10000.00 against
	// principal 10000. One cent less refuses, naming the leg.
	in := flatEntry(t)
	in.Capital.SpotPrincipal = mustMoney(t, "9999.99")
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseCapital)
	if !strings.Contains(d.RefusedBy, "spot") {
		t.Fatalf("refusal does not name the leg: %q", d.RefusedBy)
	}

	// The perp boundary: 3333.33 × 3 = 9999.99 < 10000.
	in = flatEntry(t)
	in.Capital.PerpInitialMargin = mustMoney(t, "3333.33")
	d, err = cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseCapital)
	if !strings.Contains(d.RefusedBy, "perp") {
		t.Fatalf("refusal does not name the leg: %q", d.RefusedBy)
	}
}

func TestEnterRejectsWrongKindAndInvalidInput(t *testing.T) {
	if _, err := cashcarry.Enter(flatExit(t)); !errors.Is(err, cashcarry.ErrKindWrong) {
		t.Fatalf("exit input: %v", err)
	}
	in := flatEntry(t)
	in.Hurdle = domain.HurdleBenchmark{}
	if _, err := cashcarry.Enter(in); err == nil {
		t.Fatal("unpopulated hurdle accepted")
	}
}

// TestEnterDecisionEmbedsItsInput pins §40f's recompute property at the
// spot the battery can see: the returned Decision carries the exact input
// and validates as journalable.
func TestEnterDecisionEmbedsItsInput(t *testing.T) {
	in := flatEntry(t)
	d, err := cashcarry.Enter(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.Kind != domain.KindEntry || !d.At.Equal(in.At) {
		t.Fatalf("kind %s at %v", d.Kind, d.At)
	}
	if d.Input.Policy.Rev != in.Policy.Rev || d.Input.Plan.Num != 1 {
		t.Fatal("input identities not embedded")
	}
}
