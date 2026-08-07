package application_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §7.5's replay, at the use-case layer.
//
// # The collapse checklist, run before the mutations
//
//  1. ORDER COUNT. Every fixture that makes a per-order claim holds TWO orders.
//     With one, "this order's final state" and "the first order's final state"
//     are the same value, and Recovery.Orders cannot tell a count from a
//     constant.
//  2. RECORD POSITION. The duplicate, the failure and the refusal all sit at a
//     record that is neither the first nor the last. A fixture that broke on
//     record 1 could not tell "stopped" from "never started", and one that broke
//     on the last could not tell "stopped" from "finished".
//  3. WHICH ORDER IS UNRESOLVED. The two orders in the unresolved fixtures end
//     in DIFFERENT states, one resolved and one not, and the resolved one passes
//     THROUGH an unresolved state on its way. A fixture where both end alike
//     would pass against a replayer that answered with the whole set — or with
//     the empty one.
//  4. SORT ORDER. Ids are appended in descending order, so a report that
//     returned them in insertion order and one that sorted them differ.

// journalEntry is one record as the fake emits it. The fields are per-kind and
// only one group is meaningful at a time — the shape ports.Journal refuses in
// its API and a scripted fixture is allowed, because the kind chooses.
type journalEntry struct {
	seq          uint64
	kind         string
	order        domain.Order
	id           ids.ClientOrderID
	at           clock.ServerTime
	state        domain.OrderState
	venueOrderID string

	// The SM §7 generation half. The payload fields are here so that a fixture
	// can prove they REACH the visitor unchanged; the visitor drops them, and
	// that drop is the scope of the current slice rather than a property to
	// assert. What is asserted is the liveness the ids and phases decide.
	gen   domain.GenerationID
	group domain.HedgeGroup
	res   domain.ReservationID
	qty   fixedpoint.Quantity
	path  domain.SupersessionPath
	phase domain.GenerationPhase
}

// fakeReplay is ports.JournalReplay over a scripted log.
//
// It does NOT verify a hash chain — that is the adapter's, checked in
// driven/journalreplay_internal_test.go against an independent oracle. What it
// models here is what a use case can observe: records arriving in order, and a
// walk that stops with a particular error. failAfter is how the broken-chain
// branch is reached without a database.
type fakeReplay struct {
	entries  []journalEntry
	failWith error

	// failAfter is how many entries are emitted before failWith is returned.
	// Zero with a non-nil failWith means the walk fails before emitting
	// anything, which is what a corrupt FIRST record looks like.
	failAfter int

	visited int
}

func (f *fakeReplay) Replay(v ports.JournalVisitor) error {
	for _, e := range f.entries {
		if f.failWith != nil && f.visited == f.failAfter {
			return f.failWith
		}
		f.visited++

		var err error
		switch e.kind {
		case "planned":
			err = v.Planned(e.seq, e.order)
		case "authorized":
			err = v.Authorized(e.seq, e.id, e.at)
		case "state":
			err = v.State(e.seq, e.id, e.state)
		case "acked":
			err = v.Acked(e.seq, e.id, e.venueOrderID)
		case "generation_opened":
			err = v.GenerationOpened(e.seq, e.gen, e.group, e.res, e.qty)
		case "generation_perp_submitted":
			err = v.GenerationPerpSubmitted(e.seq, e.gen, e.id)
		case "generation_authorized":
			err = v.GenerationAuthorized(e.seq, e.gen, e.qty)
		case "generation_spot_submitted":
			err = v.GenerationSpotSubmitted(e.seq, e.gen, e.id)
		case "generation_superseded":
			err = v.GenerationSuperseded(e.seq, e.gen, e.path)
		case "generation_phase":
			err = v.Phase(e.seq, e.gen, e.phase)
		default:
			return fmt.Errorf("fake replay: entry %d has kind %q", e.seq, e.kind)
		}
		if err != nil {
			return err
		}
	}
	if f.failWith != nil && f.visited == f.failAfter {
		return f.failWith
	}
	return nil
}

var _ ports.JournalReplay = (*fakeReplay)(nil)

// replayFixture builds a journal script for two orders, and returns the
// replayer with the repository it writes into.
func newReplayer(t *testing.T, entries ...journalEntry) (*application.Replayer, *fakeReplay, *fakeOrders) {
	t.Helper()

	journal := &fakeReplay{entries: entries}
	orders := newFakeOrders()
	r, err := application.NewReplayer(journal, orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	return r, journal, orders
}

// replayAnchor is a calibrated venue instant. The uncertainty varies with the
// offset for the reason the adapter's helper gives: two int64 nanosecond
// numbers that always agree cannot show that they were kept apart.
func replayAnchor(t *testing.T, offsetSeconds int) clock.ServerTime {
	t.Helper()
	at, err := clock.NewServerTime(
		time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC).Add(time.Duration(offsetSeconds)*time.Second),
		time.Duration(offsetSeconds+2)*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return at
}

// replayOrder builds a well-formed order with a CHOSEN id, so a fixture can
// hold two that differ in the field the report is keyed by.
//
// dispatchableOrder is not reused directly because it pins the id, which is the
// one dimension every claim here varies.
func replayOrder(t *testing.T, id ids.ClientOrderID) domain.Order {
	t.Helper()
	o := dispatchableOrder(t)
	o.ClientOrderID = id
	if err := o.Validate(); err != nil {
		t.Fatalf("fixture order %q: %v", id, err)
	}
	return o
}

// TestRebuildRestoresStateAndTheAnchor is the slice's central claim: after a
// crash, the projection says what the log says — INCLUDING §5.3's anchor, which
// before slice F1 no replay could produce because no record carried it.
func TestRebuildRestoresStateAndTheAnchor(t *testing.T) {
	t.Parallel()

	first, second := mintedID(t, 0), mintedID(t, 1)
	at := replayAnchor(t, 5)

	r, _, orders := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, second)},
		journalEntry{seq: 3, kind: "authorized", id: first, at: at},
		journalEntry{seq: 4, kind: "state", id: first, state: domain.StateSubmitted},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	if got.Records != 4 || got.LastSequence != 4 {
		t.Errorf("Records/LastSequence = %d/%d, want 4/4", got.Records, got.LastSequence)
	}
	if got.Orders != 2 {
		t.Errorf("Orders = %d, want 2", got.Orders)
	}

	rec, err := orders.Load(first)
	if err != nil {
		t.Fatalf("Load %q: %v", first, err)
	}
	if rec.State != domain.StateSubmitted {
		t.Errorf("replayed state = %s, want %s", rec.State, domain.StateSubmitted)
	}
	if !rec.AuthorizedAt.Instant().Equal(at.Instant()) {
		t.Errorf("replayed anchor = %s, want %s — a recovered order with no anchor "+
			"can never reach AbsentAfterFinality", rec.AuthorizedAt.Instant(), at.Instant())
	}
	if rec.AuthorizedAt.Uncertainty() != at.Uncertainty() {
		t.Errorf("replayed uncertainty = %s, want %s", rec.AuthorizedAt.Uncertainty(), at.Uncertainty())
	}

	// The second order is at StatePlanned, which §5.3 makes safe to replan or
	// discard — it is in the projection and it is not in flight.
	other, err := orders.Load(second)
	if err != nil {
		t.Fatalf("Load %q: %v", second, err)
	}
	if other.State != domain.StatePlanned {
		t.Errorf("second order replayed at %s, want %s", other.State, domain.StatePlanned)
	}
}

