package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/oms/application"
	omsdomain "github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/observability"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
	riskdomain "github.com/giiorg/arb-lab/internal/risk/domain"
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation"
)

// This file checks the composition root (slice G2, record §58).
//
// # The collapse checklist, run before the tests were written
//
//  1. The valid fixture carries TWO instruments — a spot leg and a perp leg —
//     and TWO endpoints, because a one-instrument document makes "every
//     instrument agrees with the venue" and "the first instrument agrees" the
//     same claim, and makes the scope-coverage rule vacuous (it only bites on a
//     derivative). Every refusal test then mutates that fixture in ONE field.
//  2. The two instruments have the SAME symbol, "BTCUSDT", differing only in
//     product. That is the cash-and-carry pair as it really is, and it is the
//     shape a duplicate check keyed on the symbol alone would wrongly refuse —
//     the fixture contains the distinction the key has to make.
//  3. The rate-limit profile carries a ScopeVenue limit AND two ScopeFamily
//     limits, so "the profile was built" and "the venue envelope survived the
//     translation" are different answers. A family-only profile would pass a
//     parseScope that ignored its input.
//  4. classify is tested on a name from each of the FOUR classes plus an
//     unrecognised one, because the default is the interesting branch and a
//     table with only the admitted names never reaches it.
//  5. The end-to-end boot is run on a SPOT-ONLY deployment, which is the one
//     shape whose reconcile makes no network call at all: no scope is required
//     for spot, and an empty journal has no dispatched order to join. That is
//     what makes it a test of the SEQUENCE rather than of a venue fake.

func envRef(t *testing.T, name string) config.SecretRef {
	t.Helper()
	ref, err := config.ParseSecretRef("env:" + name)
	if err != nil {
		t.Fatalf("ParseSecretRef: %v", err)
	}
	return ref
}

func dur(t *testing.T, s string) config.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatalf("ParseDuration: %v", err)
	}
	return config.NewDuration(d)
}

const (
	testKeyVar    = "ARBLAB_TEST_API_KEY"
	testSecretVar = "ARBLAB_TEST_API_SECRET"

	// testPinDigest is a well-formed 64-hex-char digest for the baseline pin
	// cases — a shape fixture, deliberately not the digest of anything.
	testPinDigest = "6c62272e07bb014262b821756295c58d6c62272e07bb014262b821756295c58d"
)

