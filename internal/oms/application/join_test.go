package application_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file checks §7.5's venue join and the net-delta invariant (slice F2,
// record §56).
//
// # The collapse checklist, run before the tests were written
//
//  1. The perp leg carries TWO orders with OPPOSITE sides and DIFFERENT sizes —
//     a SELL of 0.5 and a BUY of 0.2, netting to −0.3. Every simpler fixture
//     makes a wrong fold pass: one order makes "sums the group" and "takes the
//     first" identical; two same-side orders make a dropped sign invisible; two
//     equal sizes make −0.3 and +0.3 and 0 hard to tell apart.
//  2. The expected net is NEGATIVE and the assertion states the literal. A
//     comparison of two computed values would pass if the subtraction ran in the
//     wrong order, since both sides would flip together.
//  3. The join is run TWICE in the idempotence test. A per-boot claim about
//     duplicates is unobservable inside the boot that admits the fills: the
//     dedupe's whole purpose is visible only on the NEXT one.
//  4. One order is attributed by CLIENT ORDER ID and one by VENUE ORDER ID, in
//     the same sweep. Binance's trade rows carry no client id at all, so a
//     fixture whose rows all name one would exercise only the key Bybit
//     supplies and leave the join unable to rebuild anything on Binance.
//  5. The sweep contains a row belonging to a DIFFERENT venue order id, so
//     "attributes what is ours" and "attributes everything in the window" are
//     different answers.
//  6. The orders are planted at a version that is not 1, and one of them is
//     TERMINAL (StateFilled), because the terminal orders are exactly what
//     Recovery.Dispatched adds over Recovery.Unresolved and where this system's
//     executions actually live.

// fakeJoinVenue is ports.OrderResolver with per-order answers.
//
// Per-order rather than one status for all, because the join's whole subject is
// several orders disagreeing with each other: an absent one, a swept one, one
// whose totals do not match. A single-answer fake would make every one of those
// tests a test of the same order.
type fakeJoinVenue struct {
	status map[ids.ClientOrderID]ports.VenueOrder
	err    map[ids.ClientOrderID]error

	sweeps   map[ids.VenueInstrument]ports.ExecutionSweep
	sweepErr error

	// windows records what each sweep was asked for — the only evidence that the
	// join built its window from the order's own anchor rather than from a
	// constant.
	windows  []sweepWindow
	resolves int
}

func newFakeJoinVenue() *fakeJoinVenue {
	return &fakeJoinVenue{
		status: make(map[ids.ClientOrderID]ports.VenueOrder),
		err:    make(map[ids.ClientOrderID]error),
		sweeps: make(map[ids.VenueInstrument]ports.ExecutionSweep),
	}
}

func (f *fakeJoinVenue) Resolve(o domain.Order) (ports.VenueOrder, error) {
	f.resolves++
	if err := f.err[o.ClientOrderID]; err != nil {
		return ports.VenueOrder{}, err
	}
	status, ok := f.status[o.ClientOrderID]
	if !ok {
		return ports.VenueOrder{Presence: ports.VenueHasNoOrder}, nil
	}
	return status, nil
}

func (f *fakeJoinVenue) SweepExecutions(o domain.Order, since, until time.Time) (ports.ExecutionSweep, error) {
	f.windows = append(f.windows, sweepWindow{since: since, until: until})
	if f.sweepErr != nil {
		return ports.ExecutionSweep{}, f.sweepErr
	}
	sweep, ok := f.sweeps[o.Instrument]
	if !ok {
		return ports.ExecutionSweep{ProvenThrough: until}, nil
	}
	if sweep.ProvenThrough.IsZero() {
		sweep.ProvenThrough = until
	}
	return sweep, nil
}

// fakePositions is ports.VenuePositions at the port.
type fakePositions struct {
	reports map[ports.PositionScope]ports.PositionReport
	err     error
	scopes  []ports.PositionScope
}

func newFakePositions() *fakePositions {
	return &fakePositions{reports: make(map[ports.PositionScope]ports.PositionReport)}
}

func (f *fakePositions) Positions(s ports.PositionScope) (ports.PositionReport, error) {
	f.scopes = append(f.scopes, s)
	if f.err != nil {
		return ports.PositionReport{}, f.err
	}
	return f.reports[s], nil
}

var (
	_ ports.OrderResolver  = (*fakeJoinVenue)(nil)
	_ ports.VenuePositions = (*fakePositions)(nil)
)

const joinAccount = "sub-1"

func perpScopeFor() ports.PositionScope {
	return ports.PositionScope{Venue: perp().Venue, Product: perp().Product, Settle: "USDT"}
}

// joinNow is the engine instant every fixture here runs at.
var joinNow = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func newJoiner(t *testing.T, scopes ...ports.PositionScope) (*application.Joiner, *fakeOrders, *fakeJoinVenue, *fakePositions) {
	t.Helper()
	orders := newFakeOrders()
	v := newFakeJoinVenue()
	p := newFakePositions()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	// The journal is constructed and dropped: the tests reached through this
	// helper are the pre-#242 ones, whose claims are about fills and the
	// net-delta assertion. The finality half needs to READ it, and takes
	// newJoinerWithJournal instead.
	j, err := application.NewJoiner(v, p, orders, newFakeJournal(), clk, joinAccount, application.SpotLeg{}, scopes...)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return j, orders, v, p
}

