package venue

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// MaxFundingIntervalHours bounds FundingIntervalFromHours.
//
// # [SPEC EDIT LANDED — v0.9.9] §4 stated a units discipline for money and
// # none for time
//
// §4 required that "all numeric fields cross the boundary as dimensional
// fixedpoint (parsed from venue decimal strings), never float64" — a rule about
// money, and a good one. Nothing said anything about the venue fields that are
// DURATIONS, and one of them is load-bearing: Bybit's fundingIntervalHour is a
// whole number of hours, and §2.4's settlement enumeration divides by whatever
// it becomes.
//
// A conforming reimplementation could multiply it by time.Minute and satisfy
// that spec completely. §2.4 then enumerates a hold's settlements against a
// 480-minute interval, gets a plausible count, and the kill-gate carry figure
// is wrong by a factor of sixty with nothing failing. This is §10's own
// argument — landed in v0.9.7 for the OBSERVABILITY seam, where "a duration
// recorded in the wrong unit... renders as a plausible dashboard number and
// fails nothing" — and the venue seam is where the duration is *created*, one
// layer earlier and with the same consequence.
//
// The edit was the rule, not the constant: §4 now requires that a venue field
// whose unit is implicit in its NAME crosses this seam through a conversion
// that states the unit, that a value outside the range that unit can plausibly
// hold is REFUSED rather than converted, and that a zero or absent duration is
// refused rather than defaulted to the venue's documented value. §4 already had
// the shape of that sentence for money; it now has the other half.
//
// # It is a UNIT check, not a venue-policy one. Every perpetual either MVP venue
// runs settles at 1h, 2h, 4h or 8h (Profiles §1), so 24 admits every real value
// with room to spare; what it rejects is an argument that is not in hours. A
// venue sending 8h as 480 minutes, or a caller passing seconds, or a
// time.Duration truncated to an int — each lands orders of magnitude above the
// band, and each would otherwise produce an interval that is silently wrong by
// the same factor. §2.4's settlement count is a division by this value, so the
// error does not surface as a failure: it surfaces as a hold that captures a
// plausible-looking number of settlements.
//
// A venue that genuinely introduced a longer interval would fail here loudly,
// which is the right direction: it is a venue fact, and Profiles is where a
// venue fact is verified and dated before code relies on it.
const MaxFundingIntervalHours = 24

// Funding is one venue's normalized funding reading — §4's StreamFunding
// payload.
//
// # It carries no provenance and no instrument
//
// §4: "The Envelope is authoritative for provenance; a normalized payload
// carries none." Source, VenueSeq, SourceTime and RecvTime are fields §3.5
// defines for every stream, so a payload restating them is a second copy of one
// fact with nothing forbidding the two to disagree — and the unsafe direction is
// reachable without any implementer noticing they chose it, because gap
// detection reads VenueSeq while freshness reads the timestamps. A payload
// likewise names no instrument: SourceID is the mailbox key, so a payload naming
// a different one is a state no feed can produce and every adapter would
// otherwise have to check.
//
// # It is the venue's reading and nothing derived from it
//
// There is no freshness verdict here, no gap flag, and no forecast. Those are
// judgements about a reading, and each belongs to whoever holds the clock or the
// history it needs — marketdata computes freshness against an injected clock
// (§3.4), strategy owns the forward estimator (§2.4). A DTO that carried a
// verdict would be a verdict computed at the wire, where §3.6 rule 5 forbids
// reading the clock at all.
//
// Fields are exported for §8's capture lane, which records normalized DTOs and
// replays them.
type Funding struct {
	// Rate is the venue's current estimate for the NEXT settlement — a
	// per-settlement rate, never per-hour and never annualized. §2.4's "MUST
	// NOT multiply a per-hour rate by hours" defect starts with a field whose
	// units were ambiguous, and it is stored as a plain ratio: an 0.01% funding
	// rate is 0.0001, per fixedpoint.Rate's own contract. The percent-to-ratio
	// conversion is the adapter's, done once where the venue's own text is
	// parsed.
	Rate fixedpoint.Rate

	// Interval is the live funding interval, read on every update rather than
	// stored at entry (§2.4: Bybit can flip an alt 8h↔1h mid-position without
	// notice). Validate refuses a zero — see ErrIntervalInvalid, which is what
	// makes an omitted Bybit delta field a refusal instead of an interval.
	Interval time.Duration

	// NextSettle is the venue's next settlement instant, in the VENUE's own
	// time base. It is compared against Clock.ServerNow(venue), never
	// Clock.Now: §3.4 measures data age against the venue's clock precisely
	// because the two disagree, and the disagreement is what a venue-side stall
	// looks like.
	NextSettle time.Time

	// Cap and Floor are the venue's live clamps on the rate, carried rather
	// than configured because they move (Profiles §1: Binance overlays them via
	// fundingInfo, Bybit sends fundingCap live) and a stale copy widens the
	// modelled edge in our favour.
	Cap, Floor fixedpoint.Rate
}

