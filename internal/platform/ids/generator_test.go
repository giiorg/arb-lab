package ids_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Both of a Generator's dependencies are injected because §3.5's parity seam
// requires it: a replay that minted different ids from the same input is not a
// replay. The stubs below are what a backtester would supply, and the tests are
// mostly about the state machine that sits between them — the clamp, the
// counter, and the ordering guarantees they produce.

// stubClock is a clock the test drives, standing in for clock.TestClock. It
// satisfies ids.TimeSource structurally, which is the property that keeps this
// package free of an import edge to platform/clock.
type stubClock struct{ at time.Time }

func (c *stubClock) Now() time.Time { return c.at }
func (c *stubClock) set(ms int64)   { c.at = time.UnixMilli(ms).UTC() }
func newStubClock(ms int64) *stubClock {
	c := &stubClock{}
	c.set(ms)
	return c
}

// constEntropy yields the same byte forever, so a minted id's rand_b is known
// and the test can assert the whole 16 bytes rather than "it looks like a UUID".
type constEntropy struct{ b byte }

func (e constEntropy) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = e.b
	}
	return len(p), nil
}

// failingEntropy behaves like constEntropy except on the nth read (1-based),
// where it fails. Putting the failure at a chosen point in the sequence rather
// than only at the start is what lets a test compare the surviving sequence
// against one that never failed.
type failingEntropy struct {
	b      byte
	failOn int
	n      int
	err    error
}

func (e *failingEntropy) Read(p []byte) (int, error) {
	e.n++
	if e.n == e.failOn {
		return 0, e.err
	}
	for i := range p {
		p[i] = e.b
	}
	return len(p), nil
}

func mustGenerator(t *testing.T, c ids.TimeSource, entropy io.Reader) *ids.Generator {
	t.Helper()
	g, err := ids.NewGenerator(c, entropy)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	return g
}

func TestGeneratorLayout(t *testing.T) {
	// The RFC vector checks the PARSER against an external authority
	// (uuid_test.go). This checks the MINTER against the layout documented on
	// the UUID type, which is a weaker claim and a different one — the two
	// together are what make "we implement RFC 9562" mean something.
	//
	// One deliberate departure from the RFC's example, worth naming: our
	// rand_a holds a monotonic counter rather than random bits. RFC 9562
	// method 1 explicitly permits that, and §3.5's determinism requires it —
	// random rand_a would be entropy the backtester has to reproduce for no
	// benefit, since unguessability is rand_b's 62 bits' job.
	const ms = int64(1645557742000) // 0x017F22E279B0, the RFC's instant
	g := mustGenerator(t, newStubClock(ms), constEntropy{0xFF})

	u, err := g.NewUUIDv7()
	if err != nil {
		t.Fatalf("NewUUIDv7: %v", err)
	}

	if got := u.Timestamp().UnixMilli(); got != ms {
		t.Errorf("Timestamp().UnixMilli() = %d, want %d", got, ms)
	}
	if got := u.Version(); got != 7 {
		t.Errorf("Version() = %d, want 7", got)
	}
	if got := u.Variant(); got != 0b10 {
		t.Errorf("Variant() = %#b, want 0b10", got)
	}

	// The first id of a millisecond has counter zero, so rand_a is zero and the
	// whole 16 bytes are predictable: 6 timestamp bytes, 0x70, 0x00, then the
	// entropy with the variant bits stamped over the top of the first byte.
	want := "017f22e2-79b0-7000-bfff-ffffffffffff"
	if got := u.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	// 0xFF entropy with variant 0b10 written over the top two bits is 0xBF, and
	// the assertion above would pass equally if the variant were never applied
	// and the entropy happened to be 0xBF. Stated separately so the two claims
	// do not lean on each other.
	if u[8] != 0xBF {
		t.Errorf("byte 8 = %#x, want 0xbf (0xff entropy with variant 0b10 stamped over it)", u[8])
	}
}

func TestIDsAreOrderedWithinAMillisecond(t *testing.T) {
	// Both legs of a trade are minted in far less than a millisecond, so the
	// timestamp alone does not order them. The rand_a counter does — and the
	// ordering has to survive into the TEXT, because that is what sorts on a
	// venue's order page and what SQLite indexes.
	const n = 100
	g := mustGenerator(t, newStubClock(1645557742000), constEntropy{0x00})

	var prev ids.UUID
	seen := make(map[string]bool, n)
	for i := range n {
		u, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if seen[u.String()] {
			t.Fatalf("mint %d: duplicate id %s within one millisecond", i, u)
		}
		seen[u.String()] = true

		if i > 0 {
			if bytes.Compare(u[:], prev[:]) <= 0 {
				t.Fatalf("mint %d: %s does not sort after %s", i, u, prev)
			}
			// And in base32, which is the encoding a ClientOrderID carries.
			if u.Base32() <= prev.Base32() {
				t.Fatalf("mint %d: base32 %s does not sort after %s", i, u.Base32(), prev.Base32())
			}
		}
		prev = u
	}
}

