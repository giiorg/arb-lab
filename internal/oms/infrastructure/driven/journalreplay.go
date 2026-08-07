package driven

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Every generation record's subject is converted through domain.GenerationID at
// the point of use rather than once at the top of a method, and the repetition
// is deliberate. The subject column holds two id spaces discriminated by the
// kind (see journalcodec.go), so the conversion IS the claim about which space
// this row belongs to — hoisted into a variable it becomes a fact established
// far from the branch that justifies it, which is how a generation record ends
// up handed to a visitor as an order.

// §3.6 rule 2's compile-time assertion for the read half. A second line rather
// than a wider one on the write half's assertion, because the two ports are
// deliberately separate — see ports/journalreplay.go — and an assertion naming
// only ports.Journal would go on compiling if Replay were deleted.
var _ ports.JournalReplay = (*SQLiteJournal)(nil)

// The journal's read half: §7.5's replay, and the first code in this system
// that ever recomputes a chain link.
//
// # Verification here is not a second opinion; it is the only one
//
// Every Append writes a hash and links it to its predecessor, and until this
// file existed nothing outside a test recomputed either. A hash chain nobody
// verifies is a column, and §5.1's "tamper-evident" was a property of the
// format rather than of the system. The claim becomes real at exactly one
// place: the boot that reads the log back and decides what this process did.
//
// # One query with three LEFT JOINs, and the shape is forced
//
// The obvious implementation walks the journal table and looks up each detail
// row as it goes. It DEADLOCKS here, silently and permanently: SetMaxOpenConns(1)
// gives this pool one connection, an open sql.Rows holds it, and the nested
// query waits for a connection that cannot be released until the walk it is
// inside of finishes. A test would report TIMEOUT!!, which names no defect —
// this repo's own note that a test which hangs is not a test that fails,
// arriving in production code.
//
// The alternative to the join is reading every record into memory first, which
// works and gives up the one property a write-ahead log must not give up: the
// walk is O(1) in memory whatever the journal's length. A recovery that needs
// the whole journal resident is a recovery that stops working on the day the
// journal is big, which is the day it matters most.
//
// The cost of the join is the nullable scan below — a planned record's columns
// are NULL on a state row and vice versa — which is the "optional field whose
// zero value is legal" shape ports.Journal refuses in its API. It is admitted
// HERE and nowhere else, because the kind column decides which set is
// meaningful and a NULL where the kind requires a value is reported rather than
// read as a zero (ErrJournalDetailMissing).
//
// # The generation joins widen it and change nothing about the shape
//
// Five more LEFT JOINs, all on the same indexed primary key, and the walk stays
// O(1) in memory. What is worth naming is what did NOT change: the generation
// records are read by the SAME walk, in the SAME sequence order, because they
// live in the same chain. That is the property one chain was chosen for — a
// replay observes a generation's authorization and its spot leg's journaled
// intent in the order they were written, so FLAG-3's ordering clause is checked
// by the walk rather than asserted by the layer above it.
const replayQuery = `
SELECT j.seq, j.kind, j.subject, j.prev_hash, j.hash,
       p.venue, p.product, p.symbol, p.contract, p.side, p.order_type, p.time_in_force,
       p.qty_asset, p.qty_mantissa, p.price_base, p.price_quote, p.price_mantissa, p.reduce_only,
       s.state,
       a.instant_unix_nano, a.uncertainty_nano,
       k.venue_order_id,
       go_.account, go_.venue, go_.symbol, go_.reservation, go_.target_asset, go_.target_mantissa,
       gl.client_id,
       ga.size_asset, ga.size_mantissa,
       gs.path,
       gp.phase
FROM journal j
LEFT JOIN journal_planned                p    ON p.seq    = j.seq
LEFT JOIN journal_state                  s    ON s.seq    = j.seq
LEFT JOIN journal_authorized             a    ON a.seq    = j.seq
LEFT JOIN journal_acked                  k    ON k.seq    = j.seq
LEFT JOIN journal_generation_opened      go_  ON go_.seq  = j.seq
LEFT JOIN journal_generation_leg         gl   ON gl.seq   = j.seq
LEFT JOIN journal_generation_authorized  ga   ON ga.seq   = j.seq
LEFT JOIN journal_generation_superseded  gs   ON gs.seq   = j.seq
LEFT JOIN journal_generation_phase       gp   ON gp.seq   = j.seq
ORDER BY j.seq`

