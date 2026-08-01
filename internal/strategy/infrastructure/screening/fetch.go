package screening

// The funding-history walkers, one per venue dialect (profiles §7).
// Neither assumes a grid: an alt's history starts at its listing and its
// interval may have changed over time, so the walk's only contract is
// "every settlement the venue serves strictly before the cutoff, no
// page silently skipped". Grid facts are MEASURED from the assembled
// rows at compute time (§42a), never declared here.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Fetch sentinels.
var (
	// ErrFetchStalled reports a pagination cursor that stopped
	// advancing — the loop would otherwise refetch one page forever.
	ErrFetchStalled = fmt.Errorf("screening: pagination cursor stalled")

	// ErrTooManyPages is the runaway backstop.
	ErrTooManyPages = fmt.Errorf("screening: page count exceeds the fetch backstop")

	// ErrVenueEnvelope reports a Bybit V5 envelope with a non-zero
	// retCode — the venue answered, and the answer is a refusal.
	ErrVenueEnvelope = fmt.Errorf("screening: venue envelope error")
)

// Transport is the one seam to the network: the CLI wires real HTTP
// with per-host politeness pacing; tests wire canned pages. Get MUST
// be safe for concurrent use — the harness walks the two venues in
// parallel, and pacing under concurrency is the transport's problem,
// not the walkers'.
type Transport struct {
	Get func(url string) ([]byte, error)
}

// Row is one settled funding print: the venue-published instant
// (jitter preserved) and the venue-published rate.
type Row struct {
	At   time.Time
	Rate fixedpoint.Rate
}

// FetchBinanceFunding walks Binance's ascending-cursor dialect: rows
// arrive oldest-first, the cursor advances one millisecond past the
// last row, and an empty page ends the walk. pageLimit is a parameter
// (BinancePageLimit in production) so tests can exercise multi-page
// walks with small fixtures.
func FetchBinanceFunding(t Transport, symbol string, cutoff, fetchedAt time.Time, pageLimit int) (RawFile, error) {
	f := RawFile{
		Venue:     "binance",
		Kind:      KindFunding,
		Symbol:    symbol,
		Endpoint:  "https://fapi.binance.com/fapi/v1/fundingRate?symbol=" + symbol,
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
		CutoffUTC: cutoff.UTC().Format(time.RFC3339),
	}
	cursor := HistoryStart
	for range MaxFetchPages {
		url := fmt.Sprintf("%s&startTime=%d&endTime=%d&limit=%d",
			f.Endpoint, cursor.UnixMilli(), cutoff.UnixMilli()-1, pageLimit)
		body, err := t.Get(url)
		if err != nil {
			return RawFile{}, fmt.Errorf("fetch %s: %w", url, err)
		}
		rows, err := parseBinanceFundingPage(body)
		if err != nil {
			return RawFile{}, fmt.Errorf("fetch %s: %w", url, err)
		}
		if len(rows) == 0 {
			return f, nil
		}
		f.Pages = append(f.Pages, json.RawMessage(body))
		last := rows[len(rows)-1].At
		next := last.Add(time.Millisecond)
		if !next.After(cursor) {
			return RawFile{}, fmt.Errorf("cursor %s after page ending %s: %w",
				cursor.Format(time.RFC3339Nano), last.Format(time.RFC3339Nano), ErrFetchStalled)
		}
		cursor = next
	}
	return RawFile{}, ErrTooManyPages
}

// FetchBybitFunding walks Bybit's newest-first dialect backwards: each
// page's endTime cursor moves one millisecond below the oldest row
// returned, and the walk ends on an empty page or when it reaches
// HistoryStart. Pages are stored in arrival (newest-first) order;
// RowsFromRaw re-sorts.
func FetchBybitFunding(t Transport, symbol string, cutoff, fetchedAt time.Time, pageLimit int) (RawFile, error) {
	f := RawFile{
		Venue:     "bybit",
		Kind:      KindFunding,
		Symbol:    symbol,
		Endpoint:  "https://api.bybit.com/v5/market/funding/history?category=linear&symbol=" + symbol,
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
		CutoffUTC: cutoff.UTC().Format(time.RFC3339),
	}
	endMS := cutoff.UnixMilli() - 1
	for range MaxFetchPages {
		url := fmt.Sprintf("%s&endTime=%d&limit=%d", f.Endpoint, endMS, pageLimit)
		body, err := t.Get(url)
		if err != nil {
			return RawFile{}, fmt.Errorf("fetch %s: %w", url, err)
		}
		rows, err := parseBybitFundingPage(body)
		if err != nil {
			return RawFile{}, fmt.Errorf("fetch %s: %w", url, err)
		}
		if len(rows) == 0 {
			return f, nil
		}
		f.Pages = append(f.Pages, json.RawMessage(body))
		oldest := rows[len(rows)-1].At // newest-first: the last row is the oldest
		if !oldest.After(HistoryStart) {
			return f, nil
		}
		next := oldest.UnixMilli() - 1
		if next >= endMS {
			return RawFile{}, fmt.Errorf("cursor %d after page ending %d: %w", endMS, next, ErrFetchStalled)
		}
		endMS = next
	}
	return RawFile{}, ErrTooManyPages
}

