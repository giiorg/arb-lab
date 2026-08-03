package application_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # The fakes, and the three properties the checklist demanded of them
//
// Each was chosen before the tests were written, because each closes a fixture
// collapse that would otherwise make several assertions vacuous.
//
//  1. The journal fails a NAMED append rather than every call. A fake that
//     fails everything cannot tell "refuses to place when authorization is not
//     durable" from "reports any journal error", and those are two different
//     claims about two different steps — one a safety property, one an alerting
//     one.
//  2. The placer COUNTS its calls. §5.3's rule is a count, and every other
//     assertion in this file is satisfied by a dispatcher that calls Place
//     twice.
//  3. The repository plants records at a CHOSEN version rather than always at
//     1. Every record a real ApplyPlanned creates is at version 1, so a suite
//     that only ever dispatches version-1 records cannot tell "compares the
//     caller's expected version" from "assumes the first version" — the two
//     predicates coincide on the only value the fixture ever holds. plantAt is
//     what separates them, and the applied-call log is what makes the
//     separation an assertion rather than an inference.

// journalCall is one durable record, in the order it was written.
type journalCall struct {
	id    ids.ClientOrderID
	state domain.OrderState
}

type fakeJournal struct {
	calls   []journalCall
	planned []domain.Order

	// failOn maps a target state to the error its append returns. Keyed by
	// state rather than by call index so a test names the step it is breaking.
	failOn map[domain.OrderState]error

	// failPlannedFrom, when positive, makes the Nth AppendPlanned (1-based) and
	// every later one fail. The planning path needs a failure at a CHOSEN leg,
	// which is what proves that a plan is journaled in full before any of it is
	// applied.
	failPlannedFrom int
	failPlannedErr  error
}

func newFakeJournal() *fakeJournal {
	return &fakeJournal{failOn: make(map[domain.OrderState]error)}
}

func (j *fakeJournal) AppendPlanned(o domain.Order) error {
	j.planned = append(j.planned, o)
	if j.failPlannedFrom > 0 && len(j.planned) >= j.failPlannedFrom {
		return j.failPlannedErr
	}
	return nil
}

func (j *fakeJournal) AppendState(id ids.ClientOrderID, state domain.OrderState) error {
	j.calls = append(j.calls, journalCall{id: id, state: state})
	return j.failOn[state]
}

// states returns what was durably recorded, in order.
func (j *fakeJournal) states() []domain.OrderState {
	out := make([]domain.OrderState, len(j.calls))
	for i, c := range j.calls {
		out[i] = c.state
	}
	return out
}

// plannedIDs returns the ids of the intents recorded, in order.
func (j *fakeJournal) plannedIDs() []ids.ClientOrderID {
	out := make([]ids.ClientOrderID, len(j.planned))
	for i, o := range j.planned {
		out[i] = o.ClientOrderID
	}
	return out
}

type fakePlacer struct {
	ack ports.Ack
	err error

	calls  int
	orders []domain.Order

	// watch, when set, is read at the moment Place is entered: appendsBefore
	// records how many journal appends had completed by then. That is the
	// journal-before-network ordering, asserted as a fact about interleaving
	// rather than inferred from the two call counts at the end.
	watch         *fakeJournal
	appendsBefore []int

	// urgencies records the §7.6 shed class each Place was given. It is its own
	// field rather than a check inside Place, because the claim is that Dispatch
	// threads the CALLER's class through — and a Place that ignored the
	// argument and a Dispatch that passed a constant look identical from the
	// order list alone.
	urgencies []ports.Urgency
}

func (p *fakePlacer) Place(o domain.Order, u ports.Urgency) (ports.Ack, error) {
	p.calls++
	p.orders = append(p.orders, o)
	p.urgencies = append(p.urgencies, u)
	if p.watch != nil {
		p.appendsBefore = append(p.appendsBefore, len(p.watch.calls))
	}
	return p.ack, p.err
}

// applyCall is one write to the current-state projection, recorded with the
// version it claimed. The version is the field the tests are about: it is the
// only evidence that Dispatch threaded the caller's number through rather than
// producing one of its own.
type applyCall struct {
	id       ids.ClientOrderID
	expected int64
	to       domain.OrderState
}

// fakeOrders is ports.OrderWriteRepository at the port, not the real adapter.
//
// The application layer's tests use doubles at its own ports — that is the
// hexagonal discipline and the reason hexboundary exists — and this one deviates
// from the real adapter in exactly one direction: it lets a test plant a record
// at a chosen version and fail a chosen write. It does NOT restate SM §1's edge
// set: legality is domain.CanTransition here as it is in the adapter, because a
// fake holding a second opinion about which transitions are legal would let this
// suite pass against a dispatcher the real repository refuses.
//
// The composition against the REAL adapter is oms/module_test.go's job, and it
// is why that file is an integration test rather than a wiring assertion.
type fakeOrders struct {
	records map[ids.ClientOrderID]ports.OrderRecord
	applied []applyCall

	// failApplyState maps a target state to the error its apply returns.
	failApplyState map[domain.OrderState]error

	// failLoad, when set, is returned by every Load. It is NOT
	// ports.ErrOrderUnknown: the planner has to tell "this id is free" from
	// "the repository could not answer", and a fake that could only produce the
	// first would leave the distinction untested.
	failLoad error

	// failApplyPlannedFrom, when positive, makes the Nth ApplyPlanned (1-based)
	// and every later one fail. A count rather than a flag, because the state
	// worth pinning is the PARTIAL one — some legs applied, one refused — and a
	// fake that failed the first call could never produce it.
	failApplyPlannedFrom int
	failApplyPlannedErr  error
	appliedPlanned       int
}

