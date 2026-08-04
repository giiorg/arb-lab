package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// # A note on TIMEOUT!! (§47f's standing hazard for this slice)
//
// Nothing in this file reads a channel. The request counters are atomics and
// every httptest handler answers immediately, so a mutation that stops the
// client sending makes a test FAIL on a count rather than BLOCK on a receive.
// That is deliberate: a mutant that hangs reports TIMEOUT!!, which names no
// defect, and the cheapest way to not have that problem is to not have the
// channel.

const testFamily = ids.ProductLinearPerp

func testProfile(t *testing.T) ratelimit.Profile {
	t.Helper()
	return ratelimit.Profile{
		Venue:    testVenue,
		Families: []ids.ProductCategory{testFamily},
		Limits: []ratelimit.Limit{{
			Name:           "weight_1m",
			Scope:          ratelimit.ScopeFamily,
			Family:         testFamily,
			Dimension:      ratelimit.DimensionWeight,
			Interval:       time.Minute,
			OpeningCeiling: 1400,
			RoutineCeiling: 1800,
			Capacity:       2400,
		}, {
			// The mandatory venue envelope, roomy: this suite's subject is
			// signing and header folding, not admission policy.
			Name:           "ip_5s",
			Scope:          ratelimit.ScopeVenue,
			Dimension:      ratelimit.DimensionRequests,
			Interval:       5 * time.Second,
			OpeningCeiling: 100000,
			RoutineCeiling: 100000,
			Capacity:       100000,
		}},
		// A written waiver rather than a budget, because it is TRUE here: every
		// cost this suite presents charges weight (or is request-only), and the
		// governor's cannot-count refusal would catch the fixture lying.
		Uncounted: []ratelimit.Uncounted{{
			Family:    testFamily,
			Dimension: ratelimit.DimensionOrders,
			Reason:    "suite presents no order-charging cost; signing is the subject",
		}},
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	}
}

