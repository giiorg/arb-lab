// Package state is the clean engine fixture, and it is the positive control the
// widening needs. Without it the suite would only ever show engine code being
// rejected, and a universally-objecting gate would read as a perfect result.
//
// It is also the package that motivates the widening most sharply. engine/state
// runs under BOTH live and replay wiring, and freshness here is now minus
// RecvTime. Read ambiently under replay, that compares a replayed timestamp
// against real wall-clock now, so every snapshot reads as infinitely stale and
// the §3.4 silence timer misfires. Through the injected Clock it is correct in
// both modes, with no annotation and no configured exemption — the seam is
// expressed by taking the port.
package state

import "time"

// Clock is the injected port, declared locally so the fixture stays a single
// self-contained package. The real one is §3.5's platform/clock.
type Clock interface {
	Now() time.Time
}

type Snapshot struct {
	RecvTime time.Time
	Mark     int64
}

type Cache struct {
	clock Clock
	last  Snapshot
}

func NewCache(c Clock) *Cache { return &Cache{clock: c} }

// The computation the widening protects: one clock on both sides of the
// subtraction.
func (c *Cache) Age() time.Duration {
	return c.clock.Now().Sub(c.last.RecvTime)
}

// Freshness budgets are per-data-class (§3.4), and comparing two durations
// reads no clock.
func (c *Cache) Stale(budget time.Duration) bool {
	return c.Age() > budget
}

// Arithmetic and comparison on instants the caller already had stay legal in
// engine/ exactly as they are in domain/. The rule is about reading a clock,
// not about importing time.
func (c *Cache) SupersededBy(s Snapshot) bool {
	return s.RecvTime.After(c.last.RecvTime)
}

func (c *Cache) Apply(s Snapshot) { c.last = s }
