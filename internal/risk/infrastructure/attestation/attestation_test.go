package attestation_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/domain"
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation"
)

// The collapse checklist for this package, run BEFORE these fixtures were
// written (house rule), and what each item forced:
//
//  1. THE THREE 64-HEX FIELDS. The pinned digest, the document's
//     `credential_sha256` and the deployment's fingerprint are all lowercase hex
//     of the same length and the same character class. A fixture reusing one
//     value for two of them makes "compare the digest" and "compare the
//     credential" the same comparison, so a loader reading the wrong field
//     passes. Every one of them differs here, visibly
//     ([[format-collapse-two-fields-render-alike]]).
//  2. VENUE AND ACCOUNT. Both plain strings compared with `!=`. `binance` /
//     `binance` would make a swapped-field read correct. The account is
//     `carry-sub-7`, which no venue is ever spelled as.
//  3. ONE VIOLATION PER FIXTURE. The loader halts on the first failed rung, so
//     a document breaking two rungs demonstrates only the shallower one
//     ([[a-config-listing-two-claims-checks-only-the-shallower]]). Every case
//     below breaks exactly one thing about an otherwise-loadable document.
//  4. BOTH OBSERVED WORDS. `on` is a LOADABLE document that refuses at the
//     gate, not a malformed one; a suite testing only `off` cannot tell those
//     apart, and the difference is whether an honest operator can record what
//     they saw.
//  5. THE POSITIVE CONTROL. Every negative case starts from valid(), which is
//     asserted to load — otherwise a loader that refused everything would score
//     perfectly.

const (
	// Three distinct 64-hex values. See checklist item 1.
	credentialFingerprint = "1111111111111111111111111111111111111111111111111111111111111111"
	otherFingerprint      = "2222222222222222222222222222222222222222222222222222222222222222"
	notTheDocumentsDigest = "3333333333333333333333333333333333333333333333333333333333333333"

	testVenue   = "binance"
	testAccount = "carry-sub-7"
)

func valid() attestation.Document {
	return attestation.Document{
		Kind:             attestation.Kind,
		Version:          attestation.Version,
		Venue:            testVenue,
		Account:          testAccount,
		CredentialSHA256: credentialFingerprint,
		ObservedAt:       "2026-08-06T09:30:00Z",
		Observer:         "gb",
		Method:           "binance.com account page: Portfolio Margin shows not enrolled",
		Observed:         attestation.ObservedOff,
	}
}

func bindings() attestation.Bindings {
	return attestation.Bindings{
		Venue:       ids.Venue(testVenue),
		Account:     testAccount,
		Fingerprint: credentialFingerprint,
	}
}

// write persists doc as the operator would hand-author it and returns a pin
// over the BYTES ON DISK — the same digest a `sha256sum` of the file gives.
func write(t *testing.T, doc attestation.Document) attestation.Pin {
	t.Helper()
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the document: %v", err)
	}
	body = append(body, '\n')
	path := filepath.Join(t.TempDir(), "portfolio-margin.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing the document: %v", err)
	}
	digest := sha256.Sum256(body)
	return attestation.Pin{Path: path, SHA256: hex.EncodeToString(digest[:])}
}

// TestLoadAcceptsAnOffAttestation is the positive control: the whole ladder
// passes and the operator's `off` reaches the domain as AttestationOff.
func TestLoadAcceptsAnOffAttestation(t *testing.T) {
	got, err := attestation.Load(write(t, valid()), bindings())
	if err != nil {
		t.Fatalf("a well-formed attestation must load: %v", err)
	}
	if got != domain.AttestationOff {
		t.Fatalf("loaded %s, want %s", got, domain.AttestationOff)
	}
}

// TestLoadAcceptsAnOnAttestationAndDoesNotJudgeIt is checklist item 4, and it
// is the load-bearing claim of the whole package: a document recording
// Portfolio Margin ON is VALID and loads. The refusal is the rule table's
// (ReasonMismatch), not this loader's — a loader that rejected `on` would leave
// an honest operator with no way to write down what they saw except by not
// writing, which is indistinguishable from never having looked.
func TestLoadAcceptsAnOnAttestationAndDoesNotJudgeIt(t *testing.T) {
	doc := valid()
	doc.Observed = attestation.ObservedOn

	got, err := attestation.Load(write(t, doc), bindings())
	if err != nil {
		t.Fatalf("an ON attestation is a valid document and must load: %v", err)
	}
	if got != domain.AttestationOn {
		t.Fatalf("loaded %s, want %s", got, domain.AttestationOn)
	}
}