// TestUnresolvedIsDerivedFromTHEFINALSTATE is the claim that a report is
// computed from where each order ended rather than accumulated as the walk goes.
//
// The fixture is built so the two readings disagree: the first order passes
// THROUGH DispatchAuthorized and Submitted on its way to Filled, so an
// accumulating replayer lists it, and a deriving one does not. The second order
// stops at DispatchAuthorized and must be listed by both.
func TestUnresolvedIsDerivedFromTheFinalState(t *testing.T) {
	t.Parallel()

	// Descending, so insertion order and sorted order differ.
	high, low := mintedID(t, 9), mintedID(t, 1)
	if high <= low {
		t.Fatalf("fixture ids %q and %q are not in the order this test needs", high, low)
	}

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, high)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, low)},
		journalEntry{seq: 3, kind: "authorized", id: high, at: replayAnchor(t, 1)},
		journalEntry{seq: 4, kind: "state", id: high, state: domain.StateSubmitted},
		journalEntry{seq: 5, kind: "state", id: high, state: domain.StateAcknowledged},
		journalEntry{seq: 6, kind: "state", id: high, state: domain.StateFilled},
		journalEntry{seq: 7, kind: "authorized", id: low, at: replayAnchor(t, 2)},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	want := []ids.ClientOrderID{low}
	if fmt.Sprint(got.Unresolved) != fmt.Sprint(want) {
		t.Errorf("Unresolved = %v, want %v — a terminal order that PASSED THROUGH "+
			"the crash boundary is resolved", got.Unresolved, want)
	}
}

// TestUnresolvedIsSortedAndExcludesPlanned pins the two remaining properties of
// the set, and the second is domain's distinction rather than this package's.
//
// StatePlanned is not in it because "a Planned-only order was never dispatched
// (no venue order can exist)" — there is nothing at the venue to query about,
// and a resolution attempt would run to its deadline over an order this system
// merely has not sent.
func TestUnresolvedIsSortedAndExcludesPlanned(t *testing.T) {
	t.Parallel()

	high, low, planned := mintedID(t, 9), mintedID(t, 5), mintedID(t, 1)

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, high)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, low)},
		journalEntry{seq: 3, kind: "planned", order: replayOrder(t, planned)},
		journalEntry{seq: 4, kind: "authorized", id: high, at: replayAnchor(t, 1)},
		journalEntry{seq: 5, kind: "authorized", id: low, at: replayAnchor(t, 2)},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	want := []ids.ClientOrderID{low, high}
	if fmt.Sprint(got.Unresolved) != fmt.Sprint(want) {
		t.Errorf("Unresolved = %v, want %v (sorted, and without the Planned order)",
			got.Unresolved, want)
	}
}

// TestADuplicateStateRecordIsANoOpAndIsCounted is §5.3's duplicate deliveries
// meeting SM §1's no-op edge.
//
// journalcodec.go states that AppendState "may legitimately record the same
// order reaching the same state twice", so a replay that treated the second as a
// broken history would refuse to boot over a well-formed journal. It is counted
// rather than ignored because nothing else in the system would report a stream
// that is re-delivering.
//
// The version is the assertion that carries the no-op claim: a replay that
// applied the duplicate would advance it twice.
func TestADuplicateStateRecordIsANoOpAndIsCounted(t *testing.T) {
	t.Parallel()

	id := mintedID(t, 0)
	r, _, orders := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, id)},
		journalEntry{seq: 2, kind: "authorized", id: id, at: replayAnchor(t, 1)},
		journalEntry{seq: 3, kind: "state", id: id, state: domain.StateSubmitted},
		journalEntry{seq: 4, kind: "state", id: id, state: domain.StateSubmitted},
		journalEntry{seq: 5, kind: "state", id: id, state: domain.StateAcknowledged},
		journalEntry{seq: 6, kind: "state", id: id, state: domain.StateFilled},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", got.Duplicates)
	}
	if got.Records != 6 {
		t.Errorf("Records = %d, want 6 — a duplicate is still a record", got.Records)
	}

	rec, err := orders.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.State != domain.StateFilled {
		t.Errorf("state = %s, want %s", rec.State, domain.StateFilled)
	}
	// planned(1) -> authorized(2) -> submitted(3) -> acknowledged(4) -> filled(5).
	// The duplicate contributes nothing, which is the whole claim.
	if rec.Version != 5 {
		t.Errorf("version = %d, want 5 — the duplicate advanced the record", rec.Version)
	}
}

