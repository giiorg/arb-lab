package transport

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// The two readers under test, as values, so that a composite literal never has
// to appear inside an if-statement condition.
var (
	binanceEnv = BinanceEnvelope{}
	bybitEnv   = BybitEnvelope{}
)

// The live bodies these tests are built from, captured 2026-08-02 against the
// two testnets (Profiles §7). They are fixtures rather than illustrations: the
// Bybit one is the whole reason this file exists, and a hand-written
// approximation of it would have been written with the bug already fixed.
const (
	// HTTP 200, from GET api-testnet.bybit.com/v5/market/tickers with no
	// category. A REFUSAL under a success status.
	bybitRefusalBody = `{"retCode":10001,"retMsg":"Illegal category","result":{},"retExtInfo":{},"time":1785698144141}`

	// HTTP 200, an accepted create-order.
	bybitAcceptBody = `{"retCode":0,"retMsg":"OK","result":{"orderId":"1321003749386327552","orderLinkId":"ab-x"},"retExtInfo":{},"time":1672211918471}`

	// HTTP 400, from GET demo-fapi.binance.com/fapi/v1/depth?symbol=.
	binanceRefusalBody = `{"code":-1102,"msg":"Mandatory parameter 'symbol' was not sent, was empty/null, or malformed."}`

	// An accepted Binance order ack. It carries no "code" field at all, which
	// is the property BinanceEnvelope.Code relies on.
	binanceAckBody = `{"orderId":22542179,"clientOrderId":"ab-x","symbol":"BTCUSDT","status":"NEW"}`
)

// TestBybitEnvelopeCodeReadsTheRefusalAndNotTheAcceptance is the pair that
// matters: retCode 0 must report NO code, or ClassifyResponse would treat every
// successful Bybit call as a body-level error.
func TestBybitEnvelopeCodeReadsTheRefusalAndNotTheAcceptance(t *testing.T) {
	t.Parallel()

	code, detail, ok := bybitEnv.Code([]byte(bybitRefusalBody))
	if !ok || code != "10001" {
		t.Fatalf("Code(refusal) = %q, %q, %v; want \"10001\", the venue message, true", code, detail, ok)
	}
	if detail != "Illegal category" {
		t.Errorf("detail = %q, want the venue's own retMsg", detail)
	}

	if _, _, ok := bybitEnv.Code([]byte(bybitAcceptBody)); ok {
		t.Error("Code(retCode 0) reported an error code — every accepted Bybit call would classify as a failure")
	}
}

// TestBinanceEnvelopeCodeRequiresANegativeCode.
//
// The positive-code case is not hypothetical padding: Binance responses exist
// that carry `"code":200` meaning success, and reading any `code` as an error
// would turn a working call into EffectUnknown — a resolving query and a hold on
// new trading for a request that succeeded.
func TestBinanceEnvelopeCodeRequiresANegativeCode(t *testing.T) {
	t.Parallel()

	code, _, ok := binanceEnv.Code([]byte(binanceRefusalBody))
	if !ok || code != "-1102" {
		t.Fatalf("Code(refusal) = %q, %v; want \"-1102\", true", code, ok)
	}

	for name, body := range map[string]string{
		"an order ack with no code field": binanceAckBody,
		"a non-negative code":             `{"code":200,"msg":"success"}`,
		"an empty body":                   ``,
		"a JSON array":                    `[{"tradeId":1}]`,
	} {
		if _, _, ok := binanceEnv.Code([]byte(body)); ok {
			t.Errorf("Code(%s) reported an error code", name)
		}
	}
}

// TestClassifyResponseCatchesTheRefusalUnderASuccessStatus is the defect this
// file exists for, stated as a test.
//
// Before slice C2a the classifier read the status alone, so this exact body —
// captured live — classified as an ACCEPTED order.
func TestClassifyResponseCatchesTheRefusalUnderASuccessStatus(t *testing.T) {
	t.Parallel()

	resp := Response{Status: 200, Body: []byte(bybitRefusalBody)}
	err, penalty := ClassifyResponse("op", bybitEnv, resp, 0)
	if err == nil {
		t.Fatal("a Bybit refusal under HTTP 200 classified as success — the order would be recorded as live")
	}
	if got := err.Effect(); got != apperror.EffectNotApplied {
		t.Errorf("Effect = %v, want EffectNotApplied — retCode 10001 is a parameter refusal", got)
	}
	if penalty != ratelimit.PenaltyUnspecified {
		t.Errorf("penalty = %v, want none for a parameter error", penalty)
	}

	// The positive control. Without it a ClassifyResponse that refused every
	// 200 would score perfectly above.
	if err, _ := ClassifyResponse("op", bybitEnv, Response{Status: 200, Body: []byte(bybitAcceptBody)}, 0); err != nil {
		t.Errorf("an accepted Bybit order classified as %v, want nil", err)
	}
	if err, _ := ClassifyResponse("op", binanceEnv, Response{Status: 200, Body: []byte(binanceAckBody)}, 0); err != nil {
		t.Errorf("an accepted Binance order classified as %v, want nil", err)
	}
}

// TestClassifyResponseDefaultsAnUnknownBybitCodeToUnknown.
//
// 110007 is "insufficient balance" — an ordinary rejection, and one this table
// deliberately does NOT resolve to NotApplied. Bybit documents nothing anywhere
// about whether a failed request may nevertheless have executed (Profiles §6),
// so the conservative reading is the only one the evidence supports, and it
// costs a resolving query every time it is wrong.
func TestClassifyResponseDefaultsAnUnknownBybitCodeToUnknown(t *testing.T) {
	t.Parallel()

	resp := Response{Status: 200, Body: []byte(`{"retCode":110007,"retMsg":"ab not enough for new order","result":{}}`)}
	err, _ := ClassifyResponse("op", bybitEnv, resp, 0)
	if err == nil {
		t.Fatal("an unlisted retCode classified as success")
	}
	if got := err.Effect(); got != apperror.EffectUnknown {
		t.Errorf("Effect = %v, want EffectUnknown for a code with no rule", got)
	}
	if !errors.Is(err, ErrVenueCodeUnclassified) {
		t.Errorf("error does not carry ErrVenueCodeUnclassified: %v — the default-deny would be invisible to an operator", err)
	}
}

