package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// FundingEntry is one instrument's latest funding reading as the cache holds it.
//
// # It has no Freshness field, and that absence is the design
//
// ports.FundingSnapshot arrives carrying Transport and Data verdicts computed at
// ACCEPT time. They are dropped here rather than stored, because an observation
// sitting in a cache ages while nothing arrives to re-judge it — so a stored
// verdict reports the feed healthy for the whole outage, which is the failure
// §3.4 builds its silence timer to prevent. The package doc carries the full
// argument and the two rejected alternatives.
//
// What is kept is the Watermark, and the ages derived from it at an instant the
// CALLER supplies. Re-judging is therefore forced to the moment of the read.
type FundingEntry struct {
	// Observation is the accepted reading, already validated by marketdata.
	Observation domain.FundingObservation

	// GapDetected reports a break in the venue's own sequence for this source.
	// Always false for a source supplying no sequence — read SeqAvailable before
	// reading this as an assurance, which is ports.FundingSnapshot's own rule
	// carried across unchanged.
	GapDetected bool

	// SeqAvailable reports whether the venue supplies a sequence here at all. A
	// REST-polled funding read carries none, which §3.5 makes the ordinary case
	// rather than an error.
	SeqAvailable bool

	// Trigger is the AcceptOrdinal of the raw input this reading came from, or
	// zero when a silence timer derived it from no input at all. Carried so a
	// cached value stays traceable to the input that caused it — §3.5's causal
	// traceability does not stop at the lane.
	Trigger ports.AcceptOrdinal
}

// Watermark is the reading's provenance, which is what every age is measured
// from.
func (e FundingEntry) Watermark() domain.Watermark { return e.Observation.Watermark }

// Age is how long since this process took delivery, at the instant now. It is
// the transport-liveness question; see domain.Watermark.Age, including why a
// clock inversion is an error rather than a negative duration.
func (e FundingEntry) Age(now time.Time) (time.Duration, error) {
	return e.Observation.Watermark.Age(now)
}

// DataAge is how old the venue's own view was, against that venue's server time.
// See domain.Watermark.DataAge for why this is a separate question from Age.
func (e FundingEntry) DataAge(serverNow time.Time) (time.Duration, error) {
	return e.Observation.Watermark.DataAge(serverNow)
}

// BookEntry is one instrument's latest reconstructed book. Same shape and same
// missing field as FundingEntry, for the same reason.
type BookEntry struct {
	// Book is the reconstructed, validated snapshot.
	Book domain.BookSnapshot

	// Applied is how the input that produced this book related to the book it
	// was applied to. SeqRelationUnknown means a full snapshot REPLACED the book
	// rather than following it, which is what that value means on the domain
	// type — a consumer counting resyncs reads this.
	Applied domain.SeqRelation

	// Trigger is the AcceptOrdinal of the raw input this book came from, or zero
	// for a silence-timer re-judgement.
	Trigger ports.AcceptOrdinal
}

// Watermark is the book's provenance. Its VenueSeq is the last sequence applied.
func (e BookEntry) Watermark() domain.Watermark { return e.Book.Watermark }

// Age is how long since this process took delivery, at the instant now.
func (e BookEntry) Age(now time.Time) (time.Duration, error) {
	return e.Book.Watermark.Age(now)
}

// DataAge is how old the venue's own view was, against that venue's server time.
func (e BookEntry) DataAge(serverNow time.Time) (time.Duration, error) {
	return e.Book.Watermark.DataAge(serverNow)
}

