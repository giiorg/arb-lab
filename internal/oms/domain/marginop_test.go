package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// allMarginOpStatuses is every declared member. Asserted over in full rather
// than sampled, for allPhases' reason one type over.
var allMarginOpStatuses = []domain.MarginOpStatus{
	domain.MarginOpApplied,
	domain.MarginOpNotApplied,
	domain.MarginOpPending,
	domain.MarginOpAmbiguous,
}

// The fixture's shape is chosen so that each number the rule reads DISAGREES
// with every other number in scope. Every one of these was a live collapse
// somewhere in this repo:
//
//   - before (100) ≠ delta (25), so "moved by the delta" and "the live figure
//     doubled" are different observations;
//   - before ≠ 0, so `live − before` and `live` are different quantities;
//   - the applied live (125) ≠ before, so a rule that ignored the live reading
//     entirely would answer differently;
//   - delta's magnitude appears with BOTH signs across the cases, so a rule
//     comparing absolute values answers wrongly on at least one;
//   - the bound (30s) is exercised strictly inside, exactly at, and strictly
//     outside, so `<` and `<=` are different predicates.
const (
	marginOpBound = 30 * time.Second
	marginOpAsset = fixedpoint.Asset("USDT")
)

func marginOpWatermark() time.Time {
	return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
}

func marginMoney(t *testing.T, v string) fixedpoint.Money {
	t.Helper()
	return marginMoneyIn(t, marginOpAsset, v)
}

func marginMoneyIn(t *testing.T, asset fixedpoint.Asset, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(asset, v)
	if err != nil {
		t.Fatalf("ParseMoney(%s, %q): %v", asset, v, err)
	}
	return m
}

func marginOpInstrument() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

// marginOp is the valid record every case below starts from.
func marginOp(t *testing.T) domain.MarginOp {
	t.Helper()
	return domain.MarginOp{
		ID:           "op-1",
		Instrument:   marginOpInstrument(),
		Delta:        marginMoney(t, "25"),
		BeforePosted: marginMoney(t, "100"),
		Watermark:    marginOpWatermark(),
	}
}

func TestMarginOpValidateAcceptsAWellFormedRecord(t *testing.T) {
	// The positive control. Without it every refusal below is satisfied by a
	// Validate that objects to everything.
	if err := marginOp(t).Validate(); err != nil {
		t.Fatalf("Validate of a well-formed record: %v", err)
	}
}

func TestMarginOpValidateAcceptsANegativeDelta(t *testing.T) {
	// The reducing direction is legal and is the second half of what makes the
	// signed amount a substitute for a kind enum. A Validate that required a
	// positive delta would make the sign carry no information at all.
	op := marginOp(t)
	op.Delta = marginMoney(t, "-25")
	if err := op.Validate(); err != nil {
		t.Fatalf("Validate of a reducing op: %v", err)
	}
}

func TestMarginOpValidateRefusals(t *testing.T) {
	spot := marginOpInstrument()
	spot.Product, spot.Contract = ids.ProductSpot, ""

	cases := []struct {
		name string
		edit func(*domain.MarginOp)
		want error
	}{
		{"no id", func(o *domain.MarginOp) { o.ID = "" }, domain.ErrMarginOpIDUnset},
		{"spot instrument", func(o *domain.MarginOp) { o.Instrument = spot }, domain.ErrMarginOpNotPerp},
		{"zero delta", func(o *domain.MarginOp) { o.Delta = marginMoney(t, "0") }, domain.ErrMarginOpDeltaZero},
		{"negative before-state", func(o *domain.MarginOp) {
			o.BeforePosted = marginMoney(t, "-1")
		}, domain.ErrMarginOpBeforeNegative},
		{"assets disagree", func(o *domain.MarginOp) {
			o.BeforePosted = marginMoneyIn(t, "USDC", "100")
		}, domain.ErrMarginOpAssetsDisagree},
		{"no watermark", func(o *domain.MarginOp) { o.Watermark = time.Time{} }, domain.ErrMarginOpWatermarkUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := marginOp(t)
			tc.edit(&op)
			err := op.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate: got %v, want %v", err, tc.want)
			}
		})
	}
}

// TestMarginOpValidateAcceptsAZeroBeforeState pins the one degenerate amount
// that IS legal, so the negative refusal above is not read as "non-positive".
//
// A position holding no posted margin is a real state — every position starts
// there — and refusing it would make the first top-up against a freshly opened
// position unresolvable.
func TestMarginOpValidateAcceptsAZeroBeforeState(t *testing.T) {
	op := marginOp(t)
	op.BeforePosted = marginMoney(t, "0")
	if err := op.Validate(); err != nil {
		t.Fatalf("Validate with a zero before-state: %v", err)
	}
}

