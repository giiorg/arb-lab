package driven

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The replay's checks.
//
// # The collapse checklist, run before the mutations — and re-run, because the
// # journal's dimensions moved when the third record kind landed
//
//  1. CHAIN LENGTH. Unchanged from the append side: at length 1 "verify against
//     the predecessor" and "verify against genesis" coincide, and at length 2
//     "the predecessor" and "the first record" still coincide. Every tamper
//     fixture here is length 3 and breaks record 1, so the claim under test is
//     about a record that is neither the first nor the last.
//  2. RECORD KIND. There are three now, and a replay is where a missing kind is
//     most invisible: the walk succeeds, the visitor is never called for it, and
//     the rebuilt state is silently short. Every round-trip fixture holds all
//     three.
//  3. WHICH VERIFICATION FIRED. verifyLink has four refusals and they are
//     reachable by different tampers. A fixture that broke all of them at once
//     would pass against an implementation that had only one — see the three
//     isolating tampers below, each of which leaves the other three checks
//     satisfied.
//  4. THE ANCHOR'S TWO NUMBERS. An instant and an uncertainty are both int64
//     nanosecond columns, so a codec that wrote the instant into both, or read
//     them back swapped, produces a record that verifies. testAnchor makes them
//     differ, and the round-trip asserts each separately.

// recordingVisitor is ports.JournalVisitor as a log of what it was handed.
type recordingVisitor struct {
	planned    []domain.Order
	authorized []anchorCall
	states     []stateCall
	acked      []ackCall
	seqs       []uint64

	// The generation half. genCalls holds one entry per SM §7 record IN ORDER,
	// across all six kinds, which is what makes the "the authorization precedes
	// the spot leg" claim assertable at all: a per-kind slice each would record
	// the same six records and throw away the only fact one chain was chosen to
	// preserve.
	genCalls []genCall

	// The core §7.1 margin-op half, in order, for genCalls' reason at a third
	// subject: the claim one chain buys here is that a resolution follows its
	// dispatch, which a per-kind slice each would discard.
	marginCalls []marginCall

	// failAt, when positive, makes the visit of that sequence fail. It is how
	// the "stops at the first refusal" claim is asserted rather than inferred.
	failAt  uint64
	failErr error
}

type anchorCall struct {
	id ids.ClientOrderID
	at clock.ServerTime
}

type stateCall struct {
	id    ids.ClientOrderID
	state domain.OrderState
}

type ackCall struct {
	id           ids.ClientOrderID
	venueOrderID string
}

func (v *recordingVisitor) note(seq uint64) error {
	v.seqs = append(v.seqs, seq)
	if v.failAt == seq {
		return v.failErr
	}
	return nil
}

func (v *recordingVisitor) Planned(seq uint64, o domain.Order) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.planned = append(v.planned, o)
	return nil
}

func (v *recordingVisitor) Authorized(seq uint64, id ids.ClientOrderID, at clock.ServerTime) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.authorized = append(v.authorized, anchorCall{id: id, at: at})
	return nil
}

func (v *recordingVisitor) State(seq uint64, id ids.ClientOrderID, s domain.OrderState) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.states = append(v.states, stateCall{id: id, state: s})
	return nil
}

func (v *recordingVisitor) Acked(seq uint64, id ids.ClientOrderID, venueOrderID string) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.acked = append(v.acked, ackCall{id: id, venueOrderID: venueOrderID})
	return nil
}

// genCall is one generation record as the visitor received it.
//
// One struct across the six kinds rather than six typed slices, with `kind`
// naming which method was called. That is the shape the ORDERING claims need —
// see genCalls — and the fields it does not use per kind are the price of it.
// They are safe here in a way ports.GenerationJournal refuses in its API,
// because a test fixture's unread field is inspected by an assertion that knows
// the kind, while a port's unread field is read by a replay that does not.
type genCall struct {
	kind        string
	seq         uint64
	id          domain.GenerationID
	group       domain.HedgeGroup
	reservation domain.ReservationID
	target      fixedpoint.Quantity
	order       ids.ClientOrderID
	size        fixedpoint.Quantity
	path        domain.SupersessionPath
	phase       domain.GenerationPhase
}

