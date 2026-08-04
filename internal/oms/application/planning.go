package application

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// LegIntent is one leg of a trade as the caller decided it — §6 step 4's sizing
// output, before §6 step 5 has given it an identity.
//
// # It carries no ClientOrderID, and that is the type's whole reason for
// # existing separately from domain.Order
//
// §5.3 makes the id "deterministic, OMS-minted". A caller that could supply one
// could supply a duplicate, a venue-shaped one, or one from another trade — and
// every such id is well-formed, so nothing downstream would object until
// recovery tried to correlate it. Minting is therefore not a service this type
// offers a caller; it is the one thing Plan does that a caller cannot express.
//
// # It carries no Type and no TimeInForce either
//
// §6 step 6 fixes the execution pattern this system may send: "parallel
// marketable-IOC limit orders (never raw MARKET)". So there is exactly one
// combination a planned order can have, and a parameter would invite a caller to
// name another — ports.Journal.AppendPlanned's argument about StatePlanned,
// applied to the two fields that decide what reaches the venue.
//
// domain.Order still declares OrderTypeMarket and the other three TIFs, and that
// is not a contradiction: the domain must hold orders this system did NOT send
// (§6 step 10's foreign orders), while this type holds only orders it is about
// to. Dispatch re-checks the pair anyway, because the repository legitimately
// holds foreign orders and Dispatch is the boundary where an order stops being
// something we read.
type LegIntent struct {
	// Instrument is which venue, product and symbol the leg trades on. A
	// cash-and-carry package's two legs differ here and nowhere else structural.
	Instrument ids.VenueInstrument

	// Side is the direction.
	Side domain.Side

	// Quantity is the ordered size, strictly positive. §6 step 4's sizing is
	// responsible for having quantized it to the lot-size intersection — and
	// Plan VERIFIES that claim against the instrument's TradingFilters rather
	// than trusting it, because the failure mode of an unverified claim here
	// is not an error message: each leg is judged by its own venue against
	// its own grid, so an off-grid package is one leg filled and one leg
	// refused, which is naked exposure.
	Quantity fixedpoint.Quantity

	// LimitPrice is the marketable limit §6 step 6 requires. It is not optional
	// here the way it is on domain.Order: that type admits market orders because
	// it must represent foreign ones, and this type admits none.
	LimitPrice fixedpoint.Price

	// ReduceOnly is §5.3's flag, and it stays the caller's decision rather than
	// something Plan derives.
	//
	// The spec's rule is that it is set "only when the order reduces an existing
	// position on that instrument — never merely because it reduces package
	// delta". That is a claim about a position no layer here holds, so a Plan
	// that tried to derive it would be guessing; a wrong true produces a venue
	// rejection on a path that only runs when something has already gone wrong,
	// and a wrong false opens exposure while flattening it.
	ReduceOnly bool
}

// TradePlan is §6 step 5's input: one trade, its legs, and the attempt that
// distinguishes this planning from a previous one.
//
// # Leg index is the POSITION in Legs, not a field
//
// ids.ClientOrderRef.Leg indexes the order within the trade, and it could have
// been a field on LegIntent. It is not, because two legs carrying the same index
// mint the SAME id — and the first thing that notices is the duplicate check,
// which reports a plan that collides with itself as though it collided with an
// earlier one. An index derived from position cannot collide.
//
// # Attempt belongs to the PLAN rather than to a leg, and the direction is
// # deliberate
//
// A per-leg attempt is expressible and is the wrong default. §5.3 requires that
// a ClientOrderID is "never reused within the recoverable window", so the unsafe
// mistake is forgetting to advance one — and with a field per leg, a replan that
// bumps three of four legs reuses the fourth's id while the original may still
// be live at the venue. With one field, forgetting is impossible for a subset:
// either the whole plan is fresh or none of it is.
//
// The cost is that a leg needing no replan gets a new id anyway, and that cost
// is nothing: §5.3's danger is REUSE, not profusion.
type TradePlan struct {
	// Trade is the trade all the legs belong to. It is what makes the minted ids
	// correlate to one another after a crash, since ClientOrderID is reversible
	// and carries it (ids.ClientOrderID's doc: an operator reading a venue's
	// order page recovers the trade without database access).
	Trade ids.TradeID

	// Attempt distinguishes this planning of the trade from a previous one. It
	// increments on a REPLAN, never on a retry — §5.3 forbids transport retries
	// outright, so a second id for a leg means a new decision to submit.
	Attempt int

	// Legs are the orders to plan, in the order their leg indices are assigned.
	Legs []LegIntent
}

