package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file exists because the HTTP status is not the venue's answer on every
// venue, and slice C1 could not discover that: it sent signed requests and read
// headers, and never decoded a response body.
//
// # The fact, verified live 2026-08-02 (Profiles §7)
//
//	GET api-testnet.bybit.com/v5/market/tickers          -> HTTP 200
//	  {"retCode":10001,"retMsg":"Illegal category",...}
//	GET demo-fapi.binance.com/fapi/v1/depth?symbol=      -> HTTP 400
//	  {"code":-1102,"msg":"Mandatory parameter 'symbol' was not sent..."}
//
// Bybit answers a REFUSED request with a success status. classifyStatus returns
// (nil, PenaltyUnspecified) for any 2xx, so on Bybit a venue rejection reads as
// an acceptance — and on the Place path that is the specific error §5.3 exists
// to prevent: the OMS records an order the venue never created, at Submitted,
// with a possible-delta interval §6.7 must then quiesce to finality before it
// may compensate anything.
//
// The direction is worth stating precisely, because the WORSE outcome is one
// step further on. A Bybit refusal body carries no orderId, so the ack decode
// fails, and ClassifyAckDecodeFailure exists to call a failed decode after a
// success status EffectApplied — "the order is live and our record of it is
// broken". Left alone, an explicit venue REJECTION would therefore have been
// recorded as an order that definitely exists. Profiles §6 named this mechanism
// one layer over, on the zero-length 401: "the parse failure is then what
// classifies the request's effect — which is the §5.3 seam, reached by a path
// nobody designed."
//
// # So the status half is no longer the whole classifier
//
// classifyStatus is unexported now. It is still correct as far as it goes, and
// what it does not go far enough for is a caller with a BODY in hand, which on
// the signed path is every caller. ClassifyResponse is the whole answer and the
// only exported way to ask.

// EnvelopeReader reads a venue's own verdict out of a response body.
//
// It is the LimitReader pattern applied to a second asymmetry: the two venues
// disagree about where the answer lives, so the knowledge of which field to read
// and what its values mean belongs in a per-venue type rather than in a shared
// switch that would have to name both venues' codes and could not say which
// venue it was holding.
type EnvelopeReader interface {
	// Code reports the venue error code carried in a response body, the venue's
	// own message for it, and whether the body reported an error at all.
	//
	// A body that is empty, unparseable, or of a shape this venue does not use
	// for errors reports false. That is not a judgement that the request
	// succeeded — it is the absence of a body-level opinion, and the status
	// classification is what speaks then.
	Code(body []byte) (code, detail string, ok bool)

	// Verdict maps one of this venue's codes onto the OMS's reading, or returns
	// a nil error when the venue's HTTP STATUS is the authority and the code
	// only refines the message.
	//
	// It is called only for a code Code reported.
	Verdict(op, code, detail string, retryAfter time.Duration) (*apperror.Error, ratelimit.PenaltyKind)
}

// ClassifyResponse is the whole classification of one venue answer: what the
// status said, what the body said, and the rule for composing them.
//
// # The composition rule, and it has a direction
//
// The body may CONTRADICT a success status — that is the Bybit case this file
// exists for — but it may never SOFTEN what the status established. §5.3 forbids
// inferring "not sent", so a 5xx that classified EffectUnknown keeps that
// reading even if the body carries a code whose documented meaning is a clean
// refusal. A venue that could not answer is a venue whose account of what it did
// is exactly the thing in doubt.
//
// The last arm is the one worth reading twice: a body reporting an error beside
// a success status, for which the venue's own reader has no rule, is
// EffectUnknown rather than success. Without it this function would have a hole
// of precisely the shape it was written to close — one venue's asymmetry fixed,
// the general case left reading a self-contradicting response as an acceptance.
func ClassifyResponse(op string, env EnvelopeReader, resp Response, retryAfter time.Duration) (*apperror.Error, ratelimit.PenaltyKind) {
	code, detail, ok := env.Code(resp.Body)
	if !ok {
		code, detail = "", ""
	}

	statusErr, statusPenalty := classifyStatus(op, resp.Status, code, retryAfter, detail)
	if !ok {
		return statusErr, statusPenalty
	}

	codeErr, codePenalty := env.Verdict(op, code, detail, retryAfter)
	if codeErr == nil {
		if statusErr == nil {
			return apperror.Transport(op, apperror.EffectUnknown,
					fmt.Errorf("%w: status %d, code %s (%s)", ErrEnvelopeContradictsStatus, resp.Status, code, detail)),
				statusPenalty
		}
		return statusErr, statusPenalty
	}

	penalty := codePenalty
	if penalty == ratelimit.PenaltyUnspecified {
		penalty = statusPenalty
	}
	if statusErr != nil && statusErr.Effect() == apperror.EffectUnknown && codeErr.Effect() != apperror.EffectUnknown {
		// The status could not be answered for; the body's account of what
		// happened is the account of a venue that just told us it was not
		// working. Unknown stands.
		return statusErr, penalty
	}
	return codeErr, penalty
}

