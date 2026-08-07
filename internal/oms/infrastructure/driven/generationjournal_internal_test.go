package driven

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The generation half's checks.
//
// # The collapse checklist, run before the mutations, on the dimensions THIS
// # slice added
//
// journal_internal_test.go's four still apply and are not restated. Six more
// arrive with SM §7's records, and every one of them is a value a natural
// fixture pins:
//
//  1. THE LEG PAIR. A perp submission and a spot submission encode as (tag, id,
//     order) — byte-identical but for one leading byte. A fixture writing ONE
//     leg cannot see the two tags swapped, and a fixture writing both with the
//     SAME order id cannot see the two DETAIL rows swapped. Both variants are
//     below, and the same-id one is the isolating fixture for the tag.
//  2. GENERATION CARDINALITY. With one generation, "this generation's records"
//     and "the first generation's" are the same set, so the unknown-generation
//     refusal and the live/ended split are both untestable. Two, always.
//  3. THE ENDING KIND. A generation ended by `Complete` and one ended by
//     `Superseded` travel different methods to the same conclusion. A fixture
//     using one ending makes the other's arm dead.
//  4. THE AUTHORIZED ZERO. A nonzero size everywhere leaves "writes the size"
//     and "writes a constant" indistinguishable from the legal zero's side, and
//     the zero is the record the port's whole six-method design is about.
//  5. THE OPENING'S ADJACENT FREE STRINGS. account|venue are two adjacent
//     length-prefixed fields, so ("ab","cd") and ("a","bcd") are the pair whose
//     agreement would prove the prefixes absent.
//  6. THE STORED COLUMN NAME. Every fixture that opens a FRESH journal gets the
//     subject column from the DDL and never exercises the migration at all — so
//     the one fixture that matters builds the OLD shape by hand.

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

func testGroup(account, venue, symbol string) domain.HedgeGroup {
	return domain.HedgeGroup{Account: account, Venue: ids.Venue(venue), Symbol: symbol}
}

// testQty is a quantity whose mantissa a caller chooses, so no fixture pins the
// number that would make a swapped column invisible.
func testQty(t *testing.T, asset, decimal string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), decimal)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %s): %v", asset, decimal, err)
	}
	return q
}

