package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Deleverer is SM §7's `PackageDelever` ladder as a use case: the caller the
// generation aggregate cannot be.
//
// # The aggregate decides and cannot act, and this is the actor
//
// domain.Generation is Classifier's shape one machine over — it holds the
// phases, the never-net-short cap and FM-8's standing obligation, and it has no
// journal, no placer and no ledger. Three things §7 requires are therefore not
// expressible there and are the whole content of this type:
//
//  1. FLAG-3's DURABILITY. `SpotReduceAuthorized` is "a durable journal record …
//     so recovery knows the perp-finality precondition was met without
//     re-derivation", and an in-memory phase field is not that.
//  2. The LEDGER staying in step with the ladder. §7's "exactly backed at every
//     instant" invariant is a statement about two objects — the generation's
//     commitment and the shared reserved-reduction entry — and only something
//     holding both can keep it. Generation.OutstandingIntent states what the
//     entry must carry; this is what writes it.
//  3. DISCHARGING the premise-lost supersession. Model (a)'s P5/NC23: "a
//     standing obligation nothing is obliged to discharge is not a standing
//     obligation". It is a LIVENESS requirement, so no invariant finds it, and
//     an aggregate that can only refuse a step cannot satisfy it — refusing is
//     precisely the "merely permitted" shape the control was written against.
//
// # One generation per hedge group, held here, and that IS §7's gate
//
// "Each generation is a bounded attempt made only after the prior generation
// reaches §6.7 finality." That is a rule about the SET of live generations, so
// nothing holding one generation can enforce it. Keying by hedge group rather
// than counting per process is what §7 actually says — the gate is about one
// package's ladder, and two packages delever independently.
//
// The in-process map is not the only guard and is deliberately not trusted as
// one: a generation left in the shared ledger by a previous process would be
// invisible here, and what catches it is Classifier.PremiseFor's own refusal at
// two PackageDelever entries. See Open, where the reserve happens BEFORE the
// premise is asked for precisely so that a stale sibling makes the count two.
//
// # Not safe for concurrent use
//
// Classifier's and Dispatcher's argument, unchanged: §5.1 gives the aggregate an
// OCC version and serialisation is decided one layer out. It is stronger here
// than for either of them, because this type holds a Classifier whose ledger
// every method mutates.
type Deleverer struct {
	journal    ports.GenerationJournal
	classifier *Classifier
	dispatcher *Dispatcher
	orders     ports.OrderWriteRepository

	// filters and clk are the dispatcher's, and together they are this type's
	// half of §2.4/§69 — the admission-time re-verification the Executor does
	// per package and this one does per LEG. See verifyLeg for why the unit
	// differs and why that is not a weakening.
	filters filterPolicy
	clk     clock.Clock

	live map[domain.HedgeGroup]*domain.Generation

	// barred is SM §7's ladder-resumption invariant held as state: a group whose
	// generation ended on a path SupersessionPath.ResumesLadder answers false for
	// may not open another.
	//
	// # It is here rather than on the generation, because the generation is GONE
	//
	// A superseded generation is a value this map's key no longer points at —
	// Open's own first act is to find no live generation and proceed — so a rule
	// about what may happen AFTER it ended cannot be a property of it. Keyed by
	// hedge group for the reason `live` is: §7's gate is about one package's
	// ladder and two packages delever independently.
	//
	// It is MONOTONE and there is no unbar. Nothing here can lift it, and that is
	// the ruling rather than an omission: the two paths that set it are the kill
	// switch and the fence arbitration, in both of which the foundation has taken
	// the package away from the leverage-reduction ladder. ResumesLadder states
	// the cost of being wrong in that direction and accepts it — "a breach needs
	// a fresh escalation to reach it — an alert" — where the cost the other way is
	// this machine re-authorising itself against an arbitration that just fired.
	//
	// Nothing can CLEAR it by accident either, and the reason is arithmetic: once
	// a group is barred no generation opens on it, so no later ending can arrive
	// to overwrite the entry.
	barred map[domain.HedgeGroup]domain.SupersessionPath
}

// NewDeleverer constructs the use case FROM the process's dispatcher, on
// NewExecutor's argument and for its reason.
//
// The dispatcher carries the journal, placer, repository, clock and — the
// load-bearing one — the DispatchAuthority. A seam that could hand this type one
// authority and the executor another would fragment the double-dispatch backstop
// with no test able to see it, and a delever leg is exactly the order that would
// be dispatched twice: it is issued while something has already gone wrong, on a
// path that runs rarely enough that the fragmentation would not surface until it
// mattered.
//
// The Classifier is a separate argument rather than a second thing derived from
// the dispatcher, because it is genuinely a different collaborator: §6.7's gate
// is shared with five other clients (the one-leg-failure handler, ADL, the
// kill-switch, protection-disarm, actuator takeover), and the ledger this type
// amends is the one they all size against. Handing it a private classifier would
// give the generation a private ledger, which is FM-5's failure with the
// bookkeeping done correctly.
//
// The FILTER POLICY is inherited for the authority's reason and not the
// classifier's — it is the same object the Executor enforces, deliberately, so
// that one process cannot hold two ideas of the venue's grid. A parameter here
// would let a root wire this type the empty map, which is a legal construction
// meaning "plans nothing" and which would refuse every delever leg with
// ErrFiltersUnknown at the moment a package is nearing liquidation.
func NewDeleverer(d *Dispatcher, c *Classifier, j ports.GenerationJournal) (*Deleverer, error) {
	if d == nil {
		return nil, ErrDispatcherMissing
	}
	if c == nil {
		return nil, ErrClassifierMissing
	}
	if j == nil {
		return nil, ErrGenerationJournalMissing
	}
	return &Deleverer{
		journal:    j,
		classifier: c,
		dispatcher: d,
		orders:     d.orders,
		filters:    d.filters,
		clk:        d.clk,
		live:       make(map[domain.HedgeGroup]*domain.Generation),
		barred:     make(map[domain.HedgeGroup]domain.SupersessionPath),
	}, nil
}