// Cache is §3.2's in-memory latest-value cache.
//
// # One map per data class, and a merged one was rejected
//
// §3.4's classes are independent and their budgets are not interchangeable —
// "order books used for execution need sub-second budgets... funding metadata
// tolerates longer" — and marketdata's ports already split FundingBudgets from
// BookBudgets on exactly that ground. A single map keyed by events.SourceID with
// an `any` payload would unify the storage and lose the types, so every read
// would end in a type assertion that the compiler cannot check and that a wrong
// StreamKind would fail at runtime, in a consumer, on a value it was told was a
// book.
//
// # The zero value is USABLE here, and it is the one exception worth arguing
//
// Every constructed type in platform/ fails Validate() when nobody populated it,
// because an unpopulated cost reads as free. The direction reverses for a cache:
// its empty state is not a mis-populated configuration, it is the honest state
// of a process that has just booted and observed nothing. Every reader gets
// (zero, false) — never a value — so an empty cache cannot be mistaken for data,
// which is the property the platform/ rule is actually protecting. NewCache
// exists to allocate the maps, not to make the zero value unreachable.
type Cache struct {
	// mu guards both maps. Present despite §3.4's single-writer sentence, which
	// is a claim about who may MUTATE this and not about concurrency — the
	// readers are strategy, risk and oms, on their own goroutines. See the
	// package doc.
	mu      sync.RWMutex
	funding map[ids.VenueInstrument]FundingEntry
	books   map[ids.VenueInstrument]BookEntry
}

// NewCache returns an empty cache.
//
// It cannot fail, and so it does not return an error. Every other constructor in
// this repo refuses an incomplete dependency set; this one has no dependencies
// to refuse — the budgets it might have taken are deliberately not here (package
// doc), and the clock it might have taken is supplied per call, which is what
// keeps every age in one decision measured against one instant.
func NewCache() *Cache {
	return &Cache{
		funding: make(map[ids.VenueInstrument]FundingEntry),
		books:   make(map[ids.VenueInstrument]BookEntry),
	}
}