// parseBinanceFundingPage decodes one Binance fundingRate page: a JSON
// array, fundingTime in epoch milliseconds (with observed jitter),
// fundingRate a decimal string. markPrice is deliberately not read — it
// is empty on old rows and nothing here consumes it.
func parseBinanceFundingPage(body []byte) ([]Row, error) {
	var page []struct {
		Symbol      string `json:"symbol"`
		FundingTime int64  `json:"fundingTime"`
		FundingRate string `json:"fundingRate"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("binance funding page: %w", err)
	}
	rows := make([]Row, 0, len(page))
	for _, e := range page {
		rate, err := fixedpoint.ParseRate(e.FundingRate)
		if err != nil {
			return nil, fmt.Errorf("binance funding rate %q at %d: %w", e.FundingRate, e.FundingTime, err)
		}
		rows = append(rows, Row{At: time.UnixMilli(e.FundingTime).UTC(), Rate: rate})
	}
	return rows, nil
}

// parseBybitFundingPage decodes one Bybit V5 funding/history page:
// envelope checked, rows newest-first, fundingRateTimestamp a STRING of
// epoch milliseconds (profiles §7).
func parseBybitFundingPage(body []byte) ([]Row, error) {
	var env struct {
		RetCode int             `json:"retCode"`
		RetMsg  string          `json:"retMsg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("bybit funding envelope: %w", err)
	}
	if env.RetCode != 0 {
		return nil, fmt.Errorf("retCode %d %q: %w", env.RetCode, env.RetMsg, ErrVenueEnvelope)
	}
	var res struct {
		List []struct {
			Symbol               string `json:"symbol"`
			FundingRate          string `json:"fundingRate"`
			FundingRateTimestamp string `json:"fundingRateTimestamp"`
		} `json:"list"`
	}
	if err := json.Unmarshal(env.Result, &res); err != nil {
		return nil, fmt.Errorf("bybit funding result: %w", err)
	}
	rows := make([]Row, 0, len(res.List))
	for _, e := range res.List {
		ms, err := strconv.ParseInt(e.FundingRateTimestamp, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bybit funding timestamp %q: %w", e.FundingRateTimestamp, err)
		}
		rate, err := fixedpoint.ParseRate(e.FundingRate)
		if err != nil {
			return nil, fmt.Errorf("bybit funding rate %q at %d: %w", e.FundingRate, ms, err)
		}
		rows = append(rows, Row{At: time.UnixMilli(ms).UTC(), Rate: rate})
	}
	return rows, nil
}

// RowsFromRaw assembles a raw funding file into one clean series:
// every page parsed with the file's own venue dialect, concatenated,
// sorted ascending, exact-duplicate instants collapsed (a backwards
// walk may refetch a boundary row), and rows outside
// [HistoryStart, cutoff) dropped — the file keeps the venue's words,
// the series keeps the screening window.
func RowsFromRaw(f RawFile) ([]Row, error) {
	cutoff, err := time.Parse(time.RFC3339, f.CutoffUTC)
	if err != nil {
		return nil, fmt.Errorf("raw cutoff %q: %w", f.CutoffUTC, err)
	}
	parse := parseBinanceFundingPage
	if f.Venue == "bybit" {
		parse = parseBybitFundingPage
	}
	var rows []Row
	for i, p := range f.Pages {
		page, err := parse(p)
		if err != nil {
			return nil, fmt.Errorf("%s/%s page %d: %w", f.Venue, f.Symbol, i, err)
		}
		rows = append(rows, page...)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].At.Before(rows[j].At) })
	out := rows[:0]
	var prev time.Time
	for _, r := range rows {
		if r.At.Before(HistoryStart) || !r.At.Before(cutoff) {
			continue
		}
		if len(out) > 0 && r.At.Equal(prev) {
			continue
		}
		out = append(out, r)
		prev = r.At
	}
	return out, nil
}
