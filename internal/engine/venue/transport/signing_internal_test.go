package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # The oracle
//
// Binance publishes worked HMAC vectors and this file is where they are spent.
// They are an ORACLE in this repo's sense — an independent implementation of the
// claim — because the venue computed them, not us: a signer that agrees with all
// three agrees with the exchange, not with its own author's reading of the docs.
//
// Every vector was re-computed by hand (openssl dgst -sha256 -hmac) on
// 2026-08-02 before being written down here, so a typo in the transcription
// fails immediately rather than pinning a wrong constant. Profiles §6 records
// them with their sources.
//
// Bybit publishes one too and it is UNUSABLE: its secret is masked, so the
// signature cannot be reproduced. The Bybit tests below therefore check the
// documented STRING, which is a weaker claim and is labelled as one wherever it
// appears. Do not read the two venues' test counts as comparable evidence.

const (
	// binanceFuturesSecret and its vector: USDⓈ-M general-info.
	binanceFuturesSecret = "2b5eb11e18796d12d88f13dc27dbbd02c2cc51ff7059765ed9821957d82bb4d9"
	binanceFuturesSig    = "3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9"

	// binanceSpotSecret and its two vectors: the spot rest-api document.
	binanceSpotSecret     = "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j"
	binanceSpotASCIISig   = "c8db56825ae71d6d79447849e617115f4a920fa2acdcab2b053c4b2838bd6b71"
	binanceSpotUnicodeSig = "e1353ec6b14d888f1164ae9af8228a3dbd508bc82eb867db8ab6046442f33ef3"
)

const testVenue = ids.Venue("binance")

// serverTimeAt builds a ServerTime the signers will accept.
func serverTimeAt(t *testing.T, epochMillis int64, uncertainty time.Duration) clock.ServerTime {
	t.Helper()
	st, err := clock.NewServerTime(time.UnixMilli(epochMillis).UTC(), uncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return st
}

func binanceSignerFor(t *testing.T, secret string) *BinanceSigner {
	t.Helper()
	cred, err := NewCredential("irrelevant-to-the-signature", secret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	s, err := NewBinanceSigner(testVenue, cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	return s
}

// TestBinanceSignerMatchesPublishedVectors is the slice's oracle test.
//
// Note what each vector independently pins:
//
//   - The two ASCII vectors come from DIFFERENT documents with DIFFERENT secrets
//     and order their leading parameters differently from each other, while both
//     end recvWindow-then-timestamp. So the tail order is a claim checked twice
//     against disagreeing heads, rather than a coincidence read off one example.
//   - The non-ASCII vector pins that the signed bytes are the PERCENT-ENCODED
//     form. A signer that signed pre-encoding matches both ASCII vectors and
//     fails only this one, which is the entire reason it is here.
func TestBinanceSignerMatchesPublishedVectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		secret string
		params Params
		millis int64
		want   string
	}{
		{
			name:   "usds-m futures general-info",
			secret: binanceFuturesSecret,
			params: Params{
				{"symbol", "BTCUSDT"}, {"side", "BUY"}, {"type", "LIMIT"},
				{"quantity", "1"}, {"price", "9000"}, {"timeInForce", "GTC"},
			},
			millis: 1591702613943,
			want:   binanceFuturesSig,
		},
		{
			name:   "spot rest-api, ascii",
			secret: binanceSpotSecret,
			params: Params{
				{"symbol", "LTCBTC"}, {"side", "BUY"}, {"type", "LIMIT"},
				{"timeInForce", "GTC"}, {"quantity", "1"}, {"price", "0.1"},
			},
			millis: 1499827319559,
			want:   binanceSpotASCIISig,
		},
		{
			// The symbol is supplied RAW. If Params.Encode did not
			// percent-encode it, the payload would differ from the published
			// one and this signature would not match — which is the claim.
			name:   "spot rest-api, non-ascii percent-encoded",
			secret: binanceSpotSecret,
			params: Params{
				{"symbol", "１２３４５６"},
				{"side", "BUY"}, {"type", "LIMIT"}, {"timeInForce", "GTC"},
				{"quantity", "1"}, {"price", "0.1"},
			},
			millis: 1499827319559,
			want:   binanceSpotUnicodeSig,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire, err := binanceSignerFor(t, tc.secret).Sign(SignedRequest{
				Method:     "GET",
				Path:       "/fapi/v1/order",
				Params:     tc.params,
				RecvWindow: 5000 * time.Millisecond,
			}, serverTimeAt(t, tc.millis, 10*time.Millisecond))
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			_, got, found := strings.Cut(wire.Query, "&signature=")
			if !found {
				t.Fatalf("no signature in %q", wire.Query)
			}
			if got != tc.want {
				t.Errorf("signature\n got %s\nwant %s\npayload %s", got, tc.want, wire.Query)
			}
		})
	}
}

