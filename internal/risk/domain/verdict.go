package domain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is what §4.1's check ANSWERS with.
//
// The answer is every failed check by name, not a boolean and not the first
// failure. Two reasons, both already paid for in this repo: a checker that
// halts on the first violation never demonstrates the deeper claim, so the
// later checks become invisible rather than passing; and an operator looking at
// a refused boot needs the whole list to act once instead of iteratively,
// because each round trip here is a restart.

// FailureReason is why one check failed.
//
// The members are a PRODUCT of two questions an operator asks — was there
// evidence, and did the evidence satisfy the requirement — rather than a
// severity ladder. Every member refuses; they differ in which file to open.
type FailureReason uint8

const (
	// ReasonUnspecified is the zero value and names no reason. Refused by
	// Validate, so a Failure built without stating why cannot be rendered.
	ReasonUnspecified FailureReason = iota

	// ReasonUnobserved is a required check with NO observation at all — the
	// venue's answer never mentioned this (setting, subject).
	//
	// The common case is a configured symbol the venue returned no row for,
	// which on a flat account is the ordinary shape of a position enumeration.
	// It is a refusal rather than a pass because the requirement is a claim
	// about that symbol, and no row is no evidence (record §73b F4).
	ReasonUnobserved

	// ReasonUnstated is an observation the venue carried but did not fill in —
	// an absent field or an empty one.
	//
	// Distinct from ReasonUnobserved because the two are different defects: an
	// unobserved subject is a coverage gap (wrong scope, flat account, a symbol
	// the venue does not know), while an unstated field is a SHAPE problem — a
	// renamed key, a nesting level, the wrong endpoint. Every codec field on
	// this path is a pointer precisely so that this reason can exist instead of
	// a `false` that reads as the passing value.
	ReasonUnstated

	// ReasonUnreadable is a stated value that could not be read as the kind the
	// rule needs — a leverage or a borrow figure that is not a decimal number.
	//
	// It never falls back to a comparison on the raw text: a value this system
	// cannot parse is one it has not understood, and "not equal to the pin" is
	// a conclusion drawn from a reading that failed.
	ReasonUnreadable

	// ReasonMismatch is a stated, readable value that the requirement forbids —
	// CROSSED where ISOLATED is required, leverage 10 against a pin of 20.
	// This is the failure the whole gate exists to produce.
	ReasonMismatch

	// ReasonAmbiguous is two observations on one (setting, scope, subject) key.
	//
	// Refused rather than folded even when they agree: any rule for which one
	// wins is invented here rather than published by the venue, and the
	// disagreeing case is very likely the violation being hunted — Bybit
	// returns one position row per side in hedge mode, which is the position
	// mode this posture requires to be off.
	ReasonAmbiguous

	// ReasonUnattested is a check that requires an operator attestation and has
	// none (§73c-1's default refuse).
	//
	// Its own member rather than ReasonUnstated, because the action differs
	// completely: no endpoint will ever answer this one, and an operator who
	// reads "the venue did not say" would go looking for a venue problem that
	// does not exist.
	ReasonUnattested
)

const maxFailureReason = ReasonUnattested

