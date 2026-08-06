package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// The fixtures below are built against record §73g's collapse checklist, and
// the parameter choices are deliberate rather than arbitrary:
//
//   - TWO symbols everywhere, with each violation placed on the SECOND, so
//     "every configured symbol" and "the first configured symbol" cannot give
//     the same answer;
//   - DIFFERENT leverage pins per symbol (5 and 3), so "this symbol's pin" and
//     "the first pin" cannot give the same answer either;
//   - both dialects for every shared concept, since one venue collapses each
//     per-venue rule into "the rule";
//   - exactly TWO simultaneous failures in the multi-failure fixture, because
//     an all-failing posture collapses "reports every failure" into "reports
//     something";
//   - the pin above AND below the venue's value, because a pin equal to the
//     venue's answer in every fixture is the vacuous check of record §73b F3.

const (
	testBinance = ids.Venue("binance")
	testBybit   = ids.Venue("bybit")

	firstSymbol  = "BTCUSDT"
	secondSymbol = "ETHUSDT"
	firstPin     = 5
	secondPin    = 3
)

func observedAt(t *testing.T) clock.ServerTime {
	t.Helper()
	// A fixed instant, not the ambient clock: §3.6 rule 5's seam applies to
	// tests too, and a date literal makes the golden renders below stable.
	st, err := clock.NewServerTime(time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC), 25*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return st
}

func acct(s domain.Setting, value string) domain.Observation {
	return domain.Observation{
		Setting: s, Scope: domain.ScopeAccount, Value: value,
		Presence: domain.PresenceStated, Source: "probe/account",
	}
}

func sym(s domain.Setting, subject, value string) domain.Observation {
	return domain.Observation{
		Setting: s, Scope: domain.ScopeSymbol, Subject: subject, Value: value,
		Presence: domain.PresenceStated, Source: "probe/symbol",
	}
}

func asset(s domain.Setting, subject, value string) domain.Observation {
	return domain.Observation{
		Setting: s, Scope: domain.ScopeAsset, Subject: subject, Value: value,
		Presence: domain.PresenceStated, Source: "probe/wallet",
	}
}

func unsaid(s domain.Setting, scope domain.Scope, subject string, p domain.Presence) domain.Observation {
	return domain.Observation{
		Setting: s, Scope: scope, Subject: subject, Presence: p, Source: "probe/unsaid",
	}
}

func binanceRequirement() domain.Requirement {
	return domain.Requirement{
		Venue:   testBinance,
		Dialect: domain.DialectBinanceClassic,
		Symbols: []domain.SymbolPin{
			{Symbol: firstSymbol, Leverage: firstPin},
			{Symbol: secondSymbol, Leverage: secondPin},
		},
		PortfolioMargin: domain.AttestationOff,
	}
}

func bybitRequirement() domain.Requirement {
	return domain.Requirement{
		Venue:   testBybit,
		Dialect: domain.DialectBybitUTA,
		Symbols: []domain.SymbolPin{
			{Symbol: firstSymbol, Leverage: firstPin},
			{Symbol: secondSymbol, Leverage: secondPin},
		},
	}
}

// binanceReport is a CLEAN Binance posture: three account settings and three
// per-symbol settings for both configured symbols.
func binanceReport(t *testing.T) domain.Report {
	t.Helper()
	return domain.Report{
		Venue:      testBinance,
		ObservedAt: observedAt(t),
		Observations: []domain.Observation{
			acct(domain.SettingMultiAssets, "false"),
			acct(domain.SettingPositionMode, "false"),
			sym(domain.SettingMarginMode, firstSymbol, "ISOLATED"),
			sym(domain.SettingLeverage, firstSymbol, "5"),
			sym(domain.SettingAutoAddMargin, firstSymbol, "false"),
			sym(domain.SettingMarginMode, secondSymbol, "ISOLATED"),
			sym(domain.SettingLeverage, secondSymbol, "3"),
			sym(domain.SettingAutoAddMargin, secondSymbol, "false"),
		},
	}
}

// bybitReport is a CLEAN Bybit posture: four account settings, two coins'
// borrow figures, and three per-symbol settings for both configured symbols.
func bybitReport(t *testing.T) domain.Report {
	t.Helper()
	return domain.Report{
		Venue:      testBybit,
		ObservedAt: observedAt(t),
		Observations: []domain.Observation{
			acct(domain.SettingMarginMode, "ISOLATED_MARGIN"),
			acct(domain.SettingAccountKind, "5"),
			acct(domain.SettingSpotHedging, "OFF"),
			acct(domain.SettingSpotMarginMode, "0"),
			asset(domain.SettingBorrowAmount, "USDT", "0"),
			asset(domain.SettingSpotBorrow, "USDT", "0"),
			asset(domain.SettingBorrowAmount, "BTC", "0.00000000"),
			asset(domain.SettingSpotBorrow, "BTC", "0"),
			sym(domain.SettingPositionMode, firstSymbol, "0"),
			sym(domain.SettingLeverage, firstSymbol, "5"),
			sym(domain.SettingAutoAddMargin, firstSymbol, "0"),
			sym(domain.SettingPositionMode, secondSymbol, "0"),
			sym(domain.SettingLeverage, secondSymbol, "3"),
			sym(domain.SettingAutoAddMargin, secondSymbol, "0"),
		},
	}
}