// TestRebuildRefusesAHistoryTheLIVEPathCouldNotHaveProduced is why the replay
// writes through the repository rather than installing records.
//
// Filled -> Submitted is not an edge of SM §1. A bulk loader would install it
// and the projection would hold a state no sequence of events could reach; here
// domain.CanTransition refuses it, in the same call the live path makes.
func TestRebuildRefusesAHistoryTheLivePathCouldNotHaveProduced(t *testing.T) {
	t.Parallel()

	id := mintedID(t, 0)
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, id)},
		journalEntry{seq: 2, kind: "authorized", id: id, at: replayAnchor(t, 1)},
		journalEntry{seq: 3, kind: "state", id: id, state: domain.StateFilled},
		journalEntry{seq: 4, kind: "state", id: id, state: domain.StateSubmitted},
	)

	_, err := r.Rebuild()
	if !errors.Is(err, domain.ErrTransitionForbidden) {
		t.Fatalf("Rebuild = %v, want ErrTransitionForbidden", err)
	}
	if application.ChainBroken(err) {
		t.Error("an unreplayable history was reported as a broken chain — the log was intact")
	}
}

// TestRebuildRefusesAStateRecordForAnOrderThatWasNeverPlanned covers the other
// contents failure: a log that is internally inconsistent rather than illegal.
func TestRebuildRefusesAStateRecordForAnOrderThatWasNeverPlanned(t *testing.T) {
	t.Parallel()

	planned, orphan := mintedID(t, 0), mintedID(t, 1)
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, planned)},
		journalEntry{seq: 2, kind: "state", id: orphan, state: domain.StateSubmitted},
		journalEntry{seq: 3, kind: "state", id: planned, state: domain.StateSubmitted},
	)

	_, err := r.Rebuild()
	if !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("Rebuild = %v, want ErrOrderUnknown", err)
	}
}

// TestABrokenChainIsReportedAsSuchAndLeavesTheVERIFIEDPrefixApplied is the
// distinction §7.5's two postures rest on.
//
// The chain breaks at record 3, so records 1 and 2 were verified before they
// were handed over. What the repository holds afterwards is exactly that prefix
// — which is why the failure is a posture rather than a rollback: an operator
// inspecting the store must see what the intact part of the log says, not an
// empty map.
func TestABrokenChainIsReportedAsSuchAndLeavesTheVerifiedPrefixApplied(t *testing.T) {
	t.Parallel()

	id := mintedID(t, 0)
	journal := &fakeReplay{
		entries: []journalEntry{
			{seq: 1, kind: "planned", order: replayOrder(t, id)},
			{seq: 2, kind: "authorized", id: id, at: replayAnchor(t, 1)},
			{seq: 3, kind: "state", id: id, state: domain.StateSubmitted},
		},
		failAfter: 2,
		failWith:  fmt.Errorf("record 3: %w", ports.ErrJournalChainBroken),
	}
	orders := newFakeOrders()
	r, err := application.NewReplayer(journal, orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}

	_, rebuildErr := r.Rebuild()
	if !application.ChainBroken(rebuildErr) {
		t.Fatalf("Rebuild = %v, want a broken-chain report", rebuildErr)
	}

	rec, err := orders.Load(id)
	if err != nil {
		t.Fatalf("Load after a broken chain: %v — the verified prefix was discarded", err)
	}
	if rec.State != domain.StateDispatchAuthorized {
		t.Errorf("state = %s, want %s — the two verified records were not applied",
			rec.State, domain.StateDispatchAuthorized)
	}
}

// TestRebuildOfAnEmptyJournalIsASuccessfulRebuildOfNothing is the first boot,
// at the use-case layer.
func TestRebuildOfAnEmptyJournalIsASuccessfulRebuildOfNothing(t *testing.T) {
	t.Parallel()

	r, _, _ := newReplayer(t)
	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got.Records != 0 || got.Orders != 0 || got.LastSequence != 0 || len(got.Unresolved) != 0 {
		t.Errorf("Rebuild of an empty journal = %+v, want a zero report", got)
	}
}

// TestRebuildRefusesToReportOnAnOrderTheProjectionWillNotProduce covers the
// read the report makes AFTER the walk, and the direction it fails in is why it
// is fatal rather than skipped.
//
// The id is in the report's list because a journal record created it. A
// repository that then will not produce it means the log and the projection
// disagree about what exists — and an omitted order is an order missing from
// Unresolved, which is the set §7.5 hands to the venue join. Under-counting
// there is a live order nobody goes looking for.
func TestRebuildRefusesToReportOnAnOrderTheProjectionWillNotProduce(t *testing.T) {
	t.Parallel()

	id := mintedID(t, 0)
	journal := &fakeReplay{entries: []journalEntry{
		{seq: 1, kind: "planned", order: replayOrder(t, id)},
	}}
	orders := newFakeOrders()
	r, err := application.NewReplayer(journal, orders)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}

	// The walk itself must succeed, so that what fails is the report and not
	// the replay: the failure is planted only once the record has been applied.
	sentinel := errors.New("repository is unavailable")
	orders.failLoad = sentinel

	if _, err := r.Rebuild(); !errors.Is(err, sentinel) {
		t.Errorf("Rebuild = %v, want the repository's own failure", err)
	}
}

func TestNewReplayerRefusesAMissingPort(t *testing.T) {
	t.Parallel()

	if _, err := application.NewReplayer(nil, newFakeOrders()); !errors.Is(err, application.ErrJournalReplayMissing) {
		t.Errorf("NewReplayer(nil, orders) = %v, want ErrJournalReplayMissing", err)
	}
	if _, err := application.NewReplayer(&fakeReplay{}, nil); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewReplayer(journal, nil) = %v, want ErrOrdersMissing", err)
	}

	// Positive control: both present is accepted, so the refusals above are not
	// a constructor that refuses everything.
	if _, err := application.NewReplayer(&fakeReplay{}, newFakeOrders()); err != nil {
		t.Errorf("NewReplayer(journal, orders) = %v, want nil", err)
	}
}

