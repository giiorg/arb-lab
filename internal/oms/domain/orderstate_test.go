package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// allStates is every declared member of the fixed state set, in SM §1's order.
//
// Written out rather than derived, because a list derived from the code under
// test cannot notice a member the code forgot — which is the exact defect
// platform/events shipped for four slices with StreamOrders.
var allStates = []domain.OrderState{
	domain.StatePlanned,
	domain.StateDispatchAuthorized,
	domain.StateSubmitted,
	domain.StateSubmissionUnknown,
	domain.StateAcknowledged,
	domain.StatePartiallyFilled,
	domain.StateCancelPending,
	domain.StateCancelUnknown,
	domain.StateFilled,
	domain.StateCanceled,
	domain.StateRejected,
	domain.StateAbsentAfterFinality,
}

func TestOrderStateValidateAcceptsEveryDeclaredMember(t *testing.T) {
	t.Parallel()

	if got := len(allStates); got != 12 {
		t.Fatalf("the fixed state set has %d members, want 12 — §5.3 calls it fixed", got)
	}
	seen := make(map[domain.OrderState]bool, len(allStates))
	for _, s := range allStates {
		if seen[s] {
			t.Errorf("%q appears twice in the state set", s)
		}
		seen[s] = true
		if err := s.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", s, err)
		}
	}
}

func TestOrderStateValidateSeparatesUnsetFromUnknown(t *testing.T) {
	t.Parallel()

	// The two sentinels exist because they send a reader to different files, so
	// the test that matters is that they are not the same sentinel.
	if err := domain.OrderState("").Validate(); !errors.Is(err, domain.ErrOrderStateUnset) {
		t.Errorf(`Validate("") = %v, want ErrOrderStateUnset`, err)
	}
	// A near-miss rather than gibberish: "Filled" differs from the declared
	// "filled" only in case, which is what a hand-written journal row or a venue
	// status mapped without folding actually looks like.
	err := domain.OrderState("Filled").Validate()
	if !errors.Is(err, domain.ErrOrderStateUnknown) {
		t.Errorf(`Validate("Filled") = %v, want ErrOrderStateUnknown`, err)
	}
	if !strings.Contains(err.Error(), "Filled") {
		t.Errorf("Validate(%q) message %q does not name the value that failed", "Filled", err)
	}
	if errors.Is(err, domain.ErrOrderStateUnset) {
		t.Error("a misspelt state reports as unset; the two sentinels have collapsed")
	}
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	// Transcribed from §5.3's sentence naming the terminals, not from the code.
	terminal := map[domain.OrderState]bool{
		domain.StateFilled:              true,
		domain.StateCanceled:            true,
		domain.StateRejected:            true,
		domain.StateAbsentAfterFinality: true,
	}

	for _, s := range allStates {
		got, err := s.IsTerminal()
		if err != nil {
			t.Errorf("IsTerminal(%q) errored: %v", s, err)
			continue
		}
		if want := terminal[s]; got != want {
			t.Errorf("IsTerminal(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestIsTerminalRefusesRatherThanAnsweringForANonState(t *testing.T) {
	t.Parallel()

	// The point of the error return: without it, "" and a misspelt state join the
	// false group silently, and that group's membership stops being testable.
	for _, s := range []domain.OrderState{"", "Filled", "settled"} {
		if _, err := s.IsTerminal(); err == nil {
			t.Errorf("IsTerminal(%q) returned no error; the false group has absorbed a non-state", s)
		}
	}
}

// TestContributesPossibleDeltaIsNotTheNegationOfIsTerminal is the one this file
// exists for.
//
// §6.7's ambiguous set has seven members; there are eight non-terminal states.
// A fixture set drawn only from {a terminal, Acknowledged} cannot tell the two
// predicates apart, so the fixture that carries the whole claim is Planned —
// non-terminal AND contributing no interval.
func TestContributesPossibleDeltaIsNotTheNegationOfIsTerminal(t *testing.T) {
	t.Parallel()

	// Transcribed from §6.7's list of states that make a residual ambiguous.
	ambiguous := map[domain.OrderState]bool{
		domain.StateDispatchAuthorized: true,
		domain.StateSubmitted:          true,
		domain.StateSubmissionUnknown:  true,
		domain.StateAcknowledged:       true,
		domain.StatePartiallyFilled:    true,
		domain.StateCancelPending:      true,
		domain.StateCancelUnknown:      true,
	}
	if got := len(ambiguous); got != 7 {
		t.Fatalf("§6.7 names %d ambiguous states here, want 7", got)
	}

	disagreements := 0
	for _, s := range allStates {
		contributes, err := s.ContributesPossibleDelta()
		if err != nil {
			t.Errorf("ContributesPossibleDelta(%q) errored: %v", s, err)
			continue
		}
		if want := ambiguous[s]; contributes != want {
			t.Errorf("ContributesPossibleDelta(%q) = %v, want %v", s, contributes, want)
		}

		terminal, err := s.IsTerminal()
		if err != nil {
			t.Errorf("IsTerminal(%q) errored: %v", s, err)
			continue
		}
		if contributes == !terminal {
			continue
		}
		disagreements++
		if s != domain.StatePlanned {
			t.Errorf("ContributesPossibleDelta and !IsTerminal disagree at %q, "+
				"and Planned is the only state where they should", s)
		}
	}

	// The positive control on the whole claim: if this count is zero the two
	// predicates are the same function, and every assertion above passes anyway.
	if disagreements != 1 {
		t.Errorf("the predicates disagree on %d states, want exactly 1 (Planned); "+
			"at 0 they have collapsed into one function", disagreements)
	}
}

func TestContributesPossibleDeltaRefusesANonState(t *testing.T) {
	t.Parallel()

	for _, s := range []domain.OrderState{"", "acknowledged "} {
		if _, err := s.ContributesPossibleDelta(); err == nil {
			t.Errorf("ContributesPossibleDelta(%q) returned no error", s)
		}
	}
}

func TestOrderStateString(t *testing.T) {
	t.Parallel()

	if got := domain.StateDispatchAuthorized.String(); got != "dispatch_authorized" {
		t.Errorf("String() = %q, want %q", got, "dispatch_authorized")
	}
}