// Planner is build-spec §6 step 5: "Mint deterministic ClientOrderIDs; persist
// and fsync TradePlanned + intents to the write-ahead journal before any
// submission."
//
// # What this use case is FOR, in one sentence
//
// It is the only way an order enters the system, so that "journaled before
// submitted" is a property of the system's shape rather than of a caller's
// discipline. Dispatch loads what it sends from the repository this fills, so an
// order that skipped here cannot be dispatched at all.
//
// # The order of the three passes is the safety argument
//
// Mint-and-check every leg, then journal every leg, then apply every leg.
// Interleaving them is the smaller code and it loses two properties:
//
//   - Nothing is journaled until every leg is known to be plannable. The journal
//     is append-only, so a leg refused after its predecessors were written leaves
//     records that describe a plan that never existed — and, in the duplicate
//     case, a SECOND intent under a ClientOrderID that already had one. See
//     ErrLegAlreadyPlanned.
//   - Nothing is applied until every leg is durable. §6's write path is
//     journal-then-apply, and phase-wise rather than per-leg makes the stronger
//     reading hold: a journal failure on the last leg leaves the current-state
//     store completely untouched, so there is no partially-visible plan for
//     anything to read.
//
// # The record §6 step 5 names and this does not write
//
// [SPEC EDIT LANDED — v0.9.30] §6 step 5 said "TradePlanned + intents". This
// writes the intents — one ports.Journal.AppendPlanned per leg — and no
// trade-level TradePlanned record. Two reasons, and the second is the
// load-bearing one.
//
// §5.1's aggregate is Trade -> Orders[] -> Fills[] -> PositionAllocation and two
// of those four have no domain type yet; a TradePlanned record would mean
// inventing domain.Trade here, in the slice that owns neither.
//
// What makes the omission SAFE rather than merely cheap is a different clause of
// the same sentence: "before any submission". Because every leg is journaled
// before Plan returns anything dispatchable, the only state a partially-journaled
// plan can be in is one where nothing was sent — and §5.3 says exactly what that
// is worth: "a Planned-only order was never dispatched (no venue order can exist)
// -> safe to replan or discard". So the question a TradePlanned record would
// answer — was this trade's leg set complete? — has no consequence attached to
// it, for every state this design can reach.
//
// The change that breaks that argument is worth naming precisely, because it is
// the one a later slice might make without noticing: a planner that dispatches
// leg 0 before journaling leg 1. Then a crash between them leaves a FILLED leg
// whose package looks single-legged, and nothing in the journal says otherwise.
// The property is asserted rather than described — see the test that a journal
// failure on the last leg leaves the repository empty.
type Planner struct {
	journal ports.Journal
	orders  ports.OrderWriteRepository
	filters map[ids.VenueInstrument]domain.TradingFilters
}

