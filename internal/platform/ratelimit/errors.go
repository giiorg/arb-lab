package ratelimit

import "errors"

var (
	// ErrPriorityInvalid reports a Priority outside the declared ladder,
	// including the zero value.
	//
	// There is deliberately no default. Defaulting to PrioritySafety would let a
	// forgotten field bypass every ceiling in the package and earn the ban the
	// package exists to avoid; defaulting to PriorityOpening would let a
	// forgotten field get a flatten refused. Neither direction is safe, so the
	// refusal names the field instead.
	ErrPriorityInvalid = errors.New("ratelimit: priority invalid")

	// ErrDimensionInvalid reports a Dimension outside the declared set.
	ErrDimensionInvalid = errors.New("ratelimit: dimension invalid")

	// ErrScopeInvalid reports a Scope outside the declared set.
	ErrScopeInvalid = errors.New("ratelimit: scope invalid")

	// ErrLimitNameUnset reports a limit with no name. It is separate from
	// ErrLimitNameInvalid because the causes differ — nobody populated it
	// versus somebody built it wrong — and one sentinel covering both cannot
	// tell an operator which.
	ErrLimitNameUnset = errors.New("ratelimit: limit name unset")

	// ErrLimitNameInvalid reports a name that is too long or holds a character
	// outside the permitted set.
	ErrLimitNameInvalid = errors.New("ratelimit: limit name malformed")

	// ErrLimitInterval reports a window that is zero or negative. A zero
	// interval would evict every charge on the next call, so the budget would
	// read as permanently empty and the governor would admit everything.
	ErrLimitInterval = errors.New("ratelimit: limit interval invalid")

	// ErrLimitCeilings reports a ladder that is not a ladder: a zero capacity,
	// a zero opening ceiling, or a rung above the one that must contain it.
	ErrLimitCeilings = errors.New("ratelimit: limit ceilings invalid")

	// ErrLimitFamily reports a scope and a family that disagree — a
	// venue-scoped limit naming a family, or a family-scoped limit naming none.
	ErrLimitFamily = errors.New("ratelimit: limit scope and family disagree")

	// ErrLimitDuplicate reports two limits with the same key. The key is
	// (family, name), so the same name under two families is fine and the same
	// name twice under one is not: a silent winner between two budgets is a
	// budget nobody is enforcing.
	ErrLimitDuplicate = errors.New("ratelimit: limit declared twice")

	// ErrLimitUnknown reports an Observe or a query naming a limit the profile
	// does not declare.
	//
	// Refused rather than ignored: a reading for a limit nobody declared is a
	// header the adapter is parsing and the profile is missing, which is a
	// budget going entirely uncounted. Silently dropping it is the failure mode
	// that looks like everything working.
	ErrLimitUnknown = errors.New("ratelimit: no such limit")

	// ErrFamilyUnknown reports a request naming an API family the profile does
	// not declare.
	//
	// This is why Profile.Families exists at all. Without it a typo'd family
	// would simply match no family-scoped limit and sail past every product
	// budget, charging only the venue envelope — a request that looks governed
	// and is not.
	ErrFamilyUnknown = errors.New("ratelimit: no such API family")

	// ErrCostUnset reports a request that charges no weight. Every endpoint on
	// both venues weighs at least one, so a zero is a field somebody did not
	// populate, and an uncharged request is a bucket that never fills.
	ErrCostUnset = errors.New("ratelimit: cost unset")

	// ErrProfileEmpty reports a profile with no limits — a governor that admits
	// everything, which is worse than no governor because it looks like one.
	ErrProfileEmpty = errors.New("ratelimit: profile declares no limits")

	// ErrProfileFamilies reports a profile whose API family list is empty or
	// repeats itself.
	ErrProfileFamilies = errors.New("ratelimit: profile API families invalid")

	// ErrProfileFamilyUndeclared reports a family-scoped limit whose family is
	// not in Families. The two are declared separately so that a family with no
	// product budget of its own can still exist; that also means they can
	// disagree, and a limit attached to a family no request can name is a
	// budget that will never be consulted.
	ErrProfileFamilyUndeclared = errors.New("ratelimit: limit names an undeclared API family")

	// ErrPauseInvalid reports a pause schedule that cannot hold a venue off: a
	// non-positive throttle pause, or a ban pause shorter than it. §7.6 puts a
	// ban in a strictly more serious class than a 429, so a shorter ban pause is
	// a profile somebody transposed.
	ErrPauseInvalid = errors.New("ratelimit: pause schedule invalid")

	// ErrPenaltyInvalid reports a Penalty whose kind is outside the declared
	// set, including the zero value.
	ErrPenaltyInvalid = errors.New("ratelimit: penalty kind invalid")

	// ErrClockNil reports a governor constructed with no clock. §3.6 rule 5
	// forbids reading the ambient one, so there is no fallback to take.
	ErrClockNil = errors.New("ratelimit: clock is nil")
)
