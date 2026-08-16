package venue_test

import (
	"errors"
	"sort"
	"strings"
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

// TestBinanceEncodePostureIsThreeProbesOnTheFuturesFamily pins the step list.
//
// The step COUNT and the probe set are the claim record §73c-2 quotes a cost
// from — "one Binance posture read is three calls and 60 weight" — so they are
// asserted rather than described. A probe silently dropped from EncodePosture
// would leave its setting unobserved, and an unobserved setting is one the
// domain reports as absent: a refusal, but for the wrong reason and naming the
// wrong endpoint.
//
// It is also the test that pins record §73o's fold: /fapi/v3/positionRisk is NOT
// among these paths, because it does not publish the field §4.1 was reading from
// it. Re-adding a per-symbol probe against a POSITION enumeration would restore
// a gate no flat account can pass.
//
// # That last sentence was TRUE, GENERAL, and filed under one venue
//
// Bybit's posture read was a per-symbol probe against a position enumeration —
// precisely the shape this paragraph warns about — from v0.7 until 2026-08-16,
// and it did exactly what the warning predicts: three per-symbol settings
// unobservable on a flat account, so no boot could pass in the state a two-leg
// open requires. The hazard was named for Binance's ENDPOINT rather than for the
// MECHANISM, so nothing carried it across the file (record §97 §4). The count
// asserted below is still three because this codec's symbolConfig answers for
// every configured symbol; the Bybit test's count is now a function of the
// roster, and the two sitting side by side is the point.
func TestBinanceEncodePostureIsThreeProbesOnTheFuturesFamily(t *testing.T) {
	steps, err := binanceCodec(t).EncodePosture(postureRoster)
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	want := []venue.PostureProbe{
		venue.PostureProbeMultiAssets,
		venue.PostureProbePositionDual,
		venue.PostureProbeSymbolConfig,
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
	if weight != 60 {
		t.Errorf("one posture read charges %d weight, want 60 — record §73c-2 quotes this figure", weight)
	}
	for _, step := range steps {
		if step.Call.Path == "/fapi/v3/positionRisk" || step.Call.Path == "/fapi/v2/positionRisk" {
			t.Errorf("posture probes %s — no posture setting is read from a POSITION "+
				"enumeration (record §73o): V3 does not publish isAutoAddMargin at all, "+
				"and neither version answers for a configured symbol holding no position",
				step.Call.Path)
		}
	}
}

// TestEveryPostureStepDeclaresWhatItAnswers is the totality check on the new
// column (task #199, record §73p).
//
// The Answers list is what lets the composition root derive which families a
// deployment must route, so a step that declares nothing contributes nothing and
// its family silently stops being required — a narrowing by omission, which is
// the direction this repo refuses everywhere else. The per-probe counts are
// written out because "at least one" is satisfied by a symbolConfig step that
// admits to answering only one of its three settings, which would drop two rules
// from the required set.
func TestEveryPostureStepDeclaresWhatItAnswers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []venue.PostureStep
		want  map[venue.PostureProbe]int
	}{
		{
			name:  "binance",
			steps: encodePosture(t, binanceCodec(t)),
			want: map[venue.PostureProbe]int{
				venue.PostureProbeMultiAssets:  1,
				venue.PostureProbePositionDual: 1,
				// The three-for-one call (record §73o).
				venue.PostureProbeSymbolConfig: 3,
			},
		},
		{
			name:  "bybit",
			steps: encodePosture(t, bybitCodec(t)),
			want: map[venue.PostureProbe]int{
				// The three ACCOUNT-subject settings, which is why a spot-only
				// Bybit deployment still needs this call's family.
				venue.PostureProbeAccountInfo:  3,
				venue.PostureProbeSpotMargin:   1,
				venue.PostureProbeWallet:       2,
				venue.PostureProbePositionList: 3,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[venue.PostureSetting]venue.PostureProbe)
			for _, s := range tc.steps {
				if err := s.Validate(); err != nil {
					t.Errorf("step %s: %v", s.Probe, err)
					continue
				}
				if got := len(s.Answers); got != tc.want[s.Probe] {
					t.Errorf("%s answers %d settings %v, want %d", s.Probe, got, s.Answers, tc.want[s.Probe])
				}
				// One setting, one probe KIND. Two DIFFERENT probes answering
				// the same setting would make either family look required for
				// it, so dropping one would be invisible to the derivation —
				// and that harm is what this row asserts, so it is what the
				// comparison names.
				//
				// Repeated steps carrying the SAME probe are legal since record
				// §97: Bybit renders one position step per configured symbol,
				// all on one probe and one family, so no family can hide behind
				// another. Comparing step identity rather than probe identity
				// here would refuse the fix that made a flat account bootable.
				for _, a := range s.Answers {
					if prior, dup := seen[a]; dup && prior != s.Probe {
						t.Errorf("%s is answered by both %s and %s", a, prior, s.Probe)
					}
					seen[a] = s.Probe
				}
			}
		})
	}
}

