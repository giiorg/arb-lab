package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// §6.7's verdict rule, including the PRECEDENCE that model (a)'s NC14 is the
// control for.
//
// The row that carries the whole test is "collapsed, deadline blown": both
// inputs are set, and only one of them may win. A table without it would be
// equally satisfied by an implementation that latches the deadline verdict —
// which is NC14, whose signature is a machine stranded with every interval
// collapsed, a determined naked residual and dispatch budget in hand.
func TestClassifyPrefersACollapsedRangeOverABlownDeadline(t *testing.T) {
	t.Parallel()

	// A collapsed range that is NOT at zero: a group certainly short by 1.
	// Collapsing at zero would make "the range is a point" and "the residual is
	// zero" the same fixture, and the determined branch exists precisely for the
	// nonzero case.
	collapsed, err := domain.NoPossibleDelta("BTC").Shift(qty(t, "-1"))
	if err != nil {
		t.Fatalf("Shift: %v", err)
	}
	open := interval(t, "-1", "2")

	cases := []struct {
		name             string
		rng              domain.PossibleDelta
		deadlineExceeded bool
		want             domain.Classification
	}{
		{"collapsed, no deadline", collapsed, false, domain.ClassificationDetermined},
		{"collapsed, deadline blown", collapsed, true, domain.ClassificationDetermined},
		{"open, no deadline", open, false, domain.ClassificationAmbiguous},
		{"open, deadline blown", open, true, domain.ClassificationDeadlineExceededAmbiguous},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.Classify(tc.rng, tc.deadlineExceeded)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Classify(%s, deadline=%t) = %s, want %s", tc.rng, tc.deadlineExceeded, got, tc.want)
			}
		})
	}
}

func TestClassifyRefusesAnUnpopulatedRange(t *testing.T) {
	t.Parallel()

	if _, err := domain.Classify(domain.PossibleDelta{}, false); err == nil {
		t.Fatal("Classify(zero value) = nil error, want a refusal")
	}
}

// PermitsCompensation is TRUE for exactly one verdict, and the false group is
// enumerated rather than left to a fallthrough.
//
// Both non-permitting verdicts are named, including the deadline-exceeded one
// that DOES admit the interval floor: that action has its own entry point, and a
// single boolean covering both would let a caller size an ordinary reduction
// from a group whose exposure is not fixed.
func TestPermitsCompensationIsDeterminedOnly(t *testing.T) {
	t.Parallel()

	cases := map[domain.Classification]bool{
		domain.ClassificationDetermined:                true,
		domain.ClassificationAmbiguous:                 false,
		domain.ClassificationDeadlineExceededAmbiguous: false,
	}
	for class, want := range cases {
		got, err := class.PermitsCompensation()
		if err != nil {
			t.Fatalf("PermitsCompensation(%s): %v", class, err)
		}
		if got != want {
			t.Fatalf("PermitsCompensation(%s) = %t, want %t", class, got, want)
		}
	}
}

// The unset and the misspelt verdict are separate sentinels, and both are
// refused by BOTH entry points.
//
// Checking PermitsCompensation as well as Validate is what makes the leading
// Validate load-bearing rather than decorative: without it, a switch with a
// trailing return would put the unset value into the false group silently, and
// the false group is where the safety lives.
func TestClassificationRefusesUnsetAndUnknown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		class domain.Classification
		want  error
	}{
		{"unset", domain.Classification(""), domain.ErrClassificationUnset},
		{"misspelt", domain.Classification("deadline_exceeded"), domain.ErrClassificationUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.class.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
			if _, err := tc.class.PermitsCompensation(); !errors.Is(err, tc.want) {
				t.Fatalf("PermitsCompensation = %v, want %v", err, tc.want)
			}
		})
	}
}
