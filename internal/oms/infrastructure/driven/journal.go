package driven

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"

	// The pure-Go SQLite driver. cgo-free is a requirement rather than a
	// preference: §13 pins the deployment target to Linux and the capture
	// binary is built and shipped as a static artifact, so a cgo driver would
	// make the OMS the one component that cannot be cross-built.
	_ "modernc.org/sqlite"
)

// §3.6 rule 2's compile-time assertion. Under mutation this reports NOBUILD!!,
// which for THIS assertion is a kill rather than a non-result — see CLAUDE.md's
// verdict table.
var _ ports.Journal = (*SQLiteJournal)(nil)

// SQLiteJournal is §5.1's durable, append-only, hash-chained write-ahead
// journal, and the adapter §47e recorded as missing.
//
// # The store is SQLite because §5.3 will need it to be, not because a log file
// # would not work
//
// A hash-chained file is the smaller implementation and it was rejected on one
// sentence: §5.3 requires that applying a fill, advancing the per-instrument
// REST recovery cursor, and appending journal/outbox events happen in ONE
// transaction. driven/doc.go already states the consequence — "a transaction
// spanning two stores cannot be added to one of them alone" — so a journal in
// its own file is a journal the fill path cannot join, and the rule would be
// met by rewriting this adapter at slice D/F or not met at all. D2 and §13 name
// the store (SQLite, WAL, synchronous=FULL, local filesystem only); this is
// that store arriving for its first tenant.
//
// # Durability is per call, and it costs one fsync per record
//
// ports.Journal's contract is that a method returns only when the record is
// durable, with no Sync to pair. Here that is one transaction per Append, and
// under WAL + synchronous=FULL one transaction is one WAL fsync. The cost is
// real and it is the cost the contract asks for: batching would satisfy every
// signature and none of the meaning, and the defect surfaces once — during the
// crash the fsync existed for.
//
// The pragmas are set in the DSN rather than by a statement after opening, and
// that is not a style choice. `synchronous` is a PER-CONNECTION setting, and
// database/sql owns a POOL: a pragma issued once as a statement configures
// whichever connection happened to serve it, and the pool's next connection is
// created with the default. The failure is silent, intermittent, and in the
// direction that loses data. Set in the DSN, every connection the pool opens
// carries it — and Open then VERIFIES it rather than assuming, which is §13's
// "boot verifies journal_mode/synchronous pragmas".
//
// # What is proven here and what is not
//
// The tests below prove the configuration that produces the fsync, not the
// fsync. No portable Go test can observe a device write, and one that claimed
// to would be the more dangerous artifact. What IS bite-checkable is the
// configuration claim itself: a mutant that writes synchronous(normal) has to
// fail, and it does — see TestOpenRefusesWeakenedDurability.
//
// The remaining gap is the host, and it is worth naming because development
// happens on the wrong one: on macOS fsync(2) does not flush the drive's write
// cache (F_FULLFSYNC does, and §13 records that it is macOS-only and therefore
// NOT the mechanism). So a passing suite on a developer Mac is evidence about
// this code and not about power-loss durability. The Linux target with write
// barriers is where the claim holds.
//
// # §13's THIRD durability precondition is not checked here, and where it goes
//
// §13 names three: WAL, synchronous=FULL, and "local filesystem only — ext4/xfs
// with write barriers, NEVER NFS/network FS (SQLite documents these as
// corruption-prone)". Open verifies the first two and does not check the third.
//
// Recorded rather than left silent, because an unchecked normative claim is
// exactly what §47e found twice in a roster that read as complete, and the
// slice-close marker grep is structurally blind to an UNMARKED one. This
// carries no marker either, and deliberately: a marker means the code departs
// from the spec and the spec must change, and nothing here departs — the
// requirement is scheduled, not declined. (Phrased without the marker's literal
// wording on purpose. The grep scans code, so a comment ABOUT the marker is
// indistinguishable from the marker itself — the same collision the doc set
// avoids by being out of scope, arriving one directory in.)
//
// It is scheduled to §7.5's reconcile-only boot (slice F) rather than to this
// constructor, on placement grounds rather than effort. §7.5 already owns a
// list of refuse-to-trade posture assertions checked before trading resumes —
// margin mode, spot-as-collateral, position mode, leverage pin — and "the
// journal is on a filesystem SQLite can be trusted on" is a member of that
// list, not a property of opening a file. Splitting it off would put one
// posture check in an adapter constructor and the rest in boot, which is how a
// checklist acquires a member nobody knows to look for. Task #168.
//
// # The dependency, and what it costs the graph
//
// transport/doc.go records the engine module's FIRST third-party runtime
// dependency and the fact that it added one node with no transitive edges. This
// is the second, and the honest figure is different: modernc.org/sqlite brings
// nine more modules (libc, mathutil, memory, bigfft, go-humanize, uuid,
// go-isatty, go-strftime, golang.org/x/sys). §13 names the store rather than a
// driver, so the driver was the open choice; cgo-free decided it, because the
// alternative makes the OMS the one component in a Linux-target deployment that
// cannot be cross-built, and §12 milestone 3 has no exception for that.
//
// # Single connection, and the chain is why
//
// Each Append reads the chain head and writes the record that extends it. Two
// connections doing that concurrently is a read-modify-write race whose losing
// side writes a record whose prev_hash names a record that is no longer the
// head — a fork in a structure that has no rule for one. SetMaxOpenConns(1)
// serialises it at the pool, and BEGIN IMMEDIATE (via _txlock) takes the write
// lock at the start of the transaction rather than on the first write, so a
// second process opening the same file fails fast instead of mid-append.
type SQLiteJournal struct {
	db *sql.DB
}