func newFakeOrders() *fakeOrders {
	return &fakeOrders{
		records:        make(map[ids.ClientOrderID]ports.OrderRecord),
		failApplyState: make(map[domain.OrderState]error),
	}
}

func (r *fakeOrders) ApplyPlanned(o domain.Order) (ports.OrderRecord, error) {
	r.appliedPlanned++
	if r.failApplyPlannedFrom > 0 && r.appliedPlanned >= r.failApplyPlannedFrom {
		return ports.OrderRecord{}, r.failApplyPlannedErr
	}
	// ports.OrderWriteRepository requires this refusal of every implementation,
	// and the dispatch path reads well-formedness from the fact that a record
	// exists — so a fake that skipped it would remove the check from the system
	// rather than from this double.
	if err := o.Validate(); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("fake repository: %w", err)
	}
	if _, ok := r.records[o.ClientOrderID]; ok {
		return ports.OrderRecord{}, fmt.Errorf("fake repository: order %q already exists", o.ClientOrderID)
	}
	rec := ports.OrderRecord{Order: o, State: domain.StatePlanned, Version: 1}
	r.records[o.ClientOrderID] = rec
	return rec, nil
}

func (r *fakeOrders) Load(id ids.ClientOrderID) (ports.OrderRecord, error) {
	if r.failLoad != nil {
		return ports.OrderRecord{}, r.failLoad
	}
	rec, ok := r.records[id]
	if !ok {
		return ports.OrderRecord{}, fmt.Errorf("load order %q: %w", id, ports.ErrOrderUnknown)
	}
	return rec, nil
}

// ApplyAuthorized mirrors the real adapter's split, including its refusal of an
// unset instant. A fake that accepted one would let this suite pass against a
// dispatcher that authorizes orders no resolver could ever answer for.
func (r *fakeOrders) ApplyAuthorized(id ids.ClientOrderID, expected int64, at clock.ServerTime) (ports.OrderRecord, error) {
	if err := at.Validate(); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("fake repository: %w: %w", ports.ErrAuthorizationNeedsAnchor, err)
	}
	rec, err := r.advance(id, expected, domain.StateDispatchAuthorized)
	if err != nil {
		return ports.OrderRecord{}, err
	}
	rec.AuthorizedAt = at
	r.records[id] = rec
	return rec, nil
}

// ApplyState refuses the authorization target, as the port requires of every
// implementation — the fake holds no second opinion about that either.
func (r *fakeOrders) ApplyState(id ids.ClientOrderID, expected int64, to domain.OrderState) (ports.OrderRecord, error) {
	if to == domain.StateDispatchAuthorized {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q to %s: %w", id, to, ports.ErrAuthorizationNeedsAnchor)
	}
	return r.advance(id, expected, to)
}

// advance is the shared load-check-mutate, mirroring the adapter's own split so
// that both entry points are policed identically here as there.
func (r *fakeOrders) advance(id ids.ClientOrderID, expected int64, to domain.OrderState) (ports.OrderRecord, error) {
	r.applied = append(r.applied, applyCall{id: id, expected: expected, to: to})
	if err := r.failApplyState[to]; err != nil {
		return ports.OrderRecord{}, err
	}
	rec, ok := r.records[id]
	if !ok {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q: %w", id, ports.ErrOrderUnknown)
	}
	if rec.Version != expected {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q: %w", id, ports.ErrVersionConflict)
	}
	if err := domain.CanTransition(rec.State, to); err != nil {
		return ports.OrderRecord{}, fmt.Errorf("advance order %q: %w", id, err)
	}
	rec.State = to
	rec.Version++
	r.records[id] = rec
	return rec, nil
}

// ApplyFill is unreachable from the dispatch and planning paths — §6's fill
// handler is a later slice — and returns a refusal rather than a zero value so
// that a use case which started calling it fails loudly here.
func (r *fakeOrders) ApplyFill(domain.Fill, int64) (ports.OrderRecord, bool, error) {
	return ports.OrderRecord{}, false, errors.New("fake repository: ApplyFill has no caller in this slice")
}

var (
	_ ports.Journal              = (*fakeJournal)(nil)
	_ ports.OrderPlacer          = (*fakePlacer)(nil)
	_ ports.OrderWriteRepository = (*fakeOrders)(nil)
)

// mintedID returns a ClientOrderID this system minted, built through the real
// constructor rather than written as a literal — a hand-written id that stops
// parsing after a format change would silently turn every admit test into a
// test of ErrOrderNotOMSMinted.
func mintedID(t *testing.T, leg int) ids.ClientOrderID {
	t.Helper()
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: testTrade(t), Leg: leg})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	return id
}

func testTrade(t *testing.T) ids.TradeID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	return trade
}

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

// spot is the cash-and-carry package's OTHER leg. It carries no ContractType,
// because ids.VenueInstrument requires one exactly when the product is a
// derivative — which is what makes a two-leg plan a fixture that varies the
// instrument rather than repeating it.
func spot() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
	}
}