// TestDispatchedIsASupersetOfUnresolvedAndHoldsTheTerminals is the handoff §7.5's
// venue join takes, and the fixture is built so that the two lists DIFFER.
//
// A journal whose orders are all unresolved makes "hands over the uncertain set"
// and "hands over everything that was sent" the same list, and the join would
// then be tested against a fixture that cannot show the difference the two
// questions have. The difference is where this system's fills are: §6 step 6
// sends marketable IOCs, whose ordinary outcome is terminal within
// milliseconds, so the orders that moved the position are exactly the ones
// §6.7's set excludes.
func TestDispatchedIsASupersetOfUnresolvedAndHoldsTheTerminals(t *testing.T) {
	t.Parallel()

	filled, live, planned := mintedID(t, 9), mintedID(t, 5), mintedID(t, 1)

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, filled)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, live)},
		journalEntry{seq: 3, kind: "planned", order: replayOrder(t, planned)},
		journalEntry{seq: 4, kind: "authorized", id: filled, at: replayAnchor(t, 1)},
		journalEntry{seq: 5, kind: "authorized", id: live, at: replayAnchor(t, 2)},
		journalEntry{seq: 6, kind: "state", id: filled, state: domain.StateSubmitted},
		journalEntry{seq: 7, kind: "state", id: filled, state: domain.StateAcknowledged},
		journalEntry{seq: 8, kind: "state", id: filled, state: domain.StateFilled},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	if want := []ids.ClientOrderID{live}; fmt.Sprint(got.Unresolved) != fmt.Sprint(want) {
		t.Errorf("Unresolved = %v, want %v — the filled order's interval has collapsed", got.Unresolved, want)
	}
	// Sorted, and it holds the terminal: leg 5 sorts before leg 9.
	if want := []ids.ClientOrderID{live, filled}; fmt.Sprint(got.Dispatched) != fmt.Sprint(want) {
		t.Errorf("Dispatched = %v, want %v (sorted, WITH the filled order and WITHOUT the planned one)",
			got.Dispatched, want)
	}
	// The positive control on the whole claim: at equal lengths the two lists are
	// one list, and every assertion above would pass against a report that
	// derived both from the same predicate.
	if len(got.Dispatched) == len(got.Unresolved) {
		t.Errorf("Dispatched and Unresolved are the same size (%d); the fixture cannot show that they answer different questions",
			len(got.Dispatched))
	}
}

// ---------------------------------------------------------------------------
// acked records (task #174): the venue id survives the crash
// ---------------------------------------------------------------------------

// TestRebuildRestoresTheVenueOrderId is the ack's half of the central claim:
// the persisted id is not process memory, so a recovered projection can still
// rule a sweep row out against it. Two orders (checklist item 1), one acked
// and one not, so a replayer that stamped every order — or none — fails.
func TestRebuildRestoresTheVenueOrderId(t *testing.T) {
	t.Parallel()

	first, second := mintedID(t, 0), mintedID(t, 1)
	at := replayAnchor(t, 5)

	r, _, orders := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, second)},
		journalEntry{seq: 3, kind: "authorized", id: first, at: at},
		journalEntry{seq: 4, kind: "acked", id: first, venueOrderID: "774411"},
		journalEntry{seq: 5, kind: "state", id: first, state: domain.StateSubmitted},
		journalEntry{seq: 6, kind: "authorized", id: second, at: replayAnchor(t, 6)},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got.Records != 6 || got.LastSequence != 6 {
		t.Errorf("Records/LastSequence = %d/%d, want 6/6", got.Records, got.LastSequence)
	}

	rec, err := orders.Load(first)
	if err != nil {
		t.Fatalf("Load %q: %v", first, err)
	}
	if rec.VenueOrderID != "774411" {
		t.Errorf("replayed venue id = %q, want %q", rec.VenueOrderID, "774411")
	}
	other, err := orders.Load(second)
	if err != nil {
		t.Fatalf("Load %q: %v", second, err)
	}
	if other.VenueOrderID != "" {
		t.Errorf("second order replayed with venue id %q, want none", other.VenueOrderID)
	}
}

// TestADuplicateAckedRecordIsANoOpAndIsCounted mirrors the duplicate-state
// claim for the fourth kind: §5.3's duplicate deliveries are legitimate, and
// the restatement neither fails the replay nor moves the version — asserted
// through the SUBSEQUENT state record, whose version arithmetic would conflict
// if the no-op had bumped it. The duplicate sits mid-log (checklist item 2).
func TestADuplicateAckedRecordIsANoOpAndIsCounted(t *testing.T) {
	t.Parallel()

	id := mintedID(t, 0)
	at := replayAnchor(t, 5)

	r, _, orders := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, id)},
		journalEntry{seq: 2, kind: "authorized", id: id, at: at},
		journalEntry{seq: 3, kind: "acked", id: id, venueOrderID: "774411"},
		journalEntry{seq: 4, kind: "acked", id: id, venueOrderID: "774411"},
		journalEntry{seq: 5, kind: "state", id: id, state: domain.StateSubmitted},
	)

	got, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", got.Duplicates)
	}
	rec, err := orders.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.State != domain.StateSubmitted || rec.VenueOrderID != "774411" {
		t.Errorf("replayed as %s/%q, want %s/%q", rec.State, rec.VenueOrderID,
			domain.StateSubmitted, "774411")
	}
}

