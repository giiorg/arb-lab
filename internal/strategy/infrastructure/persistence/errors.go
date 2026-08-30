package persistence

import "errors"

// The refusals. Callers branch on these with errors.Is.
var (
	// ErrLevelUndeclared reports an entry level the file does not carry.
	// Asking for 0.0003 REFUSES; it does not return an empty table. An empty
	// table is indistinguishable from "this level was measured and nothing
	// happened", which is the reading that turns a typo into a finding.
	ErrLevelUndeclared = errors.New("persistence: entry level not declared in the file")

	// ErrCoordinateUnset reports a coordinate whose Validate() failed — a
	// field nobody populated reaching a computation as though it were a value.
	ErrCoordinateUnset = errors.New("persistence: coordinate unset")

	// ErrSpanDegenerate reports a symbol whose observed span is under one day.
	//
	// It fires on ZERO rows of the committed file (measured: 0 of 61 symbols).
	// It is declared anyway, because summarise.py:33 writes max(1, days) — a
	// papering-over that inflates an annualised return without failing
	// anything. A gate's refusing path is the untested one, so this one gets a
	// hand-built fixture rather than a committed-data one.
	ErrSpanDegenerate = errors.New("persistence: symbol span is under one day")
)