// TestAnOnAttestationRefusesAtTheGate closes the loop the previous test opens.
// Loading `on` is not the interesting claim on its own — what matters is that
// the value it produces makes §4.1's check FAIL, by name, rather than being an
// enum member nothing acts on ([[a-declared-enum-member-nothing-emits]]).
func TestAnOnAttestationRefusesAtTheGate(t *testing.T) {
	doc := valid()
	doc.Observed = attestation.ObservedOn
	attested, err := attestation.Load(write(t, doc), bindings())
	if err != nil {
		t.Fatalf("loading the ON attestation: %v", err)
	}

	req := domain.Requirement{
		Venue:           ids.Venue(testVenue),
		Dialect:         domain.DialectBinanceClassic,
		Coverage:        domain.RosterCoverageDerivatives,
		Symbols:         []domain.SymbolPin{{Symbol: "BTCUSDT", Leverage: 5}},
		PortfolioMargin: attested,
	}
	verdict, err := domain.Check(req, domain.Report{Venue: ids.Venue(testVenue), ObservedAt: observedAt(t)})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	var found bool
	for _, f := range verdict.Failures {
		if f.Setting == domain.SettingPortfolioMargin {
			found = true
			if f.Reason != domain.ReasonMismatch {
				t.Fatalf("an attested ON is a MISMATCH, got %s", f.Reason)
			}
			if f.Observed != "on" {
				t.Fatalf("the failure must carry the operator's word, got %q", f.Observed)
			}
		}
	}
	if !found {
		t.Fatal("an attested Portfolio Margin ON produced no Portfolio-Margin failure")
	}
}

