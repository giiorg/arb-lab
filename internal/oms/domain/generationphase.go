package domain

import "fmt"

// GenerationPhase is SM §7's `PackageDelever` generation phase set — the states
// of the escalation ladder core §7.1 reaches when isolated-margin liq distance
// breaches and the top-up path fails.
//
// # It is `type X string` for OrderState's reason, one machine over
//
// §3.6 rule 8's shape is for enums crossing persistence or wire, and this one
// crosses the same boundary OrderState does: SM §7's `SpotReduceAuthorized` is a
// DURABLE journal record (FLAG-3), written so that recovery "knows the
// perp-finality precondition was met without re-derivation". Under an integer
// encoding, inserting a phase renumbers every stored row — and the phases are
// ordered by ladder progress, so a shift by one turns `PerpReduceFinal` into
// `SpotReduceAuthorized`. Boot would then believe a spot sale was authorized
// that no perp reduction ever confirmed, which is the one ordering §7's
// never-net-short invariant exists to enforce.
//
// # `Held/Alert` is ONE row in SM §7 and TWO members here, and the split is the model's
//
// SM §7's table has a single `Held/Alert` state entered from either leg. Its
// resume row then says "the HELD LEG's §6.7 contract later reaches finality" —
// so the machine must know WHICH leg parked, and a single state plus a
// remembered leg identity is two states with extra steps.
//
// Formal model (a) makes the split (`Held`, `HeldPerp`) and slice E is why: §10a
// had narrowed the perp-side park away as "the harmless instance — at the perp
// leg neither the unsold transient nor a nonzero reservation exists yet
// (pdDone = 0)". Building it refuted that. `PdPerpFill` fires while the leg is
// in flight and amends the ledger entry upward, so a perp leg may partially fill
// and THEN go ambiguous — §2's own `PartiallyFilled` case — and such a park
// holds a REAL net-long transient and a NONZERO reservation, while blocking the
// next generation on §7's finality gate. `pdDone = 0` is the park taken before
// any fill: one instance, not the case.
//
// The two also differ in what supersession must do, which is the sharper reason
// they cannot share a member: the reserved SECOND leg is live at a spot-side
// park and does not exist at a perp-side one, so the quiesce-first obligation
// binds on one and has nothing to bind on at the other.
type GenerationPhase string

