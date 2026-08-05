package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit's spot balance surface (Profiles §4's dated 2026-08-05
// block; task #179, record §70d).
//
// # The `coin` parameter is NEVER passed, because the parameter decides what
// # silence means
//
// Verbatim: "If not passed, it returns non-zero asset info". The coin-less
// call is the enumeration kind whose absence carries a conclusion — an absent
// coin is a zero balance — which is exactly what the baseline assertion needs
// for an asset the venue no longer lists. A coin-scoped call's silence means
// nothing at all, so passing the parameter would flip the absence semantics
// while returning answers that look identical row by row. Profiles §9's
// version-is-part-of-the-requirement rule, arriving as a parameter.
//
// # The walletBalance/locked split is UNVERIFIED, and this codec refuses to bet
//
// No doc sentence states whether walletBalance includes locked. The equity
// formula ("Asset Equity = … = walletBalance − spotBorrow + unrealisedPnl +
// Asset Option Value") never adds locked, which IMPLIES inclusion — an
// inference, named in Profiles §4, not relied on. The total is therefore
// computed under BOTH readings and a row is pinned only where they coincide:
// locked == 0. A locked != 0 row is reported unpinned, the assertion for that
// asset concludes "incomplete, re-reportable", a resting order completes or
// cancels, and the next attempt pins. #171's testnet session is the LOOSENING
// trigger (measure walletBalance against a resting spot order) — never a
// correctness prerequisite.

// The two verbatim field names a Bybit spot balance row derives its total
// from. Exported for the same reason as Binance's: the baseline document
// stores words under these names and the loader re-derives via BybitSpotTotal.
const (
	BybitSpotWordWalletBalance = "walletBalance"
	BybitSpotWordLocked        = "locked"
)

// bybitUnifiedAccountType is the one account model this system trades on
// (Profiles §2) and the accountType this call both sends and requires echoed.
const bybitUnifiedAccountType = "UNIFIED"

// BybitSpotTotal derives one asset's holding from Bybit's verbatim words,
// under the both-readings rule.
//
// The ONE shared meaning (record §70d): with locked == 0 the two candidate
// readings of walletBalance coincide and the total IS walletBalance; with
// locked != 0 they differ by exactly locked, and the row is reported unpinned
// rather than resolved by betting on an inference. The zero SpotUnpinnedReason
// means the total is pinned.
func BybitSpotTotal(asset fixedpoint.Asset, words []SpotBalanceWord) (fixedpoint.Quantity, SpotUnpinnedReason, error) {
	values, err := spotWordValues(words, BybitSpotWordWalletBalance, BybitSpotWordLocked)
	if err != nil {
		return fixedpoint.Quantity{}, SpotUnpinnedReasonUnspecified, fmt.Errorf("bybit spot %s: %w", asset, err)
	}
	wallet, err := fixedpoint.ParseQuantity(asset, values[0])
	if err != nil {
		return fixedpoint.Quantity{}, SpotUnpinnedReasonUnspecified,
			fmt.Errorf("bybit spot %s walletBalance %q: %w", asset, values[0], err)
	}
	locked, err := fixedpoint.ParseQuantity(asset, values[1])
	if err != nil {
		return fixedpoint.Quantity{}, SpotUnpinnedReasonUnspecified,
			fmt.Errorf("bybit spot %s locked %q: %w", asset, values[1], err)
	}
	sign, err := locked.Sign()
	if err != nil {
		return fixedpoint.Quantity{}, SpotUnpinnedReasonUnspecified,
			fmt.Errorf("bybit spot %s locked %q: %w", asset, values[1], err)
	}
	if sign != 0 {
		return fixedpoint.Quantity{}, SpotUnpinnedLockedSplit, nil
	}
	return wallet, SpotUnpinnedReasonUnspecified, nil
}

// EncodeSpotBalances implements SpotBalanceCodec.
func (c *BybitOrderCodec) EncodeSpotBalances() (transport.Call, error) {
	return transport.Call{
		Op:     string(c.venue) + ".SpotBalances",
		Method: "GET",
		Path:   "/v5/account/wallet-balance",
		Params: transport.Params{
			{Key: "accountType", Value: bybitUnifiedAccountType},
			// coin is deliberately absent — see the file comment.
		},
		Family: ids.ProductSpot,

		// Bybit meters this endpoint per UID at 50/s (Profiles §4's Account
		// table row); there is no weight to charge. RequestOnly is that fact
		// with a carrier, the position surface's argument unchanged.
		Cost: ratelimit.RequestOnly(),
	}, nil
}

// bybitWalletCoinWire is one coin's row, narrowed to the fields this surface
// reads: the two words the total derives from and the two borrow figures the
// unpinned check is evidence-bound to.
type bybitWalletCoinWire struct {
	Coin          string `json:"coin"`
	WalletBalance string `json:"walletBalance"`
	Locked        string `json:"locked"`
	BorrowAmount  string `json:"borrowAmount"`
	SpotBorrow    string `json:"spotBorrow"`
}

