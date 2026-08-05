package venue

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's spot balance surface (Profiles §4's dated 2026-08-05
// block; task #179, record §70d).
//
// # The weight is 20, and the number is a CLAIM the governor charges verbatim
//
// GET /api/v3/account is request weight 20 on the spot family's weight budget
// — verified against the raw markdown of binance/binance-spot-api-docs by
// exact grep, after a summarized fetch misread it as 10. The governor charges
// exactly the caller-stated cost, so an understated weight admits past the
// venue's budget and the failure is the venue's ban, not a local refusal.
//
// # omitZeroBalances is deliberately left UNSET
//
// The parameter defaults to false, and false is the setting under which every
// asset comes back — so a row's absence carries no conclusion and none is
// needed. Sending true would make absence mean zero, which is a semantic this
// codec would then share with Bybit's, except enforced by a parameter one
// refactor away from being dropped. Not sending the parameter at all keeps the
// wire the documented default rather than a choice that can drift.

// The two verbatim field names a Binance spot balance row derives its total
// from. Exported because the baseline document stores words under these names
// and the loader re-derives through BinanceSpotTotal, which requires them.
const (
	BinanceSpotWordFree   = "free"
	BinanceSpotWordLocked = "locked"
)

// BinanceSpotTotal derives one asset's whole holding from Binance's verbatim
// words: total = free + locked.
//
// This is the ONE place the two fields' meaning lives (record §70d): the live
// decode below and the baseline loader both call it, so a live read and a
// boot-time recomputation from stored words cannot disagree about what the
// fields mean. The sum is the venue's word, not an inference — `free` is
// "Amount not currently committed to an open order" and `locked` is "Amount
// currently committed to an open order", documented partitions of holdings.
func BinanceSpotTotal(asset fixedpoint.Asset, words []SpotBalanceWord) (fixedpoint.Quantity, error) {
	values, err := spotWordValues(words, BinanceSpotWordFree, BinanceSpotWordLocked)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("binance spot %s: %w", asset, err)
	}
	free, err := fixedpoint.ParseQuantity(asset, values[0])
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("binance spot %s free %q: %w", asset, values[0], err)
	}
	locked, err := fixedpoint.ParseQuantity(asset, values[1])
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("binance spot %s locked %q: %w", asset, values[1], err)
	}
	total, err := free.Add(locked)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("binance spot %s: %w", asset, err)
	}
	return total, nil
}

// EncodeSpotBalances implements SpotBalanceCodec.
func (c *BinanceOrderCodec) EncodeSpotBalances() (transport.Call, error) {
	return transport.Call{
		Op:     string(c.venue) + ".SpotBalances",
		Method: "GET",
		Path:   "/api/v3/account",
		Family: ids.ProductSpot,
		Cost:   ratelimit.Cost{Weight: 20},
	}, nil
}

// binanceAccountWire is the account answer, narrowed to the rows this surface
// reads. The full response carries commission rates, permissions, uid and an
// updateTime none of which is a balance.
type binanceAccountWire struct {
	Balances []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	} `json:"balances"`
}

// DecodeSpotBalances implements SpotBalanceCodec.
//
// An EMPTY balances list is refused rather than read as an empty wallet: with
// omitZeroBalances left false the venue answers with every listed asset —
// hundreds of zero rows on any real account — so a body with none is a
// wrong-shaped answer (or the parameter's semantics changed under us), and an
// empty wallet is exactly the conclusion §5.3's absence rule says must never
// be inferred from an answer that may not be one.
func (c *BinanceOrderCodec) DecodeSpotBalances(resp transport.Response) (SpotBalanceReport, error) {
	var wire binanceAccountWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return SpotBalanceReport{}, fmt.Errorf("binance spot balances: %w", err)
	}
	if len(wire.Balances) == 0 {
		return SpotBalanceReport{}, fmt.Errorf("binance spot balances answer holds no rows: %w",
			ErrSpotBalancesUnreadable)
	}

	var report SpotBalanceReport
	seen := make(map[string]struct{}, len(wire.Balances))
	for i, row := range wire.Balances {
		if row.Asset == "" {
			return SpotBalanceReport{}, fmt.Errorf("binance spot balance %d carries no asset: %w",
				i, ErrSpotBalancesUnreadable)
		}
		if _, dup := seen[row.Asset]; dup {
			// Two rows for one asset: summing them would be an aggregation
			// rule the venue does not document, and taking either would drop
			// half the holding. The positions decoder's refusal, one surface
			// over.
			return SpotBalanceReport{}, fmt.Errorf("binance answers twice for %s: %w",
				row.Asset, ErrSpotBalancesUnreadable)
		}
		seen[row.Asset] = struct{}{}

		asset := fixedpoint.Asset(row.Asset)
		words := []SpotBalanceWord{
			{Name: BinanceSpotWordFree, Value: row.Free},
			{Name: BinanceSpotWordLocked, Value: row.Locked},
		}
		total, err := BinanceSpotTotal(asset, words)
		if err != nil {
			return SpotBalanceReport{}, fmt.Errorf("binance spot balance %d: %w", i, err)
		}
		report.Rows = append(report.Rows, SpotBalanceRow{Asset: asset, Total: total, Words: words})
	}

	sortSpotBalanceReport(&report)
	return report, nil
}

// sortSpotBalanceReport orders both halves by asset. Shared with the Bybit
// codec for sortPositionReport's reason: the consumer turns this answer into
// an operator-facing document and report, and venue listing order is not an
// order anyone can re-derive.
func sortSpotBalanceReport(r *SpotBalanceReport) {
	sort.Slice(r.Rows, func(i, j int) bool { return r.Rows[i].Asset < r.Rows[j].Asset })
	sort.Slice(r.Unpinned, func(i, j int) bool { return r.Unpinned[i].Asset < r.Unpinned[j].Asset })
}

// Compile-time proof that the codec satisfies the balance half too (§3.6
// rule 2).
var _ SpotBalanceCodec = (*BinanceOrderCodec)(nil)
