package driven

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The journal's checks, and the oracle they are checked against.
//
// # The collapse checklist, run BEFORE the mutations
//
// This repo has been caught nine times by a fixture pinned to the value that
// collapses the distinction the code makes. The journal has four such values
// and each is varied deliberately below:
//
//  1. CHAIN LENGTH. At length 1, "link to the previous record" and "link to the
//     genesis seed" are the same predicate. At length 2 — the fix a hurried
//     reader reaches for — "link to the previous record" and "link to the FIRST
//     record" are STILL the same predicate, because record 1 is both. Only at
//     length 3 do they separate, so the chain assertions run at 1, 2 and 3, and
//     the length-3 case asserts the inequality explicitly rather than trusting
//     the equality to imply it.
//  2. RECORD KIND. A single-kind chain makes the kind tag a constant, so
//     dropping it from the preimage changes nothing. Chains here interleave.
//  3. RECORD CONTENT. Two records that differ in nothing but their sequence are
//     what makes the sequence's presence in the preimage observable, and §5.3's
//     duplicate deliveries make them a real case rather than a contrived one.
//     TestIdenticalRecordsHashDifferently is that fixture.
//  4. FIELD SPLIT POINT. A length prefix is invisible unless two records
//     differ only in where one string ends and the next begins, with BOTH
//     sides non-empty on BOTH records — observability's collision fixture
//     failed precisely by having an empty side.
//
// # The oracle is independent, and independent means it shares no code path
//
// oracleChainHash and the encoders below rebuild the preimage from the STORED
// COLUMNS with hand-written big-endian shifts and the labels spelled out as
// literals. They never call appendString, encodePlanned, chainLabel or
// binary.BigEndian. That is what makes their agreement evidence rather than a
// restatement — the discipline fixedpoint and ids established, and the reason
// the columns are typed rather than a blob: an oracle handed the writer's own
// bytes could only re-hash them.

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

