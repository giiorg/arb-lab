package driven

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
	"github.com/giiorg/arb-lab/internal/risk/application/ports"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

const adapterVenue = ids.Venue("bybit")

var errVenueUnreachable = errors.New("venue unreachable")

// fakeReader records the urgency it was called with, because the rung this
// adapter rides is a claim (record §73c-2: both callers are Safety) and a
// constant that nothing reads is a claim nothing checks.
type fakeReader struct {
	calls      int
	urgencies  []ratelimit.Priority
	report     venue.AccountPostureReport
	err        error
	lastCtxNil bool

	// lastWant is the same kind of claim as urgencies, one field over: the
	// want-set is what makes a spot-only deployment stop issuing futures probes
	// (task #199), and a translation nothing reads is a translation nothing
	// checks.
	lastWant []venue.PostureSetting

	// lastSymbols is the field above's argument one more time, for the roster:
	// since record §97 the scope carries the configured symbols and Bybit's
	// posture read cannot observe a per-symbol setting without them, so a roster
	// that never reaches the reader is a roster nothing checks — and it fails as
	// ReasonUnobserved, which reads as the VENUE's silence rather than as ours.
	lastSymbols []string
}

func (f *fakeReader) AccountPosture(ctx context.Context, urgency ratelimit.Priority, want []venue.PostureSetting, symbols []string) (venue.AccountPostureReport, error) {
	f.calls++
	f.urgencies = append(f.urgencies, urgency)
	f.lastCtxNil = ctx == nil
	f.lastWant = want
	f.lastSymbols = symbols
	if f.err != nil {
		return venue.AccountPostureReport{}, f.err
	}
	return f.report, nil
}

// adapterScope is a scope naming the settings a test's fixture speaks about.
// Every scope must name at least one (ports.ErrPostureScopeSettingsUnstated),
// and which ones is irrelevant to the translation tests — the adapter passes
// them through and judges nothing.
func adapterScope(settings ...domain.Setting) ports.PostureScope {
	if len(settings) == 0 {
		settings = []domain.Setting{domain.SettingMultiAssets}
	}
	return ports.PostureScope{Venue: adapterVenue, Settings: settings}
}

func observedAt(t *testing.T) clock.ServerTime {
	t.Helper()
	st, err := clock.NewServerTime(time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC), 25*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return st
}

func venueReport(t *testing.T, obs ...venue.PostureObservation) venue.AccountPostureReport {
	t.Helper()
	return venue.AccountPostureReport{ObservedAt: observedAt(t), Observations: obs}
}

func adapterT(t *testing.T, r *fakeReader) *AccountPosture {
	t.Helper()
	a, err := NewAccountPosture(r, adapterVenue)
	if err != nil {
		t.Fatalf("NewAccountPosture: %v", err)
	}
	return a
}

// TestTheScopesWantSetReachesTheVenueTranslated is the other direction through
// this ACL, and the one #199 added (record §73p).
//
// The scope names DOMAIN settings and the client speaks the VENUE's vocabulary,
// so an adapter that forwarded nothing — or forwarded a list it built itself —
// would leave the read unnarrowed while every observation still translated back
// correctly on the way in. The order is asserted too: the want-set is the
// caller's statement of what it needs, and a reordering here would be a
// different read on a codec whose steps are ordered (record §73c-2's quotable
// cost is measured over that order).
func TestTheScopesWantSetReachesTheVenueTranslated(t *testing.T) {
	r := &fakeReader{report: venueReport(t)}

	scope := adapterScope(domain.SettingMarginMode, domain.SettingSpotBorrow, domain.SettingAccountKind)
	if _, err := adapterT(t, r).AccountPosture(scope); err != nil {
		t.Fatalf("AccountPosture: %v", err)
	}
	want := []venue.PostureSetting{
		venue.PostureMarginMode, venue.PostureSpotBorrow, venue.PostureAccountKind,
	}
	if len(r.lastWant) != len(want) {
		t.Fatalf("the client was asked for %d settings %v, want %d", len(r.lastWant), r.lastWant, len(want))
	}
	for i := range want {
		if r.lastWant[i] != want[i] {
			t.Errorf("want[%d] = %s, want %s", i, r.lastWant[i], want[i])
		}
	}
}

