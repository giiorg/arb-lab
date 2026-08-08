package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The act-history half of core §7.1's evidence contract (task #214).
//
// Every fixture here shares marginop_test.go's op: delta 25, before-state 100,
// watermark at noon, bound 30s. The numbers stay deliberately mutually distinct —
// 25, 100, 125 — so no assertion below can pass because two of them coincide.

// marginActAt builds one history row, defaulting to the op's own amount and
// direction so a test states only what it varies.
func marginActAt(t *testing.T, at time.Time) domain.MarginAct {
	t.Helper()
	return domain.MarginAct{
		Amount: marginMoney(t, "25"),
		Adds:   true,
		At:     at,
		Cause:  domain.MarginActUserAdjust,
	}
}

// marginActInside is a row squarely within the op's window — 10s past a 30s
// bound, so it is at neither edge and no edge test can pass by touching it.
func marginActInside(t *testing.T) domain.MarginAct {
	t.Helper()
	return marginActAt(t, marginOpWatermark().Add(10*time.Second))
}

// resolveWith runs the history arm at an instant PAST the bound, which is where
// most of the table's claims live: within the bound a silent history answers
// Pending regardless of the delta, and that rule has its own test.
func resolveWith(t *testing.T, op domain.MarginOp, live fixedpoint.Money, held bool, rows []domain.MarginAct) domain.MarginOpStatus {
	t.Helper()
	got, err := op.Resolve(live, held, domain.PublishedMarginActHistory(rows),
		marginOpWatermark().Add(time.Hour), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return got
}

// TestTheSameArithmeticResolvesOPPOSITELYOnTheTwoVENUES is the headline claim of
// this slice, and the one the whole three-state history type exists to protect.
//
// The observation is identical in both halves — the position's posted margin
// moved by EXACTLY the op's delta, 100 -> 125 — and the verdicts are opposite:
//
//	no act history exists (Bybit)          Applied
//	the act history returned no rows       Ambiguous
//
// On Bybit the delta is the whole of §7.1's contract, so a matching movement is
// the evidence. On Binance the venue has been asked whether it performed the
// adjustment and has said it did not, while the margin moved by that amount
// anyway — a second mover whose amount coincides.
//
// A nil-slice spelling of "no history" would make these two the same call, which
// is why they are asserted side by side in ONE test rather than in two that could
// drift apart.
func TestTheSameArithmeticResolvesOPPOSITELYOnTheTwoVENUES(t *testing.T) {
	op := marginOp(t)
	live := marginMoney(t, "125") // before 100 + delta 25, exactly
	now := marginOpWatermark().Add(time.Hour)

	noSource, err := op.Resolve(live, true, domain.NoMarginActHistory(), now, marginOpBound)
	if err != nil {
		t.Fatalf("Resolve with no act history: %v", err)
	}
	if noSource != domain.MarginOpApplied {
		t.Errorf("no act history: got %s, want %s", noSource, domain.MarginOpApplied)
	}

	silent, err := op.Resolve(live, true, domain.PublishedMarginActHistory(nil), now, marginOpBound)
	if err != nil {
		t.Fatalf("Resolve with a silent act history: %v", err)
	}
	if silent != domain.MarginOpAmbiguous {
		t.Errorf("silent act history: got %s, want %s", silent, domain.MarginOpAmbiguous)
	}

	if noSource == silent {
		t.Fatal("the two histories agreed — the distinction the three-state type exists for has collapsed")
	}
}

// TestTheHistoryArmsVerdicts walks the table.
//
// The live figure is varied ACROSS rows that speak, which is the point of the
// division of labour: when the history speaks the delta is not consulted at all,
// so a case with a matching row and a wildly wrong live figure must still be
// Applied. That case (`row speaks over a mismatched delta`) is the one this
// slice was built to win — it is an ordinary partial fill inside the window.
func TestTheHistoryArmsVerdicts(t *testing.T) {
	inside := marginActInside(t)

	other := marginActInside(t)
	other.Cause = domain.MarginActUnattributable

	cases := []struct {
		name string
		live fixedpoint.Money
		held bool
		rows []domain.MarginAct
		want domain.MarginOpStatus
	}{
		{
			// The win. A fill moved the posted margin to a number matching
			// nothing, and the act log holds one clean row.
			name: "row speaks over a mismatched delta",
			live: marginMoney(t, "417"), held: true,
			rows: []domain.MarginAct{inside},
			want: domain.MarginOpApplied,
		},
		{
			name: "row speaks and the delta agrees",
			live: marginMoney(t, "125"), held: true,
			rows: []domain.MarginAct{inside},
			want: domain.MarginOpApplied,
		},
		{
			// §7.1's named duplicate case. The rows carry no id, so this system
			// cannot tell its own act from a coincident one.
			//
			// live is the UNMOVED figure deliberately. A duplicate case whose
			// margin moved cannot see its own rule: drop the count test and two
			// rows fall through to the silent branch, which on a moved margin
			// answers Ambiguous as well — so `> 1` and `> 2` agree. Unmoved, the
			// silent branch answers NotApplied, and the two part company.
			name: "two matching rows",
			live: marginMoney(t, "100"), held: true,
			rows: []domain.MarginAct{inside, inside},
			want: domain.MarginOpAmbiguous,
		},
		{
			// The same claim with the margin moved, kept because it is the
			// realistic shape — but it is the case above that carries the rule.
			name: "two matching rows on a moved margin",
			live: marginMoney(t, "125"), held: true,
			rows: []domain.MarginAct{inside, inside},
			want: domain.MarginOpAmbiguous,
		},
		{
			// The venue recorded an act this system cannot attribute — reported,
			// never dropped.
			name: "one row with an unattributable cause",
			live: marginMoney(t, "125"), held: true,
			rows: []domain.MarginAct{other},
			want: domain.MarginOpAmbiguous,
		},
		{
			// One attributable and one not. Two matching rows is two matching
			// rows: the count is taken before the cause is read, because a pair
			// this system cannot separate is exactly the duplicate case.
			// Unmoved for the reason above: the count must be visible without the
			// silent branch supplying the same answer.
			name: "one attributable row beside an unattributable one",
			live: marginMoney(t, "100"), held: true,
			rows: []domain.MarginAct{inside, other},
			want: domain.MarginOpAmbiguous,
		},
		{
			name: "silent and nothing moved",
			live: marginMoney(t, "100"), held: true,
			rows: nil,
			want: domain.MarginOpNotApplied,
		},
		{
			name: "silent and something moved",
			live: marginMoney(t, "117"), held: true,
			rows: nil,
			want: domain.MarginOpAmbiguous,
		},
		{
			// A speaking row outranks flatness — see the (i) paragraph on
			// resolveWithActHistory. The position closing afterwards does not
			// unhappen an act the venue recorded.
			name: "row speaks on a flat position",
			live: fixedpoint.Money{}, held: false,
			rows: []domain.MarginAct{inside},
			want: domain.MarginOpApplied,
		},
		{
			// Silent AND flat: now there is no live posted margin to difference,
			// so #212's argument applies in full.
			name: "silent on a flat position",
			live: fixedpoint.Money{}, held: false,
			rows: nil,
			want: domain.MarginOpAmbiguous,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveWith(t, marginOp(t), tc.live, tc.held, tc.rows); got != tc.want {
				t.Errorf("Resolve: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestAFlatPositionIsReadDIFFERENTLYByTheTwoARMS pins the asymmetry #212's rule
// acquired, because the table above could satisfy an implementation that had
// simply dropped the flat rule everywhere.
//
// Same op, same flat position, same absent live figure. With no act history the
// verdict is Ambiguous (#212's rule, whose argument is about the DELTA's
// evidence); with a history that recorded the act it is Applied.
func TestAFlatPositionIsReadDIFFERENTLYByTheTwoARMS(t *testing.T) {
	op := marginOp(t)
	now := marginOpWatermark().Add(time.Hour)

	noSource, err := op.Resolve(fixedpoint.Money{}, false, domain.NoMarginActHistory(), now, marginOpBound)
	if err != nil {
		t.Fatalf("Resolve with no act history: %v", err)
	}
	if noSource != domain.MarginOpAmbiguous {
		t.Errorf("flat, no act history: got %s, want %s", noSource, domain.MarginOpAmbiguous)
	}

	withRow, err := op.Resolve(fixedpoint.Money{}, false,
		domain.PublishedMarginActHistory([]domain.MarginAct{marginActInside(t)}), now, marginOpBound)
	if err != nil {
		t.Fatalf("Resolve with a speaking act history: %v", err)
	}
	if withRow != domain.MarginOpApplied {
		t.Errorf("flat, act recorded: got %s, want %s", withRow, domain.MarginOpApplied)
	}
}

// TestTheBoundGuardsTheHISTORYsSilenceNotTheMARGINsStillness is premise (ii)
// re-derived, and it is the rule most likely to be dropped by a fix that "just"
// reused the delta arm.
//
// #212 consulted the bound on exactly one observation — nothing moved — on the
// ground that only that observation can change with time. With a second source
// the premise is false: a silent act history can fill in later, independently of
// the margin figure. So within the bound a silent history concludes NOTHING,
// whatever the margin did, and past the bound the delta separates the two
// verdicts.
//
// The two margin figures are chosen so that the mutant this test exists to kill —
// consulting the bound only when the margin is still — gives a DIFFERENT answer
// on the moved row (Ambiguous instead of Pending) while agreeing on the still one.
func TestTheBoundGuardsTheHISTORYsSilenceNotTheMARGINsStillness(t *testing.T) {
	op := marginOp(t)
	within := marginOpWatermark().Add(marginOpBound - time.Second)
	past := marginOpWatermark().Add(marginOpBound)

	cases := []struct {
		name string
		live string
		now  time.Time
		want domain.MarginOpStatus
	}{
		{"nothing moved, within the bound", "100", within, domain.MarginOpPending},
		{"something moved, within the bound", "117", within, domain.MarginOpPending},
		{"moved by exactly the delta, within the bound", "125", within, domain.MarginOpPending},
		{"nothing moved, at the bound", "100", past, domain.MarginOpNotApplied},
		{"something moved, at the bound", "117", past, domain.MarginOpAmbiguous},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := op.Resolve(marginMoney(t, tc.live), true, domain.PublishedMarginActHistory(nil),
				tc.now, marginOpBound)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("Resolve: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestARowThatARRIVEDIsNeverHeldForTheBound is the other half of premise (ii).
//
// The bound guards silence, so a history that SPOKE concludes immediately — an
// act the venue has recorded is not made more or less true by waiting. Without
// this, a "hoist the bound to the top" implementation passes every test above.
func TestARowThatARRIVEDIsNeverHeldForTheBound(t *testing.T) {
	op := marginOp(t)
	got, err := op.Resolve(marginMoney(t, "125"), true,
		domain.PublishedMarginActHistory([]domain.MarginAct{marginActAt(t, marginOpWatermark())}),
		marginOpWatermark(), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != domain.MarginOpApplied {
		t.Errorf("Resolve at the watermark itself: got %s, want %s", got, domain.MarginOpApplied)
	}
}

// TestTheMatchWindowsEDGES pins [Watermark, Watermark+bound).
//
// Four instants, and each names a distinct predicate: strictly before the open
// edge, exactly on it, strictly inside, and exactly on the close edge. A test
// using only "inside" and "far outside" would pass against `<=`, `<`, `>` and
// `>=` alike on both edges.
//
// Both exclusions land on Applied-vs-something-else, so a wrong edge is visible
// as a verdict rather than only as a row count.
func TestTheMatchWindowsEDGES(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want domain.MarginOpStatus
	}{
		{"a second before the watermark", marginOpWatermark().Add(-time.Second), domain.MarginOpAmbiguous},
		{"exactly at the watermark", marginOpWatermark(), domain.MarginOpApplied},
		{"inside", marginOpWatermark().Add(10 * time.Second), domain.MarginOpApplied},
		{"exactly at the bound", marginOpWatermark().Add(marginOpBound), domain.MarginOpAmbiguous},
		{"past the bound", marginOpWatermark().Add(marginOpBound + time.Second), domain.MarginOpAmbiguous},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// live 117: a movement that is neither zero nor the delta, so an
			// EXCLUDED row falls to Ambiguous rather than to NotApplied. That
			// keeps the assertion about the window rather than about the delta.
			got := resolveWith(t, marginOp(t), marginMoney(t, "117"), true, []domain.MarginAct{marginActAt(t, tc.at)})
			if got != tc.want {
				t.Errorf("row at %v: got %s, want %s", tc.at, got, tc.want)
			}
		})
	}
}

// TestTheMatchDiscriminatesAMOUNTAndDIRECTION.
//
// The amount case varies the row away from the op's 25 in both directions, and
// the direction case needs an op whose delta is NEGATIVE — without one, "matches
// the op's direction" and "is an add" are the same predicate on every fixture in
// this file.
func TestTheMatchDiscriminatesAMOUNTAndDIRECTION(t *testing.T) {
	t.Run("a row of the wrong amount does not match", func(t *testing.T) {
		for _, amount := range []string{"24", "26", "100", "125"} {
			row := marginActInside(t)
			row.Amount = marginMoney(t, amount)
			if got := resolveWith(t, marginOp(t), marginMoney(t, "117"), true, []domain.MarginAct{row}); got != domain.MarginOpAmbiguous {
				t.Errorf("row of %s: got %s, want %s", amount, got, domain.MarginOpAmbiguous)
			}
		}
	})

	t.Run("a row of the wrong direction does not match", func(t *testing.T) {
		row := marginActInside(t)
		row.Adds = false
		if got := resolveWith(t, marginOp(t), marginMoney(t, "117"), true, []domain.MarginAct{row}); got != domain.MarginOpAmbiguous {
			t.Errorf("a reduce row against an ADD op: got %s, want %s", got, domain.MarginOpAmbiguous)
		}
	})

	t.Run("a REDUCE op matches a reduce row and not an add", func(t *testing.T) {
		op := marginOp(t)
		op.Delta = marginMoney(t, "-25")

		reduce := marginActInside(t)
		reduce.Adds = false
		// live 117: neither the before-state nor before+delta, so a failed match
		// lands on Ambiguous and the assertion stays about the direction.
		if got := resolveWith(t, op, marginMoney(t, "117"), true, []domain.MarginAct{reduce}); got != domain.MarginOpApplied {
			t.Errorf("a reduce row against a REDUCE op: got %s, want %s", got, domain.MarginOpApplied)
		}

		add := marginActInside(t)
		if got := resolveWith(t, op, marginMoney(t, "117"), true, []domain.MarginAct{add}); got != domain.MarginOpAmbiguous {
			t.Errorf("an add row against a REDUCE op: got %s, want %s", got, domain.MarginOpAmbiguous)
		}
	})

	t.Run("the magnitude is compared, not the signed figure", func(t *testing.T) {
		// A reduce op of -25 must match a row whose recorded amount is the
		// positive 25 the venue publishes. This is what would break if the match
		// compared row.Amount against op.Delta directly.
		op := marginOp(t)
		op.Delta = marginMoney(t, "-25")
		row := marginActInside(t)
		row.Adds = false
		row.Amount = marginMoney(t, "25")
		if got := resolveWith(t, op, marginMoney(t, "117"), true, []domain.MarginAct{row}); got != domain.MarginOpApplied {
			t.Errorf("unsigned 25 against a -25 op: got %s, want %s", got, domain.MarginOpApplied)
		}
	})
}

// TestTheUNSETHistoryIsRefused is the zero value's refusal — the whole reason the
// state is an unexported enum rather than a slice.
func TestTheUNSETHistoryIsRefused(t *testing.T) {
	op := marginOp(t)
	got, err := op.Resolve(marginMoney(t, "125"), true, domain.MarginActHistory{},
		marginOpWatermark().Add(time.Hour), marginOpBound)
	if !errors.Is(err, domain.ErrMarginHistoryUnset) {
		t.Fatalf("Resolve with the zero history: got %v, want %v", err, domain.ErrMarginHistoryUnset)
	}
	if got != domain.MarginOpUnresolved {
		t.Errorf("status: got %s, want %s", got, domain.MarginOpUnresolved)
	}

	// The positive control, and it is a SHARP one: the refused value and the
	// accepted one differ only by which constructor built them, and under the
	// same arithmetic the accepted one answers Applied.
	ok, err := op.Resolve(marginMoney(t, "125"), true, domain.NoMarginActHistory(),
		marginOpWatermark().Add(time.Hour), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve with a constructed history: %v", err)
	}
	if ok != domain.MarginOpApplied {
		t.Errorf("constructed history: got %s, want %s", ok, domain.MarginOpApplied)
	}
}

// TestMarginActHistoryValidateRefusals covers the rows a venue could never send
// but an adapter could construct.
func TestMarginActHistoryValidateRefusals(t *testing.T) {
	good := marginActInside(t)

	cases := []struct {
		name string
		row  func(domain.MarginAct) domain.MarginAct
		want error
	}{
		{"zero amount", func(a domain.MarginAct) domain.MarginAct {
			a.Amount = marginMoney(t, "0")
			return a
		}, domain.ErrMarginActAmountNotPositive},
		{"negative amount", func(a domain.MarginAct) domain.MarginAct {
			a.Amount = marginMoney(t, "-25")
			return a
		}, domain.ErrMarginActAmountNotPositive},
		{"no instant", func(a domain.MarginAct) domain.MarginAct {
			a.At = time.Time{}
			return a
		}, domain.ErrMarginActInstantUnset},
		{"no cause", func(a domain.MarginAct) domain.MarginAct {
			a.Cause = domain.MarginActCauseUnset
			return a
		}, domain.ErrMarginActCauseUnset},
		{"undeclared cause", func(a domain.MarginAct) domain.MarginAct {
			a.Cause = domain.MarginActCause("auto_top_up")
			return a
		}, domain.ErrMarginActCauseInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := domain.PublishedMarginActHistory([]domain.MarginAct{tc.row(good)})
			if err := hist.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate: got %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control.
	if err := domain.PublishedMarginActHistory([]domain.MarginAct{good}).Validate(); err != nil {
		t.Fatalf("Validate on a well-formed row: %v", err)
	}
	// And an absent history validates — it is a legitimate reading, not a gap.
	if err := domain.NoMarginActHistory().Validate(); err != nil {
		t.Fatalf("Validate on an absent history: %v", err)
	}
	// As does a PUBLISHED one carrying nothing, which is the evidence-bearing
	// empty this type exists to keep distinct from the one above.
	if err := domain.PublishedMarginActHistory(nil).Validate(); err != nil {
		t.Fatalf("Validate on a silent history: %v", err)
	}
}

// TestPublishedReportsTheSTATENotTheRowCount.
//
// A silent published history reports true, and an absent one reports false. If
// Published were implemented as len(rows) > 0 every verdict test above would
// still pass — the silent-published cases would simply take the no-source arm and
// reach the same NotApplied on a still margin — except the headline test, which
// is one call. This makes the claim directly, so the two are not one fixture
// apart.
func TestPublishedReportsTheSTATENotTheRowCount(t *testing.T) {
	if !domain.PublishedMarginActHistory(nil).Published() {
		t.Error("a silent published history reports not-published")
	}
	if !domain.PublishedMarginActHistory([]domain.MarginAct{marginActInside(t)}).Published() {
		t.Error("a speaking published history reports not-published")
	}
	if domain.NoMarginActHistory().Published() {
		t.Error("an absent history reports published")
	}
	if (domain.MarginActHistory{}).Published() {
		t.Error("the zero history reports published")
	}
}

// TestOpAddsRendersTheDeltasSIGN.
func TestOpAddsRendersTheDeltasSIGN(t *testing.T) {
	op := marginOp(t)
	adds, err := op.Adds()
	if err != nil {
		t.Fatalf("Adds: %v", err)
	}
	if !adds {
		t.Error("a positive delta reported as a reduction")
	}

	op.Delta = marginMoney(t, "-25")
	adds, err = op.Adds()
	if err != nil {
		t.Fatalf("Adds: %v", err)
	}
	if adds {
		t.Error("a negative delta reported as a top-up")
	}

	op.Delta = marginMoney(t, "0")
	if _, err := op.Adds(); !errors.Is(err, domain.ErrMarginOpDeltaZero) {
		t.Fatalf("Adds on a zero delta: got %v, want %v", err, domain.ErrMarginOpDeltaZero)
	}
}