// replace swaps the observation for one (setting, subject) and fails the test
// if there was not exactly one to swap.
//
// Fatal on a miss rather than appending, because a fixture mutation that
// silently ADDS an observation instead of replacing one produces a duplicate —
// which this package reports as ambiguous, so the test would still "fail
// correctly" for entirely the wrong reason.
func replace(t *testing.T, r domain.Report, o domain.Observation) domain.Report {
	t.Helper()
	found := 0
	out := make([]domain.Observation, 0, len(r.Observations))
	for _, existing := range r.Observations {
		if existing.Setting == o.Setting && existing.Subject == o.Subject && existing.Scope == o.Scope {
			found++
			out = append(out, o)
			continue
		}
		out = append(out, existing)
	}
	if found != 1 {
		t.Fatalf("replace %s[%s]: matched %d observations, want 1", o.Setting, o.Subject, found)
	}
	r.Observations = out
	return r
}

func drop(t *testing.T, r domain.Report, s domain.Setting, subject string) domain.Report {
	t.Helper()
	found := 0
	out := make([]domain.Observation, 0, len(r.Observations))
	for _, existing := range r.Observations {
		if existing.Setting == s && existing.Subject == subject {
			found++
			continue
		}
		out = append(out, existing)
	}
	if found == 0 {
		t.Fatalf("drop %s[%s]: matched nothing", s, subject)
	}
	r.Observations = out
	return r
}

func checkT(t *testing.T, req domain.Requirement, rep domain.Report) domain.Verdict {
	t.Helper()
	v, err := domain.Check(req, rep)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return v
}

// assertClean is the positive control every negative case is paired with: a
// harness that reports "the broken thing failed" proves nothing until the real
// thing passes, or a universally-objecting predicate reads as a perfect score.
func assertClean(t *testing.T, v domain.Verdict, wantEvaluated int) {
	t.Helper()
	ok, err := v.Satisfied()
	if err != nil {
		t.Fatalf("Satisfied: %v", err)
	}
	if !ok {
		t.Fatalf("posture not satisfied, failures:\n%s", v)
	}
	if v.Evaluated != wantEvaluated {
		t.Fatalf("evaluated %d checks, want %d", v.Evaluated, wantEvaluated)
	}
}

func failureFor(t *testing.T, v domain.Verdict, s domain.Setting, subject string) domain.Failure {
	t.Helper()
	var found []domain.Failure
	for _, f := range v.Failures {
		if f.Setting == s && f.Subject == subject {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one failure for %s[%s], got %d in:\n%s", s, subject, len(found), v)
	}
	return found[0]
}

// --- positive controls -------------------------------------------------

// The exact counts are the instrument. "Evaluated > 0" would be satisfied by
// the degenerate value a broken fold produces, while 9 and 14 are predictable
// from the rule tables (3 account + 3×2 symbol; 4 account + 2×2 asset + 3×2
// symbol) and any rule silently skipped moves them.
func TestCheckCleanBinancePostureIsSatisfied(t *testing.T) {
	assertClean(t, checkT(t, binanceRequirement(), binanceReport(t)), 9)
}

func TestCheckCleanBybitPostureIsSatisfied(t *testing.T) {
	assertClean(t, checkT(t, bybitRequirement(), bybitReport(t)), 14)
}

// --- coverage from the CONFIGURED roster (record §73b F4) --------------

func TestCheckRefusesAConfiguredSymbolTheVenueNeverMentioned(t *testing.T) {
	// The violation is on the SECOND symbol: with one configured symbol, or
	// with the gap on the first, "every symbol" and "the first symbol" agree.
	rep := drop(t, binanceReport(t), domain.SettingAutoAddMargin, secondSymbol)
	v := checkT(t, binanceRequirement(), rep)

	if ok, _ := v.Satisfied(); ok {
		t.Fatal("a configured symbol with no observation must refuse")
	}
	if len(v.Failures) != 1 {
		t.Fatalf("want exactly the one gap reported, got:\n%s", v)
	}
	f := failureFor(t, v, domain.SettingAutoAddMargin, secondSymbol)
	if f.Reason != domain.ReasonUnobserved {
		t.Fatalf("reason %s, want ReasonUnobserved", f.Reason)
	}
	// The count does not shrink with the evidence: an unobserved check still
	// RAN. If it did not count, a report that mentioned nothing at all would
	// evaluate zero checks and Satisfied would refuse as vacuous rather than
	// listing what is missing.
	if v.Evaluated != 9 {
		t.Fatalf("evaluated %d, want 9", v.Evaluated)
	}
}

func TestCheckRefusesAConfiguredSymbolTheVenueNeverMentionedOnBybit(t *testing.T) {
	rep := drop(t, bybitReport(t), domain.SettingPositionMode, secondSymbol)
	v := checkT(t, bybitRequirement(), rep)

	f := failureFor(t, v, domain.SettingPositionMode, secondSymbol)
	if f.Reason != domain.ReasonUnobserved {
		t.Fatalf("reason %s, want ReasonUnobserved", f.Reason)
	}
	if v.Evaluated != 14 {
		t.Fatalf("evaluated %d, want 14", v.Evaluated)
	}
}

// An empty report is the vacuity this package is written against: every rule
// must fail BY NAME and the count must equal the table's full size.
//
// The Binance requirement here is deliberately UNATTESTED, because the
// attestation check reads the operator's word rather than the report — so with
// an attestation pinned, eight of the nine checks fail on an empty report and
// the ninth legitimately passes. That is the correct behaviour and it is
// asserted on its own below; here it would merely make the count lie.
func TestCheckEmptyReportFailsEveryCheckAndCountsThem(t *testing.T) {
	unattested := binanceRequirement()
	unattested.PortfolioMargin = domain.AttestationUnstated

	for _, tc := range []struct {
		name string
		req  domain.Requirement
		rep  domain.Report
		want int
	}{
		{"binance", unattested, domain.Report{Venue: testBinance, ObservedAt: observedAt(t)}, 9},
		{"bybit", bybitRequirement(), domain.Report{Venue: testBybit, ObservedAt: observedAt(t)}, 14 - 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := checkT(t, tc.req, tc.rep)
			if v.Evaluated != tc.want {
				t.Fatalf("evaluated %d, want %d", v.Evaluated, tc.want)
			}
			if len(v.Failures) != tc.want {
				t.Fatalf("failed %d of %d checks, want all of them:\n%s", len(v.Failures), tc.want, v)
			}
			if ok, err := v.Satisfied(); ok || err != nil {
				t.Fatalf("Satisfied() = %v, %v; want false, nil", ok, err)
			}
		})
	}
}

