package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

const (
	cliBinanceSpotBody = `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01000000"},{"filterType":"LOT_SIZE","minQty":"0.00001000","stepSize":"0.00001000"},{"filterType":"MARKET_LOT_SIZE","minQty":"0.00000000","stepSize":"0.00000000"},{"filterType":"NOTIONAL","minNotional":"5.00000000"}]}]}`
	cliBybitLinearBody = `{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[{"symbol":"BTCUSDT","contractType":"LinearPerpetual","status":"Trading","baseCoin":"BTC","quoteCoin":"USDT","priceFilter":{"tickSize":"0.10"},"lotSizeFilter":{"minOrderQty":"0.001","qtyStep":"0.001","minNotionalValue":"5"}}]}}`
)

type cliFakeLotsClient struct {
	url    string
	result transport.PollResult
	err    error
}

func (f *cliFakeLotsClient) Fetch(context.Context) (transport.PollResult, error) {
	return f.result, f.err
}

func (f *cliFakeLotsClient) URL() string { return f.url }

func cliReferenceDocument() driven.Document {
	const observed = "2026-08-01T00:00:00Z"
	return driven.Document{
		Fees: []driven.FeeRow{
			{Venue: "binance", Product: "spot", Maker: "0.001", Taker: "0.0011", Tier: "VIP0", Class: "hand_recorded", ObservedAt: observed, Source: "fee source"},
		},
		Lots: []driven.LotRow{
			{Venue: "binance", Product: "spot", Symbol: "BTCUSDT", Base: "BTC", Step: "0.9", MinQty: "0.8", Quote: "USDT", MinNotional: "7", Tick: "0.6", Class: "hand_recorded", ObservedAt: observed, Source: "old lots one"},
			{Venue: "bybit", Product: "linear_perp", Contract: "linear", Symbol: "BTCUSDT", Base: "BTC", Step: "0.5", MinQty: "0.4", Quote: "USDT", MinNotional: "3", Tick: "0.2", Class: "hand_recorded", ObservedAt: observed, Source: "old lots two"},
		},
		Hurdle: driven.Hurdle{AnnualRate: "0.046", AccessCost: "0.001", LiquidityHaircut: "0.002", CounterpartyHaircut: "0.003", AsOf: "2026-07-30T00:00:00Z", Class: "hand_recorded", ObservedAt: observed, Source: "hurdle source"},
	}
}

func cliSuccessfulClients() map[driven.FetchCell]driven.FetchClient {
	return map[driven.FetchCell]driven.FetchClient{
		{Venue: "binance", Product: "spot"}: &cliFakeLotsClient{
			url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(cliBinanceSpotBody)},
		},
		{Venue: "bybit", Product: "linear_perp"}: &cliFakeLotsClient{
			url: "https://fixture.test/bybit/linear", result: transport.PollResult{Status: 200, Body: []byte(cliBybitLinearBody)},
		},
	}
}