// writeLadder appends a full generation ladder INTERLEAVED with order records,
// and returns the journal's path.
//
// Interleaved deliberately, and this is checklist item 2 from the file one over
// arriving at generation scope: a chain of nothing but generation records makes
// every order-record arm of the replay dead, and a chain of nothing but order
// records is what the suite already had. The mix is also the realistic log —
// a generation's two legs ARE ordinary orders, journaled by §6 step 5's planner
// before the Deleverer ever references them.
func writeLadder(t *testing.T) (*SQLiteJournal, string) {
	t.Helper()
	j, path := openTestJournal(t)

	const gen = domain.GenerationID("GEN-1")
	group := testGroup("main", "binance", "BTCUSDT")

	perp := testOrder(t, "PERP-LEG", "binance", "BTCUSDT")
	spot := testOrder(t, "SPOT-LEG", "binance", "BTCUSDT")

	if err := j.AppendPlanned(perp); err != nil {
		t.Fatalf("AppendPlanned(perp): %v", err)
	}
	if err := j.AppendGenerationOpened(gen, group, "RES-1", testQty(t, "BTC", "0.75")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendGenerationPerpSubmitted(gen, perp.ClientOrderID); err != nil {
		t.Fatalf("AppendGenerationPerpSubmitted: %v", err)
	}
	if err := j.AppendState(perp.ClientOrderID, domain.StateFilled); err != nil {
		t.Fatalf("AppendState(perp): %v", err)
	}
	if err := j.AppendGenerationPhase(gen, domain.GenerationPerpReduceFinal); err != nil {
		t.Fatalf("AppendGenerationPhase(final): %v", err)
	}
	if err := j.AppendGenerationAuthorized(gen, testQty(t, "BTC", "0.5")); err != nil {
		t.Fatalf("AppendGenerationAuthorized: %v", err)
	}
	if err := j.AppendPlanned(spot); err != nil {
		t.Fatalf("AppendPlanned(spot): %v", err)
	}
	if err := j.AppendGenerationSpotSubmitted(gen, spot.ClientOrderID); err != nil {
		t.Fatalf("AppendGenerationSpotSubmitted: %v", err)
	}
	if err := j.AppendGenerationPhase(gen, domain.GenerationComplete); err != nil {
		t.Fatalf("AppendGenerationPhase(complete): %v", err)
	}
	return j, path
}

// ---------------------------------------------------------------------------
// one chain
// ---------------------------------------------------------------------------

// TestGenerationRecordsExtendTheOneChain is the slice's headline claim checked
// against the independent oracle: SM §7's records are links in §5.1's chain, not
// rows in a second store.
func TestGenerationRecordsExtendTheOneChain(t *testing.T) {
	t.Parallel()
	_, path := writeLadder(t)

	records := oracleReplay(t, path)
	if len(records) != 9 {
		t.Fatalf("journal holds %d records, want 9", len(records))
	}
	verifyChain(t, records)

	// The chain must interleave, or checklist item 2 collapses and this test
	// says nothing about the kind tags.
	var orderKinds, genKinds int
	for _, r := range records {
		switch r.kind {
		case kindPlanned, kindState, kindAuthorized, kindAcked:
			orderKinds++
		default:
			genKinds++
		}
	}
	if orderKinds == 0 || genKinds == 0 {
		t.Fatalf("fixture is not interleaved: %d order records, %d generation records", orderKinds, genKinds)
	}
}

// TestTheAuthorizationRecordPrecedesTheSpotLegInOneChain is the reason one chain
// was chosen over two, stated as an assertion.
//
// FLAG-3 makes `SpotReduceAuthorized` durable "so recovery knows the
// perp-finality precondition was met without re-derivation", and the fact
// recovery most needs is that the authorization was written BEFORE the spot leg
// went out. Under two chains this test could not be written at all: the two
// records would sit in stores with no order between them, each verifying
// internally and neither positioning the other.
//
// It compares the authorization against BOTH the generation's spot submission
// and the spot order's own planned record, because the second is the one that
// matters operationally — the planned record is what §6 step 5 writes before the
// order can be sent, so it is the boundary a crash can land inside.
func TestTheAuthorizationRecordPrecedesTheSpotLegInOneChain(t *testing.T) {
	t.Parallel()
	_, path := writeLadder(t)

	records := oracleReplay(t, path)
	verifyChain(t, records)

	seqOf := func(kind, subject string) int64 {
		t.Helper()
		var found []int64
		for _, r := range records {
			if r.kind == kind && r.subject == subject {
				found = append(found, r.seq)
			}
		}
		if len(found) != 1 {
			t.Fatalf("journal holds %d %s records for %q, want exactly 1", len(found), kind, subject)
		}
		return found[0]
	}

	authorized := seqOf(kindGenerationAuthorized, "GEN-1")
	spotSubmitted := seqOf(kindGenerationSpotSubmitted, "GEN-1")
	spotPlanned := seqOf(kindPlanned, "SPOT-LEG")

	if authorized >= spotPlanned {
		t.Errorf("authorization at seq %d does not precede the spot order's planned record at seq %d",
			authorized, spotPlanned)
	}
	if authorized >= spotSubmitted {
		t.Errorf("authorization at seq %d does not precede the spot submission at seq %d",
			authorized, spotSubmitted)
	}
}

// ---------------------------------------------------------------------------
// the kind tag, and the pair it now separates
// ---------------------------------------------------------------------------

// TestTheTwoLegKindsCannotCollide is the isolating fixture for the kind tag, and
// the fixture design is the whole of it.
//
// Both leg records carry the SAME generation and the SAME order id, so their
// preimages are identical in every field. If the tag were shared — or dropped —
// the two would hash alike, and a chain segment holding both could have either
// one removed with every remaining link still verifying. The realistic fixture
// (different ids per leg) would NOT catch that: the ids differ, so the hashes
// differ, and the test would pass against a codec with no tag at all.
//
// It is the same argument the file one over makes for the planned/state pair,
// arriving where it now matters most — these two records decide which leg
// GenerationPhase.ReservedLegLive is asked about.
func TestTheTwoLegKindsCannotCollide(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	const gen = domain.GenerationID("GEN-1")
	const order = ids.ClientOrderID("SAME-ORDER")

	if err := j.AppendGenerationOpened(gen, testGroup("main", "binance", "BTCUSDT"),
		"RES-1", testQty(t, "BTC", "1")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendGenerationPerpSubmitted(gen, order); err != nil {
		t.Fatalf("AppendGenerationPerpSubmitted: %v", err)
	}
	if err := j.AppendGenerationSpotSubmitted(gen, order); err != nil {
		t.Fatalf("AppendGenerationSpotSubmitted: %v", err)
	}

	records := oracleReplay(t, path)
	verifyChain(t, records)

	perp, spot := records[1], records[2]
	if perp.kind != kindGenerationPerpSubmitted || spot.kind != kindGenerationSpotSubmitted {
		t.Fatalf("fixture wrote kinds %q, %q", perp.kind, spot.kind)
	}
	if perp.subject != spot.subject {
		t.Fatalf("fixture varies the subject (%q vs %q); the collision it tests needs them equal",
			perp.subject, spot.subject)
	}
	if bytes.Equal(perp.preimage, spot.preimage) {
		t.Errorf("the two leg records hash the same preimage %x; the kind tag does not separate them",
			perp.preimage)
	}

	// And the tag is the ONLY difference, which is what makes the assertion
	// above a statement about the tag rather than about some other field.
	if len(perp.preimage) != len(spot.preimage) {
		t.Fatalf("preimages differ in length (%d vs %d), so the difference is not the tag alone",
			len(perp.preimage), len(spot.preimage))
	}
	if !bytes.Equal(perp.preimage[1:], spot.preimage[1:]) {
		t.Errorf("preimages differ past the tag byte; the fixture is not isolating the tag")
	}
}

// TestSupersededAndPhaseRecordsCannotCollide is the second same-shape pair the
// generation kinds introduce — both are (tag, id, one string).
//
// The strings are chosen so that the two records would be indistinguishable
// under a shared tag: a supersession path and a phase are both lowercase
// snake_case domain words, and nothing but the tag says which vocabulary a
// stored string belongs to.
func TestSupersededAndPhaseRecordsCannotCollide(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	const gen = domain.GenerationID("GEN-1")
	if err := j.AppendGenerationOpened(gen, testGroup("main", "binance", "BTCUSDT"),
		"RES-1", testQty(t, "BTC", "1")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendGenerationPhase(gen, domain.GenerationHeldSpot); err != nil {
		t.Fatalf("AppendGenerationPhase: %v", err)
	}
	if err := j.AppendGenerationSuperseded(gen, domain.SupersededByPremiseLost); err != nil {
		t.Fatalf("AppendGenerationSuperseded: %v", err)
	}

	records := oracleReplay(t, path)
	verifyChain(t, records)

	for i, r := range records {
		for k, other := range records {
			if i >= k {
				continue
			}
			if bytes.Equal(r.preimage, other.preimage) {
				t.Errorf("records %d and %d share a preimage %x", r.seq, other.seq, r.preimage)
			}
		}
	}
}

// TestGenerationOpeningFieldSplitDoesNotCollide is checklist item 5: the
// opening record's account and venue are adjacent length-prefixed strings, so
// two openings that differ only in where the first ends must not hash alike.
//
// BOTH sides are non-empty on BOTH records, which is the correction
// observability's collision fixture earned: a pair with an empty side collides
// only under the EMPTY separator and passes against every wrong prefix a person
// would actually write.
func TestGenerationOpeningFieldSplitDoesNotCollide(t *testing.T) {
	t.Parallel()

	preimageOf := func(t *testing.T, account, venue string) []byte {
		t.Helper()
		j, path := openTestJournal(t)
		if err := j.AppendGenerationOpened("GEN-1", testGroup(account, venue, "BTCUSDT"),
			"RES-1", testQty(t, "BTC", "1")); err != nil {
			t.Fatalf("AppendGenerationOpened(%q, %q): %v", account, venue, err)
		}
		records := oracleReplay(t, path)
		if len(records) != 1 {
			t.Fatalf("journal holds %d records, want 1", len(records))
		}
		return records[0].preimage
	}

	left := preimageOf(t, "ab", "cd")
	right := preimageOf(t, "a", "bcd")
	if bytes.Equal(left, right) {
		t.Errorf("(%q,%q) and (%q,%q) hash the same preimage %x; the length prefixes are not there",
			"ab", "cd", "a", "bcd", left)
	}
}

// ---------------------------------------------------------------------------
// the authorization, and its legal zero
// ---------------------------------------------------------------------------

// TestAZeroAuthorizedSizeIsALegalRecordThatReplays is the port's central claim
// checked at the store: a perp reduction that confirmed nothing authorizes no
// spot sale, so zero is a real terminal decision rather than a missing field.
//
// Both sizes are written into ONE journal, which is checklist item 4. A suite
// asserting only the zero would pass against a codec that wrote a constant
// zero, and one asserting only the nonzero would never reach the value the
// six-method port design exists to make representable.
func TestAZeroAuthorizedSizeIsALegalRecordThatReplays(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	if err := j.AppendGenerationOpened("GEN-ZERO", testGroup("main", "binance", "BTCUSDT"),
		"RES-1", testQty(t, "BTC", "1")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendGenerationAuthorized("GEN-ZERO", fixedpoint.ZeroQuantity("BTC")); err != nil {
		t.Fatalf("AppendGenerationAuthorized(zero): %v", err)
	}
	if err := j.AppendGenerationOpened("GEN-SOME", testGroup("main", "binance", "ETHUSDT"),
		"RES-2", testQty(t, "ETH", "3")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendGenerationAuthorized("GEN-SOME", testQty(t, "ETH", "2.25")); err != nil {
		t.Fatalf("AppendGenerationAuthorized(nonzero): %v", err)
	}

	verifyChain(t, oracleReplay(t, path))

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	sizes := map[domain.GenerationID]fixedpoint.Quantity{}
	for _, c := range v.genCalls {
		if c.kind == kindGenerationAuthorized {
			sizes[c.id] = c.size
		}
	}
	if got := sizes["GEN-ZERO"]; got.Mantissa() != 0 || got.Asset() != "BTC" {
		t.Errorf("zero authorization replayed as %s of %q, want 0 of BTC", got.Decimal(), got.Asset())
	}
	// Asserted positively rather than as "not zero": a negative assertion is
	// satisfied by any wrong number, and the mantissa is the value a swapped
	// column would carry.
	want := testQty(t, "ETH", "2.25")
	if got := sizes["GEN-SOME"]; got.Mantissa() != want.Mantissa() || got.Asset() != "ETH" {
		t.Errorf("authorization replayed as %s of %q, want %s of ETH",
			got.Decimal(), got.Asset(), want.Decimal())
	}
}

// ---------------------------------------------------------------------------
// AppendGenerationPhase's payload refusal
// ---------------------------------------------------------------------------

// TestAppendGenerationPhaseRefusesThePayloadPhases pairs every refusal with the
// positive control the repo's rule requires: a universally-objecting method
// would otherwise read as a perfect score.
//
// The accepted list is exhaustive over the five this method owns rather than a
// sample, because that is the half that grows — and each of the two ENDING
// phases is in it, which is what application.rebuild's liveness rule depends on.
func TestAppendGenerationPhaseRefusesThePayloadPhases(t *testing.T) {
	t.Parallel()

	refused := []domain.GenerationPhase{
		domain.GenerationPlanned,
		domain.GenerationPerpReduceInFlight,
		domain.GenerationSpotReduceAuthorized,
		domain.GenerationSpotReduceInFlight,
		domain.GenerationSuperseded,
	}
	accepted := []domain.GenerationPhase{
		domain.GenerationPerpReduceFinal,
		domain.GenerationHeldPerp,
		domain.GenerationHeldSpot,
		domain.GenerationComplete,
		domain.GenerationTerminalFlatten,
	}

	for _, phase := range refused {
		t.Run("refuses "+phase.String(), func(t *testing.T) {
			t.Parallel()
			j, _ := openTestJournal(t)
			err := j.AppendGenerationPhase("GEN-1", phase)
			if !errors.Is(err, ports.ErrGenerationPhaseNeedsPayload) {
				t.Fatalf("AppendGenerationPhase(%s) = %v, want ErrGenerationPhaseNeedsPayload", phase, err)
			}
		})
	}

	for _, phase := range accepted {
		t.Run("accepts "+phase.String(), func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)
			if err := j.AppendGenerationPhase("GEN-1", phase); err != nil {
				t.Fatalf("AppendGenerationPhase(%s) = %v, want nil", phase, err)
			}
			records := oracleReplay(t, path)
			if len(records) != 1 {
				t.Fatalf("journal holds %d records, want 1", len(records))
			}
			verifyChain(t, records)
		})
	}

	// The two lists must PARTITION the declared phases, or one of them is a
	// sample and a phase added later lands in neither while both tests pass.
	if got := len(refused) + len(accepted); got != 10 {
		t.Fatalf("the two lists name %d phases; domain declares 10", got)
	}
	seen := map[domain.GenerationPhase]bool{}
	for _, p := range append(append([]domain.GenerationPhase{}, refused...), accepted...) {
		if err := p.Validate(); err != nil {
			t.Errorf("%s is not a declared phase: %v", p, err)
		}
		if seen[p] {
			t.Errorf("%s appears in both lists", p)
		}
		seen[p] = true
	}
}

// TestEveryEndedGenerationPhaseIsReachableThroughAnEndingRecord is the claim
// application.rebuild's liveness rule rests on, made checkable here because this
// is where the reachability is decided.
//
// rebuild does not map record kinds to phases — that would be a second home for
// SM §7's table. It says instead that a generation is live unless the log
// records an ENDING, where an ending is a phase whose IsActive is false. That
// only works if every such phase can actually arrive: `Complete` and
// `TerminalFlatten` through AppendGenerationPhase, `Superseded` through its own
// kind. If a later slice made an ended phase payload-carrying, this fails and
// the liveness rule is revisited rather than silently under-reporting.
func TestEveryEndedGenerationPhaseIsReachableThroughAnEndingRecord(t *testing.T) {
	t.Parallel()

	all := []domain.GenerationPhase{
		domain.GenerationPlanned,
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
		domain.GenerationSpotReduceAuthorized,
		domain.GenerationSpotReduceInFlight,
		domain.GenerationHeldPerp,
		domain.GenerationHeldSpot,
		domain.GenerationComplete,
		domain.GenerationTerminalFlatten,
		domain.GenerationSuperseded,
	}

	var ended int
	for _, phase := range all {
		active, err := phase.IsActive()
		if err != nil {
			t.Fatalf("%s.IsActive: %v", phase, err)
		}
		if active {
			continue
		}
		ended++

		if phase == domain.GenerationSuperseded {
			// Reachable through its own kind, which carries the path.
			j, _ := openTestJournal(t)
			if err := j.AppendGenerationSuperseded("GEN-1", domain.SupersededByPremiseLost); err != nil {
				t.Errorf("AppendGenerationSuperseded: %v", err)
			}
			continue
		}
		j, _ := openTestJournal(t)
		if err := j.AppendGenerationPhase("GEN-1", phase); err != nil {
			t.Errorf("ended phase %s is not recordable through AppendGenerationPhase: %v", phase, err)
		}
	}

	// Guards the loop above against being vacuous — the checklist's own rule.
	// If IsActive ever answered true for everything, every iteration would
	// `continue` and this test would pass having asserted nothing.
	if ended != 3 {
		t.Fatalf("found %d ended phases, want 3 (Complete, TerminalFlatten, Superseded)", ended)
	}
}

// ---------------------------------------------------------------------------
// the append refusals
// ---------------------------------------------------------------------------

func TestGenerationAppendRefusals(t *testing.T) {
	t.Parallel()

	good := testGroup("main", "binance", "BTCUSDT")

	for _, tc := range []struct {
		name string
		call func(*SQLiteJournal) error
		want error
	}{
		{"opened: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationOpened("", good, "RES-1", fixedpoint.ZeroQuantity("BTC"))
		}, ErrJournalGenerationIDUnset},
		{"opened: unpopulated hedge group", func(j *SQLiteJournal) error {
			return j.AppendGenerationOpened("GEN-1", domain.HedgeGroup{}, "RES-1",
				fixedpoint.ZeroQuantity("BTC"))
		}, domain.ErrHedgeGroupAccountUnset},
		{"opened: no reservation", func(j *SQLiteJournal) error {
			return j.AppendGenerationOpened("GEN-1", good, "", fixedpoint.ZeroQuantity("BTC"))
		}, ErrJournalReservationUnset},
		{"opened: target with no asset", func(j *SQLiteJournal) error {
			return j.AppendGenerationOpened("GEN-1", good, "RES-1", fixedpoint.Quantity{})
		}, fixedpoint.ErrAssetUnset},

		{"perp leg: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationPerpSubmitted("", "ORDER-1")
		}, ErrJournalGenerationIDUnset},
		{"perp leg: no order", func(j *SQLiteJournal) error {
			return j.AppendGenerationPerpSubmitted("GEN-1", "")
		}, ErrJournalClientIDUnset},
		{"spot leg: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationSpotSubmitted("", "ORDER-1")
		}, ErrJournalGenerationIDUnset},
		{"spot leg: no order", func(j *SQLiteJournal) error {
			return j.AppendGenerationSpotSubmitted("GEN-1", "")
		}, ErrJournalClientIDUnset},

		{"authorization: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationAuthorized("", fixedpoint.ZeroQuantity("BTC"))
		}, ErrJournalGenerationIDUnset},
		{"authorization: size with no asset", func(j *SQLiteJournal) error {
			return j.AppendGenerationAuthorized("GEN-1", fixedpoint.Quantity{})
		}, fixedpoint.ErrAssetUnset},

		{"supersession: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationSuperseded("", domain.SupersededByPremiseLost)
		}, ErrJournalGenerationIDUnset},
		{"supersession: unset path", func(j *SQLiteJournal) error {
			return j.AppendGenerationSuperseded("GEN-1", "")
		}, domain.ErrSupersessionPathUnset},
		{"supersession: undeclared path", func(j *SQLiteJournal) error {
			return j.AppendGenerationSuperseded("GEN-1", "operator_said_so")
		}, domain.ErrSupersessionPathUnknown},

		{"phase: no generation id", func(j *SQLiteJournal) error {
			return j.AppendGenerationPhase("", domain.GenerationComplete)
		}, ErrJournalGenerationIDUnset},
		{"phase: unset phase", func(j *SQLiteJournal) error {
			return j.AppendGenerationPhase("GEN-1", "")
		}, domain.ErrGenerationPhaseUnset},
		{"phase: undeclared phase", func(j *SQLiteJournal) error {
			return j.AppendGenerationPhase("GEN-1", "nearly_done")
		}, domain.ErrGenerationPhaseUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)
			err := tc.call(j)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			// A refused append must write NOTHING. Checked on every case
			// rather than once, because a refusal that ran after the insert
			// would leave a chain the next append extends from — and every
			// errors.Is assertion above would still pass.
			if records := oracleReplay(t, path); len(records) != 0 {
				t.Errorf("a refused append left %d records in the journal", len(records))
			}
		})
	}

	// The positive control. Every case above is a refusal, so without this the
	// suite is satisfied by a method set that refuses unconditionally.
	t.Run("positive control", func(t *testing.T) {
		t.Parallel()
		j, path := openTestJournal(t)
		if err := j.AppendGenerationOpened("GEN-1", good, "RES-1", testQty(t, "BTC", "1")); err != nil {
			t.Fatalf("AppendGenerationOpened: %v", err)
		}
		if err := j.AppendGenerationPerpSubmitted("GEN-1", "ORDER-1"); err != nil {
			t.Fatalf("AppendGenerationPerpSubmitted: %v", err)
		}
		if err := j.AppendGenerationAuthorized("GEN-1", testQty(t, "BTC", "1")); err != nil {
			t.Fatalf("AppendGenerationAuthorized: %v", err)
		}
		if err := j.AppendGenerationSpotSubmitted("GEN-1", "ORDER-2"); err != nil {
			t.Fatalf("AppendGenerationSpotSubmitted: %v", err)
		}
		if err := j.AppendGenerationSuperseded("GEN-1", domain.SupersededByFenceReadyLost); err != nil {
			t.Fatalf("AppendGenerationSuperseded: %v", err)
		}
		records := oracleReplay(t, path)
		if len(records) != 5 {
			t.Fatalf("journal holds %d records, want 5", len(records))
		}
		verifyChain(t, records)
	})
}

