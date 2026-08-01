package calibrate

import (
	"fmt"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// Replay sentinels.
var (
	// ErrNeverClears reports a frame under which even the venue's funding
	// cap does not clear entry — a clearing level would have to be
	// invented above the realizable range.
	ErrNeverClears = fmt.Errorf("strategy/calibrate: entry never clears inside the venue clamps")

	// ErrCandidateMisaligned reports a declared candidate quantity the leg
	// quantization would move: the surrogate cost priced the candidate,
	// the exit direction scores the HELD quantity, and the two must be the
	// same number for the frame to be one trade.
	ErrCandidateMisaligned = fmt.Errorf("strategy/calibrate: candidate quantity not on the legs' lot grids")

	// ErrReplayRefused reports a replay evaluation refused for a reason
	// the funding series cannot cause — a frame defect (bad fees, stale
	// nothing, incoherent draw), never a market verdict. The replay stops
	// rather than counting a broken frame as idle time.
	ErrReplayRefused = fmt.Errorf("strategy/calibrate: replay refused on a frame-caused rule")

	// ErrGridBroken reports a settlement series that is not one uniform
	// interval grid — the replay's planned settlements would name instants
	// the era never realized.
	ErrGridBroken = fmt.Errorf("strategy/calibrate: settlement series not on the declared interval grid")
)

// flatEpoch anchors ClearingLevel's flat-rate world. Any valid instant
// gives the same level — every duration the verdict reads (hold fraction,
// plan span) is a difference the epoch cancels out of — so it is a fixed
// constant rather than an argument somebody could vary.
var flatEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// ClearingLevel is the entry-clearing level the band and occupancy
// derivations share (§41b round 3: same candidate quantity, same
// surrogate cost — one frozen definition): the smallest per-settlement
// rate, at the canonical scale, at which the DEPLOYED Enter answers
// VerdictEnter in the frame's flat-rate world — live estimate, trailing
// window and every forecast all at the candidate rate, the frozen
// dispersion shift applied exactly as the estimator applies it.
//
// The level is found by bisection over rate mantissas inside the venue
// clamps, which is exact because the verdict is monotone in the rate:
// funding income is the only term that moves, and it only grows. Both
// endpoints are checked first — a frame the cap cannot clear refuses
// (ErrNeverClears), and a frame the floor already clears answers the
// floor.
func ClearingLevel(sur Surrogate) (fixedpoint.Rate, error) {
	if err := sur.Validate(); err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("clearing level: %w", err)
	}
	enters := func(mantissa int64) (bool, error) {
		r := fixedpoint.RateFromMantissa(mantissa)
		d, err := flatEnter(sur, r)
		if err != nil {
			return false, err
		}
		if d.Verdict == domain.VerdictRefuse && !strings.HasPrefix(d.RefusedBy, cashcarry.RefuseMinFundingAPR) {
			return false, fmt.Errorf("rate %s refused by %q: %w", r, d.RefusedBy, ErrReplayRefused)
		}
		return d.Verdict == domain.VerdictEnter, nil
	}

	lo, hi := sur.Floor.Mantissa(), sur.Cap.Mantissa()
	atCap, err := enters(hi)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("clearing level: %w", err)
	}
	if !atCap {
		return fixedpoint.Rate{}, fmt.Errorf("cap %s: %w", sur.Cap, ErrNeverClears)
	}
	atFloor, err := enters(lo)
	if err != nil {
		return fixedpoint.Rate{}, fmt.Errorf("clearing level: %w", err)
	}
	if atFloor {
		return sur.Floor, nil
	}
	// Invariant: lo does not clear, hi does. Exact integer bisection.
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		up, err := enters(mid)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("clearing level: %w", err)
		}
		if up {
			hi = mid
		} else {
			lo = mid
		}
	}
	return fixedpoint.RateFromMantissa(hi), nil
}

