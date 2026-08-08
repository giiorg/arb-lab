package application

import (
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	riskdomain "github.com/giiorg/arb-lab/internal/risk/domain"
)

// This file is core §7.1's liq-distance trigger — the first consumer of BOTH
// read ports built for it (task #207's MarkRiskReader, task #208's
// VenueMarginStatus), and the place eight comments across seven packages have
// been deferring a computation to since §12's milestone-3 closure pass found
// that "nothing in this system sizes a trade today".
//
// # What this slice delivers and what it deliberately does not
//
// It ASSESSES. It computes the distance, places it on §5.2's ladder, and says
// whether it objects to a new opening. It takes no action: the margin top-up
// (B3's `MarginOp`) and the `PackageDelever` escalation (B4, the first caller of
// Deleverer.Open) are the rungs above, and they consume this answer. Splitting
// there is not staging convenience — an assessment that acted would have to hold
// the write path, and this type would then be a second writer alongside the
// Deleverer that already owns the ladder.
//
// # Every venue-side failure is a LADDER STATE, not an error
//
// The two ports can each fail to answer, and the shape that suggests itself —
// returning the error — is the wrong one here, because §7.1 attaches a
// CONSEQUENCE to not knowing: a stale mark "blocks openings and escalates open
// positions". An error a caller logs and moves past has silently lifted that
// block. So an unreadable venue answer, an absent mark and a stale mark are all
// LiqUnknownReason members on a returned assessment, and the assessment always
// says what it withholds.
//
// The error return is reserved for WIRING defects — an instrument with no
// configured policy, a side nobody stated, a venue with no mark budget. Those
// are conditions no venue can cause and no retry can clear, and collapsing them
// into the same channel as an outage is what makes an outage look like a bug
// report.

// ErrMarkRiskReaderNil, ErrVenueMarginStatusNil and ErrMarkMaxAgeUnconfigured
// report a monitor that could never do its job, refused at construction.
//
// The first two are refused rather than tolerated as "monitoring disabled"
// because a nil port would answer nothing for every instrument for ever, which
// this file's own ladder reports as the conservative unknown state — so the
// defect would wear the costume of a system being careful. markstate.Reader
// makes the identical refusal one layer down, for the identical reason.
var (
	ErrMarkRiskReaderNil      = errors.New("oms: liq monitor has no mark-risk read port")
	ErrVenueMarginStatusNil   = errors.New("oms: liq monitor has no venue margin-status port")
	ErrMarkMaxAgeUnconfigured = errors.New("oms: no mark max-age for a venue the liq policy table covers")
	ErrMarkMaxAgeNotPositive  = errors.New("oms: mark max-age is not positive")
	ErrVenueUncalibrated      = errors.New("oms: liq monitor's venue clock is not calibrated")

	// ErrLiqReasonInvalid reports an assessment reason that is the zero value or
	// is not a declared member — a hand-built fixture, since every path in this
	// file sets one.
	ErrLiqReasonInvalid = errors.New("oms: liq assessment reason is not a declared value")

	// ErrLiqAssessmentInconsistent reports an assessment whose fields contradict
	// each other: a band beside an incomplete reason, a distance beside no band,
	// a computed distance for a position the venue does not hold.
	//
	// Every one of these is unreachable from Assess and expressible by a
	// literal, which is exactly the set a consumer's fixture would otherwise be
	// free to invent.
	ErrLiqAssessmentInconsistent = errors.New("oms: liq assessment fields contradict each other")
)

// LiqUnknownReason is why an assessment carries no distance.
//
// # It is a separate axis from the band, and collapsing them loses #209
//
// riskdomain.LiqBandUnknown says THAT no distance was computed. This says WHY,
// and the why decides what happens next: §7.1 gives a stale mark a documented
// escalation (REST failover, then `T_blind`'s pre-authorized de-risk), and task
// #209's ruling gives an unstated liquidation price none. Two states that are
// identical on the band axis and opposite on the action axis.
type LiqUnknownReason uint8

