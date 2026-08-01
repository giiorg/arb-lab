package screening

// The end-to-end battery: a canned two-venue world with one rankable
// pair, one below the floor, one mismatched grid, one fetch failure and
// BTCUSDT — which must never be requested at all (the §42e seal is a
// behaviour, so the test asserts on the URLS the harness asked for, not
// on output shape alone). The same world then proves the fetch stage
// resumes without a single new request, and that compute is a pure
// function of the raw root.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// world is the canned transport. It generates 200 settlements per
// symbol (4h grid, phase-shared) ending just inside the cutoff, so
// every pair clears MinMatchedSettlements with room. The request log
// is mutex-guarded because the harness walks the venues concurrently.
type world struct {
	tb       testing.TB
	mu       sync.Mutex
	requests []string
}

func (w *world) logRequest(url string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.requests = append(w.requests, url)
}

func (w *world) requestLog() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.requests...)
}

const worldSettlements = 200

// worldGrid returns the settlement instants, oldest first.
func worldGrid() []time.Time {
	out := make([]time.Time, worldSettlements)
	last := CutoffUTC.Add(-4 * time.Hour)
	for i := range out {
		out[i] = last.Add(-time.Duration(worldSettlements-1-i) * 4 * time.Hour)
	}
	return out
}

// Per-symbol venue rates. ALPHAUSDT carries a persistent 7bp spread;
// GAMMAUSDT is quiet except one 39bp dislocation two instants before
// the end; everything else is flat.
func worldRates(symbol, venue string, at time.Time) string {
	grid := worldGrid()
	switch {
	case symbol == "ALPHAUSDT" && venue == "binance":
		return "0.0008"
	case symbol == "GAMMAUSDT" && venue == "binance" && at.Equal(grid[len(grid)-2]):
		return "0.004"
	}
	return "0.0001"
}

func (w *world) get(url string) ([]byte, error) {
	w.logRequest(url)
	switch {
	case strings.Contains(url, "exchangeInfo"):
		return []byte(`{"symbols":[
			{"symbol":"ALPHAUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"BETAUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"GAMMAUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"OMEGAUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"DEADUSDT","status":"SETTLING","contractType":"PERPETUAL","quoteAsset":"USDT"},
			{"symbol":"ONLYBNUSDT","status":"TRADING","contractType":"PERPETUAL","quoteAsset":"USDT"}]}`), nil
	case strings.Contains(url, "fundingInfo"):
		return []byte(`[
			{"symbol":"ALPHAUSDT","fundingIntervalHours":4},
			{"symbol":"BETAUSDT","fundingIntervalHours":4},
			{"symbol":"GAMMAUSDT","fundingIntervalHours":4},
			{"symbol":"OMEGAUSDT","fundingIntervalHours":4}]`), nil
	case strings.Contains(url, "ticker/24hr"):
		return []byte(`[
			{"symbol":"ALPHAUSDT","quoteVolume":"90000000.50"},
			{"symbol":"BETAUSDT","quoteVolume":"1000000"},
			{"symbol":"GAMMAUSDT","quoteVolume":"90000000"},
			{"symbol":"OMEGAUSDT","quoteVolume":"90000000"},
			{"symbol":"BTCUSDT","quoteVolume":"9000000000"}]`), nil
	case strings.Contains(url, "instruments-info"):
		return []byte(`{"retCode":0,"retMsg":"OK","result":{"nextPageCursor":"","list":[
			{"symbol":"ALPHAUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":240},
			{"symbol":"BETAUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":240},
			{"symbol":"GAMMAUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":480},
			{"symbol":"OMEGAUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":240},
			{"symbol":"BTCUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":480},
			{"symbol":"ONLYBYUSDT","status":"Trading","contractType":"LinearPerpetual","quoteCoin":"USDT","fundingInterval":240}]}}`), nil
	case strings.Contains(url, "market/tickers"):
		return []byte(`{"retCode":0,"retMsg":"OK","result":{"list":[
			{"symbol":"ALPHAUSDT","turnover24h":"80000000","openInterestValue":"12000000"},
			{"symbol":"BETAUSDT","turnover24h":"80000000","openInterestValue":"500000"},
			{"symbol":"GAMMAUSDT","turnover24h":"80000000","openInterestValue":"7000000"},
			{"symbol":"OMEGAUSDT","turnover24h":"80000000","openInterestValue":"7000000"},
			{"symbol":"BTCUSDT","turnover24h":"9000000000","openInterestValue":"9000000000"}]}}`), nil
	case strings.Contains(url, "fundingRate?symbol=OMEGAUSDT"):
		return nil, fmt.Errorf("venue refused")
	case strings.Contains(url, "fundingRate?symbol="):
		return w.binanceFunding(url)
	case strings.Contains(url, "funding/history"):
		return w.bybitFunding(url)
	}
	w.tb.Fatalf("unexpected url %s", url)
	return nil, nil
}