// newJoinerWithJournal is newJoiner plus the journal the state write goes to.
//
// The journal is a separate return rather than a field on the joiner's fakes
// because every claim in this file's finality half is a claim about TWO stores
// agreeing — a mutation that deletes the AppendState leaves the projection
// exactly right, so a test reading only the record cannot see it.
func newJoinerWithJournal(t *testing.T, scopes ...ports.PositionScope) (
	*application.Joiner, *fakeOrders, *fakeJoinVenue, *fakePositions, *fakeJournal) {
	t.Helper()
	orders := newFakeOrders()
	v := newFakeJoinVenue()
	p := newFakePositions()
	j := newFakeJournal()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	joiner, err := application.NewJoiner(v, p, orders, j, clk, joinAccount, application.SpotLeg{}, scopes...)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return joiner, orders, v, p, j
}

// venueSays is the ordinary present reading: the venue holds the order, states a
// lifecycle and a cumulative.
func venueSays(t *testing.T, state domain.OrderState, venueOrderID, executed string) ports.VenueOrder {
	t.Helper()
	return ports.VenueOrder{
		Presence:     ports.VenueHasOrder,
		State:        state,
		Executed:     qty(t, executed),
		VenueOrderID: venueOrderID,
	}
}

// stateOf reads one order's state out of the projection.
func stateOf(t *testing.T, r *fakeOrders, id ids.ClientOrderID) domain.OrderState {
	t.Helper()
	rec, err := r.Load(id)
	if err != nil {
		t.Fatalf("Load(%q): %v", id, err)
	}
	return rec.State
}

// lastApply returns the final recorded write for one order, which is where the
// version arithmetic is observable.
func (r *fakeOrders) lastApply(t *testing.T, id ids.ClientOrderID) applyCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.applied) - 1; i >= 0; i-- {
		if r.applied[i].id == id {
			return r.applied[i]
		}
	}
	t.Fatalf("no apply recorded for %q", id)
	return applyCall{}
}

// plantJoinable puts an order in the repository at a chosen state, with an
// anchor, at a version that is not 1.
func plantJoinable(t *testing.T, r *fakeOrders, o domain.Order, state domain.OrderState) {
	t.Helper()
	at, err := clock.NewServerTime(joinNow.Add(-time.Hour), testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	r.records[o.ClientOrderID] = ports.OrderRecord{
		Order: o, State: state, Version: 7, AuthorizedAt: at,

		// Zero OF THE ORDERED ASSET, which is what the port promises for a record
		// with no fills and what the real adapter produces (it reads the
		// cumulative back out of the ledger, whose zero is tagged at Register).
		// An untagged zero here would be a fake with a second opinion, and the
		// suite would then be testing a Joiner that has to defend against a
		// repository state the system cannot produce.
		Cumulative: fixedpoint.ZeroQuantity(o.Quantity.Asset()),
	}
}

func joinOrder(t *testing.T, leg int, instrument ids.VenueInstrument, side domain.Side, quantity string) domain.Order {
	t.Helper()
	return domain.Order{
		ClientOrderID: mintedID(t, leg),
		Instrument:    instrument,
		Side:          side,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, quantity),
		LimitPrice:    price(t, "50000"),
	}
}

// execution is one swept row, complete enough to become a domain.Fill — which
// is what the port carries since this slice, and what its narrowing used to
// forbid.
func execution(t *testing.T, executionID string, client ids.ClientOrderID, venueOrderID, quantity string) ports.VenueExecution {
	t.Helper()
	fee, err := fixedpoint.ParseMoney("USDT", "0.55")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	return ports.VenueExecution{
		ExecutionID:   ids.ExecutionID(executionID),
		ClientOrderID: client,
		VenueOrderID:  venueOrderID,
		Quantity:      qty(t, quantity),
		At:            joinNow.Add(-30 * time.Minute),
		Price:         price(t, "50000"),
		Fee:           fee,
	}
}

func present(venueOrderID, executed string, t *testing.T) ports.VenueOrder {
	t.Helper()
	return ports.VenueOrder{
		Presence:     ports.VenueHasOrder,
		State:        domain.StateFilled,
		Executed:     qty(t, executed),
		VenueOrderID: venueOrderID,
	}
}

// ---------------------------------------------------------------------------
// The invariant itself
// ---------------------------------------------------------------------------