const (
	// GenerationPlanned is SM §7's `Planned`: "a generation is created",
	// carrying "explicit per-leg target positions".
	//
	// Its ledger entry exists from creation and its amount is ZERO — the perp
	// leg has bought nothing, so the outstanding net intent is nothing. That is
	// FM-5's fix, and booking the TARGET here instead is what made a still-
	// neutral package read as short by the target.
	GenerationPlanned GenerationPhase = "planned"

	// GenerationPerpReduceInFlight is the perp-short reduction (buy-to-reduce)
	// submitted FIRST — "which itself relieves the liq risk".
	//
	// Submitting it drives the §6.7 classifier out of Determined for the whole
	// hedge group, and §7 DEPENDS on that: it is how §2 refuses every
	// compensating trade while the leg is live. It is also why the premise a
	// generation is judged against cannot be the classifier's raw verdict —
	// FM-9, and the whole of Premise below.
	GenerationPerpReduceInFlight GenerationPhase = "perp_reduce_in_flight"

	// GenerationPerpReduceFinal is the perp reduction confirmed to §6.7
	// finality. Confirmed is not the same as fully executed: the leg's interval
	// may collapse with less executed than targeted, and "no more spot than the
	// CONFIRMED perp reduction" is sized from what actually confirmed.
	GenerationPerpReduceFinal GenerationPhase = "perp_reduce_final"

	// GenerationSpotReduceAuthorized is FLAG-3's DURABLE journal record, entered
	// on perp-reduce finality and fsynced before the spot dispatch — the §5.3
	// crash-boundary discipline at generation scope, so recovery reads it and
	// knows the perp-finality precondition was met.
	GenerationSpotReduceAuthorized GenerationPhase = "spot_reduce_authorized"

	// GenerationSpotReduceInFlight is the reserved spot reduction submitted.
	GenerationSpotReduceInFlight GenerationPhase = "spot_reduce_in_flight"

	// GenerationHeldPerp is SM §7's `Held/Alert` entered from the PERP leg: a leg
	// that is deadline-exceeded or ambiguous "holds and alerts, never
	// blind-retries or blind-flattens". FLAG-4 rules it NONTERMINAL.
	GenerationHeldPerp GenerationPhase = "held_perp"

	// GenerationHeldSpot is SM §7's `Held/Alert` entered from the reserved SPOT
	// leg. Same rule, and the phase at which the reserved second leg is out at
	// the venue with its outcome unknown — so this is one of the two phases the
	// quiesce-before-supersession obligation binds on.
	GenerationHeldSpot GenerationPhase = "held_spot"

	// GenerationComplete is "the generation has reached §6.7 finality; only now
	// is a next generation admissible".
	GenerationComplete GenerationPhase = "complete"

	// GenerationTerminalFlatten is "if it cannot complete, its terminal
	// full-flatten routes through the same §6.7 finality contract (the
	// liquidation-imminent branch), not a blind market dump".
	GenerationTerminalFlatten GenerationPhase = "terminal_flatten"

	// GenerationSuperseded is the generation ended by one of SM §7's three
	// supersession rows — `EmergencyClosePackage` (core §5.3), the
	// `FenceReadyLost` arbitration (core §7.4) and the premise-lost row
	// (`[→ FM-8]`).
	//
	// # It is a PHASE here and a variable assignment in the model, deliberately
	//
	// Model (a)'s `Supersede` sets `pd' = "None"`, folding a superseded
	// generation into the same value as a retired one and distinguishing them
	// through a separate `superseded` variable. That is right for a model whose
	// state space is the thing being minimised, and wrong here: "which phase is
	// a superseded generation in" must have an answer a switch can be made to
	// handle, and folding it into absence puts the answer in a second field that
	// §3.6 rule 10 cannot police.
	//
	// Nothing is claimed by the split that the model does not check. The phase
	// is terminal, has no successors, and is reached only through Supersede.
	GenerationSuperseded GenerationPhase = "superseded"
)

// Validate reports whether p is one of the declared phases.
func (p GenerationPhase) Validate() error {
	switch p {
	case GenerationPlanned, GenerationPerpReduceInFlight, GenerationPerpReduceFinal,
		GenerationSpotReduceAuthorized, GenerationSpotReduceInFlight,
		GenerationHeldPerp, GenerationHeldSpot,
		GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return nil
	case "":
		return ErrGenerationPhaseUnset
	}
	return fmt.Errorf("%q: %w", string(p), ErrGenerationPhaseUnknown)
}

func (p GenerationPhase) String() string { return string(p) }

// IsActive reports whether the generation still has ladder left to run — model
// (a)'s `PdActive`, and the subject of the premise-lost supersession row.
//
// # Its false-set is the three ENDED phases, and that is not "is terminal"
//
// A generation at `Complete` has reached §6.7 finality and there is nothing left
// to supersede; the same is true at `TerminalFlatten` and `Superseded`. Every
// other phase — including both parks — is active, because FLAG-4 rules the park
// NONTERMINAL and a parked generation is one whose leg the machine is still
// waiting on, not one that has finished.
//
// The (bool, error) shape and the leading Validate are OrderState.IsTerminal's,
// for its reason: without them the false group here cannot be checked by any
// test, and the false group is where "there is nothing left to supersede" lives.
func (p GenerationPhase) IsActive() (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	switch p {
	case GenerationPlanned, GenerationPerpReduceInFlight, GenerationPerpReduceFinal,
		GenerationSpotReduceAuthorized, GenerationSpotReduceInFlight,
		GenerationHeldPerp, GenerationHeldSpot:
		return true, nil
	case GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(p), ErrGenerationPhaseUnhandled)
}

