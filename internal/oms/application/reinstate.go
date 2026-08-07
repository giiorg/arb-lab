package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// GenerationLadder is the seam §7.5's reinstatement reaches SM §7's ladder
// through — the three acts recovery may perform on it, and no fourth.
//
// # It is an interface for the reason ports.JournalReplay is, and the boundary
// # it draws is task #204's whole design problem
//
// The reinstatement has to put a rebuilt generation somewhere and re-enter its
// entry into §6.7's SHARED reserved-reduction ledger. That ledger lives on the
// Classifier, and §75h refuses to hand the replaying side a Classifier: it would
// give recovery "the ability to change the state it is reasoning about while it
// reasons", which is exactly what ports.JournalReplay exists to withhold.
//
// The way through is not a narrower Classifier but a DIFFERENT SUBJECT. What
// recovery needs is not "amend the ledger" — it is "this generation was live;
// make the ladder true again", and the ladder is the one thing that already
// holds both the generation and the ledger and is required to keep them in step.
// So the capability handed over is three whole acts of the ladder, each of which
// is atomic over both objects, rather than one primitive on either. *Deleverer
// implements it; nothing in this file can reach a ledger, a journal or a placer.
//
// Reassess is in the set and it is not decoration. FM-8's obligation is
// STANDING, and the reinstatement is a classification-changing event by
// construction — it moves the subtrahend — so a boot that restored a generation
// and did not reassess it would leave a generation running on a premise that may
// have dissolved during the outage, with nothing obliged to discharge it. That
// is model (a)'s NC23 reached through recovery.
type GenerationLadder interface {
	// Restore reinstates a live generation and re-Reserves its ledger entry.
	Restore(g *domain.Generation) error

	// BarLadder records that a supersession closed this group's ladder.
	BarLadder(group domain.HedgeGroup, path domain.SupersessionPath) error

	// Reassess re-runs FM-8's standing obligation for a group, discharging the
	// premise-lost supersession if the premise has gone.
	Reassess(group domain.HedgeGroup) (Reassessment, error)
}

// Reinstater is §7.5's rebuild at GENERATION scope: the live `PackageDelever`
// generations a previous process left behind, put back into the ladder with
// their entries back in the shared ledger.
//
// # Why this is a phase of its own and not part of the replay
//
// The replay cannot do it, and the reason is a number rather than a layering
// preference. A generation's ledger entry carries its OUTSTANDING NET INTENT,
// which before the authorization is the CONFIRMED PERP REDUCTION — and that
// quantity is in no journal. There is no perp-fill record (Deleverer.RecordPerpFill
// argues why, on a premise #204 had to correct), ports.Journal has no AppendFill
// either, and Replayer says in as many words that every order it rebuilds
// carries a cumulative of zero. The number exists exactly once in this system's
// reach: the VENUE, read by the join and admitted through §5.3's absence
// resolution.
//
// So the ordering is forced, not chosen — replay → join → resolve → REINSTATE —
// and running earlier is not merely premature. A generation whose log holds an
// authorization of s would be rebuilt against a confirmed perp reduction of
// zero, and domain.Generation.CheckSpotSize would refuse the very record the
// journal proves was written. The one phase where the rebuild is possible is the
// one where the venue's answer is already in the projection.
//
// # What it does NOT do
//
// It issues no order and reads no venue. Restoring a generation is bookkeeping —
// an entry in a ledger and an aggregate in a map — and the one act with an
// outward effect is the supersession Reassess may discharge, which writes a
// journal record and releases an entry. That is the same class of write §7.5
// already licenses for the DeactivationPending re-drive and for §5.3's
// resolutions: a reconciliation act that re-drives what already happened.
//
// # Not safe for concurrent use
//
// The Reconciler's argument, unchanged.
type Reinstater struct {
	ladder GenerationLadder
	orders ports.OrderLookup
}