// TestJoinRebuildsTheNetFromBothKeysAndAgreesWithTheVenue is the positive
// control: a clean account joins, both attribution keys work, and the report is
// empty. Without it, a Joiner that reported everything as unexplained would pass
// every refusal test below.
func TestJoinRebuildsTheNetFromBothKeysAndAgreesWithTheVenue(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())

	sell := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	buy := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, sell, domain.StateFilled)
	plantJoinable(t, orders, buy, domain.StateDispatchAuthorized)

	venue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	venue.status[buy.ClientOrderID] = present("V-2", "0.2", t)

	// One row names our client id (Bybit's shape), one names only the venue's
	// order id (Binance's), and one belongs to a venue order that is not ours.
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{
			execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5"),
			execution(t, "E-2", "", "V-2", "0.2"),
			execution(t, "E-3", "", "V-99", "3.0"),
		},
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.3")}},
	}

	report, err := joiner.Join([]ids.ClientOrderID{sell.ClientOrderID, buy.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if report.Orders != 2 || report.Fills != 2 || report.Duplicates != 0 {
		t.Errorf("orders/fills/duplicates = %d/%d/%d, want 2/2/0 — the third row belongs to venue order V-99 and must not be admitted",
			report.Orders, report.Fills, report.Duplicates)
	}
	if len(report.Absent) > 0 || len(report.Incomplete) > 0 || len(report.Unattributed) > 0 {
		t.Errorf("report = %+v, want a clean join", report)
	}
	assertion, err := joiner.Assert([]ids.ClientOrderID{sell.ClientOrderID, buy.ClientOrderID})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if assertion.Holds() {
		t.Errorf("assertion = %+v, want a clean net-delta assertion", assertion)
	}
	if got := orders.records[sell.ClientOrderID].Cumulative.Decimal(); got != "0.5" {
		t.Errorf("sell cumulative = %s, want 0.5 — attributed by client order id", got)
	}
	if got := orders.records[buy.ClientOrderID].Cumulative.Decimal(); got != "0.2" {
		t.Errorf("buy cumulative = %s, want 0.2 — attributed by VENUE order id, which is the only key Binance offers", got)
	}
}

// TestJoinReportsTheSignedUnexplainedDelta pins the number AND its direction.
func TestJoinReportsTheSignedUnexplainedDelta(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())

	sell := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	buy := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, sell, domain.StateFilled)
	plantJoinable(t, orders, buy, domain.StateFilled)

	venue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	venue.status[buy.ClientOrderID] = present("V-2", "0.2", t)
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{
			execution(t, "E-1", sell.ClientOrderID, "V-1", "0.5"),
			execution(t, "E-2", buy.ClientOrderID, "V-2", "0.2"),
		},
	}
	// Our orders explain −0.3. The venue holds −0.8, so 0.5 of short exposure
	// belongs to something this system did not place.
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.8")}},
	}

	if _, err := joiner.Join([]ids.ClientOrderID{sell.ClientOrderID, buy.ClientOrderID}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	report, err := joiner.Assert([]ids.ClientOrderID{sell.ClientOrderID, buy.ClientOrderID})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(report.Unexplained) != 1 {
		t.Fatalf("unexplained = %+v, want one instrument", report.Unexplained)
	}
	delta := report.Unexplained[0]
	if got := delta.Expected.Decimal(); got != "-0.3" {
		t.Errorf("expected = %s, want -0.3", got)
	}
	if got := delta.Observed.Decimal(); got != "-0.8" {
		t.Errorf("observed = %s, want -0.8", got)
	}
	// The literal, not a recomputation: a subtraction in the wrong order flips
	// both sides together and compares equal.
	if got := delta.Unexplained.Decimal(); got != "-0.5" {
		t.Errorf("unexplained = %s, want -0.5 (observed − expected)", got)
	}
	if !report.Holds() {
		t.Error("Holds() = false — §7.5 alerts for operator clearance and does not auto-correct")
	}
}

// TestJoinFindsAPositionOnAnInstrumentWeHoldNoOrdersFor is the half of the union
// walk that our own orders cannot reach.
func TestJoinFindsAPositionOnAnInstrumentWeHoldNoOrdersFor(t *testing.T) {
	t.Parallel()

	joiner, _, _, positions := newJoiner(t, perpScopeFor())
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions:    []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "1.25")}},
		Unrecognized: []string{"SOLUSDT"},
	}

	report, err := joiner.Assert(nil)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(report.Unexplained) != 1 || report.Unexplained[0].Unexplained.Decimal() != "1.25" {
		t.Errorf("unexplained = %+v, want the whole 1.25 — nothing we joined accounts for any of it", report.Unexplained)
	}
	if len(report.Foreign) != 1 || report.Foreign[0] != "SOLUSDT" {
		t.Errorf("foreign = %v, want [SOLUSDT]", report.Foreign)
	}
}

// TestJoinCrossChecksTheVenuesOwnTotal is the check that uses two venue surfaces
// rather than one: the order surface states a cumulative, the sweep states rows.
func TestJoinCrossChecksTheVenuesOwnTotal(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	sell := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, sell, domain.StateFilled)

	// The venue says this order executed 0.5; the sweep hands us 0.3 of it.
	venue.status[sell.ClientOrderID] = present("V-1", "0.5", t)
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", sell.ClientOrderID, "V-1", "0.3")},
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.3")}},
	}

	report, err := joiner.Join([]ids.ClientOrderID{sell.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(report.Unattributed) != 1 || report.Unattributed[0] != sell.ClientOrderID {
		t.Errorf("unattributed = %v, want the order whose totals disagree", report.Unattributed)
	}
	// The position agrees with what we rebuilt, so the delta check alone would
	// have reported nothing. That is the whole reason the cross-check exists —
	// and the reason it belongs to the JOIN's gate rather than the invariant's.
	assertion, err := joiner.Assert([]ids.ClientOrderID{sell.ClientOrderID})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(assertion.Unexplained) != 0 {
		t.Errorf("unexplained = %+v, want none — the position matches the rows we admitted", assertion.Unexplained)
	}
}

// ---------------------------------------------------------------------------
// What the join hands on rather than concluding
// ---------------------------------------------------------------------------

func TestJoinHandsAnAbsentOrderToTheStableNotFoundRule(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	order := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, order, domain.StateDispatchAuthorized)
	venue.status[order.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}
	positions.reports[perpScopeFor()] = ports.PositionReport{}

	report, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(report.Absent) != 1 || report.Absent[0] != order.ClientOrderID {
		t.Errorf("absent = %v, want the order the venue does not hold", report.Absent)
	}
	if len(report.Unattributed) != 0 {
		t.Errorf("unattributed = %v — an absence is §5.3's four-gate question, not an operator alert", report.Unattributed)
	}
	if len(venue.windows) != 0 {
		t.Errorf("swept %d windows for an order the venue says it does not hold", len(venue.windows))
	}
}