func testGovernor(t *testing.T) *ratelimit.Governor {
	t.Helper()
	clk, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{
		{Venue: testVenue, Offset: 0, Uncertainty: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(clk, testProfile(t))
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	return g
}

func bybitTestProfile() ratelimit.Profile {
	limits := []ratelimit.Limit{{
		Name: "ip_5s", Scope: ratelimit.ScopeVenue,
		Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
		OpeningCeiling: 1000, RoutineCeiling: 1000, Capacity: 1000,
	}}
	for _, endpoint := range (BybitLimitReader{}).Endpoints() {
		capacity := uint64(50)
		if endpoint.Name == BybitOrderCreateLimit {
			capacity = 10
		}
		limits = append(limits, ratelimit.Limit{
			Name: endpoint.Name, Scope: ratelimit.ScopeEndpoint,
			Dimension: endpoint.Dimension, Interval: time.Second,
			OpeningCeiling: capacity, RoutineCeiling: capacity, Capacity: capacity,
		})
	}
	return ratelimit.Profile{
		Venue: testVenue, Families: []ids.ProductCategory{testFamily}, Limits: limits,
		Uncounted: []ratelimit.Uncounted{
			{Family: testFamily, Dimension: ratelimit.DimensionWeight, Reason: "bybit publishes no per-endpoint weight"},
			{Family: testFamily, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered by order_create"},
		},
		ThrottlePause: 30 * time.Second, BanPause: 2 * time.Minute,
	}
}

func governorFromProfile(t *testing.T, p ratelimit.Profile) *ratelimit.Governor {
	t.Helper()
	clk, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{
		{Venue: testVenue, Offset: 0, Uncertainty: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	g, err := ratelimit.New(clk, p)
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	return g
}

func bybitTestGovernor(t *testing.T) *ratelimit.Governor {
	t.Helper()
	return governorFromProfile(t, bybitTestProfile())
}

func testClient(t *testing.T, srv *httptest.Server, gov *ratelimit.Governor) *SignedClient {
	t.Helper()
	clk, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{
		{Venue: testVenue, Offset: 0, Uncertainty: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	cred, err := NewCredential("test-key", "test-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	signer, err := NewBinanceSigner(testVenue, cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	c, err := NewSignedClient(SignedClientConfig{
		Scheme:     "http",
		Host:       host,
		Timeout:    5 * time.Second,
		ReadLimit:  1 << 16,
		RecvWindow: 5000 * time.Millisecond,
		Signer:     signer,
		Limits:     BinanceLimitReader{},
		Envelope:   BinanceEnvelope{},
		Governor:   gov,
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("NewSignedClient: %v", err)
	}
	return c
}

func testSignedClientConfig(t *testing.T, gov *ratelimit.Governor, limits LimitReader) SignedClientConfig {
	t.Helper()
	clk, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{
		{Venue: testVenue, Offset: 0, Uncertainty: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	cred, err := NewCredential("test-key", "test-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	signer, err := NewBinanceSigner(testVenue, cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	return SignedClientConfig{
		Scheme: "https", Host: "example.com", Timeout: 5 * time.Second, ReadLimit: 1 << 16,
		RecvWindow: 5000 * time.Millisecond, Signer: signer, Limits: limits,
		Envelope: BinanceEnvelope{}, Governor: gov, Clock: clk,
	}
}

func bybitTestClient(
	t *testing.T,
	srv *httptest.Server,
	gov *ratelimit.Governor,
	readLimit int64,
) *SignedClient {
	t.Helper()
	cfg := testSignedClientConfig(t, gov, BybitLimitReader{})
	cfg.Scheme = "http"
	cfg.Host = strings.TrimPrefix(srv.URL, "http://")
	cfg.ReadLimit = readLimit
	c, err := NewSignedClient(cfg)
	if err != nil {
		t.Fatalf("NewSignedClient: %v", err)
	}
	return c
}

func bybitReadCall() Call {
	call := testCall()
	call.Path = "/v5/order/realtime"
	call.Cost = ratelimit.RequestOnly()
	return call
}

func endpointUsed(t *testing.T, gov *ratelimit.Governor, name string) uint64 {
	t.Helper()
	for _, l := range gov.Snapshot().Limits {
		if l.Scope == ratelimit.ScopeEndpoint && l.Name == name {
			return l.Used
		}
	}
	t.Fatalf("endpoint limit %q missing from snapshot", name)
	return 0
}

func testCall() Call {
	return Call{
		Op:       "test.Call",
		Method:   "GET",
		Path:     "/fapi/v1/order",
		Params:   Params{{"symbol", "BTCUSDT"}},
		Family:   testFamily,
		Priority: ratelimit.PriorityOpening,
		Cost:     ratelimit.Cost{Weight: 1},
	}
}

// TestSignedClientSendsExactlyOnce is §5.3's "transport retries forbidden" as a
// run rather than a paragraph.
//
// The handler answers 503 — the status most likely to tempt a retry, and the
// one Binance documents as UNKNOWN execution — and the count must still be one.
func TestSignedClientSendsExactlyOnce(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := testClient(t, srv, testGovernor(t)).Do(context.Background(), testCall())
	if got := apperror.EffectOf(err); got != apperror.EffectUnknown {
		t.Errorf("effect on 503 = %v, want EffectUnknown", got)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("venue received %d requests, want exactly 1", n)
	}
}

// TestSignedClientAdmitsARequestOnlyCost is the seam #173's review found
// untested — and broken. Bybit's read surfaces charge nothing but the implicit
// request, and their truthful cost was the zero Cost, which Do's own
// validation refuses BEFORE Acquire — so every such call failed at the
// precondition while the codec suite and the governor suite each passed. The
// claim is the composition: RequestOnly() passes Do's validation, reaches the
// wire exactly once, and charges the venue envelope alone.
func TestSignedClientAdmitsARequestOnlyCost(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gov := testGovernor(t)
	call := testCall()
	call.Cost = ratelimit.RequestOnly()
	if _, err := testClient(t, srv, gov).Do(context.Background(), call); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("venue received %d requests, want 1", n)
	}
	for _, l := range gov.Snapshot().Limits {
		want := uint64(0)
		if l.Scope == ratelimit.ScopeVenue {
			want = 1
		}
		if l.Used != want {
			t.Errorf("limit %q used = %d, want %d — a request-only cost charges the envelope alone",
				l.Name, l.Used, want)
		}
	}
}

// TestSignedClientRefusedByGovernorSendsNothing: the local decline is
// EffectNotApplied, and it must be a claim about evidence — so nothing may
// reach the wire.
func TestSignedClientRefusedByGovernorSendsNothing(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	gov := testGovernor(t)
	// Push the venue into a hard pause, which refuses every priority.
	if err := gov.Penalize(ratelimit.Penalty{Kind: ratelimit.PenaltyBan}); err != nil {
		t.Fatalf("Penalize: %v", err)
	}

	_, err := testClient(t, srv, gov).Do(context.Background(), testCall())
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := apperror.EffectOf(err); got != apperror.EffectNotApplied {
		t.Errorf("effect = %v, want EffectNotApplied", got)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("venue received %d requests after a local refusal, want 0", n)
	}
}

// TestSignedClientFoldsHeadersEvenOnARefusal is the step-5-before-step-6 claim.
//
// A 429 carries the reading taken when it mattered most. If the status were
// classified first and returned, that reading would be discarded exactly when
// the governor most needs it.
func TestSignedClientFoldsHeadersEvenOnARefusal(t *testing.T) {
	t.Parallel()
	const observed = 1900
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-MBX-USED-WEIGHT-1M", fmt.Sprint(observed))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	gov := testGovernor(t)
	if _, err := testClient(t, srv, gov).Do(context.Background(), testCall()); err == nil {
		t.Fatal("expected a rate-limit error")
	}

	var found bool
	for _, ls := range gov.Snapshot().Limits {
		if ls.Name == "weight_1m" {
			found = true
			if ls.Used < observed {
				t.Errorf("used = %d after a 429 reporting %d; the header was not folded", ls.Used, observed)
			}
		}
	}
	if !found {
		t.Fatal("weight_1m missing from the snapshot")
	}
}

func TestSignedClientFoldsBybitHeadersBeforeBodyFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		readLimit int64
		handler   http.HandlerFunc
		wantErr   error
	}{
		{
			name: "mid-stream read failure", readLimit: 100,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Bapi-Limit", "50")
				w.Header().Set("X-Bapi-Limit-Status", "10")
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("short"))
			},
		},
		{
			name: "body exceeds read limit", readLimit: 4, wantErr: ErrBodyTooLarge,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Bapi-Limit", "50")
				w.Header().Set("X-Bapi-Limit-Status", "10")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("12345"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			gov := bybitTestGovernor(t)
			_, err := bybitTestClient(t, srv, gov, tc.readLimit).Do(context.Background(), bybitReadCall())
			if err == nil {
				t.Fatal("Do() = nil, want body handling error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("Do() = %v, want %v", err, tc.wantErr)
			}
			if got := endpointUsed(t, gov, BybitOrderRealtimeLimit); got != 40 {
				t.Errorf("order_realtime used = %d, want header ratchet to 40", got)
			}
		})
	}
}

func TestSignedClientChargesAndFoldsTheSameBybitEndpointWindow(t *testing.T) {
	t.Parallel()

	t.Run("local charge without headers", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		gov := bybitTestGovernor(t)
		if _, err := bybitTestClient(t, srv, gov, 1024).Do(context.Background(), bybitReadCall()); err != nil {
			t.Fatalf("Do: %v", err)
		}
		for _, endpoint := range (BybitLimitReader{}).Endpoints() {
			want := uint64(0)
			if endpoint.Name == BybitOrderRealtimeLimit {
				want = 1
			}
			if got := endpointUsed(t, gov, endpoint.Name); got != want {
				t.Errorf("%s used = %d, want %d", endpoint.Name, got, want)
			}
		}
	})

	t.Run("header fold", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Bapi-Limit", "50")
			w.Header().Set("X-Bapi-Limit-Status", "10")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		gov := bybitTestGovernor(t)
		if _, err := bybitTestClient(t, srv, gov, 1024).Do(context.Background(), bybitReadCall()); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if got := endpointUsed(t, gov, BybitOrderRealtimeLimit); got != 40 {
			t.Errorf("order_realtime used = %d, want local charge followed by header ratchet to 40", got)
		}
	})
}

func TestSignedClientRefusesAnUnbudgetedDerivedBybitEndpointBeforeSend(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Construction normally enforces the complete roster. Replacing the reader
	// after constructing a Binance-shaped fixture drives the governor's R1 as
	// defense in depth, proving the disagreement still cannot reach the wire.
	c := testClient(t, srv, testGovernor(t))
	c.cfg.Limits = BybitLimitReader{}
	_, err := c.Do(context.Background(), bybitReadCall())
	if !errors.Is(err, ratelimit.ErrEndpointUnbudgeted) {
		t.Fatalf("Do() = %v, want ErrEndpointUnbudgeted", err)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("server received %d requests, want 0", got)
	}
}

func TestNewSignedClientChecksFamilyFoldNames(t *testing.T) {
	t.Parallel()
	profile := testProfile(t)
	profile.Limits[0].Name = "spot_weight"
	cfg := testSignedClientConfig(t, testGovernor(t), BinanceLimitReader{})
	gov, err := ratelimit.New(cfg.Clock, profile)
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	cfg.Governor = gov
	_, err = NewSignedClient(cfg)
	if !errors.Is(err, ErrLimitFoldUnaddressable) {
		t.Fatalf("NewSignedClient error = %v, want ErrLimitFoldUnaddressable", err)
	}
	for _, want := range []string{"spot_weight", "weight_1m"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("NewSignedClient error %q does not name %q", err, want)
		}
	}

	if _, err := NewSignedClient(testSignedClientConfig(t, testGovernor(t), BinanceLimitReader{})); err != nil {
		t.Errorf("conforming profile was refused: %v", err)
	}
}

// TestNewSignedClientSkipsVenueScopedFoldNames.
//
// COLLAPSE CHECK: the venue envelope deliberately shares weight_1m with the
// family budget. Checking every scope would mistake that legal, separately
// keyed requests budget for a cross-dimension collision.
func TestNewSignedClientSkipsVenueScopedFoldNames(t *testing.T) {
	t.Parallel()
	profile := testProfile(t)
	profile.Limits[1].Name = "weight_1m"
	cfg := testSignedClientConfig(t, testGovernor(t), BinanceLimitReader{})
	gov, err := ratelimit.New(cfg.Clock, profile)
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	cfg.Governor = gov
	if _, err := NewSignedClient(cfg); err != nil {
		t.Fatalf("NewSignedClient: %v", err)
	}
}

func TestNewSignedClientRequiresBybitEndpointRoster(t *testing.T) {
	t.Parallel()

	t.Run("all five construct", func(t *testing.T) {
		t.Parallel()
		gov := bybitTestGovernor(t)
		if _, err := NewSignedClient(testSignedClientConfig(t, gov, BybitLimitReader{})); err != nil {
			t.Fatalf("NewSignedClient: %v", err)
		}
	})

	t.Run("one missing is refused", func(t *testing.T) {
		t.Parallel()
		p := bybitTestProfile()
		for i, l := range p.Limits {
			if l.Name == BybitOrderHistoryLimit {
				p.Limits = append(p.Limits[:i], p.Limits[i+1:]...)
				break
			}
		}
		_, err := NewSignedClient(testSignedClientConfig(t, governorFromProfile(t, p), BybitLimitReader{}))
		if !errors.Is(err, ErrEndpointBudgetMissing) {
			t.Fatalf("NewSignedClient = %v, want ErrEndpointBudgetMissing", err)
		}
		if !strings.Contains(err.Error(), BybitOrderHistoryLimit) {
			t.Errorf("NewSignedClient = %q, want missing endpoint name", err)
		}
	})

	t.Run("wrong dimension is refused", func(t *testing.T) {
		t.Parallel()
		p := bybitTestProfile()
		for i := range p.Limits {
			if p.Limits[i].Name == BybitOrderHistoryLimit {
				p.Limits[i].Dimension = ratelimit.DimensionOrders
			}
		}
		_, err := NewSignedClient(testSignedClientConfig(t, governorFromProfile(t, p), BybitLimitReader{}))
		if !errors.Is(err, ErrEndpointBudgetMissing) {
			t.Fatalf("NewSignedClient = %v, want ErrEndpointBudgetMissing", err)
		}
		for _, want := range []string{BybitOrderHistoryLimit, "DimensionOrders", "DimensionRequests"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("NewSignedClient = %q, want %q", err, want)
			}
		}
	})

	t.Run("wrong interval is refused by the dialect walk", func(t *testing.T) {
		t.Parallel()
		p := bybitTestProfile()
		for i := range p.Limits {
			if p.Limits[i].Name == BybitOrderRealtimeLimit {
				p.Limits[i].Interval = 2 * time.Second
			}
		}
		_, err := NewSignedClient(testSignedClientConfig(t, governorFromProfile(t, p), BybitLimitReader{}))
		if !errors.Is(err, ErrLimitFoldUnaddressable) {
			t.Fatalf("NewSignedClient = %v, want ErrLimitFoldUnaddressable", err)
		}
		if !strings.Contains(err.Error(), BybitOrderRealtimeLimit) {
			t.Errorf("NewSignedClient = %q, want limit name", err)
		}
	})
}

func TestNewSignedClientRefusesEndpointBudgetsForBinance(t *testing.T) {
	t.Parallel()
	p := testProfile(t)
	p.Limits = append(p.Limits, ratelimit.Limit{
		Name: "local_endpoint", Scope: ratelimit.ScopeEndpoint,
		Dimension: ratelimit.DimensionRequests, Interval: time.Second,
		OpeningCeiling: 10, RoutineCeiling: 10, Capacity: 10,
	})
	_, err := NewSignedClient(testSignedClientConfig(t, governorFromProfile(t, p), BinanceLimitReader{}))
	if !errors.Is(err, ErrLimitFoldUnaddressable) {
		t.Fatalf("NewSignedClient = %v, want ErrLimitFoldUnaddressable", err)
	}
}

// TestSignedClientDoesNotFollowRedirects.
//
// A redirect is a second request, and this client sends exactly one. It is also
// a credential leak: Go's default policy would re-send the signed query — and on
// Binance the signature travels IN that query — to whatever host the redirect
// names. The handler here redirects to a second server that records any arrival.
func TestSignedClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()
	var elsewhere atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		elsewhere.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, other.URL+"/leaked", http.StatusFound)
	}))
	defer srv.Close()

	resp, err := testClient(t, srv, testGovernor(t)).Do(context.Background(), testCall())
	if err == nil {
		t.Fatal("a 302 is not a success and must classify as an error")
	}
	if resp.Status != http.StatusFound {
		t.Errorf("status = %d, want the redirect itself to be returned", resp.Status)
	}
	if n := elsewhere.Load(); n != 0 {
		t.Errorf("the signed request reached the redirect target %d times, want 0", n)
	}
}

