package transport

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file answers §5.3's one question for the REST surface: did the side
// effect happen?
//
// The adapter is the only layer that can answer it — §5.3 forbids the OMS from
// re-deriving "not sent" from the cause — so every path out of a signed call
// leaves through a function here, and each one states the evidence its Effect
// rests on. platform/ratelimit's Penalty doc names this file's other job in
// advance: mapping "403 with retCode 10006" onto a PenaltyKind is venue
// knowledge and belongs beside the client that produces the codes.
//
// # The safe direction is EffectUnknown, and the venues do not help equally
//
// Binance documents the ambiguous case in as many words — on a 503, "the
// execution status is UNKNOWN and could have been a success" — while Bybit's
// error page contains no sentence anywhere about whether a failed or timed-out
// request may nevertheless have executed (Profiles §6). That asymmetry does not
// license different treatment. It means the conservative classification is doing
// MORE work on Bybit, not less: the venue that documents the hazard is the one
// where we could in principle do better, and the one that is silent is the one
// where nothing may be inferred.

// ClassifyTransportError turns a failure that produced NO HTTP status into the
// error the OMS reads.
//
// # The discrimination is whether a connection was ever established
//
// EffectNotApplied is a claim about evidence, never a default, so it is granted
// only where the evidence is structural: net.OpError's Op is "dial", meaning the
// connection itself never came up and therefore no request byte was written.
// Everything else — a write that failed partway, a response that never arrived,
// a TLS error, a context deadline — resolves to EffectUnknown, because Go's HTTP
// client does not report how far a request got and the difference between "the
// send failed" and "the send succeeded and the reply was lost" is invisible from
// here.
//
// Reading a timeout as NotApplied is the specific mistake that produces a
// duplicate live order: the single Place may have executed with its response
// lost, which is exactly the case §5.3 wrote the rule for.
func ClassifyTransportError(op string, err error) *apperror.Error {
	if err == nil {
		return nil
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return apperror.Transport(op, apperror.EffectNotApplied, err)
	}
	return apperror.Transport(op, apperror.EffectUnknown, err)
}

// classifyStatus turns a venue's HTTP STATUS into the error the OMS reads, or
// nil if the status reported acceptance.
//
// It also returns the rate-limit penalty the response earns, or
// PenaltyUnspecified for none — the two travel together because they are read
// off the same status and separating them invites a caller to handle one and
// forget the other.
//
// code is the venue's own error code where the caller has decoded one (Bybit's
// retCode, Binance's negative code) and "" where it has not. It refines the
// penalty; it never softens an Effect.
//
// # It is UNEXPORTED, and the reason is a venue fact rather than tidiness
//
// This function was exported and was the whole classifier, which is correct only
// for a venue whose status carries its verdict. Bybit's does not — it answers a
// refused request with HTTP 200 and a retCode (envelope.go, verified live) — so
// a caller holding a body and reaching for this one would read a rejection as an
// acceptance. There is now exactly one exported way to classify a response that
// has a body, and it is ClassifyResponse; leaving this one reachable would leave
// the wrong answer reachable by the more obvious name.
func classifyStatus(op string, status int, code string, retryAfter time.Duration, detail string) (*apperror.Error, ratelimit.PenaltyKind) {
	switch {
	case status >= 200 && status < 300:
		return nil, ratelimit.PenaltyUnspecified

	case status == http.StatusTooManyRequests:
		// Binance 429: "used when breaking a request rate limit". A refusal, so
		// the request was declined rather than executed.
		return apperror.RateLimited(op, codeOr(code, strconv.Itoa(status)), retryAfter, nil),
			ratelimit.PenaltyThrottle

	case status == http.StatusTeapot:
		// Binance 418: "used when an IP has been auto-banned for continuing to
		// send requests after receiving 429 codes". §7.6 requires an operator
		// alert, which PenaltyBan is how this layer says.
		return apperror.RateLimited(op, codeOr(code, strconv.Itoa(status)), retryAfter, nil),
			ratelimit.PenaltyBan

	case status == http.StatusUnauthorized:
		// Bybit answers auth failures with 401 and content-length: 0 — no
		// retCode and no body at all (Profiles §6, live probe). So this arm must
		// classify correctly with code == "", which is why code is a refinement
		// rather than the thing being switched on.
		return apperror.Unauthorized(op, codeOr(code, strconv.Itoa(status)), nil),
			ratelimit.PenaltyUnspecified

	case status == http.StatusForbidden:
		// 403 is two different facts on Bybit: a permission failure, and
		// §7.6's rate-limit refusal when it carries retCode 10006. The code
		// decides, and with no code the safe reading is the credential one —
		// it stops trading and calls an operator, where a wrong throttle
		// reading would simply retry later into the same refusal.
		if code == bybitCodeRateLimited {
			return apperror.RateLimited(op, code, retryAfter, nil), ratelimit.PenaltyThrottle
		}
		if code == bybitCodeIPBanned {
			return apperror.RateLimited(op, code, retryAfter, nil), ratelimit.PenaltyBan
		}
		return apperror.Unauthorized(op, codeOr(code, strconv.Itoa(status)), nil),
			ratelimit.PenaltyUnspecified

	case status >= 400 && status < 500:
		// Binance: 4XX "used for malformed requests; the issue is on the
		// sender's side" — the venue understood the request and refused it, so
		// nothing was placed. This is §5.3's explicit-rejection evidence class,
		// the one that justifies the Rejected terminal as distinct from
		// AbsentAfterFinality's proved absence.
		return apperror.VenueRejected(op, codeOr(code, strconv.Itoa(status)), detail, nil),
			ratelimit.PenaltyUnspecified

	default:
		// 5XX, and anything outside the ranges above.
		//
		// Binance states it: on a 503 "the API successfully sent the request but
		// not get a response within the timeout period. It is important to NOT
		// treat this as a failure operation; the execution status is UNKNOWN and
		// could have been a success." Bybit states nothing equivalent, so the
		// same conservative answer is reached there by default rather than by
		// documentation.
		//
		// A status below 200 lands here too. That is deliberate: a 1xx reaching
		// this function is a response shape nobody designed for, and an
		// unrecognised outcome is exactly what EffectUnknown is for.
		return apperror.Transport(op, apperror.EffectUnknown,
				fmt.Errorf("%w: status %d %s", ErrVenueUnavailable, status, detail)),
			ratelimit.PenaltyUnspecified
	}
}

