package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// SignedClient is the credential-bearing REST client §47e found missing: the
// account/order half of engine/venue that §12 claimed milestone 1 delivered.
//
// It is the poller's sibling and deliberately not the poller. Poller carries no
// credential by design ("with credentials this one deliberately cannot carry",
// rest.go), reads a public endpoint, and is driven by a schedule. This one signs,
// charges a rate-limit budget, and classifies its failures by SIDE EFFECT — three
// concerns the capture poller has none of, and each of which would be a field
// nobody validates if the two were merged.
//
// # It never retries, and that is the type's most important property
//
// §5.3: Place is called once per order, "transport retries forbidden". A retry
// inside this client would break the rule invisibly — the caller cannot see it
// happen, and the second attempt would carry the same ClientOrderID, which is a
// recovery handle and NOT a venue idempotency key. So there is no retry, no
// backoff and no redirect-following here, and the next attempt is a decision
// made upstream with the journal in hand.
type SignedClient struct {
	cfg    SignedClientConfig
	client *http.Client
}

// SignedClientConfig is one venue's authenticated REST target.
type SignedClientConfig struct {
	// Scheme is http or https. Plain http exists for the same stated reason it
	// does on PollerConfig: the tests speak real HTTP to a real httptest
	// server, which is the only way the signature actually reaches a wire.
	Scheme string

	// Host is host or host:port, no scheme and no path.
	Host string

	// Timeout bounds the whole request.
	//
	// Note what a timeout MEANS here, because it is not what it means on the
	// poller: a poll that times out is retried by the next tick, while a signed
	// order that times out is an EffectUnknown that costs a resolving query and
	// blocks new trading until it resolves (§5.3). The bound is therefore a
	// safety parameter, not a responsiveness one.
	Timeout time.Duration

	// ReadLimit caps the response body in bytes.
	ReadLimit int64

	// RecvWindow is how long the venue keeps honouring our requests. See
	// SignedRequest.RecvWindow — unset is refused, and small is the safe
	// direction.
	RecvWindow time.Duration

	// Signer composes the venue's authentication.
	Signer Signer

	// Limits reads the venue's rate-limit headers.
	Limits LimitReader

	// Envelope reads the venue's own verdict out of a response body.
	//
	// Required, and not defaultable to a reader that finds nothing: on Bybit
	// the body is where a refusal is stated at all (HTTP 200 + retCode), so a
	// client without one classifies every Bybit rejection as an acceptance.
	// The venue that needs this least — Binance, whose status is authoritative
	// — is the one whose reader looks like it could be omitted, which is why
	// there is a BinanceEnvelope rather than a nil.
	Envelope EnvelopeReader

	// Governor is the venue's §7.6 budget. Required: a client that could run
	// without one would be a client that spends an unmetered budget, which
	// looks exactly like a system comfortably inside its limits until the ban.
	Governor *ratelimit.Governor

	// Clock is the injected time seam. ServerNow is what gets signed.
	Clock clock.Clock
}

// Validate reports whether the client could ask.
func (c SignedClientConfig) Validate() error {
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
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout %s: %w", c.Timeout, ErrTimeoutRange)
	}
	if c.ReadLimit <= 0 {
		return fmt.Errorf("read limit %d: %w", c.ReadLimit, ErrReadLimitRange)
	}
	if c.RecvWindow <= 0 {
		return ErrRecvWindowUnset
	}
	if c.RecvWindow > maxRecvWindow {
		return fmt.Errorf("recv window %s exceeds %s: %w", c.RecvWindow, maxRecvWindow, ErrRecvWindowRange)
	}
	if c.Signer == nil {
		return ErrSignerUnset
	}
	if c.Limits == nil {
		return ErrLimitReaderUnset
	}
	if c.Envelope == nil {
		return ErrEnvelopeReaderUnset
	}
	if c.Governor == nil {
		return ErrGovernorUnset
	}
	if c.Clock == nil {
		return ErrClockUnset
	}
	return nil
}

