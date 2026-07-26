package boot

import (
	"fmt"
	"sort"
)

// Gate is one of §7.5's named preconditions on trading resuming.
//
// A closed vocabulary, and that is a departure from the "machinery, not a
// table" split platform/ratelimit, platform/config and platform/apperror all
// took. The reason the precedent does not carry: those packages refused to hold
// data the spec deliberately leaves to the caller — venue budgets that
// venue-profiles.md does not record, risk caps whose meaning belongs to the
// context that owns them. §7.5 is the opposite case. It NAMES these
// preconditions normatively and the list is closed, so the roster is spec
// content rather than caller content, and holding it here is what lets an
// unreported gate be a refusal that says which one.
//
// What is NOT here is how any of them is established. This package holds the
// checklist; the journal join, the net-delta computation and the posture REST
// calls belong to oms, risk and engine/venue. See the package doc.
type Gate uint8

const (
	// GateUnspecified is the zero value and names no precondition.
	GateUnspecified Gate = iota

	// GateJournalReplayed is state rebuilt from the write-ahead journal —
	// §7.5's first half, before any venue call.
	GateJournalReplayed

	// GateVenueJoined is the journal joined to venue truth: OrderByClientID,
	// Fills, Positions, FundingLedger. Known journaled client IDs are
	// auto-recovered; foreign orders and unexplained position deltas are not,
	// and they are the canonical OutcomeNeedsOperator (§7.5: they "alert for
	// operator clearance", they "are not auto-corrected").
	GateVenueJoined

	// GateNetDeltaAsserted is §7.5's net-delta invariant, asserted before
	// trading resumes.
	GateNetDeltaAsserted

	// GateAccountPosture is §4.1's account and margin-mode posture: margin
	// mode, spot-as-collateral, spot borrow, Portfolio/Unified margin, position
	// mode, the leverage pin. §7.5 makes every one of them refuse-to-trade
	// rather than a warning.
	//
	// Boot is where it is FIRST asserted, not where it is last asserted:
	// posture "is not assumed to persist from boot", and the pre-trade REST
	// re-verify (§6.3) is the backstop, because a position-mode change emits no
	// venue event to hang an invalidation on.
	GateAccountPosture

	// GateDeactivationsResolved is §7.5's journaled DeactivationPending: a
	// close whose EmergencyPlanStore.Deactivate never confirmed, re-driven to
	// acknowledgement before its hedge-group lease is released, so a closed
	// trade never leaves an active plan behind.
	GateDeactivationsResolved

	// GateMarginOpsResolved is §7.5's in-flight MarginOp, resolved by its
	// evidence contract and never re-fired blind. An Ambiguous outcome is the
	// canonical OutcomeBlocksOpening: it blocks new openings and not the safety
	// actions that read live state.
	GateMarginOpsResolved

	// GateWriterAuthority is the bot holding the writer: no latched takeover
	// epoch standing against it.
	//
	// §7.5 does not list this one; §7.4 does, from the other end — "a bot that
	// recovers mid-takeover finds itself fenced and stays in reconcile-only
	// until an operator clears it". That is a boot-time refuse-to-trade
	// condition stated in the section about the actuator, and it is the gate
	// whose clearance rule differs from every other gate's. See Outcome.
	GateWriterAuthority
)

const maxGate = GateWriterAuthority

