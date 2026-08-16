package venue

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance USDⓈ-M's posture surface (Profiles §2's assert-or-refuse
// row; task #198, record §73).
//
// # Three probes, and Profiles §2 names a FOURTH check with no read path
//
// The row reads: multiAssetsMargin==false; symbolConfig.marginType==ISOLATED;
// **Portfolio-Margin OFF**; isAutoAddMargin==false;
// positionSide/dual.dualSidePosition==false; configured leverage matches pin.
// Every item names a GET except Portfolio-Margin, and no other section of
// either document supplies one (record §73b F1). It is therefore NOT a probe
// here and is NOT silently folded into a sibling's answer: Portfolio Margin is
// a different account TYPE rather than a setting inside the classic account, so
// no classic-account setting's value establishes anything about it. The gate
// consumes an operator attestation for that one check (§73c-1), and this codec
// stays honest about reading three things.
//
// All THREE per-symbol settings ride symbolConfig, which is why three probes
// answer six checks. Until 2026-08-06 auto-add-margin was a fourth probe against
// GET /fapi/v3/positionRisk, and record §73o is why it is not: V3 does not
// publish the field, and a POSITION enumeration cannot answer for a configured
// symbol holding no position in any case.
//
// # The weights are CONSERVATIVE where they are unverified, and say so
//
// NONE of the three charges rests on a published figure. Profiles records no
// weight for any of these endpoints, and developers.binance.com answers every
// headless fetcher with a WAF challenge, so nobody here has read their lines.
// (Binance's spec-generated connector, which settled §73o's FIELD question, is
// no help on this one: it carries no weight annotations at all — a source
// answers the questions it answers.)
//
// Each probe is therefore charged 20 — the highest weight this project has
// recorded on any USDⓈ-M account read. The direction is chosen the way §7.6's
// are: an UNDER-charge admits past the venue's budget and the failure is the
// venue's ban, which is the outcome the whole governor exists to avoid; an
// over-charge costs local headroom and refuses early, by name, where an operator
// can see it. Reading any of these lines is a LOOSENING trigger, never a
// correctness prerequisite.
//
// # The wire field names below are only PARTLY verified, and this is the honest
// # statement of which
//
// Profiles §2 states, and these are taken as given: the three paths, and the
// field names `marginType`, `isAutoAddMargin` and `dualSidePosition`. That
// `isAutoAddMargin` is published by symbolConfig rather than by positionRisk is
// this project's ONE fact sourced from Binance's spec-generated connector alone
// (record §73o), and the direction is why it was landable before a credential:
// if that source is wrong, the field decodes as absent and the gate REFUSES,
// which is what it did before. Everything else on the wire structs — the
// multiAssetsMargin response key, symbolConfig's `leverage` and `symbol` keys —
// is this codec's best reading and has never met a real account, because
// credentials are deferred (record §73f). A wrong key decodes as an ABSENT
// field, which this surface reports as PostureAbsent and the domain refuses: the
// failure direction is a refused boot naming the setting, not a posture silently
// passed. That is why the pointer wire fields in this file are load-bearing
// rather than fastidious.

// binanceUnverifiedWeight is charged for every posture probe, none of whose
// published weights anybody here has read. See the file comment for the
// direction's argument.
const binanceUnverifiedWeight = 20

