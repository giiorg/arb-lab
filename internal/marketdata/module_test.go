package marketdata_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/marketdata"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const (
	binance    = ids.Venue("binance")
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
		Venue:    binance,
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

// bookBudget is the book budget every venue in these fixtures gets.
//
// Sub-second, per §3.4, and deliberately unlike any funding budget here: giving
// the two sections the same numbers would make a path that read the wrong
// section invisible in exactly the file that wires both.
//
// It is wide enough that the book fixtures below land FRESH — see bookRecvAt —
// so the book tests assert routing rather than accidentally asserting a
// staleness ladder the funding tests already cover.
var bookBudget = domain.Budget{MaxAge: 200 * time.Millisecond, Blind: 2 * time.Second}

// The book path's own provenance. bookRecvAt and bookSourceAt are chosen so both
// verdicts are Fresh under bookBudget, and bookSourceAt carries the venue skew
// so DataAge is measured against ServerNow rather than accidentally against Now.
var (
	bookRecvAt   = base.Add(-50 * time.Millisecond)
	bookSourceAt = base.Add(venueSkew - 100*time.Millisecond)
)

// The book fixture's sequences. FirstSeq is strictly below the envelope's
// VenueSeq, so the delta spans three venue sequences: venue.BookDelta carries
// only FirstSeq and the adapter reads the far end off the envelope, and with
// First == Last those two sources are the same value.
const (
	bookSnapSeq  uint64 = 5000
	bookFirstSeq uint64 = 5001
	bookLastSeq  uint64 = 5003
)

const bookHoldDepth = 8

// stubResnapshotter accepts every request and counts it.
//
// A stub rather than a real venue client because §20a defers those, which is the
// same reason Resnapshotter is the one PORT on Deps — module.go records it. The
// count is what a test asserts on: a resnapshot requested is the observable
// half of the recovery path.
type stubResnapshotter struct{ requests []ids.VenueInstrument }

func (s *stubResnapshotter) RequestSnapshot(inst ids.VenueInstrument) error {
	s.requests = append(s.requests, inst)
	return nil
}

func bookPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func bookQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func venueBook(t *testing.T) venue.BookSnapshot {
	t.Helper()
	return venue.BookSnapshot{
		Bids: []venue.BookLevel{
			{Price: bookPrice(t, "43250.10"), Quantity: bookQty(t, "0.5")},
			{Price: bookPrice(t, "43250.00"), Quantity: bookQty(t, "1.25")},
		},
		Asks: []venue.BookLevel{
			{Price: bookPrice(t, "43251.00"), Quantity: bookQty(t, "0.75")},
			{Price: bookPrice(t, "43251.50"), Quantity: bookQty(t, "1.5")},
		},
	}
}

// venueBookDelta touches both sides, so a side translation that answered
// unconditionally is visible in the composed path too.
func venueBookDelta(t *testing.T) venue.BookDelta {
	t.Helper()
	return venue.BookDelta{
		FirstSeq: bookFirstSeq,
		Changes: []venue.LevelChange{
			{Side: venue.SideBid, Price: bookPrice(t, "43250.50"), Quantity: bookQty(t, "2")},
			{Side: venue.SideAsk, Price: bookPrice(t, "43250.90"), Quantity: bookQty(t, "3")},
		},
	}
}

func bookEnvelope(inst ids.VenueInstrument, seq uint64, payload any) events.Envelope {
	return events.Envelope{
		AcceptOrdinal: seq + 11,
		IngressSeq:    seq + 31,
		Source:        events.SourceID{Instrument: inst, Stream: events.StreamBook},
		SourceTime:    bookSourceAt,
		RecvTime:      bookRecvAt,
		VenueSeq:      seq,
		Prio:          events.PrioNormal,
		Payload:       payload,
	}
}

// fullDeps is every dependency populated, which each construction case then
// breaks exactly one of.
func fullDeps(t *testing.T, venues map[ids.Venue]domain.Budget) marketdata.Deps {
	t.Helper()
	return marketdata.Deps{
		Clock:         newClock(t),
		Config:        newStore(t, venues),
		Lane:          newLane(t),
		Resnapshotter: &stubResnapshotter{},
		BookHoldDepth: bookHoldDepth,
	}
}

