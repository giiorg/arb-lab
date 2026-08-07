package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// SupersessionPath names which of SM §7's three supersession rows ended a
// generation.
//
// It is carried rather than collapsed to a boolean because the rows are not
// interchangeable: two are external arbitrations that nothing in this machine
// decides when to fire, and the third is triggered by the package's own state.
// An operator reading an alert needs to know which, and — see ResumesLadder —
// so does the escalation ladder.
type SupersessionPath string

const (
	// SupersededByEmergencyClose is core §5.3's `EmergencyClosePackage`: it
	// "supersedes and quiesces any active `PackageDelever` generation
	// (cancelling its reserved second leg to §6.7 finality first)".
	SupersededByEmergencyClose SupersessionPath = "emergency_close_package"

	// SupersededByFenceReadyLost is the core §7.4 / SM §3 arbitration, which
	// "supersedes any active `PackageDelever` generation (quiescing its reserved
	// leg to §6.7 finality first)".
	SupersededByFenceReadyLost SupersessionPath = "fence_ready_lost"

	// SupersededByPremiseLost is the third row, added with FM-8: the generation
	// "opened on 'no naked residual' and that is no longer known to hold",
	// evaluated NET OF THE GENERATION'S OWN LEGS (FM-9).
	SupersededByPremiseLost SupersessionPath = "premise_lost"
)

// Validate reports whether s is one of the declared paths.
func (s SupersessionPath) Validate() error {
	switch s {
	case SupersededByEmergencyClose, SupersededByFenceReadyLost, SupersededByPremiseLost:
		return nil
	case "":
		return ErrSupersessionPathUnset
	}
	return fmt.Errorf("%q: %w", string(s), ErrSupersessionPathUnknown)
}

func (s SupersessionPath) String() string { return string(s) }

// ResumesLadder reports whether core §7.1's escalation ladder may open a LATER
// generation after a generation ended on this path.
//
// # SM §7's ladder-resumption invariant, and the model does NOT answer it
//
// The supersession rows said a generation ends and were silent on whether the
// LADDER does. This function was the ruling; it is now SM §7's own invariant
// ("a supersession ends the GENERATION; whether it ends the LADDER is a property
// of the PATH"), and what follows is why the two classes differ.
//
// Model (a) cannot be read as answering it. Its `Supersede` requires
// `superseded = "none"` and its `PdPlan` requires the same, so ONE supersession
// per behaviour ends generations for that behaviour — a state-space bound of the
// kind §12's roster already flags for `MAX_GEN` ("the model's own bound rather
// than the spec's"), not a rule anybody stated. The companion's clause map read
// it the other way in a parenthetical — "`PdPlan`'s guard `pd = "None"`,
// reachable … (or via `Supersede`)" — which names a route that reaches the guard
// while permanently disabling the action. That parenthetical is CORRECTED in
// `ambiguous-order-model.md` as of this slice; the model is silent rather than
// either.
//
// # The two answers differ, and the failure directions are what decide it
//
// The external paths do NOT resume. `EmergencyClosePackage` is the kill-switch
// flatten and `FenceReadyLost` means writer identity or the fence is in doubt;
// in both, the foundation has taken the package away from the leverage-reduction
// ladder, and a generation opening afterwards would be this machine
// re-authorising itself against an arbitration that just fired. The cost of
// being wrong is that a breach needs a fresh escalation to reach it — an alert.
//
// The premise-lost path DOES resume, and refusing would be the unsafe direction.
// The premise dissolving means the package acquired a naked residual, which §2's
// determined branch then compensates; once it has, the package is neutral again
// and the perp short may still be nearing liquidation. Refusing would disable
// the one mechanism that reduces leverage on a neutral package for the rest of
// the process's life, at exactly the moment it is needed — a stuck state of the
// shape FM-8's own ruling was taken to prevent. Nothing is loosened by resuming:
// SM §7's entry precondition still binds, so a later generation can only open on
// a package that is genuinely Determined and flat again, and core §7.1's ladder
// is "bounded and terminal-escalating" independently of this.
//
// So one rule gives the two classes opposite answers, which is what makes this a
// property of the PATH and not an exemption for one of them.
//
// The (bool, error) shape and the leading Validate are OrderState.IsTerminal's,
// for its reason.
func (s SupersessionPath) ResumesLadder() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	switch s {
	case SupersededByPremiseLost:
		return true, nil
	case SupersededByEmergencyClose, SupersededByFenceReadyLost:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(s), ErrSupersessionPathUnhandled)
}