// bybitWalletWire is the envelope: one account entry per accountType asked
// for, coins nested under it.
type bybitWalletWire struct {
	Result struct {
		List []struct {
			AccountType string                `json:"accountType"`
			Coin        []bybitWalletCoinWire `json:"coin"`
		} `json:"list"`
	} `json:"result"`
}

// DecodeSpotBalances implements SpotBalanceCodec.
//
// An empty COIN list is a real answer — "returns non-zero asset info", so an
// account holding nothing returns no rows and every balance is zero. An empty
// account LIST is not: the call named one accountType, and an answer that
// echoes no account for it is the position decoder's category-mismatch shape,
// refused rather than read as flat.
func (c *BybitOrderCodec) DecodeSpotBalances(resp transport.Response) (SpotBalanceReport, error) {
	var wire bybitWalletWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return SpotBalanceReport{}, fmt.Errorf("bybit spot balances: %w", err)
	}
	if len(wire.Result.List) != 1 {
		return SpotBalanceReport{}, fmt.Errorf("bybit spot balances answer holds %d accounts, want 1: %w",
			len(wire.Result.List), ErrSpotBalancesUnreadable)
	}
	account := wire.Result.List[0]
	if account.AccountType != bybitUnifiedAccountType {
		return SpotBalanceReport{}, fmt.Errorf("bybit spot balances answer is accountType %q, asked for %q: %w",
			account.AccountType, bybitUnifiedAccountType, ErrSpotBalancesUnreadable)
	}

	var report SpotBalanceReport
	seen := make(map[string]struct{}, len(account.Coin))
	for i, row := range account.Coin {
		if row.Coin == "" {
			return SpotBalanceReport{}, fmt.Errorf("bybit spot balance %d carries no coin: %w",
				i, ErrSpotBalancesUnreadable)
		}
		if _, dup := seen[row.Coin]; dup {
			return SpotBalanceReport{}, fmt.Errorf("bybit answers twice for %s: %w",
				row.Coin, ErrSpotBalancesUnreadable)
		}
		seen[row.Coin] = struct{}{}

		asset := fixedpoint.Asset(row.Coin)
		borrowed, err := bybitCoinBorrowed(asset, row)
		if err != nil {
			return SpotBalanceReport{}, fmt.Errorf("bybit spot balance %d: %w", i, err)
		}
		if borrowed {
			report.Unpinned = append(report.Unpinned, SpotUnpinnedRow{Asset: row.Coin, Reason: SpotUnpinnedBorrow})
			continue
		}

		words := []SpotBalanceWord{
			{Name: BybitSpotWordWalletBalance, Value: row.WalletBalance},
			{Name: BybitSpotWordLocked, Value: row.Locked},
		}
		total, unpinned, err := BybitSpotTotal(asset, words)
		if err != nil {
			return SpotBalanceReport{}, fmt.Errorf("bybit spot balance %d: %w", i, err)
		}
		if unpinned != SpotUnpinnedReasonUnspecified {
			report.Unpinned = append(report.Unpinned, SpotUnpinnedRow{Asset: row.Coin, Reason: unpinned})
			continue
		}
		report.Rows = append(report.Rows, SpotBalanceRow{Asset: asset, Total: total, Words: words})
	}

	sortSpotBalanceReport(&report)
	return report, nil
}

// bybitCoinBorrowed reports whether the row states an outstanding borrow —
// borrowAmount or spotBorrow non-zero. Borrowed coins inflate walletBalance
// without being holdings, so a stated non-zero figure unpins the row.
//
// An EMPTY string is read as "the venue stated no borrow figure", not as a
// refusal: this arm is evidence-bound (the unpin exists because a STATED
// borrow inflates the balance), the §4.1 posture check owns "borrow must be
// zero" as a boot refusal, and a field the venue omits for an account model
// with nothing to borrow must not turn every wallet read into a failed one.
// A NON-empty figure that will not parse is a decode error, not a zero.
func bybitCoinBorrowed(asset fixedpoint.Asset, row bybitWalletCoinWire) (bool, error) {
	for _, borrow := range []struct{ name, value string }{
		{"borrowAmount", row.BorrowAmount},
		{"spotBorrow", row.SpotBorrow},
	} {
		if borrow.value == "" {
			continue
		}
		q, err := fixedpoint.ParseQuantity(asset, borrow.value)
		if err != nil {
			return false, fmt.Errorf("%s %s %q: %w", asset, borrow.name, borrow.value, err)
		}
		sign, err := q.Sign()
		if err != nil {
			return false, fmt.Errorf("%s %s %q: %w", asset, borrow.name, borrow.value, err)
		}
		if sign != 0 {
			return true, nil
		}
	}
	return false, nil
}

// Compile-time proof that the codec satisfies the balance half too (§3.6
// rule 2).
var _ SpotBalanceCodec = (*BybitOrderCodec)(nil)
