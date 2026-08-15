package pair

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// drive holds what the paired cycle needs from both sides.
type drive struct {
	cfg     Config
	runners [2]*reconcile.Runner
	rep     *Report
	stderr  io.Writer
	wait    func(time.Duration)
}

// run is the sequential cycle: open leg 0, establish it, and open leg 1 only
// against what leg 0 actually took.
//
// # Why the second leg is gated on the FILL and not on the submission
//
// A leg that submitted and filled nothing opened no exposure, so hedging it
// opens the naked position this whole root exists to prevent. A leg whose fill
// could not be established has not been measured, and sizing a hedge from an
// unmeasured zero is the same mistake made confidently. So the gate reads
// Establish's three-valued answer, and two of the three stop the run.
//
// # Leg index is not side index
//
// Leg-list order decides which leg is A and which is B; LegConfig.Venue decides
// which runner, filter map and report slot each leg belongs to. Every access
// below resolves the side through sideForLeg, because treating the leg index as
// a side index is a silent cross-wire that only a reversed document exposes.
func (d *drive) run() {
	sideA := d.sideForLeg(0)
	sideB := d.sideForLeg(1)

	open0, err := d.open(0, d.mustQuantity(0))
	d.rep.Sides[sideA].Open = open0
	if err != nil {
		// Nothing reached the venue on leg A, so nothing is exposed and leg B
		// is not touched. The refusal is the report's.
		d.rep.Refusal = err
		d.rep.Sides[sideA].Exposure = unopenedPartner(d.cfg, 0)
		d.rep.Sides[sideB].Exposure = unopenedPartner(d.cfg, 1)
		d.rep.Verdict, err = Compose(d.rep.Sides[sideA].Exposure, d.rep.Sides[sideB].Exposure)
		d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		return
	}

	d.rep.Sides[sideA].Exposure, err = d.establish(0, open0.ID)
	if err != nil {
		d.rep.Refusal = err
	}
	a := d.rep.Sides[sideA].Exposure

	// flat() is Sign()==0 with the error surfaced, never absorbed: a quantity
	// nobody populated must not read as "filled nothing".
	aFlat, flatErr := flat(a.Filled)
	if !a.Established || flatErr != nil || aFlat {
		// Three stops, one branch, and they are told apart by Compose rather
		// than here: indeterminate (a human), an unreadable quantity (a human),
		// or a clean no-fill (nothing opened). None of them sends leg 1.
		if flatErr != nil {
			d.rep.Refusal = errors.Join(d.rep.Refusal, flatErr)
			a.Established, a.Because = false, flatErr.Error()
			d.rep.Sides[sideA].Exposure = a
		}
		d.rep.Sides[sideB].Exposure = unopenedPartner(d.cfg, 1)
		d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
		d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		return
	}

	open1, openErr := d.open(1, a.Filled)
	d.rep.Sides[sideB].Open = open1
	if openErr != nil {
		// D5's refusal path: side A's fill could not be expressed as a leg on
		// side B — below its min quantity or notional, off its grid, or refused
		// by Execute before Place. Nothing was sent on B.
		d.rep.Refusal = openErr
		d.rep.Sides[sideB].Exposure = unopenedPartner(d.cfg, 1)
		d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
		d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		return
	}

	d.rep.Sides[sideB].Exposure, err = d.establish(1, open1.ID)
	if err != nil {
		d.rep.Refusal = errors.Join(d.rep.Refusal, err)
	}
	d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
	d.rep.Refusal = errors.Join(d.rep.Refusal, err)
}

// open plans one leg and executes it.
//
// On every not-sent path it returns a NON-NIL *LegReport carrying Refusal, so
// the report can tell "an attempt was refused" from "no attempt was made" — the
// distinction a nil pointer alone would lose.
func (d *drive) open(leg int, quantity fixedpoint.Quantity) (*LegReport, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()

	plan, err := d.cfg.openPlan(leg, quantity)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	records, err := m.Plan.Plan(plan)
	if err != nil {
		// The planner's §2.4 grid verification lands here, which is D5's
		// refusal: an unhedgeable fill is a refusal to send rather than a
		// rounded-off order.
		//
		// But it is NOT the only thing that lands here, and wrapping everything
		// in ErrHedgeUnsizable would tell an operator the market grid rejected a
		// size when the truth was something else entirely — a repeated
		// (Trade, Leg, Attempt), a repository error, a journal error. Reported
		// as a sizing problem, an operator would go and edit a quantity that was
		// never wrong. So the sentinel is attached ONLY to the three SIZE
		// refusals and ONLY on leg B, the only leg sized to something other than
		// the document.
		refusal := err
		if leg == 1 && isFilterSizeRefusal(err) {
			refusal = fmt.Errorf("side %d (%s): %v: %w",
				side, d.rep.Sides[side].Venue, err, ErrHedgeUnsizable)
		}
		return &LegReport{Refusal: refusal}, refusal
	}

	refs := make([]application.DispatchRef, len(records))
	for i, rec := range records {
		refs[i] = application.DispatchRef{ID: rec.Order.ClientOrderID, Version: rec.Version}
	}
	ex, err := m.Execute.Execute(d.runners[side].Checklist(), refs, application.UrgencyOpening)
	if err != nil {
		// The package preflight refused the whole package: nothing was sent.
		return &LegReport{ID: refs[0].ID, Refusal: err}, err
	}
	if ex.Legs[0].Err != nil {
		// The executor's split signature is deliberate: a per-leg Err means THIS
		// leg was not sent and remains dispatchable. Reading only the aggregate
		// error recreates the `if err != nil { return }` reflex that signature
		// exists to prevent — after which establish waits out its whole bound on
		// a still-planned order and D8 refuses the unwind as indeterminate.
		return &LegReport{ID: refs[0].ID, Refusal: ex.Legs[0].Err, Result: ex.Legs[0]}, ex.Legs[0].Err
	}
	return &LegReport{ID: refs[0].ID, Result: ex.Legs[0]}, nil
}

