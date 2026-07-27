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

	// ErrPayloadType reports an envelope whose payload is not a venue.Funding.
	// §3.5 types the lanes generically over payload and has the contexts assert
	// what they subscribed to, so this is where that assertion's failure gets a
	// name instead of a panic in a dispatcher's hot path.
	ErrPayloadType = errors.New("marketdata: envelope payload is not a funding update")

	// ErrReconstructNil, ErrNotBookStream and ErrBookPayloadType are the book
	// path's three refusals, and each is SEPARATE from the funding sentinel that
	// mirrors it rather than shared with it.
	//
	// The rule is application/errors.go's, where ErrBookPublisherNil records it:
	// the message names the wire, and one message cannot name two. The three
	// above are all phrased "funding" — a process wiring both paths and reading
	// "envelope is not a funding stream" from the book dispatch would be sent to
	// the wrong half of the module. The cost is that a caller wanting "any
	// misrouted envelope" tests two values; the alternative cost is a caller
	// that cannot tell which path refused, and only one of those two is
	// recoverable from a log.
	ErrReconstructNil = errors.New("marketdata: book use case not supplied")

	// ErrNotBookStream reports an envelope from a stream this adapter does not
	// handle. The boundary is the STREAM and not the lane, because mark and
	// funding share the market-data lane with book: a funding payload delivered
	// here would otherwise fall through to the payload check and be reported as
	// a type problem, which sends a reader to the decoder instead of to the
	// subscription that routed it wrongly.
	ErrNotBookStream = errors.New("marketdata: envelope is not a book stream")

	// ErrBookPayloadType reports a book envelope whose payload is neither a
	// venue.BookSnapshot nor a venue.BookDelta.
	//
	// The two together are the type switch's whole domain, and the sentinel is
	// one rather than two on purpose: "which of the two it is not" is not a
	// question — the failure is that it is neither, and the %T in the wrapped
	// message is what a reader actually needs.
	ErrBookPayloadType = errors.New("marketdata: envelope payload is not a book snapshot or delta")
)
