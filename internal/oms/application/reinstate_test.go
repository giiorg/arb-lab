package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The reinstatement's fixtures, and the collapses the checklist found in them
// BEFORE any of these tests were trusted.
//
//  1. TARGET vs CONFIRMED PERP REDUCTION. The entry a pre-authorization
//     generation carries is `perpDone`, and booking the TARGET instead is FM-5
//     itself. A fixture whose perp leg executed its whole target makes the two
//     numbers equal, so every test here uses target 1.0 with 0.4 executed.
//  2. AUTHORIZED vs CONFIRMED. From the authorization onward the entry carries
//     the AUTHORIZED size, which §7 permits to be strictly less than the
//     confirmed perp reduction ("no MORE spot than", where "no more" and
//     "exactly" are different predicates). So the authorized fixtures use 0.25
//     against 0.4 confirmed — equal values would make the two rules one.
//  3. GENERATION CARDINALITY. With one generation, "this generation's entry"
//     and "the only entry" are the same quantity, and a reinstatement that
//     reserved under a fixed id would pass. The two-generation case uses two
//     hedge groups (SM §7 forbids two on one) with DIFFERENT reservations and
//     different outstanding amounts.
//  4. THE STEP SEQUENCE. A rebuild that ignored the recorded order and drove a
//     canonical path to the final phase passes every fixture whose steps are
//     already canonical. The isolating fixture is a log whose steps SKIP the
//     perp submission, which no path reaches and which the edge set refuses.
//  5. THE TRACK. newDeleverer tracks the hedge group, so Restore's
//     track-if-untracked branch never runs in the delever suite. The fixture
//     here takes `tracked` as a parameter and both values are exercised.
//  6. THE BAR's PATH. Two paths do not resume and one does, so a bar keyed off
//     "a supersession happened" and one keyed off the PATH differ only when
//     both classes appear. Both external rows are covered, and the premise-lost
//     row is covered as the refusal.

// reinstateFixture is the reinstatement over a REAL ladder.
//
// The Deleverer is not doubled, and that is the point: the claim under test is
// that a generation's entry is back in the SHARED reserved-reduction ledger, and
// a fake ladder could only show that a method was called. What makes the claim
// checkable is reading the ledger through the classifier afterwards — the same
// surface §6.7's determined branch sizes from.
type reinstateFixture struct {
	t          *testing.T
	reinstater *application.Reinstater
	deleverer  *application.Deleverer
	classifier *application.Classifier
	orders     *fakeOrders
	group      *fakeGroupOrders
	journal    *fakeGenerationJournal
}