// GenerationID identifies one `PackageDelever` generation. Like ReservationID it
// is the attempt's own handle: SM §7's "each generation is a bounded attempt"
// means the identity outlives any single order on either leg.
type GenerationID string

// Premise is SM §7's entry test — "the package has no naked residual" on a
// "still-neutral package" — as a value that can be judged.
//
// # It is the ENTRY test and the STANDING trigger, and that is the whole design
//
// FM-8 made "no naked residual" a standing condition rather than a one-time
// guard, and FM-9 is the defect in the clause that resolved it. The quantity a
// live generation must be judged against is this same test RE-EVALUATED NET OF
// THE GENERATION'S OWN LEGS — so entry and trigger are one type, evaluated
// twice, rather than two rules that could drift apart. At entry the netting is
// vacuous (the generation has no legs yet), which is what makes them the same
// test rather than merely similar ones.
//
// Who computes it is deliberately not this layer: netting requires the hedge
// group's order fold and the reserved-reduction ledger, which live one layer out.
// What lives here is the JUDGEMENT, because that is the part §7 states.
type Premise struct {
	// Residual is `realized_delta − outstanding_reserved_reductions` with this
	// generation's own legs removed from BOTH sides of the subtraction — model
	// (a)'s `PremiseResidual == (realized − PdOwnRealized) − (TrueReserved −
	// PdReserved)`.
	//
	// Removing them from both sides is what makes the quantity INVARIANT under
	// every step of the generation's own ladder: a perp fill raises the realized
	// delta and amends the entry by the same amount, and an authorization moves
	// the intent and the transient together. That invariance is FM-5's fix
	// restated as a quantity, and it is why this moves only when something
	// EXTERNAL moves it — which is what "the premise dissolved" means.
	Residual fixedpoint.Quantity

	// Class is §6.7's verdict over the same netted view.
	//
	// It must be the verdict with the generation's own legs excluded, and that
	// is FM-9 itself. The generation's perp reduction is an exposure-changing
	// order on the hedge group, so submitting it drives the RAW verdict out of
	// Determined on the generation's own first step — and §7 depends on that,
	// since it is how §2 refuses every compensating trade while the leg is live.
	// Read raw, the standing trigger fires on the generation's own machinery:
	// model (a)'s control NC22 reaches it four states from Init, with nothing
	// realized, nothing outstanding and no external event whatsoever.
	Class Classification
}

// Intact reports whether the premise still holds — model (a)'s `PremiseIntact`.
func (p Premise) Intact() (bool, error) {
	if err := p.Class.Validate(); err != nil {
		return false, err
	}
	sign, err := p.Residual.Sign()
	if err != nil {
		return false, fmt.Errorf("premise residual: %w", err)
	}
	return sign == 0 && p.Class == ClassificationDetermined, nil
}

// Generation is one `PackageDelever` attempt: SM §7's machine.
//
// # Quantities are unsigned magnitudes, and the direction lives in the phase
//
// `target`, `perpDone`, `spotSold` and `authorized` are all positive: the perp
// leg BUYS to reduce a short and the spot leg SELLS, and which is which is fixed
// by SM §7's sequencing rather than by a sign a caller could get backwards. This
// is domain.Fill's convention — an unsigned quantity so the sign has one home —
// and here the home is OwnRealized, the single place the two legs become one
// signed number.
//
// # Not safe for concurrent use
//
// The Ledger's and the Classifier's argument, unchanged: §5.1 gives the
// aggregate an OCC version and serialisation is decided one layer out.
type Generation struct {
	id          GenerationID
	group       HedgeGroup
	reservation ReservationID
	phase       GenerationPhase

	// target is SM §7's "explicit per-leg target position" for this bounded
	// attempt. It is what the perp leg is submitted for, and it is NEVER what
	// the ledger entry carries — booking the target is FM-5.
	target fixedpoint.Quantity

	// perpDone is the confirmed perp reduction: what the buy-to-reduce has
	// actually executed. It is what "sells no more spot than the CONFIRMED perp
	// reduction" is sized from, and it can end strictly below target.
	perpDone fixedpoint.Quantity

	// spotSold is what the reserved spot leg has executed back.
	spotSold fixedpoint.Quantity

	// authorized is the spot leg's size, fixed at the durable authorization
	// record and never revised — which is why sending it commits nothing new.
	authorized fixedpoint.Quantity

	// perpOrder and spotOrder are this generation's OWN legs, recorded as each
	// is submitted.
	//
	// They are here, and not in a map one layer out, because "net of this
	// generation's own legs" is FM-9's whole quantity and the generation is the
	// only thing that knows which orders those are. Holding the mapping outside
	// would make the netting a caller's discipline — the shape FM-6 came out of
	// — and a netting that silently omitted a leg would compute a premise that
	// still contains its own subject, which is the defect itself.
	perpOrder ids.ClientOrderID
	spotOrder ids.ClientOrderID

	supersededBy SupersessionPath
}