// And the setting with no venue read path never reaches the client at all: a
// scope naming it is a caller bug, refused before the request is issued rather
// than forwarded as some neighbouring member.
func TestAScopeNamingTheAttestedSettingIsRefusedBeforeTheRead(t *testing.T) {
	r := &fakeReader{report: venueReport(t)}

	scope := adapterScope(domain.SettingPortfolioMargin)
	if _, err := adapterT(t, r).AccountPosture(scope); !errors.Is(err, ErrAccountPostureUntranslatable) {
		t.Errorf("a scope naming the attested setting returned %v, want ErrAccountPostureUntranslatable", err)
	}
	if r.calls != 0 {
		t.Errorf("the venue was called %d times for a setting no read can answer", r.calls)
	}
}

func TestAdapterTranslatesAWholeReport(t *testing.T) {
	r := &fakeReader{report: venueReport(t,
		venue.PostureObservation{
			Setting: venue.PostureMarginMode, Scope: venue.PostureScopeAccount,
			Value: "ISOLATED_MARGIN", Presence: venue.PostureStated,
			Probe: venue.PostureProbeAccountInfo,
		},
		venue.PostureObservation{
			Setting: venue.PostureBorrowAmount, Scope: venue.PostureScopeAsset, Subject: "USDT",
			Value: "0", Presence: venue.PostureStated, Probe: venue.PostureProbeWallet,
		},
		venue.PostureObservation{
			Setting: venue.PostureLeverage, Scope: venue.PostureScopeSymbol, Subject: "BTCUSDT",
			Presence: venue.PostureAbsent, Probe: venue.PostureProbePositionList,
		},
	)}

	got, err := adapterT(t, r).AccountPosture(adapterScope())
	if err != nil {
		t.Fatalf("AccountPosture: %v", err)
	}
	if got.Venue != adapterVenue {
		t.Fatalf("venue %s, want %s", got.Venue, adapterVenue)
	}
	if got.ObservedAt != r.report.ObservedAt {
		t.Fatal("the venue clock instant did not survive the translation")
	}

	want := []domain.Observation{
		{Setting: domain.SettingMarginMode, Scope: domain.ScopeAccount, Value: "ISOLATED_MARGIN",
			Presence: domain.PresenceStated, Source: "PostureProbeAccountInfo"},
		{Setting: domain.SettingBorrowAmount, Scope: domain.ScopeAsset, Subject: "USDT", Value: "0",
			Presence: domain.PresenceStated, Source: "PostureProbeWallet"},
		{Setting: domain.SettingLeverage, Scope: domain.ScopeSymbol, Subject: "BTCUSDT",
			Presence: domain.PresenceAbsent, Source: "PostureProbePositionList"},
	}
	if len(got.Observations) != len(want) {
		t.Fatalf("translated %d observations, want %d", len(got.Observations), len(want))
	}
	for i := range want {
		if got.Observations[i] != want[i] {
			t.Fatalf("observation %d = %+v, want %+v", i, got.Observations[i], want[i])
		}
	}
}

