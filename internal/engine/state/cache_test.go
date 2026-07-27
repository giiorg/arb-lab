package state_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// perp and spot are the two legs of a cash-and-carry package, and they are on
// DIFFERENT venues on purpose: that is the configuration in which a SourceTime
// skew would be an unsynchronized-clock subtraction, and it is the configuration
// §2.2 actually trades.
func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    ids.Venue("binance"),
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func spot() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:   ids.Venue("bybit"),
		Product: ids.ProductSpot,
		Symbol:  "BTCUSDT",
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

func price(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

// fundingAt builds a published funding snapshot delivered d after base.
//
// The published Transport/Data verdicts are set to FreshnessFresh deliberately:
// they are the fields the cache must NOT retain, so a fixture leaving them at
// the FreshnessUnknown zero value would make "dropped" and "carried" produce the
// same observable state.
func fundingAt(t *testing.T, vi ids.VenueInstrument, d time.Duration) ports.FundingSnapshot {
	t.Helper()
	return ports.FundingSnapshot{
		Observation: domain.FundingObservation{
			Instrument: vi,
			Rate:       rate(t, "0.0001"),
			Interval:   8 * time.Hour,
			NextSettle: base.Add(4 * time.Hour),
			Cap:        rate(t, "0.0075"),
			Floor:      rate(t, "-0.0075"),
			Watermark:  at(d),
		},
		Transport:    domain.FreshnessFresh,
		Data:         domain.FreshnessFresh,
		GapDetected:  true,
		SeqAvailable: true,
		Trigger:      ports.AcceptOrdinal(41),
	}
}

// bookAt builds a published book view delivered d after base. seq is stated
// because BookSnapshot.Validate refuses a zero one — §3.4's "a book admitted at
// position zero is not a wrong book" rule.
func bookAt(t *testing.T, vi ids.VenueInstrument, d time.Duration, seq uint64) ports.BookView {
	t.Helper()
	w := at(d)
	w.VenueSeq = seq
	return ports.BookView{
		Book: domain.BookSnapshot{
			Instrument: vi,
			Bids:       []domain.BookLevel{{Price: price(t, "100"), Quantity: qty(t, "2")}},
			Asks:       []domain.BookLevel{{Price: price(t, "101"), Quantity: qty(t, "3")}},
			Watermark:  w,
		},
		Transport: domain.FreshnessFresh,
		Data:      domain.FreshnessFresh,
		Applied:   domain.SeqRelationContiguous,
		Trigger:   ports.AcceptOrdinal(77),
	}
}

// TestEntriesCarryNoFreshnessVerdict is the structural assertion behind the
// package's central claim, and it is a test rather than a comment because the
// field somebody adds back would look like a helpful convenience.
//
// A verdict computed at accept time and stored is a verdict that reports a dead
// feed as healthy for the whole outage — §3.4's silence-timer failure,
// reintroduced one hop downstream. The defence is that the field does not exist,
// so this reads the struct rather than any value it could hold.
func TestEntriesCarryNoFreshnessVerdict(t *testing.T) {
	t.Parallel()

	freshness := reflect.TypeOf(domain.FreshnessFresh)
	for _, typ := range []reflect.Type{
		reflect.TypeOf(state.FundingEntry{}),
		reflect.TypeOf(state.BookEntry{}),
	} {
		for i := range typ.NumField() {
			if f := typ.Field(i); f.Type == freshness {
				t.Errorf("%s.%s is a %s: a verdict computed at accept time "+
					"reports a dead feed as fresh for the whole outage",
					typ.Name(), f.Name, freshness)
			}
		}
	}
}

func TestPutAndReadFunding(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	snap := fundingAt(t, perp(), 0)
	if err := c.PutFunding(snap); err != nil {
		t.Fatalf("PutFunding: %v", err)
	}

	got, ok := c.Funding(perp())
	if !ok {
		t.Fatal("Funding: no entry after a successful put")
	}
	if got.Observation != snap.Observation {
		t.Errorf("observation = %+v, want %+v", got.Observation, snap.Observation)
	}
	// Each provenance field checked individually, and each fixture value is
	// distinct from its zero: GapDetected and SeqAvailable are both true and the
	// ordinal is 41, so a put that dropped any one of them fails here rather than
	// coinciding with the zero value.
	if !got.GapDetected {
		t.Error("GapDetected was not carried")
	}
	if !got.SeqAvailable {
		t.Error("SeqAvailable was not carried")
	}
	if got.Trigger != ports.AcceptOrdinal(41) {
		t.Errorf("Trigger = %d, want 41 — §3.5's causal trace does not stop at the lane", got.Trigger)
	}
}

func TestPutAndReadBook(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	view := bookAt(t, perp(), 0, 7)
	if err := c.PutBook(view); err != nil {
		t.Fatalf("PutBook: %v", err)
	}

	got, ok := c.Book(perp())
	if !ok {
		t.Fatal("Book: no entry after a successful put")
	}
	if got.Book.Watermark.VenueSeq != 7 {
		t.Errorf("VenueSeq = %d, want 7", got.Book.Watermark.VenueSeq)
	}
	// SeqRelationContiguous rather than the SeqRelationUnknown zero, so a put
	// that dropped the field is visible.
	if got.Applied != domain.SeqRelationContiguous {
		t.Errorf("Applied = %v, want %v — a consumer counting resyncs reads this",
			got.Applied, domain.SeqRelationContiguous)
	}
	if got.Trigger != ports.AcceptOrdinal(77) {
		t.Errorf("Trigger = %d, want 77", got.Trigger)
	}
}

// TestMissingInstrumentIsNotAnError: nothing observed yet is the ordinary state
// of a freshly booted process, which is why the single-instrument readers return
// a bool rather than the sentinel the cross-leg query uses.
func TestMissingInstrumentIsNotAnError(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if _, ok := c.Funding(perp()); ok {
		t.Error("Funding reported an entry in an empty cache")
	}
	if _, ok := c.Book(perp()); ok {
		t.Error("Book reported an entry in an empty cache")
	}
}

// TestClassesAreIndependent puts BOTH classes for ONE instrument. §3.4's data
// classes have non-interchangeable budgets and separate lanes, and a single map
// keyed by instrument would make the second write evict the first — a book
// arriving would silently blind the funding path for that perp.
func TestClassesAreIndependent(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
		t.Fatalf("PutFunding: %v", err)
	}
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook: %v", err)
	}

	if _, ok := c.Funding(perp()); !ok {
		t.Error("the funding entry was evicted by a book write for the same instrument")
	}
	if _, ok := c.Book(perp()); !ok {
		t.Error("the book entry is missing")
	}
}

