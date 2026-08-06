package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit UTA's posture surface (Profiles §2's assert-or-refuse row;
// task #198, record §73).
//
// # The wallet is read AGAIN here, and that is not a duplicate call by accident
//
// SpotBalances already reads GET /v5/account/wallet-balance, and this surface
// reads it a second time. Sharing one read was considered and rejected, because
// the two consumers need OPPOSITE treatments of the same missing field
// (record §73b F2):
//
//	balances_bybit.go:193   an empty borrowAmount is "the venue stated no
//	                        borrow figure" and the row still PINS — correct
//	                        there, because a wallet with nothing to borrow must
//	                        not turn every balance read into a failure
//	here                    an empty borrowAmount means the claim "borrow is
//	                        zero" HAS NO EVIDENCE, and the posture refuses
//
// That file's own comment nominates this one as the owner of the strict
// reading. So bybitCoinBorrowed is deliberately NOT called below: reusing it
// would import its permissive arm into the single gate whose purpose is
// refusing borrow, and the system would then pass its borrow check on a
// response that said nothing about borrow. Two folds over one record, opposite
// safe directions.
//
// The cost of the second read is one request against a 50/s per-UID budget,
// which is the cheap half of the trade.
//
// # unifiedMarginStatus is a WHITELIST
//
// Profiles §2 admits {3,4,5,6} and refuses classic 1. The value 2 is unlisted,
// and a value the venue adds next year is unlisted by construction. This codec
// reports the number verbatim and the domain decides — but the rule there is
// admit-listed-only, because the alternative refuses a known-bad set and
// accepts everything nobody has thought about, which on this setting means
// trading an account whose collateral model we have not established.
//
// # Which field names are verified
//
// Profiles §2 states, and these are taken as given: the four paths and the
// names `marginMode`, `unifiedMarginStatus`, `spotHedgingStatus`,
// `borrowAmount`, `spotBorrow`, `positionIdx`. The rest — the spot-margin
// status response key, position/list's `leverage` and `autoAddMargin` keys —
// is this codec's best reading, unmet by any real account because credentials
// are deferred (record §73f). A wrong key decodes as PostureAbsent, which the
// domain refuses by name: the failure direction is a refused boot, not a
// silently passed posture.

// The fold names for this surface's two new Bybit endpoints live in the
// transport package beside their siblings — transport.BybitAccountInfoLimit and
// transport.BybitSpotMarginStatusLimit — because bybitEndpointRegistry is the
// roster #186's boot check reads and transport cannot import this package.

// The settleCoin the linear position enumeration is scoped by. Bybit refuses a
// linear position query carrying neither symbol nor settleCoin, which is the
// property positions.go relies on: a scope this system got wrong comes back as
// an error rather than as an empty list that looks flat.
const bybitPostureSettleCoin = "USDT"

// EncodePosture implements AccountPostureCodec.
//
// One Bybit posture read is four requests against four separate per-endpoint
// budgets — none of them weighted, all metered per UID (§73c-2's quotable
// cost).
func (c *BybitOrderCodec) EncodePosture() ([]PostureStep, error) {
	op := string(c.venue) + ".AccountPosture"

	// Routed through bybitCategory rather than a literal "linear", so the
	// posture read and the order path cannot disagree about the word — the
	// same reason positions_bybit.go routes it.
	category, err := bybitCategory(ids.ProductLinearPerp)
	if err != nil {
		return nil, fmt.Errorf("bybit posture: %w", err)
	}

	return []PostureStep{
		{
			Probe: PostureProbeAccountInfo,
			Call: transport.Call{
				Op:     op + ".AccountInfo",
				Method: "GET",
				Path:   "/v5/account/info",
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.RequestOnly(),
			},
		},
		{
			Probe: PostureProbeSpotMargin,
			Call: transport.Call{
				Op:     op + ".SpotMargin",
				Method: "GET",
				Path:   "/v5/spot-margin-uta/status",
				Family: ids.ProductSpot,
				Cost:   ratelimit.RequestOnly(),
			},
		},
		{
			Probe: PostureProbeWallet,
			Call: transport.Call{
				Op:     op + ".Wallet",
				Method: "GET",
				Path:   "/v5/account/wallet-balance",
				Params: transport.Params{
					{Key: "accountType", Value: bybitUnifiedAccountType},
					// coin deliberately absent, as on SpotBalances: the
					// coin-less call enumerates, and the borrow claim is
					// about EVERY coin.
				},
				Family: ids.ProductSpot,
				Cost:   ratelimit.RequestOnly(),
			},
		},
		{
			Probe: PostureProbePositionList,
			Call: transport.Call{
				Op:     op + ".PositionList",
				Method: "GET",
				Path:   "/v5/position/list",
				Params: transport.Params{
					{Key: "category", Value: category},
					{Key: "settleCoin", Value: bybitPostureSettleCoin},
				},
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.RequestOnly(),
			},
		},
	}, nil
}

