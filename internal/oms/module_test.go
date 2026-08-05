package oms_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"

	_ "modernc.org/sqlite"
)

// # What this file claims, and how the claim changed when Deps did
//
// It used to assert that New connected the SUPPLIED ports, with a recording
// journal fake injected at Deps.Journal. Slice B converted that field to a path,
// so there is no journal to supply and that assertion cannot be written — §48g
// item 1 named this as the conversion's cost and left the replacement to be
// decided here.
//
// The replacement is stronger rather than equivalent, and it is worth saying
// which way: this file now composes the REAL journal adapter on a real file, and
// walks §6 steps 5 and 6 end to end — plan, journal, apply, dispatch — with only
// the venue faked. The claim it makes is the one no layer test can: that the two
// use cases share one repository and one journal, so that an order planned by
// the first is the order the second sends.
//
// The price is that this file reads driven's schema. It is paid deliberately:
// the alternative — asserting only that a file appeared at the configured path —
// would pass against a module that opened a journal and then dispatched through
// a different one, which is precisely the wiring mistake worth catching.
//
// application/dispatch_test.go still walks the classification table, the
// refusals and the double-dispatch backstop against port fakes; none of that is
// repeated here.

// countingPlacer is the one wire a root still supplies, and slice C2a moved it
// one layer OUT: Deps no longer takes a ports.OrderPlacer, it takes the venue
// client, and driven.OrderGateway is built inside New.
//
// That makes this fake stronger than the one it replaces. It records
// venue.OrderRequest rather than domain.Order, so what it observes is the far
// side of the ACL — and the claim that gets checked is the one a port-level fake
// could not see: that the module TRANSLATED, and translated correctly. A wiring
// that reached the venue with the planned order's identity but the wrong side or
// the wrong shed class would have satisfied the old fake completely.
//
// It counts because §5.3's rule is a count — "submit once, NO transport retry".
type countingPlacer struct {
	ack venue.OrderAck

	calls  int
	orders []venue.OrderRequest
}

func (p *countingPlacer) Place(_ context.Context, r venue.OrderRequest) (venue.OrderAck, error) {
	p.calls++
	p.orders = append(p.orders, r)
	return p.ack, nil
}

func (p *countingPlacer) ReservePlace([]venue.OrderRequest) (*ratelimit.Hold, ratelimit.Decision, error) {
	return nil, ratelimit.Decision{}, errors.New("module test: package reservation is not exercised")
}

var _ driven.VenueOrderPlacer = (*countingPlacer)(nil)

// silentReader is the venue's READ surface, wired so that New has something to
// build the resolver over.
//
// It refuses every call rather than answering, and the refusal is the honest
// fixture for this file: nothing here exercises §5.3's rule, whose behaviour
// belongs to application/resolve_test.go against port fakes. What this file
// claims about the read path is a WIRING claim — that New builds the resolver,
// over the same journal and repository as the dispatcher — and a reader that
// answered plausibly would invite assertions about the rule in the one file that
// cannot check it properly.
type silentReader struct{}

func (silentReader) OrderByClientID(context.Context, venue.OrderRef, venue.InstrumentAssets, ratelimit.Priority) (venue.OrderStatus, error) {
	return venue.OrderStatus{}, errors.New("module test: the venue read surface is not exercised here")
}

func (silentReader) Executions(context.Context, ids.VenueInstrument, venue.InstrumentAssets, venue.ExecutionWindow, ratelimit.Priority) (venue.ExecutionPage, error) {
	return venue.ExecutionPage{}, errors.New("module test: the venue read surface is not exercised here")
}

var _ driven.VenueOrderReader = (*silentReader)(nil)

// silentPositions is the venue's POSITION surface, refusing every call on
// silentReader's argument: what this file claims about the join is that New
// builds it over the same repository and clock, and the invariant's behaviour
// belongs to application/join_test.go against port fakes.
type silentPositions struct{}

func (silentPositions) Positions(context.Context, venue.PositionScope, map[ids.VenueInstrument]venue.InstrumentAssets, ratelimit.Priority) (venue.PositionReport, error) {
	return venue.PositionReport{}, errors.New("module test: the venue position surface is not exercised here")
}

var _ driven.VenuePositionReader = (*silentPositions)(nil)