func openTestJournal(t *testing.T) (*SQLiteJournal, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journal.db")
	j, err := OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("OpenSQLiteJournal: %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j, path
}

// testAnchor builds a calibrated venue instant for an authorization record.
//
// The uncertainty is derived from the offset rather than fixed, and that is the
// collapse checklist again: a helper pinning one band would make the two
// columns indistinguishable in every fixture that used it, so a codec writing
// the instant twice — or the band into the instant's column — would produce a
// chain that verifies. Both numbers vary, and they never coincide.
func testAnchor(t *testing.T, offsetSeconds int) clock.ServerTime {
	t.Helper()
	at, err := clock.NewServerTime(
		time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC).Add(time.Duration(offsetSeconds)*time.Second),
		time.Duration(offsetSeconds+1)*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return at
}

// testOrder builds a well-formed order. Every string field is a parameter that
// a collapse-checklist test varies, so nothing here is pinned by the helper.
func testOrder(t *testing.T, clientID, venue, symbol string) domain.Order {
	t.Helper()
	qty, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	price, err := fixedpoint.ParsePrice("BTC", "USDT", "64000.25")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return domain.Order{
		ClientOrderID: ids.ClientOrderID(clientID),
		Instrument: ids.VenueInstrument{
			Venue:    ids.Venue(venue),
			Product:  ids.ProductLinearPerp,
			Symbol:   symbol,
			Contract: ids.ContractLinear,
		},
		Side:        domain.SideSell,
		Type:        domain.OrderTypeLimit,
		TimeInForce: domain.TimeInForceIOC,
		Quantity:    qty,
		LimitPrice:  price,
		ReduceOnly:  false,
	}
}

// ---------------------------------------------------------------------------
// the independent replayer
// ---------------------------------------------------------------------------

type replayed struct {
	seq      int64
	kind     string
	clientID string
	prevHash []byte
	hash     []byte
	preimage []byte
}

// oracleReplay reads the journal with its own SQL, rebuilds each record's
// preimage from the typed columns, and returns what it found. It asserts
// nothing; the callers decide which property they are testing.
func oracleReplay(t *testing.T, path string) []replayed {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("oracle open: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT seq, kind, client_id, prev_hash, hash FROM journal ORDER BY seq`)
	if err != nil {
		t.Fatalf("oracle select: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []replayed
	for rows.Next() {
		var r replayed
		if err := rows.Scan(&r.seq, &r.kind, &r.clientID, &r.prevHash, &r.hash); err != nil {
			t.Fatalf("oracle scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("oracle rows: %v", err)
	}

	for i := range out {
		out[i].preimage = oraclePreimage(t, db, out[i])
	}
	return out
}

// oraclePreimage rebuilds one record's hashed bytes from its detail row.
//
// The kind switch is a plain if/else rather than a switch over a declared
// enum-like, deliberately: kind is a stored TEXT value, and an unrecognised one
// must fail the test loudly rather than be routed by a default.
func oraclePreimage(t *testing.T, db *sql.DB, r replayed) []byte {
	t.Helper()

	switch r.kind {
	case "planned":
		var venue, product, symbol, contract, side, orderType, tif string
		var qtyAsset, priceBase, priceQuote string
		var qtyMantissa, priceMantissa int64
		var reduceOnly int
		err := db.QueryRow(`SELECT venue, product, symbol, contract, side, order_type,
			time_in_force, qty_asset, qty_mantissa, price_base, price_quote,
			price_mantissa, reduce_only FROM journal_planned WHERE seq = ?`, r.seq).
			Scan(&venue, &product, &symbol, &contract, &side, &orderType, &tif,
				&qtyAsset, &qtyMantissa, &priceBase, &priceQuote, &priceMantissa, &reduceOnly)
		if err != nil {
			t.Fatalf("oracle planned detail at seq %d: %v", r.seq, err)
		}

		var b []byte
		b = append(b, 1) // kind tag, spelled as a literal
		b = append(b, oracleStr(r.clientID)...)
		b = append(b, oracleStr(venue)...)
		b = append(b, oracleStr(product)...)
		b = append(b, oracleStr(symbol)...)
		b = append(b, oracleStr(contract)...)
		b = append(b, oracleStr(side)...)
		b = append(b, oracleStr(orderType)...)
		b = append(b, oracleStr(tif)...)
		b = append(b, oracleStr(qtyAsset)...)
		b = append(b, oracleI64(qtyMantissa)...)
		b = append(b, oracleStr(priceBase)...)
		b = append(b, oracleStr(priceQuote)...)
		b = append(b, oracleI64(priceMantissa)...)
		b = append(b, byte(reduceOnly))
		return b

	case "state":
		var state string
		if err := db.QueryRow(`SELECT state FROM journal_state WHERE seq = ?`, r.seq).
			Scan(&state); err != nil {
			t.Fatalf("oracle state detail at seq %d: %v", r.seq, err)
		}
		var b []byte
		b = append(b, 2) // kind tag, spelled as a literal
		b = append(b, oracleStr(r.clientID)...)
		b = append(b, oracleStr(state)...)
		return b

	case "authorized":
		var instant, uncertainty int64
		if err := db.QueryRow(
			`SELECT instant_unix_nano, uncertainty_nano FROM journal_authorized WHERE seq = ?`, r.seq).
			Scan(&instant, &uncertainty); err != nil {
			t.Fatalf("oracle authorized detail at seq %d: %v", r.seq, err)
		}
		var b []byte
		b = append(b, 3) // kind tag, spelled as a literal
		b = append(b, oracleStr(r.clientID)...)
		b = append(b, oracleI64(instant)...)
		b = append(b, oracleI64(uncertainty)...)
		return b

	case "acked":
		var venueOrderID string
		if err := db.QueryRow(`SELECT venue_order_id FROM journal_acked WHERE seq = ?`, r.seq).
			Scan(&venueOrderID); err != nil {
			t.Fatalf("oracle acked detail at seq %d: %v", r.seq, err)
		}
		var b []byte
		b = append(b, 4) // kind tag, spelled as a literal
		b = append(b, oracleStr(r.clientID)...)
		b = append(b, oracleStr(venueOrderID)...)
		return b
	}

	t.Fatalf("oracle: record at seq %d has unknown kind %q", r.seq, r.kind)
	return nil
}

// oracleChainHash recomputes a link. The label is a literal, not chainLabel: a
// change to the production constant must show up here as a failure, because the
// constant's own comment says a change to the encoding MUST change it.
func oracleChainHash(seq int64, prev, preimage []byte) []byte {
	h := sha256.New()
	h.Write([]byte("arb-lab/oms/journal/chain/v1"))
	h.Write(oracleI64(seq))
	h.Write(prev)
	h.Write(preimage)
	return h.Sum(nil)
}

func oracleGenesis() []byte {
	sum := sha256.Sum256([]byte("arb-lab/oms/journal/chain/v1/genesis"))
	return sum[:]
}

// Hand-rolled big-endian, so the oracle and encoding/binary can disagree.
func oracleU32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func oracleI64(v int64) []byte {
	u := uint64(v)
	return []byte{
		byte(u >> 56), byte(u >> 48), byte(u >> 40), byte(u >> 32),
		byte(u >> 24), byte(u >> 16), byte(u >> 8), byte(u),
	}
}

func oracleStr(s string) []byte { return append(oracleU32(uint32(len(s))), s...) }

// verifyChain is the oracle's whole judgement, applied to a replay.
func verifyChain(t *testing.T, records []replayed) {
	t.Helper()

	prev := oracleGenesis()
	for i, r := range records {
		if want := int64(i + 1); r.seq != want {
			t.Errorf("record %d has seq %d, want %d (sequence must be dense from 1)", i, r.seq, want)
		}
		if !bytes.Equal(r.prevHash, prev) {
			t.Errorf("seq %d prev_hash %x, want %x", r.seq, r.prevHash, prev)
		}
		if want := oracleChainHash(r.seq, r.prevHash, r.preimage); !bytes.Equal(r.hash, want) {
			t.Errorf("seq %d hash %x, oracle computes %x", r.seq, r.hash, want)
		}
		prev = r.hash
	}
}

// ---------------------------------------------------------------------------
// open, pragmas, durability configuration
// ---------------------------------------------------------------------------

// TestOpenSetsTheDurabilityPragmasToTheirREQUIREDValues asserts the values as
// LITERALS, and that is the point of the test rather than an accident of how it
// is written.
//
// journalDSN and verifyPragmas both read wantSynchronous, deliberately, so they
// cannot drift apart. The cost of that sharing is that a mutation of the
// constant is self-consistent: the DSN would request NORMAL, the verification
// would demand NORMAL, and every test that only checks "open succeeds" passes
// with a journal that no longer fsyncs on commit. It is capture's own note
// about a wrong CRC polynomial — "a checksum that is self-consistently wrong
// verifies its own output perfectly" — arriving as a configuration constant.
//
// So the required values are named here, once, as the numbers §13 requires.
func TestOpenSetsTheDurabilityPragmasToTheirREQUIREDValues(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)

	var mode string
	if err := j.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode is %q, want %q — §13 requires WAL", mode, "wal")
	}

	var sync int
	if err := j.db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("PRAGMA synchronous: %v", err)
	}
	if sync != 2 {
		t.Errorf("synchronous is %d, want 2 (FULL) — at NORMAL, WAL mode fsyncs only "+
			"at checkpoint, so Append returns before the record is durable", sync)
	}

	var fk int
	if err := j.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys is %d, want 1 — otherwise the detail tables' "+
			"REFERENCES is a comment", fk)
	}
}

// TestVerifyPragmasRefusesAWeakenedDatabase is the negative half, and it is
// paired with a positive control in the same table: a harness that objects to
// everything reads as a perfect score.
func TestVerifyPragmasRefusesAWeakenedDatabase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pragmas []string
		wantErr bool
	}{
		{
			name:    "positive control: the pragmas Open actually sets",
			pragmas: []string{"journal_mode(wal)", "synchronous(2)", "foreign_keys(1)"},
			wantErr: false,
		},
		{
			name:    "synchronous NORMAL: commits without an fsync",
			pragmas: []string{"journal_mode(wal)", "synchronous(1)", "foreign_keys(1)"},
			wantErr: true,
		},
		{
			name:    "synchronous OFF",
			pragmas: []string{"journal_mode(wal)", "synchronous(0)", "foreign_keys(1)"},
			wantErr: true,
		},
		{
			name:    "rollback journal instead of WAL",
			pragmas: []string{"journal_mode(delete)", "synchronous(2)", "foreign_keys(1)"},
			wantErr: true,
		},
		{
			name:    "foreign keys off: REFERENCES stops biting",
			pragmas: []string{"journal_mode(wal)", "synchronous(2)", "foreign_keys(0)"},
			wantErr: true,
		},
		{
			name:    "no pragmas at all: the DSN was dropped on the floor",
			pragmas: nil,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "j.db")

			dsn := "file:" + path
			for i, p := range tc.pragmas {
				sep := "&"
				if i == 0 {
					sep = "?"
				}
				dsn += sep + "_pragma=" + p
			}
			db, err := sql.Open("sqlite", dsn)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			db.SetMaxOpenConns(1)
			defer func() { _ = db.Close() }()

			err = (&SQLiteJournal{db: db}).verifyPragmas()
			if tc.wantErr {
				if !errors.Is(err, ErrJournalPragma) {
					t.Errorf("verifyPragmas = %v, want ErrJournalPragma", err)
				}
				return
			}
			if err != nil {
				t.Errorf("verifyPragmas on a correctly configured database: %v", err)
			}
		})
	}
}

func TestOpenRefusesAnEmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := OpenSQLiteJournal(""); !errors.Is(err, ErrJournalPathUnset) {
		t.Errorf("OpenSQLiteJournal(%q) = %v, want ErrJournalPathUnset", "", err)
	}
}

// ---------------------------------------------------------------------------
// the chain: collapse dimension 1, chain length
// ---------------------------------------------------------------------------

// TestChainVerifiesAtEveryLength is collapse dimension 1.
//
// Lengths 1 and 2 are here to be insufficient, and the test says so: they run
// because a chain must verify at every length, not because either one could
// catch a broken link rule. Length 3 is the shortest chain on which "link to
// the previous record" and "link to the first record" give different answers,
// and TestLinkIsToThePredecessorNotToTheFirst is where that is asserted.
func TestChainVerifiesAtEveryLength(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 3, 9} {
		t.Run(fmt.Sprintf("length_%d", n), func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)

			// All FOUR kinds interleave — collapse dimension 2, re-run for the
			// second time now that a fourth exists. A chain of one kind makes
			// the kind tag a constant, and a chain that never reaches a kind
			// makes THAT tag's presence in the preimage unobservable, which is
			// the same defect one member along: the fixture would not contain
			// the thing under test. The longest length is 9 so the cycle
			// completes twice.
			for i := range n {
				switch i % 4 {
				case 0:
					if err := j.AppendPlanned(testOrder(t, fmt.Sprintf("c-%d", i), "binance", "BTCUSDT")); err != nil {
						t.Fatalf("AppendPlanned %d: %v", i, err)
					}
				case 1:
					if err := j.AppendAuthorized(ids.ClientOrderID(fmt.Sprintf("c-%d", i-1)),
						testAnchor(t, i)); err != nil {
						t.Fatalf("AppendAuthorized %d: %v", i, err)
					}
				case 2:
					if err := j.AppendAcked(ids.ClientOrderID(fmt.Sprintf("c-%d", i-2)),
						fmt.Sprintf("77%d", i)); err != nil {
						t.Fatalf("AppendAcked %d: %v", i, err)
					}
				case 3:
					if err := j.AppendState(ids.ClientOrderID(fmt.Sprintf("c-%d", i-3)),
						domain.StateSubmitted); err != nil {
						t.Fatalf("AppendState %d: %v", i, err)
					}
				}
			}

			records := oracleReplay(t, path)
			if len(records) != n {
				t.Fatalf("replayed %d records, want %d", len(records), n)
			}
			verifyChain(t, records)
		})
	}
}