// validConfig is the fixture every refusal test mutates one field of.
func validConfig(t *testing.T, journal string) Config {
	t.Helper()
	return Config{
		JournalPath: journal,
		Account:     "sub-1",
		Venue: VenueConfig{
			Venue:        ids.Venue("binance"),
			Dialect:      DialectBinance,
			APIKeyRef:    envRef(t, testKeyVar),
			APISecretRef: envRef(t, testSecretVar),
			SpendLockDir: filepath.Dir(journal),
			Endpoints: []EndpointConfig{
				{Product: ids.ProductSpot, Scheme: "https", Host: "api.binance.com"},
				{Product: ids.ProductLinearPerp, Scheme: "https", Host: "fapi.binance.com"},
			},
			Timeout:    dur(t, "5s"),
			ReadLimit:  1 << 20,
			RecvWindow: dur(t, "5s"),
			// The family limit names follow the FOLD-DERIVED convention
			// (BinanceLimitReader turns X-Mbx-Used-Weight-1m into "weight_1m",
			// addressed at the call's family). Since #186 the convention is
			// enforced: NewSignedClient refuses a family weight/orders budget
			// under any other name, because the venue's readings could never
			// reach it through the observe seam.
			Limits: []LimitConfig{
				{Name: "ip_requests", Scope: "venue", Dimension: "requests", Interval: dur(t, "5s"),
					OpeningCeiling: 300, RoutineCeiling: 400, Capacity: 500},
				{Name: "weight_1m", Scope: "family", Product: ids.ProductSpot, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 4200, RoutineCeiling: 5400, Capacity: 6000},
				{Name: "orders_1m", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "orders",
					Interval: dur(t, "1m"), OpeningCeiling: 840, RoutineCeiling: 1080, Capacity: 1200},
				{Name: "weight_1m", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
				{Name: "orders_10s", Scope: "family", Product: ids.ProductSpot, Dimension: "orders",
					Interval: dur(t, "10s"), OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100},
			},
			ThrottlePause:    dur(t, "2m"),
			BanPause:         dur(t, "10m"),
			ClockOffset:      dur(t, "0s"),
			ClockUncertainty: dur(t, "250ms"),
		},
		Instruments: []InstrumentConfig{
			{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"},
			{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
				Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Leverage: 5},
		},
		PositionScopes: []ScopeConfig{
			{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"},
		},
		Resolution: ResolutionConfig{
			MaxLanding:  dur(t, "30s"),
			SweepMargin: dur(t, "5s"),
			AssumedReach: []ReachConfig{
				{Venue: "binance", Product: ids.ProductSpot, Reach: dur(t, "72h")},
			},
		},
		// Required-when-spot (record §70g): the fixture declares a spot
		// instrument, so the settle bound must be here or the fixture fails
		// its own positive control.
		SpotSettle:      dur(t, "2s"),
		WatchdogSilence: dur(t, "30s"),
		StopBudget:      dur(t, "2s"),
	}
}

// TestValidConfigValidates is the positive control every refusal below needs. A
// Validate that objected to everything would otherwise read as a perfect score.
func TestValidConfigValidates(t *testing.T) {
	t.Parallel()
	if err := validConfig(t, "/var/lib/arblab/journal.db").Validate(); err != nil {
		t.Fatalf("the fixture must validate, or every refusal below proves nothing: %v", err)
	}
}

func TestConfigRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want error
		mut  func(*Config)
	}{
		{"journal path unset", ErrConfigFieldUnset, func(c *Config) { c.JournalPath = "" }},
		{"account unset", ErrConfigFieldUnset, func(c *Config) { c.Account = "" }},
		{"spend lock directory unset", ErrConfigFieldUnset, func(c *Config) { c.Venue.SpendLockDir = "" }},
		{"dialect unknown", ErrDialectUnknown, func(c *Config) { c.Venue.Dialect = "kraken" }},
		{"no endpoints", ErrConfigFieldUnset, func(c *Config) { c.Venue.Endpoints = nil }},
		{"endpoint scheme", ErrConfigFieldInvalid, func(c *Config) { c.Venue.Endpoints[0].Scheme = "ftp" }},
		{"endpoint repeated", ErrConfigDuplicate, func(c *Config) {
			c.Venue.Endpoints = append(c.Venue.Endpoints, c.Venue.Endpoints[0])
		}},
		{"instrument on another venue", ErrVenueMismatch, func(c *Config) {
			c.Instruments[1].Venue = "bybit"
		}},
		{"scope on another venue", ErrVenueMismatch, func(c *Config) {
			c.PositionScopes[0].Venue = "bybit"
		}},
		{"instrument family unrouted", ErrFamilyUnrouted, func(c *Config) {
			// Drop the perp endpoint and keep the perp instrument: on Binance the
			// families are separate hosts, so this is not a missing base URL but a
			// query that cannot be issued.
			//
			// The perp LIMITS go with it, and that is the recorded
			// two-claims-one-checker trap rather than tidiness: a profile naming a
			// family the endpoints do not declare is refused by ratelimit.Profile,
			// which runs FIRST — so the version of this case that dropped only the
			// endpoint passed while never reaching the rule it is named after. A
			// checker that halts on the first violation never demonstrates the
			// deeper claim.
			c.Venue.Endpoints = c.Venue.Endpoints[:1]
			c.Venue.Limits = []LimitConfig{c.Venue.Limits[0], c.Venue.Limits[1], c.Venue.Limits[4]}
		}},
		{"instrument repeated", ErrConfigDuplicate, func(c *Config) {
			c.Instruments = append(c.Instruments, c.Instruments[0])
		}},
		{"scope repeated", ErrConfigDuplicate, func(c *Config) {
			c.PositionScopes = append(c.PositionScopes, c.PositionScopes[0])
		}},
		{"derivative with no scope", ErrScopeNotCovered, func(c *Config) { c.PositionScopes = nil }},
		// The two bounds are checked separately and each case moves the OTHER one
		// out of the way, which is the two-claims-one-checker trap again: the
		// fixture ships Timeout and RecvWindow at the same value, so a MaxLanding
		// equal to both satisfies either check and a mutation weakening one of them
		// SURVIVED against the version that only set MaxLanding. Equality is the
		// boundary under test in both — "not above" is the requirement, so `<` and
		// `<=` must be distinguishable.
		{"max_landing equal to the timeout", ErrLandingBoundTooShort, func(c *Config) {
			c.Venue.RecvWindow = dur(t, "1s")
			c.Resolution.MaxLanding = c.Venue.Timeout
		}},
		{"max_landing equal to the recv window", ErrLandingBoundTooShort, func(c *Config) {
			c.Venue.Timeout = dur(t, "1s")
			c.Resolution.MaxLanding = c.Venue.RecvWindow
		}},
		{"assumed reach repeated", ErrConfigDuplicate, func(c *Config) {
			c.Resolution.AssumedReach = append(c.Resolution.AssumedReach, c.Resolution.AssumedReach[0])
		}},
		{"limit scope unrecognised", ErrConfigFieldInvalid, func(c *Config) { c.Venue.Limits[0].Scope = "global" }},
		{"endpoint-scoped limit naming a product is refused by the kernel", ratelimit.ErrLimitFamily, func(c *Config) {
			c.Venue.Limits[0].Scope = "endpoint"
			c.Venue.Limits[0].Product = ids.ProductSpot
		}},
		{"limit dimension unrecognised", ErrConfigFieldInvalid, func(c *Config) {
			c.Venue.Limits[1].Dimension = "bytes"
		}},
		{"uncounted dimension unrecognised", ErrConfigFieldInvalid, func(c *Config) {
			c.Venue.Uncounted = []UncountedConfig{
				{Product: ids.ProductSpot, Dimension: "bytes", Reason: "typo under test"}}
		}},
		// The kernel's own §7.6 rules must be REACHABLE from the document — a
		// passthrough that dropped the block would validate everything here.
		{"uncounted waiver for requests refused by the kernel", ratelimit.ErrProfileUncountedInvalid,
			func(c *Config) {
				c.Venue.Uncounted = []UncountedConfig{
					{Product: ids.ProductSpot, Dimension: "requests", Reason: "the envelope is not waivable"}}
			}},
		{"a family with no order budget and no waiver refused by the kernel",
			ratelimit.ErrProfileDimensionUncounted, func(c *Config) {
				c.Venue.Limits = c.Venue.Limits[:4] // drop spot's orders budget
			}},
		{"watchdog silence unset", ErrConfigFieldUnset, func(c *Config) { c.WatchdogSilence = config.Duration{} }},
		{"stop budget unset", ErrConfigFieldUnset, func(c *Config) { c.StopBudget = config.Duration{} }},
		// The spot pair rules (record §70g): the settle bound is required
		// exactly when a spot instrument exists, and refused when none does —
		// a bound nothing waits is a claim nobody checks. The fixture declares
		// a spot leg, so each spotless case drops it.
		{"spot_settle unset with a spot instrument", ErrConfigFieldUnset, func(c *Config) {
			c.SpotSettle = config.Duration{}
		}},
		{"spot_settle on a spotless deployment", ErrConfigFieldInvalid, func(c *Config) {
			c.Instruments = c.Instruments[1:] // the perp leg alone; the settle bound stays behind
		}},
		// The pin here is WELL-FORMED on purpose: the shape checks run first
		// and answer the same sentinel, so a malformed pin would let this row
		// pass while never reaching the spotless rule it is named after — the
		// two-claims-one-checker trap, again.
		{"spot_baseline on a spotless deployment", ErrConfigFieldInvalid, func(c *Config) {
			c.Instruments = c.Instruments[1:]
			c.SpotSettle = config.Duration{}
			c.SpotBaseline = &BaselinePinConfig{Path: "/var/lib/arblab/baseline.json", SHA256: testPinDigest}
		}},
		{"spot_baseline path unset", ErrConfigFieldUnset, func(c *Config) {
			c.SpotBaseline = &BaselinePinConfig{SHA256: testPinDigest}
		}},
		{"spot_baseline sha256 unset", ErrConfigFieldUnset, func(c *Config) {
			c.SpotBaseline = &BaselinePinConfig{Path: "/var/lib/arblab/baseline.json"}
		}},
		{"spot_baseline sha256 not hex", ErrConfigFieldInvalid, func(c *Config) {
			c.SpotBaseline = &BaselinePinConfig{Path: "/var/lib/arblab/baseline.json",
				SHA256: strings.Repeat("zx", 32)}
		}},
		{"spot_baseline sha256 wrong length", ErrConfigFieldInvalid, func(c *Config) {
			c.SpotBaseline = &BaselinePinConfig{Path: "/var/lib/arblab/baseline.json",
				SHA256: testPinDigest[:32]}
		}},

		// #198 item 6 — the leverage pin. Zero is the value the field takes
		// when the line is simply absent from the file, which is the case that
		// matters: an unpinned leverage reads as "any leverage is acceptable".
		{"leverage pin missing on a derivative", ErrConfigFieldUnset, func(c *Config) {
			c.Instruments[1].Leverage = 0
		}},
		{"leverage pin negative on a derivative", ErrConfigFieldUnset, func(c *Config) {
			c.Instruments[1].Leverage = -5
		}},
		// The mirror: pinned where no §4.1 rule is scoped to read it.
		{"leverage pin on a spot instrument", ErrConfigFieldInvalid, func(c *Config) {
			c.Instruments[0].Leverage = 5
		}},

		// #198 item 5 — the Portfolio-Margin attestation pin. Same four shape
		// rows as spot_baseline's, because it is the same two-act pin.
		{"attestation path unset", ErrConfigFieldUnset, func(c *Config) {
			c.PortfolioMarginAttestation = &AttestationPinConfig{SHA256: testPinDigest}
		}},
		{"attestation sha256 unset", ErrConfigFieldUnset, func(c *Config) {
			c.PortfolioMarginAttestation = &AttestationPinConfig{Path: "/var/lib/arblab/pm.json"}
		}},
		{"attestation sha256 not hex", ErrConfigFieldInvalid, func(c *Config) {
			c.PortfolioMarginAttestation = &AttestationPinConfig{Path: "/var/lib/arblab/pm.json",
				SHA256: strings.Repeat("zx", 32)}
		}},
		{"attestation sha256 wrong length", ErrConfigFieldInvalid, func(c *Config) {
			c.PortfolioMarginAttestation = &AttestationPinConfig{Path: "/var/lib/arblab/pm.json",
				SHA256: testPinDigest[:32]}
		}},
		// The consumption rule, and the pin is WELL-FORMED for spot_baseline's
		// reason: a malformed one would answer the same sentinel one check
		// earlier and this row would never reach the rule it is named after.
		// Bybit UTA's table has no Portfolio-Margin row, so nothing reads it.
		{"attestation pinned on an account model that reads none", ErrConfigFieldInvalid, func(c *Config) {
			c.Venue.Dialect = DialectBybit
			c.PortfolioMarginAttestation = &AttestationPinConfig{Path: "/var/lib/arblab/pm.json",
				SHA256: testPinDigest}
		}},
	}
	for _, c := range cases {
		cfg := validConfig(t, "/var/lib/arblab/journal.db")
		c.mut(&cfg)
		err := cfg.Validate()
		if !errors.Is(err, c.want) {
			t.Errorf("%s: Validate() = %v, want %v", c.name, err, c.want)
		}
	}
}

// TestConfigAcceptsTheSpotPairsPositiveControls: the two acceptance shapes the
// spot refusal rows above lean on, stated rather than implied. A well-formed
// pin on a spot deployment validates — so the pin-shape rows fail on the pin,
// not on some other objection — and a spotless deployment carrying NEITHER
// spot field validates — so the refused-when-spotless rows fail on the
// leftover field, not on the spotless shape itself.
func TestConfigAcceptsTheSpotPairsPositiveControls(t *testing.T) {
	t.Parallel()

	withPin := validConfig(t, "/var/lib/arblab/journal.db")
	withPin.SpotBaseline = &BaselinePinConfig{Path: "/var/lib/arblab/baseline.json", SHA256: testPinDigest}
	if err := withPin.Validate(); err != nil {
		t.Errorf("a spot deployment with a well-formed pin must validate: %v", err)
	}

	spotless := validConfig(t, "/var/lib/arblab/journal.db")
	spotless.Instruments = spotless.Instruments[1:]
	spotless.SpotSettle = config.Duration{}
	if err := spotless.Validate(); err != nil {
		t.Errorf("a spotless deployment with neither spot field must validate: %v", err)
	}
}

// TestPostureRequirementDerivesTheRosterFromTheDerivatives is #198 item 6's
// central claim, and the fixture is built against the collapse checklist rather
// than against the code:
//
//   - TWO derivative instruments, with DIFFERENT pins (5 and 3). One would make
//     "every derivative" and "the first derivative" the same roster, and equal
//     pins would make "this symbol's pin" and "the first symbol's pin" the same
//     value ([[fixture-count-of-one-collapses]], and §73g item 1 — the same
//     collapse risk/domain's own M14 was aimed at).
//   - The two derivatives carry DIFFERENT symbols, and the spot row carries the
//     symbol of the FIRST of them. A derivation that filtered on something
//     other than the product would pick the spot row up and nothing about the
//     symbol list would look wrong.
//   - The pins are asserted per symbol, not as a set: a roster built by pairing
//     the right symbols with the wrong pins passes any length or membership
//     check.
func TestPostureRequirementDerivesTheRosterFromTheDerivatives(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t, "/var/lib/arblab/journal.db")
	cfg.Instruments = []InstrumentConfig{
		{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"},
		{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
			Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Leverage: 5},
		{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
			Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Leverage: 3},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	req, err := cfg.PostureRequirement(riskdomain.AttestationOff)
	if err != nil {
		t.Fatalf("PostureRequirement: %v", err)
	}
	if req.Venue != cfg.Venue.Venue {
		t.Errorf("requirement names %s, config names %s", req.Venue, cfg.Venue.Venue)
	}
	if req.Dialect != riskdomain.DialectBinanceClassic {
		t.Errorf("dialect = %s, want %s", req.Dialect, riskdomain.DialectBinanceClassic)
	}
	if req.Coverage != riskdomain.RosterCoverageDerivatives {
		t.Errorf("coverage = %s, want %s", req.Coverage, riskdomain.RosterCoverageDerivatives)
	}
	if req.PortfolioMargin != riskdomain.AttestationOff {
		t.Errorf("the attestation must arrive as handed, got %s", req.PortfolioMargin)
	}

	want := map[string]int{"BTCUSDT": 5, "ETHUSDT": 3}
	if len(req.Symbols) != len(want) {
		t.Fatalf("roster = %+v, want %d rows (the spot leg must not be on it)", req.Symbols, len(want))
	}
	for _, p := range req.Symbols {
		pin, listed := want[p.Symbol]
		if !listed {
			t.Errorf("roster carries %s, which is not a configured derivative", p.Symbol)
			continue
		}
		if p.Leverage != pin {
			t.Errorf("%s pinned at %d, config says %d", p.Symbol, p.Leverage, pin)
		}
	}
}

// TestPostureRequirementOnASpotOnlyDeploymentStatesTheEmptyRoster is the shape
// the first cut of validatePosture refused outright (record §73k F11). §5.2's
// venue block is singular, so a cross-venue carry's spot half is exactly this
// document, and refusing it would refuse a deployment this project is building
// toward.
//
// The claim is not merely "it validates": it is that the empty roster arrives
// at risk/domain as a STATED fact. A requirement that validated with the
// unspecified zero would be one where "no derivatives configured" and "the
// roster was forgotten" are the same value again.
func TestPostureRequirementOnASpotOnlyDeploymentStatesTheEmptyRoster(t *testing.T) {
	t.Parallel()

	cfg := spotOnly(t, "/var/lib/arblab/journal.db")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a spot-only deployment must validate: %v", err)
	}

	req, err := cfg.PostureRequirement(riskdomain.AttestationOff)
	if err != nil {
		t.Fatalf("PostureRequirement: %v", err)
	}
	if req.Coverage != riskdomain.RosterCoverageNoDerivatives {
		t.Errorf("coverage = %s, want %s", req.Coverage, riskdomain.RosterCoverageNoDerivatives)
	}
	if len(req.Symbols) != 0 {
		t.Errorf("roster = %+v, want empty", req.Symbols)
	}
}

// TestPostureDialectIsStatedAndNotDefaulted covers the mapping both ways plus
// its refusal. The two rows must DISAGREE — a mapping that returned the Binance
// model for everything would satisfy a one-row table.
func TestPostureDialectIsStatedAndNotDefaulted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   Dialect
		want riskdomain.Dialect
	}{
		{DialectBinance, riskdomain.DialectBinanceClassic},
		{DialectBybit, riskdomain.DialectBybitUTA},
	} {
		got, err := tc.in.PostureDialect()
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s maps to %s, want %s", tc.in, got, tc.want)
		}
	}
	if _, err := Dialect("kraken").PostureDialect(); !errors.Is(err, ErrDialectUnknown) {
		t.Errorf("an unknown dialect must have no account model, got %v", err)
	}
}

func TestParseScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want ratelimit.Scope
		err  bool
	}{
		{"venue", ratelimit.ScopeVenue, false},
		{"family", ratelimit.ScopeFamily, false},
		{"endpoint", ratelimit.ScopeEndpoint, false},
		{"global", ratelimit.ScopeUnspecified, true},
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			t.Parallel()
			got, err := parseScope(tc.text)
			if got != tc.want || (err != nil) != tc.err {
				t.Errorf("parseScope(%q) = (%s, %v), want (%s, error=%v)",
					tc.text, got, err, tc.want, tc.err)
			}
		})
	}
}

func TestProfileCarriesEndpointScopeWithoutAProduct(t *testing.T) {
	t.Parallel()
	cfg := validConfig(t, "/j.db")
	cfg.Venue.Limits = append(cfg.Venue.Limits, LimitConfig{
		Name: "endpoint_read", Scope: "endpoint", Dimension: "requests",
		Interval: dur(t, "1s"), OpeningCeiling: 7, RoutineCeiling: 9, Capacity: 10,
	})
	p, err := cfg.Venue.profile()
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	for _, l := range p.Limits {
		if l.Name == "endpoint_read" {
			if l.Scope != ratelimit.ScopeEndpoint || l.Family != "" {
				t.Errorf("endpoint_read = scope %s family %q, want ScopeEndpoint and no family", l.Scope, l.Family)
			}
			return
		}
	}
	t.Fatal("endpoint_read missing from profile")
}

// TestScopeCoverageIgnoresSpot is the other half of the coverage rule, and it is
// the half a naive "every instrument needs a scope" would get wrong.
//
// The spot leg is deliberately in the valid fixture with no spot scope, so the
// positive control above already depends on this — but only implicitly. Stated
// here so a rule tightened to cover every instrument fails a test that says why.
func TestScopeCoverageIgnoresSpot(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t, "/var/lib/arblab/journal.db")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("spot with no scope must validate — no venue publishes a position surface for it: %v", err)
	}
	// And a spot SCOPE is refused by the port's own validator, so the two halves
	// cannot both be satisfied by adding one.
	cfg.PositionScopes = append(cfg.PositionScopes,
		ScopeConfig{Venue: "binance", Product: ids.ProductSpot, Settle: "USDT"})
	if err := cfg.Validate(); err == nil {
		t.Error("a spot position scope must be refused: there is no surface to enumerate")
	}
}

// TestProfileCarriesBothScopes is checklist item 3: the §7.6 venue envelope must
// survive the string→enum translation, not just the family budgets.
func TestProfileCarriesBothScopes(t *testing.T) {
	t.Parallel()

	p, err := validConfig(t, "/j.db").Venue.profile()
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	var venueScoped, familyScoped, orderCounted int
	for _, l := range p.Limits {
		switch l.Scope.String() {
		case "ScopeVenue":
			venueScoped++
		case "ScopeFamily":
			familyScoped++
		}
		if l.Dimension.String() == "DimensionOrders" {
			orderCounted++
		}
	}
	if venueScoped != 1 || familyScoped != 4 {
		t.Errorf("scopes = %d venue / %d family, want 1/4 — §7.6's shared IP-ban envelope is the venue-scoped one",
			venueScoped, familyScoped)
	}
	// §7.6's worked failure: a profile budgeting no order-count dimension charges
	// literally nothing for a Binance futures order, which is documented at zero
	// weight. Two families, two order budgets.
	if orderCounted != 2 {
		t.Errorf("order-count budgets = %d, want 2 — one per family", orderCounted)
	}
	if len(p.Families) != 2 {
		t.Errorf("families = %v, want the two the endpoints declare", p.Families)
	}
}

// TestUncountedWaiverPassesThrough: the operator's written waiver crosses the
// string→enum seam intact and the kernel accepts it in place of the budget it
// waives — the positive control for the two kernel-reachability refusals in
// the table above.
func TestUncountedWaiverPassesThrough(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t, "/var/lib/arblab/journal.db")
	cfg.Venue.Limits = cfg.Venue.Limits[:4] // drop spot's orders budget
	cfg.Venue.Uncounted = []UncountedConfig{{
		Product: ids.ProductSpot, Dimension: "orders",
		Reason: "fixture: the spot order budget is deliberately waived here",
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a config with a written waiver must validate: %v", err)
	}
	p, err := cfg.Venue.profile()
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if len(p.Uncounted) != 1 ||
		p.Uncounted[0].Family != ids.ProductSpot ||
		p.Uncounted[0].Dimension != ratelimit.DimensionOrders ||
		p.Uncounted[0].Reason == "" {
		t.Errorf("Uncounted = %+v, want the document's waiver, translated", p.Uncounted)
	}
}

// TestEachDialectSelectsAConsistentSet is what Dialect exists for, asserted on
// the pieces rather than on the name.
//
// The envelope reader is the one that matters and the one a wrong wiring would
// hide: on Bybit a refusal is stated in the BODY of an HTTP 200, so a Bybit
// client holding BinanceEnvelope reads every rejection as an acceptance — an
// order this system believes was refused and the venue believes it holds. The
// assertion is on the concrete types, because that is the only place the
// mismatch is visible; every one of the four satisfies its interface either way.
func TestEachDialectSelectsAConsistentSet(t *testing.T) {
	t.Parallel()

	cred, err := transport.NewCredential("k", "s")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	binance, err := collaboratorsFor(DialectBinance, "binance", cred)
	if err != nil {
		t.Fatalf("collaboratorsFor(binance): %v", err)
	}
	if _, ok := binance.envelope.(transport.BinanceEnvelope); !ok {
		t.Errorf("binance envelope = %T, want BinanceEnvelope", binance.envelope)
	}
	if _, ok := binance.limits.(transport.BinanceLimitReader); !ok {
		t.Errorf("binance limits = %T, want BinanceLimitReader", binance.limits)
	}
	if _, ok := binance.codec.(*venue.BinanceOrderCodec); !ok {
		t.Errorf("binance codec = %T, want *BinanceOrderCodec", binance.codec)
	}

	bybit, err := collaboratorsFor(DialectBybit, "bybit", cred)
	if err != nil {
		t.Fatalf("collaboratorsFor(bybit): %v", err)
	}
	if _, ok := bybit.envelope.(transport.BybitEnvelope); !ok {
		t.Errorf("bybit envelope = %T, want BybitEnvelope", bybit.envelope)
	}
	if _, ok := bybit.limits.(transport.BybitLimitReader); !ok {
		t.Errorf("bybit limits = %T, want BybitLimitReader", bybit.limits)
	}
	if _, ok := bybit.codec.(*venue.BybitOrderCodec); !ok {
		t.Errorf("bybit codec = %T, want *BybitOrderCodec", bybit.codec)
	}

	if _, err := collaboratorsFor("kraken", "kraken", cred); !errors.Is(err, ErrDialectUnknown) {
		t.Errorf("collaboratorsFor(unknown) = %v, want ErrDialectUnknown", err)
	}
}

// ---------------------------------------------------------------------------
// Secret resolution (§5.2's key refs)
// ---------------------------------------------------------------------------

func TestResolveSecretFromEnv(t *testing.T) {
	t.Setenv(testKeyVar, "the-key")
	got, err := resolveSecret(envRef(t, testKeyVar))
	if err != nil || got != "the-key" {
		t.Errorf("resolveSecret = %q, %v, want %q, nil", got, err, "the-key")
	}
}

// TestResolveSecretRefusesASetButEmptyVariable is the case the third sentinel
// exists for, and it is invisible in any fixture that only ever unsets.
//
// `export X=` is SET: os.LookupEnv reports it present and the value is "". A
// credential built from it signs correctly over nothing, and the venue's answer
// is an authentication failure — which reads as a revoked key rather than as an
// unset variable on this host.
func TestResolveSecretRefusesASetButEmptyVariable(t *testing.T) {
	t.Setenv(testKeyVar, "")
	_, err := resolveSecret(envRef(t, testKeyVar))
	if !errors.Is(err, ErrSecretEmpty) {
		t.Errorf("resolveSecret(set-but-empty) = %v, want ErrSecretEmpty", err)
	}
	if errors.Is(err, ErrSecretUnresolved) {
		t.Error("a set-but-empty variable must not be reported as unresolved: the two send an operator to different places")
	}
}

func TestResolveSecretRefusesAnUnsetVariable(t *testing.T) {
	os.Unsetenv(testKeyVar)
	_, err := resolveSecret(envRef(t, testKeyVar))
	if !errors.Is(err, ErrSecretUnresolved) {
		t.Errorf("resolveSecret(unset) = %v, want ErrSecretUnresolved", err)
	}
}

