package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is §5.2's `liq_distance` policy and the ladder it defines — core
// §7.1's opening sentence, which is a claim about WHERE this lives before it is
// a claim about the numbers: *"Liq-distance policy is single-sourced (§5.2)"*.
//
// # Why it is in `risk` and not in `oms`, where its first consumer is
//
// §7.1's monitor is the OMS process-manager (§4 row 8), so the tempting home is
// `oms/domain` beside the aggregate that holds the position. The single-sourcing
// sentence is what rules it out, and it rules it out by naming a SECOND consumer
// that does not live there: §6 step 4 sizes a trade by
// `min(..., margin-for-target-liq-distance)`, and §4's context table assigns
// *"stress/naked-leg sizing"* to `risk`. Two consumers in two contexts, one
// policy — so the policy sits in the context that owns the caps (§5.2's *"Global
// risk caps"* are risk's by the same table) and the OMS reads it, rather than
// the safety context later having to import the OMS's aggregate package to size
// a trade.
//
// The edge that creates is `oms/application` → `risk/domain`, and it is argued
// in `app/reconcile`'s import fence rather than left to ride that fence's
// existing risk exemption silently: that exemption was keyed to the gate
// `risk/application` PRODUCES, and this edge produces no gate.
//
// # The ladder has two axes, and a single enum loses a region of the first one
//
// §5.2 states four numbers — *"open ≥30% / alert <25% / rebalance <20% /
// emergency <15% for BTC; tighter for alts"* — and reading them as four rungs of
// one ladder silently deletes the interval [25%, 30%), where openings are
// withheld and nothing else happens. That region is real and is where a position
// spends most of its drift.
//
// So the policy answers TWO questions rather than returning one member:
//
//	Band(d)           the monitoring ladder — Clear / Alert / Rebalance / Emergency
//	PermitsOpening(d) the §6-step-4 entry gate — d ≥ Open
//
// and [25%, 30%) is exactly `(LiqBandClear, false)`. Adding a fifth enum member
// for it would have been the other repair, and it is the wrong one: the members
// are points in a PRODUCT of two properties, and the product is what the two
// functions state ([[a-vocabulary-with-one-axis-loses-a-case]]).

// PositionSide is which way a monitored position faces.
//
// # It is a parameter rather than an assumption, and the assumption was available
//
// This system's perp leg is a SHORT — cash-and-carry holds spot and shorts the
// perpetual (§1), and §7.1's trigger is stated as *"the perp short nearing
// liquidation on a rally"*. So `LiqDistance` could have taken (mark, liq) alone
// and hard-coded the short's arithmetic. What that costs is not generality: it
// is that the premise "this position is short" would then be stated nowhere,
// held by no type, and checked by nothing — while a LONG's status put through
// the short's formula yields a NEGATIVE distance, which lands on the emergency
// rung and de-risks a healthy position.
//
// # The side cannot be recovered from the two prices, and that is the trap
//
// A short's liquidation price sits above the mark and a long's below it, so
// `liq > mark` looks like a sufficient discriminator. It is not, and the case it
// misses is the one that matters most: a short whose mark has crossed ABOVE its
// liquidation price is a short at or past liquidation — the top of the ladder —
// and is indistinguishable by that test from an ordinary long. Inferring the
// side would therefore report the single most urgent state on the ladder as a
// healthy position facing the other way.
type PositionSide uint8

const (
	// PositionSideUnspecified is the zero value and names no side. Refused by
	// Validate, so a distance computed from a struct nobody populated cannot be
	// obtained — the alternative reads an unset field as one of the two real
	// directions, and whichever one it picks is right half the time.
	PositionSideUnspecified PositionSide = iota

	// PositionSideShort is the perp leg this system actually holds. Its
	// liquidation price is ABOVE the mark and an adverse move is a rally.
	PositionSideShort

	// PositionSideLong is the other direction.
	//
	// This system opens no long perpetual today, and the member exists anyway
	// rather than being deferred: without it the arithmetic below would have no
	// reason to name a direction at all, and the premise it exists to record
	// would go back to being unstated. It is also what makes the formula
	// TOTAL — a caller holding a long gets the right number rather than a
	// negative one.
	PositionSideLong
)

// Validate refuses a side that is the zero value or is not a declared member.
func (s PositionSide) Validate() error {
	switch s {
	case PositionSideShort, PositionSideLong:
		return nil
	case PositionSideUnspecified:
		return fmt.Errorf("risk: position side is unset: %w", ErrPositionSideInvalid)
	}
	return fmt.Errorf("risk: position side %d: %w", uint8(s), ErrPositionSideInvalid)
}

