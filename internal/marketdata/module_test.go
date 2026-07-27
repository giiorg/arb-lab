package marketdata_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driving"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const (
	venue      = ids.Venue("binance")
	otherVenue = ids.Venue("bybit")
)

var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

// The venue's clock runs AHEAD of ours, and by enough that the two freshness
// verdicts land on different rungs of the ladder below. That is the point of
// the fixture: with a zero skew, transport and data freshness are the same
// measurement and this composition could be wired wrongly without failing.
const venueSkew = 90 * time.Second

// recvAt is 60s old at base, and sourceAt is chosen so DataAge is 30s. Under
// the narrow budget those are opposite verdicts — Transport stale, Data fresh —
// which is exactly the pair §3.4's singular "max-age budget" cannot express.
var (
	recvAt   = base.Add(-60 * time.Second)
	sourceAt = base.Add(60 * time.Second) // ServerNow is base+90s, so DataAge = 30s
)

var (
	narrow = domain.Budget{MaxAge: 30 * time.Second, Blind: 5 * time.Minute}
	wide   = domain.Budget{MaxAge: 2 * time.Minute, Blind: 20 * time.Minute}
)

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    venue,
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func document(t *testing.T, venues map[ids.Venue]domain.Budget) []byte {
	t.Helper()
	funding := make(map[ids.Venue]driven.BudgetSpec, len(venues))
	for v, b := range venues {
		funding[v] = driven.BudgetSpec{
			MaxAge: config.NewDuration(b.MaxAge),
			Blind:  config.NewDuration(b.Blind),
		}
	}
	doc, err := json.Marshal(driven.FreshnessConfig{Funding: funding})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return doc
}

func newStore(t *testing.T, venues map[ids.Venue]domain.Budget) *config.Store[driven.FreshnessConfig] {
	t.Helper()
	store, err := config.New[driven.FreshnessConfig]("test", base, document(t, venues))
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	return store
}

func newLane(t *testing.T) *events.MarketDataLane {
	t.Helper()
	// NoCoalescing so that two publishes are two observable envelopes. A
	// coalescing policy is the composition root's choice, not this context's,
	// and asserting through one would be asserting on somebody else's rule.
	lane, err := events.NewMarketDataLane(4, events.NoCoalescing)
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	return lane
}

