package boot_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/boot"
)

func TestPostureValidate(t *testing.T) {
	t.Parallel()

	// The zero value is LEGAL here, unlike every other enum in this kernel.
	// "Permits nothing" is a real answer and the one a zeroed struct should
	// give, so a Validate that rejected it would force callers to populate a
	// field whose safe value is zero.
	for _, p := range []boot.Posture{
		boot.PostureHalted,
		boot.PostureReconcileOnly,
		boot.PostureStopOpening,
		boot.PostureTrading,
	} {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", p, err)
		}
	}

	if err := boot.Posture(4).Validate(); !errors.Is(err, boot.ErrPostureInvalid) {
		t.Errorf("Posture(4).Validate() = %v, want ErrPostureInvalid", err)
	}
}

func TestPostureString(t *testing.T) {
	t.Parallel()

	cases := map[boot.Posture]string{
		boot.PostureHalted:        "PostureHalted",
		boot.PostureReconcileOnly: "PostureReconcileOnly",
		boot.PostureStopOpening:   "PostureStopOpening",
		boot.PostureTrading:       "PostureTrading",
		boot.Posture(9):           "Posture(9)",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Posture(%d).String() = %q, want %q", uint8(p), got, want)
		}
	}
}

// TestPostureOrdering pins the ladder itself. Permits is a >= against
// minPostureFor, so the ordering IS the semantics: swapping two constants
// changes what the process is allowed to do at a venue without changing a
// single line of logic.
func TestPostureOrdering(t *testing.T) {
	t.Parallel()

	ladder := []boot.Posture{
		boot.PostureHalted,
		boot.PostureReconcileOnly,
		boot.PostureStopOpening,
		boot.PostureTrading,
	}
	for i := 1; i < len(ladder); i++ {
		if !(ladder[i-1] < ladder[i]) {
			t.Errorf("%s must rank below %s", ladder[i-1], ladder[i])
		}
	}
	if boot.PostureHalted != 0 {
		t.Errorf("PostureHalted = %d, want 0: the zero value must be the most restrictive", boot.PostureHalted)
	}
}

// TestPosturePermits walks the full cross product, which is the only way to
// pin a ladder. A table of "the interesting cases" is exactly where an
// off-by-one rung hides: each act is checked at the posture that admits it AND
// at the one immediately below.
func TestPosturePermits(t *testing.T) {
	t.Parallel()

	want := map[boot.Posture]map[boot.Act]bool{
		boot.PostureHalted: {
			boot.ActReconcile: false, boot.ActReduce: false, boot.ActOpen: false,
		},
		boot.PostureReconcileOnly: {
			boot.ActReconcile: true, boot.ActReduce: false, boot.ActOpen: false,
		},
		boot.PostureStopOpening: {
			boot.ActReconcile: true, boot.ActReduce: true, boot.ActOpen: false,
		},
		boot.PostureTrading: {
			boot.ActReconcile: true, boot.ActReduce: true, boot.ActOpen: true,
		},
	}

	for p, acts := range want {
		for a, allowed := range acts {
			if got := p.Permits(a); got != allowed {
				t.Errorf("%s.Permits(%s) = %v, want %v", p, a, got, allowed)
			}
		}
	}
}

// TestPosturePermitsRefusesUnspecifiedAct is the default-deny direction on the
// act axis. A caller passing a zero Act has not said what it wants to do, and
// the most permissive posture must still refuse it — otherwise PostureTrading
// would wave through a struct field somebody forgot to populate.
func TestPosturePermitsRefusesUnspecifiedAct(t *testing.T) {
	t.Parallel()

	for _, a := range []boot.Act{boot.ActUnspecified, boot.Act(200)} {
		if boot.PostureTrading.Permits(a) {
			t.Errorf("PostureTrading.Permits(%s) = true, want false", a)
		}
	}
	// Positive control: the same posture does permit a real act, so the
	// refusals above are about the act and not about a predicate that objects
	// to everything.
	if !boot.PostureTrading.Permits(boot.ActOpen) {
		t.Error("PostureTrading.Permits(ActOpen) = false, want true")
	}
}

