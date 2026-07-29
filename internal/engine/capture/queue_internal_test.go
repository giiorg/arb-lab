package capture

import (
	"context"
	"slices"
	"testing"
	"time"
)

// §32l's third row — "a drop of exactly 1" collapses "hole" and "off-by-one" —
// plus the two extensions it names: a drop of two or more, and a drop at the
// FIRST record, where "there is a hole" and "the file simply starts mid-sequence"
// are the same reading.
//
// Every test here is bounded. §32l says so in its own words ("capture is
// channel-heavy and a mutation that breaks a feed makes a test BLOCK rather than
// fail"), and clock and boot both paid for it: a TIMEOUT!! is a claim that was
// never checked while looking covered. The bound is a context rather than
// time.After because notimenow polices package time across engine/.

// drain runs a consumer to completion under a deadline, returning what it saw.
//
// The deadline converts the failure mode that costs the most — a mutation that
// stops the queue feeding, so the test hangs — into an ordinary failure with a
// message.
func drain(t *testing.T, q *Queue) (seqs []uint64, gaps []Gap) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type out struct {
		seqs []uint64
		gaps []Gap
	}
	done := make(chan out, 1)
	go func() {
		var got out
		for {
			rec, gap, ok := q.Receive()
			if !ok {
				done <- got
				return
			}
			if gap != nil {
				got.gaps = append(got.gaps, *gap)
			}
			got.seqs = append(got.seqs, rec.CaptureSeq)
		}
	}()

	select {
	case got := <-done:
		return got.seqs, got.gaps
	case <-ctx.Done():
		t.Fatal("the consumer did not finish within the deadline — the queue stopped " +
			"feeding, which without this bound would be a hang rather than a failure")
		return nil, nil
	}
}

// push fills a queue with a run of sequences, reporting which were accepted.
func push(t *testing.T, q *Queue, c *fakeClock, seqs ...uint64) []uint64 {
	t.Helper()
	var accepted []uint64
	for _, s := range seqs {
		c.Advance(time.Millisecond)
		if q.Push(dataRecord(s, c, "payload")) {
			accepted = append(accepted, s)
		}
	}
	return accepted
}

func TestQueueDropNewestRecordsTheHole(t *testing.T) {
	for _, tc := range []struct {
		name string
		// depth, then the sequences pushed before anything is consumed. The
		// queue accepts `depth` of them and drops the rest, so the hole's size
		// is the row's variable.
		depth   int
		pushed  int
		wantGap Gap
	}{
		{
			// A hole of one, where "count == to-from+1" and "count == 1" agree.
			name: "a drop of exactly one", depth: 2, pushed: 3,
			wantGap: Gap{Cause: CauseOverflow, From: 3, To: 3, Count: 1},
		},
		{
			// A hole of more than one, which is what separates them.
			name: "a drop of several", depth: 2, pushed: 6,
			wantGap: Gap{Cause: CauseOverflow, From: 3, To: 6, Count: 4},
		},
		{
			// The hole at the very first sequence: with depth 0 impossible, the
			// nearest case is a queue that is full from the start, so the run
			// begins with a drop. See TestQueueDropAtTheFirstRecord.
			name: "a longer drop still", depth: 1, pushed: 5,
			wantGap: Gap{Cause: CauseOverflow, From: 2, To: 5, Count: 4},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClock(startInstant(t))
			q, err := NewQueue(tc.depth)
			if err != nil {
				t.Fatalf("NewQueue: %v", err)
			}
			seqs := make([]uint64, tc.pushed)
			for i := range seqs {
				seqs[i] = uint64(i + 1)
			}
			accepted := push(t, q, c, seqs...)
			if len(accepted) != tc.depth {
				t.Fatalf("a depth-%d queue accepted %v, want %d records", tc.depth, accepted, tc.depth)
			}

			// The hole is still open — it attaches to the NEXT accepted record,
			// which is what keeps the gap and the record it precedes in order
			// under every interleaving.
			if _, open := q.PendingGap(); !open {
				t.Fatal("the queue reports no pending hole after dropping")
			}

			// Consume one, freeing a slot, then push the record the hole
			// attaches to.
			if _, _, ok := q.Receive(); !ok {
				t.Fatal("Receive on a non-empty queue reported closed")
			}
			c.Advance(time.Millisecond)
			next := uint64(tc.pushed + 1)
			if !q.Push(dataRecord(next, c, "after the hole")) {
				t.Fatal("the push after freeing a slot was dropped")
			}
			if _, open := q.PendingGap(); open {
				t.Error("the hole is still pending after it was attached to a record")
			}
			q.Close()

			_, gaps := drain(t, q)
			if len(gaps) != 1 {
				t.Fatalf("consumer saw %d gaps, want exactly 1: %+v", len(gaps), gaps)
			}
			if gaps[0] != tc.wantGap {
				t.Errorf("gap = %+v, want %+v", gaps[0], tc.wantGap)
			}
			if err := gaps[0].Validate(); err != nil {
				t.Errorf("the queue produced a gap its own validator refuses: %v", err)
			}
		})
	}
}