func newReinstater(t *testing.T, tracked bool, records ...ports.OrderRecord) *reinstateFixture {
	t.Helper()

	groupOrders := &fakeGroupOrders{records: records}
	classifier, err := application.NewClassifier(groupOrders)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if tracked {
		if err := classifier.Track(hedgeGroup(), "BTC"); err != nil {
			t.Fatalf("Track: %v", err)
		}
	}

	oj := newFakeJournal()
	placer := &fakePlacer{watch: oj}
	orders := newFakeOrders()
	dispatcher, err := application.NewDispatcher(oj, placer, orders, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	gj := newFakeGenerationJournal(placer)
	d, err := application.NewDeleverer(dispatcher, classifier, gj)
	if err != nil {
		t.Fatalf("NewDeleverer: %v", err)
	}
	r, err := application.NewReinstater(d, orders)
	if err != nil {
		t.Fatalf("NewReinstater: %v", err)
	}
	for _, rec := range records {
		orders.records[rec.Order.ClientOrderID] = rec
	}
	return &reinstateFixture{
		t: t, reinstater: r, deleverer: d, classifier: classifier,
		orders: orders, group: groupOrders, journal: gj,
	}
}

// reserved reads the group's outstanding_reserved_reductions back out of the
// ledger — the quantity §6.7's determined branch subtracts, and the one the
// whole phase exists to restore.
func (f *reinstateFixture) reserved(group domain.HedgeGroup) fixedpoint.Quantity {
	f.t.Helper()
	verdict, err := f.classifier.Classify(group)
	if err != nil {
		f.t.Fatalf("Classify(%s): %v", group, err)
	}
	return verdict.Reserved
}

func wantQuantity(t *testing.T, got fixedpoint.Quantity, want string, what string) {
	t.Helper()
	cmp, err := got.Cmp(qty(t, want))
	if err != nil {
		t.Fatalf("%s: Cmp: %v", what, err)
	}
	if cmp != 0 {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

// livePerpGeneration is the crash state FLAG-3's durability exists for: a
// generation whose perp leg is out and partially executed, with no ending
// record.
//
// The executed quantity is planted in the ORDER repository and not in the
// generation record, because it is not in either journal — see
// Deleverer.RecordPerpFill. That is the whole reason this phase runs after the
// venue join, and a fixture carrying it on the generation would test a system
// that does not exist.
func livePerpGeneration(t *testing.T, perpOrder ids.ClientOrderID) application.GenerationRecovery {
	t.Helper()
	return application.GenerationRecovery{
		ID:          "GEN-A",
		Group:       hedgeGroup(),
		Reservation: "RES-A",
		Target:      qty(t, "1"),
		PerpOrder:   perpOrder,
		Steps:       []domain.GenerationPhase{domain.GenerationPerpReduceInFlight},
	}
}

// TestReinstatingAGenerationPutsItsEntryBackInTheSharedLedger is the slice's
// central claim, and it is asserted on the LEDGER rather than on the aggregate.
//
// A rebuild that produced a perfect generation and reserved nothing would leave
// §6.7's subtrahend short by exactly this generation's outstanding net intent,
// which reads as MORE room and is invisible to every invariant computed from
// that ledger. So the assertion is Verdict.Reserved: the number the determined
// branch actually subtracts.
func TestReinstatingAGenerationPutsItsEntryBackInTheSharedLedger(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	f := newReinstater(t, true, perp)

	rec := application.Recovery{
		LiveGenerations: []application.GenerationRecovery{
			livePerpGeneration(t, perp.Order.ClientOrderID),
		},
	}
	got, err := f.reinstater.Reinstate(rec)
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}

	// 0.4 rather than 1 is the FM-5 distinction: the entry carries what the
	// generation has COMMITTED and not yet realized, never the target it merely
	// intends.
	wantQuantity(t, f.reserved(hedgeGroup()), "0.4", "outstanding_reserved_reductions")

	if len(got.Restored) != 1 {
		t.Fatalf("Restored = %v, want one entry", got.Restored)
	}
	one := got.Restored[0]
	if one.Generation != "GEN-A" || one.Group != hedgeGroup() {
		t.Errorf("Restored[0] names %q on %s, want GEN-A on %s", one.Generation, one.Group, hedgeGroup())
	}
	if one.Phase != domain.GenerationPerpReduceInFlight {
		t.Errorf("Restored[0].Phase = %s, want %s", one.Phase, domain.GenerationPerpReduceInFlight)
	}
	wantQuantity(t, one.Outstanding, "0.4", "Restored[0].Outstanding")

	// The aggregate is in the ladder, at the phase the log left it, with the
	// venue's executed quantity as its confirmed perp reduction. Asserted beside
	// the ledger because the two are what "exactly backed at every instant" is a
	// claim about, and a rebuild that got one right and the other wrong is the
	// disagreement §7's invariant forbids.
	g, ok := f.deleverer.Generation(hedgeGroup())
	if !ok {
		t.Fatalf("Generation(%s) = _, false; want the reinstated generation", hedgeGroup())
	}
	if g.ID() != "GEN-A" {
		t.Errorf("Generation().ID = %q, want GEN-A", g.ID())
	}
	wantQuantity(t, g.PerpConfirmed(), "0.4", "PerpConfirmed")
	wantQuantity(t, g.Target(), "1", "Target")
	if !one.Reassessment.Intact {
		t.Errorf("Reassessment = %+v, want an intact premise", one.Reassessment)
	}
}

// TestAnAuthorizedGenerationCarriesTheAUTHORIZEDSize is the second arm of
// OutstandingIntent, and the fixture is what keeps it from collapsing into the
// first.
//
// The authorization is 0.25 against a confirmed perp reduction of 0.4, so
// "carries the authorized size" and "carries the confirmed reduction" are two
// different numbers. Equal ones would make this test pass against a rebuild
// that never read the authorization record at all.
func TestAnAuthorizedGenerationCarriesTheAuthorizedSize(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	f := newReinstater(t, true, perp)

	gen := livePerpGeneration(t, perp.Order.ClientOrderID)
	gen.Authorized = qty(t, "0.25")
	gen.Steps = []domain.GenerationPhase{
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
		domain.GenerationSpotReduceAuthorized,
	}

	got, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{gen},
	})
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	wantQuantity(t, f.reserved(hedgeGroup()), "0.25", "outstanding_reserved_reductions")
	wantQuantity(t, got.Restored[0].Outstanding, "0.25", "Restored[0].Outstanding")
	if got.Restored[0].Phase != domain.GenerationSpotReduceAuthorized {
		t.Errorf("Phase = %s, want %s", got.Restored[0].Phase, domain.GenerationSpotReduceAuthorized)
	}
}

