package cashcarry_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// The flat exit fixture's arithmetic, once: two settlements remain at
// adverse 0.0001 → funding $2.00 on the $10,000 notional; remaining
// fraction 15h/8760h = 0.001712329; scaled hurdle 16706.67 × 0.046 ×
// fraction = 1.315936515; band 16706.67 × 0.01 × fraction = 0.286073156.
// With opportunity as the knob: score = 2 − opp − 1.315936515, and the
// below-band boundary sits at opp = 0.970136641 exactly.

func TestContinueBaselineLedger(t *testing.T) {
	d, err := cashcarry.Continue(flatExit(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("verdict %s", d.Verdict)
	}
	assertMoney(t, "remaining funding", d.Terms.FundingQuantile, "2")
	assertMoney(t, "scaled hurdle", d.Terms.ScaledHurdle, "1.315936515")
	if d.Terms.HoldFraction.Decimal() != "0.001712329" {
		t.Fatalf("remaining fraction %s", d.Terms.HoldFraction.Decimal())
	}
	// Sunk entry costs never re-enter the exit ledger: every fee cell and
	// the slippage line are zeros — §40f rule 2's first wrong shape.
	assertMoney(t, "spot entry fee", d.Terms.SpotEntryFee, "0")
	assertMoney(t, "spot exit fee", d.Terms.SpotExitFee, "0")
	assertMoney(t, "perp entry fee", d.Terms.PerpEntryFee, "0")
	assertMoney(t, "perp exit fee", d.Terms.PerpExitFee, "0")
	assertMoney(t, "slippage", d.Terms.Slippage, "0")
	if d.Terms.Band != d.Input.Policy.Params.ExitBand {
		t.Fatalf("band %+v", d.Terms.Band)
	}
	// Score +0.684… is above the band floor: the dwell RESETS — "stays
	// below" means consecutively, and this evaluation broke the streak.
	if d.Terms.Dwell != (domain.DwellState{Declared: true}) {
		t.Fatalf("dwell %+v, want a declared reset", d.Terms.Dwell)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestContinueSunkCostsCannotMoveTheVerdict puts the score exactly ON the
// band floor — one ulp of any re-subtracted cost flips it below, which the
// dwell advance makes visible. Then it inflates every fee input tenfold and
// requires the same knife-edge verdict: a mutant charging the round trip to
// the exit branch moves a pinned outcome, not nothing.
func TestContinueSunkCostsCannotMoveTheVerdict(t *testing.T) {
	in := flatExit(t)
	in.Opportunity = mustMoney(t, "0.970136641")
	d, err := cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("at the band floor: %s", d.Verdict)
	}
	if d.Terms.Dwell != (domain.DwellState{Declared: true}) {
		t.Fatalf("at-floor dwell %+v", d.Terms.Dwell)
	}

	in.SpotFees.Taker = mustRate(t, "0.01")
	in.PerpFees.Taker = mustRate(t, "0.005")
	in.ExecCost.SpotCost = mustMoney(t, "20")
	in.ExecCost.PerpCost = mustMoney(t, "20")
	d, err = cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue || d.Terms.Dwell != (domain.DwellState{Declared: true}) {
		t.Fatalf("fee inputs moved the exit score: %s %+v", d.Verdict, d.Terms.Dwell)
	}
}

// TestContinueBelowBandBoundary crosses the floor by one ulp: the dwell
// advances on the newly realized settlement (count 1→2), reaches
// exit_persist = 2, and the verdict is Close. At the floor exactly it is
// Continue — the strict inequality, pinned from both sides.
func TestContinueBelowBandBoundary(t *testing.T) {
	in := flatExit(t)
	in.Opportunity = mustMoney(t, "0.970136642")
	d, err := cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictClose {
		t.Fatalf("one ulp below the floor: %s", d.Verdict)
	}
	want := domain.DwellState{Declared: true, Count: 2, LastRealized: baseAt.Add(464 * time.Hour)}
	if d.Terms.Dwell != want {
		t.Fatalf("dwell %+v, want %+v", d.Terms.Dwell, want)
	}
}

// TestContinueDwellCountsSettlementsNotCalls re-evaluates below the band
// with the newest settlement ALREADY counted: the dwell must not move, so
// the verdict stays Continue one advance short of the persist threshold.
func TestContinueDwellCountsSettlementsNotCalls(t *testing.T) {
	in := flatExit(t)
	in.Opportunity = mustMoney(t, "0.970136642")
	in.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt.Add(464 * time.Hour)}
	d, err := cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictContinue {
		t.Fatalf("same-settlement call closed: %s", d.Verdict)
	}
	if d.Terms.Dwell != in.Dwell {
		t.Fatalf("dwell moved on a call: %+v", d.Terms.Dwell)
	}
}

// TestContinueDeltaTermsAreCharged nudges each Δ line by one ulp from the
// knife edge: §2.4 charges cost terms exactly where the branches differ,
// and a dropped Δ member is invisible at every other score.
func TestContinueDeltaTermsAreCharged(t *testing.T) {
	for name, set := range map[string]func(*domain.DecisionInput){
		"closing-cost delta": func(in *domain.DecisionInput) { in.ClosingCostDelta = mustMoney(t, "0.000000001") },
		"risk delta":         func(in *domain.DecisionInput) { in.RiskDelta = mustMoney(t, "0.000000001") },
	} {
		in := flatExit(t)
		in.Opportunity = mustMoney(t, "0.970136641")
		set(&in)
		d, err := cashcarry.Continue(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d.Verdict != domain.VerdictClose {
			t.Fatalf("%s not charged: %s", name, d.Verdict)
		}
	}
}

// TestContinuePlanCompleteCloses evaluates past the last settlement: no
// forecastable settlement remains, the plan is finished, and the position
// closes without the dwell — the dwell prevents whipsaw exits mid-plan, it
// does not hold a finished plan open.
func TestContinuePlanCompleteCloses(t *testing.T) {
	in := flatExit(t)
	at := baseAt.Add(481 * time.Hour)
	in.At = at
	in.View.Spot.Watermark = watermark(at, 401)
	in.View.Perp.Watermark = watermark(at, 403)
	in.View.Funding.Watermark = watermark(at.Add(-time.Second), 405)
	in.View.Funding.NextSettle = at.Add(7 * time.Hour)
	in.Forecasts = nil
	in.Dwell = domain.DwellState{Declared: true, Count: 1, LastRealized: baseAt.Add(464 * time.Hour)}

	d, err := cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != domain.VerdictClose {
		t.Fatalf("finished plan: %s", d.Verdict)
	}
	assertMoney(t, "funding of nothing", d.Terms.FundingQuantile, "0")
	assertMoney(t, "hurdle of nothing", d.Terms.ScaledHurdle, "0")
	// The dwell passes through untouched: no settlement was judged.
	if d.Terms.Dwell != in.Dwell {
		t.Fatalf("plan-complete close moved the dwell: %+v", d.Terms.Dwell)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestContinueRefusesAForeignQuantileDraw(t *testing.T) {
	in := flatExit(t)
	in.Forecasts[1].Quantile = mustRate(t, "0.1")
	d, err := cashcarry.Continue(in)
	if err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, d, cashcarry.RefuseForecastDraw)
}

func TestContinueRejectsWrongKindAndTornInputs(t *testing.T) {
	if _, err := cashcarry.Continue(flatEntry(t)); !errors.Is(err, cashcarry.ErrKindWrong) {
		t.Fatalf("entry input: %v", err)
	}

	// A working plan whose terminal disagrees with the entry evidence is
	// refused by input validation — §40f's unchanged-terminal rule.
	in := flatExit(t)
	in.Plan.Plan.Terminal = in.Plan.Plan.Terminal.Add(8 * time.Hour)
	if _, err := cashcarry.Continue(in); !errors.Is(err, domain.ErrTerminalMoved) {
		t.Fatalf("moved terminal: %v", err)
	}

	// An exit candidate that is not the held quantity is a partial close
	// nobody's policy stated.
	in = flatExit(t)
	in.CandidateQty = mustQty(t, "0.05")
	in.ExecCost.Qty = in.CandidateQty
	if _, err := cashcarry.Continue(in); !errors.Is(err, domain.ErrCandidateNotHeld) {
		t.Fatalf("partial close: %v", err)
	}
}