func document(t *testing.T, venues map[ids.Venue]domain.Budget) []byte {
	t.Helper()
	toSpec := func(b domain.Budget) driven.BudgetSpec {
		return driven.BudgetSpec{
			MaxAge: config.NewDuration(b.MaxAge),
			Blind:  config.NewDuration(b.Blind),
		}
	}
	funding := make(map[ids.Venue]driven.BudgetSpec, len(venues))
	books := make(map[ids.Venue]driven.BudgetSpec, len(venues))
	for v, b := range venues {
		funding[v] = toSpec(b)
		books[v] = toSpec(bookBudget)
	}
	doc, err := json.Marshal(driven.FreshnessConfig{Funding: funding, Book: books})
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
		{Venue: binance, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

func update(t *testing.T) venue.Funding {
	t.Helper()
	return venue.Funding{
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
	full := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})

	cases := map[string]struct {
		mutate func(*marketdata.Deps)
		want   error
	}{
		"no clock":  {func(d *marketdata.Deps) { d.Clock = nil }, application.ErrClockNil},
		"no config": {func(d *marketdata.Deps) { d.Config = nil }, driven.ErrStoreNil},
		"no lane":   {func(d *marketdata.Deps) { d.Lane = nil }, driven.ErrLaneNil},

		// The two book-path dependencies. Both are the composition root's to
		// supply and neither has a safe default: a nil Resnapshotter panics on
		// the first gap, in the hot path, where the diagnosis is a stack trace
		// rather than a named refusal at boot.
		"no resnapshotter": {func(d *marketdata.Deps) { d.Resnapshotter = nil }, application.ErrResnapshotterNil},

		// Zero rather than negative, because zero is the value a caller reaches
		// by NOT setting the field — and it is the dangerous one: a buffer of
		// zero holds nothing while awaiting a resnapshot, so the REST book always
		// lands behind the live stream and the very next delta gaps, forever.
		"no hold depth": {func(d *marketdata.Deps) { d.BookHoldDepth = 0 }, application.ErrHoldDepthInvalid},
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
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	lane := deps.Lane
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	in := envelope(t, perp(), 1)
	if _, err := m.Funding.Deliver(in); err != nil {
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

	// §3.5's causal traceability, asserted end to end. This is the only test
	// that can make the claim: the ordinal enters at driving/, crosses
	// application/ as a value on the snapshot, and is stamped back onto an
	// envelope by driven/ — so each layer's own suite can show its half while
	// the chain is still broken in the middle. The replay key must NOT survive
	// the same trip; the lane holds pre-dispatch envelopes.
	if env.AcceptOrdinal != in.AcceptOrdinal {
		t.Errorf("AcceptOrdinal = %d, want the inbound envelope's %d — the published snapshot cannot be traced to its cause",
			env.AcceptOrdinal, in.AcceptOrdinal)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d, want 0 — a mailbox holds pre-dispatch envelopes", env.IngressSeq)
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
	m, err := marketdata.New(fullDeps(t, map[ids.Venue]domain.Budget{otherVenue: narrow}))
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
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	store := deps.Config
	m, err := marketdata.New(deps)
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

	if _, err := store.Reload("test", base.Add(time.Hour), document(t, map[ids.Venue]domain.Budget{binance: wide})); err != nil {
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
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	clk := deps.Clock.(*clock.TestClock)
	m, err := marketdata.New(deps)
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

// ---------------------------------------------------------------------------
// The wired book path
// ---------------------------------------------------------------------------

// TestModuleCarriesABookThroughToItsOwnMailbox is the book path's end-to-end
// claim, and the two mailboxes are what make it one.
//
// Every layer's own suite can show its half while the chain is broken in the
// middle: driving/ can translate into a use case that publishes nowhere, driven/
// can address the right mailbox from a view nobody produced. Only here does an
// envelope enter as a venue payload and leave as a reconstructed book on the
// source the rest of the system subscribes to.
func TestModuleCarriesABookThroughToItsOwnMailbox(t *testing.T) {
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	lane := deps.Lane
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	in := bookEnvelope(perp(), bookSnapSeq, venueBook(t))
	if _, err := m.Book.Deliver(in); err != nil {
		t.Fatalf("Book.Deliver: %v", err)
	}

	// The negative control comes first, and it is the assertion that the two
	// paths are wired to two mailboxes. A book published through
	// venue.FundingSource would be a valid envelope on a live mailbox, and the
	// only symptom is a book consumer that never hears from the instrument.
	if _, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamFunding}); ok {
		t.Error("a book reached the FUNDING mailbox")
	}

	env, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamBook})
	if !ok {
		t.Fatal("nothing reached the book mailbox")
	}
	view, ok := env.Payload.(application.BookView)
	if !ok {
		t.Fatalf("payload is %T, want application.BookView", env.Payload)
	}

	// §3.5's causal traceability across the whole chain: the ordinal enters at
	// driving/, crosses application/ as a value on the view, and is stamped back
	// onto an envelope by driven/. The replay key must NOT survive the trip.
	if env.AcceptOrdinal != in.AcceptOrdinal {
		t.Errorf("AcceptOrdinal = %d, want the inbound envelope's %d", env.AcceptOrdinal, in.AcceptOrdinal)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d, want 0 — a mailbox holds pre-dispatch envelopes", env.IngressSeq)
	}

	// The BOOK budget was read, not the funding one. Under narrow (30s) this
	// book would be Fresh on transport; under bookBudget (200ms) a 50ms-old
	// receive is also Fresh — so the discriminating assertion is the DATA
	// verdict, where the funding budget's 30s and the book's 200ms give the same
	// answer for a 100ms age. Both Fresh is therefore not the claim; the claim
	// is the one below, which reads the budget the two sections disagree about.
	if view.Least() != domain.FreshnessFresh {
		t.Errorf("Least() = %v, want FreshnessFresh", view.Least())
	}
	if view.Book.Watermark.VenueSeq != bookSnapSeq {
		t.Errorf("book seq = %d, want %d", view.Book.Watermark.VenueSeq, bookSnapSeq)
	}
	if view.Applied != domain.SeqRelationUnknown {
		t.Errorf("Applied = %v, want SeqRelationUnknown — a snapshot replaces", view.Applied)
	}
}

// TestModuleReadsTheBookSectionAndNotTheFundingOne is the assertion the test
// above deliberately does not make, isolated so it can be sharp.
//
// The two sections are configured with budgets that DISAGREE about this
// observation: 400ms is inside the funding budget (30s) and outside the book's
// (200ms max age). So a book path wired to FundingBudget reports Fresh and a
// correctly wired one reports Stale. Nothing else in this file distinguishes
// them — §3.4's whole point is that the numbers are not interchangeable, and a
// fixture where both sections agree cannot see which was read.
func TestModuleReadsTheBookSectionAndNotTheFundingOne(t *testing.T) {
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	env := bookEnvelope(perp(), bookSnapSeq, venueBook(t))
	env.RecvTime = base.Add(-400 * time.Millisecond)

	view, err := m.Book.Deliver(env)
	if err != nil {
		t.Fatalf("Book.Deliver: %v", err)
	}
	if view.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v for a 400ms-old book, want FreshnessStale — "+
			"the book path is reading the funding section's %v budget", view.Transport, narrow.MaxAge)
	}
}

