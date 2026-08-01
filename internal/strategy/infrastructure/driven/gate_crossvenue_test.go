package driven_test

// The §45 gate artifact's schema. The collapse checklist ran before this
// battery and pinned three fixture parameters:
//
//   - TWO symbols, not one. Every roster rule here is positional ("the nth
//     scored symbol is the nth declared symbol"), and a one-symbol fixture
//     makes the nth and the FIRST the same element — the cardinality
//     collapse, which cost a SURVIVED mutant one slice ago.
//   - The two symbols are DIFFERENT in every member the rules compare, so
//     a validator reading the wrong element has somewhere to disagree.
//   - The verdict-consistency fixture includes the EXACTLY-EQUAL case. An
//     LCB strictly above or strictly below the hurdle cannot distinguish
//     `>=` from `>`, and that boundary is the verdict: a portfolio landing
//     exactly on the hurdle clears it.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

const (
	hashA = "1111111111111111111111111111111111111111111111111111111111111111"
	hashB = "2222222222222222222222222222222222222222222222222222222222222222"
	hashC = "3333333333333333333333333333333333333333333333333333333333333333"
)

// gateDoc builds a well-formed two-symbol gate artifact document.
func gateDoc() driven.GateArtifactDoc {
	return driven.GateArtifactDoc{
		Schema: driven.GateArtifactSchema,
		Procedure: driven.GateProcedureDoc{
			Label:            "crossvenue-gate-v1",
			Commit:           "abc123",
			ExecutableSHA256: hashA,
			ReferenceFile:    "configs/reference.btcusdt.json",
			ReferenceSHA256:  hashB,
		},
		Parameters: driven.GateParametersDoc{
			Symbols: []driven.GateSymbolParamDoc{
				{Symbol: "LABUSDT", Entry: "0.001", Qty: "1683", Weight: "0.5"},
				{Symbol: "KAITOUSDT", Entry: "0.0005", Qty: "228", Weight: "0.5"},
			},
			MatchedFloor:    180,
			RefusalCeiling:  "0.25",
			VoidImbalance:   true,
			SampleEvery:     "1m0s",
			SampleSlack:     "10s",
			MaxGap:          "10m0s",
			ExitDivisor:     2,
			PairingRound:    "1m0s",
			P90:             "0.9",
			Period:          "4h0m0s",
			MeanBlock:       6,
			Resamples:       2000,
			Confidence:      "0.95",
			Seed:            20260801,
			NotionalTarget:  "250",
			BinanceTakerFee: "0.0005",
			BybitTakerFee:   "0.00055",
			Denominator:     "per-leg notional, no leverage uplift",
			Holdout:         "ruling §45f-4",
		},
		Evidence: driven.GateEvidenceDoc{
			SettledCutoff: "2026-08-01T00:00:00Z",
			BooksCutoff:   "2026-08-01T11:13:00Z",
			Before:        "2026-09-05T00:00:00Z",
			ArchiveDays: []driven.GateArchiveDayDoc{
				{Day: "2026-08-01", SHA256: hashA},
				{Day: "2026-08-02", SHA256: hashB},
			},
			SettledFiles: []driven.LedgerEntryDoc{
				{File: "research/crossvenue/rest/binance/funding.LABUSDT.2026-09-05.json", SHA256: hashC},
			},
		},
		Symbols: []driven.GateSymbolDoc{
			{Symbol: "LABUSDT", MatchedSettlements: 212, DecisionPoints: 40000,
				Episodes: 31, Scored: 29, Refused: 2, AnnualizedReturn: "0.181"},
			{Symbol: "KAITOUSDT", MatchedSettlements: 196, DecisionPoints: 39000,
				Episodes: 44, Scored: 40, Refused: 4, AnnualizedReturn: "0.042"},
		},
		Verdict: &driven.GateVerdictDoc{
			Periods:         210,
			PortfolioLCB:    "0.000021",
			AnnualizedLCB:   "0.04599",
			Hurdle:          "0.0369",
			HurdlePublished: "0.0369",
			HurdleAsOf:      "2026-07-30T00:00:00Z",
			Clears:          true,
		},
	}
}

