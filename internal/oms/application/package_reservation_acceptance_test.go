package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// acceptanceRefusalPlacer is the venue-controlled interleaving the acceptance
// claim needs. Reservation has completed when Place receives the hold; the
// venue then changes its word before consume, exactly the race package
// reservation cannot eliminate. The returned classification is the transport's
// shipped refusal shape, while Dispatcher, Executor, journal, and Resolver stay
// the real application components under test.
type acceptanceRefusalPlacer struct {
	governor *ratelimit.Governor
	mode     string
}

func (p *acceptanceRefusalPlacer) Place(o domain.Order, u ports.Urgency, hold *ratelimit.Hold) (ports.Ack, error) {
	switch p.mode {
	case "pause":
		if err := p.governor.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyThrottle}); err != nil {
			return ports.Ack{}, err
		}
	case "capacity":
		if err := p.governor.Observe("", "ip_5s", 100); err != nil {
			return ports.Ack{}, err
		}
	default:
		return ports.Ack{}, errors.New("acceptance fixture: unknown refusal mode")
	}
	priority := ratelimit.PriorityOpening
	if u == ports.UrgencySafety {
		priority = ratelimit.PrioritySafety
	}
	decision, err := hold.Consume(ratelimit.Request{
		Family: o.Instrument.Product, Priority: priority, Cost: ratelimit.RequestOnly(),
	})
	if err != nil {
		return ports.Ack{}, apperror.Precondition("transport.Do", "reservation", err)
	}
	if decision.Admitted {
		return ports.Ack{}, errors.New("acceptance fixture: venue change did not refuse consume")
	}
	return ports.Ack{}, apperror.RateLimited("transport.Do", "", 0, decision.Err())
}

// TestGovernorConsumeRefusalsRemainResolvable is the task-owed acceptance
// boundary. Both venue-initiated refusal shapes survive as a durable
// SubmissionUnknown—not a local rejection—then the real Resolver consumes that
// artifact and proves the correct terminal. The execution report is checked as
// the future package consumer will receive it: one leg with a completed outcome,
// not a package-level error that could erase its fate.
func TestGovernorConsumeRefusalsRemainResolvable(t *testing.T) {
	for _, mode := range []string{"pause", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			clk := testClock(t)
			journal := newFakeJournal()
			orders := newFakeOrders()
			reserver := newFakeReserver(t)
			placer := &acceptanceRefusalPlacer{governor: reserver.governor, mode: mode}
			dispatcher, err := application.NewDispatcher(
				journal, placer, orders, clk, application.NewDispatchAuthority(),
			)
			if err != nil {
				t.Fatalf("NewDispatcher: %v", err)
			}
			executor, err := application.NewExecutor(dispatcher, testFilters(t), testFilterMaxAge, reserver)
			if err != nil {
				t.Fatalf("NewExecutor: %v", err)
			}
			order := packageOrder(t, 0, "0.5")
			plant(t, orders, order)

			execution, err := executor.Execute(
				bootedChecklist(t), refsFor(order), ports.UrgencyOpening,
			)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(execution.Legs) != 1 {
				t.Fatalf("Execution.Legs = %d, want 1", len(execution.Legs))
			}
			leg := execution.Legs[0]
			if leg.Err != nil {
				t.Fatalf("leg error = %v, want classified outcome", leg.Err)
			}
			if leg.Outcome.State != domain.StateSubmissionUnknown ||
				!leg.Outcome.Journaled || !leg.Outcome.Applied {
				t.Fatalf("leg outcome = %+v, want durable/applied SubmissionUnknown", leg.Outcome)
			}
			record, err := orders.Load(order.ClientOrderID)
			if err != nil {
				t.Fatalf("Load after Execute: %v", err)
			}
			if record.State != domain.StateSubmissionUnknown {
				t.Fatalf("repository state = %s, want StateSubmissionUnknown", record.State)
			}
			if got := journal.states(); len(got) < 2 || got[len(got)-1] != domain.StateSubmissionUnknown {
				t.Fatalf("journal states = %v, want trailing StateSubmissionUnknown", got)
			}
			if reserved := reservedTotal(reserver.governor); reserved != 0 {
				t.Errorf("residual reservation = %d, want 0", reserved)
			}
			if mode == "capacity" {
				for _, state := range reserver.governor.Snapshot().Limits {
					if state.Name == "ip_5s" && state.Used != 100 {
						t.Errorf("capacity fold Used = %d, want exactly 100", state.Used)
					}
				}
			}

			horizon := record.AuthorizedAt.Latest().Add(testMaxLanding + testSweepMargin)
			clk.Advance(horizon.Sub(clk.Now()) + time.Second)
			venue := &fakeResolver{
				status: absentVenue(publishedReach()),
				sweeps: []ports.ExecutionSweep{{ProvenThrough: horizon}},
			}
			resolver, err := application.NewResolver(
				journal, orders, venue, orders, clk, testPolicy(),
			)
			if err != nil {
				t.Fatalf("NewResolver: %v", err)
			}
			resolution, err := resolver.Resolve(order.ClientOrderID)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if resolution.State != domain.StateAbsentAfterFinality ||
				!resolution.Journaled || !resolution.Applied {
				t.Fatalf("resolution = %+v, want durable/applied StateAbsentAfterFinality", resolution)
			}
			if record, err := orders.Load(order.ClientOrderID); err != nil || record.State != domain.StateAbsentAfterFinality {
				t.Fatalf("terminal repository record = %+v, %v", record, err)
			}
		})
	}
}

var _ ports.OrderPlacer = (*acceptanceRefusalPlacer)(nil)
