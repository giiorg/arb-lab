package application_test

import (
	"errors"
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
	j, err := application.NewJoiner(v, p, orders, clk, joinAccount, scopes...)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return j, orders, v, p
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
	if report.NeedsOperator() || report.Retryable() {
		t.Errorf("report = %+v, want a clean join", report)
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

	report, err := joiner.Join([]ids.ClientOrderID{sell.ClientOrderID, buy.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
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
	if !report.NeedsOperator() {
		t.Error("NeedsOperator() = false — §7.5 alerts for operator clearance and does not auto-correct")
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

	report, err := joiner.Join(nil)
	if err != nil {
		t.Fatalf("Join: %v", err)
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
	if !report.NeedsOperator() {
		t.Error("NeedsOperator() = false — an execution that exists and cannot be named is exposure")
	}
	// The position agrees with what we rebuilt, so the delta check alone would
	// have reported nothing. That is the whole reason the cross-check exists.
	if len(report.Unexplained) != 0 {
		t.Errorf("unexplained = %+v, want none — the position matches the rows we admitted", report.Unexplained)
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
	if report.NeedsOperator() {
		t.Error("NeedsOperator() = true — an absence is §5.3's four-gate question, not an operator alert")
	}
	if !report.Retryable() {
		t.Error("Retryable() = false — the stable-not-found rule has not run yet")
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
	if !report.Retryable() || report.NeedsOperator() {
		t.Errorf("report = %+v, want retryable and no operator", report)
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

// TestJoinNamesTheLegItCannotAssert is finding 1 as an assertion: a spot leg has
// no position surface anywhere, so the invariant covers one leg of a
// cash-and-carry package and the report has to say so.
func TestJoinNamesTheLegItCannotAssert(t *testing.T) {
	t.Parallel()

	joiner, orders, venue, positions := newJoiner(t, perpScopeFor())
	spotLeg := joinOrder(t, 1, spot(), domain.SideBuy, "0.5")
	plantJoinable(t, orders, spotLeg, domain.StateFilled)
	venue.status[spotLeg.ClientOrderID] = present("V-1", "0.5", t)
	venue.sweeps[spot()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "E-1", spotLeg.ClientOrderID, "V-1", "0.5")},
	}
	positions.reports[perpScopeFor()] = ports.PositionReport{}

	report, err := joiner.Join([]ids.ClientOrderID{spotLeg.ClientOrderID})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if report.Fills != 1 {
		t.Errorf("fills = %d, want 1 — the spot leg's executions are still rebuilt", report.Fills)
	}
	if len(report.Unasserted) != 1 || report.Unasserted[0] != spot() {
		t.Errorf("unasserted = %v, want the spot leg", report.Unasserted)
	}
	// It must not block: a leg whose invariant can never be asserted would hold
	// the posture below trading for ever.
	if report.NeedsOperator() || report.Retryable() {
		t.Errorf("report = %+v, want the limit reported rather than enforced", report)
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
	if second.NeedsOperator() {
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

	if _, err := application.NewJoiner(nil, p, orders, clk, joinAccount); !errors.Is(err, application.ErrResolverMissing) {
		t.Errorf("NewJoiner(no venue) = %v, want ErrResolverMissing", err)
	}
	if _, err := application.NewJoiner(v, nil, orders, clk, joinAccount); !errors.Is(err, application.ErrVenuePositionsMissing) {
		t.Errorf("NewJoiner(no positions) = %v, want ErrVenuePositionsMissing", err)
	}
	if _, err := application.NewJoiner(v, p, nil, clk, joinAccount); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewJoiner(no repository) = %v, want ErrOrdersMissing", err)
	}
	if _, err := application.NewJoiner(v, p, orders, nil, joinAccount); !errors.Is(err, application.ErrClockMissing) {
		t.Errorf("NewJoiner(no clock) = %v, want ErrClockMissing", err)
	}
	if _, err := application.NewJoiner(v, p, orders, clk, ""); !errors.Is(err, application.ErrJoinAccountUnset) {
		t.Errorf("NewJoiner(no account) = %v, want ErrJoinAccountUnset", err)
	}
	bad := ports.PositionScope{Venue: perp().Venue, Product: perp().Product}
	if _, err := application.NewJoiner(v, p, orders, clk, joinAccount, bad); !errors.Is(err, ports.ErrPositionScopeSettleUnset) {
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
