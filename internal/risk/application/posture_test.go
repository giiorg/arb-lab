package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/application"
	"github.com/giiorg/arb-lab/internal/risk/application/ports"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

const testVenue = ids.Venue("binance")

var errVenueUnreachable = errors.New("venue unreachable")

// fakeSource records what it was asked and how often, because two of this
// package's claims are about the CALL rather than the answer: that every Assert
// performs a fresh read (record §73c-2), and that the scope names the
// requirement's venue.
type fakeSource struct {
	calls  int
	scopes []ports.PostureScope
	report domain.Report
	err    error
}

func (f *fakeSource) AccountPosture(scope ports.PostureScope) (domain.Report, error) {
	f.calls++
	f.scopes = append(f.scopes, scope)
	if f.err != nil {
		return domain.Report{}, f.err
	}
	return f.report, nil
}

func observedAt(t *testing.T) clock.ServerTime {
	t.Helper()
	st, err := clock.NewServerTime(time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC), 25*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return st
}

func requirement() domain.Requirement {
	return domain.Requirement{
		Venue:   testVenue,
		Dialect: domain.DialectBinanceClassic,
		Symbols: []domain.SymbolPin{
			{Symbol: "BTCUSDT", Leverage: 5},
			{Symbol: "ETHUSDT", Leverage: 3},
		},
		PortfolioMargin: domain.AttestationOff,
	}
}

func cleanReport(t *testing.T) domain.Report {
	t.Helper()
	acct := func(s domain.Setting, v string) domain.Observation {
		return domain.Observation{Setting: s, Scope: domain.ScopeAccount, Value: v,
			Presence: domain.PresenceStated, Source: "probe"}
	}
	sym := func(s domain.Setting, subject, v string) domain.Observation {
		return domain.Observation{Setting: s, Scope: domain.ScopeSymbol, Subject: subject, Value: v,
			Presence: domain.PresenceStated, Source: "probe"}
	}
	return domain.Report{
		Venue:      testVenue,
		ObservedAt: observedAt(t),
		Observations: []domain.Observation{
			acct(domain.SettingMultiAssets, "false"),
			acct(domain.SettingPositionMode, "false"),
			sym(domain.SettingMarginMode, "BTCUSDT", "ISOLATED"),
			sym(domain.SettingLeverage, "BTCUSDT", "5"),
			sym(domain.SettingAutoAddMargin, "BTCUSDT", "false"),
			sym(domain.SettingMarginMode, "ETHUSDT", "ISOLATED"),
			sym(domain.SettingLeverage, "ETHUSDT", "3"),
			sym(domain.SettingAutoAddMargin, "ETHUSDT", "false"),
		},
	}
}

func guardT(t *testing.T, src ports.AccountPostureSource) *application.PostureGuard {
	t.Helper()
	g, err := application.NewPostureGuard(src, requirement())
	if err != nil {
		t.Fatalf("NewPostureGuard: %v", err)
	}
	return g
}

func TestAssertJudgesACleanPosture(t *testing.T) {
	src := &fakeSource{report: cleanReport(t)}
	v, err := guardT(t, src).Assert()
	if err != nil {
		t.Fatalf("Assert: %v", err)
	}
	ok, err := v.Satisfied()
	if err != nil || !ok {
		t.Fatalf("Satisfied() = %v, %v; want true, nil — verdict:\n%s", ok, err, v)
	}
	if v.Evaluated != 9 {
		t.Fatalf("evaluated %d, want 9", v.Evaluated)
	}
}

func TestAssertReportsTheFailuresWithoutErroring(t *testing.T) {
	rep := cleanReport(t)
	rep.Observations[0].Value = "true" // multi-assets on
	src := &fakeSource{report: rep}

	v, err := guardT(t, src).Assert()
	if err != nil {
		t.Fatalf("a posture VIOLATION is a verdict, not an error: %v", err)
	}
	if len(v.Failures) != 1 || v.Failures[0].Setting != domain.SettingMultiAssets {
		t.Fatalf("want the multi-assets failure, got:\n%s", v)
	}
}

