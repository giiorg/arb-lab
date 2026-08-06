package venue_test

import (
	"errors"
	"sort"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
)

// This file checks §4.1's account posture surface, venue-side (task #198,
// record §73).
//
// # The collapse checklist (record §73g) was run first, and it fixed these
// # fixtures
//
//   - Every per-symbol fixture holds TWO symbols with the violation on the
//     SECOND, so "every configured symbol" and "the first symbol" are different
//     answers. A decoder that reads row 0 and stops passes the one-row version
//     of each of these tests.
//   - The borrow fixtures pin the three cases the pointer wire fields exist to
//     separate — field absent, field present and empty, field present and "0" —
//     and assert they produce THREE different presences. Under the balance
//     surface's predicate the first two collapse into the third, which is the
//     defect record §73b F2 is about.
//   - Every boolean fixture includes the field ABSENT as well as present, because
//     Go's zero for bool is false and false is the PASSING value for every
//     boolean in Profiles §2's table. A plain-bool decoder passes every
//     present-field test in this file and fails only the absent ones.
//   - The cross-venue probe cases exist because each codec's decode switch names
//     the other venue's probes in a grouped case; without a fixture, that arm is
//     a branch nothing executes.

// ---------------------------------------------------------------------------
// Binance
// ---------------------------------------------------------------------------

// TestBinanceEncodePostureIsFourProbesOnTheFuturesFamily pins the step list.
//
// The step COUNT and the probe set are the claim record §73c-2 quotes a cost
// from — "one Binance posture read is four calls and 80 weight" — so they are
// asserted rather than described. A probe silently dropped from EncodePosture
// would leave its setting unobserved, and an unobserved setting is one the
// domain reports as absent: a refusal, but for the wrong reason and naming the
// wrong endpoint.
func TestBinanceEncodePostureIsFourProbesOnTheFuturesFamily(t *testing.T) {
	steps, err := binanceCodec(t).EncodePosture()
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	want := []venue.PostureProbe{
		venue.PostureProbeMultiAssets,
		venue.PostureProbePositionDual,
		venue.PostureProbeSymbolConfig,
		venue.PostureProbePositionRisk,
	}
	if len(steps) != len(want) {
		t.Fatalf("EncodePosture rendered %d steps, want %d", len(steps), len(want))
	}
	var weight uint64
	for i, step := range steps {
		if step.Probe != want[i] {
			t.Errorf("step %d is %s, want %s", i, step.Probe, want[i])
		}
		if step.Call.Method != "GET" {
			t.Errorf("step %d (%s) is %s, want GET — posture is READ, never set (record §73c-3)",
				i, step.Probe, step.Call.Method)
		}
		weight += uint64(step.Call.Cost.Weight)
	}
	// The figure is stated as a literal rather than summed from the codec's
	// own constant: a total derived from the same constant the encoder used
	// stays self-consistent under any change to it, which is the shared-
	// constant vacuity this repo has paid for.
	if weight != 80 {
		t.Errorf("one posture read charges %d weight, want 80 — record §73c-2 quotes this figure", weight)
	}
}

