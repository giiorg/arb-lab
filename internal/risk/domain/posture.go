package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is the risk context's posture VOCABULARY — what a venue said about
// the account, in this context's words rather than the exchange's.
//
// It restates `engine/venue`'s posture shape rather than importing it, and the
// restatement is not ceremony: §3.6 rule 1 forbids a domain package from
// importing `engine/`, and the reason the rule exists applies here exactly. The
// venue surface's types are shaped by what an exchange can be asked; these are
// shaped by what §4.1 requires. The two drift apart the first time a venue
// grows a field, and the ACL in risk/infrastructure/driven is where that drift
// has to be handled explicitly instead of arriving as a silently-compiling
// struct change.

// Setting names one safety-relevant account setting (§4.1).
//
// The names are this system's, not either venue's: Binance's `marginType` and
// Bybit's `marginMode` are one concern under two spellings, and the requirement
// is stated once over the concern. A setting that exists on only one venue is
// still named here, because per-venue rule tables select which settings they
// require and a concern absent from one table is absent BY NAME.
type Setting uint8

const (
	// SettingUnspecified is the zero value and names no setting. Refused, so
	// an observation nobody populated cannot satisfy a rule.
	SettingUnspecified Setting = iota

	// SettingMarginMode is the perp margin mode — Binance `marginType`,
	// Bybit `marginMode`. Isolated on both, and on Bybit it is Profiles §2's
	// "load-bearing check": under UTA it is the whole segregation property.
	SettingMarginMode

	// SettingMultiAssets is Binance's multi-assets collateral mode, which
	// forces Cross and is mutually exclusive with Isolated.
	SettingMultiAssets

	// SettingAccountKind is Bybit's `unifiedMarginStatus` — which account type
	// the credential is addressing.
	SettingAccountKind

	// SettingSpotHedging is Bybit's `spotHedgingStatus`: spot offsetting
	// derivatives re-entangles the two legs this posture exists to separate.
	SettingSpotHedging

	// SettingSpotMarginMode is Bybit's spot margin trading mode, which must be
	// off — the spot leg is a plain buy, never a borrow.
	SettingSpotMarginMode

	// SettingPositionMode is one-way vs hedge: Binance `dualSidePosition`
	// (account-wide), Bybit `positionIdx` (per symbol). The concern is one; the
	// SCOPE differs by venue, which is why the rule tables carry a scope.
	SettingPositionMode

	// SettingAutoAddMargin is whether a position may silently pull funds from
	// the wallet, which would defeat the isolation the posture rests on.
	SettingAutoAddMargin

	// SettingLeverage is the configured per-symbol leverage, compared against
	// the operator's pin — never against the venue's own earlier answer.
	SettingLeverage

	// SettingBorrowAmount and SettingSpotBorrow are Bybit's two per-coin borrow
	// figures. Profiles §2 requires "every coin borrowAmount==0 ∧
	// spotBorrow==0" — two fields, one conjunction, kept as two settings so the
	// two observations do not share a key and silently keep one of them.
	SettingBorrowAmount
	SettingSpotBorrow

	// SettingPortfolioMargin is Binance Portfolio Margin being OFF.
	//
	// # The one setting with no venue read path anywhere
	//
	// Profiles §2's Binance assert cell lists five checks; four name a GET and
	// this one names none, in either document (record §73b F1). It is also the
	// single setting that would falsify §4.1's entire premise — Portfolio
	// Margin folds the spot wallet into futures collateral — so it cannot be
	// the check that silently passes. It is satisfied by an OPERATOR
	// ATTESTATION and by nothing else (§73c-1): no sibling setting's value
	// establishes it, because Portfolio Margin is a different account TYPE
	// rather than a setting inside the classic account.
	SettingPortfolioMargin
)

const maxSetting = SettingPortfolioMargin

