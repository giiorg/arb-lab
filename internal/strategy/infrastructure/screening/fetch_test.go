package screening

// Walker tests run against canned pages with a SMALL page limit — the
// limit is a walker parameter precisely so a three-row fixture can
// exercise a multi-page walk instead of pretending one page is the
// mechanism (the fixture-too-small trap).

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var cutoff = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

func binancePage(rows ...[2]string) string {
	entries := make([]string, len(rows))
	for i, r := range rows {
		entries[i] = fmt.Sprintf(`{"symbol":"XUSDT","fundingTime":%s,"fundingRate":%q,"markPrice":""}`, r[0], r[1])
	}
	return "[" + strings.Join(entries, ",") + "]"
}

func bybitPage(rows ...[2]string) string {
	entries := make([]string, len(rows))
	for i, r := range rows {
		entries[i] = fmt.Sprintf(`{"symbol":"XUSDT","fundingRate":%q,"fundingRateTimestamp":%q}`, r[1], r[0])
	}
	return `{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[` + strings.Join(entries, ",") + `]}}`
}

// msAt is a grid instant in epoch milliseconds, as a decimal string.
func msAt(t time.Time) string { return fmt.Sprintf("%d", t.UnixMilli()) }

// The Binance walk advances its cursor one millisecond past each
// page's last row, keeps every page verbatim, and ends on an empty
// page.
func TestFetchBinanceFundingWalksAscending(t *testing.T) {
	g0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	g1 := g0.Add(4 * time.Hour)
	g2 := g0.Add(8 * time.Hour)
	var urls []string
	pages := []string{
		binancePage([2]string{msAt(g0), "0.0001"}, [2]string{msAt(g1), "0.0002"}),
		binancePage([2]string{msAt(g2), "0.0003"}),
		binancePage(),
	}
	tr := Transport{Get: func(url string) ([]byte, error) {
		urls = append(urls, url)
		if len(urls) > len(pages) {
			t.Fatalf("unexpected extra request %s", url)
		}
		return []byte(pages[len(urls)-1]), nil
	}}
	f, err := FetchBinanceFunding(tr, "XUSDT", cutoff, cutoff, 2)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(f.Pages) != 2 {
		t.Fatalf("pages %d, want 2 (the empty page is not stored)", len(f.Pages))
	}
	// The second request's cursor is one millisecond past g1.
	want := fmt.Sprintf("startTime=%d", g1.UnixMilli()+1)
	if !strings.Contains(urls[1], want) {
		t.Fatalf("second url %q, want cursor %s", urls[1], want)
	}
	rows, err := RowsFromRaw(f)
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(rows) != 3 || !rows[0].At.Equal(g0) || !rows[2].At.Equal(g2) {
		t.Fatalf("rows %v, want 3 ascending", rows)
	}
}

// A page that fails to advance the cursor is a stall, not a loop.
func TestFetchBinanceFundingStallRefuses(t *testing.T) {
	g0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	page := binancePage([2]string{msAt(g0), "0.0001"}, [2]string{msAt(g0.Add(4 * time.Hour)), "0.0002"})
	tr := Transport{Get: func(url string) ([]byte, error) { return []byte(page), nil }}
	_, err := FetchBinanceFunding(tr, "XUSDT", cutoff, cutoff, 2)
	if !errors.Is(err, ErrFetchStalled) {
		t.Fatalf("err %v, want ErrFetchStalled", err)
	}
}

// The Bybit walk pages newest-first by moving endTime below the oldest
// row seen; RowsFromRaw re-sorts ascending and collapses a boundary
// row served twice.
func TestFetchBybitFundingWalksBackwards(t *testing.T) {
	g0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	g1 := g0.Add(4 * time.Hour)
	g2 := g0.Add(8 * time.Hour)
	var urls []string
	pages := []string{
		bybitPage([2]string{msAt(g2), "0.0003"}, [2]string{msAt(g1), "0.0002"}),
		bybitPage([2]string{msAt(g0), "0.0001"}),
		bybitPage(),
	}
	tr := Transport{Get: func(url string) ([]byte, error) {
		urls = append(urls, url)
		if len(urls) > len(pages) {
			t.Fatalf("unexpected extra request %s", url)
		}
		return []byte(pages[len(urls)-1]), nil
	}}
	f, err := FetchBybitFunding(tr, "XUSDT", cutoff, cutoff, 2)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(f.Pages) != 2 {
		t.Fatalf("pages %d, want 2", len(f.Pages))
	}
	want := fmt.Sprintf("endTime=%d", g1.UnixMilli()-1)
	if !strings.Contains(urls[1], want) {
		t.Fatalf("second url %q, want %s", urls[1], want)
	}
	rows, err := RowsFromRaw(f)
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(rows) != 3 || !rows[0].At.Equal(g0) || !rows[2].At.Equal(g2) {
		t.Fatalf("rows %v, want 3 ascending", rows)
	}
}

// A Bybit envelope refusal is an error with the venue's words, not an
// empty page.
func TestFetchBybitEnvelopeRefusal(t *testing.T) {
	body := `{"retCode":10001,"retMsg":"params error","result":{}}`
	tr := Transport{Get: func(url string) ([]byte, error) { return []byte(body), nil }}
	_, err := FetchBybitFunding(tr, "XUSDT", cutoff, cutoff, 2)
	if !errors.Is(err, ErrVenueEnvelope) {
		t.Fatalf("err %v, want ErrVenueEnvelope", err)
	}
}

// RowsFromRaw drops rows outside [HistoryStart, cutoff): a row AT the
// cutoff instant is out (strictly-before), one just inside stays, and
// an exact duplicate collapses to one row.
func TestRowsFromRawFiltersWindowAndDuplicates(t *testing.T) {
	inside := cutoff.Add(-4 * time.Hour)
	f := RawFile{
		Venue: "binance", Kind: KindFunding, Symbol: "XUSDT",
		CutoffUTC: cutoff.Format(time.RFC3339),
		Pages: []json.RawMessage{
			json.RawMessage(binancePage(
				[2]string{msAt(inside), "0.0001"},
				[2]string{msAt(inside), "0.0001"},
				[2]string{msAt(cutoff), "0.0002"},
			)),
		},
	}
	rows, err := RowsFromRaw(f)
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(rows) != 1 || !rows[0].At.Equal(inside) {
		t.Fatalf("rows %v, want the single in-window row", rows)
	}
}
