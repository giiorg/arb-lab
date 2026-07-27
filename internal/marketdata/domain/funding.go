package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// maxSettlements bounds Enumerate's output. See the doc comment there: it is a
// refusal threshold, not a truncation point.
//
// A year of hourly settlements is 8,760, so this admits roughly a year of the
// shortest interval any venue currently runs. §2.4's holds are measured in
// weeks; anything asking for more than this is a unit error in the caller's
// horizon, not a legitimate plan.
const maxSettlements = 10_000

// FundingObservation is one reading of a perpetual's funding state.
//
// # It is an estimate for ONE settlement, not a curve
//
// §2.4 is explicit that "venue feeds expose an evolving estimate for the next
// settlement, not a forward curve", and the type is named and shaped to keep a
// caller from reading it as more than that. A multi-settlement hold needs
// strategy's FundingForecast; what this carries is §2.4's "raw materials"
// — the live estimate, the live cap/floor, and the live interval.
//
// # The interval is a FIELD because it is not a constant
//
// §2.4: "Funding interval is read per-symbol on every market-data update,
// because Bybit can flip an alt's interval 8h↔1h mid-position without notice."
// A stored-at-entry interval is what makes that flip invisible: the hold's
// settlement count silently octuples while every enumeration keeps returning
// the schedule computed from the old one.
type FundingObservation struct {
	// Instrument is the perpetual this reads. Spot has no funding, and
	// Validate refuses one rather than returning a zero rate — a spot leg
	// reading "funding 0" is indistinguishable from a perp in a flat regime,
	// and the carry computation would net them.
	Instrument ids.VenueInstrument

	// Rate is the venue's current estimate for the NEXT settlement — a
	// per-settlement rate, never per-hour and never annualized. §2.4's
	// "MUST NOT multiply a per-hour rate by hours" defect starts with a field
	// whose units were ambiguous.
	Rate fixedpoint.Rate

	// Interval is the live funding interval. See the type comment.
	Interval time.Duration

	// NextSettle is the venue's next settlement instant, in the venue's own
	// time base — compare against Clock.ServerNow(venue), never Clock.Now.
	NextSettle time.Time

	// Cap and Floor are the venue's live clamps on the rate. §2.4 clamps the
	// forecast's later settlements to them, so they are carried rather than
	// configured: they move, and a stale copy widens the modelled edge in our
	// favour.
	Cap, Floor fixedpoint.Rate

	// Watermark is the observation's provenance (§3.4).
	Watermark Watermark
}

// Validate reports whether the observation could have come from a real venue
// read.
//
// Checked in the order a reader would debug: what it describes, then what it
// says, then how it is bounded, then when, then where it came from.
func (o FundingObservation) Validate() error {
	if err := o.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if o.Instrument.Product == ids.ProductSpot {
		return fmt.Errorf("instrument %s: %w", o.Instrument, ErrFundingOnSpot)
	}
	if err := o.Rate.Validate(); err != nil {
		return fmt.Errorf("rate: %w", err)
	}
	if err := o.Cap.Validate(); err != nil {
		return fmt.Errorf("cap: %w", err)
	}
	if err := o.Floor.Validate(); err != nil {
		return fmt.Errorf("floor: %w", err)
	}
	// Cmp's error is unreachable here — both operands were just validated — but
	// it is checked rather than discarded, because "unreachable given the lines
	// above" is a property of the lines above, and they move.
	c, err := o.Cap.Cmp(o.Floor)
	if err != nil {
		return fmt.Errorf("compare cap to floor: %w", err)
	}
	if c < 0 {
		return fmt.Errorf("cap %s below floor %s: %w", o.Cap, o.Floor, ErrClampInverted)
	}
	if o.Interval <= 0 {
		return fmt.Errorf("interval %v: %w", o.Interval, ErrIntervalInvalid)
	}
	if o.NextSettle.IsZero() {
		return fmt.Errorf("next settle: %w", ErrNextSettleUnset)
	}
	if err := o.Watermark.Validate(); err != nil {
		return fmt.Errorf("watermark: %w", err)
	}
	return nil
}

