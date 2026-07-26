package boot_test

import (
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/boot"
)

// recvSignal is a BOUNDED receive, and the bound is not defensive politeness.
// A test that waits forever for a signal that will never arrive does not fail —
// it hangs, wedging the whole run until the outer timeout kills it, and a
// hanging test names no defect. The bite-check found this the hard way: two
// mutations that broke the subscription (dropping SIGHUP from Watched, and
// removing the signal.Notify call outright) both reported TIMEOUT!! rather than
// a kill, so the two claims this file makes about Install actually subscribing
// were unchecked while looking covered.
//
// Five seconds because the only thing being waited on is the kernel delivering
// a signal this process just sent itself; anything approaching that bound is a
// broken subscription, not a slow machine.
func recvSignal(t *testing.T, ch <-chan os.Signal, what string) os.Signal {
	t.Helper()
	select {
	case sig := <-ch:
		return sig
	case <-time.After(5 * time.Second):
		t.Fatalf("no signal delivered on %s within 5s", what)
		return nil
	}
}

func TestSignalActionValidateAndString(t *testing.T) {
	t.Parallel()

	// The zero value is legal: "ignore" is a real answer and the one most
	// signals get.
	for _, a := range []boot.SignalAction{
		boot.SignalIgnore, boot.SignalReloadConfig,
		boot.SignalGracefulShutdown, boot.SignalImmediateHalt,
	} {
		if err := a.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", a, err)
		}
		if s := a.String(); strings.HasPrefix(s, "SignalAction(") {
			t.Errorf("SignalAction(%d).String() = %q, want a name", uint8(a), s)
		}
	}
	if err := boot.SignalAction(4).Validate(); !errors.Is(err, boot.ErrSignalActionInvalid) {
		t.Errorf("SignalAction(4).Validate() = %v, want ErrSignalActionInvalid", err)
	}
	if got := boot.SignalAction(4).String(); got != "SignalAction(4)" {
		t.Errorf("SignalAction(4).String() = %q, want %q", got, "SignalAction(4)")
	}
}

func TestWatchedCoversThePolicy(t *testing.T) {
	t.Parallel()

	// Every signal the policy has an opinion about must be subscribed to, or
	// the opinion never runs — the process dies on the default disposition and
	// the policy is dead code that looks live.
	want := map[os.Signal]bool{
		syscall.SIGINT: false, syscall.SIGTERM: false, syscall.SIGHUP: false,
	}
	for _, s := range boot.Watched() {
		if _, ok := want[s]; !ok {
			t.Errorf("Watched() subscribes to %v, which the policy ignores", s)
		}
		want[s] = true
	}
	for s, covered := range want {
		if !covered {
			t.Errorf("Watched() omits %v, which the policy acts on", s)
		}
	}

	// The returned slice must be a fresh one: a caller that appended to it
	// would otherwise be extending the package's own subscription set.
	a, b := boot.Watched(), boot.Watched()
	a[0] = syscall.SIGUSR1
	if b[0] == syscall.SIGUSR1 {
		t.Error("Watched() returns shared backing storage")
	}
}

// TestSignalPolicyRatchet varies the fixture dimension that would otherwise
// make "escalate on the second" and "escalate on every one after the first"
// identical: it sends a THIRD interrupt. An operator who has pressed twice and
// seen nothing happen presses again, and a policy that counted to exactly two
// would answer that press with the graceful path it has already failed to
// complete.
func TestSignalPolicyRatchet(t *testing.T) {
	t.Parallel()

	var p boot.SignalPolicy

	if p.ShutdownRequested() {
		t.Error("a fresh policy reports a shutdown already requested")
	}
	if got := p.Next(syscall.SIGINT); got != boot.SignalGracefulShutdown {
		t.Fatalf("first SIGINT = %s, want SignalGracefulShutdown", got)
	}
	if !p.ShutdownRequested() {
		t.Error("ShutdownRequested() = false after the first interrupt")
	}
	for i, sig := range []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGINT} {
		if got := p.Next(sig); got != boot.SignalImmediateHalt {
			t.Errorf("interrupt %d (%v) = %s, want SignalImmediateHalt", i+2, sig, got)
		}
	}
}