// replayRow is one journal row with every detail table's columns attached.
//
// The fields are grouped by which kind populates them, and the nullable types
// are the join's cost rather than a design choice — see replayQuery.
// The subject field is named for the column and NOT for either id space it can
// hold, which is the same discipline journal.go's appendRecord follows at the
// write end. A field called clientID carrying a generation id half the time is
// how a `ids.ClientOrderID(r.clientID)` conversion gets written on a generation
// record and compiles.
type replayRow struct {
	seq      int64
	kind     string
	subject  string
	prevHash []byte
	hash     []byte

	venue, product, symbol, contract       sql.NullString
	side, orderType, timeInForce           sql.NullString
	qtyAsset, priceBase, priceQuote        sql.NullString
	qtyMantissa, priceMantissa, reduceOnly sql.NullInt64

	state sql.NullString

	instantUnixNano, uncertaintyNano sql.NullInt64

	venueOrderID sql.NullString

	genAccount, genVenue, genSymbol, genReservation sql.NullString
	genTargetAsset                                  sql.NullString
	genTargetMantissa                               sql.NullInt64

	genLegClientID sql.NullString

	genSizeAsset    sql.NullString
	genSizeMantissa sql.NullInt64

	genPath  sql.NullString
	genPhase sql.NullString
}

// Replay is ports.JournalReplay: walk the log in sequence order, verifying the
// chain, and hand each record to the visitor.
//
// # The chain is verified BEFORE the record is handed over, per record
//
// Not in a pass of its own, and not after the walk. A verify-then-replay pair
// reads the journal twice and leaves a window between them; a replay-then-verify
// hands a use case records it will be told to distrust, after it has already
// rebuilt state from them. Per record, in order, the visitor never sees a record
// whose link did not check out — so a caller that stops on the first error stops
// with a state built entirely from verified records.
//
// # What it does NOT do is decide what a broken chain means
//
// It returns ports.ErrJournalChainBroken and stops. The posture that follows —
// halted, latched, and why neither of the two obvious readings fits — is §7.5's
// and lives with the caller (application.Replayer, boot.OutcomeEvidenceUnusable).
func (j *SQLiteJournal) Replay(v ports.JournalVisitor) error {
	if v == nil {
		return ErrJournalVisitorMissing
	}

	rows, err := j.db.Query(replayQuery)
	if err != nil {
		return fmt.Errorf("oms journal: replay: %w", err)
	}
	defer func() { _ = rows.Close() }()

	prev := genesisHash()
	var expected int64 = 1

	for rows.Next() {
		var r replayRow
		if err := rows.Scan(
			&r.seq, &r.kind, &r.subject, &r.prevHash, &r.hash,
			&r.venue, &r.product, &r.symbol, &r.contract,
			&r.side, &r.orderType, &r.timeInForce,
			&r.qtyAsset, &r.qtyMantissa, &r.priceBase, &r.priceQuote, &r.priceMantissa, &r.reduceOnly,
			&r.state,
			&r.instantUnixNano, &r.uncertaintyNano,
			&r.venueOrderID,
			&r.genAccount, &r.genVenue, &r.genSymbol, &r.genReservation,
			&r.genTargetAsset, &r.genTargetMantissa,
			&r.genLegClientID,
			&r.genSizeAsset, &r.genSizeMantissa,
			&r.genPath,
			&r.genPhase,
		); err != nil {
			return fmt.Errorf("oms journal: replay: scan: %w", err)
		}

		body, err := replayPreimage(r)
		if err != nil {
			return err
		}
		if err := verifyLink(r, expected, prev, body); err != nil {
			return err
		}

		if err := visit(v, r); err != nil {
			return err
		}

		copy(prev[:], r.hash)
		expected++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("oms journal: replay: %w", err)
	}
	return nil
}