const (
	// LiqInputsUnassessed is the zero value and names no reason. It is what an
	// assessment nobody populated carries, and Validate refuses it: the
	// alternative is a struct that claims complete inputs by omission.
	LiqInputsUnassessed LiqUnknownReason = iota

	// LiqInputsComplete is the ordinary case — a distance WAS computed, and the
	// band is a real band. It implies the venue holds a position.
	LiqInputsComplete

	// LiqPositionFlat is the venue holding no position on this instrument.
	//
	// It withholds NOTHING, and that is the one ruling in this enum that is not
	// conservative-by-default. A flat instrument has no distance, so a monitor
	// that treated "no distance" uniformly would veto the opening — and the
	// evidence it wants (a position's liquidation price) is produced only BY
	// opening. That is a refuse-by-default gate whose evidence is a by-product
	// of the action it guards: the permitted region is unreachable from a flat
	// book, and the system would simply never trade, with every individual
	// refusal reading as the gate working.
	//
	// The entry gate for a flat instrument is §6 step 4's sizing — choose the
	// margin so the resulting distance clears the open threshold — which is a
	// computation over a HYPOTHETICAL position and is not this monitor's.
	LiqPositionFlat

	// LiqMarginUnreadable is the margin-status read failing or answering
	// something this system will not read.
	//
	// ports.VenueMarginStatus states the consequence in its own contract: an
	// unanswered read means "this position's liq distance is unknown", which is
	// the stale-input rung and blocks openings. It is emphatically NOT flat —
	// re-deriving an absence from an unanswered query is what §5.3 forbids, and
	// the cost of getting it wrong here is that the trigger stops watching a
	// live short.
	LiqMarginUnreadable

	// LiqMarkAbsent is no mark observed yet for this instrument.
	//
	// The ordinary state of a freshly booted process, and still a block: a
	// position cannot be monitored before the first mark arrives, and boot is
	// exactly when a process is most likely to try to open one.
	LiqMarkAbsent

	// LiqMarkStale is a mark past its §3.4 max-age, on either clock.
	//
	// This is the member §7.1 escalates on — the one with the REST failover and
	// `T_blind` behind it. B4 owns those rungs; what this slice guarantees is
	// that the state is REPORTED rather than folded into a healthy distance
	// computed from an old number.
	LiqMarkStale

	// LiqMarkUnreadable is the mark port refusing to date its own reading — the
	// clock inversion its contract describes, where the two instants an age is
	// computed from disagree about which came first.
	//
	// Its own member rather than LiqMarkStale because the operator action is
	// different in kind: a stale mark is a venue or network problem, and an
	// inverted one is a clock-calibration problem on this host.
	LiqMarkUnreadable

	// LiqLiquidationPriceUnstated is the venue holding a position and declining
	// to publish a liquidation price for it — TASK #209's ruling, and the ruling
	// is a SPLIT rather than a rung.
	//
	// It withholds openings, like every member above it. It does NOT escalate,
	// and the reason is that the escalation would be self-reinforcing rather
	// than that the state is benign: Bybit documents the absence as `liqPrice`
	// falling outside the symbol's price band, and §7.1's escalation
	// (`PackageDelever` toward reduced notional) RAISES liq headroom — which
	// pushes the liquidation price further outside that band, which makes the
	// venue withhold it again. A trigger whose own remedy strengthens it has no
	// fixed point except flat, so a rule escalating on this member would
	// de-risk every well-margined Bybit position to zero, one generation at a
	// time, with the safety system appearing to work throughout.
	//
	// What it is NOT, ever, is "far from liquidation". Bybit's reason really
	// does mean far — and acting on the reason requires the symbol's price BAND,
	// which converts the absence into a LOWER BOUND on the distance and is a
	// venue fact this system does not read today. Until it does, absence is
	// evidence for nothing: it never permits, and it never compels.
	LiqLiquidationPriceUnstated
)

// Validate refuses a reason that is the zero value or is not a declared member.
func (r LiqUnknownReason) Validate() error {
	switch r {
	case LiqInputsComplete, LiqPositionFlat, LiqMarginUnreadable,
		LiqMarkAbsent, LiqMarkStale, LiqMarkUnreadable, LiqLiquidationPriceUnstated:
		return nil
	case LiqInputsUnassessed:
		return fmt.Errorf("liq assessment reason is unset: %w", ErrLiqReasonInvalid)
	}
	return fmt.Errorf("liq assessment reason %d: %w", uint8(r), ErrLiqReasonInvalid)
}

