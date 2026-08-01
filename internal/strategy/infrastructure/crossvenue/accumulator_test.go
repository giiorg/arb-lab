package crossvenue

// Accumulator battery. Fixture discipline (§45g's suspects): the two
// non-follow shapes are pinned SEPARATELY — a duplicate u (lastU, which a
// wrong >= absorbs) and a jump (lastU+2, which a wrong > absorbs) — because
// a fixture whose sequence numbers are merely "not consecutive" cannot tell
// the two wrong comparisons apart; the truncation fixture holds MORE than
// the 50-level window, because at 50 truncation and no-op are the same
// slice; and delta insertions land BETWEEN snapshot levels, so the sorted
// render is doing work the venue's pre-sorted snapshot would mask.

import (
	"fmt"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

func accLevel(t *testing.T, price, qty string) venue.BookLevel {
	t.Helper()
	p, err := fixedpoint.ParsePrice("ABC", "USDT", price)
	if err != nil {
		t.Fatalf("price %q: %v", price, err)
	}
	q, err := fixedpoint.ParseQuantity("ABC", qty)
	if err != nil {
		t.Fatalf("qty %q: %v", qty, err)
	}
	return venue.BookLevel{Price: p, Quantity: q}
}

func accSnapshot(t *testing.T, u uint64, bids, asks [][2]string) venue.BybitBookMessage {
	t.Helper()
	msg := venue.BybitBookMessage{Kind: venue.BybitBookSnapshotKind, UpdateID: u}
	for _, b := range bids {
		msg.Snapshot.Bids = append(msg.Snapshot.Bids, accLevel(t, b[0], b[1]))
	}
	for _, a := range asks {
		msg.Snapshot.Asks = append(msg.Snapshot.Asks, accLevel(t, a[0], a[1]))
	}
	return msg
}

func accDelta(t *testing.T, u uint64, changes ...[3]string) venue.BybitBookMessage {
	t.Helper()
	msg := venue.BybitBookMessage{Kind: venue.BybitBookDeltaKind, UpdateID: u, Delta: venue.BookDelta{FirstSeq: u}}
	for _, c := range changes {
		side := venue.SideBid
		if c[0] == "ask" {
			side = venue.SideAsk
		}
		p, err := fixedpoint.ParsePrice("ABC", "USDT", c[1])
		if err != nil {
			t.Fatalf("price %q: %v", c[1], err)
		}
		q, err := fixedpoint.ParseQuantity("ABC", c[2])
		if err != nil {
			t.Fatalf("qty %q: %v", c[2], err)
		}
		msg.Delta.Changes = append(msg.Delta.Changes, venue.LevelChange{Side: side, Price: p, Quantity: q})
	}
	return msg
}

// wantBook fails unless the sampled book renders exactly the given
// (price, qty) sequences.
func wantBook(t *testing.T, a *BookAccumulator, bids, asks [][2]string) {
	t.Helper()
	got, ok := a.Sample()
	if !ok {
		t.Fatal("Sample: not anchored")
	}
	check := func(side string, got []venue.BookLevel, want [][2]string) {
		if len(got) != len(want) {
			t.Fatalf("%s levels %d, want %d", side, len(got), len(want))
		}
		for i, w := range want {
			wl := accLevel(t, w[0], w[1])
			if pc, err := got[i].Price.Cmp(wl.Price); err != nil || pc != 0 {
				t.Fatalf("%s level %d price %s, want %s (err %v)", side, i, got[i].Price.Decimal(), w[0], err)
			}
			if qc, err := got[i].Quantity.Cmp(wl.Quantity); err != nil || qc != 0 {
				t.Fatalf("%s level %d qty %s, want %s (err %v)", side, i, got[i].Quantity, w[1], err)
			}
		}
	}
	check("bid", got.Bids, bids)
	check("ask", got.Asks, asks)
}

// A snapshot anchors; a following delta updates, inserts BETWEEN existing
// levels, and deletes — and the render stays bids-descending,
// asks-ascending.
func TestAccumulatorAppliesFollowingDelta(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accSnapshot(t, 10,
		[][2]string{{"99.00", "1"}, {"97.00", "2"}},
		[][2]string{{"100.00", "1"}, {"102.00", "2"}}))
	a.Apply(accDelta(t, 11,
		[3]string{"bid", "98.00", "5"},  // insert between 99 and 97
		[3]string{"ask", "100.00", "3"}, // update in place
		[3]string{"ask", "102.00", "0"}, // delete
		[3]string{"ask", "101.00", "4"}, // insert between (what was) 100 and 102
	))
	wantBook(t,
		a,
		[][2]string{{"99.00", "1"}, {"98.00", "5"}, {"97.00", "2"}},
		[][2]string{{"100.00", "3"}, {"101.00", "4"}})
	st := a.Stats()
	if st.Snapshots != 1 || st.DeltasApplied != 1 || st.Breaks != 0 {
		t.Fatalf("stats %+v", st)
	}
	if a.UpdateID() != 11 {
		t.Fatalf("update id %d, want 11", a.UpdateID())
	}
}