// TestReplayRefusesConflictingAckRecords is the crash half of the conflict
// design: a journal holding two acked records that disagree — about one
// order's id, or about one id's owner — verifies as a LOG and does not
// compose as a HISTORY, and the refusal is what keeps a poisoned index from
// ever being consulted: the boot that would have built it fails first.
func TestReplayRefusesConflictingAckRecords(t *testing.T) {
	t.Parallel()

	first, second := mintedID(t, 0), mintedID(t, 1)

	t.Run("one order under two ids", func(t *testing.T) {
		t.Parallel()
		r, _, _ := newReplayer(t,
			journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
			journalEntry{seq: 2, kind: "authorized", id: first, at: replayAnchor(t, 5)},
			journalEntry{seq: 3, kind: "acked", id: first, venueOrderID: "774411"},
			journalEntry{seq: 4, kind: "acked", id: first, venueOrderID: "999999"},
		)
		_, err := r.Rebuild()
		if !errors.Is(err, ports.ErrVenueOrderIDConflict) {
			t.Fatalf("Rebuild = %v, want ErrVenueOrderIDConflict", err)
		}
	})

	t.Run("one id under two orders on the same instrument", func(t *testing.T) {
		t.Parallel()
		r, _, _ := newReplayer(t,
			journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
			journalEntry{seq: 2, kind: "planned", order: replayOrder(t, second)},
			journalEntry{seq: 3, kind: "authorized", id: first, at: replayAnchor(t, 5)},
			journalEntry{seq: 4, kind: "acked", id: first, venueOrderID: "774411"},
			journalEntry{seq: 5, kind: "authorized", id: second, at: replayAnchor(t, 6)},
			journalEntry{seq: 6, kind: "acked", id: second, venueOrderID: "774411"},
		)
		_, err := r.Rebuild()
		if !errors.Is(err, ports.ErrVenueOrderIDConflict) {
			t.Fatalf("Rebuild = %v, want ErrVenueOrderIDConflict", err)
		}
	})

	// A broken-log refusal is not the chain sentinel: the log VERIFIED, and
	// reporting it as tampering would send the operator to the wrong problem.
	t.Run("the refusal is not a chain-broken report", func(t *testing.T) {
		t.Parallel()
		r, _, _ := newReplayer(t,
			journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
			journalEntry{seq: 2, kind: "authorized", id: first, at: replayAnchor(t, 5)},
			journalEntry{seq: 3, kind: "acked", id: first, venueOrderID: "774411"},
			journalEntry{seq: 4, kind: "acked", id: first, venueOrderID: "999999"},
		)
		_, err := r.Rebuild()
		if application.ChainBroken(err) {
			t.Fatalf("ChainBroken(%v) = true — a log that verifies and does not compose is the OTHER failure", err)
		}
	})
}

// TestRebuildRefusesAnAckForAnOrderNeverPlanned closes the same door the
// state-record twin closes: a record about an order the log never created.
func TestRebuildRefusesAnAckForAnOrderNeverPlanned(t *testing.T) {
	t.Parallel()

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, mintedID(t, 0))},
		journalEntry{seq: 2, kind: "acked", id: mintedID(t, 1), venueOrderID: "774411"},
	)
	if _, err := r.Rebuild(); !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("Rebuild = %v, want ErrOrderUnknown", err)
	}
}

// ---------------------------------------------------------------------------
// SM §7's generations
// ---------------------------------------------------------------------------

// The generation half's own collapse checklist, on the dimensions the four
// above do not reach:
//
//  5. GENERATION COUNT. Every liveness claim holds TWO generations, one live
//     and one ended. With one, "the live ones" and "all of them" are the same
//     list, and with two both-live or both-ended the filter is a constant.
//  6. THE ENDING KIND. `Complete` arrives through Phase and `Superseded`
//     through its own method, so a fixture using one leaves the other's arm
//     dead. The two-generation fixture uses one of each.
//  7. RECORD POSITION, again. The opening of the SECOND generation sits after
//     the first generation's mid-ladder records, so a walk that keyed liveness
//     off the last record it saw would disagree.

func testHedgeGroup(symbol string) domain.HedgeGroup {
	return domain.HedgeGroup{Account: "main", Venue: "binance", Symbol: symbol}
}

func testTarget(t *testing.T, decimal string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", decimal)
	if err != nil {
		t.Fatalf("ParseQuantity(%s): %v", decimal, err)
	}
	return q
}

