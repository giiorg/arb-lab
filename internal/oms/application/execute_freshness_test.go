package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §69's freshness half: the executor re-judges each leg's filters row age at
// the package's ONE admission instant, against the deployment's declared
// bound. The boot-time check (app/exercise/filters.go) proves rows were fresh
// when the process started; these tests prove the claim that outlives it.

// executorOn is newExecutor with the three §69 knobs exposed: the clock the
// admission instant is read from, the filters map the ages live in, and the
// bound they are judged against.
func executorOn(t *testing.T, clk clock.Clock, filters map[ids.VenueInstrument]domain.TradingFilters,
	maxAge time.Duration) (*application.Executor, *fakeJournal, *fakePlacer, *fakeOrders) {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j, ack: ports.Ack{VenueOrderID: "V-1"}}
	r := newFakeOrders()
	d, err := application.NewDispatcher(j, p, r, clk, application.NewDispatchAuthority())
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	e, err := application.NewExecutor(d, filters, maxAge, newFakeReserver(t))
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	return e, j, p, r
}

// filtersObservedAt is testFilters with both instruments' observation
// instants pinned to one value — the boundary batteries move that one value
// against the bound.
func filtersObservedAt(t *testing.T, at time.Time) map[ids.VenueInstrument]domain.TradingFilters {
	t.Helper()
	filters := testFilters(t)
	for inst, f := range filters {
		f.ObservedAt = at
		filters[inst] = f
	}
	return filters
}

// TestExecuteFreshnessBoundary pins the comparison's direction and both sides
// of its boundary. The bound is INCLUSIVE — a row aged exactly the tolerance
// still admits — so `age == bound` is the value where `>` and `>=` part ways,
// and it is tested by name rather than left between two cases that agree.
func TestExecuteFreshnessBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		observed time.Time
		want     error // nil = the package must admit and send
	}{
		{"one second inside the bound", testNow.Add(-testFilterMaxAge + time.Second), nil},
		{"exactly at the bound", testNow.Add(-testFilterMaxAge), nil},
		{"one second past the bound", testNow.Add(-testFilterMaxAge - time.Second), application.ErrFiltersStaleAtAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e, j, p, r := executorOn(t, testClock(t), filtersObservedAt(t, tc.observed), testFilterMaxAge)
			leg0 := packageOrder(t, 0, "0.5")
			leg1 := packageOrder(t, 1, "0.5")
			plant(t, r, leg0)
			plant(t, r, leg1)

			ex, err := e.Execute(bootedChecklist(t), refsFor(leg0, leg1), ports.UrgencyOpening)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("Execute = %v, want %v", err, tc.want)
				}
				refusalAsserts(t, j, p)
				return
			}
			if err != nil {
				t.Fatalf("Execute = %v, want an admitted package", err)
			}
			for _, leg := range ex.Legs {
				if leg.Err != nil {
					t.Fatalf("leg %s = %v, want sent", leg.ID, leg.Err)
				}
			}
		})
	}
}

// TestExecuteRefusesAFutureFiltersRow: a negative age would pass the
// staleness comparison trivially, and it has a real producer — a backward
// wall-clock step after boot, met by a map resolved before the step. Its own
// sentinel, because the operator's fix differs from staleness (investigate
// the clock, not fetch a revision).
func TestExecuteRefusesAFutureFiltersRow(t *testing.T) {
	t.Parallel()

	e, j, p, r := executorOn(t, testClock(t),
		filtersObservedAt(t, testNow.Add(time.Minute)), testFilterMaxAge)
	leg0 := packageOrder(t, 0, "0.5")
	leg1 := packageOrder(t, 1, "0.5")
	plant(t, r, leg0)
	plant(t, r, leg1)

	if _, err := e.Execute(bootedChecklist(t), refsFor(leg0, leg1), ports.UrgencyOpening); !errors.Is(err, application.ErrFiltersFromTheFuture) {
		t.Fatalf("Execute with a future row = %v, want ErrFiltersFromTheFuture", err)
	}
	refusalAsserts(t, j, p)
}

