package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file is §4.1's requirement, stated once, as data.
//
// # The rule tables are the answer to "what does this account model require"
//
// Profiles §2's assert-or-refuse row is a prose cell per venue; below it is a
// list of (setting, scope, expectation) rows. The translation is the normative
// content of this slice, so each row cites the clause it encodes and the ones
// that are NOT a direct quote say so.
//
// A dialect with no table is an ERROR rather than an empty list, and that is
// the single most important line in the file: an empty table evaluates nothing,
// finds no failures, and reports a clean posture for an account nobody looked
// at.

// expectationKind is how a rule compares the venue's word against the
// requirement.
type expectationKind uint8

const (
	// expectWord admits a small set of literal words, compared case-insensitively.
	//
	// The fold is safe in the direction that matters: "ON" and "OFF" differ
	// under any folding, so folding can admit a different SPELLING of the
	// required value but never a different value. The alternative — exact
	// match — refuses a venue that changes case, which is a halt rather than a
	// silent pass, but it is a halt with no safety benefit.
	expectWord expectationKind = iota

	// expectNumberIn admits a whitelist of numbers (Bybit's
	// unifiedMarginStatus). A whitelist rather than a blacklist of known-bad
	// values: `2` is unlisted today and next year's addition is unlisted by
	// construction, and on this setting an unlisted value means an account
	// whose collateral model has not been established (record §73b F5).
	expectNumberIn

	// expectLeveragePin admits exactly the operator's pin for that symbol,
	// compared numerically so "20", "20.0" and "20.00" are one answer.
	expectLeveragePin

	// expectZero admits a numeric zero, so that "0" and "0.00000000" agree and
	// an EMPTY string does not (record §73b F2 — the empty borrow figure is the
	// whole trap this slice was built around).
	expectZero

	// expectAttested reads the operator's attestation and never the report.
	// The one check with no venue read path anywhere (record §73b F1).
	expectAttested
)

// expectation is one rule's comparison.
type expectation struct {
	kind expectationKind

	// words are the admitted literals for expectWord.
	words []string

	// numbers are the admitted values for expectNumberIn.
	numbers []int
}

// rule is one row of a dialect's table.
type rule struct {
	setting Setting
	scope   Scope
	expect  expectation

	// why cites the Profiles §2 clause or the record finding this row encodes.
	// Carried as a field rather than a comment so that a rule added without a
	// citation is visibly missing one.
	why string
}

// binanceClassicRules is Profiles §2's Binance USDⓈ-M assert-or-refuse cell.
//
// The cell reads: `GET /fapi/v1/multiAssetsMargin`==false;
// `GET /fapi/v1/symbolConfig`.marginType==ISOLATED; Portfolio-Margin OFF;
// `positionRisk.isAutoAddMargin`==false; one-way
// `positionSide/dual`.dualSidePosition==false; configured leverage matches pin.
// Six checks, six rows.
var binanceClassicRules = []rule{
	{
		setting: SettingMultiAssets,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectWord, words: []string{"false"}},
		why:     "Profiles §2: multiAssetsMargin==false — Multi-Assets forces Cross and is mutually exclusive with Isolated",
	},
	{
		setting: SettingPositionMode,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectWord, words: []string{"false"}},
		why:     "Profiles §2: one-way — dualSidePosition==false (hedge mode also forbids reduceOnly at param level)",
	},
	{
		setting: SettingPortfolioMargin,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectAttested},
		why:     "Profiles §2: Portfolio-Margin OFF — no read path in either document; operator attestation, default refuse (record §73b F1, §73c-1)",
	},
	{
		setting: SettingMarginMode,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectWord, words: []string{"ISOLATED"}},
		why:     "Profiles §2: symbolConfig.marginType==ISOLATED",
	},
	{
		setting: SettingLeverage,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectLeveragePin},
		why:     "Profiles §2: configured leverage matches pin — the pin is operator-declared, never the venue's own earlier answer (record §73b F3)",
	},
	{
		setting: SettingAutoAddMargin,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectWord, words: []string{"false"}},
		why:     "Profiles §2: positionRisk.isAutoAddMargin==false (read-only, no REST setter)",
	},
}

