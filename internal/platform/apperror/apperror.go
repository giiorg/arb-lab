// Package apperror is the error taxonomy build-spec §5.1 calls errclass.
//
// # Two axes, because there are two questions
//
// Every failure in this system is asked two things, and neither answer implies
// the other:
//
//   - Class — what went wrong. Drives backoff (§7.6), alerting, and the
//     error/rate-limit metric labels (§10).
//   - Effect — did the side effect happen. Drives the OMS order lifecycle
//     (§5.3): EffectUnknown is what puts an order into SubmissionUnknown and
//     sends recovery to OrderByClientID instead of to a resubmission.
//
// A 429 and a socket read timeout share a Class neighbourhood and differ
// absolutely on Effect. A connection refused and a read timeout share a Class
// exactly and still differ on Effect. Collapsing the two into one enum would
// make the safety-critical fact something a caller derives from the cause — and
// deriving it is precisely what §5.3 forbids when it says "not sent" is never
// inferred.
//
// # The defaults are the design
//
// Almost every value in this package is chosen so that the failure mode of
// forgetting it is the safe one:
//
//   - An error this package never saw reads as ClassInternal, EffectUnknown.
//     An unclassified failure from a library or a forgotten branch therefore
//     reports "this may have executed" rather than "this did not".
//   - MayHaveApplied answers true for everything except EffectNotApplied, which
//     is the single value that is a claim about evidence. A stray cast cannot
//     open the retry branch.
//   - Wrapping cannot make an outcome more certain: see resolveEffect.
//   - A Spec that fails Validate still produces a usable error, degraded to
//     ClassInternal/EffectUnknown and marked Malformed, because a constructor
//     on a failure path must not itself fail.
//
// # What is deliberately not here
//
// No retry loop, no backoff schedule, no circuit breaker. §7.6 assigns rate-limit
// governance to platform/ratelimit, which does not exist yet; this package
// carries the venue's Retry-After hint and interprets none of it. Nor is there a
// venue-code-to-Class table: that mapping is venue-specific knowledge and belongs
// in engine/venue beside the client that produces the codes, where
// venue-profiles.md can be cited next to it.
package apperror

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Error is a classified failure. Build it with New or one of the shorthand
// constructors; the fields are unexported so that an Error always carries a
// classification that has been through Validate, or is marked as not having.
type Error struct {
	class      Class
	effect     Effect
	op         string
	detail     string
	code       string
	retryAfter time.Duration
	cause      error

	// malformed records that the Spec this was built from did not validate. The
	// classification above has already been degraded to the safe reading; this
	// is what makes the degradation visible rather than merely survivable.
	malformed error
}

// Spec is the input to New. It is a struct rather than a positional argument
// list because the natural signature — New(class, effect, op, detail string) —
// puts two same-typed strings side by side on a failure path, which is a
// transposition waiting to happen in code that is by definition not on the
// tested-happy-path.
type Spec struct {
	// Class and Effect are both required. See their own documentation; the
	// short version is that Class routes and Effect decides safety.
	Class  Class
	Effect Effect

	// Op names the operation that failed, in the form "package.Method" —
	// "venue.Place", "clock.ServerNow". Required: it is what turns a metric
	// series into something an operator can act on.
	Op string

	// Detail is free text for a human. Optional, and it is not a metric label:
	// anything a dashboard needs to group on belongs in Class or Code.
	Detail string

	// Code is the venue's own error code, verbatim — "429", "-2011", "10006".
	// Optional. It exists because §7.6 names specific codes as hard pauses and
	// specific ones as operator alerts, and because the Class we assigned is an
	// interpretation while this is the evidence for it.
	Code string

	// RetryAfter is the venue's own hint, carried and not interpreted. Zero
	// means the venue said nothing, which is different from "retry now" —
	// deciding what to do with either is platform/ratelimit's job.
	RetryAfter time.Duration

	// Cause is the underlying error, reachable through errors.Is and errors.As.
	// If it carries its own classification, its Effect constrains the resulting
	// one: see resolveEffect.
	Cause error
}

// Validate reports whether the spec is fully classified.
func (s Spec) Validate() error {
	if err := s.Class.Validate(); err != nil {
		return err
	}
	if err := s.Effect.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.Op) == "" {
		return ErrOpUnset
	}
	return nil
}

// New builds a classified error.
//
// It returns *Error and never a second error value, on purpose. A constructor
// called on a failure path must not add a failure path of its own — the caller
// is already holding something that went wrong and has nowhere to put a second
// problem. So an invalid Spec does not fail: it produces an error that is
// degraded to ClassInternal/EffectUnknown (safe), records what was wrong with it
// (loud), and answers true to Malformed so a test or a boot check can assert
// that a code path produces none.
//
// The degradation is deliberately total rather than field-by-field. A Spec with
// a valid Class and an unspecified Effect could in principle keep its Class, but
// a classification that is half-filled is more likely to be a construction site
// that was not thought through than one field that slipped, and reporting it as
// internal puts it where an operator will see it.
func New(s Spec) *Error {
	e := &Error{
		op:         strings.TrimSpace(s.Op),
		detail:     s.Detail,
		code:       s.Code,
		retryAfter: s.RetryAfter,
		cause:      s.Cause,
	}

	if err := s.Validate(); err != nil {
		e.class = ClassInternal
		e.effect = EffectUnknown
		e.malformed = fmt.Errorf("%w: %w", ErrMalformed, err)
		if e.op == "" {
			e.op = "unknown"
		}
		return e
	}

	e.class = s.Class
	e.effect = resolveEffect(s.Effect, recordedEffect(s.Cause))
	return e
}