func symbolFrom(url, param string) string {
	i := strings.Index(url, param)
	rest := url[i+len(param):]
	if j := strings.IndexByte(rest, '&'); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

func paramInt(tb testing.TB, url, name string) int64 {
	i := strings.Index(url, name+"=")
	if i < 0 {
		tb.Fatalf("url %s: no %s", url, name)
	}
	rest := url[i+len(name)+1:]
	if j := strings.IndexByte(rest, '&'); j >= 0 {
		rest = rest[:j]
	}
	var v int64
	if _, err := fmt.Sscanf(rest, "%d", &v); err != nil {
		tb.Fatalf("url %s: %s: %v", url, name, err)
	}
	return v
}

// binanceFunding serves the ascending dialect over the world grid.
func (w *world) binanceFunding(url string) ([]byte, error) {
	symbol := symbolFrom(url, "symbol=")
	start := paramInt(w.tb, url, "startTime")
	end := paramInt(w.tb, url, "endTime")
	var rows [][2]string
	for _, at := range worldGrid() {
		ms := at.UnixMilli()
		if ms < start || ms > end || len(rows) == BinancePageLimit {
			continue
		}
		rows = append(rows, [2]string{fmt.Sprintf("%d", ms), worldRates(symbol, "binance", at)})
	}
	entries := make([]string, len(rows))
	for i, r := range rows {
		entries[i] = fmt.Sprintf(`{"symbol":%q,"fundingTime":%s,"fundingRate":%q}`, symbol, r[0], r[1])
	}
	return []byte("[" + strings.Join(entries, ",") + "]"), nil
}

// bybitFunding serves the newest-first dialect over the same grid.
func (w *world) bybitFunding(url string) ([]byte, error) {
	symbol := symbolFrom(url, "symbol=")
	end := paramInt(w.tb, url, "endTime")
	grid := worldGrid()
	var entries []string
	for i := len(grid) - 1; i >= 0; i-- {
		ms := grid[i].UnixMilli()
		if ms > end || len(entries) == BybitPageLimit {
			continue
		}
		entries = append(entries, fmt.Sprintf(
			`{"symbol":%q,"fundingRate":%q,"fundingRateTimestamp":"%d"}`,
			symbol, worldRates(symbol, "bybit", grid[i]), ms))
	}
	return []byte(`{"retCode":0,"retMsg":"OK","result":{"list":[` + strings.Join(entries, ",") + `]}}`), nil
}

func TestHarnessEndToEnd(t *testing.T) {
	w := &world{tb: t}
	dir := t.TempDir()
	h := Harness{
		T:         Transport{Get: w.get},
		Root:      filepath.Join(dir, "raw"),
		OutDir:    filepath.Join(dir, "out"),
		FetchedAt: CutoffUTC,
	}
	if err := h.FetchStage(); err != nil {
		t.Fatalf("fetch stage: %v", err)
	}

	// The §42e seal is a behaviour: BTCUSDT is never requested.
	for _, u := range w.requestLog() {
		if strings.Contains(u, "BTCUSDT") && strings.Contains(u, "funding") && !strings.Contains(u, "fundingInfo") {
			t.Fatalf("BTC funding was requested: %s", u)
		}
	}

	sum, err := h.ComputeStage()
	if err != nil {
		t.Fatalf("compute stage: %v", err)
	}
	if sum.Universe != 5 {
		t.Fatalf("universe %d, want 5", sum.Universe)
	}
	if len(sum.Shortlist) != 1 || sum.Shortlist[0] != "ALPHAUSDT" {
		t.Fatalf("shortlist %v, want [ALPHAUSDT]", sum.Shortlist)
	}
	if sum.Ranked != 1 || sum.Ineligible != 4 {
		t.Fatalf("ranked %d ineligible %d, want 1 and 4", sum.Ranked, sum.Ineligible)
	}

	var ranking Ranking
	readJSON(t, filepath.Join(h.OutDir, "ranking.json"), &ranking)
	by0 := map[string]PairRow{}
	for _, r := range ranking.Pairs {
		by0[r.Symbol] = r
	}
	if !strings.HasPrefix(byO(t, by0, "BTCUSDT").Eligible, "excluded") {
		t.Fatalf("BTC eligible %q, want excluded", byO(t, by0, "BTCUSDT").Eligible)
	}
	if byO(t, by0, "BETAUSDT").Eligible != "below_floor" {
		t.Fatalf("BETA eligible %q, want below_floor", byO(t, by0, "BETAUSDT").Eligible)
	}
	if byO(t, by0, "GAMMAUSDT").Eligible != "mismatched_grid" {
		t.Fatalf("GAMMA eligible %q, want mismatched_grid", byO(t, by0, "GAMMAUSDT").Eligible)
	}
	if byO(t, by0, "OMEGAUSDT").Eligible != "missing_raw" {
		t.Fatalf("OMEGA eligible %q, want missing_raw", byO(t, by0, "OMEGAUSDT").Eligible)
	}
	alpha := byO(t, by0, "ALPHAUSDT")
	if alpha.Eligible != "ok" || alpha.Pairing.Matched != worldSettlements {
		t.Fatalf("ALPHA %+v, want ok with %d matched", alpha, worldSettlements)
	}

	// GAMMA's mismatched grid still reaches the census, and its one
	// dislocation (39bp spread) is the only event in the world.
	var census Census
	readJSON(t, filepath.Join(h.OutDir, "census.json"), &census)
	if census.EventsTotal != 1 || census.Events[0].Symbol != "GAMMAUSDT" {
		t.Fatalf("census %+v, want exactly GAMMA's one event", census)
	}
	if census.Events[0].Peak != "0.0039" {
		t.Fatalf("census peak %s, want 0.0039", census.Events[0].Peak)
	}

	// The ledger carries OMEGA's bybit side (fetched, partner failed)
	// and the failure itself.
	var ledger Ledger
	readJSON(t, filepath.Join(h.OutDir, "ledger.json"), &ledger)
	if len(ledger.Failures) != 1 || ledger.Failures[0].Symbol != "OMEGAUSDT" || ledger.Failures[0].Venue != "binance" {
		t.Fatalf("failures %+v, want OMEGA/binance", ledger.Failures)
	}
	wantFiles := 5 + 2*3 + 1 // universe snapshots + three full pairs + OMEGA's bybit side
	if len(ledger.Files) != wantFiles {
		t.Fatalf("ledger files %d, want %d", len(ledger.Files), wantFiles)
	}

	// Resume: a second fetch stage issues NO new requests — every file
	// already on disk is skipped, and the universe snapshots are
	// reused. The one exception is the failed symbol, retried.
	before := len(w.requestLog())
	if err := h.FetchStage(); err != nil {
		t.Fatalf("second fetch stage: %v", err)
	}
	for _, u := range w.requestLog()[before:] {
		if !strings.Contains(u, "OMEGAUSDT") {
			t.Fatalf("resume refetched %s", u)
		}
	}
}

func byO(tb testing.TB, m map[string]PairRow, sym string) PairRow {
	tb.Helper()
	r, ok := m[sym]
	if !ok {
		tb.Fatalf("no ranking row for %s", sym)
	}
	return r
}

func readJSON(tb testing.TB, path string, v any) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		tb.Fatalf("decode %s: %v", path, err)
	}
}