// TestCounterIsPerMillisecondNotGlobal pins the counter's reset.
//
// It exists because a mutation that deleted the reset SURVIVED the rest of this
// file. Without the reset the counter only ever climbs, so ids stay unique and
// stay ordered — which is everything the other tests check. What actually breaks
// is quieter: after 4095 mints the generator has no counter room left in any
// millisecond, so every id borrows the next one and the timestamp drifts
// permanently ahead of the clock. The 48 bits that make a v7 sortable-by-time
// stop meaning a time.
//
// The property that catches it is that the state is per-millisecond: the first
// id of a millisecond must be exactly the id a generator that had never minted
// anything would produce there.
func TestCounterIsPerMillisecondNotGlobal(t *testing.T) {
	const ms = int64(1645557742000)

	used := newStubClock(ms)
	g := mustGenerator(t, used, constEntropy{0x7C})
	// Burn some counter room in the first millisecond.
	for i := range 40 {
		if _, err := g.NewUUIDv7(); err != nil {
			t.Fatalf("warm-up mint %d: %v", i, err)
		}
	}

	// Now step forward and take the first id of each of several new
	// milliseconds, comparing each against a generator with no history.
	for step := int64(1); step <= 5; step++ {
		used.set(ms + step)
		got, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("mint at +%dms: %v", step, err)
		}

		fresh := mustGenerator(t, newStubClock(ms+step), constEntropy{0x7C})
		want, err := fresh.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}

		if got != want {
			t.Fatalf("first id of millisecond +%d = %s, want %s — the counter carried over from the previous millisecond",
				step, got, want)
		}
	}
}

func TestCounterExhaustionBorrowsTheNextMillisecond(t *testing.T) {
	// rand_a is 12 bits, so a millisecond holds 4096 ids. The 4097th cannot
	// wrap — that would reissue an id already minted — and cannot block on a
	// clock the generator does not control. It borrows.
	const perMS = 4096
	const ms = int64(1645557742000)
	g := mustGenerator(t, newStubClock(ms), constEntropy{0x00})

	var prev ids.UUID
	for i := range perMS {
		u, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if got := u.Timestamp().UnixMilli(); got != ms {
			t.Fatalf("mint %d: timestamp %d, want %d before exhaustion", i, got, ms)
		}
		if i > 0 && bytes.Compare(u[:], prev[:]) <= 0 {
			t.Fatalf("mint %d: out of order", i)
		}
		prev = u
	}

	// The one past the end.
	borrowed, err := g.NewUUIDv7()
	if err != nil {
		t.Fatalf("mint %d: %v", perMS, err)
	}
	if got := borrowed.Timestamp().UnixMilli(); got != ms+1 {
		t.Errorf("timestamp after exhaustion = %d, want %d (borrowed)", got, ms+1)
	}
	if bytes.Compare(borrowed[:], prev[:]) <= 0 {
		t.Error("the borrowed id does not sort after the last id of the exhausted millisecond")
	}

	// And it self-corrects: once the clock catches up past the borrowed value,
	// minting returns to real time rather than running permanently ahead.
	c := newStubClock(ms + 5)
	g2 := mustGenerator(t, c, constEntropy{0x00})
	for range perMS + 1 {
		if _, err := g2.NewUUIDv7(); err != nil {
			t.Fatal(err)
		}
	}
	c.set(ms + 100)
	u, err := g2.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Timestamp().UnixMilli(); got != ms+100 {
		t.Errorf("timestamp after the clock caught up = %d, want %d — borrowing did not self-correct", got, ms+100)
	}
}

