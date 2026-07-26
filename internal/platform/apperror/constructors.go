package apperror

import "time"

// The shorthand constructors below exist for the classes whose Effect is a
// property of the class itself. Where the Effect genuinely varies — transport —
// the constructor demands it as an argument rather than picking one, because the
// whole reason this package has two axes is that transport failures are the
// place the two come apart.
//
// None of them accept a Spec's every field; a caller that needs RetryAfter on
// something other than a rate limit, or a Detail on a transport failure, uses
// New directly. These are for the common shape, not a replacement for it.

// Transport reports a failure to complete the exchange with the venue.
//
// effect is an ARGUMENT, and this is the one signature in the package where
// getting it wrong is unrecoverable rather than merely wasteful. The adapter
// making the call is the only layer that can tell a connection refused before
// the first byte (EffectNotApplied) from a read timeout after the request went
// out (EffectUnknown), and by the time the error reaches the OMS that
// information is gone. Passing EffectUnknown when unsure is always correct and
// costs a resolving query; passing EffectNotApplied when unsure is how a
// duplicate live order gets created.
func Transport(op string, effect Effect, cause error) *Error {
	return New(Spec{Class: ClassTransport, Effect: effect, Op: op, Cause: cause})
}

// RateLimited reports a refusal on budget grounds — §7.6's 429/418 on Binance,
// 403+10006 and the unheadered 10018 IP ban on Bybit.
//
// Effect is fixed at EffectNotApplied because a rate-limited request is refused
// instead of executed: the venue's gateway declines it before the matching
// engine sees it, and a local governor that declines to send never sends. Note
// that this is a claim about a REFUSAL, not about a timeout that happened to
// occur while we were near the budget — that one is a Transport error and its
// effect is unknown.
//
// retryAfter is the venue's hint verbatim; zero means it gave none.
func RateLimited(op, code string, retryAfter time.Duration, cause error) *Error {
	return New(Spec{
		Class:      ClassRateLimit,
		Effect:     EffectNotApplied,
		Op:         op,
		Code:       code,
		RetryAfter: retryAfter,
		Cause:      cause,
	})
}

// Unauthorized reports a credential or permission failure. Effect is
// EffectNotApplied: authentication is checked before the request is acted on, so
// a rejected signature moved nothing.
//
// §7.3 and §7.4 make this a security signal rather than a retryable failure. A
// key that worked a minute ago and does not now is a rotation gone wrong or a
// fence that engaged, and both want an operator rather than a backoff.
func Unauthorized(op, code string, cause error) *Error {
	return New(Spec{Class: ClassAuth, Effect: EffectNotApplied, Op: op, Code: code, Cause: cause})
}

// VenueRejected reports the venue understanding the request and refusing it on
// its merits. Effect is EffectNotApplied, and here that is not a convenience but
// the evidence class §5.3 distinguishes: an explicit venue rejection is what
// justifies the Rejected terminal, as opposed to AbsentAfterFinality's proved
// absence. Do not reach for this constructor to express "we could not find it".
func VenueRejected(op, code, detail string, cause error) *Error {
	return New(Spec{
		Class:  ClassVenueReject,
		Effect: EffectNotApplied,
		Op:     op,
		Code:   code,
		Detail: detail,
		Cause:  cause,
	})
}

// NotFound reports a lookup that found nothing.
//
// Effect is EffectNotApplied and carries no information: a query has no side
// effect, so "it did not execute" is trivially true of it. The value of this
// constructor is entirely in the Class, which is what the venue-specific
// stable-not-found rule counts (§5.3).
//
// Read the class carefully at the call site that consumes it. One of these is
// never proof that an order was not submitted — §5.3 forbids exactly that
// inference — and a caller that treats a single not-found as finality has
// skipped the rule this class exists to feed.
func NotFound(op, detail string, cause error) *Error {
	return New(Spec{Class: ClassNotFound, Effect: EffectNotApplied, Op: op, Detail: detail, Cause: cause})
}

// Precondition reports our own refusal to proceed: a failed Validate, an
// unparseable config, an operation attempted in a state that forbids it. Effect
// is EffectNotApplied because the request never left the process.
func Precondition(op, detail string, cause error) *Error {
	return New(Spec{Class: ClassPrecondition, Effect: EffectNotApplied, Op: op, Detail: detail, Cause: cause})
}

// Internal reports a defect in this program. Effect is EffectUnknown, which is
// the conservative reading and the honest one: a branch that should have been
// impossible tells us nothing about how far the operation got before it was
// reached.
func Internal(op, detail string, cause error) *Error {
	return New(Spec{Class: ClassInternal, Effect: EffectUnknown, Op: op, Detail: detail, Cause: cause})
}
