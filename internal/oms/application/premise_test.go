package application_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// neutralPackage is the state SM §7 requires a generation to open in: a
// cash-and-carry with both legs fully executed and equal, so the package is
// physically flat and every interval has collapsed.
//
// The two legs are EQUAL here, deliberately, and that is the one fixture in this
// file where equal legs are right rather than the collapse this repo keeps
// finding: §7's entry precondition is residual ZERO, so a fixture that was not
// flat could not reach the precondition at all. Every test below that needs the
// residual to be attributable varies something else.
func neutralPackage(t *testing.T) []ports.OrderRecord {
	t.Helper()
	return []ports.OrderRecord{
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "1", domain.StateFilled),
	}
}

func premiseFor(t *testing.T, c *application.Classifier, g *domain.Generation) domain.Premise {
	t.Helper()
	p, err := c.PremiseFor(hedgeGroup(), g.OwnLegs())
	if err != nil {
		t.Fatalf("PremiseFor: %v", err)
	}
	return p
}

func openGeneration(t *testing.T, c *application.Classifier, target string) *domain.Generation {
	t.Helper()
	if _, err := c.Reserve(hedgeGroup(), "pkg-1", domain.ProducerPackageDeleverSpot); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	entry, err := c.PremiseFor(hedgeGroup(), nil)
	if err != nil {
		t.Fatalf("entry PremiseFor: %v", err)
	}
	g, err := domain.OpenGeneration("gen-1", hedgeGroup(), "pkg-1", qty(t, target), entry)
	if err != nil {
		t.Fatalf("OpenGeneration: %v", err)
	}
	return g
}

// assertIntact is the premise's verdict plus its residual, because "intact" is
// two claims and asserting only the boolean would let a premise that was
// Determined at the wrong number pass.
func assertPremise(t *testing.T, p domain.Premise, class domain.Classification, residual string) {
	t.Helper()
	if p.Class != class {
		t.Fatalf("premise class = %s, want %s (residual %s)", p.Class, class, p.Residual)
	}
	if got := p.Residual.Decimal(); got != qtyDecimal(t, residual) {
		t.Fatalf("premise residual = %s, want %s", p.Residual, residual)
	}
}

// TestTheGenerationsOwnPerpLegDoesNotDissolveItsOwnPremise is FM-9, and it is the
// load-bearing test of this slice.
//
// Model (a)'s control NC22 is the v0.9.25 sentence read literally — "the §2
// classifier leaves Determined for this hedge group" — and TLC reaches its abort
// FOUR STATES FROM INIT with nothing realized, nothing outstanding and no
// external event whatsoever: a generation opened on a genuinely, physically flat
// package and cancelled one step later by its own machinery.
//
// So this asserts BOTH readings on the same state, because asserting only the
// premise would be satisfied by a classifier that had simply stopped noticing the
// perp leg. The raw verdict MUST be Ambiguous — §7 depends on it, since that is
// how §2 refuses every compensating trade while the leg is live — and the premise
// MUST be Determined at zero. The two disagreeing is the whole finding.
func TestTheGenerationsOwnPerpLegDoesNotDissolveItsOwnPremise(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t, neutralPackage(t)...)
	generation := openGeneration(t, classifier, "0.5")

	// The generation submits its OWN perp leg: a buy-to-reduce of the short,
	// live on the venue and therefore contributing an interval to the very sum
	// §2 classifies.
	perpLeg := mintedID(t, 3)
	if err := generation.SubmitPerpReduction(perpLeg, premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	orders.records = append(orders.records,
		leg(t, 3, perp(), domain.SideBuy, "0.5", "0", domain.StateAcknowledged))

	raw := classify(t, classifier)
	if raw.Class != domain.ClassificationAmbiguous {
		t.Fatalf("raw class = %s, want ambiguous — §7 DEPENDS on its own leg making the group ambiguous", raw.Class)
	}

	premise := premiseFor(t, classifier, generation)
	assertPremise(t, premise, domain.ClassificationDetermined, "0")
	intact, err := premise.Intact()
	if err != nil {
		t.Fatalf("Intact: %v", err)
	}
	if !intact {
		t.Fatalf("premise dissolved on the generation's own first step — this is NC22")
	}
}