// OpenGeneration creates a generation at `Planned` — SM §7's entry from the
// liq-distance / top-up ladder.
//
// The premise is an argument rather than a check the caller is trusted to have
// made, and it is SM §7's own precondition: core §5.3 defines `PackageDelever`
// as reducing leverage "on a STILL-NEUTRAL package" and §7 says "the package has
// no naked residual". A generation opened over an unresolved or nonzero residual
// is a scenario §7 does not specify, and every invariant below is stated about a
// generation that opened under this guard.
//
// The reservation is NOT created here, and that division is FLAG-3's as amended
// by FM-5: the entry is generation-scoped from creation, so the caller creates it
// in the same step — at ZERO, which is what OutstandingIntent reports for this
// phase. This type cannot create it because the ledger belongs to the classifier,
// which is the whole reason §6.7's mutators return a verdict.
func OpenGeneration(
	id GenerationID,
	group HedgeGroup,
	reservation ReservationID,
	target fixedpoint.Quantity,
	premise Premise,
) (*Generation, error) {
	g, err := newGeneration(id, group, reservation, target)
	if err != nil {
		return nil, err
	}
	intact, err := premise.Intact()
	if err != nil {
		return nil, fmt.Errorf("generation %q premise: %w", id, err)
	}
	if !intact {
		return nil, fmt.Errorf("generation %q: residual %s, %s: %w",
			id, premise.Residual, premise.Class, ErrGenerationPremiseNotIntact)
	}
	return g, nil
}

// RestoreGeneration reinstates a generation at `Planned` from its durable
// opening record — §7.5's rehydration entry point, and OpenGeneration WITHOUT
// the premise.
//
// # The premise is absent because it is not a property of the generation
//
// Every other check OpenGeneration makes is about the RECORD — an id, a group,
// a reservation, a positive target — and a stored value that fails one is a log
// that verifies and does not compose. The premise is not: it is §6.7's verdict
// over the hedge group AT THE INSTANT OF THE DECISION, and the decision was
// taken by a process that is gone. Re-asking it here would judge a generation
// that EXISTS against a world that has moved, and the failure direction settles
// it — a recovered generation refused for a dissolved premise is a generation
// whose ledger entry is never re-`Reserve`d, which is precisely the FM-5-shaped
// hole §7.5's refusal exists to hold. The premise still binds afterwards, and
// with more force: it is FM-8's STANDING obligation, so the reinstating caller
// reassesses immediately and the premise-lost row fires if it has dissolved.
// Refusing to rebuild and discharging the supersession look alike from one step
// away and are opposites — one leaves the exposure unaccounted, the other
// accounts for it and then ends the attempt.
//
// This is the same division §7.5 already makes for orders. `ApplyPlanned` at
// boot does not re-run the planner's admission checks — the filters, the
// quantization, the freshness bound — because those judged a decision that was
// taken; what replay re-checks is SM §1's edge set, which is a property of the
// history. The generation's edge set is checked the same way, by the Restore*
// steps below driving the SAME transition table the live ladder does.
func RestoreGeneration(
	id GenerationID,
	group HedgeGroup,
	reservation ReservationID,
	target fixedpoint.Quantity,
) (*Generation, error) {
	return newGeneration(id, group, reservation, target)
}

// newGeneration is the shape checks both entry points make, in one place so a
// rehydrated generation cannot be laxer about its own record than a fresh one.
func newGeneration(
	id GenerationID,
	group HedgeGroup,
	reservation ReservationID,
	target fixedpoint.Quantity,
) (*Generation, error) {
	if id == "" {
		return nil, ErrGenerationIDUnset
	}
	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("generation %q: %w", id, err)
	}
	if reservation == "" {
		return nil, fmt.Errorf("generation %q: %w", id, ErrReservationUnset)
	}
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("generation %q target: %w", id, err)
	}
	sign, err := target.Sign()
	if err != nil {
		return nil, fmt.Errorf("generation %q target: %w", id, err)
	}
	if sign <= 0 {
		// A generation targeting nothing would run the whole ladder — two
		// journal records, a reservation, a supersession window — and reduce
		// leverage by zero, while blocking the next generation on §7's
		// one-at-a-time gate for as long as it takes.
		return nil, fmt.Errorf("generation %q target %s: %w", id, target, ErrGenerationTargetNotPositive)
	}

	zero := fixedpoint.ZeroQuantity(target.Asset())
	return &Generation{
		id:          id,
		group:       group,
		reservation: reservation,
		phase:       GenerationPlanned,
		target:      target,
		perpDone:    zero,
		spotSold:    zero,
		authorized:  zero,
	}, nil
}

