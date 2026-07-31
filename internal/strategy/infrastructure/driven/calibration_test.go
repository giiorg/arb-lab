package driven_test

// The artifact loader battery. Fixture geometry answers §41i's named
// suspects by construction: every quantity cites a DISTINCT dataset digest
// (identical digests make per-quantity citation untestable), the hash
// fixtures include an UPPERCASE input (an all-lowercase suite tests the
// parse, not the case rule), and the pinned-hash check is exercised with a
// wrong-but-well-formed digest AND a malformed one, which are different
// refusals.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// dig mints a well-formed, distinct digest from a seed byte.
func dig(seed byte) string {
	return hex.EncodeToString(bytes.Repeat([]byte{seed}, 32))
}

func prov(family, derivation, rationale string, digests ...string) map[string]any {
	return map[string]any{
		"family":     family,
		"datasets":   digests,
		"derivation": derivation,
		"rationale":  rationale,
	}
}

// goldenDoc builds the valid artifact document as a plain map, so each
// refusal case can corrupt one member and re-marshal.
func goldenDoc() map[string]any {
	bound := func(kind, value string) map[string]any {
		m := map[string]any{"kind": kind}
		if value != "" {
			m["value"] = value
		}
		return m
	}
	interval := func(lo, hi map[string]any) map[string]any {
		return map[string]any{"lo": lo, "hi": hi}
	}
	cell := func(product, contract, side, stratum, maxQty, p90 string) map[string]any {
		return map[string]any{
			"venue": "binance", "product": product, "symbol": "BTCUSDT", "contract": contract,
			"side": side, "stratum": stratum,
			"base": "BTC", "max_qty": maxQty, "p90": p90,
			"observations": 30, "method": "walk_t_plus_latency_v1",
		}
	}
	return map[string]any{
		"schema_version": 1,
		"quantities": map[string]any{
			"dispersion": map[string]any{
				"points": []any{
					map[string]any{"q": "0.25", "shift": "-0.00013"},
					map[string]any{"q": "0.5", "shift": "-0.00001"},
					map[string]any{"q": "0.75", "shift": "0.00009"},
				},
				"provenance": prov("merged", "merge_pointwise_adverse_v1",
					"EWMA residuals over calibration-era history merged pointwise-more-adverse with captured imminent residuals",
					dig(0x11), dig(0x12)),
			},
			"entry_drift": map[string]any{
				"value":      "0.0002",
				"provenance": prov("rest_history", "drift_tail_v1", "adverse tail of the dispersion distribution", dig(0x13)),
			},
			"exit_band": map[string]any{
				"value":      "0.0015",
				"provenance": prov("rest_history", "excursion_persistence_v1", "measured regime persistence", dig(0x14)),
			},
			"adverse_basis_move": map[string]any{
				"asset":      "USDT",
				"amount":     "12.5",
				"provenance": prov("rest_history", "basis_proxy_high_quantile_v1", "hold-scale adverse basis move", dig(0x15)),
			},
			"rescue_margin": map[string]any{
				"asset":      "USDT",
				"amount":     "150",
				"provenance": prov("declared", "", "interim §40k-4 figure; real derivation is milestone 4's protocol"),
			},
			"expected_utilization": map[string]any{
				"value":      "0.62",
				"provenance": prov("rest_history", "occupancy_replay_v1", "Enter/Continue replay over the closed calibration sample, rounded down", dig(0x16)),
			},
			"cost": map[string]any{
				"adverse_fill_move":        "0.0001",
				"latency_assumed_bound_ms": 250,
				"min_evidence":             30,
				"classifier": map[string]any{
					"depth_asset": "BTC",
					"rules": []any{
						map[string]any{
							"name":   "normal",
							"spread": interval(bound("inclusive", "0"), bound("inclusive", "0.0001")),
							"depth":  interval(bound("exclusive", "0"), bound("unbounded", "")),
						},
						map[string]any{
							"name":   "wide",
							"spread": interval(bound("exclusive", "0.0001"), bound("inclusive", "1")),
							"depth":  interval(bound("exclusive", "0"), bound("unbounded", "")),
						},
					},
				},
				"cells": []any{
					cell("spot", "", "ask", "normal", "0.5", "0.0002"),
					cell("spot", "", "bid", "normal", "0.5", "0.0002"),
					cell("linear_perp", "linear", "ask", "normal", "0.5", "0.0003"),
					cell("linear_perp", "linear", "bid", "normal", "0.5", "0.0003"),
				},
				"fill_provenance": prov("captured_books", "counterfactual_walk_v1",
					"classify at t, walk at t+latency, measure against best at t", dig(0x17)),
				"latency_provenance": prov("declared", "",
					"no order path exists before milestone 3; transport p99 times a stated multiplier", dig(0x18)),
			},
			"scoring": map[string]any{
				"rolling_window_days": 90,
				"regime_rule":         "a sub-window is negative when its settlement-weighted mean funding is below zero",
				"standby_benchmark": map[string]any{
					"value":      "0.0369",
					"provenance": prov("declared", "", "13-week bill close, idle-capital benchmark"),
				},
				"max_drawdown_ceiling": map[string]any{
					"value":      "0.1",
					"provenance": prov("declared", "", "fraction of allocated capital"),
				},
				"confidence_level":  "0.95",
				"confidence_method": "bootstrap_percentile",
				"min_utilization": map[string]any{
					"value":      "0.31",
					"provenance": prov("declared", "", "eligibility cutoff"),
				},
				"min_utilization_relation": "declared as half the expected-utilization figure, rounded down at scale 2",
				"provenance": prov("declared", "",
					"scoring-protocol choices; each is a rule step 5 applies mechanically"),
			},
		},
		"datasets": map[string]any{
			"h1_boundary": "2026-02-01T00:00:00Z",
			"history": []any{
				map[string]any{
					"venue": "binance", "symbol": "BTCUSDT", "series": "funding",
					"start": "2023-01-01T00:00:00Z", "end": "2026-01-31T16:00:00Z",
					"file": "datasets/binance/funding.BTCUSDT.2023-01-01.2026-01-31.json", "sha256": dig(0x13),
					"completeness": "every settlement the venue serves inside the range, pagination exhausted",
				},
				map[string]any{
					"venue": "bybit", "symbol": "BTCUSDT", "series": "funding",
					"start": "2023-01-01T00:00:00Z", "end": "2026-01-31T16:00:00Z",
					"file": "datasets/bybit/funding.BTCUSDT.2023-01-01.2026-01-31.json", "sha256": dig(0x14),
					"completeness": "every settlement the venue serves inside the range, pagination exhausted",
				},
			},
			"manifests": []any{
				map[string]any{"day": "2026-07-30", "sha256": dig(0x17)},
			},
		},
		"procedure": map[string]any{
			"commit":            "a5b5d33000000000000000000000000000000000",
			"executable_sha256": dig(0x19),
		},
		"promotion": map[string]any{
			"venue": "binance",
			"fork":  "hybrid",
			"instruments": []any{
				map[string]any{"venue": "binance", "product": "spot", "symbol": "BTCUSDT", "contract": ""},
				map[string]any{"venue": "binance", "product": "linear_perp", "symbol": "BTCUSDT", "contract": "linear"},
			},
			"sample_start":        "2026-07-31T00:00:00Z",
			"end_rule":            "first complete three-month interval after the freeze instant",
			"missing_data_policy": "a gapped day is scored as stand-down; it is never dropped",
		},
	}
}

func docBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

const goldenID = "calibration.btcusdt.v1"

func loadGolden(t *testing.T) (*driven.Artifact, []byte) {
	t.Helper()
	data := docBytes(t, goldenDoc())
	a, err := driven.LoadArtifact(goldenID, data)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	return a, data
}

func TestLoadArtifact_IdentityAndValues(t *testing.T) {
	a, data := loadGolden(t)

	// The identity: the given ID, the hash recomputed from the exact bytes,
	// lowercase hex.
	if a.Ref.ID != goldenID {
		t.Errorf("Ref.ID = %q; want %q", a.Ref.ID, goldenID)
	}
	sum := sha256.Sum256(data)
	if want := hex.EncodeToString(sum[:]); a.Ref.Hash != want {
		t.Errorf("Ref.Hash = %s; want %s", a.Ref.Hash, want)
	}
	if a.Ref.Hash != strings.ToLower(a.Ref.Hash) {
		t.Errorf("Ref.Hash %s is not lowercase", a.Ref.Hash)
	}

	// Every class-2 value carries the artifact identity.
	for name, ref := range map[string]domain.CalibrationRef{
		"dispersion":  a.Dispersion.Artifact,
		"entry drift": a.EntryDrift.Artifact,
		"exit band":   a.ExitBand.Artifact,
		"utilization": a.ExpectedUtilization.Artifact,
		"basis":       a.AdverseBasisMove.Artifact,
		"rescue":      a.RescueMargin.Artifact,
		"cost":        a.Cost.Artifact,
		"standby":     a.Scoring.StandbyBenchmark.Artifact,
	} {
		if ref != a.Ref {
			t.Errorf("%s carries %+v; want the artifact's own ref", name, ref)
		}
	}

	// Spot values, converted.
	if got, want := a.EntryDrift.Value.Decimal(), "0.0002"; got != want {
		t.Errorf("EntryDrift = %s; want %s", got, want)
	}
	if got, want := a.AdverseBasisMove.Amount.Decimal(), "12.5"; got != want {
		t.Errorf("AdverseBasisMove = %s; want %s", got, want)
	}
	if got, want := len(a.Cost.Cells), 4; got != want {
		t.Errorf("cost cells = %d; want %d", got, want)
	}
	if got, want := a.Cost.Latency, 250*time.Millisecond; got != want {
		t.Errorf("Latency = %v; want %v", got, want)
	}
	if got, want := a.Scoring.RollingWindow, 90*24*time.Hour; got != want {
		t.Errorf("RollingWindow = %v; want %v", got, want)
	}
	if a.Promotion.Fork != "hybrid" || a.Promotion.Venue != "binance" {
		t.Errorf("Promotion = %+v; want the sealed hybrid/binance identity", a.Promotion)
	}
	if got, want := len(a.Promotion.Instruments), 2; got != want {
		t.Errorf("promotion instruments = %d; want %d", got, want)
	}
	if a.Promotion.Instruments[0] != mSpotInst {
		t.Errorf("promotion spot = %+v; want %+v", a.Promotion.Instruments[0], mSpotInst)
	}

	// The classifier came through usable: the calibration constructs a
	// working cost model (proving cells and classifier agree).
	if _, err := driven.NewConservativeCostModel(a.Cost, flatBooks(t)); err != nil {
		t.Errorf("loaded calibration does not construct the model: %v", err)
	}

	// Determinism: the same bytes load to the same identity.
	b, err := driven.LoadArtifact(goldenID, data)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if b.Ref != a.Ref {
		t.Errorf("second load ref %+v != first %+v", b.Ref, a.Ref)
	}
}