func TestBackwardsClockClamps(t *testing.T) {
	// An NTP step backwards, or a TestClock driven out of order. This is the
	// same choice clock.Wall makes and for the same reason: a stalled clock
	// beats a re-used identifier. Unlike Wall there IS an error channel here,
	// and it is deliberately not used — aborting a mint would abort a trade for
	// a reason with nothing to do with trading.
	const ms = int64(1645557742000)
	c := newStubClock(ms)
	g := mustGenerator(t, c, constEntropy{0x00})

	first, err := g.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}

	c.set(ms - 5000) // five seconds backwards
	second, err := g.NewUUIDv7()
	if err != nil {
		t.Fatalf("a backwards clock must not fail the mint: %v", err)
	}

	if got := second.Timestamp().UnixMilli(); got != ms {
		t.Errorf("timestamp = %d, want it clamped to %d", got, ms)
	}
	if bytes.Compare(second[:], first[:]) <= 0 {
		t.Error("the id minted after a backwards step does not sort after the one before it")
	}

	// And the clamp holds across many steps back, rather than only the first.
	prev := second
	for i := range 10 {
		c.set(ms - int64(6000+i*100))
		u, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if bytes.Compare(u[:], prev[:]) <= 0 {
			t.Fatalf("step %d: ordering broke under a repeatedly backwards clock", i)
		}
		prev = u
	}
}

func TestEntropyFailureLeavesTheGeneratorUntouched(t *testing.T) {
	// Everything that can fail happens before any state is mutated, so a failed
	// mint leaves the generator exactly as it was. The assertion is not that the
	// error is returned — that is easy — but that the NEXT id is the one that
	// would have been minted had the failure never happened.
	const ms = int64(1645557742000)
	boom := errors.New("entropy pool drained")

	// The reference sequence: two mints, nothing goes wrong.
	clean := mustGenerator(t, newStubClock(ms), constEntropy{0xAA})
	want := make([]ids.UUID, 2)
	for i := range want {
		u, err := clean.NewUUIDv7()
		if err != nil {
			t.Fatalf("reference mint %d: %v", i, err)
		}
		want[i] = u
	}

	// The same sequence with a failure wedged between the two, twice over — a
	// single failure and a run of them, because a state mutation that happens
	// once per failure and a state mutation that happens once are different
	// bugs and only the second shows up with one.
	for _, failures := range []int{1, 3} {
		entropy := &failingEntropy{b: 0xAA, failOn: 2, err: boom}
		g := mustGenerator(t, newStubClock(ms), entropy)

		got0, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("first mint: %v", err)
		}
		if got0 != want[0] {
			t.Fatalf("first mint diverged before any failure: %s vs %s", got0, want[0])
		}

		for i := range failures {
			entropy.failOn = entropy.n + 1
			_, err := g.NewUUIDv7()
			if !errors.Is(err, ids.ErrEntropy) {
				t.Fatalf("failure %d: err = %v, want ErrEntropy", i, err)
			}
			// The underlying cause stays reachable, so a caller can tell a
			// drained pool from a closed file.
			if !errors.Is(err, boom) {
				t.Errorf("failure %d: the underlying error is not reachable through errors.Is", i)
			}
		}

		entropy.failOn = 0 // never again
		got1, err := g.NewUUIDv7()
		if err != nil {
			t.Fatalf("mint after %d failure(s): %v", failures, err)
		}
		if got1 != want[1] {
			t.Errorf("after %d failed mint(s) the sequence diverged: got %s, want %s — a failure consumed generator state",
				failures, got1, want[1])
		}
	}
}

func TestGeneratorRejectsBadTime(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want error
	}{
		{
			// The case that actually happens: a TestClock nobody advanced, or a
			// struct field nobody populated. The zero time.Time is year 1, so
			// its UnixMilli is hugely negative and a uint64 conversion would
			// wrap it into a timestamp far in the future.
			name: "zero time", at: time.Time{}, want: ids.ErrTimeUnset,
		},
		{
			name: "before the epoch", at: time.UnixMilli(-1).UTC(), want: ids.ErrTimeUnset,
		},
		{
			// Year 10889 and beyond. Unreachable in practice and checked
			// because the alternative is truncating the high bits and minting
			// an id that sorts before every id ever issued.
			name: "past the 48-bit ceiling", at: time.UnixMilli(1 << 48).UTC(), want: ids.ErrTimeRange,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGenerator(t, &stubClock{at: tc.at}, constEntropy{0x00})
			if _, err := g.NewUUIDv7(); !errors.Is(err, tc.want) {
				t.Errorf("NewUUIDv7() = %v, want %v", err, tc.want)
			}
		})
	}

	// Positive control: one millisecond inside the ceiling must succeed, or the
	// bound above would pass with an off-by-one in either direction.
	g := mustGenerator(t, &stubClock{at: time.UnixMilli(1<<48 - 1).UTC()}, constEntropy{0x00})
	if _, err := g.NewUUIDv7(); err != nil {
		t.Errorf("NewUUIDv7() = %v at the last representable millisecond", err)
	}
}