// TestClassifyTransportError: only a failed dial is proved not-sent.
func TestClassifyTransportError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want apperror.Effect
	}{
		{"dial refused", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, apperror.EffectNotApplied},
		{"write failed", &net.OpError{Op: "write", Err: errors.New("broken pipe")}, apperror.EffectUnknown},
		{"read failed", &net.OpError{Op: "read", Err: errors.New("reset")}, apperror.EffectUnknown},
		{"deadline", context.DeadlineExceeded, apperror.EffectUnknown},
		{"bare error", errors.New("something"), apperror.EffectUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyTransportError("op", tc.err).Effect(); got != tc.want {
				t.Errorf("effect = %v, want %v", got, tc.want)
			}
		})
	}
	if ClassifyTransportError("op", nil) != nil {
		t.Error("nil error must classify to nil")
	}
}

// TestClassifyStatus.
//
// COLLAPSE CHECK: the success range is tested at 200 and 299 and refuted at 199
// and 300. A fixture using only 200 and 500 makes `>= 200 && < 300` and
// `< 400` the same predicate.
func TestClassifyStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		status     int
		code       string
		wantErr    bool
		wantEffect apperror.Effect
		wantClass  apperror.Class
		wantPen    ratelimit.PenaltyKind
	}{
		{name: "200 accepted", status: 200},
		{name: "299 still accepted", status: 299},
		{name: "199 is not a success", status: 199, wantErr: true, wantEffect: apperror.EffectUnknown, wantClass: apperror.ClassTransport},
		{name: "300 is not a success", status: 300, wantErr: true, wantEffect: apperror.EffectUnknown, wantClass: apperror.ClassTransport},
		{name: "429 throttle", status: 429, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassRateLimit, wantPen: ratelimit.PenaltyThrottle},
		{name: "418 ban", status: 418, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassRateLimit, wantPen: ratelimit.PenaltyBan},
		{name: "401 no body", status: 401, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassAuth},
		{name: "403 bare is auth", status: 403, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassAuth},
		{name: "403 with 10006 is throttle", status: 403, code: bybitCodeRateLimited, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassRateLimit, wantPen: ratelimit.PenaltyThrottle},
		{name: "403 with 10018 is a ban", status: 403, code: bybitCodeIPBanned, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassRateLimit, wantPen: ratelimit.PenaltyBan},
		{name: "400 rejected on merits", status: 400, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassVenueReject},
		{name: "499 still a rejection", status: 499, wantErr: true, wantEffect: apperror.EffectNotApplied, wantClass: apperror.ClassVenueReject},
		{name: "500 unknown", status: 500, wantErr: true, wantEffect: apperror.EffectUnknown, wantClass: apperror.ClassTransport},
		{name: "503 documented unknown", status: 503, wantErr: true, wantEffect: apperror.EffectUnknown, wantClass: apperror.ClassTransport},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err, pen := classifyStatus("op", tc.status, tc.code, 0, "detail")
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("status %d: got %v, want nil", tc.status, err)
				}
				if pen != ratelimit.PenaltyUnspecified {
					t.Errorf("status %d earned penalty %v", tc.status, pen)
				}
				return
			}
			if err == nil {
				t.Fatalf("status %d: got nil, want an error", tc.status)
			}
			if err.Effect() != tc.wantEffect {
				t.Errorf("status %d effect = %v, want %v", tc.status, err.Effect(), tc.wantEffect)
			}
			if err.Class() != tc.wantClass {
				t.Errorf("status %d class = %v, want %v", tc.status, err.Class(), tc.wantClass)
			}
			if pen != tc.wantPen {
				t.Errorf("status %d penalty = %v, want %v", tc.status, pen, tc.wantPen)
			}
		})
	}
}

