package driven

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

func readFetchFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return body
}

// The fixture bodies are trimmed 2026-08-05 live probes.
func TestFetchDecodersMatchLiveProbeValues(t *testing.T) {
	tests := []struct {
		name   string
		body   []byte
		decode func([]byte, string) (fetchedLots, error)
		want   fetchedLots
	}{
		{
			name: "binance spot", body: readFetchFixture(t, "binance_spot.json"), decode: decodeBinanceSpotLots,
			want: fetchedLots{step: "0.00001000", minQty: "0.00001000", minNotional: "5.00000000", tick: "0.01000000", base: "BTC", quote: "USDT", status: "TRADING"},
		},
		{
			name: "binance perp", body: readFetchFixture(t, "binance_fapi.json"), decode: decodeBinancePerpLots,
			want: fetchedLots{step: "0.001", minQty: "0.001", minNotional: "50", tick: "0.10", base: "BTC", quote: "USDT", status: "TRADING", contractType: "PERPETUAL"},
		},
		{
			name: "bybit spot", body: readFetchFixture(t, "bybit_spot.json"), decode: decodeBybitSpotLots,
			want: fetchedLots{step: "0.000001", minQty: "0.000001", minNotional: "5", tick: "0.1", base: "BTC", quote: "USDT", status: "Trading"},
		},
		{
			name: "bybit linear", body: readFetchFixture(t, "bybit_linear.json"), decode: decodeBybitLinearLots,
			want: fetchedLots{step: "0.001", minQty: "0.001", minNotional: "5", tick: "0.10", base: "BTC", quote: "USDT", status: "Trading", contractType: "LinearPerpetual"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.decode(tc.body, "BTCUSDT")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got != tc.want {
				t.Fatalf("decoded lots = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The fixture body is a trimmed 2026-08-05 live probe.
func TestBinancePerpSelectsTheRequestedSymbolFromTheFullRoster(t *testing.T) {
	body := readFetchFixture(t, "binance_fapi.json")
	var roster struct {
		Symbols []struct {
			Symbol string `json:"symbol"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &roster); err != nil {
		t.Fatalf("decode fixture roster: %v", err)
	}
	if len(roster.Symbols) != 3 || roster.Symbols[1].Symbol != "BTCUSDT" {
		t.Fatalf("collapse control: fixture symbols = %+v, want exactly three with BTCUSDT in the middle", roster.Symbols)
	}

	tests := []struct {
		symbol string
		want   fetchedLots
	}{
		{symbol: "ETCUSDT", want: fetchedLots{step: "0.01", minQty: "0.01", minNotional: "20", tick: "0.001", base: "ETC", quote: "USDT", status: "TRADING", contractType: "PERPETUAL"}},
		{symbol: "BTCUSDT", want: fetchedLots{step: "0.001", minQty: "0.001", minNotional: "50", tick: "0.10", base: "BTC", quote: "USDT", status: "TRADING", contractType: "PERPETUAL"}},
		{symbol: "XRPUSDT", want: fetchedLots{step: "0.1", minQty: "0.1", minNotional: "5", tick: "0.0001", base: "XRP", quote: "USDT", status: "TRADING", contractType: "PERPETUAL"}},
	}
	for _, tc := range tests {
		t.Run(tc.symbol, func(t *testing.T) {
			got, err := decodeBinancePerpLots(body, tc.symbol)
			if err != nil {
				t.Fatalf("decode %s: %v", tc.symbol, err)
			}
			if got != tc.want {
				t.Fatalf("decode %s = %+v, want %+v", tc.symbol, got, tc.want)
			}
		})
	}
	if _, err := decodeBinancePerpLots(body, "WRONGUSDT"); !errors.Is(err, ErrFetchSymbolAbsent) {
		t.Fatalf("wrong symbol error = %v, want ErrFetchSymbolAbsent", err)
	}
}

// The fixture body is a trimmed 2026-08-05 live probe.
func TestBinanceSpotUsesLotSizeRatherThanMarketLotSize(t *testing.T) {
	got, err := decodeBinanceSpotLots(readFetchFixture(t, "binance_spot.json"), "BTCUSDT")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.step != "0.00001000" {
		t.Fatalf("step = %q, want LOT_SIZE 0.00001000; MARKET_LOT_SIZE is 0.00000000", got.step)
	}
}

// The fixture bodies are trimmed 2026-08-05 live probes.
func TestBybitDecodersRefuseTheOtherCategoryFields(t *testing.T) {
	tests := []struct {
		name   string
		body   []byte
		decode func([]byte, string) (fetchedLots, error)
	}{
		{name: "spot decoder on linear", body: readFetchFixture(t, "bybit_linear.json"), decode: decodeBybitSpotLots},
		{name: "linear decoder on spot", body: readFetchFixture(t, "bybit_spot.json"), decode: decodeBybitLinearLots},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.decode(tc.body, "BTCUSDT")
			if !errors.Is(err, ErrFetchFilterAbsent) {
				t.Fatalf("error = %v, want ErrFetchFilterAbsent", err)
			}
		})
	}
}

// The fixture body is a trimmed 2026-08-05 live probe.
func TestBybitDecoderRefusesANonzeroEnvelopeCode(t *testing.T) {
	body := strings.Replace(string(readFetchFixture(t, "bybit_spot.json")), `"retCode": 0`, `"retCode": 10001`, 1)
	_, err := decodeBybitSpotLots([]byte(body), "BTCUSDT")
	if !errors.Is(err, ErrFetchStatus) {
		t.Fatalf("error = %v, want ErrFetchStatus", err)
	}
}

type fakeLotsClient struct {
	url    string
	result transport.PollResult
	err    error
	calls  int
}

func (f *fakeLotsClient) Fetch(context.Context) (transport.PollResult, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeLotsClient) URL() string { return f.url }

func fetchInputDocument() Document {
	const observed = "2026-08-01T00:00:00Z"
	return Document{
		Fees: []FeeRow{
			{Venue: "binance", Product: "spot", Maker: "0.001", Taker: "0.0011", Tier: "VIP0", Class: "hand_recorded", ObservedAt: observed, Source: "fee source one"},
			{Venue: "bybit", Product: "linear_perp", Contract: "linear", Maker: "0.0002", Taker: "0.00055", Tier: "non-VIP", Class: "hand_recorded", ObservedAt: observed, Source: "fee source two"},
		},
		Lots: []LotRow{
			{Venue: "binance", Product: "spot", Symbol: "BTCUSDT", Base: "BTC", Step: "0.9", MinQty: "0.8", Quote: "USDT", MinNotional: "7", Tick: "0.6", Class: "hand_recorded", ObservedAt: observed, Source: "old lots one"},
			{Venue: "bybit", Product: "linear_perp", Contract: "linear", Symbol: "BTCUSDT", Base: "BTC", Step: "0.5", MinQty: "0.4", Quote: "USDT", MinNotional: "3", Tick: "0.2", Class: "hand_recorded", ObservedAt: observed, Source: "old lots two"},
		},
		Hurdle: Hurdle{AnnualRate: "0.046", AccessCost: "0.001", LiquidityHaircut: "0.002", CounterpartyHaircut: "0.003", AsOf: "2026-07-30T00:00:00Z", Class: "hand_recorded", ObservedAt: observed, Source: "hurdle source"},
	}
}

func successfulFetchClients(t *testing.T) map[FetchCell]FetchClient {
	t.Helper()
	return map[FetchCell]FetchClient{
		{Venue: "binance", Product: "spot"}: &fakeLotsClient{
			url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: readFetchFixture(t, "binance_spot.json")},
		},
		{Venue: "bybit", Product: "linear_perp"}: &fakeLotsClient{
			url: "https://fixture.test/bybit/linear", result: transport.PollResult{Status: 200, Body: readFetchFixture(t, "bybit_linear.json")},
		},
	}
}

// The fixture bodies are trimmed 2026-08-05 live probes.
func TestFetchLotsFailureMatrixReturnsNoPartialDocument(t *testing.T) {
	injectedTransport := errors.New("injected transport failure")
	tests := []struct {
		name   string
		mutate func(map[FetchCell]FetchClient)
		want   error
	}{
		{
			name: "second fetch transport error",
			mutate: func(clients map[FetchCell]FetchClient) {
				clients[FetchCell{Venue: "bybit", Product: "linear_perp"}] = &fakeLotsClient{url: "https://fixture.test/bybit/linear", err: injectedTransport}
			},
			want: injectedTransport,
		},
		{
			name: "429 precedes decode",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "binance_spot.json")), `"quoteAsset": "USDT"`, `"quoteAsset": "WRONG"`, 1)
				clients[FetchCell{Venue: "binance", Product: "spot"}] = &fakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 429, Body: []byte(body)}}
			},
			want: ErrFetchStatus,
		},
		{
			name: "wrong symbol",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "binance_spot.json")), `"symbol": "BTCUSDT"`, `"symbol": "ETHUSDT"`, 1)
				clients[FetchCell{Venue: "binance", Product: "spot"}] = &fakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: ErrFetchSymbolAbsent,
		},
		{
			name: "wrong base",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "binance_spot.json")), `"baseAsset": "BTC"`, `"baseAsset": "ETH"`, 1)
				clients[FetchCell{Venue: "binance", Product: "spot"}] = &fakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: ErrFetchIdentityMismatch,
		},
		{
			name: "wrong quote",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "binance_spot.json")), `"quoteAsset": "USDT"`, `"quoteAsset": "USDC"`, 1)
				clients[FetchCell{Venue: "binance", Product: "spot"}] = &fakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: ErrFetchIdentityMismatch,
		},
		{
			name: "suspended status",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "binance_spot.json")), `"status": "TRADING"`, `"status": "BREAK"`, 1)
				clients[FetchCell{Venue: "binance", Product: "spot"}] = &fakeLotsClient{url: "https://fixture.test/binance/spot", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: ErrFetchNotTrading,
		},
		{
			name: "wrong contract type",
			mutate: func(clients map[FetchCell]FetchClient) {
				body := strings.Replace(string(readFetchFixture(t, "bybit_linear.json")), `"contractType": "LinearPerpetual"`, `"contractType": "InversePerpetual"`, 1)
				clients[FetchCell{Venue: "bybit", Product: "linear_perp"}] = &fakeLotsClient{url: "https://fixture.test/bybit/linear", result: transport.PollResult{Status: 200, Body: []byte(body)}}
			},
			want: ErrFetchIdentityMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clients := successfulFetchClients(t)
			tc.mutate(clients)
			clk, err := clock.NewTestClock(time.Date(2026, 8, 5, 12, 34, 56, 0, time.UTC), nil)
			if err != nil {
				t.Fatalf("clock: %v", err)
			}
			got, err := FetchLots(context.Background(), clients, fetchInputDocument(), clk)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tc.want)
			}
			if !reflect.DeepEqual(got, Document{}) {
				t.Fatalf("failure returned a partial document: %+v", got)
			}
		})
	}
}

// The fixture bodies are trimmed 2026-08-05 live probes.
func TestFetchLotsCompleteDocumentGolden(t *testing.T) {
	clk, err := clock.NewTestClock(time.Date(2026, 8, 5, 12, 34, 56, 0, time.FixedZone("probe", 4*60*60)), nil)
	if err != nil {
		t.Fatalf("clock: %v", err)
	}
	got, err := FetchLots(context.Background(), successfulFetchClients(t), fetchInputDocument(), clk)
	if err != nil {
		t.Fatalf("FetchLots: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal fetched document: %v", err)
	}
	wantJSON, err := os.ReadFile("testdata/reference_fetch_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !reflect.DeepEqual(append(gotJSON, '\n'), wantJSON) {
		t.Fatalf("complete fetched document does not match golden\nGOT:\n%s\nWANT:\n%s", gotJSON, wantJSON)
	}
}
