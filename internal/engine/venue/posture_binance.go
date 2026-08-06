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
// # Four probes, and Profiles §2 names a FIFTH check with no read path
//
// The row reads: multiAssetsMargin==false; symbolConfig.marginType==ISOLATED;
// **Portfolio-Margin OFF**; positionRisk.isAutoAddMargin==false;
// positionSide/dual.dualSidePosition==false; configured leverage matches pin.
// Every item names a GET except Portfolio-Margin, and no other section of
// either document supplies one (record §73b F1). It is therefore NOT a probe
// here and is NOT silently folded into a sibling's answer: Portfolio Margin is
// a different account TYPE rather than a setting inside the classic account, so
// no classic-account setting's value establishes anything about it. The gate
// consumes an operator attestation for that one check (§73c-1), and this codec
// stays honest about reading four things.
//
// # The weights are CONSERVATIVE where they are unverified, and say so
//
// Only positionRisk's charge rests on a recorded figure, and even that one is
// the maximum over three disagreeing candidates (task #180, Profiles §9 —
// positions_binance.go carries the argument). The other three endpoints have no
// weight recorded in Profiles at all, and developers.binance.com answers every
// headless fetcher with a WAF challenge, so nobody here has read their lines.
//
// Each unverified probe is therefore charged 20 — the highest weight this
// project has recorded on any USDⓈ-M account read. The direction is chosen the
// way §7.6's are: an UNDER-charge admits past the venue's budget and the
// failure is the venue's ban, which is the outcome the whole governor exists to
// avoid; an over-charge costs local headroom and refuses early, by name, where
// an operator can see it. Reading any of these lines is a LOOSENING trigger,
// never a correctness prerequisite.
//
// # The wire field names below are only PARTLY verified, and this is the honest
// # statement of which
//
// Profiles §2 states, and these are taken as given: the four paths, and the
// field names `marginType`, `isAutoAddMargin` and `dualSidePosition`. Everything
// else on the wire structs — the multiAssetsMargin response key, symbolConfig's
// `leverage` and `symbol` keys — is this codec's best reading and has never met
// a real account, because credentials are deferred (record §73f). A wrong key
// decodes as an ABSENT field, which this surface reports as PostureAbsent and
// the domain refuses: the failure direction is a refused boot naming the
// setting, not a posture silently passed. That is why the pointer wire fields
// in this file are load-bearing rather than fastidious.

// binanceUnverifiedWeight is charged for the three posture probes whose
// published weight nobody here has read. See the file comment for the
// direction's argument.
const binanceUnverifiedWeight = 20

