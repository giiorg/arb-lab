package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
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
// Splitting it makes each record type total. It also matches what the two calls
// actually mean: one records a decision that an order EXISTS, the other records
// that it reached a state.
type Journal interface {
	// AppendPlanned durably records the intent to place an order — §6 step 5's
	// "TradePlanned + intents persisted and fsynced to the write-ahead journal
	// before any submission".
	//
	// It records the order at StatePlanned; the state is not a parameter because
	// there is only one state an order can be journaled into for the first time,
	// and a parameter would invite a caller to name another.
	AppendPlanned(domain.Order) error

	// AppendState durably records an order reaching a lifecycle state.
	//
	// It takes the target state only, not the pair. The journal is append-only
	// and replayed in order, so the source state is already in it — recording it
	// again would be a second copy of a fact the log states, and a replay finding
	// the two disagree has no rule for which wins.
	//
	// Legality is the CALLER's to establish, through domain.CanTransition. This
	// port is a recorder, and a recorder that refused illegal transitions would
	// be a second home for SM §1's edge set.
	AppendState(ids.ClientOrderID, domain.OrderState) error
}