// ReservedLegLive reports whether the generation's reserved SECOND leg is out at
// the venue with its outcome not yet final — model (a)'s `PdLegLive`.
//
// This is precisely what SM §7's supersession rows require to be quiesced to
// §6.7 finality FIRST, and its true-set is two phases: the spot leg submitted,
// and the spot-side park.
//
// # `HeldPerp` is NOT in it, and the scoping is an observation rather than a proof
//
// At a perp-side park the reserved second leg has not been submitted — the
// ledger entry exists (FLAG-3, from creation) but no spot order is out — so the
// quiesce precondition has nothing to bind on and supersession is admissible
// without it. Model (a) found that behaviour with TLC rather than by design, and
// recorded it as scoping rather than as safety: §7's quiesce clause is scoped to
// the reserved SECOND leg, and although supersession drops the perp leg's
// remaining interval with finality semantics and both external paths carry their
// own quiesce barrier over all resting orders (core §7.4), that is an
// observation the model makes and not a claim it verifies.
//
// `SpotReduceAuthorized` is not in it either, for the sharper reason: the amount
// is fixed and journaled but nothing has been sent, so there is no venue-side
// commitment to quiesce.
func (p GenerationPhase) ReservedLegLive() (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	switch p {
	case GenerationSpotReduceInFlight, GenerationHeldSpot:
		return true, nil
	case GenerationPlanned, GenerationPerpReduceInFlight, GenerationPerpReduceFinal,
		GenerationSpotReduceAuthorized, GenerationHeldPerp,
		GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(p), ErrGenerationPhaseUnhandled)
}

// CommitsNewExposure reports whether ADVANCING INTO this phase commits the
// package to an exposure change it is not already committed to.
//
// It is the standing obligation's scope (`[→ FM-8]`), and every member of its
// true-set and false-set was established by a model run rather than by argument.
//
// # Why the obligation is scoped to steps rather than asserted over states
//
// SM §7's standing rule is NOT "a generation never coexists with a dissolved
// premise". That would forbid the very window the section mandates — the reserved
// second leg must reach §6.7 finality FIRST — and could be satisfied only by
// violating the quiesce rule. It is: while the premise is dissolved the machine
// MUST NOT take the generation further.
//
// So the true-set is the two decisions that commit something new: submitting the
// perp leg, and authorizing a spot sale. The false-set is everything the quiesce
// window is MADE of:
//
//   - venue events and both parks — refusing those does not prevent a
//     commitment, it strands one, which is the harm the quiesce rule forbids;
//   - `SpotReduceInFlight`. This is the exclusion found by a RUN and not by
//     design, and it is the nominal liveness configuration that found it. The
//     amount was fixed at authorization and the entry is already in the shared
//     ledger, so sending it completes a decision already taken. Guarded, TLC
//     returned a 16-state behaviour ending in STUTTERING: a generation buys 1
//     perp, authorizes ZERO spot (releasing its entry, so its transient becomes
//     a genuine naked residual), §2's determined branch correctly compensates
//     it, and the package ends PHYSICALLY FLAT and Determined while the
//     generation sits at `SpotReduceAuthorized` unable to finish. Blocking a
//     step that commits nothing buys no safety and costs termination.
//
// `Complete`, `TerminalFlatten` and `Superseded` are ends rather than
// commitments, and `Planned` is the generation's own entry — guarded by the
// entry precondition itself, not by the standing one.
func (p GenerationPhase) CommitsNewExposure() (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	switch p {
	case GenerationPerpReduceInFlight, GenerationSpotReduceAuthorized:
		return true, nil
	case GenerationPlanned, GenerationPerpReduceFinal, GenerationSpotReduceInFlight,
		GenerationHeldPerp, GenerationHeldSpot,
		GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return false, nil
	}
	return false, fmt.Errorf("%q: %w", string(p), ErrGenerationPhaseUnhandled)
}