// recordedEffect reports what err's chain actually CLAIMS about the side effect,
// as opposed to what a caller holding err should assume about it.
//
// The difference is the whole reason this is not EffectOf. EffectOf answers a
// decision — "I have a failure, may the write have landed?" — so an error it
// cannot classify must read as EffectUnknown, or an unclassified failure would
// license a resubmission. This function answers a combination — "does anything
// beneath me contradict the classification I am about to record?" — and there an
// unclassified error contradicts nothing, so it must read as EffectUnspecified.
//
// Reusing EffectOf here looks right and is not: every adapter wrapping a raw
// net/http or json error would find EffectUnknown underneath, resolveEffect
// would read that as a disagreement, and EffectNotApplied would become
// unreachable in practice. The taxonomy would still be safe — it would report
// everything as ambiguous — which is exactly why the bug would have survived.
// Note that the non-laundering rule is not weakened by this: it protects a
// recorded claim from being contradicted by an outer layer, and an unclassified
// leaf has made no claim to protect. A leaf classified WRONGLY is a different
// problem, and no combination rule can catch it.
func recordedEffect(err error) Effect {
	if err == nil {
		return EffectUnspecified
	}
	var e *Error
	if errors.As(err, &e) {
		return e.effect
	}
	return EffectUnspecified
}

// Error implements the error interface.
//
// The format leads with op and class because that is what a reader scanning a
// log is looking for, and it names the effect explicitly rather than leaving it
// to be inferred — an operator reading "may have executed" at 3am should not
// have to remember which classes are ambiguous.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.op)
	b.WriteString(": ")
	b.WriteString(e.class.String())
	if e.code != "" {
		b.WriteString(" [")
		b.WriteString(e.code)
		b.WriteString("]")
	}
	b.WriteString(": ")
	b.WriteString(e.effectPhrase())
	if e.detail != "" {
		b.WriteString(": ")
		b.WriteString(e.detail)
	}
	if e.malformed != nil {
		b.WriteString(": ")
		b.WriteString(e.malformed.Error())
	}
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

// effectPhrase renders the effect for a human rather than for a switch. The
// unspecified case cannot be reached through New — it degrades to Unknown — but
// a zero-valued Error can still be printed, and printing "EffectUnspecified"
// there would read as a missing field rather than as the hazard it is.
func (e *Error) effectPhrase() string {
	switch e.effect {
	case EffectNotApplied:
		return "did not execute"
	case EffectApplied:
		return "executed"
	case EffectUnknown, EffectUnspecified:
		return "MAY HAVE EXECUTED"
	}
	return "MAY HAVE EXECUTED (" + e.effect.String() + ")"
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }

// Class, Effect, Op, Code and RetryAfter read the classification back.
//
// Prefer the package-level ClassOf and EffectOf on anything that arrived as a
// plain error: these methods require you to have already found the *Error, and
// the whole point of the defaults is what happens when there is none to find.
func (e *Error) Class() Class              { return e.class }
func (e *Error) Effect() Effect            { return e.effect }
func (e *Error) Op() string                { return e.op }
func (e *Error) Code() string              { return e.code }
func (e *Error) RetryAfter() time.Duration { return e.retryAfter }

// Malformed reports whether this error was built from a Spec that did not
// validate, and returns what was wrong with it. A malformed error is still
// usable — it reads as ClassInternal/EffectUnknown — so this exists for the
// caller that wants to fail a test or a boot check rather than survive.
func (e *Error) Malformed() error { return e.malformed }

// ClassOf returns the class of the outermost classification in err's chain, or
// ClassInternal if there is none.
//
// ClassInternal is the right answer for an unclassified error rather than a
// separate "unclassified" constant, because it is true: an error reaching a
// decision site without a classification is a gap in this program, and it should
// aggregate with the other gaps on the dashboard instead of getting a label of
// its own that nobody has an alert for.
//
// A nil error returns ClassInternal too. Call it only when you have an error.
func ClassOf(err error) Class {
	var e *Error
	if errors.As(err, &e) {
		return e.class
	}
	return ClassInternal
}

// EffectOf returns the effect of the outermost classification in err's chain, or
// EffectUnknown if there is none.
//
// "Outermost" matters and is safe only because of resolveEffect. The outermost
// classification is the one nearest the caller's own layer and usually the most
// specific, but on its own that rule would let an outer wrap restate an
// ambiguous outcome as a certain one. New forbids that at construction, so by
// the time EffectOf reads a value it cannot be more certain than anything
// beneath it.
//
// A nil error returns EffectUnspecified, not EffectUnknown: there is no failure,
// so there is no effect to report. Use MayHaveApplied, which handles nil, rather
// than comparing this result yourself.
func EffectOf(err error) Effect {
	if err == nil {
		return EffectUnspecified
	}
	var e *Error
	if errors.As(err, &e) {
		return e.effect
	}
	return EffectUnknown
}

// MayHaveApplied reports whether err leaves open the possibility that the
// operation's side effect ran. It is the question §5.3 asks before it decides
// between resolving an order by query and treating it as never dispatched.
//
// A nil error reports false — nothing failed, so nothing is in doubt. Everything
// else reports true unless it is classified EffectNotApplied, including errors
// this package never saw.
func MayHaveApplied(err error) bool {
	if err == nil {
		return false
	}
	return EffectOf(err).mayHaveApplied()
}