func (v *recordingVisitor) GenerationOpened(seq uint64, id domain.GenerationID, group domain.HedgeGroup,
	reservation domain.ReservationID, target fixedpoint.Quantity) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{
		kind: kindGenerationOpened, seq: seq, id: id,
		group: group, reservation: reservation, target: target,
	})
	return nil
}

func (v *recordingVisitor) GenerationPerpSubmitted(seq uint64, id domain.GenerationID, order ids.ClientOrderID) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{kind: kindGenerationPerpSubmitted, seq: seq, id: id, order: order})
	return nil
}

func (v *recordingVisitor) GenerationAuthorized(seq uint64, id domain.GenerationID, size fixedpoint.Quantity) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{kind: kindGenerationAuthorized, seq: seq, id: id, size: size})
	return nil
}

func (v *recordingVisitor) GenerationSpotSubmitted(seq uint64, id domain.GenerationID, order ids.ClientOrderID) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{kind: kindGenerationSpotSubmitted, seq: seq, id: id, order: order})
	return nil
}

func (v *recordingVisitor) GenerationSuperseded(seq uint64, id domain.GenerationID, path domain.SupersessionPath) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{kind: kindGenerationSuperseded, seq: seq, id: id, path: path})
	return nil
}

func (v *recordingVisitor) Phase(seq uint64, id domain.GenerationID, phase domain.GenerationPhase) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.genCalls = append(v.genCalls, genCall{kind: kindGenerationPhase, seq: seq, id: id, phase: phase})
	return nil
}

// marginCall is one core §7.1 margin-op record as the visitor saw it.
//
// A record of its own rather than more fields on genCall, because the ORDERING
// claims genCall exists for are about SM §7's ladder — a sequence of phases on
// one subject — and a margin op has no ladder. Folding the two would make
// `gen(t, kind)`'s exactly-one rule span two machines, so a fixture writing one
// of each would satisfy an assertion about either.
type marginCall struct {
	kind   string
	seq    uint64
	op     domain.MarginOp
	id     domain.MarginOpID
	status domain.MarginOpStatus
	act    domain.MarginOpReconciliation
}

func (v *recordingVisitor) MarginOpDispatched(seq uint64, op domain.MarginOp) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.marginCalls = append(v.marginCalls, marginCall{
		kind: kindMarginOpDispatched, seq: seq, op: op, id: op.ID,
	})
	return nil
}

func (v *recordingVisitor) MarginOpResolved(seq uint64, id domain.MarginOpID, status domain.MarginOpStatus) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.marginCalls = append(v.marginCalls, marginCall{
		kind: kindMarginOpResolved, seq: seq, id: id, status: status,
	})
	return nil
}

func (v *recordingVisitor) MarginOpReconciled(seq uint64, act domain.MarginOpReconciliation) error {
	if err := v.note(seq); err != nil {
		return err
	}
	v.marginCalls = append(v.marginCalls, marginCall{
		kind: kindMarginOpReconciled, seq: seq, id: act.Op, act: act,
	})
	return nil
}

