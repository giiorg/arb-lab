package screening

// The harness: the two stages `arblab screen` runs. Fetch is
// resume-safe (a file already on disk is skipped, never refetched) and
// failure-tolerant (a symbol the venue refuses is recorded, not fatal —
// a delisting mid-run must not kill a thirty-minute walk). Compute is
// pure function of the raw root: run it twice, get the same bytes.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Harness wires the two stages.
type Harness struct {
	T      Transport
	Root   string // raw store root (git-ignored)
	OutDir string // committed outputs directory
	// FetchedAt stamps provenance on raw files; injected, never read
	// ambiently, so the stages stay replayable.
	FetchedAt time.Time
	// Logf reports progress; nil silences it.
	Logf func(format string, args ...any)
}

func (h Harness) logf(format string, args ...any) {
	if h.Logf != nil {
		h.Logf(format, args...)
	}
}

const failuresName = "fetch_failures.json"

// FetchStage pulls the universe snapshots and every non-excluded
// intersection symbol's funding history from both venues.
func (h Harness) FetchStage() error {
	if err := FetchUniverse(h.T, h.Root, CutoffUTC, h.FetchedAt); err != nil {
		return err
	}
	roster, err := Intersect(h.Root)
	if err != nil {
		return err
	}
	h.logf("universe: %d intersection symbols", len(roster))

	var failures []FetchFailure
	fetched := 0
	for _, p := range roster {
		if p.Excluded != "" {
			h.logf("skip %s: %s", p.Symbol, p.Excluded)
			continue
		}
		// The two venues' walks share nothing — different hosts,
		// different files — so they run concurrently and the wall
		// clock is the slower walk, not the sum. Politeness pacing is
		// the transport's job and is per-host, so concurrency here
		// cannot compress a venue's request spacing.
		var wg sync.WaitGroup
		var mu sync.Mutex // guards failures and writeErr
		var writeErr error
		for _, venue := range []string{"binance", "bybit"} {
			path := RawPath(h.Root, venue, KindFunding, p.Symbol)
			if _, statErr := os.Stat(path); statErr == nil {
				continue // resume: already fetched
			}
			wg.Add(1)
			go func(venue, path string) {
				defer wg.Done()
				var f RawFile
				var fetchErr error
				if venue == "binance" {
					f, fetchErr = FetchBinanceFunding(h.T, p.Symbol, CutoffUTC, h.FetchedAt, BinancePageLimit)
				} else {
					f, fetchErr = FetchBybitFunding(h.T, p.Symbol, CutoffUTC, h.FetchedAt, BybitPageLimit)
				}
				if fetchErr != nil {
					mu.Lock()
					failures = append(failures, FetchFailure{Venue: venue, Symbol: p.Symbol, Error: fetchErr.Error()})
					mu.Unlock()
					h.logf("FAIL %s/%s: %v", venue, p.Symbol, fetchErr)
					return
				}
				if _, err := WriteRaw(path, f); err != nil {
					mu.Lock()
					writeErr = err
					mu.Unlock()
				}
			}(venue, path)
		}
		wg.Wait()
		if writeErr != nil {
			return writeErr
		}
		fetched++
		if fetched%25 == 0 {
			h.logf("fetched %d/%d symbols", fetched, len(roster))
		}
	}
	// Failures are stage state, re-derivable and overwritable — unlike
	// venue words.
	data, err := json.MarshalIndent(failures, "", "  ")
	if err != nil {
		return fmt.Errorf("failures encode: %w", err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, failuresName), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("failures write: %w", err)
	}
	h.logf("fetch stage done: %d symbols, %d failures", fetched, len(failures))
	return nil
}

// Summary is what ComputeStage hands the CLI to print.
type Summary struct {
	Universe   int
	Ranked     int
	Ineligible int
	Shortlist  []string
	Events     int
}

// Pair-row eligibility reasons. Report vocabulary: strings, written
// once here, never switched over.
const (
	eligibleOK        = "ok"
	reasonExcluded    = "excluded"
	reasonMismatched  = "mismatched_grid"
	reasonMissingRaw  = "missing_raw"
	reasonNoTicker    = "no_ticker"
	reasonBelowFloor  = "below_floor"
	reasonThinMatched = "insufficient_matched"
)