// TestRebuildReportsTheGenerationsTheLogNEVERENDED is the boot refusal's
// evidence, and the fixture is what makes it a claim rather than a tautology.
//
// GEN-A runs the full ladder and COMPLETES; GEN-B opens, submits its perp leg
// and stops — a crash mid-ladder, which is the state FLAG-3's durability exists
// for. A replay that reported both, or neither, or "the last one", passes
// nothing here.
func TestRebuildReportsTheGenerationsTheLogNeverEnded(t *testing.T) {
	t.Parallel()

	perp := replayOrder(t, "PERP-A")
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: perp},
		journalEntry{seq: 2, kind: "generation_opened", gen: "GEN-A",
			group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
		journalEntry{seq: 3, kind: "generation_perp_submitted", gen: "GEN-A", id: perp.ClientOrderID},
		journalEntry{seq: 4, kind: "generation_authorized", gen: "GEN-A", qty: testTarget(t, "0.5")},
		journalEntry{seq: 5, kind: "generation_phase", gen: "GEN-A", phase: domain.GenerationComplete},
		journalEntry{seq: 6, kind: "generation_opened", gen: "GEN-B",
			group: testHedgeGroup("ETHUSDT"), res: "RES-B", qty: testTarget(t, "2")},
		journalEntry{seq: 7, kind: "generation_perp_submitted", gen: "GEN-B", id: perp.ClientOrderID},
	)

	rec, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if rec.Generations != 2 {
		t.Errorf("Generations = %d, want 2", rec.Generations)
	}
	// Asserted as the exact list rather than as a length or a containment: a
	// length is satisfied by the wrong generation, and a containment check
	// throws away the variation the fixture was built to carry.
	want := []domain.GenerationID{"GEN-B"}
	got := liveGenerationIDs(rec)
	if len(got) != len(want) {
		t.Fatalf("LiveGenerations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LiveGenerations[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestASupersededGenerationIsNotLive covers the OTHER ending kind, which travels
// its own visitor method to the same conclusion.
//
// Its mirror — the ended-by-Complete case — is above, and both are needed: a
// suite holding one of them passes against a rebuild that only handles that one,
// and the arm it does not handle is the one that leaves a phantom entry standing
// in the ledger.
func TestASupersededGenerationIsNotLive(t *testing.T) {
	t.Parallel()

	for _, path := range []domain.SupersessionPath{
		domain.SupersededByEmergencyClose,
		domain.SupersededByFenceReadyLost,
		domain.SupersededByPremiseLost,
	} {
		t.Run(path.String(), func(t *testing.T) {
			t.Parallel()
			r, _, _ := newReplayer(t,
				journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
					group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
				journalEntry{seq: 2, kind: "generation_superseded", gen: "GEN-A", path: path},
			)
			rec, err := r.Rebuild()
			if err != nil {
				t.Fatalf("Rebuild: %v", err)
			}
			if len(rec.LiveGenerations) != 0 {
				t.Errorf("LiveGenerations = %v after a %s supersession, want none",
					liveGenerationIDs(rec), path)
			}
			if rec.Generations != 1 {
				t.Errorf("Generations = %d, want 1", rec.Generations)
			}
		})
	}
}

// TestAnUnendedGenerationIsLiveAtEveryMidLadderPhase is the positive control the
// two tests above need: they show that endings END, and without this one a
// rebuild that reported nothing live under any circumstances would pass both.
//
// It runs every phase AppendGenerationPhase can record and that IsActive calls
// live, because those are the arms where "no ending seen" has to keep meaning
// live — and `HeldPerp`/`HeldSpot` are the parks, which is exactly where a crash
// leaves a generation holding a ledger entry and an alert.
func TestAnUnendedGenerationIsLiveAtEveryMidLadderPhase(t *testing.T) {
	t.Parallel()

	for _, phase := range []domain.GenerationPhase{
		domain.GenerationPerpReduceFinal,
		domain.GenerationHeldPerp,
		domain.GenerationHeldSpot,
	} {
		t.Run(phase.String(), func(t *testing.T) {
			t.Parallel()
			active, err := phase.IsActive()
			if err != nil {
				t.Fatalf("%s.IsActive: %v", phase, err)
			}
			if !active {
				t.Fatalf("fixture phase %s is not active; this test asserts the LIVE arm", phase)
			}

			r, _, _ := newReplayer(t,
				journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
					group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
				journalEntry{seq: 2, kind: "generation_phase", gen: "GEN-A", phase: phase},
			)
			rec, err := r.Rebuild()
			if err != nil {
				t.Fatalf("Rebuild: %v", err)
			}
			if len(rec.LiveGenerations) != 1 || rec.LiveGenerations[0].ID != "GEN-A" {
				t.Errorf("LiveGenerations = %v at %s, want [GEN-A]", liveGenerationIDs(rec), phase)
			}
		})
	}
}

// TestRebuildRefusesAGenerationRecordWithNoOpening is the generation-scope
// reading of "the log verifies and does not compose", and it is checked on
// EVERY dependent kind rather than one.
//
// One case would demonstrate the rule and hide the ladder: five methods call
// knownGeneration, and a fixture exercising one leaves four able to lose the
// call with the suite green. The realistic cause is head truncation, and the
// direction is what makes it a refusal — a walk that carried on would report
// zero live generations for a package that has one.
func TestRebuildRefusesAGenerationRecordWithNoOpening(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		entry journalEntry
	}{
		{"perp leg", journalEntry{seq: 1, kind: "generation_perp_submitted", gen: "GHOST", id: "ORDER-1"}},
		{"authorization", journalEntry{seq: 1, kind: "generation_authorized", gen: "GHOST",
			qty: testTarget(t, "1")}},
		{"spot leg", journalEntry{seq: 1, kind: "generation_spot_submitted", gen: "GHOST", id: "ORDER-1"}},
		{"supersession", journalEntry{seq: 1, kind: "generation_superseded", gen: "GHOST",
			path: domain.SupersededByPremiseLost}},
		{"phase", journalEntry{seq: 1, kind: "generation_phase", gen: "GHOST",
			phase: domain.GenerationComplete}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, _, _ := newReplayer(t, tc.entry)
			if _, err := r.Rebuild(); !errors.Is(err, application.ErrGenerationNotOpened) {
				t.Fatalf("Rebuild = %v, want ErrGenerationNotOpened", err)
			}
		})
	}

	// The positive control: the same record kinds compose when the opening is
	// there. Without it, a rebuild refusing every generation record passes
	// every case above.
	t.Run("positive control", func(t *testing.T) {
		t.Parallel()
		r, _, _ := newReplayer(t,
			journalEntry{seq: 1, kind: "generation_opened", gen: "GHOST",
				group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
			journalEntry{seq: 2, kind: "generation_perp_submitted", gen: "GHOST", id: "ORDER-1"},
			journalEntry{seq: 3, kind: "generation_authorized", gen: "GHOST", qty: testTarget(t, "1")},
			journalEntry{seq: 4, kind: "generation_spot_submitted", gen: "GHOST", id: "ORDER-2"},
			journalEntry{seq: 5, kind: "generation_phase", gen: "GHOST", phase: domain.GenerationComplete},
		)
		if _, err := r.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
	})
}

// TestRebuildRefusesASecondOpeningOfOneGeneration is the append-only store's own
// impossibility, reported rather than absorbed: one identity brought into
// existence twice, where the second record cannot be a correction of the first.
func TestRebuildRefusesASecondOpeningOfOneGeneration(t *testing.T) {
	t.Parallel()

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
			group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
		journalEntry{seq: 2, kind: "generation_opened", gen: "GEN-B",
			group: testHedgeGroup("ETHUSDT"), res: "RES-B", qty: testTarget(t, "2")},
		journalEntry{seq: 3, kind: "generation_opened", gen: "GEN-A",
			group: testHedgeGroup("BTCUSDT"), res: "RES-C", qty: testTarget(t, "3")},
	)
	if _, err := r.Rebuild(); !errors.Is(err, application.ErrGenerationAlreadyOpened) {
		t.Fatalf("Rebuild = %v, want ErrGenerationAlreadyOpened", err)
	}
}

