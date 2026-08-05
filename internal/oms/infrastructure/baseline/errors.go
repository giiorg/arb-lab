package baseline

import "errors"

// The loader's ladder, one sentinel per rung — each names a DIFFERENT operator
// conversation, which is why they are not one "baseline invalid" error.
var (
	// ErrPinIncomplete reports a Pin missing its path or its digest. The digest
	// half IS the two-act rule; a path-only pin would re-open the edit-in-place
	// hole the digest exists to close.
	ErrPinIncomplete = errors.New("baseline: config pin needs both path and sha256")

	// ErrJournalMissing reports a Load with nothing to verify the mark against.
	ErrJournalMissing = errors.New("baseline: journal missing")

	// ErrDialectUnknown reports a dialect this package holds no derivation for.
	ErrDialectUnknown = errors.New("baseline: venue dialect unknown")

	// ErrDigestMismatch reports document bytes that are not the ones the config
	// pins — a replaced or edited file, refused before its content is read.
	ErrDigestMismatch = errors.New("baseline: document does not match the config digest pin")

	// ErrDocumentMalformed reports bytes that are not a well-formed baseline
	// document.
	ErrDocumentMalformed = errors.New("baseline: document malformed")

	// ErrChainMarkMismatch reports a journal whose record at the marked seq is
	// not the record the document attested against — a restored or rebuilt
	// history. (A journal TRUNCATED below the mark is the journal adapter's
	// ErrJournalMarkBeyondHead, passed through named: it is a different
	// conversation.)
	ErrChainMarkMismatch = errors.New("baseline: journal hash at marked seq differs from the recorded mark")

	// ErrFingerprintUnset reports bindings carrying no credential fingerprint.
	ErrFingerprintUnset = errors.New("baseline: bindings carry no credential fingerprint")

	// ErrCredentialMismatch reports a document recorded under a different
	// spending credential — the account label may match while the credential
	// reaches another subaccount, so rotation forces a re-record, stated.
	ErrCredentialMismatch = errors.New("baseline: document recorded under a different credential")

	// ErrVenueMismatch and ErrAccountMismatch report a document describing a
	// different deployment.
	ErrVenueMismatch   = errors.New("baseline: document describes a different venue")
	ErrAccountMismatch = errors.New("baseline: document describes a different account")

	// ErrOrderOutsidePrefix reports a stored order row naming an order the
	// chain-marked journal prefix does not hold — rows from a different
	// history than the mark names.
	ErrOrderOutsidePrefix = errors.New("baseline: order row is outside the chain-marked journal prefix")

	// ErrOrderRowSideDisagrees reports a stored order row whose side
	// contradicts the journal prefix's planned record for the same id. The
	// side is the one row field the journal can recompute, and a flip inverts
	// the row's whole contribution to `ours` — the silent sign error — so it
	// is recomputed rather than trusted (§70i).
	ErrOrderRowSideDisagrees = errors.New("baseline: order row side contradicts the journal prefix")

	// ErrStoredAbsenceUninterpretable reports an absent balance row stored
	// under a dialect whose enumeration returns every asset — a silence that
	// dialect's venue could not have answered with, so no zero may be
	// re-derived from it (§70i).
	ErrStoredAbsenceUninterpretable = errors.New("baseline: stored absent row under a dialect whose silence concludes nothing")

	// ErrStoredRowUnpinnable reports a stored balance row whose words the
	// per-venue derivation cannot pin. The record command never writes one, so
	// the document was made some other way.
	ErrStoredRowUnpinnable = errors.New("baseline: stored balance row is unpinnable")
)
