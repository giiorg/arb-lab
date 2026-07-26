package ratelimit

import "fmt"

// Priority is what a request is FOR, and therefore which ceiling it may spend
// up to.
//
// Integer enum with a zero sentinel — §3.6 rule 8's documented exception, same
// as observability.Kind. It never crosses a wire or a config file; what crosses
// is the decision it produced.
//
// The three values are ordered, and the order is load-bearing: it is the
// sequence in which traffic is shed as a venue budget fills.
type Priority uint8

const (
	// PriorityUnspecified is the zero value and names no class. Unusable, and
	// with no default in either direction — see ErrPriorityInvalid.
	PriorityUnspecified Priority = iota

	// PriorityOpening is traffic that creates new exposure: a strategy entry,
	// the reads that price one. It is shed first, which is the same act §5.3
	// already names `stop-opening` — so a budget under pressure and a
	// stop-opening condition degrade the system the same way.
	PriorityOpening

	// PriorityRoutine is the traffic that keeps the risk view live: mark and
	// funding reads, account posture polls, position reconciliation.
	//
	// It outranks openings deliberately. Shedding monitoring to preserve
	// trading would leave the system trading on a view it has stopped
	// refreshing, and §7.1's stale-mark path treats exactly that as an
	// escalation, not as a saving.
	PriorityRoutine

	// PrioritySafety is traffic that reduces or resolves risk: §6.7's finality
	// resolution, §7.1's margin top-up and its REST mark failover, §7.5's
	// reconcile-only boot, a flatten.
	//
	// It spends the reserve — the band between RoutineCeiling and Capacity —
	// and it stops at Capacity like everything else. See the package doc for
	// why that is a ceiling rather than a bypass.
	PrioritySafety
)

const maxPriority = PrioritySafety

// Validate reports whether p names a class, excluding the sentinel.
func (p Priority) Validate() error {
	if p == PriorityUnspecified || p > maxPriority {
		return fmt.Errorf("priority %d: %w", uint8(p), ErrPriorityInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (p Priority) String() string {
	switch p {
	case PriorityUnspecified:
		return "PriorityUnspecified"
	case PriorityOpening:
		return "PriorityOpening"
	case PriorityRoutine:
		return "PriorityRoutine"
	case PrioritySafety:
		return "PrioritySafety"
	}
	return fmt.Sprintf("Priority(%d)", uint8(p))
}

// Dimension is what a limit counts.
//
// Three, because §7.6 names three distinct things a venue meters, and they are
// not interchangeable: an order costs both weight and an order slot, and a
// market-data read costs weight and no order slot at all.
type Dimension uint8

const (
	// DimensionUnspecified is the zero value and counts nothing.
	DimensionUnspecified Dimension = iota

	// DimensionWeight is the venue's own request-weight unit — Binance's
	// used-weight budget, Bybit's headered limit. Per-endpoint and unequal:
	// a deep order-book read weighs many times a ticker.
	DimensionWeight

	// DimensionOrders is the separate order-count budget. Binance meters it
	// over its own intervals, distinct from weight, so an account can be inside
	// its weight budget and out of order slots.
	DimensionOrders

	// DimensionRequests is a plain count of calls, which is what an IP-level
	// gateway ban actually meters — Bybit's ~600/5s (retCode 10018) is a
	// request rate with no weighting and no header.
	//
	// Every Acquire charges exactly one to every limit of this dimension, and
	// Cost has no field for it. That is deliberate: this is the dimension with
	// no header to correct a mistake, so it is also the one a caller must not
	// be able to forget.
	DimensionRequests
)

const maxDimension = DimensionRequests

// Validate reports whether d names a counted dimension, excluding the sentinel.
func (d Dimension) Validate() error {
	if d == DimensionUnspecified || d > maxDimension {
		return fmt.Errorf("dimension %d: %w", uint8(d), ErrDimensionInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (d Dimension) String() string {
	switch d {
	case DimensionUnspecified:
		return "DimensionUnspecified"
	case DimensionWeight:
		return "DimensionWeight"
	case DimensionOrders:
		return "DimensionOrders"
	case DimensionRequests:
		return "DimensionRequests"
	}
	return fmt.Sprintf("Dimension(%d)", uint8(d))
}

// Scope is what a limit is shared across.
//
// The two are §7.6's own distinction: "separate product weight/order counters"
// at family scope, and one "shared IP-ban safety envelope per venue" above them.
type Scope uint8

const (
	// ScopeUnspecified is the zero value and names no scope.
	ScopeUnspecified Scope = iota

	// ScopeVenue is shared by every API family at the venue. This is where the
	// IP-ban envelope lives, and it exists because IP-ban-track separation
	// between Binance spot and futures is inferred rather than documented
	// (§7.6, Profiles §2): maxing both product budgets independently could trip
	// a gateway limit neither of them reports on.
	ScopeVenue

	// ScopeFamily is one API family's own budget.
	ScopeFamily
)

const maxScope = ScopeFamily

// Validate reports whether s names a scope, excluding the sentinel.
func (s Scope) Validate() error {
	if s == ScopeUnspecified || s > maxScope {
		return fmt.Errorf("scope %d: %w", uint8(s), ErrScopeInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s Scope) String() string {
	switch s {
	case ScopeUnspecified:
		return "ScopeUnspecified"
	case ScopeVenue:
		return "ScopeVenue"
	case ScopeFamily:
		return "ScopeFamily"
	}
	return fmt.Sprintf("Scope(%d)", uint8(s))
}