// The Bybit asset rules have no configured roster to answer coverage from, so
// an empty coin enumeration must refuse rather than pass over nothing.
func TestCheckRefusesAnEmptyCoinEnumeration(t *testing.T) {
	rep := bybitReport(t)
	kept := make([]domain.Observation, 0, len(rep.Observations))
	for _, o := range rep.Observations {
		if o.Scope != domain.ScopeAsset {
			kept = append(kept, o)
		}
	}
	rep.Observations = kept

	v := checkT(t, bybitRequirement(), rep)
	if len(v.Failures) != 2 {
		t.Fatalf("want both borrow requirements refused, got:\n%s", v)
	}
	for _, f := range v.Failures {
		if f.Reason != domain.ReasonUnobserved {
			t.Fatalf("%s: reason %s, want ReasonUnobserved", f.Setting, f.Reason)
		}
		if f.Subject != "" {
			t.Fatalf("%s: subject %q, want empty (no coin was named)", f.Setting, f.Subject)
		}
	}
	// 4 account + 2 asset-with-no-coin + 6 symbol.
	if v.Evaluated != 12 {
		t.Fatalf("evaluated %d, want 12", v.Evaluated)
	}
}

// --- every failure is reported, not the first --------------------------

func TestCheckReportsEveryFailureNotOnlyTheFirst(t *testing.T) {
	// Exactly TWO failures, deliberately: an all-failing fixture collapses
	// "reports every failure" into "reports something". They sit on different
	// scopes and different symbols so neither can be the other's echo.
	rep := binanceReport(t)
	rep = replace(t, rep, acct(domain.SettingMultiAssets, "true"))
	rep = replace(t, rep, sym(domain.SettingMarginMode, secondSymbol, "CROSSED"))

	v := checkT(t, binanceRequirement(), rep)
	if len(v.Failures) != 2 {
		t.Fatalf("want exactly 2 failures, got %d:\n%s", len(v.Failures), v)
	}
	first := failureFor(t, v, domain.SettingMultiAssets, "")
	second := failureFor(t, v, domain.SettingMarginMode, secondSymbol)
	if first.Reason != domain.ReasonMismatch || second.Reason != domain.ReasonMismatch {
		t.Fatalf("reasons %s / %s, want two mismatches", first.Reason, second.Reason)
	}
	if first.Observed != "true" || second.Observed != "CROSSED" {
		t.Fatalf("observed %q / %q, want the venue's own words", first.Observed, second.Observed)
	}
	// Account-wide first: the whole-venue refusal must not be buried under the
	// per-symbol rows an operator skims past.
	if v.Failures[0].Setting != domain.SettingMultiAssets {
		t.Fatalf("failures are not scope-ordered:\n%s", v)
	}
}

func TestCheckSortsFailuresDeterministically(t *testing.T) {
	rep := domain.Report{Venue: testBybit, ObservedAt: observedAt(t)}
	first := checkT(t, bybitRequirement(), rep)
	second := checkT(t, bybitRequirement(), rep)

	if len(first.Failures) != len(second.Failures) {
		t.Fatalf("two runs disagree on failure count")
	}
	for i := range first.Failures {
		if first.Failures[i] != second.Failures[i] {
			t.Fatalf("failure %d differs between runs: %v vs %v", i, first.Failures[i], second.Failures[i])
		}
	}
	// Scope is the primary key, so the account rows come before the per-symbol
	// and per-asset ones regardless of table order.
	for i := 1; i < len(first.Failures); i++ {
		if first.Failures[i].Scope < first.Failures[i-1].Scope {
			t.Fatalf("failures are not scope-ordered at %d:\n%s", i, first)
		}
	}
}

