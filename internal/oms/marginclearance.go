package oms

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
)

// This file is the operator act that ends an Ambiguous margin op's incident,
// composed for the ONE caller that can take it — an offline command, with the
// engine stopped (task #213, record §82).
//
// # Why it is a package function and not a Module method
//
// RecordSpotBaseline is a Module method because minting a baseline means reading
// the venue's wallet: it needs the signed client, the rate-limit governor and
// therefore credentials. This act reads NOTHING. It walks the log the journal
// already holds and appends one record, and building a Module for it would make
// clearing a §9 accounting incident require venue credentials, a live venue and
// the one-spender lock — three things that have nothing to do with the act and
// each of which can independently be unavailable at exactly the moment an
// operator is trying to clear an incident.
//
// # It cannot run against a live engine, and both reasons point the same way
//
// The journal is a single-writer store — SetMaxOpenConns(1) with
// `_txlock=immediate`, so "a second process opening the same file fails fast
// instead of mid-append" (see driven.SQLiteJournal). So this refuses at Open
// while the engine holds the file. And even with the record written, a process
// that already latched boot.OutcomeBlocksOpening does not lift it: nothing here
// reaches into a running process, because the thing that would is core §7.3's
// control API, which does not exist.
//
// The operational shape is therefore: stop the engine, take the act, boot again.
// That is the honest MVP rather than a limitation worked around — an incident
// whose remedy is an accounting reconciliation is not one handled without
// stopping to look, and the ambiguity's posture is stop-opening, so the safety
// ladder was never taken away while the operator did.

// MarginOpClearance is what one reconciliation act did.
type MarginOpClearance struct {
	// Act is the record as it was written, echoed so the command prints what
	// went into the log rather than what was typed at it.
	Act domain.MarginOpReconciliation

	// Remaining names the ambiguities still open AFTER this act.
	//
	// Reported because the alternative is the operator conversation this whole
	// slice exists to prevent: a clearance that succeeds, a boot that still
	// blocks, and nothing anywhere saying why. The checklist holds one outcome
	// per gate rather than one per op (record §81 §1a), so a second open incident
	// produces exactly the same gate result as the first.
	Remaining []domain.MarginOpID

	// ChainSeq is the journal's head after the append — RecordedBaseline's field,
	// for the same use: an operator reporting a §9 incident can name the record
	// rather than describe it.
	ChainSeq uint64
}

// ClearanceDeps is everything the act needs from the deployment's config: where
// §5.1's journal lives, and §5.1's subaccount id.
//
// # A struct rather than two string parameters, and a mutant is why
//
// The first draft took `(journalPath, account string)`. Swapping them compiles,
// and the failure is silent in the worst way available: SQLite CREATES the file
// it is asked to open, so the act would mint an empty journal named after the
// subaccount, find no dispatch record in it, and refuse with "this op was never
// dispatched" — the same message a mistyped op id produces. An operator would be
// told their id was wrong about an incident that is still open.
//
// Named fields make the misuse inexpressible rather than merely detectable,
// which is the repo's standing preference where a signature can carry the claim.
type ClearanceDeps struct {
	// JournalPath is §5.1's write-ahead journal on this host — THE journal the
	// deployment boots against, which is why the caller reads it from the same
	// config the boot uses rather than from a flag.
	JournalPath string

	// Account is §5.2's subaccount id. It is needed only because the replay
	// rebuilds an order projection on the way past, and that projection is keyed
	// by account; nothing about the clearance itself consults it.
	Account string
}

// ReconcileMarginOp records the operator act against the journal deps names.
//
// # The replay runs BEFORE the act and its Recovery is what authorises it
//
// The four refusals — unknown op, still in flight, already settled, already
// reconciled — are answered from the log's own walk rather than from a lookup,
// because there is no lookup: the journal deliberately has no method taking a
// bare subject (see the codec's kind comment), so what the log says about one op
// is what the ordered walk concluded about it.
//
// # It walks the log a SECOND time after the append
//
// Not to verify the write — the append is durable when it returns — but to
// report what is still open, and reading it back is what makes that report about
// the LOG rather than about this function's arithmetic. The alternative is
// subtracting the cleared op from the pre-act list, which is the same answer
// derived from the value that has just been changed underneath it.
func ReconcileMarginOp(deps ClearanceDeps, act domain.MarginOpReconciliation) (MarginOpClearance, error) {
	var zero MarginOpClearance
	if err := act.Validate(); err != nil {
		return zero, err
	}

	journal, err := driven.OpenSQLiteJournal(deps.JournalPath)
	if err != nil {
		return zero, fmt.Errorf("oms: journal: %w", err)
	}
	defer func() { _ = journal.Close() }()

	orders, err := driven.NewMemoryOrderWriteRepository(deps.Account)
	if err != nil {
		return zero, fmt.Errorf("oms: write repository: %w", err)
	}
	replayer, err := application.NewReplayer(journal, orders)
	if err != nil {
		return zero, fmt.Errorf("oms: replayer: %w", err)
	}
	reconciler, err := application.NewMarginOpReconciler(journal)
	if err != nil {
		return zero, fmt.Errorf("oms: margin op reconciler: %w", err)
	}

	before, err := replayer.Rebuild()
	if err != nil {
		return zero, fmt.Errorf("oms: replay: %w", err)
	}
	if err := reconciler.Reconcile(before, act); err != nil {
		return zero, err
	}

	// A second repository for the second walk. Rebuild applies records to the one
	// it holds, so re-running against the first would replay every order record
	// onto a projection that already has them — and the duplicate-state rules
	// that would then fire are about a journal delivering a record twice, not
	// about this function reading one twice.
	verifyOrders, err := driven.NewMemoryOrderWriteRepository(deps.Account)
	if err != nil {
		return zero, fmt.Errorf("oms: write repository: %w", err)
	}
	verifier, err := application.NewReplayer(journal, verifyOrders)
	if err != nil {
		return zero, fmt.Errorf("oms: replayer: %w", err)
	}
	after, err := verifier.Rebuild()
	if err != nil {
		return zero, fmt.Errorf("oms: replay after reconciliation: %w", err)
	}

	return MarginOpClearance{
		Act:       act,
		Remaining: after.AmbiguousMarginOps,
		ChainSeq:  after.LastSequence,
	}, nil
}