// TestTheReinstatementRefusesAnAuthorizationTheVenueCannotBack is the sharpest
// check the rebuild performs, and it exists because the two facts come from
// DIFFERENT sources.
//
// The authorization is the journal's; the confirmed perp reduction is the
// venue's. §7's never-net-short cap says the first may not exceed the second,
// and a log claiming otherwise is a contradiction between two authorities. The
// refusal is the safe direction: admitting it would put an entry into the shared
// ledger claiming a sale this generation could never back.
func TestTheReinstatementRefusesAnAuthorizationTheVenueCannotBack(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.3")
	f := newReinstater(t, true, perp)

	gen := livePerpGeneration(t, perp.Order.ClientOrderID)
	gen.Authorized = qty(t, "0.5")
	gen.Steps = []domain.GenerationPhase{
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
		domain.GenerationSpotReduceAuthorized,
	}

	_, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{gen},
	})
	if !errors.Is(err, domain.ErrGenerationSpotExceedsConfirmedPerp) {
		t.Fatalf("Reinstate = %v, want ErrGenerationSpotExceedsConfirmedPerp", err)
	}
	// And the ledger is untouched: a refused reinstatement must not leave a
	// reservation behind, because an entry with no generation subtracts from
	// every later sizing for the life of the process.
	if f.classifier.Tracked(hedgeGroup()) {
		wantQuantity(t, f.reserved(hedgeGroup()), "0", "outstanding_reserved_reductions after a refusal")
	}
}

// TestTheRecordedStepOrderIsReplayed is the isolating fixture for "the steps are
// driven in the order the log wrote them".
//
// Every other fixture here carries a canonical sequence, so a rebuild that
// ignored the order and drove the shortest path to the final phase would pass
// all of them. This log SKIPS the perp submission — an authorization with no
// preceding leg, which is not an edge of SM §7's ladder and which the
// transition table refuses. A rebuild that jumped to the final phase would
// accept it, and would reinstate a generation asserting a spot sale backed by a
// perp reduction that was never submitted.
func TestTheRecordedStepOrderIsReplayed(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	f := newReinstater(t, true, perp)

	gen := livePerpGeneration(t, perp.Order.ClientOrderID)
	gen.Authorized = qty(t, "0.25")
	gen.Steps = []domain.GenerationPhase{domain.GenerationSpotReduceAuthorized}

	_, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{gen},
	})
	if !errors.Is(err, domain.ErrGenerationTransitionForbidden) {
		t.Fatalf("Reinstate = %v, want ErrGenerationTransitionForbidden", err)
	}
}

