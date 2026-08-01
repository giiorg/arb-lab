package driving

// The family-1 fetchers (§41j slice B): venue REST history → committed
// dataset files. The fetcher stores every response body VERBATIM and
// validates the assembled file before anything writes it — a fetch that
// cannot prove its closed identity (H3) or that touches the holdout era
// (H1) refuses instead of producing a file for an operator to trust.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Fetch sentinels.
var (
	// ErrFetchStalled reports a pagination cursor that stopped advancing
	// — the loop would otherwise refetch one page forever.
	ErrFetchStalled = fmt.Errorf("strategy/driving: pagination cursor stalled")

	// ErrTooManyPages is the runaway backstop, far above any real
	// fetch's page count.
	ErrTooManyPages = fmt.Errorf("strategy/driving: page count exceeds the fetch backstop")
)

// maxFetchPages bounds any single fetch: the largest real fetch (three
// years of 1h klines at 1000 rows a page) is under thirty pages.
const maxFetchPages = 10_000

// Fetcher fetches one closed dataset identity. Get is the transport seam
// — the CLI wires real HTTP with its politeness pause; the battery wires
// canned pages — so pagination, verbatim storage and the refusals are
// all testable without a network.
type Fetcher struct {
	Get func(url string) ([]byte, error)
}

// endpointFor names the venue's endpoint and paging dialect for a
// series.
func endpointFor(venue ids.Venue, series string) (base string, pageLimit int, err error) {
	switch {
	case venue == "binance" && series == SeriesFunding:
		return "https://fapi.binance.com/fapi/v1/fundingRate?symbol=%s", 1000, nil
	case venue == "binance" && series == SeriesSpotKline1h:
		return "https://api.binance.com/api/v3/klines?symbol=%s&interval=1h", 1000, nil
	case venue == "binance" && series == SeriesMarkKline1h:
		return "https://fapi.binance.com/fapi/v1/markPriceKlines?symbol=%s&interval=1h", 1000, nil
	case venue == "bybit" && series == SeriesFunding:
		return "https://api.bybit.com/v5/market/funding/history?category=linear&symbol=%s", 200, nil
	case venue == "bybit" && (series == SeriesSpotKline1h || series == SeriesMarkKline1h):
		// No derivation reads Bybit klines this milestone (§41k-2: the
		// seal fixes the scored venue to Binance; the basis and frame
		// quantities are scored-venue measurements). A fetch nobody
		// consumes is dead weight wearing provenance — a v2 fetch lands
		// WITH its consumer.
		return "", 0, fmt.Errorf("%s/%s: %w", venue, series, ErrSeriesUnsupported)
	}
	return "", 0, fmt.Errorf("%s/%s: %w", venue, series, ErrSeriesUnknown)
}

// Fetch pulls one closed identity: every page verbatim, pagination until
// the range is covered, the assembled file validated — H3 completeness
// against the declared grid, every row H1-guarded — before it is
// returned. fetchedAt is provenance only; it never enters a row.
func (f Fetcher) Fetch(venue ids.Venue, series, symbol string, start, end, fetchedAt time.Time) (DatasetFile, error) {
	if f.Get == nil {
		return DatasetFile{}, fmt.Errorf("fetcher transport unset: %w", ErrFetchStalled)
	}
	if symbol == "" {
		return DatasetFile{}, fmt.Errorf("fetch symbol empty: %w", ErrSeriesUnknown)
	}
	interval, err := SeriesInterval(series)
	if err != nil {
		return DatasetFile{}, err
	}
	grid := SettlementGrid{Start: start, End: end, Interval: interval}
	if err := grid.Validate(); err != nil {
		return DatasetFile{}, err
	}
	// H1 up front: a holdout-era identity refuses before one byte moves.
	if err := GuardCalibrationInput(end); err != nil {
		return DatasetFile{}, err
	}
	base, limit, err := endpointFor(venue, series)
	if err != nil {
		return DatasetFile{}, err
	}
	endpoint := fmt.Sprintf(base, symbol)

	var pages []json.RawMessage
	if venue == "bybit" {
		pages, err = f.pageByWindow(venue, series, endpoint, grid, limit)
	} else {
		pages, err = f.pageByCursor(venue, series, endpoint, grid, limit)
	}
	if err != nil {
		return DatasetFile{}, err
	}

	file := DatasetFile{
		Venue:     string(venue),
		Series:    series,
		Symbol:    symbol,
		Start:     start.UTC().Format(time.RFC3339),
		End:       end.UTC().Format(time.RFC3339),
		Interval:  interval.String(),
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
		Endpoint:  endpoint,
		Pages:     pages,
	}
	if err := ValidateDataset(file); err != nil {
		return DatasetFile{}, err
	}
	return file, nil
}