// ComputeStage reads the raw root and writes the four committed
// outputs.
func (h Harness) ComputeStage() (Summary, error) {
	roster, err := Intersect(h.Root)
	if err != nil {
		return Summary{}, err
	}
	generatedAt := h.FetchedAt.UTC().Format(time.RFC3339)
	params := EchoParams()

	ledger := Ledger{
		GeneratedAt: generatedAt,
		CutoffUTC:   CutoffUTC.Format(time.RFC3339),
		Params:      params,
	}
	if data, readErr := os.ReadFile(filepath.Join(h.Root, failuresName)); readErr == nil {
		if err := json.Unmarshal(data, &ledger.Failures); err != nil {
			return Summary{}, fmt.Errorf("failures decode: %w", err)
		}
	}
	for _, venue := range []string{"binance", "bybit"} {
		for _, kind := range []string{KindExchangeInfo, KindFundingInfo, KindInstruments, KindTickers} {
			path := RawPath(h.Root, venue, kind, "")
			f, digest, loadErr := LoadRaw(path)
			if loadErr != nil {
				continue // the venue-kind combinations that exist were written by FetchUniverse
			}
			ledger.Files = append(ledger.Files, LedgerEntry{
				Venue: venue, Kind: kind, Path: path, SHA256: digest,
				Pages: len(f.Pages), Endpoint: f.Endpoint, FetchedAt: f.FetchedAt,
			})
		}
	}

	var (
		rows        []PairRow
		allEvents   []Event
		censusPairs int
	)
	for _, p := range roster {
		row := PairRow{
			Symbol:                 p.Symbol,
			BinanceIntervalHours:   p.BinanceIntervalHours,
			BybitIntervalMinutes:   p.BybitIntervalMinutes,
			SameGrid:               p.SameGrid,
			BinanceQuoteVolume24h:  p.BinanceQuoteVolume24h,
			BybitTurnover24h:       p.BybitTurnover24h,
			BybitOpenInterestValue: p.BybitOpenInterestValue,
		}
		if p.Excluded != "" {
			row.Eligible = reasonExcluded + ": " + p.Excluded
			rows = append(rows, row)
			continue
		}

		// Each side that exists is ledgered even when its partner is
		// missing — the bytes are on disk and a rerun will consume
		// them, so leaving them out would understate what was fetched.
		loadSide := func(venue string) ([]Row, bool, error) {
			path := RawPath(h.Root, venue, KindFunding, p.Symbol)
			raw, digest, loadErr := LoadRaw(path)
			if loadErr != nil {
				return nil, false, nil
			}
			sideRows, rowsErr := RowsFromRaw(raw)
			if rowsErr != nil {
				return nil, false, rowsErr
			}
			ledger.Files = append(ledger.Files, ledgerEntryFor(path, raw, digest, sideRows))
			return sideRows, true, nil
		}
		bnRows, bnOK, err := loadSide("binance")
		if err != nil {
			return Summary{}, err
		}
		byRows, byOK, err := loadSide("bybit")
		if err != nil {
			return Summary{}, err
		}
		if !bnOK || !byOK {
			row.Eligible = reasonMissingRaw
			rows = append(rows, row)
			continue
		}

		matched, stats, err := PairSettlements(bnRows, byRows)
		if err != nil {
			return Summary{}, fmt.Errorf("%s: %w", p.Symbol, err)
		}
		row.Pairing = &stats

		// The census covers every pair with matched data — mismatched
		// grids and thin histories included (§42b-1).
		if len(matched) > 0 {
			censusPairs++
			allEvents = append(allEvents, CensusPair(p.Symbol, matched)...)
		}

		switch {
		case !p.SameGrid:
			row.Eligible = reasonMismatched
		case len(matched) < MinMatchedSettlements:
			row.Eligible = reasonThinMatched
		default:
			row.Eligible = floorVerdict(p)
		}
		if row.Eligible != eligibleOK {
			rows = append(rows, row)
			continue
		}

		res, err := ScreenPair(p.Symbol, matched, stats)
		if err != nil {
			return Summary{}, err
		}
		row.First = res.First.Format(time.RFC3339)
		row.Last = res.Last.Format(time.RFC3339)
		row.ModalMinutes = int(res.ModalInterval / time.Minute)
		row.IntervalChanges = res.IntervalChanges
		row.MedianAbs = res.MedianAbs.Decimal()
		row.P99Abs = res.P99Abs.Decimal()
		row.MaxAbs = res.MaxAbs.Decimal()
		for _, l := range res.Levels {
			row.Levels = append(row.Levels, levelRow(l))
		}
		best := res.Levels[res.Best]
		row.BestEntry = best.Entry.Decimal()
		row.RankScore = best.NetAnnual.Decimal()
		rows = append(rows, row)
	}

	// Order: ranked pairs by score descending, then everything else by
	// symbol; ties by symbol so the output is deterministic.
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := rows[i], rows[j]
		if (ri.Eligible == eligibleOK) != (rj.Eligible == eligibleOK) {
			return ri.Eligible == eligibleOK
		}
		if ri.Eligible == eligibleOK {
			c := cmpDecimal(ri.RankScore, rj.RankScore)
			if c != 0 {
				return c > 0
			}
		}
		return ri.Symbol < rj.Symbol
	})

	sort.Slice(ledger.Files, func(i, j int) bool { return ledger.Files[i].Path < ledger.Files[j].Path })

	summary := Summary{Universe: len(roster)}
	var shortRows []PairRow
	for _, r := range rows {
		if r.Eligible != eligibleOK {
			summary.Ineligible++
			continue
		}
		summary.Ranked++
		if len(shortRows) < ShortlistSize && cmpDecimal(r.RankScore, "0") > 0 {
			shortRows = append(shortRows, r)
			summary.Shortlist = append(summary.Shortlist, r.Symbol)
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		if !allEvents[i].Start.Equal(allEvents[j].Start) {
			return allEvents[i].Start.Before(allEvents[j].Start)
		}
		return allEvents[i].Symbol < allEvents[j].Symbol
	})
	census := Census{
		GeneratedAt:  generatedAt,
		Params:       params,
		PairsCovered: censusPairs,
		EventsTotal:  len(allEvents),
		PerYear:      map[string]int{},
	}
	for _, e := range allEvents {
		census.PerYear[fmt.Sprintf("%d", e.Start.UTC().Year())]++
		census.Events = append(census.Events, eventRow(e))
	}
	byPeak := append([]Event(nil), allEvents...)
	sort.SliceStable(byPeak, func(i, j int) bool { return mustCmp(byPeak[i].Peak, byPeak[j].Peak) > 0 })
	for i, e := range byPeak {
		if i == 50 {
			break
		}
		census.TopByPeak = append(census.TopByPeak, eventRow(e))
	}
	summary.Events = len(allEvents)

	if err := writeJSON(h.OutDir, "ledger.json", ledger); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(h.OutDir, "ranking.json", Ranking{GeneratedAt: generatedAt, Params: params, Pairs: rows}); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(h.OutDir, "shortlist.json", Shortlist{GeneratedAt: generatedAt, Params: params, Symbols: shortRows}); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(h.OutDir, "census.json", census); err != nil {
		return Summary{}, err
	}
	return summary, nil
}