// Validate reports whether s names a setting, excluding the sentinel.
func (s Setting) Validate() error {
	if s == SettingUnspecified || s > maxSetting {
		return fmt.Errorf("posture setting %d: %w", uint8(s), ErrSettingInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s Setting) String() string {
	switch s {
	case SettingUnspecified:
		return "SettingUnspecified"
	case SettingMarginMode:
		return "SettingMarginMode"
	case SettingMultiAssets:
		return "SettingMultiAssets"
	case SettingAccountKind:
		return "SettingAccountKind"
	case SettingSpotHedging:
		return "SettingSpotHedging"
	case SettingSpotMarginMode:
		return "SettingSpotMarginMode"
	case SettingPositionMode:
		return "SettingPositionMode"
	case SettingAutoAddMargin:
		return "SettingAutoAddMargin"
	case SettingLeverage:
		return "SettingLeverage"
	case SettingBorrowAmount:
		return "SettingBorrowAmount"
	case SettingSpotBorrow:
		return "SettingSpotBorrow"
	case SettingPortfolioMargin:
		return "SettingPortfolioMargin"
	}
	return fmt.Sprintf("Setting(%d)", uint8(s))
}

// Scope is the GRANULARITY at which an observation, and the rule reading it, is
// answered — how many times the check runs and what names each run.
//
// Three scopes rather than a flat list because the COVERAGE question differs
// per scope and the check asks it separately: an account-wide setting is
// present once or not at all; a per-symbol setting must be answered for every
// CONFIGURED symbol, not merely for the ones the venue happened to return
// (record §73b F4); and a per-asset setting is answered for whatever the wallet
// enumerated, since no roster of coins exists to compare against.
//
// # Scope is not the SUBJECT, and this comment used to say it was
//
// It read "what an observation, and the rule reading it, is about" until
// record §73p. That is a different property, it is Subject below, and the two
// come apart on this very table: SettingPositionMode is ScopeAccount on Binance
// and ScopeSymbol on Bybit — ONE subject at two granularities, which is exactly
// what observationKey's own comment says. Reading this field as the subject is
// what made §4.1's family requirement demand an endpoint for a leg the
// deployment does not trade (task #199).
type Scope uint8

const (
	// ScopeUnspecified is the zero value and names no scope.
	ScopeUnspecified Scope = iota

	// ScopeAccount is an account-wide setting; Subject is empty.
	ScopeAccount

	// ScopeSymbol is per contract symbol; Subject is the venue symbol.
	ScopeSymbol

	// ScopeAsset is per coin; Subject is the coin in the venue's spelling.
	ScopeAsset
)

const maxScope = ScopeAsset

// Validate reports whether s names a scope, excluding the sentinel.
func (s Scope) Validate() error {
	if s == ScopeUnspecified || s > maxScope {
		return fmt.Errorf("posture scope %d: %w", uint8(s), ErrScopeInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s Scope) String() string {
	switch s {
	case ScopeUnspecified:
		return "ScopeUnspecified"
	case ScopeAccount:
		return "ScopeAccount"
	case ScopeSymbol:
		return "ScopeSymbol"
	case ScopeAsset:
		return "ScopeAsset"
	}
	return fmt.Sprintf("Scope(%d)", uint8(s))
}

// Subject is WHICH PART OF THE DEPLOYMENT a rule's setting governs — the
// property Scope was doing duty for until task #199 (record §73p).
//
// # Why the axis has to exist
//
// §4.1's posture read is routed by product family, so a family this deployment
// declares no endpoint for makes its probes unissuable (record §73n F20). The
// first cut derived the required families from the UNION of a dialect's probes,
// which demanded a futures endpoint from a deployment holding only the spot leg
// of a cross-venue carry — a shape §5.2's singular venue block actively builds
// toward. The union is the wrong quantity: a rule that governs a leg this
// deployment does not trade has nothing to say about it, and an endpoint routed
// only to answer such a rule buys nothing while making a spot-only boot depend
// on a futures API it never calls.
//
// # It is a classification of the RULE, not of the venue
//
// Stated here rather than as "Binance is exempt", because the two venues get
// OPPOSITE answers from the same axis and neither is a special case. Every one
// of Binance's five probed rules is SubjectDerivativesLeg, and the single rule
// whose subject reaches the spot wallet — SettingPortfolioMargin, the account
// TYPE — is expectAttested and needs no endpoint at all. Bybit does not mirror
// it: GET /v5/account/info carries three SubjectAccount rules, so a spot-only
// Bybit deployment still requires the linear-perp family.
//
// A lapsed rule is never a silently skipped one. Check records it in
// Verdict.Lapsed, and Satisfied's vacuity guard is what stops an exemption from
// emptying a table: a no-derivatives Binance verdict evaluates exactly one rule.
type Subject uint8

const (
	// SubjectUnspecified is the zero value: a rule row nobody classified.
	// Refused by rulesFor's audit rather than defaulted, because either default
	// is wrong — defaulting to "always live" reinstates the union this axis
	// exists to replace, and defaulting to "lapses" drops a real check.
	SubjectUnspecified Subject = iota

	// SubjectAccount governs the account itself, whatever it holds: its type,
	// its collateral model, whether its legs are segregated. Never lapses,
	// because the account exists on every deployment.
	SubjectAccount

	// SubjectDerivativesLeg governs the perpetual leg — its position mode, its
	// margin isolation, its leverage. Lapses when this deployment configures no
	// derivative instrument, which is the only such statement any carrier makes
	// (RosterCoverage).
	SubjectDerivativesLeg

	// SubjectSpotLeg governs the spot leg — its borrow, its margin mode.
	//
	// It NEVER lapses today, and the reason is recorded rather than left to be
	// re-derived as an oversight: no carrier states "this deployment has no spot
	// leg" (RosterCoverage answers only the derivatives question), and under
	// Bybit's UTA a spot borrow is account-wide leverage regardless of which leg
	// took it on. A future coverage carrier must argue the second point before
	// reusing the first.
	SubjectSpotLeg
)

const maxSubject = SubjectSpotLeg

// Validate reports whether s names a subject, excluding the sentinel.
func (s Subject) Validate() error {
	if s == SubjectUnspecified || s > maxSubject {
		return fmt.Errorf("posture subject %d: %w", uint8(s), ErrSubjectInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s Subject) String() string {
	switch s {
	case SubjectUnspecified:
		return "SubjectUnspecified"
	case SubjectAccount:
		return "SubjectAccount"
	case SubjectDerivativesLeg:
		return "SubjectDerivativesLeg"
	case SubjectSpotLeg:
		return "SubjectSpotLeg"
	}
	return fmt.Sprintf("Subject(%d)", uint8(s))
}

// Live reports whether a rule with this subject is answerable, and required, on
// a deployment whose roster has the given coverage.
//
// The coverage is validated by the caller (Requirement.Validate), so an
// unspecified one reaching here would be a defect upstream; it answers false,
// which lapses the rule — and a lapse is visible in the verdict, where a
// silently live rule against an unstated coverage would not be.
func (s Subject) Live(c RosterCoverage) bool {
	switch s {
	case SubjectAccount, SubjectSpotLeg:
		return true
	case SubjectDerivativesLeg:
		return c == RosterCoverageDerivatives
	case SubjectUnspecified:
		return false
	}
	return false
}

// Presence is how much the venue actually said about a setting.
//
// All three non-stated members refuse, so this is not three verdicts — it is
// three DIAGNOSES of one refusal. An operator reading a refused boot needs to
// know whether the venue omitted the field (a shape change, or the wrong
// endpoint), answered it blank (a value the venue does not hold), or whether
// nobody stamped the observation at all (an ACL defect on this side of the
// wire). Those are three different files to open.
type Presence uint8

const (
	// PresenceUnspecified is the zero value: an observation nobody stamped.
	// Refused, so a struct built without thinking about presence cannot read
	// as a stated value.
	PresenceUnspecified Presence = iota

	// PresenceStated is a field the venue answered with a non-empty value.
	// The ONLY presence a rule may conclude from.
	PresenceStated

	// PresenceStatedEmpty is a field present on the wire and empty.
	PresenceStatedEmpty

	// PresenceAbsent is a field the venue's answer did not carry at all.
	PresenceAbsent
)

const maxPresence = PresenceAbsent

// Validate reports whether p names a presence, excluding the sentinel.
func (p Presence) Validate() error {
	if p == PresenceUnspecified || p > maxPresence {
		return fmt.Errorf("posture presence %d: %w", uint8(p), ErrPresenceInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (p Presence) String() string {
	switch p {
	case PresenceUnspecified:
		return "PresenceUnspecified"
	case PresenceStated:
		return "PresenceStated"
	case PresenceStatedEmpty:
		return "PresenceStatedEmpty"
	case PresenceAbsent:
		return "PresenceAbsent"
	}
	return fmt.Sprintf("Presence(%d)", uint8(p))
}

// Observation is one setting as the venue answered it.
type Observation struct {
	// Setting is which concern this is about.
	Setting Setting

	// Scope is what it is about; Subject carries the symbol or coin.
	Scope Scope

	// Subject is the venue symbol or coin in the venue's own spelling, and is
	// empty exactly when Scope is ScopeAccount.
	Subject string

	// Value is the venue's word, verbatim and untranslated. Meaningful only
	// when Presence is PresenceStated.
	Value string

	// Presence is how much the venue said.
	Presence Presence

	// Source names the venue call this came from, so a refusal can name the
	// endpoint that answered rather than only the setting that failed — the one
	// line an operator acts on.
	//
	// A free string rather than this context's enum, deliberately: the probe
	// set is the VENUE's (it changes when an exchange reshuffles its
	// endpoints), no rule here branches on it, and an enum would make every new
	// endpoint an edit to a domain type that does not care. It is carried
	// through to Failure and rendered, never compared.
	Source string
}

// Validate reports whether the observation is one a rule may reason from.
//
// It does NOT require PresenceStated: "the venue did not say" is a legitimate,
// fully formed observation and the rules refuse it BY NAME. What is refused
// here is an observation that contradicts itself, which is the shape a
// hand-built fixture reaches for.
func (o Observation) Validate() error {
	if err := o.Setting.Validate(); err != nil {
		return err
	}
	if err := o.Scope.Validate(); err != nil {
		return fmt.Errorf("%s: %w", o.Setting, err)
	}
	if err := o.Presence.Validate(); err != nil {
		return fmt.Errorf("%s: %w", o.Setting, err)
	}
	switch o.Scope {
	case ScopeAccount:
		if o.Subject != "" {
			return fmt.Errorf("%s is account-wide but names subject %q: %w",
				o.Setting, o.Subject, ErrObservationIncoherent)
		}
	case ScopeSymbol, ScopeAsset:
		if o.Subject == "" {
			return fmt.Errorf("%s is %s but names no subject: %w",
				o.Setting, o.Scope, ErrObservationIncoherent)
		}
	case ScopeUnspecified:
		return fmt.Errorf("%s: %w", o.Setting, ErrScopeInvalid)
	}
	if o.Presence != PresenceStated && o.Value != "" {
		return fmt.Errorf("%s is %s but carries value %q: %w",
			o.Setting, o.Presence, o.Value, ErrObservationIncoherent)
	}
	if o.Presence == PresenceStated && o.Value == "" {
		return fmt.Errorf("%s is stated but carries no value: %w",
			o.Setting, ErrObservationIncoherent)
	}
	return nil
}

// Report is one venue's whole posture read, as the guard receives it.
//
// It carries observations and nothing else — no verdict, no "ok" flag. The
// report is the INPUT to §4.1's rules, and a report that already contained the
// answer would make the rule checking it tautological (a stored form that
// contains the answer, one surface over).
type Report struct {
	// Venue is whose account was read. Checked against the requirement's, so
	// one exchange's answer cannot be judged against another's rule table.
	Venue ids.Venue

	// ObservedAt is the venue clock instant of the read.
	//
	// Required, and NOT because the verdict expires: §73c-2 ruled that every
	// trade performs a fresh read, so there is no TTL here to compare against.
	// It is the operator's diagnosis anchor — the answer to "when was this the
	// account's state" on a refusal — and an unset instant means an ACL
	// invented a report rather than translating one.
	ObservedAt clock.ServerTime

	// Observations are every setting the read learned about. Order is not
	// significant: the check indexes them and reports its own sorted failures.
	Observations []Observation
}

// Validate reports whether the report is one the rules can be run against.
func (r Report) Validate() error {
	if err := r.Venue.Validate(); err != nil {
		return fmt.Errorf("posture report: %w: %w", ErrReportUnusable, err)
	}
	if err := r.ObservedAt.Validate(); err != nil {
		return fmt.Errorf("posture report on %s: %w: %w", r.Venue, ErrReportUnusable, err)
	}
	for i, o := range r.Observations {
		if err := o.Validate(); err != nil {
			return fmt.Errorf("posture report on %s observation %d: %w", r.Venue, i, err)
		}
	}
	// A report with NO observations is deliberately not refused here. It is a
	// coherent thing for an ACL to hand over — a venue that answered nothing —
	// and refusing it here would move the diagnosis from a verdict listing
	// eleven unobserved settings by name to a single unusable-report error.
	// Check turns it into the former, which is what an operator can act on.
	return nil
}