// Validate reports whether r names a reason, excluding the sentinel.
func (r FailureReason) Validate() error {
	if r == ReasonUnspecified || r > maxFailureReason {
		return fmt.Errorf("failure reason %d: %w", uint8(r), ErrFailureReasonInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (r FailureReason) String() string {
	switch r {
	case ReasonUnspecified:
		return "ReasonUnspecified"
	case ReasonUnobserved:
		return "ReasonUnobserved"
	case ReasonUnstated:
		return "ReasonUnstated"
	case ReasonUnreadable:
		return "ReasonUnreadable"
	case ReasonMismatch:
		return "ReasonMismatch"
	case ReasonAmbiguous:
		return "ReasonAmbiguous"
	case ReasonUnattested:
		return "ReasonUnattested"
	}
	return fmt.Sprintf("FailureReason(%d)", uint8(r))
}

// Failure is one check that did not pass.
type Failure struct {
	// Setting, Scope and Subject identify the check. Subject is empty exactly
	// when Scope is ScopeAccount.
	Setting Setting
	Scope   Scope
	Subject string

	// Reason is why it failed.
	Reason FailureReason

	// Observed is the venue's word, verbatim, when there was one. Empty when
	// the reason is that there was not.
	Observed string

	// Required is what the check wanted, rendered for a human — "false",
	// "ISOLATED", "one of 3, 4, 5, 6", "20", "0".
	Required string

	// Source names the venue call the observation came from, when there was
	// one. Empty for an unobserved check (nothing answered) and for an
	// attestation (nothing ever will).
	Source string
}

// Validate reports whether the failure is one that can be rendered and acted
// on.
func (f Failure) Validate() error {
	if err := f.Setting.Validate(); err != nil {
		return err
	}
	if err := f.Scope.Validate(); err != nil {
		return fmt.Errorf("%s: %w", f.Setting, err)
	}
	if err := f.Reason.Validate(); err != nil {
		return fmt.Errorf("%s: %w", f.Setting, err)
	}
	return nil
}

// String renders the failure as the operator-facing line.
func (f Failure) String() string {
	var b strings.Builder
	b.WriteString(f.Setting.String())
	if f.Subject != "" {
		b.WriteString(" [" + f.Subject + "]")
	}
	b.WriteString(": " + f.Reason.String())
	if f.Observed != "" {
		b.WriteString(fmt.Sprintf(" (venue said %q", f.Observed))
		if f.Required != "" {
			b.WriteString(fmt.Sprintf(", required %s", f.Required))
		}
		b.WriteString(")")
	} else if f.Required != "" {
		b.WriteString(fmt.Sprintf(" (required %s)", f.Required))
	}
	if f.Source != "" {
		b.WriteString(" via " + f.Source)
	}
	return b.String()
}

// Verdict is the whole §4.1 check's answer for one venue.
type Verdict struct {
	// Venue and Dialect are what was checked, and under which rule table.
	Venue   ids.Venue
	Dialect Dialect

	// ObservedAt is the report's instant, carried through so a refusal can say
	// when the account was in this state.
	ObservedAt clock.ServerTime

	// Evaluated is how many checks RAN — every account rule, every
	// (symbol rule × configured symbol), every (asset rule × enumerated coin),
	// and every attestation.
	//
	// # This field is the difference between "passed" and "nothing happened"
	//
	// A Verdict with no Failures is indistinguishable from a Verdict nobody
	// populated, and the second is the more likely one to reach a gate: it is
	// the zero value. Counting the checks makes the vacuous case answerable,
	// and Satisfied refuses rather than agreeing. Coverage claims are
	// measurements — counting executions says nothing until you have counted
	// how many reached the interesting branch.
	Evaluated int

	// Failures is every check that did not pass, sorted deterministically.
	Failures []Failure
}

// Satisfied reports whether the posture may be traded on.
//
// It returns an error rather than a bare bool for the vacuity above: a verdict
// that evaluated nothing is refused (ErrVerdictVacuous), never reported as
// clean. A caller that ignores the error and reads the bool gets `false`, which
// is the safe direction of the safe direction.
func (v Verdict) Satisfied() (bool, error) {
	if v.Evaluated <= 0 {
		return false, fmt.Errorf("posture verdict on %s: %w", v.Venue, ErrVerdictVacuous)
	}
	return len(v.Failures) == 0, nil
}

// String renders the verdict as the operator-facing summary line plus one line
// per failure.
func (v Verdict) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s posture (%s): %d checks, %d failed",
		v.Venue, v.Dialect, v.Evaluated, len(v.Failures))
	for _, f := range v.Failures {
		b.WriteString("\n  " + f.String())
	}
	return b.String()
}

// sortFailures orders failures by scope, then setting, then subject, then
// reason.
//
// Deterministic because the renderer's output is compared in tests and read by
// an operator, and by SCOPE first because the account-wide refusals explain a
// whole-venue failure — burying them among per-symbol rows is how the one that
// matters gets skimmed past. The venue report's own sort takes the same key
// order for the same reason.
func sortFailures(f []Failure) {
	sort.SliceStable(f, func(i, j int) bool {
		a, b := f[i], f[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Setting != b.Setting {
			return a.Setting < b.Setting
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return a.Reason < b.Reason
	})
}
