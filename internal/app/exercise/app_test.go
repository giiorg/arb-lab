package exercise

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/observability"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// validConfig mirrors reconcile's own fixture — the embedded half is §5.2's
// block unchanged, and the trade block is the two-leg cash-and-carry §9's
// cycle drives. Every refusal test below mutates ONE field of it.
//
// The trade's 0.5 BTC at 50100/49900 conforms to the reference fixture's
// REAL BTCUSDT filters non-trivially: on both lot grids (1e-5 spot, 1e-3
// perp), on both tick grids (0.01 spot, 0.1 perp), above both notional
// floors (5 and 50 USDT).
func validConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		OMS: reconcileConfig(t),
		Trade: TradeConfig{
			ID:      "018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f",
			Attempt: 0,
			Legs: []LegConfig{
				{Product: ids.ProductSpot, Symbol: "BTCUSDT", Side: "buy",
					Quantity: "0.5", LimitPrice: "50100"},
				{Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "BTCUSDT",
					Side: "sell", Quantity: "0.5", LimitPrice: "49900"},
			},
		},
		Reference: ReferenceConfig{
			Path:   writeReference(t, time.Now().UTC().Add(-time.Hour)),
			MaxAge: dur(t, "24h"),
		},
	}
}

// writeReference commits a reference revision to the test's own directory:
// the four real BTCUSDT filter rows (profiles §5, 2026-08-03 probes) with
// observed_at supplied by the caller, so freshness is a RELATIVE fact the
// stale-row test can vary rather than a wall-clock bomb that starts failing
// max_age after the values were typed.
func writeReference(t *testing.T, observedAt time.Time) string {
	t.Helper()
	return writeReferenceTicks(t, observedAt, "0.01", "0.1")
}

// writeReferenceTicks is writeReference with each leg's tick in the caller's
// hands; "" OMITS the key — the pre-tick row shape the tick-less refusal test
// needs, which is not the same row as a declared "0" (the domain refuses that
// one at load).
func writeReferenceTicks(t *testing.T, observedAt time.Time, spotTick, perpTick string) string {
	t.Helper()
	observed := observedAt.Format(time.RFC3339)
	lot := func(product, contract, step, minQty, minNotional, tick string) string {
		tickField := ""
		if tick != "" {
			tickField = fmt.Sprintf("\"tick\": %q,", tick)
		}
		return fmt.Sprintf(`{
			"venue": "binance", "product": %q, "contract": %q, "symbol": "BTCUSDT",
			"base": "BTC", "step": %q, "min_qty": %q,
			"quote": "USDT", "min_notional": %q, %s
			"class": "hand_recorded", "observed_at": %q,
			"source": "test fixture mirroring docs/venue-profiles.md trading filters (2026-08-03)"
		}`, product, contract, step, minQty, minNotional, tickField, observed)
	}
	doc := fmt.Sprintf(`{
		"fees": [
			{"venue": "binance", "product": "spot", "contract": "", "maker": "0.001", "taker": "0.001",
			 "tier": "VIP0-modeled", "class": "hand_recorded", "observed_at": %q, "source": "test fixture"},
			{"venue": "binance", "product": "linear_perp", "contract": "linear", "maker": "0.0002", "taker": "0.0005",
			 "tier": "VIP0-modeled", "class": "hand_recorded", "observed_at": %q, "source": "test fixture"}
		],
		"lots": [%s, %s],
		"hurdle": {"annual_rate": "0.0369", "access_cost": "0", "liquidity_haircut": "0",
			"counterparty_haircut": "0", "as_of": %q, "class": "hand_recorded",
			"observed_at": %q, "source": "test fixture"}
	}`, observed, observed,
		lot("spot", "", "0.00001", "0.00001", "5", spotTick),
		lot("linear_perp", "linear", "0.001", "0.001", "50", perpTick),
		observed, observed)

	path := filepath.Join(t.TempDir(), "reference.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write reference fixture: %v", err)
	}
	return path
}

