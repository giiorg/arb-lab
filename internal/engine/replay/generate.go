package replay

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// Report is what a pump run turned out to be.
type Report struct {
	// Ingested counts tape entries consumed; Emitted counts envelopes
	// published from them. The difference is the no-news majority: chatter,
	// non-funding deltas, overlay applications.
	Ingested uint64
	Emitted  uint64

	// Dispatched counts sink deliveries (envelopes and timer firings — the
	// one dense sequence). Ordinary counts sink refusals the run's predicate
	// classified as expected outcomes (a supersede on an at-least-once
	// transport); each spent its stamp, so Dispatched includes them.
	Dispatched uint64
	Ordinary   uint64

	// RestFailures re-states the ingester's count for the run summary.
	RestFailures uint64
}

// Generate runs the arbitrating pump: the dispatcher decides the order and
// the sink chain (composed by the caller, LogWriter outermost when a log is
// wanted) freezes it. §39p-6's advance rule, stated as code:
//
//  1. Step until nothing is dispatchable — the rank drains due timers first
//     by construction, so timers due now precede everything.
//  2. If the earliest pending timer fires at or before the next record's
//     arrival, advance to the timer and resume stepping.
//  3. Otherwise move the clock to the next record's arrival and ingest the
//     whole same-instant batch (records the clock could not order — genuinely
//     concurrent, so contending them is fidelity, not fabrication).
//
// The sim's time domain is the archive's: after the last record, the loop
// drains what is due and stops; timers pending beyond the final instant stay
// unfired, because beyond the archive there is no world to re-judge.
//
// ordinary classifies sink errors that are expected outcomes of the transport
// (counted, run continues); every other error stops the run. A nil predicate
// treats every sink error as fatal.
func Generate(t *Tape, ing *Ingester, c *clock.TestClock, lane *events.MarketDataLane, d *dispatch.Dispatcher, ordinary func(error) bool) (Report, error) {
	var rep Report
	i := 0
	for {
		if err := drain(d, ordinary, &rep); err != nil {
			return rep, err
		}
		if i >= t.Len() {
			break
		}

		next := t.entries[i].rec.RecvTime
		if at, ok := c.NextTimerAt(); ok && !at.After(next) {
			c.AdvanceToNextTimer()
			continue
		}

		if err := c.SetNow(next); err != nil {
			return rep, fmt.Errorf("replay: generate at entry %d: %w", i, err)
		}
		for i < t.Len() && t.entries[i].rec.RecvTime.Equal(next) {
			env, emitted, err := ing.Ingest(t.entries[i])
			i++
			rep.Ingested++
			if err != nil {
				return rep, err
			}
			if !emitted {
				continue
			}
			if err := lane.Publish(env); err != nil {
				return rep, fmt.Errorf("replay: generate publish %s: %w", env.Source, err)
			}
			rep.Emitted++
		}
	}
	rep.RestFailures = ing.RestFailures()
	return rep, nil
}

// drain steps the dispatcher dry, splitting sink errors by the predicate.
func drain(d *dispatch.Dispatcher, ordinary func(error) bool, rep *Report) error {
	for {
		ok, err := d.Step()
		if err != nil {
			// A sink error on a dispatched item spent its stamp (§39m-4); the
			// predicate decides whether the run survives it. A refusal
			// (ok == false) never has an ordinary reading — the dispatcher
			// refused the input itself, which in a sim means the wiring is
			// wrong.
			if ok && ordinary != nil && ordinary(err) {
				rep.Dispatched++
				rep.Ordinary++
				continue
			}
			return fmt.Errorf("replay: dispatch: %w", err)
		}
		if !ok {
			return nil
		}
		rep.Dispatched++
	}
}