// ID, Group, Reservation and Phase report the generation's identity and where
// the ladder has reached.
func (g *Generation) ID() GenerationID            { return g.id }
func (g *Generation) Group() HedgeGroup           { return g.group }
func (g *Generation) Reservation() ReservationID  { return g.reservation }
func (g *Generation) Phase() GenerationPhase      { return g.phase }
func (g *Generation) Target() fixedpoint.Quantity { return g.target }

// PerpConfirmed is the confirmed perp reduction — the cap on the spot leg.
func (g *Generation) PerpConfirmed() fixedpoint.Quantity { return g.perpDone }

// Authorized is the spot leg's size as fixed at the durable authorization
// record.
func (g *Generation) Authorized() fixedpoint.Quantity { return g.authorized }

// SupersededBy reports which row ended the generation, or the empty path if none
// did.
func (g *Generation) SupersededBy() SupersessionPath { return g.supersededBy }

// OwnLegs names the orders this generation has itself put on the hedge group —
// the set the premise must be evaluated NET OF (FM-9).
//
// It grows as the ladder runs and is empty at `Planned`, which is why the entry
// test and the standing trigger are the same test: with no legs the netting is
// vacuous and the premise IS the package's ordinary residual and verdict.
func (g *Generation) OwnLegs() []ids.ClientOrderID {
	legs := make([]ids.ClientOrderID, 0, 2)
	if g.perpOrder != "" {
		legs = append(legs, g.perpOrder)
	}
	if g.spotOrder != "" {
		legs = append(legs, g.spotOrder)
	}
	return legs
}

// OwnRealized is the generation's own signed contribution to the hedge group's
// realized delta — model (a)'s `PdOwnRealized == pdDone − pdSold`.
//
// It is POSITIVE while the ladder runs, and that sign is SM §7's central safety
// claim rather than an accident of the arithmetic: the perp short is reduced
// first and the spot leg "sells no more spot than the confirmed perp reduction",
// so the momentary exposure is net-LONG — safe in the triggering rally — and
// never net-short.
func (g *Generation) OwnRealized() (fixedpoint.Quantity, error) {
	return g.perpDone.Sub(g.spotSold)
}

// OutstandingIntent is what this generation's reserved-reduction ledger entry
// must carry at the current phase — §6.7's "the exposure change its reserving
// action has already COMMITTED and not yet REALIZED, never a target it merely
// intends".
//
// It exists so the ledger and the ladder agree BY CONSTRUCTION rather than by a
// caller's discipline. FM-5 is what a disagreement costs, and it is not
// detectable downstream: §2's sizing subtracts this ledger unconditionally, so
// an entry that does not match the generation's actual commitment is
// indistinguishable from a real position, and every check that would object is
// computed from the same subtraction.
//
// The phases divide into three groups and each one's amount is a different
// sentence of §7:
//
//   - before authorization the amount is the OUTSTANDING NET INTENT — the perp
//     reduction confirmed and not yet matched by the sale — which is zero at
//     `Planned` (the leg has bought nothing) and grows as it confirms;
//   - from the durable authorization onward it is the AUTHORIZED SIZE, because
//     that is what the generation has committed to sell and it may be strictly
//     less than the transient (§7's "no MORE spot than", where "no more" and
//     "exactly" are different predicates);
//   - once ended the entry is retired and subtracts nothing.
func (g *Generation) OutstandingIntent() (fixedpoint.Quantity, error) {
	if err := g.phase.Validate(); err != nil {
		return fixedpoint.Quantity{}, err
	}
	switch g.phase {
	case GenerationPlanned, GenerationPerpReduceInFlight, GenerationPerpReduceFinal,
		GenerationHeldPerp:
		return g.OwnRealized()
	case GenerationSpotReduceAuthorized, GenerationSpotReduceInFlight, GenerationHeldSpot:
		return g.authorized, nil
	case GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return fixedpoint.ZeroQuantity(g.target.Asset()), nil
	}
	return fixedpoint.Quantity{}, fmt.Errorf("%q: %w", string(g.phase), ErrGenerationPhaseUnhandled)
}