func writeCLIReferenceInput(t *testing.T, dir string) string {
	t.Helper()
	body, err := json.Marshal(cliReferenceDocument())
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	path := filepath.Join(dir, "input.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	return path
}

func cliFixedClock(t *testing.T) clock.Clock {
	t.Helper()
	clk, err := clock.NewTestClock(time.Date(2026, 8, 5, 12, 34, 56, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("clock: %v", err)
	}
	return clk
}

func TestReferenceDispatchRoutesAndRequiresBothPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := dispatch([]string{"reference", "fetch"}, &stdout, &stderr); got != 64 {
		t.Fatalf("dispatch(reference fetch) = %d, want 64", got)
	}
	if !strings.Contains(stderr.String(), "-reference and -out are required") {
		t.Fatalf("stderr = %q, want both required flags named", stderr.String())
	}

	stderr.Reset()
	if got := dispatch([]string{"reference", "nonesuch"}, &stdout, &stderr); got != 64 {
		t.Fatalf("dispatch(reference nonesuch) = %d, want 64", got)
	}
	if !strings.Contains(stderr.String(), "arblab reference fetch") {
		t.Fatalf("usage does not advertise reference fetch: %q", stderr.String())
	}
}

func TestReferenceEndpointMappingsAreExact(t *testing.T) {
	tests := []struct {
		row     driven.LotRow
		wantURL string
	}{
		{row: driven.LotRow{Venue: "binance", Product: "spot", Symbol: "BTCUSDT"}, wantURL: "https://api.binance.com/api/v3/exchangeInfo?symbol=BTCUSDT"},
		{row: driven.LotRow{Venue: "binance", Product: "linear_perp", Symbol: "BTCUSDT"}, wantURL: "https://fapi.binance.com/fapi/v1/exchangeInfo"},
		{row: driven.LotRow{Venue: "bybit", Product: "spot", Symbol: "BTCUSDT"}, wantURL: "https://api.bybit.com/v5/market/instruments-info?category=spot&symbol=BTCUSDT"},
		{row: driven.LotRow{Venue: "bybit", Product: "linear_perp", Symbol: "BTCUSDT"}, wantURL: "https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=BTCUSDT"},
	}
	for _, tc := range tests {
		t.Run(tc.row.Venue+"/"+tc.row.Product, func(t *testing.T) {
			cfg, ok := referencePublicConfig(tc.row)
			if !ok {
				t.Fatalf("mapping absent")
			}
			if got := cfg.URL(); got != tc.wantURL {
				t.Fatalf("URL = %q, want %q", got, tc.wantURL)
			}
			if cfg.Timeout != 10*time.Second || cfg.ReadLimit != 4<<20 {
				t.Fatalf("bounds = %s/%d, want 10s/%d", cfg.Timeout, cfg.ReadLimit, 4<<20)
			}
		})
	}
	if _, ok := referencePublicConfig(driven.LotRow{Venue: "unknown", Product: "spot", Symbol: "BTCUSDT"}); ok {
		t.Fatalf("unknown venue unexpectedly has an endpoint mapping")
	}
}

func TestFetchReferenceFileFailureMatrixLeavesNoOutput(t *testing.T) {
	injectedTransport := errors.New("injected transport failure")
	tests := []struct {
		name   string
		mutate func(map[driven.FetchCell]driven.FetchClient)
		want   error
	}{
		{
			name: "second fetch transport error",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				clients[driven.FetchCell{Venue: "bybit", Product: "linear_perp"}] = &cliFakeLotsClient{url: "https://fixture.test/bybit/linear", err: injectedTransport}
			},
			want: injectedTransport,
		},
		{
			name: "429 with schema-valid body",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				body := strings.Replace(cliBinanceSpotBody, `"quoteAsset":"USDT"`, `"quoteAsset":"WRONG"`, 1)
				clients[driven.FetchCell{Venue: "binance", Product: "spot"}] = &cliFakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 429, Body: []byte(body)}}
			},
			want: driven.ErrFetchStatus,
		},
		{
			name: "wrong symbol",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				body := strings.Replace(cliBinanceSpotBody, `"symbol":"BTCUSDT"`, `"symbol":"ETHUSDT"`, 1)
				clients[driven.FetchCell{Venue: "binance", Product: "spot"}] = &cliFakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: driven.ErrFetchSymbolAbsent,
		},
		{
			name: "wrong quote",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				body := strings.Replace(cliBinanceSpotBody, `"quoteAsset":"USDT"`, `"quoteAsset":"USDC"`, 1)
				clients[driven.FetchCell{Venue: "binance", Product: "spot"}] = &cliFakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: driven.ErrFetchIdentityMismatch,
		},
		{
			name: "suspended status",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				body := strings.Replace(cliBinanceSpotBody, `"status":"TRADING"`, `"status":"BREAK"`, 1)
				clients[driven.FetchCell{Venue: "binance", Product: "spot"}] = &cliFakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: driven.ErrFetchNotTrading,
		},
		{
			name: "wrong contract type",
			mutate: func(clients map[driven.FetchCell]driven.FetchClient) {
				body := strings.Replace(cliBybitLinearBody, `"contractType":"LinearPerpetual"`, `"contractType":"InversePerpetual"`, 1)
				clients[driven.FetchCell{Venue: "bybit", Product: "linear_perp"}] = &cliFakeLotsClient{url: "https://fixture.test/bybit/linear", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: driven.ErrFetchIdentityMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := writeCLIReferenceInput(t, dir)
			output := filepath.Join(dir, "output.json")
			clients := cliSuccessfulClients()
			tc.mutate(clients)
			builder := func(driven.Document) (map[driven.FetchCell]driven.FetchClient, error) {
				return clients, nil
			}
			err := fetchReferenceFile(context.Background(), input, output, cliFixedClock(t), builder)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tc.want)
			}
			if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("output exists after failure or stat failed unexpectedly: %v", statErr)
			}
		})
	}
}

func TestFetchReferenceFileWritesANewValidatedRevision(t *testing.T) {
	dir := t.TempDir()
	input := writeCLIReferenceInput(t, dir)
	output := filepath.Join(dir, "output.json")
	builder := func(driven.Document) (map[driven.FetchCell]driven.FetchClient, error) {
		return cliSuccessfulClients(), nil
	}
	if err := fetchReferenceFile(context.Background(), input, output, cliFixedClock(t), builder); err != nil {
		t.Fatalf("fetchReferenceFile: %v", err)
	}
	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var got driven.Document
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("written document is invalid: %v", err)
	}
	for i, row := range got.Lots {
		if row.Class != "fetched" || row.ObservedAt != "2026-08-05T12:34:56Z" || !strings.HasPrefix(row.Source, "https://fixture.test/") {
			t.Fatalf("lots[%d] provenance = class %q observed %q source %q", i, row.Class, row.ObservedAt, row.Source)
		}
	}
}

func TestFetchReferenceFileRefusesAnUnmappedDocumentRowWithoutOutput(t *testing.T) {
	dir := t.TempDir()
	doc := cliReferenceDocument()
	doc.Lots[0].Venue = "unmapped"
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	input := filepath.Join(dir, "input.json")
	if err := os.WriteFile(input, body, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	output := filepath.Join(dir, "output.json")
	err = fetchReferenceFile(context.Background(), input, output, cliFixedClock(t), buildReferenceClients)
	if !errors.Is(err, errReferenceEndpoint) {
		t.Fatalf("error = %v, want errReferenceEndpoint", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output exists after unmapped-row failure or stat failed unexpectedly: %v", statErr)
	}
}

func TestFetchReferenceFileRefusesAnExistingTargetWithoutChangingIt(t *testing.T) {
	dir := t.TempDir()
	input := writeCLIReferenceInput(t, dir)
	output := filepath.Join(dir, "output.json")
	const existing = "existing target identity\n"
	if err := os.WriteFile(output, []byte(existing), 0o600); err != nil {
		t.Fatalf("write existing target: %v", err)
	}
	builder := func(driven.Document) (map[driven.FetchCell]driven.FetchClient, error) {
		return cliSuccessfulClients(), nil
	}
	err := fetchReferenceFile(context.Background(), input, output, cliFixedClock(t), builder)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("error = %v, want os.ErrExist from O_EXCL", err)
	}
	body, readErr := os.ReadFile(output)
	if readErr != nil {
		t.Fatalf("read existing target: %v", readErr)
	}
	if string(body) != existing {
		t.Fatalf("existing target changed to %q", body)
	}
}
