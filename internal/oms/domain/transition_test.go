package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// specEdges is SM §1's transition table transcribed edge by edge, plus the three
// edge groups this slice found the table silent about.
//
// # It is deliberately a different SHAPE from the implementation
//
// "Write the oracle before trusting the implementation" is this repo's rule, and
// an oracle that mirrors the code's structure shares the code's mistakes. The
// implementation is a switch returning a slice per source state; this is a flat
// list of `from->to` strings, so a transposed pair or a state written into the
// wrong case does not survive in both.
//
// Every entry cites the row it came from. The four groups marked OWED are the
// ones no row states — see transition.go for what each costs when absent.
var specEdges = []string{
	// | Planned | authorize dispatch | DispatchAuthorized |
	"planned->dispatch_authorized",

	// | DispatchAuthorized | send returns | Submitted |
	"dispatch_authorized->submitted",

	// OWED — core §6 step 6 states this edge in words ("A timeout/5xx moves the
	// order to SubmissionUnknown") while SM §1's only entry into that state
	// leaves Submitted, whose meaning a timeout does not satisfy.
	"dispatch_authorized->submission_unknown",

	// OWED — core §5.3's rules paragraph + SM §1's crash-recovery invariant +
	// FLAG-2's "applies equally to DispatchAuthorized-recovery resolution".
	"dispatch_authorized->acknowledged",
	"dispatch_authorized->partially_filled",
	"dispatch_authorized->filled",
	"dispatch_authorized->rejected",
	"dispatch_authorized->absent_after_finality",

	// OWED — the venue-initiated termination edge on the RECOVERY axis. The
	// v0.9.11 edit below added it for the two LIVE states and stopped there; §6
	// step 6 sends only marketable IOCs, so an order terminated unfilled is the
	// commonest thing a resolving query can find, and without this edge that
	// answer has no legal target at all.
	"dispatch_authorized->canceled",
	"submission_unknown->canceled",

	// | Submitted | ack / reject / timeout-5xx | Acknowledged / Rejected / SubmissionUnknown |
	"submitted->acknowledged",
	"submitted->rejected",
	"submitted->submission_unknown",

	// | SubmissionUnknown | query | Acknowledged / PartiallyFilled / Filled / Rejected / AbsentAfterFinality |
	"submission_unknown->acknowledged",
	"submission_unknown->partially_filled",
	"submission_unknown->filled",
	"submission_unknown->rejected",
	"submission_unknown->absent_after_finality",

	// | Acknowledged | fill | PartiallyFilled / Filled |  ·  | Acknowledged | cancel-req | CancelPending |
	"acknowledged->partially_filled",
	"acknowledged->filled",
	"acknowledged->cancel_pending",

	// OWED — venue-initiated termination. §6 step 6 sends only marketable-IOC;
	// SM §2 step 1: "an IOC is immediately terminal at the venue".
	"acknowledged->canceled",
	"partially_filled->canceled",

	// | PartiallyFilled | more fills | Filled |  ·  | PartiallyFilled | cancel-req | CancelPending |
	"partially_filled->filled",
	"partially_filled->cancel_pending",

	// | CancelPending | ambiguous cancel transport result | CancelUnknown |
	"cancel_pending->cancel_unknown",

	// | CancelPending / CancelUnknown | explicit venue-terminal cancel + final sweep | Canceled |
	"cancel_pending->canceled",
	"cancel_unknown->canceled",

	// | CancelPending / CancelUnknown | the cancel loses the race to a full fill | Filled |
	"cancel_pending->filled",
	"cancel_unknown->filled",

	// | AbsentAfterFinality | a discovered late execution surfaces | PartiallyFilled / Filled |
	"absent_after_finality->partially_filled",
	"absent_after_finality->filled",
}

// TestCanTransitionMatrix answers all 144 ordered pairs, so the edge set is
// asserted rather than sampled.
//
// A per-edge test would check only that the legal edges are legal, and every
// interesting failure in a state machine is the other direction: an edge that
// should not exist and does. Resubmitting from the crash boundary
// (dispatch_authorized -> planned) and entering Rejected on proved absence
// (absent_after_finality -> rejected) are both in the 115 pairs this refuses,
// and neither would appear on any list of edges to test.
func TestCanTransitionMatrix(t *testing.T) {
	t.Parallel()

	legal := make(map[string]bool, len(specEdges))
	for _, e := range specEdges {
		if legal[e] {
			t.Fatalf("the transcribed edge list contains %q twice", e)
		}
		legal[e] = true
	}
	if got := len(legal); got != 32 {
		t.Fatalf("transcribed %d edges, want 32 — the count is a check on the transcription", got)
	}

	permitted, refused := 0, 0
	for _, from := range allStates {
		for _, to := range allStates {
			if from == to {
				continue // the no-op rule has its own test
			}
			err := domain.CanTransition(from, to)
			key := from.String() + "->" + to.String()
			switch {
			case legal[key] && err != nil:
				t.Errorf("CanTransition(%s, %s) = %v, want nil (SM §1 has this edge)", from, to, err)
			case !legal[key] && err == nil:
				t.Errorf("CanTransition(%s, %s) = nil, want refusal (SM §1 has no such edge)", from, to)
			case !legal[key] && !errors.Is(err, domain.ErrTransitionForbidden):
				t.Errorf("CanTransition(%s, %s) = %v, want ErrTransitionForbidden", from, to, err)
			}
			if legal[key] {
				permitted++
			} else {
				refused++
			}
		}
	}

	// Positive control on the matrix itself. A predicate that objected to
	// everything would satisfy every assertion above if `permitted` were zero,
	// and one that permitted everything would if `refused` were.
	if permitted != 32 || refused != 12*12-12-32 {
		t.Errorf("walked %d permitted and %d refused pairs, want 32 and %d",
			permitted, refused, 12*12-12-32)
	}
}

