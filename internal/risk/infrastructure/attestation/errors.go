package attestation

import "errors"

// The attestation loader's refusals. Every one of them is a WIRING failure
// rather than a posture one: a document that will not load stops the boot with
// an error, and only a document that loads cleanly can go on to say `on` and
// refuse the gate by name.
//
// The distinction is #179's and it is worth restating, because this package is
// the place it would blur. "The operator has not attested" is not in this list
// — it is the ABSENCE of a pin, it produces domain.AttestationUnstated, and the
// gate reports it as ReasonUnattested. A pinned document that fails to load is
// the opposite situation: somebody stated something and this code cannot read
// it, which must never decay into "nobody stated anything".
var (
	// ErrPinIncomplete reports a pin missing its path or its digest.
	//
	// Both halves are required because the digest is the entire second act: a
	// path-only pin makes editing the document in place change what the gate
	// consumes, with no config edit and no trace.
	ErrPinIncomplete = errors.New("risk: portfolio-margin attestation pin needs both a path and a sha256")

	// ErrDigestMismatch reports bytes on disk that are not the ones the config
	// pins — the two-act rule biting.
	ErrDigestMismatch = errors.New("risk: portfolio-margin attestation digest does not match the config pin")

	// ErrDocumentMalformed reports a file that is not an attestation document:
	// wrong kind or version, a missing binding, an unreadable instant, or an
	// observation that is neither `off` nor `on`.
	ErrDocumentMalformed = errors.New("risk: portfolio-margin attestation document is malformed")

	// ErrFingerprintUnset reports a load attempted with no credential
	// fingerprint to bind against.
	//
	// An error rather than a skipped check: a binding that silently does not run
	// when its input is missing is the same as no binding, and the input is
	// missing exactly on the paths that did not construct a real transport.
	ErrFingerprintUnset = errors.New("risk: portfolio-margin attestation bindings carry no credential fingerprint")

	// ErrCredentialMismatch reports a document attested under a different
	// credential than the one this process holds. See Document.CredentialSHA256
	// for why the binding is conservative under both readings of Portfolio
	// Margin's scope.
	ErrCredentialMismatch = errors.New("risk: portfolio-margin attestation was recorded under a different credential")

	// ErrVenueMismatch and ErrAccountMismatch report a document describing a
	// different deployment than the one loading it.
	ErrVenueMismatch   = errors.New("risk: portfolio-margin attestation describes a different venue")
	ErrAccountMismatch = errors.New("risk: portfolio-margin attestation describes a different account")
)