// margin returns the single recorded margin call of a kind — gen's rule, and it
// binds for the same reason: the dispatch and the resolution are the two records
// whose confusion would be silent.
func (v *recordingVisitor) margin(t *testing.T, kind string) marginCall {
	t.Helper()
	var found []marginCall
	for _, c := range v.marginCalls {
		if c.kind == kind {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("recorded %d %s calls, want exactly 1", len(found), kind)
	}
	return found[0]
}

// gen returns the single recorded call of a kind, failing if there is not
// exactly one.
//
// Exactly one rather than "the first", on the repo's own selection-fixture rule:
// a helper that returned the first match would make a fixture writing two
// records of a kind pass an assertion about either of them, and the perp/spot
// pair is precisely where that would hide a swap.
func (v *recordingVisitor) gen(t *testing.T, kind string) genCall {
	t.Helper()
	var found []genCall
	for _, c := range v.genCalls {
		if c.kind == kind {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("recorded %d %s calls, want exactly 1", len(found), kind)
	}
	return found[0]
}

// writeThreeKinds appends one record of each kind and returns what was written,
// so a test asserts against the values rather than against a restatement.
func writeThreeKinds(t *testing.T, j *SQLiteJournal) (domain.Order, clock.ServerTime) {
	t.Helper()

	o := testOrder(t, "c-1", "binance", "BTCUSDT")
	if err := j.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	at := testAnchor(t, 7)
	if err := j.AppendAuthorized(o.ClientOrderID, at); err != nil {
		t.Fatalf("AppendAuthorized: %v", err)
	}
	if err := j.AppendState(o.ClientOrderID, domain.StateSubmitted); err != nil {
		t.Fatalf("AppendState: %v", err)
	}
	return o, at
}

// TestReplayRoundTripsEveryKindInOrder is the positive control every tamper
// test below needs, and the claim it makes on its own is the round trip: what
// the visitor receives is what the writer was given, field for field.
func TestReplayRoundTripsEveryKindInOrder(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)
	o, at := writeThreeKinds(t, j)

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if want := []uint64{1, 2, 3}; fmt.Sprint(v.seqs) != fmt.Sprint(want) {
		t.Errorf("visited sequences %v, want %v", v.seqs, want)
	}
	if len(v.planned) != 1 || len(v.authorized) != 1 || len(v.states) != 1 {
		t.Fatalf("visited %d planned, %d authorized, %d state records; want 1 of each",
			len(v.planned), len(v.authorized), len(v.states))
	}

	if got := v.planned[0]; got != o {
		t.Errorf("planned record replayed as %+v, want %+v", got, o)
	}

	// The anchor's two numbers, asserted separately. Compared as instants
	// rather than with == on the struct: the value that went in carries a
	// monotonic reading and a location, and what comes back is the wall clock
	// the column can hold. That is the round trip the storage claims, and
	// asserting struct equality would be asserting a claim nothing needs.
	if got := v.authorized[0]; got.id != o.ClientOrderID {
		t.Errorf("authorization replayed for order %q, want %q", got.id, o.ClientOrderID)
	}
	if got := v.authorized[0].at.Instant(); !got.Equal(at.Instant()) {
		t.Errorf("anchor instant replayed as %s, want %s", got, at.Instant())
	}
	if got := v.authorized[0].at.Uncertainty(); got != at.Uncertainty() {
		t.Errorf("anchor uncertainty replayed as %s, want %s", got, at.Uncertainty())
	}
	if err := v.authorized[0].at.Validate(); err != nil {
		t.Errorf("replayed anchor is unset: %v — an unset ServerTime answers "+
			"Uncertain to every boundary, so §5.3's rule could never conclude", err)
	}

	if got := v.states[0]; got.id != o.ClientOrderID || got.state != domain.StateSubmitted {
		t.Errorf("state record replayed as %+v, want %q/%s", got, o.ClientOrderID, domain.StateSubmitted)
	}
}

// TestReplayRoundTripsAnAckedRecord is the fourth kind's round trip, beside
// the three-kind positive control rather than folded into it — the tamper
// tests below key on that helper's sequence numbers, and shifting them to
// admit a fourth record would re-aim every one of them.
func TestReplayRoundTripsAnAckedRecord(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)
	o, _ := writeThreeKinds(t, j)
	if err := j.AppendAcked(o.ClientOrderID, "774411"); err != nil {
		t.Fatalf("AppendAcked: %v", err)
	}

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if want := []uint64{1, 2, 3, 4}; fmt.Sprint(v.seqs) != fmt.Sprint(want) {
		t.Errorf("visited sequences %v, want %v", v.seqs, want)
	}
	if len(v.acked) != 1 {
		t.Fatalf("visited %d acked records, want 1", len(v.acked))
	}
	if got := v.acked[0]; got.id != o.ClientOrderID || got.venueOrderID != "774411" {
		t.Errorf("acked record replayed as %+v, want %q/%q", got, o.ClientOrderID, "774411")
	}
}

