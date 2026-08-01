package driven

// This file is the detached approval attestation (§41e): the committed,
// machine-consumed record that a named operator approved a specific
// artifact's exact bytes, after the declared cooling delay, with the
// out-of-band confirmation named.
//
// It is detached — a sibling file, never a block inside the artifact —
// because the approval names the artifact's content hash, and an in-file
// approval would sit inside its own preimage (§41d). It is WRITE-ONCE like
// the artifact: replacing its approval instant or sign-off while the
// artifact hash stands still would leave every journaled CalibrationRef
// pointing at the same two fields, so an approval-record replacement is a
// new artifact version, never an edit.
//
// Step 5's runner consumes THIS, not a naked expected hash: a runner keyed
// on a caller-supplied hash makes an unapproved artifact mechanically
// indistinguishable from an approved one. The runner also records the
// attestation's own SHA-256 in the scored run's evidence, so the run
// identity names WHICH approval authorized it — that digest is computed
// here, at load, from the exact bytes.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Sentinels.
var (
	// ErrAttestationInvalid reports a malformed attestation member; the
	// wrapping message names it.
	ErrAttestationInvalid = fmt.Errorf("strategy/driven: invalid attestation member")

	// ErrAttestationMismatch reports an attestation that does not bind the
	// artifact beside it — a different ID, content hash, procedure commit
	// or executable digest. Every check passing against the WRONG artifact
	// is exactly the tamper case the binding exists to catch.
	ErrAttestationMismatch = fmt.Errorf("strategy/driven: attestation does not bind this artifact")

	// ErrApprovalDelayShort reports an approval instant less than the
	// declared cooling delay after derivation. §41e's solo-honest posture
	// is a time-delayed self-sign-off; an approval minted in the same
	// sitting is the delay rule broken, not a faster operator.
	ErrApprovalDelayShort = fmt.Errorf("strategy/driven: approval before the declared delay elapsed")
)

// ApprovalDelay is §41e's declared cooling window between derivation and
// approval: long enough to reread cold, short enough not to idle the
// milestone. The boundary is inclusive — approval exactly at the delay is
// admissible.
const ApprovalDelay = 24 * time.Hour

// LedgerEntryDoc is one dataset the derivation read: the file, its digest,
// and any refusals the tool hit while reading it.
type LedgerEntryDoc struct {
	File     string   `json:"file"`
	SHA256   string   `json:"sha256"`
	Refusals []string `json:"refusals"`
}

// AttestationDoc is the committed attestation document.
type AttestationDoc struct {
	// ArtifactID and ArtifactSHA256 name the approved bytes.
	ArtifactID     string `json:"artifact_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`

	// ProcedureCommit and ExecutableSHA256 bind the approval to the same
	// execution the artifact records (§41e's execution binding).
	ProcedureCommit  string `json:"procedure_commit"`
	ExecutableSHA256 string `json:"executable_sha256"`

	// Ledger enumerates what derivation read and what it refused.
	Ledger []LedgerEntryDoc `json:"ledger"`

	// DerivedAt and ApprovedAt bracket the cooling delay.
	DerivedAt  string `json:"derived_at"`
	ApprovedAt string `json:"approved_at"`

	// SignOff names the approver; OutOfBand names the out-of-band
	// confirmation. Both required — §41e says both are named in the file.
	SignOff   string `json:"sign_off"`
	OutOfBand string `json:"out_of_band"`
}

// Validate makes the document loadable.
func (d AttestationDoc) Validate() error {
	if d.ArtifactID == "" {
		return fmt.Errorf("artifact_id: %w", ErrAttestationInvalid)
	}
	if err := checkHashText(d.ArtifactSHA256); err != nil {
		return fmt.Errorf("artifact_sha256: %w", err)
	}
	if d.ProcedureCommit == "" {
		return fmt.Errorf("procedure_commit: %w", ErrAttestationInvalid)
	}
	if err := checkHashText(d.ExecutableSHA256); err != nil {
		return fmt.Errorf("executable_sha256: %w", err)
	}
	if len(d.Ledger) == 0 {
		return fmt.Errorf("ledger: %w", ErrAttestationInvalid)
	}
	for n, e := range d.Ledger {
		if e.File == "" {
			return fmt.Errorf("ledger %d file: %w", n, ErrAttestationInvalid)
		}
		if err := checkHashText(e.SHA256); err != nil {
			return fmt.Errorf("ledger %d: %w", n, err)
		}
	}
	derived, err := parseInstant("derived_at", d.DerivedAt)
	if err != nil {
		return err
	}
	approved, err := parseInstant("approved_at", d.ApprovedAt)
	if err != nil {
		return err
	}
	if approved.Sub(derived) < ApprovalDelay {
		return fmt.Errorf("derived %s, approved %s: %w", d.DerivedAt, d.ApprovedAt, ErrApprovalDelayShort)
	}
	if d.SignOff == "" {
		return fmt.Errorf("sign_off: %w", ErrAttestationInvalid)
	}
	if d.OutOfBand == "" {
		return fmt.Errorf("out_of_band: %w", ErrAttestationInvalid)
	}
	return nil
}