// EncodePosture implements AccountPostureCodec.
//
// The order is account-wide first, then per-symbol, which is the order the
// refusal reads in and the order §73c-2's quotable cost is measured in: one
// Binance posture read is three calls and 60 weight against the futures budget.
//
// The per-symbol half is ONE call, and it has to be an unscoped read of a
// surface that enumerates every symbol rather than a position enumeration —
// record §73o. A symbol-scoped read could only ask about symbols this system
// already names, and the roster coverage check needs to see what the venue
// actually returned in order to notice a configured symbol MISSING from it.
//
// # So the symbols argument is deliberately unused HERE, and that is an
// # asymmetry between the codecs rather than a default
//
// The paragraph above holds on this venue because `/fapi/v1/symbolConfig`
// volunteers a row for every symbol the account has configured, traded or not:
// the enumeration exists, so the coverage check can read it and the three
// per-symbol settings are observable on a flat account from one call.
//
// Bybit has no such surface. Its position read answers only for symbols
// carrying an OPEN POSITION, so the same design left §4.1's three per-symbol
// rows unobservable on a flat account — and there the symbol-scoped read is the
// one that surfaces an unknown symbol, by name, which is the opposite of the
// trade-off above. Record §97 §4 has the measurement. Two codecs that look
// symmetric, resting on evidence that is not: read neither one's reasoning as
// the other's.
func (c *BinanceOrderCodec) EncodePosture(_ []string) ([]PostureStep, error) {
	op := string(c.venue) + ".AccountPosture"
	return []PostureStep{
		{
			Probe: PostureProbeMultiAssets,
			Call: transport.Call{
				Op:     op + ".MultiAssets",
				Method: "GET",
				Path:   "/fapi/v1/multiAssetsMargin",
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.Cost{Weight: binanceUnverifiedWeight},
			},
			Answers: []PostureSetting{PostureMultiAssets},
		},
		{
			Probe: PostureProbePositionDual,
			Call: transport.Call{
				Op:     op + ".PositionDual",
				Method: "GET",
				Path:   "/fapi/v1/positionSide/dual",
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.Cost{Weight: binanceUnverifiedWeight},
			},
			Answers: []PostureSetting{PosturePositionMode},
		},
		{
			Probe: PostureProbeSymbolConfig,
			Call: transport.Call{
				Op:     op + ".SymbolConfig",
				Method: "GET",
				Path:   "/fapi/v1/symbolConfig",
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.Cost{Weight: binanceUnverifiedWeight},
			},
			// The three-for-one call, and the reason the probe count is three
			// rather than six (record §73o).
			Answers: []PostureSetting{
				PostureMarginMode,
				PostureLeverage,
				PostureAutoAddMargin,
			},
		},
	}, nil
}

// binanceMultiAssetsWire is GET /fapi/v1/multiAssetsMargin.
type binanceMultiAssetsWire struct {
	MultiAssetsMargin *bool `json:"multiAssetsMargin"`
}

// binancePositionDualWire is GET /fapi/v1/positionSide/dual.
type binancePositionDualWire struct {
	DualSidePosition *bool `json:"dualSidePosition"`
}

// binanceSymbolConfigWire is one row of GET /fapi/v1/symbolConfig — all three
// of §4.1's per-symbol settings.
//
// The two auto-add-margin fields are json.RawMessage rather than typed pointers
// because Binance types this ONE concern two ways across its own surfaces, which
// is documented rather than suspected: its spec-generated connector declares
// `isAutoAddMargin` a **bool** on symbolConfig and a **string** on
// positionRisk V2. A typed pointer would make the other typing a decode ERROR
// for the whole read — refusing the account-wide answers already in hand over a
// per-symbol field's JSON type, which is the misclassification record §73n F19
// is about, one layer down. Raw defers the question to the one function that
// can state it.
type binanceSymbolConfigWire struct {
	Symbol     *string `json:"symbol"`
	MarginType *string `json:"marginType"`
	Leverage   *int    `json:"leverage"`

	// Both spellings are read for the reason binanceAutoAddMarginWord gives.
	// They are distinct JSON keys, so encoding/json's case-insensitive
	// fallback cannot make either one's presence depend on key order.
	IsAutoAddMargin  json.RawMessage `json:"isAutoAddMargin"`
	AutoAddMarginAlt json.RawMessage `json:"autoAddMargin"`
}