// TestClassifyStatusFallsBackToTheHTTPStatusForACode: Bybit's zero-length 401
// carries no retCode, and an error with an empty Code is indistinguishable from
// every other error of its class in a log.
func TestClassifyStatusFallsBackToTheHTTPStatusForACode(t *testing.T) {
	t.Parallel()
	err, _ := classifyStatus("op", 401, "", 0, "")
	if err.Code() != "401" {
		t.Errorf("Code() = %q, want %q", err.Code(), "401")
	}
	withCode, _ := classifyStatus("op", 401, "10003", 0, "")
	if withCode.Code() != "10003" {
		t.Errorf("Code() = %q, want the venue's own code", withCode.Code())
	}
}

// TestClassifyAckDecodeFailureIsApplied: the rare third value.
func TestClassifyAckDecodeFailureIsApplied(t *testing.T) {
	t.Parallel()
	got := ClassifyAckDecodeFailure("op", errors.New("bad json")).Effect()
	if got != apperror.EffectApplied {
		t.Errorf("effect = %v, want EffectApplied", got)
	}
}

// TestBybitLimitReaderConvertsRemainingToUsed is the §7.6 polarity fix.
//
// COLLAPSE CHECK, and it is the sharpest one in the slice: the fixture uses
// limit 100 and remaining 40, so the answer 60 differs from BOTH inputs. With
// limit 100 and remaining 50 — the fixture a person writes without thinking —
// used, remaining and limit/2 all equal 50, and an implementation that returned
// the raw header value would pass while folding exactly the wrong quantity.
func TestBybitLimitReaderConvertsRemainingToUsed(t *testing.T) {
	t.Parallel()
	const (
		limit     = 100
		remaining = 40
		wantUsed  = limit - remaining
	)
	if wantUsed == remaining || wantUsed == limit {
		t.Fatal("fixture collapsed: used must differ from both header values")
	}

	h := http.Header{}
	h.Set("X-Bapi-Limit", fmt.Sprint(limit))
	h.Set("X-Bapi-Limit-Status", fmt.Sprint(remaining))

	obs := BybitLimitReader{}.Observations(h, Call{Family: testFamily, Path: "/v5/order/realtime"})
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}
	if obs[0].Used != wantUsed {
		t.Errorf("Used = %d, want %d (raw remaining is %d, cap is %d)", obs[0].Used, wantUsed, remaining, limit)
	}
	if obs[0].Name != "order_realtime" {
		t.Errorf("Name = %q, want the path-derived endpoint name", obs[0].Name)
	}
	if obs[0].Family != "" {
		t.Errorf("Family = %q, want the venue-level endpoint keyspace", obs[0].Family)
	}
}

