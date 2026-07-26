package boot

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// SignalAction is what the process should do about a signal it received.
//
// The policy is a value rather than a callback so it can be unit-tested without
// sending a real signal to a real process, and so the decision appears in the
// dispatcher's own loop instead of on whatever goroutine the runtime happened
// to deliver on.
type SignalAction uint8

const (
	// SignalIgnore is the zero value: this signal means nothing to the policy.
	//
	// Safe as a default, and that direction is worth stating. The alternative
	// default — treat an unrecognised signal as a shutdown request — would
	// have the process tear down its venue connections on a SIGWINCH from a
	// resized terminal.
	SignalIgnore SignalAction = iota

	// SignalReloadConfig is SIGHUP: re-read configuration and apply the fields
	// §5.2's config:"hot" tag admits. It is not a shutdown, and it is in this
	// policy rather than the config package because the signal disposition is
	// one thing and installing two of them fights.
	SignalReloadConfig

	// SignalGracefulShutdown is the first interrupt: begin the reverse-order
	// teardown, within the sequence's budget.
	SignalGracefulShutdown

	// SignalImmediateHalt is a subsequent interrupt: stop waiting.
	//
	// This is the enforcement Shutdown cannot perform on a closer that ignores
	// its context, and the reason it is safe to hand an operator is worth
	// stating, because the instinct in a trading system runs the other way.
	// Host death is a COVERED state: §7.4's off-host actuator exists precisely
	// to flatten exposure after the bot host and its journal die. A process
	// that is alive but wedged in shutdown is NOT covered — the watchdog reads
	// liveness, so a wedged process is one the watchdog will not escalate on
	// and the operator cannot fix. Between a covered failure and an uncovered
	// one, the second interrupt should produce the covered one.
	SignalImmediateHalt
)

const maxSignalAction = SignalImmediateHalt

// Validate reports whether a names an action. The zero value is legal:
// "ignore" is a real answer and the one most signals get.
func (a SignalAction) Validate() error {
	if a > maxSignalAction {
		return fmt.Errorf("signal action %d: %w", uint8(a), ErrSignalActionInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (a SignalAction) String() string {
	switch a {
	case SignalIgnore:
		return "SignalIgnore"
	case SignalReloadConfig:
		return "SignalReloadConfig"
	case SignalGracefulShutdown:
		return "SignalGracefulShutdown"
	case SignalImmediateHalt:
		return "SignalImmediateHalt"
	}
	return fmt.Sprintf("SignalAction(%d)", uint8(a))
}

// Watched is the set of signals the policy has an opinion about, and therefore
// the set Install subscribes to.
//
// SIGINT and SIGTERM both mean stop — an operator's interrupt and a
// supervisor's polite termination — and the policy deliberately does not
// distinguish them, because a container runtime sends SIGTERM and then SIGKILL
// on its own schedule, and treating the two stop signals differently would make
// the graceful path depend on which one arrived.
func Watched() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
}

// SignalBufferMin is the smallest channel buffer Install accepts.
//
// os/signal never blocks on delivery: a send to a full channel is DROPPED, with
// nothing returned and nothing logged. That turns the buffer size into a
// correctness property rather than a tuning knob, and it fails in the worst
// possible place — the second interrupt is by construction the one that arrives
// while the first is still being processed, so an unbuffered channel drops
// exactly the signal an operator presses twice to send, and the wedged shutdown
// this policy's escape hatch exists for is the case where it goes missing.
//
// Two is the minimum the policy needs to distinguish: one interrupt in hand,
// one queued behind it.
const SignalBufferMin = 2

// Install subscribes ch to the watched signals and returns the unsubscribe.
//
// It refuses a channel that cannot hold SignalBufferMin, which is the only
// reason this wrapper exists rather than a direct signal.Notify at the call
// site: the standard library's silent-drop behaviour becomes a refusal that
// names the buffer.
//
// A nil channel needs no separate guard, and adding one would be unreachable
// code: cap of a nil channel is 0, so the buffer check below refuses it first —
// and "buffer 0, minimum 2" is a true statement about a nil channel rather than
// a misleading one. That matters more than tidiness here, because signal.Notify
// PANICS on a nil channel, so the refusal has to happen before it.
func Install(ch chan<- os.Signal) (func(), error) {
	if cap(ch) < SignalBufferMin {
		return nil, fmt.Errorf("signal channel buffer %d, minimum %d: %w",
			cap(ch), SignalBufferMin, ErrSignalBufferTooSmall)
	}
	signal.Notify(ch, Watched()...)
	return func() { signal.Stop(ch) }, nil
}

// SignalPolicy maps received signals to actions, and is the only stateful part:
// it remembers whether a shutdown has already been requested.
//
// Not safe for concurrent use — it is read from the one loop that consumes the
// signal channel.
type SignalPolicy struct {
	shutdownRequested bool
}

// Next returns what to do about sig, and records the transition.
//
// The escalation is ratcheting rather than counting: once a shutdown has been
// requested, EVERY subsequent stop signal is an immediate halt, not just the
// second. An operator who has pressed twice and seen nothing happen presses
// again, and a policy that counted to exactly two would answer the third press
// with the graceful path it has already failed to complete.
//
// A SIGHUP does not disturb the ratchet in either direction: it is not a stop
// signal, so it neither arms the escalation nor disarms it. A reload requested
// during a teardown is answered as a reload, which the caller is free to
// decline — this policy says what a signal MEANS, not what the process is in a
// position to do about it.
func (p *SignalPolicy) Next(sig os.Signal) SignalAction {
	switch sig {
	case syscall.SIGHUP:
		return SignalReloadConfig
	case syscall.SIGINT, syscall.SIGTERM:
		if p.shutdownRequested {
			return SignalImmediateHalt
		}
		p.shutdownRequested = true
		return SignalGracefulShutdown
	}
	return SignalIgnore
}

// ShutdownRequested reports whether a stop signal has already been seen, which
// is what a caller checks before starting a teardown it may already be running.
func (p *SignalPolicy) ShutdownRequested() bool { return p.shutdownRequested }