// TestRebuildRefusesAnUndeclaredSupersessionPath is the replay's own
// well-formedness line: an undeclared path replays into a generation whose one
// remaining question — may core §7.1's ladder open a later generation — has no
// answer at all, because ResumesLadder cannot answer it.
func TestRebuildRefusesAnUndeclaredSupersessionPath(t *testing.T) {
	t.Parallel()

	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
			group: testHedgeGroup("BTCUSDT"), res: "RES-A", qty: testTarget(t, "1")},
		journalEntry{seq: 2, kind: "generation_superseded", gen: "GEN-A", path: "operator_said_so"},
	)
	if _, err := r.Rebuild(); !errors.Is(err, domain.ErrSupersessionPathUnknown) {
		t.Fatalf("Rebuild = %v, want ErrSupersessionPathUnknown", err)
	}
}

// TestAJournalWithNoGenerationsReportsNone is the whole feature's negative
// control, and it is the case every existing boot takes: no composition root
// constructs a Deleverer, so every journal in the field holds order records
// only. If this failed, the refusal in Reconciler would fire on every boot.
func TestAJournalWithNoGenerationsReportsNone(t *testing.T) {
	t.Parallel()

	first, second := mintedID(t, 0), mintedID(t, 1)
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "planned", order: replayOrder(t, first)},
		journalEntry{seq: 2, kind: "planned", order: replayOrder(t, second)},
		journalEntry{seq: 3, kind: "authorized", id: first, at: replayAnchor(t, 3)},
		journalEntry{seq: 4, kind: "state", id: first, state: domain.StateSubmitted},
	)
	rec, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if rec.Generations != 0 || len(rec.LiveGenerations) != 0 {
		t.Errorf("Generations = %d, LiveGenerations = %v; want 0 and none",
			rec.Generations, liveGenerationIDs(rec))
	}
}

// liveGenerationIDs projects a Recovery's live generations to their ids, so the
// assertions above stay about the SET the replay reported while the value it
// reports grew payloads.
//
// It is a projection in the TEST rather than an accessor on Recovery, and
// deliberately: an accessor would be a second way to read the field, and the
// tests that care about the payloads have to read them off the struct anyway.
func liveGenerationIDs(rec application.Recovery) []domain.GenerationID {
	ids := make([]domain.GenerationID, 0, len(rec.LiveGenerations))
	for _, g := range rec.LiveGenerations {
		ids = append(ids, g.ID)
	}
	return ids
}

// TestTheReplayBarsTheLadderOnlyForThePathsThatDoNotResume is
// SupersessionPath.ResumesLadder read off the LOG, and it is one test over all
// three paths rather than one per path for a reason the mutation campaign made
// concrete.
//
// The rule is a property of the PATH, so it has a true set and a false set and
// each is only meaningful against the other. A version that barred on EVERY
// supersession passes any fixture using only the external rows; a version that
// barred on NONE passes any fixture using only the premise-lost row; and
// inverting the predicate outright — which is what mutant M11 did — passes both
// of those suites. Running the three together is what leaves no such build
// standing.
//
// The failure directions differ, which is why neither half is optional. Losing
// an external bar lets core §7.1's ladder open a generation against an
// arbitration that has taken the package away from it. Adding a premise-lost bar
// disables the one mechanism that reduces leverage on a neutral package for the
// rest of the process's life, at exactly the moment it is needed.
func TestTheReplayBarsTheLadderOnlyForThePathsThatDoNotResume(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		path domain.SupersessionPath
		bars bool
	}{
		{domain.SupersededByEmergencyClose, true},
		{domain.SupersededByFenceReadyLost, true},
		{domain.SupersededByPremiseLost, false},
	} {
		t.Run(tc.path.String(), func(t *testing.T) {
			t.Parallel()
			group := testHedgeGroup("BTCUSDT")
			r, _, _ := newReplayer(t,
				journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
					group: group, res: "RES-A", qty: testTarget(t, "1")},
				journalEntry{seq: 2, kind: "generation_superseded", gen: "GEN-A", path: tc.path},
			)
			rec, err := r.Rebuild()
			if err != nil {
				t.Fatalf("Rebuild: %v", err)
			}
			if !tc.bars {
				if len(rec.BarredLadders) != 0 {
					t.Fatalf("BarredLadders = %v after a %s supersession, want none", rec.BarredLadders, tc.path)
				}
				return
			}
			if len(rec.BarredLadders) != 1 {
				t.Fatalf("BarredLadders = %v after a %s supersession, want one", rec.BarredLadders, tc.path)
			}
			// The GROUP comes from the generation's OPENING record, not from the
			// supersession record, which names only the generation. A bar keyed on
			// anything else could not be derived at all.
			if got := rec.BarredLadders[0]; got.Group != group || got.Path != tc.path {
				t.Errorf("BarredLadders[0] = %+v, want {%s %s}", got, group, tc.path)
			}
		})
	}
}

// TestABarSurvivesAGenerationThatEndedNormallyOnAnotherGroup is the second
// dimension the test above cannot carry: the bar is per hedge GROUP.
//
// With one group, "bars this group" and "bars every group" are the same
// observable — and the second would stop a package deleveraging because an
// unrelated package's kill switch fired.
func TestABarSurvivesAGenerationThatEndedNormallyOnAnotherGroup(t *testing.T) {
	t.Parallel()

	barred, open := testHedgeGroup("BTCUSDT"), testHedgeGroup("ETHUSDT")
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
			group: barred, res: "RES-A", qty: testTarget(t, "1")},
		journalEntry{seq: 2, kind: "generation_superseded", gen: "GEN-A",
			path: domain.SupersededByEmergencyClose},
		journalEntry{seq: 3, kind: "generation_opened", gen: "GEN-B",
			group: open, res: "RES-B", qty: testTarget(t, "2")},
		journalEntry{seq: 4, kind: "generation_phase", gen: "GEN-B", phase: domain.GenerationComplete},
	)
	rec, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if len(rec.BarredLadders) != 1 || rec.BarredLadders[0].Group != barred {
		t.Fatalf("BarredLadders = %v, want only %s", rec.BarredLadders, barred)
	}
}

