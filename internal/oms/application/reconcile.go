package application

import (
	"fmt"

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
	replay  *Replayer
	join    *Joiner
	resolve *Resolver
}

// NewReconciler constructs the use case, refusing a missing phase.
//
// All three are required and none is defaulted to a no-op. A Reconciler that
// could run with a nil Resolver would report the venue-joined gate on a boot
// where no absence was ever put to §5.3's rule, and an absence nobody resolved
// is not distinguishable in the report from one that resolved cleanly.
func NewReconciler(replay *Replayer, join *Joiner, resolve *Resolver) (*Reconciler, error) {
	if replay == nil {
		return nil, ErrReplayerMissing
	}
	if join == nil {
		return nil, ErrJoinerMissing
	}
	if resolve == nil {
		return nil, ErrResolveUseCaseMissing
	}
	return &Reconciler{replay: replay, join: join, resolve: resolve}, nil
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
	if err := c.Report(boot.GateJournalReplayed, boot.OutcomePassed); err != nil {
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
		// The assertion does not run over a rebuild that did not conclude, and the
		// gate stays UNREPORTED rather than being failed. Unreported is the honest
		// value — the check never ran — and it is not the permissive reading: the
		// fold is over the REQUIRED gates, so an unreported one holds the posture
		// at reconcile-only exactly as a failure would.
		return out, nil
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
	if assertion.Holds() {
		return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomeNeedsOperator)
	}
	return out, c.Report(boot.GateNetDeltaAsserted, boot.OutcomePassed)
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
