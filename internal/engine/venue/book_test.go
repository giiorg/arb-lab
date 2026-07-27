package venue_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func spot() ids.VenueInstrument {
	// No Contract: ids.ErrContractOnSpot refuses a settlement type on a
	// non-perp, and §5.1's canonical form leaves it empty.
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

// TestBookSourceNamesTheBookStream is FundingSource's assertion applied to its
// mirror: what is checked is the StreamKind constant, not that the result
// validates.
//
// A version writing StreamMark here returns a source that validates identically
// for a perp — same instrument, and mark is a market-data stream too. Nothing
// about validity separates them. What separates them is §3.4's mailbox, and the
// symptom of getting it wrong is a book consumer that never hears from the
// instrument, which presents as a venue outage.
func TestBookSourceNamesTheBookStream(t *testing.T) {
	src, err := venue.BookSource(linearPerp())
	if err != nil {
		t.Fatalf("BookSource(perp) = %v, want nil", err)
	}
	if src.Stream != events.StreamBook {
		t.Errorf("Stream = %q, want %q — the mailbox key is the routing decision", src.Stream, events.StreamBook)
	}
	if src.Instrument != linearPerp() {
		t.Errorf("Instrument = %v, want %v", src.Instrument, linearPerp())
	}
	if got := src.Lane(); got != events.LaneMarketData {
		t.Errorf("Lane() = %v, want LaneMarketData", got)
	}
}

// TestBookSourceAcceptsSpot is the case that makes BookSource a different
// function from FundingSource rather than a copy of it.
//
// §3.5's product table gives StreamBook to both families and StreamFunding to
// perps alone, and §4.2 makes the spot book how the spot leg is priced. So this
// asserting NIL is the whole claim: a constructor written by copying
// FundingSource and swapping the token would pass every other test in this file
// and would silently unsubscribe half of every cash-and-carry trade — the
// failure §3.5 calls out by name when it explains why the constraint lives on
// SourceID.
//
// The perp case above is not enough on its own. Both constructors accept a perp,
// so a fixture that only ever passes one tests nothing about the asymmetry.
func TestBookSourceAcceptsSpot(t *testing.T) {
	src, err := venue.BookSource(spot())
	if err != nil {
		t.Fatalf("BookSource(spot) = %v, want nil — a spot book is how §4.2 prices the spot leg", err)
	}
	if src.Instrument.Product != ids.ProductSpot {
		t.Errorf("Product = %q, want %q", src.Instrument.Product, ids.ProductSpot)
	}

	// The control, at the same call: funding on that same spot instrument MUST
	// be refused. Without it, a RequiresPerp that returned false for everything
	// would make the assertion above pass for the wrong reason.
	if _, err := venue.FundingSource(spot()); !errors.Is(err, events.ErrStreamProductMismatch) {
		t.Errorf("FundingSource(spot) = %v, want ErrStreamProductMismatch", err)
	}
}

// TestBookSourceRefusesAMalformedInstrument checks that the remaining refusal is
// still made, and that it is events' error rather than a renamed local one — a
// caller branching on the rule's owner keeps working when a third stream
// constructor lands beside these two.
func TestBookSourceRefusesAMalformedInstrument(t *testing.T) {
	if _, err := venue.BookSource(ids.VenueInstrument{}); !errors.Is(err, ids.ErrVenueUnset) {
		t.Errorf("BookSource(zero) = %v, want ErrVenueUnset", err)
	}
}

// ---------------------------------------------------------------------------
// The book payloads
// ---------------------------------------------------------------------------

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

func level(t *testing.T, p, q string) venue.BookLevel {
	t.Helper()
	return venue.BookLevel{Price: price(t, p), Quantity: qty(t, q)}
}

// bookSnap is the fixture, and two of its dimensions are chosen against a
// distinction the code makes rather than for realism.
//
// THREE levels a side, not one: Validate walks both halves in one loop and
// reports an index, so a single-level fixture makes "reports the index it
// failed at" and "reports 0" the same answer.
//
// The two sides carry DIFFERENT prices, which matters to the adapter that
// consumes this rather than to Validate — a fixture whose halves were
// interchangeable would let a translation assign asks to bids without any
// assertion noticing.
func bookSnap(t *testing.T) venue.BookSnapshot {
	t.Helper()
	return venue.BookSnapshot{
		Bids: []venue.BookLevel{
			level(t, "43250.10", "0.5"), level(t, "43250.00", "1.25"), level(t, "43249.50", "2"),
		},
		Asks: []venue.BookLevel{
			level(t, "43251.00", "0.75"), level(t, "43251.50", "1.5"), level(t, "43252.00", "3"),
		},
	}
}

// TestBookSnapshotFixtureIsValid is the positive control. Without it, every
// refusal below is satisfied by a Validate that objects to everything.
func TestBookSnapshotFixtureIsValid(t *testing.T) {
	if err := bookSnap(t).Validate(); err != nil {
		t.Fatalf("the fixture does not validate: %v", err)
	}
}

func TestBookSnapshotRejects(t *testing.T) {
	// Every case puts its bad level on the ASK side at index 2 — the last of
	// three — and both choices are deliberate. Validate walks bids first, so a
	// fault planted on the bid side at index 0 is found by a loop that never
	// reaches the second half and by a report that always says zero: "names where
	// it failed" and "names the first thing it looked at" would be the same
	// answer. The message assertion is what turns that into a claim.
	cases := map[string]struct {
		bad  venue.BookLevel
		want error
	}{
		"zero quantity": {
			// The removal encoding, which a SNAPSHOT cannot express. This is the
			// case that separates BookLevel from LevelChange, and it is the one a
			// single shared type would admit.
			venue.BookLevel{Price: price(t, "43252.00"), Quantity: qty(t, "0")},
			venue.ErrLevelQuantityNotPositive,
		},
		"negative quantity": {
			venue.BookLevel{Price: price(t, "43252.00"), Quantity: qty(t, "-1")},
			venue.ErrLevelQuantityNotPositive,
		},
		"unpopulated price": {
			// The zero value of a Price carries no asset tag. A level nobody
			// filled in must not read as free size.
			venue.BookLevel{Quantity: qty(t, "1")},
			fixedpoint.ErrAssetUnset,
		},
		"unpopulated quantity": {
			venue.BookLevel{Price: price(t, "43252.00")},
			fixedpoint.ErrAssetUnset,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := bookSnap(t)
			snap.Asks = append([]venue.BookLevel(nil), snap.Asks...)
			snap.Asks[2] = tc.bad

			err := snap.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
			if got := err.Error(); !strings.Contains(got, "ask level 2") {
				t.Errorf("Validate = %q, want it to name \"ask level 2\" — a report that "+
					"cannot say which half and which level sends a reader to the wrong slice", got)
			}
		})
	}
}