// String renders the side for operator-facing text.
func (s PositionSide) String() string {
	switch s {
	case PositionSideUnspecified:
		return "PositionSideUnspecified"
	case PositionSideShort:
		return "short"
	case PositionSideLong:
		return "long"
	}
	return fmt.Sprintf("PositionSide(%d)", uint8(s))
}

// LiqBand is where a position's liquidation distance sits on §5.2's monitoring
// ladder.
//
// # The members are deliberately NOT ordered by severity
//
// `LiqBandUnknown` is the zero value, which puts it below every rung a numeric
// comparison would rank — and a reader writing `band >= LiqBandAlert` would then
// exclude the one member that is not a distance at all. There is no ordering
// worth having here: Unknown is not "less severe than Alert" or "more severe
// than Emergency", it is a different kind of answer, and #209's ruling below is
// precisely that it does not sit on this ladder. Branch on the member, never on
// `<`.
type LiqBand uint8

const (
	// LiqBandUnknown is NO DISTANCE — the inputs did not yield one.
	//
	// It is the zero value because it is the answer that permits nothing: a band
	// nobody computed must not read as a band nothing is wrong with. See
	// LiqAssessment in oms/application for the reasons a distance goes
	// uncomputed and for what each of them does.
	LiqBandUnknown LiqBand = iota

	// LiqBandClear is a computed distance above every escalation threshold.
	//
	// It is NOT "may open": the interval between the alert threshold and the
	// open threshold is Clear, and openings there are withheld. PermitsOpening
	// is the other axis and answers separately.
	LiqBandClear

	// LiqBandAlert is §5.2's alert threshold breached — an operator
	// notification, no automated action.
	LiqBandAlert

	// LiqBandRebalance is §5.2's rebalance threshold breached: §7.1's margin
	// top-up fires from the staged rescue float.
	LiqBandRebalance

	// LiqBandEmergency is §5.2's emergency threshold breached, and it is also
	// where an ALREADY-BREACHED position lands — a short whose mark has crossed
	// its liquidation price yields a negative distance, which is below every
	// threshold. That is the intended reading rather than an edge case: the
	// venue's liquidation engine has not necessarily acted yet, and a system
	// that reported "no band" for the state past the last one would go quiet at
	// the moment it must not.
	LiqBandEmergency
)

// Validate refuses a band that is not a declared member.
//
// LiqBandUnknown passes: it is a legitimate answer with a ruling attached, not
// an unpopulated field. What makes it safe to admit here is that it permits
// nothing — Validate's usual job of catching the zero value is done by the
// member's own meaning instead.
func (b LiqBand) Validate() error {
	switch b {
	case LiqBandUnknown, LiqBandClear, LiqBandAlert, LiqBandRebalance, LiqBandEmergency:
		return nil
	}
	return fmt.Errorf("risk: liq band %d: %w", uint8(b), ErrLiqBandInvalid)
}

// String renders the band for operator-facing text and metric labels.
func (b LiqBand) String() string {
	switch b {
	case LiqBandUnknown:
		return "unknown"
	case LiqBandClear:
		return "clear"
	case LiqBandAlert:
		return "alert"
	case LiqBandRebalance:
		return "rebalance"
	case LiqBandEmergency:
		return "emergency"
	}
	return fmt.Sprintf("LiqBand(%d)", uint8(b))
}

// LiqDistancePolicy is §5.2's `liq_distance` policy for one instrument: the four
// thresholds, as fractions of the mark.
//
// The zero value is unusable — every field is an unset Rate, which Validate
// refuses. That direction is the one that matters here more than most: a policy
// of all zeroes never breaches any threshold, so a monitor wired to one reports
// every position clear, for ever, with nothing failing.
type LiqDistancePolicy struct {
	// Open is the distance at or above which a NEW position may be opened
	// (§5.2's "open ≥30%"; §6 step 4's `margin-for-target-liq-distance`).
	Open fixedpoint.Rate

	// Alert is the distance below which an operator is notified.
	Alert fixedpoint.Rate

	// Rebalance is the distance below which §7.1's margin top-up fires.
	Rebalance fixedpoint.Rate

	// Emergency is the distance below which §7.1's pre-authorized de-risk fires.
	Emergency fixedpoint.Rate
}