// TestTheReinstatementRefusesAStepNoLiveGenerationCanCarry covers the switch's
// two refusing arms, which no journal this system writes can reach.
//
// They are tested by CONSTRUCTING the value rather than by writing a log,
// because that is the only way in: the walk that produces a Recovery puts an
// ended generation in neither list, and `Planned` is the seed. A refusal
// nothing can reach is still a refusal that must be right on the day something
// can — and mutating either arm to fall through is otherwise a survivor.
func TestTheReinstatementRefusesAStepNoLiveGenerationCanCarry(t *testing.T) {
	t.Parallel()

	for _, step := range []domain.GenerationPhase{
		domain.GenerationPlanned,
		domain.GenerationComplete,
		domain.GenerationTerminalFlatten,
		domain.GenerationSuperseded,
	} {
		t.Run(step.String(), func(t *testing.T) {
			t.Parallel()
			perp := perpLeg(t, 0, "1")
			f := newReinstater(t, true, perp)
			gen := livePerpGeneration(t, perp.Order.ClientOrderID)
			gen.Steps = []domain.GenerationPhase{step}

			_, err := f.reinstater.Reinstate(application.Recovery{
				LiveGenerations: []application.GenerationRecovery{gen},
			})
			if !errors.Is(err, application.ErrGenerationStepUnreachable) {
				t.Fatalf("Reinstate with step %s = %v, want ErrGenerationStepUnreachable", step, err)
			}
		})
	}
}

// TestTheReinstatementTracksAnUntrackedHedgeGroup covers Restore's
// track-if-untracked branch, which the delever suite cannot reach because its
// own fixture tracks.
//
// At §7.5 boot nothing has tracked anything — Classifier.Track had no
// non-test caller at all before this slice — so the untracked case is the
// ORDINARY one and the tracked case is the fixture's convenience. Both run here
// for that reason.
func TestTheReinstatementTracksAnUntrackedHedgeGroup(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	f := newReinstater(t, false, perp)
	if f.classifier.Tracked(hedgeGroup()) {
		t.Fatalf("fixture group is already tracked; this test asserts the UNTRACKED path")
	}

	if _, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{
			livePerpGeneration(t, perp.Order.ClientOrderID),
		},
	}); err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	if !f.classifier.Tracked(hedgeGroup()) {
		t.Errorf("Tracked(%s) = false after a reinstatement, want true", hedgeGroup())
	}
	// The asset came from the generation's own target, and the evidence is that
	// the ledger can do arithmetic in it at all: an untagged zero fails its first
	// operation, so a group tracked with the wrong asset cannot produce a
	// verdict.
	wantQuantity(t, f.reserved(hedgeGroup()), "0.4", "outstanding_reserved_reductions")
}

// TestTwoGenerationsAreReinstatedUnderTheirOWNReservations is the cardinality
// dimension.
//
// With one generation, "this generation's entry" and "the only entry" are the
// same quantity and the same key, so a rebuild that reserved under a constant id
// or amended every entry to one amount would pass. Two groups, two reservations,
// two different outstanding amounts — and SM §7 forbids two live generations on
// one group, so two GROUPS is the only honest way to vary it.
func TestTwoGenerationsAreReinstatedUnderTheirOwnReservations(t *testing.T) {
	t.Parallel()

	first := perpLeg(t, 0, "1")
	first.Cumulative = qty(t, "0.4")
	second := perpLeg(t, 1, "1")
	second.Cumulative = qty(t, "0.7")
	// Planted in the ORDER repository alone, not in the classifier's fold. The
	// fold's double answers every group with the same records, so a leg planted
	// there would enter the OTHER generation's premise as somebody else's filled
	// buy — dissolving it, superseding the generation and releasing the very
	// entry this test is about. The realism that matters here is that the
	// executed quantity comes from the projection, which is what the rebuild
	// reads.
	f := newReinstater(t, true)
	f.orders.records[first.Order.ClientOrderID] = first
	f.orders.records[second.Order.ClientOrderID] = second

	other := domain.HedgeGroup{Account: "sub-2", Venue: "binance", Symbol: "BTCUSDT"}
	genA := livePerpGeneration(t, first.Order.ClientOrderID)
	genB := application.GenerationRecovery{
		ID:          "GEN-B",
		Group:       other,
		Reservation: "RES-B",
		Target:      qty(t, "1"),
		PerpOrder:   second.Order.ClientOrderID,
		Steps:       []domain.GenerationPhase{domain.GenerationPerpReduceInFlight},
	}

	got, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{genA, genB},
	})
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	if len(got.Restored) != 2 {
		t.Fatalf("Restored = %v, want two entries", got.Restored)
	}
	wantQuantity(t, got.Restored[0].Outstanding, "0.4", "GEN-A outstanding")
	wantQuantity(t, got.Restored[1].Outstanding, "0.7", "GEN-B outstanding")
	wantQuantity(t, f.reserved(hedgeGroup()), "0.4", "sub-1 reserved")
	wantQuantity(t, f.reserved(other), "0.7", "sub-2 reserved")
}

