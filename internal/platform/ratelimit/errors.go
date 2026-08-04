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

	// ErrLimitFamily reports a scope and a family that disagree — a venue- or
	// endpoint-scoped limit naming a family, or a family-scoped limit naming
	// none.
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

	// ErrCostUnset reports a cost that charges nothing anywhere — weight and
	// orders both zero, with no RequestOnly declaration. An earlier wording here
	// argued "every endpoint weighs at least one", and that premise is FALSE
	// (Binance's futures order weighs zero — see Cost.Validate); what survives
	// is the argument that a cost charging no dimension is a bucket that never
	// fills. A call that truly charges only the implicit request says so via
	// RequestOnly, which is exactly the distinction this sentinel polices.
	ErrCostUnset = errors.New("ratelimit: cost unset")

	// ErrCostContradicted reports a RequestOnly cost that also charges weight
	// or orders. The two claims cannot both be true, and picking either
	// silently would count the wrong thing.
	ErrCostContradicted = errors.New("ratelimit: request-only cost still charges")

	// ErrCostUncounted reports a request whose cost charges a dimension no
	// applicable budget counts — §7.6's "a governor MUST refuse a cost it
	// cannot count", enforced at decide time. Profile.Validate makes this
	// unreachable except under a written Uncounted declaration, and then the
	// arriving cost is that declaration's falsification: the waiver claimed no
	// such cost existed, so the refusal stands regardless of the waiver.
	ErrCostUncounted = errors.New("ratelimit: cost charges a dimension no budget counts")

	// ErrLimitScopeDimension reports a venue-scoped limit counting anything but
	// requests. Venue scope is §7.6's shared envelope; weight and order budgets
	// are per-family AND headered, and the header fold addresses (family, name),
	// so a venue-scoped product budget would silently never receive the venue's
	// own reading.
	ErrLimitScopeDimension = errors.New("ratelimit: venue-scoped limit counts a headered dimension")

	// ErrLimitEndpointDimension reports an endpoint-scoped limit counting
	// weight. No supported venue meters weight per endpoint; refusing the shape
	// keeps it unavailable until a venue fact can define its semantics.
	ErrLimitEndpointDimension = errors.New("ratelimit: endpoint-scoped limit counts weight")

	// ErrEndpointUnbudgeted reports a request naming an endpoint for which the
	// profile declares no endpoint-scoped budget. This is a configuration
	// disagreement, not a temporary exhaustion, so waiting cannot repair it.
	ErrEndpointUnbudgeted = errors.New("ratelimit: endpoint has no budget")

	// ErrEndpointUnnamed reports an unnamed request against a profile that uses
	// endpoint budgets. Admitting it would spend a venue-side endpoint meter
	// without any local counter knowing which one to charge.
	ErrEndpointUnnamed = errors.New("ratelimit: request names no endpoint")

	// ErrCostEndpointUncounted reports a matched endpoint budget that receives
	// zero units from the request cost. A zero charge is always admitted, so the
	// declared budget could otherwise never refuse the call it purports to meter.
	ErrCostEndpointUncounted = errors.New("ratelimit: endpoint budget cannot count request cost")

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

	// ErrProfileEnvelopeMissing reports a profile with no venue-scoped request
	// limit. §7.6's IP-ban safety envelope is SHARED per venue, and that word is
	// the requirement: family-scoped request counters can each be inside their
	// own capacity while their sum trips the gateway ban neither of them
	// reports on, so they are additive detail, never the envelope. The
	// envelope's capacity is a locally chosen safety bound ("bound combined
	// request rate"), not a venue-published fact — so "the venue documents no
	// request limit" is a reason to choose a bound, never a reason to have
	// none. This is also why there is no Uncounted waiver for requests.
	ErrProfileEnvelopeMissing = errors.New("ratelimit: profile declares no venue request envelope")

	// ErrProfileDimensionUncounted reports a family with no budget for a
	// dimension a cost can charge, and no written Uncounted declaration. §7.6:
	// a dimension the profile declares no budget for is silently uncounted, and
	// a governor that counts nothing admits everything — the worked example is
	// Binance's futures order, documented at ZERO weight, riding entirely on
	// the order-count dimension.
	ErrProfileDimensionUncounted = errors.New("ratelimit: profile counts nothing for a chargeable dimension")

	// ErrProfileUncountedInvalid reports an Uncounted declaration that cannot
	// mean anything: an undeclared family, an invalid dimension, the requests
	// dimension (see ErrProfileEnvelopeMissing), an empty reason, or a
	// duplicate. Refused rather than skipped for the same reason an
	// unrecognised config tag is: here the safe direction and the silent
	// direction are not the same one.
	ErrProfileUncountedInvalid = errors.New("ratelimit: uncounted declaration invalid")

	// ErrProfileUncountedContradicted reports an Uncounted declaration for a
	// (family, dimension) a declared budget already counts. The two cannot both
	// be meant: a budget that arrived after the waiver makes the waiver a stale
	// claim that would quietly survive the budget's later removal, so the
	// contradiction is refused while somebody is looking at the file.
	ErrProfileUncountedContradicted = errors.New("ratelimit: uncounted declaration contradicts a declared budget")

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

	// ErrReserveEmpty reports a package reservation with no requests. Such a
	// hold cannot protect a send and would make a successful zero value look
	// meaningful.
	ErrReserveEmpty = errors.New("ratelimit: reserve batch is empty")

	// ErrReservePriorityMixed reports requests from one package that name
	// different priorities. A package is one command, so choosing a ceiling per
	// window would make its urgency depend on iteration order.
	ErrReservePriorityMixed = errors.New("ratelimit: reserve priorities differ")

	// ErrReserveOverflow reports a batch whose per-window total cannot be stated
	// in uint64. Saturating would make the summary disagree with both its entries
	// and the occupancy later released from them.
	ErrReserveOverflow = errors.New("ratelimit: reserve cost overflows")

	// ErrHoldEntryUnmatched reports a request that has no still-reserved entry
	// in the hold. Wrong, consumed, and canceled requests all fail closed at the
	// last seam before a send.
	ErrHoldEntryUnmatched = errors.New("ratelimit: hold entry unmatched")
)