// schema is the journal's DDL, applied on every Open (each statement is IF NOT
// EXISTS, so an existing file is left alone).
//
// # A detail table per PAYLOAD rather than one wide one, and the reason is
// # ports.Journal's own
//
// The port declares two methods rather than one Append(Record) because a single
// record type would need "an optional field, whose zero value is then a legal
// value with no meaning". One wide table is that rejected shape in SQL: a
// planned row and a state row have disjoint field sets, so half the columns are
// NULL on every row and a state record with a stray side column is
// representable. Splitting the detail out lets every column in every table be
// NOT NULL, which is the same totality argument one layer down.
//
// The rejected alternative was one table plus a CHECK constraint asserting the
// per-kind nullability. It is expressible and it was refused because it states
// the field list twice — once in the columns, once in the CHECK — and the copy
// that drifts is always the one that is not the definition.
//
// The generation kinds follow the same rule and produce ONE exception, which is
// the case that shows what the rule is actually about. `journal_generation_leg`
// serves two kinds — the perp submission and the spot submission — because their
// column is the same column: a client order id, meaning a client order id, in
// both. Every other pair of same-shaped kinds got its own table because the
// shared shape carried DIFFERENT meanings (a state name and a venue order id are
// both one TEXT), and a column whose meaning depends on the kind is the thing
// being avoided. What was refused here is the near-miss: a `leg` discriminator
// column beside the id. That is a second spelling of a fact the kind column
// already states, with no rule for a replay that finds the two disagree —
// encodeAuthorized's own argument for leaving the state out of its preimage.
//
// # Append-only is enforced by the database, not by this package's method set
//
// The triggers are the point. An adapter that simply never issues UPDATE or
// DELETE is append-only by convention, and the convention holds until the first
// migration script, repair query or `sqlite3` session. RAISE(ABORT) makes the
// mutation fail where it is attempted. This is the standing rule about defaults
// with two directions, applied to a table: the version that is right when
// somebody forgets.
//
// # The money columns are mantissa + asset, which is fixedpoint's seam
//
// Not a formatted string. fixedpoint/codec.go argues it in full: the int64
// mantissa is exactly SQLite INTEGER and exactly Postgres BIGINT, so the stored
// form is lossless and identical across the dialects D2 names, and the asset
// tag needs its own column because no single column can carry a number and a
// tag without becoming text.
//
// # ONE chain over a widened SUBJECT column, and the alternative was cheaper
//
// The generation records (ports.GenerationJournal, SM §7) do not key by an
// order. A generation's subject is a domain.GenerationID — "each generation is a
// bounded attempt", so the identity outlives either leg — and the two legs are
// two ClientOrderIDs of which neither is the generation. The cheap shape is a
// second store with its own chain and no migration; it was refused on one
// sentence, and the sentence is FLAG-3's.
//
// The invariant a replay most wants to check is that a generation's
// AUTHORIZATION record precedes its spot order's own journaled intent. Two
// chains have no order between them: each verifies internally and neither says
// anything about the other's position, so under two stores that clause stops
// being a fact anything can check and becomes a discipline the application layer
// keeps. One chain makes the sequence itself the evidence.
//
// So the column is named for what it holds — a subject — and the KIND column is
// its namespace. That split is load-bearing rather than tidy: the two id spaces
// share no namespace, so `subject` alone identifies nothing and a lookup by it
// could match the other space. journalcodec.go states the consequence for
// queries; the short version is that this package has no method taking a bare
// subject, so the unsafe query is not writable rather than merely discouraged.
//
// Migrating an APPEND-ONLY store is the cost, and migrateSubjectColumn below is
// where it is paid. What it does not cost is the chain: renaming a column
// changes no preimage, because every encoder hashes domain values rather than
// columns — see encodeGenerationPhase's closing note.
const schema = `
CREATE TABLE IF NOT EXISTS journal (
	seq       INTEGER PRIMARY KEY,
	kind      TEXT NOT NULL,
	subject   TEXT NOT NULL,
	prev_hash BLOB NOT NULL,
	hash      BLOB NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_planned (
	seq            INTEGER PRIMARY KEY REFERENCES journal(seq),
	venue          TEXT NOT NULL,
	product        TEXT NOT NULL,
	symbol         TEXT NOT NULL,
	contract       TEXT NOT NULL,
	side           TEXT NOT NULL,
	order_type     TEXT NOT NULL,
	time_in_force  TEXT NOT NULL,
	qty_asset      TEXT NOT NULL,
	qty_mantissa   INTEGER NOT NULL,
	price_base     TEXT NOT NULL,
	price_quote    TEXT NOT NULL,
	price_mantissa INTEGER NOT NULL,
	reduce_only    INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_state (
	seq   INTEGER PRIMARY KEY REFERENCES journal(seq),
	state TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_authorized (
	seq               INTEGER PRIMARY KEY REFERENCES journal(seq),
	instant_unix_nano INTEGER NOT NULL,
	uncertainty_nano  INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_acked (
	seq            INTEGER PRIMARY KEY REFERENCES journal(seq),
	venue_order_id TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_generation_opened (
	seq             INTEGER PRIMARY KEY REFERENCES journal(seq),
	account         TEXT NOT NULL,
	venue           TEXT NOT NULL,
	symbol          TEXT NOT NULL,
	reservation     TEXT NOT NULL,
	target_asset    TEXT NOT NULL,
	target_mantissa INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_generation_leg (
	seq       INTEGER PRIMARY KEY REFERENCES journal(seq),
	client_id TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_generation_authorized (
	seq           INTEGER PRIMARY KEY REFERENCES journal(seq),
	size_asset    TEXT NOT NULL,
	size_mantissa INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_generation_superseded (
	seq  INTEGER PRIMARY KEY REFERENCES journal(seq),
	path TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_generation_phase (
	seq   INTEGER PRIMARY KEY REFERENCES journal(seq),
	phase TEXT NOT NULL
) STRICT;

-- core §7.1's MarginOp, fsynced BEFORE the venue call it describes. The
-- instrument is carried in full because a resolution has to RE-READ it, and the
-- op's subject column holds a MarginOpID rather than a symbol.
--
-- There is ONE asset column for two amounts, and that is a claim rather than a
-- saving: domain.MarginOp.Validate requires the delta and the before-state to be
-- in one asset, so a schema with two columns would be able to store a record
-- that contradicts its own type. The cost is named where it lands — the port's
-- asset check becomes unreachable from a replay, and is exercised from the
-- application side instead.
CREATE TABLE IF NOT EXISTS journal_margin_op_dispatched (
	seq              INTEGER PRIMARY KEY REFERENCES journal(seq),
	venue            TEXT NOT NULL,
	product          TEXT NOT NULL,
	symbol           TEXT NOT NULL,
	contract         TEXT NOT NULL,
	asset            TEXT NOT NULL,
	delta_mantissa   INTEGER NOT NULL,
	before_mantissa  INTEGER NOT NULL,
	watermark_unix_nano INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS journal_margin_op_resolved (
	seq    INTEGER PRIMARY KEY REFERENCES journal(seq),
	status TEXT NOT NULL
) STRICT;

-- core §7.3's per-actor audit of the one control action this system has: the
-- operator act that reconciles an Ambiguous margin op's staged float (§7.1).
--
-- Three columns for §7.3's "who / what / when" — "what" being the subject
-- column, which holds the MarginOpID this act is about — plus the note, which
-- carries what was actually done. There is deliberately no finding column: see
-- domain.MarginOpReconciliation.
--
-- at_unix_nano is the OPERATOR's instant and not a venue clock's, which is the
-- opposite choice from journal_margin_op_dispatched.watermark_unix_nano one
-- table up. The difference is what each instant is compared against: a watermark
-- anchors an evidence window closed against venue-stamped rows, and this one
-- anchors nothing — it is read by a human, in the frame the human acted in.
CREATE TABLE IF NOT EXISTS journal_margin_op_reconciled (
	seq         INTEGER PRIMARY KEY REFERENCES journal(seq),
	actor       TEXT NOT NULL,
	at_unix_nano INTEGER NOT NULL,
	note        TEXT NOT NULL
) STRICT;

CREATE TRIGGER IF NOT EXISTS journal_no_update BEFORE UPDATE ON journal
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_no_delete BEFORE DELETE ON journal
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_planned_no_update BEFORE UPDATE ON journal_planned
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_planned_no_delete BEFORE DELETE ON journal_planned
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_state_no_update BEFORE UPDATE ON journal_state
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_state_no_delete BEFORE DELETE ON journal_state
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_authorized_no_update BEFORE UPDATE ON journal_authorized
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_authorized_no_delete BEFORE DELETE ON journal_authorized
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_acked_no_update BEFORE UPDATE ON journal_acked
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_acked_no_delete BEFORE DELETE ON journal_acked
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_opened_no_update BEFORE UPDATE ON journal_generation_opened
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_opened_no_delete BEFORE DELETE ON journal_generation_opened
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_leg_no_update BEFORE UPDATE ON journal_generation_leg
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_leg_no_delete BEFORE DELETE ON journal_generation_leg
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_authorized_no_update BEFORE UPDATE ON journal_generation_authorized
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_authorized_no_delete BEFORE DELETE ON journal_generation_authorized
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_superseded_no_update BEFORE UPDATE ON journal_generation_superseded
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_superseded_no_delete BEFORE DELETE ON journal_generation_superseded
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_phase_no_update BEFORE UPDATE ON journal_generation_phase
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_generation_phase_no_delete BEFORE DELETE ON journal_generation_phase
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_dispatched_no_update BEFORE UPDATE ON journal_margin_op_dispatched
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_dispatched_no_delete BEFORE DELETE ON journal_margin_op_dispatched
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_resolved_no_update BEFORE UPDATE ON journal_margin_op_resolved
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_resolved_no_delete BEFORE DELETE ON journal_margin_op_resolved
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_reconciled_no_update BEFORE UPDATE ON journal_margin_op_reconciled
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_margin_op_reconciled_no_delete BEFORE DELETE ON journal_margin_op_reconciled
	BEGIN SELECT RAISE(ABORT, 'oms journal is append-only'); END;
`