// floorVerdict applies the tradability floor to one pair's published
// 24h turnovers. A missing or unparseable venue figure is its own named
// reason — an absent number must not read as "passed".
func floorVerdict(p PairInfo) string {
	for _, v := range []string{p.BinanceQuoteVolume24h, p.BybitTurnover24h} {
		if v == "" {
			return reasonNoTicker
		}
		m, err := fixedpoint.ParseMoney("USDT", v)
		if err != nil {
			return reasonNoTicker
		}
		c, err := m.Cmp(FloorTurnover24h)
		if err != nil || c < 0 {
			return reasonBelowFloor
		}
	}
	return eligibleOK
}

func ledgerEntryFor(path string, f RawFile, digest string, rows []Row) LedgerEntry {
	e := LedgerEntry{
		Venue: f.Venue, Kind: f.Kind, Symbol: f.Symbol, Path: path,
		SHA256: digest, Pages: len(f.Pages), Rows: len(rows),
		Endpoint: f.Endpoint, FetchedAt: f.FetchedAt,
	}
	if len(rows) > 0 {
		e.First = rows[0].At.Format(time.RFC3339)
		e.Last = rows[len(rows)-1].At.Format(time.RFC3339)
	}
	return e
}

// cmpDecimal compares two decimal strings by re-parsing them as rates —
// the report keeps strings, the ordering stays exact.
func cmpDecimal(a, b string) int {
	ra, err := fixedpoint.ParseRate(a)
	if err != nil {
		panic(err)
	}
	rb, err := fixedpoint.ParseRate(b)
	if err != nil {
		panic(err)
	}
	return mustCmp(ra, rb)
}