func TestLimitReadersDeriveEndpointNames(t *testing.T) {
	t.Parallel()
	bybit := BybitLimitReader{}
	cases := []struct {
		path string
		want string
	}{
		{"/v5/order/create", "order_create"},
		{"/v5/order/realtime", "order_realtime"},
		{"/v5/order/history", "order_history"},
		{"/v5/execution/list", "execution_list"},
		{"/v5/position/list", "position_list"},
		{"/v5/order/unknown", ""},
	}
	for _, tc := range cases {
		if got := bybit.Endpoint(tc.path); got != tc.want {
			t.Errorf("Bybit Endpoint(%q) = %q, want %q", tc.path, got, tc.want)
		}
		if got := (BinanceLimitReader{}).Endpoint(tc.path); got != "" {
			t.Errorf("Binance Endpoint(%q) = %q, want empty", tc.path, got)
		}
	}
}

func TestBinanceLimitReaderRefusesEndpointScope(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"anything", "weight_1m"} {
		err := (BinanceLimitReader{}).CheckFold(
			ratelimit.ScopeEndpoint, name, ratelimit.DimensionRequests, time.Second,
		)
		if !errors.Is(err, ErrLimitFoldUnaddressable) {
			t.Errorf("CheckFold(%q) = %v, want ErrLimitFoldUnaddressable", name, err)
		}
	}
}