// The pragma values Open requires, named so the verification and the DSN cannot
// drift apart: both read these constants.
const (
	wantJournalMode = "wal"

	// wantSynchronous is SQLite's FULL. In WAL mode this is the setting that
	// fsyncs the WAL on every commit — NORMAL fsyncs only at checkpoint, which
	// is precisely a journal that returns before the record is durable.
	wantSynchronous = 2

	// wantForeignKeys is on, so the detail tables' REFERENCES actually bite.
	// SQLite defaults foreign keys OFF, which makes a declared REFERENCES a
	// comment.
	wantForeignKeys = 1
)

// OpenSQLiteJournal opens (or creates) the journal at path and returns it ready
// to append.
//
// It is not exported to composition roots in any usable sense — singlewriter
// forbids every package outside internal/oms/ from importing this one — so the
// caller is oms/module.go. That is §26e point 3 arriving as a scheduled
// consequence: the adapter sits in a policed package, so the root supplies
// configuration and the context constructs.
func OpenSQLiteJournal(path string) (*SQLiteJournal, error) {
	if path == "" {
		return nil, ErrJournalPathUnset
	}

	db, err := sql.Open("sqlite", journalDSN(path))
	if err != nil {
		return nil, fmt.Errorf("oms journal: open %q: %w", path, err)
	}

	// One connection. See the type comment: the chain head is read and extended
	// in the same transaction, and two connections doing that concurrently fork
	// the chain.
	db.SetMaxOpenConns(1)

	j := &SQLiteJournal{db: db}
	if err := j.verifyPragmas(); err != nil {
		return nil, errors.Join(fmt.Errorf("oms journal: %q: %w", path, err), db.Close())
	}
	// BEFORE the DDL, because the DDL is the reason a migration is needed at
	// all: every statement in it is IF NOT EXISTS, so an existing journal table
	// is left exactly as it was found and a schema change would silently not
	// happen. Run first, the rename brings the table up to the shape the DDL
	// would have created, and the DDL is then a no-op on it.
	if err := j.migrateSubjectColumn(); err != nil {
		return nil, errors.Join(fmt.Errorf("oms journal: migrate %q: %w", path, err), db.Close())
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, errors.Join(fmt.Errorf("oms journal: schema %q: %w", path, err), db.Close())
	}
	return j, nil
}

