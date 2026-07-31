// Package domain holds the strategy context's value types: the vocabulary
// §3.3 names (MarketView, PositionView, Intent), §2.4's durable plan types
// (HoldPlan, SettlementForecast), and the pair that brackets the evaluation
// core — DecisionInput and Decision (design record 2026-07-31 §40).
//
// # The core is a function of ONE validated input
//
// §40b's shape: the cashcarry evaluation core (domain/cashcarry, slice C) is a
// pure function over a single validated DecisionInput, returning a Decision.
// Every term the verdict may depend on is a declared member of that input,
// because a core that reads ambient params, fetches mid-call, or keeps state
// across calls is not a function of its inputs — and every forgotten term
// reads as zero, and every zero flatters the kill gate.
//
// # The zero value is unusable, and absence is different from declared zero
//
// Every type here fails Validate when nobody populated it (§11's rule; an
// unpopulated cost reads as free). Two members of DecisionInput — the
// closing-cost delta and the risk delta — may legitimately BE zero (§2.4's
// "closing cost is branch-symmetric" common case), so for them the refusal is
// of absence, not of zero. fixedpoint already makes the two representable: a
// zero-value Money has no asset and fails Validate; ZeroMoney(asset) is a
// declared zero. No second mechanism is invented for it.
//
// # marketdata/domain is imported, not duplicated
//
// MarketView's funding block is marketdata's FundingObservation, and §40f
// names its Enumerate as "the only settlement source" — a second
// implementation over the same two fields is how two definitions of "the
// settlements in this hold" come to disagree. engine/state's doc records the
// governing principle: a duplicate type for one reading is a price paid only
// under compulsion (rule 1 compels venue vs marketdata; nothing compels here).
// §3.3's "MarketView is strategy-owned — NOT engine/state's type" is about the
// cache's types, and stays satisfied: the view, and every type an implementor
// of the read ports returns, is declared in this package.
package domain