// PutFunding stores a published funding snapshot as the latest value for its
// instrument.
//
// # It re-validates what marketdata already validated, and that is not
// # redundant
//
// The snapshot crossed a lane to get here, and §3.5 makes that lane's producer
// and consumer substitutable — a replay MarketDataSource in a backtest produces
// these with no marketdata use case in the process at all. A check that only the
// live path performs is a check the parity seam removes.
//
// # A REGRESSION is refused rather than stored
//
// A latest-value cache whose latest value goes backwards is not a cache with an
// old number in it: because every age here is measured from RecvTime, storing an
// earlier observation makes the cache report data that is STALER than it was a
// moment ago, on a feed that just delivered. The freshness ladder then walks
// down while the transport is healthy, and the one thing that would have
// explained it — the newer reading — has been overwritten by the older one.
//
// §3.4's per-SourceID mailboxes are FIFO, so within one source this should not
// arise. The dispatcher (engine/dispatch) is now built and preserves per-source
// FIFO through dispatch — and this check STAYS, ratified by the dispatcher's
// own design record (§39h): a consumer that verifies an invariant with a single
// comparison is strictly better than one that trusts it, the comparison is
// already bite-checked, and removing it would convert a documented defence into
// a documented regression for zero saved work.
//
// # EQUAL is accepted, and refusing it would break the backtester
//
// The comparison is strictly-before, not not-after. Under clock.TestClock — the
// §3.5 parity seam — time advances only when the harness advances it, so two
// observations dispatched between advances share a RecvTime exactly. Refusing
// equality would make a backtest drop every update after the first at each
// instant, which is a divergence from live that the parity claim would not
// survive and that no test of the live path could see.
func (c *Cache) PutFunding(s ports.FundingSnapshot) error {
	if err := s.Observation.Validate(); err != nil {
		return fmt.Errorf("state: funding observation: %w", err)
	}

	key := s.Observation.Instrument
	entry := FundingEntry{
		Observation:  s.Observation,
		GapDetected:  s.GapDetected,
		SeqAvailable: s.SeqAvailable,
		Trigger:      s.Trigger,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.funding == nil {
		c.funding = make(map[ids.VenueInstrument]FundingEntry)
	}
	if held, ok := c.funding[key]; ok {
		if err := notBefore(held.Watermark(), entry.Watermark(), key); err != nil {
			return err
		}
	}
	c.funding[key] = entry
	return nil
}

// PutBook stores a reconstructed book as the latest value for its instrument.
// Same refusals as PutFunding, and see it for each one's reason.
func (c *Cache) PutBook(v ports.BookView) error {
	if err := v.Book.Validate(); err != nil {
		return fmt.Errorf("state: book snapshot: %w", err)
	}

	key := v.Book.Instrument
	entry := BookEntry{Book: v.Book, Applied: v.Applied, Trigger: v.Trigger}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.books == nil {
		c.books = make(map[ids.VenueInstrument]BookEntry)
	}
	if held, ok := c.books[key]; ok {
		if err := notBefore(held.Watermark(), entry.Watermark(), key); err != nil {
			return err
		}
	}
	c.books[key] = entry
	return nil
}

// notBefore is the regression check, written once so the two write paths cannot
// disagree about which direction is backwards.
//
// It compares RecvTime rather than VenueSeq deliberately. §3.5 makes VenueSeq
// "authoritative only within a SourceID", and a full book snapshot replaces
// rather than follows — domain.SeqRelationUnknown is exactly that case — so a
// resnapshot legitimately arrives with a sequence unrelated to the one held. A
// sequence comparison would reject the recovery that fixes a desynced book,
// which is the one write this cache most needs to accept. RecvTime carries no
// such exemption: this process stamped both, from one clock.
func notBefore(held, next domain.Watermark, key ids.VenueInstrument) error {
	if next.RecvTime.Before(held.RecvTime) {
		return fmt.Errorf("state: %s received %s, cached %s: %w",
			key, next.RecvTime, held.RecvTime, ErrRegression)
	}
	return nil
}

// Funding returns the latest funding reading for an instrument, and whether
// there is one.
//
// The bool is not an error: nothing observed yet is the ordinary state of a
// freshly booted process, and §3.4's FreshnessUnknown exists to say so on the
// other side. A caller that treats absence as a fault would report every venue
// as failed for the first few seconds of every run.
func (c *Cache) Funding(vi ids.VenueInstrument) (FundingEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.funding[vi]
	return e, ok
}

// Book returns the latest reconstructed book for an instrument, and whether
// there is one. Same bool, same reason as Funding.
func (c *Cache) Book(vi ids.VenueInstrument) (BookEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.books[vi]
	return e, ok
}

// BookSkew is §3.4's max cross-leg skew across the legs of one decision,
// measured over their latest cached books.
//
// # A missing leg is an ERROR here, where it is a bool on Book
//
// The two readers are asked different questions. Book is asked about one
// instrument, and "not yet" is a real answer. This is asked about a PACKAGE, and
// a package one of whose legs has never been observed has no skew — scoring it
// would mean comparing the legs that happen to be present, which returns a
// smaller number the less the cache knows. That is the direction that reads as
// SAFE, on the input where the engine knows least.
//
// The books rather than the funding readings, because §3.4 attaches the skew
// limit to execution: "order books used for execution need sub-second budgets +
// a cross-leg skew limit". Funding metadata is on an eight-hour cadence and a
// skew across two funding reads is not a quantity any decision consumes.
func (c *Cache) BookSkew(vis ...ids.VenueInstrument) (time.Duration, error) {
	// The length check happens inside Skew, on the watermarks — but a gather
	// over one instrument would otherwise take the lock and touch the map before
	// reaching it, and more importantly a caller passing one leg deserves the
	// undefined-skew sentinel rather than an absent-entry one for whichever leg
	// it did name. Skew owns that rule; this defers to it.
	ws := make([]domain.Watermark, 0, len(vis))

	c.mu.RLock()
	for _, vi := range vis {
		e, ok := c.books[vi]
		if !ok {
			c.mu.RUnlock()
			return 0, fmt.Errorf("state: book for %s: %w", vi, ErrEntryAbsent)
		}
		ws = append(ws, e.Watermark())
	}
	c.mu.RUnlock()

	return Skew(ws...)
}

// Len reports how many instruments the cache holds an entry for, per class.
//
// For diagnostics and for the observability lane, never for a trade decision:
// a count says nothing about whether any of it is fresh, and §3.4's whole
// freshness apparatus exists because those are different questions.
func (c *Cache) Len() (funding, books int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.funding), len(c.books)
}
