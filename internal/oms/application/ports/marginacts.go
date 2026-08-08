package ports

import (
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// VenueMarginActs is the venue's own record of margin-change ACTS — the second
// half of core §7.1's margin-op evidence contract, beside VenueMarginStatus's
// posted-margin delta (task #214).
//
// # It returns a DOMAIN type, where every other read port here returns an ACL
//
// MarginReading and MarkReading exist because §3.6 rule 1 forbids this package
// from importing `engine/`, so an adapter-side translation is the only shape
// available. That argument does not apply to domain.MarginActHistory, and
// copying it into a port-local mirror would actively break the thing it is for.
//
// The type's whole content is an invariant enforced by its constructors: its
// three states are unexported, its zero value is unusable, and "this venue
// publishes no act history" cannot be spelled the same way as "the venue
// answered and recorded nothing". A port-local struct with an exported
// `Rows []MarginAct` would hand every adapter back the nil-versus-empty
// collapse — and that collapse inverts a verdict, because the identical
// posted-margin arithmetic resolves Applied under the first reading and
// Ambiguous under the second.
//
// So the value crosses the seam whole. `[[signature-closes-what-a-check-cannot]]`
// is the shape: an adapter cannot express the wrong pairing, so no validation
// downstream has to catch it.
//
// # Not every venue implements it, and that is the ANSWER rather than a gap
//
// Bybit has no usable surface here and this is a ruling, not an unbuilt adapter.
// Profiles §4: `GET /v5/account/transaction-log` records carry a unique id, but
// the ~100-value `type` enum has no value for a manual position-margin add or
// reduce, and whether a top-up emits a record at all is undocumented and MUST
// NOT be relied on. A Bybit adapter therefore answers NoMarginActHistory — a
// positive statement that the delta is the entire contract there, which is
// exactly what §7.1 says for that venue.
//
// The port is consequently NOT embedded into a venue-capability bundle the way
// engine/venue's MarginStatusCodec is. A venue whose margin status cannot be
// read is a venue this system may not hold a short at; a venue that records no
// margin acts is merely one whose ops resolve from one source.
type VenueMarginActs interface {
	// MarginActs reads the venue's margin-change history for one instrument,
	// covering the window [from, to).
	//
	// # The window is the CALLER's, and it is the resolution's own
	//
	// The rule that decides which rows are a given op's — §7.1's "symbol + type
	// + amount + time-window" — lives in domain.MarginActHistory, not here, so
	// this window is a fetch bound rather than the match. An implementation MUST
	// return every row the venue reports in it and MUST NOT filter on amount or
	// direction: a row excluded by an adapter is excluded by a rule with no name
	// and no test, and the duplicate-row case §7.1 calls Ambiguous is precisely
	// a case where two rows must both arrive to be counted.
	//
	// Both instants are VENUE time, the clock domain.MarginOp.Watermark is on.
	// Nothing in this signature can check that, so it is stated: a window opened
	// on the local clock and closed against venue-stamped rows is wrong by the
	// calibration offset, which is a small stable number that reads as correct.
	//
	// # An error is never re-derivable into an empty history
	//
	// VenueMarginStatus's rule, and it bites harder here. "The venue answered
	// and recorded nothing" is EVIDENCE — it is what turns a moved margin into
	// an Ambiguous verdict — so a failed read reported as an empty published
	// history would manufacture that evidence out of a timeout.
	MarginActs(instrument ids.VenueInstrument, from, to time.Time) (domain.MarginActHistory, error)
}
