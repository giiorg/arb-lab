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

	// ErrBootNotClean reports an execute phase declined because the boot's
	// verdict was not ExitClean.
	//
	// It is a REFUSAL, not a failure: §59d gates execution on the produced
	// gates concluding and nothing latching, and a boot that found operator
	// work or could not finish leaves a process that reports and exits — with
	// this error naming why nothing was sent.
	ErrBootNotClean = errors.New("exercise: boot verdict does not admit execution")
)
