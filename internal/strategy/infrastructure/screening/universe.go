package screening

// The candidate universe (§42b): both venues' instrument rosters,
// re-fetched at screening time — the roster moves, so the scoping
// record's counts are a dated observation, never an input. The
// intersection, interval facts, and tradability inputs all come from
// the five snapshot files this stage writes.

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// PairInfo is one intersection symbol with the facts screening needs
// about it. Volume fields are the venue's words verbatim; they are
// parsed only where the floor is applied.
type PairInfo struct {
	Symbol string `json:"symbol"`

	// Interval facts. Binance publishes hours (fundingInfo, default 8
	// when absent); Bybit publishes minutes (instruments-info).
	BinanceIntervalHours int  `json:"binance_interval_hours"`
	BybitIntervalMinutes int  `json:"bybit_interval_minutes"`
	SameGrid             bool `json:"same_grid"`

	// Tradability inputs, venue words verbatim (24h quote turnover;
	// Bybit's open-interest value rides along as evidence, ungated).
	BinanceQuoteVolume24h  string `json:"binance_quote_volume_24h"`
	BybitTurnover24h       string `json:"bybit_turnover_24h"`
	BybitOpenInterestValue string `json:"bybit_open_interest_value"`

	// Excluded carries the roster reason when the screening refuses to
	// fetch this symbol (the H1 seal); empty otherwise.
	Excluded string `json:"excluded,omitempty"`
}

// FetchUniverse pulls the five snapshot files and writes them under the
// raw root. Snapshot files already present are reused (resume), so one
// screening run sees ONE universe even if the fetch stage restarts.
func FetchUniverse(t Transport, root string, cutoff, fetchedAt time.Time) error {
	single := func(venue, kind, url string) error {
		path := RawPath(root, venue, kind, "")
		if _, _, err := LoadRaw(path); err == nil {
			return nil
		}
		body, err := t.Get(url)
		if err != nil {
			return fmt.Errorf("universe %s/%s: %w", venue, kind, err)
		}
		f := RawFile{
			Venue: venue, Kind: kind, Endpoint: url,
			FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
			CutoffUTC: cutoff.UTC().Format(time.RFC3339),
			Pages:     []json.RawMessage{json.RawMessage(body)},
		}
		_, err = WriteRaw(path, f)
		return err
	}
	if err := single("binance", KindExchangeInfo, "https://fapi.binance.com/fapi/v1/exchangeInfo"); err != nil {
		return err
	}
	if err := single("binance", KindFundingInfo, "https://fapi.binance.com/fapi/v1/fundingInfo"); err != nil {
		return err
	}
	if err := single("binance", KindTickers, "https://fapi.binance.com/fapi/v1/ticker/24hr"); err != nil {
		return err
	}
	if err := single("bybit", KindTickers, "https://api.bybit.com/v5/market/tickers?category=linear"); err != nil {
		return err
	}
	// Bybit instruments-info pages by cursor; all pages land in one file.
	path := RawPath(root, "bybit", KindInstruments, "")
	if _, _, err := LoadRaw(path); err == nil {
		return nil
	}
	base := "https://api.bybit.com/v5/market/instruments-info?category=linear&limit=1000"
	f := RawFile{
		Venue: "bybit", Kind: KindInstruments, Endpoint: base,
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
		CutoffUTC: cutoff.UTC().Format(time.RFC3339),
	}
	cursor := ""
	for range MaxFetchPages {
		url := base
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		body, err := t.Get(url)
		if err != nil {
			return fmt.Errorf("universe bybit/instruments: %w", err)
		}
		next, err := bybitNextCursor(body)
		if err != nil {
			return fmt.Errorf("universe bybit/instruments: %w", err)
		}
		f.Pages = append(f.Pages, json.RawMessage(body))
		if next == "" {
			_, err := WriteRaw(path, f)
			return err
		}
		if next == cursor {
			return fmt.Errorf("universe bybit/instruments cursor %q: %w", cursor, ErrFetchStalled)
		}
		cursor = next
	}
	return ErrTooManyPages
}

