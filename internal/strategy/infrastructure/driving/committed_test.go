package driving_test

// The committed family-1 datasets, bound to the production loaders. These
// four files are write-once (§41c): the artifact will cite their digests,
// so an edit — however innocent — would orphan every citation. Pinning
// digest, row count and era bounds here makes the write-once rule a
// build break instead of a review hope.
//
// The kline era starts 2023-04-01, NOT the funding era's 2023-01-01: the
// venue served no spot kline for 2023-03-24T13:00Z (the spot halt of
// that day), and a closed identity cannot claim an instant the venue
// never served — the completeness refusal fired live on the full range,
// which is H3 doing its job. The funding series has no such hole.

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

func TestCommittedDatasetsLoadAndPin(t *testing.T) {
	binPerp := fetchPerp
	byPerp := ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}

	cases := []struct {
		path   string
		kind   string
		inst   ids.VenueInstrument
		digest string
		rows   int
		first  time.Time
		last   time.Time
	}{
		{
			path: "../../../../datasets/binance/funding.BTCUSDT.2023-01-01.2026-01-31.json",
			kind: "funding", inst: binPerp,
			digest: "737cf6d3339d0330af96a862ce0ba6f986a8469cad7070929fd2c92a8ab3a8a4",
			rows:   3381,
			first:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			last:   time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC),
		},
		{
			path: "../../../../datasets/bybit/funding.BTCUSDT.2023-01-01.2026-01-31.json",
			kind: "funding", inst: byPerp,
			digest: "0c5b35f55df49d210e98f5736b368d68035459808e0d7b548834f36268980022",
			rows:   3381,
			first:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			last:   time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC),
		},
		{
			path: "../../../../datasets/binance/spot_kline_1h.BTCUSDT.2023-04-01.2026-01-31.json",
			kind: "kline", inst: binPerp,
			digest: "823d5053bae27e86dadbf4086795df48e4439f16a44722f2f7ca56907c851241",
			rows:   24888,
			first:  time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC),
			last:   time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC),
		},
		{
			path: "../../../../datasets/binance/mark_kline_1h.BTCUSDT.2023-04-01.2026-01-31.json",
			kind: "kline", inst: binPerp,
			digest: "19a18c387118cd14ad7b92aeccf055c567bceb47f1c428a2050e4d473bddaca7",
			rows:   24888,
			first:  time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC),
			last:   time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC),
		},
	}

	for _, c := range cases {
		f, digest, err := driving.LoadDatasetFile(c.path)
		if err != nil {
			t.Errorf("%s: %v", c.path, err)
			continue
		}
		if digest != c.digest {
			t.Errorf("%s digest %s; committed as %s — these files are write-once", c.path, digest, c.digest)
		}
		var first, last time.Time
		var n int
		switch c.kind {
		case "funding":
			rows, err := driving.FundingSettlements(f, c.inst)
			if err != nil {
				t.Errorf("%s rows: %v", c.path, err)
				continue
			}
			n, first, last = len(rows), rows[0].At, rows[len(rows)-1].At
		case "kline":
			rows, err := driving.KlinePoints(f, "BTC", "USDT")
			if err != nil {
				t.Errorf("%s rows: %v", c.path, err)
				continue
			}
			n, first, last = len(rows), rows[0].At, rows[len(rows)-1].At
		}
		if n != c.rows {
			t.Errorf("%s: %d rows; want %d", c.path, n, c.rows)
		}
		if !first.Equal(c.first) || !last.Equal(c.last) {
			t.Errorf("%s spans %s..%s; want %s..%s", c.path, first, last, c.first, c.last)
		}
	}
}
