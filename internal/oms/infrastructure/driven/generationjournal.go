package driven

import (
	"database/sql"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §3.6 rule 2's compile-time assertion for the generation half. A third line
// rather than a wider one on either of the others, for journalreplay.go's
// reason: the three ports are deliberately separate, and an assertion naming
// only ports.Journal would go on compiling if every method below were deleted.
//
// Under mutation this reports NOBUILD!!, which for THIS assertion is a kill
// rather than a non-result — see CLAUDE.md's verdict table.
var _ ports.GenerationJournal = (*SQLiteJournal)(nil)

// The journal's GENERATION half: SM §7's `PackageDelever` records, written into
// the same chain as the order records.
//
// # One adapter satisfying two ports is the point, not a shortcut
//
// ports.GenerationJournal argues at length that it must be a SECOND PORT — its
// subject is a domain.GenerationID that outlives either leg, so widening
// ports.Journal's key would put "a string that is sometimes an order and
// sometimes a generation" into a primary key. It argues just as explicitly that
// separate ports is NOT the same claim as separate stores, and this file is that
// distinction cashed: two interfaces, one type, one table, one chain.
//
// What the single chain buys is a fact rather than a discipline. FLAG-3 makes
// `SpotReduceAuthorized` durable "so recovery knows the perp-finality
// precondition was met without re-derivation", and the thing recovery most needs
// to know is that the authorization record precedes the spot leg's own journaled
// intent. Under two chains those two records have no order between them at all —
// each chain verifies internally and neither positions the other — so the clause
// would hold only as long as the application layer kept it. Here it is the
// sequence column, checked by the same walk that checks everything else.
//
// # Every method below refuses what a REPLAY could not rebuild, and nothing more
//
// AppendPlanned's boundary, restated one machine over. Transition legality is
// the caller's — domain.CanAdvanceGeneration owns SM §7's edge set, and a
// recorder holding a copy of it would be a second home for that table.
// Well-formedness is this layer's, because a record that cannot be rebuilt is a
// record that fails at §7.5's boot, on a process trying to find out what a
// crashed predecessor did. The one apparent exception is AppendGenerationPhase's
// payload refusal, and it is not an exception: it is a claim about which METHOD
// carries a phase, not about which transitions into it are legal.

// AppendGenerationOpened durably records SM §7's `Planned`.
//
// The hedge group is validated whole rather than field by field, and that is
// domain.HedgeGroup.Validate's own argument arriving here: an empty account is
// the field a hydration missed, and it is the one that silently merges every
// subaccount into one group. A generation recovered under a merged group would
// be judged against another account's exposure.
//
// The target is validated for its ASSET and not its sign, and the asymmetry is
// deliberate. An asset-less quantity cannot round-trip through
// QuantityFromMantissa, so the record would be unreplayable; a target whose sign
// or magnitude is wrong is a domain question domain.OpenGeneration has already
// asked, and asking it again here would be the second home this file refuses.
func (j *SQLiteJournal) AppendGenerationOpened(
	id domain.GenerationID,
	group domain.HedgeGroup,
	reservation domain.ReservationID,
	target fixedpoint.Quantity,
) error {
	if id == "" {
		return fmt.Errorf("oms journal: append generation opened: %w", ErrJournalGenerationIDUnset)
	}
	if err := group.Validate(); err != nil {
		return fmt.Errorf("oms journal: append generation %q opened: %w", id, err)
	}
	if reservation == "" {
		return fmt.Errorf("oms journal: append generation %q opened: %w", id, ErrJournalReservationUnset)
	}
	if err := target.Validate(); err != nil {
		return fmt.Errorf("oms journal: append generation %q opened: target %w", id, err)
	}
	return j.appendRecord(kindGenerationOpened, string(id),
		encodeGenerationOpened(id, group, reservation, target),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_generation_opened
				(seq, account, venue, symbol, reservation, target_asset, target_mantissa)
				VALUES (?,?,?,?,?,?,?)`,
				seq, group.Account, string(group.Venue), group.Symbol, string(reservation),
				string(target.Asset()), target.Mantissa())
			return err
		})
}

// AppendGenerationPerpSubmitted durably records `PerpReduceInFlight` with the
// id of the buy-to-reduce order.
//
// The empty order id is refused because a generation recovered without its perp
// leg computes domain.Generation.OwnLegs — FM-9's netting quantity — over a set
// that still contains its own perp reduction. The standing trigger then fires on
// the generation's own machinery, which is the defect arriving through recovery
// instead of through the trigger.
func (j *SQLiteJournal) AppendGenerationPerpSubmitted(id domain.GenerationID, order ids.ClientOrderID) error {
	return j.appendLeg(kindGenerationPerpSubmitted, "perp", id, order,
		encodeGenerationPerpSubmitted(id, order))
}

// AppendGenerationSpotSubmitted durably records `SpotReduceInFlight` with the id
// of the reserved spot reduction.
//
// A separate KIND from the perp submission rather than a leg column beside a
// shared one, for the reason the schema's comment gives and the port's states:
// the reserved second leg is the one all three supersession rows must quiesce to
// §6.7 finality first, so a record that named an order without saying which leg
// it was would leave recovery unable to decide whether that obligation binds.
func (j *SQLiteJournal) AppendGenerationSpotSubmitted(id domain.GenerationID, order ids.ClientOrderID) error {
	return j.appendLeg(kindGenerationSpotSubmitted, "spot", id, order,
		encodeGenerationSpotSubmitted(id, order))
}

// appendLeg is the two leg submissions' shared body.
//
// It takes the already-encoded preimage rather than choosing an encoder from the
// kind, and that is the whole reason this helper is safe to have. Two methods
// that differ only in a tag are exactly where a shared body starts silently
// serving one of them: a switch on the kind inside here would be a third place
// the perp/spot distinction is made, able to disagree with the two call sites.
// Encoded at the call site, the kind and its encoder are named on adjacent
// lines and the helper carries no opinion about which leg it is writing.
//
// leg is for the ERROR TEXT only. It is a separate parameter rather than derived
// from the kind for the same reason — deriving it would be that switch, arriving
// as a formatting convenience.
func (j *SQLiteJournal) appendLeg(
	kind, leg string,
	id domain.GenerationID,
	order ids.ClientOrderID,
	body []byte,
) error {
	if id == "" {
		return fmt.Errorf("oms journal: append generation %s leg: %w", leg, ErrJournalGenerationIDUnset)
	}
	if order == "" {
		return fmt.Errorf("oms journal: append generation %q %s leg: %w",
			id, leg, ErrJournalClientIDUnset)
	}
	return j.appendRecord(kind, string(id), body, func(tx *sql.Tx, seq int64) error {
		_, err := tx.Exec(`INSERT INTO journal_generation_leg (seq, client_id) VALUES (?,?)`,
			seq, string(order))
		return err
	})
}

// AppendGenerationAuthorized durably records FLAG-3's `SpotReduceAuthorized`
// with the size it fixed — the one record SM §7 names in its own right.
//
// # A ZERO size is accepted, and that is the record this method exists for
//
// The port says it in as many words: a perp reduction that confirmed nothing
// authorizes no spot sale and the generation completes, so zero is a real,
// legal, terminal decision. The failure this shape prevents is not a zero being
// written — it is a zero being INDISTINGUISHABLE from a dropped field, which is
// what an optional size on a shared phase record would have made it. Here
// absence has no spelling at all: there is either a record or there is not.
//
// The asset IS required, and that is not a contradiction of the paragraph above.
// A zero mantissa with no asset does not round-trip, and the value it rebuilds
// into is not "zero of nothing" but a quantity §7's never-net-short cap cannot
// compare — so the one legal zero would be the one unreplayable record.
func (j *SQLiteJournal) AppendGenerationAuthorized(id domain.GenerationID, size fixedpoint.Quantity) error {
	if id == "" {
		return fmt.Errorf("oms journal: append generation authorization: %w", ErrJournalGenerationIDUnset)
	}
	if err := size.Validate(); err != nil {
		return fmt.Errorf("oms journal: append generation %q authorization: size %w", id, err)
	}
	return j.appendRecord(kindGenerationAuthorized, string(id),
		encodeGenerationAuthorized(id, size),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(
				`INSERT INTO journal_generation_authorized (seq, size_asset, size_mantissa) VALUES (?,?,?)`,
				seq, string(size.Asset()), size.Mantissa())
			return err
		})
}

// AppendGenerationSuperseded durably records `Superseded` WITH the row that
// ended the generation.
//
// The path is checked for MEMBERSHIP, which is the same line AppendState draws:
// an undeclared path is a value §7.5 cannot replay, while whether this
// generation was eligible for that row is a claim about SM §7's edges that this
// recorder does not hold. domain.SupersessionPath.Validate separates the unset
// value from an unknown one, and the unset one is the realistic accident — a
// caller that ended a generation without saying why.
//
// What makes the membership check worth having rather than pedantic is
// ResumesLadder: it answers (bool, error), and an undeclared path reaching the
// log would replay into a generation whose one remaining question — may the
// ladder open a later generation — has no answer at all.
func (j *SQLiteJournal) AppendGenerationSuperseded(id domain.GenerationID, path domain.SupersessionPath) error {
	if id == "" {
		return fmt.Errorf("oms journal: append generation supersession: %w", ErrJournalGenerationIDUnset)
	}
	if err := path.Validate(); err != nil {
		return fmt.Errorf("oms journal: append generation %q supersession: %w", id, err)
	}
	return j.appendRecord(kindGenerationSuperseded, string(id),
		encodeGenerationSuperseded(id, path),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_generation_superseded (seq, path) VALUES (?,?)`,
				seq, string(path))
			return err
		})
}