// TestPutRefusesRegression pins the direction of "latest".
//
// Equal is accepted and strictly-earlier is refused, and both halves are load-
// bearing: under clock.TestClock every observation dispatched between advances
// shares a RecvTime, so refusing equality would make a backtest drop all but the
// first update at each instant — a live/replay divergence §3.5's parity claim
// would not survive. A fixture set testing only "much later" and "much earlier"
// passes against `Before` and `!After` alike.
func TestPutRefusesRegression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		second      time.Duration
		wantRefusal bool
	}{
		{name: "later is accepted", second: time.Second, wantRefusal: false},
		{name: "the same instant is accepted", second: 0, wantRefusal: false},
		{name: "one tick earlier is refused", second: -time.Nanosecond, wantRefusal: true},
		{name: "much earlier is refused", second: -time.Hour, wantRefusal: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			t.Run("funding", func(t *testing.T) {
				c := state.NewCache()
				if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
					t.Fatalf("first put: %v", err)
				}
				err := c.PutFunding(fundingAt(t, perp(), tc.second))
				assertRegression(t, err, tc.wantRefusal)
			})

			// The mirror. A refusal written once and applied on one of two write
			// paths is this repo's Send/Receive lesson: when a rule earns a
			// check, its symmetric case gets the same check immediately.
			t.Run("book", func(t *testing.T) {
				c := state.NewCache()
				if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
					t.Fatalf("first put: %v", err)
				}
				err := c.PutBook(bookAt(t, perp(), tc.second, 8))
				assertRegression(t, err, tc.wantRefusal)
			})
		})
	}
}

