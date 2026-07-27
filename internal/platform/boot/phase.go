package boot

import (
	"context"
	"fmt"
	"time"
)

// PhaseKind is what a boot phase is FOR, and therefore where in the sequence it
// is allowed to sit.
//
// [SPEC EDIT LANDED — v0.9.6] §3.2 stated the roster — "signal, config, obs,
// reconcile-only-boot, graceful shutdown" — as a LIST, in a table cell, and
// never said that the positions are constraints or what breaks when they are
// violated. Two of them are load-bearing and both failures are silent: signal
// installed late means an interrupt during a slow boot kills the process on the
// default disposition with no flush, and observability started late means a
// boot that dies in the journal join dies with an exit code as its only
// diagnosis. The code turns the list into checked constraints with those
// reasons attached; §3.2 now states them, including that the roster stays open,
// rather than leaving a reader to infer that a table cell's comma order was
// deliberate.
//
// This enum exists so those constraints can be checked rather than
// reviewed. What it deliberately does NOT do is close the roster: KindService
// is an open slot, because §11 gives the three binaries genuinely different
// phase sets and a closed vocabulary would force a venue client or a WebSocket
// subscription into a bucket that misnames it.
type PhaseKind uint8

const (
	// KindUnspecified is the zero value and names no kind. Unusable: a phase
	// that did not say what it is cannot be placed, and defaulting it to the
	// open slot would silently exempt it from every ordering rule.
	KindUnspecified PhaseKind = iota

	// KindSignal installs the process's signal disposition. First, always.
	//
	// The constraint is load-bearing and it is the one most likely to be got
	// wrong, because signal handling reads like a detail to wire up once
	// everything else is running. A Go process's default disposition for
	// SIGINT and SIGTERM is to die, so a phase installed after a forty-second
	// journal join means an operator's interrupt during that join terminates
	// the process outright — no graceful shutdown, no flush, and no record that
	// a shutdown was ever requested. The window where a boot is slowest is
	// exactly the window where an operator is most likely to interrupt it.
	KindSignal

	// KindConfig loads and validates configuration. After signal, before
	// observability — the direction is forced, because the exporter's endpoint,
	// the log level and the sampling policy are config.
	KindConfig

	// KindObservability starts the metrics registry, the log handler and the
	// §3.4 lossy lane. Before anything that can fail interestingly.
	//
	// Second load-bearing constraint. Every phase after this one reports its
	// failure through the machinery this phase installs, so a sequence that
	// starts observability late does not merely lose some telemetry — a boot
	// that dies in the journal join dies SILENTLY, which is the failure an
	// operator diagnoses by reading an exit code.
	KindObservability

	// KindService is the open slot: venue clients, WebSocket subscriptions, the
	// operator API, anything a particular binary needs between observability
	// and reconciliation. Any number, in any relative order.
	KindService

	// KindReconcile is §7.5's reconcile-only boot: the write-ahead journal
	// joined to venue truth. Last, because it is the phase whose result decides
	// the posture, and a phase after it would be running while the process is
	// already reporting a verdict about whether it may trade.
	//
	// Optional rather than required. §11's binaries do not all reconcile — a
	// backtest run has no venue truth to join to — and requiring it would
	// force those to declare a phase that does nothing, which is how a required
	// check becomes a formality.
	KindReconcile
)

const maxPhaseKind = KindReconcile

