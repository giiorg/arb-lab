// Package attestation is the Portfolio-Margin attestation's document: the
// stored form of the one §4.1 check no venue read path answers, and the
// verifying loader the boot runs before it governs anything (task #198,
// record §73c-1).
//
// # Why a document exists for exactly one setting
//
// Profiles §2's Binance assert-or-refuse cell lists six checks. Five name a
// `GET`; "Portfolio-Margin OFF" names none, in either document (record §73b
// F1) — and it is the single setting that would falsify §4.1's whole premise,
// because Portfolio Margin folds the spot wallet into futures collateral. So it
// cannot be the check that silently passes, and with no read path the only
// available evidence is an operator who looked. The gate consumes that word and
// refuses in its absence.
//
// # The document records what was OBSERVED, never the verdict
//
// #179's stored-form rule, applied to a human source. The file says *who looked,
// when, how, and what they saw* — and "what they saw" is `on` or `off`, not
// "posture OK". The RULE that Portfolio Margin must be off lives in
// `risk/domain`'s table and nowhere else, so an attestation recording `on`
// produces a named refusal rather than a pass. A document that could carry the
// conclusion would be a file whose author decides whether the gate opens.
//
// # There is no writer here, deliberately
//
// #179 needed `arblab baseline record` because minting a baseline meant reading
// the venue's wallet and the journal's head — machine work a human cannot do by
// hand. This document has no inputs a machine holds: every field is the
// operator's own word, and the digest for the config pin is one `sha256sum`.
// A `Write` beside `Load` would be a producer with no caller, which is the
// shape record §73b F6 refused one layer up. The loader's refusals are named
// per field precisely because hand-authoring is the intended path.
//
// # The two-act rule still binds without a writer
//
// Writing the file and trusting it stay separate acts, because the config pins
// the DIGEST as well as the path (record §70b, review round 2). Editing the
// document in place changes the bytes and the boot refuses by name until the
// second act — the config edit — is taken deliberately.
package attestation

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// Kind and Version identify the document shape. A file that does not carry them
// is not an attestation, whatever else it parses as.
const (
	Kind    = "arblab-portfolio-margin-attestation"
	Version = 1
)

// Observed is what the operator saw, in the document's own spelling.
//
// # There is no spelling for "unstated", and that is the design
//
// `risk/domain`'s Attestation has three members and the zero one is UNSTATED —
// but a document that exists and declines to say what was seen is not an
// attestation of anything, and admitting one here would let a file satisfy the
// pin while establishing nothing. Unstated is the absence of a PIN, decided one
// level up by the composition root; inside a document the only members are the
// two an operator can actually have observed.
type Observed string

const (
	// ObservedOff is Portfolio Margin seen switched off — §4.1's premise holds.
	ObservedOff Observed = "off"

	// ObservedOn is Portfolio Margin seen switched ON. It is a legal document
	// and a REFUSING one: an operator who looked and found it enabled must be
	// able to record what they saw, because the alternative is an operator with
	// a true observation and no way to write it down except by not writing.
	ObservedOn Observed = "on"
)

// Attestation translates the observed word into the domain's vocabulary.
//
// Default-deny: an unrecognised word is an error rather than a refusing
// attestation. The two are not interchangeable — a refusal tells the operator
// their account is wrong, and this tells them their document is.
func (o Observed) Attestation() (domain.Attestation, error) {
	switch o {
	case ObservedOff:
		return domain.AttestationOff, nil
	case ObservedOn:
		return domain.AttestationOn, nil
	}
	return domain.AttestationUnstated,
		fmt.Errorf("observed %q, want %q or %q: %w", string(o), ObservedOff, ObservedOn, ErrDocumentMalformed)
}