// ---------------------------------------------------------------------------
// append-only, extended to the generation tables
// ---------------------------------------------------------------------------

// TestGenerationDetailTablesAreAppendOnly is the mirror rule applied to the five
// tables this slice added: the triggers on the four original tables are what
// make append-only a property of the DATABASE rather than of this package's
// method set, and a new table with no trigger is a hole an operator's `sqlite3`
// session walks straight through.
func TestGenerationDetailTablesAreAppendOnly(t *testing.T) {
	t.Parallel()
	_, path := writeLadder(t)

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, table := range []string{
		"journal_generation_opened",
		"journal_generation_leg",
		"journal_generation_authorized",
		"journal_generation_phase",
	} {
		for _, stmt := range []string{
			fmt.Sprintf("UPDATE %s SET seq = seq WHERE seq > 0", table),
			fmt.Sprintf("DELETE FROM %s WHERE seq > 0", table),
		} {
			if _, err := db.Exec(stmt); err == nil {
				t.Errorf("%q succeeded; %s is not append-only", stmt, table)
			}
		}
	}

	// journal_generation_superseded has no row in writeLadder's chain (the
	// ladder completes rather than being superseded), so its triggers are
	// exercised on a journal of its own. A DELETE over an EMPTY table succeeds
	// without firing a BEFORE DELETE trigger — which is exactly the vacuous run
	// this repo keeps paying for — so the row has to exist.
	j2, path2 := openTestJournal(t)
	if err := j2.AppendGenerationSuperseded("GEN-X", domain.SupersededByEmergencyClose); err != nil {
		t.Fatalf("AppendGenerationSuperseded: %v", err)
	}
	db2, err := sql.Open("sqlite", "file:"+path2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db2.Close() }()
	for _, stmt := range []string{
		"UPDATE journal_generation_superseded SET path = 'premise_lost' WHERE seq > 0",
		"DELETE FROM journal_generation_superseded WHERE seq > 0",
	} {
		if _, err := db2.Exec(stmt); err == nil {
			t.Errorf("%q succeeded; journal_generation_superseded is not append-only", stmt)
		}
	}
}