// TestAReinstatedGenerationWithADissolvedPremiseIsSUPERSEDED is FM-8's standing
// obligation reached through recovery.
//
// The premise is a claim about the world NOW, and the world moved while the
// process was down. domain.RestoreGeneration deliberately does not re-ask it —
// refusing to rebuild would leave the entry out of the ledger, which is the hole
// this whole phase fills — so the obligation is discharged one step later, by
// the reassessment. This asserts the discharge HAPPENS: a reinstatement that
// restored and did not reassess leaves a generation running on a premise that
// has gone, with nothing obliged to end it (model (a)'s NC23).
func TestAReinstatedGenerationWithADissolvedPremiseIsSuperseded(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	// Somebody else's filled sale: the group is Determined and genuinely short,
	// which dissolves the premise through its RESIDUAL alone.
	f := newReinstater(t, true, perp, externalSale(t, 9, "0.2"))

	gen := livePerpGeneration(t, perp.Order.ClientOrderID)
	gen.Steps = []domain.GenerationPhase{
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
	}

	got, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{gen},
	})
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	one := got.Restored[0]
	if !one.Reassessment.Superseded {
		t.Fatalf("Reassessment = %+v, want the premise-lost row to have fired", one.Reassessment)
	}
	if !one.Reassessment.ResumesLadder {
		t.Errorf("ResumesLadder = false for a premise-lost supersession, want true")
	}
	// Superseding RELEASES the entry rather than amending it to zero, so the
	// group's reserved sum is back to nothing and §2 can see the naked residual
	// the generation left behind.
	wantQuantity(t, f.reserved(hedgeGroup()), "0", "outstanding_reserved_reductions after the supersession")
}

// TestAReinstatedGenerationWithALiveReservedLegAwaitsQuiesce is the OTHER arm of
// the reassessment, and it is what makes the boot gate report Incomplete.
//
// The premise has dissolved and the reserved SECOND leg is still out, so all
// three of §7's supersession rows require it to reach §6.7 finality FIRST. The
// obligation is owed and nothing at boot can discharge it — finality is the
// venue's to give — so the generation stays live, its entry stays in the ledger,
// and the gate refuses to pass.
func TestAReinstatedGenerationWithALiveReservedLegAwaitsQuiesce(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	spotOrder := spotLeg(t, 1, "0.25")
	f := newReinstater(t, true, perp, spotOrder, externalSale(t, 9, "0.2"))

	gen := livePerpGeneration(t, perp.Order.ClientOrderID)
	gen.Authorized = qty(t, "0.25")
	gen.SpotOrder = spotOrder.Order.ClientOrderID
	gen.Steps = []domain.GenerationPhase{
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
		domain.GenerationSpotReduceAuthorized,
		domain.GenerationSpotReduceInFlight,
	}

	got, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{gen},
	})
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	if got.AwaitingQuiesce != 1 {
		t.Fatalf("AwaitingQuiesce = %d, want 1 (reassessment %+v)", got.AwaitingQuiesce, got.Restored[0].Reassessment)
	}
	if got.Restored[0].Reassessment.Superseded {
		t.Errorf("the generation was superseded with its reserved leg live; §7 requires the quiesce first")
	}
	// The entry stays, carrying the authorized size: the generation is still
	// committed to that sale until its leg reaches finality.
	wantQuantity(t, f.reserved(hedgeGroup()), "0.25", "outstanding_reserved_reductions while quiescing")
}

