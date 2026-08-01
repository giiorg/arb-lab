package crossvenue

// Seal + store battery. Fixture discipline: the seal fixture holds a row
// EXACTLY AT the cutoff — evidence begins AT the instant, so at-cutoff is
// sealed, and only that row distinguishes < from <=; the store-conflict
// fixture disagrees across two FILES, because a within-file duplicate is
// a different rule (the venue's own page dirt, RowsFromRaw's business).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/screening"
)

func sealRow(t *testing.T, at time.Time, r string) dcv.Settlement {
	t.Helper()
	rate, err := fixedpoint.ParseRate(r)
	if err != nil {
		t.Fatal(err)
	}
	return dcv.Settlement{At: at, Rate: rate}
}

// The one row AT the cutoff is the fixture that pins strictly-before.
func TestSealSettledAtCutoffExcluded(t *testing.T) {
	before := sealRow(t, SettledCutoff.Add(-time.Millisecond), "0.0001")
	at := sealRow(t, SettledCutoff, "0.0002")
	after := sealRow(t, SettledCutoff.Add(time.Hour), "0.0003")
	kept, excluded := sealSettled([]dcv.Settlement{before, at, after})
	if len(kept) != 1 || !kept[0].At.Equal(before.At) {
		t.Fatalf("kept %+v, want only the pre-cutoff row", kept)
	}
	if excluded != 2 {
		t.Fatalf("excluded %d, want 2 (the at-cutoff row is gate evidence)", excluded)
	}
}

// storeRaw plants one raw file with the given rows via the screening
// schema, exactly as FetchSettled would have written it — each venue's
// page in that venue's own dialect (bare array vs V5 envelope,
// newest-first for Bybit).
func storeRaw(t *testing.T, root, venueName, symbol, cutoffDay string, rows ...[2]string) {
	t.Helper()
	var page json.RawMessage
	if venueName == "bybit" {
		entries := make([]string, len(rows))
		for i, r := range rows {
			entries[len(rows)-1-i] = fmt.Sprintf(`{"symbol":%q,"fundingRate":%q,"fundingRateTimestamp":%q}`, symbol, r[1], r[0])
		}
		page = json.RawMessage(`{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[` + joinComma(entries) + `]}}`)
	} else {
		entries := make([]string, len(rows))
		for i, r := range rows {
			entries[i] = fmt.Sprintf(`{"symbol":%q,"fundingTime":%s,"fundingRate":%q,"markPrice":""}`, symbol, r[0], r[1])
		}
		page = json.RawMessage("[" + joinComma(entries) + "]")
	}
	f := screening.RawFile{
		Venue: venueName, Kind: screening.KindFunding, Symbol: symbol,
		Endpoint: "test", FetchedAt: "2026-08-01T00:00:00Z", CutoffUTC: cutoffDay + "T00:00:00Z",
		Pages: []json.RawMessage{page},
	}
	path := filepath.Join(root, venueName, "funding."+symbol+"."+cutoffDay+".json")
	if _, err := screening.WriteRaw(path, f); err != nil {
		t.Fatal(err)
	}
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func ms(t time.Time) string { return fmt.Sprintf("%d", t.UnixMilli()) }

// Two stored fetches overlap on one instant with the SAME rate — a
// dedup; the merged series is ascending and each file's provenance is
// returned.
func TestLoadSettledMergesAndDedups(t *testing.T) {
	root := t.TempDir()
	g0 := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	g1 := g0.Add(4 * time.Hour)
	g2 := g0.Add(8 * time.Hour)
	storeRaw(t, root, "binance", "ABCUSDT", "2026-07-25", [2]string{ms(g0), "0.0001"}, [2]string{ms(g1), "0.0002"})
	storeRaw(t, root, "binance", "ABCUSDT", "2026-07-26", [2]string{ms(g1), "0.0002"}, [2]string{ms(g2), "0.0003"})
	rows, files, err := LoadSettled(root, "binance", "ABCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows %d, want 3 (g1 deduplicated)", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if !rows[i].At.After(rows[i-1].At) {
			t.Fatalf("rows not ascending at %d", i)
		}
	}
	if len(files) != 2 || files[0].SHA256 == "" || files[0].Rows != 2 {
		t.Fatalf("files %+v", files)
	}
}

// The same instant with a DIFFERENT rate across two files is a different
// dataset — refused, never averaged or last-write-wins.
func TestLoadSettledConflictRefuses(t *testing.T) {
	root := t.TempDir()
	g0 := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	storeRaw(t, root, "binance", "ABCUSDT", "2026-07-25", [2]string{ms(g0), "0.0001"})
	storeRaw(t, root, "binance", "ABCUSDT", "2026-07-26", [2]string{ms(g0), "0.0009"})
	_, _, err := LoadSettled(root, "binance", "ABCUSDT")
	if !errors.Is(err, ErrStoreConflict) {
		t.Fatalf("err %v, want ErrStoreConflict", err)
	}
}

// An empty store refuses — a wrong root must not read as a market that
// never settled.
func TestLoadSettledEmptyRefuses(t *testing.T) {
	_, _, err := LoadSettled(t.TempDir(), "binance", "ABCUSDT")
	if !errors.Is(err, ErrStoreEmpty) {
		t.Fatalf("err %v, want ErrStoreEmpty", err)
	}
}

// FetchSettled routes by venue, stores verbatim, digests, and refuses a
// second fetch of the same identity (write-once).
func TestFetchSettledWriteOnce(t *testing.T) {
	root := t.TempDir()
	g0 := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	pages := []string{
		fmt.Sprintf(`[{"symbol":"ABCUSDT","fundingTime":%s,"fundingRate":"0.0001","markPrice":""}]`, ms(g0)),
		`[]`,
	}
	calls := 0
	tr := screening.Transport{Get: func(url string) ([]byte, error) {
		calls++
		return []byte(pages[calls-1]), nil
	}}
	path, digest, err := FetchSettled(tr, root, "binance", "ABCUSDT", cutoff, cutoff, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" || path == "" {
		t.Fatal("no provenance returned")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stored file: %v", err)
	}
	if _, _, err := FetchSettled(tr, root, "binance", "ABCUSDT", cutoff, cutoff, 1000); !errors.Is(err, ErrStoreExists) {
		t.Fatalf("second fetch: err %v, want ErrStoreExists", err)
	}
	if calls != len(pages) {
		t.Fatalf("transport called %d times, want %d — the resume path must cost no requests", calls, len(pages))
	}
	if _, _, err := FetchSettled(tr, root, "okx", "ABCUSDT", cutoff, cutoff, 1000); !errors.Is(err, ErrStoreVenueUnknown) {
		t.Fatalf("err %v, want ErrStoreVenueUnknown", err)
	}
}