// TestBookSnapshotAcceptsAnEmptyBook pins domain.BookSnapshot's argument at this
// seam: a halted instrument publishes nothing, and refusing emptiness would make
// a delta that removes the last level unable to produce a valid result.
func TestBookSnapshotAcceptsAnEmptyBook(t *testing.T) {
	if err := (venue.BookSnapshot{}).Validate(); err != nil {
		t.Errorf("empty book = %v, want nil", err)
	}
	if err := (venue.BookSnapshot{Bids: bookSnap(t).Bids}).Validate(); err != nil {
		t.Errorf("one-sided book = %v, want nil — a side can legitimately be empty", err)
	}
}

// TestBookSnapshotDoesNotCheckTheBookInvariants pins a deliberate OMISSION, which
// is the only way an omission stays a decision instead of decaying into a gap.
//
// Both inputs here are books no venue could publish — one side sorted backwards,
// and a best bid above the best ask — and both pass. That is the documented
// contract: the ordering and cross rules belong to domain.BookSnapshot, which
// runs them on every snapshot before any of it is used, and restating them here
// would be a second copy of a comparison rule in a package that cannot import
// the one that owns it.
//
// If this test ever fails, the two copies exist. That is the event it is for.
func TestBookSnapshotDoesNotCheckTheBookInvariants(t *testing.T) {
	backwards := venue.BookSnapshot{
		Bids: []venue.BookLevel{level(t, "43249.50", "2"), level(t, "43250.10", "0.5")},
	}
	if err := backwards.Validate(); err != nil {
		t.Errorf("ascending bids = %v, want nil; the ordering rule is the domain's", err)
	}

	crossed := venue.BookSnapshot{
		Bids: []venue.BookLevel{level(t, "43260.00", "1")},
		Asks: []venue.BookLevel{level(t, "43250.00", "1")},
	}
	if err := crossed.Validate(); err != nil {
		t.Errorf("crossed book = %v, want nil; the cross rule is the domain's", err)
	}
}