func TestActValidateAndString(t *testing.T) {
	t.Parallel()

	if err := boot.ActUnspecified.Validate(); !errors.Is(err, boot.ErrActInvalid) {
		t.Errorf("ActUnspecified.Validate() = %v, want ErrActInvalid", err)
	}
	if err := boot.Act(4).Validate(); !errors.Is(err, boot.ErrActInvalid) {
		t.Errorf("Act(4).Validate() = %v, want ErrActInvalid", err)
	}
	for _, a := range []boot.Act{boot.ActReconcile, boot.ActReduce, boot.ActOpen} {
		if err := a.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", a, err)
		}
	}

	cases := map[boot.Act]string{
		boot.ActUnspecified: "ActUnspecified",
		boot.ActReconcile:   "ActReconcile",
		boot.ActReduce:      "ActReduce",
		boot.ActOpen:        "ActOpen",
		boot.Act(7):         "Act(7)",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("Act(%d).String() = %q, want %q", uint8(a), got, want)
		}
	}
}

// TestWeakestEmpty is the load-bearing case and the one a fixture set full of
// populated inputs never reaches. A fold seeded with the maximum returns
// PostureTrading here, which would make "nothing was checked" and "everything
// passed" the same answer.
func TestWeakestEmpty(t *testing.T) {
	t.Parallel()

	if got := boot.Weakest(); got != boot.PostureHalted {
		t.Errorf("Weakest() = %s, want PostureHalted", got)
	}
}

// TestWeakestSingleton separates the seed from the reduction. Weakest is
// written as `i == 0 || p < weakest` over a PostureHalted seed; dropping the
// i == 0 leaves a function that returns PostureHalted for every input, and
// every multi-element fixture whose minimum happens to be low would still pass.
func TestWeakestSingleton(t *testing.T) {
	t.Parallel()

	for _, p := range []boot.Posture{
		boot.PostureHalted,
		boot.PostureReconcileOnly,
		boot.PostureStopOpening,
		boot.PostureTrading,
	} {
		if got := boot.Weakest(p); got != p {
			t.Errorf("Weakest(%s) = %s, want %s", p, got, p)
		}
	}
}

// TestWeakestPosition varies WHERE the minimum sits. With the weakest element
// always first, a reduction that only ever keeps the seed and one that
// genuinely scans give the same answer on every fixture.
func TestWeakestPosition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []boot.Posture
		want boot.Posture
	}{
		{
			name: "weakest first",
			in:   []boot.Posture{boot.PostureReconcileOnly, boot.PostureStopOpening, boot.PostureTrading},
			want: boot.PostureReconcileOnly,
		},
		{
			name: "weakest last",
			in:   []boot.Posture{boot.PostureTrading, boot.PostureStopOpening, boot.PostureReconcileOnly},
			want: boot.PostureReconcileOnly,
		},
		{
			name: "weakest in the middle",
			in:   []boot.Posture{boot.PostureTrading, boot.PostureReconcileOnly, boot.PostureStopOpening},
			want: boot.PostureReconcileOnly,
		},
		{
			name: "all equal",
			in:   []boot.Posture{boot.PostureStopOpening, boot.PostureStopOpening},
			want: boot.PostureStopOpening,
		},
		{
			// No element is PostureHalted, so a reduction that wrongly kept its
			// seed would answer PostureHalted and be caught. Every other case
			// above has a minimum above the seed too, but this one says so.
			name: "no halted element",
			in:   []boot.Posture{boot.PostureTrading, boot.PostureTrading},
			want: boot.PostureTrading,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := boot.Weakest(tc.in...); got != tc.want {
				t.Errorf("Weakest(%v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

// TestErrorMessagesNamePackage is a small guard against a sentinel whose text
// makes an operator search the wrong package.
func TestErrorMessagesNamePackage(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		boot.ErrPostureInvalid, boot.ErrActInvalid, boot.ErrGateInvalid,
		boot.ErrOutcomeInvalid, boot.ErrChecklistEmpty, boot.ErrGateUndeclared,
		boot.ErrGateLatched, boot.ErrPhaseOrder, boot.ErrShutdownBudgetExceeded,
		boot.ErrSignalBufferTooSmall, boot.ErrStopBudgetUnset,
	} {
		if !strings.HasPrefix(err.Error(), "boot: ") {
			t.Errorf("sentinel %q does not name its package", err)
		}
	}
}