// TestModuleAppliesADeltaOnTopOfAWiredSnapshot is the composed reconstruction:
// two envelopes, one book, and the span assembled from both ends.
func TestModuleAppliesADeltaOnTopOfAWiredSnapshot(t *testing.T) {
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	lane := deps.Lane
	resnap := deps.Resnapshotter.(*stubResnapshotter)
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Book.Deliver(bookEnvelope(perp(), bookSnapSeq, venueBook(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}
	view, err := m.Book.Deliver(bookEnvelope(perp(), bookLastSeq, venueBookDelta(t)))
	if err != nil {
		t.Fatalf("Deliver(delta): %v", err)
	}

	if view.Applied != domain.SeqRelationContiguous {
		t.Errorf("Applied = %v, want SeqRelationContiguous", view.Applied)
	}
	if view.Book.Watermark.VenueSeq != bookLastSeq {
		t.Errorf("book seq = %d, want %d — the span's far end is the envelope's",
			view.Book.Watermark.VenueSeq, bookLastSeq)
	}
	if len(view.Book.Bids) != 3 || len(view.Book.Asks) != 3 {
		t.Errorf("book is %d bids / %d asks, want 3 / 3 — each side took one insert",
			len(view.Book.Bids), len(view.Book.Asks))
	}
	if len(resnap.requests) != 0 {
		t.Errorf("%d resnapshots requested, want 0 — a contiguous delta is the steady state",
			len(resnap.requests))
	}

	// Both publishes are on the lane, in order, because the fixture lane does
	// not coalesce. Two envelopes rather than one is what says the delta
	// published a book of its own rather than mutating the first in place.
	src := events.SourceID{Instrument: perp(), Stream: events.StreamBook}
	if got := lane.Len(src); got != 2 {
		t.Errorf("%d envelopes on the book mailbox, want 2", got)
	}
}

// TestModuleRequestsAResnapshotThroughTheWiredPort is the only test that can
// show Deps.Resnapshotter reaches the use case.
//
// It is the one dependency a composition root supplies as a PORT — its adapter
// is a venue REST client that does not exist yet (§20a) — so a New that dropped
// it on the floor would pass every other test here: the gap would still be
// detected, the delta would still be held, and the only missing thing is the
// request that makes recovery ever happen.
func TestModuleRequestsAResnapshotThroughTheWiredPort(t *testing.T) {
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	resnap := deps.Resnapshotter.(*stubResnapshotter)
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Book.Deliver(bookEnvelope(perp(), bookSnapSeq, venueBook(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}

	// A delta that begins well past the book's position: 6000..6002 against a
	// book at 5000. Not merely one ahead, so "gap" and "off by one" are not the
	// same fixture.
	gapped := venueBookDelta(t)
	gapped.FirstSeq = 6000
	if _, err := m.Book.Deliver(bookEnvelope(perp(), 6002, gapped)); !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("Deliver(gapped delta) = %v, want application.ErrSequenceGap", err)
	}

	if len(resnap.requests) != 1 {
		t.Fatalf("%d resnapshots requested, want 1 — the wired port never saw the gap", len(resnap.requests))
	}
	if resnap.requests[0] != perp() {
		t.Errorf("resnapshot requested for %v, want %v", resnap.requests[0], perp())
	}
}

// TestModuleArmsTheBookTimerSeparatelyFromTheFundingOne is why Module has two
// fields and two Timer methods rather than one merged surface.
//
// Both paths arm a silence timer for the SAME instrument, and the IDs must
// differ: clock.SetTimer replaces by ID, so a shared name would make each
// class's arm cancel the other's — and since the two budgets differ by orders of
// magnitude, the survivor would be whichever arrived last. Two pending timers is
// the whole assertion.
func TestModuleArmsTheBookTimerSeparatelyFromTheFundingOne(t *testing.T) {
	deps := fullDeps(t, map[ids.Venue]domain.Budget{binance: narrow})
	clk := deps.Clock.(*clock.TestClock)
	m, err := marketdata.New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := m.Funding.Deliver(envelope(t, perp(), 1)); err != nil {
		t.Fatalf("Funding.Deliver: %v", err)
	}
	if _, err := m.Book.Deliver(bookEnvelope(perp(), bookSnapSeq, venueBook(t))); err != nil {
		t.Fatalf("Book.Deliver: %v", err)
	}

	if got := clk.Pending(); got != 2 {
		t.Fatalf("%d timers armed for one instrument, want 2 — the two classes share an ID namespace", got)
	}

	// The book's fires FIRST, because its budget is sub-second and funding's is
	// tens of seconds. Asserting which one is due is what makes this a test of
	// two distinct timers rather than of a counter.
	if _, ok := clk.AdvanceToNextTimer(); !ok {
		t.Fatal("no timer to advance to")
	}
	fired, ok := clk.NextDue()
	if !ok {
		t.Fatal("NextDue reported nothing due")
	}
	if _, err := m.Book.Timer(fired.ID); err != nil {
		t.Fatalf("Book.Timer(%q) = %v — the book's sub-second budget must fire first", fired.ID, err)
	}
	// And the funding path must NOT claim it. A merged namespace would make one
	// of these two answers wrong, and this is the half that says which.
	if _, err := m.Funding.Timer(fired.ID); !errors.Is(err, application.ErrUnknownTimer) {
		t.Errorf("Funding.Timer(book ID) = %v, want ErrUnknownTimer", err)
	}
}