// verifyLink checks the three things that make the chain a chain, and the first
// is the one a reader most often thinks the database already guarantees.
//
// The sequence must be dense from 1, because a PRIMARY KEY is unique and not
// contiguous: a journal missing record 7 has a perfectly valid key column, and
// every remaining link verifies against ITS OWN predecessor's hash. Only the
// count says anything, and only if something is counting. Deleting a whole
// record is exactly what the append-only triggers make hard and exactly what an
// attacker with the file would try first.
//
// prev_hash must be the predecessor's hash — the link — and the record's own
// hash must be what the stored columns rebuild to. Those two are separate
// failures: the first is a record moved or removed, the second is a record
// edited in place.
func verifyLink(r replayRow, expectedSeq int64, prev [32]byte, body []byte) error {
	if r.seq != expectedSeq {
		return fmt.Errorf("oms journal: replay: record at position %d has sequence %d: %w",
			expectedSeq, r.seq, ports.ErrJournalChainBroken)
	}
	// There is deliberately no length check on the stored hash, though the
	// append path has one on the chain HEAD (ErrJournalChainCorrupt). It would
	// be dead here: a hash of any other length cannot equal the 32 bytes the
	// comparison below recomputes, so the record is refused either way and no
	// test could tell the two refusals apart. A branch nothing can reach reads
	// as a handled case — this repo's recorded unreachable-sentinel shape — and
	// the caller that DOES need the length (the copy into prev, back in Replay)
	// runs only after that comparison has proved it.
	if string(r.prevHash) != string(prev[:]) {
		return fmt.Errorf("oms journal: replay: record %d links to %x, its predecessor hashes to %x: %w",
			r.seq, r.prevHash, prev, ports.ErrJournalChainBroken)
	}
	if want := chainHash(uint64(r.seq), prev, body); string(r.hash) != string(want[:]) {
		return fmt.Errorf("oms journal: replay: record %d stores hash %x, its columns hash to %x: %w",
			r.seq, r.hash, want, ports.ErrJournalChainBroken)
	}
	return nil
}

// replayPreimage rebuilds a record's hashed bytes from its STORED COLUMNS.
//
// It calls the same encoders the append path does, and that is the deliberate
// half of this design rather than a shortcut: what makes the verification
// meaningful is that the preimage is rebuilt from typed columns — a venue, a
// symbol, a mantissa — rather than from a blob the writer handed us. An
// implementation that stored the preimage and re-hashed it would verify its own
// output perfectly and detect nothing. The independent SECOND implementation
// stays where it belongs, in the test oracle, which shares no code with either.
func replayPreimage(r replayRow) ([]byte, error) {
	switch r.kind {
	case kindPlanned:
		o, err := replayOrder(r)
		if err != nil {
			return nil, err
		}
		return encodePlanned(o), nil

	case kindState:
		if !r.state.Valid {
			return nil, detailMissing(r, "journal_state")
		}
		return encodeState(ids.ClientOrderID(r.subject), domain.OrderState(r.state.String)), nil

	case kindAuthorized:
		at, err := replayAnchor(r)
		if err != nil {
			return nil, err
		}
		return encodeAuthorized(ids.ClientOrderID(r.subject), at), nil

	case kindAcked:
		if !r.venueOrderID.Valid {
			return nil, detailMissing(r, "journal_acked")
		}
		return encodeAcked(ids.ClientOrderID(r.subject), r.venueOrderID.String), nil

	case kindGenerationOpened:
		group, reservation, target, err := replayGenerationOpening(r)
		if err != nil {
			return nil, err
		}
		return encodeGenerationOpened(domain.GenerationID(r.subject), group, reservation, target), nil

	case kindGenerationPerpSubmitted:
		if !r.genLegClientID.Valid {
			return nil, detailMissing(r, "journal_generation_leg")
		}
		return encodeGenerationPerpSubmitted(
			domain.GenerationID(r.subject), ids.ClientOrderID(r.genLegClientID.String)), nil

	case kindGenerationAuthorized:
		size, err := replayGenerationSize(r)
		if err != nil {
			return nil, err
		}
		return encodeGenerationAuthorized(domain.GenerationID(r.subject), size), nil

	case kindGenerationSpotSubmitted:
		if !r.genLegClientID.Valid {
			return nil, detailMissing(r, "journal_generation_leg")
		}
		return encodeGenerationSpotSubmitted(
			domain.GenerationID(r.subject), ids.ClientOrderID(r.genLegClientID.String)), nil

	case kindGenerationSuperseded:
		if !r.genPath.Valid {
			return nil, detailMissing(r, "journal_generation_superseded")
		}
		return encodeGenerationSuperseded(
			domain.GenerationID(r.subject), domain.SupersessionPath(r.genPath.String)), nil

	case kindGenerationPhase:
		if !r.genPhase.Valid {
			return nil, detailMissing(r, "journal_generation_phase")
		}
		return encodeGenerationPhase(
			domain.GenerationID(r.subject), domain.GenerationPhase(r.genPhase.String)), nil
	}

	// A kind column holding something no encoder knows. It is refused rather
	// than skipped, and the direction is §7.5's: a record this process cannot
	// interpret is a record it cannot rule out having changed its state, so
	// ignoring it would rebuild a projection that is silently missing whatever
	// the newer format recorded. The realistic cause is a downgrade — a journal
	// written by a later build, read by an earlier one.
	return nil, fmt.Errorf("oms journal: replay: record %d has kind %q: %w",
		r.seq, r.kind, ErrJournalKindUnknown)
}