func qty(t *testing.T, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", v)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", v, err)
	}
	return q
}

func price(t *testing.T, v string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", v)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", v, err)
	}
	return p
}

// dispatchableOrder is §6 step 6's own shape — a marketable-IOC limit — so
// every test below varies exactly the field it is about.
func dispatchableOrder(t *testing.T) domain.Order {
	t.Helper()
	return domain.Order{
		ClientOrderID: mintedID(t, 1),
		Instrument:    perp(),
		Side:          domain.SideSell,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, "0.5"),
		LimitPrice:    price(t, "50000"),
	}
}

// plantAt puts a record in the repository at a chosen state and version,
// standing in for the planning path this test file does not exercise.
//
// The version is a parameter rather than 1 for the reason the header states: a
// fixture pinned to the value ApplyPlanned always produces makes "threads the
// caller's version" and "assumes the first version" the same claim.
func plantAt(t *testing.T, r *fakeOrders, o domain.Order, state domain.OrderState, version int64) {
	t.Helper()
	r.records[o.ClientOrderID] = ports.OrderRecord{Order: o, State: state, Version: version}
}

// plant is the ordinary case: a freshly planned order, exactly as Plan leaves it.
func plant(t *testing.T, r *fakeOrders, o domain.Order) {
	t.Helper()
	plantAt(t, r, o, domain.StatePlanned, 1)
}

// newDispatcher builds a dispatcher over fresh fakes, with the placer watching
// the journal so every test gets the ordering assertion for free.
func newDispatcher(t *testing.T) (*application.Dispatcher, *fakeJournal, *fakePlacer, *fakeOrders) {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j}
	r := newFakeOrders()
	d, err := application.NewDispatcher(j, p, r, testClock(t))
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	return d, j, p, r
}

// testClock is calibrated for the venue every fixture here trades on, with a
// NONZERO uncertainty band.
//
// The band is not decoration. A ServerTime built with zero uncertainty makes
// Earliest, Instant and Latest the same value, so every claim about WHICH band
// edge a rule takes collapses into one — and §5.3's two time bounds deliberately
// take opposite edges. A zero-band fixture cannot tell the intended rule from
// either wrong one.
func testClock(t *testing.T) *clock.TestClock {
	t.Helper()
	clk, err := clock.NewTestClock(testNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: 250 * time.Millisecond, Uncertainty: 40 * time.Millisecond},
		{Venue: spot().Venue + "-other", Offset: 0, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return clk
}

// testNow is the engine instant every clock fixture starts at. A fixed literal
// rather than time.Now(), which §3.6 rule 5 forbids anyway, and which would make
// every duration assertion below depend on when the suite ran.
var testNow = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func TestNewDispatcherRefusesAMissingPort(t *testing.T) {
	t.Parallel()

	// One omission at a time and a DIFFERENT sentinel per case: together they
	// catch a constructor that checks only the first port, and one that answers
	// a single blanket refusal to all four.
	if _, err := application.NewDispatcher(nil, &fakePlacer{}, newFakeOrders(), testClock(t)); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewDispatcher(nil, placer, orders, clock) = %v, want ErrJournalMissing", err)
	}
	if _, err := application.NewDispatcher(newFakeJournal(), nil, newFakeOrders(), testClock(t)); !errors.Is(err, application.ErrPlacerMissing) {
		t.Errorf("NewDispatcher(journal, nil, orders, clock) = %v, want ErrPlacerMissing", err)
	}
	if _, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, nil, testClock(t)); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewDispatcher(journal, placer, nil, clock) = %v, want ErrOrdersMissing", err)
	}
	if _, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, newFakeOrders(), nil); !errors.Is(err, application.ErrClockMissing) {
		t.Errorf("NewDispatcher(journal, placer, orders, nil) = %v, want ErrClockMissing", err)
	}

	// The positive control. Without it a constructor that refused everything
	// would score four out of four above.
	if _, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, newFakeOrders(), testClock(t)); err != nil {
		t.Errorf("NewDispatcher with every port = %v, want nil", err)
	}
}

func TestDispatchPlacesAValidOrderExactlyOnce(t *testing.T) {
	t.Parallel()

	d, j, p, r := newDispatcher(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	o := dispatchableOrder(t)
	plant(t, r, o)

	out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want exactly 1", p.calls)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if out.Ack != p.ack {
		t.Errorf("Ack = %+v, want %+v", out.Ack, p.ack)
	}
	if out.Cause != nil {
		t.Errorf("Cause = %v, want nil on a clean place", out.Cause)
	}
	if !out.Journaled {
		t.Error("Journaled = false, want true")
	}
	if !out.Applied {
		t.Error("Applied = false, want true")
	}

	// Both records, in order, under the order's own id. The first is the crash
	// boundary; the second is the result.
	wantStates := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted}
	if got := j.states(); !equalStates(got, wantStates) {
		t.Errorf("journal recorded %v, want %v", got, wantStates)
	}
	for _, c := range j.calls {
		if c.id != o.ClientOrderID {
			t.Errorf("journal recorded id %q, want %q", c.id, o.ClientOrderID)
		}
	}

	// The order that reached the venue is the one the REPOSITORY held, which is
	// the one §6 step 5 journaled. Nothing the caller passed could have
	// influenced it — the caller passed an id.
	if len(p.orders) == 1 && p.orders[0] != o {
		t.Errorf("Place received %+v, want %+v", p.orders[0], o)
	}

	// The projection followed, and ends where the venue put it.
	final, err := r.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load after dispatch: %v", err)
	}
	if final.State != domain.StateSubmitted {
		t.Errorf("repository state = %s, want %s", final.State, domain.StateSubmitted)
	}
	if final.Version != 3 {
		t.Errorf("repository version = %d, want 3 — one bump per applied write", final.Version)
	}
}