// NewPlanner constructs the use case, refusing a missing port.
//
// Refused at construction on NewDispatcher's argument: the call that would
// discover a nil journal is the one recording an intent that is about to be
// submitted, and finding the write-ahead journal absent there means §6 step 5's
// guarantee silently did not hold for an order already being planned.
//
// # filters is a map rather than a port, and nil is legal — and plans nothing
//
// The filters are venue reference facts resolved by the composition root from
// the dated reference revision (§2.4's provenance rule), so what the use case
// needs is the resolved values, not a live lookup it would re-run per leg. A
// nil or empty map is a deployment that plans nothing: every leg is refused
// with ErrFiltersUnknown naming its instrument. That is the direction that is
// right when somebody forgets — the reconcile-only root wires no filters and
// never plans, and a future root that forgets the map gets a named refusal on
// its first plan rather than an unverified order on its way to a venue.
//
// Each ENTRY is validated here rather than at first use, because the call
// that would discover a zero step is the one refusing a real order — and the
// message there would name the order, not the wiring that broke.
func NewPlanner(j ports.Journal, orders ports.OrderWriteRepository,
	filters map[ids.VenueInstrument]domain.TradingFilters) (*Planner, error) {
	if j == nil {
		return nil, ErrJournalMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if err := validateFilters(filters); err != nil {
		return nil, err
	}
	return &Planner{journal: j, orders: orders, filters: filters}, nil
}

// Plan mints, journals and applies every leg of a trade.
//
// # err != nil means NOTHING IS DISPATCHABLE
//
// The contract mirrors Dispatch's, and for the mirror reason. On an error no
// caller holds a record, so no id can reach a venue — whatever was written to
// the journal describes orders in StatePlanned, which §5.3 makes safe to replan
// or discard. On success every returned record is at StatePlanned and Version 1,
// journaled, and ready for Dispatch.
//
// # Not safe for concurrent use
//
// Deliberately unsynchronised, on Dispatcher's and Ledger's precedent. The
// duplicate pre-check is a read followed by a write, so two concurrent Plans of
// the same trade and attempt can both pass it — and what refuses the second is
// ApplyPlanned, through domain.Ledger.Register. That is the mechanism; this
// check is what keeps the ordinary case from reaching the journal, and its being
// a backstop in the concurrent case is why the ledger's refusal is not removed.
func (p *Planner) Plan(plan TradePlan) ([]ports.OrderRecord, error) {
	orders, err := p.mint(plan)
	if err != nil {
		return nil, err
	}

	// §6 step 5's fsync, once per leg, before ANY of them is applied and before
	// the caller holds a record it could dispatch. ports.Journal returns only
	// when the record is durable, so there is no Sync to pair with this.
	for i, o := range orders {
		if err := p.journal.AppendPlanned(o); err != nil {
			return nil, fmt.Errorf("plan trade %s leg %d: journal intent %q: %w",
				plan.Trade, i, o.ClientOrderID, err)
		}
	}

	records := make([]ports.OrderRecord, 0, len(orders))
	for i, o := range orders {
		rec, err := p.orders.ApplyPlanned(o)
		if err != nil {
			return nil, fmt.Errorf("plan trade %s leg %d: apply intent %q: %w",
				plan.Trade, i, o.ClientOrderID, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// mint builds every leg's order and establishes that the whole plan can be
// written before any of it is.
//
// The duplicate check reads through ports.ErrOrderUnknown rather than treating
// any Load error as "not there", and the distinction is the difference between a
// refusal and a silent overwrite: a repository that failed for some other reason
// answers "unknown" under the loose reading, and the plan proceeds to journal an
// intent for an id that may already have one.
func (p *Planner) mint(plan TradePlan) ([]domain.Order, error) {
	if len(plan.Legs) == 0 {
		return nil, fmt.Errorf("plan trade %s: %w", plan.Trade, ErrPlanEmpty)
	}

	orders := make([]domain.Order, 0, len(plan.Legs))
	for i, leg := range plan.Legs {
		id, err := ids.NewClientOrderID(ids.ClientOrderRef{
			Trade: plan.Trade, Leg: i, Attempt: plan.Attempt,
		})
		if err != nil {
			return nil, fmt.Errorf("plan trade %s leg %d: %w", plan.Trade, i, err)
		}

		o := domain.Order{
			ClientOrderID: id,
			Instrument:    leg.Instrument,
			Side:          leg.Side,
			Type:          domain.OrderTypeLimit,
			TimeInForce:   domain.TimeInForceIOC,
			Quantity:      leg.Quantity,
			LimitPrice:    leg.LimitPrice,
			ReduceOnly:    leg.ReduceOnly,
		}
		if err := o.Validate(); err != nil {
			return nil, fmt.Errorf("plan trade %s leg %d: %w", plan.Trade, i, err)
		}
		if err := conformsToFilters(p.filters, leg.Instrument, leg.Quantity, leg.LimitPrice); err != nil {
			return nil, fmt.Errorf("plan trade %s leg %d: %w", plan.Trade, i, err)
		}

		switch _, err := p.orders.Load(id); {
		case err == nil:
			return nil, fmt.Errorf("plan trade %s leg %d: order %q: %w",
				plan.Trade, i, id, ErrLegAlreadyPlanned)
		case errors.Is(err, ports.ErrOrderUnknown):
			// The one answer that means this leg is plannable.
		default:
			return nil, fmt.Errorf("plan trade %s leg %d: check order %q: %w",
				plan.Trade, i, id, err)
		}

		orders = append(orders, o)
	}
	return orders, nil
}
