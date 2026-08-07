package application_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// # The executor's fixtures, and the two collapses §59f names
//
// Every ordering or routing claim below uses THREE legs, because a two-leg
// fixture cannot tell "reversed" from "swapped" — boot's three-closer finding,
// one leg down. And every assertion about a leg keys on its ClientOrderID,
// never on slice position, because position agreeing with minted order is a
// coincidence of how a fixture happens to be written.
//
// The identical-quantity fixtures exist in BOTH directions: an equal package
// that executes and an unequal one that refuses. A suite with only the equal
// fixture cannot tell the §6 step 6 check from no check at all.

// packageOrder mints one leg of the test package: instrument and side vary by
// leg so that a swap is visible in every dimension the package has.
func packageOrder(t *testing.T, leg int, quantity string) domain.Order {
	t.Helper()
	o := domain.Order{
		ClientOrderID: mintedID(t, leg),
		Instrument:    spot(),
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, quantity),
		LimitPrice:    price(t, "50100"),
	}
	if leg%2 == 1 {
		o.Instrument = perp()
		o.Side = domain.SideSell
		o.LimitPrice = price(t, "49900")
	}
	return o
}

// newExecutor builds an executor over the shared fakes, through the dispatcher
// its constructor requires — the executor inherits that dispatcher's authority
// by construction, which is the signature's whole point.
func newExecutor(t *testing.T) (*application.Executor, *fakeJournal, *fakePlacer, *fakeOrders, *application.Dispatcher) {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j}
	r := newFakeOrders()
	d, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(d, newFakeReserver(t))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	return e, j, p, r, d
}

type fakeReserver struct {
	governor *ratelimit.Governor
	calls    int
	err      error
	vector   func(*ratelimit.Hold, int) []*ratelimit.Hold
	returned []*ratelimit.Hold
}

func newFakeReserver(t *testing.T) *fakeReserver {
	t.Helper()
	g, err := ratelimit.New(testClock(t), ratelimit.Profile{
		Venue: "binance", Families: []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp},
		Limits: []ratelimit.Limit{{
			Name: "ip_5s", Scope: ratelimit.ScopeVenue, Dimension: ratelimit.DimensionRequests,
			Interval: time.Second, OpeningCeiling: 100, RoutineCeiling: 100, Capacity: 100,
		}},
		Uncounted: []ratelimit.Uncounted{
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionWeight, Reason: "fixture requests are request-only"},
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "fixture requests are request-only"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionWeight, Reason: "fixture requests are request-only"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionOrders, Reason: "fixture requests are request-only"},
		},
		ThrottlePause: time.Second, BanPause: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	return &fakeReserver{governor: g}
}

func (r *fakeReserver) ReservePlacements(orders []domain.Order, u ports.Urgency) ([]*ratelimit.Hold, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if r.returned != nil {
		return r.returned, nil
	}
	priority := ratelimit.PriorityOpening
	if u == ports.UrgencySafety {
		priority = ratelimit.PrioritySafety
	}
	reqs := make([]ratelimit.Request, len(orders))
	for i, order := range orders {
		reqs[i] = ratelimit.Request{
			Family: order.Instrument.Product, Priority: priority, Cost: ratelimit.RequestOnly(),
		}
	}
	hold, decision, err := r.governor.Reserve(reqs)
	if err != nil {
		return nil, err
	}
	if !decision.Admitted {
		return nil, decision.Err()
	}
	if r.vector != nil {
		return r.vector(hold, len(orders)), nil
	}
	holds := make([]*ratelimit.Hold, len(orders))
	for i := range holds {
		holds[i] = hold
	}
	return holds, nil
}

func executorWithReserver(t *testing.T, reserver ports.CapacityReserver) (*application.Executor, *fakeJournal, *fakePlacer, *fakeOrders) {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j, ack: ports.Ack{VenueOrderID: "V-1"}}
	r := newFakeOrders()
	d, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(d, reserver)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	return e, j, p, r
}

func mintExecutorHold(t *testing.T, order domain.Order) (*ratelimit.Hold, *ratelimit.Governor) {
	t.Helper()
	r := newFakeReserver(t)
	holds, err := r.ReservePlacements([]domain.Order{order}, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("ReservePlacements: %v", err)
	}
	return holds[0], r.governor
}

func reservedTotal(g *ratelimit.Governor) uint64 {
	var total uint64
	for _, l := range g.Snapshot().Limits {
		total += l.Reserved
	}
	return total
}

// bootedChecklist is a checklist in the state a clean §7.5 boot leaves it for
// this executor: the net-delta gate concluded Passed, and a second gate left
// Unreported — the producer-less shape every real deployment's roster has, so
// every test below also quietly asserts that silence does not constrain the
// admission read. Fresh per call: two Execute calls that must share a latch
// share the VALUE, and the ones that must not (the authority probes) take a
// fresh one, each with a comment saying why.
func bootedChecklist(t *testing.T) *boot.Checklist {
	t.Helper()
	c, err := boot.NewChecklist(boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomePassed); err != nil {
		t.Fatalf("Report(NetDeltaAsserted, Passed): %v", err)
	}
	return c
}

// refs builds DispatchRefs at version 1 — where Plan leaves every record — in
// the order given.
func refsFor(orders ...domain.Order) []application.DispatchRef {
	out := make([]application.DispatchRef, len(orders))
	for i, o := range orders {
		out[i] = application.DispatchRef{ID: o.ClientOrderID, Version: 1}
	}
	return out
}