// TestQueueDropAtTheFirstRecord is §32l's own extension of the row above: a hole
// at the FIRST record of a stream, where "there is a hole" and "the sequence
// simply starts at 4" are the same reading from the file alone.
//
// It is the gap record that distinguishes them, which is the case that makes the
// gap record worth writing at all — everywhere else the hole in CaptureSeq is
// already evidence (§32d), and here there is nothing to be a hole in yet.
func TestQueueDropAtTheFirstRecord(t *testing.T) {
	c := newFakeClock(startInstant(t))
	q, err := NewQueue(1)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	// Fill the single slot with a record, then drop 1..3 on top of it — no:
	// fill it and immediately drop, so the FIRST sequences of the run are the
	// dropped ones once the slot is occupied.
	if !q.Push(dataRecord(1, c, "occupies the slot")) {
		t.Fatal("the first push was dropped")
	}
	dropped := push(t, q, c, 2, 3)
	if len(dropped) != 0 {
		t.Fatalf("a full depth-1 queue accepted %v", dropped)
	}
	if _, _, ok := q.Receive(); !ok {
		t.Fatal("Receive reported closed")
	}
	c.Advance(time.Millisecond)
	if !q.Push(dataRecord(4, c, "first record of the next file")) {
		t.Fatal("the push after freeing the slot was dropped")
	}
	q.Close()

	seqs, gaps := drain(t, q)
	if !slices.Equal(seqs, []uint64{4}) {
		t.Errorf("consumer saw %v, want just 4", seqs)
	}
	if len(gaps) != 1 || gaps[0].From != 2 || gaps[0].To != 3 || gaps[0].Count != 2 {
		t.Fatalf("gaps = %+v, want one covering [2, 3]", gaps)
	}
}

// TestQueueKeepsTheConsumerStreamAscending is drop-newest's defining property,
// and the reason §32f chose it over drop-oldest: the writer never has to reorder
// or rewrite, and the hole is exactly the run between two accepted sequences.
func TestQueueKeepsTheConsumerStreamAscending(t *testing.T) {
	c := newFakeClock(startInstant(t))
	q, err := NewQueue(3)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	var accepted []uint64
	// Interleave pushes and receives so the queue fills, overflows, drains and
	// overflows again — a single fill-then-drain cannot tell a hole from a
	// prefix.
	for i := uint64(1); i <= 20; i++ {
		c.Advance(time.Millisecond)
		if q.Push(dataRecord(i, c, "x")) {
			accepted = append(accepted, i)
		}
		if i%4 == 0 {
			if _, _, ok := q.Receive(); !ok {
				t.Fatal("Receive reported closed mid-run")
			}
		}
	}
	if !slices.IsSorted(accepted) {
		t.Errorf("accepted sequences are not ascending: %v", accepted)
	}
	for i := 1; i < len(accepted); i++ {
		if accepted[i] == accepted[i-1] {
			t.Errorf("sequence %d was accepted twice", accepted[i])
		}
	}
	// Non-vacuity: a run that never overflowed would satisfy everything above
	// while testing nothing about the drop policy.
	if st := q.Stats(); st.Drops == 0 {
		t.Fatalf("the fixture never overflowed (%+v), so the ascent claim is untested", st)
	}
	q.Close()
	drain(t, q)
}

// TestQueuePendingGapSurvivesClose covers the case a shutdown-under-load
// produces: the hole is still open when the producer stops, so no record is left
// to carry it.
//
// A queue that discarded it would make the tidy case — we stopped while
// overflowing — the one case with no record of the loss.
func TestQueuePendingGapSurvivesClose(t *testing.T) {
	c := newFakeClock(startInstant(t))
	q, err := NewQueue(1)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	push(t, q, c, 1, 2, 3)
	q.Close()

	gap, open := q.PendingGap()
	if !open {
		t.Fatal("the hole open at close was discarded")
	}
	if gap.From != 2 || gap.To != 3 || gap.Count != 2 {
		t.Errorf("pending gap = %+v, want [2, 3] count 2", gap)
	}
	drain(t, q)
}

// TestQueueUnderConcurrentProducerAndConsumer is the -race half. mutate.sh runs
// WITHOUT -race (§32l says so explicitly), so no lock claim in this package is
// reportable by mutation — this test and `go test -race` are the only things
// that check it.
//
// The invariant is stronger than "no race": every sequence is either received
// exactly once or accounted for by exactly one gap, so accepted + dropped
// partitions the run.
func TestQueueUnderConcurrentProducerAndConsumer(t *testing.T) {
	const total = 2000
	c := newFakeClock(startInstant(t))
	q, err := NewQueue(8)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	go func() {
		for i := uint64(1); i <= total; i++ {
			c.Advance(time.Microsecond)
			q.Push(dataRecord(i, c, "x"))
		}
		q.Close()
	}()

	seqs, gaps := drain(t, q)
	seen := make(map[uint64]bool, total)
	for _, s := range seqs {
		if seen[s] {
			t.Fatalf("sequence %d was delivered twice", s)
		}
		seen[s] = true
	}
	for _, g := range gaps {
		if err := g.Validate(); err != nil {
			t.Fatalf("the queue produced an invalid gap %+v: %v", g, err)
		}
		for s := g.From; s <= g.To; s++ {
			if seen[s] {
				t.Fatalf("sequence %d was both delivered and reported dropped", s)
			}
			seen[s] = true
		}
	}
	// The hole still open at close, if any, closes the partition.
	if g, open := q.PendingGap(); open {
		for s := g.From; s <= g.To; s++ {
			seen[s] = true
		}
	}
	for i := uint64(1); i <= total; i++ {
		if !seen[i] {
			t.Fatalf("sequence %d was neither delivered nor recorded as dropped", i)
		}
	}
	if st := q.Stats(); st.Accepted+st.Drops != total {
		t.Errorf("stats account for %d of %d pushes: %+v", st.Accepted+st.Drops, total, st)
	}
}

func TestNewQueueRefusesANonPositiveDepth(t *testing.T) {
	if _, err := NewQueue(1); err != nil {
		t.Fatalf("the positive control was refused: %v", err)
	}
	for _, depth := range []int{0, -1} {
		if _, err := NewQueue(depth); !errorIs(err, ErrQueueDepth) {
			t.Errorf("NewQueue(%d) = %v, want ErrQueueDepth", depth, err)
		}
	}
}
