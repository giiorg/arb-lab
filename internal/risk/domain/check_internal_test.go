package domain

import (
	"errors"
	"testing"
)

// The rule tables are this slice's normative content — Profiles §2's two prose
// cells, translated. So they are asserted against a roster written out here BY
// HAND rather than derived from the tables themselves.
//
// Deriving it would make the test agree with whatever the table says, which is
// the shared-constant vacuity: one value feeding both the subject and its
// checker stays self-consistent under every mutation. A rule deleted, a scope
// flipped, or a whitelist quietly widened has to fail HERE, by name.
func TestRuleTablesMatchProfilesSection2(t *testing.T) {
	// The SUBJECT column is written out here for the same reason as the rest,
	// and it carries more weight than the others: it is the only column that
	// decides whether a rule RUNS. A subject silently flipped to
	// SubjectDerivativesLeg drops a check on every spot-only deployment, and no
	// fixture that violates the rule on a derivatives deployment would notice
	// (task #199, record §73p). Scope and subject are stated as separate columns
	// so a test reading one for the other fails here.
	type row struct {
		setting Setting
		scope   Scope
		subject Subject
		kind    expectationKind
	}

	for _, tc := range []struct {
		dialect Dialect
		want    []row
	}{
		{
			dialect: DialectBinanceClassic,
			want: []row{
				{SettingMultiAssets, ScopeAccount, SubjectDerivativesLeg, expectWord},
				{SettingPositionMode, ScopeAccount, SubjectDerivativesLeg, expectWord},
				{SettingPortfolioMargin, ScopeAccount, SubjectAccount, expectAttested},
				{SettingMarginMode, ScopeSymbol, SubjectDerivativesLeg, expectWord},
				{SettingLeverage, ScopeSymbol, SubjectDerivativesLeg, expectLeveragePin},
				{SettingAutoAddMargin, ScopeSymbol, SubjectDerivativesLeg, expectWord},
			},
		},
		{
			dialect: DialectBybitUTA,
			want: []row{
				{SettingMarginMode, ScopeAccount, SubjectAccount, expectWord},
				{SettingAccountKind, ScopeAccount, SubjectAccount, expectNumberIn},
				{SettingSpotHedging, ScopeAccount, SubjectAccount, expectWord},
				{SettingSpotMarginMode, ScopeAccount, SubjectSpotLeg, expectWord},
				{SettingBorrowAmount, ScopeAsset, SubjectSpotLeg, expectZero},
				{SettingSpotBorrow, ScopeAsset, SubjectSpotLeg, expectZero},
				{SettingPositionMode, ScopeSymbol, SubjectDerivativesLeg, expectWord},
				{SettingLeverage, ScopeSymbol, SubjectDerivativesLeg, expectLeveragePin},
				{SettingAutoAddMargin, ScopeSymbol, SubjectDerivativesLeg, expectWord},
			},
		},
	} {
		t.Run(string(tc.dialect), func(t *testing.T) {
			rules, err := rulesFor(tc.dialect)
			if err != nil {
				t.Fatalf("rulesFor: %v", err)
			}
			if len(rules) != len(tc.want) {
				t.Fatalf("table holds %d rules, Profiles §2 states %d", len(rules), len(tc.want))
			}
			for i, r := range rules {
				got := row{r.setting, r.scope, r.subject, r.expect.kind}
				if got != tc.want[i] {
					t.Fatalf("rule %d is %s/%s/%s/%d, want %s/%s/%s/%d",
						i, got.setting, got.scope, got.subject, got.kind,
						tc.want[i].setting, tc.want[i].scope, tc.want[i].subject, tc.want[i].kind)
				}
			}
		})
	}

	// Scope and subject must DISAGREE somewhere on this table, or every test
	// above passes against an implementation that derived one from the other —
	// which is the defect the axis exists to remove. Two independent witnesses,
	// in both directions.
	if a, b := findRule(t, DialectBinanceClassic, SettingPositionMode), findRule(t, DialectBybitUTA, SettingPositionMode); a.subject != b.subject || a.scope == b.scope {
		t.Errorf("SettingPositionMode is %s/%s on Binance and %s/%s on Bybit; the axes are meant to be one subject at two SCOPES",
			a.scope, a.subject, b.scope, b.subject)
	}
	if a, b := findRule(t, DialectBybitUTA, SettingSpotHedging), findRule(t, DialectBybitUTA, SettingSpotMarginMode); a.scope != b.scope || a.subject == b.subject {
		t.Errorf("SpotHedging is %s/%s and SpotMarginMode is %s/%s; the axes are meant to be one SCOPE with two subjects",
			a.scope, a.subject, b.scope, b.subject)
	}

	// The admitted values are asserted as literals for the same reason: a
	// whitelist widened by one member is invisible to a test that reads the
	// whitelist.
	if got := findRule(t, DialectBybitUTA, SettingAccountKind).expect.numbers; len(got) != 4 ||
		got[0] != 3 || got[1] != 4 || got[2] != 5 || got[3] != 6 {
		t.Fatalf("unifiedMarginStatus admits %v, want exactly {3,4,5,6}", got)
	}
	if got := findRule(t, DialectBybitUTA, SettingMarginMode).expect.words; len(got) != 1 || got[0] != "ISOLATED_MARGIN" {
		t.Fatalf("Bybit margin mode admits %v, want exactly ISOLATED_MARGIN", got)
	}
	if got := findRule(t, DialectBinanceClassic, SettingMarginMode).expect.words; len(got) != 1 || got[0] != "ISOLATED" {
		t.Fatalf("Binance margin mode admits %v, want exactly ISOLATED", got)
	}
}