func assertRegression(t *testing.T, err error, want bool) {
	t.Helper()
	if got := errors.Is(err, state.ErrRegression); got != want {
		t.Fatalf("error = %v, want ErrRegression: %v", err, want)
	}
	if !want && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPutBookAcceptsSequenceRegressionOnResnapshot is the case notBefore is
// written to admit, and it is the reason the check compares RecvTime rather than
// VenueSeq.
//
// §3.5 makes VenueSeq authoritative only WITHIN a SourceID, and a full snapshot
// REPLACES the book rather than following it — domain.SeqRelationUnknown is
// exactly that. A resnapshot recovering a desynced book can therefore carry a
// lower sequence than the one held, and a sequence-based check would reject the
// one write the cache most needs to accept: recovery.
func TestPutBookAcceptsSequenceRegressionOnResnapshot(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutBook(bookAt(t, perp(), 0, 5000)); err != nil {
		t.Fatalf("first put: %v", err)
	}

	// Later in engine time, far lower in venue sequence — a fresh REST snapshot
	// after a venue-side sequence reset.
	resnap := bookAt(t, perp(), time.Second, 3)
	resnap.Applied = domain.SeqRelationUnknown
	if err := c.PutBook(resnap); err != nil {
		t.Fatalf("a resnapshot with a lower VenueSeq must be accepted: %v", err)
	}

	got, ok := c.Book(perp())
	if !ok {
		t.Fatal("Book: no entry")
	}
	if got.Book.Watermark.VenueSeq != 3 {
		t.Errorf("VenueSeq = %d, want 3 — the resnapshot did not replace the book",
			got.Book.Watermark.VenueSeq)
	}
}

func TestPutRefusesInvalidPayload(t *testing.T) {
	t.Parallel()

	c := state.NewCache()

	// Positive control first: the fixtures this table mutates must themselves be
	// accepted, or a Put that refused everything would score a perfect pass.
	if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
		t.Fatalf("the fixture snapshot must be accepted: %v", err)
	}
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("the fixture view must be accepted: %v", err)
	}

	fresh := state.NewCache()

	// Funding on a SPOT instrument — the domain refusal that matters most here,
	// since a spot leg reading "funding 0" nets against a real perp rate.
	badFunding := fundingAt(t, spot(), 0)
	if err := fresh.PutFunding(badFunding); err == nil {
		t.Error("PutFunding accepted a funding observation on a spot instrument")
	}

	// A book at sequence zero: §3.4's book that validates, marks synced, and can
	// never advance.
	badBook := bookAt(t, perp(), 0, 0)
	if err := fresh.PutBook(badBook); err == nil {
		t.Error("PutBook accepted a book with no venue sequence")
	}

	if f, b := fresh.Len(); f != 0 || b != 0 {
		t.Errorf("Len() = (%d, %d), want (0, 0) — a refused put must store nothing", f, b)
	}
}

func TestBookSkewAcrossLegs(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook(perp): %v", err)
	}
	if err := c.PutBook(bookAt(t, spot(), 300*time.Millisecond, 9)); err != nil {
		t.Fatalf("PutBook(spot): %v", err)
	}

	got, err := c.BookSkew(perp(), spot())
	if err != nil {
		t.Fatalf("BookSkew: %v", err)
	}
	if want := 300 * time.Millisecond; got != want {
		t.Errorf("BookSkew = %v, want %v", got, want)
	}
}

// TestBookSkewRefusesMissingLeg: scoring the legs that happen to be present
// returns a SMALLER skew the less the cache knows, which is the permissive
// direction on the input where the engine knows least.
//
// Both orderings are exercised, because a gather that checked only the first
// instrument would pass one of them.
func TestBookSkewRefusesMissingLeg(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook: %v", err)
	}

	for _, tc := range []struct {
		name string
		legs []ids.VenueInstrument
	}{
		{name: "missing leg second", legs: []ids.VenueInstrument{perp(), spot()}},
		{name: "missing leg first", legs: []ids.VenueInstrument{spot(), perp()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.BookSkew(tc.legs...)
			if !errors.Is(err, state.ErrEntryAbsent) {
				t.Fatalf("BookSkew = (%v, %v), want ErrEntryAbsent", got, err)
			}
		})
	}
}

// TestBookSkewOfOneLegIsUndefined: the single-leg call must reach Skew's
// fold-identity refusal rather than returning a zero, and it must do so even
// when that leg IS cached — otherwise the two refusals are indistinguishable.
func TestBookSkewOfOneLegIsUndefined(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook: %v", err)
	}

	if got, err := c.BookSkew(perp()); !errors.Is(err, state.ErrSkewUndefined) {
		t.Fatalf("BookSkew(one cached leg) = (%v, %v), want ErrSkewUndefined", got, err)
	}
	if got, err := c.BookSkew(); !errors.Is(err, state.ErrSkewUndefined) {
		t.Fatalf("BookSkew() = (%v, %v), want ErrSkewUndefined", got, err)
	}
}

// TestLenCountsClassesSeparately uses ASYMMETRIC counts on purpose: with the
// same number in each class, a Len that returned its two values in the wrong
// order would be indistinguishable from a correct one.
func TestLenCountsClassesSeparately(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
		t.Fatalf("PutFunding: %v", err)
	}
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook(perp): %v", err)
	}
	if err := c.PutBook(bookAt(t, spot(), 0, 9)); err != nil {
		t.Fatalf("PutBook(spot): %v", err)
	}

	funding, books := c.Len()
	if funding != 1 || books != 2 {
		t.Errorf("Len() = (%d, %d), want (1, 2)", funding, books)
	}
}