// TestABarredLadderIsAppliedAndRefusesTheNextGeneration is ResumesLadder ACTED
// on, which is the half slice #203 named as a gap.
//
// Both external rows are covered, because the rule is a property of the PATH and
// a fixture using one leaves the other's arm dead. The refusal is asserted on
// Open — the ladder's own entry point — rather than on the bar's presence,
// because the bar existing and the bar BITING are different claims and only the
// second is what §7.1 asks for.
func TestABarredLadderIsAppliedAndRefusesTheNextGeneration(t *testing.T) {
	t.Parallel()

	for _, path := range []domain.SupersessionPath{
		domain.SupersededByEmergencyClose,
		domain.SupersededByFenceReadyLost,
	} {
		t.Run(path.String(), func(t *testing.T) {
			t.Parallel()
			f := newReinstater(t, true)

			got, err := f.reinstater.Reinstate(application.Recovery{
				BarredLadders: []application.LadderBar{{Group: hedgeGroup(), Path: path}},
			})
			if err != nil {
				t.Fatalf("Reinstate: %v", err)
			}
			if len(got.Barred) != 1 || got.Barred[0].Path != path {
				t.Fatalf("Barred = %v, want one %s", got.Barred, path)
			}

			_, err = f.deleverer.Open("GEN-NEXT", hedgeGroup(), "RES-NEXT", qty(t, "1"))
			if !errors.Is(err, application.ErrGenerationLadderBarred) {
				t.Fatalf("Open after a %s supersession = %v, want ErrGenerationLadderBarred", path, err)
			}
		})
	}
}

// TestAPremiseLostSupersessionDoesNOTBarTheLadder is the positive control the
// test above needs.
//
// Without it, a Deleverer that barred the ladder on EVERY supersession passes
// both cases above — and the cost of that mistake is the one ResumesLadder calls
// the unsafe direction: the mechanism that reduces leverage on a neutral package
// disabled for the rest of the process's life, at the moment it is needed.
func TestAPremiseLostSupersessionDoesNotBarTheLadder(t *testing.T) {
	t.Parallel()

	f := newReinstater(t, true)
	err := f.deleverer.BarLadder(hedgeGroup(), domain.SupersededByPremiseLost)
	if !errors.Is(err, application.ErrGenerationLadderResumes) {
		t.Fatalf("BarLadder(premise_lost) = %v, want ErrGenerationLadderResumes", err)
	}
	if _, err := f.deleverer.Open("GEN-NEXT", hedgeGroup(), "RES-NEXT", qty(t, "1")); err != nil {
		t.Errorf("Open after a premise-lost supersession = %v, want it admitted", err)
	}
}

// TestTheBarsAreAppliedBEFORETheRestores is the ordering claim, and the fixture
// is a log holding both for one group.
//
// That combination is a history this build cannot write — Open refuses a barred
// group — so what it demonstrates is which end catches the contradiction.
// Applying the restores first would reserve an entry and install a generation on
// a package an arbitration has closed, and then bar it: the contradiction
// accepted and recorded. Applying the bars first makes Restore refuse, which is
// the reading that produces no answer rather than a confident wrong one.
func TestTheBarsAreAppliedBeforeTheRestores(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	perp.Cumulative = qty(t, "0.4")
	f := newReinstater(t, true, perp)

	_, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{
			livePerpGeneration(t, perp.Order.ClientOrderID),
		},
		BarredLadders: []application.LadderBar{
			{Group: hedgeGroup(), Path: domain.SupersededByEmergencyClose},
		},
	})
	if !errors.Is(err, application.ErrGenerationLadderBarred) {
		t.Fatalf("Reinstate = %v, want ErrGenerationLadderBarred", err)
	}
	if _, ok := f.deleverer.Generation(hedgeGroup()); ok {
		t.Errorf("a generation was installed on a barred group")
	}
}

