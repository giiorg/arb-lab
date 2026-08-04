package application

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Joiner is the second half of build-spec §7.5's reconcile-only boot: the
// replayed journal "joined to venue truth", and the net-delta invariant asserted
// before trading resumes.
//
// # What the replay leaves it, and why the gap is the dangerous kind
//
// Replayer rebuilds STATE and cannot rebuild quantity: no fill is journaled, so
// every replayed record carries a zero cumulative. That is not a missing number
// with an obvious shape — it is a number whose absence reads as a conclusion.
// §6.7's classifier folds those cumulatives into a residual, and a group of
// zeroes is Determined and flat, which is the verdict that PERMITS a compensating
// trade. A process that booted, replayed, and started classifying would size its
// reductions against a position it cannot see.
//
// So this use case exists to turn the venue's own record into the projection's,
// and the boot checklist is what holds the interval: GateNetDeltaAsserted is a
// separate gate from GateJournalReplayed, the posture is the weakest over the
// required set, and nothing may trade until this has run.
//
// # It joins THREE venue surfaces, and the third is what makes the join a check
//
// §7.5 names OrderByClientID, Fills, Positions and FundingLedger. The first two
// answer "what did OUR orders do" and the third answers "what does the ACCOUNT
// hold", and only the pair of them can disagree. An assertion built from our own
// orders alone would be the shape this repo has paid for twice — a checker
// recomputing the thing it checks — and it would agree with itself over an
// account holding a position no order of ours explains, which is the exact
// condition §7.5 alerts on.
//
// FundingLedger is deliberately not here. It settles in the quote asset on both
// venues' linear perps, so it moves no position and can explain no delta, and a
// port whose consumer does not exist yet is a shape frozen against nothing —
// ports.OrderResolver's own argument. It lands with §8's funding accrual.
//
// # Not safe for concurrent use
//
// Dispatcher's and Resolver's argument, unchanged.
type Joiner struct {
	venue     ports.OrderResolver
	positions ports.VenuePositions
	orders    ports.OrderWriteRepository
	clk       clock.Clock
	account   string
	scopes    []ports.PositionScope
}

