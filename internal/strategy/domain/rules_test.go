package domain_test

// Targeted rule tests: each aims at a distinction the checklist says a lazy
// fixture collapses — boundaries from both sides, mismatches in both
// directions, declared-zero vs absent.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// --- absent vs declared zero (§40b's one deliberate distinction) -----------

func TestDeltaTermsRefuseAbsenceNotZero(t *testing.T) {
	// Declared zero passes — the fixtures already carry ZeroMoney deltas on
	// the entry input, so the positive half is TestFixturesAreValid. The
	// refusal half: a Go zero value (no asset) is absence, refused.
	in := entryInput(t)
	in.ClosingCostDelta = fixedpoint.Money{}
	if err := in.Validate(); err == nil {
		t.Fatal("absent closing-cost delta validated")
	}

	in = entryInput(t)
	in.RiskDelta = fixedpoint.Money{}
	if err := in.Validate(); err == nil {
		t.Fatal("absent risk delta validated")
	}
}

func TestDwellDeclaredZeroIsNotAbsence(t *testing.T) {
	// A declared, never-counted dwell is valid…
	d := domain.DwellState{Declared: true}
	if err := d.Validate(); err != nil {
		t.Fatalf("declared zero dwell: %v", err)
	}
	// …an undeclared one is not, even with the same numbers.
	if err := (domain.DwellState{}).Validate(); !errors.Is(err, domain.ErrDwellUndeclared) {
		t.Fatalf("want ErrDwellUndeclared, got %v", err)
	}
	// And the two halves of consistency, each direction separately.
	d = domain.DwellState{Declared: true, Count: 2}
	if err := d.Validate(); !errors.Is(err, domain.ErrDwellInconsistent) {
		t.Fatalf("count with no instant: want ErrDwellInconsistent, got %v", err)
	}
	d = domain.DwellState{Declared: true, LastRealized: baseAt}
	if err := d.Validate(); !errors.Is(err, domain.ErrDwellInconsistent) {
		t.Fatalf("instant with zero count: want ErrDwellInconsistent, got %v", err)
	}
}

// --- ordering: ties are refusals, not edge-passes --------------------------

func TestRealizedWindowRefusesTiesAndDisorder(t *testing.T) {
	in := entryInput(t)
	// A tie: two settlements at one instant double-counts a lump.
	in.View.Realized[1].At = in.View.Realized[0].At
	if err := in.Validate(); !errors.Is(err, domain.ErrNotAscending) {
		t.Fatalf("tied window: want ErrNotAscending, got %v", err)
	}
	// Disorder proper.
	in = entryInput(t)
	in.View.Realized[0], in.View.Realized[1] = in.View.Realized[1], in.View.Realized[0]
	if err := in.Validate(); !errors.Is(err, domain.ErrNotAscending) {
		t.Fatalf("disordered window: want ErrNotAscending, got %v", err)
	}
}

func TestHoldPlanBoundaries(t *testing.T) {
	// A settlement exactly AT terminal is inside the hold (Enumerate's
	// inclusive rule) — must validate.
	p := fixturePlanSchedule()
	p.Settlements = append(p.Settlements, p.Terminal)
	if err := p.Validate(); err != nil {
		t.Fatalf("settlement at terminal: %v", err)
	}
	// One nanosecond past terminal is a lengthened hold — refused.
	p = fixturePlanSchedule()
	p.Settlements = append(p.Settlements, p.Terminal.Add(time.Nanosecond))
	if err := p.Validate(); !errors.Is(err, domain.ErrScheduleOverrun) {
		t.Fatalf("settlement past terminal: want ErrScheduleOverrun, got %v", err)
	}
}

// --- the plan revision / trigger agreement, both directions ----------------