// TestBinanceSignatureIsNotItselfSigned checks the documented rule that the
// signature is "the end part of your query string" — appended after signing.
//
// Without this, a signer that folded the signature into its own payload would
// still produce a stable, plausible-looking hex string on every call, and no
// vector-free test would notice.
func TestBinanceSignatureIsNotItselfSigned(t *testing.T) {
	t.Parallel()
	wire, err := binanceSignerFor(t, binanceSpotSecret).Sign(SignedRequest{
		Method:     "GET",
		Path:       "/api/v3/order",
		Params:     Params{{"symbol", "LTCBTC"}, {"side", "BUY"}, {"type", "LIMIT"}, {"timeInForce", "GTC"}, {"quantity", "1"}, {"price", "0.1"}},
		RecvWindow: 5000 * time.Millisecond,
	}, serverTimeAt(t, 1499827319559, 10*time.Millisecond))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	payload, sig, found := strings.Cut(wire.Query, "&signature=")
	if !found {
		t.Fatalf("no signature in %q", wire.Query)
	}
	if strings.Contains(payload, "signature") {
		t.Errorf("signed payload contains the signature parameter: %q", payload)
	}
	if want := hmacHex([]byte(binanceSpotSecret), payload); sig != want {
		t.Errorf("signature covers something other than the payload before it:\n got %s\nwant %s", sig, want)
	}
}

// TestBinanceSignsQueryConcatenatedWithBody is the documented rule "the query
// string concatenated with the request body".
//
// COLLAPSE CHECK, and this one was found by a SURVIVED mutant rather than by
// the checklist: every other Binance fixture here carries an EMPTY body, and
// with an empty body `Encode() + string(body)` and `Encode()` are the same
// expression. Dropping the body from the signed payload therefore changed
// nothing that any test could see. The body is a fixture dimension exactly like
// cardinality and format, and it was pinned to the collapsing value throughout.
//
// The two signatures below must differ, and the one with a body must equal the
// HMAC over the concatenation — the second half is what makes this a check on
// the ORDER of the concatenation and not merely on the body mattering at all.
func TestBinanceSignsQueryConcatenatedWithBody(t *testing.T) {
	t.Parallel()
	const body = `{"note":"a body Binance would concatenate"}`
	at := serverTimeAt(t, 1499827319559, 10*time.Millisecond)
	req := SignedRequest{
		Method:     "POST",
		Path:       "/fapi/v1/order",
		Params:     Params{{"symbol", "BTCUSDT"}, {"side", "BUY"}},
		RecvWindow: 5000 * time.Millisecond,
	}

	withoutBody, err := binanceSignerFor(t, binanceSpotSecret).Sign(req, at)
	if err != nil {
		t.Fatalf("Sign without body: %v", err)
	}
	req.Body = []byte(body)
	withBody, err := binanceSignerFor(t, binanceSpotSecret).Sign(req, at)
	if err != nil {
		t.Fatalf("Sign with body: %v", err)
	}

	if withoutBody.Query == withBody.Query {
		t.Fatal("the body did not affect the signature; it is not part of the signed payload")
	}

	payload, sig, found := strings.Cut(withBody.Query, "&signature=")
	if !found {
		t.Fatalf("no signature in %q", withBody.Query)
	}
	if want := hmacHex([]byte(binanceSpotSecret), payload+body); sig != want {
		t.Errorf("signature is not HMAC(query || body):\n got %s\nwant %s", sig, want)
	}
	// The body follows the query, not the other way round: a signer that
	// concatenated in the opposite order would also fail the check above, but
	// stating it separately names which half is wrong when it does.
	if wrongOrder := hmacHex([]byte(binanceSpotSecret), body+payload); sig == wrongOrder {
		t.Error("signature matches body || query — the concatenation is reversed")
	}
}