// TestSignalPolicyDoesNotDistinguishStopSignals pins the deliberate conflation.
// A container runtime sends SIGTERM and then SIGKILL on its own schedule, so a
// policy that treated the two stop signals differently would make the graceful
// path depend on which one happened to arrive first.
func TestSignalPolicyDoesNotDistinguishStopSignals(t *testing.T) {
	t.Parallel()

	for _, first := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		for _, second := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
			var p boot.SignalPolicy
			if got := p.Next(first); got != boot.SignalGracefulShutdown {
				t.Errorf("%v then %v: first = %s, want SignalGracefulShutdown", first, second, got)
			}
			if got := p.Next(second); got != boot.SignalImmediateHalt {
				t.Errorf("%v then %v: second = %s, want SignalImmediateHalt", first, second, got)
			}
		}
	}
}

// TestSignalPolicyHangupDoesNotDisturbTheRatchet checks both directions, which
// a fixture that only ever sends SIGHUP first — or only ever last — cannot.
// A reload must not arm the escalation, and must not disarm it either.
func TestSignalPolicyHangupDoesNotDisturbTheRatchet(t *testing.T) {
	t.Parallel()

	t.Run("hangup before any interrupt does not arm", func(t *testing.T) {
		t.Parallel()
		var p boot.SignalPolicy
		for i := 0; i < 3; i++ {
			if got := p.Next(syscall.SIGHUP); got != boot.SignalReloadConfig {
				t.Fatalf("SIGHUP %d = %s, want SignalReloadConfig", i, got)
			}
			if p.ShutdownRequested() {
				t.Fatal("SIGHUP armed the shutdown ratchet")
			}
		}
		if got := p.Next(syscall.SIGINT); got != boot.SignalGracefulShutdown {
			t.Errorf("SIGINT after three SIGHUPs = %s, want SignalGracefulShutdown", got)
		}
	})

	t.Run("hangup between interrupts does not disarm", func(t *testing.T) {
		t.Parallel()
		var p boot.SignalPolicy
		if got := p.Next(syscall.SIGTERM); got != boot.SignalGracefulShutdown {
			t.Fatalf("first SIGTERM = %s, want SignalGracefulShutdown", got)
		}
		if got := p.Next(syscall.SIGHUP); got != boot.SignalReloadConfig {
			t.Fatalf("SIGHUP mid-teardown = %s, want SignalReloadConfig", got)
		}
		if !p.ShutdownRequested() {
			t.Error("SIGHUP cleared the shutdown ratchet")
		}
		if got := p.Next(syscall.SIGTERM); got != boot.SignalImmediateHalt {
			t.Errorf("SIGTERM after a SIGHUP = %s, want SignalImmediateHalt", got)
		}
	})
}

// TestSignalPolicyIgnoresUnwatched pins the default direction. The alternative
// default — treat anything unrecognised as a stop — would tear down the venue
// connections on a SIGWINCH from a resized terminal.
func TestSignalPolicyIgnoresUnwatched(t *testing.T) {
	t.Parallel()

	var p boot.SignalPolicy
	for _, sig := range []os.Signal{syscall.SIGWINCH, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGCHLD} {
		if got := p.Next(sig); got != boot.SignalIgnore {
			t.Errorf("Next(%v) = %s, want SignalIgnore", sig, got)
		}
		if p.ShutdownRequested() {
			t.Fatalf("%v armed the shutdown ratchet", sig)
		}
	}
	// Positive control: the same policy still acts on a signal it watches, so
	// the ignores above are about the signal rather than a policy that has
	// stopped responding.
	if got := p.Next(syscall.SIGINT); got != boot.SignalGracefulShutdown {
		t.Errorf("SIGINT after the ignored signals = %s, want SignalGracefulShutdown", got)
	}
}

