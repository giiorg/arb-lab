package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// MarginOpJournal is §5.1's durable, append-only, hash-chained write-ahead
// journal at MARGIN-OPERATION scope — core §7.1's `MarginOp` record and the
// verdict that ends it.
//
// # Every method returns only when the record is DURABLE, and there is no Sync
//
// Journal's contract, restated because §7.1 states it too and states it as the
// point: the record is "fsynced BEFORE dispatch", which is the §5.3
// crash-boundary discipline applied to a write that has no venue-side identity
// to look up afterwards. A buffered implementation satisfies both signatures
// below and defeats the only clause either has — after a crash, recovery finds a
// position whose margin may or may not have moved and no record that anything
// was ever attempted, which is the state the whole record exists to make
// impossible.
//
// # Why this is a THIRD port and not more methods on either existing one
//
// ports.Journal keys everything by ids.ClientOrderID and GenerationJournal by
// domain.GenerationID; this one's subject is a domain.MarginOpID. The three id
// spaces share no namespace, so widening either existing key would put "a string
// that is sometimes an order, sometimes a generation and sometimes a margin op"
// into a primary key — where a lookup for one can match another.
//
// Separate PORTS is again not the same claim as separate STORES, and the single
// chain buys the same kind of fact it buys for generations: a resolution record
// provably follows its dispatch record, checked by the walk that checks
// everything else rather than by a discipline this package keeps.
//
// # Two methods, and the asymmetry between them is the design
//
// A margin op is not a state machine with phases. It is ONE external write that
// either landed or did not, and what evolves afterwards is the evidence, which
// lives at the venue. So there is a record that says the attempt exists and a
// record that says what it turned out to be, and nothing in between.
type MarginOpJournal interface {
	// AppendMarginOpDispatched durably records core §7.1's `MarginOp` — "local
	// op ID, intended delta, before-state snapshot + venue watermark" — BEFORE
	// the venue call it describes.
	//
	// It is AppendPlanned's analogue: the record that says the subject EXISTS,
	// carrying everything fixed for its lifetime. Every field is required, and
	// the two that would be tempting to make optional are the two recovery
	// cannot proceed without: a record without its before-state has no
	// comparison to make, and one without its watermark cannot tell a dispatch
	// that is still landing from one that never did.
	//
	// It has no caller in this slice. The dispatcher — the before-state read, this
	// write, `AddIsolatedMargin`, and §7.1's one-op-in-flight-per-position
	// serialization — is part 2, which lands after the resolution path that
	// gives its ambiguous outcomes somewhere to go.
	AppendMarginOpDispatched(op domain.MarginOp) error

	// AppendMarginOpResolved durably records what the evidence concluded.
	//
	// # The verdict is STORED rather than re-derived, and both wrong answers a
	// # re-derivation gives are wrong in a different direction
	//
	// AppendGenerationSuperseded's argument at a second subject, and here it has
	// two halves rather than one. An `Ambiguous` op blocks new openings "until
	// the staged-float ledger is explicitly reconciled" (§7.5) — a fact about the
	// op, not about a process, so held only in a checklist it would die with the
	// process and a RESTART would satisfy the reconciliation requirement. And an
	// `Applied` re-derived later INVERTS: Binance's `positionMargin/history` is a
	// windowed query over rows the venue retains for a bounded time, so the same
	// op resolves by a matching row today and by that row's absence tomorrow,
	// which is `NotApplied` — the reading under which nothing happened.
	//
	// An implementation MUST refuse a status that is not CONCLUDED
	// (domain.MarginOpStatus.Concluded): `Pending` is a refusal to answer, and a
	// pending in the log is a value every later reader has to know to ignore.
	AppendMarginOpResolved(id domain.MarginOpID, status domain.MarginOpStatus) error
}