func reconcileConfig(t *testing.T) reconcile.Config {
	t.Helper()
	return reconcile.Config{
		JournalPath: "/var/lib/arblab/journal.db",
		Account:     "sub-1",
		Venue: reconcile.VenueConfig{
			Venue:        ids.Venue("binance"),
			Dialect:      reconcile.DialectBinance,
			APIKeyRef:    ref(t, "env:ARBLAB_TEST_API_KEY"),
			APISecretRef: ref(t, "env:ARBLAB_TEST_API_SECRET"),
			SpendLockDir: t.TempDir(),
			Endpoints: []reconcile.EndpointConfig{
				{Product: ids.ProductSpot, Scheme: "https", Host: "api.binance.com"},
				{Product: ids.ProductLinearPerp, Scheme: "https", Host: "fapi.binance.com"},
			},
			Timeout:    dur(t, "5s"),
			ReadLimit:  1 << 20,
			RecvWindow: dur(t, "5s"),
			Limits: []reconcile.LimitConfig{
				{Name: "ip_requests", Scope: "venue", Dimension: "requests", Interval: dur(t, "5s"),
					OpeningCeiling: 300, RoutineCeiling: 400, Capacity: 500},
				{Name: "weight_1m", Scope: "family", Product: ids.ProductSpot, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 4200, RoutineCeiling: 5400, Capacity: 6000},
				{Name: "orders_10s", Scope: "family", Product: ids.ProductSpot, Dimension: "orders",
					Interval: dur(t, "10s"), OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100},
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
			{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"},
			{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear,
				Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"},
		},
		PositionScopes: []reconcile.ScopeConfig{
			{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"},
		},
		Resolution: reconcile.ResolutionConfig{
			MaxLanding:  dur(t, "30s"),
			SweepMargin: dur(t, "5s"),
			AssumedReach: []reconcile.ReachConfig{
				{Venue: "binance", Product: ids.ProductSpot, Reach: dur(t, "72h")},
			},
		},
		// Required-when-spot (record §70g): the fixture trades a spot leg.
		SpotSettle:      dur(t, "2s"),
		WatchdogSilence: dur(t, "30s"),
		StopBudget:      dur(t, "2s"),
	}
}

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

// TestValidConfigValidates is the positive control every refusal below needs.
func TestValidConfigValidates(t *testing.T) {
	t.Parallel()
	if err := validConfig(t).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestConfigRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		edit func(*Config)
		want error
	}{
		{"an unset trade id", func(c *Config) { c.Trade.ID = "" }, ErrTradeFieldUnset},
		{"a malformed trade id", func(c *Config) { c.Trade.ID = "not-a-uuid" }, ErrTradeFieldInvalid},
		{"a negative attempt", func(c *Config) { c.Trade.Attempt = -1 }, ErrTradeFieldInvalid},
		{"no legs", func(c *Config) { c.Trade.Legs = nil }, ErrTradeFieldUnset},
		{"a leg on an undeclared instrument", func(c *Config) {
			c.Trade.Legs[0].Symbol = "ETHUSDT"
		}, ErrLegInstrumentUnknown},
		{"a side that is neither buy nor sell", func(c *Config) {
			c.Trade.Legs[1].Side = "short"
		}, ErrTradeFieldInvalid},
		{"a quantity that does not parse", func(c *Config) {
			c.Trade.Legs[0].Quantity = "half"
		}, ErrTradeFieldInvalid},
		{"a limit price that does not parse", func(c *Config) {
			c.Trade.Legs[0].LimitPrice = ""
		}, ErrTradeFieldInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig(t)
			tc.edit(&cfg)
			if err := cfg.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}

	// The embedded block's own refusals still bite through the embedding — one
	// probe, because the full matrix is reconcile's suite.
	t.Run("the embedded OMS block still validates", func(t *testing.T) {
		t.Parallel()
		cfg := validConfig(t)
		cfg.OMS.Account = ""
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() = nil with an unset account — the embed must not shadow the OMS block's checks")
		}
	})
}

// TestPlanStatesTheDocument: the translation is positional and typed — leg i of
// the config is leg i of the plan, sides parse, quantities carry the row's base
// asset.
func TestPlanStatesTheDocument(t *testing.T) {
	t.Parallel()

	plan, err := validConfig(t).plan()
	if err != nil {
		t.Fatalf("plan() = %v", err)
	}
	if got := plan.Trade.String(); got != "018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f" {
		t.Errorf("Trade = %s", got)
	}
	if len(plan.Legs) != 2 {
		t.Fatalf("len(Legs) = %d, want 2", len(plan.Legs))
	}
	if plan.Legs[0].Side != domain.SideBuy || plan.Legs[1].Side != domain.SideSell {
		t.Errorf("sides = %s/%s, want buy/sell", plan.Legs[0].Side, plan.Legs[1].Side)
	}
	if plan.Legs[0].Instrument.Product != ids.ProductSpot || plan.Legs[1].Instrument.Product != ids.ProductLinearPerp {
		t.Errorf("products = %s/%s", plan.Legs[0].Instrument.Product, plan.Legs[1].Instrument.Product)
	}
	if got := plan.Legs[0].Quantity.Asset(); got != "BTC" {
		t.Errorf("leg 0 quantity asset = %s, want BTC — the instrument row's base", got)
	}
}

// TestRunRefusesToExecuteOnAnUncleanBoot is §59d's gate, exercised through
// Run: a journal path whose directory does not exist fails #168's storage
// check, the checklist latches, and the execute step must decline with
// NOTHING planned and nothing sent. Without this test, a mutant deleting the
// verdict gate would dispatch through every unclean boot and only the chaos
// suite's clean paths would ever run.
func TestRunRefusesToExecuteOnAnUncleanBoot(t *testing.T) {
	t.Setenv("ARBLAB_TEST_API_KEY", "k")
	t.Setenv("ARBLAB_TEST_API_SECRET", "s")

	cfg := validConfig(t)
	cfg.OMS.JournalPath = filepath.Join(t.TempDir(), "no-such-dir", "journal.db")
	snap := config.Snapshot[Config]{Value: cfg}

	wall, err := clock.NewWall(cfg.OMS.Calibrations())
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	report, err := Run(t.Context(), snap, wall, observability.NewRegistry(), io.Discard)
	if err != nil {
		t.Fatalf("Run = %v — a refused boot is a report, not an error", err)
	}

	if report.Executed {
		t.Fatal("Executed = true on a latched boot — the §59d gate did not hold")
	}
	if !errors.Is(report.Refusal, ErrBootNotClean) {
		t.Errorf("Refusal = %v, want ErrBootNotClean", report.Refusal)
	}
	// The refusal must come from the VERDICT gate, not from the halted-module
	// backstop behind it — the two wrap the same sentinel, and a deleted
	// verdict gate would otherwise be masked by the guard documented as
	// unreachable.
	if report.Refusal != nil && !strings.Contains(report.Refusal.Error(), "boot verdict is") {
		t.Errorf("Refusal = %q, want the verdict gate's own wording", report.Refusal)
	}
	if got := report.ExitClass(); got != ExitOperator {
		t.Errorf("ExitClass() = %s, want ExitOperator — the storage refusal latches", got)
	}
}

// # ExitClass: boot verdict first, then the execute step's own reading

// cleanBoot is this deployment's real clean shape: the five producerless gates
// block, nothing latches. Using an EMPTY Blocking here would test a deployment
// that does not exist and would collapse "boot clean" into "checklist silent".
func cleanBoot() reconcile.Report {
	return reconcile.Report{
		Posture: boot.PostureReconcileOnly,
		Blocking: []boot.Gate{
			boot.GateAccountPosture, boot.GateDeactivationsResolved,
			boot.GateMarginOpsResolved, boot.GateWriterAuthority,
		},
	}
}

// submittedLeg is a clean leg whose venue supplied no id — the ack fields are
// zero because there was nothing to persist, which the exit class must read
// as clean rather than as two dropped writes.
func submittedLeg(id ids.ClientOrderID) application.LegResult {
	return application.LegResult{
		ID: id,
		Outcome: application.Outcome{
			State: domain.StateSubmitted, Journaled: true, Applied: true,
		},
	}
}

// ackedLeg is a clean leg whose venue supplied an id and whose ack writes both
// landed. The id is set by field ASSIGNMENT rather than a composite literal,
// because naming ports.Ack would need the one import singlewriter bars this
// package from — the gate is still right, and reading and writing a field of
// an imported struct's field needs no import at all.
func ackedLeg(id ids.ClientOrderID) application.LegResult {
	leg := submittedLeg(id)
	leg.Outcome.Ack.VenueOrderID = "774411"
	leg.Outcome.AckJournaled = true
	leg.Outcome.AckApplied = true
	return leg
}

func TestExitClassPrecedence(t *testing.T) {
	t.Parallel()

	legA := ids.ClientOrderID("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-00-00")
	legB := ids.ClientOrderID("ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-01-00")

	cases := []struct {
		name   string
		report Report
		want   ExitClass
	}{
		{
			"a latched boot is the operator's whatever else happened",
			Report{Boot: reconcile.Report{Latched: []boot.Gate{boot.GateNetDeltaAsserted},
				Blocking: []boot.Gate{boot.GateNetDeltaAsserted}}},
			ExitOperator,
		},
		{
			// §7.6 through the boot's own precedence: a banned boot is the
			// operator's, and the execute gate reading the same class is what
			// refuses to open exposure through a ban.
			"a banned boot is the operator's — §7.6 outranks the execute step",
			func() Report {
				r := Report{Boot: cleanBoot()}
				r.Boot.RateLimit = ratelimit.Snapshot{Venue: ids.Venue("binance"), Bans: 1}
				return r
			}(),
			ExitOperator,
		},
		{
			"a boot that could not finish is incomplete",
			Report{Boot: reconcile.Report{Blocking: []boot.Gate{boot.GateJournalReplayed}}},
			ExitIncomplete,
		},
		{
			"a clean boot whose execute step refused is incomplete — nothing was sent",
			Report{Boot: cleanBoot(), Executed: false, Refusal: ErrBootNotClean},
			ExitIncomplete,
		},
		{
			"a clean boot and a clean package is clean",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{submittedLeg(legA), submittedLeg(legB)},
			}},
			ExitClean,
		},
		{
			"a leg refused inside the fan-out is an execution incident",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{submittedLeg(legA),
					{ID: legB, Err: application.ErrOrderAlreadyDispatched}},
			}},
			ExitExecution,
		},
		{
			"a rejected leg is an execution incident",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{submittedLeg(legA),
					{ID: legB, Outcome: application.Outcome{State: domain.StateRejected,
						Journaled: true, Applied: true}},
				},
			}},
			ExitExecution,
		},
		{
			"a sent leg whose apply failed is an execution incident — §6 step 7's hold",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{submittedLeg(legA),
					{ID: legB, Outcome: application.Outcome{State: domain.StateSubmitted,
						Journaled: true, Applied: false}},
				},
			}},
			ExitExecution,
		},
		{
			// The positive control for the ack predicate: both writes landed,
			// so a run that persisted its ids cleanly still exits 0. Without
			// this case the two below would pass against a predicate that
			// fails every leg whose venue said anything at all.
			"a clean acked package is clean",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{ackedLeg(legA), ackedLeg(legB)},
			}},
			ExitClean,
		},
		{
			"a dropped ack journal write is an execution incident",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{ackedLeg(legA),
					func() application.LegResult {
						leg := ackedLeg(legB)
						leg.Outcome.AckJournaled = false
						return leg
					}(),
				},
			}},
			ExitExecution,
		},
		{
			"a dropped ack apply is an execution incident",
			Report{Boot: cleanBoot(), Executed: true, Execution: application.Execution{
				Legs: []application.LegResult{ackedLeg(legA),
					func() application.LegResult {
						leg := ackedLeg(legB)
						leg.Outcome.AckApplied = false
						return leg
					}(),
				},
			}},
			ExitExecution,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.report.ExitClass(); got != tc.want {
				t.Errorf("ExitClass() = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestABanDuringDispatchIsSurfaced holds Report.RateLimit's doc to its fixture,
// both halves:
//
//   - the claim that excuses ExitClass from consulting the field — a ban
//     during the fan-out IS a leg below StateSubmitted, because the 418 that
//     files the penalty classifies that leg's Place to SubmissionUnknown, so
//     the class must come out ExitExecution with no new branch anywhere;
//   - the claim that requires the field to exist at all — Boot.RateLimit is
//     the pre-execute snapshot, so the dispatch's own ban is visible ONLY in
//     the post-execute read.
//
// The fake forces the ordering: the perp Place answers 418 only after the spot
// Place has completed, so the spot leg's Acquire deterministically precedes
// the pause and exactly one leg fails, by the venue's refusal rather than by a
// race with the governor.
func TestABanDuringDispatchIsSurfaced(t *testing.T) {
	t.Skip("awaits #179's baseline document path (record §70): the spot leg now correctly holds GateNetDeltaAsserted at UnassertedBaselineUnrecorded until a baseline is pinned; unskip when this fixture can load one")
	t.Setenv("ARBLAB_TEST_API_KEY", "k")
	t.Setenv("ARBLAB_TEST_API_SECRET", "s")

	spotDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/order":
			fmt.Fprintf(w, `{"orderId":22542179,"clientOrderId":%q,"status":"NEW"}`,
				r.Form.Get("newClientOrderId"))
			close(spotDone)
		case r.Method == http.MethodPost && r.URL.Path == "/fapi/v1/order":
			select {
			case <-spotDone:
			case <-time.After(10 * time.Second):
				// The bound §59f requires: if the spot Place never arrives the
				// 418 still answers, and the spot-leg assertions below name
				// the ordering failure instead of the suite hanging.
			}
			w.WriteHeader(http.StatusTeapot)
			fmt.Fprint(w, `{"code":-1003,"msg":"Way too much request weight used; IP banned."}`)
		case r.Method == http.MethodGet && r.URL.Path == "/fapi/v3/positionRisk":
			fmt.Fprint(w, `[]`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in ban fake"}`)
		}
	}))
	defer srv.Close()

	cfg := validConfig(t)
	cfg.OMS.JournalPath = filepath.Join(t.TempDir(), "journal.db")
	host := strings.TrimPrefix(srv.URL, "http://")
	for i := range cfg.OMS.Venue.Endpoints {
		cfg.OMS.Venue.Endpoints[i].Scheme = "http"
		cfg.OMS.Venue.Endpoints[i].Host = host
	}
	snap := config.Snapshot[Config]{Value: cfg}
	wall, err := clock.NewWall(cfg.OMS.Calibrations())
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}

	report, err := Run(t.Context(), snap, wall, observability.NewRegistry(), io.Discard)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.Executed {
		t.Fatalf("Executed = false (refusal %v) — the boot should be clean, the ban comes later", report.Refusal)
	}
	if report.Boot.RateLimit.Bans != 0 {
		t.Errorf("Boot.RateLimit.Bans = %d, want 0 — the pre-execute snapshot cannot carry the dispatch's ban",
			report.Boot.RateLimit.Bans)
	}
	if report.RateLimit.Bans != 1 {
		t.Errorf("RateLimit.Bans = %d, want 1 — the post-execute read is where the ban lives", report.RateLimit.Bans)
	}
	if !report.RateLimit.Paused || report.RateLimit.PauseKind != ratelimit.PenaltyBan {
		t.Errorf("pause = (%v, %s), want an in-force PenaltyBan pause",
			report.RateLimit.Paused, report.RateLimit.PauseKind)
	}
	if got := report.ExitClass(); got != ExitExecution {
		t.Errorf("ExitClass() = %s, want ExitExecution — the 418 errored a dispatched leg", got)
	}
	// Exactly one leg is below StateSubmitted and it carries the venue's own
	// cause: the doc's claim checked per leg rather than believed in
	// aggregate. SubmissionUnknown, not Rejected — a RateLimited Place is
	// EffectNotApplied WITHOUT the explicit-rejection evidence class FLAG-2
	// demands for the terminal, so §5.3's resolving query still owns it.
	var banned, clean int
	for _, leg := range report.Execution.Legs {
		if leg.Err != nil {
			t.Errorf("leg %s: Err = %v — a Place that RAN reports through its Outcome", leg.ID, leg.Err)
			continue
		}
		switch leg.Outcome.State {
		case domain.StateSubmitted:
			clean++
		case domain.StateSubmissionUnknown:
			banned++
			if leg.Outcome.Cause == nil {
				t.Errorf("leg %s: SubmissionUnknown with a nil Cause — the 418 must travel with the leg", leg.ID)
			}
		case domain.StatePlanned, domain.StateDispatchAuthorized, domain.StateAcknowledged,
			domain.StatePartiallyFilled, domain.StateFilled, domain.StateCancelPending,
			domain.StateCancelUnknown, domain.StateCanceled, domain.StateRejected,
			domain.StateAbsentAfterFinality:
			t.Errorf("leg %s: state = %s, want StateSubmitted or StateSubmissionUnknown", leg.ID, leg.Outcome.State)
		}
	}
	if banned != 1 || clean != 1 {
		t.Errorf("legs = %d banned / %d clean, want exactly 1 / 1 — the fake's ordering did not hold", banned, clean)
	}
}