// A duplicate u — u equal to the held position — is a break, not a
// reapply. A follows-check written as u > lastU would absorb the jump
// case below but reject this one; one written as u >= lastU+1 would
// absorb the jump; each fixture kills one wrong comparison.
func TestAccumulatorDuplicateUBreaks(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}))
	a.Apply(accDelta(t, 10, [3]string{"bid", "98.00", "1"}))
	if a.Anchored() {
		t.Fatal("anchored after duplicate u")
	}
	if _, ok := a.Sample(); ok {
		t.Fatal("Sample returned a book across a break")
	}
	if st := a.Stats(); st.Breaks != 1 {
		t.Fatalf("breaks %d, want 1", st.Breaks)
	}
}

// A jump — u two past the held position — is equally a break: the venue
// advances u by exactly 1 per delta (Profiles §5, measured), so a gapped
// u means a dropped frame, not a fast feed.
func TestAccumulatorJumpBreaks(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}))
	a.Apply(accDelta(t, 12, [3]string{"bid", "98.00", "1"}))
	if a.Anchored() {
		t.Fatal("anchored after jumped u")
	}
	if st := a.Stats(); st.Breaks != 1 {
		t.Fatalf("breaks %d, want 1", st.Breaks)
	}
}

// Deltas before any anchor are counted cold, never applied; the next
// snapshot re-anchors and the book is exactly that snapshot.
func TestAccumulatorColdAndReanchor(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accDelta(t, 5, [3]string{"bid", "98.00", "1"}))
	if _, ok := a.Sample(); ok {
		t.Fatal("cold Sample returned a book")
	}
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}))
	a.Apply(accDelta(t, 12)) // break
	a.Apply(accDelta(t, 13, [3]string{"bid", "1.00", "1"}))
	a.Apply(accSnapshot(t, 30, [][2]string{{"95.00", "7"}}, [][2]string{{"96.00", "8"}}))
	wantBook(t, a, [][2]string{{"95.00", "7"}}, [][2]string{{"96.00", "8"}})
	st := a.Stats()
	if st.ColdDeltas != 2 || st.Breaks != 1 || st.Snapshots != 2 {
		t.Fatalf("stats %+v", st)
	}
	if a.UpdateID() != 30 {
		t.Fatalf("update id %d, want 30", a.UpdateID())
	}
}

// A delete for a price this book never held is window slide: a counted
// no-op that neither breaks nor disturbs the changes beside it.
func TestAccumulatorDeleteAbsentTolerated(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}))
	a.Apply(accDelta(t, 11,
		[3]string{"bid", "42.00", "0"}, // never held
		[3]string{"bid", "98.50", "2"},
	))
	wantBook(t, a, [][2]string{{"99.00", "1"}, {"98.50", "2"}}, [][2]string{{"100.00", "1"}})
	st := a.Stats()
	if st.DeletesAbsent != 1 || st.Breaks != 0 || st.DeltasApplied != 1 {
		t.Fatalf("stats %+v", st)
	}
}

// Drop unanchors without counting a break (the walker's gap/file seam),
// and the next snapshot re-anchors.
func TestAccumulatorDropIsNotABreak(t *testing.T) {
	a := NewBookAccumulator()
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, [][2]string{{"100.00", "1"}}))
	a.Drop()
	if a.Anchored() {
		t.Fatal("anchored after Drop")
	}
	if st := a.Stats(); st.Breaks != 0 {
		t.Fatalf("breaks %d, want 0", st.Breaks)
	}
	a.Apply(accSnapshot(t, 50, [][2]string{{"90.00", "1"}}, [][2]string{{"91.00", "1"}}))
	wantBook(t, a, [][2]string{{"90.00", "1"}}, [][2]string{{"91.00", "1"}})
}

// The render truncates to the venue's 50-level window. The fixture holds
// 51 asks — at exactly 50, truncation and identity are the same slice —
// and the level dropped is the WORST (highest ask), never the best.
func TestAccumulatorSampleTruncatesTo50(t *testing.T) {
	a := NewBookAccumulator()
	asks := make([][2]string, 50)
	for i := range asks {
		asks[i] = [2]string{fixedPrice(100 + i), "1"}
	}
	a.Apply(accSnapshot(t, 10, [][2]string{{"99.00", "1"}}, asks))
	// A new BEST ask slides in; the window now holds 51 and must shed
	// the worst (149.00), keeping the new best.
	a.Apply(accDelta(t, 11, [3]string{"ask", "99.50", "9"}))
	got, ok := a.Sample()
	if !ok {
		t.Fatal("not anchored")
	}
	if len(got.Asks) != 50 {
		t.Fatalf("asks %d, want 50", len(got.Asks))
	}
	if got.Asks[0].Price.Decimal() != "99.5" {
		t.Fatalf("best ask %s, want 99.5", got.Asks[0].Price.Decimal())
	}
	if last := got.Asks[49].Price.Decimal(); last != "148" {
		t.Fatalf("worst kept ask %s, want 148 (149 truncated)", last)
	}
}

func fixedPrice(n int) string {
	return fmt.Sprintf("%d.00", n)
}
