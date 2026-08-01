package driving

// The family-1 dataset file (§41c): venue REST history, fetched once,
// committed verbatim under datasets/. The file's Pages are the venue's
// exact response bytes — the wrapper adds the closed identity (venue,
// series, symbol, start AND end), the fetch provenance, and nothing else.
// Extraction parses the venue-native rows back out for validation and
// derivation; the FILE never holds a transformed row, so the digest the
// artifact cites is a digest over the venue's words.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
)

// Dataset sentinels.
var (
	// ErrSeriesUnknown reports a (venue, series) pair no parser claims.
	ErrSeriesUnknown = fmt.Errorf("strategy/driving: no parser for this venue and series")

	// ErrSeriesUnsupported reports a fetch this milestone deliberately
	// does not implement: no derivation consumes it, and dead fetch code
	// would be a liability with no consumer. A v2 fetch lands WITH its
	// consumer.
	ErrSeriesUnsupported = fmt.Errorf("strategy/driving: series has no derivation consumer this milestone")

	// ErrRowOffGrid reports a venue row whose instant is farther from
	// every grid point than the declared jitter tolerance — a row the
	// closed identity cannot claim.
	ErrRowOffGrid = fmt.Errorf("strategy/driving: row instant off the declared grid beyond tolerance")

	// ErrRowDuplicate reports two venue rows normalizing to one grid
	// instant.
	ErrRowDuplicate = fmt.Errorf("strategy/driving: two rows claim one grid instant")

	// ErrPageMalformed reports a stored page that no longer parses as the
	// venue's documented shape.
	ErrPageMalformed = fmt.Errorf("strategy/driving: venue page does not parse")
)

// Series names. The interval is part of the series identity — a 1h kline
// series and a 4h one are different populations, not one series with a
// parameter.
const (
	SeriesFunding     = "funding"
	SeriesSpotKline1h = "spot_kline_1h"
	SeriesMarkKline1h = "mark_kline_1h"
)

// SeriesInterval is each series' grid interval.
func SeriesInterval(series string) (time.Duration, error) {
	switch series {
	case SeriesFunding:
		return 8 * time.Hour, nil
	case SeriesSpotKline1h, SeriesMarkKline1h:
		return time.Hour, nil
	}
	return 0, fmt.Errorf("series %q: %w", series, ErrSeriesUnknown)
}

// GridJitterTolerance is how far a venue row's instant may sit from its
// grid point and still be that grid point's row. Binance funding
// timestamps carry millisecond jitter (observed live: 1672560000008 —
// eight milliseconds past the hour); one minute is generous against
// observed jitter and still 1/480th of the funding interval, so no row
// can be ambiguous between two grid points.
const GridJitterTolerance = time.Minute

