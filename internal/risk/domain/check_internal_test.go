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
	type row struct {
		setting Setting
		scope   Scope
		kind    expectationKind
	}

	for _, tc := range []struct {
		dialect Dialect
		want    []row
	}{
		{
			dialect: DialectBinanceClassic,
			want: []row{
				{SettingMultiAssets, ScopeAccount, expectWord},
				{SettingPositionMode, ScopeAccount, expectWord},
				{SettingPortfolioMargin, ScopeAccount, expectAttested},
				{SettingMarginMode, ScopeSymbol, expectWord},
				{SettingLeverage, ScopeSymbol, expectLeveragePin},
				{SettingAutoAddMargin, ScopeSymbol, expectWord},
			},
		},
		{
			dialect: DialectBybitUTA,
			want: []row{
				{SettingMarginMode, ScopeAccount, expectWord},
				{SettingAccountKind, ScopeAccount, expectNumberIn},
				{SettingSpotHedging, ScopeAccount, expectWord},
				{SettingSpotMarginMode, ScopeAccount, expectWord},
				{SettingBorrowAmount, ScopeAsset, expectZero},
				{SettingSpotBorrow, ScopeAsset, expectZero},
				{SettingPositionMode, ScopeSymbol, expectWord},
				{SettingLeverage, ScopeSymbol, expectLeveragePin},
				{SettingAutoAddMargin, ScopeSymbol, expectWord},
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
				got := row{r.setting, r.scope, r.expect.kind}
				if got != tc.want[i] {
					t.Fatalf("rule %d is %s/%s/%d, want %s/%s/%d",
						i, got.setting, got.scope, got.kind,
						tc.want[i].setting, tc.want[i].scope, tc.want[i].kind)
				}
			}
		})
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

// dialectConsumesAttestation is derived from the table rather than restated as
// a constant, so the two cannot disagree. This is the test that would catch a
// constant reintroduced beside it.
func TestDialectConsumesAttestationTracksTheTable(t *testing.T) {
	if !dialectConsumesAttestation(DialectBinanceClassic) {
		t.Fatal("Binance classic has the Portfolio-Margin rule and must consume the attestation")
	}
	if dialectConsumesAttestation(DialectBybitUTA) {
		t.Fatal("Bybit UTA has no Portfolio-Margin rule and must not consume the attestation")
	}
	if dialectConsumesAttestation(Dialect("unknown")) {
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
