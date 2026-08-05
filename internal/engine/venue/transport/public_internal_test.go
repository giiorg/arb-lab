package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func publicFor(t *testing.T, srv *httptest.Server, path string, params Params, limit int64) *PublicClient {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "http://")
	c, err := NewPublicClient(PublicConfig{
		Scheme: "http", Host: host, Path: path, Params: params,
		Timeout: 5 * time.Second, ReadLimit: limit,
	})
	if err != nil {
		t.Fatalf("NewPublicClient: %v", err)
	}
	return c
}

// The params reach the venue in the order given, not sorted — the fixture's
// order (symbol before category) disagrees with the sorted order, so a
// url.Values-based rendering would fail this while producing a working URL.
func TestPublicFetchSendsOrderedQuery(t *testing.T) {
	const body = `{"retCode":0}`
	var sawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v5/market/instruments-info" {
			t.Errorf("venue saw %s %s", r.Method, r.URL.Path)
		}
		sawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	params := Params{{Key: "symbol", Value: "BTCUSDT"}, {Key: "category", Value: "spot"}}
	res, err := publicFor(t, srv, "/v5/market/instruments-info", params, 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if sawQuery != "symbol=BTCUSDT&category=spot" {
		t.Fatalf("venue saw query %q, want symbol before category", sawQuery)
	}
	if res.Status != http.StatusOK || string(res.Body) != body {
		t.Fatalf("got (%d, %q), want (200, %q)", res.Status, res.Body, body)
	}
}

// A redirect is NOT followed: the URL is a provenance claim on rows that gate
// orders, and following would attribute another host's body to it. The 3xx
// surfaces as a result — the caller's non-200 refusal names it — and the
// redirect TARGET is proven untouched, which is the half a status assertion
// alone cannot show.
func TestPublicFetchDoesNotFollowRedirects(t *testing.T) {
	var targetHit bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		_, _ = w.Write([]byte(`{"looks":"valid"}`))
	}))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer src.Close()

	res, err := publicFor(t, src, "/x", nil, 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Status != http.StatusFound {
		t.Fatalf("status %d, want 302 — the redirect is the result, not a hop", res.Status)
	}
	if targetHit {
		t.Fatal("the redirect target was fetched; the recorded URL would misattribute its body")
	}
}

// A non-2xx is a result here exactly as it is for the Poller — the shared
// doGet contract, asserted through THIS call site.
func TestPublicFetchNon2xxIsAResultNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"retCode":10006,"retMsg":"rate"}`))
	}))
	defer srv.Close()

	res, err := publicFor(t, srv, "/x", nil, 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Status != http.StatusTooManyRequests || !strings.Contains(string(res.Body), "10006") {
		t.Fatalf("got (%d, %q), want the venue's own refusal verbatim", res.Status, res.Body)
	}
}

// The boundary again through THIS call site: the client hands doGet its own
// ReadLimit, and a call site hardcoding a degenerate argument is a defect a
// tested helper cannot show.
func TestPublicFetchBodyLimitBoundary(t *testing.T) {
	body := strings.Repeat("x", 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Run("exactly at the limit", func(t *testing.T) {
		res, err := publicFor(t, srv, "/x", nil, 64).Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch at limit: %v", err)
		}
		if len(res.Body) != 64 {
			t.Fatalf("body %d bytes, want 64", len(res.Body))
		}
	})
	t.Run("one byte over", func(t *testing.T) {
		_, err := publicFor(t, srv, "/x", nil, 63).Fetch(context.Background())
		if !errors.Is(err, ErrBodyTooLarge) {
			t.Fatalf("Fetch over limit = %v, want %v", err, ErrBodyTooLarge)
		}
	})
}

func TestPublicConfigValidate(t *testing.T) {
	ok := PublicConfig{Scheme: "https", Host: "api.bybit.com",
		Path:    "/v5/market/instruments-info",
		Params:  Params{{Key: "category", Value: "spot"}, {Key: "symbol", Value: "BTCUSDT"}},
		Timeout: 10 * time.Second, ReadLimit: 1 << 20}

	cases := []struct {
		name string
		mut  func(c *PublicConfig)
		want error
	}{
		{"the real target validates", func(c *PublicConfig) {}, nil},
		{"no params is legal (fapi ignores them)", func(c *PublicConfig) { c.Params = nil }, nil},
		// One case per shared sentinel proves the delegation to
		// PollerConfig.Validate is wired; that type's own battery owns the
		// full surface.
		{"scheme unset", func(c *PublicConfig) { c.Scheme = "" }, ErrSchemeUnset},
		{"path with a query is the params' job", func(c *PublicConfig) { c.Path = "/v5?x=1" }, ErrPathMalformed},
		{"timeout zero", func(c *PublicConfig) { c.Timeout = 0 }, ErrTimeoutRange},
		{"read limit zero", func(c *PublicConfig) { c.ReadLimit = 0 }, ErrReadLimitRange},
		// The offending param is the SECOND one, so the index in the message
		// and "the first param" part ways.
		{"param key unset", func(c *PublicConfig) { c.Params[1].Key = "" }, ErrParamKeyUnset},
		{"param value unset", func(c *PublicConfig) { c.Params[1].Value = "" }, ErrParamValueUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ok
			c.Params = append(Params(nil), ok.Params...)
			tc.mut(&c)
			err := c.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPublicConfigURL(t *testing.T) {
	t.Run("params in given order, percent-encoded", func(t *testing.T) {
		c := PublicConfig{Scheme: "https", Host: "api.bybit.com", Path: "/v5/market/instruments-info",
			Params: Params{{Key: "symbol", Value: "BTC USDT"}, {Key: "category", Value: "linear"}}}
		want := "https://api.bybit.com/v5/market/instruments-info?symbol=BTC+USDT&category=linear"
		if got := c.URL(); got != want {
			t.Fatalf("URL() = %q, want %q", got, want)
		}
	})
	t.Run("no params, no question mark", func(t *testing.T) {
		c := PublicConfig{Scheme: "https", Host: "fapi.binance.com", Path: "/fapi/v1/exchangeInfo"}
		if got, want := c.URL(), "https://fapi.binance.com/fapi/v1/exchangeInfo"; got != want {
			t.Fatalf("URL() = %q, want %q", got, want)
		}
	})
}