// Generation reports the group's live generation and whether it has one.
//
// A read, so that a caller driving the ladder can see where it has reached
// without this type having to return the aggregate from every step. The pointer
// is the live one rather than a copy: domain.Generation's mutators are all
// phase-guarded, so a caller cannot drive it anywhere the ladder does not go,
// and a copy would be a second generation for the ledger to disagree with.
func (d *Deleverer) Generation(group domain.HedgeGroup) (*domain.Generation, bool) {
	g, ok := d.live[group]
	return g, ok
}

// Open creates a generation at `Planned` — SM §7's entry from the liq-distance /
// top-up ladder — reserving its ledger entry and journaling it.
//
// # The reserve happens BEFORE the premise is asked for, and the order is the check
//
// SM §7's entry rule is "only after the prior generation reaches §6.7 finality",
// which is a claim about how many PackageDelever entries the shared ledger
// holds. Classifier.PremiseFor refuses at MORE THAN ONE, because the netting
// identifies "this generation's own entry" by its producer. So asking for the
// premise before reserving would see a stale sibling as a count of one — and
// silently net the SIBLING's entry out of this generation's premise, which is
// FM-9 arriving through the decomposition. Reserving first makes the sibling the
// second entry, and the existing refusal is exactly the entry gate.
//
// The consequence is that the ledger is mutated before the generation is known
// to be admissible, so every refusal below this point releases the entry. An
// entry left behind subtracts from every later sizing for the life of the
// process, and it would do so on behalf of a generation that does not exist.
//
// # CHECK, JOURNAL, APPLY
//
// Planner's three-pass rule, narrowed to one subject. The aggregate is
// constructed first — which is where the target, the premise and §7's entry
// precondition are judged — then the opening is made durable, and only then is
// the generation published to this type's map. A journal write for a generation
// that was refused would describe an attempt that never existed, in the store
// §7.5 replays.
func (d *Deleverer) Open(
	id domain.GenerationID,
	group domain.HedgeGroup,
	reservation domain.ReservationID,
	target fixedpoint.Quantity,
) (*domain.Generation, error) {
	// The ladder bar is read BEFORE the live check and before the reserve, and
	// the order is the rule rather than tidiness: a barred package must be
	// refused whatever else is true of it, and every step below this one either
	// mutates the shared ledger or reports about a generation. A barred group
	// with no live generation is the ordinary shape — the arbitration ENDED the
	// last one — so a bar read after the live check would be read only on the
	// path where it can never fire.
	if path, ok := d.barred[group]; ok {
		return nil, fmt.Errorf("delever %s: ladder closed by %s: %w",
			group, path, ErrGenerationLadderBarred)
	}
	if existing, ok := d.live[group]; ok {
		active, err := existing.Phase().IsActive()
		if err != nil {
			return nil, fmt.Errorf("delever %s: %w", group, err)
		}
		if active {
			return nil, fmt.Errorf("delever %s: generation %q is at %s: %w",
				group, existing.ID(), existing.Phase(), ErrGenerationAlreadyLive)
		}
	}

	if _, err := d.classifier.Reserve(group, reservation, domain.ProducerPackageDeleverSpot); err != nil {
		return nil, fmt.Errorf("delever %s: reserve %q: %w", group, reservation, err)
	}

	g, err := d.openUnderReservation(id, group, reservation, target)
	if err != nil {
		// The entry is released rather than left for a later pass to notice,
		// and the release error is JOINED rather than replacing this one: the
		// caller's problem is why the generation was refused, and a ledger this
		// type could not tidy is a second, separate fault that must not hide it.
		if _, _, rerr := d.classifier.Release(group, reservation); rerr != nil {
			return nil, fmt.Errorf("%w (releasing %q also failed: %w)", err, reservation, rerr)
		}
		return nil, err
	}

	d.live[group] = g
	return g, nil
}