func findRule(t *testing.T, d Dialect, s Setting) rule {
	t.Helper()
	rules, err := rulesFor(d)
	if err != nil {
		t.Fatalf("rulesFor(%s): %v", d, err)
	}
	for _, r := range rules {
		if r.setting == s {
			return r
		}
	}
	t.Fatalf("%s has no rule for %s", d, s)
	return rule{}
}

// TestSubjectLivenessIsATableNotALadder states Live's whole truth table by
// hand (task #199, record §73p).
//
// A subject is a point in a product of properties, not a rung: the members do
// not order, and "which coverage lapses it" is the only question Live answers.
// Both coverages appear for every member, because an implementation returning
// `c == RosterCoverageDerivatives` for everything passes any test that only ever
// asks about the derivatives leg.
func TestSubjectLivenessIsATableNotALadder(t *testing.T) {
	for _, tc := range []struct {
		subject  Subject
		coverage RosterCoverage
		want     bool
	}{
		{SubjectAccount, RosterCoverageDerivatives, true},
		{SubjectAccount, RosterCoverageNoDerivatives, true},
		{SubjectSpotLeg, RosterCoverageDerivatives, true},
		{SubjectSpotLeg, RosterCoverageNoDerivatives, true},
		{SubjectDerivativesLeg, RosterCoverageDerivatives, true},
		{SubjectDerivativesLeg, RosterCoverageNoDerivatives, false},

		// The sentinel lapses rather than running. It is unreachable through the
		// tables (auditRules refuses it), and the direction still matters: a
		// lapse is RECORDED in the verdict, so an unclassified rule that somehow
		// arrived would be visible, where one silently treated as live would
		// have been judged against a deployment nobody said it applied to.
		{SubjectUnspecified, RosterCoverageDerivatives, false},
		{SubjectUnspecified, RosterCoverageNoDerivatives, false},

		// An UNSPECIFIED coverage lapses every leg rule. Requirement.Validate
		// refuses it upstream, so this is the belt: the alternative reading —
		// treating an unstated coverage as "derivatives" — is the one that runs
		// checks on a deployment whose shape was never declared.
		{SubjectDerivativesLeg, RosterCoverageUnspecified, false},
		{SubjectAccount, RosterCoverageUnspecified, true},
	} {
		if got := tc.subject.Live(tc.coverage); got != tc.want {
			t.Errorf("%s.Live(%s) = %v, want %v", tc.subject, tc.coverage, got, tc.want)
		}
	}
}