// TestCanTransitionRefusesTheResubmitEdges names the two forbidden edges a
// plausible implementation actually writes, so the refusal is stated where a
// reader looks rather than only implied by the matrix.
func TestCanTransitionRefusesTheResubmitEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		from, to domain.OrderState
	}{
		{
			// §5.3: never resubmit, never infer "not sent" — the single Place may
			// have executed with its response lost.
			name: "the send never went out",
			from: domain.StateDispatchAuthorized,
			to:   domain.StatePlanned,
		},
		{
			// FLAG-2: proved absence and an explicit venue rejection are different
			// evidence classes.
			name: "proved absence read as a rejection",
			from: domain.StateAbsentAfterFinality,
			to:   domain.StateRejected,
		},
		{
			// The mirror: a rejection is explicit evidence, so nothing later
			// downgrades it to unproved absence.
			name: "a rejection downgraded to proved absence",
			from: domain.StateRejected,
			to:   domain.StateAbsentAfterFinality,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.CanTransition(tc.from, tc.to)
			if !errors.Is(err, domain.ErrTransitionForbidden) {
				t.Errorf("CanTransition(%s, %s) = %v, want ErrTransitionForbidden", tc.from, tc.to, err)
			}
			if !strings.Contains(err.Error(), tc.from.String()) ||
				!strings.Contains(err.Error(), tc.to.String()) {
				t.Errorf("message %q does not name both ends of the refused edge", err)
			}
		})
	}
}

// TestCanTransitionRefusesASelfEdgeFromBothKindsOfState covers the collapse the
// obvious fixture would hide.
//
// A self-edge test using only a terminal cannot distinguish "self-edges are
// refused" from "terminals have no successors" — both produce a refusal, and
// deleting the from == to guard would still fail the test for the right-looking
// wrong reason. Planned has a successor and Filled has none, so the pair
// separates the two rules.
func TestCanTransitionRefusesASelfEdgeFromBothKindsOfState(t *testing.T) {
	t.Parallel()

	for _, s := range []domain.OrderState{domain.StatePlanned, domain.StateFilled} {
		err := domain.CanTransition(s, s)
		if !errors.Is(err, domain.ErrTransitionNoOp) {
			t.Errorf("CanTransition(%s, %s) = %v, want ErrTransitionNoOp", s, s, err)
		}
	}
}

// TestCanTransitionValidatesBeforeTheNoOpRule pins the precedence, which is not
// cosmetic: "" -> "" is two unpopulated fields, and reporting it as a no-op
// would send a reader looking for the event that fired twice.
func TestCanTransitionValidatesBeforeTheNoOpRule(t *testing.T) {
	t.Parallel()

	if err := domain.CanTransition("", ""); !errors.Is(err, domain.ErrOrderStateUnset) {
		t.Errorf(`CanTransition("", "") = %v, want ErrOrderStateUnset`, err)
	}
	err := domain.CanTransition(domain.StatePlanned, "shipped")
	if !errors.Is(err, domain.ErrOrderStateUnknown) {
		t.Errorf("CanTransition(planned, shipped) = %v, want ErrOrderStateUnknown", err)
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("message %q does not say which end was invalid", err)
	}
	if err := domain.CanTransition("shipped", domain.StatePlanned); !strings.Contains(err.Error(), "source") {
		t.Errorf("message %q does not say which end was invalid", err)
	}
}

// TestTerminalStatesHaveNoSuccessors states the claim the (nil, nil) case in
// successors exists to make, in the one place a reader would look for it.
func TestTerminalStatesHaveNoSuccessors(t *testing.T) {
	t.Parallel()

	for _, from := range []domain.OrderState{
		domain.StateFilled, domain.StateCanceled, domain.StateRejected,
	} {
		for _, to := range allStates {
			if from == to {
				continue
			}
			if err := domain.CanTransition(from, to); !errors.Is(err, domain.ErrTransitionForbidden) {
				t.Errorf("CanTransition(%s, %s) = %v, want ErrTransitionForbidden", from, to, err)
			}
		}
	}

	// AbsentAfterFinality is terminal and is NOT in the list above, which is the
	// §3.0 rule that venue reality wins. Asserted here so that the list reads as
	// a decision rather than an oversight.
	for _, to := range []domain.OrderState{domain.StatePartiallyFilled, domain.StateFilled} {
		if err := domain.CanTransition(domain.StateAbsentAfterFinality, to); err != nil {
			t.Errorf("CanTransition(absent_after_finality, %s) = %v, want nil — "+
				"a discovered late execution supersedes proved absence", to, err)
		}
	}
}
