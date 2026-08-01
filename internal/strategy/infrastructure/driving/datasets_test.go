package driving_test

// The dataset and fetcher battery. The page fixtures are REAL venue
// bytes, captured live on 2026-08-01 from the endpoints the fetcher
// names — including Binance's jittered fundingTime 1672560000008, eight
// milliseconds past the grid instant, which is the observed fact the
// jitter tolerance exists for.
//
// Checklist shapes: the jitter boundary is exercised at EXACTLY the
// tolerance and one millisecond past it; the completeness refusals cover
// missing AND extra; the H1 refusal has the boundary instant itself.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

var fetchPerp = ids.VenueInstrument{
	Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
}

// Real Binance page bytes (2026-08-01), two rows trimmed off the live
// response; note fundingTime 1672560000008.
const binanceFundingPage = `[{"symbol":"BTCUSDT","fundingTime":1672531200000,"fundingRate":"0.00010000","markPrice":"","rateType":"Regular"},{"symbol":"BTCUSDT","fundingTime":1672560000008,"fundingRate":"0.00010000","markPrice":"","rateType":"Regular"},{"symbol":"BTCUSDT","fundingTime":1672588800000,"fundingRate":"0.00000691","markPrice":"","rateType":"Regular"}]`

// Real Bybit page bytes (2026-08-01): NEWEST FIRST, string timestamps.
const bybitFundingPage = `{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[{"symbol":"BTCUSDT","fundingRate":"0.0003643","fundingRateTimestamp":"1672560000000"},{"symbol":"BTCUSDT","fundingRate":"0.00019145","fundingRateTimestamp":"1672531200000"}]},"retExtInfo":{},"time":1785563041509}`

// Real Binance spot kline page bytes (2026-08-01), two rows.
const binanceSpotKlinePage = `[[1672531200000,"16541.77000000","16545.70000000","16508.39000000","16529.67000000","4364.83570000",1672534799999,"72146293.58672290",149854,"2179.94772000","36032352.87035720","0"],[1672534800000,"16529.59000000","16556.80000000","16525.78000000","16551.47000000","3590.06669000",1672538399999,"59376757.34460920",126556,"1730.24901000","28617416.00486430","0"]]`

func datasetOf(series, venue, start, end, interval string, pages ...string) driving.DatasetFile {
	raw := make([]json.RawMessage, len(pages))
	for i, p := range pages {
		raw[i] = json.RawMessage(p)
	}
	return driving.DatasetFile{
		Venue: venue, Series: series, Symbol: "BTCUSDT",
		Start: start, End: end, Interval: interval,
		FetchedAt: "2026-08-01T05:00:00Z",
		Endpoint:  "fixture",
		Pages:     raw,
	}
}

func TestFundingSettlementsBinanceJitterNormalized(t *testing.T) {
	f := datasetOf(driving.SeriesFunding, "binance",
		"2023-01-01T00:00:00Z", "2023-01-01T16:00:00Z", "8h0m0s", binanceFundingPage)
	got, err := driving.FundingSettlements(f, fetchPerp)
	if err != nil {
		t.Fatalf("FundingSettlements: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("%d settlements; want 3", len(got))
	}
	// The jittered 08:00:00.008 row lands ON the grid instant, and the
	// venue's trailing-zero decimal parses to the canonical scale.
	if !got[1].At.Equal(time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("row 1 at %s; want the normalized 08:00 grid instant", got[1].At)
	}
	if got[1].Rate.Decimal() != "0.0001" {
		t.Errorf("row 1 rate = %s; want 0.0001", got[1].Rate.Decimal())
	}
	if got[2].Rate.Decimal() != "0.00000691" {
		t.Errorf("row 2 rate = %s; want 0.00000691", got[2].Rate.Decimal())
	}
}

func TestFundingSettlementsBybitDescendingSorted(t *testing.T) {
	f := datasetOf(driving.SeriesFunding, "bybit",
		"2023-01-01T00:00:00Z", "2023-01-01T08:00:00Z", "8h0m0s", bybitFundingPage)
	got, err := driving.FundingSettlements(f, ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	})
	if err != nil {
		t.Fatalf("FundingSettlements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d settlements; want 2", len(got))
	}
	// The venue's newest-first page reads back ascending.
	if !got[0].At.Before(got[1].At) {
		t.Errorf("rows not ascending: %s, %s", got[0].At, got[1].At)
	}
	if got[0].Rate.Decimal() != "0.00019145" {
		t.Errorf("row 0 rate = %s; want 0.00019145 (the OLDER row)", got[0].Rate.Decimal())
	}
}

