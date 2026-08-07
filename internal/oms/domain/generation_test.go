package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// allPhases is every declared phase. The tables below are asserted over ALL of
// them rather than over a sample, which is what makes a partition's claim
// checkable: a predicate tested on three members says nothing about the group
// the other seven fall into, and this repo's recorded trap is that a group
// reached by a fallthrough is untestable by construction.
var allPhases = []domain.GenerationPhase{
	domain.GenerationPlanned,
	domain.GenerationPerpReduceInFlight,
	domain.GenerationPerpReduceFinal,
	domain.GenerationSpotReduceAuthorized,
	domain.GenerationSpotReduceInFlight,
	domain.GenerationHeldPerp,
	domain.GenerationHeldSpot,
	domain.GenerationComplete,
	domain.GenerationTerminalFlatten,
	domain.GenerationSuperseded,
}

func generationGroup() domain.HedgeGroup {
	return domain.HedgeGroup{Account: "sub-1", Venue: "binance", Symbol: "BTCUSDT"}
}

func genQty(t *testing.T, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", v)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", v, err)
	}
	return q
}

// intactPremise is SM §7's entry precondition satisfied: a determined, flat
// package.
func intactPremise(t *testing.T) domain.Premise {
	t.Helper()
	return domain.Premise{Residual: genQty(t, "0"), Class: domain.ClassificationDetermined}
}

func genOrder(t *testing.T, leg int) ids.ClientOrderID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: leg})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	return id
}

func openAt(t *testing.T, target string) *domain.Generation {
	t.Helper()
	g, err := domain.OpenGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, target), intactPremise(t))
	if err != nil {
		t.Fatalf("OpenGeneration: %v", err)
	}
	return g
}

// TestGenerationPhasePartitions asserts all three partitions over all ten
// phases.
//
// The three are DIFFERENT functions of the same set and none is another's
// negation, which is the reason they are asserted together in one table: read
// down a column and the disagreements are visible. `HeldPerp` is active, has no
// live reserved leg and commits nothing; `SpotReduceInFlight` is active, HAS a
// live leg and commits nothing; `SpotReduceAuthorized` is active, has no live
// leg and DOES commit. Any two of those rows collapsing would mean one predicate
// had been written as another's negation, which is exactly the mistake
// ContributesPossibleDelta records one machine over.
func TestGenerationPhasePartitions(t *testing.T) {
	t.Parallel()

	want := map[domain.GenerationPhase]struct{ active, legLive, commits bool }{
		domain.GenerationPlanned:              {true, false, false},
		domain.GenerationPerpReduceInFlight:   {true, false, true},
		domain.GenerationPerpReduceFinal:      {true, false, false},
		domain.GenerationSpotReduceAuthorized: {true, false, true},
		domain.GenerationSpotReduceInFlight:   {true, true, false},
		domain.GenerationHeldPerp:             {true, false, false},
		domain.GenerationHeldSpot:             {true, true, false},
		domain.GenerationComplete:             {false, false, false},
		domain.GenerationTerminalFlatten:      {false, false, false},
		domain.GenerationSuperseded:           {false, false, false},
	}
	if len(want) != len(allPhases) {
		t.Fatalf("table has %d rows for %d phases", len(want), len(allPhases))
	}

	for _, phase := range allPhases {
		expect, ok := want[phase]
		if !ok {
			t.Fatalf("phase %s has no row", phase)
		}
		active, err := phase.IsActive()
		if err != nil {
			t.Fatalf("%s IsActive: %v", phase, err)
		}
		if active != expect.active {
			t.Errorf("%s IsActive = %v, want %v", phase, active, expect.active)
		}
		legLive, err := phase.ReservedLegLive()
		if err != nil {
			t.Fatalf("%s ReservedLegLive: %v", phase, err)
		}
		if legLive != expect.legLive {
			t.Errorf("%s ReservedLegLive = %v, want %v", phase, legLive, expect.legLive)
		}
		commits, err := phase.CommitsNewExposure()
		if err != nil {
			t.Fatalf("%s CommitsNewExposure: %v", phase, err)
		}
		if commits != expect.commits {
			t.Errorf("%s CommitsNewExposure = %v, want %v", phase, commits, expect.commits)
		}
	}
}