// pageByCursor is the Binance dialect: startTime advances past the last
// returned row, rows arrive ascending, a short or empty page ends the
// walk.
func (f Fetcher) pageByCursor(venue ids.Venue, series, endpoint string, g SettlementGrid, limit int) ([]json.RawMessage, error) {
	var pages []json.RawMessage
	cursor := g.Start
	for range maxFetchPages {
		url := fmt.Sprintf("%s&startTime=%d&endTime=%d&limit=%d",
			endpoint, cursor.UnixMilli(), g.End.UnixMilli()+GridJitterTolerance.Milliseconds(), limit)
		body, err := f.Get(url)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		rows, err := parsePage(string(venue), series, json.RawMessage(body))
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		if len(rows) == 0 {
			return pages, nil
		}
		pages = append(pages, json.RawMessage(body))
		last := rows[len(rows)-1].at
		if !last.Before(g.End) {
			return pages, nil
		}
		next := last.Add(time.Millisecond)
		if !next.After(cursor) {
			return nil, fmt.Errorf("cursor %s after page ending %s: %w",
				cursor.Format(time.RFC3339Nano), last.Format(time.RFC3339Nano), ErrFetchStalled)
		}
		cursor = next
	}
	return nil, ErrTooManyPages
}

// pageByWindow is the Bybit dialect: fixed windows sized to the page
// limit (a window of limit·interval holds exactly limit grid points), so
// no page can truncate silently.
func (f Fetcher) pageByWindow(venue ids.Venue, series, endpoint string, g SettlementGrid, limit int) ([]json.RawMessage, error) {
	var pages []json.RawMessage
	window := time.Duration(limit) * g.Interval
	cursor := g.Start
	for range maxFetchPages {
		if cursor.After(g.End) {
			return pages, nil
		}
		wEnd := cursor.Add(window - time.Millisecond)
		if wEnd.After(g.End) {
			wEnd = g.End.Add(GridJitterTolerance)
		}
		url := fmt.Sprintf("%s&startTime=%d&endTime=%d&limit=%d",
			endpoint, cursor.UnixMilli(), wEnd.UnixMilli(), limit)
		body, err := f.Get(url)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		rows, err := parsePage(string(venue), series, json.RawMessage(body))
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		if len(rows) > 0 {
			pages = append(pages, json.RawMessage(body))
		}
		cursor = cursor.Add(window)
	}
	return nil, ErrTooManyPages
}

// ValidateDataset proves a file IS its closed identity: pages parse,
// rows normalize onto the declared grid (H3 complete, nothing extra,
// nothing duplicated) and every row predates the H1 boundary.
func ValidateDataset(f DatasetFile) error {
	g, err := f.Grid()
	if err != nil {
		return err
	}
	rows, err := extractRows(f)
	if err != nil {
		return err
	}
	rows, err = normalizeRows(rows, g)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := GuardCalibrationInput(r.at); err != nil {
			return err
		}
	}
	return nil
}

// DatasetPath is the committed layout: datasets/<venue>/<series>.<symbol>.<start>.<end>.json,
// dates only — the identity's instants live inside the file.
func DatasetPath(root string, f DatasetFile) (string, error) {
	g, err := f.Grid()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s.%s.%s.%s.json",
		f.Series, f.Symbol, g.Start.UTC().Format("2006-01-02"), g.End.UTC().Format("2006-01-02"))
	return filepath.Join(root, f.Venue, name), nil
}

// WriteDatasetFile commits a validated file: write-once (an existing
// path refuses — a corrected dataset is a NEW file feeding a NEW
// artifact version), parent directory created, digest returned as the
// artifact's citation.
func WriteDatasetFile(path string, f DatasetFile) (digest string, err error) {
	if err := ValidateDataset(f); err != nil {
		return "", err
	}
	if err := GuardNewVersion(path, func(p string) bool {
		_, statErr := os.Stat(p)
		return statErr == nil
	}); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", fmt.Errorf("dataset encode: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("dataset dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("dataset write: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LoadDatasetFile reads a committed dataset and re-proves its identity.
func LoadDatasetFile(path string) (DatasetFile, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DatasetFile{}, "", fmt.Errorf("dataset read: %w", err)
	}
	var f DatasetFile
	if err := json.Unmarshal(data, &f); err != nil {
		return DatasetFile{}, "", fmt.Errorf("dataset decode: %w", err)
	}
	if err := ValidateDataset(f); err != nil {
		return DatasetFile{}, "", err
	}
	sum := sha256.Sum256(data)
	return f, hex.EncodeToString(sum[:]), nil
}
