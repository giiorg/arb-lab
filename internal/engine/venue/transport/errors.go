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
