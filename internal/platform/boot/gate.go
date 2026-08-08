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

	// GateJournalDurable is §11/§13's storage precondition on the write-ahead
	// journal: SQLite's documented requirement that the database live on a LOCAL
	// filesystem, never NFS or a network FS.
	//
	// [SPEC EDIT LANDED — v0.9.39] §11 already scheduled this check "to §7.5's
	// boot posture list rather than to the adapter's constructor" and §7.5's gate
	// roster did not have it. It is a gate rather than a condition folded into
	// GateJournalReplayed for §7.5's own rule that a gate is reported about its
	// OWN evidence: the replay's evidence is the records, and this one's is the
	// filesystem under them. The merge is not merely untidy — the storage check
	// runs FIRST and its refusal LATCHES, and the replay's own first report is
	// OutcomePending, which Report refuses against a latched gate. One gate would
	// turn a correct sequencing into an error from the phase that must not
	// produce one.
	//
	// What it protects is the claim the tamper-evidence cannot make for itself.
	// §7.5 states the limit in as many words — a chain "proves nothing about
	// records that were never seen", so truncation at the tail is undetectable —
	// and a network filesystem is precisely the condition under which records are
	// lost with every remaining link still verifying. So a journal on NFS is not a
	// storage-hygiene matter: it is a journal whose verification means less than
	// it says, which is why its refusal is OutcomeEvidenceUnusable and not an
	// ordinary failure.
	GateJournalDurable

	// GateJournalReplayed is state rebuilt from the write-ahead journal —
	// §7.5's first half, before any venue call.
	GateJournalReplayed

	// GateVenueJoined is the journal joined to venue truth: OrderByClientID,
	// Fills, Positions, FundingLedger. Known journaled client IDs are
	// auto-recovered; foreign orders and unexplained position deltas are not,
	// and they are the canonical OutcomeNeedsOperator (§7.5: they "alert for
	// operator clearance", they "are not auto-corrected").
	GateVenueJoined

	// GateGenerationsReinstated is §7.5's rebuild at GENERATION scope: every
	// `PackageDelever` generation the journal opened and never ended, back in
	// SM §7's ladder with its entry back in §6.7's shared reserved-reduction
	// ledger, and every ladder a supersession closed still closed.
	//
	// [SPEC EDIT LANDED — v0.9.57] §7.5's roster did not have it, and it is a
	// gate rather than a condition folded into GateJournalReplayed for this
	// section's own rule that a gate is reported about its OWN evidence.
	// GateJournalDurable made that argument about the filesystem beneath the
	// records; this one makes it about a body of evidence the replay does not
	// have YET. The entry a generation must re-Reserve carries its confirmed perp
	// reduction, which is in no journal — there is no fill record at either scope
	// — so it can only be read from the venue, after the join and after §5.3's
	// absence resolution. Two gates because there are two phases, and the second
	// cannot run where the first does.
	//
	// The direction of its failure is the one GateJournalReplayed reported as
	// OutcomeIncomplete until this gate existed: a ledger missing an entry reads
	// as MORE room, so §6.7's determined branch sizes a compensating trade from a
	// residual too large by exactly the generation's outstanding net intent —
	// FM-5's shape through recovery, invisible to every invariant computed FROM
	// that ledger.
	GateGenerationsReinstated

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
	case GateJournalDurable:
		return "GateJournalDurable"
	case GateJournalReplayed:
		return "GateJournalReplayed"
	case GateVenueJoined:
		return "GateVenueJoined"
	case GateGenerationsReinstated:
		return "GateGenerationsReinstated"
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
// into a value: a fresh report cannot lift it, and Clear is the only thing that
// moves it.
//
// # There are TWO axes here, not one, and reading them as one loses a case
//
// How restrictive an outcome is (its posture ceiling) and whether a re-run of
// the same check may lift it (whether it latches) are independent properties,
// and this enum's members are points in that product rather than rungs of a
// ladder. A reader who treats the members as ordered by severity will reach for
// OutcomeFailed where a latching one belongs, and get a refusal any retry can
// lift.
//
// The product has six cells over the three constraining postures, and it took
// three slices to fill them — because the first gap was closed without
// tabulating at all, and the tabulation that closed the second was drawn on TWO
// axes when the type decides THREE:
//
//	                  freely re-reportable   latching
//	halted            OutcomeFailed          OutcomeEvidenceUnusable  (F1)
//	reconcile-only    OutcomeIncomplete      OutcomeNeedsOperator     (G1)
//	stop-opening      —                      OutcomeProjectionStale   (F2)
//	                                         OutcomeBlocksOpening     (B3)
//
// with OutcomeUnreported, OutcomePending and OutcomePassed outside it: the first
// two are PRE-verdict and the third constrains nothing.
//
// # The stop-opening row's left cell is EMPTY, and it was never occupied
//
// OutcomeBlocksOpening sat there for four slices with no producer at all, and
// when its one declared producer arrived (§7.5's Ambiguous MarginOp, task #212)
// it turned out to belong on the right — its own doc had been quoting the clause
// that makes it latch since the day it was written. So the cell is not vacated;
// it is revealed to have been empty, filled by a member that answered a
// different question from the row it was in.
//
// That is the third-axis lesson turned one notch further. F2's tabulation found
// a cell filled by PRE-VERDICT members and read it as occupied; this one was
// filled by a member with no producer, which is the same mistake with the
// property "does anything actually report this?" as the axis. A member is a
// claim about a condition, and until a condition reports it the claim is
// untested — so the table's cells are only as trustworthy as the producers
// standing in them.
//
// Nothing is invented to refill it. A stop-opening refusal that a re-run may
// legitimately lift is a coherent idea and no gate has one; the member for it
// lands with the condition, which is this repo's standing rule and is exactly
// what would have avoided the misplacement above.
//
// The right cell now holds TWO members, and that is checked rather than
// tolerated: they agree on every property this type decides (posture, latching,
// verdict-bearing) and differ only in CAUSE — a stale projection versus an
// unattributable margin movement — which is what the GATE names. See the
// observation in record §81 §1c: the two other latching members are likewise
// named for their first producer's cause, so the next second-producer to reach
// either has this same question to ask.
//
// Slice F1 found the halted row's empty cell, added the member its own producer
// needed, and stopped — leaving the stop-opening row in exactly the state the
// halted row had been in, one slice after the argument for looking had been
// written down. §6 step 7's rule is what fell in it: a send whose projection
// write failed "HOLDS NEW TRADING until that order's state is re-derived from
// the journal", which is stop-opening (safety execution must stay available —
// the order may be filling) and is not liftable by a re-run, because the check
// that would lift it recomputes from the projection that is known stale. That is
// this repo's mirror rule on the axis it keeps missing: a fix applied to one row
// of a table gets applied to the table.
//
// # The third axis, and why a full-looking table still had a hole
//
// F2's tabulation drew the reconcile-only row's re-reportable cell as
// "OutcomeUnreported/Pending" and read it as OCCUPIED. It was not. Both of those
// members mean the check has produced NO VERDICT — one that it never ran, one
// that it is running — and a cell filled by pre-verdict members is empty for
// every producer that has an answer. The property the table was missing is
// therefore the one every other cell has without saying so: whether the outcome
// REPORTS A RESULT. Read on that axis the halted and stop-opening rows each had
// a verdict-bearing re-reportable member and the reconcile-only row had none,
// which is visible the moment the axis is named and invisible while it is not.
//
// The general rule, one turn past the one F2 earned: a product is only as
// complete as the properties you tabulate it on, so when a cell is filled by a
// member that answers a DIFFERENT question from its row-mates, that is the tell
// that an axis is missing rather than that the cell is occupied.
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
	// mode, a leverage pin that does not match, a venue that would not answer.
	// Halts, and is freely re-reportable — a posture check that failed because
	// the venue was unreachable may legitimately pass on the next attempt.
	OutcomeFailed

	// OutcomeEvidenceUnusable is the gate refusing because the EVIDENCE it
	// reasons from cannot be trusted: a write-ahead journal whose hash chain does
	// not verify, or whose records do not compose into a state at all.
	//
	// [SPEC EDIT LANDED — v0.9.36] It halts like OutcomeFailed and latches like
	// OutcomeNeedsOperator, and until slice F1 neither of those existed together.
	// §7.5 separates its preconditions on ONE axis — self-clearing versus
	// latching — while the postures they impose vary on a SECOND, and the two are
	// independent. Three of the four combinations had a value and this one did
	// not, so the condition that wants both had to be reported as whichever half
	// mattered more, and each half is wrong in its own direction:
	//
	//   - As OutcomeFailed it is HALTED and freely re-reportable, so a re-run of
	//     the replay — over bytes that cannot change — could re-admit trading
	//     with nobody having looked. That is §7.4's "never health-flap re-entry",
	//     arriving through a check whose retry is deterministic.
	//   - As OutcomeNeedsOperator it latches, and the posture it imposes is
	//     PostureReconcileOnly — which PERMITS ActReconcile. That is the unsafe
	//     one, and the reason is a claim this package makes one type over: an
	//     ActReconcile is "exposure-neutral BY CONSTRUCTION". The construction is
	//     that it re-drives what already happened, and what it re-drives is read
	//     from the journal. §7.5's DeactivationPending re-drive fires
	//     EmergencyPlanStore.Deactivate for whichever plan the corrupt log appears
	//     to name, and that call is TERMINAL WITH NO REACTIVATION: after its ack
	//     no Replicate for the TradeID is accepted and ListActive never returns
	//     it. A plan tombstoned for a trade that is in fact still open leaves that
	//     position permanently without a takeover path — an actuator enumerating
	//     plans after host death finds none, and what the plan carried (client
	//     order ids, allocated quantities, recovery cursors) is exactly what it
	//     cannot rebuild from a dead journal — and it releases the hedge-group
	//     lease, so a successor trade may be admitted over live exposure.
	//     Exposure-neutrality is a property of the ACT given trustworthy inputs,
	//     and this is the one gate whose failure means the inputs are not.
	//
	// So it is a fourth member rather than a re-reading of a third. Reported by
	// the phase that verified the chain and found it broken; see
	// oms/application.Replayer, its first and so far only producer.
	OutcomeEvidenceUnusable

	// OutcomeIncomplete is the gate's check having RUN, concluded, and not
	// established its precondition — with nothing failed and no operator needed.
	// A further attempt may satisfy it.
	//
	// [SPEC EDIT LANDED — v0.9.38] It is the THIRD empty cell of the product this
	// type's doc tabulates, and the one the table's two axes hide: the
	// reconcile-only row's re-reportable side held only OutcomeUnreported and
	// OutcomePending, and BOTH of those are pre-verdict. Every other row has a
	// re-reportable member that means "the check ran and refused" — OutcomeFailed
	// on the halted row, OutcomeBlocksOpening on the stop-opening row. This row
	// had none, so a check that ran to completion and left work had to be reported
	// as one that had not run.
	//
	// Its producer is §7.5's own ordinary case. §5.3's stable-not-found rule
	// answers UNPROVED for an order dispatched shortly before the crash — the
	// propagation bound has not elapsed, so the venue's silence is not yet
	// evidence — and that is not a failure in any direction: the venue answered,
	// the log is intact, and the only thing wrong is that not enough time has
	// passed. Reporting it as OutcomeFailed halts a process whose venue is
	// healthy and forbids §7.5's own DeactivationPending re-drive; reporting it
	// as OutcomeNeedsOperator demands a human for a timer.
	//
	// # Why the collapse into OutcomePending is not merely a naming preference
	//
	// The two are indistinguishable in the checklist, and the distinction they
	// lose is the one an operator acts on: OutcomePending means WAIT, and this
	// means ACT — retry, or look at why the wait is not ending. A boot that
	// concluded and refuses, reported as a boot still running, leaves a process
	// that is alive, audible and below PostureStopOpening: it may not reduce
	// exposure, nobody escalates because the boot "has not finished", and §7.4's
	// watchdog reads liveness, so it does not escalate either. That is precisely
	// the alive-but-wedged state SignalImmediateHalt's doc calls an UNCOVERED
	// failure, reached this time through a vocabulary rather than through a
	// shutdown.
	//
	// Same posture as OutcomePending and the same clearance rule, deliberately:
	// what is added is a verdict, not a restriction. See oms/application.Reconciler.
	OutcomeIncomplete

	// OutcomeNeedsOperator is the gate refusing in a way no retry may lift: a
	// foreign order, an unexplained position delta, a latched takeover epoch.
	//
	// Reconcile-only, latching. See the type doc.
	OutcomeNeedsOperator

	// OutcomeBlocksOpening is the gate refusing new exposure while leaving
	// safety execution available — §7.5's Ambiguous MarginOp, which blocks
	// openings "until the staged-float ledger is explicitly reconciled".
	//
	// [SPEC EDIT LANDED — v0.9.62] It LATCHES, and until its first producer
	// landed it did not. The member was declared for exactly one condition, the
	// clause above was quoted in this comment from the day it was written, and it
	// was nonetheless placed in the freely re-reportable column — so a later
	// OutcomePassed on the same gate would have lifted a §9 accounting incident
	// with nobody having looked.
	//
	// The confusion is traceable and it is §7.5 sorting on the wrong SCOPE. That
	// section lists the self-clearing preconditions and names this gate among
	// them — "a journaled MarginOp resolves through its evidence contract […] a
	// fresh report lifts it" — which is true of the GATE and false of this
	// OUTCOME. An op that was merely in flight at the crash does resolve itself,
	// and that case is OutcomeIncomplete. The ambiguity is the exception §7.1
	// states one section earlier, and latching is a property of what was
	// reported, not of which gate reported it — which is the distinction this
	// type's doc exists to make, arriving at the member that was placed before
	// anything could test it.
	//
	// Three independent reasons, and only the third is about probability:
	//
	//   - both normative sentences say so. §7.1: on Ambiguous the staged-float
	//     ledger is "reconciled by explicit accounting / operator action, a §9
	//     case". §7.5: openings are blocked "UNTIL" that reconciliation. Neither
	//     describes a check that re-runs.
	//   - the checklist holds one outcome per GATE, not one per op. A process
	//     that resolves several margin ops reports this gate several times, so
	//     without the latch the next Passed clears the previous ambiguity — and
	//     the live top-up path that reports one at a time is the caller that
	//     makes this certain rather than possible.
	//   - re-reading the evidence is not more informative, it is more confounded.
	//     The evidence is a delta against a journaled before-state, so every
	//     later reading includes every later mutation of the position. A freely
	//     re-reportable outcome invites exactly the retry that cannot help.
	//
	// What lifts it is Clear, and across processes the durable verdict is what
	// stops a restart laundering it — see oms/application.MarginOpResolver, its
	// first producer, and ports.MarginOpJournal for why the verdict is stored
	// rather than re-derived.
	//
	// [SPEC EDIT LANDED — v0.9.63] The ACT that ends the incident exists, and it
	// had to: this outcome and OutcomeProjectionStale latch for the same reason
	// and are completable for opposite ones. The stale projection is lifted by a
	// re-derivation a fresh process performs anyway (the reconcile-only boot), so
	// its clearance came free. This one's durable verdict is stored precisely so
	// that no re-derivation lifts it — which leaves no exit at all unless
	// something outside the boot can record the reconciliation. That is
	// oms.ReconcileMarginOp, behind `arblab margin reconcile`, writing core §7.3's
	// per-actor audit into §5.1's chain (task #213, record §82).
	//
	// The obligation generalises, and it is why this note is on the OUTCOME
	// rather than on the gate: an outcome that latches across processes is only
	// completable if some act can produce the evidence that clears it, and that
	// act must land WITH OR BEFORE the first thing that can produce the outcome.
	// Here the producer (task #212) came first by one slice, which was survivable
	// only because nothing writes a MarginOp yet.
	OutcomeBlocksOpening

	// OutcomeProjectionStale is the gate refusing new exposure because the
	// CURRENT-STATE PROJECTION is known to disagree with the venue: §6 step 7's
	// dispatch that reached the venue and whose apply failed.
	//
	// [SPEC EDIT LANDED — v0.9.37] It blocks openings like OutcomeBlocksOpening
	// and latches like OutcomeNeedsOperator, and it is the second empty cell of
	// the product this type's doc tabulates. Each half of it is load-bearing in a
	// different direction:
	//
	//   - Not halted. The order may be executing right now, so the safety verbs
	//     must stay available; halting a process whose venue is filling an order
	//     it cannot see is worse than the stale reading. §7.4's mode (a) is the
	//     posture for exactly this — withhold new exposure, keep the ladder.
	//   - Latching. The check that would lift it recomputes the net delta FROM
	//     the projection, and the projection is the thing that is wrong: the
	//     order sits at StatePlanned, which is the one non-terminal state
	//     contributing NO possible delta (domain.ContributesPossibleDelta), so
	//     the group reads determined and flat. A freely re-reportable outcome
	//     would therefore be lifted by an invariant agreeing with its own
	//     corrupted input, with nobody having looked — §46 §8e's rule arriving at
	//     the boot checklist.
	//
	// What lifts it is the RE-DERIVATION §6 step 7 names: replaying the journal,
	// which is what the process the log outlives does at boot. Inside a running
	// process the only lift is Clear, and that is the correct asymmetry — a
	// re-derivation is a fresh process with a fresh checklist, so the latch does
	// not have to model it.
	OutcomeProjectionStale

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
	case OutcomeEvidenceUnusable:
		return "OutcomeEvidenceUnusable"
	case OutcomeIncomplete:
		return "OutcomeIncomplete"
	case OutcomeNeedsOperator:
		return "OutcomeNeedsOperator"
	case OutcomeBlocksOpening:
		return "OutcomeBlocksOpening"
	case OutcomeProjectionStale:
		return "OutcomeProjectionStale"
	case OutcomePassed:
		return "OutcomePassed"
	}
	return fmt.Sprintf("Outcome(%d)", uint8(o))
}