// TestResolveSecretFromFileTrimsTheTrailingNewline pins the trim, which is not a
// convenience: a secret file written by an editor or by `echo` carries one, and a
// signature computed over it fails identically to a revoked key.
func TestResolveSecretFromFileTrimsTheTrailingNewline(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("the-secret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	ref, err := config.ParseSecretRef("file:" + path)
	if err != nil {
		t.Fatalf("ParseSecretRef: %v", err)
	}
	got, err := resolveSecret(ref)
	if err != nil || got != "the-secret" {
		t.Errorf("resolveSecret(file) = %q, %v, want %q, nil", got, err, "the-secret")
	}

	// A file holding only whitespace is empty, not a one-character secret.
	if err := os.WriteFile(path, []byte("\n\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := resolveSecret(ref); !errors.Is(err, ErrSecretEmpty) {
		t.Errorf("resolveSecret(whitespace file) = %v, want ErrSecretEmpty", err)
	}
}

// ---------------------------------------------------------------------------
// #168: the journal's filesystem
// ---------------------------------------------------------------------------

// TestClassifyCoversEveryClass is checklist item 4: one name per class, plus the
// default. Without the last row the table's whole default-deny direction — the
// one thing this check is FOR — is never evaluated.
func TestClassifyCoversEveryClass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want filesystemClass
	}{
		{"ext2/3/4", classLocal},
		{"xfs", classLocal},
		{"apfs", classLocal},
		{"nfs", classRemote},
		{"cifs", classRemote},
		{"fuse", classRemote}, // sshfs and s3fs wear a local name
		{"tmpfs", classVolatile},
		{"ramfs", classVolatile},
		{"overlay", classUnknown},    // a container's default, deliberately not admitted
		{"0xdeadbeef", classUnknown}, // an unmapped Linux magic
		{"NFS", classRemote},         // the lookup is case-insensitive
	}
	for _, c := range cases {
		if got := classify(c.name); got != c.want {
			t.Errorf("classify(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestCheckJournalStorageOnThisHost is the syscall half, and it deliberately
// accepts more than one answer.
//
// The filesystem under a temporary directory is a property of the machine
// running the test — apfs on the development host, ext4 or tmpfs on a CI runner
// — so asserting "nil" would make the suite pass or fail on where it ran. What
// IS asserted is that the check reaches a DECISION: either it admits the
// filesystem, or it refuses with one of the three classified refusals. A stub
// that returned a bare error, or one that admitted everything, fails this.
func TestCheckJournalStorageOnThisHost(t *testing.T) {
	t.Parallel()

	err := checkJournalStorage(filepath.Join(t.TempDir(), "journal.db"))
	if err == nil {
		t.Log("this host's temp filesystem is admitted")
		return
	}
	switch {
	case errors.Is(err, ErrJournalStorageRemote),
		errors.Is(err, ErrJournalStorageVolatile),
		errors.Is(err, ErrJournalStorageUnknown):
		t.Logf("this host's temp filesystem is refused, with a classified reason: %v", err)
	default:
		t.Errorf("checkJournalStorage = %v, want nil or one of the three classified refusals", err)
	}
}

// TestCheckJournalStorageOnAMissingDirectory: statfs cannot answer about a
// directory that is not there, and an unanswerable question is a refusal.
func TestCheckJournalStorageOnAMissingDirectory(t *testing.T) {
	t.Parallel()

	err := checkJournalStorage(filepath.Join(t.TempDir(), "no-such-dir", "journal.db"))
	if err == nil {
		t.Error("checkJournalStorage on a missing directory = nil, want a refusal")
	}
}

// ---------------------------------------------------------------------------
// The exit class, which is the slice's second finding as a table
// ---------------------------------------------------------------------------

// TestExitClassIsNotThePosture is the finding stated as a test.
//
// Every row has the SAME posture — PostureReconcileOnly, which is what this
// binary reports on every honest run — so a verdict derived from the posture
// would answer identically to all of them. What separates them is the latch.
//
// Each of the FOUR gates this binary produces gets its own incomplete row, and
// that is not padding: the produced set is a case list in a switch, and a
// mutation dropping one member from it SURVIVED a table that exercised only
// GateNetDeltaAsserted. One row per member is what makes the list checked rather
// than sampled.
func TestExitClassIsNotThePosture(t *testing.T) {
	t.Parallel()

	// The four gates with no producer, which block on every run and mean
	// nothing. GateAccountPosture LEFT this list in #198 item 7: it now has a
	// producer here, so a blocking one is a real incompleteness and the case
	// below asserts exactly that.
	structural := []boot.Gate{
		boot.GateDeactivationsResolved,
		boot.GateMarginOpsResolved, boot.GateWriterAuthority,
	}
	cases := []struct {
		name string
		rep  Report
		want ExitClass
	}{
		{
			name: "clean: only the gates nobody produces are blocking",
			rep:  Report{Posture: boot.PostureReconcileOnly, Blocking: structural},
			want: ExitClean,
		},
		{
			name: "latched: an operator hold",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateVenueJoined}, structural...),
				Latched:  []boot.Gate{boot.GateVenueJoined}},
			want: ExitOperator,
		},
		{
			name: "incomplete: a gate this binary produces did not conclude",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateNetDeltaAsserted}, structural...)},
			want: ExitIncomplete,
		},
		{
			name: "incomplete: the replay gate alone",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateJournalReplayed}, structural...)},
			want: ExitIncomplete,
		},
		{
			name: "incomplete: the durability gate alone",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateJournalDurable}, structural...)},
			want: ExitIncomplete,
		},
		{
			name: "incomplete: the join gate alone",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateVenueJoined}, structural...)},
			want: ExitIncomplete,
		},
		{
			// The gate that crossed. Blocking-and-not-latched means the posture
			// could not be ESTABLISHED — the venue did not answer, a pinned
			// attestation would not load — which a retry may lift. A posture
			// that was READ and found wrong latches and is answered above.
			name: "incomplete: the posture gate alone",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateAccountPosture}, structural...)},
			want: ExitIncomplete,
		},
		{
			name: "operator: a posture that was read and found wrong",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateAccountPosture}, structural...),
				Latched:  []boot.Gate{boot.GateAccountPosture}},
			want: ExitOperator,
		},
		{
			name: "incomplete beats nothing, and latched beats incomplete",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking: append([]boot.Gate{boot.GateJournalDurable}, structural...),
				Latched:  []boot.Gate{boot.GateJournalDurable},
				Storage:  errors.New("nfs")},
			want: ExitOperator,
		},
		{
			// §7.6's alert with NOTHING else wrong: no gate carries the ban
			// fact, so this row is the one where the counter is the only
			// reading — the quieter shape where a fallback absorbed the
			// banned call and every produced gate concluded clean.
			name: "banned: §7.6's operator alert with every produced gate clean",
			rep: Report{Posture: boot.PostureReconcileOnly, Blocking: structural,
				RateLimit: ratelimit.Snapshot{Venue: ids.Venue("binance"), Bans: 1}},
			want: ExitOperator,
		},
		{
			// The row that pays for the branch: a banned join reports
			// OutcomeFailed, which alone reads ExitIncomplete — "a retry may
			// lift it" — and retrying into a Binance ban escalates it. Bans is
			// 2 here and 1 above, so "at least one" and "exactly one" are
			// different predicates across the pair.
			name: "banned outranks incomplete: the retry it invites is what escalates a ban",
			rep: Report{Posture: boot.PostureReconcileOnly,
				Blocking:  append([]boot.Gate{boot.GateVenueJoined}, structural...),
				RateLimit: ratelimit.Snapshot{Venue: ids.Venue("binance"), Bans: 2}},
			want: ExitOperator,
		},
	}
	for _, c := range cases {
		if got := c.rep.ExitClass(); got != c.want {
			t.Errorf("%s: ExitClass() = %s, want %s", c.name, got, c.want)
		}
		if c.rep.Posture != boot.PostureReconcileOnly {
			t.Errorf("%s: the fixture must hold the posture constant, or it is not testing this", c.name)
		}
	}
}

// ---------------------------------------------------------------------------
// The sequence, end to end
// ---------------------------------------------------------------------------

// binanceAccountBody is the fake venue's GET /api/v3/account answer: the BTC
// row the pinned baseline's words repeat, plus a USDT row — with
// omitZeroBalances unset the venue answers with every asset, and the extra row
// is what makes "the coverage set filtered" and "the whole answer consumed"
// different claims.
const binanceAccountBody = `{"balances":[` +
	`{"asset":"BTC","free":"1.00000000","locked":"0.00000000"},` +
	`{"asset":"USDT","free":"1000.00000000","locked":"0.00000000"}]}`