// CanAdvanceGeneration reports whether from → to is an edge of SM §7's ladder.
//
// Supersession is deliberately NOT an edge here. Every phase from which a
// generation may still be superseded reaches `Superseded`, so a table containing
// it would say almost nothing; the rule that matters is the quiesce-first guard,
// which is a condition on the generation's LEG and not on its phase pair, and it
// lives on Generation.Supersede where the leg is in hand.
//
// A reachability predicate rather than an (phase, event) → phase function, on
// CanTransition's argument one machine over: two rows resolve to several targets
// (a submitted spot leg reaches `Complete` or parks; a park reaches finality or
// flattens), so the event does not determine the target.
func CanAdvanceGeneration(from, to GenerationPhase) error {
	if err := from.Validate(); err != nil {
		return fmt.Errorf("generation phase source: %w", err)
	}
	if err := to.Validate(); err != nil {
		return fmt.Errorf("generation phase target: %w", err)
	}
	if from == to {
		return fmt.Errorf("%s: %w", from, ErrGenerationNoOp)
	}
	legal, err := generationSuccessors(from)
	if err != nil {
		return fmt.Errorf("generation phase source: %w", err)
	}
	for _, p := range legal {
		if p == to {
			return nil
		}
	}
	return fmt.Errorf("%s -> %s: %w", from, to, ErrGenerationTransitionForbidden)
}

// generationSuccessors is SM §7's transition table, one case per source phase.
//
// A switch rather than a map, for successors' reason: a map literal missing a key
// answers "no legal successors", which is a legitimate answer for three phases
// here, so the omission would be silent. The exhaustiveswitch gate makes it a
// build failure instead.
//
// The error return exists so the THREE ENDED phases' case is distinguishable
// from the fallthrough — both answer "no successors", so with a bare `return nil`
// at the bottom, deleting `GenerationComplete` from the ended case would change
// nothing any test could observe.
func generationSuccessors(from GenerationPhase) ([]GenerationPhase, error) {
	if err := from.Validate(); err != nil {
		return nil, err
	}
	switch from {
	case GenerationPlanned:
		// Perp-short-FIRST. `Planned → SpotReduceAuthorized` is model (a)'s
		// control `PERP_FIRST = FALSE`, and it is the edge that makes the
		// transient net-SHORT in the rally the generation was opened for.
		return []GenerationPhase{GenerationPerpReduceInFlight}, nil

	case GenerationPerpReduceInFlight:
		return []GenerationPhase{GenerationPerpReduceFinal, GenerationHeldPerp}, nil

	case GenerationHeldPerp:
		// FLAG-4: the park is nonterminal, so the HELD leg's own finality
		// resumes the remaining target. `HeldPerp → Complete` is deliberately
		// absent — that is model (a)'s `HELD_EXIT_CAUSE_BLIND` (NC17), the
		// reserved SPOT leg's barrier discharging a park whose PERP leg is the
		// unresolved one, and what it produces is a generation reporting §6.7
		// finality with its perp reduction still live.
		return []GenerationPhase{GenerationPerpReduceFinal, GenerationTerminalFlatten}, nil

	case GenerationPerpReduceFinal:
		return []GenerationPhase{GenerationSpotReduceAuthorized}, nil

	case GenerationSpotReduceAuthorized:
		// `Complete` is the zero-size authorization: a perp reduction that
		// confirmed nothing authorizes no spot sale, and there is nothing to
		// submit. It is not a shortcut — it is the honest extreme of "sells no
		// more spot than the confirmed perp reduction".
		return []GenerationPhase{GenerationSpotReduceInFlight, GenerationComplete}, nil

	case GenerationSpotReduceInFlight:
		return []GenerationPhase{GenerationComplete, GenerationHeldSpot}, nil

	case GenerationHeldSpot:
		return []GenerationPhase{GenerationComplete, GenerationTerminalFlatten}, nil

	case GenerationComplete, GenerationTerminalFlatten, GenerationSuperseded:
		return nil, nil
	}
	return nil, fmt.Errorf("%q: %w", string(from), ErrGenerationPhaseUnhandled)
}
