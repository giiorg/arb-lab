// Package state is §3.2's latest-value cache: the in-memory store that sits
// between marketdata and every consumer that prices against it.
//
// §3.1's pipeline puts it in one line — engine/venue → marketdata →
// engine/state → strategy.Evaluate — and §3.2's row gives it four jobs:
// "in-memory latest-value cache; single-writer; serves consumer-owned read
// ports; translates to the strategy-owned MarketView".
//
// # It has no DDD layering, for engine/venue's reason
//
// §3.2 divides internal/ into bounded contexts, which get domain/ +
// application/ + infrastructure/, and engine subsystems, which do not. This is
// an engine subsystem, and the test engine/venue's doc states applies here too:
// there is no domain LANGUAGE here. A cache holds what somebody else decided; the
// meaning of a funding rate is marketdata's and strategy's, and this package
// holds neither meaning.
//
// # It imports marketdata's ports, and the DUPLICATION would have been the
// # unjustified choice
//
// venue.Funding and marketdata/domain.FundingObservation are two types for one
// reading, and venue's doc is careful about why: §3.6 rule 1 FORBIDS a context's
// domain/ from importing engine/, so the domain cannot be expressed in terms of
// the venue DTO. The split is forced, and its cost is paid in one visible
// translation.
//
// Nothing forces it here. Rule 1 classifies the IMPORTER, and this package is
// neither a domain/ nor an application/, so engine/state → marketdata/…/ports
// is an edge the gate is silent about — as `hexboundary`'s own fixtures record,
// it is the inward layers that are policed, not everything they touch. What
// flows down §3.1's pipeline IS ports.FundingSnapshot and ports.BookView, so a
// third struct with the same fields would be duplication with no rule demanding
// it, and venue's doc frames that duplication as a price paid only under
// compulsion.
//
// # Single-writer is a claim about AUTHORITY, and it is not a claim about
// # synchronization — the mutex is not redundant
//
// §3.4: "engine/state is single-writer (the marketdata reducer) handing
// immutable MarketViews to readers." That sentence names who may WRITE. It says
// nothing about who may read, and the answer is: strategy, risk and oms, from
// their own goroutines, concurrently with that writer.
//
// The reading worth refusing out loud is "single-writer, therefore no lock
// needed", which is true of a single-threaded actor and false of this: one
// writer plus many concurrent readers over a Go map is a data race, and the race
// detector's report would name a map access rather than the architectural
// sentence that licensed it. So the maps are guarded by a sync.RWMutex, and
// "single-writer" continues to mean what §3.2 and the authoritative-state
// principle mean by it — that exactly one component is ALLOWED to mutate this —
// which is an invariant no mutex can supply.
//
// # The published freshness verdict does NOT survive this cache
//
// [SPEC EDIT OWED] §3.4 requires a silence timer because "a feed that simply
// stops emits no event, so staleness MUST be detected by a timer, not only by
// the next update". A latest-value cache is the same defect wearing different
// clothes, and the spec does not name it: ports.FundingSnapshot arrives carrying
// Transport and Data verdicts that marketdata computed at ACCEPT time, and an
// observation sitting in a cache ages without anything arriving to re-judge it.
// A cache that stored those fields and handed them back would report
// FreshnessFresh for the entire outage — the exact failure §3.4 built the timer
// to prevent, reintroduced one hop downstream.
//
// The response is structural rather than careful: FundingEntry and BookEntry
// carry NO Freshness field at all, so the stale verdict is not merely unused,
// it is unrepresentable. What they carry is the Watermark, and Age/DataAge
// against an instant the caller supplies. Re-judging is therefore forced to
// happen at the instant of the READ, which is what marketdata's own ports doc
// already says freshness is a property of.
//
// Two verdicts were considered and rejected. Storing them alongside the
// re-derived ones keeps a field whose only use is to be read by mistake.
// Re-deriving them HERE would make this a second freshness authority holding a
// second copy of §5.2's budgets — and freshness.NextTransition's doc already
// records what this repo paid for one mechanism written down in two places.
//
// # Cross-leg skew lives here, because this is where the legs meet
//
// §3.4 lists it on the watermark — "source sequence, exchange timestamp,
// receive timestamp, and max cross-leg skew" — and marketdata/domain.Watermark
// deliberately does not have the field: "A single Watermark has nothing to be
// skewed against. It is computed where the legs meet, and putting a field for it
// here would give every producer a number it cannot fill in." This package is
// that place. See skew.go for which clock the measurement uses and why the
// spec's silence on that question is load-bearing.
//
// # What is DEFERRED, and the construct that makes it buildable
//
// The read port and the MarketView translation are not here. §3.3 makes
// MarketView "strategy-owned", §3.6 rule 1 makes the ports "consumer-owned", and
// the strategy context does not exist — so implementing the translation now
// means inventing both ends of a contract whose only real consumer arrives with
// §12 milestone 2's cashcarry.
//
// The deferral names the construct this package would READ rather than the
// feature that motivates it, which is §3.6's own rule about deferrals and the
// one rule 10's deferral got wrong: the trigger is the first CONSUMER-OWNED READ
// PORT — strategy's MarketView reader or §6 step 8's OMS-owned mark-risk port,
// whichever lands first — and not "the strategy context exists". A port is what
// this package implements; a context is merely where one tends to live.
//
// The mark data class is absent for the plainer reason marketdata's module.go
// already gives from the other side: there is no venue.Mark and no use case, so
// there is nothing to cache. §3.4's adverse-preserving coalescing rule for the
// mark is likewise not here and does not belong here — platform/events already
// owns it as AdversePreserving, applied on the lane ahead of this cache, which
// is where §3.4 puts coalescing.
package state