// SubmitPerpReduction advances `Planned` → `PerpReduceInFlight`: SM §7's
// "reduces the perp short FIRST (buy-to-reduce — which itself relieves the liq
// risk)".
//
// It takes the premise because this step COMMITS NEW EXPOSURE and is therefore
// inside FM-8's standing obligation. See GenerationPhase.CommitsNewExposure for
// which steps are and are not, and why each boundary was settled by a model run.
func (g *Generation) SubmitPerpReduction(order ids.ClientOrderID, premise Premise) error {
	return g.advance(GenerationPerpReduceInFlight, premise, g.applyPerpSubmission(order))
}

// RestorePerpSubmission replays the perp-leg submission at §7.5 boot:
// SubmitPerpReduction with the premise guard and nothing else removed.
//
// The payload rule is shared rather than restated (applyPerpSubmission) and the
// edge check is `step2`'s, which is the same CanAdvanceGeneration the live step
// reaches through `advance` — so the transition table and the leg-id rule each
// keep exactly one home, and the only difference between the two entry points
// is the one RestoreGeneration argues for.
func (g *Generation) RestorePerpSubmission(order ids.ClientOrderID) error {
	return g.step2(GenerationPerpReduceInFlight, g.applyPerpSubmission(order))
}

func (g *Generation) applyPerpSubmission(order ids.ClientOrderID) func() error {
	return func() error {
		if order == "" {
			return fmt.Errorf("generation %q perp leg: %w", g.id, ErrGenerationLegUnidentified)
		}
		g.perpOrder = order
		return nil
	}
}

// RecordPerpFill raises the confirmed perp reduction by an executed quantity.
//
// It is not a phase change — the leg stays in flight — and it is deliberately
// NOT guarded by the premise: a venue fill is an event, not a decision, and
// refusing to record one would not prevent a commitment, it would lose one.
//
// The caller's obligation after this is to amend the ledger entry to the new
// OutstandingIntent, which §6.7 requires and which is a re-classification
// trigger emitting no order event of its own (FM-6).
func (g *Generation) RecordPerpFill(executed fixedpoint.Quantity) error {
	if g.phase != GenerationPerpReduceInFlight {
		return fmt.Errorf("generation %q at %s: %w", g.id, g.phase, ErrGenerationLegNotInFlight)
	}
	if err := executed.Validate(); err != nil {
		return fmt.Errorf("generation %q perp fill: %w", g.id, err)
	}
	sign, err := executed.Sign()
	if err != nil {
		return fmt.Errorf("generation %q perp fill: %w", g.id, err)
	}
	if sign <= 0 {
		return fmt.Errorf("generation %q perp fill %s: %w", g.id, executed, ErrGenerationFillNotPositive)
	}
	done, err := g.perpDone.Add(executed)
	if err != nil {
		return fmt.Errorf("generation %q perp fill: %w", g.id, err)
	}
	// The target bounds the leg because it bounds the ORDER: SM §7's generation
	// carries explicit per-leg target positions and the perp leg is submitted
	// for exactly that. An excess is a fill attributed to the wrong generation
	// or a double-counted event, and admitting it would raise the reserved
	// entry above what the generation can ever sell back — a permanent
	// understatement of the group's residual.
	over, err := done.Sub(g.target)
	if err != nil {
		return fmt.Errorf("generation %q perp fill: %w", g.id, err)
	}
	overSign, err := over.Sign()
	if err != nil {
		return fmt.Errorf("generation %q perp fill: %w", g.id, err)
	}
	if overSign > 0 {
		return fmt.Errorf("generation %q: %s executed against target %s: %w",
			g.id, done, g.target, ErrGenerationPerpOvershootsTarget)
	}
	g.perpDone = done
	return nil
}

// ParkPerpLeg advances `PerpReduceInFlight` → `HeldPerp`: SM §7's "a leg that is
// deadline-exceeded/ambiguous holds and alerts, never blind-retries or
// blind-flattens".
//
// Unguarded by the premise, on CommitsNewExposure's rule: a park is what a
// quiesce window is made of.
func (g *Generation) ParkPerpLeg() error {
	return g.step(GenerationHeldPerp)
}

// ConfirmPerpReduction advances to `PerpReduceFinal` from the in-flight leg or
// from its park — SM §7's "confirms that reduction to finality (§6.7)" before
// any spot sell, and FLAG-4's ruling that the park is nonterminal so the held
// leg's own finality resumes the target.
//
// Finality is not full execution: the leg's interval may collapse with less
// executed than targeted, which is the partial confirmation "no more spot than
// the confirmed perp reduction" exists to size from.
func (g *Generation) ConfirmPerpReduction() error {
	return g.step(GenerationPerpReduceFinal)
}