// migrateSubjectColumn renames `journal.client_id` to `journal.subject` on a
// database written before the generation records shared this chain.
//
// # Migrating an append-only store is not a contradiction, and the distinction
// # is what the triggers are actually for
//
// The triggers fire BEFORE UPDATE and BEFORE DELETE on rows. `ALTER TABLE …
// RENAME COLUMN` is neither: no row is written, no row is removed, and every
// stored value survives byte-for-byte under a new name. That is the whole reason
// this migration is expressible — the append-only rule is about the HISTORY, and
// renaming the column the history is recorded in changes no history.
//
// The chain is untouched for a second and independent reason, and it is worth
// separating from the first: even a migration that DID rewrite rows would not
// invalidate a hash here, because no encoder hashes a column. They hash domain
// values (see encodeGenerationPhase's closing note), so chainLabel keeps its /v1
// and a journal written by the previous build verifies unchanged after this
// runs. TestAJournalWrittenBeforeTheSubjectRenameStillVerifies is that claim.
//
// # Every shape but the two expected ones is REFUSED, including the empty one
//
// The check is over what the table HAS rather than over a version number, and a
// version row was the rejected alternative: it is a second statement of the
// schema's shape, and the copy that drifts is the one that is not the
// definition — the same argument the CHECK constraint lost on. Reading the
// columns asks the database what it is instead of asking it what it was told.
//
// A `journal` table with NEITHER column is refused rather than left to fail at
// the first query. It cannot be produced by any build of this system, so the
// realistic cause is a path pointing at somebody else's database — and the
// failure that follows from proceeding is an opaque SQL error at the first
// append, on the path §6 step 6 must not be surprised on. Both columns at once
// is refused for the sharper reason: there is no rule for which one the records
// were keyed by, and picking either would silently re-key a chain.
func (j *SQLiteJournal) migrateSubjectColumn() error {
	rows, err := j.db.Query(`PRAGMA table_info(journal)`)
	if err != nil {
		return fmt.Errorf("read journal columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hasSubject, hasClientID, tableExists bool
	for rows.Next() {
		var cid int
		var name, declType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &declType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("read journal columns: %w", err)
		}
		tableExists = true
		switch name {
		case "subject":
			hasSubject = true
		case "client_id":
			hasClientID = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read journal columns: %w", err)
	}

	switch {
	case !tableExists:
		// No journal table. A first boot, and the DDL below is about to create
		// it already carrying the subject column.
		return nil
	case hasSubject && hasClientID:
		return fmt.Errorf("journal has both a subject and a client_id column: %w", ErrJournalSchemaUnknown)
	case hasSubject:
		return nil
	case hasClientID:
		if _, err := j.db.Exec(`ALTER TABLE journal RENAME COLUMN client_id TO subject`); err != nil {
			return fmt.Errorf("rename client_id to subject: %w", err)
		}
		return nil
	}
	return fmt.Errorf("journal has neither a subject nor a client_id column: %w", ErrJournalSchemaUnknown)
}

// journalDSN builds the connection string.
//
// url.URL rather than string concatenation, because a path containing a '?' or
// a '#' would otherwise silently truncate the pragma list — and a DSN whose
// pragmas were dropped opens successfully with the defaults. verifyPragmas
// catches that, which is the argument for having it rather than an argument for
// being careless here.
func journalDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode("+wantJournalMode+")")
	q.Add("_pragma", fmt.Sprintf("synchronous(%d)", wantSynchronous))
	q.Add("_pragma", fmt.Sprintf("foreign_keys(%d)", wantForeignKeys))

	// Take the write lock when the transaction begins rather than at its first
	// write. Under deferred locking a second writer discovers the conflict
	// after this process has already read the chain head, which turns a clean
	// refusal into a retry against a head that may have moved.
	q.Set("_txlock", "immediate")

	u := url.URL{Scheme: "file", Opaque: path, RawQuery: q.Encode()}
	return u.String()
}

// verifyPragmas is §13's "boot verifies journal_mode/synchronous pragmas".
//
// It exists because the DSN is a REQUEST. A driver that ignores an unknown
// pragma, a path that swallowed the query string, a future driver upgrade that
// renames the option — each leaves a journal that opens cleanly, works
// perfectly, and is not durable. Every one of those is invisible without this
// function, and none of them is visible in a test that only appends and reads
// back.
func (j *SQLiteJournal) verifyPragmas() error {
	var mode string
	if err := j.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if mode != wantJournalMode {
		return fmt.Errorf("journal_mode is %q, want %q: %w", mode, wantJournalMode, ErrJournalPragma)
	}

	var sync int
	if err := j.db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		return fmt.Errorf("read synchronous: %w", err)
	}
	if sync != wantSynchronous {
		return fmt.Errorf("synchronous is %d, want %d (FULL): %w", sync, wantSynchronous, ErrJournalPragma)
	}

	var fk int
	if err := j.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if fk != wantForeignKeys {
		return fmt.Errorf("foreign_keys is %d, want %d: %w", fk, wantForeignKeys, ErrJournalPragma)
	}
	return nil
}

