package boot_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/boot"
)

// noopStart is a Start that acquires nothing and returns no closer.
func noopStart(context.Context) (boot.StopFunc, error) { return nil, nil }

// phaseT builds a phase with a live Start and a one-second stop budget, which
// is the shape most ordering fixtures need and none of them are about.
func phaseT(name string, kind boot.PhaseKind) boot.Phase {
	return boot.Phase{Name: name, Kind: kind, Start: noopStart, StopBudget: time.Second}
}

// validSequence is the reference wiring: every ordering rule satisfied, budget
// comfortably inside the silence threshold. Fixtures perturb ONE thing about it
// so a refusal is attributable — §10h's lesson, where four fixtures each failed
// the check after the one under test and four mutants lived on it.
func validSequence() boot.Sequence {
	return boot.Sequence{
		Phases: []boot.Phase{
			phaseT("signal", boot.KindSignal),
			phaseT("config", boot.KindConfig),
			phaseT("obs", boot.KindObservability),
			phaseT("venue", boot.KindService),
			phaseT("reconcile", boot.KindReconcile),
		},
		WatchdogSilence: time.Minute,
	}
}

func TestPhaseKindValidateAndString(t *testing.T) {
	t.Parallel()

	if err := boot.KindUnspecified.Validate(); !errors.Is(err, boot.ErrPhaseKindInvalid) {
		t.Errorf("KindUnspecified.Validate() = %v, want ErrPhaseKindInvalid", err)
	}
	if err := boot.PhaseKind(6).Validate(); !errors.Is(err, boot.ErrPhaseKindInvalid) {
		t.Errorf("PhaseKind(6).Validate() = %v, want ErrPhaseKindInvalid", err)
	}
	for _, k := range []boot.PhaseKind{
		boot.KindSignal, boot.KindConfig, boot.KindObservability,
		boot.KindService, boot.KindReconcile,
	} {
		if err := k.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", k, err)
		}
		if s := k.String(); strings.HasPrefix(s, "PhaseKind(") {
			t.Errorf("PhaseKind(%d).String() = %q, want a name", uint8(k), s)
		}
	}
	if got := boot.PhaseKind(42).String(); got != "PhaseKind(42)" {
		t.Errorf("PhaseKind(42).String() = %q, want %q", got, "PhaseKind(42)")
	}
}

// TestValidSequenceValidates is the positive control the whole file rests on.
// Without it every refusal below is consistent with a Validate that refuses
// everything, which reads as a perfect score.
func TestValidSequenceValidates(t *testing.T) {
	t.Parallel()

	if err := validSequence().Validate(); err != nil {
		t.Fatalf("the reference sequence must validate: %v", err)
	}

	// And without the optional reconcile phase, since §11's binaries do not all
	// have venue truth to join to.
	s := validSequence()
	s.Phases = s.Phases[:4]
	if err := s.Validate(); err != nil {
		t.Errorf("a sequence with no reconcile phase must validate: %v", err)
	}
}

func TestSequenceValidatePhaseFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*boot.Sequence)
		want    error
		message string
	}{
		{
			name:    "unnamed phase",
			mutate:  func(s *boot.Sequence) { s.Phases[3].Name = "" },
			want:    boot.ErrPhaseNameUnset,
			message: "phase 3",
		},
		{
			name:    "unset kind",
			mutate:  func(s *boot.Sequence) { s.Phases[3].Kind = boot.KindUnspecified },
			want:    boot.ErrPhaseKindInvalid,
			message: `"venue"`,
		},
		{
			name:    "no Start",
			mutate:  func(s *boot.Sequence) { s.Phases[3].Start = nil },
			want:    boot.ErrPhaseStartUnset,
			message: `"venue"`,
		},
		{
			name:    "negative stop budget",
			mutate:  func(s *boot.Sequence) { s.Phases[3].StopBudget = -time.Second },
			want:    boot.ErrStopBudgetNegative,
			message: `"venue"`,
		},
		{
			name:    "duplicate name",
			mutate:  func(s *boot.Sequence) { s.Phases[3].Name = "config" },
			want:    boot.ErrPhaseNameDuplicate,
			message: `"config"`,
		},
		{
			name:    "no phases at all",
			mutate:  func(s *boot.Sequence) { s.Phases = nil },
			want:    boot.ErrPhaseKindMissing,
			message: "KindSignal",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := validSequence()
			tc.mutate(&s)
			err := s.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("error %q does not name %q", err, tc.message)
			}
		})
	}
}