func TestJoinRefusesToConcludeOnATruncatedSweep(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	order := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, order, domain.StateFilled)
	venue.status[order.ClientOrderID] = present("V-1", "0.5", t)

	// A page budget spent: the rows are real, and coverage stops an hour short.
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions:    []ports.VenueExecution{execution(t, "E-1", order.ClientOrderID, "V-1", "0.5")},
		ProvenThrough: joinNow.Add(-time.Hour),
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{}

	report, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(report.Incomplete) != 1 {
		t.Errorf("incomplete = %v, want the order whose sweep fell short", report.Incomplete)
	}
	if report.Fills != 0 {
		t.Errorf("fills = %d, want 0 — a truncated sweep's rows rebuild a cumulative that is right about everything except what is missing", report.Fills)
	}
	if len(report.Unattributed) != 0 || len(report.Absent) != 0 {
		t.Errorf("report = %+v, want a short sweep and nothing else", report)
	}
}

func TestJoinRefusesAnOrderThatWasNeverDispatched(t *testing.T) {
	t.Parallel()

	joiner, orders, _, _ := newJoiner(t, perpScopeFor())
	order := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, order, domain.StatePlanned)

	_, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID})
	if !errors.Is(err, application.ErrOrderNotAtVenue) {
		t.Errorf("Join(planned order) = %v, want ErrOrderNotAtVenue", err)
	}
}

// ---------------------------------------------------------------------------
// The limit the report carries, and the property the next boot depends on
// ---------------------------------------------------------------------------

// TestJoinNamesTheLegItCannotAssert is finding 1 as an assertion, re-aimed by
// task #179: a spot leg with NO recorded baseline is reported
// UnassertedBaselineUnrecorded — and since the baseline construct made the gap
// closable, the old exemption is gone and the reason HOLDS the posture.
//
// # Slice G2 varied the fixture parameter this test had pinned
//
// It used to plant ONE order, on the spot leg, and assert that the Unasserted
// list held it and did not block. Every instrument that could land in Unasserted
// was therefore a spot one, so the two reasons were the same answer. It plants
// BOTH legs against a scope that covers the perp, so the spot leg is unasserted
// for its own reason while the perp is asserted.
func TestJoinNamesTheLegItCannotAssert(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	spotLeg := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	perpLeg := joinOrder(t, 2, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, spotLeg, domain.StateFilled)
	plantJoinable(t, orders, perpLeg, domain.StateFilled)
	venue.status[spotLeg.ClientOrderID] = present("V-1", "0.5", t)
	venue.status[perpLeg.ClientOrderID] = present("V-2", "0.5", t)
	venue.sweeps[spot()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", spotLeg.ClientOrderID, "V-1", "0.5")},
	}
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-2", perpLeg.ClientOrderID, "V-2", "0.5")},
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}

	ids2 := []ids.ClientOrderID{spotLeg.ClientOrderID, perpLeg.ClientOrderID}
	report, err := joiner.Join(ids2)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if report.Fills != 2 {
		t.Errorf("fills = %d, want 2 — the spot leg's executions are still rebuilt", report.Fills)
	}
	assertion, err := joiner.Assert(ids2)
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	want := []application.UnassertedInstrument{
		{Instrument: spot(), Reason: application.UnassertedBaselineUnrecorded},
	}
	if !reflect.DeepEqual(assertion.Unasserted, want) {
		t.Errorf("unasserted = %+v, want %+v — the spot leg alone, for the baseline it lacks",
			assertion.Unasserted, want)
	}
	// It HOLDS since #179: recording a baseline is a one-time operator act, so
	// the halts-forever argument no longer applies, and an unrecorded baseline
	// is a wallet the invariant never compared.
	if !assertion.Holds() {
		t.Errorf("assertion = %+v, want the unrecorded baseline to HOLD the posture", assertion)
	}
}

// TestAPerpNoScopeCoversHoldsThePosture is slice G2's finding as an assertion,
// and the control the finding was found by: the SAME bucket, reached by a
// derivative instrument, must not inherit the spot leg's exemption.
//
// The setup is the one Deps.PositionScopes calls legal — no scopes at all — and
// the harm is stated by what the venue holds: a real short position on an
// instrument this system never asked about. Before the reason existed, Assert
// reported it Unasserted and Holds() answered false, which is
// GateNetDeltaAsserted reporting Passed on a boot that issued no position query.
func TestAPerpNoScopeCoversHoldsThePosture(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t)

	perpLeg := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, perpLeg, domain.StateFilled)
	venue.status[perpLeg.ClientOrderID] = present("V-1", "0.5", t)
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", perpLeg.ClientOrderID, "V-1", "0.5")},
	}

	if _, err := joiner.Join([]ids.ClientOrderID{perpLeg.ClientOrderID}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	assertion, err := joiner.Assert([]ids.ClientOrderID{perpLeg.ClientOrderID})
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	want := []application.UnassertedInstrument{
		{Instrument: perp(), Reason: application.UnassertedScopeNotEnumerated},
	}
	if !reflect.DeepEqual(assertion.Unasserted, want) {
		t.Errorf("unasserted = %+v, want %+v", assertion.Unasserted, want)
	}
	if !assertion.Holds() {
		t.Errorf("assertion = %+v, want the invariant to HOLD: a position surface exists for this "+
			"instrument and was never queried", assertion)
	}
	// The evidence that no query was issued, which is the whole harm: the fake
	// records every scope it was asked for.
	if len(positions.scopes) != 0 {
		t.Errorf("position queries = %v, want none — that is what makes the pass a pass over nothing",
			positions.scopes)
	}
}