// ErrEnvelopeContradictsStatus reports a body carrying an error code beside a
// success status, with no venue rule for the code. It is a sentinel because it
// is the shape a NEW venue quirk arrives as, and an operator seeing it has
// learned something about the venue rather than about this process.
var ErrEnvelopeContradictsStatus = errors.New("transport: response body reports an error the status did not")

// BinanceEnvelope reads Binance's {"code":…,"msg":…} error body.
//
// # Only a NEGATIVE code is an error, and the discrimination is load-bearing
//
// Every documented Binance error code is negative — -1102 (malformed
// parameter), -2015 (invalid API key/IP/permissions), -2022 (ReduceOnly
// rejected), -4120 (conditional order sent to /order) — while a non-negative
// `code` field appears on Binance responses that mean success. Reading any
// `code` as an error would therefore convert some successful calls into
// EffectUnknown, which on the Place path is a resolving query and a trading hold
// for a request that worked.
type BinanceEnvelope struct{}

// Code implements EnvelopeReader.
func (BinanceEnvelope) Code(body []byte) (string, string, bool) {
	var env struct {
		Code *int64 `json:"code"`
		Msg  string `json:"msg"`
	}
	// A body that is not a JSON object — an order ack, a trades array, the
	// zero-length body of a rejected request — carries no code, which is
	// reported as the absence of an opinion rather than as an error of ours.
	if err := json.Unmarshal(body, &env); err != nil || env.Code == nil || *env.Code >= 0 {
		return "", "", false
	}
	return fmt.Sprintf("%d", *env.Code), env.Msg, true
}

// Verdict implements EnvelopeReader.
//
// Binance has NO opinion here, and that is a verified statement rather than a
// gap: the venue documents its own contract as status-first — 4XX "used for
// malformed requests; the issue is on the sender's side", 5XX "the execution
// status is UNKNOWN and could have been a success" (Profiles §6/§7). So the
// status is the authority on this venue and the code refines the message, which
// is what codeOr already does with it.
//
// Declining is not the same as declaring the 2xx-with-negative-code combination
// impossible. It is undocumented, and ClassifyResponse's last arm is what
// answers it — EffectUnknown, from the general rule, rather than from a claim
// about behaviour this project has not observed.
func (BinanceEnvelope) Verdict(string, string, string, time.Duration) (*apperror.Error, ratelimit.PenaltyKind) {
	return nil, ratelimit.PenaltyUnspecified
}

// BybitEnvelope reads Bybit's {"retCode":…,"retMsg":…} envelope.
//
// retCode 0 is success — "retCode … 0 indicates success" (Profiles §7) — and it
// is reported as NO code rather than as the code "0", for the same reason
// BybitLimitReader converts remaining-to-used inside itself: the fact that zero
// means OK is Bybit's, and a shared classifier holding it would be holding one
// venue's convention with nothing saying which venue it was applied to.
type BybitEnvelope struct{}

