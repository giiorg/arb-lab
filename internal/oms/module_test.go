package oms_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # What this file claims, and what it deliberately leaves to the layer below
//
// application/dispatch_test.go already walks the classification table, the
// journal-before-network interleaving, the three admit refusals and the
// double-dispatch backstop. Repeating any of that through a one-field struct
// would produce a suite that looks twice as thorough and checks the same code
// once, so this file makes only the claim no layer test can: that New connects
// the SUPPLIED ports into the roles they belong in.
//
// # The fixture-collapse pass, run before the assertions were written
//
// Three parameters here are pinned to values that would collapse a distinction
// the wiring makes, and each was moved off it:
//
//  1. The placer's Ack. Left at its zero value, "the venue's answer survives
//     the module" is indistinguishable from "the module returns a zero Ack" —
//     so it carries a VenueOrderID, and the assertion reads it.
//  2. The journal's recorded states. Asserting a COUNT would pass against a
//     module wired to a journal that recorded anything twice; the ordered pair
//     is what says which two records they were.
//  3. The refusal cases' sentinels. One omission at a time and a DIFFERENT
//     sentinel per case — together they catch a New that checks only the first
//     port, and a New that invented one blanket refusal instead of letting the
//     constructor that owns the wire speak.

// recordingJournal is the supplied journal, and the point of every assertion
// against it is that it is THIS instance rather than one New constructed.
type recordingJournal struct {
	states []domain.OrderState
	ids    []ids.ClientOrderID
}

func (j *recordingJournal) AppendPlanned(domain.Order) error { return nil }

func (j *recordingJournal) AppendState(id ids.ClientOrderID, state domain.OrderState) error {
	j.ids = append(j.ids, id)
	j.states = append(j.states, state)
	return nil
}

// countingPlacer is the supplied placer. It counts because §5.3's rule is a
// count — "submit once, NO transport retry" — and it records the order because
// a module that reached the venue with a DIFFERENT order than it was handed
// would satisfy the count.
type countingPlacer struct {
	ack ports.Ack

	calls  int
	orders []domain.Order
}

func (p *countingPlacer) Place(o domain.Order) (ports.Ack, error) {
	p.calls++
	p.orders = append(p.orders, o)
	return p.ack, nil
}

var (
	_ ports.Journal     = (*recordingJournal)(nil)
	_ ports.OrderPlacer = (*countingPlacer)(nil)
)

const venueOrderID = "V-77"

// fullDeps is every dependency populated, which each construction case then
// breaks exactly one of.
func fullDeps() oms.Deps {
	return oms.Deps{
		Journal: &recordingJournal{},
		Placer:  &countingPlacer{ack: ports.Ack{VenueOrderID: venueOrderID}},
	}
}

// dispatchableOrder is §6 step 6's own shape — a marketable-IOC limit — built
// through the real id constructor rather than from a literal, so a format
// change cannot quietly turn this into a test of ErrOrderNotOMSMinted.
func dispatchableOrder(t *testing.T) domain.Order {
	t.Helper()

	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: 1})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	q, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	px, err := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return domain.Order{
		ClientOrderID: id,
		Instrument: ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductLinearPerp,
			Symbol: "BTCUSDT", Contract: ids.ContractLinear,
		},
		Side:        domain.SideSell,
		Type:        domain.OrderTypeLimit,
		TimeInForce: domain.TimeInForceIOC,
		Quantity:    q,
		LimitPrice:  px,
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewRefusesEachMissingPort(t *testing.T) {
	t.Parallel()

	// Each case omits exactly ONE port and expects a DIFFERENT sentinel — the
	// one belonging to the constructor that owns that wire. module.go adds no
	// refusals of its own precisely so that these stay the dispatcher's, and
	// this is the assertion that says so: a New with its own blanket check would
	// answer one sentinel to both cases.
	cases := map[string]struct {
		mutate func(*oms.Deps)
		want   error
	}{
		"no journal": {func(d *oms.Deps) { d.Journal = nil }, application.ErrJournalMissing},
		"no placer":  {func(d *oms.Deps) { d.Placer = nil }, application.ErrPlacerMissing},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			deps := fullDeps()
			tc.mutate(&deps)

			if _, err := oms.New(deps); !errors.Is(err, tc.want) {
				t.Fatalf("New = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestNewAssemblesACompleteContext is the positive control the cases above
// need: without it, a New that refused everything would score two out of two.
func TestNewAssemblesACompleteContext(t *testing.T) {
	t.Parallel()

	m, err := oms.New(fullDeps())
	if err != nil {
		t.Fatalf("New = %v, want nil", err)
	}
	if m.Dispatch == nil {
		t.Error("Module.Dispatch is nil — New returned a context with no crash boundary")
	}
}

// ---------------------------------------------------------------------------
// The wired path: an order in, a record in the supplied journal and one call
// at the supplied placer
// ---------------------------------------------------------------------------

// TestModuleDispatchesThroughTheSuppliedPorts is the only claim this file can
// make that the layer below cannot.
//
// application/dispatch_test.go hands NewDispatcher its fakes directly, so it
// proves the use case uses the ports it was given and says nothing about
// whether New passes Deps through. A New that dropped either field and
// constructed a discarding stand-in would pass every test in that file and
// produce a running OMS that submits orders while journaling nothing — the
// exact state §6 step 6's crash boundary exists to make impossible.
func TestModuleDispatchesThroughTheSuppliedPorts(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	journal := deps.Journal.(*recordingJournal)
	placer := deps.Placer.(*countingPlacer)

	m, err := oms.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	o := dispatchableOrder(t)
	out, err := m.Dispatch.Dispatch(o)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}

	// The supplied placer, called exactly once, with the order it was handed.
	if placer.calls != 1 {
		t.Errorf("the supplied placer saw %d calls, want exactly 1", placer.calls)
	}
	if len(placer.orders) == 1 && placer.orders[0] != o {
		t.Errorf("the supplied placer received %+v, want %+v", placer.orders[0], o)
	}

	// The supplied journal, holding both records in order and under the order's
	// own id. The pair rather than the count: the first record is the crash
	// boundary and the second is the result, and a module wired to a journal
	// that wrote the same state twice would satisfy a count.
	want := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted}
	if len(journal.states) != len(want) {
		t.Fatalf("the supplied journal recorded %v, want %v", journal.states, want)
	}
	for i, state := range want {
		if journal.states[i] != state {
			t.Errorf("journal record %d = %s, want %s", i, journal.states[i], state)
		}
		if journal.ids[i] != o.ClientOrderID {
			t.Errorf("journal record %d carries id %q, want %q", i, journal.ids[i], o.ClientOrderID)
		}
	}

	// The venue's answer reaches the caller through the module. Non-zero on
	// purpose: against a zero Ack this assertion holds for a module that
	// discarded it.
	if out.Ack.VenueOrderID != venueOrderID {
		t.Errorf("Ack.VenueOrderID = %q, want %q — the venue's answer did not survive the composition",
			out.Ack.VenueOrderID, venueOrderID)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if !out.Journaled {
		t.Error("Journaled = false — the result never reached the supplied journal")
	}
}
