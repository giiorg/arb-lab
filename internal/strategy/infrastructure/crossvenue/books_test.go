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

// The cold arm's own fixture (the carry tests' mirror — single-file
// batteries leave the boundary reset inert, the §45m collapse shape one
// layer down): a data-first file boundary is NOT a proven rotation, so
// the held Binance book resets and the post-boundary instants skip.
func TestWalkBinanceBookCostsFileBoundaryResets(t *testing.T) {
	root := t.TempDir()
	xB, xA := bookAFixture()
	yB, yA := bookBFixture()
	a := sampleBase.Add(59*time.Minute + 58*time.Second)
	b := sampleBase.Add(1*time.Hour + 7*time.Second)
	writeFixture(t, filepath_Join(root, bnBookRel("2026-07-30T10.arbcap")), false,
		dataAt(a, bnDepthFrame(a, 10, xB, xA)))
	writeFixture(t, filepath_Join(root, bnBookRel("2026-07-30T11.arbcap")), false,
		dataAt(b, bnDepthFrame(b, 11, yB, yA)))
	writeManifest(t, root, sampleDay,
		bnBookRel("2026-07-30T10.arbcap"), bnBookRel("2026-07-30T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := WalkBinanceBookCosts(root, days, bnBookKeyFix, bnInstFix, bookP, abcQty(t, "2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 11:00:00 and 11:00:05 sit inside slack of file 1's frame — only the
	// cold boundary explains skipping them.
	if len(cs.AskSlips) != 0 || cs.Skipped != 2 {
		t.Fatalf("ask samples %d skipped %d, want 0 and 2", len(cs.AskSlips), cs.Skipped)
	}
}

// The Bybit mirror: across an unproven boundary the accumulator drops, so
// the continuing file's deltas are COLD (counted, never applied) and
// nothing samples until a snapshot that never comes.
func TestWalkBybitBookCostsFileBoundaryResets(t *testing.T) {
	root := t.TempDir()
	xB, xA := bookAFixture()
	snap := sampleBase.Add(59*time.Minute + 50*time.Second)
	d1 := sampleBase.Add(1*time.Hour + 3*time.Second)
	d2 := sampleBase.Add(1*time.Hour + 9*time.Second)
	writeFixture(t, filepath_Join(root, byBookRel("2026-07-30T10.arbcap")), false,
		dataAt(snap, byBookSnapFrame(snap, 100, xB, xA)))
	writeFixture(t, filepath_Join(root, byBookRel("2026-07-30T11.arbcap")), false,
		dataAt(d1, byBookDeltaFrame(d1, 101, nil, [][2]string{{"101.00", "9"}})),
		dataAt(d2, byBookDeltaFrame(d2, 102, nil, nil)),
	)
	writeManifest(t, root, sampleDay,
		byBookRel("2026-07-30T10.arbcap"), byBookRel("2026-07-30T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := WalkBybitBookCosts(root, days, byBookKeyFix, byInstFix, bookP, abcQty(t, "2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 10:59:55, 11:00:00 and 11:00:05 all skip: the book dropped at the
	// boundary and both deltas run cold against no anchor.
	if len(cs.AskSlips) != 0 || cs.Skipped != 3 {
		t.Fatalf("ask samples %d skipped %d, want 0 and 3", len(cs.AskSlips), cs.Skipped)
	}
	if cs.ColdDeltas != 2 || cs.Breaks != 0 {
		t.Fatalf("cold %d breaks %d, want 2 and 0", cs.ColdDeltas, cs.Breaks)
	}
}

// The §45n-1 carry, Binance book arm: the held snapshot survives a living
// rotation, so the instants right after the boundary walk it instead of
// skipping — and a wanted instant there measures instead of refusing.
func TestWalkBinanceBookCostsRotationCarries(t *testing.T) {
	root := t.TempDir()
	xB, xA := bookAFixture()
	yB, yA := bookBFixture()
	a := sampleBase.Add(59*time.Minute + 58*time.Second)
	b := sampleBase.Add(1*time.Hour + 7*time.Second)
	writeFixtureFrom(t, filepath_Join(root, bnBookRel("2026-07-30T10.arbcap")), false, 5,
		openAt(sampleBase, 0),
		dataAt(a, bnDepthFrame(a, 10, xB, xA)))
	writeFixtureFrom(t, filepath_Join(root, bnBookRel("2026-07-30T11.arbcap")), false, 7,
		openAt(b.Add(-7*time.Second), 6),
		dataAt(b, bnDepthFrame(b, 11, yB, yA)))
	writeManifest(t, root, sampleDay,
		bnBookRel("2026-07-30T10.arbcap"), bnBookRel("2026-07-30T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	wanted := []time.Time{sampleBase.Add(1*time.Hour + 5*time.Second)}
	cs, err := WalkBinanceBookCosts(root, days, bnBookKeyFix, bnInstFix, bookP, abcQty(t, "2"), wanted)
	if err != nil {
		t.Fatal(err)
	}
	// 11:00:00 and 11:00:05 walk book X across the carried boundary.
	if len(cs.AskSlips) != 2 || cs.Skipped != 0 {
		t.Fatalf("ask samples %d skipped %d, want 2 and 0", len(cs.AskSlips), cs.Skipped)
	}
	if cs.AskSlips[0].Decimal() != "0.005" || cs.AskSlips[1].Decimal() != "0.005" {
		t.Fatalf("ask slips %s, %s — want book X both instants", cs.AskSlips[0].Decimal(), cs.AskSlips[1].Decimal())
	}
	bs := cs.AtWanted[wanted[0].UnixMilli()]
	if !bs.OK {
		t.Fatalf("boundary %+v, want a measured sample across the carry", bs)
	}
}

// The §45n-1 carry, Bybit book arm: a delta-only continuing file keeps the
// book — the delta FOLLOWS the carried u, so nothing is cold, nothing
// breaks, and the post-boundary instants sample the reconstruction. Under
// the per-file cold rule this exact stream measured nothing after the
// boundary until a snapshot that never comes (§45m's ~9% mechanism).
func TestWalkBybitBookCostsRotationCarries(t *testing.T) {
	root := t.TempDir()
	xB, xA := bookAFixture()
	yB, _ := bookBFixture()
	snap := sampleBase.Add(59*time.Minute + 50*time.Second)
	d1 := sampleBase.Add(1*time.Hour + 3*time.Second)
	d2 := sampleBase.Add(1*time.Hour + 9*time.Second)
	writeFixtureFrom(t, filepath_Join(root, byBookRel("2026-07-30T10.arbcap")), false, 5,
		openAt(sampleBase, 0),
		dataAt(snap, byBookSnapFrame(snap, 100, xB, xA)))
	// The delta rewrites the ask side to book Y's shape: it must DELETE
	// book X's 101.00 level (size 0) — delta semantics update levels in
	// place, they do not replace the book.
	yAskDelta := [][2]string{{"100.00", "1"}, {"101.00", "0"}, {"103.00", "1"}}
	writeFixtureFrom(t, filepath_Join(root, byBookRel("2026-07-30T11.arbcap")), false, 7,
		openAt(d1.Add(-3*time.Second), 6),
		dataAt(d1, byBookDeltaFrame(d1, 101, yB, yAskDelta)),
		dataAt(d2, byBookDeltaFrame(d2, 102, nil, nil)),
	)
	writeManifest(t, root, sampleDay,
		byBookRel("2026-07-30T10.arbcap"), byBookRel("2026-07-30T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := WalkBybitBookCosts(root, days, byBookKeyFix, byInstFix, bookP, abcQty(t, "2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 10:59:55 and 11:00:00 walk book X (held across the boundary);
	// 11:00:05 walks the book after the FOLLOWING delta rewrote its
	// levels to Y — applied, not cold, only because u carried.
	if cs.ColdDeltas != 0 || cs.Breaks != 0 || cs.Skipped != 0 {
		t.Fatalf("cold %d breaks %d skipped %d, want all 0", cs.ColdDeltas, cs.Breaks, cs.Skipped)
	}
	if len(cs.AskSlips) != 3 {
		t.Fatalf("ask samples %d, want 3", len(cs.AskSlips))
	}
	if cs.AskSlips[1].Decimal() != "0.005" || cs.AskSlips[2].Decimal() != "0.015" {
		t.Fatalf("ask slips %s then %s — want X then the delta-updated Y",
			cs.AskSlips[1].Decimal(), cs.AskSlips[2].Decimal())
	}
}
