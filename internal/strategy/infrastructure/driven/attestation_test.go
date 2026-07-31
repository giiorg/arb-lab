package driven_test

// The attestation battery. §41i's named suspect: a suite of only-valid
// attestations tests the parse, not the binding — so the fixtures include
// an attestation naming a DIFFERENT hash than the artifact it sits beside,
// one naming a different artifact ID, and one whose execution identity
// disagrees; and the approval delay is exercised exactly AT the 24-hour
// boundary, where >= and > are different programs.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// goldenAttestation builds the valid attestation for the loaded golden
// artifact — approval exactly AT the delay boundary, deliberately.
func goldenAttestation(t *testing.T, a *driven.Artifact) map[string]any {
	t.Helper()
	derived := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	return map[string]any{
		"artifact_id":       a.Ref.ID,
		"artifact_sha256":   a.Ref.Hash,
		"procedure_commit":  a.Doc.Procedure.Commit,
		"executable_sha256": a.Doc.Procedure.ExecutableSHA256,
		"ledger": []any{
			map[string]any{"file": "datasets/binance/funding.BTCUSDT.2023-01-01.2026-01-31.json",
				"sha256": dig(0x13), "refusals": []any{}},
			map[string]any{"file": "datasets/bybit/funding.BTCUSDT.2023-01-01.2026-01-31.json",
				"sha256": dig(0x14), "refusals": []any{"h1_boundary: 1 row refused"}},
		},
		"derived_at":  derived.Format(time.RFC3339),
		"approved_at": derived.Add(driven.ApprovalDelay).Format(time.RFC3339),
		"sign_off":    "operator",
		"out_of_band": "note-to-self email, id 2026-08-02-a",
	}
}

func attBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestAttestation_LoadsAndBinds(t *testing.T) {
	a, _ := loadGolden(t)
	data := attBytes(t, goldenAttestation(t, a))
	at, err := driven.LoadAttestation(data)
	if err != nil {
		t.Fatalf("LoadAttestation: %v", err)
	}

	// The positive control: it binds the artifact it names — with the
	// approval instant exactly AT the 24-hour boundary.
	if err := at.Bind(a); err != nil {
		t.Errorf("Bind: %v", err)
	}

	// The attestation's own digest is recomputed from the exact bytes,
	// lowercase — the run-evidence identity §41e requires.
	sum := sha256.Sum256(data)
	if want := hex.EncodeToString(sum[:]); at.SHA256 != want {
		t.Errorf("SHA256 = %s; want %s", at.SHA256, want)
	}
	if at.SHA256 != strings.ToLower(at.SHA256) {
		t.Errorf("SHA256 %s is not lowercase", at.SHA256)
	}
}

func TestAttestation_BindRefusals(t *testing.T) {
	a, _ := loadGolden(t)

	load := func(mut func(map[string]any)) *driven.Attestation {
		doc := goldenAttestation(t, a)
		mut(doc)
		at, err := driven.LoadAttestation(attBytes(t, doc))
		if err != nil {
			t.Fatalf("LoadAttestation: %v", err)
		}
		return at
	}

	// A well-formed attestation for DIFFERENT bytes, sitting beside this
	// artifact: the §41i suspect. Every field parses; only Bind objects.
	if err := load(func(d map[string]any) { d["artifact_sha256"] = dig(0x42) }).Bind(a); !errors.Is(err, driven.ErrAttestationMismatch) {
		t.Errorf("wrong hash = %v; want ErrAttestationMismatch", err)
	}
	if err := load(func(d map[string]any) { d["artifact_id"] = "calibration.btcusdt.v2" }).Bind(a); !errors.Is(err, driven.ErrAttestationMismatch) {
		t.Errorf("wrong id = %v; want ErrAttestationMismatch", err)
	}
	if err := load(func(d map[string]any) { d["procedure_commit"] = "f000000000000000000000000000000000000000" }).Bind(a); !errors.Is(err, driven.ErrAttestationMismatch) {
		t.Errorf("wrong commit = %v; want ErrAttestationMismatch", err)
	}
	if err := load(func(d map[string]any) { d["executable_sha256"] = dig(0x43) }).Bind(a); !errors.Is(err, driven.ErrAttestationMismatch) {
		t.Errorf("wrong executable = %v; want ErrAttestationMismatch", err)
	}

	// No attestation at all is the other half of the §41i suspect.
	var missing *driven.Attestation
	if err := missing.Bind(a); err == nil {
		t.Error("nil attestation bound")
	}
	at := load(func(map[string]any) {})
	if err := at.Bind(nil); err == nil {
		t.Error("nil artifact bound")
	}
}

func TestAttestation_LoadRefusals(t *testing.T) {
	a, _ := loadGolden(t)

	cases := []struct {
		name string
		mut  func(map[string]any)
		want error
	}{
		// One second inside the cooling window: the delay rule, refused at
		// the boundary's near side (the far side is the positive control).
		{"approval one second early", func(d map[string]any) {
			derived, _ := time.Parse(time.RFC3339, d["derived_at"].(string))
			d["approved_at"] = derived.Add(driven.ApprovalDelay - time.Second).Format(time.RFC3339)
		}, driven.ErrApprovalDelayShort},
		{"uppercase artifact hash", func(d map[string]any) {
			d["artifact_sha256"] = strings.ToUpper(dig(0xab))
		}, driven.ErrArtifactHashFormat},
		{"missing sign off", func(d map[string]any) { d["sign_off"] = "" }, driven.ErrAttestationInvalid},
		{"missing out of band", func(d map[string]any) { d["out_of_band"] = "" }, driven.ErrAttestationInvalid},
		{"empty ledger", func(d map[string]any) { d["ledger"] = []any{} }, driven.ErrAttestationInvalid},
		{"missing derivation instant", func(d map[string]any) { d["derived_at"] = "" }, driven.ErrRowInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := goldenAttestation(t, a)
			tc.mut(doc)
			if _, err := driven.LoadAttestation(attBytes(t, doc)); !errors.Is(err, tc.want) {
				t.Fatalf("LoadAttestation = %v; want %v", err, tc.want)
			}
		})
	}
}