// TestGenerationPhasePartitionsRefuseUndeclaredValues keeps the unset and
// misspelt sentinels reachable. Without it the leading Validate in each
// partition could be deleted and every table above would still pass.
func TestGenerationPhasePartitionsRefuseUndeclaredValues(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		phase domain.GenerationPhase
		want  error
	}{
		{"", domain.ErrGenerationPhaseUnset},
		{"held", domain.ErrGenerationPhaseUnknown},
	} {
		if _, err := tc.phase.IsActive(); !errors.Is(err, tc.want) {
			t.Errorf("%q IsActive = %v, want %v", tc.phase, err, tc.want)
		}
		if _, err := tc.phase.ReservedLegLive(); !errors.Is(err, tc.want) {
			t.Errorf("%q ReservedLegLive = %v, want %v", tc.phase, err, tc.want)
		}
		if _, err := tc.phase.CommitsNewExposure(); !errors.Is(err, tc.want) {
			t.Errorf("%q CommitsNewExposure = %v, want %v", tc.phase, err, tc.want)
		}
	}
}

// TestGenerationLadderIsAssertedAsAMatrix answers all 100 ordered pairs.
//
// A sampled edge set is the shape that lets a WRONG edge survive, and two wrong
// edges here are things a plausible implementation has: `Planned →
// SpotReduceAuthorized` is model (a)'s control `PERP_FIRST = FALSE`, the edge
// that makes the transient net-SHORT in the rally the generation was opened for;
// `HeldPerp → Complete` is `HELD_EXIT_CAUSE_BLIND` (NC17), the reserved SPOT
// leg's barrier discharging a park whose PERP leg is the unresolved one, which
// produces a generation reporting §6.7 finality with its perp reduction still
// live. Both are absent, and a matrix is what says so.
func TestGenerationLadderIsAssertedAsAMatrix(t *testing.T) {
	t.Parallel()

	edges := map[domain.GenerationPhase][]domain.GenerationPhase{
		domain.GenerationPlanned:              {domain.GenerationPerpReduceInFlight},
		domain.GenerationPerpReduceInFlight:   {domain.GenerationPerpReduceFinal, domain.GenerationHeldPerp},
		domain.GenerationHeldPerp:             {domain.GenerationPerpReduceFinal, domain.GenerationTerminalFlatten},
		domain.GenerationPerpReduceFinal:      {domain.GenerationSpotReduceAuthorized},
		domain.GenerationSpotReduceAuthorized: {domain.GenerationSpotReduceInFlight, domain.GenerationComplete},
		domain.GenerationSpotReduceInFlight:   {domain.GenerationComplete, domain.GenerationHeldSpot},
		domain.GenerationHeldSpot:             {domain.GenerationComplete, domain.GenerationTerminalFlatten},
		domain.GenerationComplete:             nil,
		domain.GenerationTerminalFlatten:      nil,
		domain.GenerationSuperseded:           nil,
	}

	legal := func(from, to domain.GenerationPhase) bool {
		for _, p := range edges[from] {
			if p == to {
				return true
			}
		}
		return false
	}

	for _, from := range allPhases {
		for _, to := range allPhases {
			err := domain.CanAdvanceGeneration(from, to)
			switch {
			case from == to:
				if !errors.Is(err, domain.ErrGenerationNoOp) {
					t.Errorf("%s -> %s = %v, want ErrGenerationNoOp", from, to, err)
				}
			case legal(from, to):
				if err != nil {
					t.Errorf("%s -> %s = %v, want nil", from, to, err)
				}
			default:
				if !errors.Is(err, domain.ErrGenerationTransitionForbidden) {
					t.Errorf("%s -> %s = %v, want ErrGenerationTransitionForbidden", from, to, err)
				}
			}
		}
	}
}

