package ratelimit

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Reason says why a decision came out the way it did.
type Reason uint8

const (
	// ReasonUnspecified is the zero value. A Decision never carries it; it
	// exists so that a zero Decision does not read as an admission.
	ReasonUnspecified Reason = iota

	// ReasonAdmitted is the request being allowed through.
	ReasonAdmitted

	// ReasonCeiling is a budget refusal: some limit would pass this priority's
	// ceiling. Decision.Limit names which.
	ReasonCeiling

	// ReasonPaused is a venue-wide hold following a §7.6 penalty — a 429, a
	// 403+10006, a 418 or Bybit's unheadered 10018.
	//
	// It is distinct from ReasonCeiling because the two are different facts. A
	// ceiling refusal is our own policy and could be re-tuned; a pause is the
	// venue having already refused, and no amount of local budget changes it.
	ReasonPaused
)

// String implements fmt.Stringer.
func (r Reason) String() string {
	switch r {
	case ReasonUnspecified:
		return "ReasonUnspecified"
	case ReasonAdmitted:
		return "ReasonAdmitted"
	case ReasonCeiling:
		return "ReasonCeiling"
	case ReasonPaused:
		return "ReasonPaused"
	}
	return fmt.Sprintf("Reason(%d)", uint8(r))
}

// Decision is the governor's answer, and it is a value rather than a bool so
// that a refusal can say when to come back.
//
// It is self-contained: At and NotBefore are both instants, so a caller can
// compute the wait without a clock of its own and a test can assert the whole
// answer without one either.
type Decision struct {
	// Admitted is the answer. Everything else explains it.
	Admitted bool

	// Reason is why.
	Reason Reason

	// Venue is the venue the decision was made for.
	Venue ids.Venue

	// Limit names the budget that refused, for ReasonCeiling. When several
	// refused it is the one that frees LAST — see NotBefore.
	Limit string

	// At is the instant the decision was made, read from the injected clock.
	At time.Time

	// NotBefore is the earliest instant a retry could succeed.
	//
	// When several limits refuse, this is the LATEST of their release times,
	// not the earliest. Naming the earliest would send the caller back to be
	// refused by one of the others, which turns a backoff into a poll against a
	// venue that is already declining traffic.
	//
	// The zero time means "not by waiting": the request charges more than the
	// ceiling admits at any consumption, so it never becomes issuable at this
	// priority. A caller must not read it as an instant already past — that is
	// the same overflowed-backoff shape outbox.Backoff avoids, arriving through
	// a different door.
	NotBefore time.Time
}

// RetryAfter is how long to wait before retrying, or zero when there is no such
// wait — either because the request was admitted, or because NotBefore says
// waiting will not help.
func (d Decision) RetryAfter() time.Duration {
	if d.Admitted || d.NotBefore.IsZero() {
		return 0
	}
	if !d.NotBefore.After(d.At) {
		return 0
	}
	return d.NotBefore.Sub(d.At)
}

// Err renders a refusal as a classified error, and nil for an admission.
//
// Effect is EffectNotApplied, and here that is a claim this package is uniquely
// entitled to make: apperror.RateLimited fixes it for a VENUE refusal, and a
// local refusal is stronger evidence still — nothing was sent, so nothing can
// have executed. §5.3 forbids inferring "not submitted" from a venue's
// not-found, and this is the one place where the inference is not an inference.
//
// It is a method rather than the return type because refusal is an expected
// outcome, not a failure: a caller deciding whether to open a position wants a
// Decision, and a caller propagating up a stack wants an error.
func (d Decision) Err() error {
	if d.Admitted {
		return nil
	}
	return apperror.New(apperror.Spec{
		Class:      apperror.ClassRateLimit,
		Effect:     apperror.EffectNotApplied,
		Op:         "ratelimit.Acquire",
		Detail:     d.detail(),
		RetryAfter: d.RetryAfter(),
	})
}

func (d Decision) detail() string {
	switch d.Reason {
	case ReasonPaused:
		return fmt.Sprintf("venue %s is paused until %s", d.Venue, d.NotBefore.UTC().Format(time.RFC3339Nano))
	case ReasonCeiling:
		if d.NotBefore.IsZero() {
			return fmt.Sprintf("venue %s limit %q cannot admit this request at any consumption",
				d.Venue, d.Limit)
		}
		return fmt.Sprintf("venue %s limit %q is at its ceiling until %s",
			d.Venue, d.Limit, d.NotBefore.UTC().Format(time.RFC3339Nano))
	case ReasonUnspecified, ReasonAdmitted:
		// Neither is a refusal, so neither has a refusal detail to render.
		// Named rather than left to the generic line below (§3.6 rule 10):
		// that line describes a refusal, and reaching it with ReasonAdmitted
		// would print "refused the request (admitted)".
	}
	return fmt.Sprintf("venue %s refused the request (%s)", d.Venue, d.Reason)
}

// worseThan reports whether release instant a is a longer wait than b, treating
// the zero time as "never" rather than as the distant past.
//
// Written as a helper because the zero-time case is the one a comparison
// written inline gets wrong: time.Time{}.After(anything) is false, so the
// never-releasable limit would silently lose to every other refusal and the
// caller would be told to retry at an instant that cannot work.
func worseThan(a, b time.Time) bool {
	if a.IsZero() {
		return !b.IsZero()
	}
	if b.IsZero() {
		return false
	}
	return a.After(b)
}