// pinnedCleanBaseline writes a seq-0 spot baseline document and returns its
// config pin (task #179, record §70g). Seq 0 is the virgin-deployment record:
// every journal extends the empty one and `ours` was zero, so the recomputed
// baseline is exactly the stored balance — 1 BTC, the same words the fake
// venue answers live — and the boot's spot assertion concludes with a zero
// delta. The credential fingerprint binding is computed by the SAME derivation
// the root uses, not restated as a literal, because the claim is that the two
// meet.
func pinnedCleanBaseline(t *testing.T, account, apiKey, apiSecret string) *BaselinePinConfig {
	t.Helper()
	cred, err := transport.NewCredential(apiKey, apiSecret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	doc := baseline.Document{
		Kind: baseline.Kind, Version: baseline.Version,
		Venue: "binance", Account: account,
		CredentialSHA256: cred.Fingerprint(),
		ObservedAt:       "2026-08-03T11:00:00Z",
		Balances: []baseline.BalanceRow{{Asset: "BTC", Words: []baseline.Word{
			{Name: "free", Value: "1.00000000"}, {Name: "locked", Value: "0.00000000"},
		}}},
	}
	body, err := baseline.Encode(doc)
	if err != nil {
		t.Fatalf("baseline.Encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "spot-baseline.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write baseline document: %v", err)
	}
	digest := sha256.Sum256(body)
	return &BaselinePinConfig{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

// spotOnly is the deployment shape whose reconcile makes no network call UNTIL
// a baseline is pinned: no scope is required for spot, so Assert enumerates
// nothing, and an empty journal has no dispatched order to join. The clean
// boot test pins one and serves the single wallet read that buys.
// It is ALSO the deployment shape #198 item 6 nearly outlawed and did not: a
// document with no derivative instrument derives an empty §4.1 posture roster,
// and the first cut of validatePosture refused it. §5.2's venue block is
// singular, so this is the ordinary shape of a cross-venue carry's spot half —
// the empty roster is now carried to risk/domain as a STATED fact instead
// (RosterCoverageNoDerivatives), and this fixture is the reason that was found.
func spotOnly(t *testing.T, journal string) Config {
	t.Helper()
	cfg := validConfig(t, journal)
	cfg.Instruments = cfg.Instruments[:1]
	cfg.Venue.Endpoints = cfg.Venue.Endpoints[:1]
	cfg.Venue.Limits = []LimitConfig{
		{Name: "ip_requests", Scope: "venue", Dimension: "requests", Interval: dur(t, "5s"),
			OpeningCeiling: 300, RoutineCeiling: 400, Capacity: 500},
		{Name: "weight_1m", Scope: "family", Product: ids.ProductSpot, Dimension: "weight",
			Interval: dur(t, "1m"), OpeningCeiling: 4200, RoutineCeiling: 5400, Capacity: 6000},
		{Name: "orders_10s", Scope: "family", Product: ids.ProductSpot, Dimension: "orders",
			Interval: dur(t, "10s"), OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100},
	}
	cfg.PositionScopes = nil
	return cfg
}

// TestWithInstrumentFiltersReachesTheRunner pins the Option half of the §2.4
// filters thread-through — NewRunner applies options before assembling
// anything, and a runner built without the option holds the nil map, the
// deployment that plans nothing. The other half of the chain (r.filters →
// oms.Deps → the planner's refusal) is proved end to end by the exercise
// root's off-grid test, which can only refuse through a planner that actually
// received this map.
func TestWithInstrumentFiltersReachesTheRunner(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t, filepath.Join(t.TempDir(), "journal.db"))
	perp := ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear}
	filters := map[ids.VenueInstrument]omsdomain.TradingFilters{perp: {}}

	r, err := NewRunner(snapshotOf(t, cfg), testWall(t, cfg),
		observability.NewRegistry(), io.Discard, WithInstrumentFilters(filters, 36*time.Hour))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if _, ok := r.filters[perp]; !ok || len(r.filters) != 1 {
		t.Fatalf("runner filters = %v, want the supplied one-entry map", r.filters)
	}
	// The bound rides the same option as the map — §69's one-policy rule — so
	// the threading claim covers both halves. 36h is deliberately not a value
	// any config default could collide with.
	if r.filterMaxAge != 36*time.Hour {
		t.Fatalf("runner filterMaxAge = %v, want 36h", r.filterMaxAge)
	}

	bare, err := NewRunner(snapshotOf(t, cfg), testWall(t, cfg),
		observability.NewRegistry(), io.Discard)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if bare.filters != nil {
		t.Fatalf("optionless runner filters = %v, want nil — plans-nothing is the default", bare.filters)
	}
}

func snapshotOf(t *testing.T, cfg Config) config.Snapshot[Config] {
	t.Helper()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return config.Snapshot[Config]{Value: cfg, Rev: config.Revision{}}
}

func testWall(t *testing.T, cfg Config) *clock.Wall {
	t.Helper()
	w, err := clock.NewWall(cfg.Calibrations())
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	return w
}

// TestBootRunsEveryPhaseAndReportsClean is the positive control for the whole
// sequence: an empty journal on this host's filesystem, a pinned seq-0
// baseline, exactly ONE venue call — the spot assertion's wallet read — and
// the four gates this binary produces all reported. Until #179 this fixture
// could not exist: with no baseline the spot leg correctly held
// GateNetDeltaAsserted at UnassertedBaselineUnrecorded, so "clean" was not a
// reachable verdict for a spot deployment.
// binancePostureBodies is §4.1's four Binance probes answered CLEANLY.
//
// The deployment this serves configures no derivative instrument, so the
// per-symbol rules fold over an empty roster (RosterCoverageNoDerivatives) and
// what has to pass is the account-wide half: multi-assets off, one-way position
// mode, and the Portfolio-Margin attestation, which no endpoint here answers.
//
// # The two symbol-scoped bodies are NOT empty, and the asymmetry is a finding
//
// `symbolConfig` returns a row for BTCUSDT even though this deployment trades
// no perpetual, because the VENUE's answer does not depend on our config — a
// Binance account has a margin type and a leverage per futures symbol whether or
// not anything here trades it, and §73j recorded that observations for symbols
// off the configured roster are IGNORED rather than failed. `positionRisk`
// stays empty because it enumerates POSITIONS and this account holds none.
//
// The distinction is load-bearing rather than cosmetic: the codec REFUSES an
// empty symbolConfig array (a futures account always has rows, so an empty one
// means the read was wrong-shaped) and accepts an empty positionRisk. Serving
// `[]` for both is what this fixture did first, and the refusal it produced is
// record §73k F14 — on Binance a spot-only deployment cannot answer §4.1 at all.
var binancePostureBodies = map[string]string{
	"/fapi/v1/multiAssetsMargin": `{"multiAssetsMargin":false}`,
	"/fapi/v1/positionSide/dual": `{"dualSidePosition":false}`,
	"/fapi/v1/symbolConfig":      `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":5}]`,
	"/fapi/v3/positionRisk":      `[]`,
}

// pinnedPortfolioMarginAttestation writes an operator attestation recording
// Portfolio Margin OFF and returns the two-act pin over the bytes on disk —
// the same digest a `sha256sum` gives, which is how an operator produces it
// (the package has no writer; see its doc).
func pinnedPortfolioMarginAttestation(t *testing.T, account, key, secret string) *AttestationPinConfig {
	t.Helper()
	cred, err := transport.NewCredential(key, secret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	body, err := json.MarshalIndent(attestation.Document{
		Kind:             attestation.Kind,
		Version:          attestation.Version,
		Venue:            "binance",
		Account:          account,
		CredentialSHA256: cred.Fingerprint(),
		ObservedAt:       "2026-08-06T09:30:00Z",
		Observer:         "gb",
		Method:           "binance.com account page: Portfolio Margin shows not enrolled",
		Observed:         attestation.ObservedOff,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the attestation: %v", err)
	}
	body = append(body, '\n')
	path := filepath.Join(t.TempDir(), "portfolio-margin.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing the attestation: %v", err)
	}
	digest := sha256.Sum256(body)
	return &AttestationPinConfig{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

// TestAPostureThatWasReadAndFoundWrongLatches is the negative control for the
// posture phase, and a mutation campaign is what asked for it: turning the
// refusing outcome into OutcomePassed SURVIVED, because every boot in this file
// served a CLEAN posture. A gate whose passing path is tested and whose refusing
// path is not is a gate that has never been shown to refuse.
//
// The violation is multi-assets ON, and the choice matters twice over. It is an
// ACCOUNT-WIDE rule, so it fires on a deployment with no derivative instrument —
// the per-symbol half is empty here, and a fixture violating a per-symbol rule
// would prove nothing about this deployment. And it is a rule Profiles §2 calls
// mutually exclusive with Isolated, so the account really is untradeable.
//
// The outcome must be NEEDS OPERATOR rather than FAILED: §73c-3 ruled that this
// engine never repairs a posture, so no retry can lift it. That distinction is
// what the assertions below pin, because both spellings block and only one of
// them tells an operator to go and look.
func TestAPostureThatWasReadAndFoundWrongLatches(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v3/account" {
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, binanceAccountBody)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/multiAssetsMargin" {
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, `{"multiAssetsMargin":true}`)
			return
		}
		if body, ok := binancePostureBodies[r.URL.Path]; ok && r.Method == http.MethodGet {
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, body)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in posture fake"}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := spotOnly(t, filepath.Join(dir, "journal.db"))
	cfg.Venue.Endpoints[0].Scheme = "http"
	cfg.Venue.Endpoints[0].Host = strings.TrimPrefix(srv.URL, "http://")
	cfg.SpotBaseline = pinnedCleanBaseline(t, cfg.Account, "key", "secret")
	cfg.Venue.Endpoints = append(cfg.Venue.Endpoints, EndpointConfig{
		Product: ids.ProductLinearPerp, Scheme: "http",
		Host: strings.TrimPrefix(srv.URL, "http://"),
	})
	cfg.Venue.Limits = append(cfg.Venue.Limits,
		LimitConfig{Name: "weight_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "weight", Interval: dur(t, "1m"),
			OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
		LimitConfig{Name: "orders_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "orders", Interval: dur(t, "1m"),
			OpeningCeiling: 840, RoutineCeiling: 1080, Capacity: 1200},
	)
	cfg.PortfolioMarginAttestation = pinnedPortfolioMarginAttestation(t, cfg.Account, "key", "secret")
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if o, declared := r.checklist.Outcome(boot.GateAccountPosture); !declared || o != boot.OutcomeNeedsOperator {
		t.Fatalf("GateAccountPosture = %s (declared %v), want OutcomeNeedsOperator — no retry repairs a posture", o, declared)
	}
	if got := rep.ExitClass(); got != ExitOperator {
		t.Errorf("ExitClass() = %s, want ExitOperator; latched = %v", got, rep.Latched)
	}
	// The verdict reaches the report and NAMES the setting, which is what an
	// operator acts on. A latched gate with an empty verdict would be a refusal
	// nobody can diagnose.
	if rep.PostureCause != nil {
		t.Errorf("PostureCause = %v, want nil — the posture WAS established, and it is wrong", rep.PostureCause)
	}
	var named bool
	for _, f := range rep.AccountPosture.Failures {
		if f.Setting == riskdomain.SettingMultiAssets && f.Reason == riskdomain.ReasonMismatch {
			named = true
			if f.Observed != "true" {
				t.Errorf("the failure carries observed %q, want the venue's own word", f.Observed)
			}
		}
	}
	if !named {
		t.Errorf("no SettingMultiAssets mismatch in the verdict:\n%s", rep.AccountPosture)
	}
	// And the checks that PASS still pass — otherwise a fixture that broke the
	// whole read would produce the same latched gate for a different reason.
	if n := len(rep.AccountPosture.Failures); n != 1 {
		t.Errorf("%d failures, want exactly the one this fixture injects:\n%s", n, rep.AccountPosture)
	}
}

// TestAPostureThatCouldNotBeReadIsIncompleteNotAnAlert is the other half of the
// posture gate's two refusing paths, and it was also a surviving mutant: making
// the unestablishable case report OutcomePassed changed nothing anywhere.
//
// The distinction it pins is the one the whole phase is built around. A posture
// that was READ and found wrong latches — no retry repairs it. A posture that
// could not be read at all is INCOMPLETE — a retry may well answer. Collapsing
// them either way is a real cost: as an alert it sends a human to look at an
// account nobody has read, and as incompleteness it invites the retry a genuine
// violation will never satisfy.
//
// The verdict must stay the zero one, and asking it whether it is satisfied must
// REFUSE rather than say no: "the read failed" and "nothing was wrong" are the
// same zero value everywhere except through ErrVerdictVacuous.
func TestAPostureThatCouldNotBeReadIsIncompleteNotAnAlert(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v3/account" {
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, binanceAccountBody)
			return
		}
		// Every posture probe refused. A 500 rather than a 400, so the failure
		// is the venue not answering rather than this system asking wrongly.
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"code":-1001,"msg":"internal error"}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := spotOnly(t, filepath.Join(dir, "journal.db"))
	cfg.Venue.Endpoints[0].Scheme = "http"
	cfg.Venue.Endpoints[0].Host = strings.TrimPrefix(srv.URL, "http://")
	cfg.SpotBaseline = pinnedCleanBaseline(t, cfg.Account, "key", "secret")
	cfg.Venue.Endpoints = append(cfg.Venue.Endpoints, EndpointConfig{
		Product: ids.ProductLinearPerp, Scheme: "http",
		Host: strings.TrimPrefix(srv.URL, "http://"),
	})
	cfg.Venue.Limits = append(cfg.Venue.Limits,
		LimitConfig{Name: "weight_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "weight", Interval: dur(t, "1m"),
			OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
		LimitConfig{Name: "orders_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "orders", Interval: dur(t, "1m"),
			OpeningCeiling: 840, RoutineCeiling: 1080, Capacity: 1200},
	)
	cfg.PortfolioMarginAttestation = pinnedPortfolioMarginAttestation(t, cfg.Account, "key", "secret")
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if o, declared := r.checklist.Outcome(boot.GateAccountPosture); !declared || o != boot.OutcomeFailed {
		t.Fatalf("GateAccountPosture = %s (declared %v), want OutcomeFailed — a retry may answer", o, declared)
	}
	for _, g := range rep.Latched {
		if g == boot.GateAccountPosture {
			t.Errorf("a posture that was never read must not latch: %v", rep.Latched)
		}
	}
	if got := rep.ExitClass(); got != ExitIncomplete {
		t.Errorf("ExitClass() = %s, want ExitIncomplete", got)
	}
	if rep.PostureCause == nil {
		t.Error("PostureCause is nil — an unestablished posture must name why")
	}
	if n := len(rep.AccountPosture.Failures); n != 0 {
		t.Errorf("%d failures on a posture that was never read", n)
	}
	// The interlock: a caller ignoring PostureCause and asking the verdict gets
	// a REFUSAL, never a no and never a yes.
	if ok, err := rep.AccountPosture.Satisfied(); ok || !errors.Is(err, riskdomain.ErrVerdictVacuous) {
		t.Errorf("Satisfied() = %v, %v; want false and ErrVerdictVacuous", ok, err)
	}
}

func TestBootRunsEveryPhaseAndReportsClean(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	var accountReads, postureReads atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v3/account" {
			accountReads.Add(1)
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, binanceAccountBody)
			return
		}
		if body, ok := binancePostureBodies[r.URL.Path]; ok && r.Method == http.MethodGet {
			postureReads.Add(1)
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, body)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in clean-boot fake"}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := spotOnly(t, filepath.Join(dir, "journal.db"))
	cfg.Venue.Endpoints[0].Scheme = "http"
	cfg.Venue.Endpoints[0].Host = strings.TrimPrefix(srv.URL, "http://")
	cfg.SpotBaseline = pinnedCleanBaseline(t, cfg.Account, "key", "secret")
	// §4.1's posture read (#198 item 7) lives on the FUTURES host, so a
	// deployment that trades spot only still has to route the perp family for
	// the gate to be answerable at all — see record §73k F14, which is what
	// this line is evidence of rather than a fixture convenience.
	cfg.Venue.Endpoints = append(cfg.Venue.Endpoints, EndpointConfig{
		Product: ids.ProductLinearPerp, Scheme: "http",
		Host: strings.TrimPrefix(srv.URL, "http://"),
	})
	cfg.Venue.Limits = append(cfg.Venue.Limits,
		LimitConfig{Name: "weight_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "weight", Interval: dur(t, "1m"),
			OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
		LimitConfig{Name: "orders_1m", Scope: "family", Product: ids.ProductLinearPerp,
			Dimension: "orders", Interval: dur(t, "1m"),
			OpeningCeiling: 840, RoutineCeiling: 1080, Capacity: 1200},
	)
	// And the Portfolio-Margin attestation, which is the ONE §4.1 check with no
	// read path anywhere: without a pinned document this boot refuses by name
	// (ReasonUnattested), so this line is what makes the clean answer below a
	// statement about the account rather than about a missing file.
	cfg.PortfolioMarginAttestation = pinnedPortfolioMarginAttestation(t, cfg.Account, "key", "secret")
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Posture != boot.PostureReconcileOnly {
		t.Errorf("posture = %s, want PostureReconcileOnly — four gates have no producer", rep.Posture)
	}
	if len(rep.Latched) != 0 {
		t.Errorf("latched = %v, want none on a clean boot", rep.Latched)
	}
	if got := rep.ExitClass(); got != ExitClean {
		t.Errorf("ExitClass() = %s, want ExitClean; blocking = %v, cause = %v",
			got, rep.Blocking, rep.Reconciliation.Cause)
	}
	// The FIVE gates this binary produces must all have concluded, which is what
	// makes the ExitClean above a statement rather than an accident of the
	// structural four.
	for _, g := range []boot.Gate{boot.GateJournalDurable, boot.GateJournalReplayed,
		boot.GateVenueJoined, boot.GateNetDeltaAsserted, boot.GateAccountPosture} {
		o, declared := r.checklist.Outcome(g)
		if !declared || o != boot.OutcomePassed {
			t.Errorf("%s = %s (declared %v), want OutcomePassed", g, o, declared)
		}
	}
	// And the journal exists, which is the evidence the phase actually ran.
	if _, err := os.Stat(filepath.Join(dir, "journal.db")); err != nil {
		t.Errorf("journal not created: %v", err)
	}
	// The governor's snapshot reaches the report — the positive control for
	// §7.6's alert wiring, without which the ban test below could pass against
	// a report that never carried the snapshot at all (a zero snapshot has
	// zero bans too). Venue proves the wiring; Bans proves the clean answer.
	if rep.RateLimit.Venue != cfg.Venue.Venue {
		t.Errorf("RateLimit.Venue = %q, want %q — the governor's state must reach the report",
			rep.RateLimit.Venue, cfg.Venue.Venue)
	}
	if rep.RateLimit.Bans != 0 {
		t.Errorf("RateLimit.Bans = %d on a clean boot", rep.RateLimit.Bans)
	}
	// Exactly one wallet read: the spot assertion consumed the pinned baseline
	// against a live balance rather than passing vacuously, and no second look
	// ran — a zero delta needs none. An exact count, so a retry loop or a
	// skipped read cannot hide.
	if got := accountReads.Load(); got != 1 {
		t.Errorf("the venue saw %d account reads, want exactly 1", got)
	}
	// Four posture probes, the exact figure §73c-2 measured for a Binance read.
	// An exact count because it is the one instrument that distinguishes "the
	// posture was asserted" from "the verdict was satisfied for another reason"
	// — a guard that read nothing produces a verdict that evaluated nothing,
	// and Satisfied() would have refused it, but a guard that read THREE of
	// four probes would not be visible in the outcome at all.
	if got := postureReads.Load(); got != 4 {
		t.Errorf("the venue saw %d posture probes, want exactly 4", got)
	}
	if ok, err := rep.AccountPosture.Satisfied(); !ok || err != nil {
		t.Errorf("the report's verdict = %v, %v; want satisfied:\n%s", ok, err, rep.AccountPosture)
	}
	if rep.PostureCause != nil {
		t.Errorf("PostureCause = %v, want nil on a boot whose posture was established", rep.PostureCause)
	}
}