// TestSupersessionPathsGiveOppositeLadderAnswers is SM §7's ladder-resumption
// invariant, and asserting both classes is the point: a function that answered
// "no" everywhere would satisfy an external-path-only test, and one that
// answered "yes" everywhere would satisfy a premise-lost-only test. The rule is
// only visible where the two disagree.
func TestSupersessionPathsGiveOppositeLadderAnswers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		path domain.SupersessionPath
		want bool
	}{
		{domain.SupersededByPremiseLost, true},
		{domain.SupersededByEmergencyClose, false},
		{domain.SupersededByFenceReadyLost, false},
	} {
		got, err := tc.path.ResumesLadder()
		if err != nil {
			t.Fatalf("%s ResumesLadder: %v", tc.path, err)
		}
		if got != tc.want {
			t.Errorf("%s ResumesLadder = %v, want %v", tc.path, got, tc.want)
		}
	}

	if _, err := domain.SupersessionPath("").ResumesLadder(); !errors.Is(err, domain.ErrSupersessionPathUnset) {
		t.Errorf("empty path = %v, want ErrSupersessionPathUnset", err)
	}
	if _, err := domain.SupersessionPath("kill").ResumesLadder(); !errors.Is(err, domain.ErrSupersessionPathUnknown) {
		t.Errorf("unknown path = %v, want ErrSupersessionPathUnknown", err)
	}
}

// TestOpenGenerationEnforcesTheStillNeutralPrecondition — core §5.3 defines
// `PackageDelever` as reducing leverage "on a STILL-NEUTRAL package", and §7's
// invariants say "the package has no naked residual". A generation opened over
// an unresolved or nonzero residual is a scenario §7 does not specify, and every
// invariant stated below is about a generation that opened under this guard.
//
// The two refusals are separate rows because they are separate failures: a
// nonzero residual is a package that is not flat, and an ambiguous verdict is a
// package whose flatness is not KNOWN. The second is the one a plausible
// implementation admits, since the residual reads zero.
func TestOpenGenerationEnforcesTheStillNeutralPrecondition(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		premise domain.Premise
	}{
		{"a nonzero residual", domain.Premise{Residual: genQty(t, "0.25"), Class: domain.ClassificationDetermined}},
		{"a residual whose sign is not fixed", domain.Premise{Residual: genQty(t, "0"), Class: domain.ClassificationAmbiguous}},
		{"a deadline-exceeded package", domain.Premise{Residual: genQty(t, "0"), Class: domain.ClassificationDeadlineExceededAmbiguous}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.OpenGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "0.5"), tc.premise)
			if !errors.Is(err, domain.ErrGenerationPremiseNotIntact) {
				t.Fatalf("OpenGeneration = %v, want ErrGenerationPremiseNotIntact", err)
			}
		})
	}

	// The positive control. Without it a constructor that refused everything
	// would score perfectly above.
	if _, err := domain.OpenGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "0.5"), intactPremise(t)); err != nil {
		t.Fatalf("OpenGeneration on an intact premise: %v", err)
	}

	// A target of zero runs the whole ladder and reduces leverage by nothing,
	// while holding §7's one-generation-at-a-time gate closed as it does.
	if _, err := domain.OpenGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "0"), intactPremise(t)); !errors.Is(err, domain.ErrGenerationTargetNotPositive) {
		t.Fatalf("zero target = %v, want ErrGenerationTargetNotPositive", err)
	}
}