func newClock(t *testing.T) *clock.TestClock {
	t.Helper()
	c, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

func update(t *testing.T) driving.FundingUpdate {
	t.Helper()
	return driving.FundingUpdate{
		Rate:       rate(t, "0.00013"),
		Interval:   8 * time.Hour,
		NextSettle: base.Add(4 * time.Hour),
		Cap:        rate(t, "0.0075"),
		Floor:      rate(t, "-0.0075"),
	}
}

func envelope(t *testing.T, inst ids.VenueInstrument, seq uint64) events.Envelope {
	t.Helper()
	return events.Envelope{
		AcceptOrdinal: seq + 6, // distinct from both other sequence numbers
		IngressSeq:    seq + 17,
		Source:        events.SourceID{Instrument: inst, Stream: events.StreamFunding},
		SourceTime:    sourceAt,
		RecvTime:      recvAt,
		VenueSeq:      seq,
		Prio:          events.PrioNormal,
		Payload:       update(t),
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewRefusesEachMissingDependency(t *testing.T) {
	// Each case omits exactly ONE dependency, and each expects a DIFFERENT
	// sentinel — the one belonging to the constructor that owns that wire.
	// Both halves matter: one omission at a time catches a New that checks only
	// the first, and distinct sentinels catch a New that invented its own
	// blanket refusal instead of letting each constructor speak.
	full := marketdata.Deps{Clock: newClock(t), Config: newStore(t, map[ids.Venue]domain.Budget{venue: narrow}), Lane: newLane(t)}

	cases := map[string]struct {
		mutate func(*marketdata.Deps)
		want   error
	}{
		"no clock":  {func(d *marketdata.Deps) { d.Clock = nil }, application.ErrClockNil},
		"no config": {func(d *marketdata.Deps) { d.Config = nil }, driven.ErrStoreNil},
		"no lane":   {func(d *marketdata.Deps) { d.Lane = nil }, driven.ErrLaneNil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deps := full
			tc.mutate(&deps)

			if _, err := marketdata.New(deps); !errors.Is(err, tc.want) {
				t.Fatalf("New = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewRefusesAConfigWithNoBudgets(t *testing.T) {
	// A store cannot hold an empty section — FreshnessConfig.Validate refuses
	// it at load — so this asserts the refusal happens at config.New and never
	// reaches a wired module. The failure it prevents is a process that boots
	// clean and refuses every observation for the rest of its life.
	if _, err := config.New[driven.FreshnessConfig]("test", base, []byte(`{"funding":{}}`)); !errors.Is(err, driven.ErrNoVenues) {
		t.Fatalf("config.New with an empty section = %v, want ErrNoVenues", err)
	}
}

// ---------------------------------------------------------------------------
// The wired path: an envelope in, a snapshot on the lane
// ---------------------------------------------------------------------------

func TestModuleCarriesAnEnvelopeThroughToTheLane(t *testing.T) {
	lane := newLane(t)
	m, err := marketdata.New(marketdata.Deps{
		Clock:  newClock(t),
		Config: newStore(t, map[ids.Venue]domain.Budget{venue: narrow}),
		Lane:   lane,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Funding.Deliver(envelope(t, perp(), 1)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	env, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamFunding})
	if !ok {
		t.Fatal("nothing reached the funding mailbox")
	}
	snap, ok := env.Payload.(application.FundingSnapshot)
	if !ok {
		t.Fatalf("payload is %T, want application.FundingSnapshot", env.Payload)
	}

	// The two verdicts DIFFER here, which is what proves the two measurements
	// are wired to two different clocks. Equal verdicts would pass against an
	// implementation that measured both against engine time — §3.5's rejected
	// cross-clock comparison, invisible to any assertion that did not separate
	// them.
	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v, want FreshnessStale (60s past a 30s budget)", snap.Transport)
	}
	if snap.Data != domain.FreshnessFresh {
		t.Errorf("Data = %v, want FreshnessFresh (30s against the venue's own clock)", snap.Data)
	}
	if got := snap.Least(); got != domain.FreshnessStale {
		t.Errorf("Least() = %v, want the worse of the two", got)
	}
}

func TestModuleRefusesAVenueWithNoBudget(t *testing.T) {
	// The configured venue is not the one delivered, which is what shows the
	// wired Budgets really is the config adapter: a stub answering everything
	// would let this through.
	m, err := marketdata.New(marketdata.Deps{
		Clock:  newClock(t),
		Config: newStore(t, map[ids.Venue]domain.Budget{otherVenue: narrow}),
		Lane:   newLane(t),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Funding.Deliver(envelope(t, perp(), 1)); !errors.Is(err, application.ErrNoBudget) {
		t.Fatalf("Deliver = %v, want application.ErrNoBudget", err)
	}
}

func TestModuleAppliesAHotReloadedBudget(t *testing.T) {
	// The end-to-end statement of why Budgets is a port at all. Two
	// observations of the SAME age get opposite verdicts, because the operator
	// widened the budget in between — and the module was constructed before
	// either.
	store := newStore(t, map[ids.Venue]domain.Budget{venue: narrow})
	m, err := marketdata.New(marketdata.Deps{Clock: newClock(t), Config: store, Lane: newLane(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	first, err := m.Funding.Deliver(envelope(t, perp(), 1))
	if err != nil {
		t.Fatalf("first Deliver: %v", err)
	}
	if first.Transport != domain.FreshnessStale {
		t.Fatalf("Transport = %v under the narrow budget, want FreshnessStale", first.Transport)
	}

	if _, err := store.Reload("test", base.Add(time.Hour), document(t, map[ids.Venue]domain.Budget{venue: wide})); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Same RecvTime, same clock reading, higher VenueSeq so it advances the
	// source. The only thing that changed is the configuration.
	second, err := m.Funding.Deliver(envelope(t, perp(), 2))
	if err != nil {
		t.Fatalf("second Deliver: %v", err)
	}
	if second.Transport != domain.FreshnessFresh {
		t.Errorf("Transport = %v under the widened budget, want FreshnessFresh; the module is holding config from construction", second.Transport)
	}
}

func TestModuleArmsTheSilenceTimerThroughTheWiredClock(t *testing.T) {
	// §3.4's timer is the one part of this context that no amount of inbound
	// traffic exercises, because its whole subject is traffic that STOPS.
	// Wiring it to a clock the module did not receive would be invisible to
	// every other test here.
	clk := newClock(t)
	lane := newLane(t)
	m, err := marketdata.New(marketdata.Deps{
		Clock:  clk,
		Config: newStore(t, map[ids.Venue]domain.Budget{venue: narrow}),
		Lane:   lane,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Funding.Deliver(envelope(t, perp(), 1)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if clk.Pending() != 1 {
		t.Fatalf("%d timers armed, want 1", clk.Pending())
	}

	if _, ok := clk.AdvanceToNextTimer(); !ok {
		t.Fatal("no timer to advance to")
	}
	fired, ok := clk.NextDue()
	if !ok {
		t.Fatal("NextDue reported nothing due")
	}

	snap, err := m.Funding.Timer(fired.ID)
	if err != nil {
		t.Fatalf("Timer(%q): %v", fired.ID, err)
	}
	// The verdict has WORSENED, and asserting the exact rung is what makes that
	// a claim rather than a hope. Freshness is ordered by trust, so an equal
	// verdict here would be the spin domain.Budget.NextTransition records: a
	// timer firing at an instant where nothing has changed, re-arming for that
	// same instant, forever, while the ladder never reaches mark_blind_action.
	if snap.Transport != domain.FreshnessBlind {
		t.Errorf("Transport = %v after the silence timer, want FreshnessBlind — the rung past FreshnessStale", snap.Transport)
	}
}