// TestDispatchThreadsTheCallersVersionThroughBothApplies is the OCC claim, and
// the fixture is deliberately at a version no ApplyPlanned would produce.
//
// At version 1 the two readings coincide: a dispatcher that threaded `expected`
// and one that hardcoded the first version write the same two numbers. At 5 they
// diverge, and the applied-call log names which was written.
func TestDispatchThreadsTheCallersVersionThroughBothApplies(t *testing.T) {
	t.Parallel()

	d, _, _, r := newDispatcher(t)
	o := dispatchableOrder(t)
	plantAt(t, r, o, domain.StatePlanned, 5)

	out, err := d.Dispatch(o.ClientOrderID, 5, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if !out.Applied {
		t.Fatal("Applied = false, want true")
	}

	want := []applyCall{
		{id: o.ClientOrderID, expected: 5, to: domain.StateDispatchAuthorized},
		{id: o.ClientOrderID, expected: 6, to: domain.StateSubmitted},
	}
	if len(r.applied) != len(want) {
		t.Fatalf("applied %+v, want %+v", r.applied, want)
	}
	for i := range want {
		if r.applied[i] != want[i] {
			t.Errorf("apply %d = %+v, want %+v", i, r.applied[i], want[i])
		}
	}
}

// TestDispatchJournalsAuthorizationBeforePlacing asserts the interleaving, not
// the totals.
//
// §6 step 6 fsyncs DispatchAuthorized "immediately before the single Place",
// and the reason is a crash between the two: a send whose authorization is not
// on disk is an order recovery cannot find. A dispatcher that placed first and
// journaled both records afterwards would satisfy every count in the test above.
func TestDispatchJournalsAuthorizationBeforePlacing(t *testing.T) {
	t.Parallel()

	d, j, p, r := newDispatcher(t)
	o := dispatchableOrder(t)
	plant(t, r, o)

	if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("Dispatch = %v", err)
	}
	if len(p.appendsBefore) != 1 {
		t.Fatalf("Place entered %d times, want 1", len(p.appendsBefore))
	}
	if p.appendsBefore[0] != 1 {
		t.Errorf("%d journal appends had completed when Place was entered, want 1",
			p.appendsBefore[0])
	}
	if len(j.calls) == 0 || j.calls[0].state != domain.StateDispatchAuthorized {
		t.Errorf("first journal record = %v, want %s", j.states(), domain.StateDispatchAuthorized)
	}
}