// AuthorizeSpotReduction advances `PerpReduceFinal` → `SpotReduceAuthorized`,
// fixing the spot leg's size: FLAG-3's durable journal record, whose durability
// is the caller's to provide.
//
// Two guards, and they are separate rules:
//
//   - the premise, because this step commits new exposure (FM-8);
//   - the size against the CONFIRMED perp reduction, which is §7's
//     never-net-short sequencing. Zero is legal and is the honest extreme: a
//     perp reduction that confirmed nothing authorizes no spot sale, the entry
//     is retired, and the generation completes with nothing to submit.
func (g *Generation) AuthorizeSpotReduction(size fixedpoint.Quantity, premise Premise) error {
	return g.advance(GenerationSpotReduceAuthorized, premise, g.applySpotAuthorization(size))
}

// RestoreSpotAuthorization replays FLAG-3's record at §7.5 boot — the premise
// guard removed, and the never-net-short cap emphatically NOT removed.
//
// The cap is what makes this the sharpest of the restore steps: `CheckSpotSize`
// compares the recorded authorization against the CONFIRMED perp reduction, and
// at recovery that confirmation is rebuilt from the venue rather than from the
// log (there is no perp-fill record — see the caller). So a journal claiming an
// authorization of s over a perp leg the venue says executed less than s is a
// contradiction between two sources, and this refuses it rather than
// reinstating a generation whose ledger entry claims a sale it could never
// back. The direction is the safe one: the refusal holds the boot, where
// admitting it would put a too-large entry into the shared ledger and understate
// the group's residual for the life of the process.
func (g *Generation) RestoreSpotAuthorization(size fixedpoint.Quantity) error {
	return g.step2(GenerationSpotReduceAuthorized, g.applySpotAuthorization(size))
}

func (g *Generation) applySpotAuthorization(size fixedpoint.Quantity) func() error {
	return func() error {
		if err := g.CheckSpotSize(size); err != nil {
			return err
		}
		g.authorized = size
		return nil
	}
}

// CheckSpotSize is §7's never-net-short cap as a question rather than an act:
// the spot authorization must be non-negative and no greater than the CONFIRMED
// perp reduction.
//
// It exists because FLAG-3's record is durable and the journal is append-only,
// so the authorization has to be written BEFORE the aggregate carries it — and a
// record written for a size the aggregate then refuses describes a decision that
// never took effect, in the one store recovery reads. That is Planner's
// three-pass rule (check every leg, then journal every leg, then apply every
// leg) narrowed to a single subject, and the reason it is a METHOD rather than a
// copy of the comparison in the use case is that the cap is §7's central safety
// claim: a second home for it is a second thing to keep in step, and the two
// disagreeing is a spot sale larger than the perp buy that preceded it.
//
// Zero passes. See AuthorizeSpotReduction for why that is the honest extreme
// rather than a hole.
func (g *Generation) CheckSpotSize(size fixedpoint.Quantity) error {
	if err := size.Validate(); err != nil {
		return fmt.Errorf("generation %q spot size: %w", g.id, err)
	}
	sign, err := size.Sign()
	if err != nil {
		return fmt.Errorf("generation %q spot size: %w", g.id, err)
	}
	if sign < 0 {
		return fmt.Errorf("generation %q spot size %s: %w", g.id, size, ErrGenerationSpotSizeNegative)
	}
	excess, err := size.Sub(g.perpDone)
	if err != nil {
		return fmt.Errorf("generation %q spot size: %w", g.id, err)
	}
	excessSign, err := excess.Sign()
	if err != nil {
		return fmt.Errorf("generation %q spot size: %w", g.id, err)
	}
	if excessSign > 0 {
		return fmt.Errorf("generation %q: spot %s against confirmed perp %s: %w",
			g.id, size, g.perpDone, ErrGenerationSpotExceedsConfirmedPerp)
	}
	return nil
}

// CheckAdvance reports whether the generation may advance to `to` under
// `premise` — everything advance decides, decided WITHOUT deciding it.
//
// It is the seam the durable ladder needs and the in-memory one did not. Every
// phase this use case journals is written before the aggregate carries it, so a
// caller must be able to establish that a step is admissible while the log is
// still clean. Splitting it out rather than re-deriving the conditions in the
// application layer is the same argument CheckSpotSize makes: FM-8's standing
// obligation and SM §7's edge set each get exactly one home, and `advance`
// below is a caller of this rather than a second copy of it.
//
// What it deliberately does NOT cover is each step's own payload check — the
// leg id, the fill's sign, the spot size. Those live with the steps that carry
// them, and CheckSpotSize is the one a durable step needs ahead of time.
func (g *Generation) CheckAdvance(to GenerationPhase, premise Premise) error {
	commits, err := to.CommitsNewExposure()
	if err != nil {
		return fmt.Errorf("generation %q: %w", g.id, err)
	}
	if commits {
		intact, err := premise.Intact()
		if err != nil {
			return fmt.Errorf("generation %q premise: %w", g.id, err)
		}
		if !intact {
			return fmt.Errorf("generation %q to %s: residual %s, %s: %w",
				g.id, to, premise.Residual, premise.Class, ErrGenerationPremiseDissolved)
		}
	}
	if err := CanAdvanceGeneration(g.phase, to); err != nil {
		return fmt.Errorf("generation %q: %w", g.id, err)
	}
	return nil
}