// TestLinkIsToThePredecessorNotToTheFirst is the isolating check the length-1
// and length-2 fixtures cannot make.
//
// It asserts the inequality as well as the equality. The equality alone —
// prev_hash[3] == hash[2] — is satisfied by an implementation that links to the
// first record on a chain of two, which is where a hurried fixture stops.
func TestLinkIsToThePredecessorNotToTheFirst(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	for i := range 3 {
		if err := j.AppendState(ids.ClientOrderID(fmt.Sprintf("c-%d", i)),
			domain.StateSubmitted); err != nil {
			t.Fatalf("AppendState %d: %v", i, err)
		}
	}

	r := oracleReplay(t, path)
	if len(r) != 3 {
		t.Fatalf("replayed %d records, want 3", len(r))
	}

	if !bytes.Equal(r[2].prevHash, r[1].hash) {
		t.Errorf("seq 3 links to %x, want its predecessor seq 2 (%x)", r[2].prevHash, r[1].hash)
	}
	if bytes.Equal(r[2].prevHash, r[0].hash) {
		t.Errorf("seq 3 links to seq 1's hash %x — the chain links to the FIRST "+
			"record, not the previous one; a chain of two cannot tell these apart",
			r[0].hash)
	}
	if bytes.Equal(r[2].prevHash, oracleGenesis()) {
		t.Errorf("seq 3 links to the genesis seed — every record is a first record")
	}
}