// TestZeroValueCacheIsUsable pins the one place this repo's "the zero value is
// unusable" rule deliberately reverses. An empty cache is the honest state of a
// booted process, and its readers return (zero, false) rather than a value — so
// emptiness cannot be mistaken for data, which is the property that rule
// protects.
func TestZeroValueCacheIsUsable(t *testing.T) {
	t.Parallel()

	var c state.Cache
	if _, ok := c.Funding(perp()); ok {
		t.Error("a zero-value cache reported an entry")
	}
	if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
		t.Fatalf("PutFunding on a zero-value cache: %v", err)
	}
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook on a zero-value cache: %v", err)
	}
	if f, b := c.Len(); f != 1 || b != 1 {
		t.Errorf("Len() = (%d, %d), want (1, 1)", f, b)
	}
}

// TestConcurrentReadersAndWriter is the guard on the package doc's claim that
// "single-writer" is a statement about authority and not about synchronization.
//
// One writer, many readers, which is §3.4's actual topology — strategy, risk and
// oms read while the marketdata reducer writes. It is meaningful only under
// -race, which `make test` runs; without it a passing run says nothing, and that
// is stated here rather than discovered by someone reading a green suite.
func TestConcurrentReadersAndWriter(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	const writes = 200

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range writes {
			// Strictly advancing, so no write is refused as a regression and the
			// loop exercises the store rather than the guard.
			if err := c.PutFunding(fundingAt(t, perp(), time.Duration(i)*time.Millisecond)); err != nil {
				t.Errorf("PutFunding(%d): %v", i, err)
				return
			}
		}
	}()

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range writes {
				c.Funding(perp())
				c.Book(perp())
				c.Len()
			}
		}()
	}

	wg.Wait()
}

// TestEntryAgesUseTheWatermark checks the read-time re-judging seam: the ages an
// entry reports are computed against the instant the CALLER supplies, so the
// same entry answers differently as time passes with nothing arriving. That is
// the property a stored verdict would not have.
func TestEntryAgesUseTheWatermark(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutFunding(fundingAt(t, perp(), 0)); err != nil {
		t.Fatalf("PutFunding: %v", err)
	}
	e, ok := c.Funding(perp())
	if !ok {
		t.Fatal("Funding: no entry")
	}

	// Two instants, and the second is an hour later with nothing having arrived.
	// A single instant would let a constant-returning Age pass.
	for _, offset := range []time.Duration{time.Second, time.Hour} {
		got, err := e.Age(base.Add(offset))
		if err != nil {
			t.Fatalf("Age at +%v: %v", offset, err)
		}
		if got != offset {
			t.Errorf("Age at +%v = %v, want %v", offset, got, offset)
		}
	}

	// DataAge is measured against the venue's own stamp, which `at` places 37s
	// before RecvTime — a different number from Age, so a method that measured
	// the wrong field is visible rather than coincidentally right.
	dataAge, err := e.DataAge(base.Add(time.Second))
	if err != nil {
		t.Fatalf("DataAge: %v", err)
	}
	if want := 38 * time.Second; dataAge != want {
		t.Errorf("DataAge = %v, want %v", dataAge, want)
	}
}

// TestBookEntryAges is the mirror of the test above. Same rule, same reason as
// the regression table's two halves.
func TestBookEntryAges(t *testing.T) {
	t.Parallel()

	c := state.NewCache()
	if err := c.PutBook(bookAt(t, perp(), 0, 7)); err != nil {
		t.Fatalf("PutBook: %v", err)
	}
	e, ok := c.Book(perp())
	if !ok {
		t.Fatal("Book: no entry")
	}

	got, err := e.Age(base.Add(time.Hour))
	if err != nil {
		t.Fatalf("Age: %v", err)
	}
	if got != time.Hour {
		t.Errorf("Age = %v, want %v", got, time.Hour)
	}

	dataAge, err := e.DataAge(base.Add(time.Second))
	if err != nil {
		t.Fatalf("DataAge: %v", err)
	}
	if want := 38 * time.Second; dataAge != want {
		t.Errorf("DataAge = %v, want %v", dataAge, want)
	}

	if e.Watermark().VenueSeq != 7 {
		t.Errorf("Watermark().VenueSeq = %d, want 7", e.Watermark().VenueSeq)
	}
}