// AppendGenerationPhase durably records a generation reaching one of the five
// phases that carry nothing but themselves.
//
// # The five payload phases are refused HERE, on the target alone
//
// Exactly as AppendState refuses StateDispatchAuthorized one machine over, and
// the mirror is deliberate rather than a coincidence of design. Leaving any of
// them reachable through this method would leave a record with no target, no leg
// id, no size or no path in the store §7.5 replays — and the two dangerous ones
// are dangerous in opposite directions. A payload-less authorization is a
// dropped size that reads as the legal zero; a payload-less leg submission is a
// generation with no own legs, which is FM-9's netting evaluated over the wrong
// set.
//
// The refusal is on the TARGET alone, which is what makes it checkable without
// knowing where the generation came from. The port's five owning methods are
// enumerated positively rather than by negating the accepted set, because the
// accepted set is the one that grows: a phase added by a later slice lands in
// neither list, and `exhaustiveswitch` refuses the switch below until somebody
// decides which. Written the other way round, a new phase would silently become
// recordable here with whatever payload it carries dropped.
func (j *SQLiteJournal) AppendGenerationPhase(id domain.GenerationID, phase domain.GenerationPhase) error {
	switch phase {
	case domain.GenerationPlanned,
		domain.GenerationPerpReduceInFlight,
		domain.GenerationSpotReduceAuthorized,
		domain.GenerationSpotReduceInFlight,
		domain.GenerationSuperseded:
		return fmt.Errorf("oms journal: append generation %q phase %s: %w",
			id, phase, ports.ErrGenerationPhaseNeedsPayload)
	case domain.GenerationPerpReduceFinal,
		domain.GenerationHeldPerp,
		domain.GenerationHeldSpot,
		domain.GenerationComplete,
		domain.GenerationTerminalFlatten:
		// The five this method owns.
	}
	if id == "" {
		return fmt.Errorf("oms journal: append generation phase %q: %w", phase, ErrJournalGenerationIDUnset)
	}
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("oms journal: append generation %q phase: %w", id, err)
	}
	return j.appendRecord(kindGenerationPhase, string(id),
		encodeGenerationPhase(id, phase),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_generation_phase (seq, phase) VALUES (?,?)`,
				seq, string(phase))
			return err
		})
}
