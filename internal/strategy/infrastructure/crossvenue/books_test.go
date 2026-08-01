package crossvenue

// Book-cost walker battery. Fixture discipline: the two books differ
// between the open and close boundaries — identical books would collapse
// "worse of the two boundary walks" into "either boundary"; the bid and
// ask numbers differ so WorseSide has a real selection; and the Bybit
// break fixture jumps u by MORE than one mid-file, exercising the
// unanchored stretch between a break and the re-anchoring snapshot.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

var bnBookKeyFix = capture.StreamKey{Source: events.SourceID{Instrument: bnInstFix, Stream: events.StreamBook}, Topic: "abcusdt@depth20@100ms"}
var byBookKeyFix = capture.StreamKey{Source: events.SourceID{Instrument: byInstFix, Stream: events.StreamBook}, Topic: "orderbook.50.ABCUSDT"}

func bnBookRel(name string) string {
	return "v1/binance/linear_perp/linear/ABCUSDT/book/abcusdt%40depth20%40100ms/2026/07/30/" + name
}

func byBookRel(name string) string {
	return "v1/bybit/linear_perp/linear/ABCUSDT/book/orderbook.50.ABCUSDT/2026/07/30/" + name
}

func levelsJSON(levels [][2]string) string {
	parts := make([]string, len(levels))
	for i, l := range levels {
		parts[i] = fmt.Sprintf(`[%q,%q]`, l[0], l[1])
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func bnDepthFrame(recv time.Time, u uint64, bids, asks [][2]string) string {
	return fmt.Sprintf(`{"e":"depthUpdate","E":%d,"s":"ABCUSDT","U":%d,"u":%d,"pu":%d,"b":%s,"a":%s}`,
		recv.UnixMilli(), u-5, u, u-6, levelsJSON(bids), levelsJSON(asks))
}

func byBookSnapFrame(recv time.Time, u uint64, bids, asks [][2]string) string {
	return fmt.Sprintf(`{"topic":"orderbook.50.ABCUSDT","type":"snapshot","ts":%d,"data":{"s":"ABCUSDT","b":%s,"a":%s,"u":%d,"seq":%d}}`,
		recv.UnixMilli(), levelsJSON(bids), levelsJSON(asks), u, u*10)
}

func byBookDeltaFrame(recv time.Time, u uint64, bids, asks [][2]string) string {
	return fmt.Sprintf(`{"topic":"orderbook.50.ABCUSDT","type":"delta","ts":%d,"data":{"s":"ABCUSDT","b":%s,"a":%s,"u":%d,"seq":%d}}`,
		recv.UnixMilli(), levelsJSON(bids), levelsJSON(asks), u, u*10)
}

var bookP = SampleParams{Every: 5 * time.Second, Slack: 60 * time.Second}

func abcQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("ABC", s)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// Book A: ask walk of 2 costs (201-200)/200 = 0.005, bid walk costs
// (198-188.10)/198 = 0.05. Book B: ask (203-200)/200 = 0.015, bid
// (198-194.04)/198 = 0.02.
func bookAFixture() ([][2]string, [][2]string) {
	return [][2]string{{"99.00", "1"}, {"89.10", "1"}}, [][2]string{{"100.00", "1"}, {"101.00", "1"}}
}

func bookBFixture() ([][2]string, [][2]string) {
	return [][2]string{{"99.00", "1"}, {"95.04", "1"}}, [][2]string{{"100.00", "1"}, {"103.00", "1"}}
}

// The Binance walk: distributions across instants, boundary walks at
// wanted instants, and the two-boundary difference visible in AtWanted.
func TestWalkBinanceBookCostsSamplesAndWanted(t *testing.T) {
	root := t.TempDir()
	aB, aA := bookAFixture()
	bB, bA := bookBFixture()
	f1 := sampleBase.Add(8 * time.Second)
	f2 := sampleBase.Add(18 * time.Second)
	f3 := sampleBase.Add(28 * time.Second)
	writeFixture(t, filepath_Join(root, bnBookRel("2026-07-30T10.arbcap")), false,
		dataAt(f1, bnDepthFrame(f1, 100, aB, aA)),
		dataAt(f2, bnDepthFrame(f2, 110, bB, bA)),
		dataAt(f3, bnDepthFrame(f3, 120, bB, bA)),
	)
	writeManifest(t, root, sampleDay, bnBookRel("2026-07-30T10.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	open := sampleBase.Add(10 * time.Second)
	closeAt := sampleBase.Add(20 * time.Second)
	cs, err := WalkBinanceBookCosts(root, days, bnBookKeyFix, bnInstFix, bookP, abcQty(t, "2"), []time.Time{open, closeAt})
	if err != nil {
		t.Fatal(err)
	}
	// Instants :10 and :15 read book A; :20 and :25 read book B.
	wantAsk := []string{"0.005", "0.005", "0.015", "0.015"}
	wantBid := []string{"0.05", "0.05", "0.02", "0.02"}
	if len(cs.AskSlips) != 4 || len(cs.BidSlips) != 4 {
		t.Fatalf("distributions %d/%d, want 4/4", len(cs.BidSlips), len(cs.AskSlips))
	}
	for i := range wantAsk {
		if cs.AskSlips[i].Decimal() != wantAsk[i] || cs.BidSlips[i].Decimal() != wantBid[i] {
			t.Fatalf("slips[%d] bid %s ask %s, want %s / %s",
				i, cs.BidSlips[i].Decimal(), cs.AskSlips[i].Decimal(), wantBid[i], wantAsk[i])
		}
	}
	o := cs.AtWanted[open.UnixMilli()]
	c := cs.AtWanted[closeAt.UnixMilli()]
	if !o.OK || !c.OK {
		t.Fatalf("boundaries not OK: open %+v close %+v", o, c)
	}
	if o.Ask.Decimal() != "0.005" || c.Ask.Decimal() != "0.015" || o.Bid.Decimal() != "0.05" || c.Bid.Decimal() != "0.02" {
		t.Fatalf("boundary slips open %s/%s close %s/%s", o.Bid.Decimal(), o.Ask.Decimal(), c.Bid.Decimal(), c.Ask.Decimal())
	}
}

// A book too thin for the candidate is DepthShort: excluded from the
// distribution, counted, and a wanted instant there carries its reason.
func TestWalkBinanceBookCostsDepthShort(t *testing.T) {
	root := t.TempDir()
	thinB, thinA := [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}
	f1 := sampleBase.Add(8 * time.Second)
	f2 := sampleBase.Add(18 * time.Second)
	writeFixture(t, filepath_Join(root, bnBookRel("2026-07-30T10.arbcap")), false,
		dataAt(f1, bnDepthFrame(f1, 100, thinB, thinA)),
		dataAt(f2, bnDepthFrame(f2, 110, thinB, thinA)),
	)
	writeManifest(t, root, sampleDay, bnBookRel("2026-07-30T10.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	open := sampleBase.Add(10 * time.Second)
	cs, err := WalkBinanceBookCosts(root, days, bnBookKeyFix, bnInstFix, bookP, abcQty(t, "2"), []time.Time{open})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.AskSlips) != 0 || len(cs.BidSlips) != 0 {
		t.Fatalf("thin book produced samples: %d/%d", len(cs.BidSlips), len(cs.AskSlips))
	}
	if cs.DepthShort != 4 { // two instants × two sides
		t.Fatalf("depth short %d, want 4", cs.DepthShort)
	}
	b := cs.AtWanted[open.UnixMilli()]
	if b.OK || b.Reason == "" {
		t.Fatalf("boundary %+v, want a named refusal", b)
	}
}

// The Bybit walk: a mid-file u jump breaks the book; the instants between
// the break and the next snapshot are skipped WITH the wanted-instant
// reason, and the re-anchored book samples again.
func TestWalkBybitBookCostsBreakAndReanchor(t *testing.T) {
	root := t.TempDir()
	xB, xA := bookAFixture()
	yB, yA := bookBFixture()
	snap1 := sampleBase.Add(7 * time.Second)
	bad := sampleBase.Add(13 * time.Second)
	cold := sampleBase.Add(17 * time.Second)
	snap2 := sampleBase.Add(23 * time.Second)
	tail := sampleBase.Add(27 * time.Second)
	writeFixture(t, filepath_Join(root, byBookRel("2026-07-30T10.arbcap")), false,
		dataAt(snap1, byBookSnapFrame(snap1, 100, xB, xA)),
		dataAt(bad, byBookDeltaFrame(bad, 105, nil, [][2]string{{"101.00", "9"}})), // jump: break
		dataAt(cold, byBookDeltaFrame(cold, 106, nil, nil)),                        // cold, uncounted in the book
		dataAt(snap2, byBookSnapFrame(snap2, 200, yB, yA)),
		dataAt(tail, byBookDeltaFrame(tail, 201, nil, nil)),
	)
	writeManifest(t, root, sampleDay, byBookRel("2026-07-30T10.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	wanted := []time.Time{sampleBase.Add(15 * time.Second)}
	cs, err := WalkBybitBookCosts(root, days, byBookKeyFix, byInstFix, bookP, abcQty(t, "2"), wanted)
	if err != nil {
		t.Fatal(err)
	}
	// :10 reads the anchored book X; :15 and :20 fall in the broken
	// stretch; :25 reads the re-anchored book Y.
	if len(cs.AskSlips) != 2 {
		t.Fatalf("ask samples %d, want 2 (books X and Y)", len(cs.AskSlips))
	}
	if cs.AskSlips[0].Decimal() != "0.005" || cs.AskSlips[1].Decimal() != "0.015" {
		t.Fatalf("ask slips %s, %s — want X then Y", cs.AskSlips[0].Decimal(), cs.AskSlips[1].Decimal())
	}
	if cs.Skipped != 2 {
		t.Fatalf("skipped %d, want 2", cs.Skipped)
	}
	if cs.Breaks != 1 || cs.ColdDeltas != 1 {
		t.Fatalf("breaks %d cold %d, want 1 and 1", cs.Breaks, cs.ColdDeltas)
	}
	b := cs.AtWanted[wanted[0].UnixMilli()]
	if b.OK || b.Reason == "" {
		t.Fatalf("broken-stretch boundary %+v, want a named refusal", b)
	}
}
