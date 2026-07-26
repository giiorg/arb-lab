package ids

import (
	crand "crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"
)

// TimeSource is the injected clock seam.
//
// It is declared here rather than imported from platform/clock, and that is
// structural rather than stylistic: clock imports THIS package for Venue, so an
// import in the other direction would be a cycle. Go's interfaces are satisfied
// structurally, so clock.Wall and clock.TestClock — and anything else with a
// Now() time.Time — satisfy this without either package knowing about the other.
//
// One method, because a generator has one question to ask. Taking the whole
// clock.Clock interface would drag ServerNow and the timer methods into a
// dependency that mints identifiers.
type TimeSource interface {
	Now() time.Time
}

// Generator mints UUIDv7 identifiers.
//
// It holds mutable state — the last millisecond issued and a counter within it —
// because RFC 9562's timestamp alone does not order ids minted inside the same
// millisecond, and this system mints both legs of a trade in far less than one.
//
// # Determinism, and its exact limits
//
// With a deterministic TimeSource and a deterministic entropy reader, a
// single-goroutine caller gets an identical id sequence on every run. That is
// what §3.5's parity requires and what §3.4's single dispatcher provides.
//
// It is NOT deterministic under concurrent use: two goroutines racing read the
// entropy reader in whatever order they win the race, and the counter follows.
// Concurrent use is safe — the mutex guarantees uniqueness — but a backtest that
// minted ids from several goroutines would not reproduce. Stated rather than
// prevented, because the constraint belongs to the caller's architecture and
// this package cannot check it.
type Generator struct {
	mu      sync.Mutex
	time    TimeSource
	entropy io.Reader

	// lastMS is the last millisecond an id was issued for, after clamping and
	// any borrowing. Zero means none has been issued.
	lastMS uint64

	// counter occupies rand_a — the 12 bits between the version field and the
	// variant field — and orders ids minted within one millisecond.
	counter uint16
}

// maxCounter is the largest value rand_a's 12 bits hold.
const maxCounter = 0x0FFF

// maxTimestampMS is the largest value the 48-bit unix_ts_ms field holds, which
// runs out in the year 10889.
const maxTimestampMS = uint64(1)<<48 - 1

// NewGenerator builds a Generator over an injected clock and entropy source.
//
// Both are required and neither is defaulted. A nil TimeSource would panic on
// first use, far from the wiring mistake that caused it; a defaulted crypto/rand
// would silently make a backtest non-reproducible, which is worse, because the
// backtest would still run and its output would still look like a result. Use
// NewSystemGenerator for the live wiring.
func NewGenerator(ts TimeSource, entropy io.Reader) (*Generator, error) {
	if ts == nil {
		return nil, fmt.Errorf("ids: NewGenerator: time source is nil")
	}
	if entropy == nil {
		return nil, fmt.Errorf("ids: NewGenerator: entropy source is nil")
	}
	return &Generator{time: ts, entropy: entropy}, nil
}

// NewSystemGenerator builds a Generator over crypto/rand for the live binaries.
// The clock is still injected — §3.5's parity seam does not have an exception
// for "the live one".
func NewSystemGenerator(ts TimeSource) (*Generator, error) {
	return NewGenerator(ts, crand.Reader)
}

