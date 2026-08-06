package exercise

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// The resolver tests drive time with a TestClock at an ABSOLUTE instant, so
// every age below is an exact arithmetic fact rather than a race against the
// suite's own runtime — which is what lets the max-age boundary be tested AT
// the bound, where `>` and `>=` are different predicates. filtersBase is
// minutes off the hour so no age lands on a value that "looks round" for the
// wrong reason.
var filtersBase = time.Date(2026, 8, 3, 14, 37, 0, 0, time.UTC)

func filtersClock(t *testing.T) *clock.TestClock {
	t.Helper()
	clk, err := clock.NewTestClock(filtersBase, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return clk
}

// filtersConfig is validConfig with the reference block re-pointed at a
// revision whose observed_at the test controls exactly.
func filtersConfig(t *testing.T, path string) Config {
	t.Helper()
	cfg := validConfig(t)
	cfg.Reference.Path = path
	return cfg
}

// TestTradingFiltersResolveTheCommittedShape is the resolver's positive
// control, member by member: both instruments answer, and each of the eight
// values is pinned individually — the two rows differ in EVERY member, so a
// resolver that served one row's filters under the other's key, or swapped
// two members of one row, is distinguishable from a correct one.
func TestTradingFiltersResolveTheCommittedShape(t *testing.T) {
	t.Parallel()

	cfg := filtersConfig(t, writeReference(t, filtersBase.Add(-time.Hour)))
	filters, err := cfg.tradingFilters(filtersClock(t))
	if err != nil {
		t.Fatalf("tradingFilters: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("resolved %d instruments, want 2", len(filters))
	}

	spot := ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perp := ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear}

	s, ok := filters[spot]
	if !ok {
		t.Fatal("no spot entry")
	}
	if s.Step.Decimal() != "0.00001" || s.MinQty.Decimal() != "0.00001" ||
		s.Tick.Decimal() != "0.01" || s.MinNotional.Decimal() != "5" {
		t.Errorf("spot filters %s/%s/%s/%s, want 0.00001/0.00001/0.01/5",
			s.Step.Decimal(), s.MinQty.Decimal(), s.Tick.Decimal(), s.MinNotional.Decimal())
	}
	p, ok := filters[perp]
	if !ok {
		t.Fatal("no perp entry")
	}
	if p.Step.Decimal() != "0.001" || p.MinQty.Decimal() != "0.001" ||
		p.Tick.Decimal() != "0.1" || p.MinNotional.Decimal() != "50" {
		t.Errorf("perp filters %s/%s/%s/%s, want 0.001/0.001/0.1/50",
			p.Step.Decimal(), p.MinQty.Decimal(), p.Tick.Decimal(), p.MinNotional.Decimal())
	}

	// §69's carrier claim: the row's observation instant crosses into the OMS
	// vocabulary, because the executor re-judges the age at every package
	// admission and a map without dates cannot be asked. Asserted against the
	// WRITTEN instant — a dropped propagation would leave the zero value,
	// which Validate refuses later and farther from the cause.
	want := filtersBase.Add(-time.Hour)
	if !s.ObservedAt.Equal(want) || !p.ObservedAt.Equal(want) {
		t.Errorf("observed instants %s/%s, want both %s",
			s.ObservedAt, p.ObservedAt, want)
	}
}

func TestTradingFiltersRefuseAnUnreadableReference(t *testing.T) {
	t.Parallel()

	cfg := filtersConfig(t, filepath.Join(t.TempDir(), "absent.json"))
	_, err := cfg.tradingFilters(filtersClock(t))
	if !errors.Is(err, ErrReferenceUnreadable) {
		t.Fatalf("tradingFilters over a missing file = %v, want ErrReferenceUnreadable", err)
	}
	if !strings.Contains(err.Error(), "absent.json") {
		t.Errorf("refusal %q does not name the file", err)
	}
}

// TestTradingFiltersRefuseATicklessRow: the row is legal in the strategy
// domain (pre-tick revisions keep decoding) and refused by THIS consumer,
// which prices limit orders — the guard restated in terms of its consumer.
// The spot row keeps its tick, so the refusal is the perp ROW's and not a
// document-level parse failure.
func TestTradingFiltersRefuseATicklessRow(t *testing.T) {
	t.Parallel()

	path := writeReferenceTicks(t, filtersBase.Add(-time.Hour), "0.01", "")
	cfg := filtersConfig(t, path)
	_, err := cfg.tradingFilters(filtersClock(t))
	if !errors.Is(err, ErrFilterTickUnrecorded) {
		t.Fatalf("tradingFilters over a tick-less perp row = %v, want ErrFilterTickUnrecorded", err)
	}
	if !strings.Contains(err.Error(), "linear_perp") {
		t.Errorf("refusal %q does not name the tick-less row's instrument", err)
	}
}

// TestTradingFiltersFreshnessBound: the stale refusal, and the boundary from
// BOTH sides. A row aged exactly max_age is served — the check is `>`, the
// bound is the operator's declared tolerance inclusive — and one second past
// it refuses naming the age and the bound. Without the at-the-bound case,
// `>` and `>=` are the same predicate on every fixture.
func TestTradingFiltersFreshnessBound(t *testing.T) {
	t.Parallel()

	maxAge := 24 * time.Hour

	t.Run("a row aged exactly the bound is served", func(t *testing.T) {
		t.Parallel()
		cfg := filtersConfig(t, writeReference(t, filtersBase.Add(-maxAge)))
		if _, err := cfg.tradingFilters(filtersClock(t)); err != nil {
			t.Fatalf("tradingFilters at age == max_age = %v, want nil — the bound is inclusive", err)
		}
	})

	t.Run("a row one second past the bound is refused", func(t *testing.T) {
		t.Parallel()
		cfg := filtersConfig(t, writeReference(t, filtersBase.Add(-maxAge-time.Second)))
		_, err := cfg.tradingFilters(filtersClock(t))
		if !errors.Is(err, ErrFilterRowStale) {
			t.Fatalf("tradingFilters past max_age = %v, want ErrFilterRowStale", err)
		}
		if !strings.Contains(err.Error(), "24h0m0s bound") {
			t.Errorf("refusal %q does not name the operator's bound", err)
		}
	})

	t.Run("a plainly stale row is refused", func(t *testing.T) {
		t.Parallel()
		cfg := filtersConfig(t, writeReference(t, filtersBase.Add(-48*time.Hour)))
		if _, err := cfg.tradingFilters(filtersClock(t)); !errors.Is(err, ErrFilterRowStale) {
			t.Fatalf("tradingFilters over a 48h row = %v, want ErrFilterRowStale", err)
		}
	})
}

// TestRunRefusesAnOffGridTradeBeforeAnythingIsSent is the slice's harm case
// end to end: a configured quantity on the SPOT grid and off the PERP grid —
// the package that, sent, is one leg filled and one refused, i.e. naked
// exposure. The boot is REAL (fake venue, clean verdict), so this test also
// proves the filters actually thread exercise → WithInstrumentFilters →
// reconcile's boot → oms.Deps → the planner: a break anywhere in that chain
// surfaces here as ErrFiltersUnknown or as a plan that goes through.
func TestRunRefusesAnOffGridTradeBeforeAnythingIsSent(t *testing.T) {
	t.Setenv("ARBLAB_TEST_API_KEY", "k")
	t.Setenv("ARBLAB_TEST_API_SECRET", "s")

	var places int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case servePosture(w, r):
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/account":
			// The boot's spot assertion, satisfied by the pinned baseline —
			// the refusal under test must come from the PLANNER, not from a
			// boot that never got that far.
			fmt.Fprint(w, binanceAccountBody)
		case r.Method == http.MethodPost:
			places++
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-1000,"msg":"nothing may be placed in this test"}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path"}`)
		}
	}))
	defer srv.Close()

	cfg := validConfig(t)
	cfg.OMS.JournalPath = filepath.Join(t.TempDir(), "journal.db")
	cfg.OMS.SpotBaseline = pinnedCleanBaseline(t, cfg.OMS.Account, "k", "s")
	host := strings.TrimPrefix(srv.URL, "http://")
	for i := range cfg.OMS.Venue.Endpoints {
		cfg.OMS.Venue.Endpoints[i].Scheme = "http"
		cfg.OMS.Venue.Endpoints[i].Host = host
	}
	// 0.0015 BTC: on the spot grid (1e-5), OFF the perp grid (0.0015/0.001 =
	// 1.5), above both min quantities, and 74.85 USDT at the perp's limit —
	// above both notional floors. Only the perp lot check can object.
	for i := range cfg.Trade.Legs {
		cfg.Trade.Legs[i].Quantity = "0.0015"
	}

	snap := config.Snapshot[Config]{Value: cfg}
	wall, err := clock.NewWall(cfg.OMS.Calibrations())
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	report, err := Run(t.Context(), snap, wall, observability.NewRegistry(), io.Discard)
	if err != nil {
		t.Fatalf("Run = %v — a planning refusal is a report, not an error", err)
	}

	if report.Executed {
		t.Fatal("Executed = true for an off-grid package")
	}
	if !errors.Is(report.Refusal, application.ErrQuantityOffLotStep) {
		t.Fatalf("Refusal = %v, want ErrQuantityOffLotStep", report.Refusal)
	}
	// The rendering the §9 operator reads: the refusal names the quantity and
	// the grid it missed, so the fix (re-derive the size) needs no debugger.
	for _, part := range []string{"0.0015", "step", "0.001"} {
		if !strings.Contains(report.Refusal.Error(), part) {
			t.Errorf("Refusal %q does not carry %q", report.Refusal, part)
		}
	}
	if got := report.ExitClass(); got != ExitIncomplete {
		t.Errorf("ExitClass() = %s, want ExitIncomplete — nothing was sent, a resize may retry", got)
	}
	if places != 0 {
		t.Errorf("the venue saw %d Place calls for a refused plan, want 0", places)
	}
}