// --- the borrow trap: an empty figure is not a zero (record §73b F2) ---

func TestCheckBorrowEmptyIsNotZero(t *testing.T) {
	for _, tc := range []struct {
		name       string
		obs        domain.Observation
		wantReason domain.FailureReason
		clean      bool
	}{
		{name: "a stated zero passes", obs: asset(domain.SettingBorrowAmount, "BTC", "0"), clean: true},
		{name: "a scaled zero passes", obs: asset(domain.SettingBorrowAmount, "BTC", "0.000000000"), clean: true},
		{
			name:       "an EMPTY figure refuses",
			obs:        unsaid(domain.SettingBorrowAmount, domain.ScopeAsset, "BTC", domain.PresenceStatedEmpty),
			wantReason: domain.ReasonUnstated,
		},
		{
			name:       "an ABSENT figure refuses",
			obs:        unsaid(domain.SettingBorrowAmount, domain.ScopeAsset, "BTC", domain.PresenceAbsent),
			wantReason: domain.ReasonUnstated,
		},
		{
			name:       "an outstanding borrow refuses",
			obs:        asset(domain.SettingBorrowAmount, "BTC", "0.5"),
			wantReason: domain.ReasonMismatch,
		},
		{
			name:       "an unreadable figure refuses as unreadable, not as a mismatch",
			obs:        asset(domain.SettingBorrowAmount, "BTC", "none"),
			wantReason: domain.ReasonUnreadable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := checkT(t, bybitRequirement(), replace(t, bybitReport(t), tc.obs))
			if tc.clean {
				assertClean(t, v, 14)
				return
			}
			f := failureFor(t, v, domain.SettingBorrowAmount, "BTC")
			if f.Reason != tc.wantReason {
				t.Fatalf("reason %s, want %s", f.Reason, tc.wantReason)
			}
		})
	}
}

// The two unstated presences give one verdict and two DIAGNOSES, and the
// diagnosis is the whole reason they are separate members.
func TestCheckPreservesWhichKindOfSilenceItWas(t *testing.T) {
	absentV := checkT(t, bybitRequirement(),
		replace(t, bybitReport(t), unsaid(domain.SettingSpotBorrow, domain.ScopeAsset, "USDT", domain.PresenceAbsent)))
	emptyV := checkT(t, bybitRequirement(),
		replace(t, bybitReport(t), unsaid(domain.SettingSpotBorrow, domain.ScopeAsset, "USDT", domain.PresenceStatedEmpty)))

	a := failureFor(t, absentV, domain.SettingSpotBorrow, "USDT")
	e := failureFor(t, emptyV, domain.SettingSpotBorrow, "USDT")
	if a.Reason != e.Reason {
		t.Fatalf("the two silences must reach the same verdict, got %s and %s", a.Reason, e.Reason)
	}
	if a.Observed == e.Observed {
		t.Fatalf("both silences rendered as %q — the diagnosis is lost", a.Observed)
	}
	if a.Observed != domain.PresenceAbsent.String() || e.Observed != domain.PresenceStatedEmpty.String() {
		t.Fatalf("observed %q / %q, want the two presence names", a.Observed, e.Observed)
	}
}

// --- the leverage pin (record §73b F3) ---------------------------------

func TestCheckLeveragePinIsComparedInBothDirections(t *testing.T) {
	for _, tc := range []struct {
		name       string
		venueSays  string
		wantReason domain.FailureReason
		clean      bool
	}{
		{name: "equal passes", venueSays: "3", clean: true},
		{name: "trailing zeros are the same number", venueSays: "3.00", clean: true},
		{name: "venue BELOW the pin refuses", venueSays: "2", wantReason: domain.ReasonMismatch},
		{name: "venue ABOVE the pin refuses", venueSays: "20", wantReason: domain.ReasonMismatch},
		{name: "the FIRST symbol's pin does not satisfy the second", venueSays: "5", wantReason: domain.ReasonMismatch},
		{name: "an unreadable leverage refuses", venueSays: "high", wantReason: domain.ReasonUnreadable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Placed on the SECOND symbol, whose pin (3) differs from the
			// first's (5) — so a check reading "the roster's first pin" gives a
			// different answer from one reading "this symbol's pin".
			rep := replace(t, binanceReport(t), sym(domain.SettingLeverage, secondSymbol, tc.venueSays))
			v := checkT(t, binanceRequirement(), rep)
			if tc.clean {
				assertClean(t, v, 9)
				return
			}
			f := failureFor(t, v, domain.SettingLeverage, secondSymbol)
			if f.Reason != tc.wantReason {
				t.Fatalf("reason %s, want %s", f.Reason, tc.wantReason)
			}
			if tc.wantReason == domain.ReasonMismatch && f.Required != "3" {
				t.Fatalf("required %q, want the second symbol's own pin %q", f.Required, "3")
			}
		})
	}
}

// --- the account-kind whitelist (record §73b F5) -----------------------

