package domain

import (
	"fmt"
	"time"
)

// Freshness is how far an observation has aged past the budgets build-spec §5.2
// configures for it.
//
// Integer enum with a zero sentinel — §3.6 rule 8's documented exception, the
// same one boot.Posture and observability.Kind take. It never crosses a wire;
// what crosses is the snapshot and its watermark, from which any consumer
// recomputes this against its own budget.
//
// # The values are ORDERED by trust, and the zero value is the least trusted
//
// This is boot.Posture's shape, deliberately: a comparison is a complete answer
// (FreshnessFresh >= FreshnessStale), combining across legs is a minimum, and a
// Freshness nobody populated is the most adverse rather than the most
// permissive. The forgotten-field failure is therefore a de-lever, not a trade
// on a mark from an hour ago.
//
// # Unknown and Blind are separate on purpose, though they act alike
//
// Both are past every budget and both trigger §5.2's mark_blind_action. They are
// still different DIAGNOSES: Blind means an observation exists and has aged out,
// Unknown means there has never been one. Collapsing them would report a venue
// that has never connected identically to one whose feed died mid-position, and
// the operator response differs. §7.5's latching rule makes the same
// distinction from the other side — "unreported" is not "failed".
type Freshness uint8

const (
	// FreshnessUnknown is the zero value: nothing has been observed, or the
	// observation's provenance is broken (see Assess and ErrClockInversion).
	// Least trusted of all, and deliberately not aliased to FreshnessBlind.
	FreshnessUnknown Freshness = iota

	// FreshnessBlind is past the T_blind budget (§5.2, §7.1). The position is
	// being managed on a mark old enough that liq-distance is a guess, and
	// mark_blind_action (delever | close) applies.
	FreshnessBlind

	// FreshnessStale is past the max-age budget and inside T_blind: the rung
	// that exists so an operator hears about a degrading feed BEFORE the
	// automated de-lever fires. Budget.Validate refuses a configuration in
	// which it is unreachable.
	FreshnessStale

	// FreshnessFresh is inside the max-age budget. The only value that admits
	// opening new exposure.
	FreshnessFresh
)

const maxFreshness = FreshnessFresh

// Validate reports whether f is a declared value.
func (f Freshness) Validate() error {
	if f > maxFreshness {
		return fmt.Errorf("freshness %d: %w", uint8(f), ErrFreshnessInvalid)
	}
	return nil
}

// String names the value, for diagnostics and metric labels.
func (f Freshness) String() string {
	switch f {
	case FreshnessUnknown:
		return "FreshnessUnknown"
	case FreshnessBlind:
		return "FreshnessBlind"
	case FreshnessStale:
		return "FreshnessStale"
	case FreshnessFresh:
		return "FreshnessFresh"
	}
	return fmt.Sprintf("Freshness(%d)", uint8(f))
}

// AtLeast reports whether f is at least as trusted as want.
//
// A single >= is a complete answer only because the values are totally ordered
// by trust; that ordering is the type's whole semantics, not an incidental
// property of the iota order.
func (f Freshness) AtLeast(want Freshness) bool {
	return f >= want
}

// Least returns the least-trusted of the freshnesses given.
//
// The identity is FreshnessUnknown, so Least of nothing is the most adverse
// answer. That half is load-bearing and it is the same argument boot.Weakest
// makes: a caller reducing over an empty set of legs has observed nothing, and
// seeding the fold at FreshnessFresh — which is what writing a minimum
// naturally produces — would make "no legs were checked" and "every leg is
// fresh" the same answer.
//
// This is the cross-leg rule §3.4 asks for: a package is exactly as tradeable
// as its worst-informed leg.
func Least(fs ...Freshness) Freshness {
	least := FreshnessUnknown
	for i, f := range fs {
		if i == 0 || f < least {
			least = f
		}
	}
	return least
}

// Budget is §5.2's stale-mark escalation policy for one data class at one
// venue: the max-age that makes an observation stale, and the T_blind that
// makes it blind.
//
// §3.4 is explicit that freshness is per-data-class and NOT one 30 s threshold
// — an order book used for execution needs a sub-second budget while funding
// metadata tolerates longer — so a Budget is a value the caller holds per
// class, never a package-level constant.
type Budget struct {
	// MaxAge is the age past which an observation is FreshnessStale.
	MaxAge time.Duration

	// Blind is the age past which it is FreshnessBlind, firing §5.2's
	// mark_blind_action.
	Blind time.Duration
}

