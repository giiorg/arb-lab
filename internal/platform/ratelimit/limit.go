package ratelimit

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MaxLimitNameLen bounds a limit's name. Names are operator-facing — they
// appear in a refusal, in a Snapshot and in a §10 metric label — so the bound is
// the same order as observability's.
const MaxLimitNameLen = 64

// Limit is one budget: what it counts, over what window, and how much of it each
// priority may spend.
//
// The zero value is unusable, per the kernel-wide rule. An unpopulated Limit
// would declare a zero capacity over a zero window, which reads as a budget that
// is simultaneously always full and always empty — and the way that resolves in
// practice is "admits nothing", so a real budget would look like a permanent
// outage while a forgotten one looked fine.
type Limit struct {
	// Name identifies the limit within its scope, and is what Observe folds a
	// header reading into. It is ours to choose, not the venue's: the adapter
	// maps `X-MBX-USED-WEIGHT-1M` onto whatever this profile calls it.
	Name string

	// Scope and Family say what the budget is shared across. Family is required
	// for ScopeFamily and refused for ScopeVenue.
	Scope  Scope
	Family ids.ProductCategory

	// Dimension is what the limit counts.
	Dimension Dimension

	// Interval is the window, applied as a sliding one — see the package doc
	// for why that is deliberately stricter than the venue's own fixed window.
	Interval time.Duration

	// OpeningCeiling, RoutineCeiling and Capacity are the ladder, in the
	// limit's own units, and they must be non-decreasing in that order.
	//
	// Capacity is the venue's stated budget and is also PrioritySafety's
	// ceiling: no class is admitted past it. The band between RoutineCeiling
	// and Capacity is §7.1's reserved allocation — see Reserve.
	//
	// §7.6's "back off at ~70% weight" is an OpeningCeiling of ~70% of
	// Capacity, expressed as an absolute rather than a fraction. Absolute
	// because a fraction of a count is a division this package would have to
	// round, and every rounding decision in this repo is an explicit argument
	// (§3.6 rule 4); the profile that knows the capacity can do the arithmetic
	// once, in the open.
	OpeningCeiling uint64
	RoutineCeiling uint64
	Capacity       uint64
}

// Validate reports whether the limit is a usable budget.
//
// Checked in the order a reader debugs one: what it is called, what it is shared
// across, what it counts, over what window, and how much.
func (l Limit) Validate() error {
	if l.Name == "" {
		return ErrLimitNameUnset
	}
	if len(l.Name) > MaxLimitNameLen {
		return fmt.Errorf("limit %q is %d bytes, limit %d: %w",
			l.Name, len(l.Name), MaxLimitNameLen, ErrLimitNameInvalid)
	}
	if i := indexNotIdentifier(l.Name); i >= 0 {
		return fmt.Errorf("limit %q holds %q at byte %d: %w",
			l.Name, l.Name[i], i, ErrLimitNameInvalid)
	}
	if err := l.Scope.Validate(); err != nil {
		return fmt.Errorf("limit %q: %w", l.Name, err)
	}
	if err := l.validateFamily(); err != nil {
		return err
	}
	if err := l.Dimension.Validate(); err != nil {
		return fmt.Errorf("limit %q: %w", l.Name, err)
	}
	if l.Interval <= 0 {
		return fmt.Errorf("limit %q window %v: %w", l.Name, l.Interval, ErrLimitInterval)
	}
	return l.validateCeilings()
}

func (l Limit) validateFamily() error {
	switch {
	case l.Scope == ScopeVenue && l.Family != "":
		return fmt.Errorf("limit %q is venue-scoped but names family %q: %w",
			l.Name, l.Family, ErrLimitFamily)
	case l.Scope == ScopeFamily && l.Family == "":
		return fmt.Errorf("limit %q is family-scoped and names no family: %w",
			l.Name, ErrLimitFamily)
	case l.Scope == ScopeFamily:
		if err := l.Family.Validate(); err != nil {
			return fmt.Errorf("limit %q: %w", l.Name, err)
		}
	}
	return nil
}

// validateCeilings checks the ladder bottom-up, so the message names the rung
// that is wrong rather than the first comparison that happened to fail.
func (l Limit) validateCeilings() error {
	if l.Capacity == 0 {
		return fmt.Errorf("limit %q has zero capacity: %w", l.Name, ErrLimitCeilings)
	}
	if l.OpeningCeiling == 0 {
		return fmt.Errorf("limit %q has a zero opening ceiling, which admits no new exposure at all: %w",
			l.Name, ErrLimitCeilings)
	}
	if l.OpeningCeiling > l.RoutineCeiling {
		return fmt.Errorf("limit %q opening ceiling %d exceeds its routine ceiling %d: %w",
			l.Name, l.OpeningCeiling, l.RoutineCeiling, ErrLimitCeilings)
	}
	if l.RoutineCeiling > l.Capacity {
		return fmt.Errorf("limit %q routine ceiling %d exceeds its capacity %d: %w",
			l.Name, l.RoutineCeiling, l.Capacity, ErrLimitCeilings)
	}
	return nil
}