// TestAckedTamperBreaksTheChain is what makes encodeAcked's hashing real: an
// edited venue id must fail verification, which requires the column to be in
// the preimage rather than merely in the table. Without this test a
// journal_acked row would be the one detail the chain does not actually
// cover, and an attacker's cheapest edit is exactly the id a swept row is
// ruled out against.
func TestAckedTamperBreaksTheChain(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	o, _ := writeThreeKinds(t, j)
	if err := j.AppendAcked(o.ClientOrderID, "774411"); err != nil {
		t.Fatalf("AppendAcked: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("tamper open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`DROP TRIGGER journal_acked_no_update`); err != nil {
		t.Fatalf("tamper: drop trigger: %v", err)
	}
	if _, err := db.Exec(`UPDATE journal_acked SET venue_order_id = '999999' WHERE seq = 4`); err != nil {
		t.Fatalf("tamper: update: %v", err)
	}

	var v recordingVisitor
	err = j.Replay(&v)
	if !errors.Is(err, ports.ErrJournalChainBroken) {
		t.Fatalf("Replay over a tampered acked record = %v, want ErrJournalChainBroken", err)
	}
	// The tamper is at seq 4, so the intact prefix was still delivered — the
	// caller's rebuilt state is a prefix of the log, not a subset.
	if want := []uint64{1, 2, 3}; fmt.Sprint(v.seqs) != fmt.Sprint(want) {
		t.Errorf("visited sequences %v, want %v — the verified prefix only", v.seqs, want)
	}
}

// TestReplayOfAnEmptyJournalVisitsNothingAndSucceeds is the first boot.
//
// The failing direction is worth naming: an empty journal reported as an error
// would make the one case with nothing to recover the case that refuses to
// start, and every deployment's first launch is that case.
func TestReplayOfAnEmptyJournalVisitsNothingAndSucceeds(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay of an empty journal: %v", err)
	}
	if len(v.seqs) != 0 {
		t.Errorf("visited %d records of an empty journal", len(v.seqs))
	}
}

// TestReplayRefusesANilVisitor covers the refusal whose absence would be a
// silent success: a nil visitor cannot mean "verify only", because that answer
// is indistinguishable from a successful replay of an empty journal.
func TestReplayRefusesANilVisitor(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)
	writeThreeKinds(t, j)

	if err := j.Replay(nil); !errors.Is(err, ErrJournalVisitorMissing) {
		t.Errorf("Replay(nil) = %v, want ErrJournalVisitorMissing", err)
	}
}

// TestReplayStopsAtTheFirstRefusalAndReportsIt is the contract a caller relies
// on to know its rebuilt state is a prefix of the log rather than a subset.
//
// The fixture fails on record 2 of 3, which is the position where "stops" and
// "skips" differ: a replay that continued would still report the error, and
// only the absence of record 3 says it stopped.
func TestReplayStopsAtTheFirstRefusalAndReportsIt(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)
	writeThreeKinds(t, j)

	sentinel := errors.New("visitor refused")
	v := recordingVisitor{failAt: 2, failErr: sentinel}

	err := j.Replay(&v)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Replay = %v, want the visitor's own error", err)
	}
	if errors.Is(err, ports.ErrJournalChainBroken) {
		t.Error("a visitor's refusal was reported as a broken chain — the log was intact")
	}
	if want := []uint64{1, 2}; fmt.Sprint(v.seqs) != fmt.Sprint(want) {
		t.Errorf("visited %v, want %v — the walk continued past a refusal", v.seqs, want)
	}
}

// TestReplayDetectsAnEditedRecord isolates the RECOMPUTED-HASH check.
//
// Editing a column that is in the preimage leaves prev_hash correct and the
// sequence dense, so the only refusal that can fire is the one comparing the
// stored digest against what the columns hash to. Record 1 of 3 is edited so
// the fixture is not also testing "the last record".
func TestReplayDetectsAnEditedRecord(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	// Positive control first: this journal replays clean before the tamper.
	var before recordingVisitor
	if err := j.Replay(&before); err != nil {
		t.Fatalf("Replay before tamper: %v", err)
	}

	tamperClientID(t, path, 1, "c-1-EDITED")

	var after recordingVisitor
	err := j.Replay(&after)
	if !errors.Is(err, ports.ErrJournalChainBroken) {
		t.Fatalf("Replay after tamper = %v, want ErrJournalChainBroken", err)
	}
	if len(after.seqs) != 0 {
		t.Errorf("visited %v after tampering with record 1 — a record was handed "+
			"over before its link verified", after.seqs)
	}
}