// TestTamperInvalidatesEveryLaterRecord is what prev_hash buys over a per-row
// checksum, and it is unobservable on a chain shorter than three: the claim is
// that editing record 1 breaks record 3, which needs a record 3 that is not
// also record 1's immediate successor.
//
// The tamper has to DROP A TRIGGER first, and that is worth seeing in a test
// rather than only in the schema: append-only here is enforced by the database,
// so corrupting a record is not something a stray UPDATE can do by accident.
func TestTamperInvalidatesEveryLaterRecord(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	for i := range 3 {
		if err := j.AppendState(ids.ClientOrderID(fmt.Sprintf("c-%d", i)),
			domain.StateSubmitted); err != nil {
			t.Fatalf("AppendState %d: %v", i, err)
		}
	}
	before := oracleReplay(t, path)
	verifyChain(t, before) // positive control: it verified before the tamper

	tamperClientID(t, path, 1, "c-0-EDITED")

	after := oracleReplay(t, path)
	if len(after) != 3 {
		t.Fatalf("replayed %d records after tamper, want 3", len(after))
	}

	// Record 1's own hash no longer matches its contents — that much a
	// per-row checksum would also catch.
	if want := oracleChainHash(after[0].seq, after[0].prevHash, after[0].preimage); bytes.Equal(after[0].hash, want) {
		t.Error("seq 1 still verifies against its own edited contents")
	}

	// The chain's contribution: records 2 and 3 are untouched rows whose
	// stored prev_hash still names the OLD hash of record 1. A verifier
	// recomputing forward from the tampered record 1 gets a different hash and
	// the mismatch propagates. Assert it at record 3 specifically, because
	// record 2 would fail under a first-record link too.
	recomputed := oracleChainHash(after[0].seq, after[0].prevHash, after[0].preimage)
	if bytes.Equal(after[1].prevHash, recomputed) {
		t.Error("seq 2 links to the RECOMPUTED hash of the edited record — " +
			"the tamper did not propagate")
	}
	if bytes.Equal(after[2].prevHash, oracleChainHash(after[1].seq, recomputed, after[1].preimage)) {
		t.Error("seq 3 verifies against a chain recomputed from the edited record")
	}
}

// ---------------------------------------------------------------------------
// the preimage: collapse dimensions 3 and 4
// ---------------------------------------------------------------------------

// TestIdenticalRecordsHashDifferently is collapse dimension 3, and the fixture
// is not contrived: §5.3 puts the same execution on two streams and has REST
// recovery re-reading what the private stream delivered, so a duplicate
// AppendState is the ORDINARY case several times per fill.
//
// Without the sequence in the preimage these two records hash alike, and a
// chain segment holding both can have one dropped or the two reordered with
// every remaining link still verifying.
func TestIdenticalRecordsHashDifferently(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	for range 2 {
		if err := j.AppendState("c-same", domain.StateSubmitted); err != nil {
			t.Fatalf("AppendState: %v", err)
		}
	}

	r := oracleReplay(t, path)
	if len(r) != 2 {
		t.Fatalf("replayed %d records, want 2", len(r))
	}
	if !bytes.Equal(r[0].preimage, r[1].preimage) {
		t.Fatalf("fixture is broken: the two records must be byte-identical, got %x and %x",
			r[0].preimage, r[1].preimage)
	}
	if bytes.Equal(r[0].hash, r[1].hash) {
		t.Error("two byte-identical records at different sequences share a hash — " +
			"the sequence is not in the preimage")
	}
	verifyChain(t, r)
}