// Close releases the database.
//
// It does not checkpoint the WAL, and the omission is deliberate: a checkpoint
// on close would make a clean shutdown produce a different on-disk shape from a
// crash, and §7.5's recovery has to handle the crash shape anyway. Leaving them
// identical means the path recovery exercises is the path every start takes.
func (j *SQLiteJournal) Close() error {
	if err := j.db.Close(); err != nil {
		return fmt.Errorf("oms journal: close: %w", err)
	}
	return nil
}

// AppendPlanned durably records the intent to place an order — §6 step 5.
//
// It validates the order, and the boundary between what it checks and what it
// refuses to check is ports.Journal's own. The port says legality is the
// caller's, and means SM §1's TRANSITION legality: a recorder that refused
// illegal transitions would be a second home for the edge set. Well-formedness
// is a different question. A record this package cannot rebuild into a valid
// domain.Order is a record that fails at §7.5's replay — at boot, on a process
// that is trying to find out what orders exist at a venue — so it is refused
// here, where the caller still has the order in hand and nothing has been sent.
func (j *SQLiteJournal) AppendPlanned(o domain.Order) error {
	if err := o.Validate(); err != nil {
		return fmt.Errorf("oms journal: append planned: %w", err)
	}
	return j.appendRecord(kindPlanned, string(o.ClientOrderID), encodePlanned(o),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_planned
				(seq, venue, product, symbol, contract, side, order_type, time_in_force,
				 qty_asset, qty_mantissa, price_base, price_quote, price_mantissa, reduce_only)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				seq,
				string(o.Instrument.Venue), string(o.Instrument.Product),
				o.Instrument.Symbol, string(o.Instrument.Contract),
				string(o.Side), string(o.Type), string(o.TimeInForce),
				string(o.Quantity.Asset()), o.Quantity.Mantissa(),
				string(o.LimitPrice.Base()), string(o.LimitPrice.Quote()), o.LimitPrice.Mantissa(),
				o.ReduceOnly)
			return err
		})
}

