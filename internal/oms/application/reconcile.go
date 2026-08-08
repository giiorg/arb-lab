package application

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Reconciler is build-spec §7.5's reconcile-only boot as one call: the journal
// replayed, the venue joined, every absence run through §5.3's rule, and the
// net-delta invariant asserted — each reported into the boot.Checklist a
// composition root owns.
//
// # It exists because §7.5's phases are only correct in one ORDER, and nothing
// # else could hold that
//
// Slices F1 and F2 built the three phases as three use cases and left them
// unwired, each recording the boot sequence as the next slice's work. What that
// deferral hid is that the phases are not independent: §5.3's absence resolution
// runs BETWEEN the venue join and the net-delta assertion, and the fused
// Join-that-also-asserted put it after both. See ORDER below — it is this
// slice's finding, and it was invisible from inside either use case.
//
// # What it is NOT: the checklist's owner
//
// The Checklist is the composition root's, and this takes a pointer to it rather
// than building one. §7.5's roster spans contexts — the §4.1 account posture is
// risk's and engine/venue's, the takeover epoch is §7.4's — so a checklist built
// here would either declare gates this context cannot report (and hold the
// posture at reconcile-only for ever with no way for anyone else to clear them)
// or declare only the three it can, which is the more dangerous shape: a boot
// that checked three of eight preconditions and reported PostureTrading. §7.5
// argues that exact failure from the other side, about the fold's seed —
// "nothing was checked" and "everything passed" must not be the same answer —
// and a short roster produces it through the roster instead.
//
// # Not safe for concurrent use
//
// Dispatcher's, Resolver's and Joiner's argument, unchanged; and the Checklist's
// own doc makes the same claim about the sequencer that writes it.
type Reconciler struct {
	replay    *Replayer
	join      *Joiner
	resolve   *Resolver
	reinstate *Reinstater
	marginOps *MarginOpResolver
	look      SecondLook
}

// SecondLook is the spot delta's stability bracket (task #179, record §70c):
// before a non-empty SpotUnexplained may LATCH, the boot waits Settle,
// re-runs the join (idempotent by the fill ledger's dedupe), and re-asserts —
// only a delta identical across both looks reaches an operator, because the
// one-look version freezes a late-reporting fill into a false incident,
// §7.5's own "human in the loop of something no human can decide".
//
// Wait is an injected seam rather than a clock read: the application layer
// owns no ambient time (§3.6 rule on notimenow), so the root supplies the
// blocking wait — time.Sleep in a binary, a recording fake in a test.
type SecondLook struct {
	Wait   func(time.Duration)
	Settle time.Duration
}

// wired reports whether the bracket was configured at all.
func (l SecondLook) wired() bool { return l.Wait != nil }

// validate refuses the half-configured value: a Wait with no positive Settle
// waits for nothing, and a Settle with no Wait cannot wait at all — either
// half alone reads as configured while the bracket silently never runs.
func (l SecondLook) validate() error {
	if l.Wait == nil && l.Settle == 0 {
		return nil
	}
	if l.Wait == nil || l.Settle <= 0 {
		return ErrSecondLookHalfWired
	}
	return nil
}

