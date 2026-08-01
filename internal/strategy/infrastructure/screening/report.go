package screening

// The committed outputs (§42b step 5): the dataset ledger, the full
// ranking with every ineligibility named, the shortlist, and the
// census. Everything a rate, a count or a digest — the JSON carries
// decimal strings, never binary floats, so the committed evidence is
// exact and diffable.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LedgerEntry is one raw file's identity in the committed ledger.
type LedgerEntry struct {
	Venue     string `json:"venue"`
	Kind      string `json:"kind"`
	Symbol    string `json:"symbol,omitempty"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Pages     int    `json:"pages"`
	Rows      int    `json:"rows,omitempty"`
	First     string `json:"first,omitempty"`
	Last      string `json:"last,omitempty"`
	Endpoint  string `json:"endpoint"`
	FetchedAt string `json:"fetched_at"`
}

// FetchFailure records a symbol the fetch stage could not complete; the
// compute stage carries it into the ledger rather than silently
// shrinking the universe.
type FetchFailure struct {
	Venue  string `json:"venue"`
	Symbol string `json:"symbol"`
	Error  string `json:"error"`
}

// Ledger is ledger.json: every byte the screening consumed, digested,
// plus what it failed to fetch.
type Ledger struct {
	GeneratedAt string         `json:"generated_at"`
	CutoffUTC   string         `json:"cutoff_utc"`
	Params      ParamsEcho     `json:"params"`
	Files       []LedgerEntry  `json:"files"`
	Failures    []FetchFailure `json:"failures,omitempty"`
}

// ParamsEcho freezes the roster's values into every output file, so a
// reader of the artifact alone knows what produced it.
type ParamsEcho struct {
	CutoffUTC             string            `json:"cutoff_utc"`
	HistoryStart          string            `json:"history_start"`
	PairingRound          string            `json:"pairing_round"`
	MaxGapFactor          int               `json:"max_gap_factor"`
	RoundTripCost         string            `json:"round_trip_cost"`
	EntryLevels           []string          `json:"entry_levels"`
	ExitDivisor           int               `json:"exit_divisor"`
	MinMatchedSettlements int               `json:"min_matched_settlements"`
	FloorTurnover24h      string            `json:"floor_turnover_24h_usdt"`
	ShortlistSize         int               `json:"shortlist_size"`
	CensusLevel           string            `json:"census_level"`
	ExcludedSymbols       map[string]string `json:"excluded_symbols"`
}

// EchoParams renders the roster.
func EchoParams() ParamsEcho {
	levels := make([]string, len(EntryLevels))
	for i, e := range EntryLevels {
		levels[i] = e.Decimal()
	}
	return ParamsEcho{
		CutoffUTC:             CutoffUTC.Format(time.RFC3339),
		HistoryStart:          HistoryStart.Format(time.RFC3339),
		PairingRound:          PairingRound.String(),
		MaxGapFactor:          MaxGapFactor,
		RoundTripCost:         RoundTripCost().Decimal(),
		EntryLevels:           levels,
		ExitDivisor:           ExitDivisor,
		MinMatchedSettlements: MinMatchedSettlements,
		FloorTurnover24h:      FloorTurnover24h.Decimal(),
		ShortlistSize:         ShortlistSize,
		CensusLevel:           CensusLevel.Decimal(),
		ExcludedSymbols:       ExcludedSymbols,
	}
}

// LevelRow is LevelResult rendered for the report.
type LevelRow struct {
	Entry      string `json:"entry"`
	Episodes   int    `json:"episodes"`
	Wins       int    `json:"wins"`
	Accrued    int    `json:"accrued_settlements"`
	MaxRun     int    `json:"max_run"`
	Gross      string `json:"gross"`
	Net        string `json:"net"`
	NetAnnual  string `json:"net_annual"`
	AboveCount int    `json:"above_count"`
	StayCount  int    `json:"stay_count"`
	Per10k     int    `json:"continuation_per_10k"`
}

func levelRow(l LevelResult) LevelRow {
	return LevelRow{
		Entry:      l.Entry.Decimal(),
		Episodes:   l.Episodes,
		Wins:       l.Wins,
		Accrued:    l.Accrued,
		MaxRun:     l.MaxRun,
		Gross:      l.Gross.Decimal(),
		Net:        l.Net.Decimal(),
		NetAnnual:  l.NetAnnual.Decimal(),
		AboveCount: l.AboveCount,
		StayCount:  l.StayCount,
		Per10k:     l.Per10k,
	}
}

// PairRow is one pair in ranking.json. Eligible is "ok" or the named
// reason the pair cannot rank; ineligible pairs keep their universe
// facts so the exclusion is auditable.
type PairRow struct {
	Symbol   string `json:"symbol"`
	Eligible string `json:"eligible"`

	BinanceIntervalHours int  `json:"binance_interval_hours"`
	BybitIntervalMinutes int  `json:"bybit_interval_minutes"`
	SameGrid             bool `json:"same_grid"`

	BinanceQuoteVolume24h  string `json:"binance_quote_volume_24h,omitempty"`
	BybitTurnover24h       string `json:"bybit_turnover_24h,omitempty"`
	BybitOpenInterestValue string `json:"bybit_open_interest_value,omitempty"`

	Pairing *PairStats `json:"pairing,omitempty"`

	First           string     `json:"first,omitempty"`
	Last            string     `json:"last,omitempty"`
	ModalMinutes    int        `json:"modal_interval_minutes,omitempty"`
	IntervalChanges int        `json:"interval_changes,omitempty"`
	MedianAbs       string     `json:"median_abs,omitempty"`
	P99Abs          string     `json:"p99_abs,omitempty"`
	MaxAbs          string     `json:"max_abs,omitempty"`
	Levels          []LevelRow `json:"levels,omitempty"`
	BestEntry       string     `json:"best_entry,omitempty"`
	RankScore       string     `json:"rank_score,omitempty"`
}

// Ranking is ranking.json.
type Ranking struct {
	GeneratedAt string     `json:"generated_at"`
	Params      ParamsEcho `json:"params"`
	Pairs       []PairRow  `json:"pairs"`
}

// Shortlist is shortlist.json — the §42c capture expansion's input.
type Shortlist struct {
	GeneratedAt string     `json:"generated_at"`
	Params      ParamsEcho `json:"params"`
	Symbols     []PairRow  `json:"symbols"`
}

// EventRow is one census event rendered.
type EventRow struct {
	Symbol      string `json:"symbol"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Settlements int    `json:"settlements"`
	Direction   int    `json:"direction"`
	Peak        string `json:"peak"`
	Sum         string `json:"sum"`
}

func eventRow(e Event) EventRow {
	return EventRow{
		Symbol:      e.Symbol,
		Start:       e.Start.Format(time.RFC3339),
		End:         e.End.Format(time.RFC3339),
		Settlements: e.Settlements,
		Direction:   e.Direction,
		Peak:        e.Peak.Decimal(),
		Sum:         e.Sum.Decimal(),
	}
}

// Census is census.json: every event, plus the aggregates §42f-2 reads.
type Census struct {
	GeneratedAt  string         `json:"generated_at"`
	Params       ParamsEcho     `json:"params"`
	PairsCovered int            `json:"pairs_covered"`
	EventsTotal  int            `json:"events_total"`
	PerYear      map[string]int `json:"per_year"`
	// TopByPeak is the largest events, a reading aid; Events is the
	// full chronological table.
	TopByPeak []EventRow `json:"top_by_peak"`
	Events    []EventRow `json:"events"`
}

// writeJSON writes one committed output file (overwriting: outputs are
// derived, re-derivable, and versioned by git — unlike raw venue words,
// which are write-once).
func writeJSON(dir, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s encode: %w", name, err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s dir: %w", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return fmt.Errorf("%s write: %w", name, err)
	}
	return nil
}
