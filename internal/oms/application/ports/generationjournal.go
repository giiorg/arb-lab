package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// GenerationJournal is §5.1's durable, append-only, hash-chained write-ahead
// journal at GENERATION scope — SM §7's FLAG-3 record and the phases around it.
//
// # Every method returns only when the record is DURABLE, and there is no Sync
//
// Journal's contract, restated because it binds here for a sharper reason.
// FLAG-3 rules `SpotReduceAuthorized` "a durable journal record … so recovery
// knows the perp-finality precondition was met without re-derivation". A
// buffered implementation satisfies every signature below and defeats the only
// clause any of them has: after a crash, recovery finds a spot order out at the
// venue with no record that any perp reduction ever confirmed, which is the
// precise ordering §7's never-net-short sequencing exists to enforce.
//
// # Why this is a SECOND port and not more methods on Journal
//
// Journal's every method is keyed by ids.ClientOrderID, and its store keys every
// record by one — the `client_id` column is NOT NULL and every chained preimage
// begins with it. A generation's subject is a domain.GenerationID: SM §7's "each
// generation is a bounded attempt" makes the identity outlive any single order
// on either leg, so one generation's legs are two client ids and neither of them
// is the generation. Widening Journal's key to "a string that is sometimes an
// order and sometimes a generation" is the optional-field shape that port's own
// doc refuses, moved into the primary key, where it is worse: the two id spaces
// share no namespace, so a lookup for one can match the other.
//
// Separate PORTS is not the same claim as separate STORES, and the distinction
// is deliberate. An adapter may satisfy both over ONE chain, and that is the
// choice worth making: the invariant a replay most wants to check is that a
// generation's authorization record precedes its spot order's, and two chains
// have no order between them, so under two stores that invariant stops being a
// fact anything can check and becomes a discipline this package keeps.
//
// # Six methods, and the width is Journal's own totality argument applied four
// # times rather than a case analysis performed once
//
// One AppendGeneration(record) needs a type carrying a target on the opening, a
// client order id on each of the two leg submissions, a size on the
// authorization, a supersession path on one ending, and none of those on the
// remaining five phases. Every one of those is an optional field whose zero
// value is legal, and two of them are actively dangerous:
//
//   - the zero AUTHORIZATION is a real, legal, terminal decision — a perp
//     reduction that confirmed nothing authorizes no spot sale and the
//     generation completes — so a dropped size and a genuine zero are the same
//     bytes, and recovery cannot tell them apart;
//   - an absent LEG ID reads as a generation with no own legs, which is exactly
//     the netting FM-9 requires (domain.Generation.OwnLegs) evaluated over the
//     wrong set. The premise then contains its own subject and the standing
//     trigger fires on the generation's own machinery — the defect, arriving
//     through recovery instead of through the trigger.
//
// So each payload gets a METHOD, and a phase that carries one is not reachable
// through the phase recorder at all.
type GenerationJournal interface {
	// AppendGenerationOpened durably records SM §7's `Planned`: a bounded
	// attempt exists, on this hedge group, holding this reserved-reduction
	// entry, for this per-leg target.
	//
	// It is AppendPlanned's analogue — the record that says the subject EXISTS,
	// carrying everything fixed for its lifetime. The phase is not a parameter
	// for AppendPlanned's reason: there is one phase a generation can be
	// journaled into for the first time, and a parameter would invite a caller to
	// name another.
	//
	// The reservation is carried rather than derived because it is the handle the
	// ledger is keyed by, and the ledger is what §2's sizing subtracts. A
	// recovered generation that could not name its own entry could not amend it,
	// retire it, or tell it from a sibling's — and an entry nobody retires
	// subtracts from every later sizing for the life of the process.
	AppendGenerationOpened(id domain.GenerationID, group domain.HedgeGroup,
		reservation domain.ReservationID, target fixedpoint.Quantity) error

	// AppendGenerationPerpSubmitted durably records `PerpReduceInFlight` with
	// the id of the buy-to-reduce order it submitted.
	//
	// The id is here because domain.Generation.OwnLegs is FM-9's quantity, and a
	// generation recovered without it computes its premise over a set that still
	// contains its own perp reduction — the raw reading model (a)'s control NC22
	// aborts on four states from Init.
	AppendGenerationPerpSubmitted(id domain.GenerationID, order ids.ClientOrderID) error

	// AppendGenerationAuthorized durably records FLAG-3's `SpotReduceAuthorized`
	// with the size it fixed — the one record SM §7 names in its own right, and
	// the reason this port exists.
	//
	// size MUST be the authorized spot quantity, capped at the CONFIRMED perp
	// reduction, and it may legally be zero. Its own method rather than an
	// optional field on the phase record, for AppendAuthorized's reason one store
	// over and with a sharper edge: see the type doc.
	AppendGenerationAuthorized(id domain.GenerationID, size fixedpoint.Quantity) error

	// AppendGenerationSpotSubmitted durably records `SpotReduceInFlight` with the
	// id of the reserved spot reduction.
	//
	// A second method rather than a `leg` argument shared with the perp one,
	// because the two are not interchangeable to any reader of this log: the
	// reserved SECOND leg is the one all three supersession rows must quiesce to
	// §6.7 finality first (domain.GenerationPhase.ReservedLegLive), and a record
	// that named an order without saying which leg it was would leave recovery
	// unable to tell whether that obligation binds.
	AppendGenerationSpotSubmitted(id domain.GenerationID, order ids.ClientOrderID) error

	// AppendGenerationSuperseded durably records `Superseded` WITH the row that
	// ended the generation.
	//
	// The path is not decoration. domain.SupersessionPath.ResumesLadder gives the
	// three rows two different answers about whether core §7.1's ladder may open
	// a LATER generation — premise-lost resumes, the two external arbitrations do
	// not — so a supersession recorded without its path leaves recovery holding
	// an ended generation and no way to decide the one question that follows it.
	// Neither journal derives it: the external rows are arbitrations no order
	// event mentions, and the premise-lost row fires on a quantity that leaves no
	// trace at order scope at all.
	//
	// Getting it wrong is not symmetric, which is why it is stored rather than
	// re-derived at boot. Guessing "resumes" re-authorises this machine against
	// an arbitration that has just taken the package away from it; guessing "does
	// not" disables leverage reduction on a neutral package for the rest of the
	// process's life, at the moment it is needed.
	AppendGenerationSuperseded(id domain.GenerationID, path domain.SupersessionPath) error

	// AppendGenerationPhase durably records a generation reaching one of the five
	// phases that carry nothing but themselves — `PerpReduceFinal`, both
	// `Held/Alert` parks, `Complete` and `TerminalFlatten`.
	//
	// It takes the target phase only, not the pair, for AppendState's reason: the
	// log is append-only and replayed in order, so the source phase is already in
	// it, and a replay finding two records disagree has no rule for which wins.
	//
	// It MUST refuse the five phases the methods above own, on the target alone,
	// exactly as AppendState refuses StateDispatchAuthorized and with
	// ErrGenerationPhaseNeedsPayload. Leaving any of them reachable here would
	// leave a record with no target, no leg id, no size or no path in the one
	// store recovery reads.
	//
	// Legality is otherwise the CALLER's, through domain.CanAdvanceGeneration.
	// This port is a recorder, and a recorder that refused illegal transitions
	// would be a second home for SM §7's edge set. The refusals above are not an
	// exception to that: they are about which METHOD carries a phase, not about
	// which transitions into it are legal.
	AppendGenerationPhase(id domain.GenerationID, phase domain.GenerationPhase) error
}