// TestReplayDetectsALinkEditedToItsOwnPredecessor isolates the PREV_HASH check.
//
// Rewriting record 2's prev_hash to the genesis seed is the tamper a chain
// exists to catch: every record is individually present, the sequence is dense,
// and record 2 now claims to be a first record. The check that fires is the
// link comparison, before the digest comparison it would also fail.
func TestReplayDetectsALinkEditedToItsOwnPredecessor(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	genesis := genesisHash()
	tamperColumn(t, path, `UPDATE journal SET prev_hash = ? WHERE seq = 2`, genesis[:])

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, ports.ErrJournalChainBroken) {
		t.Fatalf("Replay = %v, want ErrJournalChainBroken", err)
	}
	if len(v.seqs) != 1 {
		t.Errorf("visited %v, want only record 1 — the break is at record 2", v.seqs)
	}
}

// TestReplayDetectsADeletedRecord isolates the SEQUENCE check, and it is the
// one a reader most often assumes the database already makes.
//
// A PRIMARY KEY is unique and not contiguous. With record 2 gone, records 1 and
// 3 are untouched and each verifies against its own stored predecessor hash —
// so every check except the density one passes, and a replay that trusted the
// key would rebuild a state missing whatever record 2 said.
func TestReplayDetectsADeletedRecord(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	tamperDelete(t, path, 2)

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, ports.ErrJournalChainBroken) {
		t.Fatalf("Replay = %v, want ErrJournalChainBroken", err)
	}
	if len(v.seqs) != 1 {
		t.Errorf("visited %v, want only record 1 — the hole is at position 2", v.seqs)
	}
}

// TestReplayRefusesAChainThatDoesNotStartAtOne is the ISOLATING probe for the
// sequence check, and it exists because the obvious probe is not one.
//
// TestReplayDetectsADeletedRecord above looks like the sequence check's
// fixture and is not: with record 2 gone, record 3's prev_hash still names
// record 2's hash, so the LINK check refuses it first and the mutation that
// deletes the density check survives. That is this repo's recorded shape — a
// probe two checks can see shows that something objected, not that this rule
// did.
//
// What only the density check can see is a chain that is internally perfect and
// does not begin at 1: a dropped PREFIX, re-anchored to the genesis seed. Every
// remaining record verifies against its own predecessor, the first one verifies
// against genesis, and the only surviving evidence that anything was removed is
// that the first sequence is 2. The re-anchoring is what an attacker with the
// file does — the digests are not signed, so re-linking is available to anyone
// who can write — which makes "the chain starts at 1" a load-bearing claim
// rather than a formality.
func TestReplayRefusesAChainThatDoesNotStartAtOne(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	o := testOrder(t, "c-1", "binance", "BTCUSDT")
	if err := j.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	if err := j.AppendState(o.ClientOrderID, domain.StateSubmitted); err != nil {
		t.Fatalf("AppendState: %v", err)
	}

	// Drop the prefix and re-anchor what is left. The replacement digest is
	// computed with the production encoder deliberately: the claim under test
	// is the SEQUENCE rule, so the tamper has to be one that satisfies every
	// other rule exactly.
	tamperDelete(t, path, 1)
	genesis := genesisHash()
	reanchored := chainHash(2, genesis, encodeState(o.ClientOrderID, domain.StateSubmitted))
	tamperColumn(t, path, `UPDATE journal SET prev_hash = ?, hash = ? WHERE seq = 2`,
		genesis[:], reanchored[:])

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, ports.ErrJournalChainBroken) {
		t.Fatalf("Replay of a chain starting at 2 = %v, want ErrJournalChainBroken", err)
	}
	if len(v.seqs) != 0 {
		t.Errorf("visited %v — a re-anchored chain with its prefix removed was replayed", v.seqs)
	}
}