const venueOrderID = "V-77"

// spotInstrument and perpInstrument are the two legs every fixture here uses,
// named once so that the asset table and the plan cannot disagree about which
// instruments exist.
func spotInstrument() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

func perpInstrument() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func mustQty(t *testing.T, v string) fixedpoint.Quantity {
	t.Helper()
	return mustAssetQty(t, "BTC", v)
}

func mustAssetQty(t *testing.T, asset, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %q): %v", asset, v, err)
	}
	return q
}

func mustPx(t *testing.T, v string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", v)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", v, err)
	}
	return p
}

func mustUSDT(t *testing.T, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", v)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", v, err)
	}
	return m
}

// journalPath returns a fresh path under the test's own directory. Fresh per
// test rather than shared, so a test that dispatches cannot be read as passing
// because of records another one wrote.
func journalPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "oms-journal.db")
}

// fullDeps is every dependency populated, which each construction case then
// breaks exactly one of.
func fullDeps(t *testing.T) oms.Deps {
	t.Helper()
	clk, err := clock.NewTestClock(
		time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
		[]clock.Calibration{{Venue: "binance", Offset: 250 * time.Millisecond, Uncertainty: 40 * time.Millisecond}},
	)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return oms.Deps{
		Account:         "sub-1",
		JournalPath:     journalPath(t),
		VenueOrders:     &countingPlacer{ack: venue.OrderAck{VenueOrderID: venueOrderID}},
		VenueOrderReads: silentReader{},
		VenuePositions:  silentPositions{},
		// The boot rung, stated as a root would state it (record §70f).
		RecoveryReadPriority: ratelimit.PrioritySafety,
		PositionScopes: []application.PositionScope{
			{Venue: "binance", Product: ids.ProductLinearPerp, Settle: "USDT"},
		},
		InstrumentAssets: map[ids.VenueInstrument]venue.InstrumentAssets{
			spotInstrument(): {Base: "BTC", Quote: "USDT"},
			perpInstrument(): {Base: "BTC", Quote: "USDT"},
		},
		InstrumentFilters: map[ids.VenueInstrument]domain.TradingFilters{
			// Grids deliberately off the powers of ten, and distinct per
			// instrument — planning_test.go's testFilters states the collapse
			// argument. The cashAndCarry fixture (0.5 BTC at 50000/50100)
			// conforms non-trivially to both; the observation instants sit
			// inside FilterMaxAge of the test clock's start.
			spotInstrument(): {
				Step:        mustQty(t, "0.025"),
				MinQty:      mustQty(t, "0.05"),
				Tick:        mustPx(t, "25"),
				MinNotional: mustUSDT(t, "5000"),
				ObservedAt:  time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC),
			},
			perpInstrument(): {
				Step:        mustQty(t, "0.05"),
				MinQty:      mustQty(t, "0.1"),
				Tick:        mustPx(t, "20"),
				MinNotional: mustUSDT(t, "10000"),
				ObservedAt:  time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC),
			},
		},
		FilterMaxAge: 24 * time.Hour,
		Clock:        clk,
		Resolution: application.ResolutionPolicy{
			MaxLanding:  30 * time.Second,
			SweepMargin: 5 * time.Second,
			AssumedReach: map[application.SurfaceKey]time.Duration{
				{Venue: "binance", Product: ids.ProductSpot}: 24 * time.Hour,
			},
		},
	}
}

// newModule builds the context and registers its Close, since New now acquires
// a file handle and a SQLite connection.
func newModule(t *testing.T, deps oms.Deps) *oms.Module {
	t.Helper()
	m, err := oms.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return m
}

func testTrade(t *testing.T) ids.TradeID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	return trade
}

