package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
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

	// Fees is what the venue charged for this order's executions, one entry per
	// asset actually charged, sorted by asset. Empty when nothing non-zero was
	// charged.
	//
	// It exists for §7.5's spot balance assertion (task #179): a spot BUY's fee
	// is deducted from the RECEIVED base asset on both venues, so the balance
	// credits Cumulative minus the base-asset fee — an assertion that compared
	// Cumulative alone would report every filled buy as a shortfall of exactly
	// its fee. The asset is the venue's word per fill (a BNB-discount fee on a
	// BTCUSDT trade debits BNB), which is why this is a per-asset list and not
	// one number in the order's base.
	Fees []fixedpoint.Money

	// AuthorizedAt is the instant this system committed to the single Place,
	// expressed in the VENUE's own clock frame. It is unset until the order
	// reaches StateDispatchAuthorized.
	//
	// # It is the anchor BOTH time bounds of §5.3's stable-not-found rule are
	// # measured from, and this system used to record it nowhere
	//
	// The rule has two clocks in it. SM §2 step 1 will not accept an absence
	// until a still-in-flight send can no longer land ("a DispatchAuthorized
	// send may land AFTER the query"), which is an instant plus a propagation
	// bound. §5.3's retention paragraph will not accept one from a surface that
	// has purged the row, which is an instant plus the venue's published reach.
	// Both count from the same moment — this one — and before slice D nothing in
	// the OMS held it: not domain.Order (the immutable plan-time half, which
	// does not know when dispatch will happen), not this record, and not the
	// journal, whose codec REJECTED a timestamp in slice A on the ground that
	// "nothing reads it".
	//
	// # Why a clock.ServerTime and not a time.Time
	//
	// Gate 4's comparison is against the venue's own arithmetic — Binance's
	// cliff is literally "created time + 3 days < current time", evaluated by
	// Binance on Binance's clock. A local instant compared against a venue
	// retention is this repo's recomputed-check-must-share-the-anchor defect:
	// wrong by a small stable offset, and reading as correct. ServerTime carries
	// the uncertainty band, and Passed returns a three-valued Certainty, so a
	// bound that cannot be told from the band refuses instead of guessing.
	//
	// An UNSET value is therefore meaningful and safe: it answers Uncertain to
	// every boundary, so a resolver refuses to prove absence rather than proving
	// it from an instant nobody recorded. Two different causes produce it and
	// both want the same refusal — an order not yet authorized, and an order
	// whose anchor died with the process, since this projection is in memory.
	//
	// # The journal does not carry it yet, and that is the gap slice F closes
	//
	// [SPEC EDIT LANDED — v0.9.34] §5.3 now carries the anchor and this
	// consequence, because both belong to recovery rather than to this struct: with
	// the anchor held only in a memory projection, a crash-recovered order at
	// StateDispatchAuthorized has none, so StateAbsentAfterFinality is
	// UNREACHABLE for exactly the order §5.3 says recovery resolves that way.
	// The order then sits in a state ContributesPossibleDelta answers true for,
	// its hedge group is Ambiguous for ever, and new trading is held — not a
	// wrong number, a system that halts itself with every light green. §7.5's
	// replayer is the construct that must re-derive it (slice F).
	AuthorizedAt clock.ServerTime

	// VenueOrderID is the venue's own identifier for this order, exactly as
	// the single Place's ack carried it (Ack.VenueOrderID). Empty until an ack
	// with an id was applied, and empty for ever on the order whose ack was
	// lost — which is not a degenerate case but the one §6.7 exists for.
	//
	// # Its emptiness is meaningful the way AuthorizedAt's unset is
	//
	// An empty value refuses rather than misleads: no attribution rule may
	// compare against it (attribute guards every venue-id arm on non-empty),
	// so the order it belongs to simply contributes nothing to the rule-out
	// set. The asymmetry with AuthorizedAt is what earns the plain string: an
	// anchor nobody recorded reads as "authorized at the epoch" unless the
	// type refuses, while an id nobody recorded reads as nothing at all.
	//
	// What a NON-empty value licenses rests on a banked venue fact rather than
	// on this struct: Binance's orderId is "self-increment for each specific
	// symbol" (Profiles §8, dated source), so an id observed at ack time still
	// names the same venue order when a later sweep meets it — non-reuse
	// across time, which is strictly more than the at-query-time uniqueness
	// the live paths need. Both claims are per SYMBOL, which is why every
	// consumer of this field scopes to the full instrument.
	VenueOrderID string

	// Version is §5.1's OCC version, starting at 1 and incremented by every
	// write that CHANGES the record.
	//
	// [SPEC EDIT LANDED — v0.9.11] §5.1 said only "a Version int64 (OCC)" and
	// stated neither half of what follows. Both were found by the slice-closing
	// READING PASS rather than by the marker grep — the v0.9.8 shape, since a
	// claim the code merely relies on is never marked — and both survive §15b's
	// re-judgement test: a reimplementation can get each wrong in the unsafe
	// direction while satisfying the sentence as written. §5.1 now states the
	// base and the no-op rule, with the failure direction of each.
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
	//
	// It MUST refuse an order that fails domain.Order.Validate, and that is a
	// requirement on implementations rather than a description of one — the
	// half of this port a compile-time assertion cannot check, stated here the
	// way Journal states durability and ErrVersionConflict states its sentinel.
	//
	// It is load-bearing downstream: this repository is the only source of
	// orders the dispatch path sends, so Dispatch reads well-formedness from
	// the fact that a record EXISTS rather than re-deriving it. A Validate at
	// the dispatch boundary would be a branch no test could reach, which is
	// this repo's unreachable-sentinel shape; the check therefore has to be
	// genuinely here, and an implementation that skipped it would remove it
	// from the system rather than move it.
	ApplyPlanned(domain.Order) (OrderRecord, error)

	// Load returns the current record.
	//
	// It is a read on a WRITE repository and belongs here: it is the read half
	// of read-modify-write, which is what an OCC version is for. It is not a
	// projection — §5.1 gives those to audit and reconcile, "projections over
	// the journal, never co-owners" — and a context wanting to observe orders
	// gets a projection, not this.
	Load(ids.ClientOrderID) (OrderRecord, error)

	// ApplyAuthorized moves the order to StateDispatchAuthorized AND records the
	// instant §5.3's stable-not-found rule measures its two bounds from.
	//
	// # It is a separate method so that the anchorless authorization is
	// # INEXPRESSIBLE, not merely discouraged
	//
	// The alternative was an optional instant on ApplyState, and it is refused on
	// Journal's own argument for having two Append methods: a field that is
	// meaningful for one target state and meaningless for eleven has a zero value
	// that is legal and says nothing. Here that zero value is worse than
	// meaningless — it is an order authorized at the epoch, which the retention
	// bound then judges long purged.
	//
	// Split out, every caller that commits to a Place must produce an instant to
	// do it, and ApplyState refuses the state outright. A misuse that a comment
	// would have had to forbid is now a method that does not exist.
	//
	// at MUST be in the venue's clock frame — see OrderRecord.AuthorizedAt for
	// why the frame is load-bearing rather than a formality. An implementation
	// refuses an unset ServerTime: the whole purpose of the field is to be
	// comparable against a venue retention, and an unset one answers Uncertain to
	// every boundary, so accepting it here would durably record an authorization
	// whose order can never be resolved.
	ApplyAuthorized(id ids.ClientOrderID, expected int64, at clock.ServerTime) (OrderRecord, error)

	// ApplyAck records the venue's identifier for an order the venue has
	// acknowledged — OrderRecord.VenueOrderID's one producer.
	//
	// An implementation MUST refuse: an empty venueOrderID (the absent id has
	// one spelling, an empty field, and this method is not it); an order whose
	// state answers false to MayExistAtVenue (a Planned order has no venue
	// side to have acked it, so the call describes a history §5.3 says cannot
	// have happened); and — with ErrVenueOrderIDConflict — an id that
	// CONTRADICTS the store: this order already holding a different id, or
	// another order on the same instrument already holding this one. Both
	// contradictions poison the same downstream fact, the per-instrument
	// venue-id index the resolver rules sweep rows out against, and a store
	// that admitted either would hand that index two owners for one key — at
	// which point a Go map build picks a winner by iteration order, which is
	// this repo's recorded coin-flip-in-the-code shape with a terminal state
	// on the losing side.
	//
	// Re-recording the id the order already holds is a NO-OP — (record, nil),
	// version unchanged, per Version's "every write that CHANGES the record" —
	// because §7.5's replay may legitimately meet the same acked record twice.
	ApplyAck(id ids.ClientOrderID, expected int64, venueOrderID string) (OrderRecord, error)

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
	//
	// It MUST refuse StateDispatchAuthorized with ErrAuthorizationNeedsAnchor,
	// whatever the current state and version. That target has its own method
	// above, and leaving it reachable here would leave the shape this port is
	// trying to eliminate: an order at the crash boundary with no instant to
	// resolve it from. The refusal is on the TARGET alone, deliberately — it
	// fires before the version and legality checks, because a caller that
	// reached for the wrong method has made a mistake that a version conflict
	// would misdiagnose as a race.
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
