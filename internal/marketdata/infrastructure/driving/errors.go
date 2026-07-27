package driving

import "errors"

// The driving adapter's refusals. Sentinels for branching, wrapped at the site
// with the values that failed — the same rule domain/ and application/ follow.
var (
	// ErrIngestNil reports an adapter constructed with no use case behind it.
	// Refused at construction, not at first envelope: a dispatch surface that
	// accepted traffic and dropped it would present as a silent feed, which is
	// the exact failure §3.4's silence timer exists to catch, arriving from
	// inside the process instead of from the venue.
	ErrIngestNil = errors.New("marketdata: funding use case not supplied")

	// ErrWrongStream reports an envelope from a stream this adapter does not
	// handle. Refused rather than ignored, and it is the misrouting guard
	// events.ErrWrongLane makes one layer down — with the sharper boundary,
	// because mark and book share the market-data LANE with funding. A book
	// update read as a funding message would publish a zero rate as a venue
	// reading, and a zero funding rate is indistinguishable from a flat regime.
	ErrWrongStream = errors.New("marketdata: envelope is not a funding stream")

	// ErrPayloadType reports an envelope whose payload is not a FundingUpdate.
	// §3.5 types the lanes generically over payload and has the contexts assert
	// what they subscribed to, so this is where that assertion's failure gets a
	// name instead of a panic in a dispatcher's hot path.
	ErrPayloadType = errors.New("marketdata: envelope payload is not a funding update")
)
