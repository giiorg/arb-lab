package ports

import "github.com/giiorg/arb-lab/internal/oms/domain"

// HedgeGroupOrders lists the order records belonging to one §5.1 hedge group —
// the set §6.7's classifier computes a residual over.
//
// # It is a separate port from OrderWriteRepository, and the separation is the
// # check
//
// The same adapter satisfies both, so this could have been a fifth method on the
// write repository and the wiring would look identical. It is not, because the
// classifier would then hold write authority it must never use: §6.7 is a
// DECISION, and every state change it implies is made by the caller that acts on
// the verdict, through the journal-then-apply path. Given the write repository,
// a classifier could advance an order's state directly from inside the
// classification — no journal record, nothing for §7.5 to replay, and the
// projection ahead of the log it is supposed to be a projection OF.
//
// Handing it a port with one read method makes that unwritable rather than
// discouraged. It is the same move dispatch made when it took a ClientOrderID
// instead of an order, and the same one Resolver made by not holding an
// OrderPlacer: the misuse stops being expressible.
//
// # Why the classifier fetches rather than being handed the records
//
// §6.7's third classification input is the reserved-reduction ledger, which the
// classifier owns, and every mutation of it must produce a fresh verdict — that
// is FM-6 made structural (application.Classifier). A classifier that had to be
// handed the group's records could not recompute on a ledger change without its
// caller supplying them again, which puts the re-run back into the caller's
// discipline, exactly where FM-6 found it missing.
type HedgeGroupOrders interface {
	// OrdersIn returns every order record in the group, in no particular order.
	//
	// An empty group is (nil, nil) rather than an error: a hedge group with no
	// orders is a real state — before the first plan, and after §6 step 9's
	// close — and §6.7's verdict over it is Determined with a zero residual,
	// which is the correct answer rather than a missing one.
	//
	// An implementation MUST refuse a group whose Account is not the one it
	// holds. §5.1's key is (account, venue, symbol) and an order record carries
	// only the venue and the symbol, so the account is the component nothing in
	// the record set can be checked against — see the adapter, where the
	// assumption is named and asserted rather than assumed silently.
	OrdersIn(domain.HedgeGroup) ([]OrderRecord, error)
}
