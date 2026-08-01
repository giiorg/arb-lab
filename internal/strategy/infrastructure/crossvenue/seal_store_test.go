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

// The gate seal is the exploration seal's mirror, and every boundary is
// pinned by a row sitting EXACTLY on it: at-cutoff is KEPT here (evidence
// begins AT the instant — the same fact that excludes it above), and the
// declared bound is strict, so a row at the bound belongs to the next
// run's universe, not this one's.
func TestGateSettledWindowBoundaries(t *testing.T) {
	bound := SettledCutoff.Add(48 * time.Hour)
	before := sealRow(t, SettledCutoff.Add(-time.Millisecond), "0.0001")
	at := sealRow(t, SettledCutoff, "0.0002")
	mid := sealRow(t, SettledCutoff.Add(time.Hour), "0.0003")
	atBound := sealRow(t, bound, "0.0004")
	past := sealRow(t, bound.Add(time.Hour), "0.0005")
	kept, excluded := gateSettled([]dcv.Settlement{before, at, mid, atBound, past}, bound)
	if len(kept) != 2 || !kept[0].At.Equal(at.At) || !kept[1].At.Equal(mid.At) {
		t.Fatalf("kept %+v, want the at-cutoff and mid rows", kept)
	}
	if excluded != 3 {
		t.Fatalf("excluded %d, want 3 (pre-cutoff, at-bound, past-bound)", excluded)
	}
}

// The gate door refuses a bound that does not reach past the books
// cutoff: such a call asks for gate evidence over a window holding none,
// and answering it with an empty walk would read as a market with no
// evidence rather than a misuse.
func TestRunGateSymbolsRefusesBoundAtCutoff(t *testing.T) {
	p := simParamsFixture(t)
	if _, _, err := RunGateSymbols("", "", p, BooksCutoff); !errors.Is(err, ErrPostCutoffSealed) {
		t.Fatalf("err %v, want ErrPostCutoffSealed at the cutoff itself", err)
	}
	if _, _, err := RunGateSymbols("", "", p, BooksCutoff.Add(-time.Hour)); !errors.Is(err, ErrPostCutoffSealed) {
		t.Fatalf("err %v, want ErrPostCutoffSealed before the cutoff", err)
	}
}

// The gate's day window is half-open on both ends against the same
// manifest set: the books-cutoff day is IN (it holds the first gate
// bytes), the day before it is out, and the bound day is out (still
// growing). Only a fixture holding a manifest on each of those three days
// can tell the three rules apart.
func TestLoadManifestDaysWindowBounds(t *testing.T) {
	root := t.TempDir()
	for _, day := range []string{"2026-07-31", "2026-08-01", "2026-08-02", "2026-08-03"} {
		writeManifest(t, root, day)
	}
	days, err := LoadManifestDaysWindow(root, BooksCutoff, time.Date(2026, 8, 3, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 || days[0].Day != "2026-08-01" || days[1].Day != "2026-08-02" {
		t.Fatalf("days %+v, want 08-01 and 08-02", days)
	}
}

// An unset window start refuses rather than silently enumerating from the
// beginning of the archive — the gate's universe is declared, never
// defaulted.
func TestLoadManifestDaysWindowRefusesUnsetStart(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "2026-08-01")
	if _, err := LoadManifestDaysWindow(root, time.Time{}, BooksCutoff.Add(48*time.Hour)); err == nil {
		t.Fatal("an unset window start was accepted")
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
		if calls > len(pages) {
			// A call past the script is the resume path costing a request —
			// an error, not a panic, so a broken exists-check fails the
			// assertions below instead of crashing the suite.
			return nil, fmt.Errorf("transport call %d beyond the %d scripted pages", calls, len(pages))
		}
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