// TestReplayRefusesAKindItCannotInterpret is the downgrade case: a journal
// written by a later build, read by an earlier one.
//
// It is refused rather than skipped, and the sentinel is deliberately NOT
// ErrJournalChainBroken — the chain is fine, and what is wrong is this build's
// ability to read it. The two produce different boot conversations.
func TestReplayRefusesAKindItCannotInterpret(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	tamperColumn(t, path, `UPDATE journal SET kind = 'settled' WHERE seq = 2`)

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, ErrJournalKindUnknown) {
		t.Fatalf("Replay = %v, want ErrJournalKindUnknown", err)
	}
	if errors.Is(err, ports.ErrJournalChainBroken) {
		t.Error("an unreadable kind was reported as a broken chain")
	}
}

// TestReplayRefusesARecordWithNoDetailRow covers the NULL a LEFT JOIN produces
// for a detail row that is not there.
//
// Unreachable through the adapter — both rows are one transaction and the
// foreign key is on — and reachable with sqlite3, which is the point: the
// alternative to reporting it is reading the NULL as a zero, and a state
// record whose state is the empty string is an order in no state at all.
func TestReplayRefusesARecordWithNoDetailRow(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	tamperDeleteDetail(t, path, "journal_state", 3)

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, ErrJournalDetailMissing) {
		t.Fatalf("Replay = %v, want ErrJournalDetailMissing", err)
	}
}

// TestReplayRefusesAPlannedRecordThatIsNoLongerAValidOrder is AppendPlanned's
// own claim, checked where that method said it would be checked: "a record this
// package cannot rebuild into a valid domain.Order is a record that fails at
// §7.5's replay".
//
// The edit is to a detail column rather than the chain row, so the failure is
// about the ORDER rather than about the hash — and the assertion is that the
// order's own validation is what refuses it.
func TestReplayRefusesAPlannedRecordThatIsNoLongerAValidOrder(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)
	writeThreeKinds(t, j)

	tamperColumn(t, path, `UPDATE journal_planned SET side = 'sideways' WHERE seq = 1`)

	var v recordingVisitor
	err := j.Replay(&v)
	if !errors.Is(err, domain.ErrSideInvalid) {
		t.Fatalf("Replay = %v, want the domain's own refusal of an undeclared side", err)
	}
}

// TestAppendAuthorizedRefusals covers the two an anchor can fail, plus the
// range check that has no counterpart on any other column.
func TestAppendAuthorizedRefusals(t *testing.T) {
	t.Parallel()

	// Year 3000, which UnixNano cannot represent: it wraps rather than
	// reporting, and the wrapped value is a plausible instant in the wrong
	// century. §5.3's retention gate then reads it as long purged.
	farFuture, err := clock.NewServerTime(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC), time.Second)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}

	cases := []struct {
		name string
		id   ids.ClientOrderID
		at   clock.ServerTime
		want error
	}{
		{"positive control", "c-1", testAnchor(t, 3), nil},
		{"no client order id", "", testAnchor(t, 3), ErrJournalClientIDUnset},
		{"unset anchor", "c-1", clock.ServerTime{}, ports.ErrAuthorizationNeedsAnchor},
		{"anchor outside the storable range", "c-1", farFuture, ErrJournalAnchorRange},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j, path := openTestJournal(t)

			err := j.AppendAuthorized(tc.id, tc.at)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("AppendAuthorized: %v", err)
				}
				if r := oracleReplay(t, path); len(r) != 1 {
					t.Errorf("accepted append wrote %d records, want 1", len(r))
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("AppendAuthorized = %v, want %v", err, tc.want)
			}
			if r := oracleReplay(t, path); len(r) != 0 {
				t.Errorf("a refused append left %d records behind", len(r))
			}
		})
	}
}