// TestPostureStepValidateRefusesAStepThatAnswersNothing is the negative control
// for the field above — both real codecs are complete, so without a hand-built
// step nothing has ever made Validate object.
func TestPostureStepValidateRefusesAStepThatAnswersNothing(t *testing.T) {
	step := venue.PostureStep{Probe: venue.PostureProbeMultiAssets}
	if err := step.Validate(); !errors.Is(err, venue.ErrPostureStepAnswersNothing) {
		t.Errorf("a step answering nothing validated with %v, want ErrPostureStepAnswersNothing", err)
	}
	step.Answers = []venue.PostureSetting{venue.PostureSettingUnspecified}
	if err := step.Validate(); err == nil {
		t.Error("a step answering the SENTINEL validated — the zero value is not a setting")
	}
}

// TestPostureStepsForNarrowsToTheWantedSettings is the read half of #199's
// ruling: a deployment that needs nothing from a family stops calling it.
func TestPostureStepsForNarrowsToTheWantedSettings(t *testing.T) {
	steps := encodePosture(t, bybitCodec(t))

	// Bybit under a no-derivatives roster: six settings, three probes. The
	// position enumeration drops out and account/info does NOT — which is the
	// assertion, since a narrowing keyed on "no derivatives configured" rather
	// than on the settings would drop both.
	want := []venue.PostureSetting{
		venue.PostureMarginMode, venue.PostureAccountKind, venue.PostureSpotHedging,
		venue.PostureSpotMarginMode, venue.PostureBorrowAmount, venue.PostureSpotBorrow,
	}
	got, err := venue.PostureStepsFor(steps, want)
	if err != nil {
		t.Fatalf("PostureStepsFor: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("narrowed to %d steps, want 3", len(got))
	}
	for _, s := range got {
		if s.Probe == venue.PostureProbePositionList {
			t.Errorf("the position enumeration survived a want-set that names none of its settings")
		}
	}

	// The whole set narrows to the whole list — the positive control, without
	// which a filter that dropped everything would pass the check above.
	all := []venue.PostureSetting{
		venue.PostureMarginMode, venue.PostureAccountKind, venue.PostureSpotHedging,
		venue.PostureSpotMarginMode, venue.PostureBorrowAmount, venue.PostureSpotBorrow,
		venue.PosturePositionMode, venue.PostureLeverage, venue.PostureAutoAddMargin,
	}
	if got, err := venue.PostureStepsFor(steps, all); err != nil || len(got) != len(steps) {
		t.Errorf("the full want-set narrowed to %d steps (err %v), want all %d", len(got), err, len(steps))
	}
}

// TestPostureStepsForRefusesAWantItCannotServe covers both ways the want-set and
// the codec can disagree. Each is a refusal rather than a shorter list, because
// a shorter list is a read that establishes less than the caller believes.
func TestPostureStepsForRefusesAWantItCannotServe(t *testing.T) {
	binance := encodePosture(t, binanceCodec(t))

	// A setting this codec does not answer. Binance has no spot-borrow probe,
	// and asking for one must not quietly return the three futures steps.
	if _, err := venue.PostureStepsFor(binance, []venue.PostureSetting{venue.PostureSpotBorrow}); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("a want-set naming an unanswerable setting returned %v, want ErrPostureUnreadable", err)
	}
	// A want-set whose members are PARTLY answerable is still a refusal: the
	// premise is a conjunction, so serving the answerable half is exactly the
	// partial read this surface forbids.
	partial := []venue.PostureSetting{venue.PostureMultiAssets, venue.PostureSpotBorrow}
	if _, err := venue.PostureStepsFor(binance, partial); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("a partly answerable want-set returned %v, want ErrPostureUnreadable", err)
	}
	// And the empty want-set, which would otherwise narrow to zero steps and
	// report a posture nothing was read for.
	if _, err := venue.PostureStepsFor(binance, nil); !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("an empty want-set returned %v, want ErrPostureUnreadable", err)
	}
}