// Validate reports whether the policy is one a ladder may be evaluated against.
//
// # Strictly descending, and the reason is §5.2's own rule about empty regions
//
// §5.2 requires that "a value that makes the permitted region unreachable is
// refused at startup, by name", and states the cost with the cross-leg skew
// limit: every package is refused, nothing reports an error, because each
// individual refusal is the gate working. The same shape reaches this policy
// through equal thresholds rather than through a zero. Alert == Rebalance leaves
// the alert band EMPTY — a rung no distance can ever occupy — so the operator
// notification that exists to precede an automated top-up can never fire, and
// the first thing anyone hears about the position is the top-up itself.
//
// So the four are strictly descending, and each comparison is refused by name.
//
// # No upper bound, and that is deliberate rather than an omission
//
// A threshold above 1 (a distance of more than 100% of the mark) looks like the
// unreachable-region case and is not: a short at well under 1× effective
// leverage has its liquidation price at more than twice the mark, so distances
// above 100% are ordinary. Capping at 1 would refuse a legitimate
// conservatively-sized deployment at startup.
func (p LiqDistancePolicy) Validate() error {
	for _, t := range []struct {
		name string
		rate fixedpoint.Rate
	}{
		{"open", p.Open},
		{"alert", p.Alert},
		{"rebalance", p.Rebalance},
		{"emergency", p.Emergency},
	} {
		sign, err := t.rate.Sign()
		if err != nil {
			return fmt.Errorf("liq distance policy %s: %w", t.name, err)
		}
		if sign <= 0 {
			return fmt.Errorf("liq distance policy %s is %s: %w", t.name, t.rate, ErrLiqThresholdNotPositive)
		}
	}

	for _, pair := range []struct {
		above, below string
		hi, lo       fixedpoint.Rate
	}{
		{"open", "alert", p.Open, p.Alert},
		{"alert", "rebalance", p.Alert, p.Rebalance},
		{"rebalance", "emergency", p.Rebalance, p.Emergency},
	} {
		cmp, err := pair.hi.Cmp(pair.lo)
		if err != nil {
			return fmt.Errorf("liq distance policy %s vs %s: %w", pair.above, pair.below, err)
		}
		if cmp <= 0 {
			return fmt.Errorf("liq distance policy %s %s is not above %s %s: %w",
				pair.above, pair.hi, pair.below, pair.lo, ErrLiqThresholdsNotDescending)
		}
	}
	return nil
}

// Band places a distance on the monitoring ladder.
//
// The comparisons are STRICT and the thresholds are exclusive, exactly as §5.2
// writes them ("alert <25%"), so a distance sitting precisely on a threshold is
// the band ABOVE it. Stated because the two readings differ by one tick of the
// canonical scale and no fixture built from round numbers can tell them apart.
func (p LiqDistancePolicy) Band(d fixedpoint.Rate) (LiqBand, error) {
	if err := p.Validate(); err != nil {
		return LiqBandUnknown, err
	}
	for _, rung := range []struct {
		threshold fixedpoint.Rate
		band      LiqBand
	}{
		{p.Emergency, LiqBandEmergency},
		{p.Rebalance, LiqBandRebalance},
		{p.Alert, LiqBandAlert},
	} {
		cmp, err := d.Cmp(rung.threshold)
		if err != nil {
			return LiqBandUnknown, fmt.Errorf("liq band for %s: %w", d, err)
		}
		if cmp < 0 {
			return rung.band, nil
		}
	}
	return LiqBandClear, nil
}

// PermitsOpening reports whether a distance clears §5.2's open threshold.
//
// The other axis of the ladder, and a separate question from Band because the
// interval between Alert and Open is one where the answer here is false and the
// band is Clear. A caller MUST NOT derive one from the other.
func (p LiqDistancePolicy) PermitsOpening(d fixedpoint.Rate) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	cmp, err := d.Cmp(p.Open)
	if err != nil {
		return false, fmt.Errorf("liq opening gate for %s: %w", d, err)
	}
	return cmp >= 0, nil
}

// LiqDistancePolicies is §5.2's policy per instrument — "tighter for alts".
//
// # There is no default, and that is the load-bearing decision
//
// The natural shape is one policy with per-symbol overrides, and its failure
// direction is the wrong one: an alt nobody wrote a row for would inherit BTC's
// 30/25/20/15, which §5.2 says explicitly is too loose for it. A missing row is
// therefore a refusal that names the instrument, not a fallback — §3.6 rule 4's
// argument (deny until a reason is written down) applied to a threshold table.
type LiqDistancePolicies map[ids.VenueInstrument]LiqDistancePolicy

