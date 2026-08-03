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
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
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
	d, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority())
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(d)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	return e, j, p, r, d
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
	if _, err := application.NewExecutor(nil); !errors.Is(err, application.ErrDispatcherMissing) {
		t.Errorf("NewExecutor(nil) = %v, want ErrDispatcherMissing", err)
	}

	// The positive control, without which a constructor refusing everything
	// scores above.
	d, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, newFakeOrders(),
		testClock(t), application.NewDispatchAuthority())
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if _, err := application.NewExecutor(d); err != nil {
		t.Errorf("NewExecutor(dispatcher) = %v, want nil", err)
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

	ex, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencyOpening)
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
	// Both legs' full record pairs are in the journal.
	if len(j.calls) != 4 {
		t.Errorf("journal holds %d records, want 4 (authorized+state per leg): %v", len(j.calls), j.states())
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
	gated, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority())
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(gated)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	legSpot := packageOrder(t, 0, "0.5")
	legPerp := packageOrder(t, 1, "0.5")
	plant(t, r, legSpot)
	plant(t, r, legPerp)

	done := make(chan application.Execution, 1)
	go func() {
		ex, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencyOpening)
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

func (p *gatePlacer) Place(o domain.Order, _ ports.Urgency) (ports.Ack, error) {
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

	if _, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencySafety); err != nil {
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

	if _, err := e.Execute(refsFor(leg), ports.Urgency("routine")); !errors.Is(err, ports.ErrUrgencyInvalid) {
		t.Fatalf("Execute with an undeclared urgency = %v, want ErrUrgencyInvalid", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesAnEmptyPackage(t *testing.T) {
	t.Parallel()

	e, j, p, _, _ := newExecutor(t)
	if _, err := e.Execute(nil, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageEmpty) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrOrderNotOMSMinted) {
		t.Fatalf("Execute with a venue-shaped id = %v, want ErrOrderNotOMSMinted", err)
	}
	refusalAsserts(t, j, p)
}

func TestExecuteRefusesADuplicateLeg(t *testing.T) {
	t.Parallel()

	e, j, p, r, _ := newExecutor(t)
	leg := packageOrder(t, 0, "0.5")
	plant(t, r, leg)

	if _, err := e.Execute(refsFor(leg, leg), ports.UrgencyOpening); !errors.Is(err, application.ErrPackageDuplicateLeg) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageMixedTrades) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageMixedAttempts) {
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
			if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageQuantityMismatch) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrPackageQuantityMismatch) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, ports.ErrVersionConflict) {
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, application.ErrNotMarketableIOCLimit) {
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
	_, err := e.Execute(refs, ports.UrgencyOpening)
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
	if _, err := e.Execute(refs, ports.UrgencyOpening); !errors.Is(err, ports.ErrOrderUnknown) {
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

	ex, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencyOpening)
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

	ex, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencyOpening)
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

	first, err := e.Execute(refsFor(legs[0], legs[1], legs[2]), ports.UrgencyOpening)
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
	second, err := e.Execute(refsFor(legs[2], legs[0], legs[1]), ports.UrgencyOpening)
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

	d, err := application.NewDispatcher(j, p, r, testClock(t), auth)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if _, err := d.Dispatch(legSpot.ClientOrderID, 1, ports.UrgencyOpening); err != nil {
		t.Fatalf("Dispatch = %v", err)
	}

	e, err := application.NewExecutor(d)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	ex, err := e.Execute(refsFor(legSpot, legPerp), ports.UrgencyOpening)
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
	other, err := application.NewDispatcher(j, p, r, testClock(t), application.NewDispatchAuthority())
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	split, err := application.NewExecutor(other)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	ctrl, err := split.Execute(refsFor(legSpot), ports.UrgencyOpening)
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