// TestBybitLimitReaderDerivesNothingWithoutBothHeaders: a remaining count with
// no cap cannot become a consumption figure, and a guessed cap is a budget
// quietly rescaled.
func TestBybitLimitReaderDerivesNothingWithoutBothHeaders(t *testing.T) {
	t.Parallel()
	capOnly := http.Header{}
	capOnly.Set("X-Bapi-Limit", "100")
	remainingOnly := http.Header{}
	remainingOnly.Set("X-Bapi-Limit-Status", "40")
	inverted := http.Header{}
	inverted.Set("X-Bapi-Limit", "40")
	inverted.Set("X-Bapi-Limit-Status", "100") // more remaining than the cap
	both := http.Header{}
	both.Set("X-Bapi-Limit", "100")
	both.Set("X-Bapi-Limit-Status", "40")

	cases := map[string]struct {
		h    http.Header
		call Call
	}{
		"cap only":            {capOnly, Call{Family: testFamily, Path: "/v5/order/realtime"}},
		"remaining only":      {remainingOnly, Call{Family: testFamily, Path: "/v5/order/realtime"}},
		"remaining above cap": {inverted, Call{Family: testFamily, Path: "/v5/order/realtime"}},
		"unmapped path":       {both, Call{Family: testFamily, Path: "/v5/order/unknown"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if obs := (BybitLimitReader{}).Observations(tc.h, tc.call); len(obs) != 0 {
				t.Errorf("got %d observations, want 0: %+v", len(obs), obs)
			}
		})
	}
}

// TestBinanceLimitReaderNamesBothDimensions.
//
// COLLAPSE CHECK: both a weight header and an order-count header are present,
// with DIFFERENT window suffixes and DIFFERENT values. A fixture with one
// header cannot catch the two prefixes being swapped, and equal values cannot
// catch the two readings being crossed.
func TestBinanceLimitReaderNamesBothDimensions(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("X-MBX-USED-WEIGHT-1M", "1200")
	h.Set("X-MBX-ORDER-COUNT-10S", "7")

	got := map[string]uint64{}
	for _, o := range (BinanceLimitReader{}).Observations(h, Call{Family: testFamily}) {
		got[o.Name] = o.Used
	}
	want := map[string]uint64{"weight_1m": 1200, "orders_10s": 7}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
}

func TestBinanceLimitReaderCanonicalizesHeaderWindows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		headerName string
		wantName   string
	}{
		{name: "seconds become minutes", headerName: "X-MBX-USED-WEIGHT-60S", wantName: "weight_1m"},
		{name: "minutes stay minutes", headerName: "X-MBX-USED-WEIGHT-1M", wantName: "weight_1m"},
		{name: "unparseable suffix", headerName: "X-MBX-USED-WEIGHT-XYZ"},
		{name: "overflowing suffix", headerName: "X-MBX-USED-WEIGHT-36028797018963969S"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := http.Header{}
			h.Set(tc.headerName, "17")
			obs := (BinanceLimitReader{}).Observations(h, Call{Family: testFamily})
			if tc.wantName == "" {
				if len(obs) != 0 {
					t.Fatalf("got %+v, want no observations", obs)
				}
				return
			}
			if len(obs) != 1 {
				t.Fatalf("got %d observations, want 1: %+v", len(obs), obs)
			}
			if obs[0].Name != tc.wantName {
				t.Errorf("Name = %q, want %q", obs[0].Name, tc.wantName)
			}
		})
	}
}

