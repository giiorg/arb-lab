package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// MarginOpReconciler is the operator act that ends an Ambiguous margin op's
// incident — core §7.1's "the staged-float ledger is reconciled by explicit
// accounting / operator action, a §9 case" (task #213, record §82).
//
// # What it is FOR, in one sentence that is not about margin
//
// Task #212 made the ambiguity durable and made it LATCH, which is what §7.1 and
// §7.5 both require and which is what a restart must not launder. The
// consequence it could not close: nothing in the system lifts a journaled
// ambiguity, so the next boot reads it back and blocks new openings again, for
// ever. This is the only thing that ends one, and it must exist before the
// dispatcher does — otherwise the first real ambiguity is a permanent halt.
//
// # It reads the LOG to decide, and that is what makes it a use case
//
// The act itself is one append. Everything else here is establishing that the
// act has a subject: an op the log knows, that a verdict ended, that the verdict
// was AMBIGUOUS, and that nobody has already cleared it. Those four questions
// are answered by the replay's own Recovery, which is why this takes one rather
// than a journal path — the caller has already walked the log, and a second walk
// inside here would be a second reading of a store whose whole discipline is one
// ordered walk.
//
// # It issues NOTHING at the venue, and the omission is the design
//
// MarginOpResolver's prohibition, and this one is broader: an operator
// reconciling a float may well have moved money by hand at the venue, and this
// system records that they did rather than doing it. There is no margin write
// path anywhere in this package (§7.1's dispatcher is #215), so the act cannot
// be mistaken for the remedy — it is the record that the remedy was taken.
//
// # Not safe for concurrent use
//
// The Reconciler's argument, unchanged, and here it is nearly free: the journal
// is a single-writer store and this act runs from a command with the engine
// stopped.
type MarginOpReconciler struct {
	journal ports.MarginOpJournal
}

// NewMarginOpReconciler constructs the use case.
//
// It takes the journal alone. There is no clock, and that is deliberate rather
// than an omission this file forgot: the reconciliation's instant is the
// OPERATOR's — §7.3's "when" — and reading it from a clock here would stamp the
// act with the moment the command ran rather than the moment the accounting was
// done. Those differ by however long it took the operator to get to a terminal,
// and the audit wants the second one. It arrives on the record.
func NewMarginOpReconciler(j ports.MarginOpJournal) (*MarginOpReconciler, error) {
	if j == nil {
		return nil, ErrMarginOpJournalMissing
	}
	return &MarginOpReconciler{journal: j}, nil
}

// Reconcile records the operator act, refusing every op that has no open
// incident for it to end.
//
// # Four refusals, and each names a DIFFERENT thing for the operator to do next
//
// They are separate sentinels rather than one "not an open ambiguity" because
// this is a person mid-act on a §9 incident, and the four conversations do not
// overlap: fix the id you typed; wait for the boot that concludes it; there is
// nothing to clear; it is already cleared, by this person at this time.
//
// The four rest on Recovery's collections being a PARTITION of the dispatched
// ops — in flight, ambiguous, reconciled, settled. That is what lets "in none of
// them" mean UNKNOWN and nothing else. Without the partition, an op the evidence
// settled and an op that was never dispatched would produce the same refusal,
// and one of the two messages would be a false statement about the log.
//
// # A failure leaves the log unchanged, which is the opposite of Resolve's rule
//
// MarginOpResolver keeps what it wrote on failure, because a verdict already
// reached is true and re-deriving it later is the hazard. Nothing is reached
// here: the act either happened or it did not, and a half-written audit entry
// is an act attributed to nobody. The single append is the whole write.
func (r *MarginOpReconciler) Reconcile(rec Recovery, act domain.MarginOpReconciliation) error {
	if err := act.Validate(); err != nil {
		return err
	}

	for _, prior := range rec.ReconciledMarginOps {
		if prior.Op == act.Op {
			return fmt.Errorf("margin op %q was reconciled by %q at %s: %w",
				act.Op, prior.Actor, prior.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
				ErrMarginOpAlreadyReconciled)
		}
	}
	if status, ok := rec.SettledMarginOps[act.Op]; ok {
		return fmt.Errorf("margin op %q resolved %s and blocks nothing: %w",
			act.Op, status, ErrMarginOpNotAmbiguous)
	}
	for _, op := range rec.InFlightMarginOps {
		if op.ID == act.Op {
			return fmt.Errorf("margin op %q has no verdict yet: %w",
				act.Op, ErrMarginOpNotAmbiguous)
		}
	}

	open := false
	for _, id := range rec.AmbiguousMarginOps {
		if id == act.Op {
			open = true
			break
		}
	}
	if !open {
		// Reached only when the id is in none of the four collections, which the
		// partition makes exactly "the log has no dispatch record for it". The
		// realistic cause is a mistyped id, and saying so is the whole value of
		// the partition: the alternative message — "not an open ambiguity" —
		// would be true of an id that does not exist and would send an operator
		// looking for a state change that never happened.
		return fmt.Errorf("margin op %q: %w", act.Op, ErrMarginOpNotDispatched)
	}

	if err := r.journal.AppendMarginOpReconciled(act); err != nil {
		return fmt.Errorf("record margin op %q reconciliation: %w", act.Op, err)
	}
	return nil
}