func TestSequenceValidateRequiredKinds(t *testing.T) {
	t.Parallel()

	for _, k := range []boot.PhaseKind{boot.KindSignal, boot.KindConfig, boot.KindObservability} {
		t.Run("missing "+k.String(), func(t *testing.T) {
			t.Parallel()
			s := validSequence()
			var kept []boot.Phase
			for _, p := range s.Phases {
				if p.Kind != k {
					kept = append(kept, p)
				}
			}
			s.Phases = kept
			err := s.Validate()
			if !errors.Is(err, boot.ErrPhaseKindMissing) {
				t.Fatalf("Validate() = %v, want ErrPhaseKindMissing", err)
			}
			// The message must say which kind is missing. Three rules share
			// this sentinel, so without the name a fixture failing on the wrong
			// one looks identical.
			if !strings.Contains(err.Error(), k.String()) {
				t.Errorf("error %q does not name %s", err, k)
			}
		})
	}
}

func TestSequenceValidateRepeatedKinds(t *testing.T) {
	t.Parallel()

	// KindService is the open slot and must be repeatable — that is the whole
	// point of it, so it is checked here rather than assumed.
	s := validSequence()
	s.Phases = append(s.Phases[:4:4],
		phaseT("ws", boot.KindService),
		phaseT("operator-api", boot.KindService),
		phaseT("reconcile", boot.KindReconcile),
	)
	if err := s.Validate(); err != nil {
		t.Fatalf("repeated KindService must validate: %v", err)
	}

	for _, k := range []boot.PhaseKind{
		boot.KindSignal, boot.KindConfig, boot.KindObservability, boot.KindReconcile,
	} {
		t.Run("repeated "+k.String(), func(t *testing.T) {
			t.Parallel()
			s := validSequence()
			// Insert the duplicate immediately after the original, so the only
			// broken rule is the repeat and not also the ordering.
			var out []boot.Phase
			for _, p := range s.Phases {
				out = append(out, p)
				if p.Kind == k {
					dup := p
					dup.Name = p.Name + "-again"
					out = append(out, dup)
				}
			}
			s.Phases = out
			err := s.Validate()
			if !errors.Is(err, boot.ErrPhaseKindRepeated) {
				t.Fatalf("Validate() = %v, want ErrPhaseKindRepeated", err)
			}
			if !strings.Contains(err.Error(), k.String()) {
				t.Errorf("error %q does not name %s", err, k)
			}
		})
	}
}

// TestSequenceValidateOrdering is where asserting the MESSAGE rather than the
// sentinel earns its keep: four distinct constraints share ErrPhaseOrder, so a
// fixture that broke a different one would be indistinguishable from a pass.
func TestSequenceValidateOrdering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		phases  []boot.Phase
		message string
	}{
		{
			name: "signal not first",
			phases: []boot.Phase{
				phaseT("config", boot.KindConfig),
				phaseT("signal", boot.KindSignal),
				phaseT("obs", boot.KindObservability),
			},
			message: "KindSignal at position 1 must be first",
		},
		{
			name: "observability before config",
			phases: []boot.Phase{
				phaseT("signal", boot.KindSignal),
				phaseT("obs", boot.KindObservability),
				phaseT("config", boot.KindConfig),
			},
			message: "KindConfig must precede KindObservability",
		},
		{
			name: "service before observability",
			phases: []boot.Phase{
				phaseT("signal", boot.KindSignal),
				phaseT("config", boot.KindConfig),
				phaseT("venue", boot.KindService),
				phaseT("obs", boot.KindObservability),
			},
			message: `KindService "venue" at position 2 precedes KindObservability`,
		},
		{
			name: "reconcile before observability",
			phases: []boot.Phase{
				phaseT("signal", boot.KindSignal),
				phaseT("config", boot.KindConfig),
				phaseT("reconcile", boot.KindReconcile),
				phaseT("obs", boot.KindObservability),
			},
			message: `KindReconcile "reconcile" at position 2 precedes KindObservability`,
		},
		{
			name: "reconcile not last",
			phases: []boot.Phase{
				phaseT("signal", boot.KindSignal),
				phaseT("config", boot.KindConfig),
				phaseT("obs", boot.KindObservability),
				phaseT("reconcile", boot.KindReconcile),
				phaseT("venue", boot.KindService),
			},
			message: "KindReconcile at position 3 must be last",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := boot.Sequence{Phases: tc.phases, WatchdogSilence: time.Minute}
			err := s.Validate()
			if !errors.Is(err, boot.ErrPhaseOrder) {
				t.Fatalf("Validate() = %v, want ErrPhaseOrder", err)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("error %q does not contain %q", err, tc.message)
			}
		})
	}
}