// DecodePostureStep implements AccountPostureCodec.
func (c *BinanceOrderCodec) DecodePostureStep(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	switch probe {
	case PostureProbeMultiAssets:
		var wire binanceMultiAssetsWire
		if err := json.Unmarshal(resp.Body, &wire); err != nil {
			return nil, fmt.Errorf("binance multi-assets margin: %w", err)
		}
		return []PostureObservation{
			postureBool(probe, PostureMultiAssets, PostureScopeAccount, "", wire.MultiAssetsMargin),
		}, nil

	case PostureProbePositionDual:
		var wire binancePositionDualWire
		if err := json.Unmarshal(resp.Body, &wire); err != nil {
			return nil, fmt.Errorf("binance position mode: %w", err)
		}
		return []PostureObservation{
			postureBool(probe, PosturePositionMode, PostureScopeAccount, "", wire.DualSidePosition),
		}, nil

	case PostureProbeSymbolConfig:
		return decodeBinanceSymbolConfig(probe, resp)

	case PostureProbeUnspecified,
		PostureProbeAccountInfo, PostureProbeSpotMargin,
		PostureProbeWallet, PostureProbePositionList:
		// Bybit's probes and the sentinel, named rather than defaulted
		// (§3.6 rule 10): a probe added for the other venue has to choose a
		// side here, and a `default` would let it fall through to a refusal
		// that reads like a venue error rather than a wiring one.
		return nil, fmt.Errorf("binance cannot answer %s: %w", probe, ErrPostureUnreadable)
	}
	return nil, fmt.Errorf("binance %s: %w", probe, ErrPostureProbeInvalid)
}

// decodeBinanceSymbolConfig reads all three per-symbol settings: margin type,
// leverage and auto-add-margin.
//
// An EMPTY array is refused rather than read as "no symbols configured": the
// call is unscoped, a futures account has symbol configuration for every symbol
// it can trade, and an empty answer is a wrong-shaped one. Read as an empty
// roster it would produce a report with no per-symbol observations at all,
// against which every per-symbol rule folds vacuously — the flat-account
// vacuity of record §73b F4, arriving through the response instead of the
// account.
//
// # A symbol answered TWICE is still refused here, and that is not F19 reopened
//
// Record §73n F19 folded the two POSITION enumerations to report duplicate rows
// rather than refuse, because a hedge-mode account answers with a LONG row and
// a SHORT row and that duplicate is EVIDENCE of a §4.1 violation. symbolConfig
// is not a position enumeration: it holds one row per symbol whatever the
// position mode is, so a second row for one symbol is a wrong-shaped answer with
// no documented venue behaviour behind it, and the two rows could disagree about
// the very setting being asserted. F19 kept this refusal deliberately, and
// moving auto-add-margin onto this surface does not change what a duplicate
// HERE means. Binance hedge mode is still named directly, and account-wide, by
// the dualSidePosition probe.
func decodeBinanceSymbolConfig(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	var rows []binanceSymbolConfigWire
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return nil, fmt.Errorf("binance symbol config: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("binance symbol config answer holds no rows: %w", ErrPostureUnreadable)
	}

	out := make([]PostureObservation, 0, len(rows)*3)
	seen := make(map[string]struct{}, len(rows))
	for i, row := range rows {
		if row.Symbol == nil || *row.Symbol == "" {
			return nil, fmt.Errorf("binance symbol config row %d carries no symbol: %w",
				i, ErrPostureUnreadable)
		}
		symbol := *row.Symbol
		if _, dup := seen[symbol]; dup {
			// Two rows for one symbol: taking either is a rule the venue
			// does not document, and the two could disagree about the very
			// setting being asserted.
			return nil, fmt.Errorf("binance answers twice for %s: %w", symbol, ErrPostureUnreadable)
		}
		seen[symbol] = struct{}{}

		autoAdd, err := binanceAutoAddMarginWord(symbol, row)
		if err != nil {
			return nil, err
		}

		out = append(out,
			postureObservation(probe, PostureMarginMode, PostureScopeSymbol, symbol, row.MarginType))
		out = append(out,
			postureObservation(probe, PostureLeverage, PostureScopeSymbol, symbol, binanceLeverageWord(row.Leverage)))
		out = append(out,
			postureObservation(probe, PostureAutoAddMargin, PostureScopeSymbol, symbol, autoAdd))
	}
	return out, nil
}

