package transport

import (
	"fmt"
	"strings"
)

// Endpoint is a venue websocket base, split into the three parts that vary
// INDEPENDENTLY. That split is a finding rather than a style choice, and it cost
// a verification round to learn (Profiles §5, record §35b).
//
// The capture set's seven sources were written down as topic strings, and a
// topic string cannot express them:
//
//   - Binance runs THREE hosts with THREE path disciplines. Perp funding is on
//     fstream.binance.com under /market; perp book is the same host under
//     /public; spot book is a DIFFERENT host, stream.binance.com:9443, which
//     kept its own base and takes no such prefix at all. The legacy prefix-less
//     fstream paths were decommissioned 2026-04-23 and — this is the trap §32k
//     exists for — return NOTHING rather than an error.
//   - Bybit sources 5 and 6 have BYTE-IDENTICAL topic strings
//     (orderbook.50.BTCUSDT on linear and on spot) distinguished only by URL.
//     Keyed on topic, those two sources are one source.
//
// So: Host is where to connect, Prefix is the path in front of the topic, and
// the topic is the caller's, held separately.
type Endpoint struct {
	// Scheme is ws or wss. Plain ws exists because the tests speak the real
	// protocol to a real httptest server rather than to a mock, which is the
	// only way the handshake and the ack are actually exercised.
	Scheme string

	// Host is host or host:port, with no scheme and no path.
	// "stream.binance.com:9443" is a host; the port is not incidental.
	Host string

	// Prefix is the path in front of the topic, leading slash, no trailing
	// slash. Its MEANING depends on the discipline, and that is the honest
	// reading rather than an overload:
	//
	//   path discipline    the path the topic is appended to — "/market/ws",
	//                      "/public/ws", or "/ws" on the spot host
	//   message discipline the COMPLETE path, because the topic never appears
	//                      in the URL — "/v5/public/linear"
	//
	// Empty is legal and means the root.
	Prefix string
}

// Validate reports whether the endpoint could be dialled.
func (e Endpoint) Validate() error {
	switch e.Scheme {
	case "":
		return ErrSchemeUnset
	case "ws", "wss":
	default:
		return fmt.Errorf("scheme %q: %w", e.Scheme, ErrSchemeUnknown)
	}
	if e.Host == "" {
		return ErrHostUnset
	}
	// A host that carries its own scheme or path is the realistic paste error —
	// copying "wss://fstream.binance.com/market/ws" into the host field — and it
	// produces a URL that is malformed in a way net/url may accept and the venue
	// will not.
	if strings.ContainsAny(e.Host, "/?# ") || strings.Contains(e.Host, "://") {
		return fmt.Errorf("host %q: %w", e.Host, ErrHostMalformed)
	}
	if e.Prefix != "" {
		if !strings.HasPrefix(e.Prefix, "/") {
			return fmt.Errorf("prefix %q: %w", e.Prefix, ErrPrefixMalformed)
		}
		// A trailing slash would produce "//" once the topic is appended, which
		// some proxies normalise and some do not. Refusing it here means the
		// URL we build is the URL we recorded in the Open record.
		if strings.HasSuffix(e.Prefix, "/") {
			return fmt.Errorf("prefix %q: %w", e.Prefix, ErrPrefixMalformed)
		}
		if strings.ContainsAny(e.Prefix, "?# ") {
			return fmt.Errorf("prefix %q: %w", e.Prefix, ErrPrefixMalformed)
		}
	}
	return nil
}

// String renders the endpoint as the base URL, without any topic.
func (e Endpoint) String() string { return e.Scheme + "://" + e.Host + e.Prefix }

// validateTopic checks a topic is safe to place in a URL path segment.
//
// # This is §34d's finding applied to its mirror
//
// Slice B found that a topic of ".." climbed out of the ARCHIVE tree, because a
// topic is a path segment there. A topic is ALSO a path segment here, under the
// path discipline, and the same string does the same thing to a URL: a topic of
// "../private/ws" against prefix "/market/ws" resolves to a path we did not mean
// to dial, and one net/http will happily send. The repo's rule is that a fix
// applied in one place gets applied to its mirror immediately, so this guard
// went in the moment the second path-segment site appeared rather than after the
// second finding.
//
// It is applied under BOTH disciplines even though only one builds a path. A
// topic reaching the message discipline goes into JSON, where traversal means
// nothing — but the same config file feeds both, the same topic string is what
// capture turns into an archive directory, and a rule that holds for one
// discipline and not the other is a rule nobody can state from the config alone.
//
// The charset is a positive list rather than a denial of the dangerous
// characters, for the reason every default in this repo is written that way:
// a denial list stops covering whatever a venue introduces next, silently. The
// set is what the seven real topics use — letters, digits and . _ - @ ! : —
// widened when a venue needs it and never by guessing.
func validateTopic(topic string) error {
	if topic == "" {
		return ErrTopicUnset
	}
	if topic == "." || topic == ".." {
		return fmt.Errorf("topic %q: %w", topic, ErrTopicTraversal)
	}
	for i := 0; i < len(topic); i++ {
		if !topicChar(topic[i]) {
			return fmt.Errorf("topic %q at byte %d (%q): %w",
				topic, i, string(topic[i]), ErrTopicCharset)
		}
	}
	return nil
}

// topicChar is the positive list. A "/" is absent from it, which is what makes
// the traversal check above complete rather than a spot fix: "." and ".." are
// only dangerous as WHOLE segments, and without a "/" a topic cannot introduce
// a new segment to be one.
func topicChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '.', '_', '-', '@', '!', ':':
		return true
	}
	return false
}