// TestRecvWindowReachesTheSignedPayload.
//
// COLLAPSE CHECK, found by a SURVIVED mutant: every other fixture in this file
// signs with recvWindow = 5000 ms, which is ALSO both venues' documented
// default — so "render the configured field" and "write the literal 5000" are
// the same expression everywhere else, and hardcoding it changed nothing any
// test could see. The value that collapses this dimension is the venue's own
// default, which is exactly the value a fixture drifts toward.
//
// Both venues, because §47f's mirror rule applies to the check as much as to
// the fix: on Binance recvWindow is a signed query parameter, on Bybit it is a
// header AND a component of the signed string.
func TestRecvWindowReachesTheSignedPayload(t *testing.T) {
	t.Parallel()
	const unusual = 2500 * time.Millisecond // deliberately not the 5000 default
	if unusual == 5000*time.Millisecond {
		t.Fatal("fixture collapsed: use a recvWindow that is not the venue default")
	}
	at := serverTimeAt(t, 1499827319559, 10*time.Millisecond)

	// Each subtest builds its OWN request. Sharing one and assigning
	// RecvWindow from both is a data race that only -race reports: the plain
	// run passes, and `make ci` is where it surfaces — taking every other test
	// in the package down with it, since a race abort fails the whole binary.
	newReq := func() SignedRequest {
		return SignedRequest{Method: "GET", Path: "/x", Params: Params{{"symbol", "BTCUSDT"}}}
	}

	t.Run("binance", func(t *testing.T) {
		t.Parallel()
		s := binanceSignerFor(t, binanceSpotSecret)
		req := newReq()
		req.RecvWindow = unusual
		got, err := s.Sign(req, at)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		if !strings.Contains(got.Query, "recvWindow=2500&") {
			t.Errorf("configured recvWindow absent from the signed payload: %q", got.Query)
		}
		req.RecvWindow = 5000 * time.Millisecond
		other, err := s.Sign(req, at)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		if got.Query == other.Query {
			t.Error("changing recvWindow did not change the signed payload")
		}
	})

	t.Run("bybit", func(t *testing.T) {
		t.Parallel()
		cred, err := NewCredential("k", "s")
		if err != nil {
			t.Fatalf("NewCredential: %v", err)
		}
		s, err := NewBybitSigner(ids.Venue("bybit"), cred)
		if err != nil {
			t.Fatalf("NewBybitSigner: %v", err)
		}
		req := newReq()
		req.RecvWindow = unusual
		got, err := s.Sign(req, at)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		if got.Header["X-BAPI-RECV-WINDOW"] != "2500" {
			t.Errorf("X-BAPI-RECV-WINDOW = %q, want 2500", got.Header["X-BAPI-RECV-WINDOW"])
		}
		req.RecvWindow = 5000 * time.Millisecond
		other, err := s.Sign(req, at)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		if got.Header["X-BAPI-SIGN"] == other.Header["X-BAPI-SIGN"] {
			t.Error("changing recvWindow did not change the signature")
		}
	})
}