// SubmitSpotReduction advances `SpotReduceAuthorized` to the leg in flight, or
// straight to `Complete` when the authorized size is zero and there is nothing
// to send.
//
// It is deliberately NOT guarded by the premise even though it puts an order on
// the wire, and the reason is in CommitsNewExposure: the amount was fixed at
// authorization and the entry is already in the shared ledger, so refusing to
// send it does not prevent a commitment — it strands one. That exclusion was
// found by a model run, not by design.
func (g *Generation) SubmitSpotReduction(order ids.ClientOrderID) error {
	sign, err := g.authorized.Sign()
	if err != nil {
		return fmt.Errorf("generation %q authorized: %w", g.id, err)
	}
	if sign == 0 {
		// Nothing was sent, so there is no leg to name and no leg to net out.
		// An id supplied here would enter OwnLegs and remove an order this
		// generation does not own from the premise — FM-9 arriving through the
		// netting rather than through the trigger.
		if order != "" {
			return fmt.Errorf("generation %q names order %q against a zero authorization: %w",
				g.id, order, ErrGenerationLegNotSubmitted)
		}
		return g.step(GenerationComplete)
	}
	return g.step2(GenerationSpotReduceInFlight, func() error {
		if order == "" {
			return fmt.Errorf("generation %q spot leg: %w", g.id, ErrGenerationLegUnidentified)
		}
		g.spotOrder = order
		return nil
	})
}

// ParkSpotLeg advances `SpotReduceInFlight` → `HeldSpot`. This is the park at
// which the reserved second leg is genuinely out at the venue, so it is the one
// the quiesce-before-supersession obligation binds on.
func (g *Generation) ParkSpotLeg() error {
	return g.step(GenerationHeldSpot)
}

// ConfirmSpotReduction advances to `Complete` from the in-flight spot leg or
// from its park, recording what actually executed.
//
// A partial fill's remainder is NOT re-sent — SM §7's "never blind-retries" — so
// the executed quantity may be less than authorized and the generation still
// completes. What it may not be is MORE: the authorization is the cap the
// never-net-short sequencing rests on, and an over-execution is exposure the
// ladder did not authorize.
func (g *Generation) ConfirmSpotReduction(executed fixedpoint.Quantity) error {
	return g.step2(GenerationComplete, func() error {
		if err := executed.Validate(); err != nil {
			return fmt.Errorf("generation %q spot fill: %w", g.id, err)
		}
		sign, err := executed.Sign()
		if err != nil {
			return fmt.Errorf("generation %q spot fill: %w", g.id, err)
		}
		if sign < 0 {
			return fmt.Errorf("generation %q spot fill %s: %w", g.id, executed, ErrGenerationFillNotPositive)
		}
		excess, err := executed.Sub(g.authorized)
		if err != nil {
			return fmt.Errorf("generation %q spot fill: %w", g.id, err)
		}
		excessSign, err := excess.Sign()
		if err != nil {
			return fmt.Errorf("generation %q spot fill: %w", g.id, err)
		}
		if excessSign > 0 {
			return fmt.Errorf("generation %q: spot executed %s against authorized %s: %w",
				g.id, executed, g.authorized, ErrGenerationSpotExceedsAuthorized)
		}
		sold, err := g.spotSold.Add(executed)
		if err != nil {
			return fmt.Errorf("generation %q spot fill: %w", g.id, err)
		}
		g.spotSold = sold
		return nil
	})
}

// TerminalFlatten advances a park to `TerminalFlatten`: FLAG-4's "if completion
// is impossible or the liquidation policy's imminence branch fires". The flatten
// itself "routes through the same §6.7 finality contract, not a blind market
// dump", and issuing it is the caller's.
func (g *Generation) TerminalFlatten() error {
	return g.step(GenerationTerminalFlatten)
}