// TestProbedSettingsNarrowsByCoverageAndDropsTheAttestation pins the four
// (dialect, coverage) answers as literal counts.
//
// The counts are the instrument: "Binance under NoDerivatives is empty" is the
// ruling itself, and "Bybit under NoDerivatives is six, not nine and not zero"
// is what shows the narrowing reads the subject column rather than the venue's
// name or the roster's emptiness.
func TestProbedSettingsNarrowsByCoverageAndDropsTheAttestation(t *testing.T) {
	for _, tc := range []struct {
		dialect  Dialect
		coverage RosterCoverage
		want     int
	}{
		// Six rules, five probed: the Portfolio-Margin attestation has no venue
		// read path and must never be counted as one (record §73b F1).
		{DialectBinanceClassic, RosterCoverageDerivatives, 5},
		// The ruling. Every probed Binance rule governs the derivatives leg.
		{DialectBinanceClassic, RosterCoverageNoDerivatives, 0},
		{DialectBybitUTA, RosterCoverageDerivatives, 9},
		// Nine minus the three per-symbol derivatives rules. NOT zero: three
		// account-subject and three spot-subject rules still need a read, which
		// is why this venue still requires the family they ride.
		{DialectBybitUTA, RosterCoverageNoDerivatives, 6},
	} {
		got, err := tc.dialect.ProbedSettings(tc.coverage)
		if err != nil {
			t.Fatalf("ProbedSettings(%s, %s): %v", tc.dialect, tc.coverage, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s under %s probes %d settings %v, want %d",
				tc.dialect, tc.coverage, len(got), got, tc.want)
		}
		for _, s := range got {
			if s == SettingPortfolioMargin {
				t.Errorf("%s under %s asks the venue for %s, which has no read path",
					tc.dialect, tc.coverage, s)
			}
		}
	}
}

// TestAuditRulesRefusesAnUnclassifiedRow is the negative control for the audit,
// and it needs its own fixture because both real tables are complete.
//
// Without it the audit is a predicate nothing has ever made object — and the
// direction is the one that matters: an unclassified row LAPSES, so the failure
// it prevents is a check quietly not running rather than a crash.
func TestAuditRulesRefusesAnUnclassifiedRow(t *testing.T) {
	unclassified := []rule{{
		setting: SettingMultiAssets,
		scope:   ScopeAccount,
		expect:  expectation{kind: expectWord, words: []string{"false"}},
		why:     "a row somebody added without thinking about the subject axis",
	}}
	if err := auditRules(DialectBinanceClassic, unclassified); !errors.Is(err, ErrSubjectInvalid) {
		t.Errorf("auditRules accepted an unclassified row with %v, want ErrSubjectInvalid", err)
	}
	// The positive control: the real tables pass, or the audit would be a
	// predicate that objects to everything.
	for _, d := range []Dialect{DialectBinanceClassic, DialectBybitUTA} {
		if _, err := rulesFor(d); err != nil {
			t.Errorf("rulesFor(%s) = %v, want the real table to pass its own audit", d, err)
		}
	}
}

