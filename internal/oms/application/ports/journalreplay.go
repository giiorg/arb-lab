package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// JournalReplay is §7.5's read half of the write-ahead journal: the log walked
// in sequence order, with its hash chain verified as it goes.
//
// # It is a second port rather than three more methods on Journal, and the
// # split is the same one the dispatch path already relies on
//
// Journal is what §6 WRITES; this is what §7.5 READS, and nothing holds both.
// The planner and the dispatcher are given a Journal and cannot replay one; the
// replayer is given this and cannot append. That is ports.OrderResolver's move
// (the read half of the venue surface, so that "never resubmitted" is
// unexpressible) applied to the local store — and it is worth more here than
// tidiness, because the two capabilities compose into the one thing recovery
// must never do: a replayer holding a Journal could write records while
// deciding what the records mean, and the log it was reasoning about would
// change under it.
//
// # Verification is the ADAPTER's, and it is not optional
//
// §5.1 makes the journal hash-chained and tamper-evident, and until this port
// existed nothing in the system ever recomputed a link — the chain was written
// by every append and verified by a test's independent replayer alone. An
// implementation of Replay MUST verify, because the caller has no way to: the
// preimage is the adapter's own encoding, so a use case that wanted to check
// would have to hold a second copy of the format, which is exactly what
// journalcodec.go's oracle discipline forbids.
//
// What a failed verification MEANS is the caller's, and §7.5 has an answer that
// is neither of the obvious two — see boot.OutcomeEvidenceUnusable.
type JournalReplay interface {
	// Replay walks every record in sequence order and hands each to the
	// visitor, stopping at the first one the visitor refuses.
	//
	// It returns ErrJournalChainBroken (possibly wrapped) when the chain does
	// not verify, and the visitor's own error otherwise. The two are
	// deliberately distinguishable: the first says the LOG cannot be trusted and
	// the second says the log's contents cannot be applied, and §7.5's posture
	// consequences differ.
	//
	// An empty journal is a successful walk that visits nothing. That is the
	// ordinary first boot, and reporting it as an error would make the one case
	// with nothing to recover the case that refuses to start.
	Replay(JournalVisitor) error
}

// JournalVisitor is what a replay hands each record to — one method per record
// kind, mirroring Journal's own three.
//
// # It is a visitor and not a record struct, for the reason Journal has three
// # Append methods rather than one
//
// A single JournalRecord type would carry an order on the planned records, an
// instant on the authorization records and a state on the rest — optional
// fields whose zero values are legal and meaningless, with a kind tag the
// caller must remember to branch on. The compiler would have nothing to say
// about a replayer that forgot a kind, and the record that went unhandled would
// be silently absent from the rebuilt state rather than reported.
//
// With a method per kind the omission is a compile error, and a FOURTH record
// kind added by a later slice breaks every visitor in the system at build time.
// That is exhaustiveswitch's argument (a `default:` is where the new member
// lands silently) expressed in the type system instead, which is available here
// because the set of kinds is the set of methods.
//
// Every method takes the sequence, which the visitor needs for exactly one
// thing: saying WHICH record it refused. A replay that fails on the ninth of
// nine hundred records is otherwise a failure with no location.
type JournalVisitor interface {
	// Planned is one AppendPlanned record: an order this system decided to
	// place, exactly as §6 step 5 journaled it.
	Planned(seq uint64, o domain.Order) error

	// Authorized is one AppendAuthorized record: the commitment to call Place
	// once, with §5.3's anchor.
	Authorized(seq uint64, id ids.ClientOrderID, at clock.ServerTime) error

	// Acked is one AppendAcked record: the venue's own identifier for the
	// order, as the Place ack carried it.
	Acked(seq uint64, id ids.ClientOrderID, venueOrderID string) error

	// State is one AppendState record: an order reaching a lifecycle state.
	State(seq uint64, id ids.ClientOrderID, s domain.OrderState) error
}
