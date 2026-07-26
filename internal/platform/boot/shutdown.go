package boot

import (
	"context"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// PhaseStop is what happened when one phase's closer ran.
type PhaseStop struct {
	// Name is the phase's name.
	Name string

	// Budget is what the phase declared it needed.
	Budget time.Duration

	// Took is measured on the injected clock, so a backtest and a live run
	// account identically. Zero on the failure-path unwind, which does no
	// accounting.
	Took time.Duration

	// Err is what the closer returned.
	Err error
}

// Overran reports a closer that took longer than the phase declared.
//
// Strictly greater. A closer that finishes exactly at its budget spent what it
// asked for, and calling that an overrun would report one on every phase whose
// duration is dominated by a timeout it set to its own budget — which is the
// common case, and an alert that fires on the intended behaviour is one an
// operator turns off.
func (p PhaseStop) Overran() bool { return p.Took > p.Budget }

// ShutdownReport is the outcome of closing a sequence.
type ShutdownReport struct {
	// Phases in the order their closers ran, which is REVERSE start order.
	Phases []PhaseStop

	// Took is the whole teardown, measured on the injected clock. It is what
	// gets compared against §7.4's silence threshold after the fact, where
	// Sequence.Validate compares the declared budget before it.
	Took time.Duration
}

// Failures returns the phases whose closers returned an error.
func (r ShutdownReport) Failures() []PhaseStop {
	var failed []PhaseStop
	for _, p := range r.Phases {
		if p.Err != nil {
			failed = append(failed, p)
		}
	}
	return failed
}

// Overruns returns the phases that took longer than they declared.
func (r ShutdownReport) Overruns() []PhaseStop {
	var over []PhaseStop
	for _, p := range r.Phases {
		if p.Overran() {
			over = append(over, p)
		}
	}
	return over
}

// Shutdown closes every started phase in reverse, allotting each the budget it
// declared and accounting for what it actually took.
//
// # Every closer runs, including after the budget is gone
//
// The obvious design — stop calling closers once the total budget is spent —
// is wrong here, and wrong in a way that is specific to reverse order. Reverse
// order means the phases that run LAST are the ones that started FIRST, which
// are the foundational ones: the journal, the outbox, the log handler with
// buffered records still in it. So a shared budget drained by whichever venue
// client hangs first would skip precisely the closers whose work is the reason
// a graceful shutdown exists, and it would skip them silently.
//
// Skipping also buys nothing. This package cannot make the process exit — it
// has no os.Exit and no goroutine to abandon a wedged closer on — so a skipped
// closer does not shorten the wall clock by one millisecond. It only means the
// flush did not happen while the process sat there anyway. So every closer runs,
// one failure does not abort the rest, and the report says who overran and by
// how much.
//
// What actually bounds a shutdown that will not finish is elsewhere and it is
// deliberate: the operator's second interrupt (SignalImmediateHalt), and the
// supervisor's SIGKILL behind it. This function's job is to allot, to run, and
// to account honestly — not to pretend to an enforcement it cannot perform.
func (r *Running) Shutdown(ctx context.Context, clk clock.Clock) (ShutdownReport, error) {
	if r.done {
		return ShutdownReport{}, ErrAlreadyShutDown
	}
	if clk == nil {
		return ShutdownReport{}, ErrClockNil
	}

	report := ShutdownReport{Phases: make([]PhaseStop, 0, len(r.started))}
	began := clk.Now()

	for i := len(r.started) - 1; i >= 0; i-- {
		p := r.started[i]
		at := clk.Now()
		err := p.stop(ctx)
		report.Phases = append(report.Phases, PhaseStop{
			Name:   p.name,
			Budget: p.budget,
			Took:   clk.Now().Sub(at),
			Err:    err,
		})
	}

	report.Took = clk.Now().Sub(began)
	r.started = nil
	r.done = true
	return report, nil
}
