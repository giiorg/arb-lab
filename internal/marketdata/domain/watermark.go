package domain

import (
	"fmt"
	"time"
)

// Watermark is the provenance build-spec §3.4 requires on every published
// snapshot: "source sequence, exchange timestamp, receive timestamp".
//
// # Two timestamps, because there are two ways a feed goes stale
//
// SourceTime is the venue's own stamp; RecvTime is when this process took
// delivery. §3.5 carries both "for freshness, cross-leg-skew metrics, and
// offline analysis only", and the plural matters — a feed fails in two
// independent ways and one timestamp sees only one of them:
//
//   - The transport dies. Nothing arrives, RecvTime stops advancing, and
//     SourceTime stops with it. Detected against RecvTime.
//   - The venue's own view stalls while the transport stays healthy. Updates
//     keep arriving on schedule, RecvTime advances normally, and every one of
//     them carries the same increasingly ancient SourceTime. Detected against
//     SourceTime — RecvTime reads perfectly fresh throughout.
//
// The second is the one worth stating, because it is invisible to the check a
// person writes first. See Age and DataAge, which are deliberately separate
// methods measured against deliberately different clocks.
//
// # The cross-leg skew §3.4 also names is NOT here
//
// §3.4's watermark list ends with "max cross-leg skew", which is a property of
// a view spanning two instruments, not of one observation of one of them. A
// single Watermark has nothing to be skewed against. It is computed where the
// legs meet, and putting a field for it here would give every producer a number
// it cannot fill in.
type Watermark struct {
	// VenueSeq is the venue's sequence number within this SourceID, and it is
	// authoritative only there (§3.5). Zero means the venue supplied none —
	// which is the ordinary case for a REST-polled funding read, not an error.
	// Gap detection is therefore available to streamed sources and not to
	// polled ones, and a consumer that needs it must check rather than assume.
	VenueSeq uint64

	// SourceTime is the venue's own stamp on the observation. It is compared
	// against Clock.ServerNow(venue), never against Clock.Now: the two clocks
	// are unsynchronized, and §3.5 rejects cross-venue SourceTime comparison
	// for exactly that reason.
	SourceTime time.Time

	// RecvTime is when this process took delivery, stamped from the INJECTED
	// clock. §3.6 rule 5 widened its ban to engine/ on precisely this point —
	// an adapter stamping RecvTime from the raw clock while freshness is
	// computed from the injected one puts two disagreeing clocks either side of
	// a subtraction, across the NTP step backwards the monotonic clamp exists
	// for.
	RecvTime time.Time
}

// Validate reports whether the watermark could have come from a real
// observation.
//
// VenueSeq is deliberately unchecked: see the field comment. Both timestamps
// are required, because an unset one reads as the zero instant and every age
// computed from it is about fifty-five years — which is not a value any budget
// admits, so the failure would surface as permanent staleness on a healthy feed
// rather than as the unpopulated field it is.
func (w Watermark) Validate() error {
	if w.SourceTime.IsZero() {
		return fmt.Errorf("source time: %w", ErrWatermarkUnset)
	}
	if w.RecvTime.IsZero() {
		return fmt.Errorf("receive time: %w", ErrWatermarkUnset)
	}
	return nil
}

// Age is how long since this process took delivery, measured against engine
// time (Clock.Now).
//
// This is the transport-liveness question: a feed that stopped emitting has a
// growing Age and an unchanging everything-else, and §3.4 requires that be
// detected by a timer firing on SILENCE rather than by the next update, since
// a feed that stopped sends no next update. See Budget.NextTransition, which
// computes when that timer must fire.
//
// # A negative age is a real failure with a name, and it is reported
//
// now preceding RecvTime means the two instants came from clocks that disagree
// — the exact defect §3.6 rule 5 widened its ban to engine/ to prevent. It is
// reported as ErrClockInversion rather than returned as a negative duration,
// because a negative age compares as "younger than any budget" and would read
// as PERMANENTLY FRESH: a broken clock would silently satisfy the freshness
// gate that stale marks exist to trip.
func (w Watermark) Age(now time.Time) (time.Duration, error) {
	return age(now, w.RecvTime)
}

// DataAge is how old the venue's own view was, measured against that venue's
// server time (Clock.ServerNow).
//
// This is the other question: a venue-side feed that stalls while the transport
// stays healthy keeps RecvTime advancing and SourceTime frozen, so Age reads
// fresh for the whole outage. Measured against ServerNow rather than Now
// because the two clocks are unsynchronized and §3.5 forbids treating one as
// the other.
//
// serverNow carries an uncertainty band (clock.ServerTime); this returns the
// age at the band's centre, and a caller for whom the band is material should
// widen it in the adverse direction itself rather than have this method guess
// which direction that is.
func (w Watermark) DataAge(serverNow time.Time) (time.Duration, error) {
	return age(serverNow, w.SourceTime)
}

// age is the one subtraction, written once so the inversion check cannot be
// present on one path and missing on the other.
func age(now, then time.Time) (time.Duration, error) {
	if then.IsZero() {
		return 0, ErrWatermarkUnset
	}
	if now.Before(then) {
		return 0, fmt.Errorf("now %s precedes %s: %w", now, then, ErrClockInversion)
	}
	return now.Sub(then), nil
}
