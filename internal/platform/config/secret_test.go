package config_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

func TestParseSecretRefAccepts(t *testing.T) {
	tests := []struct {
		in     string
		scheme string
		name   string
	}{
		{"env:BINANCE_API_KEY", config.SchemeEnv, "BINANCE_API_KEY"},
		{"env:_leading_underscore", config.SchemeEnv, "_leading_underscore"},
		{"env:mixedCase9", config.SchemeEnv, "mixedCase9"},
		{"file:/run/secrets/bybit.key", config.SchemeFile, "/run/secrets/bybit.key"},
		{"file:/a", config.SchemeFile, "/a"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			ref, err := config.ParseSecretRef(tc.in)
			if err != nil {
				t.Fatalf("ParseSecretRef(%q) = %v", tc.in, err)
			}
			if ref.Scheme() != tc.scheme || ref.Name() != tc.name {
				t.Errorf("got %s / %s, want %s / %s", ref.Scheme(), ref.Name(), tc.scheme, tc.name)
			}
			if err := ref.Validate(); err != nil {
				t.Errorf("Validate = %v", err)
			}
			if got := ref.String(); got != tc.in {
				t.Errorf("String = %q, want %q (a reference must round-trip)", got, tc.in)
			}
		})
	}
}

// TestParseSecretRefRejectsAnInlineSecret is the reason the type exists.
//
// §5.2 says "key refs (never inline secrets)". This is not enforced by asking
// nicely: a pasted credential has no scheme, so it fails to parse, and a
// document containing one does not boot. The API-key shapes below are the real
// ones — Binance's is 64 alphanumerics, Bybit's is 18 — and none of them
// contains a colon.
func TestParseSecretRefRejectsAnInlineSecret(t *testing.T) {
	inline := []string{
		strings.Repeat("A9f2Kd7Q", 8),      // a Binance-shaped 64-char key
		"XxK9pQ2mNvL4TrWs8B",               // a Bybit-shaped key
		"sk-live-9c3f4a2b1d",               // an API token with dashes
		"hunter2",                          // the classic
		"",                                 // an unset field
		"BINANCE_API_KEY",                  // the NAME without a scheme
		"/run/secrets/bybit.key",           // a PATH without a scheme
		"aGVsbG8gd29ybGQgdGhpcyBpcyBhIGtl", // base64, no padding
	}

	for _, in := range inline {
		if _, err := config.ParseSecretRef(in); !errors.Is(err, config.ErrSecretRefMalformed) {
			t.Errorf("ParseSecretRef(%q) = %v, want ErrSecretRefMalformed", in, err)
		}
	}
}

// TestParseSecretRefDoesNotEchoWhatItRejected is the invariant that makes the
// check safe to run on hostile input. The value it refuses is, by construction,
// the one case that is definitely a credential — so an error message quoting it
// would put the secret in the log at exactly the moment somebody pasted one.
func TestParseSecretRefDoesNotEchoWhatItRejected(t *testing.T) {
	const secret = "SECRET-SENTINEL-7d41f0ab"

	_, err := config.ParseSecretRef(secret)
	if err == nil {
		t.Fatal("ParseSecretRef accepted a bare secret")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q echoes the rejected value", err)
	}

	// The same through a document, where the value passes through the decoder
	// on its way to the error.
	_, decErr := config.Decode[testConfig]([]byte(withField(fBinanceKey, `"`+secret+`"`)))
	if decErr == nil {
		t.Fatal("Decode accepted a bare secret in a SecretRef field")
	}
	if strings.Contains(decErr.Error(), secret) {
		t.Errorf("decode error %q echoes the rejected value", decErr)
	}
}

func TestParseSecretRefRejectsMalformedReferences(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"an unknown scheme", "vault:secret/binance"},
		{"an empty scheme", ":BINANCE_API_KEY"},
		{"an empty env name", "env:"},
		{"an env name starting with a digit", "env:9KEY"},
		{"an env name with a dash", "env:BINANCE-API-KEY"},
		{"an env name with a space", "env:BINANCE KEY"},
		{"an empty file path", "file:"},
		{"a relative file path", "file:secrets/bybit.key"},
		{"a bare-tilde file path", "file:~/secrets/bybit.key"},
		{"an uppercased scheme", "ENV:BINANCE_API_KEY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.ParseSecretRef(tc.in); !errors.Is(err, config.ErrSecretRefMalformed) {
				t.Errorf("ParseSecretRef(%q) = %v, want ErrSecretRefMalformed", tc.in, err)
			}
		})
	}
}

// TestSecretRefZeroValueIsUnusable is §11's rule applied here. An unpopulated
// key reference must not read as "look up the empty environment variable" — it
// has to fail, at boot, saying so.
func TestSecretRefZeroValueIsUnusable(t *testing.T) {
	var ref config.SecretRef

	err := ref.Validate()
	if !errors.Is(err, config.ErrSecretRefMalformed) {
		t.Fatalf("zero SecretRef Validate = %v, want ErrSecretRefMalformed", err)
	}
	// The message matters, not only the sentinel. Removing the unset check
	// leaves Validate reparsing String()'s "<unset>", which also fails
	// ErrSecretRefMalformed — with a message telling the operator their
	// reference is missing a scheme, when in fact the field is missing. A
	// mutant that deleted the check survived on the sentinel alone.
	if !strings.Contains(err.Error(), "unset") {
		t.Errorf("error %q does not say the reference is unset — a different check fired", err)
	}
	if got := ref.String(); got != "<unset>" {
		t.Errorf("zero SecretRef String = %q, want <unset>", got)
	}
	if _, err := ref.MarshalJSON(); err == nil {
		t.Error("MarshalJSON accepted the zero SecretRef")
	}
}

func TestSecretRefJSONRoundTrip(t *testing.T) {
	for _, in := range []string{"env:BINANCE_API_KEY", "file:/run/secrets/bybit.key"} {
		ref, err := config.ParseSecretRef(in)
		if err != nil {
			t.Fatalf("ParseSecretRef(%q) = %v", in, err)
		}

		encoded, err := json.Marshal(ref)
		if err != nil {
			t.Fatalf("Marshal = %v", err)
		}
		if string(encoded) != `"`+in+`"` {
			t.Errorf("Marshal = %s, want %q", encoded, in)
		}

		var back config.SecretRef
		if err := json.Unmarshal(encoded, &back); err != nil {
			t.Fatalf("Unmarshal = %v", err)
		}
		if back != ref {
			t.Errorf("round trip: %v -> %v", ref, back)
		}
	}
}

// TestSecretRefRejectsNonStringJSON keeps the field's wire type pinned. Without
// it a `"key_ref": {"env": "K"}` shape would decode into the zero value on some
// future refactor and fail much later, as a lookup of nothing.
func TestSecretRefRejectsNonStringJSON(t *testing.T) {
	for _, in := range []string{`123`, `true`, `null`, `{"env":"K"}`, `["env:K"]`} {
		var ref config.SecretRef
		if err := json.Unmarshal([]byte(in), &ref); !errors.Is(err, config.ErrSecretRefMalformed) {
			t.Errorf("Unmarshal(%s) = %v, want ErrSecretRefMalformed", in, err)
		}
	}
}
