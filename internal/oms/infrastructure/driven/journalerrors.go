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

	// ErrJournalMarkBeyondHead reports HashAt asked about a sequence past the
	// last record — the one answer that means the journal has FEWER records
	// than something else says it had.
	//
	// It is its own sentinel because its one caller, the spot-baseline loader
	// (task #179), must tell it apart from every other read failure: a chain
	// mark beyond the head means the journal was truncated or replaced since
	// the baseline was recorded, which is a named operator refusal, while an
	// ordinary read error is a wiring fact. Collapsed into one error, the
	// loader would report tail truncation — the very condition §7.5 records as
	// undetectable from the chain alone — as a database hiccup.
	ErrJournalMarkBeyondHead = errors.New("oms journal: chain mark names a sequence past the journal's head")

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

	// ErrJournalClientIDUnset reports a record with no order to attach its
	// subject to — an AppendState, an AppendAcked, or either generation leg
	// submission.
	//
	// domain.Order.Validate already refuses an empty ClientOrderID on the
	// planned path, so this covers the methods that take the id on its own.
	// A state record naming no order is unreplayable in the quietest possible
	// way: §7.5 joins journal records to venue truth BY client id, so the
	// record is silently absent from the join rather than reported.
	//
	// The generation legs share it because it is the same claim about the same
	// value — and their stake is the one FM-9 names: a generation recovered
	// without a leg id computes domain.Generation.OwnLegs over a set that still
	// contains its own reduction, so the standing trigger fires on the
	// generation's own machinery.
	ErrJournalClientIDUnset = errors.New("oms journal: record has no client order id")

	// ErrJournalGenerationIDUnset reports a generation record with no subject.
	//
	// It is a SEPARATE sentinel from the client-id one and not a shared
	// "identifier unset", because the two name different id spaces sharing one
	// column — see journalcodec.go on the kind column being that column's
	// namespace. An operator reading "no client order id" on a generation record
	// would go looking for an order, and the whole point of the subject column's
	// design is that a generation is not one.
	ErrJournalGenerationIDUnset = errors.New("oms journal: generation record has no generation id")

	// ErrJournalReservationUnset reports an opening record that could not name
	// the ledger entry its generation reserved.
	//
	// Refused rather than defaulted, and the consequence is the one
	// ports.GenerationJournal states: the reservation is the handle the shared
	// reserved-reduction ledger is keyed by, so a recovered generation that
	// cannot name its own entry can neither amend it, retire it, nor tell it
	// from a sibling's. An entry nobody retires subtracts from every later
	// sizing for the life of the process — silently, and in the direction that
	// under-sizes every subsequent trade.
	ErrJournalReservationUnset = errors.New("oms journal: generation opening record has no reservation id")

	// ErrJournalSchemaUnknown reports a `journal` table this build cannot place:
	// one carrying neither the subject column nor the client_id column it is
	// migrated from, or — impossibly, and therefore all the more worth naming —
	// carrying both.
	//
	// Refused at Open rather than left to fail at the first append. No build of
	// this system produces either shape, so the realistic cause is a path
	// pointing at somebody else's database, and the failure from proceeding is
	// an opaque SQL error on §6 step 6's send path. The both-columns case gets
	// the same refusal for a sharper reason than tidiness: there is no rule for
	// which column the existing records were keyed by, and choosing either would
	// silently re-key a chain that verifies perfectly afterwards.
	ErrJournalSchemaUnknown = errors.New("oms journal: database is not a journal this build can migrate")

	// ErrJournalVenueOrderIDUnset reports an AppendAcked with no id to record.
	//
	// Refused because the absent venue id already has a spelling — no acked
	// record at all — and an empty-string record would be a second one. The
	// consumer makes the stake concrete: gate 3's attribution guards every
	// venue-id comparison on non-emptiness, so an empty id journaled here
	// would replay into a projection field every rule ignores, a record that
	// cost an fsync to say nothing.
	ErrJournalVenueOrderIDUnset = errors.New("oms journal: ack record has no venue order id")
)
