package ports

import (
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// VenueOrderIndex is the read port §5.3's gate 3 rules sweep rows out
// against: which venue order ids this system's own orders hold, per
// instrument.
//
// # A separate port rather than a method on OrderWriteRepository, on
// # HedgeGroupOrders' precedent
//
// The Resolver needs exactly this question answered and no write capability
// to answer it with; the write repository's Load is the read half of
// read-modify-write, which this is not. The same in-memory store satisfies
// both ports and is handed over as two capabilities — the wiring pattern the
// module root already uses for the journal's two halves.
//
// # The map is per INSTRUMENT, and the scope is the claim
//
// The venue fact this index rests on — Binance's orderId "is self-increment
// for each specific symbol" (Profiles §8) — is a per-symbol guarantee, so a
// venue id is a name only inside one instrument's namespace. An index keyed
// on the id alone would let a same-numbered order on another symbol rule out
// (or claim) a row it has no relation to, in the direction that proves a
// false absence. Implementations match the FULL instrument identity, all
// four components, for the reason HedgeGroupOrders deliberately matches
// fewer: a hedge group's spot and perp legs share exposure, but their venue
// order ids live in different counters.
//
// # One owner per key is a contract, not a hope
//
// An implementation MUST never return a map in which one venue id had two
// candidate owners — ApplyAck's ErrVenueOrderIDConflict is where that store
// state is refused, so an index built over an intact store cannot meet one.
// The clause is stated here because the map type cannot: a second owner
// silently overwrites the first, by iteration order, and the consumer would
// rule a row out against a coin flip.
type VenueOrderIndex interface {
	// VenueOrderIDsOn returns venue order id -> owning client order id for
	// every order this store holds on exactly this instrument. Orders with no
	// persisted venue id contribute nothing; an instrument with none returns
	// an empty map, which is a real answer (no row can be ruled out by
	// ownership, none can be claimed by it).
	VenueOrderIDsOn(instrument ids.VenueInstrument) (map[string]ids.ClientOrderID, error)
}