// establish re-joins one leg to venue truth until its state is terminal, then
// reads the projection.
//
// # It is BOUNDED, and the bound is a config field rather than a constant
//
// The loop's exit condition is a venue publishing trade rows, which is
// something this process does not control. An unbounded wait here would be a
// composition root blocking forever on a venue that went quiet, and the honest
// answer after the bound is INDETERMINATE — which stops the run for a human,
// sends nothing, and unwinds nothing.
//
// # Join is what ADVANCES the leg (task #242)
//
// Since `01d27ce` the joiner journals and applies the venue's lifecycle reading
// as its last step, so LegState reads a terminal state on the ordinary path.
// Before that, this loop would have waited out its whole bound on every
// successful order, because nothing moved a leg off Submitted.
func (d *drive) establish(leg int, id ids.ClientOrderID) (LegExposure, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()
	venue := d.rep.Sides[side].Venue

	var last LegExposure
	for attempt := 0; attempt < d.cfg.Pair.SettleAttempts; attempt++ {
		if attempt > 0 {
			d.wait(d.cfg.Pair.Settle.Duration())
		}
		joined, err := m.Join.Join([]ids.ClientOrderID{id})
		if err != nil {
			return LegExposure{Venue: venue, ID: id, Because: err.Error()}, err
		}
		st, err := m.LegState(id)
		if err != nil {
			return LegExposure{Venue: venue, ID: id, Because: err.Error()}, err
		}
		last = Establish(venue, id, st, factsFor(id, joined))
		if last.Established {
			return last, nil
		}
	}
	return last, nil
}

// factsFor turns the join report's per-order finding lists into this leg's own
// facts. All FOUR of them: Contradicted is #242's class, and a leg the join
// latched OutcomeNeedsOperator over must never read as established.
func factsFor(id ids.ClientOrderID, j application.Join) JoinFacts {
	return JoinFacts{
		Absent:       slices.Contains(j.Absent, id),
		Incomplete:   slices.Contains(j.Incomplete, id),
		Unattributed: slices.Contains(j.Unattributed, id),
		Contradicted: slices.Contains(j.Contradicted, id),
	}
}

// flat reports whether q is zero, and NEVER absorbs the error into "yes".
//
// fixedpoint.Quantity.Sign is fallible so that "a flat position and a position
// nobody populated" stay distinguishable, and at this call site that
// distinction is the difference between "leg A opened nothing" and "leg A's
// size was never read" — one of which sends leg B and the other of which must
// not.
func flat(q fixedpoint.Quantity) (bool, error) {
	sign, err := q.Sign()
	if err != nil {
		return false, err
	}
	return sign == 0, nil
}

// mustQuantity is the document's stated size for one leg.
//
// It cannot fail after Validate — legIntent is the shared half of both, which
// is exactly the property exercise's config keeps — and the panic is the
// wiring-defect assertion rather than an error path: a document that validated
// and then failed to state a quantity is a bug in this package, and returning
// an error here would invite a caller to carry on with a zero.
func (d *drive) mustQuantity(leg int) fixedpoint.Quantity {
	intent, err := d.cfg.legIntent(leg)
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate: %v", leg, err))
	}
	return intent.Quantity
}

// sideForLeg binds by venue. Leg-list order decides A/B sequencing; it is not
// a runner index. Treating it as one sends a reversed document through the
// other venue's module, filter map and report slot.
func (d *drive) sideForLeg(leg int) int {
	side, ok := d.cfg.sideFor(d.cfg.Pair.Legs[leg].Venue)
	if !ok {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate", leg))
	}
	return side
}

// unopenedPartner is the exposure of a side this root DECIDED not to send.
//
// It is Established: "we chose not to send it" is a measurement, and it is a
// different fact from "we sent it and could not read the result". Collapsing
// the two is what would let an unreadable leg travel through Compose as a
// confident zero — the hazard D4 exists for, arriving by the back door. The
// quantity carries the leg's own base asset, because a zero with no asset tag
// is the value fixedpoint refuses to let read as flat.
//
// It has no ID and no State, deliberately: there is no order to name. The
// operator report keys its exposure line on a non-empty ID, so a side that was
// never sent prints nothing rather than a fabricated flat reading.
func unopenedPartner(cfg Config, leg int) LegExposure {
	intent, err := cfg.legIntent(leg)
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate: %v", leg, err))
	}
	side, _ := cfg.sideFor(cfg.Pair.Legs[leg].Venue)
	zero, err := intent.Quantity.Sub(intent.Quantity) // zero, carrying the base asset
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d quantity is not subtractable from itself: %v", leg, err))
	}
	return LegExposure{
		Venue:       cfg.Sides[side].OMS.Venue.Venue,
		Instrument:  intent.Instrument,
		Side:        intent.Side,
		Filled:      zero,
		Established: true,
	}
}

// isFilterSizeRefusal reports whether a planning error is one the LEG'S SIZE
// caused.
//
// Three of §2.4's five refusals are: off the lot grid, below the min quantity,
// below the min notional. The other two are not about the size at all — a tick
// violation is the document's limit price, and an unknown filter row is a
// wiring defect — and reporting either as an unsizable hedge would send an
// operator to edit a quantity that was never wrong.
func isFilterSizeRefusal(err error) bool {
	return errors.Is(err, application.ErrQuantityOffLotStep) ||
		errors.Is(err, application.ErrBelowMinQty) ||
		errors.Is(err, application.ErrBelowMinNotional)
}
