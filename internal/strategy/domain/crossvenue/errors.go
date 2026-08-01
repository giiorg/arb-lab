package crossvenue

import "fmt"

// Sentinels. Every refusal names the rule that fired; the wrapping message
// carries the offending value.
var (
	// ErrRoundInvalid reports a pairing round that is not positive — a
	// zero round would bucket every instant alone and "no matches" would
	// read as a market fact instead of a caller bug.
	ErrRoundInvalid = fmt.Errorf("strategy/crossvenue: pairing round not positive")

	// ErrSeriesDisordered reports a settlement or decision series whose
	// instants decrease. The venues publish history ordered; disorder is a
	// walker bug, not venue dirt, and measuring over it would silently
	// scramble every windowed rule downstream.
	ErrSeriesDisordered = fmt.Errorf("strategy/crossvenue: series instants not ascending")

	// ErrDuplicateSettlement reports two settled prints on ONE leg at one
	// instant inside an accrual window. Pairing tolerates-and-counts that
	// dirt because the screening's committed rule does; the ledger refuses
	// it, because accrual truth with a duplicated settlement has no honest
	// reading — keeping either print, or both, invents money.
	ErrDuplicateSettlement = fmt.Errorf("strategy/crossvenue: duplicated settlement instant on one leg")

	// ErrNoPoints reports an empty decision series. A flat signal over real
	// points is honestly zero windows; NO points is the vacuous shape — a
	// wrong day or a dead stream must not read as "no episodes".
	ErrNoPoints = fmt.Errorf("strategy/crossvenue: no decision points to walk")

	// ErrEntryInvalid reports an entry level that is not positive — a zero
	// entry would open an episode on every print.
	ErrEntryInvalid = fmt.Errorf("strategy/crossvenue: entry level not positive")

	// ErrExitDivisorInvalid reports a hysteresis divisor below 1 — the exit
	// level would sit above the entry and every episode would close on its
	// first print.
	ErrExitDivisorInvalid = fmt.Errorf("strategy/crossvenue: exit divisor below 1")

	// ErrGapInvalid reports a max-gap bound that is not positive.
	ErrGapInvalid = fmt.Errorf("strategy/crossvenue: max gap not positive")

	// ErrWindowInvalid reports an episode window that closes before it
	// opens, or a direction that is not +1 or -1.
	ErrWindowInvalid = fmt.Errorf("strategy/crossvenue: episode window invalid")

	// ErrCostNegative reports a negative cost cell handed to the round-trip
	// assembly. A negative bound would CREDIT the trip — the flattering
	// direction; a market that measured negative is a governance question
	// at the procedure commit, never a discount here.
	ErrCostNegative = fmt.Errorf("strategy/crossvenue: negative cost cell in the round-trip assembly")

	// ErrPolicyInvalid reports an allocation policy that is empty, repeats
	// a symbol, carries a non-positive weight, or does not sum exactly to
	// one — a policy that under-allocates flatters the per-unit return and
	// one that over-allocates deploys capital nobody has.
	ErrPolicyInvalid = fmt.Errorf("strategy/crossvenue: allocation policy invalid")

	// ErrSeriesMisaligned reports per-symbol return series of differing
	// lengths, or a series set that does not exactly match the policy's
	// symbols — the portfolio combination is defined on one shared period
	// grid over exactly the declared holdings.
	ErrSeriesMisaligned = fmt.Errorf("strategy/crossvenue: per-symbol series misaligned with the policy")

	// ErrNoReturns reports an empty return series where a bound was to be
	// measured. An LCB of nothing is not a conservative zero — it is a
	// refusal, ErrNoResiduals' rule one package over.
	ErrNoReturns = fmt.Errorf("strategy/crossvenue: no returns to bound")

	// ErrBootstrapInvalid reports bootstrap parameters out of range: a mean
	// block below 1, a resample count below 1, or a confidence outside
	// (0,1) exclusive.
	ErrBootstrapInvalid = fmt.Errorf("strategy/crossvenue: bootstrap parameters out of range")

	// ErrDepthShort reports a book side too thin to fill the walked
	// candidate — the sample measures nothing, and a silent zero would put
	// the thinnest books at the flattering end of the distribution
	// (calibrate.ErrDepthShort's rule, this package's own name for it).
	ErrDepthShort = fmt.Errorf("strategy/crossvenue: book side too thin for the walked candidate")

	// ErrCandidateInvalid reports a walked candidate quantity that is not
	// strictly positive — a zero candidate walks nothing and reads as a
	// free fill.
	ErrCandidateInvalid = fmt.Errorf("strategy/crossvenue: walked candidate not positive")

	// ErrSideInvalid reports a book side token this package has no case
	// for.
	ErrSideInvalid = fmt.Errorf("strategy/crossvenue: book side invalid")
)