// Validate refuses an empty table or any member that will not validate.
//
// Empty is refused rather than treated as "monitors nothing", because a monitor
// wired to an empty table cannot report a single band and every position it
// holds reads as unmonitorable — which is a wiring defect wearing the costume of
// a conservative refusal.
func (ps LiqDistancePolicies) Validate() error {
	if len(ps) == 0 {
		return ErrLiqPoliciesEmpty
	}
	for vi, p := range ps {
		if err := vi.Validate(); err != nil {
			return fmt.Errorf("liq distance policies: instrument: %w", err)
		}
		if err := p.Validate(); err != nil {
			return fmt.Errorf("liq distance policies %s: %w", vi.Symbol, err)
		}
	}
	return nil
}

// For returns the policy for one instrument, refusing an instrument the table
// does not cover.
func (ps LiqDistancePolicies) For(vi ids.VenueInstrument) (LiqDistancePolicy, error) {
	p, ok := ps[vi]
	if !ok {
		return LiqDistancePolicy{}, fmt.Errorf("liq distance policy for %s on %s: %w",
			vi.Symbol, vi.Venue, ErrLiqPolicyUnconfigured)
	}
	return p, nil
}

// LiqDistance returns how far the mark must move, as a fraction of the mark, to
// reach the venue's published liquidation price for a position on this side.
//
// # The denominator is the MARK, and the alternative is not equivalent
//
// Dividing by the liquidation price instead answers a different question — how
// far the liquidation price is from the mark as a fraction of ITSELF — and gives
// a systematically smaller number for a short, because the liq price is the
// larger of the two. A policy calibrated against one denominator and evaluated
// against the other is wrong by that ratio in the permissive direction. §5.2's
// thresholds are read as price moves ("a 20% rally away from liquidation"), so
// the denominator is where the price is now.
//
// # The result may be NEGATIVE, and that is the state the ladder most needs
//
// A short whose mark has crossed above its liquidation price has a negative
// distance. Taking an absolute value here would be the natural-looking
// defensive move and is catastrophic: past liquidation, |liq − mark| GROWS as
// the rally continues, so the most urgent position on the book would report an
// improving distance and climb back down the ladder. The sign is preserved and
// LiqBandEmergency is where it lands.
//
// # RoundFloor, not RoundTowardZero
//
// Every consumer of this number treats SMALLER as more conservative — Band's
// comparisons are `<` against the thresholds, and PermitsOpening's is `≥` — so
// the safe rounding direction is toward negative infinity, uniformly.
// RoundTowardZero is the mode that looks right and is conservative only on the
// positive side: applied to a negative distance it rounds toward zero, making an
// already-breached position look marginally less breached.
func LiqDistance(side PositionSide, mark, liq fixedpoint.Price) (fixedpoint.Rate, error) {
	if err := side.Validate(); err != nil {
		return fixedpoint.Rate{}, err
	}

	// Both positivity checks are restated rather than trusted. Every production
	// path into here has already made them — venue.Mark.Validate screens the
	// mark, ports.MarginReading.Validate screens the liq price — and this
	// function is exported, so a caller that built its inputs by hand reaches it
	// with neither. A mark of zero would divide, and a negative one would invert
	// the direction of the whole ladder while every number still validated.
	markSign, err := mark.Sign()
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance mark: %w", err)
	}
	if markSign <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance mark is %s: %w", mark, ErrLiqMarkNotPositive)
	}
	liqSign, err := liq.Sign()
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance liquidation price: %w", err)
	}
	if liqSign <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance liquidation price is %s: %w", liq, ErrLiqPriceNotPositive)
	}

	// The adverse direction, and it is the whole reason the side is a parameter:
	// a short is liquidated by a rally, a long by a fall.
	var gap fixedpoint.Price
	switch side {
	case PositionSideShort:
		gap, err = liq.Sub(mark)
	case PositionSideLong:
		gap, err = mark.Sub(liq)
	case PositionSideUnspecified:
		// Unreachable: Validate above refuses it. Named because
		// `exhaustiveswitch` requires every member and because a `default` here
		// is exactly where a third side added later would land silently.
		return fixedpoint.Rate{}, side.Validate()
	}
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance gap: %w", err)
	}

	d, err := gap.Div(mark, fixedpoint.RoundFloor)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("liq distance: %w", err)
	}
	return d, nil
}