// NewReconciler constructs the use case, refusing a missing phase.
//
// All three phases are required and none is defaulted to a no-op. A Reconciler
// that could run with a nil Resolver would report the venue-joined gate on a
// boot where no absence was ever put to §5.3's rule, and an absence nobody
// resolved is not distinguishable in the report from one that resolved
// cleanly.
//
// The second look MAY be left unwired — a derivatives-only deployment has no
// spot delta to bracket — but a boot that then MEETS a spot delta fails by
// name rather than latching on one look: see Reconcile.
//
// The REINSTATEMENT may likewise be left unwired, on exactly that pattern and
// for exactly its reason. A deployment whose escalation ladder never runs has no
// generation to rebuild, and requiring the phase would make every such boot
// declare a collaborator it has no use for. What it must not do is PASS: a boot
// that meets a live generation with no reinstatement wired fails by name
// (ErrGenerationRestoreMissing), because reporting the gate satisfied would
// report §7.5's ledger rebuilt over a ledger nothing touched — which is the
// FM-5-shaped hole the gate exists to hold.
//
// The core §7.1 MARGIN-OP resolver is optional on the same pattern and for a
// reason one degree sharper: a deployment that has never topped up a position has
// no margin op to resolve, and until slice B3 part 2 lands NOTHING writes one, so
// requiring it would make every binary in this repo declare a collaborator with
// no possible work. A boot that MEETS one with no resolver wired fails by name
// (ErrMarginOpResolveMissing) — the alternative is a process that boots clean
// over a journaled write that may have moved real money.
func NewReconciler(replay *Replayer, join *Joiner, resolve *Resolver, reinstate *Reinstater,
	marginOps *MarginOpResolver, look SecondLook) (*Reconciler, error) {

	if replay == nil {
		return nil, ErrReplayerMissing
	}
	if join == nil {
		return nil, ErrJoinerMissing
	}
	if resolve == nil {
		return nil, ErrResolveUseCaseMissing
	}
	if err := look.validate(); err != nil {
		return nil, err
	}
	return &Reconciler{
		replay: replay, join: join, resolve: resolve,
		reinstate: reinstate, marginOps: marginOps, look: look,
	}, nil
}

// UnprovedOrder is one order §5.3's rule could not conclude about, and the gate
// that refused.
type UnprovedOrder struct {
	Order  ids.ClientOrderID
	Reason UnprovedReason
}

// Reconciliation is what one reconcile-only boot found.
//
// The verdicts are in the Checklist; this is the diagnosis. An operator reading
// a boot that will not trade needs both, and they are separate values because
// they answer to different rules: the checklist may only be moved through
// Report and Clear, and this is a plain record of what happened.
type Reconciliation struct {
	// Recovery is the journal replay's report, zero when the replay failed.
	Recovery Recovery

	// Join is the venue join's report, and Assertion is the net-delta
	// invariant's. Assertion is the zero value when the assertion did not run —
	// which is a real and common outcome, not an error. See ORDER.
	Join      Join
	Assertion Assertion

	// Reinstatement is the generation rebuild's report, zero when the phase did
	// not run. It is separate from Recovery because the two answer about
	// different evidence: Recovery says what the LOG held, and this says what was
	// put back — which for a generation whose premise dissolved during the outage
	// are different things, since the reassessment may have superseded it.
	Reinstatement Reinstatement

	// MarginOps is core §7.1's margin-op phase report, zero when the phase found
	// nothing — which is every real boot until slice B3 part 2 gives the record a
	// writer.
	MarginOps MarginResolution

	// Resolved is how many of Join.Absent §5.3's rule concluded about, and
	// Unproved names the ones it could not, with the gate that refused.
	Resolved int
	Unproved []UnprovedOrder

	// Stale names the orders whose resolved state was journaled and whose
	// projection write failed — §6 step 7's condition, reached through recovery
	// rather than through dispatch. They are why GateNetDeltaAsserted can be
	// reported OutcomeProjectionStale on a boot where the assertion never ran:
	// the gate is about the projection, and the projection is known wrong.
	Stale []ids.ClientOrderID

	// Cause is why a gate refused, when the refusal came from a failure rather
	// than from a finding: the replay error, the venue error, the repository
	// error.
	//
	// # It is a FIELD and not this method's error result, deliberately
	//
	// Reconcile's error result means "the report could not be filed" and nothing
	// else — a nil checklist, a gate the root did not declare. Every refusal a
	// boot can legitimately reach is a verdict, and returning one as an error
	// would put it one `if err != nil { return }` away from a caller that
	// discards it and boots anyway. It would also be actively wrong at the call
	// site this is written for: boot.Run turns a phase error into an unwind and a
	// dead process, and a journal that will not verify must leave a process that
	// is ALIVE, halted, latched and audible to §7.4's watchdog — because exiting
	// presents to that watchdog exactly as a dead host and escalates to an
	// actuator takeover.
	Cause error
}

