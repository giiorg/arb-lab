package transport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is the signed half of §4's venue contract: the composition each
// venue requires over a request before it will act on it. Profiles §6 holds the
// verified facts and their sources; the claims below cite it rather than
// restating a URL.
//
// # Why the signers are pure functions of (request, credential, server time)
//
// A signer that read a clock could not be checked against Binance's published
// test vectors, because those vectors pin a timestamp. Taking the instant as an
// argument makes the whole composition reproducible against a fact this project
// did not invent — which is the difference between a test that agrees with the
// implementation and a test that agrees with the venue.
//
// The vectors are the reason for the other unobvious choice too: parameters are
// an ORDERED list, not a url.Values map. See Params.

// aheadTolerance is how far ahead of the venue's own clock a request's timestamp
// may be. It is ONE SECOND, it is the same on both venues, and — this is the
// point — it is NOT configurable (Profiles §6):
//
//	Binance: if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow)
//	Bybit:   server_time - recv_window <= timestamp < server_time + 1000
//
// recvWindow widens only the BEHIND side. So the acceptance window is asymmetric
// while the configuration surface implies it is not, and an operator answering
// timestamp rejections with a larger recvWindow — the documented knob, and the
// only one — repairs one direction and cannot touch the other. A host whose
// clock runs fast keeps failing intermittently with the fix apparently applied.
//
// Hence it lives here as a named constant with the rule beside it, rather than
// as a number in a config file that would imply it could be raised.
const aheadTolerance = 1000 * time.Millisecond

// maxRecvWindow is Binance's documented ceiling. Bybit publishes none, so the
// tighter of the two binds both — the same direction §7.6 takes on every
// venue-behaviour disagreement, and cheap here because a large recvWindow is
// not a thing this system wants (see RecvWindow on SignedRequest).
const maxRecvWindow = 60000 * time.Millisecond

// Param is one request parameter. Order is preserved; see Params.
type Param struct {
	Key   string
	Value string
}

// Params is an ORDERED parameter list, and the ordering is load-bearing twice
// over.
//
// It is not a url.Values because both venues sign the query string as a STRING.
// url.Values.Encode sorts by key, so a map-based signer can only ever produce
// one specific ordering — and it is not the ordering Binance's published test
// vectors use (`symbol,side,type,timeInForce,quantity,price,recvWindow,timestamp`).
// Sorting would therefore have cost this package its only oracle, to buy a
// property no venue asks for.
//
// The second reason is the one that would have bitten in production: the signed
// bytes must be the bytes sent. Any layer free to re-order between signing and
// sending breaks every signature, and the failure surfaces as an authentication
// error — which reads as a credential problem, not an encoding one.
type Params []Param

// Encode renders the parameters as a query string in the order given.
//
// Values are percent-encoded, and that is checked rather than assumed: Binance
// publishes a non-ASCII vector whose signed payload is the ENCODED form
// (`symbol=%EF%BC%91...`), so a signer that signed pre-encoding would agree with
// both ASCII vectors and disagree only with that one. It is the vector worth
// having for exactly that reason.
func (p Params) Encode() string {
	var b strings.Builder
	for i, kv := range p {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(kv.Key))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(kv.Value))
	}
	return b.String()
}

// SignedRequest is one authenticated call, before signing.
type SignedRequest struct {
	// Method is GET or POST.
	Method string

	// Path is the complete request path, leading slash.
	Path string

	// Params are the request's parameters, in the order they must be signed.
	Params Params

	// Body is the raw request body, verbatim, for venues that sign it.
	//
	// It is []byte rather than any/map because Bybit signs the "jsonBodyString"
	// — the bytes actually transmitted. Handing this layer a value to serialize
	// would put a second encoding step between the signature and the wire, and
	// nothing in the type system would require the two encodings to agree.
	Body []byte

	// RecvWindow is how long the venue should keep honouring this request.
	//
	// UNSET IS REFUSED, and the direction is the whole point. Both venues
	// default an absent recvWindow to 5000 ms, so a forgotten field does not
	// fail — it silently adopts the venue's policy. And the venue's policy is
	// not this system's: recvWindow bounds how long a DELAYED, non-idempotent
	// Place can still execute after we have stopped waiting for it. Large
	// converts requests we gave up on into orders that fill; small makes the
	// venue refuse a stale submission, which is a clean EffectNotApplied rather
	// than an EffectUnknown that resolves to Applied minutes later.
	//
	// So the safe direction is SMALL, the venue's default is larger than a
	// trading client should want, and forgetting the field must be a refusal
	// that names it.
	RecvWindow time.Duration
}