// postureRoster is the configured roster these tests encode against.
//
// TWO symbols, not one, and that is the collapse this file would otherwise ship:
// with a single symbol "one step per configured symbol" and "one step, always"
// render an identical list, so every count assertion below would pass against
// the settleCoin-scoped encoder record §97 replaced. The second entry is what
// makes the per-symbol claim observable at all.
var postureRoster = []string{"BTCUSDT", "LABUSDT"}

func encodePosture(t *testing.T, c venue.AccountPostureCodec) []venue.PostureStep {
	t.Helper()
	steps, err := c.EncodePosture(postureRoster)
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	return steps
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

// TestBinanceDecodeSymbolConfigReadsEverySymbol is the cardinality claim, and
// since record §73o it is also where ALL THREE per-symbol settings are read.
//
// Two symbols, and the SECOND is the one out of posture on every setting. A
// decoder that reads the first row and returns passes a one-row fixture and this
// one's first half.
//
// Two fixture dimensions are deliberately varied rather than pinned, because
// pinning either makes a real distinction unreachable:
//
//   - auto-add-margin DIFFERS between the two symbols. Pinned to one value,
//     "this row's field" and "the first row's field" are the same answer, which
//     is the defect the margin-mode and leverage columns already guard against.
//   - the two rows state it in DIFFERENT JSON TYPES — a bool and a string.
//     Binance's own spec-generated connector declares this one field a bool on
//     symbolConfig and a string on positionRisk V2, so a fixture using one type
//     throughout cannot tell a decoder that reads both from one that reads the
//     type the fixture happened to pick (record §73o).
func TestBinanceDecodeSymbolConfigReadsEverySymbol(t *testing.T) {
	body := `[
		{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":false},
		{"symbol":"ETHUSDT","marginType":"CROSSED","leverage":20,"isAutoAddMargin":"true"}
	]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if len(obs) != 6 {
		t.Fatalf("produced %d observations, want 6 (margin mode + leverage + "+
			"auto-add-margin, per symbol)", len(obs))
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
		"BTCUSDT/PostureMarginMode":    "ISOLATED",
		"BTCUSDT/PostureLeverage":      "3",
		"BTCUSDT/PostureAutoAddMargin": "false",
		"ETHUSDT/PostureMarginMode":    "CROSSED",
		"ETHUSDT/PostureLeverage":      "20",
		"ETHUSDT/PostureAutoAddMargin": "true",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
}

// TestBinanceSymbolConfigSeparatesAbsentAutoAddMarginFromFalse.
//
// `false` is the PASSING value, so this is the same claim the multiAssets test
// makes and the reason the wire field is not a plain bool. It matters more here
// than it did before record §73o: under the old positionRisk probe a symbol with
// no answer produced NO observation and the domain's roster coverage check
// refused it as Unobserved. Now the row exists and the FIELD is missing, which
// is a different file for an operator to open — a wire-shape change rather than
// a coverage gap — and it must arrive as Unstated, not as a pass.
func TestBinanceSymbolConfigSeparatesAbsentAutoAddMarginFromFalse(t *testing.T) {
	body := `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3}]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	var found bool
	for _, o := range obs {
		if o.Setting != venue.PostureAutoAddMargin {
			continue
		}
		found = true
		if o.Presence != venue.PostureAbsent {
			t.Errorf("an OMITTED isAutoAddMargin decoded as %s — false is the value "+
				"§4.1 requires, so reading absence as it passes the check the field exists "+
				"to make", o.Presence)
		}
		if o.Value != "" {
			t.Errorf("absent auto-add-margin carries value %q, want empty", o.Value)
		}
	}
	// The observation must be PRODUCED, not skipped: a decoder that emits
	// nothing for the missing field leaves the domain to infer the gap from a
	// roster count, which reports the wrong reason and names no field.
	if !found {
		t.Fatalf("no auto-add-margin observation for a row that omitted the field; got %+v", obs)
	}
}