// Validate reports whether k names a kind, excluding the sentinel.
func (k PhaseKind) Validate() error {
	if k == KindUnspecified || k > maxPhaseKind {
		return fmt.Errorf("phase kind %d: %w", uint8(k), ErrPhaseKindInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (k PhaseKind) String() string {
	switch k {
	case KindUnspecified:
		return "KindUnspecified"
	case KindSignal:
		return "KindSignal"
	case KindConfig:
		return "KindConfig"
	case KindObservability:
		return "KindObservability"
	case KindService:
		return "KindService"
	case KindReconcile:
		return "KindReconcile"
	}
	return fmt.Sprintf("PhaseKind(%d)", uint8(k))
}

// StopFunc releases what a phase acquired. A phase with nothing to release
// returns nil.
//
// It takes a context so the caller's shutdown bound reaches it. Note what that
// does and does not buy: this package can allot a budget and account for an
// overrun, but it cannot INTERRUPT a StopFunc that ignores its context, because
// doing so would need a goroutine per closer and this kernel has none (the same
// call platform/clock and platform/ratelimit made, for the same reason). The
// enforcement is elsewhere and it is deliberate: SignalImmediateHalt is what an
// operator uses on a shutdown that will not finish.
type StopFunc func(context.Context) error

// Phase is one unit of bootstrap work.
type Phase struct {
	// Name identifies the phase in every failure message and in the shutdown
	// report. Required, and unique within a sequence.
	Name string

	// Kind places the phase against §3.2's ordering constraints.
	Kind PhaseKind

	// Start acquires whatever the phase owns and returns the closer for it.
	//
	// Contract on the failure path: a Start that returns an error MUST have
	// released anything it acquired, and MUST return a nil StopFunc. The
	// alternative — the sequence closing a phase that failed halfway — leaves
	// every Start having to be safe against a close it may not have earned,
	// which is a contract nobody can check and everybody assumes differently.
	Start func(context.Context) (StopFunc, error)

	// StopBudget is how long this phase's closer is allotted at shutdown.
	//
	// Non-negative. A phase that returns a live StopFunc must declare a
	// positive one; see ErrStopBudgetUnset for why that is checked at Run
	// rather than here.
	StopBudget time.Duration
}

// Sequence is an ordered set of phases plus the one number that bounds their
// teardown.
type Sequence struct {
	// Phases in start order. Shutdown runs their closers in reverse.
	Phases []Phase

	// WatchdogSilence is §7.4's host-silence threshold: how long the process
	// may stop being audible before the watchdog escalates and the actuator
	// takes over.
	//
	// [SPEC EDIT LANDED — v0.9.6] §7.4 gave the watchdog its trigger and §7.5
	// gave the process a graceful shutdown, and nothing in the spec joined
	// them — but they are the same wall clock, and the join has a wrong side.
	// §7.4 now states the bound. A total stop
	// budget at or above this threshold means an ORDINARY RESTART presents to
	// the watchdog exactly as a dead host: it escalates, the actuator fences
	// the bot's signer mid-flush and latches a takeover epoch, and the process
	// that comes back up finds itself fenced and — per §7.4 — "stays in
	// reconcile-only until an operator clears it". A deployment becomes an
	// operator-cleared incident, and the cause is a sum of durations in a
	// wiring file that nothing was comparing.
	//
	// Required. Unset is refused rather than read as unbounded, on the standing
	// rule that a forgotten field should fail as a refusal that names it.
	WatchdogSilence time.Duration
}

// ShutdownBudget is the sum of the phases' stop budgets: the longest teardown
// this sequence can take without any closer overrunning its own allotment.
func (s Sequence) ShutdownBudget() time.Duration {
	var total time.Duration
	for _, p := range s.Phases {
		total += p.StopBudget
	}
	return total
}

// requiredKinds are the kinds every sequence must contain exactly one of.
//
// This carries a falsifiable claim, in the same spirit as notimenow's
// transport-only escape: the assertion is that every binary in this system is a
// long-running process that loads configuration and reports on itself, so all
// three are non-negotiable. If a binary lands that legitimately has none of one
// of them — and a genuinely one-shot tool would have no signal disposition
// worth installing — relax the rule rather than declaring a phase that does
// nothing, because a required check satisfied by a stub is worse than no check.
var requiredKinds = []PhaseKind{KindSignal, KindConfig, KindObservability}

// Validate checks the sequence against §3.2's ordering constraints and the
// watchdog bound.
//
// Every refusal names the phase or phases involved and the rule, rather than
// returning a bare sentinel. That is this repo's own lesson, paid for four
// times in one table in platform/config: one sentinel cannot say WHICH check
// fired, so a fixture that fails the check AFTER the one under test looks like
// a pass, and the mutant lives.
func (s Sequence) Validate() error {
	if len(s.Phases) == 0 {
		return fmt.Errorf("%w: %s", ErrPhaseKindMissing, KindSignal)
	}

	seen := make(map[string]struct{}, len(s.Phases))
	counts := make(map[PhaseKind]int, len(s.Phases))
	firstAt := make(map[PhaseKind]int, len(s.Phases))

	for i, p := range s.Phases {
		if p.Name == "" {
			return fmt.Errorf("phase %d: %w", i, ErrPhaseNameUnset)
		}
		if err := p.Kind.Validate(); err != nil {
			return fmt.Errorf("phase %q: %w", p.Name, err)
		}
		if p.Start == nil {
			return fmt.Errorf("phase %q: %w", p.Name, ErrPhaseStartUnset)
		}
		if p.StopBudget < 0 {
			return fmt.Errorf("phase %q: %w (%s)", p.Name, ErrStopBudgetNegative, p.StopBudget)
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("phase %q: %w", p.Name, ErrPhaseNameDuplicate)
		}
		seen[p.Name] = struct{}{}
		if counts[p.Kind] == 0 {
			firstAt[p.Kind] = i
		}
		counts[p.Kind]++
	}

	for _, k := range requiredKinds {
		if counts[k] == 0 {
			return fmt.Errorf("%w: %s", ErrPhaseKindMissing, k)
		}
		if counts[k] > 1 {
			return fmt.Errorf("%w: %s appears %d times", ErrPhaseKindRepeated, k, counts[k])
		}
	}
	if counts[KindReconcile] > 1 {
		return fmt.Errorf("%w: %s appears %d times", ErrPhaseKindRepeated, KindReconcile, counts[KindReconcile])
	}

	if err := s.validateOrder(firstAt, counts); err != nil {
		return err
	}

	if s.WatchdogSilence <= 0 {
		return fmt.Errorf("%w: %s", ErrWatchdogSilenceUnset, s.WatchdogSilence)
	}
	if budget := s.ShutdownBudget(); budget >= s.WatchdogSilence {
		return fmt.Errorf("%w: %s of stop budget against a %s silence threshold",
			ErrShutdownBudgetExceeded, budget, s.WatchdogSilence)
	}
	return nil
}

// validateOrder enforces the four positional constraints §3.2's roster implies.
func (s Sequence) validateOrder(firstAt map[PhaseKind]int, counts map[PhaseKind]int) error {
	if at := firstAt[KindSignal]; at != 0 {
		return fmt.Errorf("%w: %s at position %d must be first, %q precedes it",
			ErrPhaseOrder, KindSignal, at, s.Phases[0].Name)
	}
	if firstAt[KindConfig] > firstAt[KindObservability] {
		return fmt.Errorf("%w: %s must precede %s, which reads its endpoint from it",
			ErrPhaseOrder, KindConfig, KindObservability)
	}

	obs := firstAt[KindObservability]
	for i, p := range s.Phases {
		switch p.Kind {
		case KindService, KindReconcile:
			if i < obs {
				return fmt.Errorf("%w: %s %q at position %d precedes %s, so its failure is unreported",
					ErrPhaseOrder, p.Kind, p.Name, i, KindObservability)
			}
		case KindUnspecified, KindSignal, KindConfig, KindObservability:
			// The phases that may precede observability, named rather than
			// left to fall out of the switch (§3.6 rule 10). Signal and config
			// MUST come first, and observability cannot precede itself; the
			// zero value is refused by Validate before this loop runs. A phase
			// kind added to §3.2's open roster has to choose a side here, which
			// is the one decision this loop exists to make.
		}
	}

	if counts[KindReconcile] == 1 {
		if at := firstAt[KindReconcile]; at != len(s.Phases)-1 {
			return fmt.Errorf("%w: %s at position %d must be last, %q follows it",
				ErrPhaseOrder, KindReconcile, at, s.Phases[at+1].Name)
		}
	}
	return nil
}
