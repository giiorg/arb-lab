package application

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
)

// allEffects and allClasses are the declared members, written out rather than
// derived from apperror.
//
// Same argument the domain suite makes about allStates: a list built from the
// code under test cannot notice a member the code forgot. Here it is doubly
// worth it, because apperror does not export its declared sets at all, so the
// only alternative would be to derive them from the very switch under test.
var (
	allEffects = []apperror.Effect{
		apperror.EffectUnspecified,
		apperror.EffectNotApplied,
		apperror.EffectUnknown,
		apperror.EffectApplied,
	}
	allClasses = []apperror.Class{
		apperror.ClassTransport,
		apperror.ClassRateLimit,
		apperror.ClassAuth,
		apperror.ClassVenueReject,
		apperror.ClassNotFound,
		apperror.ClassPrecondition,
		apperror.ClassInternal,
	}
)

// TestClassifyEffectAnswersEveryDeclaredMember is why classifyEffect takes
// values rather than an error.
//
// Two of these four rows are unreachable through apperror's public API —
// New degrades any invalid Spec to EffectUnknown, so no constructed *Error can
// carry EffectUnspecified, and EffectOf answers it only for a nil error, which
// classifyPlace handles before the switch. Through the exported surface alone,
// deleting EffectUnspecified from its case group would change nothing any test
// could observe.
func TestClassifyEffectAnswersEveryDeclaredMember(t *testing.T) {
	t.Parallel()

	cases := []struct {
		effect apperror.Effect
		class  apperror.Class
		want   domain.OrderState
	}{
		{apperror.EffectApplied, apperror.ClassTransport, domain.StateSubmitted},
		{apperror.EffectNotApplied, apperror.ClassVenueReject, domain.StateRejected},
		{apperror.EffectNotApplied, apperror.ClassTransport, domain.StateSubmissionUnknown},
		{apperror.EffectUnknown, apperror.ClassTransport, domain.StateSubmissionUnknown},
		{apperror.EffectUnspecified, apperror.ClassTransport, domain.StateSubmissionUnknown},
	}

	for _, tc := range cases {
		state, err := classifyEffect(tc.effect, tc.class)
		if state != tc.want {
			t.Errorf("classifyEffect(%s, %s) state = %s, want %s",
				tc.effect, tc.class, state, tc.want)
		}
		// The positive control that pairs with the fallthrough test below:
		// a declared member must be answered by a RULE, never by the fallback.
		if err != nil {
			t.Errorf("classifyEffect(%s, %s) err = %v, want nil — a declared member is not the fallback",
				tc.effect, tc.class, err)
		}
	}
}

// TestClassifyEffectReadsTheClassOnlyForNotApplied pins the one row that took
// the argument, from the other side.
//
// EffectNotApplied is the ONLY effect whose answer depends on the class, and
// ClassVenueReject is the only class that changes it. Without both halves,
// "reads the class when it matters" is indistinguishable from "reads the class
// always" — and the second would send an applied-but-unacknowledged order to
// the Rejected terminal the moment a venue's 5xx carried a reject code.
func TestClassifyEffectReadsTheClassOnlyForNotApplied(t *testing.T) {
	t.Parallel()

	for _, class := range allClasses {
		state, err := classifyEffect(apperror.EffectNotApplied, class)
		if err != nil {
			t.Fatalf("classifyEffect(EffectNotApplied, %s) err = %v", class, err)
		}
		want := domain.StateSubmissionUnknown
		if class == apperror.ClassVenueReject {
			want = domain.StateRejected
		}
		if state != want {
			t.Errorf("classifyEffect(EffectNotApplied, %s) = %s, want %s", class, state, want)
		}
	}

	// The mirror: no other effect may be moved by the class, including by the
	// one class that moves NotApplied.
	for _, effect := range allEffects {
		if effect == apperror.EffectNotApplied {
			continue
		}
		var first domain.OrderState
		for i, class := range allClasses {
			state, err := classifyEffect(effect, class)
			if err != nil {
				t.Fatalf("classifyEffect(%s, %s) err = %v", effect, class, err)
			}
			if i == 0 {
				first = state
				continue
			}
			if state != first {
				t.Errorf("classifyEffect(%s, %s) = %s, but %s gave %s — only NotApplied may read the class",
					effect, class, state, allClasses[0], first)
			}
		}
	}
}