// §73c-2's ruling is structural: every call performs the read. A guard that
// cached would answer the second Assert without asking the venue, and a
// position-mode flip between the two would be invisible — which is precisely
// the case §6.3 says the re-verify exists to catch.
func TestAssertReadsAfreshEveryTime(t *testing.T) {
	src := &fakeSource{report: cleanReport(t)}
	g := guardT(t, src)

	for i := 1; i <= 3; i++ {
		if _, err := g.Assert(); err != nil {
			t.Fatalf("Assert %d: %v", i, err)
		}
		if src.calls != i {
			t.Fatalf("after %d asserts the source was read %d times, want %d", i, src.calls, i)
		}
	}
}

func TestAssertAsksAboutTheRequirementsVenue(t *testing.T) {
	src := &fakeSource{report: cleanReport(t)}
	if _, err := guardT(t, src).Assert(); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(src.scopes) != 1 || src.scopes[0].Venue != testVenue {
		t.Fatalf("scopes = %v, want one naming %s", src.scopes, testVenue)
	}
}

// The interlock: an unanswered read must not decay into a clean posture. The
// error is propagated unwrapped, and the zero Verdict beside it REFUSES to
// answer rather than reporting satisfied.
func TestAssertPropagatesAReadFailureAndTheVerdictRefusesToAnswer(t *testing.T) {
	src := &fakeSource{err: errVenueUnreachable}
	v, err := guardT(t, src).Assert()
	if !errors.Is(err, errVenueUnreachable) {
		t.Fatalf("err = %v, want the venue's own error unwrapped", err)
	}
	if v.Evaluated != 0 || len(v.Failures) != 0 {
		t.Fatalf("a failed read must produce no verdict content, got:\n%s", v)
	}
	ok, serr := v.Satisfied()
	if ok {
		t.Fatal("a caller that ignored the error must NOT be told the posture is fine")
	}
	if !errors.Is(serr, domain.ErrVerdictVacuous) {
		t.Fatalf("Satisfied err = %v, want ErrVerdictVacuous", serr)
	}
}

// A source answering for the wrong exchange is a wiring defect in which every
// field still decodes and every rule still evaluates.
func TestAssertRefusesAnotherVenuesReport(t *testing.T) {
	rep := cleanReport(t)
	rep.Venue = ids.Venue("bybit")
	_, err := guardT(t, &fakeSource{report: rep}).Assert()
	if !errors.Is(err, domain.ErrVenueMismatch) {
		t.Fatalf("err = %v, want ErrVenueMismatch", err)
	}
}

func TestNewPostureGuardRefusesAMissingSource(t *testing.T) {
	_, err := application.NewPostureGuard(nil, requirement())
	if !errors.Is(err, application.ErrPostureSourceMissing) {
		t.Fatalf("err = %v, want ErrPostureSourceMissing", err)
	}
}

// A misconfigured requirement stops the process from booting rather than
// surfacing as a refusal on the first trade.
func TestNewPostureGuardRefusesAnInvalidRequirement(t *testing.T) {
	req := requirement()
	req.Symbols = nil
	_, err := application.NewPostureGuard(&fakeSource{}, req)
	if !errors.Is(err, domain.ErrRosterEmpty) {
		t.Fatalf("err = %v, want ErrRosterEmpty", err)
	}
}

// A guard nobody constructed must REFUSE, not panic.
//
// The zero value's source is nil, so the scope check is the only thing between
// an unconstructed guard and a nil dereference — inside §7.5's boot, where a
// crash stops the gate whose whole job is refusing to trade from reporting at
// all. The check was there and nothing exercised it: the mutation campaign
// reported it SURVIVED, which is what a defensive branch with no reachable
// caller in the suite looks like.
func TestAZeroValueGuardRefusesRatherThanCrashing(t *testing.T) {
	var g application.PostureGuard
	v, err := g.Assert()
	if err == nil {
		t.Fatal("an unconstructed guard must refuse")
	}
	if ok, _ := v.Satisfied(); ok {
		t.Fatal("an unconstructed guard reported a satisfied posture")
	}
}

// An unusable report is an error, not an empty verdict: the two get different
// operator responses.
func TestAssertRefusesAnUnusableReport(t *testing.T) {
	rep := cleanReport(t)
	rep.ObservedAt = clock.ServerTime{}
	_, err := guardT(t, &fakeSource{report: rep}).Assert()
	if !errors.Is(err, domain.ErrReportUnusable) {
		t.Fatalf("err = %v, want ErrReportUnusable", err)
	}
}