// Supersede ends the generation on one of SM §7's three rows.
//
// # The quiesce-first obligation is enforced here and not in the transition table
//
// All three paths "cancel/quiesce the generation's reserved second leg to §6.7
// finality FIRST". That is a condition on the LEG, not on the phase pair, which
// is why supersession is not an edge of CanAdvanceGeneration: refusing it while
// the leg is live is what makes "quiesce first" a rule rather than a hope, and
// the caller's answer is to drive the leg to finality and supersede then.
//
// At a perp-side park there is no reserved second leg out, so the obligation has
// nothing to bind on and supersession is admitted — see
// GenerationPhase.ReservedLegLive, where that scoping is recorded as an
// observation model (a) makes rather than one it verifies.
//
// # The three rows are NOT guarded alike, and collapsing them breaks the flow
//
// Model (a) guards the two EXTERNAL rows on `pd # "None"` alone and the
// premise-lost row on `PdActive` as well, and the difference is load-bearing
// rather than incidental:
//
//   - quiescing a live reserved leg to finality is what the obligation above
//     DEMANDS, and doing it drives the generation to `Complete`. So an
//     `EmergencyClosePackage` arriving against a generation with its spot leg
//     out is refused, the caller quiesces, and the row then fires against a
//     COMPLETE generation. Requiring the generation to still be active would
//     make the two rules contradict each other: the only way to satisfy the
//     first would be to fall out of the second's scope, and the arbitration
//     would silently never be recorded;
//   - the premise-lost row is the generation's OWN standing obligation, and a
//     generation that has reached §6.7 finality has nothing left to take
//     further. Model (a) states that outright — "a generation already at
//     Complete/TerminalFlatten has reached §6.7 finality and there is nothing
//     left to supersede". Admitting it there would record a premise loss against
//     an attempt that had already stopped.
//
// What the external rows still do against a completed generation is stop the
// LADDER — see SupersessionPath.ResumesLadder, which is where that answer lives.
//
// A generation already superseded is refused rather than re-superseded: the
// first path is what an operator will be shown, and overwriting it would make
// the alert name whichever arbitration arrived last.
func (g *Generation) Supersede(path SupersessionPath) error {
	if err := g.CheckSupersede(path); err != nil {
		return err
	}
	g.phase = GenerationSuperseded
	g.supersededBy = path
	return nil
}

// CheckSupersede is everything Supersede decides, decided without deciding it —
// CheckAdvance's seam for the one ending that is not an edge of the ladder.
//
// A supersession is journaled before it is applied, like every other phase, and
// the record carries its PATH: a row written for a supersession the aggregate
// then refuses would tell recovery that a generation ended on an arbitration
// that never fired, and — through SupersessionPath.ResumesLadder — would decide
// the next question wrongly in whichever direction that path happens to answer.
func (g *Generation) CheckSupersede(path SupersessionPath) error {
	if err := path.Validate(); err != nil {
		return fmt.Errorf("generation %q: %w", g.id, err)
	}
	if g.phase == GenerationSuperseded {
		return fmt.Errorf("generation %q superseded by %s: %w", g.id, g.supersededBy, ErrGenerationAlreadySuperseded)
	}
	if path == SupersededByPremiseLost {
		active, err := g.phase.IsActive()
		if err != nil {
			return fmt.Errorf("generation %q: %w", g.id, err)
		}
		if !active {
			return fmt.Errorf("generation %q at %s: %w", g.id, g.phase, ErrGenerationNotActive)
		}
	}
	live, err := g.phase.ReservedLegLive()
	if err != nil {
		return fmt.Errorf("generation %q: %w", g.id, err)
	}
	if live {
		return fmt.Errorf("generation %q at %s, path %s: %w",
			g.id, g.phase, path, ErrGenerationLegNotQuiesced)
	}
	return nil
}

// advance is a phase step guarded by FM-8's standing obligation.
//
// The premise is checked ONLY for steps that commit new exposure, and the check
// is driven off GenerationPhase.CommitsNewExposure rather than off the caller —
// so the obligation's scope is stated once, in the place §3.6 rule 10 polices,
// instead of being re-decided at each call site.
func (g *Generation) advance(to GenerationPhase, premise Premise, apply func() error) error {
	if err := g.CheckAdvance(to, premise); err != nil {
		return err
	}
	return g.step2(to, apply)
}

func (g *Generation) step(to GenerationPhase) error {
	return g.step2(to, func() error { return nil })
}

func (g *Generation) step2(to GenerationPhase, apply func() error) error {
	if err := CanAdvanceGeneration(g.phase, to); err != nil {
		return fmt.Errorf("generation %q: %w", g.id, err)
	}
	if err := apply(); err != nil {
		return err
	}
	g.phase = to
	return nil
}