// TestThePremiseIsInvariantUnderTheGenerationsOwnLadder is model (a)'s action
// property P6 — `[][PdOwnStep => PremiseResidual' = PremiseResidual]_vars` —
// stated as a walk.
//
// That invariance is the REASON the netted reading is the right one, and FM-7 is
// why it is a run rather than a sentence: a justification recorded once and never
// re-derived is what a later slice reads as settled. Every step of the ladder is
// taken here, each with its ledger amendment, and the premise is re-read after
// each.
//
// The package underneath is never touched, so a premise that moved would have
// been moved by the generation itself.
func TestThePremiseIsInvariantUnderTheGenerationsOwnLadder(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t, neutralPackage(t)...)
	generation := openGeneration(t, classifier, "0.5")
	assertPremise(t, premiseFor(t, classifier, generation), domain.ClassificationDetermined, "0")

	amend := func(step string) {
		t.Helper()
		intent, err := generation.OutstandingIntent()
		if err != nil {
			t.Fatalf("%s: OutstandingIntent: %v", step, err)
		}
		if _, err := classifier.Amend(hedgeGroup(), "pkg-1", intent); err != nil {
			t.Fatalf("%s: Amend: %v", step, err)
		}
		assertPremise(t, premiseFor(t, classifier, generation), domain.ClassificationDetermined, "0")
	}

	// Submit the perp leg. Its interval enters the group's sum.
	perpLeg := mintedID(t, 3)
	if err := generation.SubmitPerpReduction(perpLeg, premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	orders.records = append(orders.records,
		leg(t, 3, perp(), domain.SideBuy, "0.5", "0", domain.StateAcknowledged))
	amend("submit perp")

	// It part-fills 0.3. Realized rises by 0.3 and the entry is amended by the
	// same 0.3 — the two movements that cancel in BOTH subtractions.
	if err := generation.RecordPerpFill(qty(t, "0.3")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	orders.records[2] = leg(t, 3, perp(), domain.SideBuy, "0.5", "0.3", domain.StatePartiallyFilled)
	amend("perp part-fill")

	// The leg reaches finality with 0.3 confirmed of 0.5 targeted — the partial
	// confirmation "no more spot than the CONFIRMED perp reduction" sizes from.
	orders.records[2] = leg(t, 3, perp(), domain.SideBuy, "0.5", "0.3", domain.StateCanceled)
	if err := generation.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	amend("perp finality")

	// Authorize a spot sale STRICTLY BELOW the confirmed reduction. This is the
	// step at which the entry and the transient part company — §54f's refuted
	// derivation, the one that would have made this netting look like a no-op —
	// so a premise computed from the entry alone diverges here and this test is
	// where it shows.
	if err := generation.AuthorizeSpotReduction(qty(t, "0.2"), premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	amend("authorize spot")

	spotLeg := mintedID(t, 4)
	if err := generation.SubmitSpotReduction(spotLeg); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}
	orders.records = append(orders.records,
		leg(t, 4, spot(), domain.SideSell, "0.2", "0", domain.StateAcknowledged))
	amend("submit spot")

	if err := generation.ConfirmSpotReduction(qty(t, "0.2")); err != nil {
		t.Fatalf("ConfirmSpotReduction: %v", err)
	}
	orders.records[3] = leg(t, 4, spot(), domain.SideSell, "0.2", "0.2", domain.StateFilled)
	released, _, err := classifier.Release(hedgeGroup(), "pkg-1")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if got := released.Decimal(); got != qtyDecimal(t, "0.2") {
		t.Fatalf("released = %s, want 0.2", released)
	}
	assertPremise(t, premiseFor(t, classifier, generation), domain.ClassificationDetermined, "0")

	// And the PACKAGE is now genuinely deleveraged rather than merely reported
	// so: the perp short went from 1 to 0.7 and the spot long from 1 to 0.8, so
	// the group holds a real net long of 0.1 that the generation deliberately did
	// not sell back. Without this the walk above would be equally satisfied by a
	// ladder that moved nothing at all.
	final := classify(t, classifier)
	if final.Class != domain.ClassificationDetermined {
		t.Fatalf("final class = %s, want determined", final.Class)
	}
	if got := final.Residual.Decimal(); got != qtyDecimal(t, "0.1") {
		t.Fatalf("final residual = %s, want 0.1 — the ladder must have MOVED the package", final.Residual)
	}
}

// TestThePremiseDissolvesWhenSOMETHINGELSEMovesThePackage is the positive control
// for the test above, and without it "invariant under the generation's own steps"
// and "a quantity that never moves" are the same measurement.
//
// The external event is §6.7 row 4's own route, which is what makes FM-8 real
// rather than hypothetical: a reduction whose OWN result came back ambiguous
// "never blind-retries — its own ambiguous result routes through this same §6.7
// finality contract", releasing its ledger entry and re-entering the exposure as
// an unresolved order. Nothing tells the generation, and the generation is then
// running on a premise nobody re-checked.
func TestThePremiseDissolvesWhenSomethingElseMovesThePackage(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t, neutralPackage(t)...)
	generation := openGeneration(t, classifier, "0.5")

	perpLeg := mintedID(t, 3)
	if err := generation.SubmitPerpReduction(perpLeg, premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	orders.records = append(orders.records,
		leg(t, 3, perp(), domain.SideBuy, "0.5", "0.5", domain.StateFilled))
	if err := generation.RecordPerpFill(qty(t, "0.5")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	intent, err := generation.OutstandingIntent()
	if err != nil {
		t.Fatalf("OutstandingIntent: %v", err)
	}
	if _, err := classifier.Amend(hedgeGroup(), "pkg-1", intent); err != nil {
		t.Fatalf("Amend: %v", err)
	}
	assertPremise(t, premiseFor(t, classifier, generation), domain.ClassificationDetermined, "0")

	// Something OUTSIDE the generation moves the package: an unrelated spot sale
	// executes against the account. The generation's own legs are unchanged.
	orders.records = append(orders.records,
		leg(t, 5, spot(), domain.SideSell, "0.4", "0.4", domain.StateFilled))

	premise := premiseFor(t, classifier, generation)
	assertPremise(t, premise, domain.ClassificationDetermined, "-0.4")
	intact, err := premise.Intact()
	if err != nil {
		t.Fatalf("Intact: %v", err)
	}
	if intact {
		t.Fatalf("premise still intact after an external 0.4 sale — the standing trigger can never fire")
	}

	// FM-8's obligation: the generation MUST NOT take a further step that
	// commits new exposure, and the refusal names the premise rather than the
	// phase.
	if err := generation.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	err = generation.AuthorizeSpotReduction(qty(t, "0.5"), premiseFor(t, classifier, generation))
	if !errors.Is(err, domain.ErrGenerationPremiseDissolved) {
		t.Fatalf("AuthorizeSpotReduction = %v, want ErrGenerationPremiseDissolved", err)
	}

	// And it MUST be superseded rather than merely halted. The reserved second
	// leg was never submitted, so there is nothing to quiesce and the row fires
	// immediately.
	if err := generation.Supersede(domain.SupersededByPremiseLost); err != nil {
		t.Fatalf("Supersede: %v", err)
	}
	if generation.Phase() != domain.GenerationSuperseded {
		t.Fatalf("phase = %s, want superseded", generation.Phase())
	}
}

// TestADissolvedPremiseDoesNotStrandAnAlreadyAuthorizedLeg is the boundary of
// FM-8's standing obligation, and it is here because the claim cannot be
// mutated: SubmitSpotReduction takes no premise, so "this step is not guarded"
// is structural and a one-line mutation cannot express its negation. What CAN be
// written is the behaviour that adding the guard would break, so that is what
// this asserts.
//
// The boundary was found by a RUN rather than by design, and it was the nominal
// liveness configuration that found it. With the send guarded, TLC returned a
// 16-state behaviour ending in STUTTERING: a generation buys perp, authorizes a
// spot sale, the premise dissolves, and the generation sits at
// SpotReduceAuthorized unable to finish — its entry in the shared ledger,
// nothing on the wire, and no path forward. Refusing a step that commits nothing
// does not prevent a commitment; it STRANDS one, which is the harm the
// quiesce-first rule exists to forbid, arriving one step earlier.
func TestADissolvedPremiseDoesNotStrandAnAlreadyAuthorizedLeg(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t, neutralPackage(t)...)
	generation := openGeneration(t, classifier, "0.5")

	perpLeg := mintedID(t, 3)
	if err := generation.SubmitPerpReduction(perpLeg, premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	orders.records = append(orders.records,
		leg(t, 3, perp(), domain.SideBuy, "0.5", "0.5", domain.StateFilled))
	if err := generation.RecordPerpFill(qty(t, "0.5")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := generation.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := generation.AuthorizeSpotReduction(qty(t, "0.5"), premiseFor(t, classifier, generation)); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	intent, err := generation.OutstandingIntent()
	if err != nil {
		t.Fatalf("OutstandingIntent: %v", err)
	}
	if _, err := classifier.Amend(hedgeGroup(), "pkg-1", intent); err != nil {
		t.Fatalf("Amend: %v", err)
	}

	// Now something external moves the package, so the premise is genuinely
	// dissolved — asserted, because a test whose premise happened to stay intact
	// would demonstrate nothing at all about the guard.
	orders.records = append(orders.records,
		leg(t, 5, spot(), domain.SideSell, "0.3", "0.3", domain.StateFilled))
	dissolved, err := premiseFor(t, classifier, generation).Intact()
	if err != nil {
		t.Fatalf("Intact: %v", err)
	}
	if dissolved {
		t.Fatalf("the premise is still intact — this fixture proves nothing")
	}

	// The already-authorized leg still goes out and still reaches finality.
	spotLeg := mintedID(t, 4)
	if err := generation.SubmitSpotReduction(spotLeg); err != nil {
		t.Fatalf("SubmitSpotReduction on a dissolved premise: %v — this is the strand", err)
	}
	if err := generation.ConfirmSpotReduction(qty(t, "0.5")); err != nil {
		t.Fatalf("ConfirmSpotReduction: %v", err)
	}
	if generation.Phase() != domain.GenerationComplete {
		t.Fatalf("phase = %s, want complete", generation.Phase())
	}
}

// TestPremiseNettingRefusesWhenTwoGenerationsShareTheProducer is the guard on the
// netting's own precondition.
//
// The decomposition's only handle is the PRODUCER, so with two PackageDelever
// entries the netting removes a SIBLING's entry as well and the premise contains
// another generation's subject — FM-9's shape arriving through the
// decomposition. SM §7 forbids two live generations, which is exactly why this
// is checked here: a rule stated in another section and relied on here is a
// premise nobody re-derives.
func TestPremiseNettingRefusesWhenTwoGenerationsShareTheProducer(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t, neutralPackage(t)...)
	generation := openGeneration(t, classifier, "0.5")
	if _, err := classifier.Reserve(hedgeGroup(), "pkg-2", domain.ProducerPackageDeleverSpot); err != nil {
		t.Fatalf("Reserve second: %v", err)
	}

	_, err := classifier.PremiseFor(hedgeGroup(), generation.OwnLegs())
	if !errors.Is(err, application.ErrPremiseNettingAmbiguous) {
		t.Fatalf("PremiseFor = %v, want ErrPremiseNettingAmbiguous", err)
	}

	// A second entry from a DIFFERENT producer is fine and must stay fine: the
	// interval floor and the generation coexist by design, and a guard that
	// counted every entry would refuse the ordinary case. That is the
	// distinction this fixture exists to split.
	if _, _, err := classifier.Release(hedgeGroup(), "pkg-2"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := classifier.Reserve(hedgeGroup(), "floor-1", domain.ProducerIntervalFloor); err != nil {
		t.Fatalf("Reserve floor: %v", err)
	}
	if _, err := classifier.PremiseFor(hedgeGroup(), generation.OwnLegs()); err != nil {
		t.Fatalf("PremiseFor with a floor entry: %v", err)
	}
}

// TestPremiseRefusesAnUnidentifiedLeg — an empty id matches no record, so the leg
// stays INSIDE the quantity that decides whether the generation may continue.
// The refusal is what stops that being silent.
func TestPremiseRefusesAnUnidentifiedLeg(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t, neutralPackage(t)...)
	if _, err := classifier.Reserve(hedgeGroup(), "pkg-1", domain.ProducerPackageDeleverSpot); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	_, err := classifier.PremiseFor(hedgeGroup(), []ids.ClientOrderID{""})
	if !errors.Is(err, application.ErrPremiseLegUnidentified) {
		t.Fatalf("PremiseFor = %v, want ErrPremiseLegUnidentified", err)
	}
}