// TestFieldSplitDoesNotCollide is collapse dimension 4 — the length prefixes.
//
// Both cases move the boundary between two ADJACENT strings while keeping the
// concatenation identical, and both keep every field non-empty on both sides.
// That last part is the whole lesson from observability's series-key fixture:
// ("ab","c") against ("a","bc") with no separator collides only under the EMPTY
// separator, so it passes against every wrong choice a person would make.
func TestFieldSplitDoesNotCollide(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		aID, aVen  string
		bID, bVen  string
		concatSame string
	}{
		{
			name: "client id / venue boundary moves left",
			aID:  "AB", aVen: "CDbinance",
			bID: "ABC", bVen: "Dbinance",
			concatSame: "ABCDbinance",
		},
		{
			name: "client id / venue boundary moves right",
			aID:  "orderX", aVen: "bybit",
			bID: "order", bVen: "Xbybit",
			concatSame: "orderXbybit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.aID+tc.aVen != tc.concatSame || tc.bID+tc.bVen != tc.concatSame {
				t.Fatalf("fixture is broken: the two concatenations must be equal, "+
					"got %q and %q", tc.aID+tc.aVen, tc.bID+tc.bVen)
			}
			if tc.aID == "" || tc.aVen == "" || tc.bID == "" || tc.bVen == "" {
				t.Fatal("fixture is broken: an empty side collides only under the " +
					"empty separator, which is observability's defect")
			}

			a := encodePlanned(testOrder(t, tc.aID, tc.aVen, "BTCUSDT"))
			b := encodePlanned(testOrder(t, tc.bID, tc.bVen, "BTCUSDT"))
			if bytes.Equal(a, b) {
				t.Errorf("two orders differing only in where the client id ends and "+
					"the venue begins share a preimage:\n  %x", a)
			}
		})
	}
}

// TestPlannedAndStateRecordsCannotCollide checks the kind tag's own job.
func TestPlannedAndStateRecordsCannotCollide(t *testing.T) {
	t.Parallel()

	planned := encodePlanned(testOrder(t, "c-1", "binance", "BTCUSDT"))
	state := encodeState("c-1", domain.StatePlanned)
	if bytes.Equal(planned, state) {
		t.Error("a planned record and a state record for the same order share a preimage")
	}
	if planned[0] == state[0] {
		t.Errorf("both kinds carry tag %d — the tag is not discriminating", planned[0])
	}
}

// TestAckedAndStateRecordsCannotCollide is the pair the fourth kind makes the
// tag load-bearing for a second time: both encodings are a client id followed
// by one length-prefixed string, so a venue id spelling a state name is
// byte-identical after the tag. The fixture takes that worst case rather than
// a realistic id — a numeric orderId differs from every state name and would
// pass with the tags deleted.
func TestAckedAndStateRecordsCannotCollide(t *testing.T) {
	t.Parallel()

	state := encodeState("c-1", domain.StateSubmitted)
	acked := encodeAcked("c-1", string(domain.StateSubmitted))
	if bytes.Equal(state, acked) {
		t.Error("a state record and an acked record with identical fields share a preimage")
	}
	if state[0] == acked[0] {
		t.Errorf("both kinds carry tag %d — the tag is not discriminating", state[0])
	}
	if !bytes.Equal(state[1:], acked[1:]) {
		t.Error("fixture is broken: the two payloads must be identical past the tag, " +
			"or the test is not about the tag")
	}
}

// ---------------------------------------------------------------------------
// append-only, enforced by the database
// ---------------------------------------------------------------------------

func TestJournalIsAppendOnly(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	if err := j.AppendPlanned(testOrder(t, "c-1", "binance", "BTCUSDT")); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	if err := j.AppendState("c-1", domain.StateSubmitted); err != nil {
		t.Fatalf("AppendState: %v", err)
	}
	if err := j.AppendAuthorized("c-1", testAnchor(t, 1)); err != nil {
		t.Fatalf("AppendAuthorized: %v", err)
	}
	if err := j.AppendAcked("c-1", "774411"); err != nil {
		t.Fatalf("AppendAcked: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	mutations := []struct {
		name string
		stmt string
	}{
		{"update the chain row", `UPDATE journal SET client_id = 'x' WHERE seq = 1`},
		{"delete the chain row", `DELETE FROM journal WHERE seq = 1`},
		{"update a planned detail", `UPDATE journal_planned SET symbol = 'x' WHERE seq = 1`},
		{"delete a planned detail", `DELETE FROM journal_planned WHERE seq = 1`},
		{"update a state detail", `UPDATE journal_state SET state = 'filled' WHERE seq = 2`},
		{"delete a state detail", `DELETE FROM journal_state WHERE seq = 2`},
		{"update an authorized detail", `UPDATE journal_authorized SET instant_unix_nano = 0 WHERE seq = 3`},
		{"delete an authorized detail", `DELETE FROM journal_authorized WHERE seq = 3`},
		{"update an acked detail", `UPDATE journal_acked SET venue_order_id = 'x' WHERE seq = 4`},
		{"delete an acked detail", `DELETE FROM journal_acked WHERE seq = 4`},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			if _, err := db.Exec(m.stmt); err == nil {
				t.Errorf("%s succeeded; the journal is append-only and the "+
					"database is supposed to say so", m.stmt)
			}
		})
	}

	// Positive control: the guard is not simply refusing every statement.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM journal`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 4 {
		t.Errorf("journal holds %d records, want 4 — reads must still work", n)
	}
}