// TestRestoreRefusesAnEndedGeneration and its siblings cover Restore's three
// refusals directly, since the reinstatement's own walk cannot present two of
// them.
func TestRestoreRefusesAnEndedGeneration(t *testing.T) {
	t.Parallel()

	f := newReinstater(t, true)
	g, err := domain.RestoreGeneration("GEN-A", hedgeGroup(), "RES-A", qty(t, "1"))
	if err != nil {
		t.Fatalf("RestoreGeneration: %v", err)
	}
	if err := g.Supersede(domain.SupersededByEmergencyClose); err != nil {
		t.Fatalf("Supersede: %v", err)
	}
	if err := f.deleverer.Restore(g); !errors.Is(err, domain.ErrGenerationNotActive) {
		t.Fatalf("Restore(ended) = %v, want ErrGenerationNotActive", err)
	}
	if f.classifier.Tracked(hedgeGroup()) {
		wantQuantity(t, f.reserved(hedgeGroup()), "0", "reserved after refusing an ended generation")
	}
}

func TestRestoreRefusesASecondGenerationOnOneGroup(t *testing.T) {
	t.Parallel()

	f := newReinstater(t, true)
	first, err := domain.RestoreGeneration("GEN-A", hedgeGroup(), "RES-A", qty(t, "1"))
	if err != nil {
		t.Fatalf("RestoreGeneration: %v", err)
	}
	if err := f.deleverer.Restore(first); err != nil {
		t.Fatalf("Restore(first): %v", err)
	}
	second, err := domain.RestoreGeneration("GEN-B", hedgeGroup(), "RES-B", qty(t, "1"))
	if err != nil {
		t.Fatalf("RestoreGeneration: %v", err)
	}
	if err := f.deleverer.Restore(second); !errors.Is(err, application.ErrGenerationAlreadyLive) {
		t.Fatalf("Restore(second) = %v, want ErrGenerationAlreadyLive", err)
	}
}

func TestRestoreRefusesANilGeneration(t *testing.T) {
	t.Parallel()

	f := newReinstater(t, true)
	if err := f.deleverer.Restore(nil); !errors.Is(err, application.ErrGenerationMissing) {
		t.Fatalf("Restore(nil) = %v, want ErrGenerationMissing", err)
	}
}

func TestNewReinstaterRefusesAMissingCollaborator(t *testing.T) {
	t.Parallel()

	orders := newFakeOrders()
	if _, err := application.NewReinstater(nil, orders); !errors.Is(err, application.ErrGenerationLadderMissing) {
		t.Errorf("NewReinstater(nil, orders) = %v, want ErrGenerationLadderMissing", err)
	}
	f := newReinstater(t, true)
	if _, err := application.NewReinstater(f.deleverer, nil); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewReinstater(ladder, nil) = %v, want ErrOrdersMissing", err)
	}
}

// TestAZeroCumulativePerpLegIsNotRecordedAsAFill is the boundary the fixtures
// above all sit on the other side of.
//
// A leg that has executed nothing is the ordinary crash-immediately-after-submit
// state, and domain.Generation.RecordPerpFill refuses a non-positive fill —
// correctly, because a zero fill is not an event. So the reinstatement must SKIP
// it rather than pass it on, and the outstanding intent stays at the zero
// `Planned` already set. Without this case the skip is untested and a rebuild
// that always called RecordPerpFill fails only on a fixture nobody wrote.
func TestAZeroCumulativePerpLegIsNotRecordedAsAFill(t *testing.T) {
	t.Parallel()

	perp := perpLeg(t, 0, "1")
	f := newReinstater(t, true, perp)

	got, err := f.reinstater.Reinstate(application.Recovery{
		LiveGenerations: []application.GenerationRecovery{
			livePerpGeneration(t, perp.Order.ClientOrderID),
		},
	})
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	wantQuantity(t, got.Restored[0].Outstanding, "0", "Restored[0].Outstanding")
	wantQuantity(t, f.reserved(hedgeGroup()), "0", "outstanding_reserved_reductions")
	// An entry of zero is not the same as no entry: it holds the generation's
	// slot, which is what stops a second generation opening on the package.
	if _, err := f.deleverer.Open("GEN-B", hedgeGroup(), "RES-B", qty(t, "1")); err == nil {
		t.Errorf("Open succeeded against a live generation holding a zero entry")
	}
}