// TestUnassertedReasonsHoldOrExemptByTable pins the product this type decides,
// so a member added later cannot inherit an answer by omission.
//
// Both halves are asserted for each member: the posture answer AND that the
// reason validates. A table checking only HoldsPosture would pass against a
// member whose Validate was never wired, and the zero value is exactly such a
// member — it answers "holds" like the enumerated one and is an error besides.
func TestUnassertedReasonsHoldOrExemptByTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		reason  application.UnassertedReason
		holds   bool
		wantErr bool
	}{
		{application.UnassertedBaselineUnrecorded, true, false},
		{application.UnassertedScopeNotEnumerated, true, false},
		{application.UnassertedReasonUnspecified, true, true},
		{application.UnassertedReason(200), true, true},
	}
	for _, c := range cases {
		holds, err := c.reason.HoldsPosture()
		if holds != c.holds {
			t.Errorf("%s.HoldsPosture() = %v, want %v", c.reason, holds, c.holds)
		}
		if (err != nil) != c.wantErr {
			t.Errorf("%s.HoldsPosture() err = %v, wantErr %v", c.reason, err, c.wantErr)
		}
		if c.wantErr && !errors.Is(err, application.ErrUnassertedReasonInvalid) {
			t.Errorf("%s.HoldsPosture() err = %v, want ErrUnassertedReasonInvalid", c.reason, err)
		}
	}
}

// TestAnUnclassifiableUnassertedReasonHolds is the direction that costs: a
// report carrying a reason nothing recognises must alert rather than pass.
//
// It is asserted through Holds() rather than through HoldsPosture() alone,
// because Holds is what Reconciler reads and it is the place the error result is
// deliberately dropped — so this is the test that the drop kept the safe answer.
func TestAnUnclassifiableUnassertedReasonHolds(t *testing.T) {
	t.Parallel()

	a := application.Assertion{
		Unasserted: []application.UnassertedInstrument{
			{Instrument: perp(), Reason: application.UnassertedReason(200)},
		},
	}
	if !a.Holds() {
		t.Errorf("Holds() = false for an unrecognised reason, want true")
	}
}

// TestASecondJoinAdmitsNothingTwice is checklist item 3: the dedupe is
// unobservable in the boot that admits the fills.
func TestASecondJoinAdmitsNothingTwice(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	order := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, order, domain.StateFilled)
	venue.status[order.ClientOrderID] = present("V-1", "0.5", t)
	venue.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", order.ClientOrderID, "V-1", "0.5")},
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{
		Positions: []ports.VenuePosition{{Instrument: perp(), Net: signed(t, "-0.5")}},
	}

	first, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID})
	if err != nil {
		t.Fatalf("first Join: %v", err)
	}
	second, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID})
	if err != nil {
		t.Fatalf("second Join: %v", err)
	}

	if first.Fills != 1 || first.Duplicates != 0 {
		t.Errorf("first join fills/duplicates = %d/%d, want 1/0", first.Fills, first.Duplicates)
	}
	if second.Fills != 0 || second.Duplicates != 1 {
		t.Errorf("second join fills/duplicates = %d/%d, want 0/1 — re-derivation is what this join has instead of the live path's transaction",
			second.Fills, second.Duplicates)
	}
	if got := orders.records[order.ClientOrderID].Cumulative.Decimal(); got != "0.5" {
		t.Errorf("cumulative after two joins = %s, want 0.5", got)
	}
	if len(second.Unattributed) != 0 {
		t.Errorf("second join = %+v, want the same clean answer as the first", second)
	}
}

// TestJoinSweepsFromTheOrdersOwnAnchor pins the window's ends.
func TestJoinSweepsFromTheOrdersOwnAnchor(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	order := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, order, domain.StateFilled)
	venue.status[order.ClientOrderID] = present("V-1", "0.5", t)
	positions.reports[perpScopeFor()] = ports.PositionReport{}

	if _, err := joiner.Join([]ids.ClientOrderID{order.ClientOrderID}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(venue.windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(venue.windows))
	}
	anchor := orders.records[order.ClientOrderID].AuthorizedAt
	if !venue.windows[0].since.Equal(anchor.Earliest()) {
		t.Errorf("since = %s, want the anchor's EARLIEST edge %s — the order cannot have been created before it",
			venue.windows[0].since, anchor.Earliest())
	}
	// The venue clock's earliest edge, which is the conservative end: a window
	// claiming to reach further than the clock can prove makes a truncated sweep
	// look complete.
	wantUntil := joinNow.Add(testVenueOffset - testUncertainty)
	if !venue.windows[0].until.Equal(wantUntil) {
		t.Errorf("until = %s, want the venue clock's earliest edge %s", venue.windows[0].until, wantUntil)
	}
}

