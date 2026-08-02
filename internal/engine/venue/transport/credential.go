package transport

import (
	"errors"
	"fmt"
)

// Credential is a resolved venue API credential: the key that identifies us and
// the secret that proves it.
//
// # The secret cannot leave this package, and that is structural
//
// There is no exported accessor for the secret and no method that returns it.
// The signers read c.secret directly because they are in this package; anything
// outside it cannot, at compile time. platform/config.SecretRef holds the same
// line one layer earlier — it cannot hold a secret at all — and this type is
// where a resolved one comes to rest. Between them the credential exists in
// exactly one package, and §7.3's blast-radius argument is enforced by the type
// system rather than by review.
//
// The consequence worth noticing is the one that shaped the API: the signing
// functions could not have been put in a sibling package "for tidiness" without
// exporting the secret, so the boundary chose the layout rather than the other
// way round.
//
// # Printing it is safe by construction
//
// String renders the key's shape and never the secret, so a %v or %s on a
// Credential — or on a struct holding one, since fmt applies Stringer to fields
// — prints a redaction. Redaction is not a code path that can be forgotten,
// because there is no code path that emits the secret at all.
type Credential struct {
	key    string
	secret []byte
}

// NewCredential builds a credential from a resolved key and secret.
//
// Both are required. An empty secret is refused rather than accepted as an
// unauthenticated client, because the failure it produces otherwise is a
// correctly-formed signature over the empty key — a well-formed request the
// venue rejects on credentials, which reads to an operator as a wrong or
// revoked key rather than as a wiring fault at boot.
func NewCredential(key, secret string) (Credential, error) {
	if key == "" {
		return Credential{}, ErrCredentialKeyUnset
	}
	if secret == "" {
		return Credential{}, ErrCredentialSecretUnset
	}
	return Credential{key: key, secret: []byte(secret)}, nil
}

// Key is the API key. It is not a secret on either venue — it travels in a
// plaintext header on every request, and on Bybit it is additionally part of the
// signed string — so it is readable, unlike the secret beside it.
func (c Credential) Key() string { return c.key }

// Validate reports whether the credential could sign. The zero value fails, so a
// client constructed with the field forgotten refuses at boot rather than at the
// first order.
func (c Credential) Validate() error {
	if c.key == "" {
		return ErrCredentialKeyUnset
	}
	if len(c.secret) == 0 {
		return ErrCredentialSecretUnset
	}
	return nil
}

// String is a redaction. It names the key by its length alone.
//
// The length rather than a prefix: a prefix is the part an operator would
// recognise, which is precisely what makes it worth pasting into a bug report.
// A length is enough to tell "the wrong variable was resolved" from "no variable
// was resolved", which is the question a log line is actually being asked.
func (c Credential) String() string {
	if c.key == "" && len(c.secret) == 0 {
		return "<unset credential>"
	}
	return fmt.Sprintf("<credential key:%dB secret:%dB>", len(c.key), len(c.secret))
}

// GoString is the same redaction under %#v.
//
// Without it, %#v prints the struct's unexported fields verbatim — including
// the secret — and %#v is what a debugger, a test failure message and
// spew-style dumping all reach for. String alone does not cover that verb, so
// the two exist together or the redaction has a hole in the one place people
// look when something is already going wrong.
func (c Credential) GoString() string { return c.String() }

// MarshalJSON refuses.
//
// A credential inside anything serialized is a mistake, and the two available
// answers are "emit a redaction" and "fail". Failing is right: a redaction in a
// config dump round-trips into a Credential whose secret is the literal string
// "<credential ...>", which signs, is rejected by the venue, and looks like a
// key problem. An error names the bug where it happens.
func (c Credential) MarshalJSON() ([]byte, error) {
	return nil, ErrCredentialNotSerializable
}

// The credential's refusals.
var (
	ErrCredentialKeyUnset        = errors.New("transport: credential api key unset")
	ErrCredentialSecretUnset     = errors.New("transport: credential secret unset")
	ErrCredentialNotSerializable = errors.New("transport: a credential must not be serialized")
)