// binanceLeverageWord renders the integer leverage as the venue's word without
// inventing one for a row that omitted it.
//
// Binance states leverage as a JSON number and Bybit as a string, so the two
// venues' words differ in type for one concern. They are normalised to a
// decimal string HERE rather than in the domain, because a rule comparing a
// configured pin against "whatever type this venue used" is a rule with a
// per-venue branch inside it — and the pin is one operator-declared number.
func binanceLeverageWord(v *int) *string {
	if v == nil {
		return nil
	}
	word := fmt.Sprintf("%d", *v)
	return &word
}

// binanceAutoAddMarginWord reconciles the two spellings of auto-add-margin,
// refusing a row that states both and disagrees.
//
// # Why BOTH spellings are read
//
// Profiles §2 names the field `isAutoAddMargin` and records it as read-only with
// no REST setter. Binance's account surfaces spell this one concern two ways and
// type it two ways, and this codec has never met a real account (record §73f).
// Reading only one spelling would make the other decode as ABSENT, which the
// domain refuses, so the failure is loud either way; reading both and refusing a
// CONTRADICTION is strictly better, because the case where the venue answers
// both and they disagree is the one where picking a spelling silently picks an
// answer.
//
// # Why the TYPE is not fixed either (record §73o)
//
// This is not a hedge against an imagined venue. Binance's own spec-generated
// connector declares `isAutoAddMargin` a **bool** on `/fapi/v1/symbolConfig` and
// a **string** on `/fapi/v2/positionRisk` — one field name, one venue, two
// declared types. Accepting both here costs nothing that matters: any OTHER JSON
// shape is refused by name, so a venue answering `{}` or `1` does not decay into
// a missing field.
//
// The comparison folds case, because the domain's own word comparison does
// (expectWord, "ON" and "OFF" differ under any folding) and a stricter test HERE
// would refuse the whole read over two spellings of one agreeing answer — the
// same misclassification record §73n F19 is about, one field down. What is
// refused is a real disagreement, where picking a spelling silently picks an
// answer.
func binanceAutoAddMarginWord(symbol string, row binanceSymbolConfigWire) (*string, error) {
	fromPrimary, err := binanceBoolishWord(symbol, "isAutoAddMargin", row.IsAutoAddMargin)
	if err != nil {
		return nil, err
	}
	fromAlt, err := binanceBoolishWord(symbol, "autoAddMargin", row.AutoAddMarginAlt)
	if err != nil {
		return nil, err
	}
	switch {
	case fromPrimary == nil:
		return fromAlt, nil
	case fromAlt == nil:
		return fromPrimary, nil
	case !strings.EqualFold(*fromPrimary, *fromAlt):
		return nil, fmt.Errorf("binance %s states auto-add-margin as both %q and %q: %w",
			symbol, *fromPrimary, *fromAlt, ErrPostureUnreadable)
	default:
		return fromPrimary, nil
	}
}

// binanceBoolishWord reads one JSON value that the venue may state as a bool or
// as a string, and returns the venue's own word.
//
// A missing key and an explicit `null` both produce (nil, nil) — no observation,
// which the domain's roster coverage check turns into a named refusal. Any other
// shape is an ERROR rather than a nil: a field the venue answered in a shape
// this codec cannot read is a wire-shape change, and reporting it as "the venue
// did not say" would file it under the wrong failure reason and send an operator
// to the wrong document (record §73j — Unobserved and Unstated are different
// files to open).
func binanceBoolishWord(symbol, field string, raw json.RawMessage) (*string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		word := "false"
		if asBool {
			word = "true"
		}
		return &word, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		// Verbatim: the domain compares words and folds case, and a word
		// normalised here would hide which of them the venue actually sent.
		return &asString, nil
	}
	return nil, fmt.Errorf("binance %s states %s as %s, which is neither a bool nor a string: %w",
		symbol, field, raw, ErrPostureUnreadable)
}

// Compile-time proof that the codec satisfies the posture half too (§3.6
// rule 2).
var _ AccountPostureCodec = (*BinanceOrderCodec)(nil)