// String renders the reason for operator-facing text and metric labels.
func (r LiqUnknownReason) String() string {
	switch r {
	case LiqInputsUnassessed:
		return "unassessed"
	case LiqInputsComplete:
		return "complete"
	case LiqPositionFlat:
		return "position-flat"
	case LiqMarginUnreadable:
		return "margin-unreadable"
	case LiqMarkAbsent:
		return "mark-absent"
	case LiqMarkStale:
		return "mark-stale"
	case LiqMarkUnreadable:
		return "mark-unreadable"
	case LiqLiquidationPriceUnstated:
		return "liq-price-unstated"
	}
	return fmt.Sprintf("LiqUnknownReason(%d)", uint8(r))
}

// LiqAssessment is one instrument's liq-distance state at one instant.
type LiqAssessment struct {
	// Instrument is the perpetual assessed.
	Instrument ids.VenueInstrument

	// Side is the direction the caller stated the position faces. Carried so a
	// distance that has been passed through a layer still names the premise it
	// was computed under.
	Side riskdomain.PositionSide

	// Held is whether the venue reports a position on this instrument.
	//
	// A false is a FACT the venue stated, never an inference from a failed read
	// — see LiqMarginUnreadable.
	Held bool

	// Band is where the distance sits on §5.2's ladder, or LiqBandUnknown.
	Band riskdomain.LiqBand

	// Distance is how far the mark must move to reach liquidation, as a
	// fraction of the mark. Meaningful only when Reason is LiqInputsComplete;
	// it may be negative, which is a position at or past liquidation.
	Distance fixedpoint.Rate

	// Reason says whether the inputs were complete and, if not, which one was
	// missing.
	Reason LiqUnknownReason

	// MarkAge and MarkDataAge are the two ages the mark port measured, kept so
	// an operator reading a LiqMarkStale can tell a dead transport from a
	// venue-side stall without a second query. Zero when no mark was read.
	MarkAge     time.Duration
	MarkDataAge time.Duration

	// OpeningWithheld is this monitor's VETO on new exposure at this instrument
	// — §7.1's "blocks openings".
	//
	// It is a veto and not a permission, and the asymmetry is the point. A false
	// means this monitor objects to nothing; it does not mean an opening is
	// authorized, which is §6 step 4's sizing question and several other gates'.
	// Written as a permission, the flat-instrument case would have to answer
	// "true" to avoid deadlocking the first trade — and a monitor that AUTHORIZES
	// openings on instruments it knows nothing about is the exact inversion of
	// what §7.1 asks for.
	OpeningWithheld bool
}

// Validate reports whether an assessment is internally consistent.
//
// It exists for the reason ports.MarginReading.Validate does: this struct is
// what B3 and B4 will branch on, and a fixture built by hand can express
// combinations no assessment path produces — a complete reason beside an unknown
// band, or a band beside a flat position — each of which would let a consumer's
// test pass against a state that cannot occur.
func (a LiqAssessment) Validate() error {
	if err := a.Instrument.Validate(); err != nil {
		return fmt.Errorf("liq assessment instrument: %w", err)
	}
	if err := a.Side.Validate(); err != nil {
		return fmt.Errorf("liq assessment %s: %w", a.Instrument.Symbol, err)
	}
	if err := a.Reason.Validate(); err != nil {
		return fmt.Errorf("liq assessment %s: %w", a.Instrument.Symbol, err)
	}
	if err := a.Band.Validate(); err != nil {
		return fmt.Errorf("liq assessment %s: %w", a.Instrument.Symbol, err)
	}

	complete := a.Reason == LiqInputsComplete
	if complete != (a.Band != riskdomain.LiqBandUnknown) {
		return fmt.Errorf("liq assessment %s: reason %s beside band %s: %w",
			a.Instrument.Symbol, a.Reason, a.Band, ErrLiqAssessmentInconsistent)
	}
	if complete && !a.Held {
		return fmt.Errorf("liq assessment %s: a distance was computed for a position the venue does not hold: %w",
			a.Instrument.Symbol, ErrLiqAssessmentInconsistent)
	}
	if a.Reason == LiqPositionFlat && a.Held {
		return fmt.Errorf("liq assessment %s: reported flat and held: %w",
			a.Instrument.Symbol, ErrLiqAssessmentInconsistent)
	}
	if !complete && a.Distance != (fixedpoint.Rate{}) {
		// The MarginReading rule one type over: a value beside a flag that says
		// it means nothing is a number a field-by-field copy will carry into a
		// consumer that lost the flag.
		return fmt.Errorf("liq assessment %s: reason %s beside a distance of %s: %w",
			a.Instrument.Symbol, a.Reason, a.Distance, ErrLiqAssessmentInconsistent)
	}
	return nil
}

