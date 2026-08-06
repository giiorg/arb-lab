package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is the OTHER half of §4.1's check: what the operator declared,
// stated independently of anything the venue reports.
//
// The independence is the whole design constraint (record §73b F3). A
// requirement derived from the venue's own current answer is self-consistent
// under every mutation — it cannot fail on the first boot, which is the boot
// that would catch an account configured wrong before this system ever touched
// it.

// Dialect selects which account model's rule table applies.
//
// # Why this is not ids.Venue
//
// `ids.Venue` is an open configuration key — a deployment can name its venue
// anything, and §5.2's schema is what supplies the string. The rule table is
// keyed on the ACCOUNT MODEL instead, because that is what the rules are about:
// Profiles §2's two columns are "Binance USDⓈ-M (classic account)" and "Bybit
// UTA (linear)", and it is entirely possible for one venue to grow a second
// model (Binance Portfolio Margin IS that second model, and this posture exists
// to refuse it). A closed enum-like, following `baseline.Dialect`'s precedent
// one context over.
type Dialect string

const (
	// DialectBinanceClassic is Binance USDⓈ-M's classic segregated account:
	// spot wallet ≠ futures wallet, isolated perp margin, Portfolio Margin off.
	DialectBinanceClassic Dialect = "binance-classic"

	// DialectBybitUTA is Bybit's unified trading account in ISOLATED_MARGIN
	// mode, where segregation is per-position rather than per-wallet.
	DialectBybitUTA Dialect = "bybit-uta"
)

// Validate reports whether d names a dialect this package has rules for.
//
// It is deliberately the SAME question as "is there a rule table" — see
// ErrDialectUnknown. A dialect that validates but has no rules would evaluate
// nothing and report a clean posture.
func (d Dialect) Validate() error {
	switch d {
	case DialectBinanceClassic, DialectBybitUTA:
		return nil
	}
	return fmt.Errorf("posture dialect %q: %w", string(d), ErrDialectUnknown)
}

func (d Dialect) String() string { return string(d) }

// Attestation is an operator's out-of-band word about a setting that has no
// venue read path (§73c-1 — Portfolio Margin, and only that today).
//
// # Three members, and the zero one is the ordinary state
//
// A fresh deployment has attested nothing, and that must produce a NAMED
// refusal rather than a configuration error: the operator has to be told which
// fact to establish, not that the config is malformed. The ON member exists
// because an operator who looked and found Portfolio Margin enabled must be
// able to record what they saw — an attestation records what was OBSERVED, not
// the verdict "posture OK" (#179's inputs-not-conclusions shape).
type Attestation uint8

const (
	// AttestationUnstated is the zero value: nobody has attested. Refuses, by
	// name (ReasonUnattested).
	AttestationUnstated Attestation = iota

	// AttestationOff records that the operator observed the setting OFF.
	AttestationOff

	// AttestationOn records that the operator observed the setting ON — which
	// for Portfolio Margin means §4.1's premise is false on this account.
	AttestationOn
)

const maxAttestation = AttestationOn

// Validate reports whether a is a declared member. The UNSTATED zero is valid:
// see the type's comment.
func (a Attestation) Validate() error {
	if a > maxAttestation {
		return fmt.Errorf("attestation %d: %w", uint8(a), ErrAttestationInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (a Attestation) String() string {
	switch a {
	case AttestationUnstated:
		return "AttestationUnstated"
	case AttestationOff:
		return "AttestationOff"
	case AttestationOn:
		return "AttestationOn"
	}
	return fmt.Sprintf("Attestation(%d)", uint8(a))
}

// SymbolPin is one configured symbol and the leverage the operator pinned for
// it.
//
// # The roster and the pins are ONE list on purpose
//
// Two lists — "the symbols to check" and "the pins" — can disagree, and the
// disagreement that matters is a symbol configured with no pin. Under two lists
// that is a runtime lookup miss somebody has to remember to refuse; under one
// list it is unrepresentable, because a row exists only if it carries both.
// That is the difference between a check and a structure, and §73b F3's whole
// point is that the pin must not be able to go missing.
type SymbolPin struct {
	// Symbol is the venue's own spelling, matched against the report's
	// subjects verbatim. Not normalised: the venue answered in its spelling and
	// a normalisation here would be a second place where two spellings are
	// reconciled.
	Symbol string

	// Leverage is the operator-declared pin, as a whole multiplier. Must be
	// positive.
	Leverage int
}

// Requirement is §4.1's posture requirement for one venue's account.
type Requirement struct {
	// Venue is whose account this is about, checked against the report's.
	Venue ids.Venue

	// Dialect selects the rule table.
	Dialect Dialect

	// Symbols is the CONFIGURED roster and its pins — the coverage answer for
	// every per-symbol rule.
	//
	// Coverage is answered from here rather than from what the venue returned,
	// which is Profiles §2's own parenthetical ("not only open positions") and
	// task #179's precedent for spot baselines. A check driven by the venue's
	// rows examines zero symbols on a flat account — the state at every cold
	// boot, i.e. exactly when the gate is supposed to speak.
	Symbols []SymbolPin

	// PortfolioMargin is the operator's attestation for SettingPortfolioMargin.
	//
	// Consumed only by a dialect whose rule table names that setting. Pinning
	// one where nothing reads it is REFUSED rather than ignored (#179's
	// precedent): an operator who recorded an attestation believes a check is
	// running on it.
	PortfolioMargin Attestation
}

// Validate reports whether the requirement is one the rules can be run from.
func (r Requirement) Validate() error {
	if err := r.Venue.Validate(); err != nil {
		return fmt.Errorf("posture requirement: %w", err)
	}
	if err := r.Dialect.Validate(); err != nil {
		return fmt.Errorf("posture requirement on %s: %w", r.Venue, err)
	}
	if err := r.PortfolioMargin.Validate(); err != nil {
		return fmt.Errorf("posture requirement on %s: %w", r.Venue, err)
	}
	if len(r.Symbols) == 0 {
		return fmt.Errorf("posture requirement on %s: %w", r.Venue, ErrRosterEmpty)
	}

	seen := make(map[string]struct{}, len(r.Symbols))
	for i, p := range r.Symbols {
		if p.Symbol == "" {
			return fmt.Errorf("posture requirement on %s row %d: %w", r.Venue, i, ErrSymbolUnset)
		}
		if _, dup := seen[p.Symbol]; dup {
			return fmt.Errorf("posture requirement on %s names %s twice: %w",
				r.Venue, p.Symbol, ErrRosterDuplicate)
		}
		seen[p.Symbol] = struct{}{}
		if p.Leverage <= 0 {
			return fmt.Errorf("posture requirement on %s symbol %s pins leverage %d: %w",
				r.Venue, p.Symbol, p.Leverage, ErrLeveragePinUnset)
		}
	}

	if r.PortfolioMargin != AttestationUnstated && !dialectConsumesAttestation(r.Dialect) {
		return fmt.Errorf("posture requirement on %s (%s) pins %s: %w",
			r.Venue, r.Dialect, r.PortfolioMargin, ErrAttestationNotConsumed)
	}
	return nil
}