// Enumerate returns the settlement instants falling inside a hold ending at
// `until`, in ascending order.
//
// # This is §2.4's discrete-settlement rule, and it exists to make one specific
// wrong answer unavailable
//
// §2.4: expected funding capture "MUST enumerate the actual settlement
// timestamps inside the planned hold on each leg (from live nextFundingTime +
// fundingIntervalHour) and sum expected per-settlement rates. It MUST NOT
// multiply a per-hour rate by hours as if funding accrued continuously."
//
// The forbidden calculation is not merely imprecise, it is biased, and the bias
// runs the wrong way. Funding is paid in discrete lumps; a continuous
// approximation credits a fraction of a settlement for every partial interval at
// both ends of the hold. A 20-hour hold on an 8-hour interval captures TWO
// settlements, not two-and-a-half — and milestone 2's kill gate is a comparison
// of carry against a cost hurdle, so a systematic overstatement of carry moves
// the verdict across the threshold in the direction that keeps the project
// alive. That is the expensive direction to be wrong in.
//
// Living here rather than in strategy is deliberate: the schedule is a pure
// function of this observation's own NextSettle and Interval, and a second
// implementation over the same two fields is how two definitions of "the
// settlements in this hold" come to disagree. Strategy owns the FORECAST — the
// EWMA, the quantile, the joint aggregation rule (§2.4) — which is a judgement
// about rates. This is arithmetic about instants.
//
// # Both boundaries are inclusive, and that is the conservative reading
//
// A settlement exactly at NextSettle counts (it is the next one, and it is
// inside any hold that has begun). A settlement exactly at `until` counts too:
// the hold is planned to run through that instant. The alternative — excluding
// the terminal settlement — silently drops one lump from a plan deliberately
// sized to capture it, which understates carry. Understating is the safe
// direction for a kill gate, but it is not the CORRECT one, and a rule that is
// safe by being wrong stops being safe the moment somebody notices and
// "fixes" it.
//
// An `until` before NextSettle yields an empty schedule and no error: a hold
// too short to reach a settlement captures nothing, which is a real answer.
func (o FundingObservation) Enumerate(until time.Time) ([]time.Time, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if until.IsZero() {
		return nil, ErrHorizonUnset
	}
	if until.Before(o.NextSettle) {
		return nil, nil
	}

	// The count is computed before the loop so an absurd horizon is refused
	// rather than allocated. Deriving it by division rather than by counting
	// iterations means the refusal costs one operation, not ten thousand.
	span := until.Sub(o.NextSettle)
	n := int(span/o.Interval) + 1
	if n > maxSettlements {
		return nil, fmt.Errorf("%d settlements over %v at %v: %w", n, span, o.Interval, ErrHorizonTooLong)
	}

	// The cap is clamped, and this is not defensive noise about a case the
	// guard above already excludes. A mutation weakening that guard by one
	// nanosecond survived the suite — correctly, since the loop condition and
	// the guard are two spellings of the same rule and agree — and diagnosing
	// WHY it survived is what surfaced this: for any horizon a full interval or
	// more before NextSettle, n is NEGATIVE (-89 at thirty days on an 8h
	// interval), and make with a negative cap panics.
	//
	// So the early return's real job is not the empty schedule the loop would
	// produce anyway — it is preventing that panic, four lines away, while
	// reading as a business rule. Clamping severs the coupling, so a later
	// simplification of the guard costs a redundant allocation rather than a
	// panic in the code path that prices a hold.
	out := make([]time.Time, 0, max(n, 0))
	for t := o.NextSettle; !t.After(until); t = t.Add(o.Interval) {
		out = append(out, t)
	}
	return out, nil
}