// --- §4.1's per-symbol half, end to end (#198 item 8) ------------------
//
// The collapse checklist arriving one layer out from §73g, which was written
// against risk/domain and stopped there. Every end-to-end posture fixture above
// uses spotOnly, whose roster is EMPTY — so RosterCoverageNoDerivatives had
// every boot witness in this file and RosterCoverageDerivatives, the ordinary
// production shape, had NONE. The consequence is the one the checklist exists to
// find: the whole per-symbol chain — an operator's `leverage` pin, through
// PostureRequirement, the guard, the ACL and the venue codec, to a rule
// comparison — was reachable at boot and asserted nowhere, so a pin attached to
// the wrong symbol was invisible to every test in this package.
//
// The tell was the usual one: the fixture did not contain the thing under test.
// A roster of zero cannot show that "every configured symbol" differs from "the
// first configured symbol", and cannot show a pin travelling at all
// ([[fixture-count-of-one-collapses]], §73g item 1).

const (
	// DIFFERENT pins, so "this symbol's pin" and "the first symbol's pin" are
	// different values. §73g item 5's collapse, at the boot layer.
	firstBootPin  = 5
	secondBootPin = 3
)

// perpRoster is a deployment that CONFIGURES two derivatives and no spot leg.
//
// Two so the roster claim cannot collapse; no spot leg so that §7.5's spot
// assertion stays out of the way — this fixture is aimed at the posture gate,
// and a boot refused by a different gate would prove nothing about it.
func perpRoster(t *testing.T, journal string) Config {
	t.Helper()
	cfg := validConfig(t, journal)
	cfg.Instruments = []InstrumentConfig{
		{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
			Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Leverage: firstBootPin},
		{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
			Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Leverage: secondBootPin},
	}
	// Both bounds are required exactly when a spot instrument exists, and
	// refused when none does (record §70g), so they go with the spot row.
	cfg.SpotSettle = config.Duration{}
	cfg.Resolution.AssumedReach = nil
	return cfg
}

// rosterPostureBodies answers §4.1's four Binance probes for perpRoster's
// deployment.
//
// secondLeverage is the parameter under test in the negative case below; every
// other field is clean. The position rows carry positionAmt "0" deliberately:
// §7.5's net-delta invariant runs in the same boot, and a non-zero perp leg with
// no spot leg behind it would latch a DIFFERENT gate.
func rosterPostureBodies(secondLeverage int) map[string]string {
	return map[string]string{
		"/fapi/v1/multiAssetsMargin": `{"multiAssetsMargin":false}`,
		"/fapi/v1/positionSide/dual": `{"dualSidePosition":false}`,
		"/fapi/v1/symbolConfig": fmt.Sprintf(
			`[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":%d},`+
				`{"symbol":"ETHUSDT","marginType":"ISOLATED","leverage":%d}]`,
			firstBootPin, secondLeverage),
		"/fapi/v3/positionRisk": `[` +
			`{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"0","isAutoAddMargin":"false"},` +
			`{"symbol":"ETHUSDT","positionSide":"BOTH","positionAmt":"0","isAutoAddMargin":"false"}]`,
	}
}