// replayOrder rebuilds the domain.Order a planned record holds.
//
// It VALIDATES, and the validation is AppendPlanned's own claim arriving where
// that method said it would: "a record this package cannot rebuild into a valid
// domain.Order is a record that fails at §7.5's replay". The failure is
// deliberately here rather than at the visitor — an invalid order cannot be
// encoded into the preimage the chain covers either, so the two would fail in
// the same breath with the vaguer diagnosis.
func replayOrder(r replayRow) (domain.Order, error) {
	for _, c := range []struct {
		name string
		set  bool
	}{
		{"venue", r.venue.Valid}, {"product", r.product.Valid},
		{"symbol", r.symbol.Valid}, {"contract", r.contract.Valid},
		{"side", r.side.Valid}, {"order_type", r.orderType.Valid},
		{"time_in_force", r.timeInForce.Valid},
		{"qty_asset", r.qtyAsset.Valid}, {"qty_mantissa", r.qtyMantissa.Valid},
		{"price_base", r.priceBase.Valid}, {"price_quote", r.priceQuote.Valid},
		{"price_mantissa", r.priceMantissa.Valid}, {"reduce_only", r.reduceOnly.Valid},
	} {
		if !c.set {
			return domain.Order{}, detailMissing(r, "journal_planned."+c.name)
		}
	}

	qty, err := fixedpoint.QuantityFromMantissa(fixedpoint.Asset(r.qtyAsset.String), r.qtyMantissa.Int64)
	if err != nil {
		return domain.Order{}, fmt.Errorf("oms journal: replay: record %d quantity: %w", r.seq, err)
	}
	price, err := fixedpoint.PriceFromMantissa(
		fixedpoint.Asset(r.priceBase.String), fixedpoint.Asset(r.priceQuote.String), r.priceMantissa.Int64)
	if err != nil {
		return domain.Order{}, fmt.Errorf("oms journal: replay: record %d price: %w", r.seq, err)
	}

	o := domain.Order{
		ClientOrderID: ids.ClientOrderID(r.subject),
		Instrument: ids.VenueInstrument{
			Venue:    ids.Venue(r.venue.String),
			Product:  ids.ProductCategory(r.product.String),
			Symbol:   r.symbol.String,
			Contract: ids.ContractType(r.contract.String),
		},
		Side:        domain.Side(r.side.String),
		Type:        domain.OrderType(r.orderType.String),
		TimeInForce: domain.TimeInForce(r.timeInForce.String),
		Quantity:    qty,
		LimitPrice:  price,
		ReduceOnly:  r.reduceOnly.Int64 != 0,
	}
	if err := o.Validate(); err != nil {
		return domain.Order{}, fmt.Errorf("oms journal: replay: record %d: %w", r.seq, err)
	}
	return o, nil
}