func TestPlanRevisionTriggerAgreement(t *testing.T) {
	// Revision 2 claiming to be the entry.
	p := domain.HoldPlanRevision{Num: 2, Trigger: domain.TriggerEntry, At: baseAt, Plan: fixturePlanSchedule()}
	if err := p.Validate(); !errors.Is(err, domain.ErrRevisionSequence) {
		t.Fatalf("rev 2 entry: want ErrRevisionSequence, got %v", err)
	}
	// Revision 1 claiming to be a regeneration.
	p = domain.HoldPlanRevision{Num: 1, Trigger: domain.TriggerIntervalFlip, At: baseAt, Plan: fixturePlanSchedule()}
	if err := p.Validate(); !errors.Is(err, domain.ErrRevisionSequence) {
		t.Fatalf("rev 1 flip: want ErrRevisionSequence, got %v", err)
	}
}

// --- §2.4's horizon rule: exact cover, boundary from both sides ------------

func TestForecastHorizonExactCover(t *testing.T) {
	// One forecast short: refused.
	in := entryInput(t)
	in.Forecasts = in.Forecasts[:len(in.Forecasts)-1]
	if err := in.Validate(); !errors.Is(err, domain.ErrForecastHorizon) {
		t.Fatalf("short cover: want ErrForecastHorizon, got %v", err)
	}
	// One extra invents a settlement: refused.
	in = entryInput(t)
	in.Forecasts = append(in.Forecasts, fixtureForecast(t, in.Plan.Plan.Terminal.Add(-time.Hour)))
	if err := in.Validate(); !errors.Is(err, domain.ErrForecastHorizon) {
		t.Fatalf("extra forecast: want ErrForecastHorizon, got %v", err)
	}
	// Right count, wrong instant: refused.
	in = entryInput(t)
	in.Forecasts[1].At = in.Forecasts[1].At.Add(time.Minute)
	if err := in.Validate(); !errors.Is(err, domain.ErrForecastHorizon) {
		t.Fatalf("shifted forecast: want ErrForecastHorizon, got %v", err)
	}
}

func TestExitForecastBoundaryIsStrictlyAfter(t *testing.T) {
	// The exit fixture's At sits between settlements 1 and 2; move it to
	// EXACTLY settlement 2's instant. That settlement is settling now, so it
	// is no longer forecastable: the correct cover is only settlement 3.
	in := exitInput(t)
	settleAt := in.Plan.Plan.Settlements[1]
	in.At = settleAt
	in.Dwell.LastRealized = in.Plan.Plan.Settlements[0]

	// Covering both remaining settlements — including the one AT the
	// instant — must be refused…
	if err := in.Validate(); !errors.Is(err, domain.ErrForecastHorizon) {
		t.Fatalf("cover including boundary settlement: want ErrForecastHorizon, got %v", err)
	}
	// …and covering only the strictly-later one must pass.
	in.Forecasts = in.Forecasts[1:]
	if err := in.Validate(); err != nil {
		t.Fatalf("strictly-after cover at boundary: %v", err)
	}
}

// --- leg agreement: the assembled view is where two ports meet -------------

func TestInputRefusesForeignLegs(t *testing.T) {
	foreign := perpInst
	foreign.Symbol = "ETHUSDT"

	in := entryInput(t)
	in.PerpFees.Instrument = foreign
	if err := in.Validate(); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Fatalf("foreign fee leg: want ErrLegsMismatch, got %v", err)
	}

	in = entryInput(t)
	in.View.Funding.Instrument = foreign
	if err := in.Validate(); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Fatalf("foreign funding block: want ErrLegsMismatch, got %v", err)
	}

	// The cost estimate must have priced THIS candidate.
	in = entryInput(t)
	in.ExecCost.Qty = mustQty(t, "0.154")
	if err := in.Validate(); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Fatalf("estimate for another qty: want ErrLegsMismatch, got %v", err)
	}
}

// --- kind-dependent members, both directions -------------------------------

func TestKindMembersBothDirections(t *testing.T) {
	// An entry carrying position facts ran the wrong test.
	in := entryInput(t)
	in.Position = exitInput(t).Position
	if err := in.Validate(); !errors.Is(err, domain.ErrVerdictKindMismatch) {
		t.Fatalf("entry with position: want ErrVerdictKindMismatch, got %v", err)
	}
	// An exit without a dwell fed back.
	ex := exitInput(t)
	ex.Dwell = domain.DwellState{}
	if err := ex.Validate(); !errors.Is(err, domain.ErrDwellUndeclared) {
		t.Fatalf("exit without dwell: want ErrDwellUndeclared, got %v", err)
	}
	// An exit without position facts.
	ex = exitInput(t)
	ex.Position = domain.PositionView{}
	if err := ex.Validate(); err == nil {
		t.Fatal("exit without position validated")
	}
}