// Validate reports whether the budget could describe a real escalation.
//
// # MaxAge must be STRICTLY less than Blind, and the reason is the same one
// § 7.4 gives for the shutdown budget
//
// At MaxAge == Blind the FreshnessStale rung is unreachable: every observation
// that ages out of Fresh lands directly on Blind, so the warning that exists to
// precede the automated de-lever never fires and the first symptom an operator
// sees IS the de-lever. Nothing fails; the ladder just silently has two rungs
// where it is configured to have three. That is §7.4's total-stop-budget bound
// wearing different clothes — two durations in one config section that nothing
// was comparing — and it is refused here for the same reason: at startup, by
// name, rather than during the incident it degrades.
func (b Budget) Validate() error {
	if b.MaxAge <= 0 {
		return fmt.Errorf("max age %v: %w", b.MaxAge, ErrBudgetInvalid)
	}
	if b.Blind <= 0 {
		return fmt.Errorf("blind %v: %w", b.Blind, ErrBudgetInvalid)
	}
	if b.MaxAge >= b.Blind {
		return fmt.Errorf("max age %v not below blind %v: %w", b.MaxAge, b.Blind, ErrBudgetUnreachable)
	}
	return nil
}

// At classifies an age against the budget.
//
// The boundaries are inclusive-fresh: an observation exactly at MaxAge is still
// FreshnessFresh, and one exactly at Blind is still FreshnessStale. A budget is
// a permission to trust up to and including its bound, and the alternative
// makes a value that is precisely at the configured limit fail the limit — a
// distinction no operator reading "max age 5 s" expects to be off by one
// nanosecond.
//
// A negative age is FreshnessUnknown, not fresh: see Assess.
func (b Budget) At(a time.Duration) Freshness {
	switch {
	case a < 0:
		return FreshnessUnknown
	case a <= b.MaxAge:
		return FreshnessFresh
	case a <= b.Blind:
		return FreshnessStale
	default:
		return FreshnessBlind
	}
}

// Assess classifies a watermark's transport age at the instant now.
//
// # It returns a verdict AND an error, and both are meant
//
// On a clock inversion (§3.6 rule 5's two-disagreeing-clocks defect) the verdict
// is FreshnessUnknown — the safe direction — and ErrClockInversion is returned
// alongside it. A caller that ignores the error still de-levers rather than
// trading on a broken clock; a caller that reads it gets the diagnosis instead
// of an unexplained blind feed. Returning only the error would force every call
// site to invent a fallback verdict, and every site that forgot would invent
// the zero value of whatever variable it declared.
func (b Budget) Assess(w Watermark, now time.Time) (Freshness, error) {
	a, err := w.Age(now)
	if err != nil {
		return FreshnessUnknown, err
	}
	return b.At(a), nil
}

// AssessData is Assess against the venue's own stamp, using that venue's server
// time. See Watermark.DataAge for why the two are separate questions.
func (b Budget) AssessData(w Watermark, serverNow time.Time) (Freshness, error) {
	a, err := w.DataAge(serverNow)
	if err != nil {
		return FreshnessUnknown, err
	}
	return b.At(a), nil
}

// NextTransition returns the instant at which a watermark's freshness would next
// change if nothing further arrived, and whether such an instant exists.
//
// # This is what §3.4's silence timer is armed from
//
// §3.4 requires that a stale mark be detected "by a `Clock` timer that fires on
// SILENCE — a feed that simply stops emits no event, so staleness MUST be
// detected by a timer, not only by the next update". The natural implementation
// checks freshness when an update arrives, which is precisely the check a dead
// feed never triggers: the position stays marked fresh for the entire outage.
//
// Returning the instant rather than arming the timer keeps the arithmetic in the
// same place as the thresholds it derives from. The alternative — the
// application computing `RecvTime.Add(budget.MaxAge)` itself — is a second copy
// of the boundary rule, and §7.6 already paid for what happens when one
// mechanism is written down in more than one place.
//
// False means there is no later transition: an observation already blind has
// nowhere further to age, so there is nothing to arm.
func (b Budget) NextTransition(w Watermark, now time.Time) (time.Time, bool) {
	f, err := b.Assess(w, now)
	if err != nil {
		return time.Time{}, false
	}
	switch f {
	case FreshnessFresh:
		return w.RecvTime.Add(b.MaxAge), true
	case FreshnessStale:
		return w.RecvTime.Add(b.Blind), true
	case FreshnessBlind, FreshnessUnknown:
		return time.Time{}, false
	}
	return time.Time{}, false
}
