package transport

import "errors"

// The transport's refusals. Sentinels for branching, wrapped at the site with
// the values that failed — the same rule capture and the contexts follow.
//
// # The split that carries weight here is CONFIGURATION versus RUNTIME
//
// Everything above ErrDial is a refusal to start: a Source that could never work
// against any venue, caught before a socket is opened. Everything from ErrDial
// down is a live disruption, which is normal operation for a socket and is
// handled by reconnecting rather than by stopping.
//
// The distinction is what keeps a typo out of the reconnect loop. A misspelled
// host is a permanent failure that retries forever at the backoff ceiling while
// every light stays green and the archive stays empty — the §32k trap arriving
// through the config file instead of through a decommissioned path.
var (
	// Configuration — refuse before dialling.
	ErrSchemeUnset     = errors.New("transport: endpoint scheme unset")
	ErrSchemeUnknown   = errors.New("transport: endpoint scheme must be ws or wss")
	ErrHostUnset       = errors.New("transport: endpoint host unset")
	ErrHostMalformed   = errors.New("transport: endpoint host must be host[:port] with no scheme or path")
	ErrPrefixMalformed = errors.New("transport: endpoint prefix must start with / and not end with /")
	ErrTopicUnset      = errors.New("transport: topic unset")
	ErrTopicCharset    = errors.New("transport: topic contains a character that is not safe in a URL path segment")
	ErrTopicTraversal  = errors.New("transport: topic is a path-traversal segment")
	ErrProtocolUnknown = errors.New("transport: unknown protocol")
	ErrSinkUnset       = errors.New("transport: sink unset")
	ErrTimeoutRange    = errors.New("transport: timeout must be positive")
	ErrBackoffUnset    = errors.New("transport: backoff schedule is empty")

	// The poller's own configuration refusals. ErrRestScheme is not
	// ErrSchemeUnknown because that sentinel's words are a claim about
	// SOCKETS ("must be ws or wss"), and an error message that names the
	// wrong legal set sends the operator to fix the wrong field.
	ErrRestScheme     = errors.New("transport: poller scheme must be http or https")
	ErrPathMalformed  = errors.New("transport: path must start with / and not end with /")
	ErrReadLimitRange = errors.New("transport: read limit must be positive")

	// The signed client's configuration refusals. Every one of these is a
	// request that could never be accepted by any venue, caught before a byte
	// goes out — the same configuration/runtime line the sentinels above draw,
	// held at the point where a bad request would otherwise become an
	// authentication error and read as a credential fault.
	ErrSignMethodUnset        = errors.New("transport: signed request method unset")
	ErrSignMethodUnsupported  = errors.New("transport: signed request method must be GET or POST")
	ErrRecvWindowUnset        = errors.New("transport: recv window unset — no venue default is adopted, see SignedRequest.RecvWindow")
	ErrRecvWindowRange        = errors.New("transport: recv window exceeds the venue ceiling")
	ErrSignerUnset            = errors.New("transport: signer unset")
	ErrLimitReaderUnset       = errors.New("transport: limit reader unset")
	ErrLimitFoldUnaddressable = errors.New("transport: profile limit can never receive the venue's own reading — its name does not agree with the dialect's fold-derived names")
	ErrEnvelopeReaderUnset    = errors.New("transport: envelope reader unset — a venue that states refusals in the body would read as accepting everything")
	ErrGovernorUnset          = errors.New("transport: governor unset")
	ErrClockUnset             = errors.New("transport: clock unset")

	// The server-clock refusals. Separate sentinels because they send the
	// operator to different places: Unusable is a venue that was never
	// calibrated, Uncertain is one whose calibration is too loose to sign
	// inside the venue's acceptance window (Profiles §6).
	ErrServerTimeUnusable  = errors.New("transport: venue server time is unusable — the venue is uncalibrated")
	ErrServerTimeUncertain = errors.New("transport: venue server-clock uncertainty exceeds the acceptance window")

	// Runtime — reconnect, do not stop.
	ErrDial        = errors.New("transport: dial failed")
	ErrSubscribe   = errors.New("transport: subscribe failed")
	ErrRead        = errors.New("transport: read failed")
	ErrNoFirstData = errors.New("transport: no data within the first-data timeout")

	// ErrBodyTooLarge is the poller refusing a response body its caller's
	// archive could not hold. Runtime and per-ask: the next scheduled poll
	// is unaffected, exactly as a socket disruption leaves the next
	// reconnect unaffected.
	ErrBodyTooLarge = errors.New("transport: response body exceeds the read limit")
)