// bybitUTARules is Profiles §2's Bybit UTA assert-or-refuse cell.
//
// The cell reads: `GET /v5/account/info` marginMode==ISOLATED_MARGIN (the
// load-bearing check), unifiedMarginStatus ∈ {3,4,5,6}, spotHedgingStatus==OFF;
// `GET /v5/spot-margin-uta/status`==off; `GET /v5/account/wallet-balance` every
// coin borrowAmount==0 ∧ spotBorrow==0; auto-add-margin OFF for every
// configured linear symbol (not only open positions); one-way positionIdx==0;
// configured leverage matches pin.
var bybitUTARules = []rule{
	{
		setting: SettingMarginMode,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectWord, words: []string{"ISOLATED_MARGIN"}},
		why:     "Profiles §2: marginMode==ISOLATED_MARGIN — under UTA this IS the segregation property",
	},
	{
		setting: SettingAccountKind,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectNumberIn, numbers: []int{3, 4, 5, 6}},
		why:     "Profiles §2: unifiedMarginStatus ∈ {3,4,5,6}; classic 1 refused, 2 and anything added later unlisted (record §73b F5)",
	},
	{
		setting: SettingSpotHedging,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectWord, words: []string{"OFF"}},
		why:     "Profiles §2: spotHedgingStatus==OFF — spot hedging lets spot offset derivatives, re-entangling the legs",
	},
	{
		setting: SettingSpotMarginMode,
		scope:   ScopeAccount,
		// Profiles §2 states the SETTER as `/v5/spot-margin-trade/switch-mode 0`
		// and the read as "`GET /v5/spot-margin-uta/status`==off"; the word the
		// status endpoint answers with is NOT recorded in either document. "0"
		// is this rule's best reading, taken from the setter's argument. A venue
		// answering "OFF" would REFUSE here, by name, with its word in the
		// failure — the halt direction, and reading the endpoint's published
		// response is a loosening trigger (#171's credential, or a doc read).
		expect: expectation{kind: expectWord, words: []string{"0"}},
		why:    "Profiles §2: spot-margin mode off — the off WORD is unverified, taken from the setter's argument (see comment)",
	},
	{
		setting: SettingBorrowAmount,
		scope:   ScopeAsset,
		expect:  expectation{kind: expectZero},
		why:     "Profiles §2: every coin borrowAmount==0 — an EMPTY figure is no evidence, not a zero (record §73b F2)",
	},
	{
		setting: SettingSpotBorrow,
		scope:   ScopeAsset,
		expect:  expectation{kind: expectZero},
		why:     "Profiles §2: every coin spotBorrow==0 — the second half of one conjunction, kept a separate setting so neither can mask the other",
	},
	{
		setting: SettingPositionMode,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectWord, words: []string{"0"}},
		why:     "Profiles §2: one-way — positionIdx==0, observable only per position row, hence ScopeSymbol where Binance is ScopeAccount",
	},
	{
		setting: SettingLeverage,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectLeveragePin},
		why:     "Profiles §2: configured leverage matches pin (record §73b F3)",
	},
	{
		setting: SettingAutoAddMargin,
		scope:   ScopeSymbol,
		expect:  expectation{kind: expectWord, words: []string{"0"}},
		why:     "Profiles §2: auto-add-margin OFF for EVERY CONFIGURED linear symbol, not only open positions (record §73b F4)",
	},
}

// rulesFor returns the dialect's table.
//
// An unknown dialect is an error and never an empty table — see
// ErrDialectUnknown. The switch names every member (§3.6 rule 10), so a third
// account model added to Dialect fails to build here rather than silently
// checking nothing.
func rulesFor(d Dialect) ([]rule, error) {
	switch d {
	case DialectBinanceClassic:
		return binanceClassicRules, nil
	case DialectBybitUTA:
		return bybitUTARules, nil
	}
	return nil, fmt.Errorf("posture dialect %q: %w", string(d), ErrDialectUnknown)
}

