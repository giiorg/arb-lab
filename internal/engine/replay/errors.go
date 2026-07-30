package replay

import "errors"

// Sentinels for branching with errors.Is; wrapped at the site with the values
// that failed.
var (
	// ErrDecoderUnset and ErrDecoderUnknown report a StreamSpec whose decoder
	// token is empty or outside the declared set — §3.6 rule 8's shape.
	ErrDecoderUnset   = errors.New("replay: decoder kind unset")
	ErrDecoderUnknown = errors.New("replay: unknown decoder kind")

	// ErrNoStreams reports a Load over an empty roster. Refused because a tape
	// of nothing generates an empty log that PASSES every parity comparison —
	// the empty-tree door, §39n-3's argument one layer up.
	ErrNoStreams = errors.New("replay: no streams to load")

	// ErrNoFiles reports a stream whose directory holds no archive files. The
	// same empty-tree refusal per stream: a mistyped topic would otherwise
	// replay as a venue that said nothing.
	ErrNoFiles = errors.New("replay: stream has no archive files")

	// ErrTapeEmpty reports a loaded tape holding no consumable record at all.
	ErrTapeEmpty = errors.New("replay: tape holds no data or rest records")

	// ErrDiverged reports the rerun refusal: the dispatch log demands a step
	// the archive-derived world cannot produce, or produces differently. The
	// log and the archive are two records of one run; neither may be repaired
	// from the other, so the divergence is named and the rerun stops.
	ErrDiverged = errors.New("replay: dispatch log diverges from the archive")

	// ErrRestAnswerUnreadable reports a 2xx REST answer whose body the decoder
	// refuses. A failed poll is ordinary (recorded, no news — §37g); a poll
	// the venue answered and we cannot read is not.
	ErrRestAnswerUnreadable = errors.New("replay: recorded rest answer is unreadable")
)
