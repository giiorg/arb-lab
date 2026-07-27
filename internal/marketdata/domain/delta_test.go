package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func change(t *testing.T, side domain.BookSide, p, q string) domain.LevelChange {
	t.Helper()
	return domain.LevelChange{Side: side, Price: price(t, p), Quantity: qty(t, q)}
}

// delta is the fixture update, and its span covers MORE THAN ONE sequence on
// purpose.
//
// A single-sequence delta (First == Last) cannot be an overlap — one number
// cannot be both at-or-before the book's position and past it — so a fixture
// built from single-sequence venues tests three of Follows' four relations while
// appearing to test all four. Binance's depth events really do span several
// sequences, so this is the honest shape as well as the discriminating one.
func delta(t *testing.T, first, last uint64) domain.BookDelta {
	t.Helper()
	return domain.BookDelta{
		Instrument: perp(),
		Changes: []domain.LevelChange{
			change(t, domain.SideBid, "99", "20"),
			change(t, domain.SideAsk, "102", "30"),
		},
		FirstSeq:  first,
		LastSeq:   last,
		Watermark: bookWm(last),
	}
}

func TestLevelChangeValidate(t *testing.T) {
	t.Parallel()

	if err := change(t, domain.SideBid, "100", "1").Validate(); err != nil {
		t.Fatalf("the fixture change must be valid: %v", err)
	}

	// Zero is LEGAL here and refused on a BookLevel, and asserting the
	// asymmetry is the point: it is the whole encoding of a removal, and a
	// simplification that shared one validator between the two types would take
	// it away while every other case still passed.
	if err := change(t, domain.SideAsk, "100", "0").Validate(); err != nil {
		t.Errorf("zero quantity = %v, want nil — zero is how a change says REMOVE", err)
	}

	cases := []struct {
		name string
		c    domain.LevelChange
		want error
	}{
		{"negative quantity", change(t, domain.SideBid, "100", "-1"), domain.ErrChangeQuantityNegative},
		{"side unset", domain.LevelChange{Price: price(t, "100"), Quantity: qty(t, "1")}, domain.ErrSideUnset},
		{"side unknown", change(t, "buy", "100", "1"), domain.ErrSideUnknown},
		{"price unset", domain.LevelChange{Side: domain.SideBid, Quantity: qty(t, "1")}, fixedpoint.ErrAssetUnset},
		{"quantity unset", domain.LevelChange{Side: domain.SideBid, Price: price(t, "100")}, fixedpoint.ErrAssetUnset},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := c.c.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestLevelChangeRemoves(t *testing.T) {
	t.Parallel()

	// Both directions, because `sign == 0` and `sign <= 0` agree on every
	// non-negative fixture — and a negative quantity never reaches here, being
	// refused by Validate. The positive case is what separates them.
	for _, c := range []struct {
		q    string
		want bool
	}{{"0", true}, {"1", false}} {
		got, err := change(t, domain.SideBid, "100", c.q).Removes()
		if err != nil {
			t.Fatalf("Removes() = %v, want nil", err)
		}
		if got != c.want {
			t.Errorf("quantity %s: Removes() = %v, want %v", c.q, got, c.want)
		}
	}
}

func TestBookDeltaValidate(t *testing.T) {
	t.Parallel()

	if err := delta(t, 1001, 1005).Validate(); err != nil {
		t.Fatalf("the fixture delta must be valid: %v", err)
	}

	// An empty change list with a span is a HEARTBEAT, and it is valid. Refusing
	// it would manufacture a sequence gap out of a healthy feed, which is the
	// one failure this whole file exists to detect.
	hb := delta(t, 1001, 1005)
	hb.Changes = nil
	if err := hb.Validate(); err != nil {
		t.Errorf("heartbeat delta = %v, want nil", err)
	}

	// A single-sequence span is valid too — that is Bybit's shape, and the
	// FIRST-equals-LAST case is exactly the one an inverted-range check written
	// as `<=` would refuse.
	if err := delta(t, 1001, 1001).Validate(); err != nil {
		t.Errorf("single-sequence delta = %v, want nil", err)
	}

	cases := []struct {
		name string
		mut  func(*testing.T, *domain.BookDelta)
		want error
	}{
		{
			"instrument unset",
			func(_ *testing.T, d *domain.BookDelta) { d.Instrument = ids.VenueInstrument{} },
			ids.ErrVenueUnset,
		},
		{
			"watermark unset",
			func(_ *testing.T, d *domain.BookDelta) { d.Watermark = domain.Watermark{} },
			domain.ErrWatermarkUnset,
		},
		{
			"no sequence at all",
			func(_ *testing.T, d *domain.BookDelta) { d.FirstSeq, d.LastSeq, d.Watermark = 0, 0, bookWm(0) },
			domain.ErrDeltaSeqUnset,
		},
		{
			// LastSeq exactly one BELOW FirstSeq: the minimal inversion, which
			// is what separates `LastSeq < FirstSeq` from a check written
			// against a wider margin.
			"span ends before it starts",
			func(_ *testing.T, d *domain.BookDelta) { d.LastSeq, d.Watermark = 1000, bookWm(1000) },
			domain.ErrDeltaSeqInverted,
		},
		{
			// The two spellings of one fact, disagreeing by one. Off by one
			// rather than wildly, because a fixture using 0 or a huge number
			// would also trip the unset check or read as an obvious typo.
			"watermark sequence disagrees with the span",
			func(_ *testing.T, d *domain.BookDelta) { d.Watermark = bookWm(1004) },
			domain.ErrDeltaSeqDisagrees,
		},
		{
			"a change is malformed",
			func(t *testing.T, d *domain.BookDelta) {
				d.Changes[1] = change(t, domain.SideAsk, "102", "-1")
			},
			domain.ErrChangeQuantityNegative,
		},
		{
			"one level changed twice",
			func(t *testing.T, d *domain.BookDelta) {
				d.Changes = append(d.Changes, change(t, domain.SideBid, "99", "40"))
			},
			domain.ErrDeltaDuplicateLevel,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := delta(t, 1001, 1005)
			c.mut(t, &d)
			if err := d.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestBookDeltaOneLevelPerSideIsNotADuplicate is the positive control for the
// duplicate scan, and it is a separate test because it is the case that makes
// the scan's side guard load-bearing.
//
// A bid and an ask at the SAME price in one message is legal and ordinary: it is
// what a venue sends when the market moves through a level, removing the ask and
// resting a bid there. A duplicate check that compared prices without comparing
// sides would refuse it, and every case in the table above would still pass.
func TestBookDeltaOneLevelPerSideIsNotADuplicate(t *testing.T) {
	t.Parallel()

	d := delta(t, 1001, 1005)
	d.Changes = []domain.LevelChange{
		change(t, domain.SideAsk, "101", "0"),
		change(t, domain.SideBid, "101", "7"),
	}
	if err := d.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil — one price on two sides is not a duplicate", err)
	}
}

func TestSeqRelationValidate(t *testing.T) {
	t.Parallel()

	// The zero value is a DECLARED value here, unlike a BookSide's: it is the
	// answer for a book with no position, not an unpopulated field.
	for _, r := range []domain.SeqRelation{
		domain.SeqRelationUnknown, domain.SeqRelationStale,
		domain.SeqRelationContiguous, domain.SeqRelationOverlap, domain.SeqRelationGap,
	} {
		if err := r.Validate(); err != nil {
			t.Errorf("%s.Validate() = %v, want nil", r, err)
		}
		if got := r.String(); got == "" || got[0] != 'S' {
			t.Errorf("String() = %q, want a named value", got)
		}
	}

	// One past the highest declared value — the boundary that separates `>` from
	// `>=` in Validate. A fixture using 200 would pass against either.
	beyond := domain.SeqRelationGap + 1
	if err := beyond.Validate(); !errors.Is(err, domain.ErrSeqRelationInvalid) {
		t.Errorf("SeqRelation(%d).Validate() = %v, want ErrSeqRelationInvalid", uint8(beyond), err)
	}
	if got := beyond.String(); got != "SeqRelation(5)" {
		t.Errorf("String() = %q, want SeqRelation(5)", got)
	}
}

// TestBookDeltaFollows walks every boundary of the four-way relation.
//
// The book sits at sequence 1000 throughout, and each case is placed ONE
// sequence from the case beside it. That is deliberate: a table whose gap case
// started at 1100 would pass equally against `FirstSeq > prev+1`,
// `FirstSeq > prev+50`, and `FirstSeq != prev+1`, and only the adjacent cases
// tell those apart.
func TestBookDeltaFollows(t *testing.T) {
	t.Parallel()

	const prev = 1000

	cases := []struct {
		name        string
		prev        uint64
		first, last uint64
		want        domain.SeqRelation
	}{
		{"book has no position", 0, 1001, 1005, domain.SeqRelationUnknown},

		{"entirely behind", prev, 990, 995, domain.SeqRelationStale},
		{"ends exactly at the book", prev, 995, 1000, domain.SeqRelationStale},

		{"starts exactly after the book", prev, 1001, 1005, domain.SeqRelationContiguous},
		{"single sequence, exactly next", prev, 1001, 1001, domain.SeqRelationContiguous},

		// Overlap needs a multi-sequence span by construction; both boundaries
		// of it are here. Starting AT the book's position and starting before it
		// are different sides of the `FirstSeq <= prev` comparison.
		{"starts at the book and runs past", prev, 1000, 1005, domain.SeqRelationOverlap},
		{"starts before the book and runs past", prev, 990, 1005, domain.SeqRelationOverlap},

		// The minimal gap: exactly one sequence missing. Anything larger would
		// not distinguish the comparison's constant.
		{"one sequence missing", prev, 1002, 1005, domain.SeqRelationGap},
		{"single sequence, one missing before it", prev, 1002, 1002, domain.SeqRelationGap},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := delta(t, c.first, c.last)
			if got := d.Follows(c.prev); got != c.want {
				t.Errorf("Follows(%d) on span %d..%d = %s, want %s", c.prev, c.first, c.last, got, c.want)
			}
		})
	}
}

func TestApplyUpdatesInsertsAndRemoves(t *testing.T) {
	t.Parallel()

	b := book(t)
	d := delta(t, 1001, 1005)
	// Front, middle, back on both sides, plus a removal. The INSERT positions
	// are what matter: applyChange shifts the tail to make room, and an insert
	// at the end shifts nothing — so a fixture that only appends would leave the
	// shift entirely unexecuted while reporting the insert path as covered.
	d.Changes = []domain.LevelChange{
		change(t, domain.SideBid, "99.5", "11"),  // insert at the front-ish (index 1)
		change(t, domain.SideBid, "95", "12"),    // insert at the very end
		change(t, domain.SideBid, "98", "13"),    // update in place
		change(t, domain.SideAsk, "100.5", "14"), // insert at index 0, shifting everything
		change(t, domain.SideAsk, "103", "0"),    // remove from the middle
	}

	got, err := b.Apply(d)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}

	wantBids := []struct{ price, qty string }{
		{"100", "1"}, {"99.5", "11"}, {"99", "2"}, {"98", "13"}, {"97", "4"}, {"96", "5"}, {"95", "12"},
	}
	if len(got.Bids) != len(wantBids) {
		t.Fatalf("bid count = %d, want %d: %v", len(got.Bids), len(wantBids), got.Bids)
	}
	for i, w := range wantBids {
		if p := got.Bids[i].Price.Decimal(); p != w.price {
			t.Errorf("bid %d price = %s, want %s", i, p, w.price)
		}
		if q := got.Bids[i].Quantity.Decimal(); q != w.qty {
			t.Errorf("bid %d quantity = %s, want %s", i, q, w.qty)
		}
	}

	wantAsks := []struct{ price, qty string }{
		{"100.5", "14"}, {"101", "6"}, {"102", "7"}, {"104", "9"}, {"105", "10"},
	}
	if len(got.Asks) != len(wantAsks) {
		t.Fatalf("ask count = %d, want %d: %v", len(got.Asks), len(wantAsks), got.Asks)
	}
	for i, w := range wantAsks {
		if p := got.Asks[i].Price.Decimal(); p != w.price {
			t.Errorf("ask %d price = %s, want %s", i, p, w.price)
		}
		if q := got.Asks[i].Quantity.Decimal(); q != w.qty {
			t.Errorf("ask %d quantity = %s, want %s", i, q, w.qty)
		}
	}

	// The result takes the DELTA's provenance, which is what makes the next
	// relation computable against it.
	if got.Watermark.VenueSeq != 1005 {
		t.Errorf("result VenueSeq = %d, want 1005 — the book's position must advance", got.Watermark.VenueSeq)
	}
}

// TestApplyDoesNotMutateTheReceiver is the immutability claim, and §3.4's
// "reconstructed immutable snapshot" is the requirement it pins.
//
// It exercises all three mutation kinds, because they fail differently: an
// update writes through a shared backing array, a remove reslices one, and an
// insert appends to one — and an implementation can get any two right while
// aliasing on the third. The original is captured by VALUE before the call and
// compared level by level after it, because comparing the slice headers would
// not notice a write through a shared array.
func TestApplyDoesNotMutateTheReceiver(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		change domain.LevelChange
	}{
		{"update", change(t, domain.SideBid, "99", "999")},
		{"remove", change(t, domain.SideBid, "99", "0")},
		{"insert", change(t, domain.SideBid, "99.5", "999")},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			b := book(t)
			before := book(t) // an independent copy, built the same way

			d := delta(t, 1001, 1005)
			d.Changes = []domain.LevelChange{c.change}
			if _, err := b.Apply(d); err != nil {
				t.Fatalf("Apply() = %v, want nil", err)
			}

			if len(b.Bids) != len(before.Bids) {
				t.Fatalf("receiver bid count changed: %d, want %d", len(b.Bids), len(before.Bids))
			}
			for i := range before.Bids {
				if b.Bids[i].Price.Decimal() != before.Bids[i].Price.Decimal() ||
					b.Bids[i].Quantity.Decimal() != before.Bids[i].Quantity.Decimal() {
					t.Errorf("receiver bid %d became %v, want %v — Apply must not mutate the book it was given",
						i, b.Bids[i], before.Bids[i])
				}
			}
			if b.Watermark.VenueSeq != before.Watermark.VenueSeq {
				t.Errorf("receiver VenueSeq = %d, want %d", b.Watermark.VenueSeq, before.Watermark.VenueSeq)
			}
		})
	}
}