// --- verdict/kind agreement and refusal naming -----------------------------

func TestVerdictKindAgreement(t *testing.T) {
	d := entryDecision(t)
	d.Verdict = domain.VerdictClose
	if err := d.Validate(); !errors.Is(err, domain.ErrVerdictKindMismatch) {
		t.Fatalf("close on entry: want ErrVerdictKindMismatch, got %v", err)
	}
	d = exitDecision(t)
	d.Verdict = domain.VerdictEnter
	if err := d.Validate(); !errors.Is(err, domain.ErrVerdictKindMismatch) {
		t.Fatalf("enter on exit: want ErrVerdictKindMismatch, got %v", err)
	}
}

func TestRefusalRules(t *testing.T) {
	// A refusal with no rule named.
	d := refusalDecision(t)
	d.RefusedBy = ""
	if err := d.Validate(); !errors.Is(err, domain.ErrRefusalUnnamed) {
		t.Fatalf("unnamed refusal: want ErrRefusalUnnamed, got %v", err)
	}
	// A refusal carrying a ledger it never computed.
	d = refusalDecision(t)
	d.Terms = entryTerms(t)
	if err := d.Validate(); !errors.Is(err, domain.ErrRefusalMisplaced) {
		t.Fatalf("refusal with terms: want ErrRefusalMisplaced, got %v", err)
	}
	// A refusing rule on a scored verdict.
	d2 := entryDecision(t)
	d2.RefusedBy = "some rule"
	if err := d2.Validate(); !errors.Is(err, domain.ErrRefusalMisplaced) {
		t.Fatalf("rule on scored verdict: want ErrRefusalMisplaced, got %v", err)
	}
}

// --- crossed vs locked top -------------------------------------------------

func TestTopOfBookBoundaries(t *testing.T) {
	// Locked (bid == ask) is admitted.
	pv := fixtureView(t).Spot
	pv.Ask = pv.Bid
	if err := pv.Validate(); err != nil {
		t.Fatalf("locked top: %v", err)
	}
	// Crossed is refused.
	pv = fixtureView(t).Spot
	pv.Bid, pv.Ask = pv.Ask, pv.Bid
	if err := pv.Validate(); !errors.Is(err, domain.ErrCrossedTop) {
		t.Fatalf("crossed top: want ErrCrossedTop, got %v", err)
	}
}

// --- terms: exit-only members in both directions ---------------------------

func TestTermsKindMembers(t *testing.T) {
	// Entry terms carrying a band.
	terms := entryTerms(t)
	terms.Band = exitTerms(t).Band
	if err := terms.Validate(domain.KindEntry); !errors.Is(err, domain.ErrVerdictKindMismatch) {
		t.Fatalf("entry terms with band: want ErrVerdictKindMismatch, got %v", err)
	}
	// Exit terms without one.
	et := exitTerms(t)
	et.Band = domain.MeasuredRate{}
	if err := et.Validate(domain.KindExit); err == nil {
		t.Fatal("exit terms without band validated")
	}
}

// --- capital: one asset, and Total sums every term -------------------------

func TestCapitalAssetAgreementAndTotal(t *testing.T) {
	c := entryInput(t).Capital
	mixed, err := fixedpoint.ParseMoney("USDC", "10")
	if err != nil {
		t.Fatal(err)
	}
	c.CommittedFloat = mixed
	if err := c.Validate(); !errors.Is(err, domain.ErrAssetMixed) {
		t.Fatalf("mixed asset: want ErrAssetMixed, got %v", err)
	}

	c = entryInput(t).Capital
	total, err := c.Total()
	if err != nil {
		t.Fatal(err)
	}
	// 10000 + 3334 + 1200 + 9007199.254740993 + 0
	want := mustMoney(t, "9021733.254740993")
	if total != want {
		t.Fatalf("total %s, want %s", total, want)
	}
}