// Attestation is the loaded attestation, carrying its own digest for the
// scored run's evidence.
type Attestation struct {
	Doc AttestationDoc

	// SHA256 is the attestation file's own digest, lowercase hex —
	// recorded in run evidence so the run names WHICH approval authorized
	// it, not merely which numbers it used.
	SHA256 string

	DerivedAt  time.Time
	ApprovedAt time.Time
}

// LoadAttestation turns the attestation's exact bytes into the loaded
// value, digesting them on the way.
func LoadAttestation(data []byte) (*Attestation, error) {
	doc, err := config.Decode[AttestationDoc](data)
	if err != nil {
		return nil, fmt.Errorf("attestation: %w", err)
	}
	derived, err := parseInstant("derived_at", doc.DerivedAt)
	if err != nil {
		return nil, fmt.Errorf("attestation: %w", err)
	}
	approved, err := parseInstant("approved_at", doc.ApprovedAt)
	if err != nil {
		return nil, fmt.Errorf("attestation: %w", err)
	}
	sum := sha256.Sum256(data)
	return &Attestation{
		Doc:        doc,
		SHA256:     hex.EncodeToString(sum[:]),
		DerivedAt:  derived,
		ApprovedAt: approved,
	}, nil
}

// LoadAttestationFile loads a committed attestation from disk.
func LoadAttestationFile(path string) (*Attestation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("attestation file: %w", err)
	}
	return LoadAttestation(data)
}

// ApprovedIdentity is what an attestation actually binds: the four fields
// that together say WHICH bytes, produced by WHICH procedure, running as
// WHICH executable, were approved. It is a value rather than a second
// artifact type because approval is indifferent to what the artifact
// contains — §41e's calibration artifact and §45's cross-venue gate
// artifact are approved by the same act, and a Bind written separately for
// each would be two chances to forget one of the four comparisons.
type ApprovedIdentity struct {
	// ID and ContentSHA256 name the approved bytes.
	ID            string
	ContentSHA256 string

	// ProcedureCommit and ExecutableSHA256 are the execution binding the
	// artifact recorded.
	ProcedureCommit  string
	ExecutableSHA256 string
}

// BindIdentity verifies the attestation approves exactly this identity: same
// ID, same content hash, same procedure commit, same executable digest.
// This is the runner-side check §41e names — a consumer refuses to act on an
// artifact without an attestation that passes it.
//
// All four are compared, and each one alone is the whole check for some
// tamper: a matching hash with a different ID approves a rename, a matching
// ID with a different hash approves an edit, and matching bytes under a
// different commit or executable approve a rerun nobody reviewed.
func (at *Attestation) BindIdentity(id ApprovedIdentity) error {
	if at == nil {
		return fmt.Errorf("attestation: %w", domain.ErrUnset)
	}
	if at.Doc.ArtifactID != id.ID {
		return fmt.Errorf("attestation names %q, artifact is %q: %w",
			at.Doc.ArtifactID, id.ID, ErrAttestationMismatch)
	}
	if at.Doc.ArtifactSHA256 != id.ContentSHA256 {
		return fmt.Errorf("attestation hash %s, artifact bytes hash %s: %w",
			at.Doc.ArtifactSHA256, id.ContentSHA256, ErrAttestationMismatch)
	}
	if at.Doc.ProcedureCommit != id.ProcedureCommit {
		return fmt.Errorf("attestation commit %s, artifact records %s: %w",
			at.Doc.ProcedureCommit, id.ProcedureCommit, ErrAttestationMismatch)
	}
	if at.Doc.ExecutableSHA256 != id.ExecutableSHA256 {
		return fmt.Errorf("attestation executable %s, artifact records %s: %w",
			at.Doc.ExecutableSHA256, id.ExecutableSHA256, ErrAttestationMismatch)
	}
	return nil
}

// Bind is BindIdentity for the step-4 calibration artifact.
func (at *Attestation) Bind(a *Artifact) error {
	if at == nil {
		return fmt.Errorf("attestation: %w", domain.ErrUnset)
	}
	if a == nil {
		return fmt.Errorf("artifact: %w", domain.ErrUnset)
	}
	return at.BindIdentity(ApprovedIdentity{
		ID:               a.Ref.ID,
		ContentSHA256:    a.Ref.Hash,
		ProcedureCommit:  a.Doc.Procedure.Commit,
		ExecutableSHA256: a.Doc.Procedure.ExecutableSHA256,
	})
}