func TestLoadArtifactFile_BasenameIsTheID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.btcusdt.v1.json")
	if err := os.WriteFile(path, docBytes(t, goldenDoc()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	a, err := driven.LoadArtifactFile(path)
	if err != nil {
		t.Fatalf("LoadArtifactFile: %v", err)
	}
	if a.Ref.ID != "calibration.btcusdt.v1" {
		t.Errorf("Ref.ID = %q; want the basename without extension", a.Ref.ID)
	}
}

func TestVerifyPinned(t *testing.T) {
	a, _ := loadGolden(t)

	// The positive control.
	if err := a.VerifyPinned(a.Ref.Hash); err != nil {
		t.Errorf("correct pin refused: %v", err)
	}

	// A wrong but well-formed pin is a MISMATCH.
	if err := a.VerifyPinned(dig(0x42)); !errors.Is(err, driven.ErrArtifactHashMismatch) {
		t.Errorf("wrong pin = %v; want ErrArtifactHashMismatch", err)
	}

	// The UPPERCASE form of the CORRECT hash is a FORMAT refusal — §41d
	// fixes lowercase, and normalizing would hide a hand-edited pin. This
	// is the one-uppercase-input fixture the checklist demands.
	if err := a.VerifyPinned(strings.ToUpper(a.Ref.Hash)); !errors.Is(err, driven.ErrArtifactHashFormat) {
		t.Errorf("uppercase pin = %v; want ErrArtifactHashFormat", err)
	}

	// A short pin is a format refusal too.
	if err := a.VerifyPinned("abc123"); !errors.Is(err, driven.ErrArtifactHashFormat) {
		t.Errorf("short pin = %v; want ErrArtifactHashFormat", err)
	}
}

func TestLoadArtifact_IdentityRefusals(t *testing.T) {
	data := docBytes(t, goldenDoc())

	if _, err := driven.LoadArtifact("", data); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("empty id = %v; want ErrUnset", err)
	}
	if _, err := driven.LoadArtifact(domain.FixturePrefix+"anything", data); !errors.Is(err, driven.ErrArtifactFixtureID) {
		t.Errorf("fixture id = %v; want ErrArtifactFixtureID", err)
	}
}

// corrupt navigates the golden map by dotted path and sets the leaf.
func corrupt(t *testing.T, doc map[string]any, path string, v any) {
	t.Helper()
	keys := strings.Split(path, ".")
	m := doc
	for _, k := range keys[:len(keys)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			t.Fatalf("corrupt path %s: %q is not an object", path, k)
		}
		m = next
	}
	m[keys[len(keys)-1]] = v
}

