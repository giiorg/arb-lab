package apperror

import "fmt"

// Class names what kind of thing went wrong. It answers the routing question —
// how should the caller back off, which alert fires, which metric label does
// this failure land under — and deliberately says nothing about whether the
// operation's side effect happened. That is Effect's job, and the two do not
// determine each other: a 429 and a socket timeout are both failures reaching a
// venue over HTTP, and they differ on the safety axis, not on this one.
//
// §3.6 rule 8's shape: a string type with declared constants and a Validate. The
// shape applies rather than fixedpoint.RoundingMode's integer-enum exception,
// because a Class does cross a boundary — §10 makes it a metric label
// (error/rate-limit rate per venue) and the audit projection stores it. A
// self-describing token is worth its width when the reader is a dashboard query
// or a stored audit row six months later.
type Class string

const (
	// ClassTransport is a failure to complete the exchange with the venue at
	// all: connection refused, TLS failure, read timeout, truncated body. The
	// venue never formed an opinion, so nothing here tells us what happened to
	// the request — which is why every Transport error must state its Effect
	// explicitly rather than take a default (see Transport).
	ClassTransport Class = "transport"

	// ClassRateLimit is a refusal on budget grounds: Binance 429/418, Bybit
	// 403+10006, and the unheadered IP ban at retCode 10018 (§7.6). It is
	// separate from ClassVenueReject because §7.6 assigns it different handling
	// — a hard pause and, for 418 and the IP ban, an immediate operator alert —
	// and because platform/ratelimit needs to find these without parsing text.
	ClassRateLimit Class = "rate_limit"

	// ClassAuth is a credential or permission failure: bad signature, expired
	// key, IP not on the allowlist, endpoint not permitted for this key. It is
	// its own class because §7.3 and §7.4 make it a security signal, not a
	// retryable hiccup — a key that stops working mid-session is either a
	// rotation gone wrong or a fence that engaged, and both want an operator.
	ClassAuth Class = "auth"

	// ClassVenueReject is the venue understanding the request and refusing it on
	// its merits: below minimum notional, price outside the band, reduce-only
	// with no position, insufficient margin. This is the class that maps to the
	// order lifecycle's Rejected terminal (§5.3) — an explicit venue rejection,
	// which the spec is careful to distinguish from AbsentAfterFinality's proved
	// absence. They are different evidence classes.
	ClassVenueReject Class = "venue_reject"

	// ClassNotFound is a lookup that found nothing: no order under this client
	// ID, no such instrument. It is emphatically NOT a terminal answer on its
	// own — §5.3 forbids inferring "not submitted" from a transient not-found,
	// and the venue-specific stable-not-found rule is what turns a sequence of
	// these into finality. The class exists so that rule has something to count;
	// a caller that treats one of these as proof has skipped the rule.
	ClassNotFound Class = "not_found"

	// ClassPrecondition is our own refusal to proceed: a value that failed
	// Validate, a config that does not parse, an operation attempted in a state
	// that forbids it. The request never left the process, which is why
	// Precondition fixes Effect at EffectNotApplied.
	ClassPrecondition Class = "precondition"

	// ClassInternal is a defect in this program: an impossible branch, a nil
	// where the type says otherwise, a decode of data we ourselves wrote. It is
	// also the classification an unclassified error gets when ClassOf walks a
	// chain and finds nothing — see ClassOf, which is where the default
	// matters more than the constant does.
	ClassInternal Class = "internal"
)

// classes is the declared set, in declaration order. Validate and the
// String round-trip both read it, so adding a constant above means adding it
// here — which the declared-set test checks by counting the constants it can
// reach through the package's own API.
var classes = []Class{
	ClassTransport,
	ClassRateLimit,
	ClassAuth,
	ClassVenueReject,
	ClassNotFound,
	ClassPrecondition,
	ClassInternal,
}

// Validate reports whether c is one of the declared classes.
//
// Class is a defined string type, so Class("whatever") compiles. Validate is the
// only thing between a typo in an adapter and a metric label nobody is
// aggregating on.
func (c Class) Validate() error {
	for _, known := range classes {
		if c == known {
			return nil
		}
	}
	return fmt.Errorf("%q: %w", string(c), ErrClassInvalid)
}

// String implements fmt.Stringer. An undeclared class prints wrapped rather than
// bare, so a malformed value is visible in a log line instead of passing for a
// class that simply has not been declared yet.
func (c Class) String() string {
	if err := c.Validate(); err != nil {
		return "Class(" + string(c) + ")"
	}
	return string(c)
}