// EncodePosture implements AccountPostureCodec.
//
// The order is account-wide first, then per-symbol, which is the order the
// refusal reads in and the order §73c-2's quotable cost is measured in: one
// Binance posture read is four calls and 80 weight against the futures budget.
func (c *BinanceOrderCodec) EncodePosture() ([]PostureStep, error) {
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
		},
		{
			// Deliberately unscoped, like the positions enumeration: a
			// symbol-scoped read can only ask about symbols this system
			// already names, and the roster coverage check needs to see
			// what the venue actually returned in order to notice a
			// configured symbol MISSING from it.
			Probe: PostureProbePositionRisk,
			Call: transport.Call{
				Op:     op + ".PositionRisk",
				Method: "GET",
				Path:   "/fapi/v3/positionRisk",
				Family: ids.ProductLinearPerp,
				Cost:   ratelimit.Cost{Weight: 20},
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

// binanceSymbolConfigWire is one row of GET /fapi/v1/symbolConfig.
type binanceSymbolConfigWire struct {
	Symbol     *string `json:"symbol"`
	MarginType *string `json:"marginType"`
	Leverage   *int    `json:"leverage"`
}

// binancePositionRiskPostureWire is one row of GET /fapi/v3/positionRisk,
// narrowed to the single field this surface reads.
//
// Narrowed deliberately rather than reusing the positions decoder's wire type:
// that decoder is §7.5's net-delta input and answers a different question, and
// a shared struct would make a change made for one consumer land silently in
// the other.
type binancePositionRiskPostureWire struct {
	Symbol           *string `json:"symbol"`
	IsAutoAddMargin  *string `json:"isAutoAddMargin"`
	AutoAddMarginAlt *bool   `json:"autoAddMargin"`
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

	case PostureProbePositionRisk:
		return decodeBinancePositionRiskPosture(probe, resp)

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

// decodeBinanceSymbolConfig reads the per-symbol margin type and leverage.
//
// An EMPTY array is refused rather than read as "no symbols configured": the
// call is unscoped, a futures account has symbol configuration for every symbol
// it can trade, and an empty answer is a wrong-shaped one. Read as an empty
// roster it would produce a report with no per-symbol observations at all,
// against which every per-symbol rule folds vacuously — the flat-account
// vacuity of record §73b F4, arriving through the response instead of the
// account.
func decodeBinanceSymbolConfig(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	var rows []binanceSymbolConfigWire
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return nil, fmt.Errorf("binance symbol config: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("binance symbol config answer holds no rows: %w", ErrPostureUnreadable)
	}

	out := make([]PostureObservation, 0, len(rows)*2)
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

		out = append(out,
			postureObservation(probe, PostureMarginMode, PostureScopeSymbol, symbol, row.MarginType))
		out = append(out,
			postureObservation(probe, PostureLeverage, PostureScopeSymbol, symbol, binanceLeverageWord(row.Leverage)))
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

// decodeBinancePositionRiskPosture reads isAutoAddMargin per symbol.
//
// # The two spellings are BOTH read, and disagreement is a refusal
//
// Profiles §2 names the field `isAutoAddMargin` and records it as read-only
// with no REST setter. Binance's account surfaces have historically spelled
// this concern two ways and typed it two ways — a string "false" on some rows
// and a bool on others — and this codec has never met a real account (record
// §73f). Reading only one spelling would make the other decode as ABSENT, which
// the domain refuses, so the failure is loud either way; reading both and
// refusing a CONTRADICTION is strictly better, because the case where the venue
// answers both and they disagree is the one where picking a spelling silently
// picks an answer.
//
// An EMPTY array is NOT refused here, and that is the difference from
// symbolConfig above: positionRisk enumerates POSITIONS, and a flat account
// legitimately holds none. What must not happen is concluding "auto-add-margin
// is off" for a symbol with no row — so absence produces no observation at all,
// and the roster coverage check in the domain is what turns a configured symbol
// with no observation into a refusal (record §73b F4).
//
// # A symbol answered TWICE is reported, not refused (record §73n F19)
//
// Profiles §9 records the cause verbatim: "A hedge-mode account answers with a
// `LONG` row and a `SHORT` row for one symbol." Hedge mode is a §4.1 posture
// VIOLATION, so the duplicate is evidence rather than a wire fault — and this
// codec refusing it converts a posture that was READ AND FOUND WRONG into one
// that could not be read at all, which is the retryable arm of a distinction
// §7.5 draws precisely so that a violation no retry can lift latches instead.
// It also discards the three probes already decoded, including the account-wide
// dualSidePosition answer that names the same violation directly.
//
// So both rows travel and the domain judges them: risk/domain keys observations
// by (setting, scope, subject), sees two answers on one key, and reports
// ReasonAmbiguous — a named per-symbol failure that latches, beside every other
// check the read established. The codec's own argument was only ever against
// FOLDING the two rows into one answer, which nothing here does.
func decodeBinancePositionRiskPosture(probe PostureProbe, resp transport.Response) ([]PostureObservation, error) {
	var rows []binancePositionRiskPostureWire
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return nil, fmt.Errorf("binance position risk posture: %w", err)
	}

	out := make([]PostureObservation, 0, len(rows))
	for i, row := range rows {
		if row.Symbol == nil || *row.Symbol == "" {
			return nil, fmt.Errorf("binance position risk row %d carries no symbol: %w",
				i, ErrPostureUnreadable)
		}
		symbol := *row.Symbol

		word, err := binanceAutoAddMarginWord(symbol, row)
		if err != nil {
			return nil, err
		}
		out = append(out,
			postureObservation(probe, PostureAutoAddMargin, PostureScopeSymbol, symbol, word))
	}
	return out, nil
}

// binanceAutoAddMarginWord reconciles the two spellings, refusing a row that
// states both and disagrees.
//
// The comparison folds case, because the domain's own word comparison does
// (expectWord, "ON" and "OFF" differ under any folding) and a stricter test HERE
// would refuse the whole read over two spellings of one agreeing answer — the
// same misclassification F19 is about, one field down. What is refused is a real
// disagreement, where picking a spelling silently picks an answer.
func binanceAutoAddMarginWord(symbol string, row binancePositionRiskPostureWire) (*string, error) {
	var fromString *string
	if row.IsAutoAddMargin != nil {
		word := *row.IsAutoAddMargin
		fromString = &word
	}
	var fromBool *string
	if row.AutoAddMarginAlt != nil {
		word := "false"
		if *row.AutoAddMarginAlt {
			word = "true"
		}
		fromBool = &word
	}
	switch {
	case fromString == nil:
		return fromBool, nil
	case fromBool == nil:
		return fromString, nil
	case !strings.EqualFold(*fromString, *fromBool):
		return nil, fmt.Errorf("binance %s states auto-add-margin as both %q and %q: %w",
			symbol, *fromString, *fromBool, ErrPostureUnreadable)
	default:
		return fromString, nil
	}
}

// Compile-time proof that the codec satisfies the posture half too (§3.6
// rule 2).
var _ AccountPostureCodec = (*BinanceOrderCodec)(nil)
