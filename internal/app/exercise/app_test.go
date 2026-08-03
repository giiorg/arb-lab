package exercise

import (
	"errors"
	"io"
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
)

// validConfig mirrors reconcile's own fixture — the embedded half is §5.2's
// block unchanged, and the trade block is the two-leg cash-and-carry §9's
// cycle drives. Every refusal test below mutates ONE field of it.
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
	}
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
				{Name: "spot_weight", Scope: "family", Product: ids.ProductSpot, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 4200, RoutineCeiling: 5400, Capacity: 6000},
				{Name: "spot_orders", Scope: "family", Product: ids.ProductSpot, Dimension: "orders",
					Interval: dur(t, "10s"), OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100},
				{Name: "perp_weight", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "weight",
					Interval: dur(t, "1m"), OpeningCeiling: 1680, RoutineCeiling: 2160, Capacity: 2400},
				{Name: "perp_orders", Scope: "family", Product: ids.ProductLinearPerp, Dimension: "orders",
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

// submittedLeg is a clean leg. Its Ack is left zero deliberately: naming
// ports.Ack here would need the one import singlewriter bars this package
// from, and the gate is right — the exit class reads no ack field.
func submittedLeg(id ids.ClientOrderID) application.LegResult {
	return application.LegResult{
		ID: id,
		Outcome: application.Outcome{
			State: domain.StateSubmitted, Journaled: true, Applied: true,
		},
	}
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