func TestKlinePointsBinance(t *testing.T) {
	f := datasetOf(driving.SeriesSpotKline1h, "binance",
		"2023-01-01T00:00:00Z", "2023-01-01T01:00:00Z", "1h0m0s", binanceSpotKlinePage)
	got, err := driving.KlinePoints(f, "BTC", "USDT")
	if err != nil {
		t.Fatalf("KlinePoints: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d points; want 2", len(got))
	}
	// The OPEN price at the OPEN instant — the causal convention.
	if got[0].Price.Decimal() != "16541.77" {
		t.Errorf("point 0 price = %s; want 16541.77", got[0].Price.Decimal())
	}
	if got[1].Price.Decimal() != "16529.59" {
		t.Errorf("point 1 price = %s; want 16529.59", got[1].Price.Decimal())
	}
}

// jitterPage builds a one-row binance funding page at the given ms.
func jitterPage(ms int64) string {
	return fmt.Sprintf(`[{"symbol":"BTCUSDT","fundingTime":%d,"fundingRate":"0.0001"}]`, ms)
}

func TestJitterToleranceBoundary(t *testing.T) {
	grid := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	// Exactly the tolerance away: still the grid point's row.
	f := datasetOf(driving.SeriesFunding, "binance",
		"2023-01-01T00:00:00Z", "2023-01-01T00:00:00Z", "8h0m0s",
		jitterPage(grid+driving.GridJitterTolerance.Milliseconds()))
	if _, err := driving.FundingSettlements(f, fetchPerp); err != nil {
		t.Errorf("row exactly at tolerance refused: %v", err)
	}

	// One millisecond past it: off the grid, refused by name.
	f = datasetOf(driving.SeriesFunding, "binance",
		"2023-01-01T00:00:00Z", "2023-01-01T00:00:00Z", "8h0m0s",
		jitterPage(grid+driving.GridJitterTolerance.Milliseconds()+1))
	if _, err := driving.FundingSettlements(f, fetchPerp); !errors.Is(err, driving.ErrRowOffGrid) {
		t.Errorf("row past tolerance error = %v; want ErrRowOffGrid", err)
	}
}

func TestDatasetCompletenessRefusals(t *testing.T) {
	start, end := "2023-01-01T00:00:00Z", "2023-01-01T16:00:00Z"

	// Missing the 08:00 row.
	gap := `[{"symbol":"BTCUSDT","fundingTime":1672531200000,"fundingRate":"0.0001"},{"symbol":"BTCUSDT","fundingTime":1672588800000,"fundingRate":"0.0001"}]`
	f := datasetOf(driving.SeriesFunding, "binance", start, end, "8h0m0s", gap)
	if err := driving.ValidateDataset(f); !errors.Is(err, driving.ErrDatasetGap) {
		t.Errorf("gap error = %v; want ErrDatasetGap", err)
	}

	// A row past the declared end: extra, not absorbed.
	extra := binanceFundingPage[:len(binanceFundingPage)-1] + `,{"symbol":"BTCUSDT","fundingTime":1672617600000,"fundingRate":"0.0001"}]`
	f = datasetOf(driving.SeriesFunding, "binance", start, end, "8h0m0s", extra)
	if err := driving.ValidateDataset(f); !errors.Is(err, driving.ErrDatasetExtra) {
		t.Errorf("extra error = %v; want ErrDatasetExtra", err)
	}

	// Two rows claiming one grid instant.
	dup := `[{"symbol":"BTCUSDT","fundingTime":1672531200000,"fundingRate":"0.0001"},{"symbol":"BTCUSDT","fundingTime":1672531200008,"fundingRate":"0.0002"}]`
	f = datasetOf(driving.SeriesFunding, "binance", start, "2023-01-01T00:00:00Z", "8h0m0s", dup)
	if err := driving.ValidateDataset(f); !errors.Is(err, driving.ErrRowDuplicate) {
		t.Errorf("duplicate error = %v; want ErrRowDuplicate", err)
	}

	// A page that no longer parses.
	f = datasetOf(driving.SeriesFunding, "binance", start, end, "8h0m0s", `{"not":"an array"}`)
	if err := driving.ValidateDataset(f); !errors.Is(err, driving.ErrPageMalformed) {
		t.Errorf("malformed error = %v; want ErrPageMalformed", err)
	}
}

func TestDatasetH1BoundaryRowRefused(t *testing.T) {
	// The boundary instant ITSELF is holdout — the §41i suspect: it must
	// appear and be refused, or < and <= are the same predicate.
	boundaryMs := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	f := datasetOf(driving.SeriesFunding, "binance",
		"2026-01-31T16:00:00Z", "2026-02-01T00:00:00Z", "8h0m0s",
		fmt.Sprintf(`[{"symbol":"BTCUSDT","fundingTime":%d,"fundingRate":"0.0001"},{"symbol":"BTCUSDT","fundingTime":%d,"fundingRate":"0.0001"}]`,
			time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC).UnixMilli(), boundaryMs))
	if err := driving.ValidateDataset(f); !errors.Is(err, driving.ErrHoldoutEra) {
		t.Errorf("boundary row error = %v; want ErrHoldoutEra", err)
	}
}

