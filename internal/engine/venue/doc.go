// Package venue is §4's venue seam: the normalized DTOs every context reads a
// venue through, and the per-venue translation that produces them.
//
// # It has no DDD layering, and that is §3.2's rule rather than an omission
//
// §3.2 divides internal/ into bounded contexts, which get domain/ +
// application/ + infrastructure/, and engine subsystems, which do not. This is
// an engine subsystem. There is no domain here because there is no domain
// LANGUAGE here: a funding rate means something to marketdata and something
// else to strategy, and what this package owns is neither meaning — it is the
// shape both of them agree the venue's reading arrives in.
//
// # What §4 puts here, and what is deliberately not here yet
//
// §4 gives this package two jobs: "raw Binance/Bybit clients and normalized
// DTOs". Only the second is built. The clients are absent because nothing can
// consume one — §3.5's dispatcher is explicitly "not yet built", there is no
// composition root under app/, and a WS client with no dispatcher to feed would
// have to invent its own arbitration, which is the second-authority defect §3.5
// spends a paragraph forbidding. When they land they land beside this file.
//
// §4's nine capability interfaces are likewise not declared here, and the
// reason is the same one §3.6's gate status gives for deferring singlewriter: a
// gate over an empty tree passes trivially, and an interface with no
// implementer and no caller is a declaration nothing can disagree with. Eight of
// the nine are consumed by contexts that do not exist (oms, treasury, risk).
// The ninth is discussed below. §4's own rule is that contexts declare the
// segregated capability they need in their OWN application layer and this
// package satisfies it — which is what marketdata already does, so the
// interfaces here would be a third statement of a contract that already has
// two ends.
//
// # [SPEC EDIT OWED] §3.5 and §4 declare the market-data seam twice, with
// # different shapes, and neither says which one wins
//
// §3.5's seam block gives:
//
//	type DataSource interface { Subscribe(VenueInstrument) <-chan Envelope }
//
// while §4 gives StreamBook / StreamFunding / StreamMark, each taking a
// VenueInstrument and returning its own channel. These are the same position in
// the pipeline — both are what the dispatcher reads and what §8's backtest
// substitutes a replay adapter for — and a type satisfying one does not satisfy
// the other.
//
// §3.5's block is a REFERENCE to §4 everywhere else in it: ExecutionVenue is
// written as a composition of §4's four interfaces and says "see §4" inline.
// DataSource is the one line written as a definition instead, and its signature
// is the one that has not kept up. Three things say §4's per-stream shape is
// the authoritative one:
//
//   - §3.4 gives every SourceID its own mailbox, and a SourceID is
//     (instrument, stream). One channel per instrument merges three of them, so
//     the subscriber has to demultiplex by reading Envelope.Source — re-deriving
//     a key the seam could have kept.
//   - The three streams do not exist for the same instruments. §4 marks funding
//     and mark as perp-only and book as both. Subscribe(VenueInstrument) has no
//     way to say that, so a spot subscription silently yields an instrument with
//     a third of the streams and nothing names which third.
//   - A consumer that needs only funding would open a book subscription to get
//     it. §7.6 makes venue capacity a budgeted resource with a priority ladder;
//     an interface shape that forces an unused high-rate feed spends that budget
//     on nothing.
//
// The edit owed is small and it is a resolution, not a new rule: DataSource is
// MarketDataSource under §3.5's parity-seam name, and the per-stream shape is
// authoritative. It is recorded here rather than resolved by declaring the
// interface, because declaring it is the empty-tree case above — it gets built
// with the first client that implements it and the first dispatcher that calls
// it, in one slice, where a wrong shape would actually fail something.
//
// # Why the normalized DTO and marketdata's domain type are two types
//
// venue.Funding and marketdata/domain.FundingObservation overlap, and the
// overlap is structural rather than accidental: §3.6 rule 1 forbids a context's
// domain/ from importing engine/, so the domain CANNOT be expressed in terms of
// this type. That is the anti-corruption layer §4 asks for, and its cost is
// paid in one visible place — the field-by-field translation in marketdata's
// driving adapter — rather than spread across a shared struct that both layers
// would then be free to add to.
//
// Both types validate, and neither check is redundant with the other in the way
// it first looks. This one judges a VENUE READING and has producers the domain
// will never see: §8's capture lane records normalized DTOs, and §3.5's replay
// DataSource produces them with no context in the process at all. The domain's
// judges an OBSERVATION, which is this plus an instrument and a watermark, and
// it is the stronger check wherever both apply. Deleting either leaves a
// boundary that nothing checks.
package venue