func TestCheckAccountKindIsAWhitelist(t *testing.T) {
	for _, tc := range []struct {
		venueSays  string
		wantReason domain.FailureReason
		clean      bool
	}{
		{venueSays: "3", clean: true},
		{venueSays: "4", clean: true},
		{venueSays: "5", clean: true},
		{venueSays: "6", clean: true},
		// 1 is the recorded known-bad. 2 is UNLISTED, and 7 stands for the
		// value the venue adds next year: under a blacklist both would pass.
		{venueSays: "1", wantReason: domain.ReasonMismatch},
		{venueSays: "2", wantReason: domain.ReasonMismatch},
		{venueSays: "7", wantReason: domain.ReasonMismatch},
		{venueSays: "0", wantReason: domain.ReasonMismatch},
		{venueSays: "unified", wantReason: domain.ReasonUnreadable},
	} {
		t.Run(tc.venueSays, func(t *testing.T) {
			rep := replace(t, bybitReport(t), acct(domain.SettingAccountKind, tc.venueSays))
			v := checkT(t, bybitRequirement(), rep)
			if tc.clean {
				assertClean(t, v, 14)
				return
			}
			f := failureFor(t, v, domain.SettingAccountKind, "")
			if f.Reason != tc.wantReason {
				t.Fatalf("reason %s, want %s", f.Reason, tc.wantReason)
			}
			if f.Required != "one of 3, 4, 5, 6" {
				t.Fatalf("required %q, want the admitted set spelled out", f.Required)
			}
		})
	}
}

// --- word comparison ----------------------------------------------------

// The fold must be observable: if every fixture spells the required word
// exactly as the rule table does, EqualFold and == are the same predicate.
func TestCheckWordComparisonFoldsCaseButNotMeaning(t *testing.T) {
	lower := replace(t, bybitReport(t), acct(domain.SettingMarginMode, "isolated_margin"))
	assertClean(t, checkT(t, bybitRequirement(), lower), 14)

	mixed := replace(t, binanceReport(t), acct(domain.SettingMultiAssets, "False"))
	assertClean(t, checkT(t, binanceRequirement(), mixed), 9)

	// Folding must not admit a different VALUE — "on" is not a spelling of
	// "off".
	on := replace(t, bybitReport(t), acct(domain.SettingSpotHedging, "on"))
	f := failureFor(t, checkT(t, bybitRequirement(), on), domain.SettingSpotHedging, "")
	if f.Reason != domain.ReasonMismatch {
		t.Fatalf("reason %s, want ReasonMismatch", f.Reason)
	}
}

// --- the scope is part of the key ---------------------------------------

// A mis-scoped observation must not satisfy a rule that asked a different
// question. The value here is CORRECT — it equals the symbol's pin — so the
// only thing standing between it and a pass is the scope.
func TestCheckAMisScopedObservationDoesNotSatisfyARule(t *testing.T) {
	rep := binanceReport(t)
	rep = drop(t, rep, domain.SettingLeverage, firstSymbol)
	rep.Observations = append(rep.Observations, domain.Observation{
		Setting: domain.SettingLeverage, Scope: domain.ScopeAsset, Subject: firstSymbol,
		Value: "5", Presence: domain.PresenceStated, Source: "probe/miscoped",
	})

	v := checkT(t, binanceRequirement(), rep)
	f := failureFor(t, v, domain.SettingLeverage, firstSymbol)
	if f.Reason != domain.ReasonUnobserved {
		t.Fatalf("reason %s, want ReasonUnobserved — a per-asset answer is not a per-symbol one", f.Reason)
	}
}

// --- duplicates ---------------------------------------------------------

func TestCheckRefusesTwoAnswersForOneKey(t *testing.T) {
	// Bybit returns one position row per SIDE in hedge mode, which is the
	// posture violation this rule exists to catch — so the duplicate is not
	// merely untidy, it is evidence.
	rep := bybitReport(t)
	rep.Observations = append(rep.Observations, sym(domain.SettingPositionMode, firstSymbol, "1"))

	v := checkT(t, bybitRequirement(), rep)
	f := failureFor(t, v, domain.SettingPositionMode, firstSymbol)
	if f.Reason != domain.ReasonAmbiguous {
		t.Fatalf("reason %s, want ReasonAmbiguous", f.Reason)
	}
	if !strings.Contains(f.Observed, "0") || !strings.Contains(f.Observed, "1") {
		t.Fatalf("observed %q, want both answers named", f.Observed)
	}
	// The duplicate is a check that ran: 14 rules + the ambiguity.
	if v.Evaluated != 15 {
		t.Fatalf("evaluated %d, want 15", v.Evaluated)
	}
}

// Two IDENTICAL answers are still two answers. Folding them needs a rule for
// which one wins, and no venue publishes one.
func TestCheckRefusesTwoIdenticalAnswersForOneKey(t *testing.T) {
	rep := bybitReport(t)
	rep.Observations = append(rep.Observations, sym(domain.SettingLeverage, secondSymbol, "3"))

	v := checkT(t, bybitRequirement(), rep)
	f := failureFor(t, v, domain.SettingLeverage, secondSymbol)
	if f.Reason != domain.ReasonAmbiguous {
		t.Fatalf("reason %s, want ReasonAmbiguous", f.Reason)
	}
}

// --- the attestation (record §73b F1, §73c-1) --------------------------