// TestTheSpotLegSellsNoMoreThanTheCONFIRMEDPerpReduction is SM §7's
// never-net-short sequencing, and the fixture is built so "confirmed" and
// "targeted" are DIFFERENT numbers.
//
// With the perp leg confirming its full target, "no more than the confirmed
// reduction" and "no more than the target" are the same predicate and the
// distinction §7 draws would be unchecked. Here the target is 0.5 and only 0.3
// confirms, so an implementation capping on the target authorizes 0.5 — a spot
// sale 0.2 larger than the perp buy that preceded it, which is the transient
// going net-SHORT in the rally the generation was opened for.
func TestTheSpotLegSellsNoMoreThanTheConfirmedPerpReduction(t *testing.T) {
	t.Parallel()

	g := openAt(t, "0.5")
	if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := g.RecordPerpFill(genQty(t, "0.3")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := g.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}

	err := g.AuthorizeSpotReduction(genQty(t, "0.4"), intactPremise(t))
	if !errors.Is(err, domain.ErrGenerationSpotExceedsConfirmedPerp) {
		t.Fatalf("authorizing 0.4 against 0.3 confirmed = %v, want ErrGenerationSpotExceedsConfirmedPerp", err)
	}
	// Strictly below the cap is legal, and it must be: §7 says "no MORE spot
	// than", where "no more" and "exactly" are different predicates.
	if err := g.AuthorizeSpotReduction(genQty(t, "0.2"), intactPremise(t)); err != nil {
		t.Fatalf("authorizing 0.2 against 0.3 confirmed: %v", err)
	}
	if got := g.Authorized().Decimal(); got != genQty(t, "0.2").Decimal() {
		t.Fatalf("authorized = %s, want 0.2", g.Authorized())
	}
}

// TestAZeroAuthorizationCompletesWithNothingOnTheWire is §7's honest extreme: a
// perp reduction that confirmed nothing authorizes no spot sale, so there is
// nothing to submit and the generation is done.
//
// It also pins the leg-identity rule in the direction nobody writes: an order id
// supplied here would enter OwnLegs and net an order the generation does not own
// out of its own premise.
func TestAZeroAuthorizationCompletesWithNothingOnTheWire(t *testing.T) {
	t.Parallel()

	g := openAt(t, "0.5")
	if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := g.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := g.AuthorizeSpotReduction(genQty(t, "0"), intactPremise(t)); err != nil {
		t.Fatalf("AuthorizeSpotReduction(0): %v", err)
	}

	if err := g.SubmitSpotReduction(genOrder(t, 4)); !errors.Is(err, domain.ErrGenerationLegNotSubmitted) {
		t.Fatalf("naming a leg against a zero authorization = %v, want ErrGenerationLegNotSubmitted", err)
	}
	if err := g.SubmitSpotReduction(""); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}
	if g.Phase() != domain.GenerationComplete {
		t.Fatalf("phase = %s, want complete", g.Phase())
	}
	if legs := g.OwnLegs(); len(legs) != 1 {
		t.Fatalf("own legs = %v, want only the perp leg", legs)
	}
}