// Validate reports whether the request is well enough formed to sign.
func (r SignedRequest) Validate() error {
	switch r.Method {
	case "":
		return ErrSignMethodUnset
	case "GET", "POST":
	default:
		return fmt.Errorf("method %q: %w", r.Method, ErrSignMethodUnsupported)
	}
	if !strings.HasPrefix(r.Path, "/") || strings.ContainsAny(r.Path, "?# ") {
		return fmt.Errorf("path %q: %w", r.Path, ErrPathMalformed)
	}
	if r.RecvWindow <= 0 {
		return ErrRecvWindowUnset
	}
	if r.RecvWindow > maxRecvWindow {
		return fmt.Errorf("recv window %s exceeds %s: %w",
			r.RecvWindow, maxRecvWindow, ErrRecvWindowRange)
	}
	return nil
}

// SignedWire is a request ready to send: what goes on the URL, what goes in the
// headers, and the exact body bytes the signature covers.
type SignedWire struct {
	// Query is the complete query string, signature included where the venue
	// puts it there.
	Query string

	// Header holds the venue's authentication headers.
	Header map[string]string

	// Body is the byte-for-byte body the signature was computed over. Sending
	// anything else invalidates the signature, which is why it is returned here
	// rather than re-read from the request.
	Body []byte
}

// Signer composes one venue's authenticated request.
//
// The two implementations differ in more than a header name — Binance signs the
// query string and puts the signature IN it, Bybit signs a concatenation that
// includes its own api key and puts the signature in a HEADER — so this is a
// genuine per-venue behaviour rather than a table of strings.
type Signer interface {
	// Venue is which exchange this signer speaks for.
	Venue() ids.Venue

	// Sign composes the request at the given server time.
	//
	// The instant is a clock.ServerTime, not a time.Time, because the venue's
	// acceptance test is against ITS clock and ServerTime is the type that
	// carries both the offset and the uncertainty band. See checkServerTime.
	Sign(r SignedRequest, at clock.ServerTime) (SignedWire, error)
}

// checkServerTime refuses to sign when the clock is not good enough to sign
// with, and this is where the asymmetric window becomes a runtime rule.
//
// The binding constraint is the TIGHTER of the two sides. Ahead is fixed at
// aheadTolerance and cannot be configured; behind is recvWindow, which can be
// set smaller than that. A band wider than whichever side is tighter means the
// timestamp we are about to write may already be outside the venue's acceptance
// range — so the request would be rejected on timing grounds, and a timing
// rejection is indistinguishable at the call site from a bad credential.
//
// Refusing here is what makes that distinguishable: the error names the clock.
func checkServerTime(at clock.ServerTime, recvWindow time.Duration) error {
	if err := at.Validate(); err != nil {
		// An uncalibrated venue yields an unset ServerTime. Signing with local
		// time instead is the tempting fallback and it is the wrong one: it
		// produces a request that is accepted while the host's clock happens to
		// be close and rejected when it drifts, which is a fault that appears
		// to come and go with no local change.
		return fmt.Errorf("%w: %w", ErrServerTimeUnusable, err)
	}
	bound := recvWindow
	if aheadTolerance < bound {
		bound = aheadTolerance
	}
	if at.Uncertainty() >= bound {
		return fmt.Errorf("server-clock uncertainty %s is not inside the %s acceptance bound: %w",
			at.Uncertainty(), bound, ErrServerTimeUncertain)
	}
	return nil
}