func TestNewGeneratorRequiresBothSeams(t *testing.T) {
	// Neither is defaulted. A nil clock would panic on first use far from the
	// wiring mistake; a defaulted crypto/rand would silently make a backtest
	// non-reproducible, which is worse — the backtest would still run and its
	// output would still look like a result.
	if _, err := ids.NewGenerator(nil, constEntropy{0x00}); err == nil {
		t.Error("NewGenerator(nil, entropy) = nil error")
	}
	if _, err := ids.NewGenerator(newStubClock(1), nil); err == nil {
		t.Error("NewGenerator(clock, nil) = nil error")
	}
	if _, err := ids.NewGenerator(newStubClock(1), constEntropy{0x00}); err != nil {
		t.Errorf("NewGenerator with both seams = %v", err)
	}
}

func TestDeterminismUnderIdenticalInputs(t *testing.T) {
	// This is the property §3.5's parity rests on: same clock, same entropy,
	// same sequence. If it ever fails, a backtest stops being a replay.
	mint := func() []string {
		c := newStubClock(1645557742000)
		g := mustGenerator(t, c, constEntropy{0x5A})
		var out []string
		for i := range 50 {
			if i%7 == 0 {
				c.set(c.at.UnixMilli() + 1)
			}
			u, err := g.NewUUIDv7()
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, u.String())
		}
		return out
	}

	first, second := mint(), mint()
	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Error("two runs over identical inputs produced different id sequences")
	}
}

func TestConcurrentMintingIsUniqueAndValid(t *testing.T) {
	// Determinism is explicitly NOT claimed under concurrency — the entropy
	// reader is drawn from outside the lock, so two racing goroutines take
	// whatever order they win. Uniqueness IS claimed, and it is what the mutex
	// is for. Run under -race, which is what `make test` does.
	const goroutines, each = 8, 200
	g := mustGenerator(t, newStubClock(1645557742000), constEntropy{0x11})

	var mu sync.Mutex
	seen := make(map[ids.UUID]bool, goroutines*each)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				u, err := g.NewUUIDv7()
				if err != nil {
					t.Errorf("NewUUIDv7: %v", err)
					return
				}
				if err := u.Validate(); err != nil {
					t.Errorf("Validate: %v", err)
					return
				}
				mu.Lock()
				if seen[u] {
					t.Errorf("duplicate id %s", u)
				}
				seen[u] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != goroutines*each {
		t.Errorf("minted %d distinct ids, want %d", len(seen), goroutines*each)
	}
}

func TestTypedIDsAreMintedValid(t *testing.T) {
	g := mustGenerator(t, newStubClock(1645557742000), constEntropy{0x33})

	trade, err := g.NewTradeID()
	if err != nil {
		t.Fatal(err)
	}
	if err := trade.Validate(); err != nil {
		t.Errorf("TradeID.Validate() = %v on a freshly minted id", err)
	}
	round, err := ids.ParseTradeID(trade.String())
	if err != nil {
		t.Fatalf("ParseTradeID(%q) = %v", trade, err)
	}
	if round != trade {
		t.Errorf("TradeID did not round-trip: %v vs %v", round, trade)
	}

	intent, err := g.NewIntentID()
	if err != nil {
		t.Fatal(err)
	}
	if err := intent.Validate(); err != nil {
		t.Errorf("IntentID.Validate() = %v", err)
	}

	// The zero values are unusable, which is what catches a struct field nobody
	// populated. Without it the Nil UUID would address one shared trade.
	if err := (ids.TradeID{}).Validate(); !errors.Is(err, ids.ErrUUIDUnset) {
		t.Errorf("zero TradeID validates as %v, want ErrUUIDUnset", err)
	}
	if err := (ids.IntentID{}).Validate(); !errors.Is(err, ids.ErrUUIDUnset) {
		t.Errorf("zero IntentID validates as %v, want ErrUUIDUnset", err)
	}

	// ParseTradeID must reject a well-formed non-v7, or an id from another
	// system would become a TradeID.
	if _, err := ids.ParseTradeID("017f22e2-79b0-4cc3-98c4-dc0c0c07398f"); !errors.Is(err, ids.ErrUUIDVersion) {
		t.Errorf("ParseTradeID on a v4 = %v, want ErrUUIDVersion", err)
	}
}
