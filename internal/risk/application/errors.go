package application

import "errors"

var (
	// ErrPostureSourceMissing reports a guard constructed with no posture
	// source.
	//
	// A nil source would panic on the first Assert, which in §7.5's boot is a
	// crash where a named refusal belongs — and the gate the crash prevents
	// from reporting is the one whose job is refusing to trade.
	ErrPostureSourceMissing = errors.New("risk: posture guard has no account posture source")
)