// Validate reports whether g names a precondition, excluding the sentinel.
func (g Gate) Validate() error {
	if g == GateUnspecified || g > maxGate {
		return fmt.Errorf("gate %d: %w", uint8(g), ErrGateInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (g Gate) String() string {
	switch g {
	case GateUnspecified:
		return "GateUnspecified"
	case GateJournalReplayed:
		return "GateJournalReplayed"
	case GateVenueJoined:
		return "GateVenueJoined"
	case GateNetDeltaAsserted:
		return "GateNetDeltaAsserted"
	case GateAccountPosture:
		return "GateAccountPosture"
	case GateDeactivationsResolved:
		return "GateDeactivationsResolved"
	case GateMarginOpsResolved:
		return "GateMarginOpsResolved"
	case GateWriterAuthority:
		return "GateWriterAuthority"
	}
	return fmt.Sprintf("Gate(%d)", uint8(g))
}

// Outcome is what a gate currently reports.
//
// [SPEC EDIT LANDED — v0.9.6] §7.5 gave every one of its preconditions the same
// verb — asserted before trading resumes — while stating, in the same
// paragraph, two conditions that do NOT clear the same way. A journaled MarginOp resolves
// itself through its evidence contract; a foreign order or an unexplained
// position delta "alert for operator clearance" and are "not auto-corrected".
// §7.4 then makes the second kind explicit for the fence: return to normal
// operation requires the branch's terminal outcome, "explicit operator
// clearance, and a fresh VerifyFence propagation test — never health-flap
// re-entry".
//
// So the spec has latching preconditions and self-clearing ones, and never says
// which is which or that the distinction exists. A checklist that treated them
// uniformly would let a gate that failed and then passed on its own retry
// re-admit trading — which is exactly the health-flap re-entry §7.4 forbids,
// arriving through §7.5's door. OutcomeNeedsOperator is that distinction made
// into a value: it is the one outcome a fresh report cannot lift, and Clear is
// the only thing that moves it.
type Outcome uint8

const (
	// OutcomeUnreported is the zero value: nobody has said anything about this
	// gate.
	//
	// It maps to PostureReconcileOnly rather than to PostureHalted, and that is
	// the correct default rather than the merely-safest one. A gate nobody has
	// reported yet is the ordinary state of a boot in progress, and
	// reconcile-only is precisely what a boot in progress is permitted to do —
	// so the zero checklist describes the process at the instant it starts,
	// with no caller having to say so.
	OutcomeUnreported Outcome = iota

	// OutcomePending is the gate's work under way. Reported explicitly by a
	// phase that has begun a long join, so an operator reading a stuck boot can
	// tell "started and still running" from "never started" — which
	// OutcomeUnreported cannot say. Same posture, different diagnosis.
	OutcomePending

	// OutcomeFailed is the gate's check having run and refused: a wrong margin
	// mode, a leverage pin that does not match, a journal that will not replay.
	// Halts, and is freely re-reportable — a posture check that failed because
	// the venue was unreachable may legitimately pass on the next attempt.
	OutcomeFailed

	// OutcomeNeedsOperator is the gate refusing in a way no retry may lift: a
	// foreign order, an unexplained position delta, a latched takeover epoch.
	//
	// Reconcile-only, latching. See the type doc.
	OutcomeNeedsOperator

	// OutcomeBlocksOpening is the gate refusing new exposure while leaving
	// safety execution available — §7.5's Ambiguous MarginOp, which blocks
	// openings "until the staged-float ledger is explicitly reconciled".
	OutcomeBlocksOpening

	// OutcomePassed is the gate satisfied. Constrains nothing.
	OutcomePassed
)

const maxOutcome = OutcomePassed

// Validate reports whether o names an outcome. The zero value is legal:
// "unreported" is a real state and the one a fresh checklist is in.
func (o Outcome) Validate() error {
	if o > maxOutcome {
		return fmt.Errorf("outcome %d: %w", uint8(o), ErrOutcomeInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (o Outcome) String() string {
	switch o {
	case OutcomeUnreported:
		return "OutcomeUnreported"
	case OutcomePending:
		return "OutcomePending"
	case OutcomeFailed:
		return "OutcomeFailed"
	case OutcomeNeedsOperator:
		return "OutcomeNeedsOperator"
	case OutcomeBlocksOpening:
		return "OutcomeBlocksOpening"
	case OutcomePassed:
		return "OutcomePassed"
	}
	return fmt.Sprintf("Outcome(%d)", uint8(o))
}

// posture is the ceiling this outcome alone imposes.
func (o Outcome) posture() Posture {
	switch o {
	case OutcomeUnreported, OutcomePending, OutcomeNeedsOperator:
		return PostureReconcileOnly
	case OutcomeBlocksOpening:
		return PostureStopOpening
	case OutcomePassed:
		return PostureTrading
	}
	// OutcomeFailed, and anything unrecognised. An unrecognised outcome halting
	// is deliberate: Validate rejects it at the door, and a value that reached
	// here anyway is a corrupted enum rather than a new case to guess at.
	return PostureHalted
}

// Checklist holds §7.5's preconditions and the outcome most recently reported
// for each.
//
// Not safe for concurrent use. It is written by the boot sequence and read by
// the risk gate, which in this system are the same goroutine — §3.4's command
// sequencer serialises them — and a mutex here would suggest otherwise.
type Checklist struct {
	required []Gate
	outcomes map[Gate]Outcome
}

// NewChecklist declares the gates that must be satisfied before trading
// resumes, and returns a checklist in which every one of them is Unreported.
func NewChecklist(required ...Gate) (*Checklist, error) {
	if len(required) == 0 {
		return nil, ErrChecklistEmpty
	}
	outcomes := make(map[Gate]Outcome, len(required))
	for _, g := range required {
		if err := g.Validate(); err != nil {
			return nil, err
		}
		if _, dup := outcomes[g]; dup {
			return nil, fmt.Errorf("gate %s: %w", g, ErrGateDuplicate)
		}
		outcomes[g] = OutcomeUnreported
	}
	return &Checklist{required: append([]Gate(nil), required...), outcomes: outcomes}, nil
}

// Required returns the declared gates, in declaration order. The slice is a
// copy: a caller that sorted the returned slice in place would reorder the
// checklist's own record of what it requires.
func (c *Checklist) Required() []Gate {
	return append([]Gate(nil), c.required...)
}

// Report files an outcome for a gate.
//
// It refuses a gate this checklist never required, and refuses to move a gate
// out of OutcomeNeedsOperator — that transition belongs to Clear alone.
func (c *Checklist) Report(g Gate, o Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	current, declared := c.outcomes[g]
	if !declared {
		return fmt.Errorf("gate %s: %w", g, ErrGateUndeclared)
	}
	if current == OutcomeNeedsOperator && o != OutcomeNeedsOperator {
		return fmt.Errorf("gate %s: %w", g, ErrGateLatched)
	}
	c.outcomes[g] = o
	return nil
}

// Outcome returns the outcome last reported for g, and whether g is required at
// all. The second result matters: an undeclared gate would otherwise read as
// OutcomeUnreported, which is a real state of a real gate.
func (c *Checklist) Outcome(g Gate) (Outcome, bool) {
	o, declared := c.outcomes[g]
	return o, declared
}

// Clear lifts an operator hold, and returns the gate to OutcomeUnreported.
//
// Not to OutcomePassed, deliberately. §7.4 requires clearance AND "a fresh
// VerifyFence propagation test" before normal operation returns, so clearance
// restores the gate's ability to pass rather than the pass itself — the check
// has to run again and say so. Clearing to Passed would make an operator's
// acknowledgement of an alert into an assertion about venue state they never
// made.
func (c *Checklist) Clear(g Gate) error {
	current, declared := c.outcomes[g]
	if !declared {
		return fmt.Errorf("gate %s: %w", g, ErrGateUndeclared)
	}
	if current != OutcomeNeedsOperator {
		return fmt.Errorf("gate %s: %w", g, ErrGateNotLatched)
	}
	c.outcomes[g] = OutcomeUnreported
	return nil
}

// Posture is the weakest posture consistent with every required gate.
//
// The reduction is over the REQUIRED set rather than over the reported one,
// which is the whole default-deny mechanism: a gate that was declared and never
// reported contributes its Unreported ceiling, so forgetting to report cannot
// produce a more permissive answer than reporting a failure would.
func (c *Checklist) Posture() Posture {
	postures := make([]Posture, 0, len(c.required))
	for _, g := range c.required {
		postures = append(postures, c.outcomes[g].posture())
	}
	return Weakest(postures...)
}

// Blocking returns the required gates whose outcome is holding the posture
// below PostureTrading, in Gate order.
//
// Sorted by gate rather than left in declaration order so the answer does not
// depend on how the caller happened to build the checklist; an alert that names
// a different first cause on two identical boots is one an operator learns to
// distrust. Empty exactly when Posture is PostureTrading.
func (c *Checklist) Blocking() []Gate {
	var blocking []Gate
	for g, o := range c.outcomes {
		if o.posture() < PostureTrading {
			blocking = append(blocking, g)
		}
	}
	sort.Slice(blocking, func(i, j int) bool { return blocking[i] < blocking[j] })
	return blocking
}