// TestSequenceValidateOrderingNamesTheOffender pins the other half of the
// message: a refusal that says "signal must be first" without saying what came
// first instead sends an operator back to the wiring to work it out.
func TestSequenceValidateOrderingNamesTheOffender(t *testing.T) {
	t.Parallel()

	s := boot.Sequence{
		Phases: []boot.Phase{
			phaseT("load-config", boot.KindConfig),
			phaseT("install-signals", boot.KindSignal),
			phaseT("metrics", boot.KindObservability),
		},
		WatchdogSilence: time.Minute,
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), `"load-config"`) {
		t.Errorf("error %v does not name the phase that displaced the signal phase", err)
	}
}

func TestShutdownBudget(t *testing.T) {
	t.Parallel()

	s := boot.Sequence{Phases: []boot.Phase{
		{Name: "a", Kind: boot.KindSignal, Start: noopStart, StopBudget: 3 * time.Second},
		{Name: "b", Kind: boot.KindConfig, Start: noopStart, StopBudget: 0},
		{Name: "c", Kind: boot.KindObservability, Start: noopStart, StopBudget: 4 * time.Second},
	}}
	if got, want := s.ShutdownBudget(), 7*time.Second; got != want {
		t.Errorf("ShutdownBudget() = %s, want %s", got, want)
	}
	if got := (boot.Sequence{}).ShutdownBudget(); got != 0 {
		t.Errorf("empty ShutdownBudget() = %s, want 0", got)
	}
}

// TestWatchdogSilenceBoundary is the freeAt lesson applied in advance. Every
// other fixture in this file sits comfortably inside the threshold, where >=
// and > are the same predicate; equality is the one input that separates them,
// and equality is already too late — at exactly the silence threshold the
// watchdog fires.
func TestWatchdogSilenceBoundary(t *testing.T) {
	t.Parallel()

	// Five phases at one second each.
	const budget = 5 * time.Second

	cases := []struct {
		name    string
		silence time.Duration
		wantErr bool
	}{
		{name: "budget one nanosecond under the threshold", silence: budget + 1, wantErr: false},
		{name: "budget exactly at the threshold", silence: budget, wantErr: true},
		{name: "budget one nanosecond over the threshold", silence: budget - 1, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := validSequence()
			s.WatchdogSilence = tc.silence
			if got := s.ShutdownBudget(); got != budget {
				t.Fatalf("fixture drifted: ShutdownBudget() = %s, want %s", got, budget)
			}
			err := s.Validate()
			if tc.wantErr {
				if !errors.Is(err, boot.ErrShutdownBudgetExceeded) {
					t.Fatalf("Validate() = %v, want ErrShutdownBudgetExceeded", err)
				}
				// Both numbers must appear, because the fix is to change one of
				// them and the operator has to know which is which.
				if !strings.Contains(err.Error(), budget.String()) ||
					!strings.Contains(err.Error(), tc.silence.String()) {
					t.Errorf("error %q does not report both durations", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestWatchdogSilenceRequired(t *testing.T) {
	t.Parallel()

	for _, silence := range []time.Duration{0, -time.Second} {
		s := validSequence()
		s.WatchdogSilence = silence
		if err := s.Validate(); !errors.Is(err, boot.ErrWatchdogSilenceUnset) {
			t.Errorf("Validate() with WatchdogSilence=%s = %v, want ErrWatchdogSilenceUnset", silence, err)
		}
	}
}