// Reconcile runs §7.5's phases in order and reports each into c.
//
// # ORDER: the absence resolution runs BETWEEN the join and the assertion
//
// This is the sequence, and the middle step is the one no single use case could
// have placed:
//
//  1. Replay the journal. State, not quantity — no fill is journaled.
//  2. Join the venue: rebuild each dispatched order's executed quantity. An
//     order the venue's ORDER surface does not hold is not swept — §5.3 forbids
//     reading an absence as "no executions" — so it is handed back in
//     Join.Absent with the zero cumulative the replay gave it.
//  3. Run §5.3's rule on every absence. That is what sweeps those orders, and it
//     is where their executions reach the ledger.
//  4. Assert the net-delta invariant, over the projection as it now stands.
//
// Asserting at step 2 — which is what the fused call did — asserts over a ledger
// that is knowably short by exactly the orders step 3 is about to fill in, and
// the direction is the ordinary one rather than an edge case: §5.3 records that
// an unfilled terminal order is purged after 3 days on one venue and 24 hours on
// the other, and §6 step 6's marketable IOCs make a terminal order this system's
// commonest outcome. So our expected exposure reads LOW, the venue's standing
// position reads as an excess no order explains, and the invariant reports an
// unexplained delta — which is OutcomeNeedsOperator, which LATCHES. A healthy
// account, a healthy venue and an intact log produce an operator-clearance hold
// on the ordinary boot. The error direction is the safe one and the failure mode
// is not: a check that cries wolf on the common case is one an operator learns
// to clear without reading, which is the same halts-itself-with-every-light-green
// failure §7.5's own Unasserted ruling exists to avoid, arriving through noise
// instead of through a permanent block.
//
// # A gate is only ever reported about its OWN evidence
//
// Two conditions can hold at once with no ordering between them — a truncated
// sweep is reconcile-only and lifts on a retry, a failed projection write is
// stop-opening and latches — and one gate can carry only one outcome, so
// reporting both on the same gate would have to drop one. Whichever is dropped
// is wrong: reporting the more restrictive loses the latch, and reporting the
// latching one understates the posture. They are reported on different gates
// because they are about different evidence — the venue rebuild against
// GateVenueJoined, the projection against GateNetDeltaAsserted — and the
// checklist's fold then does the combining, which is what it is for.
func (r *Reconciler) Reconcile(c *boot.Checklist) (Reconciliation, error) {
	var out Reconciliation
	if c == nil {
		return out, ErrChecklistMissing
	}

	if err := c.Report(boot.GateJournalReplayed, boot.OutcomePending); err != nil {
		return out, err
	}
	rec, err := r.replay.Rebuild()
	if err != nil {
		out.Cause = err
		// The two failures the sentinel exists to separate. A chain that does not
		// verify is halted AND latched, because the reconciliation writes §7.5
		// would otherwise re-drive are computed from this very log.
		outcome := boot.OutcomeFailed
		if ChainBroken(err) {
			outcome = boot.OutcomeEvidenceUnusable
		}
		return out, c.Report(boot.GateJournalReplayed, outcome)
	}
	out.Recovery = rec

	// A live generation no longer holds THIS gate, and the move is a gate
	// split rather than a weakening.
	//
	// Slice #203 reported OutcomeIncomplete here whenever the log held a
	// generation it never ended, because nothing re-Reserved its ledger entry and
	// a ledger missing an entry reads as MORE room. Nothing about that direction
	// has changed; what changed is that the reinstatement exists and CANNOT run
	// here — the entry carries the confirmed perp reduction, which lives in no
	// journal and is read from the venue two phases down. So the refusal moved to
	// the gate whose evidence it actually is (boot.GateGenerationsReinstated),
	// and the posture is held identically in the meantime because the roster's
	// fold is over the REQUIRED gates: an unreported one refuses exactly as a
	// failed one does.
	//
	// The replay's own evidence is the records, and it rebuilt everything the
	// records can rebuild.
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomePassed); err != nil {
		return out, err
	}

	// §7.1's in-flight MarginOp, resolved BEFORE the join and independently of it.
	//
	// # It runs early, and the placement is an argument rather than an ordering
	// # convenience
	//
	// Every other phase below waits on something: the reinstatement needs the
	// venue's executed quantities, the assertion needs the projection the join
	// filled in. This one needs the log and one per-instrument margin read, and
	// neither of those is what the join produces. Running it here means a §9
	// accounting incident is REPORTED on a boot whose join never concluded —
	// which is the boot most likely to be happening during the venue trouble that
	// left the op in flight in the first place.
	//
	// Its evidence being independent is also why its failure does not stop the
	// sequence: the gate carries the verdict and the picture of the account
	// behind it still gets built.
	if err := r.resolveMarginOps(rec, c, &out); err != nil {
		return out, err
	}

	if err := c.Report(boot.GateVenueJoined, boot.OutcomePending); err != nil {
		return out, err
	}
	joined, err := r.join.Join(rec.Dispatched)
	out.Join = joined
	if err != nil {
		out.Cause = err
		return out, c.Report(boot.GateVenueJoined, boot.OutcomeFailed)
	}

	if err := r.resolveAbsences(joined.Absent, &out); err != nil {
		out.Cause = err
		return out, c.Report(boot.GateVenueJoined, boot.OutcomeFailed)
	}

	joinOutcome := r.joinOutcome(out)
	if err := c.Report(boot.GateVenueJoined, joinOutcome); err != nil {
		return out, err
	}

	// A failed apply is a verdict about the projection, and the assertion is the
	// check that computes FROM the projection — so reporting it also stops the
	// assertion from running. That is not an optimisation: §6 step 7's rule is
	// that the stale reading is the PERMISSIVE one, so an assertion run here
	// would be an invariant agreeing with its own corrupted input, and a pass on
	// this gate is the one thing that must not be reachable while it holds.
	if len(out.Stale) > 0 {
		return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeProjectionStale)
	}

	if joinOutcome != boot.OutcomePassed {
		// Neither the assertion nor the generation rebuild runs over a join that
		// did not conclude, and both gates stay UNREPORTED rather than being
		// failed. Unreported is the honest value — the check never ran — and it is
		// not the permissive reading: the fold is over the REQUIRED gates, so an
		// unreported one holds the posture at reconcile-only exactly as a failure
		// would.
		//
		// The reinstatement is held for a sharper reason than the assertion is. It
		// sizes each generation's ledger entry from its perp leg's executed
		// quantity, and an unconcluded join is precisely the state in which that
		// quantity is knowably SHORT — so a rebuild here would re-enter entries
		// that understate what the generations committed, and understating the
		// subtrahend is the direction that reads as more room to trade.
		return out, nil
	}

	if err := r.reinstateGenerations(rec, c, &out); err != nil {
		return out, err
	}

	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomePending); err != nil {
		return out, err
	}
	assertion, err := r.join.Assert(rec.Dispatched)
	out.Assertion = assertion
	if err != nil {
		out.Cause = err
		return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeFailed)
	}
	// The precedence is the safety claim (record §70c): the latching findings
	// are checked BEFORE the unconcluded ones, so a boot carrying both a spot
	// delta and an unpinned balance latches — the incomplete finding must
	// never mask the one that needs an operator.
	if assertion.Holds() {
		// A holding verdict whose ONLY holding finding is a spot delta gets
		// the second look before it may latch; anything else — a perp delta,
		// a foreign symbol, an uncovered instrument — latches on its own
		// evidence, spot delta or no, because the operator is coming anyway.
		if !spotDeltaOnly(assertion) {
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator)
		}
		if !r.look.wired() {
			// One look must not mint a permanent operator incident (record
			// §70c), and silently latching anyway would be exactly that — so
			// an unwired bracket meeting the one condition it exists for is a
			// FAILURE that names the wiring, not a latch and not a pass.
			out.Cause = ErrSecondLookUnwired
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeFailed)
		}
		r.look.Wait(r.look.Settle)
		second, err := r.join.Join(rec.Dispatched)
		if err != nil {
			out.Cause = err
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeFailed)
		}
		if len(second.Incomplete) > 0 || len(second.Unattributed) > 0 {
			// The re-join itself did not conclude: the world is still moving,
			// and a delta computed over it proves nothing either way.
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeIncomplete)
		}
		if newAbsence(joined.Absent, second.Absent) {
			// An order the venue held at the first look and does not hold now
			// changed evidence INSIDE the settle window — §5.3's resolution
			// decides what that absence licenses, and this bracket does not
			// run it, so the re-assert would read a projection the ordering
			// (join → resolve → assert) says is not concluded. NEWLY absent
			// only: an order absent at BOTH looks was resolved through §5.3
			// before the first assert ever ran and re-reports absent on every
			// boot for as long as the venue has purged it — treating that as
			// motion would leave any deployment holding one AbsentAfterFinality
			// order unable to latch a real spot delta, for ever (§70i).
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeIncomplete)
		}
		reasserted, err := r.join.Assert(rec.Dispatched)
		if err != nil {
			out.Cause = err
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeFailed)
		}
		out.Assertion = reasserted
		if !reasserted.Holds() {
			// The re-join explained it — the late fill's row arrived and moved
			// ours to meet the wallet. Fall through to the unconcluded check
			// below rather than passing outright.
		} else if spotDeltaOnly(reasserted) && spotDeltasEqual(assertion, reasserted) {
			// STABLE across two looks a settle bound apart: a real delta.
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator)
		} else if !spotDeltaOnly(reasserted) {
			// The second look surfaced a finding of another kind; it latches
			// on its own evidence.
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator)
		} else {
			// A delta both looks saw but not the SAME delta: still moving.
			return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeIncomplete)
		}
		assertion = reasserted
	}
	if len(assertion.SpotUnconcluded) > 0 {
		// The check ran and refused to conclude about a baselined asset whose
		// balance semantics were unpinned this read — a resting spot order
		// completes and the next attempt pins. boot.OutcomeIncomplete's cell,
		// exactly as the venue join uses it one gate over.
		return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeIncomplete)
	}
	return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomePassed)
}