// TestClassifyResponseMapsTheListedBybitCodes walks the table.
//
// Every row states an Effect AND a penalty, because the two are read off the
// same code and a table that got the penalty right while classifying a rate
// limit as a rejection would pass a check on either alone.
func TestClassifyResponseMapsTheListedBybitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		effect  apperror.Effect
		penalty ratelimit.PenaltyKind
	}{
		{"parameter error", "10001", apperror.EffectNotApplied, ratelimit.PenaltyUnspecified},
		{"timestamp outside the window", "10002", apperror.EffectNotApplied, ratelimit.PenaltyUnspecified},
		{"endpoint rate limit", "10006", apperror.EffectNotApplied, ratelimit.PenaltyThrottle},
		{"ip rate limit", "10018", apperror.EffectNotApplied, ratelimit.PenaltyBan},
		{"internal server error", "10016", apperror.EffectUnknown, ratelimit.PenaltyUnspecified},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := []byte(`{"retCode":` + tc.code + `,"retMsg":"m","result":{}}`)
			err, penalty := ClassifyResponse("op", bybitEnv, Response{Status: 200, Body: body}, 0)
			if err == nil {
				t.Fatalf("retCode %s classified as success", tc.code)
			}
			if got := err.Effect(); got != tc.effect {
				t.Errorf("retCode %s: Effect = %v, want %v", tc.code, got, tc.effect)
			}
			if penalty != tc.penalty {
				t.Errorf("retCode %s: penalty = %v, want %v", tc.code, penalty, tc.penalty)
			}
		})
	}
}

// TestClassifyResponseNeverSoftensAnUnknownStatus.
//
// The composition rule with a direction. A 5xx has already established
// EffectUnknown — Binance says so in as many words, "the execution status is
// UNKNOWN and could have been a success" — and a body arriving with it saying
// "parameter error" must not downgrade that to NotApplied. A venue that could
// not answer is a venue whose account of what it did is the thing in doubt.
func TestClassifyResponseNeverSoftensAnUnknownStatus(t *testing.T) {
	t.Parallel()

	resp := Response{Status: 503, Body: []byte(bybitRefusalBody)}
	err, _ := ClassifyResponse("op", bybitEnv, resp, 0)
	if err == nil {
		t.Fatal("a 503 classified as success")
	}
	if got := err.Effect(); got != apperror.EffectUnknown {
		t.Fatalf("Effect = %v, want EffectUnknown — the body's account of a request the venue could not complete "+
			"must not turn into evidence that no order exists", got)
	}
}

// TestClassifyResponseRefusesABodyThatContradictsASuccessStatus is the general
// rule beneath the Bybit special case.
//
// BinanceEnvelope declines to classify any code, so a Binance 2xx carrying a
// negative code reaches the last arm of ClassifyResponse. That combination is
// undocumented rather than impossible, and the answer must be EffectUnknown —
// without this arm the function would have a hole of exactly the shape it was
// written to close.
func TestClassifyResponseRefusesABodyThatContradictsASuccessStatus(t *testing.T) {
	t.Parallel()

	resp := Response{Status: 200, Body: []byte(binanceRefusalBody)}
	err, _ := ClassifyResponse("op", binanceEnv, resp, 0)
	if err == nil {
		t.Fatal("a body reporting -1102 under HTTP 200 classified as success")
	}
	if got := err.Effect(); got != apperror.EffectUnknown {
		t.Errorf("Effect = %v, want EffectUnknown", got)
	}
	if !errors.Is(err, ErrEnvelopeContradictsStatus) {
		t.Errorf("error does not carry ErrEnvelopeContradictsStatus: %v", err)
	}
}

// TestBinanceEnvelopeVerdictDeclines states the null object's behaviour as a
// checked claim rather than leaving it as an unexercised method.
//
// Declining is correct because Binance documents its contract as status-first,
// and the 4xx/5xx arms of classifyStatus already carry it. What makes this
// testable rather than vacuous is the arm above: declining has a CONSEQUENCE,
// and it is EffectUnknown rather than success.
func TestBinanceEnvelopeVerdictDeclines(t *testing.T) {
	t.Parallel()

	err, penalty := binanceEnv.Verdict("op", "-2015", "detail", time.Second)
	if err != nil || penalty != ratelimit.PenaltyUnspecified {
		t.Fatalf("Verdict = %v, %v; want a declined classification on both", err, penalty)
	}
}

// TestClassifyResponseKeepsTheStatusClassificationWhenTheBodyIsSilent is the
// regression guard for the venues' ordinary failures.
//
// Bybit answers an auth failure with HTTP 401 and content-length: 0 (Profiles
// §6), so the body has nothing to say and the status must still classify. This
// is the path that would break if ClassifyResponse required a code.
func TestClassifyResponseKeepsTheStatusClassificationWhenTheBodyIsSilent(t *testing.T) {
	t.Parallel()

	err, _ := ClassifyResponse("op", bybitEnv, Response{Status: 401, Body: nil}, 0)
	if err == nil {
		t.Fatal("a zero-length 401 classified as success")
	}
	if got := err.Effect(); got != apperror.EffectNotApplied {
		t.Errorf("Effect = %v, want EffectNotApplied — an unauthorized request placed nothing", got)
	}
}
