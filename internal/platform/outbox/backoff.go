package outbox

import (
	"fmt"
	"time"
)

// Backoff is the retry schedule: exponential from Base, capped at Max.
//
// # No jitter, and that is a decision rather than an omission
//
// Jitter exists to de-synchronise many independent clients hammering one
// server. This relay is a single in-process pump over a local store, so there is
// no herd to disperse — and randomness would make the retry instant
// unreproducible, which breaks the replay determinism §3.5 builds everything
// else on. If a future multi-process deployment creates a herd, the jitter
// belongs there with a seeded source, not here.
//
// # The zero value is unusable
//
// A zero Base makes every retry immediate, which turns a failing consumer into a
// hot loop; a zero Max caps every delay at zero, doing the same thing from the
// other side. Both fail Validate rather than being read as "no delay".
type Backoff struct {
	// Base is the delay after the first failure.
	Base time.Duration

	// Max is the ceiling. A record that keeps failing retries at this interval
	// forever — it is never dropped, because the outbox has no other copy of
	// the event (see the package doc).
	Max time.Duration
}

// Validate reports whether the schedule can produce a delay.
func (b Backoff) Validate() error {
	if b.Base <= 0 {
		return fmt.Errorf("base %v: %w", b.Base, ErrBackoffInvalid)
	}
	if b.Max < b.Base {
		return fmt.Errorf("max %v below base %v: %w", b.Max, b.Base, ErrBackoffInvalid)
	}
	return nil
}

// Delay returns the wait after the given number of prior attempts: Base doubled
// once per attempt, capped at Max.
//
// # The guard is on the RIGHT-hand side, and that is the whole trick
//
// The obvious form — shift first, then compare against Max — is wrong, and
// wrong in the direction that does damage. time.Duration is an int64 of
// nanoseconds, so with a Base of one second the shift overflows at 34 attempts,
// and an overflowed shift does not produce a very long wait: it produces a
// NEGATIVE one. A negative delay schedules RetryAt in the past, so the record is
// reclaimed on the very next pass — the backoff turns into a hot loop against a
// consumer that is already failing, which is the precise opposite of what a
// backoff is for.
//
// Comparing `Base > Max>>attempts` instead asks the same question without ever
// performing the overflowing shift. Go defines a right shift wider than the type
// as zero for a positive operand, so a huge attempt count answers `Base > 0`,
// which is true for any valid schedule — the function is total over every uint32
// with no special-case branch to leave untested.
func (b Backoff) Delay(attempts uint32) time.Duration {
	if b.Base > b.Max>>attempts {
		return b.Max
	}
	return b.Base << attempts
}
