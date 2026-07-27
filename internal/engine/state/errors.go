package state

import "errors"

// The package's sentinels. Each names a refusal a caller can branch on, in the
// style platform/apperror's taxonomy expects — a wrapped string is not something
// a composition root can act on.
var (
	// ErrSkewUndefined reports a skew query spanning fewer than two
	// observations. See Skew for why this is not a zero.
	ErrSkewUndefined = errors.New("state: cross-leg skew undefined for fewer than two observations")

	// ErrSkewLimitInvalid reports a SkewLimit that admits nothing. See
	// SkewLimit.Validate.
	ErrSkewLimitInvalid = errors.New("state: skew limit must be positive")

	// ErrEntryAbsent reports a read naming an instrument the cache holds nothing
	// for.
	//
	// It exists for the multi-leg queries, which return an error rather than a
	// found bool. The single-instrument readers return (entry, false) instead,
	// because there "nothing has been observed yet" is the ordinary startup state
	// rather than a fault — and §3.4's FreshnessUnknown makes the same
	// distinction from the other side. A cross-leg query is different: it was
	// asked about a package, and a package one of whose legs has never been
	// observed cannot be scored at all.
	ErrEntryAbsent = errors.New("state: no observation cached for instrument")

	// ErrRegression reports a write whose observation predates the one already
	// held. See Cache.PutFunding.
	ErrRegression = errors.New("state: observation predates the cached one")

	// ErrInstrumentMismatch reports a write whose payload names a different
	// instrument than the key it would be stored under. It cannot happen through
	// the exported API, which derives the key from the payload; it is the
	// assertion that keeps that true.
	ErrInstrumentMismatch = errors.New("state: observation instrument does not match its key")
)