// AppendAuthorized durably records §6 step 6's commitment to call Place exactly
// once, with the instant §5.3 measures its two bounds from.
//
// # The instant is refused for being UNSET and for being unstorable, and those
// # are two different checks
//
// The first is the port's, and ApplyAuthorized makes the same one against the
// same sentinel: an unset ServerTime answers Uncertain to every boundary, so
// accepting it here would durably record an authorization whose order can never
// be resolved.
//
// The second is this adapter's alone, because it is a property of the STORED
// form rather than of the value: the column is a Unix-nanosecond integer, and
// time.Time.UnixNano does not report an instant outside roughly 1678..2262 — it
// wraps, silently, and the journal gets a plausible instant in the wrong
// century. capture's frame codec pays for the same lesson with the same
// round-trip check, and the direction here is worse: a wrapped anchor does not
// look wrong, it looks like an order authorized long ago, which is precisely
// the reading §5.3's retention bound treats as "the venue has purged this by
// now, absence proves nothing".
func (j *SQLiteJournal) AppendAuthorized(id ids.ClientOrderID, at clock.ServerTime) error {
	if id == "" {
		return fmt.Errorf("oms journal: append authorization: %w", ErrJournalClientIDUnset)
	}
	if err := at.Validate(); err != nil {
		return fmt.Errorf("oms journal: append authorization for order %q: %w: %w",
			id, ports.ErrAuthorizationNeedsAnchor, err)
	}
	instant := at.Instant()
	if ns := instant.UnixNano(); !time.Unix(0, ns).Equal(instant) {
		return fmt.Errorf("oms journal: append authorization for order %q at %s: %w",
			id, instant.Format(time.RFC3339), ErrJournalAnchorRange)
	}
	return j.appendRecord(kindAuthorized, string(id), encodeAuthorized(id, at),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(
				`INSERT INTO journal_authorized (seq, instant_unix_nano, uncertainty_nano) VALUES (?,?,?)`,
				seq, instant.UnixNano(), int64(at.Uncertainty()))
			return err
		})
}