// TestClassifyEffectFallsBackSafelyAndSaysSo is the undeclared-member branch,
// reachable only from here.
//
// It must return BOTH a state and an error. State-only would make this branch
// return the identical (SubmissionUnknown, nil) as the EffectUnknown case, so
// dropping a member from that case would change nothing observable — the
// trailing-return trap. Error-only would leave a caller whose Place has already
// run with no state to record for a possibly-live order.
func TestClassifyEffectFallsBackSafelyAndSaysSo(t *testing.T) {
	t.Parallel()

	// Past maxEffect, and past any plausible near-term addition, so this test
	// does not start colliding with a real member the day one is declared.
	undeclared := apperror.Effect(200)

	state, err := classifyEffect(undeclared, apperror.ClassTransport)
	if state != domain.StateSubmissionUnknown {
		t.Errorf("classifyEffect on an undeclared effect = %s, want %s — the state must stay safe",
			state, domain.StateSubmissionUnknown)
	}
	if !errors.Is(err, ErrEffectUnhandled) {
		t.Errorf("classifyEffect on an undeclared effect err = %v, want ErrEffectUnhandled", err)
	}
}

// TestClassifyPlaceReturnsOnlySuccessorsOfDispatchAuthorized is §24e's assertion
// against SM §1, and it is a test rather than a runtime guard on purpose: the
// only fallback such a guard could have is SubmissionUnknown, which is what the
// fallthrough already returns, so it would change no behaviour and cost a branch
// on the dispatch path.
//
// This is also the test that made the fourth owed edge
// (DispatchAuthorized → SubmissionUnknown) a prerequisite rather than a nicety:
// without that edge in domain.CanTransition, the classifier's most common answer
// is one the state machine forbids.
func TestClassifyPlaceReturnsOnlySuccessorsOfDispatchAuthorized(t *testing.T) {
	t.Parallel()

	reached := make(map[domain.OrderState]struct{})

	record := func(state domain.OrderState) {
		reached[state] = struct{}{}
		if err := domain.CanTransition(domain.StateDispatchAuthorized, state); err != nil {
			t.Errorf("classifier returned %s, which SM §1 does not allow from %s: %v",
				state, domain.StateDispatchAuthorized, err)
		}
	}

	// The nil-error path, which classifyPlace owns rather than classifyEffect.
	state, err := classifyPlace(nil)
	if err != nil {
		t.Fatalf("classifyPlace(nil) err = %v", err)
	}
	record(state)

	for _, effect := range allEffects {
		for _, class := range allClasses {
			state, _ := classifyEffect(effect, class)
			record(state)
		}
	}
	// The undeclared branch too — its state is acted on, so it is bound by the
	// same rule as every other.
	state, _ = classifyEffect(apperror.Effect(200), apperror.ClassInternal)
	record(state)

	// Exactly these three, no more and no fewer. "No more" is the safety half;
	// "no fewer" is what fails if a case is collapsed into another — a
	// classifier that answered SubmissionUnknown to everything would satisfy
	// every CanTransition check above.
	want := []domain.OrderState{
		domain.StateSubmitted,
		domain.StateSubmissionUnknown,
		domain.StateRejected,
	}
	if len(reached) != len(want) {
		t.Errorf("classifier reached %d states (%v), want exactly %d", len(reached), reached, len(want))
	}
	for _, w := range want {
		if _, ok := reached[w]; !ok {
			t.Errorf("classifier never reached %s", w)
		}
	}
}

// TestClassifyPlaceReadsTheEffectNotTheCause is §5.3's "never infer 'not sent'",
// stated as a property of this function.
//
// Two errors whose CAUSES are identical and whose recorded effects differ must
// classify differently. An implementation that read the cause — matched on
// context.DeadlineExceeded, say, or on the message — would give one answer to
// both, and it is the answer that creates a duplicate live order.
func TestClassifyPlaceReadsTheEffectNotTheCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp: i/o timeout")

	notApplied, err := classifyPlace(apperror.Transport("place", apperror.EffectNotApplied, cause))
	if err != nil {
		t.Fatalf("classifyPlace err = %v", err)
	}
	applied, err := classifyPlace(apperror.Transport("place", apperror.EffectApplied, cause))
	if err != nil {
		t.Fatalf("classifyPlace err = %v", err)
	}
	if notApplied == applied {
		t.Errorf("both effects over one cause classified as %s; the effect is not being read", applied)
	}
}
