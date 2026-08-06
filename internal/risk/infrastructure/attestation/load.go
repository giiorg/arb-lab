package attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// Pin is §5.2's `portfolio_margin_attestation` config field: the document's
// path AND its digest.
//
// The digest is what makes writing the document and trusting it two acts. There
// is no `Write` in this package to guard with O_EXCL (see the package comment),
// so the digest is not a second line of defence here — it is the only one.
type Pin struct {
	Path   string
	SHA256 string
}

// Bindings is what the deployment knows that the document must agree with.
type Bindings struct {
	// Venue and Account are the deployment config's.
	Venue   ids.Venue
	Account string

	// Fingerprint is the constructed transport's credential fingerprint — the
	// digest the spend lock computes.
	Fingerprint string
}

// Load reads and verifies the pinned attestation, returning the operator's word
// in the domain's vocabulary.
//
// The ladder, each rung its own named refusal, in the order the claims depend on
// each other:
//
//  1. digest — the bytes are the ones the config pins (the two-act rule);
//  2. shape — the bytes are an attestation document;
//  3. credential fingerprint — the document is about the account this process
//     can actually reach, not merely the one the label names;
//  4. venue and account label — the deployment is the one described.
//
// Only then is the observed word translated. Every rung is an error, never a
// refusing attestation: see errors.go for why the two must not blur.
//
// # What this function CANNOT check, and it is the interesting part
//
// #179's baseline carries a journal chain mark, so the document is falsifiable
// — a history that moved refuses it. Nothing here plays that role. The bindings
// answer "is this document about this account", and the only event that
// invalidates the FACT — somebody enabling Portfolio Margin — reaches no
// endpoint this system reads and produces no signal anywhere (record §73k F10).
// So a loaded attestation is a point-in-time claim whose entire strength is the
// operator's word plus the deliberateness of the two acts. That is recorded
// rather than mitigated because the available mitigations are worse: a time
// bound was ruled out by §73c-1, and inferring the fact from a sibling endpoint's
// behaviour is the venue reasoning §73c-1 rejected for being unread.
func Load(pin Pin, bind Bindings) (domain.Attestation, error) {
	if pin.Path == "" || pin.SHA256 == "" {
		return domain.AttestationUnstated,
			fmt.Errorf("attestation pin path=%q sha256=%q: %w", pin.Path, pin.SHA256, ErrPinIncomplete)
	}

	body, err := os.ReadFile(pin.Path)
	if err != nil {
		return domain.AttestationUnstated, fmt.Errorf("attestation document: %w", err)
	}

	// 1. Digest, before parsing: a mismatch means these are not the pinned
	// bytes, and diagnosing their content would describe a document the operator
	// never pinned.
	digest := sha256.Sum256(body)
	if got := hex.EncodeToString(digest[:]); !strings.EqualFold(got, pin.SHA256) {
		return domain.AttestationUnstated,
			fmt.Errorf("document at %s digests %s, config pins %s: %w",
				pin.Path, got, strings.ToLower(pin.SHA256), ErrDigestMismatch)
	}

	// 2. Shape.
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return domain.AttestationUnstated,
			fmt.Errorf("attestation document: %w: %w", err, ErrDocumentMalformed)
	}
	if err := doc.Validate(); err != nil {
		return domain.AttestationUnstated, fmt.Errorf("attestation document: %w", err)
	}

	// 3. Credential fingerprint.
	if bind.Fingerprint == "" {
		return domain.AttestationUnstated, ErrFingerprintUnset
	}
	if doc.CredentialSHA256 != bind.Fingerprint {
		return domain.AttestationUnstated,
			fmt.Errorf("document attested under credential %s, transport holds %s: %w",
				doc.CredentialSHA256, bind.Fingerprint, ErrCredentialMismatch)
	}

	// 4. Venue and account label.
	if err := bind.Venue.Validate(); err != nil {
		return domain.AttestationUnstated, fmt.Errorf("attestation bindings venue: %w", err)
	}
	if doc.Venue != string(bind.Venue) {
		return domain.AttestationUnstated,
			fmt.Errorf("document describes venue %q, deployment is %q: %w",
				doc.Venue, bind.Venue, ErrVenueMismatch)
	}
	if bind.Account == "" {
		return domain.AttestationUnstated, fmt.Errorf("attestation bindings account: %w", ErrAccountMismatch)
	}
	if doc.Account != bind.Account {
		return domain.AttestationUnstated,
			fmt.Errorf("document describes account %q, deployment is %q: %w",
				doc.Account, bind.Account, ErrAccountMismatch)
	}

	// The document's own word, translated and not judged. Whether `on` is
	// acceptable is risk/domain's rule table's question, and asking it here
	// would put the answer in two places.
	return doc.Observed.Attestation()
}