// AppendAcked durably records the venue's identifier for an order.
//
// Both refusals are the port's, restated as this adapter's checks: an empty
// client id is a record about nothing, and an empty venue id is the absent
// id acquiring a second spelling — the journal's spelling of "the venue
// supplied none" is that no acked record exists.
//
// It does NOT check the id against other orders' — that is the projection's
// refusal (ApplyAck, ErrVenueOrderIDConflict), and this recorder holding a
// copy of it would be a second home for the rule, exactly what ports.Journal
// refuses about SM §1's edge set. The consequence is deliberate and is the
// crash-path half of the conflict design: a journal CAN hold two acked
// records claiming one id, and §7.5's replay is where that log is found to
// verify and not compose.
func (j *SQLiteJournal) AppendAcked(id ids.ClientOrderID, venueOrderID string) error {
	if id == "" {
		return fmt.Errorf("oms journal: append ack: %w", ErrJournalClientIDUnset)
	}
	if venueOrderID == "" {
		return fmt.Errorf("oms journal: append ack for order %q: %w", id, ErrJournalVenueOrderIDUnset)
	}
	return j.appendRecord(kindAcked, string(id), encodeAcked(id, venueOrderID),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_acked (seq, venue_order_id) VALUES (?,?)`,
				seq, venueOrderID)
			return err
		})
}

// AppendState durably records an order reaching a lifecycle state.
//
// StateDispatchAuthorized is refused HERE, on the target alone, exactly as
// MemoryOrderWriteRepository.ApplyState refuses it one store over — and the
// mirror is the point rather than a coincidence of design. The log and the
// projection are the two places an order's state lives, and a rule enforced in
// one of them leaves the other holding the shape the rule exists to eliminate.
// Here that shape is a durable authorization with no anchor, in the store §7.5
// replays.
//
// The state is otherwise checked for MEMBERSHIP and nothing else —
// OrderState.Validate, which separates the unset value from an unknown one.
// That is the same line AppendPlanned draws: an undeclared state is a value
// §7.5 cannot replay, while an illegal transition between two declared states
// is a claim about SM §1's edges that this recorder deliberately does not hold.
func (j *SQLiteJournal) AppendState(id ids.ClientOrderID, state domain.OrderState) error {
	if state == domain.StateDispatchAuthorized {
		return fmt.Errorf("oms journal: append state %s for order %q: %w",
			state, id, ports.ErrAuthorizationNeedsAnchor)
	}
	if id == "" {
		return fmt.Errorf("oms journal: append state %q: %w", state, ErrJournalClientIDUnset)
	}
	if err := state.Validate(); err != nil {
		return fmt.Errorf("oms journal: append state for order %q: %w", id, err)
	}
	return j.appendRecord(kindState, string(id), encodeState(id, state),
		func(tx *sql.Tx, seq int64) error {
			_, err := tx.Exec(`INSERT INTO journal_state (seq, state) VALUES (?,?)`,
				seq, string(state))
			return err
		})
}

// appendRecord is the chain extension both methods share: read the head, link
// to it, write both rows, commit.
//
// The head is read from the DATABASE inside the transaction rather than from a
// field on this struct, and the cheaper alternative is worth naming as rejected
// rather than left as an optimisation somebody adds later. An in-memory head is
// a second copy of a fact the table already states, and the two disagree in
// exactly the case that matters: a process that opened an existing journal, or
// one whose last commit failed after the field was updated. Re-reading costs an
// indexed single-row lookup on the primary key, against an fsync in the same
// transaction — it is not measurable next to what it is protecting.
//
// The subject is a plain string here rather than a typed id, and that is
// forced rather than lax: the column's whole point is that it holds two
// different id spaces discriminated by the kind, so no single type could name
// it. Every caller converts at the call site, where the kind and the id space
// are named on the same line and cannot be paired wrongly.
func (j *SQLiteJournal) appendRecord(kind, subject string, body []byte, detail func(*sql.Tx, int64) error) error {
	tx, err := j.db.Begin()
	if err != nil {
		return fmt.Errorf("oms journal: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	seq, prev, err := chainHead(tx)
	if err != nil {
		return fmt.Errorf("oms journal: read chain head: %w", err)
	}

	hash := chainHash(uint64(seq), prev, body)
	if _, err := tx.Exec(
		`INSERT INTO journal (seq, kind, subject, prev_hash, hash) VALUES (?,?,?,?,?)`,
		seq, kind, subject, prev[:], hash[:]); err != nil {
		return fmt.Errorf("oms journal: insert %s record at seq %d: %w", kind, seq, err)
	}
	if err := detail(tx, seq); err != nil {
		return fmt.Errorf("oms journal: insert %s detail at seq %d: %w", kind, seq, err)
	}

	// The fsync is here. Under WAL + synchronous=FULL the commit does not
	// return until the WAL is on the device, which is what lets this method's
	// return double as ports.Journal's durability guarantee.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("oms journal: commit %s record at seq %d: %w", kind, seq, err)
	}
	return nil
}

// Head reports the journal's last sequence and that record's hash — the chain
// mark task #179's baseline document carries, so a baseline can be refused
// when presented against a history it never attested.
//
// An EMPTY journal reports (0, zero-hash, nil), and the zero sequence is the
// deliberate carrier rather than a zero-value accident: a baseline recorded
// over an empty journal attested nothing (`ours` was zero by construction), so
// its mark verifies vacuously — every journal extends the empty one — and the
// loader's HashAt check is skipped by the seq, not by a flag beside it.
//
// It is a method on the concrete adapter rather than on ports.JournalReplay:
// its consumers are the composition layers (the record command and the boot
// loader), which construct this type and hand the APPLICATION layer
// already-verified baseline data. The application never sees a hash, so the
// port carries none.
func (j *SQLiteJournal) Head() (uint64, [32]byte, error) {
	var seq int64
	var hash []byte
	err := j.db.QueryRow(`SELECT seq, hash FROM journal ORDER BY seq DESC LIMIT 1`).
		Scan(&seq, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, [32]byte{}, nil
	}
	if err != nil {
		return 0, [32]byte{}, fmt.Errorf("oms journal: read head: %w", err)
	}
	if len(hash) != len(genesisHash()) {
		return 0, [32]byte{}, fmt.Errorf("head at seq %d has a %d-byte hash, want %d: %w",
			seq, len(hash), len(genesisHash()), ErrJournalChainCorrupt)
	}
	var out [32]byte
	copy(out[:], hash)
	return uint64(seq), out, nil
}

// HashAt reports the stored hash of the record at seq, distinguishing a
// sequence past the head (ErrJournalMarkBeyondHead — the journal LOST records
// relative to whoever recorded seq) from every other failure.
//
// It reads the STORED hash rather than recomputing the chain: whether the
// stored hashes are trustworthy is the replay's verification question, already
// asked by the boot this check runs beside, and a second verifier here would
// be a second copy of the chain rule. What this method answers is only "is the
// record at seq the same record the caller saw" — identity, not integrity.
func (j *SQLiteJournal) HashAt(seq uint64) ([32]byte, error) {
	if seq == 0 {
		// Zero is the absence of a sequence (chainHead's own rule), so there is
		// no record it could name; callers with a zero mark skip the check
		// rather than ask it.
		return [32]byte{}, fmt.Errorf("oms journal: hash at seq 0: %w", ErrJournalMarkBeyondHead)
	}
	var hash []byte
	err := j.db.QueryRow(`SELECT hash FROM journal WHERE seq = ?`, int64(seq)).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return [32]byte{}, fmt.Errorf("oms journal: no record at seq %d: %w", seq, ErrJournalMarkBeyondHead)
	}
	if err != nil {
		return [32]byte{}, fmt.Errorf("oms journal: read hash at seq %d: %w", seq, err)
	}
	if len(hash) != len(genesisHash()) {
		return [32]byte{}, fmt.Errorf("record at seq %d has a %d-byte hash, want %d: %w",
			seq, len(hash), len(genesisHash()), ErrJournalChainCorrupt)
	}
	var out [32]byte
	copy(out[:], hash)
	return out, nil
}

// chainHead returns the sequence the next record takes and the hash it links
// to.
//
// An empty journal yields sequence 1 and the genesis seed. One rather than
// zero, on the same rule the OCC version follows: zero is not a sequence, it is
// the absence of one, and a chain whose first record sits at 0 makes "no
// records" and "one record" the same answer to `SELECT MAX(seq)`.
func chainHead(tx *sql.Tx) (int64, [32]byte, error) {
	var lastSeq int64
	var lastHash []byte

	err := tx.QueryRow(`SELECT seq, hash FROM journal ORDER BY seq DESC LIMIT 1`).
		Scan(&lastSeq, &lastHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, genesisHash(), nil
	}
	if err != nil {
		return 0, [32]byte{}, err
	}
	if len(lastHash) != len(genesisHash()) {
		return 0, [32]byte{}, fmt.Errorf("head at seq %d has a %d-byte hash, want %d: %w",
			lastSeq, len(lastHash), len(genesisHash()), ErrJournalChainCorrupt)
	}

	var prev [32]byte
	copy(prev[:], lastHash)
	return lastSeq + 1, prev, nil
}
