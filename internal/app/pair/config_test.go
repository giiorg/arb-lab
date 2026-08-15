package pair

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation"
)

func ref(t *testing.T, s string) config.SecretRef {
	t.Helper()
	r, err := config.ParseSecretRef(s)
	if err != nil {
		t.Fatalf("ParseSecretRef(%q): %v", s, err)
	}
	return r
}

func dur(t *testing.T, s string) config.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatalf("ParseDuration(%q): %v", s, err)
	}
	return config.NewDuration(d)
}

// testWall is the injected clock every fixture here reads time from. No venue
// frame is calibrated: nothing in these tests converts to a venue's clock, and
// a calibration nobody uses is a fixture stating a fact it does not test.
func testWall(t *testing.T) *clock.Wall {
	t.Helper()
	w, err := clock.NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	return w
}

// timeAgo is a RELATIVE instant. Reference-row freshness is relative by nature,
// and a literal date in a fixture starts failing max_age some months after it
// was typed — a test that decays into a false alarm.
func timeAgo(t *testing.T, d time.Duration) time.Time {
	t.Helper()
	return testWall(t).Now().Add(-d)
}

// writeCrossVenueReference commits a four-row reference revision — one fee row
// and one lot row per VENUE — to the test's own directory.
//
// The two lot steps are the caller's because the two sides' grids are what a
// cross-venue hedge has to survive: leg A's fill is sized as an order on leg
// B's grid, so a fixture giving both venues one step would make "quantized to
// A's grid" and "quantized to B's" the same number.
func writeCrossVenueReference(t *testing.T, observedAt time.Time, stepA, stepB string) string {
	t.Helper()
	return writeCrossVenueReferenceTicks(t, observedAt, stepA, stepB, "0.0001", "0.0001")
}

func writeCrossVenueReferenceTicks(t *testing.T, observedAt time.Time, stepA, stepB, tickA, tickB string) string {
	t.Helper()
	observed := observedAt.UTC().Format(time.RFC3339)
	fee := func(venue string) string {
		return fmt.Sprintf(`{"venue": %q, "product": "linear_perp", "contract": "linear",
			"maker": "0.0002", "taker": "0.0005", "tier": "VIP0-modeled",
			"class": "hand_recorded", "observed_at": %q, "source": "test fixture"}`, venue, observed)
	}
	lot := func(venue, step, tick string) string {
		tickField := ""
		if tick != "" {
			tickField = fmt.Sprintf("\"tick\": %q,", tick)
		}
		return fmt.Sprintf(`{
			"venue": %q, "product": "linear_perp", "contract": "linear", "symbol": "LABUSDT",
			"base": "LAB", "step": %q, "min_qty": %q,
			"quote": "USDT", "min_notional": "5", %s
			"class": "hand_recorded", "observed_at": %q,
			"source": "test fixture mirroring docs/venue-profiles.md trading filters"
		}`, venue, step, step, tickField, observed)
	}
	doc := fmt.Sprintf(`{
		"fees": [%s, %s],
		"lots": [%s, %s],
		"hurdle": {"annual_rate": "0.0369", "access_cost": "0", "liquidity_haircut": "0",
			"counterparty_haircut": "0", "as_of": %q, "class": "hand_recorded",
			"observed_at": %q, "source": "test fixture"}
	}`, fee("binance"), fee("bybit"),
		lot("binance", stepA, tickA), lot("bybit", stepB, tickB),
		observed, observed)

	path := filepath.Join(t.TempDir(), "reference.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write reference fixture: %v", err)
	}
	return path
}