// TestBinanceDecodeSymbolConfigRefusesEmptyAndDuplicate.
//
// The empty case is the interesting one: symbolConfig is unscoped
// CONFIGURATION, so no rows is a wrong-shaped answer. Reading it as an empty
// roster would give a report with no per-symbol observations, against which
// every per-symbol rule folds vacuously.
//
// The duplicate refusal is NOT record §73n F19 reopened, and the distinction is
// the point. F19 folded the two POSITION enumerations to report duplicate rows,
// because a hedge-mode account answers with a LONG row and a SHORT row and that
// duplicate is evidence of a §4.1 violation. symbolConfig holds one row per
// symbol whatever the position mode is, so a duplicate here has no documented
// venue behaviour behind it — and the two rows could disagree about the very
// setting being asserted. Binance hedge mode is still named directly, and
// account-wide, by the dualSidePosition probe.
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

// TestBinanceAutoAddMarginRefusesAContradictionBetweenTheTwoSpellings.
//
// A row stating both spellings with different answers is refused, because
// picking a spelling there silently picks an answer.
//
// The fixture carries TWO symbols and the contradiction is on the SECOND. With
// one row, "refuses a contradicting row" and "refuses if the FIRST row
// contradicts" are the same claim, and a decoder that checked only the row it
// started with would pass.
func TestBinanceAutoAddMarginRefusesAContradictionBetweenTheTwoSpellings(t *testing.T) {
	codec := binanceCodec(t)

	contradiction := `[
		{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":"false","autoAddMargin":false},
		{"symbol":"ETHUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":"true","autoAddMargin":false}
	]`
	err := postureDecodeErr(t, codec, contradiction)
	if !errors.Is(err, venue.ErrPostureUnreadable) {
		t.Errorf("contradictory auto-add-margin error = %v, want ErrPostureUnreadable", err)
	}
	// The refusal must name the symbol it is about. A read refused over a
	// message naming no symbol sends an operator to every configured row.
	if err != nil && !strings.Contains(err.Error(), "ETHUSDT") {
		t.Errorf("contradiction error = %q, want it to name ETHUSDT", err)
	}

	// Positive control: the two spellings agreeing is not an error, or the
	// refusal above would be a predicate that objects to everything.
	agree := `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":"false","autoAddMargin":false}]`
	obs, err := codec.DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(agree)})
	if err != nil {
		t.Fatalf("agreeing spellings: %v", err)
	}
	if got := postureOne(t, obs, "BTCUSDT", venue.PostureAutoAddMargin); got.Value != "false" {
		t.Fatalf("agreeing spellings gave auto-add-margin %q, want %q", got.Value, "false")
	}
}