// NewReinstater constructs the use case, refusing a missing collaborator.
//
// The lookup is a READ port (ports.OrderLookup) rather than the write
// repository, and the narrowing is the point: this type rebuilds a generation
// from records whose meaning it is deciding, so a capability to apply a state or
// a fill while it does so is the composition §7.5 spends two ports avoiding.
func NewReinstater(ladder GenerationLadder, orders ports.OrderLookup) (*Reinstater, error) {
	if ladder == nil {
		return nil, ErrGenerationLadderMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	return &Reinstater{ladder: ladder, orders: orders}, nil
}

// GenerationReinstatement is what one generation's rebuild concluded.
type GenerationReinstatement struct {
	// Generation and Group name the subject.
	Generation domain.GenerationID
	Group      domain.HedgeGroup

	// Phase is where the ladder was left. It is the rebuilt aggregate's, not the
	// log's last record, and the two differ in exactly one case worth reporting:
	// a generation the reassessment SUPERSEDED, which is a phase no record in the
	// log carries.
	Phase domain.GenerationPhase

	// Outstanding is what the re-entered ledger entry carries — the quantity
	// §6.7's determined branch will subtract for as long as the generation runs.
	// It is reported because it is the number the whole phase exists to restore,
	// and an operator reading a boot that reinstated a generation has no other
	// way to see it.
	Outstanding fixedpoint.Quantity

	// Reassessment is FM-8's standing obligation, run once the entry was back.
	// Its Superseded field is the case where recovery ENDED a generation it had
	// just reinstated, which is correct and is the thing an operator will ask
	// about.
	Reassessment Reassessment
}

// Reinstatement is what one boot's generation rebuild found.
type Reinstatement struct {
	// Restored is one entry per live generation the log held, in the order
	// Recovery listed them (by id).
	Restored []GenerationReinstatement

	// Barred is the hedge groups whose ladder a supersession closed, applied to
	// the ladder rather than merely reported. It is a copy of what Recovery
	// carried, echoed here so a caller reading this report sees both halves of
	// what the phase did.
	Barred []LadderBar

	// AwaitingQuiesce counts the restored generations whose premise had dissolved
	// while their reserved SECOND leg was still out.
	//
	// It is a COUNT and the generations are in Restored, so nothing is duplicated
	// — what it buys is a single predicate for the gate, which must not have to
	// re-derive the condition by walking the reassessments. See
	// Reconciler.reinstate for why this outcome is boot.OutcomeIncomplete: the
	// supersession is OWED and nothing at boot can discharge it, because all
	// three of §7's rows quiesce that leg to §6.7 finality first.
	AwaitingQuiesce int
}

// Reinstate rebuilds every live generation the replay found and re-enters its
// ledger entry, then applies the ladder bars.
//
// # The bars go on FIRST, and the order is a check rather than a preference
//
// A bar and a live generation on one hedge group are contradictory: Deleverer.Open
// refuses to open on a barred group, so a log holding both is a log that verifies
// and does not compose. Applying the bars first makes Restore the place that says
// so; applying them after would restore the generation, reserve its entry, and
// then bar a group that has a live generation on it — the contradiction accepted
// and recorded, which is the reading that produces a confident wrong answer.
//
// # A failure leaves what it built, on Rebuild's rule
//
// The generations reinstated before the failure keep their entries. Discarding
// them would leave a ledger short by exactly the entries the log proves are
// owed, which is the hole this phase exists to fill — and nothing may trade on a
// boot that reported a failure, because the posture enforces that, not the
// emptiness of a map.
func (r *Reinstater) Reinstate(rec Recovery) (Reinstatement, error) {
	var out Reinstatement
	for _, bar := range rec.BarredLadders {
		if err := r.ladder.BarLadder(bar.Group, bar.Path); err != nil {
			return out, fmt.Errorf("reinstate: bar ladder for %s: %w", bar.Group, err)
		}
		out.Barred = append(out.Barred, bar)
	}
	for _, gen := range rec.LiveGenerations {
		one, err := r.reinstateOne(gen)
		if err != nil {
			return out, err
		}
		out.Restored = append(out.Restored, one)
		if one.Reassessment.AwaitingQuiesce {
			out.AwaitingQuiesce++
		}
	}
	return out, nil
}

// reinstateOne rebuilds one generation, hands it to the ladder and reassesses it.
func (r *Reinstater) reinstateOne(rec GenerationRecovery) (GenerationReinstatement, error) {
	g, err := r.rebuildGeneration(rec)
	if err != nil {
		return GenerationReinstatement{}, err
	}
	if err := r.ladder.Restore(g); err != nil {
		return GenerationReinstatement{}, fmt.Errorf("reinstate generation %q: %w", rec.ID, err)
	}
	outstanding, err := g.OutstandingIntent()
	if err != nil {
		return GenerationReinstatement{}, fmt.Errorf("reinstate generation %q: outstanding intent: %w", rec.ID, err)
	}
	// Read AFTER the restore, so it reports the entry the ledger actually holds
	// rather than what this type computed and hoped was written. They agree by
	// construction — Restore amends from this same method — and the point of
	// reading it here is that a future Restore that did not would be visible.
	assessment, err := r.ladder.Reassess(rec.Group)
	if err != nil {
		return GenerationReinstatement{}, fmt.Errorf("reinstate generation %q: %w", rec.ID, err)
	}
	return GenerationReinstatement{
		Generation:   rec.ID,
		Group:        rec.Group,
		Phase:        g.Phase(),
		Outstanding:  outstanding,
		Reassessment: assessment,
	}, nil
}

// rebuildGeneration drives SM §7's ladder through the recorded steps.
//
// # It replays the steps rather than jumping to the final phase, and that is
// # §7.5's own rule at generation scope
//
// "State is rebuilt by walking the journal in sequence order and applying each
// record through the same write path the live system uses, so the recovered
// projection can only reach states a live process could have produced." The
// generation's version of that write path is domain.Generation's own mutators,
// which check SM §7's edge set on every step — so a log recording an impossible
// LADDER is refused here exactly as a log recording an impossible ORDER history
// is refused by ApplyState. Setting the final phase directly would check one
// reachability question and none of the transitions.
//
// The premise is the one guard removed, and domain.RestoreGeneration argues why:
// it is a fact about the world at the instant of a decision that has already been
// taken and journaled, not a fact about the record. It binds again immediately,
// as FM-8's standing obligation, through the Reassess the caller runs next.
func (r *Reinstater) rebuildGeneration(rec GenerationRecovery) (*domain.Generation, error) {
	g, err := domain.RestoreGeneration(rec.ID, rec.Group, rec.Reservation, rec.Target)
	if err != nil {
		return nil, fmt.Errorf("reinstate generation %q: %w", rec.ID, err)
	}
	for i, step := range rec.Steps {
		if err := r.replayStep(g, rec, step); err != nil {
			return nil, fmt.Errorf("reinstate generation %q step %d (%s): %w", rec.ID, i+1, step, err)
		}
	}
	return g, nil
}

// replayStep applies one recorded phase.
//
// # The confirmed perp reduction enters HERE, with the submission, and there is
// # exactly one instant where it can
//
// domain.Generation.RecordPerpFill requires the phase to be `PerpReduceInFlight`
// — a fill is an event on a live leg — so the venue's cumulative has to be
// applied in the window between the submission record and whatever record
// follows it. That is not an accident of the API: it is the same window the live
// process filled the leg in, and applying the total there is the only reading
// under which the rebuilt aggregate's `perpDone` means what the live one's did.
//
// The quantity is the ORDER's, read from the projection the join has already
// filled in, and it is the reason this whole phase runs after the join. A zero
// cumulative is SKIPPED rather than recorded, because RecordPerpFill refuses a
// non-positive fill — correctly, since a zero fill is not an event — and a leg
// that has executed nothing leaves the outstanding intent at zero, which is what
// `Planned` already set.
//
// The switch names every phase and refuses four. `Planned` is the seed rather
// than a step, and the three ENDED phases cannot appear in a LIVE generation's
// step list at all — the walk that built it puts an ended generation in neither
// list. Both refusals are unreachable from a journal this system writes, and both
// are named rather than defaulted, because a `default:` is where a new phase
// lands silently (§3.6 rule 10, which polices this switch).
func (r *Reinstater) replayStep(g *domain.Generation, rec GenerationRecovery, step domain.GenerationPhase) error {
	switch step {
	case domain.GenerationPerpReduceInFlight:
		if err := g.RestorePerpSubmission(rec.PerpOrder); err != nil {
			return err
		}
		return r.recordPerpExecution(g, rec.PerpOrder)

	case domain.GenerationPerpReduceFinal:
		return g.ConfirmPerpReduction()

	case domain.GenerationHeldPerp:
		return g.ParkPerpLeg()

	case domain.GenerationSpotReduceAuthorized:
		return g.RestoreSpotAuthorization(rec.Authorized)

	case domain.GenerationSpotReduceInFlight:
		return g.SubmitSpotReduction(rec.SpotOrder)

	case domain.GenerationHeldSpot:
		return g.ParkSpotLeg()

	case domain.GenerationPlanned, domain.GenerationComplete,
		domain.GenerationTerminalFlatten, domain.GenerationSuperseded:
		return fmt.Errorf("%s: %w", step, ErrGenerationStepUnreachable)
	}
	return fmt.Errorf("%q: %w", string(step), domain.ErrGenerationPhaseUnhandled)
}

// recordPerpExecution raises the rebuilt generation's confirmed perp reduction
// to whatever the venue says the leg executed.
//
// The order is loaded rather than carried in the Recovery, because the number
// this reads did not exist when the Recovery was produced: the replay leaves
// every cumulative at zero and the join fills it in. Loading here is what makes
// the dependency on the join's having run a fact about the call graph rather
// than a comment.
func (r *Reinstater) recordPerpExecution(g *domain.Generation, order ids.ClientOrderID) error {
	rec, err := r.orders.Load(order)
	if err != nil {
		return fmt.Errorf("perp leg %q: %w", order, err)
	}
	sign, err := rec.Cumulative.Sign()
	if err != nil {
		return fmt.Errorf("perp leg %q executed: %w", order, err)
	}
	if sign == 0 {
		return nil
	}
	return g.RecordPerpFill(rec.Cumulative)
}