// pinnedPortfolioMarginAttestation is exercise's fixture, per side: §4.1's one
// check with no read path anywhere. Binance's posture consumes it and Bybit
// UTA's rule table has no Portfolio-Margin row at all, so the Bybit side pins
// NOTHING and a pin there is refused as a document nothing reads.
func pinnedPortfolioMarginAttestation(t *testing.T, account string) *reconcile.AttestationPinConfig {
	t.Helper()
	cred, err := transport.NewCredential("k", "s")
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
	return &reconcile.AttestationPinConfig{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

// binanceSide and bybitSide are the two §5.2 blocks, perp-only.
//
// Perp-only is the strategy's shape rather than a simplification: record §95's
// #2 is cross-venue PERP-PERP, so neither side declares a spot instrument, and
// with no spot leg there is no spot_settle and no spot baseline to pin.
func binanceSide(t *testing.T, journal string) reconcile.Config {
	t.Helper()
	return reconcile.Config{
		JournalPath:                journal,
		Account:                    "sub-1",
		PortfolioMarginAttestation: pinnedPortfolioMarginAttestation(t, "sub-1"),
		Venue: reconcile.VenueConfig{
			Venue:        ids.Venue("binance"),
			Dialect:      reconcile.DialectBinance,
			APIKeyRef:    ref(t, "env:ARBLAB_TEST_API_KEY"),
			APISecretRef: ref(t, "env:ARBLAB_TEST_API_SECRET"),
			SpendLockDir: t.TempDir(),
			Endpoints: []reconcile.EndpointConfig{
				{Product: ids.ProductLinearPerp, Scheme: "https", Host: "demo-fapi.binance.com"},
			},
			Timeout:    dur(t, "5s"),
			ReadLimit:  1 << 20,
			RecvWindow: dur(t, "5s"),
			Limits: []reconcile.LimitConfig{
				{Name: "ip_requests", Scope: "venue", Dimension: "requests", Interval: dur(t, "5s"),
					OpeningCeiling: 300, RoutineCeiling: 400, Capacity: 500},
				{Name: "weight_1m", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
				{Name: "orders_1m", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "orders",
					Interval: dur(t, "1m"), OpeningCeiling: 840, RoutineCeiling: 1080, Capacity: 1200},
			},
			ThrottlePause:    dur(t, "2m"),
			BanPause:         dur(t, "10m"),
			ClockOffset:      dur(t, "0s"),
			ClockUncertainty: dur(t, "250ms"),
		},
		Instruments: []reconcile.InstrumentConfig{
			{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
				Symbol: "LABUSDT", Base: "LAB", Quote: "USDT", Leverage: 5},
		},
		PositionScopes: []reconcile.ScopeConfig{
			{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"},
		},
		Resolution: reconcile.ResolutionConfig{
			MaxLanding:  dur(t, "30s"),
			SweepMargin: dur(t, "5s"),
		},
		WatchdogSilence: dur(t, "30s"),
		StopBudget:      dur(t, "2s"),
	}
}

func bybitSide(t *testing.T, journal string) reconcile.Config {
	t.Helper()
	cfg := binanceSide(t, journal)
	cfg.Venue.Venue = ids.Venue("bybit")
	cfg.Venue.Dialect = reconcile.DialectBybit
	// TWO endpoints on a perp-only deployment, and the extra one is a real
	// venue asymmetry rather than a spare: Bybit UTA's §4.1 posture read rides
	// the SPOT family (the account surface is one unified endpoint), so a
	// document declaring no spot route is refused with ErrFamilyUnrouted even
	// though nothing here trades spot. Binance's posture rides the derivatives
	// family it already routes.
	cfg.Venue.Endpoints = []reconcile.EndpointConfig{
		{Product: ids.ProductSpot, Scheme: "https", Host: "api-demo.bybit.com"},
		{Product: ids.ProductLinearPerp, Scheme: "https", Host: "api-demo.bybit.com"},
	}
	cfg.Instruments = []reconcile.InstrumentConfig{
		{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
			Symbol: "LABUSDT", Base: "LAB", Quote: "USDT", Leverage: 5},
	}
	cfg.PositionScopes = []reconcile.ScopeConfig{
		{Venue: "bybit", Product: ids.ProductLinearPerp, Settle: "USDT"},
	}
	// Bybit meters REQUESTS PER SECOND PER ENDPOINT and publishes no
	// per-endpoint weight at all (profiles §5, "COST — Bybit publishes no
	// per-endpoint WEIGHT"; both order surfaces are 50/s). So this side does not
	// restate Binance's weight budget under a Bybit name — it declares the
	// request budget it really has and writes §7.6's waiver for the dimension
	// the venue does not publish. A copied weight_1m here would be a fixture
	// asserting an external fact that is false.
	cfg.Venue.Limits = []reconcile.LimitConfig{
		{Name: "ip_requests", Scope: "venue", Dimension: "requests", Interval: dur(t, "5s"),
			OpeningCeiling: 300, RoutineCeiling: 400, Capacity: 500},
		{Name: "orders_1s", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "orders",
			Interval: dur(t, "1s"), OpeningCeiling: 35, RoutineCeiling: 45, Capacity: 50},
		{Name: "spot_orders_1s", Scope: "family", Product: ids.ProductSpot, Dimension: "orders",
			Interval: dur(t, "1s"), OpeningCeiling: 14, RoutineCeiling: 18, Capacity: 20},
	}
	cfg.Venue.Uncounted = []reconcile.UncountedConfig{
		{Product: ids.ProductLinearPerp, Dimension: "weight",
			Reason: "Bybit publishes no per-endpoint weight; its budget is requests per second per endpoint (profiles §5)"},
		{Product: ids.ProductSpot, Dimension: "weight",
			Reason: "Bybit publishes no per-endpoint weight; its budget is requests per second per endpoint (profiles §5)"},
	}
	// Bybit UTA's rule table has no Portfolio-Margin row, so a pin here is
	// refused as a document nothing consumes.
	cfg.PortfolioMarginAttestation = nil
	return cfg
}

// validConfig is the document every refusal below bends exactly one field of.
//
// # The two journals sit in ONE directory, deliberately
//
// That makes the positive control double as the negative control for
// sameJournal: two distinct names under one parent must validate, so an
// identity check that answered "shared" unconditionally fails HERE rather than
// passing three alias tests that only ever assert a refusal.
//
// Leg 0's limit_price and unwind_limit_price DIFFER (0.0840 against 0.0860),
// because with the two equal "uses the limit price" and "uses the unwind limit
// price" are the same assertion.
func validConfig(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	return Config{
		Sides: []SideConfig{
			{OMS: binanceSide(t, filepath.Join(dir, "binance.db"))},
			{OMS: bybitSide(t, filepath.Join(dir, "bybit.db"))},
		},
		Pair: PairConfig{
			ID:             "018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f",
			Attempt:        0,
			Mode:           ModeSequential,
			Settle:         dur(t, "10ms"),
			SettleAttempts: 3,
			Legs: []LegConfig{
				{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
					Symbol: "LABUSDT", Side: "sell", Quantity: "100",
					LimitPrice: "0.0840", UnwindLimitPrice: "0.0860"},
				{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
					Symbol: "LABUSDT", Side: "buy", Quantity: "100",
					LimitPrice: "0.0860", UnwindLimitPrice: "0.0840"},
			},
		},
		Reference: ReferenceConfig{
			Path:   writeCrossVenueReference(t, timeAgo(t, time.Hour), "1", "0.1"),
			MaxAge: dur(t, "24h"),
		},
	}
}

// aliasJournalRelativeAndAbsolute points side B at side A's journal spelled
// relative to the working directory. The strings differ; the file does not.
func aliasJournalRelativeAndAbsolute(t *testing.T) func(*Config) {
	t.Helper()
	return func(c *Config) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		rel, err := filepath.Rel(wd, c.Sides[0].OMS.JournalPath)
		if err != nil {
			t.Fatalf("Rel: %v", err)
		}
		c.Sides[1].OMS.JournalPath = rel
	}
}

// aliasJournalThroughSymlink points the two sides at one directory under two
// names.
//
// It does NOT create the database files, and that is the load-bearing detail:
// a journal that already exists is the SECOND boot. On the first boot the file
// is absent, filepath.EvalSymlinks refuses a path whose final component is
// missing, and an identity function that resolved the whole path would fall
// back to the unresolved spelling — so the refusal would be dead in the only
// boot where the journal is created.
func aliasJournalThroughSymlink(t *testing.T) func(*Config) {
	t.Helper()
	return func(c *Config) {
		root := t.TempDir()
		real := filepath.Join(root, "real")
		if err := os.Mkdir(real, 0o700); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		c.Sides[0].OMS.JournalPath = filepath.Join(real, "journal.db")
		c.Sides[1].OMS.JournalPath = filepath.Join(link, "journal.db")
	}
}

// aliasJournalThroughHardLink is the one alias fixture that MUST create the
// files: a hard link to a nonexistent path cannot be made, and os.SameFile
// needs two FileInfos. Two names, no symlink anywhere, one inode — the case
// path canonicalization alone cannot see.
func aliasJournalThroughHardLink(t *testing.T) func(*Config) {
	t.Helper()
	return func(c *Config) {
		dir := t.TempDir()
		first := filepath.Join(dir, "first.db")
		if err := os.WriteFile(first, []byte("journal"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		second := filepath.Join(dir, "second.db")
		if err := os.Link(first, second); err != nil {
			t.Fatalf("Link: %v", err)
		}
		c.Sides[0].OMS.JournalPath = first
		c.Sides[1].OMS.JournalPath = second
	}
}

// TestValidConfigValidates is the positive control every refusal below needs.
// Without it a universally-objecting Validate reads as a perfect score.
func TestValidConfigValidates(t *testing.T) {
	t.Parallel()
	if err := validConfig(t).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// TestTwoDistinctJournalsInOneDirectoryValidate is sameJournal's own negative
// control, named rather than left implicit in the fixture: without it, an
// identity check that answered "shared" for every pair would pass all three
// alias cases below and be caught by nothing that says why.
func TestTwoDistinctJournalsInOneDirectoryValidate(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	if filepath.Dir(c.Sides[0].OMS.JournalPath) != filepath.Dir(c.Sides[1].OMS.JournalPath) {
		t.Fatal("the fixture's two journals are in different directories; this control tests nothing")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for two distinct names under one parent", err)
	}
}

func TestConfigRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		bend func(*Config)
		want error
	}{
		{"one side", func(c *Config) { c.Sides = c.Sides[:1] }, ErrSideCount},
		{"three sides", func(c *Config) { c.Sides = append(c.Sides, c.Sides[0]) }, ErrSideCount},
		{
			// D2's premise, as a refusal. Both legs mint the same
			// ClientOrderID; the venue is what tells them apart.
			name: "both sides on one venue",
			bend: func(c *Config) {
				c.Sides[1].OMS = binanceSide(t, c.Sides[1].OMS.JournalPath)
			},
			want: ErrSideVenueDuplicate,
		},
		{
			// The second half of the same premise: with one journal the two
			// identical ids DO meet, and §5.1's store is single-writer.
			name: "both sides on one journal",
			bend: func(c *Config) { c.Sides[1].OMS.JournalPath = c.Sides[0].OMS.JournalPath },
			want: ErrSideJournalShared,
		},
		{
			// A raw string comparison misses the same path spelled once relative
			// and once absolute; both modules would write one physical database.
			name: "relative and absolute aliases of one journal",
			bend: aliasJournalRelativeAndAbsolute(t),
			want: ErrSideJournalShared,
		},
		{
			// EvalSymlinks is what makes a different parent spelling stop being a
			// different journal identity.
			name: "symlink aliases of one journal",
			bend: aliasJournalThroughSymlink(t),
			want: ErrSideJournalShared,
		},
		{
			// Two existing names may resolve without symlinks and still be one
			// inode. os.SameFile is the check the spelling cannot supply.
			name: "hard-link aliases of one journal",
			bend: aliasJournalThroughHardLink(t),
			want: ErrSideJournalShared,
		},
		{"three legs", func(c *Config) { c.Pair.Legs = append(c.Pair.Legs, c.Pair.Legs[0]) }, ErrLegCount},
		{"one leg", func(c *Config) { c.Pair.Legs = c.Pair.Legs[:1] }, ErrLegCount},
		{
			name: "two legs on one side",
			bend: func(c *Config) { c.Pair.Legs[1].Venue = c.Pair.Legs[0].Venue },
			want: ErrLegCount,
		},
		{"leg on an undeclared venue", func(c *Config) { c.Pair.Legs[1].Venue = ids.Venue("kraken") }, ErrLegVenueUnknown},
		{"leg on an undeclared instrument", func(c *Config) { c.Pair.Legs[1].Symbol = "DOGEUSDT" }, ErrLegInstrumentUnknown},
		{"both legs buy", func(c *Config) { c.Pair.Legs[0].Side = "buy"; c.Pair.Legs[1].Side = "buy" }, ErrLegSideSame},
		{"a side that is neither buy nor sell", func(c *Config) { c.Pair.Legs[0].Side = "short" }, ErrPairFieldInvalid},
		{"a quantity that does not parse", func(c *Config) { c.Pair.Legs[0].Quantity = "all of it" }, ErrPairFieldInvalid},
		{"a limit price that does not parse", func(c *Config) { c.Pair.Legs[0].LimitPrice = "" }, ErrPairFieldInvalid},
		{"an unwind limit price that does not parse", func(c *Config) { c.Pair.Legs[0].UnwindLimitPrice = "cheap" }, ErrPairFieldInvalid},
		{"unset trade id", func(c *Config) { c.Pair.ID = "" }, ErrPairFieldUnset},
		{"malformed trade id", func(c *Config) { c.Pair.ID = "not-a-uuid" }, ErrPairFieldInvalid},
		{"unset mode", func(c *Config) { c.Pair.Mode = "" }, ErrPairFieldUnset},
		{"unknown mode", func(c *Config) { c.Pair.Mode = Mode("parallel-ish") }, ErrPairFieldInvalid},
		{"negative attempt", func(c *Config) { c.Pair.Attempt = -1 }, ErrPairFieldInvalid},
		{
			// D7: the unwind mints Attempt+1, and 99 is ids' own ceiling.
			name: "attempt at the id format's ceiling leaves no room to unwind",
			bend: func(c *Config) { c.Pair.Attempt = 99 },
			want: ErrPairFieldInvalid,
		},
		{
			// The rung BELOW the ceiling must be admitted, or "refuses 99" and
			// "refuses everything near the top" are the same check.
			name: "unset settle",
			bend: func(c *Config) { c.Pair.Settle = config.Duration{} },
			want: ErrPairFieldUnset,
		},
		{"zero settle attempts", func(c *Config) { c.Pair.SettleAttempts = 0 }, ErrPairFieldUnset},
		{
			// Fail-closed: if you cannot state the price at which you would
			// close a leg, you may not open it.
			name: "leg with no unwind limit price",
			bend: func(c *Config) { c.Pair.Legs[0].UnwindLimitPrice = "" },
			want: ErrPairFieldUnset,
		},
		{"unset reference path", func(c *Config) { c.Reference.Path = "" }, ErrReferenceFieldUnset},
		{"unset reference max_age", func(c *Config) { c.Reference.MaxAge = config.Duration{} }, ErrReferenceFieldUnset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := validConfig(t)
			tc.bend(&c)
			err := c.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}

	// Each side's OWN block still validates through the mount — one probe,
	// because the full matrix is reconcile's suite.
	t.Run("a side's OMS block still validates", func(t *testing.T) {
		t.Parallel()
		c := validConfig(t)
		c.Sides[1].OMS.Account = ""
		if err := c.Validate(); err == nil {
			t.Error("Validate() = nil with an unset account on side B — the mount must not shadow the block's checks")
		}
	})
}

// TestTheAttemptBelowTheCeilingIsAdmitted is the boundary's other side. With
// only the 99 case, "refuses the ceiling" and "refuses anything large" are the
// same predicate, and a check written `>= 98` would pass.
func TestTheAttemptBelowTheCeilingIsAdmitted(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Pair.Attempt = 98
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() with attempt 98 = %v, want nil — 98 leaves room for the unwind's 99", err)
	}
}

// TestLegBaseMismatchIsRefused needs its own fixture rather than a bend,
// because it takes a second instrument row with a different base asset.
func TestLegBaseMismatchIsRefused(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	// Give side B an ETHUSDT row and point its leg at it: same quote, DIFFERENT
	// base, so the pair's quantity comparison would be an asset-tag mismatch at
	// runtime rather than a named config refusal.
	c.Sides[1].OMS.Instruments = append(c.Sides[1].OMS.Instruments, reconcile.InstrumentConfig{
		Venue: c.Sides[1].OMS.Venue.Venue, Product: ids.ProductLinearPerp,
		Contract: ids.ContractLinear, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Leverage: 5,
	})
	c.Pair.Legs[1].Symbol = "ETHUSDT"
	if err := c.Validate(); !errors.Is(err, ErrLegBaseMismatch) {
		t.Fatalf("Validate() = %v, want %v", err, ErrLegBaseMismatch)
	}
}

// TestOpenPlanUsesTheGivenQuantityAndNotTheDocument holds D5: leg B is sized to
// leg A's established fill. A plan that read the document's quantity would be
// right for leg A and would hedge a partial fill at full size on leg B.
func TestOpenPlanUsesTheGivenQuantityAndNotTheDocument(t *testing.T) {
	t.Parallel()
	c := validConfig(t) // both legs state quantity "100"
	got, err := c.openPlan(1, qty(t, "60"))
	if err != nil {
		t.Fatalf("openPlan(1, 60) = %v, want nil", err)
	}
	if len(got.Legs) != 1 {
		t.Fatalf("openPlan produced %d legs, want 1", len(got.Legs))
	}
	eqQty(t, got.Legs[0].Quantity, qty(t, "60"))
	if got.Legs[0].ReduceOnly {
		t.Error("ReduceOnly = true on an opening leg, want false")
	}
	if got.Legs[0].Side != domain.SideBuy {
		t.Errorf("Side = %v, want %v — leg 1's own side", got.Legs[0].Side, domain.SideBuy)
	}
	if got.Attempt != c.Pair.Attempt {
		t.Errorf("Attempt = %d, want %d", got.Attempt, c.Pair.Attempt)
	}
	// The OPENING limit, not the unwind's — the two differ in the fixture.
	wantPrice, err := fixedpoint.ParsePrice("LAB", "USDT", c.Pair.Legs[1].LimitPrice)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	eqPrice(t, got.Legs[0].LimitPrice, wantPrice)
}

// TestUnwindPlanInvertsTheSideAndAdvancesTheAttempt holds D7. The three claims
// are separated deliberately: a plan that got the side right and the
// reduce-only flag wrong opens fresh exposure while looking like a close.
func TestUnwindPlanInvertsTheSideAndAdvancesTheAttempt(t *testing.T) {
	t.Parallel()
	c := validConfig(t) // leg 0 is a SELL at limit_price, unwind at unwind_limit_price
	got, err := c.unwindPlan(0, qty(t, "60"))
	if err != nil {
		t.Fatalf("unwindPlan(0, 60) = %v, want nil", err)
	}
	if len(got.Legs) != 1 {
		t.Fatalf("unwindPlan produced %d legs, want 1", len(got.Legs))
	}
	leg := got.Legs[0]
	if leg.Side != domain.SideBuy {
		t.Errorf("Side = %v, want %v (the opposite of the opening leg's sell)", leg.Side, domain.SideBuy)
	}
	if !leg.ReduceOnly {
		t.Error("ReduceOnly = false, want true — an unwind that is not reduce-only can open a position")
	}
	// The established fill of 60, not the document's 100.
	eqQty(t, leg.Quantity, qty(t, "60"))
	if got.Attempt != c.Pair.Attempt+1 {
		t.Errorf("Attempt = %d, want %d — §5.3 forbids reusing the opening leg's id",
			got.Attempt, c.Pair.Attempt+1)
	}
	wantPrice, err := fixedpoint.ParsePrice("LAB", "USDT", c.Pair.Legs[0].UnwindLimitPrice)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	// The leg's unwind_limit_price, which the fixture makes DIFFER from its
	// limit_price — with the two equal, "uses the limit" and "uses the unwind
	// limit" are the same assertion.
	eqPrice(t, leg.LimitPrice, wantPrice)
	// And the instrument is still the leg's own: an unwind that inverted the
	// side and the VENUE would close nothing and open a mirror position.
	if leg.Instrument.Venue != c.Pair.Legs[0].Venue {
		t.Errorf("Instrument.Venue = %s, want %s", leg.Instrument.Venue, c.Pair.Legs[0].Venue)
	}
}
