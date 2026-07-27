package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// OrderRecord is what the write repository holds for one order: the immutable
// intent, the two things about it that change, and §5.1's OCC version.
//
// # It composes domain.Order rather than restating it
//
// domain.Order's own doc comment says why it carries no State and no fills, and
// ends by naming this type: "the aggregate that binds an Order to its state and
// its fills is a later slice; when it lands, it composes this rather than
// restating it." This is that slice, and the composition is the whole of it —
// there is no field here that also exists in Order, so no pair of fields can
// disagree.
//
// The orthogonality §5.3 requires survives the composition, because nothing in
// this struct relates State to Cumulative: a Filled order and a Canceled one
// both carry whatever the ledger admitted, and a late fill on a terminal order
// moves Cumulative while leaving State alone. A reader looking for the
// enforcement of that will not find it here — it is in the ABSENCE of any rule
// tying the two fields, which is the only place an absence can be.
type OrderRecord struct {
	// Order is the intent, fixed when the order was planned.
	Order domain.Order

	// State is where the order is in SM §1's lifecycle.
	State domain.OrderState

	// Cumulative is the executed quantity the fill ledger has admitted, and it
	// is zero OF THE ORDERED ASSET on a record with no fills — a real answer
	// rather than a missing one, since §6.7 sums it across a hedge group and an
	// unfilled leg contributes zero to that sum.
	Cumulative fixedpoint.Quantity

	// Fills is how many distinct executions were admitted. It exists for
	// reconciliation rather than for arithmetic: a cumulative quantity matching
	// the venue's while the count does not is what a double-counted fill plus a
	// missed one looks like from the quantity alone (domain.Ledger.FillCount
	// makes the same point one layer down).
	Fills int

	// Version is §5.1's OCC version, starting at 1 and incremented by every
	// write that CHANGES the record.
	//
	// It starts at 1 rather than 0 so that the zero value of this struct is
	// unusable in the way the rest of the kernel's types are: a caller that
	// passes the version out of a record it never loaded expects 0, which
	// matches no stored record, so the write is refused with ErrVersionConflict
	// instead of racing whatever is actually there. Zero is not a version; it is
	// the absence of one.
	//
	// "Every write that CHANGES the record" is the exact rule, and the case it
	// decides is the duplicate fill: ApplyFill reports (record, false, nil) and
	// leaves this untouched, because a version bumped by a no-op invalidates
	// every other reader's expectation for nothing.
	Version int64
}

// OrderWriteRepository is §3.6 rule 6's write repository for order and fill
// state: the APPLY half of §6's journal-then-apply path.
//
// # What this port is for, stated against the journal it is paired with
//
// §6 writes an order's state twice, to two different things, and confusing them
// is the mistake this comment exists to prevent. ports.Journal is the durable,
// append-only, hash-chained record of what HAPPENED, in order, and §7.5's
// reconcile-only boot replays it. This is the current-state store the running
// system reads — the projection of that log that answers "where is this order
// now" without a replay.
//
// The order between them is not a preference: journal first, apply second.
// §6 step 6 fsyncs DispatchAuthorized before the network write so that a
// process which dies mid-call leaves evidence, and a repository written first
// would hold state no crash could recover. The methods here are named to make
// the pairing visible at a call site — AppendPlanned/ApplyPlanned,
// AppendState/ApplyState — so that a write to one with no write to the other
// reads as the omission it is.
//
// One pairing is deliberately missing and worth naming rather than leaving to be
// discovered: there is no Journal.AppendFill, so ApplyFill has no counterpart
// yet. Journal is "narrowed to what the dispatch path writes", and the fill path
// is a later slice — one that has to land §5.3's harder requirement anyway
// (applying a fill, advancing the REST recovery cursor and appending journal and
// outbox events happen in ONE transaction), which needs a transaction seam this
// port does not have. See the scope note below.
//
// # This package is what the singlewriter gate polices
//
// Rule 6: "Only oms mutates order/fill/trade/position state, through the
// journal-then-apply path (§6). A lint forbids other contexts from importing
// the OMS write repositories." The gate reads the rule as covering the
// INTERFACE as well as the adapter, because depending on the abstraction is the
// recommended way to depend on anything — a gate that policed only
// infrastructure/driven would forbid the shape nobody writes and permit the
// shape everyone does.
//
// # Scope: orders and their fills. Not Trade, not PositionAllocation
//
// §5.1's aggregate is Trade → Orders[] → Fills[] → PositionAllocation, and two
// of those four have no domain type yet. This port holds the two that do. The
// gate is positional rather than per-type for exactly this reason: a
// TradeWriteRepository added to this package next year is policed the moment it
// is declared, by nobody remembering to police it.
type OrderWriteRepository interface {
	// ApplyPlanned creates the record for a newly planned order, at
	// StatePlanned with Version 1.
	//
	// The state is not a parameter, for the reason Journal.AppendPlanned gives:
	// there is exactly one state an order can enter the system in, and a
	// parameter would invite a caller to name another. §5.3's crash-recovery
	// rules are written against that — "a Planned-only order was never
	// dispatched (no venue order can exist)" — so a record created directly at
	// DispatchAuthorized would tell recovery that a send may have happened when
	// none could have.
	//
	// A ClientOrderID the repository already holds is refused rather than
	// replaced or ignored, on domain.Ledger.Register's argument: both
	// resolutions are recoveries that look like they worked.
	ApplyPlanned(domain.Order) (OrderRecord, error)

	// Load returns the current record.
	//
	// It is a read on a WRITE repository and belongs here: it is the read half
	// of read-modify-write, which is what an OCC version is for. It is not a
	// projection — §5.1 gives those to audit and reconcile, "projections over
	// the journal, never co-owners" — and a context wanting to observe orders
	// gets a projection, not this.
	Load(ids.ClientOrderID) (OrderRecord, error)

	// ApplyState moves the order to a new lifecycle state, refusing the write
	// unless expected matches the record's current Version.
	//
	// The transition MUST be an edge of SM §1: an implementation checks
	// domain.CanTransition and returns its error. That check is here rather than
	// in the caller for the reason Journal states in the negative — the journal
	// is a recorder and refuses to hold a second copy of the edge set, while
	// this port holds the CURRENT state and is therefore the last place that can
	// compare the two. A use case that forgot the check would otherwise store a
	// transition no evidence supports, and §7.5 would replay it as fact.
	ApplyState(id ids.ClientOrderID, expected int64, to domain.OrderState) (OrderRecord, error)

	// ApplyFill admits an execution, reporting whether it changed the record.
	//
	// A duplicate is (record, false, nil) rather than an error, on
	// domain.Ledger.Apply's argument: §4 puts the same execution on two streams
	// and §5.3 has REST recovery re-reading fills the private stream already
	// delivered, so a duplicate is the ORDINARY case several times per fill.
	// The boolean is what tells a caller there is no transaction to open.
	//
	// It never consults State, and that absence is §5.3's requirement rather
	// than an oversight: "a cumulative-execution ledger accepts any deduped fill
	// for a known order regardless of its current state — so a post-cancel or
	// post-SubmissionUnknown late fill still applies."
	ApplyFill(f domain.Fill, expected int64) (OrderRecord, bool, error)
}