// TestExecuteRefusesThePackageWholeOnOneStaleLeg: the stale row is the PERP's
// — deliberately not leg 0's — so the refusal proves the check covers every
// leg before anything is sent, package-atomically. One leg refused after its
// sibling sent would be the §62d asymmetry this check exists to prevent.
func TestExecuteRefusesThePackageWholeOnOneStaleLeg(t *testing.T) {
	t.Parallel()

	filters := testFilters(t)
	stale := filters[perp()]
	stale.ObservedAt = testNow.Add(-testFilterMaxAge - time.Hour)
	filters[perp()] = stale

	e, j, p, r := executorOn(t, testClock(t), filters, testFilterMaxAge)
	leg0 := packageOrder(t, 0, "0.5") // spot: fresh
	leg1 := packageOrder(t, 1, "0.5") // perp: stale
	plant(t, r, leg0)
	plant(t, r, leg1)

	if _, err := e.Execute(bootedChecklist(t), refsFor(leg0, leg1), ports.UrgencyOpening); !errors.Is(err, application.ErrFiltersStaleAtAdmission) {
		t.Fatalf("Execute with a stale perp row = %v, want ErrFiltersStaleAtAdmission", err)
	}
	refusalAsserts(t, j, p)
}

// steppingNowClock advances itself on every Now() read, so the number of
// reads is observable through the values they return — the fixture the
// one-instant-per-package claim needs, because on a fixed clock a per-leg
// Now() and one Now() before the loop are observationally identical.
type steppingNowClock struct {
	*clock.TestClock
	step time.Duration
}

func (s *steppingNowClock) Now() time.Time {
	now := s.TestClock.Now()
	if err := s.TestClock.Advance(s.step); err != nil {
		panic(err)
	}
	return now
}

// TestExecuteReadsOneAdmissionInstantPerPackage: two equally-dated legs, and
// a clock whose FIRST read sits inside the bound while its SECOND would sit
// outside it. One admission instant admits the package; a per-leg Now() would
// refuse leg 2 — asymmetrically, after leg 1 already passed preflight — so
// this fixture is what separates the two implementations the fixed-clock
// battery above cannot tell apart.
func TestExecuteReadsOneAdmissionInstantPerPackage(t *testing.T) {
	t.Parallel()

	// First Now() = testNow: age is one second inside the bound. The clock
	// then steps 2 minutes per read, so any second read judges the same rows
	// two minutes PAST the bound.
	clk := &steppingNowClock{TestClock: testClock(t), step: 2 * time.Minute}
	filters := filtersObservedAt(t, testNow.Add(-testFilterMaxAge+time.Second))

	e, j, _, r := executorOn(t, clk, filters, testFilterMaxAge)
	leg0 := packageOrder(t, 0, "0.5")
	leg1 := packageOrder(t, 1, "0.5")
	plant(t, r, leg0)
	plant(t, r, leg1)

	ex, err := e.Execute(bootedChecklist(t), refsFor(leg0, leg1), ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute = %v, want one admission instant to admit the package", err)
	}
	for _, leg := range ex.Legs {
		if leg.Err != nil {
			t.Fatalf("leg %s = %v, want sent", leg.ID, leg.Err)
		}
	}
	if got := len(j.planned); got != 0 {
		t.Fatalf("journal planned %d, fixture corrupted", got)
	}
}

// TestNewExecutorRefusesAMismatchedFilterPolicy: the map and the bound are
// one policy, and half of it is a wiring error named at construction — not a
// first order refusing for a reason assembled two fields apart.
func TestNewExecutorRefusesAMismatchedFilterPolicy(t *testing.T) {
	t.Parallel()

	newDispatcher := func(t *testing.T) *application.Dispatcher {
		t.Helper()
		d, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}, newFakeOrders(),
			testClock(t), application.NewDispatchAuthority())
		if err != nil {
			t.Fatalf("NewDispatcher: %v", err)
		}
		return d
	}

	if _, err := application.NewExecutor(newDispatcher(t), testFilters(t), 0, newFakeReserver(t)); !errors.Is(err, application.ErrFilterMaxAgeInvalid) {
		t.Errorf("NewExecutor(filters, no bound) = %v, want ErrFilterMaxAgeInvalid", err)
	}
	if _, err := application.NewExecutor(newDispatcher(t), testFilters(t), -time.Hour, newFakeReserver(t)); !errors.Is(err, application.ErrFilterMaxAgeInvalid) {
		t.Errorf("NewExecutor(filters, negative bound) = %v, want ErrFilterMaxAgeInvalid", err)
	}
	if _, err := application.NewExecutor(newDispatcher(t), nil, testFilterMaxAge, newFakeReserver(t)); !errors.Is(err, application.ErrFilterMaxAgeInvalid) {
		t.Errorf("NewExecutor(no filters, bound) = %v, want ErrFilterMaxAgeInvalid", err)
	}
	if _, err := application.NewExecutor(newDispatcher(t), nil, 0, newFakeReserver(t)); err != nil {
		t.Errorf("NewExecutor(no filters, no bound) = %v, want nil — the reconcile-only shape", err)
	}
}