// An UNCALIBRATED venue is refused ABOVE the fsync, and the assertion that
// matters is what did NOT happen.
//
// The dispatcher's own anchor check survived the battery: ApplyAuthorized
// refuses the same instant a second time, so deleting the check here still
// produced an error and every test that only inspected the error passed. What
// changes is the ORDER — with the check gone, the authorization is journaled and
// the order is SENT, and only then does the projection write fail. §6 step 6
// puts every caller-controlled refusal on the same side of that fsync precisely
// because the other side leaves a durable authorization for an order §5.3 must
// then resolve by querying a venue about an id it may never have seen.
//
// A missing calibration is caller-controlled: it is deployment configuration.
func TestDispatchRefusesAnUncalibratedVenueAboveTheFsync(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	p := &fakePlacer{watch: j}
	r := newFakeOrders()
	// Calibrated for SOME venue — an empty calibration list would test a clock
	// that knows nothing rather than one that does not know THIS venue, and the
	// first is not a configuration a running system produces.
	clk, err := clock.NewTestClock(testNow, []clock.Calibration{
		{Venue: perp().Venue + "-elsewhere", Offset: 0, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	d, err := application.NewDispatcher(j, p, r, clk)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	o := dispatchableOrder(t)
	plant(t, r, o)

	if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); !errors.Is(err, ports.ErrAuthorizationNeedsAnchor) {
		t.Fatalf("Dispatch on an uncalibrated venue = %v, want ErrAuthorizationNeedsAnchor", err)
	}
	if len(j.calls) != 0 {
		t.Errorf("journaled %v, want nothing — the refusal belongs ABOVE the fsync", j.states())
	}
	if p.calls != 0 {
		t.Errorf("placed %d orders, want 0", p.calls)
	}

	// And the record did not move, so the caller may fix the calibration and
	// dispatch the same order.
	rec, err := r.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.State != domain.StatePlanned || rec.Version != 1 {
		t.Errorf("record is at %s v%d, want %s v1", rec.State, rec.Version, domain.StatePlanned)
	}

	// The positive control: the SAME order through a calibrated clock sends.
	// Without it, a dispatcher refusing every venue would pass every assertion
	// above.
	d2, j2, p2, r2 := newDispatcher(t)
	plant(t, r2, o)
	if _, err := d2.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("Dispatch on a calibrated venue = %v, want nil", err)
	}
	if p2.calls != 1 || len(j2.calls) == 0 {
		t.Errorf("calibrated: placed %d and journaled %v, want 1 place and a journal", p2.calls, j2.states())
	}
}

// TestDispatchRefusesBeforeSendingAnything covers the checks admit makes on the
// order it loaded.
//
// Every case plants the offending order rather than passing it, because that is
// now the only way one can exist — and it is the realistic provenance: §6 step
// 10's reconciler puts foreign orders in this same repository, which is exactly
// why "we loaded it from our own store" is not evidence that we minted it.
//
// The type and the time in force vary INDEPENDENTLY: a fixture wrong in both at
// once cannot tell which comparison fired, and a suite of only such fixtures
// would pass against a dispatcher that checked either one alone.
func TestDispatchRefusesBeforeSendingAnything(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*domain.Order)
		want   error
	}{
		{
			name:   "a venue-minted id",
			mutate: func(o *domain.Order) { o.ClientOrderID = "autoclose-1712345678901" },
			want:   application.ErrOrderNotOMSMinted,
		},
		{
			// Right shape, right prefix, right length — and a UUID that is not
			// a v7. This is the id the domain layer's own fixtures use, and it
			// is here to prove the check is ParseClientOrderID rather than a
			// prefix test somebody could write instead.
			name:   "a well-shaped id this system did not mint",
			mutate: func(o *domain.Order) { o.ClientOrderID = "ab-0123456789ABCDEFGHJKMNPQRS-00-00" },
			want:   application.ErrOrderNotOMSMinted,
		},
		{
			name:   "the wrong type, with the right time in force",
			mutate: func(o *domain.Order) { o.Type = domain.OrderTypeMarket; o.LimitPrice = fixedpoint.Price{} },
			want:   application.ErrNotMarketableIOCLimit,
		},
		{
			name:   "the right type, with the wrong time in force",
			mutate: func(o *domain.Order) { o.TimeInForce = domain.TimeInForceGTC },
			want:   application.ErrNotMarketableIOCLimit,
		},
		{
			name:   "another wrong time in force, to show the check is not a GTC test",
			mutate: func(o *domain.Order) { o.TimeInForce = domain.TimeInForceFOK },
			want:   application.ErrNotMarketableIOCLimit,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, j, p, r := newDispatcher(t)
			o := dispatchableOrder(t)
			tc.mutate(&o)
			plant(t, r, o)

			out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
			if err == nil {
				t.Fatal("Dispatch = nil, want a refusal")
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("Dispatch = %v, want %v", err, tc.want)
			}
			// The refusal's whole value: nothing left the process, nothing
			// reached the disk and nothing moved in the projection, so §5.3's
			// "safe to replan or discard" holds.
			if p.calls != 0 {
				t.Errorf("Place called %d times on a refused order, want 0", p.calls)
			}
			if len(j.calls) != 0 {
				t.Errorf("journal recorded %v on a refused order, want nothing", j.states())
			}
			if len(r.applied) != 0 {
				t.Errorf("repository saw %+v on a refused order, want nothing", r.applied)
			}
			if out != (application.Outcome{}) {
				t.Errorf("Outcome = %+v on a refusal, want the zero value", out)
			}
		})
	}
}

// TestDispatchRefusesAnOrderTheRepositoryCannotSupply is the refusal that only
// exists because Dispatch loads what it sends — §6 step 5's ordering, enforced
// by the shape of the call rather than by a caller's discipline.
//
// The three cases are the three ways a load can fail to produce a dispatchable
// record, and they are separate because they send a reader to three different
// places: never planned, planned but moved on, planned but at another version.
func TestDispatchRefusesAnOrderTheRepositoryCannotSupply(t *testing.T) {
	t.Parallel()

	t.Run("an order that was never planned", func(t *testing.T) {
		t.Parallel()

		d, j, p, _ := newDispatcher(t)

		_, err := d.Dispatch(mintedID(t, 1), 1, ports.UrgencyOpening)
		if !errors.Is(err, ports.ErrOrderUnknown) {
			t.Fatalf("Dispatch of an unplanned order = %v, want ErrOrderUnknown", err)
		}
		if p.calls != 0 || len(j.calls) != 0 {
			t.Errorf("Place called %d times and journal recorded %v, want nothing", p.calls, j.states())
		}
	})

	t.Run("a stale version", func(t *testing.T) {
		t.Parallel()

		d, j, p, r := newDispatcher(t)
		o := dispatchableOrder(t)
		plantAt(t, r, o, domain.StatePlanned, 4)

		_, err := d.Dispatch(o.ClientOrderID, 3, ports.UrgencyOpening)
		if !errors.Is(err, ports.ErrVersionConflict) {
			t.Fatalf("Dispatch at a stale version = %v, want ErrVersionConflict", err)
		}
		if p.calls != 0 || len(j.calls) != 0 {
			t.Errorf("Place called %d times and journal recorded %v, want nothing", p.calls, j.states())
		}
	})

	t.Run("an order past Planned", func(t *testing.T) {
		t.Parallel()

		// The realistic instance: this order was dispatched by a previous
		// process, and §5.3 says such an order is resolved by query only. A
		// dispatcher that re-authorized it would be the double-send the whole
		// path exists to prevent, and the refusal comes from SM §1's edge set
		// rather than from a rule restated here.
		d, j, p, r := newDispatcher(t)
		o := dispatchableOrder(t)
		plantAt(t, r, o, domain.StateDispatchAuthorized, 2)

		_, err := d.Dispatch(o.ClientOrderID, 2, ports.UrgencyOpening)
		if !errors.Is(err, domain.ErrTransitionNoOp) {
			t.Fatalf("Dispatch of an already-authorized order = %v, want ErrTransitionNoOp", err)
		}
		if p.calls != 0 || len(j.calls) != 0 {
			t.Errorf("Place called %d times and journal recorded %v, want nothing", p.calls, j.states())
		}
	})
}