func TestNewJoinerRefusals(t *testing.T) {
	t.Parallel()

	orders := newFakeOrders()
	v := newFakeJoinVenue()
	p := newFakePositions()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{{Venue: perp().Venue, Offset: 0, Uncertainty: testUncertainty}})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}

	jr := newFakeJournal()

	if _, err := application.NewJoiner(nil, p, orders, jr, clk, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrResolverMissing) {
		t.Errorf("NewJoiner(no venue) = %v, want ErrResolverMissing", err)
	}
	if _, err := application.NewJoiner(v, nil, orders, jr, clk, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrVenuePositionsMissing) {
		t.Errorf("NewJoiner(no positions) = %v, want ErrVenuePositionsMissing", err)
	}
	if _, err := application.NewJoiner(v, p, nil, jr, clk, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewJoiner(no repository) = %v, want ErrOrdersMissing", err)
	}
	// Refused at construction rather than at the first call, for the reason
	// ErrJournalMissing already states: the call that would discover it is the
	// one whose whole purpose is a durable record.
	if _, err := application.NewJoiner(v, p, orders, nil, clk, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewJoiner(no journal) = %v, want ErrJournalMissing", err)
	}
	if _, err := application.NewJoiner(v, p, orders, jr, nil, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrClockMissing) {
		t.Errorf("NewJoiner(no clock) = %v, want ErrClockMissing", err)
	}
	if _, err := application.NewJoiner(v, p, orders, jr, clk, "", application.SpotLeg{}); !errors.Is(err, application.ErrJoinAccountUnset) {
		t.Errorf("NewJoiner(no account) = %v, want ErrJoinAccountUnset", err)
	}
	bad := ports.PositionScope{Venue: perp().Venue, Product: perp().Product}
	if _, err := application.NewJoiner(v, p, orders, jr, clk, joinAccount, application.SpotLeg{}, bad); !errors.Is(err, ports.ErrPositionScopeSettleUnset) {
		t.Errorf("NewJoiner(scope with no settlement asset) = %v, want ErrPositionScopeSettleUnset", err)
	}
}

// signed builds a signed base-asset quantity, which is what a venue position is.
func signed(t *testing.T, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", v)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", v, err)
	}
	return q
}

// ---------------------------------------------------------------------------
// the persisted ack meets the live id (task #174)
// ---------------------------------------------------------------------------

// TestJoinRefusesAStoreVenueIdTheLiveQueryContradicts: two authorities can now
// name one order's venue id — the ack the dispatch persisted and the answer
// the join's own query returns — and the loser of a disagreement decides which
// sweep rows become this order's fills. A non-empty mismatch refuses the JOIN,
// before any row is attributed.
func TestJoinRefusesAStoreVenueIdTheLiveQueryContradicts(t *testing.T) {
	t.Parallel()

	j, orders, v, _ := newJoiner(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateFilled)
	rec := orders.records[o.ClientOrderID]
	rec.VenueOrderID = "774411"
	orders.records[o.ClientOrderID] = rec

	v.status[o.ClientOrderID] = present("999999", "0.5", t)

	_, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if !errors.Is(err, application.ErrVenueOrderIDMismatch) {
		t.Fatalf("Join = %v, want ErrVenueOrderIDMismatch", err)
	}
	// Nothing was attributed for the disputed order: a fill applied under
	// either id would be a fill applied on a coin flip's authority.
	if len(v.windows) != 0 {
		t.Errorf("the join swept %v before settling whose id this is", v.windows)
	}
}

// TestJoinAttributesByThePersistedIdWhenTheSurfaceReturnsNone: past the
// mismatch check the two sources agree or one is silent, so the join
// attributes by whichever is non-empty — the persisted ack when a surface
// answered without an id. The row is Binance-shaped (no client id), so the
// venue id is the only key that can claim it.
func TestJoinAttributesByThePersistedIdWhenTheSurfaceReturnsNone(t *testing.T) {
	t.Parallel()

	j, orders, v, _ := newJoiner(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateFilled)
	rec := orders.records[o.ClientOrderID]
	rec.VenueOrderID = "774411"
	orders.records[o.ClientOrderID] = rec

	v.status[o.ClientOrderID] = present("", "0.5", t)
	v.sweeps[perp()] = ports.ExecutionSweep{
		Executions:    []ports.VenueExecution{execution(t, "e-1", "", "774411", "0.5")},
		ProvenThrough: joinNow.Add(time.Hour),
	}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if report.Fills != 1 {
		t.Errorf("Fills = %d, want 1 — the persisted id was the only key that could claim the row", report.Fills)
	}
	if len(report.Unattributed) != 0 {
		t.Errorf("Unattributed = %v, want none", report.Unattributed)
	}
}

// ---------------------------------------------------------------------------
// the venue's lifecycle reading becomes the projection's (task #242)
// ---------------------------------------------------------------------------

