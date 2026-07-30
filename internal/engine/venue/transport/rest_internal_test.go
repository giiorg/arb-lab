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

// pollerFor points a validated Poller at a running httptest server. The
// server's URL is "http://host:port" — split back into the config's fields so
// the test goes through the same URL assembly production does.
func pollerFor(t *testing.T, srv *httptest.Server, path string, limit int64) *Poller {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "http://")
	p, err := NewPoller(PollerConfig{
		Scheme: "http", Host: host, Path: path,
		Timeout: 5 * time.Second, ReadLimit: limit,
	})
	if err != nil {
		t.Fatalf("NewPoller: %v", err)
	}
	return p
}

func TestPollerFetchReturnsStatusAndBodyVerbatim(t *testing.T) {
	const body = `[{"symbol":"BTCUSDT","fundingIntervalHours":8,"updateTime":null}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/fapi/v1/fundingInfo" {
			t.Errorf("venue saw %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	res, err := pollerFor(t, srv, "/fapi/v1/fundingInfo", 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("status %d, want 200", res.Status)
	}
	if string(res.Body) != body {
		t.Fatalf("body %q, want %q", res.Body, body)
	}
}

// A non-2xx is a result: the caller archives what the venue answered, and the
// venue's own words ride the body.
func TestPollerFetchNon2xxIsAResultNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":-1003,"msg":"Too many requests."}`))
	}))
	defer srv.Close()

	res, err := pollerFor(t, srv, "/x", 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Status != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", res.Status)
	}
	if !strings.Contains(string(res.Body), "-1003") {
		t.Fatalf("body %q lost the venue's words", res.Body)
	}
}

// The boundary where > and >= part ways: a body of EXACTLY ReadLimit bytes is
// legal, one more byte is the refusal. Both sides pinned, same server.
func TestPollerFetchBodyLimitBoundary(t *testing.T) {
	body := strings.Repeat("x", 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Run("exactly at the limit", func(t *testing.T) {
		res, err := pollerFor(t, srv, "/x", 64).Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch at limit: %v", err)
		}
		if len(res.Body) != 64 {
			t.Fatalf("body %d bytes, want 64", len(res.Body))
		}
	})
	t.Run("one byte over", func(t *testing.T) {
		_, err := pollerFor(t, srv, "/x", 63).Fetch(context.Background())
		if !errors.Is(err, ErrBodyTooLarge) {
			t.Fatalf("Fetch over limit = %v, want %v", err, ErrBodyTooLarge)
		}
	})
}

func TestPollerFetchTransportFailureIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	p := pollerFor(t, srv, "/x", 1<<20)
	srv.Close() // the venue is gone before the ask

	if _, err := p.Fetch(context.Background()); err == nil {
		t.Fatal("Fetch against a closed server returned nil error")
	}
}

func TestPollerFetchHonoursCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pollerFor(t, srv, "/x", 1<<20).Fetch(ctx); err == nil {
		t.Fatal("Fetch with a cancelled context returned nil error")
	}
}

func TestPollerConfigValidate(t *testing.T) {
	ok := PollerConfig{Scheme: "https", Host: "fapi.binance.com",
		Path: "/fapi/v1/fundingInfo", Timeout: 10 * time.Second, ReadLimit: 1 << 20}

	cases := []struct {
		name string
		mut  func(c *PollerConfig)
		want error
	}{
		{"the real target validates", func(c *PollerConfig) {}, nil},
		{"http is legal for tests", func(c *PollerConfig) { c.Scheme = "http" }, nil},
		{"scheme unset", func(c *PollerConfig) { c.Scheme = "" }, ErrSchemeUnset},
		{"ws is a socket scheme", func(c *PollerConfig) { c.Scheme = "wss" }, ErrRestScheme},
		{"host unset", func(c *PollerConfig) { c.Host = "" }, ErrHostUnset},
		{"host carrying a scheme", func(c *PollerConfig) { c.Host = "https://fapi.binance.com" }, ErrHostMalformed},
		{"host carrying a path", func(c *PollerConfig) { c.Host = "fapi.binance.com/fapi" }, ErrHostMalformed},
		{"path without leading slash", func(c *PollerConfig) { c.Path = "fapi/v1/fundingInfo" }, ErrPathMalformed},
		{"path with trailing slash", func(c *PollerConfig) { c.Path = "/fapi/" }, ErrPathMalformed},
		{"path unset", func(c *PollerConfig) { c.Path = "" }, ErrPathMalformed},
		{"path with a query", func(c *PollerConfig) { c.Path = "/fapi?x=1" }, ErrPathMalformed},
		{"path with a fragment", func(c *PollerConfig) { c.Path = "/fapi#frag" }, ErrPathMalformed},
		{"path with a space", func(c *PollerConfig) { c.Path = "/fapi/v1/funding Info" }, ErrPathMalformed},
		{"host with a space", func(c *PollerConfig) { c.Host = "fapi.binance .com" }, ErrHostMalformed},
		{"timeout zero", func(c *PollerConfig) { c.Timeout = 0 }, ErrTimeoutRange},
		{"read limit zero", func(c *PollerConfig) { c.ReadLimit = 0 }, ErrReadLimitRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ok
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

func TestPollerConfigURL(t *testing.T) {
	c := PollerConfig{Scheme: "https", Host: "fapi.binance.com", Path: "/fapi/v1/fundingInfo"}
	if got, want := c.URL(), "https://fapi.binance.com/fapi/v1/fundingInfo"; got != want {
		t.Fatalf("URL() = %q, want %q", got, want)
	}
}