// Code implements EnvelopeReader.
func (BybitEnvelope) Code(body []byte) (string, string, bool) {
	var env struct {
		RetCode *int64 `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.RetCode == nil || *env.RetCode == 0 {
		return "", "", false
	}
	return fmt.Sprintf("%d", *env.RetCode), env.RetMsg, true
}

// Verdict implements EnvelopeReader.
//
// # The default is EffectUnknown, and it is expensive on purpose
//
// Bybit's error page states causes and nothing else: it contains no sentence
// anywhere about whether a failed or timed-out request may nevertheless have
// executed (Profiles §6). So every code NOT on the list below resolves to
// EffectUnknown, which sends the order to SubmissionUnknown, costs a resolving
// query and holds new trading — for a rejection as ordinary as an insufficient
// balance.
//
// That is the right way round and the cost is the point. The alternative
// default, EffectNotApplied, is silent when it is wrong: it records "no order
// exists" for an order the venue is holding, which is §5.3's forbidden
// inference made by a table of numbers rather than by a caller. The listed codes
// are the ones whose documented meaning is that the venue understood the request
// and declined to act on it, and each one names its evidence.
func (BybitEnvelope) Verdict(op, code, detail string, retryAfter time.Duration) (*apperror.Error, ratelimit.PenaltyKind) {
	switch code {
	case bybitCodeParamError:
		// "Request parameter error" — parsed and refused. A request refused for
		// its parameters cannot have created an order.
		return apperror.VenueRejected(op, code, detail, nil), ratelimit.PenaltyUnspecified

	case bybitCodeTimestampWindow:
		// "The request time exceeds the time window range" — refused at the
		// gate, before any matching. This is the clean EffectNotApplied that
		// Profiles §6 predicts a SMALL recvWindow buys: a stale submission the
		// venue refuses rather than one that executes after we stopped waiting.
		return apperror.VenueRejected(op, code, detail, nil), ratelimit.PenaltyUnspecified

	case bybitCodeRateLimited:
		// "Too many visits. Exceeded the API Rate Limit" — a refusal, so
		// nothing executed. §7.6's hard pause. Bybit does not document which
		// HTTP status carries it, which is why it is classified here as well as
		// on the 403 arm of classifyStatus rather than in one of the two.
		return apperror.RateLimited(op, code, retryAfter, nil), ratelimit.PenaltyThrottle

	case bybitCodeIPBanned:
		// "Exceeded the IP Rate Limit" — the unheadered ~600/5s gateway ban.
		return apperror.RateLimited(op, code, retryAfter, nil), ratelimit.PenaltyBan

	case bybitCodeServerError:
		// "1. internal server error; 2. Service is restarting" — Bybit's
		// analogue of the 5xx Binance documents as UNKNOWN, and Bybit says
		// nothing about it. Named rather than left to the default so the
		// classification is a decision on the record, not a fallthrough.
		return apperror.Transport(op, apperror.EffectUnknown,
			fmt.Errorf("%w: retCode %s (%s)", ErrVenueUnavailable, code, detail)), ratelimit.PenaltyUnspecified
	}

	return apperror.Transport(op, apperror.EffectUnknown,
		fmt.Errorf("%w: retCode %s (%s)", ErrVenueCodeUnclassified, code, detail)), ratelimit.PenaltyUnspecified
}

// ErrVenueCodeUnclassified reports a venue error code with no rule of its own.
//
// It is a sentinel so that the default-deny is VISIBLE: an operator can tell a
// deliberately conservative classification from a documented one, and a code
// that shows up repeatedly here is a code that has earned a row in the table.
var ErrVenueCodeUnclassified = errors.New("transport: venue error code has no classification rule")

// The Bybit codes with rules. Named for the reason the two already here were:
// a bare number in a switch is a meaning that lives in a document nobody has
// open.
const (
	// bybitCodeParamError is retCode 10001, "Request parameter error".
	bybitCodeParamError = "10001"

	// bybitCodeTimestampWindow is retCode 10002, "The request time exceeds the
	// time window range".
	bybitCodeTimestampWindow = "10002"

	// bybitCodeServerError is retCode 10016, "1. internal server error;
	// 2. Service is restarting".
	bybitCodeServerError = "10016"
)

// Compile-time proof that both readers satisfy the port (§3.6 rule 2).
var (
	_ EnvelopeReader = BinanceEnvelope{}
	_ EnvelopeReader = BybitEnvelope{}
)
