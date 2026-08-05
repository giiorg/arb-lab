package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// This file is the Poller's sibling for public endpoints that take query
// parameters — the consumer rest.go anticipated ("milestone 3's fee-tier reads
// will want the same poller"), arrived as §69's lot-filter fetch. Neither
// existing type fits it: PollerConfig refuses a query string by validated
// design (§32h's one endpoint takes none, and that refusal must stay
// unconditional), and SignedClient always signs, charges a budget, and
// classifies failures by side effect — three concerns a public reference read
// has none of.
//
// Everything else is the Poller's contract verbatim, shared as code rather
// than restated: one GET per Fetch, a non-2xx is a RESULT, a body over
// ReadLimit is a refusal, no retry (§37b — the caller's next invocation is the
// retry), and the timeout lives on the client so no member of the ambient-time
// family appears at a call site this package owns.

// PublicConfig is one public GET target with query parameters. Every field
// except Params is required, on PollerConfig's argument; Params may be empty
// because one of §69's four calls (fapi exchangeInfo) takes none — the venue
// ignores ?symbol= there, so sending it would record a URL that claims a
// scoping the venue never applied.
type PublicConfig struct {
	// Scheme, Host and Path follow PollerConfig's rules exactly, and are
	// validated by it — one authority for the shared surface.
	Scheme string
	Host   string
	Path   string

	// Params is the ordered query-parameter list, rendered by Params.Encode —
	// the signing path's percent-encoding, which is the one spelling in this
	// package with its own oracle. Order is preserved into the URL, and the
	// URL is the provenance string the caller records, so a deterministic
	// rendering is load-bearing here even without a signature over it.
	Params Params

	// Timeout and ReadLimit are PollerConfig's, for PollerConfig's reasons.
	Timeout   time.Duration
	ReadLimit int64
}

// Validate reports whether the client could ask. The five shared fields are
// judged by PollerConfig.Validate — not restated — and the params by this
// type's own rule: a present parameter must have both halves, because an empty
// key or value here is a forgotten argument that the venue would answer with
// its full-roster or error behaviour rather than refuse.
func (c PublicConfig) Validate() error {
	base := PollerConfig{
		Scheme:    c.Scheme,
		Host:      c.Host,
		Path:      c.Path,
		Timeout:   c.Timeout,
		ReadLimit: c.ReadLimit,
	}
	if err := base.Validate(); err != nil {
		return err
	}
	for i, p := range c.Params {
		if p.Key == "" {
			return fmt.Errorf("param %d: %w", i, ErrParamKeyUnset)
		}
		if p.Value == "" {
			return fmt.Errorf("param %d %q: %w", i, p.Key, ErrParamValueUnset)
		}
	}
	return nil
}

// URL renders the request URL — the same string the caller records as the
// fetched row's source, built in exactly one place.
func (c PublicConfig) URL() string {
	u := c.Scheme + "://" + c.Host + c.Path
	if len(c.Params) == 0 {
		return u
	}
	return u + "?" + c.Params.Encode()
}

// PublicClient asks one public REST endpoint. Safe for a single goroutine,
// like the Poller it mirrors.
type PublicClient struct {
	cfg    PublicConfig
	client *http.Client
}

// NewPublicClient validates the target and builds the client. The timeout
// lives on the client for NewPoller's stated reason.
//
// Redirects are NOT followed, and this is the one departure from the Poller:
// the Poller archives whatever the venue answered and a 3xx is an answer, but
// this client's URL becomes a `source` claim on rows that gate orders, and
// following a redirect would attribute a different host's body to the
// configured URL. ErrUseLastResponse surfaces the redirect as its own 3xx
// RESULT, which a provenance-writing caller refuses as it refuses any
// non-200, naming the status.
func NewPublicClient(cfg PublicConfig) (*PublicClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &PublicClient{cfg: cfg, client: &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

// URL is the request URL this client asks — the caller's provenance string,
// so the two spellings cannot drift.
func (c *PublicClient) URL() string { return c.cfg.URL() }

// Fetch performs one GET, on the Poller's exact terms — see doGet.
func (c *PublicClient) Fetch(ctx context.Context) (PollResult, error) {
	return doGet(ctx, c.client, c.cfg.URL(), c.cfg.ReadLimit)
}