// NewSignedClient validates the target and builds the HTTP client.
func NewSignedClient(cfg SignedClientConfig) (*SignedClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	snapshot := cfg.Governor.Snapshot()
	for _, l := range snapshot.Limits {
		if l.Scope != ratelimit.ScopeFamily {
			continue
		}
		if err := cfg.Limits.CheckFold(l.Name, l.Dimension, l.Interval); err != nil {
			return nil, fmt.Errorf("venue %q limit %q: %w", snapshot.Venue, l.Name, err)
		}
	}
	return &SignedClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
			// A redirect is a second request, and this client sends exactly
			// one. Following one would also re-send the signed query to a host
			// the signature never authorised.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Call is one authenticated venue call.
type Call struct {
	// Op names the operation for the error record, e.g. "binance.PlaceOrder".
	Op string

	// Method is GET or POST.
	Method string

	// Path is the complete request path.
	Path string

	// Params are the request parameters, in signing order.
	Params Params

	// Body is the raw body bytes, verbatim.
	Body []byte

	// Family is the API family this call charges (§7.6's per-(venue, family)
	// budgets — Binance spot and futures are separate weight/order buckets).
	Family ids.ProductCategory

	// Priority is §7.6's shed class: Opening, Routine or Safety.
	Priority ratelimit.Priority

	// Cost is what the call charges.
	Cost ratelimit.Cost

	// Limit names the profile limit this endpoint's rate-limit headers refer
	// to, for venues whose headers do not name it themselves.
	//
	// Binance's headers name their own window (X-MBX-USED-WEIGHT-1M carries
	// "1m" in the key) and ignore this. Bybit's are anonymous — X-Bapi-Limit
	// describes "current endpoint" without saying which — so on Bybit the name
	// can only come from the caller that knew which endpoint it was calling.
	Limit string
}

// Response is what the venue answered.
type Response struct {
	Status int
	Body   []byte
	Header http.Header
}

// Do performs exactly one signed request.
//
// The order of the steps is the contract, and each boundary is a place the
// effect classification changes:
//
//  1. Validate — our own refusal, nothing sent, EffectNotApplied.
//  2. Acquire from the governor — a local decline, nothing sent, EffectNotApplied.
//  3. Read the server clock and sign — a refusal here is still pre-send.
//  4. Send, ONCE. From this point "not sent" can no longer be inferred.
//  5. Fold the venue's headers into the governor, whatever the status.
//  6. Classify the RESPONSE — status and body together — and penalize the
//     governor if it earned one.
//
// Step 5 runs before step 6 deliberately: a 429 carries the headers that say how
// far over we went, and returning early on the status would discard exactly the
// reading taken when it mattered most.
//
// Step 6 reads the body as well as the status, and that is not a refinement:
// Bybit states a refusal in the body BENEATH a success status, so a step 6 that
// read the status alone would return (Response, nil) for a rejected order. See
// envelope.go.
func (c *SignedClient) Do(ctx context.Context, call Call) (Response, error) {
	req := ratelimit.Request{Family: call.Family, Priority: call.Priority, Cost: call.Cost}
	if err := req.Validate(); err != nil {
		return Response{}, apperror.Precondition(call.Op, "rate-limit request", err)
	}

	decision, err := c.cfg.Governor.Acquire(req)
	if err != nil {
		return Response{}, apperror.Precondition(call.Op, "governor", err)
	}
	if derr := decision.Err(); derr != nil {
		// Refused locally: nothing left the process. RateLimited fixes Effect at
		// EffectNotApplied for exactly this case.
		return Response{}, apperror.RateLimited(call.Op, decision.Reason.String(), decision.RetryAfter(), derr)
	}

	wire, err := c.cfg.Signer.Sign(SignedRequest{
		Method:     call.Method,
		Path:       call.Path,
		Params:     call.Params,
		Body:       call.Body,
		RecvWindow: c.cfg.RecvWindow,
	}, c.cfg.Clock.ServerNow(c.cfg.Signer.Venue()))
	if err != nil {
		return Response{}, apperror.Precondition(call.Op, "sign", err)
	}

	url := c.cfg.Scheme + "://" + c.cfg.Host + call.Path
	if wire.Query != "" {
		url += "?" + wire.Query
	}
	httpReq, err := http.NewRequestWithContext(ctx, call.Method, url, bytes.NewReader(wire.Body))
	if err != nil {
		return Response{}, apperror.Precondition(call.Op, "build request", err)
	}
	for k, v := range wire.Header {
		httpReq.Header.Set(k, v)
	}
	if len(wire.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return Response{}, ClassifyTransportError(call.Op, err)
	}
	defer resp.Body.Close()

	// Read one past the limit so "exactly at" and "over" are distinguishable —
	// the poller's argument, and the boundary is where a wrong comparison hides.
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.cfg.ReadLimit+1))
	if err != nil {
		// The venue answered and we failed to read it. The request definitely
		// went out, so this is not NotApplied; whether it applied is unknown.
		return Response{}, apperror.Transport(call.Op, apperror.EffectUnknown, err)
	}
	if int64(len(body)) > c.cfg.ReadLimit {
		return Response{}, apperror.Transport(call.Op, apperror.EffectUnknown,
			fmt.Errorf("%s body exceeds %d bytes: %w", url, c.cfg.ReadLimit, ErrBodyTooLarge))
	}

	c.observe(call, resp.Header)

	out := Response{Status: resp.StatusCode, Body: body, Header: resp.Header}
	aerr, penalty := ClassifyResponse(call.Op, c.cfg.Envelope, out, retryAfter(resp.Header))
	if penalty != ratelimit.PenaltyUnspecified {
		// A Penalize failure must not mask the venue's own refusal, which is the
		// fact the caller has to act on. It is folded into nothing and the
		// classified error is returned regardless.
		_ = c.cfg.Governor.Penalize(ratelimit.Penalty{Kind: penalty, RetryAfter: retryAfter(resp.Header)})
	}
	if aerr != nil {
		return out, aerr
	}
	return out, nil
}

