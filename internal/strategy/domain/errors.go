package domain

import "fmt"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract. Each names the rule that refused, because
// §40f makes "the refusing rule named" a property of the journaled record —
// an error a Decision cannot cite by name is an error the journal renders as
// "something failed".
var (
	// ErrUnset reports a required member nobody populated. The wrapping
	// message names the field; the sentinel is one value because callers act
	// on "the input is incomplete", not on which member first tripped.
	ErrUnset = fmt.Errorf("strategy: required member unset")

	// ErrSideInvalid reports a leg direction outside the declared set.
	ErrSideInvalid = fmt.Errorf("strategy: invalid side")

	// ErrKindInvalid reports an IntentKind, DecisionKind or Verdict outside
	// its declared set.
	ErrKindInvalid = fmt.Errorf("strategy: invalid kind")

	// ErrVerdictKindMismatch reports a Verdict that cannot arise from its
	// Decision's Kind — an entry decision concluding "close", or an exit
	// concluding "enter". The pair would journal cleanly and recompute as
	// whichever branch a reader assumed.
	ErrVerdictKindMismatch = fmt.Errorf("strategy: verdict disagrees with decision kind")

	// ErrLegsMismatch reports two values that must describe the same
	// instrument pair and do not — fees for a leg the view does not hold,
	// a funding block for a different perp than the view's.
	ErrLegsMismatch = fmt.Errorf("strategy: legs disagree about the instrument")

	// ErrProductWrong reports a leg whose product family is not the one the
	// field requires — a spot instrument in a perp-only position, a perp in
	// the spot leg.
	ErrProductWrong = fmt.Errorf("strategy: wrong product family for this leg")

	// ErrNotAscending reports a schedule or window whose instants are not in
	// strictly ascending order. Ties are refused too: two settlements at one
	// instant is a venue impossibility, and admitting one would double-count
	// a lump of funding.
	ErrNotAscending = fmt.Errorf("strategy: instants not strictly ascending")

	// ErrScheduleOverrun reports a plan settlement after the plan's terminal
	// time. §40f regenerates schedules "to the unchanged terminal time", so a
	// settlement beyond it is a regeneration that silently lengthened the
	// hold.
	ErrScheduleOverrun = fmt.Errorf("strategy: settlement after terminal time")

	// ErrForecastHorizon reports forecasts that do not cover exactly the
	// plan's forecastable settlements — §2.4's horizon rule ("the settlements
	// enumerated in the planned hold"), violated in either direction: a
	// missing settlement understates nothing visible, an extra one invents a
	// lump of carry.
	ErrForecastHorizon = fmt.Errorf("strategy: forecasts disagree with the plan's settlements")

	// ErrQuantileRange reports a quantile outside (0,1).
	ErrQuantileRange = fmt.Errorf("strategy: quantile outside (0,1)")

	// ErrCrossedTop reports a price view whose bid is above its ask. A
	// reconstructed book can be transiently crossed mid-update; a DECISION
	// priced against a crossed top is priced against data mid-flight.
	ErrCrossedTop = fmt.Errorf("strategy: bid above ask")

	// ErrAssetMixed reports a capital decomposition whose terms carry
	// different assets. Summing them is what fixedpoint exists to refuse; the
	// decomposition refuses earlier, at construction, where the field is
	// nameable.
	ErrAssetMixed = fmt.Errorf("strategy: capital terms in different assets")

	// ErrRevisionSequence reports a HoldPlanRevision whose Num and Trigger
	// disagree: revision 1 is the entry plan and only revision 1 may carry
	// TriggerEntry. A later "entry" is a regeneration wearing the entry's
	// label, and the dwell rule ("counts realized settlements, never schedule
	// entries") depends on the two being distinguishable.
	ErrRevisionSequence = fmt.Errorf("strategy: revision number disagrees with trigger")

	// ErrDwellUndeclared reports a dwell state nobody fed back. The zero
	// count is a legitimate value (a position before its first adverse
	// settlement), so absence needs its own bit; see DwellState.
	ErrDwellUndeclared = fmt.Errorf("strategy: dwell state undeclared")

	// ErrDwellInconsistent reports a dwell whose count and last-counted
	// instant disagree — a count with no instant, or an instant with a zero
	// count.
	ErrDwellInconsistent = fmt.Errorf("strategy: dwell count disagrees with last-counted instant")

	// ErrRefusalUnnamed reports a refusal Decision that does not name the
	// refusing rule. §40f: "a gate that only journals its approvals cannot
	// show it ever refused anything" — and a refusal with no rule journals an
	// approval-shaped hole.
	ErrRefusalUnnamed = fmt.Errorf("strategy: refusal names no rule")

	// ErrRefusalMisplaced reports a refusing rule on a Decision whose verdict
	// is not a refusal — evidence of a refusal the verdict contradicts.
	ErrRefusalMisplaced = fmt.Errorf("strategy: refusing rule on a non-refusal verdict")
)