func TestFetchPagination(t *testing.T) {
	// Two binance pages: the cursor must advance past page one's last
	// row, and both bodies must be stored VERBATIM.
	page1 := `[{"symbol":"BTCUSDT","fundingTime":1672531200000,"fundingRate":"0.0001"},{"symbol":"BTCUSDT","fundingTime":1672560000008,"fundingRate":"0.0001"}]`
	page2 := `[{"symbol":"BTCUSDT","fundingTime":1672588800000,"fundingRate":"0.00000691"}]`
	var urls []string
	fetcher := driving.Fetcher{Get: func(url string) ([]byte, error) {
		urls = append(urls, url)
		if len(urls) == 1 {
			return []byte(page1), nil
		}
		return []byte(page2), nil
	}}
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 1, 1, 16, 0, 0, 0, time.UTC)
	f, err := fetcher.Fetch("binance", driving.SeriesFunding, "BTCUSDT", start, end, time.Date(2026, 8, 1, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("%d requests; want 2", len(urls))
	}
	// The second cursor starts one millisecond past the jittered last row.
	wantCursor := fmt.Sprintf("startTime=%d", 1672560000009)
	if !strings.Contains(urls[1], wantCursor) {
		t.Errorf("second request %q; want cursor %s", urls[1], wantCursor)
	}
	if string(f.Pages[0]) != page1 || string(f.Pages[1]) != page2 {
		t.Error("pages not stored verbatim")
	}
}

func TestFetchStallAndUnsupported(t *testing.T) {
	// A venue that keeps answering the same page must be refused, not
	// looped on.
	same := `[{"symbol":"BTCUSDT","fundingTime":1672531200000,"fundingRate":"0.0001"}]`
	fetcher := driving.Fetcher{Get: func(string) ([]byte, error) { return []byte(same), nil }}
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
	if _, err := fetcher.Fetch("binance", driving.SeriesFunding, "BTCUSDT", start, end, end); !errors.Is(err, driving.ErrFetchStalled) {
		t.Errorf("stall error = %v; want ErrFetchStalled", err)
	}

	// Bybit klines have no derivation consumer this milestone.
	if _, err := fetcher.Fetch("bybit", driving.SeriesSpotKline1h, "BTCUSDT", start, end, end); !errors.Is(err, driving.ErrSeriesUnsupported) {
		t.Errorf("unsupported error = %v; want ErrSeriesUnsupported", err)
	}

	// A holdout-era identity refuses before one byte moves.
	called := false
	guard := driving.Fetcher{Get: func(string) ([]byte, error) { called = true; return nil, nil }}
	h1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := guard.Fetch("binance", driving.SeriesFunding, "BTCUSDT", start, h1, h1); !errors.Is(err, driving.ErrHoldoutEra) {
		t.Errorf("holdout fetch error = %v; want ErrHoldoutEra", err)
	}
	if called {
		t.Error("holdout-era fetch touched the transport")
	}
}

func TestWriteDatasetFileWriteOnceAndDigest(t *testing.T) {
	dir := t.TempDir()
	f := datasetOf(driving.SeriesFunding, "binance",
		"2023-01-01T00:00:00Z", "2023-01-01T16:00:00Z", "8h0m0s", binanceFundingPage)
	path, err := driving.DatasetPath(dir, f)
	if err != nil {
		t.Fatalf("DatasetPath: %v", err)
	}
	if want := filepath.Join(dir, "binance", "funding.BTCUSDT.2023-01-01.2023-01-01.json"); path != want {
		t.Errorf("path = %s; want %s", path, want)
	}
	digest, err := driving.WriteDatasetFile(path, f)
	if err != nil {
		t.Fatalf("WriteDatasetFile: %v", err)
	}

	// The digest is over the exact bytes on disk, and the loader re-derives it.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := driving.CheckDatasetDigest(digest, data); err != nil {
		t.Errorf("digest does not match the bytes on disk: %v", err)
	}
	_, loaded, err := driving.LoadDatasetFile(path)
	if err != nil {
		t.Fatalf("LoadDatasetFile: %v", err)
	}
	if loaded != digest {
		t.Errorf("loader digest %s; writer said %s", loaded, digest)
	}

	// Write-once: the same path refuses, whatever the content.
	if _, err := driving.WriteDatasetFile(path, f); !errors.Is(err, driving.ErrArtifactExists) {
		t.Errorf("overwrite error = %v; want ErrArtifactExists", err)
	}
}
