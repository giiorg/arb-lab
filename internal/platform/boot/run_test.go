package boot_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

var epoch = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

func testClockT(t *testing.T) *clock.TestClock {
	t.Helper()
	c, err := clock.NewTestClock(epoch, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

// recorder collects the order in which starts and stops actually ran, which is
// the only thing that distinguishes reverse teardown from any other order.
type recorder struct {
	events []string
}

func (r *recorder) note(s string) { r.events = append(r.events, s) }

// phase builds a phase whose Start and Stop both record, and whose Stop
// optionally consumes `takes` on the injected clock and returns `err`.
func (r *recorder) phase(name string, kind boot.PhaseKind, budget, takes time.Duration, clk *clock.TestClock, err error) boot.Phase {
	return boot.Phase{
		Name:       name,
		Kind:       kind,
		StopBudget: budget,
		Start: func(context.Context) (boot.StopFunc, error) {
			r.note("start:" + name)
			return func(context.Context) error {
				r.note("stop:" + name)
				if takes > 0 && clk != nil {
					if aerr := clk.Advance(takes); aerr != nil {
						return aerr
					}
				}
				return err
			}, nil
		},
	}
}

// failing builds a phase whose Start fails. Per the Phase.Start contract it
// returns a nil StopFunc, having cleaned up after itself.
func (r *recorder) failing(name string, kind boot.PhaseKind, err error) boot.Phase {
	return boot.Phase{
		Name:       name,
		Kind:       kind,
		StopBudget: time.Second,
		Start: func(context.Context) (boot.StopFunc, error) {
			r.note("start-failed:" + name)
			return nil, err
		},
	}
}

func joined(events []string) string { return strings.Join(events, " ") }

// TestRunStartsInOrder pins the forward half.
func TestRunStartsInOrder(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			r.phase("venue", boot.KindService, time.Second, 0, clk, nil),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "start:signal start:config start:obs start:venue start:reconcile"
	if got := joined(r.events); got != want {
		t.Errorf("start order = %q, want %q", got, want)
	}
	if got := joined(running.Names()); got != "signal config obs venue reconcile" {
		t.Errorf("Names() = %q, want start order", got)
	}
}

// TestShutdownRunsInReverse uses FIVE closers on purpose. With three, reversing
// the slice and swapping only its ends produce the same sequence, so a mutant
// that swapped ends would live — the fixture would not contain the thing under
// test.
func TestShutdownRunsInReverse(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			r.phase("venue", boot.KindService, time.Second, 0, clk, nil),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.events = nil

	report, err := running.Shutdown(context.Background(), clk)
	if err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	want := "stop:reconcile stop:venue stop:obs stop:config stop:signal"
	if got := joined(r.events); got != want {
		t.Errorf("stop order = %q, want %q", got, want)
	}
	if len(report.Phases) != 5 {
		t.Fatalf("report has %d phases, want 5", len(report.Phases))
	}
	// The report must be in the order the closers ran, not in start order:
	// it is read as a narrative of the teardown.
	if report.Phases[0].Name != "reconcile" || report.Phases[4].Name != "signal" {
		t.Errorf("report order = %s..%s, want reconcile..signal",
			report.Phases[0].Name, report.Phases[4].Name)
	}
}

// TestRunSkipsPhasesWithNoCloser puts the closer-less phase in the MIDDLE. At
// either end, "skipped" and "reversed into an end position" are hard to tell
// apart in the resulting order.
func TestRunSkipsPhasesWithNoCloser(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			// Nothing to close, so no budget is declared either.
			{Name: "obs", Kind: boot.KindObservability, Start: noopStart},
			r.phase("venue", boot.KindService, time.Second, 0, clk, nil),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := joined(running.Names()); got != "signal config venue reconcile" {
		t.Errorf("Names() = %q, want the closer-less phase absent", got)
	}
	r.events = nil
	if _, err := running.Shutdown(context.Background(), clk); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := joined(r.events); got != "stop:reconcile stop:venue stop:config stop:signal" {
		t.Errorf("stop order = %q, want the closer-less phase absent", got)
	}
}

// TestRunUnwindsOnFailure pins the failure path: only the phases that actually
// started are closed, and they are closed in reverse.
func TestRunUnwindsOnFailure(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	boom := errors.New("venue unreachable")
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			r.failing("venue", boot.KindService, boom),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if running != nil {
		t.Error("Run returned a Running alongside an error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want the phase's own error to survive wrapping", err)
	}

	var startErr *boot.StartError
	if !errors.As(err, &startErr) {
		t.Fatalf("Run() = %v, want a *StartError", err)
	}
	if startErr.Phase != "venue" {
		t.Errorf("StartError.Phase = %q, want %q", startErr.Phase, "venue")
	}
	if !strings.Contains(startErr.Error(), "venue") {
		t.Errorf("StartError.Error() = %q, does not name the phase", startErr.Error())
	}

	// The phase after the failure never started, and the three before it were
	// closed in reverse.
	want := "start:signal start:config start:obs start-failed:venue stop:obs stop:config stop:signal"
	if got := joined(r.events); got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
	if got := len(startErr.Unwind.Phases); got != 3 {
		t.Errorf("unwind report covers %d phases, want 3", got)
	}
	if startErr.Unwind.Phases[0].Name != "obs" {
		t.Errorf("unwind report starts at %q, want %q", startErr.Unwind.Phases[0].Name, "obs")
	}
}

// TestRunRefusesACloserWithNoBudget is checked at Run because that is where the
// fact exists: whether a phase has anything to close is unknown until Start has
// returned. The phase itself must still be closed by the unwind — it started
// successfully, and leaving it open would leak the thing the refusal is about.
func TestRunRefusesACloserWithNoBudget(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			// Declares no budget but returns a live closer.
			r.phase("venue", boot.KindService, 0, 0, clk, nil),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	_, err := s.Run(context.Background())
	if !errors.Is(err, boot.ErrStopBudgetUnset) {
		t.Fatalf("Run() = %v, want ErrStopBudgetUnset", err)
	}
	if !strings.Contains(err.Error(), "venue") {
		t.Errorf("error %q does not name the phase", err)
	}
	if !strings.Contains(joined(r.events), "stop:venue") {
		t.Errorf("events = %q: the offending phase was not closed by the unwind", joined(r.events))
	}
	if strings.Contains(joined(r.events), "start:reconcile") {
		t.Errorf("events = %q: the sequence continued past the refusal", joined(r.events))
	}
}

func TestRunValidatesFirst(t *testing.T) {
	t.Parallel()

	var r recorder
	s := boot.Sequence{
		Phases:          []boot.Phase{r.phase("config", boot.KindConfig, time.Second, 0, nil, nil)},
		WatchdogSilence: time.Minute,
	}
	if _, err := s.Run(context.Background()); !errors.Is(err, boot.ErrPhaseKindMissing) {
		t.Fatalf("Run() = %v, want the Validate refusal", err)
	}
	if len(r.events) != 0 {
		t.Errorf("events = %q, want nothing started before validation", joined(r.events))
	}
}

// TestShutdownContinuesPastAFailure is the "one failure does not abort the
// rest" claim. The failing closer sits in the MIDDLE of the teardown, so a
// shutdown that returned early would leave the phases below it unclosed — and
// reverse order means those are the foundational ones.
func TestShutdownContinuesPastAFailure(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	flushFailed := errors.New("journal flush failed")
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			r.phase("venue", boot.KindService, time.Second, 0, clk, flushFailed),
			r.phase("reconcile", boot.KindReconcile, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.events = nil

	report, err := running.Shutdown(context.Background(), clk)
	if err != nil {
		t.Fatalf("Shutdown returned %v; a closer failing is reported, not returned", err)
	}
	if got := joined(r.events); got != "stop:reconcile stop:venue stop:obs stop:config stop:signal" {
		t.Errorf("stop order = %q: the teardown stopped at the failure", got)
	}

	failures := report.Failures()
	if len(failures) != 1 || failures[0].Name != "venue" || !errors.Is(failures[0].Err, flushFailed) {
		t.Errorf("Failures() = %+v, want exactly the venue closer", failures)
	}
}

// TestPhaseStopOverranBoundary is the freeAt lesson on the other axis. Every
// natural fixture has a closer either comfortably inside or comfortably outside
// its budget, where > and >= are the same predicate. Exact equality is the one
// input that separates them, and it must NOT count as an overrun — a closer
// that sets its own timeout to its own budget lands there routinely, and an
// alert that fires on the intended behaviour gets turned off.
func TestPhaseStopOverranBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		takes time.Duration
		want  bool
	}{
		{name: "well inside the budget", takes: time.Second, want: false},
		{name: "one nanosecond inside", takes: 2*time.Second - 1, want: false},
		{name: "exactly at the budget", takes: 2 * time.Second, want: false},
		{name: "one nanosecond over", takes: 2*time.Second + 1, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := testClockT(t)
			var r recorder
			s := boot.Sequence{
				Phases: []boot.Phase{
					r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
					r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
					r.phase("obs", boot.KindObservability, 2*time.Second, tc.takes, clk, nil),
				},
				WatchdogSilence: time.Hour,
			}
			running, err := s.Run(context.Background())
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			report, err := running.Shutdown(context.Background(), clk)
			if err != nil {
				t.Fatalf("Shutdown: %v", err)
			}

			var obs boot.PhaseStop
			for _, p := range report.Phases {
				if p.Name == "obs" {
					obs = p
				}
			}
			if obs.Took != tc.takes {
				t.Fatalf("Took = %s, want %s", obs.Took, tc.takes)
			}
			if got := obs.Overran(); got != tc.want {
				t.Errorf("Overran() with Took=%s Budget=%s = %v, want %v",
					obs.Took, obs.Budget, got, tc.want)
			}
			overruns := report.Overruns()
			if tc.want && (len(overruns) != 1 || overruns[0].Name != "obs") {
				t.Errorf("Overruns() = %+v, want exactly the obs closer", overruns)
			}
			if !tc.want && len(overruns) != 0 {
				t.Errorf("Overruns() = %+v, want none", overruns)
			}
		})
	}
}