// TestApplyValidatesOnlyAfterEveryChange pins the ordering claim in Apply's doc
// comment, and the ORDER OF THE CHANGES is the whole fixture.
//
// The crossing change comes FIRST: a bid rests at 101, which is the current best
// ask, and the same message removes that ask. Applied together the book is
// clean. Checked after each individual change, the book is crossed at change 0
// and a correct feed is refused. Put the removal first instead and both
// implementations pass — which is precisely the pinned-fixture failure this
// project has hit nine times.
func TestApplyValidatesOnlyAfterEveryChange(t *testing.T) {
	t.Parallel()

	b := book(t)
	d := delta(t, 1001, 1005)
	d.Changes = []domain.LevelChange{
		change(t, domain.SideBid, "101", "5"), // crosses, on its own
		change(t, domain.SideAsk, "101", "0"), // uncrosses
	}

	got, err := b.Apply(d)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil — a level crossing the spread within one message is ordinary", err)
	}
	bid, ok := got.BestBid()
	if !ok || bid.Price.Decimal() != "101" {
		t.Errorf("best bid = %v (ok=%v), want 101", bid.Price, ok)
	}
	ask, ok := got.BestAsk()
	if !ok || ask.Price.Decimal() != "102" {
		t.Errorf("best ask = %v (ok=%v), want 102", ask.Price, ok)
	}
}

func TestApplyRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*testing.T, *domain.BookSnapshot, *domain.BookDelta)
		want error
	}{
		{
			"delta names another instrument",
			func(_ *testing.T, _ *domain.BookSnapshot, d *domain.BookDelta) {
				// The SPOT book of the same symbol on the same venue, which is
				// the mismatch worth testing: §5.1 makes it a distinct
				// instrument, its levels are price-plausible against the perp's,
				// and a book that silently accepted its deltas would produce a
				// spread nobody could tell from a real one.
				d.Instrument = ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
				}
			},
			domain.ErrBookInstrumentMismatch,
		},
		{
			"the book itself is invalid",
			func(t *testing.T, b *domain.BookSnapshot, _ *domain.BookDelta) {
				b.Bids[2] = level(t, "99.5", "3")
			},
			domain.ErrBookUnordered,
		},
		{
			"the delta itself is invalid",
			func(_ *testing.T, _ *domain.BookSnapshot, d *domain.BookDelta) {
				d.Watermark = bookWm(1004)
			},
			domain.ErrDeltaSeqDisagrees,
		},
		{
			// The desync detector. A delta that rests a bid above the best ask
			// and does NOT clear that ask has diverged from the venue, and the
			// application layer treats this exactly as it treats a gap.
			"the result would be crossed",
			func(t *testing.T, _ *domain.BookSnapshot, d *domain.BookDelta) {
				d.Changes = []domain.LevelChange{change(t, domain.SideBid, "103", "5")}
			},
			domain.ErrBookCrossed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := book(t)
			d := delta(t, 1001, 1005)
			c.mut(t, &b, &d)
			if _, err := b.Apply(d); !errors.Is(err, c.want) {
				t.Errorf("Apply() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestApplyRemovingAnAbsentLevelIsANoOp pins the resync behaviour: a venue
// deleting a level our starting snapshot never contained is ordinary, not a
// desync. Refusing it would turn the normal end of every resnapshot into a
// resnapshot.
func TestApplyRemovingAnAbsentLevelIsANoOp(t *testing.T) {
	t.Parallel()

	b := book(t)
	d := delta(t, 1001, 1005)
	d.Changes = []domain.LevelChange{change(t, domain.SideBid, "42", "0")}

	got, err := b.Apply(d)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}
	if len(got.Bids) != len(b.Bids) {
		t.Errorf("bid count = %d, want %d unchanged", len(got.Bids), len(b.Bids))
	}
}

// TestApplyToAnEmptyBook covers locate's degenerate input and the state a
// reconstructor starts from after a venue publishes an empty side.
func TestApplyToAnEmptyBook(t *testing.T) {
	t.Parallel()

	b := book(t)
	b.Bids, b.Asks = nil, nil

	d := delta(t, 1001, 1005)
	d.Changes = []domain.LevelChange{
		change(t, domain.SideBid, "100", "1"),
		change(t, domain.SideAsk, "101", "2"),
		change(t, domain.SideAsk, "99", "0"), // remove from an empty side
	}
	got, err := b.Apply(d)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}
	if len(got.Bids) != 1 || len(got.Asks) != 1 {
		t.Fatalf("book = %d bids / %d asks, want 1 / 1", len(got.Bids), len(got.Asks))
	}
}

// TestApplyEmptyingASideKeepsTheBookValid is the case that decided emptiness is
// legal rather than refused: Apply is a total function over legal inputs, and a
// delta clearing the last level must not turn it into a partial one.
func TestApplyEmptyingASideKeepsTheBookValid(t *testing.T) {
	t.Parallel()

	b := book(t)
	d := delta(t, 1001, 1005)
	var clear []domain.LevelChange
	for _, l := range b.Bids {
		clear = append(clear, domain.LevelChange{
			Side: domain.SideBid, Price: l.Price, Quantity: qty(t, "0"),
		})
	}
	d.Changes = clear

	got, err := b.Apply(d)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil — a book with one side empty is a state, not a defect", err)
	}
	if len(got.Bids) != 0 {
		t.Errorf("bids = %d, want 0", len(got.Bids))
	}
	if _, ok := got.BestBid(); ok {
		t.Error("BestBid ok = true on an empty side, want false")
	}
}