// TestBinanceAutoAddMarginFoldsCaseAcrossTheTwoSpellings.
//
// The two spellings agreeing in different CASE is one answer, not a
// contradiction — the domain's own comparison folds case, and refusing here
// would lose the whole read over a spelling. The contradiction test above is the
// negative control that this fold does not admit a real disagreement.
//
// The word travels VERBATIM. Normalising it here would hide which of the two the
// venue actually sent, and the domain's own comparison is what folds case.
func TestBinanceAutoAddMarginFoldsCaseAcrossTheTwoSpellings(t *testing.T) {
	body := `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":"FALSE","autoAddMargin":false}]`
	obs, err := binanceCodec(t).DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePostureStep: %v", err)
	}
	if got := postureOne(t, obs, "BTCUSDT", venue.PostureAutoAddMargin); got.Value != "FALSE" {
		t.Fatalf("auto-add-margin = %q, want the venue's own word %q verbatim", got.Value, "FALSE")
	}
}

// TestBinanceAutoAddMarginRefusesAShapeThatIsNeitherBoolNorString.
//
// The decoder accepts two JSON types because Binance declares two, and this is
// the boundary that makes that a reading rather than a shrug: any OTHER shape is
// an ERROR naming the field, not a nil.
//
// A nil would decay into "the venue did not state it" — which is a true sentence
// about a DIFFERENT world, the one where the key is missing. Both refuse, so no
// posture is wrongly passed either way; what differs is which document the
// operator opens, and a wire-shape change and a coverage gap are not the same
// file (record §73j's two-axis failure vocabulary).
//
// The positive control is the whole point of the case list: `false` and
// `"false"` must both still decode, or "refuses a bad shape" is satisfied by a
// decoder that refuses everything.
func TestBinanceAutoAddMarginRefusesAShapeThatIsNeitherBoolNorString(t *testing.T) {
	codec := binanceCodec(t)
	row := func(value string) string {
		return `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":3,"isAutoAddMargin":` + value + `}]`
	}

	for _, bad := range []string{`1`, `0`, `{}`, `[]`, `{"enabled":false}`} {
		err := postureDecodeErr(t, codec, row(bad))
		if !errors.Is(err, venue.ErrPostureUnreadable) {
			t.Errorf("isAutoAddMargin %s: error = %v, want ErrPostureUnreadable", bad, err)
		}
		if err != nil && !strings.Contains(err.Error(), "isAutoAddMargin") {
			t.Errorf("isAutoAddMargin %s: error = %q, want it to name the field", bad, err)
		}
	}

	for _, good := range []struct {
		json     string
		want     string
		presence venue.PosturePresence
	}{
		{`false`, "false", venue.PostureStated},
		{`true`, "true", venue.PostureStated},
		{`"false"`, "false", venue.PostureStated},
		// An explicit null is an ABSENT field, not a bad shape — and the
		// presence is what says so, since the value is "" either way.
		{`null`, "", venue.PostureAbsent},
	} {
		obs, err := codec.DecodePostureStep(venue.PostureProbeSymbolConfig,
			transport.Response{Body: []byte(row(good.json))})
		if err != nil {
			t.Fatalf("isAutoAddMargin %s: %v", good.json, err)
		}
		got := postureOne(t, obs, "BTCUSDT", venue.PostureAutoAddMargin)
		if got.Value != good.want || got.Presence != good.presence {
			t.Errorf("isAutoAddMargin %s decoded as %s %q, want %s %q",
				good.json, got.Presence, got.Value, good.presence, good.want)
		}
	}
}

// postureDecodeErr decodes a symbolConfig body and returns the error, failing
// the test if the decode unexpectedly SUCCEEDED — which a bare `err != nil`
// check would report as a passing case.
func postureDecodeErr(t *testing.T, codec venue.AccountPostureCodec, body string) error {
	t.Helper()
	obs, err := codec.DecodePostureStep(venue.PostureProbeSymbolConfig,
		transport.Response{Body: []byte(body)})
	if err == nil {
		t.Fatalf("decode succeeded and produced %+v, want a refusal", obs)
	}
	return err
}