// rosterPostureServer serves those bodies and refuses anything else, so a probe
// asking a path this fixture did not anticipate fails loudly rather than
// silently producing an absent observation.
func rosterPostureServer(t *testing.T, bodies map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := bodies[r.URL.Path]; ok && r.Method == http.MethodGet {
			w.Header().Set("X-Mbx-Used-Weight-1m", "20")
			fmt.Fprint(w, body)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"code":-1000,"msg":"unexpected %s in roster posture fake"}`, r.URL.Path)
	}))
}

// routeTo points every family at the fake and pins the attestation, which is the
// one §4.1 check no endpoint answers.
func routeTo(t *testing.T, cfg *Config, srv *httptest.Server) {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "http://")
	for i := range cfg.Venue.Endpoints {
		cfg.Venue.Endpoints[i].Scheme = "http"
		cfg.Venue.Endpoints[i].Host = host
	}
	cfg.PortfolioMarginAttestation = pinnedPortfolioMarginAttestation(t, cfg.Account, "key", "secret")
}

// TestBootAssertsThePerSymbolPostureOfEveryConfiguredSymbol is the positive
// control this file did not have: a boot whose §4.1 roster is NON-empty and
// whose per-symbol rules all pass.
//
// It is what makes the negative case below a statement about the rule rather
// than about a fixture that could never pass — and on its own it is the first
// evidence anywhere that a configured leverage pin reaches a comparison at all.
func TestBootAssertsThePerSymbolPostureOfEveryConfiguredSymbol(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	srv := rosterPostureServer(t, rosterPostureBodies(secondBootPin))
	defer srv.Close()

	cfg := perpRoster(t, filepath.Join(t.TempDir(), "journal.db"))
	routeTo(t, &cfg, srv)
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if o, declared := r.checklist.Outcome(boot.GateAccountPosture); !declared || o != boot.OutcomePassed {
		t.Fatalf("GateAccountPosture = %s (declared %v), want OutcomePassed:\n%s",
			o, declared, rep.AccountPosture)
	}
	// The exact count is the instrument: three account-wide rules plus three
	// per-symbol rules over TWO symbols is 9, and it is predictable from the rule
	// table. "Evaluated > 0" would be satisfied by a fold that visited one symbol
	// — which is exactly the defect a roster of one cannot see.
	if rep.AccountPosture.Evaluated != 9 {
		t.Errorf("evaluated %d checks, want 9 (3 account + 3×2 symbol):\n%s",
			rep.AccountPosture.Evaluated, rep.AccountPosture)
	}
	if ok, err := rep.AccountPosture.Satisfied(); !ok || err != nil {
		t.Errorf("the report's verdict = %v, %v; want satisfied:\n%s", ok, err, rep.AccountPosture)
	}
}

// TestBootRefusesWhenTheSECONDConfiguredSymbolViolatesItsOwnPin is §73g items 1
// and 5 at the layer they were never run at.
//
// Two collapses are broken at once, and neither is visible from a roster of one:
// the violation is on the SECOND symbol, so a fold that checked only the first
// would report a clean posture; and the leverage the venue reports for it is
// FIRST symbol's pin, so a comparison reading "the roster's first pin" instead
// of "this symbol's pin" would find it satisfied. Both wrong readings pass every
// other test in this package.
func TestBootRefusesWhenTheSECONDConfiguredSymbolViolatesItsOwnPin(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	srv := rosterPostureServer(t, rosterPostureBodies(firstBootPin))
	defer srv.Close()

	cfg := perpRoster(t, filepath.Join(t.TempDir(), "journal.db"))
	routeTo(t, &cfg, srv)
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Read and found wrong, so it latches: §73c-3 forbids this engine repairing a
	// posture, and no retry lifts a leverage the operator has to go and change.
	if o, declared := r.checklist.Outcome(boot.GateAccountPosture); !declared || o != boot.OutcomeNeedsOperator {
		t.Fatalf("GateAccountPosture = %s (declared %v), want OutcomeNeedsOperator", o, declared)
	}
	if rep.PostureCause != nil {
		t.Errorf("PostureCause = %v, want nil — the posture WAS established", rep.PostureCause)
	}
	// Exactly one failure, naming the second symbol, its OWN pin as required and
	// the venue's word as observed. Asserting only that something failed would be
	// satisfied by a fixture that broke the whole read.
	if n := len(rep.AccountPosture.Failures); n != 1 {
		t.Fatalf("%d failures, want exactly the one this fixture injects:\n%s", n, rep.AccountPosture)
	}
	f := rep.AccountPosture.Failures[0]
	if f.Setting != riskdomain.SettingLeverage || f.Subject != "ETHUSDT" {
		t.Fatalf("failure = %s on %q, want SettingLeverage on ETHUSDT", f.Setting, f.Subject)
	}
	if f.Reason != riskdomain.ReasonMismatch {
		t.Errorf("reason = %s, want %s", f.Reason, riskdomain.ReasonMismatch)
	}
	if f.Required != strconv.Itoa(secondBootPin) {
		t.Errorf("required %q, want %q — the SECOND symbol's own pin, not the first's",
			f.Required, strconv.Itoa(secondBootPin))
	}
	if f.Observed != strconv.Itoa(firstBootPin) {
		t.Errorf("observed %q, want the venue's own word %q", f.Observed, strconv.Itoa(firstBootPin))
	}
}

// TestBootOnAFlatAccountRefusesEveryConfiguredSymbol is record §73j F9 as an
// EXECUTABLE claim rather than a paragraph, and it is the evidence task #199's
// ruling needs.
//
// F9 is not a bug report, it is a consequence nobody had run: per-symbol
// coverage is answered from the CONFIGURED roster (F4), and Binance's
// auto-add-margin comes from positionRisk, which enumerates POSITIONS. So an
// account holding none — every cold boot, and every deployment before its first
// trade — refuses on isAutoAddMargin for every configured symbol, and §73c-2 puts
// the same read before every trade. The gate that must pass before the first
// position opens cannot pass until a position exists.
//
// The assertion is deliberately of the STRICT reading, which is what is
// implemented: a fixture must not silently answer a question left to an operator
// ([[a-known-exact-count-is-an-instrument]] — the count is what makes it
// unambiguous which rule fired). When #199 rules, this test changes with it, and
// its failure is the reminder that it must.
func TestBootOnAFlatAccountRefusesEveryConfiguredSymbol(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	bodies := rosterPostureBodies(secondBootPin)
	// A flat account: symbolConfig still answers for both symbols (a futures
	// account is configured whether or not it holds anything), and positionRisk
	// is empty because it enumerates positions. That asymmetry is the finding.
	bodies["/fapi/v3/positionRisk"] = `[]`
	srv := rosterPostureServer(t, bodies)
	defer srv.Close()

	cfg := perpRoster(t, filepath.Join(t.TempDir(), "journal.db"))
	routeTo(t, &cfg, srv)
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if o, declared := r.checklist.Outcome(boot.GateAccountPosture); !declared || o != boot.OutcomeNeedsOperator {
		t.Fatalf("GateAccountPosture = %s (declared %v), want OutcomeNeedsOperator", o, declared)
	}
	// Exactly two failures — one per configured symbol, and nothing else. A
	// roster of one could not tell "every configured symbol refuses" from "a
	// configured symbol refuses", which is the whole weight of F9.
	if n := len(rep.AccountPosture.Failures); n != 2 {
		t.Fatalf("%d failures, want one per configured symbol:\n%s", n, rep.AccountPosture)
	}
	for _, want := range []string{"BTCUSDT", "ETHUSDT"} {
		var found bool
		for _, f := range rep.AccountPosture.Failures {
			if f.Subject != want {
				continue
			}
			found = true
			if f.Setting != riskdomain.SettingAutoAddMargin {
				t.Errorf("%s failed on %s, want SettingAutoAddMargin", want, f.Setting)
			}
			// UNOBSERVED, not a mismatch: the venue said nothing about this
			// symbol. The two spellings send an operator to different places —
			// one to their account, one to their config.
			if f.Reason != riskdomain.ReasonUnobserved {
				t.Errorf("%s failed with %s, want %s", want, f.Reason, riskdomain.ReasonUnobserved)
			}
		}
		if !found {
			t.Errorf("no failure names %s:\n%s", want, rep.AccountPosture)
		}
	}
}

// TestBootWithoutABaselineHoldsAtUnrecorded is §70g's virgin path (§70h item
// 15): the same spot deployment with NO spot_baseline — no placeholder file
// anywhere — boots to a HOLDING gate that names UnassertedBaselineUnrecorded,
// with no venue call at all, because there is no baseline to compare the
// wallet against. This is the state the first `arblab baseline record` runs
// from, so it must be an operator conversation, never a wiring error.
func TestBootWithoutABaselineHoldsAtUnrecorded(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	cfg := spotOnly(t, filepath.Join(t.TempDir(), "journal.db"))
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := rep.ExitClass(); got != ExitOperator {
		t.Errorf("ExitClass() = %s, want ExitOperator — an uncompared wallet needs a human, not a retry", got)
	}
	if o, _ := r.checklist.Outcome(boot.GateNetDeltaAsserted); o != boot.OutcomeNeedsOperator {
		t.Errorf("GateNetDeltaAsserted = %s, want OutcomeNeedsOperator", o)
	}
	want := []application.UnassertedInstrument{{
		Instrument: ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"},
		Reason:     application.UnassertedBaselineUnrecorded,
	}}
	got := rep.Reconciliation.Assertion.Unasserted
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("Unasserted = %+v, want %+v", got, want)
	}
	if rep.RateLimit.Bans != 0 {
		t.Errorf("Bans = %d on a boot that made no venue call", rep.RateLimit.Bans)
	}
}

// TestRunnerOwnsTheSpendLockUntilItsOMSCloserRuns joins the process lock to
// the root lifecycle. The second acquisition happens after wiring has returned,
// so it detects a helper that deferred Close on its successful return; the
// reacquisition after shutdown proves the phase closer releases the same token.
func TestRunnerOwnsTheSpendLockUntilItsOMSCloserRuns(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	dir := t.TempDir()
	cfg := spotOnly(t, filepath.Join(dir, "journal.db"))
	r := newRunnerFor(t, cfg, func(string) error { return nil })
	checklist, err := boot.NewChecklist(declaredGates()...)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	r.checklist = checklist
	running, err := r.Sequence().Run(context.Background())
	if err != nil {
		t.Fatalf("Sequence.Run: %v", err)
	}

	cred, err := transport.NewCredential("key", "secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	if second, err := transport.AcquireSpendLock(cfg.Venue.SpendLockDir, transport.SpendDomainBinance, cred); !errors.Is(err, transport.ErrSpendLockHeld) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("second AcquireSpendLock while Runner is up = %v, want ErrSpendLockHeld", err)
	}
	if _, err := running.Shutdown(context.Background(), r.clk); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	reacquired, err := transport.AcquireSpendLock(cfg.Venue.SpendLockDir, transport.SpendDomainBinance, cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock after normal shutdown: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("Close reacquired lock: %v", err)
	}
}

// TestRunnerReleasesSpendLockWhenSignedClientConstructionFails exercises the
// release site inside newOrderClient. The malformed host passes document
// validation and reaches the stdlib serialization refusal only after the lock
// has been acquired.
func TestRunnerReleasesSpendLockWhenSignedClientConstructionFails(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	dir := t.TempDir()
	cfg := spotOnly(t, filepath.Join(dir, "journal.db"))
	cfg.Venue.Endpoints[0].Host = `ho"st`
	r := newRunnerFor(t, cfg, func(string) error { return nil })
	checklist, err := boot.NewChecklist(declaredGates()...)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	r.checklist = checklist
	running, err := r.Sequence().Run(context.Background())
	if err != nil {
		t.Fatalf("Sequence.Run: %v", err)
	}
	defer func() { _, _ = running.Shutdown(context.Background(), r.clk) }()
	if !r.Halted() || r.BootReport().Reconciliation.Cause == nil {
		t.Fatalf("Runner = halted %v, cause %v; want halted with wiring diagnosis",
			r.Halted(), r.BootReport().Reconciliation.Cause)
	}
	cred, err := transport.NewCredential("key", "secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	reacquired, err := transport.AcquireSpendLock(cfg.Venue.SpendLockDir, transport.SpendDomainBinance, cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock after newOrderClient failure: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("Close reacquired lock: %v", err)
	}
}

