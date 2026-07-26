package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// SecretRef is §5.2's "key refs (never inline secrets)" made into a type.
//
// # It cannot hold a secret
//
// That is the whole design, not a usage convention. A SecretRef holds a scheme
// and a name — env:BINANCE_API_KEY, file:/run/secrets/bybit.key — and there is
// no field for a value and no method that produces one. So:
//
//   - an inline credential FAILS TO PARSE, because a credential has no scheme.
//     The rule is not "please do not paste secrets here"; a pasted secret is a
//     malformed document and the process does not boot.
//   - printing one is safe by construction. String, MarshalJSON and any %v in a
//     log line render the reference. There is nothing else in there to leak, so
//     redaction is not a code path that can be forgotten.
//
// # Resolution lives elsewhere
//
// Turning a reference into a credential needs os.Getenv or a file read, and its
// own failure and audit semantics. It belongs in the composition root. A kernel
// type that could resolve is a kernel type that could hold a resolved value, and
// this package's invariant is that it prints no values at all — an invariant
// worth more than the convenience of a Resolve method here.
//
// # Two rules the schemes carry
//
// The scheme set is CLOSED — env and file — so adding a backend is a deliberate
// edit rather than a string that happens to parse. And a file reference must be
// ABSOLUTE: a relative secret path resolves against a working directory the
// daemon does not choose, which on a systemd unit or a container entrypoint is
// not the directory the operator was standing in when they wrote it.
type SecretRef struct {
	scheme string
	name   string
}

// The recognised schemes. Closed on purpose; see the type doc.
const (
	SchemeEnv  = "env"
	SchemeFile = "file"
)

// ParseSecretRef parses "<scheme>:<name>".
func ParseSecretRef(s string) (SecretRef, error) {
	scheme, name, found := strings.Cut(s, ":")
	if !found {
		// The message deliberately does not echo s: an inline credential is
		// exactly the input that reaches this branch.
		return SecretRef{}, fmt.Errorf("%w: expected \"<scheme>:<name>\" with scheme %s or %s",
			ErrSecretRefMalformed, SchemeEnv, SchemeFile)
	}

	switch scheme {
	case SchemeEnv:
		if !validEnvName(name) {
			return SecretRef{}, fmt.Errorf("%w: %s: name must match [A-Za-z_][A-Za-z0-9_]*", ErrSecretRefMalformed, SchemeEnv)
		}
	case SchemeFile:
		if name == "" {
			return SecretRef{}, fmt.Errorf("%w: %s: empty path", ErrSecretRefMalformed, SchemeFile)
		}
		if !filepath.IsAbs(name) {
			return SecretRef{}, fmt.Errorf("%w: %s: path must be absolute", ErrSecretRefMalformed, SchemeFile)
		}
	default:
		return SecretRef{}, fmt.Errorf("%w: unknown scheme (recognised: %s, %s)", ErrSecretRefMalformed, SchemeEnv, SchemeFile)
	}

	return SecretRef{scheme: scheme, name: name}, nil
}

// Scheme is where the credential lives: SchemeEnv or SchemeFile.
func (r SecretRef) Scheme() string { return r.scheme }

// Name is the variable name or file path the credential lives at. Never the
// credential.
func (r SecretRef) Name() string { return r.name }

// Validate reports whether the reference names anything. The zero SecretRef
// fails, so a schema field nobody populated is a boot failure rather than a
// lookup of the empty environment variable.
func (r SecretRef) Validate() error {
	if r.scheme == "" || r.name == "" {
		return fmt.Errorf("%w: unset", ErrSecretRefMalformed)
	}
	_, err := ParseSecretRef(r.String())
	return err
}

// String is the reference itself. Safe to log — see the type doc.
func (r SecretRef) String() string {
	if r.scheme == "" && r.name == "" {
		return "<unset>"
	}
	return r.scheme + ":" + r.name
}

// MarshalJSON round-trips the reference as the string an operator wrote.
func (r SecretRef) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.String())
}

// UnmarshalJSON accepts only a JSON string, so a schema field of this type
// cannot be populated from a number or an object.
func (r *SecretRef) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: expected a JSON string", ErrSecretRefMalformed)
	}
	ref, err := ParseSecretRef(s)
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

func validEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