// bybitAccountInfoWire is GET /v5/account/info.
type bybitAccountInfoWire struct {
	Result struct {
		MarginMode          *string `json:"marginMode"`
		UnifiedMarginStatus *int    `json:"unifiedMarginStatus"`
		SpotHedgingStatus   *string `json:"spotHedgingStatus"`
	} `json:"result"`
}

// bybitSpotMarginStatusWire is GET /v5/spot-margin-uta/status.
type bybitSpotMarginStatusWire struct {
	Result struct {
		SpotMarginMode *string `json:"spotMarginMode"`
	} `json:"result"`
}

// bybitPostureWalletWire is the wallet read, narrowed to the borrow figures.
//
// POINTER fields, unlike bybitWalletCoinWire's plain strings, and that is the
// whole difference between the two reads: the balance surface cannot tell a
// missing borrowAmount from an empty one and does not need to, while here the
// distinction decides whether a required claim has any evidence at all.
type bybitPostureWalletWire struct {
	Result struct {
		List []struct {
			AccountType string `json:"accountType"`
			Coin        []struct {
				Coin         *string `json:"coin"`
				BorrowAmount *string `json:"borrowAmount"`
				SpotBorrow   *string `json:"spotBorrow"`
			} `json:"coin"`
		} `json:"list"`
	} `json:"result"`
}

// bybitPositionPostureWire is GET /v5/position/list, narrowed to the settings.
type bybitPositionPostureWire struct {
	Result struct {
		List []struct {
			Symbol        *string `json:"symbol"`
			PositionIdx   *int    `json:"positionIdx"`
			Leverage      *string `json:"leverage"`
			AutoAddMargin *int    `json:"autoAddMargin"`
		} `json:"list"`
	} `json:"result"`
}

// DecodePostureStep implements AccountPostureCodec.
func (c *BybitOrderCodec) DecodePostureStep(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	switch probe {
	case PostureProbeAccountInfo:
		var wire bybitAccountInfoWire
		if err := json.Unmarshal(resp.Body, &wire); err != nil {
			return nil, fmt.Errorf("bybit account info: %w", err)
		}
		return []PostureObservation{
			postureObservation(probe, PostureMarginMode, PostureScopeAccount, "", wire.Result.MarginMode),
			postureObservation(probe, PostureAccountKind, PostureScopeAccount, "",
				bybitIntWord(wire.Result.UnifiedMarginStatus)),
			postureObservation(probe, PostureSpotHedging, PostureScopeAccount, "", wire.Result.SpotHedgingStatus),
		}, nil

	case PostureProbeSpotMargin:
		var wire bybitSpotMarginStatusWire
		if err := json.Unmarshal(resp.Body, &wire); err != nil {
			return nil, fmt.Errorf("bybit spot margin status: %w", err)
		}
		return []PostureObservation{
			postureObservation(probe, PostureSpotMarginMode, PostureScopeAccount, "", wire.Result.SpotMarginMode),
		}, nil

	case PostureProbeWallet:
		return decodeBybitPostureWallet(probe, resp)

	case PostureProbePositionList:
		return decodeBybitPositionPosture(probe, resp)

	case PostureProbeUnspecified,
		PostureProbeMultiAssets, PostureProbeSymbolConfig,
		PostureProbePositionDual:
		// Binance's probes and the sentinel, named rather than defaulted
		// (§3.6 rule 10) — the mirror of the Binance codec's arm.
		return nil, fmt.Errorf("bybit cannot answer %s: %w", probe, ErrPostureUnreadable)
	}
	return nil, fmt.Errorf("bybit %s: %w", probe, ErrPostureProbeInvalid)
}

// bybitIntWord renders a wire integer as the venue's word without inventing one
// for a field the venue omitted.
//
// The pointer matters most on unifiedMarginStatus: Go's zero for int is 0,
// which is not in the admitted set {3,4,5,6} — so a missing field would refuse
// rather than pass, and this function would be merely tidy. It is written this
// way anyway because the NEXT integer setting added here may well have a
// passing zero, and the guarantee should not depend on which constants the
// venue happened to choose.
func bybitIntWord(v *int) *string {
	if v == nil {
		return nil
	}
	word := fmt.Sprintf("%d", *v)
	return &word
}