// Document is one Portfolio-Margin attestation: what a human established about
// an account, bound to the deployment that will consume it.
type Document struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	// Venue and Account bind the document to the deployment: the attestation is
	// a claim about ONE subaccount at one exchange.
	Venue   string `json:"venue"`
	Account string `json:"account"`

	// CredentialSHA256 is the spending credential's fingerprint at attestation
	// time — the same digest the spend lock computes, and #179's binding taken
	// verbatim because its argument holds here unchanged: the account LABEL is
	// mutable configuration, so a credential swapped to a different subaccount
	// under an unchanged label would pass every other binding.
	//
	// # It is conservative under every reading of Portfolio Margin's scope
	//
	// Whether Binance's Portfolio Margin is a property of the master account or
	// of each subaccount is NOT established in either document, and §73c-1
	// forbade arguing from venue behaviour nobody here has read. This binding
	// does not need the answer: under the narrower reading it is exactly right,
	// and under the wider one it merely refuses more often than strictly
	// necessary. The cost is that key rotation forces a re-attestation — the
	// same cost #179 accepted, in the same direction.
	CredentialSHA256 string `json:"credential_sha256"`

	// ObservedAt is when the operator looked, RFC3339.
	//
	// A diagnosis anchor and never a staleness gate: §73c-1 ruled that an
	// attestation is invalidated by events, never by time, so nothing here
	// compares it against a clock. See ErrDocumentMalformed's neighbours in
	// errors.go for what this document can and cannot be falsified by.
	ObservedAt string `json:"observed_at"`

	// Observer is who looked — a person or a role, in whatever spelling the
	// deployment audits by.
	//
	// Required. An attestation is the one input in this system with no machine
	// provenance at all, so the name is the ONLY thing distinguishing it from an
	// anonymous assertion, and an unsigned claim is not evidence a later
	// investigation can follow.
	Observer string `json:"observer"`

	// Method is how the observation was made — which screen, which endpoint,
	// which call. Free text, required, and never parsed.
	//
	// It is the inputs half of the inputs-not-conclusions rule. `off` alone is a
	// bare assertion; `off` plus "binance.com → Portfolio Margin page, account
	// not enrolled, 2026-08-06" is a claim the next operator can go and re-check.
	// Unparsed on purpose: the moment code branched on it, the operator's
	// description would become a protocol, and a wrong description would become
	// a wrong branch instead of a readable line.
	Method string `json:"method"`

	// Observed is what was seen: `off` or `on`. See Observed.
	Observed Observed `json:"observed"`
}

// Validate reports whether the document is shaped like an attestation.
//
// SHAPE only, on the loader's ladder: whether the bindings are TRUE of this
// deployment is Load's job, against the config and the constructed transport.
func (d Document) Validate() error {
	if d.Kind != Kind {
		return fmt.Errorf("kind %q, want %q: %w", d.Kind, Kind, ErrDocumentMalformed)
	}
	if d.Version != Version {
		return fmt.Errorf("version %d, want %d: %w", d.Version, Version, ErrDocumentMalformed)
	}
	if d.Venue == "" {
		return fmt.Errorf("venue unset: %w", ErrDocumentMalformed)
	}
	if d.Account == "" {
		return fmt.Errorf("account unset: %w", ErrDocumentMalformed)
	}
	if d.CredentialSHA256 == "" {
		return fmt.Errorf("credential fingerprint unset: %w", ErrDocumentMalformed)
	}
	if _, err := time.Parse(time.RFC3339, d.ObservedAt); err != nil {
		return fmt.Errorf("observed_at %q: %w: %w", d.ObservedAt, err, ErrDocumentMalformed)
	}
	if d.Observer == "" {
		return fmt.Errorf("observer unset: %w", ErrDocumentMalformed)
	}
	if d.Method == "" {
		return fmt.Errorf("method unset: %w", ErrDocumentMalformed)
	}
	// Last, so that a document failing several checks names a binding before it
	// names the observation: a file with the wrong account is the operator's
	// wiring problem whatever it observed.
	if _, err := d.Observed.Attestation(); err != nil {
		return err
	}
	return nil
}
