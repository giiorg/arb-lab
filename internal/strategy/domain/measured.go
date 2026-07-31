package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// MeasuredRate is a class-2 number (§40e): a rate whose VALUE is a
// measurement, carried with the identity of the artifact that measured it.
//
// Class 1 (spec-defaulted strategy params) rides plain fields with config
// defaults — the EWMA half-life's "~3 days" is the spec's own number.
// Class 2 has no usable default anywhere: the exit band's zero value exits on
// the first dip's first settlement, zero adverse drift admits every entry.
// So the wrapper refuses to validate without the artifact identity attached,
// which is what makes "somebody forgot the calibration" a refusal naming the
// field instead of a gate scored on flattery.
type MeasuredRate struct {
	Value    fixedpoint.Rate
	Artifact CalibrationRef
}

// Validate reports whether the measurement carries both its value and its
// provenance.
func (m MeasuredRate) Validate() error {
	if err := m.Value.Validate(); err != nil {
		return fmt.Errorf("value: %w", err)
	}
	if err := m.Artifact.Validate(); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	return nil
}

// MeasuredMoney is MeasuredRate's shape for a dimensional amount — the
// rescue-margin figure inside C_allocated, the measured adverse basis move.
type MeasuredMoney struct {
	Amount   fixedpoint.Money
	Artifact CalibrationRef
}

// Validate reports whether the measurement carries both halves.
func (m MeasuredMoney) Validate() error {
	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("amount: %w", err)
	}
	if err := m.Artifact.Validate(); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	return nil
}
