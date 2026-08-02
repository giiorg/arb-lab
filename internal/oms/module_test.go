package oms_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
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

var _ driven.VenueOrderPlacer = (*countingPlacer)(nil)

const venueOrderID = "V-77"

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
	return oms.Deps{
		JournalPath: journalPath(t),
		VenueOrders: &countingPlacer{ack: venue.OrderAck{VenueOrderID: venueOrderID}},
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
	out, err := m.Dispatch.Dispatch(perp.Order.ClientOrderID, perp.Version, ports.UrgencyOpening)
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
	// intents from §6 step 5, then this leg's authorization and result from §6
	// step 6. The sequence is what says the two use cases share a journal — a
	// module with two would leave two records here, not four.
	records := readJournal(t, deps.JournalPath)
	want := []journalRow{
		{kind: "planned", clientID: string(planned[0].Order.ClientOrderID)},
		{kind: "planned", clientID: string(perp.Order.ClientOrderID)},
		{kind: "state", clientID: string(perp.Order.ClientOrderID), state: string(domain.StateDispatchAuthorized)},
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

	if _, err := m.Dispatch.Dispatch(id, 1, ports.UrgencyOpening); !errors.Is(err, ports.ErrOrderUnknown) {
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