func TestCheckPortfolioMarginConsumesTheAttestationAndNothingElse(t *testing.T) {
	t.Run("attested off passes", func(t *testing.T) {
		assertClean(t, checkT(t, binanceRequirement(), binanceReport(t)), 9)
	})

	t.Run("unattested refuses by name", func(t *testing.T) {
		req := binanceRequirement()
		req.PortfolioMargin = domain.AttestationUnstated
		f := failureFor(t, checkT(t, req, binanceReport(t)), domain.SettingPortfolioMargin, "")
		if f.Reason != domain.ReasonUnattested {
			t.Fatalf("reason %s, want ReasonUnattested", f.Reason)
		}
		if f.Source != "" {
			t.Fatalf("source %q, want empty — no endpoint will ever answer this one", f.Source)
		}
	})

	t.Run("attested ON refuses as a mismatch", func(t *testing.T) {
		req := binanceRequirement()
		req.PortfolioMargin = domain.AttestationOn
		f := failureFor(t, checkT(t, req, binanceReport(t)), domain.SettingPortfolioMargin, "")
		if f.Reason != domain.ReasonMismatch {
			t.Fatalf("reason %s, want ReasonMismatch", f.Reason)
		}
	})

	t.Run("an attestation stands even when the venue said nothing at all", func(t *testing.T) {
		// The complement of the case below: the attestation is evidence from a
		// different source, so a report that answered NOTHING must still leave
		// this one check passing. Eight of nine fail, not nine.
		v := checkT(t, binanceRequirement(), domain.Report{Venue: testBinance, ObservedAt: observedAt(t)})
		if len(v.Failures) != 8 {
			t.Fatalf("want 8 of 9 failing with the attestation pinned, got:\n%s", v)
		}
		for _, f := range v.Failures {
			if f.Setting == domain.SettingPortfolioMargin {
				t.Fatalf("the attested check must not fail on a silent report:\n%s", v)
			}
		}
	})

	t.Run("a venue observation cannot stand in for the attestation", func(t *testing.T) {
		// The single most likely shortcut: something reports Portfolio Margin
		// off on the wire and the gate believes it. No classic-account setting
		// establishes anything about the account TYPE (§73c-1's rejected
		// alternative), so the attestation is the only evidence admitted.
		req := binanceRequirement()
		req.PortfolioMargin = domain.AttestationUnstated
		rep := binanceReport(t)
		rep.Observations = append(rep.Observations, acct(domain.SettingPortfolioMargin, "false"))

		f := failureFor(t, checkT(t, req, rep), domain.SettingPortfolioMargin, "")
		if f.Reason != domain.ReasonUnattested {
			t.Fatalf("reason %s, want ReasonUnattested — a wire answer must not satisfy it", f.Reason)
		}
	})
}

// --- wiring refusals (errors, not verdicts) -----------------------------

func TestCheckRefusesOneVenuesAnswerAgainstAnothersRequirement(t *testing.T) {
	_, err := domain.Check(binanceRequirement(), bybitReport(t))
	if !errors.Is(err, domain.ErrVenueMismatch) {
		t.Fatalf("err = %v, want ErrVenueMismatch", err)
	}
}

func TestCheckRefusesAnUnknownDialect(t *testing.T) {
	req := binanceRequirement()
	req.Dialect = domain.Dialect("binance-portfolio-margin")
	_, err := domain.Check(req, binanceReport(t))
	if !errors.Is(err, domain.ErrDialectUnknown) {
		t.Fatalf("err = %v, want ErrDialectUnknown — an unknown dialect must never mean an empty rule table", err)
	}
}

func TestRequirementValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*domain.Requirement)
		want error
	}{
		{name: "no venue", edit: func(r *domain.Requirement) { r.Venue = "" }},
		{
			name: "empty roster",
			edit: func(r *domain.Requirement) { r.Symbols = nil },
			want: domain.ErrRosterEmpty,
		},
		{
			name: "a symbol twice",
			edit: func(r *domain.Requirement) {
				r.Symbols = append(r.Symbols, domain.SymbolPin{Symbol: firstSymbol, Leverage: 9})
			},
			want: domain.ErrRosterDuplicate,
		},
		{
			name: "a row with no symbol",
			edit: func(r *domain.Requirement) { r.Symbols[1].Symbol = "" },
			want: domain.ErrSymbolUnset,
		},
		{
			name: "an unpinned leverage",
			edit: func(r *domain.Requirement) { r.Symbols[1].Leverage = 0 },
			want: domain.ErrLeveragePinUnset,
		},
		{
			name: "a negative leverage",
			edit: func(r *domain.Requirement) { r.Symbols[1].Leverage = -5 },
			want: domain.ErrLeveragePinUnset,
		},
		{
			name: "an attestation value nobody declared",
			edit: func(r *domain.Requirement) { r.PortfolioMargin = domain.Attestation(9) },
			want: domain.ErrAttestationInvalid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := binanceRequirement()
			tc.edit(&req)
			err := req.Validate()
			if err == nil {
				t.Fatal("want a refusal")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// An attestation pinned where nothing reads it is refused, not ignored: the
// operator who recorded it believes a check is running on it (#179's
// spot-baseline-on-a-spotless-deployment precedent).
func TestRequirementRefusesAnAttestationNoRuleConsumes(t *testing.T) {
	req := bybitRequirement()
	req.PortfolioMargin = domain.AttestationOff
	if err := req.Validate(); !errors.Is(err, domain.ErrAttestationNotConsumed) {
		t.Fatalf("err = %v, want ErrAttestationNotConsumed", err)
	}

	// The UNSTATED zero is not a pin, so it must not trip the same refusal —
	// otherwise no Bybit deployment could ever validate.
	req.PortfolioMargin = domain.AttestationUnstated
	if err := req.Validate(); err != nil {
		t.Fatalf("an unattested Bybit requirement must validate: %v", err)
	}
}

func TestReportValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*domain.Report)
		want error
	}{
		{name: "no venue", edit: func(r *domain.Report) { r.Venue = "" }, want: domain.ErrReportUnusable},
		{
			name: "no observed-at",
			edit: func(r *domain.Report) { r.ObservedAt = clock.ServerTime{} },
			want: domain.ErrReportUnusable,
		},
		{
			name: "an account-wide setting naming a subject",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingMultiAssets, Scope: domain.ScopeAccount, Subject: "BTCUSDT",
					Value: "false", Presence: domain.PresenceStated,
				}}
			},
			want: domain.ErrObservationIncoherent,
		},
		{
			name: "a per-symbol setting naming no subject",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingLeverage, Scope: domain.ScopeSymbol,
					Value: "5", Presence: domain.PresenceStated,
				}}
			},
			want: domain.ErrObservationIncoherent,
		},
		{
			name: "a value beside a presence that says there was none",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingMultiAssets, Scope: domain.ScopeAccount,
					Value: "false", Presence: domain.PresenceAbsent,
				}}
			},
			want: domain.ErrObservationIncoherent,
		},
		{
			name: "a stated observation carrying no value",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingMultiAssets, Scope: domain.ScopeAccount,
					Presence: domain.PresenceStated,
				}}
			},
			want: domain.ErrObservationIncoherent,
		},
		{
			name: "an observation nobody stamped a presence on",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingMultiAssets, Scope: domain.ScopeAccount,
				}}
			},
			want: domain.ErrPresenceInvalid,
		},
		{
			name: "an observation naming no setting",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Scope: domain.ScopeAccount, Presence: domain.PresenceAbsent,
				}}
			},
			want: domain.ErrSettingInvalid,
		},
		{
			name: "an observation naming no scope",
			edit: func(r *domain.Report) {
				r.Observations = []domain.Observation{{
					Setting: domain.SettingMultiAssets, Presence: domain.PresenceAbsent,
				}}
			},
			want: domain.ErrScopeInvalid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rep := binanceReport(t)
			tc.edit(&rep)
			err := rep.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if _, cerr := domain.Check(binanceRequirement(), rep); cerr == nil {
				t.Fatal("Check must refuse a report that does not validate")
			}
		})
	}
}

// A report with NO observations is coherent — the rules turn it into named
// refusals — so Validate must not be the thing that rejects it.
func TestReportValidateAdmitsAnEmptyObservationList(t *testing.T) {
	rep := domain.Report{Venue: testBinance, ObservedAt: observedAt(t)}
	if err := rep.Validate(); err != nil {
		t.Fatalf("an empty report must validate so Check can name what is missing: %v", err)
	}
}

// --- the verdict --------------------------------------------------------

func TestVerdictRefusesToAnswerWhenNothingWasChecked(t *testing.T) {
	var v domain.Verdict
	ok, err := v.Satisfied()
	if !errors.Is(err, domain.ErrVerdictVacuous) {
		t.Fatalf("err = %v, want ErrVerdictVacuous", err)
	}
	if ok {
		t.Fatal("a verdict that evaluated nothing must never report satisfied")
	}
}

func TestVerdictSatisfiedIsFalseWithFailures(t *testing.T) {
	v := checkT(t, binanceRequirement(),
		replace(t, binanceReport(t), acct(domain.SettingMultiAssets, "true")))
	ok, err := v.Satisfied()
	if err != nil {
		t.Fatalf("Satisfied: %v", err)
	}
	if ok {
		t.Fatal("a verdict with failures must not report satisfied")
	}
}

// A golden render: substring assertions cannot see a field-order swap or a
// dropped clause, and this string is what an operator reads on a refused boot.
func TestVerdictRendersEveryFailure(t *testing.T) {
	req := binanceRequirement()
	req.PortfolioMargin = domain.AttestationUnstated
	rep := binanceReport(t)
	rep = replace(t, rep, sym(domain.SettingLeverage, secondSymbol, "20"))
	rep = replace(t, rep, unsaid(domain.SettingAutoAddMargin, domain.ScopeSymbol, firstSymbol, domain.PresenceAbsent))

	want := strings.Join([]string{
		"binance posture (binance-classic): 9 checks, 3 failed",
		"  SettingPortfolioMargin: ReasonUnattested (required an operator attestation that it is off)",
		"  SettingAutoAddMargin [BTCUSDT]: ReasonUnstated (venue said \"PresenceAbsent\", required false) via probe/unsaid",
		"  SettingLeverage [ETHUSDT]: ReasonMismatch (venue said \"20\", required 3) via probe/symbol",
	}, "\n")

	if got := checkT(t, req, rep).String(); got != want {
		t.Fatalf("verdict rendered:\n%s\n\nwant:\n%s", got, want)
	}
}