// ---------------------------------------------------------------------------
// reopen: the chain continues from what is on disk
// ---------------------------------------------------------------------------

// TestReopenContinuesTheChain is the behavioural half of the durability claim.
//
// It proves the commit, not the fsync — a process boundary is not a power cut.
// That limit is stated in the type comment rather than papered over here,
// because a test named for durability that only proves commit is worse than no
// test: it retires the question.
//
// What it does prove is the part an in-memory chain head would break: a second
// process reads the head from the table and extends it, rather than starting a
// fresh chain from the genesis seed over the top of existing records.
func TestReopenContinuesTheChain(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "journal.db")

	first, err := OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := first.AppendPlanned(testOrder(t, "c-1", "binance", "BTCUSDT")); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	if err := first.AppendAuthorized("c-1", testAnchor(t, 0)); err != nil {
		t.Fatalf("AppendAuthorized: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = second.Close() }()
	if err := second.AppendState("c-1", domain.StateSubmitted); err != nil {
		t.Fatalf("AppendState after reopen: %v", err)
	}

	r := oracleReplay(t, path)
	if len(r) != 3 {
		t.Fatalf("replayed %d records, want 3 — a reopen started a new chain", len(r))
	}
	verifyChain(t, r)
}

// ---------------------------------------------------------------------------
// refusals
// ---------------------------------------------------------------------------

func TestAppendPlannedRefusesAnIllFormedOrder(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	bad := testOrder(t, "c-1", "binance", "BTCUSDT")
	bad.Side = "sideways"

	if err := j.AppendPlanned(bad); err == nil {
		t.Fatal("AppendPlanned accepted an order with an undeclared side")
	}
	if r := oracleReplay(t, path); len(r) != 0 {
		t.Errorf("a refused append left %d records behind", len(r))
	}

	// Positive control: the same order, well formed, is accepted.
	if err := j.AppendPlanned(testOrder(t, "c-1", "binance", "BTCUSDT")); err != nil {
		t.Errorf("AppendPlanned refused a well-formed order: %v", err)
	}
}

func TestAppendStateRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		id    ids.ClientOrderID
		state domain.OrderState
		want  error
	}{
		{"positive control", "c-1", domain.StateSubmitted, nil},
		{"no client order id", "", domain.StateSubmitted, ErrJournalClientIDUnset},
		{"unset state", "c-1", "", domain.ErrOrderStateUnset},
		{"undeclared state", "c-1", "nearly_filled", domain.ErrOrderStateUnknown},

		// The crash boundary has its own method and its own record kind, so
		// this one is refused on the TARGET alone — the mirror of
		// MemoryOrderWriteRepository.ApplyState, in the store §7.5 replays.
		// Without it the log could hold an authorization with no anchor while
		// the projection could not, which is where slice D's fix stopped.
		{"the authorization target", "c-1", domain.StateDispatchAuthorized, ports.ErrAuthorizationNeedsAnchor},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)

			err := j.AppendState(tc.id, tc.state)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("AppendState: %v", err)
				}
				if r := oracleReplay(t, path); len(r) != 1 {
					t.Errorf("accepted append wrote %d records, want 1", len(r))
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("AppendState = %v, want %v", err, tc.want)
			}
			if r := oracleReplay(t, path); len(r) != 0 {
				t.Errorf("a refused append left %d records behind", len(r))
			}
		})
	}
}

func TestAppendAckedRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		id           ids.ClientOrderID
		venueOrderID string
		want         error
	}{
		{"positive control", "c-1", "774411", nil},
		{"no client order id", "", "774411", ErrJournalClientIDUnset},

		// The empty venue id is the refusal this method exists to state: the
		// absent id's one spelling is the record's absence, and a journaled
		// empty string would be a second one — replayed into a field every
		// attribution rule ignores.
		{"no venue order id", "c-1", "", ErrJournalVenueOrderIDUnset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)

			err := j.AppendAcked(tc.id, tc.venueOrderID)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("AppendAcked: %v", err)
				}
				if r := oracleReplay(t, path); len(r) != 1 {
					t.Errorf("accepted append wrote %d records, want 1", len(r))
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("AppendAcked = %v, want %v", err, tc.want)
			}
			if r := oracleReplay(t, path); len(r) != 0 {
				t.Errorf("a refused append left %d records behind", len(r))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers that break things on purpose
// ---------------------------------------------------------------------------

// tamperClientID edits a committed record, which means dropping the trigger
// that forbids it first.
func tamperClientID(t *testing.T, path string, seq int64, newID string) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("tamper open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`DROP TRIGGER journal_no_update`); err != nil {
		t.Fatalf("tamper: drop trigger: %v", err)
	}
	res, err := db.Exec(`UPDATE journal SET client_id = ? WHERE seq = ?`, newID, seq)
	if err != nil {
		t.Fatalf("tamper: update: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("tamper: updated %d rows (err %v), want 1", n, err)
	}
}

// ---------------------------------------------------------------------------
// Head / HashAt — the chain mark (#179's baseline binding)
// ---------------------------------------------------------------------------

// TestHeadAndHashAtNameTheSameRecord appends THREE records, so head, first and
// "the one before head" are three different sequences — a HashAt reading the
// head regardless of its argument, or Head reporting the first record, cannot
// agree with the per-seq reads below.
func TestHeadAndHashAtNameTheSameRecord(t *testing.T) {
	t.Parallel()

	j, _ := openTestJournal(t)
	for i := 0; i < 3; i++ {
		if err := j.AppendPlanned(testOrder(t, fmt.Sprintf("c-%d", i), "binance", "BTCUSDT")); err != nil {
			t.Fatalf("AppendPlanned %d: %v", i, err)
		}
	}

	seq, headHash, err := j.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if seq != 3 {
		t.Fatalf("Head seq = %d after three appends, want 3", seq)
	}
	at3, err := j.HashAt(3)
	if err != nil {
		t.Fatalf("HashAt(3): %v", err)
	}
	if at3 != headHash {
		t.Error("HashAt(head) disagrees with Head's own hash")
	}
	at1, err := j.HashAt(1)
	if err != nil {
		t.Fatalf("HashAt(1): %v", err)
	}
	at2, err := j.HashAt(2)
	if err != nil {
		t.Fatalf("HashAt(2): %v", err)
	}
	if at1 == headHash || at2 == headHash || at1 == at2 {
		t.Error("distinct sequences report identical hashes — HashAt is not reading per-seq")
	}
}

// TestHeadOfAnEmptyJournalIsSeqZero: the zero sequence is the deliberate
// "attested nothing" carrier, not an error.
func TestHeadOfAnEmptyJournalIsSeqZero(t *testing.T) {
	t.Parallel()

	j, _ := openTestJournal(t)
	seq, hash, err := j.Head()
	if err != nil {
		t.Fatalf("Head on empty: %v", err)
	}
	if seq != 0 || hash != [32]byte{} {
		t.Errorf("Head on empty = (%d, %x), want (0, zero)", seq, hash)
	}
}

// TestHashAtRefusesBeyondHeadAndZero: a mark past the head is the named
// truncation refusal, and it must fire at head+1 — the tightest boundary —
// not merely at some huge sequence; zero is the absence of a sequence and
// gets the same sentinel.
func TestHashAtRefusesBeyondHeadAndZero(t *testing.T) {
	t.Parallel()

	j, _ := openTestJournal(t)
	if err := j.AppendPlanned(testOrder(t, "c-0", "binance", "BTCUSDT")); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	if _, err := j.HashAt(2); !errors.Is(err, ErrJournalMarkBeyondHead) {
		t.Errorf("HashAt(head+1) = %v, want ErrJournalMarkBeyondHead", err)
	}
	if _, err := j.HashAt(1); err != nil {
		t.Errorf("HashAt(head) = %v, want nil — the boundary is strictly past the head", err)
	}
	if _, err := j.HashAt(0); !errors.Is(err, ErrJournalMarkBeyondHead) {
		t.Errorf("HashAt(0) = %v, want ErrJournalMarkBeyondHead", err)
	}
}