// TestDispatchRefusesToPlaceWhenAuthorizationIsNotDurable is the safety claim of
// step 2, and it is the reason the journal fake fails a NAMED append.
func TestDispatchRefusesToPlaceWhenAuthorizationIsNotDurable(t *testing.T) {
	t.Parallel()

	diskFull := errors.New("no space left on device")
	d, j, p, r := newDispatcher(t)
	j.failOn[domain.StateDispatchAuthorized] = diskFull
	o := dispatchableOrder(t)
	plant(t, r, o)

	out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
	if !errors.Is(err, diskFull) {
		t.Fatalf("Dispatch = %v, want the journal's error", err)
	}
	if p.calls != 0 {
		t.Errorf("Place called %d times after a failed authorization append, want 0", p.calls)
	}
	if len(r.applied) != 0 {
		t.Errorf("repository saw %+v, want nothing — the apply is downstream of the fsync", r.applied)
	}
	if out != (application.Outcome{}) {
		t.Errorf("Outcome = %+v, want the zero value when nothing was sent", out)
	}
}

// TestDispatchReportsAnUnjournaledResultWithoutFailing is step 6, and it is the
// claim the whole-journal-fails fake would have collapsed into the one above.
//
// A failure here is not a safety failure: the journal still holds
// DispatchAuthorized, which recovery resolves by query — exactly right for an
// order whose fate could not be recorded. So err stays nil, the Outcome is
// authoritative, and Journaled is what reports the alerting condition.
func TestDispatchReportsAnUnjournaledResultWithoutFailing(t *testing.T) {
	t.Parallel()

	d, j, p, r := newDispatcher(t)
	j.failOn[domain.StateSubmitted] = errors.New("no space left on device")
	o := dispatchableOrder(t)
	plant(t, r, o)

	out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil — a lost result record is not a failed dispatch", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want 1", p.calls)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if out.Journaled {
		t.Error("Journaled = true after the result append failed, want false")
	}
	// Applied stays TRUE, and the pair is the point: the two fields report
	// independent stores, and a Dispatch that collapsed them into one flag would
	// pass every assertion above.
	if !out.Applied {
		t.Error("Applied = false, want true — the projection is unaffected by a journal failure")
	}
	// The authorization record survives, and it is what recovery reads.
	if got := j.states(); len(got) == 0 || got[0] != domain.StateDispatchAuthorized {
		t.Errorf("journal = %v, want the authorization record to remain", got)
	}
}

