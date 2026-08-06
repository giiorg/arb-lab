package driven

import "errors"

var (
	// ErrAccountPostureReaderMissing reports an adapter constructed with no
	// venue client.
	ErrAccountPostureReaderMissing = errors.New("risk: account posture adapter has no venue client")

	// ErrAccountPostureVenueMismatch reports a scope naming a venue this
	// adapter's client does not speak for.
	ErrAccountPostureVenueMismatch = errors.New("risk: account posture scope names another venue")

	// ErrAccountPostureUntranslatable reports a venue observation this context
	// has no vocabulary for.
	//
	// It refuses the whole report rather than dropping the observation, and
	// that is the entire reason it exists: a dropped observation is a check
	// with no evidence, which the rules would report as an unobserved setting —
	// a message pointing an operator at the venue, for a defect that is on this
	// side of the wire.
	ErrAccountPostureUntranslatable = errors.New("risk: venue posture observation has no domain vocabulary")
)