// TestCheckWithoutVenueReadRefusesADialectThatOwesOne is the guard on the one
// path that judges an account without looking at it.
//
// Three of the four (dialect, coverage) pairs owe a read, and all three are
// asserted: a precondition tested only on the pair it was written for is a
// precondition that could have been `dialect == DialectBinanceClassic`.
func TestCheckWithoutVenueReadRefusesADialectThatOwesOne(t *testing.T) {
	for _, tc := range []struct {
		dialect  Dialect
		coverage RosterCoverage
	}{
		{DialectBinanceClassic, RosterCoverageDerivatives},
		{DialectBybitUTA, RosterCoverageDerivatives},
		{DialectBybitUTA, RosterCoverageNoDerivatives},
	} {
		req := requirementFor(t, tc.dialect, tc.coverage)
		if _, err := CheckWithoutVenueRead(req); !errors.Is(err, ErrVenueReadOwed) {
			t.Errorf("%s under %s was judged without a read (err %v), want ErrVenueReadOwed",
				tc.dialect, tc.coverage, err)
		}
	}

	// The one pair that may: Binance with no derivative configured. It must
	// still EVALUATE the attestation — a verdict of zero checks is refused by
	// Satisfied, which is what keeps this path from becoming a clean answer for
	// an account nobody read.
	req := requirementFor(t, DialectBinanceClassic, RosterCoverageNoDerivatives)
	verdict, err := CheckWithoutVenueRead(req)
	if err != nil {
		t.Fatalf("CheckWithoutVenueRead on a spot-only Binance requirement: %v", err)
	}
	if verdict.Evaluated != 1 {
		t.Errorf("evaluated %d checks, want exactly 1 (the attestation):\n%s", verdict.Evaluated, verdict)
	}
	if len(verdict.Lapsed) != 5 {
		t.Errorf("recorded %d lapses, want 5:\n%s", len(verdict.Lapsed), verdict)
	}
	if ok, err := verdict.Satisfied(); !ok || err != nil {
		t.Errorf("Satisfied() = %v, %v; the attestation is OFF, so the posture holds:\n%s", ok, err, verdict)
	}
	// The instant is the ZERO one, deliberately: there was no read, so there is
	// no moment at which the account was seen in this state. A local wall-clock
	// reading here would be a fact nobody established.
	if verdict.ObservedAt.Validate() == nil {
		t.Error("the no-read verdict carries a valid observation instant — nothing was observed")
	}
}

// requirementFor builds a minimal valid requirement for a (dialect, coverage)
// pair, with the attestation OFF so the attested rule passes.
func requirementFor(t *testing.T, d Dialect, c RosterCoverage) Requirement {
	t.Helper()
	req := Requirement{
		Venue:           "binance",
		Dialect:         d,
		Coverage:        c,
		PortfolioMargin: AttestationOff,
	}
	if d == DialectBybitUTA {
		// Bybit's table has no attested rule, and pinning one there is refused
		// (#179's precedent: an attestation nothing consumes is a claim nobody
		// reads). The asymmetry is the table's, not this helper's.
		req.Venue, req.PortfolioMargin = "bybit", AttestationUnstated
	}
	if c == RosterCoverageDerivatives {
		req.Symbols = []SymbolPin{{Symbol: "BTCUSDT", Leverage: 5}}
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("requirementFor(%s, %s): %v", d, c, err)
	}
	return req
}

// Every rule must be well formed AND cited. The citation is a field rather than
// a comment precisely so this can be checked: a rule added without one is a
// requirement nobody can trace back to a verified venue fact.
func TestEveryRuleIsWellFormedAndCited(t *testing.T) {
	for _, d := range []Dialect{DialectBinanceClassic, DialectBybitUTA} {
		rules, err := rulesFor(d)
		if err != nil {
			t.Fatalf("rulesFor(%s): %v", d, err)
		}
		if len(rules) == 0 {
			t.Fatalf("%s has an EMPTY rule table — it would evaluate nothing and report a clean posture", d)
		}
		for _, r := range rules {
			if err := r.setting.Validate(); err != nil {
				t.Fatalf("%s: %v", d, err)
			}
			if err := r.scope.Validate(); err != nil {
				t.Fatalf("%s %s: %v", d, r.setting, err)
			}
			if r.why == "" {
				t.Fatalf("%s %s cites no clause", d, r.setting)
			}
			switch r.expect.kind {
			case expectWord:
				if len(r.expect.words) == 0 {
					t.Fatalf("%s %s admits NO words — every value would refuse", d, r.setting)
				}
			case expectNumberIn:
				if len(r.expect.numbers) == 0 {
					t.Fatalf("%s %s admits NO numbers — every value would refuse", d, r.setting)
				}
			case expectLeveragePin, expectZero, expectAttested:
				// No table data of their own: the pin comes from the roster,
				// zero and the attestation are the comparison itself.
			}
		}
	}
}

func TestRulesForRefusesAnUnknownDialectRatherThanReturningNothing(t *testing.T) {
	rules, err := rulesFor(Dialect("binance-portfolio-margin"))
	if !errors.Is(err, ErrDialectUnknown) {
		t.Fatalf("err = %v, want ErrDialectUnknown", err)
	}
	if rules != nil {
		t.Fatalf("an unknown dialect returned %d rules", len(rules))
	}
}

