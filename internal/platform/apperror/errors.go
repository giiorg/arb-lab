package apperror

import "errors"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
//
// Note what is NOT here: there is no ErrRateLimited, ErrNotFound or ErrTimeout.
// Those questions are answered by ClassOf, and giving them a second answer as a
// sentinel would create two mechanisms that can disagree — a wrapped error whose
// Class says rate_limit while errors.Is finds a transport sentinel underneath.
// One mechanism, deliberately. The sentinels below are about a malformed
// classification, which is a different subject entirely.
var (
	// ErrClassInvalid reports a Class outside the declared set, in practice a
	// typo in an adapter's classification table.
	ErrClassInvalid = errors.New("apperror: invalid class")

	// ErrEffectInvalid reports an Effect outside the declared set, including
	// the unspecified zero value: a classification reached with nobody having
	// said whether the side effect happened.
	ErrEffectInvalid = errors.New("apperror: invalid effect")

	// ErrOpUnset reports a classification with no operation name. The op is what
	// makes a metric or a log line actionable — "transport failure" without
	// "venue.Place" names a symptom and no subject — so an empty one is a defect
	// in the calling code rather than a stylistic lapse.
	ErrOpUnset = errors.New("apperror: operation name unset")

	// ErrMalformed reports an Error built from a Spec that did not validate. It
	// is what Malformed errors wrap, so a boot check or a test can assert that
	// no classification in a code path is malformed without inspecting each one.
	ErrMalformed = errors.New("apperror: malformed classification")
)