// TestShutdownRunsEveryCloserPastTheBudget is the design claim in the Shutdown
// doc: an early closer that blows the whole budget must not cause the later
// ones to be skipped, because reverse order makes those the foundational ones —
// the journal, the outbox, the log handler with records still buffered.
func TestShutdownRunsEveryCloserPastTheBudget(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("journal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			// Closes first and spends more than the whole sequence's budget.
			r.phase("hung-venue", boot.KindService, time.Second, time.Hour, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}

	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.events = nil

	report, err := running.Shutdown(context.Background(), clk)
	if err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := joined(r.events); got != "stop:hung-venue stop:obs stop:config stop:journal" {
		t.Errorf("stop order = %q: closers were skipped after the budget ran out", got)
	}
	if got, want := report.Took, time.Hour; got != want {
		t.Errorf("report.Took = %s, want %s", got, want)
	}
	// The overrun is reported rather than acted on: exactly one phase overran,
	// and the three that followed it did not become overruns by association.
	overruns := report.Overruns()
	if len(overruns) != 1 || overruns[0].Name != "hung-venue" {
		t.Errorf("Overruns() = %+v, want exactly the hung closer", overruns)
	}
}

func TestShutdownRefusals(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
		},
		WatchdogSilence: time.Minute,
	}
	running, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := running.Shutdown(context.Background(), nil); !errors.Is(err, boot.ErrClockNil) {
		t.Errorf("Shutdown with a nil clock = %v, want ErrClockNil", err)
	}
	// The refused call must not have run any closer, or the "nil clock" refusal
	// would be a partial teardown wearing an error.
	if len(r.events) != 3 {
		t.Errorf("events = %q, want nothing closed by the refused call", joined(r.events))
	}

	if _, err := running.Shutdown(context.Background(), clk); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	r.events = nil
	if _, err := running.Shutdown(context.Background(), clk); !errors.Is(err, boot.ErrAlreadyShutDown) {
		t.Errorf("second Shutdown = %v, want ErrAlreadyShutDown", err)
	}
	if len(r.events) != 0 {
		t.Errorf("events = %q, want no closer run twice", joined(r.events))
	}
}

// TestRunUnwindsOnlyStartedPhases closes the other door on the failure path: a
// phase whose Start never ran has nothing to close, and closing it would call a
// StopFunc for state that was never acquired.
func TestRunUnwindsOnlyStartedPhases(t *testing.T) {
	t.Parallel()

	clk := testClockT(t)
	var r recorder
	s := boot.Sequence{
		Phases: []boot.Phase{
			r.phase("signal", boot.KindSignal, time.Second, 0, clk, nil),
			r.phase("config", boot.KindConfig, time.Second, 0, clk, nil),
			r.phase("obs", boot.KindObservability, time.Second, 0, clk, nil),
			r.failing("venue", boot.KindService, errors.New("nope")),
		},
		WatchdogSilence: time.Minute,
	}
	if _, err := s.Run(context.Background()); err == nil {
		t.Fatal("Run: want an error")
	}
	if got := strings.Count(joined(r.events), "stop:"); got != 3 {
		t.Errorf("closers run = %d, want 3", got)
	}
}
