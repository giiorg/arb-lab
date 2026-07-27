package state

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

// Skew is §3.4's max cross-leg skew: the widest gap between the instants this
// process took delivery of the observations a decision spans.
//
// # [SPEC EDIT OWED] It is measured over RecvTime, and the spec names both
// # timestamps without choosing
//
// §3.5 carries SourceTime and RecvTime "for freshness, cross-leg-skew metrics,
// and offline analysis only" — both of them, for that list of three. §3.4
// requires "a cross-leg skew limit" on sub-second book budgets. Neither says
// which timestamp the skew is computed from, and the two answers differ in a way
// that matters, because the one a reader reaches for first is the wrong one.
//
// SourceTime is the venue's own stamp, and §3.5 is explicit that the venues
// stamp with unsynchronized clocks: "sorting a merged stream by SourceTime can
// MANUFACTURE an incorrect order, and no clock-uncertainty proof justifies it".
// A cash-and-carry package is spot on one venue and the perp on another, so a
// SourceTime skew across its legs is precisely the unsynchronized-clock
// subtraction that sentence forbids — and it fails quietly, because the number
// it produces is the venues' clock OFFSET rather than any property of the
// market. Against a sub-second limit an offset of a few hundred milliseconds is
// indistinguishable from a genuinely skewed package, so the gate either blocks a
// healthy pair forever or is widened until it admits a real one.
//
// clock.ServerNow models that offset, so a corrected SourceTime skew is
// arithmetically available. It is still rejected: clock.ServerTime carries an
// uncertainty BAND, and §3.4 puts the skew limit in the same sentence as
// sub-second book budgets. A measurement whose error bar is the size of the
// threshold it feeds is not a measurement, and widening the limit to cover the
// band would spend the entire skew budget on not knowing what time it is.
//
// RecvTime is stamped by this process from the one clock both legs share — the
// injected one, which §3.6 rule 5 extends its ban into engine/ specifically to
// keep single. So the subtraction is exact, and it needs no clock reading at
// all: see below.
//
// The cost is stated rather than hidden. RecvTime skew measures when we LEARNED,
// not when the world WAS, so it absorbs any asymmetry in the two venues'
// transport latency. That is the correct quantity for the decision §3.4 attaches
// it to: what a marketable IOC is exposed to is its two legs' knowledge being
// drawn from different moments, and "how stale is each leg" is already measured
// in engine time by Watermark.Age. Measuring skew the same way makes the two
// consistent — see the identity below, which would not hold under any other
// choice.
//
// # It takes no clock, and that identity is worth naming
//
// Skew over RecvTime equals |Age(a) - Age(b)| for any single instant now, since
// the now cancels. So the quantity a caller would compute from freshness ages is
// the same quantity this returns, and it is a pure function of the watermarks —
// testable with no clock, and impossible to compute against two disagreeing
// clock readings, which is the defect rule 5 exists for.
//
// # Fewer than two observations is an ERROR, not a skew of zero
//
// Zero is the most permissive answer this function has: it says the legs are
// perfectly aligned. Returning it for a caller who supplied one leg, or none,
// makes "nothing was compared" and "everything agrees" the same answer — which
// is exactly the fold-identity trap domain.Least records on the freshness side,
// where seeding at FreshnessFresh would make "no legs were checked" read as "no
// leg is stale". A package with fewer than two legs is not a package, and a
// caller that reached here with one has a bug that a zero would conceal.
func Skew(ws ...domain.Watermark) (time.Duration, error) {
	if len(ws) < 2 {
		return 0, fmt.Errorf("%d observation(s): %w", len(ws), ErrSkewUndefined)
	}

	var earliest, latest time.Time
	for i, w := range ws {
		// Validate rather than trust: an unset RecvTime is the zero instant, and
		// a skew computed against it is about fifty-five years — a number no
		// limit admits, so an unpopulated leg would present as a permanently
		// skewed package rather than as the missing field it is. That is
		// Watermark.Validate's own argument, applied at the second place the
		// value is subtracted.
		if err := w.Validate(); err != nil {
			return 0, fmt.Errorf("observation %d: %w", i, err)
		}

		// Seeded from the first element rather than left to the zero instant.
		// Writing the max arm as `i == 0 || w.RecvTime.After(latest)` would be
		// correct only BECAUSE the zero instant precedes every real one, which
		// makes that arm unreachable — a branch that can never decide anything
		// is a branch no test can distinguish from its absence. Worse, it would
		// quietly rest on the assumption Watermark.Validate exists to refuse.
		if i == 0 {
			earliest, latest = w.RecvTime, w.RecvTime
			continue
		}
		if w.RecvTime.Before(earliest) {
			earliest = w.RecvTime
		}
		if w.RecvTime.After(latest) {
			latest = w.RecvTime
		}
	}

	// The widest gap between n instants is max - min: every pairwise difference
	// is bounded by it and this pair realises it. Written as one subtraction
	// rather than an O(n²) pairwise scan because the two are equal, and the
	// equality is a property of a total order rather than an optimisation.
	return latest.Sub(earliest), nil
}

// SkewLimit is §3.4's cross-leg skew limit for a decision class.
//
// A named type over time.Duration rather than a bare duration, for the reason
// ports.AcceptOrdinal gives for not being a uint64: the freshness budgets beside
// it in §5.2 are also durations and mean something else entirely. As bare
// durations a call site that swapped a MaxAge for a skew limit would compile and
// would gate on the wrong number — a book budget is per-leg tolerance, a skew
// limit is between-leg tolerance, and neither is a safe substitute for the
// other.
type SkewLimit time.Duration

// Validate reports whether the limit could describe a real gate.
//
// # Zero is refused, and it is the one somebody configures by accident
//
// A limit of zero admits only two legs delivered in the very same instant. Under
// clock.Wall that is unreachable in practice, so every package is refused and
// the engine simply never trades — with nothing reporting an error, because
// every individual refusal is the gate working. Under a TestClock it is
// reachable, so a backtest would pass a configuration that halts the live
// engine, which is the direction §3.5's parity claim exists to prevent.
//
// This is domain.Budget.Validate's unreachable-rung argument in the other
// direction: there a configuration made an escalation rung unreachable, here it
// makes the permitted region unreachable. Both are refused at startup, by name,
// rather than during the incident they degrade.
func (l SkewLimit) Validate() error {
	if l <= 0 {
		return fmt.Errorf("skew limit %v: %w", time.Duration(l), ErrSkewLimitInvalid)
	}
	return nil
}

// Admits reports whether a measured skew is inside the limit.
//
// The bound is INCLUSIVE, matching domain.Budget.At: "a budget is a permission
// to trust up to and including its bound, and the alternative makes a value that
// is precisely at the configured limit fail the limit — a distinction no
// operator reading 'max age 5 s' expects to be off by one nanosecond." The same
// operator reads "max skew 200 ms" the same way, and the two numbers sit in the
// same §5.2 section; an inclusive budget beside an exclusive limit is a
// difference nothing in the configuration would reveal.
//
// A negative skew is refused rather than admitted. Skew returns none — it is a
// max minus a min — so one reaching here came from a caller's own arithmetic,
// and a negative compares as smaller than every limit, which is the direction
// that reads as PERFECTLY ALIGNED. That is Watermark.Age's argument about
// negative ages, at the place the value is finally compared.
func (l SkewLimit) Admits(skew time.Duration) bool {
	return skew >= 0 && skew <= time.Duration(l)
}

// String renders the limit for diagnostics and error messages.
func (l SkewLimit) String() string { return time.Duration(l).String() }
