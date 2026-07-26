// Package clock is the injected time seam (§3.5). Domain and application code
// reads time only from here, never time.Now() — enforced by the notimenow gate,
// not by convention.
//
// Two things live in this package, and they are separate on purpose.
//
// # Clock: the port
//
// Exactly the four methods build-spec §3.5 declares. Live wiring injects Wall;
// the backtester injects TestClock, which it drives. Strategy, risk, sizing, OMS
// and monitor code are identical across both, and that interchangeability IS the
// parity mechanism rather than a testing convenience.
//
// # TimerSource: the pump, deliberately NOT part of Clock
//
// Timers have to be delivered somewhere, and the obvious design — the Clock
// pushes each one into a callback as it comes due — is wrong here. §3.5 gives the
// dispatcher a single total order over "events *and* timers", so the dispatcher
// must decide when a timer is consumed relative to a market-data envelope. A
// Clock that pushed on its own goroutine would force the dispatcher to re-queue
// everything it was handed, and the arbitration that determines the replay key
// would happen in two places.
//
// So consumption is pull-based: NextDue hands over the earliest timer that is due,
// and the caller decides when to ask. That also means this package contains no
// goroutines and no delivery-path concurrency at all.
//
// What is deliberately NOT here: waking a live consumer when a timer comes due.
// A dispatcher blocked on a channel needs a signal, and that signal belongs to
// the dispatcher's select loop — which does not exist yet (§1 defers it: it needs
// platform/events and a persisted DispatchSeq). Building the wake side now would
// be guessing at a shape with nothing to check it against.
package clock

import (
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Venue is re-exported from platform/ids, where it now lives.
//
// It sat here while this was the only package that existed and needed it — §3.5
// puts a venue in ServerNow's signature, and the shared kernel may not import a
// bounded context (§3.6 rule 9), so engine/venue was never an option. It moved
// when platform/ids landed, because config, ratelimit and oms all need to name a
// venue without wanting a clock.
//
// The alias rather than a bare rename: every existing call site keeps compiling,
// and the two names are the same type rather than two types that convert.
//
// Note the direction of the dependency. clock imports ids, never the reverse —
// ids declares its own one-method TimeSource interface instead of importing this
// package, which is what keeps the two out of a cycle.
type Venue = ids.Venue

// TimerID names a timer for cancel-and-replace. It is a key rather than an
// enum-like, so §3.6 rule 8's Validate shape does not apply: the set of timer
// names is open, and the OMS will mint them per order.
type TimerID string

var (
	// ErrTimeUnset reports a zero time.Time where an instant was required. The
	// zero time is a real, distant instant (year 1), so accepting it would make
	// every boundary look long past rather than failing.
	ErrTimeUnset = errors.New("clock: instant is the zero time")

	// ErrTimeReversed reports an attempt to move a clock backwards. Time going
	// backwards breaks the monotonicity every timer comparison assumes, and in
	// a backtest it would silently replay a window twice.
	ErrTimeReversed = errors.New("clock: time would move backwards")

	// ErrVenueDuplicate reports the same venue calibrated twice at
	// construction, which is a wiring mistake with a silent winner otherwise.
	ErrVenueDuplicate = errors.New("clock: venue calibrated twice")
)

// Clock is the injected time port — build-spec §3.5, unchanged.
type Clock interface {
	// Now is engine time. Per-venue server offset and uncertainty are modelled
	// separately, by ServerNow.
	Now() time.Time

	// ServerNow is authoritative for funding-boundary decisions, and carries
	// the uncertainty band that makes those decisions three-valued. A venue
	// with no calibration yields an unset ServerTime: see ServerTime.Validate.
	ServerNow(Venue) ServerTime

	// SetTimer schedules id to come due at `at`, cancelling and replacing any
	// timer already registered under that id. Ties in `at` are broken by prio
	// and then by id, so the firing order is total.
	//
	// It returns nothing, per §3.5. A timer scheduled in the past is therefore
	// not an error — it is simply already due, which is the useful reading: a
	// funding boundary computed from a stale snapshot should fire at once, not
	// be silently dropped.
	SetTimer(id TimerID, at time.Time, prio uint8)

	// CancelTimer removes id. Cancelling an unknown id is a no-op, not an
	// error: the caller that cancels on every exit path should not have to know
	// whether it ever armed the timer.
	CancelTimer(id TimerID)
}

// Fired is a timer that has come due.
type Fired struct {
	ID TimerID

	// At is the instant the timer was SCHEDULED for, not the instant it was
	// delivered. Determinism depends on the difference: the scheduled time is
	// identical in a replay, the delivery time is not.
	At time.Time

	Prio uint8
}

// TimerSource is the consumption side of the clock's timer queue. It is separate
// from Clock because only one caller ever pumps it — the §3.5 dispatcher, which
// owns the total order — while every domain and application component holds a
// Clock.
type TimerSource interface {
	// NextDue removes and returns the earliest timer due at or before the
	// implementation's own Now, or reports false if none is due.
	//
	// "Earliest" is by (At, Prio, ID), which is a total order because ids are
	// unique. Draining in a loop therefore yields a deterministic sequence, and
	// that determinism is the whole point: it is what lets a replay reproduce
	// the order the live run consumed.
	NextDue() (Fired, bool)

	// Pending reports how many timers are registered, due or not. It exists for
	// assertions and for the leak check a long-running dispatcher wants —
	// timers that are armed and never cancelled are how a funding-boundary
	// scheduler grows without bound.
	Pending() int
}

// Calibration is a venue's server-clock model: how far its clock is from ours,
// and how well we know that.
type Calibration struct {
	Venue Venue

	// Offset is added to engine time to estimate the venue's server time. It is
	// signed: a venue behind us has a negative offset.
	Offset time.Duration

	// Uncertainty is the half-width of the band around that estimate. Zero is
	// allowed and means a claim of perfect knowledge — which is a strong claim,
	// and one no live calibration should make.
	Uncertainty time.Duration
}

func (c Calibration) validate() error {
	if err := c.Venue.Validate(); err != nil {
		return err
	}
	if c.Uncertainty < 0 {
		return fmt.Errorf("venue %q uncertainty %v: %w", c.Venue, c.Uncertainty, ErrUncertaintyNegative)
	}
	return nil
}

// calibrationTable is the shared per-venue lookup both implementations use, so
// that "an uncalibrated venue yields an unset ServerTime" is one behaviour with
// one definition rather than two that drift.
type calibrationTable map[Venue]Calibration

func newCalibrationTable(cals []Calibration) (calibrationTable, error) {
	table := make(calibrationTable, len(cals))
	for _, c := range cals {
		if err := c.validate(); err != nil {
			return nil, err
		}
		if _, dup := table[c.Venue]; dup {
			return nil, fmt.Errorf("venue %q: %w", c.Venue, ErrVenueDuplicate)
		}
		table[c.Venue] = c
	}
	return table, nil
}

// serverNow applies a venue's calibration to an engine instant.
//
// An unknown venue yields the zero ServerTime, which fails Validate and answers
// Uncertain to everything. The alternative — a zero offset and a zero band —
// would be indistinguishable from a venue calibrated as perfectly synchronised,
// which is the most confident answer this package can give and would be given
// on the strength of no data at all.
func (t calibrationTable) serverNow(now time.Time, v Venue) ServerTime {
	cal, ok := t[v]
	if !ok {
		return ServerTime{}
	}
	st, err := NewServerTime(now.Add(cal.Offset), cal.Uncertainty)
	if err != nil {
		return ServerTime{}
	}
	return st
}