// hmacHex is the shared primitive: a lowercase hex HMAC-SHA256. Both venues
// specify exactly this, and Bybit says "lowercase HEX string" in as many words.
func hmacHex(secret []byte, payload string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// millis renders an instant as the millisecond epoch stamp both venues take.
func millis(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

// BinanceSigner signs USDⓈ-M and spot requests (Profiles §6).
type BinanceSigner struct {
	venue ids.Venue
	cred  Credential
}

// NewBinanceSigner builds a signer for one Binance credential.
func NewBinanceSigner(venue ids.Venue, cred Credential) (*BinanceSigner, error) {
	if err := venue.Validate(); err != nil {
		return nil, err
	}
	if err := cred.Validate(); err != nil {
		return nil, err
	}
	return &BinanceSigner{venue: venue, cred: cred}, nil
}

// Venue implements Signer.
func (s *BinanceSigner) Venue() ids.Venue { return s.venue }

// Sign implements Signer.
//
// The composition, verbatim from the docs (Profiles §6): the HMAC-SHA256 key is
// the secret and the value is "the query string concatenated with the request
// body"; "please make sure the signature is the end part of your query string or
// request body". The signature parameter is therefore appended AFTER signing and
// is not itself signed.
//
// recvWindow then timestamp are appended in that order because both published
// vectors end that way, and they are otherwise ordered differently from each
// other — so the two agree about the tail while disagreeing about the head,
// which is what makes the tail a checked claim rather than a coincidence of one
// example.
func (s *BinanceSigner) Sign(r SignedRequest, at clock.ServerTime) (SignedWire, error) {
	if err := r.Validate(); err != nil {
		return SignedWire{}, err
	}
	if err := checkServerTime(at, r.RecvWindow); err != nil {
		return SignedWire{}, err
	}

	p := make(Params, 0, len(r.Params)+2)
	p = append(p, r.Params...)
	p = append(p,
		Param{Key: "recvWindow", Value: strconv.FormatInt(r.RecvWindow.Milliseconds(), 10)},
		Param{Key: "timestamp", Value: millis(at.Instant())},
	)

	payload := p.Encode() + string(r.Body)
	sig := hmacHex(s.cred.secret, payload)

	return SignedWire{
		Query:  p.Encode() + "&signature=" + sig,
		Header: map[string]string{"X-MBX-APIKEY": s.cred.Key()},
		Body:   r.Body,
	}, nil
}

// BybitSigner signs V5 requests (Profiles §6).
type BybitSigner struct {
	venue ids.Venue
	cred  Credential
}

// NewBybitSigner builds a signer for one Bybit credential.
func NewBybitSigner(venue ids.Venue, cred Credential) (*BybitSigner, error) {
	if err := venue.Validate(); err != nil {
		return nil, err
	}
	if err := cred.Validate(); err != nil {
		return nil, err
	}
	return &BybitSigner{venue: venue, cred: cred}, nil
}

// Venue implements Signer.
func (s *BybitSigner) Venue() ids.Venue { return s.venue }

// Sign implements Signer.
//
// The composition, verbatim (Profiles §6): GET signs
// "timestamp + API key + recv_window + queryString", POST signs
// "timestamp + API key + recv_window + jsonBodyString". So the api key is inside
// the signed string as well as in its own header, and the signature travels in a
// header rather than in the payload — both unlike Binance.
//
// # This implementation has NO offline oracle, and that is recorded rather than
// # papered over
//
// Bybit's published worked example masks its secret, so its signature cannot be
// reproduced; the one hypothesis that would rescue it (that the secret is the
// literal placeholder) was tested and refuted. The credential-free differential
// probe — sign with an invented key twice, correctly and incorrectly, and read
// the venue's complaint — is refuted too, on both venues, because each checks
// the credential before the signature (Profiles §6).
//
// So the tests below this line check that the implementation matches the
// documented STRING, which is a real claim and a weaker one than Binance's. The
// code is symmetric with its Binance mirror; the evidence is not, and assuming
// otherwise is the specific mistake available here. Validating this composition
// needs a real testnet credential.
func (s *BybitSigner) Sign(r SignedRequest, at clock.ServerTime) (SignedWire, error) {
	if err := r.Validate(); err != nil {
		return SignedWire{}, err
	}
	if err := checkServerTime(at, r.RecvWindow); err != nil {
		return SignedWire{}, err
	}

	ts := millis(at.Instant())
	recv := strconv.FormatInt(r.RecvWindow.Milliseconds(), 10)
	query := r.Params.Encode()

	// GET signs the query string; POST signs the body. Reading the METHOD
	// rather than "whichever of the two is non-empty" is deliberate: the latter
	// silently signs the query on a POST that happens to carry no body, which
	// is a request the venue would reject for a reason the code never states.
	tail := query
	if r.Method == "POST" {
		tail = string(r.Body)
	}
	sig := hmacHex(s.cred.secret, ts+s.cred.Key()+recv+tail)

	return SignedWire{
		Query: query,
		Header: map[string]string{
			"X-BAPI-API-KEY":     s.cred.Key(),
			"X-BAPI-TIMESTAMP":   ts,
			"X-BAPI-RECV-WINDOW": recv,
			"X-BAPI-SIGN":        sig,
		},
		Body: r.Body,
	}, nil
}

// Compile-time proof that both signers satisfy the port (§3.6 rule 2).
var (
	_ Signer = (*BinanceSigner)(nil)
	_ Signer = (*BybitSigner)(nil)
)