func TestLoadArtifact_SchemaRefusals(t *testing.T) {
	cases := []struct {
		name string
		mut  func(t *testing.T, doc map[string]any)
		want error
	}{
		{"schema version", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "schema_version", 2)
		}, driven.ErrArtifactInvalid},
		{"measured quantity claiming declared", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.entry_drift.provenance",
				prov("declared", "", "just trust it"))
		}, driven.ErrFamilyWrong},
		{"declared quantity wearing a derivation", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.rescue_margin.provenance",
				prov("declared", "sneaky_procedure_v1", "declared but derived"))
		}, driven.ErrFamilyWrong},
		{"unknown family", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.exit_band.provenance",
				prov("vibes", "excursion_persistence_v1", "measured-ish", dig(0x14)))
		}, driven.ErrFamilyWrong},
		{"dispersion not merged", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.dispersion.provenance",
				prov("rest_history", "merge_pointwise_adverse_v1", "one family only", dig(0x11)))
		}, driven.ErrFamilyWrong},
		{"missing rationale", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.exit_band.provenance",
				prov("rest_history", "excursion_persistence_v1", "", dig(0x14)))
		}, driven.ErrArtifactInvalid},
		{"measured without datasets", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.exit_band.provenance",
				prov("rest_history", "excursion_persistence_v1", "measured from nothing"))
		}, driven.ErrArtifactInvalid},
		// The digest seed must hex to LETTERS (0xab → "abab…"), or ToUpper
		// is the identity and the case rule is untestable — the exact
		// hex-case fixture collapse this repo has already paid for once.
		{"uppercase dataset digest", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.exit_band.provenance",
				prov("rest_history", "excursion_persistence_v1", "case rule",
					strings.ToUpper(dig(0xab))))
		}, driven.ErrArtifactHashFormat},
		{"bad mantissa", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.entry_drift.value", "0.0.2")
		}, driven.ErrArtifactInvalid},
		{"missing quantity", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.entry_drift.value", "")
		}, driven.ErrArtifactInvalid},
		{"empty dispersion", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.dispersion.points", []any{})
		}, domain.ErrUnset},
		{"utilization above one", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "quantities.expected_utilization.value", "1.2")
		}, driven.ErrArtifactInvalid},
		{"cell stratum unknown to classifier", func(t *testing.T, d map[string]any) {
			q := d["quantities"].(map[string]any)["cost"].(map[string]any)
			cellsList := q["cells"].([]any)
			cellsList[0].(map[string]any)["stratum"] = "vanished"
		}, driven.ErrCellStratumUnknown},
		{"manifest digest malformed", func(t *testing.T, d map[string]any) {
			ds := d["datasets"].(map[string]any)
			ds["manifests"].([]any)[0].(map[string]any)["sha256"] = "nope"
		}, driven.ErrArtifactHashFormat},
		{"history end missing", func(t *testing.T, d map[string]any) {
			ds := d["datasets"].(map[string]any)
			ds["history"].([]any)[0].(map[string]any)["end"] = ""
		}, driven.ErrRowInvalid},
		{"h1 boundary missing", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "datasets.h1_boundary", "")
		}, driven.ErrRowInvalid},
		{"procedure commit missing", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "procedure.commit", "")
		}, domain.ErrUnset},
		{"executable digest malformed", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "procedure.executable_sha256", "deadbeef")
		}, driven.ErrArtifactHashFormat},
		{"promotion fork unknown", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "promotion.fork", "whichever-passes")
		}, driven.ErrArtifactInvalid},
		{"promotion end rule missing", func(t *testing.T, d map[string]any) {
			corrupt(t, d, "promotion.end_rule", "")
		}, domain.ErrUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := goldenDoc()
			tc.mut(t, doc)
			if _, err := driven.LoadArtifact(goldenID, docBytes(t, doc)); !errors.Is(err, tc.want) {
				t.Fatalf("LoadArtifact = %v; want %v", err, tc.want)
			}
		})
	}

	// An unknown member refuses — the decode is strict.
	doc := goldenDoc()
	doc["surprise"] = true
	if _, err := driven.LoadArtifact(goldenID, docBytes(t, doc)); err == nil {
		t.Error("unknown member decoded")
	}
}

// The loaded classifier is the SAME classifier the runtime runs: a book on
// the loaded artifact's classifier lands where the hand-built fixture
// classifier puts it.
func TestLoadArtifact_ClassifierRoundTrip(t *testing.T) {
	a, _ := loadGolden(t)
	book := flatBooks(t)[mSpotInst]
	feat, err := driven.BookFeaturesFor(book, mdomain.SideAsk)
	if err != nil {
		t.Fatalf("BookFeaturesFor: %v", err)
	}
	got, err := a.Cost.Classifier.Classify(feat)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "normal" {
		t.Errorf("Classify = %s; want normal", got)
	}
}
