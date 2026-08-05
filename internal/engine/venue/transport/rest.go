package transport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// This file is §32h's REST poller: one GET per Fetch, status and body back
// verbatim, error only when the transport itself failed. It lives in this
// package and not in the composition root because venue wire code is this
// package's declared competence (§32b) — milestone 3's fee-tier reads will
// want the same poller, with credentials this one deliberately cannot carry.
//
// A non-2xx is a RESULT, not an error. The caller archives what the venue
// answered, and a 429 with a venue-worded body is §32h's fact exactly as much
// as a 200 with rows in it. The error return means the ask itself failed —
// no status ever existed — which the caller records as its own kind of fact.
//
// There is no retry and no backoff here, stated as a decision (§37b): the
// next scheduled poll is the retry. The overlay's value is dated provenance
// for a slow-moving venue config, and a retry loop is the first step toward
// the reconnect machinery the socket needed and this side does not.

// PollerConfig is one REST target. Every field is required — the poller has
// no library defaults standing behind it the way the socket's ReadLimit does,
// so an unset field here is an unbounded read or a request that never times
// out, silently.
type PollerConfig struct {
	// Scheme is http or https. Plain http exists for the same stated reason
	// plain ws does on Endpoint: the tests speak real HTTP to a real
	// httptest server, because that is the only way the request and the
	// body-limit refusal are actually exercised.
	Scheme string

	// Host is host or host:port, with no scheme and no path — Endpoint's
	// rule, enforced with Endpoint's own paste-error check.
	Host string

	// Path is the complete request path, leading slash, no trailing slash.
	// There is no topic appended and no query string: §32h's one endpoint
	// takes neither, and a field nothing uses is a field nothing validates.
	Path string

	// Timeout bounds the whole request — dial, write, response headers and
	// body. One field rather than the socket's three because a poll is one
	// round trip, and the caller's own bound (§37e: at most alive_interval)
	// is about the total time the writer goes undriven.
	Timeout time.Duration

	// ReadLimit caps the response body, in bytes. Required: a body the
	// archive could not hold is refused here, at the transport, before it is
	// counted as captured — the socket's ReadLimit argument, and this side
	// has no library default to fall back on.
	ReadLimit int64
}

// Validate reports whether the poller could ask.
func (c PollerConfig) Validate() error {
	switch c.Scheme {
	case "":
		return ErrSchemeUnset
	case "http", "https":
	default:
		return fmt.Errorf("scheme %q: %w", c.Scheme, ErrRestScheme)
	}
	if c.Host == "" {
		return ErrHostUnset
	}
	if strings.ContainsAny(c.Host, "/?# ") || strings.Contains(c.Host, "://") {
		return fmt.Errorf("host %q: %w", c.Host, ErrHostMalformed)
	}
	if !strings.HasPrefix(c.Path, "/") || strings.HasSuffix(c.Path, "/") ||
		strings.ContainsAny(c.Path, "?# ") {
		return fmt.Errorf("path %q: %w", c.Path, ErrPathMalformed)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout %s: %w", c.Timeout, ErrTimeoutRange)
	}
	if c.ReadLimit <= 0 {
		return fmt.Errorf("read limit %d: %w", c.ReadLimit, ErrReadLimitRange)
	}
	return nil
}

// URL renders the request URL — the same string the caller records in its
// meta and its Open provenance, built in exactly one place.
func (c PollerConfig) URL() string { return c.Scheme + "://" + c.Host + c.Path }

// PollResult is what the venue answered: the status and the body, verbatim.
type PollResult struct {
	Status int
	Body   []byte
}

// Poller asks one REST endpoint. Safe for a single goroutine, which is all
// §37f gives it.
type Poller struct {
	cfg    PollerConfig
	client *http.Client
}

// NewPoller validates the target and builds the client. The timeout lives on
// the client rather than on a per-call context so that no member of the
// ambient-time family appears at a call site this package owns — the
// deadline machinery is net/http's, which rule 5 does not police.
func NewPoller(cfg PollerConfig) (*Poller, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Poller{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

// URL is the request URL this poller asks — what the caller records in its
// meta and its Open provenance, so the two spellings cannot drift.
func (p *Poller) URL() string { return p.cfg.URL() }

// Fetch performs one GET. The returned body is capped by ReadLimit: a body
// that would exceed it is refused as a transport failure (ErrBodyTooLarge)
// rather than truncated, because a truncated body archived with a 200 status
// is a lie two records deep — the status says the venue answered, the bytes
// are not what it answered.
func (p *Poller) Fetch(ctx context.Context) (PollResult, error) {
	return doGet(ctx, p.client, p.cfg.URL(), p.cfg.ReadLimit)
}

// doGet is the one GET both the Poller and the PublicClient perform — shared
// as code rather than restated, so the two REST reads cannot come to disagree
// about what a refusal is.
func doGet(ctx context.Context, client *http.Client, url string, readLimit int64) (PollResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PollResult{}, fmt.Errorf("transport: build request %s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return PollResult{}, fmt.Errorf("transport: %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Read one byte past the limit so "exactly at the limit" and "over it"
	// are distinguishable — LimitReader alone returns the same bytes for
	// both, and the boundary is where a wrong comparison hides.
	body, err := io.ReadAll(io.LimitReader(resp.Body, readLimit+1))
	if err != nil {
		return PollResult{}, fmt.Errorf("transport: read %s: %w", url, err)
	}
	if int64(len(body)) > readLimit {
		return PollResult{}, fmt.Errorf("transport: %s body exceeds %d bytes: %w",
			url, readLimit, ErrBodyTooLarge)
	}
	return PollResult{Status: resp.StatusCode, Body: body}, nil
}
