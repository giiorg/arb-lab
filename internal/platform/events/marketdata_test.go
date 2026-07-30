package events_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The market-data lane resolves the slow-consumer question the opposite way from
// the control lane: it never blocks, so it must lose something, and the whole
// design is about losing the RIGHT thing.
//
// A superseded order-book snapshot is worth nothing once a newer one exists, so
// coalescing it away costs nothing. A superseded perp MARK is different, and
// [→ R2c-1] is the reason this lane has two rules instead of one: liq distance
// is a function of mark, mark moves continuously with no account event, and a
// brief adverse excursion that breached a threshold must not be erased by the
// recovery that follows it. Latest-wins would erase it, silently, and the
// position would be closer to liquidation than anything in the process knew.
//
// So: mark uses AdversePreserving, everything else uses LatestWins, and the
// tests below are mostly about the difference.

// markPayload is a stand-in for the domain's mark type. The kernel cannot know
// what "adverse" means — that depends on which way a position is facing — so the
// comparison is supplied by the caller and this is the test's version of it.
type markPayload struct{ price int64 }

// worseForAShort: we are short the perp, so a HIGHER mark is adverse. That is
// the cash-and-carry posture — long spot, short perp (§2.2) — and the direction
// the whole rule exists to protect.
func worseForAShort(candidate, incumbent markPayload) bool {
	return candidate.price > incumbent.price
}

func markSource() events.SourceID { return aSource() } // aSource is already a perp mark

func bookSource() events.SourceID {
	s := aSource()
	s.Stream = events.StreamBook
	return s
}

// marketEnv builds a market-data envelope for src carrying payload.
func marketEnv(src events.SourceID, ordinal uint64, payload any) events.Envelope {
	e := anEnvelope()
	e.Source = src
	e.AcceptOrdinal = ordinal
	e.IngressSeq = ordinal
	e.RecvTime = time.Unix(1700000000+int64(ordinal), 0).UTC()
	e.Payload = payload
	return e
}

// markLane is the lane under most of the tests below: marks coalesce
// adverse-preservingly, everything else latest-wins.
func markLane(t *testing.T, depth int) *events.MarketDataLane {
	t.Helper()
	l, err := events.NewMarketDataLane(depth, func(s events.SourceID) events.MergeFunc {
		if s.Stream == events.StreamMark {
			return events.AdversePreserving(worseForAShort)
		}
		return events.LatestWins
	})
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	return l
}

func TestNewMarketDataLaneRejectsBadConstruction(t *testing.T) {
	if _, err := events.NewMarketDataLane(0, events.NoCoalescing); !errors.Is(err, events.ErrLaneCapacity) {
		t.Errorf("depth 0: err = %v, want ErrLaneCapacity", err)
	}
	// A nil policy is refused rather than defaulted. Either default would be
	// wrong somewhere: defaulting to coalescing silently erases mark
	// excursions, and defaulting to none silently makes the lane a drop-oldest
	// ring. The caller states which.
	if _, err := events.NewMarketDataLane(1, nil); !errors.Is(err, events.ErrCoalescePolicyNil) {
		t.Errorf("nil policy: err = %v, want ErrCoalescePolicyNil", err)
	}
}

// --- [→ R2c-1]: an adverse excursion survives the recovery -------------------