// ---------------------------------------------------------------------------
// the subject-column migration
// ---------------------------------------------------------------------------

// oldShapeJournal builds a database in the shape the PREVIOUS build wrote:
// `journal.client_id`, the four order detail tables, and nothing else.
//
// Hand-built rather than produced by an older binary, which is not available to
// a test — and hand-built is the stronger fixture anyway, because the rows are
// written with the ORACLE's encoders. If the migration or the codec changed a
// preimage, the chain this returns would stop verifying, and that is the claim.
func oldShapeJournal(t *testing.T, o domain.Order) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open old-shape journal: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`
CREATE TABLE journal (
	seq       INTEGER PRIMARY KEY,
	kind      TEXT NOT NULL,
	client_id TEXT NOT NULL,
	prev_hash BLOB NOT NULL,
	hash      BLOB NOT NULL
) STRICT;
CREATE TABLE journal_planned (
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
CREATE TABLE journal_state (
	seq   INTEGER PRIMARY KEY REFERENCES journal(seq),
	state TEXT NOT NULL
) STRICT;`); err != nil {
		t.Fatalf("old-shape DDL: %v", err)
	}

	// Record 1: the planned order. Its preimage is built with the oracle's
	// literals, so nothing in production decides what the old build wrote.
	var planned []byte
	planned = append(planned, 1)
	planned = append(planned, oracleStr(string(o.ClientOrderID))...)
	planned = append(planned, oracleStr(string(o.Instrument.Venue))...)
	planned = append(planned, oracleStr(string(o.Instrument.Product))...)
	planned = append(planned, oracleStr(o.Instrument.Symbol)...)
	planned = append(planned, oracleStr(string(o.Instrument.Contract))...)
	planned = append(planned, oracleStr(string(o.Side))...)
	planned = append(planned, oracleStr(string(o.Type))...)
	planned = append(planned, oracleStr(string(o.TimeInForce))...)
	planned = append(planned, oracleStr(string(o.Quantity.Asset()))...)
	planned = append(planned, oracleI64(o.Quantity.Mantissa())...)
	planned = append(planned, oracleStr(string(o.LimitPrice.Base()))...)
	planned = append(planned, oracleStr(string(o.LimitPrice.Quote()))...)
	planned = append(planned, oracleI64(o.LimitPrice.Mantissa())...)
	planned = append(planned, 0)

	h1 := oracleChainHash(1, oracleGenesis(), planned)
	if _, err := db.Exec(
		`INSERT INTO journal (seq, kind, client_id, prev_hash, hash) VALUES (1,'planned',?,?,?)`,
		string(o.ClientOrderID), oracleGenesis(), h1); err != nil {
		t.Fatalf("insert old planned record: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO journal_planned
		(seq, venue, product, symbol, contract, side, order_type, time_in_force,
		 qty_asset, qty_mantissa, price_base, price_quote, price_mantissa, reduce_only)
		VALUES (1,?,?,?,?,?,?,?,?,?,?,?,?,0)`,
		string(o.Instrument.Venue), string(o.Instrument.Product), o.Instrument.Symbol,
		string(o.Instrument.Contract), string(o.Side), string(o.Type), string(o.TimeInForce),
		string(o.Quantity.Asset()), o.Quantity.Mantissa(),
		string(o.LimitPrice.Base()), string(o.LimitPrice.Quote()), o.LimitPrice.Mantissa()); err != nil {
		t.Fatalf("insert old planned detail: %v", err)
	}

	// Record 2: a state record, so the chain has a LINK in it. At length 1 the
	// migration test would be checking only the genesis seed.
	var state []byte
	state = append(state, 2)
	state = append(state, oracleStr(string(o.ClientOrderID))...)
	state = append(state, oracleStr(string(domain.StateFilled))...)
	h2 := oracleChainHash(2, h1, state)
	if _, err := db.Exec(
		`INSERT INTO journal (seq, kind, client_id, prev_hash, hash) VALUES (2,'state',?,?,?)`,
		string(o.ClientOrderID), h1, h2); err != nil {
		t.Fatalf("insert old state record: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO journal_state (seq, state) VALUES (2,?)`,
		string(domain.StateFilled)); err != nil {
		t.Fatalf("insert old state detail: %v", err)
	}
	return path
}