// TestBinanceLimitReaderMergesEquivalentHeaderWindows.
//
// COLLAPSE CHECK: the two duration-equivalent spellings carry different
// readings. The observation count catches either one being dropped, and the
// governor's exact maximum catches a split fold target.
func TestBinanceLimitReaderMergesEquivalentHeaderWindows(t *testing.T) {
	t.Parallel()
	const (
		secondsReading = 1700
		minutesReading = 1900
	)
	h := http.Header{}
	h.Set("X-MBX-USED-WEIGHT-60S", fmt.Sprint(secondsReading))
	h.Set("X-MBX-USED-WEIGHT-1M", fmt.Sprint(minutesReading))
	obs := (BinanceLimitReader{}).Observations(h, Call{Family: testFamily})
	if len(obs) != 2 {
		t.Fatalf("got %d observations, want 2: %+v", len(obs), obs)
	}
	for _, o := range obs {
		if o.Name != "weight_1m" {
			t.Errorf("Name = %q, want weight_1m", o.Name)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-MBX-USED-WEIGHT-60S", fmt.Sprint(secondsReading))
		w.Header().Set("X-MBX-USED-WEIGHT-1M", fmt.Sprint(minutesReading))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	gov := testGovernor(t)
	if _, err := testClient(t, srv, gov).Do(context.Background(), testCall()); err != nil {
		t.Fatalf("Do: %v", err)
	}
	var found bool
	for _, l := range gov.Snapshot().Limits {
		if l.Name == "weight_1m" {
			found = true
			if l.Used != minutesReading {
				t.Errorf("weight_1m used = %d, want max reading %d", l.Used, minutesReading)
			}
		}
	}
	if !found {
		t.Fatal("weight_1m missing from the snapshot")
	}
}

// TestBinanceLimitReaderChecksFoldNames.
//
// COLLAPSE CHECK: the mismatches separate prefix from window, and request-name
// collisions vary the declared interval. A prefix-only check, a window-only
// check, or a collision check coupled to the limit's interval fails a row.
func TestBinanceLimitReaderChecksFoldNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		limitName  string
		dimension  ratelimit.Dimension
		interval   time.Duration
		wantErr    bool
		wantInText []string
	}{
		{name: "weight agrees", limitName: "weight_1m", dimension: ratelimit.DimensionWeight, interval: time.Minute},
		{name: "orders agree", limitName: "orders_10s", dimension: ratelimit.DimensionOrders, interval: 10 * time.Second},
		{name: "weight prefix mismatch", limitName: "spot_weight", dimension: ratelimit.DimensionWeight, interval: time.Minute, wantErr: true, wantInText: []string{"spot_weight", "weight_1m"}},
		{name: "weight window mismatch", limitName: "weight_1m", dimension: ratelimit.DimensionWeight, interval: 10 * time.Second, wantErr: true, wantInText: []string{"weight_1m", "weight_10s"}},
		{name: "weight cross prefix", limitName: "orders_1m", dimension: ratelimit.DimensionWeight, interval: time.Minute, wantErr: true, wantInText: []string{"orders_1m", "weight_1m"}},
		{name: "requests collide with weight", limitName: "weight_1m", dimension: ratelimit.DimensionRequests, interval: time.Minute, wantErr: true},
		{name: "requests collision ignores interval", limitName: "weight_1m", dimension: ratelimit.DimensionRequests, interval: 5 * time.Second, wantErr: true},
		{name: "requests collide with orders", limitName: "orders_10s", dimension: ratelimit.DimensionRequests, interval: time.Minute, wantErr: true},
		{name: "requests ordinary name", limitName: "ip_requests", dimension: ratelimit.DimensionRequests, interval: 5 * time.Second},
		{name: "requests weight guardrail", limitName: "weight_guardrail", dimension: ratelimit.DimensionRequests, interval: time.Minute},
		{name: "requests orders local", limitName: "orders_local", dimension: ratelimit.DimensionRequests, interval: 10 * time.Second},
		{name: "requests noncanonical window", limitName: "weight_60s", dimension: ratelimit.DimensionRequests, interval: time.Minute},
		{name: "weight unrenderable interval", limitName: "weight_500ms", dimension: ratelimit.DimensionWeight, interval: 500 * time.Millisecond, wantErr: true},
		{name: "requests suffix exceeds uint64", limitName: "weight_18446744073709551616s", dimension: ratelimit.DimensionRequests, interval: time.Minute},
		{name: "requests suffix multiplication overflows", limitName: "weight_36028797018963969s", dimension: ratelimit.DimensionRequests, interval: time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := (BinanceLimitReader{}).CheckFold(ratelimit.ScopeFamily, tc.limitName, tc.dimension, tc.interval)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("CheckFold: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrLimitFoldUnaddressable) {
				t.Fatalf("CheckFold error = %v, want ErrLimitFoldUnaddressable", err)
			}
			for _, want := range tc.wantInText {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("CheckFold error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestBinanceWindowRendersCanonically(t *testing.T) {
	t.Parallel()
	cases := []struct {
		window time.Duration
		want   string
	}{
		{10 * time.Second, "10s"},
		{90 * time.Second, "90s"},
		{time.Minute, "1m"},
		{2 * time.Hour, "2h"},
		{24 * time.Hour, "1d"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got, ok := binanceWindow(tc.window)
			if !ok || got != tc.want {
				t.Errorf("binanceWindow(%s) = %q, %t; want %q, true", tc.window, got, ok, tc.want)
			}
		})
	}
}

func TestBybitLimitReaderChecksFoldAgreement(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		scope     ratelimit.Scope
		dimension ratelimit.Dimension
		interval  time.Duration
		wantErr   bool
	}{
		{name: "endpoint row agrees", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: time.Second},
		{name: "non-roster endpoint", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: time.Second, wantErr: true},
		{name: "history wrong dimension", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionOrders, interval: time.Second, wantErr: true},
		{name: "create wrong dimension", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: time.Second, wantErr: true},
		{name: "999ms", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: 999 * time.Millisecond, wantErr: true},
		{name: "1001ms", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: 1001 * time.Millisecond, wantErr: true},
		{name: "2s", scope: ratelimit.ScopeEndpoint, dimension: ratelimit.DimensionRequests, interval: 2 * time.Second, wantErr: true},
		{name: "family roster squatter", scope: ratelimit.ScopeFamily, dimension: ratelimit.DimensionRequests, interval: time.Second, wantErr: true},
		{name: "ordinary family name", scope: ratelimit.ScopeFamily, dimension: ratelimit.DimensionWeight, interval: time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limitName := BybitOrderHistoryLimit
			switch tc.name {
			case "endpoint row agrees", "history wrong dimension", "999ms", "1001ms", "2s", "family roster squatter":
				limitName = BybitOrderHistoryLimit
			case "create wrong dimension":
				limitName = BybitOrderCreateLimit
			case "non-roster endpoint", "ordinary family name":
				limitName = "ordinary_limit"
			}
			err := (BybitLimitReader{}).CheckFold(tc.scope, limitName, tc.dimension, tc.interval)
			if tc.wantErr && !errors.Is(err, ErrLimitFoldUnaddressable) {
				t.Fatalf("CheckFold() = %v, want ErrLimitFoldUnaddressable", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("CheckFold() = %v, want nil", err)
			}
		})
	}
}

// TestBinanceLimitReaderDropsAnUnparseableCount: zero is the one value that can
// never raise the local count, so reading a malformed header as zero would be
// indistinguishable from a venue reporting no consumption.
func TestBinanceLimitReaderDropsAnUnparseableCount(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("X-MBX-USED-WEIGHT-1M", "not-a-number")
	if obs := (BinanceLimitReader{}).Observations(h, Call{Family: testFamily}); len(obs) != 0 {
		t.Errorf("got %+v, want no observations", obs)
	}
}

// TestCredentialNeverRendersTheSecret covers String, GoString, %v and %#v.
//
// COLLAPSE CHECK: key and secret have DIFFERENT lengths, so a redaction that
// crossed the two fields is visible. With equal lengths the format string could
// name them in either order and print the same text.
func TestCredentialNeverRendersTheSecret(t *testing.T) {
	t.Parallel()
	const (
		key    = "public-key-value"          // 16
		secret = "super-secret-value-longer" // 25
	)
	if len(key) == len(secret) {
		t.Fatal("fixture collapsed: key and secret must differ in length")
	}
	c, err := NewCredential(key, secret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}

	// The EXACT expected rendering, not a containment check.
	//
	// A substring assertion is what let a swapped key/secret pair survive
	// mutation here: with the two lengths traded, "key:25B secret:16B" still
	// CONTAINS "25", so `strings.Contains(rendered, len(secret))` held while the
	// fields were crossed. The fixture was already varied correctly — the two
	// lengths differ — and the ASSERTION was what could not tell the difference.
	// That is the collapse checklist's sibling: vary the fixture, then check the
	// predicate can still see the variation.
	want := fmt.Sprintf("<credential key:%dB secret:%dB>", len(key), len(secret))

	for _, rendered := range []string{
		c.String(), c.GoString(),
		fmt.Sprintf("%v", c), fmt.Sprintf("%s", c), fmt.Sprintf("%#v", c),
	} {
		if strings.Contains(rendered, secret) {
			t.Errorf("secret leaked in %q", rendered)
		}
		if rendered != want {
			t.Errorf("rendering = %q, want %q", rendered, want)
		}
	}
	// A Credential inside another struct: fmt applies the Stringer to fields, so
	// this must carry the redaction too, but the surrounding braces mean it is
	// a containment check by necessity.
	if nested := fmt.Sprintf("%v", struct{ Cred Credential }{c}); !strings.Contains(nested, want) {
		t.Errorf("nested rendering = %q, want it to contain %q", nested, want)
	} else if strings.Contains(nested, secret) {
		t.Errorf("secret leaked in nested rendering %q", nested)
	}
	if _, err := c.MarshalJSON(); !errors.Is(err, ErrCredentialNotSerializable) {
		t.Errorf("MarshalJSON: got %v, want ErrCredentialNotSerializable", err)
	}
}

// TestNewCredentialRefusesAnEmptyHalf: both halves, separately named.
func TestNewCredentialRefusesAnEmptyHalf(t *testing.T) {
	t.Parallel()
	if _, err := NewCredential("", "s"); !errors.Is(err, ErrCredentialKeyUnset) {
		t.Errorf("empty key: got %v", err)
	}
	if _, err := NewCredential("k", ""); !errors.Is(err, ErrCredentialSecretUnset) {
		t.Errorf("empty secret: got %v", err)
	}
	if err := (Credential{}).Validate(); err == nil {
		t.Error("the zero credential must not validate")
	}
}

// TestSignedClientConfigRefusesEveryMissingDependency: the zero value is
// unusable, one field at a time.
func TestSignedClientConfigRefusesEveryMissingDependency(t *testing.T) {
	t.Parallel()
	cred, err := NewCredential("k", "s")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	signer, err := NewBinanceSigner(testVenue, cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	clk, err := clock.NewTestClock(time.UnixMilli(1).UTC(), nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	good := SignedClientConfig{
		Scheme: "https", Host: "example.com", Timeout: time.Second, ReadLimit: 1024,
		RecvWindow: time.Second, Signer: signer, Limits: BinanceLimitReader{},
		Envelope: BinanceEnvelope{},
		Governor: testGovernor(t), Clock: clk,
	}
	// Positive control: the complete config must pass, or every case below
	// would "pass" against a Validate that refuses everything.
	if err := good.Validate(); err != nil {
		t.Fatalf("the complete config was refused: %v", err)
	}

	cases := map[string]func(*SignedClientConfig){
		"scheme":      func(c *SignedClientConfig) { c.Scheme = "" },
		"host":        func(c *SignedClientConfig) { c.Host = "" },
		"timeout":     func(c *SignedClientConfig) { c.Timeout = 0 },
		"read limit":  func(c *SignedClientConfig) { c.ReadLimit = 0 },
		"recv window": func(c *SignedClientConfig) { c.RecvWindow = 0 },
		"signer":      func(c *SignedClientConfig) { c.Signer = nil },
		"limits":      func(c *SignedClientConfig) { c.Limits = nil },
		"envelope":    func(c *SignedClientConfig) { c.Envelope = nil },
		"governor":    func(c *SignedClientConfig) { c.Governor = nil },
		"clock":       func(c *SignedClientConfig) { c.Clock = nil },
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := good
			break_(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Errorf("a config missing %s was accepted", name)
			}
		})
	}
}