// A TOTALITY check in both directions, driven by the venue package's own
// Validate rather than by a list written here: every setting that venue
// considers valid must translate, and everything it does not must refuse. A
// member added to the venue's vocabulary therefore fails HERE, which is the
// only place that could otherwise drop it silently.
func TestEveryVenueSettingTranslatesOrRefuses(t *testing.T) {
	translated := make(map[domain.Setting]venue.PostureSetting)
	valid := 0
	for i := 0; i <= 255; i++ {
		s := venue.PostureSetting(i)
		got, err := TranslateSetting(s)
		if s.Validate() == nil {
			valid++
			if err != nil {
				t.Fatalf("venue setting %s is valid but does not translate: %v", s, err)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("venue setting %s translated to an invalid domain setting: %v", s, err)
			}
			if prior, dup := translated[got]; dup {
				t.Fatalf("venue settings %s and %s both translate to %s", prior, s, got)
			}
			translated[got] = s
			continue
		}
		if !errors.Is(err, ErrAccountPostureUntranslatable) {
			t.Fatalf("venue setting %d is invalid but translated to %s (err %v)", i, got, err)
		}
	}
	// The venue surface names ten settings. The domain names eleven — the
	// eleventh is SettingPortfolioMargin, which no venue read path can produce
	// (record §73b F1) and which the operator attestation is the only evidence
	// for. If this count ever equals the domain's, a wire answer has become
	// able to satisfy the attested check.
	if valid != 10 {
		t.Fatalf("venue declares %d settings, want 10", valid)
	}
	if _, reachable := translated[domain.SettingPortfolioMargin]; reachable {
		t.Fatal("a venue observation can now produce SettingPortfolioMargin — the attestation is bypassable")
	}
}