// TestParamsEncodePreservesOrder.
//
// COLLAPSE CHECK: the fixture's order must DIFFER from its alphabetical order,
// or "preserve the caller's order" and "sort by key" are the same predicate and
// the test passes against a url.Values-based signer — which is the exact
// implementation this one was chosen over.
func TestParamsEncodePreservesOrder(t *testing.T) {
	t.Parallel()
	p := Params{{"symbol", "BTCUSDT"}, {"side", "BUY"}, {"price", "9000"}}
	const want = "symbol=BTCUSDT&side=BUY&price=9000"
	const sorted = "price=9000&side=BUY&symbol=BTCUSDT"
	if want == sorted {
		t.Fatal("fixture collapsed: the given order is already alphabetical")
	}
	if got := p.Encode(); got != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

// TestCheckServerTimeBoundaryIsExact.
//
// COLLAPSE CHECK: every case here sits EXACTLY on the bound or one nanosecond
// off it. A fixture strictly inside and one strictly outside would make `>=`
// and `>` the same predicate — the ratelimit freeAt finding, verbatim.
func TestCheckServerTimeBoundaryIsExact(t *testing.T) {
	t.Parallel()
	const recv = 5000 * time.Millisecond // so aheadTolerance is the tighter side

	if err := checkServerTime(serverTimeAt(t, 1, aheadTolerance-1), recv); err != nil {
		t.Errorf("one nanosecond inside the bound was refused: %v", err)
	}
	if err := checkServerTime(serverTimeAt(t, 1, aheadTolerance), recv); !errors.Is(err, ErrServerTimeUncertain) {
		t.Errorf("uncertainty exactly at the bound: got %v, want ErrServerTimeUncertain", err)
	}
}

// TestCheckServerTimeUsesTheTighterSide is the min() that the obvious fixture
// never reaches.
//
// COLLAPSE CHECK: with every recvWindow larger than aheadTolerance, `bound` is
// always aheadTolerance and "take the smaller of the two" is indistinguishable
// from "always use aheadTolerance". This case sets recvWindow BELOW it, so the
// behind side binds and the min() is the only thing that can produce a refusal.
func TestCheckServerTimeUsesTheTighterSide(t *testing.T) {
	t.Parallel()
	const recv = 500 * time.Millisecond // deliberately below aheadTolerance
	if recv >= aheadTolerance {
		t.Fatal("fixture collapsed: recvWindow must be below aheadTolerance here")
	}
	// 600ms is inside aheadTolerance and outside recvWindow, so it is refused
	// only if the tighter of the two is what binds.
	err := checkServerTime(serverTimeAt(t, 1, 600*time.Millisecond), recv)
	if !errors.Is(err, ErrServerTimeUncertain) {
		t.Errorf("uncertainty past the smaller recvWindow: got %v, want ErrServerTimeUncertain", err)
	}
}

// TestSignersRefuseAnUncalibratedVenue: an unset ServerTime must refuse rather
// than fall back to local time. Both signers, because this is the mirror rule.
func TestSignersRefuseAnUncalibratedVenue(t *testing.T) {
	t.Parallel()
	cred, err := NewCredential("key", "secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	bin, err := NewBinanceSigner(testVenue, cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	byb, err := NewBybitSigner(ids.Venue("bybit"), cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}
	req := SignedRequest{Method: "GET", Path: "/x", RecvWindow: time.Second}

	for name, s := range map[string]Signer{"binance": bin, "bybit": byb} {
		if _, err := s.Sign(req, clock.ServerTime{}); !errors.Is(err, ErrServerTimeUnusable) {
			t.Errorf("%s: got %v, want ErrServerTimeUnusable", name, err)
		}
	}
}

// TestRecvWindowUnsetIsRefused: the venue's 5000 ms default is never adopted
// silently.
func TestRecvWindowUnsetIsRefused(t *testing.T) {
	t.Parallel()
	_, err := binanceSignerFor(t, binanceSpotSecret).Sign(
		SignedRequest{Method: "GET", Path: "/x"},
		serverTimeAt(t, 1, time.Millisecond),
	)
	if !errors.Is(err, ErrRecvWindowUnset) {
		t.Errorf("got %v, want ErrRecvWindowUnset", err)
	}
}

// TestBybitSignerComposesTheDocumentedString.
//
// This is a DOCUMENTED-STRING check, not an oracle check: Bybit's published
// signature is unreproducible (masked secret), so the expected value here is
// computed from the documented concatenation rather than supplied by the venue.
// It catches a wrong order or a missing component; it cannot catch a
// misreading of the rule itself.
func TestBybitSignerComposesTheDocumentedString(t *testing.T) {
	t.Parallel()
	const (
		key    = "bybit-api-key"
		secret = "bybit-secret"
	)
	cred, err := NewCredential(key, secret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	s, err := NewBybitSigner(ids.Venue("bybit"), cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}

	const millis int64 = 1658384314791
	wire, err := s.Sign(SignedRequest{
		Method:     "GET",
		Path:       "/v5/order/realtime",
		Params:     Params{{"category", "linear"}, {"symbol", "BTCUSDT"}},
		RecvWindow: 5000 * time.Millisecond,
	}, serverTimeAt(t, millis, time.Millisecond))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// timestamp + API key + recv_window + queryString
	want := hmacHex([]byte(secret), "1658384314791"+key+"5000"+"category=linear&symbol=BTCUSDT")
	if got := wire.Header["X-BAPI-SIGN"]; got != want {
		t.Errorf("X-BAPI-SIGN = %s, want %s", got, want)
	}
	if got := wire.Header["X-BAPI-TIMESTAMP"]; got != "1658384314791" {
		t.Errorf("X-BAPI-TIMESTAMP = %s", got)
	}
	if got := wire.Header["X-BAPI-RECV-WINDOW"]; got != "5000" {
		t.Errorf("X-BAPI-RECV-WINDOW = %s", got)
	}
	if strings.Contains(wire.Query, "sign") {
		t.Errorf("Bybit puts the signature in a header, not the query: %q", wire.Query)
	}
}

// TestBybitSignsBodyOnPostAndQueryOnGet.
//
// COLLAPSE CHECK: the POST fixture carries BOTH a non-empty body and a
// non-empty query. With either one empty, "sign the body because the method is
// POST" and "sign whichever of the two is non-empty" produce the same string,
// and the branch the code actually takes is untestable — which is precisely the
// alternative implementation the comment in Sign rejects.
func TestBybitSignsBodyOnPostAndQueryOnGet(t *testing.T) {
	t.Parallel()
	const (
		key    = "k"
		secret = "s"
		body   = `{"symbol":"BTCUSDT"}`
		query  = "category=linear"
	)
	cred, err := NewCredential(key, secret)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	s, err := NewBybitSigner(ids.Venue("bybit"), cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}
	if body == query {
		t.Fatal("fixture collapsed: body and query must differ")
	}

	at := serverTimeAt(t, 1700000000000, time.Millisecond)
	req := SignedRequest{
		Path:       "/v5/order/create",
		Params:     Params{{"category", "linear"}},
		Body:       []byte(body),
		RecvWindow: 5000 * time.Millisecond,
	}

	req.Method = "POST"
	post, err := s.Sign(req, at)
	if err != nil {
		t.Fatalf("Sign POST: %v", err)
	}
	req.Method = "GET"
	get, err := s.Sign(req, at)
	if err != nil {
		t.Fatalf("Sign GET: %v", err)
	}

	prefix := "1700000000000" + key + "5000"
	if want := hmacHex([]byte(secret), prefix+body); post.Header["X-BAPI-SIGN"] != want {
		t.Errorf("POST signed something other than the body")
	}
	if want := hmacHex([]byte(secret), prefix+query); get.Header["X-BAPI-SIGN"] != want {
		t.Errorf("GET signed something other than the query")
	}
	if post.Header["X-BAPI-SIGN"] == get.Header["X-BAPI-SIGN"] {
		t.Error("POST and GET produced the same signature; the method branch is not reached")
	}
}

// TestSignedWireBodyIsTheSignedBody: the bytes returned for sending are the
// bytes the signature covered. Structural, but it is the property that makes
// "sign what you send" impossible to get wrong downstream.
func TestSignedWireBodyIsTheSignedBody(t *testing.T) {
	t.Parallel()
	cred, err := NewCredential("k", "s")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	s, err := NewBybitSigner(ids.Venue("bybit"), cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}
	body := []byte(`{"a":1}`)
	wire, err := s.Sign(SignedRequest{
		Method: "POST", Path: "/v5/order/create", Body: body, RecvWindow: time.Second,
	}, serverTimeAt(t, 1700000000000, time.Millisecond))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if string(wire.Body) != string(body) {
		t.Errorf("SignedWire.Body = %q, want %q", wire.Body, body)
	}
}