// legByID finds a leg's result by identity; failing to find it is a test
// failure, not a zero value.
func legByID(t *testing.T, ex application.Execution, id ids.ClientOrderID) application.LegResult {
	t.Helper()
	for _, leg := range ex.Legs {
		if leg.ID == id {
			return leg
		}
	}
	t.Fatalf("execution has no result for %q; legs: %+v", id, ex.Legs)
	return application.LegResult{}
}

func TestNewExecutorRefusesAMissingDispatcher(t *testing.T) {
	t.Parallel()

	// One refusal where five used to be: the executor is built FROM the
	// dispatcher, whose own constructor already refused every nil port, so the
	// only missing collaborator this constructor can meet is the dispatcher.
	if _, err := application.NewExecutor(nil, newFakeReserver(t)); !errors.Is(err, application.ErrDispatcherMissing) {
		t.Errorf("NewExecutor(nil) = %v, want ErrDispatcherMissing", err)
	}

	// The positive control, without which a constructor refusing everything
	// scores above.
	d, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, newFakeOrders(),
		testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if _, err := application.NewExecutor(d, newFakeReserver(t)); err != nil {
		t.Errorf("NewExecutor(dispatcher) = %v, want nil", err)
	}
	if _, err := application.NewExecutor(d, nil); !errors.Is(err, application.ErrReserverMissing) {
		t.Errorf("NewExecutor without reserver = %v, want ErrReserverMissing", err)
	}
}

func TestExecuteDispatchesEveryLegExactlyOnce(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	ex, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}

	if p.calls != 2 {
		t.Errorf("Place called %d times, want 2", p.calls)
	}
	for _, o := range []domain.Order{legSpot, legPerp} {
		res := legByID(t, ex, o.ClientOrderID)
		if res.Err != nil {
			t.Errorf("leg %q Err = %v, want nil", o.ClientOrderID, res.Err)
		}
		if res.Outcome.State != domain.StateSubmitted {
			t.Errorf("leg %q State = %s, want StateSubmitted", o.ClientOrderID, res.Outcome.State)
		}
		if !res.Outcome.Journaled || !res.Outcome.Applied {
			t.Errorf("leg %q Journaled/Applied = %v/%v, want true/true",
				o.ClientOrderID, res.Outcome.Journaled, res.Outcome.Applied)
		}
	}
	if ex.Dispatched() != 2 {
		t.Errorf("Dispatched() = %d, want 2", ex.Dispatched())
	}
	if ex.HoldsNewTrading() {
		t.Error("HoldsNewTrading() = true on a package whose every apply succeeded")
	}

	// Each leg's authorization was durable BEFORE its Place — the per-leg
	// ordering §6 step 6 fixes, asserted from the interleaving watch. With two
	// legs in flight the count observed at a given Place is at least the one
	// authorization that leg needed; it can be up to three (the other leg's
	// records landing first), and zero is the defect.
	for i, before := range p.appendsBefore {
		if before < 1 {
			t.Errorf("Place %d entered with %d journal appends completed, want >= 1", i, before)
		}
	}
	// Both legs' full record triples are in the journal — the ack sits between
	// each leg's authorization and its result.
	if len(j.calls) != 6 {
		t.Errorf("journal holds %d records, want 6 (authorized+acked+state per leg): %v", len(j.calls), j.states())
	}
}

// TestExecuteRunsLegsInParallel is the claim in §6 step 6's word "parallel": no
// leg's Place waits for another leg's Place to return.
//
// The gate placer blocks every Place until released. If the executor
// dispatched serially, the second arrival could never happen while the first
// is held — so the test waits, BOUNDED, for two concurrent arrivals before
// releasing anything. Every wait has a deadline arm because a mutation that
// serialises the fan-out must fail this test rather than hang it (§59f).
func TestExecuteRunsLegsInParallel(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	arrivals := make(chan ids.ClientOrderID, 8)
	release := make(chan struct{})
	p := &gatePlacer{arrivals: arrivals, release: release}
	r := newFakeOrders()
	gated, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(gated, newFakeReserver(t))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	// Built on the test goroutine: the helper may t.Fatal, which only the test
	// goroutine is permitted to do.
	checklist := bootedChecklist(t)

	done := make(chan application.Execution, 1)
	go func() {
		ex, err := e.Execute(checklist, refsFor(legSpot, legPerp), ports.UrgencyOpening)
		if err != nil {
			t.Errorf("Execute = %v, want nil", err)
		}
		done <- ex
	}()

	var released sync.Once
	free := func() { released.Do(func() { close(release) }) }
	defer free()

	for i := 0; i < 2; i++ {
		select {
		case <-arrivals:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d legs in flight after 5s — the fan-out is not parallel", i)
		}
	}
	free()

	select {
	case ex := <-done:
		if ex.Dispatched() != 2 {
			t.Errorf("Dispatched() = %d, want 2", ex.Dispatched())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after release")
	}
}

// gatePlacer signals each Place's entry and blocks it until released. It is
// concurrency-safe by construction: both fields are channels.
type gatePlacer struct {
	arrivals chan ids.ClientOrderID
	release  chan struct{}
}

func (p *gatePlacer) Place(o domain.Order, _ ports.Urgency, _ *ratelimit.Hold) (ports.Ack, error) {
	p.arrivals <- o.ClientOrderID
	<-p.release
	return ports.Ack{VenueOrderID: "V-" + string(o.ClientOrderID)}, nil
}

// TestExecuteThreadsTheCallersUrgency mirrors Dispatch's own claim one level
// up: the class is the caller's, per §6 step 6, and reaches every leg's Place.
func TestExecuteThreadsTheCallersUrgency(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	if _, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), ports.UrgencySafety); err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if len(p.urgencies) != 2 {
		t.Fatalf("Place saw %d urgencies, want 2", len(p.urgencies))
	}
	for i, u := range p.urgencies {
		if u != ports.UrgencySafety {
			t.Errorf("Place %d urgency = %s, want safety", i, u)
		}
	}
}

func TestExecuteReservesAfterPreflightAndBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	reserver := newFakeReserver(t)
	e, j, p, r := executorWithReserver(t, reserver)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	bad := refsFor(leg)
	bad[0].Version = 9
	if _, err := e.Execute(bootedChecklist(t), bad, ports.UrgencyOpening); err == nil {
		t.Fatal("preflight-invalid package was admitted")
	}
	if reserver.calls != 0 {
		t.Fatalf("ReservePlacements called %d times before preflight passed, want 0", reserver.calls)
	}
	refusalAsserts(t, j, p)

	reserver.err = errors.New("capacity unavailable")
	if _, err := e.Execute(bootedChecklist(t), refsFor(leg), ports.UrgencyOpening); !errors.Is(err, reserver.err) {
		t.Fatalf("reserve refusal = %v, want %v", err, reserver.err)
	}
	if reserver.calls != 1 {
		t.Fatalf("ReservePlacements calls = %d, want 1", reserver.calls)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteThreadsTheIndexAlignedHoldForEachLeg(t *testing.T) {
	t.Parallel()
	first := packageOrder(t, 0, "0.5")
	second := packageOrder(t, 1, "0.5")
	h0, _ := mintExecutorHold(t, first)
	h1, _ := mintExecutorHold(t, second)
	reserver := newFakeReserver(t)
	reserver.returned = []*ratelimit.Hold{h0, h1}
	e, _, p, r := executorWithReserver(t, reserver)
	plant(t, r, first)
	plant(t, r, second)

	if _, err := e.Execute(bootedChecklist(t), refsFor(first, second), ports.UrgencyOpening); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(p.orders) != 2 || len(p.holds) != 2 {
		t.Fatalf("Place observations = %d orders/%d holds, want 2/2", len(p.orders), len(p.holds))
	}
	want := map[ids.ClientOrderID]*ratelimit.Hold{first.ClientOrderID: h0, second.ClientOrderID: h1}
	for i, order := range p.orders {
		if p.holds[i] != want[order.ClientOrderID] {
			t.Errorf("order %q received hold %p, want %p", order.ClientOrderID, p.holds[i], want[order.ClientOrderID])
		}
	}
}

func TestExecuteRefusesEveryInvalidHoldVectorAndCancelsDistinctHolds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		shape func([]*ratelimit.Hold) []*ratelimit.Hold
	}{
		{"short", func(h []*ratelimit.Hold) []*ratelimit.Hold { return h[:1] }},
		{"nil-bearing", func(h []*ratelimit.Hold) []*ratelimit.Hold { return []*ratelimit.Hold{h[0], nil} }},
		{"long", func(h []*ratelimit.Hold) []*ratelimit.Hold { return h }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := packageOrder(t, 0, "0.5")
			second := packageOrder(t, 1, "0.5")
			extra := packageOrder(t, 2, "0.5")
			h0, g0 := mintExecutorHold(t, first)
			h1, g1 := mintExecutorHold(t, second)
			h2, g2 := mintExecutorHold(t, extra)
			all := []*ratelimit.Hold{h0, h1, h2}
			reserver := newFakeReserver(t)
			reserver.returned = tc.shape(all)
			e, j, p, r := executorWithReserver(t, reserver)
			plant(t, r, first)
			plant(t, r, second)

			if _, err := e.Execute(bootedChecklist(t), refsFor(first, second), ports.UrgencyOpening); !errors.Is(err, application.ErrHoldVectorInvalid) {
				t.Fatalf("Execute = %v, want ErrHoldVectorInvalid", err)
			}
			refusalAsserts(t, j, p)
			for i, g := range []*ratelimit.Governor{g0, g1, g2} {
				want := uint64(0)
				if tc.name != "long" && i == 2 {
					want = 1 // this hold was not returned, so the executor does not own it.
				}
				if tc.name == "short" && i == 1 {
					want = 1
				}
				if tc.name == "nil-bearing" && i == 1 {
					want = 1
				}
				if got := reservedTotal(g); got != want {
					t.Errorf("governor %d Reserved = %d, want %d", i, got, want)
				}
			}
			for _, h := range all {
				_ = h.Cancel()
			}
		})
	}
}

type mixedHoldPlacer struct {
	governor *ratelimit.Governor
}

func (p *mixedHoldPlacer) Place(o domain.Order, u ports.Urgency, hold *ratelimit.Hold) (ports.Ack, error) {
	if o.Side == domain.SideSell {
		// This is the leg's own pre-send admission refusal. Its entry remains
		// reserved for Executor's blanket deferred Cancel to release.
		return ports.Ack{}, apperror.Precondition("fixture.Place", "leg admission", errors.New("fixture refusal"))
	}
	priority := ratelimit.PriorityOpening
	if u == ports.UrgencySafety {
		priority = ratelimit.PrioritySafety
	}
	decision, err := hold.Consume(ratelimit.Request{
		Family: o.Instrument.Product, Priority: priority, Cost: ratelimit.RequestOnly(),
	})
	if err != nil {
		return ports.Ack{}, err
	}
	if !decision.Admitted {
		return ports.Ack{}, decision.Err()
	}
	return ports.Ack{VenueOrderID: "V-consumed"}, nil
}