// ClassifyAckDecodeFailure is the EffectApplied case, and it exists because it
// is the one that gets forgotten.
//
// A 2xx means the venue accepted the call. If the acknowledgement then fails to
// decode, the order is live and our record of it is what is broken — a different
// instruction from EffectUnknown, which sends the OMS to the venue to ask.
// Nothing needs resolving here; the local record needs correcting.
//
// It is for STATE-CHANGING calls only. A query whose response fails to decode
// applied nothing, and passing one through here would manufacture an order that
// does not exist.
func ClassifyAckDecodeFailure(op string, cause error) *apperror.Error {
	return apperror.Transport(op, apperror.EffectApplied, cause)
}

// The venue error codes this file branches on. Named rather than inline: a bare
// "10006" in a switch is a number whose meaning lives in a doc nobody has open.
const (
	// bybitCodeRateLimited is retCode 10006, "Too many visits. Exceeded the API
	// Rate Limit" — §7.6's 403+10006 hard pause.
	bybitCodeRateLimited = "10006"

	// bybitCodeIPBanned is retCode 10018, "Exceeded the IP Rate Limit" — the
	// unheadered ~600/5s gateway ban §7.6 names, which no X-Bapi-Limit header
	// ever warns about.
	bybitCodeIPBanned = "10018"
)

// codeOr falls back to the HTTP status when the venue gave no code of its own.
//
// An empty Code would make the resulting error indistinguishable from every
// other error of its class in a log, and Bybit's zero-length 401 is not a rare
// path — it is what an expired or IP-blocked key produces, which is the failure
// an unattended process is most likely to meet.
func codeOr(code, fallback string) string {
	if code != "" {
		return code
	}
	return fallback
}

// ErrVenueUnavailable is the cause carried by a 5xx classification. It is a
// sentinel so a caller can branch on "the venue could not answer" without
// re-inspecting the status, and it is deliberately NOT one of the runtime
// sentinels at the top of errors.go: those describe a socket that should
// reconnect, and this one describes a request whose effect is unknown.
var ErrVenueUnavailable = errors.New("transport: venue returned an unavailable status")