// TestSupersessionQuiescesTheReservedLegFirst is SM §7's obligation on all three
// rows, and the two cases are the rule's own scoping rather than two examples.
//
// At a spot-side park the reserved SECOND leg is out at the venue, so
// supersession is refused until it reaches finality — abandoning it there
// STRANDS the leg, which is the harm the clause exists for. At a perp-side park
// the second leg has not been submitted, so the precondition has nothing to bind
// on and supersession is admitted. Model (a) found that asymmetry with TLC
// rather than by design and recorded it as scoping, not as safety.
func TestSupersessionQuiescesTheReservedLegFirst(t *testing.T) {
	t.Parallel()

	t.Run("refused while the reserved spot leg is live", func(t *testing.T) {
		t.Parallel()

		g := openAt(t, "0.5")
		if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
			t.Fatalf("SubmitPerpReduction: %v", err)
		}
		if err := g.RecordPerpFill(genQty(t, "0.5")); err != nil {
			t.Fatalf("RecordPerpFill: %v", err)
		}
		if err := g.ConfirmPerpReduction(); err != nil {
			t.Fatalf("ConfirmPerpReduction: %v", err)
		}
		if err := g.AuthorizeSpotReduction(genQty(t, "0.5"), intactPremise(t)); err != nil {
			t.Fatalf("AuthorizeSpotReduction: %v", err)
		}
		if err := g.SubmitSpotReduction(genOrder(t, 4)); err != nil {
			t.Fatalf("SubmitSpotReduction: %v", err)
		}

		// All three rows, because the obligation is shared and a guard written
		// on one path is a guard the other two can drift from.
		for _, path := range []domain.SupersessionPath{
			domain.SupersededByEmergencyClose,
			domain.SupersededByFenceReadyLost,
			domain.SupersededByPremiseLost,
		} {
			if err := g.Supersede(path); !errors.Is(err, domain.ErrGenerationLegNotQuiesced) {
				t.Errorf("Supersede(%s) = %v, want ErrGenerationLegNotQuiesced", path, err)
			}
		}
		// The park does not release the obligation — the leg is still out.
		if err := g.ParkSpotLeg(); err != nil {
			t.Fatalf("ParkSpotLeg: %v", err)
		}
		if err := g.Supersede(domain.SupersededByEmergencyClose); !errors.Is(err, domain.ErrGenerationLegNotQuiesced) {
			t.Fatalf("Supersede at a spot park = %v, want ErrGenerationLegNotQuiesced", err)
		}

		// Quiescing it to finality is what unblocks the row — and quiescing
		// drives the generation to Complete, which is why the external rows
		// must still fire there. A guard requiring the generation to be ACTIVE
		// would make the two rules contradict: the only way to satisfy
		// "quiesce first" would be to fall out of supersession's own scope, and
		// the arbitration would never be recorded at all.
		if err := g.ConfirmSpotReduction(genQty(t, "0.5")); err != nil {
			t.Fatalf("ConfirmSpotReduction: %v", err)
		}
		if g.Phase() != domain.GenerationComplete {
			t.Fatalf("phase = %s, want complete", g.Phase())
		}
		if err := g.Supersede(domain.SupersededByEmergencyClose); err != nil {
			t.Fatalf("Supersede after finality: %v", err)
		}
		if g.SupersededBy() != domain.SupersededByEmergencyClose {
			t.Fatalf("superseded by %s, want emergency_close_package", g.SupersededBy())
		}
	})

	// The premise-lost row is the ONE that is additionally scoped to an active
	// generation, and this is the fixture that splits it from the other two: the
	// same state, three paths, and only one refuses. A Supersede written with one
	// guard for all three passes every other case in this file.
	t.Run("the premise-lost row alone is refused once the generation has finished", func(t *testing.T) {
		t.Parallel()

		g := openAt(t, "0.5")
		if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
			t.Fatalf("SubmitPerpReduction: %v", err)
		}
		if err := g.ParkPerpLeg(); err != nil {
			t.Fatalf("ParkPerpLeg: %v", err)
		}
		if err := g.TerminalFlatten(); err != nil {
			t.Fatalf("TerminalFlatten: %v", err)
		}
		if err := g.Supersede(domain.SupersededByPremiseLost); !errors.Is(err, domain.ErrGenerationNotActive) {
			t.Fatalf("premise-lost after TerminalFlatten = %v, want ErrGenerationNotActive", err)
		}
		if err := g.Supersede(domain.SupersededByEmergencyClose); err != nil {
			t.Fatalf("emergency close after TerminalFlatten: %v", err)
		}
	})

	t.Run("admitted at a perp-side park, where no second leg exists", func(t *testing.T) {
		t.Parallel()

		g := openAt(t, "0.5")
		if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
			t.Fatalf("SubmitPerpReduction: %v", err)
		}
		if err := g.ParkPerpLeg(); err != nil {
			t.Fatalf("ParkPerpLeg: %v", err)
		}
		if err := g.Supersede(domain.SupersededByFenceReadyLost); err != nil {
			t.Fatalf("Supersede at a perp park: %v", err)
		}
	})

	t.Run("a generation that has already ended is not re-superseded", func(t *testing.T) {
		t.Parallel()

		g := openAt(t, "0.5")
		if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
			t.Fatalf("SubmitPerpReduction: %v", err)
		}
		if err := g.Supersede(domain.SupersededByPremiseLost); err != nil {
			t.Fatalf("Supersede: %v", err)
		}
		if err := g.Supersede(domain.SupersededByEmergencyClose); !errors.Is(err, domain.ErrGenerationAlreadySuperseded) {
			t.Fatalf("second Supersede = %v, want ErrGenerationAlreadySuperseded", err)
		}
		if g.SupersededBy() != domain.SupersededByPremiseLost {
			t.Fatalf("superseded by %s, want the FIRST path", g.SupersededBy())
		}
	})
}