// TestTheReplayCarriesEveryOpeningPayloadIntoTheRecovery is what #203's dropped
// payloads became, and it is asserted field by field.
//
// Every one of these reached the visitor and was discarded for a slice, on the
// ground that an unread field is worse than a named gap. Reinstater is the
// reader, so each is now load-bearing: the reservation is the ledger KEY, the
// target is both the perp leg's size and the asset the group is tracked under,
// the leg ids are FM-9's netting set, and the authorized size is what the entry
// carries from FLAG-3's record onward.
func TestTheReplayCarriesEveryOpeningPayloadIntoTheRecovery(t *testing.T) {
	t.Parallel()

	group := testHedgeGroup("BTCUSDT")
	perpID, spotID := mintedID(t, 3), mintedID(t, 4)
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
			group: group, res: "RES-A", qty: testTarget(t, "1.5")},
		journalEntry{seq: 2, kind: "generation_perp_submitted", gen: "GEN-A", id: perpID},
		journalEntry{seq: 3, kind: "generation_phase", gen: "GEN-A", phase: domain.GenerationPerpReduceFinal},
		journalEntry{seq: 4, kind: "generation_authorized", gen: "GEN-A", qty: testTarget(t, "0.75")},
		journalEntry{seq: 5, kind: "generation_spot_submitted", gen: "GEN-A", id: spotID},
	)
	rec, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if len(rec.LiveGenerations) != 1 {
		t.Fatalf("LiveGenerations = %v, want one", liveGenerationIDs(rec))
	}
	got := rec.LiveGenerations[0]
	if got.ID != "GEN-A" || got.Group != group || got.Reservation != "RES-A" {
		t.Errorf("identity = {%s %s %s}, want {GEN-A %s RES-A}", got.ID, got.Group, got.Reservation, group)
	}
	if got.Target.Decimal() != testTarget(t, "1.5").Decimal() {
		t.Errorf("Target = %s, want 1.5", got.Target)
	}
	// 0.75 against a target of 1.5: the two are different numbers, so a rebuild
	// that read the authorization off the opening record fails here.
	if got.Authorized.Decimal() != testTarget(t, "0.75").Decimal() {
		t.Errorf("Authorized = %s, want 0.75", got.Authorized)
	}
	// The two legs are DIFFERENT ids, so a rebuild that assigned one record's
	// order to both fields fails — which is the shape the shared detail table
	// makes possible and the kind tag is what prevents.
	if got.PerpOrder != perpID || got.SpotOrder != spotID {
		t.Errorf("legs = {%s %s}, want {%s %s}", got.PerpOrder, got.SpotOrder, perpID, spotID)
	}
	// And the SEQUENCE, in record order. A rebuild carrying the final phase alone
	// would satisfy every assertion above while losing the transitions §7.5's own
	// replay rule requires to be re-checked.
	want := []domain.GenerationPhase{
		domain.GenerationPerpReduceInFlight,
		domain.GenerationPerpReduceFinal,
		domain.GenerationSpotReduceAuthorized,
		domain.GenerationSpotReduceInFlight,
	}
	if len(got.Steps) != len(want) {
		t.Fatalf("Steps = %v, want %v", got.Steps, want)
	}
	for i := range want {
		if got.Steps[i] != want[i] {
			t.Errorf("Steps[%d] = %s, want %s", i, got.Steps[i], want[i])
		}
	}
}

// TestTheFIRSTArbitrationToCloseALadderIsTheOneReported is
// domain.Generation.Supersede's first-writer-wins rule at group scope, and the
// mutation campaign is what asked for it.
//
// Both paths bar, so the SET of barred groups is identical either way and only
// the reported PATH differs — which is exactly the kind of claim a suite
// asserting "the ladder is barred" cannot see. What it decides is what an
// operator is told: "this package will not delever again" is a different
// conversation depending on whether the kill switch or the fence arbitration
// said so, and there is no other record of it once the generations are gone.
//
// The log is one this build can no longer write — the first bar refuses every
// later Open — which is why the fixture is scripted rather than driven. A rule
// about which of two facts survives is not testable from a world that can only
// produce one.
func TestTheFirstArbitrationToCloseALadderIsTheOneReported(t *testing.T) {
	t.Parallel()

	group := testHedgeGroup("BTCUSDT")
	r, _, _ := newReplayer(t,
		journalEntry{seq: 1, kind: "generation_opened", gen: "GEN-A",
			group: group, res: "RES-A", qty: testTarget(t, "1")},
		journalEntry{seq: 2, kind: "generation_superseded", gen: "GEN-A",
			path: domain.SupersededByEmergencyClose},
		journalEntry{seq: 3, kind: "generation_opened", gen: "GEN-B",
			group: group, res: "RES-B", qty: testTarget(t, "1")},
		journalEntry{seq: 4, kind: "generation_superseded", gen: "GEN-B",
			path: domain.SupersededByFenceReadyLost},
	)
	rec, err := r.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if len(rec.BarredLadders) != 1 {
		t.Fatalf("BarredLadders = %v, want one — the bar is per GROUP", rec.BarredLadders)
	}
	if got := rec.BarredLadders[0].Path; got != domain.SupersededByEmergencyClose {
		t.Errorf("BarredLadders[0].Path = %s, want the FIRST arbitration (%s)",
			got, domain.SupersededByEmergencyClose)
	}
}
