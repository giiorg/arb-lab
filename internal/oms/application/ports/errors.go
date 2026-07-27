package ports

import "errors"

// The refusals a driven port makes, declared beside the interfaces rather than
// in the adapters that return them.
//
// # This placement is forced, and the force is §3.6 rule 1
//
// A sentinel is only useful if a caller can branch on it, and the callers here
// are use cases in application/. Declaring ErrVersionConflict in
// infrastructure/driven would mean every caller that wanted to retry an OCC
// conflict had to import an adapter package — which hexboundary refuses — or
// match on the message text, which is the same coupling with nothing checking
// it.
//
// So the error is part of the interface, in the literal sense: an
// implementation that reports a version conflict some other way does not
// satisfy this port, whatever its method set says. That is the same claim
// Journal's doc comment makes about durability, and it has the same consequence
// — the compile-time assertion §3.6 rule 2 requires checks the signature, and
// these two sentences are the part it cannot.
var (
	// ErrOrderUnknown reports a ClientOrderID the repository holds no record
	// for.
	//
	// Distinct from domain.ErrFillUnknownOrder, which the fill ledger raises for
	// the same underlying condition on the fill path. The two are not merged
	// because they answer different questions: this one says the WRITE
	// repository never created the order — a caller that skipped Create, or a
	// process that has not replayed the journal yet (§7.5) — while the domain
	// sentinel is about an execution naming an order the ledger does not hold.
	// A reader chasing the first goes to the write path; a reader chasing the
	// second goes to the venue's execution stream.
	ErrOrderUnknown = errors.New("oms: write repository holds no record for this client order id")

	// ErrVersionConflict reports a write whose expected version is not the
	// record's current one — §5.1's OCC Version, refusing a lost update.
	//
	// # Why the check exists at all when §3.6 rule 6 already makes oms the only
	// # writer
	//
	// Rule 6 is about which CONTEXT may write; it says nothing about how many
	// goroutines inside this one may. The concurrency §6 actually has is real:
	// the private order stream, the executions stream, the REST recovery sweep
	// and the §6.7 finality timers all produce writes for the same order, and
	// §5.3 puts fills and lifecycle events on deliberately independent paths.
	//
	// The failure this refuses is the classic read-modify-write loss, and its
	// shape here is what makes it worth a sentinel rather than a mutex alone: two
	// writers read an order at Acknowledged, one advances it to Canceled and the
	// other to PartiallyFilled, and both transitions are LEGAL under
	// domain.CanTransition. Nothing detects the collision from the state set;
	// only the version does. What is lost is not a field but an event — the
	// fill's, or the cancel's — and §6.7 then classifies a hedge group from a
	// record that never saw one of them.
	//
	// It is a caller-recoverable error, and the only correct recovery is to
	// re-read and re-decide: a retry with the same intent against the new
	// version is right for a fill, and wrong for a cancel that raced a fill,
	// which is why this port does not retry for anyone.
	ErrVersionConflict = errors.New("oms: write repository record has moved since it was read")
)