// Intersect classifies grids from the venues' own units (hours vs
// minutes) and defaults an absent Binance fundingInfo entry to 8h —
// BTCUSDT (absent above, Bybit 480) is the default's same-grid case.
func TestIntersectGridFacts(t *testing.T) {
	w := &world{tb: t}
	dir := t.TempDir()
	h := Harness{T: Transport{Get: w.get}, Root: filepath.Join(dir, "raw"), FetchedAt: CutoffUTC}
	if err := FetchUniverse(h.T, h.Root, CutoffUTC, h.FetchedAt); err != nil {
		t.Fatalf("universe: %v", err)
	}
	roster, err := Intersect(h.Root)
	if err != nil {
		t.Fatalf("intersect: %v", err)
	}
	facts := map[string]PairInfo{}
	for _, p := range roster {
		facts[p.Symbol] = p
	}
	if len(roster) != 5 {
		t.Fatalf("roster %d, want 5 (status filters and one-venue symbols out)", len(roster))
	}
	if _, ok := facts["ONLYBNUSDT"]; ok {
		t.Fatalf("ONLYBNUSDT is not on both venues")
	}
	if _, ok := facts["DEADUSDT"]; ok {
		t.Fatalf("DEADUSDT is not trading")
	}
	if p := facts["GAMMAUSDT"]; p.SameGrid || p.BinanceIntervalHours != 4 || p.BybitIntervalMinutes != 480 {
		t.Fatalf("GAMMA %+v, want mismatched 4h vs 480m", p)
	}
	if p := facts["BTCUSDT"]; !p.SameGrid || p.BinanceIntervalHours != 8 || p.Excluded == "" {
		t.Fatalf("BTC %+v, want defaulted 8h, same grid, excluded", p)
	}
	if p := facts["ALPHAUSDT"]; !p.SameGrid || p.BinanceQuoteVolume24h != "90000000.50" {
		t.Fatalf("ALPHA %+v, want same grid with its volume attached", p)
	}
}
