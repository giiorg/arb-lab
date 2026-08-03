package domain

import "fmt"

// Classification is §6.7's verdict on one hedge group's residual — the states of
// SM §2.
//
// # It is `type X string` for OrderState's reason, one machine over
//
// §3.6 rule 8's shape is for enums crossing persistence or wire, and this one
// crosses §7.5's reconcile-only boot: the verdict decides whether new trading is
// held, and a boot that read the wrong constant after a renumbering would resume
// trading on a group whose exposure is still uncertain. Rule 10 then makes every
// switch over it name all three.
//
// # The three are NOT a severity ladder, and the middle one is the safe answer
//
// Determined is the verdict that UNBLOCKS a compensating trade. Ambiguous is the
// one that holds it. So the ordering that matters is "which way does an
// uncertainty resolve", and it resolves toward Ambiguous every time: a group
// whose intervals have not provably collapsed is ambiguous, an order whose state
// cannot be read is ambiguous, an arithmetic that cannot be performed is an
// error and not a verdict at all.
type Classification string

const (
	// ClassificationDetermined is §6.7's determined residual: "the exposure is
	// fixed because no exposure-changing order on the hedge group has any
	// remaining executable OR UNOBSERVED quantity".
	//
	// Fixed does not mean zero. A group certainly short by exactly 1 is
	// determined, and it is exactly the case the ExposureReduction branch
	// exists for.
	ClassificationDetermined Classification = "determined"

	// ClassificationAmbiguous is §6.7's ambiguous residual: "any counter-leg
	// exposure-changing order still has remaining executable quantity or an
	// unresolved state, so the SIGN OR SIZE of the true net exposure is not yet
	// fixed — each such order contributes a possible-delta INTERVAL, not a
	// point".
	//
	// §6.7's consequence is the whole reason the verdict exists: "acting on the
	// observed delta is then wrong in at least one reachable venue state", so
	// every such order is quiesced to finality FIRST and no compensating trade
	// is issued in the meantime.
	ClassificationAmbiguous Classification = "ambiguous"

	// ClassificationDeadlineExceededAmbiguous is the branch where the finality
	// contract "cannot complete within its deadline (queries
	// unavailable/rate-limited)": a state that "does NOT blind-compensate — it
	// holds new trading, alerts the operator, and permits only provably-safe
	// action".
	//
	// It is not terminal. §6.7's own row has the intervals later collapsing back
	// to Determined, and model (a)'s NC14 — a latched deadline verdict — is a
	// checked temporal violation rather than a hypothetical: latched, the
	// determined branch is barred for ever from compensating a residual that has
	// become exactly knowable.
	//
	// The latitude it permits exists "because the bot is the SOLE WRITER: this
	// is an ambiguous ORDER, not an ambiguous WRITER" — the explicit contrast
	// with §7.4's fence-unavailable branch, where writer identity is in doubt and
	// nothing above $1K may act at all.
	ClassificationDeadlineExceededAmbiguous Classification = "deadline_exceeded_ambiguous"
)

// Validate reports whether c is one of the declared verdicts.
func (c Classification) Validate() error {
	switch c {
	case ClassificationDetermined, ClassificationAmbiguous, ClassificationDeadlineExceededAmbiguous:
		return nil
	case "":
		return ErrClassificationUnset
	}
	return fmt.Errorf("%q: %w", string(c), ErrClassificationUnknown)
}

func (c Classification) String() string { return string(c) }

// PermitsCompensation reports whether a verdict admits the ordinary
// ExposureReduction — the "single gate for every compensating trade".
//
// It is TRUE for exactly one verdict, and writing it as `c ==
// ClassificationDetermined` at each call site is the shape this exists to
// prevent: §6.7 routes six clients through this gate (one-leg failure,
// ADL/liquidation, kill-switch/panic-MARKET, protection-disarm,
// PackageDelever/EmergencyClosePackage, actuator takeover), and a gate spelled
// out six times is a gate five of them can drift from.
//
// The deadline-exceeded branch answers FALSE even though it permits the interval
// floor, and that asymmetry is §6.7's: the floor is "the SOLE exception to 'no
// compensation before finality', and only for the reachable-state-INVARIANT
// floor, never the uncertain part". A single boolean covering both would let a
// caller size an ordinary reduction from a deadline-exceeded group.
func (c Classification) PermitsCompensation() (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	switch c {
	case ClassificationDetermined:
		return true, nil
	case ClassificationAmbiguous, ClassificationDeadlineExceededAmbiguous:
		return false, nil
	}
	// Reachable only from a DECLARED verdict with no case above. Unconditional
	// rather than a trailing `return false`, on IsTerminal's argument: a trailing
	// return makes the false group's membership untestable, and here the false
	// group is where the safety lives.
	return false, fmt.Errorf("%q: %w", string(c), ErrClassificationUnhandled)
}

// Classify is §6.7's verdict rule over a hedge group's shifted possible-delta
// range.
//
// The range is the group's residual translated by the summed intervals of every
// order that still contributes one, so it collapses to a POINT exactly when
// §6.7's condition holds — "only when every possible-delta interval has
// collapsed to a fixed residual". That is the whole of the determined test, and
// it is deliberately a test on the RANGE rather than on a list of order states:
// the states are how each interval is derived, and re-deriving the verdict from
// them would be a second copy of a rule that already produced the range.
//
// deadlineExceeded is passed in rather than timed here because it is a §9 RTO
// (quiesce-to-finality < 30 s) measured by an injected clock one layer out;
// domain has no clock, by §3.6 rule 3 and by design.
//
// # Its precedence is "collapsed beats deadline", and the direction is checked
//
// A group whose intervals have collapsed is Determined even if the deadline blew
// while they were collapsing. §6.7's own row says so — "intervals later collapse
// (late resolution / fills / any lifecycle-finality event) → Determined; the
// determined branch then applies" — and the opposite precedence is model (a)'s
// NC14, whose signature is a machine stranded with a frozen verdict, every
// interval collapsed and dispatch budget in hand.
func Classify(rng PossibleDelta, deadlineExceeded bool) (Classification, error) {
	if err := rng.Validate(); err != nil {
		return "", err
	}
	collapsed, err := rng.IsPoint()
	if err != nil {
		return "", err
	}
	if collapsed {
		return ClassificationDetermined, nil
	}
	if deadlineExceeded {
		return ClassificationDeadlineExceededAmbiguous, nil
	}
	return ClassificationAmbiguous, nil
}
