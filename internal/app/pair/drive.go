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
	d.openAndCompose()
	// ONE tail. Every branch above that composes a verdict reaches this call,
	// because no composed verdict may bypass compensation through an early
	// return — including D5's unsizable-hedge path, which is the one that
	// leaves side A exposed with nothing sent on side B.
	d.compensate()
}

// openAndCompose is the drive proper: it opens what it may and leaves a
// composed verdict in the report. It never compensates — see run.
func (d *drive) openAndCompose() {
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

// compensate is the unwind, and its NARROWNESS is the design (D8).
//
// A compensating trade is §6.7's gate's business — "the single gate for every
// compensating trade" — and record §95 §4 found that §6.7 cannot see a
// cross-venue pair at all: HedgeGroup is (Account, Venue, Symbol) and the pair
// sits in two groups with no group holding both. So this root does not get to
// compensate on a reading it could not fully establish.
//
// VerdictHalfOpen with the exposure on leg A is the ONE reading where the answer
// is unambiguous: A's fill is measured, B opened nothing, and the flat-start
// gate means closing A returns the account to where it started. Without that
// gate, an opening sell that merely reduced a pre-existing long would be
// followed by a reduce-only buy that increases the position. Every other exposed
// reading ends here for a human:
//
//   - VerdictSkewed: both sides carry exposure and the residual is a per-venue
//     quantity §6.7 exists to size. Unwinding either side by this root's
//     arithmetic would be that gate's job done outside it.
//   - VerdictIndeterminate: at least one leg's fate is unknown. An unwind
//     against an unknown leg can DOUBLE the exposure it meant to close.
//   - VerdictHalfOpen on leg B: expressible, and out of scope for phase 1 — the
//     drive opens leg A first, so the only way B is exposed alone is an A fill
//     that later read as zero, which is a contradiction the run must not
//     resolve by trading.
//
// An unreadable quantity is NOT an unwind. flat's error means leg A's fill was
// never populated, and Compose has already answered Indeterminate for it — but
// the guard checks it here rather than resting on that, because a premise about
// another function's output is exactly the thing that goes stale silently.
// aFlat stays false on the error path, so the `aErr == nil` term is what
// refuses.
func (d *drive) compensate() {
	sideA := d.sideForLeg(0)
	exposure := d.rep.Sides[sideA].Exposure
	aFlat, aErr := flat(exposure.Filled)

	if !(d.rep.Verdict == VerdictHalfOpen && exposure.Established && aErr == nil && !aFlat) {
		if Exposed(d.rep.Verdict) {
			d.rep.Refusal = errors.Join(d.rep.Refusal,
				fmt.Errorf("verdict %s: %w", d.rep.Verdict, ErrUnwindRefused))
		}
		return
	}

	unwound, err := d.unwind(0, exposure.Filled)
	d.rep.Sides[sideA].Unwind = unwound
	if err != nil {
		d.rep.Refusal = errors.Join(d.rep.Refusal, err)
	}
	if unwound == nil || unwound.Refusal != nil {
		// A refused unwind leaves the pair exactly as it was. The verdict is not
		// touched: the report must state where the pair ENDED, and an attempt is
		// not an outcome.
		return
	}

	// Re-read leg A and recompose. The close counts ONLY when the unwind's own
	// fill matches the exposure it was sized to: a partial close is still
	// exposure, and a comparison that accepted "some of it filled" would report
	// a half-closed pair as unopened — the flattering direction.
	after, _ := d.establish(0, unwound.ID)
	if !after.Established {
		return
	}
	cmp, err := after.Filled.Cmp(exposure.Filled)
	if err != nil || cmp != 0 {
		return
	}
	// Exposure is NOT overwritten: it stays the opening reading, which is a fact
	// about what the open did and belongs in the report. What changed is the
	// pair's net position, and the verdict is where that is stated. Nothing here
	// constructs a zero quantity either — a literal fixedpoint.Quantity{} carries
	// no asset tag and is precisely the value Sign refuses, so manufacturing one
	// to mean "flat now" would put an unreadable value into the report.
	d.rep.Verdict = VerdictUnopened
}

// unwind plans and sends the reduce-only close of one leg.
//
// UrgencySafety, not UrgencyOpening: reducing exposure is §7.6's safety class,
// and the difference is load-bearing rather than cosmetic. A stale-projection
// latch raised by the opening leg's own fan-in refuses the next OPENING package
// at the executor's admission read — and if the unwind were classed as one, the
// hold that a failed apply raises would block the very order that closes the
// exposure the hold is about.
func (d *drive) unwind(leg int, filled fixedpoint.Quantity) (*LegReport, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()

	plan, err := d.cfg.unwindPlan(leg, filled)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	records, err := m.Plan.Plan(plan)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	refs := []application.DispatchRef{{ID: records[0].Order.ClientOrderID, Version: records[0].Version}}
	ex, err := m.Execute.Execute(d.runners[side].Checklist(), refs, application.UrgencySafety)
	if err != nil {
		return &LegReport{ID: refs[0].ID, Refusal: err}, err
	}
	if ex.Legs[0].Err != nil {
		return &LegReport{ID: refs[0].ID, Refusal: ex.Legs[0].Err, Result: ex.Legs[0]}, ex.Legs[0].Err
	}
	return &LegReport{ID: refs[0].ID, Result: ex.Legs[0]}, nil
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