// spotDeltaOnly reports whether a holding assertion holds ONLY because of
// spot deltas — the one finding whose first observation may be a read-skew
// artifact rather than a fact (record §70c).
func spotDeltaOnly(a Assertion) bool {
	if len(a.SpotUnexplained) == 0 {
		return false
	}
	if len(a.Unexplained) > 0 || len(a.Foreign) > 0 {
		return false
	}
	for _, u := range a.Unasserted {
		if holds, _ := u.Reason.HoldsPosture(); holds {
			return false
		}
	}
	return true
}

// newAbsence reports whether the second look's join found an order absent that
// the first look's did not — evidence that changed inside the settle window,
// which only §5.3's resolution may interpret. Absences present in BOTH looks
// are not motion: they were resolved before the first assert ran (§70i).
func newAbsence(first, second []ids.ClientOrderID) bool {
	if len(second) == 0 {
		return false
	}
	seen := make(map[ids.ClientOrderID]struct{}, len(first))
	for _, id := range first {
		seen[id] = struct{}{}
	}
	for _, id := range second {
		if _, held := seen[id]; !held {
			return true
		}
	}
	return false
}

// spotDeltasEqual reports whether two looks saw the SAME spot deltas — same
// cells, same numbers. Both slices are sorted by Assert, so the comparison is
// positional; the quantities compare by value, not representation.
func spotDeltasEqual(a, b Assertion) bool {
	if len(a.SpotUnexplained) != len(b.SpotUnexplained) {
		return false
	}
	for i := range a.SpotUnexplained {
		x, y := a.SpotUnexplained[i], b.SpotUnexplained[i]
		if x.Venue != y.Venue || x.Asset != y.Asset {
			return false
		}
		cmp, err := x.Unexplained.Cmp(y.Unexplained)
		if err != nil || cmp != 0 {
			return false
		}
	}
	return true
}