// TestBinanceDecodeMultiAssetsSeparatesAbsentFromFalse is the pointer-bool
// claim, and it is the single most load-bearing test in this file.
//
// false is the PASSING value for multiAssetsMargin. A decoder using a plain
// bool cannot tell "the venue said false" from "the venue said nothing", so a
// renamed field, a wrong endpoint or a truncated body all decode as the posture
// being correct. The two subtests differ ONLY in whether the key is present.
func TestBinanceDecodeMultiAssetsSeparatesAbsentFromFalse(t *testing.T) {
	codec := binanceCodec(t)

	stated, err := codec.DecodePostureStep(venue.PostureProbeMultiAssets,
		transport.Response{Body: []byte(`{"multiAssetsMargin": false}`)})
	if err != nil {
		t.Fatalf("decode stated: %v", err)
	}
	if len(stated) != 1 {
		t.Fatalf("stated produced %d observations, want 1", len(stated))
	}
	if stated[0].Presence != venue.PostureStated {
		t.Errorf("stated presence is %s, want PostureStated", stated[0].Presence)
	}
	if stated[0].Value != "false" {
		t.Errorf("stated value is %q, want %q", stated[0].Value, "false")
	}

	absent, err := codec.DecodePostureStep(venue.PostureProbeMultiAssets,
		transport.Response{Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("decode absent: %v", err)
	}
	if len(absent) != 1 {
		t.Fatalf("absent produced %d observations, want 1", len(absent))
	}
	if absent[0].Presence != venue.PostureAbsent {
		t.Fatalf("an OMITTED multiAssetsMargin decoded as %s — a plain bool would read "+
			"this as the passing value false and the posture would silently hold",
			absent[0].Presence)
	}
	if absent[0].Value != "" {
		t.Errorf("absent carries value %q, want empty", absent[0].Value)
	}
}

// TestBinanceDecodePositionDualSeparatesAbsentFromFalse is the mirror.
//
// Written out rather than folded into a table with the case above, because the
// mirror rule earned its own entry in this repo the hard way: two symmetric
// call sites, one tested and one not, and the untested one survived a mutant.
func TestBinanceDecodePositionDualSeparatesAbsentFromFalse(t *testing.T) {
	codec := binanceCodec(t)

	stated, err := codec.DecodePostureStep(venue.PostureProbePositionDual,
		transport.Response{Body: []byte(`{"dualSidePosition": false}`)})
	if err != nil {
		t.Fatalf("decode stated: %v", err)
	}
	if len(stated) != 1 || stated[0].Presence != venue.PostureStated || stated[0].Value != "false" {
		t.Fatalf("stated = %+v, want one PostureStated %q", stated, "false")
	}
	if stated[0].Setting != venue.PosturePositionMode {
		t.Errorf("setting is %s, want PosturePositionMode", stated[0].Setting)
	}

	absent, err := codec.DecodePostureStep(venue.PostureProbePositionDual,
		transport.Response{Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("decode absent: %v", err)
	}
	if len(absent) != 1 || absent[0].Presence != venue.PostureAbsent {
		t.Fatalf("an OMITTED dualSidePosition decoded as %+v, want PostureAbsent — "+
			"position mode is the ONE setting §6.3 says emits no venue event, so a "+
			"false pass here is undetectable by any other channel", absent)
	}
}

// TestBinanceDecodeSymbolConfigReadsEverySymbol is the cardinality claim.
//
// Two symbols, and the SECOND is the one out of posture. A decoder that reads
// the first row and returns passes a one-row fixture and this one's first half.
func TestBinanceDecodeSymbolConfigReadsEverySymbol(t *testing.T) {
	body := `[
		{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3},
		{"symbol":"ETHUSDT","marginType":"CROSSED","leverage":20}
	]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 4 {
		t.Fatalf("produced %d observations, want 4 (margin mode + leverage, per symbol)", len(obs))
	}

	got := map[string]string{}
	for _, o := range obs {
		if o.Presence != venue.PostureStated {
			t.Errorf("%s/%s presence is %s, want PostureStated", o.Subject, o.Setting, o.Presence)
		}
		if o.Scope != venue.PostureScopeSymbol {
			t.Errorf("%s/%s scope is %s, want PostureScopeSymbol", o.Subject, o.Setting, o.Scope)
		}
		got[o.Subject+"/"+o.Setting.String()] = o.Value
	}
	for key, want := range map[string]string{
		"BTCUSDT/PostureMarginMode": "ISOLATED",
		"BTCUSDT/PostureLeverage":   "3",
		"ETHUSDT/PostureMarginMode": "CROSSED",
		"ETHUSDT/PostureLeverage":   "20",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
}

// TestBinanceDecodeSymbolConfigRefusesEmptyAndDuplicate.
//
// The empty case is the interesting one and it is asymmetric with positionRisk
// below: symbolConfig is unscoped configuration, so no rows is a wrong-shaped
// answer, while positionRisk enumerates positions and no rows is a flat
// account. Reading the first as an empty roster would give a report with no
// per-symbol observations, against which every per-symbol rule folds vacuously.
func TestBinanceDecodeSymbolConfigRefusesEmptyAndDuplicate(t *testing.T) {
	codec := binanceCodec(t)

	if _, err := codec.DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(`[]`)}); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("empty symbolConfig error = %v, want ErrPostureUnreadable", err)
	}

	dup := `[
		{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3},
		{"symbol":"BTCUSDT","marginType":"CROSSED","leverage":20}
	]`
	if _, err := codec.DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(dup)}); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("duplicate symbol error = %v, want ErrPostureUnreadable", err)
	}
}

// TestBinanceDecodePositionRiskAllowsEmptyAndRefusesContradiction.
//
// Two claims. An empty enumeration is a flat account and produces NO
// observations — deliberately not an "auto-add-margin is off" conclusion, which
// is the flat-account vacuity of record §73b F4. And a row stating both
// spellings of auto-add-margin with different answers is refused, because
// picking a spelling there silently picks an answer.
func TestBinanceDecodePositionRiskAllowsEmptyAndRefusesContradiction(t *testing.T) {
	codec := binanceCodec(t)

	empty, err := codec.DecodePostureStep(venue.PostureProbePositionRisk,
		transport.Response{Body: []byte(`[]`)})
	if err != nil {
		t.Fatalf("empty positionRisk: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("a flat account produced %d observations, want 0 — an absent symbol must "+
			"reach the domain's roster check as MISSING, not as a passing value", len(empty))
	}

	contradiction := `[{"symbol":"BTCUSDT","isAutoAddMargin":"true","autoAddMargin":false}]`
	if _, err := codec.DecodePostureStep(venue.PostureProbePositionRisk,
		transport.Response{Body: []byte(contradiction)}); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("contradictory auto-add-margin error = %v, want ErrPostureUnreadable", err)
	}

	// Positive control: the two spellings agreeing is not an error, or the
	// refusal above would be a predicate that objects to everything.
	agree := `[{"symbol":"BTCUSDT","isAutoAddMargin":"false","autoAddMargin":false}]`
	obs, err := codec.DecodePostureStep(venue.PostureProbePositionRisk,
		transport.Response{Body: []byte(agree)})
	if err != nil {
		t.Fatalf("agreeing spellings: %v", err)
	}
	if len(obs) != 1 || obs[0].Value != "false" || obs[0].Setting != venue.PostureAutoAddMargin {
		t.Fatalf("agreeing spellings = %+v, want one PostureAutoAddMargin %q", obs, "false")
	}
}

// TestBinanceDecodePositionRiskReportsBothSidesOfAHedgedSymbol is F19's Binance
// half, and the mirror of the Bybit test below (record §73n).
//
// Profiles §9 records the cause verbatim: "A hedge-mode account answers with a
// LONG row and a SHORT row for one symbol." Hedge mode is a §4.1 violation, so
// refusing the read here discarded the three probes already decoded — including
// the account-wide dualSidePosition answer that names the same violation — and
// reported the whole posture as unestablishable rather than as refused.
func TestBinanceDecodePositionRiskReportsBothSidesOfAHedgedSymbol(t *testing.T) {
	body := `[
		{"symbol":"BTCUSDT","positionSide":"LONG","isAutoAddMargin":"false"},
		{"symbol":"BTCUSDT","positionSide":"SHORT","isAutoAddMargin":"true"}
	]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbePositionRisk,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("produced %d observations, want 2 (one per hedge row)", len(obs))
	}
	// The DISAGREEING answers both survive. Keeping either one would answer
	// §4.1's auto-add-margin question for this symbol from a row picked here.
	var words []string
	for _, o := range obs {
		if o.Subject != "BTCUSDT" || o.Setting != venue.PostureAutoAddMargin {
			t.Fatalf("observation %+v, want BTCUSDT auto-add-margin", o)
		}
		words = append(words, o.Value)
	}
	sort.Strings(words)
	if len(words) != 2 || words[0] != "false" || words[1] != "true" {
		t.Fatalf("auto-add-margin words = %v, want both rows' answers", words)
	}
}

// TestBinanceAutoAddMarginFoldsCaseAcrossTheTwoSpellings.
//
// The two spellings agreeing in different CASE is one answer, not a
// contradiction — the domain's own comparison folds case, and refusing here
// would lose the whole read over a spelling. The contradiction test above is the
// negative control that this fold does not admit a real disagreement.
func TestBinanceAutoAddMarginFoldsCaseAcrossTheTwoSpellings(t *testing.T) {
	body := `[{"symbol":"BTCUSDT","isAutoAddMargin":"FALSE","autoAddMargin":false}]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbePositionRisk,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 1 || obs[0].Value != "FALSE" {
		t.Fatalf("observations = %+v, want one carrying the venue's own word verbatim", obs)
	}
}

// TestBinanceRefusesBybitProbes covers the grouped case in the decode switch.
//
// Without this the arm is a branch nothing executes — and §3.6 rule 10's whole
// argument is that the grouped case is where a newly added member lands.
func TestBinanceRefusesBybitProbes(t *testing.T) {
	codec := binanceCodec(t)
	for _, probe := range []venue.PostureProbe{
		venue.PostureProbeAccountInfo,
		venue.PostureProbeSpotMargin,
		venue.PostureProbeWallet,
		venue.PostureProbePositionList,
	} {
		if _, err := codec.DecodePostureStep(probe,
			transport.Response{Body: []byte(`{}`)}); !errors.Is(err, venue.ErrPostureUnreadable) {
			t.Errorf("binance asked %s: error = %v, want ErrPostureUnreadable", probe, err)
		}
	}
	if _, err := codec.DecodePostureStep(venue.PostureProbeUnspecified,
		transport.Response{Body: []byte(`{}`)}); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("binance asked the sentinel: error = %v, want ErrPostureUnreadable", err)
	}
}

// ---------------------------------------------------------------------------
// Bybit
// ---------------------------------------------------------------------------

// TestBybitEncodePostureIsFourUnweightedProbes.
func TestBybitEncodePostureIsFourUnweightedProbes(t *testing.T) {
	steps, err := bybitCodec(t).EncodePosture()
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	want := []venue.PostureProbe{
		venue.PostureProbeAccountInfo,
		venue.PostureProbeSpotMargin,
		venue.PostureProbeWallet,
		venue.PostureProbePositionList,
	}
	if len(steps) != len(want) {
		t.Fatalf("EncodePosture rendered %d steps, want %d", len(steps), len(want))
	}
	for i, step := range steps {
		if step.Probe != want[i] {
			t.Errorf("step %d is %s, want %s", i, step.Probe, want[i])
		}
		if step.Call.Method != "GET" {
			t.Errorf("step %d (%s) is %s, want GET", i, step.Probe, step.Call.Method)
		}
		if step.Call.Cost.Weight != 0 {
			t.Errorf("step %d (%s) charges weight %d — Bybit meters per endpoint per UID "+
				"and has no weight to charge", i, step.Probe, step.Call.Cost.Weight)
		}
	}
}

// TestBybitDecodeBorrowSeparatesAbsentEmptyAndZero is record §73b F2's test,
// and the reason this surface reads the wallet a second time.
//
// Three coins, three shapes of the same field, three different presences. The
// balance surface's predicate — which reads an empty borrow as "no figure
// stated" and lets the row pin — collapses the first two into the third. That
// reading is correct there and would be a silent pass here, at the one gate
// whose purpose is refusing borrow.
func TestBybitDecodeBorrowSeparatesAbsentEmptyAndZero(t *testing.T) {
	body := `{"result":{"list":[{"accountType":"UNIFIED","coin":[
		{"coin":"BTC"},
		{"coin":"ETH","borrowAmount":"","spotBorrow":""},
		{"coin":"USDT","borrowAmount":"0","spotBorrow":"0"}
	]}]}}`
	obs, err := bybitCodec(t).DecodePostureStep(venue.PostureProbeWallet,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 6 {
		t.Fatalf("produced %d observations, want 6 (two borrow fields × three coins)", len(obs))
	}

	presence := map[string]venue.PosturePresence{}
	for _, o := range obs {
		if o.Scope != venue.PostureScopeAsset {
			t.Errorf("%s/%s scope is %s, want PostureScopeAsset", o.Subject, o.Setting, o.Scope)
		}
		presence[o.Subject+"/"+o.Setting.String()] = o.Presence
	}
	for key, want := range map[string]venue.PosturePresence{
		"BTC/PostureBorrowAmount":  venue.PostureAbsent,
		"BTC/PostureSpotBorrow":    venue.PostureAbsent,
		"ETH/PostureBorrowAmount":  venue.PostureStatedEmpty,
		"ETH/PostureSpotBorrow":    venue.PostureStatedEmpty,
		"USDT/PostureBorrowAmount": venue.PostureStated,
		"USDT/PostureSpotBorrow":   venue.PostureStated,
	} {
		if presence[key] != want {
			t.Errorf("%s presence = %s, want %s", key, presence[key], want)
		}
	}
}

// TestBybitDecodePositionListReadsEverySymbol is the cardinality claim, Bybit
// side: two symbols, the SECOND in hedge posture.
func TestBybitDecodePositionListReadsEverySymbol(t *testing.T) {
	body := `{"result":{"list":[
		{"symbol":"BTCUSDT","positionIdx":0,"leverage":"3","autoAddMargin":0},
		{"symbol":"ETHUSDT","positionIdx":1,"leverage":"20","autoAddMargin":1}
	]}}`
	obs, err := bybitCodec(t).DecodePostureStep(venue.PostureProbePositionList,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 6 {
		t.Fatalf("produced %d observations, want 6 (three settings × two symbols)", len(obs))
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.Subject+"/"+o.Setting.String()] = o.Value
	}
	for key, want := range map[string]string{
		"BTCUSDT/PosturePositionMode":  "0",
		"BTCUSDT/PostureLeverage":      "3",
		"BTCUSDT/PostureAutoAddMargin": "0",
		"ETHUSDT/PosturePositionMode":  "1",
		"ETHUSDT/PostureLeverage":      "20",
		"ETHUSDT/PostureAutoAddMargin": "1",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
}

// TestBybitDecodePositionListReportsBothSidesOfAHedgedSymbol is F19's Bybit
// half (record §73n).
//
// A duplicate symbol on this endpoint is the posture violation being hunted:
// hedge mode returns one row per side. This codec used to REFUSE the whole read
// for it, which turned a posture read-and-found-wrong into one that could not be
// read — the retryable arm, for an account no retry will change, and on Bybit
// nothing else names the position mode. Both rows now travel, and risk/domain's
// ReasonAmbiguous is what judges them. Folding is still not done: neither row's
// positionIdx is picked, they are both reported.
func TestBybitDecodePositionListReportsBothSidesOfAHedgedSymbol(t *testing.T) {
	body := `{"result":{"list":[
		{"symbol":"BTCUSDT","positionIdx":1,"leverage":"3","autoAddMargin":0},
		{"symbol":"BTCUSDT","positionIdx":2,"leverage":"3","autoAddMargin":0}
	]}}`
	obs, err := bybitCodec(t).DecodePostureStep(venue.PostureProbePositionList,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 6 {
		t.Fatalf("produced %d observations, want 6 (three settings × two hedge rows)", len(obs))
	}
	// The two positionIdx answers must BOTH survive: keeping one is the fold
	// this refuses to do, and it is the fold that would pick a mode.
	var sides []string
	for _, o := range obs {
		if o.Setting == venue.PosturePositionMode {
			sides = append(sides, o.Value)
		}
	}
	sort.Strings(sides)
	if len(sides) != 2 || sides[0] != "1" || sides[1] != "2" {
		t.Fatalf("position modes = %v, want both hedge sides 1 and 2", sides)
	}
}

// TestBybitDecodeAccountInfoReadsThreeSettingsAndSeparatesAbsent.
func TestBybitDecodeAccountInfoReadsThreeSettingsAndSeparatesAbsent(t *testing.T) {
	codec := bybitCodec(t)

	body := `{"result":{"marginMode":"ISOLATED_MARGIN","unifiedMarginStatus":5,"spotHedgingStatus":"OFF"}}`
	obs, err := codec.DecodePostureStep(venue.PostureProbeAccountInfo,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("produced %d observations, want 3", len(obs))
	}
	got := map[string]string{}
	for _, o := range obs {
		if o.Scope != venue.PostureScopeAccount {
			t.Errorf("%s scope is %s, want PostureScopeAccount", o.Setting, o.Scope)
		}
		if o.Subject != "" {
			t.Errorf("%s carries subject %q, want empty for an account-wide setting", o.Setting, o.Subject)
		}
		got[o.Setting.String()] = o.Value
	}
	for key, want := range map[string]string{
		"PostureMarginMode":  "ISOLATED_MARGIN",
		"PostureAccountKind": "5",
		"PostureSpotHedging": "OFF",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}

	// An answer carrying none of the three: every observation must be absent
	// rather than the surface reporting nothing at all, because a report with
	// no observations is one every rule folds over finding no failures.
	bare, err := codec.DecodePostureStep(venue.PostureProbeAccountInfo,
		transport.Response{Body: []byte(`{"result":{}}`)})
	if err != nil {
		t.Fatalf("bare account info: %v", err)
	}
	if len(bare) != 3 {
		t.Fatalf("a bare answer produced %d observations, want 3 absent ones", len(bare))
	}
	for _, o := range bare {
		if o.Presence != venue.PostureAbsent {
			t.Errorf("%s presence = %s, want PostureAbsent", o.Setting, o.Presence)
		}
	}
}

// TestBybitPostureWalletRefusesForeignAccountType.
func TestBybitPostureWalletRefusesForeignAccountType(t *testing.T) {
	body := `{"result":{"list":[{"accountType":"CONTRACT","coin":[{"coin":"BTC","borrowAmount":"0","spotBorrow":"0"}]}]}}`
	_, err := bybitCodec(t).DecodePostureStep(venue.PostureProbeWallet,
		transport.Response{Body: []byte(body)})
	if !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("foreign accountType error = %v, want ErrPostureUnreadable", err)
	}
}

// TestBybitRefusesBinanceProbes is the mirror of the Binance case.
func TestBybitRefusesBinanceProbes(t *testing.T) {
	codec := bybitCodec(t)
	for _, probe := range []venue.PostureProbe{
		venue.PostureProbeMultiAssets,
		venue.PostureProbeSymbolConfig,
		venue.PostureProbePositionRisk,
		venue.PostureProbePositionDual,
	} {
		if _, err := codec.DecodePostureStep(probe,
			transport.Response{Body: []byte(`{}`)}); !errors.Is(err, venue.ErrPostureUnreadable) {
			t.Errorf("bybit asked %s: error = %v, want ErrPostureUnreadable", probe, err)
		}
	}
}

// ---------------------------------------------------------------------------
// The observation's own contract
// ---------------------------------------------------------------------------

// TestPostureObservationValidateRefusesIncoherentShapes.
//
// Validate is what stops a hand-built fixture asserting a reading the venue
// never gave. The two value/presence cases are the ones that matter: a value
// beside a presence that says there was none is a struct where whichever field
// a rule reads, the other contradicts it — and the permissive reading is the
// one that reads the value.
func TestPostureObservationValidateRefusesIncoherentShapes(t *testing.T) {
	base := venue.PostureObservation{
		Setting:  venue.PostureMarginMode,
		Scope:    venue.PostureScopeAccount,
		Presence: venue.PostureStated,
		Value:    "ISOLATED",
		Probe:    venue.PostureProbeSymbolConfig,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("positive control: a well-formed observation was refused: %v", err)
	}

	cases := map[string]func(o *venue.PostureObservation){
		"account-wide with a subject": func(o *venue.PostureObservation) { o.Subject = "BTCUSDT" },
		"symbol-scoped with no subject": func(o *venue.PostureObservation) {
			o.Scope = venue.PostureScopeSymbol
		},
		"absent but carrying a value": func(o *venue.PostureObservation) {
			o.Presence = venue.PostureAbsent
		},
		"stated with no value": func(o *venue.PostureObservation) { o.Value = "" },
		"no presence at all": func(o *venue.PostureObservation) {
			o.Presence = venue.PosturePresenceUnspecified
			o.Value = ""
		},
		"no setting":  func(o *venue.PostureObservation) { o.Setting = venue.PostureSettingUnspecified },
		"no probe":    func(o *venue.PostureObservation) { o.Probe = venue.PostureProbeUnspecified },
		"no scope":    func(o *venue.PostureObservation) { o.Scope = venue.PostureScopeUnspecified },
		"scope range": func(o *venue.PostureObservation) { o.Scope = venue.PostureScope(99) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			o := base
			mutate(&o)
			if err := o.Validate(); err == nil {
				t.Errorf("Validate accepted %+v", o)
			}
		})
	}
}
