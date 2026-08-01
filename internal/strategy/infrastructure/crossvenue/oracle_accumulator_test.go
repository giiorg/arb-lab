package crossvenue

// The accumulator's model-based oracle (§45g): a second implementation in
// a different shape — sorted slices with linear insertion, no maps — fed
// the same generated stream, compared book-for-book after every message.
// The generator's stream is a pinned SplitMix64 (a rerun IS a replay) and
// deliberately mixes follows, duplicate-u, jumped-u, snapshots, deletes of
// present and absent levels, and drops; the closing assertions require
// every one of those arms to have actually occurred, because a generator
// drifting into one regime is the vacuous-run shape wearing randomness.

import (
	"fmt"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
)

type oracleSplitmix struct{ state uint64 }

func (o *oracleSplitmix) draw() uint64 {
	o.state += 0x9E3779B97F4A7C15
	x := o.state
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// refBook is the reference: plain sorted slices, best-first.
type refBook struct {
	bids, asks []venue.BookLevel // bids descending, asks ascending
	anchored   bool
	lastU      uint64
}

func (r *refBook) apply(t *testing.T, msg venue.BybitBookMessage) {
	t.Helper()
	switch msg.Kind {
	case venue.BybitBookSnapshotKind:
		r.bids = append([]venue.BookLevel(nil), msg.Snapshot.Bids...)
		r.asks = append([]venue.BookLevel(nil), msg.Snapshot.Asks...)
		r.anchored = true
		r.lastU = msg.UpdateID
	case venue.BybitBookDeltaKind:
		if !r.anchored {
			return
		}
		if msg.UpdateID != r.lastU+1 {
			r.anchored = false
			r.bids, r.asks = nil, nil
			return
		}
		for _, c := range msg.Delta.Changes {
			levels := &r.bids
			descending := true
			if c.Side == venue.SideAsk {
				levels = &r.asks
				descending = false
			}
			r.applyChange(t, levels, descending, c)
		}
		r.lastU = msg.UpdateID
	}
}

func (r *refBook) applyChange(t *testing.T, levels *[]venue.BookLevel, descending bool, c venue.LevelChange) {
	t.Helper()
	sign, err := c.Quantity.Sign()
	if err != nil {
		t.Fatalf("oracle qty: %v", err)
	}
	// Linear scan for the price's position.
	at := -1
	insertAt := len(*levels)
	for i, l := range *levels {
		cmp, err := l.Price.Cmp(c.Price)
		if err != nil {
			t.Fatalf("oracle cmp: %v", err)
		}
		if cmp == 0 {
			at = i
			break
		}
		better := cmp > 0 // ascending: existing smaller is better placed first
		if descending {
			better = cmp < 0
		}
		if better && insertAt == len(*levels) {
			insertAt = i
		}
	}
	switch {
	case sign == 0 && at >= 0:
		*levels = append((*levels)[:at], (*levels)[at+1:]...)
	case sign == 0:
		// absent delete: no-op
	case at >= 0:
		(*levels)[at] = venue.BookLevel{Price: c.Price, Quantity: c.Quantity}
	default:
		*levels = append(*levels, venue.BookLevel{})
		copy((*levels)[insertAt+1:], (*levels)[insertAt:])
		(*levels)[insertAt] = venue.BookLevel{Price: c.Price, Quantity: c.Quantity}
	}
}

func (r *refBook) sample() (venue.BookSnapshot, bool) {
	if !r.anchored {
		return venue.BookSnapshot{}, false
	}
	out := venue.BookSnapshot{
		Bids: append([]venue.BookLevel(nil), r.bids...),
		Asks: append([]venue.BookLevel(nil), r.asks...),
	}
	if len(out.Bids) > 50 {
		out.Bids = out.Bids[:50]
	}
	if len(out.Asks) > 50 {
		out.Asks = out.Asks[:50]
	}
	return out, true
}

func TestAccumulatorMatchesSliceOracle(t *testing.T) {
	g := &oracleSplitmix{state: 0xB00CACC}
	acc := NewBookAccumulator()
	ref := &refBook{}

	price := func() string { return fmt.Sprintf("%d.%02d", 50+g.draw()%60, g.draw()%100) }
	qty := func() string { return fmt.Sprintf("%d.%d", 1+g.draw()%9, g.draw()%10) }

	var u uint64 = 100
	arms := map[string]int{}
	for step := 0; step < 3000; step++ {
		var msg venue.BybitBookMessage
		switch g.draw() % 10 {
		case 0: // snapshot
			u += 1 + g.draw()%3
			nb, na := int(g.draw()%4), int(g.draw()%4)
			var bids, asks [][2]string
			// Snapshot sides must be strictly ordered: generate a sorted
			// ladder around a random mid.
			mid := 50 + int(g.draw()%60)
			for i := 0; i < nb; i++ {
				bids = append(bids, [2]string{fmt.Sprintf("%d.00", mid-1-i), qty()})
			}
			for i := 0; i < na; i++ {
				asks = append(asks, [2]string{fmt.Sprintf("%d.00", mid+1+i), qty()})
			}
			msg = accSnapshot(t, u, bids, asks)
			arms["snapshot"]++
		case 1: // duplicate u
			msg = accDelta(t, u, [3]string{"bid", price(), qty()})
			arms["dup"]++
		case 2: // jumped u
			msg = accDelta(t, u+2, [3]string{"ask", price(), qty()})
			arms["jump"]++
			u += 2 // the venue really did move on; the next follow is relative to reality
		case 3: // drop (gap/file seam)
			acc.Drop()
			ref.anchored = false
			ref.bids, ref.asks = nil, nil
			arms["drop"]++
			continue
		default: // following delta with a mixed change list
			u++
			n := 1 + int(g.draw()%3)
			changes := make([][3]string, n)
			for i := range changes {
				side := "bid"
				if g.draw()%2 == 0 {
					side = "ask"
				}
				q := qty()
				if g.draw()%4 == 0 {
					q = "0" // delete — present or absent, the stream decides
					arms["delete"]++
				}
				changes[i] = [3]string{side, price(), q}
			}
			msg = accDelta(t, u, changes...)
			arms["follow"]++
		}

		acc.Apply(msg)
		ref.apply(t, msg)

		gotBook, gotOK := acc.Sample()
		wantBook, wantOK := ref.sample()
		if gotOK != wantOK {
			t.Fatalf("step %d: anchored %v, oracle %v", step, gotOK, wantOK)
		}
		if !gotOK {
			continue
		}
		compareLevels(t, step, "bids", gotBook.Bids, wantBook.Bids)
		compareLevels(t, step, "asks", gotBook.Asks, wantBook.Asks)
	}
	for _, arm := range []string{"snapshot", "dup", "jump", "drop", "follow", "delete"} {
		if arms[arm] == 0 {
			t.Fatalf("generator never exercised %q — the oracle is running on a thinner stream than it claims", arm)
		}
	}
	if acc.Stats().DeletesAbsent == 0 {
		t.Fatal("no absent delete ever occurred — the tolerated arm went untested")
	}
	if acc.Stats().Breaks == 0 {
		t.Fatal("no break ever occurred")
	}
}

func compareLevels(t *testing.T, step int, side string, got, want []venue.BookLevel) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("step %d %s: %d levels, oracle %d", step, side, len(got), len(want))
	}
	for i := range got {
		pc, err := got[i].Price.Cmp(want[i].Price)
		if err != nil || pc != 0 {
			t.Fatalf("step %d %s level %d: price %s, oracle %s (err %v)",
				step, side, i, got[i].Price.Decimal(), want[i].Price.Decimal(), err)
		}
		qc, err := got[i].Quantity.Cmp(want[i].Quantity)
		if err != nil || qc != 0 {
			t.Fatalf("step %d %s level %d: qty %s, oracle %s (err %v)",
				step, side, i, got[i].Quantity, want[i].Quantity, err)
		}
	}
}