// reinstateGenerations is §7.5's rebuild at generation scope, reported into
// GateGenerationsReinstated.
//
// # It runs here and could not run anywhere earlier
//
// A generation's ledger entry carries its OUTSTANDING NET INTENT, which before
// the authorization is the confirmed perp reduction — a quantity that exists in
// no journal at either scope and reaches this process only through the venue.
// The replay leaves every cumulative at zero, the join fills them in, and §5.3's
// resolution is what sweeps the orders the join declined to. So this is the
// first point in the sequence at which the number is a fact, and rebuilding
// earlier does not merely guess: a generation whose log holds an authorization of
// s would be rebuilt against a confirmed perp reduction of zero and refused by
// its own never-net-short cap.
//
// # An error return means the REPORT could not be filed, and nothing else
//
// Reconcile's own rule. A reinstatement that fails is a verdict — the gate
// carries it and the sequence continues to the assertion, because each gate
// reports on its own evidence and an operator holding a refusal wants the
// picture of the account behind it. What comes back as an error here is the
// checklist refusing the report.
//
// # The three outcomes, and why the empty case is PASSED rather than unreported
//
// A journal with no live generation and no closed ladder has nothing to
// reinstate, and the gate PASSES on that evidence rather than staying silent.
// Unreported would hold every deployment that never delevers below trading for
// ever, which is the permanent-block shape §7.5's own Unasserted ruling was
// taken to avoid — and it would be reporting "the check did not run" about a
// check that ran and found nothing to do.
//
// A wired reinstatement that concludes with an OWED supersession — the premise
// dissolved while the reserved second leg was still out — is Incomplete rather
// than Passed. The ledger is correct and nothing failed, but the ladder is
// carrying an obligation nothing at boot can discharge: all three of §7's rows
// quiesce that leg to §6.7 finality FIRST, and finality is the venue's to give.
// Incomplete is that cell exactly — it ran, it concluded, and it left work a
// further attempt may finish.
func (r *Reconciler) reinstateGenerations(rec Recovery, c *boot.Checklist, out *Reconciliation) error {
	if r.reinstate == nil {
		if len(rec.LiveGenerations) == 0 && len(rec.BarredLadders) == 0 {
			// Nothing to reinstate, so an unwired phase is not a wiring defect —
			// it is a deployment whose ladder never ran. SecondLook's rule: the
			// bracket is optional and its ABSENCE is only a failure on the boot
			// that meets the condition it exists for.
			return c.Report(boot.GateGenerationsReinstated, boot.OutcomePassed)
		}
		out.Cause = ErrGenerationRestoreMissing
		return c.Report(boot.GateGenerationsReinstated, boot.OutcomeFailed)
	}

	if err := c.Report(boot.GateGenerationsReinstated, boot.OutcomePending); err != nil {
		return err
	}
	reinstated, err := r.reinstate.Reinstate(rec)
	out.Reinstatement = reinstated
	if err != nil {
		out.Cause = err
		return c.Report(boot.GateGenerationsReinstated, boot.OutcomeFailed)
	}
	if reinstated.AwaitingQuiesce > 0 {
		return c.Report(boot.GateGenerationsReinstated, boot.OutcomeIncomplete)
	}
	return c.Report(boot.GateGenerationsReinstated, boot.OutcomePassed)
}