// ConsumesAttestation reports whether the dialect's table has a rule that reads
// the operator attestation.
//
// It is derived from the table rather than restated as a per-dialect constant,
// because a constant and a table are two statements of one fact and the pair
// disagrees the moment either moves.
//
// Two callers, one question, two different moments. Requirement.Validate is the
// last line of defence: a pinned attestation nothing consumes is refused
// (#179's precedent). The composition root asks it FIRST, so the operator gets
// a refusal that names the config field they wrote rather than one that names a
// requirement they never typed — which is why this is exported.
func (d Dialect) ConsumesAttestation() bool {
	rules, err := rulesFor(d)
	if err != nil {
		// An unknown dialect consumes nothing, and Requirement.Validate has
		// already refused it by name before reaching here.
		return false
	}
	for _, r := range rules {
		if r.expect.kind == expectAttested {
			return true
		}
	}
	return false
}

// observationKey identifies one answered check.
//
// Scope is part of the key rather than derived from the subject: the same
// setting legitimately arrives at DIFFERENT scopes on the two venues
// (SettingPositionMode is account-wide on Binance and per-symbol on Bybit), so
// a key without it would let a mis-scoped observation satisfy a rule that asked
// a different question.
type observationKey struct {
	setting Setting
	scope   Scope
	subject string
}

// Check evaluates §4.1's posture requirement against one venue's report.
//
// # What is an error and what is a Failure
//
// An error means the check could not be RUN — a malformed requirement, an
// unusable report, two venues confused for each other. A Failure means the
// check ran and the account does not satisfy it. The distinction is the same
// one §7.5's gate outcomes draw, and it matters because the two get different
// operator responses: fix the wiring versus fix the account.
//
// # Every failed check is reported, never the first
//
// The fold does not stop. An operator restarting a boot to discover the next
// violation pays a round trip per check, and a checker that halts on the first
// violation never demonstrates that the later ones can fire at all.
func Check(req Requirement, rep Report) (Verdict, error) {
	if err := req.Validate(); err != nil {
		return Verdict{}, err
	}
	if err := rep.Validate(); err != nil {
		return Verdict{}, err
	}
	if req.Venue != rep.Venue {
		return Verdict{}, fmt.Errorf("requirement names %s, report names %s: %w",
			req.Venue, rep.Venue, ErrVenueMismatch)
	}
	rules, err := rulesFor(req.Dialect)
	if err != nil {
		return Verdict{}, err
	}

	verdict := Verdict{
		Venue:      req.Venue,
		Dialect:    req.Dialect,
		ObservedAt: rep.ObservedAt,
	}

	index, duplicates := indexObservations(rep.Observations)
	verdict.Failures = append(verdict.Failures, duplicates...)
	// A duplicate is a check that RAN and concluded, so it counts. Otherwise a
	// report that stated every setting twice would fail every rule as
	// unobserved AND evaluate nothing — the vacuity guard reporting a clean
	// count on the most broken input there is.
	verdict.Evaluated += len(duplicates)

	pins := make(map[string]string, len(req.Symbols))
	for _, p := range req.Symbols {
		pins[p.Symbol] = strconv.Itoa(p.Leverage)
	}
	assets := enumeratedAssets(rep.Observations)

	for _, r := range rules {
		switch r.scope {
		case ScopeAccount:
			verdict.Evaluated++
			if f, failed := evaluateRule(r, index, "", req, pins); failed {
				verdict.Failures = append(verdict.Failures, f)
			}

		case ScopeSymbol:
			// The CONFIGURED roster drives this, not the venue's rows: a
			// position-driven loop checks zero symbols on a flat account, which
			// is every cold boot (record §73b F4). Requirement.Validate has
			// already refused an empty roster, so this loop cannot be the
			// vacuous one.
			for _, p := range req.Symbols {
				verdict.Evaluated++
				if f, failed := evaluateRule(r, index, p.Symbol, req, pins); failed {
					verdict.Failures = append(verdict.Failures, f)
				}
			}

		case ScopeAsset:
			// No configured roster of coins exists — the wallet enumerates —
			// so coverage is answered from what the read returned. An empty
			// enumeration is therefore REFUSED rather than passed vacuously:
			// "every coin has zero borrow" over no coins is true and worthless,
			// and the two accounts it describes are an empty wallet (which
			// cannot trade anyway, so the halt costs nothing) and a wrong-shaped
			// read (where the halt is the point).
			if len(assets) == 0 {
				verdict.Evaluated++
				verdict.Failures = append(verdict.Failures, Failure{
					Setting:  r.setting,
					Scope:    r.scope,
					Reason:   ReasonUnobserved,
					Required: r.expect.describe(""),
				})
				continue
			}
			for _, a := range assets {
				verdict.Evaluated++
				if f, failed := evaluateRule(r, index, a, req, pins); failed {
					verdict.Failures = append(verdict.Failures, f)
				}
			}

		case ScopeUnspecified:
			// A rule row with no scope would be checked against nothing. It is
			// a table defect rather than an account defect, so it is an error.
			return Verdict{}, fmt.Errorf("%s rule for %s: %w", req.Dialect, r.setting, ErrScopeInvalid)
		}
	}

	sortFailures(verdict.Failures)
	return verdict, nil
}