// TestAdversePreservingKeepsTheBreach is the claim R2c-1 exists for, and the
// single most important test in this file.
//
// A rally spikes the mark against our short and then recovers, all before the
// OMS gets a turn. Under latest-wins the consumer sees only the recovered mark
// and never learns the threshold was breached. Under this rule it sees the spike.
func TestAdversePreservingKeepsTheBreach(t *testing.T) {
	l := markLane(t, 1)
	src := markSource()

	for i, price := range []int64{100, 180, 101} { // calm, spike, recovered
		if err := l.Publish(marketEnv(src, uint64(i+1), markPayload{price})); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	got, ok := l.Consume(src)
	if !ok {
		t.Fatal("Consume found nothing")
	}
	if p := got.Payload.(markPayload).price; p != 180 {
		t.Errorf("consumed mark %d, want the 180 spike — a recovery must not erase a breach [→ R2c-1]", p)
	}
}

// TestAdversePreservingKeepsARealObservation pins something easy to get wrong in
// a way nothing else would catch.
//
// The surviving envelope is the spike's OWN envelope — its sequence numbers and
// its RecvTime — not the newest envelope's metadata carrying the worst payload.
// The alternative would manufacture an observation that never happened: a price
// stamped at a time it did not hold. Freshness would look better and the
// consumer would be reading a fabricated tick.
//
// The cost of this choice is stated rather than hidden: while an unconsumed
// excursion sits in the mailbox, RecvTime is the excursion's, so the feed reads
// slightly staler than it is. That direction is safe — it can only trigger a
// freshness check, never suppress one — and the window is bounded by how often
// the consumer drains.
func TestAdversePreservingKeepsARealObservation(t *testing.T) {
	l := markLane(t, 1)
	src := markSource()

	spike := marketEnv(src, 2, markPayload{180})
	if err := l.Publish(marketEnv(src, 1, markPayload{100})); err != nil {
		t.Fatal(err)
	}
	if err := l.Publish(spike); err != nil {
		t.Fatal(err)
	}
	if err := l.Publish(marketEnv(src, 3, markPayload{101})); err != nil {
		t.Fatal(err)
	}

	got, _ := l.Consume(src)
	if got.AcceptOrdinal != spike.AcceptOrdinal || !got.RecvTime.Equal(spike.RecvTime) {
		t.Errorf("surviving envelope = ordinal %d at %v, want the spike's own (%d at %v); "+
			"a merged envelope must be a real observation, not the worst price wearing the newest timestamp",
			got.AcceptOrdinal, got.RecvTime, spike.AcceptOrdinal, spike.RecvTime)
	}
}

// TestAdversePreservingTakesAWorseMark is the other branch, and it must be
// reachable or the rule would be "ignore everything after the first".
func TestAdversePreservingTakesAWorseMark(t *testing.T) {
	l := markLane(t, 1)
	src := markSource()

	for i, price := range []int64{100, 150, 200} { // monotonically worse
		if err := l.Publish(marketEnv(src, uint64(i+1), markPayload{price})); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := l.Consume(src)
	if p := got.Payload.(markPayload).price; p != 200 {
		t.Errorf("consumed %d, want 200 — a worse mark must replace a less-bad one", p)
	}
	if got.AcceptOrdinal != 3 {
		t.Errorf("kept ordinal %d, want 3", got.AcceptOrdinal)
	}
}

// TestAdversePreservingWindowResetsOnConsumption is the "since the OMS last
// consumed it" half of R2c-1's sentence, and without it the rule would be a
// permanent high-water mark that never came down.
func TestAdversePreservingWindowResetsOnConsumption(t *testing.T) {
	l := markLane(t, 1)
	src := markSource()

	if err := l.Publish(marketEnv(src, 1, markPayload{180})); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.Consume(src); got.Payload.(markPayload).price != 180 {
		t.Fatal("setup: expected the spike")
	}

	// New window. The market genuinely recovered and the consumer has already
	// been told about the spike; holding it forever would keep de-risking
	// against a price that no longer exists.
	if err := l.Publish(marketEnv(src, 2, markPayload{101})); err != nil {
		t.Fatal(err)
	}
	got, ok := l.Consume(src)
	if !ok {
		t.Fatal("Consume found nothing in the new window")
	}
	if p := got.Payload.(markPayload).price; p != 101 {
		t.Errorf("consumed %d, want 101 — the adverse window resets once the consumer has seen it", p)
	}
}

// TestAdversePreservingRefusesUnknownPayloads is the conservative direction on
// the one thing a generic merge cannot check at compile time.
//
// If either payload is not the expected type, the merge declines rather than
// guessing. Declining keeps BOTH envelopes, so a wiring mistake costs memory
// pressure and a drop-oldest counter — visible — instead of silently discarding
// a mark it could not compare.
func TestAdversePreservingRefusesUnknownPayloads(t *testing.T) {
	l := markLane(t, 4)
	src := markSource()

	if err := l.Publish(marketEnv(src, 1, markPayload{180})); err != nil {
		t.Fatal(err)
	}
	if err := l.Publish(marketEnv(src, 2, "not a mark at all")); err != nil {
		t.Fatal(err)
	}

	if got := l.Len(src); got != 2 {
		t.Fatalf("mailbox holds %d, want 2 — an uncomparable payload must not be merged away", got)
	}
	first, _ := l.Consume(src)
	if p, ok := first.Payload.(markPayload); !ok || p.price != 180 {
		t.Errorf("first out = %v, want the 180 mark still intact", first.Payload)
	}
}

// TestMergeTargetsTheNewestEntry pins which queued envelope a MergeFunc is
// offered, in the only situation where it is observable.
//
// A coalescing mailbox normally holds one entry — every publish merges — so
// "newest" and "oldest" are the same slot and the distinction is invisible. It
// becomes visible only after a merge DECLINES, which is the one way such a
// mailbox grows: AdversePreserving refusing a payload it cannot compare.
//
// Then the difference is an envelope silently disappearing. Merging against the
// oldest entry compares the new mark against the 180 at the front, decides it is
// not worse, keeps the front — and the new mark is gone, having been neither
// queued nor counted as dropped.
//
// Found by a surviving mutant. Every earlier coalescing test used depth 1, which
// made this whole branch unreachable while looking like coverage.
func TestMergeTargetsTheNewestEntry(t *testing.T) {
	l := markLane(t, 4)
	src := markSource()

	// 180 goes in; the string cannot be compared, so it is appended alongside.
	if err := l.Publish(marketEnv(src, 1, markPayload{180})); err != nil {
		t.Fatal(err)
	}
	if err := l.Publish(marketEnv(src, 2, "not a mark")); err != nil {
		t.Fatal(err)
	}
	// This one must be offered against the STRING (the newest), be declined,
	// and be appended — not offered against the 180 and swallowed.
	if err := l.Publish(marketEnv(src, 3, markPayload{150})); err != nil {
		t.Fatal(err)
	}

	if got := l.Len(src); got != 3 {
		t.Fatalf("mailbox holds %d, want 3 — a merge is offered the NEWEST entry, and a declined merge appends", got)
	}
	if got := l.Dropped(src); got != 0 {
		t.Errorf("Dropped() = %d, want 0 — nothing overflowed, so nothing may go missing", got)
	}

	want := []any{markPayload{180}, "not a mark", markPayload{150}}
	for i, w := range want {
		got, ok := l.Consume(src)
		if !ok {
			t.Fatalf("mailbox emptied at %d, wanted %v", i, w)
		}
		if got.Payload != w {
			t.Errorf("position %d = %v, want %v", i, got.Payload, w)
		}
	}
}

// --- latest-wins, the ordinary rule ------------------------------------------

func TestLatestWinsCoalesces(t *testing.T) {
	l := markLane(t, 1)
	src := bookSource()

	for i := range 5 {
		if err := l.Publish(marketEnv(src, uint64(i+1), i)); err != nil {
			t.Fatal(err)
		}
	}
	if got := l.Len(src); got != 1 {
		t.Errorf("mailbox holds %d after 5 publishes, want 1 — superseded snapshots coalesce", got)
	}
	got, _ := l.Consume(src)
	if got.Payload != 4 {
		t.Errorf("consumed %v, want 4 (the newest)", got.Payload)
	}
}

// TestNoCoalescingRingDropsOldest is §3.4's "bounded depth + defined overflow
// (drop-oldest + freshness gap counter)".
//
// Drop-OLDEST rather than drop-newest, and the direction is the point: the
// newest update is the one closest to the truth, so refusing it to preserve
// history would leave the consumer reading the past. The counter is what turns
// the loss from silent into measured.
func TestNoCoalescingRingDropsOldest(t *testing.T) {
	l, err := events.NewMarketDataLane(3, events.NoCoalescing)
	if err != nil {
		t.Fatal(err)
	}
	src := bookSource()

	for i := range 5 {
		if err := l.Publish(marketEnv(src, uint64(i+1), i)); err != nil {
			t.Fatal(err)
		}
	}

	if got := l.Len(src); got != 3 {
		t.Errorf("mailbox holds %d, want the bounded 3", got)
	}
	if got := l.Dropped(src); got != 2 {
		t.Errorf("Dropped() = %d, want 2 — the gap must be counted, not merely survived", got)
	}
	// The three that remain are the three NEWEST, in order.
	for _, want := range []int{2, 3, 4} {
		got, ok := l.Consume(src)
		if !ok {
			t.Fatalf("mailbox emptied early, wanted payload %d", want)
		}
		if got.Payload != want {
			t.Errorf("consumed %v, want %d", got.Payload, want)
		}
	}
}

// --- the lane's own contract -------------------------------------------------

// TestMarketDataPublishNeverBlocks is the guarantee that separates this lane
// from the control lane. A market-data feed must never be able to stall the
// reader that also carries execution traffic.
func TestMarketDataPublishNeverBlocks(t *testing.T) {
	l, err := events.NewMarketDataLane(2, events.NoCoalescing)
	if err != nil {
		t.Fatal(err)
	}
	src := bookSource()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 10000 {
			if err := l.Publish(marketEnv(src, uint64(i+1), i)); err != nil {
				t.Errorf("publish %d: %v", i, err)
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked; the market-data lane must never apply back-pressure")
	}
}

// TestMarketDataMailboxesAreIsolated is the per-VenueInstrument requirement, and
// the reason spot and perp are distinct sources at all [→ Z1].
func TestMarketDataMailboxesAreIsolated(t *testing.T) {
	l := markLane(t, 1)

	perpMark := markSource()
	spotBook := events.SourceID{
		Instrument: ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
		},
		Stream: events.StreamBook,
	}

	if err := l.Publish(marketEnv(perpMark, 1, markPayload{180})); err != nil {
		t.Fatal(err)
	}
	if err := l.Publish(marketEnv(spotBook, 2, "spot book")); err != nil {
		t.Fatal(err)
	}

	// Each mailbox holds its own. A shared one would have coalesced the second
	// over the first under the mark's own policy, which is how a spot book
	// update would erase a perp mark excursion.
	got, ok := l.Consume(perpMark)
	if !ok || got.Payload.(markPayload).price != 180 {
		t.Errorf("perp mark mailbox = %v, want the 180 mark", got.Payload)
	}
	got, ok = l.Consume(spotBook)
	if !ok || got.Payload != "spot book" {
		t.Errorf("spot book mailbox = %v, want the spot book", got.Payload)
	}

	if n := len(l.Sources()); n != 2 {
		t.Errorf("Sources() = %d, want 2", n)
	}
}

// TestMarketDataRejectsControlTraffic is the misrouting guard.
//
// The package doc names this as the quiet failure the typed lanes exist to
// prevent: an execution event on the market-data lane gets coalesced away, and a
// fill that never arrives is a position the ledger does not know about. Routing
// is StreamKind.Lane's answer, and the lane refuses anything that answers
// differently rather than trusting the caller to have asked.
func TestMarketDataRejectsControlTraffic(t *testing.T) {
	l := markLane(t, 1)

	for _, stream := range []events.StreamKind{
		events.StreamOrders, events.StreamExecutions, events.StreamAccount, events.StreamControl,
	} {
		src := aSource()
		src.Stream = stream
		err := l.Publish(marketEnv(src, 1, "a fill"))
		if !errors.Is(err, events.ErrWrongLane) {
			t.Errorf("Publish(%s) = %v, want ErrWrongLane — control traffic must never be coalescible", stream, err)
		}
	}
}

// TestMarketDataConsumeUnknownSource — a consumer asking about a source nothing
// has published to gets "nothing", not a panic and not a zero envelope that
// reads like real data.
func TestMarketDataConsumeUnknownSource(t *testing.T) {
	l := markLane(t, 1)
	if e, ok := l.Consume(bookSource()); ok {
		t.Errorf("Consume on an unused source returned %v, want no envelope", e)
	}
	if got := l.Dropped(bookSource()); got != 0 {
		t.Errorf("Dropped() on an unused source = %d, want 0", got)
	}
	if got := l.Len(bookSource()); got != 0 {
		t.Errorf("Len() on an unused source = %d, want 0", got)
	}
}

// TestPeekReportsTheOldestWithoutRemovingIt pins both halves of Peek's
// contract at once, against a mailbox deep enough to make them distinct: with
// two envelopes queued, "oldest" and "newest" are different slots (a depth-1
// fixture would collapse that distinction — the checklist's standing lesson),
// and "did not remove" is observable as Consume still returning the same head
// afterwards.
func TestPeekReportsTheOldestWithoutRemovingIt(t *testing.T) {
	// NoCoalescing, not markLane: under markLane a book source is latest-wins,
	// so the second publish would MERGE into the first and the mailbox would
	// hold one envelope — the exact fixture collapse the depth-2 queue is here
	// to avoid.
	l, err := events.NewMarketDataLane(4, events.NoCoalescing)
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	src := bookSource()

	first := marketEnv(src, 1, "snapshot-1")
	second := marketEnv(src, 2, "snapshot-2")
	if err := l.Publish(first); err != nil {
		t.Fatalf("publish first: %v", err)
	}
	if err := l.Publish(second); err != nil {
		t.Fatalf("publish second: %v", err)
	}

	got, ok := l.Peek(src)
	if !ok {
		t.Fatal("Peek on a two-deep mailbox reported empty")
	}
	if got.AcceptOrdinal != first.AcceptOrdinal {
		t.Errorf("Peek returned ordinal %d, want the oldest (%d)", got.AcceptOrdinal, first.AcceptOrdinal)
	}
	if l.Len(src) != 2 {
		t.Errorf("Len after Peek = %d, want 2 — Peek must not remove", l.Len(src))
	}
	consumed, ok := l.Consume(src)
	if !ok || consumed.AcceptOrdinal != first.AcceptOrdinal {
		t.Errorf("Consume after Peek = (%d, %v), want the same head (%d, true)",
			consumed.AcceptOrdinal, ok, first.AcceptOrdinal)
	}
}

// TestPeekOnEmptyAndUnknownSources: an unused source and a drained one answer
// the same way, and neither materialises a mailbox as a side effect.
func TestPeekOnEmptyAndUnknownSources(t *testing.T) {
	l := markLane(t, 2)

	if _, ok := l.Peek(bookSource()); ok {
		t.Error("Peek on an unused source reported an envelope")
	}

	src := markSource()
	if err := l.Publish(marketEnv(src, 1, markPayload{100})); err != nil {
		t.Fatalf("publish: %v", err)
	}
	l.Consume(src)
	if _, ok := l.Peek(src); ok {
		t.Error("Peek on a drained mailbox reported an envelope")
	}
}

// TestMarketDataConcurrentPublishIsRaceFree runs under -race (make test).
//
// Note what this does NOT claim: that the coalesced RESULT is deterministic
// under concurrency. It cannot be — whether two updates coalesce depends on
// whether the consumer drained between them, which is scheduling. §3.5 is
// precise about this: the coalescing RULE is deterministic, keyed on
// (Source, field) rather than on scheduling, and the live arbitration is frozen
// into the persisted dispatch log, which is what replay re-applies. Claiming
// more than that here would be claiming something false.
func TestMarketDataConcurrentPublishIsRaceFree(t *testing.T) {
	l := markLane(t, 4)
	src := markSource()

	var wg sync.WaitGroup
	for p := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				if err := l.Publish(marketEnv(src, uint64(i+1), markPayload{int64(p*1000 + i)})); err != nil {
					t.Errorf("publish: %v", err)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 400 {
			l.Consume(src)
		}
	}()
	wg.Wait()
}