// Ceiling is how much of this budget p may spend.
//
// A priority outside the ladder gets zero, which admits nothing. Falling through
// to Capacity instead would make an unpopulated field the most privileged value
// in the package — the exact inversion ErrPriorityInvalid exists to prevent.
func (l Limit) Ceiling(p Priority) uint64 {
	switch p {
	case PriorityUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10). This is
		// the whole point of the doc comment above: the zero Priority admits
		// nothing, and the direction is chosen so that an unpopulated field is
		// the LEAST privileged value rather than the most.
		return 0
	case PriorityOpening:
		return l.OpeningCeiling
	case PriorityRoutine:
		return l.RoutineCeiling
	case PrioritySafety:
		return l.Capacity
	}
	return 0
}

// Reserve is the band only PrioritySafety may spend: §7.1's "reserved
// rate-limit allocation so it is not starved by trading traffic".
//
// Named rather than left as a subtraction, because it is the quantity an
// operator sizes: it has to cover §6.7's finality queries, §7.1's top-up and
// its REST mark polling, concurrently, during the incident that consumes them.
// Reserve is exported and callable on a Limit that never passed Validate, which
// is why the guard is here at all: without it a transposed ladder underflows to
// eighteen quintillion and the reserve reads as unlimited.
//
// Expected surviving mutant: `>=` narrowed to `>` is semantically equivalent.
// At exactly capacity the subtraction already yields zero, so the two forms
// differ nowhere. The guard's real content is the above-capacity case, and that
// is bite-checked by deleting the guard rather than by narrowing it.
func (l Limit) Reserve() uint64 {
	if l.RoutineCeiling >= l.Capacity {
		return 0
	}
	return l.Capacity - l.RoutineCeiling
}

// key is the identity two limits collide on. Family is empty for ScopeVenue, so
// a venue envelope and a family budget can share a name without sharing a
// bucket.
type limitKey struct {
	family ids.ProductCategory
	name   string
}

// key is taken AFTER Validate, which refuses a venue-scoped limit that names a
// family. So a venue-scoped key has an empty family by construction and needs no
// branch here — a `if l.Scope == ScopeVenue` special case was written first and
// deleted when a mutation of it survived, which is the surviving mutant doing
// its other job: reporting redundant code rather than a missing test.
func (l Limit) key() limitKey {
	return limitKey{family: l.Family, name: l.Name}
}

// Cost is what one request charges.
//
// There is no Requests field, and its absence is the design. Every Acquire
// charges exactly one request to every DimensionRequests limit, so the IP
// envelope cannot be under-charged by a caller who forgot — and that is the one
// budget with no header to correct the mistake afterwards.
type Cost struct {
	// Weight is the venue-documented weight of the endpoint being called.
	Weight uint64

	// Orders is 1 for a call that places or amends an order, 0 otherwise. A
	// batch endpoint charges the number of orders it carries.
	Orders uint64
}

// Validate reports whether the cost is populated.
//
// # The refusal is "charges nothing anywhere", not "weight is zero"
//
// It was the narrower rule first, on the reasoning that "every metered endpoint
// on both venues weighs at least one, so a zero is a field nobody filled in".
// That premise is FALSE, and the endpoint it is false about is the most
// important one in this system: Binance documents `POST /fapi/v1/order` as
// verbatim "1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S); 1 on 1min order
// rate limit(X-MBX-ORDER-COUNT-1M); 0 on IP rate limit(x-mbx-used-weight-1m)"
// (Profiles §7). A futures order really does weigh zero against the weight
// budget and one against each order budget, so the truthful Cost for placing an
// order was the one Validate refused.
//
// What survives is the argument, which was never about the weight field: a cost
// that charges NO dimension is a bucket that never fills, and that looks exactly
// like a system comfortably inside its budget right up to the ban. So the check
// is that some dimension is charged. Weight zero beside Orders one is a venue
// fact; Weight zero beside Orders zero is still the field nobody filled in.
//
// DimensionRequests is deliberately not counted as "charged". Every Acquire
// charges one request by construction (see units), so counting it would make
// this check vacuous — the zero Cost would pass on a dimension the caller cannot
// get wrong, and the one budget with no header to correct it afterwards is
// exactly the IP envelope. A check whose subject is the field nobody filled in
// must read the fields a caller fills in.
func (c Cost) Validate() error {
	if c.Weight == 0 && c.Orders == 0 {
		return fmt.Errorf("weight 0 and orders 0: %w", ErrCostUnset)
	}
	return nil
}

// units is what this cost charges to a limit of dimension d.
//
// An unrecognised dimension charges zero, and that is safe here only because
// Limit.Validate has already refused one: this is reached solely through a
// validated profile. It is not a silent default standing in for a check.
func (c Cost) units(d Dimension) uint64 {
	switch d {
	case DimensionUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10). Charging
		// zero is safe only because Limit.Validate refused this dimension
		// upstream, which the doc comment above says and the code now shows.
		return 0
	case DimensionWeight:
		return c.Weight
	case DimensionOrders:
		return c.Orders
	case DimensionRequests:
		return 1
	}
	return 0
}

// indexNotIdentifier returns the byte index of the first character outside the
// permitted set, or -1.
//
// An allow-list of lowercase, digits and underscore, matching observability's
// and for the same reason: a deny-list admits every character nobody thought to
// name, and these strings land in a metric label and a log line.
//
// Uppercase is excluded rather than folded. Two limits differing only in case
// are two budgets that read as one in every place an operator sees them, and the
// governor would enforce them separately while the dashboard showed one number.
func indexNotIdentifier(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '_':
			continue
		}
		return i
	}
	return -1
}