// observe folds the venue's rate-limit headers into the governor.
//
// Errors are dropped on purpose, and this is the one place in the file where
// that is right: Observe refuses a limit the profile does not declare, which is
// a configuration mismatch rather than a reason to fail a request the venue
// already answered. The alternative — failing the call — would turn a
// mislabelled header into a trading outage. Dropping here is safe because
// NewSignedClient already refused any profile whose family-scoped names the
// reader could never address (CheckFold), so what remains at this seam is the
// venue reporting a window the profile deliberately does not budget.
func (c *SignedClient) observe(call Call, h http.Header) {
	for _, o := range c.cfg.Limits.Observations(h, call) {
		_ = c.cfg.Governor.Observe(o.Family, o.Name, o.Used)
	}
}

// retryAfter reads the venue's own hint. §7.6 treats it as a FLOOR, not a
// ceiling, and platform/ratelimit's Penalize is where that is enforced.
func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// Observation is one venue-reported budget reading, already converted to
// CONSUMPTION.
//
// The conversion happens in the LimitReader rather than here because the two
// venues report opposite quantities and only the venue-specific reader knows
// which it is holding. See BybitLimitReader.
type Observation struct {
	Family ids.ProductCategory
	Name   string
	Used   uint64
}

// LimitReader turns a venue's rate-limit response headers into observations.
type LimitReader interface {
	Observations(h http.Header, call Call) []Observation

	// CheckFold reports whether a family-scoped budget declared under name,
	// counting d over window w, can receive this reader's venue readings.
	CheckFold(name string, d ratelimit.Dimension, w time.Duration) error
}

// BinanceLimitReader reads X-MBX-USED-WEIGHT-* and X-MBX-ORDER-COUNT-*.
//
// These headers report CONSUMPTION directly, and they name their own window in
// the header key ("X-MBX-USED-WEIGHT-1M" is the one-minute weight), so the limit
// name is derived from the header rather than supplied by the caller.
type BinanceLimitReader struct{}

// CheckFold implements LimitReader.
func (BinanceLimitReader) CheckFold(name string, d ratelimit.Dimension, w time.Duration) error {
	switch d {
	case ratelimit.DimensionUnspecified:
		return fmt.Errorf("limit %q has unspecified dimension: %w", name, ErrLimitFoldUnaddressable)
	case ratelimit.DimensionWeight:
		return checkBinanceFoldName(name, "weight_", w)
	case ratelimit.DimensionOrders:
		return checkBinanceFoldName(name, "orders_", w)
	case ratelimit.DimensionRequests:
		if derived, ok := binanceDerivedName(name); ok && derived == name {
			return fmt.Errorf("declared limit %q collides with derived name %q: %w",
				name, derived, ErrLimitFoldUnaddressable)
		}
		return nil
	}
	return fmt.Errorf("limit %q has dimension %d: %w", name, uint8(d), ErrLimitFoldUnaddressable)
}

func checkBinanceFoldName(name, prefix string, w time.Duration) error {
	window, ok := binanceWindow(w)
	if !ok {
		return fmt.Errorf("limit %q window %s cannot be rendered as a binance fold name: %w",
			name, w, ErrLimitFoldUnaddressable)
	}
	want := prefix + window
	if name != want {
		return fmt.Errorf("declared limit %q does not agree with derived name %q: %w",
			name, want, ErrLimitFoldUnaddressable)
	}
	return nil
}

func binanceDerivedName(name string) (string, bool) {
	for _, prefix := range [...]string{"weight_", "orders_"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		window, ok := parseBinanceWindow(strings.TrimPrefix(name, prefix))
		if !ok {
			return "", false
		}
		canonical, ok := binanceWindow(window)
		if !ok {
			return "", false
		}
		return prefix + canonical, true
	}
	return "", false
}

func binanceWindow(w time.Duration) (string, bool) {
	if w <= 0 || w%time.Second != 0 {
		return "", false
	}
	for _, candidate := range [...]struct {
		unit   time.Duration
		suffix string
	}{
		{24 * time.Hour, "d"},
		{time.Hour, "h"},
		{time.Minute, "m"},
	} {
		if w%candidate.unit == 0 {
			return strconv.FormatInt(int64(w/candidate.unit), 10) + candidate.suffix, true
		}
	}
	return strconv.FormatInt(int64(w/time.Second), 10) + "s", true
}