// TestEverySettingRoundTripsThroughBothTranslations is the mirror of the test
// above, for the inverse the posture WANT-SET travels through (task #199,
// record §73p).
//
// The two switches are written out separately on purpose — an inverted map would
// agree with its source by construction — so this is what makes the pair
// checkable. Both directions are asserted from the same loop, because a
// one-directional test passes against a `venueSettingFor` that maps two domain
// settings onto one venue member: the read would then be issued for the wrong
// probe, and every field would still decode.
func TestEverySettingRoundTripsThroughBothTranslations(t *testing.T) {
	seen := make(map[venue.PostureSetting]domain.Setting)
	valid := 0
	for i := 0; i <= 255; i++ {
		d := domain.Setting(i)
		got, err := venueSettingFor(d)
		if d.Validate() != nil {
			if !errors.Is(err, ErrAccountPostureUntranslatable) {
				t.Fatalf("domain setting %d is invalid but translated to %s (err %v)", i, got, err)
			}
			continue
		}
		valid++
		// SettingPortfolioMargin is the one valid domain setting with NO venue
		// member, and it must refuse rather than land on a neighbour: mapping it
		// anywhere would let a wire answer satisfy §4.1's one attested check
		// (record §73b F1).
		if d == domain.SettingPortfolioMargin {
			if !errors.Is(err, ErrAccountPostureUntranslatable) {
				t.Fatalf("%s translated to venue member %s — the attestation is bypassable", d, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("domain setting %s does not translate to a venue member: %v", d, err)
		}
		if prior, dup := seen[got]; dup {
			t.Fatalf("domain settings %s and %s both translate to venue %s", prior, d, got)
		}
		seen[got] = d

		back, err := TranslateSetting(got)
		if err != nil {
			t.Fatalf("venue %s does not translate back: %v", got, err)
		}
		if back != d {
			t.Fatalf("%s -> %s -> %s; the round trip must be the identity", d, got, back)
		}
	}
	// Eleven domain settings, ten of which have a venue member. The counts are
	// literals for the reason the sibling test states: if they ever become
	// equal, the attested check has acquired a wire answer.
	if valid != 11 {
		t.Fatalf("domain declares %d settings, want 11", valid)
	}
	if len(seen) != 10 {
		t.Fatalf("%d domain settings reach a venue member, want 10", len(seen))
	}
}

func TestEveryVenueScopeTranslatesOrRefuses(t *testing.T) {
	valid := 0
	for i := 0; i <= 255; i++ {
		s := venue.PostureScope(i)
		got, err := translateScope(s)
		if s.Validate() == nil {
			valid++
			if err != nil || got.Validate() != nil {
				t.Fatalf("venue scope %s does not translate: %v", s, err)
			}
			continue
		}
		if !errors.Is(err, ErrAccountPostureUntranslatable) {
			t.Fatalf("venue scope %d is invalid but translated to %s", i, got)
		}
	}
	if valid != 3 {
		t.Fatalf("venue declares %d scopes, want 3", valid)
	}
}

// The presence mapping is the one with the most at stake: collapsing either
// silence into the other, or either into "stated", is how a field the venue
// never mentioned becomes a value the rules conclude from.
func TestEveryVenuePresenceTranslatesDistinctly(t *testing.T) {
	seen := make(map[domain.Presence]venue.PosturePresence)
	valid := 0
	for i := 0; i <= 255; i++ {
		p := venue.PosturePresence(i)
		got, err := translatePresence(p)
		if p.Validate() == nil {
			valid++
			if err != nil || got.Validate() != nil {
				t.Fatalf("venue presence %s does not translate: %v", p, err)
			}
			if prior, dup := seen[got]; dup {
				t.Fatalf("venue presences %s and %s both translate to %s — a silence was collapsed", prior, p, got)
			}
			seen[got] = p
			continue
		}
		if !errors.Is(err, ErrAccountPostureUntranslatable) {
			t.Fatalf("venue presence %d is invalid but translated to %s", i, got)
		}
	}
	if valid != 3 {
		t.Fatalf("venue declares %d presences, want 3", valid)
	}
	if seen[domain.PresenceStated] != venue.PostureStated {
		t.Fatalf("PresenceStated came from %s, want PostureStated", seen[domain.PresenceStated])
	}
	if seen[domain.PresenceStatedEmpty] != venue.PostureStatedEmpty {
		t.Fatalf("PresenceStatedEmpty came from %s", seen[domain.PresenceStatedEmpty])
	}
	if seen[domain.PresenceAbsent] != venue.PostureAbsent {
		t.Fatalf("PresenceAbsent came from %s", seen[domain.PresenceAbsent])
	}
}

// An untranslatable observation refuses the WHOLE report. Dropping it would
// leave the rules reporting an unobserved setting — a message pointing the
// operator at the venue for a defect on this side of the wire.
func TestAdapterRefusesTheWholeReportOnAnUntranslatableObservation(t *testing.T) {
	r := &fakeReader{report: venueReport(t,
		venue.PostureObservation{
			Setting: venue.PostureMarginMode, Scope: venue.PostureScopeAccount,
			Value: "ISOLATED_MARGIN", Presence: venue.PostureStated, Probe: venue.PostureProbeAccountInfo,
		},
		venue.PostureObservation{
			Setting: venue.PostureSetting(200), Scope: venue.PostureScopeAccount,
			Presence: venue.PostureAbsent, Probe: venue.PostureProbeAccountInfo,
		},
	)}

	got, err := adapterT(t, r).AccountPosture(adapterScope())
	if !errors.Is(err, ErrAccountPostureUntranslatable) {
		t.Fatalf("err = %v, want ErrAccountPostureUntranslatable", err)
	}
	if len(got.Observations) != 0 {
		t.Fatalf("a refused report carried %d observations", len(got.Observations))
	}
}

// Both callers are Safety by record §73c-2's argument, and the pre-trade one is
// the gate immediately before an authorization fsync.
func TestAdapterReadsAtSafetyPriority(t *testing.T) {
	r := &fakeReader{report: venueReport(t)}
	if _, err := adapterT(t, r).AccountPosture(adapterScope()); err != nil {
		t.Fatalf("AccountPosture: %v", err)
	}
	if len(r.urgencies) != 1 || r.urgencies[0] != ratelimit.PrioritySafety {
		t.Fatalf("urgencies = %v, want one PrioritySafety", r.urgencies)
	}
	if r.lastCtxNil {
		t.Fatal("the read was handed a nil context")
	}
}

// An empty venue report is a legitimate answer — the rules turn it into named
// refusals — so the adapter must pass it through rather than inventing an
// error or an observation.
func TestAdapterPassesAnEmptyReportThrough(t *testing.T) {
	got, err := adapterT(t, &fakeReader{report: venueReport(t)}).
		AccountPosture(adapterScope())
	if err != nil {
		t.Fatalf("AccountPosture: %v", err)
	}
	if len(got.Observations) != 0 {
		t.Fatalf("invented %d observations", len(got.Observations))
	}
	if got.Venue != adapterVenue || got.ObservedAt.Validate() != nil {
		t.Fatal("an empty report must still be a usable one")
	}
}

func TestAdapterRefusesAnotherVenuesScope(t *testing.T) {
	r := &fakeReader{report: venueReport(t)}
	_, err := adapterT(t, r).AccountPosture(ports.PostureScope{Venue: ids.Venue("binance"), Settings: []domain.Setting{domain.SettingMultiAssets}})
	if !errors.Is(err, ErrAccountPostureVenueMismatch) {
		t.Fatalf("err = %v, want ErrAccountPostureVenueMismatch", err)
	}
	if r.calls != 0 {
		t.Fatal("a mismatched scope must not reach the venue at all")
	}
}

func TestAdapterRefusesAnUnvalidatableScope(t *testing.T) {
	_, err := adapterT(t, &fakeReader{}).AccountPosture(ports.PostureScope{})
	if err == nil {
		t.Fatal("a scope naming no venue must be refused")
	}
}

func TestAdapterPropagatesTheReadFailureUnwrapped(t *testing.T) {
	_, err := adapterT(t, &fakeReader{err: errVenueUnreachable}).
		AccountPosture(adapterScope())
	if !errors.Is(err, errVenueUnreachable) {
		t.Fatalf("err = %v, want the venue's own error", err)
	}
}

func TestNewAccountPostureRefusesAMissingClientOrVenue(t *testing.T) {
	if _, err := NewAccountPosture(nil, adapterVenue); !errors.Is(err, ErrAccountPostureReaderMissing) {
		t.Fatalf("err = %v, want ErrAccountPostureReaderMissing", err)
	}
	if _, err := NewAccountPosture(&fakeReader{}, ids.Venue("")); err == nil {
		t.Fatal("an adapter with no venue must be refused")
	}
}

// TestBybitHedgeRowsReachTheDomainAsAnAmbiguity is record §73n F19's
// composition witness, and it lives here because record §73o took it away from
// where it used to be.
//
// # Why a test that crosses three layers, when all three have their own suites
//
// F19 was a defect that existed only BETWEEN layers: the codec refused a hedged
// position enumeration, so `ReasonAmbiguous` — a domain failure reason with a
// fixture whose comment calls it "the posture violation this rule exists to
// catch" — had no production path that could reach it. Both layers' suites were
// green, and encoded contradictory premises about the same wire shape.
//
// Its end-to-end fixture was a hedge-mode BINANCE boot, whose duplicate came
// from §4.1 reading `/fapi/v3/positionRisk`. Record §73o moved that read to
// `symbolConfig`, which holds one row per symbol whatever the position mode is —
// so Binance can no longer produce a duplicate per-symbol key at all, and the
// only remaining producer is Bybit's `position/list`, which genuinely is a
// position enumeration. Deleting the Binance fixture without replacing it would
// have restored exactly the blind spot F19 was found in.
//
// # What it composes, and what it deliberately does not
//
// The REAL Bybit codec decodes a REAL hedge body; the REAL adapter translates
// it; the REAL rules judge it. What is faked is only the transport. It stops
// short of a boot, because the outcome half — a read-and-wrong posture LATCHES
// rather than inviting a retry — is asserted end to end by
// TestBootOnAHedgeModeAccountLatchesAndStillNamesEveryOtherCheck, which the
// account-wide position-mode mismatch still drives.
func TestBybitHedgeRowsReachTheDomainAsAnAmbiguity(t *testing.T) {
	codec, err := venue.NewBybitOrderCodec(adapterVenue)
	if err != nil {
		t.Fatalf("NewBybitOrderCodec: %v", err)
	}

	// Hedge mode: one row per SIDE for one symbol, positionIdx 1 and 2.
	// Profiles §9 records the shape; the two rows AGREE on leverage and
	// auto-add-margin, so the position mode is the only key that doubles and
	// the ambiguity cannot be an artefact of a disagreeing value.
	const hedged = `{"result":{"list":[
		{"symbol":"BTCUSDT","positionIdx":1,"leverage":"5","autoAddMargin":0},
		{"symbol":"BTCUSDT","positionIdx":2,"leverage":"5","autoAddMargin":0}
	]}}`
	obs, err := codec.DecodePostureStep(venue.PostureProbePositionList,
		transport.Response{Body: []byte(hedged)})
	if err != nil {
		t.Fatalf("the codec refused a hedged position enumeration: %v — that refusal IS "+
			"F19, and it converts a posture READ AND FOUND WRONG into one that could "+
			"not be read", err)
	}

	report, err := adapterT(t, &fakeReader{report: venueReport(t, obs...)}).
		AccountPosture(adapterScope())
	if err != nil {
		t.Fatalf("AccountPosture: %v", err)
	}

	verdict, err := domain.Check(domain.Requirement{
		Venue:    adapterVenue,
		Dialect:  domain.DialectBybitUTA,
		Coverage: domain.RosterCoverageDerivatives,
		Symbols:  []domain.SymbolPin{{Symbol: "BTCUSDT", Leverage: 5}},
	}, report)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	var found *domain.Failure
	for i, f := range verdict.Failures {
		if f.Setting == domain.SettingPositionMode && f.Subject == "BTCUSDT" {
			found = &verdict.Failures[i]
		}
	}
	if found == nil {
		t.Fatalf("no per-symbol position-mode failure for a HEDGED account:\n%s", verdict)
	}
	if found.Reason != domain.ReasonAmbiguous {
		t.Fatalf("reason = %s, want ReasonAmbiguous — two answers on one key is not a "+
			"mismatch against one of them, and picking either would answer §4.1's "+
			"question from a row chosen here", found.Reason)
	}
	// BOTH answers are named. A failure reporting one of them has folded the
	// two rows into an answer, which is the only thing F19's original argument
	// ever actually forbade.
	for _, side := range []string{"1", "2"} {
		if !strings.Contains(found.Observed, side) {
			t.Errorf("observed = %q, want both hedge sides named", found.Observed)
		}
	}

	// The positive control: the same path with ONE row is not an ambiguity, or
	// this test would pass against a rule that called every position-mode
	// answer ambiguous.
	oneWay, err := codec.DecodePostureStep(venue.PostureProbePositionList,
		transport.Response{Body: []byte(
			`{"result":{"list":[{"symbol":"BTCUSDT","positionIdx":0,"leverage":"5","autoAddMargin":0}]}}`)})
	if err != nil {
		t.Fatalf("one-way position list: %v", err)
	}
	clean, err := adapterT(t, &fakeReader{report: venueReport(t, oneWay...)}).
		AccountPosture(adapterScope())
	if err != nil {
		t.Fatalf("AccountPosture (one-way): %v", err)
	}
	v2, err := domain.Check(domain.Requirement{
		Venue:    adapterVenue,
		Dialect:  domain.DialectBybitUTA,
		Coverage: domain.RosterCoverageDerivatives,
		Symbols:  []domain.SymbolPin{{Symbol: "BTCUSDT", Leverage: 5}},
	}, clean)
	if err != nil {
		t.Fatalf("Check (one-way): %v", err)
	}
	for _, f := range v2.Failures {
		if f.Setting == domain.SettingPositionMode && f.Reason == domain.ReasonAmbiguous {
			t.Errorf("a ONE-WAY account reported ReasonAmbiguous on its position mode:\n%s", v2)
		}
	}
}
