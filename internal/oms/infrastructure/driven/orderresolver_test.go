package driven_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// fakeReader is the venue read surface, recording what it was ASKED as well as
// answering.
//
// The urgency log is its own field for dispatch_test's reason: a pinned shed
// class and an ignored argument look identical from the answers alone, and §5.3
// makes this traffic Safety in terms.
type fakeReader struct {
	status    venue.OrderStatus
	statusErr error

	// pages is walked in order; the last one is repeated if the caller keeps
	// going, which is what lets a test drive the paging loop past its budget.
	pages    []venue.ExecutionPage
	pageErr  error
	calls    int
	windows  []venue.ExecutionWindow
	urgency  []ratelimit.Priority
	assets   []venue.InstrumentAssets
	statuses int

	// hardStop is the FAKE's own bound, and it exists so that a missing bound in
	// the code under test FAILS rather than hangs. A mutation deleting the
	// adapter's page budget would otherwise loop forever, and the harness would
	// report TIMEOUT!! — a verdict that names no defect. Set well above the
	// adapter's own budget so it can only be reached by its absence.
	hardStop int
}

func (f *fakeReader) OrderByClientID(_ context.Context, _ venue.OrderRef, a venue.InstrumentAssets, u ratelimit.Priority) (venue.OrderStatus, error) {
	f.statuses++
	f.urgency = append(f.urgency, u)
	f.assets = append(f.assets, a)
	if f.statusErr != nil {
		return venue.OrderStatus{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeReader) Executions(_ context.Context, _ ids.VenueInstrument, a venue.InstrumentAssets, w venue.ExecutionWindow, u ratelimit.Priority) (venue.ExecutionPage, error) {
	f.calls++
	f.windows = append(f.windows, w)
	f.urgency = append(f.urgency, u)
	f.assets = append(f.assets, a)
	if f.hardStop > 0 && f.calls > f.hardStop {
		return venue.ExecutionPage{}, errors.New("fake reader: the caller's page budget did not bind")
	}
	if f.pageErr != nil {
		return venue.ExecutionPage{}, f.pageErr
	}
	if len(f.pages) == 0 {
		return venue.ExecutionPage{}, nil
	}
	i := min(f.calls-1, len(f.pages)-1)
	return f.pages[i], nil
}

var _ driven.VenueOrderReader = (*fakeReader)(nil)

func resolverPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func resolverAssets() map[ids.VenueInstrument]venue.InstrumentAssets {
	return map[ids.VenueInstrument]venue.InstrumentAssets{
		resolverPerp(): {Base: "BTC", Quote: "USDT"},
	}
}

func resolverOrder(t *testing.T) domain.Order {
	t.Helper()
	return domain.Order{
		ClientOrderID: mintedID(t, 1),
		Instrument:    resolverPerp(),
		Side:          domain.SideSell,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      qty(t, "0.5"),
		LimitPrice:    px(t, "50000"),
	}
}

func px(t *testing.T, v string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", v)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return p
}

func newResolverGateway(t *testing.T, r *fakeReader) *driven.OrderResolver {
	t.Helper()
	g, err := driven.NewOrderResolver(r, resolverAssets())
	if err != nil {
		t.Fatalf("NewOrderResolver: %v", err)
	}
	return g
}

// venueReach is the seam's published horizons, with the two DIFFERENT so a
// translation that carried one into both fields is visible.
func venueReach() venue.QueryReach {
	return venue.QueryReach{
		Surface:           venue.SurfaceOrderStatus,
		UnfilledRetention: 3 * time.Hour,
		FilledRetention:   90 * time.Hour,
	}
}

func TestNewOrderResolverRefusesAnIncompleteWiring(t *testing.T) {
	t.Parallel()

	if _, err := driven.NewOrderResolver(nil, resolverAssets()); !errors.Is(err, driven.ErrVenueOrderReaderMissing) {
		t.Errorf("NewOrderResolver(nil, assets) = %v, want ErrVenueOrderReaderMissing", err)
	}
	if _, err := driven.NewOrderResolver(&fakeReader{}, nil); !errors.Is(err, driven.ErrInstrumentAssetsMissing) {
		t.Errorf("NewOrderResolver(reader, nil) = %v, want ErrInstrumentAssetsMissing", err)
	}

	// A populated table holding an unusable entry. Distinct from the empty case
	// because a constructor that checked only len() would pass that one — and
	// the entry it would admit builds Money with no asset tag, whose first
	// failure is far from here.
	bad := map[ids.VenueInstrument]venue.InstrumentAssets{resolverPerp(): {Base: "BTC"}}
	if _, err := driven.NewOrderResolver(&fakeReader{}, bad); err == nil {
		t.Error("NewOrderResolver with an unusable asset pair = nil, want a refusal")
	}

	if _, err := driven.NewOrderResolver(&fakeReader{}, resolverAssets()); err != nil {
		t.Errorf("NewOrderResolver with a complete wiring = %v, want nil", err)
	}
}

func TestResolveRefusesAnInstrumentWithNoConfiguredAssets(t *testing.T) {
	t.Parallel()

	r := &fakeReader{}
	g := newResolverGateway(t, r)
	o := resolverOrder(t)
	o.Instrument = ids.VenueInstrument{Venue: "bybit", Product: ids.ProductSpot, Symbol: "BTCUSDT"}

	if _, err := g.Resolve(o); !errors.Is(err, driven.ErrInstrumentAssetsUnconfigured) {
		t.Fatalf("Resolve = %v, want ErrInstrumentAssetsUnconfigured", err)
	}
	if r.statuses != 0 {
		t.Errorf("queried the venue %d times with no asset pair, want 0", r.statuses)
	}
}

// The translation table, walked in full. Each row is a claim about which SM §1
// state a venue reading means, and the interesting one is Terminated →
// StateCanceled: engine/venue refuses a Canceled/Expired split because the two
// venues make it unnormalizable, and SM §1 has one member for it.
func TestResolveTranslatesEveryVenueState(t *testing.T) {
	t.Parallel()

	cases := map[venue.OrderState]domain.OrderState{
		venue.OrderStateOpen:            domain.StateAcknowledged,
		venue.OrderStatePartiallyFilled: domain.StatePartiallyFilled,
		venue.OrderStateFilled:          domain.StateFilled,
		venue.OrderStateTerminated:      domain.StateCanceled,
		venue.OrderStateRejected:        domain.StateRejected,
	}
	for from, want := range cases {
		t.Run(from.String(), func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{status: venue.OrderStatus{
				Presence:         venue.OrderPresent,
				Reach:            venueReach(),
				State:            from,
				ExecutedQuantity: qty(t, "0.25"),
				VenueOrderID:     "V-9",
			}}
			g := newResolverGateway(t, r)

			got, err := g.Resolve(resolverOrder(t))
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.State != want {
				t.Errorf("Resolve(%s).State = %s, want %s", from, got.State, want)
			}
			if got.Presence != ports.VenueHasOrder {
				t.Errorf("Presence = %s, want %s", got.Presence, ports.VenueHasOrder)
			}
			if got.VenueOrderID != "V-9" {
				t.Errorf("VenueOrderID = %q, want %q", got.VenueOrderID, "V-9")
			}
		})
	}

	// An undeclared venue state is refused rather than translated to whichever
	// member is nearest. Reachable through the seam's type, which is a string.
	t.Run("undeclared", func(t *testing.T) {
		t.Parallel()

		r := &fakeReader{status: venue.OrderStatus{
			Presence: venue.OrderPresent, Reach: venueReach(), State: venue.OrderState("suspended"),
		}}
		g := newResolverGateway(t, r)
		if _, err := g.Resolve(resolverOrder(t)); !errors.Is(err, venue.ErrOrderStateInvalid) {
			t.Errorf("Resolve of an undeclared state = %v, want ErrOrderStateInvalid", err)
		}
	})
}

// An absence carries no state and no quantity, and the assertion is that they
// are ZERO rather than translated: a venue with no such order has no lifecycle
// reading, and a translated zero would be a domain.OrderState a caller could
// branch on for an order that does not exist.
func TestResolveLeavesAnAbsencesStateUnset(t *testing.T) {
	t.Parallel()

	r := &fakeReader{status: venue.OrderStatus{
		Presence: venue.OrderAbsent,
		Reach:    venue.QueryReach{Surface: venue.SurfaceOrderHistory, Unpublished: true},
	}}
	g := newResolverGateway(t, r)

	got, err := g.Resolve(resolverOrder(t))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Presence != ports.VenueHasNoOrder {
		t.Fatalf("Presence = %s, want %s", got.Presence, ports.VenueHasNoOrder)
	}
	if got.State != "" {
		t.Errorf("State = %q, want the zero state", got.State)
	}
	if got.Executed != (fixedpoint.Quantity{}) {
		t.Errorf("Executed = %v, want the zero quantity", got.Executed)
	}
	// The surface that answered is carried, because a refusal downstream names
	// it — and it is the HISTORY surface here, not the status one, so a
	// translation that hardcoded either is visible.
	if got.Reach.Surface != venue.SurfaceOrderHistory.String() {
		t.Errorf("Reach.Surface = %q, want %q", got.Reach.Surface, venue.SurfaceOrderHistory)
	}
	if got.Reach.Published {
		t.Error("Reach.Published = true for a venue that publishes no retention")
	}
}

// Both retentions cross intact and the flag is INVERTED, not copied. Published
// and Unpublished are opposite spellings of one fact, and a translation that
// assigned rather than negated would make every unpublished reach read as a
// published one with zero horizons — which the resolver's gate 4 would then
// treat as a venue that remembers nothing.
func TestResolveCarriesBothRetentionsAndInvertsThePublishedFlag(t *testing.T) {
	t.Parallel()

	r := &fakeReader{status: venue.OrderStatus{
		Presence: venue.OrderPresent, Reach: venueReach(), State: venue.OrderStateFilled,
	}}
	g := newResolverGateway(t, r)

	got, err := g.Resolve(resolverOrder(t))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Reach.Published {
		t.Error("Published = false for a venue that states both horizons")
	}
	if got.Reach.UnfilledRetention != 3*time.Hour || got.Reach.FilledRetention != 90*time.Hour {
		t.Errorf("reach = %v/%v, want 3h/90h", got.Reach.UnfilledRetention, got.Reach.FilledRetention)
	}
	if err := got.Reach.Validate(); err != nil {
		t.Errorf("the translated reach does not satisfy the port: %v", err)
	}
}

// §5.3 makes the resolving queries Safety traffic, and the class is pinned here
// rather than taken as an argument. Both methods are checked: one pinned and one
// defaulted is exactly the asymmetry the mirror rule exists to catch.
func TestResolverPinsTheSafetyShedClassOnBothCalls(t *testing.T) {
	t.Parallel()

	r := &fakeReader{status: venue.OrderStatus{
		Presence: venue.OrderPresent, Reach: venueReach(), State: venue.OrderStateFilled,
	}}
	g := newResolverGateway(t, r)

	if _, err := g.Resolve(resolverOrder(t)); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := g.SweepExecutions(resolverOrder(t), testStart, testStart.Add(time.Minute)); err != nil {
		t.Fatalf("SweepExecutions: %v", err)
	}
	if len(r.urgency) != 2 {
		t.Fatalf("recorded %d calls, want 2", len(r.urgency))
	}
	for i, u := range r.urgency {
		if u != ratelimit.PrioritySafety {
			t.Errorf("call %d ran at %s, want %s", i, u, ratelimit.PrioritySafety)
		}
	}
}

var testStart = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

// The paging walk, over THREE pages with a different watermark on each
// (checklist: one page makes "reports the last page's watermark" and "reports
// the first's" the same value, and two make "the last" and "the second" the
// same).
//
// The continuation is asserted to be the venue's own window handed back
// VERBATIM, because engine/venue returns a whole window rather than a token
// precisely so a caller cannot rebuild one wrongly — Bybit's page cursor walks
// backward into older data while a resume instant moves forward.
func TestSweepExecutionsWalksEveryPageAndReportsTheLastWatermark(t *testing.T) {
	t.Parallel()

	second := venue.ExecutionWindow{Since: venue.Cursor{At: testStart.Add(time.Minute)}, Until: testStart.Add(time.Hour), Continuation: "page-2"}
	third := venue.ExecutionWindow{Since: venue.Cursor{At: testStart.Add(2 * time.Minute)}, Until: testStart.Add(time.Hour), Continuation: "page-3"}

	r := &fakeReader{pages: []venue.ExecutionPage{
		{
			Executions: []venue.Execution{{ExecutionID: "e-1", ClientOrderID: "c-1", Quantity: qty(t, "0.1"), At: testStart.Add(time.Second)}},
			Watermark:  venue.Cursor{At: testStart.Add(time.Minute)},
			More:       &second,
		},
		{
			Executions: []venue.Execution{{ExecutionID: "e-2", Quantity: qty(t, "0.2"), At: testStart.Add(90 * time.Second)}},
			Watermark:  venue.Cursor{At: testStart.Add(2 * time.Minute)},
			More:       &third,
		},
		{
			Executions: []venue.Execution{{ExecutionID: "e-3", ClientOrderID: "c-3", Quantity: qty(t, "0.3"), At: testStart.Add(50 * time.Minute)}},
			Watermark:  venue.Cursor{At: testStart.Add(time.Hour)},
		},
	}}
	g := newResolverGateway(t, r)

	got, err := g.SweepExecutions(resolverOrder(t), testStart, testStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("SweepExecutions: %v", err)
	}
	if r.calls != 3 {
		t.Fatalf("made %d calls, want 3", r.calls)
	}
	if len(got.Executions) != 3 {
		t.Fatalf("collected %d executions, want 3 — every page's rows, not only the last", len(got.Executions))
	}
	if !got.ProvenThrough.Equal(testStart.Add(time.Hour)) {
		t.Errorf("ProvenThrough = %v, want the LAST page's watermark %v", got.ProvenThrough, testStart.Add(time.Hour))
	}

	// The FIRST window is the only one this adapter builds itself, and it was the
	// only one this test did not look at — every assertion was about the windows
	// handed BACK, so swapping since and until survived the battery. The bounds
	// differ (an instant and an hour later), so the swapped window is a different
	// value; equal bounds would make this assertion vacuous.
	first := venue.ExecutionWindow{Since: venue.Cursor{At: testStart}, Until: testStart.Add(time.Hour)}
	if r.windows[0] != first {
		t.Errorf("opened with %+v, want %+v — since and until are the caller's bounds, in that order", r.windows[0], first)
	}

	// Verbatim continuation, including the venue's own paging token.
	if r.windows[1] != second || r.windows[2] != third {
		t.Errorf("continued with %+v, want the venue's own windows %+v and %+v", r.windows[1:], second, third)
	}

	// The empty client order id on the middle row survives translation. It is
	// Binance's shape, and it is what makes a sweep inconclusive — a translation
	// that substituted anything for it would silently make every Binance sweep
	// conclusive.
	if got.Executions[1].ClientOrderID != "" {
		t.Errorf("row 2 client order id = %q, want it carried through empty", got.Executions[1].ClientOrderID)
	}
}

// The page budget binds, and the fixture is built so that its ABSENCE fails
// rather than hangs.
//
// A venue that always returns a continuation drives an unbounded loop forever; a
// mutation deleting the adapter's budget would then be reported TIMEOUT!!, which
// names no defect. The fake's own hardStop — well above the adapter's budget, so
// only its absence can reach it — turns that into an error the test observes.
//
// Exceeding the budget is a SHORT SWEEP and not a failure: the watermark says how
// far coverage got, and the resolver refuses to prove absence on it.
func TestSweepExecutionsStopsAtItsPageBudgetAndReportsWhatItProved(t *testing.T) {
	t.Parallel()

	endless := venue.ExecutionWindow{Since: venue.Cursor{At: testStart.Add(time.Minute)}, Until: testStart.Add(time.Hour), Continuation: "next"}
	r := &fakeReader{
		hardStop: 200,
		pages: []venue.ExecutionPage{{
			Executions: []venue.Execution{{ExecutionID: "e-1", ClientOrderID: "c-1", Quantity: qty(t, "0.1"), At: testStart.Add(time.Second)}},
			Watermark:  venue.Cursor{At: testStart.Add(time.Minute)},
			More:       &endless,
		}},
	}
	g := newResolverGateway(t, r)

	got, err := g.SweepExecutions(resolverOrder(t), testStart, testStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("SweepExecutions = %v, want a short sweep rather than a failure", err)
	}
	if r.calls >= r.hardStop {
		t.Fatalf("made %d calls — the page budget did not bind", r.calls)
	}

	// The budget is a KNOWN EXACT COUNT, which makes it an instrument: any
	// deviation is unambiguous, where "fewer than the hard stop" is satisfied by
	// every wrong bound between 1 and 199 — an off-by-one survived the battery
	// against that check. Stated as a literal rather than imported: the constant
	// is unexported, and a shared one would move with the code and check nothing.
	const wantPages = 32
	if r.calls != wantPages {
		t.Errorf("made %d calls, want exactly %d — maxSweepPages is what bounds this loop", r.calls, wantPages)
	}

	// Asserted POSITIVELY, against the watermark of the last page actually read.
	// The negative form — "did not reach the requested horizon" — is satisfied by
	// a zero watermark too, so it could not see a loop that recorded the
	// watermark only on the page carrying no continuation. On a truncated sweep
	// there IS no such page, which is exactly when the resolver needs the number.
	if !got.ProvenThrough.Equal(testStart.Add(time.Minute)) {
		t.Errorf("ProvenThrough = %v, want the last page READ %v — coverage is proved through the page that answered, not the window that was asked for",
			got.ProvenThrough, testStart.Add(time.Minute))
	}
	if len(got.Executions) != r.calls {
		t.Errorf("collected %d executions over %d pages, want one per page", len(got.Executions), r.calls)
	}
}

// A failed page is an error, not a short sweep: the difference is whether the
// venue ANSWERED, and §5.3 forbids the two being conflated.
func TestSweepExecutionsReportsAFailedPage(t *testing.T) {
	t.Parallel()

	boom := errors.New("rate limited")
	r := &fakeReader{pageErr: boom}
	g := newResolverGateway(t, r)

	if _, err := g.SweepExecutions(resolverOrder(t), testStart, testStart.Add(time.Minute)); !errors.Is(err, boom) {
		t.Fatalf("SweepExecutions = %v, want the venue's error", err)
	}
}

// An empty sweep still advances the watermark — "nothing happened through T" is
// a positive answer and the commonest one — and this asserts the adapter carries
// the venue's own watermark rather than inventing one.
func TestSweepExecutionsCarriesAnEmptyPagesWatermark(t *testing.T) {
	t.Parallel()

	r := &fakeReader{pages: []venue.ExecutionPage{{Watermark: venue.Cursor{At: testStart.Add(time.Hour)}}}}
	g := newResolverGateway(t, r)

	got, err := g.SweepExecutions(resolverOrder(t), testStart, testStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("SweepExecutions: %v", err)
	}
	if len(got.Executions) != 0 {
		t.Errorf("collected %d executions from an empty page", len(got.Executions))
	}
	if !got.ProvenThrough.Equal(testStart.Add(time.Hour)) {
		t.Errorf("ProvenThrough = %v, want the venue's watermark", got.ProvenThrough)
	}
}