// TestAJournalWrittenBeforeTheSubjectRenameStillVerifies is the migration's
// central claim, and the one the task warned had to be checked rather than
// assumed: widening the shared column changes the STORAGE and not the RECORD.
//
// Every encoder hashes domain values — a client order id, a venue, a mantissa —
// never "the value in the client_id column", so renaming the column cannot move
// a hash. That is why chainLabel keeps its /v1 across this migration. Had it
// been otherwise, this test would fail and the label would have to change, which
// is the loud version of a silent chain spanning two formats.
func TestAJournalWrittenBeforeTheSubjectRenameStillVerifies(t *testing.T) {
	t.Parallel()

	o := testOrder(t, "OLD-ORDER", "binance", "BTCUSDT")
	path := oldShapeJournal(t, o)

	j, err := OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("OpenSQLiteJournal on an old-shape journal: %v", err)
	}
	defer func() { _ = j.Close() }()

	// The column moved.
	if has := journalHasColumn(t, path, "subject"); !has {
		t.Error("journal has no subject column after the migration")
	}
	if has := journalHasColumn(t, path, "client_id"); has {
		t.Error("journal still has a client_id column after the migration")
	}

	// The records did not. This is the claim: the oracle recomputes both links
	// from the stored columns and the /v1 label, and they still check out.
	records := oracleReplay(t, path)
	if len(records) != 2 {
		t.Fatalf("migrated journal holds %d records, want 2", len(records))
	}
	verifyChain(t, records)

	// And the migrated journal is WRITABLE — including by the kinds that made
	// the migration necessary. A rename that left the chain unextendable would
	// pass every assertion above.
	if err := j.AppendGenerationOpened("GEN-1", testGroup("main", "binance", "BTCUSDT"),
		"RES-1", testQty(t, "BTC", "1")); err != nil {
		t.Fatalf("append to a migrated journal: %v", err)
	}
	extended := oracleReplay(t, path)
	if len(extended) != 3 {
		t.Fatalf("extended journal holds %d records, want 3", len(extended))
	}
	verifyChain(t, extended)

	// Reopening must be a no-op rather than a second migration.
	j2, err := OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("reopen a migrated journal: %v", err)
	}
	defer func() { _ = j2.Close() }()
	verifyChain(t, oracleReplay(t, path))
}