// ConsumesAttestation is derived from the table rather than restated as a
// constant, so the two cannot disagree. This is the test that would catch a
// constant reintroduced beside it.
func TestDialectConsumesAttestationTracksTheTable(t *testing.T) {
	if !DialectBinanceClassic.ConsumesAttestation() {
		t.Fatal("Binance classic has the Portfolio-Margin rule and must consume the attestation")
	}
	if DialectBybitUTA.ConsumesAttestation() {
		t.Fatal("Bybit UTA has no Portfolio-Margin rule and must not consume the attestation")
	}
	if Dialect("unknown").ConsumesAttestation() {
		t.Fatal("an unknown dialect consumes nothing")
	}
}

func TestCanonicalNumberIsOneReadingOfTheVenuesText(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		err  bool
	}{
		{in: "0", want: "0"},
		{in: "0.00000000", want: "0"},
		{in: "-0", want: "0"},
		{in: "20", want: "20"},
		{in: "20.00", want: "20"},
		{in: "20.5", want: "20.5"},
		{in: "-1", want: "-1"},
		{in: "", err: true},
		{in: "none", err: true},
		// Exponent notation is read, not refused — the shared parser supports
		// it deliberately, and a venue that publishes "1e-8" has stated a
		// number rather than something unreadable. Pinned here because this
		// package inherits the behaviour rather than choosing it, so a change
		// underneath should surface as a failing test in the package that
		// depends on it.
		{in: "1e3", want: "1000"},
		{in: "0e0", want: "0"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := canonicalNumber(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("canonicalNumber(%q) = %q, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalNumber(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("canonicalNumber(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A NEGATIVE borrow is not a zero. The check is "is this figure zero", and a
// sign test that asked "is it not positive" would admit it — which is the
// negative-assertion trap, on a value a venue could legitimately publish for a
// repaid loan.
func TestZeroExpectationRefusesANegativeFigure(t *testing.T) {
	e := expectation{kind: expectZero}
	if ok, _ := e.satisfiedBy("-0.5", ""); ok {
		t.Fatal("a negative borrow figure must not satisfy a zero requirement")
	}
	if ok, _ := e.satisfiedBy("0", ""); !ok {
		t.Fatal("positive control: a stated zero must satisfy it")
	}
}

// The attested kind never reaches the value comparison — evaluateRule answers
// it before any value exists. The branch is defensive, and it refuses: a future
// caller that reaches it must not get a silent yes.
func TestAttestedExpectationRefusesIfItIsEverComparedAgainstAValue(t *testing.T) {
	ok, reason := expectation{kind: expectAttested}.satisfiedBy("false", "")
	if ok {
		t.Fatal("the attested kind must never be satisfied by a wire value")
	}
	if reason != ReasonUnattested {
		t.Fatalf("reason %s, want ReasonUnattested", reason)
	}
}

func TestEnumeratedAssetsIsSortedAndDeduplicated(t *testing.T) {
	obs := []Observation{
		{Setting: SettingBorrowAmount, Scope: ScopeAsset, Subject: "USDT", Presence: PresenceAbsent},
		{Setting: SettingSpotBorrow, Scope: ScopeAsset, Subject: "USDT", Presence: PresenceAbsent},
		{Setting: SettingBorrowAmount, Scope: ScopeAsset, Subject: "BTC", Presence: PresenceAbsent},
		{Setting: SettingLeverage, Scope: ScopeSymbol, Subject: "ETHUSDT", Presence: PresenceAbsent},
		{Setting: SettingMultiAssets, Scope: ScopeAccount, Presence: PresenceAbsent},
	}
	got := enumeratedAssets(obs)
	if len(got) != 2 || got[0] != "BTC" || got[1] != "USDT" {
		t.Fatalf("enumeratedAssets = %v, want [BTC USDT] — sorted, deduped, and asset-scoped only", got)
	}
}