// decodeBybitPostureWallet reads the per-coin borrow figures.
//
// Note what is NOT here: any call to bybitCoinBorrowed, and any parsing of the
// borrow strings into quantities. This surface reports the venue's word and its
// presence; whether "0.00000000" is zero is a decision, and decisions are the
// domain's. Parsing here would also quietly re-introduce the permissive arm —
// an unparseable or empty value has to become SOMETHING to be compared, and
// every candidate for that something is a value the venue did not state.
func decodeBybitPostureWallet(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	var wire bybitPostureWalletWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, fmt.Errorf("bybit posture wallet: %w", err)
	}
	if len(wire.Result.List) != 1 {
		return nil, fmt.Errorf("bybit posture wallet answer holds %d accounts, want 1: %w",
			len(wire.Result.List), ErrPostureUnreadable)
	}
	account := wire.Result.List[0]
	if account.AccountType != bybitUnifiedAccountType {
		return nil, fmt.Errorf("bybit posture wallet answer is accountType %q, asked for %q: %w",
			account.AccountType, bybitUnifiedAccountType, ErrPostureUnreadable)
	}

	out := make([]PostureObservation, 0, len(account.Coin)*2)
	seen := make(map[string]struct{}, len(account.Coin))
	for i, row := range account.Coin {
		if row.Coin == nil || *row.Coin == "" {
			return nil, fmt.Errorf("bybit posture wallet row %d carries no coin: %w",
				i, ErrPostureUnreadable)
		}
		coin := *row.Coin
		if _, dup := seen[coin]; dup {
			return nil, fmt.Errorf("bybit answers twice for %s: %w", coin, ErrPostureUnreadable)
		}
		seen[coin] = struct{}{}

		out = append(out,
			postureObservation(probe, PostureBorrowAmount, PostureScopeAsset, coin, row.BorrowAmount))
		out = append(out,
			postureObservation(probe, PostureSpotBorrow, PostureScopeAsset, coin, row.SpotBorrow))
	}
	return out, nil
}

// decodeBybitPositionPosture reads positionIdx, leverage and auto-add-margin.
//
// # positionIdx is per-symbol here and account-wide on Binance, and that
// # asymmetry is the venue's, not a modelling choice
//
// Binance answers position mode once for the account (dualSidePosition);
// Bybit's one-way-ness is observable only as a property of position rows. So
// the same concern arrives at different scopes, and the domain's rule is
// written over the concern with both scopes admitted — a rule keyed to one
// scope would silently never fire on the other venue.
//
// An EMPTY list is a real answer: a flat account holds no positions, and Bybit
// returns rows for symbols with position records. It is NOT read as "one-way
// everywhere, no borrow, leverage fine" — absence produces no observation, and
// the domain's roster coverage check turns a configured symbol with no
// observation into a refusal (record §73b F4). That is the flat-account vacuity
// this slice was warned about by Profiles §2's own parenthetical, and it is
// also the case this codec cannot verify without a credential: whether Bybit
// returns a row for a configured-but-never-traded symbol is unknown here, and
// the conservative handling is correct under both answers.
//
// # A symbol answered TWICE is reported, not refused (record §73n F19)
//
// In HEDGE mode Bybit returns two rows per symbol, one per positionIdx — and
// hedge mode is a §4.1 posture VIOLATION, so the duplicate is evidence rather
// than a wire fault. Refusing it here converted a posture READ AND FOUND WRONG
// into one that could not be read at all, which §7.5 treats as retryable
// incompleteness; on Bybit no other probe names the position mode, so the one
// signal there is went out through the arm that says "nothing is known to be
// wrong". Both rows now travel and risk/domain reports ReasonAmbiguous — a
// latching per-symbol failure beside every other check. The original argument
// was against FOLDING two rows into one answer, which passing both does not do.
func decodeBybitPositionPosture(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	var wire bybitPositionPostureWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, fmt.Errorf("bybit position posture: %w", err)
	}

	out := make([]PostureObservation, 0, len(wire.Result.List)*3)
	for i, row := range wire.Result.List {
		if row.Symbol == nil || *row.Symbol == "" {
			return nil, fmt.Errorf("bybit position posture row %d carries no symbol: %w",
				i, ErrPostureUnreadable)
		}
		symbol := *row.Symbol

		out = append(out,
			postureObservation(probe, PosturePositionMode, PostureScopeSymbol, symbol,
				bybitIntWord(row.PositionIdx)))
		out = append(out,
			postureObservation(probe, PostureLeverage, PostureScopeSymbol, symbol, row.Leverage))
		out = append(out,
			postureObservation(probe, PostureAutoAddMargin, PostureScopeSymbol, symbol,
				bybitIntWord(row.AutoAddMargin)))
	}
	return out, nil
}

// Compile-time proof that the codec satisfies the posture half too (§3.6
// rule 2).
var _ AccountPostureCodec = (*BybitOrderCodec)(nil)