// resolveMarginOps is §7.5's in-flight `MarginOp` phase, reported into
// GateMarginOpsResolved — the gate's first producer, after five slices of being
// declared with none.
//
// # The empty case PASSES, and the two things it must not do are opposite
//
// reinstateGenerations' ruling, and it binds harder here because for now the
// empty case is the ONLY case any real boot reaches: nothing writes a `MarginOp`
// until part 2. Staying unreported would hold every deployment below trading for
// ever over a record kind none of them has yet written. Reporting a FAILURE
// would do the same and blame the venue for it.
//
// What makes the pass honest rather than vacuous is that the empty case is
// distinguishable from the working case only in the fixtures — which is why the
// suite writes records through ports.MarginOpJournal directly, and why a clean
// run of this phase against a real journal is evidence of nothing at all.
//
// # An unwired resolver is a wiring defect ONLY on the boot that meets work
//
// SecondLook's pattern, third instance. What it must never be is a pass: a boot
// that meets a journaled margin op with no resolver would report §7.5's
// precondition satisfied about a non-idempotent write that may have moved real
// money, and the log would still hold the op the next time.
//
// The CARRIED ambiguities are checked in this arm too, and that is the half a
// reader would omit. An op a previous process resolved to Ambiguous constrains
// this one — that is the whole reason the verdict is durable — so a deployment
// that dropped the resolver from its wiring must not thereby lose the incident.
func (r *Reconciler) resolveMarginOps(rec Recovery, c *boot.Checklist, out *Reconciliation) error {
	if r.marginOps == nil {
		if len(rec.InFlightMarginOps) == 0 && len(rec.AmbiguousMarginOps) == 0 {
			return c.Report(boot.GateMarginOpsResolved, boot.OutcomePassed)
		}
		out.Cause = ErrMarginOpResolveMissing
		return c.Report(boot.GateMarginOpsResolved, boot.OutcomeFailed)
	}

	if err := c.Report(boot.GateMarginOpsResolved, boot.OutcomePending); err != nil {
		return err
	}
	resolution, err := r.marginOps.Resolve(rec)
	out.MarginOps = resolution
	if err != nil {
		out.Cause = err
		// A venue that would not answer, or a record that could not be resolved.
		// OutcomeFailed rather than a ladder state, because this runs once inside
		// a boot and the checklist is what decides next — see
		// MarginOpResolver.Resolve.
		//
		// The partial resolution is kept: verdicts written before the failure are
		// true, and re-deriving them at the next boot is what the durable verdict
		// exists to prevent. If any of them was an ambiguity, the next boot reads
		// it back and reports this gate blocking regardless of what happens here.
		return c.Report(boot.GateMarginOpsResolved, boot.OutcomeFailed)
	}
	return c.Report(boot.GateMarginOpsResolved, resolution.Outcome())
}

