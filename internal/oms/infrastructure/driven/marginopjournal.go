package driven

import (
	"database/sql"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// §3.6 rule 2's compile-time assertion for the margin-op half. A fourth line
// rather than a wider one on any of the others, for generationjournal.go's
// reason: the ports are deliberately separate, and an assertion naming only one
// of them would go on compiling if every method below were deleted.
//
// Under mutation this reports NOBUILD!!, which for THIS assertion is a kill
// rather than a non-result — see CLAUDE.md's verdict table.
var _ ports.MarginOpJournal = (*SQLiteJournal)(nil)

// The journal's MARGIN-OP half: core §7.1's durable target-state machine,
// written into the same chain as the order and generation records.
//
// # One adapter satisfying three ports, and the third is where the chain earns
// # its keep twice
//
// ports.MarginOpJournal argues that it must be a separate PORT — its subject is
// a domain.MarginOpID, a third id space with no namespace shared with the other
// two — and just as explicitly that separate ports is not separate STORES. Here
// the single chain buys two facts rather than one:
//
//   - a resolution provably FOLLOWS its dispatch, so a log holding a verdict for
//     an op that was never journaled is a log the walk can refuse rather than one
//     a use case has to notice;
//   - a margin op's dispatch is ordered against the ORDER records around it, which
//     is what makes "the position was mutated inside the evidence window" a
//     question the log can be asked at all. Under two chains the fill and the
//     margin op have no order between them, and §7.1's second named cause of
//     ambiguity would be invisible to recovery.
//
// # What these methods refuse is what a REPLAY could not rebuild, and one thing
// # more
//
// AppendPlanned's boundary, with a single deliberate addition: a status that is
// not CONCLUDED. That is not a well-formedness check — `pending` is a perfectly
// valid domain.MarginOpStatus — it is a claim about what belongs in an
// append-only log, and it is refused here because here is the only place that
// can refuse it before it is durable.

// AppendMarginOpDispatched durably records core §7.1's `MarginOp` before the
// venue call it describes.
//
// The record is validated WHOLE rather than field by field, and unlike the
// generation opening — where the hedge group is validated whole and the target
// only for its asset — nothing here is left to a domain check further on. There
// is no aggregate for a margin op to be handed to: the record IS the state, and
// the only thing that will ever read it is a resolution comparing its three
// numbers against a live one. A field this method admits is a field that
// resolution will compute with.
func (j *SQLiteJournal) AppendMarginOpDispatched(op domain.MarginOp) error {
	if err := op.Validate(); err != nil {
		return fmt.Errorf("oms journal: append margin op dispatched: %w", err)
	}
	return j.appendRecord(kindMarginOpDispatched, string(op.ID),
		encodeMarginOpDispatched(op),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_margin_op_dispatched
				(seq, venue, product, symbol, contract, asset, delta_mantissa, before_mantissa, watermark_unix_nano)
				VALUES (?,?,?,?,?,?,?,?,?)`,
				seq, string(op.Instrument.Venue), string(op.Instrument.Product),
				op.Instrument.Symbol, string(op.Instrument.Contract),
				string(op.Delta.Asset()), op.Delta.Mantissa(), op.BeforePosted.Mantissa(),
				op.Watermark.UnixNano())
			return err
		})
}

// AppendMarginOpResolved durably records what the evidence concluded.
//
// # `Pending` is refused, and the refusal is at the WRITE end for a reason
//
// domain.MarginOpStatus.Concluded is the predicate, and a pending is not a
// verdict — it is this system declining to answer because too little time has
// passed. Written into an append-only log it becomes a permanent record of a
// momentary condition, and every later reader has to know to skip it. Refusing
// it at the read end instead would mean the bytes are already durable and the
// only question left is who ignores them.
//
// This is the one check in this file that is NOT well-formedness, which makes it
// worth naming as what it is: a claim about the log's contents rather than about
// the record's shape. It is here rather than in the use case because a use case
// that forgot it would produce a journal no later build could interpret, and the
// port is where "what may be written" is decided.
//
// Whether this op EXISTS is not checked, and that boundary is AppendState's
// unchanged: legality belongs to the caller, and a recorder holding a copy of it
// would be a second home for the rule. What the single chain gives instead is
// better than a check — a resolution record sits after its dispatch record in
// sequence order, so a replay that meets a verdict for an op it has never seen
// has a location and a name for the problem.
func (j *SQLiteJournal) AppendMarginOpResolved(id domain.MarginOpID, status domain.MarginOpStatus) error {
	if id == "" {
		return fmt.Errorf("oms journal: append margin op resolution: %w", domain.ErrMarginOpIDUnset)
	}
	concluded, err := status.Concluded()
	if err != nil {
		return fmt.Errorf("oms journal: append margin op %q resolution: %w", id, err)
	}
	if !concluded {
		return fmt.Errorf("oms journal: append margin op %q resolution %s: %w",
			id, status, ports.ErrMarginOpStatusNotConcluded)
	}
	return j.appendRecord(kindMarginOpResolved, string(id),
		encodeMarginOpResolved(id, status),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_margin_op_resolved (seq, status) VALUES (?,?)`,
				seq, string(status))
			return err
		})
}

// AppendMarginOpReconciled durably records core §7.3's per-actor audit of the
// operator act that ends an Ambiguous op's incident (task #213, record §82).
//
// # The record is validated WHOLE, and every one of its refusals is a MISSING
// # field rather than a malformed one
//
// AppendMarginOpDispatched validates whole because the record IS the state.
// This one validates whole for a different reason: its four fields are §7.3's
// who / what / when plus what-was-done, and three of them are typed by a person
// at a prompt. The realistic defect is not a wrong value but an absent one — a
// command wired with a flag nobody passed — and an absent actor is exactly the
// artifact the audit requirement exists to prevent.
//
// # Whether the op is ambiguous is NOT checked here, and that is the same
// # boundary AppendMarginOpResolved draws
//
// Legality belongs to the caller: application.MarginOpReconciler refuses an op
// that does not exist, one still in flight, one whose verdict is not ambiguous,
// and one already reconciled. A recorder holding a copy of those rules would be
// a second home for them, and it could not enforce them anyway — it would have
// to replay the log to know, which is the reader's job.
//
// What the single chain gives instead is better than a check: this record sits
// after the verdict it clears in sequence order, so a replay meeting a clearance
// for an op that is not ambiguous at that point has a location and a name for
// the problem.
func (j *SQLiteJournal) AppendMarginOpReconciled(rec domain.MarginOpReconciliation) error {
	if err := rec.Validate(); err != nil {
		return fmt.Errorf("oms journal: append margin op reconciliation: %w", err)
	}
	return j.appendRecord(kindMarginOpReconciled, string(rec.Op),
		encodeMarginOpReconciled(rec),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_margin_op_reconciled
				(seq, actor, at_unix_nano, note) VALUES (?,?,?,?)`,
				seq, rec.Actor, rec.At.UnixNano(), rec.Note)
			return err
		})
}
