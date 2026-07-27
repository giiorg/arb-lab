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
)

// There is deliberately no ErrInstrumentMismatch here, and its absence was a
// decision rather than an omission. One was declared in this slice's first draft
// — "it cannot happen through the exported API, which derives the key from the
// payload; it is the assertion that keeps that true" — and no such assertion
// existed, nor could one: both write paths derive the key from the payload one
// line before storing it, and domain.Watermark carries no instrument of its own
// to disagree with. So the sentinel was returned by nothing and reachable by
// nothing, while `errors.Is(err, ErrInstrumentMismatch)` compiled, always
// answered false, and READ as a handled case. Its two siblings are the contrast
// that makes the point: oms.ErrFillInstrumentMismatch and
// marketdata.ErrBookInstrumentMismatch are each returned by real code and pinned
// by a test, because there the two identities genuinely arrive from different
// places.