// resolveAbsences runs §5.3's rule over the orders the venue's order surface did
// not hold, and records what it could not conclude.
//
// A Resolve that returns an error is the caller's failure to report; a Resolve
// that returns OutcomeUnproved is the rule WORKING, and the reason is what
// decides whether a retry could ever help. See joinOutcome.
func (r *Reconciler) resolveAbsences(absent []ids.ClientOrderID, out *Reconciliation) error {
	for _, id := range absent {
		res, err := r.resolve.Resolve(id)
		if err != nil {
			return fmt.Errorf("resolve absent order %q: %w", id, err)
		}
		if err := res.Outcome.Validate(); err != nil {
			return fmt.Errorf("resolve absent order %q: %w", id, err)
		}
		if res.Outcome == OutcomeUnproved {
			if err := res.Reason.Validate(); err != nil {
				return fmt.Errorf("resolve absent order %q: %w", id, err)
			}
			out.Unproved = append(out.Unproved, UnprovedOrder{Order: id, Reason: res.Reason})
			continue
		}
		out.Resolved++
		if res.Journaled && !res.Applied {
			// §6 step 7's condition, reached through recovery: the journal holds a
			// state the projection does not. The journal is what a later replay
			// rebuilds from, so the record is not lost — what is wrong is the store
			// the net-delta assertion would compute over.
			out.Stale = append(out.Stale, id)
		}
	}
	return nil
}

// joinOutcome reduces the venue-rebuild findings to the one outcome
// GateVenueJoined carries.
//
// The order of the cases is the posture ladder and is not arbitrary: the first
// case that holds is the most restrictive answer available, so a boot with two
// findings reports the one that permits least.
func (r *Reconciler) joinOutcome(out Reconciliation) boot.Outcome {
	// An execution the venue states and this system cannot name is exposure that
	// exists whether or not we can attribute it. Not lifted by running the join
	// again: the same account answers the same way.
	if len(out.Join.Unattributed) > 0 {
		return boot.OutcomeNeedsOperator
	}
	// An absence §5.3 can NEVER prove is an order that will be just as
	// unresolvable on the next boot and the one after — task #175's condition,
	// and the reason UnprovedReason has to answer this question rather than the
	// caller counting reasons it recognises. A reason that does not validate is
	// treated as permanent for the direction it fails in: an unrecognised gate
	// is not one to assume a retry clears.
	for _, u := range out.Unproved {
		if lifts, err := u.Reason.RetryMayLift(); err != nil || !lifts {
			return boot.OutcomeNeedsOperator
		}
	}
	// The check ran, the venue answered, and the rebuild is short: a sweep that
	// did not prove coverage through the instant it was asked about, or an
	// absence whose finality timer has not elapsed. Nothing failed and no
	// operator is needed — see boot.OutcomeIncomplete for why that cell had to
	// exist before this line could be written.
	if len(out.Join.Incomplete) > 0 || len(out.Unproved) > 0 {
		return boot.OutcomeIncomplete
	}
	return boot.OutcomePassed
}