// TestRecordKindsCannotCollide is the kind tag's own job, now over three kinds
// rather than two.
//
// The authorization is the pair that makes it load-bearing: an authorization is
// a client id plus sixteen fixed bytes and a state record is a client id plus a
// length-prefixed string, so without the tag a state whose text encoded as
// those bytes would share a preimage with an authorization of the same order.
func TestRecordKindsCannotCollide(t *testing.T) {
	t.Parallel()

	preimages := map[string][]byte{
		"planned":    encodePlanned(testOrder(t, "c-1", "binance", "BTCUSDT")),
		"state":      encodeState("c-1", domain.StatePlanned),
		"authorized": encodeAuthorized("c-1", testAnchor(t, 1)),
	}

	tags := make(map[byte]string, len(preimages))
	for kind, b := range preimages {
		if other, dup := tags[b[0]]; dup {
			t.Errorf("kinds %q and %q carry tag %d — the tag is not discriminating",
				kind, other, b[0])
		}
		tags[b[0]] = kind
	}

	for kind, a := range preimages {
		for otherKind, b := range preimages {
			if kind >= otherKind {
				continue
			}
			if string(a) == string(b) {
				t.Errorf("a %s record and a %s record for the same order share a preimage:\n  %x",
					kind, otherKind, a)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// helpers that break things on purpose
// ---------------------------------------------------------------------------

// tamperColumn runs an UPDATE against a committed record, dropping the triggers
// that forbid it first. It is the general form of tamperClientID, which stays
// as it is because its callers assert on the row it edits.
func tamperColumn(t *testing.T, path, stmt string, args ...any) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("tamper open: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, trigger := range []string{
		"journal_no_update", "journal_planned_no_update",
		"journal_state_no_update", "journal_authorized_no_update",
		"journal_generation_opened_no_update", "journal_generation_leg_no_update",
		"journal_generation_authorized_no_update", "journal_generation_superseded_no_update",
		"journal_generation_phase_no_update",
		// The three core §7.1 margin-op tables. They were absent until task
		// #213 needed one, which is this helper's own recurring shape: a tamper
		// list that has not kept up makes every read-side check on the tables it
		// omits UNREACHABLE, and the omission looks exactly like a table nobody
		// wanted to tamper.
		"journal_margin_op_dispatched_no_update", "journal_margin_op_resolved_no_update",
		"journal_margin_op_reconciled_no_update",
	} {
		if _, err := db.Exec(`DROP TRIGGER IF EXISTS ` + trigger); err != nil {
			t.Fatalf("tamper: drop %s: %v", trigger, err)
		}
	}
	res, err := db.Exec(stmt, args...)
	if err != nil {
		t.Fatalf("tamper: %s: %v", stmt, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("tamper: %s updated %d rows (err %v), want 1", stmt, n, err)
	}
}

// tamperDelete removes a whole record — the chain row and every detail row that
// references it.
func tamperDelete(t *testing.T, path string, seq int64) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("tamper open: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, trigger := range []string{
		"journal_no_delete", "journal_planned_no_delete",
		"journal_state_no_delete", "journal_authorized_no_delete",
	} {
		if _, err := db.Exec(`DROP TRIGGER IF EXISTS ` + trigger); err != nil {
			t.Fatalf("tamper: drop %s: %v", trigger, err)
		}
	}
	for _, table := range []string{"journal_planned", "journal_state", "journal_authorized", "journal"} {
		if _, err := db.Exec(`DELETE FROM `+table+` WHERE seq = ?`, seq); err != nil {
			t.Fatalf("tamper: delete from %s: %v", table, err)
		}
	}
}

// tamperDeleteDetail removes a detail row and leaves its chain row standing.
func tamperDeleteDetail(t *testing.T, path, table string, seq int64) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("tamper open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`DROP TRIGGER IF EXISTS ` + table + `_no_delete`); err != nil {
		t.Fatalf("tamper: drop trigger: %v", err)
	}
	res, err := db.Exec(`DELETE FROM `+table+` WHERE seq = ?`, seq)
	if err != nil {
		t.Fatalf("tamper: delete: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("tamper: deleted %d rows (err %v), want 1", n, err)
	}
}
