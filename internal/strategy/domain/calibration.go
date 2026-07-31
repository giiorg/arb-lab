package domain

import (
	"fmt"
	"strings"
)

// CalibrationRef is the identity of the step-4 calibration artifact a
// measured number came from: ID plus content hash (§40e class 2).
//
// Every class-2 measured quantity — the dispersion distribution, the slippage
// model's numbers, the rescue-margin figure, the exit band's width, the
// adverse basis move — refuses to construct without one, because each one's
// zero value flatters the gate and a missing artifact must be a refusal that
// names the field, never a forecast running on a plausible fallback.
//
// The hash is hex text rather than a byte array because the identity's only
// operations are equality and journaling; nothing recomputes it here.
type CalibrationRef struct {
	// ID names the artifact (§8's pre-registered, independently-approved
	// record).
	ID string

	// Hash is the artifact's content hash, lowercase hex.
	Hash string
}

// FixturePrefix marks a calibration identity minted by a test helper. §40f
// admits "the explicit fixture-helper marker in tests" in place of a real
// artifact identity; the marker is a distinguished ID prefix so a journaled
// Decision scored against fixture numbers can never read as scored against
// the artifact. A scored run's one-artifact rule (§40e) is step 5's check;
// the prefix is what makes it checkable.
const FixturePrefix = "fixture:"

// Validate reports whether the reference identifies anything.
func (c CalibrationRef) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("calibration id: %w", ErrUnset)
	}
	if c.Hash == "" {
		return fmt.Errorf("calibration hash: %w", ErrUnset)
	}
	return nil
}

// IsFixture reports whether this identity was minted by a test helper rather
// than by the step-4 artifact governance.
func (c CalibrationRef) IsFixture() bool { return strings.HasPrefix(c.ID, FixturePrefix) }
