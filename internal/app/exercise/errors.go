package exercise

import "errors"

var (
	// ErrTradeFieldUnset and ErrTradeFieldInvalid report a trade block a §9
	// cycle cannot be driven from — a missing field, and one that parses to no
	// declared value, kept distinct on reconcile's ErrConfigFieldUnset /
	// ErrConfigFieldInvalid precedent: the operator's fix differs.
	ErrTradeFieldUnset   = errors.New("exercise: trade field is unset")
	ErrTradeFieldInvalid = errors.New("exercise: trade field is invalid")

	// ErrLegInstrumentUnknown reports a leg naming an instrument the document's
	// own instrument table does not hold.
	//
	// Refused at Validate rather than discovered at planning, because the
	// instrument row is where the BASE ASSET comes from — a leg without one has
	// no quantity, and the failure would otherwise surface as a fixedpoint
	// parse error naming neither the leg nor the missing row.
	ErrLegInstrumentUnknown = errors.New("exercise: leg names an instrument the config does not declare")

	// ErrReferenceFieldUnset reports a reference block a planning binary
	// cannot run without: no path, or no positive max_age. Required HERE and
	// not in reconcile's schema, because the requirement belongs to the
	// binary that plans — a reconcile-only boot has nothing to verify orders
	// against and nothing to verify.
	ErrReferenceFieldUnset = errors.New("exercise: reference field is unset")

	// ErrReferenceUnreadable reports a reference.path this process could not
	// read at startup.
	ErrReferenceUnreadable = errors.New("exercise: reference revision could not be read")

	// ErrFilterTickUnrecorded reports a reference row without a tick — legal
	// in the strategy domain (rows recorded before the field existed keep
	// decoding, and the scoring path never prices with one) and refused HERE,
	// because this consumer verifies limit prices: a missing tick is a
	// missing constraint, the flattering direction. The guard is restated in
	// terms of its consumer rather than widened in the domain — the
	// documented-zero rule's shape.
	ErrFilterTickUnrecorded = errors.New("exercise: reference row records no tick, which the order path verifies against")

	// ErrFilterRowStale reports a reference row older than reference.max_age.
	// Trading filters move server-side without notice, and every verification
	// this run performs passes against a stale row while each venue judges
	// the current one — the asymmetric-rejection window re-opened behind a
	// green run. The refusal names the row's age and the bound; the fix is a
	// fresh probe into the revision, not a wider bound.
	ErrFilterRowStale = errors.New("exercise: reference row is older than reference.max_age")

	// ErrBootNotClean reports an execute phase declined because the boot's
	// verdict was not ExitClean.
	//
	// It is a REFUSAL, not a failure: §59d gates execution on the produced
	// gates concluding and nothing latching, and a boot that found operator
	// work or could not finish leaves a process that reports and exits — with
	// this error naming why nothing was sent.
	ErrBootNotClean = errors.New("exercise: boot verdict does not admit execution")
)
