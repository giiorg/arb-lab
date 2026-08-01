package crossvenue

// The Bybit book accumulator (§45d): applies the orderbook stream's deltas
// to a running book. It is walker machinery, not venue wire knowledge —
// the decoder owns the bytes, marketdata's domain accumulator owns the
// live pipeline, and this one owns the archive walk's reconstruction.
//
// # The follows rule is a measured venue fact, not an assumption
//
// Within a connection every delta advances `u` by EXACTLY 1 — Profiles §5's
// 2026-08-01 own-archive observation (8.46M deltas, zero exceptions, both
// products). So a delta with u ≠ lastU+1 is a sequence break, and the only
// honest recovery is to drop the book and re-anchor at the next snapshot:
// a monotonic-but-gapped u is NOT a healthy feed, and absorbing it would
// walk depth measurements over a book with silent holes. The venue
// re-snapshots on subscribe (also measured: snapshots appear only there),
// so a break heals at the next reconnect or resubscribe.
//
// # Deletes of absent levels are window slide, not desync
//
// The stream maintains a 50-level window. A level that sat beyond the
// window at snapshot time can slide in and then be deleted — a delete this
// book never saw inserted. That is the wire's geometry, not evidence of a
// break, so it is a counted no-op. The mirrored artifact is levels of OURS
// that slid OUT of the venue's window and stopped being maintained:
// Sample truncates to the venue's own 50 per side, which bounds staleness
// to the window edge instead of accumulating an unmaintained tail.

import (
	"sort"

	"github.com/giiorg/arb-lab/internal/engine/venue"
)

// AccumulatorStats counts what the accumulator consumed and what it
// refused to absorb. Breaks and ColdDeltas are drop-and-count, never
// silent: a walk with a thousand breaks is a different measurement from a
// clean one, and only these counters can say so.
type AccumulatorStats struct {
	// Snapshots and DeltasApplied count frames absorbed into the book.
	Snapshots     int
	DeltasApplied int

	// ColdDeltas counts deltas arriving with no anchored book — a file
	// opened mid-connection, or the stretch between a break and the next
	// snapshot.
	ColdDeltas int

	// Breaks counts sequence breaks: a delta whose u was not lastU+1.
	// Each one drops the book until the next snapshot re-anchors.
	Breaks int

	// DeletesAbsent counts window-slide deletes (see the package comment):
	// tolerated no-ops, counted because a surge of them under a healthy
	// sequence would be the signature of a wire fact this file has wrong.
	DeletesAbsent int
}

// BookAccumulator reconstructs one Bybit book stream. The zero value is
// cold: nothing samples until a snapshot anchors it.
type BookAccumulator struct {
	bids map[int64]venue.BookLevel
	asks map[int64]venue.BookLevel

	anchored bool
	lastU    uint64

	stats AccumulatorStats
}

// NewBookAccumulator returns a cold accumulator.
func NewBookAccumulator() *BookAccumulator {
	return &BookAccumulator{}
}

// Stats reports the running counters.
func (a *BookAccumulator) Stats() AccumulatorStats { return a.stats }

// Anchored reports whether a book is currently held.
func (a *BookAccumulator) Anchored() bool { return a.anchored }

// UpdateID is the venue position of the held book — the u the last
// absorbed frame carried. Meaningful only while Anchored.
func (a *BookAccumulator) UpdateID() uint64 { return a.lastU }

// Drop unanchors the book without counting a break: the walker calls it
// at a recorded capture gap or a file boundary, where the hole is the
// capture's fact rather than the venue sequence's, and the next snapshot
// re-anchors exactly as after a break.
func (a *BookAccumulator) Drop() {
	a.anchored = false
	a.bids, a.asks = nil, nil
}

// Apply absorbs one decoded frame. A snapshot (re)anchors uncondition-
// ally — the venue's own rule is "a new snapshot means reset". A delta
// either follows (u == lastU+1) and is applied, or breaks the sequence
// and drops the book until the next snapshot.
func (a *BookAccumulator) Apply(msg venue.BybitBookMessage) {
	switch msg.Kind {
	case venue.BybitBookSnapshotKind:
		a.bids = make(map[int64]venue.BookLevel, len(msg.Snapshot.Bids))
		a.asks = make(map[int64]venue.BookLevel, len(msg.Snapshot.Asks))
		for _, l := range msg.Snapshot.Bids {
			a.bids[l.Price.Mantissa()] = l
		}
		for _, l := range msg.Snapshot.Asks {
			a.asks[l.Price.Mantissa()] = l
		}
		a.anchored = true
		a.lastU = msg.UpdateID
		a.stats.Snapshots++

	case venue.BybitBookDeltaKind:
		if !a.anchored {
			a.stats.ColdDeltas++
			return
		}
		if msg.UpdateID != a.lastU+1 {
			a.anchored = false
			a.bids, a.asks = nil, nil
			a.stats.Breaks++
			return
		}
		for _, c := range msg.Delta.Changes {
			side := a.bids
			if c.Side == venue.SideAsk {
				side = a.asks
			}
			key := c.Price.Mantissa()
			sign, err := c.Quantity.Sign()
			if err != nil {
				// The decoder validates every change; an unusable quantity
				// here is a programming error upstream, and absorbing the
				// change would corrupt the book silently. Treat it as a
				// break: drop the book, count, re-anchor at the snapshot.
				a.anchored = false
				a.bids, a.asks = nil, nil
				a.stats.Breaks++
				return
			}
			if sign == 0 {
				if _, ok := side[key]; !ok {
					a.stats.DeletesAbsent++
					continue
				}
				delete(side, key)
				continue
			}
			side[key] = venue.BookLevel{Price: c.Price, Quantity: c.Quantity}
		}
		a.lastU = msg.UpdateID
		a.stats.DeltasApplied++
	}
}

// Sample renders the held book best-first — bids descending, asks
// ascending, each side truncated to the venue's 50-level window (the
// package comment's staleness bound). The bool is false while cold: an
// unanchored instant is a skip the caller counts, never an empty book,
// because an empty book is a real market state and this is not that.
func (a *BookAccumulator) Sample() (venue.BookSnapshot, bool) {
	if !a.anchored {
		return venue.BookSnapshot{}, false
	}
	out := venue.BookSnapshot{
		Bids: sortedLevels(a.bids, true),
		Asks: sortedLevels(a.asks, false),
	}
	if len(out.Bids) > bybitBookDepth {
		out.Bids = out.Bids[:bybitBookDepth]
	}
	if len(out.Asks) > bybitBookDepth {
		out.Asks = out.Asks[:bybitBookDepth]
	}
	return out, true
}

// bybitBookDepth is the subscribed window's depth — orderbook.50, the
// §42c roster's one book topic on this venue.
const bybitBookDepth = 50

func sortedLevels(m map[int64]venue.BookLevel, descending bool) []venue.BookLevel {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if descending {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})
	out := make([]venue.BookLevel, len(keys))
	for i, k := range keys {
		out[i] = m[k]
	}
	return out
}