// TestInstallBufferBoundary is the boundary that matters, and it is a boundary
// precisely because os/signal DROPS a send to a full channel with nothing
// reported. A buffer of exactly the minimum must be accepted — a check written
// as <= rather than < would refuse the one size the constant declares to be
// sufficient.
func TestInstallBufferBoundary(t *testing.T) {
	t.Parallel()

	if boot.SignalBufferMin < 2 {
		t.Fatalf("SignalBufferMin = %d: the policy needs one signal in hand and one queued",
			boot.SignalBufferMin)
	}

	for _, size := range []int{0, boot.SignalBufferMin - 1} {
		ch := make(chan os.Signal, size)
		stop, err := boot.Install(ch)
		if !errors.Is(err, boot.ErrSignalBufferTooSmall) {
			t.Errorf("Install with buffer %d = %v, want ErrSignalBufferTooSmall", size, err)
		}
		if stop != nil {
			t.Errorf("Install with buffer %d returned a non-nil stop alongside its error", size)
		}
		if err != nil && !strings.Contains(err.Error(), "minimum") {
			t.Errorf("error %q does not report the minimum an operator has to meet", err)
		}
	}

	ch := make(chan os.Signal, boot.SignalBufferMin)
	stop, err := boot.Install(ch)
	if err != nil {
		t.Fatalf("Install with buffer %d = %v, want nil", boot.SignalBufferMin, err)
	}
	if stop == nil {
		t.Fatal("Install returned a nil stop")
	}
	stop()

	if _, err := boot.Install(nil); !errors.Is(err, boot.ErrSignalBufferTooSmall) {
		t.Errorf("Install(nil) = %v, want ErrSignalBufferTooSmall", err)
	}
}

// TestInstallDelivers is the end-to-end control: a subscription that refuses
// small buffers is worthless if it never actually subscribed. It also confirms
// the unsubscribe works, since a signal.Stop that did nothing would leave the
// channel live for the rest of the process.
//
// Not parallel: it manipulates the process-wide signal disposition and sends
// real signals to this process.
//
// The `guard` subscription is doing two jobs, and the first one is why the
// naive version of this test kills the test binary. SIGHUP's default
// disposition is TERMINATE, so the moment Install's unsubscribe runs, the
// second send below stops being an observation and becomes a kill — the exact
// failure the KindSignal ordering constraint exists to prevent, arriving
// through the test that documents it. Keeping a second subscriber alive holds
// the disposition off the default for the whole test.
//
// Its second job is to make the negative check mean something. Signal delivery
// is asynchronous, so a bare non-blocking read of ch would pass merely because
// nothing had arrived YET — a vacuous check that looks identical to a working
// unsubscribe. Receiving on guard first proves the signal was delivered to the
// process before ch is examined.
func TestInstallDelivers(t *testing.T) {
	guard := make(chan os.Signal, 4)
	signal.Notify(guard, syscall.SIGHUP)
	defer signal.Stop(guard)

	ch := make(chan os.Signal, boot.SignalBufferMin)
	stop, err := boot.Install(ch)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := self.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Signal: %v", err)
	}

	if got := recvSignal(t, guard, "guard"); got != syscall.SIGHUP {
		t.Fatalf("guard received %v, want SIGHUP", got)
	}
	got := recvSignal(t, ch, "the installed channel")
	if got != syscall.SIGHUP {
		t.Fatalf("received %v, want SIGHUP", got)
	}
	var p boot.SignalPolicy
	if action := p.Next(got); action != boot.SignalReloadConfig {
		t.Errorf("policy on the delivered signal = %s, want SignalReloadConfig", action)
	}

	stop()
	if err := self.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Signal after stop: %v", err)
	}
	if got := recvSignal(t, guard, "guard"); got != syscall.SIGHUP {
		t.Fatalf("guard received %v after the unsubscribe, want SIGHUP", got)
	}
	select {
	case sig := <-ch:
		t.Errorf("received %v after the unsubscribe", sig)
	default:
	}
}