// DatasetFile is the committed file's schema.
type DatasetFile struct {
	Venue    string `json:"venue"`
	Series   string `json:"series"`
	Symbol   string `json:"symbol"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Interval string `json:"interval"`

	// FetchedAt and Endpoint are provenance: when the words were taken
	// and from where.
	FetchedAt string `json:"fetched_at"`
	Endpoint  string `json:"endpoint"`

	// Pages are the venue's response bodies, verbatim, in fetch order.
	Pages []json.RawMessage `json:"pages"`
}

// Grid returns the file's declared closed identity as a grid.
func (f DatasetFile) Grid() (SettlementGrid, error) {
	start, err := time.Parse(time.RFC3339, f.Start)
	if err != nil {
		return SettlementGrid{}, fmt.Errorf("dataset start: %w", err)
	}
	end, err := time.Parse(time.RFC3339, f.End)
	if err != nil {
		return SettlementGrid{}, fmt.Errorf("dataset end: %w", err)
	}
	interval, err := time.ParseDuration(f.Interval)
	if err != nil {
		return SettlementGrid{}, fmt.Errorf("dataset interval: %w", err)
	}
	g := SettlementGrid{Start: start, End: end, Interval: interval}
	if err := g.Validate(); err != nil {
		return SettlementGrid{}, err
	}
	return g, nil
}

// rawRow is one venue row before normalization: the venue's instant and
// the venue's decimal string for the value.
type rawRow struct {
	at    time.Time
	value string
}

// extractRows parses every page under the (venue, series) dialect and
// returns rows sorted ascending by the venue's own instants. Sorting is a
// validation convenience — Bybit pages run newest-first — and touches
// only the extraction; the file keeps the venue's order.
func extractRows(f DatasetFile) ([]rawRow, error) {
	var rows []rawRow
	for n, page := range f.Pages {
		parsed, err := parsePage(f.Venue, f.Series, page)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", n, err)
		}
		rows = append(rows, parsed...)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].at.Before(rows[j].at) })
	return rows, nil
}

func parsePage(venue, series string, page json.RawMessage) ([]rawRow, error) {
	switch {
	case venue == "binance" && series == SeriesFunding:
		var rows []struct {
			FundingTime int64  `json:"fundingTime"`
			FundingRate string `json:"fundingRate"`
		}
		if err := json.Unmarshal(page, &rows); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPageMalformed, err)
		}
		out := make([]rawRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, rawRow{at: time.UnixMilli(r.FundingTime).UTC(), value: r.FundingRate})
		}
		return out, nil

	case venue == "bybit" && series == SeriesFunding:
		var body struct {
			RetCode int `json:"retCode"`
			Result  struct {
				List []struct {
					FundingRate          string `json:"fundingRate"`
					FundingRateTimestamp string `json:"fundingRateTimestamp"`
				} `json:"list"`
			} `json:"result"`
		}
		if err := json.Unmarshal(page, &body); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPageMalformed, err)
		}
		if body.RetCode != 0 {
			return nil, fmt.Errorf("retCode %d: %w", body.RetCode, ErrPageMalformed)
		}
		out := make([]rawRow, 0, len(body.Result.List))
		for _, r := range body.Result.List {
			ms, err := strconv.ParseInt(r.FundingRateTimestamp, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("timestamp %q: %w", r.FundingRateTimestamp, ErrPageMalformed)
			}
			out = append(out, rawRow{at: time.UnixMilli(ms).UTC(), value: r.FundingRate})
		}
		return out, nil

	case venue == "binance" && (series == SeriesSpotKline1h || series == SeriesMarkKline1h):
		// Klines are arrays of arrays: index 0 is the open time (ms),
		// index 1 the open price string — the price observable AT the
		// row's instant, per the calibrate.KlinePoint convention.
		var rows [][]json.RawMessage
		if err := json.Unmarshal(page, &rows); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPageMalformed, err)
		}
		out := make([]rawRow, 0, len(rows))
		for n, r := range rows {
			if len(r) < 2 {
				return nil, fmt.Errorf("kline %d has %d fields: %w", n, len(r), ErrPageMalformed)
			}
			var ms int64
			if err := json.Unmarshal(r[0], &ms); err != nil {
				return nil, fmt.Errorf("kline %d open time: %w: %w", n, ErrPageMalformed, err)
			}
			var open string
			if err := json.Unmarshal(r[1], &open); err != nil {
				return nil, fmt.Errorf("kline %d open: %w: %w", n, ErrPageMalformed, err)
			}
			out = append(out, rawRow{at: time.UnixMilli(ms).UTC(), value: open})
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s/%s: %w", venue, series, ErrSeriesUnknown)
}

// normalizeRows maps venue instants onto the declared grid within
// GridJitterTolerance, refuses a row no grid point claims or two rows
// claiming one point, and hands the normalized instants to CheckComplete
// — so a dataset that passes here IS its closed identity: every grid
// instant present exactly once, nothing extra, jitter recorded in the
// venue's bytes but erased from the identity.
func normalizeRows(rows []rawRow, g SettlementGrid) ([]rawRow, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	out := make([]rawRow, 0, len(rows))
	seen := make(map[time.Time]bool, len(rows))
	for _, r := range rows {
		grid, err := nearestGridInstant(r.at, g)
		if err != nil {
			return nil, err
		}
		if seen[grid] {
			return nil, fmt.Errorf("rows at %s: %w", grid.Format(time.RFC3339), ErrRowDuplicate)
		}
		seen[grid] = true
		out = append(out, rawRow{at: grid, value: r.value})
	}
	instants := make([]time.Time, len(out))
	for n, r := range out {
		instants[n] = r.at
	}
	if err := CheckComplete(g, instants); err != nil {
		return nil, err
	}
	return out, nil
}

// nearestGridInstant rounds a venue instant to its grid point, refusing
// one farther than the tolerance in either direction. The tolerance is
// inclusive at its boundary — a row exactly GridJitterTolerance away is
// still its grid point's row.
func nearestGridInstant(at time.Time, g SettlementGrid) (time.Time, error) {
	offset := at.Sub(g.Start)
	k := (offset + g.Interval/2) / g.Interval
	grid := g.Start.Add(k * g.Interval)
	drift := at.Sub(grid)
	if drift < 0 {
		drift = -drift
	}
	if drift > GridJitterTolerance {
		return time.Time{}, fmt.Errorf("row at %s, nearest grid %s: %w",
			at.Format(time.RFC3339Nano), grid.Format(time.RFC3339), ErrRowOffGrid)
	}
	return grid, nil
}

// FundingSettlements extracts, normalizes and types a funding dataset:
// every row H1-guarded, the venue's rate parsed at the canonical scale,
// provenance stamped from the row's own instant. This is the door the
// derivations read history through.
func FundingSettlements(f DatasetFile, inst ids.VenueInstrument) ([]domain.RealizedSettlement, error) {
	if f.Series != SeriesFunding {
		return nil, fmt.Errorf("series %q: %w", f.Series, ErrSeriesUnknown)
	}
	g, err := f.Grid()
	if err != nil {
		return nil, err
	}
	rows, err := extractRows(f)
	if err != nil {
		return nil, err
	}
	rows, err = normalizeRows(rows, g)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RealizedSettlement, 0, len(rows))
	for _, r := range rows {
		if err := GuardCalibrationInput(r.at); err != nil {
			return nil, err
		}
		rate, err := fixedpoint.ParseRate(r.value)
		if err != nil {
			return nil, fmt.Errorf("rate %q at %s: %w", r.value, r.at.Format(time.RFC3339), err)
		}
		s := domain.RealizedSettlement{
			Instrument: inst,
			At:         r.at,
			Rate:       rate,
			Watermark:  mdomain.Watermark{SourceTime: r.at, RecvTime: r.at},
		}
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("settlement at %s: %w", r.at.Format(time.RFC3339), err)
		}
		out = append(out, s)
	}
	return out, nil
}

// KlinePoints extracts, normalizes and types a kline dataset: every row
// H1-guarded, the venue's open price parsed against the given asset
// pair.
func KlinePoints(f DatasetFile, base, quote fixedpoint.Asset) ([]calibrate.KlinePoint, error) {
	if f.Series != SeriesSpotKline1h && f.Series != SeriesMarkKline1h {
		return nil, fmt.Errorf("series %q: %w", f.Series, ErrSeriesUnknown)
	}
	g, err := f.Grid()
	if err != nil {
		return nil, err
	}
	rows, err := extractRows(f)
	if err != nil {
		return nil, err
	}
	rows, err = normalizeRows(rows, g)
	if err != nil {
		return nil, err
	}
	out := make([]calibrate.KlinePoint, 0, len(rows))
	for _, r := range rows {
		if err := GuardCalibrationInput(r.at); err != nil {
			return nil, err
		}
		price, err := fixedpoint.ParsePrice(base, quote, r.value)
		if err != nil {
			return nil, fmt.Errorf("price %q at %s: %w", r.value, r.at.Format(time.RFC3339), err)
		}
		out = append(out, calibrate.KlinePoint{At: r.at, Price: price})
	}
	return out, nil
}