func TestGateArtifactValidFixtureLoads(t *testing.T) {
	t.Parallel()
	// The positive control: without it, a validator that objected to
	// everything would score perfectly on every case below.
	data, err := json.Marshal(gateDoc())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	a, err := driven.LoadGateArtifact("crossvenue-gate.v1", data)
	if err != nil {
		t.Fatalf("the well-formed fixture does not load: %v", err)
	}
	if a.Identity.ID != "crossvenue-gate.v1" {
		t.Fatalf("identity id %q", a.Identity.ID)
	}
	if a.Identity.ProcedureCommit != "abc123" || a.Identity.ExecutableSHA256 != hashA {
		t.Fatalf("identity did not take the execution binding from the document: %+v", a.Identity)
	}
	// The content hash is computed from the bytes, so it is not any hash
	// the document carries.
	if a.Identity.ContentSHA256 == hashA || a.Identity.ContentSHA256 == hashB {
		t.Fatal("content hash equals a hash the document carries — it must be of the bytes")
	}
}

func TestGateArtifactRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func(*driven.GateArtifactDoc)
		want error
	}{
		{"unknown schema", func(d *driven.GateArtifactDoc) {
			d.Schema = "arblab.crossvenue.gate.v2"
		}, driven.ErrGateArtifactSchema},
		{"empty schema", func(d *driven.GateArtifactDoc) { d.Schema = "" }, driven.ErrGateArtifactSchema},

		{"no procedure commit", func(d *driven.GateArtifactDoc) {
			d.Procedure.Commit = ""
		}, driven.ErrGateArtifactInvalid},
		{"malformed executable digest", func(d *driven.GateArtifactDoc) {
			d.Procedure.ExecutableSHA256 = "nothex"
		}, nil},
		{"no reference revision named", func(d *driven.GateArtifactDoc) {
			d.Procedure.ReferenceFile = ""
		}, driven.ErrGateArtifactInvalid},

		{"unparseable confidence", func(d *driven.GateArtifactDoc) {
			d.Parameters.Confidence = "ninety-five"
		}, driven.ErrGateArtifactInvalid},
		{"unparseable period", func(d *driven.GateArtifactDoc) {
			d.Parameters.Period = "four hours"
		}, driven.ErrGateArtifactInvalid},
		{"no denominator declaration", func(d *driven.GateArtifactDoc) {
			d.Parameters.Denominator = ""
		}, driven.ErrGateArtifactInvalid},
		{"no holdout declaration", func(d *driven.GateArtifactDoc) {
			d.Parameters.Holdout = ""
		}, driven.ErrGateArtifactInvalid},

		{"no archive days", func(d *driven.GateArtifactDoc) {
			d.Evidence.ArchiveDays = nil
		}, driven.ErrGateArtifactInvalid},
		{"no settled files", func(d *driven.GateArtifactDoc) {
			d.Evidence.SettledFiles = nil
		}, driven.ErrGateArtifactInvalid},
		{"malformed bound", func(d *driven.GateArtifactDoc) {
			d.Evidence.Before = "2026-09-05"
		}, driven.ErrGateArtifactInvalid},

		// The roster rules. The second symbol carries each fault so a
		// validator checking only the first element cannot pass.
		{"scored roster shorter than declared", func(d *driven.GateArtifactDoc) {
			d.Symbols = d.Symbols[:1]
		}, driven.ErrGateArtifactInvalid},
		{"second scored symbol is not the second declared", func(d *driven.GateArtifactDoc) {
			d.Symbols[1].Symbol = "BCHUSDT"
		}, driven.ErrGateArtifactInvalid},
		{"scored roster reordered", func(d *driven.GateArtifactDoc) {
			d.Symbols[0], d.Symbols[1] = d.Symbols[1], d.Symbols[0]
		}, driven.ErrGateArtifactInvalid},
		{"second symbol is below the floor", func(d *driven.GateArtifactDoc) {
			d.Symbols[1].MatchedSettlements = 179
		}, driven.ErrGateArtifactInvalid},

		// The verdict block.
		{"verdict absent", func(d *driven.GateArtifactDoc) { d.Verdict = nil }, driven.ErrGateArtifactInvalid},
		{"no periods", func(d *driven.GateArtifactDoc) { d.Verdict.Periods = 0 }, driven.ErrGateArtifactInvalid},
		{"clears disagrees with the numbers", func(d *driven.GateArtifactDoc) {
			d.Verdict.Clears = false // 0.04599 does clear 0.0369
		}, driven.ErrGateArtifactInvalid},
		{"does-not-clear claimed as clearing", func(d *driven.GateArtifactDoc) {
			d.Verdict.AnnualizedLCB = "0.01" // below the hurdle, Clears still true
		}, driven.ErrGateArtifactInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := gateDoc()
			tc.mut(&d)
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			_, err = driven.LoadGateArtifact("id", data)
			if err == nil {
				t.Fatal("the malformed document loaded")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// TestGateArtifactVerdictBoundary pins the one comparison the verdict IS:
// an annualized LCB exactly equal to the effective hurdle CLEARS. Strictly
// above and strictly below cannot tell `>=` from `>`, so without the equal
// case the verdict's own boundary is untested.
func TestGateArtifactVerdictBoundary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		lcb        string
		wantClears bool
	}{
		{"above the hurdle clears", "0.04", true},
		{"exactly at the hurdle clears", "0.0369", true},
		{"one scale-9 tick below does not clear", "0.036899999", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := gateDoc()
			d.Verdict.AnnualizedLCB = tc.lcb
			d.Verdict.Clears = tc.wantClears
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := driven.LoadGateArtifact("id", data); err != nil {
				t.Fatalf("LCB %s against hurdle %s with clears=%t was refused: %v",
					tc.lcb, d.Verdict.Hurdle, tc.wantClears, err)
			}

			// And the opposite verdict on the same numbers must refuse.
			d.Verdict.Clears = !tc.wantClears
			data, err = json.Marshal(d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := driven.LoadGateArtifact("id", data); err == nil {
				t.Fatalf("LCB %s with clears=%t loaded — the consistency check did not fire",
					tc.lcb, !tc.wantClears)
			}
		})
	}
}

// TestGateArtifactAttestationBinding checks that the same detached
// attestation machinery binds a gate artifact, and that each of the four
// bound fields alone is enough to refuse.
func TestGateArtifactAttestationBinding(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(gateDoc())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	art, err := driven.LoadGateArtifact("crossvenue-gate.v1", data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	good := driven.AttestationDoc{
		ArtifactID:       art.Identity.ID,
		ArtifactSHA256:   art.Identity.ContentSHA256,
		ProcedureCommit:  art.Identity.ProcedureCommit,
		ExecutableSHA256: art.Identity.ExecutableSHA256,
		Ledger:           []driven.LedgerEntryDoc{{File: "x", SHA256: hashA}},
		DerivedAt:        "2026-09-05T00:00:00Z",
		ApprovedAt:       "2026-09-06T00:00:00Z",
		SignOff:          "operator",
		OutOfBand:        "confirmed by phone",
	}
	load := func(t *testing.T, doc driven.AttestationDoc) *driven.Attestation {
		t.Helper()
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		at, err := driven.LoadAttestation(b)
		if err != nil {
			t.Fatalf("attestation load: %v", err)
		}
		return at
	}

	if err := load(t, good).BindIdentity(art.Identity); err != nil {
		t.Fatalf("the matching attestation did not bind: %v", err)
	}

	// Each field alone refuses. Named individually because each one is the
	// whole check for a different tamper.
	for name, mut := range map[string]func(*driven.AttestationDoc){
		"a rename":           func(d *driven.AttestationDoc) { d.ArtifactID = "other-artifact" },
		"an edit":            func(d *driven.AttestationDoc) { d.ArtifactSHA256 = hashC },
		"a different commit": func(d *driven.AttestationDoc) { d.ProcedureCommit = "def456" },
		"a different binary": func(d *driven.AttestationDoc) { d.ExecutableSHA256 = hashC },
	} {
		t.Run(name, func(t *testing.T) {
			doc := good
			mut(&doc)
			err := load(t, doc).BindIdentity(art.Identity)
			if !errors.Is(err, driven.ErrAttestationMismatch) {
				t.Fatalf("%s bound anyway: %v", name, err)
			}
		})
	}

	// The cooling delay is the attestation's own rule, checked here
	// because the gate artifact is the second consumer to depend on it.
	rushed := good
	rushed.ApprovedAt = "2026-09-05T23:59:59Z"
	b, err := json.Marshal(rushed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := driven.LoadAttestation(b); err == nil || !strings.Contains(err.Error(), "delay") {
		t.Fatalf("an approval one second inside the cooling delay loaded: %v", err)
	}
}