func TestMarginOpResolve(t *testing.T) {
	inside := marginOpWatermark().Add(10 * time.Second)
	atBound := marginOpWatermark().Add(marginOpBound)
	outside := marginOpWatermark().Add(45 * time.Second)

	cases := []struct {
		name  string
		delta string
		live  string
		held  bool
		now   time.Time
		want  domain.MarginOpStatus
	}{
		// The margin moved by exactly what was asked for.
		{"add applied", "25", "125", true, outside, domain.MarginOpApplied},
		// The reducing direction, which a rule comparing magnitudes would also
		// call applied on the case below it.
		{"reduce applied", "-25", "75", true, outside, domain.MarginOpApplied},
		// Same magnitude, wrong direction: an Abs() anywhere in the comparison
		// reports this as applied.
		{"reduce, margin rose by the magnitude", "-25", "125", true, outside, domain.MarginOpAmbiguous},
		{"add, margin fell by the magnitude", "25", "75", true, outside, domain.MarginOpAmbiguous},

		// Nothing moved, and the bound is what decides. Exactly AT the bound
		// concludes: a bound states how long the venue may still act, and at its
		// far edge it may no longer.
		{"nothing moved, inside the bound", "25", "100", true, inside, domain.MarginOpPending},
		{"nothing moved, exactly at the bound", "25", "100", true, atBound, domain.MarginOpNotApplied},
		{"nothing moved, past the bound", "25", "100", true, outside, domain.MarginOpNotApplied},

		// Something else moved it. More time cannot disambiguate these, so the
		// bound is not consulted — asserted from both sides of it.
		{"moved by something else, past the bound", "25", "110", true, outside, domain.MarginOpAmbiguous},
		{"moved by something else, inside the bound", "25", "110", true, inside, domain.MarginOpAmbiguous},

		// Applied does not wait either: the answer is already an answer.
		{"applied inside the bound", "25", "125", true, inside, domain.MarginOpApplied},

		// A flat position outranks every other branch. Each of these three would
		// answer differently if the flat check ran anywhere but first.
		{"flat, with a movement that would read applied", "25", "125", false, outside, domain.MarginOpAmbiguous},
		{"flat, with no movement inside the bound", "25", "100", false, inside, domain.MarginOpAmbiguous},
		{"flat, with no movement past the bound", "25", "100", false, outside, domain.MarginOpAmbiguous},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := marginOp(t)
			op.Delta = marginMoney(t, tc.delta)
			got, err := op.Resolve(marginMoney(t, tc.live), tc.held, tc.now, marginOpBound)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Resolve: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestMarginOpResolveReadsTheLiveFigureAgainstTheRecordedBefore varies the
// BEFORE-state rather than the live one, which is the half a rule that hardcoded
// "the live figure equals the delta" would still pass the table above on.
func TestMarginOpResolveReadsTheLiveFigureAgainstTheRecordedBefore(t *testing.T) {
	op := marginOp(t)
	op.BeforePosted = marginMoney(t, "400")

	got, err := op.Resolve(marginMoney(t, "425"), true, marginOpWatermark().Add(time.Minute), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != domain.MarginOpApplied {
		t.Fatalf("Resolve with a different before-state: got %s, want %s", got, domain.MarginOpApplied)
	}

	// And the live figure that WOULD have been applied against the fixture's
	// usual before-state is now something else entirely.
	got, err = op.Resolve(marginMoney(t, "125"), true, marginOpWatermark().Add(time.Minute), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != domain.MarginOpAmbiguous {
		t.Fatalf("Resolve of the other before-state's applied figure: got %s, want %s", got, domain.MarginOpAmbiguous)
	}
}

// TestMarginOpResolveMeasuresElapsedForward pins the direction of the interval.
//
// A resolution instant BEFORE the watermark is a clock inversion, and the
// subtraction reversed gives a negative interval that is below every positive
// bound — so a reversed `now.Sub(Watermark)` reports Pending for ever while the
// op ages. Asserted as its own case because the table above cannot see it: every
// row there has now after the watermark.
func TestMarginOpResolveMeasuresElapsedForward(t *testing.T) {
	op := marginOp(t)
	// One hour past the watermark, which is far outside any bound.
	got, err := op.Resolve(marginMoney(t, "100"), true, marginOpWatermark().Add(time.Hour), marginOpBound)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != domain.MarginOpNotApplied {
		t.Fatalf("Resolve of a long-aged op: got %s, want %s", got, domain.MarginOpNotApplied)
	}
}

func TestMarginOpResolveRefusals(t *testing.T) {
	now := marginOpWatermark().Add(time.Minute)

	cases := []struct {
		name  string
		edit  func(*domain.MarginOp)
		live  fixedpoint.Money
		now   time.Time
		bound time.Duration
		want  error
	}{
		{"malformed record", func(o *domain.MarginOp) { o.ID = "" },
			marginMoney(t, "125"), now, marginOpBound, domain.ErrMarginOpIDUnset},
		{"zero bound", func(*domain.MarginOp) {},
			marginMoney(t, "125"), now, 0, domain.ErrMarginOpBoundNotPositive},
		{"negative bound", func(*domain.MarginOp) {},
			marginMoney(t, "125"), now, -time.Second, domain.ErrMarginOpBoundNotPositive},
		{"zero resolution instant", func(*domain.MarginOp) {},
			marginMoney(t, "125"), time.Time{}, marginOpBound, domain.ErrMarginOpResolutionInstantUnset},
		{"live reading in another asset", func(*domain.MarginOp) {},
			marginMoneyIn(t, "USDC", "125"), now, marginOpBound, domain.ErrMarginOpAssetsDisagree},
		{"live reading with no asset", func(*domain.MarginOp) {},
			fixedpoint.Money{}, now, marginOpBound, fixedpoint.ErrAssetUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := marginOp(t)
			tc.edit(&op)
			got, err := op.Resolve(tc.live, true, tc.now, tc.bound)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Resolve: got err %v, want %v", err, tc.want)
			}
			if got != domain.MarginOpUnresolved {
				t.Fatalf("Resolve refused and still answered %s", got)
			}
		})
	}
}

// TestMarginOpResolveRefusesAWiringDefectEvenWhenFlat pins that the flat
// shortcut does not become a way past the record's own checks.
//
// The flat branch answers before any arithmetic, which is correct — there is no
// live figure to difference — but it must not answer before the record has been
// validated, or a malformed op resolves to a confident Ambiguous and the wiring
// defect that produced it is never reported.
func TestMarginOpResolveRefusesAWiringDefectEvenWhenFlat(t *testing.T) {
	op := marginOp(t)
	op.Delta = marginMoney(t, "0")

	got, err := op.Resolve(fixedpoint.Money{}, false, marginOpWatermark().Add(time.Minute), marginOpBound)
	if !errors.Is(err, domain.ErrMarginOpDeltaZero) {
		t.Fatalf("Resolve of a zero-delta op reported flat: got err %v, want %v", err, domain.ErrMarginOpDeltaZero)
	}
	if got != domain.MarginOpUnresolved {
		t.Fatalf("Resolve refused and still answered %s", got)
	}
}

func TestMarginOpStatusValidate(t *testing.T) {
	for _, s := range allMarginOpStatuses {
		if err := s.Validate(); err != nil {
			t.Fatalf("Validate(%s): %v", s, err)
		}
	}
	if err := domain.MarginOpUnresolved.Validate(); !errors.Is(err, domain.ErrMarginOpStatusUnset) {
		t.Fatalf("Validate of the zero value: got %v, want %v", err, domain.ErrMarginOpStatusUnset)
	}
	if err := domain.MarginOpStatus("applied ").Validate(); !errors.Is(err, domain.ErrMarginOpStatusInvalid) {
		t.Fatalf("Validate of an undeclared value: got %v, want %v", err, domain.ErrMarginOpStatusInvalid)
	}
}

// TestMarginOpStatusStringsAreDistinct guards the one property the log depends
// on: these strings ARE the stored representation, so two members rendering
// alike would be two verdicts a replay could not tell apart.
func TestMarginOpStatusStringsAreDistinct(t *testing.T) {
	seen := make(map[string]domain.MarginOpStatus, len(allMarginOpStatuses))
	for _, s := range allMarginOpStatuses {
		text := s.String()
		if text == "" {
			t.Fatalf("%v renders as the zero value's spelling", s)
		}
		if other, dup := seen[text]; dup {
			t.Fatalf("%v and %v both render as %q", other, s, text)
		}
		seen[text] = s
	}
}

func TestMarginOpStatusConcluded(t *testing.T) {
	cases := []struct {
		status domain.MarginOpStatus
		want   bool
	}{
		{domain.MarginOpApplied, true},
		{domain.MarginOpNotApplied, true},
		{domain.MarginOpAmbiguous, true},
		{domain.MarginOpPending, false},
	}
	if len(cases) != len(allMarginOpStatuses) {
		t.Fatalf("Concluded is asserted over %d of %d statuses", len(cases), len(allMarginOpStatuses))
	}
	for _, tc := range cases {
		t.Run(tc.status.String(), func(t *testing.T) {
			got, err := tc.status.Concluded()
			if err != nil {
				t.Fatalf("Concluded: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Concluded: got %t, want %t", got, tc.want)
			}
		})
	}

	if _, err := domain.MarginOpUnresolved.Concluded(); !errors.Is(err, domain.ErrMarginOpStatusUnset) {
		t.Fatalf("Concluded of the zero value: got %v, want %v", err, domain.ErrMarginOpStatusUnset)
	}
	if _, err := domain.MarginOpStatus("resolved").Concluded(); !errors.Is(err, domain.ErrMarginOpStatusInvalid) {
		t.Fatalf("Concluded of an undeclared value: got %v, want %v", err, domain.ErrMarginOpStatusInvalid)
	}
}

// TestMarginOpResolveOnlyEverAnswersADeclaredStatus is the totality claim the
// tables above make case by case: every reachable combination of the rule's
// inputs lands on a member, and none lands on the zero value with a nil error.
func TestMarginOpResolveOnlyEverAnswersADeclaredStatus(t *testing.T) {
	deltas := []string{"25", "-25"}
	lives := []string{"0", "75", "100", "110", "125", "400"}
	instants := []time.Time{
		marginOpWatermark(),
		marginOpWatermark().Add(10 * time.Second),
		marginOpWatermark().Add(marginOpBound),
		marginOpWatermark().Add(time.Hour),
	}
	for _, delta := range deltas {
		for _, live := range lives {
			for _, now := range instants {
				for _, held := range []bool{true, false} {
					op := marginOp(t)
					op.Delta = marginMoney(t, delta)
					got, err := op.Resolve(marginMoney(t, live), held, now, marginOpBound)
					if err != nil {
						t.Fatalf("Resolve(delta=%s live=%s held=%t): %v", delta, live, held, err)
					}
					if err := got.Validate(); err != nil {
						t.Fatalf("Resolve(delta=%s live=%s held=%t) answered %q: %v", delta, live, held, got, err)
					}
				}
			}
		}
	}
}

// marginOpReconciliation is the complete act — core §7.3's who / what / when,
// plus the note.
//
// Every field differs from every other in KIND as well as value, which matters
// here more than usual: three of the four are strings, so a fixture using the
// same word twice would let a constructor that put the actor in the note pass.
func marginOpReconciliation() domain.MarginOpReconciliation {
	return domain.MarginOpReconciliation{
		Op:    "MOP-1",
		Actor: "gb",
		At:    time.Date(2026, 8, 8, 11, 30, 0, 0, time.UTC),
		Note:  "counted the venue's isolated margin against the float ledger",
	}
}

// TestMarginOpReconciliationRefusesEveryMissingField is core §7.3's audit
// requirement as four claims rather than one.
//
// Each case blanks exactly ONE field of an otherwise complete act, so a
// constructor that checked three of the four is caught by the case for the
// fourth. Asserting on the sentinel rather than on "an error" is what makes the
// four distinguishable: the whole reason they are separate sentinels is that an
// operator missing an actor and one missing a note need different things said to
// them (record §82 §2).
func TestMarginOpReconciliationRefusesEveryMissingField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		break_ func(*domain.MarginOpReconciliation)
		want   error
	}{
		{"no op", func(r *domain.MarginOpReconciliation) { r.Op = "" }, domain.ErrMarginOpIDUnset},
		{"no actor", func(r *domain.MarginOpReconciliation) { r.Actor = "" }, domain.ErrMarginOpReconcileActorUnset},
		{"no instant", func(r *domain.MarginOpReconciliation) { r.At = time.Time{} }, domain.ErrMarginOpReconcileInstantUnset},
		{"no note", func(r *domain.MarginOpReconciliation) { r.Note = "" }, domain.ErrMarginOpReconcileNoteUnset},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rec := marginOpReconciliation()
			c.break_(&rec)
			err := rec.Validate()
			if !errors.Is(err, c.want) {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestMarginOpReconciliationAcceptsTheCompleteAct is the positive control the
// four refusals above need: a predicate that objected to everything would score
// perfectly on them.
func TestMarginOpReconciliationAcceptsTheCompleteAct(t *testing.T) {
	t.Parallel()

	if err := marginOpReconciliation().Validate(); err != nil {
		t.Fatalf("Validate() on a complete reconciliation: %v", err)
	}
}

// TestMarginOpReconciliationAdmitsNoFinding is the RULING as a compile-time
// claim, and it is here because record §82 §2a is the paragraph most likely to
// be reopened by somebody who thinks a clearance should say what was found.
//
// It is written as an exhaustive field list rather than as a comment, so adding
// a fifth field breaks this test and sends the author to §2a's argument: §7.1
// calls this state Ambiguous because the attribution may be impossible, and no
// value of a finding would keep the block on. If a field is added it must arrive
// with a consumer — `treasury`'s float ledger — and this test is where the
// argument gets re-read.
func TestMarginOpReconciliationAdmitsNoFinding(t *testing.T) {
	t.Parallel()

	// A keyed literal would compile against extra fields; this one does not.
	rec := domain.MarginOpReconciliation{
		"MOP-1",
		"gb",
		time.Date(2026, 8, 8, 11, 30, 0, 0, time.UTC),
		"counted the venue's isolated margin against the float ledger",
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}