// TestJoinAdvancesEachOrderToItsOwnVenueReading is the core claim, and the
// fixture carries checklist items 1, 2, 3 and 6 in one run.
//
//   - TWO orders with DIFFERENT readings (1): one the venue reports Filled, one
//     still Acknowledged. "advances every order" and "advances the first" are
//     different answers here, and so are "writes the venue's reading" and
//     "writes a terminal" (2).
//   - The filled one carries a swept EXECUTION (3), so ApplyFill bumps its
//     version before the state write. A fixture with no fills makes "threads the
//     post-fill version" and "threads the loaded version" the same number.
//   - Both stores are asserted (6): a mutation deleting the AppendState leaves
//     the projection correct and only the journal assertion falls.
func TestJoinAdvancesEachOrderToItsOwnVenueReading(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	filled := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	live := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, filled, domain.StateSubmitted)
	plantJoinable(t, orders, live, domain.StateSubmitted)

	v.status[filled.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	v.status[live.ClientOrderID] = venueSays(t, domain.StateAcknowledged, "v-2", "0")
	v.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{
			execution(t, "e-1", filled.ClientOrderID, "v-1", "0.5"),
		},
	}

	report, err := j.Join([]ids.ClientOrderID{filled.ClientOrderID, live.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if report.Advanced != 2 {
		t.Errorf("Advanced = %d, want 2", report.Advanced)
	}
	if len(report.Contradicted) != 0 {
		t.Errorf("Contradicted = %v, want none", report.Contradicted)
	}

	if got := stateOf(t, orders, filled.ClientOrderID); got != domain.StateFilled {
		t.Errorf("projection holds %s for the filled order, want filled", got)
	}
	if got := stateOf(t, orders, live.ClientOrderID); got != domain.StateAcknowledged {
		t.Errorf("projection holds %s for the live order, want acknowledged", got)
	}

	// The journal, separately. Order is not asserted — Join walks the ids in the
	// order it was given, but the claim here is membership, and pinning the
	// sequence would make this test fail for a reason it is not about.
	want := map[domain.OrderState]int{domain.StateFilled: 1, domain.StateAcknowledged: 1}
	got := map[domain.OrderState]int{}
	for _, s := range jr.states() {
		got[s]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("journal holds %v, want %v", got, want)
	}
}

// TestJoinAdvancesAgainstThePostFillVersion is checklist item 3 stated as its own
// claim, because the test above would still pass if the version were read from
// the record as loaded: ApplyState would then be handed a stale version and the
// repository would refuse with ErrVersionConflict — which Join reports as an
// ERROR, so the failure is loud but its cause is one line away from the fixture.
func TestJoinAdvancesAgainstThePostFillVersion(t *testing.T) {
	t.Parallel()

	j, orders, v, _, _ := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted) // planted at version 7
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	v.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "e-1", o.ClientOrderID, "v-1", "0.5")},
	}

	if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}

	// The fill bumped 7 to 8, so the state write must claim 8. The applied-call
	// log is what makes this an assertion rather than an inference from the end
	// state.
	last := orders.lastApply(t, o.ClientOrderID)
	if last.expected != 8 {
		t.Errorf("ApplyState claimed version %d, want 8 (the fill bumped 7)", last.expected)
	}
	if last.to != domain.StateFilled {
		t.Errorf("ApplyState target = %s, want filled", last.to)
	}
}

// TestJoinRereadsATerminalOrderWithoutWriting is checklist item 4: the ORDINARY
// case on every boot after the first, and the one that makes the no-op
// short-circuit observable. Without it the record's state equals the venue's,
// CanTransition answers ErrTransitionNoOp, and a joiner that inspected that error
// instead of comparing first would file a healthy re-read under Contradicted and
// hold the boot posture at NeedsOperator for ever.
func TestJoinRereadsATerminalOrderWithoutWriting(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateFilled)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if report.Advanced != 0 {
		t.Errorf("Advanced = %d, want 0 — a re-read is not an advance", report.Advanced)
	}
	if len(report.Contradicted) != 0 {
		t.Errorf("Contradicted = %v, want none — the states AGREE", report.Contradicted)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinReportsAContradictoryVenueReading is checklist item 5, and the fixture
// is the ISOLATING one: both states are declared and valid, the venue HAS the
// order, and the sweep is complete — so the only thing in the function that can
// object is the edge check. A malformed state would trip Validate instead, and a
// probe two gates can see shows that something objected rather than that this
// rule did.
//
// Acknowledged -> Rejected is also the realistic accident: a rejection is a
// submission-time verdict, so a venue reporting one about an order we watched go
// live means the two readings are about different histories.
func TestJoinReportsAContradictoryVenueReading(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	bad := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	good := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, bad, domain.StateAcknowledged)
	plantJoinable(t, orders, good, domain.StateSubmitted)
	v.status[bad.ClientOrderID] = venueSays(t, domain.StateRejected, "v-1", "0")
	v.status[good.ClientOrderID] = venueSays(t, domain.StateFilled, "v-2", "0")

	report, err := j.Join([]ids.ClientOrderID{bad.ClientOrderID, good.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil — a contradiction on ONE order is a report entry", err)
	}
	if want := []ids.ClientOrderID{bad.ClientOrderID}; !reflect.DeepEqual(report.Contradicted, want) {
		t.Errorf("Contradicted = %v, want %v", report.Contradicted, want)
	}
	// The positive control in the same run: the healthy order advanced, so this
	// is not a joiner that refused everything.
	if report.Advanced != 1 {
		t.Errorf("Advanced = %d, want 1 — the second order is the positive control", report.Advanced)
	}
	if got := stateOf(t, orders, bad.ClientOrderID); got != domain.StateAcknowledged {
		t.Errorf("the contradicted order moved to %s, want acknowledged (unchanged)", got)
	}
	if states := jr.states(); len(states) != 1 || states[0] != domain.StateFilled {
		t.Errorf("journal holds %v, want only the healthy order's filled record", states)
	}
}

// TestJoinRefusesProvedAbsenceFromAPresentOrder guards an edge this very slice
// made legal. Submitted -> AbsentAfterFinality is now a domain edge, so nothing
// in CanTransition would stop the join writing "proved stable absence" from a
// venue that is holding the order in its hand. That is FLAG-2's evidence-class
// conflation, and §5.3 gives that terminal four gates the Resolver runs.
func TestJoinRefusesProvedAbsenceFromAPresentOrder(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateAbsentAfterFinality, "v-1", "0")

	_, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if !errors.Is(err, application.ErrVenueStateNotAnAbsence) {
		t.Fatalf("Join = %v, want ErrVenueStateNotAnAbsence", err)
	}
	if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted (unchanged)", got)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinDoesNotAdvanceAnAbsentOrder — checklist item 7. An absence is the one
// answer this use case must not act on: it is the INPUT to §5.3's four-gate rule,
// and the Resolver is what runs them. Asserting the journal is empty rather than
// only that Absent is populated is the point: the report field would be right
// even if a state had been written.
func TestJoinDoesNotAdvanceAnAbsentOrder(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Absent) != 1 || report.Advanced != 0 {
		t.Errorf("Absent=%v Advanced=%d, want one absence and no advance", report.Absent, report.Advanced)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing — an absence is the resolver's input", states)
	}
}