// TestOpenRefusesAJournalTableItCannotPlace covers the two shapes no build of
// this system produces, and the reason they are refused rather than left to fail
// at the first append: the realistic cause is a path pointing at somebody else's
// database, and §6 step 6's send path is the wrong place to discover that.
func TestOpenRefusesAJournalTableItCannotPlace(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ddl  string
	}{
		{"neither column", `CREATE TABLE journal (
			seq INTEGER PRIMARY KEY, kind TEXT NOT NULL,
			whose TEXT NOT NULL, prev_hash BLOB NOT NULL, hash BLOB NOT NULL) STRICT`},
		{"both columns", `CREATE TABLE journal (
			seq INTEGER PRIMARY KEY, kind TEXT NOT NULL,
			subject TEXT NOT NULL, client_id TEXT NOT NULL,
			prev_hash BLOB NOT NULL, hash BLOB NOT NULL) STRICT`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "foreign.db")
			db, err := sql.Open("sqlite", "file:"+path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if _, err := db.Exec(tc.ddl); err != nil {
				t.Fatalf("ddl: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}

			j, err := OpenSQLiteJournal(path)
			if !errors.Is(err, ErrJournalSchemaUnknown) {
				if j != nil {
					_ = j.Close()
				}
				t.Fatalf("OpenSQLiteJournal = %v, want ErrJournalSchemaUnknown", err)
			}
		})
	}

	// The positive control: a FRESH path opens, which is what makes the two
	// refusals above statements about the shape rather than about Open.
	t.Run("positive control", func(t *testing.T) {
		t.Parallel()
		j, path := openTestJournal(t)
		if !journalHasColumn(t, path, "subject") {
			t.Error("a fresh journal has no subject column")
		}
		if err := j.AppendGenerationPhase("GEN-1", domain.GenerationComplete); err != nil {
			t.Fatalf("append to a fresh journal: %v", err)
		}
	})
}

