package clock

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
var (
	// ErrServerTimeUnset reports a ServerTime that was never constructed — in
	// practice, ServerNow asked about a venue whose clock offset was never
	// calibrated. ServerNow has no error to return (§3.5 fixes its signature),
	// so this is where an uncalibrated venue becomes visible.
	ErrServerTimeUnset = errors.New("clock: unconstructed server time")

	// ErrUncertaintyNegative reports a negative uncertainty band. A negative
	// band is not a wide one, it is a caller who subtracted two instants in the
	// wrong order — and it would make Earliest() later than Latest(), quietly
	// inverting every boundary comparison.
	ErrUncertaintyNegative = errors.New("clock: negative uncertainty band")

	// ErrCertaintyInvalid reports a Certainty outside the declared set, in
	// practice the zero value: a decision variable nobody assigned.
	ErrCertaintyInvalid = errors.New("clock: invalid certainty")
)

// Certainty is the answer to a question asked of a clock we only know to within
// a band. It has three values on purpose, and the third is the point.
//
// §3.6 rule 8's shape: a string type with declared constants and a Validate, so
// the zero value is not a member and a Certainty that was never assigned fails
// rather than reading as a decision.
type Certainty string

const (
	// CertainlyPassed: every instant the band admits is at or past the
	// boundary.
	CertainlyPassed Certainty = "certainly_passed"

	// CertainlyNotPassed: every instant the band admits is before it.
	CertainlyNotPassed Certainty = "certainly_not_passed"

	// Uncertain: the band straddles the boundary, so both answers are
	// consistent with what we know and neither is a legitimate one. A caller
	// that treats this as either has made a funding-settlement decision on a
	// coin flip.
	Uncertain Certainty = "uncertain"
)

func (c Certainty) Validate() error {
	switch c {
	case CertainlyPassed, CertainlyNotPassed, Uncertain:
		return nil
	default:
		return fmt.Errorf("%q: %w", string(c), ErrCertaintyInvalid)
	}
}

func (c Certainty) String() string {
	if err := c.Validate(); err != nil {
		return "Certainty(" + string(c) + ")"
	}
	return string(c)
}

// ServerTime is an exchange's clock as we best know it: an estimate of the
// instant, plus the half-width of the interval the true server time lies in.
//
// Exchange server time is authoritative for funding-boundary decisions (§3.5),
// and it differs from this machine's clock by an amount known only approximately.
// So the type carries the band rather than dropping it, and exposes the
// comparison rather than the operands — see Passed.
//
// The zero value is deliberately unusable, for the same reason every fixedpoint
// category's is: an unpopulated field that reads as a working value computes
// milestone 2's economic gate in our favour with nothing failing.
type ServerTime struct {
	instant     time.Time
	uncertainty time.Duration
	set         bool
}

// NewServerTime builds a ServerTime from an instant and the half-width of its
// uncertainty band.
func NewServerTime(instant time.Time, uncertainty time.Duration) (ServerTime, error) {
	if instant.IsZero() {
		return ServerTime{}, fmt.Errorf("instant is the zero time: %w", ErrServerTimeUnset)
	}
	if uncertainty < 0 {
		return ServerTime{}, fmt.Errorf("uncertainty %v: %w", uncertainty, ErrUncertaintyNegative)
	}
	return ServerTime{instant: instant, uncertainty: uncertainty, set: true}, nil
}

func (s ServerTime) Validate() error {
	if !s.set {
		return ErrServerTimeUnset
	}
	return nil
}

func (s ServerTime) Instant() time.Time         { return s.instant }
func (s ServerTime) Uncertainty() time.Duration { return s.uncertainty }

// Earliest and Latest are the band's edges — the interval the true server time
// lies in. They are part of the contract because a caller reporting "we are
// inside the band" needs to say how wide it is.
func (s ServerTime) Earliest() time.Time { return s.instant.Add(-s.uncertainty) }
func (s ServerTime) Latest() time.Time   { return s.instant.Add(s.uncertainty) }

// Passed reports whether the boundary has certainly passed, certainly not, or
// cannot be told apart from the uncertainty band.
//
// This method is the reason ServerTime is a type rather than a pair of return
// values. Every call site that did this arithmetic itself would do it slightly
// differently, and the one that forgot the band would answer "unknown" as either
// yes or no — in the code path deciding whether to hold a position through a
// funding settlement.
//
// "Passed" means server time is at or after the boundary: the boundary IS the
// settlement instant, so being exactly on it counts. That inclusivity is why
// Earliest() == boundary is CertainlyPassed while Latest() == boundary is only
// Uncertain.
//
// An unset ServerTime is always Uncertain. Safe by default and loud on request:
// a caller who skipped Validate stalls rather than acting on a clock it does not
// have, and answering either way would make an uncalibrated venue look like a
// working one.
func (s ServerTime) Passed(boundary time.Time) Certainty {
	if !s.set {
		return Uncertain
	}
	switch {
	case !s.Earliest().Before(boundary):
		return CertainlyPassed
	case s.Latest().Before(boundary):
		return CertainlyNotPassed
	default:
		return Uncertain
	}
}