// posture is the ceiling this outcome alone imposes.
func (o Outcome) posture() Posture {
	switch o {
	case OutcomeUnreported, OutcomePending, OutcomeIncomplete, OutcomeNeedsOperator:
		return PostureReconcileOnly
	case OutcomeBlocksOpening, OutcomeProjectionStale:
		return PostureStopOpening
	case OutcomePassed:
		return PostureTrading
	case OutcomeFailed, OutcomeEvidenceUnusable:
		return PostureHalted
	}
	// Anything unrecognised. An unrecognised outcome halting is deliberate:
	// Validate rejects it at the door, and a value that reached here anyway is
	// a corrupted enum rather than a new case to guess at. OutcomeFailed used
	// to share this line and is now named above (§3.6 rule 10) — a declared
	// member and a corrupted one are different facts, and only one of them
	// should change when a member is added.
	return PostureHalted
}

// latching reports whether a fresh report of the same gate may lift this
// outcome.
//
// It is a method on the outcome rather than a set the Checklist holds, because
// the property belongs to what was reported and not to which gate reported it:
// the same gate can refuse in a way a retry may lift (a venue that did not
// answer) and in a way it may not (a chain that does not verify).
//
// It names every member rather than testing the two that latch, so that
// exhaustiveswitch forces a member added later to CHOOSE. The shorter form —
// `o == OutcomeNeedsOperator || o == OutcomeEvidenceUnusable` — compiles
// forever and answers false for anything new, and false is the direction that
// costs something: a refusal meant to hold until an operator looked at it would
// be lifted by the next report of the same check.
func (o Outcome) latching() bool {
	switch o {
	case OutcomeNeedsOperator, OutcomeEvidenceUnusable, OutcomeProjectionStale, OutcomeBlocksOpening:
		return true
	case OutcomeUnreported, OutcomePending, OutcomeIncomplete, OutcomeFailed, OutcomePassed:
		return false
	}
	// An unrecognised outcome, on posture's argument: Validate refuses it at the
	// door, and a value that arrived here anyway is a corrupted enum. Latching is
	// the safe reading of one — it holds the gate until a human looks.
	return true
}

