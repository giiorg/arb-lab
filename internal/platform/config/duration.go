package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration that crosses the configuration boundary as a
// string.
//
// # Why not a plain number
//
// §5.2 is full of durations — the per-venue mark max-age, the T_blind blind
// budget, exit_persist's dwell, the forecast model's ewma_halflife — and every
// one of them is a number whose unit lives only in the head of whoever typed it.
// A JSON number decoded into a time.Duration is NANOSECONDS, so a T_blind of 300
// meaning five minutes becomes 300ns: a stale-mark budget that has expired
// before it is first checked, and a de-risk that fires on the first tick.
// Nothing errors, nothing logs, and the §7.1 escalation runs at the wrong time.
//
// So a number is REFUSED rather than interpreted. "5m" is unambiguous to Go and
// to the operator, and there is no reading of it on which they disagree.
//
// # Negatives are refused at the boundary
//
// Every §5.2 duration is a budget, a half-life or a dwell. A negative one is a
// timer that has already fired, so UnmarshalJSON rejects it rather than letting
// it live until a schema's Validate happens to look. Validate rejects it too,
// because NewDuration can construct one in code where no decoder is involved.
type Duration struct {
	d time.Duration
}

// NewDuration lifts a time.Duration. It does not reject a negative — Validate
// does, so a caller building a schema in a test can construct an invalid value
// and watch validation catch it.
func NewDuration(d time.Duration) Duration { return Duration{d: d} }

// Duration is the underlying value.
func (d Duration) Duration() time.Duration { return d.d }

// Validate rejects a negative duration.
//
// The zero duration is deliberately VALID: "no dwell", "no delay" and "check
// immediately" are meaningful settings, unlike a zero cost or a zero rate. A
// schema for which zero is wrong says so in its own Validate, where it can say
// which field and why.
func (d Duration) Validate() error {
	if d.d < 0 {
		return fmt.Errorf("%w: %s", ErrDurationNegative, d.d)
	}
	return nil
}

// String renders the duration in the syntax it is written in.
func (d Duration) String() string { return d.d.String() }

// MarshalJSON writes the duration as a string, so a round trip through this
// package produces a document this package accepts.
func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.d.String()) }

// UnmarshalJSON accepts only a JSON string in time.ParseDuration's syntax. A
// number is an error; see the type doc.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: expected a JSON string such as \"5m\", not a bare number "+
			"(a number would be read as nanoseconds)", ErrDurationMalformed)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDurationMalformed, err)
	}
	if parsed < 0 {
		return fmt.Errorf("%w: %s", ErrDurationNegative, parsed)
	}
	d.d = parsed
	return nil
}