// fakeLadder is application.GenerationLadder for the RECONCILER's tests, where
// the claim under test is the boot gate rather than the ledger.
//
// The reinstatement's own suite uses the real *Deleverer, deliberately — a
// double could only show that a method was called, and what §7.5 requires is
// that a quantity is back in a shared ledger. This one exists so a boot-sequence
// test can name an outcome (a failure, an owed supersession) without building a
// classifier, a dispatcher and a placer to reach it.
type fakeLadder struct {
	restored []domain.GenerationID
	barred   []application.LadderBar

	// failRestore, when set, is returned by Restore — the hook for the gate's
	// OutcomeFailed arm.
	failRestore error

	// quiesce, when true, makes every Reassess report the owed supersession,
	// which is the gate's OutcomeIncomplete arm.
	quiesce bool
}

func newFakeLadder() *fakeLadder { return &fakeLadder{} }

func (l *fakeLadder) Restore(g *domain.Generation) error {
	if l.failRestore != nil {
		return l.failRestore
	}
	l.restored = append(l.restored, g.ID())
	return nil
}

func (l *fakeLadder) BarLadder(group domain.HedgeGroup, path domain.SupersessionPath) error {
	l.barred = append(l.barred, application.LadderBar{Group: group, Path: path})
	return nil
}

func (l *fakeLadder) Reassess(group domain.HedgeGroup) (application.Reassessment, error) {
	if l.quiesce {
		return application.Reassessment{Group: group, AwaitingQuiesce: true}, nil
	}
	return application.Reassessment{Group: group, Intact: true}, nil
}

var _ application.GenerationLadder = (*fakeLadder)(nil)

// TestBarLadderRefusesASecondCONTRADICTINGPath is
// domain.Generation.Supersede's first-writer-wins rule one scope out.
//
// A group whose ladder two different arbitrations claim to have closed is a log
// this build cannot write — the first bar refuses every later Open, so no second
// generation exists to be superseded — but which path an operator is SHOWN is
// decided here, and silently overwriting it would make the alert name whichever
// row was replayed last. The repeat of the SAME path is idempotent, which is
// what keeps a re-run of the boot from failing on its own previous work.
func TestBarLadderRefusesASecondContradictingPath(t *testing.T) {
	t.Parallel()

	f := newReinstater(t, true)
	if err := f.deleverer.BarLadder(hedgeGroup(), domain.SupersededByEmergencyClose); err != nil {
		t.Fatalf("BarLadder(first): %v", err)
	}
	// Idempotent for the same path.
	if err := f.deleverer.BarLadder(hedgeGroup(), domain.SupersededByEmergencyClose); err != nil {
		t.Errorf("BarLadder(same path again) = %v, want nil", err)
	}
	err := f.deleverer.BarLadder(hedgeGroup(), domain.SupersededByFenceReadyLost)
	if !errors.Is(err, application.ErrGenerationLadderBarred) {
		t.Fatalf("BarLadder(contradicting) = %v, want ErrGenerationLadderBarred", err)
	}
	// The FIRST path still stands, which is the half a refusal alone would not
	// show: a version that refused and overwrote anyway passes the assertion
	// above.
	_, e := f.deleverer.Open("GEN-NEXT", hedgeGroup(), "RES-NEXT", qty(t, "1"))
	if !errors.Is(e, application.ErrGenerationLadderBarred) {
		t.Fatalf("Open = %v, want ErrGenerationLadderBarred", e)
	}
	if !strings.Contains(e.Error(), domain.SupersededByEmergencyClose.String()) {
		t.Errorf("Open refusal names %v, want the FIRST path %s", e, domain.SupersededByEmergencyClose)
	}
}