// TestExecuteCancelsOnlyTheUnsentRemainderOnAMixedResult is the load-bearing
// blanket-cleanup fixture. One leg consumes its reservation and one refuses at
// its own pre-send admit; after Execute returns the first leg's exact charge
// survives while the second leg's untimed occupancy is gone.
func TestExecuteCancelsOnlyTheUnsentRemainderOnAMixedResult(t *testing.T) {
	t.Parallel()
	reserver := newFakeReserver(t)
	journal := newFakeJournal()
	orders := newFakeOrders()
	placer := &mixedHoldPlacer{governor: reserver.governor}
	dispatcher, err := application.NewDispatcher(
		journal, placer, orders, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge,
	)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	executor, err := application.NewExecutor(dispatcher, reserver)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	first := packageOrder(t, 0, "0.5")
	second := packageOrder(t, 1, "0.5")
	plant(t, orders, first)
	plant(t, orders, second)
	execution, err := executor.Execute(
		bootedChecklist(t), refsFor(first, second), ports.UrgencyOpening,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(execution.Legs) != 2 {
		t.Fatalf("Execution.Legs = %d, want 2", len(execution.Legs))
	}
	if got := legByID(t, execution, first.ClientOrderID).Outcome.State; got != domain.StateSubmitted {
		t.Errorf("consumed leg state = %s, want StateSubmitted", got)
	}
	if got := legByID(t, execution, second.ClientOrderID).Outcome.State; got != domain.StateSubmissionUnknown {
		t.Errorf("refused leg state = %s, want StateSubmissionUnknown", got)
	}
	for _, state := range reserver.governor.Snapshot().Limits {
		if state.Used != 1 || state.Reserved != 0 {
			t.Errorf("limit %s = Used %d Reserved %d, want exactly 1/0",
				state.Name, state.Used, state.Reserved)
		}
	}
}

var _ ports.OrderPlacer = (*mixedHoldPlacer)(nil)

// # The package refusals: every one refuses WHOLE, with nothing sent and
// # nothing journaled — §59c's rule, asserted per case

// refusalAsserts is every package refusal's common postcondition.
func refusalAsserts(t *testing.T, j *fakeJournal, p *fakePlacer) {
	t.Helper()
	if p.calls != 0 {
		t.Errorf("Place called %d times on a refused package, want 0", p.calls)
	}
	if len(j.calls) != 0 {
		t.Errorf("journal recorded %v on a refused package, want nothing", j.states())
	}
}

func TestExecuteRefusesAnInvalidUrgency(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	if _, err := e.Execute(bootedChecklist(t), refsFor(leg), ports.Urgency("routine")); !errors.Is(err, ports.ErrUrgencyInvalid) {
		t.Fatalf("Execute with an undeclared urgency = %v, want ErrUrgencyInvalid", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesAnEmptyPackage(t *testing.T) {
	t.Parallel()

	e, j, p, _, _ := newExecutor(t)
	if _, err := e.Execute(bootedChecklist(t), nil, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageEmpty) {
		t.Fatalf("Execute(nil) = %v, want ErrPackageEmpty", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesAForeignID(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	refs := []application.DispatchRef{{ID: "autoclose-1234567890", Version: 1}}
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrOrderNotOMSMinted) {
		t.Fatalf("Execute with a venue-shaped id = %v, want ErrOrderNotOMSMinted", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesADuplicateLeg(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	if _, err := e.Execute(bootedChecklist(t), refsFor(leg, leg), ports.UrgencyOpening); !errors.Is(err, application.ErrPackageDuplicateLeg) {
		t.Fatalf("Execute with one leg twice = %v, want ErrPackageDuplicateLeg", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesMixedTrades(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	mine := packageOrder(t, 0, "0.5")
	plant(t, r, mine)

	otherTrade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-aaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	otherID, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: otherTrade, Leg: 1})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	other := packageOrder(t, 1, "0.5")
	other.ClientOrderID = otherID
	plant(t, r, other)

	refs := refsFor(mine, other)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageMixedTrades) {
		t.Fatalf("Execute across two trades = %v, want ErrPackageMixedTrades", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesMixedAttempts(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	first := packageOrder(t, 0, "0.5")
	plant(t, r, first)

	replanned, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: testTrade(t), Leg: 1, Attempt: 1})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	second := packageOrder(t, 1, "0.5")
	second.ClientOrderID = replanned
	plant(t, r, second)

	refs := refsFor(first, second)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageMixedAttempts) {
		t.Fatalf("Execute across two attempts = %v, want ErrPackageMixedAttempts", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesAQuantityMismatch is the identical-base-quantity check's
// biting fixture — the direction the equal-quantity tests cannot supply.
//
// BOTH orderings of the mismatch, because a single one collapses the
// comparator: with only a larger second leg, `cmp != 0` and `cmp > 0` are the
// same predicate on every fixture the suite holds, and the one-sided mutant
// survived until the 0.4 case existed (§59g — the collapse checklist's tenth
// instance, caught by the battery rather than before it).
func TestExecuteRefusesAQuantityMismatch(t *testing.T) {
	t.Parallel()

	for _, mismatched := range []string{"0.6", "0.4"} {
		t.Run(mismatched, func(t *testing.T) {
			t.Parallel()

			e, j, p, r, _ := newExecutor(t)
			legSpot := packageOrder(t, 0, "0.5")
			legPerp := packageOrder(t, 1, mismatched)
			plant(t, r, legSpot)
			plant(t, r, legPerp)

			refs := refsFor(legSpot, legPerp)
			if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageQuantityMismatch) {
				t.Fatalf("Execute with 0.5 vs %s = %v, want ErrPackageQuantityMismatch", mismatched, err)
			}
			refusalAsserts(t, j, p)
		})
	}
}

// TestExecuteRefusesAMismatchedBaseAsset: Cmp refuses the comparison itself, so
// "identical" includes "in the same base asset" — a package sized in two bases
// must not compare mantissas that happen to agree.
func TestExecuteRefusesAMismatchedBaseAsset(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	legSpot := packageOrder(t, 0, "0.5")
	plant(t, r, legSpot)

	legPerp := packageOrder(t, 1, "0.5")
	legPerp.Quantity = qtyIn(t, "ETH", "0.5")
	plantAt(t, r, legPerp, domain.StatePlanned, 1)

	refs := refsFor(legSpot, legPerp)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageQuantityMismatch) {
		t.Fatalf("Execute with BTC vs ETH bases = %v, want ErrPackageQuantityMismatch", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesAVersionConflictBeforeAnyLegSends is §59c's central claim.
// The stale leg is deliberately LAST, so the test proves the preflight covers
// every leg before the fan-out — a per-leg check interleaved with dispatch
// would have sent two legs before discovering the third.
func TestExecuteRefusesAVersionConflictBeforeAnyLegSends(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg0 := packageOrder(t, 0, "0.5")
	leg1 := packageOrder(t, 1, "0.5")
	leg2 := packageOrder(t, 2, "0.5")
	plant(t, r, leg0)
	plant(t, r, leg1)
	plantAt(t, r, leg2, domain.StatePlanned, 7)

	refs := refsFor(leg0, leg1, leg2)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Execute with a stale last leg = %v, want ErrVersionConflict", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesANonIOCLeg(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg0 := packageOrder(t, 0, "0.5")
	plant(t, r, leg0)

	// A GTC order planted directly — the reconciler's path, not the planner's,
	// which is exactly who can put a non-IOC order in this repository.
	foreign := packageOrder(t, 1, "0.5")
	foreign.TimeInForce = domain.TimeInForceGTC
	plantAt(t, r, foreign, domain.StatePlanned, 1)

	refs := refsFor(leg0, foreign)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, application.ErrNotMarketableIOCLimit) {
		t.Fatalf("Execute with a GTC leg = %v, want ErrNotMarketableIOCLimit", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesALegInATerminalState: the preflight's CanTransition check
// has to bite at the PACKAGE level — without it, a terminal leg is discovered
// by its own admit only after the other legs have already sent.
func TestExecuteRefusesALegInATerminalState(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg0 := packageOrder(t, 0, "0.5")
	leg1 := packageOrder(t, 1, "0.5")
	plant(t, r, leg0)
	plantAt(t, r, leg1, domain.StateFilled, 1)

	refs := refsFor(leg0, leg1)
	_, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening)
	if err == nil || !errors.Is(err, domain.ErrTransitionForbidden) {
		t.Fatalf("Execute with a Filled leg = %v, want SM §1's refusal", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesAnUnknownLeg(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg0 := packageOrder(t, 0, "0.5")
	plant(t, r, leg0)
	unplanned := packageOrder(t, 1, "0.5")

	refs := refsFor(leg0, unplanned)
	if _, err := e.Execute(bootedChecklist(t), refs, ports.UrgencyOpening); !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("Execute with an unplanned leg = %v, want ErrOrderUnknown", err)
	}
	refusalAsserts(t, j, p)
}

// # After the fan-out: per-leg results, never a composite error

func TestExecuteReportsEachLegForItself(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	// The placer fails EVERY Place with a venue rejection; both legs must
	// carry their own Rejected outcome rather than one error swallowing the
	// package. (A per-leg failure fixture — one leg fails, one succeeds —
	// needs a placer that discriminates, below.)
	p.err = rejectedErr(t)
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	ex, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v, want nil — after the fan-out the report is per leg", err)
	}
	for _, o := range []domain.Order{legSpot, legPerp} {
		res := legByID(t, ex, o.ClientOrderID)
		if res.Err != nil {
			t.Errorf("leg %q Err = %v, want nil (its Place RAN)", o.ClientOrderID, res.Err)
		}
		if res.Outcome.State != domain.StateRejected {
			t.Errorf("leg %q State = %s, want StateRejected", o.ClientOrderID, res.Outcome.State)
		}
	}
	if ex.Dispatched() != 2 {
		t.Errorf("Dispatched() = %d, want 2 — a rejected Place still ran", ex.Dispatched())
	}
	if ex.HoldsNewTrading() {
		t.Error("HoldsNewTrading() = true on rejected legs whose applies succeeded — a rejection is a venue answer, not a stale projection")
	}
}

// TestExecutionHoldsNewTradingWhenAnApplyFails is Outcome.Applied's §6 step 7
// consequence, aggregated: one leg's failed apply holds the package.
func TestExecutionHoldsNewTradingWhenAnApplyFails(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	r.failApplyState[domain.StateSubmitted] = errors.New("projection store down")
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	ex, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if !ex.HoldsNewTrading() {
		t.Error("HoldsNewTrading() = false with a failed apply on a sent leg — §6 step 7's hold lost")
	}
}

// # Routing and the authority: the claims that need three legs and a broken
// # projection

// TestAReExecutedPackageIsRefusedWhateverThePresentationOrder re-presents a
// broken-projection package in a DIFFERENT order and expects every leg refused
// by the process-wide authority — the one mechanism left standing when the
// projection is the thing that broke. The refusal must be a property of the
// ID, not of where a ref sits in the argument list.
//
// Three legs, not two, and rotated rather than reversed: with two, reversing
// the presentation and swapping the results are the same permutation, so a
// position-keyed guard could pass by coincidence (§59f).
func TestAReExecutedPackageIsRefusedWhateverThePresentationOrder(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	// Break the projection: every apply fails, so records stay Planned at
	// version 1 and ONLY the authority remembers the sends.
	applyDown := errors.New("apply path down")
	r.failApplyState[domain.StateDispatchAuthorized] = applyDown
	r.failApplyState[domain.StateSubmitted] = applyDown

	legs := []domain.Order{
		packageOrder(t, 0, "0.5"),
		packageOrder(t, 1, "0.5"),
		packageOrder(t, 2, "0.5"),
	}
	for _, o := range legs {
		plant(t, r, o)
	}

	first, err := e.Execute(bootedChecklist(t), refsFor(legs[0], legs[1], legs[2]), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("first Execute = %v", err)
	}
	if first.Dispatched() != 3 {
		t.Fatalf("first Execute dispatched %d, want 3", first.Dispatched())
	}
	if !first.HoldsNewTrading() {
		t.Fatal("first Execute must hold new trading — every apply failed")
	}

	// Rotated, not reversed: (2,0,1) differs from both the identity and the
	// reversal, so no coincidental permutation can satisfy it.
	//
	// A FRESH checklist, deliberately: the first execution latched its own, and
	// the admission read would refuse this package before the fan-out — which
	// is §6 step 7 working, and it is the test one below. THIS test isolates
	// the layer beneath: the authority must refuse the re-send even for a
	// caller whose checklist never heard about the first execution.
	second, err := e.Execute(bootedChecklist(t), refsFor(legs[2], legs[0], legs[1]), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("second Execute = %v — preflight sees Planned records and must admit the package", err)
	}
	for _, o := range legs {
		res := legByID(t, second, o.ClientOrderID)
		if !errors.Is(res.Err, application.ErrOrderAlreadyDispatched) {
			t.Errorf("re-presented leg %q Err = %v, want ErrOrderAlreadyDispatched", o.ClientOrderID, res.Err)
		}
	}
	if p.calls != 3 {
		t.Errorf("Place called %d times across both executions, want 3 — nothing may re-send", p.calls)
	}
	// A refused leg does NOT hold new trading: nothing new was sent and the
	// projection is not stale about it — the FIRST execution already reported
	// the holds its sends earned. This is HoldsNewTrading's `Err == nil` guard
	// biting: without it, a refused leg's zero Outcome reads as unapplied.
	if second.HoldsNewTrading() {
		t.Error("an all-refused execution holds new trading — a refusal is not a stale projection")
	}
}

// TestTheAuthorityIsSharedAcrossDispatchPaths is §59b's isolating probe: the
// leg is sent through a plain Dispatcher over a BROKEN projection, then
// re-presented to an Executor sharing the same authority. Only the authority
// can refuse it — the repository still says Planned — and the control leg
// proves the probe would sail through on a fresh authority's executor.
func TestTheAuthorityIsSharedAcrossDispatchPaths(t *testing.T) {
	t.Parallel()

	j := newFakeJournal()
	p := &fakePlacer{ack: ports.Ack{VenueOrderID: "V-1"}}
	r := newFakeOrders()
	auth := application.NewDispatchAuthority()

	applyDown := errors.New("apply path down")
	r.failApplyState[domain.StateDispatchAuthorized] = applyDown
	r.failApplyState[domain.StateSubmitted] = applyDown

	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	d, err := application.NewDispatcher(j, p, r, testClock(t), auth, testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if _, err := d.Dispatch(legSpot.ClientOrderID, 1, ports.UrgencyOpening, nil); err != nil {
		t.Fatalf("Dispatch = %v", err)
	}

	e, err := application.NewExecutor(d, newFakeReserver(t))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	ex, err := e.Execute(bootedChecklist(t), refsFor(legSpot, legPerp), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v — the package preflight reads Planned records and admits", err)
	}

	if res := legByID(t, ex, legSpot.ClientOrderID); !errors.Is(res.Err, application.ErrOrderAlreadyDispatched) {
		t.Errorf("cross-path re-presentation Err = %v, want ErrOrderAlreadyDispatched", res.Err)
	}
	if res := legByID(t, ex, legPerp.ClientOrderID); res.Err != nil {
		t.Errorf("fresh leg Err = %v, want nil — the refusal must be per leg", res.Err)
	}
	if p.calls != 2 {
		t.Errorf("Place called %d times, want 2 — the sent leg must not re-send", p.calls)
	}

	// The control: a SEPARATE authority does not know about the send, and the
	// repository cannot refuse (the projection is the thing that broke) — so
	// the double-Place happens. This is the defect §59b closes, kept running
	// as the demonstration that the shared authority is load-bearing above.
	// Expressing it now takes a second DISPATCHER on a fresh authority — the
	// executor constructor alone can no longer state it, which is the point.
	other, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority(), testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	split, err := application.NewExecutor(other, newFakeReserver(t))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	// A fresh checklist again: the earlier execution latched the one it was
	// handed, and the control must reach the authority, not the admission read.
	ctrl, err := split.Execute(bootedChecklist(t), refsFor(legSpot), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("control Execute = %v", err)
	}
	if res := legByID(t, ctrl, legSpot.ClientOrderID); res.Err != nil {
		t.Fatalf("control Err = %v — a fragmented backstop ADMITS the re-send; if this refuses, the control no longer isolates the authority", res.Err)
	}
	if p.calls != 3 {
		t.Errorf("Place called %d times after the control, want 3 — the fragmented backstop re-sent", p.calls)
	}
}

// rejectedErr builds the venue-rejection error shape classifyPlace maps to
// StateRejected, through the apperror constructor the adapters use.
func rejectedErr(t *testing.T) error {
	t.Helper()
	return apperror.VenueRejected("place", "-2010", "insufficient margin", errors.New("boom"))
}

// qtyIn is qty with the base asset a parameter — the mismatched-asset fixture's
// need, which the shared helper deliberately cannot express.
func qtyIn(t *testing.T, asset, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseQuantity(%q, %q): %v", asset, v, err)
	}
	return q
}

// # §6 step 7 at the write path: the fan-in files the hold, the admission read
// # refuses the next opening over it, and the safety verbs stay available

// TestExecuteFilesTheHoldAtTheFanIn: a sent leg whose apply failed latches
// GateNetDeltaAsserted at OutcomeProjectionStale — the live path's producer,
// the one dispatch.go's Outcome doc promises. The checklist carries a SECOND
// gate so "the gate the executor files" and "whatever gate the checklist has"
// are different answers: the other gate must come through untouched.
func TestExecuteFilesTheHoldAtTheFanIn(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)
	// EXACTLY ONE stale leg — the only fixture that separates "any leg's
	// failed apply holds" from "all legs' failed applies hold". The both-stale
	// shape is the second-opening test's, so the suite carries both
	// cardinalities.
	r.failApplyID[legPerp.ClientOrderID] = errors.New("projection store down")

	c := bootedChecklist(t)
	ex, err := e.Execute(c, refsFor(legSpot, legPerp), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if !ex.HoldsNewTrading() {
		t.Fatal("HoldsNewTrading() = false with one sent leg's apply failing")
	}
	if res := legByID(t, ex, legSpot.ClientOrderID); !res.Outcome.Applied {
		t.Fatalf("clean leg Applied = false — the fixture no longer isolates the stale leg")
	}
	if res := legByID(t, ex, legPerp.ClientOrderID); res.Outcome.Applied {
		t.Fatalf("stale leg Applied = true — the fixture no longer contains the thing under test")
	}
	if ex.Hold != nil {
		t.Errorf("Hold = %v, want nil — an unlatched declared gate admits the filing", ex.Hold)
	}
	if o, _ := c.Outcome(boot.GateNetDeltaAsserted); o != boot.OutcomeProjectionStale {
		t.Errorf("net-delta gate = %s, want OutcomeProjectionStale", o)
	}
	if got := c.Latched(); len(got) != 1 || got[0] != boot.GateNetDeltaAsserted {
		t.Errorf("Latched() = %v, want exactly the net-delta gate", got)
	}
	if o, _ := c.Outcome(boot.GateWriterAuthority); o != boot.OutcomeUnreported {
		t.Errorf("writer-authority gate = %s, want Unreported — the filing must not touch its neighbours", o)
	}
}

// TestExecuteFilesNothingWhenEveryApplyLands is the filing test's positive
// control: without it, an executor that latched every checklist it was handed
// would pass the test above.
func TestExecuteFilesNothingWhenEveryApplyLands(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	c := bootedChecklist(t)
	if _, err := e.Execute(c, refsFor(legSpot, legPerp), ports.UrgencyOpening); err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if got := c.Latched(); len(got) != 0 {
		t.Errorf("Latched() = %v after a clean execution, want none", got)
	}
	if o, _ := c.Outcome(boot.GateNetDeltaAsserted); o != boot.OutcomePassed {
		t.Errorf("net-delta gate = %s after a clean execution, want the boot's Passed untouched", o)
	}
}

// TestASecondOpeningInTheSameProcessIsRefusedOverTheHold is #169's claim end
// to end: stale execution, then a NEW opening package on the SAME checklist —
// refused at the admission read, before any fsync, with nothing sent. This is
// the loop closing inside one process, with no composition root remembering
// anything.
func TestASecondOpeningInTheSameProcessIsRefusedOverTheHold(t *testing.T) {
	t.Parallel()

	e, _, p, r, _ := newExecutor(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	r.failApplyState[domain.StateSubmitted] = errors.New("projection store down")
	legs := []domain.Order{
		packageOrder(t, 0, "0.5"),
		packageOrder(t, 1, "0.5"),
		packageOrder(t, 2, "0.5"),
		packageOrder(t, 3, "0.5"),
	}
	for _, o := range legs {
		plant(t, r, o)
	}

	c := bootedChecklist(t)
	first, err := e.Execute(c, refsFor(legs[0], legs[1]), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("first Execute = %v", err)
	}
	if !first.HoldsNewTrading() {
		t.Fatal("first Execute must hold — every apply failed")
	}
	if p.calls != 2 {
		t.Fatalf("Place called %d times after the first package, want 2", p.calls)
	}

	// FRESH legs, so the refusal cannot come from the authority or from any
	// per-order state: the only thing standing between this package and the
	// venue is the latch the previous package filed.
	_, err = e.Execute(c, refsFor(legs[2], legs[3]), ports.UrgencyOpening)
	if !errors.Is(err, application.ErrExecutionHeld) {
		t.Fatalf("second opening Execute = %v, want ErrExecutionHeld", err)
	}
	if p.calls != 2 {
		t.Errorf("Place called %d times after the refused package, want still 2 — a refusal sends nothing", p.calls)
	}

	// The asymmetry §6 step 7 is FOR: the same process, the same latch, and a
	// SAFETY package still executes — stop-opening withholds new exposure, not
	// the safety verbs.
	safety, err := e.Execute(c, refsFor(legs[2], legs[3]), ports.UrgencySafety)
	if err != nil {
		t.Fatalf("safety Execute over the latch = %v, want admitted", err)
	}
	if p.calls != 4 {
		t.Errorf("Place called %d times after the safety package, want 4 — safety execution was withheld", p.calls)
	}
	// Its applies failed too, so the fan-in re-files the same outcome — the
	// honest repeat Report admits — and the latch must be exactly where it was.
	if safety.Hold != nil {
		t.Errorf("safety Hold = %v, want nil — a repeat of the same latching outcome is admitted", safety.Hold)
	}
	if o, _ := c.Outcome(boot.GateNetDeltaAsserted); o != boot.OutcomeProjectionStale {
		t.Errorf("net-delta gate = %s after the safety package, want ProjectionStale still", o)
	}
}

// TestExecuteRefusesANilChecklist: no checklist means nowhere for §6 step 7's
// hold to go, and that is discovered before anything is sent, not at the
// fan-in with live orders behind it.
func TestExecuteRefusesANilChecklist(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	if _, err := e.Execute(nil, refsFor(leg), ports.UrgencyOpening); !errors.Is(err, application.ErrChecklistMissing) {
		t.Fatalf("Execute(nil checklist) = %v, want ErrChecklistMissing", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesAChecklistWithoutTheHoldGate: the isolating fixture
// carries a PASSED verdict on the gate it does declare, so the admission's
// PermitsNow read would admit this package — the refusal can only be the
// declared-check. A fixture that failed both checks would show that something
// objected, not that this rule did.
func TestExecuteRefusesAChecklistWithoutTheHoldGate(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	c, err := boot.NewChecklist(boot.GateWriterAuthority)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	if err := c.Report(boot.GateWriterAuthority, boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}

	if _, err := e.Execute(c, refsFor(leg), ports.UrgencyOpening); !errors.Is(err, application.ErrHoldGateUndeclared) {
		t.Fatalf("Execute over a checklist without the net-delta gate = %v, want ErrHoldGateUndeclared", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesEveryClassOnAVerdictlessChecklist is PermitsNow's
// fail-closed rule reaching the write path: a checklist no check has concluded
// in is a process §7.5's boot has not run in, and BOTH classes are refused —
// safety too, because "no boot" is below even reconcile-only.
func TestExecuteRefusesEveryClassOnAVerdictlessChecklist(t *testing.T) {
	t.Parallel()

	for _, urgency := range []ports.Urgency{ports.UrgencyOpening, ports.UrgencySafety} {
		t.Run(string(urgency), func(t *testing.T) {
			t.Parallel()

			e, j, p, r, _ := newExecutor(t)
			leg := packageOrder(t, 0, "0.5")
			plant(t, r, leg)

			c, err := boot.NewChecklist(boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
			if err != nil {
				t.Fatalf("NewChecklist: %v", err)
			}

			if _, err := e.Execute(c, refsFor(leg), urgency); !errors.Is(err, application.ErrExecutionHeld) {
				t.Fatalf("Execute(%s) over a verdictless checklist = %v, want ErrExecutionHeld", urgency, err)
			}
			refusalAsserts(t, j, p)
		})
	}
}

// TestExecuteRefusesOverAConcludedNonLatchingRefusal: OutcomeFailed does not
// latch — Latched() is EMPTY here — and it halts, so both classes are refused.
// This is the fixture that separates "reads the verdicts" from "reads the
// latches": an admission built on Latched alone admits this package.
func TestExecuteRefusesOverAConcludedNonLatchingRefusal(t *testing.T) {
	t.Parallel()

	for _, urgency := range []ports.Urgency{ports.UrgencyOpening, ports.UrgencySafety} {
		t.Run(string(urgency), func(t *testing.T) {
			t.Parallel()

			e, j, p, r, _ := newExecutor(t)
			leg := packageOrder(t, 0, "0.5")
			plant(t, r, leg)

			c := bootedChecklist(t)
			if err := c.Report(boot.GateWriterAuthority, boot.OutcomeFailed); err != nil {
				t.Fatalf("Report: %v", err)
			}
			if got := c.Latched(); len(got) != 0 {
				t.Fatalf("fixture latched %v — this test needs the non-latching refusal", got)
			}

			if _, err := e.Execute(c, refsFor(leg), urgency); !errors.Is(err, application.ErrExecutionHeld) {
				t.Fatalf("Execute(%s) over a Failed verdict = %v, want ErrExecutionHeld", urgency, err)
			}
			refusalAsserts(t, j, p)
		})
	}
}