// NewJoiner constructs the use case, refusing a missing port.
//
// # account is §5.1's first dedupe-key component, and this is its producer
//
// domain.Fill's Account had a consumer and no producer at all until here: §5.1
// puts it first in the fill dedupe key and first in the hedge-group key, and
// nothing in the system minted one. A venue execution does not carry it — the
// venue knows which subaccount it answered for and does not repeat it per row —
// so it is deployment configuration, exactly as §5.2 says (`subaccount id`), and
// this is the layer that knows it.
//
// Empty is refused rather than defaulted, on the write repository's argument for
// the same string: an empty account merges every subaccount into one dedupe
// identity, so two subaccounts sharing a venue's per-instrument execution-ID
// sequence would silently drop each other's fills as duplicates.
//
// # scopes are the position enumerations to assert over, and they are the
// # deployment's rather than derived
//
// A scope names a venue, a derivative family and a settlement asset. It is not
// derived from the instruments this system trades, because the derivation would
// be exactly backwards: the enumeration exists to find exposure on instruments
// NOTHING here knows about, and a scope computed from what we know cannot widen
// past it. An empty list is legal and means "assert nothing", which the report
// says out loud — see Join.Unasserted.
func NewJoiner(v ports.OrderResolver, p ports.VenuePositions, orders ports.OrderWriteRepository, clk clock.Clock, account string, scopes ...ports.PositionScope) (*Joiner, error) {
	if v == nil {
		return nil, ErrResolverMissing
	}
	if p == nil {
		return nil, ErrVenuePositionsMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if account == "" {
		return nil, ErrJoinAccountUnset
	}
	for _, s := range scopes {
		if err := s.Validate(); err != nil {
			return nil, err
		}
	}
	return &Joiner{
		venue:     v,
		positions: p,
		orders:    orders,
		clk:       clk,
		account:   account,
		scopes:    append([]ports.PositionScope(nil), scopes...),
	}, nil
}

// PositionScope is ports.PositionScope, re-exported for the composition seam.
//
// A composition root has to NAME this type to configure which enumerations the
// invariant is asserted over, and §3.6 rule 6's gate forbids any root importing
// oms/application/ports — the port package is write authority as far as
// singlewriter is concerned, and it polices the abstraction as well as the
// adapter. An alias here is the narrow seam that keeps both true: the root names
// application.PositionScope, which it may already import for ResolutionPolicy,
// and no import edge to the policed package appears anywhere.
type PositionScope = ports.PositionScope

// InstrumentDelta is one instrument's disagreement between the venue's position
// and the exposure this system's own orders account for.
type InstrumentDelta struct {
	// Instrument is which market.
	Instrument ids.VenueInstrument

	// Expected is the signed net this system's orders explain: the sum over
	// every joined order on this instrument of its side applied to the
	// cumulative quantity the venue reported for it.
	Expected fixedpoint.Quantity

	// Observed is the venue's own signed position.
	Observed fixedpoint.Quantity

	// Unexplained is Observed − Expected, and it is carried rather than left to
	// the reader because it is the number the operator acts on: its SIGN says
	// which way the account is exposed beyond what we can account for, and a
	// report that made the reader subtract would be one where a subtraction in
	// the wrong order is invisible.
	Unexplained fixedpoint.Quantity
}

// Join is what one venue join established.
type Join struct {
	// Orders is how many orders were joined to the venue, and Fills is how many
	// executions the join admitted into the projection.
	//
	// Duplicates counts the executions the fill ledger already held. It is not
	// noise: this join re-sweeps from each order's authorization anchor on every
	// boot rather than advancing a cursor, so on the second boot after a fill
	// EVERY row is a duplicate — a run reporting fills and no duplicates where
	// duplicates were expected means the projection lost them.
	Orders     int
	Fills      int
	Duplicates int

	// Absent are the orders the venue says it does not hold. They are NOT a
	// failure and NOT a resolution: §5.3's stable-not-found rule is what decides
	// what an absence licenses, through four gates this use case does not
	// implement, so they are handed back for Resolver to run.
	Absent []ids.ClientOrderID

	// Incomplete are the orders whose executions sweep did not prove coverage
	// through the instant it was asked about — a page budget spent, a venue that
	// stopped answering. Retryable, and the reason the join must not conclude:
	// a truncated sweep's silence is not evidence, and admitting the rows it did
	// return would rebuild a cumulative that is right about everything except
	// how much is missing.
	Incomplete []ids.ClientOrderID

	// Unattributed are the orders whose venue-reported cumulative disagrees with
	// what this join could admit for them.
	//
	// It is the join's own cross-check and it uses two DIFFERENT venue surfaces:
	// the order surface states a total, the executions sweep states the rows. A
	// disagreement means an execution exists that the sweep did not attribute to
	// this order — on Binance the ordinary cause is a trade list carrying no
	// client order id for an order whose venue id we could not read — and the
	// exposure it represents is real whether or not we can name its row.
	Unattributed []ids.ClientOrderID
}

// Assertion is §7.5's net-delta invariant as this system can state it: what the
// venue's position enumeration holds, against what our own orders account for.
//
// # It is a separate result from Join because it is a separate GATE
//
// Until slice G1 the two were one call and one report, and the fusion had a
// consequence nothing could see from inside the join: §5.3's absence resolution
// belongs BETWEEN them. An order the venue's order surface does not hold is
// handed to Resolver, and Resolver is what sweeps its executions — so at the
// instant the fused call asserted, every absent order contributed the zero
// cumulative the replay gave it, while the position those executions produced was
// standing at the venue and counted. See Reconciler, which is the composition
// that made the ordering visible.
type Assertion struct {
	// Unexplained are the instruments whose venue position exceeds what our
	// orders account for. §7.5's "unexplained position deltas", which "alert for
	// operator clearance" and are "not auto-corrected".
	Unexplained []InstrumentDelta

	// Foreign are the venue's own symbols that this deployment does not trade at
	// all, in the venue's spelling, sorted. §7.5's "foreign orders".
	Foreign []string

	// Unasserted are the instruments this join rebuilt fills for and could NOT
	// compare against a position, each with the reason it could not.
	//
	// # It is a field rather than a doc comment because ONE of its reasons is
	// # permanent
	//
	// Every spot leg is in it. Neither venue has a position surface for spot — a
	// spot balance is not a position — and the balance that exists instead
	// includes coins this system never bought, so it is comparable to our
	// exposure only against a baseline nothing records. §7.5 lists `Positions`
	// among the surfaces its join uses without saying that one of a
	// cash-and-carry package's two legs is outside it.
	//
	// That leg does NOT block the gate, and the direction is deliberate: a leg
	// whose invariant can never be asserted would hold the posture below trading
	// for ever, which is this repo's recorded halts-itself-with-every-light-green
	// failure. So the limit is reported on every boot instead, where an operator
	// sees it, rather than inherited silently by whoever reads a clean report.
	//
	// # The reason is carried because the exemption was argued from ONE cause and
	// # this bucket has two
	//
	// Slice F2 wrote the exemption above and computed membership as "no configured
	// scope covered this instrument", which a DERIVATIVE reaches too — an
	// enumeration the deployment did not configure, or a symbol dropped from the
	// config while the journal still holds an open order on it. That instrument is
	// not one whose invariant can never be asserted; it is one whose invariant did
	// not run, over an account that may be holding exposure. Exempting it makes
	// GateNetDeltaAsserted report Passed on a boot that issued no position query
	// at all. See UnassertedReason, which is where the two causes were separated.
	Unasserted []UnassertedInstrument
}

// UnassertedInstrument is one instrument the net-delta invariant did not cover,
// and why.
type UnassertedInstrument struct {
	Instrument ids.VenueInstrument
	Reason     UnassertedReason
}

// UnassertedReason is why an instrument the join rebuilt fills for was left out
// of the net-delta assertion.
//
// [SPEC EDIT LANDED — v0.9.39] §7.5 states the exemption over one cause — "an
// instrument whose exposure cannot be compared MUST be reported … and MUST NOT
// hold the posture", argued from the fact that no venue publishes a position
// surface for spot — and the code that implemented it collected instruments by a
// WIDER predicate. §7.5 now distinguishes the two and says which one holds.
//
// # Why the property is a method on the reason rather than a set the caller knows
//
// UnprovedReason's argument one type over, and it is the same failure it exists
// to foreclose: a caller that recognised "the reasons that do not hold" would
// give a reason added next year the permissive answer by omission — an instrument
// silently outside the invariant, reported as ordinary boot progress for ever.
// Naming every member here makes exhaustiveswitch force the choice instead.
type UnassertedReason uint8

const (
	// UnassertedReasonUnspecified is the zero value and names no reason. Refused
	// by Validate: an instrument outside the invariant with no account of why is
	// exactly the state this type exists to make unwritable.
	UnassertedReasonUnspecified UnassertedReason = iota

	// UnassertedNoPositionSurface is §7.5's own case: the instrument's product
	// family has no position surface at any venue, so no enumeration could ever
	// cover it. Spot, and only spot.
	//
	// A spot balance is not a position — it holds coins this system never bought
	// — so comparing it to our exposure needs a recorded baseline nothing in this
	// spec defines (task #179). Until one exists the spot leg's exposure rests on
	// the executions sweep alone.
	UnassertedNoPositionSurface

	// UnassertedScopeNotEnumerated is a DERIVATIVE instrument that no configured
	// scope covers: an enumeration this deployment did not declare, or one it
	// declared for a venue or family other than the instrument's.
	//
	// It is the same words as the case above — "we could not compare this" — about
	// a completely different situation. A position surface EXISTS for it and was
	// not asked. So the venue may be holding exposure on this instrument, and the
	// only reason nothing reported it is that the query was never issued.
	UnassertedScopeNotEnumerated
)

const maxUnassertedReason = UnassertedScopeNotEnumerated

// Validate reports whether r names a reason, excluding the sentinel.
func (r UnassertedReason) Validate() error {
	if r == UnassertedReasonUnspecified || r > maxUnassertedReason {
		return fmt.Errorf("unasserted reason %d: %w", uint8(r), ErrUnassertedReasonInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (r UnassertedReason) String() string {
	switch r {
	case UnassertedReasonUnspecified:
		return "UnassertedReasonUnspecified"
	case UnassertedNoPositionSurface:
		return "UnassertedNoPositionSurface"
	case UnassertedScopeNotEnumerated:
		return "UnassertedScopeNotEnumerated"
	}
	return fmt.Sprintf("UnassertedReason(%d)", uint8(r))
}

// HoldsPosture reports whether an instrument left out for this reason must hold
// the boot posture below trading.
//
// The two answers are not two degrees of the same thing. A permanent gap held
// against the posture halts a correct system for ever, which is why §7.5 exempts
// it; an unconfigured enumeration exempted is the invariant reporting a pass over
// an account it did not read, which is the check saying "clear" about a question
// it never asked.
//
// It mirrors UnprovedReason.RetryMayLift down to the signature, deliberately: the
// two are the same shape of decision — a property of the reason, not of the
// caller — and a reader who has understood one has understood this one.
func (r UnassertedReason) HoldsPosture() (bool, error) {
	switch r {
	case UnassertedNoPositionSurface:
		return false, nil
	case UnassertedScopeNotEnumerated:
		return true, nil
	case UnassertedReasonUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10). A zero
		// reason holds, for the reason below, and it is a distinct fact from a
		// corrupted one.
		return true, r.Validate()
	}
	// Anything unrecognised holds AND reports. Holding is the safe direction — the
	// permissive one is an instrument outside the invariant that nobody is told
	// about — and the error is what stops a caller reading the bool alone.
	return true, r.Validate()
}

// Holds reports whether this assertion found something §7.5 refuses to
// auto-correct: a position no order of ours explains, a symbol this deployment
// does not trade at all, or an instrument left outside the invariant for a reason
// that is not the permanent one.
//
// The third disjunct is what slice G2 added. Without it the invariant answers
// "clear" about instruments it never queried — see UnassertedReason.
//
// # The error result is discarded, and the reason it is SAFE to discard is a
// # property of HoldsPosture rather than a hope
//
// HoldsPosture answers true for every value it also reports an error on — the
// zero reason and any unrecognised one — so the bool alone is total in the
// direction that matters, and its table test asserts both halves for exactly
// those values. The first version of this loop tested `err != nil || holds`,
// which reads as defence in depth and is not: no input reaches it with err
// non-nil and holds false, so it is a branch nothing can execute, and a mutation
// removing it SURVIVED. An unreachable guard is worse than no guard — it reads as
// handled, and the next reader stops asking who guarantees the invariant.
func (a Assertion) Holds() bool {
	if len(a.Unexplained) > 0 || len(a.Foreign) > 0 {
		return true
	}
	for _, u := range a.Unasserted {
		if holds, _ := u.Reason.HoldsPosture(); holds {
			return true
		}
	}
	return false
}

// Join rebuilds executed quantity from the venue.
//
// # It takes the ids whose state means the venue MAY hold a record
//
// Recovery.Dispatched, not Recovery.Unresolved. The unresolved set is §6.7's —
// orders whose exposure is still uncertain — and it excludes every terminal,
// which is where this system's executions actually are: a marketable IOC that
// took part of the book ends Filled or Canceled with real fills, and the replay
// gave it a zero cumulative like everything else. Joining only the uncertain
// orders would rebuild the quantity for the orders that have none.
//
// # err != nil means the projection is PART-REBUILT and must not be traded on
//
// A venue query that fails, a repository that refuses a fill, a quantity whose
// asset tag disagrees: each leaves the fills admitted so far in place, on
// Rebuild's argument — they are what the venue said, and discarding them would
// leave an operator inspecting a store that disagrees with both the log and the
// exchange. The posture is what stops trading, not the emptiness of a map.
func (j *Joiner) Join(dispatched []ids.ClientOrderID) (Join, error) {
	report := Join{}
	for _, id := range dispatched {
		rec, err := j.load(id)
		if err != nil {
			return report, err
		}
		if _, err := j.joinOne(rec, &report); err != nil {
			return report, err
		}
		report.Orders++
	}
	return report, nil
}

// Assert is §7.5's net-delta invariant: "every unit of the venue's position is
// attributable to an order this system journaled", per derivative instrument in
// the enumerated scopes.
//
// # It re-reads the projection rather than taking Join's arithmetic
//
// Join could have handed its running total over and the two would agree on the
// boot where nothing else writes between them — which is exactly the boot this
// system does not have. §5.3's absence resolution runs BETWEEN the two calls and
// admits executions of its own, so a total computed during the join is a total
// taken before the last writer finished. Re-deriving here makes the assertion a
// function of the projection as it stands at the instant it is asserted, which is
// the only reading §7.5's "before trading resumes" can mean.
//
// It takes the same dispatched set for the same reason Join does: a Planned-only
// order was never sent, so it contributes no exposure and asking about it would
// be asking the venue about an order this system has not placed.
func (j *Joiner) Assert(dispatched []ids.ClientOrderID) (Assertion, error) {
	report := Assertion{}
	expected := make(map[ids.VenueInstrument]fixedpoint.Quantity, len(dispatched))

	for _, id := range dispatched {
		rec, err := j.load(id)
		if err != nil {
			return report, err
		}
		signed, err := signedQuantity(rec.Order.Side, rec.Cumulative)
		if err != nil {
			return report, fmt.Errorf("assert order %q executed: %w", id, err)
		}
		running, ok := expected[rec.Order.Instrument]
		if !ok {
			running = fixedpoint.ZeroQuantity(rec.Cumulative.Asset())
		}
		running, err = running.Add(signed)
		if err != nil {
			return report, fmt.Errorf("assert order %q: %w", id, err)
		}
		expected[rec.Order.Instrument] = running
	}

	if err := j.assert(expected, &report); err != nil {
		return report, err
	}

	// Sorted on the WHOLE instrument identity rather than on the symbol.
	// A cash-and-carry package is spot BTCUSDT and perp BTCUSDT, so the symbol is
	// the one component the two legs share — a symbol-keyed comparison makes them
	// equal, sort.Slice is not stable, and the two legs then swap places between
	// runs. That is Checklist.Blocking's argument arriving on a report whose most
	// common content is exactly one such pair.
	sort.Slice(report.Unexplained, func(a, b int) bool {
		return report.Unexplained[a].Instrument.String() < report.Unexplained[b].Instrument.String()
	})
	sort.Strings(report.Foreign)
	sort.Slice(report.Unasserted, func(a, b int) bool {
		return report.Unasserted[a].Instrument.String() < report.Unasserted[b].Instrument.String()
	})
	return report, nil
}

// load reads one order and refuses one the venue cannot hold.
//
// Shared by Join and Assert so the two cannot disagree about which orders are in
// scope. That is not tidiness: they are handed the same list by the same caller
// and a divergence would show up as an assertion computed over a different set
// from the one whose quantity was rebuilt — a mismatch with no symptom except a
// wrong number.
func (j *Joiner) load(id ids.ClientOrderID) (ports.OrderRecord, error) {
	rec, err := j.orders.Load(id)
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	mayExist, err := rec.State.MayExistAtVenue()
	if err != nil {
		return rec, fmt.Errorf("join order %q at %s: %w", id, rec.State, err)
	}
	if !mayExist {
		// A Planned-only order was never dispatched, so there is nothing at the
		// venue to ask about and a query would run to its deadline over an
		// order this system merely has not sent. Refused rather than skipped:
		// the caller derived this list from a state predicate, and one that
		// disagrees means the two readings have drifted.
		return rec, fmt.Errorf("join order %q at %s: %w", id, rec.State, ErrOrderNotAtVenue)
	}
	return rec, nil
}

// joinOne reads one order's venue record and admits the executions belonging to
// it, returning the record as the projection now holds it.
func (j *Joiner) joinOne(rec ports.OrderRecord, report *Join) (ports.OrderRecord, error) {
	id := rec.Order.ClientOrderID

	status, err := j.venue.Resolve(rec.Order)
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if err := status.Presence.Validate(); err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if status.Presence == ports.VenueHasNoOrder {
		// Handed to §5.3's rule rather than concluded here. An absence is the one
		// answer this use case must not act on: it is the input to the four-gate
		// stable-not-found contract, and reading it as "no executions" would be
		// the inference §5.3 forbids in as many words.
		report.Absent = append(report.Absent, id)
		return rec, nil
	}

	// The sweep runs from the order's own anchor to the venue's clock now. The
	// anchor's EARLIEST edge cannot be after the venue created the order, so no
	// execution of ours precedes it; `now`'s earliest edge is the conservative
	// end, because a window claiming to reach further than the clock can prove
	// would make a truncated sweep look complete.
	if err := rec.AuthorizedAt.Validate(); err != nil {
		// The venue holds an order whose authorization instant this system does
		// not have. The replay is what supplies it (§5.3's anchor, journaled since
		// slice F1), so an unset one here means the record and the log disagree —
		// and sweeping from the epoch instead would ask both venues for a window
		// they refuse as too wide.
		return rec, fmt.Errorf("join order %q: %w: %w", id, ports.ErrAuthorizationNeedsAnchor, err)
	}
	until := j.clk.ServerNow(rec.Order.Instrument.Venue)
	if err := until.Validate(); err != nil {
		return rec, fmt.Errorf("join order %q on %s: %w", id, rec.Order.Instrument.Venue, err)
	}

	// Two authorities can now name this order's venue id — the live query's
	// answer and the ack the dispatch persisted — and they are compared BEFORE
	// any row is attributed, because the loser of a disagreement decides which
	// sweep rows become this order's fills. A non-empty mismatch is refused
	// outright: under the banked non-reuse fact there is no innocent reading,
	// and picking either id would apply executions on a coin flip's authority.
	// Either side being empty is not a mismatch — a lost ack left the store
	// silent, and a surface that returns no id left the query silent; silence
	// agrees with everything.
	if rec.VenueOrderID != "" && status.VenueOrderID != "" && rec.VenueOrderID != status.VenueOrderID {
		return rec, fmt.Errorf("join order %q: store holds venue id %q, venue reports %q: %w",
			id, rec.VenueOrderID, status.VenueOrderID, ErrVenueOrderIDMismatch)
	}

	sweep, err := j.venue.SweepExecutions(rec.Order, rec.AuthorizedAt.Earliest(), until.Earliest())
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if sweep.ProvenThrough.Before(until.Earliest()) {
		report.Incomplete = append(report.Incomplete, id)
		return rec, nil
	}

	// Past the mismatch check the two sources agree or one is silent, so
	// "this order's venue id" has exactly one non-empty answer and either
	// field may carry it — the live id ordinarily, the persisted ack when a
	// surface answered without one. The index is deliberately NOT consulted —
	// nil, not an oversight: an entry for this order could only restate the
	// id resolved here, and entries for OTHER orders only re-derive the
	// rule-out the different-id arm already makes. See attribute's doc.
	venueID := status.VenueOrderID
	if venueID == "" {
		venueID = rec.VenueOrderID
	}
	ours, _ := attribute(sweep.Executions, id, venueID, nil)
	for _, e := range ours {
		fill := domain.Fill{
			ExecutionID: string(e.ExecutionID),
			Account:     j.account,
			Instrument:  rec.Order.Instrument,
			Order:       id,
			Quantity:    e.Quantity,
			Price:       e.Price,
			Fee:         e.Fee,
			Ts:          e.At,
		}
		applied, changed, err := j.orders.ApplyFill(fill, rec.Version)
		if err != nil {
			return rec, fmt.Errorf("join order %q execution %q: %w", id, e.ExecutionID, err)
		}
		rec = applied
		if changed {
			report.Fills++
			continue
		}
		report.Duplicates++
	}

	// The cross-check: the order surface's own total against the rows we could
	// attribute. Two venue surfaces, and a disagreement is exposure that exists
	// and cannot be named.
	agree, err := rec.Cumulative.Cmp(status.Executed)
	if err != nil {
		return rec, fmt.Errorf("join order %q cumulative against venue: %w", id, err)
	}
	if agree != 0 {
		report.Unattributed = append(report.Unattributed, id)
	}
	return rec, nil
}

// assert compares each configured scope's positions against what the join
// accounted for.
//
// # Every instrument is answered for exactly once, and the loop is written to
// # make that checkable
//
// Three sets are in play and they overlap: instruments we joined orders on,
// instruments the venue reported a position for, and instruments in neither. An
// instrument we hold orders on but that the venue did not report is FLAT at the
// venue, which is a real answer here — the enumeration is not scoped by symbol,
// so absence from it is the venue's statement rather than a question we failed
// to ask. An instrument the venue reported that we joined no orders on has an
// expected of zero, and every unit of it is unexplained.
func (j *Joiner) assert(expected map[ids.VenueInstrument]fixedpoint.Quantity, report *Assertion) error {
	asserted := make(map[ids.VenueInstrument]struct{}, len(expected))

	for _, scope := range j.scopes {
		venuePositions, err := j.positions.Positions(scope)
		if err != nil {
			return fmt.Errorf("assert net delta over %s: %w", scope, err)
		}
		report.Foreign = append(report.Foreign, venuePositions.Unrecognized...)

		observed := make(map[ids.VenueInstrument]fixedpoint.Quantity, len(venuePositions.Positions))
		for _, p := range venuePositions.Positions {
			observed[p.Instrument] = p.Net
		}

		// Our instruments in this scope first, then the venue's rows that were not
		// among them. The union is walked rather than either side alone: iterating
		// only our own would miss a position on an instrument we hold no orders
		// for, and iterating only the venue's would miss the opposite — a
		// cumulative we rebuilt against a venue that reports nothing, which is the
		// direction that means a fill was admitted twice.
		for instrument, mine := range expected {
			if instrument.Venue != scope.Venue || instrument.Product != scope.Product {
				continue
			}
			theirs, ok := observed[instrument]
			if !ok {
				theirs = fixedpoint.ZeroQuantity(mine.Asset())
			}
			if err := j.compare(instrument, mine, theirs, report); err != nil {
				return err
			}
			asserted[instrument] = struct{}{}
		}
		for instrument, theirs := range observed {
			if _, done := asserted[instrument]; done {
				continue
			}
			if err := j.compare(instrument, fixedpoint.ZeroQuantity(theirs.Asset()), theirs, report); err != nil {
				return err
			}
			asserted[instrument] = struct{}{}
		}
	}

	for instrument := range expected {
		if _, done := asserted[instrument]; done {
			continue
		}
		report.Unasserted = append(report.Unasserted, UnassertedInstrument{
			Instrument: instrument,
			Reason:     unassertedReason(instrument),
		})
	}
	return nil
}

// unassertedReason says why no scope covered this instrument.
//
// The discriminator is the product family and it is a VENUE FACT rather than a
// configuration one, which is what makes it decidable here: neither venue
// publishes a position surface for spot, so a spot instrument is outside every
// enumeration that could ever be configured. Anything else has a surface, and its
// absence from the scope list is a statement about this deployment's wiring.
//
// It does not consult j.scopes, and that is not an omission: this function is
// only ever reached for an instrument no scope covered, so asking again would
// only re-derive the loop's own answer.
func unassertedReason(instrument ids.VenueInstrument) UnassertedReason {
	if instrument.Product.IsPerp() {
		return UnassertedScopeNotEnumerated
	}
	return UnassertedNoPositionSurface
}

// compare records one instrument's disagreement, if there is one.
//
// A zero difference is not recorded, and that is the whole shape of the
// invariant: §7.5 asserts it before trading resumes, so the ordinary answer is
// an empty report and a report is a list of exceptions.
func (j *Joiner) compare(instrument ids.VenueInstrument, mine, theirs fixedpoint.Quantity, report *Assertion) error {
	unexplained, err := theirs.Sub(mine)
	if err != nil {
		return fmt.Errorf("assert net delta on %s: %w", instrument.Symbol, err)
	}
	sign, err := unexplained.Sign()
	if err != nil {
		return fmt.Errorf("assert net delta on %s: %w", instrument.Symbol, err)
	}
	if sign == 0 {
		return nil
	}
	report.Unexplained = append(report.Unexplained, InstrumentDelta{
		Instrument:  instrument,
		Expected:    mine,
		Observed:    theirs,
		Unexplained: unexplained,
	})
	return nil
}