// postureOne returns the single (subject, setting) observation, failing the test
// if there is not exactly one.
//
// It does NOT collapse into a string helper returning "" for a missing
// observation: "" is also what an ABSENT field carries, so a `want ""` case and
// a decoder that produced no observation at all would be indistinguishable —
// which is precisely the claim the null case below is making.
func postureOne(t *testing.T, obs []venue.PostureObservation,
	subject string, setting venue.PostureSetting) venue.PostureObservation {
	t.Helper()
	var found []venue.PostureObservation
	for _, o := range obs {
		if o.Subject == subject && o.Setting == setting {
			found = append(found, o)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s/%s: %d observations, want exactly 1 (all: %+v)",
			subject, setting, len(found), obs)
	}
	return found[0]
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

// TestBybitEncodePostureIsThreeAccountProbesPlusOnePerSymbol pins the step list,
// and since record §97 the count is a FUNCTION of the roster rather than a
// constant.
//
// The position read is scoped by `symbol` and no longer by `settleCoin`, because
// settleCoin scope answers only for symbols carrying an open position: on a flat
// account §4.1's three per-symbol rows were unobservable, and a flat account is
// the state a two-leg open requires. Record §73c-2's quotable cost becomes
// 3 + len(symbols).
func TestBybitEncodePostureIsThreeAccountProbesPlusOnePerSymbol(t *testing.T) {
	steps, err := bybitCodec(t).EncodePosture(postureRoster)
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	want := []venue.PostureProbe{
		venue.PostureProbeAccountInfo,
		venue.PostureProbeSpotMargin,
		venue.PostureProbeWallet,
		venue.PostureProbePositionList,
		venue.PostureProbePositionList,
	}
	if len(steps) != len(want) {
		t.Fatalf("EncodePosture rendered %d steps, want %d (3 account + %d symbols)",
			len(steps), len(want), len(postureRoster))
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

	// Each position step must name ITS OWN symbol, and must not carry settleCoin.
	//
	// Asserted per step rather than "some step names LABUSDT", because a broken
	// encoder that pinned every step to postureRoster[0] would satisfy the count
	// above and any containment check — the roster's second entry exists to make
	// that distinguishable, and only a positional assertion consumes it.
	for i, sym := range postureRoster {
		call := steps[3+i].Call
		got, ok := paramValue(call.Params, "symbol")
		if !ok || got != sym {
			t.Errorf("position step %d asks for symbol %q (present=%t), want %q", i, got, ok, sym)
		}
		if settle, ok := paramValue(call.Params, "settleCoin"); ok {
			t.Errorf("position step %d still carries settleCoin=%q — that scope answers "+
				"only for symbols with an OPEN POSITION, which is the defect record §97 fixed",
				i, settle)
		}
	}
}

// TestBybitEncodePostureRefusesAnUnsetSymbol is the roster's own default-deny.
//
// An empty string would render `symbol=` on the wire, which Bybit answers as a
// refusal — but by then the refusal names the venue's opinion of our query
// rather than our own missing configuration, at the one gate an operator reads
// most carefully.
func TestBybitEncodePostureRefusesAnUnsetSymbol(t *testing.T) {
	if _, err := bybitCodec(t).EncodePosture([]string{"BTCUSDT", ""}); err == nil {
		t.Fatal("an unset symbol encoded a posture step, want a refusal")
	}
}

// TestBybitEncodePostureWithNoSymbolsRendersNoPositionStep is the other end, and
// it is a legal shape rather than an error: a deployment configuring no
// derivative instrument has an empty roster, its three per-symbol rules lapse,
// and a position step demanding the linear-perp family would make §4.1
// unanswerable for a document §5.2 actively builds toward (record §73n F20).
func TestBybitEncodePostureWithNoSymbolsRendersNoPositionStep(t *testing.T) {
	steps, err := bybitCodec(t).EncodePosture(nil)
	if err != nil {
		t.Fatalf("EncodePosture: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("an empty roster rendered %d steps, want the 3 account probes", len(steps))
	}
	for _, s := range steps {
		if s.Probe == venue.PostureProbePositionList {
			t.Error("an empty roster still rendered a position step")
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