// NewUUIDv7 mints the next identifier.
//
// The ordering of the steps below is load-bearing. Everything that can fail —
// reading the clock, validating its range, drawing entropy — happens BEFORE any
// state is mutated, so a failed mint leaves the generator exactly as it was. The
// obvious alternative, advancing the counter and then discovering the entropy
// reader is broken, burns a counter slot per failure; harmless with 4096 of them
// and untidy in a way that makes the failure path hard to reason about.
func (g *Generator) NewUUIDv7() (UUID, error) {
	now := g.time.Now()

	unixMS := now.UnixMilli()
	if unixMS < 0 {
		// The zero time.Time lands here, which is the case that actually
		// happens: a TestClock that was never advanced, or a struct field
		// nobody populated.
		return UUID{}, fmt.Errorf("instant %v: %w", now, ErrTimeUnset)
	}
	if uint64(unixMS) > maxTimestampMS {
		return UUID{}, fmt.Errorf("instant %v: %w", now, ErrTimeRange)
	}

	var randB [8]byte
	if _, err := io.ReadFull(g.entropy, randB[:]); err != nil {
		return UUID{}, fmt.Errorf("%w: %w", ErrEntropy, err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	ms, counter, err := g.advance(uint64(unixMS))
	if err != nil {
		return UUID{}, err
	}

	var u UUID
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)
	// Version 7 in the high nibble of byte 6, the counter's high 4 bits below it.
	u[6] = 0x70 | byte(counter>>8&0x0f)
	u[7] = byte(counter)
	copy(u[8:], randB[:])
	// RFC 9562 variant 0b10 in the top two bits of byte 8, over the entropy.
	u[8] = u[8]&0x3f | 0x80
	return u, nil
}

// advance resolves the millisecond and counter for the next id, and is where
// every ordering guarantee this package makes actually lives. Caller holds mu.
func (g *Generator) advance(unixMS uint64) (ms uint64, counter uint16, err error) {
	switch {
	case unixMS > g.lastMS:
		// The common case: time moved on, so the counter restarts.
		//
		// It restarts at zero rather than at a random seed. RFC 9562 permits
		// either and suggests randomness for unguessability, but a random seed
		// would make the id sequence depend on entropy the backtester would
		// then have to reproduce — and unguessability is not a property this
		// counter needs. The 62 bits of rand_b carry that job.
		ms, counter = unixMS, 0

	case unixMS == g.lastMS:
		if g.counter >= maxCounter {
			// 4096 ids in one millisecond. Borrow the next millisecond rather
			// than wrap: wrapping would reissue an id already minted, and
			// stalling would block a caller on a clock it does not control.
			// Ids then run marginally ahead of the clock until it catches up,
			// which is bounded and self-correcting.
			ms, counter = unixMS+1, 0
		} else {
			ms, counter = unixMS, g.counter+1
		}

	default:
		// Time went backwards — an NTP step, or a TestClock driven out of
		// order. Clamp to the last millisecond issued and keep counting, which
		// is the same choice clock.Wall makes for the same reason: a stalled
		// clock beats a re-used identifier. Unlike Wall, there IS an error
		// channel here, and it is still not used — a mint that failed because
		// the host stepped its clock would abort a trade for a reason that has
		// nothing to do with trading.
		ms = g.lastMS
		if g.counter >= maxCounter {
			ms, counter = g.lastMS+1, 0
		} else {
			counter = g.counter + 1
		}
	}

	if ms > maxTimestampMS {
		// Only reachable by borrowing past the ceiling, which needs the clock
		// to already be at the year 10889. Checked because the alternative is
		// truncating the high bits and minting an id that sorts before every
		// id ever issued.
		return 0, 0, fmt.Errorf("borrowed past the ceiling: %w", ErrTimeRange)
	}

	g.lastMS, g.counter = ms, counter
	return ms, counter, nil
}

// NewTradeID and NewIntentID mint the typed identities. They are separate
// methods rather than one returning a UUID because the type is the point: a
// TradeID and an IntentID are both v7 UUIDs and assigning one to the other is a
// compile error, which is the same argument fixedpoint makes for keeping Money
// and Quantity apart.
func (g *Generator) NewTradeID() (TradeID, error) {
	u, err := g.NewUUIDv7()
	if err != nil {
		return TradeID{}, err
	}
	return TradeID{u: u}, nil
}

func (g *Generator) NewIntentID() (IntentID, error) {
	u, err := g.NewUUIDv7()
	if err != nil {
		return IntentID{}, err
	}
	return IntentID{u: u}, nil
}
