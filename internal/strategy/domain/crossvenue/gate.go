package crossvenue

// The gate's two admission rules (§45f-1 and §45n-3), as pure judgements.
// Both answer the same question — is THIS symbol's evidence fit to score? —
// and both must run BEFORE any portfolio statistic exists, because a
// portfolio is the one place a thin or suspect symbol stops being visible:
// once eight series are weighted and summed, a symbol scored on a quarter
// of its episodes reads exactly like a symbol scored on all of them.
//
// # Why "by name" is load-bearing in both
//
// The §41 lesson these inherit is the PromotionIdentityDoc one: a scoring
// trigger nobody can compute is a scoring trigger the operator declares,
// and an operator who declares it while holding the numbers has tuned the
// gate. So neither rule guesses and neither rule skips — a symbol that
// fails is REFUSED with its own counts in the message, and the refusal is
// what the artifact records against that symbol.
//
// # Exact integers, no rounding mode to choose wrong
//
// The ceiling compares a count ratio against a declared rate by
// cross-multiplication at the mantissa scale, not by dividing into a Rate
// first. Dividing would force a rounding decision exactly at the boundary
// the rule is about — round the measured fraction up and a symbol sitting
// EXACTLY at the declared ceiling refuses; round it down and one strictly
// above it passes. The cross-multiplied comparison has neither failure
// mode (StationaryBootstrapLCB's rank arithmetic, same reason).

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Gate sentinels.
var (
	// ErrEvidenceShort is ruling §45f-1's refusal: a shortlisted symbol
	// whose post-cutoff matched-settlement count has not reached the
	// declared floor. It is a refusal rather than an exclusion because the
	// gate scores a PRE-DECLARED portfolio (ruling §45f-3) — silently
	// dropping a short symbol would score a different portfolio from the
	// one the procedure commit declared, and nothing downstream would say
	// so.
	ErrEvidenceShort = fmt.Errorf("strategy/crossvenue: symbol below the declared matched-settlement floor")

	// ErrRefusalCeiling is ruling §45n-3's refusal: more of a symbol's
	// episodes were refused than the declared fraction admits. The
	// surviving episodes are not a random sample of the symbol's episodes
	// — a boundary book goes missing during stalls and reconnects, which
	// are the same instants where crossing costs most — so a symbol scored
	// on a heavily-thinned set reads as clean evidence while measuring the
	// calm subset.
	ErrRefusalCeiling = fmt.Errorf("strategy/crossvenue: refused episodes exceed the declared ceiling")

	// ErrGateBoundInvalid reports a floor or ceiling the gate cannot apply:
	// a floor below one (which would admit a symbol with no evidence at
	// all), a ceiling outside [0,1], or counts that cannot describe a walk
	// (negative, or more refusals than episodes).
	ErrGateBoundInvalid = fmt.Errorf("strategy/crossvenue: gate bound or counts out of range")
)

// CheckEvidenceFloor is ruling §45f-1's end rule: a shortlisted symbol
// qualifies for scoring once its post-cutoff MATCHED settlement count
// reaches the floor — matched, not elapsed, so a capture outage cannot
// leave a symbol thin while the calendar says go. The boundary is
// inclusive: a symbol exactly AT the floor has the evidence the floor
// asks for.
//
// The count is the symbol's own; the floor is the procedure commit's.
func CheckEvidenceFloor(symbol string, matched, floor int) error {
	if symbol == "" {
		return fmt.Errorf("unnamed symbol: %w", ErrGateBoundInvalid)
	}
	if floor < 1 {
		return fmt.Errorf("%s: floor %d: %w", symbol, floor, ErrGateBoundInvalid)
	}
	if matched < 0 {
		return fmt.Errorf("%s: matched %d: %w", symbol, matched, ErrGateBoundInvalid)
	}
	if matched < floor {
		return fmt.Errorf("%s: %d matched settlements, floor %d: %w",
			symbol, matched, floor, ErrEvidenceShort)
	}
	return nil
}

// CheckRefusalCeiling is ruling §45n-3's ceiling: a symbol whose refused
// episodes EXCEED the declared fraction of its episodes is refused by name
// and leaves the portfolio before any statistic exists. Refused exactly AT
// the ceiling is admitted — the rule refuses what exceeds the declared
// bound, and a bound that also refuses its own value is a different bound.
//
// A symbol with NO episodes passes vacuously. That is deliberate and it is
// not the same judgement as the evidence floor: zero episodes means the
// signal never crossed the entry level, which is the strategy correctly
// standing down, and its all-zero return series is the honest contribution
// (the utilization honesty, §42e). Whether the symbol has enough evidence
// to have stood down MEANINGFULLY is CheckEvidenceFloor's question, asked
// of settlements rather than episodes.
func CheckRefusalCeiling(symbol string, refused, episodes int, ceiling fixedpoint.Rate) error {
	if symbol == "" {
		return fmt.Errorf("unnamed symbol: %w", ErrGateBoundInvalid)
	}
	if err := ceiling.Validate(); err != nil {
		return fmt.Errorf("%s: ceiling: %w", symbol, err)
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		panic(err) // programmer constant
	}
	if s, err := ceiling.Sign(); err != nil {
		return fmt.Errorf("%s: ceiling: %w", symbol, err)
	} else if s < 0 {
		return fmt.Errorf("%s: ceiling %s: %w", symbol, ceiling.Decimal(), ErrGateBoundInvalid)
	}
	if c, err := ceiling.Cmp(one); err != nil {
		return fmt.Errorf("%s: ceiling: %w", symbol, err)
	} else if c >= 0 {
		// The admissible range is [0,1) — one EXCLUSIVE. A ceiling at or
		// above one can never fire: a symbol whose every episode was
		// refused sits at exactly one, so nothing exceeds the bound and
		// every symbol passes. A rule that cannot refuse is not a rule, and
		// it fails silently, so the bound itself is the error. Zero is the
		// other end and is admissible for the opposite reason — it CAN
		// fire, on the first refusal.
		return fmt.Errorf("%s: ceiling %s: %w", symbol, ceiling.Decimal(), ErrGateBoundInvalid)
	}
	if refused < 0 || episodes < 0 || refused > episodes {
		return fmt.Errorf("%s: %d refused of %d episodes: %w",
			symbol, refused, episodes, ErrGateBoundInvalid)
	}
	// refused/episodes > ceiling  <=>  refused × 1e9 > ceilingMantissa × episodes.
	// Both sides are exact int64: refused rides an episode count, and the
	// mantissa scale is 1e9 (StationaryBootstrapLCB's own literal, the same
	// scale spelled the same way), so the products stay far inside the
	// range for any walk a capture archive can hold.
	//
	// The zero-episode case needs no branch of its own, and deliberately
	// does not have one: the counts rule above forces refused to zero when
	// episodes is zero, so both sides are zero, zero does not EXCEED zero,
	// and the vacuous pass falls out of the same comparison as every other
	// answer. An `episodes == 0` early return was written here first and
	// removed when a mutation showed it could not fail — a branch whose
	// deletion changes no answer is not a guard, it is a second statement
	// of the rule that can drift from the first.
	lhs := int64(refused) * 1_000_000_000
	rhs := ceiling.Mantissa() * int64(episodes)
	if lhs > rhs {
		return fmt.Errorf("%s: %d of %d episodes refused, ceiling %s: %w",
			symbol, refused, episodes, ceiling.Decimal(), ErrRefusalCeiling)
	}
	return nil
}
