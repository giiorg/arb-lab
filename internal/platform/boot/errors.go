package boot

import "errors"

var (
	// ErrPostureInvalid reports a Posture outside the declared ladder.
	ErrPostureInvalid = errors.New("boot: posture invalid")

	// ErrActInvalid reports an Act outside the declared set.
	ErrActInvalid = errors.New("boot: act invalid")

	// ErrSignalActionInvalid reports a SignalAction outside the declared set.
	// Separate from ErrActInvalid because an Act is a venue write and a
	// SignalAction is a process-lifecycle decision — one sentinel covering both
	// could not tell an operator which enum was corrupted.
	ErrSignalActionInvalid = errors.New("boot: signal action invalid")

	// ErrGateInvalid reports a Gate outside §7.5's named preconditions.
	ErrGateInvalid = errors.New("boot: gate invalid")

	// ErrOutcomeInvalid reports an Outcome outside the declared set.
	ErrOutcomeInvalid = errors.New("boot: outcome invalid")

	// ErrChecklistEmpty reports a checklist that requires nothing.
	//
	// Refused rather than treated as "no preconditions", because the two are
	// indistinguishable at the call site and only one of them is what anybody
	// meant. A boot that asserts nothing before trading resumes is §7.5 not
	// implemented, and it would otherwise present as a clean start.
	ErrChecklistEmpty = errors.New("boot: checklist requires no gates")

	// ErrGateDuplicate reports the same gate required twice in one checklist.
	ErrGateDuplicate = errors.New("boot: gate required twice")

	// ErrGateUndeclared reports an outcome filed against a gate this checklist
	// never required.
	//
	// An error rather than a silent accept: a report against an undeclared gate
	// is a caller that believes it has satisfied a precondition, and accepting
	// it would leave the gate it meant to satisfy sitting at Unreported while
	// the caller has no way to notice.
	ErrGateUndeclared = errors.New("boot: gate not required by this checklist")

	// ErrGateLatched reports an attempt to move a gate out of
	// OutcomeNeedsOperator by filing a fresh outcome.
	//
	// §7.4 is explicit that return to normal operation requires explicit
	// operator clearance and a fresh propagation test — "never health-flap
	// re-entry". A gate that needs an operator therefore only moves through
	// Clear, and Clear returns it to Unreported rather than to Passed: the
	// clearance restores the ability to pass, not the pass.
	ErrGateLatched = errors.New("boot: gate is latched pending operator clearance")

	// ErrGateNotLatched reports a Clear against a gate that is not awaiting an
	// operator. Clearing something that was never blocked is a typo with no
	// effect, which is the shape of a control an operator believes worked.
	ErrGateNotLatched = errors.New("boot: gate is not awaiting operator clearance")

	// ErrPhaseKindInvalid reports a phase whose Kind is unset or unknown.
	ErrPhaseKindInvalid = errors.New("boot: phase kind invalid")

	// ErrPhaseNameUnset reports a phase with no name. The name is what the
	// shutdown report and every boot failure message identify a phase by, so an
	// unnamed phase turns both into a position in a slice.
	ErrPhaseNameUnset = errors.New("boot: phase name unset")

	// ErrPhaseNameDuplicate reports two phases sharing a name, which makes the
	// shutdown report ambiguous about which one overran.
	ErrPhaseNameDuplicate = errors.New("boot: phase name used twice")

	// ErrPhaseStartUnset reports a phase with no Start function.
	ErrPhaseStartUnset = errors.New("boot: phase has no Start")

	// ErrPhaseKindMissing reports a required phase kind absent from the
	// sequence. See the Sequence doc for why signal, config and observability
	// are required rather than optional, and for the falsifiable claim that
	// carries.
	ErrPhaseKindMissing = errors.New("boot: required phase kind absent")

	// ErrPhaseKindRepeated reports a kind present more than once where the
	// ordering constraints admit exactly one.
	ErrPhaseKindRepeated = errors.New("boot: phase kind repeated")

	// ErrPhaseOrder reports a sequence whose phases violate one of §3.2's
	// ordering constraints. The message names both phases and the rule.
	ErrPhaseOrder = errors.New("boot: phase order violates §3.2")

	// ErrStopBudgetNegative reports a phase declaring a negative stop budget.
	ErrStopBudgetNegative = errors.New("boot: stop budget negative")

	// ErrStopBudgetUnset reports a phase that returned a live Stop while
	// declaring a zero stop budget.
	//
	// Checked at Run rather than Validate because that is where the fact
	// exists: whether a phase has anything to close is known only once Start
	// has returned. A closer with no budget is an unbounded shutdown, which is
	// the watchdog hazard the sequence budget exists to bound, so it is refused
	// at boot rather than discovered at exit.
	ErrStopBudgetUnset = errors.New("boot: phase returned a Stop with no budget")

	// ErrWatchdogSilenceUnset reports a sequence that never declared §7.4's
	// host-silence threshold. Unset is refused rather than read as unbounded,
	// on the standing rule that the safe direction for a forgotten field is a
	// refusal that names it.
	ErrWatchdogSilenceUnset = errors.New("boot: watchdog silence threshold unset")

	// ErrShutdownBudgetExceeded reports a sequence whose phases, added up,
	// could hold the process past the point at which the watchdog treats it as
	// a dead host.
	//
	// See the package doc: a shutdown that outlasts the silence threshold turns
	// an ordinary restart into a fence, a takeover and an operator-cleared
	// incident. It is a wiring fact, so it is refused at wiring time.
	ErrShutdownBudgetExceeded = errors.New("boot: shutdown budget reaches the watchdog silence threshold")

	// ErrSequenceNotRun reports shutdown or posture queries against a sequence
	// that never started.
	ErrSequenceNotRun = errors.New("boot: sequence has not run")

	// ErrSignalBufferTooSmall reports a signal channel that cannot hold the
	// signals the policy has to distinguish.
	//
	// os/signal never blocks on delivery: a send to a full channel is dropped
	// with nothing reported. The second interrupt — the one that means "stop
	// waiting and die" — is exactly the one that arrives while the first is
	// still being handled, so an unbuffered channel drops precisely the signal
	// an operator presses twice to send.
	ErrSignalBufferTooSmall = errors.New("boot: signal channel buffer below the policy minimum")

	// ErrClockNil reports shutdown accounting wired without a clock.
	ErrClockNil = errors.New("boot: clock is nil")
)
