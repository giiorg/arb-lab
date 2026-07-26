package boot

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrAlreadyShutDown reports a second Shutdown against the same Running.
//
// An error rather than a silent no-op: closers are not required to be
// idempotent, and a venue client closed twice or a file handle released twice
// is a defect in the caller's shutdown path that a quiet second call would
// hide. Reporting it costs nothing and names the mistake.
var ErrAlreadyShutDown = errors.New("boot: sequence already shut down")

// startedPhase is a phase that returned from Start without error, together with
// what shutdown needs to close it.
type startedPhase struct {
	name   string
	budget time.Duration
	stop   StopFunc
}

// Running is a sequence that started. It holds the phases in START order;
// Shutdown reverses them.
type Running struct {
	started []startedPhase
	done    bool
}

// StartError reports a phase that failed to start, together with what happened
// while the phases before it were backed out.
//
// The two are separate fields rather than a joined error because they answer
// different questions and one of them is the diagnosis. A boot that failed in
// the journal join and then also failed to close its venue client has one cause
// and one consequence, and flattening them into a list makes an operator guess
// which is which.
type StartError struct {
	// Phase is the name of the phase whose Start returned an error.
	Phase string

	// Err is what that Start returned.
	Err error

	// Unwind is the result of closing the phases that had already started, in
	// reverse. Its Failures are secondary: they happened because of Err, not
	// alongside it.
	Unwind ShutdownReport
}

func (e *StartError) Error() string {
	return fmt.Sprintf("boot: phase %q failed to start: %v", e.Phase, e.Err)
}

// Unwrap exposes the underlying cause, so errors.Is against a phase's own
// sentinel works through the wrapper.
func (e *StartError) Unwrap() error { return e.Err }

// Run validates the sequence and starts its phases in order.
//
// On the first failure it backs out every phase that had already started, in
// reverse, and returns a *StartError carrying both the cause and the unwind
// report. The unwind is unbounded by any clock: it runs on the context it was
// given, because a boot that is failing has no exposure to protect yet and
// nothing about the watchdog bound applies to a process that never began
// trading.
func (s Sequence) Run(ctx context.Context) (*Running, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	r := &Running{started: make([]startedPhase, 0, len(s.Phases))}

	for _, p := range s.Phases {
		stop, err := p.Start(ctx)
		if err != nil {
			return nil, &StartError{Phase: p.Name, Err: err, Unwind: r.unwind(ctx)}
		}
		if stop == nil {
			continue
		}
		r.started = append(r.started, startedPhase{name: p.Name, budget: p.StopBudget, stop: stop})
		if p.StopBudget <= 0 {
			err := fmt.Errorf("phase %q: %w", p.Name, ErrStopBudgetUnset)
			return nil, &StartError{Phase: p.Name, Err: err, Unwind: r.unwind(ctx)}
		}
	}
	return r, nil
}

// Names returns the started phases that have a closer, in start order.
func (r *Running) Names() []string {
	names := make([]string, 0, len(r.started))
	for _, p := range r.started {
		names = append(names, p.name)
	}
	return names
}

// unwind closes every started phase in reverse with no time accounting, and is
// what the failure path uses. It shares no code with Shutdown deliberately:
// Shutdown's whole content is the budget accounting, and the failure path has
// no budget to account against.
//
// It does not mark the Running done, and does not need to: every caller is a
// Run failure path, which returns a nil *Running, so nothing can reach this
// object again. Setting the flag would be unobservable state kept for the look
// of the thing — and a mutation harness would report it as a surviving mutant,
// correctly.
func (r *Running) unwind(ctx context.Context) ShutdownReport {
	var report ShutdownReport
	for i := len(r.started) - 1; i >= 0; i-- {
		p := r.started[i]
		report.Phases = append(report.Phases, PhaseStop{
			Name:   p.name,
			Budget: p.budget,
			Err:    p.stop(ctx),
		})
	}
	return report
}