// bybitEnvelope decodes the V5 envelope and refuses a non-zero retCode.
func bybitEnvelope(body []byte) (json.RawMessage, error) {
	var env struct {
		RetCode int             `json:"retCode"`
		RetMsg  string          `json:"retMsg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("bybit envelope: %w", err)
	}
	if env.RetCode != 0 {
		return nil, fmt.Errorf("retCode %d %q: %w", env.RetCode, env.RetMsg, ErrVenueEnvelope)
	}
	return env.Result, nil
}

func bybitNextCursor(body []byte) (string, error) {
	result, err := bybitEnvelope(body)
	if err != nil {
		return "", err
	}
	var res struct {
		NextPageCursor string `json:"nextPageCursor"`
	}
	if err := json.Unmarshal(result, &res); err != nil {
		return "", fmt.Errorf("bybit instruments result: %w", err)
	}
	return res.NextPageCursor, nil
}

// Intersect assembles the candidate universe from the five snapshot
// files: symbols that are USDT linear perpetuals in trading status on
// BOTH venues, with interval and tradability facts attached and the
// roster exclusions applied. The result is sorted by symbol.
func Intersect(root string) ([]PairInfo, error) {
	binanceSet, err := binanceTradingPerps(root)
	if err != nil {
		return nil, err
	}
	bybitSet, err := bybitTradingPerps(root)
	if err != nil {
		return nil, err
	}
	binanceHours, err := binanceIntervalHours(root)
	if err != nil {
		return nil, err
	}
	binanceVol, err := binanceQuoteVolumes(root)
	if err != nil {
		return nil, err
	}
	bybitVol, bybitOI, err := bybitTurnovers(root)
	if err != nil {
		return nil, err
	}

	var out []PairInfo
	for sym := range binanceSet {
		byMinutes, onBybit := bybitSet[sym]
		if !onBybit {
			continue
		}
		hours, hasOverride := binanceHours[sym]
		if !hasOverride {
			// Binance's documented default when fundingInfo lists no
			// override for a symbol.
			hours = 8
		}
		p := PairInfo{
			Symbol:                 sym,
			BinanceIntervalHours:   hours,
			BybitIntervalMinutes:   byMinutes,
			SameGrid:               hours*60 == byMinutes,
			BinanceQuoteVolume24h:  binanceVol[sym],
			BybitTurnover24h:       bybitVol[sym],
			BybitOpenInterestValue: bybitOI[sym],
			Excluded:               ExcludedSymbols[sym],
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, nil
}

func binanceTradingPerps(root string) (map[string]bool, error) {
	f, _, err := LoadRaw(RawPath(root, "binance", KindExchangeInfo, ""))
	if err != nil {
		return nil, err
	}
	if len(f.Pages) != 1 {
		return nil, fmt.Errorf("binance exchange_info: want 1 page, have %d", len(f.Pages))
	}
	var info struct {
		Symbols []struct {
			Symbol       string `json:"symbol"`
			Status       string `json:"status"`
			ContractType string `json:"contractType"`
			QuoteAsset   string `json:"quoteAsset"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(f.Pages[0], &info); err != nil {
		return nil, fmt.Errorf("binance exchange_info: %w", err)
	}
	set := make(map[string]bool)
	for _, s := range info.Symbols {
		if s.Status == "TRADING" && s.ContractType == "PERPETUAL" && s.QuoteAsset == "USDT" {
			set[s.Symbol] = true
		}
	}
	return set, nil
}

func binanceIntervalHours(root string) (map[string]int, error) {
	f, _, err := LoadRaw(RawPath(root, "binance", KindFundingInfo, ""))
	if err != nil {
		return nil, err
	}
	if len(f.Pages) != 1 {
		return nil, fmt.Errorf("binance funding_info: want 1 page, have %d", len(f.Pages))
	}
	var entries []struct {
		Symbol               string `json:"symbol"`
		FundingIntervalHours int    `json:"fundingIntervalHours"`
	}
	if err := json.Unmarshal(f.Pages[0], &entries); err != nil {
		return nil, fmt.Errorf("binance funding_info: %w", err)
	}
	m := make(map[string]int)
	for _, e := range entries {
		m[e.Symbol] = e.FundingIntervalHours
	}
	return m, nil
}

func binanceQuoteVolumes(root string) (map[string]string, error) {
	f, _, err := LoadRaw(RawPath(root, "binance", KindTickers, ""))
	if err != nil {
		return nil, err
	}
	if len(f.Pages) != 1 {
		return nil, fmt.Errorf("binance tickers: want 1 page, have %d", len(f.Pages))
	}
	var entries []struct {
		Symbol      string `json:"symbol"`
		QuoteVolume string `json:"quoteVolume"`
	}
	if err := json.Unmarshal(f.Pages[0], &entries); err != nil {
		return nil, fmt.Errorf("binance tickers: %w", err)
	}
	m := make(map[string]string)
	for _, e := range entries {
		m[e.Symbol] = e.QuoteVolume
	}
	return m, nil
}

func bybitTradingPerps(root string) (map[string]int, error) {
	f, _, err := LoadRaw(RawPath(root, "bybit", KindInstruments, ""))
	if err != nil {
		return nil, err
	}
	set := make(map[string]int)
	for i, page := range f.Pages {
		result, err := bybitEnvelope(page)
		if err != nil {
			return nil, fmt.Errorf("bybit instruments page %d: %w", i, err)
		}
		var res struct {
			List []struct {
				Symbol          string `json:"symbol"`
				Status          string `json:"status"`
				ContractType    string `json:"contractType"`
				QuoteCoin       string `json:"quoteCoin"`
				FundingInterval int    `json:"fundingInterval"`
			} `json:"list"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			return nil, fmt.Errorf("bybit instruments page %d: %w", i, err)
		}
		for _, s := range res.List {
			if s.Status == "Trading" && s.ContractType == "LinearPerpetual" && s.QuoteCoin == "USDT" {
				set[s.Symbol] = s.FundingInterval
			}
		}
	}
	return set, nil
}

func bybitTurnovers(root string) (turnover, openInterest map[string]string, err error) {
	f, _, err := LoadRaw(RawPath(root, "bybit", KindTickers, ""))
	if err != nil {
		return nil, nil, err
	}
	if len(f.Pages) != 1 {
		return nil, nil, fmt.Errorf("bybit tickers: want 1 page, have %d", len(f.Pages))
	}
	result, err := bybitEnvelope(f.Pages[0])
	if err != nil {
		return nil, nil, fmt.Errorf("bybit tickers: %w", err)
	}
	var res struct {
		List []struct {
			Symbol            string `json:"symbol"`
			Turnover24h       string `json:"turnover24h"`
			OpenInterestValue string `json:"openInterestValue"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &res); err != nil {
		return nil, nil, fmt.Errorf("bybit tickers: %w", err)
	}
	turnover = make(map[string]string)
	openInterest = make(map[string]string)
	for _, e := range res.List {
		turnover[e.Symbol] = e.Turnover24h
		openInterest[e.Symbol] = e.OpenInterestValue
	}
	return turnover, openInterest, nil
}