// LiqMonitor is §7.1's liq-distance trigger.
type LiqMonitor struct {
	mark     ports.MarkRiskReader
	margin   ports.VenueMarginStatus
	policies riskdomain.LiqDistancePolicies
	maxAge   map[ids.Venue]time.Duration
	clk      clock.Clock
}

// NewLiqMonitor builds the trigger.
//
// # maxAge is per VENUE, and it is checked against the policy table
//
// §5.2 requires market-data freshness budgets "one per (data class, venue)", and
// this is the mark class's, held here rather than read from marketdata because
// §3.6 rule 1 forbids the import — the same reason MarkReading is an OMS type.
// Every venue named by the policy table must have one: a monitor that could
// compute a distance for an instrument but has no age to judge its mark against
// would silently skip the staleness rung for that venue alone, which is the
// hardest kind of gap to see because the other venue's rung keeps firing.
//
// `T_blind` and `mark_blind_action` are §7.1's other two budgets and are
// deliberately NOT taken here: they parameterize the ESCALATION, which this
// slice does not build, and a budget with no consumer is an unchecked claim
// (§5.2's own rule, learned from a spot baseline pinned on a spotless
// deployment).
//
// # The venue clock is checked at construction
//
// The data age is measured against ServerNow(venue) and an uncalibrated venue
// yields an unset ServerTime, whose zero instant would make every mark look
// impossibly old — or, past the port's inversion check, unreadable. Refused here,
// by name, rather than discovered as a permanent LiqMarkUnreadable on a venue
// that is in fact healthy.
func NewLiqMonitor(
	mark ports.MarkRiskReader,
	margin ports.VenueMarginStatus,
	policies riskdomain.LiqDistancePolicies,
	maxAge map[ids.Venue]time.Duration,
	clk clock.Clock,
) (*LiqMonitor, error) {
	if mark == nil {
		return nil, ErrMarkRiskReaderNil
	}
	if margin == nil {
		return nil, ErrVenueMarginStatusNil
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if err := policies.Validate(); err != nil {
		return nil, fmt.Errorf("oms liq monitor: %w", err)
	}

	for vi := range policies {
		age, ok := maxAge[vi.Venue]
		if !ok {
			return nil, fmt.Errorf("oms liq monitor: venue %s covers %s: %w",
				vi.Venue, vi.Symbol, ErrMarkMaxAgeUnconfigured)
		}
		if age <= 0 {
			return nil, fmt.Errorf("oms liq monitor: venue %s mark max-age %v: %w",
				vi.Venue, age, ErrMarkMaxAgeNotPositive)
		}
		if err := clk.ServerNow(vi.Venue).Validate(); err != nil {
			return nil, fmt.Errorf("oms liq monitor: venue %s: %w: %w", vi.Venue, ErrVenueUncalibrated, err)
		}
	}

	return &LiqMonitor{
		mark:     mark,
		margin:   margin,
		policies: maps.Clone(policies),
		maxAge:   maps.Clone(maxAge),
		clk:      clk,
	}, nil
}

// Assess reads both ports and places the position on §5.2's ladder.
//
// side is the direction the position faces and is a parameter rather than an
// assumption — see riskdomain.PositionSide. This system's perp leg is a short,
// and the premise is stated at the call site instead of being buried in the
// arithmetic.
//
// The error return means a WIRING defect (an unconfigured instrument, an unstated
// side). Everything a venue can do to this call comes back as an assessment.
func (m *LiqMonitor) Assess(vi ids.VenueInstrument, side riskdomain.PositionSide) (LiqAssessment, error) {
	if err := vi.Validate(); err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor: instrument: %w", err)
	}
	if err := side.Validate(); err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor %s: %w", vi.Symbol, err)
	}
	policy, err := m.policies.For(vi)
	if err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor: %w", err)
	}

	a := LiqAssessment{Instrument: vi, Side: side, Band: riskdomain.LiqBandUnknown}

	// The mark is read FIRST and unconditionally, including for a flat
	// instrument. §7.1's staleness rule blocks openings on the feed's health
	// alone — it does not ask whether a position exists — and the mark stream is
	// public, so unlike the liq price it is answerable for an instrument this
	// system holds nothing on. Reading it second would make the block
	// conditional on holding the very exposure it exists to prevent acquiring.
	reading, observed, err := m.mark.Mark(vi, m.clk.Now(), m.clk.ServerNow(vi.Venue).Instant())
	markReason, markOK := m.screenMark(vi, reading, observed, err)
	if markOK {
		a.MarkAge, a.MarkDataAge = reading.Age, reading.DataAge
	}

	status, held, err := m.margin.MarginStatus(vi)
	switch {
	case err != nil:
		// An unreadable margin answer outranks every mark verdict, because it
		// is the one that leaves this system unable to say whether it HOLDS
		// anything — and §5.3 forbids reading that silence as flat.
		a.Reason = LiqMarginUnreadable
		a.OpeningWithheld = true
		return a, nil
	case !held:
		a.Held = false
		// A flat instrument still takes the mark's verdict, which is what keeps
		// the staleness block from being conditional on exposure. When the mark
		// is healthy the reason is the flat one and nothing is withheld.
		if !markOK {
			a.Reason, a.OpeningWithheld = markReason, true
			return a, nil
		}
		a.Reason = LiqPositionFlat
		return a, nil
	}

	a.Held = true
	if !markOK {
		a.Reason, a.OpeningWithheld = markReason, true
		return a, nil
	}
	if !status.LiqPriceStated {
		// Task #209. The mark is healthy and the position is real; the venue
		// simply will not say where it liquidates. See
		// LiqLiquidationPriceUnstated for why this withholds without escalating.
		a.Reason, a.OpeningWithheld = LiqLiquidationPriceUnstated, true
		return a, nil
	}

	d, err := riskdomain.LiqDistance(side, reading.Price, status.LiqPrice)
	if err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor %s: %w", vi.Symbol, err)
	}
	band, err := policy.Band(d)
	if err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor %s: %w", vi.Symbol, err)
	}
	permitted, err := policy.PermitsOpening(d)
	if err != nil {
		return LiqAssessment{}, fmt.Errorf("oms liq monitor %s: %w", vi.Symbol, err)
	}

	a.Reason, a.Distance, a.Band, a.OpeningWithheld = LiqInputsComplete, d, band, !permitted
	return a, nil
}

// screenMark turns the mark port's three-way answer into a usable reading or the
// reason it is not one.
//
// Both ages are judged against the SAME budget and either one alone condemns the
// reading — marketdata applies one Budget to both clocks the same way. Two
// checks rather than one because a feed dies two ways and each clock sees only
// one of them: a venue-side stall keeps frames arriving on schedule with a
// frozen SourceTime, so the transport age reads perfectly fresh for the whole
// outage.
//
// The comparison is strict, matching marketdata's inclusive-fresh boundary: a
// reading exactly at the budget is still fresh, because a budget is a permission
// to trust up to and including its bound.
func (m *LiqMonitor) screenMark(vi ids.VenueInstrument, r ports.MarkReading, observed bool, err error) (LiqUnknownReason, bool) {
	switch {
	case err != nil:
		return LiqMarkUnreadable, false
	case !observed:
		return LiqMarkAbsent, false
	}
	budget := m.maxAge[vi.Venue]
	if r.Age > budget || r.DataAge > budget {
		return LiqMarkStale, false
	}
	return LiqInputsComplete, true
}