func journalHasColumn(t *testing.T, path, column string) bool {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`PRAGMA table_info(journal)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid, notNull, pk int
		var name, declType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &declType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return false
}

// ---------------------------------------------------------------------------
// the replay's dispatch
// ---------------------------------------------------------------------------

// TestReplayHandsEachGenerationKindToItsOwnVisitorMethod checks the half a
// preimage comparison cannot: the oracle proves the BYTES are right, and this
// proves the right method got the right values.
//
// The two legs carry DIFFERENT order ids here — the opposite of the collision
// fixture above, and deliberately. That test needed them equal to isolate the
// tag; this one needs them different, because the defect it is aimed at is the
// perp record reaching GenerationSpotSubmitted, which equal ids would hide.
func TestReplayHandsEachGenerationKindToItsOwnVisitorMethod(t *testing.T) {
	t.Parallel()
	j, _ := writeLadder(t)

	if err := j.AppendGenerationSuperseded("GEN-1", domain.SupersededByPremiseLost); err != nil {
		t.Fatalf("AppendGenerationSuperseded: %v", err)
	}

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	opened := v.gen(t, kindGenerationOpened)
	if opened.id != "GEN-1" {
		t.Errorf("opened id %q, want GEN-1", opened.id)
	}
	if want := testGroup("main", "binance", "BTCUSDT"); opened.group != want {
		t.Errorf("opened group %v, want %v", opened.group, want)
	}
	if opened.reservation != "RES-1" {
		t.Errorf("opened reservation %q, want RES-1", opened.reservation)
	}
	if want := testQty(t, "BTC", "0.75"); opened.target.Mantissa() != want.Mantissa() ||
		opened.target.Asset() != "BTC" {
		t.Errorf("opened target %s of %q, want %s of BTC",
			opened.target.Decimal(), opened.target.Asset(), want.Decimal())
	}

	if got := v.gen(t, kindGenerationPerpSubmitted).order; got != "PERP-LEG" {
		t.Errorf("perp submission carried order %q, want PERP-LEG", got)
	}
	if got := v.gen(t, kindGenerationSpotSubmitted).order; got != "SPOT-LEG" {
		t.Errorf("spot submission carried order %q, want SPOT-LEG", got)
	}
	if got := v.gen(t, kindGenerationSuperseded).path; got != domain.SupersededByPremiseLost {
		t.Errorf("supersession carried path %q, want premise_lost", got)
	}
	if want := testQty(t, "BTC", "0.5"); v.gen(t, kindGenerationAuthorized).size.Mantissa() != want.Mantissa() {
		t.Errorf("authorization carried size %s, want %s",
			v.gen(t, kindGenerationAuthorized).size.Decimal(), want.Decimal())
	}

	// Two phase records, so the "exactly one" helper cannot serve here — and
	// their ORDER is the assertion, because a replay that sorted or grouped by
	// kind would lose the sequence one chain exists to preserve.
	var phases []domain.GenerationPhase
	for _, c := range v.genCalls {
		if c.kind == kindGenerationPhase {
			phases = append(phases, c.phase)
		}
	}
	want := []domain.GenerationPhase{domain.GenerationPerpReduceFinal, domain.GenerationComplete}
	if len(phases) != len(want) {
		t.Fatalf("replay saw %d phase records, want %d", len(phases), len(want))
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Errorf("phase record %d is %s, want %s", i, phases[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// what an OPERATOR can put in the file
// ---------------------------------------------------------------------------

// TestReplayRefusesAnOpeningRecordItCannotRebuild closes the gap mutant M14b
// found, and the gap is worth naming because it is structural rather than an
// oversight.
//
// Every check in replayGenerationOpening has a twin on the write path, so no
// journal this adapter produced can reach any of them — which means a fixture
// built only by appending leaves every one of them dead while the suite looks
// complete. The write-side refusals were tested; the read-side ones were not,
// and a mutation deleting them survived.
//
// The checks exist anyway, because the store is a FILE an operator can open with
// sqlite3, and each column below has a distinct consequence a NULL-read-as-zero
// would produce silently: an empty account merges every subaccount into one
// hedge group, so the generation is judged against another account's exposure;
// an empty reservation is a generation that cannot retire its own ledger entry,
// which then subtracts from every later sizing for the life of the process; an
// empty target asset is a quantity §7's never-net-short cap cannot compare.
//
// This is the mirror rule collecting: the planned record's equivalent
// (TestReplayRefusesAPlannedRecordThatIsNoLongerAValidOrder) has existed since
// slice F1, and its symmetric case at generation scope was missing.
func TestReplayRefusesAnOpeningRecordItCannotRebuild(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		stmt string
		want error
	}{
		{"no account", `UPDATE journal_generation_opened SET account = '' WHERE seq = 1`,
			domain.ErrHedgeGroupAccountUnset},
		{"no venue", `UPDATE journal_generation_opened SET venue = '' WHERE seq = 1`,
			ids.ErrVenueUnset},
		{"no symbol", `UPDATE journal_generation_opened SET symbol = '' WHERE seq = 1`,
			domain.ErrHedgeGroupSymbolUnset},
		{"no reservation", `UPDATE journal_generation_opened SET reservation = '' WHERE seq = 1`,
			ErrJournalReservationUnset},
		{"no target asset", `UPDATE journal_generation_opened SET target_asset = '' WHERE seq = 1`,
			fixedpoint.ErrAssetUnset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)
			if err := j.AppendGenerationOpened("GEN-1", testGroup("main", "binance", "BTCUSDT"),
				"RES-1", testQty(t, "BTC", "1")); err != nil {
				t.Fatalf("AppendGenerationOpened: %v", err)
			}
			tamperColumn(t, path, tc.stmt)

			var v recordingVisitor
			if err := j.Replay(&v); !errors.Is(err, tc.want) {
				t.Fatalf("Replay = %v, want %v", err, tc.want)
			}
			// It must refuse BEFORE handing the record over, or a caller has
			// already rebuilt state from a group it is about to be told to
			// distrust.
			if len(v.genCalls) != 0 {
				t.Errorf("the visitor saw %d records from an unrebuildable opening", len(v.genCalls))
			}
		})
	}

	// The positive control. Every case above is a refusal, so without an
	// untampered journal replaying cleanly the suite is satisfied by a replay
	// that refuses every opening record it sees.
	t.Run("positive control", func(t *testing.T) {
		t.Parallel()
		j, _ := openTestJournal(t)
		if err := j.AppendGenerationOpened("GEN-1", testGroup("main", "binance", "BTCUSDT"),
			"RES-1", testQty(t, "BTC", "1")); err != nil {
			t.Fatalf("AppendGenerationOpened: %v", err)
		}
		var v recordingVisitor
		if err := j.Replay(&v); err != nil {
			t.Fatalf("Replay = %v, want nil", err)
		}
		if len(v.genCalls) != 1 {
			t.Fatalf("the visitor saw %d records, want 1", len(v.genCalls))
		}
	})
}
