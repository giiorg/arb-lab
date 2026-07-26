package ratelimit

import (
	"fmt"
	"time"
)

// PenaltyKind is how seriously the venue refused.
//
// Two, because §7.6 draws exactly one line: a 429 on Binance or a 403+10006 on
// Bybit is a hard pause, while a 418 or an IP ban is a hard pause AND an
// immediate operator alert. The distinction is not about duration — it is about
// whether a human has to know.
type PenaltyKind uint8

const (
	// PenaltyUnspecified is the zero value and names no penalty. Refused, so
	// that a half-built Penalty cannot pause a venue for a reason nobody stated.
	PenaltyUnspecified PenaltyKind = iota

	// PenaltyThrottle is a budget refusal the venue expects us to back off
	// from: Binance 429, Bybit 403 with retCode 10006.
	PenaltyThrottle

	// PenaltyBan is an enforcement action: Binance 418 (a 2-minute to 3-day IP
	// ban that lengthens with repetition), Bybit's unheadered ~600/5s gateway
	// ban with retCode 10018.
	//
	// §7.6 requires an immediate operator alert for these. This package does not
	// deliver alerts; Snapshot.Bans is the fact the caller alerts on, and
	// Snapshot.PauseKind says the current pause is one.
	PenaltyBan
)

const maxPenaltyKind = PenaltyBan

// Validate reports whether k names a penalty, excluding the sentinel.
func (k PenaltyKind) Validate() error {
	if k == PenaltyUnspecified || k > maxPenaltyKind {
		return fmt.Errorf("penalty kind %d: %w", uint8(k), ErrPenaltyInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (k PenaltyKind) String() string {
	switch k {
	case PenaltyUnspecified:
		return "PenaltyUnspecified"
	case PenaltyThrottle:
		return "PenaltyThrottle"
	case PenaltyBan:
		return "PenaltyBan"
	}
	return fmt.Sprintf("PenaltyKind(%d)", uint8(k))
}

// moreSevereThan reports whether k outranks other. Severity is the declaration
// order, which is why PenaltyBan is last.
//
// Strictly greater rather than greater-or-equal, deliberately: re-assigning the
// same kind over itself changes nothing, so the two forms are behaviourally
// identical and only one of them can be shown to bite.
func (k PenaltyKind) moreSevereThan(other PenaltyKind) bool { return k > other }

// Penalty is a venue refusal being reported back to the governor.
//
// The venue's error codes are not decoded here — mapping "403 with retCode
// 10006" onto PenaltyThrottle is venue knowledge, and platform/apperror already
// records why that table lives in engine/venue beside the client that produces
// the codes.
type Penalty struct {
	// Kind is how seriously to take it.
	Kind PenaltyKind

	// RetryAfter is the venue's own hint, verbatim; zero means it gave none.
	//
	// It is a floor, not a ceiling: Governor.Penalize waits the LONGER of this
	// and the profile's configured pause. A hint that undershoots would put us
	// straight back into the refusal we are serving — and on Binance that is
	// not free, because repeat violations escalate the ban from two minutes
	// toward three days. The same ratchet direction the header reading gets, for
	// the same reason.
	RetryAfter time.Duration
}

// Validate reports whether the penalty is actionable.
func (p Penalty) Validate() error {
	if err := p.Kind.Validate(); err != nil {
		return err
	}
	if p.RetryAfter < 0 {
		return fmt.Errorf("retry-after %v: %w", p.RetryAfter, ErrPenaltyInvalid)
	}
	return nil
}