// Validate reports whether the reading could have come from a real venue.
//
// # It does not check that Rate lies inside the clamps, deliberately
//
// The clamps bound what a settlement may PAY; the rate here is an evolving
// estimate for a settlement that has not happened, and a venue is free to
// publish an estimate its own clamp will later bind. Refusing that would drop
// the reading that says the rate is pinned at the cap — the single most
// informative funding reading there is — and it would drop it silently as a
// malformed message rather than loudly as a clamped one. §2.4 clamps the
// FORECAST to cap/floor, which is where the bound belongs: on the number
// entering a decision, not on the number arriving from the wire.
//
// Checked in the order a reader would debug: what it says, then how it is
// bounded, then how often, then when.
func (f Funding) Validate() error {
	if err := f.Rate.Validate(); err != nil {
		return fmt.Errorf("funding rate: %w", err)
	}
	if err := f.Cap.Validate(); err != nil {
		return fmt.Errorf("funding cap: %w", err)
	}
	if err := f.Floor.Validate(); err != nil {
		return fmt.Errorf("funding floor: %w", err)
	}

	// Cmp's error is unreachable here — both operands were just validated — but
	// it is checked rather than discarded, because "unreachable given the lines
	// above" is a property of the lines above, and they move.
	c, err := f.Cap.Cmp(f.Floor)
	if err != nil {
		return fmt.Errorf("funding compare cap to floor: %w", err)
	}
	if c < 0 {
		return fmt.Errorf("funding cap %s below floor %s: %w", f.Cap, f.Floor, ErrClampInverted)
	}

	if f.Interval <= 0 {
		return fmt.Errorf("funding interval %v: %w", f.Interval, ErrIntervalInvalid)
	}
	if f.NextSettle.IsZero() {
		return fmt.Errorf("funding: %w", ErrNextSettleUnset)
	}
	return nil
}

// FundingIntervalFromHours converts a venue's whole-hour interval field into a
// Duration.
//
// # There is no 8-hour default here, and its absence is the design
//
// Profiles §1 records that the two venues supply this differently, and the
// asymmetry is the whole reason this function exists rather than a multiply at
// each call site. Bybit sends fundingIntervalHour on the ticker, live. Binance
// sends NO interval field at all: it comes from a separate REST overlay
// (GET /fapi/v1/fundingInfo), and Profiles gives the rule as "absent ⇒ 8h
// default".
//
// The tempting implementation of that rule is a fallback on the zero value —
// if hours == 0 { return 8 * time.Hour }. It is not here, and this repo's
// standing rule says why: when a default has two directions, pick the one that
// is right when somebody forgets it. A zero-valued interval reaches this
// function from two completely different situations. One is Binance's overlay
// genuinely reporting nothing, where 8h is correct. The other is an overlay
// that was never fetched, a decode that dropped the field, or Bybit's ticker
// sending a DELTA in which an unchanged field is simply absent (Profiles §1:
// "absent field = unchanged, never null-coerce"). A zero-value fallback answers
// all three identically, and two of the three answers are a wiring failure
// wearing a correct BTC reading's clothes.
//
// So zero is refused, and the Binance adapter applies its default from a
// positive fact — an overlay that was fetched and came back empty — which is a
// state a caller has to be holding to express. That keeps the one venue whose
// alts can settle hourly (Profiles §1's 8h↔1h auto-switch, no notice) from
// having its flip absorbed by a default, which is §2.4's C8 defect exactly:
// the hold's settlement count silently octuples while every enumeration keeps
// returning the schedule computed from the old interval.
func FundingIntervalFromHours(hours int) (time.Duration, error) {
	if hours <= 0 {
		return 0, fmt.Errorf("funding interval %dh: %w", hours, ErrIntervalInvalid)
	}
	if hours > MaxFundingIntervalHours {
		return 0, fmt.Errorf("funding interval %dh above %dh: %w",
			hours, MaxFundingIntervalHours, ErrIntervalRange)
	}
	return time.Duration(hours) * time.Hour, nil
}