// TestRunnerReleasesSpendLockWhenOMSConstructionFails exercises the distinct
// release site in startOMS. An existing directory is not a SQLite journal, so
// every venue collaborator—including the locked client—constructs before the
// real OMS adapter refuses it; the root remains alive and halted with that
// diagnosis while the now-unreachable capability's domain is immediately free.
func TestRunnerReleasesSpendLockWhenOMSConstructionFails(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	dir := t.TempDir()
	lockDir := filepath.Join(dir, "locks")
	if err := os.Mkdir(lockDir, 0o755); err != nil {
		t.Fatalf("Mkdir lock dir: %v", err)
	}
	cfg := spotOnly(t, dir)
	cfg.Venue.SpendLockDir = lockDir
	cfg.Venue.Endpoints[0].Scheme = "http"
	cfg.Venue.Endpoints[0].Host = "127.0.0.1:1"
	r := newRunnerFor(t, cfg, func(string) error { return nil })
	checklist, err := boot.NewChecklist(declaredGates()...)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	r.checklist = checklist
	running, err := r.Sequence().Run(context.Background())
	if err != nil {
		t.Fatalf("Sequence.Run: %v", err)
	}
	defer func() { _, _ = running.Shutdown(context.Background(), r.clk) }()
	rep := r.BootReport()
	if !r.Halted() || rep.Reconciliation.Cause == nil {
		t.Fatalf("Runner = halted %v, cause %v; want halted with oms.New diagnosis",
			r.Halted(), rep.Reconciliation.Cause)
	}
	if outcome, _ := r.Checklist().Outcome(boot.GateJournalReplayed); outcome != boot.OutcomeFailed {
		t.Errorf("GateJournalReplayed = %s, want OutcomeFailed", outcome)
	}
	cred, err := transport.NewCredential("key", "secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	reacquired, err := transport.AcquireSpendLock(lockDir, transport.SpendDomainBinance, cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock after oms.New failure: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("Close reacquired lock: %v", err)
	}
}

// TestABanDuringTheBootIsAnOperatorAlert is §7.6's MUST delivered end to end:
// the venue answers the position query with 418, the transport files
// PenaltyBan into the governor, and the ROOT — the alert deliverer ratelimit's
// own doc names — turns the counter into ExitOperator.
//
// The gate the failure lands on reads OutcomeFailed, which alone is
// ExitIncomplete: "a retry may lift it". For a ban that advice is the harm —
// Binance escalates repeat violations from two minutes toward three days — so
// the assertion worth the fixture is the verdict, and the isolating fact is
// that NOTHING LATCHED: the class can only have come from the bans counter.
func TestABanDuringTheBootIsAnOperatorAlert(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	var banned atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/fapi/v3/positionRisk" {
			banned.Add(1)
			w.WriteHeader(http.StatusTeapot)
			fmt.Fprint(w, `{"code":-1003,"msg":"Way too much request weight used; IP banned."}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in ban fake"}`)
	}))
	defer srv.Close()

	cfg := validConfig(t, filepath.Join(t.TempDir(), "journal.db"))
	host := strings.TrimPrefix(srv.URL, "http://")
	for i := range cfg.Venue.Endpoints {
		cfg.Venue.Endpoints[i].Scheme = "http"
		cfg.Venue.Endpoints[i].Host = host
	}
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run = %v — a banned boot is a report, not an error", err)
	}
	if banned.Load() == 0 {
		t.Fatal("fixture: the fake was never asked for positions — the 418 arm is unreachable")
	}
	if rep.RateLimit.Bans != 1 {
		t.Errorf("RateLimit.Bans = %d, want 1", rep.RateLimit.Bans)
	}
	if !rep.RateLimit.Paused || rep.RateLimit.PauseKind != ratelimit.PenaltyBan {
		t.Errorf("pause = (%v, %s), want an in-force PenaltyBan pause",
			rep.RateLimit.Paused, rep.RateLimit.PauseKind)
	}
	if len(rep.Latched) != 0 {
		t.Errorf("latched = %v — with a latch the verdict below proves nothing about §7.6's branch",
			rep.Latched)
	}
	if got := rep.ExitClass(); got != ExitOperator {
		t.Errorf("ExitClass() = %s, want ExitOperator — ExitIncomplete invites the retry that escalates the ban", got)
	}
}

// TestAStorageRefusalLatchesAndTheJournalIsNeverOPENED is #168's whole point.
//
// The order is the content: a database CREATED on a network filesystem is
// already the thing this refuses, so the check has to run before the open and the
// evidence is that no file exists afterwards. The outcome is
// OutcomeEvidenceUnusable — halted AND latching — because reconcile-only would
// permit the reconciliation writes that re-drive what the untrustworthy journal
// says happened.
func TestAStorageRefusalLatchesAndTheJournalIsNeverOPENED(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	dir := t.TempDir()
	journal := filepath.Join(dir, "journal.db")
	cfg := spotOnly(t, journal)
	r := newRunnerFor(t, cfg, func(string) error { return ErrJournalStorageRemote })

	rep, err := r.run(t)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := rep.Posture; got != boot.PostureHalted {
		t.Errorf("posture = %s, want PostureHalted", got)
	}
	if len(rep.Latched) != 1 || rep.Latched[0] != boot.GateJournalDurable {
		t.Errorf("latched = %v, want [GateJournalDurable]", rep.Latched)
	}
	if got := rep.ExitClass(); got != ExitOperator {
		t.Errorf("ExitClass() = %s, want ExitOperator", got)
	}
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Errorf("journal exists after a storage refusal (stat err %v) — the check must precede the open", err)
	}
	// The replay and join gates stay UNREPORTED rather than failed: they never
	// ran, and unreported is the honest value that still holds the posture.
	for _, g := range []boot.Gate{boot.GateJournalReplayed, boot.GateVenueJoined} {
		if o, _ := r.checklist.Outcome(g); o != boot.OutcomeUnreported {
			t.Errorf("%s = %s, want OutcomeUnreported", g, o)
		}
	}
}

// TestBootRefusesASequenceWhoseTeardownOutlastsTheWatchdog is §7.4's bound
// reaching the wiring file: six phases at a two-second budget is twelve seconds,
// and a ten-second silence threshold makes an ordinary restart present to the
// watchdog exactly as a dead host.
func TestBootRefusesASequenceWhoseTeardownOutlastsTheWatchdog(t *testing.T) {
	t.Setenv(testKeyVar, "key")
	t.Setenv(testSecretVar, "secret")

	cfg := spotOnly(t, filepath.Join(t.TempDir(), "journal.db"))
	cfg.WatchdogSilence = dur(t, "10s")
	cfg.StopBudget = dur(t, "2s")
	r := newRunnerFor(t, cfg, func(string) error { return nil })

	if _, err := r.run(t); err == nil {
		t.Error("Run with a teardown budget above the watchdog threshold = nil, want a refusal")
	} else if !strings.Contains(err.Error(), "stop budget") {
		t.Errorf("Run = %v, want a refusal naming the stop budget", err)
	}
}

// newRunnerFor builds the root with the storage check injected.
//
// The seam exists because the real check's answer is a property of the machine
// the test runs on — apfs here, ext4 or tmpfs on a runner — and a sequence test
// that failed on a tmpfs /tmp would be testing the host. The real check has its
// own tests above, including one that runs it against this host and accepts every
// classified answer.
func newRunnerFor(t *testing.T, cfg Config, check func(string) error) *Runner {
	t.Helper()
	snap := snapshotOf(t, cfg)
	return &Runner{
		cfg: snap.Value, clk: testWall(t, cfg), reg: observability.NewRegistry(),
		stderr: io.Discard, rev: strings.Repeat("0", 64), storageCheck: check,
	}
}

func (r *Runner) run(t *testing.T) (Report, error) {
	t.Helper()
	checklist, err := boot.NewChecklist(declaredGates()...)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	r.checklist = checklist
	running, err := r.Sequence().Run(context.Background())
	if err != nil {
		return r.BootReport(), err
	}
	if _, err := running.Shutdown(context.Background(), r.clk); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	return r.BootReport(), nil
}