// TestJoinDoesNotAdvanceOnAnIncompleteSweep — checklist item 8, and the fixture
// is built so the two readings differ: the venue reports FILLED, which WOULD
// advance. A short-sweep fixture whose venue reading was already the record's
// would make "gated by the sweep" and "nothing to advance" the same answer.
//
// SM §2 calls the four-step contract "the ONE canonical boundary … never a
// weaker cancel-ack-only variant", and step 3 is the sweep. A terminal recorded
// from a status read alone collapses the possible-delta interval EARLY, which is
// exactly what SM §1's venue-initiated-termination row warns about.
func TestJoinDoesNotAdvanceOnAnIncompleteSweep(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
	v.sweeps[perp()] = ports.ExecutionSweep{ProvenThrough: joinNow.Add(-time.Hour)}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Incomplete) != 1 || report.Advanced != 0 {
		t.Errorf("Incomplete=%v Advanced=%d, want one incomplete and no advance",
			report.Incomplete, report.Advanced)
	}
	if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted — the sweep proved nothing", got)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinAdvancesDespiteAnUnattributedCumulative pins the ORTHOGONALITY §5.3
// requires, which is a decision rather than an accident. The cross-check
// disagreed about the AMOUNT; the venue's reading is about FINALITY, and §5.3
// keeps the two orthogonal ("nothing relates State to Cumulative"). Refusing to
// advance would leave the order in the state this slice exists to move it out
// of, to punish a number the report already carries — and the consumer that
// gates on both (#236's D4) already refuses on the report.
func TestJoinAdvancesDespiteAnUnattributedCumulative(t *testing.T) {
	t.Parallel()

	j, orders, v, _, _ := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	// The venue states 0.5 executed and the sweep attributes nothing.
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Unattributed) != 1 {
		t.Fatalf("Unattributed = %v, want one", report.Unattributed)
	}
	if report.Advanced != 1 {
		t.Errorf("Advanced = %d, want 1 — state and ledger are orthogonal", report.Advanced)
	}
}

// TestJoinErrorsWhenAStoreRefusesTheState pins Join's contract, which is the
// OPPOSITE of Resolver's and deliberately so. Resolver promises "err != nil means
// NOTHING was written", so it must report a partial write as a value. Join
// promises "err != nil means the projection is PART-REBUILT and must not be
// traded on", and the posture is what stops trading. A journal that did not take
// the record, or a projection that refused the version, IS that condition.
func TestJoinErrorsWhenAStoreRefusesTheState(t *testing.T) {
	t.Parallel()

	boom := errors.New("store refused")

	t.Run("journal", func(t *testing.T) {
		t.Parallel()
		j, orders, v, _, jr := newJoinerWithJournal(t)
		o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
		plantJoinable(t, orders, o, domain.StateSubmitted)
		v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
		jr.failOn[domain.StateFilled] = boom

		if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, boom) {
			t.Fatalf("Join = %v, want the journal's error", err)
		}
		// The apply is NOT attempted: legality is established before the journal
		// and the journal is what §7.5 replays, so a projection ahead of the log
		// is the one ordering this must never produce.
		if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
			t.Errorf("projection holds %s, want submitted — the log did not take it", got)
		}
	})

	t.Run("projection", func(t *testing.T) {
		t.Parallel()
		j, orders, v, _, _ := newJoinerWithJournal(t)
		o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
		plantJoinable(t, orders, o, domain.StateSubmitted)
		v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
		orders.failApplyState[domain.StateFilled] = boom

		if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, boom) {
			t.Fatalf("Join = %v, want the repository's error", err)
		}
	})
}

// TestJoinRefusesAMalformedVenueState is the branch above the edge check, and it
// exists because the two refusals are one line apart and produce DIFFERENT
// outcomes from the same-looking input. Without Validate the empty reading still
// stops the write — CanTransition refuses an undeclared target — but it stops it
// as a Contradicted entry, which files an ACL that returned presence without a
// lifecycle beside a venue that genuinely disagrees with us about an order's
// history. One is a translation defect and one is a venue fact, and an operator
// has one response for the bucket.
//
// The reading is otherwise complete — the venue HAS the order, the cumulative
// agrees, the sweep is whole — so the state is the only thing that can object.
func TestJoinRefusesAMalformedVenueState(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	// The zero OrderState: what an ACL that read a presence and no lifecycle
	// would hand over.
	v.status[o.ClientOrderID] = ports.VenueOrder{
		Presence: ports.VenueHasOrder, Executed: qty(t, "0"), VenueOrderID: "v-1",
	}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if !errors.Is(err, domain.ErrOrderStateUnset) {
		t.Fatalf("Join = %v, want ErrOrderStateUnset", err)
	}
	if len(report.Contradicted) != 0 {
		t.Errorf("Contradicted = %v, want none — a broken translation is not a disagreement",
			report.Contradicted)
	}
	if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted (unchanged)", got)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}