// verdict reports whether this outcome carries a RESULT: the check ran and has
// an answer, as opposed to nobody having reported (Unreported) or the work
// being under way (Pending).
//
// The property is the type doc's third axis, given a predicate. It names every
// member on latching's argument: a member added later must choose, and the
// trailing return treats a corrupted value as verdict-bearing — posture()
// already halts on the same value, and a corrupted enum that constrained
// nothing would be the one reading of it that costs something.
func (o Outcome) verdict() bool {
	switch o {
	case OutcomeUnreported, OutcomePending:
		return false
	case OutcomeFailed, OutcomeEvidenceUnusable, OutcomeIncomplete,
		OutcomeNeedsOperator, OutcomeBlocksOpening, OutcomeProjectionStale, OutcomePassed:
		return true
	}
	return true
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
// out of a LATCHING outcome — that transition belongs to Clear alone.
//
// A latched gate admits exactly two reports, and both leave it latched: the
// same outcome again, and a latching outcome that is STRICTLY MORE
// restrictive. The repeat is admitted because it is honest — the phase that
// found a chain broken finds it broken again on every attempt, and turning that
// into an error would make every producer special-case its own second call. The
// escalation is admitted because the alternative is a gate pinned at the weaker
// of two refusals by the accident of which one was discovered first.
//
// What is refused is any report that would WEAKEN the hold, and the
// non-latching ones are the reason the check cannot be a posture comparison
// alone: OutcomeFailed is more restrictive than OutcomeNeedsOperator and does
// not latch, so admitting it would launder a latched gate into one a later
// OutcomePassed could lift — clearance by two reports, with no operator in
// either.
func (c *Checklist) Report(g Gate, o Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	current, declared := c.outcomes[g]
	if !declared {
		return fmt.Errorf("gate %s: %w", g, ErrGateUndeclared)
	}
	// The two admitted cases, spelled as the doc above spells them: the SAME
	// outcome again, and a latching outcome that is STRICTLY more restrictive.
	//
	// The comparison used to be `o.posture() <= current.posture()`, which said
	// the same thing only by an accident that has now expired: the three latching
	// members had three DISTINCT postures, so equality could only hold when o and
	// current were the same member. OutcomeBlocksOpening joining
	// OutcomeProjectionStale on the stop-opening row is the second instance that
	// accident was scoped by, and under `<=` it would let one latched cause be
	// overwritten by the other — no weaker hold, but the diagnosis an operator
	// reads replaced by whichever refusal was discovered last. That is
	// domain.Generation.Supersede's first-cause-wins rule, and the reason is the
	// same: the cause shown must be the one that closed the gate.
	repeat := o == current
	escalation := o.latching() && o.posture() < current.posture()
	if current.latching() && !repeat && !escalation {
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

// Clear lifts an operator hold on a LATCHED gate, and returns it to
// OutcomeUnreported.
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
	if !current.latching() {
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

// PermitsNow reports whether the checks that have CONCLUDED permit act a — the
// running-process read, and deliberately not Posture().Permits.
//
// # Why a second fold exists, and why its default is the opposite of Posture's
//
// Posture reduces over the REQUIRED roster, so a gate nobody reported holds the
// answer down — default-deny, which is right at boot, where "unknown" must mean
// "not yet". Read the same way in a RUNNING process it answers reconcile-only
// forever on any deployment whose roster declares gates without producers,
// which is every deployment §12's sequencing produces — G2 found this wiring
// the first consumer and settled it for the exit code (see the reconcile root's
// ExitClass); this method is the same produced-versus-structural split for a
// write path that must ask per act rather than per run.
//
// So the fold here is over VERDICTS: the gates whose outcome reports a result
// (the type doc's third axis). An Unreported or Pending gate says nothing and
// constrains nothing; every concluded one imposes its ceiling, latched or not —
// a non-latching OutcomeFailed halts new trading exactly as hard as a latch
// does, it merely lifts differently.
//
// # Fail-closed on an empty verdict set
//
// A fold whose members are only the gates with answers has PostureTrading for
// its identity, and a fresh checklist would therefore answer "everything is
// permitted" — the precise "nothing was checked and everything passed" collapse
// Weakest's identity exists to refuse. So no verdict at all refuses every act:
// a process whose checklist holds no concluded check is a process §7.5's boot
// has not run in, and nothing is what it is permitted to do. The two readings
// of "no information" are opposite here and at Posture, and both are the safe
// direction for their own question.
func (c *Checklist) PermitsNow(a Act) bool {
	verdicts := 0
	weakest := PostureTrading
	for _, g := range c.required {
		o := c.outcomes[g]
		if !o.verdict() {
			continue
		}
		verdicts++
		if p := o.posture(); p < weakest {
			weakest = p
		}
	}
	if verdicts == 0 {
		return false
	}
	return weakest.Permits(a)
}

// Latched returns the required gates whose outcome only an operator can lift, in
// Gate order.
//
// # It exists because the question had no read-only answer, and Clear is not one
//
// latching is unexported and belongs to the outcome rather than to the gate, so
// until this method the only way to discover whether a gate was latched was to
// call Clear and see whether it succeeded — which is to say, by performing the
// operator clearance that answers it. §7.5's whole point is that a latched
// precondition waits for a human, and the tool that shows a human what is waiting
// could not ask.
//
// It is a STRICT SUBSET of Blocking and reported beside it, never instead of it,
// because the two answer different questions and a boot has both at once. Blocking
// says what is holding the posture down — which on any real deployment includes
// gates no code reports yet, so it is dominated by structure rather than by
// findings. Latched says what will still be there after a restart with everything
// healthy. An operator reading only the first learns that the process may not
// trade; only the second says whether anything happened.
func (c *Checklist) Latched() []Gate {
	var latched []Gate
	for g, o := range c.outcomes {
		if o.latching() {
			latched = append(latched, g)
		}
	}
	sort.Slice(latched, func(i, j int) bool { return latched[i] < latched[j] })
	return latched
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