// replayGenerationOpening rebuilds the three values SM §7's `Planned` record
// fixes for a generation's lifetime.
//
// It VALIDATES the hedge group, on replayOrder's rule and for a consequence that
// is worse here. A group with an empty account passes every join in this file and
// merges every subaccount into one — so a recovered generation would be judged,
// sized and superseded against another account's exposure, with §6.7's
// determined branch sizing compensating trades from the union. domain's own
// Validate is the one place that rule lives; calling it is what keeps this from
// being a second copy.
//
// The target is rebuilt through QuantityFromMantissa rather than assembled, so
// the asset tag is checked by fixedpoint rather than by this package. An
// asset-less target is exactly what AppendGenerationOpened refused at the write
// end, and the symmetry is the point: the refusal exists at both ends because
// the store is a file an operator can edit, and the write-side check cannot see
// what a text editor put there.
func replayGenerationOpening(r replayRow) (domain.HedgeGroup, domain.ReservationID, fixedpoint.Quantity, error) {
	for _, c := range []struct {
		name string
		set  bool
	}{
		{"account", r.genAccount.Valid}, {"venue", r.genVenue.Valid}, {"symbol", r.genSymbol.Valid},
		{"reservation", r.genReservation.Valid},
		{"target_asset", r.genTargetAsset.Valid}, {"target_mantissa", r.genTargetMantissa.Valid},
	} {
		if !c.set {
			return domain.HedgeGroup{}, "", fixedpoint.Quantity{},
				detailMissing(r, "journal_generation_opened."+c.name)
		}
	}

	group := domain.HedgeGroup{
		Account: r.genAccount.String,
		Venue:   ids.Venue(r.genVenue.String),
		Symbol:  r.genSymbol.String,
	}
	if err := group.Validate(); err != nil {
		return domain.HedgeGroup{}, "", fixedpoint.Quantity{},
			fmt.Errorf("oms journal: replay: record %d group: %w", r.seq, err)
	}

	// The reservation is checked HERE rather than by a Validate on its type,
	// because domain.ReservationID is a bare string with no constructor — it is
	// a venue-agnostic opaque handle by design. An empty one replays into a
	// generation that cannot retire its own ledger entry, which is the
	// permanent-subtraction failure ErrJournalReservationUnset names.
	if r.genReservation.String == "" {
		return domain.HedgeGroup{}, "", fixedpoint.Quantity{},
			fmt.Errorf("oms journal: replay: record %d: %w", r.seq, ErrJournalReservationUnset)
	}

	target, err := fixedpoint.QuantityFromMantissa(
		fixedpoint.Asset(r.genTargetAsset.String), r.genTargetMantissa.Int64)
	if err != nil {
		return domain.HedgeGroup{}, "", fixedpoint.Quantity{},
			fmt.Errorf("oms journal: replay: record %d target: %w", r.seq, err)
	}
	return group, domain.ReservationID(r.genReservation.String), target, nil
}

// replayGenerationSize rebuilds FLAG-3's authorized spot quantity.
//
// A zero mantissa is a legal, ordinary value here and the reconstruction says
// nothing about it — the record's EXISTENCE is what carries "an authorization
// happened", and its absence has no spelling in this store. What is refused is a
// missing COLUMN, which is a different claim: the record exists and the thing it
// describes does not.
func replayGenerationSize(r replayRow) (fixedpoint.Quantity, error) {
	if !r.genSizeAsset.Valid {
		return fixedpoint.Quantity{}, detailMissing(r, "journal_generation_authorized.size_asset")
	}
	if !r.genSizeMantissa.Valid {
		return fixedpoint.Quantity{}, detailMissing(r, "journal_generation_authorized.size_mantissa")
	}
	size, err := fixedpoint.QuantityFromMantissa(
		fixedpoint.Asset(r.genSizeAsset.String), r.genSizeMantissa.Int64)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("oms journal: replay: record %d size: %w", r.seq, err)
	}
	return size, nil
}