func parseBinanceWindow(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	var unit time.Duration
	switch s[len(s)-1] {
	case 's':
		unit = time.Second
	case 'm':
		unit = time.Minute
	case 'h':
		unit = time.Hour
	case 'd':
		unit = 24 * time.Hour
	default:
		return 0, false
	}
	n, err := strconv.ParseUint(s[:len(s)-1], 10, 64)
	if err != nil || n == 0 || n > uint64(math.MaxInt64/int64(unit)) {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

// Observations implements LimitReader.
func (BinanceLimitReader) Observations(h http.Header, call Call) []Observation {
	var out []Observation
	for key, vals := range h {
		var namePrefix, suffix string
		switch {
		case strings.HasPrefix(key, binanceWeightPrefix):
			namePrefix = "weight_"
			suffix = strings.TrimPrefix(key, binanceWeightPrefix)
		case strings.HasPrefix(key, binanceOrderPrefix):
			namePrefix = "orders_"
			suffix = strings.TrimPrefix(key, binanceOrderPrefix)
		default:
			continue
		}
		window, ok := parseBinanceWindow(strings.ToLower(suffix))
		if !ok {
			continue
		}
		canonical, ok := binanceWindow(window)
		if !ok {
			continue
		}
		if len(vals) == 0 {
			continue
		}
		used, err := strconv.ParseUint(vals[0], 10, 64)
		if err != nil {
			// An unparseable count is dropped rather than read as zero. Zero is
			// the one value that can never raise the local count, so a
			// malformed header would be indistinguishable from a venue
			// reporting no consumption at all.
			continue
		}
		out = append(out, Observation{Family: call.Family, Name: namePrefix + canonical, Used: used})
	}
	return out
}

// The canonicalized header prefixes. http.Header keys arrive in Go's canonical
// form, so these are matched as written rather than case-folded at each use.
const (
	binanceWeightPrefix = "X-Mbx-Used-Weight-"
	binanceOrderPrefix  = "X-Mbx-Order-Count-"
)

// BybitLimitReader reads X-Bapi-Limit and X-Bapi-Limit-Status.
//
// # This reader exists because the naive one is WRONG, not because it is tidy
//
// X-Bapi-Limit-Status is verbatim "your remaining requests for current
// endpoint" — the COMPLEMENT of what Binance reports (Profiles §6). §7.6's fold
// is upward-only over consumption, so folding the remaining count directly fails
// in both directions: a large remaining figure exceeds the local count and
// inflates it, while a SMALL one — the venue saying we are nearly banned — does
// not exceed it and is discarded. The discarded direction is the silent one and
// the one that ends in the ban.
//
// So the reading is converted: used = X-Bapi-Limit − X-Bapi-Limit-Status. With
// X-Bapi-Limit absent, NO observation is produced, because a remaining count
// with no cap beside it cannot become a consumption figure and a guessed cap is
// a budget quietly rescaled.
type BybitLimitReader struct{}

// CheckFold implements LimitReader and accepts every input. Bybit's
// per-endpoint fold names are caller-supplied from the venue package's roster,
// which transport cannot see. Their profile-side home belongs to #185;
// refusing non-roster names before then would refuse the only expressible
// local budgets.
func (BybitLimitReader) CheckFold(string, ratelimit.Dimension, time.Duration) error {
	return nil
}

// Observations implements LimitReader.
func (BybitLimitReader) Observations(h http.Header, call Call) []Observation {
	if call.Limit == "" {
		// Bybit's headers describe "current endpoint" without naming it, so
		// with no name from the caller there is nothing to attribute the
		// reading to. Guessing a name would charge the wrong budget.
		return nil
	}
	limit, okLimit := parseUintHeader(h, "X-Bapi-Limit")
	remaining, okRemaining := parseUintHeader(h, "X-Bapi-Limit-Status")
	if !okLimit || !okRemaining {
		return nil
	}
	if remaining > limit {
		// More remaining than the cap is a reading that cannot be true. It is
		// dropped rather than clamped to zero used: clamping would report the
		// most permissive possible consumption from a header we have just shown
		// we cannot interpret.
		return nil
	}
	return []Observation{{Family: call.Family, Name: call.Limit, Used: limit - remaining}}
}

func parseUintHeader(h http.Header, key string) (uint64, bool) {
	v := h.Get(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Compile-time proof that both readers satisfy the port (§3.6 rule 2).
var (
	_ LimitReader = BinanceLimitReader{}
	_ LimitReader = BybitLimitReader{}
)