// TestDispatchSendsWhenTheProjectionFails is the ordering consequence that is
// least obvious and most load-bearing.
//
// The authorization fsync is the commitment to call Place exactly once. So a
// repository failure AFTER it must not abort the send: refusing there would
// leave a durable DispatchAuthorized record for an order nothing ever sent, and
// §5.3 then resolves it by querying a venue that never heard of it — a
// permanently unresolvable order, produced by being careful.
func TestDispatchSendsWhenTheProjectionFails(t *testing.T) {
	t.Parallel()

	t.Run("the authorization apply fails", func(t *testing.T) {
		t.Parallel()

		d, _, p, r := newDispatcher(t)
		r.failApplyState[domain.StateDispatchAuthorized] = errors.New("projection unavailable")
		o := dispatchableOrder(t)
		plant(t, r, o)

		out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
		if err != nil {
			t.Fatalf("Dispatch = %v, want nil — the send was already authorized", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
		if out.Applied {
			t.Error("Applied = true, want false")
		}
		if !out.Journaled {
			t.Error("Journaled = false, want true — the journal is what recovery reads")
		}
		// The result apply is SKIPPED rather than attempted against a guessed
		// version: exactly one call reached the repository.
		if len(r.applied) != 1 {
			t.Errorf("repository saw %+v, want only the authorization attempt", r.applied)
		}
	})

	t.Run("the result apply fails", func(t *testing.T) {
		t.Parallel()

		d, _, p, r := newDispatcher(t)
		r.failApplyState[domain.StateSubmitted] = errors.New("projection unavailable")
		o := dispatchableOrder(t)
		plant(t, r, o)

		out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
		if err != nil {
			t.Fatalf("Dispatch = %v, want nil", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
		if out.Applied {
			t.Error("Applied = true, want false")
		}
		if len(r.applied) != 2 {
			t.Errorf("repository saw %+v, want both attempts", r.applied)
		}
	})
}

// TestDispatchClassifiesThePlaceOutcome walks the Effect × Class table through
// the public entry point.
//
// The EffectNotApplied rows include ClassVenueReject AND three non-reject
// classes. With only one, "checks the class" and "ignores the class" give the
// same answer — and the three are different constructors rather than three
// spellings of one, because §5.3's whole point is that NotApplied covers both
// "never left the socket" and "refused on its merits".
func TestDispatchClassifiesThePlaceOutcome(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")

	cases := []struct {
		name      string
		placeErr  error
		wantState domain.OrderState
	}{
		{
			name:      "a clean place is Submitted",
			placeErr:  nil,
			wantState: domain.StateSubmitted,
		},
		{
			name:      "an applied effect is Submitted, because only the ack was lost",
			placeErr:  apperror.Transport("place", apperror.EffectApplied, boom),
			wantState: domain.StateSubmitted,
		},
		{
			name:      "an explicit venue rejection is the Rejected terminal",
			placeErr:  apperror.VenueRejected("place", "-2010", "insufficient margin", boom),
			wantState: domain.StateRejected,
		},
		{
			name:      "not-applied over transport is SubmissionUnknown, not Rejected",
			placeErr:  apperror.Transport("place", apperror.EffectNotApplied, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "our own precondition refusal is SubmissionUnknown, not Rejected",
			placeErr:  apperror.Precondition("place", "governor declined", boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "a rate-limit refusal is SubmissionUnknown, not Rejected",
			placeErr:  apperror.RateLimited("place", "429", 0, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "an unknown effect is SubmissionUnknown",
			placeErr:  apperror.Transport("place", apperror.EffectUnknown, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			// An adapter that forgot to classify. apperror answers
			// EffectUnknown for anything it never saw, so the order goes to the
			// venue to ask rather than to a terminal.
			name:      "an unclassified error is SubmissionUnknown",
			placeErr:  boom,
			wantState: domain.StateSubmissionUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, j, p, r := newDispatcher(t)
			p.err = tc.placeErr
			o := dispatchableOrder(t)
			plant(t, r, o)

			out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
			if err != nil {
				t.Fatalf("Dispatch = %v, want nil — Place ran, so there is a state to report", err)
			}
			if p.calls != 1 {
				t.Errorf("Place called %d times, want exactly 1 — there is no transport retry", p.calls)
			}
			if out.State != tc.wantState {
				t.Errorf("State = %s, want %s", out.State, tc.wantState)
			}
			if tc.placeErr == nil {
				if out.Cause != nil {
					t.Errorf("Cause = %v, want nil", out.Cause)
				}
			} else if !errors.Is(out.Cause, tc.placeErr) {
				t.Errorf("Cause = %v, want it to wrap %v", out.Cause, tc.placeErr)
			}
			// A declared effect is never the fallback.
			if errors.Is(out.Cause, application.ErrEffectUnhandled) {
				t.Errorf("Cause = %v, want no ErrEffectUnhandled for a declared effect", out.Cause)
			}
			if !out.Journaled {
				t.Error("Journaled = false, want true")
			}
			want := []domain.OrderState{domain.StateDispatchAuthorized, tc.wantState}
			if got := j.states(); !equalStates(got, want) {
				t.Errorf("journal recorded %v, want %v", got, want)
			}
			// Every classification the dispatcher can produce is a legal
			// successor of DispatchAuthorized, so the projection follows it.
			// This is the assertion that would fail if a future member of the
			// table named a state SM §1 cannot reach from the crash boundary.
			if !out.Applied {
				t.Errorf("Applied = false for %s — the projection refused a state the dispatcher produced", tc.wantState)
			}
		})
	}
}

// TestDispatchRefusesASecondDispatchOfTheSameOrder uses a VALID order whose
// first dispatch SUCCEEDED, so "refuses a repeat" cannot collapse into "refuses
// an invalid order".
func TestDispatchRefusesASecondDispatchOfTheSameOrder(t *testing.T) {
	t.Parallel()

	d, j, p, r := newDispatcher(t)
	o := dispatchableOrder(t)
	plant(t, r, o)

	if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("first Dispatch = %v, want nil", err)
	}

	// Version 3 is where the first dispatch left the record, so this second call
	// is the one a caller who re-read the record would make — not one holding a
	// stale number. The refusal is therefore the guard's, not the OCC's.
	out, err := d.Dispatch(o.ClientOrderID, 3, ports.UrgencyOpening)
	if !errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Fatalf("second Dispatch = %v, want ErrOrderAlreadyDispatched", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want 1 — the second dispatch must send nothing", p.calls)
	}
	if len(j.calls) != 2 {
		t.Errorf("journal recorded %v, want only the first dispatch's two records", j.states())
	}
	if out != (application.Outcome{}) {
		t.Errorf("Outcome = %+v, want the zero value", out)
	}
}

// TestASecondDispatcherOverTheSameRepositoryIsAlsoRefused isolates the OTHER
// mechanism, and the isolation is the point.
//
// The in-process guard and the repository both refuse a repeat, so a single test
// shows that something objected rather than that either rule did. A fresh
// Dispatcher has an empty guard set by construction — which §5.3 requires, since
// a restarted process must query rather than re-dispatch — so what refuses here
// can only be SM §1's edge set, reached through the record the first dispatch
// left behind.
func TestASecondDispatcherOverTheSameRepositoryIsAlsoRefused(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	p := &fakePlacer{}
	r := newFakeOrders()
	o := dispatchableOrder(t)
	plant(t, r, o)

	first, err := application.NewDispatcher(j, p, r, testClock(t))
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if _, err := first.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("first Dispatch = %v", err)
	}

	second, err := application.NewDispatcher(j, p, r, testClock(t))
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	_, err = second.Dispatch(o.ClientOrderID, 3, ports.UrgencyOpening)
	if errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Fatal("a fresh dispatcher refused with ErrOrderAlreadyDispatched — its guard set must start empty")
	}
	if !errors.Is(err, domain.ErrTransitionForbidden) && !errors.Is(err, domain.ErrTransitionNoOp) {
		t.Fatalf("second dispatcher = %v, want SM §1's refusal", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times across both dispatchers, want 1", p.calls)
	}
}

// TestDispatchAdmitsADifferentOrder is the guard's positive control: it is keyed
// on the ClientOrderID, not a latch that closes after one dispatch.
func TestDispatchAdmitsADifferentOrder(t *testing.T) {
	t.Parallel()

	d, _, p, r := newDispatcher(t)
	first := dispatchableOrder(t)
	second := dispatchableOrder(t)
	second.ClientOrderID = mintedID(t, 0) // the other leg of the same trade
	plant(t, r, first)
	plant(t, r, second)

	if _, err := d.Dispatch(first.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("first Dispatch = %v", err)
	}
	if _, err := d.Dispatch(second.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("Dispatch of the other leg = %v, want nil", err)
	}
	if p.calls != 2 {
		t.Errorf("Place called %d times, want 2", p.calls)
	}
}

// TestDispatchDoesNotBurnTheIDOnARefusal covers the placement of the guard's
// write.
//
// The id is recorded only AFTER the authorization append succeeds, because
// everything before that point sent nothing — so a caller that corrects its
// mistake and calls again has not double-dispatched anything. Recording it in
// admit would look equivalent and would permanently reject an order whose only
// fault was a stale read.
func TestDispatchDoesNotBurnTheIDOnARefusal(t *testing.T) {
	t.Parallel()

	t.Run("after an admit refusal", func(t *testing.T) {
		t.Parallel()

		d, _, p, r := newDispatcher(t)
		o := dispatchableOrder(t)
		plant(t, r, o)

		// The SAME id both times: the refusal is the caller's stale version, and
		// the correction is re-reading it. A test using two different orders
		// would prove only that the guard is keyed on something.
		if _, err := d.Dispatch(o.ClientOrderID, 7, ports.UrgencyOpening); !errors.Is(err, ports.ErrVersionConflict) {
			t.Fatalf("Dispatch at a stale version = %v, want ErrVersionConflict", err)
		}
		if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
			t.Fatalf("Dispatch after re-reading = %v, want nil", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
	})

	t.Run("after a failed authorization append", func(t *testing.T) {
		t.Parallel()

		d, j, p, r := newDispatcher(t)
		j.failOn[domain.StateDispatchAuthorized] = errors.New("transient")
		o := dispatchableOrder(t)
		plant(t, r, o)

		if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err == nil {
			t.Fatal("Dispatch with a failing journal = nil, want a refusal")
		}
		// Nothing was sent, so a retry is legitimate — and this is the case the
		// guard must NOT swallow, since the alternative is an order that can
		// never be placed because its own journal hiccupped once.
		delete(j.failOn, domain.StateDispatchAuthorized)
		if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
			t.Fatalf("Dispatch after the journal recovered = %v, want nil", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
	})
}

// TestDispatchGuardsAgainstARepeatAfterAFailedPlace is the misuse the signature
// invites, and the reason the guard exists at all: err == nil with a non-nil
// Cause reads like "that failed, try again", and it is the one reading that
// must not send a second order.
func TestDispatchGuardsAgainstARepeatAfterAFailedPlace(t *testing.T) {
	t.Parallel()

	d, _, p, r := newDispatcher(t)
	p.err = apperror.Transport("place", apperror.EffectUnknown, errors.New("timeout"))
	o := dispatchableOrder(t)
	plant(t, r, o)

	out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if out.Cause == nil {
		t.Fatal("Cause = nil, want the timeout")
	}
	if _, err := d.Dispatch(o.ClientOrderID, 3, ports.UrgencyOpening); !errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Errorf("retry after a failed Place = %v, want ErrOrderAlreadyDispatched", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want exactly 1", p.calls)
	}
}

// TestDispatchGuardHoldsWhenTheProjectionDidNot is why the in-process guard was
// not deleted when the repository started refusing repeats.
//
// With the authorization apply broken, the record is still at StatePlanned — so
// the repository would happily admit this order a second time, for an order that
// has already reached the venue. The guard is the mechanism precisely in the
// case where the other mechanism is the thing that broke.
func TestDispatchGuardHoldsWhenTheProjectionDidNot(t *testing.T) {
	t.Parallel()

	d, _, p, r := newDispatcher(t)
	r.failApplyState[domain.StateDispatchAuthorized] = errors.New("projection unavailable")
	o := dispatchableOrder(t)
	plant(t, r, o)

	out, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if out.Applied {
		t.Fatal("Applied = true, want false — the fixture depends on the apply having failed")
	}
	// The record is untouched, so the repository has no objection to make.
	rec, err := r.Load(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.State != domain.StatePlanned || rec.Version != 1 {
		t.Fatalf("record = %s/v%d, want planned/v1 — the fixture's premise", rec.State, rec.Version)
	}

	if _, err := d.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening); !errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Errorf("second Dispatch = %v, want ErrOrderAlreadyDispatched", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want exactly 1", p.calls)
	}
}

func equalStates(got, want []domain.OrderState) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
