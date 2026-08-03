package reconcile

import (
	"fmt"
	"os"
	"strings"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// resolveSecret turns a §5.2 key ref into a value.
//
// # It lives HERE because platform/config forbade itself from having it
//
// config.SecretRef's own doc: "Turning a reference into a credential needs
// os.Getenv or a file read, and its own failure and audit semantics. It belongs
// in the composition root. A kernel type that could resolve is a kernel type that
// could hold a resolved value, and this package's invariant is that it prints no
// values at all." This function is that scheduled arrival, and the invariant it
// preserves is that the resolved string exists in exactly two places — this
// frame, and transport.Credential, which has no accessor for it.
//
// # An empty resolution is a REFUSAL, and it is the one worth the extra sentinel
//
// `export BINANCE_API_SECRET=` is a set variable, so os.Getenv returns "" with no
// error and os.LookupEnv reports it present. The credential built from it signs
// correctly over nothing, the venue answers with an authentication failure, and
// what an operator reads is "wrong or revoked key" — which sends them to the
// exchange's console rather than to the unit file on this host. Refusing here
// costs one comparison and names the variable.
func resolveSecret(ref config.SecretRef) (string, error) {
	switch ref.Scheme() {
	case config.SchemeEnv:
		value, ok := os.LookupEnv(ref.Name())
		if !ok {
			return "", fmt.Errorf("%s is not set in this process's environment: %w",
				ref.Name(), ErrSecretUnresolved)
		}
		if value == "" {
			return "", fmt.Errorf("%s: %w", ref.Name(), ErrSecretEmpty)
		}
		return value, nil
	case config.SchemeFile:
		data, err := os.ReadFile(ref.Name())
		if err != nil {
			// The path is safe to echo — it is a reference, not a value — and it is
			// the only thing that distinguishes "the secret file moved" from "the
			// mount is not there yet".
			return "", fmt.Errorf("%s: %v: %w", ref.Name(), err, ErrSecretUnresolved)
		}
		// Trimmed, because a secret file written by an editor or by `echo` carries a
		// trailing newline and a signature computed over one is wrong in a way no
		// error message anywhere would name. Trimming is not guessing: no venue
		// credential contains leading or trailing whitespace, and the alternative —
		// signing with the newline — fails as an authentication error identical to a
		// revoked key.
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("%s: %w", ref.Name(), ErrSecretEmpty)
		}
		return value, nil
	}
	// Unreachable while ParseSecretRef holds the scheme set closed, and refused
	// rather than dropped: a scheme added to that kernel type without a resolver
	// here must fail loudly, not resolve to the empty string.
	return "", fmt.Errorf("%s: %w", ref.Scheme(), ErrSecretSchemeUnknown)
}

// credential resolves both refs into the one type that may hold them.
func (v VenueConfig) credential() (transport.Credential, error) {
	key, err := resolveSecret(v.APIKeyRef)
	if err != nil {
		return transport.Credential{}, fmt.Errorf("api_key_ref: %w", err)
	}
	secret, err := resolveSecret(v.APISecretRef)
	if err != nil {
		return transport.Credential{}, fmt.Errorf("api_secret_ref: %w", err)
	}
	return transport.NewCredential(key, secret)
}

// collaborators is the four-piece set one Dialect names.
//
// A struct rather than four returns, so that a caller cannot take three of them
// and build the fourth itself — which is the mixture Dialect exists to make
// inexpressible. See Dialect for what a Binance signer with a Bybit envelope
// does to a rejected order.
type collaborators struct {
	codec    venue.OrderCodec
	signer   transport.Signer
	limits   transport.LimitReader
	envelope transport.EnvelopeReader
}

// collaboratorsFor builds the dialect's four pieces over one credential.
func collaboratorsFor(d Dialect, v ids.Venue, cred transport.Credential) (collaborators, error) {
	switch d {
	case DialectBinance:
		codec, err := venue.NewBinanceOrderCodec(v)
		if err != nil {
			return collaborators{}, err
		}
		signer, err := transport.NewBinanceSigner(v, cred)
		if err != nil {
			return collaborators{}, err
		}
		return collaborators{
			codec:    codec,
			signer:   signer,
			limits:   transport.BinanceLimitReader{},
			envelope: transport.BinanceEnvelope{},
		}, nil
	case DialectBybit:
		codec, err := venue.NewBybitOrderCodec(v)
		if err != nil {
			return collaborators{}, err
		}
		signer, err := transport.NewBybitSigner(v, cred)
		if err != nil {
			return collaborators{}, err
		}
		return collaborators{
			codec:    codec,
			signer:   signer,
			limits:   transport.BybitLimitReader{},
			envelope: transport.BybitEnvelope{},
		}, nil
	}
	return collaborators{}, fmt.Errorf("dialect %q: %w", string(d), ErrDialectUnknown)
}

// newOrderClient builds the venue surface §7.5's join reads through.
//
// # ONE governor for the venue, shared by every family's transport
//
// §7.6 keeps separate weight and order counters per product family under a
// SHARED IP-ban envelope per venue, and the sharing is what a second governor
// would break: two governors are two IP envelopes for one IP, each seeing half
// the traffic and neither seeing the ban coming. The profile carries both scopes,
// so one governor is not a simplification — it is the only wiring that can hold
// ScopeVenue at all.
func newOrderClient(cfg Config, clk clock.Clock) (*venue.OrderClient, error) {
	cred, err := cfg.Venue.credential()
	if err != nil {
		return nil, err
	}
	parts, err := collaboratorsFor(cfg.Venue.Dialect, cfg.Venue.Venue, cred)
	if err != nil {
		return nil, err
	}
	profile, err := cfg.Venue.profile()
	if err != nil {
		return nil, err
	}
	governor, err := ratelimit.New(clk, profile)
	if err != nil {
		return nil, err
	}

	transports := make(map[ids.ProductCategory]*transport.SignedClient, len(cfg.Venue.Endpoints))
	for _, e := range cfg.Venue.Endpoints {
		client, err := transport.NewSignedClient(transport.SignedClientConfig{
			Scheme:     e.Scheme,
			Host:       e.Host,
			Timeout:    cfg.Venue.Timeout.Duration(),
			ReadLimit:  cfg.Venue.ReadLimit,
			RecvWindow: cfg.Venue.RecvWindow.Duration(),
			Signer:     parts.signer,
			Limits:     parts.limits,
			Envelope:   parts.envelope,
			Governor:   governor,
			Clock:      clk,
		})
		if err != nil {
			return nil, fmt.Errorf("endpoint %s: %w", e.Product, err)
		}
		transports[e.Product] = client
	}
	return venue.NewOrderClient(venue.OrderClientConfig{Codec: parts.codec, Transports: transports})
}
