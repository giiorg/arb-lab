package driven

import "errors"

// The journal adapter's refusals.
//
// # These live here and not in application/ports, unlike ErrOrderUnknown
//
// ports/errors.go states the rule it followed: a sentinel belongs beside the
// interface when a CALLER needs to branch on it, because a use case that had to
// import an adapter package to recognise an error would be refused by
// hexboundary. That test is what puts these three in the other place — no use
// case branches on any of them, and each says why:
//
//   - ErrJournalPathUnset and ErrJournalPragma are OPEN-time. The caller is
//     oms/module.go, which is inside this context and may import this package;
//     a root sees them only as a failed New, which is the shape §3.6 rule 7
//     already gives every wiring failure.
//   - ErrJournalChainCorrupt reaches application/ as a wrapped Append failure,
//     and application's correct response to it is the one it already has for
//     any journal failure: refuse the send (§6 step 6). A sentinel would offer
//     a distinction the dispatch path must not act on differently — "the
//     journal is corrupt" is not a reason to submit an order the crash boundary
//     could not record.
//
// If a later slice does need to branch — §7.5's reconcile-only boot is the
// candidate, since a broken chain there is an operator-clearance matter rather
// than a refusal — the sentinel moves to ports/ WITH the port method that
// reports it. Moving it alone would be the coupling ports/errors.go rejects.
var (
	// ErrJournalPathUnset reports OpenSQLiteJournal called with no path.
	//
	// Refused rather than defaulted, and the rejected default is the
	// instructive one: SQLite reads "" as a private temporary database, so a
	// journal opened with an unset path works — appends succeed, the chain
	// verifies, every test passes — and vanishes when the process exits. That
	// is the zero-value-is-unusable rule meeting a library whose zero value is
	// very usable and completely wrong.
	ErrJournalPathUnset = errors.New("oms journal: no database path configured")

	// ErrJournalPragma reports a database whose durability settings are not the
	// ones §13 requires.
	//
	// It is an OPEN failure rather than a warning because the alternative is a
	// process that trades with a journal that returns before the record is
	// durable. There is no degraded mode worth having here: the journal's only
	// job is to be true after a crash.
	ErrJournalPragma = errors.New("oms journal: durability pragma is not the configured value")

	// ErrJournalChainCorrupt reports a chain head this package cannot extend.
	//
	// Distinct from a failed VERIFICATION, which is now ports.ErrJournalChainBroken
	// — declared beside the port because §7.5's replayer branches on it. This
	// one fires on the narrow case the APPEND path can see for itself: a head
	// row whose hash column is not a 32-byte digest, so there is nothing to link
	// the next record to. Linking to a truncated or padded value anyway would
	// produce a record that verifies against a head no verifier will ever
	// recompute.
	//
	// The two are not merged, and the reason is what each licenses: this one
	// refuses a WRITE, so §6 step 6's caller must not send an order; that one
	// refuses to trust a READ, and its consequence is a boot posture. A single
	// sentinel would make the dispatch path's correct response ("refuse the
	// send") and boot's correct response ("halt, latched, alert") the same
	// branch.
	ErrJournalChainCorrupt = errors.New("oms journal: chain head is not a 32-byte digest")

	// ErrJournalAnchorRange reports an authorization instant outside what Unix
	// nanoseconds can hold — before 1678 or after 2262.
	//
	// It is a claim about the STORED form, which is why it is the adapter's and
	// not the port's: time.Time.UnixNano does not report the overflow, it wraps,
	// and the journal would hold a plausible instant in the wrong century.
	// capture's frame codec refuses the same value for the same reason
	// (ErrRecvTimeRange); the consequence here is sharper, because §5.3's
	// retention gate reads a long-ago anchor as "the venue has purged this by
	// now" — an absence that proves nothing, reported as proof.
	ErrJournalAnchorRange = errors.New("oms journal: authorization instant is not representable in Unix nanoseconds")

	// ErrJournalVisitorMissing reports a Replay with no visitor to hand records
	// to. A nil visitor cannot be treated as "verify only": the walk would
	// return success having applied nothing, which is exactly what a successful
	// replay of an EMPTY journal returns, and §7.5 would resume trading against
	// a projection it never rebuilt.
	ErrJournalVisitorMissing = errors.New("oms journal: replay has no visitor")

	// ErrJournalKindUnknown reports a stored kind column no encoder knows.
	//
	// Refused rather than skipped. A record this build cannot interpret is one
	// whose effect on an order's state it cannot rule out, so skipping would
	// rebuild a projection that is silently missing whatever a later format
	// recorded — and the realistic cause is exactly that: a journal written by a
	// newer binary, read by an older one after a rollback.
	ErrJournalKindUnknown = errors.New("oms journal: record kind is not one this build can replay")

	// ErrJournalDetailMissing reports a journal row whose detail row or column
	// is absent — the record exists and the thing it describes does not.
	//
	// Unreachable through this adapter (both rows are written in one
	// transaction, and foreign_keys is verified ON at Open) and reported anyway,
	// because the store is a file an operator can open with sqlite3. The
	// alternative is reading a NULL as a zero, which produces an order with no
	// side, at no price, in no state — every field legal, nothing true.
	ErrJournalDetailMissing = errors.New("oms journal: record has no detail row for its kind")

	// ErrJournalClientIDUnset reports an AppendState with no order to attach
	// the state to.
	//
	// domain.Order.Validate already refuses an empty ClientOrderID on the
	// planned path, so this covers the method that takes the id on its own.
	// A state record naming no order is unreplayable in the quietest possible
	// way: §7.5 joins journal records to venue truth BY client id, so the
	// record is silently absent from the join rather than reported.
	ErrJournalClientIDUnset = errors.New("oms journal: state record has no client order id")
)
