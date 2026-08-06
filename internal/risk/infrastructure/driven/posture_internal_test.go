package driven

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
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
}

func (f *fakeReader) AccountPosture(ctx context.Context, urgency ratelimit.Priority) (venue.AccountPostureReport, error) {
	f.calls++
	f.urgencies = append(f.urgencies, urgency)
	f.lastCtxNil = ctx == nil
	if f.err != nil {
		return venue.AccountPostureReport{}, f.err
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

	got, err := adapterT(t, r).AccountPosture(ports.PostureScope{Venue: adapterVenue})
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
		got, err := translateSetting(s)
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

	got, err := adapterT(t, r).AccountPosture(ports.PostureScope{Venue: adapterVenue})
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
	if _, err := adapterT(t, r).AccountPosture(ports.PostureScope{Venue: adapterVenue}); err != nil {
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
		AccountPosture(ports.PostureScope{Venue: adapterVenue})
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
	_, err := adapterT(t, r).AccountPosture(ports.PostureScope{Venue: ids.Venue("binance")})
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
		AccountPosture(ports.PostureScope{Venue: adapterVenue})
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