// --- vocabulary ---------------------------------------------------------

func TestEnumValidateRefusesTheSentinelAndTheOutOfRange(t *testing.T) {
	if err := domain.SettingUnspecified.Validate(); !errors.Is(err, domain.ErrSettingInvalid) {
		t.Fatalf("Setting sentinel: %v", err)
	}
	if err := domain.Setting(200).Validate(); !errors.Is(err, domain.ErrSettingInvalid) {
		t.Fatalf("Setting out of range: %v", err)
	}
	if err := domain.ScopeUnspecified.Validate(); !errors.Is(err, domain.ErrScopeInvalid) {
		t.Fatalf("Scope sentinel: %v", err)
	}
	if err := domain.Scope(200).Validate(); !errors.Is(err, domain.ErrScopeInvalid) {
		t.Fatalf("Scope out of range: %v", err)
	}
	if err := domain.PresenceUnspecified.Validate(); !errors.Is(err, domain.ErrPresenceInvalid) {
		t.Fatalf("Presence sentinel: %v", err)
	}
	if err := domain.ReasonUnspecified.Validate(); !errors.Is(err, domain.ErrFailureReasonInvalid) {
		t.Fatalf("FailureReason sentinel: %v", err)
	}
	// The attestation's zero is the ONE sentinel that is valid: "nobody has
	// attested" is the ordinary state of a fresh deployment and must reach the
	// rules as a named failure rather than a config error.
	if err := domain.AttestationUnstated.Validate(); err != nil {
		t.Fatalf("AttestationUnstated must validate: %v", err)
	}
	if err := domain.Attestation(200).Validate(); !errors.Is(err, domain.ErrAttestationInvalid) {
		t.Fatalf("Attestation out of range: %v", err)
	}
}

// Every declared member must render as its own name. A String() falling through
// to the numeric form is how a failure line stops naming its setting.
func TestEnumStringsAreDistinctAndNamed(t *testing.T) {
	seen := make(map[string]struct{})
	for s := domain.SettingUnspecified; s <= domain.SettingPortfolioMargin; s++ {
		got := s.String()
		if strings.HasPrefix(got, "Setting(") {
			t.Fatalf("Setting(%d) has no name", uint8(s))
		}
		if _, dup := seen[got]; dup {
			t.Fatalf("two settings render as %q", got)
		}
		seen[got] = struct{}{}
	}
	if got := domain.Setting(200).String(); got != "Setting(200)" {
		t.Fatalf("undeclared setting rendered %q", got)
	}
	for r := domain.ReasonUnspecified; r <= domain.ReasonUnattested; r++ {
		if strings.HasPrefix(r.String(), "FailureReason(") {
			t.Fatalf("FailureReason(%d) has no name", uint8(r))
		}
	}
	for p := domain.PresenceUnspecified; p <= domain.PresenceAbsent; p++ {
		if strings.HasPrefix(p.String(), "Presence(") {
			t.Fatalf("Presence(%d) has no name", uint8(p))
		}
	}
	for s := domain.ScopeUnspecified; s <= domain.ScopeAsset; s++ {
		if strings.HasPrefix(s.String(), "Scope(") {
			t.Fatalf("Scope(%d) has no name", uint8(s))
		}
	}
	for a := domain.AttestationUnstated; a <= domain.AttestationOn; a++ {
		if strings.HasPrefix(a.String(), "Attestation(") {
			t.Fatalf("Attestation(%d) has no name", uint8(a))
		}
	}
}

func TestFailureValidate(t *testing.T) {
	f := domain.Failure{Setting: domain.SettingLeverage, Scope: domain.ScopeSymbol, Subject: firstSymbol}
	if err := f.Validate(); !errors.Is(err, domain.ErrFailureReasonInvalid) {
		t.Fatalf("a failure with no reason must be refused: %v", err)
	}
	f.Reason = domain.ReasonMismatch
	if err := f.Validate(); err != nil {
		t.Fatalf("a complete failure must validate: %v", err)
	}
}

// Every failure Check produces must itself validate — otherwise a renderer
// downstream is handed a row it cannot describe.
func TestCheckProducesOnlyValidFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  domain.Requirement
		rep  domain.Report
	}{
		{"binance empty", binanceRequirement(), domain.Report{Venue: testBinance, ObservedAt: observedAt(t)}},
		{"bybit empty", bybitRequirement(), domain.Report{Venue: testBybit, ObservedAt: observedAt(t)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := checkT(t, tc.req, tc.rep)
			if len(v.Failures) == 0 {
				t.Fatal("fixture produced no failures — nothing is being checked")
			}
			for _, f := range v.Failures {
				if err := f.Validate(); err != nil {
					t.Fatalf("%v: %v", f, err)
				}
				if f.Required == "" {
					t.Fatalf("%v: states no requirement, so an operator cannot act on it", f)
				}
			}
		})
	}
}