// indexObservations keys the report's observations and reports duplicates as
// failures.
//
// The first observation for a key is kept and the duplicate produces a Failure,
// so the report is still checkable — but the failure means the verdict cannot
// pass, which is the safe end of "we have two answers and no rule for which
// wins".
func indexObservations(obs []Observation) (map[observationKey]Observation, []Failure) {
	index := make(map[observationKey]Observation, len(obs))
	var dups []Failure
	for _, o := range obs {
		k := observationKey{setting: o.Setting, scope: o.Scope, subject: o.Subject}
		prior, seen := index[k]
		if !seen {
			index[k] = o
			continue
		}
		dups = append(dups, Failure{
			Setting:  o.Setting,
			Scope:    o.Scope,
			Subject:  o.Subject,
			Reason:   ReasonAmbiguous,
			Observed: prior.Value + " / " + o.Value,
			Source:   o.Source,
		})
	}
	return index, dups
}

// enumeratedAssets is the sorted set of coins the report said anything about.
//
// Sorted because it drives a loop whose failures an operator reads, and a
// map-ordered failure list changes between two runs on identical input — which
// makes a rendered verdict untestable and a diff between two boots meaningless.
func enumeratedAssets(obs []Observation) []string {
	seen := make(map[string]struct{})
	for _, o := range obs {
		if o.Scope == ScopeAsset {
			seen[o.Subject] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// evaluateRule runs one rule against one subject.
//
// The second return is whether the check FAILED, stated separately rather than
// signalled by a zero-valued Failure: a Failure whose Reason is the unspecified
// zero is the shape that reads as "no failure" to anything that inspects it,
// and this function's caller appends what it is handed.
func evaluateRule(r rule, index map[observationKey]Observation, subject string, req Requirement, pins map[string]string) (Failure, bool) {
	base := Failure{
		Setting:  r.setting,
		Scope:    r.scope,
		Subject:  subject,
		Required: r.expect.describe(pins[subject]),
	}

	if r.expect.kind == expectAttested {
		// Reads the operator's word and never the report. A venue observation
		// for this setting is not consulted even if one exists, because no
		// classic-account setting's value establishes anything about the
		// account TYPE (§73c-1's rejected alternative).
		switch req.PortfolioMargin {
		case AttestationOff:
			return Failure{}, false
		case AttestationUnstated:
			base.Reason = ReasonUnattested
			return base, true
		case AttestationOn:
			base.Reason = ReasonMismatch
			base.Observed = "on"
			return base, true
		}
		base.Reason = ReasonUnattested
		return base, true
	}

	o, found := index[observationKey{setting: r.setting, scope: r.scope, subject: subject}]
	if !found {
		base.Reason = ReasonUnobserved
		return base, true
	}
	base.Source = o.Source

	if o.Presence != PresenceStated {
		// Absent and stated-empty both land here. The DIAGNOSIS differs and is
		// preserved in Observed for the operator; the verdict does not, because
		// a required claim with no value has no evidence either way.
		base.Reason = ReasonUnstated
		base.Observed = o.Presence.String()
		return base, true
	}

	base.Observed = o.Value
	ok, reason := r.expect.satisfiedBy(o.Value, pins[subject])
	if ok {
		return Failure{}, false
	}
	base.Reason = reason
	return base, true
}

// satisfiedBy reports whether a STATED value meets the expectation, and why not
// when it does not.
//
// It is never called with an absent or empty value: evaluateRule refuses those
// first, so no branch here has to decide what "" means — which is exactly the
// decision record §73b F2 says two callers of one fold got opposite.
func (e expectation) satisfiedBy(value, pin string) (bool, FailureReason) {
	switch e.kind {
	case expectWord:
		for _, w := range e.words {
			if strings.EqualFold(w, value) {
				return true, ReasonUnspecified
			}
		}
		return false, ReasonMismatch

	case expectNumberIn:
		got, err := canonicalNumber(value)
		if err != nil {
			return false, ReasonUnreadable
		}
		for _, n := range e.numbers {
			want, err := canonicalNumber(strconv.Itoa(n))
			if err != nil {
				return false, ReasonUnreadable
			}
			if got == want {
				return true, ReasonUnspecified
			}
		}
		return false, ReasonMismatch

	case expectLeveragePin:
		got, err := canonicalNumber(value)
		if err != nil {
			return false, ReasonUnreadable
		}
		want, err := canonicalNumber(pin)
		if err != nil {
			// An unparseable pin is a requirement defect, not an account one,
			// and Requirement.Validate has already refused a non-positive one.
			// It is reported as unreadable rather than as a mismatch because a
			// mismatch would name the account as the thing to fix.
			return false, ReasonUnreadable
		}
		if got == want {
			return true, ReasonUnspecified
		}
		return false, ReasonMismatch

	case expectZero:
		got, err := canonicalNumber(value)
		if err != nil {
			return false, ReasonUnreadable
		}
		if got == "0" {
			return true, ReasonUnspecified
		}
		return false, ReasonMismatch

	case expectAttested:
		// Unreachable through evaluateRule, which handles the attestation
		// before any value exists to compare. Named rather than defaulted so
		// that §3.6 rule 10's gate keeps checking this switch, and refusing
		// rather than passing so that a future caller reaching it does not get
		// a silent yes.
		return false, ReasonUnattested
	}
	return false, ReasonUnreadable
}

// describe renders what the expectation required, for the operator's line.
func (e expectation) describe(pin string) string {
	switch e.kind {
	case expectWord:
		return strings.Join(e.words, " or ")
	case expectNumberIn:
		parts := make([]string, 0, len(e.numbers))
		for _, n := range e.numbers {
			parts = append(parts, strconv.Itoa(n))
		}
		return "one of " + strings.Join(parts, ", ")
	case expectLeveragePin:
		if pin == "" {
			return "the configured leverage pin"
		}
		return pin
	case expectZero:
		return "0"
	case expectAttested:
		return "an operator attestation that it is off"
	}
	return "an unstated requirement"
}

// canonicalNumber renders decimal text in one canonical form, so that numeric
// equality is a string comparison afterwards.
//
// It goes through fixedpoint's parser rather than strconv or a hand-rolled
// trim, for the reason §3.6 rule 4 exists: this system has exactly one decimal
// reader, it is oracle-tested against math/big, and a second one here would be
// a second set of answers about what "0.00000000" and "20.0" mean. The
// canonical form drops trailing fractional zeros, so "20", "20.0" and "20.00"
// are one string and "0", "0.0" and "-0" are all "0".
//
// A Rate rather than a Quantity because these values are dimensionless — a
// leverage multiplier, an account-kind code — and the two figures that ARE
// quantities (the borrow amounts) are only ever asked whether they are zero,
// where the asset tag decides nothing. Tagging them with the coin would invite
// the arithmetic that the tag exists to police, and none happens here.
func canonicalNumber(s string) (string, error) {
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		return "", err
	}
	return r.Decimal(), nil
}