// TestLoadRefusalsNameTheirRung walks every rung of Load's ladder. Each case
// breaks exactly ONE thing about an otherwise-loadable deployment (checklist
// item 3), so each sentinel is demonstrated rather than merely reachable.
func TestLoadRefusalsNameTheirRung(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  func(*attestation.Document)
		pin  func(*attestation.Pin)
		bind func(*attestation.Bindings)
		want error
	}{
		{
			name: "pin with no path",
			pin:  func(p *attestation.Pin) { p.Path = "" },
			want: attestation.ErrPinIncomplete,
		},
		{
			name: "pin with no digest",
			pin:  func(p *attestation.Pin) { p.SHA256 = "" },
			want: attestation.ErrPinIncomplete,
		},
		{
			// The two-act rule biting: the file was edited and the config was
			// not. A wrong-but-well-formed digest, never "" — that would be
			// answered one rung earlier by ErrPinIncomplete.
			name: "digest is not the document's",
			pin:  func(p *attestation.Pin) { p.SHA256 = notTheDocumentsDigest },
			want: attestation.ErrDigestMismatch,
		},
		{
			name: "wrong kind",
			doc:  func(d *attestation.Document) { d.Kind = "arblab-spot-baseline" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "wrong version",
			doc:  func(d *attestation.Document) { d.Version = attestation.Version + 1 },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "no venue",
			doc:  func(d *attestation.Document) { d.Venue = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "no account",
			doc:  func(d *attestation.Document) { d.Account = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "no credential fingerprint",
			doc:  func(d *attestation.Document) { d.CredentialSHA256 = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "observed_at is not RFC3339",
			doc:  func(d *attestation.Document) { d.ObservedAt = "yesterday" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			// An anonymous claim is not evidence a later investigation can
			// follow, and this is the one input in the system with no machine
			// provenance at all.
			name: "no observer",
			doc:  func(d *attestation.Document) { d.Observer = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			// The inputs half of inputs-not-conclusions: without it the file
			// asserts a verdict-shaped fact with nothing to re-check.
			name: "no method",
			doc:  func(d *attestation.Document) { d.Method = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "observed is empty",
			doc:  func(d *attestation.Document) { d.Observed = "" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			// Default-deny on the observed word: a spelling this package does
			// not know is a malformed document, never a refusing attestation.
			name: "observed is an unknown word",
			doc:  func(d *attestation.Document) { d.Observed = "disabled" },
			want: attestation.ErrDocumentMalformed,
		},
		{
			name: "bindings carry no fingerprint",
			bind: func(b *attestation.Bindings) { b.Fingerprint = "" },
			want: attestation.ErrFingerprintUnset,
		},
		{
			// The key was rotated, or points at another subaccount under the
			// same label. Re-attestation is the intended cost.
			name: "credential is not the one attested under",
			bind: func(b *attestation.Bindings) { b.Fingerprint = otherFingerprint },
			want: attestation.ErrCredentialMismatch,
		},
		{
			name: "document describes another venue",
			doc:  func(d *attestation.Document) { d.Venue = "bybit" },
			want: attestation.ErrVenueMismatch,
		},
		{
			name: "bindings name no account",
			bind: func(b *attestation.Bindings) { b.Account = "" },
			want: attestation.ErrAccountMismatch,
		},
		{
			name: "document describes another account",
			doc:  func(d *attestation.Document) { d.Account = "carry-sub-8" },
			want: attestation.ErrAccountMismatch,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := valid()
			if tc.doc != nil {
				tc.doc(&doc)
			}
			pin := write(t, doc)
			if tc.pin != nil {
				tc.pin(&pin)
			}
			bind := bindings()
			if tc.bind != nil {
				tc.bind(&bind)
			}

			got, err := attestation.Load(pin, bind)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want %v", err, tc.want)
			}
			// Every refusal must also hand back the zero attestation. A refusal
			// returning AttestationOff beside an error is the shape where a
			// caller that logs the error and carries on gets a clean posture.
			if got != domain.AttestationUnstated {
				t.Fatalf("a refused load returned %s, want %s", got, domain.AttestationUnstated)
			}
		})
	}
}

// TestLoadReportsTheDEEPERRungWhenTwoAreBroken is the inverse of checklist item
// 3, and it exists because that item's own discipline hid a claim.
//
// Every case above breaks exactly ONE rung, which is what demonstrates that each
// rung can fire. But Load's ORDER is a separate claim with its own stated
// argument — the digest runs before parsing because "diagnosing their content
// would describe a document the operator never pinned", and the credential runs
// before the labels because the label is mutable configuration. A suite of
// one-violation fixtures cannot see that order at all: swap rungs 1 and 2, or 3
// and 4, and all eighteen cases still pass, because in each of them only one of
// the pair is broken.
//
// So the ordering needs the fixture the discipline forbids elsewhere — one that
// breaks BOTH members of a pair and names which answer must win
// ([[a-config-listing-two-claims-checks-only-the-shallower]], read the other way
// round).
func TestLoadReportsTheDEEPERRungWhenTwoAreBroken(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  func(*attestation.Document)
		pin  func(*attestation.Pin)
		bind func(*attestation.Bindings)
		want error
		over error
	}{
		{
			// Rung 1 before rung 2. The bytes are not the pinned ones, so
			// whatever they say about themselves describes a document nobody
			// pinned — including the fact that they are malformed.
			name: "a mis-digested document that is ALSO malformed",
			doc:  func(d *attestation.Document) { d.Kind = "arblab-spot-baseline" },
			pin:  func(p *attestation.Pin) { p.SHA256 = notTheDocumentsDigest },
			want: attestation.ErrDigestMismatch,
			over: attestation.ErrDocumentMalformed,
		},
		{
			// Rung 3 before rung 4. The credential is the binding that cannot be
			// edited in a config file, so it is the one to report when both the
			// key and the label disagree.
			name: "a document under another credential that ALSO names another venue",
			doc:  func(d *attestation.Document) { d.Venue = "bybit" },
			bind: func(b *attestation.Bindings) { b.Fingerprint = otherFingerprint },
			want: attestation.ErrCredentialMismatch,
			over: attestation.ErrVenueMismatch,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := valid()
			if tc.doc != nil {
				tc.doc(&doc)
			}
			pin := write(t, doc)
			if tc.pin != nil {
				tc.pin(&pin)
			}
			bind := bindings()
			if tc.bind != nil {
				tc.bind(&bind)
			}

			_, err := attestation.Load(pin, bind)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want %v", err, tc.want)
			}
			// And NOT the shallower one. Asserting only the winner would be
			// satisfied by a loader that joined both, which is the answer that
			// tells an operator to fix two things when one of them is a
			// consequence of the other.
			if errors.Is(err, tc.over) {
				t.Fatalf("error %v also reports %v — the deeper rung must answer alone", err, tc.over)
			}
		})
	}
}

// TestDigestComparisonFoldsCase covers the one deliberate leniency: an operator
// who pasted `sha256sum` output through something that upper-cased it has a
// config that means the same thing. Hex differs only in case, so the fold can
// admit a different SPELLING of the digest and never a different digest.
func TestDigestComparisonFoldsCase(t *testing.T) {
	pin := write(t, valid())
	pin.SHA256 = strings.ToUpper(pin.SHA256)

	if _, err := attestation.Load(pin, bindings()); err != nil {
		t.Fatalf("an upper-cased digest pins the same bytes: %v", err)
	}
}

// TestMissingFileIsNotAMalformedDocument keeps the two apart: a pin naming a
// file that is not there is a path the operator can fix, and reporting it as a
// malformed document would send them to inspect a file that does not exist.
func TestMissingFileIsNotAMalformedDocument(t *testing.T) {
	pin := attestation.Pin{
		Path:   filepath.Join(t.TempDir(), "absent.json"),
		SHA256: notTheDocumentsDigest,
	}
	_, err := attestation.Load(pin, bindings())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error %v, want a not-exist error", err)
	}
	if errors.Is(err, attestation.ErrDocumentMalformed) {
		t.Fatal("an absent file must not be reported as a malformed document")
	}
}

// TestDocumentIsNotSelfDescribingAboutTheVerdict is the inputs-not-conclusions
// rule stated as a test over the TYPE rather than over a value: the document
// must carry no field whose name or content is the gate's answer. It is a
// reflection-free check on purpose — it reads the JSON an operator writes.
func TestDocumentIsNotSelfDescribingAboutTheVerdict(t *testing.T) {
	body, err := json.Marshal(valid())
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	for _, forbidden := range []string{"satisfied", "verdict", "ok", "pass", "gate", "approved"} {
		if strings.Contains(strings.ToLower(string(body)), `"`+forbidden+`"`) {
			t.Fatalf("the document carries a %q field — it must record what was observed, never the conclusion", forbidden)
		}
	}
}

func observedAt(t *testing.T) clock.ServerTime {
	t.Helper()
	at, err := clock.NewServerTime(time.Date(2026, 8, 6, 9, 30, 0, 0, time.UTC), 25*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return at
}