func TestLevelChangeAcceptsAZeroQuantity(t *testing.T) {
	// The removal instruction, which BookLevel refuses and this must admit. The
	// two types exist for this one value; a test asserting only the refusals
	// above would pass against a LevelChange that was a BookLevel by another name.
	c := venue.LevelChange{Side: venue.SideBid, Price: price(t, "43250.00"), Quantity: qty(t, "0")}
	if err := c.Validate(); err != nil {
		t.Errorf("zero-quantity change = %v, want nil — zero is the removal encoding", err)
	}
}

func TestLevelChangeRejects(t *testing.T) {
	ok := venue.LevelChange{Side: venue.SideAsk, Price: price(t, "43251.00"), Quantity: qty(t, "1")}

	cases := map[string]struct {
		mutate func(*venue.LevelChange)
		want   error
	}{
		"negative quantity": {
			func(c *venue.LevelChange) { c.Quantity = qty(t, "-1") },
			venue.ErrChangeQuantityNegative,
		},
		"unset side": {
			func(c *venue.LevelChange) { c.Side = "" },
			venue.ErrSideUnset,
		},
		"unknown side": {
			// A venue vocabulary this seam has not been taught. Separate from
			// unset because it sends a reader somewhere else — to the Profiles
			// entry that should have recorded the spelling.
			func(c *venue.LevelChange) { c.Side = "buy" },
			venue.ErrSideUnknown,
		},
		"unpopulated price": {
			func(c *venue.LevelChange) { c.Price = fixedpoint.Price{} },
			fixedpoint.ErrAssetUnset,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := ok
			tc.mutate(&c)
			if err := c.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestBookDeltaAcceptsAHeartbeat(t *testing.T) {
	// An empty delta advances the sequence and nothing else. Refusing it would
	// manufacture a gap out of a healthy feed — the reconstructor would see the
	// next real delta skip the heartbeat's span and resnapshot.
	d := venue.BookDelta{FirstSeq: 1004}
	if err := d.Validate(); err != nil {
		t.Errorf("empty delta = %v, want nil — a heartbeat is a real message", err)
	}
}

func TestBookDeltaRejectsAnUnsetFirstSequence(t *testing.T) {
	// The one end of the span this payload holds. A zero is a dropped field, not
	// a venue counting from zero, and left through it makes every delta claim a
	// span beginning at 1 — an enormous overlap against any live book.
	d := venue.BookDelta{
		FirstSeq: 0,
		Changes:  []venue.LevelChange{{Side: venue.SideBid, Price: price(t, "43250.00"), Quantity: qty(t, "1")}},
	}
	if err := d.Validate(); !errors.Is(err, venue.ErrDeltaFirstSeqUnset) {
		t.Errorf("Validate = %v, want ErrDeltaFirstSeqUnset", err)
	}
}

func TestBookDeltaReportsWhichChangeFailed(t *testing.T) {
	// The bad change is the SECOND of two, for the reason the snapshot cases
	// state: with one change, "names the index" and "names zero" agree.
	d := venue.BookDelta{
		FirstSeq: 1004,
		Changes: []venue.LevelChange{
			{Side: venue.SideBid, Price: price(t, "43250.00"), Quantity: qty(t, "1")},
			{Side: venue.SideAsk, Price: price(t, "43251.00"), Quantity: qty(t, "-1")},
		},
	}
	err := d.Validate()
	if !errors.Is(err, venue.ErrChangeQuantityNegative) {
		t.Fatalf("Validate = %v, want ErrChangeQuantityNegative", err)
	}
	if got := err.Error(); !strings.Contains(got, "change 1") {
		t.Errorf("Validate = %q, want it to name \"change 1\"", got)
	}
}