// flatEnter scores one entry in the flat-rate world: one realized
// settlement at the rate, the live estimate at the rate, the minimum
// policy-admissible plan.
func flatEnter(sur Surrogate, r fixedpoint.Rate) (domain.Decision, error) {
	realized := []domain.RealizedSettlement{{
		Instrument: sur.Perp,
		At:         flatEpoch,
		Rate:       r,
		Watermark:  watermarkAt(flatEpoch),
	}}
	in, err := sur.entryInput(flatEpoch, r, realized)
	if err != nil {
		return domain.Decision{}, err
	}
	return cashcarry.Enter(in)
}

// OccupancyResult is the replay's answer with the trace the artifact's
// rationale cites.
type OccupancyResult struct {
	// Utilization is occupied over deployable, rounded DOWN (§41b) — the
	// direction that understates the expectation the scored run must
	// meet.
	Utilization fixedpoint.Rate

	// Occupied and Deployable are the two durations behind the ratio.
	Occupied, Deployable time.Duration

	// FirstEvaluated is the first instant the warmup admitted.
	FirstEvaluated time.Time

	// Entries, Closes and Evaluated count the replay's moves.
	Entries, Closes, Evaluated int
}

// Occupancy replays the DEPLOYED Enter/Continue state machine over the
// calibration era (§41b: occupancy, not eligibility — a few clearing
// windows can start multi-week holds, and many isolated ones can produce
// almost no occupancy, so no proxy over clearing fractions is even
// conservative) and freezes expected time-in-position over deployable
// time, rounded down.
//
// # The replay's conventions, each stated
//
//   - Evaluations happen at settlement instants, after that settlement
//     realizes; the live-estimate surrogate is the trailing EWMA over the
//     window ending at that instant (the refetchable history carries no
//     estimate stream, and the trailing forecast is the deployed
//     estimator's own later-settlement source).
//   - The warmup is the saturation rule Residuals uses: the first
//     evaluation waits until the window spans domain.EWMAHorizon, so the
//     replayed estimator is the steady-state one throughout.
//   - A close falls through to an entry evaluation at the SAME instant:
//     a hold whose carry stays good outlives its plan through a
//     seamless successor, which is how ProposeHoldPlan's minimum-hold
//     reading composes into long occupancies.
//   - A hold still open when the era ends contributes its span up to the
//     last settlement — era truncation, stated in the rationale; the
//     deployable denominator ends at the same instant.
//   - Entry refusals on the funding floor (min_funding_apr) are market
//     verdicts and idle time; every other refusal is a frame defect and
//     an error (ErrReplayRefused) — a broken frame must not measure as
//     an idle era.
func Occupancy(history []domain.RealizedSettlement, sur Surrogate) (OccupancyResult, error) {
	if err := sur.Validate(); err != nil {
		return OccupancyResult{}, fmt.Errorf("occupancy: %w", err)
	}
	if err := validateHistory(history); err != nil {
		return OccupancyResult{}, fmt.Errorf("occupancy: %w", err)
	}
	if history[0].Instrument != sur.Perp {
		return OccupancyResult{}, fmt.Errorf("history is %s, frame perp is %s: %w",
			history[0].Instrument, sur.Perp, domain.ErrLegsMismatch)
	}
	for i := 1; i < len(history); i++ {
		if history[i].At.Sub(history[i-1].At) != sur.Interval {
			return OccupancyResult{}, fmt.Errorf("rows %d..%d %v apart, interval %v: %w",
				i-1, i, history[i].At.Sub(history[i-1].At), sur.Interval, ErrGridBroken)
		}
	}

	horizon := domain.EWMAHorizon(sur.Params.EWMAHalfLife)
	first := -1
	for i := range history {
		if history[i].At.Sub(history[0].At) >= horizon {
			first = i
			break
		}
	}
	if first < 0 || first == len(history)-1 {
		return OccupancyResult{}, fmt.Errorf("%d rows, horizon %v: %w", len(history), horizon, ErrHistoryShort)
	}

	var (
		held     bool
		entryAt  time.Time
		plan     domain.HoldPlanRevision
		position domain.PositionView
		dwell    domain.DwellState
		occupied time.Duration
		res      OccupancyResult
	)
	j := 0 // trailing window start: rows older than the horizon weigh exactly zero
	for i := first; i < len(history); i++ {
		at := history[i].At
		for history[i].At.Sub(history[j].At) >= horizon {
			j++
		}
		window := history[j : i+1]
		estimate, err := domain.EWMA(window, sur.Params.EWMAHalfLife)
		if err != nil {
			return OccupancyResult{}, fmt.Errorf("occupancy estimate at %s: %w", at.Format(time.RFC3339), err)
		}
		res.Evaluated++

		if held {
			in, err := sur.exitInput(at, estimate, window, plan, position, dwell)
			if err != nil {
				return OccupancyResult{}, fmt.Errorf("occupancy exit at %s: %w", at.Format(time.RFC3339), err)
			}
			d, err := cashcarry.Continue(in)
			if err != nil {
				return OccupancyResult{}, fmt.Errorf("occupancy continue at %s: %w", at.Format(time.RFC3339), err)
			}
			switch d.Verdict {
			case domain.VerdictContinue:
				dwell = d.Terms.Dwell
				continue
			case domain.VerdictClose:
				occupied += at.Sub(entryAt)
				res.Closes++
				held = false
				// Fall through to the entry evaluation at this instant.
			case domain.VerdictRefuse, domain.VerdictEnter, domain.VerdictDecline:
				// A refusal here is a frame defect, and the entry verdicts
				// are ones Continue cannot lawfully emit — either way the
				// replay stops rather than absorbing it.
				return OccupancyResult{}, fmt.Errorf("exit at %s verdict %q (%s): %w",
					at.Format(time.RFC3339), d.Verdict, d.RefusedBy, ErrReplayRefused)
			}
		}

		in, err := sur.entryInput(at, estimate, window)
		if err != nil {
			return OccupancyResult{}, fmt.Errorf("occupancy entry at %s: %w", at.Format(time.RFC3339), err)
		}
		d, err := cashcarry.Enter(in)
		if err != nil {
			return OccupancyResult{}, fmt.Errorf("occupancy enter at %s: %w", at.Format(time.RFC3339), err)
		}
		switch d.Verdict {
		case domain.VerdictEnter:
			target, err := cashcarry.TargetQty(in)
			if err != nil {
				return OccupancyResult{}, fmt.Errorf("occupancy target at %s: %w", at.Format(time.RFC3339), err)
			}
			if target != sur.CandidateQty {
				return OccupancyResult{}, fmt.Errorf("target %s, candidate %s: %w",
					target, sur.CandidateQty, ErrCandidateMisaligned)
			}
			held = true
			entryAt = at
			plan = in.Plan
			position = domain.PositionView{
				Spot: sur.Spot, Perp: sur.Perp, Qty: sur.CandidateQty,
				EnteredAt: at, EntryPlan: in.Plan.Plan,
			}
			dwell = domain.DwellState{Declared: true}
			res.Entries++
		case domain.VerdictDecline:
			// Scored and found wanting: idle time.
		case domain.VerdictRefuse:
			if !strings.HasPrefix(d.RefusedBy, cashcarry.RefuseMinFundingAPR) {
				return OccupancyResult{}, fmt.Errorf("entry at %s refused by %q: %w",
					at.Format(time.RFC3339), d.RefusedBy, ErrReplayRefused)
			}
			// The funding floor is a market verdict: idle time.
		case domain.VerdictContinue, domain.VerdictClose:
			// Exit verdicts Enter cannot lawfully emit.
			return OccupancyResult{}, fmt.Errorf("entry at %s verdict %q: %w",
				at.Format(time.RFC3339), d.Verdict, ErrReplayRefused)
		}
	}

	last := history[len(history)-1].At
	if held {
		occupied += last.Sub(entryAt)
	}
	deployable := last.Sub(history[first].At)

	util, err := fixedpoint.RateFromMantissa(int64(occupied)).
		Div(fixedpoint.RateFromMantissa(int64(deployable)), fixedpoint.RoundFloor)
	if err != nil {
		return OccupancyResult{}, fmt.Errorf("occupancy ratio: %w", err)
	}
	res.Utilization = util
	res.Occupied = occupied
	res.Deployable = deployable
	res.FirstEvaluated = history[first].At
	return res, nil
}