// cashAndCarry is §2.2's package: buy spot, sell the perp. Two legs rather than
// one, because a single-leg plan makes "minted per leg" and "minted once"
// the same answer.
func cashAndCarry(t *testing.T) application.TradePlan {
	t.Helper()

	spotQty, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	spotPx, err := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	perpPx, err := fixedpoint.ParsePrice("BTC", "USDT", "50100")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}

	return application.TradePlan{
		Trade: testTrade(t),
		Legs: []application.LegIntent{
			{
				Instrument: ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
				},
				Side: domain.SideBuy, Quantity: spotQty, LimitPrice: spotPx,
			},
			{
				Instrument: ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductLinearPerp,
					Symbol: "BTCUSDT", Contract: ids.ContractLinear,
				},
				Side: domain.SideSell, Quantity: spotQty, LimitPrice: perpPx,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewRefusesEachMissingDependency(t *testing.T) {
	t.Parallel()

	// Each case breaks exactly ONE dependency and expects a DIFFERENT sentinel
	// — the one belonging to the constructor that owns that wire. module.go adds
	// no refusals of its own precisely so that these stay where they are, and
	// this is the assertion that says so: a New with its own blanket check would
	// answer one sentinel to both cases.
	//
	// This comment used to add "they come from two different packages, which is
	// the sharper form of the same claim", and slice C2a made that FALSE: the
	// second dependency moved from application.ErrPlacerMissing to
	// driven.ErrVenueOrderPlacerMissing when Deps.Placer became Deps.VenueOrders,
	// so both sentinels are now driven's. The distinctness that survives is
	// per-WIRE rather than per-package, and it is still the thing being checked.
	// Corrected rather than left, because a test comment claiming more than the
	// test does is read as evidence by everyone except the person who wrote it.
	cases := map[string]struct {
		mutate func(*oms.Deps)
		want   error
	}{
		"no journal path": {func(d *oms.Deps) { d.JournalPath = "" }, driven.ErrJournalPathUnset},
		"no venue client": {func(d *oms.Deps) { d.VenueOrders = nil }, driven.ErrVenueOrderPlacerMissing},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			deps := fullDeps(t)
			tc.mutate(&deps)

			if _, err := oms.New(deps); !errors.Is(err, tc.want) {
				t.Fatalf("New = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestNewAssemblesACompleteContext is the positive control the cases above
// need: without it, a New that refused everything would score two out of two.
func TestNewAssemblesACompleteContext(t *testing.T) {
	t.Parallel()

	m := newModule(t, fullDeps(t))
	if m.Plan == nil {
		t.Error("Module.Plan is nil — New returned a context that cannot journal an intent")
	}
	if m.Dispatch == nil {
		t.Error("Module.Dispatch is nil — New returned a context with no crash boundary")
	}
}

// TestNewOpensTheJournalAtTheConfiguredPath is the claim that the path is
// configuration rather than decoration.
//
// A module that ignored the field and opened a journal of its own choosing would
// satisfy every other test in this file, because every other test reaches the
// journal through the module rather than through the filesystem.
func TestNewOpensTheJournalAtTheConfiguredPath(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	newModule(t, deps)

	// Opened through database/sql rather than os.Stat: an empty file at the path
	// would satisfy a stat, and what is claimed is that a SQLite journal with
	// this adapter's schema is there.
	db, err := sql.Open("sqlite", deps.JournalPath)
	if err != nil {
		t.Fatalf("open the configured path: %v", err)
	}
	defer func() { _ = db.Close() }()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM journal`).Scan(&count); err != nil {
		t.Fatalf("the configured path holds no journal table: %v", err)
	}
	if count != 0 {
		t.Errorf("a freshly opened journal holds %d records, want 0", count)
	}
}

// # One claim in this file's subject is NOT tested, and it is named rather than
// # papered over
//
// New now opens the journal before it has finished validating, so every refusal
// after the open joins journal.Close() into its error. That the handle is
// actually released is not portably assertable: a leaked *sql.DB shows up as a
// file that cannot be replaced on Windows and as a connection held for the
// process's life everywhere, and neither is observable from a Go test on this
// project's target. The half that IS observable — that the refusal reports the
// missing wire rather than a Close error — is what the placer case above
// asserts, and it exercises the join with a Close that succeeds.
//
// What would make the other half checkable is a Close that FAILS, and there is
// no portable way to produce one: sql.DB.Close is idempotent and answers nil. A
// test asserting only that the error string is non-empty was written and deleted
// rather than kept, because a check that passes against every implementation
// reads as coverage while measuring nothing.

// ---------------------------------------------------------------------------
// §6 steps 5 and 6 composed: plan, then dispatch what was planned
// ---------------------------------------------------------------------------

// TestPlanThenDispatchWritesOneJournalAndOneProjection is the claim this file
// exists for.
//
// Nothing below is checkable one layer down. application/planning_test.go proves
// the planner journals before it applies, and dispatch_test.go proves the
// dispatcher sends what it loaded — but both use fakes, so neither says whether
// New handed them the SAME journal and the SAME repository. A module that built
// a second repository for the dispatcher would pass both suites and produce an
// OMS where every dispatch reports ErrOrderUnknown; one that built a second
// journal would produce two files, each holding half the history §7.5 replays.
func TestPlanThenDispatchWritesOneJournalAndOneProjection(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	placer := deps.VenueOrders.(*countingPlacer)
	m := newModule(t, deps)

	planned, err := m.Plan.Plan(cashAndCarry(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("Plan returned %d records, want 2", len(planned))
	}

	// The perp leg. Index 1 rather than 0 so that "the leg asked for" and "the
	// first leg" are different records — a plan of one would make them the same.
	perp := planned[1]
	out, err := m.Dispatch.Dispatch(perp.Order.ClientOrderID, perp.Version, ports.UrgencyOpening, nil)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}

	if placer.calls != 1 {
		t.Errorf("the supplied placer saw %d calls, want exactly 1", placer.calls)
	}
	// What reached the venue is now a venue.OrderRequest, so this asserts the
	// TRANSLATION as well as the identity. The fields are spelled out rather
	// than compared against a call to the adapter's own converter: a check that
	// re-runs the translator to produce its expectation agrees with the
	// translator by construction, and would pass with every mapping reversed.
	//
	// The perp leg is a SELL at index 1, which is what makes the side assertion
	// able to fail — leg 0 is a buy, so a translator that answered Buy for
	// everything would be invisible against it.
	if len(placer.orders) == 1 {
		got := placer.orders[0]
		want := venue.OrderRequest{
			Instrument:    perp.Order.Instrument,
			ClientOrderID: perp.Order.ClientOrderID,
			Side:          venue.OrderSideSell,
			Quantity:      perp.Order.Quantity,
			LimitPrice:    perp.Order.LimitPrice,
			ReduceOnly:    false,
			Urgency:       ratelimit.PriorityOpening,
		}
		if got != want {
			t.Errorf("the venue received %+v, want %+v", got, want)
		}
	}
	// The venue's answer reaches the caller through the module. Non-zero on
	// purpose: against a zero Ack this assertion holds for a module that
	// discarded it.
	if out.Ack.VenueOrderID != venueOrderID {
		t.Errorf("Ack.VenueOrderID = %q, want %q — the venue's answer did not survive the composition",
			out.Ack.VenueOrderID, venueOrderID)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if !out.Journaled {
		t.Error("Journaled = false — the result never reached the journal New opened")
	}
	if !out.Applied {
		t.Error("Applied = false — the result never reached the repository Plan wrote to")
	}

	// ONE journal, holding both halves of the history in order: two planned
	// intents from §6 step 5, then this leg's authorization, ack and result
	// from §6 step 6. The sequence is what says the two use cases share a
	// journal — a module with two would leave two records here, not five —
	// and the acked record sits BEFORE the state record, which is Dispatch's
	// crash-window ordering asserted against the real store.
	records := readJournal(t, deps.JournalPath)
	want := []journalRow{
		{kind: "planned", clientID: string(planned[0].Order.ClientOrderID)},
		{kind: "planned", clientID: string(perp.Order.ClientOrderID)},
		// The crash boundary is its own record KIND, not a state record naming
		// StateDispatchAuthorized: the anchor §5.3 measures from travels with it,
		// and AppendState refuses that target outright.
		{kind: "authorized", clientID: string(perp.Order.ClientOrderID)},
		{kind: "acked", clientID: string(perp.Order.ClientOrderID)},
		{kind: "state", clientID: string(perp.Order.ClientOrderID), state: string(domain.StateSubmitted)},
	}
	if len(records) != len(want) {
		t.Fatalf("journal holds %+v, want %+v", records, want)
	}
	for i := range want {
		if records[i] != want[i] {
			t.Errorf("journal record %d = %+v, want %+v", i, records[i], want[i])
		}
	}
}

// TestExecuteComposesReservationThroughTheRealVenueStack protects the seams
// that no layer test can join: the driven adapter prices every leg through the
// codecs, one governor reserves the whole package, and each transport consumes
// its matching entry rather than acquiring again. Distinct families and bodies
// make omitting the last adapter leg observable; exact counters make a hidden
// second acquire observable.
func TestExecuteComposesReservationThroughTheRealVenueStack(t *testing.T) {
	const bybit ids.Venue = "bybit"

	var (
		mu         sync.Mutex
		categories []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Category    string `json:"category"`
			OrderLinkID string `json:"orderLinkId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		categories = append(categories, body.Category)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":0,"result":{"orderId":"77","orderLinkId":"` + body.OrderLinkID + `"}}`))
	}))
	t.Cleanup(srv.Close)

	clk, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{{
		Venue: clock.Venue(bybit), Offset: 0, Uncertainty: time.Millisecond,
	}})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	limits := []ratelimit.Limit{{
		Name: "ip_5s", Scope: ratelimit.ScopeVenue,
		Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
		OpeningCeiling: 8, RoutineCeiling: 9, Capacity: 10,
	}}
	for _, endpoint := range (transport.BybitLimitReader{}).Endpoints() {
		limits = append(limits, ratelimit.Limit{
			Name: endpoint.Name, Scope: ratelimit.ScopeEndpoint,
			Dimension: endpoint.Dimension, Interval: time.Second,
			OpeningCeiling: 8, RoutineCeiling: 9, Capacity: 10,
		})
	}
	gov, err := ratelimit.New(clk, ratelimit.Profile{
		Venue: bybit,
		Families: []ids.ProductCategory{
			ids.ProductSpot, ids.ProductLinearPerp,
		},
		Limits: limits,
		Uncounted: []ratelimit.Uncounted{
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionWeight, Reason: "Bybit publishes no per-endpoint weight"},
			{Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered by endpoint"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionWeight, Reason: "Bybit publishes no per-endpoint weight"},
			{Family: ids.ProductLinearPerp, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered by endpoint"},
		},
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	cred, err := transport.NewCredential("composed-key", "composed-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	signer, err := transport.NewBybitSigner(bybit, cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}
	spend, err := transport.AcquireSpendLock(t.TempDir(), signer.SpendDomain(), cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock: %v", err)
	}
	t.Cleanup(func() { _ = spend.Close() })
	signed, err := transport.NewSignedClient(transport.SignedClientConfig{
		Scheme: "http", Host: strings.TrimPrefix(srv.URL, "http://"),
		Timeout: time.Second, ReadLimit: 1 << 16, RecvWindow: 500 * time.Millisecond,
		Signer: signer, Limits: transport.BybitLimitReader{}, Envelope: transport.BybitEnvelope{},
		Governor: gov, Clock: clk, Spend: spend,
	})
	if err != nil {
		t.Fatalf("NewSignedClient: %v", err)
	}
	codec, err := venue.NewBybitOrderCodec(bybit)
	if err != nil {
		t.Fatalf("NewBybitOrderCodec: %v", err)
	}
	orders, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: codec, Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductSpot: signed, ids.ProductLinearPerp: signed,
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	spot := ids.VenueInstrument{Venue: bybit, Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	perp := ids.VenueInstrument{Venue: bybit, Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}
	deps := fullDeps(t)
	deps.Clock = clk
	deps.VenueOrders = orders
	deps.PositionScopes = []application.PositionScope{{Venue: bybit, Product: ids.ProductLinearPerp, Settle: "USDT"}}
	deps.InstrumentAssets = map[ids.VenueInstrument]venue.InstrumentAssets{
		spot: {Base: "BTC", Quote: "USDT"}, perp: {Base: "BTC", Quote: "USDT"},
	}
	// Observation instants anchor to THIS test's clock, not fullDeps' — the
	// two fixtures start years apart, and a date fresh against one is from
	// the future against the other.
	observed := time.UnixMilli(1700000000000).UTC()
	deps.InstrumentFilters = map[ids.VenueInstrument]domain.TradingFilters{
		spot: {
			Step: mustQty(t, "0.025"), MinQty: mustQty(t, "0.05"),
			Tick: mustPx(t, "25"), MinNotional: mustUSDT(t, "5000"),
			ObservedAt: observed.Add(-1 * time.Hour),
		},
		perp: {
			Step: mustQty(t, "0.05"), MinQty: mustQty(t, "0.1"),
			Tick: mustPx(t, "20"), MinNotional: mustUSDT(t, "10000"),
			ObservedAt: observed.Add(-2 * time.Hour),
		},
	}
	m := newModule(t, deps)
	plan := cashAndCarry(t)
	plan.Legs[0].Instrument = spot
	plan.Legs[1].Instrument = perp
	planned, err := m.Plan.Plan(plan)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	refs := make([]application.DispatchRef, len(planned))
	for i, record := range planned {
		refs[i] = application.DispatchRef{ID: record.Order.ClientOrderID, Version: record.Version}
	}
	checklist, err := boot.NewChecklist(boot.GateNetDeltaAsserted, boot.GateWriterAuthority)
	if err != nil {
		t.Fatalf("NewChecklist: %v", err)
	}
	if err := checklist.Report(boot.GateNetDeltaAsserted, boot.OutcomePassed); err != nil {
		t.Fatalf("Report: %v", err)
	}
	execution, err := m.Execute.Execute(checklist, refs, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(execution.Legs) != 2 {
		t.Fatalf("Execute returned %d legs, want 2", len(execution.Legs))
	}
	for i, leg := range execution.Legs {
		if leg.Err != nil {
			t.Errorf("leg %d: %v", i, leg.Err)
		}
	}
	mu.Lock()
	gotCategories := append([]string(nil), categories...)
	mu.Unlock()
	if len(gotCategories) != 2 {
		t.Fatalf("HTTP requests = %v, want exactly two", gotCategories)
	}
	seen := map[string]int{}
	for _, category := range gotCategories {
		seen[category]++
	}
	if seen["spot"] != 1 || seen["linear"] != 1 {
		t.Errorf("request categories = %v, want one spot and one linear", gotCategories)
	}

	for _, state := range gov.Snapshot().Limits {
		wantUsed := uint64(0)
		if state.Name == "ip_5s" || state.Name == transport.BybitOrderCreateLimit {
			wantUsed = 2
		}
		if state.Used != wantUsed {
			t.Errorf("limit %s Used = %d, want %d", state.Name, state.Used, wantUsed)
		}
		if state.Reserved != 0 {
			t.Errorf("limit %s Reserved = %d, want 0", state.Name, state.Reserved)
		}
	}
}

// ---------------------------------------------------------------------------
// §7.5's first half: a second process over the same journal
// ---------------------------------------------------------------------------

// TestASecondModuleRebuildsWhatTheFirstOneDid is the crash this journal exists
// for, expressed as the only thing a test can do about it: a module that plans
// and dispatches, a Close, and a SECOND module over the same file.
//
// # What no layer test can say
//
// application/replay_test.go proves the Replayer rebuilds a scripted log, and
// driven/journalreplay_internal_test.go proves the adapter walks a real one and
// verifies its chain. Neither says the module wired the replay into the SAME
// repository the dispatcher will read — and that wiring is the whole of §7.5's
// first half. A module that replayed into a second repository would pass both
// suites and produce a process that recovered its state into a store nothing
// consults.
//
// # The two orders end in different places, and each is a separate claim
//
// The perp leg was dispatched, so recovery must find it in flight: the venue may
// hold it, and §5.3 forbids resubmitting it. The spot leg was planned and never
// sent, which §5.3 makes "safe to replan or discard" — so it must still be
// dispatchable. A recovery that restored nothing would fail the first
// assertion; one that restored every order to a live state would fail the
// second. Together they say the projection holds what the log holds rather than
// merely being non-empty.
func TestASecondModuleRebuildsWhatTheFirstOneDid(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)

	// Built directly rather than through newModule: this test closes the first
	// module itself, and a registered cleanup would close it a second time.
	first, err := oms.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	planned, err := first.Plan.Plan(cashAndCarry(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	spot, perp := planned[0], planned[1]

	if _, err := first.Dispatch.Dispatch(perp.Order.ClientOrderID, perp.Version, ports.UrgencyOpening, nil); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	// The crash, as closely as a test can stage one: this process stops. The
	// journal is on disk and the projection was never anywhere else.
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A fresh module over the same path, with a fresh venue placer so a second
	// Place would be visible as a call on THIS counter rather than the first.
	next := deps
	next.VenueOrders = &countingPlacer{ack: venue.OrderAck{VenueOrderID: venueOrderID}}
	second, err := oms.New(next)
	if err != nil {
		t.Fatalf("New over the existing journal: %v", err)
	}
	defer func() { _ = second.Close() }()
	placer := next.VenueOrders.(*countingPlacer)

	got, err := second.Recover.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Two planned intents, the authorization, the ack, and the result of the
	// send.
	if got.Records != 5 || got.LastSequence != 5 {
		t.Errorf("Records/LastSequence = %d/%d, want 5/5", got.Records, got.LastSequence)
	}
	if got.Orders != 2 {
		t.Errorf("Orders = %d, want 2", got.Orders)
	}
	want := []ids.ClientOrderID{perp.Order.ClientOrderID}
	if len(got.Unresolved) != 1 || got.Unresolved[0] != want[0] {
		t.Errorf("Unresolved = %v, want %v — the dispatched leg is the one the "+
			"venue may hold, and the planned leg is not", got.Unresolved, want)
	}

	// §5.3: an order at DispatchAuthorized or beyond is resolved by query,
	// never resubmitted. The recovered record is at StateSubmitted, so
	// domain.CanTransition has no edge back to the boundary and the dispatch
	// path refuses — which is the durable half of the single-Place rule, since
	// the in-process backstop died with the first module.
	if _, err := second.Dispatch.Dispatch(perp.Order.ClientOrderID, 3, ports.UrgencyOpening, nil); err == nil {
		t.Error("the recovered in-flight order was dispatched a second time")
	}
	if placer.calls != 0 {
		t.Errorf("the second module placed %d orders, want 0", placer.calls)
	}

	// The positive control, and the other half of the recovery claim: the leg
	// that was only ever planned is still dispatchable, at the version the
	// replay rebuilt it to.
	if _, err := second.Dispatch.Dispatch(spot.Order.ClientOrderID, spot.Version, ports.UrgencyOpening, nil); err != nil {
		t.Errorf("Dispatch of the recovered PLANNED leg = %v, want nil — §5.3 makes "+
			"a Planned-only order safe to replan", err)
	}
	if placer.calls != 1 {
		t.Errorf("the second module placed %d orders after dispatching the planned leg, want 1", placer.calls)
	}
}

// TestDispatchRefusesAnOrderThisModuleNeverPlanned is §6 step 5's ordering as a
// property of the composition rather than of the dispatcher alone.
//
// The id is well-formed and OMS-minted — it is what Plan WOULD have produced for
// a trade nobody planned — so the refusal cannot be the minted-id check firing.
// What refuses it is the empty repository, which is the whole reason Dispatch
// loads rather than accepts.
func TestDispatchRefusesAnOrderThisModuleNeverPlanned(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	placer := deps.VenueOrders.(*countingPlacer)
	m := newModule(t, deps)

	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: testTrade(t), Leg: 1})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}

	if _, err := m.Dispatch.Dispatch(id, 1, ports.UrgencyOpening, nil); !errors.Is(err, ports.ErrOrderUnknown) {
		t.Fatalf("Dispatch of an unplanned order = %v, want ErrOrderUnknown", err)
	}
	if placer.calls != 0 {
		t.Errorf("Place called %d times for an order that was never journaled, want 0", placer.calls)
	}
	if got := readJournal(t, deps.JournalPath); len(got) != 0 {
		t.Errorf("journal holds %+v after a refused dispatch, want nothing", got)
	}
}

// TestCloseReleasesTheJournal asserts that Close is wired to something rather
// than being a method that returns nil.
func TestCloseReleasesTheJournal(t *testing.T) {
	t.Parallel()

	m, err := oms.New(fullDeps(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
	// A planning attempt after Close must fail, which is what says the journal
	// New opened is the journal Close closed. A Close that returned nil without
	// touching anything would leave this succeeding.
	if _, err := m.Plan.Plan(cashAndCarry(t)); err == nil {
		t.Error("Plan succeeded after Close — the journal was never released")
	}
}

// ---------------------------------------------------------------------------
// Reading the journal back
// ---------------------------------------------------------------------------

// journalRow is one record as this test reads it: enough to say WHICH record it
// is, and nothing more. state is empty on a planned row, which is the schema's
// own shape — the detail tables are disjoint precisely so no row carries a field
// with no meaning.
type journalRow struct {
	kind     string
	clientID string
	state    string
}

// readJournal opens the journal file directly and returns its records in
// sequence order.
//
// It knows driven's table names, and that coupling is the acknowledged price of
// this file's central claim — see the header. It deliberately does NOT
// re-implement the hash chain: driven's own internal test owns that oracle, and
// a second replayer here would be a second opinion about the chain rather than a
// check on the wiring.
func readJournal(t *testing.T, path string) []journalRow {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open journal %q: %v", path, err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`
		SELECT j.kind, j.client_id, coalesce(s.state, '')
		FROM journal j LEFT JOIN journal_state s ON s.seq = j.seq
		ORDER BY j.seq`)
	if err != nil {
		t.Fatalf("read journal %q: %v", path, err)
	}
	defer func() { _ = rows.Close() }()

	var out []journalRow
	for rows.Next() {
		var r journalRow
		if err := rows.Scan(&r.kind, &r.clientID, &r.state); err != nil {
			t.Fatalf("scan journal row: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate journal rows: %v", err)
	}
	return out
}

// TestAnAckConflictLatchesLiveAndRefusesTheNextBoot is the conflict design's
// COMPOSITION, against the real journal and the real repository — the two
// halves each layer test asserts alone, proved to meet: a venue handing two
// orders on one instrument the same orderId (the banked non-reuse fact
// failing, or an adapter mis-threading acks) must first latch the live
// process and then refuse the boot that would otherwise build a poisoned
// rule-out index from the journal it left behind. Neither half alone closes
// the false-absence path: the latch dies with the process, and the replay
// refusal never runs while it lives.
func TestAnAckConflictLatchesLiveAndRefusesTheNextBoot(t *testing.T) {
	t.Parallel()

	deps := fullDeps(t)
	m := newModule(t, deps)

	first, err := m.Plan.Plan(cashAndCarry(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	rePlan := cashAndCarry(t)
	rePlan.Attempt = 1
	second, err := m.Plan.Plan(rePlan)
	if err != nil {
		t.Fatalf("Plan attempt 1: %v", err)
	}

	// Both perp legs — the SAME instrument — and the counting placer answers
	// every Place with the same venue id, which is exactly the venue
	// misbehaviour the conflict design exists for.
	clean, err := m.Dispatch.Dispatch(first[1].Order.ClientOrderID, first[1].Version, ports.UrgencyOpening, nil)
	if err != nil {
		t.Fatalf("first Dispatch = %v, want nil", err)
	}
	if !clean.AckApplied || clean.AckConflict {
		t.Fatalf("first dispatch AckApplied/AckConflict = %v/%v, want true/false",
			clean.AckApplied, clean.AckConflict)
	}

	conflicted, err := m.Dispatch.Dispatch(second[1].Order.ClientOrderID, second[1].Version, ports.UrgencyOpening, nil)
	if err != nil {
		t.Fatalf("second Dispatch = %v, want nil — the send happened", err)
	}
	if !conflicted.AckConflict {
		t.Fatal("second dispatch AckConflict = false, want true")
	}
	if !conflicted.HoldsNewTrading() {
		t.Fatal("second dispatch HoldsNewTrading() = false — the live half of the design")
	}
	// The journal took BOTH acked records — it is a recorder, and its refusal
	// to compose them is deliberately the replay's.
	if !conflicted.AckJournaled {
		t.Fatal("second dispatch AckJournaled = false, want true")
	}

	// The crash half: a fresh process over this journal must refuse to boot,
	// because composing the log means picking one of two owners for the id —
	// and a boot that picked would hand the resolver the poisoned index.
	next := deps
	next.VenueOrders = &countingPlacer{ack: venue.OrderAck{VenueOrderID: venueOrderID}}
	reborn, err := oms.New(next)
	if err != nil {
		t.Fatalf("New over the existing journal: %v", err)
	}
	defer func() { _ = reborn.Close() }()

	if _, err := reborn.Recover.Rebuild(); !errors.Is(err, ports.ErrVenueOrderIDConflict) {
		t.Fatalf("Rebuild over a conflicted journal = %v, want ErrVenueOrderIDConflict", err)
	}
}
