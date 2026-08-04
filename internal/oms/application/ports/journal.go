package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Journal is §5.1's durable, append-only, hash-chained write-ahead journal,
// narrowed to what the dispatch path writes.
//
// # Every method returns only when the record is DURABLE, and there is
// # deliberately no Sync
//
// §6 step 5 says "persist **and fsync** … before any submission" and §6 step 6
// says "immediately before the single Place, **fsync DispatchAuthorized**". The
// obvious API for that is an Append plus a Sync a caller pairs it with, and it is
// the wrong shape here for one reason: forgetting the second call produces a
// journal that is fast, correct-looking and NOT durable. Nothing fails, the tests
// pass, and the defect surfaces once — during the crash the fsync existed for,
// when recovery finds no record of an order the venue is holding.
//
// So durability is not a second call; it is the contract of the first, stated
// once here and binding on every method. That is this codebase's standing rule
// about defaults with two directions, applied to a port's shape rather than to a
// field: the version that is right when somebody forgets.
//
// The consequence for adapters is deliberate — an implementation that batches or
// buffers does not satisfy this interface, whatever its method set says. The
// compile-time assertion §3.6 rule 2 requires checks the signature; this comment
// is the part it cannot.
//
// # Two methods rather than one Append(Record), and the reason is the zero value
//
// The single-method form needs a record type carrying an order's full intent on
// the plan entry and nothing but a state on the others — an optional field, whose
// zero value is then a legal value with no meaning. The alternative, restating
// the whole order on every record, is worse: two records for one ClientOrderID
// could disagree about its quantity, and the journal is what §7.5 replays to
// decide what an in-flight order was.
//
// Splitting it makes each record type total. It also matches what the calls
// actually mean: one records a decision that an order EXISTS, one records that
// it reached a state, and one records the single state that carries an instant
// with it.
//
// # THREE methods now, and the third is the write repository's split arriving
// # at the log
//
// [SPEC EDIT LANDED — v0.9.36] Slice D made the anchorless authorization
// inexpressible in the PROJECTION: OrderWriteRepository.ApplyAuthorized takes
// §5.3's anchor, and ApplyState refuses StateDispatchAuthorized outright, so no
// caller can advance an order to the crash boundary without producing the
// instant recovery measures its two bounds from. The journal kept the
// anchorless spelling as its ONLY one for a slice, and the journal is the store
// §7.5 replays — so the projection could not hold an unanchored authorization
// and the log could not hold an anchored one. A crash-recovered order therefore
// arrived at StateDispatchAuthorized with no anchor however carefully the live
// path had stamped it, which makes StateAbsentAfterFinality unreachable for
// exactly the order §5.3 says recovery resolves that way.
//
// This is the recorded mirror-rule shape with its second axis: the two stores
// are mirrors across journal-and-projection, not across two venues or two
// directions of travel, and the fix applied to one of them stopped there.
type Journal interface {
	// AppendPlanned durably records the intent to place an order — §6 step 5's
	// "TradePlanned + intents persisted and fsynced to the write-ahead journal
	// before any submission".
	//
	// It records the order at StatePlanned; the state is not a parameter because
	// there is only one state an order can be journaled into for the first time,
	// and a parameter would invite a caller to name another.
	AppendPlanned(domain.Order) error

	// AppendAuthorized durably records §6 step 6's commitment to call Place
	// exactly once — StateDispatchAuthorized, with the instant §5.3's
	// stable-not-found rule measures both of its bounds from.
	//
	// at MUST be in the VENUE's clock frame, and an implementation refuses an
	// unset one. ports.OrderRecord.AuthorizedAt argues the frame in full: gate
	// 4's comparison is against the venue's own arithmetic, so a local instant is
	// wrong by a small stable offset and reads as correct.
	//
	// It is a separate method for OrderWriteRepository.ApplyAuthorized's reason,
	// one store over: an optional instant on AppendState has a zero value that is
	// legal for eleven states and catastrophic for the twelfth — an order
	// authorized at the epoch, which the retention bound judges long purged. Here
	// the misuse is not discouraged; it is a method that does not exist.
	AppendAuthorized(id ids.ClientOrderID, at clock.ServerTime) error

	// AppendAcked durably records the venue's own identifier for an order —
	// the id the single Place's ack carried, which on Binance is the ONLY join
	// key between a swept execution and one of our orders (Ack.VenueOrderID).
	//
	// # A venue that supplied no id gets NO record, and that is the method's
	// # whole shape
	//
	// The absent id has exactly one spelling: the record does not exist. An
	// optional column on the state record would give absence a second spelling —
	// an empty string whose zero value is legal and says nothing — which is the
	// same argument that split AppendAuthorized out, one field over. So both
	// arguments are refused empty: an id-less record could only ever mean "the
	// venue said nothing", and the journal already says that by silence.
	//
	// What the record buys is stated at the consumer: §5.3's gate 3 rules a
	// swept row OUT when its venue order id belongs to another of OUR orders,
	// and the persisted ack is where "our orders' venue ids" comes from for an
	// order whose own ack was lost — which is the one scenario §6.7 runs for.
	AppendAcked(id ids.ClientOrderID, venueOrderID string) error

	// AppendState durably records an order reaching a lifecycle state.
	//
	// It takes the target state only, not the pair. The journal is append-only
	// and replayed in order, so the source state is already in it — recording it
	// again would be a second copy of a fact the log states, and a replay finding
	// the two disagree has no rule for which wins.
	//
	// It MUST refuse StateDispatchAuthorized, on the TARGET alone, exactly as
	// OrderWriteRepository.ApplyState does and with the same sentinel
	// (ErrAuthorizationNeedsAnchor). Leaving it reachable here would leave the
	// shape the method above exists to eliminate — a durable authorization with
	// no instant to resolve it from — in the one store recovery reads.
	//
	// Legality is otherwise the CALLER's to establish, through
	// domain.CanTransition. This port is a recorder, and a recorder that refused
	// illegal transitions would be a second home for SM §1's edge set. The
	// refusal above is not an exception to that: it is about which METHOD carries
	// a target, not about which transitions into it are legal.
	AppendState(ids.ClientOrderID, domain.OrderState) error
}