// replayAnchor rebuilds §5.3's authorization instant.
//
// time.Unix reconstructs a UTC instant with no monotonic reading and no
// location, which is the canonical form the column can hold — and the reason
// AppendAuthorized refuses an instant that does not round-trip through
// UnixNano. A wall clock stored and reloaded is the same instant; a
// wall-and-monotonic clock stored and reloaded is not the same time.Time, and
// nothing here needs it to be.
func replayAnchor(r replayRow) (clock.ServerTime, error) {
	if !r.instantUnixNano.Valid {
		return clock.ServerTime{}, detailMissing(r, "journal_authorized.instant_unix_nano")
	}
	if !r.uncertaintyNano.Valid {
		return clock.ServerTime{}, detailMissing(r, "journal_authorized.uncertainty_nano")
	}
	at, err := clock.NewServerTime(
		time.Unix(0, r.instantUnixNano.Int64).UTC(),
		time.Duration(r.uncertaintyNano.Int64))
	if err != nil {
		return clock.ServerTime{}, fmt.Errorf("oms journal: replay: record %d anchor: %w", r.seq, err)
	}
	return at, nil
}

// visit hands one verified record to the visitor.
//
// Split from the walk so that the walk holds the chain logic and this holds the
// dispatch, and because the two fail differently: everything above is a
// statement about the LOG, and everything the visitor returns is a statement
// about what the log MEANS. §7.5's postures differ on exactly that line.
func visit(v ports.JournalVisitor, r replayRow) error {
	switch r.kind {
	case kindPlanned:
		// Rebuilt a second time rather than threaded out of replayPreimage. The
		// duplication is real and it is worth one map lookup's cost: the value
		// the visitor receives is derived from the columns the verification just
		// covered, by the same function, so there is no path by which a
		// hand-assembled order could reach a caller as a verified one.
		o, err := replayOrder(r)
		if err != nil {
			return err
		}
		return v.Planned(uint64(r.seq), o)

	case kindState:
		return v.State(uint64(r.seq), ids.ClientOrderID(r.subject), domain.OrderState(r.state.String))

	case kindAuthorized:
		at, err := replayAnchor(r)
		if err != nil {
			return err
		}
		return v.Authorized(uint64(r.seq), ids.ClientOrderID(r.subject), at)

	case kindAcked:
		return v.Acked(uint64(r.seq), ids.ClientOrderID(r.subject), r.venueOrderID.String)

	case kindGenerationOpened:
		// Rebuilt a second time rather than threaded out of replayPreimage, on
		// the planned record's rationale: the values the visitor receives are
		// derived from the columns the verification just covered, by the same
		// function, so there is no path by which a hand-assembled group could
		// reach a caller as a verified one.
		group, reservation, target, err := replayGenerationOpening(r)
		if err != nil {
			return err
		}
		return v.GenerationOpened(uint64(r.seq), domain.GenerationID(r.subject), group, reservation, target)

	case kindGenerationPerpSubmitted:
		return v.GenerationPerpSubmitted(uint64(r.seq),
			domain.GenerationID(r.subject), ids.ClientOrderID(r.genLegClientID.String))

	case kindGenerationAuthorized:
		size, err := replayGenerationSize(r)
		if err != nil {
			return err
		}
		return v.GenerationAuthorized(uint64(r.seq), domain.GenerationID(r.subject), size)

	case kindGenerationSpotSubmitted:
		return v.GenerationSpotSubmitted(uint64(r.seq),
			domain.GenerationID(r.subject), ids.ClientOrderID(r.genLegClientID.String))

	case kindGenerationSuperseded:
		return v.GenerationSuperseded(uint64(r.seq),
			domain.GenerationID(r.subject), domain.SupersessionPath(r.genPath.String))

	case kindGenerationPhase:
		return v.Phase(uint64(r.seq),
			domain.GenerationID(r.subject), domain.GenerationPhase(r.genPhase.String))
	}
	return fmt.Errorf("oms journal: replay: record %d has kind %q: %w",
		r.seq, r.kind, ErrJournalKindUnknown)
}

// detailMissing reports a journal row whose detail row or column is absent —
// the outer record exists and the thing it describes does not.
//
// It cannot happen through this adapter: both rows are written in one
// transaction, and the foreign key is enforced (foreign_keys(1), verified at
// Open). It is reported rather than assumed away because the store is a FILE an
// operator can open, and the failure mode of assuming is reading a NULL as a
// zero — an order with no side, at no price, in no state.
func detailMissing(r replayRow, column string) error {
	return fmt.Errorf("oms journal: replay: record %d of kind %q has no %s: %w",
		r.seq, r.kind, column, ErrJournalDetailMissing)
}