// TestOutstandingIntentTracksTheCommitmentAndNotTheTarget is FM-5 expressed on
// the aggregate: the ledger entry's amount at each phase.
//
// The target (0.5), the confirmed perp reduction (0.3) and the authorized spot
// sale (0.2) are all DIFFERENT numbers, which is what makes each phase's answer
// attributable. Booking the target at Planned is FM-5 itself — a still-neutral
// package reading as short by 0.5, and §2's determined branch buying back exactly
// the exposure this generation's own perp leg is about to buy.
func TestOutstandingIntentTracksTheCommitmentAndNotTheTarget(t *testing.T) {
	t.Parallel()

	g := openAt(t, "0.5")
	assertIntent := func(phase, want string) {
		t.Helper()
		got, err := g.OutstandingIntent()
		if err != nil {
			t.Fatalf("%s: OutstandingIntent: %v", phase, err)
		}
		if got.Decimal() != genQty(t, want).Decimal() {
			t.Fatalf("%s: outstanding intent = %s, want %s", phase, got, want)
		}
	}

	assertIntent("planned", "0")
	if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	assertIntent("perp in flight, nothing filled", "0")
	if err := g.RecordPerpFill(genQty(t, "0.3")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	assertIntent("perp part-filled", "0.3")
	if err := g.ConfirmPerpReduction(); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	assertIntent("perp final", "0.3")
	if err := g.AuthorizeSpotReduction(genQty(t, "0.2"), intactPremise(t)); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	assertIntent("spot authorized", "0.2")
	if err := g.SubmitSpotReduction(genOrder(t, 4)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}
	assertIntent("spot in flight", "0.2")
	if err := g.ConfirmSpotReduction(genQty(t, "0.2")); err != nil {
		t.Fatalf("ConfirmSpotReduction: %v", err)
	}
	assertIntent("complete", "0")

	// OwnRealized is what the generation contributed to the group's realized
	// delta and it is what the premise nets out: bought 0.3, sold 0.2 back.
	own, err := g.OwnRealized()
	if err != nil {
		t.Fatalf("OwnRealized: %v", err)
	}
	if own.Decimal() != genQty(t, "0.1").Decimal() {
		t.Fatalf("own realized = %s, want 0.1", own)
	}
}

// TestAPerpFillMayNotOvershootTheGenerationsTarget — an excess is a fill
// attributed to the wrong generation or one counted twice, and admitting it
// raises the reserved entry above anything the generation can ever sell back,
// which understates the group's residual for as long as the entry lives.
func TestAPerpFillMayNotOvershootTheGenerationsTarget(t *testing.T) {
	t.Parallel()

	g := openAt(t, "0.5")
	if err := g.SubmitPerpReduction(genOrder(t, 3), intactPremise(t)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := g.RecordPerpFill(genQty(t, "0.4")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	// 0.4 + 0.2 = 0.6 against a target of 0.5. The fixture accumulates rather
	// than overshooting in one step, because a single oversized fill and a
	// running total that crosses the target are different implementations.
	if err := g.RecordPerpFill(genQty(t, "0.2")); !errors.Is(err, domain.ErrGenerationPerpOvershootsTarget) {
		t.Fatalf("RecordPerpFill = %v, want ErrGenerationPerpOvershootsTarget", err)
	}
	if err := g.RecordPerpFill(genQty(t, "0.1")); err != nil {
		t.Fatalf("filling exactly to the target: %v", err)
	}
	if err := g.RecordPerpFill(genQty(t, "0")); !errors.Is(err, domain.ErrGenerationFillNotPositive) {
		t.Fatalf("zero fill = %v, want ErrGenerationFillNotPositive", err)
	}
}

// ---------------------------------------------------------------------------
// §7.5's rehydration: the premise is the ONE guard the restore path drops
// ---------------------------------------------------------------------------

// TestRestoreAndOpenDifferOnTheirPremiseAndOnNothingElse is the pair, and it is
// a pair on purpose.
//
// Asserted one at a time, "Restore admits a dissolved premise" passes against a
// build where NEITHER entry point checks it — which would delete FM-8's entry
// precondition from the system while looking like a recovery feature. The two
// halves in one test are what make the difference the claim.
//
// Every dissolution shape is used, because Premise.Intact is a conjunction
// (`residual == 0 AND class == Determined`) and a table breaking both conjuncts
// at once cannot tell a check on the number from a check on the verdict.
func TestRestoreAndOpenDifferOnTheirPremiseAndOnNothingElse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		premise domain.Premise
	}{
		{"a nonzero residual", domain.Premise{Residual: genQty(t, "0.25"), Class: domain.ClassificationDetermined}},
		{"a residual whose sign is not fixed", domain.Premise{Residual: genQty(t, "0"), Class: domain.ClassificationAmbiguous}},
		{"a deadline-exceeded package", domain.Premise{Residual: genQty(t, "0"), Class: domain.ClassificationDeadlineExceededAmbiguous}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.OpenGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "1"), tc.premise); !errors.Is(err, domain.ErrGenerationPremiseNotIntact) {
				t.Errorf("OpenGeneration under %s = %v, want ErrGenerationPremiseNotIntact", tc.name, err)
			}
			g, err := domain.RestoreGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "1"))
			if err != nil {
				t.Fatalf("RestoreGeneration under %s = %v, want it admitted — refusing leaves the ledger entry out", tc.name, err)
			}
			if g.Phase() != domain.GenerationPlanned {
				t.Errorf("RestoreGeneration().Phase = %s, want %s", g.Phase(), domain.GenerationPlanned)
			}
		})
	}
}