// openUnderReservation is Open's body from the reserve onward, so that every
// refusal in it reaches Open's single release.
//
// Split out rather than written inline with a defer, because the release must
// NOT run on success and a defer reading a named error return is the shape where
// a later edit adds a success path that tidies away a live generation's entry.
func (d *Deleverer) openUnderReservation(
	id domain.GenerationID,
	group domain.HedgeGroup,
	reservation domain.ReservationID,
	target fixedpoint.Quantity,
) (*domain.Generation, error) {
	entry, err := d.classifier.PremiseFor(group, nil)
	if err != nil {
		return nil, fmt.Errorf("delever %s: entry premise: %w", group, err)
	}
	g, err := domain.OpenGeneration(id, group, reservation, target, entry)
	if err != nil {
		return nil, fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.journal.AppendGenerationOpened(id, group, reservation, target); err != nil {
		return nil, fmt.Errorf("delever %s: journal generation %q: %w", group, id, err)
	}
	return g, nil
}

// Restore reinstates a generation a previous process left live, re-entering its
// entry in the SHARED reserved-reduction ledger — §7.5's rebuild at generation
// scope, and the only way a generation enters this type without having been
// Opened here.
//
// # The reserve is HERE and not in the caller, and that is the whole reason this
// # method exists rather than an exported `live` map
//
// This type's stated job is that "the LEDGER stays in step with the ladder" —
// §7's "exactly backed at every instant" is a claim about two objects and only
// something holding both can keep it. A reinstatement that installed the
// aggregate and left the caller to reserve would move that invariant into a
// second place, which is precisely the shape FM-6 came out of: a rule whose
// re-run is a discipline somebody keeps rather than a call that exists. So the
// reserve and the amend are one act with the install, and every refusal below
// the reserve releases the entry, on Open's rule and for its reason.
//
// It is also what answers task #204's harder half. The reservation must re-enter
// a ledger that lives on the Classifier, and §7.5's replayer must not hold a
// Classifier — "the ability to change the state it is reasoning about while it
// reasons". The seam that satisfies both is this one: the replayer produces
// VALUES, Reinstater turns them into an aggregate, and the capability to touch
// the ledger stays where it already was.
//
// # The group is TRACKED here if nothing has tracked it, and the asset is the
// # generation's own
//
// Classifier.Track fixes a group's base asset and must happen before any
// reserve. At §7.5 boot nothing has run yet, so the ledger for this group does
// not exist — and the durable evidence of what its base asset is happens to be
// in hand: the generation's target, which is the same quantity
// domain.OpenGeneration seeds every one of its own zeros from. Taking it from
// there is what stops the ledger and the aggregate disagreeing about the asset,
// which is a disagreement that surfaces as a quantity error from inside a fold
// rather than as a refusal naming the group.
//
// An already-tracked group is LEFT ALONE rather than re-tracked, because Track
// installs a fresh ledger: re-tracking a group that holds entries would discard
// them, which is the FM-5 shape with the bookkeeping done in the other
// direction.
//
// # Three refusals, and each is a different claim about the log
//
//   - an ENDED generation. This map holds what the ladder may still act on, and
//     an ended generation has no entry to re-Reserve — retire already released
//     it — so reinstating one would put a second, backing-less entry into the
//     shared ledger for as long as the process lives.
//   - a group this call has already restored. At boot the map starts empty, so a
//     collision is not the ordinary "the last generation finished" case Open
//     tolerates; it is one log claiming two live generations on one package,
//     which SM §7's gate forbids and PremiseFor's netting cannot survive.
//   - a BARRED group. A live generation on a package whose ladder an arbitration
//     closed is a history this build cannot produce — Open refuses it — so a log
//     holding both is a log that verifies and does not compose. Unreachable
//     today and standing the day it is not.
func (d *Deleverer) Restore(g *domain.Generation) error {
	if g == nil {
		return ErrGenerationMissing
	}
	group := g.Group()
	active, err := g.Phase().IsActive()
	if err != nil {
		return fmt.Errorf("delever %s: restore %q: %w", group, g.ID(), err)
	}
	if !active {
		return fmt.Errorf("delever %s: restore %q at %s: %w",
			group, g.ID(), g.Phase(), domain.ErrGenerationNotActive)
	}
	if existing, ok := d.live[group]; ok {
		return fmt.Errorf("delever %s: restore %q, generation %q already restored: %w",
			group, g.ID(), existing.ID(), ErrGenerationAlreadyLive)
	}
	if path, ok := d.barred[group]; ok {
		return fmt.Errorf("delever %s: restore %q, ladder closed by %s: %w",
			group, g.ID(), path, ErrGenerationLadderBarred)
	}

	if !d.classifier.Tracked(group) {
		if err := d.classifier.Track(group, g.Target().Asset()); err != nil {
			return fmt.Errorf("delever %s: restore %q: %w", group, g.ID(), err)
		}
	}
	if _, err := d.classifier.Reserve(group, g.Reservation(), domain.ProducerPackageDeleverSpot); err != nil {
		return fmt.Errorf("delever %s: restore %q: reserve %q: %w", group, g.ID(), g.Reservation(), err)
	}
	if err := d.syncLedger(g); err != nil {
		// Open's join, for Open's reason: the caller's problem is why the
		// reinstatement failed, and a ledger this type could not tidy is a
		// second fault that must not hide it.
		if _, _, rerr := d.classifier.Release(group, g.Reservation()); rerr != nil {
			return fmt.Errorf("%w (releasing %q also failed: %w)", err, g.Reservation(), rerr)
		}
		return err
	}
	d.live[group] = g
	return nil
}

// BarLadder records that a generation on this group ended on a path core §7.1's
// ladder does not resume from — SM §7's ladder-resumption invariant reinstated
// at §7.5 boot.
//
// It takes the PATH rather than a bool for the reason domain.SupersessionPath
// exists: whether the ladder resumes is a property of the path, so a caller
// passing a resuming one is asking for something this rule does not do, and the
// refusal below says which path it refused. A caller that had already reduced
// the path to "does not resume" would have made that mistake unexpressible only
// by making it unreportable.
//
// It is idempotent for a repeated (group, path) and refuses a CONTRADICTING
// second path, on domain.Generation.Supersede's rule: the first arbitration is
// what an operator is shown, and silently overwriting it would make the alert
// name whichever row was replayed last.
func (d *Deleverer) BarLadder(group domain.HedgeGroup, path domain.SupersessionPath) error {
	if err := group.Validate(); err != nil {
		return fmt.Errorf("delever: %w", err)
	}
	resumes, err := path.ResumesLadder()
	if err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	if resumes {
		return fmt.Errorf("delever %s: %s: %w", group, path, ErrGenerationLadderResumes)
	}
	if existing, ok := d.barred[group]; ok && existing != path {
		return fmt.Errorf("delever %s: ladder already closed by %s, not %s: %w",
			group, existing, path, ErrGenerationLadderBarred)
	}
	d.barred[group] = path
	return nil
}

// SubmitPerpReduction dispatches the buy-to-reduce leg — SM §7's "reduces the
// perp short FIRST, which itself relieves the liq risk".
//
// ref names an order the caller has already planned, on ports.Journal's own
// write path: §6 step 5 makes Planner "the only way an order enters the system",
// so a leg minted here would be an order with no journaled intent. What this
// method does instead is VERIFY the ref against what the phase requires, which
// is the stronger reading — the checks below are made against the durable
// record rather than against a caller's assertion about it.
//
// # This step commits new exposure, so a dissolved premise DISCHARGES rather
// # than refusing
//
// See reassess. Returning ErrGenerationPremiseDissolved here and leaving the
// generation live is precisely model (a)'s NC23 — the obligation made possible
// on the next event and guaranteed by nothing.
func (d *Deleverer) SubmitPerpReduction(group domain.HedgeGroup, ref DispatchRef) (Outcome, error) {
	g, err := d.liveGeneration(group)
	if err != nil {
		return Outcome{}, err
	}
	// The premise is read BEFORE the leg is verified, and the order is the
	// discharge obligation rather than a style choice: every ATTEMPT to take the
	// generation further must end in the supersession being performed, so a
	// malformed ref must not be able to consume the attempt. Verifying first
	// would leave a caller that keeps presenting a stale version refusing
	// forever over a premise nothing ever acts on — model (a)'s NC23 reached
	// through the argument list instead of through the guard.
	premise, err := d.premiseOrDischarge(g, domain.GenerationPerpReduceInFlight)
	if err != nil {
		return Outcome{}, err
	}
	rec, err := d.verifyLeg(g, ref, legPerp, g.Target())
	if err != nil {
		return Outcome{}, err
	}
	if err := g.CheckAdvance(domain.GenerationPerpReduceInFlight, premise); err != nil {
		return Outcome{}, fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.journal.AppendGenerationPerpSubmitted(g.ID(), rec.Order.ClientOrderID); err != nil {
		return Outcome{}, fmt.Errorf("delever %s: journal perp leg %q: %w", group, ref.ID, err)
	}
	if err := g.SubmitPerpReduction(rec.Order.ClientOrderID, premise); err != nil {
		return Outcome{}, fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.syncLedger(g); err != nil {
		return Outcome{}, err
	}

	// §7.6's Safety class, not Opening: `PackageDelever` is core §5.3's
	// foundation-originated command and is "exempt from stop-opening", which is
	// the same fact stated from the other side. A leg defaulted to Opening
	// would be shed under the ~70% back-off ceiling during exactly the incident
	// the reserve band exists for.
	return d.dispatcher.Dispatch(ref.ID, ref.Version, ports.UrgencySafety, nil)
}

// RecordPerpFill raises the confirmed perp reduction and amends the ledger entry
// to match — §7's "amended as the perp reduction confirms".
//
// # It is the ONE ladder step with no generation record, and the reason is NOT
// # the one this comment used to give
//
// Every other step here writes to the generation journal because the fact it
// records exists nowhere else. This one does not, because the quantity IS the
// perp order's own cumulative execution — and the original justification went on
// to say that quantity was "durable at order scope" and rebuilt by "§7.5's
// replay". Both halves are false, and #204 found it by needing the number:
// ports.Journal has no AppendFill, so no fill is journaled at order scope
// either, and Replayer says in as many words that every record it rebuilds
// carries a ZERO cumulative. The conclusion survives on a different premise —
// §7.5's join rebuilds executed quantity from VENUE TRUTH, so the number is
// recoverable without a second record of it — and the premise it survives on is
// what fixes WHERE recovery can rebuild a generation: not at the replay, which
// has no fills, but after the join and after §5.3's absence resolution. See
// Reinstater.
//
// A second record would still be two stores holding one number with no rule for
// a replay that finds them disagreeing — and the disagreement would be about the
// exact quantity §7's spot cap is sized from.
//
// The amend is not optional and is why this method exists at all: §6.7 requires
// re-classification on every change to outstanding_reserved_reductions, and SM
// §7's own invariant list names the perp leg's confirmations as one of them.
// Recording the fill without amending is FM-5's shape with the sign the other
// way — the group's realized delta rises while the entry that is supposed to
// offset it does not, so a still-neutral package reads as net LONG by the fill
// and §2's determined branch sells the spot this generation is about to sell.
func (d *Deleverer) RecordPerpFill(group domain.HedgeGroup, executed fixedpoint.Quantity) error {
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	if err := g.RecordPerpFill(executed); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	return d.syncLedger(g)
}

// ConfirmPerpReduction records the perp reduction reaching §6.7 finality —
// "confirms that reduction to finality before any spot sell" — from the
// in-flight leg or from its park.
//
// Unguarded by the premise, on GenerationPhase.CommitsNewExposure's rule: a
// finality event commits nothing new, and refusing it would strand rather than
// prevent.
func (d *Deleverer) ConfirmPerpReduction(group domain.HedgeGroup) error {
	return d.plainStep(group, domain.GenerationPerpReduceFinal,
		func(g *domain.Generation) error { return g.ConfirmPerpReduction() })
}

// AuthorizeSpotReduction fixes the spot leg's size and makes it durable —
// FLAG-3's record, and the crash boundary this whole port exists for.
//
// # The size is checked, then journaled, then applied, and the order matters
//
// The journal is append-only, so an authorization written for a size the
// aggregate then refuses is a durable record of a decision that never took
// effect — and it is the record recovery reads to conclude that a spot sale was
// authorized. domain.Generation.CheckSpotSize exists to make the check
// available ahead of the write; see its doc for why the rule stays in one place
// rather than being restated here.
//
// # This step commits new exposure
//
// So a dissolved premise discharges the supersession rather than refusing. It
// is one of exactly two steps in the ladder that do — see
// GenerationPhase.CommitsNewExposure, where every member of the true-set and
// the false-set was settled by a model run.
func (d *Deleverer) AuthorizeSpotReduction(group domain.HedgeGroup, size fixedpoint.Quantity) error {
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	premise, err := d.premiseOrDischarge(g, domain.GenerationSpotReduceAuthorized)
	if err != nil {
		return err
	}
	if err := g.CheckAdvance(domain.GenerationSpotReduceAuthorized, premise); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	if err := g.CheckSpotSize(size); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.journal.AppendGenerationAuthorized(g.ID(), size); err != nil {
		return fmt.Errorf("delever %s: journal authorization for %q: %w", group, g.ID(), err)
	}
	if err := g.AuthorizeSpotReduction(size, premise); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	return d.syncLedger(g)
}

// SubmitSpotReduction dispatches the reserved spot reduction, or completes the
// generation when the authorized size was zero and there is nothing to send.
//
// # It is deliberately NOT premise-guarded, and the exclusion was found by a run
//
// The amount was fixed at authorization and the entry is already in the shared
// ledger, so refusing to send does not prevent a commitment — it STRANDS one,
// which is the harm §7's quiesce rule exists to forbid. Guarded, TLC returned a
// 16-state behaviour ending in stuttering: a generation authorizes zero spot,
// §2's determined branch correctly compensates the residual, and the package
// ends physically flat and Determined while the generation sits at
// `SpotReduceAuthorized` unable to finish.
//
// The zero-authorization branch takes NO ref, and that is the aggregate's rule
// surfacing: an id supplied for a leg that was never sent would enter OwnLegs
// and net an order this generation does not own out of its own premise.
func (d *Deleverer) SubmitSpotReduction(group domain.HedgeGroup, ref DispatchRef) (Outcome, error) {
	g, err := d.liveGeneration(group)
	if err != nil {
		return Outcome{}, err
	}
	sign, err := g.Authorized().Sign()
	if err != nil {
		return Outcome{}, fmt.Errorf("delever %s: authorized: %w", group, err)
	}
	if sign == 0 {
		if ref.ID != "" {
			return Outcome{}, fmt.Errorf("delever %s: generation %q names order %q against a zero authorization: %w",
				group, g.ID(), ref.ID, domain.ErrGenerationLegNotSubmitted)
		}
		return Outcome{}, d.finish(g, domain.GenerationComplete,
			func(g *domain.Generation) error { return g.SubmitSpotReduction("") })
	}

	rec, err := d.verifyLeg(g, ref, legSpot, g.Authorized())
	if err != nil {
		return Outcome{}, err
	}
	if err := d.journal.AppendGenerationSpotSubmitted(g.ID(), rec.Order.ClientOrderID); err != nil {
		return Outcome{}, fmt.Errorf("delever %s: journal spot leg %q: %w", group, ref.ID, err)
	}
	if err := g.SubmitSpotReduction(rec.Order.ClientOrderID); err != nil {
		return Outcome{}, fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.syncLedger(g); err != nil {
		return Outcome{}, err
	}
	return d.dispatcher.Dispatch(ref.ID, ref.Version, ports.UrgencySafety, nil)
}

// ConfirmSpotReduction records what the spot leg executed and completes the
// generation.
func (d *Deleverer) ConfirmSpotReduction(group domain.HedgeGroup, executed fixedpoint.Quantity) error {
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	return d.finish(g, domain.GenerationComplete,
		func(g *domain.Generation) error { return g.ConfirmSpotReduction(executed) })
}

// ParkPerpLeg and ParkSpotLeg record SM §7's `Held/Alert` for the leg that
// parked — "holds and alerts, never blind-retries or blind-flattens".
//
// Two methods rather than one taking a leg, because they are two phases here for
// a stated reason (GenerationPhase's doc): the resume edge and the
// quiesce obligation differ by which leg parked, and a single method with a
// discriminator would be the collapsed spelling of exactly that distinction.
//
// Neither is premise-guarded. A park is what the quiesce window is MADE of, and
// refusing one would not prevent a commitment.
func (d *Deleverer) ParkPerpLeg(group domain.HedgeGroup) error {
	return d.plainStep(group, domain.GenerationHeldPerp,
		func(g *domain.Generation) error { return g.ParkPerpLeg() })
}

func (d *Deleverer) ParkSpotLeg(group domain.HedgeGroup) error {
	return d.plainStep(group, domain.GenerationHeldSpot,
		func(g *domain.Generation) error { return g.ParkSpotLeg() })
}

// TerminalFlatten ends a parked generation on FLAG-4's terminal branch — "if it
// cannot complete, its terminal full-flatten routes through the same §6.7
// finality contract, not a blind market dump".
//
// Issuing the flatten itself is the caller's, through the ordinary write path,
// for the reason every leg here is: §6 step 5 owns order creation. What this
// records is that the generation has ended that way, which retires its ledger
// entry — and the retirement is the part nothing else would do.
//
// THE SPEC RETIRED THIS STATE AND THIS CODE HAS NOT CAUGHT UP YET — SM §7's
// states table, v0.9.67, task #224, record §87 §3 R6. The sentence above is
// exactly why: this method records an ending and issues nothing, so a machine
// entering it has asserted a flatten the venue has not performed, and the state
// was never reachable from `Complete`, which is where the redesigned ladder
// takes its terminal decision. The terminal rung is core §5.3's
// `EmergencyClosePackage`, owned by §7.1's remedy episode and recorded only on
// perp-flat evidence — and its both-leg close does not exist yet (record §87
// §5b). Slice B4 (task #222) removes or redefines this along with the incoming
// edges.
//
// Deliberately carries NO slice-close deviation marker, and the reason is that
// the marker points the other way: it names code that departed from the spec and
// owes a spec edit before the slice closes. Here the spec moved first, by
// decision, and the code owes the change — recorded in §87 rather than
// discovered later, which is the property the marker exists to buy. Spelling the
// marker here would also have minted one: the gate is a grep over this tree, so
// a comment ABOUT the marker is indistinguishable from a marker.
func (d *Deleverer) TerminalFlatten(group domain.HedgeGroup) error {
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	return d.finish(g, domain.GenerationTerminalFlatten,
		func(g *domain.Generation) error { return g.TerminalFlatten() })
}

// Supersede ends the generation on one of SM §7's two EXTERNAL rows —
// `EmergencyClosePackage` (core §5.3) and the `FenceReadyLost` arbitration (core
// §7.4).
//
// The premise-lost row is deliberately not reachable here. It is this machine's
// OWN standing obligation rather than something an outside caller decides, and
// it is discharged by Reassess, which computes the quantity that decides it —
// so admitting it as an argument would let a caller assert a premise loss the
// ledger does not show, which is a generation cancelled on nobody's evidence.
//
// A live reserved leg is refused rather than superseded, and the refusal IS the
// quiesce-first rule: all three rows "cancel/quiesce the generation's reserved
// second leg to §6.7 finality FIRST". The caller's answer to that refusal is to
// drive the leg to finality — which for a marketable IOC is the venue's own
// doing plus §6.7's contract, never a cancel this system issues — and supersede
// then. The row still fires afterwards, against a generation that has reached
// `Complete`, because the two external rows are not guarded on the generation
// still being active; see domain.Generation.Supersede for why requiring that
// would make the two rules contradict each other.
func (d *Deleverer) Supersede(group domain.HedgeGroup, path domain.SupersessionPath) error {
	if path == domain.SupersededByPremiseLost {
		return fmt.Errorf("delever %s: %w", group, ErrPremiseLossNotExternal)
	}
	if err := path.Validate(); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	return d.supersede(g, path)
}

// Reassessment is what one standing-obligation check concluded.
type Reassessment struct {
	// Group and Generation name the subject, so a value handed to an alerting
	// path carries what it is about.
	Group      domain.HedgeGroup
	Generation domain.GenerationID

	// Premise is the quantity the conclusion was drawn from — FM-9's netted
	// residual and verdict. It is carried whatever the conclusion, because
	// "intact" and "dissolved" are answers to a question about a NUMBER, and an
	// operator reading an alert that a generation was cancelled needs the number.
	Premise domain.Premise

	// Intact is whether the premise still holds. When it is true every other
	// field below is false and nothing was done.
	Intact bool

	// Superseded reports that the premise-lost row FIRED and the generation has
	// ended. It is the discharge model (a)'s NC23 requires.
	Superseded bool

	// ResumesLadder is domain.SupersessionPath.ResumesLadder for the row that
	// fired — true for premise-lost, so core §7.1's ladder may open a later
	// generation once the package is neutral again.
	ResumesLadder bool

	// AwaitingQuiesce reports the premise dissolved while the reserved SECOND
	// leg is still out at the venue. The supersession is OWED and not yet
	// dischargeable, because all three of §7's rows quiesce that leg to §6.7
	// finality FIRST.
	//
	// This is the one state in which the obligation is outstanding, and it is
	// bounded rather than open: the leg is a marketable IOC, so the venue itself
	// resolves it, and the two phases this can be reported from
	// (`SpotReduceInFlight`, `HeldSpot`) both have `Complete` and — from the park
	// — `TerminalFlatten` among their successors. Either ENDS the generation,
	// which discharges the obligation by making its subject finished; there is no
	// path on which a dissolved premise leaves a generation running.
	AwaitingQuiesce bool
}

// Reassess is FM-8's standing obligation, discharged.
//
// # It is the answer to model (a)'s P5/NC23, and refusing is not that answer
//
// SM §7: "the obligation must be DISCHARGED, not merely permitted — an
// implementation that makes the supersession possible on the next event and
// never guarantees it has a real stuck state". The model's counterexample is a
// generation halted at `PerpReduceFinal` with a genuinely dissolved premise, the
// dispatch budget spent so §2 cannot compensate either, and the one action that
// would end it available and never taken. It is a LIVENESS requirement, so no
// invariant finds it.
//
// So this method DOES the supersession rather than reporting that one is
// available, and every step of the ladder that commits new exposure routes
// through it rather than returning ErrGenerationPremiseDissolved. A caller
// cannot decline to discharge, because there is no call through which the
// obligation is merely reported.
//
// # It must be called on every classification-changing event, and that is §6.7's
// # own rule rather than a new one
//
// The premise is a function of the same three inputs §6.7's verdict is, so the
// events that re-run the classification are exactly the events that can dissolve
// a premise. The ladder's own steps call this themselves; what a caller owes is
// the events that are not ladder steps — another producer's entry released,
// somebody else's fill, an interval collapsing.
//
// # A generation that has ENDED is not reassessed, and the asymmetry is stated
//
// A generation at `Complete`, `TerminalFlatten` or `Superseded` has nothing left
// to take further, so the premise-lost row is refused against it by the
// aggregate. This reports Intact for it rather than treating an ended generation
// as a fault, because "the premise dissolved after the generation finished" is
// an ordinary thing for a package to do — it is what §2's determined branch then
// compensates.
func (d *Deleverer) Reassess(group domain.HedgeGroup) (Reassessment, error) {
	g, err := d.liveGeneration(group)
	if err != nil {
		return Reassessment{}, err
	}
	return d.reassess(g)
}

func (d *Deleverer) reassess(g *domain.Generation) (Reassessment, error) {
	premise, err := d.classifier.PremiseFor(g.Group(), g.OwnLegs())
	if err != nil {
		return Reassessment{}, fmt.Errorf("delever %s: premise: %w", g.Group(), err)
	}
	r := Reassessment{Group: g.Group(), Generation: g.ID(), Premise: premise}

	intact, err := premise.Intact()
	if err != nil {
		return Reassessment{}, fmt.Errorf("delever %s: premise: %w", g.Group(), err)
	}
	active, err := g.Phase().IsActive()
	if err != nil {
		return Reassessment{}, fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	if intact || !active {
		r.Intact = true
		return r, nil
	}

	live, err := g.Phase().ReservedLegLive()
	if err != nil {
		return Reassessment{}, fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	if live {
		r.AwaitingQuiesce = true
		return r, nil
	}

	if err := d.supersede(g, domain.SupersededByPremiseLost); err != nil {
		return Reassessment{}, err
	}
	resumes, err := domain.SupersededByPremiseLost.ResumesLadder()
	if err != nil {
		return Reassessment{}, fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	r.Superseded = true
	r.ResumesLadder = resumes
	return r, nil
}

// premiseOrDischarge is what the two exposure-committing steps call instead of
// reading the premise directly.
//
// On an intact premise it returns it. On a dissolved one it DISCHARGES the
// supersession and returns an error describing what it did — so the step's
// caller learns both that its step did not happen and that the generation has
// ended, which are different facts and would be one if this merely refused.
//
// `to` is carried only for the message: which step was refused is the operator's
// first question, and by the time this returns the generation's phase no longer
// says.
//
// # What the three branches below do NOT do, established by a surviving mutant
//
// They do not enforce anything. Collapsing them to "always return the premise"
// SURVIVED the campaign, because domain.Generation.CheckAdvance re-reads
// Premise.Intact at every exposure-committing step and refuses a dissolved one
// with the same sentinel. What the branches buy is the MESSAGE — whether the
// generation was ended or is waiting on its reserved leg, which the phase no
// longer distinguishes and the caller cannot otherwise learn.
//
// The discharge is not in the branches either. It is in the reassess call above
// them, which is why the mutation that actually negates this method's claim is
// removing the supersession itself rather than rearranging what is reported.
// Worth stating because the reverse reading is the natural one: this looks like
// the guard, and the guard is one layer down.
func (d *Deleverer) premiseOrDischarge(g *domain.Generation, to domain.GenerationPhase) (domain.Premise, error) {
	r, err := d.reassess(g)
	if err != nil {
		return domain.Premise{}, err
	}
	if r.Intact {
		return r.Premise, nil
	}
	if r.AwaitingQuiesce {
		return domain.Premise{}, fmt.Errorf(
			"delever %s: generation %q to %s: premise dissolved (residual %s, %s), supersession awaits the reserved leg: %w",
			g.Group(), g.ID(), to, r.Premise.Residual, r.Premise.Class, domain.ErrGenerationPremiseDissolved)
	}
	return domain.Premise{}, fmt.Errorf(
		"delever %s: generation %q to %s: premise dissolved (residual %s, %s), generation superseded: %w",
		g.Group(), g.ID(), to, r.Premise.Residual, r.Premise.Class, domain.ErrGenerationPremiseDissolved)
}

// supersede journals the row and applies it, then retires the ledger entry — but
// only if this supersession is what ENDED the generation.
//
// # The two external rows fire against generations that have already ended, and
// # those have no entry left
//
// domain.Generation.Supersede admits `EmergencyClosePackage` and `FenceReadyLost`
// against a COMPLETE generation, and the reason is the quiesce rule: the caller's
// answer to a live reserved leg is to drive it to §6.7 finality, and doing that
// takes the generation to `Complete`. So the ordinary path for an external
// arbitration arriving mid-generation is quiesce-then-supersede, and by the time
// the row fires the entry was retired by the completion.
//
// Retiring unconditionally would then release an entry the ledger does not hold,
// and the refusal would surface as a supersession that failed AFTER the row was
// durably journaled — an arbitration recorded as having ended a generation the
// process still believes is live. Reading IsActive before the step is what tells
// the two cases apart, and it is read rather than inferred from the path,
// because either external row can arrive at either kind of generation.
func (d *Deleverer) supersede(g *domain.Generation, path domain.SupersessionPath) error {
	if err := g.CheckSupersede(path); err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	active, err := g.Phase().IsActive()
	if err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	if err := d.journal.AppendGenerationSuperseded(g.ID(), path); err != nil {
		return fmt.Errorf("delever %s: journal supersession of %q: %w", g.Group(), g.ID(), err)
	}
	if err := g.Supersede(path); err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	}

	// The ladder bar is applied on the LIVE path as well as at boot, and that is
	// the point of putting it here rather than in the reinstatement: SM §7's
	// resumption invariant is one rule, and a version of it that only held over a
	// restart would be a system that refuses what it has forgotten and permits
	// what it remembers. BarLadder refuses a resuming path, so the predicate is
	// asked once, in the place that owns it.
	//
	// It runs BEFORE the retire and after the aggregate has taken the
	// supersession, because the two ends fail differently: a bar not applied
	// leaves the ladder open against an arbitration, and a retire not performed
	// leaves an entry subtracting for the life of the process. Ordering the
	// weaker-failing one second is the same argument Open's release makes.
	if resumes, err := path.ResumesLadder(); err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	} else if !resumes {
		if err := d.BarLadder(g.Group(), path); err != nil {
			return fmt.Errorf("delever %s: %w", g.Group(), err)
		}
	}

	if !active {
		return nil
	}
	return d.retire(g)
}

// plainStep is a phase change that carries no payload and commits no new
// exposure: journal it, apply it, then put the ledger where the new phase says.
func (d *Deleverer) plainStep(group domain.HedgeGroup, to domain.GenerationPhase,
	apply func(*domain.Generation) error) error {
	g, err := d.liveGeneration(group)
	if err != nil {
		return err
	}
	if err := g.CheckAdvance(to, domain.Premise{}); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	if err := d.journal.AppendGenerationPhase(g.ID(), to); err != nil {
		return fmt.Errorf("delever %s: journal phase %s for %q: %w", group, to, g.ID(), err)
	}
	if err := apply(g); err != nil {
		return fmt.Errorf("delever %s: %w", group, err)
	}
	return d.syncLedger(g)
}

// finish is plainStep for the two phases that END a generation without a
// supersession: `Complete` and `TerminalFlatten`. It retires the entry rather
// than amending it — see retire, where the difference is the whole point.
func (d *Deleverer) finish(g *domain.Generation, to domain.GenerationPhase,
	apply func(*domain.Generation) error) error {
	if err := g.CheckAdvance(to, domain.Premise{}); err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	if err := d.journal.AppendGenerationPhase(g.ID(), to); err != nil {
		return fmt.Errorf("delever %s: journal phase %s for %q: %w", g.Group(), to, g.ID(), err)
	}
	if err := apply(g); err != nil {
		return fmt.Errorf("delever %s: %w", g.Group(), err)
	}
	return d.retire(g)
}

// syncLedger writes the generation's OutstandingIntent into its shared ledger
// entry — SM §7's "exactly backed at every instant", performed rather than
// asserted.
//
// The amount is never computed here. domain.Generation.OutstandingIntent states
// what the entry must carry at each phase, and this calls it, so the ladder and
// the ledger agree BY CONSTRUCTION. A second derivation in this package is the
// FM-5 shape exactly: §2's sizing subtracts the ledger unconditionally, so an
// entry that does not match the generation's actual commitment is
// indistinguishable from a real position, and every check that would object is
// computed from the same subtraction.
func (d *Deleverer) syncLedger(g *domain.Generation) error {
	outstanding, err := g.OutstandingIntent()
	if err != nil {
		return fmt.Errorf("delever %s: outstanding intent: %w", g.Group(), err)
	}
	if _, err := d.classifier.Amend(g.Group(), g.Reservation(), outstanding); err != nil {
		return fmt.Errorf("delever %s: amend %q: %w", g.Group(), g.Reservation(), err)
	}
	return nil
}

// retire RELEASES the generation's ledger entry, and the choice of release over
// an amend to zero is the ending's whole arithmetic.
//
// Both leave the entry subtracting nothing, and they are not the same act. What
// the entry was carrying at the instant a generation ends is real exposure the
// generation put on the package and did not take back — a superseded generation
// that bought one perp and sold no spot leaves the group genuinely net LONG by
// one — and releasing says so: the residual moves by that amount, §2 sees a
// naked residual, and its determined branch compensates it. Amending to zero
// first would report the same final ledger while asserting that the exposure had
// gone somewhere, and the group would read flat while holding a leg.
//
// Classifier.Release returns the released amount because it is "the caller's
// obligation, not a convenience". Here the obligation is discharged by the
// release itself: the quantity re-enters §2's arithmetic as the generation's own
// orders, which are still in the projection and now no longer netted out of
// anything.
func (d *Deleverer) retire(g *domain.Generation) error {
	if _, _, err := d.classifier.Release(g.Group(), g.Reservation()); err != nil {
		return fmt.Errorf("delever %s: release %q: %w", g.Group(), g.Reservation(), err)
	}
	return nil
}

func (d *Deleverer) liveGeneration(group domain.HedgeGroup) (*domain.Generation, error) {
	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("delever: %w", err)
	}
	g, ok := d.live[group]
	if !ok {
		return nil, fmt.Errorf("delever %s: %w", group, ErrNoLiveGeneration)
	}
	return g, nil
}

// legRole names which of the generation's two legs a ref is being checked as.
type legRole string

const (
	legPerp legRole = "perp"
	legSpot legRole = "spot"
)

// verifyLeg establishes that a planned order really is the leg the ladder is
// about to submit, against the DURABLE record rather than the caller's word.
//
// # Five checks, and each one has a distinct harm
//
//   - the version, which is Dispatch's own (id, version) contract read early, so
//     a stale ref is refused before anything is journaled rather than after;
//   - the hedge group, because an order outside it is not in the fold the premise
//     is computed over — so it would never be netted out, and the generation
//     would judge itself against a quantity containing its own leg;
//   - the PRODUCT, which is §7's perp-short-FIRST sequencing made checkable. A
//     spot order submitted as the perp leg sells spot before any perp reduction
//     confirmed, which is the transient going net-SHORT in the rally the
//     generation was opened for — the one direction the whole ordering exists to
//     avoid, and nothing downstream would object because the sizes agree;
//   - the QUANTITY, exactly rather than as a bound. The generation carries
//     "explicit per-leg target positions" and the ledger entry is amended from
//     them, so an order smaller than what the entry claims overstates the
//     reserved reduction — the residual then reads MORE short than the package
//     is, and §2's determined branch BUYS, adding exposure while deleveraging.
//   - §2.4's GRID and §69's row DATE, which is task #184 and is the check the
//     Executor's preflight does for the package path. See below.
//
// The side is not checked, and its absence is deliberate rather than an
// omission: the perp leg buys and the spot leg sells only for a cash-and-carry
// held long-spot/short-perp, which is milestone 1's shape and not a property §7
// states. What §7 does state is the product ordering and the size cap, and those
// are what is here.
//
// # The admission unit is ONE LEG, and it is the ladder's own shape rather than
// # a weakening of §62d
//
// The Executor stamps a single instant for a whole package because its legs go
// out in one fan-out and refusing leg 2 after leg 1 was sent is the
// asymmetric-rejection harm. This ladder has no such unit to stamp: §7 sequences
// its two legs precisely because the spot cap is the CONFIRMED perp reduction,
// which does not exist until the perp leg has finalised. Two legs separated by a
// venue round trip and a finality wait cannot share an admission instant, and
// pretending they could — stamping the perp leg's instant and reusing it for the
// spot leg — would be a freshness claim about a row nobody re-read. So each leg
// is its own admission, judged at its own instant, and that is also why the pair
// could not simply be pushed down into Dispatch, whose per-leg clock read WOULD
// re-open §62d inside the executor's fan-out. filterPolicy carries the argument.
//
// # A refusal here does not STRAND the ladder, and the distinction is the reason
// # this check is admissible where the premise guard was not
//
// SubmitSpotReduction is deliberately un-premise-guarded because refusing to
// send an already-reserved spot reduction does not PREVENT a commitment, it
// strands one — TLC returned a 16-state stuttering behaviour for the guarded
// variant. This check looks like the same shape and is not, for two reasons. An
// off-grid order is one the VENUE refuses too, so refusing locally forfeits
// nothing and gains a diagnosis; the premise guard would have refused an order
// the venue would have filled. And the refusal is RECOVERABLE where a venue
// rejection is not: everything here is a read, before any generation record is
// journaled and before the aggregate advances, so the generation stays exactly
// where it was and the same rung can be re-presented against a refreshed map.
// A venue rejection burns the ClientOrderID's single unrepeatable Place.
//
// # It runs LAST, on the preflight's own ordering argument
//
// The four checks above give sharper diagnoses for the failures they name — a
// missized leg reported as ErrGenerationLegMissized names the generation's
// requirement, where the same order reported as off-lot-step names the venue's
// grid and sends an operator to the wrong map. Same reason the executor runs its
// filter pass second, after the package-shape checks.
func (d *Deleverer) verifyLeg(g *domain.Generation, ref DispatchRef, role legRole,
	want fixedpoint.Quantity) (ports.OrderRecord, error) {
	if ref.ID == "" {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg: %w",
			g.Group(), role, domain.ErrGenerationLegUnidentified)
	}
	rec, err := d.orders.Load(ref.ID)
	if err != nil {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q: %w", g.Group(), role, ref.ID, err)
	}
	if rec.Version != ref.Version {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q is at version %d, expected %d: %w",
			g.Group(), role, ref.ID, rec.Version, ref.Version, ports.ErrVersionConflict)
	}
	if !g.Group().Holds(rec.Order.Instrument) {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q is on %s: %w",
			g.Group(), role, ref.ID, rec.Order.Instrument.Symbol, domain.ErrOrderOutsideHedgeGroup)
	}
	if got := legRoleOf(rec.Order.Instrument.Product); got != role {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q is a %s order on %s: %w",
			g.Group(), role, ref.ID, rec.Order.Instrument.Product, got, ErrGenerationLegWrongProduct)
	}
	cmp, err := rec.Order.Quantity.Cmp(want)
	if err != nil {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q quantity: %w", g.Group(), role, ref.ID, err)
	}
	if cmp != 0 {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q is sized %s, the generation requires %s: %w",
			g.Group(), role, ref.ID, rec.Order.Quantity, want, ErrGenerationLegMissized)
	}
	if err := d.filters.admits(rec.Order.Instrument, rec.Order.Quantity,
		rec.Order.LimitPrice, d.clk.Now()); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("delever %s: %s leg %q: %w", g.Group(), role, ref.ID, err)
	}
	return rec, nil
}

// legRoleOf maps a product category onto the generation leg it can serve.
//
// Inverse perps answer "perp" alongside linear ones because §7's sequencing is
// about which SIDE of the cash-and-carry an order reduces, and both are the
// short leg. Whether this system trades inverse contracts at all is §2.2's
// question and is settled elsewhere; answering it here would put a roster
// decision inside a sequencing check.
func legRoleOf(p ids.ProductCategory) legRole {
	if p.IsPerp() {
		return legPerp
	}
	return legSpot
}