// TestRestoreGenerationStillChecksTheRECORD is the other side: dropping the
// premise must not have dropped the shape checks with it.
//
// Those are claims about the stored record rather than about the world — an id,
// a group, a reservation, a positive target — and a log failing one is a log
// that verifies and does not compose. They are shared with OpenGeneration
// through one constructor precisely so a rehydrated generation cannot be laxer
// about itself than a fresh one.
func TestRestoreGenerationStillChecksTheRecord(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		id          domain.GenerationID
		group       domain.HedgeGroup
		reservation domain.ReservationID
		target      string
		want        error
	}{
		{"no id", "", generationGroup(), "pkg-1", "1", domain.ErrGenerationIDUnset},
		{"no reservation", "gen-1", generationGroup(), "", "1", domain.ErrReservationUnset},
		{"a zero target", "gen-1", generationGroup(), "pkg-1", "0", domain.ErrGenerationTargetNotPositive},
		{"a negative target", "gen-1", generationGroup(), "pkg-1", "-1", domain.ErrGenerationTargetNotPositive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.RestoreGeneration(tc.id, tc.group, tc.reservation, genQty(t, tc.target))
			if !errors.Is(err, tc.want) {
				t.Errorf("RestoreGeneration with %s = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
	// The group, separately: an invalid one has its own family of errors and the
	// table above would have to name one of them.
	if _, err := domain.RestoreGeneration("gen-1", domain.HedgeGroup{}, "pkg-1", genQty(t, "1")); err == nil {
		t.Error("RestoreGeneration with an unset hedge group = nil, want a refusal")
	}
}

// TestTheRestoreStepsKeepTheirOWNChecks is what stops "drops the premise" from
// becoming "drops the guards".
//
// Each restore step is its live twin with the premise removed and nothing else,
// so both keep SM §7's edge set and both keep their payload rule. The spot one
// is the sharper case: CheckSpotSize is §7's never-net-short cap, and at
// recovery the confirmed perp reduction it compares against comes from the venue
// while the size comes from the log — so this refusal is the only thing standing
// between a contradiction between two authorities and an entry claiming a sale
// nothing backs.
func TestTheRestoreStepsKeepTheirOwnChecks(t *testing.T) {
	t.Parallel()

	t.Run("the perp submission refuses an unidentified leg", func(t *testing.T) {
		t.Parallel()
		g := restored(t)
		if err := g.RestorePerpSubmission(""); !errors.Is(err, domain.ErrGenerationLegUnidentified) {
			t.Errorf("RestorePerpSubmission(\"\") = %v, want ErrGenerationLegUnidentified", err)
		}
	})

	t.Run("the perp submission keeps the edge set", func(t *testing.T) {
		t.Parallel()
		g := restored(t)
		if err := g.RestorePerpSubmission("leg-1"); err != nil {
			t.Fatalf("RestorePerpSubmission: %v", err)
		}
		if err := g.RestorePerpSubmission("leg-2"); !errors.Is(err, domain.ErrGenerationNoOp) {
			t.Errorf("a second RestorePerpSubmission = %v, want ErrGenerationNoOp", err)
		}
	})

	t.Run("the authorization keeps the never-net-short cap", func(t *testing.T) {
		t.Parallel()
		g := restored(t)
		if err := g.RestorePerpSubmission("leg-1"); err != nil {
			t.Fatalf("RestorePerpSubmission: %v", err)
		}
		if err := g.RecordPerpFill(genQty(t, "0.3")); err != nil {
			t.Fatalf("RecordPerpFill: %v", err)
		}
		if err := g.ConfirmPerpReduction(); err != nil {
			t.Fatalf("ConfirmPerpReduction: %v", err)
		}
		// 0.5 authorized against 0.3 confirmed: the log and the venue disagree,
		// and admitting it puts an entry into the shared ledger claiming a sale
		// this generation could never back.
		if err := g.RestoreSpotAuthorization(genQty(t, "0.5")); !errors.Is(err, domain.ErrGenerationSpotExceedsConfirmedPerp) {
			t.Errorf("RestoreSpotAuthorization(0.5) over 0.3 confirmed = %v, want ErrGenerationSpotExceedsConfirmedPerp", err)
		}
		// The positive control: 0.25 is under the cap and is admitted, so the
		// refusal above is the CAP biting rather than the method refusing
		// everything.
		if err := g.RestoreSpotAuthorization(genQty(t, "0.25")); err != nil {
			t.Fatalf("RestoreSpotAuthorization(0.25) over 0.3 confirmed = %v, want it admitted", err)
		}
		if got := g.Authorized().Decimal(); got != genQty(t, "0.25").Decimal() {
			t.Errorf("Authorized() = %s, want 0.25", got)
		}
	})

	t.Run("the authorization keeps the edge set", func(t *testing.T) {
		t.Parallel()
		// Straight from `Planned`, which is not an edge: the perp leg was never
		// submitted, so a spot sale would be authorized against a perp reduction
		// that does not exist.
		g := restored(t)
		if err := g.RestoreSpotAuthorization(genQty(t, "0")); !errors.Is(err, domain.ErrGenerationTransitionForbidden) {
			t.Errorf("RestoreSpotAuthorization from Planned = %v, want ErrGenerationTransitionForbidden", err)
		}
	})
}

func restored(t *testing.T) *domain.Generation {
	t.Helper()
	g, err := domain.RestoreGeneration("gen-1", generationGroup(), "pkg-1", genQty(t, "1"))
	if err != nil {
		t.Fatalf("RestoreGeneration: %v", err)
	}
	return g
}
